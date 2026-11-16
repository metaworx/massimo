// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/components.d.ts
 *
 * The globals of the plain script ui/components.js that the parts call:
 * the signs' drawings, the blink of a mark, a menu that drops down with
 * its arrow keys, the clipboard, the result line of an action, a button
 * busy while it sends, the page's toast and a section card brought into
 * view.
 * The builders of elements stay out: a part draws its own.
 */

declare global {
  /** A kind's sign: its icon where all is well, else its base and the mark of a state. */
  interface SignDrawing {
    icon: string;
    base?: string;
    warn?: string;
    bad?: string;
  }
  /** The sign of a kind of SIGNS, or of the person's choice for the window and the radio. */
  function signOf(kind: string): SignDrawing;
  /** The state a badge's kind stands for, where a mark shows it: "warn" or "bad". */
  function badgeState(kind: string): "warn" | "bad" | undefined;
  /** Starts a mark's blinking where the mark of this key was first drawn. */
  function keepBlink(use: Element, key?: string): void;
  /** Makes a details.menu a menu that drops down: one open at a time, kept in the window. */
  function dropdown(details: HTMLDetailsElement): HTMLDetailsElement;
  /** Lets the arrow keys move the focus between the items of an open dropdown that match selector. */
  function menuArrows(details: HTMLDetailsElement, selector: string): void;
  /** Puts text on the clipboard; whether it did. */
  function copyText(text: string): Promise<boolean>;
  /** Says on el and to a screen reader that something was copied. */
  function copied(el: HTMLElement, ok: boolean): void;
  /** Says how an action went in a message element; no text hides it. */
  function showResult(el: HTMLElement, text: string, ok?: boolean): void;
  /** Runs fn, which sends a request, with btn busy; nothing while it is busy. */
  function whileBusy<T>(btn: HTMLElement | null, fn: () => Promise<T>): Promise<T | undefined>;
  /** Shows a message in the page's toast for ms milliseconds, with an action's button where given. */
  function showToast(text: string, opts?: { title?: string; ms?: number; tone?: string; action?: { label: string; run: () => void } | null }): void;
  /** Brings a section card into view, unfolded. */
  function showCard(card: HTMLElement | null, instant?: boolean): void;
}

export {};
