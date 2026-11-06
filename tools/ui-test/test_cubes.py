# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Settings → Cubes: a new name is saved and shown in the header, the
settings file's differing name is offered and taken back; hiding the
devices of a room makes its tile go, showing them again brings it back;
a Cube found on the LAN is added and removed again, and is then listed
as found. Each test puts the server's store back as it was, since every
case uses the same server; nothing is sent to the fake Cube."""
import json

import pytest

from helpers import named_buttons, scroll_width, wait_until

PANEL = ".cube-panel[data-serial='KEQ0537249']"


def open_panel(page):
    """Settings → Cubes with the fake Cube's panel open; its form."""
    page.locator("#view-settings").click()
    panel = page.locator(PANEL)
    panel.locator("summary").click()
    return panel, panel.locator("form.cube-form")


def test_name_saved_and_taken_back(page, case):
    page.locator("#view-settings").click()
    panel = page.locator(PANEL)
    assert panel.count() == 1, "the Cube in Settings → Cubes"
    panel.locator("summary").click()
    form = panel.locator("form.cube-form")
    assert form.is_visible(), "the Cube's panel opens"
    named_buttons(page, "a Cube's panel")
    form.locator("input[name=name]").fill("Renamed")
    form.locator("button[type=submit]").click()
    assert wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0537249' && c.name === 'Renamed')", 5), "the new name is saved"
    assert "Renamed" in page.locator("#cube-select option[value='KEQ0537249']").inner_text(), "the header shows the new name"
    take = panel.locator(".take-file[data-field=name]")
    assert take.count() == 1 and "MASSIMO_CUBE_KEQ0537249_NAME" in panel.locator(".cube-differs").inner_text(), \
        "the settings file's name is offered"
    take.click()
    assert wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0537249' && c.name === 'Fake Cube')", 5), "the file's value taken"
    assert panel.locator(".cube-differs").count() == 0, "no difference left"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the Cube's panel"


def test_hidden_devices(page, case):
    """The last room's devices hidden take its tile away, shown again
    bring it back."""
    panel, form = open_panel(page)
    room = page.evaluate("(() => { const r = state.rooms[state.rooms.length - 1];"
                         " return { name: r.name, rfs: r.devices.map((d) => d.rf_address) }; })()")
    for rf in room["rfs"]:
        form.locator("input[name=hide]").fill(rf)
        form.locator(".cube-hide-add button").click()
    assert form.locator(".cube-hidden-list li").count() == len(room["rfs"]), "devices listed to hide"
    form.locator("button[type=submit]").click()
    gone = "!state.rooms.some((r) => r.name === %s)" % json.dumps(room["name"])
    assert wait_until(page, gone, 5), "hiding the devices removes their room"
    page.locator("#view-tiles").click()
    assert page.evaluate("![...document.querySelectorAll('.room .room-name')].some((e) => e.textContent.trim() === %s)" % json.dumps(room["name"])), \
        "the room's tile is gone"
    page.locator("#view-settings").click()
    for _ in room["rfs"]:
        form.locator(".cube-hidden-list button").first.click()
    form.locator("button[type=submit]").click()
    assert wait_until(page, "!(" + gone + ")", 5), "showing them again brings the room back"
    assert wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0537249' && c.hidden.length === 0)", 5), "nothing hidden any more"
    assert scroll_width(page) <= case.width, "no horizontal scroll in Settings → Cubes"
    page.locator("#view-tiles").click()


def test_found_cube_added_and_removed(page):
    """The Cube the fake Cube announces, its DHCP flag shown, is added and
    served, and Remove,
    asking in the panel, lists it as found again. The server keeps what a
    search found; where nothing was searched yet, this test searches."""
    if not page.evaluate("state.found.some((c) => c.serial === 'KEQ0000000')"):
        page.locator("#cube-select").select_option("__discover")
        wait_until(page, "!discovery.running", 10)
    page.locator("#view-settings").click()
    found = page.locator(".cube-found-row[data-serial='KEQ0000000']")
    if found.count() == 0:
        pytest.skip("no found Cube to add (discovery did not answer)")
    assert " · DHCP · " in found.locator(".found-facts").inner_text(), "the found Cube's DHCP flag in Settings → Cubes"
    found.locator("button").click()
    assert wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0000000')", 5), "Add serves the found Cube"
    assert page.evaluate("state.serial") == "KEQ0537249", "the selected Cube stays"
    other = page.locator(".cube-panel[data-serial='KEQ0000000']")
    other.locator("summary").click()
    other.locator(".remove-cube").click()
    confirm = other.locator(".confirm-box")
    assert confirm.is_visible() and other.locator(".remove-cube").count() == 0, "Remove asks in the panel"
    confirm.locator(".confirm-no").click()
    assert other.locator(".confirm-box").count() == 0 and other.locator(".remove-cube").is_visible(), "Keep takes the question back"
    other.locator(".remove-cube").click()
    other.locator(".confirm-box .confirm-yes").click()
    assert wait_until(page, "!state.cubes.some((c) => c.serial === 'KEQ0000000') && state.found.some((c) => c.serial === 'KEQ0000000')", 5), \
        "Remove lists it as found again"
    page.locator("#view-tiles").click()
