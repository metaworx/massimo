# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The fixtures of the web UI suite, run by tools/ui-test/run.sh (make
ui-test) against the server and the fake Cube it starts; it hands over
the page's address in UI_TEST_URL and the API token in UI_TEST_TOKEN.
In pytest-xdist's workers, which run.sh starts with a server set each,
worker gw<n> takes set n's addresses, log and fake Cube's control from
UI_TEST_URL_<n>, UI_TEST_DOWN_URL_<n>, UI_TEST_SERVE_LOG_<n> and
UI_TEST_CONTROL_URL_<n> in place of the plain names, before this module
or a test file reads them.

Each test file starts from the recorded Cube (known_cube, autouse for
the module): the worker's fake Cube goes back to the greeting it started
with, and the server has connected to it again and read it, before the
file's first test. The servers run without their poll of the Cube (run.sh,
-poll 0), so nothing of the server's own reaches the fake between a
test's steps. A file also leaves the server as it found it: one that
leaves a user, an integration, a setting or the like otherwise fails at
its last test's tear-down, naming what it left (server_left), but for
what no API takes back (LEFT_FOR_GOOD, with its reasons).

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
keeps on the server for the token's user are removed before every page,
and every warning of the log there is marked seen (clear_preferences).
The page fixture's default timeout and its waits, as every wait of
helpers.py, last their seconds times UI_TEST_SCALE (helpers.scaled),
which run.sh passes on and raises for its retry.

UI_TEST_REPEAT=<n>, a whole number from 1, runs every collected test n
times, its ids ending in 1of<n> to <n>of<n>; UI_TEST_SHUFFLE=<seed>, a
whole number, orders the collected tests by the seed, which the run's
header prints: the files in the order of a hash of the seed and each
file's name, a file's tests together in that of the seed and each
test's id, so that one file, a -k or a --last-failed keeps the order it
had in the whole run, and the same seed with the same tests gives the
same order. run.sh's workers (--dist loadfile, which hands out whole
files) are handed the files in that order rather than the largest
first, each running a file's tests in it; which worker takes which file
still depends on when each is free. A bad value of either stops the
run. The UI soak (.github/workflows/ui-soak.yml) runs each test three
times, shuffled (AP SteadyTests, Block 8).

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
sets the page's wall time with page.clock.set_system_time and an aware
datetime, which moves Date alone and runs no timer; a float would be
taken as seconds and multiplied by 1000, as with pause_at. A test
pauses it with helpers.pause_clock alone: a test module that calls
clock.pause_at itself fails its collection (pytest_collect_file).

A test marked browser_context_args, pytest-playwright's mark, gets its
context with those arguments on top of the case's viewport and locale:
timezone_id for a page in a zone of the test's own (test_room_updates.py).

test_login.py opens a context of its own, without the token, from
pytest-playwright's new_context with the case's viewport and locale.

A page without the token counts a failed token check against 127.0.0.1
with every API request it sends, and the files of a worker share its
server: ten within a minute, a sign-in's wrong password among them, lock
the address out for five minutes (429). After a test that used
new_context, one request with the token forgets them (the autouse
fixture failed_checks_forgotten); after every test, the set's store,
massimo.db beside UI_TEST_SERVE_LOG, has none counted against an address,
or the test errors at teardown. A sign-in a test makes leaves a session
on the server where its context closes without a sign-out: after every
test the sessions it made are ended (the autouse fixture ended_sessions).

The guard (autouse) fails a test on any console error, page error or a
horizontal scroll left at its end; the refused stream of a used_ticket
test and a stubbed 503 of the discovery are expected.

pytest-playwright keeps the screenshot of a failed test (pytest.ini)
under the directory run.sh names with --output, tools/ui-test/results/,
and the screenshot, the video and the trace of a failed test that run.sh
runs once more under results/retry/.
"""
import hashlib
import io
import json
import os
import re
import sqlite3
import ssl
import time
import tokenize
import urllib.error
import urllib.parse
import urllib.request

import pytest

_WORKER = os.environ.get("PYTEST_XDIST_WORKER", "")
if _WORKER.startswith("gw"):
    for _name in ("UI_TEST_URL", "UI_TEST_DOWN_URL", "UI_TEST_SERVE_LOG", "UI_TEST_CONTROL_URL"):
        _set = f"{_name}_{_WORKER[2:]}"
        if _set not in os.environ:
            raise RuntimeError(f"worker {_WORKER} has no server set ({_set}): give run.sh UI_TEST_WORKERS rather than pytest -n")
        os.environ[_name] = os.environ[_set]

pytest.register_assert_rewrite("helpers")

from helpers import (PAGE_KIT, USED, ApiReads, Stream, cleanup, reads_of, report_to, scaled, scroll_width, undo,  # noqa: E402
                     used_ticket_first, wait_started, wait_until)

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


# The times each test runs (UI_TEST_REPEAT, 1 where it is not set) and
# the seed of the tests' order (UI_TEST_SHUFFLE, None where it is not
# set), read once by pytest_configure.
REPEAT = pytest.StashKey[int]()
SHUFFLE = pytest.StashKey[int | None]()


def env_number(name, least, what):
    """The whole number the environment variable name holds, at least
    least; None where it is not set or empty. Any other value stops the
    run with what the variable takes, before any test runs."""
    value = os.environ.get(name, "")
    if value == "":
        return None
    if not re.fullmatch(r"[0-9]+", value) or int(value) < least:
        raise pytest.UsageError(f"{name} is {what}, not {value!r}")
    return int(value)


def pytest_configure(config):
    """UI_TEST_REPEAT and UI_TEST_SHUFFLE, read and checked once, in
    pytest-xdist's controller first, so that a bad value stops the run
    before its workers start."""
    config.stash[REPEAT] = env_number("UI_TEST_REPEAT", 1, "a whole number from 1, the times each test runs") or 1
    config.stash[SHUFFLE] = env_number("UI_TEST_SHUFFLE", 0, "a seed, a whole number such as 42")


def pytest_report_header(config):
    """The repeat and the seed in the run's header, so that a shuffled
    run can be run again in the same order."""
    lines = []
    n = config.stash[REPEAT]
    if n > 1:
        lines.append(f"UI_TEST_REPEAT={n}: each test {n} times, its ids ending in 1of{n} to {n}of{n}")
    seed = config.stash[SHUFFLE]
    if seed is not None:
        lines.append(f"UI_TEST_SHUFFLE={seed}: the files and each file's tests in the seed's order")
    return lines


@pytest.fixture
def ui_test_repeat(request):
    """The repeat a test runs as, from 1 (UI_TEST_REPEAT)."""
    return request.param


@pytest.hookimpl(trylast=True)
def pytest_generate_tests(metafunc):
    """Under UI_TEST_REPEAT=<n>, every test parametrized n times more,
    after the case and the browser, so that each repeat has an id of its
    own, 1of<n> to <n>of<n>, which --last-failed and xdist tell apart."""
    n = metafunc.config.stash[REPEAT]
    if n > 1:
        metafunc.fixturenames.append("ui_test_repeat")
        metafunc.parametrize("ui_test_repeat", range(1, n + 1), indirect=True, ids=lambda i: f"{i}of{n}")


def shuffled(items, seed):
    """The items in the seed's order: the files, as xdist's loadfile takes
    them (a node id's part before ::), by a hash of the seed and the
    file's name, each file's tests together, by one of the seed and the
    test's id. An item's place depends on its own name alone, so that any
    selection of the items keeps the order it has among them all."""
    def rank(name):
        return hashlib.sha256(f"{seed} {name}".encode()).digest()
    return sorted(items, key=lambda item: (rank(item.nodeid.split("::", 1)[0]), rank(item.nodeid)))


# The message of a mark cases that names no case, kept with its test.
BAD_CASES = pytest.StashKey[str]()


@pytest.hookimpl(trylast=True)
def pytest_collection_modifyitems(config, items):
    """Under UI_TEST_SHUFFLE, the collected tests in the seed's order
    (shuffled), after pytest's own order, which moves a file's tests
    without a browser behind all others (test_parts_guard.py), so that a
    file's tests stay together; then the cases (deselect_cases)."""
    seed = config.stash[SHUFFLE]
    if seed is not None:
        items[:] = shuffled(items, seed)
    deselect_cases(config, items)


def deselect_cases(config, items):
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
    load a page, with what the mark names. Under pytest-xdist each test
    carries its worker, gw<n>, whose server set it runs against, as the
    property worker of its JUnit report, which the soak's summary names
    (soak_summary.py; AP SteadyTests, Block 11)."""
    if _WORKER:
        item.user_properties.append(("worker", _WORKER))
    if BAD_CASES in item.stash:
        pytest.fail(item.stash[BAD_CASES], pytrace=False)


@pytest.fixture
def browser_context_args(browser_context_args, case):
    return {**browser_context_args, "viewport": {"width": case.width, "height": HEIGHT}, "locale": case.locale}


@pytest.fixture
def watch():
    return Watch()


def api_call(method, path, body=None, token=None, timeout=5):
    """A request to the server's API from the test itself, with the test
    token unless another is given, answered within timeout seconds; the
    answer's JSON, None for none."""
    tls = ssl.create_default_context()
    tls.check_hostname, tls.verify_mode = False, ssl.CERT_NONE
    headers = {"Authorization": "Bearer " + (token or TOKEN)}
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(URL.rstrip("/") + "/" + path.lstrip("/"), method=method, headers=headers, data=data)
    with urllib.request.urlopen(req, timeout=timeout, context=tls) as resp:
        text = resp.read()
    return json.loads(text) if text else None


# The fake Cube's control of the worker's set (massimo fakecube -control),
# which run.sh hands over in UI_TEST_CONTROL_URL; "" for a server started
# by hand, whose fake Cube is not reset.
CONTROL = os.environ.get("UI_TEST_CONTROL_URL", "")
# The Cube of the worker's set, the fake's serial (greeting.txt).
CUBE = "KEQ0000100"
# How long a reset may take, before the scale: the server connects again
# a second after the drop (its backoff), and reads the greeting.
RESET_SECONDS = 15
# How long the writes a file left may take to show, before the scale: the
# refresh that shows them reconnects to the Cube and reads its greeting.
SHOWN_SECONDS = 20


def newest_connected():
    """The id of the newest connected event in the set's Cube's log, 0
    for none."""
    events = api_call("GET", f"api/v1/cubes/{CUBE}/log?kind=connected&limit=1") or []
    return events[-1]["id"] if events else 0


def cube_entry():
    """The set's Cube as GET /api/v1/cubes lists it; {} where it lists
    none."""
    return next((c for c in api_call("GET", "api/v1/cubes") or [] if c.get("serial") == CUBE), {})


def writes_on_their_way():
    """The writes to the set's Cube whose devices do not show them yet,
    as the server keeps them (their confirmation, pending or
    unconfirmed, docs/api-v1.md): a line per device and per room, naming
    its fields; empty where there are none."""
    out = []
    for d in api_call("GET", f"api/v1/cubes/{CUBE}/devices") or []:
        c = d.get("confirmation")
        if c:
            out.append(f"device {d.get('rf_address')} ({d.get('name') or 'unlisted'}): "
                       f"{c.get('state')} {', '.join(c.get('fields') or [])}")
    for r in api_call("GET", f"api/v1/cubes/{CUBE}/rooms") or []:
        c = r.get("confirmation")
        if c:
            out.append(f"room {r.get('id')} ({r.get('name')}): {c.get('state')} {', '.join(c.get('fields') or [])}"
                       f" of {', '.join(c.get('devices') or [])}")
    return out


def writes_shown():
    """No write of a test is on its way when the fake Cube is reset:
    without its poll the server reads the devices' states only at a
    connect, so where one is on its way it reads the Cube again (POST
    …/refresh), whose greeting shows what the fake applied at once, and
    the server has none left. A reset that came first would take the
    write back, and the server would count it as not confirmed five
    minutes later and send a room's setpoint again, into a later file.
    One still on its way after SHOWN_SECONDS, scaled, fails, naming the
    device or the room and the write's fields."""
    left = writes_on_their_way()
    if not left:
        return
    limit = scaled(SHOWN_SECONDS)
    try:
        api_call("POST", f"api/v1/cubes/{CUBE}/refresh", {}, timeout=limit)
    except (OSError, ValueError) as e:  # urllib's errors, a timeout among them
        raise RuntimeError(f"the refresh to show the writes on their way before the fake Cube's reset failed ({e}): "
                           f"{'; '.join(left)}") from e
    deadline = time.monotonic() + limit
    while left:
        if time.monotonic() > deadline:
            raise RuntimeError(f"a write still on its way before the fake Cube's reset, {limit:g} s after the refresh "
                               f"that was to show it: {'; '.join(left)}")
        time.sleep(0.05)
        left = writes_on_their_way()


def reset_cube():
    """The fake Cube back to the greeting it started with (its control's
    POST /reset), and the server connected to it again: the reset drops
    the server's connection, and the server connects again and reads
    the greeting. Done once a connected event newer than the reset is in
    the Cube's log and the Cube is listed connected; a timeout names
    what it waited for and what it saw."""
    before = newest_connected()
    req = urllib.request.Request(CONTROL.rstrip("/") + "/reset", method="POST")
    with urllib.request.urlopen(req, timeout=5) as resp:
        if resp.status != 204:
            raise RuntimeError(f"the fake Cube's reset (POST {CONTROL}/reset) answered {resp.status}")
    limit = scaled(RESET_SECONDS)
    deadline = time.monotonic() + limit
    while True:
        newest, entry = newest_connected(), cube_entry()
        link = (entry.get("link") or {}).get("state")
        if newest > before and entry.get("connected") and link == "connected":
            return
        if time.monotonic() > deadline:
            raise RuntimeError(
                f"the fake Cube's reset: the server did not connect to it again and read it within {limit:g} s: "
                f"waited for a connected event in {CUBE}'s log newer than {before} (the newest is {newest}) "
                f"and {CUBE} listed connected (connected {entry.get('connected')}, link {link})")
        time.sleep(0.05)


# What a test file may leave on the worker's server without failing the
# check of what it left (server_left), each with its reason (AP
# SteadyTests, Block 11, D11-A): no API takes it back, or the kit does
# before every page. server_as_found reads none of it.
LEFT_FOR_GOOD = {
    "the log's lines": "the server keeps every line it logs; no API removes one",
    "the log's warnings": "a warning stays until a later event settles it or a person marks it seen, a login_failed until a "
                          "sign-in of the same name (docs/api-v1.md, GET /api/v1/log/alerts); clear_preferences marks every "
                          "one present seen before every page",
    "an integration registered, disabled, with its users and its role's count of them": "DELETE …/integrations/{uuid} "
                          "revokes its key and disables it, its users, links and maximum kept for a request from the same "
                          "installation, and no route removes them: a test registers it with an installation of its own "
                          "(helpers.own_id), so that its next run makes another rather than join it",
    "a Cube found by a search": "a Cube the server found stays found while it answers a search, which the fake Cube does at "
                                "every one, and no API forgets it; a test that needs none found tells its page's reads "
                                "(helpers.configured_only)",
    "the token's user's preferences": "the page fixture removes them before every page (clear_preferences)",
}


def server_as_found():
    """What the API shows of the worker's server that a test file may
    leave changed, by kind, each entry by its key with what the check
    compares of it: the users, but an integration's that is disabled,
    with their sessions and keys and their identities; the roles; the
    providers; the integrations and the requests waiting, but those
    disabled; the settings' stored values; the Cubes configured; the
    token's user's push subscriptions; and whether the set's Cube pairs.
    What LEFT_FOR_GOOD names is left out."""
    integrations = api_call("GET", "api/v1/integrations") or []
    disabled = {i["uuid"] for i in integrations if i.get("state") == "disabled"}
    found = {"user": {}, "session or key": {}, "identity": {}}
    for u in api_call("GET", "api/v1/users") or []:
        mine = (u.get("integration") or {}).get("uuid")
        if mine in disabled:
            continue
        name = u["name"]
        found["user"][name] = ", ".join(f"{k} {u.get(k)}" for k in ("role", "display_name", "disabled", "has_password", "auth_version",
                                                                   "expires_at"))
        for t in api_call("GET", f"api/v1/users/{_quoted(name)}/tokens") or []:
            found["session or key"][f"{name} #{t['id']}"] = f"{t.get('kind')} {t.get('label', '')}".strip()
        if not mine:
            for i in api_call("GET", f"api/v1/users/{_quoted(name)}/identities") or []:
                found["identity"][f"{name} #{i['id']}"] = f"{i.get('provider')} {i.get('email') or i.get('subject') or ''}".strip()
    found["role"] = {r["key"]: f"capabilities {', '.join(r.get('capabilities') or [])}, disabled {r.get('disabled')}, name {r.get('name', '')}"
                     for r in (api_call("GET", "api/v1/roles") or {}).get("roles", [])}
    found["provider"] = {p["key"]: json.dumps({k: v for k, v in p.items() if k not in ("created_at", "updated_at")}, sort_keys=True)
                         for p in api_call("GET", "api/v1/providers") or []}
    found["integration"] = {i["uuid"]: f"{i.get('state')} {i.get('name')}, max {', '.join(i.get('max_caps') or [])}"
                            for i in integrations if i["uuid"] not in disabled}
    found["setting"] = {k: json.dumps(v.get("value")) for k, v in (api_call("GET", "api/v1/settings") or {}).items()}
    found["Cube"] = {c["serial"]: f"{c.get('name')} at {c.get('address')}" for c in api_call("GET", "api/v1/cubes") or [] if c.get("configured")}
    push = api_call("GET", "api/v1/me/push") or []
    found["push subscription"] = {str(p.get("id")): str(p.get("label", "")) for p in (push if isinstance(push, list) else [])}
    try:  # a pairing left running ends by its timer, in a later file, with a pairing_ended event
        found["pairing"] = {CUBE: "running" if (api_call("GET", f"api/v1/cubes/{CUBE}/pairing") or {}).get("active") else "off"}
    except urllib.error.HTTPError as e:
        found["pairing"] = {CUBE: f"not read (HTTP {e.code})"}
    return found


def server_left(before):
    """Fails, naming each, where the worker's server shows otherwise than
    before, a server_as_found of the file's start: what the file added,
    removed or changed, with what it was and is."""
    after = server_as_found()
    left = []
    for kind in before:
        b, a = before[kind], after.get(kind, {})
        left += [f"{kind} {k} added ({a[k]})" for k in sorted(a.keys() - b.keys())]
        left += [f"{kind} {k} removed ({b[k]})" for k in sorted(b.keys() - a.keys())]
        left += [f"{kind} {k} changed from {b[k]} to {a[k]}" for k in sorted(a.keys() & b.keys()) if a[k] != b[k]]
    if left:
        raise RuntimeError("the file leaves the server otherwise than it found it: " + "; ".join(left)
                           + " (what no API takes back is not counted: conftest.py, LEFT_FOR_GOOD)")


@pytest.fixture(scope="module", autouse=True)
def known_cube():
    """Each test file starts from the recorded Cube and leaves no write
    on its way (AP SteadyTests, Block 5): before its first test the
    worker's fake Cube is reset and the server has read it again
    (reset_cube), so that no earlier file's write, pairing or room list
    reaches it; before the reset, and after the file's last test, every
    write the server waits for its devices to show is shown
    (writes_shown), so that a failure names the file that left it. It
    also leaves the server as it found it (AP SteadyTests, Block 11,
    D11-A): what the API shows of the server's users, integrations,
    settings and the rest (server_as_found) is noted after the reset,
    and after the file's last test the file fails, at that test's
    tear-down, where it shows otherwise, naming what the file left
    (server_left); LEFT_FOR_GOOD lists what is not counted, and why.
    Without the control (UI_TEST_CONTROL_URL) or the token, as against a
    server started by hand, nothing is reset or checked."""
    if not (CONTROL and TOKEN):
        yield
        return
    writes_shown()
    reset_cube()
    found = server_as_found()
    yield
    cleanup(writes_shown, lambda: server_left(found))


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


def _quoted(name):
    """A user's name as a segment of a path: an integration's users' hold ':'."""
    return urllib.parse.quote(name, safe="")


def live_sessions():
    """The sessions the set's server holds, as (user name, token id), of
    every user but an integration's, which signs in with no session: a
    sign-in's and a key's sign-in's (kind session), not a key."""
    out = set()
    for u in api_call("GET", "api/v1/users") or []:
        if u.get("integration"):
            continue
        for t in api_call("GET", f"api/v1/users/{_quoted(u['name'])}/tokens") or []:
            if t.get("kind") == "session":
                out.add((u["name"], t["id"]))
    return out


@pytest.fixture(autouse=True)
def ended_sessions():
    """A test's sign-in, in a context of its own (new_context, owner_page)
    or in its page, leaves its session on the server where the context
    closes without a sign-out, for whichever test the worker runs next:
    after every test the sessions it made are ended (DELETE
    …/users/{name}/tokens/{id}, undo), so that it leaves none (AP
    SteadyTests, Block 11). Set up before the test's contexts, the
    page's among them, this is torn down after them: pytest sets up a
    conftest's autouse fixtures in the order of their names, and this
    one's comes before guard's, which loads the page. Without the token,
    nothing."""
    if not TOKEN:
        yield
        return
    before = live_sessions()
    yield
    cleanup(*[lambda name=name, sid=sid: undo(f"{name}'s session {sid}",
                                              lambda: api_call("DELETE", f"api/v1/users/{_quoted(name)}/tokens/{sid}"))
              for name, sid in sorted(live_sessions() - before)])


def clear_preferences():
    """The token's user keeps the page's settings on the server, and the
    four cases share the server: its preferences are removed before
    every page, its notification rules and its marks of the log among
    them, so that a test starts from the defaults as a fresh browser
    does. Then every warning of the log the server holds is marked seen,
    log.seen up to the newest alert's id (AP SteadyTests, Block 11): a
    warning stays until a later event settles it or a person marks it,
    a refused sign-in's login_failed until a sign-in of the same name,
    and one an earlier test left put the amber mark on ☰ for every later
    page of the worker. A warning made once the page has loaded is
    newer, and shows."""
    if not TOKEN:
        return
    for key in ("ui", "schemes", "notify", "log.seen"):
        api_call("DELETE", "api/v1/me/preferences/" + key)
    alerts = (api_call("GET", "api/v1/log/alerts") or {}).get("alerts") or []
    if alerts:
        api_call("PUT", "api/v1/me/preferences/log.seen", {"all": max(a["id"] for a in alerts), "lines": []})


# SETTINGS_DEFAULTS are the page's settings in a browser that keeps none
# (storedSettings() of app.js), the preference ui as the page sends it.
SETTINGS_DEFAULTS = {"poll": 10, "expert": False, "clock": "browser", "lang": "auto", "theme": "system", "scheme": "massimo",
                     "blink": "system", "iconWindow": "sash", "iconRadio": "wifi"}


def server_holds_settings(holds, token=None, **values):
    """Sets whether the server holds the settings of the token's user, the
    test token's (the page fixture's user) unless another is given: with
    holds, the preference ui with values over the page's defaults
    (SETTINGS_DEFAULTS); without, none, as clear_preferences leaves it.
    A sign-in and a page's load take the settings the server holds, and
    hand a server that holds none the browser's (loadPreferences of
    login.js), so a test whose outcome depends on which sets it at the
    moment that decides, rather than taking what the page's own pushes or
    an earlier test left (docs/ui-testing.md, section 9; AP SteadyTests
    v1.1, D14-A): while its page is signed out, or before the load,
    where no push of the page's own comes after it."""
    if holds:
        api_call("PUT", "api/v1/me/preferences/ui", {**SETTINGS_DEFAULTS, **values}, token=token)
    else:
        api_call("DELETE", "api/v1/me/preferences/ui", token=token)


@pytest.fixture
def page(context, case, watch, request):
    """The page loaded, with the token, the room tiles shown
    (helpers.wait_started: 30 s, longer than the page's own limit of a
    read, a timeout naming the reads still open) and, unless the test is
    marked used_ticket, the event stream open; the token's
    user without preferences on the server and every warning of the
    log seen (clear_preferences); marked
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
