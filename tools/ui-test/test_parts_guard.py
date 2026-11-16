# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The guard of the page's parts (AP UiParts, Block 3): data that comes
in with nothing changed takes nothing away from what the user holds.

REGIONS is the page cut into its regions, each with the containers it
lives in and its views: how a user brings it on screen and opens what it
has to open (a menu, a panel, a form). NOT_MOVED is the list of the
regions not yet rewritten as parts that fail the guard today, each with
what it loses; the block that moves a region takes its line off.

For a region off the list, in each of its views and around each data
source of SOURCES, applied with the data as it was (GUARD_JS, round):

1. identity: every element a user can hold (a button, a field, a
   summary, a link, a details, anything with a tabindex) shown in the
   region is the same element afterwards, still in the document;
2. open: every details that was open is open;
3. typed: every field shown that takes text, a number, a date or a
   time holds the value typed into it;
4. focus: the element of the region that had the focus has it;

and checks 1 to 3 once more after the focus has left the region, where
a redraw put off while the user was in it comes. Typing sets the value
and sends an input event, as a key does, never a change event, Enter or
a click on a button: nothing is saved. Every request other than a GET
is refused and counted during the whole test, and the test fails on
one: the guard sends nothing to the Cube and changes nothing in the
server's store (Writes says the one exception, outside the rounds).

A region on the list is still shown, so that the table stays true to
the page, and then skipped with its note, which the run's summary
(pytest -rs) shows. UI_GUARD_CHECK, comma-separated region names or
"all", checks regions although they are on the list: how a block sees
that its region passes before it takes the line off, and how the guard
is shown to bite.

The guard runs in two of the suite's four cases, 1100 px in English and
390 px in German (GUARD_CASES, D176-D): each width once and each
language once, since a width changes what is shown and a language
changes no element. In CI that is once in each UI job, on a worker of
its own beside the other files.
"""
import json
import os
from dataclasses import dataclass
from typing import Callable

import pytest

from helpers import open_main_menu, show_view, wait_until

# The cases the guard runs in, as (width, language) (D176-D).
GUARD_CASES = {(1100, "en"), (390, "de")}


@pytest.fixture(autouse=True)
def guard_case(case):
    """Skips the guard in the cases it does not run in, before a page is
    loaded for it."""
    if (case.width, case.lang) not in GUARD_CASES:
        pytest.skip("the guard runs at 1100 px in English and at 390 px in German")


@dataclass(frozen=True)
class View:
    """One state of a region: a name for the messages, the function that
    brings it about from wherever the page is, and, where the default
    does not fit, the element given the focus (a CSS selector within the
    region)."""

    name: str
    show: Callable
    focus: str = ""


@dataclass(frozen=True)
class Region:
    """A region of the page: the CSS selectors of its containers, the
    elements inside them that belong to another region (exclude), its
    views, and, where the fake Cube's fixture cannot show it, why
    (gap)."""

    name: str
    containers: tuple
    views: tuple = ()
    exclude: str = ""
    gap: str = ""


# --- how the regions are shown ------------------------------------------

def settle(page, ms=300):
    """Lets the loads a view starts come back before the guard marks it."""
    page.wait_for_timeout(ms)


def tile(page, n=0):
    return page.locator("#rooms .room").nth(n)


def on_rooms(page):
    """Rooms shown, with what the view before opened there closed: an
    open menu would lie over the next one's summary (Escape closes the
    menus, the holiday form and the House panel)."""
    if not page.locator("#rooms").is_visible():
        show_view(page, "tiles")
    page.keyboard.press("Escape")


def rooms_menu(page):
    on_rooms(page)
    tile(page).locator(".tile-head > details.menu > summary").click()


def rooms_more(page):
    on_rooms(page)
    tile(page).locator(".more-sum").click()


def rooms_problems(page):
    on_rooms(page)
    room = page.locator("#rooms .room", has=page.locator(".room-problems details.problems")).first
    room.locator(".room-problems .problems-sum").click()


def rooms_holiday_and_devices(page):
    on_rooms(page)
    tile(page).locator(".more-sum").click()
    tile(page).locator(".more-menu .menu-holiday").click()
    tile(page).locator(".vacation-panel").wait_for(state="visible")
    tile(page, 1).locator(".tile-info").click()
    tile(page, 1).locator(".devices .device").first.wait_for(state="visible")


def expert(page):
    """Expert mode on for this page alone: the setting in the page's
    state, not setSetting, which would store it on the server."""
    page.evaluate("() => { state.settings.expert = true; applySettings(); }")


def rooms_virtual(page):
    on_rooms(page)
    expert(page)
    room = tile(page, 2)
    room.locator(".tile-info").click()
    room.locator(".devices .device.virtual form.vwt-settings").wait_for(state="visible")
    settle(page, 500)  # the virtual thermostat's settings load and fill the form


def room_config(tab):
    def show(page):
        on_rooms(page)
        room = tile(page)
        if room.locator(".config-area .config-tabs").count() == 0:
            room.locator(".tile-cog").click()
            room.locator(".config-area .config-tabs").wait_for(state="visible")
        room.locator(f".config-tabs [role=tab][data-tab={tab}]").click()
        settle(page)
    return show


# A device without a room, as GET …/devices gives one the original
# software paired and never put into a room: the fixture has none, and one
# joins only through pairing, a write to the fake Cube.
UNLISTED_STUB = {"rf_address": "0a0f01", "type": "radiator_thermostat", "name": "", "room_id": 0, "serial": "MEQ0000901",
                 "unlisted": True, "reported": False}


def unlisted_form(page):
    """The devices without a room with UNLISTED_STUB, GET …/devices
    answered with the server's answer and it at every load, and its form
    that puts it into a room open."""
    on_rooms(page)

    def handle(route):
        try:
            devices = route.fetch().json() or []
            route.fulfill(status=200, content_type="application/json", body=json.dumps(devices + [UNLISTED_STUB]))
        except Exception:  # the route is already handled, or the page closed
            pass
    page.route("**/api/v1/cubes/*/devices?devices=real", handle)
    page.evaluate("loadRooms()")
    row = page.locator("#unlisted .device[data-rf='0a0f01']")
    row.wait_for(state="visible")
    row.locator(".put-open").click()
    row.locator("form input[name=name]").wait_for(state="visible")
    settle(page)


def house_panel(page):
    on_rooms(page)
    page.locator("#house-auto").click()
    page.locator("#house-panel").wait_for(state="visible")


def cube_choice(page):
    on_rooms(page)
    page.locator("#cube-choice").click()
    page.locator("#cube-menu .menu-items").wait_for(state="visible")


def cube_choice_found(page):
    """The Cube choice open, the Cube with problems and the Cubes found
    listed (stub_cubes)."""
    stub_cubes(page)
    on_rooms(page)
    if page.locator("#cube-menu").get_attribute("open") is None:
        page.locator("#cube-choice").click()
    page.locator("#cube-menu .menu-items button[data-value^='found:']").first.wait_for(state="visible")


def header_problems(page):
    """Every Cube's problem list open beside the Cube choice, with a
    device, the settings and the Cubes found in it (stub_cubes): the fault
    of AP UiParts Block 6."""
    stub_cubes(page)
    on_rooms(page)
    page.locator("#cube-problems .problems-sum").click()
    page.locator("#cube-problems .problems-list").wait_for(state="visible")


def main_menu(page):
    on_rooms(page)
    open_main_menu(page)


def home_notice(page):
    """The second server of run.sh, whose Cube is not connected: the
    only one that shows the notice."""
    url = os.environ.get("UI_TEST_DOWN_URL", "")
    if not url:
        pytest.skip("run.sh starts the second server, the one that shows the notice")
    page.goto(url)
    assert wait_until(page, "!document.getElementById('home-notice').classList.contains('hidden')", 10), "the notice shows"
    loaded(page)
    settle(page)


def system(page):
    if not page.locator("#system").is_visible():
        show_view(page, "system")
    settle(page)


# The fake Cube's fixture lists nothing in a Cube's problem list until
# its open window is five minutes old, and finds no Cube until a search,
# which is a POST. The Cubes' views and the header's therefore answer
# GET /api/v1/cubes with the server's answer and, from the start, a
# device with a low battery, a line of the settings file that differs,
# an address found elsewhere and a proxy that does not listen on the
# fake Cube, and a Cube and a Massimo proxy found on the network, the
# Cube outside this machine's networks: the same answer at every load,
# so that the data stay as they were.
CUBES_STUB = {
    "problems": [{"rf_address": "0a0101", "type": "radiator_thermostat", "name": "Radiator", "room": "Kitchen", "serial": "MEQ0000101",
                  "reported": True, "state": {"error": False, "battery_low": True, "link_error": False}}],
    "file_differs": [{"field": "name", "variable": "MASSIMO_CUBE_KEQ0000100_NAME", "file": "File name", "store": "Fake Cube"}],
    "found_elsewhere": {"address": "192.0.2.24:62910", "configured": "192.0.2.23:62910", "since": "2026-11-14T08:00:00+01:00"},
    "proxy_failing": [{"addr": "192.0.2.10:62910", "error": "bind: cannot assign requested address", "since": "2026-11-13T08:00:00+01:00",
                       "next_try": "2026-11-13T08:02:30+01:00", "tries": 9}],
}
FOUND_STUB = [
    {"serial": "KEQ0000300", "kind": "cube", "configured": False, "address": "192.168.0.222", "rf_address": "0a0301", "firmware": "1.1.3",
     "dhcp": False, "other_network": True, "found_at": "2026-11-14T08:00:00+01:00", "last_seen": "2026-11-14T08:00:00+01:00"},
    {"serial": "KEQ0000400", "kind": "proxy", "configured": False, "address": "192.0.2.40", "rf_address": "0a0401", "firmware": "",
     "serves": "KEQ0000300", "host": {"name": "other", "url": "http://192.0.2.40:8080"},
     "found_at": "2026-11-14T08:00:00+01:00", "last_seen": "2026-11-14T08:00:00+01:00"},
]


def stub_cubes(page):
    """Answers GET /api/v1/cubes with CUBES_STUB and FOUND_STUB, once per
    page, and loads the list."""
    if getattr(page, "_guard_cubes", False):
        return

    def handle(route):
        try:
            cubes = route.fetch().json()
            for c in cubes:
                if c.get("serial") == "KEQ0000100":
                    c.update(CUBES_STUB)
            route.fulfill(status=200, content_type="application/json", body=json.dumps(cubes + FOUND_STUB))
        except Exception:  # the route is already handled, or the page closed
            pass
    page.route("**/api/v1/cubes", handle)
    page._guard_cubes = True
    page.evaluate("loadCubes()")
    settle(page)


def cubes_closed(page):
    stub_cubes(page)
    system(page)
    page.locator("#cubes-list .cube-panel").first.wait_for()


def cubes_open(page):
    cubes_closed(page)
    page.keyboard.press("Escape")  # an open problem list lies over the head at 390 px
    panel = page.locator("#cubes-list .cube-panel").first
    if panel.get_attribute("open") is None:
        panel.locator(":scope > summary .cube-head-who").click()
    panel.locator(".cube-form").wait_for(state="visible")
    settle(page)


def cubes_problems(page):
    """A closed Cube's problem list open: the fault the owner reported."""
    cubes_closed(page)
    panel = page.locator("#cubes-list .cube-panel").first
    if panel.get_attribute("open") is not None:
        panel.locator(":scope > summary .cube-head-who").click()
    panel.locator(":scope > summary .cube-pill .problems-sum").click()
    panel.locator(":scope > summary .cube-pill .problems-list").wait_for(state="visible")


def cubes_found(page):
    cubes_closed(page)
    page.keyboard.press("Escape")
    fold = page.locator("#cubes-list .found-panel")
    if fold.get_attribute("open") is None:
        fold.locator(":scope > summary").click()
    lines = fold.locator(".found-lines")
    if lines.get_attribute("open") is None:
        lines.locator(":scope > summary").click()
    fold.locator(".cube-found-row").first.wait_for(state="visible")


def cube_page(page):
    stub_cubes(page)
    if not page.locator("#cube-page").is_visible():
        show_view(page, "cube")
    page.locator("#cube-page-body .cube-panel .cube-form").wait_for(state="visible")
    settle(page)


def cube_page_problems(page):
    cube_page(page)
    page.locator("#cube-page-body .cube-pill .problems-sum").click()
    page.locator("#cube-page-body .cube-pill .problems-list").wait_for(state="visible")


def pairing_form(page):
    cube_page(page)
    page.locator("#cube-page-body .cube-pairing .pair-open").click()
    page.locator("#cube-page-body .cube-pairing .pairing-form").wait_for(state="visible")


def users(page):
    system(page)
    panel = page.locator(".user-panel[data-name='operator']")
    panel.locator("summary").click()
    assert wait_until(page, "document.querySelectorAll(\".user-panel[data-name='operator'] .user-tokens li\").length >= 1", 5), \
        "the operator's tokens are listed"
    page.locator("#user-add").click()
    page.locator("#user-new input").first.wait_for(state="visible")


def roles(page):
    system(page)
    page.locator(".role-panel[data-key='operator'] summary").click()
    page.locator("#role-add").click()
    page.locator("#role-new input").first.wait_for(state="visible")


def providers(page):
    system(page)
    page.locator("#provider-add").click()
    page.locator("#provider-new input").first.wait_for(state="visible")


def backups_system(page):
    system(page)
    page.locator("#backups-cubes .backup-fold[open] .backup-take").wait_for(state="visible")


def backups_cube(page):
    cube_page(page)
    page.locator("#cube-backup-body .backup-take").wait_for(state="visible")


def log(page):
    show_view(page, "log")
    page.locator("#log-lines .log-line").first.wait_for()
    settle(page)


def maint(page):
    if not page.locator("#maint-page").is_visible():
        show_view(page, "maint")
    settle(page, 500)


def info(page):
    show_view(page, "info")
    page.locator("#store-info-sum").click()
    page.locator("#config-info-sum").click()
    settle(page, 500)


def personal(view):
    def show(page):
        show_view(page, view)
        settle(page)
    return show


def profile(page):
    personal("profile")(page)
    if page.locator("#me-password").is_visible():
        page.locator("#me-password-title").click()


def scheme_editor(page):
    personal("prefs")(page)
    page.locator("#scheme-customise").click()
    page.locator("#scheme-editor input").first.wait_for(state="visible")


def sections_menu(page):
    system(page)
    page.locator("#system .sec-menu > summary").click()
    page.locator("#system .sec-menu .menu-items").wait_for(state="visible")


# --- the regions ----------------------------------------------------------

# Every region of the page, the header first, then Rooms, the settings
# pages, the personal page, the page's chrome and the sign-in. A region's
# views run one after the other in one page, each from where the one
# before left it.
REGIONS = (
    Region("cube-choice", ("#cube-menu",), (
        View("the Cube choice open", cube_choice),
        View("the Cube choice open with the Cubes found", cube_choice_found),
    )),
    Region("cube-problems", ("#cube-problems",), (View("every Cube's problem list open", header_problems),)),
    Region("main-menu", ("header.top .top-facts", "header.top nav.nav"), (View("☰ open", main_menu),)),
    Region("home-notice", ("#home-notice",), (View("on the second server, its Cube not connected", home_notice),)),
    Region("rooms", ("#rooms",), (
        View("a tile's ⋯ menu open", rooms_menu),
        View("a tile's More menu open", rooms_more),
        View("a tile's problems list open", rooms_problems),
        View("a tile's holiday form, another's device list", rooms_holiday_and_devices),
        View("the virtual thermostat's settings, expert mode", rooms_virtual),
    ), exclude=".config-area"),
    Region("room-config", ("#rooms .room .config-area",),
           tuple(View(f"a tile's configuration, tab {tab}", room_config(tab)) for tab in ("room", "temps", "devices", "program", "copy"))),
    Region("house", ("#house",), (View("the House panel open", house_panel),)),
    Region("unlisted", ("#unlisted",), (View("a device without a room, its form that puts it into a room open", unlisted_form),)),
    Region("cubes-list", ("#cubes-list", "#cubes-msg"), (
        View("every panel closed", cubes_closed, focus=".cube-panel > summary"),
        View("a closed Cube's problem list open", cubes_problems),
        View("a Cube's panel open", cubes_open),
        View("the found Cubes' fold and its lines open", cubes_found),
    ), exclude=".cube-pairing"),
    Region("cube-settings", ("#cube-page-body", "#cube-page-msg"), (
        View("the chosen Cube's panel", cube_page),
        View("its problem list open", cube_page_problems),
    ), exclude=".cube-pairing"),
    Region("pairing", ("#cube-page-body .cube-pairing",), (View("the pairing form on Cube settings", pairing_form),)),
    Region("users", ("#sys-users-body",), (View("the operator's panel and the new user's form", users),)),
    Region("roles", ("#sys-roles-body",), (View("the operator role's panel and the new role's form", roles),)),
    Region("capabilities", ("#sys-caps-body",), (View("System settings", system),)),
    Region("providers", ("#sys-providers-body",), (View("the new provider's form", providers),)),
    Region("integrations", ("#sys-integrations-body",), (View("System settings", system),)),
    Region("backups", ("#sys-backup-body", "#cube-backup-body"), (
        View("System settings, the Cube's fold open", backups_system),
        View("Cube settings", backups_cube),
    )),
    Region("log", ("#logview",), (View("the log", log),)),
    Region("software-update", ("#sys-software-body",), (View("Maintenance", maint),)),
    Region("maintenance", ("#sys-maint-body",), (View("Maintenance", maint),)),
    Region("system-info", ("#sys-info-body",), (View("System info, its folds open", info),)),
    Region("my-settings", ("#prefs-sec > .dev-head", "#prefs-sec > .dev-row", "#prefs-sec > p"), (View("My settings", personal("prefs")),)),
    Region("appearance", ("#prefs-sec .appearance",), (
        View("My settings", personal("prefs")),
        View("the scheme editor open", scheme_editor),
    )),
    Region("notifications", ("#notify-sec",), (View("Notifications", personal("notify")),)),
    Region("my-profile", ("#profile-sec",), (View("My profile", profile),)),
    Region("page-chrome", ("#system .page-bar", "#system .section-head"), (View("System settings, its sections menu open", sections_menu),)),
    Region("sign-in", ("#login-dialog",),
           gap="shown only to a page without a token, which loads no data while it is open: the poll and the stream start after the sign-in"),
)

# The regions not yet moved to parts that the guard fails today, each
# with what it loses; the block that moves a region takes its line off
# (AP UiParts, "How a region moves"). A region that is not here is
# checked. The others passed the guard on the day it was written
# (2026-11-15), although none has moved: they are filled in place, or
# drawn again only by the user's own action (Create key, a backup, a
# command's detail), which the guard does not apply. The block that
# moves one of them keeps it passing. The sign-in, which the fixture
# cannot show, stays here until it can; the devices without a room,
# which it cannot show either, are given one by their view's stub.
NOT_MOVED = {
    "room-config": "plain inside its tile (config.js, Block 8): a data event fills it again, but while the focus is in it; "
                   "once the focus leaves, the fill put off comes: every element is new, and what is typed and not yet "
                   "taken by a change event is gone",
    "house": "a rooms reload builds the rooms' checkboxes again (renderHouseRooms): the ticks are carried over, the focus is not",
    "sign-in": "the dialog is filled again at each askLogin",
}


def checked_regions():
    """The regions UI_GUARD_CHECK names to be checked although they are
    on the list: comma-separated names, or all. An unknown name stops
    the run, so that a typo does not check nothing."""
    names = {n.strip() for n in os.environ.get("UI_GUARD_CHECK", "").split(",") if n.strip()}
    if names == {"all"}:
        return {r.name for r in REGIONS}
    unknown = names - {r.name for r in REGIONS}
    if unknown:
        raise pytest.UsageError(f"UI_GUARD_CHECK names {', '.join(sorted(unknown))}; the regions are {', '.join(r.name for r in REGIONS)}")
    return names


CHECK = checked_regions()

# The data sources, each applied with the data as it was, by its name in
# GUARD_JS's run: the poll the timer runs (poll(false)), the Cubes' list
# (loadCubes, also the stream's hello and disconnected), the rooms
# (loadRooms, also metadata, paired and device_forgotten), the whole
# reload (refresh, the stream's connected and resync), a state event of
# the stream for every device of the Cube, as the server sends it, the
# log button's sign (loadLogAlerts), the ages' minute (refreshAges) and
# the log's poll under All Cubes (pollLog).
SOURCES = ("poll(false)", "loadCubes()", "loadRooms()", "refresh()", "a state event per device", "loadLogAlerts()",
           "refreshAges()", "pollLog()")

# The guard in the page, window.__guard: marks what the user can hold in
# a region, types into its fields, gives it the focus, applies a data
# source and reads it all again. A path is the element's way from the
# nearest ancestor with an id, readable in a message and found again by
# querySelector once the element itself is replaced.
GUARD_JS = r"""() => {
  if (window.__guard) return;
  const HOLD = "button, input, select, textarea, summary, a[href], [tabindex], details";
  const VALUES = {
    text: ["guard text", "guard text 2"], search: ["guard text", "guard text 2"], email: ["guard@example.org", "guard2@example.org"],
    url: ["https://example.org/", "https://example.org/2"], tel: ["0123456789", "0123456780"], password: ["guard-pass", "guard-pass-2"],
    date: ["2031-02-03", "2031-03-04"], time: ["12:30", "13:00"], "datetime-local": ["2031-02-03T12:30", "2031-03-04T13:00"],
    month: ["2031-02", "2031-03"], week: ["2031-W05", "2031-W06"],
  };
  const STATE_CLASSES = new Set(["hidden", "busy", "active", "open", "selected", "copied", "changed", "invalid", "pending"]);
  const ATTRS = ["data-id", "data-name", "data-key", "data-serial", "data-rf", "data-tab", "data-target", "name", "type", "value"];
  const frames = () => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(done, 150))));
  // shown: rendered, and not in a closed details, whose content keeps
  // its boxes (content-visibility) and would count by getClientRects.
  const shown = (el) => el.getClientRects().length > 0 && el.checkVisibility();

  function step(el) {
    if (el.id) return "#" + CSS.escape(el.id);
    let s = el.tagName.toLowerCase();
    for (const c of [...el.classList].filter((c) => !STATE_CLASSES.has(c)).slice(0, 2)) s += "." + CSS.escape(c);
    for (const a of ATTRS) if (el.hasAttribute(a)) s += "[" + a + '="' + CSS.escape(el.getAttribute(a)) + '"]';
    const p = el.parentElement;
    if (p) {
      const same = [...p.children].filter((x) => x.tagName === el.tagName);
      if (same.length > 1) s += ":nth-of-type(" + (same.indexOf(el) + 1) + ")";
    }
    return s;
  }
  function path(el) {
    const parts = [];
    for (let e = el; e && e !== document.body && e !== document.documentElement; e = e.parentElement) {
      parts.unshift(step(e));
      if (e.id) break;
    }
    return parts.join(" > ") || el.tagName.toLowerCase();
  }
  function find(p) {
    try {
      return document.querySelector(p);
    } catch (e) {
      return null;
    }
  }
  function describe(el) {
    return !el || el === document.body ? "the page's body" : path(el);
  }
  function holdables(roots, exclude) {
    const out = new Set();
    for (const sel of roots) {
      for (const root of document.querySelectorAll(sel)) {
        for (const el of [root, ...root.querySelectorAll(HOLD)]) {
          if (!el.matches(HOLD) || !shown(el)) continue;
          if (el.tagName === "INPUT" && el.type === "hidden") continue;
          if (exclude && el.closest(exclude)) continue;
          out.add(el);
        }
      }
    }
    return [...out];
  }
  // valueFor is a value valid for the field's type, other than the one
  // it holds; null for a field that takes no text (a checkbox, a radio,
  // a file, a colour, a range).
  function valueFor(el) {
    const type = el.tagName === "TEXTAREA" ? "text" : (el.getAttribute("type") || "text").toLowerCase();
    if (type === "number") {
      const min = parseFloat(el.min), max = parseFloat(el.max);
      const st = el.step === "any" || !(parseFloat(el.step) > 0) ? 1 : parseFloat(el.step);
      const base = isNaN(min) ? 0 : min;
      for (const k of [1, 2, 0, 3]) {
        const v = String(Number((base + k * st).toFixed(6)));
        if ((isNaN(max) || base + k * st <= max) && v !== el.value) return v;
      }
      return null;
    }
    if (!(type in VALUES)) return null;
    const cut = (v) => (el.maxLength > 0 ? v.slice(0, el.maxLength) : v);
    const [a, b] = VALUES[type].map(cut);
    return el.value === a ? b : a;
  }
  // typeAll types into every field of the region that takes text: the
  // value set and an input event sent, as a key does; no change event,
  // so that nothing is saved.
  function typeAll(roots, exclude) {
    const typed = [];
    for (const el of holdables(roots, exclude)) {
      if (!(el.tagName === "INPUT" || el.tagName === "TEXTAREA") || el.disabled || el.readOnly) continue;
      const v = valueFor(el);
      if (v === null) continue;
      el.value = v;
      if (el.value !== v) continue; // a value the field refuses
      el.dispatchEvent(new Event("input", { bubbles: true }));
      typed.push({ el, value: v, path: path(el) });
    }
    return typed;
  }
  function stateEvents() {
    const devices = [];
    for (const r of state.rooms) {
      devices.push(...r.devices);
      if (r.vwt) devices.push(r.vwt);
    }
    devices.push(...state.unlisted);
    for (const d of devices) {
      const copy = JSON.parse(JSON.stringify(d));
      if (state.events) {
        state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({
          kind: "state", device: copy.rf_address, state: copy, time: new Date().toISOString() }) }));
      } else {
        updateDevice(copy);
      }
    }
  }
  async function run(source) {
    switch (source) {
      case "poll(false)": return poll(false);
      case "loadCubes()": return loadCubes();
      case "loadRooms()": return loadRooms();
      case "refresh()": return refresh();
      case "a state event per device": return stateEvents();
      case "loadLogAlerts()": return loadLogAlerts();
      case "refreshAges()": return refreshAges();
      case "pollLog()": return pollLog();
    }
    throw new Error("the guard knows no data source " + source);
  }
  function check(out, when, marked, open, typed, focused) {
    for (const m of marked) {
      if (!m.el.isConnected) out.push({ check: "identity" + when, path: m.path, detail: find(m.path) ? "replaced by a new element" : "gone" });
    }
    for (const m of open) {
      const el = m.el.isConnected ? m.el : find(m.path);
      if (!el || !el.open) out.push({ check: "open" + when, path: m.path, detail: el ? "closed" : "gone" });
    }
    for (const m of typed) {
      const el = m.el.isConnected ? m.el : find(m.path);
      if (!el) out.push({ check: "typed" + when, path: m.path, detail: "gone with " + JSON.stringify(m.value) });
      else if (el.value !== m.value) out.push({ check: "typed" + when, path: m.path, detail: "holds " + JSON.stringify(el.value) + ", not " + JSON.stringify(m.value) });
    }
    if (focused && document.activeElement !== focused.el) {
      out.push({ check: "focus" + when, path: focused.path, detail: "the focus is on " + describe(document.activeElement) });
    }
  }
  window.__guard = {
    // count is how many elements a user can hold the region shows.
    count({ roots, exclude }) {
      return holdables(roots, exclude).length;
    },
    // round is the guard around one data source.
    async round({ roots, exclude, focus, source }) {
      if (state.pollTimer) { // the timer's own poll would come at any time
        clearInterval(state.pollTimer);
        state.pollTimer = null;
      }
      const out = [];
      const typing = typeAll(roots, exclude);
      await frames();
      // the typing alone, before any data: a field it replaced or set back
      check(out, " (by the typing, before the data)", [], [], typing, null);
      const marked = holdables(roots, exclude).map((el) => ({ el, path: path(el) }));
      const typed = typing.filter((m) => m.el.isConnected && m.el.value === m.value);
      const open = marked.filter((m) => m.el.tagName === "DETAILS" && m.el.open);
      const inOpen = (m) => m.el.tagName !== "SUMMARY" && open.some((o) => o.el !== m.el && o.el.contains(m.el));
      const pick = (focus && marked.find((m) => m.el.matches(focus))) || typed[0] || marked.find(inOpen) || marked[0];
      let focused = null;
      if (pick) {
        pick.el.focus();
        if (document.activeElement === pick.el) focused = { el: pick.el, path: pick.path };
      }
      await run(source);
      await frames();
      check(out, "", marked, open, typed, focused);
      const active = document.activeElement;
      if (active && active !== document.body && marked.some((m) => m.el === active)) {
        active.blur();
        await frames();
        check(out, " once the focus left", marked, open, typed, null);
      }
      return { marked: marked.length, typed: typed.length, open: open.length, focused: focused ? focused.path : "", failures: out };
    },
  };
}"""


class Writes:
    """Refuses every request to the API other than a GET, and lists it
    in sent: the guard sends nothing to the Cube and changes nothing in
    the server's store. The refusal shows in the console as well, which
    conftest's guard reports too. One exception, outside the rounds
    only: the event stream's ticket that a page asks for as it loads
    (the home notice's view loads the second server's page), which the
    server keeps in memory, not in its store, and which reaches no
    Cube; it is let through and listed in let. During the rounds
    (rounds true) nothing but a GET goes."""

    TICKET = "/events/ticket"

    def __init__(self, page):
        self.sent, self.let, self.rounds = [], [], False
        page.route("**/api/**", self.handle)

    def handle(self, route):
        req = route.request
        if req.method == "GET":
            route.continue_()
        elif not self.rounds and req.method == "POST" and req.url.endswith(self.TICKET):
            self.let.append(f"{req.method} {req.url}")
            route.continue_()
        else:
            self.sent.append(f"{req.method} {req.url}{' (during the rounds)' if self.rounds else ''}")
            route.abort()


def loaded(page):
    """A page just loaded: its first sync of the settings to the server,
    which it schedules for a second later because conftest removed them
    there, is dropped (login.js takeScheduled), so that the store stays
    as conftest left it."""
    page.evaluate("() => { takeScheduled(); }")


def guard_view(page, writes, region, view):
    """The guard around every data source in one view of a region; the
    failures as lines that name the region, the view, the source, the
    check and the element, and what was held, per source."""
    args = {"roots": list(region.containers), "exclude": region.exclude, "focus": view.focus}
    lines, held = [], []
    for source in SOURCES:
        page.evaluate(GUARD_JS)
        writes.rounds = True
        try:
            res = page.evaluate("(a) => window.__guard.round(a)", {**args, "source": source})
        finally:
            writes.rounds = False
        held.append(f"{source}: {res['marked']} held, {res['typed']} typed, {res['open']} open, focus {res['focused'] or 'none'}")
        groups = {}
        for f in res["failures"]:
            groups.setdefault(f["check"], []).append(f)
        for check, fs in groups.items():
            shown = "; ".join(f"{f['path']} ({f['detail']})" for f in fs[:3])
            more = f"; and {len(fs) - 3} more" if len(fs) > 3 else ""
            lines.append(f"{region.name} [{view.name}] after {source}: {check}: {len(fs)} element(s): {shown}{more}")
    return lines, held


@pytest.mark.parametrize("region", REGIONS, ids=lambda r: r.name)
def test_guard(page, case, region):
    """A region off the list keeps, in every view and around every data
    source, what the user holds; a region on the list is shown and
    skipped with its note. Nothing but GETs is sent either way."""
    listed = region.name in NOT_MOVED
    if region.gap:
        reason = f"{region.name}: cannot be shown with the fake Cube's fixture: {region.gap}"
        pytest.skip(reason + (f"; on the list: {NOT_MOVED[region.name]}" if listed else ""))
    checked = not listed or region.name in CHECK
    loaded(page)
    writes = Writes(page)
    failures, held, counts = [], [], []
    for view in region.views:
        view.show(page)
        page.evaluate(GUARD_JS)
        n = page.evaluate("(a) => window.__guard.count(a)", {"roots": list(region.containers), "exclude": region.exclude})
        assert n > 0, f"{region.name} [{view.name}]: the region shows nothing a user can hold ({', '.join(region.containers)})"
        counts.append(f"{view.name}: {n}")
        if checked:
            lines, h = guard_view(page, writes, region, view)
            failures += lines
            held += [f"[{view.name}] {x}" for x in h]
    assert not writes.sent, f"{region.name}: the guard sent requests other than GET: {writes.sent}"
    let = f", let through as the page loaded: {'; '.join(writes.let)}" if writes.let else ""
    if not checked:
        pytest.skip(f"{region.name}: on the list, not moved yet: {NOT_MOVED[region.name]} "
                    f"(shown, elements to hold: {'; '.join(counts)}; requests other than GET: 0{let})")
    print(f"{region.name}: checked, {len(SOURCES)} data sources in {len(region.views)} view(s); requests other than GET: 0{let}")
    assert not failures, f"{region.name}: {len(failures)} failure(s) of the guard:\n" + "\n".join(failures) + \
        "\nwhat was held:\n" + "\n".join(held)


def test_the_list_names_regions(case):
    """Every line of the list is a region of the table, so that a region
    renamed does not leave a line that skips nothing."""
    names = {r.name for r in REGIONS}
    assert not set(NOT_MOVED) - names, f"the list names no region: {sorted(set(NOT_MOVED) - names)}"
    assert len(names) == len(REGIONS), "every region has a name of its own"
