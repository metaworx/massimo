// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serve

// A host name and an IP of one Cube tie the plain lines to its row
// (AP StartConfiguration, Block 21, D57-A): where no row's address is
// the plain MASSIMO_CUBE_ADDRESS as written, PlainRow looks the names
// up, within 2 s in all. The lookups are the tests' own, without the
// network.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/env"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
)

// plainAt is the plain lines with the address addr.
func plainAt(addr string) store.FileCube {
	return store.FileCube{Values: map[string]store.FileValue{store.FieldAddress: {Variable: env.Address, Value: addr}}}
}

// rowsAt are configured rows at addrs, KEQ0000100 the first's, then
// KEQ0000101 and on.
func rowsAt(addrs ...string) []store.Cube {
	var rows []store.Cube
	for i, a := range addrs {
		rows = append(rows, store.Cube{Serial: fmt.Sprintf("KEQ00001%02d", i), Address: a, Configured: true, Source: store.SourceUI})
	}
	return rows
}

// lookups is a HostLookup that answers from its names and counts what it
// was asked, in order; a name it does not know fails, as one the
// resolver does not know.
type lookups struct {
	mu    sync.Mutex
	names map[string][]string
	asked []string
}

// lookup is the HostLookup.
func (l *lookups) lookup(_ context.Context, host string) ([]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.asked = append(l.asked, host)
	if a, ok := l.names[host]; ok {
		return a, nil
	}
	return nil, errors.New("no such host")
}

// hosts returns the hosts asked, sorted.
func (l *lookups) hosts() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Sorted(slices.Values(l.asked))
}

// A name ties to the row at one of its IPs, and an IP to the row whose
// name has it, at the same port alone; IPv4 in IPv6 is IPv4. Where
// several rows have an IP of the plain host's, none is tied, and they
// are returned.
func TestPlainRowByLookup(t *testing.T) {
	t.Parallel()
	names := map[string][]string{
		"cube.example":  {"192.0.2.1"},
		"cube.lan":      {"192.0.2.1"},
		"several.lan":   {"2001:db8::7", "192.0.2.7", "192.0.2.1"},
		"mapped.lan":    {"::ffff:192.0.2.1"},
		"elsewhere.lan": {"192.0.2.9"},
	}
	for _, c := range []struct {
		plain   string
		rows    []string
		want    string   // the serial tied, "" for none
		several []string // the serials of several rows that would tie
	}{
		{"cube.example", []string{"192.0.2.1:62910"}, "KEQ0000100", nil},
		{"192.0.2.1", []string{"cube.example"}, "KEQ0000100", nil},
		{"cube.example:62910", []string{"cube.lan:62910"}, "KEQ0000100", nil},
		{"several.lan", []string{"192.0.2.9", "192.0.2.1"}, "KEQ0000101", nil},
		{"mapped.lan", []string{"192.0.2.1"}, "KEQ0000100", nil},
		{"cube.example", []string{"192.0.2.1:62911"}, "", nil},
		{"cube.example:62911", []string{"192.0.2.1"}, "", nil},
		{"cube.example", []string{"elsewhere.lan"}, "", nil},
		{"unknown.lan", []string{"192.0.2.1"}, "", nil},
		{"several.lan", []string{"192.0.2.7", "192.0.2.9", "[2001:db8::7]:62910"}, "", []string{"KEQ0000100", "KEQ0000102"}},
		{"cube.example", []string{"cube.lan", "192.0.2.1"}, "", []string{"KEQ0000100", "KEQ0000101"}},
	} {
		l := &lookups{names: names}
		row, ok, several := PlainRow(t.Context(), rowsAt(c.rows...), plainAt(c.plain), l.lookup)
		var serials []string
		for _, r := range several {
			serials = append(serials, r.Serial)
		}
		if ok != (c.want != "") || row.Serial != c.want || !slices.Equal(serials, c.several) {
			t.Errorf("%s beside %v: tied %v to %q, several %v; want %q, %v (asked %v)", c.plain, c.rows, ok, row.Serial, serials, c.want, c.several, l.hosts())
		}
	}
}

// A lookup that fails, or does not answer, ties nothing, and PlainRow
// returns within its bound, 2 s for all of its names together, also
// where the lookup does not heed its context.
func TestPlainRowLookupFails(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	for name, lookup := range map[string]HostLookup{
		"failing": func(context.Context, string) ([]string, error) { return nil, errors.New("server misbehaving") },
		"heeding": func(ctx context.Context, _ string) ([]string, error) { <-ctx.Done(); return nil, ctx.Err() },
		"deaf":    func(context.Context, string) ([]string, error) { <-release; return []string{"192.0.2.1"}, nil },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			type result struct {
				row store.Cube
				ok  bool
			}
			done := make(chan result, 1)
			start := time.Now()
			go func() {
				row, ok, _ := PlainRow(t.Context(), rowsAt("a.lan", "b.lan"), plainAt("cube.example"), lookup)
				done <- result{row, ok}
			}()
			select {
			case r := <-done:
				if r.ok {
					t.Fatalf("tied to %+v", r.row)
				}
				if took := time.Since(start); took > plainLookupTime+testkit.Scale(500*time.Millisecond) {
					t.Fatalf("took %v, the bound is %v", took, plainLookupTime)
				}
			case <-time.After(plainLookupTime + testkit.Scale(2*time.Second)):
				t.Fatalf("no answer %v after the bound of %v", testkit.Scale(2*time.Second), plainLookupTime)
			}
		})
	}
}

// Only names are looked up, each once, and only where no row is the
// plain address as written and a row is at its port.
func TestPlainRowLooksUpNamesAlone(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		plain string
		rows  []string
		asked []string
		tied  bool
	}{
		{"Cube.example", []string{"cube.example:62910"}, nil, true},
		{"192.0.2.1", []string{"192.0.2.10", "[2001:db8::1]"}, nil, false},
		{"cube.example", []string{"192.0.2.1:62911", "other.lan:62911"}, nil, false},
		{"cube.example", []string{"cube.lan", "192.0.2.9", "nowhere.lan", "nowhere.lan:62910", "late.lan:62911"}, []string{"cube.example", "cube.lan", "nowhere.lan"}, true},
	} {
		l := &lookups{names: map[string][]string{"cube.example": {"192.0.2.1"}, "cube.lan": {"192.0.2.1"}}}
		_, ok, _ := PlainRow(t.Context(), rowsAt(c.rows...), plainAt(c.plain), l.lookup)
		if got := l.hosts(); ok != c.tied || !slices.Equal(got, c.asked) {
			t.Errorf("%s beside %v: tied %v, asked %v; want %v, %v", c.plain, c.rows, ok, got, c.tied, c.asked)
		}
	}
}

// At the start, the plain lines by a name of a stored Cube's IP are that
// Cube's: one Cube served, the setting naming it, the note in the log.
// Tied before, the setting names it and nothing is looked up; a lookup
// that fails leaves the two Cubes of today, and the start goes on.
func TestServePlainNameOfAStoredCube(t *testing.T) {
	clearCubeEnv(t)
	ctx := context.Background()
	t.Setenv(env.Address, "cube.example")
	st := testStore(t, "")
	addUICube(t, st, "KEQ0000100", "192.0.2.1:62910")

	failing := &lookups{}
	su, err := setupCubes(ctx, st, Config{lookupHost: failing.lookup}, quietLog())
	sn, serr := st.PlainSerial(ctx)
	if err != nil || serr != nil || len(su.cubes) != 2 || sn != "" || !slices.Equal(failing.hosts(), []string{"cube.example"}) {
		t.Fatalf("a failing lookup: cubes %+v, the setting %q, asked %v: %v %v", su.cubes, sn, failing.hosts(), err, serr)
	}

	l := &lookups{names: map[string][]string{"cube.example": {"192.0.2.1"}}}
	logger, logs := testkit.LogWatch(nil)
	su, err = setupCubes(ctx, st, Config{lookupHost: l.lookup}, logger)
	sn, serr = st.PlainSerial(ctx)
	if err != nil || serr != nil || len(su.cubes) != 1 || su.cubes[0].Serial != "KEQ0000100" || sn != "KEQ0000100" {
		t.Fatalf("not tied: cubes %+v, the setting %q: %v %v", su.cubes, sn, err, serr)
	}
	logs.Has(t, 0, "the note of the tie", testkit.Contains("serial=KEQ0000100", "addr=cube.example"))

	again := &lookups{names: l.names}
	su, err = setupCubes(ctx, st, Config{lookupHost: again.lookup}, quietLog())
	if err != nil || len(su.cubes) != 1 || len(again.hosts()) != 0 {
		t.Fatalf("the next start: cubes %+v, asked %v: %v", su.cubes, again.hosts(), err)
	}
	if strings.Contains(su.cubes[0].Addr, "cube.example") {
		t.Fatalf("the row's address taken from the plain lines: %+v", su.cubes[0])
	}
}

// A name with the IPs of several stored Cubes ties none at the start:
// the plain lines are served as today, beside them, nothing is saved,
// and the log names the Cubes.
func TestServePlainNameOfSeveralCubes(t *testing.T) {
	clearCubeEnv(t)
	ctx := context.Background()
	t.Setenv(env.Address, "several.lan")
	st := testStore(t, "")
	addUICube(t, st, "KEQ0000100", "192.0.2.7")
	addUICube(t, st, "KEQ0000101", "192.0.2.1:62910")
	l := &lookups{names: map[string][]string{"several.lan": {"192.0.2.1", "192.0.2.7"}}}
	logger, logs := testkit.LogWatch(nil)
	su, err := setupCubes(ctx, st, Config{lookupHost: l.lookup}, logger)
	sn, serr := st.PlainSerial(ctx)
	if err != nil || serr != nil || len(su.cubes) != 3 || sn != "" {
		t.Fatalf("cubes %+v, the setting %q: %v %v", su.cubes, sn, err, serr)
	}
	logs.Has(t, 0, "the warning naming both", testkit.Msg(plainSeveralNote, "addr", "several.lan", "cubes", "KEQ0000100 at 192.0.2.7, KEQ0000101 at 192.0.2.1:62910"))
	if n := logs.Count(0, testkit.Msg(server.PlainServedNote)); n != 0 {
		t.Fatalf("%d notes of a tie", n)
	}
}
