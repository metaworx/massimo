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
FOUND2 = {"serial": "LEQ0654321", "configured": False, "address": "192.168.1.78", "rf_address": "0a1b2e",
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


def update_json(state, to, available):
    """GET /api/v1/update's answer with the update helper in the given
    state."""
    return ('{"current": "v0.0.0-test", "channel": "alpha", "latest": "%s", "available": %s, "can_update": %s, '
            '"helper": {"state": "%s", "to": "%s", "time": "2026-01-01T00:00:00Z"}}'
            % (to, "true" if available else "false", "true" if available else "false", state, to))


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


def open_settings(page):
    """The settings page, opened with the cog in the header."""
    page.locator("#view-settings").click()
    page.locator("#settings").wait_for(state="visible")
