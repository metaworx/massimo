# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Maintenance and System info follow the server while shown (AP UiParts,
Block 12, D198): an event of the stream reads again what it says has
changed, for the page shown alone, once for a burst, never for the
session after, and never in the way of an update, a restart or a reboot
the page follows itself; a load that comes in leaves what a user holds
where it is, a text selected, a question open, a choice with the focus on
it; a question goes where what it asked about changed; what follows the
server while it is away ends at a sign-out, and an answer that comes
after one shows nothing. GET /api/v1/update is stubbed; nothing is
installed, restarted or rebooted."""
import json
import time

import pytest

from conftest import TOKEN, URL, api_call
from helpers import answered, frames, hold, mark, owner_page, replaced, safe_route, scaled, show_view, update_json, wait_held, wait_until
from test_admin_updates import across
from test_unsaved import expire
from test_reboot import RebootStub, iso, reboot_text
from test_restart import AVAILABLE, BLOCKED, UNPARSED, settings_read, stub_restart
from test_system_info import PENDING, UNREAD
from test_update import channel_routes, unroute_channel

UPDATE = "**/api/v1/update*"

HISTORY = [
    {"at": "2026-11-14T18:00:00Z", "by": {"name": "anna", "display_name": "Anna"}, "from": "v0.0.2", "to": "v0.0.3"},
    {"at": "2026-11-05T12:00:00Z", "by": {"name": "ben"}, "from": "v0.0.1", "to": "v0.0.2", "reached": "v0.0.2",
     "reached_at": "2026-11-05T12:01:00Z"},
]


def update_route(page, answer):
    """GET /api/v1/update answered with answer()'s body, a dict, at every
    read; a POST is refused, so that nothing is installed. Returns the
    reads' count."""
    reads = []

    def handle(route):
        if route.request.method != "GET":
            route.abort()
            return
        reads.append(route.request.url)
        route.fulfill(status=200, content_type="application/json", body=json.dumps(answer()))
    safe_route(page, UPDATE, handle)
    return reads


def offered(version="v0.0.1-test", history=None, state="up-to-date"):
    """An update to version offered, the helper in state, with history."""
    body = json.loads(update_json(state, version, True))
    body["history"] = history or []
    return body


# /me of the next user, who may hold settings and install updates (stubbed), for a test that shows them Maintenance
NEXT_ME = {"mode": "users", "user": {"name": "operator", "role": "operator", "has_password": True}, "role": {"key": "operator"},
           "capabilities": ["read", "preferences", "settings", "update", "reboot"], "token": {"kind": "session"}}


def as_next_user(page):
    """The next user's /me stubbed with NEXT_ME and read: they may hold settings, install updates and reboot."""
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(NEXT_ME)))
    page.evaluate("loadMe()")
    assert wait_until(page, "hasCap('settings') && hasCap('reboot')", 5), "the next user's capabilities"


def wait_reads(page, reads, n, seconds=5):
    """Waits until reads has n entries, for seconds (scaled), and the
    last answer has been drawn."""
    end = time.time() + scaled(seconds)
    while len(reads) < n and time.time() < end:
        page.wait_for_timeout(50)
    frames(page)
    return len(reads) >= n


def select_text(page, selector):
    """The text of selector's element selected, as a user selects it to copy it."""
    page.evaluate("""(s) => { const r = document.createRange(); r.selectNodeContents(document.querySelector(s));
      const sel = getSelection(); sel.removeAllRanges(); sel.addRange(r); }""", selector)


def selected(page):
    return page.evaluate("getSelection().toString()")


def test_software_keeps_a_selection_and_its_question_through_a_load(page, case):
    """Software update with the last updates and Update asking: the
    header's check every half hour (MassimoParts.software.load) and Maintenance shown
    again read the same answer; the line selected stays selected, the
    question, its buttons and the lines are the elements they were."""
    reads = update_route(page, lambda: offered(history=HISTORY))
    try:
        show_view(page, "maint")
        line = "#update-history-list li:first-child"
        page.locator(line).wait_for()
        page.locator("#update-btn").click()
        page.locator(".update .confirm-box").wait_for()
        sels = [line, "#update-history-list li:last-child", ".update .confirm-no", ".update .confirm-yes", "#update-check"]
        mark(page, sels)
        select_text(page, line)
        want = selected(page)
        assert want, "a line selected"
        n = len(reads)
        page.evaluate("MassimoParts.software.load(false)")
        show_view(page, "maint")
        assert wait_reads(page, reads, n + 2), f"read twice more ({len(reads) - n})"
        frames(page)
        assert replaced(page, sels) == [], f"the same elements after the loads: {replaced(page, sels)}"
        assert selected(page) == want, "the line still selected"
        page.locator(".update .confirm-no").click()
        assert wait_until(page, "document.activeElement.id === 'update-btn'", 5), "No gives the focus back to Update"
    finally:
        page.unroute(UPDATE)


def test_a_newer_release_takes_the_question_back(page, case):
    """Update asks about v0.0.1-test; a check that finds v0.0.2-test takes
    the question back, the focus, which was on its No, back on Update, and
    Update names the newer one: the question never names a release the
    page knows to be gone."""
    now = {"version": "v0.0.1-test"}
    update_route(page, lambda: offered(now["version"]))
    try:
        show_view(page, "maint")
        page.locator("#update-btn").click()
        box = page.locator(".update .confirm-box")
        box.wait_for()
        assert "v0.0.1-test" in box.inner_text(), "the question names the release"
        assert wait_until(page, "document.activeElement.classList.contains('confirm-no')", 5), "the focus on No"
        now["version"] = "v0.0.2-test"
        page.evaluate("MassimoParts.software.load(false)")
        assert wait_until(page, "!document.querySelector('.update .confirm-box')", 5), "the question taken back"
        assert wait_until(page, "document.activeElement.id === 'update-btn'", 5), \
            f"the focus back on Update, not the body ({page.evaluate('document.activeElement.id || document.activeElement.tagName')})"
        assert "v0.0.2-test" in page.locator("#update-btn").inner_text() and page.locator("#update-btn").is_visible(), \
            "Update offers the newer release"
    finally:
        page.unroute(UPDATE)


def test_a_second_channel_during_the_first_ones_check(page, case):
    """beta chosen, its release check held; stable chosen meanwhile: both
    PUTs go out in their order, stable is shown and stored, and the first
    check's answer, let go late and saying beta, changes nothing shown."""
    puts = channel_routes(page)
    checks = []
    page.route("**/api/v1/update?refresh=1", lambda route: checks.append(route) if route.request.method == "GET" else route.fallback())
    try:
        show_view(page, "maint")
        sel = page.locator("#update-channel")
        sel.wait_for(state="visible")
        sel.select_option("beta")
        assert wait_held(page, checks), "beta's release check"
        sel.select_option("stable")
        end = time.time() + 5
        while len(checks) < 2 and time.time() < end:
            page.wait_for_timeout(50)
        assert puts == [{"update.channel": "beta"}, {"update.channel": "stable"}], f"both PUTs, in their order ({puts})"
        assert len(checks) == 2, f"stable's release check ({len(checks)})"
        checks[1].fallback()
        assert wait_until(page, "document.getElementById('update-text').textContent.includes('stable')", 5), "stable's check shown"
        assert wait_until(page, "document.getElementById('update-channel').value === 'stable'", 5), "stable chosen"
        late = json.loads(update_json("up-to-date", "v0.0.1-test", False, None, "beta"))
        with page.expect_response(lambda r: "refresh=1" in r.url):
            checks[0].fulfill(status=200, content_type="application/json", body=json.dumps(late))
        frames(page)
        assert "stable" in page.locator("#update-text").inner_text() and sel.input_value() == "stable", \
            "the first check's late answer changes nothing shown"
    finally:
        page.unroute("**/api/v1/update?refresh=1")
        unroute_channel(page)


def test_the_helpers_reading_waits_for_a_sign_in(new_context, case, watch, guard):
    """The helper installs and the page reads every 3 s; the session
    expires and the sign-in is asked for: the read that met the 401 waits
    for it, and no other read is started meanwhile, the clock run on for
    ten seconds."""
    page = owner_page(new_context, guard, watch, URL, TOKEN, clock=True)
    reads = []

    def handle(route):
        if route.request.method != "GET":
            route.abort()
            return
        reads.append(route.request.url)
        if route.request.headers.get("authorization") != "Bearer " + TOKEN:
            route.fulfill(status=401, content_type="application/json", body='{"error": "unauthorized"}')
            return
        route.fulfill(status=200, content_type="application/json", body=json.dumps(offered(state="installing")))
    safe_route(page, UPDATE, handle)
    show_view(page, "maint")
    assert wait_until(page, "document.getElementById('update-btn').classList.contains('busy')", 5), "the helper followed"
    expire(page)
    n = len(reads)
    for _ in range(5):
        page.clock.run_for(2000)
        answered(page)
    assert len(reads) - n <= 1, f"one read waits for the sign-in, none started beside it ({len(reads) - n})"


def test_the_channel_keeps_the_focus_while_it_saves(page, case):
    """The channel chosen is saved at once: while the PUT runs the choice
    stays on with beta in it and the focus on it, and beta stays once
    the release check has followed it."""
    puts = channel_routes(page)
    held = []
    try:
        show_view(page, "maint")
        sel = page.locator("#update-channel")
        sel.wait_for(state="visible")
        # the PUT held; the reads go on to the stub of channel_routes
        page.route("**/api/v1/settings", lambda route: held.append(route) if route.request.method == "PUT" else route.fallback())
        sel.focus()
        sel.select_option("beta")
        assert wait_held(page, held), "the PUT sent"
        frames(page)
        assert sel.is_enabled() and sel.input_value() == "beta", "on, with beta, while it saves"
        assert page.evaluate("document.activeElement.id") == "update-channel", "the focus on it"
        for r in held:
            r.fallback()
        assert wait_until(page, "document.getElementById('update-text').textContent.includes('beta')", 5), "the check follows beta"
        assert puts == [{"update.channel": "beta"}], f"beta stored ({puts})"
        assert wait_until(page, "document.getElementById('update-channel').value === 'beta'", 5), \
            f"beta shown ({sel.input_value()!r})"
        assert page.evaluate("document.activeElement.id") == "update-channel", "the focus still on it"
    finally:
        page.unroute("**/api/v1/settings")
        unroute_channel(page)


def software_busy(page):
    return page.evaluate("document.getElementById('update-btn').classList.contains('busy') || document.getElementById('update-check').disabled")


@pytest.mark.parametrize("how", ["sign-out", "expired"])
def test_the_session_end_ends_the_helpers_reading(new_context, case, watch, guard, how):
    """The helper installs and the page reads every 3 s: the session ends,
    and once the operator is in, the reading has ended with it, Software
    update's buttons are back; a read held across the end, let go, starts
    no reading for the next user, and the clock run past the reading's
    pace brings no read."""
    page = owner_page(new_context, guard, watch, URL, TOKEN, clock=True)
    update_route(page, lambda: offered(state="installing"))
    show_view(page, "maint")
    assert wait_until(page, "document.getElementById('update-btn').classList.contains('busy')", 5), "the helper followed"
    reads = []
    hold(page, UPDATE, "GET", reads, [])
    assert wait_held(page, reads, 8), "the next reading asked for"
    across(page, how)
    assert not software_busy(page), "the reading ended with the session"
    with page.expect_response(lambda r: "/api/v1/update" in r.url):
        for r in reads:
            r.fallback()
    frames(page)
    assert not software_busy(page), "the late answer started nothing"
    n = len(reads)
    page.clock.run_for(7000)
    answered(page, held=reads)
    assert len(reads) == n, f"no reading for the next user ({len(reads) - n})"


@pytest.mark.parametrize("how", ["sign-out", "expired"])
def test_the_next_user_is_followed_again(new_context, case, watch, guard, how):
    """The helper installs and the page reads every 3 s; the session ends
    while the reading waits on its timer: the clock run on brings no
    read, and the next user who opens Maintenance with the helper still
    at work is followed again, the buttons greyed out."""
    page = owner_page(new_context, guard, watch, URL, TOKEN, clock=True)
    update_route(page, lambda: offered(state="installing"))
    reads = counted(page, UPDATE)
    show_view(page, "maint")
    assert wait_until(page, "document.getElementById('update-btn').classList.contains('busy')", 5), "the helper followed"
    n = len(reads)
    page.clock.run_for(3100)
    assert wait_reads(page, reads, n + 1), "a reading's read answered: the reading waits on its timer"
    across(page, how)
    n = len(reads)
    page.clock.run_for(7000)
    answered(page)
    assert len(reads) == n, f"no reading for the next user ({len(reads) - n})"
    as_next_user(page)
    show_view(page, "maint")
    assert wait_until(page, "document.getElementById('update-btn').classList.contains('busy')", 5), \
        "the next user, the helper at work, followed again"


@pytest.mark.parametrize("how", ["sign-out", "expired"])
def test_the_session_end_takes_the_update_question_back(new_context, case, watch, guard, how):
    """Update asking: the session ends, and the operator finds no question."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    update_route(page, lambda: offered())
    show_view(page, "maint")
    page.locator("#update-btn").click()
    page.locator(".update .confirm-box").wait_for()
    across(page, how)
    frames(page)
    assert page.locator(".update .confirm-box").count() == 0, "no question for the next user"


def send(page, kind, summary=None):
    """A server's event of kind, as the stream brings it, dispatched on the page's stream."""
    page.evaluate("""([k, s]) => state.events.dispatchEvent(new MessageEvent(k, { data: JSON.stringify({
      kind: k, source: "system", time: new Date().toISOString(), summary: s }) }))""", [kind, summary or {}])


def counted(page, pattern, method="GET"):
    """The requests of method to pattern from now on, each let on to the routes before and the server."""
    got = []

    def handle(route):
        if route.request.method == method:
            got.append(route.request.url)
        route.fallback()
    page.route(pattern, handle)
    return got


def settings_change(**want):
    """A match of Stream.wait: the change event of a PUT /api/v1/settings
    that set exactly want, as the server sends it for a write of the
    test's own (None for a setting set back to its default)."""
    return lambda d: (d.get("summary") or {}).get("action") == "settings.update" and (d.get("summary") or {}).get("set") == want


def test_software_reads_again_at_a_resync_while_shown_only(page, case):
    """Rooms shown, a resync of the stream (how the page learns of a
    restart or an update ended elsewhere) reads no versions; with
    Maintenance shown it reads them once and shows the newer release,
    without a reload."""
    now = {"version": "v0.0.1-test"}
    update_route(page, lambda: offered(now["version"]))
    reads = counted(page, UPDATE)
    try:
        show_view(page, "tiles")
        send(page, "resync")
        answered(page)
        assert reads == [], f"no read for a page not shown ({reads})"
        show_view(page, "maint")
        assert wait_until(page, "document.getElementById('update-text').textContent.includes('v0.0.1-test')", 5), "the release"
        n = len(reads)
        now["version"] = "v0.0.2-test"
        send(page, "resync")
        assert wait_until(page, "document.getElementById('update-text').textContent.includes('v0.0.2-test')", 5), \
            "the newer release shown, no reload"
        answered(page)
        assert len(reads) == n + 1, f"one read ({len(reads) - n})"
    finally:
        page.unroute(UPDATE)


def test_a_burst_of_events_makes_one_read_and_one_after_it(page, case):
    """Five resyncs at once with Maintenance shown: one read of the
    versions in flight, held, and one more once it has answered, which
    reads for the four that came meanwhile."""
    update_route(page, lambda: offered())
    try:
        show_view(page, "maint")
        page.locator("#update-btn").wait_for(state="visible")
        held = []
        page.route(UPDATE, lambda route: held.append(route) if route.request.method == "GET" else route.fallback())
        for _ in range(5):
            send(page, "resync")
        assert wait_held(page, held), "the first read"
        answered(page, held=held)
        assert len(held) == 1, f"one read in flight ({len(held)})"
        held.pop().fallback()
        assert wait_held(page, held), "one more after it"
        answered(page, held=held)
        assert len(held) == 1, f"one read for the four ({len(held)})"
        held.pop().fallback()
        answered(page, held=held)
        assert not held, f"nothing more ({len(held)})"
    finally:
        page.unroute(UPDATE)


def test_a_burst_left_by_its_page_makes_no_second_read(page, case):
    """Two resyncs with Maintenance shown, the first read held; Rooms
    shown before it answers: the second read, asked for while
    Maintenance was shown, is not made for a page no longer shown."""
    update_route(page, lambda: offered())
    try:
        show_view(page, "maint")
        page.locator("#update-btn").wait_for(state="visible")
        held = []
        page.route(UPDATE, lambda route: held.append(route) if route.request.method == "GET" else route.fallback())
        send(page, "resync")
        send(page, "resync")
        assert wait_held(page, held), "the first read"
        show_view(page, "tiles")
        with page.expect_response(lambda r: "/api/v1/update" in r.url):
            held.pop().fallback()
        answered(page, held=held)  # the first read's answer handled: the second would have been asked for by now
        frames(page)
        assert not held, f"no second read for Rooms ({len(held)})"
    finally:
        page.unroute(UPDATE)


def test_an_update_from_another_browser_is_followed_with_its_grace(page, case):
    """An update asked for from another browser (update event): the page
    follows it as it follows its own, the reading with its 30 s of
    grace, so that a first read that still finds the helper's last state
    (up to date) does not end it; the second finds it installing."""
    asked = []
    after = {"event": False}

    def answer():
        if not after["event"]:
            return offered()
        asked.append(1)  # the reads after the event: the first still up to date, then installing
        return offered(state="installing" if len(asked) >= 2 else "up-to-date")
    update_route(page, answer)
    try:
        show_view(page, "maint")
        page.locator("#update-btn").wait_for(state="visible")
        n = 0
        after["event"] = True
        send(page, "update", {"from": "v0.0.0-test", "to": "v0.0.1-test", "channel": "alpha"})
        assert wait_until(page, "document.getElementById('update-btn').classList.contains('busy')", 2), "followed from the event"
        end = time.time() + 12
        while len(asked) < n + 2 and time.time() < end:
            page.wait_for_timeout(100)
        assert len(asked) >= n + 2, f"read twice, 3 s apart ({len(asked) - n})"
        frames(page)
        assert software_busy(page), "still followed after a first read that found the helper up to date"
    finally:
        page.unroute(UPDATE)


def test_the_channels_own_change_event_reads_nothing_behind_its_back(page, case):
    """beta chosen, then stable while beta's release check is held; the
    change event of beta's PUT comes late: it reads nothing while the
    page's own change is under way, so the select never shows beta after
    stable was chosen."""
    puts = channel_routes(page)
    checks = []
    page.route("**/api/v1/update?refresh=1", lambda route: checks.append(route) if route.request.method == "GET" else route.fallback())
    reads = counted(page, "**/api/v1/settings")
    try:
        show_view(page, "maint")
        sel = page.locator("#update-channel")
        sel.wait_for(state="visible")
        page.evaluate("""() => { window.__seen = []; const sel = document.getElementById('update-channel');
          window.__watch = setInterval(() => window.__seen.push(sel.value), 5); }""")
        sel.select_option("beta")
        assert wait_held(page, checks), "beta's check"
        sel.select_option("stable")
        end = time.time() + 5
        while len(checks) < 2 and time.time() < end:
            page.wait_for_timeout(50)
        n = len(reads)
        send(page, "change", {"action": "settings.update", "set": {"update.channel": "beta"}})
        answered(page, held=checks)
        assert len(reads) == n, f"the late event reads nothing while the page's own change is under way ({len(reads) - n})"
        for c in checks:
            c.fallback()
        assert wait_until(page, "document.getElementById('update-text').textContent.includes('stable')", 5), "stable's check"
        frames(page)
        seen = page.evaluate("() => { clearInterval(window.__watch); return window.__seen; }")
        after = seen[seen.index("stable"):] if "stable" in seen else []
        assert after and "beta" not in after, f"beta never shown after stable was chosen ({seen[-20:]})"
        assert puts == [{"update.channel": "beta"}, {"update.channel": "stable"}]
    finally:
        page.unroute("**/api/v1/settings")
        page.unroute("**/api/v1/update?refresh=1")
        unroute_channel(page)


def test_a_question_taken_back_leaves_a_focus_elsewhere(page, case):
    """Update asking; the user clicks on the versions' text, so that the
    focus is on the page's body; a check that finds a newer release takes
    the question back: the focus stays where it was, and the page does
    not scroll."""
    now = {"version": "v0.0.1-test"}
    update_route(page, lambda: offered(now["version"]))
    try:
        show_view(page, "maint")
        page.locator("#update-btn").click()
        page.locator(".update .confirm-box").wait_for()
        page.locator("#update-text").click()
        assert page.evaluate("document.activeElement === document.body"), "the focus on the body after a click on text"
        y = page.evaluate("window.scrollY")
        now["version"] = "v0.0.2-test"
        page.evaluate("MassimoParts.software.load(false)")
        assert wait_until(page, "!document.querySelector('.update .confirm-box')", 5), "the question taken back"
        frames(page)
        assert page.evaluate("document.activeElement === document.body"), \
            f"the focus not moved ({page.evaluate('document.activeElement.id || document.activeElement.tagName')})"
        assert page.evaluate("window.scrollY") == y, "no scroll"
    finally:
        page.unroute(UPDATE)

def server_now(page, seconds=0):
    """The server's time, as the page's clock corrected by the server's has it, seconds from now, as an event carries it."""
    return page.evaluate("(s) => new Date(Date.now() + serverSkew + s * 1000).toISOString()", seconds)


def send_at(page, kind, at, summary=None):
    """A server's event of kind with its time at, dispatched on the page's stream."""
    page.evaluate("""([k, at, s]) => state.events.dispatchEvent(new MessageEvent(k, { data: JSON.stringify({
      kind: k, source: "system", time: at, summary: s }) }))""", [kind, at, summary or {}])


def test_an_update_from_another_browser_takes_the_question_back(page, case):
    """Update asking; another browser asks for the update (update, fresh):
    the question goes, the buttons are greyed out, and Yes cannot be
    reached."""
    update_route(page, lambda: offered())
    try:
        show_view(page, "maint")
        page.locator("#update-btn").click()
        page.locator(".update .confirm-box").wait_for()
        send_at(page, "update", server_now(page), {"to": "v0.0.1-test"})
        assert wait_until(page, "!document.querySelector('.update .confirm-box')", 2), "the question gone"
        assert software_busy(page), "the buttons greyed out"
    finally:
        page.unroute(UPDATE)


def test_a_refused_request_leaves_the_reading_its_buttons(page, case, watch):
    """Update's Yes, its POST held; meanwhile a check finds the helper at
    work and the reading begins; the POST refused with 409 (one already
    runs): the buttons stay greyed out and the helper's line stays, with
    no word of a request not started."""
    state = {"helper": "up-to-date"}
    posts = []
    watch.expected.append(("/api/v1/update", 409))  # the refusal is the test's

    def handle(route):
        if route.request.method == "POST":
            posts.append(route)
            return
        route.fulfill(status=200, content_type="application/json", body=json.dumps(offered(state=state["helper"])))
    safe_route(page, UPDATE, handle)
    try:
        show_view(page, "maint")
        page.locator("#update-btn").click()
        page.locator(".update .confirm-yes").click()
        assert wait_held(page, posts), "the POST sent"
        state["helper"] = "installing"
        page.evaluate("MassimoParts.software.load(false)")  # the header's check: the helper at work, the reading begins
        helper = page.locator("#update-helper").inner_text()
        with page.expect_response(lambda r: r.request.method == "POST" and "/api/v1/update" in r.url):
            posts[0].fulfill(status=409, content_type="application/json", body='{"error": "an update is already running"}')
        answered(page)
        frames(page)
        assert software_busy(page), "the buttons stay greyed out"
        assert page.locator("#update-helper").inner_text() == helper, "the helper's line stays"
    finally:
        page.unroute(UPDATE)


def test_an_old_update_event_starts_no_reading(page, case):
    """An update event five minutes old by the server's clock, replayed
    after a gap of the stream: it reads the versions once and turns no
    button off."""
    update_route(page, lambda: offered())
    reads = counted(page, UPDATE)
    try:
        show_view(page, "maint")
        page.locator("#update-btn").wait_for(state="visible")
        answered(page)
        n = len(reads)
        send_at(page, "update", server_now(page, -300), {"to": "v0.0.1-test"})
        answered(page)
        frames(page)
        assert not software_busy(page), "no button off"
        assert len(reads) == n + 1, f"one read ({len(reads) - n})"
    finally:
        page.unroute(UPDATE)

def test_events_while_the_helper_is_followed_read_nothing(page, case):
    """The helper installs and the page reads every 3 s: an update event
    and resyncs meanwhile add no read and leave the reading's pace, and
    the buttons stay greyed out."""
    update_route(page, lambda: offered(state="installing"))
    try:
        show_view(page, "maint")
        assert wait_until(page, "document.getElementById('update-btn').classList.contains('busy')", 5), "the helper followed"
        held = []
        page.route(UPDATE, lambda route: held.append(route) if route.request.method == "GET" else route.fallback())
        assert wait_held(page, held, 8), "the reading's next read"
        for kind in ("update", "resync", "resync"):
            send(page, kind, {"to": "v0.0.1-test"})
        answered(page, held=held)
        assert len(held) == 1, f"the events read nothing beside the reading ({len(held)})"
        assert software_busy(page), "still followed"
        held.pop().fallback()
    finally:
        page.unroute(UPDATE)


@pytest.mark.parametrize("how", ["sign-out", "expired"])
def test_a_read_asked_for_before_the_session_ended_is_not_made(new_context, case, watch, guard, how):
    """Two resyncs with Maintenance shown, the first one's read held:
    the session ends, the read is let go, and the second, asked for in
    the session before, is not made."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    update_route(page, lambda: offered())
    show_view(page, "maint")
    page.locator("#update-btn").wait_for(state="visible")
    held = []
    page.route(UPDATE, lambda route: held.append(route) if route.request.method == "GET" else route.fallback())
    send(page, "resync")
    send(page, "resync")
    assert wait_held(page, held), "the first read"
    across(page, how)
    first = held.pop()
    first.fallback()
    answered(page, held=held)
    assert not held, f"no read for the session before ({len(held)})"


# --- Maintenance -------------------------------------------------------------

def test_an_interval_keeps_the_focus_while_it_saves(page, case):
    """The refresh's interval chosen is saved at once: while the PUT runs
    the choice stays on with 1 h in it and the focus on it, and the
    store's answer, once let go, shows it waiting for the restart. (The
    poll's is read-only: the test server runs with -poll 0.)"""
    api_call("PUT", "api/v1/settings", {"refresh_interval": None})
    held = []
    try:
        show_view(page, "maint")
        sel = page.locator("#interval-refresh")
        assert wait_until(page, "document.querySelector('#interval-refresh-text').textContent.length > 0", 5), "the line is filled"
        hold(page, "**/api/v1/settings", "PUT", held, [])
        sel.focus()
        sel.select_option("3600")
        assert wait_held(page, held), "the PUT sent"
        frames(page)
        assert sel.is_enabled() and sel.input_value() == "3600", "on, with 1 h, while it saves"
        assert page.evaluate("document.activeElement.id") == "interval-refresh", "the focus on it"
        for r in held:
            r.fallback()
        assert wait_until(page, "!document.querySelector('#interval-refresh-msg').classList.contains('hidden')", 5), "saved"
        assert sel.input_value() == "3600" and page.evaluate("document.activeElement.id") == "interval-refresh", \
            "1 h, the focus still on it"
    finally:
        page.unroute("**/api/v1/settings")
        api_call("PUT", "api/v1/settings", {"refresh_interval": None})


def test_maintenance_keeps_what_it_holds_through_its_reads(page, case):
    """The delay typed and the restart's question open: Maintenance shown
    again reads every row again, and the delay, the question with its
    buttons, the selects and Reload now are the elements they were, with
    90 in the delay. It passes on the plain page too, which filled these
    in place; it guards that the part keeps them."""
    posts = []
    stub_restart(page, AVAILABLE, posts)
    stub = RebootStub(page, {"available": True, "by": "systemd"})
    try:
        show_view(page, "maint")
        page.locator("#reboot-btn").wait_for(state="visible")
        page.locator("#restart-btn").wait_for(state="visible")
        page.locator("#reboot-delay").fill("90")
        page.locator("#restart-btn").click()
        page.locator(".restart .confirm-box").wait_for()
        sels = ["#reboot-delay", ".restart .confirm-no", ".restart .confirm-yes", "#announce-mode", "#interval-poll", "#settings-reload", "#reboot-btn"]
        mark(page, sels)
        settings_reads = counted(page, "**/api/v1/settings")
        with page.expect_response(lambda r: r.url.endswith("/api/v1/reboot")), \
                page.expect_response(lambda r: r.url.endswith("/api/v1/restart")):
            show_view(page, "maint")
        # Software's read of the channel and Maintenance's of its settings, both
        assert wait_reads(page, settings_reads, 2), f"both reads of the settings ({len(settings_reads)})"
        answered(page)
        assert replaced(page, sels) == [], f"the same elements after the reads: {replaced(page, sels)}"
        assert page.locator("#reboot-delay").input_value() == "90", "the delay typed"
        assert not posts and not stub.requests, "nothing sent"
    finally:
        stub.close()
        page.unroute("**/api/v1/restart")


def test_the_restart_question_goes_when_the_restart_is_not_offered(page, case):
    """The restart asking; Maintenance shown again finds it not offered
    any more (a terminal's server): the question goes with the button."""
    posts = []
    answer = {"body": AVAILABLE}

    def handle(route):
        if route.request.method != "GET":
            posts.append(route.request.method)
            route.abort()
            return
        route.fulfill(status=200, content_type="application/json", body=answer["body"])
    safe_route(page, "**/api/v1/restart", handle)
    try:
        show_view(page, "maint")
        page.locator("#restart-btn").click()
        page.locator(".restart .confirm-box").wait_for()
        answer["body"] = BLOCKED
        show_view(page, "maint")
        assert wait_until(page, "!document.querySelector('.restart .confirm-box')", 5), "the question gone"
        assert page.locator("#restart-btn").is_hidden() and "no service manager" in page.locator("#restart-text").inner_text(), \
            "the reason in its place"
        assert not posts
    finally:
        page.unroute("**/api/v1/restart")


def test_the_reboot_question_goes_when_a_reboot_waits(page, case):
    """The reboot asking with its delay; another browser asks for a
    reboot meanwhile, and Maintenance shown again finds it waiting: the
    question goes, Take the reboot back is offered."""
    stub = RebootStub(page, {"available": True, "by": "systemd"})
    try:
        show_view(page, "maint")
        page.locator("#reboot-btn").click()
        page.locator(".reboot .confirm-box").wait_for()
        stub.answer = {"available": False, "by": "systemd", "at": iso(600), "blocked": "a reboot is waiting", "begun": False}
        show_view(page, "maint")
        assert wait_until(page, "!document.querySelector('.reboot .confirm-box')", 5), "the question gone"
        assert page.locator("#reboot-cancel").is_visible() and page.locator("#reboot-btn").is_hidden(), "the reboot that waits"
        assert not stub.requests, "nothing sent"
    finally:
        stub.close()


@pytest.mark.parametrize("how", ["sign-out", "expired"])
def test_the_session_end_ends_the_restarts_following(new_context, case, watch, guard, how):
    """The restart asked for and followed, the server not back yet: the
    session ends, and the following has ended with it; a /healthz held
    across the end with a new start, let go, says nothing to the next
    user."""
    page = owner_page(new_context, guard, watch, URL, TOKEN, clock=True)
    posts = []
    stub_restart(page, AVAILABLE, posts)
    health = api_call("GET", "healthz")
    safe_route(page, "**/healthz", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(health)))
    show_view(page, "maint")
    page.locator("#restart-btn").click()
    page.locator(".restart .confirm-yes").click()
    restarting = page.evaluate("t('restart.restarting')")
    assert wait_until(page, "document.getElementById('restart-text').textContent === %s" % json.dumps(restarting), 5), "followed"
    later = json.dumps(dict(health, started_at="2099-01-02T03:04:05Z"))
    checks = []
    hold(page, "**/healthz", "GET", checks, [])
    assert wait_held(page, checks, 8), "the next /healthz asked for"
    across(page, how)
    assert page.locator("#restart-text").inner_text() != restarting, "the following ended with the session"
    with page.expect_response(lambda r: r.url.endswith("/healthz")):
        for r in checks:
            r.fulfill(status=200, content_type="application/json", body=later)
    frames(page)
    n = len(checks)
    page.clock.run_for(5000)
    page.evaluate("api('GET', 'api/v1/me').then(() => 1)")
    assert len(checks) == n, f"no /healthz for the next user ({len(checks) - n})"
    back = page.evaluate("t('restart.back', {time: fmtInstant(new Date(Date.parse('2099-01-02T03:04:05Z')))})")
    assert back not in page.locator("#sys-maint-body").inner_text(), "the late answer says nothing"
    assert len(posts) == 1


@pytest.mark.parametrize("how", ["sign-out", "expired"])
def test_the_session_end_forgets_the_delay_and_the_questions(new_context, case, watch, guard, how):
    """The delay typed and the reboot asking: the session ends, and the
    operator finds no question and nothing typed."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    stub = RebootStub(page, {"available": True, "by": "systemd"})
    show_view(page, "maint")
    page.locator("#reboot-delay").fill("97")
    page.locator("#reboot-btn").click()
    page.locator(".reboot .confirm-box").wait_for()
    across(page, how)
    frames(page)
    assert page.locator(".confirm-box").count() == 0, "no question for the next user"
    # the next user may reboot (/me stubbed): the delay's row is drawn again, with 60
    me = {"mode": "users", "user": {"name": "operator", "role": "operator", "has_password": True}, "role": {"key": "operator"},
          "capabilities": ["read", "preferences", "reboot"], "token": {"kind": "session"}}
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(me)))
    page.evaluate("loadMe()")
    assert wait_until(page, "!document.body.classList.contains('no-reboot')", 5), "the next user may reboot"
    show_view(page, "maint")
    page.locator("#reboot-delay").wait_for(state="visible")
    assert page.locator("#reboot-delay").input_value() == "60", "the delay typed is gone: 60 again"
    assert not stub.requests


def test_maintenance_follows_a_setting_set_elsewhere_while_shown_only(page, case, stream):
    """Another browser sets the refresh's interval (PUT, the server's
    change event): with Rooms shown nothing is read; with Maintenance
    shown the choice shows 1 h and the line that it waits, without a
    reload. (The poll's is read-only: the test server runs with -poll 0,
    and its setting waits for nothing.)"""
    api_call("PUT", "api/v1/settings", {"refresh_interval": None})
    reads = counted(page, "**/api/v1/settings")
    try:
        show_view(page, "tiles")
        mark0 = stream.mark()
        api_call("PUT", "api/v1/settings", {"refresh_interval": "2h"})
        stream.wait("the change", "change", match=settings_change(refresh_interval="2h"), mark=mark0, server=True)
        answered(page)
        assert reads == [], f"no read for a page not shown ({reads})"
        api_call("PUT", "api/v1/settings", {"refresh_interval": None})
        show_view(page, "maint")
        assert wait_until(page, "document.getElementById('interval-refresh').value === '21600'", 5), "6 h"
        api_call("PUT", "api/v1/settings", {"refresh_interval": "1h"})
        assert wait_until(page, "document.getElementById('interval-refresh').value === '3600'", 5), "1 h shown, no reload"
        assert wait_until(page, "document.querySelector('#sys-maint .section-badges').textContent.length > 0", 5), "counted as waiting"
    finally:
        page.unroute("**/api/v1/settings")
        api_call("PUT", "api/v1/settings", {"refresh_interval": None})


def test_the_settings_file_read_again_shows_what_waits_with_nothing_read(page, case):
    """A settings_read event says what waits for the next start: the
    restart's row lists it, and nothing is read for it."""
    posts = []
    stub_restart(page, AVAILABLE, posts)
    try:
        show_view(page, "maint")
        page.locator("#restart-btn").wait_for(state="visible")
        reads = counted(page, "**/api/v1/restart")
        send(page, "settings_read", {"why": "SIGHUP", "changed": ["MASSIMO_TLS"], "at_next_start": ["MASSIMO_TLS"]})
        want = page.evaluate("t('restart.pending', {names: 'MASSIMO_TLS'})")
        assert wait_until(page, f"document.getElementById('restart-pending').textContent === {json.dumps(want)}", 5), "what waits"
        answered(page)
        assert reads == [], f"nothing read ({reads})"
    finally:
        page.unroute("**/api/v1/restart")


def test_a_settings_read_warning_reads_what_the_next_start_refuses(page, case):
    """A settings_read event at warn, a read of the settings file that
    found the next start refuses the file, one that does not parse, reads
    the restart's row, which says the file's error alone; one at info then
    says the start refuses nothing, and the row no longer says it, with
    nothing read (AP ReloadChecks, D8-B)."""
    refused = UNPARSED
    answer = {"body": AVAILABLE}
    safe_route(page, "**/api/v1/restart", lambda route: route.fulfill(status=200, content_type="application/json", body=answer["body"]))
    try:
        show_view(page, "maint")
        page.locator("#restart-btn").wait_for(state="visible")
        answer["body"] = json.dumps(dict(json.loads(AVAILABLE), pending=["MASSIMO_HSTS"], refused=[refused]))
        reads = counted(page, "**/api/v1/restart")
        settings_read(page, "warn", {"why": "the settings file changed", "changed": [], "at_next_start": ["MASSIMO_HSTS"]})
        assert wait_until(page, "document.querySelector('#restart-refused .restart-refusal')?.textContent === %s" % json.dumps(refused["error"]), 5), \
            "the warning read what the next start refuses: the file's error, without a setting"
        assert len(reads) == 1, f"one read of the restart's row ({reads})"
        answer["body"] = json.dumps(dict(json.loads(AVAILABLE), pending=["MASSIMO_HSTS"]))
        settings_read(page, "", {"why": "the settings file changed", "changed": ["MASSIMO_HSTS"], "at_next_start": ["MASSIMO_HSTS"]})
        assert wait_until(page, "document.getElementById('restart-refused').classList.contains('hidden')", 5), "gone at info"
        answered(page)
        assert len(reads) == 1, f"nothing read at info ({reads})"
    finally:
        page.unroute("**/api/v1/restart")


def test_a_refusal_while_both_questions_are_open(page, case):
    """The restart's question and the reboot's open, the focus on the
    reboot's Cancel: a settings_read event at warn, whose read of the
    restart's row finds the next start refuses the file, adds the
    sentence to both texts in place, no box nor Cancel drawn anew, and
    the focus stays; one at info, the focus moved to the restart's
    Cancel, takes the sentence away from both the same way (AP
    StartConfiguration, D10-C)."""
    answer = {"body": AVAILABLE}
    safe_route(page, "**/api/v1/restart", lambda route: route.fulfill(status=200, content_type="application/json", body=answer["body"]))
    stub = RebootStub(page, {"available": True, "by": "systemd"})
    sels = [".restart .confirm-box", ".restart .confirm-no", ".reboot .confirm-box", ".reboot .confirm-no"]

    def texts():
        return page.evaluate("[...document.querySelectorAll('.restart .confirm-text, .reboot .confirm-text')].map((e) => e.textContent)")

    def focused(sel):
        return page.evaluate("(s) => document.activeElement === document.querySelector(s)", sel)
    try:
        show_view(page, "maint")
        page.locator("#restart-btn").wait_for(state="visible")
        page.locator("#reboot-btn").wait_for(state="visible")
        plain = page.evaluate("[t('restart.confirm'), tn('reboot.confirm', 60)]")
        sentence = page.evaluate("t('restart.confirmRefused')")
        page.locator("#restart-btn").click()
        page.locator("#reboot-btn").click()
        assert texts() == plain, f"both questions open, without the sentence ({texts()})"
        assert focused(".reboot .confirm-no"), "the focus on the reboot's Cancel"
        mark(page, sels)

        answer["body"] = json.dumps(dict(json.loads(AVAILABLE), refused=[UNPARSED]))
        settings_read(page, "warn", {"why": "the settings file changed", "changed": [], "at_next_start": []})
        want = [q + " " + sentence for q in plain]
        assert wait_until(page, f"JSON.stringify([...document.querySelectorAll('.restart .confirm-text, .reboot .confirm-text')].map((e) => e.textContent)) === JSON.stringify({json.dumps(want)})", 5), \
            f"the sentence in both questions ({texts()})"
        assert replaced(page, sels) == [], "the boxes and their Cancels kept"
        assert focused(".reboot .confirm-no"), "the focus still on the reboot's Cancel"

        page.locator(".restart .confirm-no").focus()
        answer["body"] = AVAILABLE
        settings_read(page, "", {"why": "the settings file changed", "changed": [], "at_next_start": []})
        assert wait_until(page, f"JSON.stringify([...document.querySelectorAll('.restart .confirm-text, .reboot .confirm-text')].map((e) => e.textContent)) === JSON.stringify({json.dumps(plain)})", 5), \
            f"the sentence gone from both ({texts()})"
        assert replaced(page, sels) == [], "the boxes and their Cancels kept again"
        assert focused(".restart .confirm-no"), "the focus still on the restart's Cancel"
        page.locator(".restart .confirm-no").click()
        page.locator(".reboot .confirm-no").click()
        assert page.locator(".restart .confirm-box, .reboot .confirm-box").count() == 0, "both taken back"
        assert not stub.requests, "nothing asked for"
    finally:
        stub.close()
        page.unroute("**/api/v1/restart")


@pytest.mark.clock
def test_a_reboot_event_while_this_pages_reboot_waits(page, case):
    """This page asks for a reboot in 30 s; a reboot event meanwhile reads
    the reboot's state, which says the same wait: the row still waits,
    with Take the reboot back, and the page follows the computer at the
    reboot's time, as the wait set it."""
    stub = RebootStub(page, {"available": True, "by": "systemd"})
    try:
        show_view(page, "maint")
        page.locator("#reboot-delay").fill("30")
        page.locator("#reboot-btn").click()
        page.locator(".reboot .confirm-yes").click()
        page.locator("#reboot-cancel").wait_for(state="visible")
        waiting = reboot_text(page)
        reads = counted(page, "**/api/v1/reboot")
        send(page, "reboot", {"by": "systemd", "at": stub.answer["at"]})
        answered(page)
        assert len(reads) == 1, f"the event read the reboot's state once ({reads})"
        assert reboot_text(page) == waiting and page.locator("#reboot-cancel").is_visible(), "still waiting"
        page.clock.run_for(31_000)
        rebooting = page.evaluate("t('reboot.rebooting')")
        assert wait_until(page, "document.getElementById('reboot-text').textContent === %s" % json.dumps(rebooting), 5), \
            "followed at the reboot's time"
    finally:
        stub.close()


def test_an_event_keeps_a_question_and_takes_back_one_whose_reboot_waits(page, case):
    """The restart and the reboot asking, a delay typed: a change of the
    announcement and a reboot event read again, and the restart's
    question and the delay stay; once a reboot waits (asked for
    elsewhere), the reboot event takes the reboot's question back."""
    posts = []
    stub_restart(page, AVAILABLE, posts)
    stub = RebootStub(page, {"available": True, "by": "systemd"})
    try:
        show_view(page, "maint")
        page.locator("#reboot-delay").fill("77")
        page.locator("#restart-btn").click()
        page.locator(".restart .confirm-box").wait_for()
        page.locator("#reboot-btn").click()
        page.locator(".reboot .confirm-box").wait_for()
        sels = [".restart .confirm-no", ".reboot .confirm-no", "#reboot-delay"]
        mark(page, sels)
        reads = counted(page, "**/api/v1/reboot")
        send(page, "change", {"action": "settings.update", "set": {"announce": "on"}})
        send(page, "reboot", {"by": "systemd"})
        # the reboot's read waits behind the settings' read in the card's burst reader: the read waited for,
        # then answered, which follows a read started after another's answer, for any read after it
        assert wait_reads(page, reads, 1), f"the reboot read ({reads})"
        answered(page)
        assert len(reads) == 1, f"the reboot read ({reads})"
        frames(page)
        assert replaced(page, sels) == [], f"the questions and the delay stay: {replaced(page, sels)}"
        assert page.locator("#reboot-delay").input_value() == "77"
        stub.answer = {"available": False, "by": "systemd", "at": iso(600), "blocked": "a reboot is waiting", "begun": False}
        send(page, "reboot", {"by": "systemd", "at": stub.answer["at"]})
        assert wait_until(page, "!document.querySelector('.reboot .confirm-box')", 5), "the reboot's question gone"
        assert page.locator(".restart .confirm-box").count() == 1, "the restart's stays"
        assert not posts and not stub.requests
    finally:
        stub.close()
        page.unroute("**/api/v1/restart")


def restart_route(page, answer):
    """GET /api/v1/restart answered with answer["body"], a dict, at every read; a POST answered 202 and counted in answer["posts"]."""
    answer.setdefault("posts", [])

    def handle(route):
        if route.request.method == "POST":
            answer["posts"].append(1)
            route.fulfill(status=202, content_type="application/json", body='{"ok": true, "by": "systemd"}')
            return
        route.fulfill(status=200, content_type="application/json", body=json.dumps(answer["body"]))
    safe_route(page, "**/api/v1/restart", handle)


def test_maintenance_reads_again_at_a_resync_while_shown_only(page, case):
    """A resync of the stream, how the page learns of a restart done
    elsewhere: with Rooms shown nothing is read; with Maintenance shown
    the restart's row is read once and shows what waits now."""
    answer = {"body": {"available": True, "by": "systemd", "pending": ["MASSIMO_LISTEN"]}}
    restart_route(page, answer)
    reads = counted(page, "**/api/v1/restart")
    try:
        show_view(page, "tiles")
        send(page, "resync")
        answered(page)
        assert reads == [], f"no read for a page not shown ({reads})"
        show_view(page, "maint")
        assert wait_until(page, "document.getElementById('restart-pending').textContent.includes('MASSIMO_LISTEN')", 5), "what waits"
        n = len(reads)
        answer["body"] = {"available": True, "by": "systemd", "pending": ["MASSIMO_TLS"]}
        send(page, "resync")
        assert wait_until(page, "document.getElementById('restart-pending').textContent.includes('MASSIMO_TLS')", 5), \
            "what waits now, no reload"
        answered(page)
        assert len(reads) == n + 1, f"one read ({len(reads) - n})"
    finally:
        page.unroute("**/api/v1/restart")


def test_a_resync_while_this_page_follows_its_restart_reads_no_row(page, case):
    """The restart asked for and followed by /healthz, the server not back
    yet: a resync reads the restart's row not at all, the following's
    /healthz alone."""
    answer = {"body": {"available": True, "by": "systemd"}}
    restart_route(page, answer)
    health = api_call("GET", "healthz")
    safe_route(page, "**/healthz", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(health)))
    try:
        show_view(page, "maint")
        page.locator("#restart-btn").click()
        page.locator(".restart .confirm-yes").click()
        restarting = page.evaluate("t('restart.restarting')")
        assert wait_until(page, "document.getElementById('restart-text').textContent === %s" % json.dumps(restarting), 5), "followed"
        reads = counted(page, "**/api/v1/restart")
        send(page, "resync")
        page.evaluate("api('GET', 'api/v1/me').then(() => 1)")
        frames(page)
        assert reads == [], f"the row not read while the restart is followed ({reads})"
        assert page.locator("#restart-text").inner_text() == restarting, "still following"
        assert len(answer["posts"]) == 1
    finally:
        page.unroute("**/healthz")
        page.unroute("**/api/v1/restart")


def test_an_intervals_own_change_event_reads_nothing_behind_its_back(page, case, stream):
    """The refresh's interval chosen, its PUT held; the change event of a
    PUT comes meanwhile: it reads the settings not at all while the
    page's own request is under way, and 1 h stays chosen throughout.
    The server's own change event of the reset is waited for before the
    reads are counted, so that it adds none. (The poll's is read-only:
    the test server runs with -poll 0.)"""
    m = stream.mark()
    api_call("PUT", "api/v1/settings", {"refresh_interval": None})
    stream.wait("the reset's change", "change", match=settings_change(refresh_interval=None), mark=m, server=True)
    held = []
    try:
        show_view(page, "maint")
        sel = page.locator("#interval-refresh")
        assert wait_until(page, "document.querySelector('#interval-refresh-text').textContent.length > 0", 5), "the line is filled"
        reads = counted(page, "**/api/v1/settings")
        page.route("**/api/v1/settings", lambda route: held.append(route) if route.request.method == "PUT" else route.fallback())
        page.evaluate("""() => { window.__seen = []; const sel = document.getElementById('interval-refresh');
          window.__watch = setInterval(() => window.__seen.push(sel.value), 5); }""")
        sel.select_option("3600")
        assert wait_held(page, held), "the PUT sent"
        send(page, "change", {"action": "settings.update", "set": {"refresh_interval": "6h"}})
        answered(page, held=held)
        assert reads == [], f"the event reads nothing while the PUT runs ({reads})"
        for r in held:
            r.fallback()
        assert wait_until(page, "!document.querySelector('#interval-refresh-msg').classList.contains('hidden')", 5), "saved"
        frames(page)
        seen = page.evaluate("() => { clearInterval(window.__watch); return window.__seen; }")
        after = seen[seen.index("3600"):] if "3600" in seen else []
        assert after and "21600" not in after, f"6 h never shown after 1 h was chosen ({seen[-20:]})"
    finally:
        page.unroute("**/api/v1/settings")
        api_call("PUT", "api/v1/settings", {"refresh_interval": None})


def test_another_setting_changed_during_the_own_put_is_read_after_it(page, case, stream):
    """The refresh's interval chosen, its PUT held; another browser sets
    the announcement meanwhile (its change event): nothing is read while
    the PUT runs, one read follows it, and the announcement shows On. The
    server's own change event of each write is waited for, the page's
    own PUT's before the page has its answer, so that every one of them
    has come while the PUT runs and the reads are counted after them: an
    event that came later would read once more, as the page does for a
    change it learns of after its own request. (The poll's interval is
    read-only: the test server runs with -poll 0.)"""
    m = stream.mark()
    api_call("PUT", "api/v1/settings", {"refresh_interval": None, "announce": None})
    stream.wait("the reset's change", "change", match=settings_change(refresh_interval=None, announce=None), mark=m, server=True)
    held = []
    try:
        show_view(page, "maint")
        assert wait_until(page, "document.querySelector('#announce-text').textContent.length > 0", 5), "the settings read"
        assert page.locator("#announce-mode").input_value() == "auto"
        reads = counted(page, "**/api/v1/settings")
        page.route("**/api/v1/settings", lambda route: held.append(route) if route.request.method == "PUT" else route.fallback())
        page.locator("#interval-refresh").select_option("3600")
        assert wait_held(page, held), "the PUT sent"
        m = stream.mark()
        api_call("PUT", "api/v1/settings", {"announce": "on"})  # the other browser
        stream.wait("the other browser's change", "change", match=settings_change(announce="on"), mark=m, server=True)
        answered(page, held=held)
        assert reads == [], f"nothing read while the PUT runs ({reads})"
        m = stream.mark()
        answers = [r.fetch() for r in held]  # at the server, the page still without its answer
        stream.wait("the page's own change", "change", match=settings_change(refresh_interval="1h"), mark=m, server=True)
        for r, a in zip(held, answers):
            r.fulfill(response=a)
        assert wait_until(page, "document.getElementById('announce-mode').value === 'on'", 5), "the announcement read after the PUT"
        answered(page)
        assert len(reads) == 1, f"one read after it ({len(reads)})"
    finally:
        page.unroute("**/api/v1/settings")
        api_call("PUT", "api/v1/settings", {"refresh_interval": None, "announce": None})


# --- System info -------------------------------------------------------------

PROXIED_BOX = {"version": "massimo v0.0.1-test (go1.26.0, linux/arm64)", "started_at": "2026-11-13T08:00:00+01:00",
               "install": "other", "os": "linux", "arch": "arm64", "hostname": "massimo-box", "fqdn": "massimo-box.example.net",
               "client_ip": "192.0.2.20", "now": "2026-11-13T09:00:00+01:00", "time_zone": "UTC", "interfaces": [],
               "cube_proxies": [{"serial": "KEQ0000100", "addr": "192.0.2.5:62910", "listening": True}],
               "proxy": {"proto": "https", "host": "massimo.example.net", "port": 443}}


def test_system_info_keeps_its_folds_a_copy_and_a_selection(page, case):
    """System info with its folds open and
    the host's name selected: the certificate read again (every 2 s while
    it is being read) keeps the name selected; a proxy's serial copied,
    the page shown again keeps it "copied" until it fades; the copy
    icons, the link, the folds and their lists are the elements they
    were, and the folds are open."""
    reads = []

    def answer(route):
        reads.append(1)
        route.fulfill(status=200, content_type="application/json", body=json.dumps(PROXIED_BOX))
    safe_route(page, "**/api/v1/system", answer)
    try:
        show_view(page, "info")
        page.locator("#info-proxies .copy-icon").first.wait_for()
        page.locator("#store-info summary").click()
        page.locator("#config-info summary").click()
        page.locator("#store-info-body .kv-rows li").first.wait_for()
        sels = ["#info-proxies .copy-icon", "#info-version .copy-icon", "#info-tls a", "#store-info > summary", "#config-info > summary",
                "#store-info-body dl", "#config-info-body dl", "#info-host strong"]
        mark(page, sels)
        select_text(page, "#info-host")
        want = selected(page)
        assert want, "the name selected"
        n = len(reads)
        assert wait_reads(page, reads, n + 1), "the certificate read again"
        assert selected(page) == want, "the name still selected"
        icon = page.locator("#info-proxies .copy-icon").first
        icon.focus()
        page.keyboard.press("Enter")
        assert wait_until(page, "document.querySelector('#info-proxies .copy-icon').classList.contains('copied')", 1), "copied"
        n = len(reads)
        show_view(page, "info")
        assert wait_reads(page, reads, n + 1), "the page read again"
        assert icon.evaluate("(b) => b.classList.contains('copied')"), "still copied, until it fades"
        assert replaced(page, sels) == [], f"the same elements after the reads: {replaced(page, sels)}"
        assert page.evaluate("document.getElementById('store-info').open && document.getElementById('config-info').open"), "the folds open"
    finally:
        page.unroute("**/api/v1/system")
        show_view(page, "tiles")


@pytest.mark.parametrize("how", ["sign-out", "expired"])
def test_the_session_end_forgets_system_info(new_context, case, watch, guard, how):
    """System info shown with the settings in effect open, a proxy's
    certificate being read: the session ends, and the operator finds no
    fact, no variable of the first; the reading held across the end, let
    go, shows nothing, and the clock run past the readings' pace brings no
    more."""
    page = owner_page(new_context, guard, watch, URL, TOKEN, clock=True)
    safe_route(page, "**/api/v1/system", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(PROXIED_BOX)))
    show_view(page, "info")
    page.locator("#config-info summary").click()
    assert wait_until(page, "document.getElementById('config-info-body').textContent.includes('MASSIMO_API_TOKEN')", 5), "the variables shown"
    held = []
    hold(page, "**/api/v1/system", "GET", held, [])
    assert wait_held(page, held, 8), "the certificate read again"
    across(page, how)
    with page.expect_response(lambda r: r.url.endswith("/api/v1/system")):
        for r in held:
            r.fallback()
    frames(page)
    n = len(held)
    page.clock.run_for(5000)
    answered(page, held=held)
    assert len(held) == n, f"no reading of the certificate for the next user ({len(held) - n})"
    assert "MASSIMO_API_TOKEN" not in page.locator("#config-info-body").inner_text(), "no variable of the first"
    assert page.locator("#info-client").inner_text() == "–" and "massimo-box" not in page.locator("#info-host").inner_text(), "no fact"


def test_system_info_reads_again_at_a_resync_while_shown_only(page, case):
    """A resync of the stream (how the page learns of a restart) with
    Rooms shown reads nothing; with System info shown it reads the facts
    and the settings in effect once, the new start shown, the fold left
    open."""
    box = dict(PROXIED_BOX, proxy=None)
    box.pop("proxy")
    safe_route(page, "**/api/v1/system", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(box)))
    facts = counted(page, "**/api/v1/system")
    configs = counted(page, "**/api/v1/config")
    try:
        show_view(page, "tiles")
        send(page, "resync")
        answered(page)
        assert facts == [] and configs == [], "no read for a page not shown"
        show_view(page, "info")
        page.locator("#config-info summary").click()
        assert wait_until(page, "document.getElementById('info-client').textContent === '192.0.2.20'", 5), "the facts"
        n, k = len(facts), len(configs)
        box["started_at"] = "2026-11-13T09:30:00+01:00"
        send(page, "resync")
        # the new start's own data, its time in the age's data-at: the cell's text holds the age's exact time,
        # hidden (Age), which inner_text leaves out and textContent does not, so the two never matched
        at = str(page.evaluate("Date.parse('2026-11-13T09:30:00+01:00')"))
        assert wait_until(page, "document.querySelector('#info-started .age')?.dataset.at === %s" % json.dumps(at), 5), "the new start"
        answered(page)
        assert len(facts) == n + 1 and len(configs) == k + 1, f"one read each ({len(facts) - n}, {len(configs) - k})"
        assert page.evaluate("document.getElementById('config-info').open"), "the fold open"
    finally:
        page.unroute("**/api/v1/system")
        show_view(page, "tiles")


def test_system_info_reads_its_facts_when_a_cube_changes(page, case):
    """A Cube's proxy changed (change, cube.update) with System info shown:
    the facts are read once and the proxy's new address is listed."""
    box = dict(PROXIED_BOX, proxy=None)
    box.pop("proxy")
    safe_route(page, "**/api/v1/system", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(box)))
    facts = counted(page, "**/api/v1/system")
    try:
        show_view(page, "info")
        assert wait_until(page, "document.getElementById('info-proxies').textContent.includes('192.0.2.5:62910')", 5), "the proxy"
        n = len(facts)
        box["cube_proxies"] = [{"serial": "KEQ0000100", "addr": "192.0.2.6:62910", "listening": True}]
        send(page, "change", {"action": "cube.update", "target": "cube:KEQ0000100"})
        assert wait_until(page, "document.getElementById('info-proxies').textContent.includes('192.0.2.6:62910')", 5), \
            "the proxy's new address"
        answered(page)
        assert len(facts) == n + 1, f"one read ({len(facts) - n})"
    finally:
        page.unroute("**/api/v1/system")
        show_view(page, "tiles")


def test_the_session_end_between_two_readings_reads_no_more(new_context, case, watch, guard):
    """A proxy's certificate being read every 2 s: the user signs out
    right after a reading's answer, while the next one waits on its
    timer (logout, without the menu's clicks, so that the timer is still
    to come); the clock run past it sends no read of the system. An
    expired session is no end of it until another user signs in."""
    page = owner_page(new_context, guard, watch, URL, TOKEN, clock=True)
    safe_route(page, "**/api/v1/system", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(PROXIED_BOX)))
    show_view(page, "info")
    page.locator("#info-proxies .copy-icon").first.wait_for()
    facts = counted(page, "**/api/v1/system")
    with page.expect_response(lambda r: r.url.endswith("/api/v1/system")):
        page.clock.run_for(2100)
    answered(page)  # the answer handled: the next reading's timer runs, the line still "being read"
    assert PENDING[case.lang] in page.locator("#info-tls").inner_text(), "the certificate being read"
    page.evaluate("() => { logout(); }")  # the session ends while the next reading's timer runs
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "signed out"
    n = len(facts)
    page.clock.run_for(5000)
    answered(page)
    assert len(facts) == n, f"no reading after the end ({len(facts) - n})"


def certificate_routes(page, box, read):
    """GET /api/v1/system answered with box, its proxy's certificate read where read["now"] is set; the reads counted."""
    asked = []
    cert = {"source": "proxy", "names": ["massimo.example.net"], "not_after": "2027-02-10T12:00:00Z", "fingerprint": "00",
            "self_signed": False, "issuer": "Example CA", "trusted": True, "checked_at": "2026-11-13T09:50:02+01:00"}

    def answer(route):
        asked.append(1)
        body = dict(box, proxy=dict(box["proxy"], certificate=cert)) if read["now"] else box
        route.fulfill(status=200, content_type="application/json", body=json.dumps(body))
    safe_route(page, "**/api/v1/system", answer)
    return asked


@pytest.mark.clock
def test_a_resync_finds_the_certificate_being_read_and_reads_it(page, case):
    """The certificate read; a resync after a restart finds it being read
    again on the new server: "being read…", and the readings every 2 s
    until it is read."""
    read = {"now": True}
    asked = certificate_routes(page, PROXIED_BOX, read)
    try:
        show_view(page, "info")
        assert wait_until(page, "document.getElementById('info-tls').textContent.includes('Example CA')", 5), "read"
        read["now"] = False
        send(page, "resync")
        assert wait_until(page, "document.getElementById('info-tls').textContent.includes(%r)" % PENDING[case.lang], 5), "being read…"
        n = len(asked)
        page.clock.run_for(2100)
        answered(page)
        assert len(asked) == n + 1, f"read again 2 s later ({len(asked) - n})"
        read["now"] = True
        page.clock.run_for(2100)
        assert wait_until(page, "document.getElementById('info-tls').textContent.includes('Example CA')", 5), "read again, shown"
    finally:
        page.unroute("**/api/v1/system")
        show_view(page, "tiles")


@pytest.mark.clock
def test_a_resync_after_the_readings_gave_up_reads_anew(page, case):
    """The certificate never read: after its 15 readings the line says it
    could not be read; a resync finds it being read on the new server:
    "being read…" again, and the readings start anew."""
    read = {"now": False}
    asked = certificate_routes(page, PROXIED_BOX, read)
    try:
        show_view(page, "info")
        for _ in range(17):
            page.clock.run_for(2100)
            answered(page)
        assert wait_until(page, "document.getElementById('info-tls').textContent.includes(%r)" % UNREAD[case.lang], 5), "gave up"
        send(page, "resync")
        assert wait_until(page, "document.getElementById('info-tls').textContent.includes(%r)" % PENDING[case.lang], 5), \
            "being read… again"
        n = len(asked)
        page.clock.run_for(2100)
        answered(page)
        assert len(asked) == n + 1, f"the readings start anew ({len(asked) - n})"
    finally:
        page.unroute("**/api/v1/system")
        show_view(page, "tiles")
