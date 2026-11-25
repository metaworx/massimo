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
reaches the fake Cube, which does not change its states. The modes are
told through the page's reads of the rooms (Modes, Reads.device) and
kept (AP ToldData, Block 2): the server's own state events of the
room's thermostats, which an earlier test's write brings, draw the
server's modes, and keep's read of the rooms draws the told ones again,
so each check waits for what it looks at. The keyboard's test takes
the room in Auto as the server has it (SET_MODE)."""
import datetime
import json
import time

from playwright.sync_api import Error as PlaywrightError

from helpers import hold, wait_held, wait_until

UNTIL = {"en": "12/24, 18:00", "de": "24.12., 18:00"}
ON_MORE = {"en": "Holiday until {}", "de": "Urlaub bis {}"}
IN_PILL = {"en": "holiday until {}", "de": "Urlaub bis {}"}

# Sets the thermostats of a room to a mode in the page alone, with until
# for a holiday, the first of them alone with one: the room of the id
# given, else the first room with two thermostats, else the first with
# one; returns the room's id.
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


# The room the tests take, as SET_MODE does: the first with two
# thermostats, else the first with one.
ROOM = "() => (state.rooms.find((r) => r.devices.filter(isThermostat).length >= 2) || state.rooms.find(roomThermostat)).id"
# A twin of a room's first thermostat, which the server does not know.
TWIN = "0a0fff"

# Whether a room's thermostats have the modes told, by radio address
# [mode, until], in the page's data, and its tile shows the mode in the
# same look: AUTO or MANU pressed, or More pressed for a holiday, with
# Hold shown in Auto alone; where they differ, nothing more.
SHOWS = """([rid, want]) => {
  const r = state.rooms.find((x) => x.id === rid);
  if (!r) return false;
  const has = Object.entries(want).every(([rf, [mode, until]]) => {
    const d = r.devices.find((x) => x.rf_address === rf);
    return !!d && !!d.state && d.state.mode === mode && (d.state.until || null) === until;
  });
  const modes = new Set(Object.values(want).map(([m]) => m));
  if (!has || modes.size > 1) return has;
  const tile = document.querySelector('.room[data-id="' + rid + '"]');
  const mode = [...modes][0];
  const on = (c) => !!tile && !!tile.querySelector(c) && tile.querySelector(c).classList.contains("active");
  const hold = tile && tile.querySelector(".more-hold");
  if (!hold || hold.classList.contains("hidden") !== (mode !== "auto")) return false;
  return mode === "auto" ? on(".mode-auto") : mode === "manual" ? on(".mode-manual") : mode === "vacation" ? on(".more-sum") : true;
}"""


class Modes:
    """The modes of the room's thermostats as the test tells them to the
    page's reads of the rooms (Reads.device), a twin of its first
    thermostat beside it where told, the room's thermostats and its
    virtual wall thermostat kept (Reads.room): the server's own state
    events of them, which bring the server's modes, are followed by a
    read that brings the told ones back, as is any read of the page's own
    (AP ToldData, Block 2)."""

    def __init__(self, page, reads):
        self.page, self.reads = page, reads
        self.rid = page.evaluate(ROOM)
        self.rfs = page.evaluate("(rid) => state.rooms.find((r) => r.id === rid).devices.filter(isThermostat).map((d) => d.rf_address)",
                                 self.rid)
        self.told = {}  # radio address: [mode, until]
        reads.room(self.rid)

    def twin(self):
        """A second thermostat for a room of one, a copy of the first, so
        that its thermostats can differ; its list is not opened, the server
        not knowing it."""
        if len(self.rfs) < 2:
            self.reads.device(TWIN, like=self.rfs[0], serial="MEQ0000999", name="Twin")
            self.rfs.append(TWIN)

    def tell(self, mode, until=None, one=False):
        """The room's thermostats, the first alone where one, in mode, with
        until for a holiday, in the reads from now on; read at once, and
        waited for until the page has them and the tile shows them
        (shows)."""
        for rf in self.rfs[:1] if one else self.rfs:
            self.reads.device(rf, state={"mode": mode, "until": until})
            self.told[rf] = [mode, until]
        self.page.evaluate("loadRooms()")
        self.shows()

    def shows(self):
        """Waits until the page has the modes told and the tile shows them
        (SHOWS): a server's event of a thermostat draws its own until
        keep's read is answered."""
        want = {rf: self.told.get(rf, [None, None]) for rf in self.rfs if rf in self.told}
        assert wait_until(self.page, "(%s)(%s)" % (SHOWS, json.dumps([self.rid, want])), 5), f"the room shows the modes told: {want}"


def open_list(tile, check, what):
    """The tile head's problem list opened, again where a redraw closed it
    (a server's event of a thermostat, then keep's read, each draw the
    list anew, closed), until check (a JS function of the list's details)
    holds."""
    details = tile.locator(".tile-head details.problems")
    end = time.time() + 5
    while True:
        try:
            if not details.evaluate("(d) => d.open", timeout=1000):
                details.locator(":scope > summary").click(timeout=1000)
            if details.evaluate(check, timeout=1000):
                return
        except PlaywrightError:
            pass
        assert time.time() < end, what
        tile.page.wait_for_timeout(100)


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


def test_eco_and_comfort_keep_the_mode(page, case, reads):
    """In Auto, Eco sends the eco temperature in Auto, until the next
    switch point, More turning meanwhile; with Hold on, Comfort sends
    the comfort temperature in Manual, and Hold stays on; in Manual, Hold
    is not shown and Eco keeps Manual; on holiday, Eco keeps the holiday
    and its end. Each item shows its temperature. Before each send the
    tile shows the mode told (Modes.shows)."""
    modes = Modes(page, reads)
    modes.tell("auto")
    rid = modes.rid
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

        modes.shows()
        assert send(page, tile, ".menu-eco", held, sent) == {"mode": "auto", "temp": temps["eco"]}, f"Eco keeps Auto: {sent}"
        assert tile.locator(".more-menu").get_attribute("open") is None, "the choice closes the menu"
        assert page.evaluate("document.querySelector(%r).classList.contains('busy')" % (sel + " .more-sum")), "More turns"
        assert tile.locator(".menu-comfort").is_disabled(), "the menu's writes are off meanwhile"
        answer(page, held, sel)

        modes.shows()
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

        modes.tell("manual")
        tile.locator(".more-sum").click()
        assert not tile.locator(".more-hold").is_visible(), "no Hold in Manual"
        page.keyboard.press("Escape")
        modes.shows()
        assert send(page, tile, ".menu-eco", held, sent) == {"mode": "manual", "temp": temps["eco"]}, f"Eco keeps Manual: {sent}"
        answer(page, held, sel)

        modes.tell("vacation", "2026-12-24 18:00")
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


def test_a_holiday_shows_its_end(page, case, reads):
    """A room on holiday: More pressed, saying until when; the room's
    pill with the suitcase, naming the holiday and its end to a screen
    reader, and listing each thermostat with it in the holiday's colour;
    the device list with the end in the same form; a holiday into
    another year with its year. One thermostat back in Auto: More partly
    pressed, its plain label, the pill with the mixed modes and the
    holiday. All in Auto: no suitcase. Each look waits for what it
    checks, in one look, and the list is opened again where a redraw
    closed it."""
    year = datetime.date.today().year
    modes = Modes(page, reads)
    modes.tell("vacation", "%d-12-24 18:00" % year)
    rid = modes.rid
    tile, sel = tile_of(page, rid)
    want = ON_MORE[case.lang].format(UNTIL[case.lang])
    item = IN_PILL[case.lang].format(UNTIL[case.lang])
    assert wait_until(page, """(([sel, want, item]) => { const m = document.querySelector(sel + ' .more-sum');
      const s = document.querySelector(sel + ' .tile-head .problems-sum');
      return !!m && m.textContent.trim() === want && m.classList.contains('active') && !!s
        && !!s.querySelector("use[href='#icon-suitcase']") && (s.getAttribute('aria-label') || '').includes(item); })(%s)"""
                      % json.dumps([sel, want, item]), 5), "More says until when, pressed; the pill with the suitcase, named to a screen reader"
    open_list(tile, """(d) => { const f = [...d.querySelectorAll('.problem-entry .flag.fill-info')];
      return f.length === %d && f.every((x) => x.innerText.trim() === %s); }""" % (len(modes.rfs), json.dumps(item)),
              "each thermostat listed with the holiday and its end")
    page.keyboard.press("Escape")
    tile.locator(".room-name-btn").click()
    page.wait_for_selector(sel + " .device", timeout=5000)
    assert wait_until(page, """(([sel, until]) => { const e = [...document.querySelectorAll(sel + ' .device-vals .cell.wide')].map((c) => c.innerText);
      return e.length > 0 && e.every((x) => x.includes(until)); })(%s)""" % json.dumps([sel, UNTIL[case.lang]]), 5), \
        "the device list's end in the same form"
    tile.locator(".room-name-btn").click()

    modes.tell("vacation", "%d-01-06 09:30" % (year + 1))
    assert wait_until(page, "document.querySelector(%r).textContent.includes(%r)" % (sel + " .more-sum", str(year + 1)), 5), \
        "another year's end with its year"

    modes.twin()
    modes.tell("vacation", "%d-12-24 18:00" % year)
    modes.tell("auto", one=True)
    plain = page.evaluate("t('mode.more')")
    # More's class, its label and the pill's icons, in one look
    look = """(([sel, plain, suitcase]) => { const m = document.querySelector(sel + ' .more-sum');
      const icons = [...document.querySelectorAll(sel + ' .tile-head .problems-sum svg > use:first-child')].map((u) => u.getAttribute('href'));
      return !!m && m.textContent.trim() === plain && (suitcase === null
        ? m.classList.contains('partial') && !m.classList.contains('active') && icons.includes('#icon-shuffle') && icons.includes('#icon-suitcase')
        : icons.includes('#icon-suitcase') === suitcase); })(%s)"""
    assert wait_until(page, look % json.dumps([sel, plain, None]), 5), \
        "More partly pressed, its plain label; the pill with the mixed modes and the holiday"
    modes.tell("auto")
    assert wait_until(page, look % json.dumps([sel, plain, False]), 5), "no suitcase once the holiday is over, More's plain label"


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
           wrapped: box(tile.querySelector('.mode-row .more-sum')).top >= first.bottom, rows,
           label: tile.querySelector('.mode-row .more-sum').textContent.trim() };
}"""


def test_a_holiday_keeps_the_tools_up_and_the_values_at_the_edge(page, case, reads):
    """A room on holiday (AP RoomsPage v1.0, Block 2): in a narrow tile,
    a phone's or one of three columns at 1100 px, More, saying until
    when, wraps under AUTO and MANU, and the tile's tools stay at the top
    of the modes' row, level with AUTO, not centred on its two lines; in
    the open device list, each row's right column, Set and Actual, ends
    at the row's right edge, at 390 px and at 1100 px. Measured in a look
    in which More says the holiday."""
    modes = Modes(page, reads)
    modes.tell("vacation", "%d-12-24 18:00" % datetime.date.today().year)
    rid = modes.rid
    tile, sel = tile_of(page, rid)
    want = ON_MORE[case.lang].format(UNTIL[case.lang])
    tile.locator(".tile-info").click()
    page.wait_for_selector(sel + " .device .device-vals", timeout=5000)
    try:
        end = time.time() + 5
        geo = page.evaluate(HOLIDAY_ROW, sel)
        while geo["label"] != want and time.time() < end:
            page.wait_for_timeout(100)
            geo = page.evaluate(HOLIDAY_ROW, sel)
        assert geo["label"] == want, f"More says until when: {geo}"
        assert geo["wrapped"] or case.width > 480, f"on a phone More wraps under AUTO: {geo}"
        assert abs(geo["tools"] - geo["first"]) <= 1, f"the tools level with AUTO: {geo}"
        assert geo["rows"], f"a device row with Set and Actual: {geo}"
        assert all(abs(r["edge"] - r["end"]) <= 2 for r in geo["rows"]), f"the values end at the row's edge: {geo['rows']}"
    finally:
        tile.locator(".tile-info").click()
