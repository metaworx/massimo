# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The fixtures of the web UI suite, run by tools/ui-test/run.sh (make
ui-test) against the server and the fake Cube it starts; it hands over
the page's address in UI_TEST_URL and the API token in UI_TEST_TOKEN.
In pytest-xdist's workers, which run.sh starts with a server set each,
worker gw<n> takes set n's addresses and log from UI_TEST_URL_<n>,
UI_TEST_DOWN_URL_<n> and UI_TEST_SERVE_LOG_<n> in place of the plain
names, before this module or a test file reads them.

Every test runs four times, at 1100 and 390 px wide and in English and
German, the language from the browser context's locale (`case`, with the
ids 1100-en, 390-en, 1100-de and 390-de); UI_TEST_CASES, comma-separated
ids, runs only those cases, as CI's jobs do, a case each. A test marked
cases (ids) runs in those cases alone: in the others it is deselected as
the suite is collected, before a fixture loads a page for it
(test_parts_guard.py). Each run gets a
browser context of its own from pytest-playwright, with the token in
localStorage before the page loads, and `page` is the page loaded with
its room tiles and its event stream open. The four cases share the one server of a worker,
so a test that changes the server's store puts it back; the settings the page
keeps on the server for the token's user are removed before every page.
The page fixture's default timeout and its waits, as every wait of
helpers.py, last their seconds times UI_TEST_SCALE (helpers.scaled),
which run.sh passes on and raises for its retry.

The page has helpers.PAGE_KIT before its own scripts, which records
what its event stream brings. The fixture reads answers the page's own
reads with the server's data and what the test put in (helpers.Reads),
and stream watches the page's event stream from a mark (helpers.Stream);
docs/ui-testing.md, section 9, says when a test needs them.

A test marked used_ticket gets a used ticket for its first stream
(`watch.refused`), which the server refuses with a 401; its page is
handed over before the stream opens again.

A test marked clock gets Playwright's clock installed before the page's
scripts run, its time flowing as the real one: the test moves it on with
page.clock.fast_forward, for what the page does after a minute. A test
pauses it with helpers.pause_clock alone: a test module that calls
clock.pause_at itself fails its collection (pytest_collect_file).

test_login.py opens a context of its own, without the token, from
pytest-playwright's new_context with the case's viewport and locale.

A page without the token counts a failed token check against 127.0.0.1
with every API request it sends, and the files of a worker share its
server: ten within a minute, a sign-in's wrong password among them, lock
the address out for five minutes (429). After a test that used
new_context, one request with the token forgets them (the autouse
fixture failed_checks_forgotten); after every test, the set's store,
massimo.db beside UI_TEST_SERVE_LOG, has none counted against an address,
or the test errors at teardown.

The guard (autouse) fails a test on any console error, page error or a
horizontal scroll left at its end; the refused stream of a used_ticket
test and a stubbed 503 of the discovery are expected.

pytest-playwright keeps the screenshot of a failed test (pytest.ini)
under the directory run.sh names with --output, tools/ui-test/results/,
and the screenshot, the video and the trace of a failed test that run.sh
runs once more under results/retry/.
"""
import io
import json
import os
import sqlite3
import ssl
import tokenize
import urllib.request

import pytest

_WORKER = os.environ.get("PYTEST_XDIST_WORKER", "")
if _WORKER.startswith("gw"):
    for _name in ("UI_TEST_URL", "UI_TEST_DOWN_URL", "UI_TEST_SERVE_LOG"):
        _set = f"{_name}_{_WORKER[2:]}"
        if _set not in os.environ:
            raise RuntimeError(f"worker {_WORKER} has no server set ({_set}): give run.sh UI_TEST_WORKERS rather than pytest -n")
        os.environ[_name] = os.environ[_set]

pytest.register_assert_rewrite("helpers")

from helpers import (PAGE_KIT, USED, ApiReads, Stream, reads_of, report_to, scaled, scroll_width, used_ticket_first,  # noqa: E402
                     wait_started, wait_until)

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
    (console errors, page errors). A test that has the server refuse a
    request on purpose lists it in expected, as (the URL's end, status)."""

    def __init__(self):
        self.streams = []
        self.refused = []
        self.problems = []
        self.expected = []


CASES = {"1100-en": (1100, "en"), "390-en": (390, "en"), "1100-de": (1100, "de"), "390-de": (390, "de")}


def case_id(p):
    """A case's id, such as 1100-en, from its (width, language)."""
    return f"{p[0]}-{p[1]}"


def chosen_cases():
    """The cases UI_TEST_CASES names, comma-separated ids such as
    1100-en,1100-de, in CASES's order; all four where it names none.
    An unknown id stops the run, so that a typo does not test nothing."""
    names = [n.strip() for n in os.environ.get("UI_TEST_CASES", "").split(",") if n.strip()]
    unknown = [n for n in names if n not in CASES]
    if unknown:
        raise pytest.UsageError(f"UI_TEST_CASES names {', '.join(unknown)}; the cases are {', '.join(CASES)}")
    return [p for n, p in CASES.items() if not names or n in names]


@pytest.fixture(params=chosen_cases(), ids=case_id)
def case(request):
    return Case(*request.param)


# The message of a mark cases that names no case, kept with its test.
BAD_CASES = pytest.StashKey[str]()


def pytest_collection_modifyitems(config, items):
    """A test marked cases runs only in the cases the mark names, ids
    such as 1100-en: in the others it is deselected here, as the suite is
    collected, rather than skipped by a fixture after the autouse guard
    has loaded its page (AP TestSpeed, D9-A): one process's summary counts
    it as deselected, xdist's leaves it out of the items. An id that is not
    a case fails the marked test in every case (pytest_runtest_setup), so
    that a typo does not deselect a test everywhere: an error raised here
    would reach no one under xdist, whose workers' collection then
    differs, and the run ends in an internal error without its cause."""
    kept, dropped = [], []
    for item in items:
        mark = item.get_closest_marker("cases")
        params = getattr(getattr(item, "callspec", None), "params", {})
        if mark is None or "case" not in params:
            kept.append(item)
            continue
        unknown = [n for n in mark.args if n not in CASES]
        if unknown or not mark.args:
            item.stash[BAD_CASES] = (f"the mark cases names {', '.join(unknown) or 'no case'}; "
                                     f"the cases are {', '.join(CASES)}")
            kept.append(item)
            continue
        (kept if case_id(params["case"]) in mark.args else dropped).append(item)
    if dropped:
        config.hook.pytest_deselected(items=dropped)
        items[:] = kept


def calls_pause_at(source):
    """The lines of a module's source that call a clock's pause_at
    themselves (clock.pause_at( in the code, not in a comment or a
    string)."""
    names = [t for t in tokenize.generate_tokens(io.StringIO(source).readline)
             if t.type not in (tokenize.NL, tokenize.NEWLINE, tokenize.INDENT, tokenize.DEDENT, tokenize.COMMENT)]
    return [names[i].start[0] for i in range(len(names) - 3)
            if [t.string for t in names[i:i + 4]] == ["clock", ".", "pause_at", "("]]


class DirectPause(pytest.File):
    """A test module that pauses Playwright's clock itself, which fails
    its collection."""

    def __init__(self, *, lines, **kwargs):
        super().__init__(**kwargs)
        self.lines = lines

    def collect(self):
        raise self.CollectError(
            f"{self.path.name} calls clock.pause_at at line {', '.join(map(str, self.lines))}: "
            "pause the clock with helpers.pause_clock(page), which takes the page's now in seconds")


def pytest_collect_file(file_path, parent):
    """A test module that calls clock.pause_at itself is an error at its
    collection, which names helpers.pause_clock: Python's pause_at takes
    seconds, and the page's Date.now() given it as they are jumped the
    clock millennia ahead. helpers.py alone may call it, and is no test
    module."""
    if file_path.suffix != ".py" or not file_path.name.startswith("test_"):
        return None
    source = file_path.read_text(encoding="utf-8")
    if "pause_at" not in source:
        return None
    lines = calls_pause_at(source)
    return DirectPause.from_parent(parent, path=file_path, lines=lines) if lines else None


@pytest.hookimpl(tryfirst=True)
def pytest_runtest_setup(item):
    """A test whose mark cases names no case fails, before its fixtures
    load a page, with what the mark names."""
    if BAD_CASES in item.stash:
        pytest.fail(item.stash[BAD_CASES], pytrace=False)


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


def address_failures():
    """The failed checks the set's store counts against an address, as
    (key, count, locked until); None without the store's directory
    (UI_TEST_SERVE_LOG), for a server started by hand."""
    log = os.environ.get("UI_TEST_SERVE_LOG", "")
    if not log:
        return None
    db = sqlite3.connect(f"file:{os.path.join(os.path.dirname(log), 'massimo.db')}?mode=ro", uri=True)
    try:
        return db.execute("SELECT key, count, locked_until FROM auth_failures WHERE key LIKE 'addr:%' ORDER BY key").fetchall()
    finally:
        db.close()


@pytest.fixture(autouse=True)
def failed_checks_forgotten(request):
    """After a test that opened a context without the token (new_context),
    whose page's requests each counted a failed token check against the
    address, a request with the token forgets them; then, after every
    test, none is left, so that no file brings the next one on its
    worker closer to the lockout. Set up before new_context, this is torn
    down after it, once its contexts are closed."""
    yield
    if TOKEN and "new_context" in request.fixturenames:
        api_call("GET", "api/v1/me")
    left = address_failures()
    assert not left, f"the test leaves failed token checks counted against the address (key, count, locked until): {left}"


def clear_preferences():
    """The token's user keeps the page's settings on the server, and the
    four cases share the server: its preferences are removed before
    every page, its notification rules and its marks of the log among
    them, so that a test starts from the defaults as a fresh browser
    does."""
    if not TOKEN:
        return
    for key in ("ui", "schemes", "notify", "log.seen"):
        api_call("DELETE", "api/v1/me/preferences/" + key)


@pytest.fixture
def page(context, case, watch, request):
    """The page loaded, with the token, the room tiles shown
    (helpers.wait_started: 30 s, longer than the page's own limit of a
    read, a timeout naming the reads still open) and, unless the test is
    marked used_ticket, the event stream open; the token's
    user without preferences on the server (clear_preferences); marked
    clock, under Playwright's clock. helpers.PAGE_KIT is in it before
    its own scripts, for the fixture stream, isolate_log and
    wait_log_live; what a route of safe_route's raises goes to the
    guard. Its default timeout and its waits take UI_TEST_SCALE
    (helpers.scaled)."""
    clear_preferences()
    if TOKEN:
        context.add_init_script(f"localStorage.setItem('massimo.token', {TOKEN!r})")
    context.add_init_script(PAGE_KIT)
    if request.node.get_closest_marker("clock") is not None:
        context.clock.install()
    page = context.new_page()
    report_to(page, watch.problems)
    page.set_default_timeout(scaled(5) * 1000)
    used = request.node.get_closest_marker("used_ticket") is not None
    if used:
        page.route("**/events/ticket", used_ticket_first(watch.refused))

    def on_console(m):
        src = m.location.get("url", "")
        # the refused stream, the refused search and what a test lists are on purpose
        expected = m.text.startswith("Failed to load resource") and (
            (watch.refused and USED in src) or (src.endswith("/cubes/discover") and "503" in m.text)
            or any(src.endswith(end) and str(status) in m.text for end, status in watch.expected))
        if m.type == "error" and not expected:
            watch.problems.append(f"console: {m.text}")

    page.on("request", lambda r: watch.streams.append(r.url) if "/events?" in r.url else None)
    page.on("console", on_console)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    api = ApiReads(page)
    page.goto(URL)
    wait_started(page, api)
    if not used and not wait_until(page, "state.eventsOpen", 10):
        raise RuntimeError("the event stream did not open")
    return page


@pytest.fixture
def reads(page):
    """The page's own reads answered with the server's data and what the
    test put in (helpers.Reads): the page takes its data anew from its
    poll and from the reads that events and actions start, so a test
    that puts data into the page tells it here, and the page keeps it."""
    return reads_of(page)


@pytest.fixture
def stream(page):
    """The page's event stream as the page took it, from a mark
    (helpers.Stream)."""
    return Stream(page)


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
    error fails the test itself, and pytest-playwright keeps its
    screenshot and, on run.sh's retry, its video and its trace. A test that
    fails on its own first carries the guard's problems in its report,
    often the cause (a route handler's error, a request that failed)."""
    guard = item.funcargs.get("guard")
    try:
        result = yield
    except BaseException as e:
        if guard is not None and guard.watch.problems:
            e.add_note("the guard's problems meanwhile: " + "; ".join(guard.watch.problems))
        raise
    if guard is not None:
        guard.verify()
    return result
