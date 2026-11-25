# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Signing in, in a browser context of its own without the stored
token, as the operator run.sh creates (password op-password). A page error
fails the test (the guard); the console is not watched, since the
page's first requests without a token are refused on purpose."""
import json
import urllib.error

import pytest
from playwright.sync_api import TimeoutError as PlaywrightTimeoutError

from conftest import TOKEN, URL, api_call
from helpers import (OPERATOR_PASSWORD, ApiReads, assert_login_card_centred, assert_login_row, frames, named_buttons,
                     open_personal_page, pause_clock, scroll_settled, show_view, sign_out, wait_held, wait_until)
from test_connect import server_page


def test_sign_in_and_out(new_context, case, watch, guard):
    """The sign-in dialog opens without a token, in the middle of the
    window, its title centred, and refuses a wrong
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
    # A card in the page, not a modal dialog, whose form a password
    # manager fills: one password field, the fields named.
    assert not page.evaluate("document.getElementById('login-dialog').matches(':modal')"), "the sign-in is not modal"
    assert page.evaluate("document.querySelectorAll('#login-form input[type=password]').length") == 1, "one password field in the form"
    assert page.evaluate("[document.getElementById('login-user').name, document.getElementById('login-password').name]") == ["username", "password"], \
        "the fields are named"
    box = page.locator("#login-dialog").bounding_box()
    vw = page.viewport_size["width"]
    assert box["x"] >= 0 and box["x"] + box["width"] <= vw, "the card fits the page's width"
    assert_login_card_centred(page, "the sign-in card")
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
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
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
    open_personal_page(page)
    assert "operator" in page.locator("#me-line").inner_text(), "the personal page says who is signed in"
    assert page.locator("#logout-btn").inner_text() == SIGN_OUT[case.lang], "a session is signed out"
    page.locator("#logout-btn").click()
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "Sign out brings the dialog back"
    assert page.evaluate("localStorage.getItem('massimo.token')") is None, "the token is forgotten"
    # Signed out, the page shows the sign-in's card alone, with the logo
    # and the version, and holds nothing of the session (AP MobileApp,
    # D44-A: no header).
    assert page.evaluate("state.rooms.length === 0 && state.cubes.length === 0 && !state.pollTimer"), "the Cube's data is forgotten"
    assert page.locator(".room").count() == 0, "no room tile is drawn"
    assert page.locator("header.top").is_hidden(), "no header while signed out"
    assert page.locator("#login-dialog > .brand-big").is_visible(), "the card's logo"
    version = page.locator("#version-short").text_content()
    assert version and page.locator("#login-version").inner_text() == version, "the version at the card's foot, as ☰ holds it"
    page.locator("#login-menu summary").click()
    assert page.locator("#login-menu-version").inner_text() == version, "and first in the card's ☰"
    page.locator("#login-menu summary").click()
    assert_login_card_centred(page, "the card after Sign out")
    for sel in ("#cube-menu", ".top-line > .nav", ".pile", "main"):
        assert not page.locator(sel).first.is_visible(), f"{sel} is hidden while signed out"
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
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
    sign_in(page, "operator", OPERATOR_PASSWORD)
    assert (len(tickets), len(mes)) == (1, 1), f"one start after the sign-in at the page's start: {len(tickets)} ticket(s), {len(mes)} /me"
    sign_out(page)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    assert (len(tickets), len(mes)) == (2, 2), f"one start after the sign-in after Log out: {len(tickets)} ticket(s), {len(mes)} /me"
    sign_out(page)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "signed out at the end"


def test_no_poll_behind_the_sign_in_card(new_context, case, watch, guard):
    """The page's poll waits for the start's sign-in: with the card open
    past two poll intervals, under the paused clock, the page reads
    neither the Cubes nor the log's alerts, which the server would refuse;
    once the operator has signed in, the page's start reads each once, and
    the poll reads the Cubes again at its interval (AP LooseEnds, Block 3).
    Fails on the page before: the start started the poll before the card,
    which read both at each interval behind it."""
    ctx = new_context()
    ctx.clock.install()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    reads = []
    page.on("request", lambda r: reads.append(r.url.split("?")[0]) if r.method == "GET" else None)

    def count(end):
        return sum(1 for u in reads if u.endswith(end))

    page.goto(URL)
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in dialog opens"
    pause_clock(page)
    try:
        every = page.evaluate("state.settings.poll") * 1000
        page.clock.run_for(2 * every + 1000)
        page.locator("#login-user").fill("operator")
        page.locator("#login-password").fill(OPERATOR_PASSWORD)
        page.locator("#login-form button[type=submit]").click()
        page.wait_for_selector(".room", timeout=10000)
        assert wait_until(page, "state.eventsOpen", 10), "the page started"
        counts = (count("/api/v1/cubes"), count("/api/v1/log/alerts"))
        assert counts == (1, 1), f"the Cubes and the alerts read once, by the start, none behind the card: {counts}"
        with page.expect_request(lambda r: r.method == "GET" and r.url.split("?")[0].endswith("/api/v1/cubes"), timeout=5000):
            page.clock.run_for(every)
        assert count("/api/v1/cubes") == 2, "the poll reads the Cubes once the card has closed"
    finally:
        page.clock.resume()
    sign_out(page)
    assert wait_until(page, DIALOG_OPEN, 5), "signed out at the end"


SIGN_IN_WITH = {"en": "Sign in with", "de": "Anmelden mit"}
PASSWORD = {"en": "Password", "de": "Passwort"}


def test_sign_in_ways(new_context, case, watch, guard):
    """The card's row "Password | API key ☰" (AP MobileApp, D45-A, D55-A,
    Block 14): on one line, the card's ☰ at its end, the group named
    "Sign in with" for a screen reader and no such label in view;
    Password chosen at first, the API key's form in its place once
    chosen, its field in focus, and Password back; opened on the API
    key's form, the focus in its field; no fingerprint and no Cancel
    outside the app."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in dialog opens"
    password = page.locator("#login-ways .login-way[data-way='password']")
    key = page.locator("#login-ways .login-way[data-way='key']")
    assert password.inner_text() == PASSWORD[case.lang], "Password, not Name"
    assert page.locator("#login-ways").get_attribute("role") == "group"
    assert page.locator("#login-ways").get_attribute("aria-label") == SIGN_IN_WITH[case.lang], "the group named for a screen reader"
    assert page.get_by_text(SIGN_IN_WITH[case.lang], exact=True).count() == 0, "no label Sign in with in view"
    assert_login_row(page, "the sign-in in a browser")
    assert password.get_attribute("aria-pressed") == "true" and key.get_attribute("aria-pressed") == "false", "Password is chosen at first"
    assert page.locator("#login-form").is_visible() and page.locator("#login-key-form").is_hidden()
    assert page.locator("#login-way-fp").is_hidden(), "no fingerprint in a browser"
    assert page.locator("#login-cancel").is_hidden(), "no Cancel in a browser"
    key.click()
    assert key.get_attribute("aria-pressed") == "true" and password.get_attribute("aria-pressed") == "false"
    assert page.locator("#login-key-form").is_visible() and page.locator("#login-form").is_hidden(), "the API key's form in its place"
    assert page.evaluate("document.activeElement.id") == "login-token", "its field in focus"
    assert page.locator("#login-key-cancel").is_hidden(), "nor on the API key's form"
    named_buttons(page, "the API key's form")
    password.click()
    assert page.locator("#login-form").is_visible() and page.locator("#login-key-form").is_hidden(), "Password back"
    assert page.evaluate("document.activeElement.id") == "login-user"
    # The card opened on the API key's form, as where the server was seen
    # open, puts the focus into its field: in with the key, then out.
    key.click()
    page.locator("#login-token").fill(TOKEN)
    page.locator("#login-key-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    page.evaluate("() => { sawOpen = true; }")
    sign_out(page)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "Forget key brings the card back"
    assert page.locator("#login-key-form").is_visible() and page.locator("#login-form").is_hidden(), "on the API key's form"
    assert wait_until(page, "document.activeElement && document.activeElement.id === 'login-token'", 3), "its field in focus"


THIS_DEVICE = {"en": "this device", "de": "dieses Gerät"}


def test_a_dead_session_is_dropped(new_context, case, watch, guard):
    """My profile marks the session in use "this device"; revoked from
    elsewhere, the page's first refused request drops it, so that its
    polls and stream do not go on sending it until the address is locked
    out (AP MobileApp, Block 13: F2, F7)."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    tid = page.evaluate("state.me.token.id")
    open_personal_page(page)
    row = page.locator("#profile-tokens .user-tokens li.token-current")
    row.wait_for(timeout=5000)
    assert row.get_attribute("data-id") == str(tid), "this device is the session in use"
    assert THIS_DEVICE[case.lang] in row.inner_text(), "named so"
    assert page.locator("#profile-tokens .user-tokens li.token-current").count() == 1, "once"
    dead = "Bearer " + page.evaluate("state.token")
    sent = []
    page.on("request", lambda r: sent.append(r.url) if r.headers.get("authorization") == dead else None)
    api_call("DELETE", "api/v1/users/operator/tokens/%d" % tid)
    page.evaluate("() => { poll(false); }")  # not awaited: it waits for the sign-in
    assert wait_until(page, "document.getElementById('login-dialog').open", 10), "the sign-in again"
    assert page.evaluate("state.token") is None, "the dead session is dropped"
    n = len(sent)
    page.wait_for_timeout(3000)
    assert len(sent) == n and n <= 3, f"no request sends it again: {n} at the refusal, {len(sent)} after"


def sessions_of(user):
    """The ids of the user's live sessions at the server."""
    return {t["id"] for t in api_call("GET", f"api/v1/users/{user}/tokens") or [] if t.get("kind") == "session"}


def test_a_replaced_session_ends(new_context, case, watch, guard):
    """A sign-in that replaces the session the page holds, the card opened
    on a running page as the open server's banner or ☰ open it: the session
    replaced is ended at the server (POST /api/v1/logout with it, AP
    MobileApp, D68-A), and the new one alone is left of the two. A key
    replaced is not sent there, which would answer 400."""
    page = new_context().new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    first, old = page.evaluate("[state.me.token.id, state.token]")
    logouts = []
    page.on("request", lambda r: logouts.append(r.headers.get("authorization")) if r.method == "POST" and r.url.endswith("/api/v1/logout") else None)
    page.evaluate("() => { askLogin(); }")
    sign_in(page, "operator", OPERATOR_PASSWORD)
    second = page.evaluate("state.me.token.id")
    assert second != first, "a new session"
    for _ in range(25):
        if logouts and first not in sessions_of("operator"):
            break
        page.wait_for_timeout(200)
    assert logouts == ["Bearer " + old], f"the replaced session is signed out, with its own token: {len(logouts)} logout(s)"
    left = sessions_of("operator")
    assert first not in left and second in left, f"the replaced session ended at the server, the new one live: {first}, {second} in {left}"
    sign_out(page)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "signed out"
    logouts.clear()
    page.locator("#login-ways .login-way[data-way='key']").click()
    page.locator("#login-token").fill(TOKEN)
    page.locator("#login-key-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert page.evaluate("state.me.token.kind") == "api", "in with the key"
    page.evaluate("() => { askLogin(); }")
    sign_in(page, "operator", OPERATOR_PASSWORD)
    assert page.evaluate("state.me.user.name") == "operator", "signed in over the key"
    assert logouts == [], f"a key replaced is not signed out: {len(logouts)} logout(s)"
    sign_out(page)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "signed out at the end"


def test_a_dead_session_leaves_another_tabs_token(new_context, case, watch, guard):
    """Two tabs of one browser signed in with one session: the second
    signs in again on its running page, which stores its new session and
    ends the first at the server (D68-A); the first tab's next request
    meets the 401 and asks for a sign-in, but leaves the stored token, the
    second tab's, as it is."""
    ctx = new_context()
    first = ctx.new_page()
    guard.page = first
    first.set_default_timeout(5000)
    first.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    first.goto(URL)
    sign_in(first, "operator", OPERATOR_PASSWORD)
    old, old_id = first.evaluate("[state.token, state.me.token.id]")
    second = ctx.new_page()
    second.set_default_timeout(5000)
    second.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    second.goto(URL)
    second.wait_for_selector(".room", timeout=10000)
    assert second.evaluate("state.token") == old, "the second tab starts with the stored session"
    second.evaluate("() => { askLogin(); }")
    sign_in(second, "operator", OPERATOR_PASSWORD)
    new = second.evaluate("state.token")
    assert new != old and first.evaluate("localStorage.getItem('massimo.token')") == new, "the new session stored"
    for _ in range(25):  # the second tab's logout of the replaced session, not awaited
        if old_id not in sessions_of("operator"):
            break
        first.wait_for_timeout(200)
    assert old_id not in sessions_of("operator"), "the replaced session ended at the server"
    first.evaluate("() => { poll(false); }")  # not awaited: it waits for the sign-in
    assert wait_until(first, "document.getElementById('login-dialog').open", 10), "the first tab's session ended: its sign-in"
    assert first.evaluate("state.token") is None, "the first tab drops the dead session"
    assert first.evaluate("localStorage.getItem('massimo.token')") == new, "and leaves the second tab's stored token"
    guard.page = second
    sign_out(second)
    assert wait_until(second, "document.getElementById('login-dialog').open", 5), "signed out at the end"


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
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
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


WRONG = {"en": "Wrong name or password.", "de": "Falscher Name oder falsches Passwort."}
DIALOG_OPEN = "document.getElementById('login-dialog').open"


def test_a_wrong_password_in_the_pages_language(new_context, case, watch, guard):
    """A wrong password is said in the case's language, not with the
    server's English words. Fails on the page before: "Nicht angemeldet:
    wrong user or password" (the walk-through of AP LooseEnds, finding 2).
    And Escape leaves no sign-in the server asks for: a forward guard, as
    Escape did nothing in the card before (AP LooseEnds, D3)."""
    page = new_context().new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in dialog opens"
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("wrong")
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, "!document.getElementById('login-error').classList.contains('hidden')", 5), "a wrong password is refused"
    said = page.locator("#login-error").inner_text()
    assert said == WRONG[case.lang], f"said in the page's language: {said!r}"
    page.keyboard.press("Escape")
    frames(page)
    assert page.evaluate(DIALOG_OPEN), "Escape leaves no sign-in the server asks for"


def test_a_keys_second_enter_signs_in_once(new_context, case, watch, guard):
    """Two Enters in the API key's form, the card opened on a running page
    as a refused request opens it: Sign in turns while the key signs in,
    and the sign-in's reads of who is in go once each, the check of who
    signed in (settleSignIn) and the page's read after it (afterLogin):
    two, counted once the page has started again. The test holds the
    first. Fails on the page before: no busy state, and each Enter settled
    the sign-in, three reads."""
    page = new_context().new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    page.evaluate("() => { askLogin(); }")
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens on the running page"
    page.locator("#login-ways .login-way[data-way='key']").click()
    page.locator("#login-token").fill(TOKEN)
    held, mes, tickets = [], [], []

    def me(route):
        if route.request.method != "GET":
            return route.continue_()
        mes.append(route.request.url)
        if len(mes) == 1:
            held.append(route)
            return
        route.continue_()

    page.route("**/api/v1/me", me)
    page.on("request", lambda r: tickets.append(r.url) if r.method == "POST" and r.url.endswith("/events/ticket") else None)
    reads = ApiReads(page)
    page.evaluate("() => { const f = document.getElementById('login-key-form'); f.requestSubmit(); f.requestSubmit(); }")
    assert wait_held(page, held), "the check of who signed in"
    submit = page.locator("#login-key-form button[type=submit]")
    assert submit.evaluate("(b) => b.classList.contains('busy') && b.disabled && b.getAttribute('aria-busy') === 'true'"), \
        "Sign in turns while the key signs in"
    held.pop().continue_()
    assert wait_until(page, f"!({DIALOG_OPEN})", 10), "signed in with the key"
    assert wait_held(page, tickets, 10), "the page started again: its stream's ticket"
    assert reads.answered(), f"the page's reads answered: {reads.open}"
    assert len(mes) == 2, f"one check of who signed in and one read after it: {len(mes)} reads of /api/v1/me"
    assert page.evaluate("state.me.token.kind") == "api", "in with the key"
    assert not submit.evaluate("(b) => b.classList.contains('busy')"), "Sign in stops turning"
    page.unroute("**/api/v1/me")
    sign_out(page)
    assert wait_until(page, DIALOG_OPEN, 5), "signed out at the end"


KEY_REFUSED = {"en": "The server does not know this API key.", "de": "Der Server kennt diesen API-Schlüssel nicht."}
KEY_ERROR = "!!document.getElementById('login-key-error') && !document.getElementById('login-key-error').classList.contains('hidden')"
WRONG_KEY = "not-a-key-of-this-server"


def on_the_keys_way(page, what):
    """The card open on the API key's way, its form shown."""
    assert page.evaluate(DIALOG_OPEN), f"{what}: the card stays open"
    assert page.locator("#login-ways .login-way[data-way='key']").get_attribute("aria-pressed") == "true", f"{what}: on the key's way"
    assert page.locator("#login-key-form").is_visible() and page.locator("#login-form").is_hidden(), f"{what}: the key's form shown"


def key_refused(page, case, what):
    """The card refused the key: open on the API key's way, the refusal in
    the page's language in its line, the field empty and in focus."""
    assert wait_until(page, f"{KEY_ERROR} || !document.getElementById('login-dialog').open || "
                            "document.getElementById('login-ways').querySelector('[data-way=key]').getAttribute('aria-pressed') !== 'true'", 5), \
        f"{what}: the key's sign-in ended"
    on_the_keys_way(page, what)
    assert wait_until(page, KEY_ERROR, 5), f"{what}: the refusal said"
    said = page.locator("#login-key-error").inner_text()
    assert said == KEY_REFUSED[case.lang], f"{what}: said in the page's language: {said!r}"
    assert page.locator("#login-token").input_value() == "", f"{what}: the refused key is gone from its field"
    assert page.evaluate("document.activeElement && document.activeElement.id") == "login-token", f"{what}: the focus in the key's field"


def send_key(page, key):
    """The API key's way chosen and key sent."""
    page.locator("#login-ways .login-way[data-way='key']").click()
    page.locator("#login-token").fill(key)
    page.locator("#login-key-form button[type=submit]").click()


def read_with(key):
    """A response of GET /api/v1/me read with key as its bearer."""
    return lambda r: r.request.method == "GET" and r.url.split("?")[0].endswith("/api/v1/me") and r.request.headers.get("authorization") == "Bearer " + key


def new_page(new_context, guard, watch):
    page = new_context().new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    return page


def colour(page, sel):
    return page.evaluate(f"getComputedStyle(document.querySelector({sel!r})).color")


def test_a_refused_key_is_said(new_context, case, watch, guard):
    """A key the server does not know (401) is said in the sign-in card,
    on the key's way, in the page's language and in the colour of the
    password's refusal, the field empty and in focus, and nothing of it is
    kept: no token in the page or in the browser's storage. The right key
    then signs in once: one stream ticket, and two reads of who is in, the
    card's try and the start's read that waited for the card. Fails on the
    page before: the card opened anew on the password's way, without a
    word (AP LooseEnds, D43)."""
    page = new_page(new_context, guard, watch)
    page.goto(URL)
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in dialog opens"
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("wrong")
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, "!document.getElementById('login-error').classList.contains('hidden')", 5), "a wrong password refused"
    refused = colour(page, "#login-error")
    send_key(page, WRONG_KEY)
    key_refused(page, case, "a wrong key at the start")
    assert colour(page, "#login-key-error") == refused, f"in the password's refusal's colour: {colour(page, '#login-key-error')} vs {refused}"
    assert page.evaluate("[state.token, localStorage.getItem('massimo.token')]") == [None, None], "no token left of the refused key"
    mes, tickets = [], []
    page.on("request", lambda r: mes.append(r.url) if r.method == "GET" and r.url.split("?")[0].endswith("/api/v1/me") else None)
    page.on("request", lambda r: tickets.append(r.url) if r.method == "POST" and r.url.endswith("/events/ticket") else None)
    reads = ApiReads(page)
    page.locator("#login-token").fill(TOKEN)
    page.locator("#login-key-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the page started"
    assert reads.answered(), f"the page's reads answered: {reads.open}"
    assert page.evaluate("state.me.token.kind") == "api", "in with the right key"
    assert (len(tickets), len(mes)) == (1, 2), f"signed in once: {len(tickets)} ticket(s), {len(mes)} reads of /api/v1/me"
    assert page.locator("#login-key-error").is_hidden(), "the refusal's line gone"
    sign_out(page)
    assert wait_until(page, DIALOG_OPEN, 5), "signed out at the end"


ANSWERS = {
    "403": (403, "the user operator is disabled", True),
    "429": (429, "too many failed sign-ins from this address; try again later", False),
    "500": (500, "the store did not answer", False),
    "unreached": (None, None, False),
}


@pytest.mark.parametrize("answer", list(ANSWERS))
def test_a_keys_other_answers_are_said(new_context, case, watch, guard, answer):
    """The key's try answered otherwise than 401: a key refused for a
    reason (403, a user or an integration disabled) is said in the
    server's words and goes from its field; a lockout (429), a server
    error (500) and a server not reached are said in their words and the
    key stays for the retry. The card stays on the key's way, the focus
    in the field, and no token is kept. Fails on the page before: nothing
    was said, the card turned to the password's way."""
    status, error, emptied = ANSWERS[answer]
    key = "answer-" + answer
    page = new_page(new_context, guard, watch)

    def me(route):
        if route.request.headers.get("authorization") != "Bearer " + key:
            return route.continue_()
        if status is None:
            return route.abort("connectionrefused")
        route.fulfill(status=status, content_type="application/json", body=json.dumps({"error": error}))

    page.route("**/api/v1/me", me)
    page.goto(URL)
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in dialog opens"
    send_key(page, key)
    assert wait_until(page, f"{KEY_ERROR} || document.getElementById('login-ways').querySelector('[data-way=key]').getAttribute('aria-pressed') !== 'true'", 5), \
        "the key's sign-in ended"
    on_the_keys_way(page, answer)
    assert wait_until(page, KEY_ERROR, 5), f"{answer}: the answer said"
    said = page.locator("#login-key-error").inner_text()
    if error:
        assert said == page.evaluate("(e) => t('login.failed', { error: e })", error), f"{answer}: in the server's words: {said!r}"
    else:
        assert said.startswith(page.evaluate("t('login.failed', { error: '' })")) and len(said) > len(page.evaluate("t('login.failed', { error: '' })")), \
            f"{answer}: in the browser's words: {said!r}"
    assert page.locator("#login-token").input_value() == ("" if emptied else key), f"{answer}: the key's field {'emptied' if emptied else 'kept'}"
    assert page.evaluate("document.activeElement && document.activeElement.id") == "login-token", f"{answer}: the focus in the key's field"
    assert page.evaluate("[state.token, localStorage.getItem('massimo.token')]") == [None, None], f"{answer}: no token kept"


def test_a_refused_key_leaves_the_session(new_context, case, watch, guard):
    """A wrong key typed in the card opened on a running page, as a
    refused request opens it, costs the user signed in nothing: once the
    key's read is answered the page's session is the same and still live
    at the server, the refusal said, and the card left, the page goes on
    as that user. Fails on the page before: the page took the wrong key as
    its token, ended the session at the server and forgot the user."""
    page = new_page(new_context, guard, watch)
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    old, tid = page.evaluate("[state.token, state.me.token.id]")
    page.evaluate("() => { askLogin(); }")
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens on the running page"
    with page.expect_response(read_with(WRONG_KEY)):
        send_key(page, WRONG_KEY)
    frames(page)
    assert page.evaluate("[state.token, localStorage.getItem('massimo.token')]") == [old, old], "the session is still the page's token"
    assert tid in sessions_of("operator"), "and still live at the server"
    assert page.evaluate("state.me && state.me.user && state.me.user.name") == "operator", "the user still in"
    key_refused(page, case, "a wrong key on a running page")
    page.evaluate("() => { document.getElementById('login-dialog').close(); }")
    assert wait_until(page, f"!({DIALOG_OPEN})", 5), "the card left"
    page.evaluate("() => { loadMe(); }")
    assert wait_until(page, "!!state.me && !!state.me.user && state.me.user.name === 'operator'", 5), "the page goes on as the operator"
    sign_out(page)
    assert wait_until(page, DIALOG_OPEN, 5), "signed out at the end"


def test_the_app_keeps_no_refused_key(new_context, case, watch, guard):
    """In the app, a refused key is not handed to the app's bridge to keep
    (setToken), and the bridge holds no token once the key's read is
    answered; the right key then is kept. Fails on the page before: the
    bridge kept the refused key."""
    page = server_page(new_context, guard, watch, {"status": {"can": False, "user": "", "ask": "start", "plain": False}})
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in card in the app"
    with page.expect_response(read_with(WRONG_KEY)):
        send_key(page, WRONG_KEY)
    frames(page)
    assert page.evaluate("window.__fakeBridge.calls.filter((c) => c[0] === 'setToken' && c[1]).length") == 0, "the refused key never handed to it"
    assert page.evaluate("window.__fakeBridge.token") is None, "the bridge keeps no token"
    key_refused(page, case, "a wrong key in the app")
    page.locator("#login-token").fill(TOKEN)
    page.locator("#login-key-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert page.evaluate("window.__fakeBridge.token") == TOKEN, "the right key kept"


def test_a_late_key_answer_makes_no_token(new_context, case, watch, guard):
    """The key's read held while the password's way signs in: the key's
    answer, let go after that sign-in, makes the key no token; the page
    keeps the password's session, in the page and in the browser's
    storage. Fails where submitKey's answer is not checked by still(): the
    late answer made the key the page's token over the session. On a page
    where the key is not read with first (before AP LooseEnds, D43) it
    fails sooner, at the hold: the key sends no read of its own."""
    page = new_page(new_context, guard, watch)
    held = []
    page.route("**/api/v1/me", lambda route: held.append(route) if route.request.headers.get("authorization") == "Bearer " + TOKEN and not held
               else route.continue_())
    page.goto(URL)
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in dialog opens"
    send_key(page, TOKEN)
    assert wait_held(page, held), "the key's read, held"
    page.locator("#login-ways .login-way[data-way='password']").click()
    sign_in(page, "operator", OPERATOR_PASSWORD)
    session = page.evaluate("state.token")
    assert page.evaluate("state.me.token.kind") == "session", "in with the password"
    with page.expect_response(read_with(TOKEN)):
        held.pop().continue_()
    frames(page)
    assert page.evaluate("[state.token, localStorage.getItem('massimo.token')]") == [session, session], "the session stays the page's token"
    assert page.evaluate("state.me.token.kind") == "session", "and who is in"
    page.unroute("**/api/v1/me")
    sign_out(page)
    assert wait_until(page, DIALOG_OPEN, 5), "signed out at the end"


def test_a_late_key_refusal_writes_nothing(new_context, case, watch, guard):
    """A wrong key's read held, the card closed and opened anew, a key
    typed there on the key's way and the focus on its way's button, a
    control the key's field is shown beside: the refusal, let go then,
    writes nothing into the new opening, neither its line, nor the key's
    field, nor the focus. Fails on the page before: the late refusal said
    itself in the new opening, emptied its field and took the focus into
    it."""
    page = new_page(new_context, guard, watch)
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    held = []
    page.route("**/api/v1/me", lambda route: held.append(route) if route.request.headers.get("authorization") == "Bearer " + WRONG_KEY
               else route.continue_())
    page.evaluate("() => { askLogin(); }")
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens on the running page"
    send_key(page, WRONG_KEY)
    assert wait_held(page, held), "the wrong key's read, held"
    page.evaluate("() => { document.getElementById('login-dialog').close(); }")
    # the dialog's close event, which ends its opening, comes in a task of its own
    assert wait_until(page, f"!({DIALOG_OPEN}) && loginPromise === null", 5), "the card left, its opening ended"
    page.evaluate("() => { askLogin(); }")
    assert wait_until(page, DIALOG_OPEN, 5), "the card opened anew"
    way = page.locator("#login-ways .login-way[data-way='key']")
    way.click()
    page.locator("#login-token").fill("typed-in-the-new-opening")
    way.focus()
    with page.expect_response(read_with(WRONG_KEY)):
        held.pop().continue_()
    frames(page)
    assert not page.evaluate(KEY_ERROR), "no refusal said in the new opening"
    assert page.locator("#login-token").input_value() == "typed-in-the-new-opening", "its key's field as typed"
    assert page.locator("#login-token").is_visible(), "the key's field shown, which a focus asked for would take"
    assert page.evaluate("document.activeElement && document.activeElement.dataset.way") == "key", "the focus where the user put it"
    assert way.get_attribute("aria-pressed") == "true", "on the way the user chose"
    page.unroute("**/api/v1/me")
    page.evaluate("() => { document.getElementById('login-dialog').close(); }")
    sign_out(page)
    assert wait_until(page, DIALOG_OPEN, 5), "signed out at the end"


def status_with(token):
    """The status the server answers GET /api/v1/me with, token its bearer."""
    try:
        api_call("GET", "api/v1/me", token=token)
    except urllib.error.HTTPError as e:
        return e.code
    return 200


def test_a_late_password_answer_makes_no_token(new_context, case, watch, guard):
    """The password's sign-in held at its finish while the API key signs
    in: the password's session, let go once the key is in, makes no token
    and is ended at the server (POST /api/v1/logout with it, AP AccessEnds,
    D2), so that a request with it answers 401; the page keeps the key, in
    the page and in the browser's storage, who is in is the key's, and the
    operator's live sessions are those before the test. Fails on the page
    before: the late session became the page's token over the key, and
    stayed live at the server."""
    page = new_page(new_context, guard, watch)
    before = sessions_of("operator")
    held = []
    page.route("**/api/v1/login/finish", lambda route: held.append(route) if route.request.method == "POST" and not held else route.continue_())
    page.goto(URL)
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in dialog opens"
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
    page.locator("#login-form button[type=submit]").click()
    assert wait_held(page, held, 10), "the password's finish, held"
    send_key(page, TOKEN)
    page.wait_for_selector(".room", timeout=10000)
    assert page.evaluate("state.me.token.kind") == "api", "in with the key"
    logout = None
    try:
        with page.expect_response(lambda r: r.request.method == "POST" and r.url.endswith("/api/v1/logout")) as ended:
            with page.expect_response(lambda r: r.request.method == "POST" and r.url.endswith("/api/v1/login/finish")) as finished:
                held.pop().continue_()
        logout = ended.value
    except PlaywrightTimeoutError:
        pass  # said below
    late = finished.value.json()["token"]
    frames(page)
    assert page.evaluate("[state.token, localStorage.getItem('massimo.token')]") == [TOKEN, TOKEN], "the key stays the page's token"
    assert page.evaluate("state.me.token.kind") == "api", "and who is in"
    assert logout is not None and logout.request.headers.get("authorization") == "Bearer " + late, "the late session signed out, with its own token"
    assert status_with(late) == 401, "a request with the late session's token answers 401"
    assert sessions_of("operator") == before, f"the operator's live sessions those before the test: {sessions_of('operator')} vs {before}"
    page.unroute("**/api/v1/login/finish")
    sign_out(page)
    assert wait_until(page, DIALOG_OPEN, 5), "signed out at the end"


SUBMIT = "#login-form button[type=submit]"
READY = f"(() => {{ const b = document.querySelector({SUBMIT!r}); return !b.disabled && !b.classList.contains('busy'); }})()"


def test_a_late_password_refusal_writes_nothing(new_context, case, watch, guard):
    """A wrong password's sign-in held at its start on a running page, the
    card closed and opened anew, a name typed there and the focus on it:
    Sign in is ready again in the new opening, and the refusal, let go
    then, writes nothing into it, neither its line nor the focus, which
    stays in the name (AP AccessEnds, Block 1). Fails on the page before:
    Sign in turned on in the new opening until the old answer came, and
    the late refusal said the wrong password there and took the focus into
    Password."""
    page = new_page(new_context, guard, watch)
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    held = []
    page.route("**/api/v1/login/start", lambda route: held.append(route) if not held else route.continue_())
    page.evaluate("() => { askLogin(); }")
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens on the running page"
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("wrong")
    page.locator(SUBMIT).click()
    assert wait_held(page, held), "the wrong password's start, held"
    page.evaluate("() => { document.getElementById('login-dialog').close(); }")
    # the dialog's close event, which ends its opening, comes in a task of its own
    assert wait_until(page, f"!({DIALOG_OPEN}) && loginPromise === null", 5), "the card left, its opening ended"
    page.evaluate("() => { askLogin(); }")
    assert wait_until(page, DIALOG_OPEN, 5), "the card opened anew"
    frames(page)
    assert page.evaluate(READY), "Sign in ready in the new opening, the old sign-in's answer still out"
    page.locator("#login-user").fill("typed-in-the-new-opening")
    page.locator("#login-user").focus()
    # the start answered, then the password's refusal, after argon2id has turned in the page
    with page.expect_response(lambda r: r.request.method == "POST" and r.url.endswith(("/api/v1/login", "/api/v1/login/finish")), timeout=10000):
        held.pop().continue_()
    frames(page)
    assert page.locator("#login-error").is_hidden(), "no refusal said in the new opening"
    assert page.evaluate("document.activeElement && document.activeElement.id") == "login-user", "the focus where the user put it"
    assert page.locator("#login-user").input_value() == "typed-in-the-new-opening", "the name as typed"
    page.unroute("**/api/v1/login/start")
    page.evaluate("() => { document.getElementById('login-dialog').close(); }")
    sign_out(page)
    assert wait_until(page, DIALOG_OPEN, 5), "signed out at the end"


def test_a_password_answer_after_a_sign_out_does_nothing(new_context, case, watch, guard):
    """The card opened on a running page, the password's sign-in held at
    its finish and the page signed out meanwhile (logout, which leaves the
    card open on the same opening): the session, let go then, is no token
    of the page's and is ended at the server, the card stays open and
    says nothing, and nobody is in (AP AccessEnds, D2). Fails where still()
    does not ask the session's number: the late session signed the page
    in again after the sign-out."""
    page = new_page(new_context, guard, watch)
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    held = []
    page.route("**/api/v1/login/finish", lambda route: held.append(route) if route.request.method == "POST" and not held else route.continue_())
    page.evaluate("() => { askLogin(); }")
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens on the running page"
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
    page.locator(SUBMIT).click()
    assert wait_held(page, held, 10), "the password's finish, held"
    page.evaluate("() => { logout(); }")  # not awaited: its start waits for the card
    assert wait_until(page, "state.token === null && state.me === null", 5), "signed out behind the card"
    logout = None
    try:
        with page.expect_response(lambda r: r.request.method == "POST" and r.url.endswith("/api/v1/logout")) as ended:
            with page.expect_response(lambda r: r.request.method == "POST" and r.url.endswith("/api/v1/login/finish")) as finished:
                held.pop().continue_()
        logout = ended.value
    except PlaywrightTimeoutError:
        pass  # said below
    late = finished.value.json()["token"]
    frames(page)
    assert page.evaluate("[state.token, localStorage.getItem('massimo.token')]") == [None, None], "the late session is no token of the page's"
    assert page.evaluate(DIALOG_OPEN) and page.locator("#login-error").is_hidden(), "the card stays open, saying nothing"
    assert logout is not None and logout.request.headers.get("authorization") == "Bearer " + late, "the late session signed out, with its own token"
    assert status_with(late) == 401, "a request with the late session's token answers 401"
    page.unroute("**/api/v1/login/finish")


def test_an_answer_after_another_way_set_its_token_makes_none(new_context, case, watch, guard):
    """On a running page the card opened, the API key's read held and the
    password signed in, its check of who signed in held too: the key's
    answer, let go while the password's sign-in has its token but has not
    closed the card, makes no token, and the page goes on with the
    password's session, which stays live (AP LooseEnds, D44). Fails where
    a way's token leaves the opening to the others: the key's answer set
    its token over the session, which was left live and unheld."""
    page = new_page(new_context, guard, watch)
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    first = page.evaluate("state.token")
    stage = {"settle": False}
    keys, settles = [], []

    def me(route):
        bearer = route.request.headers.get("authorization", "")
        if route.request.method == "GET" and bearer == "Bearer " + TOKEN and not keys:
            keys.append(route)
            return
        if route.request.method == "GET" and stage["settle"] and bearer not in ("Bearer " + TOKEN, "Bearer " + first):
            stage["settle"] = False
            settles.append(route)
            return
        route.continue_()

    page.route("**/api/v1/me", me)
    page.evaluate("() => { askLogin(); }")
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens on the running page"
    send_key(page, TOKEN)
    assert wait_held(page, keys), "the key's read, held"
    page.locator("#login-ways .login-way[data-way='password']").click()
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
    stage["settle"] = True
    page.locator(SUBMIT).click()
    assert wait_held(page, settles, 10), "the password's sign-in has its token, its check of who signed in held"
    session = page.evaluate("state.token")
    assert session not in (first, TOKEN), "the password's session is the page's token"
    with page.expect_response(read_with(TOKEN)):
        keys.pop().continue_()
    frames(page)
    assert page.evaluate("state.token") == session, "the key's answer sets no token over the session"
    settles.pop().continue_()
    assert wait_until(page, f"!({DIALOG_OPEN})", 10), "the password's sign-in closes the card"
    page.unroute("**/api/v1/me")
    assert wait_until(page, "!!state.me && !!state.me.user && state.me.user.name === 'operator'", 5), "the operator is in"
    assert page.evaluate("[state.token, localStorage.getItem('massimo.token')]") == [session, session], "with the password's session"
    assert status_with(session) == 200, "which is live at the server"
    sign_out(page)
    assert wait_until(page, DIALOG_OPEN, 5), "signed out at the end"


def held_me(page):
    """GET /api/v1/me held once the test asks (stage["hold"]), the one
    request after that; the others go through. Returns the stage and the
    list the held route lands in."""
    stage = {"hold": False}
    held = []

    def me(route):
        if stage["hold"] and route.request.method == "GET":
            stage["hold"] = False
            held.append(route)
            return
        route.continue_()

    page.route("**/api/v1/me", me)
    return stage, held


def test_the_card_empties_its_secrets_after_a_sign_in(new_context, case, watch, guard):
    """A refused sign-in keeps what was typed for the retry. Once the card
    has closed on a sign-in, the password and the API key stay while the
    page's first read after it is held, and go once it has settled: both
    empty, the password hidden, the name kept (AP LooseEnds, D13, D41-A).
    A card opened again while that read is held, typed in and left keeps
    what was typed once the read has settled and the page has started:
    nothing empties a field late. Fails on the page before: the fields
    kept the secrets until the next opening."""
    page = new_context().new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    stage, held = held_me(page)
    page.goto(URL)
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in dialog opens"
    page.locator("#login-ways .login-way[data-way='key']").click()
    page.locator("#login-token").fill("typed-key")
    page.locator("#login-ways .login-way[data-way='password']").click()
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("wrong")
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, "!document.getElementById('login-error').classList.contains('hidden')", 5), "refused"
    frames(page)
    assert page.locator("#login-password").input_value() == "wrong" and page.locator("#login-token").input_value() == "typed-key", \
        "a refusal keeps what was typed"
    eye = page.locator("#login-form .pw-eye")
    eye.wait_for(state="visible")
    eye.click()
    assert page.locator("#login-password").get_attribute("type") == "text", "the password shown"
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
    stage["hold"] = True
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, f"!({DIALOG_OPEN})", 10), "signed in, the card closed"
    assert wait_held(page, held), "the page's first read after the sign-in, held"
    frames(page)
    assert page.locator("#login-password").input_value() == OPERATOR_PASSWORD and page.locator("#login-token").input_value() == "typed-key", \
        "kept while the read after the sign-in has not settled"
    held.pop().continue_()
    assert wait_until(page, "document.getElementById('login-password').value === '' && document.getElementById('login-token').value === ''", 10), \
        "the password and the key emptied once it has"
    assert page.locator("#login-password").get_attribute("type") == "password", "the password hidden"
    assert page.locator("#login-user").input_value() == "operator", "the name kept"
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the page started"
    # a card opened again while the read after a sign-in is held, typed in and left
    sign_out(page)
    assert wait_until(page, DIALOG_OPEN, 5), "the card after Sign out"
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
    stage["hold"] = True
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, f"!({DIALOG_OPEN})", 10), "signed in again"
    assert wait_held(page, held), "the read after it, held"
    page.evaluate("() => { askLogin(); }")
    assert wait_until(page, DIALOG_OPEN, 5), "the card opened again"
    assert page.locator("#login-password").input_value() == "", "emptied at the opening"
    page.locator("#login-password").fill("typed-later")
    page.evaluate("() => { document.getElementById('login-dialog').close(); }")
    assert wait_until(page, f"!({DIALOG_OPEN})", 5), "and left"
    reads = ApiReads(page)
    held.pop().continue_()
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the page started"
    assert reads.answered(), f"the page's reads answered: {reads.open}"
    frames(page)
    assert page.locator("#login-password").input_value() == "typed-later", "what was typed after the close stays"
    page.unroute("**/api/v1/me")
    sign_out(page)
    assert wait_until(page, DIALOG_OPEN, 5), "signed out at the end"
    assert page.locator("#login-password").input_value() == "", "emptied at the opening, as before"


TITLE = {"en": "Sign in", "de": "Anmelden"}


def test_sign_in_menu(new_context, case, watch, guard):
    """Signed out, the sign-in card's ☰ sets the language and the mode:
    the card speaks the language chosen, and My settings' choices show
    both, kept in this browser."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in card"
    menu = page.locator("#login-menu")
    assert menu.locator("summary").get_attribute("aria-label"), "the ☰ is labelled"
    menu.locator("summary").click()
    assert page.locator("#login-lang").is_visible() and page.locator("#login-mode").is_visible(), "the menu shows both choices"
    named_buttons(page, "the sign-in's menu")
    other = "de" if case.lang == "en" else "en"
    page.locator("#login-lang").select_option(other)
    assert wait_until(page, f"document.getElementById('login-title').textContent === {TITLE[other]!r}", 5), "the card speaks the language chosen"
    assert page.evaluate("[document.getElementById('lang-select').value, localStorage.getItem('massimo.lang')]") == [other, other], \
        "My settings' language is the one chosen"
    page.locator("#login-mode").select_option("dark")
    assert page.evaluate("[state.settings.theme, document.getElementById('theme-select').value]") == ["dark", "dark"], \
        "My settings' mode is the one chosen"
    box = page.locator("#login-menu .menu-items").bounding_box()
    assert box["x"] >= 0 and box["x"] + box["width"] <= page.viewport_size["width"], "the menu fits the page"
    page.keyboard.press("Escape")
    assert menu.get_attribute("open") is None, "Escape closes the menu"
    ctx.close()


def test_profile_marks_a_key(new_context, case, watch, guard):
    """My profile opened at #profile?key=<id>, as the app opens it for a
    key of the phone's it could not revoke (AP MobileApp, D57-A): that
    key's row marked, it alone, and brought from below the first screen
    into the middle of the window, the key dropped from the address; a
    key named by a change of the address in the page that runs is marked
    too. Back and forward to the entries, whose addresses name no key any
    more, a change to an address that names none, and Personal settings
    opened again from ☰ mark nothing; nor does an id of no API key of the
    user's, or one that is no id (the guard: no page error). The operator
    gets keys enough for the list to run past the window, listed in the
    order made, removed at the end."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    ids = []
    try:
        for i in range(24):
            ids.append(api_call("POST", "api/v1/users/operator/tokens", {"label": f"marked test {i}"})["id"])
        mine = page.evaluate("state.me.token.id")
        last = ids[-1]
        at = ctx.new_page()
        guard.page = at
        at.set_default_timeout(5000)
        at.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))

        def opened(address):
            """A fresh load of the page at the address, My profile's keys drawn."""
            at.goto("about:blank")
            at.goto(URL + address)
            at.locator(f"#profile-tokens li[data-id='{last}']").wait_for(timeout=5000)

        def marked(tid):
            """The key's row alone marked, tinted and outlined, the key gone
            from the address, and the row in the middle of the window once
            the scroll rests, unless the page can scroll no further."""
            assert wait_until(at, f"!!document.querySelector(\"#profile-tokens li.token-marked[data-id='{tid}']\")", 5), f"the key {tid} is marked"
            assert at.locator("#profile-tokens li.token-marked").count() == 1, "it alone"
            assert at.evaluate("location.hash") == "#profile", "the key dropped from the address"
            assert scroll_settled(at), "the scroll comes to rest"
            m = at.evaluate("""(id) => {
              const r = document.querySelector(`#profile-tokens li[data-id='${id}']`).getBoundingClientRect();
              return {mid: (r.top + r.bottom) / 2, height: r.height, vh: innerHeight, y: scrollY,
                      max: document.documentElement.scrollHeight - innerHeight};
            }""", tid)
            assert abs(m["mid"] - m["vh"] / 2) <= m["height"] or m["y"] >= m["max"] - 1, f"the row brought into the middle of the window: {m}"
            row = at.locator(f"#profile-tokens li[data-id='{tid}']")
            colours = row.evaluate("(e) => [getComputedStyle(e).backgroundColor, getComputedStyle(e).boxShadow]")
            assert colours[0] not in ("rgba(0, 0, 0, 0)", "transparent") and colours[1] != "none", f"tinted and outlined: {colours}"

        def redrawn_unmarked(change, what):
            """change reads My profile's keys again, which its part draws in
            place (AP UiParts, Block 13); then none is marked."""
            with at.expect_response(lambda r: r.request.method == "GET" and r.url.endswith("/api/v1/users/operator/tokens"), timeout=5000) as read:
                change()
            read.value.finished()
            frames(at)
            assert at.locator(f"#profile-tokens li[data-id='{last}']").count() == 1, f"the keys drawn again: {what}"
            assert at.locator("#profile-tokens li.token-marked").count() == 0, f"nothing marked: {what}"

        # without the mark, the last key's row starts below the first screen
        opened("#profile")
        assert scroll_settled(at), "the scroll comes to rest"
        assert at.evaluate(f"""(() => {{ const r = document.querySelector("#profile-tokens li[data-id='{last}']");
            return r.getBoundingClientRect().top + scrollY > innerHeight; }})()"""), "the last key's row below the first screen"
        assert at.locator("#profile-tokens li.token-marked").count() == 0, "and not marked"
        # opened at its address, as the app opens it
        opened(f"#profile?key={last}")
        marked(last)
        # another key named in the page that runs
        at.evaluate(f"() => {{ location.hash = '#profile?key={ids[1]}'; }}")
        marked(ids[1])
        # back and forward: the entries read #profile now
        redrawn_unmarked(lambda: at.go_back(), "back")
        redrawn_unmarked(lambda: at.go_forward(), "forward")
        # an address that names no key
        at.evaluate(f"() => {{ location.hash = '#profile?key={ids[2]}'; }}")
        marked(ids[2])
        redrawn_unmarked(lambda: at.evaluate("() => { location.hash = '#profile?key=abc'; }"), "key=abc")
        # Personal settings opened again from ☰, after another page
        at.evaluate(f"() => {{ location.hash = '#profile?key={ids[3]}'; }}")
        marked(ids[3])
        show_view(at, "tiles")
        redrawn_unmarked(lambda: open_personal_page(at), "Personal settings from ☰")
        # no key: not a number, not the user's, a session's, nothing
        for bad in ("abc", "0", "-3", "1e3", "", "99999999", str(mine)):
            opened(f"#profile?key={bad}")
            at.wait_for_timeout(300)
            assert at.locator("#profile-tokens li.token-marked").count() == 0, f"key={bad!r} marks nothing"
        at.close()
        guard.page = page
    finally:
        for tid in ids:
            api_call("DELETE", "api/v1/users/operator/tokens/%d" % tid)
    sign_out(page)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "signed out at the end"
