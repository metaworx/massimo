// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, Backup and restore: its plain names, which call its
// parts (ui/src/backups/, MassimoParts.backups): a box per Cube, on Cube
// settings the chosen Cube's, in System settings one fold per Cube.
"use strict";

// renderBackups draws every box again, after a change of language (app.js
// applyLanguage).
function renderBackups() {
  MassimoParts.backups.render();
}

// backupCubeChanged follows the Cube choice on Cube settings (app.js
// selectCube).
function backupCubeChanged() {
  MassimoParts.backups.cubeChanged();
}

// restoreEvent takes the stream's restore_step and restore_done of a
// restore a box sent (app.js).
function restoreEvent(ev) {
  MassimoParts.backups.restoreEvent(ev);
}

// restoreResynced takes a resync of the stream (app.js), after which a
// restore's end that it lost will not come.
function restoreResynced() {
  MassimoParts.backups.resynced();
}
