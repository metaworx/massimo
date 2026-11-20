// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/ConfigDevices.tsx
 *
 * The Devices tab of a room's configuration (Config.tsx): the radiator
 * thermostats' settings where the room has one, then a form per device,
 * its name, type and identifiers, what it has not confirmed, its name, its
 * offset and a wall thermostat's display, its room, which asks before a
 * move, and Remove from the Cube, which asks in the form. Each field that
 * shows stored data is a draft with its "changed meanwhile" notice.
 */

import type { Device } from "../api";
import { Notice } from "../shared/badge";
import { DeviceIds } from "../shared/device";
import { Field, FormFooter, Meanwhile, NumberField, Select, Switch, TextField, numberNorm, useDraft, type Draft } from "../shared/field";
import { Icon } from "../shared/icon";
import { Question, useAsk } from "../shared/question";
import {
  changedFields,
  closeConfig,
  configWrite,
  placeOf,
  removeDevice,
  sendDevice,
  sending,
  sendRoomConfig,
  stored,
  VALVE_FIELDS,
  type ConfigField,
  type ConfigState,
  type DeviceChanges,
} from "./configuration";
import { load, tickOf } from "./store";

const trim = (v: string) => v.trim();

/** A number field of the room's configuration: its field, label and help key, its range and step, and its unit (°C where none is given). */
export type NumberRow = { f: ConfigField; label: string; help: string; min: number; max: number; step: number; unit?: string };

/** NumberRows draws rows of number fields from their drafts. */
export function NumberRows({ room, rows, drafts }: { room: PageRoom; rows: NumberRow[]; drafts: Partial<Record<ConfigField, Draft<string>>> }) {
  return (
    <>
      {rows.map((r) => {
        const d = drafts[r.f];
        if (!d) return null;
        return (
          <NumberField key={r.f} label={t(r.label)} id={"cfg-" + room.id + "-" + r.f} name={r.f} help={r.help} min={r.min} max={r.max} step={r.step}
            unit={r.unit ? (r.unit.startsWith("unit.") ? t(r.unit) : r.unit) : "°C"} draft={d} />
        );
      })}
    </>
  );
}


/**
 * Valves are the radiator thermostats' settings of the room: boost, its
 * valve, the day and hour of descaling, the valve's maximum and offset,
 * each a draft against the configuration as loaded, with their Send.
 */
function Valves({ room, cs }: { room: PageRoom; cs: ConfigState }) {
  load.useLoad();
  void cs.ver;
  void cs.cfg;
  const drafts: Partial<Record<ConfigField, Draft<string>>> = {};
  drafts.boost_minutes = useDraft(stored(cs, "boost_minutes"), numberNorm, placeOf(cs, "valves", "boost_minutes"));
  drafts.boost_valve_pct = useDraft(stored(cs, "boost_valve_pct"), numberNorm, placeOf(cs, "valves", "boost_valve_pct"));
  drafts.decalc_day = useDraft(stored(cs, "decalc_day"), undefined, placeOf(cs, "valves", "decalc_day"));
  drafts.decalc_hour = useDraft(stored(cs, "decalc_hour"), numberNorm, placeOf(cs, "valves", "decalc_hour"));
  drafts.max_valve_pct = useDraft(stored(cs, "max_valve_pct"), numberNorm, placeOf(cs, "valves", "max_valve_pct"));
  drafts.valve_offset_pct = useDraft(stored(cs, "valve_offset_pct"), numberNorm, placeOf(cs, "valves", "valve_offset_pct"));
  const body = changedFields(cs, VALVE_FIELDS, drafts);
  const s = sending(cs, "valves");
  const day = drafts.decalc_day;
  const hour = drafts.decalc_hour;
  const id = (f: string) => "cfg-" + room.id + "-" + f;
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (cs.sending) return;
    void configWrite(cs, "valves", () => sendRoomConfig(room, cs, body, t("cfg.deviceSettings")));
  };
  return (
    <form class="cfg-form" onSubmit={submit}>
      <h4>{t("cfg.radiators")}</h4>
      <NumberRows room={room} drafts={drafts} rows={[
        { f: "boost_minutes", label: "cfg.boostMinutes", help: "boostMinutes", min: 0, max: 60, step: 5, unit: "unit.minutes" },
        { f: "boost_valve_pct", label: "cfg.boostValve", help: "boostValve", min: 0, max: 100, step: 5, unit: "%" },
      ]} />
      <Field label={t("cfg.decalc")} id={id("decalc_day")} help="decalc" draft={day}>
        <span class="inline field-control">
          <Select row={false} id={id("decalc_day")} name="decalc_day" draft={day} options={MassimoWeekProgram.DAYS.map((d) => [d, t("day." + d)])} />
          <NumberField row={false} name="decalc_hour" aria={t("cfg.decalcHour")} min={0} max={23} step={1} draft={hour} />
          <span class="muted">:00</span>
        </span>
      </Field>
      <Meanwhile draft={hour} />
      <NumberRows room={room} drafts={drafts} rows={[
        { f: "max_valve_pct", label: "cfg.maxValve", help: "maxValve", min: 0, max: 100, step: 1, unit: "%" },
        { f: "valve_offset_pct", label: "cfg.valveOffset", help: "valveOffset", min: 0, max: 100, step: 1, unit: "%" },
      ]} />
      <FormFooter cancel={() => closeConfig(room)} submit={t("action.send")} busy={s.busy} off={Object.keys(body).length === 0 || s.other} />
    </form>
  );
}

/**
 * DeviceForm is a device of the room: its name, type and identifiers,
 * what it has not shown of a write by its deadline, then its name, a
 * thermostat's offset and a wall thermostat's display, each a draft
 * against the device as listed and its configuration as loaded, and its
 * room; Send writes what differs, a move asked first in the form, and
 * Remove from the Cube asks in the form before it removes the device.
 */
function DeviceForm({ room, cs, d }: { room: PageRoom; cs: ConfigState; d: Device }) {
  load.useLoad();
  void cs.ver;
  const loaded = (cs.cfg?.devices || []).find((x) => x.rf_address === d.rf_address) || {};
  const form = "device:" + d.rf_address;
  const name = useDraft(d.name || "", trim, placeOf<string>(cs, form, "name"));
  const offset = useDraft(loaded.temp_offset === undefined ? "" : String(loaded.temp_offset), numberNorm, placeOf<string>(cs, form, "temp_offset"));
  const display = useDraft(!!loaded.display_actual, undefined, placeOf<boolean>(cs, form, "display_actual"));
  const inRoom = useDraft(String(room.id), undefined, placeOf<string>(cs, form, "room_id"));
  const move = useAsk<HTMLSelectElement>();
  const remove = useAsk<HTMLButtonElement>();
  const thermostat = isThermostat(d);
  const wall = d.type === "wall_thermostat";
  const changes: DeviceChanges = { meta: {}, cfg: {} };
  const typed = name.value.trim();
  if (typed && typed !== (d.name || "")) changes.meta.name = typed;
  if (Number(inRoom.value) !== room.id) changes.meta.room_id = Number(inRoom.value);
  if (thermostat && offset.value.trim() !== "" && Number(offset.value) !== Number(loaded.temp_offset)) changes.cfg.temp_offset = Number(offset.value);
  if (wall && display.value !== !!loaded.display_actual) changes.cfg.display_actual = display.value;
  const changed = Object.keys(changes.meta).length + Object.keys(changes.cfg).length > 0;
  const s = sending(cs, form);
  const label = deviceName(d);
  const target = state.rooms.find((r) => r.id === Number(inRoom.value));
  const unconfirmed = confirmItem(d);
  const id = (f: string) => "cfg-" + room.id + "-" + d.rf_address + "-" + f;
  // another write of the configuration under way: nothing is sent
  const send = () => (cs.sending ? Promise.resolve(false) : configWrite(cs, form, () => sendDevice(room, cs, d, changes)));
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (cs.sending || !changed) return;
    if (changes.meta.room_id !== undefined) {
      move.ask(); // a move is asked first
      return;
    }
    void send();
  };
  return (
    <form class="cfg-form cfg-device" onSubmit={submit}>
      <div class="device-main">
        <span class="device-name">{label}</span>
        <span class="muted expert-only">{typeName(d.type)}</span>
        <span class="muted expert-only"><DeviceIds serial={d.serial} rf={d.rf_address} /></span>
      </div>
      {unconfirmed && (
        <Notice cls="device-unconfirmed">
          <Icon symbol="icon-unconfirmed" cls="ni tone-warn" />
          <span>{t("device.notConfirmed", { fields: confirmFieldsText(d) })}</span>
        </Notice>
      )}
      <TextField label={t("cfg.name")} id={id("name")} name="name" help="deviceName" cls="needs-rename" maxLength={NAME_BYTES} draft={name} />
      {thermostat && (
        <NumberField label={t("cfg.tempOffset")} id={id("temp_offset")} name="temp_offset" help="tempOffset" min={-3.5} max={3.5} step={0.5} unit="°C" draft={offset} />
      )}
      {thermostat && wall && <Switch name="display_actual" label={t("cfg.displayActual")} help="displayActual" draft={display} />}
      <Select label={t("cfg.room")} id={id("room")} name="room_id" help="deviceRoom" cls="needs-structure" draft={inRoom} inputRef={move.ref}
        options={state.rooms.map((r) => [String(r.id), r.id === room.id ? t("cfg.thisRoom", { name: r.name }) : r.name])} />
      {move.asking && (
        <Question text={t("cfg.moveConfirm", { device: label, room: target ? target.name : inRoom.value })} yes={t("cfg.move")} no={t("action.keep")}
          onYes={async () => {
            if (cs.sending) return; // held to submit's guard: asked again once the other write is done
            move.done(false);
            await send();
          }}
          onNo={() => {
            inRoom.take();
            move.done(); // the focus back on the room's select
          }} />
      )}
      {remove.asking && (
        <Question text={t("device.removeConfirm", { device: label })} yes={t("device.remove")} no={t("action.keep")}
          onYes={async () => {
            if (cs.sending) return;
            await configWrite(cs, "remove", () => removeDevice(room, cs, d));
            remove.done(false);
          }}
          onNo={() => remove.done()} />
      )}
      <FormFooter submit={t("action.send")} busy={s.busy} off={!changed || s.other}
        extra={<button ref={remove.ref} type="button" class="needs-structure remove-device" aria-label={t("device.removeLabel", { device: label })}
          onClick={remove.ask}>{t("device.remove")}</button>} />
    </form>
  );
}

/**
 * ConfigDevices is the Devices tab: the radiator settings where the room
 * has a thermostat, then its devices, each a form, or that it has none.
 */
export function ConfigDevices({ room, cs }: { room: PageRoom; cs: ConfigState }) {
  load.useLoad();
  void tickOf(room.id).n;
  return (
    <div>
      {room.devices.some(isThermostat) && <Valves room={room} cs={cs} />}
      <h4>{t("cfg.devices")}</h4>
      {room.devices.length === 0 && <p class="muted">{t("room.noDevices")}</p>}
      {room.devices.map((d) => <DeviceForm key={d.rf_address} room={room} cs={cs} d={d} />)}
    </div>
  );
}
