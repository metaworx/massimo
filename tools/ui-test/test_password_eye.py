# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The password fields' eye (AP MobileApp, D84-A; ui/src/shared/password.tsx):
a button at a password field's right end, inside its box, a closed eye
while the password is hidden and an open one while it shows, labelled
Show password and Hide password and pressed while it shows, keyboard
operable; it turns the field's type between password and text, keeping
its value, its caret and its autocomplete; the password hidden again at
each opening of the sign-in card and at the submit. Every password
field has it: the sign-in's, the first owner's and My profile's new
password, one eye for the password and its repetition, which shows and
hides both (AP MobileApp, Block 20), My profile's current one and System
settings → Users'; an API key's field has none. The repetition of a new
password is outlined in red while it differs, a line under it saying so
(D98-A). The server's
setting auth.password_reveal turns it off, as /healthz says before
anyone signs in: then no field shows the eye, and the browser's own eye
stays hidden; System settings → Users has its switch, and the page's
fields follow it at once. In a browser and in the app (fake_bridge.js).
The console is not watched where the page starts without a token: its
first requests are refused on purpose, as in test_login.py."""
import json

from conftest import URL
from helpers import OPERATOR_PASSWORD, close_fold, frames, named_buttons, open_fold, show_view, wait_until
from test_connect import server_page
from test_first_owner import OPEN, owner_card, start_page
from test_login import sign_in

SHOW = {"en": "Show password", "de": "Passwort zeigen"}
HIDE = {"en": "Hide password", "de": "Passwort verbergen"}

# A password field and its eye as they stand: the field's type, value,
# caret and autocomplete, whether it has the focus, and the eye's state,
# name and icon, and whether it sits inside the field's box at its end.
STATE = """(id) => {
  const f = document.getElementById(id);
  const eye = f.parentElement.querySelector(':scope > .pw-eye');
  const out = {type: f.type, value: f.value, focused: document.activeElement === f, caret: [f.selectionStart, f.selectionEnd],
               autocomplete: f.getAttribute('autocomplete'), eye: !!eye,
               keyboard: [f.getAttribute('autocorrect'), f.getAttribute('autocapitalize'), f.getAttribute('spellcheck')]};
  if (!eye) return out;
  const fr = f.getBoundingClientRect(), er = eye.getBoundingClientRect();
  return Object.assign(out, {pressed: eye.getAttribute('aria-pressed'), label: eye.getAttribute('aria-label'), title: eye.title,
    icon: eye.querySelector('use').getAttribute('href'), controls: eye.getAttribute('aria-controls'),
    inside: er.left >= fr.left && er.right <= fr.right + 0.5 && er.top >= fr.top - 0.5 && er.bottom <= fr.bottom + 0.5 && fr.right - er.right < 8});
}"""


def eye_of(page, field):
    return page.locator(f".pw-box:has(> #{field}) > .pw-eye")


def state(page, field):
    return page.evaluate(STATE, field)


def hidden(s, case, what):
    assert s["type"] == "password" and s["pressed"] == "false" and s["label"] == SHOW[case.lang] and s["title"] == SHOW[case.lang] \
        and s["icon"] == "#icon-eye-off", f"{what}: hidden, a closed eye, Show password: {s}"


def shown(s, case, what):
    assert s["type"] == "text" and s["pressed"] == "true" and s["label"] == HIDE[case.lang] and s["icon"] == "#icon-eye", \
        f"{what}: shown, an open eye, Hide password: {s}"


def say_reveal(page, reveal):
    """/healthz answers password_reveal so from now on, as the server's
    setting would make it."""
    def healthz(route):
        r = route.fetch()
        body = r.json()
        body["password_reveal"] = reveal
        route.fulfill(response=r, json=body)
    page.route("**/healthz", healthz)


def fresh_page(new_context, guard, watch, reveal=None):
    """A page of its own without the stored token, at the sign-in card;
    with reveal, /healthz says password_reveal so (say_reveal)."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    if reveal is not None:
        say_reveal(page, reveal)
    page.goto(URL)
    page.locator("#login-dialog[open]").wait_for()
    return page


# The sign-in card closed, its close handled, and opened anew, on the way
# given ("owner" for the first owner's form), not awaited.
REOPEN = """(on) => new Promise((done) => {
  const d = document.getElementById('login-dialog');
  d.addEventListener('close', () => setTimeout(() => { askLogin(on || undefined); done(); }, 0), { once: true });
  d.close();
})"""


def reopen(page, on=None):
    """The sign-in card closed and opened anew, as a later refusal opens it."""
    page.evaluate(REOPEN, on)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5)
    frames(page)


def check_eye(page, case, field, value, pair=None):
    """The field's eye turns it to text and back, the value, the caret and
    the autocomplete kept and the focus in the field, by a click and by
    the keyboard; with pair, the repetition's field, which has no eye of
    its own and turns with it."""
    eye = eye_of(page, field)
    eye.wait_for(state="visible")
    s = state(page, field)
    hidden(s, case, field)
    controls = field + (" " + pair if pair else "")
    assert s["inside"] and s["controls"] == controls, f"{field}: the eye inside its box at its right end, naming what it turns: {s}"
    page.locator("#" + field).fill(value)
    page.locator("#" + field).evaluate("(el) => { el.focus(); el.setSelectionRange(2, 3); }")
    auto = s["autocomplete"]
    eye.click()
    s = state(page, field)
    shown(s, case, field)
    if pair:
        p = state(page, pair)
        assert p["type"] == "text" and not p["eye"], f"{pair}: shown with {field}, no eye of its own: {p}"
    assert s["value"] == value and s["autocomplete"] == auto, f"{field}: the value and the autocomplete kept: {s}"
    assert s["keyboard"] == ["off", "none", "false"], f"{field}: shown as text, the keyboard asked for no correction: {s}"
    assert s["focused"] and s["caret"] == [2, 3], f"{field}: the focus and the caret where they were: {s}"
    eye.click()
    s = state(page, field)
    hidden(s, case, field)
    if pair:
        assert state(page, pair)["type"] == "password", f"{pair}: hidden with {field}"
    assert s["value"] == value and s["focused"] and s["caret"] == [2, 3], f"{field}: back, all kept: {s}"
    # the keyboard: the eye after its field, Space and Enter turn it
    eye.focus()
    page.keyboard.press("Space")
    shown(state(page, field), case, field + " by Space")
    page.keyboard.press("Enter")
    hidden(state(page, field), case, field + " by Enter")


def test_the_sign_in_password_eye(new_context, case, watch, guard):
    """The sign-in's password has its eye, which shows and hides the
    password, keeps what is typed and the caret, and is keyboard
    operable; the API key's field has none; the password is hidden again
    at the submit, a refused one staying in the card, and at the card's
    next opening."""
    page = fresh_page(new_context, guard, watch)
    check_eye(page, case, "login-password", OPERATOR_PASSWORD)
    named_buttons(page, "the sign-in with the eye")
    assert page.evaluate("document.querySelectorAll('#login-form input[type=password]').length") == 1, "one password field in the form"
    assert page.locator("#login-token").evaluate("(el) => !el.closest('.pw-box')"), "an API key's field has no eye"
    eye = eye_of(page, "login-password")
    # hidden again at the submit: a wrong password refused, the card stays
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("wrong-pass")
    eye.click()
    shown(state(page, "login-password"), case, "before the submit")
    page.locator("#login-form button[type=submit]").click()
    assert wait_until(page, "!document.getElementById('login-error').classList.contains('hidden')", 10), "refused"
    hidden(state(page, "login-password"), case, "after the submit")
    # hidden again at the next opening
    eye.click()
    shown(state(page, "login-password"), case, "before the next opening")
    reopen(page)
    hidden(state(page, "login-password"), case, "at the next opening")
    # the sign-in still goes through, the password as typed
    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
    eye.click()
    page.locator("#login-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert page.evaluate("state.me && state.me.user && state.me.user.name") == "operator", "signed in"


def test_the_first_owners_password_eyes(new_context, case, watch, guard):
    """The first owner's password has one eye, which shows and hides the
    password and its repetition together, the repetition drawing none (AP
    MobileApp, Block 20); both hidden again at the next opening."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    assert eye_of(page, "setup-again").count() == 0, "no eye on the repetition"
    page.locator("#setup-again").fill("own-pass")
    check_eye(page, case, "setup-password", "own-pass", pair="setup-again")
    eye_of(page, "setup-password").click()
    assert state(page, "setup-password")["type"] == "text" and state(page, "setup-again")["type"] == "text", "both shown"
    named_buttons(page, "the first owner's form with its eye")
    reopen(page, "owner")
    owner_card(page)
    hidden(state(page, "setup-password"), case, "at the next opening")
    assert state(page, "setup-again")["type"] == "password", "the repetition too"


# The repetition's look: whether it is marked invalid, its line's text
# where it has one, and whether its box is outlined in the page's red.
AGAIN = """(id) => {
  const f = document.getElementById(id);
  const line = document.getElementById(id + '-problem');
  const bad = getComputedStyle(document.body).getPropertyValue('--bad').trim();
  const probe = document.createElement('span');
  probe.style.color = bad;
  document.body.appendChild(probe);
  const red = getComputedStyle(probe).color;
  probe.remove();
  return {invalid: f.getAttribute('aria-invalid'), line: line ? line.textContent : null, described: f.getAttribute('aria-describedby'),
          outlined: getComputedStyle(f).borderTopColor === red, lineRed: line ? getComputedStyle(line).color === red : null,
          below: line ? Math.round(line.parentElement.getBoundingClientRect().bottom) : null};
}"""


def check_repetition(page, first, again, what):
    """The repetition of a new password outlined in red while it is not
    empty and differs from the first, \"The two passwords differ.\" under
    it, both gone as the two match (D98-A), the line's place kept, so that
    nothing under it moves."""
    mismatch = page.evaluate("t('setup.mismatch')")
    page.locator("#" + first).fill("own-pass-01")
    s = page.evaluate(AGAIN, again)
    assert s["invalid"] is None and s["line"] == "" and not s["outlined"], f"{what}: nothing while the repetition is empty: {s}"
    place = s["below"]
    page.locator("#" + again).fill("own-pass-0")
    s = page.evaluate(AGAIN, again)
    assert s["invalid"] == "true" and s["line"] == mismatch and s["described"] == again + "-problem", f"{what}: marked while it differs: {s}"
    assert s["outlined"] and s["lineRed"], f"{what}: in red: {s}"
    assert s["below"] == place, f"{what}: the line in its place, nothing under it moved: {s['below']} vs {place}"
    page.locator("#" + again).press("1")
    s = page.evaluate(AGAIN, again)
    assert s["invalid"] is None and s["line"] == "" and not s["outlined"] and s["below"] == place, f"{what}: gone as they match, its place kept: {s}"
    page.locator("#" + first).fill("own-pass-02")
    s = page.evaluate(AGAIN, again)
    assert s["invalid"] == "true" and s["line"] == mismatch, f"{what}: a change of the first marks it again: {s}"
    page.locator("#" + again).fill("")
    assert page.evaluate(AGAIN, again)["line"] == "", f"{what}: emptied, nothing"


def test_the_repetition_that_differs(new_context, case, watch, guard):
    """On the first owner's form and My profile's change of password, the
    new password's two users, the repetition is outlined in red while it
    differs from the password, with its line under it, both gone as the two
    match, and none again at the card's next opening."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    check_repetition(page, "setup-password", "setup-again", "the first owner's form")
    page.locator("#setup-password").fill("own-pass-01")
    page.locator("#setup-again").fill("own-pass-0")
    assert page.evaluate(AGAIN, "setup-again")["line"] != ""
    reopen(page, "owner")
    owner_card(page)
    assert page.evaluate(AGAIN, "setup-again")["line"] == "", "none at the next opening"

    page = fresh_page(new_context, guard, watch)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    show_view(page, "profile")
    open_fold(page, "#me-password")
    page.locator("#me-new").wait_for(state="visible")
    check_repetition(page, "me-new", "me-again", "My profile")


def test_my_profiles_password_eyes(new_context, case, watch, guard):
    """My profile's change of password, the current one with its eye and
    the new one twice with one eye for both; the values typed kept; a password shown is
    hidden again when the fold is closed and opened, the field drawn, also
    where the two come before the page takes the first, as one toggle
    event, as a loaded page meets them; hidden in the very task that
    opens the fold, before a paint or a read can come between (the CI's).
    Before an eye is pressed, the fold's own toggle event is taken
    (open_fold, close_fold), which starts its password fields anew and
    would hide a password shown before it."""
    page = fresh_page(new_context, guard, watch)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    show_view(page, "profile")
    open_fold(page, "#me-password")
    page.locator("#me-new").wait_for(state="visible")
    check_eye(page, case, "me-current", "not-set-me-current")
    assert eye_of(page, "me-again").count() == 0, "no eye on the repetition"
    page.locator("#me-again").fill("not-set-me-again")
    check_eye(page, case, "me-new", "not-set-me-new", pair="me-again")
    named_buttons(page, "My profile's password with its eyes")
    eye_of(page, "me-new").click()
    shown(state(page, "me-new"), case, "before the fold closes")
    close_fold(page, "#me-password")
    open_fold(page, "#me-password")
    page.locator("#me-new").wait_for(state="visible")
    s = state(page, "me-new")
    hidden(s, case, "the fold closed and opened again")
    assert s["value"] == "not-set-me-new", f"what is typed kept: {s}"
    # closed, then opened: the password hidden in the very task that opens the fold, before it can be painted, not at
    # the toggle event's later task, which a loaded page runs after other work, a paint or the test's read among it
    # (the CI's)
    eye_of(page, "me-new").click()
    shown(state(page, "me-new"), case, "before the fold closes again")
    page.evaluate("() => { document.getElementById('me-password-title').click(); }")
    assert wait_until(page, "!document.getElementById('me-password').open", 3)
    frames(page)
    typed = page.evaluate("""async () => { document.getElementById('me-password-title').click(); await Promise.resolve();
      return [document.getElementById('me-password').open, document.getElementById('me-new').type]; }""")
    assert typed == [True, "password"], f"hidden in the task that opens the fold: {typed}"
    frames(page)
    page.locator("#me-new").wait_for(state="visible")
    hidden(state(page, "me-new"), case, "opened again, drawn")
    # closed and opened before the page takes the first toggle, as a loaded page does: one event, open to open, which
    # hides it all the same
    eye_of(page, "me-new").click()
    shown(state(page, "me-new"), case, "before the fold's double press")
    events = page.evaluate("""() => new Promise((done) => { const d = document.getElementById('me-password'); const log = [];
      d.addEventListener('toggle', (e) => { log.push(e.oldState + '>' + e.newState); setTimeout(() => done(log), 0); }, { once: true });
      const t = document.getElementById('me-password-title'); t.click(); t.click(); })""")
    assert events == ["open>open"], f"one event: {events}"
    frames(page)
    hidden(state(page, "me-new"), case, "closed and opened at once")


def toggle(page, panel):
    """Opens or closes a fold from its summary by the keyboard (at 390 px a
    click may land after the header has compacted under it, and miss),
    then lets the fold's part take its toggle, so that the next one meets
    it drawn."""
    summary = panel.locator(":scope > summary")
    summary.focus()
    summary.press("Enter")
    frames(page)
    frames(page)


def test_the_users_password_eye_and_its_switch(page, case):
    """System settings → Users: the new user's password has its eye, and so
    does a user's panel's, a password shown hidden again when the panel is
    closed and opened, the field drawn; the switch of the eye, on by default with its line, turned off
    writes auth.password_reveal false and the page's fields lose their eye
    at once, a password shown before hidden, on again brings the eye back,
    the password still hidden (the setting's write stubbed, so that no
    other test meets it)."""
    bodies = []

    def settings(route):
        if route.request.method != "PUT":
            return route.continue_()
        bodies.append(json.loads(route.request.post_data or "{}"))
        route.fulfill(status=200, content_type="application/json", body="{}")
    page.route("**/api/v1/settings", settings)
    try:
        show_view(page, "system")
        switch = page.locator("#password-reveal")
        switch.wait_for(state="attached")
        assert switch.is_checked(), "on by default"
        assert page.locator("#password-reveal-hint").is_visible()
        # a user's panel: the password shown is hidden again as the panel closes
        panel = page.locator("details.user-panel[data-name='operator']")
        toggle(page, panel)
        check_eye(page, case, "user-operator-password", "new-pass")
        eye_of(page, "user-operator-password").click()
        shown(state(page, "user-operator-password"), case, "before the panel closes")
        toggle(page, panel)
        assert wait_until(page, "!document.querySelector(\"details.user-panel[data-name='operator']\").open", 3)
        # opened again, the field is drawn, its password hidden (a type turned inside the closed panel drew nothing)
        toggle(page, panel)
        page.locator("#user-operator-password").wait_for(state="visible")
        s = state(page, "user-operator-password")
        hidden(s, case, "the panel closed and opened again")
        assert s["value"] == "new-pass", f"what is typed kept: {s}"
        page.locator("#user-operator-password").fill("")
        toggle(page, panel)
        page.locator("#user-add").click()
        check_eye(page, case, "user-new-password", "new-pass")
        eye_of(page, "user-new-password").click()
        shown(state(page, "user-new-password"), case, "before the switch turns off")
        page.locator("label.switch", has=switch).locator(".slider").click()
        assert wait_until(page, "!document.querySelector('.pw-box:has(> #user-new-password) > .pw-eye')", 5), "off: the field loses its eye at once"
        assert bodies == [{"auth.password_reveal": False}], f"the setting written: {bodies}"
        assert page.locator("#user-new-password").evaluate("(el) => el.type") == "password", "the password shown before is hidden"
        page.locator("label.switch", has=switch).locator(".slider").click()
        eye_of(page, "user-new-password").wait_for(state="visible")
        assert bodies[-1] == {"auth.password_reveal": True}, f"on again: {bodies}"
        hidden(state(page, "user-new-password"), case, "the eye back, the password still hidden")
        # switched off and on while the panel holding a password shown is closed: opened again, drawn and hidden
        toggle(page, panel)
        page.locator("#user-operator-password").fill("new-pass")
        eye_of(page, "user-operator-password").click()
        toggle(page, panel)
        page.locator("label.switch", has=switch).locator(".slider").click()
        assert wait_until(page, "!document.querySelector('.pw-box:has(> #user-new-password) > .pw-eye')", 5)
        page.locator("label.switch", has=switch).locator(".slider").click()
        eye_of(page, "user-new-password").wait_for(state="visible")
        toggle(page, panel)
        page.locator("#user-operator-password").wait_for(state="visible")
        hidden(state(page, "user-operator-password"), case, "switched off and on while closed, opened again")
    finally:
        page.unroute("**/api/v1/settings")
        page.evaluate("MassimoParts.chrome.reveal(true)")
        show_view(page, "tiles")


def test_the_eye_switched_off(new_context, case, watch, guard):
    """With auth.password_reveal off, as /healthz says it before anyone
    signs in, the sign-in card's password has no eye from its first
    drawing: none while /healthz has not answered, none once it has said
    off; nor the first owner's two fields; and the browser's own eye
    stays hidden on every field (style.css, for Edge's ::-ms-reveal)."""
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    held, hold = [], [True]

    def answer(route):
        r = route.fetch()
        route.fulfill(response=r, json=dict(r.json(), password_reveal=False))

    page.route("**/healthz", lambda route: held.append(route) if hold[0] else answer(route))
    page.goto(URL)
    page.locator("#login-dialog[open]").wait_for()
    assert wait_until(page, "!!document.getElementById('login-password')", 5)
    assert held, "/healthz held"
    frames(page)
    assert page.locator(".pw-eye").count() == 0, "no eye before /healthz has said"
    hold[0] = False
    while held:
        answer(held.pop())
    assert wait_until(page, "MassimoParts && document.getElementById('login-password')", 5)
    page.wait_for_timeout(500)  # /healthz has answered long since
    assert page.locator(".pw-eye").count() == 0, "no eye once it said off"
    s = state(page, "login-password")
    assert s["type"] == "password" and not s["eye"], f"a password's field: {s}"
    assert page.locator("#login-password").evaluate("(el) => el.closest('.pw-box') && !el.closest('.pw-box').classList.contains('has-eye')"), \
        "the field's box without an eye's room"
    css = page.evaluate("fetch('ui/style.css').then((r) => r.text())")
    assert "input::-ms-reveal, input::-ms-clear { display: none; }" in css, "the browser's own eye hidden on every field"
    owner = start_page(new_context, guard, watch, OPEN)
    say_reveal(owner, False)
    owner.reload()
    owner_card(owner)
    owner.wait_for_timeout(500)
    assert owner.locator(".pw-eye").count() == 0, "nor on the first owner's form"


def test_the_eye_in_the_app(new_context, case, watch, guard):
    """In the app the sign-in's password has its eye as in a browser,
    hidden again at the next opening."""
    page = server_page(new_context, guard, watch, {"status": {"can": True, "user": "", "ask": "start", "plain": False}})
    page.locator("#login-dialog[open]").wait_for()
    check_eye(page, case, "login-password", OPERATOR_PASSWORD)
    eye_of(page, "login-password").click()
    reopen(page)
    hidden(state(page, "login-password"), case, "at the next opening in the app")
