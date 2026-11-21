// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/chrome/scroll.ts
 *
 * The page's header kept at the top and compacted once the page has
 * scrolled, and back to the top (AP UiParts, Block 14; stickyHeader,
 * backToTop and settleAtTop of components.js, moved as they were). The
 * header is the page's skeleton, header.top: nothing here draws, so that
 * no drawing runs on a scroll's path (tools/ui-test/test_scroll.py).
 */

/**
 * stickyHeader keeps the page's header at the top (style.css .top): its
 * height in --top-h, for the page's row under it, and the class
 * scrolled on the body once the page has scrolled, which compacts the
 * header. The compacted header keeps the height it gave up as its
 * bottom margin, so that the page is as long scrolled as not: a page
 * little taller than the window would otherwise become too short to stay
 * scrolled, the header grow back, and the page jump to and fro.
 * Scrolled from 96 px down, not again under 8 px.
 *
 * No layout on the way may be shorter than the page, nor move what
 * follows the header where the browser can see it (tools/ui-test/
 * test_scroll.py). Compacting measures the compacted header, a layout of
 * its own: it is made with a margin as tall as the whole header, so that
 * the page is longer for that moment, never shorter, where the browser
 * would cut a scroll near the page's end to fit (the log's Older went out
 * of view under the click). The browser's scroll anchoring is off
 * meanwhile: it took the scroll back by what the page under the header
 * moved, to the top where that was more than 96 px (the expert list, the
 * found Cubes open), the header grew back and the screen flickered. It is
 * switched off on the body as on the root, as Chrome takes the page's from
 * the body: off on the root alone, a jump down the page at 390 px went
 * back by the compacted header's 53 px (AP HeaderAnchor).
 * Growing back takes the margin and the class away in one layout.
 */
export function stickyHeader(): void {
  const top = document.querySelector<HTMLElement>(".top");
  if (!top) return;
  const root = document.documentElement;
  const anchoring = (v: string) => {
    root.style.overflowAnchor = v;
    document.body.style.overflowAnchor = v;
  };
  // the border box: compacting changes the padding, which the content
  // box does not see
  const height = () => root.style.setProperty("--top-h", top.getBoundingClientRect().height + "px");
  new ResizeObserver(height).observe(top, { box: "border-box" });
  const set = (on: boolean) => {
    if (on === document.body.classList.contains("scrolled")) return;
    anchoring("none");
    if (on) {
      const whole = top.getBoundingClientRect().height;
      const margin = parseFloat(getComputedStyle(top).marginBottom) || 0;
      top.style.marginBottom = margin + whole + "px";
      document.body.classList.add("scrolled");
      top.style.marginBottom = margin + whole - top.getBoundingClientRect().height + "px";
    } else {
      top.style.marginBottom = "";
      document.body.classList.remove("scrolled");
    }
    height(); // laid out with the margin, before the anchoring is back
    anchoring("");
  };
  const onScroll = () => {
    const y = window.scrollY;
    if (y > 96) set(true);
    else if (y < 8) set(false);
  };
  addEventListener("scroll", onScroll, { passive: true });
  onScroll();
}

/**
 * toTop takes the page back to its top, the focus on the page's sections
 * menu where it has one, else on its button in the header: what both ↑
 * do, the floating one (ToTop) and the compacted header's.
 */
export function toTop(): void {
  const smooth = !matchMedia("(prefers-reduced-motion: reduce)").matches;
  window.scrollTo({ top: 0, behavior: smooth ? "smooth" : "auto" });
  settleAtTop();
  const shown = (sel: string) => [...document.querySelectorAll<HTMLElement>(sel)].find((e) => e.offsetParent);
  const to = shown("main .settings-page:not(.hidden) .sec-menu > summary") || shown(".nav [aria-current='page'], .nav summary.active");
  if (to) to.focus({ preventScroll: true });
}

/**
 * settleAtTop keeps a scroll to the top from ending a few pixels short:
 * on a CI runner the page reached the top and was moved back 3 px as the
 * header grew back (stickyHeader), and stayed there. For 3 s a scroll
 * that leaves the page above 0 and under 8 px, where the header counts it
 * at the top, is taken the rest of the way; one from 8 px on is the
 * user's and is left alone.
 */
function settleAtTop(): void {
  const until = performance.now() + 3000;
  const settle = () => {
    if (performance.now() > until) {
      removeEventListener("scroll", settle);
      return;
    }
    const y = window.scrollY;
    if (y > 0 && y < 8) window.scrollTo({ top: 0 });
  };
  addEventListener("scroll", settle, { passive: true });
}
