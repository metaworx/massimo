# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The web UI's screenshot set, taken against a running server.

usage: screenshots.py <url> [<token>]   (tools/ui-test/run.sh screenshots)
       screenshots.py --list            (the slideshow's frames, in order)

Takes the fixed set SETS names into docs/screenshots/, each PNG named
<page>-<width>-<theme>-<lang>.png, from one prepared state, so that the
set is retaken at every change of the UI and comes out the same,
byte for byte on one machine, on every run:

- the server and the fake Cube are run.sh's, with greeting.txt: the
  device states, the window contact of "Kind 1" open;
- every time the server sends (received, last seen, the log, when it
  started) is replaced by SERVER_TIME on the way to the page, the page's
  clock is fixed at NOW (Date only; the timers run), and the browser's
  time zone is TZ, so that ages and times are the same on every run;
- the first room's devices are made 30 hours old and stale in the page,
  as the smoke test does it;
- the event stream is a stub that opens and sends nothing, so that no
  live update lands in the middle of a run (the log's one state event
  with changes is dispatched on it, with a fixed time); the poll still
  runs and gets the same answers;
- the version (the log's line of the server's start too), the Software
  section's answer, the fake Cube's address (which has run.sh's port)
  and a Cube found on the LAN are fixed answers of the routes below;
  the found Cube is listed from the Cube choice's shot on;
- reduced motion, one device pixel per CSS pixel, and the screenshots
  taken with animations stopped and the caret hidden;
- every tile rastered whole (CHROMIUM_ARGS): by default Chromium
  rasters again only the part of a tile that changed, and an
  anti-aliased edge rastered in part comes out a level of 255 off the
  same edge rastered whole, so the header's rounded buttons, painted
  before the API's answers or after, changed from run to run.

A shot is framed by what it is about (PAGES): the viewport where the
page is the subject, else the one element that is, with PADDING around
it and the whole of it, however tall; at phone width (390) every shot
is the viewport, as a phone shows the page.

Nothing is sent to the fake Cube and nothing is changed in the server's
store. A shot that cannot be taken fails the run (exit 1); the pixels
are not compared with anything.
"""
import json
import math
import re
import sys
import time
from pathlib import Path
from urllib.parse import urlparse

from helpers import open_user_menu, show_view

OUT = Path(__file__).resolve().parents[2] / "docs" / "screenshots"
NOW = "2026-11-03T20:51:00+01:00"            # the page's clock: the greeting's Cube clock in TZ
SERVER_TIME = "2026-11-03T20:39:00+01:00"    # every time the server sends: 12 minutes ago
LOG_TIME = "2026-11-03T20:45:00+01:00"       # the log's state event with changes
STALE_TIME = "2026-11-02T14:51:00+01:00"     # the stale thermostat: 30 hours ago
TZ = "Europe/Zurich"
HEIGHT = {1100: 800, 390: 844}
PHONE = 390                                  # the width whose shots are all the viewport
PADDING = 12                                 # CSS px around an element shot
LOCALE = {"en": "en-US", "de": "de-DE"}
# Every tile rastered whole, so that a pixel depends on the page and not
# on what was painted before (the docstring): with partial raster, a few
# shots of every run were off by one level on the header's buttons.
CHROMIUM_ARGS = ["--disable-partial-raster"]
VERSION = "massimo v0.0.0-screenshots (go, linux/amd64)"
UPDATE = {"current": "v0.0.0-screenshots", "channel": "alpha", "latest": "v0.0.0-screenshots",
          "available": False, "can_update": False}
FOUND = {"serial": "KEQ0000000", "configured": False, "address": "192.168.1.77", "rf_address": "0a1b2c",
         "firmware": "1.1.3", "dhcp": True, "found_at": SERVER_TIME, "last_seen": SERVER_TIME}
CUBE_ADDRESS = "192.168.1.76:62910"         # the fake Cube's loopback address, which has run.sh's port
ADDRESS_RE = re.compile(r"127\.0\.0\.1:\d+")
# The backup history the settings show: one taken by hand, one after a change.
BACKUPS = {"backups": [
    {"id": 2, "taken_at": SERVER_TIME, "reason": "manual", "format": "json", "tool": VERSION, "size": 23994,
     "sha256": "0" * 64, "user": "owner", "note": "before the holidays"},
    {"id": 1, "taken_at": "2026-11-02T18:05:00+01:00", "reason": "auto", "format": "json", "tool": VERSION, "size": 23990,
     "sha256": "1" * 64}]}
TIME_RE = re.compile(r'"20\d\d-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?(Z|[+-]\d\d:\d\d)"')
# System settings → Maintenance's store and settings in effect: run.sh's
# temporary directory and ports would differ on every run, so placeholders.
STORE = {"file": "/var/lib/massimo/massimo.db", "schema": 8, "min_reader": 1, "binary": 8,
         "node": "00000000-0000-7000-8000-000000000001", "tenant": "00000000-0000-7000-8000-000000000002",
         "rows": {"audit": 412, "backups": 14, "commands": 230, "cubes": 1, "events": 5520, "preferences": 3,
                  "roles": 6, "settings": 4, "tokens": 18, "users": 3}}
CONFIG = {"version": "v0.0.0-screenshots", "file": "/etc/massimo/env", "file_found": True, "data_dir": "/var/lib/massimo",
          "listen": ":8099", "tls": "off", "log_level": "info (default)", "variables": [
              {"name": "MASSIMO_API_TOKEN", "secret": True, "source": "file"},
              {"name": "MASSIMO_CUBE_KEQ0000100_ADDRESS", "value": CUBE_ADDRESS, "source": "file"},
              {"name": "MASSIMO_CUBE_KEQ0000100_NAME", "value": "Fake Cube", "source": "file"},
              {"name": "MASSIMO_LISTEN", "value": ":8099", "source": "file"}]}
# Its reboot as on a box with the helper and the owner's switch: the
# server of the run is in a terminal, which reboots nothing.
REBOOT = {"available": True, "by": "systemd"}

# The pages, in the order of the set and of the slideshow, each with its
# frame: None for the viewport, else the element the shot is about.
PAGES = {
    "rooms": None,
    "rooms-devices": ".room",                       # the first tile, its device list open
    "config-temperatures": ".room",                 # the first tile with each configuration tab
    "config-devices": ".room",
    "config-week": ".room",
    "config-copy": ".room",
    "house": "#house",
    "log": None,                                    # its lines run the full width
    "user-menu": None,                              # the 👤 menu open over the rooms
    "cube-choice": None,                            # the Cube choice open over the rooms
    "profile": "#profile",                          # My profile, the owner's with keys only
    "prefs": "#prefs",                              # My settings
    "appearance": "#prefs .appearance",
    "system": "#system",                            # System settings
    "maintenance": "#sys-maint",                    # its Maintenance, the store and settings open
    "cube-found": "header.top",                     # the Cube choice with the found Cube under it
    "cubes": "#system .cubes-settings",
    "nav-menu": None,                               # ☰, which only a narrow screen shows
}

# The pages of the full width: all but ☰.
WIDE = [p for p in PAGES if p != "nav-menu"]

# The set: each run is one browser context, (width, theme, language,
# pages). The first is the main set and the slideshow's frames.
SETS = [
    (1100, "light", "en", WIDE),
    (1100, "dark", "en", WIDE),
    (390, "light", "en", ["rooms", "nav-menu", "cube-choice", "prefs", "system"]),
    (1100, "light", "de", ["rooms"]),
]


def name(page, width, theme, lang):
    return f"{page}-{width}-{theme}-{lang}.png"


def all_names():
    return [name(p, w, th, lang) for w, th, lang, pages in SETS for p in pages]


# The event stream: opens, sends nothing. The page dispatches nothing on
# it by itself; the poll covers the rest.
STREAM_STUB = """
window.EventSource = class extends EventTarget {
  constructor(url) {
    super();
    this.url = url;
    this.readyState = 0;
    this.onopen = null;
    this.onerror = null;
    setTimeout(() => {
      this.readyState = 1;
      if (this.onopen) this.onopen(new Event("open"));
    }, 0);
  }
  close() { this.readyState = 2; }
};
"""


class Answers:
    """The routes' fixed answers and the times they replace."""

    def __init__(self):
        self.found = False

    def api(self, route):
        req = route.request
        path = urlparse(req.url).path
        if path == "/api/v1/update":
            return route.fulfill(status=200, content_type="application/json", body=json.dumps(UPDATE))
        if path in ("/api/v1/store", "/api/v1/config"):
            body = STORE if path == "/api/v1/store" else CONFIG
            return route.fulfill(status=200, content_type="application/json", body=json.dumps(body))
        if path == "/api/v1/reboot" and req.method == "GET":
            return route.fulfill(status=200, content_type="application/json", body=json.dumps(REBOOT))
        if path.endswith("/events/ticket"):
            return route.continue_()
        if path.endswith("/backups") and req.method == "GET":
            # The backup history depends on when the run started; the set
            # shows a fixed one.
            return route.fulfill(status=200, content_type="application/json", body=json.dumps(BACKUPS))
        if path.startswith("/api/v1/me/preferences"):
            # Each shot keeps the settings of its own context: the server
            # keeps none for it, so that one shot's mode is not the next's.
            body = "{}" if req.method == "GET" else '{"ok": true}'
            return route.fulfill(status=200, content_type="application/json", body=body)
        resp = route.fetch()
        body = TIME_RE.sub('"' + SERVER_TIME + '"', resp.text())
        if path.startswith("/api/v1/cubes") and "/rooms" not in path:
            body = ADDRESS_RE.sub(CUBE_ADDRESS, body)
        if path.endswith("/log") and resp.ok:
            # The server's start says the build's version, another at
            # every commit: the fixed one, as /healthz.
            events = json.loads(body)
            for e in events if isinstance(events, list) else []:
                if e.get("kind") == "server_started":
                    e["summary"] = {"version": VERSION}
            body = json.dumps(events)
        if path == "/api/v1/cubes" and req.method == "GET" and self.found and resp.ok:
            cubes = json.loads(body)
            if not any(c.get("serial") == FOUND["serial"] for c in cubes):
                cubes.append(FOUND)
            body = json.dumps(cubes)
        route.fulfill(response=resp, body=body)

    def healthz(self, route):
        resp = route.fetch()
        h = resp.json()
        h.update(started_at=SERVER_TIME, version=VERSION, node_id="00000000-0000-0000-0000-000000000000")
        route.fulfill(response=resp, body=json.dumps(h))


def wait_until(page, expr, seconds):
    """Polls a page expression from here: the page's CSP rules out
    wait_for_function with a string."""
    end = time.time() + seconds
    while time.time() < end:
        if page.evaluate(expr):
            return True
        page.wait_for_timeout(100)
    raise TimeoutError("not reached: " + expr)


def to_top(page, locator):
    """Scrolls so that the element starts near the top of the viewport."""
    locator.evaluate("(e) => window.scrollTo(0, Math.max(0, e.getBoundingClientRect().top + window.scrollY - 12))")


def load(page, url):
    """The page as every shot starts: loaded, the rooms in, the first
    room's devices stale, nothing pending."""
    page.goto(url)
    page.wait_for_selector(".room", timeout=10000)
    wait_until(page, "state.eventsOpen && state.rooms.length > 0", 10)
    page.evaluate("""(iso) => {
      for (const x of state.rooms[0].devices) {
        const d = JSON.parse(JSON.stringify(x));
        d.stale = true;
        d.last_seen = iso;
        for (const k of Object.keys(d.received || {})) d.received[k] = iso;
        updateDevice(d);
      }
    }""", STALE_TIME)


def settle(page):
    """No request open, the fonts in, no focus ring, nothing moving."""
    page.wait_for_load_state("networkidle")
    page.evaluate("document.activeElement && document.activeElement.blur()")
    page.evaluate("document.fonts.ready.then(() => true)")
    page.wait_for_timeout(300)


def snap(page, path, frame):
    """Takes the shot: the viewport, or the element frame names grown by
    PADDING and clamped to the page. A clip of the full page takes an
    element taller than the viewport whole, where locator.screenshot()
    has no padding."""
    opts = dict(path=path, animations="disabled", caret="hide", scale="css")
    if frame is None:
        return page.screenshot(**opts)
    # The header and the page's row stay at the top of the window as it
    # scrolls, and ↑ floats over it (components.js stickyHeader, pageBar,
    # backToTop): an element's shot is of the element alone. Through the
    # elements' style, since the page's CSP takes no style element.
    page.evaluate("""() => {
      for (const e of document.querySelectorAll('.top, .page-bar')) e.style.position = 'static';
      for (const e of document.querySelectorAll('.to-top')) e.style.display = 'none';
    }""")
    el = page.locator(frame).first
    el.scroll_into_view_if_needed()
    b = el.evaluate("""(e) => {
      const r = e.getBoundingClientRect(), d = document.documentElement;
      return { x: r.left + window.scrollX, y: r.top + window.scrollY, w: r.width, h: r.height,
               pw: d.scrollWidth, ph: d.scrollHeight };
    }""")
    x0, y0 = max(0, math.floor(b["x"] - PADDING)), max(0, math.floor(b["y"] - PADDING))
    x1 = min(b["pw"], math.ceil(b["x"] + b["w"] + PADDING))
    y1 = min(b["ph"], math.ceil(b["y"] + b["h"] + PADDING))
    page.screenshot(clip={"x": x0, "y": y0, "width": x1 - x0, "height": y1 - y0}, full_page=True, **opts)


def open_config(page, tab):
    tile = page.locator(".room").first
    tile.locator(".tile-cog").click()
    page.wait_for_selector(".room .config-body")
    tile.locator(".config-tabs [role=tab]").nth(tab).click()
    to_top(page, tile)


def step(page, which):
    """Brings the loaded page to what the shot shows."""
    tile = page.locator(".room").first
    if which == "rooms":
        return
    if which == "rooms-devices":
        tile.locator(".room-name-btn").click()
        page.wait_for_selector(".room .device")
        to_top(page, tile)
    elif which.startswith("config-"):
        tabs = {"config-temperatures": 1, "config-devices": 2, "config-week": 3, "config-copy": 4}
        open_config(page, tabs[which])
    elif which == "house":
        page.locator("#house-manu").click()
        page.locator("#house-panel").wait_for(state="visible")
        to_top(page, page.locator("#house"))
    elif which == "log":
        show_view(page, "log")
        page.wait_for_selector("#logview .log-line")
        # After the connect burst (1.5 s), which the log leaves out.
        page.wait_for_timeout(1600)
        page.evaluate("""(time) => {
          const d = JSON.parse(JSON.stringify(state.rooms.find((r) => r.name === "Kind 1").devices[0]));
          state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({
            kind: "state", device: d.rf_address, state: d, time,
            changes: { window_open: { from: false, to: true }, setpoint: { from: 18.5, to: 12 } } }) }));
        }""", LOG_TIME)
        page.wait_for_selector("#logview .log-line.kind-state >> nth=-1")
    elif which == "user-menu":
        open_user_menu(page)
    elif which == "nav-menu":
        page.locator("#nav-menu summary").click()
        page.locator("#nav-menu .menu-items").wait_for(state="visible")
    elif which == "cube-choice":
        page.locator("#cube-choice").click()
        page.locator("#cube-menu .menu-items").wait_for(state="visible")
    elif which in ("prefs", "system", "profile"):
        show_view(page, which)
    elif which == "maintenance":
        show_view(page, "system")
        wait_until(page, "document.getElementById('store-info-sum').textContent !== ''", 5)
        page.locator("#store-info summary").click()
        page.locator("#config-info summary").click()
        to_top(page, page.locator("#sys-maint"))
    elif which == "appearance":
        show_view(page, "prefs")
        page.locator("#scheme-customise").click()
        page.locator("#scheme-editor").wait_for(state="visible")
        to_top(page, page.locator("#prefs .appearance"))
    elif which == "cube-found":
        found = page.locator("#cube-found")
        found.wait_for(state="visible")
        found.locator("summary").click()
        found.locator(".env-lines").wait_for(state="visible")
    elif which == "cubes":
        show_view(page, "system")
        panel = page.locator(".cube-panel[data-serial='KEQ0000100']")
        panel.locator("summary").click()
        panel.locator("form.cube-form").wait_for(state="visible")
        page.locator(".cube-found-row[data-serial='KEQ0000000']").wait_for(state="visible")
        to_top(page, page.locator("#system .cubes-settings"))
    else:
        raise ValueError("no such page: " + which)


def shoot(browser, url, token, width, theme, lang, pages, problems):
    ctx = browser.new_context(viewport={"width": width, "height": HEIGHT[width]}, device_scale_factor=1,
                              locale=LOCALE[lang], timezone_id=TZ, color_scheme=theme, reduced_motion="reduce")
    ctx.add_init_script(f"localStorage.setItem('massimo.token', {token!r});"
                        f"localStorage.setItem('massimo.theme', {theme!r});" + STREAM_STUB)
    answers = Answers()
    ctx.route("**/api/v1/**", answers.api)
    ctx.route("**/healthz", answers.healthz)
    page = ctx.new_page()
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: problems.append(f"{width} {theme} {lang}: page error: {e}"))
    page.clock.set_fixed_time(NOW)
    for which in pages:
        file = name(which, width, theme, lang)
        # From the Cube choice's shot on, the server lists a found Cube
        # from the page's load.
        answers.found = answers.found or which == "cube-found"
        try:
            load(page, url)
            if page.evaluate("document.documentElement.dataset.theme") != theme:
                raise RuntimeError("data-theme is not " + theme)
            step(page, which)
            settle(page)
            snap(page, str(OUT / file), None if width == PHONE else PAGES[which])
            print("shot  " + file)
        except Exception as e:  # a timeout or a missing element
            problems.append(f"{file}: {e}")
            print("FAIL  " + file + ": " + str(e))
    ctx.close()


def main():
    if sys.argv[1:] == ["--list"]:
        for p in SETS[0][3]:
            print("docs/screenshots/" + name(p, *SETS[0][:3]))
        return 0
    from playwright.sync_api import sync_playwright

    url = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8119/"
    token = sys.argv[2] if len(sys.argv) > 2 else ""
    OUT.mkdir(parents=True, exist_ok=True)
    for old in OUT.glob("*.png"):
        old.unlink()
    problems = []
    with sync_playwright() as pw:
        browser = pw.chromium.launch(args=CHROMIUM_ARGS)
        for width, theme, lang, pages in SETS:
            shoot(browser, url, token, width, theme, lang, pages, problems)
        browser.close()
    if problems:
        print(f"ui-screenshots: {len(problems)} shot(s) not taken")
        for p in problems:
            print("  " + p)
        return 1
    print(f"ui-screenshots: {len(all_names())} shots in {OUT}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
