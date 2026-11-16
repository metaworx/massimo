// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/icon.tsx
 *
 * An icon, a sign and a mark as parts: iconEl, signEl and markEl of
 * components.js, drawn once and updated in place. A mark is an element of
 * its own (use.mark), which is what blinks; it is made again only where
 * its state changes, and then starts its blinking where the mark of its
 * key first came (keepBlink), so that a mark that stays keeps its phase
 * by itself.
 */

import { useLayoutEffect, useRef } from "preact/hooks";
import { useIcons } from "./look";

/**
 * Icon is a small inline icon of a symbol of the page (iconEl); with mark,
 * the symbol is a mark (use.mark), which blinks where the page's style
 * makes it, as a sign's state that calls for a look.
 */
export function Icon({ symbol, cls, mark }: { symbol: string; cls: string; mark?: boolean }) {
  return (
    <svg class={cls} aria-hidden="true">
      <use href={"#" + symbol} class={mark ? "mark" : undefined} />
    </svg>
  );
}

/**
 * MarkUse is a mark's drawing, the triangle of a warning or the cross of
 * a failure; blink names it across the page (keepBlink). Give it a key of
 * its state, so that another state is another element.
 */
function MarkUse({ href, bad, blink }: { href: string; bad: boolean; blink?: string }) {
  const ref = useRef<SVGUseElement>(null);
  useLayoutEffect(() => {
    if (ref.current) keepBlink(ref.current, blink);
  }, []);
  return <use ref={ref} href={"#" + href} class={bad ? "mark x" : "mark"} />;
}

/**
 * Sign is the icon of a kind of SIGNS in a state (signEl): "" or "ok"
 * plain, "warn" with a triangle, "bad" with a cross, coloured by the
 * state, in the drawing the person chose for the window and the radio
 * (it reads the choice itself); cls adds classes, blink names the mark across the page, and
 * label, where given, names the sign for a screen reader (a sign that
 * stands alone for a state), else it is hidden from one.
 */
export function Sign({ kind, state, cls, blink, label }: { kind: string; state: string; cls?: string; blink?: string; label?: string }) {
  useIcons(); // the drawing of the window and the radio is the person's choice
  const s = signOf(kind);
  const mark = (s as unknown as Record<string, string | undefined>)[state];
  return (
    <svg class={"ni sign" + (state ? " tone-" + state : "") + (cls ? " " + cls : "")} role={label ? "img" : undefined} aria-label={label}
      aria-hidden={label ? undefined : "true"}>
      <use href={"#" + (mark ? s.base : s.icon)} />
      {mark && <MarkUse key={mark} href={mark} bad={state === "bad"} blink={blink} />}
    </svg>
  );
}

/** Mark is a mark alone (markEl), for a badge or a list: "warn" or "bad". */
export function Mark({ state, blink }: { state: "warn" | "bad"; blink?: string }) {
  return (
    <svg class={"ni mark-sign tone-" + state} aria-hidden="true">
      <MarkUse key={state} href={state === "bad" ? "mark-x" : "mark-warn"} bad={state === "bad"} blink={blink} />
    </svg>
  );
}

/**
 * CountIcon is an icon with a number in its corner badge (foundCountEl of
 * components.js), hidden from a screen reader, which the label beside it
 * tells; cls is its class, which colours and places it.
 */
export function CountIcon({ symbol, n, cls }: { symbol: string; n: number; cls: string }) {
  return (
    <span class={cls} aria-hidden="true">
      <Icon symbol={symbol} cls="ni" />
      <b>{String(n)}</b>
    </span>
  );
}
