# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A setpoint as the eQ-3 devices show it (AP RoomMajority v1.1, Block 3):
4.5 °C is OFF in the tile's SET, its stepped value and a device row,
without a unit; a crescent moon marks a setpoint at the eco
temperature, a sun at the comfort temperature, the two side by side
where comfort is eco, in the tile by the room's and beside its value,
in a slot of its own so that nothing moves, in a device row by the
device's own and in the place of Set's gauge; the marks are labelled
in the configuration's words and in the accent colour, in light and
dark. The device entries carry the temperatures (comfort_temp,
eco_temp) as the devices' details have them, and a config event brings
new ones. The room entries (v1.2, Block 4, D19-A) name the thermostat
whose state is the room's (thermostat), which the tile's SET follows,
and carry the room's temperatures, as its configuration has them, by
which the tile marks its setpoint and More sets its presets, a wall
thermostat's own aside; a config event that changed a thermostat's
presets, and a state event of a thermostat the Cube gained or lost,
read the rooms again, a burst of them twice at most. While the room's
thermostats show different setpoints, a line under the setpoint row
names each with its setpoint and mark, not while a write of the tile or
the House tile waits for them to show it, a write of theirs is pending
or a window of the room is open, and goes once they agree. The
setpoint row fits its tile in every language (AP RoomsPage v1.0,
Block 3), its short labels whole, and a tap on IS or SET shows the full
term in a tip (Block 4). A chevron under SET's ° opens a list of the
room's temperatures, OFF and every half degree up to its highest, and
a field, whose choice or typed value is sent as the stepper sends it,
the value set already not sent again, a value outside the list refused
on the field; the list stays inside the window, upward where the room
below is short (Block 6).

The states come from state events applied in the page (updateDevice);
every write is held at its route and answered with a stub, so nothing
reaches the fake Cube. Where a test acts on them while the server may
speak, they are told to the rooms' read as well and kept (Told, AP
ToldData, Block 3): the server's own state events of the room's
thermostat and its virtual wall thermostat, which an earlier test's
write brings, draw the server's own, and any read of the rooms, the
page's or keep's, draws what was told, so a check of it waits."""
import json

import pytest

from helpers import ROOMS, busy_state, hold, scroll_width, settled, wait_held, wait_until
from test_room_more import open_list

# The room of the id given, else the first with two reported
# thermostats, else the first with one; with two given, a copy of its
# thermostat is put beside it in the page (as test_room_more.py's
# Modes.twin), so that its thermostats can differ. An open window of the
# room is closed in the page, as it would hide the members' line.
# Returns the room's id and its reported thermostats, the room
# thermostat first, by radio address, name and its eco and comfort
# temperatures; the twin's source (twin) and the windows closed (closed),
# by radio address, for Told.
ROOM = """([rid, two]) => {
  const reported = (r) => r.devices.filter((d) => isThermostat(d) && d.reported);
  const r = rid ? state.rooms.find((x) => x.id === rid)
    : state.rooms.find((x) => reported(x).length >= 2) || state.rooms.find((x) => reported(x).length >= 1);
  let source = null;
  if (two && reported(r).length < 2) {
    const twin = JSON.parse(JSON.stringify(reported(r)[0]));
    source = twin.rf_address;
    Object.assign(twin, { rf_address: "0a0fff", serial: "MEQ0000999", name: "Twin" });
    r.devices = [...r.devices, twin]; // a room's devices are a signal: its tile follows
  }
  const closed = [];
  for (const w of r.devices.filter((d) => d.type === "shutter_contact" && d.state && d.state.window_open)) {
    const d = JSON.parse(JSON.stringify(w));
    d.state.window_open = false;
    closed.push(d.rf_address);
    updateDevice(d);
  }
  const th = roomThermostat(r);
  const ths = [th, ...reported(r).filter((d) => d !== th)];
  return { id: r.id, devices: ths.map((d) => ({ rf: d.rf_address, name: deviceName(d), eco: d.eco_temp, comfort: d.comfort_temp })),
    twin: source, closed };
}"""

# Sets thermostats of a room, by radio address, to a state and their eco
# and comfort temperatures: [rid, {rf: {setpoint, mode, valve, eco,
# comfort, confirmation}}], confirmation null for none. The setpoints'
# times (received) stay as they were.
SET = """([rid, by]) => {
  const r = state.rooms.find((x) => x.id === rid);
  for (const [rf, v] of Object.entries(by)) {
    const d = JSON.parse(JSON.stringify(r.devices.find((x) => x.rf_address === rf)));
    d.state = Object.assign({}, d.state, { setpoint: v.setpoint === undefined ? d.state.setpoint : v.setpoint, mode: v.mode || "manual" });
    if (v.valve !== undefined) d.state.valve_pct = v.valve;
    if (v.eco !== undefined) d.eco_temp = v.eco;
    if (v.comfort !== undefined) d.comfort_temp = v.comfort;
    if (v.confirmation === null) delete d.confirmation;
    else if (v.confirmation) d.confirmation = Object.assign({ since: new Date().toISOString() }, v.confirmation);
    updateDevice(d);
  }
}"""

# Puts a window contact into a room in the page, a copy of the fixture's,
# and sets its window open or closed: [rid, open, stale], with stale an
# out-of-date contact whose last report (last_known) says it. Returns the
# copy's source and the contact as applied, for Told.
WINDOW = """([rid, open, stale]) => {
  const r = state.rooms.find((x) => x.id === rid);
  let w = r.devices.find((d) => d.rf_address === "0a0ffe");
  if (!w) {
    const src = state.rooms.flatMap((x) => x.devices).find((d) => d.type === "shutter_contact");
    w = Object.assign(JSON.parse(JSON.stringify(src)), { rf_address: "0a0ffe", serial: "MEQ0000998", name: "Window", room_id: rid });
    delete w.window;
    r.devices = [...r.devices, w];
  }
  const d = JSON.parse(JSON.stringify(w));
  const st = Object.assign({}, d.state || d.last_known, { window_open: open });
  if (stale) Object.assign(d, { reported: false, out_of_date: true, last_known: st, state: undefined });
  else Object.assign(d, { reported: true, out_of_date: undefined, last_known: undefined, state: st });
  updateDevice(JSON.parse(JSON.stringify(d)));
  const src = state.rooms.flatMap((x) => x.devices).find((x) => x.type === "shutter_contact" && x.rf_address !== "0a0ffe");
  return { source: src.rf_address, device: JSON.parse(JSON.stringify(d)) };
}"""

# Sets a room's eco and comfort temperatures, its entry's, as the server
# decides them over its thermostats: [rid, eco, comfort].
PRESETS = """([rid, eco, comfort]) => {
  const r = state.rooms.find((x) => x.id === rid);
  r.eco_temp = eco; // a room's fields are signals: its tile follows
  r.comfort_temp = comfort;
}"""

# Puts a wall thermostat into a room in the page, a copy of its
# thermostat with a wall thermostat's type, at a state and its own eco
# and comfort temperatures, and names it the room's thermostat, its
# entry's and its virtual wall thermostat's, as where the room's master
# decides: [rid, setpoint, eco, comfort]; returns its address and that of
# the thermostat it is a copy of.
WALL = """([rid, setpoint, eco, comfort]) => {
  const r = state.rooms.find((x) => x.id === rid);
  const src = r.devices.find((d) => isThermostat(d) && d.reported);
  const wall = Object.assign(JSON.parse(JSON.stringify(src)), { rf_address: "0a0ffd", serial: "NEQ0000998", name: "Wall",
    type: "wall_thermostat", eco_temp: eco, comfort_temp: comfort });
  wall.state = Object.assign({}, wall.state, { mode: "manual", setpoint });
  delete wall.state.valve_pct;
  r.devices = [...r.devices.filter((d) => d.rf_address !== "0a0ffd"), wall];
  r.thermostat = wall.rf_address;
  r.vwt = Object.assign({}, r.vwt, { sources: Object.assign({}, r.vwt && r.vwt.sources, { setpoint: [wall.rf_address] }) });
  return [wall.rf_address, src.rf_address];
}"""

# Gives a room's virtual wall thermostat, as its state event brings it,
# its sources.setpoint, the thermostats whose setpoint it shows: [rid,
# rfs], rfs null for none, as from a server before them.
VWT_SETPOINT = """([rid, rfs]) => {
  const r = state.rooms.find((x) => x.id === rid);
  const d = JSON.parse(JSON.stringify(r.vwt));
  d.sources = Object.assign({}, d.sources);
  if (rfs) d.sources.setpoint = rfs;
  else delete d.sources.setpoint;
  state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({ kind: "state", source: "device", level: "info",
    device: d.rf_address, serial: d.serial, time: new Date().toISOString(), state: d }) }));
}"""

TWIN = "0a0fff"
WINDOW_RF = "0a0ffe"


class Told:
    """A room (ROOM) whose states, presets, wall thermostat, window
    contact, virtual wall thermostat's sources and thermostat named go
    into the page as SET, PRESETS, WALL, WINDOW, VWT_SETPOINT and the
    entry's thermostat do, and into the rooms' read as well (Reads.device,
    Reads.room), the room's thermostats and its virtual wall thermostat
    kept (AP ToldData, Block 3). Put into the page, a state is the event
    under test where a test takes it so (updateDevice); told to the read,
    it comes back after the server's own state events of the room's
    devices, which draw the server's, and after any read of the rooms."""

    def __init__(self, page, reads, rid=None, two=False):
        self.page, self.reads = page, reads
        self.room = page.evaluate(ROOM, [rid, two])
        self.rid = self.room["id"]
        if self.room["twin"]:
            reads.device(TWIN, like=self.room["twin"], serial="MEQ0000999", name="Twin")
        for rf in self.room["closed"]:
            reads.device(rf, state={"window_open": False})
        reads.room(self.rid)

    def set(self, by):
        """SET's states of thermostats, by radio address."""
        by = {rf: dict(v) for rf, v in by.items()}
        now = self.page.evaluate("new Date().toISOString()")
        for rf, v in by.items():
            st = {"mode": v.get("mode") or "manual"}
            if v.get("setpoint") is not None:
                st["setpoint"] = v["setpoint"]
            if "valve" in v:
                st["valve_pct"] = v["valve"]
            fields = {"state": st}
            if "eco" in v:
                fields["eco_temp"] = v["eco"]
            if "comfort" in v:
                fields["comfort_temp"] = v["comfort"]
            if "confirmation" in v:
                if v["confirmation"]:
                    v["confirmation"] = dict({"since": now}, **v["confirmation"])
                fields["confirmation"] = v["confirmation"]
            self.reads.device(rf, **fields)
        self.page.evaluate(SET, [self.rid, by])

    def presets(self, eco, comfort):
        """PRESETS: the room's eco and comfort temperatures."""
        self.reads.room(self.rid, eco_temp=eco, comfort_temp=comfort)
        self.page.evaluate(PRESETS, [self.rid, eco, comfort])

    def wall(self, setpoint, eco, comfort):
        """WALL: a wall thermostat named the room's; returns its address."""
        rf, src = self.page.evaluate(WALL, [self.rid, setpoint, eco, comfort])
        self.reads.device(rf, like=src, serial="NEQ0000998", name="Wall", type="wall_thermostat", eco_temp=eco, comfort_temp=comfort,
                          state={"mode": "manual", "setpoint": setpoint, "valve_pct": None})
        self.reads.room(self.rid, thermostat=rf, sources={"setpoint": [rf]})
        return rf

    def window(self, is_open, stale):
        """WINDOW: the room's window contact, open or closed, stale or not."""
        got = self.page.evaluate(WINDOW, [self.rid, is_open, stale])
        d = got["device"]
        self.reads.device(WINDOW_RF, like=got["source"], room_id=self.rid, serial=d["serial"], name=d["name"], window=None,
                          reported=d["reported"], out_of_date=d.get("out_of_date"), last_known=d.get("last_known"), state=d.get("state"))

    def vwt_setpoint(self, rfs):
        """VWT_SETPOINT: the virtual wall thermostat's sources.setpoint, its
        state event dispatched in the page."""
        self.reads.room(self.rid, sources={"setpoint": rfs})
        self.page.evaluate(VWT_SETPOINT, [self.rid, rfs])

    def thermostat(self, rf):
        """The room entry's thermostat, None for none."""
        self.reads.room(self.rid, thermostat=rf)
        self.page.evaluate("([rid, rf]) => { state.rooms.find((r) => r.id === rid).thermostat = rf === null ? undefined : rf; }", [self.rid, rf])

    def kept(self):
        """The reads of the rooms keep has started on this page (PAGE_KIT)."""
        return self.page.evaluate("window.__uiTest.kept")

    def close(self):
        """The rooms read as the server has them again."""
        self.reads.clear(ROOMS)
        self.page.evaluate("async () => { await loadRooms(); }")


# The tile's SET and the room thermostat the page takes (roomThermostat),
# in one look.
SET_AND_NAMED = """([sel, text, rid, rf]) => document.querySelector(sel + ' .setpoint-big').textContent === text
  && (rf === null || roomThermostat(state.rooms.find((r) => r.id === rid)).rf_address === rf)"""

# What each case shows: the setpoint, the eco and comfort temperatures,
# the mark (None for none).
CASES = [
    ("off", 4.5, 17, 21, None),
    ("eco", 17, 17, 21, "eco"),
    ("comfort", 21, 17, 21, "comfort"),
    ("both", 19, 19, 19, "eco-comfort"),
    ("none", 20, 17, 21, None),
    ("highest", 30.5, 17, 21, None),
]
SYMBOL = {"eco": "#icon-eco", "comfort": "#icon-comfort", "eco-comfort": "#icon-eco-comfort"}
# The configuration's own words (cfg.eco, cfg.comfort).
LABEL = {
    "en": {"eco": "Eco temperature", "comfort": "Comfort temperature", "eco-comfort": "Eco and comfort temperature"},
    "de": {"eco": "Absenktemperatur", "comfort": "Komforttemperatur", "eco-comfort": "Absenk- und Komforttemperatur"},
}
VAL_SET = {"en": "set", "de": "Soll"}
MEMBERS = {"en": "Thermostats differ:", "de": "Thermostate weichen ab:"}
UNIT = {"en": "°C", "de": "°C"}
PUT = "**/rooms/*/setpoint"
PENDING = '{"ok": true, "discarded": false, "confirmation": "pending"}'
DISCARDED = '{"ok": true, "discarded": true}'


def num(v, lang):
    """A temperature with one digit in the language, "21.0", "21,0"."""
    s = "%.1f" % v
    return s.replace(".", ",") if lang == "de" else s


def shown(v, lang, unit):
    """A setpoint as the page writes it: OFF for 4.5 °C, else the number with unit."""
    return "OFF" if v == 4.5 else num(v, lang) + unit


def set_cell(row, lang):
    """A device row's Set cell, by its label alone (not Offset's)."""
    i = row.evaluate("(r, k) => [...r.querySelectorAll('.device-vals .cell')].findIndex((c) => c.querySelector('.k')?.textContent === k)", VAL_SET[lang])
    assert i >= 0, "the row has a Set cell"
    return row.locator(".device-vals .cell").nth(i)


# The preset marks in a place, each one's preset (by its class), label,
# tooltip, role and symbol.
MARKS = """(w) => [...w.querySelectorAll('.preset-mark')].map((m) => ({
  preset: [...m.classList].filter((c) => c.startsWith('preset-') && c !== 'preset-mark').map((c) => c.slice('preset-'.length))[0] || null,
  label: m.getAttribute('aria-label'), title: m.getAttribute('title'), role: m.getAttribute('role'),
  symbol: m.querySelector('use')?.getAttribute('href') ?? null }))"""

# The preset of the one mark in a device row's Set cell, the cell found by
# its label (not Offset's), in one look; null without the row, the cell
# or one mark in it.
ROW_SET_MARK = """([sel, k]) => {
  const row = document.querySelector(sel);
  const cell = row && [...row.querySelectorAll('.device-vals .cell')].find((c) => c.querySelector('.k')?.textContent === k);
  const marks = cell ? cell.querySelectorAll('.preset-mark') : [];
  const cls = marks.length === 1 ? [...marks[0].classList].find((c) => c.startsWith('preset-') && c !== 'preset-mark') : null;
  return cls ? cls.slice('preset-'.length) : null;
}"""


def mark_of(where):
    """The preset mark in a place, read in one look, so that a redraw
    between two looks cannot count it twice: its preset (by its class),
    its label, its tooltip and its symbol; None where there is none."""
    marks = where.evaluate(MARKS)
    if not marks:
        return None
    assert len(marks) == 1, f"one mark: {marks}"
    return marks[0]


# A case of test_marks_and_off in one look: the tile's SET and the marks
# of its setpoint row; the value, unit and marks of the device row's Set
# cell, found by its label (not Offset's), and its gauges; null without
# the cell.
CASE_LOOK = """([sel, rf, k]) => {
  const marksOf = %s;
  const tile = document.querySelector(sel);
  const row = tile && tile.querySelector('.device[data-rf="' + rf + '"]');
  const cell = row && [...row.querySelectorAll('.device-vals .cell')].find((c) => c.querySelector('.k')?.textContent === k);
  if (!cell) return null;
  return { set: tile.querySelector('.setpoint-big').textContent, marks: marksOf(tile.querySelector('.setpoint-row')),
    num: cell.querySelector('.num').innerText, unit: cell.querySelector('.u').innerText, rowMarks: marksOf(cell),
    gauge: cell.querySelectorAll("use[href='#icon-set']").length };
}""" % MARKS


def stroke(locator):
    """An icon's computed stroke colour."""
    return locator.evaluate("(e) => getComputedStyle(e).stroke")


def box(page, sel, within):
    """An element's box, x, y, width and height, x and y from the top
    left of the element within it: what moves the whole of that does not
    move it, such as the header's line of radiograms pending above the
    tiles, which comes and goes with the Cube's free memory slots."""
    return page.evaluate("""([s, w]) => {
      const r = document.querySelector(s).getBoundingClientRect(), o = document.querySelector(w).getBoundingClientRect();
      return [r.x - o.x, r.y - o.y, r.width, r.height];
    }""", [sel, within])


@pytest.mark.cases("1100-en")
def test_entries_carry_presets(page, case):
    """Every reported thermostat's entry carries its comfort and eco
    temperatures, those of its detail's configuration (D13-A); a window
    contact carries none."""
    got = page.evaluate("""async () => {
      const out = [];
      for (const r of state.rooms) {
        for (const d of r.devices) {
          if (!isThermostat(d)) {
            out.push({ rf: d.rf_address, thermostat: false, eco: d.eco_temp, comfort: d.comfort_temp });
            continue;
          }
          const x = await deviceDetail(d);
          const cfg = x.thermostat || x.wall_thermostat || {};
          out.push({ rf: d.rf_address, thermostat: true, eco: d.eco_temp, comfort: d.comfort_temp, want: { eco: cfg.eco_temp, comfort: cfg.comfort_temp } });
        }
      }
      return out;
    }""")
    ths = [d for d in got if d["thermostat"]]
    assert ths, "the fixture has thermostats"
    for d in ths:
        assert d["eco"] is not None and d["comfort"] is not None, f"{d['rf']} carries both: {d}"
        assert d["eco"] == d["want"]["eco"] and d["comfort"] == d["want"]["comfort"], f"{d['rf']}: as its detail: {d}"
    for d in got:
        if not d["thermostat"]:
            assert d["eco"] is None and d["comfort"] is None, f"{d['rf']}: no temperatures: {d}"


@pytest.mark.cases("1100-en")
def test_rooms_name_their_thermostat(page, case):
    """Every room with a thermostat names the one that stands for it
    (thermostat, the first of its one order, D19-A), a thermostat among
    its devices, which roomThermostat takes, and carries its eco and
    comfort temperatures, those of its configuration's answer; a room
    without one names none and carries none."""
    got = page.evaluate("""async () => {
      const out = [];
      for (const r of state.rooms) {
        const cfg = await api("GET", base() + "/rooms/" + r.id + "/config");
        const th = roomThermostat(r);
        out.push({ id: r.id, named: r.thermostat, th: th && th.rf_address, among: r.devices.some((d) => d.rf_address === r.thermostat && isThermostat(d)),
          eco: r.eco_temp, comfort: r.comfort_temp, any: r.devices.some(isThermostat), want: { eco: cfg.eco_temp, comfort: cfg.comfort_temp } });
      }
      return out;
    }""")
    assert any(r["any"] for r in got), "the fixture has rooms with thermostats"
    for r in got:
        if not r["any"]:
            assert r["named"] is None and r["eco"] is None and r["comfort"] is None, f"room {r['id']}: none: {r}"
            continue
        assert r["named"] and r["among"] and r["th"] == r["named"], f"room {r['id']}: its thermostat named and taken: {r}"
        assert r["eco"] == r["want"]["eco"] and r["comfort"] == r["want"]["comfort"], f"room {r['id']}: its configuration's: {r}"


def test_room_thermostat_follows_the_entry(page, case, reads):
    """Where the room's virtual wall thermostat names none
    (sources.setpoint, as from a server before it), the tile's SET is the
    setpoint of the thermostat the room's entry names (roomThermostat),
    whichever it names; an entry without it either takes the wall
    thermostat. All told to the rooms' read as well (Told), each check
    of SET and the thermostat named in one look."""
    told = Told(page, reads, two=True)
    rid, (a, b) = told.rid, told.room["devices"][:2]
    sel = '.room[data-id="%s"]' % rid
    told.set({a["rf"]: {"setpoint": 21}, b["rf"]: {"setpoint": 18}})
    lang = case.lang
    shows = lambda v, rf=None: "(%s)(%s)" % (SET_AND_NAMED, json.dumps([sel, shown(v, lang, "°"), rid, rf]))
    named = lambda: page.evaluate("(rid) => roomThermostat(state.rooms.find((r) => r.id === rid)).rf_address", rid)
    try:
        told.vwt_setpoint(None)
        told.thermostat(a["rf"])
        assert wait_until(page, shows(21, a["rf"]), 5), f"the first named: {named()}"
        told.thermostat(b["rf"])
        assert wait_until(page, shows(18, b["rf"]), 5), f"the other named: {named()}"
        wall = told.wall(23, 16, 24)
        assert wait_until(page, shows(23, wall), 5), f"a wall thermostat named: {named()}"
        told.vwt_setpoint(None)  # the wall thermostat named by the entry alone
        told.thermostat(b["rf"])
        assert wait_until(page, shows(18), 5), "the radiator named, the wall thermostat beside it"
        told.thermostat(None)
        assert wait_until(page, shows(23, wall), 5), f"none named: the wall thermostat: {named()}"
    finally:
        told.close()


def test_set_follows_the_virtual_wall_thermostats_events(page, case, reads):
    """The tile's SET follows the thermostat whose setpoint the room's
    virtual wall thermostat shows (its sources.setpoint), which its state
    events bring, without a read of the rooms (D19-A): a member changed at
    its device moves the room's state to another, as a boost's end may,
    and SET shows that one's setpoint at once, ahead of the room's entry
    (thermostat), which names it only at the next read. The events are
    dispatched in the page, their sources told to the rooms' read as well
    (Told); the reads keep starts after the server's own events are left
    out of the count (window.__uiTest.kept)."""
    told = Told(page, reads, two=True)
    rid, (a, b) = told.rid, told.room["devices"][:2]
    sel = '.room[data-id="%s"]' % rid
    told.set({a["rf"]: {"setpoint": 21}, b["rf"]: {"setpoint": 18}})
    shows = lambda v: "document.querySelector(%r).textContent === %r" % (sel + " .setpoint-big", shown(v, case.lang, "°"))
    loads = []
    kept = told.kept()
    page.on("request", lambda r: loads.append(r.url) if r.method == "GET" and "/rooms?devices=all" in r.url else None)
    try:
        told.thermostat(a["rf"])
        told.vwt_setpoint([a["rf"]])
        assert wait_until(page, shows(21), 5), "SET the room's thermostat's 21"
        told.vwt_setpoint([b["rf"]])
        assert wait_until(page, shows(18), 5), "SET the one the virtual wall thermostat names now, ahead of the entry"
        assert page.evaluate("(rid) => state.rooms.find((r) => r.id === rid).thermostat", rid) == a["rf"], "the entry as it was"
        told.vwt_setpoint([a["rf"], b["rf"]])
        assert wait_until(page, shows(21), 5), "and back, the first of them"
        page.wait_for_timeout(300)
        assert len(loads) <= told.kept() - kept, f"no read of the rooms but keep's: {loads}, keep's {told.kept() - kept}"
    finally:
        told.close()


def test_the_rooms_presets_over_a_wall_thermostats(page, case, reads):
    """Where the room's thermostat is a wall thermostat whose own eco and
    comfort differ from most of the room's (D19-A): the tile marks the
    setpoint by the room's, More shows and sends the room's, with no
    request for a detail, and the members' line marks each by its own
    (D11-A). All told to the rooms' read as well (Told): a read of the
    rooms, the page's own or after a reconnect, gives the server's eco 15
    and comfort 22 of the room otherwise; each check waits, in one
    look."""
    told = Told(page, reads)
    rid, th = told.rid, told.room["devices"][0]
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    lang = case.lang
    try:
        wall = told.wall(17, 16, 23)
        told.set({th["rf"]: {"setpoint": 17, "eco": 17, "comfort": 21}})
        told.presets(17, 21)
        assert wait_until(page, "document.querySelector(%r).textContent === %r" % (sel + " .setpoint-big", shown(17, lang, "°")), 5), "SET the wall thermostat's 17"
        assert wait_until(page, "!!document.querySelector(%r)" % (sel + " .setpoint-row .preset-mark.preset-eco"), 5), \
            "17 is the room's eco: marked, though not the wall thermostat's (16)"
        told.set({wall: {"setpoint": 16}})
        assert wait_until(page, "document.querySelector(%r).textContent === %r && !document.querySelector(%r)"
                          % (sel + " .setpoint-big", shown(16, lang, "°"), sel + " .setpoint-row .preset-mark"), 5), \
            "SET 16, the wall thermostat's eco, not the room's: no mark"
        want = {th["rf"]: "eco", wall: "eco"}
        ok, line = settled(page, lambda: members_line(tile), lambda m: m is not None and {rf: p for rf, _, _, p in m["members"]} == want)
        assert ok, f"the members' line while 16 and 17 differ, each member by its own eco, 17 and 16: {line}"

        asked = []
        page.on("request", lambda r: asked.append(r.url) if r.method == "GET" and "/devices/" in r.url else None)
        tile.locator(".more-sum").click()
        assert wait_until(page, "document.querySelector(%r).textContent === %r && document.querySelector(%r).textContent === %r"
                          % (sel + " .menu-eco .more-temp", num(17, lang) + " °C", sel + " .menu-comfort .more-temp", num(21, lang) + " °C"), 5), \
            "More's Eco the room's 17, not 16, its Comfort the room's 21, not 23"
        held, sent = [], []
        hold(page, PUT, "PUT", held, sent)
        try:
            tile.locator(".more-menu .menu-comfort").click()
            assert wait_held(page, held), "Comfort sends"
            assert sent[-1]["temp"] == 21, f"the room's comfort sent: {sent}"
            answer_all(page, held, PENDING)
        finally:
            page.unroute(PUT)
        assert not asked, f"no detail asked for: {asked}"
    finally:
        told.close()


def test_marks_and_off(page, case, reads):
    """The tile's SET and a device row at OFF, at eco, at comfort, at
    comfort equal to eco, at none of them and at the highest: OFF
    without a unit; the preset's mark, labelled, in the tile's setpoint
    row and in the device row's Set, there in the place of its gauge;
    SET keeping its icon and label; SET's value box (AP RoomsPage v1.0,
    Block 3) and the chevrons in the setpoint row, the row as large, and
    the device row's icon slot in that row, where they are in every case;
    the stepped value marked as it is stepped onto eco, the chevrons
    staying put; the marks in the accent colour of the tile's and the
    row's other icons, in light and in dark; no horizontal scroll. Each
    place is measured in its row, which whatever is above the tile moves
    as a whole. The states and presets are told to the rooms' read as
    well (Told), and each case's values and marks waited for in one
    look."""
    told = Told(page, reads)
    rid, ths = told.rid, told.room["devices"]
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    lang = case.lang
    tile.locator(".room-name-btn").click()
    row = tile.locator('.device[data-rf="%s"]' % ths[0]["rf"])
    row.wait_for()
    sp_row = sel + " .setpoint-row"
    slots, geometry = [], []
    try:
        for name, sp, eco, comfort, preset in CASES:
            told.set({d["rf"]: {"setpoint": sp, "eco": eco, "comfort": comfort} for d in ths})
            told.presets(eco, comfort)  # every thermostat of the room at them: the room's
            want = shown(sp, lang, "°")
            mark = [] if preset is None else [{"preset": preset, "label": LABEL[lang][preset], "title": LABEL[lang][preset], "role": "img",
                                               "symbol": SYMBOL[preset]}]
            expect = {"set": want, "marks": mark, "num": shown(sp, lang, ""), "unit": "" if sp == 4.5 else UNIT[lang], "rowMarks": mark,
                      "gauge": 0 if preset else 1}
            ok, got = settled(page, lambda: page.evaluate(CASE_LOOK, [sel, ths[0]["rf"], VAL_SET[lang]]), lambda g: g == expect)
            assert ok, f"{name}: the tile's SET {want} and its mark, the row's value, unit and mark in the gauge's place: {got}, want {expect}"
            assert tile.locator(".temp-tag use[href='#icon-set']").count() == 1, f"{name}: SET keeps its icon"
            assert tile.locator(".temp-tag .temp-label").nth(1).inner_text() == ("SET" if lang == "en" else "SOLL"), f"{name}: and its label"
            geometry.append((box(page, sel + " .set-val", sp_row), box(page, sel + " .stepper", sp_row), box(page, sp_row, sp_row)))

            cell = set_cell(row, lang)
            slots.append(cell.evaluate("(c) => { const r = c.querySelector('svg.di').getBoundingClientRect(), o = c.closest('.device').getBoundingClientRect(); return { x: r.x - o.x, width: r.width }; }"))

            row_box = page.evaluate("(s) => { const e = document.querySelector(s); return [e.scrollWidth, e.clientWidth]; }", sel + " .setpoint-row")
            assert row_box[0] <= row_box[1], f"{name}: the setpoint row fits its tile: {row_box}"
            assert scroll_width(page) <= case.width, f"{name}: no horizontal scroll"

        for (name, *_), b in zip(CASES, slots):
            assert abs(b["x"] - slots[0]["x"]) < 0.5 and abs(b["width"] - slots[0]["width"]) < 0.5, \
                f"{name}: the row's icon slot stays where it is: {b}, {slots[0]}"
        for (name, *_), g in zip(CASES, geometry):
            for what, a, b in zip(("the value's box", "the chevrons", "the setpoint row"), g, geometry[0]):
                assert all(abs(x - y) < 0.5 for x, y in zip(a, b)), f"{name}: {what} stays where it is in the setpoint row: {a}, {geometry[0]}"

        # The stepped value: half a degree under the eco temperature, then
        # stepped onto it; held at its route, so that nothing is sent.
        told.set({d["rf"]: {"setpoint": 16.5, "eco": 17, "comfort": 21} for d in ths})
        told.presets(17, 21)
        assert wait_until(page, "document.querySelector(%r).textContent === %r && !document.querySelector(%r)"
                          % (sel + " .setpoint-big", shown(16.5, lang, "°"), sel + " .setpoint-row .preset-mark"), 5), "16.5 is no preset"
        before = box(page, sel + " .stepper", sp_row)
        held, sent = [], []
        hold(page, PUT, "PUT", held, sent)
        try:
            tile.hover()
            tile.locator(".step-plus").click()
            assert tile.locator(".setpoint-big.pending").inner_text() == num(17, lang) + "°", "the stepped value"
            m = mark_of(tile.locator(".setpoint-row"))
            assert m and m["preset"] == "eco", f"the stepped value at eco is marked: {m}"
            after = box(page, sel + " .stepper", sp_row)
            assert all(abs(x - y) < 0.5 for x, y in zip(after, before)), f"the chevrons stay put in the setpoint row as the step reaches eco: {after}, {before}"
            tile.locator(".pending-cancel").click()
            assert mark_of(tile.locator(".setpoint-row")) is None, "the setpoint again, no mark"
            told.set({d["rf"]: {"setpoint": 5, "eco": 17, "comfort": 21} for d in ths})
            assert wait_until(page, "document.querySelector(%r).textContent === %r" % (sel + " .setpoint-big", shown(5, lang, "°")), 5), "SET 5"
            tile.hover()
            tile.locator(".step-minus").click()
            assert tile.locator(".setpoint-big.pending").inner_text() == "OFF", "stepped down to OFF"
            tile.locator(".pending-cancel").click()
            assert not sent, f"nothing sent: {sent}"
        finally:
            page.unroute(PUT)

        # The marks' colour: the accent, as SET's icon and the row's
        # Actual, in light and in dark.
        told.set({d["rf"]: {"setpoint": 17, "eco": 17, "comfort": 21} for d in ths})
        assert wait_until(page, "!!document.querySelector(%r)" % (sel + " .setpoint-row .preset-mark"), 5), "the eco mark"
        was = page.evaluate("document.documentElement.dataset.theme || ''")
        colours = {}
        try:
            for theme in ("light", "dark"):
                page.evaluate("(t) => { document.documentElement.dataset.theme = t; }", theme)
                tile_mark = stroke(tile.locator(".setpoint-row .preset-mark svg"))
                assert tile_mark == stroke(tile.locator(".temp-tag svg.ti").nth(1)), f"{theme}: the tile's mark as SET's icon"
                row_mark = stroke(set_cell(row, lang).locator(".preset-mark svg"))
                actual_icon = row.locator(".device-vals .cell svg.di", has=page.locator("use[href='#icon-is']")).first
                assert row_mark == stroke(actual_icon), f"{theme}: the row's mark as Actual's icon"
                colours[theme] = tile_mark
            assert colours["light"] != colours["dark"], f"the accent of each: {colours}"
        finally:
            page.evaluate("(t) => { if (t) document.documentElement.dataset.theme = t; else delete document.documentElement.dataset.theme; }", was)
    finally:
        tile.locator(".room-name-btn").click()


# The short labels of IS and SET (AP RoomsPage v1.0, Block 4, D2-A) and
# their full terms, the text of its tip (D3-A); the tip's button is
# named by both, the label first (WCAG 2.5.3, Label in Name).
SHORT = {"en": ("IS", "SET"), "de": ("IST", "SOLL")}
FULL = {"en": ("measured temperature", "set temperature"), "de": ("gemessene Temperatur", "Solltemperatur")}

# A tip of the setpoint row by its place, [tile, i], 0 for IS and 1 for
# SET: whether its text is shown and its first letter a capital, the
# text, its button's label and state, whether the button names the text
# as what it controls and has the focus, the text's box inside the
# window.
TIP = """([s, i]) => {
  const tip = document.querySelectorAll(s + " .setpoint-row .tip")[i];
  const b = tip.querySelector(":scope > button"), text = tip.querySelector(":scope > .tip-text");
  const r = text.getBoundingClientRect();
  return {
    shown: getComputedStyle(text).display !== "none" && r.width > 0, text: text.textContent,
    capital: getComputedStyle(text, "::first-letter").textTransform === "uppercase",
    label: b.querySelector(".temp-label")?.textContent, expanded: b.getAttribute("aria-expanded"),
    controls: !!text.id && b.getAttribute("aria-controls") === text.id, focus: document.activeElement === b,
    inside: r.left >= 0 && r.right <= document.documentElement.clientWidth,
  };
}"""


def ax_name(page, selector):
    """The accessible name Chromium gives the element selector finds,
    as a screen reader is told it (its accessibility tree, through CDP)."""
    cdp = page.context.new_cdp_session(page)
    try:
        cdp.send("Accessibility.enable")
        root = cdp.send("DOM.getDocument", {"depth": 0})["root"]["nodeId"]
        node = cdp.send("DOM.querySelector", {"nodeId": root, "selector": selector})["nodeId"]
        nodes = cdp.send("Accessibility.getPartialAXTree", {"nodeId": node, "fetchRelatives": False})["nodes"]
        return nodes[0].get("name", {}).get("value")
    finally:
        cdp.detach()


# The widths the setpoint row is checked at, each with its tiles'
# columns: phones of 346 and 390 px, one column; 610 px, two columns of
# 281 px, nearly the narrowest tile there is (280 px, .tiles); 956 px,
# three columns of 297 px, as in the owner's screenshot.
FIT_WIDTHS = {346: 1, 390: 1, 610: 2, 956: 3}

# The setpoint row of a tile measured: the tile's inner edges, the row's
# content edges, each temperature's tag (its icon and label, whether
# shown, the label's whole width and the width it is shown in) and value
# (its text's box, its font size and the width its box reserves, the
# min-width of IS's number and of SET's box with the mark's room), SET's
# value box, the mark, the chevrons, the label each tag should show, the
# page's width and the tiles' columns.
FIT = """(s) => {
  const tile = document.querySelector(s), row = tile.querySelector(".setpoint-row");
  const rect = (e) => { const r = e.getBoundingClientRect(); return { l: r.left, r: r.right, t: r.top, b: r.bottom, w: r.width }; };
  const text = (e) => { const g = document.createRange(); g.selectNodeContents(e); return rect(g); };
  const shown = (e) => getComputedStyle(e).display !== "none" && getComputedStyle(e).visibility === "visible" && e.getBoundingClientRect().width > 0;
  const tr = tile.getBoundingClientRect(), ts = getComputedStyle(tile), rr = row.getBoundingClientRect(), rs = getComputedStyle(row);
  const mark = row.querySelector(".set-val > .preset-mark");
  return {
    tile: { l: tr.left + parseFloat(ts.borderLeftWidth), r: tr.right - parseFloat(ts.borderRightWidth) },
    content: { l: rr.left + parseFloat(rs.paddingLeft), r: rr.right - parseFloat(rs.paddingRight) },
    tags: [...row.querySelectorAll(".temp-tag")].map((g) => {
      const icon = g.querySelector("svg"), label = g.querySelector(".temp-label");
      return { icon: rect(icon), label: rect(label), text: label.textContent, shown: [g, icon, label].every(shown),
        whole: label.scrollWidth, room: label.clientWidth };
    }),
    vals: [row.querySelector(".temp-is"), row.querySelector(".set-val")].map((box) => {
      const v = box.matches(".temp-big") ? box : box.querySelector(".temp-big");
      return { text: text(v), size: parseFloat(getComputedStyle(v).fontSize), reserve: parseFloat(getComputedStyle(box).minWidth) };
    }),
    setBox: rect(row.querySelector(".set-val")), mark: mark && rect(mark), stepper: rect(row.querySelector(".stepper")),
    want: [t("room.is"), t("room.set")], page: document.documentElement.scrollWidth,
    cols: getComputedStyle(tile.parentElement).gridTemplateColumns.split(" ").length,
  };
}"""


def fit_problems(m, width):
    """What is wrong with a setpoint row measured by FIT: a tag hidden or
    with another label, a label cut (AP RoomsPage v1.0, Block 4: the
    short labels fit), anything outside the tile, chevrons away from its
    right edge, the parts of the row overlapping in their order (IS's tag
    and value, SET's tag and value with its mark, the chevrons), IS's text
    wider than the width its number reserves, SET's text with its mark
    wider than the width its box reserves, a mark that is not a
    superscript right after the value's text, a horizontal scroll."""
    out = []
    (is_tag, set_tag), (is_val, set_val) = m["tags"], m["vals"]
    order = []
    for name, tag, val, want in (("IS", is_tag, is_val, m["want"][0]), ("SET", set_tag, set_val, m["want"][1])):
        if tag["text"] != want:
            out.append(f"{name}'s label {tag['text']!r}, not {want!r}")
        if tag["whole"] > tag["room"]:
            out.append(f"{name}'s label {tag['text']!r} cut: {tag['whole']} px in {tag['room']}")
        if tag["shown"]:
            order.append((f"{name}'s tag", min(tag["icon"]["l"], tag["label"]["l"]), max(tag["icon"]["r"], tag["label"]["r"])))
        else:
            out.append(f"{name}'s icon or label hidden")
        end = m["mark"]["r"] if name == "SET" and m["mark"] else val["text"]["r"]
        order.append((f"{name}'s value", val["text"]["l"], end))
    order.append(("the chevrons", m["stepper"]["l"], m["stepper"]["r"]))
    for what, left, right in order:
        if left < m["tile"]["l"] - 0.5 or right > m["tile"]["r"] + 0.5:
            out.append(f"{what} leaves the tile: {left:.1f}…{right:.1f} of {m['tile']['l']:.1f}…{m['tile']['r']:.1f}")
    for (a, _, a_right), (b, b_left, _) in zip(order, order[1:]):
        if a_right > b_left + 0.5:
            out.append(f"{a} over {b}: {a_right:.1f} > {b_left:.1f}")
    if abs(m["stepper"]["r"] - m["content"]["r"]) > 0.5:
        out.append(f"the chevrons end at {m['stepper']['r']:.1f}, not at the row's right edge {m['content']['r']:.1f}")
    if is_val["text"]["w"] > is_val["reserve"] + 0.5:
        out.append(f"IS's value {is_val['text']['w']:.1f} px wide, more than its number reserves, {is_val['reserve']:.1f}")
    set_end = m["mark"]["r"] if m["mark"] else set_val["text"]["r"]
    if set_end - m["setBox"]["l"] > set_val["reserve"] + 0.5:
        out.append(f"SET's value with its mark {set_end - m['setBox']['l']:.1f} px wide, more than its box reserves, {set_val['reserve']:.1f}")
    if m["mark"]:
        gap = m["mark"]["l"] - set_val["text"]["r"]
        if gap < -0.5 or gap > 0.25 * set_val["size"]:
            out.append(f"the mark {gap:.1f} px after the value's text, not right after it")
        if (m["mark"]["t"] + m["mark"]["b"]) / 2 >= (set_val["text"]["t"] + set_val["text"]["b"]) / 2:
            out.append(f"the mark not raised: {m['mark']} beside {set_val['text']}")
    if m["page"] > width:
        out.append(f"a horizontal scroll: {m['page']} px")
    return out


@pytest.mark.cases("390-en")
def test_the_setpoint_row_fits_in_every_language(page, case):
    """At 346 and 390 px, one column of tiles, at 610 px, two, and at
    956 px, three (AP RoomsPage v1.0, Block 3), in each of the six
    languages, whose labels of IS and SET differ in length, in light and
    dark, at the widest value and at OFF, with a mark and without:
    everything of the setpoint row inside its tile, the chevrons at its
    right edge; the icons and labels of IS and SET shown, each label,
    the short ones of Block 4, whole, none cut with an ellipsis; the
    temperatures, their tags and the chevrons side by side without
    overlapping; IS's value within the width its number reserves, SET's
    with its mark within the width its box reserves; the mark a
    superscript right after the value's text; the mark moving neither
    the value nor the chevrons; no horizontal scroll; the tip of IS and
    of SET with its full term inside the window."""
    room = page.evaluate(ROOM, [None, False])
    rid, ths = room["id"], room["devices"]
    sel = '.room[data-id="%s"]' % rid
    size = page.viewport_size
    problems = []
    try:
        for lang in ("en", "de", "fr", "it", "nl", "pl"):
            page.evaluate("async (l) => { await changeLanguage(l); applyLanguage(); }", lang)
            for width, cols in FIT_WIDTHS.items():
                page.set_viewport_size({"width": width, "height": size["height"]})
                for theme in ("light", "dark"):
                    page.evaluate("(t) => { document.documentElement.dataset.theme = t; }", theme)
                    places = {}
                    for sp, eco in ((30.5, 17), (30.5, 30.5), (4.5, 17), (4.5, 4.5)):
                        page.evaluate(SET, [rid, {d["rf"]: {"setpoint": sp, "eco": eco, "comfort": 21} for d in ths}])
                        page.evaluate(PRESETS, [rid, eco, 21])
                        marked = eco == sp
                        assert wait_until(page, "!!document.querySelector(%r) === %s" % (sel + " .set-val .preset-mark", "true" if marked else "false"), 5), \
                            f"{lang}: the mark {marked} at {sp}"
                        m = page.evaluate(FIT, sel)
                        where = f"{width} px, {lang}, {theme}, {sp}, mark {marked}"
                        assert m["cols"] == cols, f"{where}: {cols} columns of tiles: {m['cols']}"
                        problems += [f"{where}: {p}" for p in fit_problems(m, width)]
                        places.setdefault(sp, set()).add((round(m["stepper"]["l"], 1), round(m["setBox"]["l"], 1), round(m["vals"][1]["text"]["l"], 1)))
                    for sp, xs in places.items():
                        if len(xs) != 1:
                            problems.append(f"{width} px, {lang}, {theme}, {sp}: the mark moves the chevrons or the value: {xs}")
                # each tip (Block 4) with its full term, inside the window
                for i, key in enumerate(("room.measured", "room.setTemp")):
                    page.locator(sel + " .setpoint-row .tip > button").nth(i).click()
                    tip = page.evaluate(TIP, [sel, i])
                    if not (tip["shown"] and tip["inside"] and tip["text"] == page.evaluate("(k) => t(k)", key)):
                        problems.append(f"{width} px, {lang}: the tip of {key} not shown with its term inside the window: {tip}")
                    page.keyboard.press("Escape")
        assert not problems, f"{len(problems)} problems, the first ones:\n" + "\n".join(problems[:40])
    finally:
        page.set_viewport_size(size)
        page.evaluate("async () => { await changeLanguage('en'); applyLanguage(); delete document.documentElement.dataset.theme; }")


def test_a_tap_on_is_or_set_shows_its_full_term(page, case):
    """IS and SET (AP RoomsPage v1.0, Block 4): each icon with its short
    label is a button, which a click opens a tip under, the full term in
    the page's language, its first letter a capital, inside the window;
    the button's name, as Chromium tells a screen reader, is its short
    label and then the full term, the tip closed and open; Escape closes
    the tip, the focus staying on its button; so does a click outside
    it; one tip is open at a time, and
    one with the page's menus: opening a tip closes an open menu, and a
    menu opened by a click, by the keyboard or by the page's code closes
    the tip; Tab reaches the buttons, Enter and Space open and close the
    tip, the focus staying on the button."""
    room = page.evaluate(ROOM, [None, False])
    sel = '.room[data-id="%s"]' % room["id"]
    tile = page.locator(sel)
    lang = case.lang
    buttons = tile.locator(".setpoint-row .tip > button")
    assert buttons.count() == 2, "IS and SET are each a tip's button"
    more = tile.locator("details.more-menu")
    dots = tile.locator(".tile-head > details.menu")

    def tip(i):
        return page.evaluate(TIP, [sel, i])

    def name(i, what):
        n = ax_name(page, sel + " .setpoint-row > .temp-block:nth-child(%d) .tip > button" % (i + 1))
        short, full = SHORT[lang][i], FULL[lang][i]
        # open, the box's first letter is drawn a capital, which Chromium's name takes over
        assert n and n.startswith(short) and full.lower() in n.lower(), \
            f"{short}: {what}, its name holds its short label first and its full term: {n!r}"

    def closed(i, what):
        t = tip(i)
        assert not t["shown"] and t["expanded"] == "false", f"{SHORT[lang][i]}: {what}: {t}"
        return t

    def opened(i, what):
        t = tip(i)
        assert t["shown"] and t["expanded"] == "true" and t["text"] == FULL[lang][i] and t["capital"] and t["inside"], \
            f"{SHORT[lang][i]}: {what}: {t}"
        return t

    try:
        for i in (0, 1):
            t = closed(i, "closed at first")
            assert t["label"] == SHORT[lang][i] and t["controls"], \
                f"{SHORT[lang][i]}: its short label shown, its tip what it controls: {t}"
            name(i, "closed")
            buttons.nth(i).click()
            opened(i, "a click shows the full term")
            name(i, "open")
            page.keyboard.press("Escape")
            t = closed(i, "Escape closes it")
            assert t["focus"], f"{SHORT[lang][i]}: the focus stays on its button after Escape"

        buttons.nth(0).click()
        opened(0, "open")
        buttons.nth(1).click()
        opened(1, "the second opens")
        closed(0, "the second closes the first")
        tile.locator(".temp-is").click()
        closed(1, "a click outside closes it")

        buttons.nth(0).click()
        opened(0, "open before More")
        tile.locator(".more-sum").click()
        assert more.get_attribute("open") is not None, "More opens"
        closed(0, "More opened by a click closes the tip")
        buttons.nth(0).click()
        opened(0, "the tip opens over More")
        assert more.get_attribute("open") is None, "the tip opening closes More"
        dots.locator("summary").focus()
        page.keyboard.press("Enter")
        assert dots.get_attribute("open") is not None, "⋯ opens by the keyboard"
        closed(0, "⋯ opened by the keyboard closes the tip")
        page.keyboard.press("Escape")
        buttons.nth(1).click()
        opened(1, "open before the page's code opens More")
        more.evaluate("(d) => { d.open = true; }")
        # the details' toggle event, which closes the tip, comes a task later
        assert wait_until(page, "document.querySelectorAll(%r)[1].getAttribute('aria-expanded') === 'false'" % (sel + " .setpoint-row .tip > button"), 5), \
            f"More opened by the page's code closes the tip: {tip(1)}"
        closed(1, "More opened by the page's code")
        page.keyboard.press("Escape")

        dots.locator("summary").focus()
        page.keyboard.press("Tab")
        assert tip(0)["focus"], "Tab reaches IS's button after ⋯"
        page.keyboard.press("Enter")
        t = opened(0, "Enter opens it")
        assert t["focus"], "the focus stays on IS's button"
        page.keyboard.press("Enter")
        t = closed(0, "Enter again closes it")
        assert t["focus"], "the focus stays on IS's button"
        page.keyboard.press("Tab")
        assert tip(1)["focus"], "Tab goes on to SET's button"
        page.keyboard.press("Space")
        t = opened(1, "Space opens it")
        assert t["focus"], "the focus stays on SET's button"
        page.keyboard.press("Space")
        t = closed(1, "Space again closes it")
        assert t["focus"], "the focus stays on SET's button"
    finally:
        page.keyboard.press("Escape")


def test_escape_closes_the_tip_alone(page, case):
    """An Escape while a tip is open (AP RoomsPage v1.0, Block 4) closes
    the tip and nothing else: the room's holiday form, open with a
    temperature typed into it, stays open with the temperature."""
    room = page.evaluate(ROOM, [None, False])
    sel = '.room[data-id="%s"]' % room["id"]
    tile = page.locator(sel)
    form = sel + " .vacation-panel"
    try:
        tile.locator(".more-sum").click()
        tile.locator(".menu-holiday").click()
        temp = tile.locator(".vacation-temp")
        temp.click()
        temp.press("Control+a")
        page.keyboard.type("23.5")
        tile.locator(".setpoint-row .tip > button").first.click()
        assert page.evaluate(TIP, [sel, 0])["shown"], "the tip opens beside the holiday form"
        page.keyboard.press("Escape")
        t = page.evaluate(TIP, [sel, 0])
        assert not t["shown"] and t["focus"], f"Escape closes the tip, the focus on its button: {t}"
        assert page.evaluate("(s) => !document.querySelector(s).classList.contains('hidden')", form), "the holiday form stays open"
        assert temp.input_value() == "23.5", f"with the temperature typed: {temp.input_value()!r}"
        page.keyboard.press("Escape")
        assert page.evaluate("(s) => document.querySelector(s).classList.contains('hidden')", form), "the next Escape closes the form"
    finally:
        page.keyboard.press("Escape")


# The controls of the modes' row that an open tip of the tile's setpoint
# row overlaps (AP RoomsPage v1.0, Block 4), each with whether the tip is
# what lies at the middle of the overlap (elementFromPoint), and the
# setpoint row's z-index.
OVER = """(s) => {
  const tile = document.querySelector(s), row = tile.querySelector(".setpoint-row");
  const box = row.querySelector(".tip-text:not(.hidden)"), b = box.getBoundingClientRect();
  const overlaps = [];
  for (const c of tile.querySelectorAll(".mode-row button, .mode-row summary")) {
    const r = c.getBoundingClientRect();
    const l = Math.max(b.left, r.left) + 1, rt = Math.min(b.right, r.right) - 1, tp = Math.max(b.top, r.top) + 1, bt = Math.min(b.bottom, r.bottom) - 1;
    if (r.width === 0 || l >= rt || tp >= bt) continue;
    const e = document.elementFromPoint((l + rt) / 2, (tp + bt) / 2);
    overlaps.push({ what: c.className, more: c.matches(".more-sum"), top: !!e && box.contains(e), at: e && (e.className.baseVal ?? e.className) });
  }
  return { z: getComputedStyle(row).zIndex, overlaps };
}"""


def test_an_open_tip_lies_over_the_modes_row(page, case):
    """The setpoint row is a layer of its own only while a tip in it is
    open (AP RoomsPage v1.0, Block 4), so that a popup of a row never
    stays under a later tile for good; an open tip whose term is long
    enough to reach over the modes' row under it, its More's summary
    among them, is what lies wherever it overlaps them."""
    room = page.evaluate(ROOM, [None, False])
    sel = '.room[data-id="%s"]' % room["id"]
    tile = page.locator(sel)
    assert tile.locator(".setpoint-row").evaluate("(r) => getComputedStyle(r).zIndex") == "auto", \
        "the setpoint row is no layer while no tip is open"
    # SET's term five times over, the tile drawn again by a state event
    # of its thermostat: its box wraps over the modes' row
    page.evaluate("(k) => { window.__term = I18N[i18n.lang][k]; I18N[i18n.lang][k] = Array(5).fill(t(k)).join(' '); }", "room.setTemp")
    page.evaluate(SET, [room["id"], {room["devices"][0]["rf"]: {"setpoint": 20}}])
    try:
        tile.locator(".setpoint-row .tip > button").nth(1).click()
        tile.locator(".mode-row").scroll_into_view_if_needed()
        m = page.evaluate(OVER, sel)
        assert m["overlaps"] and any(o["more"] for o in m["overlaps"]), f"the tip overlaps the modes' row, More among them: {m}"
        assert all(o["top"] for o in m["overlaps"]), f"the tip lies over the modes' row: {m}"
        page.keyboard.press("Escape")
        assert tile.locator(".setpoint-row").evaluate("(r) => getComputedStyle(r).zIndex") == "auto", "no layer once it is closed"
    finally:
        page.keyboard.press("Escape")
        page.evaluate("(k) => { I18N[i18n.lang][k] = window.__term; delete window.__term; }", "room.setTemp")
        page.evaluate(SET, [room["id"], {room["devices"][0]["rf"]: {"setpoint": 20}}])


# SET's chevron and list (AP RoomsPage v1.0, Block 6, D4-A, D5-A). The
# first room of the fake Cube's whose highest setpoint is max, or the
# last where last is given, its id: the recorded greeting has rooms with
# 23 and with 30.5 (ON), each with 4.5 (OFF) its lowest.
ROOM_OF_MAX = """([m, last]) => {
  const rs = state.rooms.filter((r) => r.max_setpoint === m && r.devices.some((d) => isThermostat(d) && d.reported));
  return rs[last ? rs.length - 1 : 0].id;
}"""

# The words of SET's chevron, and what its field takes for a room
# whose highest setpoint is 23 °C, and for one whose highest is 30.5 °C.
CHOOSE = {"en": "Choose the set temperature", "de": "Solltemperatur wählen"}
RANGE_23 = {"en": "only OFF or 5.0 to 23.0 °C", "de": "nur OFF oder 5,0 bis 23,0 °C"}
RANGE_30 = {"en": "only OFF or 5.0 to 30.0 °C", "de": "nur OFF oder 5,0 bis 30,0 °C"}

# SET's chevron in a tile: the box of its icon, of the value's last
# sign (the °, a letter of OFF), of the text before it, of the value and
# of the setpoint row; the value's text, the last sign, the chevron's
# name and whether it is shown.
CHEVRON = """(s) => {
  const big = document.querySelector(s + ' .setpoint-big'), last = big.querySelector('.set-last'), b = big.querySelector('.set-open');
  const rect = (r) => ({ l: r.left, r: r.right, t: r.top, b: r.bottom });
  const g = document.createRange();
  g.selectNodeContents(last.firstChild);
  const sign = rect(g.getBoundingClientRect());
  g.selectNodeContents(big.firstChild);
  return { icon: rect(b.querySelector('svg').getBoundingClientRect()), sign, head: rect(g.getBoundingClientRect()),
    value: rect(big.getBoundingClientRect()), row: rect(document.querySelector(s + ' .setpoint-row').getBoundingClientRect()),
    text: big.innerText, last: last.firstChild.textContent, name: b.getAttribute('aria-label'), shown: getComputedStyle(b).display !== 'none' };
}"""

# SET's chevron and list in a tile: the chevron's state, whether it has
# the focus; the list shown, its entries' texts, the marks of those
# marked by their values, the entry marked current, whether it is in the
# entries' view; where the focus is in the list (an entry's value, or
# the field), what the field holds, the keypad it asks for, whether it
# is marked as refused, its message and whether it names it; the list's
# box, the value's top, the window's width and height, the bottom of the
# page's header pinned at the window's top, whether the field is what
# lies at its own middle (not under the header), the entries' height,
# the page's height and scroll. The chevron is off where it says so
# (aria-disabled), as it keeps the focus while a write runs.
LIST = """(s) => {
  const tile = document.querySelector(s), b = tile.querySelector('.set-open'), pop = tile.querySelector('.set-pop');
  const items = pop.querySelector('.set-items'), field = pop.querySelector('.set-field'), msg = pop.querySelector('.set-msg');
  const entries = items ? [...items.querySelectorAll('.set-item')] : [];
  const cur = entries.find((e) => e.getAttribute('aria-current') === 'true');
  const a = document.activeElement, r = pop.getBoundingClientRect();
  const inView = (e) => { const ir = items.getBoundingClientRect(), er = e.getBoundingClientRect(); return er.top >= ir.top - 0.5 && er.bottom <= ir.bottom + 0.5; };
  return {
    shown: getComputedStyle(pop).display !== 'none', expanded: b.getAttribute('aria-expanded'), btnFocus: a === b,
    disabled: b.disabled || b.getAttribute('aria-disabled') === 'true',
    entries: entries.map((e) => e.textContent),
    marks: Object.fromEntries(entries.filter((e) => e.querySelector('.preset-mark')).map((e) => [e.dataset.value,
      [...e.querySelector('.preset-mark').classList].find((c) => c.startsWith('preset-') && c !== 'preset-mark').slice(7)])),
    current: cur ? cur.dataset.value : null, inView: !!cur && inView(cur),
    focus: pop.contains(a) ? (a === field ? 'field' : a.dataset.value) : null, focusInView: pop.contains(a) && a !== field && inView(a),
    field: field ? field.value : null, mode: field ? field.inputMode : null, invalid: field ? field.getAttribute('aria-invalid') : null,
    message: msg ? msg.textContent : null,
    described: !!field && !!msg && field.getAttribute('aria-describedby') === msg.id,
    box: { l: r.left, r: r.right, t: r.top, b: r.bottom }, valueTop: tile.querySelector('.setpoint-big').getBoundingClientRect().top,
    vw: document.documentElement.clientWidth, vh: innerHeight, h: document.documentElement.scrollHeight, y: scrollY,
    pinned: Math.max(0, document.querySelector('header.top').getBoundingClientRect().bottom),
    fieldSeen: !!field && (() => { const f = field.getBoundingClientRect(); return document.elementFromPoint((f.left + f.right) / 2, (f.top + f.bottom) / 2) === field; })(),
    itemsH: items ? items.getBoundingClientRect().height : 0,
  };
}"""

# What SET's open list in a tile overlaps: each control of the tiles
# outside the list (a button or a menu's summary) whose box meets the
# list's within the window, whether it is of the tile's modes' row or of
# another tile, and whether the list is what lies at the middle of the
# overlap (elementFromPoint); and the setpoint row's z-index.
COVER = """(s) => {
  const tile = document.querySelector(s), pop = tile.querySelector('.set-pop'), b = pop.getBoundingClientRect();
  const overlaps = [];
  for (const c of document.querySelectorAll('.tile button, .tile summary')) {
    if (pop.contains(c)) continue;
    const r = c.getBoundingClientRect();
    const l = Math.max(b.left, r.left) + 1, rt = Math.min(b.right, r.right) - 1;
    const tp = Math.max(b.top, r.top) + 1, bt = Math.min(b.bottom, r.bottom, innerHeight) - 1;
    if (r.width === 0 || l >= rt || tp >= bt) continue;
    const e = document.elementFromPoint((l + rt) / 2, (tp + bt) / 2);
    overlaps.push({ what: c.className, modes: tile.contains(c) && !!c.closest('.mode-row'), other: !tile.contains(c), top: !!e && pop.contains(e) });
  }
  return { z: getComputedStyle(tile.querySelector('.setpoint-row')).zIndex, overlaps };
}"""


def set_list(page, sel):
    """SET's chevron and list in the tile sel, as LIST reads them."""
    return page.evaluate(LIST, sel)


def armed(sel):
    """An expression true while SET's chevron in the tile sel is on."""
    return "document.querySelector(%r).getAttribute('aria-disabled') !== 'true'" % (sel + " .set-open")


def inside_window(m):
    """Whether SET's list, as LIST reads it, lies in the window: 16 px or
    more within its sides, its top 16 px or more under the page's header
    pinned there, its field seen (not under the header), and its bottom
    16 px or more above the window's, but where its entries are at their
    least, 160 px, in a window where the room above and below is short."""
    b = m["box"]
    return (b["l"] >= 16 - 0.5 and b["r"] <= m["vw"] - 16 + 0.5 and b["t"] >= max(16, m["pinned"] + 16) - 0.5 and m["fieldSeen"]
            and (b["b"] <= m["vh"] - 16 + 0.5 or abs(m["itemsH"] - 160) < 1))


def set_room(page, reads, lang, max_setpoint, setpoint, eco=17, comfort=21, last=False, two=False):
    """The first room of the highest setpoint given, or the last, at
    setpoint, its presets eco and comfort, in the page and told to the
    rooms' read as well, its thermostats kept (Told, AP ToldData), with a
    twin beside its thermostat where two is given and it has one: its
    Told, its tile's selector and the tile. The test closes the Told at
    its end."""
    told = Told(page, reads, page.evaluate(ROOM_OF_MAX, [max_setpoint, last]), two)
    told.set({d["rf"]: {"setpoint": setpoint, "eco": eco, "comfort": comfort} for d in told.room["devices"]})
    told.presets(eco, comfort)
    sel = '.room[data-id="%s"]' % told.rid
    assert wait_until(page, "document.querySelector(%r).textContent === %r" % (sel + " .setpoint-big", shown(setpoint, lang, "°")), 5), \
        f"the room shows {setpoint}"
    return told, sel, page.locator(sel)


def test_sets_chevron_stands_under_the_degree(page, case, reads):
    """SET's chevron (AP RoomsPage v1.0, Block 6) is a button named in
    the page's language under the value's last sign, centred on the °,
    not under the number before it, between the value and the setpoint
    row's end; at OFF under its last letter; the value's text stays
    "20.5°"; hidden for a user who may not set a temperature. The
    setpoints are told to the rooms' read as well (Told), each value and
    its chevron measured in one look once the value shows."""
    told, sel, tile = set_room(page, reads, case.lang, 23, 20.5)
    try:
        for sp, last in ((20.5, "°"), (4.5, "F")):
            told.set({d["rf"]: {"setpoint": sp} for d in told.room["devices"]})
            ok, m = settled(page, lambda: page.evaluate(CHEVRON, sel), lambda m: m["text"] == shown(sp, case.lang, "°"))
            assert ok, f"{sp} shown: {m}"
            assert m["shown"] and m["name"] == CHOOSE[case.lang], f"{sp}: the chevron is a button named {CHOOSE[case.lang]!r}: {m}"
            assert m["text"] == shown(sp, case.lang, "°") and m["last"] == last, f"{sp}: the value's text as before, {last!r} its last sign: {m}"
            centre, sign = (m["icon"]["l"] + m["icon"]["r"]) / 2, (m["sign"]["l"] + m["sign"]["r"]) / 2
            assert abs(centre - sign) <= 1.5, f"{sp}: the chevron centred under {last!r}: {centre:.1f}, {sign:.1f}"
            assert centre > m["head"]["r"], f"{sp}: the chevron right of the number before {last!r}: {centre:.1f}, {m['head']}"
            assert m["icon"]["t"] >= m["value"]["b"] - 0.5 and m["icon"]["b"] <= m["row"]["b"] + 0.5, \
                f"{sp}: the chevron under the value, within the setpoint row: {m}"
        page.evaluate("document.body.classList.add('no-setpoint')")
        try:
            assert not page.evaluate(CHEVRON, sel)["shown"], "no chevron for a user who may not set a temperature"
        finally:
            page.evaluate("document.body.classList.remove('no-setpoint')")
    finally:
        told.close()


def test_sets_list_runs_from_off_to_the_highest(page, case, reads):
    """A click on SET's chevron opens its list without moving the page: a
    field holding the value shown in the page's language, then OFF once,
    every half degree from 5.0 to the room's highest setpoint, 23.0 °C
    or, where the highest is 30.5 °C (ON), 30.0 °C, without ON; the eco
    temperature marked with the moon, the comfort temperature with the
    sun; the value shown marked, in the list's view and with the focus;
    the chevron says it is open. A value the list has not got, ON, is
    marked nowhere. The list lies inside the window, 16 px from each of
    its edges and under the page's header pinned at its top, its field
    seen, in windows under 700 px tall and in expert mode, whose header is
    taller, too; where the room above and below is short, it opens
    downward, its entries at 160 px at least. The page does not grow:
    with the chevron near the window's bottom, the list opens upward, over
    the value; in the middle and at the page's end too. Each room's
    setpoint and presets are told to the rooms' read as well (Told)."""
    size = page.viewport_size
    told = None
    try:
        # (where the chevron is, the room's highest, its setpoint, the last
        # room of that highest, the window's height with the room's device
        # list open for a longer page, or None for the page's end in the
        # window as it is; expert mode, whose header is taller; and where
        # the chevron's bottom is put, from the window's top, or from its
        # bottom where negative)
        for where, most, sp, last, height, expert, at in (
                ("80 px from the bottom", 23, 20.5, True, 560, False, -80),
                ("260 px from the top", 30.5, 22, False, 560, False, 260),
                ("80 px from the bottom", 30.5, 30.5, True, 560, False, -80),
                ("370 px from the top of a 600 px window", 23, 20.5, True, 600, False, 370),
                ("370 px from the top of a 680 px window, expert mode", 30.5, 22, True, 680, True, 370),
                ("at the page's end", 30.5, 21, True, None, False, None)):
            told, sel, tile = set_room(page, reads, case.lang, most, sp, last=last)
            page.evaluate("(on) => { state.settings.expert = on; applySettings(); }", expert)
            if height:
                page.set_viewport_size({"width": size["width"], "height": height})
                tile.locator(".room-name-btn").click()
                want_at = at if at > 0 else height + at
                for _ in range(2):  # the header compacts once scrolled: once more where it moved the chevron
                    y = page.evaluate("(s) => scrollY + document.querySelector(s).getBoundingClientRect().bottom", sel + " .set-open")
                    page.evaluate("(y) => window.scrollTo(0, y)", y - want_at)
                    page.wait_for_timeout(100)
                got_at = page.evaluate("(s) => document.querySelector(s).getBoundingClientRect().bottom", sel + " .set-open")
                assert abs(got_at - want_at) <= 10, f"{where}: the chevron's bottom at {got_at:.0f} px"
            else:
                page.evaluate("window.scrollTo(0, document.documentElement.scrollHeight)")
            before, height_before = page.evaluate("[scrollY, document.documentElement.scrollHeight]")
            tile.locator(".set-open").click()
            m = set_list(page, sel)
            assert m["shown"] and m["expanded"] == "true", f"{where}: the list opens: {m}"
            assert m["y"] == before, f"{where}: the page stays where it was: {m['y']}, {before}"
            want = ["OFF"] + [num(v / 2, case.lang) + "°" for v in range(10, int(min(most, 30) * 2) + 1)]
            assert m["entries"] == want, f"OFF, then 5.0 to {min(most, 30)}: {m['entries']}"
            assert m["entries"].count("OFF") == 1 and not any(e.startswith(num(30.5, case.lang)) for e in m["entries"]), "OFF once, no ON"
            assert m["marks"] == {"17": "eco", "21": "comfort"}, f"the presets' marks: {m['marks']}"
            if sp == 30.5:
                assert m["current"] is None, f"ON is in no entry: {m}"
                assert m["focus"] == "30" and m["focusInView"], f"the focus on the nearest entry, in view: {m}"
            else:
                assert m["current"] == "%g" % sp and m["inView"] and m["focus"] == "%g" % sp, f"{sp} marked, in view, the focus on it: {m}"
            assert m["field"] == shown(sp, case.lang, "") and m["mode"] == "decimal", f"the field holds {sp}, a decimal keypad's: {m}"
            assert inside_window(m), f"{where}: the list inside the window, 16 px from its edges: {m['box']}, {m['vw']} × {m['vh']}"
            assert m["h"] == height_before, f"{where}: the page does not grow: {m['h']}, {height_before}"
            if where.endswith("bottom"):
                assert m["box"]["b"] <= m["valueTop"] + 0.5, f"{where}: the list opens upward, over the value: {m['box']}, {m['valueTop']}"
            page.keyboard.press("Escape")
            assert not set_list(page, sel)["shown"], "Escape closes it"
            assert page.evaluate("[scrollY, document.documentElement.scrollHeight]") == [before, height_before], f"{where}: the page as it was once closed"
            if height:
                tile.locator(".room-name-btn").click()
            page.evaluate("() => { state.settings.expert = false; applySettings(); }")
            page.set_viewport_size(size)
        # in the narrowest window, kept inside it
        page.set_viewport_size({"width": 320, "height": size["height"]})
        tile.locator(".set-open").click()
        m = set_list(page, sel)
        assert m["shown"] and inside_window(m), f"at 320 px the list inside the window: {m['box']}, {m['vw']} × {m['vh']}"
    finally:
        page.keyboard.press("Escape")
        page.set_viewport_size(size)
        if told:
            told.close()


def test_a_choice_or_a_typed_value_sends_the_setpoint(page, case, reads):
    """A choice in SET's list, and a value typed into its field with a
    comma or a point and Enter, close the list and send the room's
    setpoint as the stepper sends it, manual at that temperature: Save
    turns until the answer, the value shown meanwhile, the chevron off but
    keeping the focus; a typed value is rounded to the half degree, 4.6 to
    4.5, OFF. The room's setpoint is told to the rooms' read as well
    (Told)."""
    told, sel, tile = set_room(page, reads, case.lang, 23, 20.5)
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        tile.locator(".set-open").click()
        tile.locator(".set-item[data-value='22']").click()
        assert wait_held(page, held), "the choice sends the room's setpoint"
        assert sent[-1] == {"mode": "manual", "temp": 22}, f"the stepper's body: {sent}"
        m = set_list(page, sel)
        assert not m["shown"] and m["disabled"] and m["btnFocus"], f"the list closed, the chevron off while it is sent, the focus on it: {m}"
        assert busy_state(page, sel + " .pending-save") == [True, True, "true"], "Save turns"
        assert tile.locator(".setpoint-big.pending").inner_text() == num(22, case.lang) + "°", "the value chosen shown"
        answer_all(page, held, '{"ok": true, "devices": []}')
        for typed, temp in (("21,5", 21.5), ("19.5", 19.5), ("4,6", 4.5)):
            assert wait_until(page, armed(sel), 5), "the chevron is back once answered"
            tile.locator(".set-open").click()
            field = tile.locator(".set-field")
            field.fill(typed)
            field.press("Enter")
            assert wait_held(page, held), f"{typed} and Enter send"
            assert sent[-1] == {"mode": "manual", "temp": temp}, f"{typed} sends {temp}: {sent}"
            assert not set_list(page, sel)["shown"], "the list closed"
            answer_all(page, held, '{"ok": true, "devices": []}')
        page.wait_for_timeout(300)
        assert len(sent) == 4, f"one PUT each: {sent}"
    finally:
        answer_all(page, held, '{"ok": true, "devices": []}')
        page.unroute(PUT)
        page.keyboard.press("Escape")
        told.close()


def test_the_value_set_already_sends_nothing(page, case, reads):
    """A choice of the value every thermostat of the room shows already
    in manual mode, a tap on the marked entry, Enter in the field as it
    opened, or the value typed again, closes the list and sends nothing,
    sparing the radio's duty cycle, the focus on the chevron; a stepped
    value waiting is dropped then, and not sent either. The same value is
    sent where the room is in auto or boost, where its thermostats
    differ, and where a write of the room's setpoint is not confirmed by
    its thermostats (unconfirmed after its deadline). Each case's states
    are told to the rooms' read as well (Told): the value set already is
    what the room shows, so a server's own state event of a thermostat
    must not change it."""
    told, sel, tile = set_room(page, reads, case.lang, 23, 20.5, two=True)
    rid = told.rid
    rfs = [d["rf"] for d in told.room["devices"]]  # two thermostats at least, a twin where it has one
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        for how in ("the marked entry", "Enter in the field", "the value typed again"):
            tile.locator(".set-open").click()
            if how == "the marked entry":
                tile.locator(".set-item[data-value='20.5']").click()
            else:
                field = tile.locator(".set-field")
                if how == "the value typed again":
                    field.fill(num(20.5, case.lang))
                field.press("Enter")
            page.wait_for_timeout(300)
            assert sent == [], f"{how}: nothing sent: {sent}"
            m = set_list(page, sel)
            assert not m["shown"] and m["btnFocus"], f"{how}: the list closed, the focus on the chevron: {m}"
        for how, states in (("in auto", {rf: {"setpoint": 20.5, "mode": "auto"} for rf in rfs}),
                            ("in boost", {rf: {"setpoint": 20.5, "mode": "boost"} for rf in rfs}),
                            ("the thermostats differing", {rfs[0]: {"setpoint": 20.5}, rfs[1]: {"setpoint": 21}}),
                            ("a write not confirmed", {rf: {"setpoint": 20.5, "confirmation": {"state": "unconfirmed", "fields": ["setpoint"]}} for rf in rfs})):
            # the room's thermostats at 20.5 in manual first, which ends the
            # wait of the write before (awaitMembers), then the case's states
            told.set({rf: {"setpoint": 20.5, "confirmation": None} for rf in rfs})
            assert wait_until(page, "!roomUI(state.rooms.find((r) => r.id === %d)).awaiting" % rid, 5), "no write waits"
            told.set(states)
            assert wait_until(page, armed(sel), 5), "the chevron is on"
            tile.locator(".set-open").click()
            tile.locator(".set-item[data-value='20.5']").click()
            assert wait_held(page, held), f"{how}: 20.5 is sent"
            assert sent[-1] == {"mode": "manual", "temp": 20.5}, f"{how}: the stepper's body: {sent}"
            answer_all(page, held, '{"ok": true, "devices": []}')
        # a stepped value waiting, the value held chosen: the stepped value
        # dropped, nothing sent, also when its time would have come
        told.set({rf: {"setpoint": 20.5, "confirmation": None} for rf in rfs})
        assert wait_until(page, "!roomUI(state.rooms.find((r) => r.id === %d)).awaiting" % rid, 5), "no write waits"
        tile.hover()
        tile.locator(".step-plus").click()
        assert tile.locator(".pending-row").is_visible(), "a stepped value waits"
        tile.locator(".set-open").click()
        tile.locator(".set-item[data-value='20.5']").click()
        assert wait_until(page, "document.querySelector(%r).classList.contains('hidden')" % (sel + " .pending-row"), 5), "the stepped value dropped"
        assert page.evaluate("(rid) => { const u = roomUI(state.rooms.find((r) => r.id === rid)); return u.pending === null && !u.timer; }", rid), \
            "no stepped value, no timer"
        page.wait_for_timeout(2500)  # the stepper's own timer would have sent by now
        assert len(sent) == 4, f"one PUT for each case that sends, none for the stepped value dropped: {sent}"
    finally:
        answer_all(page, held, '{"ok": true, "devices": []}')
        page.unroute(PUT)
        page.keyboard.press("Escape")
        told.close()


def test_a_value_outside_the_list_is_marked_and_not_sent(page, case, reads):
    """A value typed into SET's field that the list has not got, above
    the room's highest, under 5 °C but OFF, ON, or no number, is marked
    on the field (aria-invalid, a message that names what it takes,
    described by it) and not sent, the list staying open; typing again
    clears the mark. What is typed stays through a state event, whose
    setpoint the list then marks; the event's setpoint is told to the
    rooms' read as well (Told)."""
    told, sel, tile = set_room(page, reads, case.lang, 23, 20.5)
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        tile.locator(".set-open").click()
        field = tile.locator(".set-field")
        for typed in ("23,5", "4", "abc", "30.5"):
            field.fill(typed)
            field.press("Enter")
            m = set_list(page, sel)
            assert m["shown"] and m["invalid"] == "true" and m["message"] == RANGE_23[case.lang] and m["described"], f"{typed} is marked: {m}"
            field.press("Backspace")
            m = set_list(page, sel)
            assert m["invalid"] is None and m["message"] == "" and not m["described"], f"typing clears the mark: {m}"
        field.fill("22")
        told.set({d["rf"]: {"setpoint": 19} for d in told.room["devices"]})
        assert wait_until(page, "document.querySelector(%r)?.getAttribute('aria-current') === 'true'" % (sel + " .set-item[data-value='19']"), 5), \
            "the list marks the setpoint a state event brings"
        assert set_list(page, sel)["field"] == "22", "what is typed stays"
        page.keyboard.press("Escape")
        # ON, where the room's highest is 30.5 °C: not in the list, refused
        told, sel, tile = set_room(page, reads, case.lang, 30.5, 20.5)
        tile.locator(".set-open").click()
        tile.locator(".set-field").fill("30,5")
        tile.locator(".set-field").press("Enter")
        m = set_list(page, sel)
        assert m["shown"] and m["invalid"] == "true" and m["message"] == RANGE_30[case.lang], f"ON is marked: {m}"
        page.wait_for_timeout(300)
        assert sent == [], f"nothing sent: {sent}"
    finally:
        answer_all(page, held, '{"ok": true, "devices": []}')
        page.unroute(PUT)
        page.keyboard.press("Escape")
        told.close()


def test_escape_closes_the_list_alone(page, case, reads):
    """An Escape while SET's list is open closes the list and nothing
    else, the focus back on the chevron, from an entry and from the
    field: the room's holiday form, open with a temperature typed into
    it, stays open with the temperature; a click outside the list closes
    it too. The room's setpoint is told to the rooms' read as well
    (Told)."""
    told, sel, tile = set_room(page, reads, case.lang, 23, 20.5)
    form = sel + " .vacation-panel"
    try:
        tile.locator(".more-sum").click()
        tile.locator(".menu-holiday").click()
        temp = tile.locator(".vacation-temp")
        temp.click()
        temp.press("Control+a")
        page.keyboard.type("23.5")
        for where in ("entry", "field"):
            tile.locator(".set-open").click()
            if where == "field":
                tile.locator(".set-field").click()
            assert set_list(page, sel)["focus"] == ("20.5" if where == "entry" else "field"), f"the focus in the list's {where}"
            page.keyboard.press("Escape")
            m = set_list(page, sel)
            assert not m["shown"] and m["expanded"] == "false" and m["btnFocus"], f"Escape from the {where} closes the list, the focus on the chevron: {m}"
            assert page.evaluate("(s) => !document.querySelector(s).classList.contains('hidden')", form), "the holiday form stays open"
            assert temp.input_value() == "23.5", f"with the temperature typed: {temp.input_value()!r}"
        tile.locator(".set-open").click()
        tile.locator(".temp-is").click()
        assert not set_list(page, sel)["shown"], "a click outside closes it"
        page.keyboard.press("Escape")
        assert page.evaluate("(s) => document.querySelector(s).classList.contains('hidden')", form), "the next Escape closes the form"
    finally:
        page.keyboard.press("Escape")
        told.close()


def test_sets_list_by_keyboard(page, case, reads):
    """Tab reaches SET's chevron after SET's tip; Enter opens the list,
    the focus on the value shown; the arrow keys move between the
    entries, Home and End to the first and the last, each in the list's
    view and the page not moving, also where the page was scrolled under
    the open list so that its end lies below the window's bottom; up from
    OFF is the field, down from it the value shown. Enter on an entry
    sends it, the focus staying on the chevron, which does nothing while
    the write runs; Space opens the list again; Tab out of it closes it;
    Escape closes it, the focus back on the chevron. The room's setpoint
    is told to the rooms' read as well (Told)."""
    size = page.viewport_size
    told, sel, tile = set_room(page, reads, case.lang, 23, 20.5, last=True)
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        # the page longer than the window, the chevron 200 px from its top:
        # the list opens downward to near the window's bottom
        page.set_viewport_size({"width": size["width"], "height": 520})
        tile.locator(".room-name-btn").click()
        y = page.evaluate("(s) => { const b = document.querySelector(s).getBoundingClientRect(); return scrollY + b.bottom; }", sel + " .set-open")
        page.evaluate("(y) => window.scrollTo(0, y)", y - 200)
        tile.locator(".setpoint-row .tip > button").nth(1).focus()
        page.keyboard.press("Tab")
        assert set_list(page, sel)["btnFocus"], "Tab reaches the chevron after SET's tip"
        y = page.evaluate("scrollY")
        page.keyboard.press("Enter")
        m = set_list(page, sel)
        assert m["shown"] and m["focus"] == "20.5" and m["y"] == y, f"Enter opens it, the focus on the value shown, the page still: {m}"
        page.evaluate("window.scrollBy(0, -150)")
        y = page.evaluate("scrollY")
        assert set_list(page, sel)["box"]["b"] > m["vh"], "the page scrolled under the open list, its end below the window's bottom"
        for key, want in (("ArrowDown", "21"), ("ArrowDown", "21.5"), ("ArrowUp", "21"), ("End", "23"), ("Home", "4.5"), ("ArrowUp", "field"),
                          ("ArrowDown", "20.5"), ("ArrowUp", "20"), ("ArrowDown", "20.5"), ("End", "23"), ("ArrowUp", "22.5"), ("ArrowUp", "22"),
                          ("ArrowUp", "21.5"), ("ArrowUp", "21")):
            page.keyboard.press(key)
            m = set_list(page, sel)
            assert m["focus"] == want and (want == "field" or m["focusInView"]), f"{key} goes to {want}, in the list's view: {m}"
            assert m["y"] == y, f"{key}: the page stays where it was: {m['y']}, {y}"
        page.keyboard.press("Enter")
        assert wait_held(page, held), "Enter on an entry sends it"
        assert sent == [{"mode": "manual", "temp": 21}], f"21 sent: {sent}"
        m = set_list(page, sel)
        assert not m["shown"] and m["disabled"] and m["btnFocus"], f"the list closed, the chevron off with the focus on it: {m}"
        page.keyboard.press("Enter")
        assert not set_list(page, sel)["shown"], "the chevron does nothing while the write runs"
        answer_all(page, held, '{"ok": true, "devices": []}')
        assert wait_until(page, armed(sel), 5), "the chevron is back once answered"
        assert set_list(page, sel)["btnFocus"], "the focus still on the chevron"
        page.keyboard.press("Space")
        assert set_list(page, sel)["shown"], "Space opens it"
        page.keyboard.press("Tab")
        assert not set_list(page, sel)["shown"], "Tab out of the list closes it"
        tile.locator(".set-open").focus()
        page.keyboard.press("Enter")
        assert set_list(page, sel)["shown"], "Enter opens it again"
        page.keyboard.press("Escape")
        m = set_list(page, sel)
        assert not m["shown"] and m["btnFocus"], f"Escape closes it, the focus on the chevron: {m}"
    finally:
        answer_all(page, held, '{"ok": true, "devices": []}')
        page.unroute(PUT)
        page.keyboard.press("Escape")
        page.set_viewport_size(size)
        told.close()


def test_one_list_open_on_top_of_the_page(page, case, reads):
    """One list is open at a time, and one with the page's menus and
    tips: a menu (⋯) or a tip that opens closes it, and it closes an open
    menu (More) or tip as it opens, and another tile's list; open, the setpoint
    row is a layer of its own, so that the list lies over the tile's
    modes' row and over the next tile wherever it overlaps them; closed,
    the row is none. The room's setpoint is told to the rooms' read as
    well (Told)."""
    told, sel, tile = set_room(page, reads, case.lang, 23, 20.5)
    other = '.room[data-id="%s"]' % page.evaluate(ROOM_OF_MAX, [30.5, False])
    dots = tile.locator(".tile-head > details.menu")
    more = tile.locator("details.more-menu")
    # the tile near the window's top, so that the list opens downward,
    # leaving the tile's head and the IS tip free
    page.evaluate("(s) => window.scrollTo(0, scrollY + document.querySelector(s).getBoundingClientRect().top - 80)", sel)
    try:
        tile.locator(".set-open").click()
        dots.locator("summary").click()
        assert dots.get_attribute("open") is not None and not set_list(page, sel)["shown"], "⋯ opening closes the list"
        page.keyboard.press("Escape")
        tile.locator(".more-sum").click()
        assert more.get_attribute("open") is not None, "More opens"
        tile.locator(".set-open").click()
        assert more.get_attribute("open") is None and set_list(page, sel)["shown"], "the list opening closes More"
        tile.locator(".setpoint-row .tip > button").first.click()
        assert not set_list(page, sel)["shown"], "a tip opening closes the list"
        tile.locator(".set-open").click()
        assert tile.locator(".setpoint-row .tip > button").first.get_attribute("aria-expanded") == "false", "the list opening closes the tip"
        page.locator(other + " .set-open").click()
        assert set_list(page, other)["shown"] and not set_list(page, sel)["shown"], "another tile's list closes the first"
        page.keyboard.press("Escape")
        assert tile.locator(".setpoint-row").evaluate("(r) => getComputedStyle(r).zIndex") == "auto", "no layer while the list is closed"
        page.evaluate("(s) => window.scrollTo(0, scrollY + document.querySelector(s).getBoundingClientRect().top - 80)", sel)
        tile.locator(".set-open").click()
        m = page.evaluate(COVER, sel)
        assert m["z"] != "auto", f"the setpoint row a layer of its own while the list is open: {m['z']}"
        assert any(o["modes"] for o in m["overlaps"]) and any(o["other"] for o in m["overlaps"]), \
            f"the list overlaps the modes' row and the next tile: {m}"
        assert all(o["top"] for o in m["overlaps"]), f"the list lies over them: {m}"
    finally:
        page.keyboard.press("Escape")
        told.close()


def test_a_sign_out_closes_the_list_and_the_tip(page, case, reads):
    """A sign-out (MassimoParts.signedOut) closes an open SET list and an
    open tip (shared/popup.ts), so that the next Escape is no longer
    theirs and nothing typed into the list stays for the next user: it
    opens again from the value shown, told to the rooms' read as well
    (Told)."""
    told, sel, tile = set_room(page, reads, case.lang, 23, 20.5)
    try:
        tile.locator(".set-open").click()
        tile.locator(".set-field").fill("22")
        page.evaluate("MassimoParts.signedOut()")
        assert wait_until(page, "getComputedStyle(document.querySelector(%r)).display === 'none'" % (sel + " .set-pop"), 5), "the sign-out closes the list"
        tile.locator(".setpoint-row .tip > button").first.click()
        assert page.evaluate(TIP, [sel, 0])["shown"], "a tip opens"
        page.evaluate("MassimoParts.signedOut()")
        assert wait_until(page, "document.querySelector(%r).getAttribute('aria-expanded') === 'false'" % (sel + " .setpoint-row .tip > button"), 5), \
            "the sign-out closes the tip"
        tile.locator(".set-open").click()
        assert set_list(page, sel)["field"] == shown(20.5, case.lang, ""), "the list opens again from the value shown"
    finally:
        page.keyboard.press("Escape")
        told.close()


def test_more_shows_off_from_the_entry(page, case, reads):
    """More's Eco and Comfort come from the room's entry (D19-A), with no
    request for its thermostat's detail (D13-A), and show OFF for an eco
    temperature of 4.5 °C, as the devices' displays show it. The states
    and presets are told to the rooms' read as well (Told), More's two
    temperatures waited for in one look."""
    told = Told(page, reads)
    rid, ths = told.rid, told.room["devices"]
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    told.set({d["rf"]: {"setpoint": 20, "eco": 4.5, "comfort": 21} for d in ths})
    told.presets(4.5, 21)
    asked = []
    page.on("request", lambda r: asked.append(r.url) if r.method == "GET" and "/devices/" in r.url else None)
    page.evaluate("state.details = {}")
    tile.locator(".more-sum").click()
    assert wait_until(page, "document.querySelector(%r).textContent === 'OFF' && document.querySelector(%r).textContent === %r"
                      % (sel + " .menu-eco .more-temp", sel + " .menu-comfort .more-temp", num(21, case.lang) + " °C"), 5), \
        "eco at 4.5 °C is OFF, comfort as a temperature"
    page.keyboard.press("Escape")
    assert not asked, f"no detail asked for: {asked}"


def test_a_config_event_brings_new_presets(page, case, reads):
    """A config event carries the device's entry with its new eco and
    comfort temperatures: the row's mark follows it at once; one that says
    its presets changed (changed: temps) reads the rooms again, and the
    tile's mark follows the room's new ones (D19-A); one that does not
    reads nothing. The states, the presets and the event's eco are told
    to the rooms' read as well (Told); the reads keep starts after the
    server's own events are left out of the count (window.__uiTest.kept)."""
    told = Told(page, reads)
    rid, th = told.rid, told.room["devices"][0]
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    told.set({th["rf"]: {"setpoint": 18, "eco": 16, "comfort": 21}})
    told.presets(16, 21)
    assert wait_until(page, "document.querySelector(%r).textContent === %r && !document.querySelector(%r)"
                      % (sel + " .setpoint-big", shown(18, case.lang, "°"), sel + " .setpoint-row .preset-mark"), 5), "18 is no preset"
    tile.locator(".room-name-btn").click()
    row = tile.locator('.device[data-rf="%s"]' % th["rf"])
    row.wait_for()
    config = """([rid, rf, changed]) => {
      const r = state.rooms.find((x) => x.id === rid);
      const d = JSON.parse(JSON.stringify(r.devices.find((x) => x.rf_address === rf)));
      d.eco_temp = 18;
      const ev = { kind: "config", source: "device", level: "info", device: rf, serial: d.serial, time: new Date().toISOString(),
        changes: { eco_temp: { from: 16, to: 18 } }, state: d };
      if (changed) ev.changed = changed;
      state.events.dispatchEvent(new MessageEvent("config", { data: JSON.stringify(ev) }));
    }"""
    loads = []
    kept = told.kept()
    page.on("request", lambda r: loads.append(r.url) if r.method == "GET" and "/rooms?devices=all" in r.url else None)
    try:
        reads.device(th["rf"], eco_temp=18)  # the device's entry as the event brings it
        page.evaluate(config, [rid, th["rf"], None])
        assert wait_until(page, "document.querySelector(%r)?.classList.contains('preset-eco')" % (
            sel + ' .device[data-rf="%s"] .preset-mark' % th["rf"]), 5), "the row's eco mark at once"
        assert mark_of(tile.locator(".setpoint-row")) is None, "the tile's by the room's, which did not change"
        page.wait_for_timeout(300)
        assert len(loads) <= told.kept() - kept, f"no read of the rooms without changed but keep's: {loads}, keep's {told.kept() - kept}"

        def room_eco(rooms):
            for r in rooms or []:
                if r.get("id") == rid:
                    r["eco_temp"], r["comfort_temp"] = 18, 21
                    for d in r.get("devices") or []:
                        if d.get("rf_address") == th["rf"]:
                            d["eco_temp"] = 18
                            d.setdefault("state", {})["setpoint"] = 18
            return rooms
        reads.change(ROOMS, room_eco)  # after Told's, so that the room's new eco is the read's
        page.evaluate(config, [rid, th["rf"], ["temps"]])
        assert wait_until(page, "!!document.querySelector(%r)" % (sel + " .setpoint-row .preset-mark.preset-eco"), 5), \
            "the rooms read again: the tile's eco mark by the room's new eco"
        assert loads, "the rooms were read"
        # waited for, the Set cell found at each look: a redraw of the row
        # takes the mark until the rooms are read again
        assert wait_until(page, "(%s)(%s) === 'eco'" % (ROW_SET_MARK, json.dumps(['%s .device[data-rf="%s"]' % (sel, th["rf"]), VAL_SET[case.lang]])), 5), \
            "the row's eco mark"
    finally:
        told.close()
        tile.locator(".room-name-btn").click()


def test_a_thermostat_gained_or_lost_reads_the_rooms(page, case, reads):
    """A state event of a room's thermostat whose state the Cube gained or
    lost (changes.reported) reads the rooms again, since another may then
    stand for the room (thermostat, the dead passed over, D19-A): the
    tile's SET follows the one the server names now; a state event of
    another change reads nothing."""
    room = page.evaluate(ROOM, [None, True])
    rid, (a, b) = room["id"], room["devices"][:2]
    sel = '.room[data-id="%s"]' % rid
    loads = []
    page.on("request", lambda r: loads.append(r.url) if r.method == "GET" and "/rooms?devices=all" in r.url else None)
    event = """([rid, rf, changes]) => {
      const r = state.rooms.find((x) => x.id === rid);
      const d = JSON.parse(JSON.stringify(r.devices.find((x) => x.rf_address === rf)));
      state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({ kind: "state", source: "device", level: "info",
        device: rf, serial: d.serial, time: new Date().toISOString(), changes, state: d }) }));
    }"""
    try:
        page.evaluate(event, [rid, a["rf"], {"valve_pct": {"from": 0, "to": 10}}])
        page.wait_for_timeout(300)
        assert not loads, f"no read of the rooms for a valve: {loads}"

        def named_twin(rooms):
            # the server's answer, the twin the page has beside the room's
            # thermostat named, the first one's state gone
            for r in rooms or []:
                if r.get("id") == rid:
                    first = next(d for d in r["devices"] if d.get("rf_address") == a["rf"])
                    twin = json.loads(json.dumps(first))
                    twin.update({"rf_address": b["rf"], "serial": "MEQ0000999", "name": "Twin"})
                    twin["state"] = dict(twin.get("state") or {}, setpoint=23.5)  # a setpoint none of the fixture's has
                    first.update({"reported": False, "out_of_date": True, "last_known": first.pop("state", None)})
                    r["devices"] = [d for d in r["devices"] if d.get("rf_address") != b["rf"]] + [twin]
                    r["thermostat"] = b["rf"]
                    for v in r["devices"]:
                        if v.get("virtual"):
                            v.setdefault("sources", {})["setpoint"] = [b["rf"]]
            return rooms
        reads.change("**/api/v1/cubes/*/rooms?devices=all", named_twin)
        assert page.evaluate("document.querySelector(%r).textContent" % (sel + " .setpoint-big")) != shown(23.5, case.lang, "°"), "SET not the twin's yet"
        page.evaluate(event, [rid, a["rf"], {"reported": {"from": True, "to": False}}])
        assert wait_until(page, "document.querySelector(%r).textContent === %r" % (sel + " .setpoint-big", shown(23.5, case.lang, "°")), 5), \
            "the rooms read again: SET the twin's, which the server names now"
        assert loads, "the rooms were read"
        assert page.evaluate("(rid) => roomThermostat(state.rooms.find((r) => r.id === rid)).rf_address", rid) == b["rf"], "the room thermostat the one named"
    finally:
        reads.clear("**/api/v1/cubes/*/rooms?devices=all")
        page.evaluate("async () => { await loadRooms(); }")


def test_a_burst_of_reloads_reads_the_rooms_twice(page, case):
    """A burst of the events that read the rooms again (state events of a
    thermostat whose state the Cube gained or lost, as a reconnect brings
    one per thermostat): one read runs and one more waits behind it, which
    the rest of the burst shares (app.js loadRooms), so the rooms are read
    twice, the second after the last event, and the page shows the second
    read's rooms, not those of the first, answered first."""
    room = page.evaluate(ROOM, [None, False])
    rid, th = room["id"], room["devices"][0]
    sel = '.room[data-id="%s"]' % rid
    pattern = "**/api/v1/cubes/*/rooms?devices=all"
    event = """([rid, rf]) => {
      const r = state.rooms.find((x) => x.id === rid);
      const d = JSON.parse(JSON.stringify(r.devices.find((x) => x.rf_address === rf)));
      state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({ kind: "state", source: "device", level: "info",
        device: rf, serial: d.serial, time: new Date().toISOString(), changes: { reported: { from: false, to: true } }, state: d }) }));
    }"""

    def answer(route, name):
        """Answers a held read with the server's rooms, the room renamed."""
        rooms = route.fetch().json()
        for r in rooms:
            if r.get("id") == rid:
                r["name"] = name
        route.fulfill(status=200, content_type="application/json", body=json.dumps(rooms))

    held, sent = [], []
    hold(page, pattern, "GET", held, sent)
    try:
        for _ in range(5):
            page.evaluate(event, [rid, th["rf"]])
        assert wait_held(page, held), "the first read"
        page.wait_for_timeout(300)
        assert len(sent) == 1, f"one read while it runs: {sent}"
        answer(held.pop(0), "Stale")
        assert wait_held(page, held), "the read that waited"
        page.wait_for_timeout(300)
        assert len(sent) == 2, f"the burst read twice: {sent}"
        answer(held.pop(0), "Newest")
        assert wait_until(page, "document.querySelector(%r).textContent === 'Newest'" % (sel + " .room-name"), 5), \
            "the second read's rooms shown"
        page.wait_for_timeout(300)
        assert len(sent) == 2 and not held, f"no third read: {sent}"
    finally:
        for route in held:  # the reads a failure left held go to the server
            route.continue_()
        held.clear()
        page.unroute(pattern)
        page.evaluate("async () => { await loadRooms(); }")


def test_a_cube_change_reads_its_rooms_at_once(page, case):
    """The rooms of a Cube chosen while the previous Cube's read runs are
    read at once, not after it, and the previous read's late answer
    changes nothing (app.js loadRooms): the Cube's read held, another
    serial chosen, whose read the test answers with a room of its own."""
    serial = page.evaluate("state.serial")
    other = "KEQ0000990"
    held, sent = [], []
    pattern = "**/api/v1/cubes/%s/rooms?devices=all" % serial

    def answer_other(route):
        body = [{"id": 1, "name": "Other Cube's room", "ready": True, "devices": []}] if "/rooms" in route.request.url else []
        route.fulfill(status=200, content_type="application/json", body=json.dumps(body))
    hold(page, pattern, "GET", held, sent)
    page.route("**/api/v1/cubes/%s/**" % other, answer_other)
    try:
        page.evaluate("() => { loadRooms().catch(() => {}); }")
        assert wait_held(page, held), "the first Cube's read is on its way"
        page.evaluate("(s) => { state.serial = s; loadRooms().catch(() => {}); }", other)
        assert wait_until(page, "state.roomsOf === %r" % other, 5), "the other Cube's rooms read while the first's still runs"
        held.pop(0).continue_()
        page.wait_for_timeout(500)
        got = page.evaluate("[state.roomsOf, state.rooms.map((r) => r.name)]")
        assert got == [other, ["Other Cube's room"]], f"the first Cube's late answer changes nothing: {got}"
    finally:
        for route in held:
            route.continue_()
        held.clear()
        page.unroute(pattern)
        page.unroute("**/api/v1/cubes/%s/**" % other)
        page.evaluate("async (s) => { state.serial = s; await loadRooms(); }", serial)


def members_line(tile):
    """The members' line: shown or not, its label, each member as (radio
    address, name, setpoint, preset), and its whole text."""
    line = tile.locator(".members-line")
    if "hidden" in (line.get_attribute("class") or "").split():
        return None
    out = []
    for m in line.locator(".member").all():
        mark = m.locator(".preset-mark")
        preset = None
        if mark.count():
            preset = [c[len("preset-"):] for c in mark.get_attribute("class").split() if c.startswith("preset-") and c != "preset-mark"][0]
        out.append((m.get_attribute("data-rf"), m.locator(".member-name").inner_text(), m.locator(".member-set").inner_text(), preset))
    return {"label": line.locator(".members-label").inner_text(), "members": out, "text": line.evaluate("(e) => e.textContent")}


def line_shown(page, sel):
    return "!document.querySelector(%r).classList.contains('hidden')" % (sel + " .members-line")


def line_hidden(page, sel):
    return "document.querySelector(%r).classList.contains('hidden')" % (sel + " .members-line")


def answer_all(page, held, body, status=200):
    """Answers the held writes with a stub."""
    for route in held:
        route.fulfill(status=status, content_type="application/json", body=body)
    held.clear()


def awaiting(page, rid):
    """The room's tile's wait for its thermostats, None for none."""
    return page.evaluate("(rid) => roomUI(state.rooms.find((r) => r.id === rid)).awaiting", rid)


def test_members_line(page, case, reads):
    """While the room's thermostats show different setpoints, a line under
    the setpoint row names each with its setpoint, OFF included, and its
    own mark, " · " between them; it is the same element as they change;
    not while a write of the tile runs or waits for them to show it, a
    room command reaching one after the other; gone once they agree, and
    at once again where one is changed by hand after that. The states go
    into the page, the events under test, and into the rooms' read as
    well (Told): a read in the middle of the wait draws them as the page
    has them, and ends nothing they would not end (settleAwaiting)."""
    told = Told(page, reads, two=True)
    rid, ths = told.rid, told.room["devices"][:2]
    a, b = ths
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    lang = case.lang
    told.set({a["rf"]: {"setpoint": 21, "mode": "auto", "eco": 17, "comfort": 21},
              b["rf"]: {"setpoint": 21, "mode": "auto", "eco": 17, "comfort": 21}})
    assert wait_until(page, line_hidden(page, sel), 5), "no line while they agree"

    told.set({b["rf"]: {"setpoint": 4.5, "mode": "auto", "eco": 16, "comfort": 21}})
    assert wait_until(page, line_shown(page, sel), 5), "the line once they differ"
    want = [(a["rf"], a["name"], num(21, lang) + " °C", "comfort"), (b["rf"], b["name"], "OFF", None)]
    ok, line = settled(page, lambda: members_line(tile), lambda m: m is not None and m["label"] == MEMBERS[lang] and m["members"] == want)
    assert ok, line
    assert line["text"].count(" · ") == 1, f"a separator between the two: {line['text']!r}"
    el = page.evaluate_handle("(s) => document.querySelector(s)", sel + " .members-line")
    member = page.evaluate_handle("(s) => document.querySelector(s)", sel + ' .members-line .member[data-rf="%s"]' % b["rf"])
    told.set({b["rf"]: {"setpoint": 16, "mode": "auto", "eco": 16, "comfort": 21}})
    assert wait_until(page, "document.querySelector(%r).textContent.includes(%r)" % (sel + " .members-line", num(16, lang)), 5), "the line follows"
    ok, line = settled(page, lambda: members_line(tile), lambda m: m is not None and m["members"][1:] == [(b["rf"], b["name"], num(16, lang) + " °C", "eco")])
    assert ok, f"each by its own eco temperature: {line}"
    assert page.evaluate("([e, s]) => e === document.querySelector(s)", [el, sel + " .members-line"]), "the same line"
    assert page.evaluate("([e, s]) => e === document.querySelector(s)", [member, sel + ' .members-line .member[data-rf="%s"]' % b["rf"]]), \
        "the same member, updated in place"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the line"

    # A write of the tile: MANU, held, then answered; the room thermostat
    # takes it, then the other; the line stays away until they agree.
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        assert wait_until(page, "document.querySelector(%r).textContent === %r" % (sel + " .setpoint-big", shown(21, lang, "°")), 5), \
            "SET the room thermostat's 21, which MANU sends"
        tile.locator(".mode-manual").click()
        assert wait_held(page, held), "MANU sends"
        assert sent[-1] == {"mode": "manual", "temp": 21}, f"the room thermostat's setpoint: {sent}"
        assert members_line(tile) is None, "no line while the write runs"
        answer_all(page, held, PENDING)
        assert wait_until(page, "!document.querySelector(%r).classList.contains('busy')" % (sel + " .mode-manual"), 5), "the write answered"
        assert members_line(tile) is None, "no line while the thermostats do not show it yet"
        told.set({a["rf"]: {"setpoint": 21, "mode": "manual"}})
        page.wait_for_timeout(300)
        assert members_line(tile) is None, "nor while one shows it and the other not"
        told.set({b["rf"]: {"setpoint": 21, "mode": "manual"}})
        assert wait_until(page, "roomUI(state.rooms.find((r) => r.id === %d)).awaiting === null" % rid, 5), "the wait ends once both show it"
        assert wait_until(page, line_hidden(page, sel), 5), "they agree: no line"
        told.set({b["rf"]: {"setpoint": 18, "mode": "manual"}})
        assert wait_until(page, line_shown(page, sel), 5), "a change by hand after the write shows at once"
    finally:
        page.unroute(PUT)


def test_auto_into_auto_waits_for_the_setpoints(page, case, reads):
    """AUTO while every thermostat is in auto already: the wait does not
    end with the answer, though they all show the mode, nor at a state
    event of another kind (a valve report, a window contact) while they
    still agree on their setpoints from before, but once they show
    agreeing setpoints taken since, so that the line does not flicker as
    one takes its programme's setpoint before the other (D17-A)."""
    told = Told(page, reads, two=True)
    rid, (a, b) = told.rid, told.room["devices"][:2]
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    told.set({a["rf"]: {"setpoint": 17, "mode": "auto"}, b["rf"]: {"setpoint": 17, "mode": "auto"}})
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        tile.locator(".mode-auto").click()
        assert wait_held(page, held), "AUTO sends"
        assert sent[-1] == {"mode": "auto"}, sent
        answer_all(page, held, PENDING)
        assert wait_until(page, "!document.querySelector(%r).classList.contains('busy')" % (sel + " .mode-auto"), 5), "the write answered"
        assert (awaiting(page, rid) or {}).get("mode") == "auto", "the wait holds though both show auto"
        told.set({b["rf"]: {"setpoint": 17, "mode": "auto", "valve": 40}})
        told.window(False, False)
        page.wait_for_timeout(300)
        assert (awaiting(page, rid) or {}).get("mode") == "auto", "a valve report and a window contact before any takes the programme end nothing"
        told.set({a["rf"]: {"setpoint": 21, "mode": "auto"}})
        page.wait_for_timeout(300)
        assert members_line(tile) is None, "no line while one has taken its programme's setpoint and the other not"
        told.set({b["rf"]: {"setpoint": 21, "mode": "auto"}})
        assert wait_until(page, "roomUI(state.rooms.find((r) => r.id === %d)).awaiting === null" % rid, 5), "the wait ends once they agree"
        told.set({b["rf"]: {"setpoint": 19, "mode": "auto"}})
        assert wait_until(page, line_shown(page, sel), 5), "a later difference shows at once"
    finally:
        page.unroute(PUT)


def test_the_house_holds_the_line(page, case, reads):
    """A write of the House tile to the room, from its answer on, holds
    the room's line as the tile's own write does, until its thermostats
    show it."""
    told = Told(page, reads, two=True)
    rid, (a, b) = told.rid, told.room["devices"][:2]
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    told.set({a["rf"]: {"setpoint": 20, "mode": "manual"}, b["rf"]: {"setpoint": 20, "mode": "manual"}})
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        page.locator("#house-auto").click()
        page.locator("#house-rooms .check-all input").check()
        page.locator("#house-panel button[type=submit]").click()
        assert wait_held(page, held), "the House sends"
        for _ in range(40):
            answer_all(page, held, PENDING)
            if busy_state(page, "#house-panel button[type=submit]")[1] is False and not held:
                break
            page.wait_for_timeout(200)
        assert busy_state(page, "#house-panel button[type=submit]")[1] is False, "every room answered"
        assert (awaiting(page, rid) or {}).get("mode") == "auto", "the House's write waits for the room's thermostats"
        told.set({a["rf"]: {"setpoint": 17, "mode": "auto"}})
        page.wait_for_timeout(300)
        assert members_line(tile) is None, "no line while they take it one by one"
        told.set({b["rf"]: {"setpoint": 17, "mode": "auto"}})
        assert wait_until(page, "roomUI(state.rooms.find((r) => r.id === %d)).awaiting === null" % rid, 5), "the wait ends once they agree"
    finally:
        page.unroute(PUT)
        page.locator("#house-panel .form-cancel").click()


def test_a_write_not_taken_leaves_the_wait(page, case, watch, reads):
    """A write the Cube discarded or that failed replaces no wait: with
    none before it, the line shows at once after its answer; after a write
    the Cube took, the line stays away for that one, until its
    thermostats show it (D17-A)."""
    told = Told(page, reads, two=True)
    rid, (a, b) = told.rid, told.room["devices"][:2]
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    watch.expected.append(("/rooms/%d/setpoint" % rid, 502))  # the failed write's answer, on purpose
    told.set({a["rf"]: {"setpoint": 21, "mode": "auto"}, b["rf"]: {"setpoint": 17, "mode": "auto"}})
    assert wait_until(page, line_shown(page, sel), 5), "the line while they differ"
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        tile.locator(".mode-auto").click()
        assert wait_held(page, held), "AUTO sends"
        assert members_line(tile) is None, "no line while it runs"
        answer_all(page, held, DISCARDED)
        assert wait_until(page, line_shown(page, sel), 5), "discarded, with no wait before it: the line at once"
        assert awaiting(page, rid) is None, "and no wait"

        assert wait_until(page, "document.querySelector(%r).textContent === %r" % (sel + " .setpoint-big", shown(21, case.lang, "°")), 5), \
            "SET the room thermostat's 21, which MANU sends"
        tile.locator(".mode-manual").click()
        assert wait_held(page, held), "MANU sends"
        answer_all(page, held, PENDING)
        assert wait_until(page, "roomUI(state.rooms.find((r) => r.id === %d)).awaiting !== null" % rid, 5), "MANU waits"
        want = awaiting(page, rid)
        assert want == {"mode": "manual", "setpoint": 21}, want
        for body, status, what in ((DISCARDED, 200, "a discarded write"), ('{"error": "the Cube refused"}', 502, "a failed write")):
            tile.locator(".mode-auto").click()
            assert wait_held(page, held), f"{what} is sent"
            answer_all(page, held, body, status)
            assert wait_until(page, "!document.querySelector(%r).classList.contains('busy')" % (sel + " .mode-auto"), 5), f"{what} answered"
            assert awaiting(page, rid) == want, f"{what} leaves the wait"
            assert members_line(tile) is None, f"after {what}, no line while MANU waits"
    finally:
        page.unroute(PUT)


def test_a_boost_waits_for_nothing(page, case, reads):
    """A boost changes no setpoint the line shows: it starts no wait."""
    told = Told(page, reads, two=True)
    rid, (a, b) = told.rid, told.room["devices"][:2]
    told.set({a["rf"]: {"setpoint": 21, "mode": "auto"}, b["rf"]: {"setpoint": 17, "mode": "auto"}})
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        page.evaluate("(rid) => { sendRoomSetpoint(state.rooms.find((r) => r.id === rid), { mode: 'boost' }, 'boost', 'boost'); }", rid)
        assert wait_held(page, held), "the boost is sent"
        answer_all(page, held, PENDING)
        assert wait_until(page, line_shown(page, '.room[data-id="%s"]' % rid), 5), "the line once it is answered"
        assert awaiting(page, rid) is None, "no wait"
    finally:
        page.unroute(PUT)


def test_a_pending_write_or_an_open_window_hides_the_line(page, case, reads):
    """A thermostat showing a write of its mode or setpoint pending, as the
    server says it (another client's, one from before a reload), hides the
    line, an unconfirmed one or one of the key lock does not; so does an
    open window of the room, its radiators at their window-open
    temperature, as the room checks pause for it, an out-of-date
    contact's last report too (D17-A)."""
    told = Told(page, reads, two=True)
    rid, (a, b) = told.rid, told.room["devices"][:2]
    sel = '.room[data-id="%s"]' % rid
    told.set({a["rf"]: {"setpoint": 21, "mode": "manual"}, b["rf"]: {"setpoint": 17, "mode": "manual"}})
    assert wait_until(page, line_shown(page, sel), 5), "the line while they differ"
    for conf, hides in (({"state": "pending", "fields": ["setpoint"]}, True), ({"state": "pending", "fields": ["mode"]}, True),
                        ({"state": "pending", "fields": ["locked"]}, False), ({"state": "unconfirmed", "fields": ["mode", "setpoint"]}, False),
                        (None, False)):
        told.set({b["rf"]: {"setpoint": 17, "mode": "manual", "confirmation": conf}})
        assert wait_until(page, line_hidden(page, sel) if hides else line_shown(page, sel), 5), f"{conf}: hidden {hides}"
    told.window(True, False)
    assert wait_until(page, line_hidden(page, sel), 5), "an open window hides the line"
    told.window(False, False)
    assert wait_until(page, line_shown(page, sel), 5), "the window closed, the line again"
    told.window(True, True)
    assert wait_until(page, line_hidden(page, sel), 5), "an out-of-date contact whose last report is open hides the line"
    told.window(False, True)
    assert wait_until(page, line_shown(page, sel), 5), "and one whose last report is closed does not"
    assert scroll_width(page) <= case.width, "no horizontal scroll"


@pytest.mark.clock
@pytest.mark.cases("1100-en", "390-de")
def test_members_line_after_the_deadline(page, case, reads):
    """A write some thermostat does not show: the line stays away for the
    confirmation's five minutes, then shows the setpoints that still
    differ."""
    told = Told(page, reads, two=True)
    rid, (a, b) = told.rid, told.room["devices"][:2]
    tile = page.locator('.room[data-id="%s"]' % rid)
    sel = '.room[data-id="%s"]' % rid
    told.set({a["rf"]: {"setpoint": 20, "mode": "manual"}, b["rf"]: {"setpoint": 20, "mode": "manual"}})
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        # not awaited: page.evaluate would wait for the held write's answer
        page.evaluate("(rid) => { sendRoomSetpoint(state.rooms.find((r) => r.id === rid), { mode: 'manual', temp: 22 }, 'manual 22', 'manual'); }", rid)
        assert wait_held(page, held), "the write is sent"
        answer_all(page, held, PENDING)
        assert wait_until(page, "roomUI(state.rooms.find((r) => r.id === %d)).awaiting !== null" % rid, 5), "the write waits"
        told.set({a["rf"]: {"setpoint": 22, "mode": "manual"}})
        page.clock.fast_forward(4 * 60 * 1000)
        assert members_line(tile) is None, "no line within the five minutes"
        page.clock.fast_forward(61 * 1000)
        assert wait_until(page, line_shown(page, sel), 5), "the line after them"
    finally:
        page.unroute(PUT)


def test_shows_off_of_a_write_not_confirmed(page, case, reads):
    """A device that has not shown a setpoint write says what it shows,
    OFF for 4.5 °C. Told to the rooms' read as well (Told), the list
    opened again where a redraw closed it (open_list)."""
    told = Told(page, reads)
    rid, d = told.rid, told.room["devices"][0]
    tile = page.locator('.room[data-id="%s"]' % rid)
    told.set({d["rf"]: {"setpoint": 4.5, "mode": "manual", "confirmation": {"state": "unconfirmed", "fields": ["setpoint"]}}})
    flag = "not confirmed" if case.lang == "en" else "nicht bestätigt"
    want = "setpoint (shows OFF)" if case.lang == "en" else "Solltemperatur (zeigt OFF)"
    open_list(tile, """(d) => [...d.querySelectorAll('.problem-flags .flag[title]')].some((f) => f.textContent.includes(%s)
      && f.getAttribute('title').includes(%s))""" % (json.dumps(flag), json.dumps(want)), f"the flag {flag!r} says what it shows: {want!r}")
    page.keyboard.press("Escape")


def test_plain_formatter(page, case):
    """The plain scripts' formatter (app.js: the tiles' result lines, the
    week programme's segments) writes a setpoint as the parts' does: OFF
    at 4.5 °C alone, without a unit, and no ON at 30.5 °C."""
    got = page.evaluate("[fmtSetpoint(4.5), fmtSetpoint(4.5, '°'), fmtSetpoint(5), fmtSetpoint(30.5, '°'), fmtSetpoint(undefined)]")
    assert got == ["OFF", "OFF", num(5, case.lang) + " °C", num(30.5, case.lang) + "°", "–"], json.dumps(got)
