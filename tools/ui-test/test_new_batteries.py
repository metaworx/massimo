# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""New batteries in a device's row (AP RefreshBatteries v1.0, Block 4,
D8-A): offered while the device's battery level is weak or low and its
flag is off, for a role with configure; it sends {"replaced": true} to
POST …/devices/{rf}/battery and says the answer in the row, and the
badge goes with the level's battery event or the next device list, not
with the answer. The level and the flag come from the page's reads of
the rooms (reads), since the fake Cube does not change its states; the
POST is held and answered by the test, and the route has its Go tests.
The log says new batteries in the level's line and the change line."""
import json

from helpers import css_hidden, isolate_log, named_buttons, scroll_width, show_view, stub_me, wait_log_live, wait_until

# The page's read of the chosen Cube's rooms (fetchRooms of app.js).
ROOMS = "**/rooms?devices=all"
BATTERY = "**/api/v1/cubes/*/devices/*/battery"
# The capabilities of an operator, who has configure, and of a role
# without it (/me stubbed).
OPERATOR = ["read", "preferences", "setpoint", "lock", "configure"]
NO_CONFIGURE = ["read", "preferences", "setpoint", "lock"]
STILL_LOW = "device %s still reports its batteries low: wait for its next report"
# The row's New batteries off, turning and aria-busy, as setBusy makes a
# button whose request runs; the row's result line saying a text, in a
# class (ok, bad). Waited for, in one look at each try (Device.shows).
BUSY = "(() => { const b = document.querySelector(%s); return !!b && b.disabled && b.classList.contains('busy') && b.getAttribute('aria-busy') === 'true'; })()"
RESULT = """(([sel, text, cls]) => { const r = document.querySelector(sel);
  return !!r && r.textContent === text && r.classList.contains(cls); })(%s)"""


class Device:
    """A reported thermostat of the rooms, its level and flag as the
    test tells them to the page's reads of the rooms, kept through the
    server's own events of it; its row in its tile's device list."""

    def __init__(self, page, reads):
        self.page, self.reads = page, reads
        found = page.evaluate("""() => {
          const r = state.rooms.find((r) => r.devices.some((d) => isThermostat(d) && !d.virtual && d.reported));
          const d = r.devices.find((d) => isThermostat(d) && !d.virtual && d.reported);
          return { room: r.name, rf: d.rf_address, serial: d.serial, name: deviceName(d) };
        }""")
        self.rf, self.serial, self.name = found["rf"], found["serial"], found["name"]
        self.want = {"level": "ok", "flag": False}
        reads.change(ROOMS, self._told)
        self.tile = page.locator(".room", has=page.locator(".room-name", has_text=found["room"])).first
        self.selector = ".room .device[data-rf='%s']" % self.rf
        self.row = page.locator(self.selector)
        self.button = self.row.locator(".device-batteries")

    def _told(self, rooms):
        for r in rooms or []:
            for d in r.get("devices") or []:
                if d.get("rf_address") == self.rf:
                    d["reported"] = True
                    d.setdefault("state", {})["battery_low"] = self.want["flag"]
                    if self.want["level"] is None:
                        d.pop("battery", None)
                    else:
                        d["battery"] = self.want["level"]
        return rooms

    def told(self, level, flag):
        """The device's level and flag in the rooms read from now on, and
        read at once; kept (reads.keep), since a state or config event of
        the device that the server sends by itself, after an earlier
        test's write, brings the server's battery and starts no read."""
        self.want = {"level": level, "flag": flag}
        self.reads.keep(self.rf)
        self.page.evaluate("loadRooms()")
        js_level = "undefined" if level is None else json.dumps(level)
        assert wait_until(self.page, "(() => { const d = state.rooms.flatMap((r) => r.devices).find((d) => d.rf_address === %s);"
                          " return d && d.battery === %s && d.state.battery_low === %s; })()"
                          % (json.dumps(self.rf), js_level, json.dumps(flag)), 5), f"the page has {level}, flag {flag}"

    def open(self):
        """The device's tile with its device list open."""
        if self.row.count() == 0:
            self.tile.locator(".room-name-btn").click()
        self.row.wait_for(state="visible")

    def shows(self, button, what, badge=None, first=False):
        """Waits until the row offers New batteries (visible) or not
        (absent), and, where badge says, shows the battery's badge or not,
        and where first, the button first of its actions; all in one look
        at each try. The told level and flag do not stay drawn between
        two looks: a state or config event of the device that the server
        sends by itself draws the server's battery, and keep's read of the
        rooms brings the told ones back only once its answer has come
        (reads.keep), so a check waits for them rather than looking once."""
        js = """(([sel, button, badge, first]) => {
          const row = document.querySelector(sel);
          if (!row) return false;
          const b = row.querySelector('.device-batteries');
          const i = !!row.querySelector(".device-badges .problems-sum use[href='#icon-battery']");
          if (button ? !(b && b.checkVisibility()) : !!b) return false;
          if (badge !== null && i !== badge) return false;
          return !first || row.querySelector('.device-actions > button') === b;
        })(%s)""" % json.dumps([self.selector, button, badge, first])
        assert wait_until(self.page, js, 5), what


class Held:
    """The marks of new batteries the page sends, held until the test
    answers them: each one's path and body."""

    def __init__(self, page):
        self.page, self.routes, self.paths, self.bodies = page, [], [], []
        page.route(BATTERY, self.handle)

    def handle(self, route):
        self.paths.append(route.request.url.split("/api/v1/")[-1])
        self.bodies.append(route.request.post_data_json)
        self.routes.append(route)

    def wait(self, n, seconds=5):
        """Waits until n marks have been sent."""
        for _ in range(seconds * 10):
            if len(self.routes) >= n:
                return True
            self.page.wait_for_timeout(100)
        return len(self.routes) >= n

    def answer(self, i, status, body):
        self.routes[i].fulfill(status=status, content_type="application/json", body=json.dumps(body))

    def close(self):
        self.page.unroute(BATTERY)


def test_new_batteries_offered(page, case, reads):
    """The row offers New batteries, first of its actions and named with
    the device, while its level is weak or low and its flag off; not
    while the flag is on, at ok, nor for a device of a server without
    levels; and only for a role with configure."""
    dev = Device(page, reads)
    dev.told("weak", False)
    dev.open()
    dev.shows(True, "weak, its flag off: New batteries, first of the row's actions, beside the battery's badge", badge=True, first=True)
    assert dev.button.inner_text() == page.evaluate("t('device.newBatteries')"), "its text"
    assert dev.button.get_attribute("aria-label") == page.evaluate("t('device.newBatteriesLabel', {device: %s})" % json.dumps(dev.name)), \
        "named with the device"
    assert dev.button.get_attribute("title") == page.evaluate("t('device.newBatteriesTitle')"), "its tooltip"
    named_buttons(page, "a device row with New batteries")
    assert scroll_width(page) <= case.width, "no horizontal scroll with the button"
    dev.told("weak", True)
    dev.shows(False, "not while the flag is on: the device has not reported its new batteries")
    dev.told("low", True)
    dev.shows(False, "low, the flag on: none")
    dev.told("low", False)
    dev.shows(True, "low, its flag off: New batteries")
    dev.told("ok", False)
    dev.shows(False, "none at ok")
    dev.told(None, True)
    dev.shows(False, "none by the flag alone, on a server without levels")
    dev.told("weak", False)
    try:
        stub_me(page, NO_CONFIGURE)
        assert wait_until(page, "document.body.classList.contains('no-configure')", 5), "no configure"
        assert css_hidden(page, dev.selector + " .device-batteries"), "not for a role without configure"
        page.unroute("**/api/v1/me")
        stub_me(page, OPERATOR)
        assert wait_until(page, "!document.body.classList.contains('no-configure')", 5), "configure"
        dev.shows(True, "for an operator")
    finally:
        page.unroute("**/api/v1/me")
        page.evaluate("loadMe()")


def test_new_batteries_sent(page, case, reads, watch):
    """New batteries sends {"replaced": true} to the device's route by
    its serial and turns until the answer, which its line in the row
    says; the badge and the button stay until the level's battery event
    takes them; a 409 says the server's error in the row, in red."""
    dev = Device(page, reads)
    watch.expected.append(("/devices/%s/battery" % (dev.serial or dev.rf), 409))
    held = Held(page)
    try:
        dev.told("weak", False)
        dev.open()
        dev.button.click()
        assert held.wait(1), "the mark sent"
        assert held.paths == ["cubes/KEQ0000100/devices/%s/battery" % (dev.serial or dev.rf)], f"the device's route: {held.paths}"
        assert held.bodies == [{"replaced": True}], f"its body: {held.bodies}"
        assert wait_until(page, BUSY % json.dumps(dev.selector + " .device-batteries"), 5), \
            "New batteries is off with its spinner while the request runs"
        held.answer(0, 200, {"battery": "ok", "from": "weak", "changed": True})
        done = page.evaluate("t('device.newBatteriesDone')")
        assert wait_until(page, RESULT % json.dumps([dev.selector + " .battery-result", done, "ok"]), 5), "the row says it, in green"
        dev.shows(True, "the badge and the button stay until the level's event", badge=True)
        # The level's battery event on the stream, as the server sends it.
        dev.want = {"level": "ok", "flag": False}
        page.evaluate("""(rf) => state.events.dispatchEvent(new MessageEvent("battery", { data: JSON.stringify({
          kind: "battery", device: rf, source: "device", level: "info", time: new Date().toISOString(),
          summary: { level: "ok", from: "weak", cause: "replaced" } }) }))""", dev.rf)
        dev.shows(False, "the button and the badge go with it", badge=False)

        dev.told("weak", False)
        dev.shows(True, "offered again")
        dev.button.click()
        assert held.wait(2), "sent again"
        held.answer(1, 409, {"error": STILL_LOW % dev.rf})
        failed = page.evaluate("t('device.newBatteriesFailed', {error: %s})" % json.dumps(STILL_LOW % dev.rf))
        assert wait_until(page, RESULT % json.dumps([dev.selector + " .battery-result", failed, "bad"]), 5), "the 409's error in the row, in red"
        assert wait_until(page, "(() => { const b = document.querySelector(%s); return !!b && !b.disabled; })()"
                          % json.dumps(dev.selector + " .device-batteries"), 5), "the button back"
        dev.shows(True, "the badge as it was", badge=True)
        assert scroll_width(page) <= case.width, "no horizontal scroll with the line"
    finally:
        held.close()


def test_new_batteries_in_the_log(page, case, reads):
    """The log says new batteries: the level's line ends with them and
    who marked them; the request's change line names its action and the
    device, without the body's replaced."""
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    name = page.evaluate("""() => {
      const d = state.rooms.flatMap((r) => r.devices).find((d) => isThermostat(d) && !d.virtual && d.serial && d.name);
      const by = { name: "anna", display_name: "Anna" };
      const send = (kind, ev) => state.events.dispatchEvent(new MessageEvent(kind, { data: JSON.stringify(Object.assign({ kind,
        time: new Date().toISOString(), level: "info", by }, ev)) }));
      send("change", { source: "config", serial: d.serial, summary: { action: "device.battery", target: "device:" + d.serial,
        set: { replaced: true } } });
      send("battery", { source: "device", device: d.rf_address, serial: d.serial, summary: { level: "ok", from: "low", cause: "replaced" } });
      return d.name;
    }""")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-battery, #log-lines .log-line.kind-change').length >= 2", 5), \
        "the two lines"
    want = page.evaluate("""(name) => [
      t("log.by", { text: t("log.sum.batteryReplaced", { level: t("log.battery.ok"), from: t("log.batteryBefore.low") }), name: "Anna" }),
      t("log.by", { text: t("log.sum.change", { action: t("log.action.device.battery"), target: name }), name: "Anna" })]""", name)
    got = [page.locator("#log-lines .log-line.kind-battery .log-text").first.inner_text(),
           page.locator("#log-lines .log-line.kind-change .log-text").first.inner_text()]
    assert got == want, f"the lines {got}, want {want}"
    if case.lang == "en":
        assert want[0] == "Batteries ok (new batteries, before: low), by Anna", want[0]
    show_view(page, "tiles")
