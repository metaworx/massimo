# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Writes the devices have not confirmed (AP RoomConfirm v1.0, Block 3):
a device that has not shown a write by its deadline is marked amber in
its tile's head and in its row, the title saying what it does not show
and what it shows, nothing while the write is pending; its form in the
configuration's devices tab says which values; the log's line names the
write, the room and each device with what it shows.

The confirmations come from state events applied in the page
(updateDevice) and from events sent on the stream, since the fake Cube
takes every write.
"""
from helpers import isolate_log, scroll_width, show_view, wait_log_live, wait_until

# Gives the first reported thermostat of a room the confirmation cstate
# (pending or unconfirmed, None for none) of fields, auto at 20 °C.
CONFIRMATION = """([cstate, fields]) => {
  const r = state.rooms.find((r) => r.devices.some((d) => isThermostat(d) && d.reported && !d.virtual));
  if (!r) return null;
  const d = JSON.parse(JSON.stringify(r.devices.find((d) => isThermostat(d) && d.reported && !d.virtual)));
  Object.assign(d.state, { mode: "auto", setpoint: 20 });
  if (cstate) d.confirmation = { state: cstate, since: new Date().toISOString(), fields };
  else delete d.confirmation;
  updateDevice(d);
  return { id: r.id, room: r.name, name: deviceName(d), rf: d.rf_address };
}"""

SAYS = {
    "en": {
        "badge": "not confirmed",
        "title": "The device does not show the last write: mode (shows auto), setpoint (shows 20.0 °C).",
        "form": "Not confirmed by the device: eco, Monday's program.",
        "kind": "not confirmed",
        "line": "setpoint to {room}: not confirmed after 5 min: {name} (mode auto, setpoint 20.0)",
        "late": "setpoint: confirmed after 70 s",
        "resent": "setpoint to {room}: sent once more to its thermostats",
        "resend_not": "setpoint to {room}: sent once more, still not confirmed after 5 min: {name} (mode auto, setpoint 20.0)",
        "not_resent": "setpoint to {room}: not sent once more: once a day at most, and it was today",
        "resend_ok": "setpoint to {room}: sent once more, confirmed by every thermostat after 25 s",
    },
    "de": {
        "badge": "nicht bestätigt",
        "title": "Das Gerät zeigt die letzte Änderung nicht: Modus (zeigt auto), Solltemperatur (zeigt 20,0 °C).",
        "form": "Vom Gerät nicht bestätigt: Eco, Programm am Montag.",
        "kind": "nicht bestätigt",
        "line": "Solltemperatur für {room}: nach 5 min nicht bestätigt: {name} (Modus Auto, Solltemperatur 20,0)",
        "late": "Solltemperatur: nach 70 s bestätigt",
        "resent": "Solltemperatur für {room}: nochmals an die Thermostate gesendet",
        "resend_not": "Solltemperatur für {room}: nochmals gesendet, nach 5 min weiterhin nicht bestätigt: {name} (Modus Auto, Solltemperatur 20,0)",
        "not_resent": "Solltemperatur für {room}: nicht nochmals gesendet: höchstens einmal am Tag, und heute schon geschehen",
        "resend_ok": "Solltemperatur für {room}: nochmals gesendet, nach 25 s von jedem Thermostat bestätigt",
    },
}


def room_of(page, name):
    return page.locator(".room", has=page.locator(".room-name", has_text=name)).first


def head_icons(room):
    """The symbols of a tile head's problem icons."""
    return room.locator(".tile-head .problems-sum svg use").evaluate_all("(us) => us.map((u) => u.getAttribute('href'))")


def test_unconfirmed_marks(page, case):
    """Pending shows nothing; not confirmed is an amber icon in the tile's
    head, its entry's flag titled with what the device does not show and
    what it shows; the device's row has the same flag; the form of its
    configuration names a configuration's values, a long day once."""
    says = SAYS[case.lang]
    dev = page.evaluate(CONFIRMATION, ["pending", ["mode", "setpoint"]])
    assert dev, "a reported thermostat"
    room = room_of(page, dev["room"])
    assert "#icon-unconfirmed" not in head_icons(room), "nothing while pending"

    page.evaluate(CONFIRMATION, ["unconfirmed", ["mode", "setpoint"]])
    assert wait_until(page, "document.querySelector('.room .tile-head use[href=\"#icon-unconfirmed\"]') !== null", 5), "the tile's icon"
    icon = room.locator(".tile-head .problems-sum svg", has=page.locator("use[href='#icon-unconfirmed']")).first
    assert "tone-warn" in (icon.get_attribute("class") or ""), "amber"
    room.locator(".tile-head .problems-sum").click()
    entry = room.locator(".tile-head .problem-entry", has=page.locator(".problem-name", has_text=dev["name"])).first
    flag = entry.locator(".problem-flags .flag", has_text=says["badge"]).first
    assert flag.get_attribute("title") == says["title"], flag.get_attribute("title")
    assert scroll_width(page) <= case.width, "no horizontal scroll with the list open"
    page.keyboard.press("Escape")

    room.locator(".room-name-btn").click()
    page.wait_for_selector(".room .device", timeout=5000)
    row = room.locator(".device:not(.virtual)", has=page.locator(".device-name", has_text=dev["name"])).first
    row.locator(".problems-sum").click()
    assert row.locator(".problem-flags .flag", has_text=says["badge"]).first.get_attribute("title") == says["title"], "the row's flag"
    page.keyboard.press("Escape")
    room.locator(".room-name-btn").click()

    page.evaluate(CONFIRMATION, ["unconfirmed", ["eco_temp", "program.monday", "program.monday.2"]])
    room.locator(".tile-cog").click()
    page.wait_for_selector(".room .config-body", timeout=5000)
    room.locator(".config-tabs [data-tab=devices]").click()
    form = room.locator(".cfg-device", has=page.locator(".device-name", has_text=dev["name"])).first
    assert form.locator(".device-unconfirmed").inner_text().strip() == says["form"], form.locator(".device-unconfirmed").inner_text()
    assert scroll_width(page) <= case.width, "no horizontal scroll in the devices tab"
    room.locator(".config-tabs .close").click()

    page.evaluate(CONFIRMATION, [None, []])
    assert wait_until(page, "document.querySelector('.room .tile-head use[href=\"#icon-unconfirmed\"]') === null", 5), "gone once shown"


def test_unconfirmed_log_lines(page, case, reads):
    """The stream's unconfirmed line names the write, the room, and each
    device by name with what it shows; a late one the seconds it took."""
    says = SAYS[case.lang]
    dev = page.evaluate(CONFIRMATION, [None, []])
    assert dev, "a reported thermostat"
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    page.evaluate("""(dev) => {
      const now = () => new Date().toISOString();
      const send = (kind, ev) => state.events.dispatchEvent(new MessageEvent(kind, { data: JSON.stringify(Object.assign({ kind, time: now() }, ev)) }));
      send("unconfirmed", { source: "device", level: "warn", summary: { outcome: "not_confirmed", write: "setpoint", target: "room:" + dev.id,
        room: dev.id, source: "api", expected: { mode: "manual", setpoint: 22 }, minutes: 5, confirmed: 1,
        devices: [{ rf_address: dev.rf, fields: ["mode", "setpoint"], shows: { mode: "auto", setpoint: 20 }, error: false, link_error: false, out_of_date: false }] } });
      send("unconfirmed", { source: "device", level: "info", device: dev.rf, summary: { outcome: "confirmed_late", write: "setpoint",
        target: "room:" + dev.id, room: dev.id, source: "api", expected: { mode: "manual", setpoint: 22 }, seconds: 70 } });
    }""", dev)
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-unconfirmed').length === 2", 5), "the two lines"
    lines = page.locator("#log-lines .log-line.kind-unconfirmed")
    assert lines.nth(0).locator(".log-kind").inner_text().strip() == says["kind"]
    assert lines.nth(1).locator(".log-text").inner_text() == says["line"].format(**dev), lines.nth(1).locator(".log-text").inner_text()
    assert lines.nth(0).locator(".log-text").inner_text() == says["late"], lines.nth(0).locator(".log-text").inner_text()
    show_view(page, "tiles")


def test_resend_log_lines(page, case, reads):
    """A room setpoint's re-send (AP RadioHealth v1.3, Block 8): sent once
    more, still not confirmed with each device and what it shows, not
    sent and why, and confirmed by every thermostat."""
    says = SAYS[case.lang]
    dev = page.evaluate(CONFIRMATION, [None, []])
    assert dev, "a reported thermostat"
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    page.evaluate("""(dev) => {
      const now = () => new Date().toISOString();
      const send = (kind, ev) => state.events.dispatchEvent(new MessageEvent(kind, { data: JSON.stringify(Object.assign({ kind, time: now() }, ev)) }));
      const write = { write: "setpoint", target: "room:" + dev.id, room: dev.id, source: "api", expected: { mode: "manual", setpoint: 22 } };
      const line = (level, summary) => send("unconfirmed", { source: "device", level, summary: Object.assign({}, write, summary) });
      line("info", { outcome: "resent", duty_pct: 3, free_slots: 49 });
      line("warn", { outcome: "resend_not_confirmed", minutes: 5, confirmed: 2,
        devices: [{ rf_address: dev.rf, fields: ["mode", "setpoint"], shows: { mode: "auto", setpoint: 20 }, error: true, link_error: false, out_of_date: false }] });
      line("info", { outcome: "not_resent", why: "today" });
      line("info", { outcome: "resend_confirmed", confirmed: 3, seconds: 25 });
    }""", dev)
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-unconfirmed').length === 4", 5), "the four lines"
    lines = page.locator("#log-lines .log-line.kind-unconfirmed")
    for i, key in enumerate(["resend_ok", "not_resent", "resend_not", "resent"]):
        text = lines.nth(i).locator(".log-text").inner_text()
        assert text == says[key].format(**dev), (key, text)
    assert scroll_width(page) <= case.width, "no horizontal scroll"
    show_view(page, "tiles")
