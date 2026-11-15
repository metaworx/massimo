# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The settings pages' rows (AP GlobalMenu, Block 2): on each settings
page, right-aligned before its sections menu, a button for each of the
other settings pages the role may open, in ☰'s order, its icon and its
name, the icon alone on a phone; a click opens its page."""
import json

from helpers import VIEWER, css_hidden, scroll_width, show_view, wait_until

PAGES = {"cube": ("#cube-page", "nav.cubeTitle"), "system": ("#system", "nav.settingsTitle"), "maint": ("#maint-page", "nav.maintTitle"),
         "info": ("#info-page", "nav.infoTitle")}
ADDRESS = {"cube": "#cube", "system": "#system", "maint": "#maintenance", "info": "#info"}


def links(page, section):
    return page.locator(section + " .page-bar .page-switch .page-link")


def test_each_settings_page_links_the_others(page, case):
    """Every settings page's row: the other pages in ☰'s order, each with
    its icon and its name (shown at full width, its title and accessible
    name on a phone, where the icon stands alone), right of the title and
    left of the sections menu, under the title where both do not fit on
    one line; the title whole and no horizontal scroll; a click opens the
    page."""
    for view, (section, _) in PAGES.items():
        show_view(page, view)
        page.locator(section).wait_for(state="visible")
        got = links(page, section)
        others = [v for v in PAGES if v != view]
        assert [got.nth(i).get_attribute("data-view") for i in range(got.count())] == others, f"{view}: the others, in ☰'s order"
        title = page.locator(section + " .page-bar h2").bounding_box()
        menu = page.locator(section + " .page-bar .sec-menu")
        for i, other in enumerate(others):
            b = got.nth(i)
            name = page.evaluate(f"t('{PAGES[other][1]}')")
            assert b.is_visible() and b.locator("svg.ni").count() == 1, f"{other} with its icon on {view}"
            assert b.get_attribute("title") == name and b.inner_text().strip() in ("", name), f"{other} named {name}"
            assert b.evaluate("(e) => e.textContent.trim()") == name, "its accessible name"
            label = b.locator(".page-link-label")
            box = b.bounding_box()
            if case.width >= 600:
                assert label.is_visible() and label.bounding_box()["width"] > 10, f"{other}'s name shown"
            else:
                assert label.bounding_box()["width"] <= 1 and box["width"] <= 41, f"{other} its icon alone on a phone: {box}"
            assert box["x"] >= title["x"] + title["width"] or box["y"] >= title["y"] + title["height"] - 1, \
                f"{other} right of the title, or under it where both do not fit"
            if menu.count():
                assert box["x"] + box["width"] <= menu.bounding_box()["x"], f"{other} left of the sections menu"
        assert page.locator(section + " .page-bar h2").evaluate("(e) => e.scrollWidth <= e.clientWidth + 1"), f"{view}'s title whole"
        assert scroll_width(page) <= case.width, f"no horizontal scroll on {view}"
    got = links(page, "#info-page")
    got.first.click()
    page.locator("#cube-page").wait_for(state="visible")
    assert wait_until(page, f"location.hash === '{ADDRESS['cube']}'", 3), "the click opens Cube settings"
    links(page, "#cube-page").last.click()
    page.locator("#info-page").wait_for(state="visible")


def test_a_viewer_has_no_other_page_to_switch_to(page, case):
    """A viewer (/me stubbed) opens System info alone: its row shows no
    button for Cube settings, System settings or Maintenance."""
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(VIEWER)))
    try:
        page.evaluate("loadMe()")
        assert wait_until(page, "document.body.classList.contains('no-system')", 5), "a viewer"
        page.evaluate("showView('info')")
        page.locator("#info-page").wait_for(state="visible")
        for view in ("cube", "system", "maint"):
            assert css_hidden(page, f"#info-page .page-link[data-view='{view}']"), f"no {view} button for a viewer"
    finally:
        page.unroute("**/api/v1/me")
