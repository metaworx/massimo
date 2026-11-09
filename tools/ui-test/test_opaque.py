# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The sign-in by OPAQUE in the browser, against the Go server: a user
whose password the page registered with the vendored client
(ui/vendor/opaque.js) signs in through the dialog, and no request
carries the password. Each case has a user of its own, removed at the
end; the console is not watched, since the page's first requests without
a token are refused on purpose."""
import json
import time

from conftest import TOKEN, URL, api_call
from helpers import show_view, wait_until

REGISTER = """async ([name, password, token, method]) => {
  const opaque = await opaqueModule();
  const reg = opaque.client.startRegistration({ password });
  const headers = { "Content-Type": "application/json", Authorization: "Bearer " + token };
  let r = await fetch("api/v1/users/" + name + "/register", { method: "POST", headers, body: JSON.stringify({ request: reg.registrationRequest }) });
  const answer = await r.json();
  const done = opaque.client.finishRegistration({
    clientRegistrationState: reg.clientRegistrationState, registrationResponse: answer.response, password,
    keyStretching: keyStretching(answer.ksf),
  });
  const body = { record: done.registrationRecord, ksf: answer.ksf };
  if (method === "POST") Object.assign(body, { name, role: "operator" });
  r = await fetch(method === "POST" ? "api/v1/users" : "api/v1/users/" + name, { method, headers, body: JSON.stringify(body) });
  return [r.status, await r.json()];
}"""


def register(page, name, password, method="POST"):
    """A password of version 2 for a name, registered in the page with
    its OPAQUE client and the test token: the user created with it
    (POST), or given it (PUT). The status and the user."""
    return page.evaluate(REGISTER, [name, password, TOKEN, method])


def test_sign_in_by_opaque(new_context, case, watch, guard):
    """A user of version 2 signs in through the dialog: the session is
    the user's, the password goes into no request and the password
    sign-in is not used; a wrong password is refused in the dialog."""
    name = f"opaque-{case.width}-{case.lang}"
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in dialog opens without a token"
    status, user = register(page, name, "opaque-pass")
    try:
        assert status == 201 and user["auth_version"] == 2 and user["has_password"], f"registered: {status} {user}"
        posts = []
        page.on("request", lambda r: posts.append((r.url, r.post_data or "")) if r.method == "POST" else None)
        page.locator("#login-user").fill(name)
        page.locator("#login-password").fill("wrong-pass")
        page.locator("#login-form button[type=submit]").click()
        assert wait_until(page, "!document.getElementById('login-error').classList.contains('hidden')", 10), "a wrong password is refused"
        page.locator("#login-password").fill("opaque-pass")
        began = time.monotonic()
        page.locator("#login-form button[type=submit]").click()
        page.wait_for_selector(".room", timeout=10000)
        took = time.monotonic() - began
        assert page.evaluate("state.me.user.name") == name and page.evaluate("state.me.user.auth_version") == 2, "signed in as the user"
        signed = [u for u, body in posts if "opaque-pass" in body]
        assert not signed, f"the password went to the server: {signed}"
        # The wrong password: a start, then the password sign-in that takes
        # it over while the server takes that; the right one: a start and
        # its finish.
        finishes = [i for i, (u, _) in enumerate(posts) if u.endswith("/api/v1/login/finish")]
        assert len(finishes) == 1, f"one finish, the right password's, not {len(finishes)}"
        right = [u for u, _ in posts[finishes[0] + 1:] if u.endswith("/api/v1/login")]
        assert not right, "the right password needs no password sign-in"
        print(f"\n{case.width}-{case.lang}: the sign-in by OPAQUE took {took:.2f} s in the browser")
    finally:
        api_call("DELETE", f"api/v1/users/{name}")


def sign_in_in(page, name, password):
    """A sign-in by the page's own client (login.js), no dialog: whether
    the password opens the user's record."""
    return page.evaluate("async ([n, p]) => { try { return !!(await signIn(n, p)).token; } catch (e) { return false; } }", [name, password])


def test_passwords_registered_in_the_browser(page, case, new_context):
    """Every password set in the page is registered by OPAQUE and none
    goes to the server: the owner adds a user with one and sets another,
    in System settings → Users, and the user changes its own in My profile with a
    proof of the current one."""
    name = f"reg-{case.width}-{case.lang}"
    posts = []
    record = lambda r: posts.append((r.url, r.post_data or "")) if r.method in ("POST", "PUT") else None
    page.on("request", record)
    # The sign-ins that check a password run in a page of their own, whose
    # refused requests on purpose the guard does not watch.
    ctx = new_context()
    probe = ctx.new_page()
    probe.on("request", record)
    probe.goto(URL)
    try:
        show_view(page, "system")
        page.locator("#user-add").click()
        new = page.locator("#user-new")
        new.locator("[name=name]").fill(name)
        new.locator("[name=password]").fill("first-pass")
        new.locator("button[type=submit]").click()
        assert wait_until(page, f"!!document.querySelector(\".user-panel[data-name='{name}']\")", 10), "the user is added"
        user = api_call("GET", f"api/v1/users/{name}")
        assert user["auth_version"] == 2 and user["has_password"], f"added with a record: {user}"
        panel = page.locator(f".user-panel[data-name='{name}']")
        panel.locator("summary").click()
        panel.locator("[name=password]").fill("second-pass")
        panel.locator(".user-form button[type=submit]").first.click()
        # The add's "<name> created." is ok too: the save's own words.
        saved = page.evaluate("(name) => t('users.saved', { name })", name)
        assert wait_until(page, f"document.getElementById('users-msg').textContent === {json.dumps(saved)} && document.getElementById('users-msg').classList.contains('ok')", 10), \
            "the password is set"
        assert sign_in_in(probe, name, "second-pass") and not sign_in_in(probe, name, "first-pass"), "the second password replaced the first"

        own = ctx.new_page()
        own.set_default_timeout(5000)
        own.on("request", record)
        own.goto(URL)
        assert wait_until(own, "document.getElementById('login-dialog').open", 5), "the sign-in dialog opens"
        own.locator("#login-user").fill(name)
        own.locator("#login-password").fill("second-pass")
        own.locator("#login-form button[type=submit]").click()
        own.wait_for_selector(".room", timeout=10000)
        show_view(own, "profile")
        own.locator("#me-password summary").click()
        own.locator("#me-current").fill("second-pass")
        own.locator("#me-new").fill("third-pass")
        own.locator("#me-again").fill("third-pass")
        own.locator("#me-password-form button[type=submit]").click()
        assert wait_until(own, "document.getElementById('me-password-msg').classList.contains('ok')", 10), "one's own password is changed"
        assert any(u.endswith("/api/v1/login/start") and '"purpose":"proof"' in body for u, body in posts), "the current password is proven by a sign-in"
        assert sign_in_in(probe, name, "third-pass") and not sign_in_in(probe, name, "second-pass"), "the own change took"
        # The two old passwords tried on purpose fall back to the password
        # sign-in while the server takes it; nothing else sends one.
        fallback = [u for u, _ in posts if u.endswith("/api/v1/login")]
        assert len(fallback) == 2, f"a password sign-in beside the two old passwords tried: {fallback}"
        sent = [u for u, body in posts if not u.endswith("/api/v1/login") and any(p in body for p in ("first-pass", "second-pass", "third-pass"))]
        assert not sent, f"a password went to the server: {sent}"
    finally:
        ctx.close()
        api_call("DELETE", f"api/v1/users/{name}")


def test_a_record_of_the_command_line_opens_in_the_browser(new_context, case, guard):
    """run.sh adds the operator with massimo user add, which registers
    both halves in Go with the browser's key stretching: the page's
    client opens that record, without the password sign-in."""
    user = api_call("GET", "api/v1/users/operator")
    assert user["auth_version"] == 2, f"massimo user add registered by OPAQUE: {user}"
    ctx = new_context()
    probe = ctx.new_page()
    posts = []
    probe.on("request", lambda r: posts.append(r.url) if r.method == "POST" else None)
    probe.goto(URL)
    try:
        assert sign_in_in(probe, "operator", "op-pass"), "the browser opens the record made in Go"
        assert not [u for u in posts if u.endswith("/api/v1/login")], "without the password sign-in"
    finally:
        ctx.close()


def test_a_user_of_version_1_is_upgraded(new_context, case, guard):
    """A user whose password the server hashes (set through the API)
    signs in through the dialog: by the password, once, after the start
    by OPAQUE, and is upgraded at once; the next sign-in is by OPAQUE
    alone."""
    name = f"old-{case.width}-{case.lang}"
    api_call("POST", "api/v1/users", {"name": name, "role": "viewer", "password": "old-pass"})
    ctx = new_context()
    try:
        page = ctx.new_page()
        guard.page = page
        page.set_default_timeout(5000)
        posts = []
        page.on("request", lambda r: posts.append((r.method, r.url, r.post_data or "")) if r.method in ("POST", "PUT") else None)
        page.goto(URL)
        assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in dialog opens"
        page.locator("#login-user").fill(name)
        page.locator("#login-password").fill("old-pass")
        page.locator("#login-form button[type=submit]").click()
        page.wait_for_selector(".room", timeout=10000)
        user = api_call("GET", f"api/v1/users/{name}")
        assert user["auth_version"] == 2 and user["has_password"], f"upgraded: {user}"
        crossed = [u for _, u, body in posts if "old-pass" in body]
        assert len(crossed) == 1 and crossed[0].endswith("/api/v1/login"), f"the password crosses once, at the upgrade: {crossed}"
        assert any(m == "PUT" and u.endswith(f"/api/v1/users/{name}") and '"record"' in body for m, u, body in posts), "the record is sent"
        posts.clear()
        assert sign_in_in(page, name, "old-pass"), "the next sign-in"
        assert not [u for _, u, _ in posts if u.endswith("/api/v1/login")], "by OPAQUE alone"
    finally:
        ctx.close()
        api_call("DELETE", f"api/v1/users/{name}")


def test_the_password_sign_in_switched_off(page, case, new_context):
    """System settings → Users has the switch of the password sign-in and says
    how many users still have a password the server checks; off, such a
    user is refused in the dialog and one of version 2 signs in."""
    name = f"v1-{case.width}-{case.lang}"
    api_call("POST", "api/v1/users", {"name": name, "role": "viewer", "password": "v1-pass"})
    try:
        show_view(page, "system")
        box = page.locator("#password-login")
        assert wait_until(page, "document.getElementById('password-login').checked", 5), "the password sign-in is on by default"
        assert wait_until(page, "/\\d/.test(document.getElementById('password-login-hint').textContent)", 5), "the hint counts the users of version 1"
        box.uncheck(force=True)
        assert wait_until(page, "document.getElementById('users-msg').classList.contains('ok')", 5), "the switch is saved"
        assert api_call("GET", "api/v1/settings")["auth.password_login"]["value"] == "false", "the server's setting is off"
        ctx = new_context()
        probe = ctx.new_page()
        probe.goto(URL)
        refused = probe.evaluate("async ([n, p]) => { try { await signIn(n, p); return false; } catch (e) { return true; } }", [name, "v1-pass"])
        assert refused, "a user of version 1 is refused with the switch off"
        assert sign_in_in(probe, "operator", "op-pass"), "a user of version 2 signs in"
        ctx.close()
    finally:
        api_call("PUT", "api/v1/settings", {"auth.password_login": None})
        api_call("DELETE", f"api/v1/users/{name}")
