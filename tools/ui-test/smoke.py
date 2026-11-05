# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Smoke test of the Massimo web UI against a running server.

usage: smoke.py <url> [<token>]   (tools/ui-test/run.sh starts the server)

At 1100 and 390 px wide, in English and German: the page loads with
room tiles, a tile's device list and its configuration open, the log
and the settings page open, and the language follows the browser. The
event stream opens with tickets, never with the token, and after a
refused stream (a 401) the page opens it again with a new ticket. The
room with the open window contact shows "Open" on its tile, and the
badge goes when a state event closes the contact; the measured
temperature shows its age, and a device made stale (30 hours, through a
state event) greys its values and warns on the tile and in the device
list, also in expert mode. Any console error, page error or horizontal
scroll fails the test.
"""
import sys
import time

from playwright.sync_api import sync_playwright

url = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8119/"
token = sys.argv[2] if len(sys.argv) > 2 else ""
failures = []
OPEN = {"en": "Open", "de": "Offen"}
USED = "0" * 48
STALE = {"en": "Not seen for 30 hours", "de": "Seit 30 Stunden keine Meldung"}


def check(cond, what):
    print(("ok    " if cond else "FAIL  ") + what)
    if not cond:
        failures.append(what)


def wait_until(page, expr, seconds):
    """Polls a page expression from here: the page's CSP rules out
    wait_for_function with a string."""
    end = time.time() + seconds
    while time.time() < end:
        if page.evaluate(expr):
            return True
        page.wait_for_timeout(200)
    return False


def stream(page, label, streams, refused):
    """The first stream request had a used ticket and got a 401; the page
    must have opened a second one with a new ticket."""
    check(len(refused) == 1 and streams and streams[0].endswith("ticket=" + USED), f"{label}: the first stream request had the used ticket")
    check(wait_until(page, "state.eventsOpen", 10), f"{label}: event stream open again after a 401")
    check(len(streams) >= 2, f"{label}: a new stream request after the 401")
    check(all("ticket=" in u and "token=" not in u for u in streams), f"{label}: stream opened with tickets only")
    tickets = [u.split("ticket=", 1)[1] for u in streams if "ticket=" in u]
    check(len(set(tickets)) == len(tickets), f"{label}: a new ticket for every stream request")


def ages(page, label, lang):
    width = page.viewport_size["width"]
    room = page.locator(".room", has=page.locator(".room-name", has_text="Kind 1"))
    badge = room.locator(".window-open")
    check(badge.is_visible() and badge.inner_text().startswith(OPEN[lang]), f"{label}: Open on the tile of the room with the open window")
    check(page.locator(".room").first.locator(".window-open").is_hidden(), f"{label}: no Open on a room with closed windows")
    check(page.locator(".room .temp-age .age").count() > 0, f"{label}: age next to the measured temperature")
    page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.some((d) => d.type === "shutter_contact"));
      const d = JSON.parse(JSON.stringify(r.devices.find((d) => d.type === "shutter_contact")));
      d.state.window_open = false;
      updateDevice(d);
    }""")
    check(room.locator(".window-open").is_hidden(), f"{label}: Open goes when the window closes")
    page.evaluate("""() => {
      const iso = new Date(Date.now() - 30 * 3600 * 1000).toISOString();
      for (const x of state.rooms[0].devices) {
        const d = JSON.parse(JSON.stringify(x));
        d.stale = true;
        d.last_seen = iso;
        for (const k of Object.keys(d.received || {})) d.received[k] = iso;
        updateDevice(d);
      }
    }""")
    tile = page.locator(".room").first
    note = tile.locator(".stale-note")
    check(note.is_visible() and STALE[lang] in note.inner_text(), f"{label}: stale warning on the tile")
    check(tile.locator(".temp-val.stale").count() == 1, f"{label}: stale measured temperature greyed")
    check("30" in tile.locator(".temp-age").inner_text(), f"{label}: the measured temperature is 30 hours old")
    tile.locator(".room-name-btn").click()
    check(STALE[lang] in tile.locator(".device .stale-badge").first.inner_text(), f"{label}: stale badge in the device list")
    check(tile.locator(".device-vals.stale .age").count() > 0, f"{label}: ages of the stale values in the device list")
    check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll with the ages")
    page.evaluate("() => { state.settings.expert = true; applySettings(); }")
    check(tile.locator(".device-vals .age-exact").first.is_visible(), f"{label}: exact times in expert mode")
    check(tile.locator(".device-seen").first.is_visible(), f"{label}: last seen in expert mode")
    check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll in expert mode")
    page.evaluate("() => { state.settings.expert = false; applySettings(); }")
    tile.locator(".room-name-btn").click()


def run(page, label, lang):
    page.goto(url)
    page.wait_for_selector(".room", timeout=10000)
    check(page.locator(".room").count() > 0, f"{label}: room tiles shown")
    check(page.evaluate("document.documentElement.lang") == lang, f"{label}: page language {lang}")
    tile = page.locator(".room").first
    tile.locator(".room-name-btn").click()
    page.wait_for_selector(".room .device", timeout=5000)
    check(tile.locator(".device").count() > 0, f"{label}: device list opens")
    tile.locator(".tile-cog").click()
    page.wait_for_selector(".room .config-body", timeout=5000)
    tabs = tile.locator(".config-tabs [role=tab]")
    check(tabs.count() >= 4, f"{label}: configuration opens with its tabs")
    for name in tabs.all_text_contents():
        tabs.filter(has_text=name).first.click()
        check(tile.locator(".config-body").is_visible(), f"{label}: tab {name.strip()}")
    tile.locator(".config-tabs .close").click()
    check(tile.locator(".config-body").count() == 0, f"{label}: configuration closes")
    width = page.viewport_size["width"]
    check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll on the tiles")
    page.locator("#view-log").click()
    check(page.locator("#logview").is_visible(), f"{label}: log opens")
    page.locator("#view-settings").click()
    check(page.locator("#settings").is_visible(), f"{label}: settings open")
    page.wait_for_timeout(500)
    check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll on the settings")
    page.locator("#view-tiles").click()
    check(page.locator(".room").first.is_visible(), f"{label}: back to the tiles")


def used_ticket_first(refused):
    """A route handler for the ticket requests: the first one gets a used
    ticket, so the server refuses that stream with a 401; the others go
    through. (A handler with a second parameter would get the request.)"""

    def handle(route):
        if not refused:
            refused.append(USED)
            route.fulfill(status=200, content_type="application/json", body='{"ticket": "%s"}' % USED)
        else:
            route.continue_()

    return handle


with sync_playwright() as pw:
    browser = pw.chromium.launch()
    for locale, lang in (("en-US", "en"), ("de-DE", "de")):
        for width in (1100, 390):
            label = f"{width} px {lang}"
            ctx = browser.new_context(viewport={"width": width, "height": 900}, locale=locale)
            if token:
                ctx.add_init_script(f"localStorage.setItem('massimo.token', {token!r})")
            page = ctx.new_page()
            page.set_default_timeout(5000)
            streams, refused = [], []

            page.route("**/events/ticket", used_ticket_first(refused))

            def on_console(m, label=label, refused=refused):
                expected = m.text.startswith("Failed to load resource") and refused and USED in m.location.get("url", "")
                if m.type == "error" and not expected:
                    check(False, f"{label}: console: {m.text}")

            page.on("request", lambda r, streams=streams: streams.append(r.url) if "/events?" in r.url else None)
            page.on("console", on_console)
            page.on("pageerror", lambda e, label=label: check(False, f"{label}: page error: {e}"))
            try:
                run(page, label, lang)
                stream(page, label, streams, refused)
                ages(page, label, lang)
            except Exception as e:  # a timeout or a missing element
                check(False, f"{label}: {e}")
            ctx.close()
    browser.close()

if failures:
    print(f"ui-test: {len(failures)} failure(s)")
    sys.exit(1)
print("ui-test: passed")
