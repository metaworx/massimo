# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Signing in, in a browser context of its own without the stored
token, as the operator run.sh creates (password op-pass). A page error
fails the test (the guard); the console is not watched, since the
page's first requests without a token are refused on purpose."""
from conftest import URL
from helpers import named_buttons, wait_until


def test_sign_in_and_out(new_context, case, watch, guard):
    """The sign-in dialog opens without a token and refuses a wrong
    password; the operator signs in, sees the tiles and no Settings →
    Users, Roles or Cubes; Log out forgets the token and brings the
    dialog back."""
    ctx = new_context()  # the case's viewport and locale, from browser_context_args
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in dialog opens without a token"
    named_buttons(page, "the sign-in dialog")
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("wrong")
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, "!document.getElementById('login-error').classList.contains('hidden')", 5), "a wrong password is refused in the dialog"
    assert page.evaluate("document.getElementById('login-dialog').open"), "the dialog stays open"
    page.locator("#login-password").fill("op-pass")
    page.locator("#login-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert page.evaluate("state.me && state.me.user && state.me.user.name") == "operator", "signed in as the operator"
    assert page.evaluate("document.body.classList.contains('no-users-admin') && document.body.classList.contains('no-rename')"), \
        "the missing capabilities are on the body"
    assert page.locator(".room .tile-cog").first.is_visible() and not page.locator(".room .menu-delete").first.is_visible(), \
        "an operator configures but does not delete rooms"
    page.locator("#view-settings").click()
    assert not page.locator(".users-settings").is_visible(), "no Settings → Users for an operator"
    assert not page.locator(".roles-settings").is_visible(), "no Settings → Roles for an operator"
    assert not page.locator(".cubes-settings.needs-cubes").is_visible(), "no Settings → Cubes for an operator"
    assert "operator" in page.locator("#me-line").inner_text(), "the settings say who is signed in"
    named_buttons(page, "the settings as an operator")
    page.locator("#logout-btn").click()
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "Log out brings the dialog back"
    assert page.evaluate("localStorage.getItem('massimo.token')") is None, "the token is forgotten"
