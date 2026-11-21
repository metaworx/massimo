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

from conftest import TOKEN, URL
from helpers import frames, hold, mark, owner_page, replaced, safe_route, show_view, update_json, wait_held, wait_until
from test_admin_updates import across
from test_unsaved import expire
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
    """Waits until reads has n entries, and the last answer has been drawn."""
    end = time.time() + seconds
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
    header's check every half hour (loadUpdate) and Maintenance shown
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
        page.evaluate("loadUpdate(false)")
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
        page.evaluate("loadUpdate(false)")
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
    answered(page)
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


def answered(page):
    """A request of the page's own answered: one that the page sent before it has reached its route."""
    page.evaluate("fetch('healthz').then((r) => r.status)")
    frames(page)


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
        answered(page)
        assert len(held) == 1, f"one read in flight ({len(held)})"
        held.pop().fallback()
        assert wait_held(page, held), "one more after it"
        answered(page)
        assert len(held) == 1, f"one read for the four ({len(held)})"
        held.pop().fallback()
        answered(page)
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
        answered(page)  # the first read's answer handled: the second would have been asked for by now
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
        answered(page)
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
        page.evaluate("loadUpdate(false)")
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
        page.evaluate("loadUpdate(false)")  # the header's check: the helper at work, the reading begins
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
        answered(page)
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
    answered(page)
    assert not held, f"no read for the session before ({len(held)})"


# --- System info ---