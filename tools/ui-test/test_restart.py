# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Settings → Software: the server's restart. GET and POST
/api/v1/restart are stubbed, and /healthz once the restart is asked for,
so the test server never restarts."""
import json

from conftest import api_call
from helpers import open_settings, wait_until

AVAILABLE = '{"available": true, "by": "systemd"}'
BLOCKED = '{"available": false, "blocked": "no service manager would start this server again"}'


def stub_restart(page, answer, posts):
    """GET /api/v1/restart answers answer; a POST is counted and answered
    202."""
    def handle(route):
        if route.request.method == "POST":
            posts.append(1)
            route.fulfill(status=202, content_type="application/json", body='{"ok": true, "by": "systemd"}')
        else:
            route.fulfill(status=200, content_type="application/json", body=answer)
    page.route("**/api/v1/restart", handle)


def test_restart_not_offered(page):
    """Where nobody would start the server again, the button stays hidden
    and the reason is shown."""
    posts = []
    stub_restart(page, BLOCKED, posts)
    open_settings(page)
    assert wait_until(page, "!document.getElementById('restart-text').classList.contains('hidden')", 5), "the reason is shown"
    assert "no service manager" in page.locator("#restart-text").inner_text(), "the server's reason"
    assert page.locator("#restart-btn").is_hidden(), "no button"
    page.unroute("**/api/v1/restart")


def test_restart_asks_in_the_page(page):
    """The button asks in the Software section, and Cancel takes the
    question back without a request."""
    posts = []
    stub_restart(page, AVAILABLE, posts)
    open_settings(page)
    btn = page.locator("#restart-btn")
    btn.wait_for(state="visible")
    assert page.locator("#restart-text").is_hidden(), "nothing to explain"
    btn.click()
    box = page.locator(".restart .confirm-box")
    assert box.is_visible() and btn.is_hidden(), "the restart asks in the page"
    box.locator(".confirm-no").click()
    assert page.locator(".restart .confirm-box").count() == 0 and btn.is_visible() and not posts, "Cancel takes it back, nothing sent"
    page.unroute("**/api/v1/restart")


def test_restart_follows_the_server_back(page):
    """After the restart is asked for, the page asks /healthz until it
    reports a new start, then says the server is back."""
    posts = []
    stub_restart(page, AVAILABLE, posts)
    health = api_call("GET", "healthz")
    later = dict(health, started_at="2099-01-02T03:04:05Z")
    calls = {"after": 0}

    def healthz(route):
        if posts:
            calls["after"] += 1
        # the first check after the request still finds the old server
        body = later if calls["after"] > 1 else health
        route.fulfill(status=200, content_type="application/json", body=json.dumps(body))
    page.route("**/healthz", healthz)
    open_settings(page)
    btn = page.locator("#restart-btn")
    btn.wait_for(state="visible")
    btn.click()
    page.locator(".restart .confirm-yes").click()
    assert wait_until(page, "restartState.following", 5), "the page follows the restart"
    restarting = page.evaluate("t('restart.restarting')")
    assert page.locator("#restart-text").inner_text() == restarting, "it says the server restarts"
    back = page.evaluate("t('restart.back', {time: fmtInstant(new Date(Date.parse('2099-01-02T03:04:05Z')))})")
    assert wait_until(page, "!restartState.following", 15), "the page sees the server back"
    assert page.locator("#restart-text").inner_text() == back, "it says the server is back"
    assert len(posts) == 1 and calls["after"] >= 2 and not btn.is_disabled(), "one request, the button usable again"
    page.unroute("**/healthz")
    page.unroute("**/api/v1/restart")


def test_restart_lists_what_waits(page):
    """The settings of the settings file that wait for the next start are
    listed under the button, by name."""
    posts = []
    stub_restart(page, '{"available": true, "by": "systemd", "pending": ["MASSIMO_LISTEN", "MASSIMO_TLS"]}', posts)
    open_settings(page)
    page.locator("#restart-btn").wait_for(state="visible")
    want = page.evaluate("t('restart.pending', {names: 'MASSIMO_LISTEN, MASSIMO_TLS'})")
    assert wait_until(page, f"document.getElementById('restart-text').textContent === {json.dumps(want)}", 5), \
        "what applies at the next restart"
    page.unroute("**/api/v1/restart")


def test_sign_in_offers_the_key_after_an_open_server(page):
    """The sign-in dialog opens its API-key part once the page has seen
    the server open, since the owner made from MASSIMO_API_TOKEN has no
    password; before that it stays folded."""
    page.evaluate("askLogin(); null")
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the dialog opens"
    assert not page.evaluate("document.getElementById('login-token').closest('details').open"), "folded on a server that always asked"
    page.evaluate("document.getElementById('login-dialog').close()")
    body = json.dumps({"mode": "open", "user": None, "role": None, "capabilities": [], "setup": False})
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=body))
    page.evaluate("loadMe()")
    page.unroute("**/api/v1/me")
    page.evaluate("askLogin(); null")
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the dialog opens again"
    assert page.evaluate("document.getElementById('login-token').closest('details').open"), "the API-key part is open"
    page.evaluate("document.getElementById('login-dialog').close()")
