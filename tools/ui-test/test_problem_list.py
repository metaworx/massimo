# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The problem list, the same at every level (ui/src/shared/problems.tsx
Problems, as the header's pile draws every Cube's): a heading whose own
sign is fine is listed for the problems under it, its badges stand under
its name, and the entries under it are indented to the name, a line down
from its icon. The Cubes are the page's data, a signal (UiParts D123-A):
the test writes them, the poll held, and the next load puts the server's
back."""
from helpers import held_poll

# The Cubes as AP CubeBadges lists them: a Cube with its own warning (its
# connection held) and, under it, a device of its with a failure; a Cube
# that is fine, its device with a warning; a Cube without any problem,
# not listed. The first is the page's own Cube, so that the Cube chosen
# stays one of the list.
CUBES = """() => {
  const dev = (name, state) => ({ rf_address: "0a0f0" + name.length, type: "radiator_thermostat", name, room: "Bath",
    serial: "MEQ000010" + name.length, reported: true, state: { error: false, battery_low: false, link_error: false, ...state } });
  const own = state.cubes[0];
  state.cubes = [
    { ...own, name: "Cube held", connected: false, link: { state: "held", until: new Date(Date.now() + 3600000).toISOString() },
      problems: [dev("Radiator one", { error: true })] },
    { ...own, serial: "KEQ0000200", name: "Cube fine", problems: [dev("Radiator two", { link_error: true })] },
    { ...own, serial: "KEQ0000300", name: "Cube quiet", problems: [] },
  ];
}"""

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


def test_one_entry_format(page, case):
    """A Cube listed with its own warning and its device under it; a Cube
    whose own sign is fine listed for its device's warning alone, without
    badges of its own; a Cube without problems not listed. At every level
    the badges stand under the name, the entries under an entry start at
    its name, and a line runs down from its icon's middle."""
    with held_poll(page):
        page.evaluate(CUBES)
        try:
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
            page.evaluate("loadCubes()")
