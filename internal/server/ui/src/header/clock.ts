// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/clock.ts
 *
 * The header's one clock: the time as a signal, moved on at each whole
 * second, which every time the header shows reads (the pile's list, the
 * expert line, Times.tsx), so that they move on together and never
 * disagree; the two reads they show, on the browser's clock; and one
 * formatter of how long ago, in the page's language, "14 sec ago".
 */

import { signal } from "@preact/signals";
import { chosenCube } from "../model/cube";

/** now is the browser's time (ms), moved on at each whole second. */
export const now = signal(Date.now());

/** nowAgain moves now on at once, for data that has just come (header.render). */
export function nowAgain(): void {
  now.value = Date.now();
}

/** tick moves now on, and again at the next whole second. */
function tick(): void {
  now.value = Date.now();
  setTimeout(tick, 1000 - (Date.now() % 1000) + 5);
}
tick();

/**
 * lastReads are when the chosen Cube was last read (its updated, the
 * last message the server took from it, moved off the server's clock by
 * serverSkew of header.js) and when the server last answered the page
 * (state.repliedAt of app.js), on the browser's clock (ms); 0 where there
 * is none.
 */
export function lastReads(): { cube: number; server: number } {
  const cube = chosenCube();
  const at = cube ? Date.parse(cube.updated || "") : NaN;
  return { cube: isNaN(at) ? 0 : at - serverSkew, server: state.repliedAt || 0 };
}

/** clockOf is an instant (ms, on the browser's clock) as its time of day (fmtTimeOfDay of app.js); "–" where there is none. */
export function clockOf(ms: number): string {
  return !ms || isNaN(ms) ? "–" : fmtTimeOfDay(new Date(ms));
}

/** The formatters of how long ago, by language. */
const agoFormats = new Map<string, Intl.RelativeTimeFormat>();

/**
 * fmtAgo is how long before now an instant (ms) was, in the page's
 * language: seconds under a minute, then minutes, hours, from two days on
 * days ("14 sec ago", "vor 3 Min."); "–" where there is none.
 */
export function fmtAgo(ms: number, at: number = now.value): string {
  if (!ms || isNaN(ms)) return "–";
  const sec = Math.max(0, Math.floor((at - ms) / 1000));
  const [n, unit]: [number, Intl.RelativeTimeFormatUnit] =
    sec < 60 ? [sec, "second"] : sec < 3600 ? [Math.floor(sec / 60), "minute"] : sec < 48 * 3600 ? [Math.floor(sec / 3600), "hour"] : [Math.floor(sec / 86400), "day"];
  let f = agoFormats.get(i18n.lang);
  if (!f) {
    f = new Intl.RelativeTimeFormat(i18n.lang, { numeric: "always", style: "short" });
    agoFormats.set(i18n.lang, f);
  }
  return f.format(-n, unit);
}
