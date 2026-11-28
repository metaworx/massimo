# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The sign-in as parts (AP UiParts, Block 14): what a user holds on the
first owner's form and on the sign-in's card stays through everything
that draws them again, and each button that sends sends once. The app's
side runs on fake_bridge.js, as in test_connect.py; the console is not
watched on a page without a token, whose first requests are refused on
purpose (test_login.py)."""
import json

import pytest
from playwright.sync_api import TimeoutError as PlaywrightTimeoutError

from conftest import TOKEN, URL, api_call
from helpers import LOGIN_BOX, OPERATOR_PASSWORD, frames, same_box, sign_out, wait_held, wait_until
from test_connect import server_page
from test_login import status_with

OPEN_SETUP = json.dumps({"mode": "open", "user": None, "role": None, "capabilities": [], "setup": True})


def test_create_the_owner_stays_busy_while_the_app_keeps(new_context, case, watch, guard):
    """In the app, Create the owner with the switch on stays busy until
    the app has answered whether it kept the login by fingerprint (UiParts
    D213), and then until the sign-in card that holds the form has closed
    (AP FirstOwner): a second tap while the system's prompt is open
    creates nothing, where it posted the owner again and said the name was
    taken. The second submit is the form's own (requestSubmit, as Enter
    on another form's field would be): Enter on a field whose default
    button is off never submits, so a key press would try the button's
    disabled attribute and not the part's guard. A second creation would
    be sent at once, the creation being stubbed: the posts are counted
    once the first has ended. The owner's creation and sign-in are stubbed
    (POST /api/v1/users, and the page's registerPassword and signIn), so
    that the test server keeps its users; /api/v1/me says an open server
    without users, which opens the card on the running page, and the
    server's own answer once the owner is made."""
    page = server_page(new_context, guard, watch, {"status": {"can": True, "user": "", "ask": "start", "plain": False}}, token=TOKEN)
    page.wait_for_selector(".room", timeout=10000)
    posts = []
    stage = {"open": True}

    def users(route):
        if route.request.method != "POST":
            return route.continue_()
        posts.append(route.request.url)
        stage["open"] = False
        route.fulfill(status=201, content_type="application/json", body="{}")

    def me(route):
        if not stage["open"]:
            return route.continue_()
        route.fulfill(status=200, content_type="application/json", body=OPEN_SETUP)

    page.route("**/api/v1/users", users)
    page.route("**/api/v1/me", me)
    try:
        page.evaluate("""() => {
          registerPassword = async () => ({ record: "r", ksf: "m=19456,t=2,p=1" });
          signIn = async () => ({ token: %s });
          const login = window.massimo.login;
          login.keep = () => new Promise((resolve) => { window.__letKeepGo = () => resolve(true); });
          loadMe();
        }""" % json.dumps(TOKEN))
        page.locator("#setup-keep-row").wait_for(state="visible")
        page.locator("#setup-user").fill(f"owner-{case.width}-{case.lang}")
        page.locator("#setup-password").fill("own-secret-1")
        page.locator("#setup-again").fill("own-secret-1")
        page.locator("#setup-keep-row .slider").click()
        submit = page.locator("#setup-form button[type=submit]")
        submit.click()
        assert wait_until(page, "typeof window.__letKeepGo === 'function'", 5), "the app is asked to keep the login"
        assert len(posts) == 1, f"the owner is created once: {posts}"
        assert submit.evaluate("(b) => b.classList.contains('busy') && b.disabled"), "Create the owner turns while the app keeps the login"
        page.evaluate("() => { document.getElementById('setup-form').requestSubmit(); }")
        page.evaluate("() => { window.__letKeepGo(); }")
        assert wait_until(page, "!document.getElementById('login-dialog').open && "
                                "!document.querySelector('#setup-form button[type=submit]').classList.contains('busy')", 5), \
            "the card closes, and Create the owner stops turning"
        assert len(posts) == 1, f"a second submit while the app keeps the login creates nothing: {posts}"
        assert page.evaluate("document.getElementById('toast-body').textContent === t('setup.done', {name: %s})"
                             % json.dumps(f"owner-{case.width}-{case.lang}")), \
            page.evaluate("document.getElementById('toast-body').textContent")
    finally:
        page.unroute("**/api/v1/me")
        page.unroute("**/api/v1/users")


MARK = "(ids) => { for (const id of ids) document.getElementById(id).__held = true; }"
SAME = "(ids) => ids.filter((id) => !document.getElementById(id) || !document.getElementById(id).__held)"
HELD = ["login-user", "login-password", "login-form", "login-way-fp", "login-menu"]


def open_card(new_context, guard, watch):
    """A page without a token, its sign-in's card open."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in's card"
    return page


def test_the_card_keeps_what_is_typed(new_context, case, watch, guard):
    """A name and a password typed, the focus in the password and a
    refusal shown stay, the same elements, through everything that may
    draw the card again while it is open: a second 401's askLogin, which
    hands out the same opening, the providers read again (one arriving),
    a change of language and who is in read again (a 401 too). A
    forward guard: the plain card passed it, drawn again by nothing but an
    opening."""
    page = open_card(new_context, guard, watch)
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("wrong")
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, "!document.getElementById('login-error').classList.contains('hidden')", 5), "a wrong password is refused"
    said = page.locator("#login-error").inner_text()
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
    page.locator("#login-password").focus()
    page.evaluate(MARK, HELD)
    provider = json.dumps([{"key": "test-idp", "name": "Test IdP"}])
    page.route("**/api/v1/login/providers", lambda route: route.fulfill(status=200, content_type="application/json", body=provider))
    try:
        other = "de" if case.lang == "en" else "en"
        with page.expect_response(lambda r: r.url.endswith("/api/v1/me")):
            page.evaluate("async (other) => { askLogin(); await renderLoginProviders(); await changeLanguage(other); applyLanguage(); loadMe(); }", other)
        page.locator("#login-providers button[data-provider='test-idp']").wait_for(state="visible")
        assert wait_until(page, f"i18n.lang === {other!r} && document.getElementById('login-title').textContent === t('login.title')", 5), \
            "the card drawn again in the other language"
        frames(page)  # who is in's 401 taken and drawn
    finally:
        page.unroute("**/api/v1/login/providers")
    assert page.evaluate(SAME, HELD) == [], "the card's elements are the same"
    assert page.locator("#login-user").input_value() == "operator" and page.locator("#login-password").input_value() == OPERATOR_PASSWORD, \
        "what is typed stays"
    assert page.evaluate("document.activeElement.id") == "login-password", "the focus stays in the password"
    assert page.locator("#login-error").inner_text() == said, "the refusal stays said"
    assert page.locator("#login-ways .login-way[data-way='password']").get_attribute("aria-pressed") == "true", "the way stays chosen"
    page.locator("#login-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)


def test_the_fields_are_emptied_at_an_opening_and_after_a_success(new_context, case, watch, guard):
    """The password signed in with is gone once the card has closed on the
    sign-in and the reads after it, the name staying (AP LooseEnds, D13,
    which changes UiParts D210: the password stayed until the next opening), and both are gone
    at the next opening. A forward guard of the opening's: the plain card
    emptied the fields there too."""
    page = open_card(new_context, guard, watch)
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
    page.locator("#login-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert page.evaluate("!document.getElementById('login-dialog').open"), "signed in"
    assert wait_until(page, "document.getElementById('login-password').value === ''", 5), "the password emptied after the sign-in"
    assert page.locator("#login-user").input_value() == "operator", "the name kept until the next opening"
    sign_out(page)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the card again"
    assert wait_until(page, "document.getElementById('login-user').value === '' && document.getElementById('login-password').value === ''", 3), \
        "emptied at the opening"
    assert page.evaluate("document.activeElement.id") == "login-user", "the focus in the name"


def test_the_fingerprint_keeps_its_name_and_its_turn(new_context, case, watch, guard):
    """In the app with a login kept: the fingerprint's button keeps the
    user's name at a change of language, where the plain card's markup
    named it "Login by fingerprint" again; and while the system's prompt
    is open, the button stays the same element and turns through the
    app's status answering and the language changing, and the name typed
    and the way chosen stay. The prompt is held until the test lets it go,
    and answers dismissed."""
    page = server_page(new_context, guard, watch, {"status": {"can": True, "user": "operator", "ask": "start", "plain": False}, "fingerprint": None})
    fp = page.locator("#login-way-fp")
    fp.wait_for(state="visible")
    assert "operator" in fp.get_attribute("aria-label"), "named with the user's name"
    other = "de" if case.lang == "en" else "en"
    page.locator("#login-menu summary").click()
    page.locator("#login-lang").select_option(other)
    assert wait_until(page, f"i18n.lang === {other!r}", 5), "the language chosen"
    page.keyboard.press("Escape")
    assert "operator" in fp.get_attribute("aria-label") and "operator" in fp.get_attribute("title"), \
        f"still named with the user's name: {fp.get_attribute('aria-label')!r}"
    page.locator("#login-user").fill("operator")
    page.evaluate(MARK, HELD)
    page.evaluate("""() => { const login = window.massimo.login; const answer = login.fingerprint;
      login.fingerprint = () => new Promise((resolve) => { window.__letFingerprintGo = () => resolve(answer.call(login)); }); }""")
    fp.click()
    assert wait_until(page, "document.getElementById('login-way-fp').classList.contains('busy')", 3), "the button turns"
    page.evaluate("() => MassimoParts.signIn.readApp()")
    page.locator("#login-menu summary").click()
    page.locator("#login-lang").select_option("auto")
    assert wait_until(page, f"i18n.lang === {case.lang!r}", 5), "the language back"
    assert page.evaluate(SAME, HELD) == [], "the card's elements are the same"
    assert fp.evaluate("(b) => b.classList.contains('busy') && b.disabled"), "still turning while the prompt is open"
    assert page.locator("#login-user").input_value() == "operator", "the name typed stays"
    assert page.locator("#login-ways .login-way[data-way='password']").get_attribute("aria-pressed") == "true", "the way stays chosen"
    page.evaluate("() => { window.__letFingerprintGo(); }")
    assert wait_until(page, "!document.getElementById('login-way-fp').classList.contains('busy')", 3), "dismissed: it stops turning"
    assert page.evaluate("document.getElementById('login-dialog').open"), "and the card stays"


# the app's prompt held at each call until the test lets it go, then answered as the fake bridge answers
HOLD_FINGER = """() => { const login = window.massimo.login; const answer = login.fingerprint;
  login.fingerprint = () => new Promise((resolve, reject) => { window.__letFingerGo = () => answer.call(login).then(resolve, reject); }); }"""
FP_HELD = "typeof window.__letFingerGo === 'function'"
FP_DONE = "!document.getElementById('login-way-fp').classList.contains('busy')"
KEPT_LOGIN = {"status": {"can": True, "user": "operator", "ask": "start", "plain": False}}
# what is off while the card is working: both ways' Sign in, both Cancels, Select server
WORKING_OFF = ["#login-form button[type=submit]", "#login-key-form button[type=submit]", "#login-cancel", "#login-key-cancel", "#login-menu-servers"]
OFF_NOW = "(sels) => sels.filter((s) => !document.querySelector(s).disabled)"


def fingerprint_session(case):
    """A session of the operator's made from a key of its own (POST
    /api/v1/login/key), as the app's login by fingerprint makes one, and
    the key's id, whose removal ends it."""
    label = f"ui-test-fingerprint-{case.width}-{case.lang}"
    key = api_call("POST", "api/v1/users/operator/tokens", {"label": label})
    return api_call("POST", "api/v1/login/key", {"label": label}, token=key["token"])["token"], key["id"]


def logout_with(page, act):
    """The response of the POST /api/v1/logout that act leads to, None
    where none comes."""
    try:
        with page.expect_response(lambda r: r.request.method == "POST" and r.url.endswith("/api/v1/logout")) as ended:
            act()
        return ended.value
    except PlaywrightTimeoutError:
        return None


def test_the_fingerprint_keeps_the_card_working(new_context, case, watch, guard):
    """In the app with a login kept: while the password's sign-in runs,
    the fingerprint's button is off; from the fingerprint's press until
    its answer the card is working, as while the first owner is created:
    both ways' Sign in, both Cancels and Select server are off, and a
    submit of the password's form starts nothing. The finger then answers:
    the page signs in with the fingerprint's session, the page's token and
    the app's, live at the server (AP AccessEnds, D1, D2). Fails on the
    page before: the password's Sign in stayed on behind the prompt, where
    a sign-in by it would be ended by the app as it keeps the
    fingerprint's session."""
    fsession, kid = fingerprint_session(case)
    try:
        page = server_page(new_context, guard, watch, {**KEPT_LOGIN, "fingerprint": fsession})
        page.locator("#login-dialog[open]").wait_for()
        fp = page.locator("#login-way-fp")
        fp.wait_for(state="visible")
        held = []
        page.route("**/api/v1/login/start", lambda route: held.append(route))
        page.locator("#login-user").fill("operator")
        page.locator("#login-password").fill("wrong")
        page.locator("#login-form button[type=submit]").click()
        assert wait_held(page, held), "the password's start, held"
        assert fp.evaluate("(b) => b.disabled"), "the fingerprint off while the password signs in"
        with page.expect_response(lambda r: r.url.endswith("/api/v1/login/start")):
            held.pop().fulfill(status=503, content_type="application/json", body=json.dumps({"error": "not ready"}))
        assert wait_until(page, "!document.getElementById('login-way-fp').disabled", 3), "and on again once it was refused"
        page.unroute("**/api/v1/login/start")
        starts = []
        page.on("request", lambda r: starts.append(r.url) if r.url.endswith("/api/v1/login/start") else None)
        page.evaluate(HOLD_FINGER)
        fp.click()
        assert wait_until(page, FP_HELD, 3), "the prompt open, held"
        frames(page)
        assert page.evaluate(OFF_NOW, WORKING_OFF) == [], "the other ways and the ways out off while the prompt is open"
        page.locator("#login-password").fill(OPERATOR_PASSWORD)
        page.evaluate("() => { document.getElementById('login-form').requestSubmit(); }")
        frames(page)
        assert not page.evaluate("document.querySelector('#login-form button[type=submit]').classList.contains('busy')") and starts == [], \
            f"a submit of the password's form starts nothing: {starts}"
        page.evaluate("() => { window.__letFingerGo(); }")
        page.wait_for_selector(".room", timeout=10000)
        assert page.evaluate("[state.token, window.__fakeBridge.token]") == [fsession, fsession], "in with the fingerprint's session, the page's and the app's"
        assert status_with(fsession) == 200, "which is live at the server"
        assert page.evaluate(OFF_NOW, WORKING_OFF[:2]) == WORKING_OFF[:2], "the card's Sign in on again"
    finally:
        api_call("DELETE", "api/v1/users/operator/tokens/%d" % kid)


def test_a_late_fingerprint_makes_no_token(new_context, case, watch, guard):
    """In the app with a login kept, the card opened on a running page: a
    refusal of the fingerprint's prompt held while the card is closed and
    opened anew says nothing in the new opening, and the app's status is
    read again, its key being gone; and a prompt held while the page signs
    out behind the card (logout, which leaves the same opening open), then
    passed: its session, which the app keeps as setToken would, makes no
    token, is ended at the server with its own token and kept by the app
    no more (AP AccessEnds, D2). Fails on the page before: the late
    session became the page's token and the app's after the sign-out; and
    where the app was handed the page's empty token back, it forgot the
    session, left live at the server."""
    fsession, kid = fingerprint_session(case)
    try:
        page = server_page(new_context, guard, watch, {**KEPT_LOGIN, "fingerprint": "error:the key was refused"})
        page.locator("#login-dialog[open]").wait_for()
        fp = page.locator("#login-way-fp")
        fp.wait_for(state="visible")
        page.locator("#login-user").fill("operator")
        page.locator("#login-password").fill(OPERATOR_PASSWORD)
        page.locator("#login-form button[type=submit]").click()
        page.wait_for_selector(".room", timeout=10000)
        assert page.evaluate("state.me.token.kind") == "session", "in with the password"

        # a refusal let go in a later opening
        page.evaluate("() => { askLogin(); }")
        assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in opens on the running page"
        fp.wait_for(state="visible")
        page.evaluate(HOLD_FINGER)
        fp.click()
        assert wait_until(page, FP_HELD, 3), "the prompt open, held"
        page.evaluate("() => { document.getElementById('login-dialog').close(); }")
        assert wait_until(page, "!document.getElementById('login-dialog').open && loginPromise === null", 5), "the card left, its opening ended"
        page.evaluate("() => { askLogin(); }")
        assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the card opened anew"
        status = "window.__fakeBridge.calls.filter((c) => c[0] === 'login.status').length"
        assert wait_until(page, f"{status} > 0", 3)
        frames(page)
        n = page.evaluate(status)
        page.evaluate("() => { window.__letFingerGo(); }")
        assert wait_until(page, FP_DONE, 3), "the prompt refused, late"
        frames(page)
        assert page.locator("#login-error").is_hidden(), "no refusal said in the new opening"
        assert wait_until(page, f"{status} === {n + 1}", 3), f"the app's status read again: {page.evaluate(status)} reads, {n} before"

        # a session that comes once the page signed out behind the card
        page.evaluate("(t) => { window.__fakeBridgeConfig.login.fingerprint = t; window.__letFingerGo = null; }", fsession)
        assert wait_until(page, FP_DONE + " && !document.getElementById('login-way-fp').disabled", 3), "the fingerprint ready again"
        fp.click()
        assert wait_until(page, FP_HELD, 3), "the prompt open again, held"
        page.evaluate("() => { logout(); }")  # not awaited: its start waits for the card
        assert wait_until(page, "state.token === null && state.me === null", 5), "signed out behind the card"
        logout = logout_with(page, lambda: page.evaluate("() => { window.__letFingerGo(); }"))
        assert wait_until(page, FP_DONE, 3), "the prompt passed, late"
        frames(page)
        assert page.evaluate("[state.token, window.__fakeBridge.token]") == [None, None], "neither the page nor the app keeps the late session"
        assert logout is not None and logout.request.headers.get("authorization") == "Bearer " + fsession, "the late session signed out, with its own token"
        assert status_with(fsession) == 401, "a request with it answers 401"
        assert page.evaluate("document.getElementById('login-dialog').open") and page.locator("#login-error").is_hidden(), \
            "the card stays open, saying nothing"
    finally:
        api_call("DELETE", "api/v1/users/operator/tokens/%d" % kid)


WAY = "#login-ways .login-way[data-way='%s']"
IN_FIELD = "!!document.activeElement && ['INPUT', 'SELECT', 'TEXTAREA'].includes(document.activeElement.tagName)"
KEPT = {"status": {"can": True, "user": "operator", "ask": "start", "plain": False}}


def card_in(new_context, guard, watch, kept, **touch):
    """The sign-in's card open in a context of its own (touch, where
    given: has_touch and is_mobile), in the app with a kept login (the
    fingerprint's way shown) or in a browser."""
    ctx = lambda: new_context(**touch)  # noqa: E731, the context server_page makes
    if kept:
        page = server_page(ctx, guard, watch, KEPT)
        page.locator("#login-dialog[open]").wait_for()
        page.locator("#login-way-fp").wait_for(state="visible")
    else:
        page = ctx().new_page()
        guard.page = page
        page.set_default_timeout(5000)
        page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
        page.goto(URL)
        assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in's card"
    frames(page)
    return page


@pytest.mark.parametrize("kept", (False, True), ids=("browser", "app"))
def test_a_touch_screen_gets_no_focus_and_no_move(new_context, case, watch, guard, kept):
    """On a touch screen (UiParts D224) the card puts no focus into a field
    by itself, at its opening or at a way's tap, so that the soft keyboard
    comes when a field is tapped, and a switch of the way, both directions,
    leaves the card where it is and as tall: the two ways' forms take one
    height. Fails on the page before: it focused the way's first field and
    the card changed its height with the way."""
    page = card_in(new_context, guard, watch, kept, has_touch=True, is_mobile=case.width == 390)
    assert page.evaluate("matchMedia('(pointer: coarse)').matches"), "the context is a touch screen"
    assert not page.evaluate(IN_FIELD), "no focus in a field at the opening"
    before = page.evaluate(LOGIN_BOX)
    for way in ("key", "password"):
        page.locator(WAY % way).tap()
        frames(page)
        assert page.locator(WAY % way).get_attribute("aria-pressed") == "true", f"{way} chosen"
        assert not page.evaluate(IN_FIELD), f"no focus in a field after {way}'s tap"
        same_box(page, before, f"after {way}'s tap")


@pytest.mark.parametrize("kept", (False, True), ids=("browser", "app"))
def test_a_desktop_keeps_the_focus_and_no_move(new_context, case, watch, guard, kept):
    """On a desktop the card puts the focus into the chosen way's first
    field at its opening and at a way's click, as before, and a switch of
    the way leaves the card where it is and as tall (UiParts D224). The
    focus is a forward guard, the page before gave it so too; the card's
    place fails there."""
    page = card_in(new_context, guard, watch, kept)
    assert wait_until(page, "document.activeElement && document.activeElement.id === 'login-user'", 3), "the name in focus at the opening"
    before = page.evaluate(LOGIN_BOX)
    for way, field in (("key", "login-token"), ("password", "login-user")):
        page.locator(WAY % way).click()
        frames(page)
        assert page.evaluate("document.activeElement.id") == field, f"{field} in focus after {way}'s click"
        same_box(page, before, f"after {way}'s click")
