# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The log: it opens, newest first, a state event with changes shows what
changed, the kind and the device chosen narrow it, Clear sets them back,
the range limits it by count or by time, Older pages back, and an answer
that comes late or fails leaves the log as it should be."""
import json
import re
import time
from datetime import datetime, timezone
from urllib.parse import parse_qs, urlparse

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
    page.locator("#log-device").select_option(rf)
    assert wait_until(page, "logState.device === %r && document.querySelectorAll('#log-lines .log-line').length > 0 && "
                            "[...document.querySelectorAll('#log-lines .log-line')].every((l) => l.querySelector('.log-rf') && "
                            "l.querySelector('.log-rf').textContent === %r)" % (rf, rf), 5), "only the device's events"
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
    page.locator("#log-device").select_option(rf)
    assert "device=" + rf in held(), "the device chosen asked for"
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
