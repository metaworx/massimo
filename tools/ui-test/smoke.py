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
list, also in expert mode, which shows when the server started. The Cube choice's search entry runs a
discovery, which the fake Cube answers as a Cube that is not configured,
shows "searching" until the answer is in, then lists the found Cube as
not selectable with the env lines to add, and the log has its
cube_found; a cube_found event adds another one live, and a refused
search (stubbed 503) shows the server's message. Any console error, page error or horizontal scroll fails the
test.
"""
import sys
import time

from playwright.sync_api import sync_playwright

url = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8119/"
token = sys.argv[2] if len(sys.argv) > 2 else ""
failures = []
OPEN = {"en": "Open", "de": "Offen"}
USED = "0" * 48
FOUND2 = {"serial": "LEQ0654321", "configured": False, "address": "192.168.1.78", "rf_address": "0a1b2e",
          "firmware": "1.1.3", "found_at": "2026-11-05T18:40:00+01:00", "last_seen": "2026-11-05T18:40:00+01:00"}
SEARCHING = {"en": "Searching…", "de": "Suche läuft…"}
FOUND_N = {"en": "{n} Cube{s} found, not configured", "de": "{n} Cube{s} gefunden, nicht eingerichtet"}
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
    started = page.locator("#server-started")
    check(started.is_visible() and started.inner_text().strip() not in ("", "–"), f"{label}: server start time in expert mode")
    check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll in expert mode")
    page.evaluate("() => { state.settings.expert = false; applySettings(); }")
    tile.locator(".room-name-btn").click()


def found_text(lang, n):
    return FOUND_N[lang].format(n=n, s="" if n == 1 else "s")


def discover(page, label, lang):
    width = page.viewport_size["width"]
    sel = page.locator("#cube-select")
    check(sel.locator("option[value=__discover]").count() == 1, f"{label}: search entry in the Cube choice")
    serial = page.evaluate("state.serial")
    held = []
    page.route("**/api/v1/cubes/discover", lambda route: held.append(route))
    sel.select_option("__discover")
    end = time.time() + 5
    while not held and time.time() < end:
        page.wait_for_timeout(100)
    check(page.locator("#cube-search").inner_text() == SEARCHING[lang], f"{label}: searching shown")
    check(sel.locator("option[value=__discover]").is_disabled(), f"{label}: search entry disabled while searching")
    page.unroute("**/api/v1/cubes/discover")
    if held:
        held[0].continue_()  # on to the server, which asks the fake Cube
    check(wait_until(page, "!discovery.running", 10), f"{label}: search done")
    if "bad" in (page.locator("#cube-search").get_attribute("class") or ""):
        print(f"skip  {label}: the server has no discovery socket (UDP 23272 in use?): "
              + page.locator("#cube-search").inner_text())
    else:
        check(page.locator("#cube-search").inner_text() == found_text(lang, 1), f"{label}: search result shown")
        opt = sel.locator("option[value='found:KEQ0000000']")
        check(opt.count() == 1 and opt.is_disabled(), f"{label}: found Cube in the choice, not selectable")
        check(sel.input_value() == serial, f"{label}: the selected Cube stays")
        check(page.locator("#cube-found").get_attribute("open") is not None, f"{label}: found Cubes opened")
        check("MASSIMO_CUBE_KEQ0000000_ADDRESS=127.0.0.1" in page.locator("#cube-found .env-lines").inner_text(), f"{label}: env lines for the found Cube")
        check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll with the found Cubes")
        page.locator("#view-log").click()
        check(wait_until(page, "[...document.querySelectorAll('.log-line.kind-cube_found')].some((l) => l.textContent.includes('KEQ0000000'))", 5),
              f"{label}: cube_found in the log")
        page.locator("#view-tiles").click()
    before = page.evaluate("state.found.length")
    page.evaluate("(c) => state.events.dispatchEvent(new MessageEvent('cube_found', { data: JSON.stringify("
                  "{ kind: 'cube_found', cube: c, time: c.found_at }) }))", FOUND2)
    check(sel.locator("option[value='found:LEQ0654321']").count() == 1, f"{label}: cube_found adds a Cube live")
    check(page.locator("#cube-found summary").inner_text() == found_text(lang, before + 1), f"{label}: found Cubes counted")
    page.route("**/api/v1/cubes/discover", lambda route: route.fulfill(
        status=503, content_type="application/json", body='{"error": "no discovery socket"}'))
    sel.select_option("__discover")
    check(wait_until(page, "document.getElementById('cube-search').classList.contains('bad')", 5)
          and "no discovery socket" in page.locator("#cube-search").inner_text(), f"{label}: a refused search shows the server's message")
    page.unroute("**/api/v1/cubes/discover")


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
                src = m.location.get("url", "")
                # the refused stream and the refused search are on purpose
                expected = m.text.startswith("Failed to load resource") and (
                    (refused and USED in src) or (src.endswith("/cubes/discover") and "503" in m.text))
                if m.type == "error" and not expected:
                    check(False, f"{label}: console: {m.text}")

            page.on("request", lambda r, streams=streams: streams.append(r.url) if "/events?" in r.url else None)
            page.on("console", on_console)
            page.on("pageerror", lambda e, label=label: check(False, f"{label}: page error: {e}"))
            try:
                run(page, label, lang)
                stream(page, label, streams, refused)
                ages(page, label, lang)
                discover(page, label, lang)
            except Exception as e:  # a timeout or a missing element
                check(False, f"{label}: {e}")
            ctx.close()
    browser.close()

if failures:
    print(f"ui-test: {len(failures)} failure(s)")
    sys.exit(1)
print("ui-test: passed")
