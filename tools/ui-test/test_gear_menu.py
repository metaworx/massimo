# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""⚙, the menu of the settings pages (AP PageMenu, Block 1): Cube
settings, System settings and System info, each for the roles that may
open it; ☰ holds them under Log on a phone. Each opens its page, puts it
into the address, highlights ⚙ (☰ on a phone) and closes the menu; back
returns to the page before."""
import json

from helpers import VIEWER, css_hidden, show_view, wait_until

GEAR = ["#gear-cube", "#gear-system", "#gear-info"]
FOLDED = ["#menu-log", "#menu-cube", "#menu-system", "#menu-info"]
PAGES = {"cube": ("#cube-page", "#cube"), "system": ("#system", "#system"), "info": ("#info-page", "#info")}


def menu(page, case):
    """⚙ at full width, ☰ below 600 px: the menu that holds the pages."""
    return page.locator("#gear-menu" if case.width >= 600 else "#nav-menu")


def open_menu(page, case):
    m = menu(page, case)
    if m.get_attribute("open") is None:
        m.locator("summary").click()
    m.locator(".menu-items").wait_for(state="visible")
    return m


def test_gear_menu_entries(page, case):
    """At full width ⚙ beside the log, not ☰, its three entries with their
    icons in this order; on a phone ☰ in its place, with Log and the same
    three."""
    if case.width >= 600:
        assert page.locator("#gear-menu").is_visible() and css_hidden(page, "#nav-menu"), "⚙, no ☰"
        log, gear = page.locator("#view-log").bounding_box(), page.locator("#view-gear").bounding_box()
        assert gear["x"] > log["x"], "⚙ after the log"
        items = GEAR
    else:
        assert page.locator("#nav-menu").is_visible() and css_hidden(page, "#gear-menu"), "☰, no ⚙"
        items = FOLDED
    m = open_menu(page, case)
    ys = [page.locator(sel).bounding_box()["y"] for sel in items]
    assert ys == sorted(ys), f"in this order: {items}"
    for sel in items:
        assert page.locator(sel).is_visible(), f"{sel} shown to the owner"
        assert page.locator(sel + " > svg.ni").count() == 1, f"{sel} shows its page's icon"
    m.locator("summary").click()


def test_gear_menu_opens_each_page(page, case):
    """Each entry opens its page and its address, closes the menu and
    highlights it; back returns to the page before, Rooms here."""
    for view, (section, address) in PAGES.items():
        show_view(page, view)
        page.locator(section).wait_for(state="visible")
        assert wait_until(page, f"location.hash === '{address}'", 3), f"{view} in the address"
        m = menu(page, case)
        assert m.get_attribute("open") is None, "the choice closes the menu"
        assert m.locator("summary").evaluate("(e) => e.classList.contains('active')"), f"the menu highlighted while {view} is shown"
        assert page.locator("#view-tiles").evaluate("(e) => !e.classList.contains('active')"), "Rooms not"
        others = [s for v, (s, _) in PAGES.items() if v != view]
        assert all(page.locator(s).is_hidden() for s in others), "the other settings pages hidden"
        page.go_back()
        page.locator("#rooms").wait_for(state="visible")
        assert wait_until(page, "location.hash === ''", 3), "back to Rooms"
        assert not m.locator("summary").evaluate("(e) => e.classList.contains('active')"), "and the menu no longer highlighted"


def test_an_address_opens_its_settings_page(page, case):
    """#cube and #info open their pages at a reload, as #system does."""
    for view, (section, address) in PAGES.items():
        page.evaluate(f"history.replaceState(null, '', location.pathname + '{address}')")
        page.reload()
        page.locator(section).wait_for(state="visible", timeout=10000)


def test_a_viewer_has_system_info_alone(page, case):
    """A viewer (/me stubbed) sees ⚙ (☰ on a phone) with System info
    alone; Cube settings and System settings in the address show Rooms;
    #info opens."""
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(VIEWER)))
    try:
        page.evaluate("loadMe()")
        assert wait_until(page, "document.body.classList.contains('no-system')", 5), "a viewer"
        m = open_menu(page, case)
        prefix = "#gear-" if case.width >= 600 else "#menu-"
        assert page.locator(prefix + "info").is_visible(), "System info for a viewer"
        for name in ("cube", "system"):
            assert css_hidden(page, prefix + name), f"no {name} settings for a viewer"
        m.locator("summary").click()
        for view in ("cube", "system"):
            page.evaluate(f"showView('{view}')")
            assert page.locator(PAGES[view][0]).is_hidden() and page.locator("#rooms").is_visible(), f"{view} gives way to Rooms"
        page.evaluate("showView('info')")
        page.locator("#info-page").wait_for(state="visible")
    finally:
        page.unroute("**/api/v1/me")
