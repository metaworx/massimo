// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/update"
)

// packagedUpdate sets the update options of a box whose binary a package
// manager owns (install.Packaged), below root (the tests). Where apt owns
// it, the web UI's button asks the Debian package's update helper, which
// runs apt (update_apt.go): the channel is then the suite of the massimo
// source, read at each check, unless the store names another, to which
// the helper switches the source; and the helper must be on. Anywhere
// else, and where the source is missing or MASSIMO_UPDATE_CHANNEL names
// another channel, the manager's reason stays. A release is offered once
// apt's index has it (aptNewest).
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
	upd.Source = func(ctx context.Context, channel string) (string, error) {
		return aptNewest(ctx, source, channel, runtime.GOARCH)
	}
}

// aptNewest is the newest massimo in the apt index the next update
// fetches, as a tag: the suite of channel where that is an update
// channel, as the helper switches the source to it (aptSwitch), else the
// source's own; of the source's repository and component, for goarch.
func aptNewest(ctx context.Context, source, channel, goarch string) (string, error) {
	f, err := install.ReadAptSource(source)
	if err != nil {
		return "", err
	}
	suite := f.Suite
	if slices.Contains(update.Flavours, channel) {
		suite = channel
	}
	component := f.Component
	if component == "" {
		component = "main"
	}
	arch := update.AptArch(goarch)
	if arch == "" {
		return "", fmt.Errorf("the apt repository has no packages for %s", goarch)
	}
	return update.AptNewest(ctx, nil, f.URI, suite, component, arch)
}
