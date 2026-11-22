// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/backups/store.ts
 *
 * What a Cube's box of Backup and restore is drawn from, read and sent,
 * line for line as backup.js did: kept by the Cube's serial, so that its
 * boxes on Cube settings and in System settings show the same. A Cube's
 * backups the server keeps (GET …/backups), whether it keeps them at all
 * (a 503 says it does not), when its configuration changed against the
 * newest one and when the automatic one follows, the comparisons asked
 * for, the restore shown and the one sent; a backup taken by hand (POST
 * …/backups), deleted (DELETE …/backups/{id}), a download of the Cube as
 * it is now or of a stored backup, a restore's plan (POST
 * …/restore/plan) and its steps sent (POST …/restore), followed on the
 * event stream. Each box has its own message line and its format. An
 * answer after a sign-out is dropped (shared/session.ts).
 */

import type { BackupList, RestorePlan, StoredBackup } from "../api";
import type { Outcome } from "../shared/action";
import { liveMap } from "../shared/live";
import { cubePath } from "../shared/path";
import { session, stillSession } from "../shared/session";
import { fmtWhen } from "../shared/time";
import { can } from "../shared/me";

/** A restore shown: its title, the stored backup or the file's body and type, and its plan. */
export interface Plan {
  title: string;
  backup?: number;
  body?: string;
  type?: string;
  plan: RestorePlan;
}

/** A restore sent: its id once the server has one, each step's outcome, its state, the steps the Cube took and its error. */
export interface Run {
  id: string;
  results: ({ ok: boolean; error?: string } | undefined)[];
  state: string;
  sent?: number;
  error?: string;
}

/**
 * Per serial, what the Cube has: its list, whether the server keeps its
 * backups (history) and whether the list was loaded, when its
 * configuration changed and when the automatic backup follows, the
 * comparisons by backup id, the restore shown and the one sent.
 */
export const cubeData = liveMap(() => ({
  list: [] as StoredBackup[],
  history: true,
  loaded: false,
  changed: null as string | null,
  due: null as string | null,
  diff: {} as Record<number, { steps?: { what?: string }[] }>,
  plan: null as Plan | null,
  run: null as Run | null,
  /** The place whose box asked for the plan shown, which brings it into view once (PlanBox); "" once it has. */
  asked: "",
}));

/** Per place ("cube" or "sys") and serial, a box's message line and the format of its downloads. */
export const boxUI = liveMap(() => ({ said: null as Outcome | null, format: "yaml" }));

/** Per serial, its fold in System settings open; a single Cube's is open at first. */
export const foldUI = liveMap(() => ({ open: null as boolean | null }));

/** The serials whose data the region holds, for the restore events and a sign-out. */
const known = new Set<string>();

/** of is a Cube's data, by its serial. */
export function of(serial: string) {
  known.add(serial);
  return cubeData.of(serial);
}

/** boxOf is a box's state, by its place and its Cube's serial. */
export function boxOf(place: string, serial: string) {
  return boxUI.of(place + ":" + serial);
}

/** say shows text in a box's message line, ok green, else red; no text hides it; nothing where n, the session it was asked in, has ended. */
export function say(place: string, serial: string, text: string, ok?: boolean, n = session()): void {
  if (stillSession(n)) boxOf(place, serial).said = text ? { text, ok: !!ok } : null;
}

/** running says whether a restore sent to the Cube runs: its Restore… and Restore from file… are off meanwhile, in both places. */
export function running(serial: string): boolean {
  return of(serial).run?.state === "running";
}

/** loadBackups reads a Cube's list; a 503 says that the server keeps none of it (no row in its store). */
async function loadBackups(serial: string): Promise<void> {
  if (!serial || !can("backup")) return;
  const n = session();
  const b = of(serial);
  try {
    const r = await api<BackupList>("GET", cubePath(serial) + "/backups");
    if (!stillSession(n)) return; // signed out meanwhile: nothing of it is the next user's
    b.history = true;
    b.list = r?.backups || [];
    b.changed = r?.changed || null;
    b.due = r?.due || null;
  } catch (e) {
    if (!stillSession(n)) return;
    if ((e as Error & { status?: number }).status !== 503) throw e;
    b.history = false;
    b.list = [];
    b.changed = b.due = null;
  }
  b.loaded = true;
  // a stored backup's plan goes with the backup, once a load no longer lists it
  if (b.plan?.backup && !b.run && !b.list.some((x) => x.id === b.plan?.backup)) b.plan = null;
}

/** refreshBackups reads a Cube's list again and draws its boxes; a failure is said in each of them. */
export async function refreshBackups(serial: string): Promise<void> {
  const n = session();
  try {
    await loadBackups(serial);
  } catch (e) {
    for (const place of ["cube", "sys"]) say(place, serial, t("backup.failed", { error: (e as Error).message }), false, n);
  }
}

/** saveResponse saves an answer as a file, named as the server says. */
async function saveResponse(resp: Response, fallback: string): Promise<void> {
  const m = /filename=([^;]+)/.exec(resp.headers.get("Content-Disposition") || "");
  downloadBlob(m ? m[1].trim() : fallback, await resp.blob());
}

/** downloadNow saves the Cube as it is now, in the box's format. */
export async function downloadNow(place: string, serial: string): Promise<void> {
  const n = session();
  const f = boxOf(place, serial).format === "json" ? "json" : "yaml";
  try {
    await saveResponse(await apiRaw("GET", cubePath(serial) + "/backup?format=" + f), "massimo-backup-" + serial + "." + f);
  } catch (e) {
    say(place, serial, t("backup.failed", { error: (e as Error).message }), false, n);
  }
}

/** downloadStored saves a stored backup, in the box's format. */
export async function downloadStored(place: string, serial: string, b: StoredBackup): Promise<void> {
  const n = session();
  const f = boxOf(place, serial).format === "json" ? "json" : "yaml";
  try {
    await saveResponse(await apiRaw("GET", cubePath(serial) + "/backups/" + b.id + "?format=" + f), "massimo-backup-" + serial + "-" + b.id + "." + f);
  } catch (e) {
    say(place, serial, t("backup.failed", { error: (e as Error).message }), false, n);
  }
}

/** takeBackup keeps a backup taken by hand, and reads the list again. */
export async function takeBackup(place: string, serial: string): Promise<void> {
  const n = session();
  say(place, serial, "");
  try {
    await api("POST", cubePath(serial) + "/backups", {});
    if (!stillSession(n)) return;
    say(place, serial, t("backup.taken"), true);
    await refreshBackups(serial);
  } catch (e) {
    say(place, serial, t("backup.failed", { error: (e as Error).message }), false, n);
  }
}

/** compareBackup asks what changed from the backup before: the planner's steps, shown under the row in both boxes. */
export async function compareBackup(place: string, serial: string, b: StoredBackup, older: StoredBackup): Promise<void> {
  const n = session();
  try {
    const d = await api<{ steps?: { what?: string }[] }>("GET", cubePath(serial) + "/backups/" + b.id + "/diff?against=" + older.id);
    if (!stillSession(n)) return;
    const c = of(serial);
    c.diff = { ...c.diff, [b.id]: d };
  } catch (e) {
    say(place, serial, t("backup.failed", { error: (e as Error).message }), false, n);
  }
}

/** deleteBackup deletes a stored backup and reads the list again; it resolves to whether the deletion was taken. */
export async function deleteBackup(place: string, serial: string, b: StoredBackup): Promise<boolean> {
  const n = session();
  try {
    await api("DELETE", cubePath(serial) + "/backups/" + b.id);
    if (!stillSession(n)) return false;
    say(place, serial, t("backup.deleted"), true);
    const c = of(serial);
    const { [b.id]: _gone, ...rest } = c.diff;
    c.diff = rest;
    await refreshBackups(serial);
    return true;
  } catch (e) {
    say(place, serial, t("backup.failed", { error: (e as Error).message }), false, n);
    return false;
  }
}

/** showPlan asks the server what restoring the source would send and shows it in the Cube's boxes; nothing is sent yet. */
async function showPlan(place: string, serial: string, src: Omit<Plan, "plan">): Promise<boolean> {
  if (running(serial)) return false; // a running restore stays in sight
  const n = session();
  say(place, serial, "");
  const q = src.backup ? "?backup=" + src.backup : "";
  try {
    const resp = await apiRaw("POST", cubePath(serial) + "/restore/plan" + q, src.body, src.type);
    const plan = (await resp.json()) as RestorePlan;
    if (!stillSession(n) || running(serial)) return false;
    const c = of(serial);
    c.plan = { ...src, plan };
    c.run = null;
    c.asked = place;
    return true;
  } catch (e) {
    say(place, serial, t("restore.planFailed", { error: (e as Error).message }), false, n);
    return false;
  }
}

/** planStored shows the plan of a stored backup's restore. */
export function planStored(place: string, serial: string, b: StoredBackup): Promise<boolean> {
  return showPlan(place, serial, { title: t("restore.planStored", { when: fmtWhen(b.taken_at) }), backup: b.id });
}

/** planFile shows the plan of a file's restore, YAML or JSON by its name or its first character. */
export async function planFile(place: string, serial: string, file: File): Promise<boolean> {
  const text = await file.text();
  const yaml = /\.ya?ml$/i.test(file.name) || text.trimStart()[0] !== "{";
  return showPlan(place, serial, { title: t("restore.planFile", { file: file.name }), body: text, type: yaml ? "application/yaml" : "application/json" });
}

/** closePlan takes the restore shown away. */
export function closePlan(serial: string): void {
  const c = of(serial);
  c.plan = null;
  c.run = null;
}

/** sendRestore sends the plan's steps; the stream then says how each went. */
export async function sendRestore(place: string, serial: string): Promise<void> {
  const c = of(serial);
  const p = c.plan;
  if (!p || c.run) return;
  const n = session();
  let q = "?plan=" + encodeURIComponent(p.plan.plan);
  if (p.backup) q += "&backup=" + p.backup;
  c.run = { id: "", results: [], state: "running" };
  try {
    const r = (await (await apiRaw("POST", cubePath(serial) + "/restore" + q, p.body, p.type)).json()) as { id: string };
    if (!stillSession(n)) return;
    if (c.run && !c.run.id) c.run = { ...c.run, id: r.id };
  } catch (e) {
    if (!stillSession(n)) return;
    c.run = null;
    say(place, serial, t("restore.refused", { error: (e as Error).message }), false);
  }
}

/** A restore's event of the stream, restore_step or restore_done. */
export interface RestoreEvent {
  kind?: string;
  cube?: string;
  restore?: { id: string; step?: number; ok?: boolean; error?: string; state?: string; sent?: number };
}

/**
 * restoreEvent takes the stream's restore_step and restore_done of a
 * restore a box sent: the Cube whose run has its id, or the one whose run
 * has none yet, its Cube named in the event where it is.
 */
export function restoreEvent(ev: RestoreEvent): void {
  const r = ev?.restore;
  if (!r) return;
  const runs = [...known].map((s) => [s, cubeData.of(s)] as const).filter(([, b]) => b.run);
  const found = runs.find(([, b]) => b.run?.id === r.id) || runs.find(([s, b]) => !b.run?.id && (!ev.cube || ev.cube === s));
  if (!found) return;
  const [serial, b] = found;
  const run = { ...(b.run as Run), id: r.id };
  if (ev.kind === "restore_step") {
    run.results = [...run.results];
    run.results[(r.step || 1) - 1] = { ok: !!r.ok, error: r.error };
  } else {
    run.state = r.state || (r.ok ? "done" : "failed");
    run.sent = r.sent;
    run.error = r.error;
    refreshBackups(serial);
  }
  b.run = run;
}

/**
 * resynced takes a resync of the stream, after which what it lost will not
 * come (app.js): a restore still running is shown as one whose end the
 * page did not see, the log saying how it ended, and no longer counts as
 * running, so that its Cube's Restore buttons are back; the server
 * refuses a second restore while one runs, as before.
 */
export function resynced(): void {
  for (const s of known) {
    const b = cubeData.of(s);
    if (b.run?.state !== "running") continue;
    b.run = { ...b.run, state: "unseen" };
  }
}

/** forgetBackups forgets what the boxes kept of the user who was signed in (MassimoParts.signedOut): every Cube's data, every box's message. */
export function forgetBackups(): void {
  known.clear();
  cubeData.clear();
  boxUI.clear();
  foldUI.clear();
}
