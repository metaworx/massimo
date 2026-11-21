// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/question.tsx
 *
 * The question box as a part (confirmBox of components.js), and a
 * button's request under way: the question in the page, No and Yes, Yes
 * busy and No off from Yes's click until its request has answered, so
 * that a double click sends one; Escape for No. A part shows it in place
 * of the row of actions that asked, and gives the focus back to the
 * button that asked once it goes.
 */

import type { Ref } from "preact";
import { useId, useLayoutEffect, useRef, useState } from "preact/hooks";

/**
 * useBusy is a request under way, for the button that sends it: busy
 * from run's start until the promise fn returns has settled; a second
 * run meanwhile does nothing and returns undefined (whileBusy).
 */
export function useBusy(): { busy: boolean; run<T>(fn: () => Promise<T>): Promise<T | undefined> } {
  const [busy, setBusy] = useState(false);
  const on = useRef(false);
  return {
    busy,
    async run(fn) {
      if (on.current) return undefined;
      on.current = true;
      setBusy(true);
      try {
        return await fn();
      } finally {
        on.current = false;
        setBusy(false);
      }
    },
  };
}

/**
 * useSetAtOnce is a setting saved at once (a switch without a Save): value
 * is stored, the value the server last gave, but while its request runs
 * the value the user chose, so that the control shows it meanwhile; set
 * sends fn's request with the value chosen. The control stays on, so
 * that the keyboard's focus stays on it: a change while a request runs
 * sends nothing and is drawn back to the value chosen. Once the request
 * has answered the control shows stored again, the new value where it
 * was taken, the old one where it was refused.
 */
export function useSetAtOnce<T>(stored: T): { value: T; busy: boolean; set(v: T, fn: (v: T) => Promise<unknown>): void } {
  const [chosen, setChosen] = useState<{ v: T } | null>(null);
  const [, redraw] = useState(0);
  const sending = useBusy();
  return {
    value: chosen ? chosen.v : stored,
    busy: sending.busy,
    set(v, fn) {
      if (sending.busy) {
        redraw((n) => n + 1); // the control drawn back to the value chosen
        return;
      }
      setChosen({ v });
      sending.run(() => fn(v)).finally(() => setChosen(null));
    },
  };
}

/**
 * BusyButton is a button that sends a request and turns until its answer
 * (useBusy, busyProps): cls its class, label its text, run the request;
 * id its element's (a button the page names); aria-label names it for a
 * screen reader where its text does not say enough; off keeps it off (a
 * request that may not be sent now); data are its data- attributes (a
 * Link button's provider in My profile); a click while it turns sends
 * nothing.
 */
export function BusyButton({ id, cls, label, run, aria, off, data }: {
  id?: string;
  cls?: string;
  label: string;
  run: () => Promise<unknown>;
  aria?: string;
  off?: boolean;
  data?: Record<string, string>;
}) {
  const b = useBusy();
  const attrs: Record<string, string> = {};
  for (const [k, v] of Object.entries(data || {})) attrs["data-" + k] = v;
  return <button type="button" id={id} {...busyProps(cls || "", b.busy, off)} aria-label={aria} {...attrs} onClick={() => b.run(run)}>{label}</button>;
}

/**
 * busyProps are a button's attributes while its request runs (setBusy):
 * off, the class busy (a turning ring after its text) and aria-busy;
 * off also where nothing is to be sent.
 */
export function busyProps(cls: string, busy: boolean, off?: boolean) {
  return { class: busy ? cls + " busy" : cls, disabled: busy || !!off, "aria-busy": busy ? ("true" as const) : undefined };
}

/**
 * Question asks whether to go on: text, then No and Yes (their labels),
 * Yes the primary; Yes runs onYes, which returns the request's promise,
 * No and Escape run onNo. The focus goes to No when it comes. boxRef,
 * where given, holds the box (useAsk's box), which tells whether the
 * focus is in it as it goes.
 */
export function Question({ text, yes, no, onYes, onNo, boxRef }: {
  text: string;
  yes: string;
  no: string;
  onYes: () => Promise<unknown>;
  onNo: () => void;
  boxRef?: Ref<HTMLDivElement>;
}) {
  const id = useId();
  const noRef = useRef<HTMLButtonElement>(null);
  const sending = useBusy();
  // in the drawing's own turn, as the plain question did, not after the next frame
  useLayoutEffect(() => {
    noRef.current?.focus();
  }, []);
  return (
    <div
      ref={boxRef}
      class="panel confirm-box"
      role="group"
      aria-labelledby={id}
      onKeyDown={(ev) => {
        if (ev.key !== "Escape") return;
        ev.preventDefault();
        ev.stopPropagation();
        onNo();
      }}
    >
      <span id={id} class="confirm-text">{text}</span>
      <div class="actions">
        <button ref={noRef} type="button" class="confirm-no" disabled={sending.busy} onClick={onNo}>{no}</button>
        <button type="button" {...busyProps("confirm-yes primary", sending.busy)} onClick={() => sending.run(onYes)}>{yes}</button>
      </div>
    </div>
  );
}

/**
 * useAsk is a row of actions that asks in its own place: asking while
 * the question is shown; ask shows it, done takes it back and puts the
 * focus on the element ref holds, the button that asked, once the row is
 * drawn again: in that drawing's turn (a layout effect), as the plain
 * questions did at once, not after the next frame. held, where given,
 * keeps whether it asks beyond the part, a store's live state, so that
 * the store takes the question back where what it asked about changed,
 * and at a sign-out (a part drawn once for the page's life); without it
 * the part keeps it. Where the caller hands box to its Question, a
 * question the store takes back while the focus is in its box gives the
 * focus back the same way, judged as the drawing that takes it away
 * begins, before the box leaves the page; done(false) gives it back to
 * nothing, as before. Where the button cannot take it (hidden or off),
 * the element row holds takes it (a row of the caller's, which holds it
 * with tabindex -1 until it leaves).
 */
export function useAsk<E extends HTMLElement>(held?: { asking: boolean }) {
  const [own, setOwn] = useState(false);
  const asking = held ? held.asking : own;
  const setAsking = (v: boolean) => {
    if (held) held.asking = v;
    else setOwn(v);
  };
  const back = useRef(false);
  const ref = useRef<E>(null);
  const row = useRef<HTMLElement>(null);
  const box = useRef<HTMLDivElement>(null);
  const quiet = useRef(false);
  const inside = useRef(false);
  // the drawing that takes the question away begins with its box still in the page: whether the focus is in it
  if (!asking && box.current && box.current.isConnected && box.current.contains(document.activeElement)) inside.current = true;
  // in that drawing's own turn, as the plain questions did, not after the next frame
  useLayoutEffect(() => {
    const lost = inside.current && !quiet.current;
    inside.current = false;
    quiet.current = false;
    if (!asking && (back.current || lost)) {
      back.current = false;
      const b = ref.current as (E & { disabled?: boolean }) | null;
      const r = row.current;
      if (b && !b.disabled && b.offsetParent) b.focus();
      else if (r) {
        // the row takes the focus for as long as it holds it, and no click on its text after
        r.tabIndex = -1;
        r.addEventListener("blur", () => r.removeAttribute("tabindex"), { once: true });
        r.focus();
      }
    }
  }, [asking]);
  return {
    asking,
    ref,
    row,
    box,
    ask: () => setAsking(true),
    done: (focus = true) => {
      back.current = focus;
      quiet.current = !focus;
      setAsking(false);
    },
  };
}
