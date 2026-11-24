# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The ways to the first owner on a running page (AP FirstOwner, Block
4): the open server's warning with Create the owner… and Hide, ☰'s
Create the owner… and System settings → Users' button open the sign-in
card with the first owner's form; the page behind it stands still
(FirstOwner D35) and is shown again after Create (D37); "Back to the
page" gives the focus back (D36). GET /api/v1/me is stubbed (D16): the
suite's server has users, and its token serves the page's other
requests, as on an open server."""
import json

import pytest

from conftest import TOKEN
from helpers import ApiReads, frames, scroll_width, show_view, wait_held, wait_until
from test_first_owner import DIALOG_OPEN, OPEN, owner_card, sent, start_page, turned

KEPT = dict(OPEN, kept_open=True)
NONE = {"mode": "none", "user": None, "role": None, "capabilities": []}
KEY = "massimo.openBanner"


def page_kept(new_context, guard, watch, stage=None, **kw):
    """A running page of a server kept open (no card asks), who is in
    answered by stage["me"] (None for the server's own)."""
    stage = stage if stage is not None else {"me": KEPT}
    page = start_page(new_context, guard, watch, lambda: stage["me"], **kw)
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the page started"
    return page, stage


def open_by(page, way):
    """The card opened by one of the three ways; the control that opened it."""
    if way == "banner":
        page.locator("#open-banner-create").click()
        back = "open-banner-create"
    elif way == "menu":
        page.locator("#nav-menu > summary").click()
        page.locator("#menu-owner").click()
        back = None  # ☰'s button, its summary
    else:
        show_view(page, "system")
        page.locator("#users-create-owner").click()
        back = "users-create-owner"
    owner_card(page)
    return back


def stub_creation(page, stage):
    """The owner's creation stubbed: the registration and the sign-in in
    the page, POST /api/v1/users answered 201, who is in the server's own
    (the owner's key) from then on."""
    def users(route):
        if route.request.method != "POST":
            return route.continue_()
        stage["me"] = None
        route.fulfill(status=201, content_type="application/json", body="{}")
    page.route("**/api/v1/users", users)
    page.evaluate("""(token) => {
      registerPassword = async () => ({ record: "r", ksf: "m=19456,t=2,p=1" });
      signIn = async () => ({ token });
    }""", TOKEN)


def create(page, name="owner"):
    page.locator("#setup-user").fill(name)
    page.locator("#setup-password").fill("pw")
    page.locator("#setup-again").fill("pw")
    page.locator("#setup-form button[type=submit]").click()


@pytest.mark.parametrize("way", ["banner", "menu", "users"])
def test_each_way_opens_the_card(new_context, case, watch, guard, way):
    """Each of the three ways opens the card with the first owner's form
    on a running page: the page hidden behind it and standing still, the
    poll stopped and the stream closed (FirstOwner D35), no Keep it open…
    on a server kept open, and "Back to the page", which closes it, marks
    nothing (a later way opens it again), starts the poll and the stream
    again and gives the focus back to the control that opened it (D36).
    Fails on the page before: none of the three ways."""
    page, _ = page_kept(new_context, guard, watch)
    tickets = len(sent(page, "POST", "/events/ticket"))
    back = open_by(page, way)
    assert page.locator("main").is_hidden(), "the page hidden behind the card"
    assert page.evaluate("state.pollTimer === null && state.events === null"), "the poll stopped and the stream closed"
    assert page.locator("#setup-keep-open").is_hidden(), "no Keep it open… on a server kept open"
    skip = page.locator("#setup-skip")
    assert skip.inner_text() == page.evaluate("t('setup.back')"), "Back to the page"
    skip.click()
    assert wait_until(page, f"!({DIALOG_OPEN})", 5), "the card closes"
    assert wait_until(page, "state.eventsOpen && state.pollTimer !== null", 10), "the poll and the stream again"
    assert len(sent(page, "POST", "/events/ticket")) == tickets + 1, "the stream opened once more"
    focus = page.evaluate("document.activeElement && (document.activeElement.id || document.activeElement.tagName)")
    if back:
        assert focus == back, f"the focus back on what opened the card: {focus}"
    else:
        assert page.evaluate("document.activeElement === document.querySelector('#nav-menu > summary')"), f"the focus on ☰'s button: {focus}"
    open_by(page, way)
    assert page.evaluate(DIALOG_OPEN), "the card opens again: nothing was skipped"


def focus_back(page, back):
    """The focus on the control that opened the card: back's id, or ☰'s button for None."""
    if back:
        return page.evaluate("document.activeElement && document.activeElement.id") == back
    return page.evaluate("document.activeElement === document.querySelector('#nav-menu > summary')")


@pytest.mark.parametrize("way", ["banner", "menu", "users"])
def test_escape_leaves_a_card_the_user_opened(new_context, case, watch, guard, way):
    """Escape in the card opened by one of the three ways leaves it as
    Back to the page does (AP LooseEnds, D3): the page shown, its poll and
    stream again, the focus back on what opened it. With the card's ☰
    open, the first Escape closes the ☰ alone, the focus on its button,
    and the next leaves the card. Fails on the page before: Escape did
    nothing (the walk-through of AP LooseEnds, finding 5)."""
    page, _ = page_kept(new_context, guard, watch)
    back = open_by(page, way)
    page.keyboard.press("Escape")
    assert wait_until(page, f"!({DIALOG_OPEN})", 5), "Escape closes the card"
    assert page.locator("main").is_visible(), "the page shown"
    assert wait_until(page, "state.eventsOpen && state.pollTimer !== null", 10), "the poll and the stream again"
    assert focus_back(page, back), "the focus back on what opened the card"
    open_by(page, way)
    page.locator("#login-menu summary").click()
    assert page.locator("#login-menu").get_attribute("open") is not None, "the card's ☰ open"
    page.keyboard.press("Escape")
    frames(page)
    assert page.locator("#login-menu").get_attribute("open") is None, "the first Escape closes the ☰"
    assert page.evaluate(DIALOG_OPEN), "and leaves the card open"
    assert page.evaluate("document.activeElement === document.querySelector('#login-menu summary')"), "the focus on the ☰'s button"
    page.keyboard.press("Escape")
    assert wait_until(page, f"!({DIALOG_OPEN})", 5), "the next Escape leaves the card"
    assert focus_back(page, back), "the focus back on what opened the card"


def test_escape_on_the_confirmation_and_not_at_the_start(new_context, case, watch, guard):
    """Escape does nothing in the card the page's start opens, on the
    form or on the confirmation that keeps the server open; in the card
    opened from the banner after Skip for now, Escape on the confirmation
    leaves the card, the focus back on Create the owner…. Fails on the
    page before: Escape did nothing in either (a forward guard for the
    start's card)."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    page.keyboard.press("Escape")
    frames(page)
    assert page.evaluate(DIALOG_OPEN), "the start's card stays at Escape"
    page.locator("#setup-keep-open").click()
    assert page.locator("#keep-open").is_visible(), "the confirmation"
    page.keyboard.press("Escape")
    frames(page)
    assert page.evaluate(DIALOG_OPEN) and page.locator("#keep-open").is_visible(), "the start's confirmation stays at Escape"
    page.locator("#keep-open-back").click()
    page.locator("#setup-skip").click()
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the page started"
    back = open_by(page, "banner")
    page.locator("#setup-keep-open").click()
    assert page.locator("#keep-open").is_visible(), "the confirmation"
    page.keyboard.press("Escape")
    assert wait_until(page, f"!({DIALOG_OPEN})", 5), "Escape on the confirmation leaves the card"
    assert focus_back(page, back), "the focus back on Create the owner…"


def test_no_escape_while_create_turns(new_context, case, watch, guard):
    """Escape does nothing while Create the owner turns in a card the user
    opened: the card stays until the creation has ended, then closes on the
    owner. Fails on the page before: Escape did nothing at all, so a
    forward guard of the guard in the card's Escape."""
    page, stage = page_kept(new_context, guard, watch)
    open_by(page, "banner")
    page.route("**/api/v1/users", lambda route: route.fulfill(status=201, content_type="application/json", body="{}")
               if route.request.method == "POST" else route.continue_())
    page.evaluate("""(token) => {
      registerPassword = async () => ({ record: "r", ksf: "m=19456,t=2,p=1" });
      signIn = () => new Promise((res) => { window.__letSignIn = () => res({ token }); });
    }""", TOKEN)
    create(page)
    assert wait_until(page, "typeof window.__letSignIn === 'function'", 5), "the creation reached its sign-in"
    page.keyboard.press("Escape")
    frames(page)
    assert page.evaluate(DIALOG_OPEN), "the card stays while Create the owner turns"
    stage["me"] = None
    page.evaluate("() => { window.__letSignIn(); }")
    assert wait_until(page, f"!({DIALOG_OPEN})", 10), "the card closes on the owner"


def test_the_banner_hidden_stays_hidden(new_context, case, watch, guard):
    """Hide hides the warning at once, the focus on ☰'s button, and the
    browser keeps it so for that server: after a reload it is still
    hidden while there is no owner; once who is in says the server has
    users, the hiding is forgotten, and the server open again shows its
    warning (FirstOwner D2). Fails on the page before: no Hide."""
    page, stage = page_kept(new_context, guard, watch)
    banner = page.locator("#open-banner")
    assert banner.is_visible(), "the warning"
    page.locator("#open-banner-hide").click()
    assert wait_until(page, "document.getElementById('open-banner').classList.contains('hidden')", 3), "hidden at once"
    assert page.evaluate("document.activeElement === document.querySelector('#nav-menu > summary')"), "the focus on ☰'s button"
    assert page.evaluate(f"localStorage.getItem({KEY!r})") == "hidden", "kept in the browser"
    page.reload()
    page.wait_for_selector(".room", timeout=10000)
    frames(page)
    assert banner.is_hidden(), "still hidden after a reload"
    stage["me"] = None
    page.evaluate("() => loadMe()")
    assert wait_until(page, f"localStorage.getItem({KEY!r}) === null", 3), "forgotten once the server has users"
    stage["me"] = KEPT
    page.evaluate("() => loadMe()")
    assert wait_until(page, "!document.getElementById('open-banner').classList.contains('hidden')", 3), "shown on a server open again"


def test_the_banner_hidden_where_storage_refuses(new_context, case, watch, guard):
    """Where the browser's storage refuses a write, Hide hides the
    warning for that page alone: the next load shows it. Fails on the
    page before: no Hide."""
    refuse = "Storage.prototype.setItem = function () { throw new DOMException('refused', 'SecurityError'); };"
    page, _ = page_kept(new_context, guard, watch, init=refuse)
    page.locator("#open-banner-hide").click()
    assert wait_until(page, "document.getElementById('open-banner').classList.contains('hidden')", 3), "hidden for this page"
    page.reload()
    page.wait_for_selector(".room", timeout=10000)
    frames(page)
    assert page.locator("#open-banner").is_visible(), "shown again at the next load"


def test_no_buttons_on_a_server_without_a_store(new_context, case, watch, guard):
    """A server without a store keeps its warning as it is: no owner can
    be made from the page, so neither Create the owner… nor Hide, and ☰
    has no entry for it (FirstOwner D39). A forward guard: the warning had
    no buttons before."""
    page, _ = page_kept(new_context, guard, watch, stage={"me": NONE})
    assert page.locator("#open-banner").is_visible(), "the warning"
    assert page.locator("#open-banner-create").count() == 0 and page.locator("#open-banner-hide").count() == 0, "no buttons"
    assert page.locator("#menu-owner").is_hidden(), "no entry in ☰"


def test_menu_entries(new_context, case, watch, guard):
    """☰ has Create the owner… where Sign out stands on an open server,
    kept open or not, and Sign out without it on a server with users
    (FirstOwner D9, D7). Fails on the page before: no entry."""
    page, stage = page_kept(new_context, guard, watch)
    assert page.locator("#menu-owner").count() == 1 and not page.evaluate("document.getElementById('menu-owner').classList.contains('hidden')"), \
        "Create the owner… on a server kept open"
    assert page.evaluate("document.getElementById('menu-logout').classList.contains('hidden')"), "no Sign out"
    stage["me"] = None
    page.evaluate("() => loadMe()")
    assert wait_until(page, "document.getElementById('menu-owner').classList.contains('hidden') && "
                            "!document.getElementById('menu-logout').classList.contains('hidden')", 3), "Sign out with users, no Create"


def test_hidden_banner_kept_open_menu_to_create(new_context, case, watch, guard):
    """A warning hidden on a server kept open leaves ☰ the way to the
    owner: ☰ → the card → Create the owner, and the page, signed in as
    the owner, shown again where it was, read anew (FirstOwner D37).
    Fails on the page before: no Hide, no entry."""
    page, stage = page_kept(new_context, guard, watch)
    page.locator("#open-banner-hide").click()
    show_view(page, "info")
    open_by(page, "menu")
    assert page.evaluate(f"localStorage.getItem({KEY!r})") == "hidden", "the owner's form opened: the hiding kept (FirstOwner D45)"
    stub_creation(page, stage)
    reads = ApiReads(page)
    create(page)
    assert wait_until(page, f"!({DIALOG_OPEN})", 10), "the card closes"
    assert reads.answered(10), f"the page's reads answered: {reads.open}"
    assert page.evaluate("MassimoParts.pages.shown()") == "info", "the page it was opened on shown again"
    assert page.locator("#sys-info").is_visible(), "System info in view"
    assert wait_until(page, "!document.getElementById('menu-logout').classList.contains('hidden') && "
                            "document.getElementById('menu-owner').classList.contains('hidden')", 5), "signed in: Sign out, no Create"
    assert wait_until(page, "state.eventsOpen && state.pollTimer !== null", 10), "the poll and the stream again"


def test_create_from_users_shows_the_owner(new_context, case, watch, guard):
    """System settings → Users on a server without users shows its line,
    Create the owner… and, kept open, the switch, without the password
    sign-in, the list or Add a user (FirstOwner D14); Create the owner
    from there closes the card on System settings, which reads its users
    again and lists them. Fails on the page before: no button there."""
    page, stage = page_kept(new_context, guard, watch)
    show_view(page, "system")
    assert page.locator("#users-no-owner").is_visible() and page.locator("#users-create-owner").is_visible(), "the line and the button"
    assert page.locator("#keep-open-switch").is_visible(), "the switch, kept open"
    assert page.locator("#password-login").count() == 0 and page.locator("#users-list").count() == 0 and page.locator("#user-add").count() == 0, \
        "no password sign-in, list or Add a user"
    open_by(page, "users")
    stub_creation(page, stage)
    create(page)
    assert wait_until(page, f"!({DIALOG_OPEN})", 10), "the card closes"
    assert wait_until(page, "!!document.querySelector('#users-list .user-panel, #users-list details')", 10), "the users listed"
    assert page.evaluate("MassimoParts.pages.shown()") == "system", "System settings shown again"
    assert page.locator("#users-open").count() == 0, "the open server's section gone"


def test_a_setpoint_under_way_lands(new_context, case, watch, guard):
    """A setpoint's write under way as the card is opened on a running
    page goes out once and lands while the card is up, and the page,
    shown again by Back to the page, says nothing of it as not sent.
    Fails on the page before: no way to open the card."""
    page, _ = page_kept(new_context, guard, watch)
    held, writes = [], []

    def setpoint(route):
        if route.request.method != "PUT":
            return route.continue_()
        writes.append(route.request.url)
        held.append(route)
    page.route("**/api/v1/cubes/*/rooms/*/setpoint", setpoint)
    page.locator(".room .mode-auto").first.click()
    assert wait_held(page, held), "the write sent"
    open_by(page, "menu")
    held.pop().continue_()
    assert wait_until(page, "!document.querySelector('.room .mode-auto.busy')", 5), "the write answered"
    page.locator("#setup-skip").click()
    assert wait_until(page, f"!({DIALOG_OPEN})", 5), "the card closes"
    frames(page)
    assert len(writes) == 1, f"sent once: {writes}"
    assert not page.evaluate("document.body.innerText.includes(t('api.notSent'))"), "nothing said not sent"


def test_no_turn_while_create_runs(new_context, case, watch, guard):
    """A request refused for lack of a session while Create the owner
    runs (the owner made, the page not yet signed in) does not turn the
    card: a read waits for the sign-in the creation ends in and is sent
    again under the owner; a write is not sent again and says so. Fails
    on the page before: no way to open the card on a running page."""
    page, stage = page_kept(new_context, guard, watch)
    open_by(page, "menu")
    page.route("**/api/v1/users", lambda route: (stage.update(me=None), route.fulfill(status=201, content_type="application/json", body="{}")) if route.request.method == "POST" else route.continue_())
    page.evaluate("""(token) => {
      registerPassword = async () => ({ record: "r", ksf: "m=19456,t=2,p=1" });
      signIn = () => new Promise((res) => { window.__letSignIn = () => res({ token }); });
    }""", TOKEN)
    refused = {"n": 0}

    def once(route):
        if refused["n"] == 0:
            refused["n"] += 1
            return route.fulfill(status=401, content_type="application/json", body='{"error":"authentication required"}')
        route.continue_()
    page.route("**/api/v1/me?again=1", once)
    page.route("**/api/v1/preferences-probe", lambda route: route.fulfill(status=401, content_type="application/json", body='{"error":"authentication required"}'))
    create(page)
    assert wait_until(page, "typeof window.__letSignIn === 'function'", 5), "the creation reached its sign-in"
    page.evaluate("""() => {
      window.__read = null; window.__write = null;
      api('GET', 'api/v1/me?again=1').then((m) => { window.__read = m && m.user ? m.user.name : 'none'; });
      api('PUT', 'api/v1/preferences-probe', {}).catch((e) => { window.__write = e.message; });
    }""")
    assert wait_until(page, "window.__write !== null", 5), "the write refused"
    frames(page)
    assert page.locator("#setup").is_visible() and page.locator("#login-form").is_hidden(), "the card did not turn"
    assert page.evaluate("window.__write === t('api.notSent')"), page.evaluate("window.__write")
    page.evaluate("() => { window.__letSignIn(); }")
    assert wait_until(page, f"!({DIALOG_OPEN})", 10), "the card closes on the owner"
    assert wait_until(page, "window.__read !== null && window.__read !== 'none'", 10), "the read sent again under the owner"
    assert refused["n"] == 1


def test_the_turn_on_a_running_page_forgets_the_open_page(new_context, case, watch, guard):
    """Where the server gets its owner elsewhere while the card is up on a
    running page, the turn to the sign-in answers as a refused request's
    sign-in: a stepped setpoint still waiting is dropped, and another
    user's sign-in forgets the open page (who was in, the open server),
    sending nothing of it under the new user. Fails on the page before: no
    way to open the card on a running page."""
    page, stage = page_kept(new_context, guard, watch)
    writes = []
    page.route("**/api/v1/cubes/*/rooms/*/setpoint", lambda route: (writes.append(route.request.url), route.continue_()))
    page.locator(".room:has(.setpoint-row:not(.hidden)) .step-plus").first.click()
    assert not page.evaluate("Object.values(state.ui).every((u) => !u.timer && u.pending === null)"), "a stepped value waits"
    open_by(page, "menu")
    page.route("**/api/v1/me?again=1", lambda route: route.fulfill(status=401, content_type="application/json", body='{"error":"authentication required"}'))
    page.evaluate("() => { api('GET', 'api/v1/me?again=1').catch(() => {}); }")
    turned(page)
    assert page.evaluate("Object.values(state.ui).every((u) => !u.timer && u.pending === null)"), "the stepped value dropped at the turn"
    page.unroute("**/api/v1/me?again=1")
    stage["me"] = None
    page.evaluate("() => { signIn = window.__signIn || signIn; }")
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("op-pass")
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, f"!({DIALOG_OPEN}) && state.eventsOpen", 10), "signed in, the page started"
    assert page.evaluate("state.me.user.name") == "operator", "as the operator"
    assert writes == [], f"no setpoint sent under the operator: {writes}"


def test_no_stream_behind_the_card(new_context, case, watch, guard):
    """Behind a card opened on a running page no stream is opened, also
    where something of the page asks for it meanwhile (a restart's end
    does): the card's leaving opens it once. Fails on the page before:
    no way to open the card on a running page."""
    page, _ = page_kept(new_context, guard, watch)
    open_by(page, "menu")
    tickets = len(sent(page, "POST", "/events/ticket"))
    page.evaluate("() => { connectEvents(); }")
    frames(page)
    assert len(sent(page, "POST", "/events/ticket")) == tickets and page.evaluate("state.events === null"), "no stream behind the card"
    page.locator("#setup-skip").click()
    assert wait_until(page, "state.eventsOpen", 10), "the stream once the card is left"
    assert len(sent(page, "POST", "/events/ticket")) == tickets + 1, "opened once"


def test_a_refusal_during_a_failed_create_turns_the_card(new_context, case, watch, guard):
    """A read refused while Create runs (the server has users now) does not
    turn the card then; where the creation then fails without a session
    (the network), the card turns to the sign-in, and the read waiting is
    answered after a sign-in there. Fails on the page before: the card
    stayed on the owner's form and the read waited until it was left."""
    page, stage = page_kept(new_context, guard, watch)
    open_by(page, "menu")
    held = []
    page.route("**/api/v1/users", lambda route: held.append(route) if route.request.method == "POST" else route.continue_())
    page.evaluate("""() => { registerPassword = async () => ({ record: "r", ksf: "m=19456,t=2,p=1" }); }""")
    refused = {"n": 0}

    def once(route):
        if refused["n"] == 0:
            refused["n"] += 1
            return route.fulfill(status=401, content_type="application/json", body='{"error":"authentication required"}')
        route.continue_()
    page.route("**/api/v1/me?again=1", once)
    watch.expected.append(("/api/v1/users", "net::"))
    create(page)
    assert wait_held(page, held), "the creation sent"
    page.evaluate("() => { window.__read = null; api('GET', 'api/v1/me?again=1').then((m) => { window.__read = m && m.user ? m.user.name : 'none'; }); }")
    frames(page)
    assert page.locator("#setup").is_visible(), "not turned while Create runs"
    held.pop().abort()
    turned(page)
    stage["me"] = None
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("op-pass")
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, f"!({DIALOG_OPEN}) && window.__read === 'operator'", 10), "signed in, the read answered"


def test_a_half_typed_form_stays_through_the_card(new_context, case, watch, guard):
    """A form of System settings half typed (the Cube's name), the card
    opened by ☰ and left by Back to the page: the same field, its value
    and the focus where the card gave it back. A forward guard: the page
    behind the card is only hidden, as at every opening."""
    page, _ = page_kept(new_context, guard, watch)
    show_view(page, "system")
    panel = page.locator("#cubes-list .cube-panel[data-serial='KEQ0000100']")
    if panel.get_attribute("open") is None:
        panel.locator(":scope > summary .cube-head-who").click()
    name = panel.locator("form.cube-form input[name=name]")
    name.wait_for(state="visible")
    name.fill("Half typed")
    page.evaluate("() => { document.querySelector(\"#cubes-list .cube-panel[data-serial='KEQ0000100'] form.cube-form input[name=name]\").__held = true; }")
    open_by(page, "menu")
    reads = ApiReads(page)
    page.locator("#setup-skip").click()
    assert wait_until(page, f"!({DIALOG_OPEN})", 5), "the card left"
    assert wait_until(page, "state.eventsOpen", 10), "the stream open again"
    page.evaluate("() => poll(false)")  # the poll's reload, which the poll started again sends at its first tick
    assert reads.answered(), f"the page's reads answered: {reads.open}"
    frames(page)
    assert name.input_value() == "Half typed", "what was typed stays"
    assert name.evaluate("(e) => e.__held === true"), "the same element"


def test_a_refused_read_forgets_the_hiding(new_context, case, watch, guard):
    """Hide, then a read of who is in refused (another device made the
    owner): this browser learns that the server has users, and forgets
    the hiding at once, so that the server open again shows its warning
    (FirstOwner D45). Fails on the page before: the hiding stayed, the
    browser never having seen the server with users."""
    page, stage = page_kept(new_context, guard, watch)
    page.locator("#open-banner-hide").click()
    assert page.evaluate(f"localStorage.getItem({KEY!r})") == "hidden"
    stage["me"] = "refuse"
    page.unroute("**/api/v1/me")

    def me(route):
        if stage["me"] == "refuse":
            return route.fulfill(status=401, content_type="application/json", body='{"error":"authentication required"}')
        route.fulfill(status=200, content_type="application/json", body=json.dumps(stage["me"]))
    page.route("**/api/v1/me", me)
    watch.expected.append(("/api/v1/me", 401))
    page.evaluate("() => { loadMe(); }")
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in asked for"
    assert wait_until(page, f"localStorage.getItem({KEY!r}) === null", 3), "the hiding forgotten"
    stage["me"] = KEPT
    page.evaluate("() => { document.getElementById('login-dialog').close(); }")
    assert wait_until(page, f"!({DIALOG_OPEN})", 5)
    page.reload()
    page.wait_for_selector(".room", timeout=10000)
    frames(page)
    assert page.locator("#open-banner").is_visible(), "the warning back on the server open again"


def test_a_storage_that_throws_at_the_read(new_context, case, watch, guard):
    """A storage that throws when the warning's key is read: the warning
    shows, and nothing fails on the page. A forward guard: the page reads
    the storage through readStored, which takes the throw."""
    throws = """(() => { const get = Storage.prototype.getItem;
      Storage.prototype.getItem = function (k) { if (k === %s) throw new DOMException('refused', 'SecurityError'); return get.call(this, k); }; })();""" % json.dumps(KEY)
    page, _ = page_kept(new_context, guard, watch, init=throws)
    frames(page)
    assert page.locator("#open-banner").is_visible() and page.locator("#open-banner-hide").is_visible(), "the warning and its Hide"


def test_the_banner_buttons_measure(new_context, case, watch, guard):
    """The warning's two buttons inside it, each label on one line, no
    horizontal scroll, light and dark, at 390 px in all six languages.
    Fails on the page before: no buttons."""
    page, _ = page_kept(new_context, guard, watch)
    langs = ("en", "de", "fr", "it", "nl", "pl") if case.width == 390 else (case.lang,)
    for lang in langs:
        page.evaluate("async (l) => { await changeLanguage(l); applyLanguage(); }", lang)
        for theme in ("light", "dark"):
            page.evaluate("(t) => { document.documentElement.dataset.theme = t; }", theme)
            frames(page)
            m = page.evaluate("""() => {
              const banner = document.getElementById('open-banner').getBoundingClientRect();
              return [...document.querySelectorAll('#open-banner button')].map((b) => {
                const r = b.getBoundingClientRect();
                const tops = new Set();
                for (const n of b.childNodes) {
                  if (n.nodeType !== Node.TEXT_NODE || !n.textContent.trim()) continue;
                  const range = document.createRange();
                  range.selectNodeContents(n);
                  for (const x of range.getClientRects()) tops.add(Math.round(x.top));
                }
                return {name: b.id, lines: tops.size, inside: r.left >= banner.left - 0.5 && r.right <= banner.right + 0.5 && r.top >= banner.top - 0.5 && r.bottom <= banner.bottom + 0.5};
              });
            }""")
            assert len(m) == 2 and all(b["lines"] == 1 and b["inside"] for b in m), f"{lang}, {theme}: {m}"
            assert scroll_width(page) <= case.width, f"no horizontal scroll in {lang}"
    page.evaluate("() => { delete document.documentElement.dataset.theme; }")
