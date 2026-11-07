# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The sign-in by OPAQUE in the browser, against the Go server: a user
whose password the page registered with the vendored client
(ui/vendor/opaque.js) signs in through the dialog, and no request
carries the password. Each case has a user of its own, removed at the
end; the console is not watched, since the page's first requests without
a token are refused on purpose."""
import time

from conftest import TOKEN, URL, api_call
from helpers import wait_until

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
        finishes = [i for i, (u, _) in enumerate(posts) if u.endswith("/api/v1/login/finish")]
        assert len(finishes) == 2, f"a finish per sign-in, not {len(finishes)}"
        right = [u for u, _ in posts[finishes[1] + 1:] if u.endswith("/api/v1/login")]
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
    in Settings → Users, and the user changes its own in Settings with a
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
        page.locator("#view-settings").click()
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
        assert wait_until(page, "!document.getElementById('users-msg').classList.contains('hidden') && document.getElementById('users-msg').classList.contains('ok')", 10), "the password is set"
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
        own.locator("#view-settings").click()
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
