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

Leaving the page (Block 3): while a form has changes, also one still
being typed in, an attempt to close or reload the page asks with the
browser's prompt; staying, the page's dialog lists the forms, each
opening its form, and the form is unsaved; leaving drops the draft. Sign
out asks with the same dialog; moving inside the page asks nothing.
Playwright answers the browser's prompt (page.on("dialog")): dismissed,
the page stays, as a user's Stay; page.close(run_before_unload=True)
asks it without waiting for a navigation.

The page runs under Playwright's clock (the marker clock), which the
tests move on for the minute and for the check's five seconds. No PUT
reaches the server: each is held, answered or refused by a route, so the
store and the fake Cube keep their settings."""
import json
import re
import time

import pytest

from conftest import URL, api_call
from helpers import CUBES, choose_cube, describe_event, found_only, held_poll, hold, show_view, sign_out, wait_held, wait_until
from test_login import sign_in

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


def test_forgotten_when_the_cube_goes(page, case, reads):
    """A Cube the list no longer gives takes what was typed for it: back
    in the list, its form shows the stored values and no disk."""
    panel, form = open_panel(page)
    form.locator("input[name=name]").fill("Typed for a Cube that goes")
    leave(page)
    wait_save(page, form, [False, True, False, "warn"], "left")

    reads.change(CUBES, found_only)
    try:
        page.evaluate("loadCubes()")
        assert wait_until(page, "!state.cubes.length", 5), "the list gives no Cube"
    finally:
        reads.clear(CUBES)
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
STORED_ELSEWHERE = "0a0102"  # the virtual thermostat's address as another client saved it meanwhile
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


def drawings(page, ms, poll=False):
    """How often the virtual thermostat's form (its key vwt.method) and
    the pills that name it (vwt.formName) were drawn, by the calls of t()
    with those keys, as {key: count}: with poll, while a poll of the
    page's own (poll(false)) runs, awaited; then in ms."""
    return page.evaluate("""async ([ms, polled]) => {
      const orig = window.t;
      const n = {};
      window.t = function (key, params) {
        if (key === "vwt.method" || key === "vwt.formName") n[key] = (n[key] || 0) + 1;
        return orig.call(this, key, params);
      };
      try {
        if (polled) await poll(false);
        await new Promise((done) => setTimeout(done, ms));
      } finally {
        window.t = orig;
      }
      return n;
    }""", [ms, poll])


def answered(page, action):
    """Runs action(), then waits until every request it started has been
    answered, the stream's left out: what the page draws from their
    answers is drawn then, those the action did not await among them (a
    poll's check of the version, its log alerts). Returns action's
    result."""
    started, ended = [], []

    def start(r):
        if "/events?" not in r.url:
            started.append(r)

    def end(r):
        ended.append(r)

    page.on("request", start)
    page.on("requestfinished", end)
    page.on("requestfailed", end)
    try:
        result = action()
        until = time.time() + 5
        while any(r not in ended for r in started) and time.time() < until:
            page.wait_for_timeout(50)
        assert all(r in ended for r in started), f"the requests answered: {[r.url for r in started if r not in ended]}"
    finally:
        page.remove_listener("request", start)
        page.remove_listener("requestfinished", end)
        page.remove_listener("requestfailed", end)
    page.evaluate("new Promise((done) => requestAnimationFrame(() => setTimeout(done)))")  # their answers handled
    return result


def came(stream, mark):
    """The events the page's stream brought since mark, in a few words
    each, for a failure's message: a stray event of the server's can then
    be told from a fault."""
    seen = stream.since(mark)
    return f"{len(seen)} events on the stream meanwhile" + (": " + ", ".join(describe_event(e) for e in seen[-20:]) if seen else "")


def stored_elsewhere(detail):
    """The virtual thermostat's detail with the address another client
    saved meanwhile."""
    detail.setdefault("settings", {})["rf_address"] = STORED_ELSEWHERE
    return detail


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


def test_vwt_settings_kept_and_the_disk_opens_them(page, case, reads, stream):
    """The virtual thermostat's settings: Save off while clean, Reset
    beside it; changed, then unsaved once the focus left, with the disk in
    the row's pill and the tile's; the page settles while it shows, the
    page's poll held: nothing draws them with nothing changed, a poll a
    few times and then nothing; what is typed stays when a device reports
    and the rooms load again with the address stored elsewhere meanwhile,
    which the form then says under the field, and when the device list
    closes and opens; a click on the tile's disk opens the list with the
    focus in the form; Reset takes the stored values back."""
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
        # the page's poll held while the drawings are counted, so that the
        # count does not depend on its period: a poll draws the pills again
        with held_poll(page):
            mark = stream.mark()
            n = drawings(page, 2000)
            assert not n, f"the page settles while the form is unsaved: drawn {n} in 2 s with nothing changed; {came(stream, mark)}"
            n = answered(page, lambda: drawings(page, 0, poll=True))
            assert sum(n.values()) < 10, f"a poll draws the pills a few times: {n}"
            mark = stream.mark()
            n = drawings(page, 1000)
            assert not n, f"and then nothing: drawn {n} in the second after the poll; {came(stream, mark)}"
        # the address stored elsewhere meanwhile, as another client's save
        # does: the load reads the detail anew, the details forgotten as a
        # save of another form forgets them (config.js)
        meanwhile = page.evaluate("(v) => t('field.changedMeanwhile', {value: v})", STORED_ELSEWHERE)
        field_meanwhile = at(room, ".device.virtual form.vwt-settings .field-meanwhile")
        reads.change(room["path"], stored_elsewhere)
        try:
            page.evaluate("state.details = {}")
            page.evaluate("(id) => { MassimoParts.rooms.data(id); return loadRooms(); }", room["id"])  # a device reports, the rooms load
            assert wait_until(page, "[...document.querySelectorAll(%s)].some((e) => e.textContent.includes(%s))"
                              % (json.dumps(field_meanwhile), json.dumps(meanwhile)), 5), "the form says the address stored meanwhile"
            assert value(page, room, "rf_address") == OTHER_ADDRESS, "the draft kept at a device's report and a load of the rooms"
            assert save_state(form)[3] == "warn", "still unsaved"
        finally:
            reads.clear(room["path"])
            page.evaluate("state.details = {}")
            page.evaluate("loadRooms()")
        assert wait_until(page, "!document.querySelector(%s)" % json.dumps(field_meanwhile), 5), "the address stored back, nothing said"
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
def test_vwt_settings_forgotten(page, case, gone, reads):
    """What is typed is forgotten once a load no longer gives the virtual
    thermostat, its room or its Cube: back, its form shows the stored
    values, clean, and no disk. A load from a Cube that is not connected
    (one not greeted answers no rooms) forgets nothing."""
    room = vwt_room(page)
    try:
        form, stored = unsaved(page, room)
        rooms = re.compile(r"/api/v1/cubes/[^/]+/rooms\?devices=all$")

        def without(all_):
            if gone in ("room", "disconnected"):
                return [] if gone == "disconnected" else [r for r in all_ if r["id"] != room["id"]]
            for r in all_:
                if r["id"] == room["id"]:
                    r["devices"] = [d for d in r["devices"] if not d.get("virtual")]
            return all_

        if gone == "cube":
            reads.change(CUBES, found_only)
            try:
                page.evaluate("loadCubes()")
                assert wait_until(page, "!state.cubes.length", 5), "the list gives no Cube"
            finally:
                reads.clear(CUBES)
            page.evaluate("refresh()")
            assert wait_until(page, "state.roomsOf && state.rooms.some((r) => r.vwt)", 5), "the Cube and its rooms back"
        else:
            if gone == "disconnected":
                reads.cube(page.evaluate("state.roomsOf"), connected=False)
                page.evaluate("loadCubes()")
            reads.change(rooms, without)
            try:
                page.evaluate("loadRooms()")
                assert wait_until(page, "!state.rooms.some((r) => r.id === %d && r.vwt)" % room["id"], 5), "the load gives it no more"
            finally:
                reads.clear(rooms)
                reads.clear(CUBES)
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


# --- leaving the page (AP UnsavedMarks, Block 3) ---

DIALOG = "#leaving dialog.forms-dialog"
SIGN_OUT = {"en": "Sign out", "de": "Abmelden"}
FORGET_KEY = {"en": "Forget key", "de": "Schlüssel vergessen"}


class Prompts(list):
    """The types of the browser's prompts on leaving the page, in order;
    accept says how the next is answered."""
    accept = False


def prompts(page, accept=False):
    """Records the browser's prompts on leaving the page, each answered:
    accepted, the page left, or dismissed, the user staying, as the list's
    accept says. Playwright dismisses a prompt nobody listens for."""
    seen = Prompts()
    seen.accept = accept

    def answer(d):
        seen.append(d.type)
        if seen.accept:
            d.accept()
        else:
            d.dismiss()

    page.on("dialog", answer)
    return seen


def dialog_open(page):
    return page.evaluate("(sel) => !!document.querySelector(sel)?.open", DIALOG)


def dialog_forms(page):
    """The texts of the forms the page's dialog lists."""
    return [s.strip() for s in page.locator(DIALOG + " .dialog-form").all_text_contents()]


def stay_after_prompt(page, seen):
    """An attempt to close the page, its prompt dismissed: the user stays,
    and the page's dialog shows."""
    n = len(seen)
    page.close(run_before_unload=True)
    for _ in range(25):
        if len(seen) > n:
            break
        page.wait_for_timeout(200)
    assert seen[n:] == ["beforeunload"], f"the browser's prompt asks: {seen}"
    assert not dialog_open(page), "not at once: a moment after the prompt"
    assert wait_until(page, "!!document.querySelector(%s)?.open" % json.dumps(DIALOG), 5), "the page's dialog shows where the user stays"


def listed(page):
    """Waits until the page lists a form with changes: a form drawn again
    after the key that changed it, a frame or two."""
    assert wait_until(page, "MassimoParts.leaving.changes()", 5), "the form listed as changed"


def reloaded(page):
    """Waits until the page loaded again has its Cubes and its stream."""
    assert wait_until(page, "typeof state === 'object' && state.cubes.length > 0 && state.eventsOpen", 10), "the page loads again"


def focused(page, sel):
    return wait_until(page, "document.activeElement === document.querySelector(%s)" % json.dumps(sel), 5)


def activated(page):
    """A click on the page's name in the header, which does nothing: the
    page has had a click, without which a browser shows no prompt on
    leaving, whatever the page asks."""
    page.locator(".brand h1").click()
    assert page.evaluate("navigator.userActivation.hasBeenActive"), "the page has had a click"


@pytest.mark.parametrize("how", ["untouched", "reset"])
def test_leaving_without_changes_asks_nothing(page, case, how):
    """With no form changed, or one changed and reset, a reload of a page
    that has had a click asks nothing: no prompt, no dialog."""
    seen = prompts(page)
    activated(page)
    if how == "reset":
        _, form = open_panel(page)
        form.locator("input[name=name]").fill("Typed, then reset")
        form.locator(".form-reset").click()
        wait_save(page, form, [True, False, False, ""], "reset")
    page.reload()
    reloaded(page)
    assert seen == [], f"no prompt without changes: {seen}"
    assert not dialog_open(page), "and no dialog"


def test_leaving_with_a_changed_form_asks(page, case):
    """A form changed and still focused: an attempt to close the page asks
    through the browser's prompt; staying, the page's dialog lists it,
    the focus on its Close, and the form is unsaved now, with its sign and
    its disk. Close gives the focus back. From Rooms, the dialog's button
    of the form closes it and opens Cube settings with the focus in the
    form and what was typed."""
    seen = prompts(page)
    _, form = open_panel(page)
    name = form.locator("input[name=name]")
    name.click()
    name.press("End")
    page.keyboard.type(" typed")
    wait_save(page, form, [False, True, False, ""], "changed, the focus in the form")
    stay_after_prompt(page, seen)
    assert dialog_forms(page) == [UNSAVED[case.lang]], f"the form in the dialog: {dialog_forms(page)}"
    assert focused(page, DIALOG + " .dialog-stay"), "the focus on Close"
    assert page.evaluate("(sel) => document.querySelector(sel).matches(':modal')", DIALOG), "a modal dialog"
    assert page.locator(DIALOG + " .dialog-go").count() == 0, "nothing to go on to"
    wait_save(page, form, [False, True, False, "warn"], "unsaved after the attempt")
    assert item(page, PANEL + " > summary .cube-pill").count() == 1, "with its disk"
    page.locator(DIALOG + " .dialog-stay").click()
    assert not dialog_open(page), "Close closes it"
    assert focused(page, PANEL + " form.cube-form input[name=name]"), "and gives the focus back"

    show_view(page, "tiles")
    stay_after_prompt(page, seen)
    page.keyboard.press("Escape")
    assert not dialog_open(page), "Escape closes it"
    stay_after_prompt(page, seen)
    page.locator(DIALOG + " .dialog-form").click()
    assert not dialog_open(page), "the form's button closes it"
    page.locator("#cube-page").wait_for(state="visible")
    assert focused(page, PAGE_PANEL + " form.cube-form input[name=name]"), "Cube settings, the focus in the form"
    assert page.locator(PAGE_PANEL + " form.cube-form input[name=name]").input_value() == "Fake Cube typed", "with what was typed"
    page.locator(PAGE_PANEL + " form.cube-form .form-reset").click()
    assert not page.evaluate("MassimoParts.leaving.changes()"), "reset: nothing to ask about"


def test_leaving_accepted_drops_the_draft(page, case):
    """The prompt accepted, the page reloads, and what was typed is gone."""
    seen = prompts(page, accept=True)
    _, form = open_panel(page)
    form.locator("input[name=name]").click()
    page.keyboard.type("Dropped")
    listed(page)
    page.reload()
    reloaded(page)
    assert seen == ["beforeunload"], f"the browser's prompt asked: {seen}"
    assert not dialog_open(page), "no dialog on the page that comes back"
    _, form = open_panel(page)
    assert form.locator("input[name=name]").input_value() == "Fake Cube", "the stored name"
    assert save_state(form) == [True, False, False, ""], "clean"


def test_moving_inside_asks_nothing(page, case):
    """With a form changed, another page of the app and another Cube
    (a second Cube added to the server, which never connects) ask
    nothing: no prompt, no dialog, and the draft stays."""
    seen = prompts(page)
    added = False
    try:
        _, form = open_panel(page)
        form.locator("input[name=name]").click()
        page.keyboard.type("Kept")
        for view in ("tiles", "cube", "log", "me", "system"):
            show_view(page, view)
        api_call("POST", "api/v1/cubes", {"serial": SECOND, "address": "127.0.0.1:9", "name": "Second"})
        added = True
        page.evaluate("loadCubes()")
        assert wait_until(page, "state.cubes.some((c) => c.serial === %s)" % json.dumps(SECOND), 5), "the second Cube listed"
        show_view(page, "tiles")
        choose_cube(page, SECOND)
        assert wait_until(page, "state.serial === %s" % json.dumps(SECOND), 5), "the second Cube chosen"
        page.wait_for_timeout(300)
        assert seen == [] and not dialog_open(page), f"nothing asked: {seen}"
        assert page.evaluate("MassimoParts.leaving.changes()"), "the draft kept"
    finally:
        page.evaluate("() => { if (state.serial !== 'KEQ0000100') selectCube('KEQ0000100'); }")
        if added:
            api_call("DELETE", "api/v1/cubes/" + SECOND)
        page.evaluate("loadCubes()")


def changed_cube_form(page, text):
    """Cube settings in System settings with the name typed into; the form."""
    _, form = open_panel(page)
    name = form.locator("input[name=name]")
    name.click()
    name.press("End")
    page.keyboard.type(text)
    listed(page)
    return form


def test_leaving_dialog_waits_for_a_page_that_goes(page, case):
    """The dialog of a stay comes a moment after the prompt, not at once:
    accepted, with the next page held for half a second, the page that
    goes never shows it. Where the page stays, the dialog that comes is
    taken back as the page is hidden for the browser's back button."""
    seen = prompts(page, accept=True)
    changed_cube_form(page, " slow")
    page.evaluate("""(sel) => setInterval(() => {
      if (document.querySelector(sel)?.open) localStorage.setItem('ui-test.leavingShown', '1');
    }, 20)""", DIALOG)

    def slow(route):
        time.sleep(0.5)
        route.continue_()

    page.route(re.compile(r"/(#.*)?$"), slow)
    try:
        page.reload()
        reloaded(page)
    finally:
        page.unroute(re.compile(r"/(#.*)?$"))
    assert seen == ["beforeunload"], f"the prompt asked: {seen}"
    assert page.evaluate("localStorage.getItem('ui-test.leavingShown')") is None, "no dialog on the page that went"
    page.evaluate("localStorage.removeItem('ui-test.leavingShown')")

    seen.accept = False
    changed_cube_form(page, " kept")
    stay_after_prompt(page, seen)
    page.evaluate("dispatchEvent(new PageTransitionEvent('pagehide', {persisted: true}))")
    assert wait_until(page, "!document.querySelector(%s).open" % json.dumps(DIALOG), 5), "taken back as the page is hidden"
    assert page.evaluate("MassimoParts.leaving.changes()"), "the draft kept"


def test_leaving_dialog_shows_a_failed_form_and_closes_itself(page, case, watch):
    """A form whose save failed is listed in red; a save that goes through
    while the dialog is open closes it, and so does a Reset."""
    seen = prompts(page)
    watch.expected.append(("/cubes/KEQ0000100", "ERR_FAILED"))
    page.route(CUBE_PATH, lambda route: route.abort() if route.request.method == "PUT" else route.continue_())
    try:
        form = changed_cube_form(page, " failed")
        form.locator("button[type=submit]").click()
        wait_save(page, form, [False, True, False, "bad"], "failed")
    finally:
        page.unroute(CUBE_PATH)
    stay_after_prompt(page, seen)
    assert dialog_forms(page) == [FAILED[case.lang]], f"the failed form: {dialog_forms(page)}"
    assert page.locator(DIALOG + " .dialog-form .sign.tone-bad").count() == 1, "in red"
    page.evaluate("(sel) => document.querySelector(sel).click()", PANEL + " form.cube-form .form-reset")
    assert wait_until(page, "!document.querySelector(%s).open" % json.dumps(DIALOG), 5), "a Reset closes it"

    held, sent = [], []
    hold(page, CUBE_PATH, "PUT", held, sent)
    try:
        form = changed_cube_form(page, " saving")
        form.locator("button[type=submit]").click()
        assert wait_held(page, held), "the PUT is sent"
        stay_after_prompt(page, seen)
        assert dialog_forms(page) == [UNSAVED[case.lang]], f"the form saving is listed: {dialog_forms(page)}"
        for route in held:
            route.fulfill(status=200, content_type="application/json", body="{}")
        assert wait_until(page, "!document.querySelector(%s).open" % json.dumps(DIALOG), 5), "a save that went through closes it"
    finally:
        page.unroute(CUBE_PATH)


def test_sign_out_with_a_key_says_forget_key(page, case):
    """With the owner's key, the dialog's button says Forget key, as the
    page's own; Stay keeps the key."""
    changed_cube_form(page, " key")
    sign_out(page)
    assert wait_until(page, "!!document.querySelector(%s)?.open" % json.dumps(DIALOG), 5), "the dialog asks"
    assert page.locator(DIALOG + " .dialog-go").inner_text().strip() == FORGET_KEY[case.lang], "Forget key"
    page.locator(DIALOG + " .dialog-stay").click()
    assert not dialog_open(page), "Stay closes it"
    assert page.evaluate("!!state.token && !!state.me"), "the key kept"
    show_view(page, "system")
    _, form = open_panel(page)
    form.locator(".form-reset").click()


def expire(page):
    """The session taken as expired: a token the server does not know, and
    a request that meets its 401, which asks for a sign-in."""
    page.evaluate("() => { setToken('expired-session'); loadMe(); }")
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in asked for"


def test_sign_in_after_a_401_keeps_drafts_for_the_same_user(new_context, case, watch, guard):
    """A 401 asks for a sign-in and asks nothing about the drafts: the same
    user signed in again finds them, another user does not."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    sign_in(page, "operator", "op-pass")
    room = vwt_room(page)
    unsaved(page, room)
    expire(page)
    sign_in(page, "operator", "op-pass")
    assert page.evaluate("MassimoParts.leaving.changes()"), "the same user: the drafts kept"
    expire(page)
    sign_in(page, "guest1", "guest-pass")
    assert page.evaluate("state.me.user.name") == "guest1", "another user"
    assert not page.evaluate("MassimoParts.leaving.changes()"), "the first user's drafts forgotten"


def test_sign_out_asks_first(new_context, case, watch, guard):
    """The operator, signed in, with the virtual thermostat's settings
    changed: Sign out asks with the page's dialog, which lists them;
    Stay keeps the session and the draft; Sign out there signs out and
    the draft is gone. Signed in again, without changes, Sign out asks
    nothing; with a save under way, it asks, and once the save has gone
    through the sign-out goes on. A page of its own, without the test's token, as
    test_login.py's."""
    ctx = new_context()  # the case's viewport and locale
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    seen = prompts(page)
    page.goto(URL)
    sign_in(page, "operator", "op-pass")
    room = vwt_room(page)
    form, stored = unsaved(page, room)
    want = VWT_UNSAVED[case.lang].format(room=room["name"])
    sign_out(page)
    assert wait_until(page, "!!document.querySelector(%s)?.open" % json.dumps(DIALOG), 5), "the dialog asks"
    assert dialog_forms(page) == [want], f"listing the form: {dialog_forms(page)}"
    assert page.locator(DIALOG + " .dialog-go").inner_text().strip() == SIGN_OUT[case.lang], "its Sign out"
    assert focused(page, DIALOG + " .dialog-stay"), "the focus on Stay"
    page.locator(DIALOG + " .dialog-stay").click()
    assert not dialog_open(page), "Stay closes it"
    page.wait_for_timeout(300)
    assert page.evaluate("state.me && state.me.user && state.me.user.name") == "operator", "still signed in"
    assert not page.evaluate("document.getElementById('login-dialog').open"), "no sign-in asked"
    show_view(page, "tiles")
    form = open_vwt(page, room)
    assert value(page, room, "rf_address") == OTHER_ADDRESS, "the draft kept"

    sign_out(page)
    assert wait_until(page, "!!document.querySelector(%s)?.open" % json.dumps(DIALOG), 5), "the dialog asks again"
    page.locator(DIALOG + " .dialog-go").click()
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "signed out"
    assert not dialog_open(page), "the dialog gone"
    assert not page.evaluate("MassimoParts.leaving.changes()"), "the draft dropped"

    sign_in(page, "operator", "op-pass")
    expert(page)
    open_vwt(page, room)
    loaded(page)
    assert value(page, room, "rf_address") == stored[1], "the stored address, signed in again"
    sign_out(page)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "without changes, Sign out at once"
    assert not dialog_open(page), "no dialog"

    # a save that goes through while the question is open: nothing is lost
    # any more, and the sign-out goes on
    sign_in(page, "operator", "op-pass")
    form, _ = unsaved(page, room)
    held, sent = [], []
    hold(page, room["path"] + "/config", "PUT", held, sent)
    try:
        form.locator("button[type=submit]").click()
        assert wait_held(page, held), "the PUT is sent"
        sign_out(page)
        assert wait_until(page, "!!document.querySelector(%s)?.open" % json.dumps(DIALOG), 5), "the dialog asks while it saves"
        for route in held:
            route.fulfill(status=200, content_type="application/json", body="{}")
        assert wait_until(page, "document.getElementById('login-dialog').open", 5), "saved: the sign-out goes on"
        assert not dialog_open(page), "the dialog gone"
    finally:
        page.unroute(room["path"] + "/config")
    assert seen == [], f"no browser prompt: {seen}"
