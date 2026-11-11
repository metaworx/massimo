// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package packaging carries the service files of this directory into the
// binary, so that `massimo install` writes exactly what is documented and
// reviewed here (see README.md).
package packaging

import "embed"

// Files holds systemd/ and launchd/ as they are in the
// repository, and env.template, the settings file a fresh install gets.
//
//go:embed env.template
//go:embed systemd/massimo.service systemd/massimo-update.path systemd/massimo-update.service
//go:embed systemd/massimo-reboot.path systemd/massimo-reboot.service launchd/ch.metaworx.massimo-reboot.plist
//go:embed launchd/ch.metaworx.massimo.plist launchd/ch.metaworx.massimo-update.plist
var Files embed.FS
