// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The payload's files: files.json, encrypted with them, names each one's
// role and where it was, so that a restore puts each back; and the
// names a payload may hold, checked for every system it may be opened
// on.

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.metaworx.ch/massimo/internal/store"
)

// FilesName is files.json, the payload's list of its other files.
const FilesName = "files.json"

// Role is what a file of the payload is.
type Role string

// The roles.
const (
	RoleStore     Role = "store"      // massimo.db
	RoleOpaqueKey Role = "opaque-key" // opaque.key
	RoleNodeKey   Role = "node-key"   // node.key
	RoleEnv       Role = "env"        // the env file serve reads
	RoleTLSCert   Role = "tls-cert"   // the certificate of MASSIMO_TLS_CERT
	RoleTLSKey    Role = "tls-key"    // its key
	// The ACME directory's files: the account key, a certificate (with
	// its key, as autocert keeps one), a DNS-01 order's key. Any role
	// "acme-" and lower-case letters, digits and hyphens is taken too.
	RoleACMEAccount Role = "acme-account"
	RoleACMECert    Role = "acme-cert"
	RoleACMEKey     Role = "acme-key"
)

// TLS reports whether r is the role of a file under tls/.
func (r Role) TLS() bool {
	if r == RoleTLSCert || r == RoleTLSKey {
		return true
	}
	rest, ok := strings.CutPrefix(string(r), "acme-")
	return ok && rest != "" && len(rest) <= 32 && strings.Trim(rest, "abcdefghijklmnopqrstuvwxyz0123456789-") == ""
}

// TLSFile is a TLS file a backup carries, with its role.
type TLSFile struct {
	Role Role
	Path string
}

// InnerFile is a file of the payload as files.json names it.
type InnerFile struct {
	// Name is its name in the payload: massimo.db, opaque.key,
	// node.key, env, or tls/ and a name of its own.
	Name string `json:"name"`
	Role Role   `json:"role"`
	// Source is where it was on the box that made the backup, as that
	// box wrote it: text, never a path to write to unchecked.
	Source string `json:"source"`
}

// filesList is files.json.
type filesList struct {
	Files []InnerFile `json:"files"`
}

// maxSource bounds a source path in files.json.
const maxSource = 4096

// roleOfName is the role a name of the payload must have; "" for a TLS
// file, whose role is one of Role.TLS.
func roleOfName(name string) Role {
	switch name {
	case store.FileName:
		return RoleStore
	case store.OpaqueKeyFileName:
		return RoleOpaqueKey
	case store.KeyFileName:
		return RoleNodeKey
	case EnvName:
		return RoleEnv
	}
	return ""
}

// checkFiles refuses a files.json that does not name each of the
// payload's other files once, names, roles and sources checked, or one
// without the store.
func checkFiles(list filesList, inZip map[string]bool) error {
	seen := map[string]bool{}
	for _, f := range list.Files {
		want := roleOfName(f.Name)
		switch {
		case f.Name == FilesName || !validInnerName(f.Name):
			return fmt.Errorf("%w: files.json names %q, which a payload may not hold", ErrNotABackup, f.Name)
		case seen[f.Name]:
			return fmt.Errorf("%w: files.json names %q twice", ErrNotABackup, f.Name)
		case !inZip[f.Name]:
			return fmt.Errorf("%w: files.json names %q, which the payload lacks", ErrNotABackup, f.Name)
		case want != "" && f.Role != want, want == "" && !f.Role.TLS():
			return fmt.Errorf("%w: files.json gives %q the role %q", ErrNotABackup, f.Name, f.Role)
		case !plainText(f.Source, maxSource):
			return fmt.Errorf("%w: files.json's source of %q", ErrNotABackup, f.Name)
		}
		seen[f.Name] = true
	}
	for name := range inZip {
		if name != FilesName && !seen[name] {
			return fmt.Errorf("%w: the payload holds %q, which files.json does not name", ErrNotABackup, name)
		}
	}
	if !seen[store.FileName] {
		return fmt.Errorf("%w: the payload holds no %s", ErrNotABackup, store.FileName)
	}
	return nil
}

// parseFiles reads files.json.
func parseFiles(b []byte) (filesList, error) {
	var list filesList
	if err := json.Unmarshal(b, &list); err != nil {
		return list, fmt.Errorf("%w: %s: %w", ErrNotABackup, FilesName, err)
	}
	if len(list.Files) > maxTLSFiles+4 {
		return list, fmt.Errorf("%w: %s names %d files", ErrNotABackup, FilesName, len(list.Files))
	}
	return list, nil
}

// plainText reports whether s is UTF-8 of at most max bytes without a
// control or format character.
func plainText(s string, max int) bool {
	return len(s) <= max && utf8.ValidString(s) &&
		strings.IndexFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) < 0
}

// validInnerName reports whether name is one a payload may hold:
// files.json, massimo.db, opaque.key, node.key, env, or tls/ and a name
// validTLSName takes.
func validInnerName(name string) bool {
	if name == FilesName || roleOfName(name) != "" {
		return true
	}
	t, ok := strings.CutPrefix(name, TLSDir+"/")
	return ok && validTLSName(t)
}

// validTLSName reports whether name is a file name tls/ may hold on
// every system a backup may be opened on: one element, local, not
// hidden, without a character Windows refuses, a trailing dot or space,
// or a name of a Windows device (CON, NUL, COM1, …, with any extension).
func validTLSName(name string) bool {
	switch {
	case name == "", len(name) > 255, !utf8.ValidString(name), !filepath.IsLocal(name),
		strings.HasPrefix(name, "."), strings.HasSuffix(name, "."), strings.HasSuffix(name, " "),
		strings.ContainsAny(name, `<>:"/\|?*`), strings.IndexFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0:
		return false
	}
	return !windowsDevice(name)
}

// windowsDevice reports whether name, its extension left aside, is one
// of Windows' reserved device names.
func windowsDevice(name string) bool {
	base, _, _ := strings.Cut(strings.ToUpper(name), ".")
	base = strings.TrimRight(base, " ")
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if len(base) < 4 || (!strings.HasPrefix(base, "COM") && !strings.HasPrefix(base, "LPT")) {
		return false
	}
	switch base[3:] {
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
		return true
	}
	return false
}

// innerPath returns where the payload's file name goes under dir, or
// false for a name the payload may not hold.
func innerPath(dir, name string) (string, bool) {
	if !validInnerName(name) || !filepath.IsLocal(filepath.FromSlash(name)) {
		return "", false
	}
	return filepath.Join(dir, filepath.FromSlash(name)), true
}

// tlsName returns the payload's name of a TLS file at path: tls/ and its
// name, or a number before it where that is taken.
func tlsName(path string, taken map[string]bool) (string, error) {
	base := filepath.Base(path)
	if !validTLSName(base) {
		return "", fmt.Errorf("serverbackup: the TLS file %s: its name is not one a backup carries", path)
	}
	name := TLSDir + "/" + base
	for n := 2; taken[name]; n++ {
		name = fmt.Sprintf("%s/%d-%s", TLSDir, n, base)
	}
	if !validInnerName(name) {
		return "", errors.New("serverbackup: the TLS file " + path + ": its name is too long")
	}
	return name, nil
}
