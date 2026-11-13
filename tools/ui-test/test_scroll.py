# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A slow scroll down from the top (components.js stickyHeader): the
header compacts once the page is 96 px down and stays so, and the page
goes down at every step. With expert mode on and the found Cubes open,
the browser's scroll anchoring once took the page back to the top each
time the header compacted, the header grew back, and the screen
flickered until a scroll went past what compacting hides."""
from helpers import FOUND2, wait_until


def test_slow_scroll_goes_down(page, case):
    """Expert mode on, a found Cube shown and open, the page scrolled
    down 8 px at a time: every step goes down, and the header compacts
    once and does not grow back on the way; then up again to the top,
    where the header grows back once."""
    page.evaluate("""(c) => {
      state.settings.expert = true;
      applySettings();
      state.events.dispatchEvent(new MessageEvent('cube_found', { data: JSON.stringify(
        { kind: 'cube_found', cube: c, time: c.found_at }) }));
      const found = document.getElementById('cube-found');
      if (found && 'open' in found) found.open = true;
      // room to scroll at every width
      const spacer = document.createElement('div');
      spacer.id = 'scroll-spacer';
      spacer.style.height = '2000px';
      document.querySelector('main').appendChild(spacer);
      window.scrollTo(0, 0);
    }""", FOUND2)
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
      state.settings.expert = false;
      applySettings();
      window.scrollTo(0, 0);
    }""")
