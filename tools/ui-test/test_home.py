# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Rooms without a connected Cube: the notice above the rooms, against
run.sh's second server, whose Cube sits at a closed port, and with the
Cube list stubbed for no Cube and for a Cube that dropped after its rooms
loaded."""
import json
import os

import pytest

from conftest import TOKEN
from helpers import wait_until

DOWN_URL = os.environ.get("UI_TEST_DOWN_URL", "")
NOT_CONNECTED = {"en": "Cube Cellar Cube (KEQ0000200) is not connected.", "de": "Cube Cellar Cube (KEQ0000200) ist nicht verbunden."}
NO_CUBE = {"en": "No Cube is configured.", "de": "Es ist kein Cube eingerichtet."}
NO_DATA = {"en": "No data has come from it", "de": "keine Daten von ihm"}
LAST_DATA = {"en": "Its last data came at", "de": "Seine letzten Daten kamen um"}

NOTICE_SHOWN = "!document.getElementById('home-notice').classList.contains('hidden')"


def notice_text(page, part):
    return page.locator("#home-notice .home-notice-" + part).inner_text()


def test_a_cube_not_connected_says_why_rooms_are_empty(context, case, watch, guard):
    """On the second server Rooms says that its Cube is not connected,
    the address the server tries, that no data has come, and what to
    check; its button opens System settings."""
    if not DOWN_URL:
        pytest.skip("run.sh starts the second server")
    if TOKEN:
        context.add_init_script(f"localStorage.setItem('massimo.token', {TOKEN!r})")
    page = context.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(DOWN_URL)
    assert wait_until(page, NOTICE_SHOWN, 10), "the notice shows"
    assert notice_text(page, "title") == NOT_CONNECTED[case.lang], "it names the Cube"
    detail = notice_text(page, "detail")
    assert "127.0.0.1:9" in detail and NO_DATA[case.lang] in detail, f"the address, and no data yet: {detail}"
    assert notice_text(page, "check"), "what to check"
    assert page.locator("#rooms .room").count() == 0, "no rooms"
    page.locator("#home-notice-cubes").click()
    page.locator("#system .cubes-settings").wait_for(state="visible")
    assert page.locator("#home-notice").is_hidden(), "the notice belongs to Rooms"


def test_no_cube_configured(page, case):
    """With no Cube in the list (stubbed), Rooms says so, and for the
    owner where to add one; the notice goes once the Cube is back."""
    page.route("**/api/v1/cubes", lambda route: route.fulfill(status=200, content_type="application/json", body="[]"))
    try:
        page.evaluate("loadCubes()")
        assert wait_until(page, NOTICE_SHOWN, 5), "the notice shows"
        assert notice_text(page, "title") == NO_CUBE[case.lang], "no Cube"
        assert page.locator("#home-notice-cubes").is_visible(), "the way to System settings → Cubes"
    finally:
        page.unroute("**/api/v1/cubes")
    page.evaluate("loadCubes()")
    assert wait_until(page, "document.getElementById('home-notice').classList.contains('hidden')", 5), "gone with the Cube back"


def test_a_cube_that_dropped_keeps_its_rooms_under_the_notice(page, case):
    """A Cube that dropped after its rooms loaded (the list stubbed with
    connected false): the notice above the rooms, which stay, with when
    its last data came."""
    def dropped(route):
        cubes = route.fetch().json()
        for c in cubes:
            c["connected"] = False
        route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes))

    page.route("**/api/v1/cubes", dropped)
    try:
        page.evaluate("loadCubes()")
        assert wait_until(page, NOTICE_SHOWN, 5), "the notice shows"
        assert "KEQ0000100" in notice_text(page, "title"), "it names the Cube"
        assert LAST_DATA[case.lang] in notice_text(page, "detail"), "with when its last data came"
        notice = page.locator("#home-notice").bounding_box()
        first = page.locator(".room").first
        assert first.is_visible() and first.bounding_box()["y"] > notice["y"], "the rooms stay, under the notice"
    finally:
        page.unroute("**/api/v1/cubes")
    page.evaluate("loadCubes()")
    assert wait_until(page, "document.getElementById('home-notice').classList.contains('hidden')", 5), "gone once connected"
