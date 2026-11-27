# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The Cubes' backups, a box per Cube (AP GlobalMenu, Block 5), against
the server run.sh starts, whose store holds the fake Cube. On Cube
settings, in Backup and restore, the chosen Cube's box: the backup as
it is now downloads, one taken by hand shows in the list and downloads,
its plan says that the Cube holds it already, it compares with the one
before, and it is deleted again. On Maintenance, in Cube backups (AP
ServerBackup, Block 3), a fold per Cube, a single Cube's open, each
with its count; a fold kept as it was when the page is shown again; a
role with backup alone has Maintenance with Cube backups and no System
settings. A load onto either page shows the Cube's list, read once,
though the page is shown before it knows its Cubes; under a clean
Cube's list a line says why no newer backup follows, and none does
while one is pending. A Cube the server keeps backups of and does not
serve, by the server's list of them, has a fold after the served
Cubes', without what needs the Cube served. The four cases share the
server, so each removes what it took."""
import json
from datetime import datetime, timedelta, timezone

from conftest import api_call
from helpers import VIEWER, assert_busy, busy_state, css_hidden, hold, named_buttons, open_main_menu, safe_route, show_view, wait_held, wait_until

SERIAL = "KEQ0000100"
SECOND = "KEQ0000200"
BOX = "#cube-backup .backup-box"
SYS_BOX = f'#backups-cubes > .backup-fold[data-serial="{SERIAL}"] .backup-box'
LIST = f"**/api/v1/cubes/{SERIAL}/backups"
REASON_MANUAL = {"en": "by hand", "de": "von Hand"}
NOTHING = {"en": "The Cube already holds this backup: nothing to send.",
           "de": "Der Cube enthält diese Sicherung bereits: nichts zu senden."}


def kept(serial=SERIAL):
    """The Cube's backups the server keeps, newest first, as the page lists them."""
    return (api_call("GET", f"api/v1/cubes/{serial}/backups") or {}).get("backups") or []


def manual():
    return len([b for b in kept() if b["reason"] == "manual"])


def rows(page, n, box=BOX):
    """The box's list loaded (something in it: its rows, or that there is none) and showing n rows."""
    return wait_until(page, f"document.querySelector('{box} .backups-list')?.children.length > 0"
                            f" && document.querySelectorAll('{box} .backups-list .backup-row').length === {n}", 10)


def test_backup_box_on_cube_settings(page, case):
    show_view(page, "cube")
    box = page.locator(BOX)
    box.scroll_into_view_if_needed()
    assert box.get_attribute("data-serial") == SERIAL, "the chosen Cube's box"
    assert rows(page, len(kept())), "the list loaded"
    named_buttons(page, "the backup box")
    count = page.evaluate("(n) => tn('backup.badge.kept', n)", len(kept()))
    assert page.locator("#cube-backup .section-badges").inner_text() == count, "the count in the card's head"

    with page.expect_download() as now:
        box.locator(".backup-download").click()
    assert now.value.suggested_filename.endswith(".yaml"), now.value.suggested_filename

    before = manual()
    box.locator(".backup-take").click()
    shown = f"[...document.querySelectorAll('{BOX} .backup-reason')].filter((e) => e.textContent === {REASON_MANUAL[case.lang]!r}).length"
    assert wait_until(page, f"{shown} === {before + 1}", 10) and manual() == before + 1 and rows(page, len(kept())), "the backup in the list"
    taken = next(b["id"] for b in kept() if b["reason"] == "manual")
    index = next(i for i, b in enumerate(kept()) if b["id"] == taken)
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

    if len(kept()) > index + 1:
        row.locator(".backup-actions button").nth(2).click()
        row = box.locator(".backups-list .backup-row").nth(index)
        row.locator(".backup-diff").wait_for(state="visible", timeout=10000)

    row.locator(".backup-actions button").last.click()
    left = len(kept()) - 1
    row.locator(".confirm-yes").click()
    assert rows(page, left) and not any(b["id"] == taken for b in kept()), "the backup deleted"
    show_view(page, "tiles")


def test_backup_folds_on_maintenance(page, case):
    """One Cube: one fold, open, its sign, name, serial and count in its
    head, its box; a second Cube added: two folds, the first kept open as
    it was, the second closed with a box of its own; the card counts every
    Cube's backups."""
    show_view(page, "maint")
    folds = page.locator("#backups-cubes > .backup-fold")
    assert wait_until(page, "document.querySelectorAll('#backups-cubes > .backup-fold').length === 1", 5), "one fold"
    first = folds.first
    assert first.get_attribute("data-serial") == SERIAL and first.get_attribute("open") is not None, "the Cube's, open"
    head = first.locator(":scope > summary")
    assert head.locator(".cube-icon").count() == 1 and head.locator(".conn-sign").count() == 1 and SERIAL in head.inner_text(), \
        "the cube, its serial and its sign"
    order = head.evaluate("(s) => [...s.children].map((e) => e.classList.contains('cube-icon') ? 'cube' : e.classList.contains('conn-sign') ? 'sign' : "
                          "e.classList.contains('mono') ? 'serial' : '').filter(Boolean)")
    assert order == ["cube", "serial", "sign"], f"the cube before the serial, the sign after it: {order}"
    count = page.evaluate("(n) => tn('backup.badge.kept', n)", len(kept()))
    assert wait_until(page, f"document.querySelector('#backups-cubes > .backup-fold .panel-badges')?.textContent === {count!r}", 10), \
        "its list loaded, its count in its head"
    assert first.locator(".backup-box").get_attribute("data-serial") == SERIAL, "its box"
    first.evaluate("(d) => { d.dataset.kept = 'yes'; }")
    api_call("POST", "api/v1/cubes", {"serial": SECOND, "address": "127.0.0.1:9", "name": "Second"})
    try:
        page.evaluate("loadCubes()")
        assert wait_until(page, f"state.cubes.some((c) => c.serial === '{SECOND}')", 5), "the second Cube listed"
        show_view(page, "tiles")
        show_view(page, "maint")
        assert wait_until(page, "document.querySelectorAll('#backups-cubes > .backup-fold').length === 2", 5), "two folds"
        assert folds.first.get_attribute("data-kept") == "yes" and folds.first.get_attribute("open") is not None, \
            "the first kept as it was, open"
        second = folds.nth(1)
        assert second.get_attribute("data-serial") == SECOND and second.get_attribute("open") is None, "the second closed"
        second.locator(":scope > summary").click()
        assert second.locator(".backup-box").get_attribute("data-serial") == SECOND, "its own box"
        assert wait_until(page, f"document.querySelector(\"#backups-cubes > .backup-fold[data-serial='{SECOND}'] .backups-list\").children.length > 0", 10), \
            "its list asked for"
        total = page.evaluate("(n) => tn('backup.badge.kept', n)", len(kept()) + len(kept(SECOND)))
        assert wait_until(page, f"document.querySelector('#sys-backup > .section-head .section-badges').innerText === {total!r}", 5), \
            "the card counts them all"
    finally:
        api_call("DELETE", "api/v1/cubes/" + SECOND)
        page.evaluate("loadCubes()")
    show_view(page, "tiles")


def test_backup_alone_has_maintenance(page, case):
    """A role with backup alone (/me stubbed; the owner's token still
    answers the reads): ☰ has Maintenance and no System settings;
    Maintenance holds Cube backups, by its title, with the Cube's fold
    and its list, and not the Maintenance card, its sections menu naming
    Cube backups; System settings does not open."""
    me = dict(VIEWER, capabilities=["read", "preferences", "backup"])
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(me)))
    try:
        page.evaluate("loadMe()")
        assert wait_until(page, "document.body.classList.contains('no-system') && !document.body.classList.contains('no-maint')"
                                " && !document.body.classList.contains('no-backup')", 5), "backup alone: Maintenance, no System settings"
        open_main_menu(page)
        assert page.locator("#menu-maint").is_visible(), "Maintenance in ☰"
        assert css_hidden(page, "#menu-system"), "System settings not in ☰"
        page.keyboard.press("Escape")
        show_view(page, "maint")
        card = page.locator("#sys-backup")
        assert card.is_visible(), "Cube backups on Maintenance"
        assert card.locator(".section-title").inner_text() == page.evaluate("t('backup.cubesTitle')"), "by its title"
        assert css_hidden(page, "#sys-maint"), "not the Maintenance card"
        assert wait_until(page, f"document.querySelector('{SYS_BOX} .backups-list')?.children.length > 0", 10), "the Cube's fold with its list"
        menu = page.locator("#maint-page .sec-menu")
        menu.locator("summary").click()
        toc = menu.locator(".toc-link")
        shown = [toc.nth(i).get_attribute("data-target") for i in range(toc.count()) if toc.nth(i).is_visible()]
        assert "sys-backup" in shown and "sys-maint" not in shown, f"the sections menu names Cube backups and not Maintenance: {shown}"
        page.keyboard.press("Escape")
        page.evaluate("MassimoParts.pages.showView('system')")
        assert page.locator("#system").is_hidden(), "System settings does not open"
    finally:
        page.unroute("**/api/v1/me")
        page.evaluate("loadMe()")
    assert wait_until(page, "!document.body.classList.contains('no-system')", 5), "the owner has System settings again"
    show_view(page, "tiles")


def test_back_up_now_turns_until_answered(page, case):
    """Back up now turns until the server answers, and a second click
    sends nothing; the POST is answered with a stub, so no backup is
    stored. What the second click sent is known once a read the page
    sends after it has come through the same route: a POST of the
    click's would have come before it."""
    show_view(page, "cube")
    take = BOX + " .backup-take"
    assert wait_until(page, f"!!document.querySelector('{BOX} .backups-list').children.length && !document.querySelector('{take}').classList.contains('hidden')", 10), \
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
        page.evaluate(f"api('GET', 'api/v1/cubes/{SERIAL}/backups')")  # the read after the click, through the route
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


# The loads that drew the card empty (the analysis of AP BackupWatch, the
# cases A, B, E and H), and the one that had a Cube to read from at once:
# the address, the Cube stored or none ('massimo.serial'), the page's
# section and the box of the Cube.
LOADS = [("#maintenance", False, "#maint-page", SYS_BOX), ("#maintenance", True, "#maint-page", SYS_BOX),
         ("#cube", False, "#cube-page", BOX), ("#cube", True, "#cube-page", BOX)]


def test_backup_lists_after_a_load_onto_their_page(page, case):
    """A load onto Maintenance without a Cube stored, as a first open
    is, a reload onto it with the Cube stored, as checkVersion's after an
    update is, and a reload onto Cube settings without and with a stored
    Cube: each shows the Cube's list, though the page shows its page
    before it knows its Cubes, and reads it once, whoever asks for it
    (the page shown, the box drawn)."""
    n = len(kept())
    assert n > 0, "the Cube has run.sh's backup by hand"
    read = []
    page.on("request", lambda r: read.append(r.url) if r.method == "GET" and r.url.endswith(f"/cubes/{SERIAL}/backups") else None)
    for address, stored, section, box in LOADS:
        what = f"{address}, " + ("the Cube stored" if stored else "no Cube stored")
        page.evaluate("([address, serial]) => {"
                      " if (serial) localStorage.setItem('massimo.serial', serial); else localStorage.removeItem('massimo.serial');"
                      " history.replaceState(null, '', location.pathname + address); }", [address, SERIAL if stored else None])
        mark = len(read)
        page.reload()
        page.locator(section).wait_for(state="visible", timeout=10000)
        assert wait_until(page, "state.eventsOpen", 10), f"{what}: the page started"
        assert rows(page, n, box), f"{what}: the list's {n} rows"
        assert len(read) - mark == 1, f"{what}: the list read once: {read[mark:]}"
    show_view(page, "tiles")


def test_clean_cube_says_why_no_backup_follows(page, case, reads):
    """Under a list with backups and nothing pending (the list's changed
    and due taken out of the server's answer), on Cube settings and on
    Maintenance, the line that nothing changed since the newest
    backup; with a backup pending (changed and due put in), the pending
    line instead, and no such line."""
    assert kept(), "the Cube has run.sh's backup by hand"
    now = datetime.now(timezone.utc)

    def clean(body):
        body.pop("changed", None)
        body.pop("due", None)
        return body

    def pending(body):
        body["changed"] = (now - timedelta(minutes=5)).isoformat()
        body["due"] = (now + timedelta(minutes=55)).isoformat()
        return body

    text = page.evaluate("t('backup.unchanged')")
    line = " .backups-list .backup-clean"
    try:
        reads.change(LIST, clean)
        for view, box in (("cube", BOX), ("maint", SYS_BOX)):
            show_view(page, view)
            assert rows(page, len(kept()), box), f"{view}: the list loaded"
            assert wait_until(page, f"document.querySelector('{box}{line}')?.textContent === {text!r}", 5), f"{view}: the clean Cube's line"
            assert page.locator(box + " .backup-pending").is_hidden(), f"{view}: nothing pending"
        reads.clear(LIST)
        reads.change(LIST, pending)
        for view, box in (("cube", BOX), ("maint", SYS_BOX)):
            show_view(page, "tiles")
            show_view(page, view)
            assert wait_until(page, f"!document.querySelector('{box} .backup-pending').classList.contains('hidden')", 5), f"{view}: the pending line"
            assert page.locator(box + line).count() == 0, f"{view}: no clean Cube's line while a backup is pending"
            assert rows(page, len(kept()), box), f"{view}: the list still shown"
    finally:
        reads.clear(LIST)
    show_view(page, "tiles")


# A Cube the server keeps backups of and does not serve, which discovery
# does not hear, and one found without backups: the list of the Cubes not
# served, the Cubes' list and the first's backups are the test's, so that
# the shared server keeps no row or backup of either.
OTHER = "KEQ0000300"
EMPTY = "KEQ0000301"
OTHER_NAME = "Old hall"
OTHER_BOX = f'#backups-cubes > .backup-fold[data-serial="{OTHER}"] .backup-box'
OTHER_STORED = [
    {"id": 902, "taken_at": "2026-11-20T10:00:00Z", "reason": "manual", "format": "yaml", "size": 24000, "sha256": "ab" * 32, "note": "the old Cube"},
    {"id": 901, "taken_at": "2026-11-09T10:00:00Z", "reason": "auto", "format": "json", "size": 23990, "sha256": "cd" * 32},
]


def found_cube(serial, backups):
    """A Cube found on the network and not served, as GET /api/v1/cubes lists it, with the backups the server keeps of it."""
    return {"serial": serial, "kind": "cube", "configured": False, "address": "192.0.2.30", "rf_address": "0a0302", "firmware": "1.1.3",
            "found_at": "2026-11-24T08:00:00+01:00", "last_seen": "2026-11-24T08:00:00+01:00", "backups": backups}


def test_backups_of_a_cube_not_served(page, case, reads):
    """After the served Cube's fold, one for the Cube not served that
    keeps backups, from the server's list of them, though discovery does
    not hear it, and none for a found Cube without backups: closed at
    first, its name, serial and that it is not served, its count in its
    head and in the card's; its box has the format, a note and the
    backups with Download, Compare with the one before and Delete, and no
    Download backup, Take a backup now, Restore… or Restore from file…; a
    backup downloads, compares and is deleted, after which the list of
    the Cubes not served is read again, and the last one deleted takes
    the fold away."""
    assert kept(), "the Cube has run.sh's backup by hand"
    stored = list(OTHER_STORED)
    unserved_reads = []

    def answer_unserved(route):
        if route.request.method != "GET":
            route.fallback()
            return
        unserved_reads.append(route.request.url)
        listed = [{"serial": OTHER, "name": OTHER_NAME, "backups": len(stored), "newest": stored[0]["taken_at"]}] if stored else []
        route.fulfill(status=200, content_type="application/json", body=json.dumps(listed))

    def answer_list(route):
        if route.request.method != "GET":
            route.fallback()
            return
        route.fulfill(status=200, content_type="application/json", body=json.dumps({"backups": stored}))

    def answer_file(route):
        method, url = route.request.method, route.request.url
        if method == "DELETE":
            gone = int(url.rstrip("/").rsplit("/", 1)[1])
            stored[:] = [b for b in stored if b["id"] != gone]
            route.fulfill(status=204, body="")
        elif method != "GET":
            route.fallback()
        elif "/diff?" in url:
            route.fulfill(status=200, content_type="application/json",
                          body=json.dumps({"steps": [{"what": "write room and device names", "radio": False}], "notes": [], "radio_commands": 0}))
        else:
            route.fulfill(status=200, content_type="application/yaml", body="version: 1\n",
                          headers={"Content-Disposition": f"attachment; filename=massimo-backup-{OTHER}-20261120-110000.yaml"})

    folds = "#backups-cubes > .backup-fold"
    routes = [("**/api/v1/backups/unserved", answer_unserved), (f"**/api/v1/cubes/{OTHER}/backups", answer_list),
              (f"**/api/v1/cubes/{OTHER}/backups/**", answer_file)]
    routed = [(pattern, safe_route(page, pattern, handler)) for pattern, handler in routes]
    try:
        reads.found(found_cube(EMPTY, 0))
        page.evaluate("loadCubes()")
        assert page.evaluate(f"!state.found.some((c) => c.serial === '{OTHER}') && state.found.some((c) => c.serial === '{EMPTY}')"), \
            "the Cube not served is no found Cube"
        show_view(page, "maint")
        assert wait_until(page, f"[...document.querySelectorAll('{folds}')].map((f) => f.dataset.serial).join() === '{SERIAL},{OTHER}'", 10), \
            "the served Cube's fold, then the one not served with backups, none for the found one without"
        fold = page.locator(f'{folds}[data-serial="{OTHER}"]')
        assert fold.get_attribute("open") is None, "closed at first"
        head = fold.locator(":scope > summary")
        text = head.inner_text()
        assert page.evaluate("t('backup.notServed')") in text and OTHER in text and OTHER_NAME in text, "its name and serial, marked not served"
        count = page.evaluate("(n) => tn('backup.badge.kept', n)", len(stored))
        assert wait_until(page, f"[...document.querySelectorAll('{folds}[data-serial=\"{OTHER}\"] > summary .flag')].some((b) => b.textContent === {count!r})", 10), \
            "its count in its head"
        total = page.evaluate("(n) => tn('backup.badge.kept', n)", len(kept()) + len(stored))
        assert wait_until(page, f"document.querySelector('#sys-backup > .section-head .section-badges').innerText === {total!r}", 10), \
            "the card counts its backups too"
        head.click()
        box = page.locator(OTHER_BOX)
        assert rows(page, len(stored), OTHER_BOX), "its backups listed"
        assert box.locator(f"select#backup-format-sys-{OTHER}").count() == 1, "the format"
        assert box.locator(".backup-download, .backup-take, .restore-file").count() == 0, "no Download backup, Take a backup now or Restore from file…"
        assert box.locator(".backup-unserved-note").inner_text() == page.evaluate("t('backup.notServedNote')"), "the note"
        assert box.locator(".backup-clean, .restore-plan").count() == 0, "no clean Cube's line, no plan"
        words = page.evaluate("[t('backup.downloadOne'), t('backup.compare'), t('action.delete')]")
        newest, oldest = box.locator(".backups-list .backup-row").nth(0), box.locator(".backups-list .backup-row").nth(1)
        assert newest.locator(".backup-actions button").all_inner_texts() == words, "Download, Compare with the one before and Delete"
        assert oldest.locator(".backup-actions button").all_inner_texts() == [words[0], words[2]], "the oldest without Compare"
        named_buttons(page, "the fold of a Cube not served")

        with page.expect_download() as file:
            newest.locator(".backup-actions button").nth(0).click()
        assert file.value.suggested_filename.startswith(f"massimo-backup-{OTHER}-"), file.value.suggested_filename
        newest.locator(".backup-actions button").nth(1).click()
        newest.locator(".backup-diff").wait_for(state="visible", timeout=10000)
        assert "write room and device names" in newest.locator(".backup-diff").inner_text(), "what changed from the one before"

        mark = len(unserved_reads)
        oldest.locator(".backup-actions button").last.click()
        oldest.locator(".confirm-yes").click()
        assert rows(page, 1, OTHER_BOX), "the backup deleted"
        for _ in range(50):  # the route's handler runs while the page is waited on
            if len(unserved_reads) > mark:
                break
            page.wait_for_timeout(100)
        assert len(unserved_reads) > mark, "the list of the Cubes not served read again after the delete"
        assert fold.count() == 1, "the fold kept while a backup is left"
        row = box.locator(".backups-list .backup-row").nth(0)
        row.locator(".backup-actions button").last.click()
        row.locator(".confirm-yes").click()
        assert wait_until(page, f"!document.querySelector('{folds}[data-serial=\"{OTHER}\"]')", 10), "the last one deleted, the fold gone"
    finally:
        reads.clear()
        for pattern, handler in routed:
            page.unroute(pattern, handler)
        page.evaluate("loadCubes()")
    show_view(page, "tiles")
