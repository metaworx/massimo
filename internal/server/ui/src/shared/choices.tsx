// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/choices.tsx
 *
 * A choice of several among a list as a part: a group with a control per
 * option, and where asked All, which chooses every option that is on, and
 * None. Drawn as switches under a heading, with All and None as buttons in
 * the heading's row (the configuration's Copy tab), or under a legend (an
 * integration's maximum), or without a heading (a role's capabilities); or
 * as checkboxes in a row, All a checkbox of its own before them (the House
 * panel); or, where one option alone is chosen, as radio buttons in a row
 * (a suggested password's Words or Characters). An option may be locked (shown chosen or not, as it is, and off:
 * the owner's set, a capability's switch while its request runs), and carry
 * a tooltip and a badge (a capability disabled for every role). The caller
 * keeps what is chosen.
 */

import type { ComponentChildren } from "preact";
import { Switch } from "./field";

/**
 * More of an option of Choices: its tooltip, a badge after its label,
 * and lock, which shows it as chosen or not and keeps it from being
 * changed.
 */
export interface ChoiceMore {
  title?: string;
  badge?: ComponentChildren;
  lock?: boolean;
}

/** An option of Choices: its value, its label, whether it cannot be chosen (shown not chosen, and off), and more. */
export type Choice = [value: string, label: string, disabled?: boolean, more?: ChoiceMore];

/**
 * Choices is the group (named label for a screen reader, id its element's,
 * cls a class it adds) of a control per option, on where chosen holds its
 * value; set is told of what is chosen after each change; empty is said
 * where there is no option. look "switch" (the default) draws switches
 * under the heading title, all giving the labels of All and None, buttons
 * in the heading's row, each off where it would change nothing, or with
 * legend a fieldset under that legend; without a heading, a legend or All
 * it draws no head. look "check" draws checkboxes (.checks), all.all the
 * label of All, a checkbox first, on while every option is chosen, and no
 * None (all.none left out), its options without more. look "one" draws
 * radio buttons in a row (.checks.one), a group of the name id, set told
 * of the one chosen alone, without All, None or more.
 */
export function Choices({ title, legend, label, id, cls, options, chosen, set, all, empty, look = "switch" }: {
  title?: string;
  legend?: string;
  label: string;
  id?: string;
  cls?: string;
  options: Choice[];
  chosen: string[];
  set: (chosen: string[]) => void;
  all?: { all: string; none?: string };
  empty?: string;
  look?: "switch" | "check" | "one";
}) {
  const free = options.filter(([, , off, more]) => !off && !more?.lock).map(([v]) => v);
  const every = free.every((v) => chosen.includes(v));
  const toggle = (v: string, on: boolean) => set(on ? [...chosen, v] : chosen.filter((x) => x !== v));
  if (look === "one") {
    return (
      <div id={id} class={"checks one" + (cls ? " " + cls : "")} role="radiogroup" aria-label={label}>
        {options.map(([v, l, off]) => (
          <label key={v} class="check">
            <input type="radio" name={id} value={v} checked={chosen.includes(v)} disabled={off} onChange={(ev) => { if (ev.currentTarget.checked) set([v]); }} /> {l}
          </label>
        ))}
      </div>
    );
  }
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
  const head = (title || all) && (
    <div class="choices-head">
      {title && <h4>{title}</h4>}
      {all && <button type="button" disabled={every} onClick={() => set(free)}>{all.all}</button>}
      {all?.none && <button type="button" disabled={chosen.length === 0} onClick={() => set([])}>{all.none}</button>}
    </div>
  );
  return (
    <ChoiceGroup id={id} cls={cls} label={label} legend={legend} head={head || undefined}>
      {options.length === 0 && empty && <p class="muted">{empty}</p>}
      {options.map(([v, l, off, more]) => (
        <Switch key={v} row={false} label={l} checked={!off && chosen.includes(v)} disabled={off || more?.lock} title={more?.title}
          badge={more?.badge} onChange={(on) => toggle(v, on)} />
      ))}
    </ChoiceGroup>
  );
}

/**
 * ChoiceGroup is the frame of a choice of several drawn as switches, which
 * Choices is drawn with and a region uses where each switch is a part of
 * its own (Capabilities for every role, each saved at once): the group of
 * the class choices and cls, named label for a screen reader, or a
 * fieldset under legend; its head (a heading, All and None) where given;
 * and its list (.choices-list), the switches as children.
 */
export function ChoiceGroup({ id, cls, label, legend, head, children }: {
  id?: string;
  cls?: string;
  label: string;
  legend?: string;
  head?: ComponentChildren;
  children?: ComponentChildren;
}) {
  const c = "choices" + (cls ? " " + cls : "");
  const list = <div class="choices-list">{children}</div>;
  if (legend) {
    return (
      <fieldset id={id} class={c}>
        <legend>{legend}</legend>
        {head}
        {list}
      </fieldset>
    );
  }
  return (
    <div id={id} class={c} role="group" aria-label={label}>
      {head}
      {list}
    </div>
  );
}
