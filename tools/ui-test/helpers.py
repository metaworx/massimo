# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""What the tests of the web UI suite share: the texts they expect in
English and German, the fixed answers they stub, and the helpers that
wait on the page or look at all of it.

The page's Content-Security-Policy rules out wait_for_function with a
string, so waiting on the page's state polls page.evaluate from here
(wait_until). `state` is a top-level const of app.js, in the page's
global scope but not on window; page.evaluate reaches it there.
"""
import time

OPEN = {"en": "Open", "de": "Offen"}
USED = "0" * 48
FOUND2 = {"serial": "LEQ0000506", "configured": False, "address": "192.168.1.78", "rf_address": "0a1b2e",
          "firmware": "1.1.3", "found_at": "2026-11-05T18:40:00+01:00", "last_seen": "2026-11-05T18:40:00+01:00"}
CHANGES = {"en": "window opened, set 18.0 → 5.0", "de": "Fenster geöffnet, Soll 18,0 → 5,0"}
SEARCHING = {"en": "Searching…", "de": "Suche läuft…"}
FOUND_N = {"en": "{n} Cube{s} found, not configured", "de": "{n} Cube{s} gefunden, nicht eingerichtet"}
STALE = {"en": "Not seen for 30 hours", "de": "Seit 30 Stunden keine Meldung"}
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


def open_user_menu(page):
    """The 👤 menu in the header, open."""
    menu = page.locator("#user-menu")
    if menu.get_attribute("open") is None:
        menu.locator("summary").click()
    page.locator("#user-menu .menu-items").wait_for(state="visible")


def sign_out(page):
    """Sign out (or Forget key) from the 👤 menu."""
    open_user_menu(page)
    page.locator("#logout-btn").click()


# /me of a viewer signed in, for a test that stubs it: no capability of
# System settings.
VIEWER = {"mode": "users", "user": {"name": "viewer1", "role": "viewer", "has_password": True},
          "role": {"key": "viewer"}, "capabilities": ["read", "preferences"], "token": {"kind": "session"}}


# The pages of the main area: how a user opens each, and the section it
# shows.
VIEW_BUTTON = {"tiles": "#view-tiles", "log": "#view-log", "system": "#view-settings"}
VIEW_MENU = {"profile": "#menu-profile", "prefs": "#menu-prefs", "notify": "#menu-notify"}
VIEW_FOLDED = {"log": "#menu-log", "system": "#menu-system"}  # in ☰ below 600 px
VIEW_SECTION = {"tiles": "#rooms", "log": "#logview", "profile": "#profile", "prefs": "#prefs", "notify": "#notify", "system": "#system"}


def show_view(page, view):
    """A page of the main area, opened as a user opens it: Rooms, the log
    and System settings with their button in the header, or on a narrow
    screen the log and System settings from ☰, My profile, My settings
    and Notifications from the 👤 menu."""
    if view in VIEW_MENU:
        open_user_menu(page)
        page.locator(VIEW_MENU[view]).click()
    elif view in VIEW_FOLDED and page.locator("#nav-menu").is_visible():
        menu = page.locator("#nav-menu")
        if menu.get_attribute("open") is None:
            menu.locator("summary").click()
        page.locator(VIEW_FOLDED[view]).click()
    else:
        page.locator(VIEW_BUTTON[view]).click()
    page.locator(VIEW_SECTION[view]).wait_for(state="visible")