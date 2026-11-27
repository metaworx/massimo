// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

import (
	"archive/zip"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"

	"go.metaworx.ch/massimo/internal/store"
)

// The errors of a backup that is refused.
var (
	// ErrNotABackup is a file that is not a server backup, or one
	// whose structure is broken.
	ErrNotABackup = errors.New("serverbackup: not a server backup")
	// ErrBadSignature is a manifest that its signature does not
	// verify: it was changed since it was made.
	ErrBadSignature = errors.New("serverbackup: the manifest's signature does not verify; the file was changed")
	// ErrPayloadChanged is a payload whose size or hash is not the
	// manifest's.
	ErrPayloadChanged = errors.New("serverbackup: payload.age is not the one the manifest names; the file was changed or cut short")
	// ErrWrongKey is a payload that none of the keys given opens.
	ErrWrongKey = errors.New("serverbackup: none of the keys given opens this backup")
)

// Limits of what is read before the signature is checked.
const (
	maxManifest  = 1 << 20
	maxSignature = 1 << 10
)

// Verdict is who made a backup, by its signature.
type Verdict int

// The verdicts.
const (
	// Bad is a signature that does not verify.
	Bad Verdict = iota
	// ThisBox is a backup signed by this box's node key.
	ThisBox
	// OtherBox is a backup signed by another box's, whose fingerprint
	// the signature names.
	OtherBox
)

// String names the verdict.
func (v Verdict) String() string {
	switch v {
	case ThisBox:
		return "this box"
	case OtherBox:
		return "another box"
	}
	return "bad"
}

// Signature is what manifest.sig says.
type Signature struct {
	// Valid is a signature that verifies with the node key the
	// manifest names, whose fingerprint is the manifest's too.
	Valid bool
	// Fingerprint is that node key's.
	Fingerprint string

	publicKey ed25519.PublicKey
}

// Verdict says who made the backup: this box where its node key's
// public part is local (nil where unknown), another box, or Bad.
func (s Signature) Verdict(local ed25519.PublicKey) Verdict {
	switch {
	case !s.Valid:
		return Bad
	case local != nil && s.publicKey.Equal(local):
		return ThisBox
	}
	return OtherBox
}

// archive is a backup's outer zip, read.
type archive struct {
	manifest []byte
	m        Manifest
	sig      Signature
	payload  *zip.File
}

// ReadManifest reads a backup's manifest from r, size bytes, and checks
// its signature, without the key and without reading the payload: the
// zip's central directory, manifest.json and manifest.sig. A manifest
// that does not verify is returned with the verdict Bad; a file that is
// no server backup, or of a newer format, is an error.
func ReadManifest(r io.ReaderAt, size int64) (Manifest, Signature, error) {
	a, err := readArchive(r, size)
	if err != nil {
		return Manifest{}, Signature{}, err
	}
	return a.m, a.sig, nil
}

// Verify reads the manifest as ReadManifest does and checks payload.age
// against it, its size and its SHA-256, without the key: a whole read of
// the file. A bad signature is ErrBadSignature, a payload that differs
// ErrPayloadChanged.
func Verify(r io.ReaderAt, size int64) (Manifest, Signature, error) {
	a, err := verify(r, size)
	if err != nil {
		return Manifest{}, Signature{}, err
	}
	return a.m, a.sig, nil
}

// verify is Verify, returning the archive.
func verify(r io.ReaderAt, size int64) (archive, error) {
	a, err := readArchive(r, size)
	if err != nil {
		return a, err
	}
	if !a.sig.Valid {
		return a, ErrBadSignature
	}
	pr, err := a.payload.Open()
	if err != nil {
		return a, fmt.Errorf("%w: %w", ErrNotABackup, err)
	}
	defer pr.Close()
	sum, err := hashOf(pr, a.m.Payload.Size)
	if err != nil || sum != a.m.Payload.SHA256 {
		return a, ErrPayloadChanged
	}
	return a, nil
}

// readArchive reads the outer zip's directory, the manifest and the
// signature.
func readArchive(r io.ReaderAt, size int64) (archive, error) {
	var a archive
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return a, fmt.Errorf("%w: %w", ErrNotABackup, err)
	}
	var mf, sf *zip.File
	for _, f := range zr.File {
		var slot **zip.File
		switch f.Name {
		case ManifestName:
			slot = &mf
		case PayloadName:
			slot = &a.payload
		case SignatureName:
			slot = &sf
		default:
			return a, fmt.Errorf("%w: it holds %q", ErrNotABackup, f.Name)
		}
		if *slot != nil {
			return a, fmt.Errorf("%w: %s twice", ErrNotABackup, f.Name)
		}
		*slot = f
	}
	if mf == nil || sf == nil || a.payload == nil {
		return a, fmt.Errorf("%w: it lacks %s, %s or %s", ErrNotABackup, ManifestName, PayloadName, SignatureName)
	}
	if a.manifest, err = readSmall(mf, maxManifest); err != nil {
		return a, err
	}
	sb, err := readSmall(sf, maxSignature)
	if err != nil {
		return a, err
	}
	if err := json.Unmarshal(a.manifest, &a.m); err != nil {
		return a, fmt.Errorf("%w: %s: %w", ErrNotABackup, ManifestName, err)
	}
	switch {
	case a.m.Format > Format:
		return a, fmt.Errorf("serverbackup: the backup is of format %d, newer than this Massimo reads (%d); update first", a.m.Format, Format)
	case a.m.Format < 1:
		return a, fmt.Errorf("%w: %s names no format", ErrNotABackup, ManifestName)
	case a.m.Kind != KindStoreSnapshot:
		return a, fmt.Errorf("serverbackup: a backup of kind %q, which this Massimo does not read", a.m.Kind)
	}
	a.sig = signature(a.manifest, sb, a.m.Node)
	if a.payload.Method != zip.Store || a.payload.UncompressedSize64 != uint64(a.m.Payload.Size) {
		// The manifest is signed; the zip's directory is not.
		if a.sig.Valid {
			return a, ErrPayloadChanged
		}
	}
	return a, nil
}

// readSmall reads the zip's file f, of at most limit bytes.
func readSmall(f *zip.File, limit int) ([]byte, error) {
	if f.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("%w: %s is too long", ErrNotABackup, f.Name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrNotABackup, f.Name, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, int64(limit)+1))
	if err != nil || len(b) > limit {
		return nil, fmt.Errorf("%w: %s: %v", ErrNotABackup, f.Name, err)
	}
	return b, nil
}

// signature checks sig, manifest.sig, over the manifest's bytes with the
// node key the manifest names.
func signature(manifest, sig []byte, n Node) Signature {
	pub, err := base64.StdEncoding.DecodeString(n.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return Signature{Fingerprint: n.Fingerprint}
	}
	s := Signature{Fingerprint: fingerprint(pub), publicKey: ed25519.PublicKey(pub)}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	s.Valid = err == nil && n.Fingerprint == s.Fingerprint && store.VerifyNode(manifest, raw, s.publicKey)
	return s
}

// Open opens the backup in r, size bytes, with the identities
// (ParseIdentities): the signature and the payload's hash are checked
// first, then the payload is decrypted and its files written into dir,
// which must be empty or not exist, each with mode 0600. Nothing of the
// payload is held whole in memory. Where it fails, what it wrote is
// removed.
func Open(r io.ReaderAt, size int64, identities []age.Identity, dir string) (Manifest, error) {
	if len(identities) == 0 {
		return Manifest{}, errors.New("serverbackup: no key to open the backup with")
	}
	a, err := verify(r, size)
	if err != nil {
		return Manifest{}, err
	}
	off, err := a.payload.DataOffset()
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrNotABackup, err)
	}
	plain, plainSize, err := age.DecryptReaderAt(io.NewSectionReader(r, off, a.m.Payload.Size), a.m.Payload.Size, identities...)
	var nomatch *age.NoIdentityMatchError
	switch {
	case errors.As(err, &nomatch):
		return Manifest{}, fmt.Errorf("%w; it was made for %s", ErrWrongKey, recipientsText(a.m.Recipients))
	case err != nil:
		return Manifest{}, fmt.Errorf("serverbackup: decrypt: %w", err)
	}
	zr, err := zip.NewReader(plain, plainSize)
	if err != nil {
		return Manifest{}, fmt.Errorf("serverbackup: the payload: %w", err)
	}
	if err := extract(zr, dir); err != nil {
		return Manifest{}, err
	}
	return a.m, nil
}

// recipientsText names the recipients for an error.
func recipientsText(rs []RecipientInfo) string {
	var parts []string
	for _, r := range rs {
		label := r.Label
		if label == "" {
			label = string(r.Kind)
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", label, r.Fingerprint))
	}
	if len(parts) == 0 {
		return "no recipient it names"
	}
	return strings.Join(parts, ", ")
}

// validTLSName reports whether name is a file name tls/ may hold: one
// element, not hidden, no separator of any system.
func validTLSName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.HasPrefix(name, ".") && !strings.ContainsAny(name, `/\:`) && len(name) <= 255
}

// innerPath returns where the payload's file name goes under dir, or
// false for a name the payload may not hold.
func innerPath(dir, name string) (string, bool) {
	switch name {
	case store.FileName, store.OpaqueKeyFileName, store.KeyFileName, EnvName:
		return filepath.Join(dir, name), true
	}
	if t, ok := strings.CutPrefix(name, TLSDir+"/"); ok && validTLSName(t) {
		return filepath.Join(dir, TLSDir, t), true
	}
	return "", false
}

// extract writes the payload's files into dir, after checking every
// name and that their sizes fit the free space.
func extract(zr *zip.Reader, dir string) (err error) {
	made := false
	switch entries, err := os.ReadDir(dir); {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("serverbackup: %w", err)
		}
		made = true
	case err != nil:
		return fmt.Errorf("serverbackup: %w", err)
	case len(entries) > 0:
		return fmt.Errorf("serverbackup: %s is not empty", dir)
	}
	// written are the files and the directory made, in their order,
	// removed where a later one fails.
	var written []string
	defer func() {
		switch {
		case err == nil:
		case made:
			os.RemoveAll(dir)
		default:
			for i := len(written) - 1; i >= 0; i-- {
				os.Remove(written[i])
			}
		}
	}()

	if len(zr.File) > maxTLSFiles+4 {
		return fmt.Errorf("serverbackup: the payload holds %d files", len(zr.File))
	}
	seen := map[string]bool{}
	var total uint64
	for _, f := range zr.File {
		if _, ok := innerPath(dir, f.Name); !ok || seen[f.Name] {
			return fmt.Errorf("serverbackup: the payload holds %q, which it may not", f.Name)
		}
		seen[f.Name] = true
		total += f.UncompressedSize64
	}
	if !seen[store.FileName] {
		return fmt.Errorf("serverbackup: the payload holds no %s", store.FileName)
	}
	if free, err := freeSpace(dir); err == nil && free < total {
		return fmt.Errorf("serverbackup: %s has %s free; the backup's files need %s", dir, bytesText(int64(free)), bytesText(int64(total)))
	}
	tls := filepath.Join(dir, TLSDir)
	for _, f := range zr.File {
		p, _ := innerPath(dir, f.Name)
		if filepath.Dir(p) == tls && !seen[TLSDir+"/"] {
			if err := os.Mkdir(tls, 0o700); err != nil {
				return fmt.Errorf("serverbackup: %w", err)
			}
			seen[TLSDir+"/"] = true
			written = append(written, tls)
		}
		if err := extractFile(f, p); err != nil {
			return err
		}
		written = append(written, p)
	}
	return nil
}

// extractFile writes the zip's file f at p, which must not exist, mode
// 0600; where it fails after making p, p is removed.
func extractFile(f *zip.File, p string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("serverbackup: %s: %w", f.Name, err)
	}
	defer rc.Close()
	out, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("serverbackup: %w", err)
	}
	// The zip's reader refuses more or fewer bytes than the file's
	// size, and a wrong checksum.
	_, err = io.Copy(out, rc)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(p)
		return fmt.Errorf("serverbackup: %s: %w", f.Name, err)
	}
	return nil
}
