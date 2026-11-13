# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The Send buttons of the Rooms page: a write of a tile, of the House
panel or of a room's configuration keeps its buttons off, the one sent
turning, until the answer, so that a double click sends one radio
command; a configuration form's Send is off while nothing differs from
what the Cube has.

Every write is held at its route and then answered with a stub, so
nothing reaches the fake Cube and its state stays as the other tests
expect it."""
from helpers import assert_busy, busy_state, hold, wait_held, wait_until

TILE = ".room:has(.mode-auto:not(.hidden))"
OK = '{"ok": true, "devices": []}'


def answer(held, body="{}"):
    """Answers the held writes with a stub."""
    for route in held:
        route.fulfill(status=200, content_type="application/json", body=body)
    held.clear()


def first_tile(page):
    """The first tile with a thermostat; its selector for page.evaluate."""
    tile = page.locator(TILE).first
    tile.wait_for()
    rid = tile.get_attribute("data-id")
    return tile, '.room[data-id="%s"]' % rid


def test_a_mode_click_sends_once(page, case):
    """AUTO turns from its click until the answer, the tile's other write
    buttons are off meanwhile, and a second click sends nothing; once
    the answer is in, the buttons are back."""
    tile, sel = first_tile(page)
    held, sent = [], []
    hold(page, "**/rooms/*/setpoint", "PUT", held, sent)
    try:
        tile.locator(".mode-auto").click()
        assert wait_held(page, held), "the setpoint PUT is sent"
        assert_busy(page, sel + " .mode-auto", "AUTO")
        assert busy_state(page, sel + " .mode-manual")[:2] == [True, False], "MANU is off, without a spinner"
        assert busy_state(page, sel + " .step-plus")[:2] == [True, False], "the stepper is off"
        tile.locator(".mode-auto").click(force=True)
        page.wait_for_timeout(300)
        assert len(sent) == 1, f"one PUT for two clicks: {sent}"
        answer(held)
        assert wait_until(page, "!document.querySelector(%r).disabled" % (sel + " .mode-auto"), 5), "AUTO is back once answered"
        assert busy_state(page, sel + " .mode-auto") == [False, False, None], "AUTO stops turning"
        assert not page.locator(sel + " .mode-manual").is_disabled(), "MANU is back"
    finally:
        page.unroute("**/rooms/*/setpoint")


def test_a_stepper_value_stays_with_save_turning(page, case):
    """Save of a stepper value turns until the answer, the value shown
    meanwhile, and Cancel and the stepper are off; then the value goes."""
    tile, sel = first_tile(page)
    held, sent = [], []
    hold(page, "**/rooms/*/setpoint", "PUT", held, sent)
    try:
        tile.locator(".step-plus").click()
        tile.locator(".pending-save").click()
        assert wait_held(page, held), "the setpoint PUT is sent"
        assert_busy(page, sel + " .pending-save", "Save")
        assert tile.locator(".pending-row").is_visible(), "the value waits, shown, until the answer"
        assert tile.locator(".pending-cancel").is_disabled() and tile.locator(".step-minus").is_disabled(), "Cancel and the stepper are off"
        assert sent[0]["mode"] == "manual", f"the stepper sends manual: {sent}"
        answer(held)
        assert wait_until(page, "document.querySelector(%r).classList.contains('hidden')" % (sel + " .pending-row"), 5), \
            "the value goes once answered"
        page.wait_for_timeout(2500)  # the stepper's own timer would have sent by now
        assert len(sent) == 1, f"one PUT: {sent}"
    finally:
        page.unroute("**/rooms/*/setpoint")


def open_tab(tile, tab):
    """The tile's configuration, on a tab."""
    if not tile.locator(".config-body").count():
        tile.locator(".tile-cog").click()
    tile.locator(".config-body").wait_for()
    tile.locator(".config-tabs [role=tab][data-tab=%s]" % tab).click()
    tile.locator(".config-body form").first.wait_for()


def test_configuration_send_is_off_until_a_change(page, case):
    """The Room tab's and the Temperatures tab's Send are off while
    nothing differs; a keystroke turns Send on, a value put back turns
    it off; a held Send turns, a second click sends nothing, and once
    the Cube took the value nothing differs again."""
    tile, sel = first_tile(page)
    send = tile.locator(".config-body form button[type=submit]").first
    open_tab(tile, "room")
    assert send.is_disabled(), "the Room tab's Send is off with the name as it is"
    name = tile.locator(".config-body input[type=text]").first
    was_name = name.input_value()
    name.fill(was_name + "x")
    assert send.is_enabled(), "a changed name turns Send on"
    name.fill(was_name)
    assert send.is_disabled(), "the name put back turns Send off"
    open_tab(tile, "temps")
    assert send.is_disabled(), "the Temperatures tab's Send is off while nothing differs"
    comfort = tile.locator(".config-body input[type=number]").first
    was = comfort.input_value()
    other = "%g" % (float(was) + 0.5 if float(was) < 30 else float(was) - 0.5)
    comfort.fill(other)
    assert send.is_enabled(), "a keystroke turns Send on"
    comfort.press("Tab")
    assert send.is_enabled(), "a changed value keeps it on"
    comfort.fill(was)
    comfort.press("Tab")
    assert send.is_disabled(), "the value put back turns Send off"
    held, sent = [], []
    hold(page, "**/rooms/*/config", "PUT", held, sent)
    try:
        comfort.fill(other)
        comfort.press("Tab")
        send.click()
        assert wait_held(page, held), "the config PUT is sent"
        assert_busy(page, sel + " .config-body form button[type=submit]", "Send")
        send.click(force=True)
        page.wait_for_timeout(300)
        assert len(sent) == 1, f"one PUT for two clicks: {sent}"
        assert list(sent[0].keys()) == ["comfort_temp"], f"the changed field alone is sent: {sent}"
        answer(held, OK)
        assert wait_until(page, "(() => { const b = document.querySelector(%r); return b && !b.classList.contains('busy'); })()"
                          % (sel + " .config-body form button[type=submit]"), 5), "Send stops turning once answered"
        assert send.is_disabled(), "once the Cube took the value, nothing differs"
    finally:
        page.unroute("**/rooms/*/config")
    tile.locator(".config-tabs .close").click()


def test_week_programme_send_is_off_until_a_point_changes(page, case):
    """The week programme's Send is off until a switch point changes, and
    off again after Reset all."""
    tile, _ = first_tile(page)
    open_tab(tile, "program")
    send = tile.locator(".config-body form button[type=submit]")
    assert send.is_disabled(), "Send is off with the programme as loaded"
    temp = tile.locator(".prog-point input[type=number]").first
    was = float(temp.input_value())
    temp.fill("%g" % (was + 0.5 if was < 30 else was - 0.5))
    temp.press("Tab")
    assert wait_until(page, "!!document.querySelector('.prog-days .day.dirty')", 5), "the day is marked as changed"
    assert send.is_enabled(), "a changed point turns Send on"
    tile.locator(".prog-reset-all").click()
    assert send.is_disabled(), "Reset all turns Send off"
    tile.locator(".config-tabs .close").click()


def test_copy_send_is_off_until_a_room_is_ticked(page, case):
    """Copy's Send is off until a room to copy to is ticked, and again
    once it is unticked."""
    tile, _ = first_tile(page)
    open_tab(tile, "copy")
    send = tile.locator(".config-body form button[type=submit]")
    assert send.is_disabled(), "Send is off with no room ticked"
    room = tile.locator(".copy-section").first.locator(".copy-list .switch").first
    room.wait_for()
    room.click()
    assert send.is_enabled(), "a room ticked turns Send on"
    tile.locator(".copy-section").first.locator(".copy-list .switch").first.click()
    assert send.is_disabled(), "the room unticked turns Send off"
    tile.locator(".config-tabs .close").click()


def test_house_send_turns_and_needs_a_room(page, case):
    """The House panel's Send is off while no room is ticked; held, it
    turns, and a second click sends nothing."""
    page.locator("#house-auto").click()
    send = page.locator("#house-panel button[type=submit]")
    page.locator("#house-all").uncheck()
    assert send.is_disabled(), "Send is off with no room ticked"
    page.locator("#house-all").check()
    assert send.is_enabled(), "All rooms turns Send on"
    held, sent = [], []
    hold(page, "**/rooms/*/setpoint", "PUT", held, sent)
    try:
        send.click()
        assert wait_held(page, held), "the first room's PUT is sent"
        assert_busy(page, "#house-panel button[type=submit]", "the House panel's Send")
        send.click(force=True)
        page.wait_for_timeout(300)
        assert len(sent) == 1, f"one PUT at a time, the second click sending nothing: {sent}"
        for _ in range(20):
            answer(held)
            if busy_state(page, "#house-panel button[type=submit]")[1] is False:
                break
            page.wait_for_timeout(200)
        assert busy_state(page, "#house-panel button[type=submit]") == [False, False, None], "Send is back once every room answered"
    finally:
        page.unroute("**/rooms/*/setpoint")
    page.locator("#house-cancel").click()


def test_boost_from_the_room_menu(page, case):
    """The room's ⋯ has Boost: it sends the room's setpoint with mode
    boost, the menu closing, the item turning until the answer."""
    tile, sel = first_tile(page)
    held, sent = [], []
    hold(page, "**/rooms/*/setpoint", "PUT", held, sent)
    try:
        tile.locator("details.menu summary").click()
        tile.locator(".menu-boost").click()
        assert wait_held(page, held), "the setpoint PUT is sent"
        assert sent == [{"mode": "boost"}], f"the body names the boost alone: {sent}"
        assert tile.locator("details.menu").get_attribute("open") is None, "the menu closes"
        answer(held)
        assert wait_until(page, "!document.querySelector(%r).disabled" % (sel + " .mode-auto"), 5), "the tile's buttons are back"
    finally:
        page.unroute("**/rooms/*/setpoint")
