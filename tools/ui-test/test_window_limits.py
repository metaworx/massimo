# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Maintenance → Limits, the window limits (AP ServerLimits v1.0,
Block 1): a card of its own after Maintenance, for a holder of settings,
whose two rows choose how long a window may stay open before its sign
warns and before it alerts, each saved at once, its default the first
choice and sent as null (D3-A); the steps the server would refuse are
off, equal allowed (D4-A); a refusal shows on its row in the page's
language, by its code; a change, from this page or another, reads the
rooms again, so that an open window's sign follows at once (D5-A); a
role holding settings alone opens the page (D6-A). The window contact
of the fake Cube's greeting is open from the set's start; each test
puts the limits back to their defaults."""
import json
from datetime import datetime, timedelta

import pytest

from conftest import CUBE, api_call
from helpers import (cleanup, css_hidden, frames, hold, named_buttons, safe_route, scroll_width, settled, show_view, stub_me, taken, undo,
                     wait_held, wait_until)
from test_maint_updates import settings_change
from test_rooms import head_shows

WARN, ALERT = "#limit-window-warn", "#limit-window-alert"
# The steps of both rows in seconds: 1 to 45 min, 1 to 24 h (the plan's list).
STEPS = [60, 120, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 14400, 28800, 43200, 86400]
DEFAULTS = {"window_warn_after": None, "window_alert_after": None}
# A row's options as the page has them: value, text and whether it is off.
OPTIONS = "(s) => [...document.querySelector(s).options].map((o) => [o.value, o.textContent, o.disabled])"
# The steps' texts as the part writes them (intervalLabel of Maintenance.tsx).
LABELS = """(steps) => steps.map((s) => s % 86400 === 0 ? fmtUnit(s / 86400, 'day')
  : s % 3600 === 0 ? fmtUnit(s / 3600, 'hour') : fmtUnit(s / 60, 'minute'))"""
# The refusal's params of a PUT of the alert at 15 min under a warning at 30 min: the two durations.
REFUSED = {"alert": "15m", "warning": "30m"}
# The settings' route, which the page reads and writes.
SETTINGS = "**/api/v1/settings"
# The lines under the two rows and under the announcement, in that order.
MESSAGES = "() => ['#limit-window-warn-msg', '#limit-window-alert-msg', '#announce-msg'].map((s) => document.querySelector(s).textContent)"


def limit(key):
    """A limit as GET /api/v1/settings gives it."""
    return (api_call("GET", "api/v1/settings") or {}).get(key) or {}


def values_are(page, warn, alert):
    """Waits until the two rows have warn and alert chosen."""
    return wait_until(page, "document.querySelector(%s).value === %s && document.querySelector(%s).value === %s"
                      % (json.dumps(WARN), json.dumps(warn), json.dumps(ALERT), json.dumps(alert)), 5)


def offs(page, sel):
    """Each option's value of a row, with whether it is off."""
    return {v: off for v, _, off in page.evaluate(OPTIONS, sel)}


def reset():
    """A cleanup's step: both limits back at their defaults, a failure said (undo)."""
    undo("the window limits at their defaults", lambda: api_call("PUT", "api/v1/settings", DEFAULTS), absent=())


def put_taken(page):
    """taken for this page's PUT of the settings: a block whose end waits until the page has read its answer."""
    return taken(page, lambda r: r.request.method == "PUT" and r.url.endswith("/api/v1/settings"))


def open_contact():
    """The open window contact as the server lists it now, with its window's times."""
    for room in api_call("GET", f"api/v1/cubes/{CUBE}/rooms?devices=all") or []:
        for d in room.get("devices") or []:
            if d.get("type") == "shutter_contact" and (d.get("state") or {}).get("window_open") and d.get("window"):
                return d
    return None


def test_the_rows_show_the_limits_in_effect(page, case):
    """The card after Maintenance, titled Limits with its help: both rows at
    their defaults, Default first and chosen, named with its value, 5 and
    10 min, the steps 1 min to 24 h after it, the line under each saying
    what it does and describing its select; a warning set through the API
    that is no step, 7 min, listed as itself between 5 and 10 min and
    chosen, without a reload."""
    api_call("PUT", "api/v1/settings", DEFAULTS)
    try:
        show_view(page, "maint")
        assert page.evaluate("document.getElementById('sys-maint').nextElementSibling.id") == "sys-limits", "after Maintenance"
        assert page.locator("#sys-limits .section-title").inner_text() == page.evaluate("t('limits.title')"), "titled Limits"
        assert page.locator("#sys-limits .section-head .help-btn").count() == 1, "with its help"
        assert values_are(page, "default", "default"), "both at Default"
        labels = page.evaluate(LABELS, STEPS)
        for sel, title, hint, minutes in ((WARN, "limits.windowWarn", "limits.windowWarnHint", 5), (ALERT, "limits.windowAlert", "limits.windowAlertHint", 10)):
            first = page.evaluate("(n) => t('limits.default', {value: fmtUnit(n, 'minute')})", minutes)
            got = page.evaluate(OPTIONS, sel)
            assert [o[:2] for o in got] == [["default", first]] + [[str(s), lab] for s, lab in zip(STEPS, labels)], f"{sel}: {got}"
            assert page.locator(f"label[for='{sel[1:]}']").inner_text() == page.evaluate("(k) => t(k)", title), f"{sel}'s title"
            assert page.locator(f"{sel}-text").inner_text() == page.evaluate("(k) => t(k)", hint), f"{sel}'s line"
            assert page.locator(sel).get_attribute("aria-describedby") == sel[1:] + "-text", f"{sel} described by its line"
        api_call("PUT", "api/v1/settings", {"window_warn_after": "7m"})
        assert values_are(page, "420", "default"), "7 min chosen, without a reload"
        values = [o[0] for o in page.evaluate(OPTIONS, WARN)]
        assert values[values.index("300") + 1] == "420" and values[values.index("420") + 1] == "600", f"7 min between 5 and 10: {values}"
        assert page.evaluate(OPTIONS, WARN)[values.index("420")][1] == page.evaluate("fmtUnit(7, 'minute')"), "named as itself"
        named_buttons(page, "Maintenance with the limits")
        assert scroll_width(page) <= case.width, "no horizontal scroll with the limits"
    finally:
        reset()
    show_view(page, "tiles")


def test_a_step_is_sent_as_chosen_and_default_as_null(page, case):
    """3 min chosen for the warning sends exactly that, held at its route,
    the choice on with it and the focus on it while it saves; let go, the
    row says it was saved and the server holds 3 min, applied at once.
    Default sends null, and the server holds none."""
    api_call("PUT", "api/v1/settings", DEFAULTS)
    held, sent = [], []
    try:
        show_view(page, "maint")
        assert values_are(page, "default", "default"), "both at Default"
        hold(page, SETTINGS, "PUT", held, sent)
        sel = page.locator(WARN)
        sel.focus()
        sel.select_option("180")
        assert wait_held(page, held), "the PUT sent"
        assert sent == [{"window_warn_after": "3m"}], f"the step as a duration: {sent}"
        frames(page)
        assert sel.is_enabled() and sel.input_value() == "180", "on, with 3 min, while it saves"
        assert page.evaluate("document.activeElement.id") == WARN[1:], "the focus on it"
        with put_taken(page):
            held[0].continue_()
        said = page.evaluate("t('limits.saved')")
        assert wait_until(page, "document.querySelector('%s-msg').textContent === %s" % (WARN, json.dumps(said)), 5), "saved"
        assert "ok" in (page.locator(WARN + "-msg").get_attribute("class") or ""), "in green"
        frames(page)  # the save ended: the choice takes the next one
        stored = limit("window_warn_after")
        assert stored.get("value") == "3m" and stored.get("set") is True and stored.get("in_effect") == "3m", f"3 min, at once: {stored}"
        sel.select_option("default")
        assert wait_held(page, held, n=2), "the second PUT sent"
        assert sent[1] == {"window_warn_after": None}, f"Default as null: {sent}"
        with put_taken(page):
            held[1].continue_()
        assert wait_until(page, "document.querySelector('%s').value === 'default' && document.querySelector('%s-msg').textContent === %s"
                          % (WARN, WARN, json.dumps(said)), 5), "Default, saved"
        assert limit("window_warn_after").get("set") is False, "the server holds none"
    finally:
        cleanup(lambda: page.unroute(SETTINGS), reset)
    show_view(page, "tiles")


def test_the_steps_the_server_refuses_are_off(page, case):
    """The warning at 30 min, the alert at 45 min: in the warning's row each
    step above 45 min is off, 45 min itself open, and Default, 5 min,
    open; in the alert's each step under 30 min is off, 30 min itself
    open, and Default, 10 min, off. Both at Default again, from another
    browser: the warning's steps above 10 min off, the alert's under
    5 min."""
    api_call("PUT", "api/v1/settings", {"window_warn_after": "30m", "window_alert_after": "45m"})
    try:
        show_view(page, "maint")
        assert values_are(page, "1800", "2700"), "30 and 45 min chosen"
        assert offs(page, WARN) == {"default": False, **{str(s): s > 2700 for s in STEPS}}, f"the warning's: {offs(page, WARN)}"
        assert offs(page, ALERT) == {"default": True, **{str(s): s < 1800 for s in STEPS}}, f"the alert's: {offs(page, ALERT)}"
        api_call("PUT", "api/v1/settings", DEFAULTS)
        assert values_are(page, "default", "default"), "both at Default, without a reload"
        assert offs(page, WARN) == {"default": False, **{str(s): s > 600 for s in STEPS}}, f"the warning's at Default: {offs(page, WARN)}"
        assert offs(page, ALERT) == {"default": False, **{str(s): s < 300 for s in STEPS}}, f"the alert's at Default: {offs(page, ALERT)}"
    finally:
        reset()
    show_view(page, "tiles")


def test_a_refusal_shows_on_its_row_by_its_code(page, case, watch):
    """The warning set to 30 min and the alert to 45 min from another
    browser while this page's reads of the settings are held, so that it
    still holds the defaults: 15 min, which it offers for the alert, is
    refused by the server with the code setting_alert_under_warning, and
    the alert's row says so in the page's language, its choice back at
    Default; the read that follows the refusal shows what the server
    holds, the alert's steps under 30 min off."""
    watch.expected.append(("/api/v1/settings", 400))
    api_call("PUT", "api/v1/settings", DEFAULTS)
    held, sent = [], []
    try:
        show_view(page, "maint")
        assert values_are(page, "default", "default"), "both at Default"
        hold(page, SETTINGS, "GET", held, sent)
        api_call("PUT", "api/v1/settings", {"window_warn_after": "30m", "window_alert_after": "45m"})
        assert not offs(page, ALERT)["900"], "15 min offered: the page holds the defaults"
        with put_taken(page) as answer:
            page.locator(ALERT).select_option("900")
        body = answer.value.json()
        assert answer.value.status == 400 and body.get("code") == "setting_alert_under_warning" and body.get("params") == REFUSED, \
            f"refused by its code: {answer.value.status} {body}"
        want = page.evaluate("(p) => t('limits.failed', {error: t('err.setting_alert_under_warning', p)})", REFUSED)
        assert wait_until(page, "document.querySelector('%s-msg').textContent === %s" % (ALERT, json.dumps(want)), 5), \
            f"the row says it in the page's language: {page.locator(ALERT + '-msg').inner_text()!r}"
        assert "bad" in (page.locator(ALERT + "-msg").get_attribute("class") or ""), "in red"
        assert wait_until(page, "document.querySelector(%s).value === 'default'" % json.dumps(ALERT), 5), "the choice back at Default"
        # the change event's read and the one after the refusal, each held
        assert wait_held(page, held, n=2), f"the settings read again after the refusal: {len(held)} held"
        for r in held:
            r.continue_()
        page.unroute(SETTINGS)
        assert values_are(page, "1800", "2700"), "what the server holds"
        assert offs(page, ALERT)["900"] and not offs(page, ALERT)["1800"], "15 min off now, 30 min open"
    finally:
        cleanup(lambda: page.unroute(SETTINGS), reset)
    show_view(page, "tiles")


def test_a_failed_read_says_so_on_the_rows(page, case, watch, stream):
    """The server's settings refused once the card is read (503, its text
    stubbed): 3 min chosen for the warning is not saved, and its row says
    why; the read after it, refused too, says the server could not be
    asked on the alert's row and under the announcement, and leaves the
    warning's own line. Maintenance shown again, its read refused: both
    rows say it. The server answering again, a change from another
    browser reads the settings, and that read takes back all three
    lines."""
    watch.expected.append(("/api/v1/settings", 503))
    api_call("PUT", "api/v1/settings", DEFAULTS)
    down = {"error": "the store is closed"}
    try:
        show_view(page, "maint")
        assert values_are(page, "default", "default"), "both at Default"
        safe_route(page, SETTINGS, lambda route: route.fulfill(status=503, content_type="application/json", body=json.dumps(down)))
        page.locator(WARN).select_option("180")
        failed = page.evaluate("(e) => t('limits.failed', e)", down)
        said = page.evaluate("(e) => t('maint.cannotAsk', e)", down)
        ok, got = settled(page, lambda: page.evaluate(MESSAGES), lambda g: g == [failed, said, said])
        assert ok, f"the warning's own line, the read's on the alert's and under the announcement: {got}"
        show_view(page, "tiles")
        show_view(page, "maint")
        ok, got = settled(page, lambda: page.evaluate(MESSAGES), lambda g: g == [said, said, said])
        assert ok, f"both rows say the read failed, as the announcement's does: {got}"
        page.unroute(SETTINGS)
        mark = stream.mark()
        api_call("PUT", "api/v1/settings", DEFAULTS)
        stream.wait("the change", "change", match=settings_change(**DEFAULTS), mark=mark, server=True)
        ok, got = settled(page, lambda: page.evaluate(MESSAGES), lambda g: g == ["", "", ""])
        assert ok, f"a read that succeeds takes the three lines back: {got}"
    finally:
        cleanup(lambda: page.unroute(SETTINGS), reset)
    show_view(page, "tiles")


def test_the_card_is_a_holder_of_settings(page, case):
    """A role with restart and without settings (/me stubbed) opens
    Maintenance without the card, its sections menu without its entry;
    a role holding settings alone opens Maintenance, which it could not
    before (D6-A), and finds the card with both rows open."""
    try:
        stub_me(page, ["read", "preferences", "restart"])
        assert wait_until(page, "document.body.classList.contains('no-settings') && !document.body.classList.contains('no-maint')", 5), \
            "restart without settings"
        show_view(page, "maint")
        assert css_hidden(page, "#sys-limits"), "no Limits without settings"
        assert css_hidden(page, "#maint-page .toc-link[data-target='sys-limits']"), "nor its entry in the sections menu"
        page.unroute("**/api/v1/me")
        show_view(page, "tiles")
        stub_me(page, ["read", "preferences", "settings"])
        assert wait_until(page, "document.body.classList.contains('no-restart') && !document.body.classList.contains('no-settings')", 5), \
            "settings alone"
        show_view(page, "maint")
        assert page.locator("#sys-limits").is_visible(), "the card, with settings alone"
        assert values_are(page, "default", "default") and page.locator(WARN).is_enabled() and page.locator(ALERT).is_enabled(), \
            "both rows, open"
        named_buttons(page, "Maintenance with settings alone")
    finally:
        cleanup(lambda: page.unroute("**/api/v1/me"), lambda: page.evaluate("loadMe()"))
    show_view(page, "tiles")


@pytest.mark.clock
def test_a_window_sign_follows_a_change_of_the_limits(page, case, stream):
    """The page's clock 50 minutes after the contact's window opened: with
    the warning at 1 h its room shows it plain; the warning set to 30 min
    from another browser turns it amber with a triangle, and the alert set
    to 45 min on Maintenance red with a cross, each without a reload: the
    change event reads the rooms again (D5-A). Fails on the page before,
    whose tiles kept the times of the last read of the rooms."""
    api_call("PUT", "api/v1/settings", {"window_warn_after": "1h", "window_alert_after": "2h"})
    try:
        contact = open_contact()
        assert contact, "the open window contact, with its window's times"
        page.clock.set_system_time(datetime.fromisoformat(contact["window"]["open_since"]) + timedelta(minutes=50))
        page.evaluate("loadRooms()")  # the times of 1 h and 2 h, read
        room = page.locator(".room", has=page.locator(".room-name", has_text="Kind 1"))
        head_shows(room, ["#icon-window-sash-open"], "open 50 min, the warning at 1 h: plain")
        mark = stream.mark()
        api_call("PUT", "api/v1/settings", {"window_warn_after": "30m"})
        stream.wait("the warning's change", "change", match=settings_change(window_warn_after="30m"), mark=mark, server=True)
        head_shows(room, ["#icon-window-sash-open+#mark-sash-warn"], "the warning at 30 min: amber, a triangle, without a reload")
        show_view(page, "maint")
        assert values_are(page, "1800", "7200"), "30 min and 2 h"
        mark = stream.mark()
        page.locator(ALERT).select_option("2700")
        stream.wait("the alert's change", "change", match=settings_change(window_alert_after="45m"), mark=mark, server=True)
        show_view(page, "tiles")
        head_shows(room, ["#icon-window-sash-open+#mark-sash-x"], "the alert at 45 min, set on Maintenance: red, a cross")
    finally:
        reset()
    show_view(page, "tiles")
