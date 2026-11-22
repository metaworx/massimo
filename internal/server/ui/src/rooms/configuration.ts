// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/configuration.ts
 *
 * What a room's configuration keeps and sends (Config.tsx): per room of
 * its Cube, live, the tab shown, the room's configuration and week
 * programme as loaded with the programme's tag, the result line and the
 * write under way, the tab and the Copy tab's choice kept across a close;
 * its opening, which loads both, and its closing, which keeps every
 * form's edit (each tab but Copy, a command, a form of its own,
 * shared/forms.ts, by cfgFormKey); a tab's form opened from
 * anywhere with the focus in it; and the writes of its
 * tabs, line for line as config.js sent them: the room's name, its
 * temperatures and radiator settings, a device's name, room, offset and
 * display, a device removed from the Cube, the week programme with its
 * tag, and the copy to other rooms. One write runs at a time, so that a
 * double click sends one radio command.
 */

import type { Budget, Device, DeviceResult, ProgramWithTag, RoomConfig } from "../api";
import type { Outcome } from "../shared/action";
import { focusForm, useForm, type Form, type FormName } from "../shared/forms";
import { liveMap } from "../shared/live";
import { byteLength, NAME_BYTES } from "../shared/name";
import { session, stillSession } from "../shared/session";
import { removeFromCube } from "./send";
import { cfgFormKey, openRoom, roomOf } from "./store";
import { can } from "../shared/me";

/** The common fields of a room's configuration on the Temperatures tab, as GET and PUT /rooms/{id}/config have them, in the order they are sent. */
export const TEMP_FIELDS = ["comfort_temp", "eco_temp", "window_open_temp", "window_open_minutes", "max_setpoint", "min_setpoint"] as const;

/** The radiator thermostats' fields on the Devices tab, in the order they are sent. */
export const VALVE_FIELDS = ["boost_minutes", "boost_valve_pct", "decalc_day", "decalc_hour", "max_valve_pct", "valve_offset_pct"] as const;

/** A field of the room's configuration that the tabs edit. */
export type ConfigField = (typeof TEMP_FIELDS)[number] | (typeof VALVE_FIELDS)[number];

/** The result line under a tab, with reload where the programme changed elsewhere and Load the current programme follows it. */
export type ConfigResult = Outcome & { reload?: boolean };

/**
 * A week programme as the configuration keeps it: the editor's model
 * (MassimoWeekProgram.model) and the tag it was loaded with, which its
 * write sends back; replace where it was loaded again on the user's word
 * (Load the current programme), in place of the one edited.
 */
export interface Programme {
  model: WeekModel;
  etag: string;
  replace?: boolean;
}

/** programmeOf is a programme as GET /rooms/{id}/program gives it, as the configuration keeps it; null gives every day 17 °C, as config.js had it. */
function programmeOf(prog: ProgramWithTag | null, replace?: boolean): Programme {
  return { model: MassimoWeekProgram.model(prog), etag: (prog && prog.etag) || "", replace };
}

/** What a room's configuration keeps, live. */
export interface ConfigState {
  /** The tab shown, room, temps, devices, program or copy, kept across a close. */
  tab: string;
  /** Its loads are under way. */
  loading: boolean;
  /** Why it could not be loaded, "" while it could. */
  error: string;
  /** The room's configuration as loaded, and as the writes since changed it. */
  cfg: RoomConfig | null;
  /** The week programme as loaded, with its tag; null before. */
  program: Programme | null;
  /** The result line of the last write. */
  result: ConfigResult | null;
  /** The Copy tab's choice, a command's and no settings form: the rooms chosen by their ids, and which settings, kept across a close. */
  copyRooms: string[];
  copyWhat: string[];
  /** The key of the write under way, null while none runs. */
  sending: string | null;
  /** Counts the loads, so that an answer to an earlier one is dropped. */
  seq: number;
}

/** What each room's configuration keeps, by its Cube and its id. */
const configs = liveMap<ConfigState>(() => ({
  tab: "",
  loading: false,
  error: "",
  cfg: null,
  program: null,
  result: null,
  copyRooms: [],
  copyWhat: ["temps", "radiator"],
  sending: null,
  seq: 0,
}));

/** configOf is what the room's configuration keeps, by its Cube and its id. */
export function configOf(id: number): ConfigState {
  return configs.of((state.roomsOf || "") + ":" + id);
}

/** The tab a configuration opens on where none is asked for: the Room tab is a rename, which the role may not do, so the temperatures come first then. */
function firstTab(): string {
  return can("rename") ? "room" : "temps";
}

/**
 * openConfig opens the room's configuration on tab, else on the tab it
 * was on when it closed, else on its first,
 * the device list closed (one of the two at a time) and the tile's menus
 * (el) closed, and loads the room's configuration and its week programme.
 */
export async function openConfig(room: PageRoom, el: HTMLElement | null, tab?: string): Promise<void> {
  closeMenu(el);
  const ui = roomUI(room);
  ui.open = false;
  const cs = configOf(room.id);
  cs.tab = tab || (cs.tab && (cs.tab !== "room" || can("rename")) ? cs.tab : firstTab());
  cs.loading = true;
  cs.error = "";
  cs.result = null;
  cs.sending = null;
  const seq = ++cs.seq;
  const n = session();
  ui.config = true;
  try {
    const [cfg, prog] = await Promise.all([
      api<RoomConfig>("GET", base() + "/rooms/" + room.id + "/config"),
      api<ProgramWithTag>("GET", base() + "/rooms/" + room.id + "/program").catch(() => null),
    ]);
    if (seq !== cs.seq || !stillSession(n)) return; // a later opening's, or an answer after a sign-out
    cs.cfg = cfg;
    cs.program = programmeOf(prog); // its tag sent back with the write (docs/api-v1.md, "The programme's tag")
  } catch (e) {
    if (seq !== cs.seq || !stillSession(n)) return;
    cs.error = (e as Error).message;
  } finally {
    if (seq === cs.seq) cs.loading = false;
  }
}

/** closeConfig closes the room's configuration; what is typed in its tabs stays, each in its form. */
export function closeConfig(room: PageRoom): void {
  roomUI(room).config = false;
}

/**
 * openForm shows a form of a room's configuration and puts the focus into
 * it, from any page (openRoom): the configuration opened on tab, or shown
 * on it where it is open, and the focus put into the form of key
 * (focusForm) once it is drawn.
 */
export function openForm(serial: string, id: number, tab: string, key: string): void {
  openRoom(serial, id, (room) => {
    if (roomUI(room).config) configOf(room.id).tab = tab;
    else void openConfig(room, null, tab);
    focusForm(key);
  });
}

/**
 * useConfigForm is a form of the room's configuration (useForm) by its
 * key, cfgFormKey of form, on tab, named name in a problem list's item,
 * whose click opens it from anywhere (openForm).
 */
export function useConfigForm(room: PageRoom, form: string, tab: string, name: FormName): Form {
  const serial = state.roomsOf || "";
  const key = cfgFormKey(serial, room.id, form);
  return useForm({ key, name, open: () => openForm(serial, room.id, tab, key) });
}

/** cancelOf is a form's Cancel: its changes reset, the configuration closed. */
export function cancelOf(form: Form, room: PageRoom): () => void {
  return () => {
    form.reset();
    closeConfig(room);
  };
}

/** saving runs fn, a form's write, as the form's save (useForm), and resolves to whether it was taken, for a question that waits for it. */
export function saving(form: Form, fn: () => Promise<boolean>): Promise<boolean> {
  let done = Promise.resolve(false);
  form.save(() => (done = fn()));
  return done;
}

/** say sets the result line under the tab. */
function say(cs: ConfigState, text: string, ok: boolean, reload?: boolean): void {
  cs.result = { text, ok, reload };
}

/**
 * configWrite runs fn, a write of the configuration, as key: every Send
 * of the configuration is off meanwhile, key's turning, and a write asked
 * for while one runs sends nothing and resolves to false.
 */
export async function configWrite(cs: ConfigState, key: string, fn: () => Promise<boolean>): Promise<boolean> {
  if (cs.sending) return false;
  cs.sending = key;
  try {
    return await fn();
  } finally {
    cs.sending = null;
  }
}

/** stored is a field of the room's configuration as loaded, as a number field's text. */
export function stored(cs: ConfigState, f: ConfigField): string {
  const v = cs.cfg ? cs.cfg[f] : undefined;
  return v === undefined || v === null ? "" : String(v);
}

/**
 * changedFields is the body of a room config write: the fields whose
 * draft differs from the configuration as loaded, in the order of
 * fields; an emptied field is left out, the day of descaling sent as its
 * word, the others as numbers.
 */
export function changedFields(cs: ConfigState, fields: readonly ConfigField[], drafts: Partial<Record<ConfigField, { value: string }>>): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  for (const f of fields) {
    const d = drafts[f];
    if (!d || d.value.trim() === "") continue;
    const was = cs.cfg ? cs.cfg[f] : undefined;
    if (f === "decalc_day") {
      if (d.value !== String(was)) body[f] = d.value;
    } else if (Number(d.value) !== Number(was)) {
      body[f] = Number(d.value);
    }
  }
  return body;
}

/** The state of a tab's Send while the configuration writes: busy where key's write runs, off where another's does. */
export function sending(cs: ConfigState, key: string): { busy: boolean; other: boolean } {
  return { busy: cs.sending === key, other: !!cs.sending && cs.sending !== key };
}

/** sourceName is the name of the thermostat a room's values come from, by its radio address, else the address. */
export function sourceName(cfg: RoomConfig | null, src: string | undefined): string {
  if (!src) return "";
  const dev = (cfg?.devices || []).find((d) => d.rf_address === src);
  return dev && dev.name ? dev.name : src;
}

/** A write's answer as far as the configuration reads it: whether every device took it, per device how, the radio's budget, and for the programme its new tag. */
interface Answer {
  ok?: boolean;
  discarded?: boolean;
  confirmation?: string;
  etag?: string;
  devices?: DeviceResult[];
  budget?: Partial<Budget>;
}

/** deviceResults are the lines of a room config write's answer: whether every device took it, then per device. */
function deviceResults(label: string, res: Answer | null): string {
  const lines = [t(res && res.ok ? "result.ok" : "cfg.notAll", { label })];
  for (const d of (res && res.devices) || []) {
    const name = deviceName(d as Device);
    if (d.ok) lines.push(t("cfg.devOk", { name }));
    else if (d.discarded) lines.push(t("cfg.devDiscarded", { name }));
    else lines.push(t("cfg.devFailed", { name, error: d.error || t("cfg.failed") }));
  }
  if (res && res.budget) lines.push(t("cfg.budgetLine", { duty: res.budget.duty_cycle_pct ?? "", slots: res.budget.free_memory_slots ?? "" }));
  return lines.join("\n");
}

/**
 * sendRoomName renames the room (PUT /rooms/{id}) and loads the rooms
 * again; it resolves to whether the name was taken, also where it is the
 * room's already.
 */
export async function sendRoomName(room: PageRoom, cs: ConfigState, typed: string): Promise<boolean> {
  const name = typed.trim();
  if (!name) {
    say(cs, t("cfg.nameRequired"), false);
    return false;
  }
  if (byteLength(name) > NAME_BYTES) {
    say(cs, t("cfg.nameTooLong", { n: NAME_BYTES }), false);
    return false;
  }
  if (name === room.name) {
    say(cs, t("cfg.nameUnchanged"), true);
    return true;
  }
  say(cs, t("cfg.renaming"), true);
  const n = session();
  try {
    await api("PUT", base() + "/rooms/" + room.id, { name });
    if (!stillSession(n)) return false; // an answer after a sign-out says nothing
    say(cs, t("cfg.renamed", { name }), true);
    await loadRooms();
    return true;
  } catch (e) {
    if (!stillSession(n)) return false;
    say(cs, t("cfg.renameFailed", { error: (e as Error).message }), false);
    return false;
  }
}

/**
 * sendRoomConfig writes body, the fields that differ from the room's
 * configuration as loaded (PUT /rooms/{id}/config), which then counts
 * them as loaded where every device took them; label names it in the
 * result line. It resolves to whether every device took it: one that was
 * discarded or failed in part is a failed save, and the configuration
 * keeps what it had, since the answer does not say what each device
 * stores.
 */
export async function sendRoomConfig(room: PageRoom, cs: ConfigState, body: Record<string, unknown>, label: string): Promise<boolean> {
  if (Object.keys(body).length === 0) {
    say(cs, t("cfg.nothingChanged", { label }), true);
    return true;
  }
  say(cs, t("result.waiting", { label }), true);
  const n = session();
  let ok = false;
  try {
    const res = await api<Answer | null>("PUT", base() + "/rooms/" + room.id + "/config", body);
    if (!stillSession(n)) return false; // an answer after a sign-out says nothing
    ok = !!(res && res.ok);
    say(cs, deviceResults(label, res), ok);
    if (ok && cs.cfg) cs.cfg = { ...cs.cfg, ...body };
    state.details = {};
  } catch (e) {
    if (!stillSession(n)) return false;
    say(cs, errorText(label, e as Error), false);
  }
  loadCubes().catch(() => {});
  return ok;
}

/** What a device's Send writes: meta, the name and the room, through PUT /devices/{rf}; cfg, the offset and the display, through its config. */
export interface DeviceChanges {
  meta: { name?: string; room_id?: number; copy?: boolean };
  cfg: { temp_offset?: number; display_actual?: boolean };
}

/**
 * sendDevice writes what changed on a device of the room (changes), its
 * config first, then its name and its room; a move to another room is
 * asked first (moveConfirmed), and where the role may not configure it
 * moves without the room's settings. It resolves to whether all of it was
 * taken; a config the Cube discarded is not counted as stored. After a
 * change of the name or the room the rooms load again.
 */
export async function sendDevice(room: PageRoom, cs: ConfigState, d: Device, changes: DeviceChanges): Promise<boolean> {
  const label = deviceName(d);
  const { meta, cfg } = changes;
  if (meta.name !== undefined && byteLength(meta.name) > NAME_BYTES) {
    say(cs, t("cfg.devNameTooLong", { name: label, n: NAME_BYTES }), false);
    return false;
  }
  if (meta.room_id !== undefined && !can("configure")) meta.copy = false; // the copy of the room's settings needs configure
  if (Object.keys(meta).length === 0 && Object.keys(cfg).length === 0) {
    say(cs, t("cfg.nothingChanged", { label }), true);
    return true;
  }
  say(cs, t("result.waiting", { label }), true);
  const n = session();
  const lines: string[] = [];
  let ok = true;
  try {
    if (Object.keys(cfg).length) {
      const res = await api<Answer | null>("PUT", devicePath(d) + "/config", cfg);
      if (!stillSession(n)) return false; // an answer after a sign-out: nothing written of it, and no second request
      lines.push(resultText(t("cfg.devSettings", { name: label }), res));
      if (res && res.discarded) ok = false;
      else if (cs.cfg) {
        const devices = (cs.cfg.devices || []).map((x) => (x.rf_address === d.rf_address ? { ...x, ...cfg } : x));
        cs.cfg = { ...cs.cfg, devices };
      }
      state.details = {};
    }
    if (Object.keys(meta).length) {
      const res = await api<Answer | null>("PUT", devicePath(d), meta);
      const done = t(meta.room_id !== undefined ? "cfg.devMoved" : "cfg.devRenamed", { name: label });
      lines.push(res && res.budget ? t("cfg.dutySuffix", { text: done, duty: res.budget.duty_cycle_pct ?? "" }) : done);
      if (res && res.confirmation === "pending") lines.push(t("result.structurePending"));
    }
  } catch (e) {
    lines.push(errorText(label, e as Error));
    ok = false;
  }
  if (!stillSession(n)) return false; // an answer after a sign-out says nothing and reloads nothing
  say(cs, lines.join("\n"), ok);
  if (Object.keys(meta).length) {
    await loadRooms();
    if (!stillSession(n)) return false;
    const again = roomOf(room.id);
    if (again && roomUI(again).config) cs.tab = "devices";
  }
  loadCubes().catch(() => {});
  return ok;
}

/**
 * removeDevice removes a device of the room from the Cube (removeFromCube
 * of rooms/send.ts), which loads the rooms again, and says how it went on
 * the Devices tab, or on the tile where the configuration closed
 * meanwhile.
 */
export async function removeDevice(room: PageRoom, cs: ConfigState, d: Device): Promise<boolean> {
  say(cs, t("device.removing", { device: deviceName(d) }), true);
  const n = session();
  const res = await removeFromCube(d);
  if (!res || !stillSession(n)) return false; // an answer after a sign-out says nothing
  const again = roomOf(room.id);
  if (!again || !roomUI(again).config) {
    showTileResult(room.id, res.text, res.ok);
    return res.ok;
  }
  cs.tab = "devices";
  say(cs, res.text, res.ok);
  return res.ok;
}

/** dayShort is a weekday's short name in the page's language. */
function dayShort(day: string): string {
  return t("dayShort." + day);
}

/**
 * sendProgram writes the days of the programme edited that differ from
 * the one loaded, naming its tag (If-Match); once the Cube took them they
 * count as loaded, the tag is the answer's, and it is the configuration's
 * programme as stored from then on (cs.program). A programme changed
 * elsewhere since it was loaded, or a write still on its way, sends
 * nothing (412): the edits stay, and the result line offers to load it as
 * it is now. It resolves to whether the days were taken.
 */
export async function sendProgram(room: PageRoom, cs: ConfigState, edited: Programme): Promise<boolean> {
  const W = MassimoWeekProgram;
  const p = edited.model;
  const body: Record<string, { temp: number; until: string }[]> = {};
  for (const day of W.dirtyDays(p)) {
    W.sortPoints(p.days[day]); // a time typed may not have been left yet
    const err = W.validateDay(p.days[day], t);
    if (err) {
      say(cs, t("prog.dayError", { day: dayShort(day), error: err }), false);
      return false;
    }
    body[day] = W.copyPoints(p.days[day]);
  }
  if (Object.keys(body).length === 0) {
    say(cs, t("prog.nothing"), true);
    return true;
  }
  const label = t("prog.label", { days: Object.keys(body).map(dayShort).join(", ") });
  say(cs, t("result.waiting", { label }), true);
  const n = session();
  let ok = false;
  try {
    const tag = edited.etag;
    const res = await api<Answer | null>("PUT", base() + "/rooms/" + room.id + "/program", body, tag ? { "If-Match": '"' + tag + '"' } : undefined);
    if (!stillSession(n)) return false; // an answer after a sign-out says nothing
    ok = !(res && res.discarded);
    if (ok) for (const d of Object.keys(body)) p.orig[d] = W.copyPoints(body[d]);
    if (res && res.etag) edited.etag = res.etag;
    if (ok) {
      // the programme sent is the one stored now: the Copy tab copies it,
      // and a programme loaded before it at an opening no longer counts
      edited.replace = undefined;
      cs.program = edited;
    }
    say(cs, resultText(label, res), ok);
    state.details = {};
  } catch (e) {
    if (!stillSession(n)) return false;
    if ((e as { status?: number }).status === 412) {
      // Changed elsewhere since it was loaded, or a write still on its
      // way: nothing was sent; the edits stay until the reload.
      say(cs, t("prog.outdated"), false, true);
      return false;
    }
    say(cs, errorText(label, e as Error), false);
  }
  loadCubes().catch(() => {});
  return ok;
}

/** reloadProgram loads the room's programme as it is now, with its tag, in place of the one edited. */
export async function reloadProgram(room: PageRoom, cs: ConfigState): Promise<void> {
  const n = session();
  try {
    const prog = await api<ProgramWithTag>("GET", base() + "/rooms/" + room.id + "/program");
    if (!stillSession(n)) return; // an answer after a sign-out: nothing of it for the next user
    cs.program = programmeOf(prog, true);
    say(cs, t("prog.reloaded"), true);
  } catch (e) {
    if (!stillSession(n)) return;
    say(cs, (e as Error).message, false);
  }
}

/** What the Copy tab copies: the rooms by their ids, and which of the room's stored settings. */
export interface CopyChoice {
  rooms: number[];
  temps: boolean;
  program: boolean;
  radiator: boolean;
}

/**
 * sendCopy writes this room's stored settings to every room chosen, one
 * room after another: the temperatures and the radiator settings in one
 * config write, the week programme as loaded as a second write. It
 * resolves to whether every room took all of it.
 */
export async function sendCopy(cs: ConfigState, k: CopyChoice): Promise<boolean> {
  const ids = k.rooms;
  if (ids.length === 0) {
    say(cs, t("copy.noRoom"), false);
    return false;
  }
  const program = k.program && cs.program;
  if (!k.temps && !k.radiator && !program) {
    say(cs, t("copy.noSetting"), false);
    return false;
  }
  const cfg = cs.cfg;
  if (!cfg || !cfg.source) {
    say(cs, t("copy.noSource"), false);
    return false;
  }
  const fields: ConfigField[] = [...(k.temps ? TEMP_FIELDS : []), ...(k.radiator ? VALVE_FIELDS : [])];
  const body: Record<string, unknown> = {};
  for (const f of fields) if (cfg[f] !== undefined && cfg[f] !== null) body[f] = cfg[f];
  const days = program ? MassimoWeekProgram.copyDays(program.model.orig) : null;
  const n = session();
  const lines: string[] = [];
  let allOk = true;
  for (const id of ids) {
    if (!stillSession(n)) return false; // signed out meanwhile: the next rooms are not written, nothing is said
    const target = state.rooms.find((r) => r.id === id);
    const name = target ? target.name : String(id);
    say(cs, lines.concat([t("copy.sending", { room: name })]).join("\n"), allOk);
    const parts: string[] = [];
    try {
      if (Object.keys(body).length) {
        const res = await api<Answer | null>("PUT", base() + "/rooms/" + id + "/config", body);
        if (!stillSession(n)) return false; // an answer after a sign-out: no programme's write after it
        parts.push(t(res && res.ok ? "copy.settingsOk" : "copy.settingsPartial"));
        if (!(res && res.ok)) allOk = false;
      }
      if (days) {
        const pr = await api<Answer | null>("PUT", base() + "/rooms/" + id + "/program", days);
        parts.push(t(pr && pr.discarded ? "copy.programDiscarded" : "copy.programOk"));
        if (pr && pr.discarded) allOk = false;
      }
      lines.push(t("copy.roomResult", { room: name, parts: parts.join(", ") }));
    } catch (e) {
      lines.push(t("copy.roomResult", { room: name, parts: parts.concat([(e as Error).message]).join(", ") }));
      allOk = false;
    }
  }
  if (!stillSession(n)) return false;
  state.details = {};
  say(cs, lines.join("\n"), allOk);
  loadCubes().catch(() => {});
  return allOk;
}
