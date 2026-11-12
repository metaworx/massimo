// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/server"
)

// packagedUpdate sets the update options of a box whose binary a package
// manager owns (install.Packaged), below root (the tests). Where apt owns
// it, the web UI's button asks the Debian package's update helper, which
// runs apt (update_apt.go): the channel is then the suite of the massimo
// source, read at each check, unless the store names another, to which
// the helper switches the source; and the helper must be on. Anywhere
// else, and where the source is missing or MASSIMO_UPDATE_CHANNEL names
// another channel, the manager's reason stays.
func packagedUpdate(upd *server.UpdateOptions, m install.Manager, root string, getenv func(string) string) {
	upd.Managed = m.String()
	if m.Name != "apt" {
		return
	}
	upd.Manager = m.Name
	source := filepath.Join(root, install.AptSource)
	suite, err := install.AptSuite(source)
	if err != nil {
		upd.Managed = fmt.Sprintf("%s; the update helper upgrades from the apt source %s, which is missing (packaging/README.md, \"The apt repository\")", m, install.AptSource)
		return
	}
	if c := getenv("MASSIMO_UPDATE_CHANNEL"); c != "" && c != suite {
		upd.Managed = fmt.Sprintf("apt follows the suite %s of %s; MASSIMO_UPDATE_CHANNEL=%s is not used on a box apt updates", suite, install.AptSource, c)
		return
	}
	upd.Managed, upd.Channel = "", getenv("MASSIMO_UPDATE_CHANNEL")
	upd.Suite = func() string {
		s, _ := install.AptSuite(source)
		return s
	}
	upd.Helper = func() string {
		if _, err := os.Lstat(filepath.Join(root, install.UpdatePathWant)); err != nil {
			return "the update helper is off: sudo massimo install turns it on, or upgrade by hand with " + m.Command
		}
		return ""
	}
}
