// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"runtime"
	"testing"

	"github.com/bytemare/opaque"
)

// testClient is the client's side of OPAQUE in Go, with the key
// stretching of the browser's library (opaque-ke): a salt of 16 zero
// bytes and an output as long as the OPRF's.
type testClient struct {
	t testing.TB
	c *opaque.Client
}

func newTestClient(t testing.TB) testClient {
	t.Helper()
	c, err := opaqueConf.Client()
	if err != nil {
		t.Fatal(err)
	}
	return testClient{t: t, c: c}
}

func testKSF() *opaque.ClientOptions { return BrowserKSF(argonMemory, argonTime, argonThreads) }

// register runs a registration against the store and returns the record.
func (c testClient) register(s *Store, name, password string) []byte {
	c.t.Helper()
	req, err := c.c.RegistrationInit([]byte(password))
	if err != nil {
		c.t.Fatal(err)
	}
	resp, err := s.OpaqueRegistration(name, req.Serialize())
	if err != nil {
		c.t.Fatal(err)
	}
	r, err := c.c.Deserialize.RegistrationResponse(resp)
	if err != nil {
		c.t.Fatal(err)
	}
	rec, _, err := c.c.RegistrationFinalize(r, nil, nil, testKSF())
	if err != nil {
		c.t.Fatal(err)
	}
	return rec.Serialize()
}

// login runs a sign-in against the store: the KE3 when the client could
// open the answer, nil when the password or the record was wrong.
func (c testClient) login(s *Store, name, password string) (OpaqueStart, []byte) {
	c.t.Helper()
	ke1, err := c.c.GenerateKE1([]byte(password))
	if err != nil {
		c.t.Fatal(err)
	}
	start, err := s.StartOpaque(context.Background(), name, ke1.Serialize())
	if err != nil {
		c.t.Fatal(err)
	}
	ke2, err := c.c.Deserialize.KE2(start.KE2)
	if err != nil {
		c.t.Fatal(err)
	}
	ke3, _, _, err := c.c.GenerateKE3(ke2, nil, nil, testKSF())
	if err != nil {
		return start, nil
	}
	return start, ke3.Serialize()
}

// The key file is made once, 0600, and read back: a record made under it
// still signs in after the store is opened again.
func TestOpaqueKeyFile(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	ctx := context.Background()
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(OpaqueKeyPath(dir))
		if err != nil || fi.Mode().Perm() != 0o600 {
			t.Fatalf("key file mode %v %v", fi, err)
		}
	}
	key, _ := os.ReadFile(OpaqueKeyPath(dir))
	rec := newTestClient(t).register(s, "anna", "secret")
	if _, err := s.AddOpaqueUser(ctx, User{Name: "anna"}, rec, OpaqueKSF, 0); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = open(t, dir)
	if again, _ := os.ReadFile(OpaqueKeyPath(dir)); !bytes.Equal(key, again) {
		t.Error("the key file was made again")
	}
	if start, ke3 := newTestClient(t).login(s, "anna", "secret"); ke3 == nil || s.FinishOpaque(start, ke3) != nil {
		t.Error("the record does not sign in after the store is opened again")
	}
	s.Close()
	if err := os.WriteFile(OpaqueKeyPath(dir), []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(dir, Options{Logger: quiet()}); err == nil {
		s.Close()
		t.Error("a broken key file was accepted")
	}
}

// A user of version 2 signs in; a wrong password cannot even finish, and
// a KE3 of another sign-in or of nothing is a wrong password.
func TestOpaqueSignIn(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	rec := newTestClient(t).register(s, "Anna", "secret")
	u, err := s.AddOpaqueUser(ctx, User{Name: "anna", Role: RoleOperator}, rec, OpaqueKSF, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !u.HasPassword || u.AuthVersion != 2 {
		t.Fatalf("has_password %v, auth_version %d", u.HasPassword, u.AuthVersion)
	}
	var hash string
	s.db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, u.ID).Scan(&hash)
	if hash != "" {
		t.Errorf("password_hash %q beside a record", hash)
	}
	start, ke3 := newTestClient(t).login(s, "ANNA", "secret")
	if ke3 == nil {
		t.Fatal("the right password did not open the answer")
	}
	if start.UserID() != u.ID || start.KSF != OpaqueKSF || len(start.KE2) != 320 {
		t.Errorf("start: user %d, ksf %q, %d bytes", start.UserID(), start.KSF, len(start.KE2))
	}
	if err := s.FinishOpaque(start, ke3); err != nil {
		t.Errorf("finish: %v", err)
	}
	if _, ke3 := newTestClient(t).login(s, "anna", "wrong"); ke3 != nil {
		t.Error("a wrong password opened the answer")
	}
	other, _ := newTestClient(t).login(s, "anna", "secret")
	if err := s.FinishOpaque(other, ke3); !errors.Is(err, ErrBadPassword) {
		t.Errorf("a KE3 of another sign-in: %v", err)
	}
	if err := s.FinishOpaque(other, make([]byte, 64)); !errors.Is(err, ErrBadPassword) {
		t.Errorf("a KE3 of zeros: %v", err)
	}
	if err := s.CheckPassword(ctx, "anna", "secret"); !errors.Is(err, ErrBadPassword) {
		t.Errorf("a password sign-in of a user of version 2: %v", err)
	}
}

// A name without a record, unknown or of version 1, is answered from a
// fake record of the same shape; the name's evaluation is the same every
// time and stays the same once the name has a record, so neither tells
// which names exist.
func TestOpaqueUnknownName(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	if _, err := s.AddUser(ctx, User{Name: "bert"}, "secret", 0); err != nil {
		t.Fatal(err)
	}
	c := newTestClient(t)
	ke1, err := c.c.GenerateKE1([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	evaluation := func(name string) []byte {
		t.Helper()
		start, err := s.StartOpaque(ctx, name, ke1.Serialize())
		if err != nil {
			t.Fatal(err)
		}
		if start.UserID() != 0 || start.KSF != OpaqueKSF || len(start.KE2) != 320 {
			t.Errorf("%s: user %d, ksf %q, %d bytes", name, start.UserID(), start.KSF, len(start.KE2))
		}
		return start.KE2[:32]
	}
	nobody := evaluation("nobody")
	if !bytes.Equal(nobody, evaluation("nobody")) {
		t.Error("an unknown name's evaluation changed between two tries")
	}
	if bytes.Equal(nobody, evaluation("bert")) {
		t.Error("two names share an evaluation")
	}
	for _, name := range []string{"nobody", "bert"} {
		if start, ke3 := newTestClient(t).login(s, name, "secret"); ke3 != nil || s.FinishOpaque(start, make([]byte, 64)) == nil {
			t.Errorf("%s signed in without a record", name)
		}
	}
	rec := newTestClient(t).register(s, "nobody", "secret")
	if _, err := s.AddOpaqueUser(ctx, User{Name: "nobody"}, rec, OpaqueKSF, 0); err != nil {
		t.Fatal(err)
	}
	start, err := s.StartOpaque(ctx, "nobody", ke1.Serialize())
	if err != nil || !bytes.Equal(start.KE2[:32], nobody) {
		t.Errorf("the evaluation changed when the name got a record: %v", err)
	}
}

// A password of version 1 replaces a record, and a record a hash.
func TestOpaqueAndPassword(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	if _, err := s.AddUser(ctx, User{Name: "anna"}, "old", 0); err != nil {
		t.Fatal(err)
	}
	rec := newTestClient(t).register(s, "anna", "new")
	if err := s.SetOpaqueRecord(ctx, "anna", rec, OpaqueKSF, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckPassword(ctx, "anna", "old"); !errors.Is(err, ErrBadPassword) {
		t.Errorf("the hash of version 1 still signs in: %v", err)
	}
	if start, ke3 := newTestClient(t).login(s, "anna", "new"); ke3 == nil || s.FinishOpaque(start, ke3) != nil {
		t.Error("the record does not sign in")
	}
	if err := s.SetPassword(ctx, "anna", "again", 0); err != nil {
		t.Fatal(err)
	}
	u, _, _ := s.User(ctx, "anna")
	if u.AuthVersion != 1 || !u.HasPassword {
		t.Errorf("auth_version %d, has_password %v", u.AuthVersion, u.HasPassword)
	}
	if start, _ := newTestClient(t).login(s, "anna", "new"); start.UserID() != 0 {
		t.Error("the record outlived a password of version 1")
	}
	if err := s.SetOpaqueRecord(ctx, "nobody", rec, OpaqueKSF, 0); !errors.Is(err, ErrNoUser) {
		t.Errorf("a record for nobody: %v", err)
	}
	if _, err := s.AddOpaqueUser(ctx, User{Name: "anna"}, rec, OpaqueKSF, 0); !errors.Is(err, ErrUserExists) {
		t.Errorf("a taken name: %v", err)
	}
}

// A record that does not decode, and key stretching out of bounds, are
// refused before anything is written.
func TestOpaqueRecordChecked(t *testing.T) {
	s := open(t, t.TempDir())
	ctx := context.Background()
	rec := newTestClient(t).register(s, "anna", "secret")
	for _, c := range []struct {
		record []byte
		ksf    string
	}{
		{rec[:191], OpaqueKSF},
		{make([]byte, 192), OpaqueKSF},
		{rec, ""},
		{rec, "m=19456,t=1,p=1"},
		{rec, "m=524288,t=2,p=1"},
		{rec, "m=19456,t=2,p=0"},
		{rec, "t=2,m=19456,p=1"},
	} {
		if _, err := s.AddOpaqueUser(ctx, User{Name: "anna"}, c.record, c.ksf, 0); !errors.Is(err, ErrBadMessage) {
			t.Errorf("%d bytes, %q: %v", len(c.record), c.ksf, err)
		}
	}
	for _, ksf := range []string{OpaqueKSF, "m=12288,t=4,p=1", "m=65536,t=3,p=4"} {
		if _, _, _, err := ParseKSF(ksf); err != nil {
			t.Errorf("%q: %v", ksf, err)
		}
	}
	if _, err := s.OpaqueRegistration("anna", make([]byte, 31)); !errors.Is(err, ErrBadMessage) {
		t.Errorf("a short registration request: %v", err)
	}
	if _, err := s.StartOpaque(ctx, "anna", make([]byte, 96)); !errors.Is(err, ErrBadMessage) {
		t.Errorf("a KE1 of zeros: %v", err)
	}
	if n, _ := s.CountUsers(ctx); n != 0 {
		t.Errorf("%d users written", n)
	}
}

// BenchmarkOpaqueLogin is what a sign-in of version 2 costs the server:
// the start (KE1 to KE2) for a user with a record and for a name
// without, and the finish. The key stretching is the client's and not
// in it. On one core and a 32-bit build:
//
//	GOMAXPROCS=1 go test -run '^$' -bench OpaqueLogin -benchmem ./internal/store
//	GOARCH=386 GOMAXPROCS=1 go test -run '^$' -bench OpaqueLogin -benchmem ./internal/store
func BenchmarkOpaqueLogin(b *testing.B) {
	s, err := Open(b.TempDir(), Options{Logger: quiet()})
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	rec := newTestClient(b).register(s, "anna", "secret")
	if _, err := s.AddOpaqueUser(ctx, User{Name: "anna"}, rec, OpaqueKSF, 0); err != nil {
		b.Fatal(err)
	}
	ke1, err := newTestClient(b).c.GenerateKE1([]byte("secret"))
	if err != nil {
		b.Fatal(err)
	}
	for _, name := range []string{"anna", "nobody"} {
		b.Run("start-"+map[string]string{"anna": "record", "nobody": "fake"}[name], func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := s.StartOpaque(ctx, name, ke1.Serialize()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	b.Run("finish", func(b *testing.B) {
		start, ke3 := newTestClient(b).login(s, "anna", "secret")
		b.ReportAllocs()
		for b.Loop() {
			if err := s.FinishOpaque(start, ke3); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// The command line registers both halves here, with the browser's key
// stretching: the user signs in by OPAQUE, a taken name, an unknown role
// and a missing user are refused before the key stretching, and a store
// opened read-only reads the key to check a sign-in.
func TestOpaqueInProcess(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	ctx := context.Background()
	u, err := s.AddUserOpaque(ctx, User{Name: "Anna", Role: RoleOperator}, "secret", 0)
	if err != nil || u.AuthVersion != 2 || !u.HasPassword {
		t.Fatalf("add: %+v %v", u, err)
	}
	if start, ke3 := newTestClient(t).login(s, "anna", "secret"); ke3 == nil || s.FinishOpaque(start, ke3) != nil {
		t.Fatal("the record made here does not sign in")
	}
	if _, err := s.AddUserOpaque(ctx, User{Name: "anna"}, "x", 0); !errors.Is(err, ErrUserExists) {
		t.Errorf("a taken name: %v", err)
	}
	if _, err := s.AddUserOpaque(ctx, User{Name: "bob", Role: "nope"}, "x", 0); !errors.Is(err, ErrNoRole) {
		t.Errorf("an unknown role: %v", err)
	}
	if err := s.SetPasswordOpaque(ctx, "nobody", "x", 0); !errors.Is(err, ErrNoUser) {
		t.Errorf("a missing user: %v", err)
	}
	if err := s.SetPasswordOpaque(ctx, "anna", "", 0); !errors.Is(err, ErrBadMessage) {
		t.Errorf("an empty password: %v", err)
	}
	if err := s.SetPasswordOpaque(ctx, "anna", "other", 0); err != nil {
		t.Fatal(err)
	}
	s.Close()
	ro, err := OpenReadOnly(dir, Options{Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if start, ke3 := newTestClient(t).login(ro, "anna", "other"); ke3 == nil || ro.FinishOpaque(start, ke3) != nil {
		t.Error("the new password does not sign in against the store opened read-only")
	}
}
