# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A slow scroll down from the top (components.js stickyHeader): the
header compacts once the page is 96 px down and stays so, and the page
goes down at every step. With expert mode on and the found Cubes open,
the browser's scroll anchoring once took the page back to the top each
time the header compacted, the header grew back, and the screen
flickered until a scroll went past what compacting hides.

The header is let settle first (quiet_header): one that shrinks while the
page is a few pixels down, a notice of a radiogram pending that goes,
has the browser's scroll anchoring keep the content where it was by
scrolling up, which a step would read as going back."""
from helpers import wait_until


def quiet_header(page):
    """Waits until nothing changes the header's height any more: no
    radiogram pending, whose notice goes once the Cube acknowledges it,
    and the header as high for 0.8 s."""
    assert wait_until(page, "document.getElementById('pending').classList.contains('hidden')", 10), "no radiogram pending"
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
    """Expert mode on, a found Cube counted under the header, the page
    scrolled down 8 px at a time: every step goes down, and the header
    compacts once and does not grow back on the way; then up again to the
    top, where the header grows back once. The found Cube is one the
    server found, by a search: one only the page was told of goes at the
    next poll, and the header with it."""
    page.evaluate("""async () => {
      state.settings.expert = true;
      applySettings();
      await api('POST', 'api/v1/cubes/discover');
      await loadCubes();
      // room to scroll at every width
      const spacer = document.createElement('div');
      spacer.id = 'scroll-spacer';
      spacer.style.height = '2000px';
      document.querySelector('main').appendChild(spacer);
      window.scrollTo(0, 0);
    }""")
    assert wait_until(page, "state.found.length > 0 && !document.getElementById('cube-found').classList.contains('hidden')", 10), \
        "a found Cube counted under the header"
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
