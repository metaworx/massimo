// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/menu.tsx
 *
 * A menu that drops down, as dropdown of components.js makes one of a
 * details.menu: one open at a time, kept in the window, closed by a choice
 * of one of its items, by Escape and by a click outside it; an item of a
 * menu (MenuItem); and the choice of one item (Picker), whose summary
 * shows the item chosen. The details is made once and kept, so that an
 * open menu stays open, and its items follow, when data comes in.
 */

import { createRef, type ComponentChildren, type Ref, type TargetedMouseEvent } from "preact";
import { useLayoutEffect, useRef } from "preact/hooks";
import { Icon } from "./icon";
import { busyProps } from "./question";

/** mainMenuButton is the header's ☰ button, filled by the page's chrome (its Dropdown's summaryRef), for a part that gives the focus back there (the open server's Hide, ☰'s Create the owner…). */
export const mainMenuButton = createRef<HTMLElement>();

/**
 * Dropdown is a details of the classes "menu" and cls (its id id): its
 * summary (summaryCls, its id summaryId, label its name for a screen
 * reader and, without title, its tooltip; without label it is named by
 * what it shows) shows summary, its list (.menu-items and listCls) the
 * children; onClick sees every click in it, onOpen is told each time
 * it opens (a list that loads what it shows) and onClose each time it
 * closes (a list that keeps a timer while open). busy makes the summary turn
 * while a request sent from its list runs, as busyProps marks a button. With
 * arrows, a selector of its items, the arrow keys move between them
 * (menuArrows of components.js); with focusCurrent opening the menu puts
 * the focus on the item marked aria-current, else on the first, among
 * arrows' items, or its list's buttons that are on without arrows. A click
 * on any button of its list closes it (dropdown), as every menu of the
 * page, but on one marked keepOpen (MenuItem). summaryRef holds its
 * summary, for an item whose result gives the focus back there (☰'s
 * Create the owner…).
 */
export function Dropdown({ id, cls, summaryId, summaryCls, summaryRef, label, title, summary, listCls, arrows, focusCurrent, busy, onClick, onOpen, onClose, children }: {
  id?: string;
  summaryRef?: Ref<HTMLElement>;
  cls?: string;
  summaryId?: string;
  summaryCls?: string;
  label?: string;
  title?: string;
  summary: ComponentChildren;
  listCls?: string;
  arrows?: string;
  focusCurrent?: boolean;
  busy?: boolean;
  onClick?: (ev: TargetedMouseEvent<HTMLDetailsElement>) => void;
  onOpen?: () => void;
  onClose?: () => void;
  children?: ComponentChildren;
}) {
  const ref = useRef<HTMLDetailsElement>(null);
  const opened = useRef(onOpen);
  opened.current = onOpen;
  const closed = useRef(onClose);
  closed.current = onClose;
  // biome-ignore lint/correctness/useExhaustiveDependencies: the listeners are added once to the details, which is made once; arrows and focusCurrent are a menu's for its life, and listed they would add the listeners again.
  useLayoutEffect(() => {
    const details = ref.current;
    if (!details) return;
    dropdown(details);
    details.addEventListener("toggle", () => {
      const told = details.open ? opened.current : closed.current;
      if (told) told();
    });
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
  const sum = summaryCls || "";
  return (
    <details ref={ref} id={id} class={cls ? "menu " + cls : "menu"} onClick={onClick}>
      <summary ref={summaryRef} id={summaryId} class={(busy ? sum + " busy" : sum) || undefined} title={title || label} aria-label={label}
        aria-busy={busy ? "true" : undefined}>{summary}</summary>
      <div class={listCls ? "menu-items " + listCls : "menu-items"}>{children}</div>
    </details>
  );
}

/**
 * MenuItem is a button of a menu's list: its
 * icon (an element such as a sign; with blank and no icon an empty place
 * of an icon's width, so that the labels line up), its label with icons
 * after it on its line where given, and under it in grey a second line
 * wherever sub is given, a text or elements, empty too (one that fills
 * once loaded), of the classes menu-item-sub and subCls; cls is the button's class, value its
 * data-value, current marks it aria-current ("page" for the entry of
 * the page shown, as ☰'s), busy makes it turn while the request it sent
 * runs (busyProps); keepOpen leaves its menu open after a click on it
 * (data-keep-open, which dropdown of components.js reads), for an item
 * whose result shows in the list itself. id, title and aria are the
 * button's id, tooltip and name for a screen reader where the page names
 * them (☰'s entries), labelId the label's id, and after what follows the
 * label on its line, a mark at the item's end; data are its data-
 * attributes (the card a sections menu's item brings into view).
 */
export function MenuItem({ id, data, icon, blank, label, labelId, icons, after, sub, subCls, cls, title, aria, value, current, disabled, busy, keepOpen, onClick }: {
  id?: string;
  data?: Record<string, string>;
  icon?: ComponentChildren;
  blank?: boolean;
  label: string;
  labelId?: string;
  icons?: ComponentChildren;
  after?: ComponentChildren;
  sub?: ComponentChildren;
  subCls?: string;
  cls?: string;
  title?: string;
  aria?: string;
  value?: string;
  current?: boolean | "page";
  disabled?: boolean;
  busy?: boolean;
  keepOpen?: boolean;
  onClick?: () => void;
}) {
  const name = icons ? (
    <span class="menu-item-line">
      <span id={labelId} class="menu-item-label">{label}</span>
      <span class="menu-item-icons">{icons}</span>
    </span>
  ) : <span id={labelId} class="menu-item-label">{label}</span>;
  const b = busyProps(cls || "", !!busy, disabled);
  const attrs: Record<string, string> = {};
  for (const [k, v] of Object.entries(data || {})) attrs["data-" + k] = v;
  return (
    <button type="button" id={id} {...attrs} class={b.class || undefined} title={title || undefined} aria-label={aria || undefined} data-value={value} disabled={b.disabled}
      aria-busy={b["aria-busy"]} aria-current={current === "page" ? "page" : current ? "true" : undefined} data-keep-open={keepOpen ? "" : undefined}
      onClick={onClick}>
      {icon || (blank && <span class="menu-icon-none" />)}
      {sub !== undefined ? <span class="menu-item-text">{name}<span class={subCls ? "menu-item-sub " + subCls : "menu-item-sub"}>{sub}</span></span> : name}
      {after}
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
 * Picker makes a menu the choice of one item, where a select could not show the chosen item's name
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
        return <MenuItem key={e.value} value={e.value} icon={e.icon} blank={blank} label={e.label} icons={e.icons} sub={e.sub || undefined}
          current={e.value === value} disabled={e.disabled} onClick={() => onPick(e.value)} />;
      })}
    </Dropdown>
  );
}
