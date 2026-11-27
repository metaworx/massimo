# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Maintenance → Server backups (AP ServerBackup, Block 4), against the
server run.sh starts, whose store holds no key, no target and no
server backup at first: without a key the card says that backups are
off and Back up now is off; a post-quantum key made shows its secret
part once, as text and as a file, a wrong last six refused and the right
ones confirming it; a classic key pasted beside it is refused with why;
a key made's secret part gone with another page or the card folded,
and a confirmation after its hour; a target added from the kinds' form
and tested, a backup made onto it and checked; the form of stubbed
kinds: a field's note while its value matches the warn its kind gives
it, a switch's default, a secret that goes with another kind; a target
of a kind the page does not know keeping its fields; Delete of the one
copy there; a copy that failed, retried by a user with backup.retry
alone, who sees the state and Retry and nothing else. The four cases
share the server, so each test starts from nothing and takes away what
it made (clean): the backups' copies first, each backup going with its
last, then the targets and the keys.

A target's directory is an absolute path in the set's data directory,
beside its serve.log: run.sh starts the server in it, so that the store
knows it as "." and a relative path would be taken as it is."""
import json
import os
import urllib.error
from pathlib import Path

from conftest import api_call
from helpers import VIEWER, named_buttons, safe_route, show_view, wait_until

CARD = "#sys-server-backups"
BODY = "#sys-server-backups-body"
KEYS = "api/v1/server-backup-keys"
TARGETS = "api/v1/server-backup-targets"
BACKUPS = "api/v1/server-backups"


def data_dir(*parts):
    """A path in the set's data directory, where its massimo.db is."""
    return os.path.join(os.path.dirname(os.environ["UI_TEST_SERVE_LOG"]), *parts)


def backup_ids():
    """The uuids of the server backups the server lists."""
    return {b["uuid"] for b in (api_call("GET", BACKUPS) or {}).get("backups") or []}


def clean():
    """No server backup, no target and no key on the server: every copy
    deleted, its file with it, so that a backup goes with its last, then
    the targets, then the keys."""
    for b in (api_call("GET", BACKUPS) or {}).get("backups") or []:
        for c in b.get("copies") or []:
            try:
                api_call("DELETE", f"{BACKUPS}/{b['uuid']}/copies/{c['target']}")
            except urllib.error.HTTPError:
                pass  # gone with the target meanwhile
    for tg in api_call("GET", TARGETS) or []:
        api_call("DELETE", f"{TARGETS}/{tg['uuid']}")
    for k in api_call("GET", KEYS) or []:
        api_call("DELETE", f"{KEYS}/{k['uuid']}")


def key_set():
    """A classic key made and confirmed through the API, as the owner."""
    made = api_call("POST", KEYS, {"label": "test", "make": "classic"})
    api_call("POST", f"{KEYS}/{made['key']['uuid']}/confirm", {"last_six": made["identity"][-6:]})


def text(page, key, **args):
    """A text of the page's language, with its arguments."""
    return page.evaluate("([k, a]) => t(k, a)", [key, args])


def shown_maint(page):
    """Maintenance shown again, so that the card reads what the API changed."""
    show_view(page, "tiles")
    show_view(page, "maint")
    page.locator(CARD).scroll_into_view_if_needed()


def test_no_key(page, case):
    """No key: the card says that server backups are off until one is set,
    and to the owner where; Back up now is off; the card's head says off."""
    clean()
    show_view(page, "maint")
    card = page.locator(CARD)
    card.scroll_into_view_if_needed()
    assert card.is_visible(), "the card on Maintenance"
    assert card.locator(".section-title").inner_text() == text(page, "serverbackup.title"), "by its title"
    note = page.locator("#sb-no-key")
    note.wait_for(state="visible", timeout=10000)
    assert note.inner_text() == text(page, "serverbackup.noKey") + " " + text(page, "serverbackup.noKeyOwner"), "off until a key is set"
    assert page.locator("#sb-now").is_disabled(), "Back up now is off"
    assert wait_until(page, f"document.querySelector('{CARD} .section-badges').innerText.includes({text(page, 'serverbackup.badge.off')!r})", 5), \
        "the card's head says off"
    assert page.locator("#sb-schedule").count() == 0, "no schedule while off"
    named_buttons(page, "Server backups")
    show_view(page, "tiles")


def test_key_made_confirmed_and_other_kind_refused(page, case, watch):
    """A post-quantum key made: its secret part shown once, as text and as
    a file to save, the key pending until its last six are typed back, a
    wrong six refused; confirmed, the secret part goes and the schedule
    shows; a classic key pasted beside it is refused with why, and not
    added. The server's refusals, a 422 and a 409, are on purpose."""
    watch.expected += [("/confirm", 422), ("/server-backup-keys", 409)]
    clean()
    classic = api_call("POST", KEYS, {"label": "tmp", "make": "classic"})
    api_call("DELETE", f"{KEYS}/{classic['key']['uuid']}")
    recipient = classic["key"]["recipient"]
    try:
        show_view(page, "maint")
        page.locator("#sb-no-key").wait_for(state="visible", timeout=10000)
        page.locator("#sb-key-add").click()
        assert page.evaluate("document.activeElement.id") == "sb-key-label", "the form takes the focus in its label"
        page.fill("#sb-key-label", "safe")
        assert page.locator("#sb-key-way").input_value() == "pq", "post-quantum first"
        assert page.locator("#sb-key-way-line").inner_text() == text(page, "serverbackup.key.line.pq"), "with its line"
        page.select_option("#sb-key-way", "classic")
        assert page.locator("#sb-key-way-line").inner_text() == text(page, "serverbackup.key.line.classic"), "classic second, with its line"
        page.select_option("#sb-key-way", "pq")
        page.locator("#sb-key-new button[type=submit]").click()

        assert wait_until(page, "document.querySelectorAll('.sb-key .sb-identity').length === 1", 10), "the secret part shown"
        keys = api_call("GET", KEYS)
        assert len(keys) == 1 and keys[0]["pending"] and keys[0]["kind"] == "pq", f"one key, pending: {keys}"
        uuid = keys[0]["uuid"]
        row = page.locator(f".sb-key[data-uuid='{uuid}']")
        identity = row.locator(".sb-identity").inner_text()
        assert identity.startswith("AGE-SECRET-KEY-PQ-1"), "a post-quantum identity"
        assert row.locator(".flag.warn").inner_text().endswith(text(page, "serverbackup.key.pendingBadge")), "marked not confirmed"
        with page.expect_download() as saved:
            row.locator(".sb-key-save").click()
        assert saved.value.suggested_filename == "massimo-backup-key-safe.txt", saved.value.suggested_filename
        assert identity in Path(saved.value.path()).read_text(encoding="utf-8"), "the key file holds it"
        named_buttons(page, "a key made")

        six = page.locator(f"#sb-key-{uuid}-six")
        confirm = row.locator(".sb-key-confirm-btn")
        assert confirm.is_disabled(), "Confirm is off until six are typed"
        six.fill("ZZZZZZ" if not identity.endswith("ZZZZZZ") else "YYYYYY")
        confirm.click()
        msg = page.locator("#sb-keys-msg")
        refused = text(page, "serverbackup.key.refused", error=text(page, "serverbackup.key.wrongSix"))
        assert wait_until(page, f"document.querySelector('#sb-keys-msg').textContent === {refused!r}", 10), msg.inner_text()
        assert api_call("GET", KEYS)[0]["pending"], "still pending"
        assert row.locator(".sb-identity").count() == 1, "the secret part still shown"
        six.fill(identity.lower())  # the whole key pasted gives its last six
        confirm.click()
        assert wait_until(page, f"!document.querySelector(\".sb-key[data-uuid='{uuid}'] .sb-identity\")", 10), "the secret part gone"
        assert not api_call("GET", KEYS)[0]["pending"], "confirmed"
        assert msg.inner_text() == text(page, "serverbackup.key.confirmed", label="safe")
        assert row.locator(".flag.warn").count() == 0, "no longer marked"
        page.locator("#sb-schedule").wait_for(state="visible", timeout=10000)
        assert page.locator("#sb-no-key").count() == 0 and page.locator("#sb-now").is_enabled(), "backups on"

        page.locator("#sb-key-add").click()
        page.fill("#sb-key-label", "laptop")
        page.select_option("#sb-key-way", "paste")
        page.fill("#sb-key-recipient", recipient)
        page.locator("#sb-key-new button[type=submit]").click()
        mixed = text(page, "serverbackup.key.refused", error=text(page, "serverbackup.key.mixed", kinds=text(page, "serverbackup.key.group.pq")))
        assert wait_until(page, f"document.querySelector('#sb-keys-msg').textContent === {mixed!r}", 10), msg.inner_text()
        assert len(api_call("GET", KEYS)) == 1, "not added"
        assert page.locator("#sb-key-new form").is_visible(), "the form stays, with what was typed"
        page.locator("#sb-key-new .form-cancel").click()
    finally:
        clean()
    show_view(page, "tiles")


def make_key(page, label):
    """A classic key made through the card, its secret part shown; its uuid."""
    page.locator("#sb-key-add").click()
    page.fill("#sb-key-label", label)
    page.select_option("#sb-key-way", "classic")
    page.locator("#sb-key-new button[type=submit]").click()
    assert wait_until(page, "document.querySelectorAll('.sb-key .sb-identity').length === 1", 10), "the secret part shown"
    return next(k["uuid"] for k in api_call("GET", KEYS) if k["label"] == label)


def test_made_key_shown_once(page, case, watch):
    """A key made's secret part is shown that once: another page shown, or
    the card folded, takes it away, the key still waiting for its
    confirmation; a confirmation the server no longer takes (404, its hour
    over) reads the keys again and says why."""
    watch.expected.append(("/confirm", 404))
    clean()
    try:
        show_view(page, "maint")
        page.locator("#sb-no-key").wait_for(state="visible", timeout=10000)
        uuid = make_key(page, "away")
        show_view(page, "tiles")
        show_view(page, "maint")
        row = page.locator(f".sb-key[data-uuid='{uuid}']")
        row.wait_for(timeout=10000)
        assert row.locator(".sb-identity").count() == 0, "another page shown: the secret part gone"
        assert row.locator(".sb-key-confirm-btn").count() == 1, "still waiting for its confirmation"
        api_call("DELETE", f"{KEYS}/{uuid}")

        uuid = make_key(page, "folded")
        fold = page.locator(f"{CARD} .section-fold")
        fold.click()
        fold.click()
        row = page.locator(f".sb-key[data-uuid='{uuid}']")
        assert row.locator(".sb-identity").count() == 0, "the card folded: the secret part gone"

        reads = []
        page.on("request", lambda r: reads.append(r.method) if r.url.endswith("/api/v1/server-backup-keys") else None)
        safe_route(page, "**/api/v1/server-backup-keys/*/confirm",
                   lambda route: route.fulfill(status=404, content_type="application/json", body='{"error": "no server backup key"}'))
        row.locator(f"#sb-key-{uuid}-six").fill("ABCDEF")
        row.locator(".sb-key-confirm-btn").click()
        expired = text(page, "serverbackup.key.refused", error=text(page, "serverbackup.key.expired"))
        assert wait_until(page, f"document.querySelector('#sb-keys-msg').textContent === {expired!r}", 10), page.locator("#sb-keys-msg").inner_text()
        assert "GET" in reads, f"the keys read again: {reads}"
        page.unroute("**/api/v1/server-backup-keys/*/confirm")
    finally:
        clean()
    show_view(page, "tiles")


def test_target_added_tested_and_backup_made(page, case):
    """A target added through the kinds' form: the kind first, its field a
    directory of this box; tested; Back up now makes a backup whose copy
    is there on it, and its check finds every copy whole."""
    clean()
    key_set()
    name = f"Place {case.width} {case.lang}"
    where = data_dir(f"sb-test-{case.width}-{case.lang}")
    try:
        show_view(page, "maint")
        page.locator("#sb-schedule").wait_for(state="visible", timeout=10000)
        page.locator("#sb-target-add").click()
        assert page.evaluate("document.activeElement.id") == "sb-target-new-kind", "the form takes the focus in the kind"
        assert wait_until(page, "document.querySelector('#sb-target-new-kind').value === 'local'", 5), "the kind the server knows"
        path = page.locator("#sb-target-new-field-path")
        assert path.get_attribute("placeholder") == "backups", "its field from the kinds, its default as placeholder"
        page.fill("#sb-target-new-name", name)
        path.fill(where)
        named_buttons(page, "the new target's form")
        page.locator("#sb-target-new button[type=submit]").click()
        assert wait_until(page, "document.querySelectorAll('.sb-target').length >= 1", 10), "its panel"
        target = next(tg for tg in api_call("GET", TARGETS) if tg["name"] == name)
        assert target["config"]["path"] == where, target
        panel = page.locator(f".sb-target[data-uuid='{target['uuid']}']")
        assert panel.get_attribute("open") is not None, "added, its panel is open"
        assert page.locator("#sb-targets-msg").inner_text() == text(page, "serverbackup.target.added", name=name)
        panel.locator(".sb-target-test").click()
        works = text(page, "serverbackup.target.works", name=name)
        assert wait_until(page, f"document.querySelector(\".sb-target[data-uuid='{target['uuid']}'] .sb-target-msg\").textContent === {works!r}", 10), \
            "the test says it works"

        before = backup_ids()
        page.locator("#sb-now").click()
        assert wait_until(page, "document.querySelector('#sb-msg').classList.contains('ok')", 30), page.locator("#sb-msg").inner_text()
        made = [b for b in (api_call("GET", BACKUPS) or {}).get("backups") or [] if b["uuid"] not in before]
        assert len(made) == 1 and made[0]["trigger"] == "manual", made
        fold = page.locator(f".sb-backup[data-uuid='{made[0]['uuid']}']")
        fold.wait_for(timeout=10000)
        assert fold.locator(".sb-trigger").inner_text() == text(page, "serverbackup.trigger.manual"), "why it was made"
        fold.locator(":scope > summary").click()
        copy = fold.locator(f".sb-copy[data-target='{target['uuid']}']")
        assert copy.get_attribute("data-state") == "there", "its copy is there"
        named_buttons(page, "a backup's fold")
        fold.locator(".sb-check").click()
        checked = text(page, "serverbackup.checked")
        assert wait_until(page, f"document.querySelector(\".sb-backup[data-uuid='{made[0]['uuid']}'] .sb-backup-msg\").textContent === {checked!r}", 30), \
            "every copy whole"
        assert page.evaluate("document.documentElement.scrollWidth") <= case.width, "no horizontal scroll"
    finally:
        clean()
    show_view(page, "tiles")


def stub_kinds(page, kinds):
    """The kinds of target answered with kinds, before Maintenance first reads them."""
    safe_route(page, "**/api/v1/server-backup-targets/kinds",
               lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(kinds)))


def test_kind_form(page, case):
    """The new target's form from a stubbed answer of the kinds: the local
    kind's path warns of /warned/ (as WebDAV's url warns of plain http), a
    second kind has a secret and a switch on by default. While the value
    matches, the note stands under it, amber with its mark, a general one
    where the kind has no text of its own, and describes the input; Add
    stays on; another value takes it away. The switch shows its default; a
    secret typed goes with another kind, and is not sent for a kind
    without one."""
    kinds = [{"name": "local", "fields": [{"key": "path", "type": "path", "required": False, "default": "backups", "warn": "^/warned/"}]},
             {"name": "vault", "secret": "password", "fields": [{"key": "password", "type": "secret", "required": False},
                                                                  {"key": "verify", "type": "bool", "required": False, "default": "true"}]}]
    stub_kinds(page, kinds)
    clean()
    sent = []
    page.on("request", lambda r: sent.append(r.post_data_json) if r.method == "POST" and r.url.endswith("/api/v1/server-backup-targets") else None)
    try:
        show_view(page, "maint")
        page.locator(CARD).scroll_into_view_if_needed()
        page.locator("#sb-target-add").click()
        path = page.locator("#sb-target-new-field-path")
        path.wait_for()
        note = page.locator("#sb-target-new-field-path-warn")
        assert note.count() == 0, "no note for the default"
        path.fill("/warned/backups")
        note.wait_for(timeout=5000)
        assert note.inner_text().strip() == text(page, "serverbackup.target.warned"), "the general note: the kind has no text of its own"
        assert note.locator(".mark-sign").count() == 1 and note.get_attribute("role") == "note", "a note with its mark"
        assert "sb-target-new-field-path-warn" in (path.get_attribute("aria-describedby") or "").split(), "it describes the input"
        assert page.locator("#sb-target-new button[type=submit]").is_enabled(), "Add stays on"
        path.fill(data_dir(f"sb-kind-{case.width}-{case.lang}"))
        assert wait_until(page, "!document.querySelector('#sb-target-new-field-path-warn')", 5), "another value: no note"
        assert not path.get_attribute("aria-describedby"), "and no description"

        page.select_option("#sb-target-new-kind", "vault")
        assert page.locator("#sb-target-new input[name=verify]").is_checked(), "a switch shows its default"
        page.fill("#sb-target-new-secret", "s3cret")
        page.select_option("#sb-target-new-kind", "local")
        assert page.locator("#sb-target-new-secret").count() == 0, "no secret for the local kind"
        page.select_option("#sb-target-new-kind", "vault")
        assert page.locator("#sb-target-new-secret").input_value() == "", "another kind chosen: the secret typed is gone"
        page.fill("#sb-target-new-secret", "s3cret")
        page.select_option("#sb-target-new-kind", "local")
        page.fill("#sb-target-new-name", f"Kind {case.width} {case.lang}")
        page.locator("#sb-target-new button[type=submit]").click()
        assert wait_until(page, "!document.querySelector('#sb-target-new form')", 10), "added"
        assert sent and "secret" not in sent[-1] and sent[-1]["kind"] == "local", f"no secret sent for the local kind: {sent}"
    finally:
        clean()
    show_view(page, "tiles")


def test_unknown_kind_keeps_its_fields(page, case):
    """A target whose kind the page does not know (the kinds answered
    without it, as by a server newer than the page): its fields shown off,
    with why; its name saved, its directory stays."""
    stub_kinds(page, [])
    clean()
    where = data_dir(f"sb-usb-{case.width}-{case.lang}")
    target = api_call("POST", TARGETS, {"name": "USB", "kind": "local", "config": {"path": where}})["target"]
    puts = []
    page.on("request", lambda r: puts.append(r.post_data_json) if r.method == "PUT" and r.url.endswith(target["uuid"]) else None)
    try:
        show_view(page, "maint")
        panel = page.locator(f".sb-target[data-uuid='{target['uuid']}']")
        panel.wait_for(timeout=10000)
        panel.locator(":scope > summary").click()
        assert panel.locator(".sb-kind-unknown").inner_text().strip() == text(page, "serverbackup.target.kindUnknown", kind=text(page, "serverbackup.kind.local"))
        field = panel.locator(f"#sb-target-{target['uuid']}-field-path")
        assert field.is_disabled() and field.input_value() == where, "its directory shown, off"
        panel.locator(f"#sb-target-{target['uuid']}-name").fill("USB stick")
        panel.locator("button[type=submit]").click()
        saved = text(page, "serverbackup.target.saved", name="USB stick")
        assert wait_until(page, f"document.querySelector(\".sb-target[data-uuid='{target['uuid']}'] .sb-target-msg\")?.textContent === {saved!r}", 10)
        assert puts and "config" not in puts[-1], f"its fields not sent: {puts}"
        now = next(tg for tg in api_call("GET", TARGETS) if tg["uuid"] == target["uuid"])
        assert now["name"] == "USB stick" and now["config"]["path"] == where, now
    finally:
        clean()
    show_view(page, "tiles")


def test_copy_failed_retried_by_backup_retry_alone(page, case):
    """A backup onto two targets, the copy on one failed (its directory
    under a file); a user with backup.retry alone (/me stubbed; the owner's
    token still answers): the card shows the state, the failure with its
    error, and Retry, and no other button, no key and no target's form; the
    target mended through the API, Retry puts the copy there."""
    clean()
    key_set()
    here = api_call("POST", TARGETS, {"name": "Here", "kind": "local", "config": {"path": data_dir(f"sb-here-{case.width}-{case.lang}")}})["target"]
    broken = api_call("POST", TARGETS, {"name": "Broken", "kind": "local", "config": {"path": data_dir("massimo.db", "sb")}})["target"]
    made = api_call("POST", BACKUPS, {})["backup"]
    states = {c["target"]: c["state"] for c in made["copies"]}
    assert states.get(here["uuid"]) == "there" and states.get(broken["uuid"]) == "failed", states
    # the owner first: Delete of the one copy that is there says that no other is, the failed one beside it counting for nothing
    show_view(page, "maint")
    fold = page.locator(f".sb-backup[data-uuid='{made['uuid']}']")
    fold.wait_for(timeout=10000)
    fold.locator(":scope > summary").click()
    there = fold.locator(f".sb-copy[data-target='{here['uuid']}']")
    there.locator(".sb-copy-delete").click()
    asked = there.locator(".confirm-text").inner_text()
    assert asked == text(page, "serverbackup.deleteLastAsk", target="Here"), asked
    there.locator(".confirm-no").click()
    fold.locator(":scope > summary").click()
    show_view(page, "tiles")
    me = dict(VIEWER, capabilities=["read", "preferences", "backup.retry"])
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(me)))
    try:
        page.evaluate("loadMe()")
        assert wait_until(page, "document.body.classList.contains('no-backup-server') && !document.body.classList.contains('no-backup-retry')", 5), \
            "backup.retry alone"
        shown_maint(page)
        assert page.locator(CARD).is_visible(), "the card for backup.retry"
        fold = page.locator(f".sb-backup[data-uuid='{made['uuid']}']")
        fold.wait_for(timeout=10000)
        assert fold.locator(":scope > summary .flag.bad").count() == 1, "marked: a copy failed"
        fold.locator(":scope > summary").click()
        row = fold.locator(f".sb-copy[data-target='{broken['uuid']}']")
        assert row.get_attribute("data-state") == "failed" and row.locator(".sb-copy-error").inner_text(), "the failure with its error"
        assert fold.locator(f".sb-copy[data-target='{here['uuid']}']").get_attribute("data-state") == "there"
        # a backup the schedule made and failed meanwhile has a Retry of its own (its Delete is backup.server's)
        buttons = page.evaluate(f"[...document.querySelectorAll('{BODY} button')].filter((b) => b.offsetParent).map((b) => b.textContent)")
        assert buttons and set(buttons) == {text(page, "serverbackup.retry")}, f"Retry and nothing else: {buttons}"
        own = fold.evaluate("(d) => [...d.querySelectorAll('button')].map((b) => b.textContent)")
        assert own == [text(page, "serverbackup.retry")], f"its Retry: {own}"
        assert page.locator(f"{BODY} .sb-keys, {BODY} .sb-target, #sb-now, #sb-upload").count() == 0, "no key, no target's form, no making"
        for tg in (here, broken):
            assert page.locator(f"{BODY} .sb-target-state[data-uuid='{tg['uuid']}']").count() == 1, f"{tg['name']}'s state"

        api_call("PUT", f"{TARGETS}/{broken['uuid']}", {"config": {"path": data_dir(f"sb-mended-{case.width}-{case.lang}")}})
        fold.locator(".sb-retry").click()
        assert wait_until(page, f"document.querySelector(\".sb-backup[data-uuid='{made['uuid']}'] .sb-copy[data-target='{broken['uuid']}']\")"
                                f"?.dataset.state === 'there'", 30), "retried: the copy there"
        assert fold.locator(".sb-backup-msg").inner_text().startswith(text(page, "serverbackup.retried")), "it says so"
        now = next(b for b in api_call("GET", BACKUPS)["backups"] if b["uuid"] == made["uuid"])
        if all(c["state"] == "there" for c in now["copies"]):
            assert wait_until(page, f"!document.querySelector(\".sb-backup[data-uuid='{made['uuid']}'] .sb-retry\")", 5), "nothing left to retry"
    finally:
        page.unroute("**/api/v1/me")
        page.evaluate("loadMe()")
        clean()
    assert wait_until(page, "!document.body.classList.contains('no-backup-server')", 5), "the owner again"
    show_view(page, "tiles")
