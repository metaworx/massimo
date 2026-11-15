# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The header: its order at full width and ☰ on a narrow screen, the log
button's symbol, its baseline, the personal page 👤 opens, the page
shown in the address, each page's own scroll position, and expert mode's
line of the chosen Cube's radio budget."""
import json

from helpers import VIEWER, css_hidden, hold, open_personal_page, scroll_settled, scroll_width, set_version, show_view, wait_held, wait_until

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


def active(page, selector):
    return page.locator(selector).evaluate("(e) => e.classList.contains('active')")


def test_header_order_and_the_narrow_header(page, case):
    """At full width the brand and the time of the last update with ↻
    beside it at the left (the version and the time under the brand, both,
    where they leave no room), then on the first line the Cube choice with
    the network sign of its link, then 🏠 and ☰ alone. Below 600 px 🏠 ☰
    beside the brand and the Cube choice on a line of its own under them."""
    order = ["h1", "#updated", "#refresh-btn", "#refresh-btn svg", "#cube-choice", "#view-tiles", "#nav-menu > summary"]
    b = {sel: box(page, sel) for sel in order}
    assert b["#updated"]["x"] < b["#refresh-btn svg"]["x"], "↻ after the time it renews, in the same button"
    assert page.locator("#cube-choice .picker-icon .conn-sign use[href='#icon-net']").count() == 1, "the link, a network sign in the Cube choice"
    assert b["#view-tiles"]["x"] < b["#nav-menu > summary"]["x"], "🏠 left of ☰"
    shown = page.locator(".nav > *").evaluate_all("(es) => es.filter((e) => e.offsetParent).map((e) => e.id)")
    assert shown == ["view-tiles", "nav-menu"], f"🏠 and ☰ alone in the header: {shown}"
    if case.width >= 600:
        right = ["#cube-choice", "#view-tiles", "#nav-menu > summary"]
        xs = [b[sel]["x"] for sel in right]
        assert xs == sorted(xs), f"left to right in this order: {right}"
        assert max(b[sel]["x"] + b[sel]["width"] for sel in ("h1", "#updated", "#refresh-btn")) < b["#cube-choice"]["x"], \
            f"the brand and the time left of the Cube choice: {b}"
        first_line = b["h1"]["y"] + b["h1"]["height"]
        assert all(b[sel]["y"] < first_line for sel in right), f"the Cube choice and the pages on the first line: {b}"
        return
    assert b["#cube-choice"]["y"] >= b["#view-tiles"]["y"] + b["#view-tiles"]["height"], "the Cube choice under them"


# The baseline of an element's text: the bottom of an empty inline-block
# put at its end, whose baseline is its bottom edge.
BASELINE = """(sel) => {
  const probe = document.createElement('span');
  probe.style.display = 'inline-block';
  document.querySelector(sel).appendChild(probe);
  const y = probe.getBoundingClientRect().bottom;
  probe.remove();
  return y;
}"""

# The drawing shown in the header's brand, the logo or (under 600 px) the
# small mark: its box.
BRAND_BOX = """() => {
  const svg = [...document.querySelectorAll('.top .brand-name svg')].find((e) => e.getBoundingClientRect().width > 0);
  const b = svg.getBoundingClientRect();
  return {top: b.top, bottom: b.bottom, height: b.height};
}"""

# The middle of an element's digits: its baseline less half the height
# of a 0 in its font.
DIGITS_MIDDLE = """(sel) => {
  const e = document.querySelector(sel);
  const probe = document.createElement('span');
  probe.style.display = 'inline-block';
  e.appendChild(probe);
  const base = probe.getBoundingClientRect().bottom;
  probe.remove();
  const ctx = document.createElement('canvas').getContext('2d');
  ctx.font = getComputedStyle(e).font;
  return base - ctx.measureText('0').actualBoundingBoxAscent / 2;
}"""

VERSION_TITLE = {"en": "{v} (server version) · click to copy", "de": "{v} (Server-Version) · Klick kopiert"}


# A build's version, as long as one between releases.
BUILD = "v0.15.0-alpha.1-293-gd36148f"

# The facts' state: the version cut or not, shown or not, the time shown,
# all inside the window, and whether they have a row of their own.
FACTS = """() => {
  const v = document.getElementById("version-short");
  const r = (e) => e.getBoundingClientRect();
  const right = Math.max(r(v).right, r(document.getElementById("refresh-btn")).right);
  return { cut: v.scrollWidth > v.clientWidth + 1, version: v.offsetParent !== null, time: document.getElementById("updated").offsetParent !== null,
    inside: right <= document.documentElement.clientWidth + 0.5, row: document.querySelector(".top-line").classList.contains("facts-row") };
}"""


def test_version_never_hidden(page, case):
    """A build's long version, at widths from a wide screen to a phone:
    beside the logo where it fits, else on a row of its own, never cut,
    inside the window; compacted, the version goes and the server's sign
    and the time stay."""
    size = page.viewport_size
    page.evaluate("""(v) => { window.__version = [state.version, document.getElementById("version-short").textContent];
      state.version = v; document.getElementById("version-short").textContent = v; renderVersionTitle(); }""", BUILD)
    try:
        rows = {}
        for w in (1280, 1100, 900, 750, 600, 520, 430, 390):
            page.set_viewport_size({"width": w, "height": size["height"]})
            page.wait_for_timeout(150)
            f = page.evaluate(FACTS)
            rows[w] = f["row"]
            assert f["version"] and not f["cut"] and f["time"] and f["inside"], f"at {w} px the version whole, the time, inside: {f}"
            page.evaluate("document.body.classList.add('scrolled')")
            f = page.evaluate(FACTS)
            assert not f["version"] and f["time"], f"at {w} px compacted: the time, not the version: {f}"
            page.evaluate("document.body.classList.remove('scrolled')")
            page.wait_for_timeout(50)
        assert rows[390], f"a row of their own on a phone: {rows}"
    finally:
        page.set_viewport_size(size)
        page.evaluate("""() => { const [v, short] = window.__version; state.version = v;
          document.getElementById("version-short").textContent = short; renderVersionTitle(); }""")
        page.wait_for_timeout(150)


def test_facts_fitted_again_when_the_time_comes(page, case):
    """The version, the sign and the time are fitted again when what they
    show changes: with a short version (a build's from a checkout without
    tags, as CI's) they fit beside the logo on a phone before the time of
    the last update has come, and the time then takes them to their row,
    the version whole, not cut under the logo. The Cube's age is held
    meanwhile (its timer stopped, the poll too), so that the test alone
    writes the time."""
    page.evaluate("""() => { clearInterval(state.pollTimer); clearTimeout(updatedTimer);
      const v = document.getElementById('version-short'), u = document.getElementById('updated');
      window.__facts = [v.textContent, u.textContent]; v.textContent = 'v0.0.0-dev'; u.textContent = ''; fitFacts(); }""")
    try:
        page.wait_for_timeout(100)
        page.evaluate("document.getElementById('updated').textContent = '59 sec'")
        assert wait_until(page, "(() => { const v = document.getElementById('version-short'); "
                          "return v.scrollWidth <= v.clientWidth + 1; })()", 3), "the version whole once the time came"
    finally:
        page.evaluate("""() => { const [v, u] = window.__facts; document.getElementById('version-short').textContent = v;
          document.getElementById('updated').textContent = u; renderUpdated(); startPolling(); }""")


def test_header_on_one_baseline(page, case):
    """The logo (the small mark under 600 px), the version and the time of
    the last update stand on one baseline, the drawing's bottom edge, and
    the M is as tall as the page buttons, where they fit beside it: a
    release's short version (CI's build has one too) always does, a long
    build's on a phone does not, and then the version and the time stand
    under the mark, the version whole, on the time's line or a line of its
    own above it. Each version is set here, so that the build's own does
    not decide which case is tested. ↻ stands on the digits' middle.
    The time and ↻ are one button without a frame, larger to a finger
    than it looks; the version's tooltip is the whole version."""
    brand = page.evaluate(BRAND_BOX)
    button = box(page, "#view-tiles")
    assert abs(brand["height"] - button["height"]) <= 1, f"the M as tall as the page buttons: {brand} vs {button}"
    assert abs(brand["top"] - button["y"]) <= 1, f"and level with them: {brand} vs {button}"
    # fitted once the version has come (header.js fitFacts)
    assert wait_until(page, "(() => { const v = document.getElementById('version-short'); return v.textContent !== '' && v.scrollWidth <= v.clientWidth + 1; })()", 5), \
        "the version whole"
    real = page.locator("#version-short").inner_text()
    try:
        for version, beside in (("v0.0.0-dev", True), ("v0.0.0-dev+0123456789ab-dirty", case.width >= 600)):
            set_version(page, version)
            assert wait_until(page, "(() => { const v = document.getElementById('version-short'); return v.scrollWidth <= v.clientWidth + 1; })()", 3), \
                f"{version} whole"
            lines = {sel: page.evaluate(BASELINE, sel) for sel in ("#version-short", "#updated")}
            if beside:
                assert abs(lines["#version-short"] - lines["#updated"]) <= 1, f"{version} and the time on one line: {lines}"
                assert abs(lines["#updated"] - brand["bottom"]) <= 1, f"on the logo's baseline with {version}: {lines} vs {brand['bottom']}"
            else:
                assert lines["#version-short"] <= lines["#updated"] + 1, f"{version} on the time's line or above it: {lines}"
                assert lines["#version-short"] > brand["bottom"] + 8, f"both under the mark with {version}: {lines} vs {brand['bottom']}"
    finally:
        set_version(page, real)
    icon = box(page, "#refresh-btn svg")
    digits = page.evaluate(DIGITS_MIDDLE, "#updated")
    assert abs(icon["y"] + icon["height"] / 2 - digits) <= 1, f"↻ on the digits' middle: {icon} vs {digits}"
    btn = page.locator("#refresh-btn")
    assert btn.evaluate("(e) => getComputedStyle(e).borderTopStyle") == "none", "no frame"
    b = box(page, "#refresh-btn")
    x = b["x"] + b["width"] / 2
    for y in (b["y"] - 8, b["y"] + b["height"] + 8):
        # the button, or another control where one is that close (the Cube
        # choice under it on a phone)
        hit = page.evaluate("([x, y]) => { const e = document.elementFromPoint(x, y); "
                            "return !!e && (!!e.closest('#refresh-btn') || !!e.closest('button, summary, a, input, select')); }", [x, y])
        assert hit, f"a tap 8 px off the button still hits it, where no other control is (y {y}, button {b})"
    label = btn.get_attribute("aria-label") or ""
    assert label == btn.get_attribute("title") and label != page.evaluate("t('header.refreshTitle')"), \
        f"its tooltip and its name say the time: {label!r}"
    version = page.evaluate("state.version")
    assert page.locator("#version-short").get_attribute("title") == VERSION_TITLE[case.lang].format(v=version), \
        "the tooltip is the whole version"


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
    """The header's version and System Info's copy the whole version, as
    the tooltip names it, without " (server version)"; System Info's by a
    click on it and by its icon (D94-A)."""
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
    whole = page.evaluate("state.version")
    page.evaluate("navigator.clipboard.writeText('')")
    page.locator("#version-short").click()
    assert wait_until(page, "navigator.clipboard.readText().then((t) => t === state.version)", 3), "the header's version copies whole"
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
SET_AGE = """([ago, skew]) => {
  serverSkew = skew;
  const c = state.cubes.find((x) => x.serial === state.serial);
  c.updated = new Date(Date.now() - ago * 1000).toISOString();
  renderUpdated();
  const d = new Date(Date.parse(c.updated)), pad = (n) => String(n).padStart(2, '0');
  const time = pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds());
  return [document.getElementById('updated').textContent, document.getElementById('refresh-btn').title,
          t('header.refreshUpdated', {time})];
}"""


def test_updated_is_the_cubes_age(page, case):
    """Beside ↻ how long ago the chosen Cube was heard from (its updated),
    on the server's clock: seconds under a minute, then minutes, hours,
    days; it counts on by itself; ↻'s tooltip says the time itself. The
    server's clock is taken from its answers. The poll is held meanwhile,
    so that it does not bring the Cube's own time back."""
    assert wait_until(page, "Math.abs(serverSkew) < 3000", 5), "the server's clock from its answers: this machine's"
    page.evaluate("clearInterval(state.pollTimer)")
    try:
        for ago, n, unit in AGES:
            text, title, want = page.evaluate(SET_AGE, [ago, 0])
            assert text == page.evaluate(f"fmtUnit({n}, '{unit}')"), f"{ago} s ago: {text!r}"
            assert title == want, f"the tooltip says the time: {title!r}"
        text, _, _ = page.evaluate(SET_AGE, [0, 60000])
        assert text == page.evaluate("fmtUnit(1, 'minute')"), f"on the server's clock, a minute ahead: {text!r}"
        page.evaluate(SET_AGE, [3, 0])
        assert wait_until(page, "document.getElementById('updated').textContent === fmtUnit(5, 'second')", 4), "it counts on"
        assert page.locator("#updated").evaluate("(e) => getComputedStyle(e).fontVariantNumeric").find("tabular-nums") >= 0, \
            "digits of one width"
    finally:
        page.evaluate("startPolling(); poll(true)")


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

