# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Refresh now (AP RefreshBatteries, Block 2, D10-B): Maintenance's row
under Refresh the Cubes every reads every Cube the server serves again,
one after the other, each Cube's outcome under the row; a Cube's panel
has Refresh beside its Reboot, its outcome in the Cubes' message line.
Both need restart: a role without it sees neither, a manager without
settings sees the row without the interval. POST …/refresh is held and
answered by the test, so that the fake Cube is left alone; the route
has its Go tests."""
import json
import re

from conftest import api_call
from helpers import VIEWER, assert_busy, busy_state, css_hidden, named_buttons, scroll_width, show_view, wait_until

REFRESH = "**/api/v1/cubes/*/refresh"
SECOND = "KEQ0000300"
PAGE_PANEL = "#cube-page .cube-panel[data-serial='KEQ0000100']"
OK = '{"ok": true, "budget": {"duty_cycle_pct": 0, "free_memory_slots": 50, "updated_at": "2026-11-23T18:00:00Z"}}'
NOT_REACHED = "refresh of %s: context deadline exceeded (link offline)"


class Held:
    """The refreshes the page sends, held until the test answers them:
    each one's serial in turn, its body, and its route."""

    def __init__(self, page):
        self.page, self.routes, self.serials, self.bodies = page, [], [], []
        page.route(REFRESH, self.handle)

    def handle(self, route):
        self.serials.append(route.request.url.split("/")[-2])
        self.bodies.append(route.request.post_data_json)
        self.routes.append(route)

    def wait(self, n, seconds=5):
        """Waits until n refreshes have been sent."""
        for _ in range(seconds * 10):
            if len(self.routes) >= n:
                return True
            self.page.wait_for_timeout(100)
        return len(self.routes) >= n

    def answer(self, i, status=200, error=""):
        body = json.dumps({"error": error}) if error else OK
        self.routes[i].fulfill(status=status, content_type="application/json", body=body)

    def close(self):
        self.page.unroute(REFRESH)


def done_re(page, cube):
    """The line of a Cube read again, its time as the page writes it."""
    said = page.evaluate("t('maint.refreshDone', {cube: %s, time: '\\u0000'})" % json.dumps(cube))
    before, after = said.split("\u0000")
    return re.compile(re.escape(before) + r"\d\d:\d\d:\d\d" + re.escape(after) + "$")


def test_refresh_now_reads_every_cube(page, case, watch):
    """The row comes right after the refresh's interval; Refresh now
    sends one Cube's refresh at a time, turning until the last has
    answered; each Cube's line says it was read again with its time, or
    the error, as its answer comes."""
    watch.expected.append(("/cubes/%s/refresh" % SECOND, 502))
    api_call("POST", "api/v1/cubes", {"serial": SECOND, "address": "127.0.0.1:9", "name": "Second"})
    held = Held(page)
    try:
        page.evaluate("loadCubes()")
        show_view(page, "maint")
        assert wait_until(page, "state.cubes.length === 2", 5), "two Cubes served"
        row = page.locator(".action-row.refresh-now")
        assert row.is_visible(), "the row, for a holder of restart"
        assert page.evaluate("document.querySelector('#interval-refresh').closest('.interval-row').nextElementSibling"
                             ".classList.contains('refresh-now')"), "right after Refresh the Cubes every"
        assert row.locator(".action-title").inner_text() == page.evaluate("t('maint.refreshNowTitle')"), "its title"
        btn = page.locator("#refresh-now")
        assert btn.inner_text() == page.evaluate("t('maint.refreshNow')") and btn.is_enabled(), "Refresh now"
        btn.click()
        assert held.wait(1), "the first Cube's refresh sent"
        assert_busy(page, "#refresh-now", "Refresh now")
        page.wait_for_timeout(300)
        assert held.serials == ["KEQ0000100"] and held.bodies == [{}], f"one Cube at a time, nothing in the body: {held.serials}"
        held.answer(0)
        assert held.wait(2), "the second Cube's after the first has answered"
        assert held.serials == ["KEQ0000100", SECOND], f"every Cube in the list's order: {held.serials}"
        lines = row.locator(".refresh-result")
        first = page.evaluate("state.cubes[0].name || state.cubes[0].serial")
        assert wait_until(page, "document.querySelectorAll('.refresh-now .refresh-result')[0].classList.contains('ok')", 5), \
            "the first Cube's line"
        assert done_re(page, first).match(lines.nth(0).inner_text()), f"read again, with its time: {lines.nth(0).inner_text()!r}"
        assert lines.nth(1).is_hidden(), "nothing yet beside the second"
        assert_busy(page, "#refresh-now", "Refresh now, until the last")
        held.answer(1, 502, NOT_REACHED % SECOND)
        assert wait_until(page, "document.querySelectorAll('.refresh-now .refresh-result')[1].classList.contains('bad')", 5), \
            "the second Cube's line"
        want = page.evaluate("t('maint.refreshFailed', {cube: 'Second', error: %s})" % json.dumps(NOT_REACHED % SECOND))
        assert lines.nth(1).inner_text() == want, f"the error: {lines.nth(1).inner_text()!r}"
        assert wait_until(page, "!document.getElementById('refresh-now').disabled", 5), "the button back"
        assert busy_state(page, "#refresh-now") == [False, False, None], "and no longer turning"
        named_buttons(page, "Maintenance")
        assert scroll_width(page) <= case.width, "no horizontal scroll with the lines"
    finally:
        held.close()
        api_call("DELETE", "api/v1/cubes/" + SECOND)
        page.evaluate("loadCubes()")
    show_view(page, "tiles")


def test_cube_refresh_beside_reboot(page, case, watch):
    """A Cube's panel on Cube settings: Refresh the Cube beside Reboot,
    which asks nothing and turns until its answer; the Cubes' message
    line says the Cube was read again, or the error."""
    watch.expected.append(("/cubes/KEQ0000100/refresh", 502))
    held = Held(page)
    try:
        show_view(page, "cube")
        panel = page.locator(PAGE_PANEL)
        panel.wait_for(state="visible")
        btn = panel.locator(".cube-refresh-btn")
        assert btn.is_visible() and btn.inner_text() == page.evaluate("t('cubes.refresh')"), "Refresh the Cube"
        assert panel.locator(".cube-refresh").evaluate("(e) => e.nextElementSibling.classList.contains('cube-reboot')"), "beside Reboot"
        btn.click()
        assert held.wait(1), "the refresh sent"
        assert panel.locator(".confirm-box").count() == 0, "no question"
        assert_busy(page, PAGE_PANEL + " .cube-refresh-btn", "Refresh the Cube")
        assert held.serials == ["KEQ0000100"] and held.bodies == [{}], f"the Cube's refresh: {held.serials}"
        held.answer(0)
        want = page.evaluate("t('cubes.refreshed', {serial: 'KEQ0000100'})")
        assert wait_until(page, "document.getElementById('cube-page-msg').textContent === %s" % json.dumps(want), 5), "read again"
        assert "ok" in (page.locator("#cube-page-msg").get_attribute("class") or ""), "in green"
        assert wait_until(page, "!document.querySelector(%s).disabled" % json.dumps(PAGE_PANEL + " .cube-refresh-btn"), 5), "back"
        btn.click()
        assert held.wait(2), "sent again"
        held.answer(1, 502, NOT_REACHED % "KEQ0000100")
        want = page.evaluate("t('cubes.failed', {serial: 'KEQ0000100', error: %s})" % json.dumps(NOT_REACHED % "KEQ0000100"))
        assert wait_until(page, "document.getElementById('cube-page-msg').textContent === %s" % json.dumps(want), 5), "the error"
        assert "bad" in (page.locator("#cube-page-msg").get_attribute("class") or ""), "in red"
        named_buttons(page, "Cube settings")
        assert scroll_width(page) <= case.width, "no horizontal scroll with the button"
    finally:
        held.close()
    show_view(page, "tiles")


def stub_me(page, capabilities):
    """The page signed in with these capabilities (/me stubbed); the
    owner's token still answers the reads."""
    me = dict(VIEWER, capabilities=capabilities)
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(me)))
    page.evaluate("loadMe()")


def test_refresh_needs_restart(page, case):
    """Without restart (settings and pair, /me stubbed), neither the row
    nor the panel's Refresh, while the interval and the pairing are
    there; with restart and without settings, as a manager, the row
    without the interval, and Refresh beside the pairing."""
    try:
        stub_me(page, ["read", "preferences", "settings", "pair"])
        assert wait_until(page, "document.body.classList.contains('no-restart')", 5), "no restart"
        show_view(page, "maint")
        assert page.locator("#interval-refresh").is_visible(), "the interval, with settings"
        assert css_hidden(page, ".action-row.refresh-now"), "no Refresh now without restart"
        show_view(page, "cube")
        page.locator(PAGE_PANEL).wait_for(state="visible")
        assert css_hidden(page, PAGE_PANEL + " .cube-refresh"), "no Refresh on the Cube's panel"
        page.unroute("**/api/v1/me")
        stub_me(page, ["read", "preferences", "restart", "pair"])
        assert wait_until(page, "document.body.classList.contains('no-settings') && !document.body.classList.contains('no-restart')", 5), \
            "restart without settings"
        show_view(page, "maint")
        assert page.locator(".action-row.refresh-now").is_visible(), "Refresh now for a manager"
        assert css_hidden(page, ".interval-row:has(#interval-refresh)"), "without the interval"
        show_view(page, "cube")
        page.locator(PAGE_PANEL).wait_for(state="visible")
        assert page.locator(PAGE_PANEL + " .cube-refresh-btn").is_visible(), "Refresh on the Cube's panel"
    finally:
        page.unroute("**/api/v1/me")
        page.evaluate("loadMe()")
    show_view(page, "tiles")
