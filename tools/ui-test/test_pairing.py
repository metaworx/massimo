# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Pairing and removal: in System settings → Cubes a device is paired, named
(the form offers "<room> <type> <n>" for the room chosen, D200-A, one
too long for the Cube marked and not sent until shortened, D201),
put into a room with the room's state and settings, which the form's
outcome names, and removed from the Cube again in the room's Devices tab; a device paired while the page was not
looking is put into a room from Devices without a room, and another
removed from there. The fake Cube's device joins two seconds after n:
(run.sh, -pair) and again after it was removed, so every case pairs it;
each test stops pairing and removes the device again at its end, as the
four cases share the server. The fake Cube's lines are the Go tests'
(internal/server); here the server's log says what was sent."""
import json
import os
import ssl
import time
import urllib.error
import urllib.request

import pytest

from helpers import named_buttons, scroll_width, show_view, wait_until

URL = os.environ.get("UI_TEST_URL", "http://127.0.0.1:8119/").rstrip("/")
TOKEN = os.environ.get("UI_TEST_TOKEN", "")
SERVE_LOG = os.environ.get("UI_TEST_SERVE_LOG", "")
CUBE = "/api/v1/cubes/KEQ0000100"
DEV = "0abcde"
PANEL = ".cube-panel[data-serial='KEQ0000100']"
ENDED = {"en": "Pairing ended; 1 device joined.", "de": "Anlernen beendet; 1 Gerät hat sich gemeldet."}
LEFT = {"en": "s left", "de": "noch"}
RANGE = {"en": "only 1 to 255", "de": "nur 1 bis 255"}
# The name offered for the fake's radiator thermostat (D200-A): the
# room's name, the type in the page's language, the lowest free n. The
# German "Wohnzimmer Heizkörperthermostat 1" is 34 bytes, past the 32 the
# Cube keeps: it is offered as it is, marked, and not sent until it is
# shortened (D201), to SHORT; the English is 32.
OFFERED = {"en": {1: "Wohnzimmer radiator thermostat 1", 3: "Küche radiator thermostat 1"},
           "de": {1: "Wohnzimmer Heizkörperthermostat 1", 3: "Küche Heizkörperthermostat 1"}}
TOO_LONG = {"en": False, "de": True}
LONG = {"en": "longer than 32 bytes", "de": "länger als 32 Bytes"}
SHORT = "Wohnzimmer Heizkörper 1"
IN_ROOM = "state.rooms.some((r) => r.id === %d && r.devices.some((d) => d.rf_address === '" + DEV + "'))"
GONE = "!state.rooms.some((r) => r.devices.some((d) => d.rf_address === '" + DEV + "')) && !state.unlisted.some((d) => d.rf_address === '" + DEV + "')"


def api(method, path, body=None):
    """The API with the suite's token; the status and the answer."""
    tls = ssl.create_default_context()
    tls.check_hostname, tls.verify_mode = False, ssl.CERT_NONE
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(URL + path, data=data, method=method,
                                 headers={"Authorization": "Bearer " + TOKEN, "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=60, context=tls) as resp:
            return resp.status, json.loads(resp.read() or b"null")
    except urllib.error.HTTPError as e:
        return e.code, None


def settle():
    """No pairing running and the fake's device not on the server."""
    api("DELETE", CUBE + "/pairing")
    if api("GET", CUBE + "/devices/" + DEV)[0] == 200:
        assert api("DELETE", CUBE + "/devices/" + DEV)[0] == 200, "the paired device removed"


@pytest.fixture
def clean():
    settle()
    yield
    settle()


def serve_log():
    """What the server logged so far, empty without UI_TEST_SERVE_LOG."""
    if not SERVE_LOG:
        return ""
    with open(SERVE_LOG, encoding="utf-8", errors="replace") as f:
        return f.read()


def pair_by_api():
    """Pairing through the API until the fake's device is on the server,
    then stopped: a device that joined while the page was not looking."""
    assert api("POST", CUBE + "/pairing", {"seconds": 30})[0] == 200, "pairing started"
    end = time.time() + 10
    while time.time() < end and api("GET", CUBE + "/devices/" + DEV)[0] != 200:
        time.sleep(0.2)
    assert api("DELETE", CUBE + "/pairing")[0] == 200, "pairing stopped"
    assert api("GET", CUBE + "/devices/" + DEV)[0] == 200, "the device joined"


def first_room(page):
    return page.evaluate("(() => { const r = state.rooms[0]; return { id: r.id, name: r.name, n: r.devices.length }; })()")


def name_in(page, room):
    """The name the device has in a room of the page."""
    return page.evaluate("((state.rooms.find((r) => r.id === %d) || {devices: []}).devices.find((d) => d.rf_address === '%s') || {}).name"
                         % (room, DEV))


def check_name_mark(page, case, form, long, submit=False):
    """The form's name field marked as too long for the Cube (D201), as
    the pairing time's field is out of range: the box red, the limit in
    amber before it, above the box on its own line, the field's
    description; or neither. With submit, a too long name puts nothing
    into a room: the submit goes back to the field."""
    name, hint = form.locator("input[name=name]"), form.locator(".pair-name-long")
    if not long:
        assert hint.is_hidden() and name.get_attribute("aria-invalid") is None, "a name that fits is not marked"
        return
    assert hint.is_visible() and hint.inner_text() == LONG[case.lang], "the limit, before the box"
    assert name.get_attribute("aria-invalid") == "true", "the field marked"
    assert name.get_attribute("aria-describedby") == hint.get_attribute("id"), "the limit is the field's description"
    assert hint.evaluate("(el) => getComputedStyle(el).color") == colour_of(page, "--warn"), "the limit in amber"
    assert name.evaluate("(el) => getComputedStyle(el).borderTopColor") == colour_of(page, "--bad"), "the box in red"
    box, before = name.bounding_box(), hint.bounding_box()
    assert before["y"] + before["height"] <= box["y"] + 1, "the limit above the box"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the limit"
    if not submit:
        return
    form.locator("button[type=submit]").click()
    page.wait_for_timeout(500)
    assert name.evaluate("(el) => el === document.activeElement"), "the submit goes back to the field"
    assert (api("GET", CUBE + "/devices/" + DEV)[1] or {}).get("room_id") == 0, "the device put into no room"


def test_pair_put_into_a_room_and_remove(page, case, clean):
    room = first_room(page)
    before = serve_log()
    show_view(page, "system")
    panel = page.locator(PANEL)
    panel.locator(":scope > summary").click()
    pairing = panel.locator(".cube-pairing")
    pairing.locator(".pair-open").click()
    pairing.locator("input[name=seconds]").fill("5")
    pairing.locator(".pairing-form button[type=submit]").click()
    countdown = "(document.querySelector(\"%s .pairing-countdown\") || {}).textContent || ''" % PANEL
    assert wait_until(page, "(%s).includes(%s)" % (countdown, json.dumps(LEFT[case.lang])), 5), "the countdown"
    row = pairing.locator(".pair-device[data-rf='%s']" % DEV)
    row.wait_for(timeout=10000)
    assert "TEQ0000001" in row.inner_text(), "the device's serial"
    named_buttons(page, "the pairing")
    assert scroll_width(page) <= case.width, "no horizontal scroll with the joined device"
    form = row.locator("form")
    name = form.locator("input[name=name]")
    assert room["id"] == 1 and form.locator("select[name=room]").input_value() == "1", "the first room chosen, Wohnzimmer"
    assert name.input_value() == OFFERED[case.lang][1], "the name offered for the room, as it is"
    check_name_mark(page, case, form, TOO_LONG[case.lang])
    form.locator("select[name=room]").select_option("3")
    assert name.input_value() == OFFERED[case.lang][3], "the name offered follows the room"
    check_name_mark(page, case, form, False)
    name.fill("Pair test")
    form.locator("select[name=room]").select_option(str(room["id"]))
    assert name.input_value() == "Pair test", "a name typed stays when the room changes"
    assert form.locator("input[name=copy]").is_checked(), "the copy is on by default"
    with page.expect_response(lambda r: r.request.method == "PUT" and CUBE + "/devices/" in r.url, timeout=60000) as put:
        form.locator("button[type=submit]").click()
    copied = put.value.json()["copied"]
    assert wait_until(page, IN_ROOM % room["id"], 30), "the device is in the room"
    done = "(() => { const r = document.querySelector(\"%s .pair-device[data-rf='%s']\"); return !!r && !r.querySelector('form'); })()" % (PANEL, DEV)
    assert wait_until(page, done, 30), "the row says how it went once the copy is done"
    outcome = row.locator(".pair-result").inner_text()
    assert room["name"] in outcome, "the outcome names the room"
    # The state the copy's 0x40 carried, the room's from before (AP
    # RoomMajority, Block 2), worded as the form words it, and the days.
    said = page.evaluate("""(c) => {
      const s = c.state, temp = s.setpoint === undefined ? "" : fmtNum(s.setpoint, 1) + " °C";
      const state = s.mode === "manual" ? t("put.stateManual", { temp }) : s.mode === "vacation" ? t("put.stateVacation", { temp, until: fmtUntil(s.until) })
        : s.setpoint === undefined ? t("put.stateAuto") : t("put.stateAutoTemp", { temp });
      const head = c.state_from === "default" ? t(c.mode_not_set ? "put.modeNotSetDefault" : "put.stateDefault")
        : t(c.mode_not_set ? (c.room_mode ? "put.roomMode" : "put.modeNotSet") : "put.state", { state });
      return [head, t("put.copied", { days: c.days })];
    }""", copied)
    assert "state" in copied and copied.get("state_from") == "room" and not copied.get("config_skipped"), f"the copy's answer: {copied}"
    assert said[0] in outcome and said[1] in outcome, f"the outcome names the state and the days: {outcome!r}, want {said}"
    assert wait_until(page, "(document.querySelector(\"%s .pairing-msg\") || {}).textContent === %s" % (PANEL, json.dumps(ENDED[case.lang])), 15), \
        "the end of pairing said"
    assert pairing.locator(".pairing-running").is_hidden() and pairing.locator(".pair-open").is_visible(), "Pair a device again"
    if SERVE_LOG:
        new = serve_log()[len(before):]
        assert "device paired" in new and "settings copied" in new and "device=0abcde" in new, "the server paired, put and copied"

    show_view(page, "tiles")
    tile = page.locator(".room[data-id='%d']" % room["id"])
    tile.locator(".tile-cog").click()
    tile.locator(".config-tabs [data-tab=devices]").click()
    dev = tile.locator(".cfg-device", has_text="Pair test")
    dev.locator(".remove-device").click()
    dev.locator(".confirm-box .confirm-yes").click()
    assert wait_until(page, GONE, 15), "the device is gone"
    assert page.evaluate("state.rooms.find((r) => r.id === %d).devices.length" % room["id"]) == room["n"], "the room as before"
    assert tile.locator(".config-body").is_visible(), "the configuration stays open"
    if SERVE_LOG:
        assert "device removed" in serve_log()[len(before):], "the server removed it"
    assert scroll_width(page) <= case.width, "no horizontal scroll in the Devices tab"
    tile.locator(".config-tabs .close").click()


def test_put_into_a_room_from_devices_without_a_room(page, case, clean):
    pair_by_api()
    page.reload()
    page.wait_for_selector(".room")
    room = first_room(page)
    before = serve_log()
    row = page.locator("#unlisted .device[data-rf='%s']" % DEV)
    assert row.is_visible(), "the device among the devices without a room"
    named_buttons(page, "the devices without a room")
    row.locator(".remove-device").click()
    assert row.locator(".confirm-box").is_visible(), "Remove asks in the row"
    row.locator(".confirm-box .confirm-no").click()
    assert row.locator(".confirm-box").count() == 0 and row.locator(".put-open").is_visible(), "Keep takes the question back"
    row.locator(".put-open").click()
    form = row.locator("form.pair-device-form")
    form.locator("select[name=room]").select_option(str(room["id"]))
    name = form.locator("input[name=name]")
    assert name.input_value() == OFFERED[case.lang][room["id"]], "the name offered for the room, as it is"
    form.locator(".pair-copy .switch").click()
    assert not form.locator("input[name=copy]").is_checked(), "the copy switched off"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the form"
    check_name_mark(page, case, form, TOO_LONG[case.lang], submit=True)
    want = OFFERED[case.lang][room["id"]]
    if TOO_LONG[case.lang]:
        name.fill(SHORT)
        check_name_mark(page, case, form, False)
        want = SHORT
    form.locator("button[type=submit]").click()
    assert wait_until(page, IN_ROOM % room["id"], 20), "the device is in the room"
    assert name_in(page, room["id"]) == want, "under the name offered, or the one shortened"
    assert page.locator("#unlisted .device[data-rf='%s']" % DEV).count() == 0, "no longer without a room"
    if SERVE_LOG:
        new = serve_log()[len(before):]
        assert "settings copied" not in new, "nothing copied with the switch off"


def test_remove_from_devices_without_a_room(page, case, clean):
    pair_by_api()
    page.reload()
    page.wait_for_selector(".room")
    row = page.locator("#unlisted .device[data-rf='%s']" % DEV)
    row.locator(".remove-device").click()
    row.locator(".confirm-box .confirm-yes").click()
    assert wait_until(page, GONE, 15), "the device is gone"
    assert api("GET", CUBE + "/devices/" + DEV)[0] == 404, "the server forgot it"
    assert page.locator("#unlisted").is_hidden(), "no devices without a room left"


def test_pairing_without_the_capability_cubes(page, case):
    """A role with pair but without cubes, the default manager's, as
    login.js marks it on the body: the Cube's panel holds the pairing
    alone; without pair either, System settings → Cubes is gone."""
    page.evaluate("document.body.classList.add('no-cubes')")
    show_view(page, "system")
    panel = page.locator(PANEL)
    assert panel.is_visible(), "the Cube's panel with pair alone"
    panel.locator(":scope > summary").click()
    assert panel.locator(".pair-open").is_visible(), "Pair a device"
    assert panel.locator("form.cube-form").is_hidden(), "not the Cube's settings"
    assert page.locator(".cube-found-list").count() == 0 or page.locator(".cube-found-list").is_hidden(), "not the found Cubes"
    page.evaluate("document.body.classList.add('no-pair')")
    assert page.locator("#system .cubes-settings").is_hidden(), "no System settings → Cubes without either"
    page.evaluate("document.body.classList.remove('no-cubes', 'no-pair')")
    show_view(page, "tiles")


def colour_of(page, var):
    """The colour a CSS variable holds, as getComputedStyle gives it."""
    return page.evaluate("""(v) => {
        const probe = document.createElement("span");
        probe.style.color = "var(" + v + ")";
        document.body.append(probe);
        const c = getComputedStyle(probe).color;
        probe.remove();
        return c;
    }""", var)


def test_pairing_time_at_most_255(page, case, clean):
    """The pairing time is 1 to 255 seconds, 255 the longest a Cube keeps
    the mode: a time out of range outlines the field in red and shows the
    range in amber before it as it is typed, without the browser's bubble;
    Start then starts nothing and goes back to the field; a time in range
    clears both."""
    show_view(page, "system")
    panel = page.locator(PANEL)
    panel.locator(":scope > summary").click()
    pairing = panel.locator(".cube-pairing")
    pairing.locator(".pair-open").click()
    secs = pairing.locator("input[name=seconds]")
    hint = pairing.locator(".pairing-range")
    assert secs.get_attribute("max") == "255", "the field's limit"
    assert pairing.locator(".pairing-form").evaluate("(f) => f.noValidate"), "not the browser's bubble"
    assert hint.is_hidden() and secs.get_attribute("aria-invalid") is None, "60 is in range"
    secs.fill("660")
    assert hint.is_visible() and hint.inner_text() == RANGE[case.lang], "the range, as it is typed"
    assert secs.get_attribute("aria-invalid") == "true", "the field marked"
    assert secs.get_attribute("aria-describedby") == hint.get_attribute("id"), "the range is the field's description"
    assert hint.evaluate("(el) => getComputedStyle(el).color") == colour_of(page, "--warn"), "the range in amber"
    assert secs.evaluate("(el) => getComputedStyle(el).borderTopColor") == colour_of(page, "--bad"), "the box in red"
    box, before = secs.bounding_box(), hint.bounding_box()
    assert before["x"] + before["width"] <= box["x"] + 1, "the range before the box"
    assert before["y"] < box["y"] + box["height"] and box["y"] < before["y"] + before["height"], "on the box's line"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the range"
    pairing.locator(".pairing-form button[type=submit]").click()
    page.wait_for_timeout(500)
    assert secs.evaluate("(el) => el === document.activeElement"), "Start goes back to the field"
    assert pairing.locator(".pairing-running").is_hidden(), "no countdown"
    assert api("GET", CUBE + "/pairing")[1]["active"] is False, "no pairing started"
    secs.fill("")
    assert hint.is_visible() and secs.get_attribute("aria-invalid") == "true", "an empty field marked"
    secs.fill("255")
    assert hint.is_hidden(), "255 hides the range"
    assert secs.get_attribute("aria-invalid") is None and secs.get_attribute("aria-describedby") is None, "255 clears the mark"
    show_view(page, "tiles")
