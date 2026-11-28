# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A request that meets an expired session (AP SignInResend): a write is
never sent again after the sign-in, whoever signs in; it fails at once,
the sign-in opens, and the page says so where the action's result shows,
also once it is back. A read is sent again, and the event stream's
ticket waits for the sign-in rather than retrying.

The 401 is answered by a route, once per request armed, so that the
session itself stays valid and no failed token check is counted against
the address; the requests of the route's method and path are counted.
No write reaches the server: each is refused, or answered by the route.

How a test knows that nothing more is coming: the page sends what
waited for a sign-in at the moment the sign-in is in, before its own
restart (afterLogin) reads who is in, the rooms, and then asks for the
stream's ticket. So once the ticket asked for after the sign-in is seen
and the stream is open, a write sent again would have been seen; where
a read waited too, its second request is the moment the write would
have gone with it. The settings are the exception: the page pushes them
a second after the sign-in (schedulePreferences), after the stream is
open, so a test of a settings change waits for that push itself
(sent_requests, wait_held of helpers.py) before it counts."""
import json
import time

import pytest

from conftest import TOKEN, URL, api_call, clear_preferences
from helpers import (OPERATOR_PASSWORD, frames, held_timers, open_cube_panel, open_personal_page, scaled, sent_requests, show_view, sign_out,
                     wait_held, wait_until)
from test_login import sign_in

NOT_SENT = {"en": "not sent, since you were signed out: do it again once signed in",
            "de": "nicht gesendet, da Sie abgemeldet waren: tun Sie es angemeldet noch einmal"}
SERIAL = "KEQ0000100"
CUBE_PATH = "**/api/v1/cubes/" + SERIAL
DIALOG_OPEN = "document.getElementById('login-dialog').open"


class Refusal:
    """A route on pattern that counts every request of method in sent and
    answers the next armed ones 401; the others go to the server, or are
    answered by answer, a route's handler."""

    def __init__(self, page, pattern, method, armed=1, answer=None, match=None):
        self.method = method
        self.match = match
        self.sent = []
        self.armed = armed
        self.answer = answer
        page.route(pattern, self.handle)

    def handle(self, route):
        if route.request.method != self.method or (self.match and not self.match(route.request)):
            route.continue_()
            return
        self.sent.append(route.request.url)
        if self.armed > 0:
            self.armed -= 1
            route.fulfill(status=401, content_type="application/json", body=json.dumps({"error": "unauthorized"}))
        elif self.answer:
            self.answer(route)
        else:
            route.continue_()


def ok(route):
    """The route's answer of a write taken, so that nothing reaches the server."""
    route.fulfill(status=200, content_type="application/json", body="{}")


def tickets(page):
    """The stream's tickets asked for, a list that grows (sent_requests)."""
    return sent_requests(page, "POST", "**/events/ticket")


def settings_pushes(page):
    """The page's pushes of its settings, a list that grows (sent_requests)."""
    return sent_requests(page, "PUT", "**/api/v1/me/preferences/ui")


def back(page, seen, before):
    """Waits for the page's restart after the sign-in: a ticket asked for
    after the before ones, and the stream open, for 10 s (scaled)."""
    assert wait_until(page, f"!({DIALOG_OPEN})", 10), "the sign-in closed"
    assert wait_until(page, "!document.body.classList.contains('signed-out')", 5), "the page is shown"
    end = time.monotonic() + scaled(10)
    while not (len(seen) > before and page.evaluate("state.eventsOpen")):
        if time.monotonic() >= end:
            raise AssertionError(f"the stream did not open again after the sign-in: {len(seen) - before} new ticket(s)")
        page.wait_for_timeout(200)


def sign_in_key(page):
    """The dialog's sign-in with the owner's key, as the page fixture's."""
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in asked for"
    page.locator("#login-ways .login-way[data-way='key']").click()
    page.locator("#login-token").fill(TOKEN)
    page.locator("#login-key-form button[type=submit]").click()


THERMOSTAT_ROOM = ".room:has(.setpoint-row:not(.hidden))"


def shows_not_sent(page, case):
    """Whether the page shows a not-sent text anywhere, as the user sees it."""
    return page.evaluate("(text) => document.body.innerText.includes(text)", NOT_SENT[case.lang])


def nothing_waits(page):
    """Whether no tile holds a stepped setpoint waiting for its time."""
    return page.evaluate("Object.values(state.ui).every((u) => !u.timer && u.pending === null)")


def own_page(new_context, guard, watch):
    """A page of its own, without the test's token, for a sign-in by
    name, as test_login.py's; the console is not watched there."""
    page = new_context().new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    return page


def test_write_not_sent_again(new_context, case, watch, guard):
    """A setpoint that meets a 401 fails at once on its tile, the sign-in
    opens, and a read that met it too waits: signed in again as the same
    user, the read is sent again and the setpoint is not, and the tile
    still says why; once more, signed in as another user, the setpoint is
    not sent either."""
    page = own_page(new_context, guard, watch)
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    seen = tickets(page)
    writes = Refusal(page, "**/api/v1/cubes/*/rooms/*/setpoint", "PUT")
    reads = Refusal(page, "**/api/v1/me?again=1", "GET")
    tile = page.locator(".room").first
    tile.locator(".mode-auto").click()
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens"
    assert wait_until(page, "document.querySelector('.room .tile-result')?.textContent.includes(%s)" % json.dumps(NOT_SENT[case.lang]), 5), \
        "the tile says the setpoint was not sent, at once"
    assert not page.evaluate("!!document.querySelector('.room .mode-auto.busy')"), "its button no longer turns"
    page.evaluate("() => { window.readAgain = null; api('GET', 'api/v1/me?again=1').then((m) => { window.readAgain = m.user.name; }); }")
    assert wait_held(page, reads.sent), "the read meets the 401"
    before = len(seen)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    back(page, seen, before)
    assert wait_until(page, "window.readAgain === 'operator'", 5), "the read is sent again and answered"
    assert len(reads.sent) == 2, f"the read is sent once more: {len(reads.sent)}"
    assert len(writes.sent) == 1, f"the setpoint is not sent again by the same user: {len(writes.sent)}"
    assert NOT_SENT[case.lang] in tile.locator(".tile-result").inner_text(), "the tile still says it after the sign-in"

    writes.armed = 1
    tile.locator(".mode-auto").click()
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens again"
    before = len(seen)
    sign_in(page, "guest1", "visit-pass")
    back(page, seen, before)
    assert page.evaluate("state.me.user.name") == "guest1", "another user"
    assert len(writes.sent) == 2, f"the setpoint is not sent again by another user: {len(writes.sent)}"
    assert not shows_not_sent(page, case), "nothing tells the other user of the first one's setpoint"


def test_stream_ticket_waits_for_the_sign_in(page, case, watch):
    """A ticket that meets a 401 opens the sign-in and waits for it, as a
    read: while it is open no retry of the stream is planned, and the
    stream opens again once the key is in."""
    watch.expected.append(("/events/ticket", 401))
    seen = tickets(page)
    refusal = Refusal(page, "**/events/ticket", "POST")
    page.evaluate("() => { connectEvents(); }")  # not awaited: it waits for the sign-in
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens"
    assert wait_held(page, refusal.sent), "the ticket meets the 401"
    # a ticket that failed at once would have planned the stream's retry (retryEvents)
    assert page.evaluate("eventsTimer === null"), "no retry of the stream planned while the sign-in is open"
    assert len(refusal.sent) == 1, f"no ticket asked for again while the sign-in is open: {len(refusal.sent)}"
    before = len(seen)
    sign_in_key(page)
    # two tickets: the one that waited, sent again at once, whose stream the page's restart (afterLogin) then
    # closes, and the restart's own (connectEvents), whose stream stays
    back(page, seen, before + 1)
    assert page.evaluate("!!state.events && state.events.readyState === EventSource.OPEN"), "the stream is open"


def test_upload_not_sent_again(page, case, watch):
    """A backup's file (apiRaw) that meets a 401 is not sent again after
    the sign-in, and the box says why, also once the page is back."""
    watch.expected.append(("/restore/plan", 401))
    seen = tickets(page)
    refusal = Refusal(page, "**/restore/plan*", "POST")
    show_view(page, "cube")
    box = page.locator("#cube-backup .backup-box")
    box.scroll_into_view_if_needed()
    box.locator("input[type=file]").set_input_files(files=[{"name": "backup.json", "mimeType": "application/json", "buffer": b"{}"}])
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens"
    msg = "document.querySelector('#cube-backup .backup-msg')"
    said = f"!{msg}.classList.contains('hidden') && {msg}.textContent.includes({json.dumps(NOT_SENT[case.lang])})"
    assert wait_until(page, said, 5), "the box says the file was not sent"
    before = len(seen)
    sign_in_key(page)
    back(page, seen, before)
    assert len(refusal.sent) == 1, f"the file is not sent again: {len(refusal.sent)}"
    assert page.evaluate(said), "the box still says it after the sign-in"


def test_cube_save_after_the_sign_in(page, case, watch):
    """Cube settings' Save that meets a 401 is a failed save: the cross
    on Save, the draft kept, the region's line saying why, also after the
    sign-in, and nothing sent again; the next Save sends it once."""
    from test_unsaved import save_state, wait_save
    watch.expected.append(("/cubes/" + SERIAL, 401))
    seen = tickets(page)
    refusal = Refusal(page, CUBE_PATH, "PUT", answer=ok)
    _, form = open_cube_panel(page)
    field = form.locator("input[name=name]")
    field.fill("Typed before the sign-in")
    form.locator("button[type=submit]").click()
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens"
    wait_save(page, form, [False, True, False, "bad"], "the save that met the 401")
    said = "!document.getElementById('cubes-msg').classList.contains('hidden') && " \
           f"document.getElementById('cubes-msg').textContent.includes({json.dumps(NOT_SENT[case.lang])})"
    assert page.evaluate(said), "the region's line says it was not sent"
    before = len(seen)
    sign_in_key(page)
    back(page, seen, before)
    assert len(refusal.sent) == 1, f"the save is not sent again: {len(refusal.sent)}"
    assert field.input_value() == "Typed before the sign-in", "the draft is kept"
    assert save_state(form) == [False, True, False, "bad"], f"still a failed save: {save_state(form)}"
    assert page.evaluate(said), "the region's line still says it after the sign-in"
    with page.expect_response(lambda r: r.request.method == "PUT" and r.url.endswith("/cubes/" + SERIAL)):
        form.locator("button[type=submit]").click()
    wait_save(page, form, [True, False, False, "ok"], "the save after the sign-in")
    assert len(refusal.sent) == 2, f"the next Save sends it once: {len(refusal.sent)}"


def test_role_save_says_not_sent(page, case, watch):
    """A plain form: a role's Save that meets a 401 says so in the roles'
    line, also once the page is back, and is not sent again."""
    watch.expected.append(("/roles/operator", 401))
    seen = tickets(page)
    refusal = Refusal(page, "**/api/v1/roles/operator", "PUT", answer=ok)
    show_view(page, "system")
    assert wait_until(page, "document.querySelectorAll('.role-panel').length >= 6", 5), "the roles are listed"
    panel = page.locator(".role-panel[data-key='operator']")
    panel.locator("summary").click()
    panel.locator("input[name=name]").fill("Operator resend")
    panel.locator("form button[type=submit]").click()
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens"
    said = "!document.getElementById('roles-msg').classList.contains('hidden') && " \
           f"document.getElementById('roles-msg').textContent.includes({json.dumps(NOT_SENT[case.lang])})"
    assert wait_until(page, said, 5), "the roles' line says it was not sent"
    before = len(seen)
    sign_in_key(page)
    back(page, seen, before)
    assert len(refusal.sent) == 1, f"the role is not sent again: {len(refusal.sent)}"
    assert page.evaluate(said), "the roles' line still says it after the sign-in"


def test_message_line_after_the_sign_in(page, case, watch):
    """The page's message line, which the reload after the sign-in clears:
    a settings change whose save meets a 401 says so there, and says it
    again once the page is back; the change is not sent again. Only the
    change's own PUTs are counted: the page's start sends the browser's
    settings to a server without them by itself."""
    watch.expected.append(("/me/preferences/ui", 401))
    seen, pushes = tickets(page), settings_pushes(page)
    refusal = Refusal(page, "**/api/v1/me/preferences/ui", "PUT", match=lambda r: (r.post_data_json or {}).get("poll") == 37)
    # the page's start pushes the browser's settings to a server without them a second after it read them: gone before
    # the change, so that the change is a push of its own, and the sign-in reads the settings the server has
    assert wait_until(page, "!prefsSync.timer", 5), "the page's start has pushed its settings"
    show_view(page, "prefs")
    page.locator("#poll-input").fill("37")
    page.locator("#poll-input").dispatch_event("change")
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens, a second after the change"
    said = "!document.getElementById('message').classList.contains('hidden') && " \
           f"document.getElementById('message').textContent.includes({json.dumps(NOT_SENT[case.lang])})"
    assert wait_until(page, said, 5), "the message line says it was not sent"
    before, pushed = len(seen), len(pushes)
    sign_in_key(page)
    back(page, seen, before)
    assert page.evaluate(said), "the message line says it again after the sign-in"
    assert wait_held(page, pushes, n=pushed + 1), "the settings pushed a second after the sign-in, the push the change could go with"
    assert len(refusal.sent) == 1, f"the change is not sent again: {len(refusal.sent)}, with {[p.post_data_json for p in pushes[pushed:]]}"


def test_stepped_setpoint_dropped_at_the_sign_in(new_context, case, watch, guard):
    """A setpoint stepped, which waits two seconds to be sent, when another
    request's 401 opens the sign-in: dropped at once, its tile saying it
    was not sent, and no PUT after a quick sign-in of the same user; the
    same with another user, whose page shows nothing of it. No timer is
    left that could send it: what the tiles hold is read."""
    page = own_page(new_context, guard, watch)
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    seen = tickets(page)
    writes = Refusal(page, "**/api/v1/cubes/*/rooms/*/setpoint", "PUT", armed=0, answer=ok)
    reads = Refusal(page, "**/api/v1/me?again=1", "GET", armed=0)
    tile = page.locator(THERMOSTAT_ROOM).first
    for user, password in (("operator", OPERATOR_PASSWORD), ("guest1", "visit-pass")):
        reads.armed = 1  # the read sent again after the sign-in is answered
        tile.locator(".step-plus").click()
        assert not nothing_waits(page), "the stepped value waits for its time"
        page.evaluate("() => { api('GET', 'api/v1/me?again=1').catch(() => {}); }")
        assert wait_until(page, DIALOG_OPEN, 5), "the read's 401 opens the sign-in"
        assert nothing_waits(page), "the stepped value is dropped as the sign-in opens"
        assert NOT_SENT[case.lang] in tile.locator(".tile-result").inner_text(), "its tile says it was not sent"
        before = len(seen)
        sign_in(page, user, password)
        back(page, seen, before)
        assert writes.sent == [], f"no setpoint sent after the sign-in of {user}: {writes.sent}"
        tile = page.locator(THERMOSTAT_ROOM).first
    assert not shows_not_sent(page, case), "the other user sees nothing of it"


def test_another_user_gets_nothing_of_the_first(new_context, case, watch, guard):
    """The operator's setpoint, settings change and own password meet a
    401, each saying it was not sent, on a tile, on the page's message line
    and in a form. The owner signs in with the key, with no settings on
    the server: the page shows none of the three texts, and the owner's
    first settings push carries the defaults, not the operator's poll."""
    clear_preferences()
    page = own_page(new_context, guard, watch)
    page.goto(URL)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    assert wait_until(page, "!document.body.classList.contains('no-sync')", 5), "the operator's settings follow it"
    seen = tickets(page)
    Refusal(page, "**/api/v1/cubes/*/rooms/*/setpoint", "PUT", answer=ok)
    Refusal(page, "**/api/v1/users/operator/register", "POST")
    Refusal(page, "**/api/v1/users/operator", "PUT", armed=0, answer=ok)
    pushes = settings_pushes(page)
    prefs = Refusal(page, "**/api/v1/me/preferences/ui", "PUT", match=lambda r: (r.post_data_json or {}).get("poll") == 37)
    page.locator(THERMOSTAT_ROOM).first.locator(".mode-auto").click()
    assert wait_until(page, DIALOG_OPEN, 5), "the setpoint's 401 opens the sign-in"
    page.evaluate("setSetting('poll', 37)")
    assert wait_held(page, prefs.sent), "the settings push meets the 401"
    page.evaluate("""(current) => {
      for (const [id, v] of [["me-current", current], ["me-new", "not-set-1"], ["me-again", "not-set-1"]]) {
        const el = document.getElementById(id);
        el.value = v;
        el.dispatchEvent(new Event("input", { bubbles: true })); // typed, as a key does: the form's part takes it
      }
    }""", OPERATOR_PASSWORD)
    frames(page)  # the form drawn with what is typed, as before a click
    page.evaluate("() => { document.getElementById('me-password-form').requestSubmit(); }")
    said = "(id) => document.getElementById(id).textContent.includes(%s)" % json.dumps(NOT_SENT[case.lang])
    for where in ("message", "me-password-msg"):
        for _ in range(50):
            if page.evaluate(said, where):
                break
            page.wait_for_timeout(200)
        assert page.evaluate(said, where), f"#{where} says it was not sent"
    assert NOT_SENT[case.lang] in page.locator(".room .tile-result").first.inner_text(), "the tile says it was not sent"
    before, pushed = len(seen), len(pushes)
    sign_in_key(page)
    back(page, seen, before)
    assert page.evaluate("state.me.user.name") == "owner", "the owner"
    # the owner's server has no settings: the page sends this browser's, a second after it read them
    assert wait_held(page, pushes, n=pushed + 1), "the owner's first settings push"
    bodies = [p.post_data_json or {} for p in pushes[pushed:]]
    assert all(b.get("poll") != 37 for b in bodies), f"the operator's poll is not pushed to the owner: {bodies}"
    assert len(prefs.sent) == 1, f"the operator's poll is not sent again: {len(prefs.sent)}"
    assert page.evaluate("state.settings.poll") != 37, "the owner does not get the operator's poll"
    assert not shows_not_sent(page, case), "the page tells the owner nothing of the operator's writes"
    open_personal_page(page)
    assert not page.evaluate(said, "me-password-msg"), "nor the password form"
    clear_preferences()


def test_delete_not_sent_again(page, case, watch):
    """A deletion that meets a 401, a backup's, is not sent again after
    the sign-in, and the backup stays."""
    seen = tickets(page)
    show_view(page, "cube")
    box = page.locator("#cube-backup .backup-box")
    box.scroll_into_view_if_needed()
    taken = api_call("POST", f"api/v1/cubes/{SERIAL}/backups", {})
    try:
        show_view(page, "tiles")
        show_view(page, "cube")  # the list read again
        bid = taken["id"]
        listed = (api_call("GET", f"api/v1/cubes/{SERIAL}/backups") or {}).get("backups") or []
        assert wait_until(page, f"document.querySelectorAll('#cube-backup .backups-list .backup-row').length === {len(listed)}", 10), \
            "the backup is listed"
        watch.expected.append((f"/backups/{bid}", 401))
        refusal = Refusal(page, f"**/backups/{bid}", "DELETE")
        index = next(i for i, b in enumerate(listed) if b["id"] == bid)
        row = box.locator(".backups-list .backup-row").nth(index)
        row.locator(".backup-actions button").last.click()
        row.locator(".confirm-yes").click()
        assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens"
        msg = "document.querySelector('#cube-backup .backup-msg')"
        assert wait_until(page, f"{msg}.textContent.includes({json.dumps(NOT_SENT[case.lang])})", 5), "the box says it was not sent"
        before = len(seen)
        sign_in_key(page)
        back(page, seen, before)
        assert len(refusal.sent) == 1, f"the deletion is not sent again: {len(refusal.sent)}"
        assert any(b["id"] == bid for b in (api_call("GET", f"api/v1/cubes/{SERIAL}/backups") or {}).get("backups") or []), "the backup stays"
    finally:
        api_call("DELETE", f"api/v1/cubes/{SERIAL}/backups/{taken['id']}")


def test_two_tiles_one_sign_in(page, case, watch):
    """Two setpoints on two tiles meet the 401 of one sign-in: each fails
    at once and says so on its tile, and neither is sent again. The 401
    drops the page's token (AP MobileApp, F7), so that its reads until the
    sign-in go without one and are refused too."""
    watch.expected.extend([("/setpoint", 401), ("/api/v1/cubes", 401)])
    seen = tickets(page)
    writes = Refusal(page, "**/api/v1/cubes/*/rooms/*/setpoint", "PUT", armed=2, answer=ok)
    tiles = page.locator(THERMOSTAT_ROOM)
    assert tiles.count() >= 2, "two rooms with a thermostat"
    tiles.nth(0).locator(".mode-auto").click()
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens"
    # the page is hidden behind the sign-in: the second tile's button clicked as the page would be
    page.evaluate("document.querySelectorAll(%s)[1].querySelector('.mode-auto').click()" % json.dumps(THERMOSTAT_ROOM))
    assert wait_held(page, writes.sent, n=2), f"both setpoints meet the 401: {len(writes.sent)}"
    for i in (0, 1):
        assert wait_until(page, "document.querySelectorAll(%s)[%d].querySelector('.tile-result').textContent.includes(%s)"
                          % (json.dumps(THERMOSTAT_ROOM), i, json.dumps(NOT_SENT[case.lang])), 5), f"tile {i + 1} says it was not sent"
    before = len(seen)
    sign_in_key(page)
    back(page, seen, before)
    assert len(writes.sent) == 2, f"neither is sent again: {len(writes.sent)}"
    for i in (0, 1):
        assert NOT_SENT[case.lang] in tiles.nth(i).locator(".tile-result").inner_text(), f"tile {i + 1} still says it"


def sessions_of(user):
    """The ids of the user's live sessions at the server."""
    return {t["id"] for t in api_call("GET", f"api/v1/users/{user}/tokens") or [] if t.get("kind") == "session"}


def test_late_refusal_of_an_earlier_sign_in(new_context, case, watch, guard):
    """F36 (AP MobileApp, D68-A): a read sent without a token while the
    card is open meets its 401 only once the sign-in has answered and the
    card has closed, as a poll whose answer a slow link held back: no
    second card opens, the read is sent again with the new session and
    answered, and that session is the only one the sign-in left at the
    server."""
    before = sessions_of("operator")
    page = own_page(new_context, guard, watch)
    held, sent = [], []

    def handle(route):
        sent.append(route.request.headers.get("authorization") or "")
        if len(sent) == 1:
            held.append(route)  # its 401 held back until the sign-in is in
        else:
            route.continue_()

    page.route("**/api/v1/me?late=1", handle)
    page.goto(URL)
    assert wait_until(page, DIALOG_OPEN, 5), "the card opens without a token"
    page.evaluate("""() => {
      window.lateRead = null;
      api('GET', 'api/v1/me?late=1').then((m) => { window.lateRead = m.user.name; }, (e) => { window.lateRead = 'failed: ' + e.message; });
    }""")
    assert wait_held(page, held), "the read is sent while the card is open"
    assert sent == [""], f"without a token: {sent}"
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, f"!({DIALOG_OPEN}) && !!state.token", 10), "signed in, the card closed"
    page.wait_for_selector(".room", timeout=10000)
    held[0].fulfill(status=401, content_type="application/json", body=json.dumps({"error": "unauthorized"}))
    wait_until(page, f"window.lateRead !== null || {DIALOG_OPEN}", 5)
    for _ in range(10):  # a second card would open at the 401
        assert not page.evaluate(DIALOG_OPEN), "no second card"
        page.wait_for_timeout(200)
    assert wait_until(page, "window.lateRead !== null", 5), "the read is answered"
    assert not page.evaluate("document.body.classList.contains('signed-out')"), "the page stays shown"
    assert page.evaluate("window.lateRead") == "operator", f"the read is answered for the new session: {page.evaluate('window.lateRead')}"
    token = page.evaluate("state.token")
    assert sent == ["", "Bearer " + token], "sent again once, with the new session"
    made = sessions_of("operator") - before
    assert made == {page.evaluate("state.me.token.id")}, f"the sign-in's session the only one it left at the server: {made}"
    sign_out(page)


def test_late_refusal_of_a_write(new_context, case, watch, guard):
    """A write sent without a token while the card is open, by api() and
    by apiRaw (a backup's file), whose 401 comes only once the sign-in has
    answered and the card has closed (AP MobileApp, D68-A): each fails as
    not sent, sent once, and no second card opens."""
    page = own_page(new_context, guard, watch)
    held, sent = [], []

    def handle(route):
        sent.append(route.request.method + " " + route.request.url.split("/api/v1/")[-1])
        held.append(route)

    page.route("**/api/v1/me/preferences/late", handle)
    page.route("**/restore/plan?late=1", handle)
    page.goto(URL)
    assert wait_until(page, DIALOG_OPEN, 5), "the card opens without a token"
    page.evaluate("""() => {
      window.late = {};
      const said = (k) => [() => { window.late[k] = 'sent'; }, (e) => { window.late[k] = e.message; }];
      api('PUT', 'api/v1/me/preferences/late', {}).then(...said('api'));
      apiRaw('POST', 'api/v1/cubes/KEQ0000100/restore/plan?late=1', '{}', 'application/json').then(...said('raw'));
    }""")
    for _ in range(25):
        if len(held) == 2:
            break
        page.wait_for_timeout(200)
    assert len(held) == 2, f"both writes are sent while the card is open: {sent}"
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, f"!({DIALOG_OPEN}) && !!state.token", 10), "signed in, the card closed"
    page.wait_for_selector(".room", timeout=10000)
    for route in held:
        route.fulfill(status=401, content_type="application/json", body=json.dumps({"error": "unauthorized"}))
    assert wait_until(page, "window.late.api !== undefined && window.late.raw !== undefined", 5), "both answered"
    for _ in range(10):
        assert not page.evaluate(DIALOG_OPEN), "no second card"
        page.wait_for_timeout(200)
    late = page.evaluate("window.late")
    assert late == {"api": NOT_SENT[case.lang], "raw": NOT_SENT[case.lang]}, f"each fails as not sent: {late}"
    assert sorted(sent) == ["POST cubes/KEQ0000100/restore/plan?late=1", "PUT me/preferences/late"], f"each sent once: {sent}"
    sign_out(page)


def test_dismissed_then_a_later_sign_in(page, case, watch):
    """A settings change whose push meets a 401 says so on the page's
    message line; the sign-in dismissed, the page is back with the line.
    A later read's 401 and its sign-in show nothing of that write again."""
    watch.expected.extend([("/me/preferences/ui", 401), ("/me?again=1", 401)])
    seen, pushes = tickets(page), settings_pushes(page)
    Refusal(page, "**/api/v1/me/preferences/ui", "PUT", match=lambda r: (r.post_data_json or {}).get("poll") == 37)
    reads = Refusal(page, "**/api/v1/me?again=1", "GET")
    show_view(page, "prefs")
    page.locator("#poll-input").fill("37")
    page.locator("#poll-input").dispatch_event("change")
    assert wait_until(page, DIALOG_OPEN, 5), "the sign-in opens, a second after the change"
    said = "!document.getElementById('message').classList.contains('hidden') && " \
           f"document.getElementById('message').textContent.includes({json.dumps(NOT_SENT[case.lang])})"
    assert wait_until(page, said, 5), "the message line says it was not sent"
    # the dismissal sends the settings' push it held a second later, without a session: refused and said not sent
    # anew, it would open a sign-in of its own; held until the later read's 401 has opened the sign-in, which takes it
    with held_timers(page, "push"):
        page.evaluate("document.getElementById('login-dialog').close()")
        assert wait_until(page, "!document.body.classList.contains('signed-out')", 5), "dismissed: the page is back"
        assert page.evaluate(said), "with the line"
        page.evaluate("() => { api('GET', 'api/v1/me?again=1').catch(() => {}); }")
        assert wait_until(page, DIALOG_OPEN, 5), "a read's 401 opens the sign-in again"
        assert wait_held(page, reads.sent), "the read meets the 401"
    before, pushed = len(seen), len(pushes)
    sign_in_key(page)
    back(page, seen, before)
    assert wait_held(page, pushes, n=pushed + 1), "the settings pushed a second after the sign-in, which could say it again"
    assert len(reads.sent) == 2, f"the read is sent again: {len(reads.sent)}"
    assert not page.evaluate(said), "the message line shows nothing of the write before"
