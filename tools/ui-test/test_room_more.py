# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A room tile's More menu (AP RoomTile): Eco and Comfort set the room's
eco or comfort temperature (AP RoomMajority, D19-A) and keep the room's
mode, the Hold (Manual) switch, shown while the room is in Auto, makes
them hold it in Manual, and Holiday… opens the holiday form; the arrow
keys move between its items and Escape closes it. A room on holiday
shows the holiday's end on More, in the room's problem pill and in the
device list; its tile's tools stay level with AUTO where More wraps
under it, and the device list's values end at the row's right edge
(AP RoomsPage v1.0, Block 2).

Every write is held at its route and answered with a stub, so nothing
reaches the fake Cube; the modes come from state events applied in the
page (updateDevice), since the fake Cube does not change its states."""
import datetime

from helpers import hold, wait_held, wait_until

UNTIL = {"en": "12/24, 18:00", "de": "24.12., 18:00"}
ON_MORE = {"en": "Holiday until {}", "de": "Urlaub bis {}"}
IN_PILL = {"en": "holiday until {}", "de": "Urlaub bis {}"}

# Sets the thermostats of a room to a mode, with until for a holiday,
# the first of them alone with one: the room of the id given, else the
# first room with two thermostats, else the first with one; returns the
# room's id.
SET_MODE = """([mode, until, one, rid]) => {
  const r = rid ? state.rooms.find((r) => r.id === rid)
    : state.rooms.find((r) => r.devices.filter(isThermostat).length >= 2) || state.rooms.find(roomThermostat);
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


# Gives a room of one thermostat a second, a copy of the first, in the
# page, so that its thermostats can differ; its list is not opened, the
# server not knowing it.
TWIN = """(rid) => {
  const r = state.rooms.find((r) => r.id === rid);
  if (r.devices.filter(isThermostat).length >= 2) return;
  const twin = JSON.parse(JSON.stringify(r.devices.find(isThermostat)));
  Object.assign(twin, { rf_address: "0a0fff", serial: "MEQ0000999", name: "Twin" });
  r.devices = [...r.devices, twin]; // a room's devices are a signal: its tile follows
}"""


def tile_of(page, rid):
    """The room's tile and its selector for page.evaluate."""
    sel = '.room[data-id="%s"]' % rid
    return page.locator(sel), sel


def presets(page, rid):
    """The room's eco and comfort temperatures, as the page reads them
    (presetTemps: the room's entry's)."""
    return page.evaluate("(rid) => presetTemps(state.rooms.find((r) => r.id === rid))", rid)


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
        assert not tile.locator(".more-hold input").is_checked(), "and off"
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
        assert tile.locator(".more-hold input").is_checked(), "Hold switched on"
        assert tile.locator(".more-menu").get_attribute("open") == "", "the menu stays open for the switch"
        tile.locator(".more-menu .menu-comfort").click()
        assert wait_held(page, held), "Comfort sends"
        assert sent[-1] == {"mode": "manual", "temp": temps["comfort"]}, f"with Hold, Comfort holds it in Manual: {sent}"
        answer(page, held, sel)
        tile.locator(".more-sum").click()
        assert tile.locator(".more-hold input").is_checked(), "Hold stays on"
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
    # the focused element by its class, the Hold switch's checkbox by its switch's
    focused = "(() => { const a = document.activeElement; return !a ? '' : a.matches('.more-hold input') ? 'more-hold' : a.className; })()"
    tile.locator(".more-sum").focus()
    page.keyboard.press("Enter")
    assert tile.locator(".more-menu").get_attribute("open") == "", "Enter opens the menu"
    order = []
    for _ in range(5):
        page.keyboard.press("ArrowDown")
        order.append(page.evaluate(focused))
    assert order == ["menu-eco", "menu-comfort", "more-hold", "menu-holiday", "menu-eco"], f"down through the items: {order}"
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
    tile.locator(".vacation-panel .form-cancel").click()
    assert not tile.locator(".vacation-panel").is_visible(), "Cancel closes it"


def head_icons(tile):
    """The symbols of the icons in a tile head's problem pill."""
    return [u.get_attribute("href") for u in tile.locator(".tile-head .problems-sum svg > use:first-child").all()]


def test_a_holiday_shows_its_end(page, case):
    """A room on holiday: More pressed, saying until when; the room's
    pill with the suitcase, naming the holiday and its end to a screen
    reader, and listing each thermostat with it in the holiday's colour;
    the device list with the end in the same form; a holiday into
    another year with its year. One thermostat back in Auto: More partly
    pressed, its plain label, the pill with the mixed modes and the
    holiday. All in Auto: no suitcase."""
    year = datetime.date.today().year
    rid = page.evaluate(SET_MODE, ["vacation", "%d-12-24 18:00" % year, False])
    tile, sel = tile_of(page, rid)
    more = tile.locator(".more-sum")
    want = ON_MORE[case.lang].format(UNTIL[case.lang])
    assert more.inner_text().strip() == want, f"More says until when: {more.inner_text()!r}"
    assert "active" in more.get_attribute("class"), "More is pressed"
    assert "#icon-suitcase" in head_icons(tile), f"the pill has the suitcase: {head_icons(tile)}"
    item = IN_PILL[case.lang].format(UNTIL[case.lang])
    summary = tile.locator(".tile-head .problems-sum")
    assert item in summary.get_attribute("aria-label"), f"named to a screen reader: {summary.get_attribute('aria-label')!r}"
    summary.click()
    flags = tile.locator(".tile-head .problem-entry .flag.fill-info")
    thermostats = page.evaluate("(rid) => state.rooms.find((r) => r.id === rid).devices.filter(isThermostat).length", rid)
    assert flags.count() == thermostats, f"each thermostat listed with the holiday: {flags.all_inner_texts()}"
    assert all(t.strip() == item for t in flags.all_inner_texts()), f"with its end: {flags.all_inner_texts()}"
    page.keyboard.press("Escape")
    tile.locator(".room-name-btn").click()
    page.wait_for_selector(sel + " .device", timeout=5000)
    ends = tile.locator(".device-vals .cell.wide").all_inner_texts()
    assert ends and all(UNTIL[case.lang] in e for e in ends), f"the device list's end in the same form: {ends}"
    tile.locator(".room-name-btn").click()

    page.evaluate(SET_MODE, ["vacation", "%d-01-06 09:30" % (year + 1), False, rid])
    assert str(year + 1) in more.inner_text(), f"another year's end with its year: {more.inner_text()!r}"

    page.evaluate(TWIN, rid)
    page.evaluate(SET_MODE, ["vacation", "%d-12-24 18:00" % year, False, rid])
    page.evaluate(SET_MODE, ["auto", None, True, rid])
    cls = more.get_attribute("class")
    assert "partial" in cls and "active" not in cls, f"More partly pressed: {cls}"
    assert more.inner_text().strip() == page.evaluate("t('mode.more')"), "with its plain label"
    icons = head_icons(tile)
    assert "#icon-shuffle" in icons and "#icon-suitcase" in icons, f"the mixed modes and the holiday: {icons}"
    page.evaluate(SET_MODE, ["auto", None, False, rid])
    assert "#icon-suitcase" not in head_icons(tile), "no suitcase once the holiday is over"
    assert more.inner_text().strip() == page.evaluate("t('mode.more')"), "More's plain label"


# A room's tile on holiday as it is laid out: the top of its first mode
# button and of its tools, whether More wraps under AUTO, and, for each
# row of its open device list whose right column shows a value, where
# the last value of that column ends and where the row does (a row
# hidden outside expert mode left out).
HOLIDAY_ROW = """(sel) => {
  const tile = document.querySelector(sel);
  const box = (el) => el.getBoundingClientRect();
  const first = box(tile.querySelector('.mode-row .mode-group > button'));
  const rows = [...tile.querySelectorAll('.devices .device')].flatMap((row) => {
    const cells = [...row.querySelectorAll('.device-vals > .cell:not(.wide)')];
    const right = cells.filter((c, i) => i % 2 === 1 && c.querySelector('.num').textContent !== '');
    if (!right.length || !box(row).width) return [];
    const ends = [...right[right.length - 1].querySelectorAll('*')].map(box).filter((b) => b.width > 0).map((b) => b.right);
    return [{ end: Math.round(Math.max(...ends) * 10) / 10, edge: Math.round(box(row).right * 10) / 10 }];
  });
  return { first: first.top, tools: box(tile.querySelector('.tile-tools .tool-btn')).top,
           wrapped: box(tile.querySelector('.mode-row .more-sum')).top >= first.bottom, rows };
}"""


def test_a_holiday_keeps_the_tools_up_and_the_values_at_the_edge(page, case):
    """A room on holiday (AP RoomsPage v1.0, Block 2): in a narrow tile,
    a phone's or one of three columns at 1100 px, More, saying until
    when, wraps under AUTO and MANU, and the tile's tools stay at the top
    of the modes' row, level with AUTO, not centred on its two lines; in
    the open device list, each row's right column, Set and Actual, ends
    at the row's right edge, at 390 px and at 1100 px."""
    rid = page.evaluate(SET_MODE, ["vacation", "%d-12-24 18:00" % datetime.date.today().year, False])
    tile, sel = tile_of(page, rid)
    tile.locator(".tile-info").click()
    page.wait_for_selector(sel + " .device .device-vals", timeout=5000)
    try:
        geo = page.evaluate(HOLIDAY_ROW, sel)
        assert geo["wrapped"] or case.width > 480, f"on a phone More wraps under AUTO: {geo}"
        assert abs(geo["tools"] - geo["first"]) <= 1, f"the tools level with AUTO: {geo}"
        assert geo["rows"], f"a device row with Set and Actual: {geo}"
        assert all(abs(r["edge"] - r["end"]) <= 2 for r in geo["rows"]), f"the values end at the row's edge: {geo['rows']}"
    finally:
        tile.locator(".tile-info").click()
