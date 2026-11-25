// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/SetValue.tsx
 *
 * SET's value in a room's tile (AP RoomsPage v1.0, Block 6, D4-A, D5-A):
 * the setpoint shown, its eco or comfort mark after it, and a small
 * chevron under its last sign, the °, a button that opens a list under
 * the value. The list has a field on top, which starts from the value
 * shown and takes a temperature typed with a point or a comma, Enter
 * sending it; under it OFF and every half degree from the room's lowest
 * setpoint to its highest, without ON, each with its mark, the value
 * shown marked and scrolled to within the list alone, never the page. A
 * choice, and a value typed within the list's range, is sent at once as
 * Save sends the stepper's value (rooms.js chooseSetpoint), but for the
 * value the room holds already, which is not sent again, a stepped value
 * waiting dropped then; a value typed outside the range is marked on the
 * field and not sent. The list is a popup (shared/popup.ts): one is open
 * at a time, and one with the page's menus and tips; a click outside it,
 * the focus moving out of it and a sign-out close it; Escape closes it
 * and nothing else, the focus back on the chevron. It lies inside the
 * window and under the page's header, upward where the room below is
 * short and it fits above. The arrow keys move in the list, Enter and
 * Space choose. While a write of the tile runs, the chevron and the
 * entries do nothing but keep the focus (aria-disabled), but for the
 * chevron closing an open list. Built once per tile and updated in
 * place; what is typed stays while the list is open, whatever data comes
 * in.
 */

import { useLayoutEffect, useRef, useState } from "preact/hooks";
import { ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { Icon } from "../shared/icon";
import { placePopup, usePopup } from "../shared/popup";
import { fmtSetpoint, type Preset, PresetMark, presetOf, TEMP_OFF } from "../shared/setpoint";

/** The list's lowest and highest temperatures above OFF where the room's entry has none (a server before them): the thermostats' 5 to 30 °C. */
const LOW = 5;
const HIGH = 30;

/**
 * setTemps are the temperatures SET's list offers for a room (D4-A):
 * OFF, then every half degree from the room's lowest setpoint to its
 * highest, its entry's min_setpoint and max_setpoint, kept within 5 and
 * 30 °C, so that OFF comes once and ON (30.5 °C) not at all; OFF alone
 * where the lowest is above the highest, as a thermostat can store them.
 * OFF is offered whatever the room's lowest, as the stepper offers it at
 * its bottom (clampTemp).
 */
export function setTemps(room: PageRoom): number[] {
  const lo = Math.max(LOW, Math.ceil((room.min_setpoint ?? LOW) * 2) / 2);
  const hi = Math.min(HIGH, Math.floor((room.max_setpoint ?? HIGH) * 2) / 2);
  const out = [TEMP_OFF];
  for (let v = lo; v <= hi; v += 0.5) out.push(v);
  return out;
}

/**
 * typedTemp is the setpoint a text typed into SET's field asks for
 * (D5-A): a number with a point or a comma, a ° or °C after it allowed,
 * rounded to the half degree, 4.5 °C being OFF; or OFF itself, in any
 * case; null for anything else.
 */
export function typedTemp(text: string): number | null {
  const s = text.trim().replace(/\s*°\s*c?$/i, "").replace(",", ".");
  if (s.toLowerCase() === t("temp.off").toLowerCase()) return TEMP_OFF;
  if (!/^(\d+(\.\d*)?|\.\d+)$/.test(s)) return null;
  return Math.round(parseFloat(s) * 2) / 2;
}

/** fieldText is a setpoint as SET's field starts from it: OFF, else the number in the page's language ("21,5" in German), "" for none. */
function fieldText(v: number | null): string {
  if (v === null) return "";
  return v === TEMP_OFF ? t("temp.off") : fmtNum(v, 1);
}

/** rangeText says what SET's field takes: OFF and the list's range, or OFF alone. */
function rangeText(temps: number[]): string {
  if (temps.length < 2) return t("room.setOnlyOff", { off: t("temp.off") });
  return t("room.setRange", { off: t("temp.off"), min: fmtNum(temps[1], 1), max: fmtTemp(temps[temps.length - 1]) });
}

/** inView scrolls the list's entries, and nothing else, so that entry is in their view, in their middle where centre is given. */
function inView(items: HTMLElement, entry: HTMLElement, centre?: boolean) {
  const ir = items.getBoundingClientRect();
  const er = entry.getBoundingClientRect();
  if (centre) items.scrollTop += er.top - ir.top - (items.clientHeight - er.height) / 2;
  else if (er.top < ir.top) items.scrollTop -= ir.top - er.top;
  else if (er.bottom > ir.bottom) items.scrollTop += er.bottom - ir.bottom;
}

/**
 * SetValue is SET's value in the tile: shown the setpoint shown (a
 * stepped one too, pending), "" without a thermostat (th), preset its
 * mark; its chevron and list as the file says, doing nothing but
 * keeping the focus while the tile sends (ui.sending), hidden for a user
 * who may not set a temperature.
 */
export function SetValue({ room, ui, th, shown, preset }: { room: PageRoom; ui: RoomUI; th: boolean; shown: number | null; preset: Preset | null }) {
  const btn = useRef<HTMLButtonElement>(null);
  const pop = useRef<HTMLDivElement>(null);
  const items = useRef<HTMLDivElement>(null);
  const field = useRef<HTMLInputElement>(null);
  const [draft, setDraft] = useState("");
  const [bad, setBad] = useState(false);
  // The list keeps under the page's header, pinned at the window's top
  // over the page (style.css .top), as it opens upward.
  const place = () => {
    if (pop.current && items.current) placePopup(pop.current, items.current, document.querySelector("header.top")?.getBoundingClientRect().bottom ?? 0);
  };
  const { id, open, toggle, close } = usePopup("set-list", {
    within: () => [btn.current, pop.current],
    place,
    escaped: (inside) => {
      if (inside) btn.current?.focus({ preventScroll: true });
    },
    leave: true,
  });
  const off = !!ui.sending;
  const temps = setTemps(room);
  const presets = roomPresets(room);
  const text = th ? fmtSetpoint(shown, "°") : "";
  // The list placed as it opens, before it is drawn, and again where the
  // message under the field came or went, which changes its height.
  // biome-ignore lint/correctness/useExhaustiveDependencies: bad is not read, but listed so that the list is placed again where its message came or went; place is made at every drawing and reads its elements alone.
  useLayoutEffect(() => {
    if (open) place();
  }, [open, bad]);
  // As the list opens, in the drawing that shows it, once it is placed:
  // the field from the value shown, the list scrolled to it in itself,
  // the focus on its entry, or on the nearest where the list has not got
  // it (ON, or none: the stepper's start), without scrolling the page.
  // biome-ignore lint/correctness/useExhaustiveDependencies: the field and the list's scroll are set from the value only when the list opens; with shown listed, every state event would write over what the user typed.
  useLayoutEffect(() => {
    if (!open) return;
    setDraft(fieldText(shown));
    setBad(false);
    const l = items.current;
    if (!l) return;
    const entries = [...l.querySelectorAll<HTMLButtonElement>(".set-item")];
    const want = shown ?? DEFAULT_TEMP;
    const at = entries.find((e) => e.getAttribute("aria-current") === "true") ||
      entries.reduce<HTMLButtonElement | undefined>((a, e) => (!a || Math.abs(Number(e.dataset.value) - want) < Math.abs(Number(a.dataset.value) - want) ? e : a), undefined);
    if (!at) return;
    inView(l, at, true);
    at.focus({ preventScroll: true });
  }, [open]);
  // A choice closes the list, the focus back on the chevron, and sends
  // it, but for the value the room holds already (chooseSetpoint); while
  // a write of the tile runs, it does nothing.
  const choose = (v: number) => {
    if (off) return;
    close();
    btn.current?.focus({ preventScroll: true });
    chooseSetpoint(room, v);
  };
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (off) return;
    const v = typedTemp(draft);
    if (v === null || !temps.includes(v)) setBad(true);
    else choose(v);
  };
  // The arrow keys move between the entries, the field above the first;
  // Home and End go to the first and the last; the entries scroll in
  // themselves, never the page.
  const onKeyDown = (ev: KeyboardEvent) => {
    const l = items.current;
    if (!l) return;
    const entries = [...l.querySelectorAll<HTMLButtonElement>(".set-item")];
    if (!entries.length) return;
    const i = entries.indexOf(document.activeElement as HTMLButtonElement);
    let next: HTMLElement | undefined;
    if (document.activeElement === field.current) {
      if (ev.key !== "ArrowDown") return;
      next = entries.find((e) => e.getAttribute("aria-current") === "true") || entries[0];
    } else if (i >= 0) {
      if (ev.key === "ArrowDown") next = entries[Math.min(i + 1, entries.length - 1)];
      else if (ev.key === "ArrowUp") next = i === 0 ? (field.current ?? undefined) : entries[i - 1];
      else if (ev.key === "Home") next = entries[0];
      else if (ev.key === "End") next = entries[entries.length - 1];
      else return;
    } else return;
    ev.preventDefault();
    if (!next) return;
    next.focus({ preventScroll: true });
    if (next !== field.current) inView(l, next);
  };
  const head = text.slice(0, -1);
  const last = text.slice(-1);
  const msgId = id + "-msg";
  return (
    <span class="set-val">
      <span class={"temp-big setpoint-big" + (ui.pending !== null ? " pending" : "")} aria-label={t("room.setTemp")}>
        {head}
        {th && (
          <span class="set-last">
            {last}
            <button ref={btn} type="button" class="set-open needs-setpoint" aria-label={t("room.setChoose")} title={t("room.setChoose")}
              aria-haspopup="dialog" aria-expanded={open ? "true" : "false"} aria-controls={id} aria-disabled={off ? "true" : undefined} onClick={() => {
                if (open || !off) toggle();
              }}><Icon symbol="icon-chevron-down" cls="ni set-caret" /></button>
          </span>
        )}
      </span>
      {preset && <PresetMark preset={preset} cls="ti" />}
      <div ref={pop} id={id} class={"set-pop needs-setpoint" + hid(!open)} role="dialog" aria-label={t("room.setChoose")} onKeyDown={onKeyDown}>
        {open && (
          <>
            <form class="set-form" onSubmit={submit} noValidate>
              <input ref={field} type="text" class="set-field" inputMode="decimal" enterKeyHint="send" autocomplete="off" spellcheck={false}
                aria-label={t("room.setField")} value={draft} aria-invalid={bad ? "true" : undefined} aria-describedby={bad ? msgId : undefined}
                onInput={(ev) => {
                  setDraft(ev.currentTarget.value);
                  setBad(false);
                }} />
              <ResultLine id={msgId} cls="small set-msg" role="status" res={bad ? { text: rangeText(temps), ok: false } : null} />
            </form>
            <div ref={items} class="set-items" role="group" aria-label={t("room.setTemp")}>
              {temps.map((v) => {
                const mark = presetOf(v, presets.eco, presets.comfort);
                return (
                  <button key={String(v)} type="button" class="set-item" data-value={String(v)} tabIndex={-1} aria-disabled={off ? "true" : undefined}
                    aria-current={v === shown ? "true" : undefined} onClick={() => choose(v)}>
                    <span class="set-item-text">{fmtSetpoint(v, "°")}</span>
                    {mark && <PresetMark preset={mark} cls="ti" />}
                  </button>
                );
              })}
            </div>
          </>
        )}
      </div>
    </span>
  );
}
