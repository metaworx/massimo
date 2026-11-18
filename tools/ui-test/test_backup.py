# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Backup and restore, a box per Cube (AP GlobalMenu, Block 5), against
the server run.sh starts, whose store holds the fake Cube. On Cube
settings the chosen Cube's box: the backup as it is now downloads, one
taken by hand shows in the list and downloads, its plan says that the
Cube holds it already, it compares with the one before, and it is
deleted again. In System settings a fold per Cube, a single Cube's
open, each with its count; a fold kept as it was when the page is shown
again. The four cases share the server, so each removes what it
took."""
from conftest import api_call
from helpers import assert_busy, busy_state, hold, named_buttons, show_view, wait_held, wait_until

SERIAL = "KEQ0000100"
SECOND = "KEQ0000200"
BOX = "#cube-backup .backup-box"
REASON_MANUAL = {"en": "by hand", "de": "von Hand"}
NOTHING = {"en": "The Cube already holds this backup: nothing to send.",
           "de": "Der Cube enthält diese Sicherung bereits: nichts zu senden."}


def manual(page):
    return page.evaluate(f"backupOf('{SERIAL}').list.filter((b) => b.reason === 'manual').length")


def test_backup_box_on_cube_settings(page, case):
    show_view(page, "cube")
    box = page.locator(BOX)
    box.scroll_into_view_if_needed()
    assert box.get_attribute("data-serial") == SERIAL, "the chosen Cube's box"
    assert wait_until(page, f"backupOf('{SERIAL}').loaded && backupOf('{SERIAL}').history"
                      f" && document.querySelector('{BOX} .backups-list').children.length > 0", 10), "the list loaded"
    named_buttons(page, "the backup box")
    kept = page.evaluate(f"tn('backup.badge.kept', backupOf('{SERIAL}').list.length)")
    assert page.locator("#cube-backup .section-badges").inner_text() == kept, "the count in the card's head"

    with page.expect_download() as now:
        box.locator(".backup-download").click()
    assert now.value.suggested_filename.endswith(".yaml"), now.value.suggested_filename

    before = manual(page)
    box.locator(".backup-take").click()
    assert wait_until(page, f"backupOf('{SERIAL}').list.filter((b) => b.reason === 'manual').length === {before + 1}", 10), \
        "the backup in the list"
    taken = page.evaluate(f"backupOf('{SERIAL}').list.find((b) => b.reason === 'manual').id")
    index = page.evaluate(f"backupOf('{SERIAL}').list.findIndex((b) => b.id === {taken})")
    row = box.locator(".backups-list .backup-row").nth(index)
    assert row.locator(".backup-reason").inner_text() == REASON_MANUAL[case.lang], "its reason"

    with page.expect_download() as stored:
        row.locator(".backup-actions button").nth(0).click()
    assert stored.value.suggested_filename.endswith(".yaml"), stored.value.suggested_filename

    row.locator(".backup-actions button").nth(1).click()
    assert wait_until(page, f"!!document.querySelector('{BOX} .restore-box .restore-plan')", 10), "the plan"
    plan = box.locator(".restore-box .restore-plan")
    assert plan.locator(".restore-result").inner_text() == NOTHING[case.lang], "the Cube holds it already"
    named_buttons(page, "the plan")
    plan.locator(".restore-actions button").last.click()
    assert wait_until(page, f"!document.querySelector('{BOX} .restore-box .restore-plan')", 5), "the plan closed"

    if page.evaluate(f"backupOf('{SERIAL}').list.length > {index + 1}"):
        row.locator(".backup-actions button").nth(2).click()
        assert wait_until(page, f"!!backupOf('{SERIAL}').diff[{taken}]", 10), "the comparison"
        row = box.locator(".backups-list .backup-row").nth(index)
        assert row.locator(".backup-diff").is_visible(), "the comparison shown"

    row.locator(".backup-actions button").last.click()
    row.locator(".confirm-yes").click()
    assert wait_until(page, f"!backupOf('{SERIAL}').list.some((b) => b.id === {taken})", 10), "the backup deleted"
    show_view(page, "tiles")


def test_backup_folds_in_system_settings(page, case):
    """One Cube: one fold, open, its sign, name, serial and count in its
    head, its box; a second Cube added: two folds, the first kept open as
    it was, the second closed with a box of its own; the card counts every
    Cube's backups."""
    show_view(page, "system")
    folds = page.locator("#backups-cubes > .backup-fold")
    assert wait_until(page, "document.querySelectorAll('#backups-cubes > .backup-fold').length === 1", 5), "one fold"
    first = folds.first
    assert first.get_attribute("data-serial") == SERIAL and first.get_attribute("open") is not None, "the Cube's, open"
    head = first.locator(":scope > summary")
    assert head.locator(".conn-sign").count() == 1 and SERIAL in head.inner_text(), "its sign and serial"
    assert wait_until(page, f"backupOf('{SERIAL}').loaded", 10), "its list loaded"
    kept = page.evaluate(f"tn('backup.badge.kept', backupOf('{SERIAL}').list.length)")
    assert wait_until(page, f"document.querySelector('#backups-cubes > .backup-fold .panel-badges').textContent === {kept!r}", 5), \
        "its count in its head"
    assert first.locator(".backup-box").get_attribute("data-serial") == SERIAL, "its box"
    first.evaluate("(d) => { d.dataset.kept = 'yes'; }")
    api_call("POST", "api/v1/cubes", {"serial": SECOND, "address": "127.0.0.1:9", "name": "Second"})
    try:
        page.evaluate("loadCubes()")
        assert wait_until(page, f"state.cubes.some((c) => c.serial === '{SECOND}')", 5), "the second Cube listed"
        show_view(page, "tiles")
        show_view(page, "system")
        assert wait_until(page, "document.querySelectorAll('#backups-cubes > .backup-fold').length === 2", 5), "two folds"
        assert folds.first.get_attribute("data-kept") == "yes" and folds.first.get_attribute("open") is not None, \
            "the first kept as it was, open"
        second = folds.nth(1)
        assert second.get_attribute("data-serial") == SECOND and second.get_attribute("open") is None, "the second closed"
        second.locator(":scope > summary").click()
        assert second.locator(".backup-box").get_attribute("data-serial") == SECOND, "its own box"
        assert wait_until(page, f"backupOf('{SECOND}').loaded", 10), "its list asked for"
        total = page.evaluate(f"tn('backup.badge.kept', backupOf('{SERIAL}').list.length + backupOf('{SECOND}').list.length)")
        assert page.locator("#sys-backup > .section-head .section-badges").inner_text() == total, "the card counts them all"
    finally:
        api_call("DELETE", "api/v1/cubes/" + SECOND)
        page.evaluate("loadCubes()")
    show_view(page, "tiles")


def test_back_up_now_turns_until_answered(page, case):
    """Back up now turns until the server answers, and a second click
    sends nothing; the POST is answered with a stub, so no backup is
    stored. What the second click sent is known once a read the page
    sends after it has come through the same route: a POST of the
    click's would have come before it."""
    show_view(page, "cube")
    take = BOX + " .backup-take"
    assert wait_until(page, f"backupOf('{SERIAL}').loaded && !document.querySelector('{take}').classList.contains('hidden')", 10), \
        "the server keeps backups"
    held, sent, came = [], [], []
    hold(page, "**/backups", "POST", held, sent)
    page.on("request", lambda r: came.append(r.method) if r.url.endswith("/backups") else None)
    try:
        page.locator(take).click()
        assert wait_held(page, held), "the POST is sent"
        assert_busy(page, take, "Back up now")
        mark = len(came)
        page.locator(take).click(force=True)
        page.evaluate(f"api('GET', cubeRoute('{SERIAL}') + '/backups')")  # the read after the click, through the route
        assert "GET" in came[mark:], f"the read after the click came: {came}"
        assert came.count("POST") == 1, f"one POST for two clicks: {came}"
        for route in held:
            route.fulfill(status=201, content_type="application/json", body="{}")
        assert wait_until(page, f"!document.querySelector('{take}').classList.contains('busy')", 5), "Back up now is back"
        assert busy_state(page, take) == [False, False, None], "and on"
        assert came.count("POST") == 1 and len(sent) == 1, f"and none sent while the first waited or once it was answered: {came}"
    finally:
        page.unroute("**/backups")
    show_view(page, "tiles")
