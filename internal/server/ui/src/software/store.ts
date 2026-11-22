// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/software/store.ts
 *
 * What Maintenance → Software update is drawn from, read and sent, line
 * for line as update.js did: the running version, the channel and the
 * newest release on it (GET /api/v1/update, ?refresh=1 to ask now), the
 * setting update.channel for a holder of settings (GET and PUT
 * /api/v1/settings), the request to the update helper (POST
 * /api/v1/update), and the reading every 3 s while the helper works,
 * which rides out the server's restart. While Maintenance is shown, an
 * event of the stream that says the versions or the channel changed reads
 * them again (serverEvent). A load writes the live state the
 * part draws, so that a newer release, the helper's progress and the last
 * updates show in place; an answer after a sign-out is dropped
 * (shared/session.ts), and a sign-out ends the reading and takes the
 * question back.
 */

import type { Setting, UpdateInfo, UpdateStatus } from "../api";
import { type ServerEvent, settingsChanged } from "../model/event";
import { burstReader } from "../shared/burst";
import { counter } from "../shared/counter";
import { live } from "../shared/live";
import { session, stillSession } from "../shared/session";
import { isShown } from "../shared/pages";
import { can } from "../shared/me";

/** A text said in the page's language at its drawing. */
export type Say = () => string;

/** The section's load counter: every part of it uses it. */
export const load = counter();

/**
 * UPDATE_FLAVOURS are the channels a box may follow, the most rolling
 * first, as update.Flavours has them: each takes its own releases and
 * those of every more stable one.
 */
export const UPDATE_FLAVOURS = ["devel", "nightly", "alpha", "beta", "rc", "stable"];

/** What the section shows, as the last loads gave it. */
export const data = live({
  /** GET /api/v1/update's last answer, null before one came. */
  info: null as UpdateInfo | null,
  /** Why the last load failed, said in the version's line in place of it; null after one that went through. */
  failed: null as Say | null,
  /** What the request to the helper said, in its line until the next load. */
  said: null as Say | null,
  /** The setting update.channel, for a holder of settings; null where it could not be read. */
  setting: null as Setting | null,
  /** Why the last change of the channel failed, in the channel's note until it is read again. */
  channelSaid: null as Say | null,
  /** The helper works, or the request to it runs: Update turns and Check for updates is off. */
  busy: false,
});

/** The question Update asks in place of the section's buttons, and the version it asked about. */
export const question = live({ asking: false, version: "" });

/**
 * The reading while the helper works: whether it runs, its generation
 * (a stopped reading's read that answers late reads no more), its timer,
 * when it gives up, and the grace after a request.
 */
const follow = { on: false, gen: 0, timer: 0 as ReturnType<typeof setTimeout> | 0, until: 0, grace: 0 };

/** channelSeq counts the changes of the channel: a later change's reads supersede an earlier one's; channelOwn counts this page's changes under way, their PUT and their reads. */
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
  data.info = u;
  data.failed = null;
  data.said = null;
  if (question.asking && (!u.can_update || u.latest !== question.version)) question.asking = false;
  if (updateBusy(u.helper)) {
    // busy follows what a read finds: the helper at work greys the buttons out, whoever started it
    setBusy(true);
    if (!follow.on) followUpdate();
  }
  load.bump();
  renderServerSign(); // header.js
  return u;
}

/** loadChannel reads the setting update.channel, for a holder of settings; mine as loadUpdate's. */
export async function loadChannel(mine: () => boolean = () => true): Promise<void> {
  if (!can("settings")) return;
  const n = session();
  let setting: Setting | null;
  try {
    const all = (await api<Record<string, Setting>>("GET", "api/v1/settings")) || {};
    setting = all["update.channel"] || null;
  } catch {
    setting = null;
  }
  if (!stillSession(n) || !mine()) return;
  data.setting = setting;
  data.channelSaid = null;
  load.bump();
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
export async function setChannel(value: string): Promise<void> {
  const n = session();
  const seq = ++channelSeq;
  channelOwn++;
  let failed: Error | null = null;
  try {
    const all = await api<Record<string, Setting> | null>("PUT", "api/v1/settings", { "update.channel": value || null });
    if (stillSession(n) && seq === channelSeq && all && all["update.channel"]) data.setting = all["update.channel"];
  } catch (e) {
    failed = e as Error;
  }
  if (!stillSession(n)) {
    channelOwn--;
    return;
  }
  void channelReads(n, seq, failed).finally(ownChannelEnded);
}

/** channelReads are a change's reads after its PUT: the release check on the channel, then the setting; a later change's supersede them. */
async function channelReads(n: number, seq: number, failed: Error | null): Promise<void> {
  const mine = () => seq === channelSeq;
  await loadUpdate(true, mine);
  await loadChannel(mine);
  if (failed && stillSession(n) && mine()) {
    const error = failed.message;
    data.channelSaid = () => t("update.channelFailed", { error });
  }
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
    const busy = !u || updateBusy(u.helper) || Date.now() < follow.grace;
    if (!busy || Date.now() > follow.until) {
      stopFollow();
      setBusy(false);
      return;
    }
    follow.timer = setTimeout(step, 3000);
  };
  follow.timer = setTimeout(step, 3000);
}

/** ask asks in the section whether to install the update the last load offers. */
export function ask(): void {
  const u = data.info;
  if (!u || !u.latest) return;
  question.version = u.latest;
  question.asking = true;
}

/** runUpdate asks the server's update helper to install the version asked about, then follows it. */
export async function runUpdate(): Promise<void> {
  const version = question.version;
  const n = session();
  setBusy(true);
  let requested = version;
  try {
    const r = await api<{ requested?: string } | null>("POST", "api/v1/update", {});
    // the server checks again and installs its newest: the line names what it says it asked for
    if (r && r.requested) requested = r.requested;
  } catch (e) {
    if (!stillSession(n)) return;
    // refused (409: one already runs) while a reading follows the helper: the buttons stay off, the helper's line stays
    if (follow.on) return;
    setBusy(false);
    const error = (e as Error).message;
    data.said = () => t("update.notStarted", { error });
    return;
  }
  if (!stillSession(n)) return;
  data.said = () => t("update.requested", { version: requested });
  follow.grace = Date.now() + 30 * 1000;
  followUpdate();
}

/** ownFlow says whether the page follows an update of its own: its request runs, or the reading every 3 s while the helper works. */
function ownFlow(): boolean {
  return data.busy || follow.on;
}

/**
 * The section's reads for the stream's events, one at a time while
 * Maintenance is shown: "update" the versions, "channel" the setting
 * update.channel. A read is dropped where this page's own change of the
 * channel is under way (its reads read the same), and its answer shows
 * nothing where a choice was made meanwhile.
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
 * versions on it after update.channel was set (change). An update asked
 * for from another browser (update) is followed as this page follows its
 * own: the reading with its grace, since right after the request the
 * helper may still report its last state. An update this page follows
 * reads by its own pace, so its events read nothing.
 */
export function serverEvent(ev: ServerEvent): void {
  if (!isShown("maint")) return;
  if (ev.kind === "resync") reader("update", "channel");
  else if (settingsChanged(ev).includes("update.channel")) reader("update", "channel");
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

/** updateOut tells whether an update is out, for one who may install it (the server's sign, update.js). */
export function updateOut(): boolean {
  const u = data.info;
  return !!u && !!u.available && can("settings");
}

/** updateLatest is the version an update would install, "" for none (update.js). */
export function updateLatest(): string {
  return (data.info && data.info.latest) || "";
}

/**
 * forgetSoftware forgets what the section kept of the user who was
 * signed in (MassimoParts.signedOut): the reading while the helper works
 * ends, the question goes, and the channel's setting, a holder of
 * settings' alone, is read again for the next. The versions, which every
 * user may read, stay for the server's sign.
 */
export function forgetSoftware(): void {
  stopFollow();
  data.busy = false;
  data.said = null;
  data.setting = null;
  data.channelSaid = null;
  question.asking = false;
  question.version = "";
  load.bump();
}
