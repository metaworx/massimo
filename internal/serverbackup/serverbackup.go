// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package serverbackup makes and opens the server's own backups (AP
// ServerBackup; docs/store.md, "Server backups"): a zip, stored without
// compression, of three files, in this order:
//
//   - manifest.json, plain: what the backup is, without a name of a
//     Cube, a room or a user (Manifest);
//   - payload.age, an age file (filippo.io/age) for the owner's
//     recipients, of a deflated zip with files.json, which names the
//     other files' roles and where they were (files.go), a snapshot of
//     the store (massimo.db), opaque.key, node.key, the env file serve
//     reads (env) and the TLS files (tls/<name>);
//   - manifest.sig, manifest.json signed by node.key, in base64.
//
// The manifest is read, its text checked, and the payload's size and
// hash checked, without the key (ReadManifest, Verify), so a target
// lists its backups by ranged reads; only the owner's identity opens
// the payload (Open), with Massimo or with age -d anywhere. The server
// keeps the recipients, never an identity (ParseRecipient, GenerateKey).
package serverbackup

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"filippo.io/age"
	"github.com/google/uuid"

	"go.metaworx.ch/massimo/internal/safefile"
	"go.metaworx.ch/massimo/internal/store"
)

// The archive's format and its files.
const (
	// Format is the manifest's format version this package writes and
	// reads; a newer one is refused.
	Format = 1
	// KindStoreSnapshot is the manifest's kind of a snapshot of the
	// whole store; a later export would be a kind of its own.
	KindStoreSnapshot = "store-snapshot"

	ManifestName  = "manifest.json"
	PayloadName   = "payload.age"
	SignatureName = "manifest.sig"

	// EnvName is the env file in the payload, and TLSDir the directory
	// of the TLS files.
	EnvName = "env"
	TLSDir  = "tls"
)

// Trigger is why a backup was made.
type Trigger string

// The triggers.
const (
	TriggerScheduled     Trigger = "scheduled"
	TriggerManual        Trigger = "manual"
	TriggerBeforeUpdate  Trigger = "before-update"
	TriggerBeforeRestore Trigger = "before-restore"
	// TriggerUploaded is a file that came in by upload; its manifest
	// keeps the trigger it was made with, so Write refuses this one.
	TriggerUploaded Trigger = "uploaded"
)

// Valid reports whether t is one of the triggers.
func (t Trigger) Valid() bool {
	switch t {
	case TriggerScheduled, TriggerManual, TriggerBeforeUpdate, TriggerBeforeRestore, TriggerUploaded:
		return true
	}
	return false
}

// Manifest is manifest.json: plain, signed, and without a name of a
// Cube, a room or a user.
type Manifest struct {
	Format  int       `json:"format"`
	Kind    string    `json:"kind"`
	UUID    string    `json:"uuid"` // the backup's, the same on every target
	Created time.Time `json:"created"`
	Trigger Trigger   `json:"trigger"`
	Label   string    `json:"label,omitempty"`
	Version string    `json:"version"` // Massimo's, of the server that made it
	Schema  Schema    `json:"schema"`
	Node    Node      `json:"node"`
	// Counts are the snapshot's rows per scope.
	Counts store.SnapshotCounts `json:"counts"`
	Files  Files                `json:"files"`
	// Recipients are the keys the payload is encrypted for.
	Recipients []RecipientInfo `json:"recipients"`
	Payload    Payload         `json:"payload"`
}

// Schema is the snapshot's schema version and min_reader.
type Schema struct {
	Version   int `json:"version"`
	MinReader int `json:"min_reader"`
}

// Node is the box that made the backup: its node key's public part, in
// base64, which manifest.sig verifies with, and its fingerprint.
type Node struct {
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
}

// Files says what the payload holds beside the store and its two key
// files: whether the env file, and how many TLS files.
type Files struct {
	Env bool `json:"env"`
	TLS int  `json:"tls"`
}

// RecipientInfo is a recipient as the manifest names it.
type RecipientInfo struct {
	Label       string `json:"label"`
	Kind        Kind   `json:"kind"`
	Fingerprint string `json:"fingerprint"`
}

// Payload is payload.age's size and SHA-256, in hex: the encrypted
// file's, so that a copy is checked without the key.
type Payload struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Inputs is what Write backs up.
type Inputs struct {
	// Store is the store, opened with store.Open, whose node key signs;
	// its directory, the data directory, holds opaque.key, node.key and
	// the temporary files.
	Store *store.Store
	// EnvFile is the env file serve reads; "", a path that does not
	// exist and one that is not a regular file (/dev/null) are none.
	EnvFile string
	// TLSFiles are the TLS files with their roles, each in the payload
	// as tls/<its name>, a number before it where two share one.
	TLSFiles []TLSFile
	Trigger  Trigger
	// Label is the owner's text for the backup, at most MaxLabel
	// characters, in plain in the manifest.
	Label string
	// Version is Massimo's version, printable ASCII of at most
	// maxVersion bytes; "" is "unknown".
	Version string
	// Time is when the backup is made; zero is now.
	Time time.Time
}

// MaxLabel is the longest label, in characters.
const MaxLabel = 100

// maxTLSFiles bounds the TLS files of a payload.
const maxTLSFiles = 256

// maxVersion bounds the manifest's version.
const maxVersion = 128

// tempPrefix begins the names of Write's temporary directories in the
// data directory, and staleAfter is the age at which a later Write
// removes one, the leftover of a crash: no backup runs that long.
const (
	tempPrefix = ".serverbackup-"
	staleAfter = 6 * time.Hour
)

// The seams the tests replace: the free space of a directory, and the
// filling of the archive's temporary file.
var (
	freeSpace   = diskFree
	archiveFill = fillArchive
)

// Write makes a server backup of in for the recipients at out, which
// must not exist: a snapshot of the store into a temporary directory in
// the data directory, the inner zip deflated and encrypted with age into
// a temporary payload.age while it is hashed, the manifest signed, then
// the outer zip, written whole (internal/safefile) with mode 0600.
// When Write returns, its temporary files are gone, and so is out where
// it failed. A crash leaves them: the temporary directory, which holds
// the unencrypted snapshot, is removed by the next Write once older than
// staleAfter; out, empty, and safefile's temporary file beside it hold
// no part of the store in plain. About twice the store must be free in
// the data directory, and once the store where out is; less is refused
// with the numbers.
func Write(ctx context.Context, in Inputs, recipients []Recipient, out string) (Manifest, error) {
	if in.Version == "" {
		in.Version = "unknown"
	}
	if err := in.check(); err != nil {
		return Manifest{}, err
	}
	if err := CheckKinds(recipients); err != nil {
		return Manifest{}, err
	}
	for _, r := range recipients {
		if err := CheckLabel(r.Label); err != nil {
			return Manifest{}, fmt.Errorf("%w (of the recipient %s)", err, r.Fingerprint)
		}
	}
	pub := in.Store.NodePublicKey()
	if len(pub) == 0 {
		return Manifest{}, errors.New("serverbackup: the store has no node key; open it with store.Open")
	}
	dir := in.Store.Dir()
	removeStale(dir, time.Now())
	if err := checkSpace(dir, filepath.Dir(out)); err != nil {
		return Manifest{}, err
	}
	created := in.Time
	if created.IsZero() {
		created = time.Now()
	}
	created = created.UTC().Truncate(time.Millisecond)
	id, err := uuid.NewV7()
	if err != nil {
		return Manifest{}, fmt.Errorf("serverbackup: %w", err)
	}

	tmp, err := os.MkdirTemp(dir, tempPrefix+"*")
	if err != nil {
		return Manifest{}, fmt.Errorf("serverbackup: %w", err)
	}
	defer os.RemoveAll(tmp)
	snapPath := filepath.Join(tmp, store.FileName)
	snap, err := in.Store.Snapshot(ctx, snapPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("serverbackup: %w", err)
	}
	payloadPath := filepath.Join(tmp, PayloadName)
	files, payload, err := writePayload(ctx, payloadPath, snapPath, in, recipients, created)
	if err != nil {
		return Manifest{}, err
	}
	// The snapshot is in the payload; its room is the outer zip's.
	os.Remove(snapPath)

	m := Manifest{
		Format: Format, Kind: KindStoreSnapshot, UUID: id.String(), Created: created,
		Trigger: in.Trigger, Label: in.Label, Version: in.Version,
		Schema: Schema{Version: snap.Schema, MinReader: snap.MinReader},
		Node:   Node{PublicKey: base64.StdEncoding.EncodeToString(pub), Fingerprint: fingerprint(pub)},
		Counts: snap.Counts, Files: files, Payload: payload,
	}
	for _, r := range recipients {
		m.Recipients = append(m.Recipients, RecipientInfo{Label: r.Label, Kind: r.Kind, Fingerprint: r.Fingerprint})
	}
	mb, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Manifest{}, fmt.Errorf("serverbackup: manifest: %w", err)
	}
	mb = append(mb, '\n')
	sig, err := in.Store.Sign(mb)
	if err != nil {
		return Manifest{}, fmt.Errorf("serverbackup: %w", err)
	}
	sb := []byte(base64.StdEncoding.EncodeToString(sig) + "\n")
	if err := ctx.Err(); err != nil {
		return Manifest{}, fmt.Errorf("serverbackup: %w", err)
	}
	if err := writeArchive(out, mb, payloadPath, sb, created); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// check refuses inputs Write cannot back up.
func (in Inputs) check() error {
	switch {
	case in.Store == nil:
		return errors.New("serverbackup: no store")
	case !in.Trigger.Valid() || in.Trigger == TriggerUploaded:
		return fmt.Errorf("serverbackup: trigger %q", in.Trigger)
	case !validVersion(in.Version):
		return errors.New("serverbackup: the version is not printable ASCII of at most 128 bytes")
	case len(in.TLSFiles) > maxTLSFiles:
		return fmt.Errorf("serverbackup: %d TLS files, at most %d", len(in.TLSFiles), maxTLSFiles)
	}
	for _, f := range in.TLSFiles {
		if !f.Role.TLS() {
			return fmt.Errorf("serverbackup: the TLS file %s has the role %q", f.Path, f.Role)
		}
	}
	return CheckLabel(in.Label)
}

// validVersion reports whether v is printable ASCII of 1 to maxVersion
// bytes, as Massimo's versions are.
func validVersion(v string) bool {
	return v != "" && len(v) <= maxVersion && strings.IndexFunc(v, func(r rune) bool { return r < 0x20 || r > 0x7e }) < 0
}

// removeStale removes the temporary directories of Write in dir that are
// older than staleAfter: what a crash left, the unencrypted snapshot
// among it. A running Write's are younger.
func removeStale(dir string, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), tempPrefix) {
			continue
		}
		if fi, err := e.Info(); err == nil && now.Sub(fi.ModTime()) > staleAfter {
			os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

// CheckLabel refuses a label of more than MaxLabel characters, or one
// that is not UTF-8 or holds a control or format character, which the
// manifest would carry in plain to every target.
func CheckLabel(label string) error {
	switch {
	case !utf8.ValidString(label):
		return errors.New("serverbackup: the label is not UTF-8")
	case utf8.RuneCountInString(label) > MaxLabel:
		return fmt.Errorf("serverbackup: the label is longer than %d characters", MaxLabel)
	case strings.IndexFunc(label, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) >= 0:
		return errors.New("serverbackup: the label holds a control character")
	}
	return nil
}

// checkSpace refuses a backup when the data directory has less than
// about twice the store free, the snapshot and the payload, or out's
// directory less than the store, the archive.
func checkSpace(dataDir, outDir string) error {
	var size int64
	for _, p := range []string{store.Path(dataDir), store.Path(dataDir) + "-wal"} {
		if fi, err := os.Stat(p); err == nil {
			size += fi.Size()
		}
	}
	for _, c := range []struct {
		dir  string
		need int64
		what string
	}{
		{dataDir, 2 * size, "twice the store"},
		{outDir, size, "the store"},
	} {
		free, err := freeSpace(c.dir)
		switch {
		case errors.Is(err, errors.ErrUnsupported):
			continue
		case err != nil:
			return fmt.Errorf("serverbackup: free space of %s: %w", c.dir, err)
		case free < uint64(c.need):
			return fmt.Errorf("serverbackup: %s has %s free; a backup needs about %s there, %s of %s", c.dir, bytesText(int64(free)), bytesText(c.need), c.what, bytesText(size))
		}
	}
	return nil
}

// bytesText is n in bytes and MiB.
func bytesText(n int64) string {
	return fmt.Sprintf("%d bytes (%.1f MiB)", n, float64(n)/(1<<20))
}

// writePayload writes payload.age at path: the inner zip, deflated, with
// files.json first, then the snapshot, the key files, the env file and
// the TLS files, encrypted for the recipients and hashed as it is
// written.
func writePayload(ctx context.Context, path, snapPath string, in Inputs, recipients []Recipient, created time.Time) (Files, Payload, error) {
	var files Files
	type entry struct {
		InnerFile
		path string // what is read: the snapshot for the store
	}
	dir := in.Store.Dir()
	entries := []entry{
		{InnerFile{store.FileName, RoleStore, absPath(store.Path(dir))}, snapPath},
		{InnerFile{store.OpaqueKeyFileName, RoleOpaqueKey, absPath(store.OpaqueKeyPath(dir))}, store.OpaqueKeyPath(dir)},
		{InnerFile{store.KeyFileName, RoleNodeKey, absPath(store.KeyPath(dir))}, store.KeyPath(dir)},
	}
	if in.EnvFile != "" {
		fi, err := os.Stat(in.EnvFile)
		switch {
		case err == nil && fi.Mode().IsRegular():
			entries = append(entries, entry{InnerFile{EnvName, RoleEnv, absPath(in.EnvFile)}, in.EnvFile})
			files.Env = true
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			return files, Payload{}, fmt.Errorf("serverbackup: the env file: %w", err)
		}
	}
	taken := map[string]bool{}
	for _, t := range in.TLSFiles {
		name, err := tlsName(t.Path, taken)
		if err != nil {
			return files, Payload{}, err
		}
		taken[name] = true
		entries = append(entries, entry{InnerFile{name, t.Role, absPath(t.Path)}, t.Path})
	}
	files.TLS = len(in.TLSFiles)
	var list filesList
	for _, e := range entries {
		if !plainText(e.Source, maxSource) {
			return files, Payload{}, fmt.Errorf("serverbackup: the path of %s is not one files.json carries", e.Name)
		}
		list.Files = append(list.Files, e.InnerFile)
	}
	listJSON, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return files, Payload{}, fmt.Errorf("serverbackup: %s: %w", FilesName, err)
	}

	ars := make([]age.Recipient, len(recipients))
	for i, r := range recipients {
		ars[i] = r.r
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return files, Payload{}, fmt.Errorf("serverbackup: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	cw := &countWriter{w: io.MultiWriter(f, h)}
	aw, err := age.Encrypt(cw, ars...)
	if err != nil {
		return files, Payload{}, fmt.Errorf("serverbackup: encrypt: %w", err)
	}
	zw := zip.NewWriter(aw)
	if err := addFile(zw, FilesName, bytes.NewReader(append(listJSON, '\n')), created); err != nil {
		return files, Payload{}, err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return files, Payload{}, fmt.Errorf("serverbackup: %w", err)
		}
		src, err := os.Open(e.path)
		if err != nil {
			return files, Payload{}, fmt.Errorf("serverbackup: %w", err)
		}
		err = addFile(zw, e.Name, src, created)
		src.Close()
		if err != nil {
			return files, Payload{}, err
		}
	}
	if err := zw.Close(); err != nil {
		return files, Payload{}, fmt.Errorf("serverbackup: payload: %w", err)
	}
	if err := aw.Close(); err != nil {
		return files, Payload{}, fmt.Errorf("serverbackup: payload: %w", err)
	}
	if err := f.Close(); err != nil {
		return files, Payload{}, fmt.Errorf("serverbackup: payload: %w", err)
	}
	return files, Payload{Size: cw.n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

// absPath is p made absolute, or p where that fails.
func absPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// addFile deflates what src holds into zw as name.
func addFile(zw *zip.Writer, name string, src io.Reader, modified time.Time) error {
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: modified}
	hdr.SetMode(0o600)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return fmt.Errorf("serverbackup: %s: %w", name, err)
	}
	if _, err := io.Copy(w, src); err != nil {
		return fmt.Errorf("serverbackup: %s: %w", name, err)
	}
	return nil
}

// writeArchive writes the outer zip at out, which must not exist: the
// name is taken first, empty, then filled whole through a temporary file
// beside it, as the store's copies are; where that fails, safefile
// removes its temporary file and writeArchive the name.
func writeArchive(out string, manifest []byte, payloadPath string, sig []byte, modified time.Time) error {
	f, err := os.OpenFile(out, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("serverbackup: %w", err)
	}
	f.Close()
	err = safefile.Replace(out, safefile.Options{Mode: 0o600}, func(tmp string) error {
		return archiveFill(tmp, manifest, payloadPath, sig, modified)
	})
	if err != nil {
		os.Remove(out)
		return fmt.Errorf("serverbackup: %s: %w", out, err)
	}
	return nil
}

// fillArchive writes the outer zip's three files, stored, to path.
func fillArchive(path string, manifest []byte, payloadPath string, sig []byte, modified time.Time) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	p, err := os.Open(payloadPath)
	if err != nil {
		return err
	}
	defer p.Close()
	zw := zip.NewWriter(f)
	for _, e := range []struct {
		name string
		r    io.Reader
	}{
		{ManifestName, strings.NewReader(string(manifest))},
		{PayloadName, p},
		{SignatureName, strings.NewReader(string(sig))},
	} {
		hdr := &zip.FileHeader{Name: e.name, Method: zip.Store, Modified: modified}
		hdr.SetMode(0o600)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, e.r); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}

// countWriter counts what it writes.
type countWriter struct {
	w io.Writer
	n int64
}

// Write writes p and counts it.
func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
