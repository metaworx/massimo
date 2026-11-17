# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings → Cubes: a new name is saved and shown in the header, the
settings file's differing name is offered and taken back; hiding the
devices of a room makes its tile go, showing them again brings it back;
a Cube found on the LAN is added and removed again, and is then listed
as found, with its removed row to restore. Each test puts the server's store back as it was, since every
case uses the same server; nothing is sent to the fake Cube."""
import json

from helpers import assert_busy, busy_state, choose_cube, cube_item, hold, named_buttons, open_found, scroll_width, show_view, wait_held, wait_until

PANEL = ".cube-panel[data-serial='KEQ0000100']"


def open_panel(page):
    """System settings → Cubes with the fake Cube's panel open; its form."""
    show_view(page, "system")
    panel = page.locator(PANEL)
    panel.locator(":scope > summary").click()
    return panel, panel.locator("form.cube-form")


def test_name_saved_and_taken_back(page, case):
    show_view(page, "system")
    panel = page.locator(PANEL)
    assert panel.count() == 1, "the Cube in System settings → Cubes"
    panel.locator(":scope > summary").click()
    form = panel.locator("form.cube-form")
    assert form.is_visible(), "the Cube's panel opens"
    named_buttons(page, "a Cube's panel")
    form.locator("input[name=name]").fill("Renamed")
    form.locator("button[type=submit]").click()
    assert wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0000100' && c.name === 'Renamed')", 5), "the new name is saved"
    assert "Renamed" in cube_item(page, "KEQ0000100").text_content(), "the Cube choice shows the new name"
    # The panel is drawn again after the state changed: waited for.
    take = panel.locator(".take-file[data-field=name]")
    assert wait_until(page, f"!!document.querySelector(\"{PANEL} .take-file[data-field=name]\")", 5) and \
        "MASSIMO_CUBE_KEQ0000100_NAME" in panel.locator(".cube-differs").inner_text(), "the settings file's name is offered"
    take.click()
    assert wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0000100' && c.name === 'Fake Cube')", 5), "the file's value taken"
    assert wait_until(page, f"!document.querySelector(\"{PANEL} .cube-differs\")", 5), "no difference left"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the Cube's panel"


def test_hidden_devices(page, case):
    """The last room's devices hidden take its tile away, shown again
    bring it back."""
    panel, form = open_panel(page)
    room = page.evaluate("(() => { const r = state.rooms[state.rooms.length - 1];"
                         " return { name: r.name, rfs: r.devices.map((d) => d.rf_address) }; })()")
    for rf in room["rfs"]:
        form.locator("input[name=hide]").fill(rf)
        form.locator(".cube-hide-add button").click()
    assert form.locator(".cube-hidden-list li").count() == len(room["rfs"]), "devices listed to hide"
    form.locator("button[type=submit]").click()
    gone = "!state.rooms.some((r) => r.name === %s)" % json.dumps(room["name"])
    assert wait_until(page, gone, 5), "hiding the devices removes their room"
    show_view(page, "tiles")
    assert page.evaluate("![...document.querySelectorAll('.room .room-name')].some((e) => e.textContent.trim() === %s)" % json.dumps(room["name"])), \
        "the room's tile is gone"
    show_view(page, "system")
    for _ in room["rfs"]:
        form.locator(".cube-hidden-list button").first.click()
    form.locator("button[type=submit]").click()
    assert wait_until(page, "!(" + gone + ")", 5), "showing them again brings the room back"
    assert wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0000100' && c.hidden.length === 0)", 5), "nothing hidden any more"
    assert scroll_width(page) <= case.width, "no horizontal scroll in System settings → Cubes"
    show_view(page, "tiles")


def test_found_cube_added_and_removed(page):
    """The Cube the fake Cube announces, its DHCP flag shown, is added and
    served, and Remove,
    asking in the panel, lists it as found again, with a Restore for its
    removed row. It is restored where an earlier case removed it, else
    added as new, so that the server's store keeps one removed row of it.
    The server keeps what a search found; where nothing was searched yet,
    this test searches."""
    if not page.evaluate("state.found.some((c) => c.serial === 'KEQ0000000')"):
        choose_cube(page, "__discover")
        wait_until(page, "!document.querySelector(\"#cube-menu button[data-value='__discover']\").disabled", 10)
    show_view(page, "system")
    open_found(page)
    found = page.locator(".cube-found-row[data-serial='KEQ0000000']")
    assert found.count() == 1, "the found Cube in System settings → Cubes"
    assert " · DHCP · " in found.locator(".found-facts").inner_text(), "the found Cube's DHCP flag in System settings → Cubes"
    restore = found.locator(".restore-cube")
    (restore.first if restore.count() else found.locator(".add-cube")).click()
    assert wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0000000')", 5), "Add or Restore serves the found Cube"
    assert page.evaluate("state.serial") == "KEQ0000100", "the selected Cube stays"
    other = page.locator(".cube-panel[data-serial='KEQ0000000']")
    other.locator(":scope > summary").click()
    other.locator(".remove-cube").click()
    confirm = other.locator(".confirm-box")
    assert confirm.is_visible() and other.locator(".remove-cube").count() == 0, "Remove asks in the panel"
    confirm.locator(".confirm-no").click()
    assert other.locator(".confirm-box").count() == 0 and other.locator(".remove-cube").is_visible(), "Keep takes the question back"
    other.locator(".remove-cube").click()
    other.locator(".confirm-box .confirm-yes").click()
    assert wait_until(page, "!state.cubes.some((c) => c.serial === 'KEQ0000000') && state.found.some((c) => c.serial === 'KEQ0000000')", 5), \
        "Remove lists it as found again"
    assert wait_until(page, "state.found.some((c) => c.serial === 'KEQ0000000' && (c.removed || []).length === 1)", 5), "with its removed row"
    open_found(page)
    assert found.locator(".restore-cube").count() == 1 and found.locator(".add-cube").is_visible(), "Restore beside Add as new"
    show_view(page, "tiles")


def test_redraw_of_an_open_panel_loads_nothing(page, case):
    """A redraw of System settings → Cubes, as after a save, keeps the open
    panel and asks for nothing again, so that no answer arriving later
    draws the pairing again under the user's typing; the test holds such
    a request until it has typed."""
    panel, form = open_panel(page)
    assert wait_until(page, "!!document.querySelector(%r)" % (PANEL + " .pair-open"), 5), "the pairing is drawn"
    page.wait_for_timeout(500)  # the panel's own loads are in
    held = []
    page.route("**/api/v1/cubes/KEQ0000100/pairing", lambda route: held.append(route))
    try:
        page.evaluate("renderCubes()")
        panel.locator(".pair-open").click()
        seconds = "pair-seconds-KEQ0000100"
        assert wait_until(page, "document.activeElement && document.activeElement.id === %r" % seconds, 5), "the time has the focus"
        page.keyboard.press("End")
        page.keyboard.type("5")
        asked = len(held)
        for route in held:
            route.continue_()
        page.wait_for_timeout(500)
        assert page.evaluate("document.activeElement && document.activeElement.id") == seconds, "the focus stays in the time"
        page.keyboard.type("0")
        assert page.locator("#" + seconds).input_value() == "6050", "every key typed is in the time"
        assert asked == 0, f"a redraw asks for the pairing again: {asked} request(s)"
    finally:
        page.unroute("**/api/v1/cubes/KEQ0000100/pairing")
    show_view(page, "tiles")


def test_hide_and_show_keep_what_is_typed(page, case):
    """Hide and Show again change the list of hidden devices in place:
    the name and the proxy address typed in the same form stay. Nothing
    is saved."""
    panel, form = open_panel(page)
    rf = page.evaluate("state.rooms[0].devices[0].rf_address")
    form.locator("input[name=name]").fill("Typed, not saved")
    form.locator("input[name=proxy]").fill("127.0.0.1:62999")
    form.locator("input[name=hide]").fill(rf)
    form.locator(".cube-hide-add button").click()
    assert form.locator(".cube-hidden-list li").count() == 1 and rf in form.locator(".cube-hidden-list").inner_text(), "the device listed to hide"
    assert form.locator("input[name=name]").input_value() == "Typed, not saved", "the name typed stays after Hide"
    assert form.locator("input[name=proxy]").input_value() == "127.0.0.1:62999", "the proxy typed stays after Hide"
    form.locator(".cube-hidden-list button").first.click()
    assert wait_until(page, "!document.querySelector(%r)" % (PANEL + " .cube-hidden-list button"), 5), "Show again takes it off the list"
    assert form.locator("input[name=name]").input_value() == "Typed, not saved", "the name typed stays after Show again"
    assert form.locator("input[name=proxy]").input_value() == "127.0.0.1:62999", "the proxy typed stays after Show again"
    assert page.evaluate("state.cubes.find((c) => c.serial === 'KEQ0000100').name") == "Fake Cube", "nothing saved"
    show_view(page, "tiles")


def test_save_is_off_until_a_change_and_turns(page, case):
    """A Cube's Save is off while nothing differs from the Cube's
    settings, and also once a hidden device is added and shown again;
    held, it turns and a second click sends nothing. The PUT is answered
    with a stub, so the store keeps its name."""
    panel, form = open_panel(page)
    save = form.locator("button[type=submit]")
    assert save.is_disabled(), "Save is off while nothing differs"
    form.locator("input[name=hide]").fill("0abcde")
    form.locator(".cube-hide-add button").click()
    assert save.is_enabled(), "a device to hide turns Save on"
    form.locator(".cube-hidden-list button").first.click()
    assert save.is_disabled(), "shown again, nothing differs"
    held, sent = [], []
    hold(page, "**/api/v1/cubes/KEQ0000100", "PUT", held, sent)
    try:
        form.locator("input[name=name]").fill("Held name")
        assert save.is_enabled(), "a changed name turns Save on"
        save.click()
        assert wait_held(page, held), "the PUT is sent"
        assert_busy(page, PANEL + " form.cube-form button[type=submit]", "Save")
        save.click(force=True)
        page.wait_for_timeout(300)
        assert sent == [{"name": "Held name"}], f"one PUT, with the name: {sent}"
        for route in held:
            route.fulfill(status=200, content_type="application/json", body="{}")
        assert wait_until(page, "(() => { const b = document.querySelector(%r); return b && !b.classList.contains('busy'); })()"
                          % (PANEL + " form.cube-form button[type=submit]"), 5), "Save stops turning once answered"
    finally:
        page.unroute("**/api/v1/cubes/KEQ0000100")
    assert busy_state(page, PANEL + " form.cube-form button[type=submit]")[0], "the Cube's name as it was: nothing differs"
    show_view(page, "tiles")


def test_cube_choice_menu(page, case):
    """The Cube choice is a menu of the page: closed the chosen Cube's
    network sign and name, its tooltip the serial, address and link; open
    a sign, the name and under it the serial and address per Cube, the chosen
    one marked and focused, the arrow keys moving between the entries;
    choosing the Cube shown closes it and keeps it."""
    summary = page.locator("#cube-choice")
    assert summary.locator(".conn-sign.tone-ok").count() == 1, "the chosen Cube connected, a green network sign"
    assert summary.locator(".conn-sign").get_attribute("aria-label"), "the sign named for a screen reader"
    title = summary.get_attribute("title") or ""
    assert "KEQ0000100" in title and "127.0.0.1" in title, f"the tooltip has the serial and the address ({title!r})"
    summary.click()
    item = cube_item(page, "KEQ0000100")
    assert item.get_attribute("aria-current") == "true", "the chosen Cube marked"
    assert wait_until(page, "document.activeElement === document.querySelector(\"#cube-menu button[data-value='KEQ0000100']\")", 3), \
        "the focus on the chosen Cube"
    assert "KEQ0000100" in item.locator(".menu-item-sub").inner_text(), "under the name, the serial and the address"
    page.keyboard.press("ArrowDown")
    assert page.evaluate("document.activeElement.closest('#cube-menu') !== null && document.activeElement.tagName === 'BUTTON'"), \
        "ArrowDown moves within the list"
    if case.width >= 600:
        widths = page.evaluate("[document.querySelector('#cube-menu .menu-items').offsetWidth, document.querySelector('#cube-choice').offsetWidth]")
        assert widths[0] >= widths[1], f"the list at least as wide as the closed choice ({widths})"
    item.click()
    assert page.locator("#cube-menu").get_attribute("open") is None, "a choice closes the menu"
    assert page.evaluate("state.serial") == "KEQ0000100", "the Cube stays chosen"


DUTY = {"en": "Duty cycle", "de": "Duty Cycle"}


def test_cube_facts(page, case):
    """A Cube's panel: its head names the host without the port and shows
    duty cycle, free slots and pending; open, a table of its facts, which a
    load of the list refreshes in place, what is typed in the form staying."""
    show_view(page, "system")
    cube = page.evaluate("state.cubes.find((c) => c.serial === 'KEQ0000100')")
    host, port = cube["address"].rsplit(":", 1)
    panel = page.locator(PANEL)
    head = panel.locator(":scope > summary")
    assert head.locator(".mono").inner_text() == "KEQ0000100 · " + host, "the serial and the host, without the port"
    stats = head.locator(".cube-head-stats").inner_text()
    assert stats.startswith(DUTY[case.lang] + " ") and stats.count(" · ") == 2, f"duty cycle, free slots and pending in the head ({stats!r})"
    head.click()
    info = panel.locator(".cube-info")
    assert info.is_visible(), "the facts, at the top of the open panel"

    def fact(name):
        return info.locator(f"[data-fact={name}]").inner_text().strip()

    assert fact("host") == host and fact("port") == port, "the address and the port"
    assert fact("firmware") == cube["firmware"] and fact("radio") == cube["rf_address"], "the firmware and the radio address"
    assert fact("clock") not in ("", "–"), "the Cube's clock"
    assert fact("duty").endswith(" %") and fact("slots") != "–" and fact("pending") != "–" and fact("queued") != "–", "the radio budget"
    form = panel.locator("form.cube-form")
    form.locator("input[name=name]").fill("Typing")
    page.evaluate("""() => {
      const c = state.cubes.find((x) => x.serial === 'KEQ0000100');
      c.budget = Object.assign({}, c.budget, { duty_cycle_pct: 42 });
      c.pending_radiograms = 3;
      cubesPolled();
    }""")
    assert fact("duty") == "42 %" and fact("pending") == "3", "a load of the list refreshes the open panel's facts"
    assert "42 %" in head.locator(".cube-head-stats").inner_text(), "and its head"
    assert form.locator("input[name=name]").input_value() == "Typing", "what is typed stays"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the Cube's facts"
    form.locator("input[name=name]").fill(cube.get("name") or "")
    head.click()
    show_view(page, "tiles")


FAILING = {"addr": "192.0.2.10:62910", "error": "listen tcp 192.0.2.10:62910: bind: cannot assign requested address",
           "since": "2026-11-13T08:00:00+01:00", "next_try": "2026-11-13T08:02:30+01:00", "tries": 9}
PROXY_BADGE = {"en": "1 proxy not listening", "de": "1 Proxy horcht nicht"}


def test_proxy_not_listening(page, case):
    """A proxy listener that does not listen (proxy_failing, stubbed):
    the Cubes card and the Cube's panel say so in a badge (the panel's
    top line), the panel
    under the proxy address with the error and since when; an open
    panel drops the notice once the listener listens."""
    def failing(route):
        try:
            cubes = route.fetch().json()
            for c in cubes:
                if c.get("serial") == "KEQ0000100":
                    c["proxy_failing"] = [FAILING]
            route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes))
        except Exception:  # Route is already handled, or the page closed
            pass
    page.route("**/api/v1/cubes", failing)
    try:
        show_view(page, "system")
        page.evaluate("loadCubes()")
        assert wait_until(page, "!!document.querySelector(\"%s .cube-top-line\") && "
                          "document.querySelector(\"%s .cube-top-line\").textContent.includes(%s)"
                          % (PANEL, PANEL, json.dumps(PROXY_BADGE[case.lang])), 5), "the panel's badge"
        assert PROXY_BADGE[case.lang] in page.locator("#sys-cubes .section-badges").inner_text(), "the card's badge"
        panel, form = open_panel(page)
        notice = form.locator(".cube-proxy-state .notice")
        assert notice.count() == 1, "the notice under the proxy address"
        text = notice.inner_text()
        assert "192.0.2.10:62910" in text and "cannot assign requested address" in text, "its address and its error"
        assert notice.locator(".notice-since").count() == 1 and notice.get_attribute("title"), "since when, the next try"
        assert scroll_width(page) <= case.width, "no horizontal scroll with a long error"
    finally:
        page.unroute("**/api/v1/cubes")
    page.evaluate("loadCubes()")
    assert wait_until(page, "!document.querySelector(\"%s .cube-proxy-state .notice\")" % PANEL, 5), \
        "the notice gone once the listener listens, the panel open"


ELSEWHERE = {"address": "192.0.2.24:62910", "configured": "192.0.2.23:62910", "since": "2026-11-14T08:00:00+01:00"}
ELSEWHERE_BADGE = {"en": "found elsewhere", "de": "anderswo gefunden"}
ELSEWHERE_COUNT = {"en": "1 found elsewhere", "de": "1 anderswo gefunden"}
ELSEWHERE_ITEM = {"en": "found at 192.0.2.24:62910", "de": "gefunden auf 192.0.2.24:62910"}


def test_cube_found_elsewhere(page, case):
    """A Cube with a fixed address that discovery finds at another one
    (found_elsewhere, stubbed): a badge on the panel's top line and in
    the Cubes card, an item in the Cube's pill, a notice above its facts
    with since when, and two buttons that PUT the found address and
    follow_discovery (the PUT stubbed, nothing changes); the notice goes
    once the Cube answers at its address, the panel open."""
    # a request still under way when its route is removed (the PUTs'
    # reload of the Cubes) is let go, as in test_announce_setting.py
    def elsewhere(route):
        try:
            cubes = route.fetch().json()
            for c in cubes:
                if c.get("serial") == "KEQ0000100":
                    c["found_elsewhere"] = ELSEWHERE
            route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes))
        except Exception:  # Route is already handled, or the page closed
            pass
    puts = []
    def put(route):
        if route.request.method != "PUT":
            route.fallback()
            return
        puts.append(route.request.post_data_json)
        try:
            route.fulfill(status=200, content_type="application/json", body=json.dumps(route.fetch(method="GET").json()))
        except Exception:  # as above
            pass
    page.route("**/api/v1/cubes", elsewhere)
    page.route("**/api/v1/cubes/KEQ0000100", put)
    try:
        show_view(page, "system")
        page.evaluate("loadCubes()")
        assert wait_until(page, "!!document.querySelector(\"%s .cube-top-line\") && "
                          "document.querySelector(\"%s .cube-top-line\").textContent.includes(%s)"
                          % (PANEL, PANEL, json.dumps(ELSEWHERE_BADGE[case.lang])), 5), "the panel's badge"
        assert ELSEWHERE_COUNT[case.lang] in page.locator("#sys-cubes .section-badges").inner_text(), "the card's badge"
        assert ELSEWHERE_ITEM[case.lang] in page.locator(PANEL + " .cube-pill").text_content(), "the item in the Cube's pill"
        panel, _ = open_panel(page)
        notice = panel.locator(".cube-address-state .notice")
        assert notice.count() == 1, "the notice above the facts"
        text = notice.inner_text()
        assert "192.0.2.24:62910" in text and "192.0.2.23:62910" in text, "the found and the configured address"
        assert notice.locator(".notice-since").count() == 1, "since when"
        named_buttons(page, "the found address's buttons")
        for name, n in ((".use-found", 1), (".follow-found", 2)):
            panel.locator(".cube-address-state " + name).click()
            for _ in range(50):
                if len(puts) >= n:
                    break
                page.wait_for_timeout(100)
            assert len(puts) >= n, "%s sends a PUT" % name
        assert {"address": "192.0.2.24:62910"} in puts and {"follow_discovery": True} in puts, "the two PUTs' bodies: %r" % puts
        assert scroll_width(page) <= case.width, "no horizontal scroll with the notice"
    finally:
        page.unroute("**/api/v1/cubes/KEQ0000100")
        page.unroute("**/api/v1/cubes")
    page.evaluate("loadCubes()")
    assert wait_until(page, "!document.querySelector(\"%s .cube-address-state .notice\")" % PANEL, 5), \
        "the notice gone once the Cube answers at its address, the panel open"


SLOTS_LOW = {
    "warn": {"level": "warn", "since": "2026-11-14T07:00:00+01:00", "free_slots": 4, "logged": "2026-11-14T08:00:00+01:00"},
    "error": {"level": "error", "since": "2026-11-14T07:00:00+01:00", "full_since": "2026-11-14T08:00:00+01:00", "free_slots": 1,
              "logged": "2026-11-14T09:00:00+01:00"},
}
SLOTS_BADGE = {"warn": {"en": "few memory slots free", "de": "wenige Speicherplätze frei"},
               "error": {"en": "memory slots full", "de": "Speicherplätze voll"}}
SLOTS_TEXT = {"warn": {"en": "Few of the Cube's memory slots are free (4)", "de": "Nur wenige Speicherplätze des Cubes sind frei (4)"},
              "error": {"en": "memory slots stay full (1 free)", "de": "Speicherplätze des Cubes bleiben voll (1 frei)"}}
SLOTS_TONE = {"warn": "warn", "error": "bad"}
SLOTS_REBOOT = {"en": "Reboot the Cube, below", "de": "Ein Neustart des Cubes, unten"}


def test_cube_slots_low(page, case):
    """A Cube whose memory slots stayed held (slots_low, stubbed), with
    fewer than 5 free a warning, with fewer than 2 an error (D192-A): a
    badge on the panel's top line, amber and red, and a notice above its
    facts with the free slots and since when that points to the Reboot
    below; both go once the slots are free, the panel open."""
    level = {"now": "warn"}

    def held(route):
        try:
            cubes = route.fetch().json()
            for c in cubes:
                if c.get("serial") == "KEQ0000100":
                    c["slots_low"] = SLOTS_LOW[level["now"]]
            route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes))
        except Exception:  # Route is already handled, or the page closed
            pass
    page.route("**/api/v1/cubes", held)
    try:
        show_view(page, "system")
        for lv in ("warn", "error"):
            level["now"] = lv
            page.evaluate("loadCubes()")
            badge = SLOTS_BADGE[lv][case.lang]
            assert wait_until(page, "!!document.querySelector(\"%s .cube-top-line\") && "
                              "document.querySelector(\"%s .cube-top-line\").textContent.includes(%s)"
                              % (PANEL, PANEL, json.dumps(badge)), 5), f"the panel's badge at {lv}"
            flag = page.locator(PANEL + " .cube-top-line .flag", has_text=badge).first
            assert SLOTS_TONE[lv] in (flag.get_attribute("class") or ""), f"the badge's tone at {lv}"
            if lv == "warn":
                panel, _ = open_panel(page)
            assert wait_until(page, "(document.querySelector(\"%s .cube-slots-state .notice\") || {}).textContent?.includes(%s)"
                              % (PANEL, json.dumps(SLOTS_TEXT[lv][case.lang])), 5), f"the notice at {lv}"
            notice = panel.locator(".cube-slots-state .notice")
            assert notice.count() == 1, "the notice above the facts"
            assert SLOTS_REBOOT[case.lang] in notice.inner_text(), "the Reboot: %r" % notice.inner_text()
            assert notice.locator(".notice-since").count() == 1, "since when"
            assert panel.locator(".cube-reboot-btn").count() == 1, "the Reboot it points to"
            assert scroll_width(page) <= case.width, "no horizontal scroll with the notice"
    finally:
        page.unroute("**/api/v1/cubes")
    page.evaluate("loadCubes()")
    assert wait_until(page, "!document.querySelector(\"%s .cube-slots-state .notice\")" % PANEL, 5), \
        "the notice gone once the slots are free, the panel open"
    assert SLOTS_BADGE["error"][case.lang] not in page.locator(PANEL + " .cube-top-line").inner_text(), "the badge gone"


STALE = {"en": "it still holds the server's old connection", "de": "Er hält noch dessen alte Verbindung"}
STALE_BADGE = {"en": "old connection", "de": "alte Verbindung"}


def test_cube_link_stale(page, case):
    """A Cube that refuses the server while it holds the server's old
    connection (link stale, stubbed): the panel's badge says so, the open
    panel a notice above its facts with since when, and Rooms' notice
    the same first; the notice goes once the Cube is connected again."""
    def stale(route):
        cubes = route.fetch().json()
        for c in cubes:
            if c.get("serial") == "KEQ0000100":
                c["connected"] = False
                c["link"] = {"state": "stale", "since": "2026-11-13T21:41:24+01:00"}
        route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes))
    page.route("**/api/v1/cubes", stale)
    try:
        show_view(page, "system")
        page.evaluate("loadCubes()")
        assert wait_until(page, "!!document.querySelector(\"%s .cube-top-line\") && "
                          "document.querySelector(\"%s .cube-top-line\").textContent.includes(%s)"
                          % (PANEL, PANEL, json.dumps(STALE_BADGE[case.lang])), 5), "the panel's badge"
        panel, _ = open_panel(page)
        notice = panel.locator(".cube-link-state .notice")
        assert notice.count() == 1 and STALE[case.lang] in notice.inner_text(), "the notice above the facts"
        assert notice.locator(".notice-since").count() == 1, "since when"
        assert scroll_width(page) <= case.width, "no horizontal scroll with the notice"
        panel.locator(":scope > summary").click()
        show_view(page, "tiles")
        assert wait_until(page, "document.querySelector('#home-notice .home-notice-detail').textContent.includes(%s)"
                          % json.dumps(STALE[case.lang]), 5), "Rooms' notice says it"
    finally:
        page.unroute("**/api/v1/cubes")
    page.evaluate("loadCubes()")
    show_view(page, "system")
    assert wait_until(page, "!document.querySelector(\"%s .cube-link-state .notice\")" % PANEL, 5), \
        "the notice gone once the Cube is connected"
    show_view(page, "tiles")


OTHER_NETWORK = {"en": "lies outside the networks of this machine", "de": "liegt außerhalb der Netze dieses Rechners"}


def test_cube_in_another_network(page, case):
    """A served Cube not connected whose address lies outside this
    machine's networks (other_network, stubbed): its open panel says so
    above its facts, with the netconfig line of its serial, which copies;
    the notice goes once the Cube is connected."""
    def away(route):
        cubes = route.fetch().json()
        for c in cubes:
            if c.get("serial") == "KEQ0000100":
                c["connected"], c["other_network"] = False, True
        route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes))
    page.route("**/api/v1/cubes", away)
    try:
        show_view(page, "system")
        page.evaluate("loadCubes()")
        panel, _ = open_panel(page)
        note = panel.locator(".cube-link-state .other-network")
        note.wait_for(state="visible")
        assert OTHER_NETWORK[case.lang] in note.inner_text() and "192.168.0.222" in note.inner_text(), "why, and the fallback address"
        assert note.locator(".netconfig-line").inner_text().startswith("massimo netconfig -sn KEQ0000100 -broadcast"), "the line"
        assert note.locator("button.copy-icon").count() == 1, "which copies"
        assert scroll_width(page) <= case.width, "no horizontal scroll with the line"
        panel.locator(":scope > summary").click()
    finally:
        page.unroute("**/api/v1/cubes")
    page.evaluate("loadCubes()")
    assert wait_until(page, "!document.querySelector(\"%s .cube-link-state .other-network\")" % PANEL, 5), \
        "the notice gone once the Cube is connected"
    show_view(page, "tiles")


REBOOT_PENDING = {"en": "that 2 radiograms are not delivered yet", "de": "dass 2 Funktelegramme noch nicht zugestellt sind"}
REBOOTED = {"en": "Cube KEQ0000100 is rebooting", "de": "Cube KEQ0000100 startet neu"}


def test_cube_reboot(page, case):
    """Reboot the Cube (the request stubbed, so that the fake Cube is left
    alone): it asks in place of the button, naming the serial and the
    radiograms pending as the Cube last said; No puts the button back and
    sends nothing, Yes sends the reboot with the count it showed."""
    def pending(route):
        cubes = route.fetch().json()
        for c in cubes:
            if c.get("serial") == "KEQ0000100":
                c["pending_radiograms"] = 2
                c["budget"] = dict(c.get("budget") or {}, updated_at="2026-11-13T21:40:00+01:00")
        route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes))
    sent = []

    def reboot(route):
        sent.append(route.request.post_data_json)
        route.fulfill(status=202, content_type="application/json", body='{"sent": true, "answered": true, "status": 2}')
    page.route("**/api/v1/cubes", pending)
    page.route("**/api/v1/cubes/KEQ0000100/reboot", reboot)
    try:
        show_view(page, "system")
        page.evaluate("loadCubes()")
        panel, _ = open_panel(page)
        btn = panel.locator(".cube-reboot-btn")
        assert btn.is_visible(), "the Reboot at the bottom of the panel"
        btn.click()
        box = panel.locator(".confirm-box")
        text = box.locator(".confirm-text").inner_text()
        assert "KEQ0000100" in text and REBOOT_PENDING[case.lang] in text, f"the question names the serial and the pending radiograms ({text!r})"
        assert scroll_width(page) <= case.width, "no horizontal scroll with the question"
        box.locator(".confirm-no").click()
        assert panel.locator(".cube-reboot-btn").is_visible() and not sent, "No puts the button back and sends nothing"
        panel.locator(".cube-reboot-btn").click()
        panel.locator(".confirm-box .confirm-yes").click()
        assert wait_until(page, "document.getElementById('cubes-msg').textContent.includes(%s)" % json.dumps(REBOOTED[case.lang]), 5), \
            "the message says it reboots"
        assert sent == [{"pending": 2}], f"the reboot with the count shown ({sent!r})"
    finally:
        page.unroute("**/api/v1/cubes")
        page.unroute("**/api/v1/cubes/KEQ0000100/reboot")
    page.evaluate("loadCubes()")
    show_view(page, "tiles")


def test_cube_head_pill_top_right(page, case):
    """A Cube's head with a long name: its sign, name, serial and radio
    budget wrap at the left, its pill stays top-right on the first line,
    the green check where nothing is wrong, and the source badge takes a
    line of its own under them."""
    def long(route):
        cubes = route.fetch().json()
        for c in cubes:
            if c.get("serial") == "KEQ0000100":
                c["name"] = "A Cube with a name long enough to wrap its head on a phone"
                c["problems"] = []
        route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes))
    page.route("**/api/v1/cubes", long)
    try:
        show_view(page, "system")
        page.evaluate("loadCubes()")
        assert wait_until(page, "!!document.querySelector(\"%s > summary .cube-pill .ok-pill\")" % PANEL, 5), "the green check"
        b = page.evaluate("""(sel) => {
          const s = document.querySelector(sel + " > summary");
          const r = (q) => s.querySelector(q).getBoundingClientRect().toJSON();
          return { who: r(".cube-head-who"), pill: r(".cube-pill"), badges: r(".panel-badges"), name: r(".panel-name") };
        }""", PANEL)
        assert b["pill"]["left"] >= b["who"]["right"] - 1, f"the pill right of the Cube's name and facts: {b}"
        assert b["pill"]["top"] <= b["name"]["top"] + 6, f"on the first line: {b}"
        assert b["badges"]["top"] >= b["who"]["bottom"] - 1, f"the source badge a line of its own under them: {b}"
        assert scroll_width(page) <= case.width, "no horizontal scroll"
    finally:
        page.unroute("**/api/v1/cubes")
        page.evaluate("loadCubes()")
        show_view(page, "tiles")


def test_cube_panel_pill(page, case):
    """A closed Cube's head: its network sign before the name, one pill
    with every problem, the neutral source badge, no "connected" badge; a
    click on the pill opens its list and leaves the panel closed; the open
    panel shows the Cube's own badges as its top line."""
    def stale(route):
        cubes = route.fetch().json()
        for c in cubes:
            if c.get("serial") == "KEQ0000100":
                c["connected"] = False
                c["link"] = {"state": "stale", "since": "2026-11-13T21:41:24+01:00"}
                c["problems"] = [{"rf_address": "0a0101", "type": "radiator_thermostat", "name": "Radiator", "room": "Kitchen",
                                  "serial": "MEQ0000101", "reported": True, "state": {"error": False, "battery_low": True, "link_error": False}}]
        route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes))
    def clean(route):
        # nothing wrong: the fake's window, open since the server started,
        # counts as a problem once it has been open five minutes
        cubes = route.fetch().json()
        for c in cubes:
            c["problems"] = []
        route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes))
    show_view(page, "system")
    head = page.locator(PANEL).locator(":scope > summary")
    page.route("**/api/v1/cubes", clean)
    page.evaluate("loadCubes()")
    assert wait_until(page, "!document.querySelector(\"%s > summary .cube-pill .problems\")" % PANEL, 5), "nothing wrong: no problem pill"
    assert head.locator(".cube-pill .ok-pill").count() == 1, "the green check in its place"
    assert head.locator(".panel-badges .flag").count() == 1, "connected: the source badge alone, no connected badge"
    page.unroute("**/api/v1/cubes")
    page.route("**/api/v1/cubes", stale)
    try:
        page.evaluate("loadCubes()")
        assert wait_until(page, "!!document.querySelector(\"%s > summary .cube-pill .problems\")" % PANEL, 5), "the pill"
        uses = head.locator(".conn-sign use").evaluate_all("(us) => us.map((u) => u.getAttribute('href'))")
        assert uses == ["#icon-net-base", "#mark-net-x"], f"the network sign with its cross ({uses})"
        box = (head.locator(".conn-sign").bounding_box(), head.locator(".panel-name, .mono").first.bounding_box())
        assert box[0]["x"] < box[1]["x"], "the sign before the name"
        head.locator(".cube-pill .problems-sum").click()
        assert head.locator(".cube-pill details").get_attribute("open") is not None, "the pill's list open"
        assert page.locator(PANEL).get_attribute("open") is None, "the panel still closed"
        listed = head.locator(".cube-pill .problems-list").inner_text()
        assert "Kitchen › Radiator" in listed, f"the devices in the pill's list ({listed!r})"
        head.locator(".cube-pill .problems-list .problem-name").first.click()
        assert page.locator(PANEL).get_attribute("open") is None, "a click in the list leaves the panel closed"
        page.keyboard.press("Escape")
        head.locator(".cube-head-stats").click()
        top = page.locator(PANEL).locator(".cube-top-line")
        assert top.is_visible() and top.locator(".flag.bad").count() == 1, "the open panel's top line: the link, failed"
        assert scroll_width(page) <= case.width, "no horizontal scroll"
        head.locator(".cube-head-stats").click()
    finally:
        page.unroute("**/api/v1/cubes")
    page.evaluate("loadCubes()")


CHANGED = {"en": "Changed meanwhile on the server", "de": "Inzwischen auf dem Server geändert"}


def test_cube_drafts_across_polls(page, case):
    """An open Cube's form across the polls: a field not touched follows a
    name stored meanwhile by another client; a field being edited keeps
    the draft and says under it that the stored value changed, Take
    theirs taking it, Save then having nothing to send."""
    panel, form = open_panel(page)
    name = form.locator("input[name=name]")
    proxy_serial = form.locator("input[name=proxy_serial]")
    before = page.evaluate("state.cubes.find((c) => c.serial === 'KEQ0000100').name || ''")
    put = "(body) => api('PUT', 'api/v1/cubes/KEQ0000100', body)"
    try:
        page.evaluate(put, {"name": "Stored meanwhile"})
        page.evaluate("loadCubes()")
        assert wait_until(page, "document.querySelector(\"%s input[name=name]\").value === 'Stored meanwhile'" % PANEL, 5), \
            "an untouched field follows"
        assert form.locator(".field-meanwhile").count() == 0, "without a word"
        proxy_serial.fill("KEQ0000555")
        page.evaluate(put, {"proxy_serial": "KEQ0000777"})
        page.evaluate("loadCubes()")
        conflict = form.locator(".field-meanwhile")
        assert wait_until(page, "!!document.querySelector(\"%s .field-meanwhile\")" % PANEL, 5), "the change said"
        assert proxy_serial.input_value() == "KEQ0000555", "the draft kept"
        assert CHANGED[case.lang] in conflict.inner_text() and "KEQ0000777" in conflict.inner_text(), "with the stored value"
        conflict.locator(".take-theirs").click()
        assert proxy_serial.input_value() == "KEQ0000777" and form.locator(".field-meanwhile").count() == 0, "Take theirs takes it"
        assert form.locator("button[type=submit]").is_disabled(), "nothing left to save"
        assert scroll_width(page) <= case.width, "no horizontal scroll"
    finally:
        page.evaluate(put, {"name": before, "proxy_serial": ""})
        page.evaluate("loadCubes()")
        panel.locator(":scope > summary").click()


HELD_BADGE = {"en": "1 connection held", "de": "1 Verbindung gehalten"}
LOST_BADGE = {"en": "1 not connected", "de": "1 nicht verbunden"}


def test_card_badges_follow_the_poll(page, case):
    """The Cubes card's badges follow every poll, a Cube's panel open or
    not, one per link state: a held connection amber, an old connection
    (stale) red, nothing once the Cube is back (D92-A, D93-A)."""
    link = {"state": None}

    def stub(route):
        cubes = route.fetch().json()
        for c in cubes:
            if c.get("serial") == "KEQ0000100" and link["state"]:
                c["connected"] = False
                c["link"] = {"state": link["state"], "since": "2026-11-14T20:00:00+01:00", "until": "2026-11-14T20:45:00+01:00"}
        route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes))
    badges = page.locator("#sys-cubes .section-badges")
    panel, _ = open_panel(page)
    page.route("**/api/v1/cubes", stub)
    try:
        link["state"] = "held"
        page.evaluate("loadCubes()")
        assert wait_until(page, "document.querySelector('#sys-cubes .section-badges').textContent.includes(%s)" % json.dumps(HELD_BADGE[case.lang]), 5), \
            "held: the card says so, the panel open"
        held = badges.locator(".flag.warn", has_text=HELD_BADGE[case.lang])
        assert held.count() == 1 and held.locator("svg.mark-sign use").get_attribute("href") == "#mark-warn", "amber, a triangle"
        assert badges.locator(".flag.bad").count() == 0, "not red"
        link["state"] = "stale"
        page.evaluate("loadCubes()")
        assert wait_until(page, "document.querySelector('#sys-cubes .section-badges .flag.bad') !== null", 5), "stale: red"
        assert LOST_BADGE[case.lang] in badges.locator(".flag.bad").inner_text(), "not connected"
        link["state"] = None
        page.evaluate("loadCubes()")
        assert wait_until(page, "!document.querySelector('#sys-cubes .section-badges .flag.bad') && "
                          "!document.querySelector('#sys-cubes .section-badges').textContent.includes(%s)" % json.dumps(HELD_BADGE[case.lang]), 5), \
            "back: no badge of the link, the panel still open"
        assert panel.get_attribute("open") is not None, "the panel stayed open"
    finally:
        page.unroute("**/api/v1/cubes")
        page.evaluate("loadCubes()")
        panel.locator(":scope > summary").click()


def test_cube_facts_copy(page, case):
    """A Cube's facts: its serial, address and radio address each with a
    copy icon; the serial copies (D97-A, AP CopyFacts' component)."""
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
    panel, _ = open_panel(page)
    for fact in ("serial", "host", "radio"):
        assert panel.locator(f".cube-info [data-fact={fact}] .copy-icon").count() == 1, f"{fact} with its copy icon"
    page.evaluate("navigator.clipboard.writeText('')")
    panel.locator(".cube-info [data-fact=serial] .copy-icon").click()
    assert wait_until(page, "navigator.clipboard.readText().then((t) => t === 'KEQ0000100')", 3), "the serial copies"
    panel.locator(":scope > summary").click()
