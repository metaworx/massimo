// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/certs"
	"go.metaworx.ch/massimo/internal/cubelan/fakecube"
	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/view"
)

const fixture = "../protocol/cube/testdata/cube-fw0113-greeting.txt"

func countType(fc *fakecube.Server, typ byte) int {
	n := 0
	for _, m := range fc.Received() {
		if m.Type == typ {
			n++
		}
	}
	return n
}

func start(t *testing.T, token string) (*fakecube.Server, *httptest.Server, *Server) {
	t.Helper()
	return startWith(t, Options{Token: token})
}

// startWith starts a server over the fake Cube with opts; the Cube and
// the queue's gap are set here.
func startWith(t *testing.T, opts Options) (*fakecube.Server, *httptest.Server, *Server) {
	t.Helper()
	token := opts.Token
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	opts.Cubes = []CubeConfig{{Serial: "KEQ0537249", Addr: addr}}
	opts.Queue = engine.Options{Gap: time.Millisecond}
	s := New(opts)
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	// Wait for the greeting.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var cubes []cubeView
		getJSON(t, ts, token, "/api/v1/cubes", &cubes)
		if len(cubes) == 1 && cubes[0].Connected {
			return fc, ts, s
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("cube never connected")
	return nil, nil, nil
}

func getJSON(t *testing.T, ts *httptest.Server, token, path string, v any) int {
	t.Helper()
	req, _ := http.NewRequest("GET", ts.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if v != nil && resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

func TestServerReadsAndSetpoints(t *testing.T) {
	fc, ts, s := start(t, "")
	var c cubeView
	if getJSON(t, ts, "", "/api/v1/cubes/keq0537249", &c) != 200 || c.Rooms != 6 || c.Devices != 6 || c.Firmware != "1.1.3" || len(c.RoomList) != 6 {
		t.Fatalf("cube %+v", c)
	}
	var devs []view.Device
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/devices", &devs)
	if len(devs) != 6 || devs[0].Name == "" {
		t.Fatalf("devices %+v", devs)
	}
	var rooms []view.Room
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/rooms", &rooms)
	if len(rooms) != 6 {
		t.Fatalf("rooms %+v", rooms)
	}
	for _, r := range rooms {
		for _, d := range r.Devices {
			if d.Reported != (d.State != nil) || (!d.Reported && r.Ready) {
				t.Fatalf("room %d ready=%v device %+v", r.ID, r.Ready, d)
			}
		}
	}
	var d deviceDetail
	if getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/devices/1514b6", &d) != 200 || d.Thermostat == nil || d.Thermostat.ComfortTemp != 21.5 || d.RoomID != 6 {
		t.Fatalf("device %+v", d)
	}
	if getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/devices/abcdef", nil) != 404 {
		t.Fatal("unknown device not 404")
	}
	// Setpoint on a device, then on a room.
	for _, c := range []struct{ path, body, want string }{
		{"/api/v1/cubes/KEQ0537249/devices/1514b6/setpoint", `{"mode":"manual","temp":21.5}`, "s:AARAAAAAFRS2Bms="},
		{"/api/v1/cubes/KEQ0537249/rooms/6/setpoint", `{"mode":"auto"}`, "s:AARAAAAAFRS2BgA="},
	} {
		req, _ := http.NewRequest("PUT", ts.URL+c.path, strings.NewReader(c.body))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var res map[string]any
		json.NewDecoder(resp.Body).Decode(&res)
		resp.Body.Close()
		if resp.StatusCode != 200 || res["ok"] != true {
			t.Fatalf("%s: %d %v", c.path, resp.StatusCode, res)
		}
		var got []string
		for _, m := range fc.Received() {
			if m.Type == 's' {
				got = append(got, m.String())
			}
		}
		if got[len(got)-1] != c.want {
			t.Fatalf("%s sent %v, want last %s", c.path, got, c.want)
		}
	}
	// Key lock: wakeup of the room, then radiogram 0x80 to the device.
	req, _ := http.NewRequest("PUT", ts.URL+"/api/v1/cubes/KEQ0537249/devices/1514b6/lock", strings.NewReader(`{"locked":false}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("lock: %d", resp.StatusCode)
	}
	var lines []string
	for _, m := range fc.Received() {
		lines = append(lines, m.String())
	}
	if n := len(lines); n < 2 || lines[n-1] != "s:AACAAAAAFRS2AAA=" || !strings.HasPrefix(lines[n-2], "z:1e,G,06") {
		t.Fatalf("lock sent %v", lines)
	}
	// Deleting rooms: occupied ones are refused, unknown ones are 404,
	// an empty one goes to the cube as a metadata write.
	for _, c := range []struct {
		path string
		want int
	}{{"/api/v1/cubes/KEQ0537249/rooms/6", 409}, {"/api/v1/cubes/KEQ0537249/rooms/42", 404}} {
		req, _ = http.NewRequest("DELETE", ts.URL+c.path, nil)
		resp, _ = http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Fatalf("DELETE %s: %d, want %d", c.path, resp.StatusCode, c.want)
		}
	}
	rt := s.cubes["keq0537249"]
	withRoom := *rt.client.Snapshot().Metadata()
	withRoom.Rooms = append(append([]cube.RoomMeta(nil), withRoom.Rooms...), cube.RoomMeta{ID: 7, Name: "Empty"})
	cmds, _ := cube.WriteMetadataCommands(&withRoom)
	_, data, _ := strings.Cut(cmds[0].Data, ",")
	rt.client.ApplyLocal(cube.Message{Type: 'M', Data: "00,01," + data})
	before := countType(fc, 'm')
	req, _ = http.NewRequest("DELETE", ts.URL+"/api/v1/cubes/KEQ0537249/rooms/7", nil)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 200 || countType(fc, 'm') != before+1 || len(rt.client.Snapshot().Metadata().Rooms) != 6 {
		t.Fatalf("DELETE empty room: %d, m lines %d→%d, rooms %d", resp.StatusCode, before, countType(fc, 'm'), len(rt.client.Snapshot().Metadata().Rooms))
	}
	// Bad requests.
	req, _ = http.NewRequest("PUT", ts.URL+"/api/v1/cubes/KEQ0537249/devices/1514b6/setpoint", strings.NewReader(`{"mode":"boost"}`))
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("boost accepted: %d", resp.StatusCode)
	}
	// Backup download.
	resp, _ = http.Get(ts.URL + "/api/v1/cubes/KEQ0537249/backup?format=yaml")
	body := make([]byte, 200)
	n, _ := resp.Body.Read(body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body[:n]), "serial: KEQ0537249") {
		t.Fatalf("backup %d %q", resp.StatusCode, body[:n])
	}
}

func TestServerToken(t *testing.T) {
	_, ts, _ := start(t, "secret")
	if getJSON(t, ts, "", "/api/v1/cubes", nil) != 401 {
		t.Fatal("no token accepted")
	}
	if getJSON(t, ts, "wrong", "/api/v1/cubes", nil) != 401 {
		t.Fatal("wrong token accepted")
	}
	if getJSON(t, ts, "", "/healthz", nil) != 200 {
		t.Fatal("healthz needs a token")
	}
}

func TestHealthzStarted(t *testing.T) {
	before := time.Now()
	_, ts, _ := start(t, "secret")
	var h struct {
		OK        bool      `json:"ok"`
		StartedAt time.Time `json:"started_at"`
	}
	if code := getJSON(t, ts, "", "/healthz", &h); code != 200 {
		t.Fatalf("healthz: %d", code)
	}
	if !h.OK || h.StartedAt.Before(before.Add(-time.Second)) || h.StartedAt.After(time.Now()) {
		t.Fatalf("healthz %+v, want ok and started_at since %v", h, before)
	}
}

func TestPeriodicRefreshReconnects(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0537249", Addr: addr}}, RefreshInterval: 50 * time.Millisecond, Queue: engine.Options{Gap: time.Millisecond}})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	t.Cleanup(func() { cancel(); fc.Close() })
	deadline := time.Now().Add(5 * time.Second)
	for fc.Connections() < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if fc.Connections() < 3 {
		t.Fatalf("connections %d", fc.Connections())
	}
}

func TestDeviceValueTimes(t *testing.T) {
	greeting, err := fakecube.LoadGreeting(fixture)
	if err != nil {
		t.Fatal(err)
	}
	// 1514b6 reports in the greeting: manual, 21 °C, valve 30 %.
	l := &greeting[len(greeting)-1]
	raw, _ := base64.StdEncoding.DecodeString(l.Data)
	copy(raw, []byte{0x0b, 0x15, 0x14, 0xb6, 0x09, 0x12, 0x01, 30, 42, 0, 0, 0})
	l.Data = base64.StdEncoding.EncodeToString(raw)
	fc := &fakecube.Server{Greeting: greeting}
	addr, err := fc.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().Add(-time.Second)
	s := New(Options{Cubes: []CubeConfig{{Serial: "KEQ0537249", Addr: addr}}, Queue: engine.Options{Gap: time.Millisecond}})
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); cancel(); fc.Close() })
	var c cubeView
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && !c.Connected; time.Sleep(20 * time.Millisecond) {
		getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249", &c)
	}
	if !c.Connected || c.ObservedSince.Before(before) {
		t.Fatalf("cube %+v", c)
	}
	var d map[string]any
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/devices/1514b6", &d)
	rec, _ := d["received"].(map[string]any)
	if d["last_seen"] == nil || rec["setpoint"] == nil || rec["valve_pct"] == nil || d["stale"] != nil {
		t.Fatalf("device %v", d)
	}
	var u map[string]any
	getJSON(t, ts, "", "/api/v1/cubes/KEQ0537249/devices/14d08e", &u)
	if _, ok := u["last_seen"]; ok || u["rf_address"] != "14d08e" {
		t.Fatalf("unreported device has times: %v", u)
	}
}

// fakeSource stands in for a certificate source: what the server does
// with one (the HSTS header, /healthz) is tested over httptest's plain
// listener, so no certificate is ever served.
type fakeSource struct{ st certs.Status }

func (f fakeSource) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return nil, errors.New("not served in the test")
}

func (f fakeSource) Status() certs.Status { return f.st }

func TestHSTSAndHealthzTLS(t *testing.T) {
	st := certs.Status{Source: "files", Names: []string{"massimo.example.net"}, NotAfter: time.Now().Add(24 * time.Hour).Truncate(time.Second), Fingerprint: "SHA256:00:11"}
	header := func(ts *httptest.Server, token, path string) (string, map[string]any) {
		t.Helper()
		req, _ := http.NewRequest("GET", ts.URL+path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body)
		return resp.Header.Get("Strict-Transport-Security"), body
	}
	t.Run("a year", func(t *testing.T) {
		_, ts, _ := startWith(t, Options{Token: "secret", TLS: fakeSource{st}, HSTSDays: 365})
		h, body := header(ts, "", "/healthz")
		if h != "max-age=31536000" {
			t.Fatalf("HSTS %q", h)
		}
		tlsInfo, _ := body["tls"].(map[string]any)
		if tlsInfo["source"] != "files" || tlsInfo["fingerprint"] != "SHA256:00:11" || tlsInfo["self_signed"] != false {
			t.Fatalf("healthz tls %v", body["tls"])
		}
		if h, _ := header(ts, "secret", "/api/v1/cubes"); h != "max-age=31536000" {
			t.Fatalf("HSTS on the API %q", h)
		}
	})
	t.Run("cleared", func(t *testing.T) {
		_, ts, _ := startWith(t, Options{TLS: fakeSource{st}, HSTSDays: 0})
		if h, _ := header(ts, "", "/healthz"); h != "max-age=0" {
			t.Fatalf("HSTS %q", h)
		}
	})
	t.Run("plain", func(t *testing.T) {
		_, ts, _ := start(t, "")
		h, body := header(ts, "", "/healthz")
		if h != "" {
			t.Fatalf("HSTS without TLS %q", h)
		}
		if _, ok := body["tls"]; ok {
			t.Fatal("tls reported without TLS")
		}
	})
}

// TestLockoutBehindProxy counts failed token checks per client the proxy
// names, not per proxy, and the stream asks nginx not to buffer.
func TestLockoutBehindProxy(t *testing.T) {
	proxies, _ := ParseTrustedProxies("127.0.0.1,::1")
	_, ts, _ := startWith(t, Options{Token: "secret", TrustedProxies: proxies, AuthFailures: 3})
	try := func(client, token string) int {
		req, _ := http.NewRequest("GET", ts.URL+"/api/v1/cubes", nil)
		req.Header.Set("X-Forwarded-For", client)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	for i := 0; i < 3; i++ {
		if try("192.0.2.10", "wrong") != 401 {
			t.Fatal("not 401")
		}
	}
	if try("192.0.2.10", "secret") != 429 {
		t.Fatal("the client that failed is not locked out")
	}
	if try("192.0.2.11", "secret") != 200 {
		t.Fatal("another client behind the same proxy locked out")
	}
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/cubes/KEQ0537249/events", nil)
	req.Header.Set("Authorization", "Bearer secret")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp, err = http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("stream: %d %q", resp.StatusCode, resp.Header.Get("X-Accel-Buffering"))
	}
}
