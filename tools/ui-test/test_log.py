# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The log: it opens, a state event with changes shows what changed,
the kind and the device chosen narrow it, and Older pages back."""
import json
import re
import time
from urllib.parse import parse_qs, urlparse

from helpers import CHANGES, named_buttons, wait_until

OLDER = {"en": "Older events", "de": "Ältere Ereignisse"}
NO_OLDER = {"en": "No older events", "de": "Keine älteren Ereignisse"}


def test_log_opens(page):
    page.locator("#view-log").click()
    assert page.locator("#logview").is_visible(), "log opens"
    named_buttons(page, "the log")


def test_log_changes(page, case):
    """A state event with changes (as the server sends after the first
    one of a device) shows what changed, not the whole state. The log is
    opened first: opening it loads the server's log. The page leaves the
    stream's first 1.5 s out of the log (the burst of state events a
    connection starts with), so the event comes after that."""
    page.locator("#view-log").click()
    page.wait_for_timeout(1600)
    page.evaluate("""() => {
      const d = JSON.parse(JSON.stringify(state.rooms[0].devices[0]));
      state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({
        kind: "state", device: d.rf_address, state: d, time: new Date().toISOString(),
        changes: { window_open: { from: false, to: true }, setpoint: { from: 18, to: 5 } } }) }));
    }""")
    line = page.locator(".log-line.kind-state").last
    assert CHANGES[case.lang] in line.inner_text(), "the log says what changed"
    page.locator("#view-tiles").click()


def test_log_filters(page, case):
    """The kind and the device chosen narrow the log the server answers;
    back to all, the lines of before; the fake Cube's log is one page, so
    Older says there is nothing older."""
    page.locator("#view-log").click()
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
    page.locator("#log-device").select_option("")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line').length >= %d" % before, 5), "all again"
    older = page.locator("#log-older")
    assert older.is_disabled() and older.inner_text() == NO_OLDER[case.lang], "one page: nothing older"
    named_buttons(page, "the log with its choices")
    page.locator("#view-tiles").click()


def test_log_older(page, case):
    """Older puts the page before the oldest line above the lines, the
    view staying where it was, until a short page ends the paging; the
    server's answers are stubbed: a full page of 200, then two events."""
    now = time.time()

    def stamp(i):
        return time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime(now - 3600 + i)) + "Z"

    def answer(route):
        q = parse_qs(urlparse(route.request.url).query)
        before = q.get("before", [""])[0]
        if not before:
            evs = [{"id": i, "kind": "hello", "time": stamp(i)} for i in range(201, 401)]
        elif before == "201":
            evs = [{"id": i, "kind": "hello", "time": stamp(i)} for i in (1, 2)]
        else:
            evs = []
        route.fulfill(status=200, content_type="application/json", body=json.dumps(evs))

    pattern = re.compile(r"/api/v1/cubes/[^/]+/log\?")
    page.route(pattern, answer)
    try:
        page.locator("#view-log").click()
        assert wait_until(page, "document.querySelectorAll('#log-lines .kind-hello').length === 200", 5), "a page of 200"
        older = page.locator("#log-older")
        assert older.is_enabled() and older.inner_text() == OLDER[case.lang], "Older offered"
        page.evaluate("document.getElementById('log-lines').scrollTop = 0")
        older.click()
        assert wait_until(page, "document.querySelectorAll('#log-lines .kind-hello').length === 202", 5), "two older events above"
        assert page.evaluate("logState.lines[0].id === 1 && logState.lines[1].id === 2 && logState.lines[2].id === 201"), "the oldest first"
        assert page.evaluate("document.getElementById('log-lines').scrollTop") > 0, "the view stays where it was"
        assert wait_until(page, "document.getElementById('log-older').disabled", 5), "a short page ends the paging"
        assert older.inner_text() == NO_OLDER[case.lang], "nothing older"
    finally:
        page.unroute(pattern)
    page.locator("#view-tiles").click()
