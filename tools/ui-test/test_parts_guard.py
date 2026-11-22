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
source of SOURCES and of its own (Region.sources: the page shown again,
or the stream's server events of the kinds named, each dispatched on the
page's stream, whose reads the guard waits for until their requests have
answered, or the settings applied again, as when the user's arrive from
the server, applyStoredSettings()), applied with the data as it was (GUARD_JS, round):

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
changes no element. In the other two its tests are deselected as the
suite is collected (the mark cases, conftest.py), so that none loads a
page only to be skipped (AP TestSpeed, D9-A). In CI that is in the jobs
of 1100-en and 390-de, on a worker of its own beside the other files.
"""
import json
import os
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Callable

import pytest

from helpers import LOG_ALERT_A, LOG_ALERT_B, LOG_CMD, LOG_CMD_LINE, STATE_EVENTS_JS, RepairStub, StubLog, log_lines, open_main_menu, reads_of, safe_route, show_view, stub_log_command, wait_until

# The cases the guard runs in (D176-D); in the others its tests are
# deselected at collection (D9-A).
GUARD_CASES = ("1100-en", "390-de")
pytestmark = pytest.mark.cases(*GUARD_CASES)


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
    views, where the fake Cube's fixture cannot show it, why (gap), and
    the data sources of its own beside SOURCES (sources), by their
    names in GUARD_JS's run, and what is undone once its views are done
    (after: a stub's routes)."""

    name: str
    containers: tuple
    views: tuple = ()
    exclude: str = ""
    gap: str = ""
    sources: tuple = ()
    after: Callable | None = None


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


def with_unlisted(devices):
    """GET …/devices as the server answers it, and UNLISTED_STUB."""
    return (devices or []) + [UNLISTED_STUB]


def unlisted_form(page):
    """The devices without a room with UNLISTED_STUB, GET …/devices
    answered with the server's answer and it at every load, and its form
    that puts it into a room open."""
    on_rooms(page)
    reads_of(page).change("**/api/v1/cubes/*/devices?devices=real", with_unlisted)
    page.evaluate("loadRooms()")
    row = page.locator("#unlisted .device[data-rf='0a0f01']")
    row.wait_for(state="visible")
    row.locator(".put-open").click()
    row.locator("form input[name=name]").wait_for(state="visible")
    settle(page)


# A repair's preview, as GET …/repair/… answers one with something to
# send: the fixture's previews mostly have nothing to do, and a listing
# asks for a name only for a device the Cube's list lacks. A run is a
# POST, which Writes refuses.
REPAIR_STUB = {"plan": "plan-guard", "radiograms": 2, "duty_cost_pct": 2.5, "slots": 2, "held_slots": 0, "resets": 0, "wakeups": 1,
               "metadata": False, "notes": [], "budget": {"duty_cycle_pct": 3, "free_memory_slots": 50, "updated_at": "2026-11-15T10:00:00Z"},
               "steps": [{"what": "link", "device": "0a0101", "room": 1, "lines": ["s:AAAA", "s:BBBB"]}]}
LISTING_STUB = {"from": 0, "to": 1, "listed": False, "new_room": False, "name": "Bath radiator thermostat 1", "room_name": "Bath",
                "type": "radiator_thermostat"}


def stub_repairs(page):
    """Answers every GET of a repair's preview with REPAIR_STUB, a
    listing's as of a device the Cube's list lacks, once per page
    (RepairStub); the other methods go on to Writes."""
    if getattr(page, "_guard_repairs", False):
        return
    previews = {key: dict(REPAIR_STUB, action=key, details={}) for key in ("reset", "master", "relink", "unlink-relink", "config", "place")}
    previews["list"] = dict(REPAIR_STUB, action="list", details=LISTING_STUB)
    RepairStub(page, previews, runs=False)
    page._guard_repairs = True


def ask_run(item):
    """Run… of a repair's item clicked: its question open in its place."""
    item.locator(".repair-run").wait_for(state="visible")
    item.locator(".repair-run").click()
    item.locator(".confirm-box").wait_for(state="visible")


def rooms_repair_tile(page):
    """A room's repairs open under its tile from ⋯ (stub_repairs), the
    re-link's question open."""
    stub_repairs(page)
    on_rooms(page)
    room = tile(page)
    if room.locator(".repair-box:not(.hidden)").count() == 0:
        room.locator(".tile-head > details.menu > summary").click()
        room.locator(".menu-repair").click()
    ask_run(room.locator(".repair-box .repair-item[data-repair=relink]"))
    settle(page)


def rooms_repair_device(page):
    """A device's repairs open under its row in its tile's device list
    (stub_repairs), its listing's name there for the guard to type and its
    question open."""
    stub_repairs(page)
    on_rooms(page)
    room = tile(page)
    if not room.locator(".devices .device").first.is_visible():
        room.locator(".tile-info").click()
    device = room.locator(".devices .device:not(.virtual)").first
    device.wait_for(state="visible")
    if device.locator(".repair-box:not(.hidden)").count() == 0:
        device.locator(".repair-open").click()
    item = device.locator(".repair-item[data-repair=list]")
    item.locator("input[name=name]").wait_for(state="visible")
    ask_run(item)
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
    """Every Cube's problem list open in the header's pile, with a
    device, the settings and the Cubes found in it (stub_cubes): the fault
    of AP UiParts Block 6."""
    stub_cubes(page)
    on_rooms(page)
    page.locator("#cube-problems .problems-sum").click()
    page.locator("#cube-problems .problems-list").wait_for(state="visible")


def pile_refreshed(page):
    """The pile's list open after its Refresh now, which polls and leaves
    it open (AP GlobalMenu, Block 12), the poll come back."""
    header_problems(page)
    page.locator("#cube-problems .refresh-now").click()
    assert wait_until(page, "!document.querySelector('#cube-problems .refresh-now').disabled", 5), "the poll came back"
    page.locator("#cube-problems .problems-list").wait_for(state="visible")


def main_menu(page):
    on_rooms(page)
    open_main_menu(page)


def main_menu_item(page):
    """☰ open, the focus moved onto System settings' entry."""
    main_menu(page)
    page.locator("#menu-system").focus()


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
    reads = reads_of(page)
    reads.cube("KEQ0000100", **CUBES_STUB)
    for found in FOUND_STUB:
        reads.found(found)
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


def cubes_repair(page):
    """A Cube's panel open with its repairs open (stub_repairs), the
    reset's question open."""
    stub_repairs(page)
    cubes_open(page)
    panel = page.locator("#cubes-list .cube-panel").first
    if panel.locator(".repair-box:not(.hidden)").count() == 0:
        panel.locator(".cube-repair .repair-open").click()
    ask_run(panel.locator(".repair-box .repair-item[data-repair=reset]"))
    settle(page)


def cube_page_repair(page):
    """Cube settings with the Cube's repairs open (stub_repairs), the
    reset's question open."""
    stub_repairs(page)
    cube_page(page)
    panel = page.locator("#cube-page-body .cube-panel")
    if panel.locator(".repair-box:not(.hidden)").count() == 0:
        panel.locator(".cube-repair .repair-open").click()
    ask_run(panel.locator(".repair-box .repair-item[data-repair=reset]"))
    settle(page)


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


# A device that joins while the Cube pairs, as GET …/pairing gives one:
# pairing starts only by a POST, a write to the fake Cube.
JOINED_STUB = {"type": "radiator_thermostat", "rf_address": "0a0f02", "serial": "MEQ0000902"}


def pairing_running(page):
    """The chosen Cube pairing on Cube settings, GET …/pairing answered as
    running for ten minutes more with JOINED_STUB at every load, its form
    there with a room chosen; the name the guard types."""
    until = (datetime.now(timezone.utc) + timedelta(minutes=10)).isoformat()
    reads_of(page).change("**/api/v1/cubes/*/pairing", lambda st: dict(st or {}, active=True, until=until, paired=[JOINED_STUB]))
    cube_page(page)
    page.evaluate("pairingEvent(state.serial)")
    form = page.locator("#cube-page-body .cube-pairing .pair-device[data-rf='0a0f02'] form")
    form.wait_for(state="visible")
    assert wait_until(page, "document.querySelectorAll(\"#cube-page-body .pair-device select[name=room] option\").length > 1", 5), "the rooms offered"
    form.locator("select[name=room]").select_option(index=1)
    settle(page)


def users(page):
    system(page)
    panel = page.locator(".user-panel[data-name='operator']")
    if not panel.evaluate("(d) => d.open"):  # open already in a view before
        panel.locator("summary").click()
    assert wait_until(page, "document.querySelectorAll(\".user-panel[data-name='operator'] .user-tokens li\").length >= 1", 5), \
        "the operator's tokens are listed"
    if page.locator("#user-add").is_visible():  # shown already in a view before
        page.locator("#user-add").click()
    page.locator("#user-new input").first.wait_for(state="visible")


def roles(page):
    system(page)
    page.locator(".role-panel[data-key='operator'] summary").click()
    page.locator("#role-add").click()
    page.locator("#role-new input").first.wait_for(state="visible")


def users_question(page):
    """A user's Remove asking in its panel, the operator's."""
    users(page)
    page.locator(".user-panel[data-name='operator'] .remove-user").click()
    page.locator(".user-panel[data-name='operator'] .confirm-box").wait_for()


def providers(page):
    system(page)
    if page.locator("#provider-add").is_visible():  # shown already in a view before
        page.locator("#provider-add").click()
    page.locator("#provider-new input").first.wait_for(state="visible")


# The fixture has no sign-in provider and no integration: the views of
# their panels answer GET /api/v1/providers and the integrations' reads
# with one constant answer each, so that the data stay as they were.
PROVIDER_STUB = {"key": "guard", "name": "Guard", "issuer": "https://issuer.example.org", "client_id": "guard", "has_secret": True,
                 "secret_lost": False, "scopes": "openid email profile", "auto_create": True, "auto_role": "viewer", "auto_domains": ["example.org"],
                 "groups_claim": "", "allowed_groups": [], "enabled": False, "callback": "api/v1/login/oidc/callback",
                 "created_at": "2026-11-15T10:00:00Z", "updated_at": "2026-11-15T10:00:00Z"}
GUARD_IG = "01a1e000-0000-7000-8000-00000000a001"
GUARD_REQ = "01a1e000-0000-7000-8000-00000000a002"
IG_STUB = {"uuid": GUARD_IG, "kind": "homeassistant", "name": "Guard home", "reported_name": "Guard home", "state": "active",
           "instance_id": "a1" * 16, "url": "http://192.0.2.30:8123", "version": "2026.11.1", "max_caps": ["read", "setpoint"],
           "key_issued_at": "2026-11-15T10:00:00Z", "created_at": "2026-11-15T10:00:00Z", "seen": True}
REQ_STUB = {"uuid": GUARD_REQ, "kind": "homeassistant", "name": "Guard request", "reported_name": "Guard request", "state": "pending",
            "instance_id": "a2" * 16, "request_code": "GRD-123", "request_remote": "192.0.2.31", "created_at": "2026-11-15T10:00:00Z",
            "request_expires": "2099-01-01T00:00:00Z"}
IG_USER = {"id": "b" * 32, "name": "Guard user", "display": "Guard home › Guard user", "user": GUARD_IG + ":" + "b" * 32,
           "own_user": GUARD_IG + ":" + "b" * 32, "linked": False, "last_level": "control", "last_used_at": "2026-11-15T10:00:00Z"}
FOUND_IG = {"found": [{"kind": "homeassistant", "name": "Guard LAN", "instance_id": "a3" * 16, "address": "192.0.2.32"}], "looks": True}


def provider_panel(page):
    """A provider's panel open, its form with every field, the new provider's form beside it."""
    reads_of(page).change("**/api/v1/providers", lambda body: [PROVIDER_STUB])
    show_view(page, "tiles")
    system(page)
    page.locator(".provider-panel[data-key='guard'] > summary").click()
    page.locator(".provider-panel[data-key='guard'] input[name=name]").wait_for(state="visible")
    if page.locator("#provider-add").is_visible():
        page.locator("#provider-add").click()
    page.locator("#provider-new input").first.wait_for(state="visible")


def integrations(page):
    """A request waiting, an integration's panel open with a user and its
    choice of whom it acts as, and an installation seen on the LAN with
    Register's address asked for."""
    reads_of(page).change("**/api/v1/integrations", lambda body: [REQ_STUB, IG_STUB])
    safe_route(page, f"**/api/v1/integrations/{GUARD_IG}", lambda route: route.fulfill(
        status=200, content_type="application/json", body=json.dumps(dict(IG_STUB, users=[IG_USER]))))
    reads_of(page).change("**/api/v1/integrations/found*", lambda body: FOUND_IG)
    show_view(page, "tiles")
    system(page)
    panel = page.locator(f".integration-panel[data-uuid='{GUARD_IG}']")
    panel.locator("> summary").click()
    panel.locator(".integration-user select").wait_for(state="visible")
    page.locator("#integrations-found .register-integration").click()
    page.locator("#integrations-found .register-address input").wait_for(state="visible")


def backups_system(page):
    system(page)
    page.locator("#backups-cubes .backup-fold[open] .backup-take").wait_for(state="visible")


def backups_cube(page):
    cube_page(page)
    page.locator("#cube-backup-body .backup-take").wait_for(state="visible")


def backups_question(page):
    """Cube settings, a kept backup's Delete asking in its row (run.sh took one by hand)."""
    backups_cube(page)
    row = page.locator("#cube-backup-body .backup-row").first
    row.wait_for(timeout=10000)
    row.locator(".backup-actions button").last.click()
    row.locator(".confirm-box").wait_for()


def log(page):
    """The log, the search empty again: a view's round types into it."""
    show_view(page, "log")
    page.locator("#log-filter").fill("")
    page.locator("#log-lines .log-line").first.wait_for()
    settle(page)


def stub_log(page):
    """The log's reads of the Cube answered with the log's lines of
    helpers.py (StubLog), a command's line among them, its detail, and two
    of them problems not yet seen, once per page, each route taken away
    once the region's views are done (Region.after); the other methods go
    on to Writes."""
    if getattr(page, "_guard_log", False):
        return
    stub = StubLog(page, log_lines())
    command = stub_log_command(page)
    alerts = [{"id": LOG_ALERT_A, "kind": "hello", "level": "warn", "time": "2026-11-20T08:00:17Z"},
              {"id": LOG_ALERT_B, "kind": "hello", "level": "error", "time": "2026-11-20T08:00:16Z"}]
    problems = safe_route(page, "**/api/v1/log/alerts*", lambda r: r.fulfill(status=200, content_type="application/json",
                                                                               body=json.dumps({"level": "error", "alerts": alerts})))

    def off():
        stub.off()
        page.unroute(f"**/api/v1/commands/{LOG_CMD}", command)
        page.unroute("**/api/v1/log/alerts*", problems)
        page._guard_log = False

    page._guard_log_off = off
    page._guard_log = True


def unstub_log(page):
    """stub_log's routes taken away, once the region's views are done."""
    off = getattr(page, "_guard_log_off", None)
    if off and not page.is_closed():
        off()
        page.evaluate("loadLogAlerts()")  # ☰'s mark as the server has it again


def log_stubbed(page):
    """The log with the stubbed lines (stub_log), its problems read."""
    stub_log(page)
    show_view(page, "log")
    page.evaluate("loadLog().then(() => loadLogAlerts())")
    page.locator(f"#log-lines .log-line[data-id='{LOG_ALERT_A}'] .log-mark").wait_for()
    settle(page)


def log_command(page):
    """The log with a command's detail open (stub_log)."""
    log_stubbed(page)
    btn = page.locator(f"#log-lines .log-line[data-id='{LOG_CMD_LINE}'] .log-cmd")
    if btn.get_attribute("aria-expanded") != "true":
        btn.click()
    page.locator(f"#log-lines .log-line[data-id='{LOG_CMD_LINE}'] .log-cmd-head").wait_for()
    settle(page)


def maint(page):
    if not page.locator("#maint-page").is_visible():
        show_view(page, "maint")
    settle(page, 500)


# The test server runs in a terminal and offers no update, no restart and
# no reboot: the views of Software update and Maintenance answer their
# reads with one constant answer each, so that the data stay as they
# were; the other methods go on to Writes, which refuses them.
UPDATE_STUB = {"current": "v0.0.0-test", "flavour": "alpha", "channel": "alpha", "latest": "v0.0.1-test", "available": True, "can_update": True,
               "checked_at": "2026-11-15T10:00:00Z", "helper": {"state": "up-to-date", "time": "2026-11-15T10:00:00Z"},
               "history": [{"at": "2026-11-14T18:00:00Z", "by": {"name": "anna"}, "from": "v0.0.0", "to": "v0.0.1-test"}]}
RESTART_STUB = {"available": True, "by": "systemd", "pending": ["MASSIMO_LISTEN"], "file": "/etc/massimo/env", "read_at": "2026-11-15T10:00:00Z"}
REBOOT_STUB = {"available": True, "by": "systemd"}


def constant(page, pattern, body):
    """Every GET of pattern answered with body; the other methods go on to Writes."""
    page.route(pattern, lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(body))
               if route.request.method == "GET" else route.fallback())


def software_question(page):
    """Maintenance with an update offered and Update asking in the section; the channel's choice above it."""
    constant(page, "**/api/v1/update*", UPDATE_STUB)
    show_view(page, "maint")
    page.locator("#update-btn").click()
    page.locator(".update .confirm-box").wait_for()
    page.locator("#update-channel").wait_for(state="visible")


def maint_question(page):
    """Maintenance with the restart and the reboot offered, the restart
    asking in its row; the guard types a delay into the reboot's."""
    constant(page, "**/api/v1/restart", RESTART_STUB)
    constant(page, "**/api/v1/reboot", REBOOT_STUB)
    show_view(page, "maint")
    page.locator("#restart-btn").click()
    page.locator(".restart .confirm-box").wait_for()
    page.locator("#reboot-delay").wait_for(state="visible")


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


def personal_sections(page):
    """The personal page's sections menu open, the focus on My settings'
    item."""
    personal("me")(page)
    page.keyboard.press("Escape")
    page.locator("#me .sec-menu > summary").click()
    page.locator("#me .sec-menu .toc-link[data-target='prefs']").focus()


def first_owner(page):
    """The first owner's form in the sign-in card (AP FirstOwner), GET
    /api/v1/me stubbed as an open server without users for a caller on
    the local network for the rest of the test, which opens the card on
    the running page (the region's own source, loadMe(), reads the stub
    again and hands out the same opening); the focus moved onto Again by
    the view."""
    body = json.dumps({"mode": "open", "user": None, "role": None, "capabilities": [], "setup": True})
    safe_route(page, "**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=body))
    on_rooms(page)
    page.evaluate("() => { loadMe(); }")
    page.locator("#login-dialog[open] #setup-form").wait_for(state="visible")
    settle(page)


def keep_open_step(page):
    """The confirmation that keeps the server open (AP FirstOwner, Block
    3), shown from the first owner's form in the card, an address typed."""
    if not page.locator("#login-dialog[open] #setup").is_visible():
        first_owner(page)
    page.locator("#setup-keep-open").click()
    page.locator("#keep-open-address").fill("http://example.invalid")
    settle(page)


def sign_in_card(page):
    """The sign-in's card on the signed-in page, opened as a 401 opens it
    (askLogin), the password's way chosen."""
    page.evaluate("askLogin(); null")
    page.locator("#login-dialog[open] #login-user").wait_for(state="visible")
    settle(page)


def sign_in_key(page):
    """The card's API key's way chosen."""
    page.locator("#login-ways .login-way[data-way='key']").click()
    page.locator("#login-token").wait_for(state="visible")


def sign_in_menu(page):
    """The card's ☰ open, back on the password's way."""
    page.locator("#login-ways .login-way[data-way='password']").click()
    page.locator("#login-menu > summary").click()
    page.locator("#login-lang").wait_for(state="visible")


def users_fold(folded):
    """System settings, the Users card folded (or opened again) by its
    button, the focus on it; the fold kept in the browser is taken back by
    the view that opens it again, the next of the region's."""
    def show(page):
        system(page)
        page.keyboard.press("Escape")
        fold = page.locator("#sys-users .section-fold")
        if (fold.get_attribute("aria-expanded") == "true") == folded:
            fold.click()
        fold.focus()
    return show


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
    Region("pile", (".pile",), (
        View("every Cube's problem list open", header_problems),
        View("the pile's list open after Refresh now", pile_refreshed),
    )),
    # ☰ and the page buttons are parts (AP UiParts, Block 14), drawn from
    # who is in as well: who is in read again (loadMe) is a source of
    # theirs; both views passed on the page before the move, where nothing
    # drew ☰ again.
    Region("main-menu", ("header.top nav.nav",), (
        View("☰ open", main_menu),
        View("☰ open, the focus on an entry", main_menu_item, "#menu-system"),
    ), sources=("loadMe()",)),
    Region("home-notice", ("#home-notice",), (View("on the second server, its Cube not connected", home_notice),)),
    Region("rooms", ("#rooms",), (
        View("a tile's ⋯ menu open", rooms_menu),
        View("a tile's More menu open", rooms_more),
        View("a tile's problems list open", rooms_problems),
        View("a tile's holiday form, another's device list", rooms_holiday_and_devices),
        View("the virtual thermostat's settings, expert mode", rooms_virtual),
        # Passed on the page before the move as well: repair.js drew a box
        # again at its own reads alone, and the islands around it at a
        # rename, none of SOURCES. From now on it guards that a box, its
        # question open, stays through the data; the renames and the
        # reads again are test_repair_updates.py's and test_repair.py's.
        View("a room's repairs open, the re-link's question open", rooms_repair_tile),
        # Passed on the page before the move as well: repair.js drew a box
        # again at its own reads alone, and the islands around it at a
        # rename, none of SOURCES. From now on it guards that a box, its
        # question open and its listing's name typed, stays through the data; the renames and the
        # reads again are test_repair_updates.py's and test_repair.py's.
        View("a device's repairs open, its listing's name and question", rooms_repair_device),
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
        # Passed on the page before the move as well: repair.js drew a box
        # again at its own reads alone, and the islands around it at a
        # rename, none of SOURCES. From now on it guards that a box, its
        # question open, stays through the data; the renames and the
        # reads again are test_repair_updates.py's and test_repair.py's.
        View("a Cube's repairs open, the reset's question open", cubes_repair),
    ), exclude=".cube-pairing"),
    Region("cube-settings", ("#cube-page-body", "#cube-page-msg"), (
        View("the chosen Cube's panel", cube_page),
        View("its problem list open", cube_page_problems),
        # Passed on the page before the move as well: repair.js drew a box
        # again at its own reads alone, and the islands around it at a
        # rename, none of SOURCES. From now on it guards that a box, its
        # question open, stays through the data; the renames and the
        # reads again are test_repair_updates.py's and test_repair.py's.
        View("the Cube's repairs open, the reset's question open", cube_page_repair),
    ), exclude=".cube-pairing"),
    Region("pairing", ("#cube-page-body .cube-pairing",), (
        View("the pairing form on Cube settings", pairing_form),
        # Passed on the page before the move as well: the plain pairing was
        # drawn again by its own reads and events, none of SOURCES. From now
        # on it guards that a running pairing, its countdown moving on in its
        # line, keeps a joined device's form through the data; the pairing's
        # own events are test_pairing_updates.py's.
        View("a pairing running, a device joined with its room chosen", pairing_running),
    )),
    Region("users", ("#sys-users-body",), (
        View("the operator's panel and the new user's form", users),
        # Passed on the page before the move as well: none of SOURCES draws
        # the users again. From now on it guards that the question stays through the data;
        # the list's own reads (an action in another panel, the list read
        # again, the page shown again) are test_admin_updates.py's.
        View("the operator's Remove asking", users_question),
    )),
    Region("roles", ("#sys-roles-body",), (View("the operator role's panel and the new role's form", roles),)),
    Region("capabilities", ("#sys-caps-body",), (View("System settings", system),)),
    Region("providers", ("#sys-providers-body",), (
        View("the new provider's form", providers),
        # Passed on the page before the move as well: none of SOURCES draws
        # the providers again. From now on it guards that a provider's panel with its form typed stays through the data;
        # the list's own reads (an action in another panel, the list read
        # again, the page shown again) are test_admin_updates.py's.
        View("a provider's panel open and the new provider's form", provider_panel),
    )),
    Region("integrations", ("#sys-integrations-body",), (
        View("System settings", system),
        # Passed on the page before the move as well: none of SOURCES draws
        # the integrations again. From now on it guards that a request, an integration's panel and Register's address stays through the data;
        # the list's own reads (an action in another panel, the list read
        # again, the page shown again) are test_admin_updates.py's.
        View("a request, an integration's panel with a user, Register's address", integrations),
    )),
    Region("backups", ("#sys-backup-body", "#cube-backup-body"), (
        View("System settings, the Cube's fold open", backups_system),
        View("Cube settings", backups_cube),
        # Passed on the page before the move as well: none of SOURCES draws
        # a Cube's backups again. From now on it guards that the question stays through the data;
        # the list's own reads (an action in another panel, the list read
        # again, the page shown again) are test_admin_updates.py's.
        View("Cube settings, a backup's Delete asking", backups_question),
    )),
    # The log is a part (AP UiParts, Block 11). Its head passed on the
    # plain page too, which never built it again. Its lines are a region
    # of their own here, so that the round's typing into the search, which
    # filters them all out, leaves them: their views fail on the plain
    # page, where loadRooms and loadLogAlerts built every line again.
    Region("log", ("#logview",), (
        View("the log", log),
        View("the kind's choice in focus", log, "#log-kind"),
    )),
    Region("log-lines", ("#log-lines",), (
        View("a problem's Mark seen in focus", log_stubbed, f"#log-lines .log-line[data-id='{LOG_ALERT_A}'] .log-mark"),
        View("a command's detail open, the focus on its button", log_command, f"#log-lines .log-line[data-id='{LOG_CMD_LINE}'] .log-cmd"),
    ), after=unstub_log),
    # Software update's and Maintenance's views pass on the page before
    # the move too (AP UiParts, Block 12): update.js and maintenance.js
    # filled their elements in place but for the options of a select and
    # the last updates' lines, which nobody holds; what they lost is in
    # test_maint_updates.py (a line selected, a select's focus while it
    # saved, a question about a release gone). System info's fails there:
    # system-info.js made the copy icons and the link anew at each read.
    # The stream's events (UiParts D198) are a source of the regions that
    # follow the server while shown, a forward guard: the plain page read
    # nothing for them, so that their views passed there too;
    # test_maint_updates.py shows the reads they now make, one per burst,
    # for the page shown alone. An update event starts the reading of the
    # helper (C3), a flow, and is left out here.
    Region("software-update", ("#sys-software-body",), (
        View("Maintenance", maint),
        View("Update asking, the channel's choice above it", software_question, "#update-channel"),
    ), sources=("loadUpdate(false)", "the page shown again: maint", "the stream's events: resync change")),
    Region("maintenance", ("#sys-maint-body",), (
        View("Maintenance", maint),
        View("the restart asking, a delay typed for the reboot", maint_question),
    ), sources=("the page shown again: maint", "the stream's events: resync change reboot reboot_cancelled settings_read")),
    Region("system-info", ("#sys-info-body",), (View("System info, its folds open", info),), sources=("the page shown again: info", "the stream's events: resync settings_read change.cube")),
    # the settings applied again, as when the user's arrive from the server at a sign-in (loadPreferences)
    Region("my-settings", ("#prefs-rows", "#prefs-sec > p"), (View("My settings", personal("prefs")),), sources=("applyStoredSettings()",)),
    Region("appearance", ("#prefs-sec .appearance",), (
        View("My settings", personal("prefs")),
        View("the scheme editor open", scheme_editor),
    ), sources=("applyStoredSettings()",)),
    Region("notifications", ("#notify-sec",), (View("Notifications", personal("notify")),), sources=("the page shown again: me",)),
    Region("my-profile", ("#profile-sec",), (View("My profile", profile),), sources=("the page shown again: me",)),
    # The pages' rows are parts (AP UiParts, Block 14), drawn from who is
    # in as well, and the cards' folds are the skeleton's, their badges and
    # pills parts: a card folded, the focus on its fold, passed on the page
    # before the move as well, where nothing drew a head again; the personal
    # page's sections menu failed there around who is in read again, which
    # emptied it (pageBar).
    Region("page-chrome", ("#system .page-bar", "#system .section-head", "#me .page-bar"), (
        View("System settings, its sections menu open", sections_menu),
        View("the Users card folded, the focus on its fold", users_fold(True), "#sys-users .section-fold"),
        View("the Users card opened again, the focus on its fold", users_fold(False), "#sys-users .section-fold"),
        View("the personal page's sections menu open, the focus on an item", personal_sections, "#me .sec-menu .toc-link[data-target='prefs']"),
    ), sources=("loadMe()",)),
    # The first owner's form is a part (AP UiParts, Block 14), in the
    # sign-in card since AP FirstOwner, opened from who is in: who is in
    # read again (loadMe, the same opening), the page's language drawn
    # again and the app's status read again are its sources; the view
    # passed on the page before the move as well, where nothing drew the
    # plain form again.
    Region("first-owner", ("#setup",), (View("the first owner's form in the card, the focus in Again", first_owner, "#setup-again"),
                                        View("the confirmation that keeps the server open, an address typed", keep_open_step, "#keep-open-address")),
           sources=("loadMe()", "applyLanguage()", "MassimoParts.signIn.readApp()")),
    # The sign-in's card is a part (AP UiParts, Block 14), shown here as a
    # 401 opens it on a page signed in: its sources are a second askLogin
    # (another request's 401, which hands out the same opening), the
    # providers' list read again, the page's language drawn again and who
    # is in read again. The plain card passed these views as well: nothing
    # drew it again but at an opening.
    Region("sign-in", ("#login-dialog",), (
        View("the sign-in's card, a name and a password typed", sign_in_card),
        View("its API key's way chosen, a key typed", sign_in_key, "#login-token"),
        View("its ☰ open", sign_in_menu, "#login-lang"),
    ), sources=("askLogin()", "renderLoginProviders()", "applyLanguage()", "loadMe()")),
)

# The regions not yet moved to parts that the guard fails today, each
# with what it loses; the block that moves a region takes its line off
# (AP UiParts, "How a region moves"). A region that is not here is
# checked. The others passed the guard on the day it was written
# (2026-11-15), although none has moved: they are filled in place, or
# drawn again only by the user's own action (Create key, a backup, a
# command's detail), which the guard does not apply. The block that
# moves one of them keeps it passing. The sign-in left it last (Block
# 14), shown on a page signed in as a 401 shows it: the list is empty,
# and Block 15 removes it; the devices without a room, which the fixture
# cannot show, are given one by their view's stub.
NOT_MOVED = {}


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
  const stateEvents = """ + STATE_EVENTS_JS + r""";
  // the page's requests in flight, so that a source that only starts
  // its reads (the page shown again) is waited for until they answered
  let inflight = 0;
  const realFetch = window.fetch;
  window.fetch = function (...args) {
    inflight++;
    return realFetch.apply(this, args).finally(() => { inflight--; });
  };
  // settled says whether they did within 5 s; a round that did not fails with that as its reason
  async function settled() {
    for (let quiet = 0, n = 0; n < 100; n++) {
      await new Promise((done) => setTimeout(done, 50));
      quiet = inflight ? 0 : quiet + 1;
      if (quiet >= 2) return true;
    }
    return false;
  }
  async function run(source) {
    if (source.startsWith("the page shown again: ")) {
      // what showing the page again does: the viewshown event its loaders take (pages.js showView)
      document.dispatchEvent(new CustomEvent("viewshown", { detail: source.slice("the page shown again: ".length) }));
      return settled();
    }
    if (source.startsWith("the stream's events: ")) {
      // what the server sends when what the region shows changed; a change sets every setting the regions show
      const set = { "update.channel": null, announce: null, poll_interval: null, refresh_interval: null, discover_interval: null, duty_wait: null };
      // change.cube is a change of a Cube, whose proxy System info lists
      const summary = { change: { action: "settings.update", set }, "change.cube": { action: "cube.update", target: "cube:KEQ0000100" },
        settings_read: { why: "SIGHUP", changed: [], at_next_start: [] } };
      for (const name of source.slice("the stream's events: ".length).split(" ")) {
        const kind = name.split(".")[0];
        state.events.dispatchEvent(new MessageEvent(kind, { data: JSON.stringify({ kind, source: "system", time: new Date().toISOString(),
          summary: summary[name] || {} }) }));
      }
      return settled();
    }
    switch (source) {
      case "loadUpdate(false)": return loadUpdate(false);
      case "poll(false)": return poll(false);
      case "loadCubes()": return loadCubes();
      case "loadRooms()": return loadRooms();
      case "refresh()": return refresh();
      case "a state event per device": return stateEvents();
      case "loadLogAlerts()": return loadLogAlerts();
      case "refreshAges()": return refreshAges();
      case "pollLog()": return pollLog();
      case "applyStoredSettings()": return applyStoredSettings();
      case "loadMe()": return loadMe();
      case "askLogin()": askLogin(); return settled(); // the opening it hands out is the one open
      case "renderLoginProviders()": return renderLoginProviders();
      case "applyLanguage()": applyLanguage(); return settled();
      case "MassimoParts.signIn.readApp()": return MassimoParts.signIn.readApp().then(settled); // the app's status read again, as at an opening
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
      const ran = await run(source);
      await frames();
      if (ran === false) out.push({ check: "settled", path: "the page's requests", detail: "still under way after 5 s: the round would check a page that still reads" });
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
    for source in SOURCES + region.sources:
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
    try:
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
    finally:
        if region.after:
            region.after(page)
    assert not writes.sent, f"{region.name}: the guard sent requests other than GET: {writes.sent}"
    let = f", let through as the page loaded: {'; '.join(writes.let)}" if writes.let else ""
    if not checked:
        pytest.skip(f"{region.name}: on the list, not moved yet: {NOT_MOVED[region.name]} "
                    f"(shown, elements to hold: {'; '.join(counts)}; requests other than GET: 0{let})")
    print(f"{region.name}: checked, {len(SOURCES) + len(region.sources)} data sources in {len(region.views)} view(s); requests other than GET: 0{let}")
    assert not failures, f"{region.name}: {len(failures)} failure(s) of the guard:\n" + "\n".join(failures) + \
        "\nwhat was held:\n" + "\n".join(held)


def test_the_list_names_regions(case):
    """Every line of the list is a region of the table, so that a region
    renamed does not leave a line that skips nothing."""
    names = {r.name for r in REGIONS}
    assert not set(NOT_MOVED) - names, f"the list names no region: {sorted(set(NOT_MOVED) - names)}"
    assert len(names) == len(REGIONS), "every region has a name of its own"
