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
 * hidden too, as a name's parts named by id do. A tip is a popup
 * (popup.ts): one tip or other popup is open at a time, and one with the
 * page's menus: the click that opens a tip closes an open menu, as every
 * click outside a menu does (dropdown of components.js), and a menu that
 * opens, by a click, by the keyboard or by the page's code, closes the
 * tip. Escape closes it and nothing else, and a click outside it closes
 * it too, as a sign-out does. The button says whether the tip is open
 * (aria-expanded) and names the text as what it controls; the button
 * and Escape leave the focus on it. A tip is no menu: it has no items
 * and takes no focus.
 */

import type { ComponentChildren } from "preact";
import { useLayoutEffect, useRef } from "preact/hooks";
import { hid } from "./cls";
import { placePopup, usePopup } from "./popup";

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
  const wrap = useRef<HTMLSpanElement>(null);
  const box = useRef<HTMLSpanElement>(null);
  const place = () => {
    if (box.current) placePopup(box.current);
  };
  const { id, open, toggle } = usePopup("tip", { within: () => [wrap.current], place });
  // biome-ignore lint/correctness/useExhaustiveDependencies: text is not read, but listed so that the box is placed again where its text changed (the language); it is placed as it opens, before it is drawn.
  useLayoutEffect(() => {
    if (open && box.current) placePopup(box.current);
  }, [open, text]);
  return (
    <span ref={wrap} class="tip">
      <button type="button" class={cls ? "tip-btn " + cls : "tip-btn"} aria-labelledby={id + "-label " + id} aria-expanded={open ? "true" : "false"}
        aria-controls={id} onClick={toggle}>{children}<span id={id + "-label"} class={labelCls}>{label}</span></button>
      <span ref={box} id={id} class={"tip-text" + hid(!open)}>{text}</span>
    </span>
  );
}
