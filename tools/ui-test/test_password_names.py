# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The names in a new password, in the page (AP PasswordRules, Block 3,
D50-A; ui/src/shared/password.tsx): a new password that contains the
user's name or display name, the server's name, massimo or maxcube,
whatever the case, is refused in the page's forms as Go refuses it,
the line under the field saying so while it is typed and, a submit
refused, that it was refused. The line follows the names as well as the
password: a name typed or changed after the password changes the line
without a key in the password's field. The server's name is the first
label of the page's host, none for an IP address or localhost (the
suite's page is at 127.0.0.1), and in the app the name the app gives
the server; the hosts of cases.json give the page's hostWord Go's
answers. Test_password_rule.py holds the rule's cases and its length."""
import json

from conftest import URL, api_call
from helpers import OPERATOR_PASSWORD, frames, show_view, wait_until
from test_first_owner import OPEN, owner_card, start_page, stub_creation
from test_login import sign_in
from test_password_eye import fresh_page
from test_password_rule import CASES, line, words, writes
from test_users import remove_user

# The suite's server at a host name whose first label is a word:
# Chromium resolves *.localhost to the loopback by itself, and the
# server does not check the Host.
KITCHEN = URL.replace("127.0.0.1", "kitchen.localhost")

CONTEXT = "password.context"
REFUSED = "password.refusedContext"
FINE = "password.fine"


def said(page, field, key, invalid=None):
    """The field's line says key, with the warning's sign where it refuses,
    and the field is marked invalid as invalid says."""
    s = line(page, field)
    sign = "#icon-check" if key == FINE else "#mark-warn"
    assert s["text"] == words(page, key) and s["sign"] == sign and s["invalid"] == invalid, f"{field}: {key}: {s}"


def test_the_page_answers_each_host_as_go(page, case):
    """Every host of internal/password/testdata/cases.json, given to the
    page's hostWord through its handle, gets Go's HostWord answer. Fails
    on the page before: no handle."""
    with open(CASES, encoding="utf-8") as f:
        hosts = json.load(f).get("hosts", [])
    assert len(hosts) >= 10, f"the hosts read: {len(hosts)}"
    answers = page.evaluate("""(hosts) => hosts.map((h) => {
      const w = window.MassimoParts.hostWord;
      return typeof w === 'function' ? w(h.host) : 'no handle';
    })""", hosts)
    wrong = [f"{h['why']}: {a!r}, Go's {h['word']!r}" for h, a in zip(hosts, answers) if a != h["word"]]
    assert not wrong, "the page answers otherwise than Go:\n" + "\n".join(wrong)


def test_the_first_owners_line_follows_the_name(new_context, case, watch, guard):
    """The first owner's form: a password typed before the name is fine;
    the name typed after it, the line says that it contains the name,
    with no key in Password; Enter refuses it there, nothing sent, the
    form's line empty; another name brings the line back to fine, not
    marked; a name filled without an event is read at the submit; and
    a name's change event without an input event (a password manager's
    fill) is followed too; massimo is refused with no name at all. Fails
    on the page before: the names refused nothing."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    made = []
    stub_creation(page, made)
    pw = page.locator("#setup-password")
    pw.fill("x-anna-2026")
    page.locator("#setup-again").fill("x-anna-2026")
    frames(page)
    said(page, "setup-password", FINE)
    # the name's field in focus first, so that Password's change event at its blur has been drawn before a key
    name = page.locator("#setup-user")
    name.focus()
    frames(page)
    name.press_sequentially("Anna")
    frames(page)
    said(page, "setup-password", CONTEXT)
    # a name filled without an input event, its change event alone: the line follows it
    page.evaluate("() => { const f = document.getElementById('setup-user'); f.value = 'bert'; f.dispatchEvent(new Event('change', {bubbles: true})); }")
    frames(page)
    said(page, "setup-password", FINE)
    page.evaluate("() => { const f = document.getElementById('setup-user'); f.value = 'anna'; f.dispatchEvent(new Event('change', {bubbles: true})); }")
    frames(page)
    said(page, "setup-password", CONTEXT)
    pw.focus()
    pw.press("Enter")
    frames(page)
    said(page, "setup-password", REFUSED, "true")
    assert page.evaluate("document.activeElement.id") == "setup-password", "the focus in Password"
    assert page.locator("#setup-msg").inner_text() == "", "the form's line says nothing of it"
    name.focus()
    frames(page)
    name.fill("bert")
    frames(page)
    said(page, "setup-password", FINE)
    # a name filled without an event (a password manager's): the submit reads it
    page.evaluate("() => { document.getElementById('setup-user').value = 'ANNA'; document.getElementById('setup-form').requestSubmit(); }")
    frames(page)
    said(page, "setup-password", REFUSED, "true")
    page.locator("#setup-user").fill("")
    pw.fill("my-MASSIMO-1")
    frames(page)
    said(page, "setup-password", CONTEXT)
    assert not made, f"nothing sent: {made}"


def test_the_first_owner_in_the_app_takes_the_servers_name(new_context, case, watch, guard):
    """In the app, the name it gives the server is a word: a password with
    one of its words is refused, the same with one letter changed is
    fine. Fails on the page before: the names refused nothing."""
    app = {"status": {"can": True, "user": "", "ask": "start", "plain": False,
                      "server": {"url": URL.rstrip("/"), "name": "Attic Box", "node": "", "fp": ""}}}
    page = start_page(new_context, guard, watch, OPEN, app=app)
    owner_card(page)
    assert wait_until(page, "!!document.getElementById('setup-password')", 5), "the form"
    page.locator("#setup-user").fill("bert")
    page.locator("#setup-password").fill("my-ATTIC-2026")
    frames(page)
    said(page, "setup-password", CONTEXT)
    page.locator("#setup-password").fill("my-attik-2026")
    frames(page)
    said(page, "setup-password", FINE)


def test_my_profile_refuses_the_users_name(page, case):
    """My profile's Set a password of the test's owner, named owner and
    without a display name, whose empty word refuses nothing: the
    owner's name in capitals is refused, and Enter says so under the
    field, nothing sent. Fails on the page before: the names refused
    nothing."""
    show_view(page, "profile")
    if not page.locator("#me-password").evaluate("(d) => d.open"):
        page.locator("#me-password-title").click()
    page.locator("#me-new").wait_for(state="visible")
    out, stop = writes(page)
    try:
        page.locator("#me-new").fill("ferry-lamp-7")
        frames(page)
        said(page, "me-new", FINE)
        page.locator("#me-new").fill("x-OWNER-2026")
        page.locator("#me-again").fill("x-OWNER-2026")
        frames(page)
        said(page, "me-new", CONTEXT)
        page.locator("#me-new").press("Enter")
        frames(page)
        said(page, "me-new", REFUSED, "true")
        assert page.locator("#me-password-msg").inner_text() == "", "the form's line says nothing of it"
        assert not out, f"nothing sent: {out}"
    finally:
        stop()
        page.locator("#me-password-form .form-reset").click()
        page.locator("#me-password-title").click()
    show_view(page, "tiles")


def test_adding_a_user_follows_its_names(page, case):
    """Add a user: the password typed first; the name typed after it, then
    the display name, each changes the line without a key in the
    password; Create refused while it holds the display name, nothing
    sent; the display name taken away, Create adds the user with its
    password. Fails on the page before: the names refused nothing."""
    name = f"dora-{case.width}-{case.lang}"
    remove_user(name)
    show_view(page, "system")
    out, stop = writes(page)
    field = "user-new-password"
    try:
        page.locator("#user-add").click()
        page.locator("#" + field).wait_for(state="visible")
        page.locator("#" + field).fill("x-river-2026")
        frames(page)
        said(page, field, FINE)
        page.locator("#user-new input[name=name]").fill("river")
        frames(page)
        said(page, field, CONTEXT)
        page.locator("#user-new input[name=name]").fill(name)
        frames(page)
        said(page, field, FINE)
        page.locator("#user-new input[name=display_name]").fill("River Stone")
        frames(page)
        said(page, field, CONTEXT)
        page.locator("#user-new button[type=submit]").click()
        frames(page)
        said(page, field, REFUSED, "true")
        assert page.evaluate("document.activeElement.id") == field, "the focus in the password"
        assert not out, f"nothing sent: {out}"
        page.locator("#user-new input[name=display_name]").fill("")
        frames(page)
        said(page, field, FINE)
        with page.expect_response(lambda r: r.request.method == "POST" and r.url.endswith("/api/v1/users")):
            page.locator("#user-new button[type=submit]").click()
        assert wait_until(page, "!document.querySelector('#user-new form')", 10), "the user is added"
        assert [m for m, u in out if u.endswith("/api/v1/users")] == ["POST"], f"one creation: {out}"
    finally:
        stop()
        remove_user(name)
    show_view(page, "tiles")


def test_a_users_panel_follows_its_names(page, case):
    """A user's panel: the user's login name in the password is refused;
    a password without it is fine until the display name typed holds one
    of its words; Save then sends nothing. Fails on the page before: the
    names refused nothing."""
    name = f"lake-{case.width}-{case.lang}"
    remove_user(name)
    api_call("POST", "api/v1/users", {"name": name, "role": "viewer", "password": "fixed-pw-01"})
    field = f"user-{name}-password"
    out, stop = writes(page)
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='%s']" % name)
        panel.wait_for()
        panel.locator("summary").click()
        page.locator("#" + field).wait_for(state="visible")
        page.locator("#" + field).fill("x-LAKE-2026")
        frames(page)
        said(page, field, CONTEXT)
        page.locator("#" + field).fill("x-river-2026")
        frames(page)
        said(page, field, FINE)
        panel.locator("input[name=display_name]").fill("River Stone")
        frames(page)
        said(page, field, CONTEXT)
        panel.locator("form button[type=submit]").click()
        frames(page)
        said(page, field, REFUSED, "true")
        assert not [x for x in out if x[0] == "PUT" or x[1].endswith("/register")], f"nothing sent: {out}"
    finally:
        stop()
        remove_user(name)
    show_view(page, "tiles")



def owner_line(page, pw):
    """The first owner's name bert, which holds no word of the passwords
    below, and pw typed in Password."""
    page.locator("#setup-user").fill("bert")
    page.locator("#setup-password").fill(pw)
    frames(page)


def test_the_page_at_a_host_name_takes_its_first_label(new_context, case, watch, guard):
    """The page opened at kitchen.localhost: the first label of its host
    is the server's name, a password with it is refused and the same with
    one letter changed is fine. Fails without the page's host among the
    server's names."""
    page = start_page(new_context, guard, watch, OPEN, url=KITCHEN)
    owner_card(page)
    assert page.evaluate("location.hostname") == "kitchen.localhost", "the page at kitchen.localhost"
    owner_line(page, "my-KITCHEN-26")
    said(page, "setup-password", CONTEXT)
    owner_line(page, "my-kitchan-26")
    said(page, "setup-password", FINE)


def test_the_apps_name_that_is_the_host_gives_no_word(new_context, case, watch, guard):
    """In the app, a name that is the server's host and port, as the app
    names a server added by its address, gives no word: localhost, a run
    of it, is not refused. Fails where the rule takes that name."""
    host = KITCHEN.split("//", 1)[1].rstrip("/")
    app = {"status": {"can": True, "user": "", "ask": "start", "plain": False,
                      "server": {"url": KITCHEN.rstrip("/"), "name": host, "node": "", "fp": ""}}}
    page = start_page(new_context, guard, watch, OPEN, app=app, url=KITCHEN)
    owner_card(page)
    owner_line(page, "my-localhost-1")
    said(page, "setup-password", FINE)


def test_the_apps_name_that_is_an_address_gives_no_word(new_context, case, watch, guard):
    """In the app, a name that is an address (with ://) gives no word:
    example, a run of it, is not refused. Fails where the rule takes that
    name."""
    app = {"status": {"can": True, "user": "", "ask": "start", "plain": False,
                      "server": {"url": URL.rstrip("/"), "name": "https://lake.example", "node": "", "fp": ""}}}
    page = start_page(new_context, guard, watch, OPEN, app=app)
    owner_card(page)
    owner_line(page, "my-example-1")
    said(page, "setup-password", FINE)


def test_a_users_panel_takes_the_stored_display_name(page, case):
    """A user's panel: the display name the server holds is a name beside
    the one typed in its place, as the server's PUT checks both. Fails
    without the stored display name among the panel's names."""
    name = f"pnl-{case.width}-{case.lang}"
    remove_user(name)
    api_call("POST", "api/v1/users", {"name": name, "role": "viewer", "display_name": "Lake View", "password": "fixed-pw-01"})
    field = f"user-{name}-password"
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='%s']" % name)
        panel.wait_for()
        panel.locator("summary").click()
        page.locator("#" + field).wait_for(state="visible")
        panel.locator("input[name=display_name]").fill("River Stone")
        page.locator("#" + field).fill("x-VIEW-2026")
        frames(page)
        said(page, field, CONTEXT)
        panel.locator("form .form-reset").click()
    finally:
        remove_user(name)
    show_view(page, "tiles")


def test_my_profile_takes_the_display_name(new_context, case, watch, guard):
    """My profile of the operator, given a display name for the test: a
    word of it in the new password is refused. Fails without the display
    name among My profile's names."""
    before = api_call("GET", "api/v1/users/operator").get("display_name") or ""
    api_call("PUT", "api/v1/users/operator", {"display_name": "River Stone"})
    try:
        page = fresh_page(new_context, guard, watch)
        sign_in(page, "operator", OPERATOR_PASSWORD)
        show_view(page, "profile")
        if not page.locator("#me-password").evaluate("(d) => d.open"):
            page.locator("#me-password-title").click()
        page.locator("#me-new").wait_for(state="visible")
        page.locator("#me-new").fill("x-STONE-2026")
        frames(page)
        said(page, "me-new", CONTEXT)
        page.locator("#me-new").fill("x-stane-2026")
        frames(page)
        said(page, "me-new", FINE)
    finally:
        api_call("PUT", "api/v1/users/operator", {"display_name": before})
