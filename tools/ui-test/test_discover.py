# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The Cube choice's search for new Cubes. The fake Cube answers
discovery on a loopback port as a Cube that is not configured,
KEQ0000000; the server's own discovery socket is on a loopback port of
its own (serve -discover-listen, run.sh), so the search needs no UDP
23272."""
import time

from helpers import FOUND2, SEARCHING, choose_cube, cube_item, found_text, scroll_width, show_view, wait_until

GATEWAY = {"en": "gateway", "de": "Gateway"}
VIA_PROXY = {"en": "via proxy at {address}", "de": "über Proxy {address}"}


def test_search_held_then_answered(page, case):
    """The search shows "searching" while the request is held, then the
    found Cube in the choice and counted under the header, a link to
    System settings → Cubes, which lists it with its DHCP flag and the env
    lines to add it, and the log has its cube_found."""
    assert cube_item(page, "__discover").count() == 1, "search entry in the Cube choice"
    serial = page.evaluate("state.serial")
    held = []
    page.route("**/api/v1/cubes/discover", lambda route: held.append(route))
    choose_cube(page, "__discover")
    end = time.time() + 5
    while not held and time.time() < end:
        page.wait_for_timeout(100)
    assert page.locator("#cube-search").inner_text() == SEARCHING[case.lang], "searching shown"
    assert cube_item(page, "__discover").is_disabled(), "search entry disabled while searching"
    # Released before the route goes: a page without routes stops
    # intercepting, and Playwright would let the held request through.
    if held:
        held[0].continue_()  # on to the server, which asks the fake Cube
    page.unroute("**/api/v1/cubes/discover")
    assert wait_until(page, "!discovery.running", 10), "search done"
    assert page.locator("#cube-search").inner_text() == found_text(case.lang, 1), "search result shown"
    opt = cube_item(page, "found:KEQ0000000")
    assert opt.count() == 1 and not opt.is_disabled(), "found Cube in the choice, which opens System settings with cubes"
    assert page.evaluate("state.serial") == serial, "the selected Cube stays"
    n = page.evaluate("state.found.length")
    link = page.locator("#cube-found-link")
    assert link.is_visible() and link.inner_text() == found_text(case.lang, n), "the found Cubes counted under the header, a link"
    assert page.locator("#cube-found-text").is_hidden(), "the plain count only without cubes"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the found Cubes"
    link.click()
    assert wait_until(page, "currentView === 'system'", 3), "the link opens System settings"
    # under the page's row, once the cards above it have their data
    assert wait_until(page, """(() => {
        const r = document.getElementById('sys-cubes').getBoundingClientRect();
        const bar = document.querySelector('#system .page-bar').getBoundingClientRect().bottom;
        return r.top >= bar - 1 && r.top < bar + 40;
    })()""", 5), "with the Cubes card in view, under the page's row"
    page.wait_for_timeout(2500)
    assert page.evaluate("""(() => {
        const r = document.getElementById('sys-cubes').getBoundingClientRect();
        const bar = document.querySelector('#system .page-bar').getBoundingClientRect().bottom;
        return r.top >= bar - 1 && r.top < bar + 40;
    })()"""), "and still there once everything has come"
    row = page.locator(".cube-found-row[data-serial='KEQ0000000']")
    assert row.is_visible(), "the found Cube in System settings → Cubes"
    assert " · DHCP · " in row.locator(".found-facts").inner_text(), "the found Cube's DHCP flag"
    # The gateway it uses, from its answer to N, the fake's lease: its
    # own address's /24 with the gateway at .1. In the expert line,
    # hidden here: its text is there all the same.
    assert GATEWAY[case.lang] + " 127.0.0.1" in row.locator(".found-extra").text_content(), "the found Cube's gateway"
    assert "MASSIMO_CUBE_KEQ0000000_ADDRESS=127.0.0.1" in page.locator("#cubes-list .env-lines").inner_text(), \
        "env lines for the found Cube under the list"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the found Cubes in System settings"
    show_view(page, "tiles")
    choose_cube(page, "found:KEQ0000000")
    assert wait_until(page, "currentView === 'system'", 3), "the found Cube in the choice opens System settings too"
    # A Cube is announced once per server: where an earlier test on this
    # server found it, its cube_found is older than the log's last 20
    # events can be, so the log is asked for the kind.
    show_view(page, "log")
    page.locator("#log-kind").select_option("cube_found")
    assert wait_until(page, "[...document.querySelectorAll('.log-line.kind-cube_found')].some((l) => l.textContent.includes('KEQ0000000'))", 5), \
        "cube_found in the log"
    show_view(page, "tiles")


def test_cube_found_live(page, case):
    """A cube_found event adds another found Cube live, counted under the
    header, and System settings → Cubes lists it with the massimo proxy
    that answers for it; without cubes the count is text, and the choice's
    entry cannot be chosen."""
    before = page.evaluate("state.found.length")
    cube = dict(FOUND2, proxies=["192.0.2.7"])
    page.evaluate("(c) => state.events.dispatchEvent(new MessageEvent('cube_found', { data: JSON.stringify("
                  "{ kind: 'cube_found', cube: c, time: c.found_at }) }))", cube)
    assert cube_item(page, "found:LEQ0000506").count() == 1, "cube_found adds a Cube live"
    assert page.locator("#cube-found-link").inner_text() == found_text(case.lang, before + 1), "found Cubes counted"
    show_view(page, "system")
    facts = page.locator(".cube-found-row[data-serial='LEQ0000506'] .found-facts")
    assert facts.count() == 1 and VIA_PROXY[case.lang].format(address="192.0.2.7") in facts.inner_text(), "the proxy that answers for it"
    show_view(page, "tiles")
    # a role without cubes: the count alone, the choice's entry information
    page.evaluate("() => { document.body.classList.add('no-cubes'); const k = hasCap; window.hasCap = (c) => c !== 'cubes' && k(c); renderCubeSelect(); }")
    text = page.locator("#cube-found-text")
    assert text.is_visible() and text.inner_text() == found_text(case.lang, before + 1), "without cubes the count is text"
    assert page.locator("#cube-found-link").is_hidden(), "and no link"
    assert cube_item(page, "found:LEQ0000506").is_disabled(), "the choice's found entry cannot be chosen without cubes"


def test_refused_search(page):
    """A refused search (a stubbed 503) shows the server's message."""
    page.route("**/api/v1/cubes/discover", lambda route: route.fulfill(
        status=503, content_type="application/json", body='{"error": "no discovery socket"}'))
    choose_cube(page, "__discover")
    assert wait_until(page, "document.getElementById('cube-search').classList.contains('bad')", 5) \
        and "no discovery socket" in page.locator("#cube-search").inner_text(), "a refused search shows the server's message"
    page.unroute("**/api/v1/cubes/discover")
