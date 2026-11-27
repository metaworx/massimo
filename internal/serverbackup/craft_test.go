// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// Backups made by hand, signed with a key of their own, as another box
// or a crafted file would be: what Open and ReadManifest refuse, and
// what Open leaves behind when it fails halfway.

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/crc32"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/google/uuid"
)

// innerEntry is a file of a crafted payload.
type innerEntry struct {
	name string
	data []byte
	// badCRC stores the file with a checksum that does not match; size,
	// where not 0, is the size the zip declares for it.
	badCRC bool
	size   uint64
}

// listOf is the files.json of entries, each role by its name, a TLS
// file's tls-cert.
func listOf(entries []innerEntry) []byte {
	var list filesList
	for _, e := range entries {
		role := roleOfName(e.name)
		if role == "" {
			role = RoleTLSCert
		}
		list.Files = append(list.Files, InnerFile{Name: e.name, Role: role, Source: "/var/lib/massimo/" + e.name})
	}
	b, _ := json.Marshal(list)
	return b
}

// craft makes a backup by hand at a new path: the inner zip of entries,
// with files where it is not nil (an empty one leaves files.json out)
// or else listOf(entries), encrypted for r, its manifest changed by edit
// and signed by key, a key of its own where nil. It returns the path and
// the signing key's public part.
func craft(t *testing.T, r Recipient, entries []innerEntry, files []byte, key ed25519.PrivateKey, edit func(*Manifest)) (string, ed25519.PublicKey) {
	t.Helper()
	if files == nil {
		files = listOf(entries)
	}
	var inner bytes.Buffer
	zw := zip.NewWriter(&inner)
	if len(files) > 0 {
		entries = append([]innerEntry{{name: FilesName, data: files}}, entries...)
	}
	for _, e := range entries {
		if !e.badCRC && e.size == 0 {
			w, err := zw.CreateHeader(&zip.FileHeader{Name: e.name, Method: zip.Deflate})
			if err != nil {
				t.Fatal(err)
			}
			w.Write(e.data)
			continue
		}
		crc := crc32.ChecksumIEEE(e.data)
		if e.badCRC {
			crc ^= 0x5a5a5a5a
		}
		size := uint64(len(e.data))
		if e.size != 0 {
			size = e.size
		}
		w, err := zw.CreateRaw(&zip.FileHeader{Name: e.name, Method: zip.Store, CRC32: crc, CompressedSize64: uint64(len(e.data)), UncompressedSize64: size})
		if err != nil {
			t.Fatal(err)
		}
		w.Write(e.data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	var payload bytes.Buffer
	aw, err := age.Encrypt(&payload, r.r)
	if err != nil {
		t.Fatal(err)
	}
	aw.Write(inner.Bytes())
	aw.Close()

	if key == nil {
		_, key, _ = ed25519.GenerateKey(rand.Reader)
	}
	pub := key.Public().(ed25519.PublicKey)
	sum := sha256.Sum256(payload.Bytes())
	m := Manifest{
		Format: Format, Kind: KindStoreSnapshot, UUID: uuid.NewString(), Created: time.Now().UTC().Truncate(time.Millisecond),
		Trigger: TriggerManual, Version: "v0.18.0", Schema: Schema{Version: 26, MinReader: 24},
		Node:       Node{PublicKey: base64.StdEncoding.EncodeToString(pub), Fingerprint: fingerprint(pub)},
		Recipients: []RecipientInfo{{Label: r.Label, Kind: r.Kind, Fingerprint: r.Fingerprint}},
		Payload:    Payload{Size: int64(payload.Len()), SHA256: hex.EncodeToString(sum[:])},
	}
	if edit != nil {
		edit(&m)
	}
	mb, _ := json.MarshalIndent(m, "", "  ")
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(key, mb)) + "\n"

	var outer bytes.Buffer
	ow := zip.NewWriter(&outer)
	for _, f := range []struct {
		name string
		b    []byte
	}{{ManifestName, mb}, {PayloadName, payload.Bytes()}, {SignatureName, []byte(sig)}} {
		w, err := ow.CreateHeader(&zip.FileHeader{Name: f.name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		w.Write(f.b)
	}
	if err := ow.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "crafted.zip")
	if err := os.WriteFile(path, outer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, pub
}

// openInto opens the backup at path into dir.
func openInto(t *testing.T, path string, ids []age.Identity, dir string) (Opened, error) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return Open(bytes.NewReader(b), int64(len(b)), ids, dir)
}

// emptyAfter fails where dir, given empty, holds anything after a
// failed Open.
func emptyAfter(t *testing.T, what, dir string) {
	t.Helper()
	if left, _ := os.ReadDir(dir); len(left) > 0 {
		var names []string
		for _, e := range left {
			names = append(names, e.Name())
		}
		t.Errorf("%s: left %v", what, names)
	}
}

// A backup signed by another box opens, and Open says which box; with
// that box's key as this one's, it is this box's.
func TestOpenAnotherBox(t *testing.T) {
	r, ids := keyOf(t, KindX25519, "laptop")
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	path, pub := craft(t, r, []innerEntry{{name: "massimo.db", data: []byte("a store")}, {name: "tls/acme_account+key", data: []byte("a key")}}, nil, key, nil)
	dir := filepath.Join(t.TempDir(), "opened")
	o, err := openInto(t, path, ids, dir)
	if err != nil {
		t.Fatal(err)
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	if o.Signature.Verdict(other) != OtherBox || o.Signature.Verdict(pub) != ThisBox || o.Signature.Fingerprint != fingerprint(pub) {
		t.Fatalf("signature %+v", o.Signature)
	}
	if len(o.Files) != 2 || o.Files[1] != (InnerFile{"tls/acme_account+key", RoleTLSCert, "/var/lib/massimo/tls/acme_account+key"}) {
		t.Fatalf("files %+v", o.Files)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "tls", "acme_account+key")); string(b) != "a key" {
		t.Fatalf("the TLS file: %q", b)
	}
}

// A payload that fails halfway through its files leaves the directory
// as it was: a new one gone, an empty one empty, the payload's copy
// removed too.
func TestOpenCleansUpMidExtraction(t *testing.T) {
	r, ids := keyOf(t, KindX25519, "laptop")
	good := innerEntry{name: "massimo.db", data: bytes.Repeat([]byte("a store "), 1000)}
	for name, bad := range map[string]innerEntry{
		"a wrong checksum":  {name: "opaque.key", data: []byte("a key"), badCRC: true},
		"longer than said":  {name: "opaque.key", data: []byte("a longer key"), size: 3},
		"shorter than said": {name: "opaque.key", data: []byte("a key"), size: 300},
	} {
		path, _ := craft(t, r, []innerEntry{good, bad}, nil, nil, nil)
		fresh := filepath.Join(t.TempDir(), "opened")
		// The store is written, and opaque.key fails after it.
		if _, err := openInto(t, path, ids, fresh); err == nil || !strings.Contains(err.Error(), "serverbackup: opaque.key: ") {
			t.Errorf("%s: %v", name, err)
		}
		if _, err := os.Stat(fresh); err == nil {
			t.Errorf("%s: the directory Open made stays", name)
		}
		given := t.TempDir()
		if _, err := openInto(t, path, ids, given); err == nil {
			t.Errorf("%s: opened", name)
		}
		emptyAfter(t, name, given)
	}
}

// The names a payload may not hold are refused, for every system, before
// anything is written.
func TestOpenRefusesNames(t *testing.T) {
	r, ids := keyOf(t, KindX25519, "laptop")
	for _, name := range []string{
		"../x", "tls/../x", "tls/../../x", "/etc/x", "tls//x", "tls/CON", "tls/nul.txt", "tls/com1", "tls/LPT9.pem",
		"tls/CONIN$", "tls/x.", "tls/x ", "tls/a:b", `tls\x`, "tls/sub/x", "tls/.hidden", "TLS/x", "x", "massimo.db/x", "tls/",
		"C:/x", "tls/a\x01b",
	} {
		path, _ := craft(t, r, []innerEntry{{name: "massimo.db", data: []byte("a store")}, {name: name, data: []byte("x")}}, nil, nil, nil)
		dir := t.TempDir()
		if _, err := openInto(t, path, ids, dir); err == nil {
			t.Errorf("%q: opened", name)
		}
		emptyAfter(t, name, dir)
	}
	for _, name := range []string{"cert.pem", "acme_account+key", "massimo.example.net", "2-cert.pem", "console.pem", "nul-not.pem", "com10"} {
		if !validTLSName(name) {
			t.Errorf("%q refused", name)
		}
	}
	for _, name := range []string{"CON", "con.pem", "Nul", "AUX.tar.gz", "com1", "LPT3", "com¹", "x.", "x ", "..", ".", "", "a/b", `a\b`, "a:b", "a*b"} {
		if validTLSName(name) {
			t.Errorf("%q taken", name)
		}
	}
}

// files.json must name each other file of the payload once, with the
// role its name has and a source that is text, and the store among them.
func TestOpenChecksFilesJSON(t *testing.T) {
	r, ids := keyOf(t, KindX25519, "laptop")
	store := innerEntry{name: "massimo.db", data: []byte("a store")}
	key := innerEntry{name: "opaque.key", data: []byte("a key")}
	for name, c := range map[string]struct {
		entries []innerEntry
		files   []byte
	}{
		"no files.json":         {[]innerEntry{store}, []byte{}},
		"not JSON":              {[]innerEntry{store}, []byte("{")},
		"a file it lacks":       {[]innerEntry{store}, listOf([]innerEntry{store, key})},
		"a file it leaves out":  {[]innerEntry{store, key}, listOf([]innerEntry{store})},
		"no store":              {[]innerEntry{key}, nil},
		"a role not its name's": {[]innerEntry{store}, []byte(`{"files":[{"name":"massimo.db","role":"env","source":"/x"}]}`)},
		"a TLS file as a store": {[]innerEntry{store, {name: "tls/x", data: nil}}, []byte(`{"files":[{"name":"massimo.db","role":"store","source":"/x"},{"name":"tls/x","role":"store","source":"/x"}]}`)},
		"a source with a break": {[]innerEntry{store}, []byte(`{"files":[{"name":"massimo.db","role":"store","source":"/x\ny"}]}`)},
		"itself":                {[]innerEntry{store}, []byte(`{"files":[{"name":"massimo.db","role":"store","source":"/x"},{"name":"files.json","role":"tls-cert","source":"/x"}]}`)},
		"a file twice":          {[]innerEntry{store}, []byte(`{"files":[{"name":"massimo.db","role":"store","source":"/x"},{"name":"massimo.db","role":"store","source":"/x"}]}`)},
	} {
		path, _ := craft(t, r, c.entries, c.files, nil, nil)
		dir := t.TempDir()
		if _, err := openInto(t, path, ids, dir); !errors.Is(err, ErrNotABackup) {
			t.Errorf("%s: %v", name, err)
		}
		emptyAfter(t, name, dir)
	}
}

// Sizes the free space cannot take, or that overflow, are refused before
// a file is written.
func TestOpenRefusesSizes(t *testing.T) {
	r, ids := keyOf(t, KindX25519, "laptop")
	huge := uint64(math.MaxInt64)
	path, _ := craft(t, r, []innerEntry{
		{name: "massimo.db", data: []byte("a store"), size: huge},
		{name: "opaque.key", data: []byte("a key"), size: huge},
		{name: "node.key", data: []byte("a key"), size: huge},
	}, nil, nil, nil)
	freeSpace = func(string) (uint64, error) { return math.MaxUint64, nil }
	t.Cleanup(func() { freeSpace = diskFree })
	dir := t.TempDir()
	if _, err := openInto(t, path, ids, dir); err == nil || !strings.Contains(err.Error(), "sizes are not ones") {
		t.Errorf("sizes that overflow: %v", err)
	}
	emptyAfter(t, "overflow", dir)

	path, _ = craft(t, r, []innerEntry{{name: "massimo.db", data: []byte("a store"), size: 1 << 30}}, nil, nil, nil)
	freeSpace = func(string) (uint64, error) { return 1 << 20, nil }
	if _, err := openInto(t, path, ids, dir); err == nil || !strings.Contains(err.Error(), "massimo.db alone needs") {
		t.Errorf("a file larger than the free space: %v", err)
	}
	emptyAfter(t, "too large", dir)
}

// The payload is read from the file once: what is decrypted is what was
// hashed, whatever a reader would answer a second time.
func TestOpenReadsThePayloadOnce(t *testing.T) {
	st := testStore(t)
	r, ids := keyOf(t, KindX25519, "laptop")
	out, m := write(t, st, Inputs{}, r)
	b, _ := os.ReadFile(out)
	zr, _ := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	off, _ := zr.File[1].DataOffset()
	mid := off + m.Payload.Size/2
	rr := &recordingReader{r: bytes.NewReader(b)}
	if _, err := Open(rr, int64(len(b)), ids, filepath.Join(t.TempDir(), "opened")); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, rg := range rr.ranges {
		if rg[0] <= mid && mid < rg[1] {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("the payload's middle byte read %d times", n)
	}
}

// A manifest whose text is not one Write writes is no backup, signed or
// not, and nothing of it is returned; a newer format or kind asks for an
// update.
func TestManifestTextChecked(t *testing.T) {
	r, _ := keyOf(t, KindX25519, "laptop")
	entries := []innerEntry{{name: "massimo.db", data: []byte("a store")}}
	for name, edit := range map[string]func(*Manifest){
		"a label with a control": func(m *Manifest) { m.Label = "a\x1b[2Jb" },
		"a long label":           func(m *Manifest) { m.Label = strings.Repeat("x", MaxLabel+1) },
		"an upload's trigger":    func(m *Manifest) { m.Trigger = TriggerUploaded },
		"another trigger":        func(m *Manifest) { m.Trigger = "evil" },
		"no uuid":                func(m *Manifest) { m.UUID = "not-a-uuid" },
		"a braced uuid":          func(m *Manifest) { m.UUID = "{" + uuid.NewString() + "}" },
		"a version with a NUL":   func(m *Manifest) { m.Version = "v1\x00" },
		"no version":             func(m *Manifest) { m.Version = "" },
		"no time":                func(m *Manifest) { m.Created = time.Time{} },
		"a count below 0":        func(m *Manifest) { m.Counts.Users = -1 },
		"no recipient":           func(m *Manifest) { m.Recipients = nil },
		"a recipient's label":    func(m *Manifest) { m.Recipients[0].Label = "a\nb" },
		"a recipient's kind":     func(m *Manifest) { m.Recipients[0].Kind = "rsa" },
		"a recipient's print":    func(m *Manifest) { m.Recipients[0].Fingerprint = "SHA256:x" },
		"a mix": func(m *Manifest) {
			m.Recipients = append(m.Recipients, RecipientInfo{"pq", KindPostQuantum, m.Recipients[0].Fingerprint})
		},
		"a node's print":    func(m *Manifest) { m.Node.Fingerprint = "\x1b[2J" },
		"a hash":            func(m *Manifest) { m.Payload.SHA256 = strings.ToUpper(m.Payload.SHA256) },
		"a kind not a word": func(m *Manifest) { m.Kind = "Store Snapshot!" },
		"no format":         func(m *Manifest) { m.Format = 0 },
	} {
		path, _ := craft(t, r, entries, nil, nil, edit)
		m, sig, err := readFile(t, path)
		if !errors.Is(err, ErrNotABackup) || m.UUID != "" || sig.Valid {
			t.Errorf("%s: %+v %v", name, m, err)
		}
	}
	for name, edit := range map[string]func(*Manifest){
		"format 2":     func(m *Manifest) { m.Format = 2 },
		"another kind": func(m *Manifest) { m.Kind = "store-export" },
	} {
		path, _ := craft(t, r, entries, nil, nil, edit)
		if _, _, err := readFile(t, path); !errors.Is(err, ErrNewerFormat) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Write removes what a crashed Write left in the data directory, once
// older than six hours, and leaves a running one's.
func TestWriteRemovesStale(t *testing.T) {
	st := testStore(t)
	r, _ := keyOf(t, KindX25519, "")
	old := filepath.Join(st.Dir(), tempPrefix+"old")
	young := filepath.Join(st.Dir(), tempPrefix+"young")
	for _, d := range []string{old, young} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(d, "massimo.db"), []byte("a store in plain"), 0o600)
	}
	then := time.Now().Add(-staleAfter - time.Minute)
	if err := os.Chtimes(old, then, then); err != nil {
		t.Fatal(err)
	}
	write(t, st, Inputs{}, r)
	if _, err := os.Stat(old); err == nil {
		t.Error("an old temporary directory stays")
	}
	if _, err := os.Stat(filepath.Join(young, "massimo.db")); err != nil {
		t.Errorf("a young temporary directory went: %v", err)
	}
}

// A Write that fails while it writes the archive leaves neither the
// backup's name nor safefile's temporary file, nor its own directory.
func TestWriteFailureLeavesNothing(t *testing.T) {
	st := testStore(t)
	r, _ := keyOf(t, KindX25519, "")
	archiveFill = func(path string, _ []byte, _ string, _ []byte, _ time.Time) error {
		os.WriteFile(path, []byte("PK half an archive"), 0o600)
		return errors.New("the disk went away")
	}
	t.Cleanup(func() { archiveFill = fillArchive })
	out := filepath.Join(t.TempDir(), "backup.zip")
	if _, err := Write(context.Background(), Inputs{Store: st, Trigger: TriggerManual}, []Recipient{r}, out); err == nil || !strings.Contains(err.Error(), "the disk went away") {
		t.Fatalf("%v", err)
	}
	emptyAfter(t, "the archive's directory", filepath.Dir(out))
	noTemporaries(t, st, out)
}
