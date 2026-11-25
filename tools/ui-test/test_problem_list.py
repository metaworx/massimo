# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The problem list, the same at every level (ui/src/shared/problems.tsx
Problems, as the header's pile draws every Cube's): a heading whose own
sign is fine is listed for the problems under it, its badges stand under
its name, and the entries under it are indented to the name, a line down
from its icon. The Cubes are the page's data, a signal (UiParts D123-A):
the test tells them to the page's reads of the Cubes (reads.change) and
has the page load them, so that its poll keeps them (docs/ui-testing.md,
section 9; AP ToldData, Block 4), and the test's end puts the server's
back."""
from datetime import datetime, timedelta, timezone

from helpers import CUBES, configured_only, wait_until


def dev(name, **st):
    """A device of a Cube's problems, the fields of its state given in st."""
    return {"rf_address": "0a0f0%d" % len(name), "type": "radiator_thermostat", "name": name, "room": "Bath",
            "serial": "MEQ000010%d" % len(name), "reported": True,
            "state": dict({"error": False, "battery_low": False, "link_error": False}, **st)}


def three_cubes(cubes):
    """The Cubes as AP CubeBadges lists them, in place of the server's: a
    Cube with its own warning (its connection held) and, under it, a
    device of its with a failure; a Cube that is fine, its device with a
    warning; a Cube without any problem, not listed. The first is the
    page's own Cube, so that the Cube chosen stays one of the list."""
    own = configured_only(cubes)[0]
    until = (datetime.now(timezone.utc) + timedelta(hours=1)).strftime("%Y-%m-%dT%H:%M:%S.000Z")
    return [
        dict(own, name="Cube held", connected=False, link={"state": "held", "until": until}, problems=[dev("Radiator one", error=True)]),
        dict(own, serial="KEQ0000200", name="Cube fine", problems=[dev("Radiator two", link_error=True)]),
        dict(own, serial="KEQ0000300", name="Cube quiet", problems=[]),
    ]

BOXES = """() => {
  const b = (e) => e.getBoundingClientRect();
  const list = document.querySelector("#cube-problems .problems-list");
  const entry = (title) => [...list.children].find((e) => e.querySelector(":scope > .problem-who .problem-name")?.textContent === title);
  return ["Cube held", "Cube fine", "Cube quiet"].map((title) => {
    const e = entry(title);
    if (!e) return null;
    return {
      icon: b(e.querySelector(":scope > .problem-icon")),
      name: b(e.querySelector(":scope > .problem-who .problem-name")),
      flags: e.querySelector(":scope > .problem-flags") ? b(e.querySelector(":scope > .problem-flags")) : null,
      sub: e.querySelector(":scope > .problem-subs .problem-entry .problem-icon")
        ? b(e.querySelector(":scope > .problem-subs .problem-entry .problem-icon")) : null,
      subs: e.querySelectorAll(":scope > .problem-subs .problem-entry").length,
      line: getComputedStyle(e.querySelector(":scope > .problem-subs") || e).borderLeftStyle,
      lineX: e.querySelector(":scope > .problem-subs") ? b(e.querySelector(":scope > .problem-subs")).left : null,
    };
  });
}"""


def test_one_entry_format(page, case, reads):
    """A Cube listed with its own warning and its device under it; a Cube
    whose own sign is fine listed for its device's warning alone, without
    badges of its own; a Cube without problems not listed. At every level
    the badges stand under the name, the entries under an entry start at
    its name, and a line runs down from its icon's middle."""
    reads.change(CUBES, three_cubes)
    try:
        page.evaluate("loadCubes()")
        assert wait_until(page, "state.cubes.map((c) => c.name).join() === 'Cube held,Cube fine,Cube quiet'", 5), "the page has the three Cubes"
        page.locator("#cube-problems .problems-sum").click()
        page.locator("#cube-problems .problems-list").wait_for(state="visible")
        held, fine, quiet = page.evaluate(BOXES)
        assert held and fine and quiet is None, f"two Cubes listed, the quiet one not ({held}, {fine}, {quiet})"
        assert held["flags"] and abs(held["flags"]["left"] - held["name"]["left"]) <= 1, "the badges under the name, not the icon"
        assert fine["flags"] is None, "the fine Cube listed for its device alone, without badges of its own"
        for e in (held, fine):
            assert e["subs"] == 1, f"a device under each Cube ({e})"
            assert abs(e["sub"]["left"] - e["name"]["left"]) <= 1, f"the device's icon under the Cube's name ({e})"
            assert e["line"] == "solid", "a line down from the Cube's icon"
            middle = e["icon"]["left"] + e["icon"]["width"] / 2
            assert abs(e["lineX"] + 1 - middle) <= 1.5, f"from the icon's middle ({e['lineX']} vs {middle})"
        assert page.evaluate("document.documentElement.scrollWidth") <= case.width, "within the page's width"
    finally:
        page.keyboard.press("Escape")
        reads.clear(CUBES)
        page.evaluate("loadCubes()")
