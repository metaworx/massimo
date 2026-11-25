# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The repairs (ui/src/shared/repair.tsx): a room's from its ⋯ menu, a
device's from its row, the Cube's from its panel. Their routes are
stubbed, a GET with a preview or a refusal, a POST with a run, so that
nothing reaches the fake Cube; the routes have their Go tests
(internal/server/repair_api_test.go)."""
import json
import time
from urllib.parse import parse_qs, urlparse

from helpers import REPAIR_BUDGET, RepairStub, focus_in_turn, named_buttons, scroll_width, show_view, wait_until

PANEL = ".cube-panel[data-serial='KEQ0000100']"

RELINK = {"action": "relink", "plan": "plan-relink", "radiograms": 9, "duty_cost_pct": 11.25, "slots": 12, "held_slots": 3,
          "resets": 0, "wakeups": 2, "metadata": False, "notes": [],
          "budget": REPAIR_BUDGET,
          "steps": [{"what": "link", "device": "0a0101", "room": 1, "lines": ["s:AAAA", "s:BBBB"]},
                    {"what": "link", "device": "0a0102", "room": 1, "lines": ["s:CCCC"]}]}
RESETS = {"action": "reset", "plan": "plan-reset", "radiograms": 0, "duty_cost_pct": 0, "slots": 0, "held_slots": 0, "resets": 2,
          "wakeups": 0, "metadata": False, "notes": [], "budget": RELINK["budget"],
          "steps": [{"what": "reset", "lines": ["r:01,AAAA", "r:01,BBBB"]}]}
UNLINK = dict(RELINK, action="unlink_relink", plan="plan-unlink", radiograms=25, duty_cost_pct=31.25, slots=37, held_slots=12,
              wakeups=6, untested=True, notes=["not yet run on hardware"])
LIST = dict(RESETS, action="list", plan="plan-list", resets=0, metadata=True, steps=[{"what": "list", "lines": ["m:00,AAAA"]}])
NONE = "no device of room 1 has a communication error"


# What a preview costs, in the page's language, as the box says it: its
# radiograms with their duty cycle, slots and wakeups, else a list write,
# else its resets.
COST_JS = """(p) => p.radiograms > 0
  ? t('repair.costRadio', {radiograms: p.radiograms, duty: fmtNum(p.duty_cost_pct, 1), slots: p.slots, held: p.held_slots, wakeups: p.wakeups})
  : p.metadata ? t('repair.costMetadata') : t('repair.costResets', {n: p.resets})"""


def cost(page, p):
    return page.evaluate(COST_JS, p)


def test_room_repairs(page, case):
    """The ⋯ menu's Repair… opens the room's repairs under the tile: each
    named and explained, a preview's cost, a refusal's reason. Run… asks
    with the cost, Cancel sends nothing, Run now sends the run with the
    preview's plan and says how it went."""
    stub = RepairStub(page, {"reset": NONE, "master": "room 1's master is a thermostat of the room already", "relink": RELINK})
    try:
        tile = page.locator(".room").first
        tile.locator(".tile-head .menu > summary").click()
        tile.locator(".menu-repair").click()
        box = tile.locator(".repair-box:not(.hidden)")
        box.wait_for(state="visible")
        relink = box.locator(".repair-item[data-repair=relink]")
        want = cost(page, RELINK)
        assert wait_until(page, "[...document.querySelectorAll('.repair-item[data-repair=relink] .repair-cost')].some((e) => e.textContent === %s)"
                          % json.dumps(want), 5), "the re-link's cost"
        assert relink.locator(".repair-name").inner_text() == page.evaluate("t('repair.relink')"), "the re-link named"
        reset = box.locator(".repair-item[data-repair=roomReset]")
        assert reset.locator(".repair-none").inner_text() == NONE, "the reset says why it has nothing to do"
        assert reset.locator(".repair-run").count() == 0, "and offers no Run…"
        named_buttons(page, "a room's repairs")
        assert scroll_width(page) <= case.width, "no horizontal scroll with the repairs"
        relink.locator(".repair-run").click()
        question = relink.locator(".confirm-box .confirm-text")
        assert question.inner_text() == page.evaluate("t('repair.confirm', {what: t('repair.relink'), cost: %s})" % json.dumps(want)), \
            "Run… asks, naming the cost"
        assert focus_in_turn(page, ".room .repair-item[data-repair=relink] .confirm-no", ".room .repair-item[data-repair=relink] .repair-run"), \
            "Cancel gives the focus back to Run… in the drawing that shows it"
        assert not stub.runs and relink.locator(".repair-run").is_visible(), "Cancel sends nothing"
        relink.locator(".repair-run").click()
        relink.locator(".confirm-yes").click()
        done = page.evaluate("t('repair.done', {accepted: 3, lines: 3}) + ' ' + t('result.structurePending')")
        assert wait_until(page, "[...document.querySelectorAll('.repair-item[data-repair=relink] .repair-result')].some((e) => e.textContent === %s)"
                          % json.dumps(done), 5), "the outcome"
        assert len(stub.runs) == 1 and stub.runs[0][1].get("plan") == ["plan-relink"], f"one run with the preview's plan: {stub.runs}"
        tile = page.locator(".room").first
        tile.locator(".repair-close").click()
        assert tile.locator(".repair-box:not(.hidden)").count() == 0, "Close closes the box"
    finally:
        stub.close()


def test_device_repairs(page, case):
    """A device's Repair… opens its repairs under its row; a listing into
    a room the Cube's list lacks asks for the room's name first and then
    shows what it would write."""
    stub = RepairStub(page, {"config": "room 1 has no other thermostat to take the configuration from",
                             "place": "the Cube has the device in room 1 already, as its list", "list": LIST, "list-name": True})
    try:
        tile = page.locator(".room").first
        tile.locator(".room-name-btn").click()
        device = tile.locator(".device").first
        device.wait_for(state="visible")
        device.locator(".repair-open").click()
        box = page.locator(".device .repair-box:not(.hidden)").first
        box.wait_for(state="visible")
        listing = box.locator(".repair-item[data-repair=list]")
        listing.locator("input[name=room_name]").wait_for(state="visible")
        assert listing.locator("label").inner_text() == page.evaluate("t('repair.roomName', {room: 7})"), "the name asked for"
        listing.locator("input[name=room_name]").fill("Attic")
        listing.locator("button[type=submit]").click()
        want = page.evaluate("t('repair.costMetadata')")
        assert wait_until(page, "[...document.querySelectorAll('.repair-item[data-repair=list] .repair-cost')].some((e) => e.textContent === %s)"
                          % json.dumps(want), 5), "the listing's cost: none"
        named_buttons(page, "a device's repairs")
        assert scroll_width(page) <= case.width, "no horizontal scroll with a device's repairs"
        page.locator(".repair-item[data-repair=list] .repair-run").click()
        page.locator(".repair-item[data-repair=list] .confirm-yes").click()
        assert wait_until(page, "document.querySelector('.repair-item[data-repair=list] .repair-result.ok') !== null", 5), "the listing ran"
        assert stub.runs and stub.runs[0][1] == {"room_name": ["Attic"], "plan": ["plan-list"]}, f"with the name and the plan: {stub.runs}"
    finally:
        stub.close()


def test_listing_names_the_device(page, case, reads):
    """A listing of a device the Cube's list lacks offers its name (D185):
    the room's, the type in the page's language and the lowest free
    number, whatever the case of the names the room's devices have, reads
    the preview with it and runs with it; a name changed reads the preview
    again, one too long is marked on its field and not read (D201)."""
    unlisted = dict(LIST, details={"from": 0, "to": 1, "listed": False, "new_room": False, "name": "Bath radiator thermostat 1",
                                   "room_name": "Bath", "type": "radiator_thermostat"})
    stub = RepairStub(page, {"config": "room 1 has no other thermostat to take the configuration from",
                             "place": "the Cube's list does not list device 0a0101: list it instead", "list": unlisted})
    names = []

    def seen(req):
        if req.method == "GET" and "/repair/list" in req.url:
            names.append(parse_qs(urlparse(req.url).query).get("name"))

    page.on("request", seen)
    try:
        word = page.evaluate("t('type.radiator_thermostat')")
        # Room 1's devices named, as far as it has them, "bath <TYPE> 1",
        # "Bath <type> 3" and "Bath <type> x" (GET …/rooms answered so):
        # the lowest free number is 2, whatever the case.
        rename_room_devices(page, reads, 1, ["bath " + word.upper() + " 1", "Bath " + word + " 3", "Bath " + word + " x"])
        want = "Bath " + word + " 2"
        listing = listing_offering(page, 1, "bath " + word.upper() + " 1", want)
        field = listing.locator("input[name=name]")
        assert listing.locator(".repair-device-name-form label").inner_text() == page.evaluate("t('repair.deviceName')"), "the name asked for"
        assert wait_until(page, "document.querySelector('.repair-item[data-repair=list] .repair-cost') !== null", 5), "the preview shown"
        assert names and names[-1] == [want], f"the preview read with the name offered: {names}"
        asked = len(names)
        field.fill("x" * 33)
        hint = listing.locator(".repair-name-long")
        assert hint.is_visible() and hint.inner_text() == page.evaluate("t('name.long', {n: 32})"), "a name too long marked as it is typed"
        assert field.get_attribute("aria-invalid") == "true" and field.get_attribute("aria-describedby") == hint.get_attribute("id"), \
            "the field marked, the limit its description"
        listing.locator(".repair-device-name-form button[type=submit]").click()
        page.wait_for_timeout(300)
        assert field.evaluate("(el) => el === document.activeElement") and len(names) == asked, "and not read: back to the field"
        field.fill("Sofa side")
        assert hint.is_hidden() and field.get_attribute("aria-invalid") is None, "a name that fits clears the mark"
        listing.locator(".repair-device-name-form button[type=submit]").click()
        assert wait_until(page, "document.querySelector('.repair-item[data-repair=list] input[name=name]') && "
                          "document.querySelector('.repair-item[data-repair=list] input[name=name]').value === 'Sofa side' && "
                          "document.querySelector('.repair-item[data-repair=list] .repair-cost') !== null", 5), "the preview read again"
        assert names[-1] == ["Sofa side"], f"with the name changed: {names}"
        named_buttons(page, "a listing's name")
        assert scroll_width(page) <= case.width, "no horizontal scroll with the listing's name"
        page.locator(".repair-item[data-repair=list] .repair-run").click()
        page.locator(".repair-item[data-repair=list] .confirm-yes").click()
        assert wait_until(page, "document.querySelector('.repair-item[data-repair=list] .repair-result.ok') !== null", 5), "the listing ran"
        assert stub.runs and stub.runs[0][1] == {"name": ["Sofa side"], "plan": ["plan-list"]}, f"with the name and the plan: {stub.runs}"
    finally:
        page.remove_listener("request", seen)
        stub.close()


def rename_room_devices(page, reads, room, names):
    """The room's real devices named names, in their order, as far as it
    has them (GET …/rooms answered so, Reads.device), and the rooms loaded
    again; the devices kept, since the server's own events of them bring
    their names back (AP ToldData, Block 4): listing_offering waits for
    them."""
    rfs = page.evaluate("(room) => state.rooms.find((r) => r.id === room).devices.map((d) => d.rf_address)", room)
    for rf, name in zip(rfs, names):
        reads.device(rf, name=name)
    page.evaluate("loadRooms()")
    assert wait_until(page, "state.rooms.find((r) => r.id === %d).devices.some((d) => d.name === %s)" % (room, json.dumps(names[0])), 5), \
        "the room's devices renamed"


# The name a device's listing offers, once its preview has come: "" before.
OFFERED = """() => { const f = document.querySelector('.device .repair-box:not(.hidden) .repair-item[data-repair=list] input[name=name]');
  return f ? f.value : ""; }"""


def listing_offering(page, room, name, want):
    """The first tile's device list open, its first device's repairs open
    until their listing offers want: its item. The name is offered once,
    from the names of the room's devices as the page has them when the
    preview comes, and a server's event of a device renamed (Reads.device,
    kept) draws the server's name until keep's read brings the told one
    back; so where the name offered is not want, the repairs are closed
    and opened again once the page has the told name (name, of the room
    room) again."""
    tile = page.locator(".room").first
    tile.locator(".room-name-btn").click()
    device = tile.locator(".device").first
    device.wait_for(state="visible")
    end = time.time() + 15
    while True:
        assert wait_until(page, "state.rooms.find((r) => r.id === %d).devices.some((d) => d.name === %s)" % (room, json.dumps(name)), 5), \
            "the room's devices as renamed"
        device.locator(".repair-open").click()
        assert wait_until(page, "(%s)() !== ''" % OFFERED, 5), "a name offered"
        got = page.evaluate(OFFERED)
        if got == want:
            return page.locator(".device .repair-box:not(.hidden) .repair-item[data-repair=list]").first
        assert time.time() < end, f"the name offered: {got!r}, want {want!r}"
        device.locator(".repair-open").click()  # closed, to be opened anew
        assert wait_until(page, "(%s)() === ''" % OFFERED, 5), "the repairs closed"


def open_device_listing(page):
    """The first tile's device list open, its first device's repairs open:
    their listing's item."""
    tile = page.locator(".room").first
    tile.locator(".room-name-btn").click()
    device = tile.locator(".device").first
    device.wait_for(state="visible")
    device.locator(".repair-open").click()
    return page.locator(".device .repair-box .repair-item[data-repair=list]").first


def test_listing_without_a_room_name(page, case, reads):
    """A listing into a room whose name the Cube's list does not give
    offers the type alone with the lowest free number: "wall thermostat
    2" where the room has a "wall thermostat 1"."""
    word = page.evaluate("t('type.wall_thermostat')")
    rename_room_devices(page, reads, 1, [word + " 1"])
    unlisted = dict(LIST, details={"from": 0, "to": 1, "listed": False, "new_room": False, "name": "wall thermostat 2", "room_name": "",
                                   "type": "wall_thermostat"})
    stub = RepairStub(page, {"place": "the Cube's list does not list device 0a0101: list it instead", "list": unlisted})
    try:
        listing_offering(page, 1, word + " 1", word + " 2")  # the type alone, the lowest free number
        assert not stub.runs, "nothing run"
    finally:
        stub.close()


def test_a_question_goes_with_its_preview(page, case):
    """A question never outlives the preview it asked about: a question
    open in a room's box is gone once another repair of the box has run
    and the previews are read again; a listing's question is gone once a
    name is given, and the run after a new Yes sends the new preview's
    plan."""
    master = dict(RELINK, action="master", plan="plan-master")
    stub = RepairStub(page, {"reset": NONE, "master": master, "relink": RELINK})
    try:
        tile = page.locator(".room").first
        tile.locator(".tile-head .menu > summary").click()
        tile.locator(".menu-repair").click()
        relink = tile.locator(".repair-item[data-repair=relink]")
        relink.locator(".repair-run").click()
        assert relink.locator(".confirm-box").is_visible(), "the re-link asks"
        other = tile.locator(".repair-item[data-repair=master]")
        other.locator(".repair-run").click()
        other.locator(".confirm-yes").click()
        assert wait_until(page, "document.querySelector('.repair-item[data-repair=master] .repair-result.ok') !== null", 5), "the master ran"
        assert wait_until(page, "!!document.querySelector('.repair-item[data-repair=relink] .repair-run')", 5), "the re-link read again"
        assert relink.locator(".confirm-box").count() == 0, "its question gone with its preview"
        assert len(stub.runs) == 1, f"one run: {stub.runs}"
        tile.locator(".repair-close").click()
    finally:
        stub.close()

    def listing(query):
        name = (query.get("name") or ["offered"])[0]
        return dict(LIST, plan="plan-" + name, details={"from": 0, "to": 1, "listed": False, "new_room": False, "name": name,
                                                         "room_name": "Bath", "type": "radiator_thermostat"})

    stub = RepairStub(page, {"place": "the Cube's list does not list device 0a0101: list it instead", "list": listing})
    try:
        item = open_device_listing(page)
        item.locator(".repair-run").click()
        assert item.locator(".confirm-box").is_visible(), "the listing asks"
        item.locator("input[name=name]").fill("Sofa side")
        item.locator(".repair-device-name-form button[type=submit]").click()
        assert wait_until(page, "!!document.querySelector('.repair-item[data-repair=list] .repair-run')", 5), "the preview read with the name"
        assert item.locator(".confirm-box").count() == 0, "its question gone with its preview"
        item.locator(".repair-run").click()
        item.locator(".confirm-yes").click()
        assert wait_until(page, "document.querySelector('.repair-item[data-repair=list] .repair-result.ok') !== null", 5), "the listing ran"
        assert [q.get("plan") for _, q in stub.runs] == [["plan-Sofa side"]], f"the new preview's plan: {stub.runs}"
    finally:
        stub.close()


# The name a listing into Wohnzimmer offers a radiator thermostat: the
# German "Wohnzimmer Heizkörperthermostat 1" is 34 bytes, past the 32 the
# Cube keeps; the English 32.
WOHNZIMMER = {"en": "Wohnzimmer radiator thermostat 1", "de": "Wohnzimmer Heizkörperthermostat 1"}


def test_listing_name_too_long(page, case):
    """A name offered past the Cube's 32 bytes stays in its field as it
    is, marked (D201): no preview is read with it and nothing runs until
    it is shortened; then the preview is read with the shorter name and
    runs with it. The English name fits and is read at once."""
    unlisted = dict(LIST, details={"from": 0, "to": 1, "listed": False, "new_room": False, "name": "Wohnzimmer radiator thermostat 1",
                                   "room_name": "Wohnzimmer", "type": "radiator_thermostat"})
    stub = RepairStub(page, {"config": "room 1 has no other thermostat to take the configuration from",
                             "place": "the Cube's list does not list device 0a0101: list it instead", "list": unlisted})
    names = []

    def seen(req):
        if req.method == "GET" and "/repair/list" in req.url:
            names.append(parse_qs(urlparse(req.url).query).get("name"))

    page.on("request", seen)
    try:
        offered = WOHNZIMMER[case.lang]
        tile = page.locator(".room").first
        tile.locator(".room-name-btn").click()
        device = tile.locator(".device").first
        device.wait_for(state="visible")
        device.locator(".repair-open").click()
        listing = page.locator(".device .repair-box:not(.hidden) .repair-item[data-repair=list]").first
        field = listing.locator("input[name=name]")
        field.wait_for(state="visible")
        assert field.input_value() == offered, "the name offered, as it is"
        hint = listing.locator(".repair-name-long")
        name = offered
        if case.lang == "de":
            assert hint.is_visible() and hint.inner_text() == "länger als 32 Bytes", "the limit, on the field"
            assert field.get_attribute("aria-invalid") == "true", "the field marked"
            box, before = field.bounding_box(), hint.bounding_box()
            assert before["y"] + before["height"] <= box["y"] + 1, "the limit above the box"
            assert listing.locator(".repair-run").count() == 0 and listing.locator(".repair-cost").count() == 0, "nothing to run"
            assert names == [None], f"no preview read with it: {names}"
            assert scroll_width(page) <= case.width, "no horizontal scroll with the limit"
            listing.locator(".repair-device-name-form button[type=submit]").click()
            page.wait_for_timeout(300)
            assert field.evaluate("(el) => el === document.activeElement") and names == [None], "Show what it sends goes back to the field"
            name = "Wohnzimmer Heizkörper 1"
            field.fill(name)
            assert hint.is_hidden() and field.get_attribute("aria-invalid") is None, "the shorter name clears the mark"
            listing.locator(".repair-device-name-form button[type=submit]").click()
        else:
            assert hint.is_hidden() and field.get_attribute("aria-invalid") is None, "a name of 32 bytes fits"
        assert wait_until(page, "document.querySelector('.repair-item[data-repair=list] .repair-run') !== null", 5), "Run… once it fits"
        assert names[-1] == [name], f"the preview read with the name: {names}"
        page.locator(".repair-item[data-repair=list] .repair-run").click()
        page.locator(".repair-item[data-repair=list] .confirm-yes").click()
        assert wait_until(page, "document.querySelector('.repair-item[data-repair=list] .repair-result.ok') !== null", 5), "the listing ran"
        assert stub.runs and stub.runs[0][1] == {"name": [name], "plan": ["plan-list"]}, f"with the name and the plan: {stub.runs}"
    finally:
        page.remove_listener("request", seen)
        stub.close()


def test_cube_repairs(page, case):
    """The Cube's panel has Repair…, with the reset of every communication
    error, free; its run says the device list shows no flag any more."""
    stub = RepairStub(page, {"reset": RESETS})
    try:
        show_view(page, "system")
        panel = page.locator(PANEL)
        panel.locator(":scope > summary").click()
        panel.locator(".cube-repair .repair-open").click()
        item = panel.locator(".repair-item[data-repair=reset]")
        want = cost(page, RESETS)
        assert wait_until(page, "[...document.querySelectorAll(%s)].some((e) => e.textContent === %s)"
                          % (json.dumps(PANEL + " .repair-item[data-repair=reset] .repair-cost"), json.dumps(want)), 5), "the resets cost no radio time"
        named_buttons(page, "the Cube's repairs")
        assert scroll_width(page) <= case.width, "no horizontal scroll with the Cube's repairs"
        item.locator(".repair-run").click()
        item.locator(".confirm-yes").click()
        done = page.evaluate("t('repair.done', {accepted: 3, lines: 3}) + ' ' + t('repair.cleared')")
        assert wait_until(page, "[...document.querySelectorAll('.repair-item[data-repair=reset] .repair-result')].some((e) => e.textContent === %s)"
                          % json.dumps(done), 5), "the outcome"
    finally:
        stub.close()
    show_view(page, "tiles")


def test_unlink_relink_warns_and_asks_twice(page, case):
    """"Unlink all and re-link" warns first, naming its cost and what a
    run that stops halfway leaves; Continue asks once more, and only its
    Yes sends the run; Cancel at either step sends nothing."""
    stub = RepairStub(page, {"reset": NONE, "master": NONE, "relink": RELINK, "unlink-relink": UNLINK})
    try:
        tile = page.locator(".room").first
        tile.locator(".tile-head .menu > summary").click()
        tile.locator(".menu-repair").click()
        item = tile.locator(".repair-item[data-repair=unlinkRelink]")
        item.locator(".repair-run").wait_for(state="visible")
        assert focus_in_turn(page, ".room .repair-item[data-repair=unlinkRelink] .repair-run", ".room .repair-item[data-repair=unlinkRelink] .repair-warning-no"), \
            "the warning's Cancel takes the focus in the drawing that shows it"
        warning = item.locator(".repair-warning")
        want = page.evaluate("(c) => t('repair.unlinkRelinkWarning', {cost: c})", cost(page, UNLINK))
        assert warning.locator(".repair-warning-text").inner_text() == want, "the warning names the cost and the risk"
        assert scroll_width(page) <= case.width, "no horizontal scroll with the warning"
        named_buttons(page, "the warning")
        warning.locator(".repair-warning-no").click()
        assert item.locator(".repair-warning").count() == 0 and not stub.runs, "Cancel sends nothing"
        item.locator(".repair-run").click()
        item.locator(".repair-warning-yes").click()
        box = item.locator(".confirm-box")
        assert box.locator(".confirm-text").inner_text() == page.evaluate("t('repair.unlinkRelinkConfirm')"), "the second question"
        box.locator(".confirm-no").click()
        assert not stub.runs and item.locator(".repair-run").is_visible(), "Cancel at the second question sends nothing"
        item.locator(".repair-run").click()
        item.locator(".repair-warning-yes").click()
        item.locator(".confirm-yes").click()
        assert wait_until(page, "document.querySelector('.repair-item[data-repair=unlinkRelink] .repair-result.ok') !== null", 5), "it ran"
        assert len(stub.runs) == 1 and stub.runs[0][1].get("plan") == ["plan-unlink"], f"one run with its plan: {stub.runs}"
    finally:
        stub.close()
