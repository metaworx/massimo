// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/tabs.tsx
 *
 * A row of tabs as a part: the tabs in a tab list, each a button with its
 * key in data-tab, the one shown active and selected, and what follows
 * the list in the row (a close button). The caller keeps which tab is
 * shown and draws its panel.
 */

import type { ComponentChildren } from "preact";

/** A tab: its key, its label, and classes it adds to tab (a capability's needs-…). */
export interface Tab {
  key: string;
  label: string;
  cls?: string;
}

/**
 * Tabs is the row (its class cls) with the tab list (listCls, named label
 * for a screen reader): tabs, current the key of the one shown, onPick
 * told of a click on one; children follow the list in the row.
 */
export function Tabs({ cls, listCls, label, tabs, current, onPick, children }: {
  cls: string;
  listCls: string;
  label: string;
  tabs: Tab[];
  current: string;
  onPick: (key: string) => void;
  children?: ComponentChildren;
}) {
  return (
    <div class={cls}>
      <span class={listCls} role="tablist" aria-label={label}>
        {tabs.map((tb) => (
          <button key={tb.key} type="button" class={"tab" + (tb.cls ? " " + tb.cls : "") + (tb.key === current ? " active" : "")} role="tab"
            data-tab={tb.key} aria-selected={tb.key === current ? "true" : "false"} onClick={() => onPick(tb.key)}>
            {tb.label}
          </button>
        ))}
      </span>
      {children}
    </div>
  );
}
