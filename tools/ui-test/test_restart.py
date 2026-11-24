# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings → Software: the server's restart. GET and POST
/api/v1/restart are stubbed, and /healthz once the restart is asked for,
so the test server never restarts."""
import json

import pytest

from conftest import TOKEN, api_call
from helpers import pause_clock, show_view, wait_held, wait_until

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
    show_view(page, "maint")
    assert wait_until(page, "!document.getElementById('restart-text').classList.contains('hidden')", 5), "the reason is shown"
    assert "no service manager" in page.locator("#restart-text").inner_text(), "the server's reason"
    assert page.locator("#restart-btn").is_hidden(), "no button"
    page.unroute("**/api/v1/restart")


def test_restart_asks_in_the_page(page):
    """The button asks in System settings → Maintenance, and Cancel takes
    the question back without a request."""
    posts = []
    stub_restart(page, AVAILABLE, posts)
    show_view(page, "maint")
    btn = page.locator("#restart-btn")
    btn.wait_for(state="visible")
    want = page.evaluate("t('maint.restartText')")
    assert page.locator("#restart-text").inner_text() == want, "what a restart does"
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
    show_view(page, "maint")
    btn = page.locator("#restart-btn")
    btn.wait_for(state="visible")
    btn.click()
    page.locator(".restart .confirm-yes").click()
    restarting = page.evaluate("t('restart.restarting')")
    assert wait_until(page, "document.getElementById('restart-text').textContent === %s" % json.dumps(restarting), 5), \
        "the page follows the restart, and says the server restarts"
    assert page.locator("#restart-btn").is_disabled(), "no second restart meanwhile"
    back = page.evaluate("t('restart.back', {time: fmtInstant(new Date(Date.parse('2099-01-02T03:04:05Z')))})")
    assert wait_until(page, "document.getElementById('restart-text').textContent === %s" % json.dumps(back), 15), \
        "the page sees the server back, and says so"
    assert len(posts) == 1 and calls["after"] >= 2 and not btn.is_disabled(), "one request, the button usable again"
    page.unroute("**/healthz")
    page.unroute("**/api/v1/restart")


@pytest.mark.clock
def test_restart_lists_what_waits(page):
    """The settings of the settings file that wait for the next start are
    listed under the restart, by name, and counted in the card's badge,
    which the card's head shows in the same turn as the list. The
    restart's read is held until the rest of Maintenance is drawn, so
    that its answer is the last thing drawn, and the clock is paused, so
    that no frame of the page's comes between the list and the read of
    the head: a badge given a frame later, as it was, is not there yet
    (CI, 2026-11-21, read the head empty)."""
    answer = '{"available": true, "by": "systemd", "pending": ["MASSIMO_LISTEN", "MASSIMO_TLS"]}'
    held, let_go = [], []

    def restart(route):
        """The first GET held until the test lets it go, the others answered."""
        if held or route.request.method != "GET":
            route.fulfill(status=200, content_type="application/json", body=answer)
        else:
            held.append(route)
    page.route("**/api/v1/restart", restart)
    pause_clock(page)
    try:
        show_view(page, "maint")
        assert wait_held(page, held), "Maintenance reads the restart"
        assert wait_until(page, "document.getElementById('interval-poll-text').textContent.length > 0", 5), "the rest of it drawn"
        let_go.append(1)
        held[0].fulfill(status=200, content_type="application/json", body=answer)
        want = page.evaluate("t('restart.pending', {names: 'MASSIMO_LISTEN, MASSIMO_TLS'})")
        assert wait_until(page, f"document.getElementById('restart-pending').textContent === {json.dumps(want)}", 5), \
            "what applies at the next restart"
        badge = page.evaluate("tn('maint.badge.pending', 2)")
        assert page.locator("#sys-maint .section-badges").inner_text() == badge, "counted in the card's badge, in the same turn"
    finally:
        if held and not let_go:
            held[0].fulfill(status=200, content_type="application/json", body=answer)
        page.clock.resume()
        page.unroute("**/api/v1/restart")


def test_reload_settings(page):
    """Maintenance names the settings file and when it was read; Reload
    now asks the server to read it again (POST /api/v1/reload, stubbed:
    the route has its Go test) and says what changed."""
    posts = []
    stub_restart(page, '{"available": true, "by": "systemd", "file": "/etc/massimo/env", "read_at": "2026-11-08T19:52:00Z"}', posts)
    reloads = []

    def reload(route):
        reloads.append(route.request.method)
        route.fulfill(status=200, content_type="application/json",
                      body='{"changed": ["MASSIMO_LISTEN"], "pending": ["MASSIMO_LISTEN"], "read_at": "2026-11-08T20:00:00Z"}')
    page.route("**/api/v1/reload", reload)
    try:
        show_view(page, "maint")
        assert wait_until(page, "document.getElementById('settings-file-text').textContent.includes('/etc/massimo/env')", 5), \
            "the settings file named, with when it was read"
        page.locator("#settings-reload").click()
        want = page.evaluate("t('maint.reloaded', {names: 'MASSIMO_LISTEN'})")
        assert wait_until(page, f"document.getElementById('settings-reload-msg').textContent === {json.dumps(want)}", 5), \
            "what changed"
        assert reloads == ["POST"], f"one POST /api/v1/reload ({reloads})"
    finally:
        page.unroute("**/api/v1/reload")
        page.unroute("**/api/v1/restart")


# What the last read of the settings file found the next start refuses
# (AP ReloadChecks, D8-B): a setting with markup in its value, and two
# settings of one check with a path whose segment, without a hyphen or a
# space, is wider than a phone and wraps only anywhere.
REFUSED = [
    {"settings": ["MASSIMO_TRUSTED_PROXIES"],
     "error": '-trusted-proxies or MASSIMO_TRUSTED_PROXIES: "<b>10.0.0.300</b><img src=x>" is neither an address nor a CIDR'},
    {"settings": ["MASSIMO_TLS", "MASSIMO_TLS_CERT"],
     "error": "tls: open /var/lib/massimo/certificates/adirectorynamefartoolongforanyphonescreenwhatsoevertowrapwithoutbreaking/cert.pem: "
              "no such file or directory"},
]


def test_reload_shows_what_the_next_start_refuses(page):
    """Under the restart, beside what waits for the next start, the card
    says what that start refuses, a line each, the server's text as text:
    no element of its markup, nothing wider than the page. The restart is
    still offered. Reload now reads the file again (POST /api/v1/reload,
    stubbed: the route has its Go test), and once the server finds
    nothing refused, the restart's row no longer says it."""
    answer = {"body": {"available": True, "by": "systemd", "pending": ["MASSIMO_TLS", "MASSIMO_TLS_CERT", "MASSIMO_TRUSTED_PROXIES"],
                       "file": "/etc/massimo/env", "read_at": "2026-11-08T19:52:00Z", "refused": REFUSED}}

    def restart(route):
        route.fulfill(status=200, content_type="application/json", body=json.dumps(answer["body"]))

    def reload(route):
        """The file fixed: the TLS lines gone, the proxy a good one, which still waits for the next start."""
        good = {"available": True, "by": "systemd", "pending": ["MASSIMO_TRUSTED_PROXIES"], "file": "/etc/massimo/env",
                "read_at": "2026-11-08T20:00:00Z"}
        answer["body"] = good
        route.fulfill(status=200, content_type="application/json", body=json.dumps({
            "changed": ["MASSIMO_TLS", "MASSIMO_TLS_CERT", "MASSIMO_TRUSTED_PROXIES"], "pending": good["pending"], "refused": [],
            "read_at": good["read_at"]}))
    page.route("**/api/v1/restart", restart)
    page.route("**/api/v1/reload", reload)
    try:
        show_view(page, "maint")
        notice = page.locator("#restart-refused")
        assert wait_until(page, "document.querySelectorAll('#restart-refused .restart-refusal').length === 2", 5), "a line each"
        want = page.evaluate("""(refused) => [t('restart.refused')].concat(refused.map((f) =>
          t('restart.refusal', { names: f.settings.join(', '), error: f.error })))""", REFUSED)
        got = page.evaluate("""() => { const n = document.getElementById('restart-refused');
          return [n.firstChild.textContent].concat([...n.querySelectorAll('.restart-refusal')].map((s) => s.textContent)); }""")
        assert got == want, f"what the next start refuses, as text ({got})"
        assert notice.is_visible() and page.locator("#restart-refused b, #restart-refused img").count() == 0, "no markup of the server's text"
        assert page.evaluate("""() => { const n = document.getElementById('restart-refused');
          return n.previousElementSibling.id === 'restart-pending' && n.scrollWidth <= n.clientWidth; }"""), \
            "beside what waits for the next start, wrapped within the card"
        assert page.evaluate("document.documentElement.scrollWidth <= window.innerWidth"), "nothing wider than the page"
        assert page.locator("#restart-btn").is_visible() and page.locator("#restart-btn").is_enabled(), "the restart still offered"
        page.locator("#settings-reload").click()
        assert wait_until(page, "document.getElementById('restart-refused').classList.contains('hidden')", 5), "gone after a good reload"
        assert page.locator("#restart-refused .restart-refusal").count() == 0, "no line left"
        names = page.evaluate("t('restart.pending', {names: 'MASSIMO_TRUSTED_PROXIES'})")
        assert page.locator("#restart-pending").inner_text() == names, "what still waits"
    finally:
        page.unroute("**/api/v1/reload")
        page.unroute("**/api/v1/restart")


def test_sign_in_offers_the_key_after_a_server_without_a_store(page):
    """The sign-in dialog chooses its API key's form once the page has
    seen a server without a store ask for no key, since the only way in
    there is MASSIMO_API_TOKEN; a server seen open with a store keeps the
    name's form, its owner made with a password by the first owner's
    form (AP FirstOwner, D42); before either the name's form. The open
    server's stub says the caller is not on the local network, so that
    no first owner's card opens."""
    def seen(mode):
        body = json.dumps({"mode": mode, "user": None, "role": None, "capabilities": [], "setup": False})
        page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=body))
        page.evaluate("loadMe()")
        page.unroute("**/api/v1/me")
        page.evaluate("askLogin(); null")
        assert wait_until(page, "document.getElementById('login-dialog').open", 5), f"the dialog opens after mode {mode}"

    def close():
        page.evaluate("document.getElementById('login-dialog').close()")
        # askLogin() hands out the open dialog's promise until its close event ran.
        assert wait_until(page, "loginPromise === null", 5), "the dialog's close handled"

    page.evaluate("askLogin(); null")
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the dialog opens"
    assert page.locator("#login-form").is_visible() and page.locator("#login-key-form").is_hidden(), "the name's form on a server that always asked"
    close()
    seen("open")
    assert page.locator("#login-form").is_visible() and page.locator("#login-key-form").is_hidden(), "the name's form after an open server"
    close()
    seen("none")
    assert page.locator("#login-key-form").is_visible() and page.locator("#login-form").is_hidden(), "the API key's form is chosen"
    close()


def test_store_and_settings_in_effect(page):
    """System Info, for a holder of settings, at its end: the store with
    its schema and rows per table, and the settings in effect, the API
    token listed as set without its value; neither in Maintenance."""
    show_view(page, "info")
    assert page.locator("#sys-info #store-info").count() == 1 and page.locator("#sys-info #config-info").count() == 1, \
        "both in System Info"
    assert page.locator("#sys-maint #store-info, #sys-maint #config-info").count() == 0, "not in Maintenance"
    assert page.locator("#system #sys-info").count() == 0, "nor System Info in System settings"
    last = page.evaluate("[...document.querySelectorAll('#sys-info-body > *')].slice(-2).map((e) => e.id)")
    assert last == ["store-info", "config-info"], f"at the end of System Info ({last})"
    assert wait_until(page, "document.getElementById('store-info-sum').textContent.includes('massimo.db')", 5), \
        "the store's file in the head of its part"
    page.locator("#store-info summary").click()
    assert page.locator("#store-info-body .kv-rows li").count() >= 5, "the rows per table"
    page.locator("#config-info summary").click()
    body = page.locator("#config-info-body").inner_text()
    secret = page.evaluate("t('maint.secretSet')")
    assert "MASSIMO_API_TOKEN" in body and secret in body, "the token listed as set"
    assert TOKEN and TOKEN not in page.content(), "its value nowhere in the page"
    page.locator("#store-info summary").click()
    page.locator("#config-info summary").click()
