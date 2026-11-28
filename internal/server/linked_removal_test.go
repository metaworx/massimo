// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// lan is the address the owner's requests and the integration's come
// from.
const lan = "192.168.1.20:5000"

// actsAs is GET /api/v1/me for the integration's user id at the level
// given: the user the call acts as, the link and the capabilities left.
func actsAs(t *testing.T, s *Server, key, id, level string) meView {
	t.Helper()
	rec := sendFromWith(s, "GET", "/api/v1/me", lan, key, "", as(id, level))
	var me meView
	if json.Unmarshal(rec.Body.Bytes(), &me); rec.Code != 200 || me.User == nil || me.ActsFor == nil {
		t.Fatalf("%s at %s: %d %s", id, level, rec.Code, rec.Body.String())
	}
	return me
}

// linkedToAnna makes the integration's user haAnna at its first call and
// links it to the Massimo user anna, a manager with a preference of her
// own, through the API as the owner (secret); the call then acts as anna
// and reads her preferences.
func linkedToAnna(t *testing.T, s *Server, key string, i store.Integration) store.User {
	t.Helper()
	st := s.opts.Store
	anna, err := st.AddUser(t.Context(), store.User{Name: "anna", Role: store.RoleManager}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetPreference(t.Context(), anna.ID, "theme", json.RawMessage(`"anna's"`)); err != nil {
		t.Fatal(err)
	}
	actsAs(t, s, key, haAnna, LevelControl)
	if rec := sendFrom(s, "PUT", "/api/v1/integrations/"+i.UUID+"/users/"+haAnna, lan, "secret", `{"user":"anna"}`); rec.Code != 200 {
		t.Fatalf("the link: %d %s", rec.Code, rec.Body.String())
	}
	if me := actsAs(t, s, key, haAnna, LevelControl); me.User.Name != "anna" || !me.ActsFor.Linked || !strings.Contains(preferencesOf(t, s, key), "anna's") {
		t.Fatalf("linked: %+v %+v", me.User, me.ActsFor)
	}
	return anna
}

// preferencesOf is GET /api/v1/me/preferences of the call for haAnna.
func preferencesOf(t *testing.T, s *Server, key string) string {
	t.Helper()
	rec := sendFromWith(s, "GET", "/api/v1/me/preferences", lan, key, "", as(haAnna, LevelControl))
	if rec.Code != 200 {
		t.Fatalf("preferences: %d %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// removedPastTheStore marks the user id removed and nothing else, as a
// store of before this version left an integration's user removed while
// linked: its identity stayed.
func removedPastTheStore(t *testing.T, st *store.Store, id int64) {
	t.Helper()
	if _, err := storekit.Raw(t, st).ExecContext(t.Context(), `UPDATE users SET deleted_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
}

// orphanUnseen checks what a client sees of the integration i's user
// haAnna whose own user was removed while it stayed linked to anna: the
// Integrations card lists it no more, anna's links in the users list
// leave it out and no user is shown acting as anna, and the link route
// knows it no more (404).
func orphanUnseen(t *testing.T, s *Server, i store.Integration) {
	t.Helper()
	var integ struct {
		Users []integrationUserView `json:"users"`
	}
	rec := sendFrom(s, "GET", "/api/v1/integrations/"+i.UUID, lan, "secret", "")
	if json.Unmarshal(rec.Body.Bytes(), &integ); rec.Code != 200 || slices.ContainsFunc(integ.Users, func(v integrationUserView) bool { return v.ID == haAnna }) {
		t.Errorf("the integration's users: %d %s", rec.Code, rec.Body.String())
	}
	var list []userView
	rec = sendFrom(s, "GET", "/api/v1/users", lan, "secret", "")
	json.Unmarshal(rec.Body.Bytes(), &list)
	at := slices.IndexFunc(list, func(v userView) bool { return v.Name == "anna" })
	if rec.Code != 200 || at < 0 || len(list[at].Links) != 0 || slices.ContainsFunc(list, func(v userView) bool { return strings.Contains(v.Display, ", as anna") }) {
		t.Errorf("the users list: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sendFrom(s, "PUT", "/api/v1/integrations/"+i.UUID+"/users/"+haAnna, lan, "secret", `{"user":"anna"}`); rec.Code != 404 {
		t.Errorf("linked again: %d %s", rec.Code, rec.Body.String())
	}
}

// Removing an integration's user that acts as a Massimo user ends the
// link, in a store of this version and in one where its identity outlived
// its removal, which until its next call no list shows and no link takes
// (orphanUnseen): the next call acts as the integration's user made anew,
// unlinked, with what the first call of a user the integration never
// named has, and reads none of the Massimo user's preferences; one not
// linked is made anew as before (AP AccessEnds, D54-A; AP LinkedRemoval,
// D1-A).
func TestRemovalEndsTheLink(t *testing.T) {
	t.Parallel()
	for _, how := range []string{"linked", "older store", "unlinked"} {
		t.Run(how, func(t *testing.T) {
			t.Parallel()
			s := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret"))).s
			st := s.opts.Store
			i, key := registerHA(t, st, []string{"read", "preferences", "setpoint", "lock"})
			name := i.UUID + ":" + haAnna
			if how == "unlinked" {
				actsAs(t, s, key, haAnna, LevelControl)
			} else {
				linkedToAnna(t, s, key, i)
			}
			was, _, _ := st.User(t.Context(), name)
			if how == "older store" {
				removedPastTheStore(t, st, was.ID)
				orphanUnseen(t, s, i)
			} else if rec := sendFrom(s, "DELETE", "/api/v1/users/"+name, lan, "secret", ""); rec.Code != 200 {
				t.Fatalf("the removal: %d %s", rec.Code, rec.Body.String())
			}
			me := actsAs(t, s, key, haAnna, LevelControl)
			now, _, _ := st.User(t.Context(), name)
			if me.User.Name != name || me.ActsFor.Linked || me.User.Role != store.RoleIntegration || now.ID == was.ID {
				t.Fatalf("the next call: %+v %+v, its user %d, was %d", me.User, me.ActsFor, now.ID, was.ID)
			}
			if first := actsAs(t, s, key, haBruno, LevelControl); !slices.Equal(me.Capabilities, first.Capabilities) {
				t.Errorf("the next call: %v, a first call: %v", me.Capabilities, first.Capabilities)
			}
			if prefs := preferencesOf(t, s, key); strings.Contains(prefs, "anna's") {
				t.Errorf("the next call reads anna's preferences: %s", prefs)
			}
		})
	}
}

// The Massimo user an integration's user is linked to removed, the next
// call acts as the integration's user again, unlinked, within its role
// as the owner left it, the role integration and the maximum: a role
// beyond the maximum, which the users routes mark, acts within them, and
// a disabled user stays refused (AP LinkedRemoval, D2-A).
func TestLinkedUserRemovedActsAsItsOwn(t *testing.T) {
	t.Parallel()
	maximum := []string{"read", "preferences", "setpoint", "lock"}
	for _, c := range []struct {
		role     string
		disabled bool
		want     []string
	}{
		{store.RoleIntegration, false, []string{"lock", "preferences", "read", "setpoint"}},
		{store.RoleViewer, false, []string{"preferences", "read"}},
		{store.RoleOwner, false, []string{"lock", "preferences", "read", "setpoint"}},
		{store.RoleIntegration, true, nil},
	} {
		t.Run(c.role+map[bool]string{true: " disabled"}[c.disabled], func(t *testing.T) {
			t.Parallel()
			s := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret"))).s
			st := s.opts.Store
			i, key := registerHA(t, st, maximum)
			name := i.UUID + ":" + haAnna
			linkedToAnna(t, s, key, i)
			change, _ := json.Marshal(map[string]any{"role": c.role, "disabled": c.disabled})
			if rec := sendFrom(s, "PUT", "/api/v1/users/"+name, lan, "secret", string(change)); rec.Code != 200 {
				t.Fatalf("the owner's change: %d %s", rec.Code, rec.Body.String())
			}
			was, _, _ := st.User(t.Context(), name)
			if rec := sendFrom(s, "DELETE", "/api/v1/users/anna", lan, "secret", ""); rec.Code != 200 {
				t.Fatalf("anna removed: %d %s", rec.Code, rec.Body.String())
			}
			if c.disabled {
				rec := sendFromWith(s, "GET", "/api/v1/me", lan, key, "", as(haAnna, LevelControl))
				if rec.Code != 403 || !strings.Contains(rec.Body.String(), `"code": "user_disabled"`) {
					t.Fatalf("the disabled user's next call: %d %s", rec.Code, rec.Body.String())
				}
				return
			}
			me := actsAs(t, s, key, haAnna, LevelControl)
			now, _, _ := st.User(t.Context(), name)
			got := slices.Sorted(slices.Values(me.Capabilities))
			if me.User.Name != name || me.ActsFor.Linked || me.User.Role != c.role || now.ID != was.ID || !slices.Equal(got, c.want) {
				t.Fatalf("the next call: %+v %+v %v, its user %d, was %d", me.User, me.ActsFor, got, now.ID, was.ID)
			}
			if prefs := preferencesOf(t, s, key); strings.Contains(prefs, "anna's") {
				t.Errorf("the next call reads anna's preferences: %s", prefs)
			}
			var list []userView
			rec := sendFrom(s, "GET", "/api/v1/users", lan, "secret", "")
			json.Unmarshal(rec.Body.Bytes(), &list)
			at := slices.IndexFunc(list, func(v userView) bool { return v.Name == name })
			if rec.Code != 200 || at < 0 || list[at].May == nil || list[at].May.Beyond != (c.role == store.RoleOwner) {
				t.Errorf("the users list: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// An event stream opened through an integration for its user linked to a
// Massimo user ends once the link ends, at a keep-alive: at the
// integration's user's removal, at an unlink and at its disabling; the
// stream of another of the integration's users goes on (AP LinkedRemoval,
// Block 1).
func TestStreamEndsWithTheLink(t *testing.T) {
	t.Parallel()
	for _, how := range []string{"removed", "unlinked", "disabled"} {
		t.Run(how, func(t *testing.T) {
			t.Parallel()
			r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
			ts, s := r.ts, r.s
			s.streamKeepAlive.Store(int64(30 * time.Millisecond))
			i, key := registerHA(t, s.opts.Store, []string{"read", "preferences", "setpoint", "lock"})
			linkedToAnna(t, s, key, i)
			ends := openStreamWith(t, ts, key, as(haAnna, LevelControl))
			stays := openStreamWith(t, ts, key, as(haBruno, LevelControl))
			ends.Await(t, 0, "a keep-alive for the linked user", isKeepAlive)
			stays.Await(t, 0, "a keep-alive for the other user", isKeepAlive)
			name := i.UUID + ":" + haAnna
			method, path, body := "DELETE", "/api/v1/users/"+name, ""
			switch how {
			case "unlinked":
				method, path, body = "PUT", "/api/v1/integrations/"+i.UUID+"/users/"+haAnna, `{"user":null}`
			case "disabled":
				method, body = "PUT", `{"disabled":true}`
			}
			if rec := sendFrom(s, method, path, lan, "secret", body); rec.Code != 200 {
				t.Fatalf("%s: %d %s", how, rec.Code, rec.Body.String())
			}
			testkit.Eventually(t, "the end of the linked user's stream", ends.Ended)
			if ends.Err() != nil {
				t.Fatalf("the stream was cut, not ended: %v", ends.Err())
			}
			stays.AwaitN(t, stays.Mark(), "three keep-alives for the other user after the end", 3, isKeepAlive)
		})
	}
}

// An event stream opened through an integration for one of its users
// ends at a keep-alive once the owner accepts another installation id,
// as the old installation's calls are refused: one opened with the key
// and the header, and one opened with a ticket the old installation
// asked for. The keep-alive records no installation pending. A new
// installation's call, which records its id, ends nothing, nor does the
// old installation's call that records its own id again once the new
// one is accepted; a ticket the old installation asked for before the
// acceptance opens no stream then. The new installation's stream, the
// integration's own without the header, the owner's and a plain key's
// go on; the old installation accepted again, the new one's stream ends
// in turn (AP LinkedRemoval, D4-A).
func TestStreamEndsWithItsInstallation(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, s := r.ts, r.s
	s.streamKeepAlive.Store(int64(30 * time.Millisecond))
	st := s.opts.Store
	i, key := registerHA(t, st, []string{"read", "preferences", "setpoint", "lock"})
	const newer = "fedcba9876543210fedcba9876543210"
	call := func(instance string) (int, string) {
		rec := sendFromWith(s, "GET", "/api/v1/me", lan, key, "", asFrom(haAnna, LevelControl, instance))
		return rec.Code, rec.Body.String()
	}
	pending := func(instance string) {
		t.Helper()
		if code, text := call(instance); code != 403 || !strings.Contains(text, `"code": "instance_pending"`) {
			t.Fatalf("a call from %s: %d %s", instance, code, text)
		}
		if got, _, _ := st.IntegrationByUUID(t.Context(), i.UUID); got.PendingInstance != instance {
			t.Fatalf("%s not recorded: %+v", instance, got)
		}
	}
	accept := func(instance string) {
		t.Helper()
		rec := sendFrom(s, "POST", "/api/v1/integrations/"+i.UUID+"/accept-instance", lan, "secret", "")
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"instance_id": "`+instance+`"`) {
			t.Fatalf("accept %s: %d %s", instance, rec.Code, rec.Body.String())
		}
	}
	old := openStreamWith(t, ts, key, as(haAnna, LevelControl))
	ticket := openTicketStream(t, ts, ticketForWith(t, ts, key, "KEQ0000100", as(haBruno, LevelView)))
	own := openStreamWith(t, ts, key, nil)
	stay := []*testkit.Watch[testkit.Event]{own, openStream(t, ts, "secret"), openStream(t, ts, userToken(t, st, "viewer1", store.RoleViewer))}
	for _, w := range append([]*testkit.Watch[testkit.Event]{old, ticket}, stay...) {
		w.Await(t, 0, "a keep-alive", isKeepAlive)
	}

	pending(newer)
	old.AwaitN(t, old.Mark(), "two keep-alives of the old installation's stream while it is the accepted one", 2, isKeepAlive)
	late := ticketForWith(t, ts, key, "KEQ0000100", as(haAnna, LevelView))
	accept(newer)
	testkit.Eventually(t, "the end of the old installation's stream", old.Ended)
	testkit.Eventually(t, "the end of the stream of the old installation's ticket", ticket.Ended)
	if old.Err() != nil || ticket.Err() != nil {
		t.Fatalf("the streams were cut, not ended: %v, %v", old.Err(), ticket.Err())
	}
	if got, _, _ := st.IntegrationByUUID(t.Context(), i.UUID); got.InstanceID != newer || got.PendingInstance != "" {
		t.Fatalf("the keep-alives after the acceptance wrote: %+v", got)
	}

	news := openStreamWith(t, ts, key, asFrom(haAnna, LevelControl, newer))
	pending(haInstance)
	if code, text := call(newer); code != 200 {
		t.Fatalf("a call from the new installation: %d %s", code, text)
	}
	if code, text := callAPI(t, ts, "GET", "/api/v1/cubes/KEQ0000100/events?ticket="+late, "", ""); code != 401 {
		t.Fatalf("a ticket the old installation asked for before the acceptance: %d %s", code, text)
	}
	for _, w := range append(stay, news) {
		w.AwaitN(t, w.Mark(), "three keep-alives of a stream that stays, after the old installation's end", 3, isKeepAlive)
	}

	accept(haInstance)
	testkit.Eventually(t, "the end of the new installation's stream, the old one accepted again", news.Ended)
	own.AwaitN(t, own.Mark(), "three keep-alives of the integration's own stream after that end", 3, isKeepAlive)
}

// A request attached to the integration gives it the request's
// installation id, as an acceptance does: the event stream the
// installation accepted before opened for one of its users ends at a
// keep-alive, as its calls are refused, and the integration's own stream
// without the header goes on (AP LinkedRemoval, D4-A).
func TestStreamEndsAtAnAttach(t *testing.T) {
	t.Parallel()
	r := startServer(t, withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, s := r.ts, r.s
	s.streamKeepAlive.Store(int64(30 * time.Millisecond))
	st := s.opts.Store
	i, key := registerHA(t, st, []string{"read", "preferences", "setpoint", "lock"})
	old := openStreamWith(t, ts, key, as(haAnna, LevelControl))
	own := openStreamWith(t, ts, key, nil)
	old.Await(t, 0, "a keep-alive of the installation's stream", isKeepAlive)
	own.Await(t, 0, "a keep-alive of the integration's own stream", isKeepAlive)
	req, err := st.AddIntegrationRequest(t.Context(), store.IntegrationRequest{Kind: store.KindHomeAssistant, Name: "Casa Leone", InstanceID: "abcdef0123456789abcdef0123456789"}, "GHIJKL")
	if err != nil {
		t.Fatal(err)
	}
	if rec := sendFrom(s, "POST", "/api/v1/integrations/"+req.UUID+"/approve", lan, "secret", `{"attach":"`+i.UUID+`"}`); rec.Code != 200 {
		t.Fatalf("the attach: %d %s", rec.Code, rec.Body.String())
	}
	testkit.Eventually(t, "the end of the stream of the installation accepted before the attach", old.Ended)
	if old.Err() != nil {
		t.Fatalf("the stream was cut, not ended: %v", old.Err())
	}
	own.AwaitN(t, own.Mark(), "three keep-alives of the integration's own stream after that end", 3, isKeepAlive)
	if rec := sendFromWith(s, "GET", "/api/v1/me", lan, key, "", as(haAnna, LevelControl)); rec.Code != 403 || !strings.Contains(rec.Body.String(), `"code": "instance_pending"`) {
		t.Fatalf("a call of the installation accepted before the attach: %d %s", rec.Code, rec.Body.String())
	}
}
