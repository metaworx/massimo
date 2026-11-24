# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A setpoint as the eQ-3 devices show it (AP RoomMajority v1.1, Block 3):
4.5 °C is OFF in the tile's SET, its stepped value and a device row,
without a unit; a crescent moon marks a setpoint at the eco
temperature, a sun at the comfort temperature, the two side by side
where comfort is eco, in the tile by the room thermostat's and beside
its value, in a slot of its own so that nothing moves, in a device row
by the device's own and in the place of Set's gauge; the marks are
labelled in the configuration's words and in the accent colour, in
light and dark. The device entries carry the temperatures (comfort_temp,
eco_temp) as the devices' details have them, and a config event brings
new ones. While the room's thermostats show different setpoints, a line
under the setpoint row names each with its setpoint and mark, not while
a write of the tile or the House tile waits for them to show it, a
write of theirs is pending or a window of the room is open, and goes
once they agree.

The states come from state events applied in the page (updateDevice);
every write is held at its route and answered with a stub, so nothing
reaches the fake Cube."""
import json

import pytest

from helpers import busy_state, hold, scroll_width, wait_held, wait_until

# The room of the id given, else the first with two reported
# thermostats, else the first with one; with two given, a copy of its
# thermostat is put beside it in the page (as test_room_more.py's TWIN),
# so that its thermostats can differ. An open window of the room is
# closed in the page, as it would hide the members' line. Returns the
# room's id and its reported thermostats, the room thermostat first, by
# radio address, name and its eco and comfort temperatures.
ROOM = """([rid, two]) => {
  const reported = (r) => r.devices.filter((d) => isThermostat(d) && d.reported);
  const r = rid ? state.rooms.find((x) => x.id === rid)
    : state.rooms.find((x) => reported(x).length >= 2) || state.rooms.find((x) => reported(x).length >= 1);
  if (two && reported(r).length < 2) {
    const twin = JSON.parse(JSON.stringify(reported(r)[0]));
    Object.assign(twin, { rf_address: "0a0fff", serial: "MEQ0000999", name: "Twin" });
    r.devices = [...r.devices, twin]; // a room's devices are a signal: its tile follows
  }
  for (const w of r.devices.filter((d) => d.type === "shutter_contact" && d.state && d.state.window_open)) {
    const d = JSON.parse(JSON.stringify(w));
    d.state.window_open = false;
    updateDevice(d);
  }
  const th = roomThermostat(r);
  const ths = [th, ...reported(r).filter((d) => d !== th)];
  return { id: r.id, devices: ths.map((d) => ({ rf: d.rf_address, name: deviceName(d), eco: d.eco_temp, comfort: d.comfort_temp })) };
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
# out-of-date contact whose last report (last_known) says it.
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
}"""

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


def mark_of(where):
    """The preset mark in a place: its preset (by its class), its label,
    its tooltip and its symbol; None where there is none."""
    marks = where.locator(".preset-mark")
    if marks.count() == 0:
        return None
    assert marks.count() == 1, "one mark"
    m = marks.first
    cls = [c[len("preset-"):] for c in (m.get_attribute("class") or "").split() if c.startswith("preset-") and c != "preset-mark"]
    return {"preset": cls[0] if cls else None, "label": m.get_attribute("aria-label"), "title": m.get_attribute("title"),
            "role": m.get_attribute("role"), "symbol": m.locator("use").get_attribute("href")}


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


def test_marks_and_off(page, case):
    """The tile's SET and a device row at OFF, at eco, at comfort, at
    comfort equal to eco, at none of them and at the highest: OFF
    without a unit, the mark at the top right of the tile's value, out of
    the flow, and in the place of the row's gauge, labelled; the tile's
    value and the chevrons in the setpoint row, the row as large, and the
    device row's icon slot in that row, where they are in every case, the
    mark costing no width; the
    stepped value marked as it is stepped onto eco, nothing moving; the
    marks in the accent colour of the tile's and the row's other icons,
    in light and in dark; no horizontal scroll. Each place is measured
    in its row, which whatever is above the tile moves as a whole."""
    room = page.evaluate(ROOM, [None, False])
    rid, ths = room["id"], room["devices"]
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
            page.evaluate(SET, [rid, {d["rf"]: {"setpoint": sp, "eco": eco, "comfort": comfort} for d in ths}])
            want = shown(sp, lang, "°")
            assert wait_until(page, "document.querySelector(%r).textContent === %r" % (sel + " .setpoint-big", want), 5), \
                f"{name}: the tile's SET shows {want}: {tile.locator('.setpoint-big').inner_text()!r}"
            m = mark_of(tile.locator(".setpoint-row"))
            if preset is None:
                assert m is None, f"{name}: no mark in the tile: {m}"
            else:
                label = LABEL[lang][preset]
                assert m == {"preset": preset, "label": label, "title": label, "role": "img", "symbol": SYMBOL[preset]}, f"{name}: the tile's mark: {m}"
            assert tile.locator(".temp-tag use[href='#icon-set']").count() == 1, f"{name}: SET keeps its icon"
            assert tile.locator(".temp-tag .temp-label").nth(1).inner_text() == ("SET" if lang == "en" else "SOLL"), f"{name}: and its label"
            geometry.append((box(page, sel + " .setpoint-big", sp_row), box(page, sel + " .stepper", sp_row), box(page, sp_row, sp_row)))

            cell = set_cell(row, lang)
            assert cell.locator(".num").inner_text() == shown(sp, lang, ""), f"{name}: the row's value: {cell.locator('.num').inner_text()!r}"
            assert cell.locator(".u").inner_text() == ("" if sp == 4.5 else UNIT[lang]), f"{name}: the row's unit"
            rm = mark_of(cell)
            if preset is None:
                assert rm is None and cell.locator("use[href='#icon-set']").count() == 1, f"{name}: the gauge, no mark: {rm}"
            else:
                label = LABEL[lang][preset]
                assert rm == {"preset": preset, "label": label, "title": label, "role": "img", "symbol": SYMBOL[preset]}, f"{name}: the row's mark: {rm}"
                assert cell.locator("use[href='#icon-set']").count() == 0, f"{name}: the mark in the gauge's place"
            slots.append(cell.evaluate("(c) => { const r = c.querySelector('svg.di').getBoundingClientRect(), o = c.closest('.device').getBoundingClientRect(); return { x: r.x - o.x, width: r.width }; }"))

            row_box = page.evaluate("(s) => { const e = document.querySelector(s); return [e.scrollWidth, e.clientWidth]; }", sel + " .setpoint-row")
            assert row_box[0] <= row_box[1], f"{name}: the setpoint row fits its tile: {row_box}"
            assert scroll_width(page) <= case.width, f"{name}: no horizontal scroll"

        for (name, *_), b in zip(CASES, slots):
            assert abs(b["x"] - slots[0]["x"]) < 0.5 and abs(b["width"] - slots[0]["width"]) < 0.5, \
                f"{name}: the row's icon slot stays where it is: {b}, {slots[0]}"
        for (name, *_), g in zip(CASES, geometry):
            for what, a, b in zip(("the value", "the chevrons", "the setpoint row"), g, geometry[0]):
                assert all(abs(x - y) < 0.5 for x, y in zip(a, b)), f"{name}: {what} stays where it is in the setpoint row: {a}, {geometry[0]}"

        # The stepped value: half a degree under the eco temperature, then
        # stepped onto it; held at its route, so that nothing is sent.
        page.evaluate(SET, [rid, {d["rf"]: {"setpoint": 16.5, "eco": 17, "comfort": 21} for d in ths}])
        assert wait_until(page, "!document.querySelector(%r)" % (sel + " .setpoint-row .preset-mark"), 5), "16.5 is no preset"
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
            page.evaluate(SET, [rid, {d["rf"]: {"setpoint": 5, "eco": 17, "comfort": 21} for d in ths}])
            tile.hover()
            tile.locator(".step-minus").click()
            assert tile.locator(".setpoint-big.pending").inner_text() == "OFF", "stepped down to OFF"
            tile.locator(".pending-cancel").click()
            assert not sent, f"nothing sent: {sent}"
        finally:
            page.unroute(PUT)

        # The marks' colour: the accent, as SET's icon and the row's
        # Actual, in light and in dark.
        page.evaluate(SET, [rid, {d["rf"]: {"setpoint": 17, "eco": 17, "comfort": 21} for d in ths}])
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


@pytest.mark.cases("390-en")
def test_the_setpoint_row_fits_in_every_language(page, case):
    """At 390 px, in each of the six languages, whose labels of IS and SET
    differ in length (Italian's the longest): the setpoint row fits its
    tile and the page does not scroll, at the widest value and at OFF,
    with a mark and without, in light and dark; the mark costs no width,
    the chevrons where they are in the row without it."""
    room = page.evaluate(ROOM, [None, False])
    rid, ths = room["id"], room["devices"]
    sel = '.room[data-id="%s"]' % rid
    got = []
    try:
        for lang in ("en", "de", "fr", "it", "nl", "pl"):
            page.evaluate("async (l) => { await changeLanguage(l); applyLanguage(); }", lang)
            for theme in ("light", "dark"):
                page.evaluate("(t) => { document.documentElement.dataset.theme = t; }", theme)
                steppers = {}
                for sp, eco in ((30.5, 17), (30.5, 30.5), (4.5, 17), (4.5, 4.5)):
                    page.evaluate(SET, [rid, {d["rf"]: {"setpoint": sp, "eco": eco, "comfort": 21} for d in ths}])
                    marked = eco == sp
                    assert wait_until(page, "!!document.querySelector(%r) === %s" % (sel + " .set-val .preset-mark", "true" if marked else "false"), 5), \
                        f"{lang}: the mark {marked} at {sp}"
                    m = page.evaluate("""(s) => {
                      const row = document.querySelector(s + " .setpoint-row");
                      const r = row.getBoundingClientRect(), cs = getComputedStyle(row);
                      const lo = r.left + parseFloat(cs.paddingLeft), hi = r.right - parseFloat(cs.paddingRight);
                      const boxes = [...row.querySelectorAll(".temp-tag, .temp-big, .preset-mark, .stepper")].map((e) => e.getBoundingClientRect());
                      const left = Math.min(...boxes.map((b) => b.left)), right = Math.max(...boxes.map((b) => b.right));
                      return { scroll: row.scrollWidth, client: row.clientWidth, inside: left >= lo - 0.5 && right <= hi + 0.5,
                        spare: Math.round((lo - left) * 10) / 10 + "/" + Math.round((right - hi) * 10) / 10,
                        stepper: row.querySelector(".stepper").getBoundingClientRect().x - r.x, page: document.documentElement.scrollWidth };
                    }""", sel)
                    got.append((lang, theme, sp, marked, m))
                    assert m["scroll"] <= m["client"] and m["inside"], f"{lang}, {theme}, {sp}, mark {marked}: the row's content fits inside its padding: {m}"
                    assert m["page"] <= case.width, f"{lang}, {theme}: no horizontal scroll: {m}"
                    steppers.setdefault(sp, set()).add(round(m["stepper"], 1))
                for sp, xs in steppers.items():
                    assert len(xs) == 1, f"{lang}, {theme}, {sp}: the mark moves the chevrons: {xs}"
    finally:
        page.evaluate("async () => { await changeLanguage('en'); applyLanguage(); delete document.documentElement.dataset.theme; }")


def test_more_shows_off_from_the_entry(page, case):
    """More's Eco and Comfort come from the room thermostat's entry, with
    no request for its detail (D13-A), and show OFF for an eco
    temperature of 4.5 °C, as the devices' displays show it."""
    room = page.evaluate(ROOM, [None, False])
    rid, ths = room["id"], room["devices"]
    tile = page.locator('.room[data-id="%s"]' % rid)
    page.evaluate(SET, [rid, {d["rf"]: {"setpoint": 20, "eco": 4.5, "comfort": 21} for d in ths}])
    asked = []
    page.on("request", lambda r: asked.append(r.url) if r.method == "GET" and "/devices/" in r.url else None)
    page.evaluate("state.details = {}")
    tile.locator(".more-sum").click()
    assert wait_until(page, "document.querySelector(%r).textContent !== ''" % ('.room[data-id="%s"] .menu-eco .more-temp' % rid), 5), \
        "Eco shows its temperature"
    assert tile.locator(".menu-eco .more-temp").inner_text() == "OFF", "eco at 4.5 °C is OFF"
    assert tile.locator(".menu-comfort .more-temp").inner_text() == num(21, case.lang) + " °C", "comfort as a temperature"
    page.keyboard.press("Escape")
    assert not asked, f"no detail asked for: {asked}"


def test_a_config_event_brings_new_presets(page, case):
    """A config event carries the device's entry with its new eco and
    comfort temperatures: the tile's and the row's marks follow it."""
    room = page.evaluate(ROOM, [None, False])
    rid, th = room["id"], room["devices"][0]
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    page.evaluate(SET, [rid, {th["rf"]: {"setpoint": 18, "eco": 16, "comfort": 21}}])
    assert wait_until(page, "!document.querySelector(%r)" % (sel + " .setpoint-row .preset-mark"), 5), "18 is no preset"
    tile.locator(".room-name-btn").click()
    row = tile.locator('.device[data-rf="%s"]' % th["rf"])
    row.wait_for()
    try:
        page.evaluate("""([rid, rf]) => {
          const r = state.rooms.find((x) => x.id === rid);
          const d = JSON.parse(JSON.stringify(r.devices.find((x) => x.rf_address === rf)));
          d.eco_temp = 18;
          state.events.dispatchEvent(new MessageEvent("config", { data: JSON.stringify({ kind: "config", source: "device", level: "info",
            device: rf, serial: d.serial, time: new Date().toISOString(), changes: { eco_temp: { from: 16, to: 18 } }, state: d }) }));
        }""", [rid, th["rf"]])
        assert wait_until(page, "!!document.querySelector(%r)" % (sel + " .setpoint-row .preset-mark.preset-eco"), 5), "the tile's eco mark"
        assert mark_of(set_cell(row, case.lang))["preset"] == "eco", "the row's eco mark"
    finally:
        tile.locator(".room-name-btn").click()


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


def test_members_line(page, case):
    """While the room's thermostats show different setpoints, a line under
    the setpoint row names each with its setpoint, OFF included, and its
    own mark, " · " between them; it is the same element as they change;
    not while a write of the tile runs or waits for them to show it, a
    room command reaching one after the other; gone once they agree, and
    at once again where one is changed by hand after that."""
    room = page.evaluate(ROOM, [None, True])
    rid, ths = room["id"], room["devices"][:2]
    a, b = ths
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    lang = case.lang
    page.evaluate(SET, [rid, {a["rf"]: {"setpoint": 21, "mode": "auto", "eco": 17, "comfort": 21},
                              b["rf"]: {"setpoint": 21, "mode": "auto", "eco": 17, "comfort": 21}}])
    assert wait_until(page, line_hidden(page, sel), 5), "no line while they agree"

    page.evaluate(SET, [rid, {b["rf"]: {"setpoint": 4.5, "mode": "auto", "eco": 16, "comfort": 21}}])
    assert wait_until(page, line_shown(page, sel), 5), "the line once they differ"
    line = members_line(tile)
    assert line["label"] == MEMBERS[lang] and line["members"] == [(a["rf"], a["name"], num(21, lang) + " °C", "comfort"), (b["rf"], b["name"], "OFF", None)], line
    assert line["text"].count(" · ") == 1, f"a separator between the two: {line['text']!r}"
    el = page.evaluate_handle("(s) => document.querySelector(s)", sel + " .members-line")
    member = page.evaluate_handle("(s) => document.querySelector(s)", sel + ' .members-line .member[data-rf="%s"]' % b["rf"])
    page.evaluate(SET, [rid, {b["rf"]: {"setpoint": 16, "mode": "auto", "eco": 16, "comfort": 21}}])
    assert wait_until(page, "document.querySelector(%r).textContent.includes(%r)" % (sel + " .members-line", num(16, lang)), 5), "the line follows"
    assert members_line(tile)["members"][1] == (b["rf"], b["name"], num(16, lang) + " °C", "eco"), "each by its own eco temperature"
    assert page.evaluate("([e, s]) => e === document.querySelector(s)", [el, sel + " .members-line"]), "the same line"
    assert page.evaluate("([e, s]) => e === document.querySelector(s)", [member, sel + ' .members-line .member[data-rf="%s"]' % b["rf"]]), \
        "the same member, updated in place"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the line"

    # A write of the tile: MANU, held, then answered; the room thermostat
    # takes it, then the other; the line stays away until they agree.
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        tile.locator(".mode-manual").click()
        assert wait_held(page, held), "MANU sends"
        assert sent[-1] == {"mode": "manual", "temp": 21}, f"the room thermostat's setpoint: {sent}"
        assert members_line(tile) is None, "no line while the write runs"
        answer_all(page, held, PENDING)
        assert wait_until(page, "!document.querySelector(%r).classList.contains('busy')" % (sel + " .mode-manual"), 5), "the write answered"
        assert members_line(tile) is None, "no line while the thermostats do not show it yet"
        page.evaluate(SET, [rid, {a["rf"]: {"setpoint": 21, "mode": "manual"}}])
        page.wait_for_timeout(300)
        assert members_line(tile) is None, "nor while one shows it and the other not"
        page.evaluate(SET, [rid, {b["rf"]: {"setpoint": 21, "mode": "manual"}}])
        assert wait_until(page, "roomUI(state.rooms.find((r) => r.id === %d)).awaiting === null" % rid, 5), "the wait ends once both show it"
        assert members_line(tile) is None, "they agree: no line"
        page.evaluate(SET, [rid, {b["rf"]: {"setpoint": 18, "mode": "manual"}}])
        assert wait_until(page, line_shown(page, sel), 5), "a change by hand after the write shows at once"
    finally:
        page.unroute(PUT)


def test_auto_into_auto_waits_for_the_setpoints(page, case):
    """AUTO while every thermostat is in auto already: the wait does not
    end with the answer, though they all show the mode, nor at a state
    event of another kind (a valve report, a window contact) while they
    still agree on their setpoints from before, but once they show
    agreeing setpoints taken since, so that the line does not flicker as
    one takes its programme's setpoint before the other (D17-A)."""
    room = page.evaluate(ROOM, [None, True])
    rid, (a, b) = room["id"], room["devices"][:2]
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    page.evaluate(SET, [rid, {a["rf"]: {"setpoint": 17, "mode": "auto"}, b["rf"]: {"setpoint": 17, "mode": "auto"}}])
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        tile.locator(".mode-auto").click()
        assert wait_held(page, held), "AUTO sends"
        assert sent[-1] == {"mode": "auto"}, sent
        answer_all(page, held, PENDING)
        assert wait_until(page, "!document.querySelector(%r).classList.contains('busy')" % (sel + " .mode-auto"), 5), "the write answered"
        assert (awaiting(page, rid) or {}).get("mode") == "auto", "the wait holds though both show auto"
        page.evaluate(SET, [rid, {b["rf"]: {"setpoint": 17, "mode": "auto", "valve": 40}}])
        page.evaluate(WINDOW, [rid, False, False])
        page.wait_for_timeout(300)
        assert (awaiting(page, rid) or {}).get("mode") == "auto", "a valve report and a window contact before any takes the programme end nothing"
        page.evaluate(SET, [rid, {a["rf"]: {"setpoint": 21, "mode": "auto"}}])
        page.wait_for_timeout(300)
        assert members_line(tile) is None, "no line while one has taken its programme's setpoint and the other not"
        page.evaluate(SET, [rid, {b["rf"]: {"setpoint": 21, "mode": "auto"}}])
        assert wait_until(page, "roomUI(state.rooms.find((r) => r.id === %d)).awaiting === null" % rid, 5), "the wait ends once they agree"
        page.evaluate(SET, [rid, {b["rf"]: {"setpoint": 19, "mode": "auto"}}])
        assert wait_until(page, line_shown(page, sel), 5), "a later difference shows at once"
    finally:
        page.unroute(PUT)


def test_the_house_holds_the_line(page, case):
    """A write of the House tile to the room, from its answer on, holds
    the room's line as the tile's own write does, until its thermostats
    show it."""
    room = page.evaluate(ROOM, [None, True])
    rid, (a, b) = room["id"], room["devices"][:2]
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    page.evaluate(SET, [rid, {a["rf"]: {"setpoint": 20, "mode": "manual"}, b["rf"]: {"setpoint": 20, "mode": "manual"}}])
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
        page.evaluate(SET, [rid, {a["rf"]: {"setpoint": 17, "mode": "auto"}}])
        page.wait_for_timeout(300)
        assert members_line(tile) is None, "no line while they take it one by one"
        page.evaluate(SET, [rid, {b["rf"]: {"setpoint": 17, "mode": "auto"}}])
        assert wait_until(page, "roomUI(state.rooms.find((r) => r.id === %d)).awaiting === null" % rid, 5), "the wait ends once they agree"
    finally:
        page.unroute(PUT)
        page.locator("#house-panel .form-cancel").click()


def test_a_write_not_taken_leaves_the_wait(page, case, watch):
    """A write the Cube discarded or that failed replaces no wait: with
    none before it, the line shows at once after its answer; after a write
    the Cube took, the line stays away for that one, until its
    thermostats show it (D17-A)."""
    room = page.evaluate(ROOM, [None, True])
    rid, (a, b) = room["id"], room["devices"][:2]
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    watch.expected.append(("/rooms/%d/setpoint" % rid, 502))  # the failed write's answer, on purpose
    page.evaluate(SET, [rid, {a["rf"]: {"setpoint": 21, "mode": "auto"}, b["rf"]: {"setpoint": 17, "mode": "auto"}}])
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


def test_a_boost_waits_for_nothing(page, case):
    """A boost changes no setpoint the line shows: it starts no wait."""
    room = page.evaluate(ROOM, [None, True])
    rid, (a, b) = room["id"], room["devices"][:2]
    page.evaluate(SET, [rid, {a["rf"]: {"setpoint": 21, "mode": "auto"}, b["rf"]: {"setpoint": 17, "mode": "auto"}}])
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


def test_a_pending_write_or_an_open_window_hides_the_line(page, case):
    """A thermostat showing a write of its mode or setpoint pending, as the
    server says it (another client's, one from before a reload), hides the
    line, an unconfirmed one or one of the key lock does not; so does an
    open window of the room, its radiators at their window-open
    temperature, as the room checks pause for it, an out-of-date
    contact's last report too (D17-A)."""
    room = page.evaluate(ROOM, [None, True])
    rid, (a, b) = room["id"], room["devices"][:2]
    sel = '.room[data-id="%s"]' % rid
    page.evaluate(SET, [rid, {a["rf"]: {"setpoint": 21, "mode": "manual"}, b["rf"]: {"setpoint": 17, "mode": "manual"}}])
    assert wait_until(page, line_shown(page, sel), 5), "the line while they differ"
    for conf, hides in (({"state": "pending", "fields": ["setpoint"]}, True), ({"state": "pending", "fields": ["mode"]}, True),
                        ({"state": "pending", "fields": ["locked"]}, False), ({"state": "unconfirmed", "fields": ["mode", "setpoint"]}, False),
                        (None, False)):
        page.evaluate(SET, [rid, {b["rf"]: {"setpoint": 17, "mode": "manual", "confirmation": conf}}])
        assert wait_until(page, line_hidden(page, sel) if hides else line_shown(page, sel), 5), f"{conf}: hidden {hides}"
    page.evaluate(WINDOW, [rid, True, False])
    assert wait_until(page, line_hidden(page, sel), 5), "an open window hides the line"
    page.evaluate(WINDOW, [rid, False, False])
    assert wait_until(page, line_shown(page, sel), 5), "the window closed, the line again"
    page.evaluate(WINDOW, [rid, True, True])
    assert wait_until(page, line_hidden(page, sel), 5), "an out-of-date contact whose last report is open hides the line"
    page.evaluate(WINDOW, [rid, False, True])
    assert wait_until(page, line_shown(page, sel), 5), "and one whose last report is closed does not"
    assert scroll_width(page) <= case.width, "no horizontal scroll"


@pytest.mark.clock
@pytest.mark.cases("1100-en", "390-de")
def test_members_line_after_the_deadline(page, case):
    """A write some thermostat does not show: the line stays away for the
    confirmation's five minutes, then shows the setpoints that still
    differ."""
    room = page.evaluate(ROOM, [None, True])
    rid, (a, b) = room["id"], room["devices"][:2]
    tile = page.locator('.room[data-id="%s"]' % rid)
    sel = '.room[data-id="%s"]' % rid
    page.evaluate(SET, [rid, {a["rf"]: {"setpoint": 20, "mode": "manual"}, b["rf"]: {"setpoint": 20, "mode": "manual"}}])
    held, sent = [], []
    hold(page, PUT, "PUT", held, sent)
    try:
        # not awaited: page.evaluate would wait for the held write's answer
        page.evaluate("(rid) => { sendRoomSetpoint(state.rooms.find((r) => r.id === rid), { mode: 'manual', temp: 22 }, 'manual 22', 'manual'); }", rid)
        assert wait_held(page, held), "the write is sent"
        answer_all(page, held, PENDING)
        assert wait_until(page, "roomUI(state.rooms.find((r) => r.id === %d)).awaiting !== null" % rid, 5), "the write waits"
        page.evaluate(SET, [rid, {a["rf"]: {"setpoint": 22, "mode": "manual"}}])
        page.clock.fast_forward(4 * 60 * 1000)
        assert members_line(tile) is None, "no line within the five minutes"
        page.clock.fast_forward(61 * 1000)
        assert wait_until(page, line_shown(page, sel), 5), "the line after them"
    finally:
        page.unroute(PUT)


def test_shows_off_of_a_write_not_confirmed(page, case):
    """A device that has not shown a setpoint write says what it shows,
    OFF for 4.5 °C."""
    room = page.evaluate(ROOM, [None, False])
    rid, d = room["id"], room["devices"][0]
    tile = page.locator('.room[data-id="%s"]' % rid)
    page.evaluate(SET, [rid, {d["rf"]: {"setpoint": 4.5, "mode": "manual", "confirmation": {"state": "unconfirmed", "fields": ["setpoint"]}}}])
    tile.locator(".tile-head .problems-sum").click()
    flag = tile.locator(".tile-head .problem-flags .flag[title]", has_text="not confirmed" if case.lang == "en" else "nicht bestätigt").first
    want = "setpoint (shows OFF)" if case.lang == "en" else "Solltemperatur (zeigt OFF)"
    assert want in (flag.get_attribute("title") or ""), flag.get_attribute("title")
    page.keyboard.press("Escape")


def test_plain_formatter(page, case):
    """The plain scripts' formatter (app.js: the tiles' result lines, the
    week programme's segments) writes a setpoint as the parts' does: OFF
    at 4.5 °C alone, without a unit, and no ON at 30.5 °C."""
    got = page.evaluate("[fmtSetpoint(4.5), fmtSetpoint(4.5, '°'), fmtSetpoint(5), fmtSetpoint(30.5, '°'), fmtSetpoint(undefined)]")
    assert got == ["OFF", "OFF", num(5, case.lang) + " °C", num(30.5, case.lang) + "°", "–"], json.dumps(got)
