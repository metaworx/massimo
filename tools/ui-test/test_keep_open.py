# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Keeping a server without users open (AP FirstOwner, Block 3): under
the first owner's form, Keep it open… shows a confirmation in the card,
the owner's warning first, this page's address to copy and a field for
it, Confirm off until the address entered is this page's own (FirstOwner
D13, D28); Confirm writes auth.keep_open and the page goes on; System
settings → Users undoes it while the server is kept open (D5, D32). GET
/api/v1/me and the setting's PUT are stubbed (D16): the suite's server
has users. The address rows run on the test server's own origin and on
the same server reached as localhost (D33); an https page, an IPv6 host
and a page under a path prefix are read in store.ts sameAddress, not
served here."""
import json

import pytest

from conftest import URL
from helpers import ApiReads, assert_login_card_centred, frames, scroll_width, show_view, wait_held, wait_until
from test_first_owner import APP, DIALOG_OPEN, IN_FIELD, OPEN, owner_card, sent, start_page, turned, wrapped

BOX = "(() => { const r = document.getElementById('login-dialog').getBoundingClientRect(); return [r.top, r.height]; })()"
KEPT = dict(OPEN, kept_open=True)
CONFIRM = "#keep-open-confirm"


def same_box(page, before, what):
    """The card where it was and as tall, within half a pixel."""
    after = page.evaluate(BOX)
    assert all(abs(a - b) <= 0.5 for a, b in zip(after, before)), f"{what}: the card moved, {before} → {after}"


def keep_step(page):
    """The confirmation shown from the first owner's form, drawn."""
    page.locator("#setup-keep-open").click()
    page.locator("#keep-open").wait_for(state="visible")
    frames(page)


def put_route(page, bodies, answer=None):
    """PUT /api/v1/settings noted in bodies and answered by answer(route)
    (200 with an empty object by default); a GET goes to the server."""
    def handle(route):
        if route.request.method != "PUT":
            return route.continue_()
        bodies.append(json.loads(route.request.post_data or "{}"))
        if answer:
            return answer(route)
        route.fulfill(status=200, content_type="application/json", body="{}")
    page.route("**/api/v1/settings", handle)


def rows(origin):
    """The address table on a page at origin (scheme://host:port): an
    input and whether it is this page's address."""
    scheme, rest = origin.split("://")
    host = rest.rsplit(":", 1)[0]
    return [
        (origin, True),
        ("  " + origin + "/  ", True),
        (scheme.upper() + "://" + rest, True),
        (origin + "/", True),
        (origin + "/x", False),
        (rest, False),
        (scheme + "://" + host, False),
        ("https://" + rest, False),
        (scheme + "://user@" + rest, False),
        (origin + "?x", False),
        (origin + "#x", False),
        (origin + " x", False),
        ("", False),
    ]


def check_rows(page, table):
    field = page.locator("#keep-open-address")
    for text, match in table:
        field.fill(text)
        frames(page)
        off = page.evaluate(f"document.querySelector('{CONFIRM}').disabled")
        hint = page.locator("#keep-open-hint").count() > 0 and page.locator("#keep-open-hint").is_visible()
        assert off == (not match), f"{text!r}: Confirm {'off' if off else 'on'}"
        assert hint == (bool(text.strip()) and not match), f"{text!r}: the line under the field {'shown' if hint else 'not shown'}"


def test_keep_open_asks_for_the_address(new_context, case, watch, guard):
    """Keep it open… shows the confirmation in the form's place, the card
    neither moved nor resized: the owner's warning first and word for
    word, this page's address, the field in focus on a desktop; Confirm
    is off for every other address, said under the field, and on for this
    page's, typed or pasted; Enter with another address sends nothing (no
    check of the browser's own stands in); Back shows the form with what
    was typed in both, the focus on Keep it open…. Fails on the page
    before: no Keep it open…."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    bodies = []
    put_route(page, bodies)
    page.locator("#setup-user").fill("owner")
    before = page.evaluate(BOX)
    keep_step(page)
    same_box(page, before, "Keep it open…")
    assert page.locator("#setup-form").is_hidden(), "the form unseen"
    assert page.evaluate("document.getElementById('login-title').textContent === t('keepOpen.title')"), "the confirmation's title"
    assert page.evaluate("document.getElementById('keep-open').firstElementChild.id") == "keep-open-warning", "the warning first"
    warning = page.locator("#keep-open-warning").inner_text()
    assert warning == page.evaluate("t('keepOpen.warning')"), "the warning's text"
    if case.lang == "en":
        assert warning == "Not recommended - ONLY use it on a local network you trust!", "the owner's words"
    origin = page.evaluate("location.origin")
    assert page.locator("#keep-open-origin").inner_text() == origin, "this page's address shown"
    assert wait_until(page, "document.activeElement && document.activeElement.id === 'keep-open-address'", 3), "the field in focus"
    check_rows(page, rows(origin))
    field = page.locator("#keep-open-address")
    field.fill(origin + "/x")
    field.press("Enter")
    frames(page)
    assert not bodies and page.locator("#keep-open").is_visible(), "Enter with another address sends nothing"
    assert not page.evaluate("document.querySelector('#keep-open-address').matches(':invalid')") or \
        page.evaluate("document.getElementById('keep-open').noValidate"), "the browser's own check does not stand in"
    field.fill("")
    field.focus()
    page.keyboard.insert_text(origin)
    frames(page)
    assert not page.evaluate(f"document.querySelector('{CONFIRM}').disabled"), "on for this page's address, pasted"
    page.locator("#keep-open-back").click()
    frames(page)
    same_box(page, before, "Back")
    assert page.locator("#setup-form").is_visible() and page.locator("#keep-open").is_hidden(), "the form again"
    assert page.locator("#setup-user").input_value() == "owner", "the name typed stays"
    assert page.evaluate("document.activeElement.id") == "setup-keep-open", "the focus on Keep it open…"
    keep_step(page)
    assert field.input_value() == origin, "the address typed stays through Back"
    assert not bodies, f"nothing sent: {bodies}"


def test_keep_open_on_another_name_of_the_server(new_context, case, watch, guard):
    """The same server reached as localhost: its own address matches,
    the host in capitals too, and the address of 127.0.0.1 does not (the
    other host). Fails on the page before: no Keep it open…."""
    page = start_page(new_context, guard, watch, OPEN, url=URL.replace("127.0.0.1", "localhost"))
    owner_card(page)
    keep_step(page)
    origin = page.evaluate("location.origin")
    assert "localhost" in origin, origin
    check_rows(page, [(origin, True), (origin.replace("localhost", "LOCALHOST"), True), (origin.replace("localhost", "127.0.0.1"), False)])


def test_confirm_keeps_the_server_open(new_context, case, watch, guard):
    """Confirm sends auth.keep_open true once and turns, Back and the
    card's ☰ entry off meanwhile; then who is in says kept_open, the card
    closes and the page starts once, with its banner; a reload asks no
    more. Fails on the page before: no Keep it open…."""
    stage = {"me": OPEN}
    page = start_page(new_context, guard, watch, lambda: stage["me"], app=APP)
    owner_card(page)
    bodies, held = [], []
    put_route(page, bodies, lambda route: held.append(route))
    keep_step(page)
    page.locator("#keep-open-address").fill(page.evaluate("location.origin"))
    page.locator(CONFIRM).click()
    assert wait_held(page, held), "the setting's write sent"
    assert page.evaluate(f"document.querySelector('{CONFIRM}').classList.contains('busy')"), "Confirm turns"
    assert page.evaluate("document.getElementById('keep-open-back').disabled && document.getElementById('login-menu-servers').disabled"), \
        "Back and Select server off while it runs"
    stage["me"] = KEPT
    reads = ApiReads(page)
    held.pop().fulfill(status=200, content_type="application/json", body="{}")
    assert wait_until(page, f"!({DIALOG_OPEN})", 5), "the card closes"
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the page started"
    assert reads.answered(), f"the page's reads answered: {reads.open}"
    assert bodies == [{"auth.keep_open": True}], f"the setting sent once: {bodies}"
    assert len(sent(page, "POST", "/events/ticket")) == 1, "the page started once"
    assert page.locator("#open-banner").is_visible(), "the open server's banner stays (FirstOwner D30)"
    page.reload()
    page.wait_for_selector(".room", timeout=10000)
    assert not page.evaluate(DIALOG_OPEN), "no card at the next load"


@pytest.mark.parametrize("refusal", ["401", "409", "403", "network"])
def test_a_refusal_of_keep_open(new_context, case, watch, guard, refusal):
    """A refusal for lack of a session (401) or because the server has
    users now (409) turns the card to the sign-in; any other (403, the
    network) is said under the field and the confirmation stays, Confirm
    on again. Fails on the page before: no Keep it open…."""
    def answer(route):
        if refusal == "network":
            return route.abort()
        status = int(refusal)
        route.fulfill(status=status, content_type="application/json", body=json.dumps({"error": f"refused {status}"}))
    page = start_page(new_context, guard, watch, OPEN)
    if refusal == "network":
        watch.expected.append(("/api/v1/settings", "net::"))
    else:
        watch.expected.append(("/api/v1/settings", int(refusal)))
    owner_card(page)
    put_route(page, [], answer)
    keep_step(page)
    page.locator("#keep-open-address").fill(page.evaluate("location.origin"))
    page.locator(CONFIRM).click()
    if refusal in ("401", "409"):
        turned(page)
        return
    assert wait_until(page, "!document.getElementById('keep-open-msg').classList.contains('hidden')", 5), "the refusal said"
    assert page.evaluate("document.getElementById('keep-open-msg').textContent.startsWith(t('keepOpen.failed', {error: ''}))"), \
        page.locator("#keep-open-msg").inner_text()
    assert page.locator("#keep-open").is_visible() and page.evaluate(DIALOG_OPEN), "the confirmation stays"
    assert wait_until(page, f"!document.querySelector('{CONFIRM}').disabled && !document.getElementById('keep-open-back').disabled", 3), \
        "Confirm and Back on again"


def test_keep_open_on_a_touch_screen(new_context, case, watch, guard):
    """On a touch screen the confirmation puts no focus into its field,
    so that no soft keyboard comes by itself: the warning takes it, which
    a screen reader then reads first. Fails on the page before: no Keep it
    open…."""
    page = start_page(new_context, guard, watch, OPEN, has_touch=True, is_mobile=case.width == 390)
    owner_card(page)
    page.locator("#setup-keep-open").tap()
    page.locator("#keep-open").wait_for(state="visible")
    frames(page)
    assert not page.evaluate(IN_FIELD), "no field in focus"
    assert page.evaluate("document.activeElement.id") == "keep-open-warning", "the warning in focus"


def test_no_keep_open_where_kept_open(new_context, case, watch, guard):
    """Where the server is kept open already, the card opened on it (as
    the banner, ☰ and Users will open it) offers no Keep it open…. Fails
    on the page before: the button did not exist, so nothing is hidden;
    a forward guard for the opening of the block after."""
    page = start_page(new_context, guard, watch, KEPT)
    page.wait_for_selector(".room", timeout=10000)
    assert not page.evaluate(DIALOG_OPEN), "no card on a server kept open"
    page.evaluate("() => { askLogin('owner'); }")
    owner_card(page)
    assert page.locator("#setup-keep-open").is_hidden(), "no Keep it open…"
    assert page.locator("#setup-skip").is_visible(), "Skip for now there"


def users_section(page):
    show_view(page, "system")
    page.locator("#sys-users").wait_for(state="visible")
    frames(page)


SWITCH = {"en": "Keep this server open without an owner", "de": "Diesen Server ohne Eigentümer offen halten"}
SWITCH_LABEL = "document.getElementById('keep-open-switch').closest('label').querySelector('.switch-label').textContent"


def test_the_switch_undoes_keep_open(new_context, case, watch, guard):
    """System settings → Users on a server kept open shows the switch,
    on; turned off it sends auth.keep_open false once, then shows off and
    no longer to be turned, with the line that the page asks again at its
    next load, the focus on that line; no card opens. Shown again, the
    section has no such row. The switch's label holds on and off: "Keep
    this server open without an owner" (AP LooseEnds, Block 2), which
    failed on the page before, its label "This server is kept open without
    an owner" while off too (the walk-through, finding 7). Fails on the
    page before that: no switch."""
    stage = {"me": KEPT}
    page = start_page(new_context, guard, watch, lambda: stage["me"])
    page.wait_for_selector(".room", timeout=10000)
    bodies = []

    def answer(route):
        stage["me"] = OPEN
        route.fulfill(status=200, content_type="application/json", body="{}")
    put_route(page, bodies, answer)
    users_section(page)
    switch = page.locator("#keep-open-switch")
    assert switch.is_checked(), "on while the server is kept open"
    assert page.evaluate(SWITCH_LABEL) == SWITCH[case.lang], f"its label, on: {page.evaluate(SWITCH_LABEL)!r}"
    assert page.locator("#keep-open-switch-hint").inner_text() == page.evaluate("t('users.keptOpenHint')"), "what it means"
    switch.focus()
    page.keyboard.press("Space")
    assert wait_until(page, "document.getElementById('keep-open-switch').disabled", 5), "off and no longer to be turned"
    assert not switch.is_checked(), "off"
    assert page.evaluate(SWITCH_LABEL) == SWITCH[case.lang], f"its label, off: {page.evaluate(SWITCH_LABEL)!r}"
    assert page.locator("#keep-open-switch-hint").inner_text() == page.evaluate("t('users.keptOpenOff')"), "the next load asks"
    assert page.evaluate("document.activeElement.id") == "keep-open-switch-hint", "the focus on the line"
    assert bodies == [{"auth.keep_open": False}], f"turned off once: {bodies}"
    assert not page.evaluate(DIALOG_OPEN), "no card at once (FirstOwner D31)"
    show_view(page, "tiles")
    users_section(page)
    assert page.locator("#keep-open-switch").count() == 0, "the row gone when the section is shown again"


def test_the_switch_refused_and_where_it_is_not(new_context, case, watch, guard):
    """A refused turn leaves the switch on, its own line under Users
    saying why; a server not kept open, open or with users, shows no
    switch: turning it on is the card's confirmation alone (FirstOwner
    D32). Who is in is read again by hand for each; on the open server
    that read opens the first owner's card, as every read of who is in
    on a server that asks, and the switch is looked for under it. Fails
    on the page before: no switch."""
    stage = {"me": KEPT}
    page = start_page(new_context, guard, watch, lambda: stage["me"])
    page.wait_for_selector(".room", timeout=10000)
    watch.expected.append(("/api/v1/settings", 500))
    put_route(page, [], lambda route: route.fulfill(status=500, content_type="application/json", body='{"error":"refused"}'))
    users_section(page)
    page.locator("#keep-open-switch").focus()
    page.keyboard.press("Space")
    assert wait_until(page, "document.getElementById('users-msg').classList.contains('bad')", 5), "the refusal said"
    assert page.evaluate("document.getElementById('users-msg').textContent === t('users.keepOpenFailed', {error: 'refused'})"), \
        page.locator("#users-msg").inner_text()
    assert wait_until(page, "document.getElementById('keep-open-switch').checked && !document.getElementById('keep-open-switch').disabled", 3), \
        "the switch on again"
    for me, card in ((OPEN, True), (None, False)):
        stage["me"] = me
        page.evaluate("() => loadMe()")
        frames(page)
        assert page.locator("#keep-open-switch").count() == 0, f"no switch for {me and me['mode'] or 'users'}"
        assert page.evaluate(DIALOG_OPEN) == card, f"the first owner's card {'open' if card else 'not open'} for {me and me['mode'] or 'users'}"
        if card:
            page.locator("#setup-skip").click()  # left, so that the next read is seen alone
            assert wait_until(page, f"!({DIALOG_OPEN})", 5), "the card left"


def test_the_switch_and_the_card_agree(new_context, case, watch, guard):
    """Turned off in Users, the server is kept open no more for the whole
    page: who is in read again, by a read that opens no card (FirstOwner
    D31, D44), the card opened afterwards offers Keep it open…; kept open
    again from it in the same load, the switch is back, on. Fails on the
    page before: the card hid Keep it open… and the switch stayed off
    until a reload."""
    stage = {"me": KEPT}
    page = start_page(new_context, guard, watch, lambda: stage["me"])
    page.wait_for_selector(".room", timeout=10000)

    def answer(route):
        body = json.loads(route.request.post_data or "{}")
        stage["me"] = KEPT if body.get("auth.keep_open") else OPEN
        route.fulfill(status=200, content_type="application/json", body="{}")
    put_route(page, [], answer)
    users_section(page)
    with page.expect_response(lambda r: r.url.endswith("/api/v1/me")):
        page.locator("#keep-open-switch").focus()
        page.keyboard.press("Space")
    assert wait_until(page, "document.getElementById('keep-open-switch').disabled", 5), "off"
    frames(page)
    assert not page.evaluate(DIALOG_OPEN), "who is in read again, no card at once"
    page.evaluate("() => { askLogin('owner'); }")
    owner_card(page)
    assert page.locator("#setup-keep-open").is_visible(), "Keep it open… offered on a server kept open no more"
    keep_step(page)
    page.locator("#keep-open-address").fill(page.evaluate("location.origin"))
    page.locator(CONFIRM).click()
    assert wait_until(page, f"!({DIALOG_OPEN})", 5), "the card closes"
    assert wait_until(page, "!!document.getElementById('keep-open-switch') && document.getElementById('keep-open-switch').checked && "
                            "!document.getElementById('keep-open-switch').disabled", 5), "the switch back, on"


def test_the_turn_lets_the_card_go(new_context, case, watch, guard):
    """Where the read of who is in after Keep it open meets a refusal, the
    card turns to the sign-in, and its ways out are on there again: Select
    server in the card's ☰. Fails on the page before: it stayed off."""
    stage = {"me": OPEN}
    page = start_page(new_context, guard, watch, lambda: stage["me"], app=APP)
    owner_card(page)

    def answer(route):
        stage["me"] = "refuse"
        route.fulfill(status=200, content_type="application/json", body="{}")
    put_route(page, [], answer)

    def me(route):
        if stage["me"] == "refuse":
            return route.fulfill(status=401, content_type="application/json", body='{"error":"authentication required"}')
        route.fulfill(status=200, content_type="application/json", body=json.dumps(stage["me"]))
    page.unroute("**/api/v1/me")
    page.route("**/api/v1/me", me)
    keep_step(page)
    page.locator("#keep-open-address").fill(page.evaluate("location.origin"))
    page.locator(CONFIRM).click()
    turned(page)
    assert not page.evaluate("document.getElementById('login-menu-servers').disabled"), "Select server on again"


@pytest.mark.parametrize("app", (False, True), ids=("browser", "app"))
def test_keep_open_measures(new_context, case, watch, guard, app):
    """The confirmation centred and no wider than the window, each
    button's text on one line, at 390 px in all six languages; in the app
    without the clipboard's API (as over plain HTTP, where the page is no
    secure context) a tap on the address copies it by the page's other
    way, in a browser by the clipboard's API, and says so (FirstOwner D8);
    the address stays a text to select.
    Fails on the page before: no Keep it open…."""
    no_clipboard = "Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true });"
    page = start_page(new_context, guard, watch, OPEN, app=APP if app else None, init=no_clipboard if app else None)
    owner_card(page)
    keep_step(page)
    assert_login_card_centred(page, "the confirmation")
    assert scroll_width(page) <= case.width, "no horizontal scroll"
    if app:
        page.evaluate("() => { document.addEventListener('copy', () => { window.__copied = String(document.getSelection()); }); }")
    else:
        page.context.grant_permissions(["clipboard-read", "clipboard-write"])
    page.locator("#keep-open-origin").click()
    assert wait_until(page, "document.getElementById('keep-open-origin').dataset.copied === t('copy.done')", 3), "it says it copied"
    copied = page.evaluate("window.__copied") if app else page.evaluate("navigator.clipboard.readText()")
    assert copied == page.evaluate("location.origin"), f"this page's address copied: {copied!r}"
    assert page.evaluate("getComputedStyle(document.getElementById('keep-open-origin')).userSelect") != "none", "a text to select"
    if case.width == 390:
        page.locator("#login-menu summary").click()
        for lang in ("fr", "it", "nl", "pl"):
            page.locator("#login-lang").select_option(lang)
            assert wait_until(page, f"i18n.lang === {lang!r} && document.getElementById('login-title').textContent === t('keepOpen.title')", 5), lang
            frames(page)
            assert_login_card_centred(page, f"the confirmation in {lang}")
            assert scroll_width(page) <= case.width, f"no horizontal scroll in {lang}"
            assert not wrapped(page), f"each button's text on one line in {lang}: {wrapped(page)}"
        page.locator("#login-lang").select_option("auto")
        page.keyboard.press("Escape")


def test_the_server_named_on_the_owner_steps(new_context, case, watch, guard):
    """In the app the server's line under the card's title (AP MobileApp,
    D62-A) shows on the first owner's form and on the confirmation that
    keeps the server open alike, the title's text each step's own, and
    the card stays where it is as the steps turn."""
    app = dict(APP, status=dict(APP["status"], server={"name": "Home", "url": URL, "node": "11111111-1111-4111-8111-111111111111", "fp": ""}))
    page = start_page(new_context, guard, watch, OPEN, app=app)
    owner_card(page)
    line = page.locator("#login-server")
    line.wait_for(state="visible")
    assert page.evaluate("document.getElementById('login-title').nextElementSibling.id") == "login-server", "after the title"
    assert page.evaluate("document.getElementById('login-title').textContent === t('setup.title')"), "the form's title alone"
    before = page.evaluate(BOX)
    keep_step(page)
    assert line.is_visible(), "on the confirmation too"
    assert page.evaluate("document.getElementById('login-title').textContent === t('keepOpen.title')"), "the confirmation's title alone"
    same_box(page, before, "the confirmation shown")
