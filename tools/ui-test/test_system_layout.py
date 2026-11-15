# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings as section cards (components.js sectionCards,
pageBar, backToTop): their order, the sections menu and no row of
section buttons, folding kept in the browser, back to top, the columns
where there is room, and a Cube's panel in two columns."""
from helpers import named_buttons, scroll_settled, scroll_width, show_view, wait_until

CARDS = ["sys-info", "sys-software", "sys-maint", "sys-cubes", "sys-users", "sys-providers", "sys-integrations", "sys-roles", "sys-caps", "sys-backup"]


def card_ids(page):
    return page.evaluate("[...document.querySelectorAll('#system .section-card')].map((c) => c.id)")


def left(page, sel):
    return page.evaluate("(s) => Math.round(document.querySelector(s).getBoundingClientRect().left)", sel)


def open_sections(page):
    """System settings' sections menu, open."""
    menu = page.locator("#system .sec-menu")
    if menu.get_attribute("open") is None:
        menu.locator("summary").click()
    return menu


def test_cards_and_contents(page, case):
    """The cards in the owner's order, each with an item in the sections
    menu, and no row of section buttons; the cards flow in two columns at
    1100 px and stack at 390; nothing overflows."""
    show_view(page, "system")
    assert card_ids(page) == CARDS, "System Info first, Backup and restore last"
    assert page.locator("#system .toc").count() == 0, "no row of section buttons"
    toc = open_sections(page).locator(".toc-link")
    assert toc.count() == len(CARDS), "a menu item per card"
    assert [toc.nth(i).get_attribute("data-target") for i in range(toc.count())] == CARDS, "the items in the cards' order"
    named_buttons(page, "System settings' sections menu")
    page.keyboard.press("Escape")
    assert wait_until(page, "document.querySelectorAll('.role-panel').length >= 6", 5), "the roles are listed"
    lefts = {left(page, "#" + c) for c in ("sys-users", "sys-roles", "sys-caps")}
    if case.width >= 900:
        assert len(lefts) == 2, f"Users, Roles and Capabilities in two columns ({lefts})"
    else:
        assert len(lefts) == 1, f"one column on a phone ({lefts})"
    assert scroll_width(page) <= case.width, "no horizontal scroll in System settings"
    show_view(page, "tiles")


def test_fold_kept_and_opened_from_contents(page, case):
    """A folded card keeps its head and badges, stays folded after a
    reload, and its item in the sections menu opens it again."""
    show_view(page, "system")
    card = page.locator("#sys-users")
    fold = card.locator(".section-fold")
    assert wait_until(page, "document.querySelector('#sys-users .section-badges').children.length > 0", 5), "the users' card has its badge"
    fold.click()
    try:
        assert fold.get_attribute("aria-expanded") == "false" and page.locator("#sys-users-body").is_hidden(), "the card folds"
        assert card.locator(".section-badges .flag").is_visible(), "a folded card shows its badges"
        page.reload()
        page.locator("#system").wait_for(state="visible")
        assert page.locator("#sys-users-body").is_hidden(), "the card is still folded after a reload"
        open_sections(page).locator(".toc-link[data-target='sys-users']").click()
        assert page.locator("#sys-users-body").is_visible(), "its item in the menu opens it"
        assert wait_until(page, "document.activeElement === document.querySelector('#sys-users .section-fold')", 3), \
            "the focus on the card's heading"
    finally:
        page.evaluate("localStorage.removeItem('massimo.fold.sys-users')")
    show_view(page, "tiles")


def test_back_to_top(page, case):
    """Once the first card's head is out of sight, ↑ takes the page back
    to its top, the focus on the sections menu."""
    show_view(page, "system")
    top = page.locator("#system .to-top")
    assert top.is_hidden(), "no ↑ at the top of the page"
    page.locator("#sys-backup").scroll_into_view_if_needed()
    assert wait_until(page, "!document.querySelector('#system .to-top').classList.contains('hidden')", 3), "↑ once scrolled"
    top.click()
    assert wait_until(page, "window.scrollY === 0", 3), "↑ goes back to the top"
    assert page.evaluate("document.activeElement === document.querySelector('#system .sec-menu > summary')"), \
        "the focus on the sections menu"
    show_view(page, "tiles")


def test_cube_panel_columns(page, case):
    """A Cube's panel in three groups: its facts (Info), its settings and
    its actions; Info and the settings side by side at 1100 px, one after
    the other at 390; the actions in one row under both, Pair a device,
    Reboot the Cube and Remove, the settings form without Remove (D97-A)."""
    show_view(page, "system")
    panel = page.locator(".cube-panel[data-serial='KEQ0000100']")
    panel.locator(":scope > summary").click()
    panel.locator(".cube-form").wait_for(state="visible")
    info, form, acts = page.evaluate("""() => ['.cube-info', '.cube-form', '.cube-actions'].map((s) =>
      document.querySelector(".cube-panel[data-serial='KEQ0000100'] " + s).getBoundingClientRect().toJSON())""")
    if case.width >= 900:
        assert form["left"] > info["right"], f"the settings beside Info ({form}, {info})"
    else:
        assert form["top"] >= info["bottom"] - 1, f"the settings under Info ({form}, {info})"
    assert acts["top"] >= max(info["bottom"], form["bottom"]) - 1, f"the actions last ({acts})"
    row = panel.locator(".cube-actions")
    for sel in (".pair-open", ".cube-reboot-btn", ".remove-cube"):
        assert row.locator(sel).count() == 1, f"{sel} in the actions' row"
    # in one go: the page may still be scrolling to the opened panel
    tops = page.evaluate("""() => ['.pair-open', '.cube-reboot-btn', '.remove-cube'].map((s) =>
      document.querySelector(".cube-panel[data-serial='KEQ0000100'] .cube-actions " + s).getBoundingClientRect().top)""")
    if case.width >= 900:
        assert max(tops) - min(tops) <= 2, f"on one row ({tops})"
    assert panel.locator(".cube-form .remove-cube").count() == 0, "Remove no longer in the settings"
    panel.locator(":scope > summary").click()
    show_view(page, "tiles")

def test_header_and_page_bar_stay(page, case):
    """Scrolled, the header and System settings' row stay at the top, the
    header compacted (on a phone without the brand); the sections menu
    marks the card in view and brings the one chosen under the row."""
    show_view(page, "system")
    assert wait_until(page, "document.querySelectorAll('.role-panel').length >= 6", 5), "the roles are listed"
    tall = page.evaluate("document.documentElement.scrollHeight")
    page.evaluate("document.getElementById('sys-backup').scrollIntoView({block: 'start'})")
    assert wait_until(page, "document.body.classList.contains('scrolled')", 3), "the page counts as scrolled"
    assert abs(page.evaluate("document.documentElement.scrollHeight") - tall) <= 1, \
        "the page as long scrolled as not, the compacted header keeping its height as margin"
    geo = page.evaluate("""() => ({
        top: document.querySelector('.top').getBoundingClientRect().top,
        topBottom: document.querySelector('.top').getBoundingClientRect().bottom,
        bar: document.querySelector('#system .page-bar').getBoundingClientRect().top,
        brand: getComputedStyle(document.querySelector('.top-left .brand')).display,
        cube: document.querySelector('.top-line > .cube').getBoundingClientRect().top,
        nav: document.querySelector('.top-line > .nav').getBoundingClientRect().top,
        time: getComputedStyle(document.getElementById('refresh-btn')).display,
    })""")
    assert abs(geo["top"]) <= 1, f"the header at the top ({geo})"
    assert abs(geo["bar"] - geo["topBottom"]) <= 2, f"the page's row under it ({geo})"
    if case.width < 600:
        assert geo["brand"] == "none" and abs(geo["cube"] - geo["nav"]) <= 2, f"on a phone the Cube choice takes the brand's line ({geo})"
    assert geo["time"] != "none", f"the time of the last update stays ({geo})"
    menu = page.locator("#system .sec-menu")
    menu.locator("summary").click()
    current = menu.locator(".toc-link[aria-current='true']")
    assert current.count() == 1, "the menu marks the card in view"
    menu.locator(".toc-link[data-target='sys-users']").click()
    assert menu.get_attribute("open") is None, "a choice closes the menu"
    assert wait_until(page, """(() => {
        const bar = document.querySelector('#system .page-bar').getBoundingClientRect().bottom;
        const y = document.getElementById('sys-users').getBoundingClientRect().top;
        return y >= bar - 1 && y <= bar + 40;
    })()""", 5), "the card chosen comes in under the row"
    # an instant scroll sent while the menu's smooth one ends can be undone
    # by it (scroll_settled)
    assert scroll_settled(page), "the menu's scroll comes to rest"
    page.evaluate("window.scrollTo(0, 0)")
    assert wait_until(page, "window.scrollY === 0 && !document.body.classList.contains('scrolled')", 3), \
        "at the top the header is whole again"
    show_view(page, "tiles")
