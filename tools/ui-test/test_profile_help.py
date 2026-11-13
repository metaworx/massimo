# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""My profile explains its controls (AP Oidc v1.2, Block 11): an ⓘ
behind the last button of each group, the display name, the password,
the sessions and keys and, with a sign-in provider, the linked accounts,
each labelled and opening its explanation below the group."""
from conftest import URL, api_call
from helpers import named_buttons, show_view
from test_oidc import CLIENT_ID, SECRET, FakeProvider

KEYS = ["profileName", "profilePassword", "profileTokens", "profileLinked"]


def test_profile_help(page, case):
    key = f"help-{case.width}-{case.lang}"
    provider = FakeProvider(f"help-{case.width}-{case.lang}@example.com")
    api_call("POST", "api/v1/providers", {"key": key, "name": "Help", "issuer": provider.issuer, "client_id": CLIENT_ID, "client_secret": SECRET})
    try:
        page.goto(URL)
        page.wait_for_selector(".room", timeout=10000)
        show_view(page, "profile")
        page.locator("#profile-name form").wait_for(state="visible")
        named_buttons(page, "My profile with its help")
        for k in KEYS:
            btn = page.locator(f".help-btn[aria-controls^='help-{k}-']")
            assert btn.count() == 1, f"one ⓘ for {k}"
            if k == "profilePassword":
                page.locator("#me-password summary").click()
            text = page.locator("#" + btn.get_attribute("aria-controls"))
            assert text.is_hidden(), f"{k}: hidden until asked for"
            btn.click()
            assert text.is_visible() and text.inner_text() == page.evaluate(f"t('help.{k}')"), f"{k}: its explanation"
            assert btn.get_attribute("aria-expanded") == "true"
            btn.click()
            assert text.is_hidden(), f"{k}: closed again"
    finally:
        api_call("DELETE", f"api/v1/providers/{key}")
        provider.close()
