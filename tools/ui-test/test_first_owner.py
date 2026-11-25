# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The first owner in the sign-in card (AP FirstOwner, Block 2): a
server without users asks whoever opens its page from the local network
for its owner, in the sign-in card, at the page's start, where it is not
kept open and the load has not skipped it. GET /api/v1/me is stubbed to
say what such a server says (FirstOwner D16): the suite's server has
users, and its token, the owner's key, serves the page's other requests,
as on a server that is open. The owner's creation is stubbed where the
test server would keep a user it made."""
import json

import pytest

from conftest import TOKEN, URL
from helpers import NAME_BAD, ApiReads, assert_login_card_centred, frames, pause_clock, scroll_width, wait_held, wait_until
from test_connect import FAKE_BRIDGE

OPEN = {"mode": "open", "user": None, "role": None, "capabilities": [], "setup": True}
DIALOG_OPEN = "document.getElementById('login-dialog').open"
OWNER_SHOWN = "!document.getElementById('setup').classList.contains('hidden')"
IN_FIELD = "!!document.activeElement && ['INPUT', 'SELECT', 'TEXTAREA'].includes(document.activeElement.tagName)"
APP = {"status": {"can": True, "user": "", "ask": "start", "plain": False}}


def me_route(page, answer):
    """GET /api/v1/me answered by answer(): a dict, or None for the server's own answer."""
    def handle(route):
        body = answer()
        if body is None:
            return route.continue_()
        route.fulfill(status=200, content_type="application/json", body=json.dumps(body))
    page.route("**/api/v1/me", handle)


def start_page(new_context, guard, watch, me, app=None, init=None, clock=False, url=URL, **ctx_args):
    """A page loaded with GET /api/v1/me stubbed from its start (me, a
    dict, or a function as me_route takes it), with the owner's key: in
    a browser, or in the app (fake_bridge.js) with the login status app;
    init a script run before the page's own; with clock, under
    Playwright's clock, its time flowing until the test pauses it."""
    ctx = new_context(**ctx_args)
    if clock:
        ctx.clock.install()
    if app is None:
        ctx.add_init_script(f"localStorage.setItem('massimo.token', {TOKEN!r})")
    else:
        ctx.add_init_script(f"window.__fakeBridgeConfig = {json.dumps({'kind': 'server', 'token': TOKEN, 'login': app})};")
        ctx.add_init_script(path=FAKE_BRIDGE)
    if init:
        ctx.add_init_script(init)
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.requests = []
    page.on("request", lambda r: page.requests.append((r.method, r.url)))
    me_route(page, me if callable(me) else (lambda: me))
    page.goto(url)
    return page


def wrapped(page):
    """The first owner's buttons shown whose text takes more than one line,
    counted by a Range over the text, as assert_login_row counts them."""
    return page.evaluate("""() => [...document.querySelectorAll('#setup button')].filter((b) => b.offsetParent).filter((b) => {
      const tops = new Set();
      for (const n of b.childNodes) {
        if (n.nodeType !== Node.TEXT_NODE || !n.textContent.trim()) continue;
        const range = document.createRange();
        range.selectNodeContents(n);
        for (const r of range.getClientRects()) tops.add(Math.round(r.top));
      }
      return tops.size > 1;
    }).map((b) => b.textContent)""")


def sent(page, method, end):
    """The requests of method whose path ends with end, sent so far."""
    return [u for m, u in page.requests if m == method and u.split("?")[0].endswith(end)]


def owner_card(page):
    """The card open with the first owner's form."""
    assert wait_until(page, DIALOG_OPEN, 10), "the card opens"
    assert wait_until(page, OWNER_SHOWN, 5), "with the first owner's form"
    frames(page)


def turned(page):
    """The card turned to the sign-in: the password's way and its form,
    the reason said, the first owner's form gone, nothing carried over."""
    assert wait_until(page, "document.getElementById('setup').classList.contains('hidden')", 5), "the first owner's form gone"
    assert page.evaluate(DIALOG_OPEN), "the card stays open"
    assert page.locator("#login-form").is_visible() and page.locator("#login-ways").is_visible(), "the sign-in, its ways shown"
    assert page.locator("#login-ways .login-way[data-way='password']").get_attribute("aria-pressed") == "true", "the password's way"
    assert page.evaluate("document.getElementById('login-title').textContent === t('login.title')"), "the sign-in's title"
    assert page.evaluate("document.getElementById('login-error').textContent === t('setup.exists')"), "it says that the owner exists"
    assert page.locator("#login-user").input_value() == "", "no name carried into the sign-in"


def test_the_card_asks_at_the_start(new_context, case, watch, guard):
    """An open server that asks for its owner gets the sign-in card with
    the first owner's form at the page's start, the page behind it
    hidden and not read: no ways, no provider, ☰ there, the name in focus
    on a desktop. Skip for now shows the page as an open server's, its
    banner and its rooms, without putting the focus anywhere (FirstOwner
    D24); the next load asks again (D10). Fails on the page before: the
    form was a tile among the rooms, and no card opened."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    assert page.locator("#setup-form").is_visible(), "the form in the card"
    assert page.evaluate("document.getElementById('login-title').textContent === t('setup.title')"), "titled that there are no users"
    assert page.locator("#login-ways").is_hidden() and page.locator("#login-form").is_hidden(), "no way to sign in"
    assert page.locator("#login-providers").is_hidden() and not sent(page, "GET", "/api/v1/login/providers"), "no provider read"
    assert page.locator("#login-menu").is_visible(), "the card's ☰"
    assert page.locator("main").is_hidden(), "the page hidden"
    assert wait_until(page, "document.activeElement && document.activeElement.id === 'setup-user'", 3), "the name in focus"
    assert_login_card_centred(page, "the first owner's card")
    api = sorted({u.split("?")[0].split("/api/v1/")[-1] for m, u in page.requests if "/api/v1/" in u or "/events" in u})
    assert set(api) <= {"me", "update"}, f"behind the card only who is in and the update's state, none of the page's data: {api}"
    page.locator("#setup-skip").click()
    assert wait_until(page, f"!({DIALOG_OPEN})", 5), "Skip for now closes the card"
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the page started"
    assert len(sent(page, "POST", "/events/ticket")) == 1, "started once"
    assert page.locator("#open-banner").is_visible(), "the open server's banner"
    assert not page.evaluate(IN_FIELD) and page.evaluate("document.activeElement === document.body"), "no focus placed"
    page.reload()
    owner_card(page)


@pytest.mark.parametrize("me", ["kept", "none", "users"])
def test_no_card_where_not_asked(new_context, case, watch, guard, me):
    """No card on a server kept open (auth.keep_open: kept_open), on a
    server without a store (mode none: no owner to create, FirstOwner
    D6), nor on one with users and the page's key: the page loads to its
    rooms, and the dialog is never shown. A forward guard: the page
    before showed no card on any server."""
    stub = {"kept": dict(OPEN, kept_open=True), "none": {"mode": "none", "user": None, "role": None, "capabilities": []}, "users": None}[me]
    shown = "HTMLDialogElement.prototype.show = ((show) => function () { window.__shown = (window.__shown || 0) + 1; return show.call(this); })(HTMLDialogElement.prototype.show);"
    page = start_page(new_context, guard, watch, stub, init=shown)
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the page started"
    assert page.evaluate("window.__shown || 0") == 0, "the card never shown"
    assert page.locator("#open-banner").is_visible() == (me != "users"), "the banner where the server is open"


def stub_creation(page, posts):
    """The owner's creation stubbed: the password's registration and the
    sign-in in the page (the sign-in's session is the owner's key), and
    POST /api/v1/users, noted in posts and answered 201."""
    def users(route):
        if route.request.method != "POST":
            return route.continue_()
        posts.append(route.request.url)
        route.fulfill(status=201, content_type="application/json", body="{}")
    page.route("**/api/v1/users", users)
    page.evaluate("""(token) => {
      registerPassword = async () => ({ record: "r", ksf: "m=19456,t=2,p=1" });
      signIn = async () => ({ token });
    }""", TOKEN)


def test_create_the_owner_from_the_card(new_context, case, watch, guard):
    """Create the owner creates it, signs in and shows the page as the
    owner, its rooms and Sign out in ☰, with a toast that says so
    (FirstOwner D22); it turns until the card has closed, which waits for
    the read of who is in that follows the creation: a second submit
    then creates nothing (where the page before ended its turn first and
    a second tap was told that the name is taken). One read of who is in
    from the click to the page, as a sign-in at the start has. Fails on
    the page before: no card."""
    made = []
    held = []
    stage = {"me": OPEN}

    def me_answer(route):
        if stage["me"] == "hold":
            held.append(route)
            return
        if stage["me"] is None:
            return route.continue_()
        route.fulfill(status=200, content_type="application/json", body=json.dumps(stage["me"]))

    ctx = new_context()
    ctx.add_init_script(f"localStorage.setItem('massimo.token', {TOKEN!r})")
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    mes = []
    page.on("request", lambda r: mes.append(r.url) if r.method == "GET" and r.url.endswith("/api/v1/me") else None)
    page.route("**/api/v1/me", me_answer)
    page.goto(URL)
    owner_card(page)
    stub_creation(page, made)
    page.locator("#setup-user").fill(f"owner-{case.width}-{case.lang}")
    page.locator("#setup-password").fill("pw")
    page.locator("#setup-again").fill("pw")
    before = len(mes)
    stage["me"] = "hold"
    submit = page.locator("#setup-form button[type=submit]")
    submit.click()
    assert wait_held(page, held), "the read of who is in after the creation"
    assert len(made) == 1, f"the owner created once: {made}"
    assert submit.evaluate("(b) => b.classList.contains('busy') && b.disabled"), "Create the owner turns while who is in is read"
    assert page.evaluate(DIALOG_OPEN), "the card still open"
    page.evaluate("() => { document.getElementById('setup-form').requestSubmit(); }")
    stage["me"] = None
    held.pop().continue_()
    assert wait_until(page, f"!({DIALOG_OPEN})", 5), "the card closes"
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the page started"
    assert len(made) == 1, f"a second submit while it turned created nothing: {made}"
    assert len(mes) - before == 1, f"one read of who is in from the click to the page: {mes[before:]}"
    assert page.locator("#toast").is_visible() and page.evaluate(
        "document.getElementById('toast-body').textContent.startsWith(t('setup.done', {name: %s}))" % json.dumps(f"owner-{case.width}-{case.lang}")), \
        "a toast says the owner was created"
    assert page.evaluate("!document.getElementById('menu-logout').classList.contains('hidden')"), "signed in: Sign out in ☰"
    assert page.locator("#open-banner").is_hidden(), "no banner"


@pytest.mark.parametrize("refusal", ["401", "409"])
def test_a_refusal_turns_the_card_to_the_sign_in(new_context, case, watch, guard, refusal):
    """Where the server has users after all (another device made the
    owner meanwhile), the card becomes the sign-in's for that server
    (FirstOwner D15): the registration refused for lack of a session
    (401), or the creation refused as the first owner exists already
    (409). The password's way chosen, the reason said, the providers read
    once, the name not carried over, the focus in the name on a desktop;
    signed in, the page says nothing of a write not sent, since the
    form's own request is moot now. Fails on the page before: no card."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    if refusal == "401":
        page.route("**/api/v1/users/*/register", lambda route: route.fulfill(
            status=401, content_type="application/json", body='{"error":"authentication required"}'))
    else:
        page.evaluate("""() => { registerPassword = async () => ({ record: "r", ksf: "m=19456,t=2,p=1" }); }""")
        page.route("**/api/v1/users", lambda route: route.fulfill(
            status=409, content_type="application/json", body='{"error":"the first owner exists already: sign in"}'))
    page.locator("#setup-user").fill("owner")
    page.locator("#setup-password").fill("pw")
    page.locator("#setup-again").fill("pw")
    page.locator("#setup-form button[type=submit]").click()
    turned(page)
    assert wait_until(page, "document.activeElement && document.activeElement.id === 'login-user'", 3), "the name in focus"
    assert page.locator("#setup-msg").is_hidden(), "the first owner's form says nothing"
    page.unroute("**/api/v1/me")
    frames(page)
    assert len(sent(page, "GET", "/api/v1/login/providers")) == 1, "the providers read once"
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("op-pass")
    page.locator("#login-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, f"!({DIALOG_OPEN}) && state.eventsOpen", 10), "signed in, the page started"
    assert page.evaluate("state.me && state.me.user && state.me.user.name") == "operator", "as the one who signed in"
    assert not page.evaluate("!document.getElementById('message').classList.contains('hidden') && "
                             "document.getElementById('message').textContent.includes(t('api.notSent'))"), "no write said not sent"


@pytest.mark.parametrize("where", ["card", "skipped"])
def test_a_page_left_open_learns_of_the_owner(new_context, case, watch, guard, where):
    """A page of an open server that another device has given an owner
    learns it when the server next refuses it: nothing reads who is in at
    a resync, but the stream's next ticket is refused once the server
    has users, and the page asks for a sign-in. With the first owner's
    card shown on a running page the page stands still, no stream opened
    (AP FirstOwner, D35), and what still reads by itself (the log's own
    poll, a read here) is refused: the card turns to the sign-in; after
    Skip for now the sign-in's card opens. The ticket asked again stands
    in for the stream the server ends at its next keep-alive. Fails on
    the page before: no card, no Skip."""
    stage = {"me": None if where == "card" else OPEN}
    page = start_page(new_context, guard, watch, lambda: stage["me"])
    if where == "card":
        # the card opened on a running page, at a read of who is in that finds the server open
        page.wait_for_selector(".room", timeout=10000)
        assert wait_until(page, "state.eventsOpen", 10), "the page started"
        stage["me"] = OPEN
        page.evaluate("() => { loadMe(); }")
        owner_card(page)
    else:
        owner_card(page)
        page.locator("#setup-skip").click()
        page.wait_for_selector(".room", timeout=10000)
        assert wait_until(page, "state.eventsOpen", 10), "the page started"
    page.route("**/events/ticket", lambda route: route.fulfill(status=401, content_type="application/json", body='{"error":"authentication required"}'))
    if where == "card":
        page.route("**/api/v1/me?again=1", lambda route: route.fulfill(status=401, content_type="application/json",
                                                                     body='{"error":"authentication required"}'))
        page.evaluate("() => { api('GET', 'api/v1/me?again=1').catch(() => {}); }")
        turned(page)
        page.unroute("**/api/v1/me?again=1")
    else:
        page.evaluate("() => { connectEvents(); }")
    if where != "card":
        assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens"
        assert page.locator("#login-ways").is_visible() and page.locator("#setup").is_hidden(), "the sign-in's card"
        assert page.locator("#login-ways .login-way[data-way='password']").get_attribute("aria-pressed") == "true", \
            "the password's way: a server seen open with a store (FirstOwner D42)"
    page.unroute("**/events/ticket")
    page.unroute("**/api/v1/me")
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("op-pass")
    page.locator("#login-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, f"!({DIALOG_OPEN}) && state.eventsOpen", 10), "signed in, the stream open again"


@pytest.mark.parametrize("app", (False, True), ids=("browser", "app"))
def test_the_owner_card_measures(new_context, case, watch, guard, app):
    """The card with the first owner's form in the middle of the window,
    no wider than it, in a browser and in the app with its switch and
    Cancel; at 390 px in French, Italian, Dutch and Polish as well,
    chosen in the card's ☰ (FirstOwner D23: the owner's state at its own
    height). Fails on the page before: no card."""
    page = start_page(new_context, guard, watch, OPEN, app=APP if app else None)
    owner_card(page)
    if app:
        page.locator("#setup-keep-row").wait_for(state="visible")
        assert page.locator("#setup-cancel").is_visible(), "Cancel in the app"
    assert_login_card_centred(page, "the first owner's card")
    assert scroll_width(page) <= case.width, "no horizontal scroll"
    assert not wrapped(page), f"each button's text on one line: {wrapped(page)}"
    if case.width == 390:
        page.locator("#login-menu summary").click()
        for lang in ("fr", "it", "nl", "pl"):
            page.locator("#login-lang").select_option(lang)
            assert wait_until(page, f"i18n.lang === {lang!r} && document.getElementById('login-title').textContent === t('setup.title')", 5), \
                f"the card in {lang}"
            frames(page)
            assert_login_card_centred(page, f"the first owner's card in {lang}")
            assert scroll_width(page) <= case.width, f"no horizontal scroll in {lang}"
            assert not wrapped(page), f"each button's text on one line in {lang}: {wrapped(page)}"
        page.locator("#login-lang").select_option("auto")
        page.keyboard.press("Escape")


def test_a_touch_screen_gets_no_focus_in_the_owner_card(new_context, case, watch, guard):
    """On a touch screen the first owner's card puts no focus into a
    field when it opens, so that the soft keyboard comes when a field is
    tapped (UiParts D224). Fails on the page before: no card."""
    page = start_page(new_context, guard, watch, OPEN, has_touch=True, is_mobile=case.width == 390)
    owner_card(page)
    assert page.evaluate("matchMedia('(pointer: coarse)').matches"), "the context is a touch screen"
    assert not page.evaluate(IN_FIELD), "no focus in a field at the opening"


def held_creation(page, keep=None):
    """The owner's creation stubbed and its sign-in held until
    window.__letSignIn(): POST /api/v1/users answered 201, the
    password's registration in the page; keep, where given, what the
    app's login.keep() answers."""
    def users(route):
        if route.request.method != "POST":
            return route.continue_()
        route.fulfill(status=201, content_type="application/json", body="{}")
    page.route("**/api/v1/users", users)
    page.evaluate("""([token, keep]) => {
      registerPassword = async () => ({ record: "r", ksf: "m=19456,t=2,p=1" });
      window.__signIn = signIn;
      signIn = () => new Promise((res) => { window.__letSignIn = () => res({ token }); });
      if (keep !== null) window.massimo.login.keep = async () => keep;
    }""", [TOKEN, keep])


def fill_owner(page, name="owner", password="pw", again="pw"):
    page.locator("#setup-user").fill(name)
    page.locator("#setup-password").fill(password)
    page.locator("#setup-again").fill(again)


def test_skip_and_cancel_are_off_while_create_turns(new_context, case, watch, guard):
    """While Create the owner turns, Skip for now and the app's Cancel are
    off; a card closed meanwhile all the same (the dialog's close stands
    in for any other way) lets the start go on once, and the creation's
    late end then closes and starts nothing: one stream ticket, no toast.
    Fails on the page before: Skip was live, and the late end started the
    page a second time."""
    page = start_page(new_context, guard, watch, OPEN, app=APP)
    owner_card(page)
    held_creation(page)
    fill_owner(page)
    submit = page.locator("#setup-form button[type=submit]")
    submit.click()
    assert wait_until(page, "typeof window.__letSignIn === 'function'", 5), "the sign-in reached"
    assert page.evaluate("document.getElementById('setup-skip').disabled && document.getElementById('setup-cancel').disabled"), \
        "Skip for now and Cancel off while Create the owner turns"
    assert page.evaluate("document.getElementById('login-menu-servers').disabled"), "and the card's Select server"
    page.evaluate("() => { document.getElementById('login-dialog').close(); }")
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the page started"
    reads = ApiReads(page)
    page.evaluate("() => { window.__letSignIn(); }")
    assert wait_until(page, "!document.querySelector('#setup-form button[type=submit]').classList.contains('busy')", 5), "the creation ended"
    # what a second start would send (the preferences, the Cubes and the rooms, then the ticket) answered before the count
    assert reads.answered(), f"the page's reads answered: {reads.open}"
    assert len(sent(page, "POST", "/events/ticket")) == 1, f"the page started once: {sent(page, 'POST', '/events/ticket')}"
    assert page.locator("#toast").is_hidden(), "no toast for a creation whose card had closed"


def test_a_late_creation_leaves_a_later_sign_in_open(new_context, case, watch, guard):
    """A card closed while Create the owner turns, then a 401 that opens
    the sign-in: the creation's late end leaves that sign-in open, and a
    sign-in there answers the reads that wait for it. Fails on the page
    before: the late end closed the new card, whose promise then never
    answered."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    held_creation(page)
    fill_owner(page)
    page.locator("#setup-form button[type=submit]").click()
    assert wait_until(page, "typeof window.__letSignIn === 'function'", 5), "the sign-in reached"
    page.evaluate("() => { document.getElementById('login-dialog').close(); }")
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the page started"
    page.route("**/events/ticket", lambda route: route.fulfill(status=401, content_type="application/json", body='{"error":"authentication required"}'))
    page.evaluate("() => { connectEvents(); }")
    assert wait_until(page, DIALOG_OPEN, 5), "a sign-in opens for the 401"
    page.evaluate("() => { window.__p = loginPromise; window.__p.then((ok) => { window.__answered = ok; }); }")
    page.evaluate("() => { window.__letSignIn(); }")
    assert wait_until(page, "!document.querySelector('#setup-form button[type=submit]').classList.contains('busy')", 5), "the creation ended"
    frames(page)
    assert page.evaluate(DIALOG_OPEN) and page.evaluate("loginPromise === window.__p"), "the sign-in stays open, its promise the same"
    page.unroute("**/events/ticket")
    page.unroute("**/api/v1/me")
    page.evaluate("() => { signIn = window.__signIn; }")  # the sign-in by OPAQUE again, for the operator
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("op-pass")
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, f"!({DIALOG_OPEN}) && window.__answered === true && state.eventsOpen", 10), \
        page.evaluate("({open: document.getElementById('login-dialog').open, answered: window.__answered, events: state.eventsOpen, "
                      "said: document.getElementById('login-error').textContent, lp: loginPromise === null})")


def test_the_passwords_are_checked_before_anything_is_sent(new_context, case, watch, guard):
    """The two passwords read at the submit: where they differ, the form
    says so, the mismatch first even with the first one empty, and
    nothing is sent. A forward guard: the page before checked them so in
    its tile."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    for password, again in (("pw", "other"), ("", "pw")):
        fill_owner(page, password=password, again=again)
        page.locator("#setup-form button[type=submit]").click()
        assert wait_until(page, "document.getElementById('setup-msg').textContent === t('setup.mismatch')", 3), \
            f"the passwords differ: {password!r}, {again!r}"
    assert not sent(page, "POST", "/register") and not sent(page, "POST", "/api/v1/users"), "nothing sent"


def test_the_form_says_only_what_is_wrong_now(new_context, case, watch, guard):
    """Each Create the owner says what is wrong at that submit alone and
    puts the focus on the field to change, in its handler: a mismatch says
    so with the focus on Again; Again fixed and a bad name, the name's rule
    alone, the mismatch's line gone, the focus on the name. No password at
    all the browser refuses itself (the fields are required), Password
    invalid and in focus. Nothing is sent. Fails on the page before: the
    mismatch's line stayed beside the name's rule, and the focus stayed on
    Create the owner (the walk-through of AP LooseEnds, finding 3)."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    submit = page.locator("#setup-form button[type=submit]")
    fill_owner(page, password="pw", again="other")
    submit.click()
    assert wait_until(page, "document.getElementById('setup-msg').textContent === t('setup.mismatch')", 3), "the passwords differ"
    assert page.evaluate("document.activeElement.id") == "setup-again", "the focus on Again"
    fill_owner(page, name="bad name", password="pw", again="pw")
    submit.click()
    assert wait_until(page, "!document.getElementById('setup-name-msg').classList.contains('hidden')", 3), "the name's rule"
    assert page.locator("#setup-name-msg").inner_text() == NAME_BAD[case.lang], "the store's rule"
    assert page.locator("#setup-msg").is_hidden(), "and no line of the earlier attempt"
    assert page.evaluate("document.activeElement.id") == "setup-user", "the focus on the name"
    fill_owner(page, password="", again="")
    submit.click()
    assert wait_until(page, "document.activeElement && document.activeElement.id === 'setup-password'", 3), "the browser's focus on Password"
    assert page.evaluate("document.getElementById('setup-password').matches(':invalid')"), "Password refused by the browser"
    assert not sent(page, "POST", "/register") and not sent(page, "POST", "/api/v1/users"), "nothing sent"


def test_the_owner_card_empties_its_passwords_after_the_owner(new_context, case, watch, guard):
    """A mismatch and Keep it open… and Back keep both passwords. Once the
    owner is made and the card has closed, the two stay while the page's
    read of the owner's settings, its first after the close, is held, and
    go once it has settled: both empty and hidden, the name kept (AP
    LooseEnds, D13, D41-A). Fails on the page before: they kept the
    passwords until the next opening (FirstOwner D43)."""
    stage = {"me": OPEN}
    page = start_page(new_context, guard, watch, lambda: stage["me"])
    owner_card(page)
    stub_creation(page, [])
    held, once = [], {"held": False}

    def prefs(route):
        if route.request.method == "GET" and not once["held"]:
            once["held"] = True
            held.append(route)
            return
        route.continue_()

    page.route("**/api/v1/me/preferences", prefs)
    fill_owner(page, password="pw", again="other")
    page.locator("#setup-form button[type=submit]").click()
    assert wait_until(page, "document.getElementById('setup-msg').textContent === t('setup.mismatch')", 3), "the passwords differ"
    page.locator("#setup-keep-open").click()
    page.locator("#keep-open-back").click()
    frames(page)
    assert [page.locator(f).input_value() for f in ("#setup-password", "#setup-again")] == ["pw", "other"], \
        "a refusal and Back keep both"
    page.locator("#setup-again").fill("pw")
    eye = page.locator("#setup-step .pw-eye").last
    eye.wait_for(state="visible")
    eye.click()
    assert page.locator("#setup-again").get_attribute("type") == "text", "Again shown"
    stage["me"] = None
    page.locator("#setup-form button[type=submit]").click()
    assert wait_until(page, f"!({DIALOG_OPEN})", 10), "the owner made, the card closed"
    assert wait_held(page, held), "the owner's settings read, held"
    frames(page)
    assert [page.locator(f).input_value() for f in ("#setup-password", "#setup-again")] == ["pw", "pw"], \
        "kept while the read after the close has not settled"
    held.pop().continue_()
    assert wait_until(page, "document.getElementById('setup-password').value === '' && document.getElementById('setup-again').value === ''", 10), \
        "both emptied once it has"
    for f in ("#setup-password", "#setup-again"):
        assert page.locator(f).get_attribute("type") == "password", f"{f} hidden"
    assert page.locator("#setup-user").input_value() == "owner", "the name kept"


def test_the_name_is_checked_as_it_is_typed(new_context, case, watch, guard):
    """The owner's name, read through the field alone, is checked 2 s
    after the last keystroke, under the paused clock: nothing said before,
    the store's rule after; a fixed name clears the line at once. A
    forward guard: the tile checked it so."""
    page = start_page(new_context, guard, watch, OPEN, clock=True)
    owner_card(page)
    pause_clock(page)
    name = page.locator("#setup-user")
    name.fill("bad name")
    page.clock.run_for(1500)
    assert page.locator("#setup-name-msg").is_hidden(), "nothing said while it is typed"
    page.clock.run_for(600)
    assert wait_until(page, "!document.getElementById('setup-name-msg').classList.contains('hidden')", 3), "checked 2 s later"
    assert page.locator("#setup-name-msg").inner_text() == NAME_BAD[case.lang], "the store's rule"
    assert name.get_attribute("aria-invalid") == "true", "the field marked"
    name.fill("owner")
    assert wait_until(page, "document.getElementById('setup-name-msg').classList.contains('hidden')", 3), "a fixed name clears the line"
    page.clock.resume()


def test_the_toast_says_the_login_was_not_kept(new_context, case, watch, guard):
    """In the app, with the switch on and the login by fingerprint not
    kept, the owner is in all the same, and the toast says both
    (FirstOwner D22, D26). Fails on the page before: no card, no toast."""
    stage = {"me": OPEN}
    page = start_page(new_context, guard, watch, lambda: stage["me"], app=APP)
    owner_card(page)
    held_creation(page, keep=False)
    page.locator("#setup-keep-row").wait_for(state="visible")
    fill_owner(page, name=f"owner-{case.width}-{case.lang}")
    page.locator("#setup-keep-row .slider").click()
    page.locator("#setup-form button[type=submit]").click()
    assert wait_until(page, "typeof window.__letSignIn === 'function'", 5), "the sign-in reached"
    stage["me"] = None
    page.evaluate("() => { window.__letSignIn(); }")
    assert wait_until(page, f"!({DIALOG_OPEN})", 10), "the card closes"
    want = "t('setup.done', {name: %s}) + ' ' + t('setup.notKept')" % json.dumps(f"owner-{case.width}-{case.lang}")
    assert wait_until(page, f"document.getElementById('toast-body').textContent === {want}", 5), \
        page.evaluate("document.getElementById('toast-body').textContent")


def test_the_owner_card_on_a_short_screen(new_context, case, watch, guard):
    """In the app on a phone of 390 by 640 px, the owner's card, taller
    than the window, starts at the window's top and its last button can
    be scrolled to (AP MobileApp, F27). Fails on the page before: no card."""
    page = start_page(new_context, guard, watch, OPEN, app=APP)
    page.set_viewport_size({"width": 390, "height": 640})
    owner_card(page)
    page.locator("#setup-keep-row").wait_for(state="visible")
    box = page.evaluate("(() => { const r = document.getElementById('login-dialog').getBoundingClientRect(); return [r.top, r.height, innerHeight]; })()")
    assert box[1] > box[2], f"the card is taller than the window: {box}"
    assert 0 <= box[0] <= 17, f"it starts at the window's top: {box}"
    skip = page.locator("#setup-skip")
    skip.scroll_into_view_if_needed()
    b = skip.bounding_box()
    assert b and b["y"] >= 0 and b["y"] + b["height"] <= 640, f"Skip for now scrolled into view: {b}"
    assert scroll_width(page) <= 390, "no horizontal scroll"


def test_no_sign_in_notice_on_the_owner_card_over_plain_http(new_context, case, watch, guard):
    """In the app over plain HTTP outside the home network, the owner's
    card shows neither the switch nor the sign-in's notice, whose words
    are for a sign-in by password. Fails on the page before: no card."""
    page = start_page(new_context, guard, watch, OPEN, app={"status": {"can": True, "user": "", "ask": "start", "plain": True}})
    owner_card(page)
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'login.status')", 5), "the app's status read"
    frames(page)
    assert page.locator("#login-fp-plain").is_hidden(), "no sign-in's notice"
    assert page.locator("#setup-keep-row").is_hidden(), "no switch over plain HTTP"
