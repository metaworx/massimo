# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The fixtures of the web UI suite, run by tools/ui-test/run.sh (make
ui-test) against the server and the fake Cube it starts; it hands over
the page's address in UI_TEST_URL and the API token in UI_TEST_TOKEN.

Every test runs four times, at 1100 and 390 px wide and in English and
German, the language from the browser context's locale (`case`, with the
ids 1100-en, 390-en, 1100-de and 390-de). Each run gets a browser context
of its own from pytest-playwright, with the token in localStorage before
the page loads, and `page` is the page loaded with its room tiles and
its event stream open. The four cases share the one server, so a test
that changes the server's store puts it back; the settings the page
keeps on the server for the token's user are removed before every page.

A test marked used_ticket gets a used ticket for its first stream
(`watch.refused`), which the server refuses with a 401; its page is
handed over before the stream opens again.

test_login.py opens a context of its own, without the token, from
pytest-playwright's new_context with the case's viewport and locale.

The guard (autouse) fails a test on any console error, page error or a
horizontal scroll left at its end; the refused stream of a used_ticket
test and a stubbed 503 of the discovery are expected.

pytest-playwright keeps the trace, the screenshot and the video of a
failed test (pytest.ini) under the directory run.sh names with
--output, tools/ui-test/results/.
"""
import json
import os
import ssl
import urllib.request

import pytest

pytest.register_assert_rewrite("helpers")

from helpers import USED, scroll_width, used_ticket_first, wait_until  # noqa: E402

URL = os.environ.get("UI_TEST_URL", "http://127.0.0.1:8119/")
TOKEN = os.environ.get("UI_TEST_TOKEN", "")
LOCALE = {"en": "en-US", "de": "de-DE"}
HEIGHT = 900


class Case:
    """One width and one language."""

    def __init__(self, width, lang):
        self.width = width
        self.lang = lang
        self.locale = LOCALE[lang]


class Watch:
    """What the page did that a test or the guard looks at afterwards:
    the stream requests, the used ticket handed out, and the problems
    (console errors, page errors)."""

    def __init__(self):
        self.streams = []
        self.refused = []
        self.problems = []


@pytest.fixture(params=[(1100, "en"), (390, "en"), (1100, "de"), (390, "de")],
                ids=lambda p: f"{p[0]}-{p[1]}")
def case(request):
    return Case(*request.param)


@pytest.fixture
def browser_context_args(browser_context_args, case):
    return {**browser_context_args, "viewport": {"width": case.width, "height": HEIGHT}, "locale": case.locale}


@pytest.fixture
def watch():
    return Watch()


def api_call(method, path, body=None, token=None):
    """A request to the server's API from the test itself, with the test
    token unless another is given; the answer's JSON, None for none."""
    tls = ssl.create_default_context()
    tls.check_hostname, tls.verify_mode = False, ssl.CERT_NONE
    headers = {"Authorization": "Bearer " + (token or TOKEN)}
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(URL.rstrip("/") + "/" + path.lstrip("/"), method=method, headers=headers, data=data)
    with urllib.request.urlopen(req, timeout=5, context=tls) as resp:
        text = resp.read()
    return json.loads(text) if text else None


def clear_preferences():
    """The token's user keeps the page's settings on the server, and the
    four cases share the server: its preferences are removed before
    every page, so that a test starts from the defaults as a fresh
    browser does."""
    if not TOKEN:
        return
    for key in ("ui", "schemes"):
        api_call("DELETE", "api/v1/me/preferences/" + key)


@pytest.fixture
def page(context, case, watch, request):
    """The page loaded, with the token, the room tiles shown and, unless
    the test is marked used_ticket, the event stream open; the token's
    user without preferences on the server (clear_preferences)."""
    clear_preferences()
    if TOKEN:
        context.add_init_script(f"localStorage.setItem('massimo.token', {TOKEN!r})")
    page = context.new_page()
    page.set_default_timeout(5000)
    used = request.node.get_closest_marker("used_ticket") is not None
    if used:
        page.route("**/events/ticket", used_ticket_first(watch.refused))

    def on_console(m):
        src = m.location.get("url", "")
        # the refused stream and the refused search are on purpose
        expected = m.text.startswith("Failed to load resource") and (
            (watch.refused and USED in src) or (src.endswith("/cubes/discover") and "503" in m.text))
        if m.type == "error" and not expected:
            watch.problems.append(f"console: {m.text}")

    page.on("request", lambda r: watch.streams.append(r.url) if "/events?" in r.url else None)
    page.on("console", on_console)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    page.wait_for_selector(".room", timeout=10000)
    if not used and not wait_until(page, "state.eventsOpen", 10):
        raise RuntimeError("the event stream did not open")
    return page


class Guard:
    """The checks every test ends with."""

    def __init__(self, page, case, watch):
        self.page = page
        self.case = case
        self.watch = watch

    def verify(self):
        problems = list(self.watch.problems)
        if self.page is not None and not self.page.is_closed() and scroll_width(self.page) > self.case.width:
            problems.append(f"a horizontal scroll at the end: {scroll_width(self.page)} px wide")
        assert not problems, "; ".join(problems)


@pytest.fixture(autouse=True)
def guard(request, case, watch):
    """The page fixture's page; a test that opens its own page without
    it (test_login.py) hands it to the guard itself."""
    page = request.getfixturevalue("page") if "page" in request.fixturenames else None
    return Guard(page, case, watch)


@pytest.hookimpl(wrapper=True)
def pytest_runtest_call(item):
    """The guard's checks run as part of the test, so that a console
    error fails the test itself and pytest-playwright keeps its trace."""
    result = yield
    guard = item.funcargs.get("guard")
    if guard is not None:
        guard.verify()
    return result
