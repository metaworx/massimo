# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Back to top (AP PageMenu, Block 4): ↑ filled in the accent's blue at
the bottom right of every page once it is a screen down, and the
header's ↑ left of ☰ while the header is compacted; each
takes the page back to its top."""
from helpers import scroll_settled, show_view, wait_until

PAGES = {"tiles": "rooms", "log": "logview", "me": "me", "cube": "cube-page", "system": "system", "info": "info-page"}
SHOWN = "!document.getElementById('to-top').classList.contains('hidden')"


def long_page(page, section):
    """A page made long enough to scroll several screens, whatever it holds
    on the test's server."""
    page.evaluate("""(id) => { const d = document.createElement('div');
      d.className = 'test-spacer'; d.style.height = '3000px'; document.getElementById(id).appendChild(d); }""", section)


def scrolled_to(page, y):
    page.evaluate("(y) => window.scrollTo({top: y, behavior: 'instant'})", y)
    scroll_settled(page)


def test_to_top_on_every_page(page, case):
    """On every page: no ↑ at the top nor half a screen down, ↑ at the
    bottom right a screen down, in the accent's blue; a click takes the
    page to its top and ↑ goes."""
    accent = page.evaluate("getComputedStyle(document.documentElement).getPropertyValue('--accent').trim()")
    top = page.locator("#to-top")
    for view, section in PAGES.items():
        show_view(page, view)
        long_page(page, section)
        scrolled_to(page, 0)
        assert top.is_hidden(), f"no ↑ at the top of {view}"
        height = page.evaluate("window.innerHeight")
        scrolled_to(page, height // 2)
        assert top.is_hidden(), f"none half a screen down on {view}"
        scrolled_to(page, height + 40)
        assert wait_until(page, SHOWN, 3), f"↑ a screen down on {view}"
        b = top.bounding_box()
        assert b["x"] + b["width"] <= case.width - 16 + 1 and b["y"] + b["height"] <= page.evaluate("window.innerHeight") - 16 + 1, \
            f"at the bottom right: {b}"
        fill = top.evaluate("(e) => getComputedStyle(e).backgroundColor")
        same = page.evaluate("([a, b]) => { const d = document.createElement('div'); d.style.color = a; document.body.appendChild(d);"
                             " const c = getComputedStyle(d).color; d.remove(); return c === b; }", [accent, fill])
        assert same, f"filled in the accent's blue ({fill} vs {accent})"
        top.click()
        assert wait_until(page, "window.scrollY === 0", 3), f"↑ takes {view} to its top"
        assert wait_until(page, "document.getElementById('to-top').classList.contains('hidden')", 3), "and goes"
        page.evaluate("document.querySelectorAll('.test-spacer').forEach((e) => e.remove())")


def test_to_top_in_the_compacted_header(page, case):
    """The header's ↑ shows while the header is compacted, left of ☰, and
    takes the page to its top."""
    btn = page.locator("#top-btn")
    long_page(page, "rooms")
    try:
        assert btn.is_hidden(), "no ↑ in the whole header"
        scrolled_to(page, 400)
        assert wait_until(page, "document.body.classList.contains('scrolled')", 3), "the header compacted"
        assert btn.is_visible(), "↑ in the compacted header"
        last = "#nav-menu > summary"
        b, n = btn.bounding_box(), page.locator(last).bounding_box()
        assert b["x"] + b["width"] <= n["x"] and abs(b["y"] - n["y"]) <= 1, f"left of {last}, level with it: {b} vs {n}"
        btn.click()
        assert wait_until(page, "window.scrollY === 0", 3), "↑ takes the page to its top"
        assert wait_until(page, "!document.body.classList.contains('scrolled')", 3), "the header whole again"
        assert btn.is_hidden(), "and ↑ gone from it"
    finally:
        page.evaluate("document.querySelectorAll('.test-spacer').forEach((e) => e.remove())")
