// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/software/store.ts
 *
 * What Maintenance → Software update is drawn from, read and sent, line
 * for line as update.js did: the running version, the channel and the
 * newest release on it (GET /api/v1/update, ?refresh=1 to ask now), the
 * settings update.channel, update.auto and update.auto_window for a
 * holder of settings (GET and PUT /api/v1/settings), the request to the
 * update helper (POST /api/v1/update; no_backup for Update without a
 * backup, where the backup before the last update failed) and the
 * answer to the automatic update's question (PUT
 * /api/v1/update/answer), and the reading every
 * 3 s while the helper works,
 * which rides out the server's restart. Update and Install now read
 * first whether the server's next start refuses the settings file (GET
 * /api/v1/restart), which their question then says (AP AutoUpdate,
 * D12-A to D14-A). While Maintenance is shown, an
 * event of the stream that says the versions or the channel changed reads
 * them again (serverEvent). A load writes the live state the
 * part draws, so that a newer release, the helper's progress and the last
 * updates show in place; an answer after a sign-out is dropped
 * (shared/session.ts), and a sign-out ends the reading and takes the
 * questions back.
 */

import type { RestartInfo, Setting, UpdateInfo, UpdateStatus } from "../api";
import { type ServerEvent, settingsChanged } from "../model/event";
import { burstReader } from "../shared/burst";
import { live } from "../shared/live";
import { session, stillSession } from "../shared/session";
import { isShown } from "../shared/pages";
import { can } from "../shared/me";
import { update } from "../shared/update";

/** A text said in the page's language at its drawing. */
export type Say = () => string;

/**
 * UPDATE_FLAVOURS are the channels a box may follow, the most rolling
 * first, as update.Flavours has them: each takes its own releases and
 * those of every more stable one.
 */
export const UPDATE_FLAVOURS = ["devel", "nightly", "alpha", "beta", "rc", "stable"];

/** What the section shows, as the last loads gave it. */
export const data = live({
  /** Why the last load failed, said in the version's line in place of it; null after one that went through. */
  failed: null as Say | null,
  /** What the request to the helper said, in its line until the next load. */
  said: null as Say | null,
  /** The setting update.channel, for a holder of settings; null where it could not be read. */
  setting: null as Setting | null,
  /** The settings update.auto and update.auto_window, as setting is read; null where they could not be. */
  auto: null as Setting | null,
  window: null as Setting | null,
  /** Why the last change of the channel failed, in the channel's note until it is read again. */
  channelSaid: null as Say | null,
  /** Why the last change of the automatic update or its window failed, in its note until it is read again. */
  autoSaid: null as Say | null,
  /** What the last answer to the automatic update's question said, where it was refused, under its buttons until the next load. */
  answerSaid: null as Say | null,
  /** An answer to the question is on its way: its buttons are off. */
  answering: false,
  /** The helper works, or the request to it runs: Update turns and Check for updates is off. */
  busy: false,
});

/**
 * The question Update asks in place of the section's buttons: the
 * version it asked about, whether the server's next start refuses the
 * settings file, as read at the press (refused), whether that read
 * runs (reading), Update turning meanwhile, and whether it was Update
 * without a backup that asked (noBackup), which the request then says.
 */
export const question = live({ asking: false, version: "", refused: false, reading: false, noBackup: false });

/**
 * The question Install now asks while the server's next start refuses
 * the settings file, where it was pressed, in place of the automatic
 * update's three answers: under the header (update-ask) and in the
 * card's copy (update-card-ask), each with the version it asked about
 * and whether its read runs.
 */
export const nowAsks = {
  "update-ask": live({ asking: false, version: "", reading: false }),
  "update-card-ask": live({ asking: false, version: "", reading: false }),
};

/** AskPlace is where the automatic update's question is asked, the prefix of its answers' ids. */
export type AskPlace = keyof typeof nowAsks;

/**
 * The reading while the helper works: whether it runs, its generation
 * (a stopped reading's read that answers late reads no more), its timer,
 * when it gives up, and the grace after a request.
 */
const follow = { on: false, gen: 0, timer: 0 as ReturnType<typeof setTimeout> | 0, until: 0, grace: 0 };

/** channelSeq counts the changes of the update's settings, the channel, the automatic update and its window: a later change's reads supersede an earlier one's; channelOwn counts this page's changes under way, their PUT and their reads. */
let channelSeq = 0;
let channelOwn = 0;

/** channelMissed says that an event's read of the channel was skipped while this page's own change ran: it is read once that change has ended. */
let channelMissed = false;

/** ownChannelEnded counts one of this page's changes of the channel as ended, and reads what an event asked for meanwhile, once. */
function ownChannelEnded(): void {
  channelOwn--;
  if (channelOwn || !channelMissed) return;
  channelMissed = false;
  reader("update", "channel");
}

/** updateBusy reports whether the helper is working on an update. */
export function updateBusy(h: UpdateStatus | undefined): boolean {
  return !!h && ["checking", "installing", "restarting"].includes(h.state);
}

/**
 * updateWorking reports whether an update is under way: being started,
 * the server's backup running before the helper is asked (AP
 * ServerBackup, Block 10), or at the helper.
 */
function updateWorking(u: UpdateInfo): boolean {
  return !!u.starting || updateBusy(u.helper);
}

/** setBusy turns Update and greys out Check while an update runs (update.js setUpdateBusy). */
function setBusy(busy: boolean): void {
  data.busy = busy;
}

/**
 * loadUpdate reads GET /api/v1/update (refresh: ask for a release now)
 * and draws it: the question goes where the release it asked about is no
 * longer the one an update installs, and the helper at work already
 * (after a reload while it restarts the server, an update started from
 * another browser) is followed. It returns the answer, null where the
 * load failed, which the version's line says.
 */
export async function loadUpdate(refresh: boolean, mine: () => boolean = () => true): Promise<UpdateInfo | null> {
  const n = session();
  let u: UpdateInfo;
  try {
    u = await api<UpdateInfo>("GET", "api/v1/update" + (refresh ? "?refresh=1" : ""));
  } catch (e) {
    const error = (e as Error).message;
    if (stillSession(n) && mine()) data.failed = () => t("update.cannotAsk", { error });
    return null;
  }
  if (!stillSession(n)) return null; // signed out meanwhile: the next user's reads are their own
  if (!mine()) return u; // a later change of the channel reads its own: this answer shows nothing
  update.value = u;
  data.failed = null;
  data.said = null;
  if (question.asking && (!u.can_update || u.latest !== question.version)) question.asking = false;
  takeNowBack(u);
  if (updateWorking(u)) {
    // busy follows what a read finds: the backup or the helper at work greys the buttons out, whoever started it
    setBusy(true);
    if (!follow.on) followUpdate();
  }
  renderServerSign(); // header.js
  return u;
}

/** loadChannel reads the settings update.channel, update.auto and update.auto_window, for a holder of settings; mine as loadUpdate's. */
export async function loadChannel(mine: () => boolean = () => true): Promise<void> {
  if (!can("settings")) return;
  const n = session();
  let all: Record<string, Setting> = {};
  try {
    all = (await api<Record<string, Setting>>("GET", "api/v1/settings")) || {};
  } catch {
    all = {};
  }
  if (!stillSession(n) || !mine()) return;
  takeSettings(all);
  data.channelSaid = null;
  data.autoSaid = null;
}

/** takeSettings keeps the update's settings of an answer of GET or PUT /api/v1/settings. */
function takeSettings(all: Record<string, Setting>): void {
  data.setting = all["update.channel"] || null;
  data.auto = all["update.auto"] || null;
  data.window = all["update.auto_window"] || null;
}

/**
 * setChannel stores the chosen channel, none ("") for the build's
 * flavour: the PUT alone is the choice's request, which the select shows
 * while it runs; its answer, all the settings, shows the channel stored.
 * The release check on it and the setting's read follow outside it, as
 * the plain page sent them, so that another choice meanwhile goes out at
 * once: a later choice's reads supersede an earlier one's, whose late
 * answers change nothing shown. A refusal is said in the channel's note.
 */
export function setChannel(value: string): Promise<void> {
  return saveSetting("update.channel", value || null, (error) => (data.channelSaid = () => t("update.channelFailed", { error })));
}

/** setAuto stores the automatic update's mode, off or night, as setChannel stores the channel. */
export function setAuto(value: string): Promise<void> {
  return saveSetting("update.auto", value, (error) => (data.autoSaid = () => t("update.autoFailed", { error })));
}

/** setWindow stores the automatic update's window, HH:MM-HH:MM on the server's clock, as setChannel stores the channel. */
export function setWindow(value: string): Promise<void> {
  return saveSetting("update.auto_window", value, (error) => (data.autoSaid = () => t("update.autoFailed", { error })));
}

/**
 * saveSetting stores one of the update's settings, key, as setChannel
 * says: the PUT, whose answer shows the settings stored, then the reads
 * outside it; refused says why where the server refused it.
 */
async function saveSetting(key: string, value: string | null, refused: (error: string) => void): Promise<void> {
  const n = session();
  const seq = ++channelSeq;
  channelOwn++;
  let failed: Error | null = null;
  try {
    const all = await api<Record<string, Setting> | null>("PUT", "api/v1/settings", { [key]: value });
    if (stillSession(n) && seq === channelSeq && all && all[key]) takeSettings(all);
  } catch (e) {
    failed = e as Error;
  }
  if (!stillSession(n)) {
    channelOwn--;
    return;
  }
  void channelReads(n, seq, failed, refused).finally(ownChannelEnded);
}

/** channelReads are a change's reads after its PUT: the release check on the channel, then the settings; a later change's supersede them. */
async function channelReads(n: number, seq: number, failed: Error | null, refused: (error: string) => void): Promise<void> {
  const mine = () => seq === channelSeq;
  await loadUpdate(true, mine);
  await loadChannel(mine);
  if (failed && stillSession(n) && mine()) refused(failed.message);
}

/** stopFollow ends the reading while the helper works. */
function stopFollow(): void {
  if (follow.timer) clearTimeout(follow.timer);
  follow.timer = 0;
  follow.on = false;
  follow.gen++;
}

/**
 * followUpdate reads 3 s after the last read while the helper works, one
 * read at a time, for at most five minutes, the end checked before each
 * read: a read that waits (for a sign-in after a 401) starts no other.
 * The server goes away for the restart, which the reading rides out.
 * Right after a request the helper may not have picked it up yet, so that
 * its last state still says done: the grace covers that gap. A sign-out
 * ends it.
 */
function followUpdate(): void {
  stopFollow();
  const n = session();
  const gen = follow.gen;
  follow.on = true;
  follow.until = Date.now() + 5 * 60 * 1000;
  const step = async () => {
    follow.timer = 0;
    if (Date.now() > follow.until) {
      stopFollow();
      setBusy(false);
      return;
    }
    const u = await loadUpdate(false);
    if (!stillSession(n) || gen !== follow.gen) return; // stopped meanwhile: the sign-out, or a reading begun anew
    // the server's backup before the update may outlast the reading's five minutes: they count from its end
    if (u && u.starting) follow.until = Math.max(follow.until, Date.now() + 5 * 60 * 1000);
    const busy = !u || updateWorking(u) || Date.now() < follow.grace;
    if (!busy || Date.now() > follow.until) {
      stopFollow();
      setBusy(false);
      return;
    }
    follow.timer = setTimeout(step, 3000);
  };
  follow.timer = setTimeout(step, 3000);
}

/**
 * readRefused reads GET /api/v1/restart under the session's number and
 * says whether the server's next start refuses the settings file. A
 * holder of restart reads it, and so does one of update or reboot, which
 * each include restart (AP StartConfiguration, D37). It answers true
 * where the answer's refused is not empty; false where the read failed
 * (a 403 for a user with none of the three, or where restart is disabled
 * for every role; no answer); null where the session changed meanwhile,
 * its answer the user's who was.
 */
export async function readRefused(): Promise<boolean | null> {
  const n = session();
  let v: RestartInfo | null;
  try {
    v = await api<RestartInfo | null>("GET", "api/v1/restart");
  } catch {
    return stillSession(n) ? false : null;
  }
  if (!stillSession(n)) return null;
  return !!v && !!v.refused && v.refused.length > 0;
}

/**
 * ask asks in the section whether to install the update the last load
 * offers, once it has read whether the server's next start refuses the
 * settings file (readRefused), which the question then says (D12-A); a
 * failed read asks as before. Update turns while it reads, and a press
 * meanwhile reads nothing more; the question names the release offered
 * once the answer is in, and goes nowhere where none is to be installed
 * then, or the session changed. noBackup is Update without a backup's
 * press, offered where the last update's backup failed (AP ServerBackup,
 * Block 10): the question says that no backup is taken first.
 */
export async function ask(noBackup = false): Promise<void> {
  if (question.reading || !update.value || !update.value.latest) return;
  question.noBackup = noBackup;
  question.reading = true;
  const refused = await readRefused();
  if (refused === null) return; // signed out meanwhile: forgetSoftware has ended the read
  question.reading = false;
  const u = update.value;
  // a load or an event meanwhile: what loadUpdate and serverEvent take back is not asked
  if (!u || !u.latest || !u.can_update || data.busy) return;
  question.version = u.latest;
  question.refused = refused;
  question.asking = true;
}

/**
 * installNow is Install now, pressed where the automatic update's
 * question is asked (place) about version: it reads whether the server's
 * next start refuses the settings file (readRefused), and where it does
 * asks there first, in place of the three answers (D14-A); where it does
 * not, or the read failed, it answers now at once. A press while the
 * read runs reads nothing more, and a sign-out meanwhile sends nothing.
 */
export async function installNow(place: AskPlace, version: string): Promise<void> {
  const q = nowAsks[place];
  if (q.reading || data.answering) return;
  q.reading = true;
  const refused = await readRefused();
  if (refused === null) return; // signed out meanwhile: forgetSoftware has ended the read
  q.reading = false;
  // a load meanwhile that offers another version, or none: what takeNowBack takes back is not asked
  if (!stillAsks(update.value, version)) return;
  if (!refused) {
    await answerUpdate(version, "now");
    return;
  }
  q.version = version;
  q.asking = true;
}

/** stillAsks says whether u, GET /api/v1/update's answer, still asks the automatic update's question about version. */
function stillAsks(u: UpdateInfo | null, version: string): boolean {
  return !!u && !!u.auto && !!u.auto.ask && !!u.available && u.latest === version;
}

/**
 * takeNowBack takes Install now's questions back where u no longer asks
 * about the version they asked about: another version offered, or the
 * question answered, from any browser.
 */
function takeNowBack(u: UpdateInfo): void {
  for (const q of Object.values(nowAsks)) {
    if (q.asking && !stillAsks(u, q.version)) q.asking = false;
  }
}

/**
 * runUpdate asks the server's update helper to install the version asked
 * about, then follows it: the server takes a backup of itself first,
 * unless Update without a backup asked (no_backup), and answers at once
 * while it is being started; the reading follows the backup too, and a
 * start that failed after the answer, its backup's among them, is said
 * in the helper's line from GET's start_error. A refusal is said there
 * once the versions are read again.
 */
export async function runUpdate(): Promise<void> {
  const version = question.version;
  const noBackup = question.noBackup;
  const n = session();
  setBusy(true);
  let requested = version;
  try {
    const r = await api<{ requested?: string } | null>("POST", "api/v1/update", noBackup ? { no_backup: true } : {});
    // the server checks again and installs its newest: the line names what it says it asked for
    if (r && r.requested) requested = r.requested;
  } catch (e) {
    if (!stillSession(n)) return;
    // refused (409: one already runs) while a reading follows the helper: the buttons stay off, the helper's line stays
    if (follow.on) return;
    setBusy(false);
    const error = (e as Error).message;
    await loadUpdate(false); // the versions again: a start under way, or the backup that failed with Update without a backup
    if (!stillSession(n)) return;
    data.said = () => t("update.notStarted", { error });
    return;
  }
  if (!stillSession(n)) return;
  data.said = () => t("update.requested", { version: requested });
  follow.grace = Date.now() + 30 * 1000;
  followUpdate();
}

/**
 * answerUpdate answers the automatic update's question about version
 * (PUT /api/v1/update/answer), for a holder of update: now starts the
 * update, which the page then follows as its own; tonight and skip end
 * the question, the answer's GET /api/v1/update drawn at once. A refusal
 * is said under the buttons; the versions are read again either way.
 */
export async function answerUpdate(version: string, answer: "now" | "tonight" | "skip"): Promise<void> {
  const n = session();
  data.answering = true;
  data.answerSaid = null;
  try {
    const r = await api<UpdateInfo & { requested?: string }>("PUT", "api/v1/update/answer", { version, answer });
    if (!stillSession(n)) return;
    if (answer === "now") {
      question.asking = false;
      setBusy(true);
      data.said = () => t("update.requested", { version: (r && r.requested) || version });
      follow.grace = Date.now() + 30 * 1000;
      followUpdate();
      return;
    }
    if (r && r.auto) {
      update.value = r;
      takeNowBack(r);
      renderServerSign(); // header.js
    }
  } catch (e) {
    if (!stillSession(n)) return;
    const error = (e as Error).message;
    data.answerSaid = () => t("update.askFailed", { error });
    await loadUpdate(false);
  } finally {
    if (stillSession(n)) data.answering = false;
  }
}

/** ownFlow says whether the page follows an update of its own: its request runs, or the reading every 3 s while the helper works. */
function ownFlow(): boolean {
  return data.busy || follow.on;
}

/**
 * The section's reads for the stream's events, one at a time while
 * Maintenance is shown: "update" the versions, "channel" the update's
 * settings, the channel, the automatic update and its window. A read is
 * dropped where this page's own change of one is under way (its reads
 * read the same), and its answer shows nothing where a choice was made
 * meanwhile.
 */
const reader = burstReader(async (what) => {
  if (channelOwn) {
    channelMissed = true; // read once this page's own change has ended
    return;
  }
  const seq = channelSeq;
  const mine = () => seq === channelSeq && !channelOwn;
  // the flow reads the versions every 3 s itself: an event adds no read, and moves nothing of it
  if (what.has("update") && !ownFlow()) await loadUpdate(false, mine);
  if (what.has("channel")) await loadChannel(mine);
}, () => isShown("maint"));

/**
 * serverEvent reads again, while Maintenance is shown, what an event of
 * the stream says has changed: everything the section shows at a resync
 * of the stream, which is how the page learns of a restart (the server's
 * start and an update's end are written before any stream opens, and a
 * stream opened again gets no replay of them); the channel and the
 * versions on it after update.channel, update.auto or
 * update.auto_window was set (change). An update asked
 * for from another browser (update) is followed as this page follows its
 * own: the reading with its grace, since right after the request the
 * helper may still report its last state. An update this page follows
 * reads by its own pace, so its events read nothing.
 */
export function serverEvent(ev: ServerEvent): void {
  // an answer to the automatic update's question, from any browser: the banner shows on every page
  if (ev.kind === "change" && (ev.summary || {}).action === "update.answer" && (can("update") || can("settings")) && !ownFlow()) {
    void loadUpdate(false);
    return;
  }
  if (!isShown("maint")) return;
  if (ev.kind === "resync") reader("update", "channel");
  else if (settingsChanged(ev).some((k) => k === "update.channel" || k === "update.auto" || k === "update.auto_window")) reader("update", "channel");
  else if (ev.kind === "update" && !ownFlow()) {
    if (!fresh(ev)) {
      reader("update"); // replayed after a gap of the stream: read once, as any other event
      return;
    }
    question.asking = false; // what it asked about changed: an update is running
    setBusy(true);
    follow.grace = Date.now() + 30 * 1000;
    followUpdate();
  }
}

/**
 * fresh says whether an event is less than a minute old: its time, the
 * server's, against this page's clock corrected by the server's (the
 * Date header's skew of header.js); an event without a time is fresh.
 */
function fresh(ev: ServerEvent): boolean {
  const at = Date.parse(ev.time || "");
  return isNaN(at) || Date.now() + serverSkew - at < 60 * 1000;
}

/**
 * forgetSoftware forgets what the section kept of the user who was
 * signed in (MassimoParts.signedOut): the reading while the helper works
 * ends, the questions go, Update's and Install now's with the reads
 * before them, and the channel's setting, a holder of settings' alone,
 * is read again for the next. The versions, which every user may read,
 * stay for the server's sign.
 */
export function forgetSoftware(): void {
  stopFollow();
  channelMissed = false; // a read skipped for the user who was is no read of the next one's
  data.busy = false;
  data.said = null;
  data.setting = null;
  data.auto = null;
  data.window = null;
  data.channelSaid = null;
  data.autoSaid = null;
  data.answerSaid = null;
  data.answering = false;
  question.asking = false;
  question.version = "";
  question.refused = false;
  question.reading = false;
  question.noBackup = false;
  for (const q of Object.values(nowAsks)) {
    q.asking = false;
    q.version = "";
    q.reading = false;
  }
}
