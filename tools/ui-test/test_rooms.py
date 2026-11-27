# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The Rooms page: the tiles, a tile's device list and its
configuration; the problems in a tile's head and in a device's; the ages
of the values, a stale device, and what expert mode adds.

The window, the faults and the levels come from state events applied
in the page (updateDevice) and are told to the page's own reads of the
rooms as well (Reads.device), the devices kept; the staleness, a room
not ready and the Cube's findings from those reads alone, answered by
the fixture reads (ROOMS), since the fake Cube does not change its
states. The server's own state events of a device kept, which an
earlier test's write brings, draw the server's data until keep's read
draws the told ones again, so a check of them waits, and a problem list
is opened again where a redraw closed it (open_list; AP ToldData,
Block 4).
"""
import json
import re

from helpers import ECO_BUTTON, OPEN, STALE, named_buttons, scroll_width, settled, wait_held, wait_until
from test_new_batteries import Device
from test_room_more import open_list

# The page's read of the chosen Cube's rooms (fetchRooms of app.js), which
# the fixture reads answers with what a test puts in.
ROOMS = "**/rooms?devices=all"


def change_room(room_id, fn):
    """A change of the rooms read (reads.change) that applies fn to the
    room room_id alone."""
    def change(rooms):
        for r in rooms or []:
            if r.get("id") == room_id:
                fn(r)
        return rooms
    return change


def test_tiles_device_list_configuration(page, case):
    assert page.locator(".room").count() > 0, "room tiles shown"
    assert page.evaluate("document.documentElement.lang") == case.lang, f"page language {case.lang}"
    named_buttons(page, "the rooms")
    tile = page.locator(".room").first
    tile.locator(".room-name-btn").click()
    page.wait_for_selector(".room .device", timeout=5000)
    assert tile.locator(".device").count() > 0, "device list opens"
    named_buttons(page, "a device list")
    tile.locator(".tile-cog").click()
    page.wait_for_selector(".room .config-body", timeout=5000)
    tabs = tile.locator(".config-tabs [role=tab]")
    assert tabs.count() >= 4, "configuration opens with its tabs"
    for name in tabs.all_text_contents():
        tabs.filter(has_text=name).first.click()
        assert tile.locator(".config-body").is_visible(), f"tab {name.strip()}"
        named_buttons(page, f"the configuration tab {name.strip()}")
    tile.locator(".config-tabs .close").click()
    assert tile.locator(".config-body").count() == 0, "configuration closes"
    assert scroll_width(page) <= case.width, "no horizontal scroll on the tiles"


def sign_icons(summary):
    """The icons of a problems' summary, each by its symbols: a sign with
    a mark as its drawing and its mark, "#icon-radio-small+#mark-corner-x"."""
    return [svg.evaluate("(s) => [...s.querySelectorAll('use')].map((u) => u.getAttribute('href')).join('+')")
            for svg in summary.locator("svg").all()]


def head_icons(room):
    """The icons of a tile head's problems, by their symbols."""
    return sign_icons(room.locator(".tile-head .problems-sum"))


def head_shows(room, want, what, tone=None):
    """Waits until a tile head's problems show the icons want, the first
    in tone where given (settled)."""
    def look():
        icons = head_icons(room)
        first = room.locator(".tile-head .problems-sum svg").first.get_attribute("class", timeout=1000) or "" if tone else ""
        return icons, first
    ok, got = settled(room.page, look, lambda g: (g[0] == want if not callable(want) else want(g[0])) and (tone is None or tone in g[1]))
    assert ok, f"{what}: {got}"
    return got[0]


# Gives the open window contact the times of a window open for `ago`
# minutes, by the server's limits of 5 and 10 minutes; returns its radio
# address and its window.
WINDOW_TIMES = """(ago) => {
  const r = state.rooms.find((r) => r.devices.some((d) => d.type === "shutter_contact"));
  const d = JSON.parse(JSON.stringify(r.devices.find((d) => d.type === "shutter_contact")));
  const since = Date.now() - ago * 60000;
  const iso = (m) => new Date(since + m * 60000).toISOString();
  d.state.window_open = true;
  d.window = { open_since: iso(0), warn_at: iso(5), alert_at: iso(10) };
  updateDevice(d);
  return { rf: d.rf_address, window: d.window };
}"""


def window_times(page, reads, ago):
    """WINDOW_TIMES in the page and told to the rooms' read, the contact
    kept (Reads.device)."""
    w = page.evaluate(WINDOW_TIMES, ago)
    reads.device(w["rf"], state={"window_open": True}, window=w["window"])


def window_badge(room):
    """The class and the leading mark of the open window's badge in the
    room's list, in one look with the list open, None without; the list
    closed again."""
    details = room.locator(".tile-head details.problems")
    try:
        if not details.evaluate("(d) => d.open", timeout=1000):
            details.locator(":scope > summary").click(timeout=1000)
        got = details.evaluate("""(d) => { const f = d.open && d.querySelector('.problem-entry .flag');
          return f ? [f.getAttribute('class'), [...f.querySelectorAll('svg.mark-sign use')].map((u) => u.getAttribute('href'))] : null; }""",
                               timeout=1000)
    finally:
        room.page.keyboard.press("Escape")
    return tuple(got) if got else None


def badge_shows(room, want, what):
    """Waits until the open window's badge in the room's list is want
    (window_badge, settled)."""
    ok, got = settled(room.page, lambda: window_badge(room), lambda g: g == want)
    assert ok, f"{what}: {got}"


def test_open_window(page, case, reads):
    """The room with the open window contact shows the open window in its
    tile's head, which lists the contact, open, with its age: blue and
    plain for its first five minutes, amber with a triangle up to ten,
    red with a cross after; the icon goes when a state event closes the
    contact; the measured temperature shows its age. The window's times
    are told to the rooms' read as well, the contact kept."""
    room = page.locator(".room", has=page.locator(".room-name", has_text="Kind 1"))
    window_times(page, reads, 1)
    head_shows(room, ["#icon-window-sash-open"], "open briefly: the open window, plain, in blue", tone="tone-open")
    badge_shows(room, ("flag fill-open", []), "its badge filled blue, no mark")
    window_times(page, reads, 6)
    head_shows(room, ["#icon-window-sash-open+#mark-sash-warn"], "open long: amber, a triangle")
    badge_shows(room, ("flag warn", ["#mark-warn"]), "its badge an amber outline led by a triangle")
    window_times(page, reads, 11)
    head_shows(room, ["#icon-window-sash-open+#mark-sash-x"], "open too long: red, a cross")
    badge_shows(room, ("flag fill-bad", ["#mark-x"]), "its badge filled red, led by a cross")
    w = page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.some((d) => d.type === "shutter_contact"));
      const d = JSON.parse(JSON.stringify(r.devices.find((d) => d.type === "shutter_contact")));
      d.window = { open_since: new Date(Date.now() - 299500).toISOString(), warn_at: new Date(Date.now() + 500).toISOString(),
        alert_at: new Date(Date.now() + 300500).toISOString() };
      updateDevice(d);
      return { rf: d.rf_address, window: d.window };
    }""")
    reads.device(w["rf"], window=w["window"])
    assert head_icons(room) == ["#icon-window-sash-open"], "half a second before the warning"
    assert wait_until(page, "document.querySelector('.room .tile-head .problems-sum use[href=\"#mark-sash-warn\"]') !== null", 3), \
        "the tile turns amber by the clock alone"
    summary = room.locator(".tile-head .problems-sum")
    assert OPEN[case.lang] in (summary.get_attribute("aria-label") or "").lower(), "named to a screen reader"
    assert page.locator(".room").first.locator(".problems").count() == 0, "nothing in the head of a room with closed windows"
    open_list(room, """(d) => { const e = d.querySelectorAll('.problem-entry');
      return e.length === 1 && e[0].querySelector('.flag').innerText.startsWith(%s) && e[0].querySelectorAll('.flag .age').length === 1; }"""
              % json.dumps(OPEN[case.lang]), "the contact, open, with the opening's age")
    page.keyboard.press("Escape")
    assert room.locator(".tile-head details.problems").get_attribute("open") is None, "Escape closes the list"
    assert page.locator(".room .temp-age .age").count() > 0, "age next to the measured temperature"
    rf = page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.some((d) => d.type === "shutter_contact"));
      const d = JSON.parse(JSON.stringify(r.devices.find((d) => d.type === "shutter_contact")));
      d.state.window_open = false;
      updateDevice(d);
      return d.rf_address;
    }""")
    reads.device(rf, state={"window_open": False})
    head_shows(room, [], "the icon goes when the window closes")


# Gives the first reported thermostat a communication error, as the
# server's state events would: with the radiogram that failed (failed,
# an API object, or null for none) and, where link, a link error too;
# returns its room's name, its name and its radio address.
RADIO_FAULTS = """([failed, link]) => {
  const r = state.rooms.find((r) => r.devices.some((d) => isThermostat(d) && d.reported));
  if (!r) return null;
  const d = JSON.parse(JSON.stringify(r.devices.find((d) => isThermostat(d) && d.reported)));
  Object.assign(d.state, { error: true, link_error: link, failed_radiogram: failed });
  if (!failed) delete d.state.failed_radiogram;
  updateDevice(d);
  return { room: r.name, name: deviceName(d), rf: d.rf_address };
}"""

# The badges of a problem list (its details) in one look, text: title, of
# its first entry whose name has name, or of its first entry; null while
# it is closed or has none.
FLAGS = """(d, name) => {
  const e = d.open && [...d.querySelectorAll('.problem-entry')].find((x) => !name || (x.querySelector('.problem-name')?.textContent || '').includes(name));
  if (!e) return null;
  return Object.fromEntries([...e.querySelectorAll('.problem-flags .flag')].map((f) => [f.innerText.trim(), f.getAttribute('title')]));
}"""


def flags_of(details, name=None):
    """A problem list's badges (FLAGS), opened by its summary where it is
    closed."""
    if not details.evaluate("(d) => d.open", timeout=1000):
        details.locator(":scope > summary").click(timeout=1000)
    return details.evaluate(FLAGS, name, timeout=1000)


def radio_faults(page, reads, failed, link):
    """RADIO_FAULTS in the page and told to the rooms' read, the device
    kept (Reads.device)."""
    faults = page.evaluate(RADIO_FAULTS, [failed, link])
    if faults:
        reads.device(faults["rf"], state={"error": True, "link_error": link, "failed_radiogram": failed})
    return faults


def test_radio_fault_badges(page, case, reads):
    """A communication error's badge says so and its title the radiogram
    that failed, by its technical name and code, or by its code alone
    where the original's list lacks it; without a radiogram the title
    explains the error alone. A link error's badge says link error, its
    title the original's sentence. The sentences are the page's language,
    the radiogram's name is not translated; the device list's badge has
    the same title. The faults are told to the rooms' read as well, the
    device kept, and each list opened again until it shows them."""
    comm, link = page.evaluate("[t('badge.communicationError'), t('badge.linkError')]")
    explain, sentence = page.evaluate("[t('badge.communicationErrorTitle'), t('badge.linkErrorTitle')]")
    if case.lang == "en":
        assert (comm, link) == ("communication error", "link error"), (comm, link)
        assert sentence == "The radio communication with another MAX! device in this room is disturbed.", sentence
    else:
        assert comm == "Kommunikationsstörung", comm

    def failed(text):
        return explain + "\n" + page.evaluate("(r) => t('badge.failedRadiogram', { radiogram: r })", text)

    def badges(faults, ok):
        """The badges of the device in its room's list, text: title, once
        ok holds of them, the list opened again where a redraw closed it."""
        room = page.locator(".room", has=page.locator(".room-name", has_text=faults["room"])).first
        _, out = settled(page, lambda: flags_of(room.locator(".tile-head details.problems"), faults["name"]), lambda g: g is not None and ok(g))
        assert scroll_width(page) <= case.width, "no horizontal scroll with the list open"
        page.keyboard.press("Escape")
        return out if isinstance(out, dict) else {}

    faults = radio_faults(page, reads, {"code": 3, "name": "TimeInformation"}, False)
    assert faults, "a reported thermostat"
    got = badges(faults, lambda g: g.get(comm) == failed("TimeInformation (0x03)"))
    assert got.get(comm) == failed("TimeInformation (0x03)"), f"the radiogram by its name and code ({got})"
    assert case.lang == "en" or "Last radiogram" not in got[comm], "the sentence translated, the name not"

    # The device list's badge, the same title.
    room = page.locator(".room", has=page.locator(".room-name", has_text=faults["room"])).first
    room.locator(".room-name-btn").click()
    page.wait_for_selector(".room .device", timeout=5000)
    dev = room.locator(".device:not(.virtual)", has=page.locator(".device-name", has_text=faults["name"])).first
    ok, got = settled(page, lambda: flags_of(dev.locator(".device-badges details.problems")),
                      lambda g: g is not None and g.get(comm) == failed("TimeInformation (0x03)"))
    assert ok, f"the device's badge, its title ({got})"
    page.keyboard.press("Escape")
    room.locator(".room-name-btn").click()

    # No radiogram, and a link error.
    got = badges(radio_faults(page, reads, None, True), lambda g: g.get(comm) == explain and g.get(link) == sentence)
    assert got.get(comm) == explain, f"no radiogram: the explanation alone ({got})"
    assert got.get(link) == sentence, f"the link error with the original's sentence ({got})"

    # A code the original's list lacks, by its code.
    got = badges(radio_faults(page, reads, {"code": 246}, False), lambda g: g.get(comm) == failed("0xf6"))
    assert got.get(comm) == failed("0xf6"), f"a code the list lacks, by its code ({got})"


def test_problems_in_the_heads(page, case, reads):
    """A radiator thermostat with a transmit error and a weak battery, and
    two thermostats in different modes: the room's head shows radio,
    battery and the mixed modes, radio in red with a cross; the list names the devices,
    each once, with their badges, a failure bold and led by its cross in
    place of its kind's icon, a warning by its triangle; the device's head shows its problems as
    icons, its mode a word, and every device its type's icon; the room's
    virtual thermostat, whose flags copy the wall thermostat's, is not
    listed. The states are told to the rooms' read as well, the devices
    kept: the head's icons waited for, the list opened again until it
    shows them."""
    told = page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.filter((d) => isThermostat(d)).length >= 2) || state.rooms[0];
      window.__room = r.name;
      const [a, b] = r.devices.filter((d) => isThermostat(d));
      const da = JSON.parse(JSON.stringify(a));
      Object.assign(da.state, { error: true, battery_low: true, mode: "manual" });
      da.battery = "weak";
      updateDevice(da);
      if (b) {
        const db = JSON.parse(JSON.stringify(b));
        db.state.mode = "auto";
        updateDevice(db);
      }
      return { a: a.rf_address, b: b ? b.rf_address : null, vwt: r.vwt ? r.vwt.rf_address : null };
    }""")
    reads.device(told["a"], state={"error": True, "battery_low": True, "mode": "manual"}, battery="weak")
    if told["b"]:
        reads.device(told["b"], state={"mode": "auto"})
    name = page.evaluate("window.__room")
    room = page.locator(".room", has=page.locator(".room-name", has_text=name)).first
    two = page.evaluate("state.rooms.find((r) => r.name === window.__room).devices.filter((d) => isThermostat(d)).length >= 2")
    icons = head_shows(room, lambda i: i[:2] == ["#icon-radio-small+#mark-corner-x", "#icon-battery"] and (not two or "#icon-shuffle" in i),
                       "radio with its cross, then battery, and the mixed modes where two differ, radio in red", tone="tone-bad")
    open_list(room, """(d) => { const e = d.querySelector('.problem-entry'); return !!e && e.innerText.includes(t('badge.communicationError'))
      && e.innerText.includes(t('badge.batteryWeak')); }""", "the device with its badges")
    entries = room.locator(".tile-head .problem-entry")
    first = entries.first.inner_text()
    assert page.evaluate("t('badge.communicationError')") in first and page.evaluate("t('badge.batteryWeak')") in first, \
        f"the device with its badges ({first!r})"
    bad = entries.first.locator(".flag.bad").first
    assert bad.locator("svg.mark-sign use").get_attribute("href") == "#mark-x", "a failure led by its cross"
    assert int(bad.evaluate("(e) => getComputedStyle(e).fontWeight")) >= 600, "and bold"
    assert bad.locator("use[href='#icon-radio']").count() == 0, "the cross in place of the radio's icon"
    warn = entries.first.locator(".flag.warn").first
    assert warn.locator("svg.mark-sign use").get_attribute("href") == "#mark-warn", "a warning led by its triangle"
    names = [e.locator(".problem-name").inner_text() for e in entries.all()]
    assert len(names) == len(set(names)), f"each device once ({names})"
    listed = entries.count()
    page.keyboard.press("Escape")
    room.locator(".room-name-btn").click()
    page.wait_for_selector(".room .device", timeout=5000)
    for dev in room.locator(".device:not(.virtual)").all():
        assert dev.locator(".device-type-icon[aria-label] svg use").count() == 1, "every device its type's icon"
    sick = room.locator(".device:not(.virtual)", has=page.locator(".problems-sum")).first
    ok, got = settled(page, lambda: sign_icons(sick.locator(".problems-sum")), lambda g: g == ["#icon-radio-small+#mark-corner-x", "#icon-battery"])
    assert ok, f"the device's problems as icons: {got}"
    assert sick.locator(".device-badges > .flag").count() >= 1, "its mode a word"
    # the virtual thermostat with the wall thermostat's flags: not listed
    page.evaluate("""() => {
      const r = state.rooms.find((r) => r.name === window.__room);
      if (!r.vwt) return;
      const v = JSON.parse(JSON.stringify(r.vwt));
      Object.assign(v.state, { error: true });
      updateDevice(v);
    }""")
    if told["vwt"]:
        reads.device(told["vwt"], state={"error": True})
    open_list(room, "(d) => d.querySelectorAll('.problem-entry').length === %d" % listed, "the virtual thermostat not listed")
    page.keyboard.press("Escape")
    room.locator(".room-name-btn").click()


def test_stale_and_expert(page, case, reads):
    """The first room's devices made 30 hours old and stale: the clock
    among the tile's pill's icons, no line on the tile; the pill lists each
    device not seen for 30 hours; the values are greyed and the device list
    says so; expert mode
    adds the type, the radio address, the exact times and the Cube's
    radio budget in the header."""
    iso = page.evaluate("new Date(Date.now() - 30 * 3600 * 1000).toISOString()")

    def stale(room):
        # its devices, the virtual wall thermostat among them, as the
        # server makes that stale when every source of its temperature is
        room["stale"] = True
        for d in room.get("devices") or []:
            d["stale"] = True
            d["last_seen"] = iso
            for k in d.get("received") or {}:
                d["received"][k] = iso

    reads.change(ROOMS, change_room(page.evaluate("state.rooms[0].id"), stale))
    # its devices and its virtual wall thermostat kept: the server's own
    # events of them bring them fresh
    for rf in page.evaluate("(() => { const r = state.rooms[0]; return r.devices.map((d) => d.rf_address).concat(r.vwt ? [r.vwt.rf_address] : []); })()"):
        reads.keep(rf)
    page.evaluate("loadRooms()")
    tile = page.locator(".room").first
    tile.locator(".room-problems .problems-sum use[href='#icon-set-dial']").wait_for(state="attached", timeout=5000)
    assert tile.locator(".tile-notices").is_hidden(), "no line on the tile: the room's commands are not refused"
    open_list(tile, "(d) => d.querySelector('.problems-list').innerText.includes(%s)" % json.dumps(STALE[case.lang]),
              "the pill lists the devices not seen for long")
    page.keyboard.press("Escape")
    assert tile.locator(".temp-val.stale").count() == 1, "stale measured temperature greyed"
    assert "30" in tile.locator(".temp-age").inner_text(), "the measured temperature is 30 hours old"
    tile.locator(".room-name-btn").click()
    assert STALE[case.lang] in tile.locator(".device .stale-badge").first.inner_text(), "stale badge in the device list"
    assert tile.locator(".device-vals.stale .age").count() > 0, "ages of the stale values in the device list"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the ages"
    assert tile.locator(".device .device-sub").first.is_hidden(), "no type and radio address outside expert mode"
    page.evaluate("() => { state.settings.expert = true; applySettings(); }")
    assert tile.locator(".device .device-rf").first.is_visible(), "radio address in expert mode"
    ids = tile.locator(".device .device-rf").first.inner_text()
    assert ids.startswith("S/N ") and " · RF 0a" in ids, f"the serial beside the radio address in expert mode ({ids!r})"
    assert tile.locator(".device-vals .age-exact").first.is_visible(), "exact times in expert mode"
    geo = page.evaluate("""() => {
      const cell = document.querySelector('.room .device-vals .cell.aged');
      const r = (sel) => cell.querySelector(sel).getBoundingClientRect();
      const [k, num, text, exact] = [r('.k'), r('.num'), r('.age-text'), r('.age-exact')];
      return { k: k.left, kBottom: k.bottom, num: num.left, text: text.left, textTop: text.top, exact: exact.left,
               exactTop: exact.top, exactH: exact.height, line: parseFloat(getComputedStyle(cell.querySelector('.age-exact')).lineHeight) };
    }""")
    assert abs(geo["text"] - geo["k"]) <= 1 and geo["textTop"] >= geo["kBottom"] - 2, f"how old under the value's name ({geo})"
    assert abs(geo["exact"] - geo["num"]) <= 1 and abs(geo["exactTop"] - geo["textTop"]) <= 2, f"the time under the value ({geo})"
    assert geo["exactH"] <= geo["line"] * 1.5, f"on one line ({geo})"
    virtual = tile.locator(".device.virtual .device-seen", has_text=page.evaluate("t('vwt.from', {devices: ''})").strip())
    if virtual.count():
        text = virtual.first.inner_text()
        assert re.search(r"[A-Z]{3}[0-9]{7}", text) and not re.search(r"\b0a0[0-9a-f]{3}\b", text), \
            f"the virtual thermostat's sources by serial ({text!r})"
    assert tile.locator(".device.virtual .device-type-icon use").first.get_attribute("href") == "#icon-room", \
        "the virtual thermostat's icon a room's"
    assert tile.locator(".device-seen").first.is_visible(), "last seen in expert mode"
    stats = page.locator("#cube-stats")
    assert stats.is_visible() and page.locator("#stats-cube").inner_text() == page.evaluate(
        "(() => { const c = state.cubes.find((x) => x.serial === state.serial); return c.name || c.serial; })()"), \
        "the Cube's name and its radio budget in the header in expert mode"
    assert scroll_width(page) <= case.width, "no horizontal scroll in expert mode"
    page.evaluate("() => { state.settings.expert = false; applySettings(); }")
    tile.locator(".room-name-btn").click()


# A room's open device list as it is laid out, by each row shown with an
# aged value (its radio address): where the row ends, where the last value
# of its right column ends, where anything of its values does, and each
# cell with an age: its width, its age's text, whether that shows whole
# inside the cell, and how far its exact time starts from its value, where
# expert mode shows it.
AGED_ROWS = """(sel) => {
  const box = (el) => el.getBoundingClientRect();
  const r1 = (v) => Math.round(v * 10) / 10;
  const ends = (el) => r1(Math.max(...[...el.querySelectorAll('*')].map(box).filter((b) => b.width > 0).map((b) => b.right)));
  const rows = {};
  for (const row of document.querySelectorAll(sel + ' .devices .device')) {
    const vals = row.querySelector('.device-vals');
    if (!box(row).width || !vals || !vals.querySelector('.cell.aged')) continue;
    const cells = [...vals.querySelectorAll(':scope > .cell:not(.wide)')];
    const right = cells.filter((c, i) => i % 2 === 1 && c.querySelector('.num').textContent !== '');
    rows[row.dataset.rf] = { edge: r1(box(row).right), end: ends(right[right.length - 1]), out: ends(vals),
      aged: [...vals.querySelectorAll(':scope > .cell.aged')].map((c) => {
        const a = c.querySelector('.age-text'), x = c.querySelector('.age-exact');
        return { width: r1(box(c).width), text: a.textContent,
                 whole: a.scrollWidth <= a.clientWidth + 1 && box(a).right <= box(c).right + 0.5,
                 under: x && box(x).width ? r1(box(x).left - box(c.querySelector('.num')).left) : null };
      }) };
  }
  return rows;
}"""

# How old the values are told, in seconds, and whether they were already
# there when the server connected ("≥", as the server's own are): now, then
# a two-digit age, which the server's reach once a run has lasted ten
# minutes, and the longest of each unit, minutes, hours, and days to three
# digits.
AGES = [(0, False), (17.5 * 60, True), (59.5 * 60, True), (47.5 * 3600, True), (999.5 * 86400, True)]


def test_an_age_does_not_widen_its_cell(page, case, reads):
    """How old a value is grows all day, and once it has two digits
    ("≥ 17 Min.") it widened its cell, so that the device list's right
    column left the row in German in a tile of three columns at 1100 px,
    narrower than a phone's (AP SteadyTests v1.1, Block 14). Told two
    digits and the longest of each unit, every aged cell of the room's
    open device list is as wide as with "now", the right column's values
    end at the row's edge, nothing of the values leaves the row, and each
    age shows whole inside its cell, under the value's name. In expert
    mode, beside the exact time under the value, an age may take room the
    row has to spare, and wraps where there is none: the values still end
    at the row's edge and the age shows whole."""
    rid = page.evaluate("state.rooms.find(roomThermostat).id")
    sel = '.room[data-id="%s"]' % rid
    tile = page.locator(sel)
    tile.locator(".room-name-btn").click()
    page.wait_for_selector(sel + " .device .device-vals .cell.aged", timeout=5000)
    try:
        for expert in (False, True):
            page.evaluate("(x) => { state.settings.expert = x; applySettings(); }", expert)
            widths = None
            for seconds, already in AGES:
                want = reads.age(rid, seconds, already)
                ok, rows = settled(page, lambda: page.evaluate(AGED_ROWS, sel),
                                   lambda g: bool(g) and all(c["text"] == want for r in g.values() for c in r["aged"]))
                assert ok, f"expert {expert}: every age says {want!r}: {rows}"
                got = {rf: [c["width"] for c in r["aged"]] for rf, r in rows.items()}
                widths = widths or got
                assert expert or got.keys() == widths.keys() and all(
                    abs(a - b) <= 0.5 for rf in got for a, b in zip(got[rf], widths[rf])), \
                    f"{want!r} widens no cell: {got}, with {widths} as now"
                for rf, r in rows.items():
                    assert abs(r["edge"] - r["end"]) <= 2 and r["out"] <= r["edge"] + 2, \
                        f"expert {expert}, {want!r}: the values of {rf} end at the row's edge: {r}"
                    assert all(c["whole"] for c in r["aged"]), f"expert {expert}: {want!r} whole in its cell: {r}"
                    assert all((c["under"] is not None and abs(c["under"]) <= 1) == expert for c in r["aged"]), \
                        f"expert {expert}, {want!r}: in expert mode the exact time under the value: {r}"
    finally:
        page.evaluate("() => { state.settings.expert = false; applySettings(); }")
        tile.locator(".room-name-btn").click()


def test_eco_button(page, case):
    """The greeting's eco button, typed eco_switch by the API: its row in
    the device list carries the leaf and its type's name, in expert mode
    also as text."""
    room = page.evaluate("""(() => {
      const r = state.rooms.find((r) => r.devices.some((d) => d.rf_address === "0a010e"));
      return r && { name: r.name, type: r.devices.find((d) => d.rf_address === "0a010e").type };
    })()""")
    assert room and room["type"] == "eco_switch", f"the eco button in a room, as the API types it ({room})"
    tile = page.locator(".room", has=page.locator(".room-name", has_text=room["name"])).first
    tile.locator(".room-name-btn").click()
    dev = tile.locator(".device[data-rf='0a010e']")
    dev.wait_for(state="visible", timeout=5000)
    icon = dev.locator(".device-type-icon")
    assert icon.get_attribute("aria-label") == ECO_BUTTON[case.lang], f"its type's name ({icon.get_attribute('aria-label')!r})"
    assert icon.get_attribute("title") == ECO_BUTTON[case.lang], "and as the icon's title"
    assert icon.locator("svg use").get_attribute("href") == "#icon-leaf", "the leaf as its icon"
    page.evaluate("() => { state.settings.expert = true; applySettings(); }")
    assert dev.locator(".device-type").inner_text() == ECO_BUTTON[case.lang], "its type in words in expert mode"
    page.evaluate("() => { state.settings.expert = false; applySettings(); }")
    tile.locator(".room-name-btn").click()


def test_typing_in_the_configuration_survives_a_state_event(page, case):
    """A state event for a room whose configuration has the focus takes
    nothing from the user's typing: every key typed is in the name, the
    state is taken all the same, and the tile is the one it was, also once
    the focus has left the configuration, which still shows. Nothing is
    sent."""
    tile = page.locator(".room").first
    tile.locator(".tile-cog").click()
    page.wait_for_selector(".room .config-body", timeout=5000)
    tile.locator(".config-tabs [role=tab][data-tab=devices]").click()
    name = tile.locator(".config-area .cfg-device input[type=text]").first
    name.fill("Typed")
    page.evaluate("document.querySelector('.room').dataset.mark = 'before'")
    page.evaluate("""() => {
      const d = JSON.parse(JSON.stringify(state.rooms[0].devices[0]));
      d.state = Object.assign({}, d.state, { valve_pct: 77 });
      state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({
        kind: "state", device: d.rf_address, state: d, time: new Date().toISOString() }) }));
    }""")
    page.keyboard.type(" on")
    assert name.input_value() == "Typed on", "every key typed is in the name"
    assert page.evaluate("document.querySelector('.room').dataset.mark") == "before", "the tile is not drawn again while typed in"
    assert page.evaluate("state.rooms[0].devices[0].state.valve_pct") == 77, "the state is taken all the same"
    page.evaluate("document.activeElement.blur()")
    page.wait_for_timeout(200)  # the configuration is filled again once the focus has left it
    assert page.evaluate("document.querySelector('.room').dataset.mark") == "before", "the tile is the one it was once the focus left"
    assert tile.locator(".config-body").is_visible(), "the configuration still shows"
    tile.locator(".config-tabs .close").click()
    assert tile.locator(".config-body").count() == 0, "the configuration closes, nothing sent"


def test_a_switch_point_takes_a_full_time(page, case):
    """The programme's time input keeps the focus while a time is typed
    part by part (Chrome fires change at each part), and Send right after
    it sends the time; the PUT is stubbed, nothing reaches the Cube."""
    tile = page.locator(".room").first
    tile.locator(".tile-cog").click()
    page.wait_for_selector(".room .config-body", timeout=5000)
    tile.locator(".config-tabs [role=tab][data-tab=program]").click()
    time_input = tile.locator(".prog-point input[type=time]").first
    time_input.wait_for()
    sent = []

    def answer(route):
        sent.append(route.request.post_data_json)
        route.fulfill(status=200, content_type="application/json", body="{}")

    page.route("**/rooms/*/program", answer)
    try:
        before = time_input.input_value()
        time_input.focus()  # the hour part
        page.keyboard.type("0115A")  # headless Chromium shows 12 hours and AM/PM in both languages
        assert time_input.input_value() == "01:15", f"the full time typed, from {before}"
        assert "dirty" in tile.locator(".prog-days .day.editing").get_attribute("class") and \
            tile.locator(".prog-reset-day").is_enabled(), "the day is marked as changed in place"
        tile.locator(".config-body button[type=submit]").click()
        assert wait_held(page, sent), "Send right after the time sends it"
        points = next(iter(sent[0].values()))
        assert points[0]["until"] == "01:15", f"the time sent: {points}"
    finally:
        page.unroute("**/rooms/*/program")
    tile.locator(".config-tabs .close").click()


def test_virtual_wall_thermostat(page, case):
    """The tile's temperature is the room's virtual wall thermostat's;
    expert mode lists it in the device list, with its settings, and
    outside expert mode it is not shown."""
    room = page.evaluate("() => { const r = state.rooms[0]; return {id: r.id, vwt: r.vwt, real: r.devices.some((d) => d.virtual)}; }")
    assert room["vwt"] and room["vwt"]["virtual"], "the room has its virtual wall thermostat"
    assert not room["real"], "the room's devices are the real ones"
    tile = page.locator(".room").first
    actual = (room["vwt"].get("state") or {}).get("actual_temp")
    if actual is not None:
        shown = tile.locator(".temp-is").text_content().replace(",", ".")
        assert ("%.1f" % actual) in shown, f"the tile shows the room's temperature {actual} ({shown!r})"
    tile.locator(".room-name-btn").click()
    page.wait_for_selector(".room .device", timeout=5000)
    assert tile.locator(".device.virtual").is_hidden(), "not shown outside expert mode"
    page.evaluate("() => { state.settings.expert = true; applySettings(); }")
    virtual = tile.locator(".device.virtual")
    assert virtual.is_visible(), "listed in expert mode"
    assert wait_until(page, "document.querySelector('.room .device.virtual select[name=temp_method]').value === 'median'", 5), "its setting loaded"
    assert virtual.locator("input[name=rf_address]").get_attribute("placeholder") == room["vwt"]["rf_address"], "its address as the placeholder"
    named_buttons(page, "the virtual wall thermostat")
    page.evaluate("() => { state.settings.expert = false; applySettings(); }")


def test_battery_levels(page, case, reads):
    """The battery's level, the server's (AP LegacyLogs v1.0, Block 11):
    weak in amber with its word, low in red with its own; the flag alone
    of a device that has a level shows nothing. Told to the rooms' read
    and kept (test_new_batteries.py's Device), the tile's icon waited for
    and its list opened again until it says the level."""
    dev = Device(page, reads)
    room = dev.tile

    def battery_tone(level, flag, want):
        """The level and the flag told, and the tile head's battery icon
        waited for: its tone want, or none where want is None."""
        dev.told(level, flag)

        def look():
            icons = room.locator(".tile-head .problems-sum svg", has=page.locator("use[href='#icon-battery']"))
            return icons.first.get_attribute("class", timeout=1000) or "" if icons.count() else None
        ok, got = settled(page, look, lambda g: g is None if want is None else g is not None and want in g)
        assert ok, f"{level}, flag {flag}: the battery's icon {want}: {got}"

    battery_tone("weak", False, "tone-warn")
    open_list(room, "(d) => !!d.querySelector('.problem-entry') && d.querySelector('.problem-entry').innerText.includes(t('badge.batteryWeak'))",
              "weak's word")
    page.keyboard.press("Escape")
    battery_tone("low", True, "tone-bad")
    open_list(room, "(d) => !!d.querySelector('.problem-entry') && d.querySelector('.problem-entry').innerText.includes(t('badge.batteryLow'))",
              "low's word")
    page.keyboard.press("Escape")
    battery_tone("ok", True, None)


def test_out_of_date_device(page, case, reads):
    """A device the Cube no longer hears keeps the values of its last
    report on its tile, greyed, with the badge out of date in place of
    not reported (AP LegacyLogs v1.0, Block 13). Told to the rooms' read
    as well, the device kept, and its row waited for in one look."""
    told = page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.some((d) => isThermostat(d) && !d.virtual && d.reported));
      window.__room = r.name;
      const d = JSON.parse(JSON.stringify(r.devices.find((d) => isThermostat(d) && !d.virtual && d.reported)));
      window.__dev = JSON.parse(JSON.stringify(d));
      window.__name = d.name;
      d.reported = false;
      d.out_of_date = true;
      d.last_known = Object.assign({}, d.state, { setpoint: 18.5 });
      delete d.state;
      delete d.battery;
      updateDevice(d);
      return { rf: d.rf_address, last_known: d.last_known };
    }""")
    reads.device(told["rf"], reported=False, out_of_date=True, last_known=told["last_known"], state=None, battery=None)
    name = page.evaluate("window.__room")
    room = page.locator(".room", has=page.locator(".room-name", has_text=name)).first
    room.locator(".room-name-btn").click()
    page.wait_for_selector(".room .device", timeout=5000)
    dev = room.locator(".device:not(.virtual)", has=page.locator(".device-name", has_text=page.evaluate("window.__name"))).first
    out_of_date, not_reported = page.evaluate("[t('badge.outOfDate'), t('badge.notReported')]")
    ok, got = settled(page, lambda: dev.evaluate("""(r) => ({ badges: r.querySelector('.device-badges').innerText,
      cls: r.querySelector('.device-vals').getAttribute('class') || '', vals: r.querySelector('.device-vals').innerText })""", timeout=1000),
                      lambda g: out_of_date in g["badges"] and not_reported not in g["badges"] and "out-of-date" in g["cls"] and "18" in g["vals"])
    assert ok, f"the badge out of date, not 'not reported'; the values greyed, the last report's setpoint: {got}"
    page.evaluate("() => updateDevice(window.__dev)")


def test_not_ready_line(page, case, reads):
    """A room whose devices have not all reported since the Cube started:
    the tile's line says that its commands are discarded, Show opens its
    pill, which lists the room's own notice and the device not reported,
    the pause sign among its icons; a device the Cube no longer hears is
    out of date there with its age, as its row says; the line goes once
    the rooms are the server's again."""
    names = page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.some((d) => isThermostat(d) && !d.virtual && d.reported));
      const d = r.devices.find((d) => isThermostat(d) && !d.virtual && d.reported);
      return { id: r.id, room: r.name, rf: d.rf_address, seen: new Date(Date.now() - 600e3).toISOString() };
    }""")
    told = {"out_of_date": False}

    def unreported(room):
        room["ready"] = False
        for d in room.get("devices") or []:
            if d.get("rf_address") == names["rf"]:
                d["reported"] = False
                if told["out_of_date"]:
                    d["out_of_date"] = True
                    d["last_seen"] = names["seen"]

    reads.change(ROOMS, change_room(names["id"], unreported))
    reads.keep(names["rf"])  # the server's own events of it say it reported, and the room ready
    page.evaluate("loadRooms()")
    tile = page.locator(".room", has=page.locator(".room-name", has_text=names["room"])).first
    line = tile.locator(".tile-notices")
    not_ready = page.evaluate("t('room.notReady')")
    line.wait_for(state="visible", timeout=5000)
    assert line.inner_text().startswith(not_ready), f"the line says the commands are discarded: {line.inner_text()!r}"
    line.locator(".tile-notices-show").click()
    listed = tile.locator(".room-problems .problems-list")
    listed.wait_for(state="visible")
    open_list(tile, """(d) => { const text = d.querySelector('.problems-list').innerText; return text.includes(%s) && text.includes(t('badge.notReported'))
      && d.querySelectorAll(":scope > summary use[href='#icon-paused']").length === 1; }""" % json.dumps(not_ready),
              "the room's own notice and the device not reported in the pill, the pause sign among its icons")
    page.keyboard.press("Escape")
    told["out_of_date"] = True
    page.evaluate("loadRooms()")
    line.locator(".tile-notices-show").click()
    listed.wait_for(state="visible")
    assert wait_until(page, "[...document.querySelectorAll('.room-problems .problems-list')]"
                      ".some((l) => l.innerText.includes(t('badge.outOfDate')))", 5), "out of date, as the device's row says"
    assert listed.locator(".flag .age").count() > 0, "with its age"
    page.keyboard.press("Escape")
    reads.clear(ROOMS)
    page.evaluate("loadRooms()")
    line.wait_for(state="hidden", timeout=5000)


MISMATCH_NOTE = {
    "en": "{device}: in {cube} by its configuration on the Cube, in {list} in the room list.",
    "de": "{device}: laut seiner Konfiguration im Cube in {cube}, in der Raumliste in {list}.",
}
MISMATCH_BADGE = {"en": "in {cube} for the Cube", "de": "in {cube} laut Cube"}
MISMATCH_PROGRAMME = {"en": "own week programme", "de": "eigenes Wochenprogramm"}


def test_mismatch_marks(page, case, reads):
    """A device whose C line names another room than the room list, and a
    member whose week programme differs (mismatches, as the server sends
    them; AP RadioHealth, Block 7): the sliders among the tile's pill's
    icons, no line on the tile; in the pill the device's entry has a badge
    for each, with since when, the whole sentence naming the device and both rooms its tooltip;
    the device's row has the same badges; the marks go with the
    findings."""
    names = page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.some((d) => isThermostat(d) && !d.virtual));
      const other = state.rooms.find((x) => x.id !== r.id);
      const d = r.devices.find((d) => isThermostat(d) && !d.virtual);
      const since = new Date(Date.now() - 600e3).toISOString();
      const moved = { finding: "room", device: d.rf_address, name: d.name, room: r.id, room_name: r.name,
        cube_room: other.id, cube_room_name: other.name, list_room: r.id, list_room_name: r.name, since };
      const programme = { finding: "programme", device: d.rf_address, name: d.name, room: r.id, room_name: r.name, since };
      return { id: r.id, rf: d.rf_address, findings: [moved, programme], room: r.name, other: other.name, device: deviceName(d) };
    }""")

    def found(room):
        room["mismatches"] = names["findings"]
        for d in room.get("devices") or []:
            if d.get("rf_address") == names["rf"]:
                d["mismatches"] = names["findings"]

    reads.change(ROOMS, change_room(names["id"], found))
    reads.keep(names["rf"])  # the server's own events of it bring it without its findings
    page.evaluate("loadRooms()")
    tile = page.locator(".room", has=page.locator(".room-name", has_text=names["room"])).first
    sliders = tile.locator(".room-problems .problems-sum use[href='#icon-sliders']")
    sliders.wait_for(state="attached", timeout=5000)
    assert tile.locator(".tile-notices").is_hidden(), "no line on the tile: the room's commands are not refused"
    tile.locator(".room-problems .problems-sum").click()
    tile.locator(".room-problems .problems-list").wait_for(state="visible")
    entry = tile.locator(".room-problems .problems-list .problem-entry", has=page.locator(".problem-name", has_text=names["device"])).first
    flags = entry.locator(".flag", has=page.locator(".badge-since"))
    assert flags.count() == 2, "a badge per finding, each with since when"
    want = MISMATCH_NOTE[case.lang].format(device=names["device"], cube=names["other"], list=names["room"])
    first = flags.first
    assert first.inner_text().startswith(MISMATCH_BADGE[case.lang].format(cube=names["other"])), f"the row's words: {first.inner_text()!r}"
    assert first.get_attribute("title") == want, f"the whole sentence its tooltip: {first.get_attribute('title')!r}"
    page.keyboard.press("Escape")
    tile.locator(".room-name-btn").click()
    badge = tile.locator(".device .mismatch-badge").first
    assert badge.inner_text().strip().endswith(MISMATCH_BADGE[case.lang].format(cube=names["other"])), \
        f"the device's badge: {badge.inner_text()!r}"
    assert badge.get_attribute("title") == want, "the badge's title the whole sentence"
    assert MISMATCH_PROGRAMME[case.lang] in tile.locator(".device .device-badges").first.inner_text(), "the programme's badge"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the badges"
    reads.clear(ROOMS)
    page.evaluate("loadRooms()")
    sliders.wait_for(state="detached", timeout=5000)
    assert tile.locator(".device .mismatch-badge").count() == 0, "the device's badges gone with the findings"


def test_an_age_moves_on_with_the_minute(page, case):
    """A value's age in a tile's device list follows the page's minute,
    a signal of the parts (MassimoParts.minute), where a plain timer wrote
    into the parts' ages before: an hour on, it says so at the next
    minute."""
    tile = page.locator(".room").first
    tile.locator(".room-name-btn").click()
    age = tile.locator(".device-vals .age .age-text").first
    try:
        age.wait_for()
        was = age.inner_text()
        page.evaluate("""() => {
          window.__realNow = Date.now;
          Date.now = () => window.__realNow() + 3600e3;
          MassimoParts.minute();
        }""")
        assert wait_until(page, "document.querySelector('.room .device-vals .age .age-text').textContent !== %s" % json.dumps(was), 5), \
            f"the age moves on with the minute: {was}"
    finally:
        page.evaluate("() => { if (window.__realNow) Date.now = window.__realNow; if (MassimoParts.minute) MassimoParts.minute(); }")
        tile.locator(".room-name-btn").click()
