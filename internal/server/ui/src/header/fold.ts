// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/fold.ts
 *
 * The header's row folds where it is short, step by step, the first step
 * at which it fits (AP GlobalMenu, D195-B): the logo becomes the M, then
 * the Cube choice a cube button, then the pile goes to the next row. The
 * step is data-fold on the row, which style.css reads. The row is
 * measured whole (data-fold "measure", each item at its whole width) and
 * as if ↑ were among the pages, so that scrolling never changes the fold;
 * again whenever the row or one of its items changes size, and at each of
 * the header's renders, which bring the pile's icons and the Cube's name.
 * The measure is clipped to the row's width (style.css): the row at its
 * whole width is wider than a phone's page, which would become as wide,
 * and the browser would zoom out and cut the scroll of a page near its
 * end, at every poll.
 * Each fit places an open list of the row again (placeMenus of
 * components.js): the pill and the Cube choice move as the row folds and
 * as its items change width, and a list placed for where its summary
 * was would hang out of the window (the pile's at 390 px).
 */

/** The steps, each a value of data-fold, the first that fits taken. */
const STEPS = ["", "logo", "logo cube", "logo cube pile"];

/** width is an element's width, 0 where it is not shown. */
function width(el: Element | null): number {
  return el ? el.getBoundingClientRect().width : 0;
}

/** fit sets the row's fold, measured whole: the first step at which its items fit. */
function fit(line: HTMLElement): void {
  line.dataset.fold = "measure";
  const nav = line.querySelector<HTMLElement>(".nav");
  const home = line.querySelector<HTMLElement>("#view-tiles");
  const up = line.querySelector<HTMLElement>("#top-btn");
  const name = line.querySelector<HTMLElement>(".brand-name");
  if (!nav || !nav.offsetParent || !name) {
    // signed out: the logo and the version alone
    line.dataset.fold = "";
    return;
  }
  const gap = parseFloat(getComputedStyle(line).columnGap) || 0;
  const style = getComputedStyle(name);
  const button = width(home);
  // ↑ counted while it is not shown, as wide as ⌂, with the pages' gap
  const pages = width(nav) + (up && !up.offsetParent ? button + (parseFloat(getComputedStyle(nav).columnGap) || 0) : 0);
  const logo = width(line.querySelector(".brand"));
  const mark = (parseFloat(style.getPropertyValue("--brand-h")) || button) * (parseFloat(style.getPropertyValue("--brand-mark-ratio")) || 1.09);
  const pile = width(line.querySelector(".pile"));
  const cube = width(line.querySelector("#cube-choice"));
  // the Cube choice is one of the pages' group, nearer to them than the
  // row's gap: the distance as it is, measured
  const choice = line.querySelector(".cube");
  const join = choice && choice.getBoundingClientRect().width ? nav.getBoundingClientRect().left - choice.getBoundingClientRect().right : gap;
  const row = (...ws: number[]) => ws.reduce((a, b) => a + b, 0) + gap * (ws.filter(Boolean).length - 1);
  const needs = [row(logo, pile, cube), row(mark, pile, cube), row(mark, pile, button)].map((w) => w + join + pages);
  const step = needs.findIndex((n) => n <= line.clientWidth + 0.5);
  line.dataset.fold = STEPS[step < 0 ? STEPS.length - 1 : step];
  placeMenus(line);
}

/** The observer of the row and its items, made at the first watch. */
let observer: ResizeObserver | null = null;
/** The elements observed, each once. */
const watched = new WeakSet<Element>();
/** The frame a fit waits for. */
let frame = 0;

/** later fits the row at the next frame, once for every change before it. */
function later(line: HTMLElement): void {
  cancelAnimationFrame(frame);
  frame = requestAnimationFrame(() => fit(line));
}

/**
 * watchFold fits the header's row at the next frame, and from then on
 * whenever the row or one of its items changes size; called at each of
 * the header's renders, it observes the items its parts have drawn since.
 */
export function watchFold(): void {
  const line = document.querySelector<HTMLElement>(".top-line");
  if (!line) return;
  if (!observer) observer = new ResizeObserver(() => later(line));
  // the pill itself: on its own row the pile is as wide as the row
  for (const el of [line, ...line.querySelectorAll(".brand, #cube-problems, #cube-choice, .nav")]) {
    if (watched.has(el)) continue;
    watched.add(el);
    observer.observe(el);
  }
  later(line);
}
