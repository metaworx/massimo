// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.metaworx.ch/massimo/internal/serverbackup"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// relabelled writes the backup at path again, its manifest's text from
// changed to to, as a zip tool would, and returns the new file.
func relabelled(t *testing.T, path, from, to string) string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == serverbackup.ManifestName {
			b = bytes.Replace(b, []byte(from), []byte(to), 1)
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		w.Write(b)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "changed.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// noUsage is the usage of the store commands' tests, which print none.
func noUsage() {}

// store keygen takes no default kind (AP ServerBackup, D17-C): without
// -kind, or with another, it is refused and makes nothing.
func TestStoreKeygenNeedsAKind(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, args := range [][]string{nil, {"-kind", "x25519"}, {"-kind", ""}, {"-out", filepath.Join(dir, "key.txt")}} {
		var out, errOut bytes.Buffer
		err := storeKeygen(args, noUsage, &out, &errOut)
		if err == nil || !strings.Contains(err.Error(), "-kind pq") || !strings.Contains(err.Error(), "-kind classic") || out.Len() > 0 {
			t.Errorf("%v: %v, %q", args, err, out.String())
		}
	}
	if left, _ := os.ReadDir(dir); len(left) > 0 {
		t.Fatalf("made %v", left)
	}
}

// store keygen writes the secret key to -out, mode 0600, never over a
// file, and prints the public key; without -out the key goes to
// standard output with a warning.
func TestStoreKeygen(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "key.txt")
	var out, errOut bytes.Buffer
	if err := storeKeygen([]string{"-kind", "classic", "-out", path}, noUsage, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(out.String())
	if !strings.HasPrefix(line, "age1") || strings.HasPrefix(line, "age1pq1") || !strings.Contains(string(b), "# public key: "+line+"\nAGE-SECRET-KEY-1") ||
		strings.Contains(out.String()+errOut.String(), "AGE-SECRET-KEY") || !strings.Contains(errOut.String(), "keep it safe") {
		t.Fatalf("printed %q and %q, wrote %q", out.String(), errOut.String(), b)
	}
	if fi, _ := os.Stat(path); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode())
	}
	if _, err := serverbackup.ParseIdentities(b); err != nil {
		t.Fatal(err)
	}
	if err := storeKeygen([]string{"-kind", "pq", "-out", path}, noUsage, &out, &errOut); err == nil {
		t.Fatal("a key written over another")
	}
	if again, _ := os.ReadFile(path); !bytes.Equal(again, b) {
		t.Fatal("the key file changed")
	}

	out.Reset()
	errOut.Reset()
	if err := storeKeygen([]string{"-kind", "pq"}, noUsage, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\nAGE-SECRET-KEY-PQ-1") || !strings.Contains(errOut.String(), "public key: age1pq1") || !strings.Contains(errOut.String(), "on standard output") {
		t.Fatalf("printed %q and %q", out.String(), errOut.String())
	}
	if _, err := serverbackup.ParseIdentities(out.Bytes()); err != nil {
		t.Fatal(err)
	}
}

// store inspect reads a backup without the key and says which box made
// it; store decrypt opens it with the key from store keygen.
func TestStoreInspectAndDecrypt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	st := storekit.Open(t, dir)
	if _, err := st.AddCube(context.Background(), store.Cube{Serial: "KEQ0000000", Configured: true}); err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(t.TempDir(), "key.txt")
	var out, errOut bytes.Buffer
	if err := storeKeygen([]string{"-kind", "pq", "-out", keyFile}, noUsage, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	r, err := serverbackup.ParseRecipient(out.String())
	if err != nil {
		t.Fatal(err)
	}
	r.Label = "laptop"
	backup := filepath.Join(t.TempDir(), "backup.zip")
	m, err := serverbackup.Write(context.Background(), serverbackup.Inputs{Store: st, Trigger: serverbackup.TriggerManual, Label: "by hand", Version: "test"}, []serverbackup.Recipient{r}, backup)
	if err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := storeInspect([]string{backup}, noUsage, dir, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"backup:      " + m.UUID + " (store-snapshot, format 1)", ", manual\n", "label:       by hand\n", "1 Cubes",
		"laptop (pq, " + r.Fingerprint + ")", "signature:   good, by this box (" + m.Node.Fingerprint + ")", "payload:     as the manifest names it",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("no %q in\n%s", want, out.String())
		}
	}
	out.Reset()
	if err := storeInspect([]string{backup}, noUsage, t.TempDir(), &out); err != nil || !strings.Contains(out.String(), "whether it is this one is not known") {
		t.Fatalf("without this box's key: %v\n%s", err, out.String())
	}
	if err := storeInspect([]string{keyFile}, noUsage, dir, &out); err == nil {
		t.Fatal("a key file inspected as a backup")
	}
	// A manifest changed after it was signed: nothing of it is shown.
	changed := relabelled(t, backup, "by hand", "By hand")
	out.Reset()
	if err := storeInspect([]string{changed}, noUsage, dir, &out); !errors.Is(err, serverbackup.ErrBadSignature) ||
		strings.Contains(out.String(), "By hand") || strings.Contains(out.String(), m.UUID) || !strings.Contains(out.String(), "signature:   bad") {
		t.Fatalf("a changed manifest: %v\n%s", err, out.String())
	}
	if err := storeInspect(nil, noUsage, dir, &out); err == nil {
		t.Fatal("no file")
	}

	opened := filepath.Join(t.TempDir(), "opened")
	out.Reset()
	for _, args := range [][]string{{backup, "-key", keyFile}, {"-out", opened, backup}, {backup, "-key", backup, "-out", opened}} {
		if err := storeDecrypt(args, noUsage, dir, &out, &errOut); err == nil {
			t.Errorf("%v: opened", args)
		}
	}
	if _, err := os.Stat(opened); err == nil {
		t.Fatal("a failed decrypt left its directory")
	}
	errOut.Reset()
	if err := storeDecrypt([]string{backup, "-key", keyFile, "-out", opened}, noUsage, dir, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "opened the backup "+m.UUID) || !strings.Contains(out.String(), "signature:   good, by this box ("+m.Node.Fingerprint+")") || errOut.Len() > 0 {
		t.Fatalf("printed %q and %q", out.String(), errOut.String())
	}
	for _, name := range []string{store.FileName, store.OpaqueKeyFileName, store.KeyFileName, "files.json"} {
		if _, err := os.Stat(filepath.Join(opened, name)); err != nil {
			t.Error(err)
		}
	}

	// Opened on a box whose node.key is another's: a warning.
	other := t.TempDir()
	storekit.Open(t, other)
	out.Reset()
	if err := storeDecrypt([]string{backup, "-key", keyFile, "-out", filepath.Join(t.TempDir(), "elsewhere")}, noUsage, other, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "signature:   good, by another box ("+m.Node.Fingerprint+")") || !strings.Contains(errOut.String(), "warning: the backup was made by the box "+m.Node.Fingerprint) {
		t.Fatalf("printed %q and %q", out.String(), errOut.String())
	}
}
