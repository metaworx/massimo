// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The restore of a server backup by scope (AP ServerBackup v1.0, Block 5,
// D6-A and D7-A; docs/store.md, "Restore by scope"): the backup opened
// with the owner's key, held in memory alone, into a temporary directory
// in the data directory; its store migrated there where older, refused
// where newer; a pinned local backup taken first; each scope replaced
// whole in one transaction (internal/store, Restore); opaque.key with
// users and sign-in, node.key with the identity, the env file and the
// TLS files where asked for, each file replaced kept beside as
// <name>.before-restore. Plan answers what a restore would do, and
// CheckKey opens a backup and checks its store, changing nothing. Who
// may restore is the caller's to decide: the owner's routes, or a new
// box's first-owner window (Block 11).

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.metaworx.ch/massimo/internal/safefile"
	"go.metaworx.ch/massimo/internal/store"
)

// ErrNewerSchema is a backup whose store is of a newer schema than this
// server's: it is restored once this server is updated (D6-A).
var ErrNewerSchema = errors.New("serverbackup: the backup's store is newer than this server's; update first")

// BeforeRestoreSuffix is added to the name of a file a restore replaced,
// which stays beside.
const BeforeRestoreSuffix = ".before-restore"

// maxRestoredFile bounds a file a restore puts in place: a key file, the
// env file or a TLS file, read whole.
const maxRestoredFile = 1 << 20

// TLSPlace is where a restore puts a backup's TLS files on this box: the
// certificate and its key (files mode's, or the self-signed pair's), and
// the ACME directory, which takes its files by their names.
type TLSPlace struct {
	Cert, Key string
	ACMEDir   string
}

// RestoreOptions ask for a restore, its plan or a check with the key.
type RestoreOptions struct {
	// Backup is a backup of the catalogue, by its uuid, read from a
	// target that holds it; File a backup file. One of the two.
	Backup string
	File   string
	// Key is the key file's text, an age identity or an OpenSSH
	// Ed25519 key (ParseIdentities): held in memory for this call
	// alone, never written (D4-A).
	Key []byte
	// Scopes are the scopes (ParseScopes); a plan with none plans
	// every one.
	Scopes []string
	// Identity takes the backup's node.key and node id; without, this
	// box keeps its own (D7-A).
	Identity bool
	// Env puts the backup's env file at the catalogue's EnvFile, and
	// TLS its TLS files at TLSPlace, each on its own, since a new box's
	// paths can differ.
	Env      bool
	TLS      bool
	TLSPlace TLSPlace
	// UserID is who restores, for the backup taken first.
	UserID int64
	// NoBackupFirst skips the pinned local backup every restore takes
	// first (D6-A): for a new box started from a backup (Block 11),
	// which has nothing to keep and no key to encrypt it for.
	NoBackupFirst bool
}

// PlannedFile is a file of the backup a restore would put in place, or
// put: its role, its name in the payload, and where it goes on this box;
// Before is where the file it replaced stays, "" where there was none or
// it was the same.
type PlannedFile struct {
	Role   Role   `json:"role"`
	Name   string `json:"name"`
	To     string `json:"to"`
	Before string `json:"before,omitempty"`
}

// PlanCube is a Cube the backup holds, and whether this store holds it.
type PlanCube struct {
	Serial string `json:"serial"`
	Name   string `json:"name"`
	Live   bool   `json:"live"`
}

// RestorePlan is what a restore would do.
type RestorePlan struct {
	Manifest Manifest `json:"manifest"`
	// ThisBox is a backup this box signed; NodeID is the backup's node
	// id, which a box that still answers on the network announces.
	ThisBox bool   `json:"this_box"`
	NodeID  string `json:"node_id"`
	// Migrated is a backup of an older schema, brought to this one's.
	Migrated  bool      `json:"migrated"`
	Selection Selection `json:"selection"`
	// Cubes are every Cube the backup holds.
	Cubes []PlanCube `json:"cubes"`
	// Env is a backup that holds an env file, Files the files the
	// options would put in place.
	Env   bool          `json:"env"`
	TLS   int           `json:"tls"`
	Files []PlannedFile `json:"files"`
	// Report is what the selection would do of the store; Refused why
	// the restore would be refused, "" for not.
	Report  store.RestoreReport `json:"report"`
	Refused string              `json:"refused,omitempty"`
}

// Restored is what a restore did.
type Restored struct {
	// Before is the pinned local backup taken first, and Steps what its
	// making did; zero with NoBackupFirst.
	Before    store.ServerBackup `json:"-"`
	Steps     []Outcome          `json:"-"`
	Selection Selection          `json:"selection"`
	Report    store.RestoreReport
	// Files are the files put in place.
	Files []PlannedFile `json:"files"`
}

// KeyCheck is what CheckKey found: the backup opened with the key, its
// files, and its store's PRAGMA integrity_check, "ok" where sound.
type KeyCheck struct {
	Manifest  Manifest    `json:"manifest"`
	ThisBox   bool        `json:"this_box"`
	Files     []InnerFile `json:"files"`
	Integrity []string    `json:"integrity"`
	OK        bool        `json:"ok"`
}

// openedBackup is a backup opened into a temporary directory.
type openedBackup struct {
	tmp   string // removed by close: it holds the store in plain
	dir   string // the payload's files, the backup's data directory
	m     Manifest
	sig   Signature
	files []InnerFile
	// migrated, nodeID and cubes are the store's, once opened.
	migrated bool
	nodeID   string
	cubes    []store.Cube
}

func (o *openedBackup) close() { os.RemoveAll(o.tmp) }

// file is the payload's file of a role, the first.
func (o *openedBackup) file(role Role) (InnerFile, bool) {
	for _, f := range o.files {
		if f.Role == role {
			return f, true
		}
	}
	return InnerFile{}, false
}

// openBackup opens the backup the options name with their key into a
// temporary directory in the data directory, a backup of a newer schema
// than the store's refused first; with migrate its store is opened there
// and brought to this store's schema.
func (c *Catalogue) openBackup(ctx context.Context, o RestoreOptions, migrate bool) (*openedBackup, error) {
	ids, err := ParseIdentities(o.Key)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, errors.New("serverbackup: no key to open the backup with")
	}
	var f File
	var size int64
	switch {
	case o.Backup != "" && o.File != "":
		return nil, errors.New("serverbackup: a backup of the catalogue or a file, not both")
	case o.Backup != "":
		if f, size, _, err = c.OpenCopy(ctx, o.Backup); err != nil {
			return nil, err
		}
	case o.File != "":
		fh, err := os.Open(o.File)
		if err != nil {
			return nil, fmt.Errorf("serverbackup: %w", err)
		}
		fi, err := fh.Stat()
		if err != nil {
			fh.Close()
			return nil, fmt.Errorf("serverbackup: %w", err)
		}
		f, size = fh, fi.Size()
	default:
		return nil, errors.New("serverbackup: no backup named")
	}
	defer f.Close()
	m, _, _, err := ReadManifest(f, size)
	if err != nil {
		return nil, err
	}
	live, _ := c.Store.SchemaVersion()
	if m.Schema.Version > live {
		return nil, fmt.Errorf("%w (the backup's schema version %d, this server's %d)", ErrNewerSchema, m.Schema.Version, live)
	}
	removeStale(c.Store.Dir(), time.Now())
	tmp, err := os.MkdirTemp(c.Store.Dir(), tempPrefix+"restore-*")
	if err != nil {
		return nil, fmt.Errorf("serverbackup: %w", err)
	}
	ob := &openedBackup{tmp: tmp, dir: filepath.Join(tmp, "backup")}
	opened, err := Open(f, size, ids, ob.dir)
	if err != nil {
		ob.close()
		return nil, err
	}
	ob.m, ob.sig, ob.files = opened.Manifest, opened.Signature, opened.Files
	if !migrate {
		return ob, nil
	}
	// The backup's store, opened where it was opened: its migrations
	// run on this copy alone, and its own opaque.key and node.key are
	// read.
	st, err := store.Open(ob.dir, store.Options{Version: c.Version, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		ob.close()
		return nil, fmt.Errorf("serverbackup: the backup's store: %w", err)
	}
	v, _ := st.SchemaVersion()
	ob.migrated = v > m.Schema.Version
	ob.nodeID = st.NodeID()
	ob.cubes, err = st.Cubes(ctx)
	if cerr := st.Close(); err == nil {
		err = cerr
	}
	if err == nil && v != live {
		err = fmt.Errorf("the backup's store is of schema version %d, this server's of %d", v, live)
	}
	if err != nil {
		ob.close()
		return nil, fmt.Errorf("serverbackup: the backup's store: %w", err)
	}
	return ob, nil
}

// storeSelection is what the store replaces for the options: the scopes,
// the identity's with Identity, and opaque.key with users and sign-in.
func storeSelection(sel Selection, o RestoreOptions, dir string, dry bool) store.RestoreSelection {
	scopes := slices.Clone(sel.Scopes)
	if o.Identity {
		scopes = append(scopes, ScopeIdentity)
	}
	return store.RestoreSelection{Scopes: scopes, Serials: sel.Serials, Source: dir, OpaqueKey: sel.Has(ScopeUsers), DryRun: dry}
}

// placeFiles are the files of the backup the options put in place, and
// where: opaque.key with users and sign-in, node.key with the identity,
// the env file and the TLS files where asked for. A file asked for that
// the backup lacks, or that has no place on this box, is an error.
func (c *Catalogue) placeFiles(ob *openedBackup, sel Selection, o RestoreOptions) ([]PlannedFile, error) {
	var out []PlannedFile
	dataDir := c.Store.Dir()
	if sel.Has(ScopeUsers) {
		if f, ok := ob.file(RoleOpaqueKey); ok {
			out = append(out, PlannedFile{Role: f.Role, Name: f.Name, To: store.OpaqueKeyPath(dataDir)})
		}
	}
	if o.Identity {
		f, ok := ob.file(RoleNodeKey)
		if !ok {
			return nil, errors.New("serverbackup: the backup holds no node.key to take")
		}
		out = append(out, PlannedFile{Role: f.Role, Name: f.Name, To: store.KeyPath(dataDir)})
	}
	if o.Env {
		f, ok := ob.file(RoleEnv)
		switch {
		case !ok:
			return nil, errors.New("serverbackup: the backup holds no env file")
		case !envTarget(c.EnvFile):
			return nil, errors.New("serverbackup: this server reads no env file to restore it into; start it with one (MASSIMO_ENV_FILE)")
		}
		out = append(out, PlannedFile{Role: f.Role, Name: f.Name, To: c.EnvFile})
	}
	if o.TLS {
		n := 0
		for _, f := range ob.files {
			if !f.Role.TLS() {
				continue
			}
			n++
			var to string
			switch {
			case f.Role == RoleTLSCert:
				to = o.TLSPlace.Cert
			case f.Role == RoleTLSKey:
				to = o.TLSPlace.Key
			case o.TLSPlace.ACMEDir != "":
				to = filepath.Join(o.TLSPlace.ACMEDir, acmeName(f))
			}
			if to == "" {
				return nil, fmt.Errorf("serverbackup: this server has no place for the TLS file %s (%s)", f.Name, f.Role)
			}
			out = append(out, PlannedFile{Role: f.Role, Name: f.Name, To: to})
		}
		if n == 0 {
			return nil, errors.New("serverbackup: the backup holds no TLS files")
		}
	}
	return out, nil
}

// envTarget reports whether path is a place for the env file: a regular
// file, or none yet in a directory that exists; not "" nor /dev/null.
func envTarget(path string) bool {
	if path == "" {
		return false
	}
	fi, err := os.Stat(path)
	if err == nil {
		return fi.Mode().IsRegular()
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false
	}
	d, err := os.Stat(filepath.Dir(path))
	return err == nil && d.IsDir()
}

// acmeName is the name an ACME file takes in this box's ACME directory:
// its name on the box that made the backup, where that is one a backup
// carries, else its name in the payload.
func acmeName(f InnerFile) string {
	if base := filepath.Base(strings.ReplaceAll(f.Source, `\`, "/")); validTLSName(base) {
		return base
	}
	return strings.TrimPrefix(f.Name, TLSDir+"/")
}

// Plan answers what a restore of the options would do, changing nothing:
// the backup opened with the key, its store migrated beside where older,
// and the restore run and rolled back. A restore the store would refuse,
// a reference missing or no owner left, is answered in Refused, with
// the report's list.
func (c *Catalogue) Plan(ctx context.Context, o RestoreOptions) (RestorePlan, error) {
	sel, err := ParseScopes(o.Scopes)
	if err != nil {
		return RestorePlan{}, err
	}
	if len(sel.Scopes) == 0 {
		sel.Scopes = slices.Clone(Scopes)
	}
	ob, err := c.openBackup(ctx, o, true)
	if err != nil {
		return RestorePlan{}, err
	}
	defer ob.close()
	p := RestorePlan{Manifest: ob.m, ThisBox: ob.sig.Verdict(c.Store.NodePublicKey()) == ThisBox, NodeID: ob.nodeID, Migrated: ob.migrated,
		Selection: sel, Env: ob.m.Files.Env, TLS: ob.m.Files.TLS}
	live := map[string]bool{}
	if cs, err := c.Store.Cubes(ctx); err == nil {
		for _, cb := range cs {
			live[cb.Serial] = true
		}
	}
	for _, cb := range ob.cubes {
		p.Cubes = append(p.Cubes, PlanCube{Serial: cb.Serial, Name: cb.Name, Live: live[cb.Serial]})
	}
	if p.Files, err = c.placeFiles(ob, sel, o); err != nil {
		p.Refused = err.Error()
		return p, nil
	}
	p.Report, err = c.Store.Restore(ctx, storeSelection(sel, o, ob.dir, true))
	switch {
	case errors.Is(err, store.ErrRestoreMissing), errors.Is(err, store.ErrRestoreNoOwner):
		p.Refused = err.Error()
	case err != nil:
		return p, fmt.Errorf("serverbackup: %w", err)
	}
	return p, nil
}

// Restore restores the options' scopes and files from a backup (see the
// file's comment): the restore planned first, so that a refusal takes no
// backup; then the pinned local backup; then the store, in one
// transaction; then the files. The server is to restart after: its store
// keeps opaque.key and its node in memory. A refusal of the store is
// store.ErrRestoreMissing or store.ErrRestoreNoOwner, with the report.
func (c *Catalogue) Restore(ctx context.Context, o RestoreOptions) (Restored, error) {
	sel, err := ParseScopes(o.Scopes)
	if err != nil {
		return Restored{}, err
	}
	if len(sel.Scopes) == 0 && !o.Identity && !o.Env && !o.TLS {
		return Restored{}, errNothing
	}
	ob, err := c.openBackup(ctx, o, true)
	if err != nil {
		return Restored{}, err
	}
	defer ob.close()
	res := Restored{Selection: sel}
	files, err := c.placeFiles(ob, sel, o)
	if err != nil {
		return res, err
	}
	data := make([][]byte, len(files))
	for i, f := range files {
		if data[i], err = readInner(ob.dir, f.Name); err != nil {
			return res, err
		}
	}
	ss := storeSelection(sel, o, ob.dir, true)
	if len(ss.Scopes) > 0 {
		if res.Report, err = c.Store.Restore(ctx, ss); err != nil {
			return res, err
		}
	}
	if !o.NoBackupFirst {
		b, outs, err := c.Make(ctx, MakeRequest{Trigger: TriggerBeforeRestore, UserID: o.UserID, Local: true, PinFor: PinBefore})
		res.Before, res.Steps = b, outs
		if err != nil {
			return res, fmt.Errorf("serverbackup: the backup before the restore: %w", err)
		}
	}
	if len(ss.Scopes) > 0 {
		ss.DryRun = false
		if res.Report, err = c.Store.Restore(ctx, ss); err != nil {
			return res, err
		}
	}
	for i, f := range files {
		before, err := putFile(f.To, data[i])
		if err != nil {
			return res, fmt.Errorf("serverbackup: the store is restored, but %s is not: %w", f.To, err)
		}
		f.Before = before
		res.Files = append(res.Files, f)
	}
	c.log().Info("server restored from a backup", "backup", ob.m.UUID, "scopes", strings.Join(ss.Scopes, ","), "files", len(res.Files))
	return res, nil
}

// readInner reads a file of the opened payload, whole.
func readInner(dir, name string) ([]byte, error) {
	p, ok := innerPath(dir, name)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotABackup, quoted(name))
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("serverbackup: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxRestoredFile+1))
	if err != nil {
		return nil, fmt.Errorf("serverbackup: %s: %w", name, err)
	}
	if len(b) > maxRestoredFile {
		return nil, fmt.Errorf("serverbackup: %s is longer than %d bytes", name, maxRestoredFile)
	}
	return b, nil
}

// putFile writes data at path whole (internal/safefile), keeping the
// file it replaces beside as path.before-restore, which it returns; a
// file that is already data is left, and "" returned.
func putFile(path string, data []byte) (string, error) {
	old, err := os.ReadFile(path)
	switch {
	case err == nil && bytes.Equal(old, data):
		return "", nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return "", err
	}
	before := ""
	if err == nil {
		before = path + BeforeRestoreSuffix
		if err := safefile.Write(before, old, safefile.Options{Mode: 0o600}); err != nil {
			return "", err
		}
	} else if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := safefile.Write(path, data, safefile.Options{}); err != nil {
		return "", err
	}
	return before, nil
}

// CheckKey opens a backup with the key into a temporary directory and
// checks its store with PRAGMA integrity_check, without migrating it or
// changing anything (D4-A: "Check with the key"); the directory goes
// when it returns.
func (c *Catalogue) CheckKey(ctx context.Context, o RestoreOptions) (KeyCheck, error) {
	ob, err := c.openBackup(ctx, o, false)
	if err != nil {
		return KeyCheck{}, err
	}
	defer ob.close()
	lines, err := store.IntegrityCheck(ctx, filepath.Join(ob.dir, store.FileName))
	if err != nil {
		return KeyCheck{}, err
	}
	return KeyCheck{Manifest: ob.m, ThisBox: ob.sig.Verdict(c.Store.NodePublicKey()) == ThisBox, Files: ob.files, Integrity: lines,
		OK: len(lines) == 1 && lines[0] == "ok"}, nil
}
