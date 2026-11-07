# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Settings → Users and Roles as the owner (the test token is the
owner's): the operator run.sh creates is listed with the owner, the
roles with the six default ones."""
from conftest import api_call
from helpers import NAME_BAD, NAME_TAKEN, assert_busy, hold, named_buttons, scroll_width, wait_held, wait_until


def test_users_and_roles(page, case):
    """The operator is listed with its tokens, the roles with the
    operator's seven capabilities on, every button has a name, and nothing
    overflows."""
    page.locator("#view-settings").click()
    users = page.locator(".users-settings")
    assert users.is_visible(), "Settings → Users shown to the owner"
    assert wait_until(page, "document.querySelectorAll('.user-panel').length >= 2", 5), "the users are listed"
    assert page.locator(".user-panel[data-name='operator']").count() == 1, "the operator is listed"
    page.locator(".user-panel[data-name='operator'] summary").click()
    assert wait_until(page, "document.querySelectorAll(\".user-panel[data-name='operator'] .user-tokens li\").length >= 1", 5), \
        "the operator's tokens are listed"
    named_buttons(page, "Settings → Users")
    roles = page.locator(".roles-settings")
    assert roles.is_visible() and wait_until(page, "document.querySelectorAll('.role-panel').length >= 6", 5), "the roles are listed"
    page.locator(".role-panel[data-key='operator'] summary").click()
    assert page.locator(".role-panel[data-key='operator'] .cap-grid input:checked").count() == 7, "the operator's seven capabilities are on"
    named_buttons(page, "Settings → Roles")
    assert scroll_width(page) <= case.width, "no horizontal scroll in Settings → Users and Roles"
    page.locator("#view-tiles").click()


def revoke_keys(prefix):
    """The operator's API keys whose label starts with prefix are
    revoked, so that the four cases leave the server as they found it."""
    for tk in api_call("GET", "api/v1/users/operator/tokens") or []:
        if (tk.get("label") or "").startswith(prefix):
            api_call("DELETE", "api/v1/users/operator/tokens/%d" % tk["id"])


def test_new_key_shown_once(page, case):
    """In an open user panel, Create key shows the new key, which the
    server shows only once, lists every token once, and sends one POST
    per click: the panel loads its tokens through one path, not through
    the toggle of a panel built open and a direct call both."""
    prefix = f"ui-test-key-{case.width}-{case.lang}-"
    revoke_keys(prefix)
    posts = []
    page.on("request", lambda r: posts.append(r.url) if r.method == "POST" and r.url.endswith("/users/operator/tokens") else None)
    page.locator("#view-settings").click()
    panel = page.locator(".user-panel[data-name='operator']")
    panel.wait_for()
    panel.locator("summary").click()
    tokens = ".user-panel[data-name='operator'] .user-tokens li"
    assert wait_until(page, "document.querySelectorAll(%r).length >= 1" % tokens, 5), "the operator's tokens are listed"
    try:
        for n in (1, 2):
            panel.locator("input[name=label]").fill(prefix + str(n))
            panel.locator(".create-key").click()
            assert wait_until(page, "document.querySelectorAll('.user-panel[data-name=\"operator\"] li[data-id]').length > 0 && "
                                    "[...document.querySelectorAll(%r)].some((li) => li.textContent.includes(%r))" % (tokens, prefix + str(n)), 5), \
                f"key {n} is listed"
            page.wait_for_timeout(500)  # a second load of the tokens would have come back by now
            key = panel.locator(".user-key-box")
            assert key.is_visible() and panel.locator(".user-key").inner_text().strip(), f"key {n} is shown, once"
            ids = page.evaluate("[...document.querySelectorAll(%r)].map((li) => li.dataset.id).filter((id) => id)" % tokens)
            assert len(ids) == len(set(ids)), f"every token once after key {n}: {ids}"
            assert len(posts) == n, f"one POST per click, {len(posts)} after {n}"
    finally:
        revoke_keys(prefix)
    page.locator("#view-tiles").click()


def remove_user(name):
    """A user the test made, gone again; none is fine."""
    try:
        api_call("DELETE", "api/v1/users/" + name)
    except Exception:  # noqa: BLE001 - absent already
        pass


def test_an_unchanged_end_date_is_not_sent(page, case):
    """A user with an end date whose display name alone changes is saved
    without the end date: the form's minute and the server's time with
    its seconds and offset are the same end; Save is off while nothing
    differs, so nothing is sent. The test makes the guest and removes it."""
    name = f"end-{case.width}-{case.lang}"
    remove_user(name)
    api_call("POST", "api/v1/users", {"name": name, "role": "guest", "password": "end-pass", "expires_at": "2099-01-02T10:20:30+01:00"})
    puts = []
    page.on("request", lambda r: puts.append(r.post_data_json) if r.method == "PUT" and r.url.endswith("/users/" + name) else None)
    try:
        page.locator("#view-settings").click()
        panel = page.locator(".user-panel[data-name='%s']" % name)
        panel.wait_for()
        panel.locator("summary").click()
        save = panel.locator("form button[type=submit]")
        assert save.is_disabled(), "Save is off while nothing differs from the user"
        panel.locator("input[name=display_name]").fill("Guest")
        assert save.is_enabled(), "a change turns Save on"
        panel.locator("input[name=display_name]").fill("")
        assert save.is_disabled(), "Save is off again once the change is undone"
        page.wait_for_timeout(300)
        assert not puts, f"nothing is sent without a change: {puts}"
        panel.locator("input[name=display_name]").fill("Guest with an end")
        save.click()
        assert wait_until(page, "document.querySelector(\".user-panel[data-name='%s'] .panel-name\").textContent === 'Guest with an end'" % name, 5), \
            "the display name is saved"
        assert puts == [{"display_name": "Guest with an end"}], f"the display name alone is sent: {puts}"
    finally:
        remove_user(name)
    page.locator("#view-tiles").click()


def test_a_double_click_on_yes_removes_once(page, case):
    """Remove asks in the panel, and a double click on its Yes sends one
    DELETE: Yes is off while the removal runs, so the message says the
    user is removed, not that a second DELETE found nobody. The test
    makes the user."""
    name = f"rm-{case.width}-{case.lang}"
    remove_user(name)
    api_call("POST", "api/v1/users", {"name": name, "role": "viewer", "password": "rm-pass"})
    deletes = []
    page.on("request", lambda r: deletes.append(r.url) if r.method == "DELETE" and r.url.endswith("/users/" + name) else None)
    try:
        page.locator("#view-settings").click()
        panel = page.locator(".user-panel[data-name='%s']" % name)
        panel.wait_for()
        panel.locator("summary").click()
        panel.locator(".remove-user").click()
        box = panel.locator(".confirm-box")
        assert box.is_visible() and panel.locator(".remove-user").count() == 0, "Remove asks in the panel"
        box.locator(".confirm-no").click()
        assert panel.locator(".confirm-box").count() == 0 and \
            page.evaluate("document.activeElement && document.activeElement.classList.contains('remove-user')"), \
            "Keep takes the question back and the focus to Remove"
        panel.locator(".remove-user").click()
        panel.locator(".confirm-box .confirm-yes").dblclick()
        assert wait_until(page, "!document.querySelector(\".user-panel[data-name='%s']\")" % name, 5), "the user is removed"
        page.wait_for_timeout(300)
        assert len(deletes) == 1, f"one DELETE for a double click: {len(deletes)}"
        assert "ok" in page.locator("#users-msg").get_attribute("class"), "the message says it worked"
    finally:
        remove_user(name)
    page.locator("#view-tiles").click()


def test_create_is_busy_until_answered(page, case):
    """Create is off with a spinner from its click until the server
    answers, so that a second click or Enter sends nothing: on the Pi 1
    the hashing took 4.6 s and a second click was refused as "user
    exists". The test holds the POST, then lets it go."""
    name = f"busy-{case.width}-{case.lang}"
    remove_user(name)
    held, posts = [], []
    hold(page, "**/api/v1/users", "POST", held, posts)
    try:
        page.locator("#view-settings").click()
        page.locator("#user-add").click()
        form = page.locator("#user-new form")
        form.locator("input[name=name]").fill(name)
        form.locator("input[name=password]").fill("busy-pass")
        form.locator("button[type=submit]").click()
        assert wait_held(page, held), "the POST is sent"
        assert_busy(page, "#user-new button[type=submit]", "Create")
        form.locator("button[type=submit]").click(force=True)
        form.locator("input[name=name]").press("Enter")
        page.wait_for_timeout(300)
        assert len(posts) == 1, f"one POST while the first is unanswered: {len(posts)}"
        for route in held:
            route.continue_()
        assert wait_until(page, "!document.querySelector('#user-new form') && document.getElementById('users-msg').classList.contains('ok')", 5), \
            "the user is created and the form closed"
        assert page.locator(".user-panel[data-name='%s']" % name).count() == 1, "the user is listed once"
        assert len(posts) == 1, f"one POST in all: {len(posts)}"
    finally:
        page.unroute("**/api/v1/users")
        remove_user(name)
    page.locator("#view-tiles").click()


def test_role_save_is_off_until_a_change(page, case):
    """A role's Save is off while its name and set are as stored; a
    capability switched turns it on, switched back off again. Nothing
    is sent."""
    page.locator("#view-settings").click()
    panel = page.locator(".role-panel[data-key='operator']")
    assert wait_until(page, "!!document.querySelector(\".role-panel[data-key='operator']\")", 5), "the roles are listed"
    panel.locator("summary").click()
    save = panel.locator("form button[type=submit]")
    assert save.is_disabled(), "Save is off with the role as stored"
    switch = panel.locator(".cap-grid .switch").first
    switch.click()
    assert save.is_enabled(), "a capability switched turns Save on"
    switch.click()
    assert save.is_disabled(), "switched back, nothing differs"
    page.locator("#view-tiles").click()


def name_msg(page, selector):
    """The name check's message under a field: its text, or None while
    hidden."""
    return page.evaluate("(() => { const p = document.querySelector(%r); return p && !p.classList.contains('hidden') ? p.textContent : null; })()"
                         % selector)


def test_new_user_name_checked_while_typed(page, case, watch):
    """A new user's name is checked 2 s after the last keystroke and when
    the field is left, with the page's own words; a fixed name clears the
    message at once; a name in use is reported; Create with a bad name
    sends nothing, and a name the server finds in use is worded alike."""
    msg = "#user-new-name-msg"
    posts = []
    page.on("request", lambda r: posts.append(r.url) if r.method == "POST" and r.url.endswith("/api/v1/users") else None)
    page.locator("#view-settings").click()
    page.locator("#user-add").click()
    name = page.locator("#user-new-name")
    name.fill("test manager")
    page.wait_for_timeout(500)
    assert name_msg(page, msg) is None, "nothing is said while the name is being typed"
    assert wait_until(page, "!document.querySelector(%r).classList.contains('hidden')" % msg, 4), "the name is checked after 2 s"
    assert name_msg(page, msg) == NAME_BAD[case.lang], f"the page's words, without store: {name_msg(page, msg)}"
    assert name.get_attribute("aria-invalid") == "true" and name.get_attribute("aria-describedby") == "user-new-name-msg", \
        "the field is marked and described by the message"
    name.fill("test-manager")
    assert name_msg(page, msg) is None and name.get_attribute("aria-invalid") is None, "a fixed name clears the message at once"
    name.fill("x y")
    name.press("Tab")
    assert name_msg(page, msg) == NAME_BAD[case.lang], "leaving the field checks at once"
    name.fill("Operator")
    name.press("Tab")
    assert name_msg(page, msg) == NAME_TAKEN[case.lang].format(name="operator"), "a listed name is in use, in lower case"
    name.fill("bad name")
    page.locator("#user-new input[name=password]").fill("pw")
    page.locator("#user-new button[type=submit]").click()
    page.wait_for_timeout(300)
    assert name_msg(page, msg) == NAME_BAD[case.lang] and not posts, f"Create with a bad name sends nothing: {posts}"
    late = f"late-{case.width}-{case.lang}"
    remove_user(late)
    watch.expected.append(("/api/v1/users", 409))
    api_call("POST", "api/v1/users", {"name": late, "role": "viewer"})  # after the list was read
    try:
        name.fill(late)
        page.locator("#user-new button[type=submit]").click()
        assert wait_until(page, "!document.querySelector(%r).classList.contains('hidden')" % msg, 5), "the server's 409 is said"
        assert name_msg(page, msg) == NAME_TAKEN[case.lang].format(name=late), "worded as the page's check words it"
        assert "store:" not in page.locator("#users-msg").inner_text(), "no store: in the list's message"
    finally:
        remove_user(late)
    page.locator("#user-new .cancel-user").click()
    page.locator("#view-tiles").click()


def test_first_owner_name_checked(page, case):
    """The first owner's form checks its name the same way; with a bad
    name, Create the owner sends nothing. /api/v1/me is stubbed as an
    open server without users, and put back."""
    posts = []
    page.on("request", lambda r: posts.append(r.url) if r.method == "POST" and r.url.endswith("/api/v1/users") else None)
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body='{"mode": "open", "setup": true}'))
    try:
        page.evaluate("loadMe()")
        assert wait_until(page, "!document.getElementById('setup').classList.contains('hidden')", 5), "the first owner's form shows"
        name = page.locator("#setup-user")
        name.fill("test manager")
        name.press("Tab")
        assert name_msg(page, "#setup-name-msg") == NAME_BAD[case.lang], "leaving the field checks it"
        page.locator("#setup-password").fill("pw")
        page.locator("#setup-again").fill("pw")
        page.locator("#setup-form button[type=submit]").click()
        page.wait_for_timeout(300)
        assert not posts, f"Create the owner with a bad name sends nothing: {posts}"
        name.fill("owner")
        assert name_msg(page, "#setup-name-msg") is None, "a good name clears the message"
    finally:
        page.unroute("**/api/v1/me")
        page.evaluate("loadMe()")
    assert wait_until(page, "document.getElementById('setup').classList.contains('hidden')", 5), "the form goes again"
