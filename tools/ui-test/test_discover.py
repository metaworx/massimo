# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The Cube choice's search for new Cubes. The fake Cube answers
discovery on a loopback port as a Cube that is not configured,
KEQ0000000; the server's own discovery socket is UDP 23272, so where
another program holds that port the search answers 503 and the test of
the answer is skipped."""
import time

import pytest

from helpers import FOUND2, SEARCHING, found_text, scroll_width, show_view, wait_until


def test_search_held_then_answered(page, case):
    """The search shows "searching" while the request is held, then the
    found Cube as not selectable with its DHCP flag and the env lines to
    add, and the log has its cube_found."""
    sel = page.locator("#cube-select")
    assert sel.locator("option[value=__discover]").count() == 1, "search entry in the Cube choice"
    serial = page.evaluate("state.serial")
    held = []
    page.route("**/api/v1/cubes/discover", lambda route: held.append(route))
    sel.select_option("__discover")
    end = time.time() + 5
    while not held and time.time() < end:
        page.wait_for_timeout(100)
    assert page.locator("#cube-search").inner_text() == SEARCHING[case.lang], "searching shown"
    assert sel.locator("option[value=__discover]").is_disabled(), "search entry disabled while searching"
    # Released before the route goes: a page without routes stops
    # intercepting, and Playwright would let the held request through.
    if held:
        held[0].continue_()  # on to the server, which asks the fake Cube
    page.unroute("**/api/v1/cubes/discover")
    assert wait_until(page, "!discovery.running", 10), "search done"
    if "bad" in (page.locator("#cube-search").get_attribute("class") or ""):
        pytest.skip("the server has no discovery socket (UDP 23272 in use?): " + page.locator("#cube-search").inner_text())
    assert page.locator("#cube-search").inner_text() == found_text(case.lang, 1), "search result shown"
    opt = sel.locator("option[value='found:KEQ0000000']")
    assert opt.count() == 1 and opt.is_disabled(), "found Cube in the choice, not selectable"
    assert sel.input_value() == serial, "the selected Cube stays"
    assert page.locator("#cube-found").get_attribute("open") is not None, "found Cubes opened"
    assert "MASSIMO_CUBE_KEQ0000000_ADDRESS=127.0.0.1" in page.locator("#cube-found .env-lines").inner_text(), "env lines for the found Cube"
    facts = page.locator("#cube-found [data-serial='KEQ0000000'] .found-facts")
    assert facts.count() == 1 and " · DHCP · " in facts.inner_text(), "the found Cube's DHCP flag"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the found Cubes"
    show_view(page, "log")
    assert wait_until(page, "[...document.querySelectorAll('.log-line.kind-cube_found')].some((l) => l.textContent.includes('KEQ0000000'))", 5), \
        "cube_found in the log"
    show_view(page, "tiles")


def test_cube_found_live(page, case):
    """A cube_found event adds another found Cube live."""
    sel = page.locator("#cube-select")
    before = page.evaluate("state.found.length")
    page.evaluate("(c) => state.events.dispatchEvent(new MessageEvent('cube_found', { data: JSON.stringify("
                  "{ kind: 'cube_found', cube: c, time: c.found_at }) }))", FOUND2)
    assert sel.locator("option[value='found:LEQ0000506']").count() == 1, "cube_found adds a Cube live"
    assert page.locator("#cube-found summary").inner_text() == found_text(case.lang, before + 1), "found Cubes counted"


def test_refused_search(page):
    """A refused search (a stubbed 503) shows the server's message."""
    sel = page.locator("#cube-select")
    page.route("**/api/v1/cubes/discover", lambda route: route.fulfill(
        status=503, content_type="application/json", body='{"error": "no discovery socket"}'))
    sel.select_option("__discover")
    assert wait_until(page, "document.getElementById('cube-search').classList.contains('bad')", 5) \
        and "no discovery socket" in page.locator("#cube-search").inner_text(), "a refused search shows the server's message"
    page.unroute("**/api/v1/cubes/discover")
