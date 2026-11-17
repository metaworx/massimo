// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/Devices.tsx
 *
 * A device's row, in a tile's device list and among the devices without a
 * room: its type's icon before its name, its problems as icons that list
 * them, its badges in words, in expert mode its type, serial and radio
 * address; a thermostat's values, Lock and the device's Repair…, whose box
 * stays plain (repair.js) in an element the part leaves to it. The room's
 * virtual wall thermostat, in expert mode, has its two settings under it,
 * a settings form kept by the room (shared/forms.ts), whose unsaved
 * changes its row's pill and the tile's show. Built once per device and
 * updated in place, so that a focused Lock and an address typed stay when
 * data comes in.
 */

import type { ComponentChildren } from "preact";
import { useEffect, useState } from "preact/hooks";
import type { Device, DeviceDetail } from "../api";
import { deviceEntry, deviceIcon } from "../model/device";
import { actionRunner, type Outcome } from "../shared/action";
import { Age, Badge, ResultLine } from "../shared/badge";
import { DeviceIds } from "../shared/device";
import { FormFooter, Select, TextField, useDraft } from "../shared/field";
import { hasChanges, useForm } from "../shared/forms";
import { Icon } from "../shared/icon";
import { Island } from "../shared/island";
import { Problems } from "../shared/problems";
import { busyProps, useBusy } from "../shared/question";
import { deviceBadges, deviceItems, problemsLabel, vwtItems } from "./facts";
import { load, openSettings, vwtFormKey } from "./store";

/**
 * useDetail is a device's detail (deviceDetail of app.js, which keeps it in
 * state.details), where want says that its row shows some of it (a
 * thermostat's offset, the virtual thermostat's settings): asked for again
 * at every drawing of the row, so that the row follows once a write
 * emptied state.details; undefined before it came or where it failed,
 * error then what failed ("" once it came). again asks for it again and
 * resolves once it is there; where it cannot be had, the detail known
 * takes the settings sent (sent), which the server took.
 */
function useDetail(d: Device, want: boolean): {
  detail: DeviceDetail | undefined;
  error: string;
  again: (sent: DeviceDetail["settings"]) => Promise<void>;
} {
  const [detail, setDetail] = useState<DeviceDetail | undefined>(state.details[d.rf_address]);
  const [error, setError] = useState("");
  useEffect(() => {
    if (!want) return;
    let gone = false;
    deviceDetail(d).then((x) => {
      if (gone) return;
      setDetail(x);
      setError("");
    }, (e: Error) => {
      if (!gone) setError(e.message);
    });
    return () => {
      gone = true;
    };
  });
  const again = (sent: DeviceDetail["settings"]) => deviceDetail(d).then(setDetail, () => {
    setDetail((x) => (x ? { ...x, settings: { ...x.settings, ...sent } } : x));
  });
  return { detail, error, again };
}

/**
 * Cell is one cell of the values grid: label, number, unit, an icon and
 * the value's age after it. The number sits right-aligned in a fixed-width
 * span so the decimal points of a column line up; an empty label makes a
 * spacer. Temperatures (an icon given) are bold.
 */
function Cell({ label, value, unit, icon, at, stale }: { label: string; value: string; unit: string; icon?: string; at?: string; stale?: boolean }) {
  const aged = !!at && value !== "";
  return (
    <span class={aged ? "cell aged" : "cell"}>
      <span class="k">{label}</span>
      <span class={icon ? "num temp" : "num"}>{value}</span>
      <span class="u">{value === "" ? "" : unit}</span>
      {icon && value !== "" && <Icon symbol={icon} cls="di" />}
      {aged && !icon && <span />}
      {aged && <Age at={at} stale={stale} />}
    </span>
  );
}

/** A cell's values, as Cell takes them. */
type CellOf = { label: string; value: string; unit: string; icon?: string; at?: string };

/**
 * Values is a thermostat's two-column grid: valve and offset on the left,
 * set and actual on the right, a holiday's end under them; shown is the
 * device as its values are shown (an out-of-date one's last report).
 */
function Values({ d, shown, offset }: { d: Device; shown: Device; offset?: number }) {
  const s = shown.state || {};
  const left: CellOf[] = [];
  if (s.valve_pct !== undefined) left.push({ label: t("val.valve"), value: String(s.valve_pct), unit: "%", at: received(shown, "valve_pct") });
  if (isThermostat(shown)) left.push({ label: t("val.offset"), value: offset === undefined ? "" : (offset > 0 ? "+" : "") + fmtNum(offset, 1), unit: "°C" });
  const right: CellOf[] = [
    { label: t("val.set"), value: fmtTemp(s.setpoint, ""), unit: "°C", icon: "icon-set" },
    { label: t("val.actual"), value: fmtTemp(s.actual_temp, ""), unit: "°C", icon: "icon-is", at: received(shown, "actual_temp") },
  ];
  const rows = Math.max(left.length, right.length);
  const empty: CellOf = { label: "", value: "", unit: "" };
  const cells: CellOf[] = [];
  for (let i = 0; i < rows; i++) cells.push(left[i] || empty, right[i] || empty);
  const cls = "device-vals" + (d.reported ? "" : " out-of-date") + (rows === 0 ? " hidden" : "") + (shown.stale ? " stale" : "");
  return (
    <div class={cls}>
      {cells.map((c, i) => <Cell key={i} {...c} stale={shown.stale} />)}
      {s.until && (
        // The vacation end spans the grid, under the temperatures.
        <span class="cell wide">
          <span class="k">{t("val.until")}</span>
          <span>{fmtUntil(s.until)}</span>
        </span>
      )}
    </div>
  );
}

/**
 * Lock is a thermostat's Lock or Unlock, a write of its room's tile, busy
 * while it runs and off while another write of the tile does; a device in
 * no room turns by itself.
 */
function Lock({ d, ui }: { d: Device; ui?: RoomUI }) {
  const own = useBusy();
  const locked = !!d.state?.panel_locked;
  const key = "device:" + d.rf_address;
  const busy = ui ? ui.sending === key : own.busy;
  const off = !!ui?.sending && !busy;
  return (
    <button type="button" {...busyProps("needs-lock device-lock", busy, off)} onClick={() => {
      if (ui) lockDevice(d, !locked, null);
      else own.run(() => lockDevice(d, !locked, null));
    }}>
      {t(locked ? "device.unlock" : "device.lock")}
    </button>
  );
}

const rfNorm = (x: string) => x.trim().toLowerCase();

/**
 * VirtualSettings are the two settings of the room's virtual wall
 * thermostat: how a room without a wall thermostat takes its temperature
 * from its radiator thermostats, and its radio address. Each is a draft
 * against the value stored (useDraft), which comes with the device's
 * detail, kept by the room (useForm, vwtFormKey), so that what is typed
 * stays when the device list closes, the rooms load again or a device
 * reports, until it is saved, reset or the room or its virtual
 * thermostat is gone. A form that has its drafts draws them at once, from
 * the stored values they hold, before the detail comes again; one that
 * has none yet keeps its fields and Save off until it has come, or says
 * in its result line that it could not be had. Save writes both, then
 * asks for the detail again before the drafts take it; where that cannot
 * be had, they take what was sent.
 */
function VirtualSettings({ v, room, serial, detail, error, saved }: {
  v: Device;
  room: PageRoom;
  serial: string;
  detail?: DeviceDetail;
  error: string;
  saved: (sent: DeviceDetail["settings"]) => Promise<void>;
}) {
  const form = useForm({ key: vwtFormKey(serial, room.id), name: { key: "vwt.formName", args: { room: room.name } }, open: () => openSettings(serial, room.id) });
  const s = detail?.settings || {};
  const methodAt = form.place<string>("temp_method");
  const addrAt = form.place<string>("rf_address");
  const kept = { method: methodAt.known(), addr: addrAt.known() };
  // the stored values: the detail's, else those the drafts hold (a form drawn again); none before either
  const ready = !!detail || (kept.method !== undefined && kept.addr !== undefined);
  const method = useDraft<string>(detail ? s.temp_method || "median" : kept.method ?? "median", undefined, ready ? methodAt : undefined);
  const addr = useDraft<string>(detail ? s.rf_address || "" : kept.addr ?? "", rfNorm, ready ? addrAt : undefined);
  const [result, setResult] = useState<Outcome | null>(null);
  const changed = method.changed || addr.changed;
  if (ready) form.track(changed, [method, addr]); // before, the form keeps what it said last
  const id = (f: string) => "vwt-" + v.rf_address + "-" + f;
  const runner = actionRunner({ set: setResult });
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (!ready || !changed) return;
    const sent = { temp_method: method.value as "median" | "mean", rf_address: rfNorm(addr.value) };
    form.save(() => runner.run(async () => {
      await api("PUT", devicePath(v) + "/config", sent);
      state.details = {};
      await saved(sent); // its detail asked for again, before the drafts take it
    }, t("vwt.saved"), (e) => e));
  };
  const failed: Outcome | null = !ready && error ? { text: t("msg.cannotLoad", { error }), ok: false } : null;
  return (
    <form {...form.props} class="vwt-settings needs-configure" onSubmit={submit}>
      <Select label={t("vwt.method")} id={id("method")} hint={t("vwt.methodHint")} name="temp_method" draft={method} disabled={!ready}
        options={["median", "mean"].map((m) => [m, t("vwt." + m)])} />
      <TextField label={t("vwt.address")} id={id("address")} hint={t("vwt.addressHint")} name="rf_address" wide={false} maxLength={6}
        pattern="[0-9a-fA-F]{6}" placeholder={v.rf_address} draft={addr} disabled={!ready} />
      <FormFooter submit={t("action.save")} look={form.look} reset={form.reset} off={!ready || !changed} />
      <ResultLine res={result || failed} cls="tile-result" />
    </form>
  );
}

/**
 * DeviceRow is a device of a room's list or of the devices without a
 * room: ui is its room's tile's, absent for a device in no room; room
 * names the sources of a virtual wall thermostat by the room's devices;
 * actions follow Lock and Repair… in the row's actions, children follow
 * the row (a form, a question).
 */
export function DeviceRow({ d, ui, room, actions, children }: {
  d: Device;
  ui?: RoomUI;
  room?: PageRoom;
  actions?: ComponentChildren;
  children?: ComponentChildren;
}) {
  load.useLoad();
  const virtual = !!d.virtual;
  // An out-of-date device shows the values of its last report.
  const values = isThermostat(d) && !!(d.reported || (d.out_of_date && d.last_known));
  const { detail, error, again } = useDetail(d, values || virtual);
  const serial = state.roomsOf || "";
  // the virtual thermostat's row is an expert's, but while its settings have changes not saved
  const expert = virtual && !(room && hasChanges(vwtFormKey(serial, room.id)));
  const type = virtual ? t("vwt.type") : typeName(d.type);
  // the virtual thermostat's row lists its settings while unsaved (vwtItems), as the tile's head does
  const items = virtual && room ? [...deviceItems(d), ...vwtItems(room)] : deviceItems(d);
  const seen = Date.parse(d.last_seen || "");
  const seenLine = isNaN(seen) ? "" : t("age.lastSeen", { time: fmtShortTime(seen) });
  // the virtual thermostat's sources by their serial, the radio address of one not in the room
  const known = new Map((room?.devices || []).map((x) => [x.rf_address, x]));
  const from = virtual ? (d.sources?.actual_temp || []).map((rf) => known.get(rf)?.serial || rf) : [];
  const cfg = detail?.thermostat || detail?.wall_thermostat;
  const scope = repairScope(state.serial || "", "device", d.rf_address);
  return (
    <div class={virtual ? "device virtual" + (expert ? " expert-only" : "") : "device"} data-rf={d.rf_address}>
      <div class="device-head">
        <span class="device-type-icon" role="img" title={type} aria-label={type}>
          <Icon symbol={virtual ? "icon-room" : deviceIcon(d)} cls="ni" />
        </span>
        <span class="device-name">{d.name || t("device.unnamed") + " · " + (d.serial || d.rf_address)}</span>
        <span class="device-badges">
          <Problems entries={[deviceEntry(d, items)]} label={problemsLabel(items)} />
          {(d.mismatches || []).map((m, i) => (
            <Badge key={"m" + i} kind="warn mismatch-badge" text={t("mismatchBadge." + m.finding, mismatchArgs(m))} title={mismatchText(m)} />
          ))}
          {deviceBadges(d).map((b) => (
            <Badge key={b.kind + ":" + b.text} kind={b.kind} text={b.text} title={b.kind.includes("stale-badge") && seenLine ? seenLine : undefined}>
              <Age at={b.at} stale={d.stale} />
            </Badge>
          ))}
        </span>
      </div>
      <div class="device-sub muted expert-only">
        <span class="device-type">{type}</span>
        <span class="device-rf mono"><DeviceIds serial={d.serial} rf={d.rf_address} /></span>
        {seenLine && <span class="device-seen expert-only">{seenLine}</span>}
        {from.length > 0 && <span class="device-seen">{t("vwt.from", { devices: from.join(", ") })}</span>}
      </div>
      {values ? <Values d={d} shown={d.reported ? d : { ...d, state: d.last_known }} offset={cfg?.temp_offset} /> : <div class="device-vals hidden" />}
      <div class="device-actions">
        {isThermostat(d) && !virtual && <Lock d={d} ui={ui} />}
        {!virtual && (
          // The device's repairs (repair.js): its configuration for a
          // thermostat, its room by radio or in the list.
          <Island tag="span" cls="part-slot" keys={[scope, deviceName(d), isThermostat(d)]} fill={(el) => el.replaceChildren(
            repairButton(scope, "device", devicePath(d), { label: deviceName(d), skip: isThermostat(d) ? [] : ["config"] }))} />
        )}
        {actions}
      </div>
      {!virtual && <Island cls="part-slot" keys={[scope]} fill={(el) => el.replaceChildren(repairSlot(scope))} />}
      {virtual && room && <VirtualSettings v={d} room={room} serial={serial} detail={detail} error={error} saved={again} />}
      {children}
    </div>
  );
}

/**
 * DeviceList is a tile's device list: its devices, or that it has none,
 * then in expert mode the room's virtual wall thermostat.
 */
export function DeviceList({ room, ui }: { room: PageRoom; ui: RoomUI }) {
  load.useLoad();
  return (
    <>
      {room.devices.length === 0 && <p class="muted">{t("room.noDevices")}</p>}
      {room.devices.map((d) => <DeviceRow key={d.rf_address} d={d} ui={ui} room={room} />)}
      {room.vwt && <DeviceRow key={"vwt:" + room.id} d={room.vwt} ui={ui} room={room} />}
    </>
  );
}
