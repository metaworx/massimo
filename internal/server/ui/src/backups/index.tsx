// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/backups/index.tsx
 *
 * Backup and restore as parts (AP UiParts, Block 10): Cube settings' card
 * with the chosen Cube's box, and System settings' with a fold per Cube,
 * each drawn once into its place and updated in place; what a Cube has is
 * kept by its serial, so that both show the same. The lists are read when
 * their page is shown, after every change and after a restore; a box
 * drawn on the page shown whose list has not loaded (a load onto the
 * page, which shows it before it knows its Cubes) reads it once, where no
 * read of it runs (Box, readUnloaded). After the served Cubes' folds,
 * System settings has one for each Cube the server keeps backups of and
 * does not serve (unserved), whose list the page's showing reads, and
 * their backups as theirs are. The plain code calls
 * MassimoParts.backups (app.js selectCube and its stream); the cards'
 * heads count the backups kept, System settings' those of the Cubes not
 * served too.
 */

import { useLayoutEffect } from "preact/hooks";
import { mounter, register } from "../shared/region";
import { CubeBackup, keptText, SystemBackups } from "./Backups";
import { type RestoreEvent, boxUI, forgetBackups, loadUnserved, of, refreshBackups, restoreEvent, resynced, unserved } from "./store";
import { cardBadges } from "../shared/card";
import { isShown, onView } from "../shared/pages";
import { can } from "../shared/me";
import { useLook } from "../shared/look";

/** systemSerials are the Cubes of System settings' folds: the served ones, then those not served with backups. */
function systemSerials(): string[] {
  return [...state.cubes.map((c) => c.serial), ...unserved().map((c) => c.serial)];
}

/** The badges of System settings' card: every fold's backups kept, once one list has come. */
function systemCount(): string {
  const kept = systemSerials().map((s) => of(s)).filter((b) => b.loaded && b.history);
  return kept.length ? tn("backup.badge.kept", kept.reduce((sum, b) => sum + b.list.length, 0)) : "";
}

/** System is System settings' card with its head's count. */
function System() {
  useLook();
  const count = systemCount();
  // the card's head counts every fold's backups
  useLayoutEffect(() => {
    cardBadges("sys-backup", count ? [{ text: count, kind: "mode" }] : []);
  }, [count]);
  return <SystemBackups />;
}

/** Page is Cube settings' card with its head's count, the chosen Cube's. */
function Page() {
  useLook();
  const count = state.serial ? keptText(state.serial) : "";
  // the card's head counts the chosen Cube's backups
  useLayoutEffect(() => {
    cardBadges("cube-backup", count ? [{ text: count, kind: "mode" }] : []);
  }, [count]);
  return <CubeBackup />;
}

const mountSystem = mounter("backups-cubes", () => <System />);
const mountPage = mounter("cube-backup-body", () => <Page />);

/** quiet takes every box's message back, as a page is shown. */
function quiet(): void {
  for (const s of systemSerials()) for (const place of ["cube", "sys"]) boxUI.of(place + ":" + s).said = null;
}

/** showPage draws Cube settings' card and reads the chosen Cube's list again. */
function showPage(): void {
  if (!can("backup")) return;
  mountPage();
  if (state.serial) refreshBackups(state.serial);
}

/** The functions the plain code calls, MassimoParts.backups. */
const backups = register("backups", {
  /** cubeChanged follows the Cube choice on Cube settings (app.js selectCube). */
  cubeChanged(): void {
    if (isShown("cube")) showPage();
  },
  /** restoreEvent takes the stream's restore_step and restore_done (app.js connectEvents). */
  restoreEvent(ev: RestoreEvent): void {
    restoreEvent(ev);
  },
  /** resynced takes a resync of the stream, which lost what a restore said meanwhile (app.js connectEvents). */
  resynced,
});

/** MassimoParts.backups, as index.ts declares it. */
export type BackupsPart = typeof backups;

/** signedOut forgets what the boxes kept of the user who was signed in (MassimoParts.signedOut, index.ts). */
export const signedOut = forgetBackups;

onView("system", () => {
  quiet();
  if (!can("backup")) return;
  mountSystem();
  for (const s of systemSerials()) refreshBackups(s);
  // a Cube not served that comes with it reads its list once its fold is drawn (Box)
  void loadUnserved();
});
onView("cube", () => {
  quiet();
  showPage();
});
