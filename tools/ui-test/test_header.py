# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The header: the log button's symbol, the 👤 menu."""
from helpers import open_user_menu

# The symbol a page button shows, drawn in an svg of its own outside the
# header, so that its size does not depend on whether the button is shown;
# [width, height] of what it draws, in the symbol's 24-unit box.
SYMBOL_BOX = """(sel) => {
  const ns = 'http://www.w3.org/2000/svg';
  const svg = document.createElementNS(ns, 'svg');
  const use = document.createElementNS(ns, 'use');
  use.setAttribute('href', document.querySelector(sel + ' use').getAttribute('href'));
  svg.appendChild(use);
  document.body.appendChild(svg);
  const b = use.getBBox();
  svg.remove();
  return [b.width, b.height];
}"""


def test_log_button_shows_a_sheet(page, case):
    """The log button shows a sheet, taller than wide, its lines inside a
    frame: three lines alone read as a menu."""
    width, height = page.evaluate(SYMBOL_BOX, "#view-log")
    assert height >= 16 and height > width, f"the log's symbol is a sheet, taller than wide: {width} x {height}"


FORGET_KEY = {"en": "Forget key", "de": "Schlüssel vergessen"}
OPEN_LINE = {"en": "No users yet", "de": "Noch keine Benutzer"}


def menu_open(page):
    return page.locator("#user-menu").get_attribute("open") is not None


def test_user_menu_says_who_is_in(page, case):
    """👤 drops down who is in, here the owner by the suite's API key, My
    settings and Forget key; Escape, a click outside and a choice close it,
    and My settings opens the settings."""
    open_user_menu(page)
    line = page.locator("#me-line")
    assert line.is_visible() and "owner" in line.inner_text() and "MASSIMO_API_TOKEN" in line.inner_text(), \
        f"the menu names the key's user and label: {line.inner_text()!r}"
    assert page.locator("#logout-btn").inner_text() == FORGET_KEY[case.lang], "an API key is only forgotten"
    box = page.locator("#user-menu .menu-items").bounding_box()
    assert box["x"] >= 0 and box["x"] + box["width"] <= case.width, f"the menu fits the window: {box}"
    page.keyboard.press("Escape")
    assert not menu_open(page), "Escape closes the menu"
    open_user_menu(page)
    page.locator("h1").click()
    assert not menu_open(page), "a click outside closes it"
    open_user_menu(page)
    page.locator("#menu-prefs").click()
    assert not menu_open(page), "a choice closes it"
    assert page.locator("#settings").is_visible(), "My settings opens the settings"
    assert not page.locator("#settings #logout-btn, #settings #me-line").count(), "the settings no longer hold who is in"
    page.locator("#view-tiles").click()


def test_user_menu_on_an_open_server(page, case):
    """On an open server (/me stubbed) the menu says that it is open to
    everyone and offers My settings, without Sign out."""
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body='{"mode": "open"}'))
    try:
        page.evaluate("loadMe()")
        open_user_menu(page)
        assert OPEN_LINE[case.lang] in page.locator("#me-line").inner_text(), "the menu says the server is open"
        assert page.locator("#menu-prefs").is_visible() and not page.locator("#logout-btn").is_visible(), \
            "My settings, and nothing to sign out of"
    finally:
        page.unroute("**/api/v1/me")
        page.evaluate("loadMe()")


def test_room_menus_and_the_user_menu_one_at_a_time(page, case):
    """A room's ⋯ and 👤 are one open at a time."""
    room_menu = page.locator(".room details.menu:not(.hidden)").first
    room_menu.locator("summary").click()
    assert room_menu.get_attribute("open") is not None, "the room's menu opens"
    page.locator("#user-menu summary").click()
    assert menu_open(page) and room_menu.get_attribute("open") is None, "👤 opening closes the room's menu"
    room_menu.locator("summary").click()
    assert not menu_open(page), "and the other way round"
    page.keyboard.press("Escape")
