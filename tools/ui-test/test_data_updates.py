# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""What each data source reads, page by page (AP UiParts, Block 15): a
part that draws from the page's data follows it as a signal, and an
effect of a part drawn again could read again at each write. On every
page, shown and come to rest with the poll held, each data source is
applied once with the data as they were, and the requests under /api/v1/
it causes, of every method, path by path, are those of TABLE, which was
taken on the page before the data became signals (the tree of AP UiParts'
Block 14, an export of it, 2026-11-21: the same in 1100-en and 390-de).

A forward guard: the page before passes it, from which TABLE was taken.
A row that a decision changes says so beside it. What it does not cover:
the test's server has one Cube, so a change of Cube (selectCube, which
opens the stream anew) is no source of it, nor is a sign-out's
clearCubeData, which ends the data, nor the clock of an open window's
warning; each page is shown as it opens, its panels and folds closed,
so an effect inside an open panel is the regions' own tests'."""
from collections import Counter

import pytest

from helpers import STATE_EVENTS_JS, show_view, wait_until

pytestmark = pytest.mark.cases("1100-en", "390-de")

# The pages, each shown once, Rooms last.
PAGES = ("system", "maint", "info", "me", "cube", "log", "tiles")

# A Cube found on the network, not served, as a cube_found event brings it.
FOUND = '{ serial: "KEQ0000990", address: "192.0.2.99", configured: false }'

# The stream's events that make the page read, each as the server sends it.
STREAM_JS = """(() => {
  const at = new Date().toISOString();
  const send = (kind, data) => state.events.dispatchEvent(new MessageEvent(kind, { data: JSON.stringify({ kind, time: at, ...data }) }));
  send("hello", { source: "cube", serial: state.serial });
  send("metadata", { source: "cube", serial: state.serial });
  send("pairing_started", { source: "cube", serial: state.serial });
  send("cube_found", { source: "system", cube: { serial: "KEQ0000991", address: "192.0.2.98", configured: false } });
  send("change", { source: "system", summary: { action: "settings.update", set: { announce: null } } });
})()"""

# The data sources, by their name in TABLE: the poll, the loads of the
# Cubes and of the rooms, the whole reload, a state event per device, the
# log's sign, the log's poll under All Cubes, the ages' minute, who is in
# read again, the settings applied again (as the user's arrive from the
# server), the language applied again, a Cube's daylight-saving rules
# read, a Cube found on the network, and the stream's events that make
# the page read (hello, metadata, a pairing's start, a Cube found, a
# change of the settings), each dispatched on the page's stream.
SOURCES = {
    "poll(false)": "poll(false)",
    "loadCubes()": "loadCubes()",
    "loadRooms()": "loadRooms()",
    "refresh()": "refresh()",
    "state events": "(" + STATE_EVENTS_JS + ")()",
    "loadLogAlerts()": "loadLogAlerts()",
    "pollLog()": "pollLog()",
    "refreshAges()": "refreshAges()",
    "loadMe()": "loadMe()",
    "applyStoredSettings()": "applyStoredSettings()",
    "applyLanguage()": "applyLanguage()",
    "loadTimeZone()": "loadTimeZone(state.serial)",
    "a Cube found": "addFound(" + FOUND + ")",
    "the stream's events": STREAM_JS,
}

# The page's own requests counted while they run: fetch wrapped once.
INFLIGHT = """() => { if (window.__dataInflight !== undefined) return; window.__dataInflight = 0; const f = window.fetch;
  window.fetch = function (...a) { window.__dataInflight++; return f.apply(this, a).finally(() => { window.__dataInflight--; }); }; }"""
# Settled: no request of the page's own running for 300 ms, within 6 s.
SETTLED = """async () => { for (let quiet = 0, n = 0; n < 120; n++) { await new Promise((d) => setTimeout(d, 50));
  quiet = window.__dataInflight ? 0 : quiet + 1; if (quiet >= 6) return true; } return false; }"""

# The poll's timer cleared, before each source and after it: the settings
# applied again start it anew (applySettings), and a poll would read in a
# source's place.
HOLD = "clearInterval(state.pollTimer)"

# What each source read, taken as the docstring says: the same on every
# page but for the stream's events on Maintenance, which reads the
# settings again for a change of them (a page shown reads at its showing,
# which test_shell_updates.py counts, and no part read again for a
# source).
STREAM_READ = {"cubes": 1, "cubes/KEQ0000100/devices": 1, "cubes/KEQ0000100/pairing": 1, "cubes/KEQ0000100/rooms": 1}
READ = {
    "poll(false)": {"cubes": 1, "log/alerts": 1},
    "loadCubes()": {"cubes": 1},
    "loadRooms()": {"cubes/KEQ0000100/devices": 1, "cubes/KEQ0000100/rooms": 1},
    "refresh()": {"cubes": 1, "cubes/KEQ0000100/devices": 1, "cubes/KEQ0000100/rooms": 1, "log/alerts": 1},
    "state events": {},
    "loadLogAlerts()": {"log/alerts": 1},
    "pollLog()": {},
    "refreshAges()": {},
    "loadMe()": {"me": 1},
    "applyStoredSettings()": {},
    "applyLanguage()": {},
    "loadTimeZone()": {"cubes/KEQ0000100": 1},
    "a Cube found": {},
    "the stream's events": STREAM_READ,
}
TABLE = {f"{view} {name}": READ[name] for view in PAGES for name in SOURCES}
TABLE["maint the stream's events"] = {**STREAM_READ, "settings": 1}


def test_each_source_reads_what_it_read(page, case):
    """Every page shown in turn, the poll held, and on each every data
    source applied once: what it reads under /api/v1/ is TABLE's, path
    for path, so that no part reads again because the data it draws from
    were written."""
    page.evaluate(INFLIGHT)
    got, cur = {}, []

    def note(r):
        if "/api/v1/" in r.url:
            path = r.url.split("/api/v1/", 1)[1].split("?")[0]
            cur.append(path if r.method == "GET" else r.method + " " + path)

    page.on("request", note)
    try:
        for view in PAGES:
            page.evaluate(HOLD)
            show_view(page, view)
            # a setting sent to the server a second after it changed (login.js prefsSync) is the page's, not a source's
            assert wait_until(page, "!prefsSync.timer", 5), "no setting waits to be sent"
            assert page.evaluate(SETTLED), f"{view} comes to rest"
            for name, expr in SOURCES.items():
                page.evaluate(HOLD)
                cur.clear()
                page.evaluate("async () => { await (%s); %s; }" % (expr, HOLD))
                assert page.evaluate(SETTLED), f"{view}, {name}: comes to rest"
                got[f"{view} {name}"] = dict(sorted(Counter(cur).items()))
    finally:
        page.remove_listener("request", note)
        if not page.is_closed():
            page.evaluate("startPolling()")
    wrong = {k: (v, TABLE.get(k)) for k, v in got.items() if TABLE.get(k) != v}
    assert not wrong, "read, against TABLE:\n" + "\n".join(f"{k}: {v} (TABLE {w})" for k, (v, w) in wrong.items()) + \
        "\nthe whole table read:\n" + repr(got)
