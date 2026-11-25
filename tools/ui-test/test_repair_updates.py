# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A box of repairs follows the data in place (AP UiParts, Block 9): a
device's box under its row, a name typed in its listing and its question
open, stays the elements it was, with what is typed and the focus,
through a state event, a reload of the rooms and a rename of the device,
and so does its Repair…; a Cube's box with its question open stays open
through a rename of the Cube. At a sign-out, and for another user
signed in after an expired session, no box and no question is left.
The repairs' routes are stubbed (RepairStub), a GET with a preview;
nothing is run (test_repair.py has the runs)."""
import json

from conftest import TOKEN, URL
from helpers import OPERATOR_PASSWORD, RepairStub, frames, mark, owner_page, replaced, show_view, sign_out, state_events, wait_until
from test_login import sign_in
from test_unsaved import expire

PANEL = ".cube-panel[data-serial='KEQ0000100']"
# A preview with something to send, and a listing of a device the
# Cube's list lacks, which asks for its name.
PREVIEW = {"plan": "plan-x", "radiograms": 2, "duty_cost_pct": 2.5, "slots": 2, "held_slots": 0, "resets": 0, "wakeups": 1,
           "metadata": False, "notes": [], "budget": {"duty_cycle_pct": 3, "free_memory_slots": 50, "updated_at": "2026-11-15T10:00:00Z"},
           "steps": [{"what": "link", "device": "0a0101", "room": 1, "lines": ["s:AAAA"]}]}
LISTING = {"from": 0, "to": 1, "listed": False, "new_room": False, "name": "Bath radiator thermostat 1", "room_name": "Bath",
           "type": "radiator_thermostat"}


def stub(page):
    """The repairs' routes (RepairStub): every repair's preview PREVIEW, a
    listing's as one of a device the list lacks."""
    previews = {key: dict(PREVIEW, action=key, details={}) for key in ("reset", "master", "relink", "unlink-relink", "config", "place")}
    previews["list"] = dict(PREVIEW, action="list", details=LISTING)
    return RepairStub(page, previews)


def device_question(page):
    """The first tile's device list open, its first device's repairs open
    and its listing's question open; the device's row, as a selector."""
    tile = page.locator("#rooms .room").first
    tile.locator(".tile-info").click()
    device = tile.locator(".devices .device:not(.virtual)").first
    device.wait_for(state="visible")
    row = "#rooms .device[data-rf='%s']" % device.get_attribute("data-rf")
    device.locator(".repair-open").click()
    item = device.locator(".repair-item[data-repair=list]")
    item.locator(".repair-run").wait_for(state="visible")
    item.locator(".repair-run").click()
    item.locator(".confirm-box").wait_for(state="visible")
    return row


def test_a_device_box_keeps_what_is_typed(page, case, reads):
    """A device's repairs open under its row, its listing's question open
    and a name typed in its field, the focus in it: a state event per
    device, a reload of the rooms and a rename of the device keep the
    box, the field, the question and Repair… the elements they were, the
    name typed and the focus where they were."""
    repairs = stub(page)
    try:
        row = device_question(page)
        rf = page.locator(row).get_attribute("data-rf")
        item = page.locator(row + " .repair-item[data-repair=list]")
        field = item.locator("input[name=name]")
        field.fill("Sofa side")
        held = [row + " .repair-box", row + " .repair-item[data-repair=list] input[name=name]",
                row + " .repair-item[data-repair=list] .confirm-no", row + " .repair-open"]
        mark(page, held)
        field.focus()
        state_events(page)
        page.evaluate("loadRooms()")

        def renamed(rooms):
            for room in rooms or []:
                for d in room.get("devices") or []:
                    if d.get("rf_address") == rf:
                        d["name"] = "Renamed by the test"
            return rooms

        reads.change("**/api/v1/cubes/*/rooms?devices=all", renamed)
        page.evaluate("loadRooms()")
        assert wait_until(page, "document.querySelector(%s).textContent === 'Renamed by the test'" % json.dumps(row + " .device-name"), 5), \
            "the device's new name shown"
        frames(page)
        assert replaced(page, held) == [], "the box, its field, its question and Repair… are the ones they were"
        assert field.input_value() == "Sofa side", "the name typed stays"
        assert field.evaluate("(el) => el === document.activeElement"), "the focus stays in the name"
        assert item.locator(".confirm-box").is_visible(), "the question stays open"
        assert not repairs.runs, f"nothing run: {repairs.runs}"
    finally:
        repairs.close()


def test_a_cube_box_stays_open_through_a_rename(page, case, reads):
    """The Cube's repairs open in its panel, the reset's question open
    with the focus on its No: the Cube renamed by a poll keeps the box,
    the question and the focus."""
    repairs = stub(page)
    try:
        show_view(page, "system")
        panel = page.locator("#cubes-list " + PANEL)
        panel.locator(":scope > summary").click()
        panel.locator(".cube-repair .repair-open").click()
        item = panel.locator(".repair-item[data-repair=reset]")
        item.locator(".repair-run").wait_for(state="visible")
        item.locator(".repair-run").click()
        no = item.locator(".confirm-box .confirm-no")
        no.wait_for(state="visible")
        base = "#cubes-list " + PANEL
        held = [base + " .repair-box", base + " .repair-item[data-repair=reset] .confirm-no", base + " .cube-repair .repair-open"]
        mark(page, held)
        no.focus()
        reads.cube("KEQ0000100", name="Renamed Cube")
        page.evaluate("loadCubes()")
        assert wait_until(page, "state.cubes.some((c) => c.name === 'Renamed Cube')", 5), "the Cube renamed"
        frames(page)
        assert replaced(page, held) == [], "the box, its question and Repair… are the ones they were"
        assert no.evaluate("(el) => el === document.activeElement"), "the focus stays on No"
        assert not repairs.runs, f"nothing run: {repairs.runs}"
    finally:
        repairs.close()
    show_view(page, "tiles")


def no_repairs_left(page):
    """Nothing of a box of repairs in the page, the first tile's device
    list open again where the box was: no box, no question (a box kept
    would be drawn there, hidden from a role without repair)."""
    tile = page.locator("#rooms .room").first
    tile.locator(".tile-info").click()
    tile.locator(".devices .device:not(.virtual)").first.wait_for(state="visible")
    frames(page)
    assert page.locator(".repair-box").count() == 0, "no box of repairs"
    assert page.locator(".repair-item .confirm-box").count() == 0, "no question of one"


def test_sign_out_forgets_the_boxes(new_context, case, watch, guard):
    """A device's repairs open with their question: Sign out, and the next
    user signed in finds no box and no question."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    repairs = stub(page)
    try:
        device_question(page)
        sign_out(page)
        sign_in(page, "operator", OPERATOR_PASSWORD)
        no_repairs_left(page)
        assert not repairs.runs, f"nothing run: {repairs.runs}"
    finally:
        repairs.close()


def test_another_user_after_a_401_finds_no_box(new_context, case, watch, guard):
    """A device's repairs open with their question, the session taken as
    expired: another user signed in finds no box and no question."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    repairs = stub(page)
    try:
        device_question(page)
        expire(page)
        sign_in(page, "operator", OPERATOR_PASSWORD)
        no_repairs_left(page)
        assert not repairs.runs, f"nothing run: {repairs.runs}"
    finally:
        repairs.close()
