# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Maintenance → Server backups: the owner's restore of a server backup
and its check with the key (AP ServerBackup, Block 5, D6-A, D7-A), in
the dialog a backup's row opens. Against the server run.sh starts, a
backup made of it for a classic key: the key refused when it does not
open the backup, read from a file in the browser, the plan's scopes with
their counts and the fake Cube by its serial, the choice planned and
confirmed, the key cleared as the dialog closes and at a sign-out; the
check with the key and its verdict. With the plan and the restore
stubbed, so that the test server is never restored: a backup of another
box, its identity with the warning and that box still answering, an env
file this server cannot write, off with why, the TLS files; a restore
refused with the rows missing; a restore whose server restarts, followed
until it is back (/healthz stubbed), with its Cube offered to the Cube
restore. The four cases share the server, so each test starts from
nothing and takes away what it made (test_server_backups.clean)."""
import json

from conftest import api_call
from helpers import named_buttons, safe_route, show_view, wait_until
from test_server_backups import BACKUPS, CARD, KEYS, TARGETS, clean, data_dir, text

DIALOG = "dialog.sb-restore-dialog"
PLAN = "**/api/v1/server-backups/*/restore/plan"
RESTORE = "**/api/v1/server-backups/*/restore"


def made_backup(case):
    """A classic key made and confirmed, a local target, and a backup of
    the server onto it: the backup's uuid and the key file's text. The
    target keeps the last 50, so that the schedule's run that the server
    makes up five minutes after its start (serverBackupLoop), which may
    come meanwhile, prunes nothing of the test's: the target's default
    keeps the newest of each day alone."""
    clean()
    made = api_call("POST", KEYS, {"label": "restore", "make": "classic"})
    api_call("POST", f"{KEYS}/{made['key']['uuid']}/confirm", {"last_six": made["identity"][-6:]})
    api_call("POST", TARGETS, {"name": "Here", "kind": "local", "config": {"path": data_dir(f"sb-restore-{case.width}-{case.lang}")},
                               "retention": {"last": 50}})
    return api_call("POST", BACKUPS, {})["backup"]["uuid"], made["key_file"]


def other_key():
    """The key file of a classic key the server no longer has, which opens no backup of it."""
    made = api_call("POST", KEYS, {"label": "other", "make": "classic"})
    api_call("DELETE", f"{KEYS}/{made['key']['uuid']}")
    return made["key_file"]


def open_dialog(page, uuid, button=".sb-restore"):
    """Maintenance shown, the backup's fold opened and its Restore… (or
    another of its buttons) pressed: the dialog."""
    show_view(page, "maint")
    page.locator(CARD).scroll_into_view_if_needed()
    fold = page.locator(f".sb-backup[data-uuid='{uuid}']")
    fold.wait_for(timeout=10000)
    if fold.get_attribute("open") is None:
        fold.locator(":scope > summary").click()
    fold.locator(button).click()
    dialog = page.locator(DIALOG)
    dialog.wait_for(state="visible", timeout=5000)
    return dialog


def tick(page, id):
    """A switch of the dialog flipped by its label, as a user does."""
    page.locator(f"{DIALOG} label.switch:has(#{id})").click()


def focused(page):
    """The id, or else the class, of the element that has the focus."""
    return page.evaluate("document.activeElement.id || document.activeElement.className")


def is_open(page):
    """Whether the dialog is open."""
    return page.evaluate(f"!!document.querySelector('{DIALOG}')?.open")


def test_dialog_plans_the_restore(page, case, watch):
    """Restore… opens the dialog, the focus in the key's box and its
    button off without a key; a key that does not open the backup is
    refused with why; the key file read in the browser opens it, and the
    choice shows each scope with its counts, the fake Cube by its serial,
    nothing ticked, users and sign-in off where the server cannot restart
    itself, no identity for a backup of this box; the choice of the
    settings is planned and confirmed, naming what it replaces and the
    backup taken first; Back, then Escape closes the dialog, and the key
    is gone when it opens again, as at a sign-out."""
    watch.expected.append(("/restore/plan", 422))
    uuid, key_file = made_backup(case)
    wrong = other_key()
    restart = api_call("GET", "api/v1/restart") or {}
    plan = api_call("POST", f"{BACKUPS}/{uuid}/restore/plan", {"key_file": key_file})
    try:
        dialog = open_dialog(page, uuid)
        when = page.locator(f".sb-backup[data-uuid='{uuid}'] > summary .panel-name").inner_text()
        assert dialog.locator("h3").inner_text() == text(page, "serverbackup.restore.title", when=when), "named by the backup's time"
        assert focused(page) == "sb-restore-key", "the key's box takes the focus"
        assert page.locator("#sb-restore-open").is_disabled(), "no key, nothing to open"
        page.fill("#sb-restore-key", wrong)
        page.locator("#sb-restore-open").click()
        prefix = text(page, "serverbackup.restore.openFailed", error="")
        assert wait_until(page, f"document.querySelector('#sb-restore-msg').textContent.startsWith({prefix!r})", 15), \
            page.locator("#sb-restore-msg").inner_text()
        assert page.locator("#sb-restore-msg").inner_text() != prefix, "with the server's reason"

        page.set_input_files("#sb-restore-keyfile-input", files=[{"name": "restore-key.txt", "mimeType": "text/plain", "buffer": key_file.encode()}])
        line = page.locator(".sb-restore-keyfile")
        line.wait_for(timeout=5000)
        assert text(page, "serverbackup.restore.keyFrom", name="restore-key.txt") in line.inner_text(), "the file named"
        assert page.locator("#sb-restore-key").count() == 0, "its text not shown"
        page.locator("#sb-restore-open").click()
        page.locator(".sb-restore-choose").wait_for(timeout=15000)
        assert "sb-restore-h" in focused(page), "the choice's heading takes the focus"

        counts = {}
        for tb in (plan.get("report") or {}).get("tables") or []:
            c = counts.setdefault(tb["scope"], [0, 0])
            c[0] += tb.get("restored") or 0
            c[1] += (tb.get("removed") or 0) - (tb.get("kept_removed") or 0)
        for s in ("settings", "users", "integrations", "history"):
            box = page.locator(f"#sb-restore-scope-{s}")
            assert box.count() == 1 and not box.is_checked(), f"{s}: offered, not ticked"
            got = page.locator(f".sb-restore-scope:has(#sb-restore-scope-{s}) .sb-restore-line").inner_text()
            restored, removed = counts.get(s, [0, 0])
            want = page.evaluate("([k, r, d]) => t(k, {restored: fmtNum(r, 0), removed: fmtNum(d, 0)})", ["serverbackup.restore.count", restored, removed])
            if s == "history":
                # the audit grows with every request meanwhile, the plans' own among them
                assert got.split(" ")[0] == want.split(" ")[0], f"{s}: {got!r}, want {want!r}"
            else:
                assert got == want, f"{s}: {got!r}, want {want!r}"
        cube = page.locator(".sb-restore-scope:has(#sb-restore-scope-cube-KEQ0000100)")
        assert cube.locator("label.switch").inner_text().strip() == text(page, "serverbackup.restore.cube", name="Fake Cube", serial="KEQ0000100")
        assert cube.locator(".sb-restore-line").inner_text() == text(page, "serverbackup.restore.cubeLive"), "this server has it"
        if not restart.get("available"):
            assert page.locator("#sb-restore-no-restart").is_visible(), "the server cannot restart itself, and says so"
            assert page.locator("#sb-restore-scope-users").is_disabled(), "users and sign-in off without a restart"
        assert page.locator("#sb-restore-identity").count() == 0, "a backup of this box: no identity to take"
        named_buttons(page, "the restore's choice")
        assert page.locator("#sb-restore-next").is_disabled(), "nothing ticked, nothing to restore"

        tick(page, "sb-restore-scope-settings")
        assert page.locator("#sb-restore-scope-settings").is_checked()
        page.locator("#sb-restore-next").click()
        confirm = page.locator(".sb-restore-confirm")
        confirm.wait_for(timeout=15000)
        assert "sb-restore-h" in focused(page), "the confirmation's heading takes the focus"
        assert confirm.locator(".sb-restore-what li").all_inner_texts() == [text(page, "serverbackup.restore.scope.settings")], "what it replaces"
        assert text(page, "serverbackup.restore.backupFirst") in confirm.inner_text(), "a backup taken first"
        assert page.evaluate("document.documentElement.scrollWidth") <= case.width, "no horizontal scroll"
        assert dialog.evaluate("(d) => d.scrollWidth <= d.clientWidth"), "nothing wider than the dialog"
        confirm.locator(".form-cancel").click()
        page.locator(".sb-restore-choose").wait_for()
        assert page.locator("#sb-restore-scope-settings").is_checked(), "Back keeps the choice"

        page.keyboard.press("Escape")
        assert wait_until(page, f"!document.querySelector('{DIALOG}').open", 5), "Escape closes the dialog"
        assert focused(page) != "sb-restore-key"
        open_dialog(page, uuid)
        assert page.locator("#sb-restore-key").input_value() == "" and page.locator(".sb-restore-keyfile").count() == 0, "closed, the key is gone"
        page.fill("#sb-restore-key", key_file)
        page.evaluate("MassimoParts.signedOut()")
        assert wait_until(page, f"!document.querySelector('{DIALOG}')?.open", 5), "a sign-out closes the dialog"
        show_view(page, "tiles")
        open_dialog(page, uuid)
        assert page.locator("#sb-restore-key").input_value() == "", "after a sign-out, the key is gone"
        page.keyboard.press("Escape")
    finally:
        clean()
    show_view(page, "tiles")


def test_check_with_the_key(page, case, watch):
    """Check with the key… asks for the key, then gives the check's
    verdict: the backup opens, its store is whole, this box made it, the
    files it holds; a key that does not open it is refused with why."""
    watch.expected.append(("/check-key", 422))
    uuid, key_file = made_backup(case)
    wrong = other_key()
    try:
        dialog = open_dialog(page, uuid, ".sb-check-key")
        when = page.locator(f".sb-backup[data-uuid='{uuid}'] > summary .panel-name").inner_text()
        assert dialog.locator("h3").inner_text() == text(page, "serverbackup.checkKey.title", when=when), "named by the backup's time"
        page.fill("#sb-restore-key", wrong)
        page.locator("#sb-restore-open").click()
        prefix = text(page, "serverbackup.checkKey.failed", error="")
        assert wait_until(page, f"document.querySelector('#sb-restore-msg').textContent.startsWith({prefix!r})", 15), \
            page.locator("#sb-restore-msg").inner_text()
        page.fill("#sb-restore-key", key_file)
        page.locator("#sb-restore-open").click()
        verdict = page.locator(".sb-check-verdict")
        verdict.wait_for(timeout=15000)
        assert verdict.get_attribute("data-ok") == "true", "the store is whole"
        assert verdict.locator("h4").inner_text() == text(page, "serverbackup.checkKey.ok")
        assert "sb-restore-h" in focused(page), "the verdict takes the focus"
        assert text(page, "serverbackup.checkKey.thisBox") in verdict.inner_text(), "this box made it"
        assert "massimo.db" in verdict.locator(".sb-check-files").inner_text(), "the files it holds"
        named_buttons(page, "the check's verdict")
        verdict.locator("button.primary").click()
        assert wait_until(page, f"!document.querySelector('{DIALOG}').open", 5), "Close closes it"
        assert "sb-check-key" in focused(page), "the focus back on the button that opened it"
    finally:
        clean()
    show_view(page, "tiles")


def first_plan(uuid, **over):
    """A plan of every scope as the server answers it, of a backup of
    another box that still answers, of an older schema, with an env file
    this server cannot write and two TLS files it can; over replaces
    fields."""
    plan = {
        "backup": {"uuid": uuid, "time": "2026-11-20T03:30:00Z", "trigger": "scheduled", "label": "", "version": "v0.18.0", "schema": 40},
        "identity": {"this_box": False, "fingerprint": "SHA256:placeholder", "node": "node-of-the-other-box", "answers": True},
        "migrated": True, "scopes": ["settings", "users", "integrations", "cubes", "history"], "serials": [],
        "cubes": [{"serial": "KEQ0000100", "name": "Fake Cube", "live": True}, {"serial": "KEQ0000300", "name": "Attic", "live": False}],
        "env": True, "tls": 2, "files": [], "report": {"tables": [{"scope": "users", "table": "users", "restored": 3, "replaced": 1, "removed": 1}]},
        "env_refused": "serverbackup: this server cannot write /etc/massimo/env (permission denied)", "tls_refused": "",
        "restart": {"available": True, "by": "systemd", "blocked": ""}, "refused": "",
    }
    plan.update(over)
    return plan


def stub_plan(page, uuid, sent, **over):
    """The plans answered: every scope's as first_plan, a choice's with
    nothing refused; each body kept in sent."""
    def handle(route):
        body = route.request.post_data_json
        sent.append(body)
        p = first_plan(uuid, **over)
        if body.get("scopes"):
            p["scopes"] = body["scopes"]
        route.fulfill(status=200, content_type="application/json", body=json.dumps(p))
    safe_route(page, PLAN, handle)


def to_confirmation(page, uuid, scope):
    """The dialog opened, a key typed, the backup opened, scope ticked and Restore… pressed: the confirmation."""
    open_dialog(page, uuid)
    page.fill("#sb-restore-key", "AGE-SECRET-KEY-1PLACEHOLDER")
    page.locator("#sb-restore-open").click()
    page.locator(".sb-restore-choose").wait_for(timeout=10000)
    tick(page, scope)
    page.locator("#sb-restore-next").click()
    page.locator(".sb-restore-confirm").wait_for(timeout=10000)


def test_restore_refused_with_its_list(page, case, watch):
    """A backup of another box: its identity offered with the warning, and
    that the other box still answers; the env file off with why this
    server cannot write it, the TLS files offered; a new Cube beside the
    one it has. A restore of a Cube refused (409, missing) goes back to
    the choice with the refusal and the rows missing; the restore's body
    carried the key and the Cube by its serial, nothing else."""
    watch.expected.append(("/restore", 409))
    uuid, _ = made_backup(case)
    sent, posts = [], []
    stub_plan(page, uuid, sent)
    refusal = "store: the restore names rows this server lacks"

    def restore(route):
        posts.append(route.request.post_data_json)
        route.fulfill(status=409, content_type="application/json", body=json.dumps({
            "error": refusal, "code": "missing",
            "report": {"missing": [{"table": "users", "uuid": "u-1", "name": "olga", "by": "cube_users"}]}}))
    safe_route(page, RESTORE, restore)
    try:
        open_dialog(page, uuid)
        page.fill("#sb-restore-key", "AGE-SECRET-KEY-1PLACEHOLDER")
        page.locator("#sb-restore-open").click()
        page.locator(".sb-restore-choose").wait_for(timeout=10000)
        made = page.locator(".sb-restore-made").inner_text()
        assert text(page, "serverbackup.restore.otherBox") in made and text(page, "serverbackup.restore.migrated") in made, made
        assert page.locator("#sb-restore-no-restart").count() == 0, "the server can restart itself"
        assert page.locator("#sb-restore-identity").is_enabled(), "another box's identity offered"
        warn = page.locator(".sb-restore-scope:has(#sb-restore-identity) .warn-text").inner_text()
        assert warn == text(page, "serverbackup.restore.identityWarn"), "with its warning"
        assert page.locator("#sb-restore-answers").inner_text() == text(page, "serverbackup.restore.answers"), "that box still answers"
        assert page.locator("#sb-restore-env").is_disabled(), "the env file off"
        assert page.locator("#sb-restore-env-off").inner_text() == first_plan(uuid)["env_refused"], "with why"
        assert page.locator("#sb-restore-tls").is_enabled(), "the TLS files offered"
        tls = page.locator(".sb-restore-scope:has(#sb-restore-tls) label.switch").inner_text().strip()
        assert tls == text(page, "serverbackup.restore.tls", n=2), tls
        attic = page.locator(".sb-restore-scope:has(#sb-restore-scope-cube-KEQ0000300) .sb-restore-line").inner_text()
        assert attic == text(page, "serverbackup.restore.cubeNew"), "a Cube this server has not"
        users = page.locator(".sb-restore-scope:has(#sb-restore-scope-users) .sb-restore-line").inner_text()
        assert users == text(page, "serverbackup.restore.count", restored="3", removed="1"), users
        assert page.locator(".sb-restore-refused").count() == 0, "nothing refused"
        assert sent[0] == {"key_file": "AGE-SECRET-KEY-1PLACEHOLDER", "scopes": []}, f"the first plan, every scope: {sent}"

        tick(page, "sb-restore-scope-cube-KEQ0000100")
        page.locator("#sb-restore-next").click()
        confirm = page.locator(".sb-restore-confirm")
        confirm.wait_for(timeout=10000)
        assert sent[-1]["scopes"] == ["cube:KEQ0000100"], f"the choice planned: {sent[-1]}"
        assert confirm.locator(".sb-restore-what li").all_inner_texts() == [text(page, "serverbackup.restore.cube", name="Fake Cube", serial="KEQ0000100")]
        assert text(page, "serverbackup.restore.restarts") in confirm.inner_text(), "the server then restarts"
        named_buttons(page, "the restore's confirmation")
        page.locator("#sb-restore-run").click()
        refused = page.locator(".sb-restore-refused")
        refused.wait_for(timeout=10000)
        assert page.locator(".sb-restore-choose").count() == 1, "back to the choice"
        assert refused.locator("p.bad").inner_text() == text(page, "serverbackup.restore.refused", error=refusal), refused.inner_text()
        rows = refused.locator(".sb-restore-missing li").all_inner_texts()
        assert rows == [text(page, "serverbackup.restore.missingRow", name="olga", table="users", by="cube_users")], rows
        assert posts == [{"key_file": "AGE-SECRET-KEY-1PLACEHOLDER", "scopes": ["cube:KEQ0000100"], "take_identity": False, "env": False, "tls": False}], posts
        assert page.locator("#sb-restore-scope-cube-KEQ0000100").is_checked(), "the choice kept"
        tick(page, "sb-restore-scope-settings")
        assert page.locator(".sb-restore-refused").count() == 0, "another choice: the refusal goes"
        page.keyboard.press("Escape")
        assert wait_until(page, f"!document.querySelector('{DIALOG}').open", 5)
    finally:
        page.unroute(RESTORE)
        page.unroute(PLAN)
        clean()
    show_view(page, "tiles")


def test_restore_restarts_and_offers_the_cube_restore(page, case, watch):
    """A restore whose server restarts: the outcome lists what it
    restored, the secrets to enter again and the writes in neither
    backup; the page follows the server through /healthz (stubbed) and
    says when it is back; then the restored Cube is offered to the Cube
    restore, whose link closes the dialog and shows Cube settings at that
    Cube's Backup and restore, its heading with the focus."""
    uuid, _ = made_backup(case)
    sent, posts = [], []
    stub_plan(page, uuid, sent)
    health = api_call("GET", "healthz")
    later = dict(health, started_at="2099-01-02T03:04:05Z")
    calls = {"after": 0}

    def healthz(route):
        if posts:
            calls["after"] += 1
        # the first look after the restore still finds the old server
        route.fulfill(status=200, content_type="application/json", body=json.dumps(later if calls["after"] > 1 else health))

    def restore(route):
        posts.append(route.request.post_data_json)
        route.fulfill(status=200, content_type="application/json", body=json.dumps({
            "restart": {"started": True, "by": "systemd"}, "before_backup": "b-1", "before_backup_time": "2026-11-27T10:00:00Z",
            "restored_at": "2026-11-27T10:00:05Z", "scopes": ["settings", "cubes"], "serials": ["KEQ0000100"],
            "report": {"secrets_lost": 2}, "files": [],
            "cubes": [{"serial": "KEQ0000100", "name": "Fake Cube", "live": True, "backup": "7"}]}))
    safe_route(page, "**/healthz", healthz)
    safe_route(page, RESTORE, restore)
    try:
        open_dialog(page, uuid)
        page.fill("#sb-restore-key", "AGE-SECRET-KEY-1PLACEHOLDER")
        page.locator("#sb-restore-open").click()
        page.locator(".sb-restore-choose").wait_for(timeout=10000)
        tick(page, "sb-restore-scope-settings")
        tick(page, "sb-restore-scope-cube-KEQ0000100")
        page.locator("#sb-restore-next").click()
        page.locator(".sb-restore-confirm").wait_for(timeout=10000)
        page.locator("#sb-restore-run").click()
        done = page.locator(".sb-restore-done")
        done.wait_for(timeout=10000)
        assert "sb-restore-h" in focused(page), "the outcome's heading takes the focus"
        assert posts and posts[0]["scopes"] == ["settings", "cube:KEQ0000100"], posts
        assert done.locator(".sb-restore-what li").all_inner_texts() == [
            text(page, "serverbackup.restore.scope.settings"), text(page, "serverbackup.restore.cube", name="Fake Cube", serial="KEQ0000100")]
        assert page.evaluate("tn('serverbackup.restore.secretsLost', 2)") in done.inner_text(), "the secrets to enter again"
        assert done.locator(".sb-restore-gap").is_visible(), "the writes in neither"
        restarting = text(page, "restart.restarting")
        back = page.evaluate("t('restart.back', {time: fmtInstant(new Date(Date.parse('2099-01-02T03:04:05Z')))})")
        assert wait_until(page, f"['{restarting}', {json.dumps(back)}].includes(document.querySelector('#sb-restore-restart')?.textContent)", 5), \
            "the page follows the restart"
        assert wait_until(page, f"document.querySelector('#sb-restore-restart').textContent === {json.dumps(back)}", 15), \
            "the server back, and since when"
        assert calls["after"] >= 2, calls
        link = page.locator(".sb-cube-restore[data-serial='KEQ0000100']")
        link.wait_for(timeout=5000)
        want = text(page, "serverbackup.restore.cubeLink", cube=text(page, "serverbackup.restore.cube", name="Fake Cube", serial="KEQ0000100"))
        assert link.inner_text() == want, link.inner_text()
        named_buttons(page, "the restore's outcome")
        assert is_open(page), "the dialog stays until it is closed"
        link.click()
        assert wait_until(page, f"!document.querySelector('{DIALOG}').open", 5), "the link closes the dialog"
        assert wait_until(page, "MassimoParts.pages.shown() === 'cube' && state.serial === 'KEQ0000100'", 5), "Cube settings, that Cube"
        assert wait_until(page, "document.activeElement === document.querySelector('#cube-backup .section-fold')", 5), \
            f"its Backup and restore's heading has the focus: {focused(page)}"
    finally:
        page.unroute(RESTORE)
        page.unroute("**/healthz")
        page.unroute(PLAN)
        clean()
    show_view(page, "tiles")
