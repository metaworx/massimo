// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The SMB target (AP ServerBackup v1.0, Block 8, D9-B, D16-A): a folder
// of a share on a NAS, a Windows computer or Samba, reached in-process
// over SMB 2 and 3 with github.com/cloudsoda/go-smb2, the same on every
// platform, so that Windows' service account needs no rights on the
// share. It signs in by NTLM with a user and its password, the target's
// secret. It signs every message, which also refuses a server that takes
// a wrong password for its guest, and encrypts where the server asks for
// it (SMB 3: Samba's server smb encrypt, Windows' EncryptData). Each call
// of the target connects, mounts the share and logs off again; a copy is
// written under a hidden temporary name beside its own and renamed.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudsoda/go-smb2"
)

// KindSMB is the SMB target's kind, and DefaultSMBPort its port where
// none is given.
const (
	KindSMB        = "smb"
	DefaultSMBPort = 445
)

const (
	// smbTimeout bounds each call to the server: the connection with
	// its sign-in and mount, and each read, write, rename or listing
	// after it. A call that takes longer ends the connection.
	smbTimeout = time.Minute
	// smbCloseTimeout bounds the unmount and log-off after a call.
	smbCloseTimeout = 5 * time.Second
	// smbChunk is how much one write sends: the most a Windows server
	// takes in one request.
	smbChunk = 1 << 20
)

func init() {
	Register(Kind{
		Name: KindSMB,
		Fields: []Field{
			{Key: "host", Type: FieldText, Required: true},
			{Key: "port", Type: FieldNumber, Default: strconv.Itoa(DefaultSMBPort)},
			{Key: "share", Type: FieldText, Required: true},
			{Key: "path", Type: FieldText},
			{Key: "user", Type: FieldText, Required: true},
			{Key: "password", Type: FieldSecret, Required: true},
			{Key: "domain", Type: FieldText},
		},
		New: func(cfg map[string]string, secret []byte) (Target, error) {
			return NewSMB(cfg, secret)
		},
	})
}

// SMB is a target in a folder of an SMB share. Its String is where it
// is, //host/share/path, never its password.
type SMB struct {
	host     string // a name or an address, without brackets
	addr     string // host:port
	share    string
	dir      string // in the share, its parts joined by \; "" for its root
	user     string
	domain   string
	password string
	label    string
	timeout  time.Duration
	dial     func(ctx context.Context, t *SMB) (smbShare, error)
}

var (
	// smbHostName is a host's name: letters, digits, ".", "_" and "-".
	smbHostName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,252}$`)
	// smbBadChars are the characters a share's or a folder's name may
	// not hold, as Windows refuses them.
	smbBadChars = `\/:*?"<>|`
)

// NewSMB returns the SMB target of a kind's fields, as Kind.Check left
// them, and its password. The host is a name or an address, without
// smb:// or \\; the path is a folder in the share, its parts separated
// by / or \, made at the first put or check where it is missing. A user
// written DOMAIN\user, with no domain given, is taken as both.
func NewSMB(cfg map[string]string, password []byte) (*SMB, error) {
	host := strings.TrimSuffix(strings.TrimPrefix(cfg["host"], "["), "]")
	if ip, err := netip.ParseAddr(host); err == nil {
		host = ip.String()
	} else if !smbHostName.MatchString(host) {
		return nil, fmt.Errorf("serverbackup: SMB target: host %q: a name or an address, without smb:// or \\\\", cfg["host"])
	}
	port := DefaultSMBPort
	if p := cfg["port"]; p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("serverbackup: SMB target: port %q: from 1 to 65535", p)
		}
		port = n
	}
	share := cfg["share"]
	if share == "" || len(share) > 80 || strings.ContainsAny(share, smbBadChars) || smbHasControl(share) {
		return nil, fmt.Errorf("serverbackup: SMB target: share %q: a share's name, without / or \\", share)
	}
	dir, err := smbDir(cfg["path"])
	if err != nil {
		return nil, err
	}
	user, domain := cfg["user"], cfg["domain"]
	if i := strings.IndexByte(user, '\\'); i >= 0 && domain == "" {
		domain, user = user[:i], user[i+1:]
	}
	if user == "" {
		return nil, errors.New("serverbackup: SMB target: a user is required")
	}
	t := &SMB{
		host: host, addr: net.JoinHostPort(host, strconv.Itoa(port)),
		share: share, dir: dir, user: user, domain: domain, password: string(password),
		timeout: smbTimeout, dial: dialSMB,
	}
	t.label = "//" + host
	if strings.Contains(host, ":") {
		t.label = "//[" + host + "]"
	}
	if port != DefaultSMBPort {
		t.label += ":" + strconv.Itoa(port)
	}
	t.label += "/" + share
	if dir != "" {
		t.label += "/" + strings.ReplaceAll(dir, `\`, "/")
	}
	return t, nil
}

// smbDir reads a folder in a share: its parts, separated by / or \,
// joined by \, without "." or "..", empty parts left out.
func smbDir(p string) (string, error) {
	var parts []string
	for part := range strings.FieldsFuncSeq(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == "." || part == ".." || len(part) > 255 || strings.ContainsAny(part, smbBadChars) || smbHasControl(part) ||
			strings.TrimRight(part, ". ") != part {
			return "", fmt.Errorf("serverbackup: SMB target: path %q: a folder in the share, its parts without . or .. or %s", p, `:*?"<>|`)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, `\`), nil
}

// smbHasControl reports whether s holds a control character.
func smbHasControl(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f })
}

// String is where the target is: //host[:port]/share[/path].
func (t *SMB) String() string { return t.label }

// GoString is String, so that %#v shows no password either.
func (t *SMB) GoString() string { return t.label }

// path is name's path in the share.
func (t *SMB) path(name string) string {
	if t.dir == "" {
		return name
	}
	return t.dir + `\` + name
}

// fail wraps an error of the server with the target.
func (t *SMB) fail(err error) error {
	return fmt.Errorf("serverbackup: SMB target %s: %w", t.label, err)
}

// Put writes r, size bytes, to name: to a hidden temporary file beside
// it, flushed and closed, then renamed to name, so that a put broken off
// leaves nothing under name; the temporary file is removed where the put
// fails, and one a crash left by the next put after a day. The folder is
// made where it is missing. Fewer or more bytes than size are refused.
func (t *SMB) Put(ctx context.Context, name string, r io.Reader, size int64) error {
	if err := CheckName(name); err != nil {
		return err
	}
	c, err := t.connect(ctx)
	if err != nil {
		return err
	}
	defer c.close()
	if err := t.mkdirAll(c); err != nil {
		return t.fail(err)
	}
	t.removeStaleParts(c, time.Now())
	tmp := "." + name + "." + smbRandomHex() + partSuffix
	if err := t.writePart(ctx, c, tmp, r, size); err != nil {
		t.removeQuietly(c, tmp)
		return t.fail(err)
	}
	if err := c.do(func() error { return c.share.Rename(t.path(tmp), t.path(name)) }); err != nil {
		t.removeQuietly(c, tmp)
		return t.fail(err)
	}
	return nil
}

// writePart writes r, size bytes, to a new file tmp, flushed and closed.
func (t *SMB) writePart(ctx context.Context, c *smbConn, tmp string, r io.Reader, size int64) error {
	var f smbFile
	if err := c.do(func() (err error) {
		f, err = c.share.OpenFile(t.path(tmp), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		return err
	}); err != nil {
		return err
	}
	n, err := c.copy(f, io.LimitReader(ctxReader{ctx, r}, size+1))
	if err == nil && n != size {
		err = fmt.Errorf("%s: %d bytes, not %d", tmp, n, size)
	}
	if err == nil {
		err = c.do(f.Sync)
	}
	if cerr := c.do(f.Close); err == nil {
		err = cerr
	}
	return err
}

// removeQuietly removes name where it can; a file left is a stale part
// the next put removes.
func (t *SMB) removeQuietly(c *smbConn, name string) {
	_ = c.do(func() error { return c.share.Remove(t.path(name)) })
}

// removeStaleParts removes the temporary files of puts a crash left,
// older than a day by the server's clock.
func (t *SMB) removeStaleParts(c *smbConn, now time.Time) {
	var fis []fs.FileInfo
	if c.do(func() (err error) { fis, err = c.share.ReadDir(t.dir); return err }) != nil {
		return
	}
	for _, fi := range fis {
		if strings.HasPrefix(fi.Name(), ".") && strings.HasSuffix(fi.Name(), partSuffix) &&
			fi.Mode().IsRegular() && now.Sub(fi.ModTime()) > partStale {
			t.removeQuietly(c, fi.Name())
		}
	}
}

// mkdirAll makes the target's folder and those above it where they are
// missing.
func (t *SMB) mkdirAll(c *smbConn) error {
	if t.dir == "" {
		return nil
	}
	if fi, err := t.stat(c, t.dir); err == nil && fi.IsDir() {
		return nil
	}
	parts := strings.Split(t.dir, `\`)
	for i := range parts {
		p := strings.Join(parts[:i+1], `\`)
		err := c.do(func() error { return c.share.Mkdir(p, 0o700) })
		if err == nil || errors.Is(err, fs.ErrExist) {
			continue
		}
		if fi, serr := t.stat(c, p); serr == nil && fi.IsDir() {
			continue
		}
		return err
	}
	return nil
}

// stat returns a file's or a folder's information.
func (t *SMB) stat(c *smbConn, p string) (fs.FileInfo, error) {
	var fi fs.FileInfo
	err := c.do(func() (err error) { fi, err = c.share.Stat(p); return err })
	return fi, err
}

// Open opens name for ranged reads, on a connection of its own that
// Close ends.
func (t *SMB) Open(ctx context.Context, name string) (File, int64, error) {
	if err := CheckName(name); err != nil {
		return nil, 0, err
	}
	c, err := t.connect(ctx)
	if err != nil {
		return nil, 0, err
	}
	var f smbFile
	if err := c.do(func() (err error) { f, err = c.share.OpenFile(t.path(name), os.O_RDONLY, 0); return err }); err != nil {
		c.close()
		return nil, 0, t.fail(err)
	}
	var fi fs.FileInfo
	err = c.do(func() (err error) { fi, err = f.Stat(); return err })
	if err == nil && !fi.Mode().IsRegular() {
		err = fmt.Errorf("%s is no regular file", name)
	}
	if err != nil {
		_ = c.do(f.Close)
		c.close()
		return nil, 0, t.fail(err)
	}
	return &smbReader{t: t, c: c, f: f}, fi.Size(), nil
}

// List lists the backups of the folder: its regular files whose names
// a backup may have; a folder not made yet holds none.
func (t *SMB) List(ctx context.Context) ([]Entry, error) {
	c, err := t.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.close()
	var fis []fs.FileInfo
	err = c.do(func() (err error) { fis, err = c.share.ReadDir(t.dir); return err })
	if errors.Is(err, fs.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, t.fail(err)
	}
	out := []Entry{}
	for _, fi := range fis {
		if CheckName(fi.Name()) != nil || !fi.Mode().IsRegular() {
			continue
		}
		out = append(out, Entry{Name: fi.Name(), Size: fi.Size(), Time: fi.ModTime()})
	}
	return out, nil
}

// Delete removes name; one not there is no error.
func (t *SMB) Delete(ctx context.Context, name string) error {
	if err := CheckName(name); err != nil {
		return err
	}
	c, err := t.connect(ctx)
	if err != nil {
		return err
	}
	defer c.close()
	err = c.do(func() error { return c.share.Remove(t.path(name)) })
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return t.fail(err)
	}
	return nil
}

// Check signs in, mounts the share, makes the folder where it is
// missing, and writes, reads back and deletes a small file in it: a
// target that cannot delete cannot prune.
func (t *SMB) Check(ctx context.Context) error {
	c, err := t.connect(ctx)
	if err != nil {
		return err
	}
	defer c.close()
	if err := t.mkdirAll(c); err != nil {
		return t.fail(err)
	}
	nonce := smbRandomHex()
	probe, want := ".check-"+nonce+partSuffix, "massimo "+nonce+"\n"
	if err := t.writePart(ctx, c, probe, strings.NewReader(want), int64(len(want))); err != nil {
		t.removeQuietly(c, probe)
		return t.fail(err)
	}
	got, err := t.readBack(c, probe, len(want))
	if err == nil && got != want {
		err = fmt.Errorf("%s reads back other bytes than were written", probe)
	}
	if err != nil {
		t.removeQuietly(c, probe)
		return t.fail(err)
	}
	if err := c.do(func() error { return c.share.Remove(t.path(probe)) }); err != nil {
		return t.fail(err)
	}
	return nil
}

// readBack reads up to n bytes of name.
func (t *SMB) readBack(c *smbConn, name string, n int) (string, error) {
	var f smbFile
	if err := c.do(func() (err error) { f, err = c.share.OpenFile(t.path(name), os.O_RDONLY, 0); return err }); err != nil {
		return "", err
	}
	b := make([]byte, n+1)
	var k int
	err := c.do(func() (err error) { k, err = f.ReadAt(b, 0); return err })
	if errors.Is(err, io.EOF) {
		err = nil
	}
	if cerr := c.do(f.Close); err == nil {
		err = cerr
	}
	return string(b[:k]), err
}

// smbRandomHex is 16 random hexadecimal digits, for a temporary name.
func smbRandomHex() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // crypto/rand.Read never fails
	return hex.EncodeToString(b)
}

// smbShare is what the target asks of a mounted share: *smb2.Share's
// calls of these names (smbMount), or the tests' fake. Paths are in the
// share, their parts joined by \.
type smbShare interface {
	OpenFile(name string, flag int, perm fs.FileMode) (smbFile, error)
	Stat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.FileInfo, error)
	Mkdir(name string, perm fs.FileMode) error
	Rename(oldpath, newpath string) error
	Remove(name string) error
	// Close unmounts the share, logs off and ends the connection.
	Close() error
}

// smbFile is a file of a share, open.
type smbFile interface {
	io.Writer
	io.ReaderAt
	Stat() (fs.FileInfo, error)
	Sync() error
	Close() error
}

// smbConn is one connection to the share, for one call of the target:
// each call on it runs with the timeout armed (do), and one that takes
// longer ends the connection.
type smbConn struct {
	share   smbShare
	cancel  context.CancelFunc
	timeout time.Duration
	stalled atomic.Bool
}

// connect dials the server, signs in and mounts the share, within the
// timeout; ctx bounds every call on the connection after it.
func (t *SMB) connect(ctx context.Context) (*smbConn, error) {
	cctx, cancel := context.WithCancel(ctx)
	c := &smbConn{cancel: cancel, timeout: t.timeout}
	if err := c.do(func() (err error) { c.share, err = t.dial(cctx, t); return err }); err != nil {
		cancel()
		return nil, t.fail(err)
	}
	return c, nil
}

// do runs one call on the connection with the timeout armed: a call that
// takes longer cancels the connection's context, which ends it and every
// call after it, and its error says so.
func (c *smbConn) do(f func() error) error {
	timer := time.AfterFunc(c.timeout, func() {
		c.stalled.Store(true)
		c.cancel()
	})
	err := f()
	timer.Stop()
	if err != nil && c.stalled.Load() {
		return fmt.Errorf("no answer within %s: %w", c.timeout, err)
	}
	return err
}

// copy writes src to f in chunks, each write a call of its own, and
// returns how many bytes it wrote; an error reading src says so.
func (c *smbConn) copy(f smbFile, src io.Reader) (int64, error) {
	buf := make([]byte, smbChunk)
	var n int64
	for {
		k, rerr := io.ReadFull(src, buf)
		if k > 0 {
			if err := c.do(func() error { _, err := f.Write(buf[:k]); return err }); err != nil {
				return n, err
			}
			n += int64(k)
		}
		switch {
		case errors.Is(rerr, io.EOF) || errors.Is(rerr, io.ErrUnexpectedEOF):
			return n, nil
		case rerr != nil:
			return n, fmt.Errorf("reading the backup: %w", rerr)
		}
	}
}

// close unmounts the share and logs off, then ends the connection. Its
// errors change nothing of a call done, and are not returned.
func (c *smbConn) close() {
	_ = c.share.Close()
	c.cancel()
}

// smbReader is a file opened for ranged reads, with its connection.
type smbReader struct {
	t    *SMB
	c    *smbConn
	f    smbFile
	once sync.Once
}

// ReadAt reads from the file at off. A read that reaches the end returns
// io.EOF itself, as io.ReaderAt asks: go-smb2 answers one that ends
// within p with the bytes it read and no error.
func (r *smbReader) ReadAt(p []byte, off int64) (int, error) {
	var n int
	err := r.c.do(func() (err error) { n, err = r.f.ReadAt(p, off); return err })
	switch {
	case err == nil && n < len(p), errors.Is(err, io.EOF):
		return n, io.EOF
	case err != nil:
		return n, r.t.fail(err)
	}
	return n, nil
}

// Close closes the file and ends its connection.
func (r *smbReader) Close() error {
	var err error
	r.once.Do(func() {
		err = r.c.do(r.f.Close)
		r.c.close()
	})
	if err != nil {
		return r.t.fail(err)
	}
	return nil
}

// smbMount is a share mounted with go-smb2, with its session and its
// TCP connection.
type smbMount struct {
	conn  net.Conn
	sess  *smb2.Session
	share *smb2.Share
}

// dialSMB connects to the target's server, negotiates SMB 2 or 3 with
// every message signed, signs in by NTLM and mounts the share; ctx
// bounds the connection and every call on it.
//
// It encrypts only where the server asks for it, for the session or the
// share: go-smb2 announces that it can encrypt, which a server that
// wants it answers by asking, but has no setting by which a client asks
// for encryption a server merely offers, as SMB 3 would let it. A share
// that does not ask is reached signed, not encrypted; a copy is
// encrypted by age in any case.
func dialSMB(ctx context.Context, t *SMB) (smbShare, error) {
	var nd net.Dialer
	conn, err := nd.DialContext(ctx, "tcp", t.addr)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	d := &smb2.Dialer{
		Negotiator: smb2.Negotiator{RequireMessageSigning: true},
		Initiator:  &smb2.NTLMInitiator{User: t.user, Password: t.password, Domain: t.domain},
	}
	sess, err := d.DialConn(ctx, conn, t.host)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("sign in as %s: %w", t.userName(), err)
	}
	sess = sess.WithContext(ctx)
	share, err := sess.Mount(`\\` + t.host + `\` + t.share)
	if err != nil {
		m := &smbMount{conn: conn, sess: sess}
		_ = m.Close()
		return nil, fmt.Errorf("mount the share %s: %w", t.share, err)
	}
	return &smbMount{conn: conn, sess: sess, share: share.WithContext(ctx)}, nil
}

// userName is the user as the server knows it, DOMAIN\user where a
// domain is given.
func (t *SMB) userName() string {
	if t.domain == "" {
		return t.user
	}
	return t.domain + `\` + t.user
}

// OpenFile opens a file of the share.
func (m *smbMount) OpenFile(name string, flag int, perm fs.FileMode) (smbFile, error) {
	f, err := m.share.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Stat returns a file's or a folder's information.
func (m *smbMount) Stat(name string) (fs.FileInfo, error) { return m.share.Stat(name) }

// ReadDir lists a folder; "" is the share's root.
func (m *smbMount) ReadDir(name string) ([]fs.FileInfo, error) { return m.share.ReadDir(name) }

// Mkdir makes a folder.
func (m *smbMount) Mkdir(name string, perm fs.FileMode) error { return m.share.Mkdir(name, perm) }

// Rename renames a file, over one of the new name.
func (m *smbMount) Rename(oldpath, newpath string) error { return m.share.Rename(oldpath, newpath) }

// Remove removes a file.
func (m *smbMount) Remove(name string) error { return m.share.Remove(name) }

// Close unmounts the share and logs off, within smbCloseTimeout whatever
// the connection's context, then closes the TCP connection.
func (m *smbMount) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), smbCloseTimeout)
	defer cancel()
	var errs []error
	if m.share != nil {
		errs = append(errs, m.share.WithContext(ctx).Umount())
	}
	errs = append(errs, m.sess.WithContext(ctx).Logoff())
	if err := m.conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
