// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The targets of the server's backups (AP ServerBackup v1.0, Block 2,
// D11-A): a place that keeps copies, the same file on each, behind one
// interface. Each kind of target registers itself in its file's init
// (Register): its fields, which the card's form is drawn from (GET
// /api/v1/server-backup-targets/kinds), and how a target of it is made.
// The local kind is local.go's; WebDAV, SFTP, SMB and S3 follow.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Target keeps copies of server backups.
type Target interface {
	// Put stores r under name by way of a temporary name, so that a put
	// broken off leaves nothing under name.
	Put(ctx context.Context, name string, r io.Reader, size int64) error
	// Open returns name for ranged reads, with its size.
	Open(ctx context.Context, name string) (File, int64, error)
	// List returns the backups there (names ending in .zip).
	List(ctx context.Context) ([]Entry, error)
	// Delete removes name; a name not there is no error.
	Delete(ctx context.Context, name string) error
	// Check proves the target reachable and writable (the card's Test).
	Check(ctx context.Context) error
}

// File is a copy opened for ranged reads.
type File interface {
	io.ReaderAt
	io.Closer
}

// Entry is a file a target lists.
type Entry struct {
	Name string
	Size int64
	Time time.Time
}

// Kind describes a kind of target: its fields, for the card's form
// (GET /api/v1/server-backup-targets/kinds), and how to make one.
type Kind struct {
	// Name is the kind's key: lower-case letters and digits, as the
	// store keeps it (server_backup_targets.kind).
	Name   string
	Fields []Field
	// New makes a target of the kind from its fields, as Check left
	// them, and its secret field's value, nil where it has none. A
	// field of type path is absolute by then: the server takes a
	// relative one as relative to the data directory.
	New func(cfg map[string]string, secret []byte) (Target, error)
}

// Field is one field of a kind's form.
type Field struct {
	Key      string // also the text key's last part: serverbackup.field.<kind>.<key>
	Type     string // text, number, path, secret, bool, choice
	Required bool
	Default  string
	Choices  []string
	// Warn is a pattern, in the syntax RE2 and JavaScript share, of the
	// values the card warns of beside the field, with the text
	// serverbackup.warn.<kind>.<key>; it refuses nothing.
	Warn string
}

// The types of a field. A kind has at most one secret field, whose
// value the store keeps sealed apart from the others; a path is a
// directory of this box, relative to the data directory where it is not
// absolute (a remote directory is text).
const (
	FieldText   = "text"
	FieldNumber = "number"
	FieldPath   = "path"
	FieldSecret = "secret"
	FieldBool   = "bool"
	FieldChoice = "choice"
)

var (
	kindsMu  sync.RWMutex
	kinds    = map[string]Kind{}
	kindName = regexp.MustCompile(`^[a-z][a-z0-9]{0,15}$`)
	fieldKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
)

// Register adds a kind; each target file registers its own in init.
// A kind of a name taken, a field of no known type, two fields of one
// key, two secret fields, or a choice without choices is a mistake of
// the code, and panics.
func Register(k Kind) {
	if err := k.valid(); err != nil {
		panic(err)
	}
	kindsMu.Lock()
	defer kindsMu.Unlock()
	if _, ok := kinds[k.Name]; ok {
		panic(fmt.Sprintf("serverbackup: kind %q registered twice", k.Name))
	}
	kinds[k.Name] = k
}

// valid checks a kind before it is registered.
func (k Kind) valid() error {
	if !kindName.MatchString(k.Name) || k.New == nil {
		return fmt.Errorf("serverbackup: kind %q: a name of lower-case letters and digits, and New", k.Name)
	}
	seen, secrets := map[string]bool{}, 0
	for _, f := range k.Fields {
		switch {
		case !fieldKey.MatchString(f.Key) || seen[f.Key]:
			return fmt.Errorf("serverbackup: kind %s: field %q: a key of its own, of lower-case letters, digits and _", k.Name, f.Key)
		case !slices.Contains([]string{FieldText, FieldNumber, FieldPath, FieldSecret, FieldBool, FieldChoice}, f.Type):
			return fmt.Errorf("serverbackup: kind %s: field %s: type %q", k.Name, f.Key, f.Type)
		case f.Type == FieldChoice && len(f.Choices) == 0:
			return fmt.Errorf("serverbackup: kind %s: field %s: a choice without choices", k.Name, f.Key)
		case f.Type == FieldSecret && f.Default != "":
			return fmt.Errorf("serverbackup: kind %s: field %s: a secret has no default", k.Name, f.Key)
		case f.Warn != "" && f.Type == FieldSecret:
			return fmt.Errorf("serverbackup: kind %s: field %s: a secret, never shown, has no warning", k.Name, f.Key)
		}
		if f.Warn != "" {
			if _, err := regexp.Compile(f.Warn); err != nil {
				return fmt.Errorf("serverbackup: kind %s: field %s: its warning: %w", k.Name, f.Key, err)
			}
		}
		if f.Default != "" {
			if _, err := f.check(f.Default); err != nil {
				return fmt.Errorf("serverbackup: kind %s: field %s: its default: %w", k.Name, f.Key, err)
			}
		}
		seen[f.Key] = true
		if f.Type == FieldSecret {
			secrets++
		}
	}
	if secrets > 1 {
		return fmt.Errorf("serverbackup: kind %s: more than one secret field", k.Name)
	}
	return nil
}

// Kinds lists the kinds registered, by name.
func Kinds() []Kind {
	kindsMu.RLock()
	defer kindsMu.RUnlock()
	out := make([]Kind, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, k)
	}
	slices.SortFunc(out, func(a, b Kind) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// KindOf returns the kind of a name.
func KindOf(name string) (Kind, bool) {
	kindsMu.RLock()
	defer kindsMu.RUnlock()
	k, ok := kinds[name]
	return k, ok
}

// Secret returns the kind's secret field, false where it has none.
func (k Kind) Secret() (Field, bool) {
	for _, f := range k.Fields {
		if f.Type == FieldSecret {
			return f, true
		}
	}
	return Field{}, false
}

// maxFieldValue bounds a field's value.
const maxFieldValue = 4096

// Check reads a target's fields for the kind: each value checked by its
// field's type and trimmed, a field not given or empty taking its
// default, a required one without either refused, and a key the kind
// does not have refused, its secret field's among them, which is given
// apart (hasSecret says whether it is set). The error names the field.
func (k Kind) Check(cfg map[string]string, hasSecret bool) (map[string]string, error) {
	out := map[string]string{}
	for key := range cfg {
		f, ok := k.field(key)
		switch {
		case !ok:
			return nil, fmt.Errorf("serverbackup: a target of kind %s has no field %q", k.Name, key)
		case f.Type == FieldSecret:
			return nil, fmt.Errorf("serverbackup: %s is the secret, given apart", key)
		}
	}
	for _, f := range k.Fields {
		if f.Type == FieldSecret {
			if f.Required && !hasSecret {
				return nil, fmt.Errorf("serverbackup: %s is required", f.Key)
			}
			continue
		}
		v := strings.TrimSpace(cfg[f.Key])
		if v == "" {
			v = f.Default
		}
		if v == "" {
			if f.Required {
				return nil, fmt.Errorf("serverbackup: %s is required", f.Key)
			}
			continue
		}
		v, err := f.check(v)
		if err != nil {
			return nil, fmt.Errorf("serverbackup: %s: %w", f.Key, err)
		}
		out[f.Key] = v
	}
	return out, nil
}

// field returns the kind's field of a key.
func (k Kind) field(key string) (Field, bool) {
	for _, f := range k.Fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}

// check reads one value of the field, as it is kept.
func (f Field) check(v string) (string, error) {
	switch {
	case len(v) > maxFieldValue:
		return "", fmt.Errorf("longer than %d bytes", maxFieldValue)
	case strings.ContainsFunc(v, func(r rune) bool { return r < 0x20 || r == 0x7f }):
		return "", errors.New("a control character")
	}
	switch f.Type {
	case FieldNumber:
		if _, err := strconv.ParseInt(v, 10, 64); err != nil {
			return "", fmt.Errorf("%q is no whole number", v)
		}
	case FieldBool:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return "", fmt.Errorf("%q: want true or false", v)
		}
		v = strconv.FormatBool(b)
	case FieldChoice:
		if !slices.Contains(f.Choices, v) {
			return "", fmt.Errorf("%q: want one of %s", v, strings.Join(f.Choices, ", "))
		}
	}
	return v, nil
}

// maxName bounds a backup's name on a target.
const maxName = 200

// validName is a backup's name on a target: letters, digits, ".", "_"
// and "-", not hidden, ending in .zip.
var validName = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*\.zip$`)

// CheckName refuses a name a target may not hold a backup under: one
// with a separator or of a hidden file, one not ending in .zip, a long
// one. Every target checks the names it is given with it.
func CheckName(name string) error {
	if len(name) > maxName || !validName.MatchString(name) || strings.Contains(name, "..") {
		return fmt.Errorf("serverbackup: %q is no name of a backup on a target", name)
	}
	return nil
}

// FileName is a backup's name on every target: massimo-, the time it
// was made in UTC, its uuid, .zip.
func FileName(m Manifest) string {
	return "massimo-" + m.Created.UTC().Format("20060102-150405") + "-" + m.UUID + ".zip"
}

// Takes says which backups a target takes (D11-A): every one, or the
// first of each day, week or month, in the box's time.
type Takes string

// The values of Takes.
const (
	TakesEvery   Takes = "every"
	TakesDaily   Takes = "daily"
	TakesWeekly  Takes = "weekly"
	TakesMonthly Takes = "monthly"
)

// Valid reports whether t is one of the values.
func (t Takes) Valid() bool {
	switch t {
	case TakesEvery, TakesDaily, TakesWeekly, TakesMonthly:
		return true
	}
	return false
}

// Takes reports whether a target that takes t takes a backup made at,
// where it holds copies made at have: every one, or one of a day, a week
// (Monday to Sunday) or a month of which it holds none. The times are
// taken in their own location, the box's.
func (t Takes) Takes(at time.Time, have []time.Time) bool {
	if t == TakesEvery || t == "" {
		return true
	}
	b := bucketOf(t)
	for _, h := range have {
		if b(h) == b(at) {
			return false
		}
	}
	return true
}

// Period is how long a target that takes t may go without a copy of a
// daily schedule's backups: a day, a week or a month (31 days); a target
// two periods behind is marked (Behind).
func (t Takes) Period() time.Duration {
	switch t {
	case TakesWeekly:
		return 7 * 24 * time.Hour
	case TakesMonthly:
		return 31 * 24 * time.Hour
	}
	return 24 * time.Hour
}

// Behind reports whether a target that takes t is two periods behind at
// now: its last copy, or where it has none its creation, longer ago than
// twice its Period.
func (t Takes) Behind(last, created, now time.Time) bool {
	since := last
	if since.IsZero() {
		since = created
	}
	return !since.IsZero() && now.Sub(since) > 2*t.Period()
}

// bucketOf is the bucket a time falls into for t: its day, ISO week or
// month.
func bucketOf(t Takes) func(time.Time) int {
	switch t {
	case TakesWeekly:
		return weekOf
	case TakesMonthly:
		return monthOf
	}
	return dayOf
}

func dayOf(t time.Time) int   { y, m, d := t.Date(); return y*10000 + int(m)*100 + d }
func weekOf(t time.Time) int  { y, w := t.ISOWeek(); return y*100 + w }
func monthOf(t time.Time) int { y, m, _ := t.Date(); return y*100 + int(m) }
func yearOf(t time.Time) int  { return t.Year() }
