# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The log: it opens, and a state event with changes shows what
changed."""
from helpers import CHANGES, named_buttons


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
