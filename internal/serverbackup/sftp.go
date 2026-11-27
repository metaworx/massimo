// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serverbackup

// The SFTP target (AP ServerBackup v1.0, Block 7, D8-A): a directory on
// a host that speaks SSH, reached in-process through
// golang.org/x/crypto/ssh and github.com/pkg/sftp. Each target has a key
// pair of its own, made when it is added (Kind.MakeSecret): its private
// part is the target's secret, sealed in the store, and its public line
// goes into the user's authorized_keys on the host. The host's key is
// pinned once the owner confirms its fingerprint (host_key); until then,
// and when it changes, every connection is refused with a HostKeyError
// before anything is sent (docs/store.md, "The SFTP target").

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// KindSFTP is the SFTP target's kind.
const KindSFTP = "sftp"

// The SFTP target's timeouts: to connect, for the handshake up to the
// SFTP session, and without a byte from the host, which the keepalives
// keep from striking a connection that is merely idle.
const (
	sftpDialTimeout = 30 * time.Second
	sftpHandshake   = 45 * time.Second
	sftpIdle        = 2 * time.Minute
	sftpKeepAlive   = 30 * time.Second
)

// sftpKeyComment ends the public line of a target's key, so that the
// owner knows it in authorized_keys.
const sftpKeyComment = "massimo-server-backup"

// sftpCheckName is the stem of the probe Check writes and removes.
const sftpCheckName = "massimo-check"

// sftpHostKeyAlgos are the host keys asked for, in this order, where no
// key line pins one: Ed25519 first, so that the fingerprint shown is the
// one of the key most hosts have; no certificate, and no RSA signature
// by SHA-1.
var sftpHostKeyAlgos = []string{
	ssh.KeyAlgoED25519,
	ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521,
	ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256,
}

func init() {
	Register(Kind{
		Name: KindSFTP,
		Fields: []Field{
			{Key: "host", Type: FieldText, Required: true},
			{Key: "port", Type: FieldNumber, Default: "22"},
			{Key: "user", Type: FieldText, Required: true},
			{Key: "path", Type: FieldText},
			{Key: "host_key", Type: FieldText},
		},
		New:        newSFTP,
		MakeSecret: MakeSFTPKey,
		Public:     SFTPPublicKey,
	})
}

// MakeSFTPKey makes an SFTP target's key pair: an Ed25519 key, its
// private part in OpenSSH's PEM form, the target's secret.
func MakeSFTPKey() (string, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("serverbackup: sftp key: %w", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, sftpKeyComment)
	if err != nil {
		return "", fmt.Errorf("serverbackup: sftp key: %w", err)
	}
	return string(pem.EncodeToMemory(block)), nil
}

// SFTPPublicKey is the public line of an SFTP target's key, as
// authorized_keys takes it: "ssh-ed25519 AAAA… massimo-server-backup".
func SFTPPublicKey(secret string) (string, error) {
	signer, err := ssh.ParsePrivateKey([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("serverbackup: sftp key: %w", err)
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	return line + " " + sftpKeyComment, nil
}

// HostKeyError is an SFTP host whose key is not the one confirmed. Where
// none is confirmed yet (Pinned is ""), the owner compares Fingerprint
// with the host's own and confirms it as the target's host_key; where
// the key changed, both are named, and nothing is sent to the host.
type HostKeyError struct {
	// Type is the host key's type, ssh-ed25519 for one.
	Type string
	// Fingerprint is the host's key's, "SHA256:" and unpadded base64, as
	// ssh-keygen -l shows it.
	Fingerprint string
	// Pinned is the fingerprint the target's host_key confirmed, "" where
	// none is.
	Pinned string
}

// Error says what the host showed and what to do.
func (e *HostKeyError) Error() string {
	if e.Pinned == "" {
		return fmt.Sprintf("the host's %s key is %s, not confirmed yet: compare it with "+
			"ssh-keygen -lf of the host's key (/etc/ssh/ssh_host_*_key.pub) and confirm it as host_key", e.Type, e.Fingerprint)
	}
	return fmt.Sprintf("the host's %s key %s is not the one confirmed, %s: refused; "+
		"where the host's key was changed on purpose, confirm the new one as host_key", e.Type, e.Fingerprint, e.Pinned)
}

// sftpTarget is a directory on an SFTP host.
type sftpTarget struct {
	addr   string // host:port
	user   string
	dir    string // the remote directory, relative to the login directory where not absolute
	where  string // user@host:port, for the errors
	signer ssh.Signer
	pinned string   // the host key's fingerprint confirmed, "" where none is
	algos  []string // the host key algorithms asked for
}

// newSFTP makes an SFTP target from its fields and its key, the secret.
func newSFTP(cfg map[string]string, secret []byte) (Target, error) {
	if len(secret) == 0 {
		return nil, errors.New(`serverbackup: sftp target: no key; save the target with "secret": "" to make one`)
	}
	signer, err := ssh.ParsePrivateKey(secret)
	if err != nil {
		return nil, fmt.Errorf("serverbackup: sftp target: its key: %w", err)
	}
	host := strings.TrimSuffix(strings.TrimPrefix(cfg["host"], "["), "]")
	if host == "" || strings.ContainsAny(host, "/@ \\") {
		return nil, fmt.Errorf("serverbackup: sftp target: host %q: want a name or an address", cfg["host"])
	}
	port := 22
	if v := cfg["port"]; v != "" {
		if port, err = strconv.Atoi(v); err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("serverbackup: sftp target: port %q: want 1 to 65535", v)
		}
	}
	user := cfg["user"]
	if user == "" {
		return nil, errors.New("serverbackup: sftp target: user is required")
	}
	dir := "."
	if v := cfg["path"]; v != "" {
		dir = path.Clean(v)
	}
	pinned, algos, err := parseHostKey(cfg["host_key"])
	if err != nil {
		return nil, fmt.Errorf("serverbackup: sftp target: host_key: %w", err)
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	return &sftpTarget{addr: addr, user: user, dir: dir, where: user + "@" + addr, signer: signer, pinned: pinned, algos: algos}, nil
}

// parseHostKey reads a host_key: a fingerprint as ssh-keygen -l shows it
// ("SHA256:…"), or the host's public key line as ssh-keyscan or
// known_hosts has it, which also asks for its kind of key alone. It
// returns the fingerprint and the host key algorithms to ask for.
func parseHostKey(v string) (string, []string, error) {
	v = strings.TrimSpace(v)
	switch {
	case v == "":
		return "", sftpHostKeyAlgos, nil
	case strings.HasPrefix(v, "SHA256:"):
		if raw, err := base64.RawStdEncoding.DecodeString(v[len("SHA256:"):]); err != nil || len(raw) != 32 {
			return "", nil, fmt.Errorf("%q is no SHA256 fingerprint", v)
		}
		return v, sftpHostKeyAlgos, nil
	}
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(v))
	if err != nil {
		return "", nil, errors.New("neither a SHA256 fingerprint nor a public key line")
	}
	algos := []string{key.Type()}
	if key.Type() == ssh.KeyAlgoRSA {
		algos = []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}
	}
	return ssh.FingerprintSHA256(key), algos, nil
}

// checkHostKey admits the host whose key is the one confirmed.
func (s *sftpTarget) checkHostKey(_ string, _ net.Addr, key ssh.PublicKey) error {
	if fp := ssh.FingerprintSHA256(key); s.pinned == "" || fp != s.pinned {
		return &HostKeyError{Type: key.Type(), Fingerprint: fp, Pinned: s.pinned}
	}
	return nil
}

// sftpConn is one connection to the host: SSH, the SFTP session over it,
// and the watch of its context, which closes it once the context is
// done.
type sftpConn struct {
	ssh   *ssh.Client
	sftp  *sftp.Client
	stop  func() bool
	done  chan struct{}
	close sync.Once
}

// Close ends the session and the connection.
func (c *sftpConn) Close() error {
	var err error
	c.close.Do(func() {
		c.stop()
		close(c.done)
		err = c.sftp.Close()
		c.ssh.Close()
	})
	return err
}

// idleConn fails a read or a write that waits longer than idle for the
// host.
type idleConn struct {
	net.Conn
	idle time.Duration
}

// Read reads with a deadline of idle from now.
func (c idleConn) Read(p []byte) (int, error) {
	if err := c.Conn.SetReadDeadline(time.Now().Add(c.idle)); err != nil {
		return 0, err
	}
	return c.Conn.Read(p)
}

// Write writes with a deadline of idle from now.
func (c idleConn) Write(p []byte) (int, error) {
	if err := c.Conn.SetWriteDeadline(time.Now().Add(c.idle)); err != nil {
		return 0, err
	}
	return c.Conn.Write(p)
}

// connect opens a connection to the host for as long as ctx lasts: the
// host's key checked before the target's key is offered, then the SFTP
// session.
func (s *sftpTarget) connect(ctx context.Context) (*sftpConn, error) {
	raw, err := (&net.Dialer{Timeout: sftpDialTimeout}).DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return nil, s.fail(ctx, err)
	}
	stop := context.AfterFunc(ctx, func() { raw.Close() })
	timer := time.AfterFunc(sftpHandshake, func() { raw.Close() })
	broken := func(err error) (*sftpConn, error) {
		timer.Stop()
		stop()
		raw.Close()
		return nil, s.fail(ctx, err)
	}
	cc, chans, reqs, err := ssh.NewClientConn(idleConn{raw, sftpIdle}, s.addr, &ssh.ClientConfig{
		User:              s.user,
		Auth:              []ssh.AuthMethod{ssh.PublicKeys(s.signer)},
		HostKeyCallback:   s.checkHostKey,
		HostKeyAlgorithms: s.algos,
	})
	if err != nil {
		return broken(err)
	}
	client := ssh.NewClient(cc, chans, reqs)
	// Writes go out without waiting for each one's answer, which a put
	// to a host far away needs; a write that fails leaves holes, but
	// only in a temporary file, which Put then removes.
	sc, err := sftp.NewClient(client, sftp.UseConcurrentWrites(true))
	if err != nil {
		client.Close()
		return broken(err)
	}
	if !timer.Stop() {
		sc.Close()
		client.Close()
		return broken(fmt.Errorf("no SFTP session within %s", sftpHandshake))
	}
	c := &sftpConn{ssh: client, sftp: sc, stop: stop, done: make(chan struct{})}
	go keepAlive(client, c.done)
	return c, nil
}

// keepAlive asks the host for an answer every sftpKeepAlive until done
// is closed or the connection fails, so that a connection merely idle
// is not taken for a host gone.
func keepAlive(client *ssh.Client, done <-chan struct{}) {
	t := time.NewTicker(sftpKeepAlive)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
			if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return
			}
		}
	}
}

// fail wraps an error with the target, saying what refused: the host's
// key, the target's key, or the context that ended.
func (s *sftpTarget) fail(ctx context.Context, err error) error {
	var hk *HostKeyError
	switch {
	case errors.As(err, &hk):
		return fmt.Errorf("serverbackup: sftp target %s: %w", s.where, hk)
	case ctx.Err() != nil:
		return fmt.Errorf("serverbackup: sftp target %s: %w", s.where, ctx.Err())
	case strings.Contains(err.Error(), "unable to authenticate"):
		return fmt.Errorf("serverbackup: sftp target %s: the host refused the target's key; put its public key into %s's authorized_keys: %w", s.where, s.user, err)
	}
	return fmt.Errorf("serverbackup: sftp target %s: %w", s.where, err)
}

// with runs f on a connection of its own, closed after.
func (s *sftpTarget) with(ctx context.Context, f func(c *sftp.Client) error) error {
	c, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := f(c.sftp); err != nil {
		return s.fail(ctx, err)
	}
	return nil
}

// file is a name in the target's directory.
func (s *sftpTarget) file(name string) string { return path.Join(s.dir, name) }

// mkdir makes the target's directory where it is missing.
func (s *sftpTarget) mkdir(c *sftp.Client) error {
	if s.dir == "." || s.dir == "/" {
		return nil
	}
	return c.MkdirAll(s.dir)
}

// partName is a temporary name beside name, hidden: ".<name>.<random>.part".
func partName(name string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return "." + name + "." + hex.EncodeToString(b) + partSuffix
}

// isPart reports whether a name is one of partName's, of a backup or of
// Check's probe, so that a stale one is ours to remove.
func isPart(n string) bool {
	if !strings.HasPrefix(n, ".") || !strings.HasSuffix(n, partSuffix) {
		return false
	}
	stem := strings.TrimSuffix(n[1:], partSuffix)
	i := strings.LastIndexByte(stem, '.')
	if i < 0 {
		return false
	}
	stem = stem[:i]
	return stem == sftpCheckName || CheckName(stem) == nil
}

// removeStaleParts removes the temporary files of puts broken off,
// older than a day; the others' files stay.
func (s *sftpTarget) removeStaleParts(c *sftp.Client, now time.Time) {
	entries, err := c.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Mode().IsRegular() && isPart(e.Name()) && now.Sub(e.ModTime()) > partStale {
			c.Remove(s.file(e.Name()))
		}
	}
}

// rename moves a temporary file to its name: in one step where the host
// offers posix-rename@openssh.com, which replaces the name, and else by
// removing the name first, which SFTP's own rename refuses to replace.
func (s *sftpTarget) rename(c *sftp.Client, from, to string) error {
	if _, ok := c.HasExtension("posix-rename@openssh.com"); ok {
		return c.PosixRename(from, to)
	}
	if err := c.Remove(to); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return c.Rename(from, to)
}

// Put writes r, size bytes, to name: to a hidden temporary file beside
// it, mode 0600 where the host takes it, then renamed to name, so that
// a put broken off leaves nothing under name. Fewer or more bytes than
// size are refused.
func (s *sftpTarget) Put(ctx context.Context, name string, r io.Reader, size int64) error {
	if err := CheckName(name); err != nil {
		return err
	}
	return s.with(ctx, func(c *sftp.Client) error {
		if err := s.mkdir(c); err != nil {
			return err
		}
		s.removeStaleParts(c, time.Now())
		tmp := s.file(partName(name))
		f, err := c.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
		if err != nil {
			return err
		}
		done := false
		defer func() {
			if !done {
				f.Close()
				c.Remove(tmp)
			}
		}()
		f.Chmod(0o600)
		n, err := io.Copy(f, io.LimitReader(ctxReader{ctx, r}, size+1))
		switch {
		case err != nil:
			return fmt.Errorf("%s: %w", name, err)
		case n != size:
			return fmt.Errorf("%s: %d bytes, not %d", name, n, size)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := s.rename(c, tmp, s.file(name)); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		done = true
		return nil
	})
}

// sftpFile is a copy opened for ranged reads, on a connection of its
// own, which Close ends.
type sftpFile struct {
	f *sftp.File
	c *sftpConn
}

// ReadAt reads len(p) bytes at off.
func (f *sftpFile) ReadAt(p []byte, off int64) (int, error) { return f.f.ReadAt(p, off) }

// Close closes the file and its connection.
func (f *sftpFile) Close() error {
	err := f.f.Close()
	return errors.Join(err, f.c.Close())
}

// Open opens name for ranged reads while ctx lasts.
func (s *sftpTarget) Open(ctx context.Context, name string) (File, int64, error) {
	if err := CheckName(name); err != nil {
		return nil, 0, err
	}
	c, err := s.connect(ctx)
	if err != nil {
		return nil, 0, err
	}
	f, err := c.sftp.Open(s.file(name))
	if err != nil {
		c.Close()
		return nil, 0, s.fail(ctx, err)
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		f.Close()
		c.Close()
		return nil, 0, fmt.Errorf("serverbackup: sftp target %s: %s is no regular file", s.where, name)
	}
	return &sftpFile{f: f, c: c}, fi.Size(), nil
}

// List lists the backups of the directory: its regular files whose
// names a backup may have; a directory not made yet holds none.
func (s *sftpTarget) List(ctx context.Context) ([]Entry, error) {
	out := []Entry{}
	err := s.with(ctx, func(c *sftp.Client) error {
		entries, err := c.ReadDirContext(ctx, s.dir)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			if CheckName(e.Name()) == nil && e.Mode().IsRegular() {
				out = append(out, Entry{Name: e.Name(), Size: e.Size(), Time: e.ModTime()})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Delete removes name; one not there is no error.
func (s *sftpTarget) Delete(ctx context.Context, name string) error {
	if err := CheckName(name); err != nil {
		return err
	}
	return s.with(ctx, func(c *sftp.Client) error {
		if err := c.Remove(s.file(name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	})
}

// Check reaches the host, its key confirmed, logs in with the target's
// key, makes the directory where it is missing, and writes, reads back
// and removes a probe in it.
func (s *sftpTarget) Check(ctx context.Context) error {
	return s.with(ctx, func(c *sftp.Client) error {
		if err := s.mkdir(c); err != nil {
			return fmt.Errorf("%s: %w", s.dir, err)
		}
		probe := s.file(partName(sftpCheckName))
		want := []byte("massimo\n")
		f, err := c.OpenFile(probe, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
		if err != nil {
			return fmt.Errorf("%s is not writable: %w", s.dir, err)
		}
		defer c.Remove(probe)
		_, werr := f.Write(want)
		if err := errors.Join(werr, f.Close()); err != nil {
			return fmt.Errorf("%s: %w", s.dir, err)
		}
		f, err = c.Open(probe)
		if err != nil {
			return fmt.Errorf("%s: %w", s.dir, err)
		}
		got, rerr := io.ReadAll(io.LimitReader(f, int64(len(want))+1))
		f.Close()
		switch {
		case rerr != nil:
			return fmt.Errorf("%s: %w", s.dir, rerr)
		case !bytes.Equal(got, want):
			return fmt.Errorf("%s: the probe read back is not the one written", s.dir)
		}
		return nil
	})
}
