# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The settings: My settings and System settings open, and a language
chosen in My settings redraws what the scripts cloned from the page's
templates; My profile changes the own display name and keys; a role
without System settings' capabilities has no ⚙."""
import json

from conftest import api_call
from helpers import PANEL_TEXTS, VIEWER, assert_busy, css_hidden, hold, named_buttons, scroll_width, show_view, wait_held, wait_until


def test_settings_open(page, case):
    """My settings holds the page's settings, System settings the
    server's; each opens alone, named and without a horizontal scroll."""
    show_view(page, "prefs")
    assert page.locator("#prefs #poll-input").is_visible() and page.locator("#prefs .appearance").is_visible(), \
        "My settings: the poll and the appearance"
    assert page.locator("#system").is_hidden() and page.locator("#profile").is_hidden(), "My settings alone"
    named_buttons(page, "My settings")
    assert scroll_width(page) <= case.width, "no horizontal scroll on My settings"
    show_view(page, "system")
    page.wait_for_timeout(500)
    for part in (".cubes-settings", ".backup-settings", ".users-settings", ".roles-settings", ".update"):
        assert page.locator("#system " + part).is_visible(), f"System settings: {part}"
    assert page.locator("#prefs").is_hidden(), "System settings alone"
    named_buttons(page, "System settings")
    assert scroll_width(page) <= case.width, "no horizontal scroll on System settings"
    show_view(page, "tiles")
    assert page.locator(".room").first.is_visible(), "back to the tiles"


def test_language_redraw(page):
    """With the Cube's panel open in System settings, English and then
    German chosen in My settings: its labels in the language chosen and
    the panel still open; back to the browser's language at the end."""
    show_view(page, "system")
    panel = page.locator(".cube-panel[data-serial='KEQ0000100']")
    if panel.get_attribute("open") is None:
        panel.locator("summary").click()
    for lang in ("en", "de"):
        show_view(page, "prefs")
        page.locator("#lang-select").select_option(lang)
        assert wait_until(page, "document.documentElement.lang === %s" % json.dumps(lang), 5), f"{lang} chosen in the settings"
        show_view(page, "system")
        want = PANEL_TEXTS[lang]
        got = [panel.locator(sel).inner_text().strip() for sel in ("label[for='cube-KEQ0000100-address']", ".cube-hidden-title", "form.cube-form button[type=submit]")]
        assert panel.get_attribute("open") is not None and got == want, f"the open Cube panel redrawn in {lang}: {got}"
    show_view(page, "prefs")
    page.locator("#lang-select").select_option("auto")
    show_view(page, "tiles")


def test_a_superseded_language_says_nothing(page):
    """French chosen, then German before French has come: German applies,
    and French, failing afterwards, shows no error on the German page.
    The test holds French's table and answers it with something that is
    no JSON, which fails the load without a console error."""
    held = []
    page.route("**/i18n/fr.json", lambda route: held.append(route))
    try:
        show_view(page, "prefs")
        page.locator("#lang-select").select_option("fr")
        page.locator("#lang-select").select_option("de")
        assert wait_until(page, "document.documentElement.lang === 'de'", 5), "German applies"
        assert held, "French's table was asked for"
        held[0].fulfill(status=200, content_type="application/json", body="not a table")
        page.wait_for_timeout(500)
        assert page.locator("#message").is_hidden(), f"no error of the French load: {page.locator('#message').inner_text()}"
        assert page.evaluate("document.documentElement.lang") == "de", "the page stays German"
    finally:
        page.unroute("**/i18n/fr.json")
    page.locator("#lang-select").select_option("auto")
    show_view(page, "tiles")


def test_my_profile_changes_the_own_display_name(page, case):
    """My profile, for the owner the suite's key signs in: Save is off
    until the display name differs, turns while it is sent, and the name
    is the user's afterwards; put back at the end."""
    show_view(page, "profile")
    name = page.locator("#profile-display-name")
    name.wait_for()
    save = page.locator("#profile-name button[type=submit]")
    was = name.input_value()
    assert save.is_disabled(), "Save is off while the name is the stored one"
    name.fill("Owner " + case.lang)
    assert save.is_enabled(), "a changed name turns Save on"
    held, sent = [], []
    hold(page, "**/api/v1/users/owner", "PUT", held, sent)
    try:
        save.click()
        assert wait_held(page, held), "the PUT is sent"
        assert_busy(page, "#profile-name button[type=submit]", "Save")
        assert sent == [{"display_name": "Owner " + case.lang}], f"the display name alone: {sent}"
        for route in held:
            route.continue_()
    finally:
        page.unroute("**/api/v1/users/owner")
    assert wait_until(page, "!document.getElementById('profile-msg').classList.contains('hidden')", 5), "it says it saved"
    assert api_call("GET", "api/v1/users/owner")["display_name"] == "Owner " + case.lang, "the server has it"
    api_call("PUT", "api/v1/users/owner", {"display_name": was})
    show_view(page, "tiles")


def test_my_profile_lists_and_creates_own_keys(page, case):
    """My profile lists the own sessions and keys, the suite's among them,
    and Create key shows a new key once; Revoke takes it again."""
    show_view(page, "profile")
    rows = page.locator("#profile-tokens .user-tokens li")
    assert wait_until(page, "document.querySelectorAll('#profile-tokens .user-tokens li .token-label').length > 0", 5), "the own keys are listed"
    assert "MASSIMO_API_TOKEN" in page.locator("#profile-tokens .user-tokens").inner_text(), "the suite's key among them"
    before = rows.count()
    label = "profile test " + case.lang
    page.locator("#profile-tokens input[name=label]").fill(label)
    page.locator("#profile-tokens .create-key").click()
    assert wait_until(page, "!document.querySelector('#profile-tokens .user-key-box').classList.contains('hidden')", 5), "the new key is shown"
    assert rows.count() == before + 1, "and listed"
    row = page.locator("#profile-tokens .user-tokens li", has_text=label)
    row.locator(".revoke-token").click()
    assert wait_until(page, "document.querySelectorAll('#profile-tokens .user-tokens li').length === %d" % before, 5), "Revoke takes it"
    show_view(page, "tiles")


def test_no_system_settings_without_their_capabilities(page, case):
    """A viewer (/me stubbed) has no ⚙, and an address or a page asking
    for System settings shows Rooms; the operator, with backup, has it."""
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(VIEWER)))
    try:
        show_view(page, "system")
        page.evaluate("loadMe()")
        assert wait_until(page, "document.body.classList.contains('no-system')", 5), "no System settings for a viewer"
        assert css_hidden(page, "#view-settings") and css_hidden(page, "#menu-system"), "no ⚙, and nothing in ☰"
        assert page.locator("#system").is_hidden() and page.locator(".room").first.is_visible(), \
            "System settings, open as the viewer signs in, gives way to Rooms"
        page.evaluate("showView('system')")
        assert page.locator("#system").is_hidden(), "and does not open"
    finally:
        page.unroute("**/api/v1/me")
        page.evaluate("loadMe()")
    assert wait_until(page, "!document.body.classList.contains('no-system')", 5), "the owner has it again"
