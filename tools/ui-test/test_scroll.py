# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A slow scroll down from the top (components.js stickyHeader): the
header compacts once the page is 96 px down and stays so, and the page
goes down at every step. With expert mode on and the found Cubes counted
under the header, the browser's scroll anchoring once took the page back
to the top each time the header compacted, the header grew back, and the
screen flickered until a scroll went past what compacting hides; that
line has gone, and a line of the test's own under the header, which
compacting hides as it did that one, stands in for it.

The header is let settle first (quiet_header): one that shrinks while the
page is a few pixels down, a notice of a radiogram pending that goes,
has the browser's scroll anchoring keep the content where it was by
scrolling up, which a step would read as going back. A notice that stays
moves nothing: with the fake Cube one radiogram counts as pending for good
once any test has sent a command, the fake answering every command with
49 of its 50 slots free and a Cube saying its slots only then."""
import pytest
from helpers import scroll_settled, wait_until
from test_send import first_tile, open_tab


def quiet_header(page):
    """Waits until nothing changes the header's height any more: the
    header as high for 0.8 s, a notice of a radiogram pending gone or
    there to stay."""
    assert wait_until(page, """(() => {
        const h = document.querySelector('.top').getBoundingClientRect().height;
        const now = performance.now();
        if (window.quietHeight !== h) {
          window.quietHeight = h;
          window.quietSince = now;
        }
        return now - window.quietSince >= 800;
    })()""", 10), "the header's height settled"


def test_slow_scroll_goes_down(page, case):
    """Expert mode on, a line under the header that compacting hides, as
    the found Cubes' count was, the page scrolled down 8 px at a time:
    every step goes down, and the header compacts once and does not grow
    back on the way; then up again to the top, where the header grows back
    once."""
    page.evaluate("""async () => {
      state.settings.expert = true;
      applySettings();
      const line = document.createElement('p');
      line.id = 'scroll-line';
      line.className = 'notice';
      line.textContent = 'a line under the header';
      document.getElementById('tls-notice').after(line);
      // hidden with the header compacted, in the same layout, as the
      // count was: a rule of the page's style sheet, the policy taking no
      // style element of the test's
      const sheet = [...document.styleSheets].find((s) => (s.href || '').endsWith('style.css'));
      window.scrollRule = sheet.insertRule('body.scrolled #scroll-line { display: none !important; }', sheet.cssRules.length);
      // room to scroll at every width
      const spacer = document.createElement('div');
      spacer.id = 'scroll-spacer';
      spacer.style.height = '2000px';
      document.querySelector('main').appendChild(spacer);
      window.scrollTo(0, 0);
    }""")
    quiet_header(page)
    assert wait_until(page, "window.scrollY === 0 && !document.body.classList.contains('scrolled')", 3), \
        "at the top, the header whole"
    page.evaluate("""() => {
      window.scrollLog = [];
      let last = false;
      new MutationObserver(() => {
        const now = document.body.classList.contains('scrolled');
        if (now !== last) window.scrollLog.push(now);
        last = now;
      }).observe(document.body, { attributes: true, attributeFilter: ['class'] });
    }""")
    ys = []
    for _ in range(40):
        page.evaluate("window.scrollBy(0, 8)")
        page.wait_for_timeout(40)
        ys.append(page.evaluate("Math.round(window.scrollY)"))
    page.wait_for_timeout(200)
    ys.append(page.evaluate("Math.round(window.scrollY)"))
    back = [(i, ys[i - 1], ys[i]) for i in range(1, len(ys)) if ys[i] < ys[i - 1]]
    assert not back, f"the page went back up while scrolling down: (step, from, to) {back[:5]}, all {ys}"
    assert ys[-1] >= 300, f"the page went down 320 px in steps ({ys})"
    toggles = page.evaluate("window.scrollLog")
    assert toggles == [True], f"the header compacted once and stayed so ({toggles})"
    # and back up to the top, where the header grows back once
    ys = [ys[-1]]
    while ys[-1] > 0 and len(ys) < 80:
        page.evaluate("window.scrollBy(0, -8)")
        page.wait_for_timeout(40)
        ys.append(page.evaluate("Math.round(window.scrollY)"))
    down = [(i, ys[i - 1], ys[i]) for i in range(1, len(ys)) if ys[i] > ys[i - 1]]
    assert not down and ys[-1] == 0, f"the page went up to the top step by step ({ys})"
    page.wait_for_timeout(200)
    toggles = page.evaluate("window.scrollLog")
    assert toggles == [True, False], f"at the top the header grew back once ({toggles})"
    page.evaluate("""() => {
      document.getElementById('scroll-spacer').remove();
      document.getElementById('scroll-line').remove();
      [...document.styleSheets].find((s) => (s.href || '').endsWith('style.css')).deleteRule(window.scrollRule);
      state.settings.expert = false;
      applySettings();
      window.scrollTo(0, 0);
    }""")


def test_scroll_to_the_bottom_stays_there(page, case):
    """The page scrolled to its bottom at once, as a click on Older at the
    log's end does: the header compacts and the page stays at its bottom.
    Measuring the compacted header once laid the page out shorter for a
    moment, the browser cut the scroll to fit it, and the button at the
    end went out of view."""
    page.evaluate("""() => {
      const spacer = document.createElement('div');
      spacer.id = 'scroll-spacer';
      spacer.style.height = '400px';
      document.querySelector('main').appendChild(spacer);
      window.scrollTo(0, 0);
    }""")
    quiet_header(page)
    assert wait_until(page, "window.scrollY === 0 && !document.body.classList.contains('scrolled')", 3), \
        "at the top, the header whole"
    page.evaluate("window.scrollTo(0, document.documentElement.scrollHeight)")
    assert wait_until(page, "document.body.classList.contains('scrolled')", 3), "the header compacted"
    page.wait_for_timeout(200)
    y, bottom = page.evaluate("[Math.round(window.scrollY), document.documentElement.scrollHeight - innerHeight]")
    assert abs(y - bottom) <= 1, f"the page stays at its bottom ({y}, its bottom {bottom})"
    page.evaluate("() => { document.getElementById('scroll-spacer').remove(); window.scrollTo(0, 0); }")




def test_a_jump_down_stays_where_it_went(page, case):
    """A room's configuration open on its week programme, the page
    scrolled at once from the top, as a control scrolled into view is, to
    every 25 px down to 1000: the header compacts and the page stays where
    it went. The browser's scroll anchoring, switched off on the root
    alone, took the page back by the compacted header's height for a band
    of such jumps, 53 px at 390 px, and a click aimed before the
    compaction landed above its target. Only a move back counts: one
    forward is the anchoring that keeps the view where it is while
    something above it grows (725 → 728 at 390 px on a CI runner,
    2026-11-15, what grew not seen)."""
    tile, _ = first_tile(page)
    open_tab(tile, "program")
    end = page.evaluate("Math.min(1000, document.documentElement.scrollHeight - innerHeight)")
    moved = []
    for want in range(150, end + 1, 25):
        page.evaluate("window.scrollTo(0, 0)")
        assert wait_until(page, "!document.body.classList.contains('scrolled')", 3), "the header whole at the top"
        page.evaluate("(y) => window.scrollTo(0, y)", want)
        assert wait_until(page, "document.body.classList.contains('scrolled')", 3), "the header compacted"
        assert scroll_settled(page), "the page comes to rest"
        y = page.evaluate("Math.round(window.scrollY)")
        if y < want - 1:
            moved.append(f"{want} → {y}")
    assert not moved, f"the page went back after the jump: {', '.join(moved)}"
    page.evaluate("window.scrollTo(0, 0)")
    tile.locator(".config-tabs .close").click()
