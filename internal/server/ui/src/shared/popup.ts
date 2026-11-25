// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/popup.ts
 *
 * A popup: a box that its button opens beside itself, such as a tip's
 * text (tip.tsx) or a room tile's list of temperatures. One popup is
 * open at a time, and one with the page's menus: the click that opens
 * one closes an open menu or popup, as every click outside them does
 * (dropdown of components.js), and a menu that opens, by a click, by the
 * keyboard or by the page's code, closes it. A click outside it closes
 * it, and, for a popup that takes the focus, the focus moving to an
 * element outside it (Tab out of it). Escape closes it and nothing else:
 * seen on its way down, before any other listener of the page, it goes
 * no further. A sign-out closes it (closePopups). placePopup keeps its
 * box in the window, and, where asked, under what is pinned at the
 * window's top (a page's header), upward where it fits only above, else
 * downward with its height capped to the room below, not under LEAST.
 */

import { signal } from "@preact/signals";
import { useLayoutEffect, useRef, useState } from "preact/hooks";

/** The id of the popup that is open, "" while none is. */
const openPopup = signal("");

/** Counts the popups made, so that each has an id of its own for the page's life. */
let made = 0;

/** The room a popup keeps to the window's edges, as a menu's list does (placeMenu of components.js). */
const EDGE = 16;

/** The gap between a popup opened upward and what it opens from, as a tip's under its button (style.css .tip-text). */
const GAP = 4;

/** The least height of a popup's scrolling part capped to the room below it, as a menu's list (placeMenu of components.js), where the window has room for it. */
const LEAST = 160;

/** closePopups closes the popup that is open: at a sign-out (MassimoParts.signedOut), so that nothing of it stays for the next user. */
export function closePopups(): void {
  openPopup.value = "";
}

/** What a popup is told and asked by usePopup. */
export interface PopupOptions {
  /** The elements a click, or with leave the focus, may go into without closing it: its button and its box. */
  within: () => (Element | null | undefined)[];
  /** Places its box again at a resize of the window; where it opens, the caller's own layout effect places it. */
  place?: () => void;
  /** Told after Escape closed it, inside whether the focus was within it, for a popup that gives the focus back to its button. */
  escaped?: (inside: boolean) => void;
  /** Closes it where the focus moves to an element outside it, for a popup that takes the focus. */
  leave?: boolean;
}

/**
 * usePopup is a popup's state, its id prefix and a number, whether it
 * is open, and toggle and close for its button and its actions; while it
 * is open, the page's listeners that close it, added in the drawing that
 * opens it, so that an Escape or a click right after the one that opened
 * it finds them. opts are read as they are at each event.
 */
export function usePopup(prefix: string, opts: PopupOptions): { id: string; open: boolean; toggle: () => void; close: () => void } {
  const [id] = useState(() => prefix + "-" + ++made);
  const open = openPopup.value === id;
  const now = useRef(opts);
  now.current = opts;
  const close = () => {
    if (openPopup.peek() === id) openPopup.value = "";
  };
  useLayoutEffect(() => {
    if (!open) return;
    const shut = () => {
      if (openPopup.peek() === id) openPopup.value = "";
    };
    const inside = (n: EventTarget | null) => n instanceof Node && now.current.within().some((e) => !!e && e.contains(n));
    const onClick = (ev: MouseEvent) => {
      if (!inside(ev.target)) shut();
    };
    // Marked as the menus' Escape is (components.js); neither the room
    // tiles' open panels (closePanels of rooms.js) nor a card or a field
    // that Escape leaves see it.
    const onKey = (ev: KeyboardEvent) => {
      if (ev.key !== "Escape") return;
      const had = inside(document.activeElement);
      shut();
      now.current.escaped?.(had);
      ev.preventDefault();
      ev.stopPropagation();
    };
    // A details' toggle does not bubble: it is seen on its way down.
    const onToggle = (ev: Event) => {
      const d = ev.target;
      if (d instanceof HTMLDetailsElement && d.open && d.matches("details.menu")) shut();
    };
    const onFocus = (ev: FocusEvent) => {
      if (now.current.leave && !inside(ev.target)) shut();
    };
    const onResize = () => now.current.place?.();
    document.addEventListener("click", onClick);
    document.addEventListener("keydown", onKey, true);
    document.addEventListener("toggle", onToggle, true);
    document.addEventListener("focusin", onFocus, true);
    addEventListener("resize", onResize);
    return () => {
      document.removeEventListener("click", onClick);
      document.removeEventListener("keydown", onKey, true);
      document.removeEventListener("toggle", onToggle, true);
      document.removeEventListener("focusin", onFocus, true);
      removeEventListener("resize", onResize);
    };
  }, [open, id]);
  return {
    id,
    open,
    toggle: () => {
      openPopup.value = open ? "" : id;
    },
    close,
  };
}

/**
 * placePopup keeps an open popup's box in the window, EDGE from its
 * edges: where the style sheet's place, under what it opens from, runs
 * out of the window at a side, the box is moved just inside it. With
 * scroll, the element in it that scrolls (a list), the box is kept
 * inside the window's height too, its top EDGE under pinned, the bottom
 * of what is pinned at the window's top and lies over the page (a sticky
 * header; 0 for nothing): where the room below is less than it needs and
 * the box fits above, it opens upward, GAP above the element it is
 * placed in; else it stays below, scroll's height capped to the room
 * there, but not under LEAST, where the window has room for that, so
 * that a list in a short room still shows a few entries. scroll keeps
 * its place in itself.
 */
export function placePopup(box: HTMLElement, scroll?: HTMLElement, pinned = 0): void {
  const kept = scroll ? scroll.scrollTop : 0;
  Object.assign(box.style, { left: "", top: "", bottom: "" });
  if (scroll) scroll.style.maxHeight = "";
  const r = box.getBoundingClientRect();
  const x = Math.max(EDGE, Math.min(r.left, document.documentElement.clientWidth - EDGE - r.width));
  if (Math.abs(x - r.left) > 0.5) box.style.left = x - r.left + "px";
  const holder = box.offsetParent;
  if (!scroll || !holder) return;
  const top = Math.max(0, pinned) + EDGE;
  const below = window.innerHeight - EDGE - r.top;
  if (r.height > below) {
    if (r.height <= holder.getBoundingClientRect().top - GAP - top) {
      Object.assign(box.style, { top: "auto", bottom: `calc(100% + ${GAP}px)` });
    } else {
      const part = scroll.getBoundingClientRect().height;
      const rest = r.height - part;
      const least = window.innerHeight - EDGE - top >= rest + LEAST ? LEAST : 0;
      scroll.style.maxHeight = Math.max(least, part - (r.height - below), 0) + "px";
    }
  }
  scroll.scrollTop = kept;
}
