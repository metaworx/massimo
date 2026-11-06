# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Settings → Users and Roles as the owner (the test token is the
owner's): the operator run.sh creates is listed with the owner, the
roles with the six default ones."""
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
