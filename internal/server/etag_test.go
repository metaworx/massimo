// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// programWith reads a programme route, with its tag from the body and the
// header.
func programWith(t *testing.T, url string) (map[string]any, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode != 200 {
		t.Fatalf("GET %s: %d %v", url, resp.StatusCode, body)
	}
	tag, _ := body["etag"].(string)
	if tag == "" || resp.Header.Get("ETag") != `"`+tag+`"` {
		t.Fatalf("GET %s: tag %q, header %q", url, tag, resp.Header.Get("ETag"))
	}
	return body, tag
}

// putProgram writes a programme with If-Match where given.
func putProgram(t *testing.T, url, ifMatch, body string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("PUT", url, strings.NewReader(body))
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// A programme's tag covers the room: a write with an old tag is refused,
// one with the current tag goes, a write still on its way counts, and a
// member's own write changes the room's tag.
func TestProgramTag(t *testing.T) {
	fc, ts, _, _ := startRoomsOpts(t, roomsOpts{applyAfter: 600 * time.Millisecond, fast: true, slow: true})
	base := ts.URL + "/api/v1/cubes/KEQ0000100/"
	room, tag := programWith(t, base+"rooms/2/program")
	if room["pending"] != false || room["program_consistent"] != true || room["source"] != "0a0204" {
		t.Fatalf("room 2's programme %v", room)
	}
	if _, dtag := programWith(t, base+"devices/0a0205/program"); dtag != tag {
		t.Fatalf("a member's tag %s, the room's %s", dtag, tag)
	}
	if _, vtag := programWith(t, base+"devices/room-2/program"); vtag != tag {
		t.Fatalf("the virtual wall thermostat's tag %s", vtag)
	}

	sent := len(fc.Received())
	monday := `{"monday":[{"temp":17,"until":"06:00"},{"temp":21.5,"until":"22:00"},{"temp":17,"until":"24:00"}]}`
	if code, body := putProgram(t, base+"rooms/2/program", `"p0000"`, monday); code != 412 || body["etag"] != tag {
		t.Fatalf("an old tag: %d %v", code, body)
	}
	for _, m := range fc.Received()[sent:] {
		if m.Type == 's' {
			t.Fatalf("a refused write sent %s", m)
		}
	}
	code, body := putProgram(t, base+"rooms/2/program", `W/"`+tag+`"`, monday)
	if code != 200 || body["etag"] == tag || body["etag"] == "" {
		t.Fatalf("the current tag: %d %v", code, body)
	}
	written := body["etag"].(string)
	// On its way: the room shows the programme written, pending.
	room, now := programWith(t, base+"rooms/2/program")
	if now != written || room["pending"] != true {
		t.Fatalf("on its way: %s pending %v, want %s", now, room["pending"], written)
	}
	if code, _ := putProgram(t, base+"rooms/2/program", `"`+tag+`"`, monday); code != 412 {
		t.Fatalf("a tag from before the write on its way: %d", code)
	}
	// Confirmed: the same tag, no longer pending.
	eventually(t, "the written programme confirmed", func() bool {
		room, now = programWith(t, base+"rooms/2/program")
		return room["pending"] == false
	})
	if now != written {
		t.Fatalf("confirmed under %s, written as %s", now, written)
	}
	// One member's own write: the room's tag changes, its members differ.
	if code, _ := putProgram(t, base+"devices/0a0206/program", `"`+written+`"`, `{"sunday":[{"temp":19,"until":"24:00"}]}`); code != 200 {
		t.Fatalf("a member's write: %d", code)
	}
	room, member := programWith(t, base+"rooms/2/program")
	if member == written || room["program_consistent"] != false {
		t.Fatalf("after a member's write: %s consistent %v", member, room["program_consistent"])
	}
	// Each write is confirmed before the next, so that no re-read is in
	// flight when it is sent (cubelan.ErrBusy).
	eventually(t, "the member's programme confirmed", func() bool {
		room, _ = programWith(t, base+"rooms/2/program")
		return room["pending"] == false
	})
	// Without If-Match a write is unconditional.
	if code, _ := putProgram(t, base+"rooms/2/program", "", monday); code != 200 {
		t.Fatalf("without If-Match: %d", code)
	}
}

// A confirmed programme change is a config event of the member, with
// what changed and the new tag, and of its room's virtual wall
// thermostat.
func TestProgramConfigEvents(t *testing.T) {
	_, ts, _, _ := startRoomsOpts(t, roomsOpts{fast: true, slow: true})
	evs := readStreamURL(t, ts.URL+"/api/v1/cubes/KEQ0000100/events?devices=all")
	for i := 0; i < 10; i++ {
		next(t, evs)
	}
	if code, body := putProgram(t, ts.URL+"/api/v1/cubes/KEQ0000100/rooms/2/program", "", `{"tuesday":[{"temp":18,"until":"24:00"}]}`); code != 200 {
		t.Fatalf("write: %d %v", code, body)
	}
	seen := map[string]bool{}
	deadline := time.After(5 * time.Second)
	for !(seen["0a0204"] && seen["000002"]) {
		select {
		case ev := <-evs:
			if ev.kind != "config" {
				continue
			}
			var e event
			json.Unmarshal([]byte(ev.data), &e)
			if len(e.Changed) == 1 && e.Changed[0] == "program" && e.Etag != "" {
				seen[e.Device] = true
			}
		case <-deadline:
			t.Fatalf("config events with changed and etag: %v", seen)
		}
	}
}
