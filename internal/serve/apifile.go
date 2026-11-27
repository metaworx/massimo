// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package serve

// The API's address as serve publishes it in its data directory, for the
// commands that go through a running serve (massimo store backup,
// backups, push, pull): the address it listens on, however it was given
// (-listen, MASSIMO_LISTEN in the settings file or a systemd drop-in,
// the Windows service's arguments), and the node it serves, which a
// command compares with the store's before it sends a key.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/safefile"
)

// APIFileName is the file in the data directory that names a running
// serve's API, removed when it stops; one a crash left names an address
// that no longer answers.
const APIFileName = "serve.api"

// APIFile is what serve.api holds.
type APIFile struct {
	// URL is the API's base, http(s)://host:port, its host this box's
	// loopback where serve listens on every interface.
	URL string `json:"url"`
	// Node is the store's node.id, which GET /healthz says too.
	Node string `json:"node"`
	PID  int    `json:"pid"`
}

// APIFilePath is serve.api in the data directory dir.
func APIFilePath(dir string) string { return filepath.Join(dir, APIFileName) }

// ReadAPIFile reads serve.api of the data directory dir; fs.ErrNotExist
// where no serve published one.
func ReadAPIFile(dir string) (APIFile, error) {
	b, err := os.ReadFile(APIFilePath(dir))
	if err != nil {
		return APIFile{}, err
	}
	var f APIFile
	if err := json.Unmarshal(b, &f); err != nil || f.URL == "" {
		return APIFile{}, fmt.Errorf("serve: %s: not a serve's API file", APIFilePath(dir))
	}
	return f, nil
}

// publishAPI writes serve.api for the listener's address, and returns
// what removes it; an address no /healthz can be asked at, or a file
// that cannot be written, is a warning and nothing more.
func (i *Instance) publishAPI() func() {
	if i.st == nil {
		return func() {}
	}
	health, err := install.HealthURL(i.Addr(), i.src != nil)
	if err != nil {
		i.log.Warn("the API's address not published for the commands", "error", err)
		return func() {}
	}
	b, _ := json.Marshal(APIFile{URL: health[:len(health)-len("/healthz")], Node: i.st.NodeID(), PID: os.Getpid()})
	p := APIFilePath(i.st.Dir())
	if err := safefile.Write(p, append(b, '\n'), safefile.Options{Mode: 0o600}); err != nil {
		i.log.Warn("the API's address not published for the commands", "path", p, "error", err)
		return func() {}
	}
	return func() {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			i.log.Warn("serve.api not removed", "path", p, "error", err)
		}
	}
}
