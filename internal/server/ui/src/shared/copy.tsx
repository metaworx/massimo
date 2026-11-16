// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/copy.tsx
 *
 * Copying as parts (copyable, copyIcon and copyEl of components.js): a
 * text that copies itself on a click, Enter or Space, a copy icon, and
 * the two side by side; each named by what it copies, for a screen reader
 * and as its tooltip. The page's copyText and copied do the copying and
 * say that it was done.
 */

import type { TargetedKeyboardEvent } from "preact";
import { Icon } from "./icon";

/** copyHandlers copy what get returns on a click, Enter or Space. */
function copyHandlers(get: () => string) {
  const run = async (ev: Event) => {
    ev.preventDefault();
    ev.stopPropagation();
    const el = ev.currentTarget as HTMLElement;
    const text = get();
    if (text) copied(el, await copyText(text));
  };
  return {
    onClick: run,
    onKeyDown: (ev: TargetedKeyboardEvent<HTMLElement>) => {
      if (ev.key === "Enter" || ev.key === " ") run(ev);
    },
  };
}

/** CopyIcon is a small button with the copy icon that copies what get returns; label names what. */
export function CopyIcon({ get, label }: { get: () => string; label: string }) {
  return (
    <button type="button" class="copy-icon copyable" aria-label={label} title={label} {...copyHandlers(get)}>
      <Icon symbol="icon-copy" cls="ni" />
    </button>
  );
}

/**
 * Copy is a text that copies itself, a monospaced span by default (cls),
 * and with icon a copy icon after it; value, where given, is what it
 * copies in place of the text shown; label names what it copies, the
 * text itself without one.
 */
export function Copy({ text, value, label, icon, cls }: { text: string; value?: string; label?: string; icon?: boolean; cls?: string }) {
  const name = t("copy.what", { what: label || text });
  const get = () => (value === undefined ? text : value);
  const span = (
    <span class={(cls || "mono") + " copyable"} role="button" tabIndex={0} aria-label={name} title={name} {...copyHandlers(get)}>
      {text}
    </span>
  );
  if (!icon) return span;
  return (
    <span class="copy-pair">
      {span}
      <CopyIcon get={get} label={name} />
    </span>
  );
}
