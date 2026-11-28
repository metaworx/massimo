// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/flow.ts
 *
 * A Cube's pairing (AP UiParts, Block 9), what pairing.js kept per Cube as
 * pairingUI: the flow, live, by the Cube's serial (the form shown, the time
 * typed, the server's state, the devices that joined while the page
 * watched, the last outcome, the request under way, the seconds left), what
 * each joined device's form keeps, the Cube's rooms the form offers, and
 * what the pairing sends: GET, POST and DELETE …/pairing, as pairing.js
 * sent them. The countdown moves on every second in the flow itself, so
 * that the pairing's part changes its text in place; it asks for the state
 * every third second where the stream does not report it.
 */

import { signal } from "@preact/signals";
import type { NewDevice, PairingState, Room } from "../api";
import type { Outcome } from "../shared/action";
import { liveMap } from "../shared/live";
import { cubePath } from "../shared/path";
import { session, stillSession } from "../shared/session";
import { newPut, type Put } from "../shared/put";

/** A Cube's pairing as its part draws it. */
export interface Flow {
  /** The time and Start shown, after Pair a device. */
  form: boolean;
  /** The time typed, in seconds, as the field holds it. */
  seconds: string;
  /** The server's state: pairing runs, until when (ms, 0 for none). */
  active: boolean;
  until: number;
  /** The devices that joined while this page watched the pairing run. */
  paired: NewDevice[];
  /** The last outcome: an error, or the end of a pairing the page saw. */
  msg: Outcome | null;
  /** The request under way: Start's, Stop's, or none. */
  sending: "" | "start" | "stop";
  /** The seconds left while pairing runs, which the countdown shows. */
  left: number;
  /** What each joined device's form keeps, by its radio address; a new pairing starts them afresh. */
  puts: Record<string, Put>;
}

/** The flows by the Cube's serial. */
const flows = liveMap<Flow>(() => ({ form: false, seconds: "60", active: false, until: 0, paired: [], msg: null, sending: "", left: 0, puts: {} }));

/** flowOf is a Cube's pairing. */
export function flowOf(serial: string): Flow {
  return flows.of(serial);
}

/**
 * listed lists devices as the Cube's devices that joined, each with what
 * its form keeps, made where it is listed first, never while a part draws.
 */
function listed(f: Flow, devices: NewDevice[]): void {
  const fresh = devices.filter((d) => !f.puts[d.rf_address]);
  if (fresh.length) {
    const puts = { ...f.puts };
    for (const d of fresh) puts[d.rf_address] = newPut();
    f.puts = puts;
  }
  f.paired = devices;
}

/** Per serial, the countdown's timer and the reads that failed in a row. */
const ticks = new Map<string, ReturnType<typeof setInterval>>();
const failures = new Map<string, number>();

/** The rooms of each Cube a device can be put into, by its serial, as GET …/rooms gave them. */
export const cubeRooms = signal<Record<string, Room[]>>({});

/** loadCubeRooms reads the rooms a device of the Cube can be put into, its devices with them for the name offered; none on an error. */
export async function loadCubeRooms(serial: string): Promise<void> {
  if (serial === state.roomsOf) return; // the shown Cube's are the page's own, state.rooms (UiParts D216)
  const n = session();
  let list: Room[] = [];
  try {
    list = (await api<Room[] | null>("GET", cubePath(serial) + "/rooms?devices=real")) || [];
  } catch {
    list = [];
  }
  if (!stillSession(n)) return; // an answer after a sign-out: nothing of it for the next user
  cubeRooms.value = { ...cubeRooms.value, [serial]: list };
}

/** leftOf is the seconds left of a pairing that runs until until (ms). */
function leftOf(until: number): number {
  return Math.ceil((until - Date.now()) / 1000);
}

/**
 * apply takes the server's pairing state, the answer of GET, POST and
 * DELETE …/pairing. The devices of a pairing this page did not see
 * running are not shown; the end of one it saw is said. known is, for a
 * read, the devices the page listed when it asked (loadPairing): one
 * listed since, by a paired event or a later answer, that the answer
 * lacks joined after the answer was given, and stays listed, its form
 * the one it was (since).
 */
function apply(serial: string, st: PairingState | null, known?: NewDevice[]): void {
  const f = flowOf(serial);
  const was = f.active;
  f.active = !!st?.active;
  f.until = Date.parse(st?.until || "") || 0;
  if (f.active || was) listed(f, since(st?.paired || [], f.paired, known));
  if (f.active) f.form = false;
  if (was && !f.active) f.msg = { text: tn("pairing.ended", f.paired.length), ok: true };
  f.left = leftOf(f.until);
  if (f.active) startTick(serial);
  else stopTick(serial);
}

/**
 * since is the devices an answer lists, answer, and after them those the
 * page lists now, now, that it did not list when it asked, known, and
 * the answer lacks: a device a paired event listed after the read was
 * asked for, whose answer the page takes after the event (AP SteadyTests
 * v1.2, Block 21), which would otherwise be dropped until the pairing
 * ends, since the stream open on the shown Cube brings no other read.
 * Without known, the answer's.
 */
function since(answer: NewDevice[], now: NewDevice[], known?: NewDevice[]): NewDevice[] {
  if (!known) return answer;
  const has = (list: NewDevice[], d: NewDevice) => list.some((x) => x.rf_address === d.rf_address);
  const later = now.filter((d) => !has(known, d) && !has(answer, d));
  return later.length ? [...answer, ...later] : answer;
}

/**
 * loadPairing asks for a Cube's pairing state. Three failures in a row
 * while it runs end the countdown, which would ask on forever, and say
 * why. Its answer keeps the devices listed after it was asked for
 * (apply).
 */
export async function loadPairing(serial: string): Promise<void> {
  const f = flowOf(serial);
  const sn = session();
  const known = f.paired;
  try {
    const st = await api<PairingState>("GET", cubePath(serial) + "/pairing");
    if (!stillSession(sn)) return; // an answer after a sign-out starts no countdown
    failures.set(serial, 0);
    apply(serial, st, known);
  } catch (e) {
    if (!stillSession(sn)) return;
    const n = (failures.get(serial) || 0) + 1;
    failures.set(serial, n);
    if (f.active && n >= 3) {
      f.active = false;
      stopTick(serial);
      f.msg = { text: t("pairing.failed", { error: (e as Error).message }), ok: false };
    }
  }
}

/** write runs fn, Start's or Stop's request, as key: its button turns, and a second click sends nothing. */
async function write(serial: string, key: "start" | "stop", fn: () => Promise<void>): Promise<void> {
  const f = flowOf(serial);
  if (f.sending) return;
  f.sending = key;
  try {
    await fn();
  } finally {
    f.sending = "";
  }
}

/** secondsOk says whether a time typed is one a Cube takes: a whole number of seconds from 1 to 255 (cube.MaxInclusion). */
export function secondsOk(v: string): boolean {
  const n = Number(v);
  return v.trim() !== "" && Number.isInteger(n) && n >= 1 && n <= 255;
}

/** startPairing sends Start: POST …/pairing with the seconds, which the form checked; the joined devices' forms start afresh. */
export function startPairing(serial: string): Promise<void> {
  const f = flowOf(serial);
  const seconds = Number(f.seconds);
  return write(serial, "start", async () => {
    const n = session();
    let st: PairingState;
    try {
      st = await api<PairingState>("POST", cubePath(serial) + "/pairing", { seconds });
    } catch (e) {
      if (!stillSession(n)) return;
      f.msg = { text: t("pairing.failed", { error: (e as Error).message }), ok: false };
      return;
    }
    if (!stillSession(n)) return; // an answer after a sign-out starts no countdown
    f.msg = null;
    f.puts = {};
    f.active = false;
    apply(serial, st);
    loadCubeRooms(serial);
  });
}

/** stopPairing sends Stop: DELETE …/pairing. */
export function stopPairing(serial: string): Promise<void> {
  return write(serial, "stop", async () => {
    const n = session();
    try {
      const st = await api<PairingState>("DELETE", cubePath(serial) + "/pairing");
      if (stillSession(n)) apply(serial, st);
    } catch (e) {
      if (stillSession(n)) flowOf(serial).msg = { text: t("pairing.failed", { error: (e as Error).message }), ok: false };
    }
  });
}

/**
 * startTick counts down every second while pairing runs. The stream
 * reports the shown Cube's devices and the end; another Cube's, or the
 * shown one's without a stream or past its time, are asked for every
 * third second. It stops where the Cube is no longer served.
 */
function startTick(serial: string): void {
  if (ticks.has(serial)) return;
  let n = 0;
  ticks.set(serial, setInterval(() => {
    if (!state.cubes.some((c) => c.serial === serial)) {
      stopTick(serial);
      return;
    }
    n++;
    const f = flowOf(serial);
    f.left = leftOf(f.until);
    const watched = serial === state.serial && state.eventsOpen && Date.now() < f.until;
    if (!watched && n % 3 === 0) loadPairing(serial);
  }, 1000));
}

/** stopTick ends a Cube's countdown. */
function stopTick(serial: string): void {
  const id = ticks.get(serial);
  if (id !== undefined) clearInterval(id);
  ticks.delete(serial);
}

/** pairingEvent takes the shown Cube's pairing_started and pairing_ended events (app.js connectEvents): the state is asked for. */
export function pairingEvent(serial: string): void {
  loadPairing(serial);
}

/** pairingJoined takes a paired event of the shown Cube: the device is listed at once, or, with no pairing known to run, the state is asked for. */
export function pairingJoined(serial: string, dev: NewDevice | undefined): void {
  const f = flowOf(serial);
  if (!f.active) {
    loadPairing(serial);
    return;
  }
  if (dev && !f.paired.some((x) => x.rf_address === dev.rf_address)) listed(f, [...f.paired, dev]);
}

/**
 * forgetPairing forgets every Cube's pairing as this page kept it, at a
 * sign-out and for another user's sign-in after a 401 (forgetSession of
 * login.js): the flows with what their joined devices' forms hold, their
 * countdowns stopped, and the Cubes' rooms, so that the next user finds
 * nothing of the last one's.
 */
export function forgetPairing(): void {
  for (const serial of [...ticks.keys()]) stopTick(serial);
  failures.clear();
  flows.clear();
  cubeRooms.value = {};
}
