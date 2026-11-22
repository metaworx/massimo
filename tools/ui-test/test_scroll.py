# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A slow scroll down from the top (ui/src/chrome/scroll.ts stickyHeader): the
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
moves nothing: with the fake Cube one radiogram counts as pending once any
test has sent a command, the fake answering every command with 49 of its
50 slots free and a Cube saying its slots only then; it goes when the
server reconnects for the budget, two minutes after the last write, and
the greeting says all 50 free, at a moment no test chooses."""
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
    once. A step is read by the page's content, the room below the header,
    as a user sees it move: a notice of a radiogram pending that goes in the
    middle of the scroll (two minutes after an earlier test's write, at a
    moment no test chooses) makes the header lower, and the browser's
    scroll anchoring then keeps the content where it was by scrolling up by
    the notice's height, which window.scrollY would read as going back."""
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
    content = "Math.round(document.getElementById('scroll-spacer').getBoundingClientRect().top)"
    ys, tops = [], [page.evaluate(content)]
    for _ in range(40):
        page.evaluate("window.scrollBy(0, 8)")
        page.wait_for_timeout(40)
        ys.append(page.evaluate("Math.round(window.scrollY)"))
        tops.append(page.evaluate(content))
    page.wait_for_timeout(200)
    ys.append(page.evaluate("Math.round(window.scrollY)"))
    tops.append(page.evaluate(content))
    back = [(i, tops[i - 1], tops[i]) for i in range(1, len(tops)) if tops[i] > tops[i - 1]]
    assert not back, f"the content came back down while scrolling down: (step, from, to) {back[:5]}, all {tops}"
    assert tops[0] - tops[-1] >= 300, f"the content went up 320 px in steps ({tops})"
    toggles = page.evaluate("window.scrollLog")
    assert toggles == [True], f"the header compacted once and stayed so ({toggles})"
    # and back up to the top, where the header grows back once
    ys, tops = [ys[-1]], [tops[-1]]
    while ys[-1] > 0 and len(ys) < 80:
        page.evaluate("window.scrollBy(0, -8)")
        page.wait_for_timeout(40)
        ys.append(page.evaluate("Math.round(window.scrollY)"))
        tops.append(page.evaluate(content))
    down = [(i, tops[i - 1], tops[i]) for i in range(1, len(tops)) if tops[i] < tops[i - 1]]
    assert not down and ys[-1] == 0, f"the page went up to the top step by step ({ys}, the content {tops})"
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


# The page scrolled at once to y, and the top in the page of every node in
# view kept, in the same call, before anything can move: every element
# with a box and every text node (measured through a Range) that reaches
# into the view's height by its box or by anything inside it, the walk
# going through display: contents and leaving out what display: none,
# fixed, sticky and overflow-anchor: none take out. Blink's scroll
# anchoring holds one node in place and moves the page by as much as that
# node went down (scroll_anchor.cc): a box or a text node in view by its
# box and what overflows it, never one of those left out, so it is among
# these, whichever it is; a generated box (::before, ::after), which no
# script can measure, lies in its element's box, which is among them. The
# test does not tell which one Blink holds. Returns how many were kept.
JUMP = """(y) => {
  window.scrollTo(0, y);
  const view = innerHeight;
  const kept = [];
  const textRect = (n) => {
    const range = document.createRange();
    range.selectNodeContents(n);
    return range.getClientRects().length ? range.getBoundingClientRect() : null;
  };
  // the walk returns the lowest bottom of n and of what is inside it, in
  // the view's coordinates; keep is false in a subtree left out, whose
  // boxes still reach down for the boxes around them
  const walk = (n, keep) => {
    if (n.nodeType === Node.TEXT_NODE) {
      const r = textRect(n);
      if (!r) return -Infinity;
      if (keep && r.top < view && r.bottom > 0) kept.push([n, r.top + window.scrollY]);
      return r.bottom;
    }
    if (n.nodeType !== Node.ELEMENT_NODE) return -Infinity;
    const s = getComputedStyle(n);
    if (s.display === 'none' || s.position === 'fixed') return -Infinity;
    const inside = keep && s.position !== 'sticky' && s.overflowAnchor !== 'none';
    const boxed = n.getClientRects().length > 0;
    const r = n.getBoundingClientRect();
    let low = boxed ? r.bottom : -Infinity;
    for (const c of [...(n.shadowRoot ? n.shadowRoot.childNodes : []), ...n.childNodes]) low = Math.max(low, walk(c, inside));
    if (inside && boxed && (r.width > 0 || r.height > 0) && r.top < view && low > 0) kept.push([n, r.top + window.scrollY]);
    return low;
  };
  walk(document.documentElement, true);
  window.jumpKept = kept;
  return kept.length;
}"""

# The page's position, and the farthest any node kept at the jump has gone
# down in the page since: the most that grew above the view; null where
# none of them is left in the page.
GREW = """() => {
  let most = null;
  for (const [n, top] of window.jumpKept) {
    if (!n.isConnected) continue;
    let r;
    if (n.nodeType === Node.TEXT_NODE) {
      const range = document.createRange();
      range.selectNodeContents(n);
      if (!range.getClientRects().length) continue;
      r = range.getBoundingClientRect();
    } else {
      if (!n.getClientRects().length) continue;
      r = n.getBoundingClientRect();
    }
    const down = r.top + window.scrollY - top;
    if (most === null || down > most) most = down;
  }
  return [Math.round(window.scrollY), most];
}"""


def test_a_jump_down_stays_where_it_went(page, case):
    """A room's configuration open on its week programme, the page
    scrolled at once from the top, as a control scrolled into view is, to
    every 25 px down to 1000: the header compacts and the page stays where
    it went. The browser's scroll anchoring, switched off on the root
    alone, took the page back by the compacted header's height for a band
    of such jumps, 53 px at 390 px, and a click aimed before the
    compaction landed above its target. Then only a move back counted,
    since one forward is the anchoring that keeps the view where it is
    while something above it grows (725 → 728 at 390 px on a CI runner,
    2026-11-15, what grew not seen); a forward jump of a bug's making
    passed as well. Now a move forward counts where it is larger than
    what grew above the view (AP TestInfrastructure, D20-A), taken as the
    farthest any node in view at the jump (JUMP) went down in the page
    until the page came to rest, and 1 px. The anchoring moves the page by
    as much as the one node it holds went down, and that node is among
    them, so a correct page does not fail; a page that goes forward with
    nothing grown above it does. The jumps end where the page ends as it
    is at each one, read at the top: the header's notice of a radiogram
    pending goes when the server's reconnect for the budget, two minutes
    after a write of an earlier test, finds the fake Cube's slots free,
    and the page is then 63 px shorter at 1100 px; a bound read once
    took the jumps past its end, which the browser cut to it (500 → 488
    on a CI runner, 2026-11-20)."""
    tile, _ = first_tile(page)
    open_tab(tile, "program")
    moved = []
    for want in range(150, 1001, 25):
        page.evaluate("window.scrollTo(0, 0)")
        assert wait_until(page, "!document.body.classList.contains('scrolled')", 3), "the header whole at the top"
        if want > page.evaluate("document.documentElement.scrollHeight - innerHeight"):
            break
        kept = page.evaluate(JUMP, want)
        assert kept, f"nodes in view after the jump to {want}"
        assert wait_until(page, "document.body.classList.contains('scrolled')", 3), "the header compacted"
        assert scroll_settled(page), "the page comes to rest"
        y, grew = page.evaluate(GREW)
        assert grew is not None, f"none of the {kept} nodes in view after the jump to {want} is still in the page"
        if y < want - 1:
            moved.append(f"{want} → {y}, back")
        elif y > want + max(grew, 0) + 1:
            moved.append(f"{want} → {y}, at most {grew:.1f} px grown above the view")
    assert not moved, f"the page went back, or forward by more than grew above the view, after the jump: {', '.join(moved)}"
    page.evaluate("window.scrollTo(0, 0)")
    tile.locator(".config-tabs .close").click()
