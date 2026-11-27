# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A new box started from a backup in the first owner's card (AP
ServerBackup v1.0, Block 11): Start from a backup… shows, in the first
owner's form's place, the backup as a file of this device or one of a
target's, which the server lists and fetches, its key pasted or read from
a file, the plan with the switches of the identity, the env file and the
TLS files, a confirmation, then the restore, the restart followed, and the
card turned to the sign-in, saying that the backup's users sign in now.
GET /api/v1/me and the routes of /api/v1/setup/backup are stubbed, as the
first owner's tests stub who is in (FirstOwner D16): the suite's server
has users, whose operator signs in after the turn; the kinds of target are
the server's own."""
import json
import time

from helpers import OPERATOR_PASSWORD, assert_login_card_placed, frames, scaled, scroll_width, wait_held, wait_until
from test_first_owner import DIALOG_OPEN, OPEN, owner_card, sent, start_page

KEY = "# created: 2026-11-20T02:30:00Z\n# public key: age1placeholder\nAGE-SECRET-KEY-1NOTAREALKEY\n"
HELD = {"uuid": "01930000-0000-7000-8000-000000000001", "time": "2026-11-20T02:30:00Z", "trigger": "scheduled", "version": "v0.18.0",
        "schema": 100, "newer": False, "fingerprint": "SHA256:placeholder", "size": 26717,
        "counts": {"users": 3, "integrations": 0, "cubes": 1, "rooms": 6, "audit": 10, "log": 20},
        "files": {"env": True, "tls": 2}, "recipients": [{"label": "laptop", "fingerprint": "placeholder"}]}
PLAN = {"backup": {"uuid": HELD["uuid"], "time": HELD["time"], "trigger": "scheduled", "label": "", "version": "v0.18.0", "schema": 100},
        "identity": {"this_box": False, "fingerprint": "SHA256:placeholder", "node": "node", "answers": True}, "migrated": False,
        "scopes": ["settings", "users", "integrations", "cubes", "history"], "serials": [],
        "cubes": [{"serial": "KEQ0000000", "name": "Upstairs", "live": False}], "env": True, "tls": 2, "files": [],
        "report": {"tables": [], "missing": [], "cleared": [], "removed": [], "secrets_lost": 0, "cubes": []}, "refused": "",
        "restart": {"available": True, "by": "systemd", "blocked": ""}}
RESTORED = {"restart": {"started": True, "by": "systemd"}, "restored_at": "2026-11-27T10:00:00Z",
            "scopes": PLAN["scopes"], "serials": [], "report": PLAN["report"], "files": [], "cubes": []}
SHOWN = "!document.getElementById('new-box').classList.contains('hidden')"


def setup_routes(page, answers, bodies):
    """The routes of /api/v1/setup/backup stubbed: each request's body
    noted in bodies by the route's last part ("upload" for the file), and
    answered from answers[part], (status, body) or a body for 200."""
    def handle(route):
        req = route.request
        part = req.url.split("?")[0].rstrip("/").split("/")[-1]
        part = "upload" if part == "backup" else part
        bodies.setdefault(part, []).append(req.post_data_buffer if part == "upload" else json.loads(req.post_data or "null"))
        answer = answers[part]
        status, body = answer if isinstance(answer, tuple) else (200, answer)
        route.fulfill(status=status, content_type="application/json", body=json.dumps(body))
    page.route("**/api/v1/setup/backup", handle)
    page.route("**/api/v1/setup/backup/*", handle)


def backup_step(page):
    """The start from a backup shown from the first owner's form, its
    title said, the source's choice in focus."""
    page.locator("#setup-from-backup").click()
    assert wait_until(page, SHOWN, 5), "the start from a backup shown"
    assert page.locator("#setup-step").is_hidden(), "in the form's place"
    assert page.evaluate("document.getElementById('login-title').textContent === t('newBox.title')"), "its title"
    assert wait_until(page, "document.activeElement && document.activeElement.id === 'new-box-source'", 3), "the source in focus"
    frames(page)


def hand_file(page):
    """The backup file chosen on this device and handed to the server."""
    page.locator("#new-box-file-input").set_input_files(files=[{"name": "massimo-old.zip", "mimeType": "application/zip", "buffer": b"PK placeholder"}])
    page.locator("#new-box-held").wait_for(state="visible")


def test_start_from_a_file(new_context, case, watch, guard):
    """A file of this device: sent as it is, its manifest's line, the
    keys it opens with; the key pasted, the plan with what is replaced,
    the Cubes, the identity switch and the warning that the old box still
    answers, the env file's and the TLS files' switches, each switch
    planned again; Start asks first, then restores with the key and the
    switches, the key's field emptied, the restart followed by /healthz's
    start; then who is in is refused, and the card turns to the sign-in,
    saying that the backup's users sign in now, where the operator signs
    in and the page starts. The key is in no storage of the browser. Fails
    on the page before: no Start from a backup…."""
    stage = {"me": "open", "health": "before"}
    page = start_page(new_context, guard, watch, OPEN)

    def me_users(route):
        if stage["me"] != "users":
            return route.fallback()
        route.fulfill(status=401, content_type="application/json", body='{"error":"authentication required"}')
    page.route("**/api/v1/me", me_users)
    owner_card(page)
    bodies = {}
    setup_routes(page, {"upload": {"backup": HELD}, "plan": PLAN, "restore": RESTORED}, bodies)
    held_health = []

    def back(route):
        r = route.fetch()
        body = r.json()
        body["started_at"] = "2026-11-27T10:00:05Z"
        route.fulfill(response=r, body=json.dumps(body))

    def healthz(route):
        if stage["health"] == "before":
            return route.continue_()
        if stage["health"] == "hold":
            held_health.append(route)
            return
        back(route)
    page.route("**/healthz", healthz)

    backup_step(page)
    assert_login_card_placed(page, "the start from a backup")
    hand_file(page)
    assert bodies["upload"] == [b"PK placeholder"], "the file sent as it is"
    held = page.locator("#new-box-held-text").text_content()
    assert "v0.18.0" in held and page.evaluate(
        "document.getElementById('new-box-held-text').textContent.endsWith(t('newBox.holds', {users: '3', cubes: '1', rooms: '6'}))"), \
        f"the backup's line: {held}"
    assert "laptop" in page.locator("#new-box-key-hint").text_content(), "the keys it opens with"
    page.locator("#new-box-key").fill(KEY)
    page.locator("#new-box-open").click()
    page.locator("#new-box-plan").wait_for(state="visible")
    assert bodies["plan"][-1] == {"backup": HELD["uuid"], "key_file": KEY.strip(), "take_identity": False, "env": False, "tls": False}, \
        f"the plan's body: {bodies['plan']}"
    assert "Upstairs (KEQ0000000)" in page.locator("#new-box-cubes").text_content(), "the Cubes"
    assert page.locator("#new-box-answers").is_visible(), "the old box still answers, warned of"
    assert page.locator("#new-box-env").is_visible() and page.locator("#new-box-tls").is_visible(), "the env file and the TLS files offered"
    page.locator("label.switch:has(#new-box-env)").click()
    assert wait_until(page, "document.getElementById('new-box-env').checked", 5), "the env file ticked"
    assert wait_until(page, "!document.getElementById('new-box-start').disabled", 5), "Start on again once planned"
    assert len(bodies["plan"]) == 2 and bodies["plan"][-1]["env"] is True, f"planned again with the env file: {bodies['plan']}"
    assert_login_card_placed(page, "the start from a backup with its plan")
    assert scroll_width(page) <= case.width, "no horizontal scroll"

    page.locator("#new-box-start").click()
    page.locator(".confirm-box").wait_for(state="visible")
    assert page.evaluate("document.querySelector('.confirm-box .confirm-text').textContent === t('newBox.confirm')"), "it asks first"
    assert "restore" not in bodies, "nothing restored before the yes"
    page.locator(".confirm-box .confirm-yes").click()
    end = time.time() + scaled(5)
    while "restore" not in bodies and time.time() < end:
        page.wait_for_timeout(50)
    stage["health"] = "hold"  # the start before was read: the restart's follow is held
    assert bodies.get("restore") == [{"backup": HELD["uuid"], "key_file": KEY.strip(), "take_identity": False, "env": True, "tls": False}], \
        f"the restore's body: {bodies.get('restore')}"
    assert wait_until(page, "document.getElementById('new-box-msg').textContent.startsWith(t('newBox.restarting'))", 5), "it says the server restarts"
    assert page.locator("#new-box-key").input_value() == "", "the key's field emptied once sent"
    assert wait_held(page, held_health), "the restart followed by /healthz"
    assert page.locator("#new-box-back").is_disabled() and page.evaluate(SHOWN), "nothing leaves the card while it restarts"

    # the server back, with the backup's users: who is in is refused, and the card turns
    stage["me"], stage["health"] = "users", "back"
    for route in held_health:
        back(route)
    assert wait_until(page, "document.getElementById('setup').classList.contains('hidden')", 10), "the card turned after the restart"
    assert page.evaluate(DIALOG_OPEN), "the card stays open"
    assert page.locator("#login-form").is_visible(), "the sign-in"
    assert page.evaluate("document.getElementById('login-error').textContent === t('newBox.signIn')"), "it says the backup's users sign in now"
    stored = page.evaluate("JSON.stringify(Object.assign({}, localStorage)) + JSON.stringify(Object.assign({}, sessionStorage)) + document.cookie")
    assert "NOTAREALKEY" not in stored, "the key in no storage of the browser"
    page.unroute("**/api/v1/me")
    page.unroute("**/healthz")
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
    page.locator("#login-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, f"!({DIALOG_OPEN}) && state.eventsOpen", 10), "signed in, the page started"
    assert page.evaluate("state.me && state.me.user && state.me.user.name") == "operator", "as the one who signed in"


def test_start_from_a_target(new_context, case, watch, guard):
    """A target's backups: the kinds the server reads from, without the
    one that makes its own key (SFTP); the kind's fields drawn from what
    the server says of it, with their texts, the secret a password field;
    its backups listed with the target in the body, and the one picked
    fetched, after which its line shows. Fails on the page before: no
    Start from a backup…."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    bodies = {}
    older = dict(HELD, uuid="01930000-0000-7000-8000-000000000002", time="2026-11-19T02:30:00Z", name="massimo-older.zip")
    newest = dict(HELD, name="massimo-newest.zip")
    setup_routes(page, {"list": {"backups": [newest, older], "skipped": 1}, "fetch": {"backup": newest}}, bodies)
    backup_step(page)
    page.locator("#new-box-source").select_option("target")
    page.locator("#new-box-kind").wait_for(state="visible")
    kinds = page.evaluate("[...document.getElementById('new-box-kind').options].map((o) => o.value)")
    assert "webdav" in kinds and "local" in kinds and "sftp" not in kinds, f"the kinds a new box reads from: {kinds}"
    page.locator("#new-box-kind").select_option("webdav")
    page.locator("#new-box-field-url").wait_for(state="visible")
    assert page.evaluate("document.querySelector('label[for=new-box-field-url]').textContent === t('serverbackup.field.webdav.url')"), "the field's text"
    assert page.locator("#new-box-field-password").get_attribute("type") == "password", "the secret a password field"
    page.locator("#new-box-field-url").fill("https://nas.example.com/massimo/")
    page.locator("#new-box-field-user").fill("massimo")
    page.locator("#new-box-field-password").fill("nas-secret")
    page.locator("#new-box-list").click()
    page.locator("#new-box-pick").wait_for(state="visible")
    assert bodies["list"] == [{"kind": "webdav", "config": {"url": "https://nas.example.com/massimo/", "user": "massimo"}, "secret": "nas-secret"}], \
        f"the list's body: {bodies['list']}"
    assert page.locator("#new-box-skipped").is_visible(), "the files that are no backup counted"
    page.locator("#new-box-pick").select_option("massimo-newest.zip")
    page.locator("#new-box-fetch").click()
    page.locator("#new-box-held").wait_for(state="visible")
    assert bodies["fetch"][-1]["name"] == "massimo-newest.zip" and bodies["fetch"][-1]["secret"] == "nas-secret", f"the fetch's body: {bodies['fetch']}"
    assert_login_card_placed(page, "the start from a target")
    assert not sent(page, "POST", "/api/v1/server-backup-targets"), "no target stored"


def test_refusals_and_back(new_context, case, watch, guard):
    """A key the server refuses is said under the step, and so is a
    restore it refuses, the card staying; a backup of a newer Massimo
    says so, Open with the key off; Back shows the first owner's form
    again, the focus on Start from a backup…, the key's field emptied.
    Fails on the page before: no Start from a backup…."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    bodies = {}
    answers = {"upload": {"backup": HELD}, "plan": (422, {"error": "serverbackup: the key does not open the backup"}), "restore": PLAN}
    setup_routes(page, answers, bodies)
    backup_step(page)
    hand_file(page)
    page.locator("#new-box-key").fill(KEY)
    page.locator("#new-box-open").click()
    assert wait_until(page, "document.getElementById('new-box-msg').textContent.includes('does not open the backup')", 5), "the refusal said"
    assert page.locator("#new-box-plan").is_hidden() and page.locator("#new-box-start").is_disabled(), "no plan, no Start"
    answers["plan"] = dict(PLAN, refused="this server cannot restart itself")
    page.locator("#new-box-open").click()
    page.locator("#new-box-refused").wait_for(state="visible")
    assert page.locator("#new-box-start").is_disabled(), "Start off with the plan's refusal"
    answers["plan"] = PLAN
    answers["restore"] = (409, {"error": "an update is running"})
    page.locator("#new-box-open").click()
    assert wait_until(page, "!document.getElementById('new-box-start').disabled", 5), "Start on"
    page.locator("#new-box-start").click()
    page.locator(".confirm-box .confirm-yes").click()
    assert wait_until(page, "document.getElementById('new-box-msg').textContent.includes('an update is running')", 5), "the restore's refusal said"
    assert page.evaluate(SHOWN) and page.evaluate(DIALOG_OPEN), "the card stays, with the step"
    answers["upload"] = {"backup": dict(HELD, newer=True)}
    hand_file(page)
    page.locator("#new-box-newer").wait_for(state="visible")
    assert page.locator("#new-box-open").is_disabled(), "a newer backup is not opened"
    page.locator("#new-box-back").click()
    assert wait_until(page, "!document.getElementById('setup-step').classList.contains('hidden') && " + "!(" + SHOWN + ")", 5), "the form again"
    assert wait_until(page, "document.activeElement && document.activeElement.id === 'setup-from-backup'", 3), "the focus on the button that asked"
    assert page.evaluate("document.getElementById('new-box-key') === null || document.getElementById('new-box-key').value === ''"), "the key emptied"
