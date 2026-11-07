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
