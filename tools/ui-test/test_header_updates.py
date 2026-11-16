# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The header follows the data in place (AP UiParts, Block 6): every
Cube's problem list, opened beside the Cube choice, stays open through a
poll and lists what the poll brought (the fault: it was left as it was
while open, and stopped following the data); and the Cube choice, open,
stays open with the focus where it was and shows what the poll brought.
The Cubes' list is stubbed; nothing is sent to the fake Cube."""
import json

from helpers import wait_until

# A radiator thermostat with a low battery, as the Cubes' list gives a
# device with a problem: the header's pill has its list from the start.
LOW = {"rf_address": "0a0101", "type": "radiator_thermostat", "name": "Radiator", "room": "Kitchen",
       "serial": "MEQ0000101", "reported": True, "state": {"error": False, "battery_low": True, "link_error": False}}

# A second device with a problem, which a later poll brings: a wall
# thermostat whose radio link is disturbed.
LINK = {"rf_address": "0a0102", "type": "wall_thermostat", "name": "Wall", "room": "Hall",
        "serial": "MEQ0000102", "reported": True, "state": {"error": False, "battery_low": False, "link_error": True}}

PILL = "#cube-problems details.problems"
ITEM = "#cube-menu .menu-items button[data-value='KEQ0000100']"


class Cubes:
    """GET /api/v1/cubes as the server answers it, with the fake Cube's
    entry changed by what change holds (a dict merged into it), which a
    test may change between two loads."""

    def __init__(self, page, **change):
        self.change = dict(change)
        page.route("**/api/v1/cubes", self.handle)

    def handle(self, route):
        try:
            cubes = route.fetch().json()
            for c in cubes:
                if c.get("serial") == "KEQ0000100":
                    c.update(self.change)
            route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes))
        except Exception:  # the route is already handled, or the page closed
            pass


def held(page):
    """The poll's timer stopped, so that only the test's loads come."""
    page.evaluate("clearInterval(state.pollTimer); state.pollTimer = null")


def loaded(page):
    """A load of the Cubes' list, as the poll makes it."""
    page.evaluate("loadCubes()")
    page.wait_for_timeout(300)


def mark(page, selector):
    """Marks the element of selector, so that a later look can tell
    whether it is still the same element."""
    page.evaluate("(s) => { document.querySelector(s).__mark = true; }", selector)


def same_and_open(page, selector):
    return page.evaluate("(s) => { const d = document.querySelector(s); return !!d && !!d.__mark && d.open; }", selector)


def test_problem_list_follows_while_open(page, case):
    """The header's pill opened on a Cube's device with a low battery, a
    poll brings a second device with a problem: the list is the same
    element, stays open, and lists the second device."""
    held(page)
    stub = Cubes(page, problems=[LOW])
    try:
        loaded(page)
        assert wait_until(page, "!!document.querySelector(%s)" % json.dumps(PILL), 5), "the header's pill with its list"
        page.locator(PILL + " > summary").click()
        page.locator(PILL + " .problems-list").wait_for(state="visible")
        assert "Kitchen › Radiator" in page.locator(PILL + " .problems-list").inner_text(), "the first device listed"
        mark(page, PILL)
        stub.change["problems"] = [LOW, LINK]
        loaded(page)
        assert wait_until(page, "document.querySelector(%s).innerText.includes('Hall › Wall')" % json.dumps(PILL + " .problems-list"), 3), \
            "the open list follows the poll: the second device is listed"
        assert same_and_open(page, PILL), "the list is the same element and still open"
        page.keyboard.press("Escape")
    finally:
        page.unroute("**/api/v1/cubes")
        page.evaluate("startPolling(); poll(true)")


def test_cube_choice_follows_while_open(page, case):
    """The Cube choice opened, the focus on the chosen Cube, a poll
    brings the Cube's new name and a settings file that differs: the menu
    is the same element, stays open, the focus stays on the Cube's
    entry, and the entry shows the name and the settings' cog."""
    held(page)
    stub = Cubes(page)
    try:
        loaded(page)
        page.locator("#cube-choice").click()
        page.locator(ITEM).wait_for(state="visible")
        assert wait_until(page, "document.activeElement === document.querySelector(%s)" % json.dumps(ITEM), 3), \
            "the focus on the chosen Cube"
        mark(page, "#cube-menu")
        page.evaluate("(s) => { document.querySelector(s).__mark = true; }", ITEM)
        stub.change.update(name="Renamed Cube", file_differs=[{"field": "name", "variable": "MASSIMO_CUBE_KEQ0000100_NAME",
                                                               "file": "File name", "store": "Renamed Cube"}])
        loaded(page)
        assert wait_until(page, "document.querySelector(%s).innerText.includes('Renamed Cube')" % json.dumps(ITEM), 3), \
            "the open choice follows the poll: the Cube's new name"
        hrefs = page.locator(ITEM + " .menu-item-icons use").evaluate_all("(es) => es.map((e) => e.getAttribute('href'))")
        assert any("cog" in h for h in hrefs), f"the settings' cog after the name: {hrefs}"
        assert same_and_open(page, "#cube-menu"), "the menu is the same element and still open"
        assert page.evaluate("(s) => { const b = document.querySelector(s); return !!b.__mark && document.activeElement === b; }", ITEM), \
            "the focus stays on the Cube's entry, the same element"
        page.keyboard.press("Escape")
    finally:
        page.unroute("**/api/v1/cubes")
        page.evaluate("startPolling(); poll(true)")
