# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A room tile's More menu (AP RoomTile): Eco and Comfort set the room
thermostat's eco or comfort temperature and keep the room's mode, the
Hold (Manual) switch, shown while the room is in Auto, makes them hold
it in Manual, and Holiday… opens the holiday form; the arrow keys move
between its items and Escape closes it.

Every write is held at its route and answered with a stub, so nothing
reaches the fake Cube; the modes come from state events applied in the
page (updateDevice), since the fake Cube does not change its states."""
from helpers import hold, wait_held, wait_until

# Sets the thermostats of the first room with two (else the first room
# with one) to a mode, with until for a holiday, the first of them alone
# with one; returns the room's id.
SET_MODE = """([mode, until, one]) => {
  const r = state.rooms.find((r) => r.devices.filter(isThermostat).length >= 2) || state.rooms.find(roomThermostat);
  const ths = r.devices.filter(isThermostat);
  for (const d0 of one ? ths.slice(0, 1) : ths) {
    const d = JSON.parse(JSON.stringify(d0));
    d.state = Object.assign({}, d.state, { mode });
    if (until) d.state.until = until;
    else delete d.state.until;
    updateDevice(d);
  }
  return r.id;
}"""


def tile_of(page, rid):
    """The room's tile and its selector for page.evaluate."""
    sel = '.room[data-id="%s"]' % rid
    return page.locator(sel), sel


def presets(page, rid):
    """The room thermostat's eco and comfort temperatures, as the page
    reads them from its configuration."""
    return page.evaluate("(rid) => presetTemps(roomThermostat(state.rooms.find((r) => r.id === rid)))", rid)


def answer(page, held, sel):
    """Answers the held writes with a stub and waits until the tile's
    write is over."""
    for route in held:
        route.fulfill(status=200, content_type="application/json", body="{}")
    held.clear()
    assert wait_until(page, "!document.querySelector(%r).classList.contains('busy')" % (sel + " .more-sum"), 5), \
        "More stops turning once answered"


def send(page, tile, item, held, sent):
    """Chooses an item of the More menu and returns the body it sent."""
    tile.locator(".more-sum").click()
    tile.locator(".more-menu " + item).click()
    assert wait_held(page, held), f"{item} sends the room's setpoint"
    return sent[-1]


def test_eco_and_comfort_keep_the_mode(page, case):
    """In Auto, Eco sends the eco temperature in Auto, until the next
    switch point, More turning meanwhile; with Hold on, Comfort sends
    the comfort temperature in Manual, and Hold stays on; in Manual, Hold
    is not shown and Eco keeps Manual; on holiday, Eco keeps the holiday
    and its end. Each item shows its temperature."""
    rid = page.evaluate(SET_MODE, ["auto", None, False])
    tile, sel = tile_of(page, rid)
    temps = presets(page, rid)
    assert temps.get("eco") is not None and temps.get("comfort") is not None, f"the thermostat has both: {temps}"
    held, sent = [], []
    hold(page, "**/rooms/*/setpoint", "PUT", held, sent)
    try:
        tile.locator(".more-sum").click()
        assert tile.locator(".more-hold").is_visible(), "Hold is shown while the room is in Auto"
        assert not tile.locator(".hold-input").is_checked(), "and off"
        assert wait_until(page, "document.querySelector(%r).textContent !== ''" % (sel + " .menu-eco .more-temp"), 5), \
            "Eco shows its temperature"
        eco_text = page.evaluate("fmtTemp(%r)" % temps["eco"])
        assert tile.locator(".menu-eco .more-temp").inner_text() == eco_text, "the eco temperature"
        tile.locator(".more-sum").click()
        assert tile.locator(".more-menu").get_attribute("open") is None, "a second click closes the menu"

        assert send(page, tile, ".menu-eco", held, sent) == {"mode": "auto", "temp": temps["eco"]}, f"Eco keeps Auto: {sent}"
        assert tile.locator(".more-menu").get_attribute("open") is None, "the choice closes the menu"
        assert page.evaluate("document.querySelector(%r).classList.contains('busy')" % (sel + " .more-sum")), "More turns"
        assert tile.locator(".menu-comfort").is_disabled(), "the menu's writes are off meanwhile"
        answer(page, held, sel)

        tile.locator(".more-sum").click()
        tile.locator(".more-hold").click()
        assert tile.locator(".hold-input").is_checked(), "Hold switched on"
        assert tile.locator(".more-menu").get_attribute("open") == "", "the menu stays open for the switch"
        tile.locator(".more-menu .menu-comfort").click()
        assert wait_held(page, held), "Comfort sends"
        assert sent[-1] == {"mode": "manual", "temp": temps["comfort"]}, f"with Hold, Comfort holds it in Manual: {sent}"
        answer(page, held, sel)
        tile.locator(".more-sum").click()
        assert tile.locator(".hold-input").is_checked(), "Hold stays on"
        page.keyboard.press("Escape")

        page.evaluate(SET_MODE, ["manual", None, False])
        tile.locator(".more-sum").click()
        assert not tile.locator(".more-hold").is_visible(), "no Hold in Manual"
        page.keyboard.press("Escape")
        assert send(page, tile, ".menu-eco", held, sent) == {"mode": "manual", "temp": temps["eco"]}, f"Eco keeps Manual: {sent}"
        answer(page, held, sel)

        page.evaluate(SET_MODE, ["vacation", "2026-12-24 18:00", False])
        body = send(page, tile, ".menu-eco", held, sent)
        assert body == {"mode": "vacation", "temp": temps["eco"], "until": "2026-12-24 18:00"}, f"Eco keeps the holiday: {sent}"
        answer(page, held, sel)
        assert len(sent) == 4, f"one PUT a choice: {sent}"
    finally:
        page.unroute("**/rooms/*/setpoint")


def test_the_menu_by_keyboard(page, case):
    """Enter opens More, the arrow keys move between Eco, Comfort, Hold
    and Holiday…, round at the ends, Escape closes it with the focus
    back on More, and Holiday… opens the holiday form."""
    rid = page.evaluate(SET_MODE, ["auto", None, False])
    tile, sel = tile_of(page, rid)
    focused = "(() => { const a = document.activeElement; return a ? a.className : ''; })()"
    tile.locator(".more-sum").focus()
    page.keyboard.press("Enter")
    assert tile.locator(".more-menu").get_attribute("open") == "", "Enter opens the menu"
    order = []
    for _ in range(5):
        page.keyboard.press("ArrowDown")
        order.append(page.evaluate(focused))
    assert order == ["menu-eco", "menu-comfort", "hold-input", "menu-holiday", "menu-eco"], f"down through the items: {order}"
    page.keyboard.press("ArrowUp")
    assert page.evaluate(focused) == "menu-holiday", "up from the first to the last"
    page.keyboard.press("Escape")
    assert tile.locator(".more-menu").get_attribute("open") is None, "Escape closes the menu"
    assert "more-sum" in page.evaluate(focused), "the focus back on More"
    page.keyboard.press("Enter")
    page.keyboard.press("ArrowUp")
    page.keyboard.press("Enter")
    assert tile.locator(".vacation-panel").is_visible(), "Holiday… opens the holiday form"
    assert page.evaluate(focused).startswith("vacation-until"), "its end date has the focus"
    tile.locator(".vacation-cancel").click()
    assert not tile.locator(".vacation-panel").is_visible(), "Cancel closes it"
