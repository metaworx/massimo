// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"golang.org/x/crypto/ssh"

	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/testkit/storekit"
)

// testStore opens a store of the kit with a Cube, two rooms and a user.
func testStore(t *testing.T) *store.Store {
	t.Helper()
	st := storekit.Open(t, "")
	ctx := context.Background()
	c, err := st.AddCube(ctx, store.Cube{Serial: "KEQ0000000", Configured: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SyncRooms(ctx, c.ID, []uint8{1, 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddUser(ctx, store.User{Name: "anna", Role: store.RoleOperator}, "", 0); err != nil {
		t.Fatal(err)
	}
	return st
}

// keyOf makes a recipient of kind, labelled, and the identities that
// open what is encrypted for it, read as a key file is.
func keyOf(t *testing.T, kind Kind, label string) (Recipient, []age.Identity) {
	t.Helper()
	var file []byte
	var r Recipient
	switch kind {
	case KindSSH:
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		spk, err := ssh.NewPublicKey(pub)
		if err != nil {
			t.Fatal(err)
		}
		if r, err = ParseRecipient(string(ssh.MarshalAuthorizedKey(spk)) + " anna@laptop"); err != nil {
			t.Fatal(err)
		}
		blk, err := ssh.MarshalPrivateKey(priv, "")
		if err != nil {
			t.Fatal(err)
		}
		file = pem.EncodeToMemory(blk)
	default:
		k, err := GenerateKey(kind)
		if err != nil {
			t.Fatal(err)
		}
		r, file = k.Recipient, k.File(time.Now())
	}
	if r.Kind != kind {
		t.Fatalf("kind %q, not %q", r.Kind, kind)
	}
	ids, err := ParseIdentities(file)
	if err != nil {
		t.Fatal(err)
	}
	r.Label = label
	return r, ids
}

// sideFiles writes an env file and two TLS files.
func sideFiles(t *testing.T) (envFile string, tls []TLSFile) {
	t.Helper()
	d := t.TempDir()
	envFile = filepath.Join(d, "env")
	tls = []TLSFile{{RoleTLSCert, filepath.Join(d, "cert.pem")}, {RoleTLSKey, filepath.Join(d, "key.pem")}}
	for p, s := range map[string]string{envFile: "MASSIMO_LISTEN=:8080\n", tls[0].Path: "a certificate\n", tls[1].Path: "a key\n"} {
		if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return envFile, tls
}

// write makes a backup of st for rs at a new path and returns it.
func write(t *testing.T, st *store.Store, in Inputs, rs ...Recipient) (string, Manifest) {
	t.Helper()
	in.Store = st
	if in.Trigger == "" {
		in.Trigger = TriggerManual
	}
	out := filepath.Join(t.TempDir(), "backup.zip")
	m, err := Write(context.Background(), in, rs, out)
	if err != nil {
		t.Fatal(err)
	}
	return out, m
}

// readFile reads a backup's manifest from its file.
func readFile(t *testing.T, path string) (Manifest, Signature, error) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, sig, _, err := ReadManifest(bytes.NewReader(b), int64(len(b)))
	return m, sig, err
}

// openFile opens a backup's file into a new directory, returned.
func openFile(t *testing.T, path string, ids []age.Identity) (string, Opened, error) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "opened")
	o, err := Open(bytes.NewReader(b), int64(len(b)), ids, dir)
	return dir, o, err
}

// noTemporaries fails where the data directory or out's directory keeps
// anything but what it should.
func noTemporaries(t *testing.T, st *store.Store, out string) {
	t.Helper()
	if left, _ := filepath.Glob(filepath.Join(st.Dir(), ".serverbackup-*")); len(left) > 0 {
		t.Errorf("left in the data directory: %v", left)
	}
	entries, _ := os.ReadDir(filepath.Dir(out))
	for _, e := range entries {
		if e.Name() != filepath.Base(out) {
			t.Errorf("left beside the backup: %s", e.Name())
		}
	}
}

// A backup made for each kind of recipient reads without the key, opens
// with it, and gives back the store, its key files and the files beside.
func TestRoundTrip(t *testing.T) {
	for _, kind := range []Kind{KindPostQuantum, KindX25519, KindSSH} {
		t.Run(string(kind), func(t *testing.T) {
			st := testStore(t)
			r, ids := keyOf(t, kind, "laptop")
			envFile, tls := sideFiles(t)
			at := time.Date(2026, 11, 27, 3, 30, 0, 123456789, time.UTC)
			out, m := write(t, st, Inputs{EnvFile: envFile, TLSFiles: tls, Trigger: TriggerScheduled, Label: "vor dem Umzug", Version: "v0.18.0", Time: at}, r)
			noTemporaries(t, st, out)
			if fi, err := os.Stat(out); err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600) {
				t.Fatalf("the backup: %v %v", fi, err)
			}

			got, sig, err := readFile(t, out)
			if err != nil {
				t.Fatal(err)
			}
			if got.UUID != m.UUID || got.Format != 1 || got.Kind != KindStoreSnapshot || !got.Created.Equal(at.Truncate(time.Millisecond)) ||
				got.Trigger != TriggerScheduled || got.Label != "vor dem Umzug" || got.Version != "v0.18.0" {
				t.Fatalf("manifest %+v", got)
			}
			v, mr := st.SchemaVersion()
			if got.Schema != (Schema{v, mr}) || got.Files != (Files{Env: true, TLS: 2}) {
				t.Fatalf("schema %+v, files %+v", got.Schema, got.Files)
			}
			if c := got.Counts; c.Users != 1 || c.Cubes != 1 || c.Rooms != 2 || c.Integrations != 0 {
				t.Fatalf("counts %+v", c)
			}
			if len(got.Recipients) != 1 || got.Recipients[0] != (RecipientInfo{"laptop", kind, r.Fingerprint}) {
				t.Fatalf("recipients %+v", got.Recipients)
			}
			if got.Node.Fingerprint != fingerprint(st.NodePublicKey()) || sig.Fingerprint != got.Node.Fingerprint {
				t.Fatalf("node %+v, signature %+v", got.Node, sig)
			}
			other, _, _ := ed25519.GenerateKey(rand.Reader)
			if sig.Verdict(st.NodePublicKey()) != ThisBox || sig.Verdict(other) != OtherBox || sig.Verdict(nil) != OtherBox {
				t.Fatalf("verdicts %v %v %v", sig.Verdict(st.NodePublicKey()), sig.Verdict(other), sig.Verdict(nil))
			}
			b, _ := os.ReadFile(out)
			for _, name := range []string{"anna", "KEQ0000000", "MASSIMO_LISTEN", "certificate"} {
				if bytes.Contains(b, []byte(name)) {
					t.Errorf("%q in plain in the backup", name)
				}
			}

			dir, opened, err := openFile(t, out, ids)
			if err != nil {
				t.Fatal(err)
			}
			if opened.Manifest.UUID != m.UUID || opened.Signature.Verdict(st.NodePublicKey()) != ThisBox {
				t.Fatalf("opened %s (%v), made %s", opened.Manifest.UUID, opened.Signature.Verdict(st.NodePublicKey()), m.UUID)
			}
			abs := func(p string) string { a, _ := filepath.Abs(p); return a }
			want := map[string]InnerFile{
				store.FileName:          {store.FileName, RoleStore, abs(store.Path(st.Dir()))},
				store.OpaqueKeyFileName: {store.OpaqueKeyFileName, RoleOpaqueKey, abs(store.OpaqueKeyPath(st.Dir()))},
				store.KeyFileName:       {store.KeyFileName, RoleNodeKey, abs(store.KeyPath(st.Dir()))},
				EnvName:                 {EnvName, RoleEnv, abs(envFile)},
				"tls/cert.pem":          {"tls/cert.pem", RoleTLSCert, abs(tls[0].Path)},
				"tls/key.pem":           {"tls/key.pem", RoleTLSKey, abs(tls[1].Path)},
			}
			if len(opened.Files) != len(want) {
				t.Fatalf("files.json: %+v", opened.Files)
			}
			for _, f := range opened.Files {
				if want[f.Name] != f {
					t.Errorf("files.json: %+v, not %+v", f, want[f.Name])
				}
				if f.Role == RoleStore {
					continue
				}
				a, _ := os.ReadFile(f.Source)
				g, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.Name)))
				if err != nil || !bytes.Equal(a, g) {
					t.Errorf("%s: %v", f.Name, err)
				}
			}
			if _, err := os.Stat(filepath.Join(dir, FilesName)); err != nil {
				t.Error(err)
			}
			if left, _ := filepath.Glob(filepath.Join(dir, ".*")); len(left) > 0 {
				t.Errorf("left in the directory: %v", left)
			}
			if runtime.GOOS != "windows" {
				for _, name := range []string{store.FileName, store.KeyFileName, "tls/key.pem"} {
					if fi, err := os.Stat(filepath.Join(dir, name)); err != nil || fi.Mode().Perm() != 0o600 {
						t.Errorf("%s: %v %v", name, fi, err)
					}
				}
			}
			cp, err := store.OpenReadOnly(dir, store.Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer cp.Close()
			in, err := cp.Info(context.Background())
			if err != nil || in.Counts["cubes"] != 1 || in.NodeID != st.NodeID() {
				t.Fatalf("the store opened: %+v %v", in, err)
			}
		})
	}
}

// The file opens with age's own Decrypt, as age -d -i would: the payload
// is a plain age file, its plaintext a zip with the store.
func TestAgeOpensThePayload(t *testing.T) {
	st := testStore(t)
	r, ids := keyOf(t, KindX25519, "")
	out, _ := write(t, st, Inputs{}, r)
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		if f.Method != zip.Store {
			t.Errorf("%s is compressed", f.Name)
		}
	}
	if strings.Join(names, ",") != "manifest.json,payload.age,manifest.sig" {
		t.Fatalf("the archive's files: %v", names)
	}
	rc, err := zr.File[1].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	plain, err := age.Decrypt(rc, ids...)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(plain)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	names = names[:0]
	for _, f := range inner.File {
		names = append(names, f.Name)
		if f.Method != zip.Deflate {
			t.Errorf("%s is not deflated", f.Name)
		}
	}
	if strings.Join(names, ",") != "files.json,massimo.db,opaque.key,node.key" {
		t.Fatalf("the payload's files: %v", names)
	}
}

// Post-quantum and classic recipients do not go into one file; classic
// ones of two kinds do.
func TestMixRefused(t *testing.T) {
	pq, _ := keyOf(t, KindPostQuantum, "pq")
	x, _ := keyOf(t, KindX25519, "x")
	sh, shIDs := keyOf(t, KindSSH, "ssh")
	for _, rs := range [][]Recipient{{pq, x}, {sh, pq}, {pq, pq, x}} {
		if err := CheckKinds(rs); !errors.Is(err, ErrMixedKinds) {
			t.Errorf("%v: %v", rs, err)
		}
	}
	if err := CheckKinds(nil); err == nil {
		t.Error("no recipient")
	}
	if err := CheckKinds([]Recipient{{Kind: KindX25519}}); err == nil {
		t.Error("a recipient not parsed")
	}
	st := testStore(t)
	out := filepath.Join(t.TempDir(), "backup.zip")
	if _, err := Write(context.Background(), Inputs{Store: st, Trigger: TriggerManual}, []Recipient{pq, x}, out); !errors.Is(err, ErrMixedKinds) {
		t.Fatalf("a mixed backup: %v", err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("a mixed backup was written")
	}
	path, m := write(t, st, Inputs{}, x, sh)
	if len(m.Recipients) != 2 {
		t.Fatalf("recipients %+v", m.Recipients)
	}
	if _, _, err := openFile(t, path, shIDs); err != nil {
		t.Fatalf("opened by the second recipient: %v", err)
	}
}

// rebuild writes the backup at path again with change applied to its
// files' contents, each stored with its checksum, as a zip tool would.
func rebuild(t *testing.T, path string, change func(name string, b []byte) []byte) string {
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
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		w.Write(change(f.Name, b))
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

// A changed byte of the payload or of the manifest is refused, whether
// the zip's checksum was mended or not; a newer format asks for an
// update.
func TestChangesRefused(t *testing.T) {
	st := testStore(t)
	r, ids := keyOf(t, KindPostQuantum, "laptop")
	out, m := write(t, st, Inputs{Label: "nightly"}, r)
	b, _ := os.ReadFile(out)
	zr, _ := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	off, err := zr.File[1].DataOffset()
	if err != nil {
		t.Fatal(err)
	}

	// The payload: a byte flipped in place, and one in a mended zip.
	raw := bytes.Clone(b)
	raw[off+m.Payload.Size/2] ^= 1
	rawPath := filepath.Join(t.TempDir(), "raw.zip")
	os.WriteFile(rawPath, raw, 0o600)
	mended := rebuild(t, out, func(name string, c []byte) []byte {
		if name == PayloadName {
			c[len(c)/2] ^= 1
		}
		return c
	})
	for _, p := range []string{rawPath, mended} {
		if _, _, err := readFile(t, p); err != nil {
			t.Errorf("%s: the manifest is unchanged, but: %v", p, err)
		}
		if _, _, err := openFile(t, p, ids); !errors.Is(err, ErrPayloadChanged) {
			t.Errorf("%s: opened with a changed payload: %v", p, err)
		}
		pb, _ := os.ReadFile(p)
		if _, _, err := Verify(bytes.NewReader(pb), int64(len(pb))); !errors.Is(err, ErrPayloadChanged) {
			t.Errorf("%s: verified with a changed payload: %v", p, err)
		}
	}

	// The manifest: a byte of its label flipped in place, and in a
	// mended zip.
	i := bytes.Index(b, []byte(`"nightly"`))
	raw = bytes.Clone(b)
	raw[i+1] = 'N'
	os.WriteFile(rawPath, raw, 0o600)
	if _, _, err := readFile(t, rawPath); err == nil {
		t.Error("a manifest with a broken checksum read")
	}
	if _, _, err := openFile(t, rawPath, ids); err == nil {
		t.Error("a manifest with a broken checksum opened")
	}
	mended = rebuild(t, out, func(name string, c []byte) []byte {
		if name == ManifestName {
			return bytes.Replace(c, []byte(`"nightly"`), []byte(`"Nightly"`), 1)
		}
		return c
	})
	got, sig, err := readFile(t, mended)
	if err != nil || got.Label != "Nightly" || sig.Valid || sig.Verdict(st.NodePublicKey()) != Bad {
		t.Fatalf("a changed manifest: %q %+v %v", got.Label, sig, err)
	}
	if _, _, err := openFile(t, mended, ids); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("opened with a changed manifest: %v", err)
	}

	// Another box's key in the manifest, signed by this box: bad too.
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	swapped := rebuild(t, out, func(name string, c []byte) []byte {
		if name != ManifestName {
			return c
		}
		var mm Manifest
		json.Unmarshal(c, &mm)
		mm.Node = Node{PublicKey: encodeKey(other), Fingerprint: fingerprint(other)}
		c, _ = json.Marshal(mm)
		return c
	})
	if _, sig, err := readFile(t, swapped); err != nil || sig.Valid {
		t.Fatalf("another node key: %+v %v", sig, err)
	}

	newer := rebuild(t, out, func(name string, c []byte) []byte {
		if name == ManifestName {
			return bytes.Replace(c, []byte(`"format": 1`), []byte(`"format": 2`), 1)
		}
		return c
	})
	if _, _, err := readFile(t, newer); !errors.Is(err, ErrNewerFormat) || !strings.Contains(err.Error(), "update first") {
		t.Fatalf("a newer format: %v", err)
	}
	extra := rebuild(t, out, func(name string, c []byte) []byte { return c })
	if _, _, err := readFile(t, extra); err != nil {
		t.Fatalf("the backup rebuilt unchanged: %v", err)
	}
}

// encodeKey is a public key as the manifest writes it.
func encodeKey(k ed25519.PublicKey) string { return base64.StdEncoding.EncodeToString(k) }

// recordingReader records the ranges read from it.
type recordingReader struct {
	r      io.ReaderAt
	ranges [][2]int64
}

// ReadAt reads and records the range.
func (rr *recordingReader) ReadAt(p []byte, off int64) (int, error) {
	n, err := rr.r.ReadAt(p, off)
	rr.ranges = append(rr.ranges, [2]int64{off, off + int64(n)})
	return n, err
}

// The manifest is read without the key and without the payload: the
// reads stay off the payload but for the tail of the zip's last kilobyte,
// so that a target lists its backups by ranged reads.
func TestManifestReadWithoutThePayload(t *testing.T) {
	st := testStore(t)
	r, _ := keyOf(t, KindX25519, "laptop")
	out, m := write(t, st, Inputs{}, r)
	b, _ := os.ReadFile(out)
	zr, _ := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	off, _ := zr.File[1].DataOffset()
	if m.Payload.Size < 8<<10 {
		t.Fatalf("a payload of %d bytes proves nothing", m.Payload.Size)
	}
	rr := &recordingReader{r: bytes.NewReader(b)}
	got, sig, raw, err := ReadManifest(rr, int64(len(b)))
	if err != nil || got.UUID != m.UUID || !sig.Valid {
		t.Fatalf("%+v %+v %v", got, sig, err)
	}
	// The raw manifest is the signed bytes, checked again as kept.
	if !bytes.HasPrefix(raw.Manifest, []byte("{\n  \"format\": 1,")) {
		t.Fatalf("raw manifest %q", raw.Manifest)
	}
	if again, sig2, err := ParseManifest(raw); err != nil || again.UUID != m.UUID || sig2.Verdict(st.NodePublicKey()) != ThisBox {
		t.Fatalf("again: %+v %+v %v", again, sig2, err)
	}
	raw.Manifest = bytes.Replace(raw.Manifest, []byte(`"manual"`), []byte(`"scheduled"`), 1)
	if _, sig2, err := ParseManifest(raw); err != nil || sig2.Valid {
		t.Fatalf("a changed raw manifest: %+v %v", sig2, err)
	}
	for _, rg := range rr.ranges {
		if rg[0] < off+m.Payload.Size-2048 && rg[1] > off {
			t.Errorf("read %d to %d, in the payload's %d to %d", rg[0], rg[1], off, off+m.Payload.Size)
		}
	}
}

// Too little free space is refused before anything is written, with the
// numbers.
func TestTooLittleSpace(t *testing.T) {
	st := testStore(t)
	r, _ := keyOf(t, KindX25519, "")
	freeSpace = func(string) (uint64, error) { return 1000, nil }
	t.Cleanup(func() { freeSpace = diskFree })
	out := filepath.Join(t.TempDir(), "backup.zip")
	_, err := Write(context.Background(), Inputs{Store: st, Trigger: TriggerManual}, []Recipient{r}, out)
	if err == nil || !strings.Contains(err.Error(), "has 1000 bytes (0.0 MiB) free") || !strings.Contains(err.Error(), "twice the store") {
		t.Fatalf("%v", err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("written")
	}
	noTemporaries(t, st, out)
}

// Open refuses the wrong key, naming the recipients, a directory that is
// not empty, and no key; the files it wrote are removed where it fails.
func TestOpenRefusals(t *testing.T) {
	st := testStore(t)
	r, ids := keyOf(t, KindX25519, "laptop")
	_, otherIDs := keyOf(t, KindX25519, "")
	out, _ := write(t, st, Inputs{}, r)
	dir, _, err := openFile(t, out, otherIDs)
	if !errors.Is(err, ErrWrongKey) || !strings.Contains(err.Error(), "laptop ("+r.Fingerprint+")") {
		t.Fatalf("the wrong key: %v", err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("the directory made for a wrong key stays")
	}
	if _, _, err := openFile(t, out, nil); err == nil {
		t.Fatal("no key")
	}
	b, _ := os.ReadFile(out)
	full := t.TempDir()
	os.WriteFile(filepath.Join(full, "x"), nil, 0o600)
	if _, err := Open(bytes.NewReader(b), int64(len(b)), ids, full); err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("a full directory: %v", err)
	}

	// Too little room for the files: nothing is left in an empty
	// directory given.
	empty := t.TempDir()
	freeSpace = func(string) (uint64, error) { return 1000, nil }
	t.Cleanup(func() { freeSpace = diskFree })
	if _, err := Open(bytes.NewReader(b), int64(len(b)), ids, empty); err == nil || !strings.Contains(err.Error(), "1000 bytes") {
		t.Fatalf("too little room: %v", err)
	}
	if left, _ := os.ReadDir(empty); len(left) > 0 {
		t.Fatalf("left: %v", left)
	}
}

// Recipients: the three kinds, their canonical lines and fingerprints;
// a secret key, an RSA key and nonsense refused, never repeated.
func TestParseRecipient(t *testing.T) {
	pq, _ := GenerateKey(KindPostQuantum)
	x, _ := GenerateKey(KindX25519)
	if !strings.HasPrefix(pq.Recipient.Line, "age1pq1") || len(pq.Recipient.Line) < 1900 || !strings.HasPrefix(pq.Identity, "AGE-SECRET-KEY-PQ-1") {
		t.Fatalf("post-quantum %d characters", len(pq.Recipient.Line))
	}
	if !strings.HasPrefix(x.Recipient.Line, "age1") || len(x.Recipient.Line) != 62 || !strings.HasPrefix(x.Identity, "AGE-SECRET-KEY-1") {
		t.Fatalf("X25519 %q", x.Recipient.Line)
	}
	if _, err := GenerateKey(KindSSH); err == nil {
		t.Fatal("an ssh key made")
	}
	again, err := ParseRecipient("  " + x.Recipient.Line + "\r\n")
	if err != nil || again.Line != x.Recipient.Line || again.Fingerprint != x.Recipient.Fingerprint || !strings.HasPrefix(again.Fingerprint, "SHA256:") {
		t.Fatalf("again %+v %v", again, err)
	}
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	spk, _ := ssh.NewPublicKey(pub)
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(spk)))
	sr, err := ParseRecipient(line + " anna@laptop")
	if err != nil || sr.Line != line || sr.Fingerprint != ssh.FingerprintSHA256(spk) || sr.Kind.PostQuantum() {
		t.Fatalf("ssh %+v %v", sr, err)
	}
	for _, bad := range []string{
		x.Identity, pq.Identity, "-----BEGIN OPENSSH PRIVATE KEY-----",
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ anna@laptop", "age1notakey", "age1pq1notakey", "ssh-ed25519 AAAAnotakey", "hello", "",
	} {
		_, err := ParseRecipient(bad)
		if err == nil {
			t.Errorf("%.20q taken", bad)
			continue
		}
		if len(bad) > 8 && strings.Contains(err.Error(), bad[8:]) {
			t.Errorf("the error repeats the line: %v", err)
		}
	}
	if _, err := ParseIdentities([]byte("AGE-SECRET-KEY-1NOTAKEY\n")); err == nil || strings.Contains(err.Error(), "NOTAKEY") {
		t.Fatalf("a bad key file: %v", err)
	}
	if _, err := ParseIdentities([]byte("# nothing\n")); err == nil {
		t.Fatal("an empty key file")
	}
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	blk, _ := ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte("a passphrase"))
	if _, err := ParseIdentities(pem.EncodeToMemory(blk)); err == nil || !strings.Contains(err.Error(), "passphrase") {
		t.Fatalf("a protected SSH key: %v", err)
	}
}

// Write refuses the trigger of an upload, a label with a control
// character or too long, a version that is not printable, a TLS file
// without a TLS role or that is not there; two TLS files of one name are
// two files of the payload.
func TestWriteRefusals(t *testing.T) {
	st := testStore(t)
	r, ids := keyOf(t, KindX25519, "")
	_, tls := sideFiles(t)
	for name, in := range map[string]Inputs{
		"uploaded":       {Trigger: TriggerUploaded},
		"no trigger":     {},
		"control":        {Trigger: TriggerManual, Label: "a\x1b[2Jb"},
		"bidi":           {Trigger: TriggerManual, Label: "a‮b"},
		"long":           {Trigger: TriggerManual, Label: strings.Repeat("ä", MaxLabel+1)},
		"version":        {Trigger: TriggerManual, Version: "v1\n"},
		"not a TLS role": {Trigger: TriggerManual, TLSFiles: []TLSFile{{RoleStore, tls[0].Path}}},
		"no TLS file":    {Trigger: TriggerManual, TLSFiles: []TLSFile{{RoleTLSCert, filepath.Join(t.TempDir(), "gone.pem")}}},
		"a device name":  {Trigger: TriggerManual, TLSFiles: []TLSFile{{RoleTLSCert, filepath.Join(t.TempDir(), "NUL.pem")}}},
	} {
		in.Store = st
		out := filepath.Join(t.TempDir(), "backup.zip")
		if _, err := Write(context.Background(), in, []Recipient{r}, out); err == nil {
			t.Errorf("%s: written", name)
		}
		if _, err := os.Stat(out); err == nil {
			t.Errorf("%s: a file left", name)
		}
		noTemporaries(t, st, out)
	}
	// Two TLS files of one name: the second numbered.
	dup := filepath.Join(t.TempDir(), "cert.pem")
	os.WriteFile(dup, []byte("another certificate\n"), 0o600)
	path, _ := write(t, st, Inputs{TLSFiles: []TLSFile{tls[0], {RoleACMECert, dup}}}, r)
	dir, o, err := openFile(t, path, ids)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "tls", "2-cert.pem")); string(b) != "another certificate\n" || o.Files[len(o.Files)-1].Role != RoleACMECert {
		t.Fatalf("the second cert.pem: %q, %+v", b, o.Files)
	}

	// A recipient's label is in plain too.
	bad := r
	bad.Label = "a\nb"
	if _, err := Write(context.Background(), Inputs{Store: st, Trigger: TriggerManual}, []Recipient{bad}, filepath.Join(t.TempDir(), "backup.zip")); err == nil {
		t.Fatal("a recipient's label with a line break")
	}
	// A missing env file and /dev/null are none.
	_, m := write(t, st, Inputs{EnvFile: filepath.Join(t.TempDir(), "gone")}, r)
	if m.Files.Env {
		t.Fatal("a missing env file backed up")
	}
	if runtime.GOOS != "windows" {
		if _, m := write(t, st, Inputs{EnvFile: os.DevNull}, r); m.Files.Env {
			t.Fatal("/dev/null backed up")
		}
	}
	if _, err := Write(context.Background(), Inputs{Store: st, Trigger: TriggerManual}, []Recipient{r}, filepath.Join(t.TempDir(), "x", "backup.zip")); err == nil {
		t.Fatal("written into a directory that does not exist")
	}
}
