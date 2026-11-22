// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/put.tsx
 *
 * The form that puts a device that is in no room into one (the capability
 * structure), as a part: wherever such a device is shown, a device that
 * joined while the Cube pairs, or one of the devices without a room. Its
 * name (with rename), offered by default for the room chosen until one is
 * typed (D200-A), one too long for the Cube marked on its field and not
 * sent (D201); the room, one of the rooms given; and for a thermostat the
 * switch that copies the room's settings (with configure, which the copy
 * needs as well), on by default; then PUT …/devices/{rf} and its outcome.
 * What is typed, the request under way and the outcome are kept by the
 * caller (newPut), so that they outlive the form's being drawn again or
 * in another place.
 */

import { useEffect, useLayoutEffect, useRef } from "preact/hooks";
import type { Device, Room } from "../api";
import type { Outcome } from "./action";
import { ResultLine } from "./badge";
import { FormFooter, Select, Switch, TextField } from "./field";
import { useLang } from "./lang";
import { live } from "./live";
import { byteLength, defaultDeviceName, NAME_BYTES } from "./name";
import { cubePath } from "./path";
import { session, stillSession } from "./session";
import { can } from "./me";

/** What a form that puts a device into a room keeps: the name typed, the room and the copy chosen, its request under way, its outcome, and done once the device is in its room. */
export interface Put {
  /** The name typed; the one offered while typed is false. */
  name: string;
  typed: boolean;
  /** The room chosen, undefined for the first of the rooms. */
  room: number | undefined;
  copy: boolean;
  busy: boolean;
  result: Outcome | null;
  done: boolean;
}

/** newPut is a form's state before anything is typed, live, for the caller to keep. */
export function newPut(): Put {
  return live<Put>({ name: "", typed: false, room: undefined, copy: true, busy: false, result: null, done: false });
}

/** The device as the form reads it: its type, radio address and serial, a device of a Cube's list or one that joined. */
type Joined = { type: Device["type"]; rf_address: string; serial?: string; name?: string };

/** The answer of PUT …/devices/{rf}, as far as the form reads it: what of the room's settings was copied, the radio budget, a confirmation pending. */
type Answer = { copied?: { days?: number; mode_not_set?: boolean }; budget?: { duty_cycle_pct?: number; free_memory_slots?: number }; confirmation?: string };

/** offered is the name the form offers in room: the room's name, the device type's word in the page's language and the lowest free number (defaultDeviceName). */
function offered(dev: Joined, room: Room | undefined): string {
  if (!room) return "";
  return defaultDeviceName(room.name, typeName(dev.type), (room.devices || []).map((d) => d.name || ""));
}

/**
 * PutForm is the form for dev of the Cube serial: id is its fields' id's
 * beginning, rooms the Cube's rooms it offers, put what it keeps
 * (newPut); reload loads what the request changed, before onDone, once
 * the device is in its room, or onFail, after a refusal; onCancel, where
 * given, is its Cancel, and focus puts the focus into its first field
 * when it comes.
 */
export function PutForm({ id, serial, dev, rooms, put, reload, onDone, onFail, onCancel, focus }: {
  id: string;
  serial: string;
  dev: Joined;
  rooms: Room[];
  put: Put;
  reload: () => Promise<unknown>;
  onDone?: () => void;
  onFail?: () => void;
  onCancel?: () => void;
  focus?: boolean;
}) {
  useLang(); // its texts follow the page's language
  const nameRef = useRef<HTMLInputElement>(null);
  const formRef = useRef<HTMLFormElement>(null);
  // the focus in the first field, in the drawing that shows the form
  useLayoutEffect(() => {
    if (focus) formRef.current?.querySelector<HTMLElement>("input, select")?.focus();
  }, [focus]);
  // The room is pinned at the first drawing, the first of the rooms, and
  // again where the one chosen is gone: a reload that brings the rooms in
  // another order does not change it.
  useEffect(() => {
    if (rooms.length && !rooms.some((r) => r.id === put.room)) put.room = rooms[0].id;
  }, [rooms, put.room, put]);
  const room = rooms.find((r) => r.id === put.room) || rooms[0];
  const name = put.typed ? put.name : offered(dev, room);
  const long = byteLength(name.trim()) > NAME_BYTES;
  const thermostat = isThermostat(dev as Device);
  const none: Outcome | null = rooms.length ? null : { text: t("put.noRooms"), ok: false };
  const submit = async (ev: Event) => {
    ev.preventDefault();
    if (put.busy || !room) return;
    // Without rename the field is hidden: the server names the device by
    // its default.
    const nm = can("rename") ? name.trim() : "";
    const label = nm || deviceName(dev as Device);
    const body: { room_id: number; copy?: boolean; name?: string } = { room_id: room.id };
    if (thermostat) body.copy = can("configure") && put.copy;
    if (nm) {
      if (long) {
        nameRef.current?.focus(); // its field says why, as the pairing time's does
        return;
      }
      body.name = nm;
    }
    put.busy = true;
    put.result = { text: t("put.putting", { device: label, room: room.name }), ok: true };
    const n = session();
    let res: Answer | null;
    try {
      res = await api<Answer | null>("PUT", cubePath(serial) + "/devices/" + encodeURIComponent(dev.serial || dev.rf_address), body);
    } catch (err) {
      if (!stillSession(n)) return; // an answer after a sign-out says nothing and reloads nothing
      // A copy that stopped leaves the device in its room: the reload
      // shows where it is, and onFail says the error where it is seen.
      put.busy = false;
      put.result = { text: errorText(label, err as Error), ok: false };
      await reload().catch(() => {});
      if (!stillSession(n)) return; // signed out during the reload: nothing said
      if (onFail) onFail();
      return;
    }
    if (!stillSession(n)) return;
    let text = t("put.done", { device: label, room: room.name });
    if (res?.copied) {
      text += " " + t("put.copied", { days: res.copied.days ?? 0 });
      if (res.copied.mode_not_set) text += " " + t("put.modeNotSet");
    }
    if (res?.budget) text = t("result.budget", { text, duty: res.budget.duty_cycle_pct ?? 0, slots: res.budget.free_memory_slots ?? 0 });
    if (res?.confirmation === "pending") text += " " + t("result.structurePending");
    put.result = { text, ok: true };
    put.done = true;
    put.busy = false;
    await reload().catch(() => {});
    if (!stillSession(n)) return; // signed out during the reload: nothing said
    if (onDone) onDone();
  };
  return (
    <form ref={formRef} class="pair-device-form needs-structure" onSubmit={submit}>
      <TextField label={t("put.name")} id={id + "-name"} name="name" cls="needs-rename" inputRef={nameRef} value={name}
        set={(v) => {
          put.name = v;
          put.typed = true;
        }}
        limit={{ text: t("name.long", { n: NAME_BYTES }), over: long, cls: "pair-name-long" }} />
      <Select label={t("put.room")} id={id + "-room"} name="room" wide options={rooms.map((r) => [String(r.id), r.name])}
        value={room ? String(room.id) : ""} onChange={(v) => {
          put.room = Number(v);
        }} />
      {thermostat && (
        <Switch name="copy" label={t("put.copy")} rowCls="pair-copy needs-configure" help="putCopy" checked={put.copy}
          onChange={(on) => {
            put.copy = on;
          }} />
      )}
      <ResultLine res={none || put.result} cls="small pair-result" role="status" />
      <FormFooter submit={t("put.submit")} busy={put.busy} off={!rooms.length} cancel={onCancel} />
    </form>
  );
}
