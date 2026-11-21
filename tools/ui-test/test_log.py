# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The log: it opens, newest first, a state event with changes shows what
changed, the kind and the device chosen narrow it, Clear sets them back,
the range limits it by count or by time, the address holds the range,
the kind and the device, Older pages back, under All Cubes a poll reads
on after the newest line's id, a log the server no longer has loads
again, and an answer that comes late or fails leaves the log as it
should be."""
import json
import re
import time
from datetime import datetime, timedelta, timezone
from urllib.parse import parse_qs, urlparse

from conftest import api_call
from helpers import CHANGES, isolate_log, named_buttons, scroll_width, show_view, wait_held, wait_log_live, wait_until

OLDER = {"en": "Older events", "de": "Ältere Ereignisse"}
NO_OLDER = {"en": "No older events", "de": "Keine älteren Ereignisse"}


def test_log_opens(page):
    show_view(page, "log")
    assert page.locator("#logview").is_visible(), "log opens"
    named_buttons(page, "the log")


def test_log_changes(page, case, reads):
    """A state event with changes (as the server sends after the first
    one of a device) shows what changed, not the whole state. The log is
    opened first and holds the test's own lines alone (isolate_log); the
    page leaves the stream's first 1.5 s out of the log (the burst of
    state events a connection starts with), so the event comes once the
    log takes them (wait_log_live). The log tests that send events do
    the same."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    page.evaluate("""() => {
      const d = JSON.parse(JSON.stringify(state.rooms[0].devices[0]));
      state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({
        kind: "state", device: d.rf_address, state: d, time: new Date().toISOString(),
        changes: { window_open: { from: false, to: true }, setpoint: { from: 18, to: 5 } } }) }));
    }""")
    line = page.locator(".log-line.kind-state").first
    assert CHANGES[case.lang] in line.inner_text(), "the log says what changed"
    show_view(page, "tiles")


def test_log_radio_faults(page, case, reads):
    """The log names a device's flags as its badges do: a communication
    error that came with the radiogram that failed, by its technical
    name and code; another radiogram failing while the error stays; the
    error cleared; a link error."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    page.evaluate("""() => {
      const d = JSON.parse(JSON.stringify(state.rooms[0].devices[0]));
      const send = (changes) => state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({
        kind: "state", device: d.rf_address, state: d, time: new Date().toISOString(), changes }) }));
      send({ error: { from: false, to: true }, failed_radiogram: { from: null, to: { code: 3, name: "TimeInformation" } } });
      send({ failed_radiogram: { from: { code: 3, name: "TimeInformation" }, to: { code: 246 } } });
      send({ error: { from: true, to: false }, failed_radiogram: { from: { code: 246 }, to: null } });
      send({ link_error: { from: false, to: true } });
    }""")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-state').length >= 4", 5), "the four lines"
    texts = [page.locator(".log-line.kind-state").nth(i).locator(".log-text").inner_text() for i in range(4)]
    want = page.evaluate("""() => [t("log.linkError"), t("log.ch.errorCleared"),
      t("log.ch.failedRadiogram", { radiogram: "0xf6" }), t("log.errorRadiogram", { radiogram: "TimeInformation (0x03)" })]""")
    assert texts == want, f"newest first: {texts}, want {want}"
    if case.lang == "en":
        assert want[3] == "communication error: TimeInformation (0x03)", want[3]
    else:
        assert want[3] == "Kommunikationsstörung: TimeInformation (0x03)", want[3]
    show_view(page, "tiles")


ROOM_CHECK_SAYS = {
    "en": {
        "kinds": ("room disagrees", "programme drift"),
        "differ": "{room}: its thermostats differ in setpoint for 11 min: {a}: manual set 21.0, {b}: manual set 18.0",
        "resent": "{room}: manual set 21.0 sent again to its thermostats, as Massimo last set it",
        "majority": "{room}: auto on the week programme sent again to its thermostats, as most of its thermostats have it",
        "drift": "does not follow its week programme: 12.0 instead of 21.0 since the switch point at 18:20, while the room's other"
        " thermostats follow theirs; its clock or its programme is suspect; it shows the programme an hour off;"
        " communication error: TimeInformation (0x03)",
    },
    "de": {
        "kinds": ("Raum uneinig", "Programmabweichung"),
        "differ": "{room}: die Thermostate unterscheiden sich seit 11 Min. in Solltemperatur: {a}: Manuell Soll 21,0, {b}: Manuell Soll 18,0",
        "resent": "{room}: Manuell Soll 21,0 nochmals an die Thermostate gesendet, wie Massimo es zuletzt einstellte",
        "majority": "{room}: Auto nach Wochenprogramm nochmals an die Thermostate gesendet, wie die meisten seiner Thermostate es haben",
        "drift": "folgt seinem Wochenprogramm nicht: 12,0 statt 21,0 seit dem Schaltpunkt um 18:20, während die anderen Thermostate"
        " des Raums ihrem folgen; seine Uhr oder sein Programm ist verdächtig; es zeigt das Programm um eine Stunde verschoben;"
        " Kommunikationsstörung: TimeInformation (0x03)",
    },
}


def test_log_room_check_lines(page, case, reads):
    """The room checks' lines (AP RadioHealth v1.2, Block 9), sent on the
    stream: a room whose thermostats disagree names the room and each
    thermostat with its setting, the state sent again says from where,
    Massimo's setpoint or the majority's (D188); a
    thermostat off its week programme says what it shows and what the
    programme says, that it shows it an hour off and the radiogram that
    failed."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    names = page.evaluate("""() => {
      const r = state.rooms.find((x) => x.devices.filter((d) => !d.virtual).length >= 2);
      const ds = r.devices.filter((d) => !d.virtual);
      const now = () => new Date().toISOString();
      const send = (kind, ev) => state.events.dispatchEvent(new MessageEvent(kind, { data: JSON.stringify(Object.assign({ kind, time: now() }, ev)) }));
      send("disagree", { source: "cube", level: "warn", summary: { room: r.id, outcome: "differ", minutes: 11, differs: ["setpoint"],
        members: [{ rf: ds[0].rf_address, mode: "manual", setpoint: 21 }, { rf: ds[1].rf_address, mode: "manual", setpoint: 18 }] } });
      send("disagree", { source: "cube", level: "info", summary: { room: r.id, outcome: "resent", from: "massimo", mode: "manual", setpoint: 21 } });
      send("disagree", { source: "cube", level: "info", summary: { room: r.id, outcome: "resent", from: "majority", mode: "auto" } });
      send("drift", { source: "device", level: "warn", device: ds[1].rf_address, serial: ds[1].serial, summary: { room: r.id,
        outcome: "not_following", setpoint: 12, expected: 21, switch_at: "18:20", hour_off: true, auto_dst: true,
        failed_radiogram: { code: 3, name: "TimeInformation" } } });
      return { room: r.name, a: deviceName(ds[0]), b: deviceName(ds[1]) };
    }""")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-drift').length > 0", 5), "the drift line"
    says = ROOM_CHECK_SAYS[case.lang]
    lines = page.locator("#log-lines .log-line.kind-disagree")
    assert lines.count() == 3, "the three lines of the room"
    assert lines.nth(0).locator(".log-kind").inner_text().strip() == says["kinds"][0]
    assert lines.nth(2).locator(".log-text").inner_text() == says["differ"].format(**names), lines.nth(2).locator(".log-text").inner_text()
    assert lines.nth(1).locator(".log-text").inner_text() == says["resent"].format(**names), lines.nth(1).locator(".log-text").inner_text()
    assert lines.nth(0).locator(".log-text").inner_text() == says["majority"].format(**names), lines.nth(0).locator(".log-text").inner_text()
    drift = page.locator("#log-lines .log-line.kind-drift").first
    assert drift.locator(".log-kind").inner_text().strip() == says["kinds"][1]
    assert drift.locator(".log-text").inner_text() == says["drift"], drift.locator(".log-text").inner_text()
    show_view(page, "tiles")


def test_log_names_devices_by_serial(page, case, reads):
    """A line names a device without a name by its S/N, the radio address
    in expert mode only; the Cube's own configuration is the Cube's; a
    removed device is named by its type, S/N and name as it was."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    d = page.evaluate("state.rooms[0].devices[0]")
    cube_sn = page.evaluate("state.serial")
    page.evaluate("""([d, cubeSN]) => {
      const now = () => new Date().toISOString();
      const send = (kind, ev) => state.events.dispatchEvent(new MessageEvent(kind, { data: JSON.stringify(ev) }));
      send("config", { kind: "config", device: "0a0fee", serial: "MEQ0000999", source: "device", time: now() });
      send("config", { kind: "config", device: "0a0fef", serial: cubeSN, source: "cube", time: now() });
      send("device_forgotten", { kind: "device_forgotten", device: d.rf_address, serial: d.serial, source: "device",
        state: { rf_address: d.rf_address, type: d.type, name: "Old name", room_id: d.room_id, serial: d.serial, reported: false },
        time: now() });
    }""", [d, cube_sn])
    lines = page.locator("#log-lines .log-line")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-device_forgotten').length > 0", 5), "the removed line"
    unnamed = lines.filter(has_text="MEQ0000999").first
    assert unnamed.locator(".log-device").inner_text() == "MEQ0000999", "a device without a name by its S/N"
    rf = unnamed.locator(".log-rf")
    assert rf.inner_text() == "0a0fee" and not rf.is_visible(), "its radio address for expert mode only"
    own = page.locator("#log-lines .log-line.kind-config").filter(has_text=cube_sn).first
    assert own.locator(".log-device").inner_text() == "Cube " + cube_sn, "the Cube's own configuration"
    gone = page.locator("#log-lines .log-line.kind-device_forgotten").first
    text = gone.locator(".log-text").inner_text()
    type_name = page.evaluate("(t) => typeName(t)", d["type"])
    assert d["serial"] in text and "(Old name)" in text and type_name in text, f"the removed device as it was: {text}"
    assert gone.locator(".log-device").count() == 0, "named by its text alone"
    show_view(page, "tiles")


def test_log_names_devices_once_the_rooms_come(page):
    """A reload on the log, the Cube remembered, draws the log before the
    rooms arrive (a Cube switch there too): a state line names its device
    by the name it kept meanwhile, and every line gets the device's name
    once the rooms are there, without leaving the log. The rooms' answer
    is held back here so that the order is certain. The log shows the
    state lines alone, kept in the address across the reload: by default
    it shows the last 20 events, and the tests that ran before on the
    same server may have logged more than that since the devices' state
    lines (a backup's lines and their commands)."""
    show_view(page, "log")
    page.locator("#log-kind").select_option("state")
    assert wait_until(page, "location.hash.includes('kind=state')", 5), "the state lines alone, in the address"
    held = []
    page.route("**/rooms?devices=all", lambda route: held.append(route))
    page.reload()
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-state').length > 0", 10), "the log drawn"
    # the page knows its Cube from the browser's storage at once, and asks
    # for the rooms only after the Cubes: wait for the request itself
    assert wait_held(page, held), "the rooms asked for and held"
    first = page.locator("#log-lines .log-line.kind-state").first
    kept = page.evaluate("logState.lines.filter((l) => l.state).at(-1).state.name")
    assert first.locator(".log-device").inner_text() == kept, "the name the line kept, before the rooms"
    # Lines that kept no name show the device's type until the rooms come,
    # and its S/N as the tooltip.
    page.evaluate("logState.lines.forEach((l) => { if (l.state) l.state = { ...l.state, name: '' }; }); renderLog();")
    last = page.evaluate("(() => { const l = logState.lines.filter((x) => x.state).at(-1); return { serial: l.serial, type: upperFirst(typeName(l.state.type)) }; })()")
    assert first.locator(".log-device").inner_text() == last["type"], "the type, no name yet"
    assert first.locator(".log-device").get_attribute("title") == last["serial"], "the S/N as the tooltip"
    for route in held:
        route.continue_()
    page.unroute("**/rooms?devices=all")
    rf = page.evaluate("logState.lines.filter((l) => l.state).at(-1).device")
    want = """(() => { const r = state.rooms.find((x) => x.devices.some((d) => d.rf_address === %r));
      const d = r && r.devices.find((x) => x.rf_address === %r);
      return d && d.name ? t("log.devicePath", { room: r.name, device: d.name }) : ""; })()""" % (rf, rf)
    assert wait_until(page, "state.roomsOf === state.serial", 10), "the rooms arrived"
    path = page.evaluate(want)
    assert path and wait_until(page, "document.querySelector('#log-lines .log-line.kind-state .log-device').textContent === %r" % path, 5), \
        "the device's room and name once the rooms are there"
    show_view(page, "tiles")


def test_log_names_room_and_device(page, case, reads):
    """A line names its device by its room and name, "Kitchen › Window";
    one without a name by its room and type; its S/N is the tooltip and,
    beside the radio address, shows in expert mode only; the filter finds
    the line by its room and by its S/N (AP LogCubes v1.0, Block 2)."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    d = page.evaluate("state.rooms[0].devices[0]")
    room = page.evaluate("state.rooms[0].name")
    page.evaluate("""(d) => state.events.dispatchEvent(new MessageEvent("config", { data: JSON.stringify({
      kind: "config", device: d.rf_address, serial: d.serial, source: "device", time: new Date().toISOString() }) }))""", d)
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-config').length > 0", 5), "the line"
    line = page.locator("#log-lines .log-line.kind-config").first
    dev = line.locator(".log-device")
    assert dev.inner_text() == room + " › " + d["name"], f"the room and the name ({dev.inner_text()!r})"
    assert dev.get_attribute("title") == d["serial"], "the S/N as the tooltip"
    sn = line.locator(".log-sn")
    assert sn.inner_text() == d["serial"] and not sn.is_visible(), "the S/N for expert mode only"
    page.evaluate("() => { state.settings.expert = true; applySettings(); }")
    assert sn.is_visible() and line.locator(".log-rf").is_visible(), "the S/N and the radio address in expert mode"
    page.evaluate("() => { state.settings.expert = false; applySettings(); }")
    for what in (room, d["serial"]):
        page.locator("#log-filter").fill(what)
        assert page.locator("#log-lines .log-line.kind-config").count() > 0, f"the filter finds it by {what!r}"
    page.locator("#log-filter").fill("")
    name = d["name"]
    page.evaluate("state.rooms[0].devices[0].name = ''; renderLog();")
    try:
        want = page.evaluate("(t) => upperFirst(typeName(t))", d["type"])
        assert line.locator(".log-device").inner_text() == room + " › " + want, "no name: the room and the type"
    finally:
        page.evaluate("(n) => { state.rooms[0].devices[0].name = n; renderLog(); }", name)
    show_view(page, "tiles")


SECOND_CUBE = "KEQ0000200"


def every_cubes_log(url):
    """Whether a request is one for every Cube's log, GET /api/v1/log."""
    return urlparse(url).path.endswith("/api/v1/log")


def log_page(lines, url):
    """The status and the page a stubbed log of lines, oldest first,
    answers a request with, as the server does: after an id, the first
    limit lines after it, and 409 for an id beyond the newest line; before
    an id, the last limit lines before it; else the newest limit."""
    q = parse_qs(urlparse(url).query)
    limit = int(q.get("limit", ["200"])[0])
    if "after" in q:
        after = int(q["after"][0])
        if after > max((ln["id"] for ln in lines), default=0):
            return 409, {"error": f"after {after} is beyond the log's newest line: reload the log"}
        return 200, [ln for ln in lines if ln["id"] > after][:limit]
    if "before" in q:
        before = int(q["before"][0])
        return 200, [ln for ln in lines if ln["id"] < before][-limit:]
    return 200, lines[-limit:]


def old_log_page(lines, url):
    """The status and the page a stubbed log answers a request with as a
    server before after= (AP LogCursor v1.0, Block 1) does: after= is a
    parameter it does not know, and ignores; since a time, before an id,
    the newest limit."""
    q = parse_qs(urlparse(url).query)
    limit = int(q.get("limit", ["200"])[0])
    page = lines
    if "since" in q:
        since = datetime.fromisoformat(q["since"][0].replace("Z", "+00:00"))
        page = [ln for ln in page if datetime.fromisoformat(ln["time"].replace("Z", "+00:00")) >= since]
    if "before" in q:
        page = [ln for ln in page if ln["id"] < int(q["before"][0])]
    return 200, page[-limit:]


def answer_log(route, lines, asked, page_of=log_page):
    """Answers a request for the log from the stubbed lines (page_of, the
    server's log_page by default), its URL noted in asked."""
    asked.append(route.request.url)
    status, body = page_of(lines, route.request.url)
    route.fulfill(status=status, content_type="application/json", body=json.dumps(body))


def open_every_cubes_log(page, query, ready):
    """Opens the log under All Cubes by its address (#log?query), one
    Cube being enough, until it is loaded and ready (an expression) holds,
    and holds the log's own poll: its timer cleared, and logPollSet
    starts no other while it is set, so that the test's polls are the
    only ones."""
    page.evaluate("(h) => { location.hash = h; }", "#log?" + query)
    assert wait_until(page, "logState.cubes === 'all' && !logState.loading && " + ready, 5), "every Cube's log, the test's lines"
    page.evaluate("clearInterval(logState.pollTimer)")


# POLL_ENDS runs a poll of the log and says whether it ended within 3 s.
POLL_ENDS = "Promise.race([pollLog().then(() => 'ended'), new Promise((r) => setTimeout(() => r('still polling'), 3000))])"

# HOLD_LOG has the page's api() hold every read of the log, after= too,
# until the test answers it from window.logAnswers; the log button's
# question (log/alerts) and everything else go through.
HOLD_LOG = """() => {
  window.logAnswers = [];
  const real = api;
  window.api = (method, path, body) => !path.includes("/log") || path.includes("/log/alerts") ? real(method, path, body)
    : new Promise((resolve, reject) => window.logAnswers.push({ path, resolve, reject }));
}"""


def test_log_of_every_cube(page, case):
    """With a second Cube the log shows its Cubes choice, and not before.
    All Cubes reads GET /api/v1/log and is in the address; a line names
    its Cube first, a Cube's own line the Cube alone, the server's own no
    Cube; the stream adds no line there, a poll does, after the newest
    line's id (AP LogCursor v1.0, D6-A); the device choice groups the
    devices by Cube; Clear sets the choice back (AP LogCubes v1.0,
    Block 3). The second Cube never connects; the lines are the test's
    own, so that what they name is certain."""
    show_view(page, "log")
    choice = page.locator("#log-cubes")
    assert choice.is_hidden(), "no Cubes choice with one Cube"
    api_call("POST", "api/v1/cubes", {"serial": SECOND_CUBE, "address": "127.0.0.1:9", "name": "Second"})
    try:
        page.evaluate("loadCubes()")
        assert wait_until(page, "!document.getElementById('log-cubes').classList.contains('hidden')", 5), "the choice with two Cubes"
        with page.expect_response(lambda r: urlparse(r.url).path.endswith("/api/v1/log")) as answer:
            choice.select_option("all")
        assert answer.value.status == 200 and isinstance(answer.value.json(), list), "every Cube's log from the server"
        assert "cubes=all" in page.evaluate("location.hash"), "All Cubes in the address"
        head = page.evaluate("(() => { const c = state.cubes.find((x) => x.serial === state.serial); return { sn: c.serial, name: c.name || c.serial }; })()")
        d = page.evaluate("state.rooms[0].devices[0]")
        room = page.evaluate("state.rooms[0].name")
        now = datetime.now(timezone.utc).replace(microsecond=0)

        def iso(s):
            return (now + timedelta(seconds=s)).strftime("%Y-%m-%dT%H:%M:%SZ")

        lines = [
            {"id": 900001, "kind": "server_started", "source": "server", "summary": {"version": "test"}, "time": iso(-3)},
            {"id": 900002, "kind": "disconnected", "source": "cube", "cube_serial": SECOND_CUBE, "time": iso(-2)},
            {"id": 900003, "kind": "config", "device": d["rf_address"], "serial": d["serial"], "source": "device", "cube_serial": head["sn"], "time": iso(-1)},
        ]
        asked = []
        page.route(every_cubes_log, lambda route: answer_log(route, lines, asked))
        page.evaluate("loadLog()")
        assert wait_until(page, "logState.lines.some((l) => l.id === 900003)", 5), "the lines of the test"
        dev = page.locator("#log-lines .log-line.kind-config").first.locator(".log-device")
        assert dev.inner_text() == head["name"] + " › " + room + " › " + d["name"], f"the Cube, the room and the device ({dev.inner_text()!r})"
        own = page.locator("#log-lines .log-line.kind-disconnected").first.locator(".log-device")
        assert own.inner_text() == "Second", "a Cube's own line names the Cube alone"
        assert page.locator("#log-lines .log-line.kind-server_started").first.locator(".log-device").count() == 0, "the server's line no Cube"
        groups = page.evaluate("[...document.querySelectorAll('#log-device optgroup')].map((g) => g.label)")
        assert head["name"] in groups, f"the devices grouped by Cube ({groups})"
        # The stream adds nothing under All Cubes; a poll brings the new lines.
        n = page.evaluate("logState.lines.length")
        page.evaluate("""(d) => state.events.dispatchEvent(new MessageEvent("config", { data: JSON.stringify({
          kind: "config", device: d.rf_address, serial: d.serial, source: "device", time: new Date().toISOString() }) }))""", d)
        assert page.evaluate("logState.lines.length") == n, "no line from the stream"
        lines.append({"id": 900004, "kind": "config", "device": "0a0fee", "serial": "MEQ0000999", "source": "device", "cube_serial": SECOND_CUBE, "time": iso(0)})
        asked.clear()
        # pollLog asks again until a poll brings it: one of the timer's may be on its way
        assert wait_until(page, "(pollLog(), logState.lines.some((l) => l.id === 900004))", 5), "the poll's new line"
        assert page.evaluate("logState.lines.filter((l) => l.id >= 900001).length") == 4, "each line once"
        q = parse_qs(urlparse(asked[0]).query)
        assert q.get("after") == ["900003"] and "since" not in q, f"asked after the newest line's id ({q})"
        new = page.locator("#log-lines .log-line.kind-config").first.locator(".log-device")
        assert new.inner_text() == "Second › MEQ0000999", f"the other Cube's device ({new.inner_text()!r})"
        page.unroute(every_cubes_log)
        page.locator("#log-clear").click()
        assert wait_until(page, "logState.cubes === '' && !location.hash.includes('cubes=')", 5) and choice.input_value() == "", "Clear: this Cube again"
    finally:
        page.unroute(every_cubes_log)
        api_call("DELETE", f"api/v1/cubes/{SECOND_CUBE}")
        page.evaluate("loadCubes()")
    assert wait_until(page, "document.getElementById('log-cubes').classList.contains('hidden')", 5), "no choice with one Cube again"
    assert scroll_width(page) <= case.width, "no horizontal scroll"
    show_view(page, "tiles")


def test_log_of_every_cube_loads_again_after_409(page, case, watch):
    """Under All Cubes a 409 to the poll's after=, a cursor beyond the
    server's newest line (a store restored from an older copy, a server
    without a store restarted with its clock set back), loads the log
    again, once: each line once, none missing; a burst longer than a page
    comes in one poll, page after page, and the last 20 are the newest 20
    (AP LogCursor v1.0, D6-A). The log's answers are stubbed (log_page),
    and its own poll is held (open_every_cubes_log)."""
    now = time.time()
    first = [{"id": 900000 + i, "kind": "hello", "time": iso(now - 100 + i)} for i in (1, 2, 3)]
    lines = list(first)
    asked = []
    show_view(page, "log")
    page.route(every_cubes_log, lambda route: answer_log(route, lines, asked))
    try:
        open_every_cubes_log(page, "cubes=all", "logState.lines.some((l) => l.id === 900003)")
        # The server's log starts over: the three lines with ids of its own, and one more.
        lines[:] = [dict(ln, id=11 + i) for i, ln in enumerate(first)] + [{"id": 14, "kind": "hello", "time": iso(now - 50)}]
        watch.expected.append(("after=900003", 409))
        asked.clear()
        page.evaluate("pollLog()")
        assert wait_until(page, "!logState.loading && logState.lines.some((l) => l.id === 14)", 5), "the log loaded again"
        page.evaluate("pollLog()")  # the end: a read after the reload, through the same route
        after = [parse_qs(urlparse(u).query).get("after") for u in asked]
        assert after == [["900003"], None, ["14"]], f"the poll after its newest id, the log again once, the next poll: {after}"
        ids = page.evaluate("logState.lines.map((l) => l.id)")
        assert ids == [11, 12, 13, 14], f"each line once, none missing: {ids}"
        assert page.locator("#log-lines .log-line").count() == 4, "and the view shows them"
        lines.extend({"id": 14 + i, "kind": "hello", "time": iso(now - 40 + i // 10)} for i in range(1, 251))
        asked.clear()
        ids = page.evaluate("pollLog().then(() => logState.lines.map((l) => l.id))")
        after = [parse_qs(urlparse(u).query).get("after") for u in asked]
        assert after == [["14"], ["214"]], f"one poll, a page after the other: {after}"
        assert ids == list(range(245, 265)), f"the newest 20 after a burst of 250: {ids}"
        assert page.locator("#log-lines .log-line").count() == 20, "and the view shows them"
    finally:
        page.unroute(every_cubes_log)
    show_view(page, "tiles")


def test_log_of_every_cube_loads_again_after_a_long_pause(page, case):
    """A poll reads on at most LOG_POLL_PAGES (10) full pages: a backlog of
    more, as after a laptop's sleep with the log open, loads the log
    again, its newest page drawn once, in place of a walk through every
    line since (AP LogCursor v1.0, Block 3). With all events, where the
    walk would draw 2000 lines one by one; the answers are stubbed."""
    now = time.time()
    lines = [{"id": i, "kind": "hello", "time": iso(now - 9000 + i)} for i in range(1, 21)]
    asked = []
    show_view(page, "log")
    page.route(every_cubes_log, lambda route: answer_log(route, lines, asked))
    try:
        open_every_cubes_log(page, "cubes=all&range=all", "logState.lines.some((l) => l.id === 20)")
        lines.extend({"id": 20 + i, "kind": "hello", "time": iso(now - 8000 + i)} for i in range(1, 2501))
        asked.clear()
        assert page.evaluate(POLL_ENDS) == "ended", "the poll ends"
        assert wait_until(page, "!logState.loading && logState.lines.some((l) => l.id === 2520)", 5), "the log loaded again"
        after = [parse_qs(urlparse(u).query).get("after", [""])[0] for u in asked]
        assert after == [str(20 + 200 * n) for n in range(10)] + [""], f"ten pages, then the log again: {after}"
        ids = page.evaluate("logState.lines.map((l) => l.id)")
        assert ids == list(range(2321, 2521)), f"the newest page: {ids[:3]}…{ids[-3:]} ({len(ids)})"
        assert page.locator("#log-lines .log-line").count() == 200, "and the view shows it"
    finally:
        page.unroute(every_cubes_log)
    show_view(page, "tiles")


def test_log_of_every_cube_on_an_older_server(page, case):
    """A server before after= ignores it and answers its newest page: the
    poll keeps the lines after its cursor alone, and ends where a full
    page brings nothing after the cursor, two requests here, the last 20
    the newest 20. The answers are those of such a server, stubbed
    (old_log_page)."""
    now = time.time()
    lines = [{"id": i, "kind": "hello", "time": iso(now - 3000 + i)} for i in range(1, 301)]
    asked = []
    show_view(page, "log")
    page.route(every_cubes_log, lambda route: answer_log(route, lines, asked, old_log_page))
    try:
        open_every_cubes_log(page, "cubes=all", "logState.lines.some((l) => l.id === 300)")
        lines.extend({"id": 300 + i, "kind": "hello", "time": iso(now - 100 + i)} for i in range(1, 6))
        asked.clear()
        assert page.evaluate(POLL_ENDS) == "ended", f"the poll ends ({len(asked)} requests)"
        after = [parse_qs(urlparse(u).query).get("after") for u in asked]
        assert after == [["300"], ["305"]], f"two requests, the second after the first's last: {after}"
        ids = page.evaluate("logState.lines.map((l) => l.id)")
        assert ids == list(range(286, 306)), f"the newest 20: {ids}"
    finally:
        page.unroute(every_cubes_log)
    show_view(page, "tiles")


def test_log_of_every_cube_on_an_older_server_in_a_range(page, case):
    """Under a time range, the last hour, the newest page of a server
    that ignores after= reaches back before the hour: the poll adds none
    of those lines, which the range's check would never take out, only
    the new ones after its cursor. The answers are those of such a
    server, stubbed (old_log_page): 290 lines of two hours ago, 10 of the
    last ten minutes, then two new ones."""
    now = time.time()
    lines = [{"id": i, "kind": "hello", "time": iso(now - 7500 + i)} for i in range(1, 291)]
    lines += [{"id": 290 + i, "kind": "hello", "time": iso(now - 600 + i)} for i in range(1, 11)]
    show_view(page, "log")
    page.route(every_cubes_log, lambda route: answer_log(route, lines, [], old_log_page))
    try:
        open_every_cubes_log(page, "cubes=all&range=1h", "logState.range === '1h' && logState.lines.length === 10")
        lines.extend({"id": 300 + i, "kind": "hello", "time": iso(now - 5 + i)} for i in range(1, 3))
        assert page.evaluate(POLL_ENDS) == "ended", "the poll ends"
        ids = page.evaluate("logState.lines.map((l) => l.id)")
        assert ids == list(range(291, 303)), f"the hour's lines and the two new ones, none from before the hour: {ids[:3]}…({len(ids)})"
    finally:
        page.unroute(every_cubes_log)
    show_view(page, "tiles")


def test_log_of_every_cube_polls_one_at_a_time(page, case):
    """While a poll is on its way, another, as the timer's, asks nothing;
    the page's api() holds the log's reads (HOLD_LOG)."""
    now = time.time()
    lines = [{"id": 900000 + i, "kind": "hello", "time": iso(now - 100 + i)} for i in (1, 2, 3)]
    show_view(page, "log")
    page.route(every_cubes_log, lambda route: answer_log(route, lines, []))
    try:
        open_every_cubes_log(page, "cubes=all", "logState.lines.some((l) => l.id === 900003)")
    finally:
        page.unroute(every_cubes_log)
    page.evaluate(HOLD_LOG)
    assert page.evaluate("() => { pollLog(); pollLog(); return window.logAnswers.length; }") == 1, "one poll on its way, the second asks nothing"
    page.evaluate("window.logAnswers.shift().resolve([])")
    assert wait_until(page, "!logState.polling", 5), "the poll ended"
    show_view(page, "tiles")


SAYS = {
    "en": {"down": "down for 5 min", "missed": "(while disconnected)", "renamed": "renamed", "comfort": "comfort 21.0 → 21.5"},
    "de": {"down": "5 min getrennt", "missed": "(während getrennt)", "renamed": "umbenannt", "comfort": "Komfort 21,0 → 21,5"},
}


def test_log_says_what_changed(page, case, reads):
    """A reconnect is one line with its summary, what changed meanwhile
    follows it with a note, a configuration's line says its differences,
    and a debug line shows in expert mode only."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    d = page.evaluate("state.rooms[0].devices[0]")
    page.evaluate("""(d) => {
      const now = () => new Date().toISOString();
      const send = (kind, ev) => state.events.dispatchEvent(new MessageEvent(kind, { data: JSON.stringify(ev) }));
      send("hello", { kind: "hello", source: "cube", level: "debug", summary: { firmware: "1.1.3" }, time: now() });
      send("connected", { kind: "connected", source: "cube", level: "info", time: now(),
        summary: { devices: 7, rooms: 3, firmware: "1.1.3", duty_cycle_pct: 3, free_memory_slots: 180, cause: "loss", down_for_s: 300 } });
      send("metadata", { kind: "metadata", source: "cube", level: "info", while_disconnected: true, time: now(),
        list_changes: [{ what: "room_renamed", room: 1, name: "Bathroom", from: "Bath" }] });
      send("config", { kind: "config", source: "device", level: "info", device: d.rf_address, serial: d.serial, time: now(),
        changes: { comfort_temp: { from: 21, to: 21.5 } } });
    }""", d)
    says = SAYS[case.lang]
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-connected').length > 0", 5), "the connected line"
    connected = page.locator("#log-lines .log-line.kind-connected").first.locator(".log-text").inner_text()
    assert "1.1.3" in connected and says["down"] in connected, f"the reconnect's summary: {connected}"
    meta = page.locator("#log-lines .log-line.kind-metadata").first.locator(".log-text").inner_text()
    assert "Bathroom" in meta and says["renamed"] in meta and says["missed"] in meta, f"what changed meanwhile: {meta}"
    cfg = page.locator("#log-lines .log-line.kind-config").first.locator(".log-text").inner_text()
    assert says["comfort"] in cfg, f"the configuration's difference: {cfg}"
    assert page.locator("#log-lines .log-line.kind-hello").count() == 0, "the debug line outside expert mode"
    show_view(page, "tiles")


SERVER_SAYS = {
    "en": {"started": "started, version v9.9.9, before v9.9.8", "backup": "backup 12 taken (by hand), by Anna"},
    "de": {"started": "gestartet, Version v9.9.9, vorher v9.9.8", "backup": "Backup 12 erstellt (von Hand), durch Anna"},
}


def test_log_server_lines(page, case, reads):
    """The server's own lines say what they are, and who asked for one."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    page.evaluate("""() => {
      const now = () => new Date().toISOString();
      const send = (kind, ev) => state.events.dispatchEvent(new MessageEvent(kind, { data: JSON.stringify(ev) }));
      send("server_started", { kind: "server_started", source: "system", level: "info", time: now(), summary: { version: "v9.9.9", before: "v9.9.8" } });
      send("backup", { kind: "backup", source: "system", level: "info", time: now(), summary: { reason: "manual", id: 12, bytes: 2048 },
        by: { name: "anna", display_name: "Anna" } });
    }""")
    says = SERVER_SAYS[case.lang]
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-backup').length > 0", 5), "the backup's line"
    started = page.locator("#log-lines .log-line.kind-server_started").first.locator(".log-text").inner_text()
    backup = page.locator("#log-lines .log-line.kind-backup").first.locator(".log-text").inner_text()
    assert started == says["started"], f"the start: {started}"
    assert backup == says["backup"], f"the backup: {backup}"
    show_view(page, "tiles")


CHANGE_SAYS = {
    "en": "Cube changed KEQ0000000: name – → Upstairs, by Anna",
    "de": "Cube geändert KEQ0000000: Name – → Upstairs, durch Anna",
}


def test_log_change_line(page, case, reads):
    """A change made through the API says what was changed, from and to,
    and by whom."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    page.evaluate("""() => state.events.dispatchEvent(new MessageEvent("change", { data: JSON.stringify({
      kind: "change", source: "config", level: "info", time: new Date().toISOString(),
      summary: { action: "cube.update", target: "cube:KEQ0000000", changes: { name: { from: "", to: "Upstairs" } } },
      by: { name: "anna", display_name: "Anna" } }) }))""")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-change').length > 0", 5), "the change line"
    text = page.locator("#log-lines .log-line.kind-change").first.locator(".log-text").inner_text()
    assert text == CHANGE_SAYS[case.lang], f"the change: {text}"
    show_view(page, "tiles")


THROUGH_SAYS = {
    "en": "Cube changed KEQ0000000: name – → Upstairs, by Home › Anna, as anna",
    "de": "Cube geändert KEQ0000000: Name – → Upstairs, durch Home › Anna, as anna",
}


def test_log_names_integration_users(page, case, reads):
    """A change made through an integration for its user linked to anna
    names the integration's user by its chain, ", as anna" after it, and
    is kept by the user filter of either user (AP StoreRaise v1.2, Block
    4)."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    real = "01a1e000-0000-7000-8000-000000000001:" + "a" * 32
    page.evaluate("""(real) => state.events.dispatchEvent(new MessageEvent("change", { data: JSON.stringify({
      kind: "change", source: "config", level: "info", time: new Date().toISOString(),
      summary: { action: "cube.update", target: "cube:KEQ0000000", changes: { name: { from: "", to: "Upstairs" } } },
      by: { name: "anna", display_name: "Anna" },
      effective_user: { name: "anna", display_name: "Anna", display: "Anna" },
      real_user: { name: real, display_name: "Anna", integration: "01a1e000-0000-7000-8000-000000000001", display: "Home › Anna, as anna" } }) }))""", real)
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-change').length > 0", 5), "the change line"
    text = page.locator("#log-lines .log-line.kind-change").first.locator(".log-text").inner_text()
    assert text == THROUGH_SAYS[case.lang], f"the change: {text}"
    for who in ("anna", real):
        kept = page.evaluate("""([who, real]) => { const was = logState.user; logState.user = who;
          const line = logState.lines.find((l) => l.kind === "change" && l.realName === real);
          const ok = logOfChoices(line); logState.user = was; return ok; }""", [who, real])
        assert kept, f"the user filter of {who} keeps it"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the chain"
    show_view(page, "tiles")


SLOTS_SAYS = {
    "en": ("few of the Cube's memory slots are free, 4 since ", "the Cube's memory slots stay full, 1 free since ",
           "the Cube's memory slots are free again, 50 free (fewer than 5 since "),
    "de": ("nur wenige Speicherplätze des Cubes sind frei, 4 seit ", "die Speicherplätze des Cubes bleiben voll, 1 frei seit ",
           "die Speicherplätze des Cubes sind wieder frei, 50 frei (weniger als 5 seit "),
}


def test_log_slots_lines(page, case, reads):
    """A Cube's memory slots that stay held, short and full, and are free
    again, say so with the free slots and since when (AP RadioHealth,
    Block 5, D192-A)."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    page.evaluate("""() => {
      const since = new Date(Date.now() - 7200e3).toISOString();
      const full_since = new Date(Date.now() - 3600e3).toISOString();
      const send = (summary, level) => state.events.dispatchEvent(new MessageEvent("slots", { data: JSON.stringify({
        kind: "slots", source: "cube", level, time: new Date().toISOString(), summary }) }));
      send({ state: "low", free_slots: 4, since }, "warn");
      send({ state: "full", from: "low", free_slots: 1, since, full_since }, "error");
      send({ state: "free", free_slots: 50, since }, "info");
    }""")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-slots').length === 3", 5), "the three lines"
    texts = page.locator("#log-lines .log-line.kind-slots .log-text").all_inner_texts()
    for want, what in zip(SLOTS_SAYS[case.lang], ("short", "full", "free again")):
        assert any(s.startswith(want) for s in texts), f"the slots {what}: {texts}"
    show_view(page, "tiles")


MISMATCH_SAYS = {
    "en": ("Ghost: in Upstairs by its configuration on the Cube, and not in the room list, which can block the room's commands.",
           "agrees again: Ghost: in Upstairs by its configuration on the Cube, and not in the room list"),
    "de": ("Ghost: laut seiner Konfiguration im Cube in Upstairs und nicht in der Raumliste, was die Befehle an den Raum blockieren kann.",
           "stimmt wieder überein: Ghost: laut seiner Konfiguration im Cube in Upstairs und nicht in der Raumliste"),
}


def test_log_mismatch_lines(page, case, reads):
    """A C line in a room for a device the room list does not have, and
    the same agreeing again, name the device and the room (AP
    RadioHealth, Block 7)."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    page.evaluate("""() => {
      const m = { finding: "unlisted", device: "0a0a01", name: "Ghost", room: 9, room_name: "Upstairs", cube_room: 9,
        cube_room_name: "Upstairs", cube_type: "radiator_thermostat", since: new Date().toISOString() };
      // as the stream's handler takes them, without its reload of the
      // rooms, which reads the log again from the server
      const send = (st, level) => appendLogEvent({ kind: "mismatch", source: "cube", level, device: m.device,
        time: new Date().toISOString(), summary: Object.assign({ state: st }, m) });
      send("found", "warn");
      send("cleared", "info");
    }""")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-mismatch').length === 2", 5), \
        "the two lines, and none of the server's: its fixture agrees with its room list"
    texts = page.locator("#log-lines .log-line.kind-mismatch .log-text").all_inner_texts()
    found, cleared = MISMATCH_SAYS[case.lang]
    assert found in texts, f"the finding: {texts}"
    assert any(s.startswith(cleared) for s in texts), f"the finding agreeing again: {texts}"
    show_view(page, "tiles")


ERROR_RESET_SAYS = {
    "en": ("communication error reset", "at the device's report; the server does it once a day at most"),
    "de": ("Kommunikationsstörung zurückgesetzt", "bei der Meldung des Geräts; der Server tut es höchstens einmal am Tag"),
}


def test_log_error_reset_line(page, case, reads):
    """A device's automatic reset of its communication error, sent on the
    stream, is a line of its device with its kind and what it says (AP
    RadioHealth v1.2, Block 4)."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    d = page.evaluate("state.rooms[0].devices[0]")
    page.evaluate("""(d) => state.events.dispatchEvent(new MessageEvent("error_reset", { data: JSON.stringify({
      kind: "error_reset", source: "device", level: "info", device: d.rf_address, serial: d.serial, time: new Date().toISOString(),
      summary: { room: d.room_id || 1, automatic: true } }) }))""", d)
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-error_reset').length > 0", 5), "the reset's line"
    line = page.locator("#log-lines .log-line.kind-error_reset").first
    kind, text = ERROR_RESET_SAYS[case.lang]
    assert line.locator(".log-kind").inner_text().strip() == kind, f"the kind: {line.locator('.log-kind').inner_text()}"
    assert line.locator(".log-text").inner_text() == text, f"the text: {line.locator('.log-text').inner_text()}"
    show_view(page, "tiles")


ERROR_WATCH_SAYS = {
    "en": ("error watch",
           "communication error back at the device's next report, 60 min after the automatic reset; "
           "last radiogram that failed: TimeInformation (0x03); no automatic reset until it stays clear a day",
           "link error set for 24 hours, since ",
           "communication error clear again; set for 26 h; automatic resets resume once it stays clear a day"),
    "de": ("Fehlerüberwachung",
           "Kommunikationsstörung bei der nächsten Meldung des Geräts zurück, 60 Min. nach dem automatischen Zurücksetzen; "
           "zuletzt fehlgeschlagenes Funktelegramm: TimeInformation (0x03); kein automatisches Zurücksetzen, bis sie einen Tag lang ausbleibt",
           "Verbindungsfehler 24 Stunden lang gesetzt, seit ",
           "Kommunikationsstörung wieder weg; 26 Std. gesetzt; automatisches Zurücksetzen wieder, sobald sie einen Tag lang ausbleibt"),
}


def test_log_error_watch_lines(page, case, reads):
    """A device's watched flags, sent on the stream, are lines of its device
    with their kind: its communication error back after the automatic reset
    with the radiogram that failed, its link error set for 24 hours, and
    the communication error clear again (AP RadioHealth v1.2, Block 6)."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    d = page.evaluate("state.rooms[0].devices[0]")
    page.evaluate("""(d) => {
      const send = (level, summary) => state.events.dispatchEvent(new MessageEvent("error_watch", { data: JSON.stringify({
        kind: "error_watch", source: "device", level, device: d.rf_address, serial: d.serial, time: new Date().toISOString(), summary }) }));
      const reset = new Date(Date.now() - 3600000).toISOString();
      send("warn", { what: "back", flag: "error", reset, minutes: 60, next_report: true, held: true,
        failed_radiogram: { code: 3, name: "TimeInformation" } });
      send("error", { what: "lasting", flag: "link_error", since: reset, hours: 24 });
      send("info", { what: "clear", flag: "error", since: reset, hours: 26, held: true });
    }""", d)
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-error_watch').length >= 3", 5), "the watch's lines"
    kind, back, lasting, clear = ERROR_WATCH_SAYS[case.lang]
    texts = page.locator("#log-lines .log-line.kind-error_watch .log-text").all_inner_texts()
    kinds = page.locator("#log-lines .log-line.kind-error_watch .log-kind").all_inner_texts()
    assert all(k.strip() == kind for k in kinds), f"the kinds: {kinds}"
    assert back in texts, f"the flag back: {texts}"
    assert clear in texts, f"the clear: {texts}"
    assert any(x.startswith(lasting) for x in texts), f"the link error: {texts}"
    show_view(page, "tiles")


def test_log_source_and_level(page, case):
    """The source and the level chosen are asked of the server and kept in
    the address after the log's page; a reload keeps them, Clear sets
    them back to all and info."""
    show_view(page, "log")
    with page.expect_request(lambda r: "/log?" in r.url and "source=" in r.url) as asked:
        page.locator("#log-device").select_option("src:system")
    q = parse_qs(urlparse(asked.value.url).query)
    assert q.get("source") == ["system"] and "device" not in q, f"the source asked for: {q}"
    with page.expect_request(lambda r: "/log?" in r.url and "level=" in r.url) as asked:
        page.locator("#log-level").select_option("warn")
    q = parse_qs(urlparse(asked.value.url).query)
    assert q.get("level") == ["warn"] and q.get("source") == ["system"], f"the level asked for: {q}"
    assert wait_until(page, "location.hash === '#log?source=system&level=warn'", 5), f"the address: {page.evaluate('location.hash')}"
    page.reload()
    assert log_shown(page) and devices_loaded(page), "the log again"
    assert page.locator("#log-device").input_value() == "src:system" and page.locator("#log-level").input_value() == "warn", "the choices kept"
    page.locator("#log-clear").click()
    assert wait_until(page, "location.hash === '#log'", 5), "Clear takes them out"
    assert page.locator("#log-device").input_value() == "" and page.locator("#log-level").input_value() == "info", "all and info again"
    named_buttons(page, "the log with its source and level")
    show_view(page, "tiles")


def stamp(now, i):
    return time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime(now - 7200 + i)) + "Z"


def all_events(page):
    """The range chosen: all events, the log loaded again."""
    page.locator("#log-range").select_option("all")
    assert wait_until(page, "logState.range === 'all' && !logState.loading", 5), "all events"


def test_log_filters(page, case):
    """The kind and the device chosen narrow the log the server answers;
    back to all, the lines of before, and Older offered as the page's
    size says: nothing older after a page shorter than 200 (all events)."""
    show_view(page, "log")
    all_events(page)
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line').length >= 7", 5), "the log loaded"
    before = page.locator("#log-lines .log-line").count()
    page.locator("#log-kind").select_option("state")
    assert wait_until(page, "logState.kind === 'state' && [...document.querySelectorAll('#log-lines .log-line')]"
                            ".every((l) => l.classList.contains('kind-state'))", 5), "only state events"
    assert page.locator("#log-lines .log-line").count() >= 7, "the seven devices' states"
    page.locator("#log-kind").select_option("")
    rf = page.evaluate("state.rooms[0].devices[0].rf_address")
    sn = page.evaluate("state.rooms[0].devices[0].serial")
    with page.expect_request(lambda r: "/log?" in r.url) as asked:
        page.locator("#log-device").select_option(sn)
    assert parse_qs(urlparse(asked.value.url).query).get("device") == [sn], "the device asked for by its S/N"
    assert wait_until(page, "logState.device === %r && document.querySelectorAll('#log-lines .log-line').length > 0 && "
                            "[...document.querySelectorAll('#log-lines .log-line')].every((l) => l.querySelector('.log-rf') && "
                            "l.querySelector('.log-rf').textContent === %r)" % (sn, rf), 5), "only the device's events"
    kinds = set(page.evaluate("[...document.querySelectorAll('#log-lines .log-line')].map((l) => [...l.classList].find((c) => c.startsWith('kind-')))"))
    assert kinds <= {"kind-config", "kind-state"}, f"the device's kinds: {kinds}"
    with page.expect_response(lambda r: "/log?" in r.url) as answer:
        page.locator("#log-device").select_option("")
    n = len(answer.value.json())
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line').length >= %d" % before, 5), "all again"
    assert wait_until(page, "!logState.loading", 5), "the page is in"
    older = page.locator("#log-older")
    if n < 200:
        assert older.is_disabled() and older.inner_text() == NO_OLDER[case.lang], f"a page of {n}: nothing older"
    else:
        assert older.is_enabled() and older.inner_text() == OLDER[case.lang], f"a page of {n}: Older offered"
    named_buttons(page, "the log with its choices")
    show_view(page, "tiles")


def test_log_older(page, case):
    """Older, at the bottom, puts the page before the oldest line under
    the lines, the view staying where it was, until a short page ends the
    paging; the server's answers are stubbed: a full page of 200, then two
    events."""
    now = time.time()

    def answer(route):
        q = parse_qs(urlparse(route.request.url).query)
        before = q.get("before", [""])[0]
        if not before:
            evs = [{"id": i, "kind": "hello", "time": stamp(now, i)} for i in range(201, 401)]
        elif before == "201":
            evs = [{"id": i, "kind": "hello", "time": stamp(now, i)} for i in (1, 2)]
        else:
            evs = []
        route.fulfill(status=200, content_type="application/json", body=json.dumps(evs))

    pattern = re.compile(r"/api/v1/cubes/[^/]+/log\?")
    show_view(page, "log")
    page.route(pattern, answer)
    try:
        all_events(page)
        assert wait_until(page, "document.querySelectorAll('#log-lines .kind-hello').length === 200", 5), "a page of 200"
        older = page.locator("#log-older")
        assert older.is_enabled() and older.inner_text() == OLDER[case.lang], "Older offered"
        box = "document.getElementById('log-lines')"
        assert page.evaluate("document.getElementById('log-lines').compareDocumentPosition(document.getElementById('log-older'))"
                             " & Node.DOCUMENT_POSITION_FOLLOWING"), "Older is under the lines"
        page.evaluate(box + ".scrollTop = 120")
        top = page.evaluate(box + ".scrollTop")
        older.click()
        assert wait_until(page, "document.querySelectorAll('#log-lines .kind-hello').length === 202", 5), "two older events"
        assert page.evaluate("logState.lines[0].id === 1 && logState.lines[1].id === 2 && logState.lines[2].id === 201"), "kept oldest first"
        assert page.evaluate("[...document.querySelectorAll('#log-lines .kind-hello')].slice(-2).map((l) => l.querySelector('.log-time').textContent)") \
            == page.evaluate("[logState.lines[1], logState.lines[0]].map((l) => fmtTimeOfDay(l.time))"), "drawn at the bottom"
        assert page.evaluate(box + ".scrollTop") == top, "the view stays where it was"
        assert wait_until(page, "document.getElementById('log-older').disabled", 5), "a short page ends the paging"
        assert older.inner_text() == NO_OLDER[case.lang], "nothing older"
    finally:
        page.unroute(pattern)
    show_view(page, "tiles")


# RING is where the ids of the lines a server keeps in memory start: its
# start in microseconds, far above a store's ids.
RING = 1_763_000_000_000_000


def test_log_older_after_the_log_moved(page, case, watch):
    """Older first asks whether the server still has the newest line the
    log shows, after= its id; a 409 loads the log again in place of the
    page before the oldest line (AP LogCursor v1.0, Block 3). That is a
    Cube served for this run, whose lines the server kept in memory, that
    has gained its row in the store: its log is then the store's, whose
    ids are far below, and the page before the oldest line would be the
    store's newest, under the lines as if older, a live line in it a
    second time. The server's answers are stubbed (log_page), and the
    server's own lines are kept out of the log (isolate_log)."""
    now = time.time()
    lines = [{"id": RING + i, "kind": "hello", "time": stamp(now, i)} for i in range(1, 201)]
    asked = []
    show_view(page, "log")
    isolate_log(page)
    page.route(LOG_ROUTE, lambda route: answer_log(route, lines, asked))
    try:
        all_events(page)
        assert wait_until(page, "logState.lines.filter((l) => l.id > %d).length === 200" % RING, 5), "the page the server kept in memory"
        rf = page.evaluate("state.rooms[0].devices[0].rf_address")
        at = iso(now)
        page.evaluate("""([rf, at]) => state.events.dispatchEvent(new MessageEvent("config", { data: JSON.stringify({
          kind: "config", device: rf, time: at }) }))""", [rf, at])
        assert wait_until(page, "logState.lines.some((l) => l.kind === 'config' && !l.id)", 5), "the live line"
        # The Cube gains its row: its log is the store's, the live line in it with an id of the store's.
        lines[:] = [{"id": 7, "kind": "hello", "time": iso(now - 1)}, {"id": 8, "kind": "config", "device": rf, "time": at}]
        watch.expected.append((f"after={RING + 200}", 409))
        asked.clear()
        page.locator("#log-older").click()
        assert wait_until(page, "!logState.loading && logState.lines.some((l) => l.id === 8)", 5), "the store's log"
        q = [parse_qs(urlparse(u).query) for u in asked]
        assert q[0].get("after") == [str(RING + 200)] and not any("before" in x or "after" in x for x in q[1:]), \
            f"the newest line's id asked, then the log again, no page before the oldest: {q}"
        ids = page.evaluate("logState.lines.map((l) => l.id)")
        assert ids == [7, 8], f"the store's lines, once each, none of the memory's: {ids}"
        assert page.locator("#log-lines .log-line").count() == 2, "and the view shows them"
        assert wait_until(page, "document.getElementById('log-older').disabled", 5), "nothing older"
    finally:
        page.unroute(LOG_ROUTE)
    show_view(page, "tiles")


def test_log_older_question_after_another_choice(page, case):
    """Another choice made while Older's question (after=, logGone) is on
    its way loads the log for it; a 409 to the question that comes after
    loads it no second time, and Older asks for no page: the choice's
    page is the log. The page's api() holds the log's reads (HOLD_LOG),
    and only the events the test sends reach the log (isolate_log)."""
    now = time.time()
    show_view(page, "log")
    all_events(page)
    isolate_log(page)
    page.evaluate(HOLD_LOG)
    page.evaluate("() => { loadLog(); }")  # not awaited: the test answers it
    assert wait_until(page, "window.logAnswers.length === 1", 5), "the log's page held"
    page.evaluate("(evs) => window.logAnswers.shift().resolve(evs)",
                  [{"id": i, "kind": "hello", "time": stamp(now, i)} for i in range(201, 401)])
    assert wait_until(page, "!logState.loading && logState.more", 5), "a full page, Older offered"
    page.locator("#log-older").click()
    assert wait_until(page, "window.logAnswers.length === 1", 5), "Older's question held"
    assert "after=400" in page.evaluate("window.logAnswers[0].path"), "Older asks after the newest line"
    page.locator("#log-kind").select_option("hello")
    assert wait_until(page, "window.logAnswers.length === 2", 5), "the choice's page held"
    page.evaluate("() => { const e = new Error('beyond'); e.status = 409; window.logAnswers.shift().reject(e); }")
    page.evaluate("(evs) => window.logAnswers.shift().resolve(evs)", [{"id": 5, "kind": "hello", "time": stamp(now, 5)}])
    # A reload by the question would have taken the choice's page's place: the log would still be loading.
    assert wait_until(page, "!logState.loading && logState.lines.map((l) => l.id).join() === '5'", 5), \
        f"the choice's page, and no reload after it: {page.evaluate('window.logAnswers.map((a) => a.path)')}"
    assert page.evaluate("window.logAnswers.length") == 0, "nothing more asked"
    show_view(page, "tiles")


def test_log_answers(page, case):
    """What comes back late or fails leaves the log as it should be: an
    Older answer after Clear is dropped, a Cube switch takes the device
    chosen back to all, a page fetched with Older stays when a live
    event comes past 2000 lines, and the error line shows with a kind
    chosen. The page's api() holds the log's requests, which the test
    answers in its own time; the log button's question (log/alerts) goes
    through, and Older's question whether the log is still there
    (after=, logGone) is answered at once, as a log that is. Only the
    events the test sends reach the log (isolate_log): a line of the test
    server's own, such as a backup's or a device's state from the fake
    Cube's 10 s poll, would push one of the 2000 counted out."""
    now = time.time()
    show_view(page, "log")
    all_events(page)
    isolate_log(page)
    page.evaluate("""() => {
      window.logAnswers = [];
      const real = api;
      window.api = (method, path, body) => !path.includes("/log") || path.includes("/log/alerts") ? real(method, path, body)
        : path.includes("&after=") ? Promise.resolve([])
        : new Promise((resolve, reject) => window.logAnswers.push({ path, resolve, reject }));
    }""")

    def held(n=1):
        assert wait_until(page, "window.logAnswers.length === %d" % n, 5), f"{n} request(s) of the log held"
        return page.evaluate("window.logAnswers[0].path")

    def answer(ids):
        page.evaluate("(evs) => window.logAnswers.shift().resolve(evs)",
                      [{"id": i, "kind": "hello", "time": stamp(now, i)} for i in ids])

    hellos = "logState.lines.filter((l) => l.kind === 'hello').length"
    older = page.locator("#log-older")

    page.locator("#log-kind").select_option("hello")
    held()
    answer(range(201, 401))
    assert wait_until(page, hellos + " === 200", 5), "a page of 200"
    older.click()
    assert "before=201" in held(), "Older asks before the oldest"
    page.locator("#log-clear").click()
    assert "kind=" not in page.evaluate("window.logAnswers[1].path"), "Clear asks for every kind again"
    answer(range(1, 201))  # the Older answer, dropped
    answer([])  # Clear's own page
    page.wait_for_timeout(200)
    assert page.evaluate(hellos) == 0, "the Older answer after Clear is dropped"
    assert older.is_disabled(), "nothing to page after Clear"

    rf = page.evaluate("state.rooms[0].devices[0].rf_address")
    sn = page.evaluate("state.rooms[0].devices[0].serial")
    page.locator("#log-device").select_option(sn)
    assert "device=" + sn in held(), "the device chosen asked for, by its S/N"
    answer([])
    page.evaluate("selectCube(state.serial)")
    assert "device=" not in held(), "a Cube switch asks for all devices"
    assert page.evaluate("logState.device === '' && document.getElementById('log-device').value === ''"), \
        "a Cube switch takes the device chosen back to all"
    answer([])

    page.evaluate("() => { loadLog(); }")  # not awaited: the test answers it
    held()
    answer(range(1801, 3801))
    assert wait_until(page, hellos + " === 2000", 5), "a page of 2000"
    older.click()
    assert "before=1801" in held(), "Older from the oldest of 2000"
    answer(range(1601, 1801))
    assert wait_until(page, hellos + " === 2200", 5), "2200 lines"
    page.evaluate("""(rf) => state.events.dispatchEvent(new MessageEvent("config", { data: JSON.stringify({
      kind: "config", device: rf, time: new Date().toISOString() }) }))""", rf)
    assert wait_until(page, "logState.lines.some((l) => l.kind === 'config')", 5), "the live event in the log"
    assert page.evaluate(hellos) == 2200 and page.evaluate("logState.lines[0].id === 1601 && logState.cursor === '1601'"), \
        "a live event keeps the page fetched"

    page.evaluate("() => { loadLog(); }")  # not awaited: the test answers it
    held()
    answer(range(1, 2001))
    assert wait_until(page, hellos + " === 2000", 5), "a page of 2000"
    page.evaluate("""(rf) => state.events.dispatchEvent(new MessageEvent("config", { data: JSON.stringify({
      kind: "config", device: rf, time: new Date().toISOString() }) }))""", rf)
    assert wait_until(page, "logState.lines[0].id >= 2 && logState.cursor === String(logState.lines[0].id)", 5), \
        "past 2000 lines the oldest go, and Older goes on from the oldest kept"
    assert page.evaluate("document.querySelectorAll('#log-lines .log-line').length === logState.lines.filter(logLineMatches).length"), \
        "the view drops the lines that went"

    page.evaluate("() => { loadLog(); }")  # not awaited: the test answers it
    held()
    page.evaluate("window.logAnswers.shift().reject(new Error('nope'))")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-error').length === 1", 5), \
        "the error line shows with a kind chosen"
    show_view(page, "tiles")


def test_log_keeps_events_that_come_while_it_loads(page, case):
    """A live event that comes while a page of the log is on its way is
    kept after the page, and one the page holds already is not doubled.
    The page's api() holds the log's request until the test answers it,
    and the server's own lines are kept out of the log (isolate_log); only
    the kinds the test sends are counted."""
    now = time.time()
    show_view(page, "log")
    assert wait_until(page, "!logState.loading", 5), "the log loaded"
    isolate_log(page)
    page.evaluate("""() => {
      window.logAnswers = [];
      const real = api;
      window.api = (method, path, body) => path.includes("/log") && !path.includes("/log/alerts")
        ? new Promise((resolve, reject) => window.logAnswers.push({ path, resolve, reject }))
        : real(method, path, body);
    }""")
    rf = page.evaluate("state.rooms[0].devices[0].rf_address")
    page.evaluate("() => { loadLog(); }")  # not awaited: the test answers it
    assert wait_until(page, "window.logAnswers.length === 1", 5), "the log's request held"
    late = time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime(now + 5)) + "Z"
    both = time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime(now + 1)) + "Z"
    for at in (both, late):
        page.evaluate("""([rf, at]) => state.events.dispatchEvent(new MessageEvent("config", { data: JSON.stringify({
          kind: "config", device: rf, time: at }) }))""", [rf, at])
    page.evaluate("(evs) => window.logAnswers.shift().resolve(evs)",
                  [{"id": i, "kind": "hello", "time": stamp(now, i)} for i in (1, 2)] +
                  [{"id": 3, "kind": "config", "device": rf, "time": both}])
    assert wait_until(page, "!logState.loading", 5), "the page is in"
    kinds = page.evaluate("logState.lines.filter((l) => l.kind === 'hello' || l.kind === 'config').map((l) => l.kind + ' ' + l.at)")
    assert kinds == ["hello " + stamp(now, 1), "hello " + stamp(now, 2), "config " + both, "config " + late], \
        f"the page, then the live event it lacks, once each: {kinds}"
    assert page.locator("#log-lines .log-line.kind-hello, #log-lines .log-line.kind-config").count() == 4, "and the view shows them"
    show_view(page, "tiles")


def test_log_clear_sets_the_filter_back(page, case):
    """Clear is off while nothing narrows the log; after a kind and a text,
    it sets the text, the kind and the device back to all and loads the
    lines again, and is off again."""
    show_view(page, "log")
    assert wait_until(page, "!logState.loading && document.querySelectorAll('#log-lines .log-line').length > 0", 5), "the log loaded"
    clear = page.locator("#log-clear")
    assert clear.is_disabled(), "Clear is off with nothing chosen"
    before = page.locator("#log-lines .log-line").count()
    page.locator("#log-kind").select_option("connected")
    assert wait_until(page, "!logState.loading", 5), "the kind's page is in"
    page.locator("#log-filter").fill("zzz no such line")
    assert page.locator("#log-lines .log-line").count() == 0, "nothing matches the text"
    assert clear.is_enabled(), "Clear is on"
    with page.expect_response(lambda r: "/log?" in r.url) as answer:
        clear.click()
    assert "kind=" not in answer.value.url and "device=" not in answer.value.url, f"every kind and device asked for: {answer.value.url}"
    assert page.locator("#log-filter").input_value() == "" and page.locator("#log-kind").input_value() == "" \
        and page.locator("#log-device").input_value() == "", "the text, the kind and the device back to all"
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line').length >= %d" % before, 5), "the lines back"
    assert clear.is_disabled(), "Clear is off again"
    show_view(page, "tiles")


def test_log_reads_newest_first(page, case):
    """The newest line is at the top; a live event comes in on top, and a
    view scrolled down keeps the lines it shows where they are."""
    now = time.time()

    def answer(route):
        evs = [{"id": i, "kind": "hello", "time": stamp(now, i)} for i in range(1, 61)]
        route.fulfill(status=200, content_type="application/json", body=json.dumps(evs))

    pattern = re.compile(r"/api/v1/cubes/[^/]+/log\?")
    show_view(page, "log")
    page.route(pattern, answer)
    try:
        all_events(page)
        assert wait_until(page, "document.querySelectorAll('#log-lines .kind-hello').length === 60", 5), "the page is in"
    finally:
        page.unroute(pattern)
    times = page.evaluate("[...document.querySelectorAll('#log-lines .log-line')].map((l) => l.querySelector('.log-time').textContent)")
    assert times == sorted(times, reverse=True), "newest first"
    rf = page.evaluate("state.rooms[0].devices[0].rf_address")
    live = """(rf) => state.events.dispatchEvent(new MessageEvent("config", { data: JSON.stringify({
      kind: "config", device: rf, time: new Date().toISOString() }) }))"""
    isolate_log(page)
    wait_log_live(page)
    page.evaluate(live, rf)
    assert wait_until(page, "document.querySelector('#log-lines .log-line').classList.contains('kind-config')", 5), "a live event on top"
    box = "document.getElementById('log-lines')"
    page.evaluate(box + ".scrollTop = 200")
    page.evaluate(box + ".dispatchEvent(new Event('scroll'))")
    shown = page.evaluate("(() => { const b = document.getElementById('log-lines'); const r = b.getBoundingClientRect(); "
                          "return document.elementFromPoint(r.left + 40, r.top + 20).closest('.log-line').textContent; })()")
    page.evaluate(live, rf)
    assert wait_until(page, "document.querySelectorAll('#log-lines .kind-config').length === 2", 5), "the second live event"
    still = page.evaluate("(() => { const b = document.getElementById('log-lines'); const r = b.getBoundingClientRect(); "
                          "return document.elementFromPoint(r.left + 40, r.top + 20).closest('.log-line').textContent; })()")
    assert still == shown, "a view scrolled down keeps its place"
    show_view(page, "tiles")


LOG_ROUTE = re.compile(r"/api/v1/cubes/[^/]+/log\?")


def iso(t):
    return datetime.fromtimestamp(t, timezone.utc).strftime("%Y-%m-%dT%H:%M:%S") + "Z"


def test_log_range_is_the_last_20_by_default(page, case):
    """The range is the last 20 events at first: the page asks for 20 and
    no time, has no Older, and keeps the newest 20 as live events come:
    the three sent push out the three oldest. The server's own lines are
    kept out of the log from before the stubbed load on (isolate_log), so
    that the log holds the stub's lines and the test's alone."""
    now = time.time()
    asked = []

    def answer(route):
        asked.append(parse_qs(urlparse(route.request.url).query))
        evs = [{"id": i, "kind": "hello", "time": iso(now - 100 + i)} for i in range(1, 21)]
        route.fulfill(status=200, content_type="application/json", body=json.dumps(evs))

    isolate_log(page)
    page.route(LOG_ROUTE, answer)
    try:
        show_view(page, "log")
        assert wait_until(page, "document.querySelectorAll('#log-lines .kind-hello').length === 20", 5), "the 20 are in"
    finally:
        page.unroute(LOG_ROUTE)
    assert page.locator("#log-range").input_value() == "20", "the last 20 events"
    assert asked[-1].get("limit") == ["20"] and "since" not in asked[-1], f"20 asked for, no time: {asked[-1]}"
    assert page.locator("#log-older").is_hidden(), "no Older"
    wait_log_live(page)
    rf = page.evaluate("state.rooms[0].devices[0].rf_address")
    for _ in range(3):
        page.evaluate("""(rf) => state.events.dispatchEvent(new MessageEvent("config", { data: JSON.stringify({
          kind: "config", device: rf, time: new Date().toISOString() }) }))""", rf)
    assert wait_until(page, "document.querySelectorAll('#log-lines .kind-config').length === 3", 5), "the live events"
    assert page.locator("#log-lines .log-line").count() == 20, "the newest 20 kept"
    assert page.evaluate("logState.lines.length") == 20, "and no more held"
    # the stubbed lines left are the newest 17, the three oldest gone for the
    # three events sent
    kept = sorted(int(i) for i in page.evaluate(
        "[...document.querySelectorAll('#log-lines .kind-hello')].map((l) => l.dataset.id)"))
    assert kept == list(range(4, 21)), f"the three oldest gone, the newest 17 kept: {kept}"
    show_view(page, "tiles")


def test_log_range_by_time(page, case):
    """The last hour asks for the events since an hour ago and pages back
    within it; a line that falls out of the hour goes at the minute's
    check; with all events, a line of another day shows its date."""
    now = time.time()
    asked = []

    def answer(route):
        q = parse_qs(urlparse(route.request.url).query)
        asked.append(q)
        if "since" not in q:
            evs = [{"id": 1, "kind": "hello", "time": iso(now - 2 * 86400)}, {"id": 2, "kind": "hello", "time": iso(now - 10)}]
        elif "before" not in q:
            evs = [{"id": 100 + i, "kind": "hello", "time": iso(now - 3500 + i * 15)} for i in range(200)]
        else:
            evs = [{"id": 90 + i, "kind": "hello", "time": iso(now - 3550 + i * 5)} for i in range(5)]
        route.fulfill(status=200, content_type="application/json", body=json.dumps(evs))

    show_view(page, "log")
    page.route(LOG_ROUTE, answer)
    try:
        page.locator("#log-range").select_option("1h")
        assert wait_until(page, "document.querySelectorAll('#log-lines .kind-hello').length === 200", 5), "a page of the hour"
        since = datetime.fromisoformat(asked[-1]["since"][0].replace("Z", "+00:00")).timestamp()
        assert abs(since - (now - 3600)) < 120 and asked[-1].get("limit") == ["200"], f"since an hour ago: {asked[-1]}"
        older = page.locator("#log-older")
        assert older.is_visible() and older.is_enabled(), "Older offered within the hour"
        older.click()
        assert wait_until(page, "document.querySelectorAll('#log-lines .kind-hello').length === 205", 5), "the page before"
        assert "since" in asked[-1] and asked[-1].get("before") == ["100"], f"Older within the hour: {asked[-1]}"
        assert wait_until(page, "document.getElementById('log-older').disabled", 5), "a short page ends it"
        start = page.evaluate("(() => { const later = new Date(Date.now() + 10 * 60000); pruneLogRange(later); "
                              "return logRangeStart('1h', later).getTime() / 1000; })()")
        kept = page.evaluate("logState.lines.map((l) => l.time.getTime() / 1000)")
        want = len([i for i in range(200) if int(now - 3500 + i * 15) >= start])
        assert kept and min(kept) >= start and abs(len(kept) - want) <= 1, f"the lines out of the hour gone: {len(kept)}, want {want}"
        assert page.locator("#log-lines .log-line").count() == len(kept), "and from the view"
        all_events(page)
        assert wait_until(page, "document.querySelectorAll('#log-lines .kind-hello').length === 2", 5), "all events"
    finally:
        page.unroute(LOG_ROUTE)
    times = page.evaluate("[...document.querySelectorAll('#log-lines .log-time')].map((e) => e.textContent)")
    assert len(times[0]) == 8 and len(times[1]) > 8, f"today's time alone, another day's with its date: {times}"
    show_view(page, "tiles")


def log_shown(page):
    return wait_until(page, "!document.getElementById('logview').classList.contains('hidden')", 10)


def choices(page):
    """The log's range, kind and device as its choices show them."""
    return [page.locator(sel).input_value() for sel in ("#log-range", "#log-kind", "#log-device")]


def devices_loaded(page):
    return wait_until(page, "state.roomsOf === state.serial", 10)


def test_log_choices_in_the_address(page, case):
    """The range, the kind and the device chosen are in the address after
    the log's page, in place of its entry, so that back still leaves the
    log; a reload opens the log with them, asked of the server; Clear
    takes the kind and the device out; another page and back, or Rooms
    and the log again, keep the range; the last 20 again is the plain
    page."""
    show_view(page, "log")
    sn = page.evaluate("state.rooms[0].devices[0].serial")
    length = page.evaluate("history.length")
    page.locator("#log-range").select_option("1h")
    page.locator("#log-kind").select_option("state")
    page.locator("#log-device").select_option(sn)
    want = "#log?range=1h&kind=state&device=" + sn
    assert wait_until(page, "location.hash === %r" % want, 5), f"the choices in the address: {page.evaluate('location.hash')}"
    assert page.evaluate("history.length") == length, "in place of the log's entry"
    with page.expect_request(lambda r: "/log?" in r.url) as asked:
        page.reload()
    q = parse_qs(urlparse(asked.value.url).query)
    assert q.get("kind") == ["state"] and q.get("device") == [sn] and "since" in q, f"the reload asks for them: {q}"
    assert log_shown(page) and devices_loaded(page), "the log again, the Cube's devices loaded"
    assert choices(page) == ["1h", "state", sn] and page.evaluate("location.hash") == want, "the choices kept"
    page.locator("#log-clear").click()
    assert wait_until(page, "location.hash === '#log?range=1h'", 5), "Clear takes the kind and the device out, the range stays"
    show_view(page, "system")
    page.go_back()
    assert log_shown(page) and page.evaluate("location.hash") == "#log?range=1h", "back: the log"
    assert choices(page) == ["1h", "", ""], "with its range"
    show_view(page, "tiles")
    show_view(page, "log")
    assert page.evaluate("location.hash") == "#log?range=1h", "the log again: its range in the address"
    page.locator("#log-range").select_option("20")
    assert wait_until(page, "location.hash === '#log'", 5), "the last 20: the plain page"
    show_view(page, "tiles")


def test_log_from_a_link(page, case):
    """A link to the log with its choices, in any order, opens the log
    with them, asked of the server, and the address in its own order, the
    device by its S/N; an old link's radio address takes the S/N once the
    devices are loaded; another address typed in its place shows its
    choices, the others at their defaults."""
    rf = page.evaluate("state.rooms[0].devices[0].rf_address")
    sn = page.evaluate("state.rooms[0].devices[0].serial")
    page.evaluate("(h) => history.replaceState(null, '', location.pathname + h)", f"#log?device={sn.lower()}&kind=state&range=today")
    with page.expect_request(lambda r: "/log?" in r.url) as asked:
        page.reload()
    q = parse_qs(urlparse(asked.value.url).query)
    assert q.get("kind") == ["state"] and q.get("device") == [sn] and "since" in q and q.get("limit") == ["200"], \
        f"the link's choices asked of the server: {q}"
    assert log_shown(page) and devices_loaded(page), "the log, the Cube's devices loaded"
    assert choices(page) == ["today", "state", sn], "the link's choices"
    assert page.evaluate("location.hash") == f"#log?range=today&kind=state&device={sn}", "the address in its order"
    page.evaluate("(h) => history.replaceState(null, '', location.pathname + h)", f"#log?device={rf}")
    page.reload()
    assert log_shown(page) and devices_loaded(page), "the old link's log"
    assert wait_until(page, "location.hash === %r" % f"#log?device={sn}", 5), \
        f"the old link's radio address takes the S/N: {page.evaluate('location.hash')}"
    assert choices(page) == ["20", "", sn], "the device chosen by its S/N"
    page.evaluate("location.hash = '#log?range=week'")
    assert wait_until(page, "logState.range === 'week' && !logState.kind && !logState.device", 5), "the address typed"
    assert choices(page) == ["week", "", ""], "its range, every kind and device"
    show_view(page, "tiles")


def test_log_address_with_unknown_values(page, case):
    """Unknown values in the log's address fall back to their defaults and
    the address drops them; a device the Cube does not have goes once its
    devices are loaded, and the log is asked for every device."""
    asked = []
    page.on("request", lambda r: asked.append(r.url) if "/log?" in r.url else None)
    page.evaluate("history.replaceState(null, '', location.pathname + '#log?range=year&kind=nope&device=0a0fff&x=1')")
    page.reload()
    assert log_shown(page) and devices_loaded(page), "the log, the Cube's devices loaded"
    assert wait_until(page, "location.hash === '#log'", 5), f"the address drops them: {page.evaluate('location.hash')}"
    assert choices(page) == ["20", "", ""], "the last 20 events, every kind and device"
    end = time.time() + 5
    while not (asked and "device=" not in asked[-1]) and time.time() < end:
        page.wait_for_timeout(100)
    assert asked and "device=" not in asked[-1] and "kind=" not in asked[-1], f"the log asked for every device: {asked}"
    show_view(page, "tiles")


CMD_SAYS = {"en": {"open": "command ", "head": "setpoint"}, "de": {"open": "Befehl ", "head": "setpoint"}}


def test_log_command_detail(page, case, stream):
    """A line of a command opens the command under it: what was asked,
    its state, its steps and, for the owner, its audit row; the button
    says whether it is open, and closes it again. The command's lines
    come on the stream, watched from before the write, and the line taken
    is one of the write's own command, by its number: the log holds the
    server's earlier commands too, the four cases' among them."""
    show_view(page, "log")
    wait_log_live(page)
    serial = page.evaluate("state.serial")
    room = page.evaluate("state.rooms[0].id")
    mark = stream.mark()
    api_call("PUT", f"api/v1/cubes/{serial}/rooms/{room}/setpoint", {"mode": "auto"})
    ev = stream.wait("the write's command", "command", match=lambda d: bool(d and d.get("command")), mark=mark, server=True, seconds=10)
    cmd = ev["data"]["command"]
    label = page.evaluate("(id) => t('log.cmd.open', {id})", cmd)
    assert label.startswith(CMD_SAYS[case.lang]["open"]), f"the button's text: {label}"
    # the write's command's lines, each with its button; the detail opens under each
    line = page.locator("#log-lines .log-line.kind-command").filter(
        has=page.locator(".log-cmd", has_text=re.compile("^" + re.escape(label) + "$"))).first
    btn = line.locator(".log-cmd")
    assert wait_until(page, "[...document.querySelectorAll('#log-lines .log-line.kind-command .log-cmd')].some((b) => b.textContent === %s)"
                      % json.dumps(label), 10), f"a line of the write's command {cmd}"
    assert btn.get_attribute("aria-expanded") == "false", "closed at first"
    btn.click()
    detail = line.locator(".log-cmd-detail")
    assert wait_until(page, "!!document.querySelector('#log-lines .log-cmd-detail .log-cmd-head')", 10), "the command's detail"
    asked = api_call("GET", f"api/v1/commands/{cmd}")
    what = f"{asked.get('action')} {asked.get('target') or '–'}"
    head = detail.locator(".log-cmd-head").inner_text()
    assert head.startswith(what + ": ") and CMD_SAYS[case.lang]["head"] in head, f"what was asked, of command {cmd} ({what}): {head!r}"
    assert detail.locator(".log-cmd-steps li").count() >= 1, "its steps"
    assert detail.locator(".log-cmd-audit").count() == 1, "the owner sees its audit row"
    assert btn.get_attribute("aria-expanded") == "true", "the button says it is open"
    assert page.evaluate("document.documentElement.scrollWidth") <= case.width, "no horizontal scroll with the detail open"
    btn.click()
    assert wait_until(page, "!document.querySelector('#log-lines .log-cmd-detail')", 5), "closed again"
    show_view(page, "tiles")
