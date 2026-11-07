# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Settings → Backup and restore against the server run.sh starts, whose
store holds the fake Cube: the backup of the Cube as it is now downloads,
one taken by hand shows in the list and downloads, its plan says that
the Cube holds it already, it compares with the one before, and it is
deleted again. The four cases share the server, so each removes what it
took."""
from helpers import assert_busy, busy_state, hold, named_buttons, open_settings, wait_held, wait_until

REASON_MANUAL = {"en": "by hand", "de": "von Hand"}
NOTHING = {"en": "The Cube already holds this backup: nothing to send.",
           "de": "Der Cube enthält diese Sicherung bereits: nichts zu senden."}


def test_backup_panel(page, case):
    open_settings(page)
    panel = page.locator(".backup-settings")
    panel.scroll_into_view_if_needed()
    assert wait_until(page, "backupUI.history && document.getElementById('backups-list').children.length > 0", 10), "the list loaded"
    named_buttons(page, "the backup panel")

    with page.expect_download() as now:
        page.locator("#backup-download").click()
    assert now.value.suggested_filename.endswith(".yaml"), now.value.suggested_filename

    before = page.evaluate("backupUI.list.filter((b) => b.reason === 'manual').length")
    page.locator("#backup-take").click()
    assert wait_until(page, "backupUI.list.filter((b) => b.reason === 'manual').length === %d" % (before + 1), 10), "the backup in the list"
    taken = page.evaluate("backupUI.list.find((b) => b.reason === 'manual').id")
    index = page.evaluate("backupUI.list.findIndex((b) => b.id === %d)" % taken)
    row = page.locator("#backups-list .backup-row").nth(index)
    assert row.locator(".backup-reason").inner_text() == REASON_MANUAL[case.lang], "its reason"

    with page.expect_download() as stored:
        row.locator(".backup-actions button").nth(0).click()
    assert stored.value.suggested_filename.endswith(".yaml"), stored.value.suggested_filename

    row.locator(".backup-actions button").nth(1).click()
    assert wait_until(page, "!!document.querySelector('#restore-box .restore-plan')", 10), "the plan"
    plan = page.locator("#restore-box .restore-plan")
    assert plan.locator(".restore-result").inner_text() == NOTHING[case.lang], "the Cube holds it already"
    named_buttons(page, "the plan")
    plan.locator(".restore-actions button").last.click()
    assert wait_until(page, "!document.querySelector('#restore-box .restore-plan')", 5), "the plan closed"

    if page.evaluate("backupUI.list.length > %d" % (index + 1)):
        row.locator(".backup-actions button").nth(2).click()
        assert wait_until(page, "!!backupUI.diff[%d]" % taken, 10), "the comparison"
        row = page.locator("#backups-list .backup-row").nth(index)
        assert row.locator(".backup-diff").is_visible(), "the comparison shown"

    row.locator(".backup-actions button").last.click()
    row.locator(".confirm-yes").click()
    assert wait_until(page, "!backupUI.list.some((b) => b.id === %d)" % taken, 10), "the backup deleted"
    page.locator("#view-tiles").click()


def test_back_up_now_turns_until_answered(page, case):
    """Back up now turns until the server answers, and a second click
    sends nothing; the POST is answered with a stub, so no backup is
    stored."""
    open_settings(page)
    assert wait_until(page, "backupUI.history && !document.getElementById('backup-take').classList.contains('hidden')", 10), \
        "the server keeps backups"
    held, sent = [], []
    hold(page, "**/backups", "POST", held, sent)
    try:
        page.locator("#backup-take").click()
        assert wait_held(page, held), "the POST is sent"
        assert_busy(page, "#backup-take", "Back up now")
        page.locator("#backup-take").click(force=True)
        page.wait_for_timeout(300)
        assert len(sent) == 1, f"one POST for two clicks: {sent}"
        for route in held:
            route.fulfill(status=201, content_type="application/json", body="{}")
        assert wait_until(page, "!document.getElementById('backup-take').classList.contains('busy')", 5), "Back up now is back"
        assert busy_state(page, "#backup-take") == [False, False, None], "and on"
    finally:
        page.unroute("**/backups")
    page.locator("#view-tiles").click()
