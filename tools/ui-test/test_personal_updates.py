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
from helpers import frames, hold, mark, open_main_menu, owner_page, replaced, show_view, undo, wait_held, wait_until
from test_admin_updates import menu_mark, out
from test_login import sign_in
from test_notifications import FAKE_PUSH, reload_with
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


# --- the Appearance card and the scheme editor ---------------------------

def open_editor(page):
    """My settings shown, the scheme editor open on the chosen scheme."""
    show_view(page, "prefs")
    if page.locator("#scheme-customise").get_attribute("aria-expanded") != "true":
        page.locator("#scheme-customise").click()
    page.locator("#scheme-name").wait_for(state="visible")


def test_the_scheme_editor_keeps_an_edit_when_the_settings_arrive(page, case):
    """In the scheme editor a name typed and a hex value half typed, the
    focus in it; the user's settings arrive, as at a sign-in
    (loadPreferences applies them again): the editor's fields are the
    elements they were, with what is typed and the focus; Close drops
    the draft."""
    open_editor(page)
    page.locator("#scheme-name").fill("Typed scheme name")
    hexf = page.locator("#scheme-light-line-hex")
    hexf.fill("#12")  # not a colour yet: judged when the field is left
    sels = ["#scheme-name", "#scheme-light-line-hex", "#scheme-light-bg", "#scheme-delete"]
    mark(page, sels)
    page.evaluate("() => applyStoredSettings()")
    frames(page)
    assert replaced(page, sels) == [], f"the same elements: {replaced(page, sels)}"
    assert page.locator("#scheme-name").input_value() == "Typed scheme name", "the name typed kept"
    assert hexf.input_value() == "#12", "the hex half typed kept"
    assert page.evaluate("document.activeElement.id") == "scheme-light-line-hex", "the focus where it was"
    page.locator("#scheme-name").focus()  # the hex left: refused, its line shown, before Close is aimed at
    assert wait_until(page, "document.getElementById('scheme-light-line-hex').getAttribute('aria-invalid') === 'true'", 5), "#12 refused once left"
    page.locator("#scheme-editor .scheme-close").click()
    assert page.locator("#scheme-editor").is_hidden() and not page.evaluate("!!themeState.draft"), "Close drops the draft"
    assert not page.evaluate("MassimoParts.leaving.changes()"), "and its form"
    show_view(page, "tiles")


def test_the_scheme_editor_says_unsaved(page, case):
    """A colour changed in the editor and the editor left: My settings'
    pill shows the disk naming it, ☰ the mark and its Personal settings
    the disk; the pill's item, from a folded card, brings the editor into
    sight with the focus in it; Reset puts the colour back and clears them."""
    open_editor(page)
    was = page.locator("#scheme-light-bg").input_value()
    page.locator("#scheme-light-bg").fill("#112233")
    page.locator("#prefs .section-fold").focus()  # out of the editor: left
    assert wait_until(page, "!!document.querySelector('#prefs .section-pill .problems')", 5), "the card's pill"
    assert menu_mark(page) == "tone-warn", "☰ amber"
    page.locator("#prefs .section-fold").click()
    assert wait_until(page, "document.getElementById('prefs').classList.contains('folded')", 5), "the card folded"
    show_view(page, "tiles")
    open_main_menu(page)
    assert page.locator("#menu-me .menu-unsaved.tone-warn").count() == 1, "Personal settings with the disk"
    page.locator("#menu-me").click()
    pill = page.locator("#prefs .section-pill .problems")
    pill.locator("summary").click()
    want = page.evaluate("t('form.unsavedItem', {form: t('appearance.editorTitle')})")
    action = pill.locator(".problem-action")
    assert action.inner_text().strip() == want, f"the item names the editor: {action.inner_text()!r}"
    action.click()
    assert wait_until(page, "!document.getElementById('prefs').classList.contains('folded')", 5), "the card unfolded"
    assert wait_until(page, "!!document.activeElement && !!document.activeElement.closest('#scheme-editor')", 5), "the focus in the editor"
    assert page.locator("#scheme-light-bg").input_value() == "#112233", "the colour kept"
    page.locator("#scheme-editor .scheme-reset").click()
    assert page.locator("#scheme-light-bg").input_value() == was, "Reset puts it back"
    assert wait_until(page, "!document.querySelector('#menu-me .menu-unsaved')", 5), "no disk"
    assert menu_mark(page) == "", "nothing on ☰"
    page.locator("#scheme-editor .scheme-close").click()
    show_view(page, "tiles")


def drop_scheme(page, name):
    """The user's scheme of name gone from the browser, the default chosen."""
    page.evaluate("(n) => { themeState.custom = themeState.custom.filter((c) => c.name !== n); storeCustomSchemes(); chooseScheme('massimo'); }", name)


def test_save_as_and_enter_save_only_a_change(page, case):
    """The editor opened on the chosen scheme holds no change: Save as is
    off and Enter in the name saves nothing (D204). A colour changed and a
    name typed: Enter saves as Save as does, the scheme the user's own and
    chosen, Save as with its check, and nothing left unsaved."""
    name = f"Saved by Enter {case.width}-{case.lang}"
    open_editor(page)
    save = page.locator("#scheme-editor .scheme-name button")
    assert save.is_disabled(), "Save as off while nothing differs"
    page.locator("#scheme-name").press("Enter")
    frames(page)
    assert page.locator("#scheme-msg").is_hidden() and not page.evaluate("themeState.custom.some((c) => c.name.startsWith('Saved by Enter'))"), \
        "Enter saved nothing"
    page.locator("#scheme-light-bg").fill("#112233")
    page.locator("#scheme-name").fill(name)
    assert save.is_enabled(), "Save as on with a change"
    try:
        page.locator("#scheme-name").press("Enter")
        assert wait_until(page, "state.settings.scheme === %s" % json.dumps("custom:" + name), 5), "saved and chosen"
        assert page.evaluate("(n) => themeState.custom.some((c) => c.name === n && c.light.bg === '#112233')", name), "the user's own, with its colour"
        assert save.locator(".ok-check").count() == 1 and save.is_disabled(), "Save as with its check, off"
        assert not page.evaluate("MassimoParts.leaving.changes()"), "nothing left to lose"
        assert page.locator("#prefs .section-pill .problems").count() == 0, "no disk on the card"
    finally:
        page.locator("#scheme-editor .scheme-close").click()
        drop_scheme(page, name)
    show_view(page, "tiles")


def test_another_scheme_or_an_import_drops_the_draft(page, case, tmp_path):
    """A colour changed in the editor: choosing another scheme closes the
    editor and drops the draft and its form; so does an import, which
    chooses the scheme imported."""
    open_editor(page)
    page.locator("#scheme-light-bg").fill("#112233")
    assert wait_until(page, "MassimoParts.leaving.changes()", 5), "a change"
    page.locator("#scheme-select").select_option("contrast")
    assert page.locator("#scheme-editor").is_hidden() and not page.evaluate("!!themeState.draft"), "another scheme: the editor closed, the draft gone"
    assert not page.evaluate("MassimoParts.leaving.changes()"), "and its form"
    open_editor(page)
    page.locator("#scheme-light-bg").fill("#223344")
    assert wait_until(page, "MassimoParts.leaving.changes()", 5), "a change again"
    name = f"Imported {case.width}-{case.lang}"
    doc = page.evaluate("(n) => Object.assign(schemeDoc(schemeById('massimo')), { name: n })", name)
    f = tmp_path / "scheme.json"
    f.write_text(json.dumps(doc))
    try:
        page.locator("#scheme-file").set_input_files(str(f))
        assert wait_until(page, "state.settings.scheme === %s" % json.dumps("custom:" + name), 5), "the import chosen"
        assert page.locator("#scheme-editor").is_hidden() and not page.evaluate("!!themeState.draft"), "an import: the editor closed, the draft gone"
        assert not page.evaluate("MassimoParts.leaving.changes()"), "and its form"
    finally:
        drop_scheme(page, name)
    show_view(page, "tiles")


@pytest.mark.parametrize("how", ["sign-out", "expired"])
def test_the_next_user_finds_no_scheme_being_edited(new_context, case, watch, guard, how):
    """The owner's scheme editor open with a colour changed, which the page
    shows; the session ends, and the operator signed in finds the editor
    closed, the page in its own colours and nothing left to lose."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    open_editor(page)
    page.locator("#scheme-light-bg").fill("#112233")
    across(page, how)
    frames(page)
    assert not page.evaluate("!!themeState.draft"), "no draft left"
    assert page.evaluate("getComputedStyle(document.documentElement).getPropertyValue('--bg').trim()") != "#112233", "the page not in the draft's colours"
    show_view(page, "prefs")
    assert page.locator("#scheme-editor").is_hidden(), "the editor closed"
    assert not page.evaluate("MassimoParts.leaving.changes()"), "nothing left to lose"


# --- Notifications ---------------------------------------------------------

WINDOW = "#notify-rules .notify-row[data-kind=window_open] input[type=number]"


def push_read(page):
    """A wait for the card's read of the devices (GET …/me/push)."""
    return page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith("/api/v1/me/push"))


def notify_shown(page):
    """Notifications shown, its rules and devices read."""
    with push_read(page):
        show_view(page, "notify")
    page.locator(WINDOW).wait_for()


def test_a_rule_being_written_stays_when_shown_again(page, case):
    """A window's delay typed out of bounds, refused on its field and not
    sent, the focus in it: the personal page shown again reads the rules
    and devices again, and the field is the element it was, with what is
    typed, its mark and the range before it; the name of this device typed
    stays too, in a browser that can be told (conftest's fake push).
    Nothing is sent."""
    reload_with(page, FAKE_PUSH)
    notify_shown(page)
    box = page.locator(WINDOW)
    box.fill("2000")
    box.dispatch_event("change")
    row = page.locator("#notify-rules .notify-row[data-kind=window_open]")
    row.locator(".warn-text").wait_for(state="visible")
    label = page.locator("#notify-label")
    label.fill("Typed device name")
    box.focus()
    sels = [WINDOW, "#notify-rules .notify-row[data-kind=frost] input[type=checkbox]", "#notify-label"]
    mark(page, sels)
    writes = []
    page.on("request", lambda r: writes.append(r.url) if r.method != "GET" and "/api/v1/me/" in r.url else None)
    with push_read(page):
        page.evaluate("() => document.dispatchEvent(new CustomEvent('viewshown', { detail: 'me' }))")  # what showing the page does
    frames(page)
    assert replaced(page, sels) == [], f"the same elements after the read: {replaced(page, sels)}"
    assert box.input_value() == "2000", "the delay typed kept"
    assert box.get_attribute("aria-invalid") == "true" and row.locator(".warn-text").is_visible(), "still marked, the range before it"
    assert page.evaluate("document.activeElement === document.querySelector(%r)" % WINDOW), "the focus where it was"
    assert label.input_value() == "Typed device name", "the name typed kept"
    assert writes == [], f"nothing sent: {writes}"
    box.fill("30")
    box.dispatch_event("change")
    row.locator(".warn-text").wait_for(state="hidden")
    show_view(page, "tiles")


@pytest.mark.parametrize("how", ["sign-out", "expired"])
def test_the_next_user_finds_no_device_name_typed(new_context, case, watch, guard, how):
    """The owner's name of this device typed, in a browser that can be
    told (the fake push); the session ends, and the operator signed in
    finds the field empty."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    reload_with(page, FAKE_PUSH)
    notify_shown(page)
    label = page.locator("#notify-label")
    label.fill("Typed by the first")
    across(page, how)
    notify_shown(page)
    assert label.input_value() == "", "the name typed is gone"
    nothing_of_the_first(page)


OWNERS_DEVICE = [{"id": 990001, "endpoint": "http://127.0.0.1:9/push/ui-test/owner", "label": "The first user's phone",
                  "created_at": "2026-11-20T08:00:00Z", "failures": 0, "current": False}]


@pytest.mark.parametrize("how", ["sign-out", "expired"])
def test_a_late_read_of_the_devices_reaches_nothing_of_the_next_user(new_context, case, watch, guard, how):
    """The owner's devices read for the card and held while the session
    ends; answered once the operator is in, with a device of the owner's,
    it draws nothing of it."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    held = []
    hold(page, "**/api/v1/me/push", "GET", held, [])
    try:
        show_view(page, "notify")
        assert wait_held(page, held), "the devices asked for"
        across(page, how)
        with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith("/api/v1/me/push")) as answer:
            for r in held:
                r.fulfill(status=200, content_type="application/json", body=json.dumps(OWNERS_DEVICE))
        answer.value.finished()  # its body read by the page too
        frames(page)
        frames(page)  # the read's own drawing, where it drew one
        assert page.locator(".notify-device", has_text="The first user's phone").count() == 0, "no device of the first user"
    finally:
        page.unroute("**/api/v1/me/push")


STALE = "#notify-rules .notify-row[data-kind=device_stale] input[type=checkbox]"


def notify_puts(page):
    """The page's PUTs of …/me/preferences/notify from now on: (the token it carried, its body)."""
    puts = []
    page.on("request", lambda r: puts.append((r.headers.get("authorization", ""), r.post_data_json))
            if r.method == "PUT" and r.url.endswith("/api/v1/me/preferences/notify") else None)
    return puts


def forget_rules():
    undo("the owner's notify preference", lambda: api_call("DELETE", "api/v1/me/preferences/notify"))


def pause(page):
    """Playwright's clock paused at the page's now: the rule's second does
    not pass by itself. Under it the test drives the page by evaluate and
    by Python's own waits (wait_until), none of which waits on a timer or
    a frame of the page's."""
    page.clock.pause_at(page.evaluate("Date.now()") + 1)


def flip_stale(page):
    """The device_stale rule's switch flipped, as a click does, without the
    click's own waits for the page's frames."""
    page.locator(STALE).evaluate("(i) => i.click()")


def test_a_rule_changed_just_before_sign_out_is_sent_first(new_context, case, watch, guard):
    """A rule changed and Sign out within its second (D205), the clock
    paused so that the second cannot pass by itself: the change is sent
    once, under the owner's token, as Sign out ends the session; the
    operator signed in afterwards, the clock running again, gets no PUT of
    it."""
    page = owner_page(new_context, guard, watch, URL, TOKEN, clock=True)
    try:
        notify_shown(page)
        puts = notify_puts(page)
        pause(page)
        flip_stale(page)
        page.evaluate("() => { logout(); }")  # Sign out, not waited for: it ends in the sign-in dialog
        assert wait_until(page, "document.getElementById('login-dialog').open", 5), "signed out"
        assert len(puts) == 1 and puts[0][0] == "Bearer " + TOKEN, f"one PUT, under the owner's token: {[p[0][:12] for p in puts]}"
        assert puts[0][1]["device_stale"]["on"] is True, f"the change itself: {puts[0][1]}"
        page.clock.resume()
        sign_in(page, "operator", "op-pass")  # waits a second after the stream opens: a save left waiting would have gone by then
        assert len(puts) == 1, f"no PUT for the operator: {len(puts)}"
    finally:
        forget_rules()


def test_a_rule_changed_before_another_users_sign_in_is_dropped(new_context, case, watch, guard):
    """A rule changed, the session taken as expired within its second (the
    clock paused until the sign-in has opened), the operator signed in:
    nothing of it is sent, and the operator's card shows the rules the
    server holds for the operator."""
    page = owner_page(new_context, guard, watch, URL, TOKEN, clock=True)
    try:
        notify_shown(page)
        puts = notify_puts(page)
        pause(page)  # the rule's second cannot end before the 401 opens the sign-in
        flip_stale(page)
        expire(page)  # the sign-in open: holdPushes has taken the rule's save
        page.clock.resume()
        sign_in(page, "operator", "op-pass")
        with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith("/api/v1/me/preferences")) as prefs:
            notify_shown(page)
        held = ((prefs.value.json() or {}).get("notify") or {}).get("device_stale", {}).get("on", False)
        assert puts == [], f"nothing sent: {puts}"
        assert page.locator(STALE).is_checked() == held, "the operator's own rule shown"
    finally:
        forget_rules()


@pytest.mark.clock
def test_a_read_that_finds_a_change_waiting_keeps_it(page, case):
    """The personal page shown again, the rules' read held; the clock
    paused and a rule changed, its save waiting; the read answers: it keeps
    the rule as changed and sends nothing; the second run on, the change
    sent is it."""
    notify_shown(page)
    try:
        was = page.locator(STALE).is_checked()
        held = []
        hold(page, "**/api/v1/me/preferences", "GET", held, [])
        puts = notify_puts(page)
        page.evaluate("() => document.dispatchEvent(new CustomEvent('viewshown', { detail: 'me' }))")  # what showing the page does
        assert wait_held(page, held), "the rules asked for"
        pause(page)  # the save's second cannot pass before the read has answered
        flip_stale(page)
        with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith("/api/v1/me/preferences")) as answer:
            for r in held:
                r.continue_()
        answer.value.finished()
        page.clock.run_for(100)  # the read's drawing, not the save
        assert page.locator(STALE).is_checked() != was, "the change kept through the read"
        assert puts == [], f"nothing sent before the second: {puts}"
        with page.expect_request(lambda r: r.method == "PUT" and r.url.endswith("/api/v1/me/preferences/notify")):
            page.clock.run_for(1000)
        assert len(puts) == 1 and puts[0][1]["device_stale"]["on"] is (not was), f"the change sent: {puts}"
    finally:
        page.clock.resume()
        page.unroute("**/api/v1/me/preferences")
        forget_rules()
    show_view(page, "tiles")


STORED = {"notify": {"window_open": {"on": True, "after_min": 45}, "device_stale": {"on": True}}}


def test_the_rules_wait_for_their_first_read(new_context, case, watch, guard):
    """The card shown, its first read of the rules held: the switches and
    the values are off, so that nothing is changed or sent before it; the
    read answered, they show the rules stored and are on."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    held = []
    hold(page, "**/api/v1/me/preferences", "GET", held, [])
    puts = notify_puts(page)
    try:
        show_view(page, "notify")
        assert wait_held(page, held), "the rules asked for"
        page.locator(STALE).wait_for(state="attached")
        assert page.locator("#notify-rules input[type=checkbox]:not([disabled])").count() == 0, "every switch off"
        assert page.locator("#notify-rules input[type=number]:not([disabled])").count() == 0, "every value off"
        with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith("/api/v1/me/preferences")) as answer:
            for r in held:
                r.fulfill(status=200, content_type="application/json", body=json.dumps(STORED))
        answer.value.finished()
        frames(page)
        frames(page)
        assert page.locator(STALE).is_checked() and page.locator(STALE).is_enabled(), "the stored rule, on"
        assert page.locator(WINDOW).input_value() == "45", "the stored delay"
        assert puts == [], f"nothing sent: {puts}"
    finally:
        page.unroute("**/api/v1/me/preferences")
