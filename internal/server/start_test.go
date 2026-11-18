// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The one start of the server's tests (AP TestInfrastructure, Block 5),
// the recorded greeting they share, and the kit's clock as the server
// takes it. docs/testing.md, "The start".

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/notify"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
)

// fixture is the greeting recorded from a Cube with firmware 1.1.3, the
// fake Cube's KEQ0000100.
const fixture = "../protocol/cube/testdata/cube-fw0113-greeting.txt"

// recordedGreeting is the fixture's lines, read once for every test.
var recordedGreeting = sync.OnceValues(func() ([]cube.Message, error) { return fakecube.LoadGreeting(fixture) })

// recorded returns a copy of the recorded greeting, the test's to
// change.
func recorded(t testing.TB) []cube.Message {
	t.Helper()
	g, err := recordedGreeting()
	if err != nil {
		t.Fatal(err)
	}
	return slices.Clone(g)
}

// kitClock is the kit's clock as the notify.Clock of the server's
// options (Options.clock, Options.notifyClock).
type kitClock struct{ *testkit.Clock }

func (c kitClock) AfterFunc(d time.Duration, f func()) notify.Timer { return c.Clock.AfterFunc(d, f) }

// rig is a server startServer started for a test, and what the test
// reaches it by.
type rig struct {
	t     *testing.T
	fc    *fakecube.Server             // the fake Cube KEQ0000100; nil with withCubes
	addr  string                       // fc's address
	lines *testkit.Watch[cube.Message] // every line fc received, from its first
	s     *Server
	ts    *httptest.Server               // the server's API
	st    *store.Store                   // nil without a store
	saved bool                           // the Cube is a row of st (stored, onStore)
	rt    *runtime                       // KEQ0000100's runtime; nil with withCubes
	log   *testkit.Watch[testkit.Record] // the server's log, with withLog; nil without
	// stop stops the server: the API, then Run, whose end it waits for.
	// It is the cleanup's too, and does nothing the second time.
	stop func()
}

// startConfig is what the options of startServer chose.
type startConfig struct {
	t        *testing.T
	opts     Options
	optsSet  bool
	edits    []func(*Options)
	st       *store.Store
	stored   bool                      // the Cube is a row of the store
	seed     func(*store.Store, int64) // writes to the store before the start
	greeting []cube.Message
	fake     []func(*fakecube.Server)
	fc       *fakecube.Server // the test's, started: overFake
	fcAddr   string
	cube     []func(*CubeConfig)
	cubes    []CubeConfig // withCubes
	given    bool         // withCubes was given
	also     []CubeConfig // alsoCube
	prep     []func(*Server)
	quiet    bool
	log      bool
	level    slog.Leveler
}

// A startOpt is an option of startServer.
type startOpt func(*startConfig)

// withOptions gives the server's options, which the other options then
// change. The start sets their Cubes (withCubes gives others), and the
// queue's gap to 1 ms where they set none. Given once at most.
func withOptions(o Options) startOpt {
	return func(c *startConfig) {
		if c.optsSet {
			c.t.Fatal("startServer: withOptions given twice")
		}
		c.opts, c.optsSet = o, true
	}
}

// editOptions changes the server's options, after withOptions in
// whatever order the two are given.
func editOptions(f func(*Options)) startOpt {
	return func(c *startConfig) { c.edits = append(c.edits, f) }
}

// stored gives the server a store in a temporary directory, or the one
// its options name, and serves the fake Cube as the store's row
// KEQ0000100, added with the fake's address, as serve does.
func stored() startOpt { return func(c *startConfig) { c.stored = true } }

// onStore is stored over st, the store of an earlier start: its row of
// KEQ0000100 is served where it has one.
func onStore(st *store.Store) startOpt {
	return func(c *startConfig) { c.st, c.stored = st, true }
}

// seedStore writes to the store before the server starts, given the
// Cube's row id; with stored or onStore.
func seedStore(f func(st *store.Store, cubeID int64)) startOpt {
	return func(c *startConfig) { c.seed = f }
}

// withGreeting gives the fake Cube a greeting in place of the recorded
// one.
func withGreeting(g []cube.Message) startOpt { return func(c *startConfig) { c.greeting = g } }

// changeGreeting changes the fake Cube's greeting, the recorded one or
// another given, before the fake starts.
func changeGreeting(f func([]cube.Message) []cube.Message) startOpt {
	return withFake(func(fc *fakecube.Server) { fc.Greeting = f(fc.Greeting) })
}

// rooms gives the fake Cube the rooms fixture (fakecube.RoomsFixture):
// room 1 with a wall thermostat and two radiator thermostats, room 2
// with three radiator thermostats, room 3 with a window contact; the
// fake carries out the commands it takes (fakecube.Server.Apply).
func rooms() startOpt {
	return func(c *startConfig) {
		g, err := fakecube.RoomsFixture()
		if err != nil {
			c.t.Fatal(err)
		}
		c.greeting = g
		c.fake = append(c.fake, func(fc *fakecube.Server) { fc.Apply = true })
	}
}

// withFake changes the fake Cube before it starts.
func withFake(f func(*fakecube.Server)) startOpt {
	return func(c *startConfig) { c.fake = append(c.fake, f) }
}

// overFake serves fc at addr, a fake Cube the test or an earlier start
// started, in place of one of the start's own; the rig does not close
// it.
func overFake(fc *fakecube.Server, addr string) startOpt {
	return func(c *startConfig) { c.fc, c.fcAddr = fc, addr }
}

// again starts over the store and the fake Cube of r, an earlier start,
// as a server starts again after a restart. A store r took through its
// options, not stored, is the test's to give again the same way: again
// does not make its Cube a row of it.
func again(r *rig) startOpt {
	return func(c *startConfig) {
		if r.st != nil && r.saved {
			onStore(r.st)(c)
		}
		overFake(r.fc, r.addr)(c)
	}
}

// withCube changes the configuration of the fake Cube's KEQ0000100.
func withCube(f func(*CubeConfig)) startOpt {
	return func(c *startConfig) { c.cube = append(c.cube, f) }
}

// withCubes serves cubes in place of the fake Cube: the start starts no
// fake and waits for no Cube, for a test of a Cube that is not there
// yet, held by another client, or found by discovery. With none, the
// server serves no Cube.
func withCubes(cubes ...CubeConfig) startOpt {
	return func(c *startConfig) { c.cubes, c.given = cubes, true }
}

// alsoCube serves cfg beside the fake Cube, after it; the start does not
// wait for it. With a store it is a row too, added where the store has
// none.
func alsoCube(cfg CubeConfig) startOpt {
	return func(c *startConfig) { c.also = append(c.also, cfg) }
}

// beforeRun changes the server once it is made, before it runs.
func beforeRun(f func(*Server)) startOpt {
	return func(c *startConfig) { c.prep = append(c.prep, f) }
}

// quiet keeps the server from reading the Cube by itself during a test:
// a poll an hour apart, and no refresh.
func quiet() startOpt { return func(c *startConfig) { c.quiet = true } }

// withLog hands the server a logger at level (nil for Info) whose
// records the rig's log watches.
func withLog(level slog.Leveler) startOpt {
	return func(c *startConfig) { c.log, c.level = true, level }
}

// startServer starts a server for a test, over a fake Cube KEQ0000100
// with the recorded greeting unless the options say otherwise, and
// returns once the Cube is connected, the pump has handled its greeting
// (pumpSync) and the store's writer has written what came before: the
// same for every test. With withCubes it waits for no Cube. The server
// stops at the test's cleanup, or at the rig's stop.
func startServer(t *testing.T, opts ...startOpt) *rig {
	t.Helper()
	c := &startConfig{t: t}
	for _, o := range opts {
		o(c)
	}
	o := c.opts
	if o.Cubes != nil {
		t.Fatal("startServer: the Cubes are withCubes', not the options'")
	}
	if o.Queue.Gap == 0 {
		o.Queue.Gap = time.Millisecond
	}
	for _, f := range c.edits {
		f(&o)
	}
	if c.quiet {
		o.PollInterval, o.RefreshInterval = time.Hour, 0
	}
	r := &rig{t: t, st: o.Store, saved: c.stored}
	if c.st != nil {
		r.st = c.st
	}
	if r.st == nil && c.stored {
		r.st = testStore(t, "")
	}
	o.Store = r.st
	if c.given {
		o.Cubes = c.cubes
	} else {
		r.fc, r.addr = c.fc, c.fcAddr
		if r.fc == nil {
			g := c.greeting
			if g == nil {
				g = recorded(t)
			}
			r.fc = &fakecube.Server{Greeting: g}
			for _, f := range c.fake {
				f(r.fc)
			}
			addr, err := r.fc.Start("127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			// Before anything else that can fail the test; registered
			// before the server's, so closed after its Run ended.
			fc := r.fc
			t.Cleanup(func() { fc.Close() })
			r.addr = addr
		}
		r.lines = cubewatch.Lines(r.fc)
		cfg := CubeConfig{Serial: "KEQ0000100", Addr: r.addr}
		if c.stored {
			cfg = storedCube(t, r.st, cfg, c.seed)
		}
		for _, f := range c.cube {
			f(&cfg)
		}
		o.Cubes = []CubeConfig{cfg}
	}
	for _, cfg := range c.also {
		if c.stored {
			cfg = storedCube(t, r.st, cfg, nil)
		}
		o.Cubes = append(o.Cubes, cfg)
	}
	if c.log {
		o.Logger, r.log = testkit.LogWatch(c.level)
	}
	r.s = New(o)
	for _, f := range c.prep {
		f(r.s)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.s.Run(ctx); close(done) }()
	r.ts = httptest.NewServer(r.s.Handler())
	var once sync.Once
	r.stop = func() {
		once.Do(func() {
			r.ts.Close()
			cancel()
			// Run ends once the server's goroutines have, the steps of
			// the commands it drained are written (5 s at most) and the
			// log's writer has stopped.
			select {
			case <-done:
			case <-time.After(testkit.Scale(10 * time.Second)):
				t.Errorf("the server's Run did not end within %v of its stop", testkit.Scale(10*time.Second))
			}
		})
	}
	t.Cleanup(r.stop)
	if !c.given {
		r.s.mu.RLock()
		r.rt = r.s.cubes[strings.ToLower(o.Cubes[0].Serial)]
		r.s.mu.RUnlock()
		if r.rt == nil {
			t.Fatalf("startServer: no runtime for %q", o.Cubes[0].Serial)
		}
		testkit.Eventually(t, "the cube's connect", r.rt.client.Connected)
		// The greeting's events through the pump: a test that counts the
		// stream's events or the log's lines, or measures a change against
		// the greeting, would otherwise meet them.
		pumpSync(t, r.s)
	}
	if r.st != nil {
		// What the start wrote (server_started) waits in the writer's
		// batch; a line the test writes itself does not, and written in
		// the same millisecond would come before it.
		if err := r.st.FlushEvents(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

// storedCube is cfg as the store's row of its serial, added as a
// configured Cube of the settings file where st has none; seed, where
// given, writes to the store first, given the row's id.
func storedCube(t *testing.T, st *store.Store, cfg CubeConfig, seed func(*store.Store, int64)) CubeConfig {
	t.Helper()
	ctx := context.Background()
	row, ok, err := st.Cube(ctx, cfg.Serial)
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case !ok:
		if row, err = st.AddCube(ctx, store.Cube{Serial: cfg.Serial, Configured: true, Address: cfg.Addr, Source: store.SourceEnv}); err != nil {
			t.Fatal(err)
		}
	case row.Address != cfg.Addr:
		t.Fatalf("startServer: the store's row of %s is at %q, the Cube at %q", cfg.Serial, row.Address, cfg.Addr)
	}
	if seed != nil {
		seed(st, row.ID)
	}
	out, err := CubeFromRow(row)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
