# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Settings → Users and Roles as the owner (the test token is the
owner's): the operator run.sh creates is listed with the owner, the
roles with the six default ones."""
from conftest import api_call
from helpers import named_buttons, scroll_width, wait_until


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
    its seconds and offset are the same end; Save without a change says
    so and sends nothing. The test makes the guest and removes it."""
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
        panel.locator("form button[type=submit]").click()
        assert wait_until(page, "!document.getElementById('users-msg').classList.contains('hidden')", 5), "Save says how it went"
        page.wait_for_timeout(300)
        assert not puts, f"Save without a change sends nothing: {puts}"
        panel.locator("input[name=display_name]").fill("Guest with an end")
        panel.locator("form button[type=submit]").click()
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
