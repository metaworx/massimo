# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The log as a part (AP UiParts, Block 11): its lines follow the server
in place. A new line, every line drawn again (the rooms came, the
language changed), the log loaded again and its unseen problems read
again keep what a user holds in it: a command's detail open and the focus
on its button, a selection of text, the focus on Mark seen, the search
typed and the choices, a choice's options; a line that came live keeps
its element when a load brings the same event. At a sign-out, and for
another user after an expired session, an answer still on its way of
each of the log's reads reaches nothing of the next user; at the
stream's resync a Cube's log shown reads on from its newest line. The log's reads of the
Cube are answered by the test (StubLog), and the server's stream lines
kept out (isolate_log), so that the lines are the test's own."""
import json

import pytest

from conftest import TOKEN, URL
from helpers import LOG_ALERT_A as ALERT_A
from helpers import LOG_ALERT_B as ALERT_B
from helpers import LOG_CMD as CMD
from helpers import LOG_CMD_DETAIL as DETAIL
from helpers import LOG_CMD_LINE as CMD_LINE
from helpers import LOG_FIRST as FIRST
from helpers import LOG_TEXT_LINE as TEXT_LINE
from helpers import StubLog, frames, isolate_log, mark, owner_page, replaced, safe_route, show_view, sign_out, wait_until
from helpers import log_event as ev
from helpers import log_lines as lines
from helpers import stub_log_command as stub_command
from test_log_alerts import alerts_route
from test_login import sign_in
from test_unsaved import expire

def open_log(page, n=20, hash_="#log"):
    """The log shown with the test's lines alone; the stub."""
    show_view(page, "log")
    isolate_log(page)
    if hash_ != "#log":
        page.evaluate("(h) => { location.hash = h; }", hash_)
        assert wait_until(page, "location.hash === %s && !logState.loading" % json.dumps(hash_), 5), "the log's address"
    stub = StubLog(page, lines(n))
    stub_command(page)
    page.evaluate("loadLog()")
    frames(page)
    assert wait_until(page, "!!document.querySelector(\"#log-lines .log-line[data-id='%d']\")" % CMD_LINE, 5), "the test's lines"
    return stub


NEW_LINE = """() => appendLogEvent({ kind: 'hello', source: 'cube', level: 'info', time: new Date().toISOString(),
  summary: { firmware: 'new-line', duty_cycle_pct: 1, free_memory_slots: 1 } })"""

# What comes in while a user holds something in the log: a new line, every
# line drawn again (renderLog, as after the rooms or a language), the rooms
# read again, the log loaded again, its unseen problems read again.
ACTS = (("a new line on top", NEW_LINE), ("every line drawn again", "renderLog()"), ("the rooms read again", "loadRooms()"),
        ("the log loaded again", "loadLog()"), ("its problems read again", "loadLogAlerts()"))


def row(n):
    return "#log-lines .log-line[data-id='%d']" % n


def focused(page, sel):
    return page.evaluate("(s) => document.activeElement === document.querySelector(s)", sel)


def test_a_commands_detail_and_its_focus_stay(page, case):
    """A command's detail opened by its button, the focus on the button:
    through a new line, every line drawn again, the rooms, the log loaded
    again and its problems read again, the line, its button and its detail
    are the elements they were, the detail open and the focus on the
    button. The plain page built every line again for each of them but a
    new line, and for its own toggle (the first analysis's case)."""
    open_log(page)
    btn = page.locator(row(CMD_LINE) + " .log-cmd")
    btn.click()
    page.locator(row(CMD_LINE) + " .log-cmd-detail .log-cmd-head").wait_for()
    assert focused(page, row(CMD_LINE) + " .log-cmd"), "the button keeps the focus as it opens the detail"
    sels = [row(CMD_LINE), row(CMD_LINE) + " .log-cmd", row(CMD_LINE) + " .log-cmd-detail"]
    mark(page, sels)
    for what, call in ACTS:
        page.evaluate(call)
        frames(page)
        assert replaced(page, sels) == [], f"{what}: the same line, button and detail ({replaced(page, sels)})"
        assert btn.get_attribute("aria-expanded") == "true", f"{what}: the detail open"
        assert focused(page, row(CMD_LINE) + " .log-cmd"), f"{what}: the focus on its button"
    btn.click()
    assert wait_until(page, "!document.querySelector(%s)" % json.dumps(row(CMD_LINE) + " .log-cmd-detail"), 5), "closed again"
    show_view(page, "tiles")


SELECTED = """(s) => { const el = document.querySelector(s); const r = document.createRange(); r.selectNodeContents(el);
  const sel = getSelection(); sel.removeAllRanges(); sel.addRange(r); return sel.toString(); }"""
STILL_SELECTED = """(s) => { const sel = getSelection(); const el = document.querySelector(s);
  return sel.rangeCount === 1 && !!el && el.contains(sel.anchorNode) && sel.toString(); }"""


def test_a_selection_of_text_stays(page, case):
    """A line's text selected, as a user selects it to copy it: through
    the same as above, the selection stays on that text. The plain page
    built the lines again, and the selection went with them."""
    open_log(page)
    sel = row(TEXT_LINE) + " .log-text"
    want = page.evaluate(SELECTED, sel)
    assert "selected-firmware" in want, f"the line's text selected: {want!r}"
    for what, call in ACTS:
        page.evaluate(call)
        frames(page)
        assert page.evaluate(STILL_SELECTED, sel) == want, f"{what}: the selection stays"
    page.evaluate("getSelection().removeAllRanges()")
    show_view(page, "tiles")


def test_mark_seen_keeps_the_focus_while_another_line_changes(page, case):
    """Two lines are problems; the focus on the first's Mark seen. The
    second's turns from a warning to an error: only that line is drawn
    again, the first and its button stay the elements they were, with the
    focus. The plain page drew every line again where a level changed."""
    open_log(page)
    alerts = [{"id": ALERT_A, "kind": "hello", "level": "warn", "time": "2026-11-20T08:00:17Z"},
              {"id": ALERT_B, "kind": "hello", "level": "warn", "time": "2026-11-20T08:00:16Z"}]
    alerts_route(page, alerts)
    try:
        page.evaluate("loadLogAlerts()")
        assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.log-alert').length === 2", 5), "both lines highlighted"
        page.locator(row(ALERT_A) + " .log-mark").focus()
        sels = [row(ALERT_A), row(ALERT_A) + " .log-mark"]
        mark(page, sels)
        alerts[1]["level"] = "error"
        page.evaluate("loadLogAlerts()")
        assert wait_until(page, "document.querySelector(%s).classList.contains('log-alert-bad')" % json.dumps(row(ALERT_B)), 5), "the second line red"
        assert replaced(page, sels) == [], f"the first line and its Mark seen the same ({replaced(page, sels)})"
        assert focused(page, row(ALERT_A) + " .log-mark"), "the focus on its Mark seen"
    finally:
        page.unroute("**/api/v1/log/alerts*")
        page.evaluate("loadLogAlerts()")
    show_view(page, "tiles")


def test_the_search_and_the_choices_stay(page, case):
    """The search typed and a kind chosen: through a new line, the log
    loaded again, another page and the log again, the search holds what
    was typed and the choice the kind, and the lines drawn match them. A
    forward guard: the plain page kept its head, which it never built
    again, so that this passes there too."""
    open_log(page)
    page.locator("#log-kind").select_option("hello")
    assert wait_until(page, "logState.kind === 'hello' && !logState.loading", 5), "the kind chosen, its page in"
    page.locator("#log-filter").fill("selected-firm")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line').length === 1", 5), "the line that matches alone"
    for what, call in (("a new line on top", NEW_LINE), ("the log loaded again", "loadLog()")):
        page.evaluate(call)
        frames(page)
        assert page.locator("#log-filter").input_value() == "selected-firm", f"{what}: the search kept"
        assert page.locator("#log-kind").input_value() == "hello", f"{what}: the kind kept"
    show_view(page, "tiles")
    show_view(page, "log")
    assert wait_until(page, "!logState.loading", 5), "the log loaded again as it is shown"
    frames(page)
    assert page.locator("#log-filter").input_value() == "selected-firm", "another page and back: the search kept"
    assert page.locator("#log-kind").input_value() == "hello", "and the kind"
    assert page.evaluate("[...document.querySelectorAll('#log-lines .log-line')].map((l) => l.dataset.id)") == [str(TEXT_LINE)], \
        "the lines drawn match them"
    page.locator("#log-clear").click()
    assert wait_until(page, "logState.kind === '' && !logState.filter", 5), "Clear"
    show_view(page, "tiles")


def test_a_choices_options_stay(page, case):
    """The focus on the kind's choice: the rooms read again (which made the
    plain page fill every choice anew) and the Cubes read again keep its
    options and the device choice's the elements they were, and the focus."""
    open_log(page)
    sels = ["#log-kind", "#log-kind optgroup:first-of-type option:first-child", "#log-device option:first-child", "#log-device optgroup option"]
    page.locator("#log-kind").focus()
    mark(page, sels)
    for what, call in (("the rooms read again", "loadRooms()"), ("the Cubes read again", "loadCubes()"), ("every line drawn again", "renderLog()")):
        page.evaluate(call)
        frames(page)
        assert replaced(page, sels) == [], f"{what}: the same options ({replaced(page, sels)})"
        assert focused(page, "#log-kind"), f"{what}: the focus on the choice"
    show_view(page, "tiles")


OBSERVE = """() => { const box = document.getElementById('log-lines'); window.__logMuts = { added: 0, removed: 0 };
  window.__logObs = new MutationObserver((ms) => { for (const m of ms) { window.__logMuts.added += m.addedNodes.length;
    window.__logMuts.removed += m.removedNodes.length; } });
  window.__logObs.observe(box, { childList: true }); }"""
TAKEN = "() => { window.__logObs.takeRecords(); const m = window.__logMuts; window.__logMuts = { added: 0, removed: 0 }; return m; }"


def test_a_new_line_adds_one_element(page, case):
    """With 200 lines shown, a page (every event, range all): a new line adds one
    element and removes none; every line drawn again, and the log loaded
    again with the same events, add and remove none (the elements of the
    same events are kept). The plain page added one for the new line too,
    and built all 200 again for the other two."""
    open_log(page, 200, "#log?range=all")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line').length === 200", 5), "200 lines (a page)"
    page.evaluate(OBSERVE)
    try:
        page.evaluate(NEW_LINE)
        frames(page)
        assert page.evaluate(TAKEN) == {"added": 1, "removed": 0}, "a new line: one element added, none removed"
        page.evaluate("renderLog()")
        frames(page)
        assert page.evaluate(TAKEN) == {"added": 0, "removed": 0}, "every line drawn again: none added or removed"
        page.evaluate("loadLog()")
        frames(page)
        # the new line, live, is not in the page the stub answers: it goes
        assert page.evaluate(TAKEN) == {"added": 0, "removed": 1}, "the log loaded again: the 200 kept, the live line gone"
    finally:
        page.evaluate("window.__logObs.disconnect()")
    page.evaluate("location.hash = '#log'")
    show_view(page, "tiles")


def test_a_live_line_keeps_its_element_when_a_load_brings_it(page, case):
    """A line that came live, without an id, and is then in a page the log
    loads, with its id: its element is the one it was, the user's focus on
    it kept (UiParts Q5-A), and a problem of that id highlights it. The
    plain page built every line again."""
    stub = open_log(page)
    at = "2026-11-20T09:00:00.000Z"
    page.evaluate("""(at) => appendLogEvent({ kind: 'hello', source: 'cube', level: 'info', time: at,
      summary: { firmware: 'live-then-loaded', duty_cycle_pct: 1, free_memory_slots: 1 } })""", at)
    frames(page)
    live = "#log-lines .log-line:first-child"
    assert "live-then-loaded" in page.locator(live + " .log-text").inner_text(), "the live line on top"
    page.evaluate("(s) => { const el = document.querySelector(s); el.tabIndex = -1; el.focus(); el.__mark = 'live'; }", live)
    stub.lines = stub.lines + [dict(ev(40), time=at, summary={"firmware": "live-then-loaded", "duty_cycle_pct": 1, "free_memory_slots": 1})]
    page.evaluate("loadLog()")
    frames(page)
    assert page.evaluate("document.querySelector(%s).dataset.id" % json.dumps(live)) == str(FIRST + 40), "the loaded line, with its id, on top"
    assert page.evaluate("document.querySelector(%s).__mark === 'live'" % json.dumps(live)), "the element the live line had"
    assert page.evaluate("document.activeElement === document.querySelector(%s)" % json.dumps(live)), "the focus on it kept"
    alerts_route(page, [{"id": FIRST + 40, "kind": "hello", "level": "warn", "time": at}])
    try:
        page.evaluate("loadLogAlerts()")
        assert wait_until(page, "document.querySelector(%s).classList.contains('log-alert-warn')" % json.dumps(live), 5), \
            "a problem of its id highlights it, the id it came to have"
    finally:
        page.unroute("**/api/v1/log/alerts*")
        page.evaluate("loadLogAlerts()")
    show_view(page, "tiles")



# The box made small by a rule of the page's own style sheet (the policy
# takes no style element of the test's), so that it scrolls at every width.
SMALL_BOX = """() => { const sheet = [...document.styleSheets].find((s) => (s.href || '').endsWith('style.css'));
  window.__logRule = sheet.insertRule('#log-lines { max-height: 160px !important; }', sheet.cssRules.length); }"""
NO_SMALL_BOX = """() => { const sheet = [...document.styleSheets].find((s) => (s.href || '').endsWith('style.css'));
  if (window.__logRule !== undefined) sheet.deleteRule(window.__logRule); window.__logRule = undefined; }"""
# The first line in the box's view, and how far below the box's top it stands.
READ_LINE = """() => { const b = document.getElementById('log-lines'); const top = b.getBoundingClientRect().top;
  for (const el of b.children) { const r = el.getBoundingClientRect(); if (r.bottom > top) return { id: el.dataset.id, at: Math.round(r.top - top) }; }
  return null; }"""
SCROLL_DOWN = """(y) => { const b = document.getElementById('log-lines'); b.scrollTop = y; b.dispatchEvent(new Event('scroll')); }"""


def assert_place_kept(page, what):
    """A live line in, the box scrolled down: the line read first stays where it stood."""
    page.evaluate(SCROLL_DOWN, 90)
    was = page.evaluate(READ_LINE)
    assert was and was["id"], f"{what}: a line in view"
    n = page.evaluate("document.querySelectorAll('#log-lines .log-line').length")
    page.evaluate(NEW_LINE)
    frames(page)
    assert page.evaluate("document.querySelectorAll('#log-lines .log-line').length") == n, f"{what}: a line in, one cut at the bottom"
    now = page.evaluate(READ_LINE)
    assert now["id"] == was["id"] and abs(now["at"] - was["at"]) <= 1, f"{what}: the line read first stays where it stood ({was} → {now})"


def test_a_view_scrolled_down_keeps_its_place_as_lines_are_cut(page, case):
    """The box scrolled down, a live line comes in on top and one goes at
    the bottom: with the last 20, where each new line cuts one, and
    at the 2000 lines a view holds, the line the user reads first stays
    where it stood. The plain page removed the cut lines before it
    measured; a reading of the height with them still there moved the view
    by a line at every new line."""
    page.evaluate(SMALL_BOX)
    try:
        open_log(page)
        assert page.evaluate("document.querySelectorAll('#log-lines .log-line').length") == 20, "the last 20"
        assert_place_kept(page, "the last 20")
        show_view(page, "tiles")
        page.evaluate("location.hash = '#log?range=all'")
        assert wait_until(page, "logState.range === 'all' && !logState.loading", 5), "every event"
        stub = StubLog(page, lines(2000), whole=True)
        page.evaluate("loadLog()")
        frames(page)
        assert page.evaluate("logState.lines.length") == 2000, "2000 lines, the view's room"
        assert_place_kept(page, "2000 lines")
        stub.off()
        page.evaluate("location.hash = '#log'")
    finally:
        page.evaluate(NO_SMALL_BOX)
    show_view(page, "tiles")


def test_a_line_shown_keeps_its_element_through_a_filter(page, case):
    """A line's button in focus: a kind chosen that keeps the line, and all
    kinds again, and the search typed and cleared, keep the line, its
    button and the focus the elements they were: a line keeps its
    block, and so its element, while it is among the lines, shown or not.
    A forward guard: the stub ignores the kind, so that it does not reach
    the fault the first blocks had, and the blocks cut over the lines'
    order cannot split a block by their making."""
    open_log(page, 200, "#log?range=all")
    sel = row(CMD_LINE) + " .log-cmd"
    page.locator(sel).focus()
    mark(page, [row(CMD_LINE), sel])
    for what, act in (("the kind chosen", lambda: page.locator("#log-kind").select_option("command")),
                      ("every kind again", lambda: page.locator("#log-kind").select_option(""))):
        act()
        assert wait_until(page, "!logState.loading", 5), f"{what}: its page in"
        frames(page)
        assert replaced(page, [row(CMD_LINE), sel]) == [], f"{what}: the same line and button"
    page.locator(sel).focus()
    for text in ("command", ""):  # the kind's own name, in every language
        page.evaluate("(t) => { const f = document.getElementById('log-filter'); f.value = t; f.dispatchEvent(new Event('input', { bubbles: true })); }", text)
        frames(page)
        assert replaced(page, [row(CMD_LINE), sel]) == [], f"the search {text!r}: the same line and button"
        assert focused(page, sel), f"the search {text!r}: the focus on its button"
    page.evaluate("location.hash = '#log'")
    show_view(page, "tiles")


def test_mark_seen_hands_the_focus_on(page, case):
    """Mark seen of a problem's line, by the keyboard: the button goes with
    its mark, and the focus goes to the next problem's Mark seen below it;
    the last one's to the search. The plain page left it on the
    page's body."""
    open_log(page)
    alerts = [{"id": ALERT_A, "kind": "hello", "level": "warn", "time": "2026-11-20T08:00:17Z"},
              {"id": ALERT_B, "kind": "hello", "level": "warn", "time": "2026-11-20T08:00:16Z"}]
    alerts_route(page, alerts)
    try:
        page.evaluate("loadLogAlerts()")
        assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.log-alert').length === 2", 5), "both lines highlighted"
        page.locator(row(ALERT_A) + " .log-mark").focus()
        page.keyboard.press("Enter")
        assert wait_until(page, "document.activeElement === document.querySelector(%s)" % json.dumps(row(ALERT_B) + " .log-mark"), 5), \
            "the focus on the next problem's Mark seen"
        page.keyboard.press("Enter")
        assert wait_until(page, "document.activeElement === document.getElementById('log-filter')", 5), "the last one's: the search"
    finally:
        page.unroute("**/api/v1/log/alerts*")
        page.evaluate("() => { localStorage.removeItem('massimo.log.seen'); logAlerts.seenOf = null; return loadLogAlerts(); }")
    show_view(page, "tiles")

# --- a sign-out (step 2) ---------------------------------------------------

def sign_off(page, how):
    """The session ended as how says, then the operator in: a sign-out,
    or the session taken as expired and the operator signed in after it."""
    if how == "sign-out":
        sign_out(page)
    else:
        expire(page)
    sign_in(page, "operator", "op-pass")


class Held:
    """The first GET of pattern held until the test lets it go, with the
    answer of answer(url) (the server's where it gives None); later ones
    go on."""

    def __init__(self, page, pattern, answer=None):
        self.routes, self.answer, self.pattern, self.page = [], answer, pattern, page
        safe_route(page, pattern, self.handle)

    def handle(self, route):
        if route.request.method != "GET" or self.routes:
            route.fallback()
            return
        self.routes.append(route)

    def ask(self, what, act):
        """act, which makes the page ask for what is held, waited for until it asks."""
        with self.page.expect_request(self.pattern):
            act()
        frames(self.page)  # the route has the request
        assert self.routes, f"{what} held"

    def let_go(self, ended="true"):
        """The held request answered, and waited for until the page took it:
        the response finished, then ended (an expression: the page's own
        read over, a promise it gave) holds, then a drawing."""
        route = self.routes[0]
        url = route.request.url
        with self.page.expect_response(lambda r: r.url == url) as info:
            body = self.answer(url) if self.answer else None
            if body is None:
                route.fallback()
            else:
                route.fulfill(status=200, content_type="application/json", body=json.dumps(body))
        info.value.finished()
        self.page.evaluate("async () => { await (%s); }" % ended)
        frames(self.page)


FIRSTS = 990000  # the first user's lines, which the next must not see


def late_page(url):
    """A page of the first user's lines, as a late answer brings it."""
    return [ev(FIRSTS - FIRST + k, summary={"firmware": "the-first-users", "duty_cycle_pct": 1, "free_memory_slots": 1}) for k in range(3)]


NOTHING_LEFT = """() => logState.lines.every((l) => l.id < 990000 || l.id > 990010) && logState.commands.size === 0
  && logState.openCommands.size === 0 && !logState.filter && !document.querySelector('#log-lines [data-id^="99000"]')
  && document.getElementById('log-filter').value === ''"""


def owner_log(new_context, guard, watch, hash_="#log"):
    """The owner's page of its own (owner_page), the log shown by hash_
    with the search typed into."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    show_view(page, "log")
    if hash_ != "#log":
        page.evaluate("(h) => { location.hash = h; }", hash_)
        assert wait_until(page, "location.hash === %s && !logState.loading" % json.dumps(hash_), 5), "the log's address"
    assert wait_until(page, "!logState.loading", 5), "the log loaded"
    page.locator("#log-filter").fill("typed by the first")
    return page


def assert_forgotten(page, what):
    """Nothing of the first user's log is left: no line of a late answer,
    no command's detail, no text typed in the search."""
    assert page.evaluate(NOTHING_LEFT), f"{what}: nothing of the first user's log left " + page.evaluate(
        "JSON.stringify({ids: logState.lines.map((l) => l.id).filter((i) => i >= 990000), commands: logState.commands.size, filter: logState.filter})")


HOWS = ("sign-out", "expired")


@pytest.mark.parametrize("how", HOWS)
def test_a_late_page_of_the_log_reaches_nothing_of_the_next(new_context, case, watch, guard, how):
    """The log's page asked for and held while the session ends: let go once
    the operator is in, it draws nothing, and the search typed is gone.
    The plain page took it, the log as the first user left it."""
    page = owner_log(new_context, guard, watch)
    held = Held(page, "**/api/v1/cubes/*/log?*", late_page)
    held.ask("the log's page", lambda: page.evaluate("() => { window.__read = loadLog(); }"))
    sign_off(page, how)
    held.let_go("window.__read")
    assert_forgotten(page, "a late page")


@pytest.mark.parametrize("how", HOWS)
def test_a_late_poll_reaches_nothing_of_the_next(new_context, case, watch, guard, how):
    """Every Cube's log, its poll held while the session ends: let go, it
    adds nothing, and the search typed is gone. For the poll a forward
    guard: the plain page's poll ended too, as the log was no longer
    shown; on the plain page this test fails by the search it kept."""
    page = owner_log(new_context, guard, watch, "#log?cubes=all")
    page.evaluate("clearInterval(logState.pollTimer)")
    held = Held(page, "**/api/v1/log?*", late_page)
    held.ask("the poll", lambda: page.evaluate("() => { window.__read = pollLog(); }"))
    sign_off(page, how)
    held.let_go("window.__read")
    assert_forgotten(page, "a late poll")


@pytest.mark.parametrize("how", HOWS)
def test_a_late_question_of_older_reaches_nothing_of_the_next(new_context, case, watch, guard, how):
    """Older's question (after=, logGone) held while the session ends: let
    go, Older asks for no page and nothing is drawn. The plain page went on
    to ask for the page before the oldest, and drew it."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    show_view(page, "log")
    StubLog(page, lines(200))
    page.evaluate("location.hash = '#log?range=all'")
    assert wait_until(page, "logState.range === 'all' && !logState.loading && logState.more", 5), "a full page, Older offered"
    asked = []
    page.on("request", lambda r: asked.append(r.url) if "before=" in r.url else None)
    held = Held(page, "**/api/v1/cubes/*/log?limit=1&after=*")
    held.ask("Older's question", lambda: page.locator("#log-older").click())
    sign_off(page, how)
    held.let_go()
    frames(page)
    assert not asked, f"no page before the oldest asked for: {asked}"
    assert page.evaluate("logState.lines.length") == 0, "no line of the first user"


@pytest.mark.parametrize("how", HOWS)
def test_a_late_command_reaches_nothing_of_the_next(new_context, case, watch, guard, how):
    """A command's detail asked for and held while the session ends: let
    go, it is kept nowhere, and no detail is open for the next user. The
    plain page kept it and the command open."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    show_view(page, "log")
    StubLog(page, lines())
    page.evaluate("loadLog()")
    held = Held(page, f"**/api/v1/commands/{CMD}", lambda url: DETAIL)
    held.ask("the command's detail", lambda: page.locator(row(CMD_LINE) + " .log-cmd").click())
    sign_off(page, how)
    held.let_go()
    assert page.evaluate("logState.commands.size === 0 && logState.openCommands.size === 0"), "no command's detail kept, none open"


@pytest.mark.parametrize("how", HOWS)
def test_a_late_list_of_users_reaches_nothing_of_the_next(new_context, case, watch, guard, how):
    """The who choice's users asked for as the log opens, held while the
    session ends: let go, the list is kept nowhere, and the next user's log
    asks for its own. The plain page kept the first user's list for good."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    users = [{"name": "the-first-users-user", "display_name": "First"}]
    held = Held(page, "**/api/v1/users", lambda url: users)
    held.ask("the users", lambda: show_view(page, "log"))
    sign_off(page, how)
    held.let_go()
    assert page.evaluate("logState.users === null"), "the users' list forgotten, to be asked for again"
    assert page.locator("#log-user option[value='the-first-users-user']").count() == 0, "not in the who choice"


@pytest.mark.parametrize("how", HOWS)
def test_a_late_cubes_rooms_reach_nothing_of_the_next(new_context, case, watch, guard, how):
    """Under All Cubes a Cube's rooms asked for, held while the session
    ends: let go, they are kept nowhere. The plain page kept them in every
    Cube's rooms for the next user."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    show_view(page, "log")
    page.evaluate("logState.cubeRooms.clear()")
    held = Held(page, "**/api/v1/cubes/*/rooms?devices=all")
    held.ask("a Cube's rooms", lambda: page.evaluate("location.hash = '#log?cubes=all'"))
    sign_off(page, how)
    held.let_go()
    assert page.evaluate("[...logState.cubeRooms.values()].every((v) => !v)"), "no Cube's rooms kept"


@pytest.mark.parametrize("how", HOWS)
def test_late_marks_reach_nothing_of_the_next(new_context, case, watch, guard, how):
    """The first user's marks (its preferences) asked for and held while
    the session ends: let go, they are not the next user's. The plain page
    took them as the marks of whoever was in."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    held = Held(page, "**/api/v1/me/preferences", lambda url: {"log.seen": {"all": 990999, "lines": []}})
    held.ask("the marks", lambda: page.evaluate("() => { logAlerts.seenOf = null; window.__read = loadLogAlerts(); }"))
    sign_off(page, how)
    held.let_go("window.__read")
    assert page.evaluate("logAlerts.seen.all") != 990999, "not the first user's marks"


@pytest.mark.parametrize("how", HOWS)
def test_late_problems_reach_nothing_of_the_next(new_context, case, watch, guard, how):
    """The log's unseen problems asked for and held while the session ends:
    let go once the operator's own are in, they are not drawn, nor counted
    on ☰. The plain page drew them."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    late = {"level": "error", "alerts": [{"id": 990003, "kind": "restart", "level": "error", "time": "2026-11-20T08:00:03Z"}]}
    held = Held(page, "**/api/v1/log/alerts*", lambda url: late)
    held.ask("the problems", lambda: page.evaluate("() => { window.__read = loadLogAlerts(); }"))
    sign_off(page, how)
    held.let_go("window.__read")
    assert page.evaluate("!logAlerts.list.some((a) => a.id === 990003)"), "not the first user's problems"


def test_the_same_user_after_a_401_keeps_the_log(new_context, case, watch, guard):
    """The operator's log with the search typed, a page held while the
    session expires and the operator signs in again: nothing is forgotten,
    the held page is drawn, and the search holds what was typed, as on the
    plain page. A forward guard."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    sign_in(page, "operator", "op-pass")
    show_view(page, "log")
    assert wait_until(page, "!logState.loading", 5), "the log loaded"
    page.locator("#log-filter").fill("the-first")
    held = Held(page, "**/api/v1/cubes/*/log?*", late_page)
    held.ask("the log's page", lambda: page.evaluate("() => { window.__read = loadLog(); }"))
    expire(page)
    # signed in again in the dialog, the log still shown: sign_in waits for the tiles
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("op-pass")
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, "!document.getElementById('login-dialog').open && currentView === 'log'", 10), "signed in again, the log shown"
    held.let_go("window.__read")
    assert page.locator("#log-filter").input_value() == "the-first", "the search as typed"
    assert page.evaluate("logState.lines.filter((l) => l.id >= 990000 && l.id < 990010).length") == 3, "the held page drawn"



def test_the_who_choice_follows_another_user_without_a_sign_out(new_context, case, watch, guard):
    """The operator's log, whose who choice it may not have; the owner's key
    pasted into the sign-in without a sign-out before it, as the page
    takes it (askLogin, no start waiting): the log shown again asks for
    the users for the owner, and the who choice is there. The
    plain page kept the operator's empty list until a reload."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    sign_in(page, "operator", "op-pass")
    assert not page.evaluate("hasCap('users')"), "the operator may not list the users"
    show_view(page, "log")
    assert wait_until(page, "Array.isArray(logState.users) && logState.users.length === 0", 5), "no users for the operator"
    assert page.locator("#log-user").is_hidden(), "no who choice"
    show_view(page, "tiles")
    page.evaluate("() => { askLogin(); }")
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in"
    page.locator("#login-ways .login-way[data-way='key']").click()
    page.locator("#login-token").fill(TOKEN)
    with page.expect_response(lambda r: r.url.endswith("/api/v1/me")):
        page.locator("#login-key-form button[type=submit]").click()
    assert wait_until(page, "!document.getElementById('login-dialog').open && hasCap('users')", 10), "the owner in"
    with page.expect_response(lambda r: r.url.endswith("/api/v1/users")) as info:
        show_view(page, "log")
    info.value.finished()
    assert wait_until(page, "!document.getElementById('log-user').classList.contains('hidden')", 5), "the who choice, for the owner"
    show_view(page, "tiles")

# --- the stream's resync (step 3) ----------------------------------------

RESYNC = "state.events.dispatchEvent(new MessageEvent('resync', { data: '{}' }))"
MISSED = {"firmware": "missed-by-the-stream", "duty_cycle_pct": 1, "free_memory_slots": 1}


def reads_on(stub, since):
    """The log's reads after a mark: the pages on from a line (after=, not
    Older's question of one line) and the pages loaded (neither)."""
    asked = stub.asked[since:]
    on = [u for u in asked if "after=" in u and "limit=1&" not in u]
    loads = [u for u in asked if "after=" not in u and "before=" not in u]
    return on, loads


def test_a_cubes_log_reads_on_at_a_resync(page, case):
    """A Cube's own log shown, older pages under it, the box scrolled down
    and a command's detail open, and a line in the server's log that the
    stream did not bring: at the stream's resync the log reads on from its
    newest line, once, and loads nothing; the line the stream missed is on
    top, every line before it still there, the line read first where it
    stood and the detail open (UiParts D208). The plain page read nothing
    there, the line missing until a reload (AP LogCursor's NOTE on how its
    run went, "Found, not changed" 1)."""
    page.evaluate(SMALL_BOX)
    try:
        stub = open_log(page, 400, "#log?range=all")
        page.locator("#log-older").click()
        assert wait_until(page, "!logState.loading && logState.lines.length === 400", 5), "an older page under the lines"
        page.locator(row(CMD_LINE) + " .log-cmd").click()
        page.locator(row(CMD_LINE) + " .log-cmd-detail .log-cmd-head").wait_for()
        page.evaluate(SCROLL_DOWN, 90)
        was = page.evaluate(READ_LINE)
        stub.lines = stub.lines + [ev(30, summary=MISSED)]
        since = len(stub.asked)
        with page.expect_response(lambda r: "after=" in r.url) as info:
            page.evaluate(RESYNC)
        info.value.finished()
        assert wait_until(page, "!!document.querySelector(%s)" % json.dumps(row(FIRST + 30)), 5), "the line the stream missed"
        frames(page)
        on, loads = reads_on(stub, since)
        assert len(on) == 1 and not loads, f"read on once, nothing loaded: {on} {loads}"
        assert page.evaluate("logState.lines.length") == 401, "every line before it still there"
        now = page.evaluate(READ_LINE)
        assert now["id"] == was["id"] and abs(now["at"] - was["at"]) <= 1, f"the line read first where it stood ({was} → {now})"
        assert page.locator(row(CMD_LINE) + " .log-cmd").get_attribute("aria-expanded") == "true", "the detail open"
    finally:
        page.evaluate(NO_SMALL_BOX)
    page.evaluate("location.hash = '#log'")
    show_view(page, "tiles")


def test_a_resync_waits_for_older(page, case):
    """Older's page held while the stream's resync comes: the resync reads
    nothing meanwhile; Older's page is there when it answers, and then the
    log reads on once (UiParts D208). A load at the resync would have
    dropped Older's page."""
    stub = open_log(page, 200, "#log?range=all")
    held = Held(page, "**/api/v1/cubes/*/log?*before=*")
    held.ask("Older's page", lambda: page.locator("#log-older").click())
    stub.lines = [ev(-400 + i) for i in range(5)] + stub.lines + [ev(30, summary=MISSED)]
    since = len(stub.asked)
    page.evaluate(RESYNC)
    frames(page)
    assert reads_on(stub, since) == ([], []), "nothing read while Older's page is on its way"
    with page.expect_response(lambda r: "after=" in r.url and "limit=1&" not in r.url) as info:
        held.let_go("new Promise((r) => { const t = () => (logState.loading ? setTimeout(t, 20) : r()); t(); })")
    info.value.finished()
    assert wait_until(page, "!!document.querySelector(%s)" % json.dumps(row(FIRST + 30)), 5), "the line the stream missed"
    on, loads = reads_on(stub, since)
    assert len(on) == 1 and not loads, f"then read on once: {on} {loads}"
    assert page.evaluate("logState.lines.length") == 206, "Older's page kept"
    page.evaluate("location.hash = '#log'")
    show_view(page, "tiles")


def test_a_resync_loads_the_log_where_the_servers_log_ends_before_it(page, case, watch):
    """A resync whose read on meets a 409, a log that ends before the
    newest line (a store restored, the server's log started again): the
    log loads again, once, as under All Cubes."""
    stub = open_log(page)
    stub.gone = True
    watch.expected.append((f"after={CMD_LINE}", 409))
    since = len(stub.asked)
    with page.expect_response(lambda r: "after=" not in r.url and "/log?" in r.url and "alerts" not in r.url) as info:
        page.evaluate(RESYNC)
    info.value.finished()
    assert wait_until(page, "!logState.loading", 5), "the load ended"
    frames(page)
    on, loads = reads_on(stub, since)
    assert len(on) == 1 and len(loads) == 1, f"read on, then one load: {on} {loads}"
    show_view(page, "tiles")


ORDERED = """() => logState.lines.every((l, i, a) => !i || a[i - 1].time <= l.time) && !logState.lines.some((l) => l.kind === 'error')"""


@pytest.mark.parametrize("before", ["failed", "empty"])
def test_a_resync_without_a_stored_line_loads_the_log(page, case, watch, before):
    """A Cube's log that holds no line of the server's log, live lines in it
    since: after a load that failed (the server not up yet: the error's
    line), or one that found no line. At the stream's resync there is no
    place to read on from: the log loads, once, the error's line goes, and
    the lines are in their order. A read on from nothing put the
    newest page above the live lines, out of order, the error's line
    under it; with the last 20 its cut hid that, so every event is shown."""
    stub = open_log(page, 20, "#log?range=all")
    page_lines = stub.lines
    if before == "failed":
        stub.fail = True
        watch.expected.append(("/log?limit=200", 503))  # the load that fails
    else:
        stub.lines = []
    page.evaluate("loadLog()")
    frames(page)
    if before == "failed":
        assert page.locator("#log-lines .log-line.kind-error").count() == 1, "the error's line"
    page.evaluate(NEW_LINE)
    page.evaluate(NEW_LINE)
    frames(page)
    stub.fail, stub.lines = False, page_lines
    since = len(stub.asked)
    with page.expect_response(lambda r: "/log?" in r.url and "alerts" not in r.url) as info:
        page.evaluate(RESYNC)
    info.value.finished()
    assert wait_until(page, "!logState.loading", 5), "the load ended"
    frames(page)
    on, loads = reads_on(stub, since)
    assert not on and len(loads) == 1, f"one load, no read on: {on} {loads}"
    assert page.evaluate(ORDERED), "no error's line, the lines in their order: " + page.evaluate(
        "JSON.stringify(logState.lines.map((l) => [l.kind, l.at]).slice(-4))")
    page.evaluate("location.hash = '#log'")
    show_view(page, "tiles")

