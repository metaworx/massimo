# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Cube settings keep what is typed until it is saved or reset, and say
where it is unsaved (AP UnsavedMarks, Block 1): Save plain and off while
nothing is changed, primary once something is, with the warning mark
before its text once the focus has left the form or a minute has passed,
turning while it saves, with the check for a while after a save and the
cross after a failed one, whether the server was not reached or refused
the value; Reset beside it. While a Cube's settings are unsaved, the
Cube's problem lists, in its panel's head and in the header, show the
disk, amber, red after a failed save, and a click on it opens the form
with the focus in it. What is typed goes with the page from System
settings to Cube settings and back, and is forgotten when the Cube is no
longer served.

The page runs under Playwright's clock (the marker clock), which the
tests move on for the minute and for the check's five seconds. No PUT
reaches the server: each is held, answered or refused by a route, so the
store and the fake Cube keep their settings."""
import json
import re

import pytest

from conftest import api_call
from helpers import hold, show_view, wait_held, wait_until

pytestmark = pytest.mark.clock

PANEL = "#cubes-list .cube-panel[data-serial='KEQ0000100']"
PAGE_PANEL = "#cube-page-body .cube-panel[data-serial='KEQ0000100']"
CUBE_PATH = "**/api/v1/cubes/KEQ0000100"
UNSAVED = {"en": "Cube settings: not saved", "de": "Cube-Einstellungen: nicht gespeichert"}
FAILED = {"en": "Cube settings: saving failed", "de": "Cube-Einstellungen: Speichern fehlgeschlagen"}
REFUSED = "address: not a host:port"


def open_panel(page):
    """System settings → Cubes with the fake Cube's panel open; its form."""
    show_view(page, "system")
    panel = page.locator(PANEL)
    if panel.get_attribute("open") is None:
        panel.locator(":scope > summary .cube-head-who").click()
    form = panel.locator("form.cube-form")
    form.wait_for(state="visible")
    return panel, form


def save_state(form):
    """Save's state: [off, primary, busy, the class of its first child's sign or ""]."""
    return form.locator("button[type=submit]").evaluate("""(b) => {
      const s = b.firstElementChild;
      const sign = !s ? "" : s.matches(".mark-sign.tone-warn") ? "warn" : s.matches(".mark-sign.tone-bad") ? "bad" : s.matches(".ok-check") ? "ok" : "?";
      return [b.disabled, b.classList.contains("primary"), b.classList.contains("busy"), sign];
    }""")


def leave(page):
    """The focus taken out of the form, by a click beside it in the
    panel, on the Cube's facts' corner, which takes no focus."""
    page.locator(PANEL + " .cube-info").click(position={"x": 1, "y": 1})


def item(page, where):
    """The unsaved item, a button, in a problem list; where its place."""
    return page.locator(where + " .problems-list .problem-action")


def wait_save(page, form, want, what):
    """Waits until Save is in the state want (save_state), what naming it."""
    for _ in range(25):
        if save_state(form) == want:
            return
        page.wait_for_timeout(200)
    assert save_state(form) == want, f"{what}: Save is {save_state(form)}, not {want}"


def test_save_shows_the_state(page, case):
    """Clean, changed, unsaved once the focus left, changed again at the
    next key, saving, then saved with the check, which goes after five
    seconds; Reset off while clean."""
    panel, form = open_panel(page)
    name = form.locator("input[name=name]")
    reset = form.locator(".form-reset")
    assert save_state(form) == [True, False, False, ""], "clean: off, plain, no sign"
    assert reset.is_disabled(), "Reset off while clean"
    name.fill("Typed name")
    wait_save(page, form, [False, True, False, ""], "changed: on, primary, no sign")
    assert reset.is_enabled(), "Reset on once changed"
    assert item(page, PANEL + " > summary .cube-pill").count() == 0, "no disk while the user is in the form"
    leave(page)
    wait_save(page, form, [False, True, False, "warn"], "left")
    assert form.locator("button[type=submit] > .mark-sign").count() == 1, "the sign a child of the button, before its text"
    pill = item(page, PANEL + " > summary .cube-pill")
    assert pill.count() == 1 and pill.text_content().strip() == UNSAVED[case.lang], f"the disk in the panel's pill: {pill.all_text_contents()}"
    assert page.locator(PANEL + " > summary .cube-pill .problems-sum use[href='#icon-disk-small']").count() == 1, "the disk in its summary"
    name.press("End")
    name.type("!")
    assert save_state(form) == [False, True, False, ""], "a key in the form: changed again"
    held, sent = [], []
    hold(page, CUBE_PATH, "PUT", held, sent)
    try:
        form.locator("button[type=submit]").click()
        assert wait_held(page, held), "the PUT is sent"
        assert save_state(form)[:3] == [True, True, True], "saving: off and turning"
        assert sent == [{"name": "Typed name!"}], f"the name: {sent}"
        for route in held:
            route.fulfill(status=200, content_type="application/json", body="{}")
        wait_save(page, form, [True, False, False, "ok"], "saved")
        assert name.input_value() == "Fake Cube", "the form shows the Cube as the server has it"
        assert item(page, PANEL + " > summary .cube-pill").count() == 0, "no disk once saved"
        page.clock.fast_forward(5100)
        wait_save(page, form, [True, False, False, ""], "five seconds after the save")
    finally:
        page.unroute(CUBE_PATH)


def test_unsaved_a_minute_after_the_last_change(page, case):
    """The focus kept in the form: amber a minute after the last change,
    not before; Reset takes the stored value back."""
    panel, form = open_panel(page)
    name = form.locator("input[name=name]")
    name.fill("Slow typist")
    page.clock.fast_forward(30_000)
    page.wait_for_timeout(300)
    assert save_state(form)[3] == "", "half a minute: still changed"
    name.type("s")
    page.clock.fast_forward(45_000)
    page.wait_for_timeout(300)
    assert save_state(form)[3] == "", "the minute starts again at each change"
    page.clock.fast_forward(16_000)
    wait_save(page, form, [False, True, False, "warn"], "a minute after the last change")
    assert name.evaluate("(e) => document.activeElement === e"), "the focus stayed in the field"
    form.locator(".form-reset").click()
    assert name.input_value() == "Fake Cube", "Reset takes the stored value"
    wait_save(page, form, [True, False, False, ""], "reset")


@pytest.mark.parametrize("how", ["not reached", "refused"])
def test_failed_save_turns_red(page, case, watch, how):
    """A save the server does not answer, and one it refuses: the cross on
    Save, the message line red with the error, the disk red; no field
    outlined; Reset takes the stored values and clears it."""
    panel, form = open_panel(page)
    address = form.locator("input[name=address]")

    def handle(route):
        if route.request.method != "PUT":
            route.continue_()
        elif how == "not reached":
            route.abort()
        else:
            route.fulfill(status=400, content_type="application/json", body=json.dumps({"error": REFUSED}))

    watch.expected.append(("/cubes/KEQ0000100", "ERR_FAILED" if how == "not reached" else 400))
    page.route(CUBE_PATH, handle)
    try:
        address.fill("not an address")
        form.locator("button[type=submit]").click()
        wait_save(page, form, [False, True, False, "bad"], "failed")
        line = page.locator("#cubes-msg")
        assert line.is_visible() and "KEQ0000100" in line.inner_text(), f"the message line says it: {line.inner_text()}"
        if how == "refused":
            assert REFUSED in line.inner_text(), "with the server's error"
        assert form.locator("[aria-invalid=true]").count() == 0, "no field outlined"
        assert address.input_value() == "not an address", "the draft kept"
        leave(page)
        pill = item(page, PANEL + " > summary .cube-pill")
        assert pill.count() == 1 and pill.text_content().strip() == FAILED[case.lang], f"the disk says it: {pill.all_text_contents()}"
        assert pill.locator(".flag.bad").count() == 1, "red"
        address.type("x")
        assert save_state(form)[3] == "bad", "a key keeps it red"
        form.locator(".form-reset").click()
        wait_save(page, form, [True, False, False, ""], "reset")
        assert address.input_value() == page.evaluate("state.cubes.find((c) => c.serial === 'KEQ0000100').configured_address || ''"), \
            "Reset takes the stored address"
        assert item(page, PANEL + " > summary .cube-pill").count() == 0, "no disk after Reset"
    finally:
        page.unroute(CUBE_PATH)


def test_typing_kept_and_the_disk_opens_it(page, case):
    """What is typed in System settings stays on Cube settings and back;
    from Rooms, the header's problem list shows the disk under the Cube,
    and a click on it opens Cube settings with the focus in the form."""
    panel, form = open_panel(page)
    form.locator("input[name=name]").fill("Kept across pages")
    form.locator("input[name=proxy]").fill("127.0.0.1:62999")
    show_view(page, "cube")
    page_form = page.locator(PAGE_PANEL + " form.cube-form")
    page_form.wait_for(state="visible")
    assert page_form.locator("input[name=name]").input_value() == "Kept across pages", "the name typed, on Cube settings"
    assert page_form.locator("input[name=proxy]").input_value() == "127.0.0.1:62999", "the proxy typed, on Cube settings"
    assert save_state(page_form)[3] == "warn", "the form taken away while changed: unsaved"
    show_view(page, "tiles")
    page.locator("#cube-problems .problems-sum").click()
    disk = item(page, "#cube-problems")
    assert disk.count() == 1 and disk.inner_text().strip() == UNSAVED[case.lang], f"the header's list shows the disk: {disk.all_inner_texts()}"
    disk.click()
    page.locator("#cube-page").wait_for(state="visible")
    name = page.locator(PAGE_PANEL + " form.cube-form input[name=name]")
    assert wait_until(page, "document.activeElement === document.querySelector(%s)" % json.dumps(PAGE_PANEL + " form.cube-form input[name=name]"), 5), \
        "the focus in the form's first field"
    assert name.input_value() == "Kept across pages", "with what was typed"
    show_view(page, "system")
    _, form = open_panel(page)
    assert form.locator("input[name=name]").input_value() == "Kept across pages", "and back in System settings"
    form.locator(".form-reset").click()
    assert page.locator("#cube-problems .problems-list .problem-action").count() == 0, "Reset takes the disk away"


def test_forgotten_when_the_cube_goes(page, case):
    """A Cube the list no longer gives takes what was typed for it: back
    in the list, its form shows the stored values and no disk."""
    panel, form = open_panel(page)
    form.locator("input[name=name]").fill("Typed for a Cube that goes")
    leave(page)
    wait_save(page, form, [False, True, False, "warn"], "left")

    def none(route):
        try:
            all_ = route.fetch().json()
            route.fulfill(status=200, content_type="application/json", body=json.dumps([c for c in all_ if c.get("configured") is False]))
        except Exception:  # the page closed
            pass

    page.route("**/api/v1/cubes", none)
    try:
        page.evaluate("loadCubes()")
        assert wait_until(page, "!state.cubes.length", 5), "the list gives no Cube"
    finally:
        page.unroute("**/api/v1/cubes")
    page.evaluate("loadCubes()")
    assert wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0000100')", 5), "the Cube back"
    panel, form = open_panel(page)
    assert form.locator("input[name=name]").input_value() == "Fake Cube", "its stored name"
    assert save_state(form) == [True, False, False, ""], "clean"
    assert item(page, PANEL + " > summary .cube-pill").count() == 0, "no disk"


@pytest.mark.parametrize("how", ["reset", "saved"])
def test_taken_away_after_reset_or_save(page, case, how):
    """A form reset and at once taken away, as a Cancel that resets and
    closes does (Cube settings shown in the same task as the click, so
    that the form is not drawn again before it goes), or one saved and
    then taken away: nothing is left unsaved, no disk anywhere, and the
    form shows the stored values where it is drawn next."""
    panel, form = open_panel(page)
    form.locator("input[name=name]").fill("Typed, then " + how)
    leave(page)
    wait_save(page, form, [False, True, False, "warn"], "left")
    page.route(CUBE_PATH, lambda route: route.fulfill(status=200, content_type="application/json", body="{}")
               if route.request.method == "PUT" else route.continue_())
    try:
        if how == "reset":
            page.evaluate("(sel) => { document.querySelector(sel).click(); showView('cube'); }", PANEL + " form.cube-form .form-reset")
        else:
            form.locator("button[type=submit]").click()
            wait_save(page, form, [True, False, False, "ok"], "saved")
            page.evaluate("() => showView('cube')")
        page_form = page.locator(PAGE_PANEL + " form.cube-form")
        page_form.wait_for(state="visible")
        page.wait_for_timeout(300)
        assert page_form.locator("input[name=name]").input_value() == "Fake Cube", "the stored name"
        assert save_state(page_form)[3] in ("", "ok"), f"no warning on Save: {save_state(page_form)}"
        assert page.locator(PAGE_PANEL + " > summary .cube-pill .problem-action").count() == 0, "no disk in the panel's pill"
        assert page.locator("#cube-problems .problem-action").count() == 0, "no disk in the header's list"
    finally:
        page.unroute(CUBE_PATH)



# --- the virtual wall thermostat's settings (AP UnsavedMarks, Block 2) ---

TILE = "#rooms .room"
VWT_UNSAVED = {"en": "{room}, virtual thermostat: not saved", "de": "{room}, virtueller Thermostat: nicht gespeichert"}
VWT_FAILED = {"en": "{room}, virtual thermostat: saving failed", "de": "{room}, virtueller Thermostat: Speichern fehlgeschlagen"}
OTHER_ADDRESS = "0a0101"
SECOND = "KEQ0000700"


def vwt_room(page):
    """The first room with a virtual wall thermostat: {id, name, path (its
    device's API path, a glob)}."""
    return page.evaluate("""() => {
      const r = state.rooms.find((x) => x.vwt);
      return {id: r.id, name: r.name, path: '**/api/v1/cubes/' + state.roomsOf + '/devices/' + r.vwt.serial};
    }""")


def at(room, sel=""):
    """A selector in the room's tile."""
    return f"{TILE}[data-id='{room['id']}'] {sel}".strip()


def expert(page, on=True):
    """Expert mode for this page alone, as the rooms' tests set it."""
    page.evaluate("(on) => { state.settings.expert = on; applySettings(); }", on)


def open_vwt(page, room):
    """The room's device list open with the virtual thermostat's settings
    drawn; its form."""
    tile = page.locator(at(room))
    if tile.locator(".devices .device.virtual").count() == 0:
        tile.locator(".tile-info").click()
    form = tile.locator(".devices .device.virtual form.vwt-settings")
    form.wait_for(state="attached")
    return form


def loaded(page):
    """Waits until the virtual thermostat's detail has come."""
    assert wait_until(page, "!!state.details[state.rooms.find((x) => x.vwt).vwt.rf_address]", 5), "the virtual thermostat's detail loads"
    page.wait_for_timeout(200)


def value(page, room, name):
    """A field's value in the room's virtual thermostat's form, read in the page."""
    return page.evaluate("(sel) => document.querySelector(sel).value", at(room, f".device.virtual [name={name}]"))


def leave_tile(page, room):
    """The focus taken out of the form, by a click on the tile's measured
    temperature, which takes no focus."""
    page.locator(at(room, ".temp-block")).first.click()


def row_disk(page, room):
    return item(page, at(room, ".device.virtual .device-badges"))


def tile_disk(page, room):
    return item(page, at(room, ".tile-head .room-problems"))


def unsaved(page, room):
    """The virtual thermostat's settings typed into and left: unsaved; the
    stored values [method, address] before."""
    expert(page)
    form = open_vwt(page, room)
    loaded(page)
    stored = [value(page, room, "temp_method"), value(page, room, "rf_address")]
    form.locator("input[name=rf_address]").fill(OTHER_ADDRESS)
    leave_tile(page, room)
    wait_save(page, form, [False, True, False, "warn"], "left")
    return form, stored


def drawings(page, ms):
    """How often, in ms, the virtual thermostat's form and the pills that
    name it were drawn: the calls of t() with their texts' keys."""
    return page.evaluate("""(ms) => new Promise((done) => {
      const orig = window.t;
      let n = 0;
      window.t = function (key, params) {
        if (key === "vwt.method" || key === "vwt.formName") n++;
        return orig.call(this, key, params);
      };
      setTimeout(() => { window.t = orig; done(n); }, ms);
    })""", ms)


def hold_detail(page, room, method=None):
    """Holds the GETs of the virtual thermostat's detail; release() lets
    them go, the method stored set to method where given."""
    held = []

    def handle(route):
        if route.request.method == "GET":
            held.append(route)
        else:
            route.continue_()

    def release():
        assert wait_held(page, held), "the detail is asked for"
        for route in list(held):
            res = route.fetch()
            body = res.json()
            if method:
                body.setdefault("settings", {})["temp_method"] = method
            route.fulfill(response=res, body=json.dumps(body))
        held.clear()

    page.route(room["path"], handle)
    return release


def test_vwt_settings_kept_and_the_disk_opens_them(page, case):
    """The virtual thermostat's settings: Save off while clean, Reset
    beside it; changed, then unsaved once the focus left, with the disk in
    the row's pill and the tile's; the page settles while it shows; what
    is typed stays when the device list closes and opens, a device
    reports and the rooms load again; a click on the tile's disk opens the
    list with the focus in the form; Reset takes the stored values back."""
    room = vwt_room(page)
    expert(page)
    try:
        form = open_vwt(page, room)
        loaded(page)
        method = form.locator("select[name=temp_method]")
        address = form.locator("input[name=rf_address]")
        stored = [method.input_value(), address.input_value()]
        assert save_state(form) == [True, False, False, ""], "clean: off, plain, no sign"
        assert form.locator(".form-reset").is_disabled(), "Reset off while clean"
        method.select_option("mean" if stored[0] != "mean" else "median")
        address.fill(OTHER_ADDRESS)
        wait_save(page, form, [False, True, False, ""], "changed")
        assert tile_disk(page, room).count() == 0, "no disk while the user is in the form"
        leave_tile(page, room)
        wait_save(page, form, [False, True, False, "warn"], "left")
        want = VWT_UNSAVED[case.lang].format(room=room["name"])
        for where, disk in (("row", row_disk(page, room)), ("tile", tile_disk(page, room))):
            assert disk.count() == 1 and disk.text_content().strip() == want, f"the disk in the {where}'s pill: {disk.all_text_contents()}"
        n = drawings(page, 2000)
        assert n < 10, f"the page settles while the form is unsaved: {n} drawings in 2 s"
        page.evaluate("(id) => MassimoParts.rooms.data(id)", room["id"])  # a device of the room reports
        page.evaluate("loadRooms()")
        page.wait_for_timeout(300)
        assert value(page, room, "rf_address") == OTHER_ADDRESS, "kept at a device's report and a load of the rooms"
        assert save_state(form)[3] == "warn", "still unsaved"
        page.locator(at(room, ".tile-info")).click()
        assert page.locator(at(room, ".device.virtual")).count() == 0, "the device list closed"
        assert tile_disk(page, room).count() == 1, "the tile's disk stays while the list is closed"
        form = open_vwt(page, room)
        assert value(page, room, "temp_method") != stored[0], "the method chosen, kept"
        assert value(page, room, "rf_address") == OTHER_ADDRESS, "the address typed, kept"
        assert save_state(form)[3] == "warn", "still unsaved"
        page.locator(at(room, ".tile-info")).click()
        page.locator(at(room, ".tile-head .room-problems .problems-sum")).click()
        tile_disk(page, room).click()
        sel = at(room, ".device.virtual form.vwt-settings select[name=temp_method]")
        assert wait_until(page, "document.activeElement === document.querySelector(%s)" % json.dumps(sel), 5), "the focus in the form's first field"
        form = open_vwt(page, room)
        assert value(page, room, "rf_address") == OTHER_ADDRESS, "with what was typed"
        form.locator(".form-reset").click()
        wait_save(page, form, [True, False, False, ""], "reset")
        assert [value(page, room, "temp_method"), value(page, room, "rf_address")] == stored, "Reset takes the stored values"
        assert row_disk(page, room).count() == 0 and tile_disk(page, room).count() == 0, "no disk after Reset"
    finally:
        expert(page, False)


def test_vwt_settings_outside_expert_mode(page, case):
    """Outside expert mode, the row and the disk show while the settings
    are unsaved; reset, the row is an expert's again."""
    room = vwt_room(page)
    try:
        form, _ = unsaved(page, room)
        expert(page, False)
        row = page.locator(at(room, ".device.virtual"))
        assert row.is_visible(), "the row shown while unsaved"
        assert tile_disk(page, room).count() == 1, "the tile's disk"
        form.locator(".form-reset").click()
        assert wait_until(page, "!document.querySelector(%s).checkVisibility()" % json.dumps(at(room, ".device.virtual")), 5), \
            "the row hidden again after Reset"
        assert tile_disk(page, room).count() == 0, "no disk"
    finally:
        expert(page, False)


def test_vwt_settings_saved_and_failed(page, case, watch):
    """A save sends both settings, shows the check and the stored values as
    the server gives them again, with no "changed meanwhile"; one taken
    whose detail cannot be had again shows what was sent; one not reached
    turns Save and the disks red, and Reset clears it."""
    room = vwt_room(page)
    expert(page)
    held, sent = [], []
    config = room["path"] + "/config"
    try:
        form = open_vwt(page, room)
        loaded(page)
        method = form.locator("select[name=temp_method]")
        stored = method.input_value()
        other = "mean" if stored != "mean" else "median"
        method.select_option(other)
        hold(page, config, "PUT", held, sent)
        form.locator("button[type=submit]").click()
        assert wait_held(page, held), "the PUT is sent"
        assert save_state(form)[:3] == [True, True, True], "saving: off and turning"
        assert sent == [{"temp_method": other, "rf_address": form.locator("input[name=rf_address]").input_value()}], f"both settings: {sent}"
        for route in held:
            route.fulfill(status=200, content_type="application/json", body="{}")
        wait_save(page, form, [True, False, False, "ok"], "saved")
        assert method.input_value() == stored, "the form shows the settings as the server has them"
        assert form.locator(".field-meanwhile").count() == 0, "no \"changed meanwhile\" for the detail asked for again"
        page.unroute(config)
        # taken, its detail not had again: what was sent
        watch.expected.append(("/" + room["path"].rsplit("/", 1)[1], "ERR_FAILED"))
        page.route(config, lambda route: route.fulfill(status=200, content_type="application/json", body="{}"))
        page.route(room["path"], lambda route: route.abort() if route.request.method == "GET" else route.continue_())
        page.clock.fast_forward(5100)
        method.select_option(other)
        form.locator("button[type=submit]").click()
        wait_save(page, form, [True, False, False, "ok"], "saved, the detail not had")
        assert method.input_value() == other, "the form shows what was sent"
        assert form.locator(".field-meanwhile").count() == 0, "no \"changed meanwhile\""
        page.unroute(room["path"])
        page.unroute(config)
        watch.expected.append(("/config", "ERR_FAILED"))
        page.route(config, lambda route: route.abort() if route.request.method == "PUT" else route.continue_())
        form.locator("input[name=rf_address]").fill(OTHER_ADDRESS)
        form.locator("button[type=submit]").click()
        wait_save(page, form, [False, True, False, "bad"], "failed")
        leave_tile(page, room)
        disk = tile_disk(page, room)
        assert disk.count() == 1 and disk.text_content().strip() == VWT_FAILED[case.lang].format(room=room["name"]), \
            f"the tile's disk says it: {disk.all_text_contents()}"
        assert disk.locator(".flag.bad").count() == 1, "red"
        form.locator(".form-reset").click()
        wait_save(page, form, [True, False, False, ""], "reset")
        assert tile_disk(page, room).count() == 0, "no disk after Reset"
    finally:
        page.unroute(config)
        page.unroute(room["path"])
        expert(page, False)


def test_vwt_settings_wait_for_their_values(page, case):
    """Before the first detail the fields and Save are off, and a draft
    starts from the stored values, not from what is shown meanwhile; typed
    after it, kept. A list opened again shows the draft at once, before
    the detail comes again, its marks with it."""
    room = vwt_room(page)
    expert(page)
    release = hold_detail(page, room, method="mean")
    try:
        page.evaluate("() => { state.details = {}; }")
        form = open_vwt(page, room)
        page.wait_for_timeout(300)
        assert form.locator("select[name=temp_method]").is_disabled() and form.locator("input[name=rf_address]").is_disabled(), \
            "the fields off before the settings came"
        assert save_state(form)[0], "Save off"
        release()
        assert wait_until(page, "document.querySelector(%s).value === 'mean'" % json.dumps(at(room, ".device.virtual select[name=temp_method]")), 5), \
            "the stored method shown"
        assert form.locator("input[name=rf_address]").is_enabled(), "the fields on"
        assert save_state(form) == [True, False, False, ""], "clean once the settings came"
        assert form.locator(".field-meanwhile").count() == 0, "no \"changed meanwhile\" for the value shown meanwhile"
        form.locator("input[name=rf_address]").fill(OTHER_ADDRESS)
        leave_tile(page, room)
        wait_save(page, form, [False, True, False, "warn"], "left")
        page.locator(at(room, ".tile-info")).click()
        page.evaluate("() => { state.details = {}; }")
        form = open_vwt(page, room)
        assert value(page, room, "rf_address") == OTHER_ADDRESS, "the draft at once, before the detail came again"
        assert value(page, room, "temp_method") == "mean", "the method stored, not the placeholder"
        assert save_state(form)[3] == "warn", "still unsaved"
        assert tile_disk(page, room).count() == 1, "the disk stays"
        release()
        page.wait_for_timeout(300)
        assert value(page, room, "rf_address") == OTHER_ADDRESS, "kept once it came"
        form.locator(".form-reset").click()
        wait_save(page, form, [True, False, False, ""], "reset")
    finally:
        page.unroute(room["path"])
        expert(page, False)


def test_vwt_disk_under_its_cube(page, case):
    """From System settings, the Cube's problem lists, in its panel's head
    and in the header, show the room's disk; a click on the header's shows
    the rooms, opens the device list and puts the focus into the form.
    With another Cube chosen (a second Cube added to the server, which
    never connects and answers no rooms), the click chooses the room's
    Cube first."""
    room = vwt_room(page)
    want = VWT_UNSAVED[case.lang].format(room=room["name"])
    sel = json.dumps(at(room, ".device.virtual form.vwt-settings select[name=temp_method]"))
    added = False
    try:
        unsaved(page, room)
        expert(page, False)
        page.locator(at(room, ".tile-info")).click()
        show_view(page, "system")
        panel_disk = item(page, PANEL + " > summary .cube-pill")
        assert want in panel_disk.all_text_contents(), f"the panel head's disk: {panel_disk.all_text_contents()}"
        page.locator("#cube-problems .problems-sum").click()
        disk = page.locator("#cube-problems .problems-list .problem-action", has_text=want)
        assert disk.count() == 1, f"the header's disk: {item(page, '#cube-problems').all_text_contents()}"
        disk.click()
        page.locator("#rooms").wait_for(state="visible")
        assert wait_until(page, "document.activeElement === document.querySelector(%s)" % sel, 5), "the rooms, the list, the focus in the form"
        assert value(page, room, "rf_address") == OTHER_ADDRESS, "with what was typed"

        api_call("POST", "api/v1/cubes", {"serial": SECOND, "address": "127.0.0.1:9", "name": "Second"})
        added = True
        page.evaluate("loadCubes()")
        assert wait_until(page, "state.cubes.some((c) => c.serial === %s)" % json.dumps(SECOND), 5), "the second Cube listed"
        page.evaluate("(s) => selectCube(s)", SECOND)
        assert wait_until(page, "state.roomsOf === %s" % json.dumps(SECOND), 5), "the second Cube's rooms shown"
        show_view(page, "system")
        page.locator("#cube-problems .problems-sum").click()
        disk = page.locator("#cube-problems .problems-list .problem-action", has_text=want)
        assert disk.count() == 1, "the disk under its Cube while another is chosen"
        disk.click()
        assert wait_until(page, "state.serial === 'KEQ0000100' && state.roomsOf === 'KEQ0000100'", 5), "its Cube chosen"
        assert wait_until(page, "document.activeElement === document.querySelector(%s)" % sel, 5), "the list opened, the focus in the form"
        assert value(page, room, "rf_address") == OTHER_ADDRESS, "the draft kept across the choice"
        open_vwt(page, room).locator(".form-reset").click()
    finally:
        page.evaluate("() => { if (state.serial !== 'KEQ0000100') selectCube('KEQ0000100'); }")
        if added:
            api_call("DELETE", "api/v1/cubes/" + SECOND)
        page.evaluate("loadCubes()")
        expert(page, False)


@pytest.mark.parametrize("gone", ["vwt", "room", "cube", "disconnected"])
def test_vwt_settings_forgotten(page, case, gone):
    """What is typed is forgotten once a load no longer gives the virtual
    thermostat, its room or its Cube: back, its form shows the stored
    values, clean, and no disk. A load from a Cube that is not connected
    (one not greeted answers no rooms) forgets nothing."""
    room = vwt_room(page)
    try:
        form, stored = unsaved(page, room)
        rooms = re.compile(r"/api/v1/cubes/[^/]+/rooms\?devices=all$")

        def without(route):
            try:
                all_ = route.fetch().json()
                if gone in ("room", "disconnected"):
                    all_ = [] if gone == "disconnected" else [r for r in all_ if r["id"] != room["id"]]
                else:
                    for r in all_:
                        if r["id"] == room["id"]:
                            r["devices"] = [d for d in r["devices"] if not d.get("virtual")]
                route.fulfill(status=200, content_type="application/json", body=json.dumps(all_))
            except Exception:  # the page closed
                pass

        def no_cubes(route):
            try:
                all_ = route.fetch().json()
                route.fulfill(status=200, content_type="application/json", body=json.dumps([c for c in all_ if c.get("configured") is False]))
            except Exception:  # the page closed
                pass

        if gone == "cube":
            page.route("**/api/v1/cubes", no_cubes)
            try:
                page.evaluate("loadCubes()")
                assert wait_until(page, "!state.cubes.length", 5), "the list gives no Cube"
            finally:
                page.unroute("**/api/v1/cubes")
            page.evaluate("refresh()")
            assert wait_until(page, "state.roomsOf && state.rooms.some((r) => r.vwt)", 5), "the Cube and its rooms back"
        else:
            if gone == "disconnected":
                page.evaluate("() => { state.cubes.find((c) => c.serial === state.roomsOf).connected = false; }")
            page.route(rooms, without)
            try:
                page.evaluate("loadRooms()")
                assert wait_until(page, "!state.rooms.some((r) => r.id === %d && r.vwt)" % room["id"], 5), "the load gives it no more"
            finally:
                page.unroute(rooms)
            page.evaluate("refresh()")
            assert wait_until(page, "state.rooms.some((r) => r.id === %d && r.vwt)" % room["id"], 5), "back"
        expert(page)
        form = open_vwt(page, room)
        loaded(page)
        if gone == "disconnected":
            assert value(page, room, "rf_address") == OTHER_ADDRESS, "the draft kept"
            assert save_state(form)[3] == "warn", "still unsaved"
            form.locator(".form-reset").click()
            return
        assert value(page, room, "rf_address") == stored[1], "the stored address"
        assert save_state(form) == [True, False, False, ""], "clean"
        assert row_disk(page, room).count() == 0 and tile_disk(page, room).count() == 0, "no disk"
    finally:
        expert(page, False)
