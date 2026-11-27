// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/copy.tsx
 *
 * Copying as parts (copyable, copyIcon and copyEl of components.js): a
 * text that copies itself on a click, Enter or Space, a copy icon, and
 * something with its copy icon beside it; each named by what it copies, for a screen reader
 * and as its tooltip. The page's copyText and copied do the copying and
 * say that it was done, in the page's bubble (showBubble). A text that
 * copies itself may be quiet: its words hidden from a screen reader, its
 * name alone saying what it copies (a suggested password, AP AccessEnds,
 * D52-A).
 */

import type { ComponentChildren, TargetedKeyboardEvent } from "preact";
import { IconButton } from "./icon";

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
  return <IconButton symbol="icon-copy" label={label} cls="copy-icon copyable" {...copyHandlers(get)} />;
}

/**
 * CopyPair is something shown with its copy icon after it (a span of the
 * class copy-pair), the icon copying what get returns, label naming what:
 * a text that copies itself (Copy), or a link (an address served).
 */
export function CopyPair({ get, label, children }: { get: () => string; label: string; children: ComponentChildren }) {
  return (
    <span class="copy-pair">
      {children}
      <CopyIcon get={get} label={label} />
    </span>
  );
}

/**
 * A Copy's properties: quiet only with a name, so that a text hidden from
 * a screen reader is never named after itself, which would tell it.
 */
type CopyProps = { id?: string; text: string; value?: string; label?: string; icon?: boolean; cls?: string } & (
  | { quiet?: false; name?: string }
  | { quiet: true; name: string }
);

/**
 * Copy is a text that copies itself, a monospaced span by default (cls),
 * and with icon a copy icon after it; value, where given, is what it
 * copies in place of the text shown; label names what it copies, the
 * text itself without one, as "Copy …"; name, where given, is its whole
 * name in place of that (the server's version, which copies more than it
 * shows); id is its element's, where the page names it. quiet hides the
 * text shown from a screen reader, inside the element, so that no live
 * region the text appears in speaks it and only the name is told: given
 * only with a name, which says what it copies without the text (a
 * suggested password, "Copy the password"), since the name made of the
 * text would tell it (CopyProps).
 */
export function Copy({ id, text, value, label, name, icon, quiet, cls }: CopyProps) {
  const named = name || t("copy.what", { what: label || text });
  const get = () => (value === undefined ? text : value);
  const span = (
    <span id={id} class={(cls || "mono") + " copyable"} role="button" tabIndex={0} aria-label={named} title={named} {...copyHandlers(get)}>
      {quiet ? <span aria-hidden="true">{text}</span> : text}
    </span>
  );
  if (!icon) return span;
  return <CopyPair get={get} label={named}>{span}</CopyPair>;
}
