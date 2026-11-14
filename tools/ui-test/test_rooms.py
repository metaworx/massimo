# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The Rooms page: the tiles, a tile's device list and its
configuration; the problems in a tile's head and in a device's; the ages
of the values, a stale device, and what expert mode adds.

The window and the staleness come from state events applied in the page
(updateDevice), since the fake Cube does not change its states.
"""
import re

from helpers import OPEN, STALE, named_buttons, scroll_width, wait_until


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


# Gives the open window contact the times of a window open for `ago`
# minutes, by the server's limits of 5 and 10 minutes.
WINDOW_TIMES = """(ago) => {
  const r = state.rooms.find((r) => r.devices.some((d) => d.type === "shutter_contact"));
  const d = JSON.parse(JSON.stringify(r.devices.find((d) => d.type === "shutter_contact")));
  const since = Date.now() - ago * 60000;
  const iso = (m) => new Date(since + m * 60000).toISOString();
  d.state.window_open = true;
  d.window = { open_since: iso(0), warn_at: iso(5), alert_at: iso(10) };
  updateDevice(d);
}"""


def window_badge(room):
    """The class and the leading mark of the open window's badge in the
    room's list."""
    room.locator(".tile-head .problems-sum").click()
    flag = room.locator(".tile-head .problem-entry .flag").first
    out = flag.get_attribute("class"), [u.get_attribute("href") for u in flag.locator("svg.mark-sign use").all()]
    room.page.keyboard.press("Escape")
    return out


def test_open_window(page, case):
    """The room with the open window contact shows the open window in its
    tile's head, which lists the contact, open, with its age: blue and
    plain for its first five minutes, amber with a triangle up to ten,
    red with a cross after; the icon goes when a state event closes the
    contact; the measured temperature shows its age."""
    room = page.locator(".room", has=page.locator(".room-name", has_text="Kind 1"))
    page.evaluate(WINDOW_TIMES, 1)
    assert head_icons(room) == ["#icon-window-open"], "open briefly: the open window, plain"
    assert "tone-open" in room.locator(".tile-head .problems-sum svg").first.get_attribute("class"), "in blue"
    assert window_badge(room) == ("flag fill-open", []), "its badge filled blue, no mark"
    page.evaluate(WINDOW_TIMES, 6)
    assert head_icons(room) == ["#icon-window-open-small+#mark-corner-warn"], "open long: amber, a triangle"
    assert window_badge(room) == ("flag warn", ["#mark-warn"]), "its badge an amber outline led by a triangle"
    page.evaluate(WINDOW_TIMES, 11)
    assert head_icons(room) == ["#icon-window-open-small+#mark-corner-x"], "open too long: red, a cross"
    assert window_badge(room) == ("flag fill-bad", ["#mark-x"]), "its badge filled red, led by a cross"
    page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.some((d) => d.type === "shutter_contact"));
      const d = JSON.parse(JSON.stringify(r.devices.find((d) => d.type === "shutter_contact")));
      d.window = { open_since: new Date(Date.now() - 299500).toISOString(), warn_at: new Date(Date.now() + 500).toISOString(),
        alert_at: new Date(Date.now() + 300500).toISOString() };
      updateDevice(d);
    }""")
    assert head_icons(room) == ["#icon-window-open"], "half a second before the warning"
    assert wait_until(page, "document.querySelector('.room .tile-head .problems-sum use[href=\"#mark-corner-warn\"]') !== null", 3), \
        "the tile turns amber by the clock alone"
    summary = room.locator(".tile-head .problems-sum")
    assert OPEN[case.lang] in (summary.get_attribute("aria-label") or "").lower(), "named to a screen reader"
    assert page.locator(".room").first.locator(".problems").count() == 0, "nothing in the head of a room with closed windows"
    summary.click()
    entry = room.locator(".tile-head .problem-entry")
    assert entry.count() == 1 and entry.locator(".flag").inner_text().startswith(OPEN[case.lang]), "the contact, open"
    assert entry.locator(".flag .age").count() == 1, "with the opening's age"
    page.keyboard.press("Escape")
    assert room.locator(".tile-head details.problems").get_attribute("open") is None, "Escape closes the list"
    assert page.locator(".room .temp-age .age").count() > 0, "age next to the measured temperature"
    page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.some((d) => d.type === "shutter_contact"));
      const d = JSON.parse(JSON.stringify(r.devices.find((d) => d.type === "shutter_contact")));
      d.state.window_open = false;
      updateDevice(d);
    }""")
    assert head_icons(room) == [], "the icon goes when the window closes"


def test_problems_in_the_heads(page, case):
    """A radiator thermostat with a transmit error and a low battery, and
    two thermostats in different modes: the room's head shows radio,
    battery and the mixed modes, radio in red with a cross; the list names the devices,
    each once, with their badges, a failure bold and led by its cross in
    place of its kind's icon, a warning by its triangle; the device's head shows its problems as
    icons, its mode a word, and every device its type's icon; the room's
    virtual thermostat, whose flags copy the wall thermostat's, is not
    listed."""
    page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.filter((d) => isThermostat(d)).length >= 2) || state.rooms[0];
      window.__room = r.name;
      const [a, b] = r.devices.filter((d) => isThermostat(d));
      const da = JSON.parse(JSON.stringify(a));
      Object.assign(da.state, { error: true, battery_low: true, mode: "manual" });
      updateDevice(da);
      if (b) {
        const db = JSON.parse(JSON.stringify(b));
        db.state.mode = "auto";
        updateDevice(db);
      }
    }""")
    name = page.evaluate("window.__room")
    room = page.locator(".room", has=page.locator(".room-name", has_text=name)).first
    icons = head_icons(room)
    assert icons[:2] == ["#icon-radio-small+#mark-corner-x", "#icon-battery"], f"radio with its cross, then battery ({icons})"
    two = page.evaluate("state.rooms.find((r) => r.name === window.__room).devices.filter((d) => isThermostat(d)).length >= 2")
    if two:
        assert "#icon-shuffle" in icons, f"the mixed modes ({icons})"
    radio = room.locator(".tile-head .problems-sum svg").first
    assert "tone-bad" in (radio.get_attribute("class") or ""), "radio in red"
    room.locator(".tile-head .problems-sum").click()
    entries = room.locator(".tile-head .problem-entry")
    first = entries.first.inner_text()
    assert page.evaluate("t('badge.transmitError')") in first and page.evaluate("t('badge.batteryLow')") in first, \
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
    assert sign_icons(sick.locator(".problems-sum")) == ["#icon-radio-small+#mark-corner-x", "#icon-battery"], \
        "the device's problems as icons"
    assert sick.locator(".device-badges > .flag").count() >= 1, "its mode a word"
    # the virtual thermostat with the wall thermostat's flags: not listed
    page.evaluate("""() => {
      const r = state.rooms.find((r) => r.name === window.__room);
      if (!r.vwt) return;
      const v = JSON.parse(JSON.stringify(r.vwt));
      Object.assign(v.state, { error: true });
      updateDevice(v);
    }""")
    room.locator(".tile-head .problems-sum").click()
    assert room.locator(".tile-head .problem-entry").count() == listed, "the virtual thermostat not listed"
    page.keyboard.press("Escape")
    room.locator(".room-name-btn").click()


def test_stale_and_expert(page, case):
    """The first room's devices made 30 hours old and stale: the tile
    warns, the values are greyed and the device list says so; expert mode
    adds the type, the radio address, the exact times and the Cube's
    radio budget in the header."""
    page.evaluate("""() => {
      const iso = new Date(Date.now() - 30 * 3600 * 1000).toISOString();
      for (const x of state.rooms[0].devices) {
        const d = JSON.parse(JSON.stringify(x));
        d.stale = true;
        d.last_seen = iso;
        for (const k of Object.keys(d.received || {})) d.received[k] = iso;
        updateDevice(d);
      }
      // The room's virtual wall thermostat, as the server makes it stale
      // when every source of its temperature is.
      const v = JSON.parse(JSON.stringify(state.rooms[0].vwt));
      v.stale = true;
      for (const k of Object.keys(v.received || {})) v.received[k] = iso;
      updateDevice(v);
    }""")
    tile = page.locator(".room").first
    note = tile.locator(".stale-note")
    assert note.is_visible() and STALE[case.lang] in note.inner_text(), "stale warning on the tile"
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


def test_typing_in_the_configuration_survives_a_state_event(page, case):
    """A state event for a room whose configuration has the focus waits:
    the tile is not drawn again under the user's typing, and is once the
    focus leaves the configuration, with the new state. Nothing is sent."""
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
    assert wait_until(page, "document.querySelector('.room').dataset.mark === undefined", 5), "the tile is drawn once the focus leaves"
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
        page.wait_for_timeout(500)
        assert sent, "Send right after the time sends it"
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
