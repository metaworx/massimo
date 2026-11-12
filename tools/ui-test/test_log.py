# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The log: it opens, newest first, a state event with changes shows what
changed, the kind and the device chosen narrow it, Clear sets them back,
the range limits it by count or by time, the address holds the range,
the kind and the device, Older pages back, and an answer that comes late
or fails leaves the log as it should be."""
import json
import re
import time
from datetime import datetime, timezone
from urllib.parse import parse_qs, urlparse

from conftest import api_call
from helpers import CHANGES, named_buttons, show_view, wait_until

OLDER = {"en": "Older events", "de": "Ältere Ereignisse"}
NO_OLDER = {"en": "No older events", "de": "Keine älteren Ereignisse"}


def test_log_opens(page):
    show_view(page, "log")
    assert page.locator("#logview").is_visible(), "log opens"
    named_buttons(page, "the log")


def test_log_changes(page, case):
    """A state event with changes (as the server sends after the first
    one of a device) shows what changed, not the whole state. The log is
    opened first: opening it loads the server's log. The page leaves the
    stream's first 1.5 s out of the log (the burst of state events a
    connection starts with), so the event comes after that."""
    show_view(page, "log")
    page.wait_for_timeout(1600)
    page.evaluate("""() => {
      const d = JSON.parse(JSON.stringify(state.rooms[0].devices[0]));
      state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({
        kind: "state", device: d.rf_address, state: d, time: new Date().toISOString(),
        changes: { window_open: { from: false, to: true }, setpoint: { from: 18, to: 5 } } }) }));
    }""")
    line = page.locator(".log-line.kind-state").first
    assert CHANGES[case.lang] in line.inner_text(), "the log says what changed"
    show_view(page, "tiles")


def test_log_names_devices_by_serial(page, case):
    """A line names a device without a name by its S/N, the radio address
    in expert mode only; the Cube's own configuration is the Cube's; a
    removed device is named by its type, S/N and name as it was."""
    show_view(page, "log")
    page.wait_for_timeout(1600)
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


SAYS = {
    "en": {"down": "down for 5 min", "missed": "(while disconnected)", "renamed": "renamed", "comfort": "comfort 21.0 → 21.5"},
    "de": {"down": "5 min getrennt", "missed": "(während getrennt)", "renamed": "umbenannt", "comfort": "Komfort 21,0 → 21,5"},
}


def test_log_says_what_changed(page, case):
    """A reconnect is one line with its summary, what changed meanwhile
    follows it with a note, a configuration's line says its differences,
    and a debug line shows in expert mode only."""
    show_view(page, "log")
    page.wait_for_timeout(1600)
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


def test_log_server_lines(page, case):
    """The server's own lines say what they are, and who asked for one."""
    show_view(page, "log")
    page.wait_for_timeout(1600)
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


def test_log_change_line(page, case):
    """A change made through the API says what was changed, from and to,
    and by whom."""
    show_view(page, "log")
    page.wait_for_timeout(1600)
    page.evaluate("""() => state.events.dispatchEvent(new MessageEvent("change", { data: JSON.stringify({
      kind: "change", source: "config", level: "info", time: new Date().toISOString(),
      summary: { action: "cube.update", target: "cube:KEQ0000000", changes: { name: { from: "", to: "Upstairs" } } },
      by: { name: "anna", display_name: "Anna" } }) }))""")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-change').length > 0", 5), "the change line"
    text = page.locator("#log-lines .log-line.kind-change").first.locator(".log-text").inner_text()
    assert text == CHANGE_SAYS[case.lang], f"the change: {text}"
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
            == page.evaluate("[logState.lines[1], logState.lines[0]].map((l) => fmtLogTime(l.time))"), "drawn at the bottom"
        assert page.evaluate(box + ".scrollTop") == top, "the view stays where it was"
        assert wait_until(page, "document.getElementById('log-older').disabled", 5), "a short page ends the paging"
        assert older.inner_text() == NO_OLDER[case.lang], "nothing older"
    finally:
        page.unroute(pattern)
    show_view(page, "tiles")


def test_log_answers(page, case):
    """What comes back late or fails leaves the log as it should be: an
    Older answer after Clear is dropped, a Cube switch takes the device
    chosen back to all, a page fetched with Older stays when a live
    event comes past 2000 lines, and the error line shows with a kind
    chosen. The page's api() holds the log's requests, which the test
    answers in its own time."""
    now = time.time()
    show_view(page, "log")
    all_events(page)
    page.evaluate("""() => {
      window.logAnswers = [];
      const real = api;
      window.api = (method, path, body) => path.includes("/log")
        ? new Promise((resolve, reject) => window.logAnswers.push({ path, resolve, reject }))
        : real(method, path, body);
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
    The page's api() holds the log's request until the test answers it."""
    now = time.time()
    show_view(page, "log")
    assert wait_until(page, "!logState.loading", 5), "the log loaded"
    page.evaluate("""() => {
      window.logAnswers = [];
      const real = api;
      window.api = (method, path, body) => path.includes("/log")
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
    kinds = page.evaluate("logState.lines.map((l) => l.kind + ' ' + l.at)")
    assert kinds == ["hello " + stamp(now, 1), "hello " + stamp(now, 2), "config " + both, "config " + late], \
        f"the page, then the live event it lacks, once each: {kinds}"
    assert page.locator("#log-lines .log-line").count() == 4, "and the view shows them"
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
    page.wait_for_timeout(1600)  # after the stream's first burst, which the log leaves out
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
    no time, has no Older, and keeps the newest 20 as live events come."""
    now = time.time()
    asked = []

    def answer(route):
        asked.append(parse_qs(urlparse(route.request.url).query))
        evs = [{"id": i, "kind": "hello", "time": iso(now - 100 + i)} for i in range(1, 21)]
        route.fulfill(status=200, content_type="application/json", body=json.dumps(evs))

    page.route(LOG_ROUTE, answer)
    try:
        show_view(page, "log")
        assert wait_until(page, "document.querySelectorAll('#log-lines .kind-hello').length === 20", 5), "the 20 are in"
    finally:
        page.unroute(LOG_ROUTE)
    assert page.locator("#log-range").input_value() == "20", "the last 20 events"
    assert asked[-1].get("limit") == ["20"] and "since" not in asked[-1], f"20 asked for, no time: {asked[-1]}"
    assert page.locator("#log-older").is_hidden(), "no Older"
    page.wait_for_timeout(1600)  # after the stream's first burst, which the log leaves out
    rf = page.evaluate("state.rooms[0].devices[0].rf_address")
    for _ in range(3):
        page.evaluate("""(rf) => state.events.dispatchEvent(new MessageEvent("config", { data: JSON.stringify({
          kind: "config", device: rf, time: new Date().toISOString() }) }))""", rf)
    assert wait_until(page, "document.querySelectorAll('#log-lines .kind-config').length === 3", 5), "the live events"
    assert page.locator("#log-lines .log-line").count() == 20, "the newest 20 kept"
    assert page.evaluate("logState.lines.length") == 20 and page.locator("#log-lines .kind-hello").count() == 17, \
        "the three oldest gone"
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


def test_log_command_detail(page, case):
    """A line of a command opens the command under it: what was asked,
    its state, its steps and, for the owner, its audit row; the button
    says whether it is open, and closes it again."""
    show_view(page, "log")
    page.wait_for_timeout(1600)
    serial = page.evaluate("state.serial")
    room = page.evaluate("state.rooms[0].id")
    api_call("PUT", f"api/v1/cubes/{serial}/rooms/{room}/setpoint", {"mode": "auto"})
    btn = page.locator("#log-lines .log-line.kind-command .log-cmd").first
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-command .log-cmd').length > 0", 10), "a line of the command"
    assert btn.inner_text().startswith(CMD_SAYS[case.lang]["open"]), f"the button: {btn.inner_text()}"
    assert btn.get_attribute("aria-expanded") == "false", "closed at first"
    btn.click()
    assert wait_until(page, "!!document.querySelector('#log-lines .log-cmd-detail .log-cmd-head')", 10), "the command's detail"
    detail = page.locator("#log-lines .log-cmd-detail").first
    assert CMD_SAYS[case.lang]["head"] in detail.locator(".log-cmd-head").inner_text(), "what was asked"
    assert detail.locator(".log-cmd-steps li").count() >= 1, "its steps"
    assert detail.locator(".log-cmd-audit").count() == 1, "the owner sees its audit row"
    btn = page.locator("#log-lines .log-line.kind-command .log-cmd").first
    assert btn.get_attribute("aria-expanded") == "true", "the button says it is open"
    assert page.evaluate("document.documentElement.scrollWidth") <= case.width, "no horizontal scroll with the detail open"
    btn.click()
    assert wait_until(page, "!document.querySelector('#log-lines .log-cmd-detail')", 5), "closed again"
    show_view(page, "tiles")
