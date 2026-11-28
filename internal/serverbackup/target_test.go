// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// The local kind is registered with its one field; a kind the code gets
// wrong panics at its registration.
func TestKinds(t *testing.T) {
	t.Parallel()
	k, ok := KindOf(KindLocal)
	if f := k.Fields; !ok || len(f) != 1 || f[0].Key != "path" || f[0].Type != FieldPath || f[0].Default != DefaultLocalDir || f[0].Required {
		t.Fatalf("local %+v %v", k, ok)
	}
	if !slices.ContainsFunc(Kinds(), func(k Kind) bool { return k.Name == KindLocal }) {
		t.Fatal("Kinds lacks local")
	}
	if _, ok := k.Secret(); ok {
		t.Fatal("local has a secret")
	}
	newT := func(map[string]string, []byte) (Target, error) { return nil, nil }
	for name, bad := range map[string]Kind{
		"a name taken":    {Name: KindLocal, New: newT},
		"no New":          {Name: "nonew"},
		"a bad name":      {Name: "Bad-Name", New: newT},
		"two secrets":     {Name: "twosecrets", New: newT, Fields: []Field{{Key: "a", Type: FieldSecret}, {Key: "b", Type: FieldSecret}}},
		"a key twice":     {Name: "twice", New: newT, Fields: []Field{{Key: "a", Type: FieldText}, {Key: "a", Type: FieldText}}},
		"an unknown type": {Name: "untyped", New: newT, Fields: []Field{{Key: "a", Type: "date"}}},
		"a bare choice":   {Name: "bare", New: newT, Fields: []Field{{Key: "a", Type: FieldChoice}}},
		"a bad default":   {Name: "baddefault", New: newT, Fields: []Field{{Key: "a", Type: FieldNumber, Default: "x"}}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s registered", name)
				}
			}()
			Register(bad)
		}()
	}
}

// A kind's Check fills the defaults and refuses what its fields refuse.
func TestKindCheck(t *testing.T) {
	t.Parallel()
	k := Kind{Name: "probe", Fields: []Field{
		{Key: "url", Type: FieldText, Required: true},
		{Key: "port", Type: FieldNumber, Default: "443"},
		{Key: "tls", Type: FieldBool, Default: "true"},
		{Key: "style", Type: FieldChoice, Choices: []string{"path", "host"}, Default: "path"},
		{Key: "password", Type: FieldSecret, Required: true},
	}}
	got, err := k.Check(map[string]string{"url": " https://example.org ", "tls": "0"}, true)
	if err != nil || got["url"] != "https://example.org" || got["port"] != "443" || got["tls"] != "false" || got["style"] != "path" || len(got) != 4 {
		t.Fatalf("check %v %v", got, err)
	}
	for name, c := range map[string]struct {
		cfg    map[string]string
		secret bool
		want   string
	}{
		"no url":         {map[string]string{}, true, "url is required"},
		"no secret":      {map[string]string{"url": "u"}, false, "password is required"},
		"a secret given": {map[string]string{"url": "u", "password": "p"}, true, "given apart"},
		"an unknown key": {map[string]string{"url": "u", "colour": "red"}, true, `no field "colour"`},
		"a bad number":   {map[string]string{"url": "u", "port": "4x"}, true, "port"},
		"a bad switch":   {map[string]string{"url": "u", "tls": "maybe"}, true, "tls"},
		"a bad choice":   {map[string]string{"url": "u", "style": "dns"}, true, "style"},
		"a control":      {map[string]string{"url": "u\x00v"}, true, "control"},
	} {
		if _, err := k.Check(c.cfg, c.secret); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
}

// The names a backup may have on a target, and the one it has.
func TestCheckName(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"massimo-20261127-033000-0193e7a0-0000-7000-8000-000000000000.zip", "a.zip", "x_y-z.1.zip"} {
		if err := CheckName(ok); err != nil {
			t.Errorf("%s refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"", ".zip", ".hidden.zip", "a/b.zip", `a\b.zip`, "a..zip", "a.txt", "a.zip.part", "../a.zip", strings.Repeat("a", 200) + ".zip", "a b.zip"} {
		if err := CheckName(bad); err == nil {
			t.Errorf("%q taken", bad)
		}
	}
	m := Manifest{UUID: "0193e7a0-0000-7000-8000-000000000000", Created: time.Date(2026, 11, 27, 4, 30, 0, 0, time.FixedZone("CET", 3600))}
	if got := FileName(m); got != "massimo-20261127-033000-0193e7a0-0000-7000-8000-000000000000.zip" || CheckName(got) != nil {
		t.Fatalf("file name %s", got)
	}
}

// What a target takes (D11-A), and when it is two periods behind.
func TestTakes(t *testing.T) {
	t.Parallel()
	at := func(s string) time.Time {
		v, err := time.Parse("2006-01-02 15:04", s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, c := range []struct {
		takes Takes
		have  []string
		at    string
		want  bool
	}{
		{TakesEvery, []string{"2026-11-27 03:30"}, "2026-11-27 10:00", true},
		{TakesDaily, []string{"2026-11-27 03:30"}, "2026-11-27 10:00", false},
		{TakesDaily, []string{"2026-11-26 03:30"}, "2026-11-27 03:30", true},
		{TakesWeekly, []string{"2026-11-23 03:30"}, "2026-11-29 03:30", false}, // Monday and Sunday of one week
		{TakesWeekly, []string{"2026-11-29 03:30"}, "2026-11-30 03:30", true},  // Sunday, then Monday
		{TakesMonthly, []string{"2026-11-01 03:30"}, "2026-11-30 03:30", false},
		{TakesMonthly, []string{"2026-11-30 03:30"}, "2026-12-01 03:30", true},
		{TakesMonthly, nil, "2026-11-30 03:30", true},
	} {
		var have []time.Time
		for _, h := range c.have {
			have = append(have, at(h))
		}
		if got := c.takes.Takes(at(c.at), have); got != c.want {
			t.Errorf("%s with %v takes %s: %v", c.takes, c.have, c.at, got)
		}
	}
	now := at("2026-11-27 12:00")
	for _, c := range []struct {
		takes         Takes
		last, created string
		want          bool
		what          string
	}{
		{TakesDaily, "2026-11-26 03:30", "2026-01-01 00:00", false, "a day ago"},
		{TakesDaily, "2026-11-25 03:30", "2026-01-01 00:00", true, "two and a half days"},
		{TakesWeekly, "2026-11-15 03:30", "2026-01-01 00:00", false, "twelve days, weekly"},
		{TakesWeekly, "2026-11-12 03:30", "2026-01-01 00:00", true, "fifteen days, weekly"},
		{TakesEvery, "", "2026-11-27 00:00", false, "made today, nothing yet"},
		{TakesEvery, "", "2026-11-20 00:00", true, "made a week ago, nothing yet"},
	} {
		var last time.Time
		if c.last != "" {
			last = at(c.last)
		}
		if got := c.takes.Behind(last, at(c.created), now); got != c.want {
			t.Errorf("%s: behind %v", c.what, got)
		}
	}
}

// candidates are backups made at the times given, named by them.
func candidates(t *testing.T, times ...string) []Candidate {
	t.Helper()
	var out []Candidate
	for _, s := range times {
		at, err := time.Parse(time.DateTime, s)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, Candidate{UUID: s, Time: at})
	}
	return out
}

// kept is what Keep keeps, sorted.
func kept(p Policy, cs []Candidate, now time.Time) []string {
	var out []string
	for u := range Keep(p, cs, now) {
		out = append(out, u)
	}
	slices.Sort(out)
	return out
}

// restic's example of a missed Wednesday (docs, "Removing snapshots
// according to a policy"): forget --keep-daily 5 keeps the newest of each
// of the last five days that hold a backup, the Friday before included.
func TestKeepResticDaily(t *testing.T) {
	t.Parallel()
	cs := candidates(t,
		"2025-04-21 11:00:00", "2025-04-22 11:00:00", "2025-04-23 11:00:00", "2025-04-24 11:00:00",
		"2025-04-25 11:00:00", "2025-04-25 23:00:00", "2025-04-28 11:00:00", "2025-04-29 11:00:00",
		"2025-05-01 11:00:00", "2025-05-02 11:00:00", "2025-05-02 23:00:00")
	now := time.Date(2025, 5, 3, 0, 0, 0, 0, time.UTC)
	want := []string{"2025-04-25 23:00:00", "2025-04-28 11:00:00", "2025-04-29 11:00:00", "2025-05-01 11:00:00", "2025-05-02 23:00:00"}
	if got := kept(Policy{Daily: 5}, cs, now); !slices.Equal(got, want) {
		t.Fatalf("--keep-daily 5 keeps %v, restic %v", got, want)
	}
}

// restic's daily backups over years with --keep-daily 7 --keep-weekly 5
// --keep-monthly 12 --keep-yearly 75: the seven newest days, four more
// weeks (the dailies hold one: the newest is a Sunday, so that the seven
// are one week, as restic's example counts them), eleven more months
// (the dailies' month is one), and of the years the one that is none of
// these.
func TestKeepResticYears(t *testing.T) {
	t.Parallel()
	var cs []Candidate
	for d := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC); !d.After(time.Date(2026, 11, 29, 12, 0, 0, 0, time.UTC)); d = d.AddDate(0, 0, 1) {
		cs = append(cs, Candidate{UUID: d.Format(time.DateOnly), Time: d})
	}
	want := []string{
		"2024-12-31",
		"2025-12-31", "2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30", "2026-05-31", "2026-06-30", "2026-07-31", "2026-08-31", "2026-09-30", "2026-10-31",
		"2026-11-01", "2026-11-08", "2026-11-15", "2026-11-22",
		"2026-11-23", "2026-11-24", "2026-11-25", "2026-11-26", "2026-11-27", "2026-11-28", "2026-11-29",
	}
	got := kept(Policy{Daily: 7, Weekly: 5, Monthly: 12, Yearly: 75}, cs, time.Date(2026, 11, 29, 13, 0, 0, 0, time.UTC))
	if !slices.Equal(got, want) || len(got) != 7+4+11+1 {
		t.Fatalf("kept %d: %v\nwant 7 + 4 + 11 + 1: %v", len(got), got, want)
	}
}

// Beyond restic: the last N, a pinned backup while pinned, a pulled one,
// the newest whatever the policy, and a policy of zeros keeping all.
func TestKeepRules(t *testing.T) {
	t.Parallel()
	cs := candidates(t, "2026-11-01 03:30:00", "2026-11-02 03:30:00", "2026-11-03 03:30:00", "2026-11-04 03:30:00", "2026-11-05 03:30:00")
	now := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	if got := kept(Policy{Last: 2}, cs, now); strings.Join(got, ",") != "2026-11-04 03:30:00,2026-11-05 03:30:00" {
		t.Errorf("last 2 %v", got)
	}
	if got := kept(Policy{}, cs, now); len(got) != 5 {
		t.Errorf("an empty policy pruned: %v", got)
	}
	cs[0].PinnedUntil = now.Add(time.Hour)
	cs[1].PinnedUntil = now.Add(-time.Hour)
	cs[2].Pulled = true
	if got := kept(Policy{Last: 1}, cs, now); strings.Join(got, ",") != "2026-11-01 03:30:00,2026-11-03 03:30:00,2026-11-05 03:30:00" {
		t.Errorf("pinned, pulled and the newest %v", got)
	}
	// The newest is kept by a policy that counts only years it has.
	if got := kept(Policy{Yearly: 1}, cs[:2], now); !slices.Contains(got, "2026-11-02 03:30:00") {
		t.Errorf("the newest pruned: %v", got)
	}
	if err := (Policy{Daily: -1}).Check(); err == nil {
		t.Error("a negative count accepted")
	}
}

// local opens a local target in a new directory.
func local(t *testing.T) *Local {
	t.Helper()
	l, err := NewLocal(filepath.Join(t.TempDir(), "backups"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// failingReader gives n bytes, then fails.
type failingReader struct{ n int }

func (f *failingReader) Read(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, errors.New("the line went down")
	}
	k := min(len(p), f.n)
	clear(p[:k])
	f.n -= k
	return k, nil
}

// A put goes by a temporary name: broken off, or short, it leaves
// nothing, under the name or beside it; done, the file is whole, 0600,
// and read by ranges.
func TestLocalPut(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := local(t)
	if _, err := NewLocal("backups"); err == nil {
		t.Fatal("a relative directory taken")
	}
	if err := l.Put(ctx, "a.zip", &failingReader{n: 100}, 1000); err == nil {
		t.Fatal("a put broken off succeeded")
	}
	if err := l.Put(ctx, "a.zip", strings.NewReader("short"), 1000); err == nil {
		t.Fatal("a short put succeeded")
	}
	if err := l.Put(ctx, "a.zip", strings.NewReader("too long"), 3); err == nil {
		t.Fatal("a long put succeeded")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := l.Put(cancelled, "a.zip", strings.NewReader("data"), 4); err == nil {
		t.Fatal("a put of a context ended succeeded")
	}
	if entries, _ := os.ReadDir(l.Dir()); len(entries) != 0 {
		t.Fatalf("left behind: %v", entries)
	}
	if err := l.Put(ctx, "../a.zip", strings.NewReader("data"), 4); err == nil {
		t.Fatal("a name outside taken")
	}
	data := bytes.Repeat([]byte("massimo "), 1000)
	if err := l.Put(ctx, "a.zip", bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(l.Dir(), "a.zip"))
	if err != nil || fi.Size() != int64(len(data)) || (fi.Mode().Perm()&0o077 != 0 && os.PathSeparator == '/') {
		t.Fatalf("the file %v %v", fi, err)
	}
	f, size, err := l.Open(ctx, "a.zip")
	if err != nil || size != int64(len(data)) {
		t.Fatalf("open %d %v", size, err)
	}
	part := make([]byte, 8)
	if _, err := f.ReadAt(part, 8*500); err != nil || string(part) != "massimo " {
		t.Fatalf("ranged read %q %v", part, err)
	}
	f.Close()
	// A stale part a crash left goes at the next put; the list holds the
	// backups alone.
	stale := filepath.Join(l.Dir(), ".b.zip.123"+partSuffix)
	os.WriteFile(stale, []byte("x"), 0o600)
	os.WriteFile(filepath.Join(l.Dir(), "notes.txt"), []byte("x"), 0o600)
	old := time.Now().Add(-2 * partStale)
	os.Chtimes(stale, old, old)
	if err := l.Put(ctx, "b.zip", strings.NewReader("bb"), 2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("a stale part stayed")
	}
	list, err := l.List(ctx)
	if err != nil || len(list) != 2 || list[0].Name != "a.zip" || list[1].Name != "b.zip" || list[1].Size != 2 {
		t.Fatalf("list %+v %v", list, err)
	}
	if err := l.Delete(ctx, "a.zip"); err != nil {
		t.Fatal(err)
	}
	if err := l.Delete(ctx, "a.zip"); err != nil {
		t.Fatalf("a name not there: %v", err)
	}
	if err := l.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if empty, _ := NewLocal(filepath.Join(t.TempDir(), "none")); empty != nil {
		if list, err := empty.List(ctx); err != nil || len(list) != 0 {
			t.Fatalf("a directory not made: %v %v", list, err)
		}
	}
}

// A scan finds a backup by its manifest, read by ranges, and skips a
// file that is none.
func TestScan(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testStore(t)
	r, _ := keyOf(t, KindX25519, "laptop")
	path, m := write(t, st, Inputs{Trigger: TriggerScheduled}, r)
	l := local(t)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	fi, _ := f.Stat()
	if err := l.Put(ctx, FileName(m), f, fi.Size()); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := l.Put(ctx, "other.zip", strings.NewReader("no zip"), 6); err != nil {
		t.Fatal(err)
	}
	found, skipped, err := Scan(ctx, l)
	if err != nil || len(found) != 1 || len(skipped) != 1 || skipped[0].Name != "other.zip" || !errors.Is(skipped[0].Err, ErrNotABackup) {
		t.Fatalf("scan %+v %+v %v", found, skipped, err)
	}
	if g := found[0]; g.Manifest.UUID != m.UUID || g.Entry.Name != FileName(m) || g.Signature.Verdict(st.NodePublicKey()) != ThisBox {
		t.Fatalf("found %+v", g)
	}
}
