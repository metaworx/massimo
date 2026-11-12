# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Signing in, in a browser context of its own without the stored
token, as the operator run.sh creates (password op-pass). A page error
fails the test (the guard); the console is not watched, since the
page's first requests without a token are refused on purpose."""
from conftest import URL
from helpers import named_buttons, open_user_menu, show_view, sign_out, wait_until


def test_sign_in_and_out(new_context, case, watch, guard):
    """The sign-in dialog opens without a token and refuses a wrong
    password, and two submits send one sign-in; the operator signs in,
    sees the tiles and no System settings →
    Users, Roles or Cubes; the 👤 menu names the operator, and Sign out
    forgets the token and the Cube's data and brings the dialog back,
    with nothing of the page but the name and the version; signed in
    again, the page is back."""
    ctx = new_context()  # the case's viewport and locale, from browser_context_args
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in dialog opens without a token"
    named_buttons(page, "the sign-in dialog")
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("wrong")
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, "!document.getElementById('login-error').classList.contains('hidden')", 5), "a wrong password is refused in the dialog"
    assert page.evaluate("document.getElementById('login-dialog').open"), "the dialog stays open"
    logins = []
    page.on("request", lambda r: logins.append(r.url) if r.method == "POST" and r.url.endswith(("/api/v1/login", "/api/v1/login/start")) else None)
    with page.expect_response(lambda r: r.url.endswith("/api/v1/login")):
        page.evaluate("() => { const f = document.getElementById('login-form'); f.requestSubmit(); f.requestSubmit(); }")
    page.wait_for_timeout(300)
    # A wrong password: a start by OPAQUE that it does not open, then the
    # password sign-in, while the server takes that.
    assert len(logins) == 2, f"two submits send one sign-in, not {logins}"
    page.locator("#login-password").fill("op-pass")
    page.locator("#login-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert page.evaluate("state.me && state.me.user && state.me.user.name") == "operator", "signed in as the operator"
    assert page.evaluate("document.body.classList.contains('no-users-admin') && document.body.classList.contains('no-rename')"), \
        "the missing capabilities are on the body"
    assert page.locator(".room .tile-cog").first.is_visible() and not page.locator(".room .menu-delete").first.is_visible(), \
        "an operator configures but does not delete rooms"
    show_view(page, "system")
    assert not page.locator(".users-settings").is_visible(), "no System settings → Users for an operator"
    assert not page.locator(".roles-settings").is_visible(), "no System settings → Roles for an operator"
    assert not page.locator(".cubes-settings.needs-cubes").is_visible(), "no System settings → Cubes for an operator"
    assert not page.locator(".restart.needs-restart").is_visible(), "no restart for an operator"
    named_buttons(page, "the settings as an operator")
    open_user_menu(page)
    assert "operator" in page.locator("#me-line").inner_text(), "the 👤 menu says who is signed in"
    assert page.locator("#logout-btn").inner_text() == SIGN_OUT[case.lang], "a session is signed out"
    page.locator("#logout-btn").click()
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "Sign out brings the dialog back"
    assert page.evaluate("localStorage.getItem('massimo.token')") is None, "the token is forgotten"
    # Signed out, the page shows its name, its version and the sign-in,
    # and holds nothing of the session.
    assert page.evaluate("state.rooms.length === 0 && state.cubes.length === 0 && !state.pollTimer"), "the Cube's data is forgotten"
    assert page.locator(".room").count() == 0, "no room tile is drawn"
    assert page.locator(".brand h1").is_visible() and page.locator("#version-short").is_visible(), "the name and the version show"
    for sel in ("#cube-menu", ".top-line > .nav", ".updated-line", "#clock-plain", "main"):
        assert not page.locator(sel).first.is_visible(), f"{sel} is hidden while signed out"
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("op-pass")
    page.locator("#login-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert page.locator("#cube-menu").is_visible() and page.locator(".top-line > .nav").is_visible(), "signed in, the header is back"
    assert page.evaluate("!!state.pollTimer"), "the poll runs again"


SIGN_OUT = {"en": "Sign out", "de": "Abmelden"}


def sign_in(page, user, password):
    """The dialog's sign-in, once the dialog is open; the tiles shown."""
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in dialog opens"
    page.locator("#login-user").fill(user)
    page.locator("#login-password").fill(password)
    page.locator("#login-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the stream is open"
    page.wait_for_timeout(1000)  # a second start would have asked by now


def test_sign_in_starts_the_page_once(new_context, case, watch, guard):
    """A sign-in that the page's start waited for, or Log out's, lets
    that start go on, and starts nothing beside it: one stream ticket and
    one read of who is in per sign-in."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    tickets, mes = [], []
    page.on("request", lambda r: tickets.append(r.url) if r.method == "POST" and r.url.endswith("/events/ticket") else None)
    page.on("response", lambda r: mes.append(r.url) if r.request.method == "GET" and r.url.endswith("/api/v1/me") and r.ok else None)
    page.goto(URL)
    sign_in(page, "operator", "op-pass")
    assert (len(tickets), len(mes)) == (1, 1), f"one start after the sign-in at the page's start: {len(tickets)} ticket(s), {len(mes)} /me"
    sign_out(page)
    sign_in(page, "operator", "op-pass")
    assert (len(tickets), len(mes)) == (2, 2), f"one start after the sign-in after Log out: {len(tickets)} ticket(s), {len(mes)} /me"
    sign_out(page)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "signed out at the end"


def test_sign_in_is_busy_until_answered(new_context, case, watch, guard):
    """Sign in is off with a spinner from its click until the sign-in is
    done, and a second Enter sends nothing. The test holds the sign-in's
    first request, the start by OPAQUE, then lets it go."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    held, posts = [], []

    def handle(route):
        posts.append(route.request.url)
        held.append(route)

    page.route("**/api/v1/login/start", handle)
    page.goto(URL)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in dialog opens"
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("op-pass")
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, "document.querySelector('#login-form button[type=submit]').classList.contains('busy')", 5), "Sign in turns"
    state = page.evaluate("(() => { const b = document.querySelector('#login-form button[type=submit]'); "
                          "return [b.disabled, b.getAttribute('aria-busy')]; })()")
    assert state == [True, "true"], f"Sign in is off and aria-busy while the request runs: {state}"
    page.locator("#login-password").press("Enter")
    page.wait_for_timeout(300)
    assert len(posts) == 1, f"one sign-in started while the first is unanswered: {len(posts)}"
    for route in held:
        route.continue_()
    page.unroute("**/api/v1/login/start")
    page.wait_for_selector(".room", timeout=10000)
    assert page.evaluate("state.me && state.me.user && state.me.user.name") == "operator", "signed in as the operator"
    assert not page.evaluate("document.querySelector('#login-form button[type=submit]').classList.contains('busy')"), \
        "Sign in stops turning once the answer is in"
    sign_out(page)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "signed out at the end"
