# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Menus kept in the window (components.js dropdown, placeMenu): the
header's problem list, long, opens inside the window at every width, as
high as the room below it, and scrolls in itself without the page; its
badges stay inside it. The Cube choice and ☰ open inside the window too."""
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
    page's (helpers.Reads)."""
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
        page.keyboard.press("Escape")
    finally:
        page.evaluate(RESTORE)
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
