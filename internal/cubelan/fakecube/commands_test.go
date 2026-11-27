// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

// The fake Cube carries out every command the protocol package builds,
// or names it as one it only answers, with its reason (AP SteadyTests,
// Block 7), so that it cannot silently do less than a Cube, as it once
// ignored 0x22 and left a device in room 0 after a reconnect.
//
// The commands are read from the protocol package's source, not from a
// list: Go cannot enumerate functions by reflection, and a table of the
// builders kept beside them would be one more list to fall behind. The
// source is the one list that cannot: a builder added there, in any of
// its files, fails this test until commandCases sends it. The cases are
// the one list kept here by hand, and the source holds them to it both
// ways.
//
// What the fake only answers is named in this file, not in the fake:
// the fake consults no list, since it answers what it does not carry
// out as a Cube does (A:, or S: for s: and r:).

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/protocol/cube"
)

// protocolDir holds the source of the command builders.
const protocolDir = "../../protocol/cube"

// notCommands are the exported functions of the protocol package that
// give a Message without building a command, each with its reason.
var notCommands = map[string]string{
	"ParseLine":  "parses a line the Cube sent",
	"SplitLines": "splits the lines the Cube sent",
}

// acknowledgedOnly are the commands the fake answers without carrying
// them out, by commandKey, each with its reason: what they change on a
// Cube is not in the fake's lines. A command leaves the list once the
// fake carries it out, and its case then checks its effect.
var acknowledgedOnly = map[string]string{
	"u:":     "a Cube keeps its portal URL in its own C line; the fake keeps the greeting's and has no portal client",
	"i:":     "a Cube keeps its portal polling in its own C line; the fake keeps the greeting's and has no portal client",
	"e:":     "the fake has no portal key: it answers E: with the data it was given",
	"d:":     "the fake has no portal key: it answers D: with the data it was given",
	"z:":     "the fake's devices take every command at once: it has no radio wakeups to show",
	"v:":     "a Cube keeps its daylight-saving rules in its own C line; the fake keeps the greeting's, and its H line's clock",
	"r:":     "a Cube clears the devices' error flags (docs/cube-protocol.md); the fake does not yet, and keeps those its control and a refused 0x23 set",
	"s:0x20": "a device's links are the one datum no line of a Cube shows (docs/cube-protocol.md): nothing to change",
	"s:0x21": "a device's links are the one datum no line of a Cube shows (docs/cube-protocol.md): nothing to change",
}

// commandKey names the command of a line: its letter and colon, and for
// a radio command its code too, "s:0x40".
func commandKey(m cube.Message) (string, error) {
	if m.Type != 's' {
		return string(m.Type) + ":", nil
	}
	rc, err := cube.ParseRadioCommand(m.Data)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("s:0x%02x", rc.Command), nil
}

// protocolSource is what the protocol package's source declares: its
// command builders, the exported functions and variables that give a
// Message or several, and its radio command codes, the constants RF…,
// by name.
type protocolSource struct {
	builders []string
	radio    map[string]byte
}

// readProtocol reads the protocol package's source in dir: the files
// that build for this system, as the package that the test calls is
// built.
func readProtocol(dir string) (protocolSource, error) {
	pkg, err := build.ImportDir(dir, 0)
	if err != nil {
		return protocolSource{}, err
	}
	src := protocolSource{radio: map[string]byte{}}
	consts := map[string]ast.Expr{}
	var codes []string
	fset := token.NewFileSet()
	for _, name := range pkg.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return protocolSource{}, err
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() && givesMessage(d.Type.Results) {
					src.builders = append(src.builders, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, n := range vs.Names {
						var value ast.Expr
						if i < len(vs.Values) {
							value = vs.Values[i]
						}
						switch {
						case d.Tok == token.VAR && n.IsExported() && (isMessage(vs.Type) || isMessageLiteral(value)):
							src.builders = append(src.builders, n.Name)
						case d.Tok == token.CONST:
							if value != nil {
								consts[n.Name] = value
							}
							if n.IsExported() && strings.HasPrefix(n.Name, "RF") {
								codes = append(codes, n.Name)
							}
						}
					}
				}
			}
		}
	}
	for _, name := range codes {
		v, err := constValue(consts, name, consts[name])
		if err != nil {
			return protocolSource{}, err
		}
		if v < 0 || v > 0xff {
			return protocolSource{}, fmt.Errorf("%s: %d is not a radio command code", name, v)
		}
		src.radio[name] = byte(v)
	}
	sort.Strings(src.builders)
	return src, nil
}

// isMessage reports whether e is the type Message or []Message.
func isMessage(e ast.Expr) bool {
	if a, ok := e.(*ast.ArrayType); ok && a.Len == nil {
		e = a.Elt
	}
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "Message"
}

// isMessageLiteral reports whether e is a Message literal.
func isMessageLiteral(e ast.Expr) bool {
	lit, ok := e.(*ast.CompositeLit)
	return ok && isMessage(lit.Type)
}

// givesMessage reports whether a function's results hold a Message or
// several.
func givesMessage(results *ast.FieldList) bool {
	if results == nil {
		return false
	}
	return slices.ContainsFunc(results.List, func(f *ast.Field) bool { return isMessage(f.Type) })
}

// constValue is the value of the constant name, whose expression is e:
// a number, a conversion of one, or another constant; anything else is
// an error, so that a code the test cannot read fails it.
func constValue(consts map[string]ast.Expr, name string, e ast.Expr) (int64, error) {
	for range 8 {
		switch x := e.(type) {
		case *ast.BasicLit:
			if x.Kind == token.INT {
				return strconv.ParseInt(x.Value, 0, 64)
			}
		case *ast.ParenExpr:
			e = x.X
			continue
		case *ast.CallExpr: // a conversion, byte(…)
			if len(x.Args) == 1 {
				e = x.Args[0]
				continue
			}
		case *ast.Ident:
			if v, ok := consts[x.Name]; ok {
				e = v
				continue
			}
		}
		return 0, fmt.Errorf("%s: its value (%T) is not one the test reads from the source", name, e)
	}
	return 0, fmt.Errorf("%s: its value is too deep for the test to read", name)
}

// commandCase sends a builder's command to a fake.
type commandCase struct {
	builder string // its name in the protocol package
	variant string // which of its commands, where it builds several
	build   func() ([]cube.Message, error)
	setup   func(t *testing.T, s *Server) // the fake before it starts; nil for none
	// carried reads the command's effect from the fake once its lines
	// were sent and answered (ex), and says what it misses: "" where the
	// fake carried the command out; nil for a command acknowledgedOnly
	// names.
	carried func(r *commandRun, ex exchange) string
}

// name is the case's name in a failure and in its subtest.
func (c commandCase) name() string {
	if c.variant == "" {
		return c.builder
	}
	return c.builder + " " + c.variant
}

// commandRun is a case's fake, its address, and a client of it that has
// read its greeting.
type commandRun struct {
	t    *testing.T
	s    *Server
	addr string
	c    *client
}

// exchange is what the fake answered to lines sent: a line for each,
// until a read failed (err).
type exchange struct {
	replies []cube.Message
	err     error
}

// answered reports whether each of n lines was answered with a line of
// type typ.
func (ex exchange) answered(n int, typ byte) bool {
	return ex.err == nil && len(ex.replies) == n &&
		!slices.ContainsFunc(ex.replies, func(m cube.Message) bool { return m.Type != typ })
}

func (ex exchange) String() string {
	var s []string
	for _, m := range ex.replies {
		s = append(s, m.String())
	}
	if ex.err != nil {
		s = append(s, "error: "+ex.err.Error())
	}
	return "[" + strings.Join(s, " ") + "]"
}

// send sends lines on the run's connection and reads a line for each.
func (r *commandRun) send(lines ...cube.Message) exchange {
	var ex exchange
	for _, m := range lines {
		if _, ex.err = r.c.conn.Write([]byte(m.Wire())); ex.err != nil {
			return ex
		}
		reply, err := r.c.r.Read()
		if ex.err = err; err != nil {
			return ex
		}
		ex.replies = append(ex.replies, reply)
	}
	return ex
}

// lines are the fake's lines as a new client reads them: its greeting
// and its F line.
func (r *commandRun) lines() []string {
	c := dial(r.t, r.addr)
	var out []string
	for _, m := range c.greet {
		out = append(out, m.String())
	}
	return append(out, c.send(cube.CmdGetNTP).String())
}

// startCommandRun serves the rooms fixture with the commands applied,
// after setup, and connects to it.
func startCommandRun(t *testing.T, setup func(*testing.T, *Server)) *commandRun {
	t.Helper()
	greeting, err := RoomsFixture()
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Greeting: greeting, Apply: true}
	if setup != nil {
		setup(t, s)
	}
	addr, err := s.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return &commandRun{t: t, s: s, addr: addr, c: dial(t, addr)}
}

// firstDifference is where two lists of lines first differ.
func firstDifference(before, after []string) string {
	for i := range max(len(before), len(after)) {
		var b, a string
		if i < len(before) {
			b = before[i]
		}
		if i < len(after) {
			a = after[i]
		}
		if a != b {
			return fmt.Sprintf("line %d was %q, is %q", i+1, b, a)
		}
	}
	return "none"
}

// commandCases send each builder of the protocol package with
// representative arguments to a fake serving the rooms fixture, and
// check its effect where the fake carries it out.
func commandCases() []commandCase {
	const joining cube.RFAddress = 0x0a0b0c
	room2 := cube.Target{RFAddress: 0x0a0204, RoomID: 2, Room: true}
	dev := cube.Target{RFAddress: 0x0a0204}
	one := func(m cube.Message) func() ([]cube.Message, error) {
		return func() ([]cube.Message, error) { return []cube.Message{m}, nil }
	}
	oneOrError := func(m cube.Message, err error) func() ([]cube.Message, error) {
		return func() ([]cube.Message, error) { return []cube.Message{m}, err }
	}
	winter := cube.TimeZoneRule{Name: "CET", Month: 10, Hour: 3, OffsetSeconds: 3600}
	summer := cube.TimeZoneRule{Name: "CEST", Month: 3, Hour: 2, OffsetSeconds: 7200}
	day := cube.DayProgram{{Temp: 17, Until: 360}, {Temp: 21.5, Until: 1320}, {Temp: 17, Until: 1440}}
	temps := cube.TemperatureSettings{Comfort: 23, Eco: 16.5, Max: 29, Min: 5, Offset: 1.5, WindowOpen: 12, WindowOpenMinutes: 20}
	valve := cube.ValveSettings{BoostMinutes: 10, BoostValvePct: 60, DecalcDay: cube.Tuesday, DecalcHour: 11, MaxValvePct: 100}
	return []commandCase{
		{builder: "CmdList", build: one(cube.CmdList), carried: func(r *commandRun, ex exchange) string {
			if !ex.answered(1, 'L') || ex.replies[0] != lastList(r.c.greet) {
				return fmt.Sprintf("l: answered %s, not its L line", ex)
			}
			return ""
		}},
		{builder: "CmdQuit", build: one(cube.CmdQuit), carried: func(_ *commandRun, ex exchange) string {
			if len(ex.replies) != 0 || !errors.Is(ex.err, io.EOF) {
				return fmt.Sprintf("q: answered %s, not the connection's end", ex)
			}
			return ""
		}},
		{builder: "CmdGetNTP", build: one(cube.CmdGetNTP),
			setup: func(_ *testing.T, s *Server) { s.NTP = []string{"ntp.example.org"} },
			carried: func(_ *commandRun, ex exchange) string {
				if !ex.answered(1, 'F') || ex.replies[0].Data != "ntp.example.org" {
					return fmt.Sprintf("f: answered %s, not its NTP server", ex)
				}
				return ""
			}},
		{builder: "CmdSetNTP", build: one(cube.CmdSetNTP("ntp.example.net", "ntp.example.com")), carried: func(r *commandRun, ex exchange) string {
			const want = "ntp.example.net,ntp.example.com"
			if !ex.answered(1, 'F') || ex.replies[0].Data != want {
				return fmt.Sprintf("f: with servers answered %s, not the servers", ex)
			}
			if again := r.send(cube.CmdGetNTP); !again.answered(1, 'F') || again.replies[0].Data != want {
				return fmt.Sprintf("a later f: answered %s, not the servers set", again)
			}
			return ""
		}},
		{builder: "CmdInclusion", build: one(cube.CmdInclusion(time.Minute)),
			setup: func(t *testing.T, s *Server) {
				p, err := PairingFrom(s.Greeting, 0x0a0204, joining, "TEQ0000001", time.Millisecond)
				if err != nil {
					t.Fatal(err)
				}
				s.Pair = p
			},
			carried: func(r *commandRun, ex exchange) string {
				if !ex.answered(1, 'N') || ex.replies[0] != NewDeviceLine(r.s.Pair.Device) {
					return fmt.Sprintf("n: with a device to join brought %s, not its N line", ex)
				}
				if l := r.send(cube.CmdList); !l.answered(1, 'L') || !listed(r.t, l.replies[0], joining) {
					return "the device that joined is not in the L line"
				}
				return ""
			}},
		{builder: "CmdCancelInclusion", build: one(cube.CmdCancelInclusion), carried: func(_ *commandRun, ex exchange) string {
			if !ex.answered(1, 'N') || ex.replies[0].Data != "" {
				return fmt.Sprintf("x: answered %s, not the inclusion's end", ex)
			}
			return ""
		}},
		{builder: "CmdDeleteDevices", build: one(cube.CmdDeleteDevices(true, 0x0a0207)), carried: func(r *commandRun, ex exchange) string {
			if !ex.answered(1, 'A') {
				return fmt.Sprintf("t: answered %s", ex)
			}
			if l := r.send(cube.CmdList); !l.answered(1, 'L') || listed(r.t, l.replies[0], 0x0a0207) {
				return "the deleted device is still in the L line"
			}
			if config, _ := greeted(r.t, dial(r.t, r.addr).greet, 0x0a0207); config {
				return "a new greeting still holds the deleted device's C line"
			}
			return ""
		}},
		{builder: "WriteMetadataCommands",
			build: func() ([]cube.Message, error) {
				greeting, err := RoomsFixture()
				if err != nil {
					return nil, err
				}
				md, err := cube.ParseMetadata([]cube.MetadataPart{metadataPart(greeting)})
				if err != nil {
					return nil, err
				}
				md.Rooms[0].Name = "Stube"
				return cube.WriteMetadataCommands(md)
			},
			carried: func(r *commandRun, ex exchange) string {
				if !ex.answered(1, 'A') {
					return fmt.Sprintf("m: answered %s", ex)
				}
				if md := metadataOf(r.t, dial(r.t, r.addr).greet); md.Rooms[0].Name != "Stube" {
					return fmt.Sprintf("a new greeting's M line names room 1 %q, not the name written", md.Rooms[0].Name)
				}
				return ""
			}},
		{builder: "CmdSetPortalURL", build: one(cube.CmdSetPortalURL("http://192.0.2.10:8080"))},
		{builder: "CmdSetPortalInterval", build: one(cube.CmdSetPortalInterval(-1))},
		{builder: "CmdEncrypt", build: one(cube.CmdEncrypt([]byte("massimo")))},
		{builder: "CmdDecrypt", build: one(cube.CmdDecrypt([]byte("massimo")))},
		{builder: "CmdWakeup", build: one(cube.CmdWakeup(30, cube.WakeupRoom, "02"))},
		{builder: "CmdSetTimeZone", build: oneOrError(cube.CmdSetTimeZone(winter, summer))},
		{builder: "CmdSetTime", build: oneOrError(cube.CmdSetTime(winter, summer, time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)))},
		{builder: "CmdResetError", build: one(cube.CmdResetError(0x0a0204)),
			setup: func(t *testing.T, s *Server) {
				flagged := true
				if err := s.SetState(0x0a0204, State{Error: &flagged}); err != nil {
					t.Fatal(err)
				}
			}},
		{builder: "CmdSetTemperature", build: oneOrError(cube.CmdSetTemperature(room2, 22.5, cube.ModeManual, cube.Until{})), carried: func(r *commandRun, ex exchange) string {
			if !ex.answered(1, 'S') {
				return fmt.Sprintf("s: answered %s", ex)
			}
			st := r.c.states()
			for _, rf := range []cube.RFAddress{0x0a0204, 0x0a0205, 0x0a0206} {
				if s := st[rf]; s.Mode != cube.ModeManual || s.Setpoint == nil || *s.Setpoint != 22.5 {
					return fmt.Sprintf("%s is in %s, want manual at 22.5", rf, s.Mode)
				}
			}
			return ""
		}},
		{builder: "CmdSetDayProgram", build: func() ([]cube.Message, error) { return cube.CmdSetDayProgram(room2, cube.Monday, day) }, carried: func(r *commandRun, ex exchange) string {
			if !ex.answered(1, 'S') {
				return fmt.Sprintf("s: answered %s", ex)
			}
			if got := r.c.config(0x0a0204).Thermostat.WeekProgram[cube.Monday]; !slices.Equal(got, day) {
				return fmt.Sprintf("Monday is %+v in the C line, want %+v", got, day)
			}
			return ""
		}},
		{builder: "CmdSetTemperatures", build: oneOrError(cube.CmdSetTemperatures(dev, temps)), carried: func(r *commandRun, ex exchange) string {
			if !ex.answered(1, 'S') {
				return fmt.Sprintf("s: answered %s", ex)
			}
			th := r.c.config(0x0a0204).Thermostat
			if th.ComfortTemp != temps.Comfort || th.EcoTemp != temps.Eco || th.TempOffset != temps.Offset || th.WindowOpenMins != temps.WindowOpenMinutes {
				return fmt.Sprintf("the C line holds comfort %.1f, eco %.1f, offset %.1f, window open %d min; want %+v",
					th.ComfortTemp, th.EcoTemp, th.TempOffset, th.WindowOpenMins, temps)
			}
			return ""
		}},
		{builder: "CmdSetValveConfig", build: oneOrError(cube.CmdSetValveConfig(dev, valve)), carried: func(r *commandRun, ex exchange) string {
			if !ex.answered(1, 'S') {
				return fmt.Sprintf("s: answered %s", ex)
			}
			th := r.c.config(0x0a0204).Thermostat
			if th.BoostMinutes != valve.BoostMinutes || th.BoostValvePct != valve.BoostValvePct || th.DecalcDay != valve.DecalcDay || th.DecalcHour != valve.DecalcHour {
				return fmt.Sprintf("the C line holds a boost of %d min at %d %%, descaling on %s at %d h; want %+v",
					th.BoostMinutes, th.BoostValvePct, th.DecalcDay, th.DecalcHour, valve)
			}
			return ""
		}},
		{builder: "CmdLinkPartner", variant: "add", build: one(cube.CmdLinkPartner(dev, true, 0x0a0201, cube.DeviceWallThermostat))},
		{builder: "CmdLinkPartner", variant: "remove", build: one(cube.CmdLinkPartner(dev, false, 0x0a0201, cube.DeviceWallThermostat))},
		{builder: "CmdSetGroup", variant: "set", build: one(cube.CmdSetGroup(0x0a0204, 1, false)), carried: func(r *commandRun, ex exchange) string {
			if !ex.answered(1, 'S') {
				return fmt.Sprintf("s: answered %s", ex)
			}
			if room := r.c.config(0x0a0204).RoomID; room != 1 {
				return fmt.Sprintf("the C line names room %d, want 1", room)
			}
			return ""
		}},
		{builder: "CmdSetGroup", variant: "remove", build: one(cube.CmdSetGroup(0x0a0204, 2, true)), carried: func(r *commandRun, ex exchange) string {
			if !ex.answered(1, 'S') {
				return fmt.Sprintf("s: answered %s", ex)
			}
			if room := r.c.config(0x0a0204).RoomID; room != 0 {
				return fmt.Sprintf("the C line names room %d, want none", room)
			}
			return ""
		}},
		{builder: "CmdLockControls", build: one(cube.CmdLockControls(dev, true)), carried: func(r *commandRun, ex exchange) string {
			if !ex.answered(1, 'S') {
				return fmt.Sprintf("s: answered %s", ex)
			}
			if !r.c.states()[0x0a0204].PanelLocked {
				return "the L entry is not locked"
			}
			return ""
		}},
		{builder: "CmdSetDisplayActual", build: one(cube.CmdSetDisplayActual(cube.Target{RFAddress: 0x0a0201}, true)), carried: func(r *commandRun, ex exchange) string {
			if !ex.answered(1, 'S') {
				return fmt.Sprintf("s: answered %s", ex)
			}
			if d := r.c.states()[0x0a0201].DisplayActual; d == nil || !*d {
				return "the wall thermostat's L entry does not show the measured temperature"
			}
			return ""
		}},
	}
}

// metadataPart is a greeting's M line of one block.
func metadataPart(greeting []cube.Message) cube.MetadataPart {
	for _, m := range greeting {
		if m.Type == 'M' {
			if p, err := cube.ParseMetadataPart(m.Data); err == nil {
				return p
			}
		}
	}
	return cube.MetadataPart{}
}

// Every builder of the protocol package has a case; each case's command
// is carried out by the fake, its effect visible in the fake's lines, or
// named in acknowledgedOnly, and then answered and changing none of the
// fake's lines. A builder without a case, a command neither carried out
// nor named, and a radio command code no case sends fail the test,
// naming it.
func TestFakeHandlesEveryCommand(t *testing.T) {
	t.Parallel()
	src, err := readProtocol(protocolDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(src.builders) == 0 || len(src.radio) == 0 {
		t.Fatalf("no builders or no radio command codes in %s: did the package move?", protocolDir)
	}
	cases := commandCases()
	cased := map[string]bool{}
	for _, c := range cases {
		cased[c.builder] = true
	}
	built := map[string]bool{}
	for _, b := range src.builders {
		built[b] = true
		_, not := notCommands[b]
		switch {
		case not && cased[b]:
			t.Errorf("%s: notCommands names it, and commandCases sends it", b)
		case !not && !cased[b]:
			t.Errorf("%s: a builder of %s that commandCases does not send: add its case, which checks its effect on the fake or finds its command in acknowledgedOnly; a function that builds no command goes into notCommands", b, protocolDir)
		}
	}
	for _, b := range slices.Sorted(maps.Keys(cased)) {
		if !built[b] {
			t.Errorf("%s: not a builder of %s; take its case out of commandCases", b, protocolDir)
		}
	}
	for _, b := range slices.Sorted(maps.Keys(notCommands)) {
		if !built[b] {
			t.Errorf("%s: notCommands names it, and %s has no such function returning a Message", b, protocolDir)
		}
	}

	keys := make([]string, len(cases))
	lines := make([][]cube.Message, len(cases))
	sent := map[string]bool{}
	for i, c := range cases {
		ls, err := c.build()
		if err != nil || len(ls) == 0 {
			t.Errorf("%s: built %v, %v", c.name(), ls, err)
			continue
		}
		for _, m := range ls {
			k, err := commandKey(m)
			if err != nil {
				t.Errorf("%s: %s: %v", c.name(), m, err)
			}
			if keys[i] != "" && k != keys[i] {
				t.Errorf("%s: builds %s and %s; give each its own case", c.name(), keys[i], k)
			}
			keys[i] = k
		}
		lines[i] = ls
		sent[keys[i]] = true
	}
	for _, name := range slices.Sorted(maps.Keys(src.radio)) {
		if k := fmt.Sprintf("s:0x%02x", src.radio[name]); !sent[k] {
			t.Errorf("%s (%s): a radio command code that no case of commandCases sends", name, k)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(acknowledgedOnly)) {
		if !sent[k] {
			t.Errorf("acknowledgedOnly names %s, which no case sends", k)
		}
	}

	for i, c := range cases {
		if lines[i] == nil {
			continue
		}
		t.Run(c.name(), func(t *testing.T) {
			t.Parallel()
			checkCommand(t, c, keys[i], lines[i])
		})
	}
}

// checkCommand sends a case's lines, whose command is key, to a fake of
// its own, and fails t where the fake neither carries the command out
// nor is said to only answer it, or does other than it is said to.
func checkCommand(t *testing.T, c commandCase, key string, lines []cube.Message) {
	reason, only := acknowledgedOnly[key]
	switch {
	case c.carried != nil && only:
		t.Fatalf("%s (%s): its effect is checked here, and acknowledgedOnly says the fake only answers it (%s): take it off the list", c.name(), key, reason)
	case c.carried == nil && !only:
		t.Fatalf("%s (%s): the fake neither carries it out nor is it named: carry it out in the fake (apply.go, fakecube.go) and check its effect in its case, or name %s in acknowledgedOnly with its reason", c.name(), key, key)
	}
	r := startCommandRun(t, c.setup)
	if c.carried != nil {
		if miss := c.carried(r, r.send(lines...)); miss != "" {
			t.Fatalf("%s (%s): the fake does not carry it out: %s; carry it out in the fake (apply.go, fakecube.go), or name %s in acknowledgedOnly with its reason", c.name(), key, miss, key)
		}
		return
	}
	before := r.lines()
	if ex := r.send(lines...); ex.err != nil || len(ex.replies) != len(lines) {
		t.Fatalf("%s (%s): acknowledgedOnly names it, and the fake did not answer it: %s", c.name(), key, ex)
	}
	if after := r.lines(); !slices.Equal(before, after) {
		t.Fatalf("%s (%s): acknowledgedOnly says the fake only answers it (%s), and its lines changed, %s: check its effect in its case and take it off the list", c.name(), key, reason, firstDifference(before, after))
	}
}
