// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/fold.tsx
 *
 * A fold that opens and closes, as detailsPanel of components.js builds a
 * panel: a details whose open state the caller keeps (live state, so that
 * it outlives the fold's being drawn in another place), told of every
 * open and close the user makes. The toggle event a details fires when it
 * is made open, or when the caller's state opens it, is not the user's,
 * and is not passed on.
 */

import type { ComponentChildren, HTMLAttributes } from "preact";

/**
 * Fold is a details of the class cls ("panel cube-panel"; a plain fold
 * without "panel"), open as the caller says, onToggle told of the user's
 * open and close; summary is what its summary shows (FoldHead for a
 * panel's name, serial and badges), summaryProps its summary's attributes
 * (a tabIndex, a click handler), the rest the fold's body; data are its
 * data- attributes.
 */
export function Fold({ cls, open, onToggle, summary, summaryProps, data, children }: {
  cls: string;
  open: boolean;
  onToggle?: (open: boolean) => void;
  summary: ComponentChildren;
  summaryProps?: HTMLAttributes<HTMLElement>;
  data?: Record<string, string>;
  children?: ComponentChildren;
}) {
  const attrs: Record<string, string> = {};
  for (const [k, v] of Object.entries(data || {})) attrs["data-" + k] = v;
  return (
    <details class={cls} open={open} {...attrs} onToggle={(ev) => {
      const now = ev.currentTarget.open;
      if (now !== open && onToggle) onToggle(now);
    }}>
      <summary {...summaryProps}>{summary}</summary>
      {children}
    </details>
  );
}

/**
 * FoldHead is a panel's summary as detailsPanel draws it: before (a sign,
 * a count), the name, a monospaced part (a serial) and the badges, each
 * left out where not given.
 */
export function FoldHead({ before, name, mono, badges }: { before?: ComponentChildren; name?: string; mono?: string; badges?: ComponentChildren[] }) {
  const shown = (badges || []).filter(Boolean);
  return (
    <>
      {before}
      {name && <span class="panel-name">{name}</span>}
      {mono && <span class="mono">{mono}</span>}
      {shown.length > 0 && <span class="panel-badges">{shown}</span>}
    </>
  );
}
