# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The sign-in as parts (AP UiParts, Block 14): what a user holds on the
first owner's form and on the sign-in's card stays through everything
that draws them again, and each button that sends sends once. The app's
side runs on fake_bridge.js, as in test_connect.py; the console is not
watched on a page without a token, whose first requests are refused on
purpose (test_login.py)."""
import json

from conftest import TOKEN
from helpers import wait_until
from test_connect import calls, server_page

OPEN_SETUP = json.dumps({"mode": "open", "user": None, "role": None, "capabilities": [], "setup": True})


def test_create_the_owner_stays_busy_while_the_app_keeps(new_context, case, watch, guard):
    """In the app, Create the owner with the switch on stays busy until
    the app has answered whether it kept the login by fingerprint (UiParts
    D213): a second tap while the system's prompt is open creates nothing,
    where it posted the owner again and said the name was taken. The
    second submit is the form's own (requestSubmit, as Enter on another
    form's field would be): Enter on a field whose default button is off
    never submits, so a key press would try the button's disabled
    attribute and not the part's guard. A second creation would be sent
    at once, the creation being stubbed: the posts are counted once the
    first has ended. The owner's creation and sign-in are stubbed (POST
    /api/v1/users, and the page's registerPassword and signIn), so that
    the test server keeps its users; /api/v1/me says an open server
    without users."""
    page = server_page(new_context, guard, watch, {"status": {"can": True, "user": "", "ask": "start", "plain": False}}, token=TOKEN)
    page.wait_for_selector(".room", timeout=10000)
    posts = []

    def users(route):
        if route.request.method != "POST":
            return route.continue_()
        posts.append(route.request.url)
        route.fulfill(status=201, content_type="application/json", body="{}")

    page.route("**/api/v1/users", users)
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=OPEN_SETUP))
    try:
        page.evaluate("""() => {
          registerPassword = async () => ({ record: "r", ksf: "m=19456,t=2,p=1" });
          signIn = async () => ({ token: %s });
          const login = window.massimo.login;
          login.keep = () => new Promise((resolve) => { window.__letKeepGo = () => resolve(true); });
          loadMe();
        }""" % json.dumps(TOKEN))
        page.locator("#setup-keep-row").wait_for(state="visible")
        page.locator("#setup-user").fill(f"owner-{case.width}-{case.lang}")
        page.locator("#setup-password").fill("pw")
        page.locator("#setup-again").fill("pw")
        page.locator("#setup-keep-row .slider").click()
        submit = page.locator("#setup-form button[type=submit]")
        submit.click()
        assert wait_until(page, "typeof window.__letKeepGo === 'function'", 5), "the app is asked to keep the login"
        assert len(posts) == 1, f"the owner is created once: {posts}"
        assert submit.evaluate("(b) => b.classList.contains('busy') && b.disabled"), "Create the owner turns while the app keeps the login"
        page.evaluate("() => { document.getElementById('setup-form').requestSubmit(); }")
        page.evaluate("() => { window.__letKeepGo(); }")
        assert wait_until(page, "!document.getElementById('setup-msg').classList.contains('hidden') && "
                                "!document.querySelector('#setup-form button[type=submit]').classList.contains('busy')", 5), \
            "the form says how it went, and Create the owner stops turning"
        assert len(posts) == 1, f"a second submit while the app keeps the login creates nothing: {posts}"
        assert not page.evaluate("document.getElementById('setup-msg').classList.contains('bad')"), \
            page.evaluate("document.getElementById('setup-msg').textContent")
    finally:
        page.unroute("**/api/v1/me")
        page.unroute("**/api/v1/users")
