# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Pairing and removal: in Settings → Cubes a device is paired, named,
put into a room with the room's settings and removed from the Cube again
in the room's Devices tab; a device paired while the page was not
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

from helpers import named_buttons, scroll_width, wait_until

URL = os.environ.get("UI_TEST_URL", "http://127.0.0.1:8119/").rstrip("/")
TOKEN = os.environ.get("UI_TEST_TOKEN", "")
SERVE_LOG = os.environ.get("UI_TEST_SERVE_LOG", "")
CUBE = "/api/v1/cubes/KEQ0537249"
DEV = "0abcde"
PANEL = ".cube-panel[data-serial='KEQ0537249']"
ENDED = {"en": "Pairing ended; 1 device joined.", "de": "Anlernen beendet; 1 Gerät hat sich gemeldet."}
LEFT = {"en": "s left", "de": "noch"}
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


def test_pair_put_into_a_room_and_remove(page, case, clean):
    room = first_room(page)
    before = serve_log()
    page.locator("#view-settings").click()
    panel = page.locator(PANEL)
    panel.locator("summary").click()
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
    form.locator("input[name=name]").fill("Pair test")
    form.locator("select[name=room]").select_option(str(room["id"]))
    assert form.locator("input[name=copy]").is_checked(), "the copy is on by default"
    form.locator("button[type=submit]").click()
    assert wait_until(page, IN_ROOM % room["id"], 30), "the device is in the room"
    done = "(() => { const r = document.querySelector(\"%s .pair-device[data-rf='%s']\"); return !!r && !r.querySelector('form'); })()" % (PANEL, DEV)
    assert wait_until(page, done, 30), "the row says how it went once the copy is done"
    assert room["name"] in row.locator(".pair-result").inner_text(), "the outcome names the room"
    assert wait_until(page, "(document.querySelector(\"%s .pairing-msg\") || {}).textContent === %s" % (PANEL, json.dumps(ENDED[case.lang])), 15), \
        "the end of pairing said"
    assert pairing.locator(".pairing-running").is_hidden() and pairing.locator(".pair-open").is_visible(), "Pair a device again"
    if SERVE_LOG:
        new = serve_log()[len(before):]
        assert "device paired" in new and "settings copied" in new and "device=0abcde" in new, "the server paired, put and copied"

    page.locator("#view-tiles").click()
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
    form.locator(".pair-copy .switch").click()
    assert not form.locator("input[name=copy]").is_checked(), "the copy switched off"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the form"
    form.locator("button[type=submit]").click()
    assert wait_until(page, IN_ROOM % room["id"], 20), "the device is in the room"
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
    alone; without pair either, Settings → Cubes is gone."""
    page.evaluate("document.body.classList.add('no-cubes')")
    page.locator("#view-settings").click()
    panel = page.locator(PANEL)
    assert panel.is_visible(), "the Cube's panel with pair alone"
    panel.locator("summary").click()
    assert panel.locator(".pair-open").is_visible(), "Pair a device"
    assert panel.locator("form.cube-form").is_hidden(), "not the Cube's settings"
    assert page.locator(".cube-found-list").count() == 0 or page.locator(".cube-found-list").is_hidden(), "not the found Cubes"
    page.evaluate("document.body.classList.add('no-pair')")
    assert page.locator("#settings .cubes-settings").is_hidden(), "no Settings → Cubes without either"
    page.evaluate("document.body.classList.remove('no-cubes', 'no-pair')")
    page.locator("#view-tiles").click()
