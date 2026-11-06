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
search (stubbed 503) shows the server's message. A state event with
changes shows in the log as what changed. In Settings → Cubes a name
is saved and taken back from the settings file, hidden devices take
their room's tile away and back, and a found Cube is added and removed,
Remove asking in the panel.
In Settings → Appearance a
scheme changes the computed --bg, the dark mode sets data-theme, the
editor applies a colour live and keeps a saved scheme in the browser,
its hex fields take ff0000 without the #, widen #ABC to #aabbcc and
refuse nope with the colour kept, its badge column comes first so the
colour inputs line up,
Export downloads the scheme document (a blob: link under the page's
Content-Security-Policy), Import reads it back with a missing token
filled from the default and an unknown one dropped, and a file that
is not a scheme shows a message. On every page it opens, every button
has an accessible name (text, aria-label or title). Any console error, page error or horizontal scroll fails the
test.
"""
import json
import sys
import tempfile
import time
from pathlib import Path

from playwright.sync_api import sync_playwright

url = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8119/"
token = sys.argv[2] if len(sys.argv) > 2 else ""
failures = []
OPEN = {"en": "Open", "de": "Offen"}
USED = "0" * 48
FOUND2 = {"serial": "LEQ0654321", "configured": False, "address": "192.168.1.78", "rf_address": "0a1b2e",
          "firmware": "1.1.3", "found_at": "2026-11-05T18:40:00+01:00", "last_seen": "2026-11-05T18:40:00+01:00"}
CHANGES = {"en": "window opened, set 18.0 → 5.0", "de": "Fenster geöffnet, Soll 18,0 → 5,0"}
SEARCHING = {"en": "Searching…", "de": "Suche läuft…"}
FOUND_N = {"en": "{n} Cube{s} found, not configured", "de": "{n} Cube{s} gefunden, nicht eingerichtet"}
STALE = {"en": "Not seen for 30 hours", "de": "Seit 30 Stunden keine Meldung"}
CONTRAST_SCHEME = {"en": "High contrast", "de": "Hoher Kontrast"}
SAVED = {"en": "Saved as “Test scheme”.", "de": "Als „Test scheme“ gespeichert."}
HEX_LABEL = {"en": "Lines, hex", "de": "Linien, Hex"}
HEX_BAD = {"en": "Lines: “nope” is not a colour. Write it as #rrggbb or #rgb, the # may be left out; Escape puts the colour back.",
           "de": "Linien: „nope“ ist keine Farbe. Als #rrggbb oder #rgb schreiben, das # darf fehlen; Escape stellt die Farbe wieder her."}
NOT_A_SCHEME = {"en": "Not a Massimo colour scheme: unknown format", "de": "Kein Massimo-Farbschema: unbekanntes Format"}


def check(cond, what):
    print(("ok    " if cond else "FAIL  ") + what)
    if not cond:
        failures.append(what)


def named_buttons(page, label, where):
    """Every button of the page has an accessible name: its text, an
    aria-label, an aria-labelledby or a title. The scripts build most of
    them, so this runs on every page the test opens."""
    nameless = page.evaluate("""() => [...document.querySelectorAll('button')].filter((b) => {
      const by = (b.getAttribute('aria-labelledby') || '').split(/\\s+/).map((id) => document.getElementById(id))
        .filter((e) => e).map((e) => e.textContent).join('');
      return ![b.textContent, b.getAttribute('aria-label'), b.title, by].some((n) => n && n.trim());
    }).map((b) => b.outerHTML.slice(0, 160))""")
    check(not nameless, f"{label}: every button has a name on {where}" + (f": {nameless}" if nameless else ""))


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
    check(tile.locator(".device .device-sub").first.is_hidden(), f"{label}: no type and radio address outside expert mode")
    page.evaluate("() => { state.settings.expert = true; applySettings(); }")
    check(tile.locator(".device .device-rf").first.is_visible(), f"{label}: radio address in expert mode")
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


def cubes(page, label, lang):
    """Settings → Cubes: a new name is saved and shown in the header; the
    settings file's differing name is offered and taken back; hiding the
    devices of a room makes its tile go, showing them again brings it
    back; a Cube found on the LAN is added and removed again, and is then
    listed as found. Everything is put back as it was, since the next run
    uses the same server."""
    width = page.viewport_size["width"]
    page.locator("#view-settings").click()
    panel = page.locator(".cube-panel[data-serial='KEQ0537249']")
    check(panel.count() == 1, f"{label}: the Cube in Settings → Cubes")
    panel.locator("summary").click()
    form = panel.locator("form")
    check(form.is_visible(), f"{label}: the Cube's panel opens")
    named_buttons(page, label, "a Cube's panel")
    form.locator("input[name=name]").fill("Renamed")
    form.locator("button[type=submit]").click()
    check(wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0537249' && c.name === 'Renamed')", 5), f"{label}: the new name is saved")
    check("Renamed" in page.locator("#cube-select option[value='KEQ0537249']").inner_text(), f"{label}: the header shows the new name")
    take = panel.locator(".take-file[data-field=name]")
    check(take.count() == 1 and "MASSIMO_CUBE_KEQ0537249_NAME" in panel.locator(".cube-differs").inner_text(),
          f"{label}: the settings file's name is offered")
    take.click()
    check(wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0537249' && c.name === 'Fake Cube')", 5), f"{label}: the file's value taken")
    check(panel.locator(".cube-differs").count() == 0, f"{label}: no difference left")
    check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll with the Cube's panel")

    room = page.evaluate("(() => { const r = state.rooms[state.rooms.length - 1];"
                         " return { name: r.name, rfs: r.devices.map((d) => d.rf_address) }; })()")
    for rf in room["rfs"]:
        form.locator("input[name=hide]").fill(rf)
        form.locator(".cube-hide-add button").click()
    check(form.locator(".cube-hidden-list li").count() == len(room["rfs"]), f"{label}: devices listed to hide")
    form.locator("button[type=submit]").click()
    gone = "!state.rooms.some((r) => r.name === %s)" % json.dumps(room["name"])
    check(wait_until(page, gone, 5), f"{label}: hiding the devices removes their room")
    page.locator("#view-tiles").click()
    check(page.evaluate("![...document.querySelectorAll('.room .room-name')].some((e) => e.textContent.trim() === %s)" % json.dumps(room["name"])),
          f"{label}: the room's tile is gone")
    page.locator("#view-settings").click()
    for _ in room["rfs"]:
        form.locator(".cube-hidden-list button").first.click()
    form.locator("button[type=submit]").click()
    check(wait_until(page, "!(" + gone + ")", 5), f"{label}: showing them again brings the room back")
    check(wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0537249' && c.hidden.length === 0)", 5), f"{label}: nothing hidden any more")

    found = page.locator(".cube-found-row[data-serial='KEQ0000000']")
    if found.count() == 0:
        print(f"skip  {label}: no found Cube to add (discovery did not answer)")
    else:
        found.locator("button").click()
        check(wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0000000')", 5), f"{label}: Add serves the found Cube")
        check(page.evaluate("state.serial") == "KEQ0537249", f"{label}: the selected Cube stays")
        other = page.locator(".cube-panel[data-serial='KEQ0000000']")
        other.locator("summary").click()
        other.locator(".remove-cube").click()
        confirm = other.locator(".confirm-box")
        check(confirm.is_visible() and other.locator(".remove-cube").count() == 0, f"{label}: Remove asks in the panel")
        confirm.locator(".confirm-no").click()
        check(other.locator(".confirm-box").count() == 0 and other.locator(".remove-cube").is_visible(), f"{label}: Keep takes the question back")
        other.locator(".remove-cube").click()
        other.locator(".confirm-box .confirm-yes").click()
        check(wait_until(page, "!state.cubes.some((c) => c.serial === 'KEQ0000000') && state.found.some((c) => c.serial === 'KEQ0000000')", 5),
              f"{label}: Remove lists it as found again")
    check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll in Settings → Cubes")
    page.locator("#view-tiles").click()


def log_changes(page, label, lang):
    """A state event with changes (as the server sends after the first
    one of a device) shows what changed, not the whole state. The log is
    opened first: opening it loads the server's log."""
    page.locator("#view-log").click()
    page.wait_for_timeout(500)
    page.evaluate("""() => {
      const d = JSON.parse(JSON.stringify(state.rooms[0].devices[0]));
      state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({
        kind: "state", device: d.rf_address, state: d, time: new Date().toISOString(),
        changes: { window_open: { from: false, to: true }, setpoint: { from: 18, to: 5 } } }) }));
    }""")
    line = page.locator(".log-line.kind-state").last
    check(CHANGES[lang] in line.inner_text(), f"{label}: the log says what changed")
    page.locator("#view-tiles").click()


def scheme_hex(page, label, lang):
    """The editor's hex fields: a value typed without the # applies, #ABC
    is widened to #aabbcc, a value that is not a colour is refused and
    the colour stays, Escape puts the field back, and the picker writes
    the field. The badge, or its empty place, comes before the colour,
    so the colour inputs and the hex fields of a set line up."""
    line = lambda: page.evaluate("getComputedStyle(document.documentElement).getPropertyValue('--line').trim()")
    hex_field = page.locator("#scheme-light-line-hex")
    picker = page.locator("#scheme-light-line")
    msg = page.locator("#scheme-hex-msg")
    check(hex_field.get_attribute("aria-label") == HEX_LABEL[lang], f"{label}: hex field labelled in {lang}")
    hex_field.fill("ff0000")
    check(line() == "#ff0000" and picker.input_value() == "#ff0000", f"{label}: ff0000 typed without # applies live")
    hex_field.press("Tab")
    check(hex_field.input_value() == "#ff0000", f"{label}: the hex field is normalised on change")
    hex_field.fill("#ABC")
    hex_field.press("Tab")
    check(hex_field.input_value() == "#aabbcc" and line() == "#aabbcc", f"{label}: #ABC becomes #aabbcc")
    hex_field.fill("nope")
    hex_field.press("Tab")
    check(hex_field.get_attribute("aria-invalid") == "true" and line() == "#aabbcc" and picker.input_value() == "#aabbcc",
          f"{label}: nope is refused and the colour stays")
    check(msg.is_visible() and msg.inner_text() == HEX_BAD[lang], f"{label}: refused hex explained in {lang}")
    hex_field.focus()
    hex_field.press("Escape")
    check(hex_field.input_value() == "#aabbcc" and hex_field.get_attribute("aria-invalid") is None and msg.is_hidden(),
          f"{label}: Escape puts the hex field back")
    picker.fill("#123456")
    check(hex_field.input_value() == "#123456" and line() == "#123456", f"{label}: the picker writes the hex field")
    for i, mode in enumerate(("light", "dark")):
        cols = page.evaluate("""(i) => [...document.querySelectorAll('#scheme-editor .scheme-set')[i].querySelectorAll('.scheme-row')]
            .map((r) => {
              const x = (sel) => Math.round(r.querySelector(sel).getBoundingClientRect().left * 10) / 10;
              return {badge: x('.contrast'), colour: x('input[type=color]'), hex: x('.scheme-hex'),
                      real: !r.querySelector('.contrast').classList.contains('none')};
            })""", i)
        check(len(cols) == 19 and any(c["real"] for c in cols) and any(not c["real"] for c in cols)
              and len({c["colour"] for c in cols}) == 1 and len({c["hex"] for c in cols}) == 1
              and len({c["badge"] for c in cols}) == 1 and all(c["badge"] < c["colour"] < c["hex"] for c in cols),
              f"{label}: badge column first, colours and hex fields aligned in the {mode} set")


def appearance(page, label, lang, tmp):
    """Settings → Appearance: a scheme changes the page's colours, the
    mode sets data-theme, the editor applies live and keeps a saved
    scheme, Export downloads the scheme document and Import reads it
    back, both under the page's Content-Security-Policy."""
    width = page.viewport_size["width"]
    page.locator("#view-settings").click()
    bg = lambda: page.evaluate("getComputedStyle(document.documentElement).getPropertyValue('--bg').trim()")
    stored = lambda: page.evaluate("JSON.parse(localStorage.getItem('massimo.schemes'))")
    default_bg = bg()
    sel = page.locator("#scheme-select")
    check(sel.locator("option").count() >= 3, f"{label}: built-in schemes in the choice")
    sel.select_option("contrast")
    check(bg() == "#ffffff" and bg() != default_bg, f"{label}: a scheme changes the computed --bg")
    check(sel.locator("option:checked").inner_text() == CONTRAST_SCHEME[lang], f"{label}: scheme named in {lang}")
    check(page.evaluate("localStorage.getItem('massimo.scheme')") == "contrast", f"{label}: scheme stored")
    mode = page.locator("#theme-select")
    mode.select_option("dark")
    check(page.evaluate("document.documentElement.dataset.theme") == "dark", f"{label}: mode dark sets data-theme")
    check(bg() == "#000000", f"{label}: the dark set in dark mode")
    check(page.evaluate("localStorage.getItem('massimo.theme')") == "dark", f"{label}: mode stored")
    mode.select_option("system")
    check(not page.evaluate("document.documentElement.hasAttribute('data-theme')"), f"{label}: mode system removes data-theme")
    page.locator("#scheme-customise").click()
    editor = page.locator("#scheme-editor")
    check(editor.is_visible() and editor.locator("input[type=color]").count() == 38, f"{label}: editor with a colour input per token and set")
    check(editor.locator(".contrast.ok").count() == 14, f"{label}: contrast badges, all ok for a built-in scheme")
    check(editor.locator("#scheme-delete").is_disabled(), f"{label}: no Delete for a built-in scheme")
    named_buttons(page, label, "the scheme editor")
    scheme_hex(page, label, lang)
    page.locator("#scheme-light-bg").fill("#112233")
    check(bg() == "#112233", f"{label}: the editor applies live")
    check(editor.locator(".contrast.bad").count() == 1, f"{label}: a bad contrast is graded")
    check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll with the editor")
    page.locator("#scheme-name").fill("Test scheme")
    editor.locator("button.primary").click()
    check(sel.input_value() == "custom:Test scheme", f"{label}: Save as chooses the saved scheme")
    check(page.locator("#scheme-msg").inner_text() == SAVED[lang], f"{label}: saved message in {lang}")
    kept = stored()
    check(len(kept) == 1 and kept[0]["name"] == "Test scheme" and kept[0]["light"]["bg"] == "#112233"
          and len(kept[0]["light"]) == 19 and len(kept[0]["dark"]) == 19, f"{label}: the scheme is in the browser")
    check(not editor.locator("#scheme-delete").is_disabled(), f"{label}: Delete for the user's scheme")
    with page.expect_download() as dl:
        page.locator("#scheme-export").click()
    doc = json.loads(Path(dl.value.path()).read_text())
    check(dl.value.suggested_filename == "massimo-scheme-test-scheme.json", f"{label}: export file name")
    check(doc.get("format") == "massimo-scheme/1" and doc.get("name") == "Test scheme" and doc["light"]["bg"] == "#112233"
          and len(doc["light"]) == 19 and len(doc["dark"]) == 19 and doc["dark"]["bg"] == "#000000", f"{label}: export is the scheme document")
    editor.locator("#scheme-delete").click()
    check(sel.input_value() == "massimo" and bg() == default_bg and editor.is_hidden(), f"{label}: Delete goes back to the default")
    check(page.evaluate("localStorage.getItem('massimo.schemes')") is None, f"{label}: nothing stored after Delete")
    doc["name"] = "Round trip"
    del doc["dark"]["bg"]
    doc["light"]["nope"] = "#123456"
    good = tmp / f"scheme-{width}-{lang}.json"
    good.write_text(json.dumps(doc))
    page.locator("#scheme-file").set_input_files(str(good))
    check(wait_until(page, "state.settings.scheme === 'custom:Round trip'", 5), f"{label}: Import chooses the imported scheme")
    check(bg() == "#112233", f"{label}: Import round-trips the colours")
    check(stored()[0]["dark"]["bg"] == "#151514", f"{label}: a missing token takes the default")
    check("nope" not in stored()[0]["light"], f"{label}: an unknown token is ignored")
    bad = tmp / f"bad-{width}-{lang}.json"
    bad.write_text('{"hello": 1}')
    page.locator("#scheme-file").set_input_files(str(bad))
    msg = page.locator("#scheme-msg")
    check(wait_until(page, "document.getElementById('scheme-msg').classList.contains('bad')", 5)
          and msg.inner_text() == NOT_A_SCHEME[lang], f"{label}: a bad file shows a message in {lang}")
    check(page.evaluate("state.settings.scheme") == "custom:Round trip", f"{label}: a bad file changes nothing")
    page.locator("#scheme-customise").click()
    check(editor.is_visible() and page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll with the editor open on the imported scheme")
    editor.locator(".actions-right button").click()
    check(editor.is_hidden(), f"{label}: Close hides the editor")
    sel.select_option("massimo")
    check(bg() == default_bg, f"{label}: back to the default scheme")
    page.locator("#view-tiles").click()


def run(page, label, lang):
    page.goto(url)
    page.wait_for_selector(".room", timeout=10000)
    check(page.locator(".room").count() > 0, f"{label}: room tiles shown")
    check(page.evaluate("document.documentElement.lang") == lang, f"{label}: page language {lang}")
    named_buttons(page, label, "the rooms")
    tile = page.locator(".room").first
    tile.locator(".room-name-btn").click()
    page.wait_for_selector(".room .device", timeout=5000)
    check(tile.locator(".device").count() > 0, f"{label}: device list opens")
    named_buttons(page, label, "a device list")
    tile.locator(".tile-cog").click()
    page.wait_for_selector(".room .config-body", timeout=5000)
    tabs = tile.locator(".config-tabs [role=tab]")
    check(tabs.count() >= 4, f"{label}: configuration opens with its tabs")
    for name in tabs.all_text_contents():
        tabs.filter(has_text=name).first.click()
        check(tile.locator(".config-body").is_visible(), f"{label}: tab {name.strip()}")
        named_buttons(page, label, f"the configuration tab {name.strip()}")
    tile.locator(".config-tabs .close").click()
    check(tile.locator(".config-body").count() == 0, f"{label}: configuration closes")
    width = page.viewport_size["width"]
    check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll on the tiles")
    page.locator("#view-log").click()
    check(page.locator("#logview").is_visible(), f"{label}: log opens")
    named_buttons(page, label, "the log")
    page.locator("#view-settings").click()
    check(page.locator("#settings").is_visible(), f"{label}: settings open")
    page.wait_for_timeout(500)
    named_buttons(page, label, "the settings")
    check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll on the settings")
    update_busy(page, label)
    page.locator("#view-tiles").click()
    check(page.locator(".room").first.is_visible(), f"{label}: back to the tiles")


def update_json(state, to, available):
    return ('{"current": "v0.0.0-test", "channel": "alpha", "latest": "%s", "available": %s, "can_update": %s, '
            '"helper": {"state": "%s", "to": "%s", "time": "2026-01-01T00:00:00Z"}}'
            % (to, "true" if available else "false", "true" if available else "false", state, to))


def update_busy(page, label):
    """While the update helper works, both buttons of the Software section
    are greyed out, also when the page only learns it from GET
    /api/v1/update (a reload during an update, or an update started from
    another browser), and come back once the helper reports done."""
    page.route("**/api/v1/update*", lambda route: route.fulfill(
        status=200, content_type="application/json", body=update_json("installing", "v0.0.1-test", True)))
    page.locator("#update-check").click()
    check(wait_until(page, "document.getElementById('update-check').disabled && document.getElementById('update-btn').disabled", 5),
          f"{label}: Software buttons greyed out while the helper installs")
    page.unroute("**/api/v1/update*")
    page.route("**/api/v1/update*", lambda route: route.fulfill(
        status=200, content_type="application/json", body=update_json("done", "v0.0.1-test", False)))
    check(wait_until(page, "!document.getElementById('update-check').disabled && !document.getElementById('update-btn').disabled", 10),
          f"{label}: Software buttons back once the helper is done")
    page.unroute("**/api/v1/update*")


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


with sync_playwright() as pw, tempfile.TemporaryDirectory() as tmpdir:
    tmp = Path(tmpdir)
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
                cubes(page, label, lang)
                log_changes(page, label, lang)
                appearance(page, label, lang, tmp)
            except Exception as e:  # a timeout or a missing element
                check(False, f"{label}: {e}")
            ctx.close()
    browser.close()

if failures:
    print(f"ui-test: {len(failures)} failure(s)")
    sys.exit(1)
print("ui-test: passed")
