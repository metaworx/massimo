# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Personal settings follow the data in place (AP UiParts, Block 13): My
profile keeps a display name and a password being typed, the elements
they were, through the personal page shown again, which reads the user,
its sessions, keys and identities again; what its settings forms hold
not saved shows as the disk in the card's pill, which opens the form,
and on ☰ and its Personal settings (AP UnsavedMarks, Block 5). At a
sign-out, and for another user signed in after an expired session,
nothing typed is left, and an answer for the user who was reaches
nothing of the next. Nothing typed is saved."""
import json
import os

import pytest

from conftest import TOKEN, URL, api_call
from helpers import frames, hold, mark, open_main_menu, owner_page, replaced, show_view, wait_held, wait_until
from test_admin_updates import menu_mark, out
from test_login import sign_in
from test_oidc import CLIENT_ID, SECRET, provider  # noqa: F401, the fixture
from test_unsaved import expire

FAKE_BRIDGE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "fake_bridge.js")

NAME = "#profile-display-name"
NEW = "#me-new"


def user_read(page, name):
    """A wait for My profile's read of the user name (GET …/users/{name})."""
    return page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith("/api/v1/users/" + name))


def tokens_read(page, name):
    """A wait for the read of the user's sessions and keys that follows it."""
    return page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith(f"/api/v1/users/{name}/tokens"))


def profile_shown(page, name="owner"):
    """My profile shown, its user and its sessions and keys read."""
    with tokens_read(page, name), user_read(page, name):
        show_view(page, "profile")
    page.locator(NAME).wait_for()


def open_password(page):
    """Change password, or Set a password, open."""
    if not page.locator("#me-password").evaluate("(d) => d.open"):
        page.locator("#me-password-title").click()
    page.locator(NEW).wait_for(state="visible")


def test_my_profile_keeps_what_is_typed_when_shown_again(page, case):
    """A display name and a new password typed, not saved: the personal
    page shown again reads the user, its sessions and keys again, and the
    name's field, its Save, the password's field and Create key are the
    elements they were, with what is typed; nothing is sent."""
    profile_shown(page)
    page.locator(NAME).fill("Typed and not saved")
    open_password(page)
    page.locator(NEW).fill("typed-not-sent")
    sels = [NAME, "#profile-name button[type=submit]", NEW, "#profile-tokens .create-key"]
    mark(page, sels)
    writes = []
    page.on("request", lambda r: writes.append(r.url) if r.method != "GET" and "/api/v1/users/" in r.url else None)
    show_view(page, "tiles")
    profile_shown(page)
    assert replaced(page, sels) == [], f"the same elements after the read: {replaced(page, sels)}"
    assert page.locator(NAME).input_value() == "Typed and not saved", "the display name typed kept"
    assert page.locator(NEW).input_value() == "typed-not-sent", "the password typed kept"
    assert writes == [], f"nothing sent: {writes}"
    page.locator("#profile-name .form-reset").click()
    page.locator("#me-password-form .form-reset").click()
    assert page.locator(NEW).input_value() == "", "Reset empties the password"
    show_view(page, "tiles")


def test_my_profile_pill_and_the_menu_say_unsaved(page, case):
    """The display name typed and left, the card folded and Rooms shown:
    ☰ carries the amber mark and its Personal settings the disk; on the
    personal page the card's pill names the form, and its item unfolds
    the card with the focus in the form, what is typed kept; Reset clears
    them all."""
    profile_shown(page)
    page.locator(NAME).fill("Typed and left")
    page.locator("#profile .section-fold").focus()  # out of the form: left
    assert wait_until(page, "!!document.querySelector('#profile .section-pill .problems')", 5), "the card's pill"
    page.locator("#profile .section-fold").click()
    assert wait_until(page, "document.getElementById('profile').classList.contains('folded')", 5), "the card folded"
    show_view(page, "tiles")
    assert menu_mark(page) == "tone-warn", "☰ amber"
    open_main_menu(page)
    assert page.locator("#menu-me .menu-unsaved.tone-warn").count() == 1, "Personal settings with the disk"
    page.locator("#menu-me").click()
    pill = page.locator("#profile .section-pill .problems")
    pill.locator("summary").click()
    action = pill.locator(".problem-action")
    want = page.evaluate("t('form.unsavedItem', {form: t('users.displayName')})")
    assert action.inner_text().strip() == want, f"the item names the form: {action.inner_text()!r}"
    action.click()
    assert wait_until(page, "!document.getElementById('profile').classList.contains('folded')", 5), "the card unfolded"
    assert wait_until(page, "document.activeElement && document.activeElement.id === 'profile-display-name'", 5), "the focus in the form"
    assert page.locator(NAME).input_value() == "Typed and left", "what is typed kept"
    page.locator("#profile-name .form-reset").click()
    assert wait_until(page, "!document.querySelector('#menu-me .menu-unsaved')", 5), "Reset: no disk"
    assert menu_mark(page) == "", "nothing on ☰"
    show_view(page, "tiles")


def typed(page):
    """On the owner's page, My profile with its display name and a new password typed."""
    profile_shown(page)
    page.locator(NAME).fill("Typed by the first")
    open_password(page)
    page.locator(NEW).fill("secret-of-the-first")
    page.locator("#me-again").fill("secret-of-the-first")


def across(page, how):
    """The session ended as how says: a sign-out, through the leaving
    dialog where it asks, or expired with another user's sign-in."""
    if how == "sign-out":
        out(page)
    else:
        expire(page)
    sign_in(page, "operator", "op-pass")
    assert page.evaluate("state.me.user.name") == "operator", "the operator is in"


def nothing_of_the_first(page):
    """No field of the page holds what the first user typed, and no form of it is left with changes."""
    frames(page)
    left = page.evaluate("[...document.querySelectorAll('input')].filter((i) => ['Typed by the first', 'secret-of-the-first'].includes(i.value)).length")
    assert left == 0, f"{left} field(s) still hold what the first user typed"
    assert not page.evaluate("MassimoParts.leaving.changes()"), "no form with changes"


@pytest.mark.parametrize("how", ["sign-out", "expired"])
def test_the_next_user_finds_nothing_typed_in_my_profile(new_context, case, watch, guard, how):
    """The owner's display name and a new password typed in My profile;
    the session ends, and the operator signed in finds neither, in My
    profile or anywhere."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    typed(page)
    across(page, how)
    nothing_of_the_first(page)
    profile_shown(page, "operator")
    nothing_of_the_first(page)
    assert page.locator(NAME).input_value() != "Typed by the first", "the operator's own name"


@pytest.mark.parametrize("how", ["sign-out", "expired"])
def test_a_late_read_of_my_profile_reaches_nothing_of_the_next_user(new_context, case, watch, guard, how):
    """The owner's user read for My profile and held while the session
    ends; let go once the operator is in, it draws nothing of the owner."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    held = []
    hold(page, "**/api/v1/users/owner", "GET", held, [])
    try:
        show_view(page, "profile")
        assert wait_held(page, held), "the user asked for"
        across(page, how)
        with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith("/api/v1/users/owner")) as answer:
            for r in held:
                r.continue_()
        answer.value.finished()  # its body read by the page too
        frames(page)
        frames(page)  # the read's own drawing, where it drew one
        assert page.locator(NAME).count() == 0, "no display name drawn: the operator has not shown My profile since"
        assert page.locator("#profile-tokens li").count() == 0, "nor any session or key"
        assert page.evaluate("document.getElementById('profile-msg').textContent") == "", "nor a message"
    finally:
        page.unroute("**/api/v1/users/owner")


def test_the_app_names_a_linked_account_by_its_provider(new_context, case, watch, guard, provider):
    """In the app (fake_bridge.js), which shows no Link buttons, My profile
    still names a linked account by its provider's name, as linked.js did
    from every enabled provider, not by the provider's key."""
    key = f"app-{case.width}-{case.lang}"
    name = "App " + case.lang
    api_call("POST", "api/v1/providers", {"key": key, "name": name, "issuer": provider.issuer, "client_id": CLIENT_ID, "client_secret": SECRET})
    try:
        api_call("POST", "api/v1/users/owner/identities", {"provider": key, "email": provider.email})
        ctx = new_context()
        ctx.add_init_script(f"window.__fakeBridgeConfig = {json.dumps({'kind': 'server', 'token': TOKEN})};")
        ctx.add_init_script(path=FAKE_BRIDGE)
        page = ctx.new_page()
        guard.page = page
        page.set_default_timeout(5000)
        page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
        page.goto(URL)
        page.wait_for_selector(".room", timeout=10000)
        assert page.evaluate("!!appBridge()"), "the page runs in the app"
        profile_shown(page)
        row = page.locator("#profile-identities .user-identities li[data-id]").filter(has_text=provider.email)
        row.wait_for(state="visible")
        assert row.locator(".identity-provider").inner_text() == name, "the account named by its provider's name"
        assert page.locator("#profile-identities button[data-provider]").count() == 0, "no Link button in the app"
    finally:
        api_call("DELETE", f"api/v1/providers/{key}")


def test_a_current_password_alone_is_no_change(new_context, case, watch, guard):
    """The operator, who has a password, its current one filled in alone,
    as a password manager does: the form is not changed and nothing is
    left to lose; the new one typed is (UnsavedMarks D28)."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    sign_in(page, "operator", "op-pass")
    profile_shown(page, "operator")
    open_password(page)
    current = page.locator("#me-current")
    assert current.is_visible(), "Change password asks for the current one"
    current.fill("filled-by-a-manager")
    frames(page)
    assert not page.evaluate("MassimoParts.leaving.changes()"), "the current password alone is no change"
    page.locator(NEW).fill("typed-not-sent")
    assert wait_until(page, "MassimoParts.leaving.changes()", 5), "the new one is"
    page.locator("#me-password-form .form-reset").click()


def test_the_own_password_is_marked_and_set_again_empty(page, case):
    """A password typed in My profile marks its form changed for the
    leaving dialog, never keeping it by a key; Reset empties the fields
    and the form is clean again."""
    profile_shown(page)
    open_password(page)
    page.locator(NEW).fill("typed-not-sent")
    assert wait_until(page, "MassimoParts.leaving.changes()", 5), "the form listed as changed"
    page.locator("#me-password-form .form-reset").click()
    assert page.locator(NEW).input_value() == "", "Reset empties it"
    assert not page.evaluate("MassimoParts.leaving.changes()"), "nothing left to lose"
    show_view(page, "tiles")


# --- My settings ---------------------------------------------------------

def test_a_poll_being_typed_stays_when_the_settings_arrive(page, case):
    """Poll every typed and not yet left; the user's settings arrive with
    another poll, as at a sign-in (loadPreferences: kept in the browser,
    then applied again): the field is the element it was with what is
    typed and the focus, and nothing typed was saved; left, it saves what
    it holds."""
    show_view(page, "prefs")
    field = page.locator("#poll-input")
    was = page.evaluate("state.settings.poll")
    other = 31 if was != 31 else 32
    field.fill("27")
    mark(page, ["#poll-input"])
    try:
        page.evaluate("(v) => { storeUserSetting('poll', v); return applyStoredSettings(); }", other)
        frames(page)
        assert replaced(page, ["#poll-input"]) == [], "the same field"
        assert field.input_value() == "27", "what is typed kept"
        assert page.evaluate("document.activeElement.id") == "poll-input", "the focus where it was"
        assert page.evaluate("state.settings.poll") == other, "nothing typed saved"
        page.locator("#expert-toggle").focus()
        assert wait_until(page, "state.settings.poll === 27", 5), "left, it saves what it holds"
    finally:
        page.evaluate("(v) => { setSetting('poll', v); applySettings(); }", was)
    show_view(page, "tiles")
