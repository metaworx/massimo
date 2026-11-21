# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""An answer that comes after a sign-out changes nothing on the page
(AP UiParts, D197 and D212): the regions moved to parts before Block 10
take the session's number before a request and drop its answer, and
whatever would follow it, where a sign-out came meanwhile, as the later
blocks' do (ui/src/shared/session.ts).

Each test holds a request of one of them on the owner's page (its key),
ends the session one of two ways, a sign-out (Forget key) or an expired
session, signs the operator in, and only then lets the request go with
an answer of its own, so that it comes in the next user's session. It
waits until the page has taken the answer (the response finished, two
frames for what a part draws and sends after it) and asserts that
nothing of it is drawn, said, sent or started: a toast, a line, a next
request, a countdown.

The other rows of the design's table cannot reach the next user and
have no test here: a room's configuration's lines and data (state.ui is
cleared at the sign-out and openConfig starts the configuration anew;
its second writes can, and have theirs: a device's name or room after
its settings, a copy's further rooms), a device's detail (its row's
gone flag drops a late answer), the repairs (their boxes are cleared at
the sign-out), a Cube's devices and a pairing's rooms (data the next
user's own read replaces)."""
import json
from datetime import datetime, timedelta, timezone

import pytest

from conftest import TOKEN, URL
from helpers import choose_cube, frames, hold, owner_page, reads_of, show_view, sign_out, wait_held, wait_until
from test_login import sign_in
from test_unsaved import at, cfg_room, cfg_tab, expire

SERIAL = "KEQ0000100"
WAYS = ("sign-out", "expired")


def end_session(page, way):
    """The owner's session ended one of the two ways, the sign-in asked
    for, and the operator signed in."""
    if way == "sign-out":
        sign_out(page)
        go = page.locator("#leaving dialog[open] .dialog-go")
        if wait_until(page, "!!document.querySelector('#leaving dialog[open]') || document.getElementById('login-dialog').open", 5) \
                and go.count():
            go.click()  # a form still saving: the dialog asks first, and Sign out goes on
    else:
        expire(page)
    sign_in(page, "operator", "op-pass")
    assert page.evaluate("state.me.user.name") == "operator", "the next user is in"


def let_go(page, held, method, end, body):
    """The held request answered with body, waited for until the page has
    taken it: its response finished and two frames drawn."""
    with page.expect_response(lambda r: r.request.method == method and r.url.split("?")[0].endswith(end)) as answered:
        held[0].fulfill(status=200, content_type="application/json", body=json.dumps(body))
    answered.value.finished()
    frames(page)


def requests_of(page, method, end):
    """The requests of a method whose path ends with end, from now on."""
    seen = []
    page.on("request", lambda r: seen.append(r.url) if r.method == method and r.url.split("?")[0].endswith(end) else None)
    return seen


@pytest.mark.parametrize("way", WAYS)
def test_the_search_for_cubes(new_context, case, watch, guard, way):
    """The header's search for Cubes (header/send.ts), answered in the next
    user's session, reads the Cubes no more and shows no toast. Fails on
    the page before: it read the Cubes and said "No new Cube found"."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    held, sent = [], []
    hold(page, "**/api/v1/cubes/discover", "POST", held, sent)
    choose_cube(page, "__discover")
    assert wait_held(page, held), "the search is on its way"
    end_session(page, way)
    cubes = requests_of(page, "GET", "/api/v1/cubes")
    let_go(page, held, "POST", "/cubes/discover", {"found": []})
    assert not cubes, f"the Cubes not read for the search: {cubes}"
    assert not page.evaluate("document.getElementById('toast-body').textContent.includes(t('discover.none'))"), "no toast of the search"


@pytest.mark.parametrize("way", WAYS)
def test_the_house_to_every_room(new_context, case, watch, guard, way):
    """The House panel's Auto to every room (rooms/House.tsx), its first
    room's setpoint answered in the next user's session, writes no further
    room and says nothing. Fails on the page before: it wrote the next
    room under the next user's key and drew the rooms' outcome."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    page.locator("#house-auto").click()
    page.locator("#house-rooms .check-all input").check()
    held, sent = [], []
    hold(page, "**/rooms/*/setpoint", "PUT", held, sent)
    page.locator("#house-panel button[type=submit]").click()
    assert wait_held(page, held), "the first room's setpoint is on its way"
    end_session(page, way)
    line = "document.querySelector('#house-panel .tile-result').textContent"
    said = page.evaluate(line)
    let_go(page, held, "PUT", "/setpoint", {})
    assert len(sent) == 1, f"no further room written: {sent}"
    assert page.evaluate(line) == said, "the House says no room's outcome"


@pytest.mark.parametrize("way", WAYS)
def test_the_house_in_eco(new_context, case, watch, guard, way):
    """The House panel's Eco (rooms/House.tsx) reads a room's thermostat
    for its eco temperature, then sets it: that read answered in the next
    user's session (a read is sent again after any sign-in) sends no
    setpoint. Fails on the page before: the setpoint went out under the
    next user's key, a write that user never made."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    page.locator("#house-eco").click()
    page.locator("#house-rooms .check-all input").check()
    held, sent = [], []
    hold(page, "**/api/v1/cubes/*/devices/*", "GET", held, sent)
    setpoints = requests_of(page, "PUT", "/setpoint")
    page.locator("#house-panel button[type=submit]").click()
    assert wait_held(page, held), "the thermostat's read is on its way"
    end_session(page, way)
    let_go(page, held, "GET", held[0].request.url.split("?")[0].rsplit("/", 1)[1], {"thermostat": {"eco_temp": 17, "comfort_temp": 21}})
    assert not setpoints, f"no setpoint sent in the next user's session: {setpoints}"


@pytest.mark.parametrize("way", WAYS)
def test_the_pairing_started(new_context, case, watch, guard, way):
    """Cube settings' pairing Start (cubes/flow.ts), answered in the next
    user's session as running, starts no countdown: the flow forgotten at
    the sign-out is not made again. Fails on the page before: the Cube's
    pairing showed as running, its countdown ticking, for the next user."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    show_view(page, "cube")
    pairing = page.locator("#cube-page-body .cube-pairing")
    pairing.locator(".pair-open").click()
    pairing.locator("input[name=seconds]").fill("60")
    held, sent = [], []
    hold(page, "**/api/v1/cubes/%s/pairing" % SERIAL, "POST", held, sent)
    pairing.locator(".pairing-form button[type=submit]").click()
    assert wait_held(page, held), "Start is on its way"
    end_session(page, way)
    until = (datetime.now(timezone.utc) + timedelta(minutes=5)).isoformat()
    let_go(page, held, "POST", "/pairing", {"active": True, "until": until, "paired": []})
    running = page.evaluate("[...document.querySelectorAll('.cube-pairing .pairing-running')].some((e) => !e.classList.contains('hidden'))")
    assert not running, "no pairing running for the next user"


@pytest.mark.parametrize("way", WAYS)
def test_the_cubes_save(new_context, case, watch, guard, way):
    """Cube settings' Save (cubes/send.ts, the Cubes' runner), answered in
    the next user's session, reloads nothing and says nothing on the
    plain line under the panel. Fails on the page before: it read the
    Cubes again and said the Cube was saved."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    show_view(page, "cube")
    form = page.locator("#cube-page-body form.cube-form")
    form.wait_for(state="visible")
    form.locator("input[name=name]").fill("Renamed late")
    held, sent = [], []
    hold(page, "**/api/v1/cubes/%s" % SERIAL, "PUT", held, sent)
    form.locator("button[type=submit]").click()
    assert wait_held(page, held), "the save is on its way"
    end_session(page, way)
    cubes = requests_of(page, "GET", "/api/v1/cubes")
    let_go(page, held, "PUT", "/cubes/" + SERIAL, {})
    assert not cubes, f"the Cubes not read for the save: {cubes}"
    for line in ("cube-page-msg", "cubes-msg"):
        assert page.evaluate(f"document.getElementById('{line}').classList.contains('hidden')"), f"#{line} says nothing"


UNLISTED = {"rf_address": "0a0f01", "type": "radiator_thermostat", "name": "", "room_id": 0, "serial": "MEQ0000901",
            "unlisted": True, "reported": False}


def unlisted_row(page):
    """Rooms with a device without a room, GET …/devices answered with the
    server's answer and it at every load."""
    reads_of(page).change("**/api/v1/cubes/*/devices?devices=real", lambda devices: (devices or []) + [UNLISTED])
    page.evaluate("loadRooms()")
    row = page.locator("#unlisted .device[data-rf='0a0f01']")
    row.wait_for(state="visible")
    return row


LINES = "['message', 'unlisted-msg'].map((id) => { const el = document.getElementById(id); return el.classList.contains('hidden') ? '' : el.textContent; })"


@pytest.mark.parametrize("way", WAYS)
def test_the_device_removed(new_context, case, watch, guard, way):
    """A device without a room removed from the Cube (rooms/Unlisted.tsx),
    answered in the next user's session, says nothing on the page's plain
    lines. Fails on the page before: it said the device was removed."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    row = unlisted_row(page)
    held, sent = [], []
    hold(page, "**/api/v1/cubes/*/devices/0a0f01", "DELETE", held, sent)
    hold(page, "**/api/v1/cubes/*/devices/MEQ0000901", "DELETE", held, sent)
    row.locator(".remove-device").click()
    row.locator(".confirm-yes").click()
    assert wait_held(page, held), "the removal is on its way"
    end_session(page, way)
    lines = page.evaluate(LINES)
    # the plain removeFromCube reads the rooms after the answer, and the line was said after that read
    with page.expect_response(lambda r: r.request.method == "GET" and r.url.split("?")[0].endswith("/rooms")) as read:
        let_go(page, held, "DELETE", held[0].request.url.rsplit("/", 1)[1], {})
    read.value.finished()
    frames(page)
    assert page.evaluate(LINES) == lines, "nothing said of the removal"


@pytest.mark.parametrize("way", WAYS)
def test_the_device_put_into_a_room(new_context, case, watch, guard, way):
    """A device without a room put into a room (shared/put.tsx, through the
    devices without a room), answered in the next user's session, reloads
    nothing and says nothing on the page's plain lines. Fails on the page
    before: it read the rooms again and said the device was put."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    row = unlisted_row(page)
    row.locator(".put-open").click()
    form = row.locator("form.pair-device-form")
    form.wait_for(state="visible")
    form.locator("select").first.select_option(index=1)
    held, sent = [], []
    hold(page, "**/api/v1/cubes/*/devices/*", "PUT", held, sent)
    form.locator("button[type=submit]").click()
    assert wait_held(page, held), "the put is on its way"
    end_session(page, way)
    rooms = requests_of(page, "GET", "/rooms")
    lines = page.evaluate(LINES)
    let_go(page, held, "PUT", held[0].request.url.rsplit("/", 1)[1], {})
    assert not rooms, f"the rooms not read for the put: {rooms}"
    assert page.evaluate(LINES) == lines, "nothing said of the put"


@pytest.mark.parametrize("way", WAYS)
def test_the_copy_to_rooms(new_context, case, watch, guard, way):
    """A room's settings copied to two rooms (rooms/configuration.ts
    sendCopy), the first room's write answered in the next user's session,
    writes the second room no more. Fails on the page before: it went on
    to write the second room under the next user's key."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    room = cfg_room(page)
    cfg_tab(page, room, "copy")
    choices = page.locator(at(room, ".config-body .choices"))
    switches = choices.first.locator(".switch")
    switches.nth(0).click()
    switches.nth(1).click()
    held, sent = [], []
    hold(page, "**/api/v1/cubes/*/rooms/*/config", "PUT", held, sent)
    page.locator(at(room, ".config-body form button[type=submit]")).click()
    assert wait_held(page, held), "the first room's write is on its way"
    end_session(page, way)
    let_go(page, held, "PUT", "/config", {"ok": True, "devices": []})
    assert len(sent) == 1, f"no further room written: {sent}"


@pytest.mark.parametrize("way", WAYS)
def test_the_device_put_during_its_reload(new_context, case, watch, guard, way):
    """A device without a room put into a room (shared/put.tsx), its write
    answered at once and the rooms' read that follows it answered in the
    next user's session, says nothing on the page's plain lines. Fails on
    the page before: it said the device was put once the read had come."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    row = unlisted_row(page)
    row.locator(".put-open").click()
    form = row.locator("form.pair-device-form")
    form.wait_for(state="visible")
    form.locator("select").first.select_option(index=1)
    page.route("**/api/v1/cubes/*/devices/*", lambda route: route.fulfill(status=200, content_type="application/json", body="{}")
               if route.request.method == "PUT" else route.continue_())
    held = []

    def first_read(route):
        if not held and route.request.method == "GET":
            held.append(route)  # the put's reload; every read after it goes
        else:
            route.continue_()

    page.route("**/api/v1/cubes/*/rooms?devices=all", first_read)
    form.locator("button[type=submit]").click()
    assert wait_held(page, held), "the put's reload is on its way"
    end_session(page, way)
    lines = page.evaluate(LINES)
    with page.expect_response(lambda r: r.request is held[0].request) as read:
        held[0].continue_()
    read.value.finished()
    frames(page)
    assert page.evaluate(LINES) == lines, "nothing said of the put"


@pytest.mark.parametrize("way", WAYS)
def test_the_device_settings_then_its_name(new_context, case, watch, guard, way):
    """A device's settings and its name saved together in its room's
    configuration (rooms/configuration.ts sendDevice), the settings' write
    answered in the next user's session, sends the name's write no more.
    Fails on the page before: the name went out after it under the next
    user's key."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    room = cfg_room(page)
    cfg_tab(page, room, "devices")
    device = page.locator(at(room, ".config-body .cfg-device")).first
    offset = device.locator("input[name=temp_offset]")
    offset.fill("1" if offset.input_value() not in ("1", "1.0") else "1.5")
    device.locator("input[name=name]").fill("Renamed late")
    held, sent = [], []
    hold(page, "**/api/v1/cubes/*/devices/*/config", "PUT", held, sent)
    names = []
    page.on("request", lambda r: names.append(r.url) if r.method == "PUT" and "/devices/" in r.url and not r.url.endswith("/config") else None)
    device.locator("button[type=submit]").click()
    assert wait_held(page, held), "the settings' write is on its way"
    end_session(page, way)
    let_go(page, held, "PUT", "/config", {"ok": True, "devices": []})
    assert not names, f"no name's write after the settings' answer: {names}"
