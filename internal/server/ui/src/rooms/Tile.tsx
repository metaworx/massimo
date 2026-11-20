// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/Tile.tsx
 *
 * A room's tile: its head with the room's name, its problems and its ⋯
 * menu, whose problems list its notices too (a room not ready, devices
 * not seen for long, the Cube's lines that differ from its room list;
 * rooms/facts.ts roomEntries); one line while the room's commands are
 * refused; the measured and
 * the set temperature with the stepper; the modes with More; the info and
 * the cog; the holiday form, the deletion's question, the room's repairs,
 * the result line, the device list and the configuration. Built once per
 * room and updated in place, so that an open menu, the focus, a click and
 * what is typed stay when data comes in. The tile's actions are the plain
 * ones (rooms.js, tiles.js), which set the tile's live state (roomUI); the
 * repairs (repair.js) stay plain, in an element the part leaves to them;
 * the configuration is a part of its own (Config.tsx).
 */

import { useLayoutEffect, useRef, useState } from "preact/hooks";
import { Age, Notice, ResultLine } from "../shared/badge";
import { DateTime, FormFooter, Stepper, Switch } from "../shared/field";
import { Icon, IconButton } from "../shared/icon";
import { Island } from "../shared/island";
import { Dropdown, MenuItem } from "../shared/menu";
import { Problems } from "../shared/problems";
import { busyProps, Question } from "../shared/question";
import { Config } from "./Config";
import { closeConfig, openConfig } from "./configuration";
import { DeviceList } from "./Devices";
import { problemsLabel, roomEntries } from "./facts";
import { load, tickOf } from "./store";

/** hid is the class hidden, with its space, where off. */
function hid(off: boolean): string {
  return off ? " hidden" : "";
}

/** The arrow keys of More move between these: its buttons that are on, and the Hold switch where shown. */
const MORE_ITEMS = ".menu-items button:not(:disabled), .menu-items .switch:not(.hidden) input";

/**
 * NotReadyLine is the tile's one line while the room's commands are
 * refused, until every device has reported (room.ready), as that is why a
 * change does nothing; its Show opens the pill, which lists the devices
 * that have not reported (roomEntries). The tile's other notices are the
 * pill's entries alone.
 */
function NotReadyLine({ room, tile }: { room: PageRoom; tile: () => HTMLElement | null }) {
  const open = () => {
    // After the click, which the page's close of an open menu on a click
    // outside it (components.js) sees first, as every other listener does.
    setTimeout(() => {
      const list = tile()?.querySelector<HTMLDetailsElement>(".room-problems details.problems");
      if (!list) return;
      list.open = true;
      list.querySelector("summary")?.focus();
    });
  };
  return (
    <Notice cls="tile-notices" hidden={room.ready} text={room.ready ? undefined : t("room.notReady")}>
      {!room.ready && " "}
      {!room.ready && <button type="button" class="tile-notices-show" onClick={open}>{t("action.show")}</button>}
    </Notice>
  );
}

/**
 * RoomMenu is the tile's ⋯ menu: a Boost, lock and unlock of the room's
 * thermostats, the deletion of an empty room, and Repair…, which opens the
 * room's repairs under the tile (repair.js). A room with devices but no
 * thermostat has nothing to offer, so its menu is hidden.
 */
function RoomMenu({ room, ui, tile }: { room: PageRoom; ui: RoomUI; tile: () => HTMLElement | null }) {
  const thermostats = room.devices.filter(isThermostat).length;
  const off = !!ui.sending;
  return (
    <Dropdown cls={(thermostats === 0 && room.devices.length > 0) ? "hidden" : undefined} label={t("room.actions")} title={t("room.actionsTitle")} summary="⋯">
      <MenuItem cls={"menu-boost needs-setpoint" + hid(!thermostats)} label={t("room.boost")} busy={ui.sending === "boost"} disabled={off} onClick={() => {
        // A boost opens the valves by the thermostats' boost settings for
        // their boost time; they then return to the mode they had.
        closeMenu(tile());
        leaveEditing(room);
        sendRoomSetpoint(room, { mode: "boost" }, t("label.boost"), "boost");
      }} />
      <MenuItem cls={"menu-lock needs-lock" + hid(!thermostats)} label={t("room.lock")} busy={ui.sending === "lock"} disabled={off}
        onClick={() => lockRoom(room, true, tile())} />
      <MenuItem cls={"menu-unlock needs-lock" + hid(!thermostats)} label={t("room.unlock")} busy={ui.sending === "unlock"} disabled={off}
        onClick={() => lockRoom(room, false, tile())} />
      <MenuItem cls={"menu-delete needs-structure" + hid(room.devices.length > 0)} label={t("room.delete")} onClick={() => {
        closeMenu(tile());
        ui.confirm = true;
      }} />
      <MenuItem cls="menu-repair needs-repair" label={t("repair.open")} onClick={() => {
        closeMenu(tile());
        toggleRepair(roomRepairScope(room), "room", base() + "/rooms/" + room.id);
      }} />
    </Dropdown>
  );
}

/** The state of AUTO and MANU: pressed, or partly where the room's thermostats differ and use it. */
function pressed(mode: string | null, modes: string[], m: string): { cls: string; aria: "true" | "false" | "mixed" } {
  const mixed = modes.length > 1;
  const on = !mixed && (mode === m || (m === "manual" && mode === "boost"));
  const part = mixed && modes.includes(m);
  return { cls: (on ? " active" : "") + (part ? " partial" : ""), aria: part ? "mixed" : on ? "true" : "false" };
}

/**
 * More is the tile's More menu (D99): Eco and Comfort, the room
 * thermostat's eco or comfort temperature in the room's mode, each with its
 * temperature once the menu has opened; the Hold switch while the room is
 * in Auto, which makes them hold theirs in Manual; and Holiday…, the
 * holiday form. On holiday it says until when (D99), and with the
 * thermostats' modes differing it is outlined where one is on holiday.
 */
function More({ room, ui, mode, modes, tile }: { room: PageRoom; ui: RoomUI; mode: string | null; modes: string[]; tile: () => HTMLElement | null }) {
  const [temps, setTemps] = useState<{ eco?: number; comfort?: number }>({});
  const mixed = modes.length > 1;
  const s = roomState(room);
  const holiday = mode === "vacation" ? t("mode.holidayUntil", { until: fmtUntil(s?.until) }) : "";
  const cls = "more-sum" + (!mixed && mode === "vacation" ? " active" : "") + (mixed && modes.includes("vacation") ? " partial" : "");
  const busy = ui.sending === "vacation" || ui.sending === "eco" || ui.sending === "comfort";
  const off = !!ui.sending; // the menu's buttons send, or open what sends
  return (
    <Dropdown cls="more-menu" summaryCls={cls} title={holiday || t("room.moreTitle")} busy={busy} arrows={MORE_ITEMS}
      onOpen={() => {
        const th = roomThermostat(room);
        if (th) presetTemps(th).then(setTemps).catch(() => {});
      }}
      summary={<><span class="more-label">{holiday || t("mode.more")}</span><Icon symbol="icon-chevron-down" cls="ni caret" /></>}>
      {(["eco", "comfort"] as const).map((which) => (
        <MenuItem key={which} cls={"menu-" + which} label={t("room." + which)} subCls="more-temp"
          sub={temps[which] === undefined ? "" : fmtTemp(temps[which])} disabled={off} onClick={() => sendPreset(room, which)} />
      ))}
      <Switch row={false} cls={"more-hold" + hid(mode !== "auto")} label={t("room.hold")} checked={ui.hold}
        onChange={(on) => {
          ui.hold = on;
        }} />
      <hr class="menu-sep" />
      <MenuItem cls="menu-holiday" label={t("room.holidayForm")} disabled={off} onClick={() => openVacation(room, tile())} />
    </Dropdown>
  );
}

/**
 * Vacation is the holiday form: a temperature kept until a date and time.
 * Its fields hold what is typed while it is open; it starts from the
 * room's setpoint and tomorrow morning each time it opens, the focus on
 * its end.
 */
function Vacation({ room, ui }: { room: PageRoom; ui: RoomUI }) {
  const open = ui.vacation;
  const [temp, setTemp] = useState("17");
  const [until, setUntil] = useState("");
  const untilRef = useRef<HTMLInputElement>(null);
  // biome-ignore lint/correctness/useExhaustiveDependencies: the form is filled from the room only when it opens; with room listed, every state event would write over what the user typed.
  useLayoutEffect(() => {
    if (!open) return;
    const s = roomState(room);
    setTemp(String(s && s.setpoint !== undefined ? s.setpoint : 17));
    setUntil(tomorrowMorning());
    untilRef.current?.focus();
  }, [open]);
  const send = (ev: Event) => {
    ev.preventDefault();
    const v = clampTemp(parseFloat(temp));
    const u = apiUntil(until);
    if (!u) return;
    closeVacation(room);
    sendRoomSetpoint(room, { mode: "vacation", temp: v, until: u }, t("label.vacation", { temp: fmtNum(v, 1), until: u }), "vacation");
  };
  return (
    <form class={"panel vacation-panel" + hid(!open)} onSubmit={send}>
      <div class="panel-row">
        <span class="muted">{t("room.vacation")}</span>
        <Stepper value={temp} set={setTemp} min={MIN_TEMP} max={MAX_TEMP} step={STEP} cls="vacation-temp">
          <span class="muted">{t("room.degUntil")}</span>
        </Stepper>
        <DateTime inputRef={untilRef} cls="vacation-until" step={1800} required value={until} set={setUntil} />
      </div>
      <FormFooter cancel={() => closeVacation(room)} submit={t("action.send")} off={!!ui.sending} />
    </form>
  );
}

/** DeleteQuestion asks, while the room's deletion is asked for, whether to delete it: Keep or Delete; Keep puts the focus back on ⋯. */
function DeleteQuestion({ room, ui, tile }: { room: PageRoom; ui: RoomUI; tile: () => HTMLElement | null }) {
  return (
    <div class="room-confirm">
      {ui.confirm && (
        <Question text={t("room.deleteConfirm")} yes={t("action.delete")} no={t("action.keep")}
          onYes={async () => {
            await deleteRoom(room);
            ui.confirm = false;
          }}
          onNo={() => {
            ui.confirm = false;
            tile()?.querySelector<HTMLElement>(".tile-head > details.menu > summary")?.focus();
          }} />
      )}
    </div>
  );
}

/** Tile is a room's tile. */
export function Tile({ room }: { room: PageRoom }) {
  load.useLoad();
  const tick = tickOf(room.id);
  void tick.n; // drawn again when a device of the room changed
  const ui = roomUI(room);
  const ref = useRef<HTMLElement>(null);
  const tile = () => ref.current;
  const th = roomThermostat(room);
  const s = roomState(room);
  const current = s && s.setpoint !== undefined ? s.setpoint : null;
  const modes = roomModes(room);
  const mode = modes.length > 1 ? null : s ? s.mode || null : null;
  const actual = roomActual(room);
  const src = roomActualDevice(room);
  const entries = roomEntries(room);
  const res = ui.result;
  const cfgOpen = ui.config;
  const off = !!ui.sending;
  const auto = pressed(mode, modes, "auto");
  const manual = pressed(mode, modes, "manual");
  // a write of the tile turns its button and keeps the others off (tileWrite)
  const write = (cls: string, key: string) => busyProps(cls, ui.sending === key, off);
  return (
    <article ref={ref} class="tile room" data-id={String(room.id)}>
      <header class="tile-head">
        <button type="button" class="room-name-btn" onClick={() => toggleDevices(room, tile())}><h2 class="room-name">{room.name}</h2></button>
        <span class="room-problems"><Problems entries={entries} label={problemsLabel(entries.flatMap((e) => e.items || []))} /></span>
        <RoomMenu room={room} ui={ui} tile={tile} />
      </header>
      <NotReadyLine room={room} tile={tile} />
      <div class={"setpoint-row" + hid(!th)}>
        <div class="temp-block">
          <span class="temp-tag"><Icon symbol="icon-is" cls="ti" /><span class="temp-label">{t("room.is")}</span></span>
          <span class={"temp-val" + (src?.stale ? " stale" : "")}>
            <span class="temp-big temp-is" aria-label={t("room.measured")} title={t(actual === undefined ? "room.noMeasured" : "room.measured")}>{fmtTemp(actual, "°")}</span>
            <span class="temp-age">{src && <Age at={received(src, "actual_temp")} stale={src.stale} />}</span>
          </span>
        </div>
        <div class="temp-block">
          <span class="temp-tag"><Icon symbol="icon-set" cls="ti" /><span class="temp-label">{t("room.set")}</span></span>
          <span class={"temp-big setpoint-big" + (ui.pending !== null ? " pending" : "")} aria-label={t("room.setTemp")}>
            {th ? fmtTemp(ui.pending !== null ? ui.pending : current, "°") : ""}
          </span>
        </div>
        <div class="stepper needs-setpoint">
          <button type="button" class="step step-plus" aria-label={t("temp.more")} title={t("temp.moreTitle")} disabled={off} onClick={() => stepSetpoint(room, STEP)}>⌃</button>
          <button type="button" class="step step-minus" aria-label={t("temp.less")} title={t("temp.lessTitle")} disabled={off} onClick={() => stepSetpoint(room, -STEP)}>⌄</button>
        </div>
      </div>
      <div class={"pending-row" + hid(ui.pending === null)}>
        <button type="button" {...write("pending-save primary", "setpoint")} onClick={() => flushSetpoint(room)}>{t("action.save")}</button>
        <button type="button" class="pending-cancel" disabled={off} onClick={() => cancelSetpoint(room)}>{t("action.cancel")}</button>
      </div>
      <div class="mode-row">
        <span class={"mode-group needs-setpoint" + hid(!th)} role="group" aria-label={t("room.modeGroup")}>
          <button type="button" {...write("mode-auto" + auto.cls, "auto")} aria-pressed={auto.aria} onClick={() => {
            leaveEditing(room);
            sendRoomSetpoint(room, { mode: "auto" }, t("label.auto"), "auto");
          }}>{t("mode.auto")}</button>
          <button type="button" {...write("mode-manual" + manual.cls, "manual")} aria-pressed={manual.aria} onClick={() => {
            // Manual keeps the room's current setpoint; a pending stepper value
            // is dropped. Without a known setpoint nothing is guessed: the user
            // picks a temperature with the stepper, which sends manual itself.
            leaveEditing(room);
            if (current === null) showTileResult(room.id, t("room.noSetpoint"), false);
            else sendRoomSetpoint(room, { mode: "manual", temp: current }, t("label.manual", { temp: fmtNum(current, 1) }), "manual");
          }}>{t("mode.manu")}</button>
          <More room={room} ui={ui} mode={mode} modes={modes} tile={tile} />
        </span>
        <span class="tile-tools">
          <IconButton symbol="icon-info" cls={"tool-btn tile-info" + (ui.open ? " active" : "")} label={t("room.devices")} title={t("room.devicesTitle")}
            aria-expanded={ui.open ? "true" : "false"} onClick={() => toggleDevices(room, tile())} />
          <IconButton symbol="icon-cog" cls={"tool-btn tile-cog needs-configure" + (cfgOpen ? " active" : "")} label={t("room.configure")}
            title={t("room.configureTitle")} aria-expanded={cfgOpen ? "true" : "false"}
            onClick={() => (cfgOpen ? closeConfig(room) : openConfig(room, tile()))} />
        </span>
      </div>
      <Vacation room={room} ui={ui} />
      <DeleteQuestion room={room} ui={ui} tile={tile} />
      <Island cls="repair-area" keys={[roomRepairScope(room)]} fill={(el) => el.replaceChildren(repairSlot(roomRepairScope(room)))} />
      <ResultLine res={res} cls="tile-result" />
      <div class={"devices" + hid(!ui.open)}>{ui.open && <DeviceList room={room} ui={ui} />}</div>
      <Config room={room} ui={ui} />
    </article>
  );
}
