// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/backups/index.tsx
 *
 * Backup and restore as parts (AP UiParts, Block 10): Cube settings' card
 * with the chosen Cube's box, and System settings' with a fold per Cube,
 * each drawn once into its place and updated in place; what a Cube has is
 * kept by its serial, so that both show the same. The lists are read when
 * their page is shown, after every change and after a restore. The plain
 * code calls MassimoParts.backups (backup.js renderBackups,
 * backupCubeChanged, restoreEvent and restoreResynced); the cards' heads count the
 * backups kept.
 */

import { useEffect } from "preact/hooks";
import { mounter, register } from "../shared/region";
import { CubeBackup, keptText, SystemBackups } from "./Backups";
import { type RestoreEvent, boxUI, forgetBackups, load, of, refreshBackups, restoreEvent, resynced } from "./store";

/** The badges of System settings' card: every Cube's backups kept, once one list has come. */
function systemCount(): string {
  const kept = state.cubes.map((c) => of(c.serial)).filter((b) => b.loaded && b.history);
  return kept.length ? tn("backup.badge.kept", kept.reduce((sum, b) => sum + b.list.length, 0)) : "";
}

/** System is System settings' card with its head's count. */
function System() {
  load.useLoad();
  const count = systemCount();
  // the card's head counts every Cube's backups
  useEffect(() => {
    sectionBadges($("sys-backup"), count ? [badgeEl(count, "mode")] : []);
  }, [count]);
  return <SystemBackups />;
}

/** Page is Cube settings' card with its head's count, the chosen Cube's. */
function Page() {
  load.useLoad();
  const count = state.serial ? keptText(state.serial) : "";
  // the card's head counts the chosen Cube's backups
  useEffect(() => {
    sectionBadges($("cube-backup"), count ? [badgeEl(count, "mode")] : []);
  }, [count]);
  return <CubeBackup />;
}

const mountSystem = mounter("backups-cubes", () => <System />);
const mountPage = mounter("cube-backup-body", () => <Page />);

/** quiet takes every box's message back, as a page is shown. */
function quiet(): void {
  for (const c of state.cubes) for (const place of ["cube", "sys"]) boxUI.of(place + ":" + c.serial).said = null;
}

/** showPage draws Cube settings' card and reads the chosen Cube's list again. */
function showPage(): void {
  if (!hasCap("backup")) return;
  mountPage();
  load.bump();
  if (state.serial) refreshBackups(state.serial);
}

/** The functions the plain code calls, MassimoParts.backups. */
const backups = register("backups", {
  /** render draws every box again, after a change of language (backup.js renderBackups). */
  render(): void {
    load.bump();
  },
  /** cubeChanged follows the Cube choice on Cube settings (backup.js backupCubeChanged). */
  cubeChanged(): void {
    if (isShown("cube")) showPage();
  },
  /** restoreEvent takes the stream's restore_step and restore_done (backup.js restoreEvent). */
  restoreEvent(ev: RestoreEvent): void {
    restoreEvent(ev);
  },
  /** resynced takes a resync of the stream, which lost what a restore said meanwhile (backup.js restoreResynced). */
  resynced,
});

/** MassimoParts.backups, as index.ts declares it. */
export type BackupsPart = typeof backups;

/** signedOut forgets what the boxes kept of the user who was signed in (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetBackups;

onView("system", () => {
  quiet();
  if (!hasCap("backup")) return;
  mountSystem();
  load.bump();
  for (const c of state.cubes) refreshBackups(c.serial);
});
onView("cube", () => {
  quiet();
  showPage();
});
