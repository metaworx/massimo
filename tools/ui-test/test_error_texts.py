# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A refusal of the server in the page's language (AP ErrorCodes,
Block 1): a room's setpoint refused with the server's own answer, the
owner's case of room_thermostat_unplaced that the Go test
TestRoomThermostatRefusals checks as it is sent
(internal/server/testdata/room_thermostat_unplaced.json), is said on the
room's tile in German at de and in English at en, the room and the
thermostat by their names; a code the page has no text for shows the
server's English error; and neither is put in as markup. The setpoint's
PUT is answered at its route, so that nothing reaches the fake Cube.

An error kept with a result (Block 5), the reboot's blocked and its last
failure and a log line's error, is said by its code alike; one kept
before the codes, by its English text."""
import json
from pathlib import Path

from helpers import isolate_log, show_view, wait_log_live, wait_until

# Room 2 of greeting.txt, "Büro", whose only thermostat is 0a0109,
# "Radiator Büro": the room and the thermostat of the Go test's answer.
ROOM = 2
TILE = '.room[data-id="%d"]' % ROOM
LINE = TILE + " .tile-result"
SETPOINT = "**/api/v1/cubes/*/rooms/%d/setpoint" % ROOM

UNPLACED = (Path(__file__).resolve().parents[2] / "internal/server/testdata/room_thermostat_unplaced.json").read_text(encoding="utf-8")

SAID = {
    "en": "auto: the Cube has no thermostat in the room Büro: its list has Radiator Büro there, "
          "but the Cube has it in another room or in none; the repair “Place in its listed room” puts it there",
    "de": "auto: der Cube hat kein Thermostat im Raum Büro: seine Liste führt Radiator Büro dort, "
          "der Cube aber in einem anderen Raum oder in keinem; die Reparatur „In den gelisteten Raum setzen“ setzt es dorthin",
}


def refuse_with(page, watch, body):
    """The room's setpoint answered with 409 and body; any other method
    of the route goes on to the server."""
    watch.expected.append(("/rooms/%d/setpoint" % ROOM, 409))
    page.unroute(SETPOINT)
    page.route(SETPOINT, lambda route: route.fulfill(status=409, content_type="application/json", body=body)
               if route.request.method == "PUT" else route.continue_())


def refused_line(page):
    """AUTO pressed on the room's tile; the tile's line once it says the
    failure, a text it did not say before, as text, and how many
    elements it holds."""
    before = page.evaluate("(s) => document.querySelector(s)?.textContent || ''", LINE)
    page.locator(TILE + " .mode-auto").click()
    assert wait_until(page, "(() => { const el = document.querySelector(%s); return !!el && el.classList.contains('bad') && el.textContent !== %s; })()"
                      % (json.dumps(LINE), json.dumps(before)), 5), "the tile says it failed"
    return page.evaluate("(s) => { const el = document.querySelector(s); return [el.textContent, el.children.length]; }", LINE)


def test_refusal_in_the_page_language(page, case, watch):
    """The server's answer of the owner's case, room_thermostat_unplaced,
    is said in the page's language: the room and its thermostat by their
    names, the repair by its label there; as text."""
    try:
        refuse_with(page, watch, UNPLACED)
        text, elements = refused_line(page)
        assert text == SAID[case.lang], f"the tile's line: {text!r}"
        assert elements == 0, "the line is text"
        assert json.loads(UNPLACED)["error"] not in text, "not the server's English"
    finally:
        page.unroute(SETPOINT)


def test_unknown_code_says_the_english_error(page, case, watch):
    """A code the page has no text for shows the server's English error,
    in either language; markup in the error, or in a parameter of a code
    it knows, is shown as the characters it is."""
    try:
        refuse_with(page, watch, json.dumps({"error": "the <b>server</b> refused it", "code": "not_a_code_yet", "params": {"room": ROOM}}))
        text, elements = refused_line(page)
        assert text == "auto: the <b>server</b> refused it", f"the tile's line: {text!r}"
        assert elements == 0, "the error is text"

        refuse_with(page, watch, json.dumps({"error": "room 77 has no thermostat", "code": "room_no_thermostat", "params": {"room": "<i>77</i>"}}))
        text, elements = refused_line(page)
        want = {"en": "auto: the room <i>77</i> has no thermostat", "de": "auto: der Raum <i>77</i> hat kein Thermostat"}[case.lang]
        assert text == want, f"the tile's line: {text!r}"
        assert elements == 0, "the parameter is text"
    finally:
        page.unroute(SETPOINT)


REBOOT = "**/api/v1/reboot"
MISSING = "no reboot helper installed: sudo massimo install -reboot-helper"
KEPT = {
    "en": ("No reboot from here: no reboot helper is installed: on the box, run sudo massimo install -reboot-helper. "
           "The last reboot failed: the reboot failed: exit status 1"),
    "de": ("Von hier aus ist kein Neustart des Computers möglich: kein Neustart-Helfer installiert: auf der Box sudo massimo install "
           "-reboot-helper ausführen. Der letzte Neustart ist fehlgeschlagen: der Neustart ist fehlgeschlagen: exit status 1"),
}
KEPT_BEFORE = {
    "en": "No reboot from here: " + MISSING + ". The last reboot failed: exit status 1",
    "de": "Von hier aus ist kein Neustart des Computers möglich: " + MISSING + ". Der letzte Neustart ist fehlgeschlagen: exit status 1",
}


def reboot_says(page, answer):
    """Maintenance's reboot text once GET /api/v1/reboot answers answer."""
    page.unroute(REBOOT)
    page.route(REBOOT, lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(answer))
               if route.request.method == "GET" else route.abort())
    show_view(page, "tiles")
    show_view(page, "maint")
    assert wait_until(page, "(document.getElementById('reboot-text')?.textContent || '').includes('exit status 1')", 5), "the reboot's text"
    return page.locator("#reboot-text").inner_text()


def test_kept_errors_in_the_page_language(page, case):
    """GET /api/v1/reboot's blocked and its last failure are said by
    their codes, blocked_code and code, in the page's language; kept
    without codes, by their English text."""
    try:
        text = reboot_says(page, {"available": False, "by": "systemd", "blocked": MISSING, "blocked_code": "reboot_no_helper",
                                  "error": "exit status 1", "code": "reboot_failed", "params": {"reason": "exit status 1"}})
        assert text == KEPT[case.lang], f"the reboot's text: {text!r}"
        text = reboot_says(page, {"available": False, "by": "systemd", "blocked": MISSING, "error": "exit status 1"})
        assert text == KEPT_BEFORE[case.lang], f"the reboot's text without codes: {text!r}"
    finally:
        page.unroute(REBOOT)
        show_view(page, "tiles")


LOGGED = {
    "en": "the Cube is not connected yet",
    "de": "der Cube ist noch nicht verbunden",
}


def test_log_line_error_in_the_page_language(page, case, reads):
    """A log line's error is said by its code in the page's language, as
    text; a line kept before the codes says its English error."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    page.evaluate("""() => {
      const now = () => new Date().toISOString();
      const send = (kind, ev) => state.events.dispatchEvent(new MessageEvent(kind, { data: JSON.stringify(ev) }));
      send("backup", { kind: "backup", source: "system", level: "warn", time: now(), summary: { reason: "auto", retry_in: "30m0s" },
        error: "backup: no greeting from the <b>cube</b> yet", code: "cube_not_connected_yet" });
      send("restore_done", { kind: "restore_done", source: "system", level: "warn", time: now(),
        error: "step 1 (names): engine: command <i>discarded</i> by the cube", restore: { id: "r1", steps: 1, ok: false, state: "failed" } });
    }""")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-backup').length > 0 && "
                            "document.querySelectorAll('#log-lines .log-line.kind-restore_done').length > 0", 5), "the two lines"
    backup = page.locator("#log-lines .log-line.kind-backup").first.locator(".log-text")
    done = page.locator("#log-lines .log-line.kind-restore_done").first.locator(".log-text")
    text = backup.inner_text()
    assert text.endswith(LOGGED[case.lang]), f"the backup's line: {text!r}"
    assert "greeting" not in text, "not the server's English"
    text = done.inner_text()
    assert text.endswith("step 1 (names): engine: command <i>discarded</i> by the cube"), f"the line kept before the codes: {text!r}"
    assert backup.evaluate("(el) => el.querySelectorAll('b, i').length") == 0 and done.evaluate("(el) => el.querySelectorAll('b, i').length") == 0, "text"
    show_view(page, "tiles")
