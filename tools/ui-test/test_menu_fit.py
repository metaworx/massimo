# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Menus kept in the window (components.js dropdown, placeMenu): the
header's problem list, long, opens inside the window at every width, as
high as the room below it, and scrolls in itself without the page; its
badges stay inside it, and it stays inside the window as its pill moves,
placed again without losing its scroll. The Cube choice and ☰ open
inside the window too."""
from helpers import CUBES, configured_only, reads_of, wait_until


def many_problems(n):
    """n devices in trouble, as GET /api/v1/cubes gives them."""
    return [{"rf_address": "0a01" + str(10 + i), "type": "radiator_thermostat", "name": "Radiator by the long window %d" % i,
             "room": "A room with a rather long name %d" % i, "serial": "MEQ00001" + str(10 + i), "reported": True,
             "state": {"error": i % 2 == 0, "battery_low": True, "link_error": i % 3 == 0}} for i in range(n)]


# The page made long enough to scroll, and back.
SPACER = """() => {
  const spacer = document.createElement("div");
  spacer.id = "fit-spacer";
  spacer.style.height = "3000px";
  document.querySelector("main").appendChild(spacer);
}"""

RESTORE = """() => {
  document.getElementById("fit-spacer")?.remove();
  window.scrollTo(0, 0);
}"""

# The open list's box, the window's, and the badges' right edges.
BOX = """(sel) => {
  const l = document.querySelector(sel);
  const r = l.getBoundingClientRect();
  const flags = [...l.querySelectorAll(".flag")].map((f) => f.getBoundingClientRect().right);
  return { left: r.left, right: r.right, bottom: r.bottom, vw: document.documentElement.clientWidth, vh: innerHeight,
    scrolls: l.scrollHeight > l.clientHeight + 1, flagRight: Math.max(...flags, 0) };
}"""


def inside(b):
    return b["left"] >= 15.5 and b["right"] <= b["vw"] - 15.5


def test_problem_list_fits(page, case):
    """The header's pill opens a long list inside the window, as high as
    the room below it, its badges inside it; the wheel over it scrolls
    the list, not the page. The first Cube's devices in trouble are in
    GET /api/v1/cubes, without the Cubes found, at every read of the
    page's (helpers.Reads); a read places the list again, and it keeps
    its scroll."""
    reads = reads_of(page)
    reads.change(CUBES, configured_only)
    reads.cube("KEQ0000100", problems=many_problems(14))
    page.evaluate("loadCubes()")
    page.evaluate(SPACER)
    try:
        page.locator("#cube-problems .problems-sum").click()
        sel = "#cube-problems .problems-list"
        assert wait_until(page, "!!document.querySelector(%r) && document.querySelector(%r).style.maxHeight !== ''" % (sel, sel), 3), \
            "the list placed"
        b = page.evaluate(BOX, sel)
        assert inside(b), f"the list inside the window, 16 px from its edges ({b})"
        assert b["bottom"] <= b["vh"] - 15, f"as high as the room below it ({b})"
        assert b["scrolls"], "a long list scrolls in itself"
        assert b["flagRight"] <= b["right"] + 0.5, f"its badges inside it ({b})"
        box = page.locator(sel).bounding_box()
        page.mouse.move(box["x"] + box["width"] / 2, box["y"] + box["height"] / 2)
        for _ in range(12):
            page.mouse.wheel(0, 400)
        page.wait_for_timeout(300)
        state = page.evaluate("(sel) => ({ list: document.querySelector(sel).scrollTop, page: scrollY })", sel)
        assert state["list"] > 0, f"the wheel scrolls the list ({state})"
        assert state["page"] == 0, f"and not the page ({state})"
        # a read, as the poll's: the header's row fitted again, the list placed again (placeMenus)
        page.evaluate("loadCubes()")
        page.evaluate("() => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done)))")
        kept = page.evaluate("(sel) => document.querySelector(sel).scrollTop", sel)
        assert kept == state["list"], f"placed again, the list keeps its scroll: {state['list']}, then {kept}"
        page.keyboard.press("Escape")
    finally:
        page.evaluate(RESTORE)
        reads.clear(CUBES)
        page.evaluate("loadCubes()")


# A Cube found on the network, not set up: the found Cubes' icon in the pill.
FOUND = {"serial": "KEQ0000300", "kind": "cube", "configured": False, "address": "192.168.0.222", "rf_address": "0a0301", "firmware": "1.1.3",
         "dhcp": False, "found_at": "2026-11-14T08:00:00+01:00", "last_seen": "2026-11-14T08:00:00+01:00"}

# Where the pill's left edge is, and the row's fold (ui/src/header/fold.ts).
PILL_LEFT = "document.querySelector('#cube-problems .problems-sum').getBoundingClientRect().left"
FOLD = "document.querySelector('.top-line').dataset.fold"


def test_pile_list_follows_its_pill(page, case, reads):
    """The header's problem list, open, stays inside the window as its
    pill moves: problems come while it is open (a poll, Refresh now),
    the pill grows to the left, and at 390 px the row folds the pile onto
    a row of its own (ui/src/header/fold.ts); then they go, and the row
    unfolds. The list is placed again each time (components.js
    placeMenus), not left at the offset from the pill it was placed at."""
    sel = "#cube-problems .problems-list"
    page.locator("#cube-problems .problems-sum").click()
    assert wait_until(page, "document.querySelector(%r).style.maxHeight !== ''" % sel, 3), "the list placed"
    b = page.evaluate(BOX, sel)
    assert inside(b), f"the list inside the window ({b})"

    def come():
        reads.cube("KEQ0000100", problems=many_problems(3), connected=False)
        reads.found(FOUND)

    try:
        for what, tell in (("problems come", come), ("problems go", lambda: reads.clear(CUBES))):
            left = page.evaluate(PILL_LEFT)
            tell()
            page.evaluate("loadCubes()")
            assert wait_until(page, f"{PILL_LEFT} !== {left}", 5), f"{what}: the pill moves"
            assert wait_until(page, "(() => { const r = document.querySelector(%r).getBoundingClientRect();"
                                    " return r.left >= 15.5 && r.right <= document.documentElement.clientWidth - 15.5; })()" % sel, 3), \
                f"{what}: the list inside the window again ({page.evaluate(BOX, sel)}, the pill at {page.evaluate(PILL_LEFT)}, " \
                f"the row's fold {page.evaluate(FOLD)!r})"
            assert page.evaluate("document.querySelector('#cube-problems details').open"), f"{what}: the list still open"
        page.keyboard.press("Escape")
    finally:
        reads.clear(CUBES)
        page.evaluate("loadCubes()")


def test_menus_fit(page, case):
    """The Cube choice and ☰ open inside the window."""
    page.locator("#cube-choice").click()
    sel = "#cube-menu > .menu-items"
    assert wait_until(page, "document.querySelector(%r).getBoundingClientRect().width > 0" % sel, 3)
    page.wait_for_timeout(100)
    b = page.evaluate(BOX, sel)
    assert inside(b), f"the Cube choice inside the window ({b})"
    page.keyboard.press("Escape")
    page.locator("#nav-menu > summary").click()
    page.wait_for_timeout(100)
    b = page.evaluate(BOX, "#nav-menu > .menu-items")
    assert inside(b), f"☰ inside the window ({b})"
    page.keyboard.press("Escape")
