// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/serverbackups/restoring.ts
 *
 * The owner's restore of a server backup and the check with the key,
 * from a backup's row (AP ServerBackup, Block 5, D6-A, D7-A;
 * docs/api-v1.md, "Server backups"): the dialog's state and its
 * requests. The key, pasted or read from a file in the browser, is held
 * here while the dialog is open, sent in each request's body and stored
 * nowhere; closing the dialog, another page shown and a sign-out clear
 * it. With the key, POST …/{id}/restore/plan answers what the backup
 * holds: every scope with its counts, the Cubes by their serials, what is
 * missing, whether the env file and the TLS files can be put in place
 * and whether the server can restart itself. The owner ticks what to
 * restore; the plan of that choice is asked for before the confirmation,
 * which names what it replaces, and refuses there with its list; then
 * POST …/{id}/restore, a pinned backup taken first, refused with its
 * list or done, the server's restart followed (shared/restart.ts) and
 * the Cubes the Cube restore could write offered. POST …/{id}/check-key
 * gives the check's verdict. An answer after the dialog was closed or
 * after a sign-out shows nothing; while the restore's request runs the
 * dialog stays open.
 */

import type { ServerBackup, ServerRestorePlan, ServerRestoreReport, ServerRestoreRequest, ServerRestoreResult } from "../api";
import type { Outcome } from "../shared/action";
import { showCard } from "../shared/card";
import { live } from "../shared/live";
import { isShown, showView } from "../shared/pages";
import { followStart, RESTART_WAIT_MS, startedAt } from "../shared/restart";
import { session, stillSession } from "../shared/session";
import { loadShown } from "./send";

/** What the dialog does: restore, or check the backup with the key. */
export type RestoreMode = "restore" | "check";

/** The dialog's step: the key, the choice from the plan, the confirmation, the outcome. */
export type RestoreStep = "key" | "choose" | "confirm" | "done";

/** A row missing here that the backup's rows name, as a refusal lists it. */
export type MissingRow = NonNullable<ServerRestoreReport["missing"]>[number];

/** The verdict of the check with the key (POST …/{id}/check-key). */
export interface KeyCheck {
  /** The store is whole. */
  ok?: boolean;
  /** "ok", or what PRAGMA integrity_check found. */
  integrity?: string[];
  this_box?: boolean;
  files?: { name?: string; role?: string }[];
}

/** KEY_FILE_MAX is the largest file read as a key: an age identity is a line, an OpenSSH key a few lines. */
const KEY_FILE_MAX = 64 * 1024;

/** The scopes the dialog offers by name, in the plan's order; each Cube is offered by its serial (cube:<serial>). */
export const SCOPES = ["settings", "users", "integrations", "history"] as const;

/** The restore's dialog, for the backup it is open for, null while it is closed. */
export const rs = live({
  /** The backup the dialog is open for; null while it is closed. */
  backup: null as ServerBackup | null,
  /** What it does. */
  mode: "restore" as RestoreMode,
  /** Its step. */
  step: "key" as RestoreStep,
  /** The key pasted. */
  key: "",
  /** The key file read in the browser: its name and its text. */
  file: null as { name: string; text: string } | null,
  /** The plan of every scope, as the key opened the backup. */
  plan: null as ServerRestorePlan | null,
  /** The scopes ticked: settings, users, integrations, history and cube:<serial>. */
  scopes: [] as string[],
  /** The backup's identity ticked. */
  identity: false,
  /** The env file ticked. */
  env: false,
  /** The TLS files ticked. */
  tls: false,
  /** The plan of the choice, for the confirmation. */
  chosen: null as ServerRestorePlan | null,
  /** Why the choice, or every scope at first, would be refused, "" for not. */
  refused: "",
  /** The refusal is every scope's together, the first plan's, not the choice's. */
  refusedAll: false,
  /** The rows missing that the refusal lists. */
  missing: [] as MissingRow[],
  /** The restore's answer. */
  result: null as ServerRestoreResult | null,
  /** The check's verdict. */
  verdict: null as KeyCheck | null,
  /** How the dialog's last request went, where it failed. */
  said: null as Outcome | null,
  /** The restore's request runs: the dialog stays open. */
  running: false,
});

/** The server's restart after a restore, as the page follows it: its line, null for none, and whether it is followed now. */
export const restoreRestart = live({
  /** The line, said in the page's language at its drawing. */
  text: null as (() => string) | null,
  /** It went well so far. */
  ok: true,
  /** The page follows the server now. */
  on: false,
});

/** The dialog's opening: an answer to an earlier opening shows nothing. */
let opening = 0;

/** The end of the restart's following, null for none. */
let stopFollow: (() => void) | null = null;

/** clearDialog takes the dialog's state back, the key first. */
function clearDialog(): void {
  Object.assign(rs, {
    key: "", file: null, backup: null, mode: "restore", step: "key", plan: null, scopes: [], identity: false, env: false, tls: false, chosen: null,
    refused: "", refusedAll: false, missing: [], result: null, verdict: null, said: null, running: false,
  });
}

/** openRestore opens the dialog for a backup, to restore it or to check it with the key. */
export function openRestore(b: ServerBackup, mode: RestoreMode): void {
  opening++;
  clearDialog();
  rs.mode = mode;
  rs.backup = b;
}

/** closeRestore closes the dialog and clears the key, but while the restore's request runs. */
export function closeRestore(): void {
  if (rs.running) return;
  opening++;
  clearDialog();
}

/**
 * forgetRestore forgets the dialog and the restart's line at a sign-out
 * (MassimoParts.signedOut), the key first, and ends the following, whose
 * reads the session's end stops anyway.
 */
export function forgetRestore(): void {
  opening++;
  clearDialog();
  stopFollow?.();
  stopFollow = null;
  Object.assign(restoreRestart, { text: null, ok: true, on: false });
}

/** quietRestore takes the restart's line back as Maintenance is shown, but while the page follows the server. */
export function quietRestore(): void {
  if (!restoreRestart.on) Object.assign(restoreRestart, { text: null, ok: true });
}

/** keyText is the key as it is sent: the file read, else what is pasted. */
function keyText(): string {
  return rs.file ? rs.file.text : rs.key;
}

/** hasKey reports whether a key is given, pasted or read. */
export function hasKey(): boolean {
  return keyText().trim() !== "";
}

/** readKeyFile reads a key file in the browser into the dialog, never sent but with a request, never stored; one too large is refused. */
export async function readKeyFile(file: File): Promise<void> {
  const at = opening;
  rs.said = null;
  if (file.size > KEY_FILE_MAX) {
    rs.said = { text: t("serverbackup.restore.keyTooLarge", { name: file.name }), ok: false };
    return;
  }
  const text = await file.text();
  if (at !== opening) return;
  rs.file = { name: file.name, text };
}

/** forgetKeyFile drops the key file read, the field for a key pasted shown again. */
export function forgetKeyFile(): void {
  rs.file = null;
}

/** backupPath is the backup's path in the API. */
function backupPath(b: ServerBackup): string {
  return "api/v1/server-backups/" + encodeURIComponent(b.uuid || "");
}

/** bodyOf is a request's body: the key, the scopes and the files ticked. */
function bodyOf(scopes: string[]): ServerRestoreRequest {
  return { key_file: keyText(), scopes, take_identity: rs.identity, env: rs.env, tls: rs.tls };
}

/** A request's error, with the answer's status and body. */
type ApiError = Error & { status?: number; data?: (ServerRestoreResult & { code?: string; report?: ServerRestoreReport }) | null };

/** mine says, for a request begun now, whether its answer may still be shown: the same opening of the dialog, the same session. */
function mine(): () => boolean {
  const at = opening;
  const n = session();
  return () => at === opening && stillSession(n);
}

/** refuse shows a refusal in the choice with the rows it lists, all where it is every scope's together. */
function refuse(text: string, report?: ServerRestoreReport, all = false): void {
  rs.refused = text;
  rs.refusedAll = all && !!text;
  rs.missing = report?.missing || [];
}

/**
 * openBackup opens the backup with the key (POST …/restore/plan, every
 * scope, no file): the choice shows what it holds, nothing ticked, and
 * what every scope together would refuse. A key that does not open it
 * says so in the key's step.
 */
export async function openBackup(): Promise<boolean> {
  const b = rs.backup;
  if (!b) return false;
  const still = mine();
  rs.said = null;
  try {
    const p = await api<ServerRestorePlan>("POST", backupPath(b) + "/restore/plan", { key_file: keyText(), scopes: [] });
    if (!still()) return false;
    Object.assign(rs, { plan: p, scopes: [], identity: false, env: false, tls: false, chosen: null });
    refuse(p.refused || "", p.report, true);
    rs.step = "choose";
    return true;
  } catch (e) {
    if (still()) rs.said = { text: t("serverbackup.restore.openFailed", { error: (e as Error).message }), ok: false };
    return false;
  }
}

/** tick ticks a scope (settings, users, integrations, history or cube:<serial>) or takes it back, in the plan's order; the refusal shown goes. */
export function tick(scope: string, on: boolean): void {
  const order = [...SCOPES, ...(rs.plan?.cubes || []).map((c) => "cube:" + c.serial)];
  const set = new Set(rs.scopes);
  if (on) set.add(scope);
  else set.delete(scope);
  rs.scopes = order.filter((s) => set.has(s));
  refuse("");
}

/** tickFile ticks the identity, the env file or the TLS files, or takes it back; the refusal shown goes. */
export function tickFile(what: "identity" | "env" | "tls", on: boolean): void {
  rs[what] = on;
  refuse("");
}

/** chosenAny reports whether anything is ticked. */
export function chosenAny(): boolean {
  return rs.scopes.length > 0 || rs.identity || rs.env || rs.tls;
}

/**
 * planChoice asks the plan of what is ticked (POST …/restore/plan) and
 * goes on to the confirmation, or stays in the choice with the refusal
 * and its list. A choice of files alone has no plan of its own, since a
 * plan without scopes is every scope's: the first one's answer about
 * the files and the restart holds for it.
 */
export async function planChoice(): Promise<boolean> {
  const b = rs.backup;
  if (!b || !chosenAny()) return false;
  rs.said = null;
  if (!rs.scopes.length) {
    rs.chosen = null;
    rs.step = "confirm";
    return true;
  }
  const still = mine();
  try {
    const p = await api<ServerRestorePlan>("POST", backupPath(b) + "/restore/plan", bodyOf(rs.scopes));
    if (!still()) return false;
    rs.chosen = p;
    if (p.refused) {
      refuse(p.refused, p.report);
      return false;
    }
    refuse("");
    rs.step = "confirm";
    return true;
  } catch (e) {
    if (still()) rs.said = { text: t("serverbackup.restore.planFailed", { error: (e as Error).message }), ok: false };
    return false;
  }
}

/** backToChoice goes back from the confirmation to the choice. */
export function backToChoice(): void {
  rs.said = null;
  rs.step = "choose";
}

/** backToKey goes back from the choice to the key, the plan forgotten. */
export function backToKey(): void {
  Object.assign(rs, { said: null, step: "key", plan: null, chosen: null, scopes: [], identity: false, env: false, tls: false });
  refuse("");
}

/**
 * runRestore restores what is ticked (POST …/restore): the server's
 * start read first, so that its restart is followed from it. A refusal
 * of the store (409 with its report: rows missing, no owner left) goes
 * back to the choice with its list; another error stays in the
 * confirmation. A restore done, or one whose store changed before a file
 * failed (500 with the answer), shows its outcome; its restart started,
 * the page follows the server until it is back.
 */
export async function runRestore(): Promise<void> {
  const b = rs.backup;
  if (!b || rs.running) return;
  const n = session();
  rs.running = true;
  rs.said = null;
  try {
    const before = await startedAt();
    if (!stillSession(n)) return;
    let res: ServerRestoreResult;
    try {
      res = await api<ServerRestoreResult>("POST", backupPath(b) + "/restore", bodyOf(rs.scopes));
    } catch (e) {
      const err = e as ApiError;
      if (!stillSession(n)) return;
      if (err.status !== 500 || !err.data?.restart) {
        if (err.data?.code && err.data.report) {
          refuse(err.message, err.data.report);
          rs.step = "choose";
        } else {
          rs.said = { text: t("serverbackup.restore.failed", { error: err.message }), ok: false };
        }
        return;
      }
      res = err.data; // the store restored, a file not put in place: the server restarts all the same
    }
    if (!stillSession(n)) return;
    rs.result = res;
    rs.step = "done";
    if (res.restart?.started) followRestore(before);
    else void loadShown();
  } finally {
    if (stillSession(n)) rs.running = false;
  }
}

/**
 * followRestore follows the server through the restart a restore
 * started (followStart): its line says that it restarts, then since
 * when it is back, the card read again while Maintenance is shown, or
 * that it did not come back.
 */
function followRestore(before: string): void {
  stopFollow?.();
  Object.assign(restoreRestart, { text: () => t("restart.restarting"), ok: true, on: true });
  stopFollow = followStart(before, Date.now() + RESTART_WAIT_MS, (now) => {
    stopFollow = null;
    Object.assign(restoreRestart, { text: () => t("restart.back", { time: fmtInstant(new Date(Date.parse(now))) }), ok: true, on: false });
    if (isShown("maint")) void loadShown();
  }, () => {
    stopFollow = null;
    Object.assign(restoreRestart, { text: () => t("restart.notBack"), ok: false, on: false });
  });
}

/** runCheck checks the backup with the key (POST …/check-key): its verdict, or why it failed in the key's step. */
export async function runCheck(): Promise<boolean> {
  const b = rs.backup;
  if (!b) return false;
  const still = mine();
  rs.said = null;
  try {
    const v = await api<KeyCheck>("POST", backupPath(b) + "/check-key", { key_file: keyText() });
    if (!still()) return false;
    rs.verdict = v;
    rs.step = "done";
    return true;
  } catch (e) {
    if (still()) rs.said = { text: t("serverbackup.checkKey.failed", { error: (e as Error).message }), ok: false };
    return false;
  }
}

/**
 * toCubeRestore closes the dialog and shows the Cube restore of a Cube
 * the restore brought back: Cube settings, that Cube chosen, at its
 * Backup and restore, whose newest backup is the one restored. The page
 * is shown in a timer, once the dialog has given the focus back, so that
 * the card's heading keeps it.
 */
export function toCubeRestore(serial: string): void {
  closeRestore();
  setTimeout(() => {
    if (state.serial !== serial) selectCube(serial);
    showView("cube");
    showCard("cube-backup");
  }, 0);
}
