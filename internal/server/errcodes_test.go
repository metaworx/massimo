// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/cubelan"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/testkit/cubewatch"
)

// codeName is the form of a code's name and of its parameters' names.
var codeName = regexp.MustCompile(`^[a-z]+(_[a-z]+)*$`)

// formatVerb finds the verbs of a format, %% left out.
var formatVerb = regexp.MustCompile(`%[-+# 0]*[0-9]*(\.[0-9]+)?[a-zA-Z]`)

// placeholder finds the {name} placeholders of a page's text.
var placeholder = regexp.MustCompile(`\{(\w+)\}`)

// errorCodesHeading opens the table of the codes in docs/api-v1.md.
const errorCodesHeading = "### Error codes"

// TestErrorCodes checks the declared codes (errcodes.go): each unique,
// its name and its parameters' lower-case words joined by _, as many
// verbs in its format as it names parameters; its text err.<code> in
// all six tables of the page with exactly its parameters as
// placeholders, and no err.<code> of a code not declared;
// room_thermostat_unplaced's naming the repair by the table's own label
// of it (repair.place); and its row in the table of docs/api-v1.md,
// "Error codes", with its parameters, and no row of a code not
// declared. Every code is declared at the package's level, and every
// refusal names one with as many parameters as it declares.
func TestErrorCodes(t *testing.T) {
	t.Parallel()
	if len(errorCodes) < 10 {
		t.Fatalf("only %d codes declared", len(errorCodes))
	}
	declared := map[string]*errCode{}
	for _, c := range errorCodes {
		if !codeName.MatchString(c.name) {
			t.Errorf("code %q: not lower-case words joined by _", c.name)
		}
		if _, twice := declared[c.name]; twice {
			t.Errorf("code %q declared twice", c.name)
		}
		declared[c.name] = c
		seen := map[string]bool{}
		for _, p := range c.params {
			if !codeName.MatchString(p) || seen[p] {
				t.Errorf("code %s: parameter %q is not lower-case words joined by _, or named twice", c.name, p)
			}
			seen[p] = true
		}
		if n := len(formatVerb.FindAllString(strings.ReplaceAll(c.format, "%%", ""), -1)); n != len(c.params) {
			t.Errorf("code %s: %d verbs in %q, %d parameters %v", c.name, n, c.format, len(c.params), c.params)
		}
	}

	for _, lang := range uiLanguages {
		b, err := uiFiles.ReadFile("ui/i18n/" + lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var table map[string]string
		if err := json.Unmarshal(b, &table); err != nil {
			t.Fatalf("ui/i18n/%s.json: %v", lang, err)
		}
		for _, c := range errorCodes {
			text, ok := table["err."+c.name]
			if !ok {
				t.Errorf("ui/i18n/%s.json: no err.%s", lang, c.name)
				continue
			}
			var got []string
			for _, m := range placeholder.FindAllStringSubmatch(text, -1) {
				got = append(got, m[1])
			}
			if want := slices.Sorted(slices.Values(c.params)); !slices.Equal(slices.Sorted(slices.Values(got)), want) {
				t.Errorf("ui/i18n/%s.json: err.%s has the placeholders %v, its code the parameters %v", lang, c.name, got, want)
			}
		}
		for _, k := range slices.Sorted(maps.Keys(table)) {
			if name, ok := strings.CutPrefix(k, "err."); ok && declared[name] == nil {
				t.Errorf("ui/i18n/%s.json: %s is no declared code's", lang, k)
			}
		}
		if place := table["repair.place"]; place == "" || !strings.Contains(table["err.room_thermostat_unplaced"], place) {
			t.Errorf("ui/i18n/%s.json: err.room_thermostat_unplaced does not name the repair %q (repair.place)", lang, place)
		}
	}

	rows := errorCodeRows(t)
	for _, c := range errorCodes {
		params, ok := rows[c.name]
		if !ok {
			t.Errorf("docs/api-v1.md: code %s has no row in %q", c.name, errorCodesHeading)
			continue
		}
		if !slices.Equal(params, c.params) {
			t.Errorf("docs/api-v1.md: row %s names the parameters %v, its code %v", c.name, params, c.params)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(rows)) {
		if declared[name] == nil {
			t.Errorf("docs/api-v1.md: row %s is no declared code's", name)
		}
	}

	refusalsNameDeclaredCodes(t, declared)
}

// errorCodeRows reads the table of docs/api-v1.md, "Error codes": each
// row's code and the parameters it names, in their order.
func errorCodeRows(t *testing.T) map[string][]string {
	t.Helper()
	doc, err := os.ReadFile("../../docs/api-v1.md")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(doc), "\n")
	start := slices.Index(lines, errorCodesHeading)
	if start < 0 {
		t.Fatalf("docs/api-v1.md has no section %q", errorCodesHeading)
	}
	span := regexp.MustCompile("`([^`]+)`")
	rows := map[string][]string{}
	for _, l := range lines[start+1:] {
		if strings.HasPrefix(l, "## ") || strings.HasPrefix(l, "### ") {
			break
		}
		if !strings.HasPrefix(l, "| `") {
			continue
		}
		cells := strings.Split(strings.Trim(l, "|"), "|")
		if len(cells) != 4 {
			t.Errorf("docs/api-v1.md: a row of %q has %d cells, not 4: %s", errorCodesHeading, len(cells), l)
			continue
		}
		m := span.FindStringSubmatch(cells[0])
		if m == nil {
			t.Errorf("docs/api-v1.md: row %s: the first cell is no code in a code span", l)
			continue
		}
		if _, twice := rows[m[1]]; twice {
			t.Errorf("docs/api-v1.md: row %s appears twice", m[1])
		}
		var params []string
		for _, p := range span.FindAllStringSubmatch(cells[2], -1) {
			params = append(params, p[1])
		}
		rows[m[1]] = params
	}
	if len(rows) == 0 {
		t.Fatalf("docs/api-v1.md: no row under %q", errorCodesHeading)
	}
	return rows
}

// refusalsNameDeclaredCodes reads the package's sources: errorCode is
// called only in a declaration of the package's level, and refuse and
// refuseSaying name a code so declared, with as many parameters after it
// as it names.
func refusalsNameDeclaredCodes(t *testing.T, declared map[string]*errCode) {
	t.Helper()
	files, _ := filepath.Glob("*.go")
	fset := token.NewFileSet()
	var parsed []*ast.File
	vars := map[string]*errCode{} // the package's variables declared by errorCode
	atLevel := map[token.Pos]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, file)
		for _, d := range file.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.VAR {
				continue
			}
			for _, spec := range g.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, v := range vs.Values {
					call, ok := v.(*ast.CallExpr)
					if !ok || !isCall(call, "errorCode") || i >= len(vs.Names) || len(call.Args) == 0 {
						continue
					}
					lit, ok := call.Args[0].(*ast.BasicLit)
					if !ok {
						t.Errorf("%s: errorCode's name is no literal", fset.Position(call.Pos()))
						continue
					}
					name, _ := strconv.Unquote(lit.Value)
					vars[vs.Names[i].Name] = declared[name]
					atLevel[call.Pos()] = true
				}
			}
		}
	}
	refusals := 0
	for _, file := range parsed {
		// A function that declares a name refuse or refuseSaying of its
		// own (oidc.go's) calls that, not the package's.
		shadowed := map[ast.Node]bool{}
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if as, ok := n.(*ast.AssignStmt); ok && as.Tok == token.DEFINE {
					for _, l := range as.Lhs {
						if id, ok := l.(*ast.Ident); ok && (id.Name == "refuse" || id.Name == "refuseSaying") {
							shadowed[fn] = true
						}
					}
				}
				return true
			})
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if shadowed[n] {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			at := fset.Position(call.Pos())
			code := -1
			switch {
			case isCall(call, "errorCode"):
				if !atLevel[call.Pos()] {
					t.Errorf("%s: a code is declared in a declaration of the package's level", at)
				}
			case isCall(call, "refuse"):
				code = 1
			case isCall(call, "refuseSaying"):
				code = 2
			}
			if code < 0 {
				return true
			}
			refusals++
			if len(call.Args) <= code || call.Ellipsis.IsValid() {
				t.Errorf("%s: a refusal names its code and lists its parameters", at)
				return true
			}
			id, ok := call.Args[code].(*ast.Ident)
			if !ok || vars[id.Name] == nil {
				t.Errorf("%s: a refusal names a declared code's variable", at)
				return true
			}
			if c := vars[id.Name]; len(call.Args)-code-1 != len(c.params) {
				t.Errorf("%s: %s with %d parameters, where it declares %v", at, c.name, len(call.Args)-code-1, c.params)
			}
			return true
		})
	}
	if refusals < 10 {
		t.Errorf("only %d refusals with a code found", refusals)
	}
}

// isCall reports whether call calls the package's function name.
func isCall(call *ast.CallExpr, name string) bool {
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == name
}

// rawRequest sends a request and returns the status and the answer's
// body as it came.
func rawRequest(t *testing.T, ts *httptest.Server, method, path, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(b)
}

// unplacedBody is the answer to a setpoint of room 2 of the recorded
// greeting whose only thermostat, 0a0109, the Cube's list has in the
// room and whose C line names none: the owner's case (D4-A), which the
// page's test of it answers with (tools/ui-test/test_error_texts.py).
const unplacedBody = "testdata/room_thermostat_unplaced.json"

// The owner's case (AP ErrorCodes, D4-A): room 2's only thermostat,
// 0a0109, is in room 2 by the Cube's list, and its C line names no room.
// A setpoint, a configuration and a programme of the room are refused
// with 409 and room_thermostat_unplaced, naming the room and the
// thermostat, and a read of its programme with 404; with the thermostat
// in room 1 by both, room 2 has no thermostat, and the same are refused
// with room_no_thermostat. Each answer is checked as it is sent.
func TestRoomThermostatRefusals(t *testing.T) {
	t.Parallel()
	want, err := os.ReadFile(unplacedBody)
	if err != nil {
		t.Fatal(err)
	}
	unplaced := strings.ReplaceAll(string(want), "\r\n", "\n")
	none := "{\n  \"code\": \"room_no_thermostat\",\n  \"error\": \"room 2 has no thermostat\",\n  \"params\": {\n    \"room\": 2\n  }\n}\n"
	const base = "/api/v1/cubes/KEQ0000100/rooms/2"
	writes := []struct{ method, path, body string }{
		{"PUT", base + "/setpoint", `{"mode": "manual", "temp": 19.5}`},
		{"PUT", base + "/setpoint", `{"mode": "boost"}`},
		{"PUT", base + "/config", `{"comfort_temp": 21}`},
		{"PUT", base + "/program", `{"monday": [{"until": "24:00", "temp": 20}]}`},
	}
	for _, tc := range []struct {
		name   string
		change func(*testing.T, []cube.Message)
		want   string
	}{
		{"unplaced", func(t *testing.T, g []cube.Message) {
			withConfigRoom(t, g, 0x0a0109, 0)
		}, unplaced},
		{"none", func(t *testing.T, g []cube.Message) {
			withConfigRoom(t, g, 0x0a0109, 1)
			withMetadata(t, g, func(md *cube.Metadata) {
				for i := range md.Devices {
					if md.Devices[i].RFAddress == 0x0a0109 {
						md.Devices[i].RoomID = 1
					}
				}
			})
		}, none},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rg := startServer(t, quiet(), changeGreeting(func(g []cube.Message) []cube.Message {
				tc.change(t, g)
				return g
			}))
			for _, w := range writes {
				if code, body := rawRequest(t, rg.ts, w.method, w.path, w.body); code != http.StatusConflict || body != tc.want {
					t.Errorf("%s %s %s: %d\n%s\nwant 409\n%s", w.method, w.path, w.body, code, body, tc.want)
				}
			}
			if code, body := rawRequest(t, rg.ts, "GET", base+"/program", ""); code != http.StatusNotFound || body != tc.want {
				t.Errorf("GET %s/program: %d\n%s\nwant 404\n%s", base, code, body, tc.want)
			}
			if n := len(rg.lines.Since(0, cubewatch.Type('s'))); n != 0 {
				t.Errorf("%d radiograms sent", n)
			}
		})
	}
}

// withoutState takes rf's entry out of the greeting's L line: the Cube
// holds no state for it.
func withoutState(t *testing.T, greeting []cube.Message, rf cube.RFAddress) {
	t.Helper()
	for i, g := range greeting {
		if g.Type != 'L' {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(g.Data)
		if err != nil {
			t.Fatal(err)
		}
		var kept []byte
		found := false
		for j := 0; j < len(raw); j += 1 + int(raw[j]) {
			e := raw[j:min(len(raw), j+1+int(raw[j]))]
			if len(e) >= 4 && cube.RFAddress(int(e[1])<<16|int(e[2])<<8|int(e[3])) == rf {
				found = true
				continue
			}
			kept = append(kept, e...)
		}
		if !found {
			t.Fatalf("no L entry for %s", rf)
		}
		greeting[i] = cube.Message{Type: 'L', Data: base64.StdEncoding.EncodeToString(kept)}
		return
	}
	t.Fatal("no L line")
}

// The lookups every write passes answer with their codes: no Cube, no
// room, no device by its address and by its serial.
func TestLookupCodes(t *testing.T) {
	t.Parallel()
	rg := startServer(t, quiet())
	for _, tc := range []struct{ method, path, body, want string }{
		{"PUT", "/api/v1/cubes/KEQ0000999/rooms/2/setpoint", `{"mode": "auto"}`,
			"{\n  \"code\": \"cube_unknown\",\n  \"error\": \"no cube \\\"KEQ0000999\\\"\",\n  \"params\": {\n    \"cube\": \"KEQ0000999\"\n  }\n}\n"},
		{"PUT", "/api/v1/cubes/KEQ0000100/rooms/99/setpoint", `{"mode": "auto"}`,
			"{\n  \"code\": \"room_unknown\",\n  \"error\": \"no room 99\",\n  \"params\": {\n    \"room\": 99\n  }\n}\n"},
		{"PUT", "/api/v1/cubes/KEQ0000100/rooms/99/config", `{"comfort_temp": 21}`,
			"{\n  \"code\": \"room_unknown\",\n  \"error\": \"no room 99\",\n  \"params\": {\n    \"room\": 99\n  }\n}\n"},
		{"PUT", "/api/v1/cubes/KEQ0000100/devices/0a0fff/setpoint", `{"mode": "auto"}`,
			"{\n  \"code\": \"device_unknown\",\n  \"error\": \"no device 0a0fff\",\n  \"params\": {\n    \"device\": \"0a0fff\"\n  }\n}\n"},
		{"PUT", "/api/v1/cubes/KEQ0000100/devices/0a0fff/lock", `{"locked": true}`,
			"{\n  \"code\": \"device_unknown\",\n  \"error\": \"no device 0a0fff\",\n  \"params\": {\n    \"device\": \"0a0fff\"\n  }\n}\n"},
		{"PUT", "/api/v1/cubes/KEQ0000100/devices/MEQ0000999/config", `{"comfort_temp": 21}`,
			"{\n  \"code\": \"device_unknown\",\n  \"error\": \"no device MEQ0000999\",\n  \"params\": {\n    \"device\": \"MEQ0000999\"\n  }\n}\n"},
	} {
		if code, body := rawRequest(t, rg.ts, tc.method, tc.path, tc.body); code != http.StatusNotFound || body != tc.want {
			t.Errorf("%s %s: %d\n%s\nwant 404\n%s", tc.method, tc.path, code, body, tc.want)
		}
	}
}

// commandError gives each result's error its code, its English text the
// error's as before, and the room's members without state as params
// beside no_state; an error of no kind it knows keeps none.
func TestCommandErrorCodes(t *testing.T) {
	t.Parallel()
	// 0a0109, room 2's only thermostat, has no state: its entry is gone
	// from the L line.
	rg := startServer(t, quiet(), changeGreeting(func(g []cube.Message) []cube.Message {
		withoutState(t, g, 0x0a0109)
		return g
	}))
	answer := func(err error) (int, string) {
		t.Helper()
		var he *httpError
		if !errors.As(err, &he) {
			t.Fatalf("%v is no httpError", err)
		}
		rec := httptest.NewRecorder()
		writeJSON(rec, he.status, he.body())
		return he.status, rec.Body.String()
	}
	plain := func(code, msg string) string {
		b, _ := json.Marshal(msg)
		if code == "" {
			return fmt.Sprintf("{\n  \"error\": %s\n}\n", b)
		}
		return fmt.Sprintf("{\n  \"code\": %q,\n  \"error\": %s\n}\n", code, b)
	}
	noState := "engine: command discarded by the cube: the cube holds no state for room 2 members 0a0109 (not reported since the cube's last restart); the cube refuses room commands until they report or are removed"
	for _, tc := range []struct {
		err    error
		room   uint8
		status int
		want   string
	}{
		{engine.ErrSuperseded, 0, http.StatusConflict, plain("command_superseded", "engine: command superseded by a newer one with the same key")},
		{engine.ErrDiscarded, 0, http.StatusBadGateway, plain("command_discarded", "engine: command discarded by the cube")},
		{engine.ErrDiscarded, 2, http.StatusBadGateway, "{\n  \"code\": \"command_discarded_no_state\",\n  \"error\": \"" + noState +
			"\",\n  \"no_state\": [\n    \"0a0109\"\n  ],\n  \"params\": {\n    \"devices\": [\n      \"0a0109\"\n    ],\n    \"room\": 2\n  }\n}\n"},
		{engine.ErrUnwanted, 0, http.StatusBadGateway, plain("command_unwanted", "engine: command no longer wanted when its turn came")},
		{cubelan.ErrNotConnected, 0, http.StatusBadGateway, plain("cube_not_connected", "cubelan: not connected")},
		{fmt.Errorf("wakeup: %w", cubelan.ErrNotConnected), 0, http.StatusBadGateway, plain("cube_not_connected", "wakeup: cubelan: not connected")},
		{cubelan.ErrBusy, 0, http.StatusBadGateway, plain("cube_busy", "cubelan: another command is waiting for its reply")},
		{context.DeadlineExceeded, 0, http.StatusBadGateway, plain("command_timed_out", "context deadline exceeded")},
		{errors.New("something else"), 0, http.StatusBadGateway, plain("", "something else")},
	} {
		status, body := answer(rg.s.commandError(rg.rt, engine.Result{Err: tc.err}, tc.room))
		if status != tc.status || body != tc.want {
			t.Errorf("%v, room %d: %d\n%s\nwant %d\n%s", tc.err, tc.room, status, body, tc.status, tc.want)
		}
	}
	if err := rg.s.commandError(rg.rt, engine.Result{}, 0); err != nil {
		t.Errorf("no error: %v", err)
	}
}
