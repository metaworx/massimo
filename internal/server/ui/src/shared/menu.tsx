// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/menu.tsx
 *
 * A menu that drops down, as dropdown of components.js makes one of a
 * details.menu: one open at a time, kept in the window, closed by a choice
 * of one of its items, by Escape and by a click outside it. The details is
 * made once and kept, so that an open menu stays open when data comes in.
 */

import type { ComponentChildren, TargetedMouseEvent } from "preact";
import { useLayoutEffect, useRef } from "preact/hooks";

/**
 * Dropdown is a details of the classes "menu" and cls: its summary
 * (summaryCls, label its tooltip and its name for a screen reader) shows
 * summary, its list (.menu-items and listCls) the children; onClick sees
 * every click in it.
 */
export function Dropdown({ cls, summaryCls, label, summary, listCls, onClick, children }: {
  cls?: string;
  summaryCls?: string;
  label: string;
  summary: ComponentChildren;
  listCls?: string;
  onClick?: (ev: TargetedMouseEvent<HTMLDetailsElement>) => void;
  children?: ComponentChildren;
}) {
  const ref = useRef<HTMLDetailsElement>(null);
  useLayoutEffect(() => {
    if (ref.current) dropdown(ref.current);
  }, []);
  return (
    <details ref={ref} class={cls ? "menu " + cls : "menu"} onClick={onClick}>
      <summary class={summaryCls} title={label} aria-label={label}>{summary}</summary>
      <div class={listCls ? "menu-items " + listCls : "menu-items"}>{children}</div>
    </details>
  );
}
