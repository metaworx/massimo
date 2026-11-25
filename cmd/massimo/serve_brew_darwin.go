// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"go.metaworx.ch/massimo/internal/install"
	"go.metaworx.ch/massimo/internal/server"
	"go.metaworx.ch/massimo/internal/update"
)

// serveBrewUpdate lets the web UI's button update through brew where
// brew services runs the server as its user's job (update_brew.go): the
// server starts the helper itself, with the request and status files in
// the prefix's var/massimo and its log in var/log. Run as root, by hand
// or with another channel, the manager's reason stays, with why. A
// release is offered once the tap's formula has it (brewFormulaURL); a
// keg whose tap is not the project's is offered GitHub's.
func serveBrewUpdate(log *slog.Logger, upd *server.UpdateOptions, m install.Manager, listen string, tls bool) {
	job := os.Getenv("XPC_SERVICE_NAME")
	exe, err := brewExecutable()
	var k brewKeg
	if err == nil {
		k, err = brewHelperPlan(exe, os.Geteuid(), job)
	}
	if err == nil {
		if c := os.Getenv("MASSIMO_UPDATE_CHANNEL"); c != "" && c != flavour {
			err = fmt.Errorf("brew follows the channel %s of the formula %s; MASSIMO_UPDATE_CHANNEL=%s is not used under brew", flavour, k.formula, c)
		}
	}
	var health string
	if err == nil {
		health, err = install.HealthURL(listen, tls)
	}
	if err != nil {
		upd.Managed = fmt.Sprintf("%s (%v)", m, err)
		log.Info("updates cannot be started from the API", "reason", upd.Managed)
		return
	}
	dir := filepath.Join(k.prefix, "var", "massimo")
	upd.Request, upd.Status = filepath.Join(dir, "update-request"), filepath.Join(dir, "update-status.json")
	restart := fmt.Sprintf("gui/%d/%s", os.Getuid(), job)
	upd.Start = brewHelperStarter(log, exe, upd.Request, upd.Status, restart, health, filepath.Join(k.prefix, "var", "log", "massimo-update.log"))
	upd.Managed, upd.Manager, upd.Channel = "", "brew", flavour
	if formula, err := brewFormulaURL(k); err != nil {
		log.Info("the tap's formula is not read: the release on GitHub is offered", "reason", err)
	} else {
		upd.Source = func(ctx context.Context, _ string) (string, error) { return update.FormulaNewest(ctx, nil, formula) }
	}
}
