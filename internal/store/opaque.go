// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The sign-in by OPAQUE (RFC 9807; docs/store.md section 6), the users of
// auth_version 2: the client blinds the password, runs the key
// stretching and keeps the password; the server evaluates the blinded
// value under a key of its own for the user and keeps a registration
// record, which holds no hash of anything the client sends. A guess
// against the record needs the server's key file and one run of the
// protocol per guess. The suite is ristretto255-SHA512 with 3DH, the
// configuration of github.com/bytemare/opaque by default and of
// @serenity-kit/opaque in the browser, with the empty context and the
// public keys as identities.

import (
	"context"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bytemare/opaque"
)

// OpaqueKeyFileName is the server's OPAQUE keys in the data directory:
// the OPRF seed and the AKE key pair. Lost, every record of version 2
// is useless and its user needs a password set again.
const OpaqueKeyFileName = "opaque.key"

// opaquePEM is the PEM block type of the key file; its bytes are the
// AKE private key (32 bytes, the public key follows from it) and the
// OPRF seed (64).
const opaquePEM = "MASSIMO OPAQUE KEY"

// OpaqueKSF is the client's key stretching for a new record, as
// opaque_ksf keeps it: argon2id at the parameters of the server's own
// hash (password.go), so that a record costs a guess what a hash does.
var OpaqueKSF = formatKSF(argonMemory, argonTime, argonThreads)

// Errors of the OPAQUE messages.
var (
	// ErrBadMessage is an OPAQUE message or record that does not decode,
	// or key stretching parameters out of bounds.
	ErrBadMessage = errors.New("store: malformed OPAQUE message")
	// ErrNoOpaque is a store without the server's keys: one opened
	// read-only.
	ErrNoOpaque = errors.New("store: no OPAQUE keys: opaque.key is not readable in a store opened read-only")
)

// opaqueConf is the configuration, the same on both sides.
var opaqueConf = opaque.DefaultConfiguration()

// OpaqueKeyPath returns the OPAQUE key file in the data directory dir.
func OpaqueKeyPath(dir string) string { return filepath.Join(dir, OpaqueKeyFileName) }

// credentialID is the credential identifier of a name, from which the
// server derives the user's OPRF key: the login name, which does not
// change, so that a name meets the same key whether it has a record,
// a version 1 hash or no user at all.
func credentialID(name string) []byte { return []byte(NormalName(name)) }

// loadOpaque reads the server's OPAQUE keys, creating the key file
// (mode 0600) when there is none, as loadIdentity does node.key; a store
// opened read-only only reads it, and goes without where it cannot.
func (s *Store) loadOpaque() error {
	srv, err := opaqueConf.Server()
	if err != nil {
		return fmt.Errorf("store: opaque: %w", err)
	}
	p := OpaqueKeyPath(s.dir)
	b, err := os.ReadFile(p)
	switch {
	case err == nil:
		blk, _ := pem.Decode(b)
		if blk == nil || blk.Type != opaquePEM {
			return fmt.Errorf("store: %s: not a PEM %s", p, opaquePEM)
		}
		if err := setOpaqueKey(srv, blk.Bytes); err != nil {
			return fmt.Errorf("store: %s: %w", p, err)
		}
		s.opq = srv
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("store: %w", err)
	case s.readOnly:
		return nil
	}
	sk, _ := opaqueConf.KeyGen()
	key := append(sk.Encode(), opaqueConf.GenerateOPRFSeed()...)
	if err := setOpaqueKey(srv, key); err != nil {
		return fmt.Errorf("store: opaque key: %w", err)
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	if err := pem.Encode(f, &pem.Block{Type: opaquePEM, Bytes: key}); err != nil {
		f.Close()
		os.Remove(p)
		return fmt.Errorf("store: %s: %w", p, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("store: %s: %w", p, err)
	}
	s.opq = srv
	s.log.Info("OPAQUE key created", "path", p)
	return nil
}

// setOpaqueKey gives the server its keys from the key file's bytes.
func setOpaqueKey(srv *opaque.Server, key []byte) error {
	g := opaqueConf.AKE.Group()
	if len(key) != g.ScalarLength()+opaqueConf.Hash.Size() {
		return fmt.Errorf("%d bytes, want %d", len(key), g.ScalarLength()+opaqueConf.Hash.Size())
	}
	sk, err := opaque.DeserializeScalar(g, key[:g.ScalarLength()])
	if err != nil {
		return err
	}
	return srv.SetKeyMaterial(&opaque.ServerKeyMaterial{
		PrivateKey:     sk,
		PublicKeyBytes: g.Base().Multiply(sk).Encode(),
		OPRFGlobalSeed: key[g.ScalarLength():],
	})
}

// formatKSF writes argon2id parameters as opaque_ksf keeps them.
func formatKSF(memory, time uint32, threads uint8) string {
	return fmt.Sprintf("m=%d,t=%d,p=%d", memory, time, threads)
}

// ParseKSF reads opaque_ksf: the memory in KiB, the passes and the
// lanes. It refuses parameters over the ceiling of a stored hash
// (password.go), and below the cost of the server's own: memory times
// passes at least what the current parameters spend.
func ParseKSF(ksf string) (memory, time uint32, threads uint8, err error) {
	bad := fmt.Errorf("%w: key stretching %q, want m=<KiB>,t=<passes>,p=<lanes>", ErrBadMessage, ksf)
	var v [3]uint64
	parts := strings.Split(ksf, ",")
	if len(parts) != 3 {
		return 0, 0, 0, bad
	}
	for i, k := range []string{"m=", "t=", "p="} {
		n, perr := strconv.ParseUint(strings.TrimPrefix(parts[i], k), 10, 32)
		if !strings.HasPrefix(parts[i], k) || perr != nil || n == 0 {
			return 0, 0, 0, bad
		}
		v[i] = n
	}
	switch {
	case v[0] > maxArgonMemory || v[1] > maxArgonTime || v[2] > maxArgonThreads:
		return 0, 0, 0, fmt.Errorf("%w: key stretching %q is over m=%d,t=%d,p=%d", ErrBadMessage, ksf, maxArgonMemory, maxArgonTime, maxArgonThreads)
	case v[0]*v[1] < argonMemory*argonTime:
		return 0, 0, 0, fmt.Errorf("%w: key stretching %q costs less than %s", ErrBadMessage, ksf, OpaqueKSF)
	}
	return uint32(v[0]), uint32(v[1]), uint8(v[2]), nil
}

// OpaqueRegistration answers a client's registration request for a
// name, the first of the two steps that set a password of version 2:
// the blinded password evaluated under the name's OPRF key, and the
// server's public key. Nothing is kept; the client finishes with the
// password and sends the record (SetOpaqueRecord, AddOpaqueUser).
func (s *Store) OpaqueRegistration(name string, request []byte) ([]byte, error) {
	if s.opq == nil {
		return nil, ErrNoOpaque
	}
	req, err := s.opq.Deserialize.RegistrationRequest(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadMessage, err)
	}
	resp, err := s.opq.RegistrationResponse(req, credentialID(name), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadMessage, err)
	}
	return resp.Serialize(), nil
}

// checkRecord validates a registration record and its key stretching.
func (s *Store) checkRecord(record []byte, ksf string) error {
	if s.opq == nil {
		return ErrNoOpaque
	}
	if _, err := s.opq.Deserialize.RegistrationRecord(record); err != nil {
		return fmt.Errorf("%w: %v", ErrBadMessage, err)
	}
	_, _, _, err := ParseKSF(ksf)
	return err
}

// SetOpaqueRecord sets a user's password of version 2: the record and
// its key stretching written, the hash of version 1 emptied. Its
// sessions are the caller's to revoke (RevokeSessions), as with
// SetPassword.
func (s *Store) SetOpaqueRecord(ctx context.Context, name string, record []byte, ksf string, by int64) error {
	if err := s.writable(); err != nil {
		return err
	}
	if err := s.checkRecord(record, ksf); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = '', opaque_record = ?, opaque_ksf = ?, auth_version = 2, updated_at = ?, updated_by = ?, origin = ?, version = version + 1
		WHERE name = ? AND deleted_at IS NULL`, record, ksf, s.nowText(), nullID(by), s.nodeID, NormalName(name))
	if err != nil {
		return fmt.Errorf("store: user %s: %w", name, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %s", ErrNoUser, NormalName(name))
	}
	return nil
}

// opaqueRecordFor runs both halves of a registration in this process, in
// a slot of the store's gate: the client's key stretching as the
// browser's library runs it (opaque-ke: a salt of 16 zero bytes, an
// output as long as the OPRF's, OpaqueKSF's parameters), so that the
// record opens in the browser.
func (s *Store) opaqueRecordFor(ctx context.Context, name, password string) ([]byte, error) {
	if s.opq == nil {
		return nil, ErrNoOpaque
	}
	if password == "" {
		return nil, fmt.Errorf("%w: an empty password", ErrBadMessage)
	}
	leave, err := s.hashes.enter(ctx)
	if err != nil {
		return nil, err
	}
	defer leave()
	c, err := opaqueConf.Client()
	if err != nil {
		return nil, fmt.Errorf("store: opaque: %w", err)
	}
	defer c.ClearState()
	req, err := c.RegistrationInit([]byte(password))
	if err != nil {
		return nil, fmt.Errorf("store: opaque: %w", err)
	}
	resp, err := s.opq.RegistrationResponse(req, credentialID(name), nil)
	if err != nil {
		return nil, fmt.Errorf("store: opaque: %w", err)
	}
	m, t, p, err := ParseKSF(OpaqueKSF)
	if err != nil {
		return nil, err
	}
	rec, _, err := c.RegistrationFinalize(resp, nil, nil, BrowserKSF(m, t, p))
	if err != nil {
		return nil, fmt.Errorf("store: opaque: %w", err)
	}
	return rec.Serialize(), nil
}

// BrowserKSF is the key stretching of @serenity-kit/opaque's client
// (opaque-ke) for bytemare/opaque's: argon2id at the parameters, with a
// salt of 16 zero bytes and an output of 64, which RFC 9807 leaves to
// the implementation; with the library's own defaults a record made in
// Go does not open in the browser.
func BrowserKSF(memory, time uint32, threads uint8) *opaque.ClientOptions {
	return &opaque.ClientOptions{KSFSalt: make([]byte, 16), KSFLength: 64, KSFParameters: []uint64{uint64(time), uint64(memory), uint64(threads)}}
}

// OpaqueStart is a sign-in started (KE1 answered by KE2) that waits for
// the client's KE3; the server keeps it, never the client.
type OpaqueStart struct {
	// KE2 is the answer to send.
	KE2 []byte
	// KSF is the client's key stretching for the record, OpaqueKSF for a
	// name without one.
	KSF string

	userID int64 // 0: a fake record
	mac    []byte
}

// UserID is the user the sign-in is for; 0 when the name had no record.
func (o OpaqueStart) UserID() int64 { return o.userID }

// StartOpaque answers a client's KE1 for a name. A name without a
// record, unknown or of version 1, is answered from a fake record of
// the same shape, so that the answer does not say which names exist;
// its finish fails.
func (s *Store) StartOpaque(ctx context.Context, name string, ke1 []byte) (OpaqueStart, error) {
	if s.opq == nil {
		return OpaqueStart{}, ErrNoOpaque
	}
	k, err := s.opq.Deserialize.KE1(ke1)
	if err != nil {
		return OpaqueStart{}, fmt.Errorf("%w: %v", ErrBadMessage, err)
	}
	var id int64
	var raw []byte
	var ksf string
	err = s.db.QueryRowContext(ctx, `SELECT id, opaque_record, opaque_ksf FROM users WHERE name = ? AND deleted_at IS NULL`, NormalName(name)).Scan(&id, &raw, &ksf)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return OpaqueStart{}, fmt.Errorf("store: user %s: %w", name, err)
	}
	cred := credentialID(name)
	var record *opaque.ClientRecord
	if raw != nil {
		if rec, err := s.opq.Deserialize.RegistrationRecord(raw); err == nil {
			record = &opaque.ClientRecord{RegistrationRecord: rec, CredentialIdentifier: cred}
		} else {
			s.log.Warn("store: OPAQUE record does not decode; the user signs in as one without", "user", id, "error", err)
		}
	}
	if record == nil {
		if record, err = opaqueConf.GetFakeRecord(cred); err != nil {
			return OpaqueStart{}, fmt.Errorf("store: opaque: %w", err)
		}
		id, ksf = 0, OpaqueKSF
	}
	ke2, out, err := s.opq.GenerateKE2(k, record)
	if err != nil {
		return OpaqueStart{}, fmt.Errorf("%w: %v", ErrBadMessage, err)
	}
	return OpaqueStart{KE2: ke2.Serialize(), KSF: ksf, userID: id, mac: out.ClientMAC}, nil
}

// FinishOpaque checks a client's KE3 against a started sign-in;
// ErrBadPassword when it does not match, the password was wrong or the
// name had no record.
func (s *Store) FinishOpaque(start OpaqueStart, ke3 []byte) error {
	if s.opq == nil {
		return ErrNoOpaque
	}
	k, err := s.opq.Deserialize.KE3(ke3)
	if err != nil || len(start.mac) == 0 {
		return ErrBadPassword
	}
	if err := s.opq.LoginFinish(k, start.mac); err != nil || start.userID == 0 {
		return ErrBadPassword
	}
	return nil
}
