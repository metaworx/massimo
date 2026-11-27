# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Maintenance → Server backups, the targets' pieces (AP ServerBackup,
Blocks 6 and 7), against the server run.sh starts, which offers every
kind of target: an SFTP target's key line, which copies itself, and the
host's key its test stops at, trusted with a button where none is and
shown beside the one trusted, with no button, where it changed (the
test's answer stubbed: no SSH server runs here). As
test_server_backups.py, each test starts from nothing and takes away
what it made (clean)."""
import json

from conftest import api_call
from helpers import named_buttons, safe_route, show_view, wait_until
from test_server_backups import CARD, TARGETS, clean, shown_maint, text

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
