// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package logfile

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

var fixed = time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)

func open(t *testing.T, o Options) (*File, func() []Rotation) {
	t.Helper()
	if o.Now == nil {
		o.Now = func() time.Time { return fixed }
	}
	f, narrowErr, err := Open(o)
	if err != nil || narrowErr != nil {
		t.Fatalf("Open: %v, %v", err, narrowErr)
	}
	t.Cleanup(func() { f.Close() })
	var mu sync.Mutex
	var got []Rotation
	f.Notify(func(r Rotation) {
		mu.Lock()
		got = append(got, r)
		mu.Unlock()
	})
	return f, func() []Rotation {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(p, ".gz") {
		zr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if b, err = io.ReadAll(zr); err != nil {
			t.Fatal(err)
		}
	}
	return string(b)
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func line(n int) []byte { return []byte(`{"msg":"` + strings.Repeat("x", n-10) + `"}` + "\n") }

// A line that would take the file past Size rotates it first: the old
// file ends with a line in its format, and the rotations before it move
// up by one, all but the newest gzipped.
func TestRotateAtSize(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "serve.log")
	f, got := open(t, Options{Path: p, Format: JSON, Size: 100, Keep: 8})
	for range 5 {
		if _, err := f.Write(line(60)); err != nil {
			t.Fatal(err)
		}
	}
	// The size's Rotations are handed on in goroutines of their own.
	for end := time.Now().Add(5 * time.Second); len(got()) < 4 && time.Now().Before(end); {
		time.Sleep(10 * time.Millisecond)
	}
	if w := []string{"serve.log", "serve.log.1", "serve.log.2.gz", "serve.log.3.gz", "serve.log.4.gz"}; !slices.Equal(names(t, dir), w) {
		t.Fatalf("files %v, want %v", names(t, dir), w)
	}
	// serve.log.1: the rotation before it, the line, the rotation after it.
	old := strings.Split(strings.TrimSpace(read(t, p+".1")), "\n")
	if len(old) != 3 {
		t.Fatalf("serve.log.1 holds %d lines:\n%s", len(old), read(t, p+".1"))
	}
	var first, last map[string]any
	if err := json.Unmarshal([]byte(old[2]), &last); err != nil || last["msg"] != "log rotated" || last["reason"] != ReasonSize || last["next"] != "serve.log" {
		t.Errorf("the old file's last line: %v %v", last, err)
	}
	if err := json.Unmarshal([]byte(old[0]), &first); err != nil || first["msg"] != "log rotated" || first["previous"] != "serve.log.1" || first["kept"] != float64(3) {
		t.Errorf("the old file's first line: %v %v", first, err)
	}
	if s := read(t, p); !strings.HasPrefix(s, `{"time":"2026-11-11T10:00:00Z","level":"INFO","msg":"log rotated","source":"system","reason":"size","previous":"serve.log.1","kept":4,"removed":[]}`) {
		t.Errorf("serve.log:\n%s", s)
	}
	if s := read(t, p+".4.gz"); !strings.Contains(s, "log rotated") {
		t.Errorf("serve.log.4.gz: %q", s)
	}
	if r := got(); len(r) != 4 || r[0].Reason != ReasonSize || r[0].Previous != "serve.log.1" || r[0].Err != nil {
		t.Errorf("rotations: %+v", r)
	}
}

// The oldest is deleted only while more than Keep are kept and the
// others still cover KeepMin.
func TestRetention(t *testing.T) {
	for _, c := range []struct {
		name    string
		keepMin int64
		want    int
	}{
		{"by count", 0, 2},
		{"small files stay", 1 << 20, 5},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "serve.log")
			f, got := open(t, Options{Path: p, Format: JSON, Keep: 2, KeepMin: c.keepMin})
			for range 5 {
				f.Write(line(40))
				f.Rotate("reload")
			}
			if n := len(names(t, dir)) - 1; n != c.want {
				t.Errorf("%d rotated files kept, want %d: %v", n, c.want, names(t, dir))
			}
			r := got()
			last := r[len(r)-1]
			if last.Kept != c.want {
				t.Errorf("Kept %d, want %d", last.Kept, c.want)
			}
			if c.keepMin == 0 && !slices.Equal(last.Removed, []string{"serve.log.3.gz"}) {
				t.Errorf("Removed %v", last.Removed)
			}
		})
	}
}

// A file whose last line is plain is rotated at the open of a JSON
// writer, its last line plain too; the Rotation waits for Notify. An
// empty file and one in the writer's format are not rotated.
func TestRotateAtFormatChange(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "serve.log")
	os.WriteFile(p, []byte("time=2026-11-10T10:00:00Z level=INFO msg=\"api listening\"\n"), 0o640)
	f, narrowErr, err := Open(Options{Path: p, Format: JSON, Keep: 8, Now: func() time.Time { return fixed }})
	if err != nil || narrowErr != nil {
		t.Fatal(err, narrowErr)
	}
	f.Write(line(30))
	f.Close()
	var got []Rotation
	f.Notify(func(r Rotation) { got = append(got, r) })
	if len(got) != 1 || got[0].Reason != ReasonFormat || got[0].Err != nil {
		t.Fatalf("rotations: %+v", got)
	}
	if s := read(t, p+".1"); !strings.HasSuffix(s, "level=INFO msg=\"log rotated\" source=system reason=format next=serve.log\n") {
		t.Errorf("serve.log.1:\n%s", s)
	}
	if s := read(t, p); !strings.HasPrefix(s, "{") || strings.Contains(s, "time=") {
		t.Errorf("serve.log:\n%s", s)
	}
	// Now JSON: a JSON writer leaves it, a plain one rotates it.
	for format, want := range map[Format]int{JSON: 0, Plain: 1} {
		var n []Rotation
		f, _, err := Open(Options{Path: p, Format: format, Keep: 8})
		if err != nil {
			t.Fatal(err)
		}
		f.Notify(func(r Rotation) { n = append(n, r) })
		f.Close()
		if len(n) != want {
			t.Errorf("%s writer on a JSON file: %d rotations, want %d", format, len(n), want)
		}
	}
	empty := filepath.Join(dir, "empty.log")
	f, _, _ = Open(Options{Path: empty, Format: Plain})
	if f.Rotate("reload") {
		t.Error("an empty file was rotated")
	}
	f.Close()
}

// The files logrotate made keep their names and move up with the others.
func TestAdoptsLogrotateFiles(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "serve.log")
	os.WriteFile(p+".1", []byte("one\n"), 0o640)
	os.WriteFile(p+".2.gz", gz(t, "two\n"), 0o640)
	os.WriteFile(p+".x", []byte("not ours\n"), 0o640)
	f, _ := open(t, Options{Path: p, Format: JSON, Keep: 8})
	f.Write(line(30))
	f.Rotate("reload")
	if w := []string{"serve.log", "serve.log.1", "serve.log.2.gz", "serve.log.3.gz", "serve.log.x"}; !slices.Equal(names(t, dir), w) {
		t.Fatalf("files %v, want %v", names(t, dir), w)
	}
	if read(t, p+".2.gz") != "one\n" || read(t, p+".3.gz") != "two\n" {
		t.Error("the old files did not move up")
	}
}

func gz(t *testing.T, s string) []byte {
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	zw.Write([]byte(s))
	zw.Close()
	return b.Bytes()
}

// A rename that fails leaves the file where it is, appended to, and the
// Rotation says why.
func TestRefusedRename(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory modes")
	}
	if os.Geteuid() == 0 {
		t.Skip("root renames anyway")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "serve.log")
	f, got := open(t, Options{Path: p, Format: JSON, Keep: 8})
	f.Write(line(30))
	os.Chmod(dir, 0o500)
	defer os.Chmod(dir, 0o700)
	f.Rotate("reload")
	if r := got(); len(r) != 1 || r[0].Err == nil {
		t.Fatalf("rotations: %+v", r)
	}
	if _, err := f.Write(line(30)); err != nil {
		t.Fatalf("not appended to after the refusal: %v", err)
	}
	// The line, the old file's last, the failure's, the line after it.
	if s := read(t, p); strings.Count(s, "\n") != 4 || !strings.Contains(s, `"level":"WARN","msg":"log rotated"`) {
		t.Errorf("serve.log:\n%s", s)
	}
}

// The log file is made 0640, and one others may read more widely is
// narrowed to it; a narrower one stays as it is.
func TestMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows keeps the file's ACLs")
	}
	dir := t.TempDir()
	for _, c := range []struct {
		name   string
		before os.FileMode // 0: no file yet
		want   os.FileMode
	}{
		{"new.log", 0, 0o640},
		{"wide.log", 0o644, 0o640},
		{"open.log", 0o666, 0o640},
		{"narrow.log", 0o600, 0o600},
	} {
		p := filepath.Join(dir, c.name)
		if c.before != 0 {
			os.WriteFile(p, nil, c.before)
			os.Chmod(p, c.before) // past the umask
		}
		f, narrowErr, err := Open(Options{Path: p, Format: JSON})
		if err != nil || narrowErr != nil {
			t.Fatalf("%s: %v, %v", c.name, err, narrowErr)
		}
		f.Close()
		fi, _ := os.Stat(p)
		// A new file's mode is 0640 less the umask.
		if got := fi.Mode().Perm(); got&^c.want != 0 || got&0o600 != 0o600 || (c.before != 0 && got != c.want) {
			t.Errorf("%s: mode %#o, want %#o", c.name, got, c.want)
		}
	}
}

// Many writers and a rotation at the same time lose no line.
func TestConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "serve.log")
	f, _ := open(t, Options{Path: p, Format: JSON, Size: 2000, Keep: 100})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 50 {
				f.Write(line(50))
			}
		})
	}
	wg.Go(func() { f.Rotate("reload") })
	wg.Wait()
	lines := 0
	for _, n := range names(t, dir) {
		for _, l := range strings.Split(read(t, filepath.Join(dir, n)), "\n") {
			if strings.HasPrefix(l, `{"msg":"x`) {
				lines++
			}
		}
	}
	if lines != 400 {
		t.Errorf("%d lines, want 400", lines)
	}
}
