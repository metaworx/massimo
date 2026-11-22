# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The Cube choice's search for new Cubes. The fake Cube answers
discovery on a loopback port as a Cube that is not configured,
KEQ0000000; the server's own discovery socket is on a loopback port of
its own (serve -discover-listen, run.sh), so the search needs no UDP
23272."""
import json

import pytest

from helpers import FOUND2, SEARCHING, choose_cube, cube_item, found_text, open_found, scroll_width, show_view, wait_held, wait_until

GATEWAY = {"en": "gateway", "de": "Gateway"}
VIA_PROXY = {"en": "via proxy at {address}", "de": "über Proxy {address}"}
GROUP = {"en": "Found on the network", "de": "Im Netzwerk gefunden"}
SERVES = {"en": "Massimo proxy, serves {serial}", "de": "Massimo-Proxy, bedient {serial}"}
SERVED_BY = {"en": "served by Massimo on {host}", "de": "von Massimo auf {host} bedient"}
OTHER_NETWORK = {"en": "192.168.0.222 lies outside the networks of this machine",
                 "de": "192.168.0.222 liegt außerhalb der Netze dieses Rechners"}
ADD_NEW = {"en": "Add as new", "de": "Neu hinzufügen"}
RESTORE = {"en": "Restore ", "de": "Wiederherstellen "}
NETCONFIG = "massimo netconfig -sn LEQ0000507 -broadcast -static <ip> -gateway <gateway> -netmask <netmask>"

# The pill's found group: the last entry of its list.
FOUND_GROUP = "#cube-problems .problems-list > .problem-entry:last-child"


def found_sign(page):
    """The found Cubes' sign in the header's pill, and the number on it."""
    sign = page.locator("#cube-problems .problems-sum .found-count")
    return sign, (sign.locator("b").inner_text() if sign.count() else "")


def open_pill(page):
    """Opens the header's problem pill, where it is closed."""
    pill = page.locator("#cube-problems details.problems")
    if not pill.evaluate("(d) => d.open"):
        pill.locator(":scope > summary").click()


def test_search_held_then_answered(page, case):
    """The search's entry in the Cube choice says "searching" while the
    request is held, then the outcome shows in the page's toast with Show
    and no line beside the Cube choice; the found Cube in the choice, and
    in the header's pill the found Cubes' sign with their number and their
    group last in its list, its link
    opening System settings → Cubes with the found Cubes' fold open, which
    lists it with its DHCP flag and, folded, the lines for the settings
    file, copied by one icon; and the log has its cube_found."""
    assert cube_item(page, "__discover").count() == 1, "search entry in the Cube choice"
    serial = page.evaluate("state.serial")
    held = []
    page.route("**/api/v1/cubes/discover", lambda route: held.append(route))
    choose_cube(page, "__discover")
    assert wait_held(page, held), "the search is asked for and held"
    assert SEARCHING[case.lang] in cube_item(page, "__discover").text_content(), "the entry says searching"
    assert page.locator("#toast").is_hidden(), "no toast yet"
    assert cube_item(page, "__discover").is_disabled(), "search entry disabled while searching"
    # Released before the route goes: a page without routes stops
    # intercepting, and Playwright would let the held request through.
    if held:
        held[0].continue_()  # on to the server, which asks the fake Cube
    page.unroute("**/api/v1/cubes/discover")
    assert wait_until(page, "!document.querySelector(\"#cube-menu button[data-value='__discover']\").disabled", 10), "search done"
    toast = page.locator("#toast")
    assert toast.is_visible() and page.locator("#toast-body").inner_text() == found_text(case.lang, 1), "the outcome in the toast"
    assert page.locator("#toast-action").inner_text() == page.evaluate("t('discover.show')"), "with Show"
    assert page.locator("#cube-search").count() == 0, "no line beside the Cube choice"
    page.locator("#toast-close").click()
    assert toast.is_hidden(), "× closes it"
    opt = cube_item(page, "found:KEQ0000000")
    assert opt.count() == 1 and not opt.is_disabled(), "found Cube in the choice, which opens System settings with cubes"
    assert page.evaluate("state.serial") == serial, "the selected Cube stays"
    n = page.evaluate("state.found.length")
    sign, number = found_sign(page)
    assert sign.is_visible() and number == str(n), "the found Cubes' sign in the header's pill, their number on it"
    assert page.locator("#cube-found").count() == 0, "no line under the header"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the found Cubes"
    open_pill(page)
    group = page.locator(FOUND_GROUP)
    assert group.locator(":scope > .problem-who .problem-name").inner_text() == GROUP[case.lang], "the found Cubes' group last"
    cube = group.locator(".problem-subs .problem-entry", has_text="KEQ0000000")
    assert cube.count() == 1 and "127.0.0.1" in cube.inner_text(), "the found Cube in it with its address"
    group.locator(".found-to-cubes").click()
    assert wait_until(page, "MassimoParts.pages.shown() === 'system'", 3), "the link opens System settings"
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
    assert row.is_visible(), "the found Cube in System settings → Cubes, the fold open"
    assert " · DHCP · " in row.locator(".found-facts").inner_text(), "the found Cube's DHCP flag"
    # The gateway it uses, from its answer to N, the fake's lease: its
    # own address's /24 with the gateway at .1. In the expert line,
    # hidden here: its text is there all the same.
    assert GATEWAY[case.lang] + " 127.0.0.1" in row.locator(".found-extra").text_content(), "the found Cube's gateway"
    lines = page.locator("#cubes-list .found-lines")
    assert not lines.evaluate("(d) => d.open"), "the lines for the settings file folded"
    lines.locator(":scope > summary .panel-name").click()
    assert "MASSIMO_CUBE_KEQ0000000_ADDRESS=127.0.0.1" in lines.locator(".env-lines").inner_text(), \
        "the lines for the found Cube, unfolded"
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
    page.evaluate("navigator.clipboard.writeText('')")
    lines.locator(":scope > summary .copy-icon").click()
    assert wait_until(page, "navigator.clipboard.readText().then((t) => t.includes('MASSIMO_CUBE_KEQ0000000_ADDRESS=127.0.0.1\\nMASSIMO_CUBE_KEQ0000000_NAME='))", 3), \
        "one icon copies the lines"
    assert lines.evaluate("(d) => d.open"), "and leaves the fold as it is"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the found Cubes in System settings"
    show_view(page, "tiles")
    choose_cube(page, "found:KEQ0000000")
    assert wait_until(page, "MassimoParts.pages.shown() === 'system'", 3), "the found Cube in the choice opens System settings too"
    # A Cube is announced once per server: where an earlier test on this
    # server found it, its cube_found is older than the log's last 20
    # events can be, so the log is asked for the kind.
    show_view(page, "log")
    page.locator("#log-kind").select_option("cube_found")
    assert wait_until(page, "[...document.querySelectorAll('.log-line.kind-cube_found')].some((l) => l.textContent.includes('KEQ0000000'))", 5), \
        "cube_found in the log"
    show_view(page, "tiles")


@pytest.fixture
def found_event(page, reads):
    """found_event(cube) tells the page of a found Cube, as the server's
    cube_found does. The page takes its found Cubes anew from every read
    of the Cubes (its poll, every 10 s by default, and the reads that
    events and actions start), so a Cube that only an event brought
    would be gone at the next one. A server that found a Cube lists it
    from then on, once per serial: the page's reads list it (reads.found),
    a Cube in the list before its event goes out."""

    def tell(cube):
        reads.found(cube)
        page.evaluate("(c) => state.events.dispatchEvent(new MessageEvent('cube_found', { data: JSON.stringify("
                      "{ kind: 'cube_found', cube: c, time: c.found_at }) }))", cube)
    return tell


def test_cube_found_live(page, case, found_event):
    """A cube_found event adds another found Cube live, counted on the
    pill's sign, and System settings → Cubes lists it with the massimo
    proxy that answers for it; without cubes the pill's group has no link,
    and the choice's entry cannot be chosen."""
    before = page.evaluate("state.found.length")
    found_event(dict(FOUND2, proxies=["192.0.2.7"]))
    assert cube_item(page, "found:LEQ0000506").count() == 1, "cube_found adds a Cube live"
    assert found_sign(page)[1] == str(before + 1), "found Cubes counted on the pill's sign"
    show_view(page, "system")
    open_found(page)
    facts = page.locator(".cube-found-row[data-serial='LEQ0000506'] .found-facts")
    assert facts.count() == 1 and VIA_PROXY[case.lang].format(address="192.0.2.7") in facts.inner_text(), "the proxy that answers for it"
    show_view(page, "tiles")
    # a role without cubes: the group without its link, the choice's entry information
    page.evaluate("() => { document.body.classList.add('no-cubes'); state.me = { ...state.me, mode: 'users', capabilities: CAPABILITIES.filter((c) => c !== 'cubes') }; }")
    assert found_sign(page)[1] == str(before + 1), "without cubes the sign all the same"
    open_pill(page)
    assert page.locator(FOUND_GROUP + " .found-to-cubes").count() == 0, "and no link"
    page.keyboard.press("Escape")
    assert cube_item(page, "found:LEQ0000506").is_disabled(), "the choice's found entry cannot be chosen without cubes"


def test_found_proxy(page, case, found_event):
    """A Massimo proxy found on the network, with the Cube it serves and
    the server it runs on, as the server describes it: in the pill's group
    and in System settings → Cubes, the proxy's sign, what it serves and
    the server's name a link to the address its announcement gives,
    without Add and without lines for the settings file; a url that is not
    http(s) is no link."""
    proxy = dict(FOUND2, serial="LEQ0000601", address="192.0.2.31", kind="proxy", serves="KEQ0000201",
                 host={"name": "box-a", "url": "https://box-a.example:8443"})
    odd = dict(FOUND2, serial="LEQ0000602", address="192.0.2.32", kind="proxy",
               host={"name": "box-b", "url": "javascript:alert(1)"})
    found_event(proxy)
    found_event(odd)
    open_pill(page)
    entry = page.locator(FOUND_GROUP + " .problem-subs .problem-entry", has_text="LEQ0000601")
    assert SERVES[case.lang].format(serial="KEQ0000201") in entry.inner_text(), "what the proxy serves, in the pill"
    link = entry.locator("a.found-host")
    assert link.inner_text() == "box-a" and link.get_attribute("href") == "https://box-a.example:8443", "its server a link"
    assert link.get_attribute("rel") == "noopener noreferrer" and link.get_attribute("target") == "_blank", "to a tab of its own"
    page.keyboard.press("Escape")
    show_view(page, "system")
    open_found(page)
    row = page.locator(".cube-found-row[data-serial='LEQ0000601']")
    assert row.get_attribute("data-kind") == "proxy" and row.locator(".add-cube").count() == 0, "a proxy has no Add"
    assert row.locator("use").first.get_attribute("href") == "#icon-proxy", "and the proxy's sign"
    facts = row.locator(".found-facts").inner_text()
    assert SERVES[case.lang].format(serial="KEQ0000201") in facts and SERVED_BY[case.lang].format(host="box-a") in facts, \
        "what it serves and the server serving it"
    assert row.locator("a.found-host").get_attribute("href") == "https://box-a.example:8443", "its server a link"
    other = page.locator(".cube-found-row[data-serial='LEQ0000602']")
    assert other.locator("a").count() == 0 and SERVED_BY[case.lang].format(host="box-b") in other.inner_text(), \
        "a url that is not http(s) is the name alone"
    assert "LEQ0000601" not in " ".join(page.locator("#cubes-list .env-lines").all_text_contents()), "no lines for a proxy"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the proxies"
    show_view(page, "tiles")


def test_found_cube_with_removed_rows(page, case, watch, found_event):
    """A found Cube whose serial has removed rows offers Add as new and a
    Restore with the date per removed row, newest first, the name in its
    label and its tooltip; Restore sends the row's uuid with the serial.
    The POST is answered here, refused, so the store stays as it was."""
    removed = [{"uuid": "00000000-0000-4000-8000-000000000002", "name": "Upstairs", "removed_at": "2026-11-16T09:00:00Z"},
               {"uuid": "00000000-0000-4000-8000-000000000001", "name": "", "removed_at": "2026-11-01T09:00:00Z"}]
    found_event(dict(FOUND2, serial="LEQ0000508", address="192.168.1.79", removed=removed))
    show_view(page, "system")
    open_found(page)
    row = page.locator(".cube-found-row[data-serial='LEQ0000508']")
    assert row.locator(".add-cube").inner_text() == ADD_NEW[case.lang], "Add as new beside removed rows"
    restores = row.locator(".restore-cube")
    assert restores.count() == 2, "a Restore per removed row"
    first = restores.nth(0)
    assert first.get_attribute("data-uuid") == removed[0]["uuid"], "newest first"
    assert first.inner_text().startswith(RESTORE[case.lang]) and "2026" in first.inner_text(), "Restore with the date"
    assert "Upstairs" in first.get_attribute("aria-label") and first.get_attribute("title") == "Upstairs", "the name in its label and tooltip"
    assert "Upstairs" not in (restores.nth(1).get_attribute("aria-label") or ""), "a row without a name"
    sent = []

    def refuse(route):
        if route.request.method != "POST":
            route.fallback()
            return
        sent.append(json.loads(route.request.post_data))
        route.fulfill(status=409, content_type="application/json", body='{"error":"test"}')
    watch.expected.append(("/api/v1/cubes", 409))
    page.route("**/api/v1/cubes", refuse)
    try:
        first.click()
        assert wait_until(page, "!document.querySelector(\".cube-found-row[data-serial='LEQ0000508'] .restore-cube\").disabled", 5), \
            "the Restores come back after the answer"
    finally:
        page.unroute("**/api/v1/cubes", refuse)
    assert sent == [{"serial": "LEQ0000508", "restore": removed[0]["uuid"]}], "Restore sends the row's uuid"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the Restores"
    show_view(page, "tiles")


def test_found_cube_in_another_network(page, case, found_event):
    """A found Cube outside this machine's networks (other_network), as a
    Cube without a DHCP lease answers from 192.168.0.222: its row in
    System settings → Cubes says so, under its facts, with the fallback
    address and the netconfig line with its serial, which copies; a Cube
    in the network has no such line."""
    found_event(dict(FOUND2, serial="LEQ0000507", address="192.168.0.222", other_network=True))
    show_view(page, "system")
    open_found(page)
    row = page.locator(".cube-found-row[data-serial='LEQ0000507']")
    note = row.locator(".other-network")
    assert note.count() == 1 and OTHER_NETWORK[case.lang] in note.inner_text(), "the row says it is in another network"
    assert row.locator(".netconfig-line").inner_text() == NETCONFIG, "with the netconfig line of its serial"
    assert note.locator("button.copy-icon").count() == 1, "which copies"
    assert note.evaluate("(p) => p.getBoundingClientRect().top") > row.locator(".found-serial").evaluate(
        "(s) => s.getBoundingClientRect().bottom") - 1, "on a line of its own"
    assert page.locator(".cube-found-row[data-serial='KEQ0000000'] .other-network").count() == 0, "none in the network"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the line"
    show_view(page, "tiles")


def test_search_toast_shows_the_found(page, case):
    """Show in the search's toast opens System settings → Cubes with the
    found Cubes' fold open, and closes the toast."""
    choose_cube(page, "__discover")
    page.locator("#toast-action").wait_for(state="visible", timeout=10000)
    page.locator("#toast-action").click()
    assert wait_until(page, "MassimoParts.pages.shown() === 'system'", 3), "Show opens System settings"
    assert page.locator("#toast").is_hidden(), "and closes the toast"
    assert page.locator(".cube-found-row[data-serial='KEQ0000000']").is_visible(), "the found Cube, the fold open"
    assert scroll_width(page) <= case.width, "no horizontal scroll"
    show_view(page, "tiles")


def test_refused_search(page):
    """A refused search (a stubbed 503) shows the server's message in the
    toast, its edge red, and no Show."""
    page.route("**/api/v1/cubes/discover", lambda route: route.fulfill(
        status=503, content_type="application/json", body='{"error": "no discovery socket"}'))
    choose_cube(page, "__discover")
    assert wait_until(page, "document.getElementById('toast').classList.contains('bad')", 5) \
        and "no discovery socket" in page.locator("#toast-body").inner_text(), "a refused search shows the server's message"
    assert page.locator("#toast-action").is_hidden(), "nothing to show"
    page.unroute("**/api/v1/cubes/discover")
