# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The settings pages' rows (AP GlobalMenu, Blocks 2 and 9): on each
settings page, in the title's place, a button for every settings page the
role may open, in ☰'s order, so that each keeps its place, the page shown
filled and marked current; each its icon and its name, below 900 px the
icon alone but for the page shown, which is as wide on every page (the
longest name, D159-A) with the sections menu's place kept; a click opens
its page, on the page shown nothing. A role with one settings page alone
sees its title."""
import json

from helpers import VIEWER, css_hidden, scroll_width, show_view, wait_until

PAGES = {"cube": ("#cube-page", "nav.cubeTitle"), "system": ("#system", "nav.settingsTitle"), "maint": ("#maint-page", "nav.maintTitle"),
         "info": ("#info-page", "nav.infoTitle")}
ADDRESS = {"cube": "#cube", "system": "#system", "maint": "#maintenance", "info": "#info"}
NARROW = 900  # style.css: below it the icon alone, but for the page shown


def links(page, section):
    return page.locator(section + " .page-bar .page-switch .page-link")


def test_each_settings_page_holds_them_all(page, case):
    """Every settings page's row: all four pages in ☰'s order, the one
    shown filled as ☰ is and marked current, each with its icon and its
    name (shown at full width; on a phone the shown page's alone, the
    others an icon with their name as title and accessible name); each
    button in the same place on every page at full width; on a phone the
    page shown as wide on every page, the buttons ending at the same place
    with the sections menu's place kept, so that a button before the page
    shown, and one after it, is where it is on every other page with it
    there; the title hidden but the page's name; the sections menu at the
    row's right end on the same line; one line, and no horizontal
    scroll."""
    xs = {}
    wide = {}
    ends = {}
    for view, (section, key) in PAGES.items():
        show_view(page, view)
        page.locator(section).wait_for(state="visible")
        got = links(page, section)
        assert [got.nth(i).get_attribute("data-view") for i in range(got.count())] == list(PAGES), f"{view}: all four, in ☰'s order"
        bar = page.locator(section + " .page-bar")
        title = bar.locator("h2")
        name = page.evaluate(f"t('{key}')")
        assert title.text_content() == name and title.bounding_box()["width"] <= 1, f"{view}'s title hidden, still its name"
        assert page.locator(section).get_attribute("aria-labelledby") == title.get_attribute("id"), f"{view} named by its title"
        menu_bg = page.evaluate("getComputedStyle(document.querySelector('#nav-menu > summary')).backgroundColor")
        xs[view] = []
        for i, other in enumerate(PAGES):
            b = got.nth(i)
            other_name = page.evaluate(f"t('{PAGES[other][1]}')")
            assert b.is_visible() and b.locator("svg.ni").count() == 1, f"{other} with its icon on {view}"
            assert b.get_attribute("title") == other_name and b.evaluate("(e) => e.textContent.trim()") == other_name, \
                f"{other} named {other_name}"
            here = other == view
            assert (b.get_attribute("aria-current") == "page") == here, f"only {view} marked current on its page"
            bg = b.evaluate("(e) => getComputedStyle(e).backgroundColor")
            assert (bg == menu_bg) == here, f"{other} filled as ☰ is only on its own page ({bg}, ☰ {menu_bg})"
            label = b.locator(".page-link-label").bounding_box()
            box = b.bounding_box()
            if case.width >= NARROW or here:
                assert label["width"] > 10, f"{other}'s name shown on {view}"
                if case.lang == "en":  # every English name fits at 390 px (the others, D159-A, are cut)
                    assert b.locator(".page-link-label").evaluate("""(e) => { const r = document.createRange(); r.selectNodeContents(e);
                      return r.getBoundingClientRect().width <= e.getBoundingClientRect().width + 0.01; }"""), f"{other}'s name whole"
            else:
                assert label["width"] <= 1 and box["width"] <= 41, f"{other} its icon alone on a phone: {box}"
            xs[view].append(box["x"])
        bar_box = bar.bounding_box()
        assert bar_box["height"] <= 53, f"{view}'s row one line: {bar_box}"
        last = got.last.bounding_box()
        wide[view] = got.nth(list(PAGES).index(view)).bounding_box()["width"]
        ends[view] = last["x"] + last["width"]
        if case.width < NARROW:
            assert ends[view] <= bar_box["x"] + bar_box["width"] - 48 + 1, f"the sections menu's place kept on {view}"
        menu = bar.locator(".sec-menu")
        if menu.count():
            m = menu.bounding_box()
            assert last["x"] + last["width"] <= m["x"], f"the buttons left of {view}'s sections menu"
            assert abs(m["x"] + m["width"] - (bar_box["x"] + bar_box["width"])) <= 2, f"{view}'s sections menu at the right end"
            assert abs(m["y"] - last["y"]) <= 3, f"{view}'s sections menu on the buttons' line"
        assert scroll_width(page) <= case.width, f"no horizontal scroll on {view}"
    for view in PAGES:
        here = list(PAGES).index(view)
        for i in range(len(PAGES)):
            # before the page shown as on System info (the last), after it as on Cube settings (the first)
            ref = "info" if case.width >= NARROW or i < here else "cube"
            if i != here:
                assert abs(xs[view][i] - xs[ref][i]) <= 1, f"button {i} in its place on {view}: {xs[view]} vs {xs[ref]}"
        if case.width < NARROW:
            assert abs(wide[view] - wide["cube"]) <= 1, f"the page shown as wide on {view} as on Cube settings: {wide}"
            assert abs(ends[view] - ends["cube"]) <= 1, f"the buttons end at the same place on {view}: {ends}"
    links(page, "#info-page").last.click()
    page.wait_for_timeout(200)
    assert page.locator("#info-page").is_visible() and page.evaluate("location.hash") == ADDRESS["info"], \
        "a click on the page shown opens nothing"
    links(page, "#info-page").first.click()
    page.locator("#cube-page").wait_for(state="visible")
    assert wait_until(page, f"location.hash === '{ADDRESS['cube']}'", 3), "the click opens Cube settings"
    links(page, "#cube-page").last.click()
    page.locator("#info-page").wait_for(state="visible")


def test_a_viewer_sees_its_title_and_no_button(page, case):
    """A viewer (/me stubbed) may open System info alone: its row shows
    the title, and no button at all; the next role with more pages gets
    the buttons back."""
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(VIEWER)))
    try:
        page.evaluate("loadMe()")
        assert wait_until(page, "document.body.classList.contains('no-system')", 5), "a viewer"
        page.evaluate("showView('info')")
        page.locator("#info-page").wait_for(state="visible")
        title = page.locator("#info-title")
        assert title.is_visible() and title.bounding_box()["width"] > 10, "its title shown"
        assert title.inner_text() == page.evaluate("t('nav.infoTitle')"), "named System info"
        assert css_hidden(page, "#info-page .page-switch"), "no button"
        assert scroll_width(page) <= case.width, "no horizontal scroll"
    finally:
        page.unroute("**/api/v1/me")
    page.evaluate("loadMe()")
    assert wait_until(page, "!document.body.classList.contains('no-system')", 5), "the token's role again"
    assert not css_hidden(page, "#info-page .page-switch") and links(page, "#info-page").first.is_visible(), "the buttons back"
    assert page.locator("#info-title").bounding_box()["width"] <= 1, "the title hidden again"
