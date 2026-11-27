// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/serverbackups/send.ts
 *
 * What Maintenance → Server backups reads and sends (docs/api-v1.md,
 * "Server backups"): the list, for a holder of backup.retry; the
 * targets, the keys and the kinds, for a holder of backup.server, the
 * kinds once; a backup now, a file uploaded, a backup downloaded,
 * checked, pinned or unpinned, its copy pushed, pulled or deleted
 * (backup.server); a failed backup or copy retried (backup.retry); a
 * target added, changed, tested or removed (backup.server); a key
 * pasted or made, confirmed by its last six characters, or removed (the
 * owner's). Each action is this page's own request: it reads again what
 * it changed, and says how it went in the line of its place, a job's
 * steps that failed among it; an event of the stream that a job of the
 * server sent reads the list again while Maintenance is shown, but not
 * while one of this page's own runs, whose reading follows anyway. An
 * answer after a sign-out is dropped (shared/session.ts).
 */

import type { ServerBackup, ServerBackupCopy, ServerBackupKey, ServerBackupKind, ServerBackupList, ServerBackupResult, ServerBackupStep, ServerBackupTarget, ServerBackupTargetState } from "../api";
import type { ServerEvent } from "../model/event";
import { actionRunner, type Outcome } from "../shared/action";
import { burstReader } from "../shared/burst";
import { forgetForms } from "../shared/forms";
import { can } from "../shared/me";
import { isShown } from "../shared/pages";
import { saveResponse } from "../shared/save";
import { session, stillSession } from "../shared/session";
import { fmtWhen } from "../shared/time";
import { backupUI, card, sb, targetFormKey, targetUI } from "./store";

/** The answer of a key added: the key, and for one made its secret part and the key file, once. */
interface KeyAdded {
  key: ServerBackupKey;
  identity?: string;
  key_file?: string;
}

/** The answer of a target added: the target, the backups found on it, the files skipped, and why it could not be read. */
interface TargetAdded {
  target: ServerBackupTarget;
  found?: number;
  skipped?: string[];
  scan_error?: string;
}

/** This page's own requests under way: an event's read waits for their reading. */
const own = { n: 0 };

const BACKUPS = "api/v1/server-backups";
const TARGETS = "api/v1/server-backup-targets";
const KEYS = "api/v1/server-backup-keys";

/** loadList reads the backups, the targets' states, the keys set and the daily time, for a holder of backup.retry; a backup gone forgets its fold. */
export async function loadList(): Promise<void> {
  if (!can("backup.retry")) return;
  const n = session();
  try {
    const list = (await api<ServerBackupList>("GET", BACKUPS)) || {};
    if (!stillSession(n)) return;
    const now = new Set((list.backups || []).map((b) => b.uuid));
    for (const b of sb.list?.backups || []) if (!now.has(b.uuid)) backupUI.forget(b.uuid || "");
    sb.list = list;
    sb.failed = "";
  } catch (e) {
    if (stillSession(n)) sb.failed = (e as Error).message;
  }
}

/** loadTargets reads the targets with their fields, for a holder of backup.server; a target gone forgets its form and its panel. */
export async function loadTargets(): Promise<void> {
  if (!can("backup.server")) return;
  const n = session();
  try {
    const list = (await api<ServerBackupTarget[]>("GET", TARGETS)) || [];
    if (!stillSession(n)) return;
    const now = new Set(list.map((tg) => tg.uuid));
    for (const tg of sb.targets) {
      if (now.has(tg.uuid)) continue;
      forgetForms(targetFormKey(tg.uuid || ""));
      targetUI.forget(tg.uuid || "");
    }
    sb.targets = list;
    sb.targetsLoaded = true;
  } catch (e) {
    if (stillSession(n)) card.targets = { text: t("serverbackup.readFailed", { error: (e as Error).message }), ok: false };
  }
}

/** loadKeys reads the keys, for a holder of backup.server; the key made is forgotten once it is no longer pending. */
export async function loadKeys(): Promise<void> {
  if (!can("backup.server")) return;
  const n = session();
  try {
    const list = (await api<ServerBackupKey[]>("GET", KEYS)) || [];
    if (!stillSession(n)) return;
    const made = card.made;
    if (made && !list.some((k) => k.uuid === made.uuid && k.pending)) card.made = null; // confirmed, removed or its hour over: its secret part goes
    sb.keys = list;
    sb.keysLoaded = true;
  } catch (e) {
    if (stillSession(n)) card.keys = { text: t("serverbackup.readFailed", { error: (e as Error).message }), ok: false };
  }
}

/** loadKinds reads the kinds of target and their fields once, for a holder of backup.server: they change only with the server, whose new version loads the page again. */
export async function loadKinds(): Promise<void> {
  if (sb.kinds || !can("backup.server")) return;
  const n = session();
  try {
    const kinds = (await api<ServerBackupKind[]>("GET", TARGETS + "/kinds")) || [];
    if (stillSession(n)) sb.kinds = kinds;
  } catch (e) {
    if (stillSession(n)) card.targets = { text: t("serverbackup.readFailed", { error: (e as Error).message }), ok: false };
  }
}

/** loadShown reads everything the card shows, as Maintenance is shown: each read asks for the capability it needs. */
export function loadShown(): Promise<unknown> {
  return Promise.all([loadList(), loadTargets(), loadKeys(), loadKinds()]);
}

/** reloadJob reads again what a backup's action changes: the list, and the targets, whose last copy it moves. */
function reloadJob(): Promise<unknown> {
  return Promise.all([loadList(), loadTargets()]);
}

/** reloadKeys reads again what a key's action changes: the keys, and the list, which counts them. */
function reloadKeys(): Promise<unknown> {
  return Promise.all([loadKeys(), loadList()]);
}

/**
 * act sends fn's request as this page's own, reads again (reload), and
 * says through set how it went: what said makes of the answer, or what
 * fail makes of the error, a text key with {error}. It resolves to
 * whether the request was taken.
 */
async function act<T>(set: (res: Outcome | null) => void, reload: () => Promise<unknown>, fn: () => Promise<T>, said: (r: T) => Outcome, fail: string): Promise<boolean> {
  let answer: T | undefined;
  own.n++;
  try {
    const runner = actionRunner({ set, reload, bound: true });
    const ok = await runner.run(async () => {
      answer = await fn();
    }, "", (error) => t(fail, { error }));
    if (ok) set(said(answer as T));
    return ok;
  } finally {
    own.n--;
  }
}

/** stepsSaid is a job's outcome: ok where every step went through, else ok's text with each step that failed, the target and the error. */
function stepsSaid(steps: ServerBackupStep[] | undefined, ok: string): Outcome {
  const bad = (steps || []).filter((s) => s.ok === false);
  if (!bad.length) return { text: ok, ok: true };
  const lines = bad.map((s) => (s.target ? t("serverbackup.stepFailed", { target: s.target, error: s.error || "" }) : s.error || ""));
  return { text: [ok, ...lines].join(" "), ok: false };
}

/** sayCard says a backup's outcome in the line under Back up now. */
const sayCard = (res: Outcome | null) => {
  card.backups = res;
};

/** sayBackup says an outcome in a backup's own line. */
function sayBackup(b: ServerBackup): (res: Outcome | null) => void {
  return (res) => {
    backupUI.of(b.uuid || "").said = res;
  };
}

/** backupPath is a backup's path in the API. */
function backupPath(b: ServerBackup): string {
  return BACKUPS + "/" + encodeURIComponent(b.uuid || "");
}

/** backUpNow makes a backup now, to every target that takes it (POST /api/v1/server-backups). */
export function backUpNow(): Promise<boolean> {
  return act(sayCard, reloadJob, () => api<ServerBackupResult>("POST", BACKUPS, {}),
    (r) => stepsSaid(r.steps, t("serverbackup.made", { when: r.backup?.time ? fmtWhen(r.backup.time) : "" })), "serverbackup.makeFailed");
}

/** upload takes a backup file made elsewhere to the local target, pulled, kept until it is deleted (POST …/upload); off while it runs. */
export async function upload(file: File): Promise<boolean> {
  if (card.uploading) return false;
  card.uploading = true;
  try {
    return await act(sayCard, reloadJob, async () => (await (await apiRaw("POST", BACKUPS + "/upload", file, "application/zip")).json()) as ServerBackupResult,
      (r) => stepsSaid(r.steps, t("serverbackup.uploaded", { when: r.backup?.time ? fmtWhen(r.backup.time) : "" })), "serverbackup.uploadFailed");
  } finally {
    card.uploading = false;
  }
}

/** download saves a backup's file, from a target that holds it (GET …/{id}/file). */
export async function download(b: ServerBackup): Promise<void> {
  const n = session();
  try {
    await saveResponse(await apiRaw("GET", backupPath(b) + "/file"), "massimo-server-backup-" + (b.uuid || "") + ".zip");
  } catch (e) {
    if (stillSession(n)) sayBackup(b)({ text: t("serverbackup.failedAction", { error: (e as Error).message }), ok: false });
  }
}

/** check reads each copy of a backup back, for its size and its hash, without the key (POST …/{id}/check). */
export function check(b: ServerBackup): Promise<boolean> {
  return act(sayBackup(b), reloadJob, () => api<ServerBackupResult>("POST", backupPath(b) + "/check", {}),
    (r) => stepsSaid(r.steps, t("serverbackup.checked")), "serverbackup.failedAction");
}

/** pin keeps a backup from every retention until it is unpinned, or unpins it (PUT …/{id}/pin). */
export function pin(b: ServerBackup, on: boolean): Promise<boolean> {
  return act(sayBackup(b), reloadJob, () => api("PUT", backupPath(b) + "/pin", { pinned: on }),
    () => ({ text: t(on ? "serverbackup.pinnedSaid" : "serverbackup.unpinnedSaid"), ok: true }), "serverbackup.failedAction");
}

/** push copies a backup onto a target from a copy that is there (PUT …/{id}/copies/{target}). */
export function push(b: ServerBackup, target: ServerBackupTargetState): Promise<boolean> {
  return act(sayBackup(b), reloadJob, () => api<ServerBackupResult>("PUT", backupPath(b) + "/copies/" + encodeURIComponent(target.uuid || "")),
    (r) => stepsSaid(r.steps, t("serverbackup.pushed", { target: target.name || "" })), "serverbackup.failedAction");
}

/** pull fetches a target's copy of a backup to the local target, where it stays until it is deleted (POST …/copies/{target}/pull). */
export function pull(b: ServerBackup, c: ServerBackupCopy): Promise<boolean> {
  return act(sayBackup(b), reloadJob, () => api<ServerBackupResult>("POST", backupPath(b) + "/copies/" + encodeURIComponent(c.target || "") + "/pull"),
    (r) => stepsSaid(r.steps, t("serverbackup.pulled", { target: c.target_name || "" })), "serverbackup.failedAction");
}

/** deleteCopy removes a backup's copy from a target, its file included; the backup goes with its last copy (DELETE …/copies/{target}). */
export function deleteCopy(b: ServerBackup, c: ServerBackupCopy, last: boolean): Promise<boolean> {
  const set = last ? sayCard : sayBackup(b);
  return act(set, reloadJob, () => api("DELETE", backupPath(b) + "/copies/" + encodeURIComponent(c.target || "")),
    () => ({ text: t(last ? "serverbackup.deletedLast" : "serverbackup.deleted", { target: c.target_name || "", when: b.time ? fmtWhen(b.time) : "" }), ok: true }),
    "serverbackup.failedAction");
}

/** retry tries a failed backup, or a backup's failed copies, again at once, as the schedule would (POST …/{id}/retry, backup.retry). */
export function retry(b: ServerBackup): Promise<boolean> {
  return act(sayBackup(b), reloadJob, () => api<ServerBackupResult>("POST", backupPath(b) + "/retry"),
    (r) => stepsSaid(r.steps, t("serverbackup.retried")), "serverbackup.failedAction");
}

/** targetPath is a target's path in the API. */
function targetPath(tg: ServerBackupTarget): string {
  return TARGETS + "/" + encodeURIComponent(tg.uuid || "");
}

/** sayTargets says an outcome in the line under the targets. */
const sayTargets = (res: Outcome | null) => {
  card.targets = res;
};

/** addTarget adds a target and has it scanned for the backups it holds (POST /api/v1/server-backup-targets); added, its panel is open. */
export function addTarget(body: Record<string, unknown>): Promise<boolean> {
  const name = String(body.name || "");
  return act(sayTargets, reloadJob, () => api<TargetAdded>("POST", TARGETS, body), (r) => {
    if (r.target?.uuid) targetUI.of(r.target.uuid).open = true;
    if (r.scan_error) return { text: t("serverbackup.target.scanFailed", { name, error: r.scan_error }), ok: false };
    return { text: r.found ? t("serverbackup.target.addedFound", { name, n: r.found }) : t("serverbackup.target.added", { name }), ok: true };
  }, "serverbackup.target.failed");
}

/** saveTarget sends a target's fields (PUT …/{uuid}), the secret only where one is typed. */
export function saveTarget(tg: ServerBackupTarget, body: Record<string, unknown>): Promise<boolean> {
  const name = String(body.name || tg.name || "");
  return act((res) => {
    targetUI.of(tg.uuid || "").said = res;
  }, reloadJob, () => api("PUT", targetPath(tg), body), () => ({ text: t("serverbackup.target.saved", { name }), ok: true }), "serverbackup.target.failed");
}

/** testTarget has the server reach the target and write to it (POST …/{uuid}/test), and says whether it works. */
export function testTarget(tg: ServerBackupTarget): Promise<boolean> {
  const name = tg.name || "";
  return act((res) => {
    targetUI.of(tg.uuid || "").said = res;
  }, async () => undefined, () => api<{ ok: boolean; error?: string }>("POST", targetPath(tg) + "/test"),
  (r) => (r.ok ? { text: t("serverbackup.target.works", { name }), ok: true } : { text: t("serverbackup.target.fails", { name, error: r.error || "" }), ok: false }),
  "serverbackup.target.failed");
}

/** removeTarget removes a target and its copies from the list, its files staying (DELETE …/{uuid}). */
export function removeTarget(tg: ServerBackupTarget): Promise<boolean> {
  const name = tg.name || "";
  return act(sayTargets, reloadJob, () => api("DELETE", targetPath(tg)), () => ({ text: t("serverbackup.target.removed", { name }), ok: true }),
    "serverbackup.target.failed");
}

/** sayKeys says an outcome in the line under the keys. */
const sayKeys = (res: Outcome | null) => {
  card.keys = res;
};

/** keyKinds names the kind of the keys set, for the refusal of a mix: post-quantum or classic. */
function keyKinds(): string {
  return t(sb.keys.some((k) => k.kind === "pq") ? "serverbackup.key.group.pq" : "serverbackup.key.group.classic");
}

/**
 * addKey adds a key (POST /api/v1/server-backup-keys, the owner's): a
 * public key pasted (way "paste"), or one made of the kind way names
 * ("pq" or "classic", D17-C), whose secret part the answer gives once
 * and the card shows until it is confirmed. A key of another kind than
 * the keys set is refused, and the card says why (mixed_kinds).
 */
export function addKey(label: string, way: string, recipient: string): Promise<boolean> {
  const body = way === "paste" ? { label, recipient } : { label, make: way };
  return act(sayKeys, reloadKeys, () => api<KeyAdded>("POST", KEYS, body).catch((e: Error & { data?: { code?: string } }) => {
    if (e.data?.code === "mixed_kinds") throw new Error(t("serverbackup.key.mixed", { kinds: keyKinds() }));
    throw e;
  }), (r) => {
    if (r.identity && r.key?.uuid) {
      card.made = { uuid: r.key.uuid, identity: r.identity, file: r.key_file || r.identity + "\n" };
      return { text: t("serverbackup.key.madeSaid", { label: label || r.key.fingerprint || "" }), ok: true };
    }
    return { text: t("serverbackup.key.added", { label: label || r.key?.fingerprint || "" }), ok: true };
  }, "serverbackup.key.refused");
}

/** confirmKey sends a key made's last six characters (POST …/{uuid}/confirm), which make it a key; others are refused. */
export function confirmKey(k: ServerBackupKey, lastSix: string): Promise<boolean> {
  return act(sayKeys, reloadKeys, () => api("POST", KEYS + "/" + encodeURIComponent(k.uuid || "") + "/confirm", { last_six: lastSix }).catch((e: Error & { status?: number }) => {
    if (e.status === 422) throw new Error(t("serverbackup.key.wrongSix"));
    throw e;
  }), () => {
    card.made = null; // confirmed: its secret part goes from the page
    return { text: t("serverbackup.key.confirmed", { label: k.label || k.fingerprint || "" }), ok: true };
  }, "serverbackup.key.refused");
}

/** removeKey removes a key (DELETE …/{uuid}); backups made before stay encrypted for it. */
export function removeKey(k: ServerBackupKey): Promise<boolean> {
  return act(sayKeys, reloadKeys, () => api("DELETE", KEYS + "/" + encodeURIComponent(k.uuid || "")), () => {
    if (card.made?.uuid === k.uuid) card.made = null;
    return { text: t("serverbackup.key.removed", { label: k.label || k.fingerprint || "" }), ok: true };
  }, "serverbackup.failedAction");
}

/** The card's reader of the stream's events: one read for a burst, while Maintenance is shown. */
const reader = burstReader(async (what) => {
  if (what.has("list")) await loadList();
  if (what.has("targets")) await loadTargets();
  if (what.has("keys")) await loadKeys();
}, () => isShown("maint"));

/**
 * serverEvent reads again, while Maintenance is shown, what an event of
 * the stream says has changed: everything at a resync of the stream,
 * which is how the page learns of a restart; the list and the targets'
 * last copies at a step of a job of the server's (server_backup: a
 * backup made, copied, pruned, scanned, checked or failed), not while a
 * request of this page's own runs, which reads them after it anyway.
 */
export function serverEvent(ev: ServerEvent): void {
  if (!isShown("maint") || !can("backup.retry")) return;
  if (ev.kind === "resync") reader("list", "targets", "keys");
  else if (ev.kind === "server_backup" && !own.n) reader("list", "targets");
}
