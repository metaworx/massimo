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
value stored meanwhile. The House tile's one form, what each choice
offers and its defaults, and its lowest and highest, which follow a
state event in place, are with them too (AP RoomsPage v1.0, Block 7),
and tomorrow morning, the default end of its MANU until a date and of a
room's holiday form, on the days of a change of the clocks, the page in
Zurich's zone under Playwright's clock.

Data comes the way the page gets it: a state event is updateDevice with
a device of the room as it is, what every state event of the stream
runs; a reload is loadRooms(), what a metadata event runs, and the poll
while the stream is down. The poll's timer is stopped, so that every
redraw is the test's. The setpoint's PUT is answered with a stub where
a test presses +, so that nothing reaches the fake Cube. A state that a
test makes up, the House tile's virtual wall thermostats', is told to
the rooms' read as well and kept (tell_vwt, AP ToldData), so that the
server's own events and any read of the rooms bring it back, and its
check waits for it."""
from datetime import datetime, timedelta, timezone

import pytest

from helpers import boxes, frames, held_poll, hold, mark, replaced, wait_held, wait_until
from helpers import settled as looked_until


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


# The House form's one choice of a mode (AP RoomsPage v1.0, Block 7, D7-A):
# what it offers and its defaults by the choice; nothing is sent.

def house_mode(page):
    """The House form's mode chosen, its radio button's value."""
    return page.evaluate("(() => { const e = document.querySelector('#house-panel input[name=house-mode]:checked'); return e ? e.value : null; })()")


def durations_shown(page):
    """The durations the House form shows, its radio buttons' values, and the one chosen."""
    boxes = page.locator("#house-panel input[name=house-duration]")
    shown = [boxes.nth(i).get_attribute("value") for i in range(boxes.count()) if boxes.nth(i).is_visible()]
    chosen = [boxes.nth(i).get_attribute("value") for i in range(boxes.count()) if boxes.nth(i).is_visible() and boxes.nth(i).is_checked()]
    return shown, chosen


def week_ahead(page):
    """A week from the page's today at 06:00, as a date and time field holds it."""
    return page.evaluate("""(() => {
      const d = new Date(), p = (n) => String(n).padStart(2, "0");
      const w = new Date(d.getFullYear(), d.getMonth(), d.getDate() + 7, 6, 0);
      return w.getFullYear() + "-" + p(w.getMonth() + 1) + "-" + p(w.getDate()) + "T06:00";
    })()""")


def first_setpoint(page):
    """The first room's setpoint, of the room with a thermostat the House lists first, as its temperature field holds it."""
    return page.evaluate("String(roomState(state.rooms.find(roomThermostat)).setpoint)")


def test_house_holiday_opens_with_its_defaults(page, case):
    """The House tile's Holiday, its case icon in the modes' row, opens the
    form with Holiday chosen: the temperature OFF (4.5), Until chosen and
    Permanently not offered, the end a week ahead at 06:00, shown (D6-A,
    D7-A). Fails on the page before, which had no Holiday."""
    held(page)
    button = page.locator("#house .house-modes #house-holiday")
    assert button.locator("use[href='#icon-suitcase']").count() == 1, "the case icon"
    button.click()
    page.locator("#house-panel").wait_for(state="visible")
    assert house_mode(page) == "vacation", f"Holiday chosen: {house_mode(page)}"
    assert page.locator("#house-temp").is_visible() and page.locator("#house-temp").input_value() == "4.5", \
        f"the temperature OFF: {page.locator('#house-temp').input_value()}"
    assert durations_shown(page) == (["until"], ["until"]), f"Until alone, chosen: {durations_shown(page)}"
    until = page.locator("#house-until")
    assert until.is_visible() and until.input_value() == week_ahead(page), f"a week ahead at 06:00: {until.input_value()}"
    page.locator("#house-panel .form-cancel").click()


def test_house_manu_after_holiday_offers_permanently(page, case):
    """Holiday, then MANU chosen in the form: Permanently offered again and
    chosen, the temperature the first room's setpoint, the date hidden;
    Holiday chosen again: OFF and a week ahead once more (D7-A, D8-A)."""
    held(page)
    page.locator("#house-holiday").click()
    page.locator("#house-panel").wait_for(state="visible")
    page.locator("#house-panel input[name=house-mode][value=manual]").check()
    assert durations_shown(page) == (["permanent", "until"], ["permanent"]), f"Permanently again, chosen: {durations_shown(page)}"
    assert page.locator("#house-temp").input_value() == first_setpoint(page), \
        f"the first room's setpoint: {page.locator('#house-temp').input_value()} for {first_setpoint(page)}"
    assert not page.locator("#house-until").is_visible(), "no date while Permanently"
    page.locator("#house-panel input[name=house-mode][value=vacation]").check()
    assert page.locator("#house-temp").input_value() == "4.5", "Holiday's OFF again"
    assert durations_shown(page) == (["until"], ["until"]), f"Until alone: {durations_shown(page)}"
    assert page.locator("#house-until").input_value() == week_ahead(page), "a week ahead again"
    page.locator("#house-panel .form-cancel").click()


def test_house_keeps_a_typed_temperature(page, case):
    """A temperature typed in the form stays when another choice with a
    temperature is chosen; one untouched takes the choice's default."""
    held(page)
    page.locator("#house-manu").click()
    page.locator("#house-panel").wait_for(state="visible")
    temp = page.locator("#house-temp")
    assert temp.input_value() == first_setpoint(page), f"MANU's the first room's setpoint: {temp.input_value()}"
    page.locator("#house-panel input[name=house-mode][value=vacation]").check()
    assert temp.input_value() == "4.5", f"untouched, Holiday's OFF: {temp.input_value()}"
    page.locator("#house-panel input[name=house-mode][value=manual]").check()
    assert temp.input_value() == first_setpoint(page), f"untouched, MANU's setpoint again: {temp.input_value()}"
    temp.fill("23.5")
    page.locator("#house-panel input[name=house-mode][value=vacation]").check()
    assert temp.input_value() == "23.5", "the typed temperature stays for Holiday"
    page.locator("#house-panel .form-cancel").click()


def days_ahead(page, days, hhmm):
    """A day so many days from the page's today at a time, as a date and time field holds it."""
    return page.evaluate("""([days, hhmm]) => {
      const d = new Date(), p = (n) => String(n).padStart(2, "0");
      const w = new Date(d.getFullYear(), d.getMonth(), d.getDate() + days);
      return w.getFullYear() + "-" + p(w.getMonth() + 1) + "-" + p(w.getDate()) + "T" + hhmm;
    }""", [days, hhmm])


def test_house_keeps_a_changed_date(page, case):
    """An end the user changed stays when another choice is chosen, Holiday
    and COMFORT with Until; one untouched takes the choice's default."""
    held(page)
    page.locator("#house-manu").click()
    page.locator("#house-panel").wait_for(state="visible")
    page.locator("#house-panel input[name=house-duration][value=until]").check()
    until = page.locator("#house-until")
    assert until.input_value() == days_ahead(page, 1, "06:00"), f"untouched, tomorrow morning: {until.input_value()}"
    page.locator("#house-panel input[name=house-mode][value=vacation]").check()
    assert until.input_value() == week_ahead(page), f"untouched, Holiday's week ahead: {until.input_value()}"
    changed = days_ahead(page, 10, "18:00")
    until.fill(changed)
    page.locator("#house-panel input[name=house-mode][value=manual]").check()
    assert until.input_value() == changed, f"the changed end stays for MANU: {until.input_value()}"
    page.locator("#house-panel input[name=house-mode][value=comfort]").check()
    assert until.is_visible() and until.input_value() == changed, f"and for COMFORT with Until: {until.input_value()}"
    page.locator("#house-panel .form-cancel").click()


def test_house_untouched_date_at_each_open(page, case):
    """An end the user has not changed is the choice's default again at
    each open: MANU's tomorrow morning, then Holiday's week ahead, then
    MANU's tomorrow morning once more, not the end the form held."""
    held(page)
    until = page.locator("#house-until")
    page.locator("#house-manu").click()
    page.locator("#house-panel input[name=house-duration][value=until]").check()
    assert until.input_value() == days_ahead(page, 1, "06:00"), f"MANU's tomorrow morning: {until.input_value()}"
    page.locator("#house-panel .form-cancel").click()
    page.locator("#house-holiday").click()
    assert until.input_value() == week_ahead(page), f"Holiday's week ahead at the next open: {until.input_value()}"
    page.locator("#house-panel .form-cancel").click()
    page.locator("#house-manu").click()
    assert until.is_visible() and until.input_value() == days_ahead(page, 1, "06:00"), f"MANU's tomorrow morning again: {until.input_value()}"
    page.locator("#house-panel .form-cancel").click()


def test_house_forgets_a_past_end(page, case):
    """An end the user typed that has passed by the next open is forgotten
    there: the choice's default instead. Fails on the page before, which
    kept a typed end however old."""
    held(page)
    page.locator("#house-manu").click()
    page.locator("#house-panel input[name=house-duration][value=until]").check()
    page.locator("#house-until").fill("2020-01-02T18:00")
    page.locator("#house-panel .form-cancel").click()
    page.locator("#house-manu").click()
    until = page.locator("#house-until")
    assert until.is_visible() and until.input_value() == days_ahead(page, 1, "06:00"), f"tomorrow morning, not 2020: {until.input_value()}"
    page.locator("#house-panel .form-cancel").click()


# Tomorrow morning across a change of the clocks: the page in Zurich's
# zone under Playwright's clock, set to a moment whose day or the next
# has 23 or 25 hours; the page's local time then, and the end expected.
ZURICH = "Europe/Zurich"
CLOCK_CHANGES = [
    # the evening before the spring change, Saturday 23:30 CET
    (datetime(2027, 3, 27, 23, 30, tzinfo=timezone(timedelta(hours=1))), "2027-03-27T23:30", "2027-03-28T06:00"),
    # just after midnight on the day of the autumn change, Sunday 00:30 CEST
    (datetime(2027, 10, 31, 0, 30, tzinfo=timezone(timedelta(hours=2))), "2027-10-31T00:30", "2027-11-01T06:00"),
]


@pytest.mark.clock
@pytest.mark.browser_context_args(timezone_id=ZURICH)
@pytest.mark.parametrize("now, local, tomorrow", CLOCK_CHANGES, ids=["spring-evening", "autumn-night"])
def test_tomorrow_morning_across_a_change_of_the_clocks(page, case, now, local, tomorrow):
    """The default end of the House tile's MANU until a date and of a
    room's holiday form is tomorrow at 06:00 also across a change of the
    clocks: on the evening before the spring change and in the night of
    the autumn one, in Zurich's zone. Fails on the page before, which
    added 24 hours: Monday 06:00 on the spring evening, two days ahead,
    and Sunday 06:00 in the autumn night, the day itself."""
    held(page)
    page.clock.set_system_time(now)
    seen = page.evaluate("[Intl.DateTimeFormat().resolvedOptions().timeZone, localDateTimeValue(new Date())]")
    assert seen == [ZURICH, local], f"the page in {ZURICH} at {local}: {seen}"
    page.locator("#house-manu").click()
    page.locator("#house-panel").wait_for(state="visible")
    page.locator("#house-panel input[name=house-duration][value=until]").check()
    house_end = page.locator("#house-until").input_value()
    page.locator("#house-panel .form-cancel").click()
    _, tile = first_room(page)
    page.locator(tile + " .more-sum").click()
    page.locator(tile + " .menu-holiday").click()
    holiday_end = page.locator(tile + " .vacation-until").input_value()
    page.locator(tile + " .vacation-panel .form-cancel").click()
    assert (house_end, holiday_end) == (tomorrow, tomorrow), \
        f"tomorrow at 06:00, {tomorrow}: the House tile's end {house_end}, the room's holiday's {holiday_end}"


def test_house_date_hidden_while_permanently(page, case):
    """The House form's date and time is hidden while Permanently is
    chosen and shown with Until (D8-A); for AUTO the duration and the date
    are hidden. Fails on the page before, where the date was only off."""
    held(page)
    page.locator("#house-manu").click()
    page.locator("#house-panel").wait_for(state="visible")
    page.locator("#house-panel input[name=house-duration][value=permanent]").check()
    assert not page.locator("#house-until").is_visible(), "no date while Permanently"
    page.locator("#house-panel input[name=house-duration][value=until]").check()
    assert page.locator("#house-until").is_visible(), "the date with Until"
    page.locator("#house-panel input[name=house-mode][value=auto]").check()
    assert durations_shown(page) == ([], []), f"no duration for AUTO: {durations_shown(page)}"
    assert not page.locator("#house-until").is_visible(), "no date for AUTO"
    page.locator("#house-panel .form-cancel").click()


# The House tile's lowest and highest (D9-A, D10-A): under the modes' row,
# from the rooms' virtual wall thermostats.

RANGE = "#house .house-range"


def vwt_range(page, field):
    """The lowest and the highest of a field of the rooms' virtual wall
    thermostats' states, the rooms without one passed over; for the
    measured temperature a stale one left out while any is fresh."""
    return page.evaluate("""(f) => {
      let vs = state.rooms.map((r) => r.vwt).filter((v) => v && v.state && typeof v.state[f] === "number");
      if (f === "actual_temp" && vs.some((v) => !v.stale)) vs = vs.filter((v) => !v.stale);
      const v = vs.map((x) => x.state[f]);
      return [Math.min(...v), Math.max(...v)];
    }""", field)


def range_texts(page, row):
    """A row's lowest and highest as it shows them."""
    return [page.locator("%s .%s .range-%s" % (RANGE, row, which)).inner_text().strip() for which in ("low", "high")]


def test_house_lowest_and_highest(page, case):
    """Under the House tile's modes, the thermometer with IS and its tip,
    the rooms' lowest and highest measured temperature, and the gauge with
    SET and its tip, their lowest and highest setpoint, OFF where it is,
    from the rooms' virtual wall thermostats; no room's name. The values
    are aligned in columns where the rows are two (390 px), and on one
    line where there is room (1100 px). Fails on the page before, which
    had no such rows."""
    held(page)
    width = case.width
    page.locator(RANGE).wait_for(state="visible")
    for row, field, fmt, tag, full in (("range-is", "actual_temp", "fmtTemp", "room.is", "room.measured"),
                                       ("range-set", "setpoint", "fmtSetpoint", "room.set", "room.setTemp")):
        want = [page.evaluate("(v) => %s(v, '°')" % fmt, v) for v in vwt_range(page, field)]
        assert range_texts(page, row) == want, f"{row}: {range_texts(page, row)} for {want}"
        btn = page.locator("%s .%s button.tip-btn" % (RANGE, row))
        assert btn.inner_text().strip() == page.evaluate("(k) => t(k)", tag), f"{row}'s short label: {btn.inner_text()!r}"
        btn.click()
        tip = page.locator("%s .%s .tip-text" % (RANGE, row))
        assert tip.is_visible() and tip.inner_text().strip().lower() == page.evaluate("(k) => t(k)", full).lower(), \
            f"{row}'s full term on a tap: {tip.inner_text()!r}"
        page.keyboard.press("Escape")
    text = page.locator(RANGE).inner_text()
    names = [n for n in page.evaluate("state.rooms.map((r) => r.name)") if n and n in text]
    assert not names, f"no room's name in the rows: {names}"
    cells = [(row, which) for row in ("range-is", "range-set") for which in ("low", "high")]
    box = dict(zip(cells, boxes(page, *("%s .%s .range-%s" % (RANGE, row, which) for row, which in cells))))
    one_line = abs(box["range-is", "low"]["y"] - box["range-set", "low"]["y"]) < 1
    if width >= 1100:
        assert one_line, f"one line at {width} px: {box}"
    else:
        assert not one_line, f"two rows at {width} px: {box}"
        for which in ("low", "high"):
            assert abs(box["range-is", which]["x"] - box["range-set", which]["x"]) < 0.5, f"the {which} values aligned: {box}"


def tell_vwt(page, reads, rid, stale=None, **fields):
    """A state event of the room's virtual wall thermostat with these
    fields of its state, marked stale where stale is given: told to the
    rooms' read as well and kept (Reads.device, AP ToldData), so that the
    server's own events of it, which bring it as the server has it, and any
    read of the rooms bring it back; and put into the page as the event
    under test (updateDevice). A check of it waits (looked_until, helpers.settled)."""
    rf = page.evaluate("(rid) => state.rooms.find((r) => r.id === rid).vwt.rf_address", rid)
    told = {"state": fields}
    if stale is not None:
        told["stale"] = stale
    reads.device(rf, **told)
    page.evaluate("""([rid, st, stale]) => {
      const r = state.rooms.find((x) => x.id === rid);
      const v = JSON.parse(JSON.stringify(r.vwt));
      Object.assign(v.state, st);
      if (stale !== null) v.stale = stale;
      updateDevice(v);
    }""", [rid, fields, stale])


# The House tile's rows and what they are drawn from, in one look: each
# room's virtual wall thermostat as the page has it (id, stale, measured
# temperature, setpoint); the lowest and highest the rows should show by
# them, a stale measured temperature left out while any is fresh
# (vwt_range's rule); and the rows' values and the measured ones' classes.
RANGE_LOOK = """() => {
  const vwts = state.rooms.filter((r) => r.vwt && r.vwt.state)
    .map((r) => ({ id: r.id, stale: !!r.vwt.stale, actual: r.vwt.state.actual_temp, setpoint: r.vwt.state.setpoint }));
  const measured = vwts.filter((v) => typeof v.actual === "number");
  const used = measured.some((v) => !v.stale) ? measured.filter((v) => !v.stale) : measured;
  const a = used.map((v) => v.actual), s = vwts.map((v) => v.setpoint).filter((x) => typeof x === "number");
  const el = (row, which) => document.querySelector("#house .house-range ." + row + " .range-" + which);
  return { vwts,
    want: { is: [fmtTemp(Math.min(...a), "°"), fmtTemp(Math.max(...a), "°")], set: [fmtSetpoint(Math.min(...s), "°"), fmtSetpoint(Math.max(...s), "°")] },
    is: [el("range-is", "low").textContent.trim(), el("range-is", "high").textContent.trim()],
    set: [el("range-set", "low").textContent.trim(), el("range-set", "high").textContent.trim()],
    isCls: [el("range-is", "low").className, el("range-is", "high").className] };
}"""


def test_house_lowest_and_highest_follow_a_state_event(page, case, reads):
    """A state event of a room's virtual wall thermostat that brings a new
    lowest temperature and an OFF setpoint is shown in the House tile's
    rows, their tips' buttons the same elements. The state is told to the
    rooms' read as well (tell_vwt), the check waiting for it."""
    held(page)
    page.locator(RANGE).wait_for(state="visible")
    buttons = [RANGE + " .range-is button.tip-btn", RANGE + " .range-set button.tip-btn"]
    mark(page, buttons)
    rid = page.evaluate("state.rooms.find((r) => r.vwt && r.vwt.state).id")
    tell_vwt(page, reads, rid, actual_temp=9.5, setpoint=4.5)
    low, off = page.evaluate("[fmtTemp(9.5, '°'), t('temp.off')]")
    ok, got = looked_until(page, lambda: page.evaluate(RANGE_LOOK), lambda g: g["is"][0] == low and g["set"][0] == off)
    assert ok, f"the new lowest {low} and OFF the lowest setpoint: {got}"
    assert not replaced(page, buttons), "the tips' buttons stay"


def test_house_lowest_and_highest_leave_out_a_stale_temperature(page, case, reads):
    """A room whose virtual wall thermostat's measured temperature is stale
    counts in neither the lowest nor the highest while any room has a fresh
    one, as the server leaves a stale thermostat out within a room; with
    only stale ones, they are shown, greyed as a tile greys them. The
    states are told to the rooms' read as well (tell_vwt), each check
    waiting for the page to hold them and the rows to show them. Fails on
    the page before, which counted the stale value."""
    held(page)
    page.locator(RANGE).wait_for(state="visible")
    rids = page.evaluate("state.rooms.filter((r) => r.vwt && r.vwt.state && r.vwt.state.actual_temp !== undefined).map((r) => r.id)")
    assert len(rids) >= 2, f"rooms with a measured temperature: {rids}"
    low = page.evaluate("fmtTemp(9.5, '°')")
    tell_vwt(page, reads, rids[0], stale=True, actual_temp=9.5)

    def one_stale(g):
        mine = next((v for v in g["vwts"] if v["id"] == rids[0]), None)
        return bool(mine) and mine["stale"] and mine["actual"] == 9.5 and g["is"] == g["want"]["is"] and g["is"][0] != low \
            and all("stale" not in c for c in g["isCls"])

    ok, got = looked_until(page, lambda: page.evaluate(RANGE_LOOK), one_stale)
    assert ok, f"the stale 9.5 left out, the fresh values not greyed: {got}"
    for rid in rids[1:]:
        tell_vwt(page, reads, rid, stale=True)

    def all_stale(g):
        return all(v["stale"] for v in g["vwts"] if v["actual"] is not None) and g["is"] == g["want"]["is"] and g["is"][0] == low \
            and all("stale" in c for c in g["isCls"])

    ok, got = looked_until(page, lambda: page.evaluate(RANGE_LOOK), all_stale)
    assert ok, f"only stale ones: shown, greyed: {got}"


# A device without a room, as GET …/devices gives one (the fake Cube has
# none), at every load of the rooms.
UNLISTED_DEVICE = {"rf_address": "0a0f01", "type": "radiator_thermostat", "name": "", "room_id": 0, "serial": "MEQ0000901",
                   "unlisted": True, "reported": False}


def test_a_put_stays_with_its_cube_through_a_change_of_cube(page, case, reads):
    """A device without a room, its form that puts it into a room open,
    and another Cube chosen, the Cubes' and the rooms' reads held: the form
    stays the shown rooms' Cube's, its ids and what it would send, until
    the other Cube's rooms have come. Fails on the page before, where the
    form took the new Cube's serial over the old Cube's device and rooms
    at once, and a put sent then went to the wrong Cube."""
    reads.change("**/api/v1/cubes/*/devices?devices=real", lambda devices: (devices or []) + [UNLISTED_DEVICE])
    page.evaluate("loadRooms()")
    row = page.locator("#unlisted .device[data-rf='0a0f01']")
    row.wait_for(state="visible")
    row.locator(".put-open").click()
    form = row.locator("form.pair-device-form")
    form.wait_for(state="visible")
    before = form.locator("select").first.get_attribute("id")
    assert "KEQ0000100" in before, f"the form the Cube's: {before}"
    held, sent = [], []
    hold(page, "**/api/v1/cubes", "GET", held, sent)
    try:
        page.evaluate("selectCube('KEQ0000200')")
        assert wait_held(page, held), "the Cubes' read on its way"
        frames(page)
        assert page.evaluate("state.serial") == "KEQ0000200", "another Cube chosen"
        assert form.locator("select").first.get_attribute("id") == before, "the form still the shown rooms' Cube's"
    finally:
        page.unroute("**/api/v1/cubes")
        for route in held:
            route.continue_()
    assert wait_until(page, "state.serial === 'KEQ0000100'", 10), "the Cube served chosen again by the next load"


def test_the_house_follows_a_room_renamed(page, case, reads):
    """A room renamed, as a metadata event's load of the rooms brings it,
    without a load of the Cubes: its tile says the new name, and so does
    the House panel's list of rooms. Fails on the page where the House
    tile read the rooms' list untracked and followed a load of the Cubes
    alone."""
    with held_poll(page):
        rid = page.evaluate("state.rooms.find(roomThermostat).id")

        def rename(rooms):
            for r in rooms or []:
                if r.get("id") == rid:
                    r["name"] = "Renamed room"
            return rooms

        reads.change("**/rooms?devices=all", rename)
        page.evaluate("loadRooms()")
        assert wait_until(page, "state.rooms.some((r) => r.name === 'Renamed room')", 5), "the rooms read again"
        assert wait_until(page, "document.querySelector('#rooms').textContent.includes('Renamed room')", 5), "the tile follows"
        frames(page)
        house = page.evaluate("document.getElementById('house-rooms').textContent")
        assert "Renamed room" in house, f"the House panel's rooms follow: {house!r}"


def test_the_house_follows_a_room_removed(page, case, reads):
    """A room gone, as a metadata event's load of the rooms brings it: the
    House panel no longer offers it."""
    with held_poll(page):
        gone = page.evaluate("state.rooms.filter(roomThermostat)[0].name")
        rid = page.evaluate("state.rooms.filter(roomThermostat)[0].id")
        reads.change("**/rooms?devices=all", lambda rooms: [r for r in (rooms or []) if r.get("id") != rid])
        page.evaluate("loadRooms()")
        assert wait_until(page, "!state.rooms.some((r) => r.id === %d)" % rid, 5), "the rooms read again"
        frames(page)
        house = page.evaluate("document.getElementById('house-rooms').textContent")
        assert gone not in house, f"the House panel no longer offers {gone!r}: {house!r}"
