// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/components.d.ts
 *
 * The globals of the plain script ui/components.js that the parts call:
 * the signs' drawings, the blink of a mark, a menu that drops down with
 * its arrow keys, the clipboard, the bubble of a short text, the result
 * line of an action, a button busy while it sends and the page's toast.
 * The builders of elements stay out: a part draws its own.
 */

declare global {
  /** How long a result that went well stays shown, in ms. */
  const RESULT_FADE_MS: number;
  /** A kind's sign: its icon where all is well, else its base and the mark of a state. */
  interface SignDrawing {
    icon: string;
    base?: string;
    warn?: string;
    bad?: string;
  }
  /** The drawing of the window or the radio the person chose: the window's closed type icon, and the sign. */
  function iconChoice(kind: "window" | "radio"): { type?: string; sign: SignDrawing };
  /** The sign of a kind of SIGNS, or of the person's choice for the window and the radio. */
  function signOf(kind: string): SignDrawing;
  /** The state a badge's kind stands for, where a mark shows it: "warn" or "bad". */
  function badgeState(kind: string): "warn" | "bad" | undefined;
  /** Starts a mark's blinking where the mark of this key was first drawn. */
  function keepBlink(use: Element, key?: string): void;
  /** Makes a details.menu a menu that drops down: one open at a time, kept in the window. */
  function dropdown(details: HTMLDetailsElement): HTMLDetailsElement;
  /** Places the list of every open menu under root (the whole page by default) in the window again, where its summary has moved. */
  function placeMenus(root?: ParentNode): void;
  /** Lets the arrow keys move the focus between the items of an open dropdown that match selector. */
  function menuArrows(details: HTMLDetailsElement, selector: string): void;
  /** Puts text on the clipboard; whether it did. */
  function copyText(text: string): Promise<boolean>;
  /** Says in a bubble above el, or with el null at the window's foot, and to a screen reader, that something was copied, or that it could not be. */
  function copied(el: HTMLElement | null, ok: boolean): void;
  /**
   * Shows text for a moment (ms, 1.4 s by default) in a bubble above ctrl, or below it where the window
   * has no room above, or at the window's bottom centre without one; said once to a screen reader.
   */
  function showBubble(text: string, ctrl?: HTMLElement | null, opts?: { ms?: number }): void;
  /** Says how an action went in a message element; no text hides it. */
  function showResult(el: HTMLElement, text: string, ok?: boolean): void;
  /** Runs fn, which sends a request, with btn busy; nothing while it is busy. */
  function whileBusy<T>(btn: HTMLElement | null, fn: () => Promise<T>): Promise<T | undefined>;
  /** Shows a message in the page's toast for ms milliseconds, with an action's button where given. */
  function showToast(text: string, opts?: { title?: string; ms?: number; tone?: string; action?: { label: string; run: () => void } | null }): void;
  /** Saves a blob as a file of the name given. */
  function downloadBlob(name: string, blob: Blob): void;
}

export {};
