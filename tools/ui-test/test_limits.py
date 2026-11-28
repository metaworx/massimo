# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Maintenance → Limits (AP ServerLimits v1.0): a card of its own after
Maintenance, for a holder of settings. Its rows choose how long a window
may stay open before its sign warns and before it alerts (Block 1), how
long a radiator and a wall thermostat may report nothing new before they
count as stale, and how long the log keeps the Cubes' events (Block 2),
each saved at once, its default the first choice and sent as null
(D3-A); the window's steps the server would refuse are off, equal
allowed (D4-A); a refusal shows on its row in the page's language, by its
code; a change of a window limit or a stale threshold, from this page or
another, reads the rooms again, so that the tiles' signs follow at once
(D5-A); a role holding settings alone opens the page (D6-A); a threshold
-stale-after fixes is read-only; a shorter log asks first (D7-A). The
window contact of the fake Cube's greeting is open from the set's start;
each test puts the limits back to their defaults."""
import json
import sys
from datetime import datetime, timedelta

import pytest

from conftest import CUBE, api_call
from helpers import (ROOMS, answered, cleanup, css_hidden, frames, hold, named_buttons, safe_route, scroll_width, sent_requests, settled, show_view,
                     stub_me, taken, undo, wait_held, wait_until)
from test_maint_updates import settings_change
from test_rooms import change_room, head_shows

WARN, ALERT = "#limit-window-warn", "#limit-window-alert"
STALE, STALE_WALL, KEEP = "#limit-stale", "#limit-stale-wall", "#limit-log-keep"
# The steps of both window rows in seconds: 1 to 45 min, 1 to 24 h (the plan's list).
STEPS = [60, 120, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 14400, 28800, 43200, 86400]
H, D = 3600, 86400
# The stale thresholds' steps in seconds, and the log's in days, Forever (0) last (the plan's lists).
STALE_STEPS = [12 * H, 24 * H, 36 * H, 50 * H, 3 * D, 4 * D, 5 * D, 7 * D, 14 * D, 30 * D]
STALE_WALL_STEPS = [6 * H, 12 * H, 24 * H, 36 * H, 48 * H, 3 * D, 7 * D]
KEEP_STEPS = [14, 30, 60, 90, 180, 365, 730, 1825, 3650, 0]
DEFAULTS = {"window_warn_after": None, "window_alert_after": None}
# Every limit of the card at its default.
ALL_DEFAULTS = {**DEFAULTS, "stale_after": None, "stale_after_wall": None, "log.keep_days": None}
# A row's options as the page has them: value, text and whether it is off.
OPTIONS = "(s) => [...document.querySelector(s).options].map((o) => [o.value, o.textContent, o.disabled])"
# The steps' texts as the part writes them (limitLabel of Limits.tsx): a
# duration in minutes, in hours up to two days, in days from three; the
# log's days in days, in years from two, Forever for 0.
LABELS = """([steps, days]) => steps.map((n) => days ? (n === 0 ? t('limits.forever') : n >= 730 && n % 365 === 0 ? fmtUnit(n / 365, 'year')
  : fmtUnit(n, 'day')) : n >= 3 * 86400 && n % 86400 === 0 ? fmtUnit(n / 86400, 'day') : n % 3600 === 0 ? fmtUnit(n / 3600, 'hour')
  : fmtUnit(n / 60, 'minute'))"""
# The refusal's params of a PUT of the alert at 15 min under a warning at 30 min: the two durations.
REFUSED = {"alert": "15m", "warning": "30m"}
# The settings' route, which the page reads and writes.
SETTINGS = "**/api/v1/settings"
# The lines under the card's rows and under the announcement, in that order.
ROWS = [WARN, ALERT, STALE, STALE_WALL, KEEP]
MESSAGES = "(ids) => ids.map((s) => document.querySelector(s + '-msg').textContent)"


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
    """A cleanup's step: every limit of the card back at its default, a failure said (undo)."""
    undo("the limits at their defaults", lambda: api_call("PUT", "api/v1/settings", ALL_DEFAULTS), absent=())


def labels(page, steps, days=False):
    """The steps' texts as the part writes them."""
    return page.evaluate(LABELS, [steps, days])


def messages(page):
    """The lines under the card's rows and under the announcement."""
    return page.evaluate(MESSAGES, ROWS + ["#announce"])


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
        names = labels(page, STEPS)
        for sel, title, hint, minutes in ((WARN, "limits.windowWarn", "limits.windowWarnHint", 5), (ALERT, "limits.windowAlert", "limits.windowAlertHint", 10)):
            first = page.evaluate("(n) => t('limits.default', {value: fmtUnit(n, 'minute')})", minutes)
            got = page.evaluate(OPTIONS, sel)
            assert [o[:2] for o in got] == [["default", first]] + [[str(s), lab] for s, lab in zip(STEPS, names)], f"{sel}: {got}"
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
    asked on every other row, the stale thresholds' and the log's among
    them, and under the announcement, and leaves the warning's own line.
    Maintenance shown again, its read refused: every row says it. The
    server answering again, a change from another browser reads the
    settings, and that read takes back every line."""
    watch.expected.append(("/api/v1/settings", 503))
    api_call("PUT", "api/v1/settings", ALL_DEFAULTS)
    down = {"error": "the store is closed"}
    try:
        show_view(page, "maint")
        assert values_are(page, "default", "default"), "both at Default"
        safe_route(page, SETTINGS, lambda route: route.fulfill(status=503, content_type="application/json", body=json.dumps(down)))
        page.locator(WARN).select_option("180")
        failed = page.evaluate("(e) => t('limits.failed', e)", down)
        said = page.evaluate("(e) => t('maint.cannotAsk', e)", down)
        ok, got = settled(page, lambda: messages(page), lambda g: g == [failed] + [said] * 5)
        assert ok, f"the warning's own line, the read's on the other rows and under the announcement: {got}"
        show_view(page, "tiles")
        show_view(page, "maint")
        ok, got = settled(page, lambda: messages(page), lambda g: g == [said] * 6)
        assert ok, f"every row says the read failed, as the announcement's does: {got}"
        page.unroute(SETTINGS)
        mark = stream.mark()
        api_call("PUT", "api/v1/settings", ALL_DEFAULTS)
        stream.wait("the change", "change", match=settings_change(**ALL_DEFAULTS), mark=mark, server=True)
        ok, got = settled(page, lambda: messages(page), lambda g: g == [""] * 6)
        assert ok, f"a read that succeeds takes the lines back: {got}"
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


# --- the stale thresholds and the log's days (Block 2) -----------------------

def row_of(page, sel):
    """The row of a limit's select."""
    return page.locator(".limit-row", has=page.locator(sel))


def test_the_new_rows_show_what_is_in_effect(page, case):
    """The stale thresholds and the log's days at their defaults: Default
    first and chosen, named with its value, 50 h, 24 h and 30 days, the
    plan's steps after it, the log's Forever last, the line under each
    saying what it does; a threshold set through the API that is no step,
    40 h, listed as itself between 36 and 50 h, and the log's 45 days
    between 30 and 60, chosen, without a reload."""
    api_call("PUT", "api/v1/settings", ALL_DEFAULTS)
    try:
        show_view(page, "maint")
        assert wait_until(page, "['%s', '%s', '%s'].every((s) => document.querySelector(s).value === 'default')" % (STALE, STALE_WALL, KEEP), 5), \
            "all three at Default"
        for sel, title, hint, steps, days, first in (
                (STALE, "limits.stale", "limits.staleHint", STALE_STEPS, False, page.evaluate("fmtUnit(50, 'hour')")),
                (STALE_WALL, "limits.staleWall", "limits.staleWallHint", STALE_WALL_STEPS, False, page.evaluate("fmtUnit(24, 'hour')")),
                (KEEP, "limits.keepLog", "limits.keepLogHint", KEEP_STEPS, True, page.evaluate("fmtUnit(30, 'day')"))):
            got = page.evaluate(OPTIONS, sel)
            want = [["default", page.evaluate("(v) => t('limits.default', {value: v})", first)]] + \
                [[str(n), lab] for n, lab in zip(steps, labels(page, steps, days))]
            assert [o[:2] for o in got] == want, f"{sel}: {got}"
            assert not any(o[2] for o in got) and page.locator(sel).is_enabled(), f"{sel}: every choice open"
            assert page.locator(f"label[for='{sel[1:]}']").inner_text() == page.evaluate("(k) => t(k)", title), f"{sel}'s title"
            assert page.locator(f"{sel}-text").inner_text() == page.evaluate("(k) => t(k)", hint), f"{sel}'s line"
        assert page.evaluate(OPTIONS, KEEP)[-1][:2] == ["0", page.evaluate("t('limits.forever')")], "Forever last"
        api_call("PUT", "api/v1/settings", {"stale_after": "40h", "log.keep_days": 45})
        assert wait_until(page, "document.querySelector('%s').value === '%d' && document.querySelector('%s').value === '45'" % (STALE, 40 * H, KEEP), 5), \
            "40 h and 45 days chosen, without a reload"
        stale = [o[0] for o in page.evaluate(OPTIONS, STALE)]
        keep = [o[0] for o in page.evaluate(OPTIONS, KEEP)]
        assert stale[stale.index(str(36 * H)) + 1] == str(40 * H) and stale[stale.index(str(40 * H)) + 1] == str(50 * H), f"40 h between 36 and 50: {stale}"
        assert keep[keep.index("30") + 1] == "45" and keep[keep.index("45") + 1] == "60", f"45 days between 30 and 60: {keep}"
        named_buttons(page, "Maintenance with every limit")
        assert scroll_width(page) <= case.width, "no horizontal scroll with every limit"
    finally:
        reset()
    show_view(page, "tiles")


def test_the_new_rows_send_their_steps(page, case):
    """Each choice sends exactly its value, held at its route: 36 h for the
    radiator thermostats, 3 days for the wall thermostats, as durations;
    90 days and Forever for the log, as numbers, at once, being no shorter;
    Default of a threshold as null. Each row's line says it was saved in
    its own words: the thresholds' that the rooms follow, the log's that it
    applies at once. The server holds each, applied at once."""
    api_call("PUT", "api/v1/settings", ALL_DEFAULTS)
    held, sent = [], []
    try:
        show_view(page, "maint")
        assert wait_until(page, "document.querySelector('%s').value === 'default'" % KEEP, 5), "read"
        hold(page, SETTINGS, "PUT", held, sent)
        for n, (sel, choice, body, saved) in enumerate([(STALE, str(36 * H), {"stale_after": "36h"}, "limits.saved"),
                                                        (STALE_WALL, str(3 * D), {"stale_after_wall": "72h"}, "limits.saved"),
                                                        (KEEP, "90", {"log.keep_days": 90}, "limits.keepSaved"),
                                                        (KEEP, "0", {"log.keep_days": 0}, "limits.keepSaved"),
                                                        (STALE, "default", {"stale_after": None}, "limits.saved")]):
            page.locator(sel).select_option(choice)
            assert wait_held(page, held, n=n + 1), f"{body} sent"
            assert sent[n] == body, f"exactly {body}: {sent}"
            assert row_of(page, sel).locator(".confirm-box").count() == 0, f"{body}: no question"
            with put_taken(page):
                held[n].continue_()
            frames(page)  # the save ended: the choice takes the next one
            want = page.evaluate("(k) => t(k)", saved)
            assert page.locator(sel + "-msg").inner_text() == want, f"{body}: the row's own line, {page.locator(sel + '-msg').inner_text()!r}"
        got = api_call("GET", "api/v1/settings")
        assert got["stale_after"]["set"] is False and got["stale_after"]["in_effect"] == "50h", f"the default again: {got['stale_after']}"
        assert got["stale_after_wall"]["value"] == "72h" and got["stale_after_wall"]["in_effect"] == "72h", f"3 days, at once: {got['stale_after_wall']}"
        assert got["log.keep_days"]["value"] == "0" and got["log.keep_days"]["set"] is True, f"Forever: {got['log.keep_days']}"
    finally:
        cleanup(lambda: page.unroute(SETTINGS), reset)
    show_view(page, "tiles")


def test_a_shorter_log_asks_first(page, case):
    """The log at its default, 30 days: 14 days asks in its row, naming 14
    days, the choice shown while it asks and the focus left on it; Cancel
    leaves Default and the focus on the choice. 14 days again and Shorten
    send 14, the row's line saying it applies at once, the
    first request of the row: Cancel sent nothing. 60 days then and
    Forever are sent at once; Default from Forever asks too, naming 30
    days, and Shorten sends null (D7-A)."""
    api_call("PUT", "api/v1/settings", ALL_DEFAULTS)
    held, sent = [], []
    sel, row = page.locator(KEEP), None
    try:
        show_view(page, "maint")
        row = row_of(page, KEEP)
        assert wait_until(page, "document.querySelector('%s').value === 'default'" % KEEP, 5), "30 days, the default"
        hold(page, SETTINGS, "PUT", held, sent)
        sel.focus()  # as a user's choice leaves it, where select_option does not
        sel.select_option("14")
        row.locator(".confirm-box").wait_for()
        want = page.evaluate("t('limits.keepAsk', {value: fmtUnit(14, 'day')})")
        assert row.locator(".confirm-text").inner_text() == want, f"it names 14 days: {row.locator('.confirm-text').inner_text()!r}"
        assert row.locator(".confirm-yes").inner_text() == page.evaluate("t('limits.keepShorten')"), "Shorten"
        assert sel.input_value() == "14", "the choice shown while it asks"
        assert page.evaluate("document.activeElement.id") == KEEP[1:], "the focus left on the list"
        row.locator(".confirm-no").click()
        assert row.locator(".confirm-box").count() == 0, "the question goes"
        assert wait_until(page, "document.querySelector('%s').value === 'default' && document.activeElement === document.querySelector('%s')"
                          % (KEEP, KEEP), 5), "Default again, the focus on the choice"
        sel.select_option("14")
        row.locator(".confirm-box").wait_for()
        row.locator(".confirm-yes").click()
        assert wait_held(page, held), "Shorten sends it"
        assert sent == [{"log.keep_days": 14}], f"14 days, the row's first request: {sent}"
        with put_taken(page):
            held[0].continue_()
        frames(page)
        assert page.locator(KEEP + "-msg").inner_text() == page.evaluate("t('limits.keepSaved')"), "the log's line: it applies at once"
        for n, (choice, body) in enumerate([("60", {"log.keep_days": 60}), ("0", {"log.keep_days": 0})], start=1):
            sel.select_option(choice)
            assert wait_held(page, held, n=n + 1), f"{body} sent at once"
            assert sent[n] == body and row.locator(".confirm-box").count() == 0, f"{body}, no question: {sent}"
            with put_taken(page):
                held[n].continue_()
            frames(page)
        sel.select_option("default")
        row.locator(".confirm-box").wait_for()
        want = page.evaluate("t('limits.keepAsk', {value: fmtUnit(30, 'day')})")
        assert row.locator(".confirm-text").inner_text() == want, "Default from Forever asks, naming 30 days"
        row.locator(".confirm-yes").click()
        assert wait_held(page, held, n=4), "Shorten sends it"
        assert sent[3] == {"log.keep_days": None}, f"Default as null: {sent}"
        with put_taken(page):
            held[3].continue_()
        assert api_call("GET", "api/v1/settings")["log.keep_days"]["set"] is False, "the server holds none"
    finally:
        cleanup(lambda: page.unroute(SETTINGS), reset)
    show_view(page, "tiles")


def test_the_question_goes_where_its_choice_shortens_nothing(page, case):
    """14 days asks, the log at its default; another browser shortens the
    log to 7 days meanwhile: the change event's read leaves 14 days
    shortening nothing, and the store takes the question back, the list
    showing the 7 days stored, listed as itself, and the focus on it."""
    api_call("PUT", "api/v1/settings", ALL_DEFAULTS)
    try:
        show_view(page, "maint")
        row, sel = row_of(page, KEEP), page.locator(KEEP)
        assert wait_until(page, "document.querySelector('%s').value === 'default'" % KEEP, 5), "30 days, the default"
        sel.focus()  # as a user's choice leaves it, where select_option does not
        sel.select_option("14")
        row.locator(".confirm-box").wait_for()
        api_call("PUT", "api/v1/settings", {"log.keep_days": 7})
        assert wait_until(page, "!document.querySelector('.limit-row .confirm-box') && document.querySelector('%s').value === '7'" % KEEP, 5), \
            "the question gone, 7 days shown"
        assert page.evaluate("document.activeElement.id") == KEEP[1:], "the focus on the list"
    finally:
        reset()
    show_view(page, "tiles")


def test_shorten_waits_for_the_rows_save(page, case):
    """60 days chosen, its PUT held; 14 days chosen meanwhile asks, and
    Shorten stays off while the 60 days' save runs, so that it sends
    nothing unsaid; the save let go, 14 days still shortens the 60 now in
    effect, Shorten comes on, and it sends 14 after 60."""
    api_call("PUT", "api/v1/settings", ALL_DEFAULTS)
    held, sent = [], []
    try:
        show_view(page, "maint")
        row, sel = row_of(page, KEEP), page.locator(KEEP)
        assert wait_until(page, "document.querySelector('%s').value === 'default'" % KEEP, 5), "30 days, the default"
        hold(page, SETTINGS, "PUT", held, sent)
        sel.select_option("60")
        assert wait_held(page, held), "60 days sent"
        sel.select_option("14")
        row.locator(".confirm-box").wait_for()
        assert row.locator(".confirm-yes").is_disabled(), "Shorten off while the row's save runs"
        with put_taken(page):
            held[0].continue_()
        assert wait_until(page, "!document.querySelector('.limit-row .confirm-yes').disabled", 5), "Shorten on once it has answered"
        assert row.locator(".confirm-box").count() == 1, "14 days still shortens the 60 in effect"
        row.locator(".confirm-yes").click()
        assert wait_held(page, held, n=2), "Shorten sends it"
        assert sent == [{"log.keep_days": 60}, {"log.keep_days": 14}], f"60, then 14: {sent}"
        with put_taken(page):
            held[1].continue_()
    finally:
        cleanup(lambda: page.unroute(SETTINGS), reset)
    show_view(page, "tiles")


@pytest.mark.skipif(sys.platform == "darwin", reason="Chromium on the Mac opens a closed list at an arrow key instead of taking its next choice")
def test_the_arrow_keys_move_through_the_log_while_it_asks(page, case):
    """The log's list at its default, the keyboard on it: ArrowDown takes
    14 days, which asks, the focus staying on the list; ArrowDown again
    takes 30 days, which shortens nothing: the question goes and 30 days
    is sent at once. ArrowUp takes 14 days again, which asks, and Escape
    on the list takes it back; ArrowUp asks again, Tab reaches Cancel and
    then Shorten, and Enter sends 14. Chromium off the Mac takes an
    arrow key's choice in a closed list at once, as it does here."""
    api_call("PUT", "api/v1/settings", ALL_DEFAULTS)
    held, sent = [], []
    try:
        show_view(page, "maint")
        row, sel = row_of(page, KEEP), page.locator(KEEP)
        assert wait_until(page, "document.querySelector('%s').value === 'default'" % KEEP, 5), "30 days, the default"
        hold(page, SETTINGS, "PUT", held, sent)
        sel.focus()
        page.keyboard.press("ArrowDown")
        row.locator(".confirm-box").wait_for()
        assert sel.input_value() == "14" and page.evaluate("document.activeElement.id") == KEEP[1:], "14 days asks, the focus on the list"
        page.keyboard.press("ArrowDown")
        assert wait_held(page, held), "30 days sent at once"
        assert sent == [{"log.keep_days": 30}] and row.locator(".confirm-box").count() == 0, f"30 days, no question: {sent}"
        with put_taken(page):
            held[0].continue_()
        frames(page)
        page.keyboard.press("ArrowUp")
        row.locator(".confirm-box").wait_for()
        assert sel.input_value() == "14", "14 days asks again"
        page.keyboard.press("Escape")
        assert wait_until(page, "!document.querySelector('.limit-row .confirm-box') && document.querySelector('%s').value === '30'" % KEEP, 5), \
            "Escape on the list takes the question back"
        assert page.evaluate("document.activeElement.id") == KEEP[1:], "the focus on the list"
        page.keyboard.press("ArrowUp")
        row.locator(".confirm-box").wait_for()
        page.keyboard.press("Tab")
        assert page.evaluate("document.activeElement.classList.contains('confirm-no')"), "Tab reaches Cancel"
        page.keyboard.press("Tab")
        assert page.evaluate("document.activeElement.classList.contains('confirm-yes')"), "and then Shorten"
        page.keyboard.press("Enter")
        assert wait_held(page, held, n=2), "Enter on Shorten sends it"
        assert sent[1] == {"log.keep_days": 14}, f"14 days: {sent}"
        with put_taken(page):
            held[1].continue_()
    finally:
        cleanup(lambda: page.unroute(SETTINGS), reset)
    show_view(page, "tiles")


def test_a_threshold_a_flag_fixes_is_read_only(page, case):
    """The settings' read as a server started with -stale-after 2m answers
    it (stubbed): both stale thresholds are read-only, their lines naming
    the flag and its 2 min; the window limits and the log stay open."""
    def fixed(route):
        if route.request.method != "GET":
            route.fallback()
            return
        resp = route.fetch()
        body = resp.json()
        for key in ("stale_after", "stale_after_wall"):
            body[key] = dict(body[key], fixed_by="-stale-after", in_effect="2m")
        route.fulfill(response=resp, json=body)
    safe_route(page, SETTINGS, fixed)
    try:
        show_view(page, "maint")
        assert wait_until(page, "document.querySelector('%s').disabled && document.querySelector('%s').disabled" % (STALE, STALE_WALL), 5), \
            "both thresholds read-only"
        for sel, hint in ((STALE, "limits.staleHint"), (STALE_WALL, "limits.staleWallHint")):
            want = page.evaluate("""(k) => (t(k) + ' ' + t('maint.intervalFixed', {by: '-stale-after', value: fmtUnit(2, 'minute')}))
              .replace(/\\.\\.$/, '.')""", hint)
            assert page.locator(f"{sel}-text").inner_text() == want, f"{sel}'s line names the flag: {page.locator(sel + '-text').inner_text()!r}"
        for sel in (WARN, ALERT, KEEP):
            assert page.locator(sel).is_enabled(), f"{sel} open"
    finally:
        page.unroute(SETTINGS)
    show_view(page, "tiles")


def test_a_stale_threshold_reads_the_rooms_again(page, case, reads, stream):
    """The server flags a thermostat stale by its threshold at each read of
    the rooms and sends no event for a change of the threshold: a change
    of either threshold from another browser reads the rooms again, without
    a reload, and the tile draws what that read brings. The fake Cube's
    thermostats reported at the set's start, minutes ago, and a threshold
    is an hour at the least, so the server's own flag cannot turn within a
    test: the reads from the change on say the first room's devices 30
    hours old and stale (reads.change), and its tile shows the clock (D5-A).
    Fails on the page before, which read nothing at the change. The limits
    are at their defaults (each test's reset), and the page's own reads
    have all answered before the reads are counted."""
    iso = page.evaluate("new Date(Date.now() - 30 * 3600 * 1000).toISOString()")

    def stale(room):
        room["stale"] = True
        for d in room.get("devices") or []:
            d["stale"] = True
            d["last_seen"] = iso
            for k in d.get("received") or {}:
                d["received"][k] = iso
    clock = ".room-problems .problems-sum use[href='#icon-set-dial']"
    try:
        tile = page.locator(".room").first
        assert tile.locator(clock).count() == 0, "the first room's tile without the clock"
        answered(page)
        rooms = sent_requests(page, "GET", ROOMS)
        reads.change(ROOMS, change_room(page.evaluate("state.rooms[0].id"), stale))
        mark = stream.mark()
        api_call("PUT", "api/v1/settings", {"stale_after": "36h"})
        stream.wait("the threshold's change", "change", match=settings_change(stale_after="36h"), mark=mark, server=True)
        assert wait_held(page, rooms), "the rooms read again at the change"
        tile.locator(clock).wait_for(state="attached")
        mark = stream.mark()
        api_call("PUT", "api/v1/settings", {"stale_after_wall": "12h"})
        stream.wait("the wall threshold's change", "change", match=settings_change(stale_after_wall="12h"), mark=mark, server=True)
        assert wait_held(page, rooms, n=2), "and again at the wall thermostats' threshold's"
    finally:
        reset()
    show_view(page, "tiles")
