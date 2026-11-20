// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/choices.tsx
 *
 * A choice of several among a list as a part: a group with a control per
 * option, and where asked All, which chooses every option that is on, and
 * None. Drawn as switches under a heading, with All and None as buttons in
 * the heading's row (the configuration's Copy tab), or as checkboxes in a
 * row, All a checkbox of its own before them (the House panel). The caller
 * keeps what is chosen.
 */

import { Switch } from "./field";

/** An option of Choices: its value, its label, and whether it cannot be chosen. */
export type Choice = [value: string, label: string, disabled?: boolean];

/**
 * Choices is the group (named label for a screen reader, id its element's)
 * of a control per option, on where chosen holds its value; set is told of
 * what is chosen after each change; empty is said where there is no
 * option. look "switch" (the default) draws switches under the heading
 * title, all giving the labels of All and None, buttons in the heading's
 * row, each off where it would change nothing; look "check" draws
 * checkboxes (.checks), all.all the label of All, a checkbox first, on
 * while every option is chosen, and no None (all.none left out).
 */
export function Choices({ title, label, id, options, chosen, set, all, empty, look = "switch" }: {
  title?: string;
  label: string;
  id?: string;
  options: Choice[];
  chosen: string[];
  set: (chosen: string[]) => void;
  all?: { all: string; none?: string };
  empty?: string;
  look?: "switch" | "check";
}) {
  const free = options.filter(([, , off]) => !off).map(([v]) => v);
  const every = free.every((v) => chosen.includes(v));
  const toggle = (v: string, on: boolean) => set(on ? [...chosen, v] : chosen.filter((x) => x !== v));
  if (look === "check") {
    return (
      <div id={id} class="checks" role="group" aria-label={label}>
        {all && (
          <label class="check check-all">
            <input type="checkbox" checked={every} onChange={(ev) => set(ev.currentTarget.checked ? free : [])} /> {all.all}
          </label>
        )}
        {options.length === 0 && empty && <p class="muted">{empty}</p>}
        {options.map(([v, l, off]) => (
          <label key={v} class="check">
            <input type="checkbox" value={v} checked={!off && chosen.includes(v)} disabled={off} onChange={(ev) => toggle(v, ev.currentTarget.checked)} /> {l}
          </label>
        ))}
      </div>
    );
  }
  return (
    <div id={id} class="choices" role="group" aria-label={label}>
      <div class="choices-head">
        {title && <h4>{title}</h4>}
        {all && <button type="button" disabled={every} onClick={() => set(free)}>{all.all}</button>}
        {all?.none && <button type="button" disabled={chosen.length === 0} onClick={() => set([])}>{all.none}</button>}
      </div>
      <div class="choices-list">
        {options.length === 0 && empty && <p class="muted">{empty}</p>}
        {options.map(([v, l, off]) => (
          <Switch key={v} row={false} label={l} checked={!off && chosen.includes(v)} disabled={off} onChange={(on) => toggle(v, on)} />
        ))}
      </div>
    </div>
  );
}
