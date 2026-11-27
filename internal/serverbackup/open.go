// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"github.com/google/uuid"

	"go.metaworx.ch/massimo/internal/store"
)

// The errors of a backup that is refused.
var (
	// ErrNotABackup is a file that is not a server backup, or one
	// whose structure or text is broken.
	ErrNotABackup = errors.New("serverbackup: not a server backup")
	// ErrNewerFormat is a backup of a format or kind that a newer
	// Massimo writes.
	ErrNewerFormat = errors.New("serverbackup: the backup is of a newer format than this Massimo reads; update first")
	// ErrBadSignature is a manifest that its signature does not
	// verify: it was changed since it was made.
	ErrBadSignature = errors.New("serverbackup: the manifest's signature does not verify; the file was changed")
	// ErrPayloadChanged is a payload whose size or hash is not the
	// manifest's.
	ErrPayloadChanged = errors.New("serverbackup: payload.age is not the one the manifest names; the file was changed or cut short")
	// ErrWrongKey is a payload that none of the keys given opens.
	ErrWrongKey = errors.New("serverbackup: none of the keys given opens this backup")
)

// Limits of what is read whole: before the signature is checked, and
// files.json.
const (
	maxManifest   = 1 << 20
	maxSignature  = 1 << 10
	maxFilesJSON  = 1 << 20
	maxRecipients = 64
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

// Raw is a manifest as the file holds it: manifest.json's bytes, which
// are signed, and manifest.sig's, the signature in base64.
type Raw struct {
	Manifest  []byte
	Signature []byte
}

// archive is a backup's outer zip, read.
type archive struct {
	raw     Raw
	m       Manifest
	sig     Signature
	payload *zip.File
}

// ReadManifest reads a backup's manifest from r, size bytes, checks its
// text and its signature, without the key and without reading the
// payload: the zip's central directory, manifest.json and manifest.sig,
// each once. A manifest that does not verify is returned with the
// verdict Bad; a file that is no server backup, or whose manifest's text
// is not one Write writes, is ErrNotABackup; one of a newer format
// ErrNewerFormat. Raw is the manifest as signed, to be kept and checked
// again with ParseManifest.
func ReadManifest(r io.ReaderAt, size int64) (Manifest, Signature, Raw, error) {
	a, err := readArchive(r, size)
	if err != nil {
		return Manifest{}, Signature{}, Raw{}, err
	}
	return a.m, a.sig, a.raw, nil
}

// ParseManifest checks a manifest kept as ReadManifest returned it, as
// ReadManifest does: its text and its signature.
func ParseManifest(raw Raw) (Manifest, Signature, error) {
	return parseManifest(raw)
}

// Verify reads the manifest as ReadManifest does and checks payload.age
// against it, its size and its SHA-256, without the key: a whole read of
// the file. A bad signature is ErrBadSignature, a payload that differs
// ErrPayloadChanged.
func Verify(r io.ReaderAt, size int64) (Manifest, Signature, error) {
	a, err := readArchive(r, size)
	if err != nil {
		return Manifest{}, Signature{}, err
	}
	if !a.sig.Valid {
		return Manifest{}, Signature{}, ErrBadSignature
	}
	pr, err := a.payload.Open()
	if err != nil {
		return Manifest{}, Signature{}, fmt.Errorf("%w: %w", ErrNotABackup, err)
	}
	defer pr.Close()
	if err := copyHashed(io.Discard, pr, a.m.Payload); err != nil {
		return Manifest{}, Signature{}, err
	}
	return a.m, a.sig, nil
}

// copyHashed copies the payload from r to w, which must be its size and
// hash; ErrPayloadChanged where they are not. The zip's reader refuses
// a checksum that does not match, too.
func copyHashed(w io.Writer, r io.Reader, p Payload) error {
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(w, h), io.LimitReader(r, p.Size+1))
	if err != nil || n != p.Size || hex.EncodeToString(h.Sum(nil)) != p.SHA256 {
		return ErrPayloadChanged
	}
	return nil
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
			return a, fmt.Errorf("%w: it holds a file %s", ErrNotABackup, quoted(f.Name))
		}
		if *slot != nil {
			return a, fmt.Errorf("%w: %s twice", ErrNotABackup, f.Name)
		}
		*slot = f
	}
	if mf == nil || sf == nil || a.payload == nil {
		return a, fmt.Errorf("%w: it lacks %s, %s or %s", ErrNotABackup, ManifestName, PayloadName, SignatureName)
	}
	if a.raw.Manifest, err = readSmall(mf, maxManifest); err != nil {
		return a, err
	}
	if a.raw.Signature, err = readSmall(sf, maxSignature); err != nil {
		return a, err
	}
	if a.m, a.sig, err = parseManifest(a.raw); err != nil {
		return a, err
	}
	if a.payload.Method != zip.Store || a.payload.UncompressedSize64 != uint64(a.m.Payload.Size) {
		// The manifest is signed; the zip's directory is not.
		if a.sig.Valid {
			return a, ErrPayloadChanged
		}
	}
	return a, nil
}

// quoted is s for an error: Go's quoting, at most 64 bytes of it, so
// that no control character of a file reaches a terminal.
func quoted(s string) string {
	if len(s) > 64 {
		s = s[:64] + "…"
	}
	return fmt.Sprintf("%q", s)
}

// parseManifest parses the manifest, checks its text and its signature.
func parseManifest(raw Raw) (Manifest, Signature, error) {
	var m Manifest
	if err := json.Unmarshal(raw.Manifest, &m); err != nil {
		return Manifest{}, Signature{}, fmt.Errorf("%w: %s: %w", ErrNotABackup, ManifestName, err)
	}
	switch {
	case m.Format > Format:
		return Manifest{}, Signature{}, fmt.Errorf("%w (format %d; this one reads %d)", ErrNewerFormat, m.Format, Format)
	case m.Format < 1:
		return Manifest{}, Signature{}, fmt.Errorf("%w: %s names no format", ErrNotABackup, ManifestName)
	case m.Kind != KindStoreSnapshot && validWord(m.Kind):
		return Manifest{}, Signature{}, fmt.Errorf("%w (kind %s)", ErrNewerFormat, m.Kind)
	}
	if err := checkManifest(m); err != nil {
		return Manifest{}, Signature{}, fmt.Errorf("%w: %s: %s", ErrNotABackup, ManifestName, err)
	}
	return m, signature(raw, m.Node), nil
}

// validWord reports whether s is 1 to 32 lower-case letters and hyphens.
func validWord(s string) bool {
	return s != "" && len(s) <= 32 && strings.Trim(s, "abcdefghijklmnopqrstuvwxyz-") == ""
}

// checkManifest refuses a manifest whose text is not one Write writes,
// so that nothing of it is shown or kept unchecked.
func checkManifest(m Manifest) error {
	if _, err := uuid.Parse(m.UUID); err != nil || len(m.UUID) != 36 {
		return errors.New("its uuid is not one")
	}
	c := m.Counts
	switch {
	case m.Kind != KindStoreSnapshot:
		return errors.New("its kind is not one")
	case m.Created.IsZero():
		return errors.New("it names no time")
	case !m.Trigger.Valid() || m.Trigger == TriggerUploaded:
		return errors.New("its trigger is not one")
	case CheckLabel(m.Label) != nil:
		return errors.New("its label is not one Write takes")
	case !validVersion(m.Version):
		return errors.New("its version is not printable ASCII of at most 128 bytes")
	case m.Schema.Version < 1 || m.Schema.MinReader < 0:
		return errors.New("its schema version is not one")
	case !validFingerprint(m.Node.Fingerprint):
		return errors.New("its node's fingerprint is not one")
	case c.Users < 0 || c.Integrations < 0 || c.Cubes < 0 || c.Rooms < 0 || c.Audit < 0 || c.Log < 0:
		return errors.New("a count is below 0")
	case m.Files.TLS < 0 || m.Files.TLS > maxTLSFiles:
		return errors.New("its count of TLS files is not one")
	case len(m.Recipients) == 0 || len(m.Recipients) > maxRecipients:
		return fmt.Errorf("it names %d recipients", len(m.Recipients))
	case m.Payload.Size < 0 || len(m.Payload.SHA256) != 64 || strings.Trim(m.Payload.SHA256, "0123456789abcdef") != "":
		return errors.New("its payload's size or hash is not one")
	}
	for _, r := range m.Recipients {
		switch {
		case CheckLabel(r.Label) != nil:
			return errors.New("a recipient's label is not one Write takes")
		case r.Kind != KindPostQuantum && r.Kind != KindX25519 && r.Kind != KindSSH:
			return errors.New("a recipient's kind is not one")
		case r.Kind.PostQuantum() != m.Recipients[0].Kind.PostQuantum():
			return errors.New("its recipients mix post-quantum and classic")
		case !validFingerprint(r.Fingerprint):
			return errors.New("a recipient's fingerprint is not one")
		}
	}
	return nil
}

// validFingerprint reports whether s is one fingerprint makes: SHA256:
// and 43 characters of unpadded base64.
func validFingerprint(s string) bool {
	b, ok := strings.CutPrefix(s, "SHA256:")
	if !ok || len(b) != 43 {
		return false
	}
	raw, err := base64.RawStdEncoding.DecodeString(b)
	return err == nil && len(raw) == sha256.Size
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
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrNotABackup, f.Name, err)
	}
	if len(b) > limit {
		return nil, fmt.Errorf("%w: %s is too long", ErrNotABackup, f.Name)
	}
	return b, nil
}

// signature checks manifest.sig over the manifest's bytes with the node
// key the manifest names.
func signature(raw Raw, n Node) Signature {
	pub, err := base64.StdEncoding.DecodeString(n.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return Signature{Fingerprint: n.Fingerprint}
	}
	s := Signature{Fingerprint: fingerprint(pub), publicKey: ed25519.PublicKey(pub)}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw.Signature)))
	s.Valid = err == nil && n.Fingerprint == s.Fingerprint && store.VerifyNode(raw.Manifest, sig, s.publicKey)
	return s
}

// Opened is what Open read and wrote.
type Opened struct {
	Manifest  Manifest
	Signature Signature
	// Files is files.json: each file written, with its role and where
	// it was on the box that made the backup.
	Files []InnerFile
}

// Open opens the backup in r, size bytes, with the identities
// (ParseIdentities) into dir, which must be empty or not exist. A bad
// signature is refused (ErrBadSignature); a backup of another box opens,
// and Opened.Signature says which (Verdict). The payload is read from r
// once, copied into dir while it is hashed, and decrypted from that copy
// alone, so that what is opened is what was hashed. Its files are
// checked against files.json, every name for every system, and written
// each with mode 0600, files.json too; nothing of the payload is held
// whole in memory. Where it fails, what it wrote is removed, and dir
// where Open made it; the copy is removed in any case.
func Open(r io.ReaderAt, size int64, identities []age.Identity, dir string) (o Opened, err error) {
	if len(identities) == 0 {
		return Opened{}, errors.New("serverbackup: no key to open the backup with")
	}
	a, err := readArchive(r, size)
	if err != nil {
		return Opened{}, err
	}
	if !a.sig.Valid {
		return Opened{}, ErrBadSignature
	}
	made, err := emptyDir(dir)
	if err != nil {
		return Opened{}, err
	}
	// written are the files and the directory made, in their order,
	// removed where a later step fails.
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

	cp, err := copyPayload(a, dir)
	if err != nil {
		return Opened{}, err
	}
	defer func() {
		cp.Close()
		os.Remove(cp.Name())
	}()
	plain, plainSize, err := age.DecryptReaderAt(cp, a.m.Payload.Size, identities...)
	var nomatch *age.NoIdentityMatchError
	switch {
	case errors.As(err, &nomatch):
		return Opened{}, fmt.Errorf("%w; it was made for %s", ErrWrongKey, recipientsText(a.m.Recipients))
	case err != nil:
		return Opened{}, fmt.Errorf("serverbackup: decrypt: %w", err)
	}
	zr, err := zip.NewReader(plain, plainSize)
	if err != nil {
		return Opened{}, fmt.Errorf("%w: the payload: %w", ErrNotABackup, err)
	}
	list, err := checkPayload(zr)
	if err != nil {
		return Opened{}, err
	}
	if err := extract(zr, dir, &written); err != nil {
		return Opened{}, err
	}
	return Opened{Manifest: a.m, Signature: a.sig, Files: list.Files}, nil
}

// emptyDir makes sure dir is empty, making it where it does not exist,
// and reports whether it made it.
func emptyDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return false, fmt.Errorf("serverbackup: %w", err)
		}
		return true, nil
	case err != nil:
		return false, fmt.Errorf("serverbackup: %w", err)
	case len(entries) > 0:
		return false, fmt.Errorf("serverbackup: %s is not empty", dir)
	}
	return false, nil
}

// copyPayload copies payload.age into a hidden temporary file in dir,
// checking its size and hash as it reads it once, and returns the copy,
// open.
func copyPayload(a archive, dir string) (*os.File, error) {
	if free, err := freeSpace(dir); err == nil && free < uint64(a.m.Payload.Size) {
		return nil, fmt.Errorf("serverbackup: %s has %s free; the backup's payload needs %s", dir, bytesText(int64(free)), bytesText(a.m.Payload.Size))
	}
	pr, err := a.payload.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotABackup, err)
	}
	defer pr.Close()
	f, err := os.CreateTemp(dir, ".payload-*.age")
	if err != nil {
		return nil, fmt.Errorf("serverbackup: %w", err)
	}
	if err := copyHashed(f, pr, a.m.Payload); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}

// checkPayload checks the inner zip's names, each once and one a
// payload may hold, and its files.json against them, and returns it.
func checkPayload(zr *zip.Reader) (filesList, error) {
	if len(zr.File) > maxTLSFiles+5 {
		return filesList{}, fmt.Errorf("%w: the payload holds %d files", ErrNotABackup, len(zr.File))
	}
	inZip := map[string]bool{}
	var listFile *zip.File
	for _, f := range zr.File {
		if _, ok := innerPath(".", f.Name); !ok || inZip[f.Name] {
			return filesList{}, fmt.Errorf("%w: the payload holds %s, which it may not", ErrNotABackup, quoted(f.Name))
		}
		inZip[f.Name] = true
		if f.Name == FilesName {
			listFile = f
		}
	}
	if listFile == nil {
		return filesList{}, fmt.Errorf("%w: the payload holds no %s", ErrNotABackup, FilesName)
	}
	b, err := readSmall(listFile, maxFilesJSON)
	if err != nil {
		return filesList{}, err
	}
	list, err := parseFiles(b)
	if err != nil {
		return filesList{}, err
	}
	return list, checkFiles(list, inZip)
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

// extract writes the payload's files, their names checked, into dir,
// after checking that their sizes, each alone and all together, fit the
// free space; written takes each file and directory made.
func extract(zr *zip.Reader, dir string, written *[]string) error {
	free, ferr := freeSpace(dir)
	var total uint64
	for _, f := range zr.File {
		n := f.UncompressedSize64
		switch {
		case n > math.MaxInt64 || total+n < total:
			return fmt.Errorf("%w: the payload's sizes are not ones", ErrNotABackup)
		case ferr == nil && n > free:
			return fmt.Errorf("serverbackup: %s has %s free; the backup's %s alone needs %s", dir, bytesText(int64(free)), f.Name, bytesText(int64(n)))
		}
		total += n
	}
	if ferr == nil && total > free {
		return fmt.Errorf("serverbackup: %s has %s free; the backup's files need %s", dir, bytesText(int64(free)), bytesText(int64(total)))
	}
	tls := filepath.Join(dir, TLSDir)
	tlsMade := false
	for _, f := range zr.File {
		p, ok := innerPath(dir, f.Name)
		if !ok {
			return fmt.Errorf("%w: the payload holds %s, which it may not", ErrNotABackup, quoted(f.Name))
		}
		if filepath.Dir(p) == tls && !tlsMade {
			if err := os.Mkdir(tls, 0o700); err != nil {
				return fmt.Errorf("serverbackup: %w", err)
			}
			tlsMade = true
			*written = append(*written, tls)
		}
		if err := extractFile(f, p); err != nil {
			return err
		}
		*written = append(*written, p)
	}
	return nil
}

// extractFile writes the zip's file f at p, which must not exist, mode
// 0600: no more than its size, and refused where it holds more or fewer
// bytes or its checksum does not match. Where it fails after making p,
// p is removed.
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
	size := int64(f.UncompressedSize64)
	n, err := io.Copy(out, io.LimitReader(rc, size))
	if err == nil && n != size {
		err = io.ErrUnexpectedEOF
	}
	if err == nil {
		// The end, where the zip's reader checks the checksum; a byte
		// more is a file longer than its size.
		var one [1]byte
		if k, rerr := io.ReadAtLeast(rc, one[:], 1); k > 0 {
			err = errors.New("longer than its size")
		} else if !errors.Is(rerr, io.EOF) {
			err = rerr
		}
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(p)
		return fmt.Errorf("serverbackup: %s: %w", f.Name, err)
	}
	return nil
}
