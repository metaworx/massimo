# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The Rooms page: the tiles, a tile's device list and its
configuration; the open-window badge; the ages of the values, a stale
device, and what expert mode adds.

The window and the staleness come from state events applied in the page
(updateDevice), since the fake Cube does not change its states.
"""
from helpers import OPEN, STALE, named_buttons, scroll_width, wait_until


def test_tiles_device_list_configuration(page, case):
    assert page.locator(".room").count() > 0, "room tiles shown"
    assert page.evaluate("document.documentElement.lang") == case.lang, f"page language {case.lang}"
    named_buttons(page, "the rooms")
    tile = page.locator(".room").first
    tile.locator(".room-name-btn").click()
    page.wait_for_selector(".room .device", timeout=5000)
    assert tile.locator(".device").count() > 0, "device list opens"
    named_buttons(page, "a device list")
    tile.locator(".tile-cog").click()
    page.wait_for_selector(".room .config-body", timeout=5000)
    tabs = tile.locator(".config-tabs [role=tab]")
    assert tabs.count() >= 4, "configuration opens with its tabs"
    for name in tabs.all_text_contents():
        tabs.filter(has_text=name).first.click()
        assert tile.locator(".config-body").is_visible(), f"tab {name.strip()}"
        named_buttons(page, f"the configuration tab {name.strip()}")
    tile.locator(".config-tabs .close").click()
    assert tile.locator(".config-body").count() == 0, "configuration closes"
    assert scroll_width(page) <= case.width, "no horizontal scroll on the tiles"


def test_open_window(page, case):
    """The room with the open window contact shows Open on its tile, and
    the badge goes when a state event closes the contact; the measured
    temperature shows its age."""
    room = page.locator(".room", has=page.locator(".room-name", has_text="Kind 1"))
    badge = room.locator(".window-open")
    assert badge.is_visible() and badge.inner_text().startswith(OPEN[case.lang]), "Open on the tile of the room with the open window"
    assert page.locator(".room").first.locator(".window-open").is_hidden(), "no Open on a room with closed windows"
    assert page.locator(".room .temp-age .age").count() > 0, "age next to the measured temperature"
    page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.some((d) => d.type === "shutter_contact"));
      const d = JSON.parse(JSON.stringify(r.devices.find((d) => d.type === "shutter_contact")));
      d.state.window_open = false;
      updateDevice(d);
    }""")
    assert room.locator(".window-open").is_hidden(), "Open goes when the window closes"


def test_stale_and_expert(page, case):
    """The first room's devices made 30 hours old and stale: the tile
    warns, the values are greyed and the device list says so; expert mode
    adds the type, the radio address, the exact times and the Cube's
    radio budget in the header."""
    page.evaluate("""() => {
      const iso = new Date(Date.now() - 30 * 3600 * 1000).toISOString();
      for (const x of state.rooms[0].devices) {
        const d = JSON.parse(JSON.stringify(x));
        d.stale = true;
        d.last_seen = iso;
        for (const k of Object.keys(d.received || {})) d.received[k] = iso;
        updateDevice(d);
      }
    }""")
    tile = page.locator(".room").first
    note = tile.locator(".stale-note")
    assert note.is_visible() and STALE[case.lang] in note.inner_text(), "stale warning on the tile"
    assert tile.locator(".temp-val.stale").count() == 1, "stale measured temperature greyed"
    assert "30" in tile.locator(".temp-age").inner_text(), "the measured temperature is 30 hours old"
    tile.locator(".room-name-btn").click()
    assert STALE[case.lang] in tile.locator(".device .stale-badge").first.inner_text(), "stale badge in the device list"
    assert tile.locator(".device-vals.stale .age").count() > 0, "ages of the stale values in the device list"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the ages"
    assert tile.locator(".device .device-sub").first.is_hidden(), "no type and radio address outside expert mode"
    page.evaluate("() => { state.settings.expert = true; applySettings(); }")
    assert tile.locator(".device .device-rf").first.is_visible(), "radio address in expert mode"
    ids = tile.locator(".device .device-rf").first.inner_text()
    assert ids.startswith("S/N ") and " · RF 0a" in ids, f"the serial beside the radio address in expert mode ({ids!r})"
    assert tile.locator(".device-vals .age-exact").first.is_visible(), "exact times in expert mode"
    assert tile.locator(".device-seen").first.is_visible(), "last seen in expert mode"
    stats = page.locator("#cube-stats")
    assert stats.is_visible() and page.locator("#stats-cube").inner_text() == page.evaluate(
        "(() => { const c = state.cubes.find((x) => x.serial === state.serial); return c.name || c.serial; })()"), \
        "the Cube's name and its radio budget in the header in expert mode"
    assert scroll_width(page) <= case.width, "no horizontal scroll in expert mode"
    page.evaluate("() => { state.settings.expert = false; applySettings(); }")
    tile.locator(".room-name-btn").click()


def test_typing_in_the_configuration_survives_a_state_event(page, case):
    """A state event for a room whose configuration has the focus waits:
    the tile is not drawn again under the user's typing, and is once the
    focus leaves the configuration, with the new state. Nothing is sent."""
    tile = page.locator(".room").first
    tile.locator(".tile-cog").click()
    page.wait_for_selector(".room .config-body", timeout=5000)
    tile.locator(".config-tabs [role=tab][data-tab=devices]").click()
    name = tile.locator(".config-area .cfg-device input[type=text]").first
    name.fill("Typed")
    page.evaluate("document.querySelector('.room').dataset.mark = 'before'")
    page.evaluate("""() => {
      const d = JSON.parse(JSON.stringify(state.rooms[0].devices[0]));
      d.state = Object.assign({}, d.state, { valve_pct: 77 });
      state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({
        kind: "state", device: d.rf_address, state: d, time: new Date().toISOString() }) }));
    }""")
    page.keyboard.type(" on")
    assert name.input_value() == "Typed on", "every key typed is in the name"
    assert page.evaluate("document.querySelector('.room').dataset.mark") == "before", "the tile is not drawn again while typed in"
    assert page.evaluate("state.rooms[0].devices[0].state.valve_pct") == 77, "the state is taken all the same"
    page.evaluate("document.activeElement.blur()")
    assert wait_until(page, "document.querySelector('.room').dataset.mark === undefined", 5), "the tile is drawn once the focus leaves"
    tile.locator(".config-tabs .close").click()
    assert tile.locator(".config-body").count() == 0, "the configuration closes, nothing sent"


def test_a_switch_point_takes_a_full_time(page, case):
    """The programme's time input keeps the focus while a time is typed
    part by part (Chrome fires change at each part), and Send right after
    it sends the time; the PUT is stubbed, nothing reaches the Cube."""
    tile = page.locator(".room").first
    tile.locator(".tile-cog").click()
    page.wait_for_selector(".room .config-body", timeout=5000)
    tile.locator(".config-tabs [role=tab][data-tab=program]").click()
    time_input = tile.locator(".prog-point input[type=time]").first
    time_input.wait_for()
    sent = []

    def answer(route):
        sent.append(route.request.post_data_json)
        route.fulfill(status=200, content_type="application/json", body="{}")

    page.route("**/rooms/*/program", answer)
    try:
        before = time_input.input_value()
        time_input.focus()  # the hour part
        page.keyboard.type("0115A")  # headless Chromium shows 12 hours and AM/PM in both languages
        assert time_input.input_value() == "01:15", f"the full time typed, from {before}"
        assert "dirty" in tile.locator(".prog-days .day.editing").get_attribute("class") and \
            tile.locator(".prog-reset-day").is_enabled(), "the day is marked as changed in place"
        tile.locator(".config-body button[type=submit]").click()
        page.wait_for_timeout(500)
        assert sent, "Send right after the time sends it"
        points = next(iter(sent[0].values()))
        assert points[0]["until"] == "01:15", f"the time sent: {points}"
    finally:
        page.unroute("**/rooms/*/program")
    tile.locator(".config-tabs .close").click()
