// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The server runs unprivileged and cannot replace its own binary. It asks
// for an update by writing a request file into its state directory; a
// systemd path unit sees the file and starts the update helper as root
// (packaging/systemd/massimo-update.*), which reads and removes it,
// finds, verifies and installs the release itself and reports in a status
// file the server reads. The request holds nothing but the channel, so
// the server can neither choose a version nor hand over a binary.
const (
	DefaultRequestPath = "/var/lib/massimo/update-request"
	DefaultStatusPath  = "/var/lib/massimo-update/status.json"
)

// WriteRequest asks the helper for an update to the newest release on
// the channel.
func WriteRequest(path, channel string) error {
	if !ValidChannel(channel) {
		return fmt.Errorf("invalid channel %q", channel)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(channel+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// TakeRequest reads and removes the request file; it reports false when
// there is none. The file lies in a directory the unprivileged user
// owns, so it is opened without following a symbolic link, must be a
// small regular file, and holds a channel name or nothing.
func TakeRequest(path string) (channel string, ok bool, err error) {
	f, err := openNoFollow(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		os.Remove(path)
		return "", false, fmt.Errorf("request %s: %w", path, err)
	}
	// Taken: removed once read (after closing, which Windows needs), so a
	// request arriving from then on starts another run.
	b, err := readRegular(f, path)
	f.Close()
	os.Remove(path)
	if err != nil {
		return "", false, err
	}
	if len(b) > 256 {
		return "", false, fmt.Errorf("request %s is too long", path)
	}
	channel = strings.TrimSpace(string(b))
	if channel != "" && !ValidChannel(channel) {
		return "", false, fmt.Errorf("request %s names an invalid channel", path)
	}
	return channel, true, nil
}

func readRegular(f *os.File, path string) ([]byte, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("request %s is not a regular file", path)
	}
	return io.ReadAll(io.LimitReader(f, 257))
}

// Status is what the helper reports about its last run.
type Status struct {
	State   string    `json:"state"` // checking, installing, restarting, done, up-to-date, failed, rolled-back
	From    string    `json:"from,omitempty"`
	To      string    `json:"to,omitempty"`
	Channel string    `json:"channel,omitempty"`
	Message string    `json:"message,omitempty"`
	Time    time.Time `json:"time"`
}

// Running reports whether the helper is in the middle of a run.
func (s Status) Running() bool {
	switch s.State {
	case "checking", "installing", "restarting":
		return s.Time.After(time.Now().Add(-15 * time.Minute)) // an older one died
	}
	return false
}

// WriteStatus replaces the status file atomically, readable by everyone.
func WriteStatus(path string, s Status) error {
	if path == "" {
		return nil
	}
	if s.Time.IsZero() {
		s.Time = time.Now()
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadStatus reads the status file; no file gives a zero Status.
func ReadStatus(path string) (Status, error) {
	var s Status
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(b, &s)
}
