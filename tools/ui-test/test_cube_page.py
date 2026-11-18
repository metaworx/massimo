# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Cube settings, the page of ⚙ (AP PageMenu, Block 3): its title row
kept under the header (AP GlobalMenu, Block 1), then the panel of the
Cube chosen in the header, open, its head folding nothing; drawn there alone while the page is shown, and in System
settings' list again after; a change saved there shows in System
settings; another Cube chosen, the page follows; a role with pair alone
sees the pairing."""
import json

from conftest import api_call
from helpers import VIEWER, choose_cube, scroll_width, show_view, wait_until

PAGE_PANEL = "#cube-page .cube-panel"
SECOND = "KEQ0000200"


def test_cube_page_shows_the_chosen_cube(page, case):
    """The page's title row, kept under the header as the page scrolls;
    the chosen Cube's facts, settings and actions, its panel open; a
    click on its head folds nothing, and the head scrolls with the panel;
    System settings' list without it meanwhile, with it again once System
    settings is shown."""
    show_view(page, "cube")
    panel = page.locator(PAGE_PANEL)
    assert wait_until(page, "!!document.querySelector('#cube-page .cube-panel')", 5), "the Cube's panel on the page"
    assert panel.count() == 1 and panel.get_attribute("data-serial") == page.evaluate("state.serial"), "the chosen Cube's, alone"
    assert panel.get_attribute("open") is not None, "open"
    for part in (".cube-info", ".cube-form", ".cube-actions"):
        assert panel.locator(part).is_visible(), f"{part} shown"
    assert page.locator("#cubes-list .cube-panel").count() == 0, "not drawn in System settings meanwhile"
    panel.locator(":scope > summary .cube-head-who").click()
    assert panel.get_attribute("open") is not None, "a click on its head folds nothing"
    assert panel.locator(":scope > summary").get_attribute("tabindex") == "-1", "its head out of the tab order"
    bar = page.locator("#cube-page .page-bar")
    assert bar.is_visible() and bar.locator(".page-link.current").get_attribute("data-view") == "cube", "the page's row, itself current"
    assert bar.locator("h2").text_content() == page.evaluate("t('nav.cubeTitle')"), "named Cube settings"
    assert bar.evaluate("(e) => getComputedStyle(e).position") == "sticky", "kept under the header"
    head = panel.locator(":scope > summary")
    assert head.evaluate("(e) => getComputedStyle(e).position") == "static", "the panel's head scrolls with it"
    assert head.bounding_box()["y"] >= bar.bounding_box()["y"] + bar.bounding_box()["height"] - 1, "under the row"
    assert scroll_width(page) <= case.width, "no horizontal scroll on Cube settings"
    show_view(page, "system")
    assert wait_until(page, "!!document.querySelector(\"#cubes-list .cube-panel[data-serial='KEQ0000100']\")", 5), \
        "System settings lists it again"
    assert page.locator(PAGE_PANEL).count() == 0, "and the page no longer holds it"


def test_cube_page_sections_menu(page, case):
    """Cube settings' sections menu, at its row's right end: the Cube and
    Backup and restore, by name as System settings' are; Backup and
    restore brought into view under the row, the Cube back under it."""
    show_view(page, "cube")
    assert wait_until(page, "!!document.querySelector('#cube-page .cube-panel')", 5), "the Cube's panel on the page"
    page.locator("#cube-page .sec-menu > summary").click()
    items = page.locator("#cube-page .sec-menu .toc-link")
    names = [items.nth(i).inner_text().strip() for i in range(items.count())]
    assert names == [page.evaluate("t('header.cube')"), page.evaluate("t('backup.title')")], f"the Cube and its backups: {names}"
    items.nth(1).click()
    bar = page.locator("#cube-page .page-bar")
    assert wait_until(page, """(() => { const r = document.getElementById('cube-backup').getBoundingClientRect();
      return r.top >= document.querySelector('#cube-page .page-bar').getBoundingClientRect().bottom - 1 && r.top < innerHeight; })()""", 3), \
        "Backup and restore in view, under the row"
    assert bar.bounding_box()["y"] >= 0, "the row still shown"
    page.locator("#cube-page .sec-menu > summary").click()
    items.nth(0).click()
    assert wait_until(page, """(() => { const r = document.getElementById('cube-page-body').getBoundingClientRect();
      const under = document.querySelector('#cube-page .page-bar').getBoundingClientRect().bottom;
      return r.top >= under - 1 && r.top <= under + 30; })()""", 3), "the Cube back under the row"


def test_cube_page_saves_as_system_settings_shows(page, case):
    """A name saved on Cube settings shows in System settings' panel; the
    settings file's name taken back there on the page."""
    show_view(page, "cube")
    panel = page.locator(PAGE_PANEL)
    panel.locator(".cube-form").wait_for(state="visible")
    form = panel.locator("form.cube-form")
    form.locator("input[name=name]").fill("Renamed")
    form.locator("button[type=submit]").click()
    assert wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0000100' && c.name === 'Renamed')", 5), "saved"
    assert wait_until(page, "document.querySelector('#cube-page .cube-panel .panel-name').textContent === 'Renamed'", 5), \
        "the page's head names it"
    # The outcome comes after the reload that draws the name: waited for.
    assert wait_until(page, "!document.getElementById('cube-page-msg').classList.contains('hidden')", 5), \
        "the outcome under the panel"
    show_view(page, "system")
    assert wait_until(page, "(() => { const n = document.querySelector(\"#cubes-list .cube-panel[data-serial='KEQ0000100'] .panel-name\");"
                      " return !!n && n.textContent === 'Renamed'; })()", 5), "System settings shows the new name"
    show_view(page, "cube")
    take = page.locator(PAGE_PANEL + " .take-file[data-field=name]")
    take.wait_for(state="visible")
    take.click()
    assert wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0000100' && c.name === 'Fake Cube')", 5), "the file's name back"


def test_cube_page_follows_the_choice(page, case):
    """Another Cube chosen in the header, the page shows its panel; the
    first chosen again, the page follows back. The second Cube never
    connects."""
    api_call("POST", "api/v1/cubes", {"serial": SECOND, "address": "127.0.0.1:9", "name": "Second"})
    try:
        page.evaluate("loadCubes()")
        show_view(page, "cube")
        assert wait_until(page, "!!document.querySelector(\"#cube-page .cube-panel[data-serial='KEQ0000100']\")", 5), "the first Cube"
        choose_cube(page, SECOND)
        assert wait_until(page, "!!document.querySelector(\"#cube-page .cube-panel[data-serial='%s']\")" % SECOND, 5), \
            "the page follows the choice"
        assert page.locator(PAGE_PANEL).count() == 1, "one panel"
        choose_cube(page, "KEQ0000100")
        assert wait_until(page, "!!document.querySelector(\"#cube-page .cube-panel[data-serial='KEQ0000100']\")", 5), "and back"
    finally:
        page.evaluate("selectCube('KEQ0000100')")
        api_call("DELETE", "api/v1/cubes/" + SECOND)
        page.evaluate("loadCubes()")


def test_cube_page_pairing_alone_with_pair(page, case):
    """A role with pair and without cubes (/me stubbed): ⚙ offers Cube
    settings, which shows the pairing and not the Cube's settings."""
    me = dict(VIEWER, capabilities=["read", "preferences", "pair"])
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(me)))
    try:
        page.evaluate("loadMe()")
        assert wait_until(page, "document.body.classList.contains('no-cubes') && !document.body.classList.contains('no-pair')", 5), \
            "pair without cubes"
        show_view(page, "cube")
        panel = page.locator(PAGE_PANEL)
        panel.wait_for(state="visible")
        assert panel.locator(".pair-open").is_visible(), "Pair a device"
        assert panel.locator(".cube-form").is_hidden(), "not the Cube's settings"
    finally:
        page.unroute("**/api/v1/me")
