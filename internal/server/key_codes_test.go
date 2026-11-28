// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"go.metaworx.ch/massimo/internal/store"
)

// The refusals AP KeyLife added to the API keys' routes carry their
// codes (errcodes_keys.go): each answers its status, its code, its
// params by name, and its English error, the code's format filled. The
// key routes' refusals from before KeyLife keep none here (AP
// ErrorCodes, Block 4).
func TestKeyLifeCodes(t *testing.T) {
	t.Parallel()
	r := startServer(t, quiet(), withOptions(storeOptions(t, t.TempDir(), "secret")))
	ts, st := r.ts, r.s.opts.Store
	ctx := t.Context()
	anna, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "placeholder-pass-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	annaKey, annaSecret, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "Loxone"})
	annaOther, _, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "phone"})
	ended, _, _ := st.AddKey(ctx, anna.ID, store.KeyOptions{Label: "old"})
	endToken(t, st, ended.ID)
	_, sessionSecret, _ := st.AddToken(ctx, anna.ID, store.TokenSession, "Firefox", time.Now().Add(SessionLifetime))
	session, _, _ := st.TokenByHash(ctx, store.HashToken(sessionSecret))
	i, _ := registerHA(t, st, []string{"read"})
	integ, err := st.Tokens(ctx, i.UserID)
	if err != nil || len(integ) != 1 {
		t.Fatalf("the integration's key: %+v %v", integ, err)
	}
	past := time.Now().Add(-time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339)
	tokens := "/api/v1/users/anna/tokens/"
	id := func(n int64) string { return strconv.FormatInt(n, 10) }
	for _, c := range []struct {
		what, method, path, bearer, body string
		status                           int
		code                             string
		params                           map[string]any
		args                             []any
	}{
		{"POST, an end that has passed", "POST", "/api/v1/users/anna/tokens", "secret", `{"label":"x","expires_at":"` + past + `"}`,
			400, "key_end_passed", map[string]any{"expires_at": past}, []any{past}},
		{"PUT, an end that has passed", "PUT", tokens + id(annaKey.ID), "secret", `{"expires_at":"` + past + `"}`,
			400, "key_end_passed", map[string]any{"expires_at": past}, []any{past}},
		{"POST, an end of year one", "POST", "/api/v1/users/anna/tokens", "secret", `{"label":"x","expires_at":"0001-01-01T00:00:00Z"}`,
			400, "end_date_year_one", map[string]any{"expires_at": "0001-01-01T00:00:00Z"}, []any{"0001-01-01T00:00:00Z"}},
		{"PUT a user, an end of year one", "PUT", "/api/v1/users/anna", "secret", `{"expires_at":"0001-01-01T00:00:00Z"}`,
			400, "end_date_year_one", map[string]any{"expires_at": "0001-01-01T00:00:00Z"}, []any{"0001-01-01T00:00:00Z"}},
		{"POST, kept by who manages nobody", "POST", "/api/v1/users/anna/tokens", annaSecret, `{"label":"x","idle_exempt":true}`,
			403, "key_keep_forbidden", map[string]any{"user": "anna"}, []any{"anna"}},
		{"PUT, kept by who manages nobody", "PUT", tokens + id(annaOther.ID), annaSecret, `{"idle_exempt":true}`,
			403, "key_keep_forbidden", map[string]any{"user": "anna"}, []any{"anna"}},
		{"PUT, a session", "PUT", tokens + id(session.ID), "secret", `{"idle_exempt":true}`,
			400, "key_life_session", map[string]any{"token": float64(session.ID)}, []any{session.ID}},
		{"PUT, an integration's key kept", "PUT", "/api/v1/users/" + i.UUID + ":/tokens/" + id(integ[0].ID), "secret", `{"idle_exempt":true}`,
			400, "key_life_integration_kept", nil, nil},
		{"PUT, the key itself", "PUT", tokens + id(annaKey.ID), annaSecret, `{"expires_at":null}`,
			403, "key_life_own_key", nil, nil},
		{"rekey, a session", "POST", tokens + id(session.ID) + "/rekey", "secret", "",
			400, "new_secret_session", map[string]any{"token": float64(session.ID)}, []any{session.ID}},
		{"rekey, an integration's key", "POST", "/api/v1/users/" + i.UUID + ":/tokens/" + id(integ[0].ID) + "/rekey", "secret", "",
			403, "new_secret_integration", nil, nil},
		{"rekey, the key itself", "POST", tokens + id(annaKey.ID) + "/rekey", annaSecret, "",
			403, "new_secret_own_key", nil, nil},
		{"rekey, a key that has ended", "POST", tokens + id(ended.ID) + "/rekey", "secret", "",
			409, "new_secret_ended", map[string]any{"key": float64(ended.ID)}, []any{ended.ID}},
	} {
		code, text := callAPI(t, ts, c.method, c.path, c.bearer, c.body)
		var got struct {
			Error  string         `json:"error"`
			Code   string         `json:"code"`
			Params map[string]any `json:"params"`
		}
		if err := json.Unmarshal([]byte(text), &got); err != nil {
			t.Errorf("%s: %d %s: %v", c.what, code, text, err)
			continue
		}
		var format string
		for _, d := range errorCodes {
			if d.name == c.code {
				format = d.format
			}
		}
		if code != c.status || got.Code != c.code || fmt.Sprint(got.Params) != fmt.Sprint(c.params) || got.Error != fmt.Sprintf(format, c.args...) {
			t.Errorf("%s: %d %s, want %d %s %v %q", c.what, code, text, c.status, c.code, c.params, fmt.Sprintf(format, c.args...))
		}
	}
}
