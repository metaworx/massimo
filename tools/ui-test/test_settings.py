# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The settings page: it opens, and a language chosen there redraws what
the scripts cloned from the page's templates."""
import json

from helpers import PANEL_TEXTS, named_buttons, scroll_width, wait_until


def test_settings_open(page, case):
    page.locator("#view-settings").click()
    assert page.locator("#settings").is_visible(), "settings open"
    page.wait_for_timeout(500)
    named_buttons(page, "the settings")
    assert scroll_width(page) <= case.width, "no horizontal scroll on the settings"
    page.locator("#view-tiles").click()
    assert page.locator(".room").first.is_visible(), "back to the tiles"


def test_language_redraw(page):
    """With the Cube's panel open, English and then German chosen in the
    settings: its labels in the language chosen and the panel still
    open; back to the browser's language at the end."""
    page.locator("#view-settings").click()
    panel = page.locator(".cube-panel[data-serial='KEQ0000100']")
    if panel.get_attribute("open") is None:
        panel.locator("summary").click()
    for lang in ("en", "de"):
        page.locator("#lang-select").select_option(lang)
        assert wait_until(page, "document.documentElement.lang === %s" % json.dumps(lang), 5), f"{lang} chosen in the settings"
        want = PANEL_TEXTS[lang]
        got = [panel.locator(sel).inner_text().strip() for sel in ("label[for='cube-KEQ0000100-address']", ".cube-hidden-title", "form.cube-form button[type=submit]")]
        assert panel.get_attribute("open") is not None and got == want, f"the open Cube panel redrawn in {lang}: {got}"
    page.locator("#lang-select").select_option("auto")
    page.locator("#view-tiles").click()


def test_a_superseded_language_says_nothing(page):
    """French chosen, then German before French has come: German applies,
    and French, failing afterwards, shows no error on the German page.
    The test holds French's table and answers it with something that is
    no JSON, which fails the load without a console error."""
    held = []
    page.route("**/i18n/fr.json", lambda route: held.append(route))
    try:
        page.locator("#view-settings").click()
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
    page.locator("#view-tiles").click()
