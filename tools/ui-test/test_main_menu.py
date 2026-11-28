# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""☰, the page's one menu (AP GlobalMenu, Block 3), at every width: the
log; Cube settings, System settings, Maintenance and System info;
Personal settings; in the app App settings, This server's settings and
Select server, a rule before and after them (AP MobileApp, Block 20);
and Sign out, each for the roles that may open it, a separator between
the groups, a browser drawing one rule where the app's three are not.
Each opens its page, puts it into the address, highlights ☰, marks its
entry current and closes the menu; back returns to the page before.
Sign out is the personal page's button: Forget key for a key, gone
where that button is. The version row's tap copies the whole version,
says so at the window's foot and closes ☰ (D115-D)."""
import json

from conftest import URL
from helpers import OPERATOR_PASSWORD, VIEWER, boxes, css_hidden, held_timers, open_main_menu, show_view, wait_bubble_foot, wait_until
from test_login import sign_in

ORDER = ["#menu-log", "hr", "#menu-cube", "#menu-system", "#menu-maint", "#menu-info", "hr", "#menu-me", "#menu-app-sep", "#menu-app-settings",
         "#menu-server-settings", "#menu-servers", "#menu-out-sep", "#menu-logout", "#menu-owner"]
# in the app alone: App settings and This server's settings on the app's own page, and Select server, back to the
# app's servers (AP MobileApp, D73-A; test_connect.py), and the rule after them, before Sign out (Block 20); on a
# server without users alone, in Sign out's place: Create the owner… (AP FirstOwner, test_ways.py)
APP_ONLY = {"#menu-servers", "#menu-server-settings", "#menu-app-settings", "#menu-out-sep", "#menu-owner"}
RULES = {"hr", "#menu-app-sep", "#menu-out-sep"}
PAGES = {"log": ("#logview", "#log", "#menu-log"), "cube": ("#cube-page", "#cube", "#menu-cube"),
         "system": ("#system", "#system", "#menu-system"), "maint": ("#maint-page", "#maintenance", "#menu-maint"),
         "info": ("#info-page", "#info", "#menu-info"),
         "me": ("#me", "#me", "#menu-me")}


def test_main_menu_entries(page, case):
    """☰ at every width, the server's version its first row, a rule under
    it, then its entries with their icons in this order, the groups apart, the app's three not shown in a browser; Sign out
    with the personal page's label and icon, in its accent and weight."""
    assert page.locator("#nav-menu").is_visible(), "☰ shown"
    m = open_main_menu(page)
    got = m.locator(".menu-items > *").evaluate_all("(es) => es.map((e) => e.id ? '#' + e.id : e.tagName.toLowerCase())")
    assert got[0] == "p" and m.locator(".menu-items > .menu-version #version-short").count() == 1, f"the version first: {got}"
    assert got[1] == "hr", f"a rule under it: {got}"
    got = got[2:]
    assert got == ORDER, f"☰'s entries in this order: {got}"
    for sel in ORDER:
        if sel in APP_ONLY:
            assert page.locator(sel).is_hidden(), f"{sel} not shown to the owner of a browser"
            continue
        if sel in RULES:
            continue
        assert page.locator(sel + " > svg.ni").count() == 1, f"{sel} with its icon"
        assert page.locator(sel).is_visible(), f"{sel} shown to the owner"
    shown = m.locator(".menu-items > *").evaluate_all("(es) => es.filter((e) => e.getClientRects().length).map((e) => e.id ? '#' + e.id : e.tagName.toLowerCase())")
    assert shown[shown.index("#menu-me"):] == ["#menu-me", "#menu-app-sep", "#menu-logout"], f"one rule between Personal settings and Sign out: {shown}"
    ys = [b["y"] for b in boxes(page, *(sel for sel in ORDER if sel not in RULES and sel not in APP_ONLY))]
    assert ys == sorted(ys), "one under the other"
    assert page.locator("#menu-logout-label").inner_text() == page.evaluate("document.getElementById('logout-label').textContent"), \
        "Sign out says what the personal page's button says"
    assert page.locator("#menu-logout-icon").get_attribute("href") == page.locator("#logout-icon").get_attribute("href"), "and its icon"
    look = "(e) => [getComputedStyle(e).color, getComputedStyle(e).fontWeight]"
    assert page.locator("#menu-logout").evaluate(look) == page.locator("#logout-btn").evaluate(look), \
        "in the accent and bold, as the personal page's button"
    assert page.locator("#menu-logout").evaluate(look) != page.locator("#menu-me").evaluate(look), "unlike the other entries"
    m.locator("summary").click()


def test_the_version_row_copies_and_closes(page, case):
    """☰'s version row, the version and its copy icon: a tap on either
    copies the whole version, says "Copied" at the window's foot, not over
    the row, which goes with ☰, and closes ☰, the focus back on its button;
    the keyboard's Enter too (AP MobileApp, Block 20, D115-D)."""
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
    done = page.evaluate("t('copy.done')")
    row = page.locator("#nav-menu .menu-items > .menu-version")
    for sel, how in (("#version-short", "click"), ("#nav-menu .menu-version .copy-icon", "click"), ("#version-short", "Enter")):
        open_main_menu(page)
        assert row.locator(".copy-icon").count() == 1 and row.locator(".copy-icon").is_visible(), "the row's copy icon"
        page.evaluate("navigator.clipboard.writeText('')")
        with held_timers(page, "copied"):
            if how == "Enter":
                page.locator(sel).focus()
                page.keyboard.press("Enter")
            else:
                page.locator(sel).click()
            assert wait_until(page, "navigator.clipboard.readText().then((t) => t === state.version)", 3), f"{sel} by {how}: the whole version"
            where = wait_bubble_foot(page, done)
            assert where == "foot", f"{sel} by {how}: Copied at the window's foot: {where}"
            assert page.locator("#nav-menu").get_attribute("open") is None, f"{sel} by {how}: ☰ closed"
            assert page.evaluate("document.activeElement === document.querySelector('#nav-menu > summary')"), f"{sel} by {how}: the focus on ☰"


def test_main_menu_opens_each_page(page, case):
    """Each entry opens its page and its address, closes ☰, highlights it
    and marks the entry current; back returns to Rooms and ☰ is plain."""
    summary = page.locator("#nav-menu > summary")
    for view, (section, address, entry) in PAGES.items():
        show_view(page, view)
        page.locator(section).wait_for(state="visible")
        assert wait_until(page, f"location.hash.split('?')[0] === '{address}'", 3), f"{view} in the address"
        assert page.locator("#nav-menu").get_attribute("open") is None, "the choice closes ☰"
        assert summary.evaluate("(e) => e.classList.contains('active')"), f"☰ highlighted while {view} is shown"
        assert page.locator(entry).get_attribute("aria-current") == "page", f"{entry} current"
        others = [e for v, (_, _, e) in PAGES.items() if v != view]
        assert all(page.locator(e).get_attribute("aria-current") is None for e in others), "no other entry current"
        assert page.locator("#view-tiles").evaluate("(e) => !e.classList.contains('active')"), "Rooms not"
        page.go_back()
        page.locator("#rooms").wait_for(state="visible")
        assert wait_until(page, "location.hash === ''", 3), "back to Rooms"
        assert not summary.evaluate("(e) => e.classList.contains('active')"), "and ☰ no longer highlighted"


def test_an_address_opens_its_settings_page(page, case):
    """#cube, #maintenance and #info open their pages at a reload, as
    #system does."""
    for view in ("cube", "system", "maint", "info"):
        section, address, _ = PAGES[view]
        page.evaluate(f"history.replaceState(null, '', location.pathname + '{address}')")
        page.reload()
        page.locator(section).wait_for(state="visible", timeout=10000)


def test_a_viewer_has_its_entries_alone(page, case):
    """A viewer (/me stubbed) has the log, System info, Personal settings
    and Sign out in ☰; Cube settings, System settings and Maintenance
    neither there nor by their address, which shows Rooms."""
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(VIEWER)))
    try:
        page.evaluate("loadMe()")
        assert wait_until(page, "document.body.classList.contains('no-system')", 5), "a viewer"
        m = open_main_menu(page)
        for sel in ("#menu-log", "#menu-info", "#menu-me", "#menu-logout"):
            assert page.locator(sel).is_visible(), f"{sel} for a viewer"
        for sel in ("#menu-cube", "#menu-system", "#menu-maint"):
            assert css_hidden(page, sel), f"no {sel} for a viewer"
        assert page.locator("#menu-logout-label").inner_text() == page.evaluate("t('me.signOut')"), "Sign out for a session"
        m.locator("summary").click()
        for view in ("cube", "system", "maint"):
            page.evaluate(f"MassimoParts.pages.showView('{view}')")
            assert page.locator(PAGES[view][0]).is_hidden() and page.locator("#rooms").is_visible(), f"{view} gives way to Rooms"
        page.evaluate("MassimoParts.pages.showView('info')")
        page.locator("#info-page").wait_for(state="visible")
    finally:
        page.unroute("**/api/v1/me")


def test_an_open_server_has_no_sign_out(page, case):
    """On an open server (/me stubbed) there is nothing to sign out of:
    ☰ has no Sign out, as the personal page has no button, and no rule
    stands at its end."""
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json",
                                                           body=json.dumps({"mode": "open", "capabilities": ["read"]})))
    try:
        page.evaluate("loadMe()")
        assert wait_until(page, "document.getElementById('menu-logout').classList.contains('hidden')", 5), "no Sign out in ☰"
        assert page.locator("#logout-btn").evaluate("(e) => e.classList.contains('hidden')"), "as on the personal page"
        assert page.evaluate("document.getElementById('menu-app-sep').classList.contains('hidden')"), "and no rule left at ☰'s end"
    finally:
        page.unroute("**/api/v1/me")


def test_sign_out_from_the_menu(new_context, case, watch, guard):
    """☰'s Sign out does what the personal page's button does: the
    operator, signed in by name and password, signs out from ☰ and the
    sign-in is asked for again. A page of its own, without the test's
    token, as test_login.py's."""
    ctx = new_context()  # the case's viewport and locale
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    open_main_menu(page)
    assert page.locator("#menu-logout-label").inner_text() == page.evaluate("t('me.signOut')"), "Sign out for a session"
    page.locator("#menu-logout").click()
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in asked for again"
