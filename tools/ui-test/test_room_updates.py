# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A room tile keeps what is open, focused, typed and selected in it
when data comes in (AP UiParts, Block 7): the cases of the room tile in
the fresh look's probe and the spike's four more, each a test, and the
House panel's two. Before the tiles were parts, a state event built the
tile again and a reload of the rooms every tile and the House panel's
checkboxes, and sixteen of the twenty cases lost what the user had, and
one of the House panel's. The configuration's three cases of the probe
are with them, and since it is a part (Block 8) its tabs are checked to
be the elements they were once the focus has left them, and to say a
value stored meanwhile.

Data comes the way the page gets it: a state event is updateDevice with
a device of the room as it is, what every state event of the stream
runs; a reload is loadRooms(), what a metadata event runs, and the poll
while the stream is down. The poll's timer is stopped, so that every
redraw is the test's. The setpoint's PUT is answered with a stub where
a test presses +, so that nothing reaches the fake Cube."""
import pytest

from helpers import wait_until


def held(page):
    """The poll's timer stopped, so that only the test's data come."""
    page.evaluate("clearInterval(state.pollTimer); state.pollTimer = null")


def first_room(page):
    """The first room with a thermostat: its id and its tile's selector."""
    rid = page.evaluate("state.rooms.find(roomThermostat).id")
    return rid, '.room[data-id="%s"]' % rid


def state_event(page, rid):
    """A state event of a thermostat of the room (else its first device),
    as it is: what the stream's state event runs."""
    page.evaluate("""(rid) => {
      const r = state.rooms.find((x) => x.id === rid);
      const d = r.devices.find(isThermostat) || r.devices[0];
      updateDevice(JSON.parse(JSON.stringify(d)));
    }""", rid)
    page.wait_for_timeout(50)


def rooms_loaded(page):
    """The rooms loaded again, as a metadata event loads them."""
    page.evaluate("loadRooms()")
    page.wait_for_timeout(400)


def settled(page):
    """A frame and a task of the page awaited: what the focus leaving set
    off at setTimeout(0), as the plain configuration's fill put off was,
    has run by then."""
    page.evaluate("new Promise((done) => requestAnimationFrame(() => setTimeout(done)))")


def active(page):
    """The element with the focus, by its tag and classes."""
    return page.evaluate("""() => {
      const a = document.activeElement;
      if (!a || a === document.body) return "(nothing)";
      return a.tagName.toLowerCase() + (typeof a.className === "string" && a.className.trim() ? "." + a.className.trim().split(/\\s+/).join(".") : "")
        + (a.name ? "[name=" + a.name + "]" : "");
    }""")


@pytest.fixture
def refused(page):
    """The setpoint's PUT answered at its route with a stub, so that a
    press on + that a test does not cancel in time sends nothing to the
    fake Cube."""
    page.route("**/rooms/*/setpoint", lambda route: route.fulfill(status=200, content_type="application/json", body="{}")
               if route.request.method == "PUT" else route.continue_())
    yield
    page.unroute("**/rooms/*/setpoint")


def test_more_menu_stays_open(page, case):
    """More, opened, stays open through a state event of its room."""
    held(page)
    rid, tile = first_room(page)
    page.locator(tile + " .more-sum").click()
    assert page.evaluate("(s) => document.querySelector(s).open", tile + " .more-menu"), "More opens"
    state_event(page, rid)
    assert page.evaluate("(s) => document.querySelector(s).open", tile + " .more-menu"), "More is still open"
    page.keyboard.press("Escape")


def test_room_menu_stays_open(page, case):
    """The tile's ⋯ menu, opened, stays open through a state event."""
    held(page)
    rid, tile = first_room(page)
    page.locator(tile + " .tile-head > details.menu > summary").click()
    state_event(page, rid)
    assert page.evaluate("(s) => document.querySelector(s).open", tile + " .tile-head > details.menu"), "⋯ is still open"
    page.keyboard.press("Escape")


def test_problem_list_stays_open(page, case):
    """A room's problem list (the fixture's open window), opened, stays
    open through a state event of its room."""
    held(page)
    prid = page.evaluate("""() => {
      const el = document.querySelector(".room .room-problems details.problems");
      return el ? Number(el.closest(".room").dataset.id) : null;
    }""")
    assert prid is not None, "the fixture has a room with a problem"
    ptile = '.room[data-id="%s"]' % prid
    page.locator(ptile + " .room-problems .problems-sum").click()
    state_event(page, prid)
    assert page.evaluate("(s) => { const d = document.querySelector(s); return !!d && d.open; }", ptile + " .room-problems details"), \
        "the problem list is still open"
    page.keyboard.press("Escape")


def test_focus_on_plus_stays(page, case):
    """The focus on the stepper's + stays through a state event."""
    held(page)
    rid, tile = first_room(page)
    page.locator(tile + " .step-plus").focus()
    state_event(page, rid)
    assert "step-plus" in active(page), f"the focus on +: {active(page)}"


def test_focus_on_plus_stays_after_enter(page, case, refused):
    """Enter on + steps the setpoint and leaves the focus on +."""
    held(page)
    rid, tile = first_room(page)
    page.locator(tile + " .step-plus").focus()
    page.keyboard.press("Enter")
    page.wait_for_timeout(50)
    focus = active(page)
    page.locator(tile + " .pending-cancel").click()
    assert "step-plus" in focus, f"the focus on + after Enter: {focus}"


def test_text_selection_stays(page, case):
    """The room's name selected stays selected through a state event."""
    held(page)
    rid, tile = first_room(page)
    page.evaluate("""(s) => {
      const r = document.createRange();
      r.selectNodeContents(document.querySelector(s));
      getSelection().removeAllRanges();
      getSelection().addRange(r);
    }""", tile + " .room-name")
    before = page.evaluate("getSelection().toString()")
    state_event(page, rid)
    after = page.evaluate("getSelection().toString()")
    page.evaluate("getSelection().removeAllRanges()")
    assert before and before == after, f"the selection: {before!r} -> {after!r}"


def test_failed_result_stays(page, case):
    """The result line of a write that failed stays through a state event."""
    held(page)
    rid, tile = first_room(page)
    page.evaluate("(rid) => showTileResult(rid, 'The write failed', false)", rid)
    state_event(page, rid)
    text = page.evaluate("(s) => { const p = document.querySelector(s); return p.classList.contains('hidden') ? '' : p.textContent; }",
                         tile + " .tile-result")
    assert text == "The write failed", f"the result line shows {text!r}"


def open_holiday(page, tile):
    """The holiday form opened from More, 23.5 typed into its temperature."""
    page.locator(tile + " .more-sum").click()
    page.locator(tile + " .menu-holiday").click()
    temp = page.locator(tile + " .vacation-temp")
    temp.click()
    temp.press("Control+a")
    page.keyboard.type("23.5")
    return temp


def test_holiday_temperature_stays(page, case):
    """A temperature typed into the holiday form, the field not left,
    stays through a state event."""
    held(page)
    rid, tile = first_room(page)
    temp = open_holiday(page, tile)
    state_event(page, rid)
    value = temp.input_value()
    page.locator(tile + " .vacation-panel .form-cancel").click()
    assert value == "23.5", f"the temperature typed: {value!r}"


def test_holiday_focus_stays(page, case):
    """The focus in the holiday form's temperature stays through a state
    event."""
    held(page)
    rid, tile = first_room(page)
    open_holiday(page, tile)
    state_event(page, rid)
    focus = active(page)
    page.locator(tile + " .vacation-panel .form-cancel").click()
    assert "vacation-temp" in focus, f"the focus in the temperature: {focus}"


def test_holiday_stays_through_a_reload(page, case):
    """The holiday form's temperature typed, and its focus, stay through a
    reload of the rooms."""
    held(page)
    _, tile = first_room(page)
    temp = open_holiday(page, tile)
    rooms_loaded(page)
    value, focus = temp.input_value(), active(page)
    page.locator(tile + " .vacation-panel .form-cancel").click()
    assert value == "23.5" and "vacation-temp" in focus, f"the temperature {value!r}, the focus {focus}"


def open_devices(page, tile):
    """The tile's device list opened."""
    page.locator(tile + " .tile-info").click()
    page.locator(tile + " .devices .device").first.wait_for(state="visible")


def test_device_list_stays_open(page, case):
    """The tile's device list, opened, stays open through a state event."""
    held(page)
    rid, tile = first_room(page)
    open_devices(page, tile)
    state_event(page, rid)
    assert page.locator(tile + " .devices").is_visible(), "the device list is still open"
    page.locator(tile + " .tile-info").click()


def test_focus_on_lock_stays(page, case):
    """The focus on a device's Lock in the list stays through a state
    event."""
    held(page)
    rid, tile = first_room(page)
    open_devices(page, tile)
    page.locator(tile + " .devices .device-lock").first.focus()
    state_event(page, rid)
    focus = active(page)
    page.locator(tile + " .tile-info").click()
    assert "device-lock" in focus, f"the focus on Lock: {focus}"


def test_virtual_thermostat_address_stays(page, case):
    """The virtual wall thermostat's radio address typed, in expert mode,
    stays through a state event."""
    held(page)
    rid = page.evaluate("state.rooms.find((r) => r.vwt).id")
    tile = '.room[data-id="%s"]' % rid
    page.evaluate("() => { state.settings.expert = true; applySettings(); }")
    open_devices(page, tile)
    addr = page.locator(tile + " .devices .device.virtual input[name=rf_address]")
    addr.wait_for(state="visible")
    page.wait_for_timeout(300)  # its settings load and fill the form
    addr.click()
    addr.press("Control+a")
    page.keyboard.type("0a0b0c")
    state_event(page, rid)
    page.wait_for_timeout(300)
    value = page.evaluate("(s) => { const i = document.querySelector(s); return i ? i.value : null; }",
                          tile + " .devices .device.virtual input[name=rf_address]")
    page.evaluate("() => { state.settings.expert = false; applySettings(); }")
    page.locator(tile + " .tile-info").click()
    assert value == "0a0b0c", f"the address typed: {value!r}"


def config_name(page, tile):
    """The configuration opened on its Room tab, a name typed."""
    page.locator(tile + " .tile-cog").click()
    name = page.locator(tile + " .config-area input[type=text]").first
    name.wait_for()
    name.click()
    name.press("Control+a")
    page.keyboard.type("Typed name")
    return name


def test_config_name_stays_while_focused(page, case):
    """A name typed in the configuration, the focus in it, stays with the
    focus through a state event."""
    held(page)
    rid, tile = first_room(page)
    config_name(page, tile)
    state_event(page, rid)
    value = page.evaluate("(s) => document.querySelector(s).value", tile + " .config-area input[type=text]")
    focus = active(page)
    page.locator(tile + " .config-tabs .close").click()
    assert value == "Typed name" and focus.startswith("input"), f"the name {value!r}, the focus {focus}"


def test_config_name_stays_once_left(page, case):
    """The same name stays once the focus has left the configuration."""
    held(page)
    rid, tile = first_room(page)
    config_name(page, tile)
    state_event(page, rid)
    page.locator(tile + " .room-name-btn").focus()
    settled(page)
    value = page.evaluate("(s) => { const i = document.querySelector(s); return i ? i.value : null; }", tile + " .config-area input[type=text]")
    page.locator(tile + " .config-tabs .close").click()
    assert value == "Typed name", f"the name {value!r}"


def pressed(page, rid, tile, between):
    """A click on the tile's + with between() run after the press and
    before the release, about as long as a click takes; whether the click
    stepped the setpoint, which is then cancelled."""
    box = page.locator(tile + " .step-plus").bounding_box()
    page.mouse.move(box["x"] + box["width"] / 2, box["y"] + box["height"] / 2)
    page.mouse.down()
    between()
    page.wait_for_timeout(80)
    page.mouse.up()
    page.wait_for_timeout(100)
    took = page.evaluate("(rid) => state.ui[rid].pending !== null", rid)
    if took:
        page.locator(tile + " .pending-cancel").click()
    return took


def test_click_on_plus_through_a_state_event(page, case, refused):
    """A click on + counts with a state event between its press and its
    release."""
    held(page)
    rid, tile = first_room(page)
    assert pressed(page, rid, tile, lambda: state_event(page, rid)), "the click stepped the setpoint"


def test_click_on_plus_through_a_reload(page, case, refused):
    """A click on + counts with the rooms loaded again between its press
    and its release."""
    held(page)
    rid, tile = first_room(page)
    assert pressed(page, rid, tile, lambda: page.evaluate("loadRooms()")), "the click stepped the setpoint"


def test_click_on_plus_from_the_configuration(page, case, refused):
    """A click on the tile's + from its configuration counts, a state
    event having come while the focus was in the configuration."""
    held(page)
    rid, tile = first_room(page)
    page.locator(tile + " .tile-cog").click()
    name = page.locator(tile + " .config-area input[type=text]").first
    name.wait_for()
    name.click()
    state_event(page, rid)
    took = pressed(page, rid, tile, lambda: None)
    page.locator(tile + " .config-tabs .close").click()
    assert took, "the click stepped the setpoint"


# The configuration's own cases (AP UiParts, Block 8): before it was a
# part, a state event or a reload filled it again at once where the focus
# was elsewhere, and once the focus had left it where it was in it, every
# element then a new one; a value stored meanwhile was not said.

CONFIG_TABS = ("room", "temps", "devices", "program", "copy")


def open_config(page, tile, tab):
    """The tile's configuration open on tab, its form drawn."""
    if page.locator(tile + " .config-body").count() == 0:
        page.locator(tile + " .tile-cog").click()
    page.locator(tile + " .config-tabs [role=tab][data-tab=%s]" % tab).click()
    page.locator(tile + " .config-body form").first.wait_for()


def mark_config(page, tile):
    """Every button, field and select the configuration shows, marked."""
    return page.evaluate("""(s) => {
      const all = [...document.querySelectorAll(s + " .config-area button, " + s + " .config-area input, " + s + " .config-area select")];
      all.forEach((el) => { el.__mark = true; });
      return all.length;
    }""", tile)


def unmarked(page, tile):
    """The configuration's buttons, fields and selects not marked, and the marked ones gone from the page."""
    return page.evaluate("""(s) => {
      const now = [...document.querySelectorAll(s + " .config-area button, " + s + " .config-area input, " + s + " .config-area select")];
      return now.filter((el) => !el.__mark).map((el) => el.outerHTML.slice(0, 80));
    }""", tile)


@pytest.mark.parametrize("tab", CONFIG_TABS)
def test_config_is_the_same_once_the_focus_left(page, case, tab):
    """Each tab of the configuration is the elements it was, and its first
    field holds what is typed (the Copy tab has none, and the week
    programme's focus is on a day), after a state event, a reload of the
    rooms and the focus leaving it; the tab stays shown."""
    held(page)
    rid, tile = first_room(page)
    open_config(page, tile, tab)
    # the week programme draws itself again at its own change, which the
    # focus leaving a typed field makes: a day's button holds the focus there
    field = page.locator(tile + " .config-body form input:is([type=text], [type=number])").first
    typed = None
    if tab != "program" and field.count():
        field.click()
        field.press("Control+a")
        typed = "7" if field.get_attribute("type") == "number" else "Typed"
        page.keyboard.type(typed)
    else:
        page.locator(tile + " .config-body form button").first.focus()
    n = mark_config(page, tile)
    assert n > 0, "the tab shows buttons and fields"
    state_event(page, rid)
    rooms_loaded(page)
    page.locator(tile + " .room-name-btn").focus()
    settled(page)
    fresh = unmarked(page, tile)
    value = field.input_value() if typed is not None else None
    selected = page.locator(tile + " .config-tabs [role=tab][aria-selected=true]").get_attribute("data-tab")
    page.locator(tile + " .config-tabs .close").click()
    assert not fresh, f"the {tab} tab built again: {fresh[:3]}"
    assert value == typed, f"what is typed: {value!r}"
    assert selected == tab, f"the tab shown: {selected}"


def test_config_says_a_name_stored_meanwhile(page, case, reads):
    """A room name typed in the configuration stays when the rooms load with
    another name stored meanwhile, which the field then says, with Take
    theirs; the device's name the head of its form shows follows a state
    event while the focus is in its form."""
    held(page)
    rid, tile = first_room(page)
    open_config(page, tile, "room")
    name = page.locator(tile + " .config-body form input[type=text]").first
    name.fill("Typed name")
    reads.change("**/api/v1/cubes/*/rooms?devices=all",
                 lambda rooms: [dict(r, name="Renamed elsewhere") if r.get("id") == rid else r for r in rooms])
    try:
        rooms_loaded(page)
        notice = page.locator(tile + " .config-body .field-meanwhile")
        assert wait_until(page, "!!document.querySelector(%r)" % (tile + " .config-body .field-meanwhile"), 5), "the field says the name stored meanwhile"
        assert "Renamed elsewhere" in notice.inner_text(), notice.inner_text()
        assert name.input_value() == "Typed name", "what is typed stays"
        notice.locator(".take-theirs").click()
        assert name.input_value() == "Renamed elsewhere", "Take theirs takes the name stored"
    finally:
        reads.clear("**/api/v1/cubes/*/rooms?devices=all")
    open_config(page, tile, "devices")
    field = page.locator(tile + " .config-body .cfg-device input[type=text]").first
    field.click()
    page.evaluate("""(rid) => {
      const r = state.rooms.find((x) => x.id === rid);
      const d = JSON.parse(JSON.stringify(r.devices[0]));
      d.name = "Named by an event";
      updateDevice(d);
    }""", rid)
    head = page.locator(tile + " .config-body .cfg-device .device-main .device-name").first
    assert wait_until(page, "document.querySelector(%r).textContent === 'Named by an event'" % (tile + " .config-body .cfg-device .device-main .device-name"), 3), \
        "the form's head follows, the focus in the form: " + head.inner_text()
    assert page.evaluate("document.activeElement === document.querySelector(%r)" % (tile + " .config-body .cfg-device input[type=text]")), \
        "the focus stays in the name"
    page.locator(tile + " .config-tabs .close").click()


def test_tile_is_the_same_element(page, case):
    """The tile is the element it was after a state event and a reload."""
    held(page)
    rid, tile = first_room(page)
    page.evaluate("(s) => { document.querySelector(s).__mark = true; }", tile)
    state_event(page, rid)
    rooms_loaded(page)
    assert page.evaluate("(s) => !!document.querySelector(s).__mark", tile), "the same tile"


def test_new_setpoint_is_shown(page, case):
    """A setpoint a state event brings is shown."""
    held(page)
    rid, tile = first_room(page)
    page.evaluate("""(rid) => {
      const r = state.rooms.find((x) => x.id === rid);
      const d = JSON.parse(JSON.stringify(r.devices.find(isThermostat)));
      d.state.setpoint = 27.5;
      updateDevice(d);
    }""", rid)
    assert wait_until(page, "document.querySelector(%r).textContent.includes('27')" % (tile + " .setpoint-big"), 3), \
        "the new setpoint: " + page.locator(tile + " .setpoint-big").inner_text()


def open_house(page):
    """The House panel opened with AUTO."""
    page.locator("#house-auto").click()
    page.locator("#house-panel").wait_for(state="visible")


def test_house_named_after_the_cube(page, case):
    """The House tile is named after the chosen Cube, the cube before its
    name; its tooltip and its name for a screen reader say that it is all
    the Cube's rooms (AP GlobalMenu, D193-A)."""
    name = page.evaluate("(() => { const c = state.cubes.find((x) => x.serial === state.serial); return c.name || c.serial; })()")
    title = page.locator("#house .tile-head h2")
    assert title.inner_text().strip() == name, f"the Cube's name: {title.inner_text()!r}"
    assert title.locator(".cube-icon use[href='#icon-cube3d']").count() == 1, "the cube before it"
    whole = page.evaluate("(n) => t('house.roomsOf', {cube: n})", name)
    assert title.get_attribute("title") == whole and title.get_attribute("aria-label") == whole, "all the Cube's rooms, its tooltip"


def test_house_room_unticked_stays(page, case):
    """A room unticked in the House panel stays unticked through a reload
    of the rooms."""
    held(page)
    open_house(page)
    box = page.locator("#house-rooms .check:not(.check-all) input").first
    box.uncheck()
    rooms_loaded(page)
    checked = page.locator("#house-rooms .check:not(.check-all) input").first.is_checked()
    page.locator("#house-panel .form-cancel").click()
    assert not checked, "the room is still unticked"


def test_house_focus_stays(page, case):
    """The focus on a room's checkbox in the House panel stays through a
    reload of the rooms."""
    held(page)
    open_house(page)
    page.locator("#house-rooms .check:not(.check-all) input").first.focus()
    rooms_loaded(page)
    focus = page.evaluate("document.activeElement === document.querySelector('#house-rooms .check:not(.check-all) input')")
    page.locator("#house-panel .form-cancel").click()
    assert focus, f"the focus on the room's checkbox: {active(page)}"
