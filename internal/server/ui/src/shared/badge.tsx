// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/badge.tsx
 *
 * A badge, a notice, a value's age and an action's result line as parts:
 * badgeEl and showResult of components.js and app.js, where the plain
 * code still has them. An age shows how old its value is by the page's
 * minute (minuteNow of shared/data.ts), which it reads as a signal.
 */

import type { ComponentChildren } from "preact";
import type { Outcome } from "./action";
import { minuteNow } from "./data";
import { Mark } from "./icon";

/**
 * Badge is a small label (.flag) of a kind (ok, warn, bad, mode, fill-…):
 * a warning or a failure led by its mark, so that it reads without its
 * colour; blink names the mark across the page, by default its kind and
 * text; title is its tooltip, children follow the text (an icon, an age).
 */
export function Badge({ kind, text, blink, title, children }: { kind: string; text?: string; blink?: string; title?: string; children?: ComponentChildren }) {
  const state = badgeState(kind);
  return (
    <span class={"flag " + kind} title={title}>
      {state && <Mark state={state} blink={blink || (text ? kind + ":" + text : undefined)} />}
      {text}
      {children}
    </span>
  );
}

/**
 * Notice is a lasting warning (.notice) of a state the server keeps
 * trying to leave: since, an API time, adds from when it holds; next,
 * when the server tries again, is its tooltip. id and role are its
 * element's (a line of the page that a screen reader is told of, role
 * status), and hidden keeps it in its place without showing it, for a
 * notice that comes and goes where a test or the page's style finds it.
 */
export function Notice({ text, since, next, cls, id, role, hidden, children }: {
  text?: string;
  since?: string;
  next?: string;
  cls?: string;
  id?: string;
  role?: "status" | "alert" | "note";
  hidden?: boolean;
  children?: ComponentChildren;
}) {
  const from = Date.parse(since || "");
  const at = Date.parse(next || "");
  return (
    <p id={id} class={"notice" + (cls ? " " + cls : "") + (hidden ? " hidden" : "")} role={role}
      title={isNaN(at) ? undefined : t("notice.next", { time: new Date(at).toLocaleTimeString(i18n.lang) })}>
      {text}
      {!isNaN(from) && " "}
      {!isNaN(from) && <span class="notice-since">{t("notice.since", { time: fmtShortTime(from) })}</span>}
      {children}
    </p>
  );
}

/**
 * ResultLine is the line that says how an action went (showResult of
 * components.js), as an actionRunner's set gives it: the text, green where
 * it went well, red where not; hidden, in its place, while there is none.
 * cls is its class (tile-result, the small result line under a tile's or
 * a form's actions); role status has a screen reader told of each, alert
 * at once (the sign-in's lines); id is
 * its element's, for a field it describes (aria-describedby) or a
 * section's message line the page names.
 */
export function ResultLine({ res, cls, role, id }: { res: Outcome | null | undefined; cls: string; role?: "status" | "alert"; id?: string }) {
  return <p id={id} class={cls + (res ? (res.ok ? " ok" : " bad") : " hidden")} role={role}>{res?.text}</p>;
}

/**
 * Age is how old a value is that the server received at at (an API
 * time): the age, the exact time as its tooltip and in expert mode after
 * it; greyed when stale; plain leaves out the "already there" mark, for a
 * time that is no device value. Nothing without a time.
 */
export function Age({ at, stale, plain }: { at?: string; stale?: boolean; plain?: boolean }) {
  void minuteNow.value; // how old it is moves on with the page's minute
  const ms = Date.parse(at || "");
  if (isNaN(ms)) return null;
  const since = ms - observedSince();
  const already = !plain && since >= 0 && since < SINCE_MS;
  return (
    <span class={"age" + (stale ? " stale" : "")} data-at={String(ms)} data-since={already ? "1" : undefined}
      title={t(already ? "age.titleSince" : "age.title", { time: fmtInstant(new Date(ms)) })}>
      <span class="age-text">{fmtAge(ms, already)}</span>
      <span class="age-exact expert-only">{fmtShortTime(ms)}</span>
    </span>
  );
}

/**
 * Since says from when a badge's state holds, "since 16.11., 00:41", an
 * API time, after its text (an item of a problem list); nothing without
 * a time. Notice says its own since in its line.
 */
export function Since({ at }: { at?: string }) {
  const from = Date.parse(at || "");
  if (isNaN(from)) return null;
  return <span class="badge-since">{t("badge.since", { time: fmtShortTime(from) })}</span>;
}
