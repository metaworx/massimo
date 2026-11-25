// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/House.tsx
 *
 * The House tile, above the rooms, named after the chosen Cube, the cube
 * before its name, "All rooms of …" its tooltip: ECO, AUTO, COMFORT, MANU
 * and Holiday, its case before it, for the whole house or the rooms
 * ticked, each opening the one panel with its choice (AP RoomsPage v1.0,
 * Block 7, D7-A), which sets what the panel offers and its defaults: a
 * temperature for MANU, the first room's setpoint, and for Holiday, OFF;
 * permanently or until a date and time for all but AUTO, Holiday until a
 * date alone, a week ahead at 06:00 (D6-A), the date shown only with
 * until (D8-A), a field hidden no part of the form; and a room with a
 * thermostat each. Send refuses an end that has passed, and else sets one
 * room after another and says how each went. Under the modes, the house's
 * lowest and highest measured and set temperature, from the rooms'
 * virtual wall thermostats, a stale measured one left out while any is
 * fresh, without a room's name (D9-A, D10-A). Built
 * once and updated in place, so that the rooms ticked, the focus and what
 * is typed stay when the rooms load again.
 */

import { useEffect, useLayoutEffect, useRef } from "preact/hooks";
import { CubeIcon } from "../model/cube";
import { ResultLine } from "../shared/badge";
import { Choices } from "../shared/choices";
import { DateTime, FormFooter, Stepper } from "../shared/field";
import { hid } from "../shared/cls";
import { Icon } from "../shared/icon";
import { useBusy } from "../shared/question";
import { session, stillSession } from "../shared/session";
import { fmtSetpoint, TEMP_OFF } from "../shared/setpoint";
import { Tip } from "../shared/tip";
import { untracked } from "@preact/signals";
import { useLook } from "../shared/look";
import { house } from "./store";
import { apiUntil } from "../model/room";

/** The modes of the House tile, by their value, with their button's id and their text; Holiday the fifth (D7-A). */
const MODES: [string, string, string][] = [["eco", "house-eco", "mode.eco"], ["auto", "house-auto", "mode.auto"], ["comfort", "house-comfort", "mode.comfort"],
  ["manual", "house-manu", "mode.manu"], ["vacation", "house-holiday", "house.holiday"]];

/** A duration of the House panel: permanently, or until a date and time. */
type Duration = "permanent" | "until";

/** withTemp says whether a mode takes the panel's temperature: MANU and Holiday; ECO and COMFORT take each room's own. */
function withTemp(mode: string): boolean {
  return mode === "manual" || mode === "vacation";
}

/** durationsOf are the durations the panel offers for a mode: Holiday's until alone, else both (AUTO's row hidden). */
function durationsOf(mode: string): Duration[] {
  return mode === "vacation" ? ["until"] : ["permanent", "until"];
}

/** durationOf is the duration in effect for a mode: until for Holiday, else the one chosen, which Holiday leaves as it was. */
function durationOf(mode: string): Duration {
  return mode === "vacation" ? "until" : house.duration;
}

/** weekAhead is Holiday's end by default: a week from today at 06:00 (D6-A). */
function weekAhead(): string {
  const d = new Date();
  d.setDate(d.getDate() + 7);
  d.setHours(6, 0, 0, 0);
  return localDateTimeValue(d);
}

/** passed says whether an end, as the date and time field holds it, is not after now; an end that cannot be read counts as passed. */
function passed(until: string): boolean {
  return !(Date.parse(until) > Date.now());
}

/**
 * choose makes mode the panel's choice with its defaults (D7-A), each
 * where the user has not changed the value: the temperature, MANU's the
 * first room's setpoint (DEFAULT_TEMP where it has none) and Holiday's
 * OFF; the end, Holiday's a week ahead at 06:00, else tomorrow morning.
 * An end refused as passed is no longer marked once another choice is
 * made.
 */
function choose(mode: string): void {
  house.mode = mode;
  if (!house.tempTyped) {
    if (mode === "vacation") house.temp = String(TEMP_OFF);
    else if (mode === "manual") {
      const first = state.rooms.find(roomThermostat);
      const s = first ? roomState(first) : null;
      house.temp = String(s && s.setpoint !== undefined ? s.setpoint : DEFAULT_TEMP);
    }
  }
  if (!house.untilTyped) house.until = mode === "vacation" ? weekAhead() : tomorrowMorning();
  house.untilRefused = false;
}

/**
 * Segments is a choice of one among a few, as radio buttons side by side
 * (.seg), each option its value and its label; name groups the buttons.
 */
function Segments({ name, options, value, set }: { name: string; options: [string, string][]; value: string; set: (v: string) => void }) {
  return (
    <span class="seg">
      {options.map(([v, label]) => (
        <label key={v}>
          <input type="radio" name={name} value={v} checked={value === v} onChange={() => set(v)} /> <span>{label}</span>
        </label>
      ))}
    </span>
  );
}

/** What openHouse asks to have the focus once the panel is drawn: the temperature, the end, or the mode chosen. */
let focusNext: "temp" | "until" | "mode" | null = null;

/**
 * openHouse opens the panel with mode chosen and its defaults (choose):
 * the temperature the choice's each time, the end the choice's where none
 * is typed yet or the one typed has passed, the result of the last Send
 * gone; the focus on the temperature for MANU, on the end for Holiday, as
 * the tile's holiday form has it, else on the mode.
 */
export function openHouse(mode: string): void {
  house.tempTyped = false;
  if (house.untilTyped && passed(house.until)) house.untilTyped = false;
  choose(mode);
  house.result = null;
  house.open = true;
  focusNext = mode === "manual" ? "temp" : mode === "vacation" ? "until" : "mode";
}

/** closeHouse closes the panel; what it holds stays for the next time it opens. */
export function closeHouse(): void {
  house.open = false;
}

/** ticked says whether a room is ticked: every room is, until it is unticked. */
function ticked(id: number): boolean {
  return house.ticks[id] !== false;
}

/**
 * houseRequests applies the panel: one room command after another, a
 * choice with an end sent as a holiday until it (Holiday always), each
 * room's outcome in the panel's result line as it comes, its setpoint as
 * the devices show it (OFF for 4.5 °C), and each room's tile waiting for
 * its thermostats to show it, as after the tile's own write.
 */
async function houseRequests(): Promise<void> {
  const mode = house.mode;
  const until = durationOf(mode) === "until" && mode !== "auto" ? apiUntil(house.until) : "";
  if (mode === "vacation" && !until) return; // a holiday has an end: the field asks for it (required)
  const typedTemp = clampTemp(parseFloat(house.temp) || DEFAULT_TEMP);
  const rooms = state.rooms.filter((r) => roomThermostat(r) && ticked(r.id));
  if (!rooms.length) {
    house.result = { text: t("house.noRoom"), ok: true };
    return;
  }
  house.result = { text: t("house.sending"), ok: true };
  const n = session();
  const results: string[] = [];
  let failed = false;
  for (const room of rooms) {
    if (!stillSession(n)) return; // signed out meanwhile: the next rooms are not written, nothing is said
    if (!roomThermostat(room)) continue;
    try {
      let temp: number | undefined = typedTemp;
      if (mode === "eco" || mode === "comfort") {
        temp = (await presetTemps(room))[mode]; // the room's (RoomMajority.D19-A)
        if (!stillSession(n)) return; // a read answered after a sign-out, sent again for whoever signed in: no setpoint after it
        if (temp === undefined) throw new Error(t(mode === "eco" ? "house.noEco" : "house.noComfort"));
      }
      let body: Record<string, unknown> = { mode: "auto" };
      if (mode !== "auto") body = until ? { mode: "vacation", temp, until } : { mode: "manual", temp };
      const res = await api<{ discarded?: boolean } | null>("PUT", base() + "/rooms/" + room.id + "/setpoint", body);
      if (!stillSession(n)) return;
      awaitMembers(room, body, res); // the room's tile waits for its thermostats, as after its own write
      const what = mode === "auto" ? t("house.auto") : until ? t("house.tempUntil", { temp: fmtSetpoint(temp), until }) : fmtSetpoint(temp);
      results.push(res && res.discarded ? t("house.roomDiscarded", { room: room.name }) : t("house.roomOk", { room: room.name, what }));
      if (res && res.discarded) failed = true;
    } catch (e) {
      results.push(t("house.roomError", { room: room.name, error: (e as Error).message }));
      failed = true;
    }
    if (!stillSession(n)) return;
    house.result = { text: results.join("\n"), ok: true };
  }
  house.result = { text: results.join("\n"), ok: !failed };
  loadCubes().catch(() => {});
}

/** lowHigh is the lowest and the highest of the values, those undefined passed over; null where none is left. */
function lowHigh(values: (number | undefined)[]): [number, number] | null {
  const known = values.filter((v): v is number => typeof v === "number");
  return known.length ? [Math.min(...known), Math.max(...known)] : null;
}

/**
 * RangeRow is a row of the House tile's lowest and highest: the icon and
 * the short label, a tip with the full term (shared/tip.tsx), then the
 * lowest, a dash and the highest, each as fmt shows it, greyed where
 * stale, as a tile greys a stale measured temperature; "–" alone where
 * no room has a value. A value is a text and does nothing on a tap
 * (D9-A): the rows name no room.
 */
function RangeRow({ cls, symbol, label, text, values, fmt, stale }: {
  cls: string;
  symbol: string;
  label: string;
  text: string;
  values: [number, number] | null;
  fmt: (v: number) => string;
  stale?: boolean;
}) {
  const val = "range-val" + (stale ? " stale" : "");
  return (
    <div class={"range-row " + cls}>
      <Tip cls="range-tag" label={label} labelCls="temp-label" text={text}><Icon symbol={symbol} cls="ti" /></Tip>
      <span class={val + " range-low"}>{values ? fmt(values[0]) : "–"}</span>
      <span class="range-dash">{values ? "–" : ""}</span>
      <span class={val + " range-high"}>{values ? fmt(values[1]) : ""}</span>
    </div>
  );
}

/**
 * Range is the House tile's lowest and highest under its modes (D9-A,
 * D10-A): of the measured temperature and of the setpoint, OFF where it
 * is, over the rooms' virtual wall thermostats (room.vwt), the rooms
 * without a value passed over. A stale measured temperature is left out
 * while any room has a fresh one, as the server leaves a stale thermostat
 * out within a room (internal/view/vwt.go); with only stale ones, they
 * are shown, greyed. It reads each room's virtual thermostat as the
 * signal it is (shared/data.ts), so that a state event of one draws these
 * rows again, in place, and not the panel.
 */
function Range() {
  useLook();
  const vwts = state.rooms.map((r) => r.vwt);
  const measured = vwts.filter((v) => v?.state?.actual_temp !== undefined);
  const fresh = measured.filter((v) => !v?.stale);
  const stale = fresh.length === 0 && measured.length > 0;
  return (
    <div class="house-range">
      <RangeRow cls="range-is" symbol="icon-is" label={t("room.is")} text={t("room.measured")}
        values={lowHigh((stale ? measured : fresh).map((v) => v?.state?.actual_temp))} fmt={(v) => fmtTemp(v, "°")} stale={stale} />
      <RangeRow cls="range-set" symbol="icon-set" label={t("room.set")} text={t("room.setTemp")} values={lowHigh(vwts.map((v) => v?.state?.setpoint))}
        fmt={(v) => fmtSetpoint(v, "°")} />
    </div>
  );
}

/**
 * House is the House tile's modes, its lowest and highest, and its panel,
 * drawn into #house-slot; rooms.js shows and hides the tile, as there is
 * a room with a thermostat.
 */
export function House() {
  useLook();
  const sending = useBusy();
  const panel = useRef<HTMLFormElement>(null);
  const untilRef = useRef<HTMLInputElement>(null);
  // the rooms by the list, which it follows; their devices' states draw nothing here, as before
  const list = state.rooms;
  const rooms = untracked(() => list.filter((r) => roomThermostat(r)));
  const any = rooms.some((r) => ticked(r.id));
  const res = house.result;
  const mode = house.mode;
  const dated = mode !== "auto" && durationOf(mode) === "until";
  useLayoutEffect(() => {
    if (!focusNext || !house.open) return;
    const el = panel.current?.querySelector<HTMLElement>(focusNext === "temp" ? "#house-temp" : focusNext === "until" ? "#house-until" : 'input[name="house-mode"]:checked');
    focusNext = null;
    el?.focus();
  });
  useEffect(() => {
    // A click outside closes the panel once it has sent (and an open
    // menu, dropdown of components.js).
    const outside = (ev: MouseEvent) => {
      if (house.open && house.result && !(ev.target as Element).closest("#house")) closeHouse();
    };
    document.addEventListener("click", outside);
    return () => document.removeEventListener("click", outside);
  }, []);
  // the Cube whose rooms are shown, its name as the Cubes' list last gave it
  const cube = state.cubes.find((c) => c.serial === (state.roomsOf || state.serial));
  const name = cube ? cube.name || cube.serial : t("house.title");
  const whole = cube ? t("house.roomsOf", { cube: name }) : name;
  return (
    <>
      <header class="tile-head">
        <h2 class="house-title" title={whole} aria-label={whole}>
          <CubeIcon />
          <span class="house-name">{name}</span>
        </h2>
      </header>
      <div class="house-modes">
        <span class="muted">{t("house.mode")}</span>
        {MODES.map(([m, id, key]) => (
          <button key={m} type="button" id={id} class={m === "vacation" ? "house-holiday" : undefined} onClick={() => openHouse(m)}>
            {m === "vacation" && <Icon symbol="icon-suitcase" cls="ni" />}
            {t(key)}
          </button>
        ))}
      </div>
      <Range />
      <form ref={panel} id="house-panel" class={house.open ? "panel" : "panel hidden"} onSubmit={(ev) => {
        ev.preventDefault();
        if (dated && passed(house.until)) {
          // An end that has passed is sent to no room: the field says why, with the focus.
          house.untilRefused = true;
          untilRef.current?.focus();
          return;
        }
        sending.run(houseRequests);
      }}>
        <div class="panel-row">
          <span class="muted">{t("house.mode")}</span>
          <Segments name="house-mode" options={MODES.map(([m, , key]) => [m, t(key)])} value={mode} set={choose} />
          <Stepper id="house-temp" hidden={!withTemp(mode)} disabled={!withTemp(mode)} value={house.temp} set={(v) => {
            house.temp = v;
            house.tempTyped = true;
          }} min={MIN_TEMP} max={MAX_TEMP} step={STEP}>
            <span class="muted">{Number(house.temp) === TEMP_OFF ? t("house.unitOff", { off: t("temp.off") }) : "°C"}</span>
          </Stepper>
        </div>
        <div class={"panel-row" + hid(mode === "auto")}>
          <span class="muted">{t("house.duration")}</span>
          <Segments name="house-duration" options={durationsOf(mode).map((d) => [d, t("house." + d)])} value={durationOf(mode)} set={(d) => {
            house.duration = d as Duration;
          }} />
          <DateTime id="house-until" inputRef={untilRef} cls={"house-until" + hid(!dated)} step={1800} required={dated} disabled={!dated}
            limit={{ text: t("house.untilPast"), over: dated && house.untilRefused }} value={house.until} set={(v) => {
              house.until = v;
              house.untilTyped = true;
              house.untilRefused = false;
            }} />
        </div>
        <div class="panel-row rooms-row">
          <span class="muted">{t("house.rooms")}</span>
          <Choices look="check" id="house-rooms" label={t("house.rooms")} all={{ all: t("house.allRooms") }}
            options={rooms.map((r) => [String(r.id), r.name])} chosen={rooms.filter((r) => ticked(r.id)).map((r) => String(r.id))}
            set={(chosen) => {
              house.ticks = Object.fromEntries(rooms.map((r) => [r.id, chosen.includes(String(r.id))]));
            }} />
        </div>
        <FormFooter cancel={closeHouse} submit={t("action.send")} busy={sending.busy} off={!any} />
        <ResultLine res={res} cls="tile-result" />
      </form>
    </>
  );
}
