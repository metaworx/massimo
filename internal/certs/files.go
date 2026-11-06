// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package certs

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Files serves a certificate from two PEM files, the certificate with
// its chain after it (as every ACME client writes it) and the key,
// re-read when either changes: a stat of both per handshake, which is
// microseconds, so that a renewal by certbot, acme.sh or win-acme is
// served at the next handshake with no signal and no restart. A changed
// pair that does not load is logged once and the loaded one kept.
type Files struct {
	source            string
	certPath, keyPath string
	log               *slog.Logger

	mu                sync.Mutex
	cert              *tls.Certificate
	status            Status
	certInfo, keyInfo fileInfo
	warned            bool // the files changed and do not load, or cannot be read; said once
}

// fileInfo is what a change of a file shows in a stat.
type fileInfo struct {
	size int64
	mod  time.Time
}

// OpenFiles loads the pair. A file that is missing or does not load is
// an error: a server without a certificate cannot start.
func OpenFiles(certPath, keyPath string, log *slog.Logger) (*Files, error) {
	return openFiles("files", certPath, keyPath, log)
}

func openFiles(source, certPath, keyPath string, log *slog.Logger) (*Files, error) {
	if log == nil {
		log = slog.Default()
	}
	f := &Files{source: source, certPath: certPath, keyPath: keyPath, log: log}
	ci, ki, err := f.stat()
	if err != nil {
		return nil, fmt.Errorf("tls: %w", err)
	}
	cert, st, err := Load(source, certPath, keyPath)
	if err != nil {
		return nil, err
	}
	f.cert, f.status, f.certInfo, f.keyInfo = cert, st, ci, ki
	return f, nil
}

// Load reads a certificate and key pair from PEM files and describes it.
func Load(source, certPath, keyPath string) (*tls.Certificate, Status, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, Status{}, fmt.Errorf("tls: %w", err)
	}
	st, err := Describe(source, &cert)
	if err != nil {
		return nil, Status{}, fmt.Errorf("tls: %s: %w", certPath, err)
	}
	return &cert, st, nil
}

// Paths are the files served.
func (f *Files) Paths() (certPath, keyPath string) { return f.certPath, f.keyPath }

func (f *Files) stat() (ci, ki fileInfo, err error) {
	for _, s := range []struct {
		path string
		info *fileInfo
	}{{f.certPath, &ci}, {f.keyPath, &ki}} {
		fi, err := os.Stat(s.path)
		if err != nil {
			return ci, ki, err
		}
		*s.info = fileInfo{size: fi.Size(), mod: fi.ModTime()}
	}
	return ci, ki, nil
}

// GetCertificate serves the pair, re-read when the files changed.
func (f *Files) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reloadLocked()
	return f.cert, nil
}

// Check warns under WarnBefore of the end, in the status and the log,
// for a pair someone else renews.
func (f *Files) Check(now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reloadLocked()
	f.status.Warning = expiryWarning(f.status.NotAfter, now)
	if f.status.Warning != "" {
		f.log.Warn("tls: "+f.status.Warning, "cert", f.certPath, "expires", f.status.NotAfter)
	}
}

// Reload re-reads the files now, for a pair just written.
func (f *Files) Reload() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reloadLocked()
}

// Status describes the pair served.
func (f *Files) Status() Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reloadLocked()
	return f.status
}

// reloadLocked re-reads the files when their size or time changed.
func (f *Files) reloadLocked() {
	ci, ki, err := f.stat()
	if err != nil {
		f.warnOnce("tls: certificate files cannot be read; keeping the loaded certificate", err)
		return
	}
	if ci == f.certInfo && ki == f.keyInfo {
		f.warned = false
		return
	}
	f.certInfo, f.keyInfo = ci, ki
	cert, st, err := Load(f.source, f.certPath, f.keyPath)
	if err != nil {
		f.warnOnce("tls: certificate files changed but do not load; keeping the loaded certificate", err)
		return
	}
	f.warned = false
	st.Warning = f.status.Warning
	f.cert, f.status = cert, st
	f.log.Info("tls: certificate reloaded", "names", st.Names, "expires", st.NotAfter, "fingerprint", st.Fingerprint)
}

func (f *Files) warnOnce(msg string, err error) {
	if f.warned {
		return
	}
	f.warned = true
	f.log.Warn(msg, "cert", f.certPath, "key", f.keyPath, "error", err)
}
