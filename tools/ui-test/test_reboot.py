# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings → Maintenance: the reboot of the computer. GET, POST
and DELETE /api/v1/reboot are stubbed, and /healthz once the reboot has
begun, so nothing is rebooted; the routes have their Go tests."""
import json
from datetime import datetime, timedelta, timezone

from conftest import api_call
from helpers import css_hidden, show_view, wait_until

AVAILABLE = {"available": True, "by": "systemd"}
MISSING = "no reboot helper installed: sudo massimo install -reboot-helper"
ROUTE = "**/api/v1/reboot"


def iso(seconds):
    """The time seconds from now, as the server writes it."""
    return (datetime.now(timezone.utc) + timedelta(seconds=seconds)).strftime("%Y-%m-%dT%H:%M:%SZ")


class RebootStub:
    """GET answers self.answer; a POST or a DELETE is kept with its body
    and moves the answer on: a POST to a reboot that waits (or has begun,
    with no delay), a DELETE back to available."""

    def __init__(self, page, answer):
        self.page, self.answer, self.requests = page, dict(answer), []
        page.route(ROUTE, self.handle)

    def handle(self, route):
        req = route.request
        if req.method == "GET":
            route.fulfill(status=200, content_type="application/json", body=json.dumps(self.answer))
            return
        self.requests.append((req.method, json.loads(req.post_data) if req.post_data else None))
        if req.method == "POST":
            delay = json.loads(req.post_data).get("delay", 60)
            at = iso(delay)
            self.answer = {"available": False, "by": "systemd", "at": at,
                           "blocked": "the reboot has begun" if delay == 0 else "a reboot is waiting", "begun": delay == 0}
            route.fulfill(status=202, content_type="application/json", body=json.dumps({"ok": True, "by": "systemd", "at": at}))
        else:
            self.answer = dict(AVAILABLE)
            route.fulfill(status=200, content_type="application/json", body='{"ok": true}')

    def close(self):
        self.page.unroute(ROUTE)


def reboot_text(page):
    return page.locator("#reboot-text").inner_text()


def test_reboot_not_offered(page):
    """Where the computer cannot be rebooted from here, the delay and the
    button stay hidden and the server's reason is shown."""
    stub = RebootStub(page, {"available": False, "by": "systemd", "blocked": MISSING})
    try:
        show_view(page, "system")
        want = page.evaluate("t('reboot.unavailable', {reason: %s})" % json.dumps(MISSING))
        assert wait_until(page, "document.getElementById('reboot-text').textContent === %s" % json.dumps(want), 5), "the reason"
        assert page.locator("#reboot-btn").is_hidden() and page.locator("#reboot-delay-row").is_hidden(), "no delay, no button"
        assert page.locator("#reboot-cancel").is_hidden(), "nothing to take back"
    finally:
        stub.close()


def test_reboot_asks_with_its_delay(page):
    """The button asks in Maintenance, naming the delay, 60 s to begin
    with, or now for 0; Cancel takes the question back without a request,
    and a delay the server would refuse is refused in the page."""
    stub = RebootStub(page, AVAILABLE)
    try:
        show_view(page, "system")
        btn = page.locator("#reboot-btn")
        btn.wait_for(state="visible")
        assert reboot_text(page) == page.evaluate("t('maint.rebootText')"), "what a reboot does"
        delay = page.locator("#reboot-delay")
        assert delay.input_value() == "60", "60 s to begin with"
        for value, want in (("60", "tn('reboot.confirm', 60)"), ("0", "t('reboot.confirmNow')")):
            delay.fill(value)
            btn.click()
            box = page.locator(".reboot .confirm-box")
            assert box.is_visible() and btn.is_hidden(), "the reboot asks in the page"
            assert box.locator(".confirm-text").inner_text() == page.evaluate(want), f"the question for {value} s"
            box.locator(".confirm-no").click()
            assert page.locator(".reboot .confirm-box").count() == 0 and btn.is_visible(), "Cancel takes it back"
        for value in ("3601", "1.5", "-1", ""):
            delay.fill(value)
            btn.click()
            want = page.evaluate("t('reboot.badDelay', {max: 3600})")
            assert page.locator("#reboot-msg").inner_text() == want and page.locator(".reboot .confirm-box").count() == 0, \
                f"{value!r} refused in the page"
        assert not stub.requests, "nothing sent"
    finally:
        stub.close()


def test_reboot_waits_and_is_taken_back(page):
    """A reboot after a delay is asked for with that delay; while it waits
    the row says when, and Take the reboot back takes it back."""
    stub = RebootStub(page, AVAILABLE)
    try:
        show_view(page, "system")
        page.locator("#reboot-btn").wait_for(state="visible")
        page.locator("#reboot-delay").fill("90")
        page.locator("#reboot-btn").click()
        page.locator(".reboot .confirm-yes").click()
        cancel = page.locator("#reboot-cancel")
        cancel.wait_for(state="visible")
        assert stub.requests == [("POST", {"delay": 90})], f"one request with the delay: {stub.requests}"
        at = stub.answer["at"]
        want = page.evaluate("t('reboot.waiting', {time: fmtInstant(new Date(Date.parse(%s)))})" % json.dumps(at))
        assert reboot_text(page) == want, "when it comes"
        assert page.locator("#reboot-btn").is_hidden() and page.locator("#reboot-delay-row").is_hidden(), "no second reboot"
        assert not page.evaluate("rebootState.following"), "not yet followed"
        cancel.click()
        page.locator("#reboot-btn").wait_for(state="visible")
        assert stub.requests[-1] == ("DELETE", None), f"taken back: {stub.requests}"
        assert page.locator("#reboot-msg").inner_text() == page.evaluate("t('reboot.cancelled')"), "it says so"
        assert cancel.is_hidden(), "nothing more to take back"
    finally:
        stub.close()


def test_reboot_follows_the_computer_back(page):
    """With no delay the page follows the computer at once: it asks
    /healthz until it reports a new start, then says the computer is
    back."""
    stub = RebootStub(page, AVAILABLE)
    health = api_call("GET", "healthz")
    later = dict(health, started_at="2099-01-02T03:04:05Z")
    calls = {"after": 0}

    def healthz(route):
        if stub.requests:
            calls["after"] += 1
        body = later if calls["after"] > 2 else health
        route.fulfill(status=200, content_type="application/json", body=json.dumps(body))
    page.route("**/healthz", healthz)
    try:
        show_view(page, "system")
        page.locator("#reboot-btn").wait_for(state="visible")
        page.locator("#reboot-delay").fill("0")
        page.locator("#reboot-btn").click()
        page.locator(".reboot .confirm-yes").click()
        assert wait_until(page, "rebootState.following", 5), "the page follows the reboot"
        assert reboot_text(page) == page.evaluate("t('reboot.rebooting')"), "it says the computer reboots"
        back = page.evaluate("t('reboot.back', {time: fmtInstant(new Date(Date.parse('2099-01-02T03:04:05Z')))})")
        assert wait_until(page, "!rebootState.following", 20), "the page sees the computer back"
        assert reboot_text(page) == back, "it says the computer is back"
        assert stub.requests == [("POST", {"delay": 0})], f"one request: {stub.requests}"
    finally:
        page.unroute("**/healthz")
        stub.close()


def test_reboot_that_failed(page):
    """A reboot that began but failed, while the old server still answers,
    is drawn again with the server's reason."""
    stub = RebootStub(page, AVAILABLE)
    try:
        show_view(page, "system")
        page.locator("#reboot-btn").wait_for(state="visible")
        page.locator("#reboot-delay").fill("0")
        page.locator("#reboot-btn").click()
        page.locator(".reboot .confirm-yes").click()
        assert wait_until(page, "rebootState.following", 5), "the page follows the reboot"
        error = "the reboot helper did not take the request within 30s"
        stub.answer = dict(AVAILABLE, error=error)
        want = page.evaluate("t('maint.rebootText') + ' ' + t('reboot.lastFailed', {error: %s})" % json.dumps(error))
        assert wait_until(page, "!rebootState.following", 10), "the page stops following"
        assert reboot_text(page) == want, "it says why"
        assert page.locator("#reboot-btn").is_visible(), "and offers the reboot again"
    finally:
        stub.close()


def test_reboot_row_needs_its_capability(page):
    """A manager (/me stubbed), who may restart the server, sees
    Maintenance without the reboot."""
    me = {"mode": "users", "user": {"name": "manager1", "role": "manager", "has_password": True},
          "role": {"key": "manager"}, "capabilities": ["read", "preferences", "restart"], "token": {"kind": "session"}}
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(me)))
    try:
        page.evaluate("loadMe()")
        assert wait_until(page, "document.body.classList.contains('no-reboot')", 5), "no reboot for a manager"
        show_view(page, "system")
        assert page.locator("#sys-maint").is_visible() and page.locator("#restart-btn").count() == 1, "Maintenance is there"
        assert css_hidden(page, ".action-row.reboot"), "without the reboot"
    finally:
        page.unroute("**/api/v1/me")
        page.evaluate("loadMe()")
