# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings' lists follow the data in place (AP UiParts, Block 10):
a draft typed in one panel stays, the element it was with what is typed
and the focus, through an action in another panel and a reload of the
list, and so does the new item's form; what a user's settings form holds
not saved shows as the disk in the section's pill, which opens it, and on
☰ and its System settings, the worse of it and the log's (AP UnsavedMarks,
Block 5); a user gone from the server forgets its form. At a sign-out,
and for another user signed in after an expired session, nothing typed,
no list and no key shown once is left. The users the tests make are
removed again, and nothing typed is saved."""
import json

import pytest

from conftest import TOKEN, URL, api_call
from helpers import frames, hold, mark, open_main_menu, owner_page, reads_of, replaced, safe_route, show_view, sign_out, undo, wait_held, wait_until
from test_log_alerts import alerts_route
from test_login import sign_in
from test_oidc import provider  # noqa: F401, the fixture
from test_unsaved import DIALOG, expire

OP = ".user-panel[data-name='operator']"


def open_panel(page, sel):
    """System settings shown and the panel of sel open; the panel."""
    show_view(page, "system")
    panel = page.locator(sel)
    panel.wait_for()
    if not panel.evaluate("(d) => d.open"):
        panel.locator("> summary").click()
    return panel


def make_user(name):
    """A viewer the test makes, gone first where a run before left it."""
    undo(f"the user {name}", lambda: api_call("DELETE", "api/v1/users/" + name))
    api_call("POST", "api/v1/users", {"name": name, "role": "viewer", "password": "admin-pass-1"})


def remove_user(name):
    undo(f"the user {name}", lambda: api_call("DELETE", "api/v1/users/" + name))


def reload_users(page):
    """The list read again, as a change made in My profile does (users.js loadUsers)."""
    page.evaluate("loadUsers()")
    frames(page)


def leave(page):
    """The focus out of the form, onto the section's Add a user: the form is left."""
    page.locator("#user-add").focus()
    frames(page)


def test_a_users_draft_stays_through_another_panel_and_a_reload(page, case):
    """The operator's display name typed and its role chosen, not saved:
    a save in another user's panel and a reload of the list keep the
    operator's panel, its fields and Save the elements they were, with
    what is typed; the focus in the field stays through the reload. The
    new user's form, with a name typed, stays too."""
    other = f"adm-{case.width}-{case.lang}"
    make_user(other)
    try:
        panel = open_panel(page, OP)
        name = panel.locator("input[name=display_name]")
        typed = f"typed {case.width}"
        name.fill(typed)
        panel.locator("select[name=role]").select_option("guest")
        page.locator("#user-add").click()
        page.locator("#user-new input[name=name]").fill("not-sent")
        sels = [OP + " input[name=display_name]", OP + " select[name=role]", OP + " button[type=submit]", "#user-new input[name=name]"]
        mark(page, sels)
        name.focus()
        reload_users(page)
        assert replaced(page, sels) == [], f"the same elements after a reload: {replaced(page, sels)}"
        assert page.evaluate("document.activeElement.name") == "display_name", "the focus where it was"
        assert name.input_value() == typed and panel.locator("select[name=role]").input_value() == "guest", "what is typed kept"
        mine = open_panel(page, f".user-panel[data-name='{other}']")
        mine.locator("input[name=display_name]").fill("Saved here")
        mine.locator("button[type=submit]").click()
        assert wait_until(page, "document.getElementById('users-msg').classList.contains('ok')", 5), "the other user saved"
        assert wait_until(page, f"document.querySelector(\".user-panel[data-name='{other}'] > summary .panel-name\").textContent === 'Saved here'",
                          5), "and shown under its new name"
        assert replaced(page, sels) == [], f"the same elements after a save in another panel: {replaced(page, sels)}"
        assert name.input_value() == typed and page.locator("#user-new input[name=name]").input_value() == "not-sent", "what is typed kept"
        panel.locator(".form-reset").click()
        stored = (api_call("GET", "api/v1/users/operator") or {}).get("display_name") or ""
        assert name.input_value() == stored, "Reset takes the stored name back"
        page.locator("#user-new .form-cancel").click()
    finally:
        remove_user(other)
    show_view(page, "tiles")


def test_a_users_pill_and_the_menu_say_unsaved(page, case, watch):
    """The operator's display name typed and the form left: the section's
    pill shows the disk naming the user, ☰ the amber mark and its System
    settings the disk; the pill's item opens the panel, closed meanwhile,
    with the focus in the form. A save that fails turns all three red, and
    a warning of the log beside it leaves ☰ red; Reset clears them, and
    the log's warning alone leaves ☰ amber."""
    alerts = []
    alerts_route(page, alerts)
    page.evaluate("loadLogAlerts()")
    panel = open_panel(page, OP)
    name = panel.locator("input[name=display_name]")
    name.fill("Unsaved name")
    leave(page)
    pill = page.locator("#sys-users .section-pill .problems")
    assert wait_until(page, "!!document.querySelector('#sys-users .section-pill .problems')", 5), "the section's pill"
    want = page.evaluate("(n) => t('form.unsavedItem', {form: t('users.formName', {name: n})})",
                         page.locator(OP + " > summary .panel-name").inner_text())
    menu = "#nav-menu > summary .mark-sign"
    item = "#menu-system .menu-unsaved"
    assert "tone-warn" in (page.locator(menu).get_attribute("class") or ""), "☰ amber"
    assert "tone-warn" in (page.locator(item).get_attribute("class") or ""), "System settings with the disk"
    assert page.evaluate("t('problem.unsaved')") in (page.locator("#nav-menu > summary").get_attribute("aria-label") or ""), "said in ☰'s name"
    panel.locator("> summary").click()
    assert not panel.evaluate("(d) => d.open"), "the panel closed"
    pill.locator("summary").click()
    action = pill.locator(".problem-action")
    assert action.inner_text().strip() == want, f"the item names the form: {action.inner_text()!r}"
    action.click()
    assert wait_until(page, "document.querySelector(%r).open" % OP, 5), "the item opens the panel"
    assert wait_until(page, "document.activeElement && document.activeElement.closest(%r)" % (OP + " form"), 5), "the focus in its form"
    assert name.input_value() == "Unsaved name", "what is typed kept"

    page.route("**/api/v1/users/operator", lambda r: r.fulfill(status=500, content_type="application/json", body='{"error":"refused"}')
               if r.request.method == "PUT" else r.continue_())
    try:
        panel.locator("button[type=submit]").click()
        assert wait_until(page, "document.querySelector(%r)?.classList.contains('tone-bad')" % menu, 5), "a failed save: ☰ red"
        assert "tone-bad" in (page.locator(item).get_attribute("class") or ""), "and System settings' disk"
        alerts.append({"id": 900101, "kind": "disconnected", "level": "warn", "cube_serial": "KEQ0000100", "time": "2026-11-14T20:00:00Z"})
        page.evaluate("loadLogAlerts()")
        frames(page)
        assert "tone-bad" in (page.locator(menu).get_attribute("class") or ""), "the worse of the two: red"
    finally:
        page.unroute("**/api/v1/users/operator")
    watch.problems[:] = [p for p in watch.problems if "500" not in p]
    panel.locator(".form-reset").click()
    assert wait_until(page, "!document.querySelector('#sys-users .section-pill .problems')", 5), "Reset: no pill"
    assert page.locator(item).count() == 0, "no disk on System settings"
    assert "tone-warn" in (page.locator(menu).get_attribute("class") or ""), "☰ amber for the log alone"
    alerts.clear()
    page.evaluate("loadLogAlerts()")
    assert wait_until(page, "!document.querySelector(%r)" % menu, 5), "nothing on ☰"
    show_view(page, "tiles")


def test_a_user_gone_forgets_its_form(page, case):
    """A user removed by another client while its form holds a change: the
    next load of the list drops its panel and its disk."""
    other = f"gone-{case.width}-{case.lang}"
    make_user(other)
    try:
        panel = open_panel(page, f".user-panel[data-name='{other}']")
        panel.locator("input[name=display_name]").fill("Typed and gone")
        leave(page)
        assert wait_until(page, "!!document.querySelector('#sys-users .section-pill .problems')", 5), "its disk"
        api_call("DELETE", "api/v1/users/" + other)
        reload_users(page)
        assert wait_until(page, f"!document.querySelector(\".user-panel[data-name='{other}']\")", 5), "its panel gone"
        assert wait_until(page, "!document.querySelector('#sys-users .section-pill .problems')", 5), "and its disk"
        assert not page.evaluate("MassimoParts.leaving.changes()"), "nothing left to lose"
    finally:
        remove_user(other)
    show_view(page, "tiles")


def type_everything(page, label):
    """In the operator's panel a display name and a new password typed, a
    key made, which the page shows once, and the new user's form with a
    name; the key's text."""
    panel = open_panel(page, OP)
    panel.locator("input[name=display_name]").fill("Typed by the first")
    panel.locator("input[name=password]").fill("secret-of-the-first")
    panel.locator("input[name=label]").fill(label)
    panel.locator(".create-key").click()
    assert wait_until(page, "!document.querySelector(%r).classList.contains('hidden')" % (OP + " .user-key-box"), 5), "the key shown"
    key = panel.locator(".user-key").inner_text().strip()
    assert key, "a key"
    page.locator("#user-add").click()
    page.locator("#user-new input[name=name]").fill("typed-new-user")
    return key


def nothing_left(page, key):
    """Nothing of the first user's lists in the page: no panel, no form,
    nothing typed, not the key."""
    frames(page)
    html = page.evaluate("document.body.innerHTML")
    for what in ("Typed by the first", "typed-new-user", key):
        assert what not in html, f"{what!r} is gone"
    assert page.evaluate("[...document.querySelectorAll('input')].every((i) => i.value !== 'secret-of-the-first')"), "the password typed is gone"
    assert page.locator(".user-panel").count() == 0 and page.locator("#user-new form").count() == 0, "no list, no new user's form"
    assert not page.evaluate("MassimoParts.leaving.changes()"), "no draft"


def revoke(label):
    for tk in api_call("GET", "api/v1/users/operator/tokens") or []:
        if tk.get("label") == label:
            api_call("DELETE", "api/v1/users/operator/tokens/%d" % tk["id"])


def test_sign_out_forgets_the_users(new_context, case, watch, guard):
    """The owner's page with a display name and a password typed, a key
    shown once and the new user's form: Sign out, and the operator signed
    in finds none of it."""
    label = f"ui-test-signout-{case.width}-{case.lang}"
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    try:
        key = type_everything(page, label)
        sign_out(page)
        assert wait_until(page, "!!document.querySelector(%s)?.open" % json.dumps(DIALOG), 5), "the dialog asks first"
        page.locator(DIALOG + " .dialog-go").click()
        sign_in(page, "operator", "op-pass")
        nothing_left(page, key)
    finally:
        revoke(label)


def test_another_user_after_a_401_finds_no_users(new_context, case, watch, guard):
    """The same, the session taken as expired: the operator signed in
    instead finds none of it."""
    label = f"ui-test-expired-{case.width}-{case.lang}"
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    try:
        key = type_everything(page, label)
        expire(page)
        sign_in(page, "operator", "op-pass")
        nothing_left(page, key)
    finally:
        revoke(label)


ROLE_OP = ".role-panel[data-key='operator']"


def make_role(key):
    """A role the test makes with read alone, gone first where a run before left it."""
    remove_role(key)
    api_call("POST", "api/v1/roles", {"key": key, "capabilities": ["read"]})


def remove_role(key):
    undo(f"the role {key}", lambda: api_call("DELETE", "api/v1/roles/" + key))


def test_a_roles_draft_stays_through_another_panel_and_a_reload(page, case):
    """The operator role's name typed and a capability switched, not saved:
    a save in another role's panel and System settings shown again keep
    the panel, its fields and Save the elements they were, with what is
    changed; the form left, the section's pill shows its disk; Reset
    clears it. The new role's form, with a key typed, stays too."""
    other = f"adm{case.width}{case.lang}"
    make_role(other)
    try:
        panel = open_panel(page, ROLE_OP)
        panel.locator("input[name=name]").fill("Typed role")
        switch = panel.locator(".cap-grid .switch").last
        was = switch.locator("input").is_checked()
        switch.click()
        page.locator("#role-add").click()
        page.locator("#role-new input[name=key]").fill("notsent")
        sels = [ROLE_OP + " input[name=name]", ROLE_OP + " .cap-grid .switch:last-child input", ROLE_OP + " button[type=submit]", "#role-new input[name=key]"]
        mark(page, sels)
        mine = open_panel(page, f".role-panel[data-key='{other}']")
        mine.locator("input[name=name]").fill("Saved role")
        mine.locator("button[type=submit]").click()
        assert wait_until(page, "document.getElementById('roles-msg').classList.contains('ok')", 5), "the other role saved"
        show_view(page, "tiles")
        show_view(page, "system")
        assert wait_until(page, f"document.querySelector(\".role-panel[data-key='{other}'] > summary .panel-name\").textContent === 'Saved role'", 5), \
            "the other role under its new name"
        assert replaced(page, sels) == [], f"the same elements: {replaced(page, sels)}"
        assert panel.locator("input[name=name]").input_value() == "Typed role", "the name typed kept"
        assert switch.locator("input").is_checked() != was, "the capability switched kept"
        assert page.locator("#role-new input[name=key]").input_value() == "notsent", "the new role's key kept"
        assert wait_until(page, "!!document.querySelector('#sys-roles .section-pill .problems')", 5), "the roles' pill shows its disk"
        panel.locator(".form-reset").click()
        assert switch.locator("input").is_checked() == was, "Reset takes the stored set back"
        assert wait_until(page, "!document.querySelector('#sys-roles .section-pill .problems')", 5), "and the pill goes"
        page.locator("#role-new .form-cancel").click()
    finally:
        remove_role(other)
    show_view(page, "tiles")


def test_a_capability_refused_shows_it_as_stored(page, case, watch):
    """A capability switched for every role, its PUT refused: the switch
    shows the user's position while the request runs and the capability as
    stored after it, and the roles' line says why. Nothing is stored."""
    held, sent = [], []
    page.route("**/api/v1/capabilities/*", lambda r: (sent.append(r), held.append(r)))
    try:
        show_view(page, "system")
        switch = page.locator("#caps-list .switch").first
        switch.wait_for()
        was = switch.locator("input").is_checked()
        switch.click()
        assert wait_held(page, held) and len(sent) == 1, "the PUT is sent"
        assert switch.locator("input").is_checked() != was, "the user's position while it runs"
        held[0].fulfill(status=403, content_type="application/json", body='{"error":"refused here"}')
        assert wait_until(page, "document.getElementById('roles-msg').classList.contains('bad')", 5), "the roles' line says it failed"
        assert switch.locator("input").is_enabled() and switch.locator("input").is_checked() == was, "shown as stored"
    finally:
        page.unroute("**/api/v1/capabilities/*")
    watch.problems[:] = [p for p in watch.problems if "403" not in p]
    show_view(page, "tiles")


def test_a_providers_draft_stays_and_its_secret_is_kept_nowhere(page, case):
    """A provider made off by the test: its name and a client secret typed
    in its panel, not saved, stay through System settings shown again,
    the elements they were; the form left, the section's pill shows its
    disk; Reset clears both."""
    key = f"adm{case.width}{case.lang}"
    undo(f"the provider {key}", lambda: api_call("DELETE", f"api/v1/providers/{key}"))
    api_call("POST", "api/v1/providers", {"key": key, "name": "Admin test", "issuer": "https://issuer.example.org", "client_id": "x", "enabled": False})
    sel = f".provider-panel[data-key='{key}']"
    try:
        panel = open_panel(page, sel)
        panel.locator("input[name=name]").fill("Typed provider")
        panel.locator("input[name=client_secret]").fill("secret-typed-here")
        sels = [sel + " input[name=name]", sel + " input[name=client_secret]", sel + " button[type=submit]"]
        mark(page, sels)
        page.locator("#provider-add").focus()
        show_view(page, "tiles")
        show_view(page, "system")
        assert wait_until(page, "!!document.querySelector('#sys-providers .section-pill .problems')", 5), "the providers' pill shows its disk"
        assert replaced(page, sels) == [], f"the same elements: {replaced(page, sels)}"
        assert panel.locator("input[name=name]").input_value() == "Typed provider", "the name typed kept"
        assert panel.locator("input[name=client_secret]").input_value() == "secret-typed-here", "the secret typed kept in its field"
        panel.locator(".form-reset").click()
        assert panel.locator("input[name=client_secret]").input_value() == "", "Reset clears the secret"
        assert wait_until(page, "!document.querySelector('#sys-providers .section-pill .problems')", 5), "and the pill goes"
    finally:
        undo(f"the provider {key}", lambda: api_call("DELETE", f"api/v1/providers/{key}"))
    show_view(page, "tiles")


def out(page):
    """Sign out, through the leaving dialog where a form has changes."""
    sign_out(page)
    assert wait_until(page, "document.getElementById('login-dialog').open || !!document.querySelector(%s)?.open" % json.dumps(DIALOG), 5), \
        "signed out or asked first"
    if page.evaluate("!!document.querySelector(%s)?.open" % json.dumps(DIALOG)):
        page.locator(DIALOG + " .dialog-go").click()


def menu_mark(page):
    """☰'s mark's tone, "" for none."""
    return page.evaluate("(() => { const m = document.querySelector('#nav-menu > summary .mark-sign'); "
                         "return m ? ([...m.classList].find((c) => c.startsWith('tone-')) || '') : ''; })()")


def test_cube_settings_unsaved_mark_the_menu(page, case):
    """UnsavedMarks D24: a Cube's name typed on Cube settings and the form
    left, Rooms shown: ☰ carries the amber mark and its Cube settings the
    disk; Reset clears both."""
    show_view(page, "cube")
    field = page.locator("#cube-page-body .cube-form input[name=name]")
    field.fill("Typed Cube name")
    # the focus on Reset is still inside the form: nothing is left yet; Rooms
    # shown hides Cube settings, which takes the focus out of the form: left
    page.locator("#cube-page-body .cube-form .form-reset").focus()
    show_view(page, "tiles")
    assert wait_until(page, "!!document.querySelector('#menu-cube .menu-unsaved.tone-warn')", 5), "Cube settings with the disk"
    assert menu_mark(page) == "tone-warn", "☰ amber"
    show_view(page, "cube")
    page.locator("#cube-page-body .cube-form .form-reset").click()
    assert wait_until(page, "!document.querySelector('#menu-cube .menu-unsaved')", 5), "Reset: no disk"
    assert menu_mark(page) == "", "nothing on ☰"
    show_view(page, "tiles")


def test_the_pill_opens_a_folded_card(page, case):
    """A1: the operator's display name typed and left, the card folded,
    Rooms and System settings shown again: the pill's item unfolds the
    card and opens the panel with the focus in its form."""
    panel = open_panel(page, OP)
    name = panel.locator("input[name=display_name]")
    name.fill("Typed and folded")
    leave(page)
    page.locator("#sys-users .section-fold").click()
    assert wait_until(page, "document.getElementById('sys-users').classList.contains('folded')", 5), "the card folded"
    show_view(page, "tiles")
    show_view(page, "system")
    pill = page.locator("#sys-users .section-pill .problems")
    pill.locator("summary").click()
    pill.locator(".problem-action").click()
    assert wait_until(page, "!document.getElementById('sys-users').classList.contains('folded')", 5), "the card unfolded"
    assert wait_until(page, "document.activeElement && document.activeElement.closest(%r)" % (OP + " form"), 5), "the focus in its form"
    assert name.is_visible() and name.input_value() == "Typed and folded", "in sight, what is typed kept"
    panel.locator(".form-reset").click()
    show_view(page, "tiles")


def remove_question(page, panel, remove_sel, other_save, reload):
    """An item's Remove asking: it stays through a save in another panel,
    the list read again and System settings shown again; its Yes is
    there."""
    panel.locator(remove_sel).click()
    panel.locator(".confirm-box").wait_for()
    mark(page, [".confirm-box .confirm-yes"])
    other_save()
    reload()
    show_view(page, "tiles")
    show_view(page, "system")
    frames(page)
    assert panel.locator(".confirm-box").is_visible(), "the question stays"
    assert replaced(page, [".confirm-box .confirm-yes"]) == [], "the same question"


def deletes_of(page, sent):
    page.on("request", lambda r: sent.append(r.url) if r.method == "DELETE" else None)


def test_a_users_remove_question_stays_and_removes_that_user(page, case):
    """A4: a user's Remove asking stays through a save in another panel, a
    reload and System settings shown again; its Yes sends that user's
    DELETE and nothing else."""
    victim, other = f"rmq-{case.width}-{case.lang}", f"rmo-{case.width}-{case.lang}"
    make_user(victim)
    make_user(other)
    sent = []
    try:
        panel = open_panel(page, f".user-panel[data-name='{victim}']")
        mine = open_panel(page, f".user-panel[data-name='{other}']")

        def save_other():
            mine.locator("input[name=display_name]").fill("Saved meanwhile")
            mine.locator("button[type=submit]").click()
            assert wait_until(page, "document.getElementById('users-msg').classList.contains('ok')", 5), "the other saved"

        remove_question(page, panel, ".remove-user", save_other, lambda: reload_users(page))
        deletes_of(page, sent)
        # the list read after the DELETE, then the other's open panel's keys: waited for before the cleanup removes it
        with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith(f"/users/{other}/tokens")):
            panel.locator(".confirm-yes").click()
        assert wait_until(page, f"!document.querySelector(\".user-panel[data-name='{victim}']\")", 5), "the user removed"
        assert [u for u in sent if "/users/" in u] == [URL.rstrip("/") + f"/api/v1/users/{victim}"], f"its DELETE alone: {sent}"
    finally:
        remove_user(victim)
        remove_user(other)
    show_view(page, "tiles")


def test_a_roles_remove_question_stays_and_removes_that_role(page, case):
    """A4: a role's Remove asking, the same."""
    victim, other = f"rmq{case.width}{case.lang}", f"rmo{case.width}{case.lang}"
    make_role(victim)
    make_role(other)
    sent = []
    try:
        panel = open_panel(page, f".role-panel[data-key='{victim}']")
        mine = open_panel(page, f".role-panel[data-key='{other}']")

        def save_other():
            mine.locator("input[name=name]").fill("Saved meanwhile")
            mine.locator("button[type=submit]").click()
            assert wait_until(page, "document.getElementById('roles-msg').classList.contains('ok')", 5), "the other saved"

        remove_question(page, panel, ".remove-role", save_other, lambda: None)
        deletes_of(page, sent)
        with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith("/api/v1/roles")):  # the list read after the DELETE
            panel.locator(".confirm-yes").click()
        assert wait_until(page, f"!document.querySelector(\".role-panel[data-key='{victim}']\")", 5), "the role removed"
        assert [u for u in sent if "/roles/" in u] == [URL.rstrip("/") + f"/api/v1/roles/{victim}"], f"its DELETE alone: {sent}"
    finally:
        remove_role(victim)
        remove_role(other)
    show_view(page, "tiles")


def test_a_providers_remove_question_stays_and_removes_that_provider(page, case):
    """A4: a provider's Remove asking, the same; providers made off."""
    victim, other = f"rmq{case.width}{case.lang}", f"rmo{case.width}{case.lang}"
    for k in (victim, other):
        undo(f"the provider {k}", lambda k=k: api_call("DELETE", f"api/v1/providers/{k}"))
        api_call("POST", "api/v1/providers", {"key": k, "name": k, "issuer": "https://issuer.example.org", "client_id": "x", "enabled": False})
    sent = []
    try:
        panel = open_panel(page, f".provider-panel[data-key='{victim}']")
        mine = open_panel(page, f".provider-panel[data-key='{other}']")

        def save_other():
            mine.locator("input[name=name]").fill("Saved meanwhile")
            mine.locator("button[type=submit]").click()
            assert wait_until(page, "document.getElementById('providers-msg').classList.contains('ok')", 5), "the other saved"

        remove_question(page, panel, ".remove-provider", save_other, lambda: None)
        deletes_of(page, sent)
        with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith("/api/v1/providers")):  # the list read after the DELETE
            panel.locator(".confirm-yes").click()
        assert wait_until(page, f"!document.querySelector(\".provider-panel[data-key='{victim}']\")", 5), "the provider removed"
        assert [u for u in sent if "/providers/" in u] == [URL.rstrip("/") + f"/api/v1/providers/{victim}"], f"its DELETE alone: {sent}"
    finally:
        for k in (victim, other):
            undo(f"the provider {k}", lambda k=k: api_call("DELETE", f"api/v1/providers/{k}"))
    show_view(page, "tiles")


def test_a_remove_question_goes_with_its_user(page, case):
    """C2: a user's Remove asking, the user removed by another client: the
    list read again drops the panel, and no question and no Yes is left."""
    victim = f"rmg-{case.width}-{case.lang}"
    make_user(victim)
    try:
        panel = open_panel(page, f".user-panel[data-name='{victim}']")
        panel.locator(".remove-user").click()
        panel.locator(".confirm-box").wait_for()
        api_call("DELETE", "api/v1/users/" + victim)
        reload_users(page)
        assert wait_until(page, f"!document.querySelector(\".user-panel[data-name='{victim}']\")", 5), "its panel gone"
        assert page.locator("#sys-users-body .confirm-box, #sys-users-body .confirm-yes").count() == 0, "no question, no Yes"
    finally:
        remove_user(victim)
    show_view(page, "tiles")


def test_a_key_made_is_gone_at_the_next_read(page, case):
    """A12: a key just made shows once: the panel closed and opened again,
    which reads the keys again, shows it no more."""
    label = f"ui-test-once-{case.width}-{case.lang}"
    revoke(label)
    try:
        panel = open_panel(page, OP)
        panel.locator("input[name=label]").fill(label)
        panel.locator(".create-key").click()
        box = OP + " .user-key-box"
        assert wait_until(page, "!document.querySelector(%r).classList.contains('hidden')" % box, 5), "the key shown"
        panel.locator("> summary").click()
        panel.locator("> summary").click()
        assert wait_until(page, "document.querySelector(%r).classList.contains('hidden')" % box, 5), "gone at the next read"
        assert not panel.locator(".user-key").inner_text().strip(), "its text too"
    finally:
        revoke(label)
    show_view(page, "tiles")


def switch_saved_at_once(page, switch, pattern, then_refused):
    """A switch saved at once, its PUT held: Space shows the user's position
    while it runs and keeps the focus on the switch; a second Space sends
    nothing and changes nothing shown; refused, the stored position again."""
    held, sent = [], []
    hold(page, pattern, "PUT", held, sent)
    try:
        box = switch.locator("input")
        was = box.is_checked()
        box.focus()
        page.keyboard.press("Space")
        assert wait_held(page, held), "the PUT is sent"
        frames(page)
        assert box.is_checked() != was, "the user's position while it runs"
        assert box.evaluate("(e) => e === document.activeElement"), "the focus stays on the switch"
        page.keyboard.press("Space")
        frames(page)
        assert box.is_checked() != was and len(sent) == 1, f"a second press sends nothing and changes nothing shown: {len(sent)}"
        if then_refused:
            held[0].fulfill(status=403, content_type="application/json", body='{"error":"refused here"}')
            assert wait_until(page, f"document.activeElement && document.activeElement.checked === {str(was).lower()}", 5), "refused: as stored"
        else:
            held[0].continue_()
        assert box.evaluate("(e) => e === document.activeElement"), "the focus still on the switch"
    finally:
        page.unroute(pattern)


def test_a_capability_switch_keeps_the_focus(page, case, watch):
    """A5, B4: a capability for every role, refused: the position shown
    while it runs, the focus kept, the stored one after it. Nothing is
    stored."""
    show_view(page, "system")
    cap = page.locator("#caps-list .switch").first
    cap.wait_for()
    switch_saved_at_once(page, cap, "**/api/v1/capabilities/*", True)
    watch.problems[:] = [p for p in watch.problems if "403" not in p]
    show_view(page, "tiles")


def test_two_capabilities_switched_while_one_saves(page, case, watch):
    """C3: two capabilities switched one after the other while the first's
    PUT is held: both PUTs go out and both positions hold; refused, both
    show as stored again. Nothing is stored."""
    held, sent = [], []
    hold(page, "**/api/v1/capabilities/*", "PUT", held, sent)
    try:
        show_view(page, "system")
        first, second = page.locator("#caps-list .switch").nth(0), page.locator("#caps-list .switch").nth(1)
        first.wait_for()
        was = [first.locator("input").is_checked(), second.locator("input").is_checked()]
        first.click()
        assert wait_held(page, held), "the first PUT is sent"
        with page.expect_request(lambda r: r.method == "PUT" and "/capabilities/" in r.url):
            second.click()
        frames(page)
        assert len(held) == 2, f"both PUTs go out: {len(held)}"
        assert [first.locator("input").is_checked(), second.locator("input").is_checked()] == [not was[0], not was[1]], "both positions hold"
        for r in held:
            r.fulfill(status=403, content_type="application/json", body='{"error":"refused here"}')
        assert wait_until(page, "(() => { const s = document.querySelectorAll('#caps-list .switch input'); return s[0].checked === %s && s[1].checked === %s; })()"
                          % (str(was[0]).lower(), str(was[1]).lower()), 5), "refused: both as stored"
    finally:
        page.unroute("**/api/v1/capabilities/*")
    watch.problems[:] = [p for p in watch.problems if "403" not in p]
    show_view(page, "tiles")


def test_an_identity_form_goes_with_its_provider(page, case, provider):
    """C4: a draft in an identity's Change form, its provider removed in
    Sign-in providers: no pill, nothing on ☰."""
    from test_oidc import CLIENT_ID, SECRET
    user, key = f"idp-{case.width}-{case.lang}", f"idp{case.width}{case.lang}"
    undo(f"the provider {key}", lambda: api_call("DELETE", f"api/v1/providers/{key}"))
    make_user(user)
    api_call("POST", "api/v1/providers", {"key": key, "name": "Gone", "issuer": provider.issuer, "client_id": CLIENT_ID, "client_secret": SECRET})
    api_call("POST", f"api/v1/users/{user}/identities", {"provider": key, "email": f"gone-{case.width}{case.lang}@example.org"})
    try:
        page.goto(URL)  # the enabled providers read again
        page.wait_for_selector(".room", timeout=10000)
        panel = open_panel(page, f".user-panel[data-name='{user}']")
        row = panel.locator(".user-identities li[data-id]")
        row.wait_for()
        row.locator(".change-identity").click()
        row.locator(".identity-change input[name=account]").fill("draft@example.org")
        leave(page)
        assert wait_until(page, "!!document.querySelector('#sys-users .section-pill .problems')", 5), "the pill names it"
        prov = open_panel(page, f".provider-panel[data-key='{key}']")
        prov.locator(".remove-provider").click()
        prov.locator(".confirm-yes").click()
        assert wait_until(page, f"!document.querySelector(\".provider-panel[data-key='{key}']\")", 5), "the provider removed"
        assert wait_until(page, "!document.querySelector('#sys-users .section-pill .problems')", 5), "no pill"
        assert menu_mark(page) == "", "nothing on ☰"
    finally:
        remove_user(user)
        undo(f"the provider {key}", lambda: api_call("DELETE", f"api/v1/providers/{key}"))
    show_view(page, "tiles")


def test_a_closed_panel_reads_no_identities(page, case, provider):
    """D3: a user's panel with an identity opened and closed again, no
    draft in its forms: a change in another panel sends no GET of its
    identities."""
    from test_oidc import CLIENT_ID, SECRET
    user, other, key = f"idq-{case.width}-{case.lang}", f"ido-{case.width}-{case.lang}", f"idq{case.width}{case.lang}"
    undo(f"the provider {key}", lambda: api_call("DELETE", f"api/v1/providers/{key}"))
    make_user(user)
    make_user(other)
    api_call("POST", "api/v1/providers", {"key": key, "name": "Quiet", "issuer": provider.issuer, "client_id": CLIENT_ID, "client_secret": SECRET})
    api_call("POST", f"api/v1/users/{user}/identities", {"provider": key, "email": f"quiet-{case.width}{case.lang}@example.org"})
    gets = []
    try:
        page.goto(URL)  # the enabled providers read again
        page.wait_for_selector(".room", timeout=10000)
        panel = open_panel(page, f".user-panel[data-name='{user}']")
        panel.locator(".user-identities li[data-id]").wait_for()
        panel.locator("> summary").click()
        page.on("request", lambda r: gets.append(r.url) if r.method == "GET" and r.url.endswith(f"/users/{user}/identities") else None)
        mine = open_panel(page, f".user-panel[data-name='{other}']")
        mine.locator("input[name=display_name]").fill("Saved elsewhere")
        with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith(f"/users/{other}/identities")):  # the change's reads
            mine.locator("button[type=submit]").click()
        frames(page)
        assert gets == [], f"no read of the closed panel's identities: {gets}"
    finally:
        remove_user(user)
        remove_user(other)
        undo(f"the provider {key}", lambda: api_call("DELETE", f"api/v1/providers/{key}"))
    show_view(page, "tiles")


def test_an_identitys_change(page, case, provider):
    """A2, A3: an identity at an enabled provider, its Change typed and
    left: the pill names it; saved with another e-mail, the form closes;
    changed again and the identity removed, nothing names it any more:
    no pill, no mark on ☰, nothing for the leaving dialog."""
    from test_oidc import CLIENT_ID, SECRET
    user, key = f"idc-{case.width}-{case.lang}", f"idc{case.width}{case.lang}"
    undo(f"the provider {key}", lambda: api_call("DELETE", f"api/v1/providers/{key}"))
    make_user(user)
    api_call("POST", "api/v1/providers", {"key": key, "name": "Ident", "issuer": provider.issuer, "client_id": CLIENT_ID, "client_secret": SECRET})
    api_call("POST", f"api/v1/users/{user}/identities", {"provider": key, "email": f"first-{case.width}{case.lang}@example.org"})
    try:
        page.goto(URL)  # the enabled providers read again
        page.wait_for_selector(".room", timeout=10000)
        panel = open_panel(page, f".user-panel[data-name='{user}']")
        row = panel.locator(".user-identities li[data-id]")
        row.wait_for()
        row.locator(".change-identity").click()
        field = row.locator(".identity-change input[name=account]")
        field.fill(f"second-{case.width}{case.lang}@example.org")
        row.locator(".identity-change button[type=submit]").click()
        assert wait_until(page, f"!document.querySelector(\".user-panel[data-name='{user}'] .identity-change\")", 5), "saved, the form closes"
        row.locator(".change-identity").click()
        row.locator(".identity-change input[name=account]").fill("third@example.org")
        leave(page)
        assert wait_until(page, "!!document.querySelector('#sys-users .section-pill .problems')", 5), "the pill names it"
        row.locator(".remove-identity").click()
        assert wait_until(page, f"!document.querySelector(\".user-panel[data-name='{user}'] .user-identities li[data-id]\")", 5), "removed"
        assert wait_until(page, "!document.querySelector('#sys-users .section-pill .problems')", 5), "no pill"
        assert menu_mark(page) == "", "nothing on ☰"
        assert not page.evaluate("MassimoParts.leaving.changes()"), "nothing for the leaving dialog"
    finally:
        remove_user(user)
        undo(f"the provider {key}", lambda: api_call("DELETE", f"api/v1/providers/{key}"))
    show_view(page, "tiles")
