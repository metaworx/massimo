// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/menu.tsx
 *
 * A menu that drops down, as dropdown of components.js makes one of a
 * details.menu: one open at a time, kept in the window, closed by a choice
 * of one of its items, by Escape and by a click outside it; an item of a
 * menu (menuItem); and the choice of one item (pickerMenu), whose summary
 * shows the item chosen. The details is made once and kept, so that an
 * open menu stays open, and its items follow, when data comes in.
 */

import type { ComponentChildren, TargetedMouseEvent } from "preact";
import { useLayoutEffect, useRef } from "preact/hooks";
import { Icon } from "./icon";

/**
 * Dropdown is a details of the classes "menu" and cls (its id id): its
 * summary (summaryCls, its id summaryId, label its name for a screen
 * reader and, without title, its tooltip) shows summary, its list
 * (.menu-items and listCls) the children; onClick sees every click in it.
 * With arrows, a selector of its items, the arrow keys move between them
 * (menuArrows of components.js); with focusCurrent opening the menu puts
 * the focus on the item marked aria-current, else on the first, among
 * arrows' items, or its list's buttons that are on without arrows. A click
 * on any button of its list closes it (dropdown), as every menu of the
 * page.
 */
export function Dropdown({ id, cls, summaryId, summaryCls, label, title, summary, listCls, arrows, focusCurrent, onClick, children }: {
  id?: string;
  cls?: string;
  summaryId?: string;
  summaryCls?: string;
  label: string;
  title?: string;
  summary: ComponentChildren;
  listCls?: string;
  arrows?: string;
  focusCurrent?: boolean;
  onClick?: (ev: TargetedMouseEvent<HTMLDetailsElement>) => void;
  children?: ComponentChildren;
}) {
  const ref = useRef<HTMLDetailsElement>(null);
  // biome-ignore lint/correctness/useExhaustiveDependencies: the listeners are added once to the details, which is made once; arrows and focusCurrent are a menu's for its life, and listed they would add the listeners again.
  useLayoutEffect(() => {
    const details = ref.current;
    if (!details) return;
    dropdown(details);
    if (focusCurrent) {
      const among = arrows || ".menu-items button:not(:disabled)";
      details.addEventListener("toggle", () => {
        if (!details.open) return;
        const items = [...details.querySelectorAll<HTMLElement>(among)];
        const b = items.find((x) => x.getAttribute("aria-current") === "true") || items[0];
        if (b) b.focus();
      });
    }
    if (arrows) menuArrows(details, arrows);
  }, []);
  return (
    <details ref={ref} id={id} class={cls ? "menu " + cls : "menu"} onClick={onClick}>
      <summary id={summaryId} class={summaryCls} title={title || label} aria-label={label}>{summary}</summary>
      <div class={listCls ? "menu-items " + listCls : "menu-items"}>{children}</div>
    </details>
  );
}

/**
 * MenuItem is a button of a menu's list (menuItem of components.js): its
 * icon (an element such as a sign; with blank and no icon an empty place
 * of an icon's width, so that the labels line up), its label with icons
 * after it on its line where given, and under it in grey a second line
 * (sub); value is its data-value, current marks it aria-current.
 */
export function MenuItem({ icon, blank, label, icons, sub, value, current, disabled, onClick }: {
  icon?: ComponentChildren;
  blank?: boolean;
  label: string;
  icons?: ComponentChildren;
  sub?: string;
  value?: string;
  current?: boolean;
  disabled?: boolean;
  onClick?: () => void;
}) {
  const name = icons ? (
    <span class="menu-item-line">
      <span class="menu-item-label">{label}</span>
      <span class="menu-item-icons">{icons}</span>
    </span>
  ) : <span class="menu-item-label">{label}</span>;
  return (
    <button type="button" data-value={value} disabled={disabled} aria-current={current ? "true" : undefined} onClick={onClick}>
      {icon || (blank && <span class="menu-icon-none" />)}
      {sub ? <span class="menu-item-text">{name}<span class="menu-item-sub">{sub}</span></span> : name}
    </button>
  );
}

/**
 * An entry of a Picker's list: an item to choose, by its value, with its
 * icon, label, the icons after it, its grey second line, and whether it
 * cannot be chosen; or a group's heading; or a separator.
 */
export type PickEntry =
  | { value: string; label: string; sub?: string; icon?: ComponentChildren; icons?: ComponentChildren; disabled?: boolean }
  | { group: string }
  | { separator: true };

/**
 * Picker makes a menu the choice of one item (pickerMenu of
 * components.js), where a select could not show the chosen item's name
 * alone while its list shows more: its summary shows shown's icon and
 * label, its tooltip shown.title and its name for a screen reader
 * shown.aria; its list the entries, the item of value marked current. A
 * click on an item calls onPick with its value; the arrow keys move
 * between the items that can be chosen, and opening puts the focus on the
 * chosen one. id, cls, summaryId and summaryCls are the Dropdown's.
 */
export function Picker({ id, cls, summaryId, summaryCls, entries, value, onPick, shown }: {
  id?: string;
  cls?: string;
  summaryId?: string;
  summaryCls?: string;
  entries: PickEntry[];
  value: string;
  onPick: (value: string) => void;
  shown: { icon?: ComponentChildren; label: string; title?: string; aria?: string };
}) {
  const blank = entries.some((e) => "value" in e && !!e.icon);
  return (
    <Dropdown id={id} cls={cls} summaryId={summaryId} summaryCls={summaryCls} label={shown.aria || shown.label} title={shown.title || shown.label}
      arrows=".menu-items button:not(:disabled)" focusCurrent summary={
        <>
          <span class="picker-icon">{shown.icon}</span>
          <span class="picker-label">{shown.label}</span>
          <Icon symbol="icon-chevron-down" cls="ni caret" />
        </>
      }>
      {entries.map((e, i) => {
        if ("group" in e) return <p key={"group:" + e.group} class="menu-group">{e.group}</p>;
        if ("separator" in e) return <hr key={"sep:" + i} class="menu-sep" />;
        return <MenuItem key={e.value} value={e.value} icon={e.icon} blank={blank} label={e.label} icons={e.icons} sub={e.sub}
          current={e.value === value} disabled={e.disabled} onClick={() => onPick(e.value)} />;
      })}
    </Dropdown>
  );
}
