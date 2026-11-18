# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A Cube's panel follows the data in place (AP UiParts, Block 5): a
Cube's problem list stays open through a poll, its panel closed or open,
in System settings and on Cube settings (the fault reported); Reboot's
question names the radiograms pending now, not those of when the panel
was drawn; and a line of the settings file that comes to differ while
the panel is shown gets its row with Take the file's value (D126). The
Cubes' list is the server's with what the test puts in (the fixture
reads); nothing is sent to the fake Cube."""
import json

import pytest

from helpers import CUBES, show_view, wait_until

PANEL = ".cube-panel[data-serial='KEQ0000100']"

# A radiator thermostat with a low battery, as the Cubes' list gives a
# device with a problem: the Cube's pill has its list from the start.
LOW = {"rf_address": "0a0101", "type": "radiator_thermostat", "name": "Radiator", "room": "Kitchen",
       "serial": "MEQ0000101", "reported": True, "state": {"error": False, "battery_low": True, "link_error": False}}

# A line of the settings file that differs from the store, as the Cubes'
# list gives it.
DIFFERS = {"field": "name", "variable": "MASSIMO_CUBE_KEQ0000100_NAME", "file": "File name", "store": "Fake Cube"}

PENDING = {"en": "that 7 radiograms are not delivered yet", "de": "dass 7 Funktelegramme noch nicht zugestellt sind"}


def loaded(page):
    """A load of the Cubes' list, as the poll makes it."""
    page.evaluate("loadCubes()")
    page.wait_for_timeout(200)


def mark(page, selector):
    """Marks the element of selector, so that a later look can tell
    whether it is still the same element."""
    page.evaluate("(s) => { document.querySelector(s).__mark = true; }", selector)


def same_and_open(page, selector):
    return page.evaluate("(s) => { const d = document.querySelector(s); return !!d && !!d.__mark && d.open; }", selector)


@pytest.mark.parametrize("where", ["closed", "open", "page"])
def test_problem_list_stays_open_through_a_poll(page, case, where, reads):
    """The Cube's pill opened, a poll comes: the list is the same
    element and stays open, with its panel closed or open in System
    settings, and on Cube settings."""
    reads.cube("KEQ0000100", problems=[LOW])
    try:
        if where == "page":
            show_view(page, "cube")
            panel = "#cube-page-body " + PANEL
        else:
            show_view(page, "system")
            panel = "#cubes-list " + PANEL
        loaded(page)
        pill = panel + " > summary .cube-pill details.problems"
        assert wait_until(page, "!!document.querySelector(%s)" % json.dumps(pill), 5), "the Cube's pill with its list"
        if where == "open":
            page.locator(panel + " > summary .cube-head-who").click()
            page.locator(panel + " .cube-form").wait_for(state="visible")
        page.locator(pill + " > summary").click()
        assert page.locator(pill).get_attribute("open") is not None, "the list open"
        mark(page, pill)
        page.evaluate("poll(false)")
        page.wait_for_timeout(300)
        assert same_and_open(page, pill), f"the list stays open through a poll, the panel {where}"
        loaded(page)
        assert same_and_open(page, pill), f"and through a load of the Cubes, the panel {where}"
    finally:
        reads.clear(CUBES)
    page.keyboard.press("Escape")
    loaded(page)
    show_view(page, "tiles")


@pytest.mark.parametrize("where", ["system", "page"])
def test_reboot_names_the_radiograms_pending_now(page, case, where, reads):
    """The panel drawn with nothing pending, then the Cube says 7 are:
    Reboot's question names the 7, and Yes sends the count it names (the
    reboot stubbed)."""
    reads.cube("KEQ0000100", pending_radiograms=0)
    sent = []

    def reboot(route):
        sent.append(route.request.post_data_json)
        route.fulfill(status=202, content_type="application/json", body='{"sent": true, "answered": true, "status": 2}')
    page.route("**/api/v1/cubes/KEQ0000100/reboot", reboot)
    try:
        if where == "page":
            show_view(page, "cube")
            panel = page.locator("#cube-page-body " + PANEL)
        else:
            show_view(page, "system")
            panel = page.locator("#cubes-list " + PANEL)
            panel.locator(":scope > summary .cube-head-who").click()
        panel.locator(".cube-reboot-btn").wait_for(state="visible")
        loaded(page)
        reads.cube("KEQ0000100", pending_radiograms=7, budget={"duty_cycle_pct": 0, "free_memory_slots": 43,
                                                               "updated_at": "2026-11-13T21:40:00+01:00"})
        loaded(page)
        assert wait_until(page, "state.cubes[0].pending_radiograms === 7", 5), "the Cube says 7 pending"
        panel.locator(".cube-reboot-btn").click()
        text = panel.locator(".confirm-box .confirm-text").inner_text()
        assert PENDING[case.lang] in text, f"the question names the radiograms pending now ({text!r})"
        panel.locator(".confirm-box .confirm-yes").click()
        assert wait_until(page, "!!document.querySelector(%s)" % json.dumps(PANEL + " .cube-reboot-btn"), 5), "the button back"
        assert sent == [{"pending": 7}], f"the reboot with the count named ({sent!r})"
    finally:
        reads.clear(CUBES)
        page.unroute("**/api/v1/cubes/KEQ0000100/reboot")
    loaded(page)
    show_view(page, "tiles")


@pytest.mark.parametrize("where", ["system", "page"])
def test_a_file_difference_that_comes_later_gets_its_row(page, case, where, reads):
    """The panel shown without a difference, then the settings file's
    name comes to differ: the panel gets its row with Take the file's
    value, beside its badge, and loses it once the difference goes."""
    reads.cube("KEQ0000100")
    try:
        if where == "page":
            show_view(page, "cube")
            panel = page.locator("#cube-page-body " + PANEL)
        else:
            show_view(page, "system")
            panel = page.locator("#cubes-list " + PANEL)
            panel.locator(":scope > summary .cube-head-who").click()
        panel.locator(".cube-form").wait_for(state="visible")
        loaded(page)
        assert panel.locator(".cube-differs").count() == 0, "no difference at first"
        reads.cube("KEQ0000100", file_differs=[DIFFERS])
        loaded(page)
        assert wait_until(page, "state.cubes[0].file_differs && state.cubes[0].file_differs.length === 1", 5), "the difference in the data"
        row = panel.locator(".cube-differs")
        assert wait_until(page, "!!document.querySelector(\"%s .cube-differs .take-file[data-field=name]\")" % PANEL, 5), \
            "the row of the difference, with Take the file's value"
        assert "MASSIMO_CUBE_KEQ0000100_NAME" in row.inner_text() and "File name" in row.inner_text(), "naming the line and its value"
        reads.cube("KEQ0000100")
        loaded(page)
        assert wait_until(page, "!document.querySelector(\"%s .cube-differs\")" % PANEL, 5), "the row goes with the difference"
    finally:
        reads.clear(CUBES)
    loaded(page)
    show_view(page, "tiles")
