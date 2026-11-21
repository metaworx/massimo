# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The log's button by the log's unseen problems (AP LogAlerts): plain
without one, amber with a triangle for a warning, red with a cross for
an error, also ☰'s log entry; a problem's line is highlighted with its
mark and Mark seen, Mark all as seen marks every one, and opening the
log marks nothing. GET /api/v1/log/alerts is stubbed, with lines of the
real log where the log shows them."""
import json

from conftest import api_call
from helpers import show_view, wait_until


def alerts_route(page, alerts):
    """Stubs GET /api/v1/log/alerts with the list alerts as it is at each
    request, the worst level from it; returns the URLs asked."""
    asked = []

    def handle(route):
        asked.append(route.request.url)
        level = "error" if any(a["level"] == "error" for a in alerts) else ("warn" if alerts else "")
        route.fulfill(status=200, content_type="application/json", body=json.dumps({"level": level, "alerts": alerts}))

    page.route("**/api/v1/log/alerts*", handle)
    return asked


# ☰'s Log entry's sign in a tone, as wait_until takes it
SIGN_TONE = "document.querySelector('#menu-log svg').getAttribute('class').includes('tone-%s')"


def sign(page, selector):
    """The classes and the symbols of a button's icon."""
    return page.evaluate("""(sel) => {
        const svg = document.querySelector(sel + ' svg');
        return {cls: svg.getAttribute('class'), uses: [...svg.querySelectorAll('use')].map((u) => u.getAttribute('href'))};
    }""", selector)


def test_log_sign_by_level(page):
    """No problem: the plain log in ☰'s entry, no mark on ☰. A warning:
    amber with the triangle, the count in the entry's name and ☰'s, ☰ with
    the amber mark. An error: red with the cross."""
    alerts = []
    alerts_route(page, alerts)
    try:
        page.evaluate("loadLogAlerts()")
        s = sign(page, "#menu-log")
        assert "tone-" not in s["cls"] and s["uses"] == ["#icon-log"], f"no problem: the plain log ({s})"
        assert page.locator("#nav-menu > summary .mark-sign").count() == 0, "no mark on ☰"
        alerts.append({"id": 900001, "kind": "disconnected", "level": "warn", "cube_serial": "KEQ0000100", "time": "2026-11-14T20:00:00Z"})
        page.evaluate("loadLogAlerts()")
        # ☰ is a part (AP UiParts, Block 14), drawn after the alerts are read: waited for by what changed
        assert wait_until(page, SIGN_TONE % "warn", 3), "the entry turns amber"
        s = sign(page, "#menu-log")
        assert "tone-warn" in s["cls"] and s["uses"] == ["#icon-log-small", "#mark-corner-warn"], f"a warning: amber, the triangle ({s})"
        count = page.evaluate("t('nav.logAlerts', {count: 1})")
        assert page.locator("#menu-log").get_attribute("aria-label") == count, "the count named"
        assert count in (page.locator("#nav-menu > summary").get_attribute("aria-label") or ""), "in ☰'s name too"
        assert "tone-warn" in (page.locator("#nav-menu > summary .mark-sign").get_attribute("class") or ""), "☰ with the amber mark"
        mark = page.locator("#nav-menu > summary .mark-sign")
        disc = mark.evaluate("(e) => [getComputedStyle(e).backgroundColor, getComputedStyle(e).borderRadius]")
        card = page.evaluate("""() => { const p = document.createElement('div'); p.style.background = 'var(--card)';
          document.body.appendChild(p); const c = getComputedStyle(p).backgroundColor; p.remove(); return c; }""")
        assert disc == [card, "50%"], f"the mark on a disc of the card's colour, readable on the filled ☰: {disc}, card {card}"
        alerts.append({"id": 900002, "kind": "restart", "level": "error", "time": "2026-11-14T20:01:00Z"})
        page.evaluate("loadLogAlerts()")
        assert wait_until(page, SIGN_TONE % "bad", 3), "the entry turns red"
        s = sign(page, "#menu-log")
        assert "tone-bad" in s["cls"] and s["uses"] == ["#icon-log-small", "#mark-corner-x"], f"an error: red, the cross ({s})"
    finally:
        page.unroute("**/api/v1/log/alerts*")


def test_log_marks(page):
    """Two lines of the log are problems, a warning and an error: both are
    highlighted, the error red. Opening the log and asking again marks
    nothing. Mark seen on the error leaves the warning, the entry amber,
    and the mark is kept; Mark all as seen leaves none, the entry plain,
    and the next question asks after the newest."""
    puts = []
    page.on("request", lambda r: puts.append(r.url) if r.method == "PUT" and "log.seen" in r.url else None)
    show_view(page, "log")
    assert wait_until(page, "logState.lines.filter((l) => l.id && logLineMatches(l)).length >= 2", 10), "two lines with ids"
    ids = page.evaluate("logState.lines.filter((l) => l.id && logLineMatches(l)).slice(-2).map((l) => l.id)")
    older, newer = min(ids), max(ids)
    alerts = [{"id": newer, "kind": "state", "level": "warn", "time": "2026-11-14T20:01:00Z"},
              {"id": older, "kind": "restart", "level": "error", "time": "2026-11-14T20:00:00Z"}]
    asked = alerts_route(page, alerts)
    try:
        page.evaluate("loadLogAlerts()")
        assert page.locator("#log-lines .log-line.log-alert").count() == 2, "both lines highlighted"
        error_row = page.locator("#log-lines .log-line[data-id='%d']" % older)
        assert "log-alert-bad" in error_row.get_attribute("class") and error_row.locator(".mark-sign").count() == 1, "the error red, its mark"
        assert page.locator("#log-seen-all").is_visible(), "Mark all as seen"

        show_view(page, "tiles")
        show_view(page, "log")
        page.evaluate("loadLogAlerts()")
        assert page.locator("#log-lines .log-line.log-alert").count() == 2 and not puts, "opening the log marks nothing"

        synced = page.evaluate("canSync()")
        if synced:
            with page.expect_response(lambda r: "log.seen" in r.url and r.request.method == "PUT"):
                error_row.locator(".log-mark").click()
            assert (api_call("GET", "api/v1/me/preferences/log.seen") or {}).get("lines") == [older], "the user's preference keeps the mark"
        else:
            error_row.locator(".log-mark").click()
            assert json.loads(page.evaluate("localStorage.getItem('massimo.log.seen')"))["lines"] == [older], "the browser keeps the mark"
        assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.log-alert').length === 1", 5), "the error's line marked"
        assert wait_until(page, SIGN_TONE % "warn", 3), "the entry amber"
        page.evaluate("loadLogAlerts()")
        assert page.locator("#log-lines .log-line.log-alert").count() == 1, "marked, it stays marked"

        page.locator("#log-seen-all").click()
        assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.log-alert').length === 0", 5), "every line marked"
        assert wait_until(page, "!document.querySelector('#menu-log svg').getAttribute('class').includes('tone-')", 3), "the entry plain again"
        s = sign(page, "#menu-log")
        assert "tone-" not in s["cls"] and page.locator("#log-seen-all").is_hidden(), "the entry plain, Mark all gone"
        assert page.evaluate("logAlerts.seen") == {"all": newer, "lines": []}, "all up to the newest, the line it covers gone"
        page.evaluate("loadLogAlerts()")
        assert asked[-1].endswith("after=%d" % newer), f"asked after the newest ({asked[-1]})"
    finally:
        page.unroute("**/api/v1/log/alerts*")
    show_view(page, "tiles")
