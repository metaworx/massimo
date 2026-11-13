# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The header: its order at full width and ☰ on a narrow screen, the log
button's symbol, the 👤 menu, the page shown in the address, and expert
mode's line of the chosen Cube's radio budget."""
import json

from helpers import VIEWER, css_hidden, open_user_menu, scroll_width, show_view, wait_until

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
    assert page.locator("#prefs").is_visible(), "My settings opens"
    assert page.locator("#user-menu summary").evaluate("e => e.classList.contains('active')"), "👤 is highlighted while it is open"
    assert not page.locator("#prefs #logout-btn, #prefs #me-line").count(), "the settings no longer hold who is in"
    show_view(page, "tiles")


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
        assert not page.locator("#menu-profile").is_visible(), "no profile without a user"
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


def shown(page, section):
    return page.locator(section).first.is_visible()


def test_the_address_names_the_page(page, case):
    """Each page puts its name into the address, Rooms none; back and
    forward move between the pages; a reload opens the page again, the
    header in view."""
    show_view(page, "log")
    assert page.evaluate("location.hash") == "#log", "the log in the address"
    show_view(page, "system")
    assert page.evaluate("location.hash") == "#system", "System settings in the address"
    show_view(page, "prefs")
    assert page.evaluate("location.hash") == "#prefs", "My settings in the address"
    page.go_back()
    assert wait_until(page, "!document.getElementById('system').classList.contains('hidden')", 5), "back: System settings"
    page.go_back()
    assert wait_until(page, "!document.getElementById('logview').classList.contains('hidden')", 5), "back: the log"
    page.go_back()
    assert wait_until(page, "location.hash === ''", 5) and shown(page, ".room"), "back: Rooms, without a page in the address"
    page.go_forward()
    assert wait_until(page, "!document.getElementById('logview').classList.contains('hidden')", 5), "forward: the log"
    page.reload()
    page.locator("#logview").wait_for(state="visible", timeout=10000)
    assert page.evaluate("location.hash") == "#log" and page.evaluate("window.scrollY") == 0, "a reload opens the log again, the header in view"
    assert wait_until(page, "document.querySelectorAll('#logview .log-line').length > 0", 10), "with its lines"
    show_view(page, "profile")
    page.reload()
    page.locator("#profile").wait_for(state="visible", timeout=10000)
    assert page.locator("#profile-display-name").is_visible(), "a reload opens My profile again, loaded"
    show_view(page, "tiles")
    assert page.evaluate("location.hash") == "", "Rooms takes the page out of the address"


def test_an_address_the_user_may_not_open_shows_rooms(page, case):
    """System settings in the address of a viewer (/me stubbed) shows
    Rooms, and the address drops it."""
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(VIEWER)))
    try:
        page.evaluate("history.replaceState(null, '', location.pathname + '#system')")
        page.reload()
        page.wait_for_selector(".room", timeout=10000)
        assert wait_until(page, "location.hash === ''", 5), "the address drops System settings"
        assert page.locator("#system").is_hidden() and css_hidden(page, "#view-settings"), "Rooms, and no ⚙"
    finally:
        page.unroute("**/api/v1/me")


def box(page, selector):
    return page.locator(selector).bounding_box()


def active(page, selector):
    return page.locator(selector).evaluate("(e) => e.classList.contains('active')")


def test_header_order_and_the_narrow_header(page, case):
    """At full width the brand and the time of the last update with ↻
    beside it at the left (the time under the brand where a long version
    leaves no room), then on the first line the Cube choice with the plug
    of its connection, then 🏠 👤 📄 ⚙, without ☰. Below 600 px 🏠 👤 ☰
    beside the brand and the Cube choice on a line of its own under them; ☰ holds Log and System settings, opens each, and is
    highlighted while one of them is shown."""
    order = ["h1", "#updated", "#refresh-btn", "#cube-choice", "#view-tiles", "#user-menu summary"]
    b = {sel: box(page, sel) for sel in order}
    assert b["#updated"]["x"] < b["#refresh-btn"]["x"], "↻ after the time it renews"
    assert page.locator("#cube-choice .picker-icon .conn").count() == 1, "the connection, a plug in the Cube choice"
    assert b["#view-tiles"]["x"] < b["#user-menu summary"]["x"], "🏠 left of 👤"
    if case.width >= 600:
        right = ["#cube-choice", "#view-tiles", "#user-menu summary", "#view-log", "#view-settings"]
        b.update({sel: box(page, sel) for sel in ("#view-log", "#view-settings")})
        xs = [b[sel]["x"] for sel in right]
        assert xs == sorted(xs), f"left to right in this order: {right}"
        assert max(b[sel]["x"] + b[sel]["width"] for sel in ("h1", "#updated", "#refresh-btn")) < b["#cube-choice"]["x"], \
            f"the brand and the time left of the Cube choice: {b}"
        first_line = b["h1"]["y"] + b["h1"]["height"]
        assert all(b[sel]["y"] < first_line for sel in right), f"the Cube choice and the pages on the first line: {b}"
        assert page.locator("#view-log").is_visible() and page.locator("#view-settings").is_visible(), "📄 and ⚙ shown"
        assert page.locator("#nav-menu").is_hidden(), "no ☰"
        return
    assert page.locator("#nav-menu").is_visible(), "☰ shown"
    assert page.locator("#view-log").is_hidden() and page.locator("#view-settings").is_hidden(), "📄 and ⚙ in it"
    assert b["#user-menu summary"]["x"] < box(page, "#nav-menu summary")["x"], "🏠 👤 ☰"
    assert b["#cube-choice"]["y"] >= b["#view-tiles"]["y"] + b["#view-tiles"]["height"], "the Cube choice under them"
    for view, section in (("log", "#logview"), ("system", "#system")):
        page.locator("#nav-menu summary").click()
        page.locator("#menu-" + view).click()
        page.locator(section).wait_for(state="visible")
        assert page.locator("#nav-menu").get_attribute("open") is None, "the choice closes ☰"
        assert active(page, "#nav-menu summary"), f"☰ highlighted while {view} is shown"
    show_view(page, "tiles")
    assert not active(page, "#nav-menu summary"), "and not on Rooms"


def test_menu_icons_and_user_title(page, case):
    """The 👤 menu's items and ☰'s each show their icon; Forget key a key's;
    the 👤 button's tooltip names who is signed in, here the owner of the
    test's API key."""
    open_user_menu(page)
    for sel in ("#menu-profile", "#menu-prefs", "#logout-btn"):
        assert page.locator(sel + " > svg.ni").count() == 1, f"{sel} shows its icon"
    assert page.locator("#logout-icon").get_attribute("href") == "#icon-key", "Forget key shows a key"
    title = page.locator("#user-menu-btn").get_attribute("title") or ""
    assert "owner" in title, f"the 👤 tooltip names the user ({title!r})"
    assert page.locator("#user-menu-btn").get_attribute("aria-label") == title, "and says it to a screen reader"
    page.keyboard.press("Escape")
    for sel in ("#menu-log", "#menu-system"):
        assert page.locator(sel + " > svg.ni").count() == 1, f"{sel} shows its icon"


DUTY = {"en": "Duty cycle", "de": "Duty Cycle"}
QUEUED = {"en": "Queued", "de": "Warteschlange"}


def test_expert_line_of_the_cube(page, case):
    """In expert mode a line under the header's first: the chosen Cube's
    name, duty cycle, free slots and pending, queued only while commands
    wait; it stays when the header compacts, on every page, and is gone
    outside expert mode, where the Cube's clock shows at the top all the same."""
    line = page.locator("#cube-stats")
    assert line.is_hidden(), "no line outside expert mode"
    page.evaluate("() => { state.settings.expert = true; applySettings(); }")
    assert line.is_visible(), "the line in expert mode"
    name = page.evaluate("(() => { const c = state.cubes.find((x) => x.serial === state.serial); return c.name || c.serial; })()")
    assert page.locator("#stats-cube").inner_text() == name, "the chosen Cube's name first"
    text = line.inner_text()
    assert DUTY[case.lang] in text and "%" in text, f"the duty cycle ({text!r})"
    assert page.locator("#stat-queued").is_hidden(), "queued only while commands wait"
    page.evaluate("""() => {
      const c = Object.assign({}, state.cubes.find((x) => x.serial === state.serial), { queued: 2 });
      renderHeader(c);
    }""")
    assert page.locator("#stat-queued").is_visible() and QUEUED[case.lang] + " 2" in line.inner_text(), "queued while commands wait"
    assert page.locator("#clock-plain").is_visible(), "the Cube's clock at the top in expert mode too"
    page.evaluate("document.body.classList.add('scrolled')")
    assert line.is_visible(), "the line stays when the header compacts"
    page.evaluate("document.body.classList.remove('scrolled')")
    show_view(page, "log")
    assert line.is_visible(), "on every page"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the line"
    show_view(page, "tiles")
    page.evaluate("() => { state.settings.expert = false; applySettings(); }")
    assert line.is_hidden(), "gone outside expert mode"
