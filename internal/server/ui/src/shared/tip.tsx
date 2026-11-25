// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/tip.tsx
 *
 * A tip: a button with a short label, which shows a text under itself
 * while open, such as the label's full term (a room tile's IS and SET).
 * The button is named by its label and then the text, "IS measured
 * temperature", so that its name holds what it shows (WCAG 2.5.3) and
 * what it opens; the text's box gives its text to the name while
 * hidden too, as a name's parts named by id do. One tip is open at a
 * time, and one with the page's menus: the click that opens a tip
 * closes an open menu, as every click outside a menu does (dropdown of
 * components.js), and a menu that opens, by a click, by the keyboard or
 * by the page's code, closes the tip. Escape closes it and nothing
 * else, and a click outside it closes it too. The button says whether
 * the tip is open (aria-expanded) and names the text as what it
 * controls; the button and Escape leave the focus on it. A tip is no
 * menu: it has no items and takes no focus.
 */

import { signal } from "@preact/signals";
import type { ComponentChildren } from "preact";
import { useLayoutEffect, useRef, useState } from "preact/hooks";
import { hid } from "./cls";

/** The id of the tip that is open, "" while none is. */
const openTip = signal("");

/** Counts the tips made, so that each has an id of its own for the page's life. */
let made = 0;

/** The room a tip's text keeps to the window's edges, as a menu's list does (placeMenu of components.js). */
const EDGE = 16;

/**
 * placeTip keeps a tip's text in the window: where the style sheet's
 * place, under its button from the button's left edge, runs out of the
 * window, the text is moved just inside it.
 */
function placeTip(text: HTMLElement) {
  text.style.left = "";
  const r = text.getBoundingClientRect();
  const x = Math.max(EDGE, Math.min(r.left, document.documentElement.clientWidth - EDGE - r.width));
  if (Math.abs(x - r.left) > 0.5) text.style.left = x - r.left + "px";
}

/**
 * Tip is a button of the classes tip-btn and cls, its content the
 * children (an icon) and then label, its short label, in a span of the
 * class labelCls; named by label and then text (aria-labelledby), and
 * text in a small box under it while the tip is open; a click on the
 * button opens and closes it, as Enter and Space do.
 */
export function Tip({ text, label, labelCls, cls, children }: {
  text: string;
  label: string;
  labelCls?: string;
  cls?: string;
  children?: ComponentChildren;
}) {
  const [id] = useState(() => "tip-" + ++made);
  const open = openTip.value === id;
  const wrap = useRef<HTMLSpanElement>(null);
  const box = useRef<HTMLSpanElement>(null);
  // biome-ignore lint/correctness/useExhaustiveDependencies: text is not read, but listed so that the box is placed again where its text changed (the language); it is placed as it opens, before it is drawn.
  useLayoutEffect(() => {
    if (open && box.current) placeTip(box.current);
  }, [open, text]);
  // The page's listeners that close it, while it is open: added in the
  // drawing that opens it, so that an Escape or a click right after the
  // one that opened it finds them.
  useLayoutEffect(() => {
    if (!open) return;
    const close = () => {
      if (openTip.peek() === id) openTip.value = "";
    };
    const onClick = (ev: MouseEvent) => {
      if (!wrap.current?.contains(ev.target as Node)) close();
    };
    // Escape closes the tip and nothing else: seen on its way down,
    // before any other listener of the page, it goes no further, so that
    // neither the room tiles' open panels (closePanels of rooms.js) nor a
    // card or a field that Escape leaves see it; marked as the menus'
    // Escape is (components.js).
    const onKey = (ev: KeyboardEvent) => {
      if (ev.key !== "Escape") return;
      close();
      ev.preventDefault();
      ev.stopPropagation();
    };
    // A details' toggle does not bubble: it is seen on its way down.
    const onToggle = (ev: Event) => {
      const d = ev.target;
      if (d instanceof HTMLDetailsElement && d.open && d.matches("details.menu")) close();
    };
    const onResize = () => {
      if (box.current) placeTip(box.current);
    };
    document.addEventListener("click", onClick);
    document.addEventListener("keydown", onKey, true);
    document.addEventListener("toggle", onToggle, true);
    addEventListener("resize", onResize);
    return () => {
      document.removeEventListener("click", onClick);
      document.removeEventListener("keydown", onKey, true);
      document.removeEventListener("toggle", onToggle, true);
      removeEventListener("resize", onResize);
    };
  }, [open, id]);
  return (
    <span ref={wrap} class="tip">
      <button type="button" class={cls ? "tip-btn " + cls : "tip-btn"} aria-labelledby={id + "-label " + id} aria-expanded={open ? "true" : "false"}
        aria-controls={id} onClick={() => {
          openTip.value = open ? "" : id;
        }}>{children}<span id={id + "-label"} class={labelCls}>{label}</span></button>
      <span ref={box} id={id} class={"tip-text" + hid(!open)}>{text}</span>
    </span>
  );
}
