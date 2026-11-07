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
