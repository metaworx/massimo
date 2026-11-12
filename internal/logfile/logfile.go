// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package logfile is serve's -log file (AP OneLog, Block 2): a writer
// that appends to it and rotates it itself, on every platform, rather
// than leaving that to logrotate, which runs on Linux alone and keeps
// files by count only.
//
// A rotation renames serve.log to serve.log.1 and the older ones up by
// one, under logrotate's names, so that the files logrotate made before
// join the rotation; every rotated file but the newest is gzipped, as
// logrotate's delaycompress does. It happens when the file reaches a
// size, when the file's last line is in another format than the one the
// writer writes (at the open, so that every file holds one format), and
// when serve asks for it (a reload, SIGHUP). The oldest rotated file is
// deleted only while more than Keep are kept and the others still cover
// KeepMin bytes on disk, so that many small files stay.
//
// A rotation says so in the files and to serve: the old file's last line,
// in that file's format, the new file's first line, written before any
// other, and a Rotation handed to the function Notify sets, which serve
// writes on stderr and as the log_rotated event. The writer never logs
// through serve's logger: a line logged from inside Write would wait for
// the handler that called it, and one logged after it could come second.
package logfile

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Mode is a log file's mode: its owner writes it, the admin group (the
// directory's, set-group-id, massimo install) reads it.
const Mode = 0o640

// The defaults of serve's -log-rotate-size, -log-keep and -log-keep-min.
const (
	DefaultSize    = 10 << 20
	DefaultKeep    = 8
	DefaultKeepMin = 5 << 20
)

// tailRead is how much of a file's end the format check reads.
const tailRead = 64 << 10

// Format is the format of a file's lines.
type Format string

// The formats: slog's JSON handler starts every line with "{", its text
// handler with "time=".
const (
	JSON  Format = "json"
	Plain Format = "plain"
)

// The reasons of a rotation.
const (
	ReasonSize   = "size"
	ReasonFormat = "format"
)

// Options say where the file is, the format written to it and when it is
// rotated.
type Options struct {
	Path   string
	Format Format
	// Size rotates the file once a line would take it past this many
	// bytes; 0 never.
	Size int64
	// Keep and KeepMin: the oldest rotated file is deleted while more
	// than Keep are kept and the others still cover KeepMin bytes.
	Keep    int
	KeepMin int64
	// Now is the clock of the old file's last line; nil is time.Now.
	Now func() time.Time
}

// Rotation is what a rotation did.
type Rotation struct {
	Reason string
	Time   time.Time
	// Previous is the name the old file has now, serve.log.1.
	Previous string
	// Kept is the number of rotated files kept, Removed those deleted.
	Kept    int
	Removed []string
	// Err is what went wrong: the file is then appended to as it was, or
	// a step after the rename failed (a gzip, a deletion).
	Err error
}

// File is the log file, written by any number of goroutines.
type File struct {
	opts Options

	mu     sync.Mutex
	f      *os.File
	size   int64
	format Format // the format of the open file's lines: opts.Format once written

	nmu     sync.Mutex
	notify  func(Rotation)
	pending []Rotation
}

// Open opens the log file for appending, made Mode, and narrows one that
// others may read more widely to Mode; narrowErr is that narrowing's
// error, as for a file of another user's, beside a file that is open all
// the same. A file whose last line is in another format is rotated
// first; its Rotation waits for Notify. Windows keeps the file's ACLs.
func Open(opts Options) (f *File, narrowErr error, err error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	f = &File{opts: opts}
	if narrowErr, err = f.open(); err != nil {
		return nil, nil, err
	}
	if last, ok := lastLineFormat(opts.Path); ok && last != opts.Format {
		f.format = last
		f.mu.Lock()
		r, did := f.rotateLocked(ReasonFormat)
		f.mu.Unlock()
		if did {
			f.queue(r)
		}
	}
	return f, narrowErr, nil
}

// open opens the file at the path, narrowed as Open says.
func (f *File) open() (narrowErr error, err error) {
	file, err := os.OpenFile(f.opts.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, Mode)
	if err != nil {
		return nil, err
	}
	fi, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if perm := fi.Mode().Perm(); runtime.GOOS != "windows" && perm&^Mode != 0 {
		narrowErr = file.Chmod(perm & Mode)
	}
	f.f, f.size, f.format = file, fi.Size(), f.opts.Format
	return narrowErr, nil
}

// Write appends p, one line of the log, rotating the file first when p
// would take it past Size; the Rotation is handed on in a goroutine of
// its own, since Write runs inside a handler that logging would wait for.
func (f *File) Write(p []byte) (int, error) {
	f.mu.Lock()
	var r Rotation
	var did bool
	if f.opts.Size > 0 && f.size > 0 && f.size+int64(len(p)) > f.opts.Size {
		r, did = f.rotateLocked(ReasonSize)
	}
	n, err := f.f.Write(p)
	f.size += int64(n)
	f.format = f.opts.Format
	f.mu.Unlock()
	if did {
		go f.queue(r)
	}
	return n, err
}

// Rotate rotates the file for reason, unless it is empty, and hands the
// Rotation on before it returns; it reports whether it rotated. Call it
// outside a logging call (a reload, SIGHUP).
func (f *File) Rotate(reason string) bool {
	f.mu.Lock()
	r, did := f.rotateLocked(reason)
	f.mu.Unlock()
	if did {
		f.queue(r)
	}
	return did
}

// Close closes the file.
func (f *File) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.f.Close()
}

// Notify sets the function the rotations are handed to, and hands it
// those that came before, oldest first.
func (f *File) Notify(fn func(Rotation)) {
	f.nmu.Lock()
	f.notify = fn
	pending := f.pending
	f.pending = nil
	f.nmu.Unlock()
	for _, r := range pending {
		fn(r)
	}
}

func (f *File) queue(r Rotation) {
	f.nmu.Lock()
	fn := f.notify
	if fn == nil {
		f.pending = append(f.pending, r)
	}
	f.nmu.Unlock()
	if fn != nil {
		fn(r)
	}
}

// rotateLocked rotates the file, f.mu held: the old file's last line,
// the renames, the new file, the gzip of the one before and the
// retention. A file that is empty is not rotated. Where the rename fails
// the old file is appended to again, and the Rotation says why.
func (f *File) rotateLocked(reason string) (Rotation, bool) {
	if f.size == 0 {
		return Rotation{}, false
	}
	now := f.opts.Now()
	base := filepath.Base(f.opts.Path)
	r := Rotation{Reason: reason, Time: now, Previous: base + ".1"}
	f.f.Write(lastLine(f.format, now, reason, base))
	f.f.Close()
	if err := f.shift(); err != nil {
		r.Err = err
		if _, err := f.open(); err != nil {
			// Nothing to write to: the next writes fail, as an unwritable
			// file's do.
			r.Err = errors.Join(r.Err, err)
			return r, true
		}
		f.writeLocked(firstLine(f.format, r))
		return r, true
	}
	if _, err := f.open(); err != nil {
		r.Err = err
		return r, true
	}
	if err := gzipFile(f.opts.Path + ".2"); err != nil && !errors.Is(err, os.ErrNotExist) {
		r.Err = err
	}
	kept, removed, err := f.retain()
	r.Kept, r.Removed = kept, removed
	if err != nil {
		r.Err = errors.Join(r.Err, err)
	}
	f.writeLocked(firstLine(f.format, r))
	return r, true
}

// writeLocked appends a line the writer makes itself, f.mu held.
func (f *File) writeLocked(p []byte) {
	n, _ := f.f.Write(p)
	f.size += int64(n)
}

// lastLine is the old file's last line, in its format: the reason, and
// the file the log goes on in.
func lastLine(format Format, now time.Time, reason, next string) []byte {
	rec := slog.NewRecord(now, slog.LevelInfo, "log rotated", 0)
	rec.AddAttrs(slog.String("source", "system"), slog.String("reason", reason), slog.String("next", next))
	return formatLine(format, rec)
}

// firstLine is the new file's first line, in its format, as serve logs
// the rotation on stderr: the reason, the old file's name, the rotated
// files kept and deleted; at warn with the error where a step failed.
func firstLine(format Format, r Rotation) []byte {
	level := slog.LevelInfo
	if r.Err != nil {
		level = slog.LevelWarn
	}
	rec := slog.NewRecord(r.Time, level, "log rotated", 0)
	rec.AddAttrs(r.Attrs()...)
	return formatLine(format, rec)
}

// Attrs are a rotation's attributes as its lines carry them.
func (r Rotation) Attrs() []slog.Attr {
	removed := r.Removed
	if removed == nil {
		removed = []string{}
	}
	a := []slog.Attr{slog.String("source", "system"), slog.String("reason", r.Reason), slog.String("previous", r.Previous),
		slog.Int("kept", r.Kept), slog.Any("removed", removed)}
	if r.Err != nil {
		a = append(a, slog.String("error", r.Err.Error()))
	}
	return a
}

// formatLine is a record as slog's handler of format writes it.
func formatLine(format Format, rec slog.Record) []byte {
	var b bytes.Buffer
	var h slog.Handler = slog.NewTextHandler(&b, nil)
	if format == JSON {
		h = slog.NewJSONHandler(&b, nil)
	}
	h.Handle(context.Background(), rec)
	return b.Bytes()
}

// rotated is a rotated file: its number, whether it is gzipped, its size.
type rotated struct {
	n    int
	gz   bool
	size int64
}

func (r rotated) name(path string) string {
	n := path + "." + strconv.Itoa(r.n)
	if r.gz {
		n += ".gz"
	}
	return n
}

// list returns the rotated files beside path, by number.
func list(path string) []rotated {
	dir, base := filepath.Dir(path), filepath.Base(path)
	entries, _ := os.ReadDir(dir)
	var out []rotated
	for _, e := range entries {
		name, ok := strings.CutPrefix(e.Name(), base+".")
		if !ok || !e.Type().IsRegular() {
			continue
		}
		r := rotated{}
		name, r.gz = strings.CutSuffix(name, ".gz")
		n, err := strconv.Atoi(name)
		if err != nil || n < 1 || strconv.Itoa(n) != name {
			continue
		}
		r.n = n
		if fi, err := e.Info(); err == nil {
			r.size = fi.Size()
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].n < out[j].n })
	return out
}

// shift renames the rotated files up by one, the highest first, and the
// file to .1.
func (f *File) shift() error {
	files := list(f.opts.Path)
	for i := len(files) - 1; i >= 0; i-- {
		from := files[i].name(f.opts.Path)
		next := files[i]
		next.n++
		if err := os.Rename(from, next.name(f.opts.Path)); err != nil {
			return fmt.Errorf("log not rotated: %w", err)
		}
	}
	if err := os.Rename(f.opts.Path, f.opts.Path+".1"); err != nil {
		return fmt.Errorf("log not rotated: %w", err)
	}
	return nil
}

// retain deletes the oldest rotated files while more than Keep are kept
// and the others cover KeepMin; it returns how many are kept and the
// names of those deleted.
func (f *File) retain() (int, []string, error) {
	files := list(f.opts.Path)
	var total int64
	for _, r := range files {
		total += r.size
	}
	var removed []string
	var errs []error
	for len(files) > f.opts.Keep && total-files[len(files)-1].size >= f.opts.KeepMin {
		oldest := files[len(files)-1]
		name := oldest.name(f.opts.Path)
		if err := os.Remove(name); err != nil {
			errs = append(errs, err)
			break
		}
		removed = append(removed, filepath.Base(name))
		total -= oldest.size
		files = files[:len(files)-1]
	}
	return len(files), removed, errors.Join(errs...)
}

// gzipFile compresses path into path.gz, made Mode, and removes path.
func gzipFile(path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := path + ".gz.tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, Mode)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(out)
	_, err = io.Copy(zw, in)
	err = errors.Join(err, zw.Close(), out.Sync(), out.Close())
	if err != nil {
		os.Remove(tmp)
		return err
	}
	in.Close()
	if err := os.Rename(tmp, path+".gz"); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Remove(path)
}

// lastLineFormat reports the format of the last line of the file at
// path, read from its last tailRead bytes; ok is false for an empty or
// unreadable file.
func lastLineFormat(path string) (Format, bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()
	fi, err := file.Stat()
	if err != nil || fi.Size() == 0 {
		return "", false
	}
	off := max(fi.Size()-tailRead, 0)
	buf := make([]byte, fi.Size()-off)
	if _, err := file.ReadAt(buf, off); err != nil && !errors.Is(err, io.EOF) {
		return "", false
	}
	buf = bytes.TrimRight(buf, "\r\n")
	if i := bytes.LastIndexByte(buf, '\n'); i >= 0 {
		buf = buf[i+1:]
	}
	buf = bytes.TrimLeft(buf, " \t")
	if len(buf) == 0 {
		return "", false
	}
	if buf[0] == '{' {
		return JSON, true
	}
	return Plain, true
}
