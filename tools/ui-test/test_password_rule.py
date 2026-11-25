# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The rule of a new password in the page (AP PasswordRules, Block 2;
ui/src/shared/password.tsx): 8 to 64 code points, as internal/password
holds it in Go, its cases (testdata/cases.json) answered alike through
MassimoParts.passwordProblem. The page's five forms that set a password,
the first owner's, My profile's Set a password and Change password, and
in System settings → Users the adding of a user and a user's panel,
say under the new password while it is typed what it lacks, in a line
that is the field's description, a status region written only when what
it says changes, with a sign beside the words, its place kept so that
the form's other rows stay where they are; the hint, with its own sign,
while the field is empty. A submit the rule refuses, also by Enter in
the field, sends nothing: the line says that it was refused, the field
is marked invalid and has the focus, and the form's own message line
says nothing of it; the next key brings the line back to what the
password is, and once it can be set nothing on the page says otherwise.
The rule's refusal comes before the mismatch. 8 characters are taken.
An empty field where it means unchanged or keys only is not checked; the
upgrade of a version-1 user at its sign-in registers the password it
signed in with, of any length (the sign-in's own wrong passwords, of
five characters, are test_login.py's)."""
import json
import os

from conftest import TOKEN, api_call
from helpers import OPERATOR_PASSWORD, frames, show_view, wait_until
from test_first_owner import OPEN, owner_card, sent, start_page, stub_creation
from test_login import sign_in
from test_password_eye import fresh_page
from test_users import remove_user

CASES = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "internal", "password", "testdata", "cases.json")

SHORT = "mxtqvzl"  # seven letters, refused
FINE = "mxtqvzlp"  # eight, the fewest taken
LONG = "a" * 65

# The field's description as it stands: the line it names, its role, its
# text, the sign beside the words (the symbol of its icon), the line's
# height, whether the field is marked invalid; and the top of the row
# after the field's, which the line must not move.
LINE = """(id) => {
  const f = document.getElementById(id);
  const d = f.getAttribute('aria-describedby');
  const l = d ? document.getElementById(d) : null;
  const row = f.closest('.field');
  const next = row && row.nextElementSibling;
  if (!l) return {id: d, line: false};
  const use = l.querySelector('use');
  return {id: d, line: true, role: l.getAttribute('role'), text: l.textContent, sign: use ? use.getAttribute('href') : null,
          invalid: f.getAttribute('aria-invalid'),
          height: Math.round(l.getBoundingClientRect().height), next: next ? Math.round(next.getBoundingClientRect().top) : null};
}"""

# From now on, a count of what is written into the line, or into its
# parent's list of children (window.__lineWrites), and the line's element
# itself, which must stay the same one (window.__lineEl).
WATCH_LINE = """(id) => {
  window.__lineWrites = 0;
  const l = document.getElementById(document.getElementById(id).getAttribute('aria-describedby'));
  window.__lineEl = l;
  if (window.__lineWatch) window.__lineWatch.disconnect();
  window.__lineWatch = new MutationObserver((records) => { window.__lineWrites += records.length; });
  window.__lineWatch.observe(l, {childList: true, characterData: true, subtree: true});
  window.__lineWatch.observe(l.parentElement, {childList: true});
}"""
SAME_LINE = "(id) => document.getElementById(document.getElementById(id).getAttribute('aria-describedby')) === window.__lineEl"

# Whether any of the words is shown on the page (innerText leaves out the
# line's hidden texts, which hold its place).
SHOWN = "(ws) => ws.filter((w) => document.body.innerText.includes(w))"


def words(page, key):
    return page.evaluate("(k) => t(k)", key)


def line(page, field):
    return page.evaluate(LINE, field)


def check_line(page, field):
    """The field's line while it is typed in: the hint with its sign while
    empty, too short with its sign, fine with its own, too long; the row
    after the field's where it was at each; a ninth and a tenth character
    write nothing into the line, which stays the same element; the field
    left empty."""
    f = page.locator("#" + field)
    f.fill("")
    frames(page)
    s = line(page, field)
    assert s["line"] and s["id"] == field + "-rule", f"{field}: described by its rule's line: {s}"
    assert s["role"] == "status", f"{field}: the line is a status region: {s}"
    assert s["text"] == words(page, "password.hint") and s["sign"] == "#icon-info", f"{field}: empty, the hint with its sign: {s}"
    seen = [s]
    page.evaluate(WATCH_LINE, field)
    for value, key, sign in ((SHORT, "password.tooShort", "#mark-warn"), (FINE, "password.fine", "#icon-check"),
                             (LONG, "password.tooLong", "#mark-warn")):
        f.fill(value)
        frames(page)
        s = line(page, field)
        assert s["text"] == words(page, key) and s["sign"] == sign, f"{field}: {len(value)} characters, {key} with its sign: {s}"
        seen.append(s)
    assert page.evaluate(SAME_LINE, field), f"{field}: the line is the same element through its changes"
    assert len({(x["height"], x["next"]) for x in seen}) == 1, f"{field}: the line keeps its place, the next row unmoved: {seen}"
    f.fill(FINE)
    frames(page)
    page.evaluate(WATCH_LINE, field)
    f.press_sequentially("ab")
    frames(page)
    assert f.input_value() == FINE + "ab", f"{field}: typed on"
    assert page.evaluate("window.__lineWrites") == 0, f"{field}: a ninth and a tenth character write nothing into the line"
    assert page.evaluate(SAME_LINE, field), f"{field}: and leave it the same element"
    assert line(page, field)["text"] == words(page, "password.fine"), f"{field}: still fine"
    page.evaluate("() => { window.__lineWatch.disconnect(); }")
    f.fill("")
    frames(page)


def check_refusal(page, field, msg=None):
    """Seven characters typed (and Again, where the form has one, left as
    it is): Enter in the field, where the focus is already, is refused,
    seen and heard: the line says that it was refused, with its sign, the
    field is marked invalid and keeps the focus, and the form's message
    line, msg, says nothing of it. An eighth character brings the line to
    a password that can be set, and nothing on the page says it is too
    short."""
    f = page.locator("#" + field)
    f.fill(SHORT)
    f.focus()
    f.press("Enter")
    frames(page)
    s = line(page, field)
    assert s["text"] == words(page, "password.refusedShort") and s["sign"] == "#mark-warn", f"{field}: the refusal said under the field: {s}"
    assert s["invalid"] == "true", f"{field}: marked invalid: {s}"
    assert page.evaluate("document.activeElement.id") == field, f"{field}: the focus in the field"
    if msg:
        assert page.evaluate("(sel) => document.querySelector(sel).textContent", msg) == "", f"{field}: the form's line says nothing of it"
    f.press_sequentially("x")
    frames(page)
    s = line(page, field)
    assert s["text"] == words(page, "password.fine") and s["invalid"] is None, f"{field}: an eighth character, it can be set: {s}"
    too = page.evaluate(SHOWN, [words(page, k) for k in ("password.tooShort", "password.refusedShort")])
    assert not too, f"{field}: nothing on the page says it is too short: {too}"


def check_reset(page, field, reset):
    """Refused, then the form's Reset: the field emptied is no longer
    marked, and its line is the hint again."""
    f = page.locator("#" + field)
    f.fill(SHORT)
    f.focus()
    f.press("Enter")
    frames(page)
    assert line(page, field)["invalid"] == "true", f"{field}: refused"
    page.locator(reset).click()
    frames(page)
    s = line(page, field)
    assert f.input_value() == "" and s["invalid"] is None and s["text"] == words(page, "password.hint"), \
        f"{field}: emptied by Reset, not marked, the hint: {s}"


# registerPassword of login.js replaced by one that notes the password it
# is given and answers a record the server is not asked for; the real one
# kept, to be put back.
STUB_REGISTER = """() => {
  window.__registered = [];
  if (!window.__realRegister) window.__realRegister = registerPassword;
  registerPassword = async (name, password) => { window.__registered.push(password); return { record: "r", ksf: "m=19456,t=2,p=1" }; };
}"""
REAL_REGISTER = "() => { if (window.__realRegister) { registerPassword = window.__realRegister; delete window.__realRegister; } }"


def writes(page):
    """The requests other than GET the page sends from now on to the users'
    routes and the sign-in's, as (method, URL), and the function that stops
    counting them; the page's own pushes of its preferences are no
    password's."""
    out = []

    def note(r):
        if r.method != "GET" and ("/api/v1/users" in r.url or "/api/v1/login" in r.url):
            out.append((r.method, r.url))
    page.on("request", note)
    return out, lambda: page.remove_listener("request", note)


def test_the_page_answers_each_case_as_go(page, case):
    """Every case of internal/password/testdata/cases.json, given to the
    page's rule through its handle with its context words, gets Go's
    answer; a case answered otherwise fails by its why. Where the two
    could count apart, the file holds no case: a lone surrogate counts
    one in JavaScript and cannot be in a Go string (JSON turns it into
    U+FFFD, also one), and bytes that are not UTF-8 count one each in Go
    and cannot be in a JavaScript string; neither reaches the other side,
    since the page sends no password but an OPAQUE record (TextEncoder
    turning a lone surrogate into U+FFFD) and Go checks only the command
    line's and the API's plain passwords. Fails on the page before: there
    was no rule, and no handle."""
    with open(CASES, encoding="utf-8") as f:
        cases = json.load(f)["cases"]
    assert len(cases) >= 10, f"the cases read: {len(cases)}"
    answers = page.evaluate("""(cases) => cases.map((c) => {
      const p = window.MassimoParts.passwordProblem;
      return typeof p === 'function' ? (p(c.password, c.context) || 'ok') : 'no handle';
    })""", cases)
    wrong = [f"{c['why']}: {a!r}, Go's {c['answer']!r}" for c, a in zip(cases, answers) if a != c["answer"]]
    assert not wrong, "the page answers otherwise than Go:\n" + "\n".join(wrong)


def test_the_first_owners_form(new_context, case, watch, guard):
    """The first owner's form: its line under Password; a refusal by
    Enter; the rule's refusal before the mismatch, the two fields
    differing and too short; a fill that fires no input event (a password
    manager's) brought into the line at the submit and by its change
    event; eight characters create the owner."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    check_line(page, "setup-password")
    made = []
    stub_creation(page, made)
    page.locator("#setup-user").fill("owner")
    page.locator("#setup-again").fill("other-pass")
    check_refusal(page, "setup-password", "#setup-msg")
    submit = page.locator("#setup-form button[type=submit]")
    page.locator("#setup-password").fill(SHORT)
    submit.click()
    frames(page)
    assert line(page, "setup-password")["text"] == words(page, "password.refusedShort"), "differing and too short: too short first"
    assert page.locator("#setup-msg").inner_text() == "", "and not the mismatch"
    assert page.evaluate("document.activeElement.id") == "setup-password", "the focus in Password"
    page.locator("#setup-password").fill(FINE)
    submit.click()
    assert wait_until(page, "document.getElementById('setup-msg').textContent === t('setup.mismatch')", 3), "then the mismatch"
    assert page.evaluate("document.activeElement.id") == "setup-again", "its focus on Again"
    # a fill without an input event: the line is brought up to date at the submit, and by a change event
    page.locator("#setup-password").fill("")
    page.locator("#setup-user").focus()
    frames(page)
    page.evaluate("(v) => { document.getElementById('setup-password').value = v; document.getElementById('setup-form').requestSubmit(); }", SHORT)
    frames(page)
    assert line(page, "setup-password")["text"] == words(page, "password.refusedShort"), "a silent fill: the submit reads it"
    page.evaluate("(v) => { const f = document.getElementById('setup-password'); f.value = v; f.dispatchEvent(new Event('change', {bubbles: true})); }", FINE)
    frames(page)
    assert line(page, "setup-password")["text"] == words(page, "password.fine"), "a change event: the line follows"
    assert not made and not sent(page, "POST", "/register"), "nothing sent"
    # refused, then a fill without an event that passes: the submit goes through, the mark and Refused gone with it
    page.locator("#setup-password").fill(SHORT)
    submit.click()
    frames(page)
    assert line(page, "setup-password")["invalid"] == "true", "refused again"
    # the sign-in after the creation held, so that the card stays while the line is read
    page.evaluate("(token) => { signIn = () => new Promise((res) => { window.__letSignIn = () => res({ token }); }); }", TOKEN)
    after = page.evaluate("""async (v) => {
      for (const id of ['setup-password', 'setup-again']) document.getElementById(id).value = v;
      document.getElementById('setup-form').requestSubmit();
      await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
      const f = document.getElementById('setup-password');
      return {invalid: f.getAttribute('aria-invalid'), text: document.getElementById('setup-password-rule').textContent};
    }""", FINE)
    assert after["invalid"] is None and after["text"] == words(page, "password.fine"), f"a silent fill that passes: not marked, fine: {after}"
    assert wait_until(page, "typeof window.__letSignIn === 'function'", 5), "the creation reached its sign-in"
    page.evaluate("() => { window.__letSignIn(); }")
    assert wait_until(page, "!document.getElementById('login-dialog').open", 10), "eight characters create the owner"
    assert len(made) == 1, f"created once: {made}"


def submit_profile(page, new, again):
    page.locator("#me-new").fill(new)
    page.locator("#me-again").fill(again)
    page.locator("#me-password-form button[type=submit]").click()


def check_profile(page, name):
    """My profile's form open: its line under the new password; a refusal
    by Enter; the two differing and too short refused by the rule first;
    eight are registered and sent (the PUT stubbed, so that the user's
    password stays)."""
    if not page.locator("#me-password").evaluate("(d) => d.open"):
        page.locator("#me-password-title").click()
    page.locator("#me-new").wait_for(state="visible")
    check_line(page, "me-new")
    put = []

    def user(route):
        if route.request.method != "PUT":
            return route.continue_()
        put.append(json.loads(route.request.post_data or "{}"))
        route.fulfill(status=200, content_type="application/json", body="{}")
    page.route(f"**/api/v1/users/{name}", user)
    page.evaluate(STUB_REGISTER)
    out, stop = writes(page)
    try:
        check_reset(page, "me-new", "#me-password-form .form-reset")
        if page.locator("#me-current").is_visible():
            page.locator("#me-current").fill(OPERATOR_PASSWORD)  # Reset empties the current one too
        page.locator("#me-again").fill(SHORT + "x")
        check_refusal(page, "me-new", "#me-password-msg")
        submit_profile(page, SHORT, SHORT + "x")
        frames(page)
        assert line(page, "me-new")["text"] == words(page, "password.refusedShort"), "differing and too short: too short first"
        assert page.locator("#me-password-msg").inner_text() == "", "and not the mismatch"
        assert page.evaluate("document.activeElement.id") == "me-new", "the focus in the new password"
        assert not out and page.evaluate("window.__registered") == [], f"nothing registered, nothing sent: {out}"
        submit_profile(page, FINE, FINE + "x")
        assert wait_until(page, "document.getElementById('me-password-msg').textContent === t('setup.mismatch')", 3), "then the mismatch"
        submit_profile(page, FINE, FINE)
        assert wait_until(page, "document.getElementById('me-password-msg').classList.contains('ok')", 10), "eight characters are saved"
        assert page.evaluate("window.__registered") == [FINE] and len(put) == 1 and put[0].get("record") == "r", f"registered and sent: {put}"
    finally:
        stop()
        page.evaluate(REAL_REGISTER)
        page.unroute(f"**/api/v1/users/{name}")


def test_my_profiles_set_a_password(page, case):
    """My profile of a user with keys only (the test's owner): Set a password."""
    assert page.evaluate("state.me.user && !state.me.user.has_password"), "the test's owner has keys only"
    show_view(page, "profile")
    check_profile(page, "owner")
    page.locator("#me-password-title").click()
    show_view(page, "tiles")


def test_my_profiles_change_password(new_context, case, watch, guard):
    """My profile of the operator, with a password: Change password, the
    current one typed."""
    page = fresh_page(new_context, guard, watch)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    show_view(page, "profile")
    if not page.locator("#me-password").evaluate("(d) => d.open"):
        page.locator("#me-password-title").click()
    page.locator("#me-current").fill(OPERATOR_PASSWORD)
    check_profile(page, "operator")


def test_adding_a_user(page, case):
    """Add a user: its password's line; a refusal by Enter, then by Create
    with the focus elsewhere, which puts it in the field; eight characters
    create the user, its creation the only one sent, so that the refusals
    before it sent nothing; an empty password is a user with keys only,
    not checked."""
    names = [f"rule-{case.width}-{case.lang}", f"rule-keys-{case.width}-{case.lang}"]
    for n in names:
        remove_user(n)
    show_view(page, "system")
    out, stop = writes(page)
    try:
        page.locator("#user-add").click()
        page.locator("#user-new-password").wait_for(state="visible")
        check_line(page, "user-new-password")
        page.locator("#user-new input[name=name]").fill(names[0])
        check_refusal(page, "user-new-password")
        page.locator("#user-new-password").fill(SHORT)
        page.locator("#user-new input[name=name]").focus()
        page.locator("#user-new button[type=submit]").click()
        frames(page)
        assert page.evaluate("document.activeElement.id") == "user-new-password", "the focus in the password"
        assert line(page, "user-new-password")["text"] == words(page, "password.refusedShort"), "the line says it was refused"
        page.locator("#user-new-password").fill(FINE)
        with page.expect_response(lambda r: r.request.method == "POST" and r.url.endswith("/api/v1/users")):
            page.locator("#user-new button[type=submit]").click()
        assert wait_until(page, "!document.querySelector('#user-new form')", 10), "eight characters create the user"
        assert [m for m, u in out if u.endswith("/api/v1/users")] == ["POST"] and len([u for _, u in out if u.endswith("/register")]) == 1, \
            f"one creation, one registration: the refusals sent nothing: {out}"
        user = api_call("GET", f"api/v1/users/{names[0]}")
        assert user["has_password"] and user["auth_version"] == 2, f"with its password: {user}"
        page.locator("#user-add").click()
        page.locator("#user-new input[name=name]").fill(names[1])
        page.locator("#user-new button[type=submit]").click()
        assert wait_until(page, "!document.querySelector('#user-new form')", 10), "an empty password: a user with keys only"
        assert not api_call("GET", f"api/v1/users/{names[1]}")["has_password"], "without a password"
    finally:
        stop()
        for n in names:
            remove_user(n)
    show_view(page, "tiles")


def test_a_users_panel(page, case):
    """A user's panel: its new password's line; a refusal by Enter, then by
    Save with the focus elsewhere, which puts it in the field; eight
    characters saved, the only PUT sent, so that the refusals before it
    sent nothing; an empty field leaves the password unchanged and is not
    checked."""
    name = f"rule-panel-{case.width}-{case.lang}"
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
        check_line(page, field)
        save = panel.locator("form button[type=submit]")
        check_reset(page, field, ".user-panel[data-name='%s'] form .form-reset" % name)
        check_refusal(page, field)
        page.locator("#" + field).fill(SHORT)
        panel.locator("input[name=display_name]").focus()
        save.click()
        frames(page)
        assert page.evaluate("document.activeElement.id") == field, "the focus in the password"
        page.locator("#" + field).fill(FINE)
        with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith(f"/users/{name}/tokens")):
            save.click()
        puts = [u for m, u in out if m == "PUT"]
        registered = [u for m, u in out if u.endswith("/register")]
        assert len(puts) == 1 and len(registered) == 1, f"one registration, one PUT: the refusals sent nothing: {out}"
        assert api_call("GET", f"api/v1/users/{name}")["auth_version"] == 2, "eight characters saved, registered by OPAQUE"
        page.locator("#" + field).fill("")
        panel.locator("input[name=display_name]").fill("Ruled")
        with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith(f"/users/{name}/tokens")):
            save.click()
        user = api_call("GET", f"api/v1/users/{name}")
        assert user["display_name"] == "Ruled" and user["has_password"], f"an empty field: unchanged, not checked: {user}"
    finally:
        stop()
        remove_user(name)
    show_view(page, "tiles")


def test_the_upgrade_at_the_sign_in_takes_a_short_password(page, case):
    """A version-1 user's upgrade (upgradePassword, login.js) registers the
    very password it is given, two characters too, and sends it: the page
    holds no rule there (D7-A). It is called as signIn calls it after a
    password sign-in that answers an upgrade, not through a sign-in: the
    server makes no version-1 user with a short password any more, and a
    sign-in by OPAQUE that is not finished holds the test's address for
    the other tests' sign-ins. So a rule placed in signIn, before it calls
    upgradePassword, would not be caught here; that the server takes the
    short password there is TestOpaqueUpgrade's. The PUT is stubbed, so
    that the user stays as it was. A forward guard."""
    name = f"rule-v1-{case.width}-{case.lang}"
    remove_user(name)
    api_call("POST", "api/v1/users", {"name": name, "role": "viewer", "password": "fixed-pw-01"})
    put = []

    def user(route):
        if route.request.method != "PUT":
            return route.continue_()
        put.append(json.loads(route.request.post_data or "{}"))
        route.fulfill(status=200, content_type="application/json", body="{}")
    page.route(f"**/api/v1/users/{name}", user)
    try:
        version = page.evaluate("""async ([name, token]) => {
          const session = { token, upgrade: "proof", user: { name, auth_version: 1 } };
          await upgradePassword(session, "ab");
          return session.user.auth_version;
        }""", [name, TOKEN])
        assert version == 2 and len(put) == 1 and put[0].get("record") and put[0].get("proof") == "proof", \
            f"the short password registered and sent: {version}, {put}"
    finally:
        page.unroute(f"**/api/v1/users/{name}")
        remove_user(name)
