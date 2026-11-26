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
and what a test put in, a device's or a room's in the rooms' read among
it (Reads.device, Reads.room), kept through the server's own events of
them; Stream watches the page's event stream from a mark; isolate_log
keeps a log of the test's own lines; wait_log_live waits until the log
takes the stream's state events; safe_route is the one route handler
that lets go a request the page no longer waits for; undo is a cleanup
that says what it could not undo; held_poll holds the page's poll while
a test measures what the page does by itself; pause_clock pauses
Playwright's clock at the page's now. open_fold, close_fold and
toggle_fold take a fold's toggle event before a test acts inside it.
wait_started waits for the page's start, the page fixture's and
owner_page's, and says on a timeout which read it waited for; answered
waits until the page has read all it reads now, a read it starts only
after another has answered among them.

Every wait of this kit, and the page fixture's, lasts its seconds times
UI_TEST_SCALE (scaled), a positive number, 1 where it is not set: a
slow or loaded runner sets more, and run.sh's retry, which runs with the
trace, raises it. A window that waits for nothing to come is not
scaled: ApiReads.answered's quiet time, scroll_settled's still frames.
"""
import contextlib
import json
import math
import os
import re
import sys
import time
import urllib.error
import weakref
from datetime import datetime, timedelta, timezone
from urllib.parse import parse_qs, urlparse

from playwright.sync_api import Error
from playwright.sync_api import TimeoutError as PlaywrightTimeout

# The password of the operator that run.sh adds with massimo user add, in
# every set: of 8 characters or more, as a new password is
# (internal/password).
OPERATOR_PASSWORD = "op-password"

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


def parse_scale(v):
    """UI_TEST_SCALE's value v as a number: 1 where it is empty, else a
    positive number, as MASSIMO_TEST_SCALE is for the Go tests
    (internal/testkit); anything else is refused, so that a typo does not
    run every wait at a length nobody meant."""
    if v == "":
        return 1.0
    try:
        f = float(v)
    except ValueError:
        f = math.nan
    if not f > 0 or math.isinf(f):
        raise ValueError(f"UI_TEST_SCALE is a positive number, not {v!r}")
    return f


SCALE = parse_scale(os.environ.get("UI_TEST_SCALE", ""))


def scaled(seconds):
    """seconds times UI_TEST_SCALE: the time a wait gives what comes
    within seconds on an unhurried machine. Not for a window that waits
    for nothing to come, which a scale would only make longer."""
    return seconds * SCALE


def wait_until(page, expr, seconds):
    """Polls a page expression from here, for seconds (scaled): the
    page's CSP rules out wait_for_function with a string."""
    end = time.time() + scaled(seconds)
    while time.time() < end:
        if page.evaluate(expr):
            return True
        page.wait_for_timeout(200)
    return False


def settled(page, look, ok, seconds=5):
    """Looks (look(), from here) until ok holds of what it saw, for
    seconds (scaled) at most, and returns whether it held and the last
    look: for a check of what a test told the page's reads
    (Reads.device), which a server's own event of the device draws over
    until keep's read brings it back, and which takes several of
    Playwright's calls to look at; a Playwright error of a look, an
    element drawn anew meanwhile, is a look that does not hold."""
    end = time.time() + scaled(seconds)
    while True:
        try:
            got = look()
        except Error as e:
            got = e
        if not isinstance(got, Error) and ok(got):
            return True, got
        if time.time() >= end:
            return False, got
        page.wait_for_timeout(100)


def pause_clock(page, ahead_ms=50):
    """Playwright's clock paused at the page's now and ahead_ms on. At the
    pause Playwright runs once every timer due up to the pause's time, a
    pending animation frame and a timer due within ahead_ms among them;
    after it, no timer and no frame of the page's comes by itself: what a
    test gets is that nothing due later than now plus ahead_ms has run. The
    test runs them with page.clock.run_for, and drives the page meanwhile by
    evaluate and by
    Python's own waits (wait_until), none of which waits on a timer or a
    frame of the page's. Python's pause_at takes seconds, the page's
    Date.now() milliseconds: given those, the clock jumped millennia ahead
    and fired every timer of the page's at once, its poll, its stream's
    renewal and a held read's time limit among them, which aborts it
    (apiRequest, AP MobileApp, D82-A). It pauses ahead_ms on, so that the
    time asked for is not past once the call arrives; where it is past all
    the same, as on a run with the trace, whose every call takes longer,
    Playwright refuses it and it asks again from the page's new now, twice
    as far on, up to four times (100 to 800 ms, the timers due within them
    run at that pause). It is the one way the tests pause the
    clock: conftest.py fails the collection of a test module that calls
    pause_at itself."""
    for attempt in range(5):
        try:
            page.clock.pause_at((page.evaluate("Date.now()") + ahead_ms * 2 ** attempt) / 1000)
            return
        except Error as e:
            if "Cannot fast-forward to the past" not in str(e) or attempt == 4:
                raise


def scroll_settled(page, seconds=3):
    """Waits until the page has not scrolled for ten animation frames;
    false if it is still scrolling after so many seconds (scaled; the
    ten frames are not). Chromium runs a smooth scroll on the compositor,
    and an instant scroll sent while it ends can be undone by it: the
    page back where the smooth scroll was going. Frames, not polls from
    here: two equal readings 200 ms apart can be one pause of a starved
    renderer."""
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
    })""", scaled(seconds) * 1000)


def scrolled_to(page, y):
    """Scrolls the window to y at once and waits until it rests there
    (scroll_settled); the scroll it rests at."""
    page.evaluate("(y) => window.scrollTo({top: y, behavior: 'instant'})", y)
    assert scroll_settled(page), f"the page rests at {y}"
    return page.evaluate("window.scrollY")


# The ApiReads of each page, the first made for it (api_reads).
_API_READS = weakref.WeakKeyDictionary()


class ApiReads:
    """The page's requests to the server that have no answer yet, its API's
    and /healthz (the header's version check, a restart's following),
    counted from the moment it is made: a page shown sends its cards' reads
    at once, and their answers come in any order, so a card that grows with
    a later one (System settings → Integrations, with why the server does
    not look on the LAN, api/v1/integrations/found) changes the page's
    height after the answer a test waited for. The event stream, which
    stays open, is left out. The first made for a page is its own
    (api_reads): the page fixture's and owner_page's, made before the
    page's start. It holds its page weakly, so that the page's entry goes
    with the page."""

    def __init__(self, page):
        self._page = weakref.ref(page)
        self.open = set()
        self.changes = 0  # the reads started and ended so far, for answered's quiet time
        page.on("request", self._sent)
        page.on("requestfinished", self._done)
        page.on("requestfailed", self._done)
        _API_READS.setdefault(page, self)

    @property
    def page(self):
        """The page, None once it is gone."""
        return self._page()

    @staticmethod
    def _api(request):
        path = urlparse(request.url).path
        return ("/api/v1/" in path and not re.search(r"/events$", path)) or path.endswith("/healthz")  # not the stream (app.js connectEvents)

    def _sent(self, request):
        if self._api(request):
            self.open.add(request)
            self.changes += 1

    def _done(self, request):
        if request in self.open:
            self.open.discard(request)
            self.changes += 1

    def pending(self):
        """The reads without an answer, each as its method and URL, sorted."""
        return sorted(f"{r.method} {r.url}" for r in self.open)

    def _waiting(self, held):
        """The reads without an answer but those of held, routes (or
        requests) the test holds."""
        kept = {getattr(h, "request", h) for h in held}
        return [r for r in self.open if r not in kept]

    def answered(self, seconds=5, quiet_ms=200, held=()):
        """Waits until every read has its answer, but those of held, the
        routes (or requests) the test holds, a list it adds to meanwhile
        among them; and for quiet_ms no read has started or ended and
        nothing more is drawn (the page's height unchanged). A read the page
        starts only after another has answered comes within quiet_ms of
        that answer and is waited for in turn: a burst's next read
        (shared/burst.ts), the next of the page's start (app.js init), a
        load an answer sets off. quiet_ms is not scaled: it waits for
        nothing to come. Measured from here, by Playwright's own timer, so
        that a page under a paused clock is waited for as well. False after
        so many seconds (scaled)."""
        page = self.page
        end = time.monotonic() + scaled(seconds)
        while time.monotonic() < end:
            if self._waiting(held):
                page.wait_for_timeout(25)
                continue
            changes = self.changes
            height = page.evaluate("document.documentElement.scrollHeight")
            page.wait_for_timeout(quiet_ms)
            if self.changes == changes and page.evaluate("document.documentElement.scrollHeight") == height and not self._waiting(held):
                return True
        return False


def api_reads(page):
    """The page's ApiReads (the first made for it): the page fixture's and
    owner_page's, made before the page's start; one made now for a page
    of a test's own, which sees no read the page sent before."""
    return _API_READS.get(page) or ApiReads(page)


def answered(page, held=(), seconds=5):
    """Waits until the page's reads have all answered and none starts
    within the quiet time (ApiReads.answered of api_reads), held aside,
    the routes the test holds; fails, naming the reads still open, after
    so many seconds (scaled). The one wait for "the page has read all it
    reads now", before a count of what an action read or a check of what
    the answers drew: it follows a read that starts only after another
    has answered, where a request of the test's own that overtakes the
    page's does not (docs/ui-testing.md, section 9)."""
    reads = api_reads(page)
    assert reads.answered(seconds, held=held), \
        f"the page's reads answered within {scaled(seconds):g} s: still open {', '.join(reads.pending()) or 'none'}"


def load_cubes(page):
    """A load of the Cubes' list, as the poll makes it (loadCubes()), and
    the reads it sets off answered (answered)."""
    page.evaluate("loadCubes()")
    answered(page)


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


def wait_held(page, held, seconds=5, n=1):
    """Waits until n held requests, one by default, have reached their
    route, for seconds (scaled)."""
    end = time.time() + scaled(seconds)
    while len(held) < n and time.time() < end:
        page.wait_for_timeout(100)
    return len(held) >= n


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
# times the page handed it to MassimoParts.log.event (Stream reads them); and,
# once the page's scripts are in,
# its MassimoParts.log.event, which isolate_log can have keep the server's lines
# out and wait_log_live probes. The page itself is not changed: app.js
# calls the log's event through window.MassimoParts at each event. Every
# stream also reads the rooms again after the server's state and config
# events of a device in window.__uiTest.keep (Reads.keep), once the page's
# own listeners have drawn the event; kept counts those reads.
PAGE_KIT = """(() => {
  const k = window.__uiTest = { events: [], streams: 0, fromServer: false, isolated: false, probes: 0, probed: 0, appends: 0,
    keep: new Set(), kept: 0 };
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
  const readKept = (ev) => {
    if (!ev.isTrusted || !k.keep.size || typeof loadRooms !== "function") return;
    let device;
    try {
      device = JSON.parse(ev.data).device;
    } catch {
      return;
    }
    if (!k.keep.has(device)) return;
    k.kept++;
    setTimeout(() => loadRooms().catch(() => {}), 0);
  };
  class Watched extends Real {
    constructor(url, opts) {
      super(url, opts);
      this.__uiStream = ++k.streams;
      for (const kind of ["open", "error"]) add.call(this, kind, (ev) => record(this, ev, 0));
      for (const kind of ["state", "config"]) add.call(this, kind, readKept);
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
    const log = window.MassimoParts && window.MassimoParts.log;
    if (!log || typeof log.event !== "function") return;
    const append = log.event;
    log.event = function (ev) {
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
# The page's read of the chosen Cube's rooms (fetchRooms of app.js), the
# pattern Reads.device and Reads.room tell it by.
ROOMS = "**/api/v1/cubes/*/rooms?devices=all"
# A read of the chosen Cube's rooms, as fetchRooms of app.js sends it: a
# pattern of Reads.clear that matches it forgets the devices kept and
# the devices and rooms told.
ROOMS_READ = "http://127.0.0.1/api/v1/cubes/KEQ0000100/rooms?devices=all"


def route_matches(pattern, url):
    """Whether a route's pattern matches url: a compiled regex where it
    finds it, a glob where it matches the whole of it, * within a path's
    segment and ** across them."""
    if isinstance(pattern, re.Pattern):
        return pattern.search(url) is not None
    parts = re.split(r"(\*\*|\*)", pattern)
    return re.fullmatch("".join(".*" if p == "**" else "[^/]*" if p == "*" else re.escape(p) for p in parts), url) is not None


def configured_only(cubes):
    """GET /api/v1/cubes without the Cubes found on the network."""
    return [c for c in cubes if c.get("configured") is not False]


def found_only(cubes):
    """GET /api/v1/cubes with the Cubes found on the network alone: the
    list gives no Cube the server serves."""
    return [c for c in cubes if c.get("configured") is False]


def _told_merged(told, fields):
    """fields told after those in told: each in place of its value told
    before, a state's or sources' fields beside those told before."""
    for k, v in fields.items():
        v = json.loads(json.dumps(v))  # a copy: the test may change its own afterwards
        if k in ("state", "sources") and isinstance(v, dict) and isinstance(told.get(k), dict):
            told[k] = dict(told[k], **v)
        else:
            told[k] = v


def _told_into(entry, fields):
    """The fields told put into an entry of the rooms' read: a state's
    fields into its state, any other in place of the entry's own; None
    takes a field out (state None the state)."""
    for k, v in fields.items():
        if k == "state" and isinstance(v, dict):
            st = dict(entry.get("state") or {})
            for sk, sv in v.items():
                if sv is None:
                    st.pop(sk, None)
                else:
                    st[sk] = json.loads(json.dumps(sv))
            entry["state"] = st
        elif v is None:
            entry.pop(k, None)
        else:
            entry[k] = json.loads(json.dumps(v))


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
        self.kept = set()  # the devices kept (keep), by radio address
        self.devices = {}  # radio address: {"like": a copy's source, "fields": told} (device)
        self.rooms = {}  # room id: the fields told for its entry (room)

    @property
    def page(self):
        """The page, None once it is gone."""
        return self._page()

    def change(self, pattern, fn):
        """Every GET of pattern from now on answered with fn(the answer
        so far), the server's JSON with the changes told before; fn may
        change it in place and returns it. The same function told again
        for the pattern is applied once (a new lambda is another). A read
        that the patterns of several changes match takes each of them, by
        the order their patterns were first told: of two routes that match
        a request, Playwright runs the later one alone, which answers it."""
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
        url = route.request.url
        for p, fns in list(self.changes.items()):
            if p != pattern and not route_matches(p, url):
                continue
            for fn in list(fns):
                body = fn(body)
        # the server's own headers, its Date among them, which the page
        # takes the server's clock from; the length is the new body's
        headers = {k: v for k, v in answer.headers.items() if k.lower() != "content-length"}
        route.fulfill(status=answer.status, headers=headers, body=json.dumps(body))

    def clear(self, pattern=CUBES):
        """The reads of pattern answered by the server alone again, what
        was told for them forgotten; a pattern that the rooms' read
        matches forgets the devices kept (keep) and the devices and rooms
        told (device, room) as well."""
        if pattern == CUBES:
            self.cubes.clear()
            self.found_cubes.clear()
        self.changes.pop(pattern, None)
        routed = self.routed.pop(pattern, None)
        page = self.page
        if routed is not None and page is not None and not page.is_closed():
            page.unroute(pattern, routed)
        if route_matches(pattern, ROOMS_READ):
            self.devices.clear()
            self.rooms.clear()
        if self.kept and route_matches(pattern, ROOMS_READ):
            self.kept.clear()
            if page is not None and not page.is_closed():
                page.evaluate("() => window.__uiTest.keep.clear()")

    def keep(self, rf):
        """The device rf kept as the test told it to the rooms' read: from
        now on, after each state or config event of rf that the server
        sends, which brings the device as the server has it, the page
        reads its rooms again, so that what was told is drawn again. The
        server sends them by itself, at its poll after an earlier write
        and at a follow-up's re-read of a C line, and a test cannot time
        them. Every stream of the page does it, a reopened one too
        (PAGE_KIT), once the page's own listeners have drawn the event;
        an event a test dispatches starts no read. It holds within one
        document, as a Stream's mark: a reload forgets it, and the test
        keeps again after it; clear of the rooms' read forgets it."""
        self.kept.add(rf)
        self.page.evaluate("(rf) => { window.__uiTest.keep.add(rf); }", rf)

    def device(self, rf, like=None, **fields):
        """The device rf's entry in the rooms' read (ROOMS) with fields in
        it from now on, beside those told for it before, each in place of
        its value told before: state's fields into the device's state, any
        other in place of the entry's own, a None taking a field out (state
        None the state, until None a holiday's end); the device kept
        (keep), so that what is told comes back after the server's own
        state and config events of it, which bring it as the server has
        it, and after any read of the page's own. With like, a device the
        server does not list: a copy of the server's entry of like, at the
        end of the devices of its room, or of the room room_id among fields
        names. The page shows it at its next read of the rooms: the test
        reads them (loadRooms()) or puts the same into the page itself (the
        state event under test, say). A check after it waits (wait_until),
        since keep's read brings it back a moment after such an event has
        drawn the server's."""
        told = self.devices.setdefault(rf, {"like": None, "fields": {}})
        if like is not None:
            told["like"] = like
        _told_merged(told["fields"], fields)
        self.change(ROOMS, self._rooms_told)
        self.keep(rf)

    def room(self, rid, **fields):
        """The room rid's entry in the rooms' read (ROOMS) with fields in it
        from now on, beside those told before, each in place of its value
        told before, a None taking a field out: its presets (eco_temp,
        comfort_temp), its thermostat, and sources, those of its virtual
        wall thermostat (setpoint: the thermostats whose setpoint it shows,
        None for none), beside the sources told before. The room's
        thermostats and its virtual wall thermostat, as the page has them,
        kept (keep), so that what is told of them stays drawn after the
        server's own events of them: without fields, room keeps them
        alone. The page shows it at its next read, as device says."""
        if fields:
            _told_merged(self.rooms.setdefault(rid, {}), fields)
            self.change(ROOMS, self._rooms_told)
        for rf in self.page.evaluate("""(rid) => { const r = state.rooms.find((x) => x.id === rid);
          return !r ? [] : r.devices.filter(isThermostat).map((d) => d.rf_address).concat(r.vwt ? [r.vwt.rf_address] : []); }""", rid):
            self.keep(rf)

    def _rooms_told(self, rooms):
        rooms = rooms if isinstance(rooms, list) else []
        listed = {d.get("rf_address"): d for r in rooms for d in r.get("devices") or []}
        # the copies first, each of the server's entry of its source
        for rf, told in self.devices.items():
            src = listed.get(told["like"])
            if rf in listed or src is None:
                continue
            copy = json.loads(json.dumps(src))
            copy["rf_address"] = rf
            rid = told["fields"].get("room_id", src.get("room_id"))
            room = next((r for r in rooms if r.get("id") == rid), None)
            if room is not None:
                room["devices"] = (room.get("devices") or []) + [copy]
        for r in rooms:
            for d in r.get("devices") or []:
                told = self.devices.get(d.get("rf_address"))
                if told:
                    _told_into(d, told["fields"])
            fields = dict(self.rooms.get(r.get("id")) or {})
            sources = fields.pop("sources", None)
            _told_into(r, fields)
            if sources:
                for v in r.get("devices") or []:
                    if v.get("virtual"):
                        v["sources"] = dict(v.get("sources") or {})
                        _told_into(v["sources"], sources)
        return rooms

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
        the page handed it to MassimoParts.log.event, those isolate_log keeps out
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
        server says which (true for the server's), what naming it, within
        seconds (scaled); on a timeout an AssertionError with the events
        that came since the mark."""
        end = time.time() + scaled(seconds)
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
        raise AssertionError(f"{what}: no {kind} event within {scaled(seconds):g} s; {len(seen)} since the mark{': ' if seen else ''}{shown}")


def describe_event(e):
    """An event of Stream in a few words: its kind, its device where it
    has one, and who sent it."""
    data = e.get("data")
    device = data.get("device") if isinstance(data, dict) else None
    return " ".join(x for x in (e["kind"], device, "(server)" if e["server"] else "(test)") if x)


# The probe of wait_log_live: a state event without a state, which the
# page's state listener hands to MassimoParts.log.event once the stream's first
# 1.5 s are over, and PAGE_KIT's wrapper takes out before the log.
LOG_PROBE = """() => {
  const k = window.__uiTest;
  if (!k || !state.events) return false;
  const n = ++k.probes;
  state.events.dispatchEvent(new MessageEvent("state", { data: JSON.stringify({ kind: "state", __probe: n }) }));
  return k.probed === n;
}"""


def wait_log_live(page, seconds=5):
    """Waits until the log takes the stream's state events, for seconds
    (scaled, by wait_until): the page leaves a stream's first 1.5 s out
    of the log, the burst of states a connection starts with, and again
    after a resync (app.js, connectEvents). A probe, which the log never
    shows, is sent until the page's state listener hands one on to the
    log."""
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
        page.evaluate("MassimoParts.log.load()")
        assert wait_until(page, "!MassimoParts.log.state.loading && MassimoParts.log.state.lines.length === 0", 5), "the log loaded, without a line of the server's"


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


# The sign-in card's box and its title's middle, and the room the
# signed-out page gives the card: the window's height less the body's
# padding above and below it (style.css, body.signed-out), 868 px of the
# suite's 900.
LOGIN_CARD = """() => {
  const card = document.getElementById('login-dialog').getBoundingClientRect();
  const range = document.createRange();
  range.selectNodeContents(document.getElementById('login-title'));
  const title = range.getBoundingClientRect();
  const body = getComputedStyle(document.body);
  const pad = [parseFloat(body.paddingTop), parseFloat(body.paddingBottom)];
  return {top: card.top, bottom: innerHeight - card.bottom, height: card.height, left: card.left,
          right: document.documentElement.clientWidth - card.right, card: (card.left + card.right) / 2,
          title: (title.left + title.right) / 2, pad, room: innerHeight - pad[0] - pad[1], scrollY};
}"""

# Where the card is, whether it shows all it holds (a card that cuts or
# scrolls what overflows it does not), and each control of it shown: its
# name, whether it lies inside the card's box, and its top and bottom in
# the page, its scroll added.
LOGIN_CARD_LOOK = """() => {
  const card = document.getElementById('login-dialog');
  const box = card.getBoundingClientRect();
  const name = (e) => e.id || e.getAttribute('aria-label') || e.textContent.trim().slice(0, 40) || e.tagName;
  const controls = [...card.querySelectorAll('button, input, select, textarea, summary, a[href], [role=button]')]
    .filter((e) => e.checkVisibility({ visibilityProperty: true })).map((e) => {
      const r = e.getBoundingClientRect();
      return {name: name(e), top: r.top + scrollY, bottom: r.bottom + scrollY,
              inside: r.top >= box.top - 0.5 && r.bottom <= box.bottom + 0.5 && r.left >= box.left - 0.5 && r.right <= box.right + 0.5};
    });
  return {top: box.top, bottom: box.bottom, left: box.left, innerHeight, scrollY, controls,
          whole: card.scrollHeight <= card.clientHeight + 1, held: [card.scrollHeight, card.clientHeight]};
}"""


def controls_reached(look, reach, what):
    """The card of LOGIN_CARD_LOOK's look showing all it holds, and each
    of its controls inside its box and within the part of the page a
    user's scroll reaches, from its start to reach, the page's bottom edge
    at its end (the window's height where the page does not scroll): each
    in view at some scroll, however tall the card."""
    assert look["whole"], f"the card shows all it holds on {what}: its content and its box, {look['held']}"
    cut = [c["name"] for c in look["controls"] if not c["inside"]]
    assert not cut, f"each control of the card inside the card's box on {what}: {cut}"
    out = [c["name"] for c in look["controls"] if c["top"] < -0.5 or c["bottom"] > reach + 0.5]
    assert not out, f"each control of the card in view at some scroll on {what}: {out}, the page reached to {reach}"


def assert_login_card_centred(page, what):
    """The sign-in card, open, in the middle of the window: no taller than
    the room the page gives it, the window less the body's padding above
    and below (AP AccessEnds, D18), as much room above it as below and
    left as right, its title's text in the middle of the card (AP
    MobileApp, Block 14: F27), each within 2 px, and each of its controls
    shown inside it and in view (controls_reached). The sign-in card's own
    measure: one that grows taller than the room by accident fails here
    (D16)."""
    page.evaluate("scrollTo({ top: 0, behavior: 'instant' })")
    m = page.evaluate(LOGIN_CARD)
    assert m["height"] <= m["room"], f"the card fits the window's height on {what}: {m}"
    assert abs(m["top"] - m["bottom"]) <= 2, f"the card in the middle of the window's height on {what}: {m}"
    centred_across(m, what)
    look = page.evaluate(LOGIN_CARD_LOOK)
    controls_reached(look, look["innerHeight"], what)


# The sign-in card's top and height, for a test that checks the card
# stays where it was (same_box).
LOGIN_BOX = "(() => { const r = document.getElementById('login-dialog').getBoundingClientRect(); return [r.top, r.height]; })()"


def same_box(page, before, what):
    """The sign-in card where it was and as tall (LOGIN_BOX, before taken
    earlier), within half a pixel."""
    after = page.evaluate(LOGIN_BOX)
    assert all(abs(a - b) <= 0.5 for a, b in zip(after, before)), f"{what}: the card moved, {before} → {after}"


def centred_across(m, what):
    """The card of LOGIN_CARD's measure m in the middle of the window's
    width, and its title's text in the middle of the card, each within
    2 px."""
    assert abs(m["left"] - m["right"]) <= 2, f"the card in the middle of the window's width on {what}: {m}"
    assert abs(m["card"] - m["title"]) <= 2, f"the title in the middle of the card on {what}: {m}"


def assert_login_card_placed(page, what):
    """The sign-in card, open, where the signed-out page places it
    (style.css; AP PasswordRules, D57-A): a card within the room, the
    window less the body's padding above and below, in the middle of the
    window as assert_login_card_centred has it; a taller one, from the page
    at its start, its top at the body's padding, and at the page's end, as
    the wheel scrolls it, its bottom inside the window (AP AccessEnds,
    D17); either way each control of the card shown inside the card and in
    view at some scroll (controls_reached), the card in the middle of the
    window's width, its title in the middle of the card, and no horizontal
    scroll. The page is left at its start. The first owner's card's
    measure, which a press of the dice makes taller than the room the
    window leaves it in the app at 390 px (D16)."""
    page.evaluate("scrollTo({ top: 0, behavior: 'instant' })")
    m = page.evaluate(LOGIN_CARD)
    start = page.evaluate(LOGIN_CARD_LOOK)
    if m["height"] <= m["room"]:
        assert abs(m["top"] - m["bottom"]) <= 2, f"the card in the middle of the window's height on {what}: {m}"
        reach = start["innerHeight"]
    else:
        assert abs(start["top"] - m["pad"][0]) <= 1, f"the card taller than its room starts at its top on {what}: {m}"
        # to the page's end as a user scrolls, by the wheel over the card's own padding, which a page that cannot be
        # scrolled does not move, where a script's scroll would; the scroll runs after the wheel's event, until the end
        page.mouse.move(start["left"] + 8, start["innerHeight"] / 2)
        page.mouse.wheel(0, page.evaluate("document.documentElement.scrollHeight"))
        wait_until(page, "scrollY >= document.documentElement.scrollHeight - innerHeight - 1", 2)
        assert scroll_settled(page), f"the page comes to rest at its end on {what}"
        end = page.evaluate(LOGIN_CARD_LOOK)
        page.evaluate("scrollTo({ top: 0, behavior: 'instant' })")
        assert end["bottom"] <= end["innerHeight"], f"the page scrolls to the card's bottom on {what}: {m}, {end}"
        reach = end["scrollY"] + end["innerHeight"]
    controls_reached(start, reach, what)
    centred_across(m, what)
    assert page.evaluate("document.documentElement.scrollWidth <= innerWidth"), f"no horizontal scroll on {what}: {scroll_width(page)}"


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


def stub_me(page, capabilities):
    """The page signed in with these capabilities: GET /api/v1/me answered
    as VIEWER's with them (safe_route), and read (loadMe()); the owner's
    token still answers the other reads. The route as routed; a test
    takes it away with page.unroute("**/api/v1/me")."""
    me = dict(VIEWER, capabilities=capabilities)
    routed = safe_route(page, "**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(me)))
    page.evaluate("loadMe()")
    return routed


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


# The fake Cube's panel in System settings → Cubes.
CUBE_PANEL = "#cubes-list .cube-panel[data-serial='KEQ0000100']"


def open_cube_panel(page):
    """System settings → Cubes with the fake Cube's panel open, opened by
    its name in the summary where it is closed (a click elsewhere on the
    summary may meet one of its controls); the panel and its form, shown."""
    show_view(page, "system")
    panel = page.locator(CUBE_PANEL)
    if panel.get_attribute("open") is None:
        panel.locator(":scope > summary .cube-head-who").click()
    form = panel.locator("form.cube-form")
    form.wait_for(state="visible")
    return panel, form

# --- the same element ----------------------------------------------------

def mark(page, selectors):
    """Marks the element of each selector, so that a later look tells
    whether it is still the same element (replaced)."""
    page.evaluate("(ss) => ss.forEach((s) => { document.querySelector(s).__mark = s; })", selectors)


def replaced(page, selectors):
    """The selectors whose element is not the one mark marked, or is gone."""
    return page.evaluate("(ss) => ss.filter((s) => { const e = document.querySelector(s); return !e || e.__mark !== s; })", selectors)


def same_and_open(page, selector):
    """Whether the element of selector, a details, is the one mark marked
    and is open: a list that a load left in place and open."""
    return page.evaluate("(s) => { const d = document.querySelector(s); return !!d && d.__mark === s && d.open; }", selector)


def focus_in_turn(page, click, focused):
    """Clicks the element of click in the page and reports whether the
    element of focused has the focus once the drawing that follows is
    done, before a frame: a part gives the focus in a layout effect, in
    the drawing's own turn, where an effect gives it a frame later, when a
    user (or a test) may be typing elsewhere (common.md, "The focus is
    given in a layout effect")."""
    return page.evaluate("""async ([click, focused]) => {
      document.querySelector(click).click();
      for (let i = 0; i < 10; i++) await Promise.resolve();
      return document.activeElement === document.querySelector(focused);
    }""", [click, focused])


def frames(page):
    """Two frames and a task: a part drawn again after a change has been drawn."""
    page.evaluate("() => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(done, 0))))")


# --- a fold ---------------------------------------------------------------

# A fold's summary clicked, and the fold's toggle event taken, in ms
# milliseconds at most: the event, which tells the page's part the fold's
# new state (Fold's onToggle), comes in a task of its own after the
# click, and a part that starts anew at it (#me-password and .user-panel
# start their password fields anew) drops what the test did inside the
# fold before it. A click that toggles nothing fails once ms have passed,
# not at the test's end. Resolves to whether the fold is open.
FOLD_TOGGLE = """([sel, ms]) => new Promise((done, fail) => {
  const d = document.querySelector(sel);
  if (!d) {
    fail(new Error("no fold " + sel));
    return;
  }
  const took = () => {
    clearTimeout(timer);
    setTimeout(() => done(d.open), 0);
  };
  const timer = setTimeout(() => {
    d.removeEventListener("toggle", took);
    fail(new Error("no toggle event of " + sel + " within " + ms + " ms"));
  }, ms);
  d.addEventListener("toggle", took, { once: true });
  d.querySelector(":scope > summary").click();
})"""


def toggle_fold(page, sel, seconds=3):
    """The fold sel, a <details>, opened or closed by its summary, its
    toggle event taken within seconds (scaled) and the part drawn again
    (frames); returns whether it is open."""
    is_open = page.evaluate(FOLD_TOGGLE, [sel, int(scaled(seconds) * 1000)])
    frames(page)
    return is_open


def open_fold(page, sel, seconds=3):
    """The fold sel open, its toggle event taken (toggle_fold) where it
    was closed."""
    if not page.evaluate("(sel) => document.querySelector(sel).open", sel):
        assert toggle_fold(page, sel, seconds), f"the fold {sel} opens"


def close_fold(page, sel, seconds=3):
    """The fold sel closed, its toggle event taken (toggle_fold) where it
    was open."""
    if page.evaluate("(sel) => document.querySelector(sel).open", sel):
        assert not toggle_fold(page, sel, seconds), f"the fold {sel} closes"


# --- the page's start -----------------------------------------------------

# How long the page's start may take, before the scale: longer than the
# page's own limit of a read, API_WAIT_MS of app.js (20 s), so that a
# read the server answers late is the page's to give up, which it says
# on its message line, and not a wait of the test's that ends first.
START_SECONDS = 30


def wait_started(page, api, seconds=START_SECONDS):
    """Waits until the page's start is over, its room tiles drawn, for
    seconds (scaled): the page fixture's wait after its goto, and
    owner_page's. api is an ApiReads made before the goto; on a timeout
    an AssertionError names the API reads still open and the page's
    message line, which say which read the start waited for."""
    try:
        page.wait_for_selector(".room", timeout=scaled(seconds) * 1000)
    except PlaywrightTimeout:
        line = page.evaluate("(() => { const m = document.getElementById('message'); "
                             "return m && !m.classList.contains('hidden') ? m.textContent : ''; })()")
        raise AssertionError(f"the page's start: no room tile within {scaled(seconds):g} s; "
                             f"the API reads still open: {', '.join(api.pending()) or 'none'}; "
                             f"the message line: {line or 'empty'}") from None


# --- a page of its own ------------------------------------------------------

def owner_page(new_context, guard, watch, url, token, clock=False):
    """A page of its own with the owner's key token at url, the tiles
    shown (wait_started) and the stream open, as the fixture page has
    it: for a test that signs out or meets a 401 (test_pairing_updates.py,
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
    page.set_default_timeout(scaled(5) * 1000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    api = ApiReads(page)
    page.goto(url)
    wait_started(page, api)
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


# --- the log's own lines (test_log_updates.py, the guard's log-lines) ----

# The test's lines of the log, a command's line with its detail among
# them, and the stub that answers the log's reads of the Cube with them.

LOG_BASE = datetime(2026, 11, 20, 8, 0, tzinfo=timezone.utc)
LOG_FIRST = 970000  # the test's ids, above the server's


def log_event(n, kind="hello", **kw):
    """The test's event n: a hello of the Cube unless told otherwise."""
    e = {"id": LOG_FIRST + n, "kind": kind, "source": "cube", "level": "info", "time": (LOG_BASE + timedelta(seconds=n)).isoformat().replace("+00:00", "Z"),
         "summary": {"firmware": "1.1.3", "duty_cycle_pct": n % 50, "free_memory_slots": 10}}
    e.update(kw)
    return e


LOG_CMD = 77  # the command of the test's command line
LOG_CMD_LINE = LOG_FIRST + 19
LOG_TEXT_LINE = LOG_FIRST + 18
LOG_ALERT_A, LOG_ALERT_B = LOG_FIRST + 17, LOG_FIRST + 16


def log_lines(n=20):
    """n lines, oldest first, the last ones a command's line and lines to hold."""
    out = [log_event(i - n) for i in range(n - 4)]  # older than the four below
    out += [log_event(16), log_event(17), log_event(18, summary={"firmware": "selected-firmware", "duty_cycle_pct": 1, "free_memory_slots": 2}),
            log_event(19, "command", source="queue", command=LOG_CMD, summary={"state": "acknowledged"})]
    return out


class StubLog:
    """Answers the log's reads of the Cube from lines (oldest first), as
    the server pages them: the newest limit, before an id, after one; with
    gone true, an after= with 409, a log that ends before that line; with
    fail true, every read with 503, a server not up yet; with
    whole true, the newest page with every line, whatever the limit (a log
    at its 2000 lines in one load). off() takes the route away."""

    PATTERN = "**/api/v1/cubes/*/log?*"

    def __init__(self, page, lines, whole=False):
        self.page, self.lines, self.whole, self.gone, self.fail = page, lines, whole, False, False
        self.asked = []
        self.routed = safe_route(page, self.PATTERN, self.answer)

    def answer(self, route):
        url = route.request.url
        self.asked.append(url)
        q = parse_qs(urlparse(url).query)
        limit = int(q.get("limit", ["200"])[0])
        if self.fail:
            route.fulfill(status=503, content_type="application/json", body=json.dumps({"error": "the server is starting"}))
            return
        if "after" in q and self.gone:
            route.fulfill(status=409, content_type="application/json", body=json.dumps({"error": "beyond the newest"}))
            return
        if "after" in q:
            body = [x for x in self.lines if x["id"] > int(q["after"][0])][:limit]
        else:
            page = self.lines
            if "before" in q:
                page = [x for x in page if x["id"] < int(q["before"][0])]
            body = page if self.whole else page[-limit:]
        route.fulfill(status=200, content_type="application/json", body=json.dumps(body))

    def off(self):
        if not self.page.is_closed():
            self.page.unroute(self.PATTERN, self.routed)


LOG_CMD_DETAIL = {"id": "00000000-0000-0000-0000-000000000077", "action": "setpoint", "target": "room:1", "state": "acknowledged", "user": "owner",
          "steps": [{"state": "acknowledged", "target": "room 1"}]}


def stub_log_command(page):
    """The test's command's detail, GET /api/v1/commands/77; the route as routed."""
    return safe_route(page, f"**/api/v1/commands/{LOG_CMD}", lambda r: r.fulfill(status=200, content_type="application/json", body=json.dumps(LOG_CMD_DETAIL)))
