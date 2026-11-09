# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The week programme editor, <massimo-week-program> (week-program.js;
AP HomeAssistant v1.0, Block 3): the days edited together, "only this
day", the copy to other days, the resets, the 13 switch points and the
day's bar. Nothing is sent: each test leaves the editor with Cancel, and
the fake Cube's programme stays as the other tests expect it."""
from test_send import first_tile, open_tab
from helpers import wait_until

DAYS = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]


def day_button(tile, day):
    """The day's button in the row of days to edit."""
    return tile.locator(".prog-days button[data-day=%s]" % day)


def marked(tile, cls):
    """The days whose button in the row of days to edit has the class."""
    return [d for d in DAYS if cls in (day_button(tile, d).get_attribute("class") or "").split()]


def bump_first_temp(tile):
    """Changes the first switch point's temperature by half a degree."""
    temp = tile.locator(".prog-point input[type=number]").first
    was = float(temp.input_value())
    temp.fill("%g" % (was + 0.5 if was < 30 else was - 0.5))
    temp.press("Tab")


def only_on(tile):
    """Turns the "Only {day}" switch on, by its label, as a user does."""
    tile.locator("massimo-week-program label.switch").click()
    assert tile.locator("massimo-week-program label.switch input[type=checkbox]").is_checked(), "only is on"


def leave(tile):
    """Leaves the editor without sending."""
    tile.locator(".config-tabs .close").click()


def test_week_program_is_the_element(page, case):
    """The programme tab is the element, its parts in the form."""
    tile, _ = first_tile(page)
    open_tab(tile, "program")
    editor = tile.locator(".config-body form massimo-week-program")
    assert editor.count() == 1, "the tab holds one <massimo-week-program>"
    for part in (".prog-days", ".prog-bar", ".prog-points", ".day-copy", ".prog-note"):
        assert editor.locator(part).count() == 1, f"the editor draws {part}"
    assert tile.locator(".config-body form .actions .prog-reset-day").count() == 1, "the resets are in the form's footer"
    leave(tile)


def test_an_edit_goes_to_every_day_edited_together(page, case):
    """A day's click selects every day with the same programme; an edit
    then changes them all, and Reset all puts them back."""
    tile, _ = first_tile(page)
    open_tab(tile, "program")
    day_button(tile, "monday").click()
    together = marked(tile, "selected")
    assert "monday" in together, "the edited day is among the selected"
    assert marked(tile, "editing") == ["monday"], "Monday is the day edited"
    bump_first_temp(tile)
    assert wait_until(page, "!!document.querySelector('.prog-days .day.dirty')", 5), "the edit marks days changed"
    assert marked(tile, "dirty") == together, "every day edited together changed"
    tile.locator(".prog-reset-all").click()
    assert marked(tile, "dirty") == [], "Reset all puts every day back"
    leave(tile)


def test_only_this_day_sets_the_day_apart(page, case):
    """With "Only {day}" on, an edit changes the day alone, which leaves
    the selection; Reset day puts it back."""
    tile, _ = first_tile(page)
    open_tab(tile, "program")
    day_button(tile, "monday").click()
    only_on(tile)
    bump_first_temp(tile)
    assert wait_until(page, "!!document.querySelector('.prog-days .day.dirty')", 5), "the edit marks the day changed"
    assert marked(tile, "dirty") == ["monday"], "only Monday changed"
    assert marked(tile, "selected") == ["monday"], "Monday alone is selected"
    reset_day = tile.locator(".prog-reset-day")
    assert reset_day.is_enabled(), "Reset day is on for a changed day"
    reset_day.click()
    assert marked(tile, "dirty") == [], "Reset day puts the day back"
    assert reset_day.is_disabled(), "and turns itself off"
    leave(tile)


def test_copy_gives_a_day_the_edited_programme(page, case):
    """A day ticked in the copy section and copied gets the edited day's
    programme: it is changed too, and with "only" on it stays apart."""
    tile, _ = first_tile(page)
    open_tab(tile, "program")
    day_button(tile, "monday").click()
    only_on(tile)
    bump_first_temp(tile)
    assert wait_until(page, "!!document.querySelector('.prog-days .day.dirty')", 5), "Monday changed"
    copy_days = tile.locator(".day-copy .day-bar.week button")
    tuesday = copy_days.nth(DAYS.index("tuesday"))
    assert tuesday.is_enabled(), "Tuesday, now different, can be ticked"
    tuesday.click()
    assert "chosen" in tile.locator(".day-copy .day-bar.week button").nth(1).get_attribute("class"), "Tuesday is ticked"
    tile.locator(".day-copy .day.copy").click()
    assert marked(tile, "dirty") == ["monday", "tuesday"], "Tuesday got Monday's programme"
    assert marked(tile, "selected") == ["monday"], "with only on, Tuesday stays apart"
    tile.locator(".prog-reset-all").click()
    leave(tile)


def test_a_day_holds_thirteen_switch_points(page, case):
    """+ adds a switch point until a day has 13, then it is gone; the bar
    has a segment per point."""
    tile, _ = first_tile(page)
    open_tab(tile, "program")
    rows = tile.locator(".prog-point")
    assert tile.locator(".prog-bar .prog-seg").count() == rows.count(), "a segment per point"
    for _ in range(13):
        if rows.count() >= 13:
            break
        tile.locator(".prog-point").last.locator("button.step").click()
    assert rows.count() == 13, "13 switch points"
    assert tile.locator(".prog-point").last.locator("button.step").count() == 0, "no + at 13"
    assert tile.locator(".prog-bar .prog-seg").count() == 13, "the bar has 13 segments"
    tile.locator(".prog-reset-all").click()
    leave(tile)
