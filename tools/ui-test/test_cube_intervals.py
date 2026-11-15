# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Maintenance, the Cube intervals (AP CubePoll v1.0,
Block 2), as the owner: the test server runs without -poll and -refresh,
so those two can be chosen, and with -discover-interval 0, so the search
is read-only and names the flag. A choice is stored and waits for the
next restart, counted in the card's head; the default is stored as none.
Each case puts the settings back."""
from conftest import api_call
from helpers import named_buttons, scroll_width, show_view, wait_until


def setting(key):
    return (api_call("GET", "api/v1/settings") or {}).get(key) or {}


def badges(page):
    return page.locator("#sys-maint .section-badges").inner_text()


def test_poll_interval_choice(page, case):
    """10 s at first, the line saying what a poll does and this run's
    value; 30 s is stored, waits for the restart and is counted; 10 s
    again clears the store's value."""
    api_call("PUT", "api/v1/settings", {"poll_interval": None})
    try:
        show_view(page, "maint")
        sel = page.locator("#interval-poll")
        assert wait_until(page, "document.querySelector('#interval-poll-text').textContent.length > 0", 5), "the line is filled"
        assert sel.is_enabled() and sel.input_value() == "10", "10 s, and the choice open"
        before, head = page.locator("#interval-poll-text").inner_text(), badges(page)
        assert before.startswith(page.evaluate("t('maint.pollHint')")), "the line says what a poll does"
        assert not before.endswith(".."), "one full stop after a unit that has one"
        sel.select_option("30")
        assert wait_until(page, "!document.querySelector('#interval-poll-msg').classList.contains('hidden')", 5), "the change says how it went"
        assert "ok" in (page.locator("#interval-poll-msg").get_attribute("class") or ""), "saved"
        stored = setting("poll_interval")
        assert stored.get("value") == "30s" and stored.get("set") is True, "30 s stored"
        assert stored.get("in_effect") == "10s", "in effect only at the next start"
        assert page.locator("#interval-poll-text").inner_text() != before, "the line says the change waits for the restart"
        assert wait_until(page, "document.querySelector('#sys-maint .section-badges').textContent.length > 0", 5), "counted in the card's head"
        assert badges(page) != head, "the head counts one more"
        sel.select_option("10")
        assert wait_until(page, "!document.querySelector('#interval-poll').disabled", 5)
        assert wait_until(page, "document.querySelector('#interval-poll-text').textContent === %r" % before, 5), "the line as at first"
        assert setting("poll_interval").get("set") is False, "the default stored as none"
        assert badges(page) == head, "the head as at first"
        named_buttons(page, "Maintenance")
        assert scroll_width(page) <= case.width, "no horizontal scroll on Maintenance"
    finally:
        api_call("PUT", "api/v1/settings", {"poll_interval": None})
        show_view(page, "tiles")


def test_interval_outside_the_steps(page, case):
    """A value set through the API that is none of the steps is listed
    and chosen; Off is a step of the refresh."""
    api_call("PUT", "api/v1/settings", {"poll_interval": "45s", "refresh_interval": "0"})
    try:
        show_view(page, "maint")
        assert wait_until(page, "document.querySelector('#interval-poll').value === '45'", 5), "45 s listed and chosen"
        assert wait_until(page, "document.querySelector('#interval-refresh').value === '0'", 5), "Off chosen"
        assert page.locator("#interval-refresh").is_enabled(), "the refresh open without -refresh"
    finally:
        api_call("PUT", "api/v1/settings", {"poll_interval": None, "refresh_interval": None})
        show_view(page, "tiles")


def test_interval_fixed_by_flag(page, case):
    """-discover-interval decides the search for this run: read-only, the
    line naming the flag and the run's value, Off."""
    show_view(page, "maint")
    assert wait_until(page, "document.querySelector('#interval-discover').disabled", 5), "read-only where the flag decides"
    text = page.locator("#interval-discover-text").inner_text()
    assert "-discover-interval" in text, "the line names the flag"
    assert setting("discover_interval").get("fixed_by") == "-discover-interval"
    show_view(page, "tiles")


def test_duty_wait_choice(page, case):
    """The duty cycle's hold (AP LegacyLogs v1.0, Block 9): 10 min at
    first, open, the line saying what it does; 30 min is stored and
    waits for the restart; 10 min again clears the store's value."""
    api_call("PUT", "api/v1/settings", {"duty_wait": None})
    try:
        show_view(page, "maint")
        sel = page.locator("#interval-duty")
        assert wait_until(page, "document.querySelector('#interval-duty-text').textContent.length > 0", 5), "the line is filled"
        assert sel.is_enabled() and sel.input_value() == "600", "10 min, and the choice open"
        assert page.locator("#interval-duty-text").inner_text().startswith(page.evaluate("t('maint.dutyWaitHint')")), \
            "the line says what the hold does"
        sel.select_option("1800")
        assert wait_until(page, "!document.querySelector('#interval-duty-msg').classList.contains('hidden')", 5), "the change says how it went"
        stored = setting("duty_wait")
        assert stored.get("value") == "30m" and stored.get("set") is True, "30 min stored"
        assert stored.get("in_effect") == "10m", "in effect only at the next start"
        sel.select_option("600")
        assert wait_until(page, "!document.querySelector('#interval-duty').disabled", 5)
        assert setting("duty_wait").get("set") is False, "the default stored as none"
    finally:
        api_call("PUT", "api/v1/settings", {"duty_wait": None})
        show_view(page, "tiles")
