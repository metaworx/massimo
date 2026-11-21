# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""What the tests of the web UI suite share: the texts they expect in
English and German, the fixed answers they stub, and the helpers that
wait on the page or look at all of it.

The page's Content-Security-Policy rules out wait_for_function with a
string, so waiting on the page's state polls page.evaluate from here
(wait_until). `state` is a top-level const of app.js, in the page's
global scope but not on window; page.evaluate reaches it there.

What the page reads and what its stream brings (docs/ui-testing.md,
section 9): Reads answers the page's own reads with the server's data
and what a test put in; Stream watches the page's event stream from a
mark; isolate_log keeps a log of the test's own lines; wait_log_live
waits until the log takes the stream's state events; safe_route is the
one route handler that lets go a request the page no longer waits for;
undo is a cleanup that says what it could not undo; held_poll holds the
page's poll while a test measures what the page does by itself.
"""
import contextlib
import json
import re
import sys
import time
import urllib.error
import weakref
from urllib.parse import parse_qs, urlparse

from playwright.sync_api import Error

OPEN = {"en": "open", "de": "offen"}
USED = "0" * 48
FOUND2 = {"serial": "LEQ0000506", "configured": False, "address": "192.168.1.78", "rf_address": "0a1b2e",
          "firmware": "1.1.3", "found_at": "2026-11-05T18:40:00+01:00", "last_seen": "2026-11-05T18:40:00+01:00"}
CHANGES = {"en": "window opened, set 18.0 → 5.0", "de": "Fenster geöffnet, Soll 18,0 → 5,0"}
SEARCHING = {"en": "Searching…", "de": "Suche läuft…"}
FOUND_N = {"en": "{n} Cube{s} found, not configured", "de": "{n} Cube{s} gefunden, nicht eingerichtet"}
STALE = {"en": "Not seen for 30 hours", "de": "Seit 30 Stunden keine Meldung"}
ECO_BUTTON = {"en": "eco button", "de": "Eco-Taster"}
CONTRAST_SCHEME = {"en": "High contrast", "de": "Hoher Kontrast"}
SAVED = {"en": "Saved as “Test scheme”.", "de": "Als „Test scheme“ gespeichert."}
HEX_LABEL = {"en": "Lines, hex", "de": "Linien, Hex"}
HEX_BAD = {"en": "Lines: “nope” is not a colour. Write it as #rrggbb or #rgb, the # may be left out; Escape puts the colour back.",
           "de": "Linien: „nope“ ist keine Farbe. Als #rrggbb oder #rgb schreiben, das # darf fehlen; Escape stellt die Farbe wieder her."}
PANEL_TEXTS = {"en": ["Address", "Hidden devices", "Save"], "de": ["Adresse", "Ausgeblendete Geräte", "Speichern"]}
NAME_BAD = {"en": "Only the letters a to z, digits and . _ - @, without spaces, starting with a letter or a digit; at most 64 characters.",
            "de": "Nur die Buchstaben a bis z, Ziffern und . _ - @, ohne Leerzeichen, am Anfang ein Buchstabe oder eine Ziffer; höchstens 64 Zeichen."}
NAME_TAKEN = {"en": "There is already a user named {name}.", "de": "Es gibt schon einen Benutzer {name}."}
NOT_A_SCHEME = {"en": "Not a Massimo colour scheme: unknown format", "de": "Kein Massimo-Farbschema: unbekanntes Format"}


def wait_until(page, expr, seconds):
    """Polls a page expression from here: the page's CSP rules out
    wait_for_function with a string."""
    end = time.time() + seconds
    while time.time() < end:
        if page.evaluate(expr):
            return True
        page.wait_for_timeout(200)
    return False


def scroll_settled(page, seconds=3):
    """Waits until the page has not scrolled for ten animation frames;
    false if it is still scrolling after so many seconds. Chromium runs a
    smooth scroll on the compositor, and an instant scroll sent while it
    ends can be undone by it: the page back where the smooth scroll was
    going. Frames, not polls from here: two equal readings 200 ms apart
    can be one pause of a starved renderer."""
    return page.evaluate("""(ms) => new Promise((done) => {
        const end = performance.now() + ms;
        let last = window.scrollY, still = 0;
        const frame = () => {
            if (window.scrollY !== last) {
                last = window.scrollY;
                still = 0;
            } else if (++still >= 10) {
                return done(true);
            }
            if (performance.now() > end) return done(false);
            requestAnimationFrame(frame);
        };
        requestAnimationFrame(frame);
    })""", seconds * 1000)


class ApiReads:
    """The page's requests to the API that have no answer yet, counted from
    the moment it is made: a page shown sends its cards' reads at once, and
    their answers come in any order, so a card that grows with a later one
    (System settings → Integrations, with why the server does not look on
    the LAN, api/v1/integrations/found) changes the page's height after the
    answer a test waited for. The event stream, which stays open, is left
    out."""

    def __init__(self, page):
        self.page = page
        self.open = set()
        page.on("request", self._sent)
        page.on("requestfinished", self._done)
        page.on("requestfailed", self._done)

    @staticmethod
    def _api(request):
        return "/api/v1/" in request.url and not re.search(r"/events(\?|$)", request.url)  # not the stream (app.js connectEvents)

    def _sent(self, request):
        if self._api(request):
            self.open.add(request)

    def _done(self, request):
        self.open.discard(request)

    def answered(self, seconds=5, quiet_ms=200):
        """Waits until every read has its answer and nothing more is drawn
        for quiet_ms (the page's height unchanged); false after so many
        seconds."""
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            if not self.open and self.page.evaluate("""(quiet) => new Promise((done) => {
                const h = document.documentElement.scrollHeight;
                setTimeout(() => done(document.documentElement.scrollHeight === h), quiet);
            })""", quiet_ms) and not self.open:
                return True
            self.page.wait_for_timeout(25)
        return False


def scroll_width(page):
    """The page's width with what sticks out of it; more than the
    viewport's width is a horizontal scroll."""
    return page.evaluate("document.documentElement.scrollWidth")


def named_buttons(page, where):
    """Every button of the page has an accessible name: its text, an
    aria-label, an aria-labelledby or a title. The scripts build most of
    them, so this runs on every page the tests open."""
    nameless = page.evaluate("""() => [...document.querySelectorAll('button')].filter((b) => {
      const by = (b.getAttribute('aria-labelledby') || '').split(/\\s+/).map((id) => document.getElementById(id))
        .filter((e) => e).map((e) => e.textContent).join('');
      return ![b.textContent, b.getAttribute('aria-label'), b.title, by].some((n) => n && n.trim());
    }).map((b) => b.outerHTML.slice(0, 160))""")
    assert not nameless, f"every button has a name on {where}: {nameless}"


def found_text(lang, n):
    return FOUND_N[lang].format(n=n, s="" if n == 1 else "s")


def open_found(page):
    """Opens System settings → Cubes' fold of what was found on the
    network, where it is closed."""
    fold = page.locator("#cubes-list .found-panel")
    fold.wait_for(state="attached")
    if not fold.evaluate("(d) => d.open"):
        fold.locator(":scope > summary").click()


def update_json(state, to, available, manager=None, channel="alpha", suite=None):
    """GET /api/v1/update's answer with the update helper in the given
    state, the package manager that owns the binary, if any, the channel
    in effect and apt's suite; the build's flavour is alpha."""
    return ('{"current": "v0.0.0-test", "flavour": "alpha", "channel": "%s", %s%s"latest": "%s", "available": %s, "can_update": %s, '
            '"helper": {"state": "%s", "to": "%s", "time": "2026-01-01T00:00:00Z"}}'
            % (channel, '"manager": "%s", ' % manager if manager else "", '"suite": "%s", ' % suite if suite else "",
               to, "true" if available else "false", "true" if available else "false", state, to))


def used_ticket_first(refused):
    """A route handler for the ticket requests: the first one gets a used
    ticket, so the server refuses that stream with a 401; the others go
    through. (A handler with a second parameter would get the request.)"""

    def handle(route):
        if not refused:
            refused.append(USED)
            route.fulfill(status=200, content_type="application/json", body='{"ticket": "%s"}' % USED)
        else:
            route.continue_()

    return handle


def hold(page, pattern, method, held, sent):
    """Holds the requests of a method to pattern until the test lets them
    go (route.continue_() or route.fulfill()), as a slow server would,
    and counts them in sent; other methods go through."""

    def handle(route):
        if route.request.method == method:
            sent.append(route.request.post_data_json if route.request.post_data else route.request.url)
            held.append(route)
        else:
            route.continue_()

    page.route(pattern, handle)


def wait_held(page, held, seconds=5):
    """Waits until a held request has reached its route."""
    end = time.time() + seconds
    while not held and time.time() < end:
        page.wait_for_timeout(100)
    return bool(held)


@contextlib.contextmanager
def held_poll(page):
    """The page's poll held for the with block: its timer cleared, and
    started again at the end (startPolling()), for a test that measures
    what the page does by itself, which a poll would change (docs/ui-
    testing.md, section 9). Whatever calls startPolling() meanwhile ends
    the hold unseen: applySettings() (a setting changed, expert mode, a
    sign-in), the poll's field, a sign-out. So a block that ends without
    an error of its own fails where the timer is no longer the one the
    hold cleared."""
    timer = page.evaluate("(() => { const t = state.pollTimer; clearInterval(t); return t; })()")
    try:
        yield
        now = page.evaluate("state.pollTimer")
        assert now == timer, f"the page's poll stayed held: something started it again meanwhile (timer {timer}, now {now})"
    finally:
        if not page.is_closed():
            page.evaluate("startPolling()")


# --- what the page reads and what its stream brings -------------------

# PAGE_KIT is put into every page of the page fixture before the page's
# own scripts (conftest.py). It wraps the page's EventSource, so that
# every event the page's listeners take is recorded in
# window.__uiTest.events, in order, the server's and those a test
# dispatches, each with whether it is trusted (the server's) and how many
# times the page handed it to appendLogEvent (Stream reads them); and,
# once the page's scripts are in,
# its appendLogEvent, which isolate_log can have keep the server's lines
# out and wait_log_live probes. The page itself is not changed: it calls
# its own functions by their global names.
PAGE_KIT = """(() => {
  const k = window.__uiTest = { events: [], streams: 0, fromServer: false, isolated: false, probes: 0, probed: 0, appends: 0 };
  const Real = window.EventSource;
  if (!Real) return;
  const add = EventTarget.prototype.addEventListener;
  const remove = EventTarget.prototype.removeEventListener;
  const wrapped = new WeakMap();
  const seen = new WeakMap();
  const record = (es, ev, logged) => {
    let r = seen.get(ev);
    if (!r) {
      const data = typeof ev.data === "string" ? ev.data : null;
      r = { n: k.events.length, kind: ev.type, id: ev.lastEventId || "", data, server: ev.isTrusted, stream: es.__uiStream || 0,
        logged: 0, probe: !!data && data.includes('"__probe"'), t: Math.round(performance.now()) };
      seen.set(ev, r);
      k.events.push(r);
    }
    r.logged += logged;
  };
  class Watched extends Real {
    constructor(url, opts) {
      super(url, opts);
      this.__uiStream = ++k.streams;
      for (const kind of ["open", "error"]) add.call(this, kind, (ev) => record(this, ev, 0));
    }
    addEventListener(type, fn, opts) {
      if (typeof fn !== "function") return add.call(this, type, fn, opts);
      let w = wrapped.get(fn);
      if (!w) {
        w = function (ev) {
          const was = k.fromServer;
          const appends = k.appends;
          k.fromServer = ev.isTrusted;
          try {
            return fn.call(this, ev);
          } finally {
            k.fromServer = was;
            record(this, ev, k.appends - appends);
          }
        };
        wrapped.set(fn, w);
      }
      return add.call(this, type, w, opts);
    }
    removeEventListener(type, fn, opts) {
      return remove.call(this, type, wrapped.get(fn) || fn, opts);
    }
  }
  window.EventSource = Watched;
  document.addEventListener("DOMContentLoaded", () => {
    if (typeof window.appendLogEvent !== "function") return;
    const append = window.appendLogEvent;
    window.appendLogEvent = function (ev) {
      if (ev && ev.__probe) {
        k.probed = ev.__probe;
        return;
      }
      k.appends++;
      if (k.isolated && k.fromServer) return;
      return append.apply(this, arguments);
    };
  });
})();"""

# The guard's problems of each page (report_to), where safe_route says
# what a route handler of the test's raised.
_PROBLEMS = weakref.WeakKeyDictionary()


def report_to(page, problems):
    """Has safe_route add what a route handler of the test's raised on
    page to problems, the guard's (conftest.py's page fixture)."""
    _PROBLEMS[page] = problems


def report(page, problem):
    """A problem of a route handler's for the guard of page, printed at
    once as well: the test may fail on its own before the guard looks
    (conftest.py adds the guard's problems to such a failure)."""
    print("ui-test: " + problem, file=sys.stderr)
    if page in _PROBLEMS:
        _PROBLEMS[page].append(problem)


def safe_route(page, pattern, handler):
    """page.route with handler, the one place that lets go a request the
    page no longer waits for. Where the handler meets Playwright's Error,
    the request is aborted: where the page closed with the request on
    its way, or the request was answered already, the abort fails as
    well and that is all, since neither is the test's; where the abort
    goes through, the page still waited for it (the server did not
    answer route.fetch(), say), and that goes to the guard's problems,
    which fail the test. Anything else the handler raises (a failed
    assertion, a typo) is the test's too: Playwright would only print it,
    so it goes to the guard's problems and the request is aborted, so
    that the page does not wait for it. Returns the handler as routed,
    for page.unroute."""

    def routed(route):
        try:
            handler(route)
        except Error as e:
            try:
                route.abort()
            except Error:
                return  # the page closed, or the request was answered
            first = (str(e).splitlines() or [""])[0]  # without Playwright's call log
            report(page, f"the route of {pattern} met {first}; the request was aborted")
        except Exception as e:  # noqa: BLE001 - reported to the guard, not swallowed
            report(page, f"the route of {pattern} raised {type(e).__name__}: {e}")
            try:
                route.abort()
            except Error:
                pass

    page.route(pattern, routed)
    return routed


CUBES = "**/api/v1/cubes"


def configured_only(cubes):
    """GET /api/v1/cubes without the Cubes found on the network."""
    return [c for c in cubes if c.get("configured") is not False]


def found_only(cubes):
    """GET /api/v1/cubes with the Cubes found on the network alone: the
    list gives no Cube the server serves."""
    return [c for c in cubes if c.get("configured") is False]


class Reads:
    """The page's own reads answered with the server's data and what the
    test put in (D13-A of AP TestInfrastructure). The page takes its data
    anew from every read: its poll, every 10 s by default, and the reads
    that events and actions start (loadCubes, loadRooms, refresh), so
    data a test put into the page alone, into `state` or by an event, is
    gone at the next one. What a test tells this is in the server's
    answer from then on: every GET of the pattern is answered with the
    server's answer, changed by each change told for it in turn, until
    the test clears it or its page closes. The test then shows it by a
    read of the page's own, such as loadCubes(); other methods than GET
    go on to the test's other routes or the server. One per page
    (reads_of), the fixture reads of conftest.py; it holds its page
    weakly, so that the page's entry in reads_of goes with the page."""

    def __init__(self, page):
        self._page = weakref.ref(page)
        self.changes = {}  # pattern: [the changes told, in turn]
        self.routed = {}  # pattern: the handler as routed
        self.cubes = {}  # serial: the fields told for the Cube
        self.found_cubes = {}  # serial: a found Cube's whole entry

    @property
    def page(self):
        """The page, None once it is gone."""
        return self._page()

    def change(self, pattern, fn):
        """Every GET of pattern from now on answered with fn(the answer
        so far), the server's JSON with the changes told before; fn may
        change it in place and returns it. The same function told again
        for the pattern is applied once (a new lambda is another)."""
        if pattern not in self.changes:
            self.changes[pattern] = []
            self.routed[pattern] = safe_route(self.page, pattern, lambda route: self._answer(pattern, route))
        if fn not in self.changes[pattern]:
            self.changes[pattern].append(fn)

    def _answer(self, pattern, route):
        if route.request.method != "GET":
            route.fallback()
            return
        answer = route.fetch()
        if not answer.ok:
            route.fulfill(response=answer)
            return
        text = answer.text()
        body = json.loads(text) if text else None
        for fn in list(self.changes.get(pattern, ())):
            body = fn(body)
        # the server's own headers, its Date among them, which the page
        # takes the server's clock from; the length is the new body's
        headers = {k: v for k, v in answer.headers.items() if k.lower() != "content-length"}
        route.fulfill(status=answer.status, headers=headers, body=json.dumps(body))

    def clear(self, pattern=CUBES):
        """The reads of pattern answered by the server alone again, what
        was told for them forgotten."""
        if pattern == CUBES:
            self.cubes.clear()
            self.found_cubes.clear()
        self.changes.pop(pattern, None)
        routed = self.routed.pop(pattern, None)
        page = self.page
        if routed is not None and page is not None and not page.is_closed():
            page.unroute(pattern, routed)

    def cube(self, serial, **fields):
        """The server's entry of the Cube serial in GET /api/v1/cubes with
        fields in it, from now on, in place of those told for it before;
        without fields, the server's own entry again."""
        self._told_cubes()
        self.cubes[serial] = fields

    def found(self, cube):
        """A Cube found on the network in GET /api/v1/cubes from now on, in
        place of the server's entry of its serial: a server that found a
        Cube lists it from then on, once per serial."""
        self._told_cubes()
        self.found_cubes[cube["serial"]] = cube

    def _told_cubes(self):
        self.change(CUBES, self._cubes_told)  # once, beside other changes of the list

    def _cubes_told(self, cubes):
        listed = [dict(c, **self.cubes.get(c.get("serial"), {})) for c in cubes if c.get("serial") not in self.found_cubes]
        return listed + list(self.found_cubes.values())


_READS = weakref.WeakKeyDictionary()


def reads_of(page):
    """The page's Reads, made at the first call: a helper that has the
    page alone reaches the one its test holds."""
    if page not in _READS:
        _READS[page] = Reads(page)
    return _READS[page]


class Stream:
    """The page's event stream as the page took it (PAGE_KIT): every
    event its listeners handled, the server's and those a test
    dispatched, in order, each recorded once the page's listener has run.
    A mark says "from now on"; wait returns the first event from a mark
    that matches and skips the rest, and on a timeout says what did come
    since the mark. A mark holds within one document: a reload starts
    the record again, and the test marks again after it. The probes of
    wait_log_live are left out. The fixture stream of conftest.py."""

    def __init__(self, page):
        self.page = page

    def mark(self):
        """From now on: the number of events recorded so far."""
        return self.page.evaluate("window.__uiTest.events.length")

    def since(self, mark=0):
        """The events recorded since mark, as dicts: n (its place in the
        record), kind, id, data (the event's JSON), server (the event's
        isTrusted: true for what the server sent and for the browser's own
        open and error, false for what a test dispatched), stream (the
        page's stream it came on, counted from 1), logged (how many times
        the page handed it to appendLogEvent, those isolate_log keeps out
        of the log among them), t (the page's milliseconds)."""
        events = self.page.evaluate("(m) => window.__uiTest.events.slice(m)", mark)
        for e in events:
            try:
                e["data"] = json.loads(e["data"]) if e.get("data") else None
            except ValueError:
                pass  # not JSON: the text as it came
        return [e for e in events if not e["probe"]]

    def wait(self, what, kind, match=None, mark=0, server=None, seconds=5):
        """The first event of kind since mark for which match(data) holds
        (any where match is None), trusted or dispatched by a test where
        server says which (true for the server's), what naming it; on a
        timeout an AssertionError with the events that came since the
        mark."""
        end = time.time() + seconds
        cursor = mark
        while True:
            for e in self.since(cursor):
                cursor = e["n"] + 1
                if e["kind"] == kind and (server is None or e["server"] == server) and (match is None or match(e["data"])):
                    return e
            if time.time() >= end:
                break
            self.page.wait_for_timeout(100)
        seen = self.since(mark)
        shown = ", ".join(describe_event(e) for e in seen[-20:])
        raise AssertionError(f"{what}: no {kind} event within {seconds} s; {len(seen)} since the mark{': ' if seen else ''}{shown}")


def describe_event(e):
    """An event of Stream in a few words: its kind, its device where it
    has one, and who sent it."""
    data = e.get("data")
    device = data.get("device") if isinstance(data, dict) else None
    return " ".join(x for x in (e["kind"], device, "(server)" if e["server"] else "(test)") if x)


# The probe of wait_log_live: a state event without a state, which the
# page's state listener hands to appendLogEvent once the stream's first
# 1.5 s are over, and PAGE_KIT's wrapper takes out before the log.
LOG_PROBE = """() => {
  const k = window.__uiTest;
  if (!k || !state.events) return false;
  const n = ++k.probes;
  state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({ kind: "state", __probe: n }) }));
  return k.probed === n;
}"""


def wait_log_live(page, seconds=5):
    """Waits until the log takes the stream's state events: the page
    leaves a stream's first 1.5 s out of the log, the burst of states a
    connection starts with, and again after a resync (app.js,
    connectEvents). A probe, which the log never shows, is sent until
    the page's state listener hands one on to the log."""
    assert wait_until(page, LOG_PROBE, seconds), "the log takes the stream's state events (a stream's first 1.5 s are left out)"


# The log's reads of a Cube: its pages, not its alerts (api/v1/log/alerts)
# nor every Cube's (api/v1/log).
CUBE_LOG = re.compile(r"/api/v1/cubes/[^/]+/log\?")


def no_lines(lines):
    """A page of the log without a line."""
    return []


def isolate_log(page, reads=None):
    """Has the log hold the test's own lines alone: the lines of the
    events the server sends on the stream are kept out of the log from
    now on (the page's other work on them goes on); with reads, the log's
    reads of the Cube are answered with no line and the log is loaded
    anew, so that it starts empty. A test that answers the log's reads
    itself gives no reads."""
    page.evaluate("window.__uiTest.isolated = true")
    if reads is not None:
        reads.change(CUBE_LOG, no_lines)
        page.evaluate("loadLog()")
        assert wait_until(page, "!logState.loading && logState.lines.length === 0", 5), "the log loaded, without a line of the server's"


def undo(what, call, absent=(404,)):
    """A cleanup's step: call(), what naming what it undoes. An answer
    with a status of absent means there was nothing to undo; anything
    else that fails is said, so that a test which leaves the shared
    server changed fails where it did, not a later one on its worker."""
    try:
        return call()
    except urllib.error.HTTPError as e:
        if e.code in absent:
            return None
        raise AssertionError(f"the cleanup could not undo {what}: HTTP {e.code} {e.reason}") from e
    except Exception as e:  # noqa: BLE001 - said, with what it could not undo
        raise AssertionError(f"the cleanup could not undo {what}: {type(e).__name__}: {e}") from e


def cleanup(*steps):
    """Runs every step of a cleanup in turn, each a callable without
    arguments, one that fails among them, so that a failed step leaves no
    later one undone; the first error is raised once all have run, the
    later ones as its notes."""
    errors = []
    for step in steps:
        try:
            step()
        except Exception as e:  # noqa: BLE001 - raised below, the first of them
            errors.append(e)
    if errors:
        for e in errors[1:]:
            errors[0].add_note(f"and then: {type(e).__name__}: {e}")
        raise errors[0]


def busy_state(page, selector):
    """A button's [disabled, turning (class busy), aria-busy]."""
    return page.evaluate("(() => { const b = document.querySelector(%r); return b && [b.disabled, b.classList.contains('busy'), "
                         "b.getAttribute('aria-busy')]; })()" % selector)


def assert_busy(page, selector, what):
    """The button is off, turns (class busy) and is aria-busy, as setBusy
    makes a button whose request runs."""
    state = busy_state(page, selector)
    assert state == [True, True, "true"], f"{what} is off with its spinner while the request runs: {state}"


def css_hidden(page, selector):
    """Whether the page's style hides an element (display none), shown or
    not where it sits: an item of a closed menu is not."""
    return page.locator(selector).evaluate("(e) => getComputedStyle(e).display === 'none'")


CUBE_ITEM = "#cube-menu .menu-items button[data-value='%s']"


def cube_item(page, value):
    """An entry of the Cube choice by its value: a Cube's serial,
    found:<serial>, or __discover; in the page whether open or not."""
    return page.locator(CUBE_ITEM % value)


def choose_cube(page, value):
    """The Cube choice opened and an entry chosen, as a user does."""
    menu = page.locator("#cube-menu")
    if menu.get_attribute("open") is None:
        menu.locator("summary").click()
    cube_item(page, value).click()


def open_main_menu(page):
    """☰ in the header opened, as a user does."""
    menu = page.locator("#nav-menu")
    if menu.get_attribute("open") is None:
        menu.locator("summary").click()
    menu.locator(".menu-items").wait_for(state="visible")
    return menu


def open_personal_page(page):
    """The personal page, opened from ☰ → Personal settings."""
    open_main_menu(page)
    page.locator("#menu-me").click()
    page.locator("#me").wait_for(state="visible")


def sign_out(page):
    """Sign out (or Forget key) at the top of the personal page."""
    open_personal_page(page)
    page.locator("#logout-btn").click()


def assert_login_card_centred(page, what):
    """The sign-in card, open, in the middle of the window, as much room
    above it as below and left as right, and its title's text in the
    middle of the card (AP MobileApp, Block 14: F27), each within 2 px."""
    m = page.evaluate("""() => {
      const card = document.getElementById('login-dialog').getBoundingClientRect();
      const range = document.createRange();
      range.selectNodeContents(document.getElementById('login-title'));
      const title = range.getBoundingClientRect();
      return {top: card.top, bottom: innerHeight - card.bottom, left: card.left, right: document.documentElement.clientWidth - card.right,
              card: (card.left + card.right) / 2, title: (title.left + title.right) / 2, fits: card.height < innerHeight};
    }""")
    assert m["fits"], f"the card fits the window's height on {what}: {m}"
    assert abs(m["top"] - m["bottom"]) <= 2, f"the card in the middle of the window's height on {what}: {m}"
    assert abs(m["left"] - m["right"]) <= 2, f"the card in the middle of the window's width on {what}: {m}"
    assert abs(m["card"] - m["title"]) <= 2, f"the title in the middle of the card on {what}: {m}"


def assert_login_row(page, what):
    """The sign-in card's row on one line: each button's text on one line
    (its lines counted by a Range over its text, since a button of a fixed
    height does not grow with a line that wraps), the ways shown, then the
    card's ☰, all at one height, the ☰ at the card's right edge inside its
    padding and nothing past it (AP MobileApp, Block 14: F27)."""
    m = page.evaluate("""() => {
      const card = document.getElementById('login-dialog');
      const cs = getComputedStyle(card);
      const edge = card.getBoundingClientRect().right - parseFloat(cs.paddingRight) - parseFloat(cs.borderRightWidth);
      const items = [...document.querySelectorAll('#login-ways > .login-way:not(.hidden)'), document.querySelector('#login-menu > summary')];
      const row = document.getElementById('login-row');
      const lines = (e) => {
        const tops = new Set();
        for (const n of e.childNodes) {
          if (n.nodeType !== Node.TEXT_NODE || !n.textContent.trim()) continue;
          const range = document.createRange();
          range.selectNodeContents(n);
          for (const r of range.getClientRects()) tops.add(Math.round(r.top));
        }
        return tops.size;
      };
      return {edge, last: row.lastElementChild.id, items: items.map((e) => {
        const b = e.getBoundingClientRect();
        return {name: e.textContent.trim() || e.getAttribute('aria-label'), top: b.top, right: b.right, lines: lines(e)};
      })};
    }""")
    items = m["items"]
    assert all(i["lines"] <= 1 for i in items), f"each button's text on one line on {what}: {items}"
    assert m["last"] == "login-menu", f"the card's ☰ ends the row on {what}: {m}"
    assert max(i["top"] for i in items) - min(i["top"] for i in items) <= 1, f"the row on one line on {what}: {items}"
    assert abs(items[-1]["right"] - m["edge"]) <= 1, f"the ☰ at the card's right edge on {what}: {m}"
    assert all(i["right"] <= m["edge"] + 1 for i in items), f"nothing past the card's edge on {what}: {m}"


# /me of a viewer signed in, for a test that stubs it: no capability of
# System settings.
VIEWER = {"mode": "users", "user": {"name": "viewer1", "role": "viewer", "has_password": True},
          "role": {"key": "viewer"}, "capabilities": ["read", "preferences"], "token": {"kind": "session"}}


# The pages of the main area: how a user opens each, and the section it
# shows.
VIEW_BUTTON = {"tiles": "#view-tiles"}
VIEW_MENU = {"log": "#menu-log", "cube": "#menu-cube", "system": "#menu-system", "maint": "#menu-maint", "info": "#menu-info",
             "me": "#menu-me"}  # in ☰
VIEW_CARD = ("profile", "prefs", "notify")  # the personal page's cards, from its sections menu
VIEW_SECTION = {"tiles": "#rooms", "log": "#logview", "me": "#me", "profile": "#profile", "prefs": "#prefs", "notify": "#notify",
                "cube": "#cube-page", "system": "#system", "maint": "#maint-page", "info": "#info-page"}


def show_view(page, view):
    """A page of the main area, opened as a user opens it: Rooms with its
    button in the header, every other page from ☰; My profile, My
    settings and Notifications from the personal page's sections menu."""
    if view in VIEW_CARD:
        open_personal_page(page)
        menu = page.locator("#me .sec-menu")
        if menu.get_attribute("open") is None:
            menu.locator("summary").click()
        menu.locator(f".toc-link[data-target='{view}']").click()
        page.locator(VIEW_SECTION[view]).wait_for(state="visible")
        assert scroll_settled(page), f"the smooth scroll to {view} comes to rest"  # the menu brings the card in with one
    elif view in VIEW_MENU:
        open_main_menu(page)
        page.locator(VIEW_MENU[view]).click()
    else:
        page.locator(VIEW_BUTTON[view]).click()
    page.locator(VIEW_SECTION[view]).wait_for(state="visible")

# --- the same element ----------------------------------------------------

def mark(page, selectors):
    """Marks the element of each selector, so that a later look tells
    whether it is still the same element (replaced)."""
    page.evaluate("(ss) => ss.forEach((s) => { document.querySelector(s).__mark = s; })", selectors)


def replaced(page, selectors):
    """The selectors whose element is not the one mark marked, or is gone."""
    return page.evaluate("(ss) => ss.filter((s) => { const e = document.querySelector(s); return !e || e.__mark !== s; })", selectors)


def frames(page):
    """Two frames and a task: a part drawn again after a change has been drawn."""
    page.evaluate("() => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(done, 0))))")


# --- a page of its own ------------------------------------------------------

def owner_page(new_context, guard, watch, url, token, clock=False):
    """A page of its own with the owner's key token at url, the tiles
    shown and the stream open, as the fixture page has it: for a test
    that signs out or meets a 401 (test_pairing_updates.py,
    test_repair_updates.py); with clock, under Playwright's clock, its
    time flowing as the real one (page.clock.run_for moves it on), as the
    mark clock gives the fixture page."""
    ctx = new_context()
    if token:
        ctx.add_init_script(f"localStorage.setItem('massimo.token', {token!r})")
    if clock:
        ctx.clock.install()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(url)
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the stream is open"
    return page


# --- the repairs' routes --------------------------------------------------

# The radio budget a repair's preview and run name, as the Cube reports it.
REPAIR_BUDGET = {"duty_cycle_pct": 3, "free_memory_slots": 50, "updated_at": "2026-11-15T10:00:00Z"}


def repair_nothing(repair, why):
    """The preview of a repair with nothing to send, and why."""
    return {"action": repair, "plan": "", "steps": [], "radiograms": 0, "duty_cost_pct": 0, "slots": 0, "held_slots": 0, "resets": 0,
            "wakeups": 0, "metadata": False, "notes": [], "budget": REPAIR_BUDGET, "nothing": why}


class RepairStub:
    """The repairs' routes (safe_route): a GET answers previews[repair],
    the repair the path's last part, a preview, the text of one with
    nothing to do, or a function of the query (parse_qs) that returns
    either; with previews["list-name"] a listing without room_name asks
    for room 7's name. With runs a POST is kept in runs with its path and
    query and answers a run whose steps the Cube accepted; without, it
    goes on to the test's other routes (the guard's, which refuse it)."""

    PATTERN = "**/repair/**"

    def __init__(self, page, previews, runs=True):
        self.page, self.previews, self.answer_runs, self.runs = page, previews, runs, []
        self.routed = safe_route(page, self.PATTERN, self.handle)

    def handle(self, route):
        url = urlparse(route.request.url)
        repair = url.path.rsplit("/", 1)[1]
        query = parse_qs(url.query)
        if route.request.method != "GET":
            if not self.answer_runs:
                route.fallback()
                return
            self.runs.append((url.path, query))
            body = {"ok": True, "action": repair, "steps": [{"what": "x", "lines": 3, "accepted": 3}], "budget": REPAIR_BUDGET}
            if repair in ("relink", "unlink-relink"):
                body["confirmation"] = "pending"
            if repair == "reset":
                body["still_flagged"] = []
            route.fulfill(status=200, content_type="application/json", body=json.dumps(body))
            return
        answer = self.previews.get(repair, "nothing to repair")
        if callable(answer):
            answer = answer(query)
        if repair == "list" and "room_name" not in query and self.previews.get("list-name"):
            answer = dict(repair_nothing(repair, "room 7 is not in the list"), details={"code": "room_name_needed", "room": 7})
        elif not isinstance(answer, dict):
            answer = repair_nothing(repair, answer)
        route.fulfill(status=200, content_type="application/json", body=json.dumps(answer))

    def close(self):
        """The repairs' routes answered by the server again."""
        page = self.page
        if not page.is_closed():
            page.unroute(self.PATTERN, self.routed)


# --- a state event per device ---------------------------------------------

# A state event of the stream for every device of the chosen Cube, as the
# server sends it, the devices without a room's too; without a stream,
# the page's updateDevice. A function for page.evaluate; the guard's
# round calls it too (test_parts_guard.py, GUARD_JS).
STATE_EVENTS_JS = """() => {
    const devices = [];
    for (const r of state.rooms) {
      devices.push(...r.devices);
      if (r.vwt) devices.push(r.vwt);
    }
    devices.push(...state.unlisted);
    for (const d of devices) {
      const copy = JSON.parse(JSON.stringify(d));
      if (state.events) {
        state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({
          kind: "state", device: copy.rf_address, state: copy, time: new Date().toISOString() }) }));
      } else {
        updateDevice(copy);
      }
    }
  }"""


def state_events(page):
    """A state event per device, as STATE_EVENTS_JS sends them."""
    page.evaluate(STATE_EVENTS_JS)
