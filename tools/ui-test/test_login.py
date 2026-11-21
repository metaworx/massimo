# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Signing in, in a browser context of its own without the stored
token, as the operator run.sh creates (password op-pass). A page error
fails the test (the guard); the console is not watched, since the
page's first requests without a token are refused on purpose."""
from conftest import TOKEN, URL, api_call
from helpers import (assert_login_card_centred, assert_login_row, named_buttons, open_personal_page, scroll_settled, show_view, sign_out,
                     wait_until)


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
    sign_in(page, "operator", "op-pass")
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
    sign_in(page, "operator", "op-pass")
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
            """change draws My profile's keys again; then none is marked."""
            at.evaluate("() => { document.querySelector('#profile-tokens .user-tokens').dataset.old = '1'; }")
            change()
            assert wait_until(at, """(() => { const ul = document.querySelector('#profile-tokens .user-tokens');
                return !!ul && !ul.dataset.old && !!ul.querySelector('li[data-id]'); })()""", 5), f"the keys drawn again: {what}"
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
