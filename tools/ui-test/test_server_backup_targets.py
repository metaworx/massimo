# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Maintenance → Server backups, the targets' pieces (AP ServerBackup,
Blocks 6 and 7) and Delete of a backup no target holds (Block 4),
against the server run.sh starts, which offers every kind of target:
an SFTP target's key line, which copies itself, and the host's key its
test stops at, trusted with a button where none is and shown beside the
one trusted, with no button, where it changed (the test's answer
stubbed: no SSH server runs here); a WebDAV target's password asked for
again when its URL changes, and plain http warned of; a backup whose
copy is missing deleted, after a question, and a failed one's Delete
refused with the question kept. As test_server_backups.py,
each test starts from nothing and takes away what it made (clean)."""
import json
import os

from conftest import api_call
from helpers import named_buttons, safe_route, show_view, wait_until
from test_server_backups import BACKUPS, CARD, TARGETS, backup_ids, clean, data_dir, key_set, shown_maint, text

# The fingerprints a stubbed test answers: the host's key, and another one trusted before.
HOST_KEY = "SHA256:" + "A" * 43
OTHER_KEY = "SHA256:" + "B" * 43


def sftp_target(name):
    """An SFTP target added through the API, its host a loopback port
    where nothing listens, so that the add's read fails at once; the
    server makes its key. The target, with its public_key."""
    config = {"host": "127.0.0.1", "port": "1", "user": "backup", "path": "massimo"}
    return api_call("POST", TARGETS, {"name": name, "kind": "sftp", "config": config})["target"]


def target_now(uuid):
    """The target uuid as the server lists it now."""
    return next(tg for tg in api_call("GET", TARGETS) if tg["uuid"] == uuid)


def msg_is(page, uuid, want):
    """Whether the line of the target uuid's form says want, within 10 s."""
    return wait_until(page, f"document.querySelector(\".sb-target[data-uuid='{uuid}'] .sb-target-msg\")?.textContent === {want!r}", 10)


def test_sftp_key_and_host_key(page, case):
    """An SFTP target: the new form says that the server makes its key,
    and asks for no secret; its panel shows the key line the server made,
    which copies itself, and where it goes. A test that stops at a host's
    key not trusted yet shows its fingerprint and Trust this host key,
    which saves it as the target's host_key, its other fields as stored,
    the form following; one that stops at another key than the one
    trusted shows both, in red, and no button; Maintenance shown again
    takes either away."""
    clean()
    name = f"Nas {case.width} {case.lang}"
    target = sftp_target(name)
    uuid, key = target["uuid"], target["public_key"]
    assert key.startswith("ssh-ed25519 AAAA"), target
    answers = []
    route = safe_route(page, f"**/api/v1/server-backup-targets/{uuid}/test",
                       lambda r: r.fulfill(status=200, content_type="application/json", body=json.dumps(answers[-1])))
    try:
        show_view(page, "maint")
        page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
        page.locator(CARD).scroll_into_view_if_needed()
        page.locator("#sb-target-add").click()
        assert wait_until(page, "[...document.querySelectorAll('#sb-target-new-kind option')].some((o) => o.value === 'sftp')", 5), "the server offers sftp"
        page.select_option("#sb-target-new-kind", "sftp")
        assert page.locator("#sb-target-new-kind-note").inner_text() == text(page, "serverbackup.kindNote.sftp"), "the kind's note"
        assert "sb-target-new-kind-note" in (page.locator("#sb-target-new-kind").get_attribute("aria-describedby") or ""), "describing the choice"
        assert page.locator("#sb-target-new-secret").count() == 0, "no secret: the server makes its key"
        page.select_option("#sb-target-new-kind", "local")
        assert page.locator("#sb-target-new-kind-note").count() == 0, "a kind without a note has none"
        page.locator("#sb-target-new .form-cancel").click()

        panel = page.locator(f".sb-target[data-uuid='{uuid}']")
        panel.wait_for(timeout=10000)
        panel.locator(":scope > summary").click()
        line = panel.locator(f"#sb-target-{uuid}-public-key")
        assert line.inner_text() == key, "the key line the server made"
        assert line.get_attribute("aria-label") == text(page, "serverbackup.target.copyPublicKey", name=name), "named by what it copies"
        assert panel.locator(".sb-public-key-hint").inner_text() == text(page, "serverbackup.target.publicKeyHint"), "and where it goes"
        page.evaluate("navigator.clipboard.writeText('')")
        panel.locator(".sb-public-key .copy-icon").click()
        assert wait_until(page, f"navigator.clipboard.readText().then((t) => t === {key!r})", 3), "the line copies"

        answers.append({"ok": False, "error": "the host's key is not confirmed", "host_key": HOST_KEY})
        panel.locator(".sb-target-test").click()
        host = panel.locator(f"#sb-target-{uuid}-host")
        host.wait_for(timeout=10000)
        assert host.get_attribute("data-state") == "new", "a key to trust"
        assert host.locator(".sb-host-key-shown").inner_text() == HOST_KEY, "with its fingerprint"
        assert host.locator(".warn-text").inner_text().strip() == text(page, "serverbackup.target.hostKeyNew")
        named_buttons(page, "a host's key to trust")
        assert page.evaluate("document.documentElement.scrollWidth") <= case.width, "no horizontal scroll"
        host.locator(".sb-host-key-trust").click()
        assert msg_is(page, uuid, text(page, "serverbackup.target.hostKeyTrusted", name=name)), "it says so"
        assert host.count() == 0, "trusted: the host's key goes from the panel"
        now = target_now(uuid)
        assert now["config"] == dict(target["config"], host_key=HOST_KEY), f"host_key set, the rest as stored: {now['config']}"
        assert now["public_key"] == key, "its own key stays"
        assert wait_until(page, f"document.querySelector('#sb-target-{uuid}-field-host_key').value === {HOST_KEY!r}", 5), "the form follows"

        answers.append({"ok": False, "error": "the host's key is not the one confirmed: refused", "host_key": OTHER_KEY, "pinned_host_key": HOST_KEY})
        panel.locator(".sb-target-test").click()
        host.wait_for(timeout=10000)
        assert host.get_attribute("data-state") == "changed", "a changed key"
        assert host.locator(".sb-host-key-pinned").inner_text() == HOST_KEY and host.locator(".sb-host-key-shown").inner_text() == OTHER_KEY, \
            "both fingerprints"
        assert host.locator(".sb-host-key-bad").inner_text().strip() == text(page, "serverbackup.target.hostKeyChanged")
        assert host.locator("button").count() == 0, "and no button"
        assert page.evaluate("document.documentElement.scrollWidth") <= case.width, "no horizontal scroll"
        shown_maint(page)
        assert panel.locator(f"#sb-target-{uuid}-host").count() == 0, "Maintenance shown again: gone"
    finally:
        page.unroute(f"**/api/v1/server-backup-targets/{uuid}/test", route)
        clean()
    show_view(page, "tiles")


def test_secret_asked_again_and_http_warned(page, case):
    """A WebDAV target's password (Block 6): kept while only the target's
    name changes; asked for again as soon as its URL changes, the field
    required and a note under it saying why, since the server takes a
    change to where the password goes only with it, so that Save sends
    nothing without it; with it, both are saved, and the URL back as it
    was asks no more. Plain http in the URL is warned of (Block 4's warn),
    in the new target's form as under the target's own field."""
    clean()
    name = f"Dav {case.width} {case.lang}"
    target = api_call("POST", TARGETS, {"name": name, "kind": "webdav", "config": {"url": "http://127.0.0.1:1/dav", "user": "backup"},
                                        "secret": "first"})["target"]
    uuid = target["uuid"]
    puts = []
    page.on("request", lambda r: puts.append(r.post_data_json) if r.method == "PUT" and r.url.endswith("/" + uuid) else None)
    try:
        show_view(page, "maint")
        page.locator(CARD).scroll_into_view_if_needed()
        page.locator("#sb-target-add").click()
        assert wait_until(page, "[...document.querySelectorAll('#sb-target-new-kind option')].some((o) => o.value === 'webdav')", 5), "the server offers webdav"
        page.select_option("#sb-target-new-kind", "webdav")
        url = page.locator("#sb-target-new-field-url")
        url.fill("http://nas.example/remote.php/dav/files/backup/")
        note = page.locator("#sb-target-new-field-url-warn")
        note.wait_for(timeout=5000)
        assert note.inner_text().strip() == text(page, "serverbackup.warn.webdav.url"), "plain http warned of, in WebDAV's own words"
        url.fill("https://nas.example/remote.php/dav/files/backup/")
        assert wait_until(page, "!document.querySelector('#sb-target-new-field-url-warn')", 5), "https: no note"
        page.locator("#sb-target-new .form-cancel").click()

        panel = page.locator(f".sb-target[data-uuid='{uuid}']")
        panel.wait_for(timeout=10000)
        panel.locator(":scope > summary").click()
        own = panel.locator(f"#sb-target-{uuid}-field-url")
        assert panel.locator(f"#sb-target-{uuid}-field-url-warn").inner_text().strip() == text(page, "serverbackup.warn.webdav.url"), \
            "the stored URL warned of too"
        secret = panel.locator(f"#sb-target-{uuid}-secret")
        again = panel.locator(f"#sb-target-{uuid}-secret-again")
        save = panel.locator("button[type=submit]")
        assert not secret.evaluate("(i) => i.required") and again.count() == 0, "kept: not asked for"
        panel.locator(f"#sb-target-{uuid}-name").fill(name + " 2")
        assert again.count() == 0 and not secret.evaluate("(i) => i.required"), "another field than the kind's asks for nothing"
        panel.locator(f"#sb-target-{uuid}-name").fill(name)

        own.fill("https://127.0.0.1:1/dav")
        again.wait_for(timeout=5000)
        assert again.inner_text().strip() == text(page, "serverbackup.target.secretAgain"), "asked for again, saying why"
        assert secret.evaluate("(i) => i.required") and secret.input_value() == "", "required, and empty"
        assert f"sb-target-{uuid}-secret-again" in (secret.get_attribute("aria-describedby") or "").split(), "the note describes it"
        named_buttons(page, "a target's form asking for its secret again")
        assert page.evaluate("document.documentElement.scrollWidth") <= case.width, "no horizontal scroll"
        save.click()
        assert secret.evaluate("(i) => i.validity.valueMissing"), "Save stops at it"
        page.wait_for_timeout(300)
        assert not puts, f"nothing sent without it: {puts}"
        secret.fill("second")
        save.click()
        assert msg_is(page, uuid, text(page, "serverbackup.target.saved", name=name)), "saved with it"
        assert puts and puts[-1].get("secret") == "second" and puts[-1]["config"]["url"] == "https://127.0.0.1:1/dav", f"both sent: {puts}"
        now = target_now(uuid)
        assert now["config"]["url"] == "https://127.0.0.1:1/dav" and now["has_secret"], now
        assert wait_until(page, f"!document.querySelector('#sb-target-{uuid}-secret-again')", 5), "saved: asked for no more"

        own.fill("http://127.0.0.1:2/dav")
        again.wait_for(timeout=5000)
        own.fill("https://127.0.0.1:1/dav")
        assert wait_until(page, f"!document.querySelector('#sb-target-{uuid}-secret-again')", 5), "the URL back as stored: not asked for"
        assert not secret.evaluate("(i) => i.required")
    finally:
        clean()
    show_view(page, "tiles")


def test_backup_without_copy_deleted(page, case):
    """A backup no target holds a copy of (Block 4): its file taken from
    its target's directory and a check marking its copy missing, its fold
    offers Delete, which asks first, Cancel keeping it; Yes takes it from
    the list and from the server, saying so under Back up now. A backup
    with a copy there offers none."""
    clean()
    key_set()
    where = data_dir(f"sb-gone-{case.width}-{case.lang}")
    # the last ten kept, so that the second backup prunes nothing of the first
    target = api_call("POST", TARGETS, {"name": "Gone", "kind": "local", "config": {"path": where}, "retention": {"last": 10}})["target"]
    gone = api_call("POST", BACKUPS, {"label": "gone"})["backup"]
    kept = api_call("POST", BACKUPS, {"label": "kept"})["backup"]
    copy = next(c for c in gone["copies"] if c["target"] == target["uuid"])
    os.remove(os.path.join(where, copy["path"]))
    api_call("POST", f"{BACKUPS}/{gone['uuid']}/check")
    states = [c["state"] for b in api_call("GET", BACKUPS)["backups"] if b["uuid"] == gone["uuid"] for c in b["copies"]]
    assert states == ["missing"], f"its copy missing: {states}"
    try:
        show_view(page, "maint")
        fold = page.locator(f".sb-backup[data-uuid='{gone['uuid']}']")
        fold.wait_for(timeout=10000)
        fold.locator(":scope > summary").click()
        other = page.locator(f".sb-backup[data-uuid='{kept['uuid']}']")
        other.locator(":scope > summary").click()
        assert other.locator(".sb-backup-delete").count() == 0, "a backup with a copy there: no Delete"
        delete = fold.locator(".sb-backup-delete")
        assert delete.inner_text() == text(page, "action.delete"), "Delete"
        named_buttons(page, "a backup no target holds")
        delete.click()
        when = fold.locator(":scope > summary .panel-name").inner_text()
        assert fold.locator(".confirm-text").inner_text() == text(page, "serverbackup.deleteBackupAsk", when=when), "it asks first"
        assert page.evaluate("document.documentElement.scrollWidth") <= case.width, "no horizontal scroll"
        fold.locator(".confirm-no").click()
        assert gone["uuid"] in backup_ids(), "Cancel keeps it"
        assert page.evaluate("document.activeElement.classList.contains('sb-backup-delete')"), "the focus back on Delete"
        delete.click()
        fold.locator(".confirm-yes").click()
        assert wait_until(page, f"!document.querySelector(\".sb-backup[data-uuid='{gone['uuid']}']\")", 10), "gone from the list"
        assert gone["uuid"] not in backup_ids() and kept["uuid"] in backup_ids(), "and from the server, the other kept"
        assert page.locator("#sb-msg").inner_text() == text(page, "serverbackup.deletedLast", when=when), "it says so"
    finally:
        clean()
    show_view(page, "tiles")


# A failed backup the schedule made, which the list's answer is given: one made by hand that fails is not kept.
FAILED = {"uuid": "01a21e2c-0000-7000-8000-0000000000f1", "time": "2026-11-27T02:30:00Z", "trigger": "scheduled", "state": "failed",
          "error": "serverbackup: no target took the backup", "tries": 3, "copies": []}


def test_failed_backup_offers_delete(page, case, watch):
    """A failed backup the schedule made (added to the list's answer) offers
    Retry and Delete; Delete asks, and where the server refuses (the
    backup is not one it has: 404, on purpose) the question stays and the
    backup's line says why."""
    watch.expected.append((FAILED["uuid"], 404))
    clean()

    def with_failed(route):
        if route.request.method != "GET":
            route.fallback()
            return
        answer = route.fetch()
        body = answer.json()
        body["backups"] = [FAILED] + (body.get("backups") or [])
        route.fulfill(response=answer, body=json.dumps(body))

    handler = safe_route(page, "**/api/v1/server-backups", with_failed)
    try:
        show_view(page, "maint")
        fold = page.locator(f".sb-backup[data-uuid='{FAILED['uuid']}']")
        fold.wait_for(timeout=10000)
        fold.locator(":scope > summary").click()
        assert fold.locator(".sb-retry").count() == 1 and fold.locator(".sb-backup-delete").count() == 1, "Retry and Delete"
        fold.locator(".sb-backup-delete").click()
        fold.locator(".confirm-yes").click()
        line = f".sb-backup[data-uuid='{FAILED['uuid']}'] .sb-backup-msg"
        assert wait_until(page, f"document.querySelector(\"{line}\")?.classList.contains('bad')", 10), "refused, under the backup"
        assert page.locator(line).inner_text().startswith(text(page, "serverbackup.failedAction", error="")), page.locator(line).inner_text()
        assert fold.locator(".confirm-box").count() == 1, "the question stays"
        fold.locator(".confirm-no").click()
    finally:
        page.unroute("**/api/v1/server-backups", handler)
        clean()
    show_view(page, "tiles")
