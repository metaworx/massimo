# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The header: its order, its fold where it is short, the log button's
symbol, its baseline, the pile's list and the reads' times, the personal
page 👤 opens, the page shown in the address, each page's own scroll
position, and expert mode's line of the chosen Cube's radio budget."""
import json

from helpers import (CUBES, VIEWER, assert_busy, css_hidden, held_poll, hold, open_personal_page, scaled, scroll_settled, scroll_width, scrolled_to,
                     settled, show_view, wait_held, wait_until)

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
    """☰'s log entry shows a sheet, taller than wide, its lines inside a
    frame: three lines alone read as a menu."""
    width, height = page.evaluate(SYMBOL_BOX, "#menu-log")
    assert height >= 16 and height > width, f"the log's symbol is a sheet, taller than wide: {width} x {height}"


FORGET_KEY = {"en": "Forget key", "de": "Schlüssel vergessen"}
OPEN_LINE = {"en": "No users yet", "de": "Noch keine Benutzer"}


CARDS = ["profile", "prefs", "notify"]
CARD_ICONS = {"profile": "#icon-id", "prefs": "#icon-sliders", "notify": "#icon-bell"}


def test_personal_page_says_who_is_in(page, case):
    """☰ → Personal settings opens the personal page, ☰ highlighted while it is shown: at
    its top who is in, here the owner by the suite's API key, and Forget
    key left of ☰; ☰ lists My profile, My settings and Notifications with
    the icons their cards' heads show; the cards side by side from 900 px,
    one under the other on a phone."""
    open_personal_page(page)
    assert page.evaluate("location.hash") == "#me", "the page in the address"
    assert active(page, "#nav-menu > summary"), "☰ highlighted while the page is shown"
    line = page.locator("#me-line")
    assert line.is_visible() and "owner" in line.inner_text() and "MASSIMO_API_TOKEN" in line.inner_text(), \
        f"the page names the key's user and label: {line.inner_text()!r}"
    logout = page.locator("#logout-btn")
    assert logout.inner_text() == FORGET_KEY[case.lang], "an API key is only forgotten"
    out, menu = box(page, "#logout-btn"), box(page, "#me .sec-menu > summary")
    assert out["x"] + out["width"] <= menu["x"] and abs(out["y"] - menu["y"]) <= 2, f"Forget key left of ☰, on its row: {out} {menu}"
    assert menu["x"] + menu["width"] <= case.width, "☰ at the right end"
    page.locator("#me .sec-menu > summary").click()
    items = page.locator("#me .sec-menu .toc-link")
    assert [items.nth(i).get_attribute("data-target") for i in range(items.count())] == CARDS, "☰ lists the three cards"
    for card in CARDS:
        item = page.locator(f"#me .sec-menu .toc-link[data-target='{card}'] svg use")
        head = page.locator(f"#{card} .section-head svg.section-icon use")
        assert item.get_attribute("href") == head.get_attribute("href") == CARD_ICONS[card], f"{card}: the same icon in ☰ and its head"
    page.keyboard.press("Escape")
    lefts = {page.evaluate(f"Math.round(document.getElementById('{c}').getBoundingClientRect().left)") for c in CARDS}
    if case.width >= 900:
        assert len(lefts) == 2, f"two columns from 900 px ({lefts})"
    else:
        assert len(lefts) == 1, f"one column on a phone ({lefts})"
    assert scroll_width(page) <= case.width, "no horizontal scroll on the personal page"
    show_view(page, "tiles")
    assert not active(page, "#nav-menu > summary"), "and not on Rooms"


def test_personal_page_on_an_open_server(page, case):
    """On an open server (/me stubbed) the personal page says that it is
    open to everyone and has My settings and Notifications, without My
    profile and without Sign out."""
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body='{"mode": "open"}'))
    try:
        page.evaluate("loadMe()")
        open_personal_page(page)
        assert OPEN_LINE[case.lang] in page.locator("#me-line").inner_text(), "the page says the server is open"
        assert page.locator("#prefs").is_visible() and not page.locator("#logout-btn").is_visible(), \
            "My settings, and nothing to sign out of"
        assert not page.locator("#profile").is_visible(), "no profile without a user"
        page.locator("#me .sec-menu > summary").click()
        assert page.locator("#me .sec-menu .toc-link[data-target='profile']").count() == 0, "nor in ☰"
        page.keyboard.press("Escape")
    finally:
        page.unroute("**/api/v1/me")
        page.evaluate("loadMe()")
    show_view(page, "tiles")


def test_room_menus_one_at_a_time(page, case):
    """A room's ⋯ and the Cube choice are one open at a time."""
    room_menu = page.locator(".room details.menu:not(.hidden)").first
    cube_menu = page.locator("#cube-menu")
    room_menu.locator("summary").click()
    assert room_menu.get_attribute("open") is not None, "the room's menu opens"
    cube_menu.locator("summary").click()
    assert cube_menu.get_attribute("open") is not None and room_menu.get_attribute("open") is None, \
        "the Cube choice opening closes the room's menu"
    room_menu.locator("summary").click()
    assert cube_menu.get_attribute("open") is None, "and the other way round"
    page.keyboard.press("Escape")


def shown(page, section):
    return page.locator(section).first.is_visible()


def test_the_address_names_the_page(page, case):
    """Each page puts its name into the address, Rooms none; back and
    forward move between the pages; a reload opens the page again, the
    header in view. What a reload brings is waited for, not looked at
    once after the page is shown: the page's start goes on after it."""
    show_view(page, "log")
    assert page.evaluate("location.hash") == "#log", "the log in the address"
    show_view(page, "system")
    assert page.evaluate("location.hash") == "#system", "System settings in the address"
    show_view(page, "me")
    assert page.evaluate("location.hash") == "#me", "the personal page in the address"
    page.go_back()
    assert wait_until(page, "!document.getElementById('system').classList.contains('hidden')", 5), "back: System settings"
    page.go_back()
    assert wait_until(page, "!document.getElementById('logview').classList.contains('hidden')", 5), "back: the log"
    page.go_back()
    assert wait_until(page, "location.hash === ''", 5) and settled(page, lambda: shown(page, ".room"), bool)[0], \
        "back: Rooms, without a page in the address"
    page.go_forward()
    assert wait_until(page, "!document.getElementById('logview').classList.contains('hidden')", 5), "forward: the log"
    page.reload()
    page.locator("#logview").wait_for(state="visible", timeout=scaled(10) * 1000)
    assert wait_until(page, "location.hash === '#log' && window.scrollY === 0", 5), "a reload opens the log again, the header in view"
    assert wait_until(page, "document.querySelectorAll('#logview .log-line').length > 0", 10), "with its lines"
    page.evaluate("history.replaceState(null, '', location.pathname + '#profile')")
    page.reload()
    page.locator("#me").wait_for(state="visible", timeout=scaled(10) * 1000)
    assert settled(page, lambda: shown(page, "#profile-display-name"), bool)[0], \
        "My profile's address opens the personal page, My profile loaded"
    assert page.evaluate("location.hash") == "#profile", "and stays in the address"
    page.evaluate("history.replaceState(null, '', location.pathname + '#notify')")
    page.reload()
    page.locator("#me").wait_for(state="visible", timeout=scaled(10) * 1000)
    assert wait_until(page, """(() => {
        const bar = document.querySelector('#me .page-bar').getBoundingClientRect().bottom;
        const y = document.getElementById('notify').getBoundingClientRect().top;
        return Math.abs(y - bar) <= 40;
    })()""", 5), "Notifications' address brings its card under the row"
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
        assert page.locator("#system").is_hidden() and css_hidden(page, "#menu-system"), "Rooms, and no System settings in ☰"
    finally:
        page.unroute("**/api/v1/me")


def box(page, selector):
    return page.locator(selector).bounding_box()


def edge(page, selector):
    """An element's lower edge."""
    b = box(page, selector)
    return b["y"] + b["height"]


def active(page, selector):
    return page.locator(selector).evaluate("(e) => e.classList.contains('active')")


def test_header_order(page, case):
    """The header's row: the logo at the left (its M where the row is
    short); at the right the pile (every Cube's problems, the server's
    sign first), the Cube choice, its cube in its link's colour, then 🏠
    and ☰ alone, all on one lower edge, the Cube choice as near to 🏠 as
    the pages are to each other (AP GlobalMenu, D199-A to D201-A)."""
    order = ["h1", ".pile", "#cube-choice", "#view-tiles", "#nav-menu > summary"]
    b = {sel: box(page, sel) for sel in order}
    row = box(page, ".top-line")
    assert page.locator("#cube-choice .picker-icon .cube-icon.tone-ok use[href='#icon-cube3d']").count() == 1, \
        "the cube before the Cube's name, green while connected"
    assert page.locator("#cube-choice .conn-sign").count() == 0, "no network sign in the closed choice"
    shown = page.locator(".nav > *").evaluate_all("(es) => es.filter((e) => e.offsetParent).map((e) => e.id)")
    assert shown == ["view-tiles", "nav-menu"], f"🏠 and ☰ alone in the header: {shown}"
    assert abs(b["h1"]["x"] - row["x"]) <= 1, f"the logo at the left: {b['h1']} vs {row}"
    menu = b["#nav-menu > summary"]
    assert abs(menu["x"] + menu["width"] - (row["x"] + row["width"])) <= 1, f"☰ at the right: {b}"
    xs = [b[sel]["x"] for sel in order]
    assert xs == sorted(xs), f"left to right in this order: {order}: {b}"
    bottom = b["#view-tiles"]["y"] + b["#view-tiles"]["height"]
    assert all(abs(b[sel]["y"] + b[sel]["height"] - bottom) <= 1 for sel in order[1:]), f"on one lower edge: {b}"
    assert b[".pile"]["x"] - (b["h1"]["x"] + b["h1"]["width"]) > 16, f"the pile away from the logo, at the right: {b}"
    pages = box(page, "#nav-menu > summary")["x"] - (b["#view-tiles"]["x"] + b["#view-tiles"]["width"])
    choice = b["#view-tiles"]["x"] - (b["#cube-choice"]["x"] + b["#cube-choice"]["width"])
    assert abs(choice - pages) <= 1, f"the Cube choice as near to 🏠 as the pages to each other: {choice} vs {pages}"


# The header's fold at the window's width, the row's height and whether
# anything sticks out of the window.
FOLD = """() => ({fold: document.querySelector('.top-line').dataset.fold || '',
  wide: document.querySelector('header.top').scrollWidth, row: document.querySelector('.top-line').clientWidth})"""
STEPS = ["", "logo", "logo cube", "logo cube pile"]


def test_header_folds_step_by_step(page, case):
    """Where the row is short it folds, step by step, the first that fits:
    the logo becomes the M, then the Cube choice a cube button, then the
    pile goes to the next row, at the left; the M always there; compacted
    the same, ↑ among the pages; nothing outside the window. The cube
    button names the chosen Cube in its tooltip and opens the list."""
    size = page.viewport_size
    try:
        last = 0
        for w in (1100, 900, 700, 600, 500, 450, 400, 390, 360, 340, 320):
            page.set_viewport_size({"width": w, "height": size["height"]})
            folds = []
            for scrolled in (False, True):
                page.evaluate(f"document.body.classList.toggle('scrolled', {str(scrolled).lower()})")
                assert wait_until(page, "document.querySelector('.top-line').dataset.fold !== 'measure'", 2), "measured"
                page.wait_for_timeout(100)
                f = page.evaluate(FOLD)
                assert f["wide"] <= f["row"] + 1, f"at {w} px, compacted {scrolled}: nothing outside the window: {f}"
                assert page.locator(".top-line > .brand").is_visible(), f"at {w} px the logo or its M always there"
                folds.append(f["fold"])
            assert folds[0] == folds[1], f"at {w} px the same fold compacted: {folds}"
            step = STEPS.index(folds[0])
            assert step >= last, f"at {w} px the steps in their order: {folds[0]!r} after {STEPS[last]!r}"
            last = step
            logo, mark = page.locator(".top .brand-name .brand-logo"), page.locator(".top .brand-name .brand-mark")
            assert (mark if step else logo).is_visible() and (logo if step else mark).is_hidden(), f"at {w} px the logo or the M"
            label = page.locator("#cube-choice .picker-label")
            assert label.is_hidden() == (step >= 2), f"at {w} px the Cube choice a cube button: {step >= 2}"
            assert page.locator("#cube-choice .cube-icon").is_visible(), f"at {w} px the cube always"
            pile, home = box(page, ".pile"), box(page, "#view-tiles")
            on_next = pile["y"] >= home["y"] + home["height"] - 1
            assert on_next == (step == 3), f"at {w} px the pile on the next row: {step == 3}"
            if step == 3:
                assert abs(box(page, "#cube-problems .problems-sum")["x"] - box(page, ".top-line")["x"]) <= 1, "at the left"
        assert last == 3, "every step at 320 px"
        page.evaluate("document.body.classList.remove('scrolled')")
        name = page.evaluate("(() => { const c = state.cubes.find((x) => x.serial === state.serial); return c.name || c.serial; })()")
        assert page.locator("#cube-choice").get_attribute("title").startswith(name), "the cube button's tooltip names the chosen Cube"
        page.locator("#cube-choice").click()
        page.locator("#cube-menu .menu-items").wait_for(state="visible")
        page.keyboard.press("Escape")
    finally:
        page.evaluate("document.body.classList.remove('scrolled')")
        page.set_viewport_size(size)


# The drawing shown in the header's brand, the logo or the M: its box.
BRAND_BOX = """() => {
  const svg = [...document.querySelectorAll('.top .brand-name svg')].find((e) => e.getBoundingClientRect().width > 0);
  const b = svg.getBoundingClientRect();
  return {top: b.top, bottom: b.bottom, height: b.height, width: b.width};
}"""

VERSION_TITLE = {"en": "{v} (server version) · click to copy", "de": "{v} (Server-Version) · Klick kopiert"}


# A build's version, as long as one between releases.
BUILD = "v0.15.0-alpha.1-293-gd36148f"

def test_version_in_the_menu(page, case, reads):
    """The server's version is ☰'s first row, before the log, small and
    grey, after Massimo's icon, a rule under it, whole for a build's long
    one; its tooltip is the whole version. It is not in the header's row. The server runs the build for the page
    from its load on: its reads of /healthz, its poll's among them, answer
    the build's version (helpers.Reads), so that the page never takes the
    server for one that runs another; the page closes with it."""
    reads.change("**/healthz", lambda health: dict(health, version=BUILD))
    page.reload()
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.version === %r" % BUILD, 5), "the page loaded with the build's version"
    page.locator("#nav-menu > summary").click()
    v = page.locator("#version-short")
    v.wait_for(state="visible")
    first = page.locator("#nav-menu .menu-items > *").first
    assert first.evaluate("(e) => e.contains(document.getElementById('version-short'))"), "☰'s first row"
    assert v.bounding_box()["y"] < page.locator("#menu-log").bounding_box()["y"], "above the log"
    assert v.evaluate("(e) => e.scrollWidth <= e.clientWidth + 1"), "whole"
    assert v.get_attribute("title") == VERSION_TITLE[case.lang].format(v=BUILD), "the tooltip is the whole version"
    icon = first.locator(".menu-brand .brand-mark")
    assert wait_until(page, "!!document.querySelector('#nav-menu .menu-brand .brand-mark')", 5), "Massimo's icon drawn"
    assert icon.is_visible() and first.locator(".menu-brand .brand-logo").is_hidden(), "the icon, not the logo"
    assert icon.bounding_box()["x"] + icon.bounding_box()["width"] <= v.bounding_box()["x"], "before the version"
    assert first.evaluate("(e) => e.nextElementSibling.tagName") == "HR", "a rule under the row"
    assert page.locator("#login-version").is_hidden(), "not the sign-in card's while signed in"
    page.keyboard.press("Escape")


def test_header_on_one_baseline(page, case):
    """The row stands on one line: the logo's drawing (the M where the row
    is short) with its bottom edge, the letters' baseline, the pill of the
    pile, the Cube choice and the page buttons end there; the M is as tall
    as the page buttons."""
    brand = page.evaluate(BRAND_BOX)
    button = box(page, "#view-tiles")
    assert abs(brand["height"] - button["height"]) <= 1, f"the M as tall as the page buttons: {brand} vs {button}"
    bottom = button["y"] + button["height"]
    ends = {"logo": brand["bottom"], "pill": edge(page, "#cube-problems .problems-sum"), "cube": edge(page, "#cube-choice")}
    assert all(abs(y - bottom) <= 1 for y in ends.values()), f"on the buttons' lower edge {bottom}: {ends}"


def test_cube_clock_in_its_facts(page, case):
    """The Cube's clock is not in the header: its facts show it, moved on
    from when the server read it, and how far it is off where that is
    90 s or more."""
    assert page.locator("#clock-plain").count() == 0, "no Cube clock in the header"
    got = page.evaluate("""() => {
      const c = state.cubes.find((x) => x.serial === state.serial);
      const read = cubeInstant(c.cube_time, state.tz[c.serial]);
      if (!read) return null;
      const plain = clockText(Object.assign({}, c, { cube_time_at: read.toISOString() })).text;
      const late = clockText(Object.assign({}, c, { cube_time_at: new Date(read.getTime() + 10 * 60000).toISOString() })).text;
      return { plain, late, note: tn("budget.clockBehind", 10) };
    }""")
    assert got, "the fake Cube's clock and time zone are known"
    assert "(" not in got["plain"], f"no note where the clock is right: {got}"
    assert got["late"].endswith("(" + got["note"] + ")"), f"read 10 minutes late, the clock is 10 min behind: {got}"


def test_menu_icons_and_user_title(page, case):
    """Forget key shows a key's icon; ☰'s items each their page's; ☰'s
    Personal settings names whose personal page it opens in its tooltip,
    here the owner of the test's API key."""
    open_personal_page(page)
    assert page.locator("#logout-btn > svg.ni").count() == 1, "Forget key shows its icon"
    assert page.locator("#logout-icon").get_attribute("href") == "#icon-key", "Forget key shows a key"
    title = page.locator("#menu-me").get_attribute("title") or ""
    assert "owner" in title, f"Personal settings' tooltip names the user ({title!r})"
    for sel in ("#menu-log", "#menu-cube", "#menu-system", "#menu-info", "#menu-me", "#menu-logout"):
        assert page.locator(sel + " > svg.ni").count() == 1, f"{sel} shows its icon"


DUTY = {"en": "Duty cycle", "de": "Duty Cycle"}
QUEUED = {"en": "Queued", "de": "Warteschlange"}


def test_expert_line_of_the_cube(page, case, reads):
    """In expert mode a line under the header's first: the chosen Cube's
    name, how long ago it was read and the server replied, duty cycle,
    free slots and pending, queued only while commands wait; it stays when the header compacts, on every page, and is gone
    outside expert mode."""
    line = page.locator("#cube-stats")
    assert line.is_hidden(), "no line outside expert mode"
    page.evaluate("() => { state.settings.expert = true; applySettings(); }")
    assert line.is_visible(), "the line in expert mode"
    name = page.evaluate("(() => { const c = state.cubes.find((x) => x.serial === state.serial); return c.name || c.serial; })()")
    assert page.locator("#stats-cube").inner_text() == name, "the chosen Cube's name first"
    times = page.locator("#cube-stats .read-time")
    assert times.count() == 2, "how long ago the Cube was read and the server replied"
    labels = [times.nth(i).locator("span").first.inner_text() for i in range(2)]
    assert labels == page.evaluate("[t('header.cubeRead'), t('header.serverReplied')]"), f"named so: {labels}"
    assert box(page, "#stats-cube")["x"] < times.first.bounding_box()["x"], "after the Cube's name"
    text = line.inner_text()
    assert DUTY[case.lang] in text and "%" in text, f"the duty cycle ({text!r})"
    assert page.locator("#stat-queued").is_hidden(), "queued only while commands wait"
    reads.cube(page.evaluate("state.serial"), queued=2)
    page.evaluate("loadCubes()")
    assert page.locator("#stat-queued").is_visible() and QUEUED[case.lang] + " 2" in line.inner_text(), "queued while commands wait"
    page.evaluate("document.body.classList.add('scrolled')")
    assert line.is_visible(), "the line stays when the header compacts"
    page.evaluate("document.body.classList.remove('scrolled')")
    show_view(page, "log")
    assert line.is_visible(), "on every page"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the line"
    show_view(page, "tiles")
    reads.clear(CUBES)
    page.evaluate("() => { state.settings.expert = false; applySettings(); }")
    assert line.is_hidden(), "gone outside expert mode"


def test_no_buttons_before_the_session_is_known(page, case, watch):
    """The page starts signed out (login.js loadMe): while the server has
    not said who is in, the header shows the name and none of its buttons,
    so that none flickers before a sign-in; once it has, they show. A
    server that answers with an error shows the page all the same, with
    what it can say."""
    held, sent = [], []
    hold(page, "**/api/v1/me", "GET", held, sent)
    page.reload()
    assert wait_held(page, held), "the page asks who is in"
    for part in (".top-line > .nav", ".top-line .cube"):
        assert page.locator(part).is_hidden(), f"{part} hidden before the answer"
    assert page.locator(".top .brand-name").is_visible(), "the name shows"
    held[0].continue_()
    page.unroute("**/api/v1/me")
    assert wait_until(page, "!document.body.classList.contains('signed-out')", 5), "shown once the server has answered"
    assert page.locator(".top-line > .nav").is_visible() and page.locator(".top-line .cube").is_visible(), "the buttons once signed in"
    watch.expected.append(("/api/v1/me", 500))
    page.route("**/api/v1/me", lambda route: route.fulfill(status=500, content_type="application/json", body='{"error":"test"}'))
    page.reload()
    assert wait_until(page, "!document.body.classList.contains('signed-out')", 10), "a server that cannot say shows the page all the same"
    page.unroute("**/api/v1/me")
    page.reload()
    page.wait_for_selector(".room", timeout=10000)


# Room to scroll on every page, whatever the window's height: a spacer at
# the end of each page's section.
SPACERS = """() => { for (const id of ['rooms', 'system']) {
  const d = document.createElement('div');
  d.className = 'test-spacer';
  d.style.height = '2000px';
  document.getElementById(id).appendChild(d);
} }"""


def test_each_page_keeps_its_scroll(page, case):
    """Each page comes back where it was left: Rooms scrolled, System
    settings opened at its top the first time, scrolled on its own, and
    each of them back at its own place when it is shown again, by its
    button and by the browser's back button."""
    page.evaluate(SPACERS)
    try:
        show_view(page, "tiles")
        house = scrolled_to(page, 400)
        assert house == 400, "Rooms scrolled"
        show_view(page, "system")
        assert scroll_settled(page), "System settings comes to rest"
        assert page.evaluate("window.scrollY") == 0, "System settings at its top the first time"
        system = scrolled_to(page, 250)
        show_view(page, "tiles")
        assert scroll_settled(page), "Rooms comes to rest"
        assert page.evaluate("window.scrollY") == house, "Rooms back where it was left"
        show_view(page, "system")
        assert scroll_settled(page), "System settings again comes to rest"
        assert page.evaluate("window.scrollY") == system, "System settings back at its own place"
        page.go_back()
        page.locator("#rooms").wait_for(state="visible")
        assert scroll_settled(page), "Rooms after back comes to rest"
        assert page.evaluate("window.scrollY") == house, "the back button too"
    finally:
        page.evaluate("document.querySelectorAll('.test-spacer').forEach((e) => e.remove()); window.scrollTo({top: 0, behavior: 'instant'})")
        show_view(page, "tiles")


def test_versions_copy(page, case):
    """☰'s version and System Info's copy the whole version, as
    the tooltip names it, without " (server version)"; System Info's by a
    click on it and by its icon (D94-A)."""
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
    whole = page.evaluate("state.version")
    page.evaluate("navigator.clipboard.writeText('')")
    page.locator("#nav-menu > summary").click()
    page.locator("#version-short").click()
    assert wait_until(page, "navigator.clipboard.readText().then((t) => t === state.version)", 3), "☰'s version copies whole"
    assert page.evaluate("document.getElementById('nav-menu').open"), "☰ stays open"
    page.keyboard.press("Escape")
    assert "(" not in whole or "server version" not in whole, "without the tooltip's words"
    show_view(page, "info")
    cell = page.locator("#info-version")
    assert wait_until(page, "!!document.querySelector('#info-version .copy-icon')", 5), "System Info's version with its icon"
    page.evaluate("navigator.clipboard.writeText('')")
    cell.locator(".copy-icon").click()
    assert wait_until(page, "navigator.clipboard.readText().then((t) => t !== '')", 3), "its icon copies"
    copied = page.evaluate("navigator.clipboard.readText()")
    system = page.evaluate("api('GET', 'api/v1/system').then((v) => v.version)")
    assert copied == system and len(copied) >= len(cell.locator(".mono").inner_text().strip()), \
        f"the whole version, not the short one shown ({copied!r} vs {system!r})"
    show_view(page, "tiles")


AGES = ((5, 5, "second"), (125, 2, "minute"), (3 * 3600 + 5, 3, "hour"), (3 * 86400, 3, "day"))
# The chosen Cube read ago seconds before now on a server's clock skew ms
# ahead: what the list says of it, its tooltip, and what it should say.
# The Cube's entry is written whole into state.cubes, as the page's loads
# write it, so that the parts that follow the Cubes draw it. Written in
# place, it was drawn only where the render's clock (nowAgain) moved on,
# which it does not where the header's tick came in the same millisecond:
# the list then still said what it said at the tick.
SET_READ = """async ([ago, skew]) => {
  serverSkew = skew;
  const updated = new Date(Date.now() + skew - ago * 1000).toISOString();
  state.cubes = state.cubes.map((c) => (c.serial === state.serial ? { ...c, updated } : c));
  MassimoParts.header.render();
  await new Promise((done) => setTimeout(done)); // the part draws after the call
  const v = document.querySelector('#cube-problems .read-times > :nth-child(2)');
  return [v.textContent, v.title, fmtTimeOfDay(new Date(Date.parse(updated) - skew))];
}"""
# How long ago, in the page's language, as the header says it.
AGO = "([n, unit]) => new Intl.RelativeTimeFormat(i18n.lang, {numeric: 'always', style: 'short'}).format(-n, unit)"
# The values of the list's times and of the expert line's, as they show.
TIMES = """() => [
  [...document.querySelectorAll('#cube-problems .read-times > :nth-child(even)')].map((e) => e.textContent),
  [...document.querySelectorAll('#cube-stats .read-time > span:last-child')].map((e) => e.textContent),
]"""


def test_reads_times_ago(page, case):
    """How long ago the chosen Cube was read (its updated, on the server's
    clock) and the server last replied, relative in the page's language:
    seconds under a minute, then minutes, hours, days; the time of day the
    tooltip; counting on by one clock, the list's and the expert line's the
    same. The server's clock is taken from its answers. The page's poll is
    held meanwhile (held_poll), so that it does not bring the Cube's own
    time back over the one SET_READ writes."""
    assert wait_until(page, "Math.abs(serverSkew) < 3000", 5), "the server's clock from its answers: this machine's"
    assert page.locator(".pile-age").count() == 0, "no age above the pile"
    page.evaluate("() => { state.settings.expert = true; applySettings(); }")  # before the hold: applySettings restarts the running poll
    page.locator("#cube-problems .problems-sum").click()  # open, the list's times move on
    try:
        with held_poll(page):
            for ago, n, unit in AGES:
                text, title, clock = page.evaluate(SET_READ, [ago, 0])
                assert text == page.evaluate(AGO, [n, unit]), f"{ago} s ago: {text!r}"
                assert title == clock, f"its time of day the tooltip: {title!r} vs {clock!r}"
            text, _, _ = page.evaluate(SET_READ, [90, 60000])
            assert text == page.evaluate(AGO, [1, "minute"]), f"on the server's clock, a minute ahead: {text!r}"
            page.evaluate(SET_READ, [3, 0])
            want = page.evaluate(AGO, [5, "second"])
            assert wait_until(page, "document.querySelector('#cube-problems .read-times > :nth-child(2)').textContent === %r" % want, 4), \
                "it counts on"
            listed, expert = page.evaluate(TIMES)
            assert expert == listed[:2], f"the expert line's times are the list's: {expert} vs {listed}"
    finally:
        page.keyboard.press("Escape")
        page.evaluate("() => { state.settings.expert = false; applySettings(); poll(true); }")  # the Cube's own time back


# The list's times as they are drawn, and again after a timer of the
# page's own due after any tick the header's clock would make: a list
# whose times still move shows it.
TIMES_STILL = """async () => {
  const text = () => document.querySelector('#cube-problems .read-times').textContent;
  await new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done)));
  const still = text();
  await new Promise((done) => setTimeout(done, 1100));
  return [still, text()];
}"""

# A read of the chosen Cube's rooms, which Refresh now's poll makes
# (poll(true)) and the page's own poll does not while the stream is open.
ROOMS_READ = "/rooms?devices=all"


def test_refresh_now_keeps_the_list_open(page, case):
    """The pile's list starts with Refresh now, under it three rows of a
    label and its time: Cube read, Server replied, Current time, the times
    right-aligned under each other, the current time moving on while the
    list is open. A click polls, the rooms too, turns while it waits for
    the server and leaves the list open, which then shows the server's
    answer; closed, the times stand still. The page's poll is held
    meanwhile (held_poll): the click's poll is then the only one."""
    with held_poll(page):
        page.locator("#cube-problems .problems-sum").click()
        first = page.locator("#cube-problems .problems-list > *").first
        first.wait_for(state="visible")
        assert "refresh-now" in (first.get_attribute("class") or ""), "Refresh now first"
        assert first.locator(".menu-item-label").inner_text() == page.evaluate("t('header.refreshTitle')"), "named so"
        cells = first.locator(".read-times > span")
        labels = [cells.nth(i).inner_text() for i in (0, 2, 4)]
        assert labels == page.evaluate("['header.cubeRead', 'header.serverReplied', 'header.timeNow'].map((k) => t(k))"), f"the labels: {labels}"
        rights = [cells.nth(i).bounding_box() for i in (1, 3, 5)]
        assert max(b["x"] + b["width"] for b in rights) - min(b["x"] + b["width"] for b in rights) <= 1, f"the times right-aligned: {rights}"
        ys = [b["y"] for b in rights]
        assert ys == sorted(ys) and ys[1] - ys[0] > 8, f"a row each: {ys}"
        now = cells.nth(5).inner_text()
        assert wait_until(page, "document.querySelector('#cube-problems .read-times > :nth-child(6)').textContent !== %r" % now, 3), \
            "the current time moves on"
        replied = page.evaluate("state.repliedAt")
        rooms_read = []
        page.on("request", lambda r: rooms_read.append(r.url) if r.method == "GET" and ROOMS_READ in r.url else None)
        held, sent = [], []
        hold(page, CUBES, "GET", held, sent)  # the click's read of the Cubes held, as a slow server would
        try:
            first.click()
            assert wait_held(page, held), "a click polls: the Cubes are read"
            assert_busy(page, "#cube-problems .refresh-now", "Refresh now")
            for route in held:
                route.continue_()
            held.clear()
            assert wait_until(page, "!document.querySelector('#cube-problems .refresh-now').classList.contains('busy') "
                                    f"&& state.repliedAt > {replied}", 5), "the server answered: Refresh now turns no more"
        finally:
            for route in held:
                route.continue_()
            page.unroute(CUBES)
        assert rooms_read, "the click's poll reads the rooms too, as the page's own poll does not while the stream is open"
        assert wait_until(page, "document.querySelector('#cube-problems .read-times > :nth-child(4)').title "
                                "=== fmtTimeOfDay(new Date(state.repliedAt))", 3), "the list shows the server's answer"
        assert page.evaluate("document.querySelector('#cube-problems details').open"), "and leaves the list open"
        page.keyboard.press("Escape")
        assert wait_until(page, "!document.querySelector('#cube-problems details').open", 3), "Escape closes it"
        still, later = page.evaluate(TIMES_STILL)
        assert later == still, f"closed, the times stand still: {still!r}, then {later!r}"


def test_pile_list_as_wide_as_its_content(page, case):
    """The pile's list is as wide as its entries need, up to the page's
    width (the body's, less its gutters), under the pill, inside the
    window, once placed: components.js placeMenu sets its height's cap
    as it places it, as test_menu_fit.py waits for it. Before, the list
    is where the style sheet puts it, right-aligned to the pill, which
    at 390 px is out of the window."""
    page.locator("#cube-problems .problems-sum").click()
    sel = "#cube-problems .problems-list"
    lst = page.locator(sel)
    lst.wait_for(state="visible")
    assert wait_until(page, "document.querySelector(%r).style.maxHeight !== ''" % sel, 3), "the list placed"
    widest = page.evaluate("Math.min(document.body.clientWidth, 1100) - 32")
    b = lst.bounding_box()
    assert b["width"] <= widest + 1, f"up to the page's width: {b['width']} vs {widest}"
    assert lst.evaluate("(e) => e.scrollWidth <= e.clientWidth + 1"), "its entries inside it"
    assert b["x"] >= 0 and b["x"] + b["width"] <= case.width, f"inside the window: {b}"
    page.keyboard.press("Escape")


def test_cube_choice_list_shows_problems(page, case, reads):
    """Open, the Cube choice's list shows after each Cube's name its link's
    sign, first and always, then its problems as icons, its worst tone
    each, its link not again: here its settings file differing, the cog,
    for the owner (AP GlobalMenu, D190-A). The problems are in the page's reads of the Cubes
    (helpers.Reads)."""
    reads.cube("KEQ0000100", file_differs=["MASSIMO_CUBE_KEQ0000100_NAME"], connected=False)
    try:
        page.evaluate("loadCubes()")
        page.locator("#cube-choice").click()
        item = page.locator("#cube-menu .menu-items button[data-value='KEQ0000100']")
        item.wait_for(state="visible")
        icons = item.locator(".menu-item-icons > svg")
        hrefs = icons.evaluate_all("(es) => es.map((e) => [...e.querySelectorAll('use')].map((u) => u.getAttribute('href')).join(' '))")
        assert any("cog" in h for h in hrefs), f"the settings' cog after the name: {hrefs}"
        assert "#icon-net" in hrefs[0], f"its link's sign first after the name: {hrefs}"
        assert sum("#icon-net" in h for h in hrefs) == 1, f"its link once, its sign: {hrefs}"
        # Both boxes at once, once the list is placed: placeMenu
        # (components.js) moves it into the window on the frame after it
        # opens, and two reads apart could straddle the move.
        name, line = item.evaluate("""(e) => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(() => {
            const box = (el) => { const b = el.getBoundingClientRect(); return {x: b.x, y: b.y, width: b.width, height: b.height}; };
            done([box(e.querySelector('.menu-item-label')), box(e.querySelector('.menu-item-icons > svg'))]);
        })))""")
        assert line["x"] >= name["x"] + name["width"] and abs((line["y"] + line["height"] / 2) - (name["y"] + name["height"] / 2)) <= 6, \
            f"after the name, on its line: {name} {line}"
        page.keyboard.press("Escape")
    finally:
        reads.clear(CUBES)
        page.evaluate("loadCubes()")

