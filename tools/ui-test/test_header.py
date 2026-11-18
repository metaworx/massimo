# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The header: its order at full width and ☰ on a narrow screen, the log
button's symbol, its baseline, the personal page 👤 opens, the page
shown in the address, each page's own scroll position, and expert mode's
line of the chosen Cube's radio budget."""
import json

from helpers import VIEWER, css_hidden, hold, open_personal_page, scroll_settled, scroll_width, show_view, wait_held, wait_until

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
    header in view."""
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
    assert wait_until(page, "location.hash === ''", 5) and shown(page, ".room"), "back: Rooms, without a page in the address"
    page.go_forward()
    assert wait_until(page, "!document.getElementById('logview').classList.contains('hidden')", 5), "forward: the log"
    page.reload()
    page.locator("#logview").wait_for(state="visible", timeout=10000)
    assert page.evaluate("location.hash") == "#log" and page.evaluate("window.scrollY") == 0, "a reload opens the log again, the header in view"
    assert wait_until(page, "document.querySelectorAll('#logview .log-line').length > 0", 10), "with its lines"
    page.evaluate("history.replaceState(null, '', location.pathname + '#profile')")
    page.reload()
    page.locator("#me").wait_for(state="visible", timeout=10000)
    assert page.locator("#profile-display-name").is_visible(), "My profile's address opens the personal page, My profile loaded"
    assert page.evaluate("location.hash") == "#profile", "and stays in the address"
    page.evaluate("history.replaceState(null, '', location.pathname + '#notify')")
    page.reload()
    page.locator("#me").wait_for(state="visible", timeout=10000)
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


def test_header_order_and_the_narrow_header(page, case):
    """The header's row: the logo, the pile (the age above every Cube's
    problems, the server's sign first), the Cube choice with the network
    sign of its link, then 🏠 and ☰ alone at the right. Below 600 px the
    logo, the pile, 🏠 and ☰ on the first row and the Cube choice alone
    under them, as wide as the row."""
    order = ["h1", ".pile", "#cube-choice", "#view-tiles", "#nav-menu > summary"]
    b = {sel: box(page, sel) for sel in order}
    assert page.locator("#cube-choice .picker-icon .conn-sign use[href='#icon-net']").count() == 1, "the link, a network sign in the Cube choice"
    shown = page.locator(".nav > *").evaluate_all("(es) => es.filter((e) => e.offsetParent).map((e) => e.id)")
    assert shown == ["view-tiles", "nav-menu"], f"🏠 and ☰ alone in the header: {shown}"
    first = ["h1", ".pile", "#view-tiles", "#nav-menu > summary"]
    if case.width >= 600:
        first = order
    xs = [b[sel]["x"] for sel in first]
    assert xs == sorted(xs), f"left to right in this order: {first}: {b}"
    bottom = b["#view-tiles"]["y"] + b["#view-tiles"]["height"]
    assert all(abs(b[sel]["y"] + b[sel]["height"] - bottom) <= 1 for sel in first if sel != "h1"), f"on the first row's lower edge: {b}"
    if case.width >= 600:
        return
    c = b["#cube-choice"]
    assert c["y"] >= bottom, "the Cube choice under them"
    row = box(page, ".top-line")
    assert abs(c["x"] - row["x"]) <= 1 and abs(c["width"] - row["width"]) <= 1, f"as wide as the row: {c} vs {row}"


# The drawing shown in the header's brand, the logo or (under 600 px) the
# small mark: its box.
BRAND_BOX = """() => {
  const svg = [...document.querySelectorAll('.top .brand-name svg')].find((e) => e.getBoundingClientRect().width > 0);
  const b = svg.getBoundingClientRect();
  return {top: b.top, bottom: b.bottom, height: b.height};
}"""

VERSION_TITLE = {"en": "{v} (server version) · click to copy", "de": "{v} (Server-Version) · Klick kopiert"}


# A build's version, as long as one between releases.
BUILD = "v0.15.0-alpha.1-293-gd36148f"

def test_version_in_the_menu(page, case):
    """The server's version is ☰'s first row, before the log, small and
    grey, whole for a build's long one; its tooltip is the whole version.
    It is not in the header's row."""
    page.evaluate("""(v) => { window.__version = [state.version, document.getElementById("version-short").textContent];
      state.version = v; document.getElementById("version-short").textContent = v; renderVersionTitle(); }""", BUILD)
    try:
        page.locator("#nav-menu > summary").click()
        v = page.locator("#version-short")
        v.wait_for(state="visible")
        first = page.locator("#nav-menu .menu-items > *").first
        assert first.evaluate("(e) => e.contains(document.getElementById('version-short'))"), "☰'s first row"
        assert v.bounding_box()["y"] < page.locator("#menu-log").bounding_box()["y"], "above the log"
        assert v.evaluate("(e) => e.scrollWidth <= e.clientWidth + 1"), "whole"
        assert v.get_attribute("title") == VERSION_TITLE[case.lang].format(v=BUILD), "the tooltip is the whole version"
        assert page.locator("#version-out").is_hidden(), "none beside the logo while signed in"
        page.keyboard.press("Escape")
    finally:
        page.evaluate("""() => { const [v, short] = window.__version; state.version = v;
          document.getElementById("version-short").textContent = short; renderVersionTitle(); }""")


def test_pile_on_a_line_of_its_own(page, case):
    """Where the logo, the pile and the pages do not fit one row (here a
    window 230 px wide, compacted, ↑ among the pages), the pile takes a
    line of its own under them, the age at the pill's right; back on the
    first row when they fit again; all inside the window."""
    size = page.viewport_size
    try:
        page.set_viewport_size({"width": 230, "height": size["height"]})
        assert wait_until(page, "!document.querySelector('.pile').dataset.line", 3), "230 px whole: the pile fits beside the pages"
        page.evaluate("document.body.classList.add('scrolled')")
        assert wait_until(page, "document.querySelector('.pile').dataset.line === 'own'", 3), "compacted: a line of its own"
        pill, age, nav = box(page, "#cube-problems .problems-sum"), box(page, ".pile-age"), box(page, ".nav")
        assert pill["y"] >= nav["y"] + nav["height"] - 1, f"under the pages: {pill} vs {nav}"
        assert age["x"] >= pill["x"] + pill["width"], f"the age at the pill's right: {age} vs {pill}"
        assert abs((age["y"] + age["height"] / 2) - (pill["y"] + pill["height"] / 2)) <= 3, f"level with it: {age} vs {pill}"
        assert box(page, "#cube-choice")["y"] >= pill["y"] + pill["height"], "the Cube choice under it"
        assert page.evaluate("document.querySelector('header.top').scrollWidth") <= 230, "the header inside the window"
        page.evaluate("document.body.classList.remove('scrolled')")
        assert wait_until(page, "!document.querySelector('.pile').dataset.line", 3), "on the first row again"
    finally:
        page.evaluate("document.body.classList.remove('scrolled')")
        page.set_viewport_size(size)


def test_logo_by_width(page, case):
    """The logo stays, compacted too, down to 331 px (the M alone under
    600 px); under 331 px it goes, the pile first in the row."""
    size = page.viewport_size
    try:
        for w, shown in ((390, True), (331, True), (330, False), (300, False)):
            page.set_viewport_size({"width": w, "height": size["height"]})
            for scrolled in (False, True):
                page.evaluate(f"document.body.classList.toggle('scrolled', {str(scrolled).lower()})")
                page.wait_for_timeout(50)
                assert page.locator(".top-line > .brand").is_visible() == shown, f"at {w} px, compacted {scrolled}: the logo {shown}"
            if not shown:
                assert abs(box(page, ".pile")["x"] - box(page, ".top-line")["x"]) <= 1, f"at {w} px the pile first"
    finally:
        page.evaluate("document.body.classList.remove('scrolled')")
        page.set_viewport_size(size)


def test_header_on_one_baseline(page, case):
    """The row stands on one line: the logo's drawing (the small mark under
    600 px) with its bottom edge, the letters' baseline, the pill of the
    pile, the Cube choice beside it at full width and the page buttons end
    there; the M is as tall as the page buttons. The age stands centred
    above the pill, its digits of one width."""
    brand = page.evaluate(BRAND_BOX)
    button = box(page, "#view-tiles")
    assert abs(brand["height"] - button["height"]) <= 1, f"the M as tall as the page buttons: {brand} vs {button}"
    bottom = button["y"] + button["height"]
    ends = {"logo": brand["bottom"], "pill": edge(page, "#cube-problems .problems-sum")}
    if case.width >= 600:
        ends["cube"] = edge(page, "#cube-choice")
    assert all(abs(y - bottom) <= 1 for y in ends.values()), f"on the buttons' lower edge {bottom}: {ends}"
    pill, age = box(page, "#cube-problems .problems-sum"), box(page, ".pile-age")
    assert wait_until(page, "document.querySelector('.pile-age').textContent !== ''", 5), "the age has come"
    age = box(page, ".pile-age")
    assert age["y"] + age["height"] <= pill["y"] + 1, f"the age above the pill: {age} vs {pill}"
    assert abs((age["x"] + age["width"] / 2) - (pill["x"] + pill["width"] / 2)) <= 1, f"centred over it: {age} vs {pill}"
    assert "tabular-nums" in page.locator(".pile-age").evaluate("(e) => getComputedStyle(e).fontVariantNumeric"), "digits of one width"


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


def test_expert_line_of_the_cube(page, case):
    """In expert mode a line under the header's first: the chosen Cube's
    name, duty cycle, free slots and pending, queued only while commands
    wait; it stays when the header compacts, on every page, and is gone
    outside expert mode."""
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
    page.evaluate("document.body.classList.add('scrolled')")
    assert line.is_visible(), "the line stays when the header compacts"
    page.evaluate("document.body.classList.remove('scrolled')")
    show_view(page, "log")
    assert line.is_visible(), "on every page"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the line"
    show_view(page, "tiles")
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
    for part in (".top-line > .nav", ".top-line > .cube"):
        assert page.locator(part).is_hidden(), f"{part} hidden before the answer"
    assert page.locator(".top .brand-name").is_visible(), "the name shows"
    held[0].continue_()
    page.unroute("**/api/v1/me")
    assert wait_until(page, "!document.body.classList.contains('signed-out')", 5), "shown once the server has answered"
    assert page.locator(".top-line > .nav").is_visible() and page.locator(".top-line > .cube").is_visible(), "the buttons once signed in"
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


def scrolled_to(page, y):
    """Scrolls the window to y at once and waits until it rests there."""
    page.evaluate("(y) => window.scrollTo({top: y, behavior: 'instant'})", y)
    scroll_settled(page)
    return page.evaluate("window.scrollY")


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
        scroll_settled(page)
        assert page.evaluate("window.scrollY") == 0, "System settings at its top the first time"
        system = scrolled_to(page, 250)
        show_view(page, "tiles")
        scroll_settled(page)
        assert page.evaluate("window.scrollY") == house, "Rooms back where it was left"
        show_view(page, "system")
        scroll_settled(page)
        assert page.evaluate("window.scrollY") == system, "System settings back at its own place"
        page.go_back()
        page.locator("#rooms").wait_for(state="visible")
        scroll_settled(page)
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
SET_AGE = """async ([ago, skew]) => {
  serverSkew = skew;
  const c = state.cubes.find((x) => x.serial === state.serial);
  c.updated = new Date(Date.now() - ago * 1000).toISOString();
  renderUpdated();
  await new Promise((done) => setTimeout(done)); // the part draws the age after the call
  const time = fmtTimeOfDay(new Date(Date.parse(c.updated) - skew)); // on the browser's clock, as the age
  return [document.querySelector('.pile-age').textContent, t('header.cubeRead', {time})];
}"""


def test_updated_is_the_cubes_age(page, case):
    """Above the pile how long ago the chosen Cube was heard from (its
    updated), on the server's clock: seconds under a minute, then minutes,
    hours, days; it counts on by itself; the pile's Refresh now says the
    time itself. The server's clock is taken from its answers. The poll is
    held meanwhile, so that it does not bring the Cube's own time back."""
    assert wait_until(page, "Math.abs(serverSkew) < 3000", 5), "the server's clock from its answers: this machine's"
    page.evaluate("clearInterval(state.pollTimer)")
    try:
        for ago, n, unit in AGES:
            text, line = page.evaluate(SET_AGE, [ago, 0])
            assert text == page.evaluate(f"fmtUnit({n}, '{unit}')"), f"{ago} s ago: {text!r}"
            times = page.locator("#cube-problems .refresh-times").evaluate("(e) => e.textContent")
            assert times.split("\n")[0] == line, f"Refresh now says the time, with its date where not today's: {times!r} vs {line!r}"
        text, _ = page.evaluate(SET_AGE, [0, 60000])
        assert text == page.evaluate("fmtUnit(1, 'minute')"), f"on the server's clock, a minute ahead: {text!r}"
        page.evaluate(SET_AGE, [3, 0])
        assert wait_until(page, "document.querySelector('.pile-age').textContent === fmtUnit(5, 'second')", 4), "it counts on"
    finally:
        page.evaluate("startPolling(); poll(true)")


def test_refresh_now_keeps_the_list_open(page, case):
    """The pile's list starts with Refresh now, under it when the chosen
    Cube was read, when the server last answered and the time now, a line
    each, the time now moving on while the list is open. A click polls
    and leaves the list open, the server's answer then newer; closed, the
    times stand still."""
    page.locator("#cube-problems .problems-sum").click()
    first = page.locator("#cube-problems .problems-list > *").first
    first.wait_for(state="visible")
    assert "refresh-now" in (first.get_attribute("class") or ""), "Refresh now first"
    assert first.locator(".menu-item-label").inner_text() == page.evaluate("t('header.refreshTitle')"), "named so"
    times = page.locator("#cube-problems .refresh-times")
    lines = times.evaluate("(e) => e.textContent").split("\n")
    words = page.evaluate("['header.cubeRead', 'header.serverReplied', 'header.timeNow'].map((k) => t(k, {time: ''}).trim())")
    assert len(lines) == 3 and all(line.startswith(w) for line, w in zip(lines, words)), f"three times: {lines}"
    tall = times.evaluate("(e) => e.getBoundingClientRect().height / parseFloat(getComputedStyle(e).lineHeight || '16')")
    assert tall >= 2.5, f"a line each: {tall} lines tall"
    now = lines[2]
    assert wait_until(page, f"document.querySelector('#cube-problems .refresh-times').textContent.split('\\n')[2] !== {now!r}", 3), \
        "the time now moves on"
    page.evaluate("window.__replied = state.repliedAt")
    page.wait_for_timeout(1100)  # a second on, so that the answer's time shows newer
    first.click()
    assert wait_until(page, "state.repliedAt > window.__replied", 5), "a click polls"
    assert page.evaluate("document.querySelector('#cube-problems details').open"), "and leaves the list open"
    page.keyboard.press("Escape")
    assert wait_until(page, "!document.querySelector('#cube-problems details').open", 3), "Escape closes it"
    still = times.evaluate("(e) => e.textContent")
    page.wait_for_timeout(1500)
    assert times.evaluate("(e) => e.textContent") == still, "closed, the times stand still"


SET_PROBLEMS = """() => {
  const c = state.cubes.find((x) => x.serial === 'KEQ0000100');
  c.file_differs = ['MASSIMO_CUBE_KEQ0000100_NAME'];
  c.connected = false;
  renderCubeSelect();
}"""


def test_cube_choice_list_shows_problems(page, case):
    """Open, the Cube choice's list shows each Cube's problems as icons
    after its name, its worst tone each, its link left out (the Cube's own
    sign shows it): here its settings file differing, the cog, for the
    owner. The poll is held meanwhile."""
    page.evaluate("clearInterval(state.pollTimer)")
    try:
        page.evaluate(SET_PROBLEMS)
        page.locator("#cube-choice").click()
        item = page.locator("#cube-menu .menu-items button[data-value='KEQ0000100']")
        item.wait_for(state="visible")
        icons = item.locator(".menu-item-icons > svg")
        hrefs = icons.evaluate_all("(es) => es.map((e) => [...e.querySelectorAll('use')].map((u) => u.getAttribute('href')).join(' '))")
        assert any("cog" in h for h in hrefs), f"the settings' cog after the name: {hrefs}"
        assert not any("#icon-net" in h for h in hrefs), f"not its link, which its sign shows: {hrefs}"
        name, line = item.locator(".menu-item-label").bounding_box(), icons.first.bounding_box()
        assert line["x"] >= name["x"] + name["width"] and abs((line["y"] + line["height"] / 2) - (name["y"] + name["height"] / 2)) <= 6, \
            f"after the name, on its line: {name} {line}"
        page.keyboard.press("Escape")
    finally:
        page.evaluate("startPolling(); poll(true)")

