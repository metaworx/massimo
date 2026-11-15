# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Every Cube's problems beside the Cube choice (app.js
renderCubeProblems): a Cube with a problem as the heading, its devices
under it, the settings last for one who may change them; nothing where
nothing is wrong."""

# Sets the first Cube's link, devices with a problem and proxy failures
# as GET /api/v1/cubes would give them, and draws the header again.
SET = """([link, problems, failing]) => {
  const c = state.cubes[0];
  window.__saved = window.__saved || JSON.parse(JSON.stringify(state.cubes));
  c.link = link ? { state: link } : undefined;
  c.connected = !link || link === "connected";
  c.problems = problems;
  c.proxy_failing = failing;
  state.found = [];
  renderCubeSelect();
}"""

RESTORE = "() => { state.cubes = window.__saved || state.cubes; delete window.__saved; renderCubeSelect(); }"

# A radiator thermostat of the fixture's with a transmit error, as the
# Cubes' list gives a device with a problem.
FAULTY = {"rf_address": "0a0101", "type": "radiator_thermostat", "name": "Radiator", "room": "Kitchen",
          "serial": "MEQ0000101", "reported": True, "state": {"error": True, "battery_low": False, "link_error": False}}


def summary_icons(page):
    return [svg.evaluate("(s) => [...s.querySelectorAll('use')].map((u) => u.getAttribute('href')).join('+')")
            for svg in page.locator("#cube-problems .problems-sum svg").all()]


def test_header_problems(page, case):
    """No problem: the green check in the pill's place. A device's failure: the radio's cross in the
    pill; its list has the Cube as the heading in green, the device under
    it as "Room › Device" with its badge. A held link: the Cube's sign
    with a triangle and its badge. A proxy that does not listen: the
    settings last, the cog with a triangle. The Cube choice's own list
    keeps each Cube's sign."""
    try:
        page.evaluate(SET, [None, [], []])
        assert page.locator("#cube-problems .problems").count() == 0, "nothing wrong: no problem pill"
        assert page.locator("#cube-problems .ok-pill .ok-check").count() == 1, "the green check in its place"
        page.evaluate(SET, [None, [FAULTY], []])
        assert summary_icons(page) == ["#icon-radio-small+#mark-corner-x"], "the radio's cross"
        page.locator("#cube-problems .problems-sum").click()
        heads = page.locator("#cube-problems .problems-list > .problem-entry")
        assert heads.count() == 1, "one Cube"
        cube = heads.first
        assert "tone-ok" in cube.locator(":scope > .problem-icon").get_attribute("class"), "the Cube's own sign green"
        sub = cube.locator(".problem-subs .problem-entry")
        assert sub.locator(".problem-name").inner_text() == "Kitchen › Radiator", "the device under it, Room › Device"
        assert sub.locator(".flag.bad").count() == 1, "with its badge"
        page.keyboard.press("Escape")
        page.evaluate(SET, ["held", [FAULTY], [{"addr": "0.0.0.0:62910", "error": "in use"}]])
        icons = summary_icons(page)
        assert icons[0] == "#icon-net-base+#mark-net-warn" and icons[-1] == "#icon-cog-small+#mark-corner-warn", \
            f"the held link first, the settings last ({icons})"
        page.locator("#cube-problems .problems-sum").click()
        heads = page.locator("#cube-problems .problems-list > .problem-entry")
        assert heads.count() == 2, "the Cube, then the settings"
        assert heads.first.locator(":scope > .problem-flags .flag.warn").count() == 1, "the link's badge"
        assert "0.0.0.0:62910" in heads.last.inner_text(), "the proxy that does not listen"
        box = page.locator("#cube-problems .problems-list").bounding_box()
        assert box["x"] >= 0 and box["x"] + box["width"] <= case.width, f"the list within the page ({box})"
        page.keyboard.press("Escape")
        page.locator("#cube-choice").click()
        assert page.locator("#cube-menu .menu-items .conn-sign").count() >= 1, "the choice's list keeps each Cube's sign"
        page.keyboard.press("Escape")
    finally:
        page.evaluate(RESTORE)
