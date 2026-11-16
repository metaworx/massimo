// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/House.tsx
 *
 * The House tile, above the rooms: ECO, AUTO, COMFORT and MANU for the
 * whole house or the rooms ticked, each opening the panel with the mode
 * chosen, its temperature for MANU, permanently or until a date and time,
 * and a room with a thermostat each; Send sets one room after another and
 * says how each went. Built once and updated in place, so that the rooms
 * ticked, the focus and what is typed stay when the rooms load again.
 */

import { useEffect, useLayoutEffect, useRef } from "preact/hooks";
import { ResultLine } from "../shared/badge";
import { DateTime, FormFooter, Stepper } from "../shared/field";
import { useBusy } from "../shared/question";
import { house, load } from "./store";

/** The modes of the House tile, by their value, with their button's id and their text. */
const MODES: [string, string, string][] = [["eco", "house-eco", "mode.eco"], ["auto", "house-auto", "mode.auto"], ["comfort", "house-comfort", "mode.comfort"],
  ["manual", "house-manu", "mode.manu"]];

/** The durations of the House panel: permanently, or until a date and time. */
const DURATIONS = ["permanent", "until"];

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

/** What openHouse asks to have the focus once the panel is drawn: the temperature, or the mode chosen. */
let focusNext: "temp" | "mode" | null = null;

/**
 * openHouse opens the panel with mode chosen: the end a day ahead where
 * none is typed yet, the temperature the first room's setpoint, the
 * result of the last Send gone; the focus on the temperature for MANU,
 * else on the mode.
 */
export function openHouse(mode: string): void {
  house.mode = mode;
  if (!house.until) house.until = tomorrowMorning();
  const first = state.rooms.find(roomThermostat);
  const s = first ? roomState(first) : null;
  if (s && s.setpoint !== undefined) house.temp = String(s.setpoint);
  house.result = null;
  house.open = true;
  focusNext = mode === "manual" ? "temp" : "mode";
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
 * houseRequests applies the panel: one room command after another, each
 * room's outcome in the panel's result line as it comes.
 */
async function houseRequests(): Promise<void> {
  const mode = house.mode;
  const until = house.duration === "until" && mode !== "auto" ? apiUntil(house.until) : "";
  const manualTemp = clampTemp(parseFloat(house.temp) || DEFAULT_TEMP);
  const rooms = state.rooms.filter((r) => roomThermostat(r) && ticked(r.id));
  if (!rooms.length) {
    house.result = { text: t("house.noRoom"), ok: true };
    return;
  }
  house.result = { text: t("house.sending"), ok: true };
  const results: string[] = [];
  let failed = false;
  for (const room of rooms) {
    const th = roomThermostat(room);
    if (!th) continue;
    try {
      let temp: number | undefined = manualTemp;
      if (mode === "eco" || mode === "comfort") {
        temp = (await presetTemps(th))[mode];
        if (temp === undefined) throw new Error(t(mode === "eco" ? "house.noEco" : "house.noComfort"));
      }
      let body: Record<string, unknown> = { mode: "auto" };
      if (mode !== "auto") body = until ? { mode: "vacation", temp, until } : { mode: "manual", temp };
      const res = await api<{ discarded?: boolean } | null>("PUT", base() + "/rooms/" + room.id + "/setpoint", body);
      const what = mode === "auto" ? t("house.auto") : t(until ? "house.tempUntil" : "house.temp", { temp: fmtNum(temp, 1), until });
      results.push(res && res.discarded ? t("house.roomDiscarded", { room: room.name }) : t("house.roomOk", { room: room.name, what }));
      if (res && res.discarded) failed = true;
    } catch (e) {
      results.push(t("house.roomError", { room: room.name, error: (e as Error).message }));
      failed = true;
    }
    house.result = { text: results.join("\n"), ok: true };
  }
  house.result = { text: results.join("\n"), ok: !failed };
  loadCubes().catch(() => {});
}

/**
 * House is the House tile's modes and its panel, drawn into #house-slot;
 * rooms.js shows and hides the tile, as there is a room with a thermostat.
 */
export function House() {
  load.useLoad();
  const sending = useBusy();
  const panel = useRef<HTMLFormElement>(null);
  const rooms = state.rooms.filter((r) => roomThermostat(r));
  const all = rooms.every((r) => ticked(r.id));
  const any = rooms.some((r) => ticked(r.id));
  const res = house.result;
  useLayoutEffect(() => {
    if (!focusNext || !house.open) return;
    const el = panel.current?.querySelector<HTMLElement>(focusNext === "temp" ? "#house-temp" : 'input[name="house-mode"]:checked');
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
  const tick = (id: number, on: boolean) => {
    house.ticks = { ...house.ticks, [id]: on };
  };
  return (
    <>
      <div class="house-modes">
        <span class="muted">{t("house.mode")}</span>
        {MODES.map(([mode, id, key]) => <button key={mode} type="button" id={id} onClick={() => openHouse(mode)}>{t(key)}</button>)}
      </div>
      <form ref={panel} id="house-panel" class={house.open ? "panel" : "panel hidden"} onSubmit={(ev) => {
        ev.preventDefault();
        sending.run(houseRequests);
      }}>
        <div class="panel-row">
          <span class="muted">{t("house.mode")}</span>
          <Segments name="house-mode" options={MODES.map(([mode, , key]) => [mode, t(key)])} value={house.mode} set={(mode) => {
            house.mode = mode;
          }} />
          <Stepper id="house-temp" hidden={house.mode !== "manual"} value={house.temp} set={(v) => {
            house.temp = v;
          }} min={MIN_TEMP} max={MAX_TEMP} step={STEP}>
            <span class="muted">°C</span>
          </Stepper>
        </div>
        <div class={house.mode === "auto" ? "panel-row hidden" : "panel-row"}>
          <span class="muted">{t("house.duration")}</span>
          <Segments name="house-duration" options={DURATIONS.map((d) => [d, t("house." + d)])} value={house.duration} set={(d) => {
            house.duration = d as "permanent" | "until";
          }} />
          <DateTime step={1800} value={house.until} disabled={house.duration !== "until"} set={(v) => {
            house.until = v;
          }} />
        </div>
        <div class="panel-row rooms-row">
          <span class="muted">{t("house.rooms")}</span>
          <div id="house-rooms" class="checks">
            <label class="check">
              <input type="checkbox" id="house-all" checked={all} onChange={(ev) => {
                const on = ev.currentTarget.checked;
                house.ticks = Object.fromEntries(rooms.map((r) => [r.id, on]));
              }} /> {t("house.allRooms")}
            </label>
            {rooms.map((r) => (
              <label key={(state.roomsOf || "") + ":" + r.id} class="check">
                <input type="checkbox" data-room={String(r.id)} checked={ticked(r.id)} onChange={(ev) => tick(r.id, ev.currentTarget.checked)} /> {r.name}
              </label>
            ))}
          </div>
        </div>
        <FormFooter cancel={closeHouse} submit={t("action.send")} busy={sending.busy} off={!any} />
        <ResultLine res={res} cls="tile-result" />
      </form>
    </>
  );
}
