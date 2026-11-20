# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Every Cube's problems beside the Cube choice (header.js
renderCubeProblems): a Cube with a problem as the heading, its devices
under it, the settings last for one who may change them; nothing where
nothing is wrong."""
from helpers import CUBES, configured_only, reads_of, wait_until


def told(page, link, problems, failing):
    """The fake Cube's link, devices with a problem and proxy failures in
    GET /api/v1/cubes, without the Cubes found, at every read of the
    page's from now on (helpers.Reads), and the list read."""
    reads = reads_of(page)
    reads.change(CUBES, configured_only)
    reads.cube("KEQ0000100", link={"state": link} if link else None, connected=not link or link == "connected",
               problems=problems, proxy_failing=failing)
    page.evaluate("loadCubes()")


def restored(page):
    """The server's own list again."""
    reads_of(page).clear(CUBES)
    page.evaluate("loadCubes()")


# A radiator thermostat of the fixture's with a transmit error, as the
# Cubes' list gives a device with a problem.
FAULTY = {"rf_address": "0a0101", "type": "radiator_thermostat", "name": "Radiator", "room": "Kitchen",
          "serial": "MEQ0000101", "reported": True, "state": {"error": True, "battery_low": False, "link_error": False}}


def summary_icons(page):
    """The pill's icons, the server's sign first."""
    return [svg.evaluate("(s) => [...s.querySelectorAll('use')].map((u) => u.getAttribute('href')).join('+')")
            for svg in page.locator("#cube-problems .problems-sum svg").all()]


def test_header_problems(page, case):
    """No problem: the green check after the server's sign, the list there
    all the same, saying nothing is wrong. A device's failure: the radio's cross in the
    pill; its list has the Cube as the heading in green, the device under
    it as "Room › Device" with its badge. A held link: the Cube's sign
    with a triangle and its badge. A proxy that does not listen: the
    settings last, the cog with a triangle. The Cube choice's own list
    keeps each Cube's sign."""
    try:
        told(page, None, [], [])
        assert summary_icons(page)[1:] == ["#icon-check"], f"nothing wrong: the green check after the sign ({summary_icons(page)})"
        page.locator("#cube-problems .problems-sum").click()
        assert page.locator("#cube-problems .problems-ok").inner_text() == page.evaluate("t('problem.none')"), "the list says so"
        page.keyboard.press("Escape")
        told(page, None, [FAULTY], [])
        assert summary_icons(page)[1:] == ["#icon-radio-small+#mark-corner-x"], "the radio's cross"
        page.locator("#cube-problems .problems-sum").click()
        heads = page.locator("#cube-problems .problems-list > .problem-entry")
        assert heads.count() == 1, "one Cube"
        cube = heads.first
        assert cube.locator(":scope > .problem-icon use[href='#icon-cube3d']").count() == 1, "the cube before the Cube's name"
        assert "tone-ok" in cube.locator(":scope > .problem-who .problem-tail .conn-sign").get_attribute("class"), \
            "the Cube's own sign after its name, green"
        sub = cube.locator(".problem-subs .problem-entry")
        assert sub.locator(".problem-name").inner_text() == "Kitchen › Radiator", "the device under it, Room › Device"
        assert sub.locator(".flag.bad").count() == 1, "with its badge"
        page.keyboard.press("Escape")
        told(page, "held", [FAULTY], [{"addr": "0.0.0.0:62910", "error": "in use"}])
        icons = summary_icons(page)[1:]
        assert icons[0] == "#icon-net-base+#mark-net-warn" and icons[-1] == "#icon-cog-small+#mark-corner-warn", \
            f"the held link first, the settings last ({icons})"
        page.locator("#cube-problems .problems-sum").click()
        heads = page.locator("#cube-problems .problems-list > .problem-entry")
        assert heads.count() == 2, "the Cube, then the settings"
        assert heads.first.locator(":scope > .problem-flags .flag.warn").count() == 1, "the link's badge"
        assert "0.0.0.0:62910" in heads.last.inner_text(), "the proxy that does not listen"
        # placed as it opens (components.js dropdown, placeMenu), and again as its pill moves when the
        # problems told change the row (ui/src/header/fold.ts): waited for, not read at once
        within = ("(() => { const r = document.querySelector('#cube-problems .problems-list').getBoundingClientRect();"
                  " return r.left >= 0 && r.right <= document.documentElement.clientWidth; })()")
        assert wait_until(page, within, 3), f"the list within the page ({page.locator('#cube-problems .problems-list').bounding_box()})"
        page.keyboard.press("Escape")
        page.locator("#cube-choice").click()
        assert page.locator("#cube-menu .menu-items .conn-sign").count() >= 1, "the choice's list keeps each Cube's sign"
        page.keyboard.press("Escape")
    finally:
        restored(page)
