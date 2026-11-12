# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The sign-in through an OpenID Connect provider (AP Oidc v1.0,
Block 4), against a provider the test serves on loopback, which the
server allows over http there: the owner adds the provider in System
settings → Sign-in providers and assigns an e-mail to a user in System
settings → Users; signed out, the card shows the provider's button,
which goes to the provider and back signed in as that user, the identity
then bound. Each case has a provider and a user of its own, removed at
the end."""
import base64
import hashlib
import json
import secrets
import threading
import time
import urllib.parse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest

from conftest import URL, api_call
from helpers import named_buttons, show_view, wait_until

CLIENT_ID = "massimo-ui-test"
SECRET = "ui-test-secret"


class FakeProvider:
    """A provider that signs in whoever comes, as the e-mail it was
    given, verified: discovery, an authorization endpoint that sends the
    browser straight back with a code, a token endpoint that answers the
    code with an id_token once the client and the PKCE verifier match."""

    def __init__(self, email):
        self.email = email
        self.grants = {}
        provider = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def answer(self, status, body, headers=()):
                data = json.dumps(body).encode()
                self.send_response(status)
                for k, v in headers:
                    self.send_header(k, v)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def do_GET(self):
                url = urllib.parse.urlsplit(self.path)
                if url.path == "/.well-known/openid-configuration":
                    self.answer(200, {"issuer": provider.issuer, "authorization_endpoint": provider.issuer + "/authorize",
                                      "token_endpoint": provider.issuer + "/token"})
                    return
                if url.path == "/authorize":
                    q = dict(urllib.parse.parse_qsl(url.query))
                    code = secrets.token_urlsafe(16)
                    provider.grants[code] = q
                    back = q["redirect_uri"] + "?" + urllib.parse.urlencode({"code": code, "state": q["state"]})
                    self.send_response(302)
                    self.send_header("Location", back)
                    self.end_headers()
                    return
                self.answer(404, {})

            def do_POST(self):
                form = dict(urllib.parse.parse_qsl(self.rfile.read(int(self.headers["Content-Length"])).decode()))
                auth = self.headers.get("Authorization", "")
                ok_client = auth == "Basic " + base64.b64encode(f"{CLIENT_ID}:{SECRET}".encode()).decode()
                q = provider.grants.pop(form.get("code", ""), None)
                challenge = base64.urlsafe_b64encode(hashlib.sha256(form.get("code_verifier", "").encode()).digest()).rstrip(b"=").decode()
                if not ok_client or not q or q["code_challenge"] != challenge or q["redirect_uri"] != form.get("redirect_uri"):
                    self.answer(400, {"error": "invalid_grant"})
                    return
                claims = {"iss": provider.issuer, "aud": CLIENT_ID, "sub": "sub-" + provider.email, "exp": int(time.time()) + 300,
                          "nonce": q["nonce"], "email": provider.email, "email_verified": True}
                payload = base64.urlsafe_b64encode(json.dumps(claims).encode()).rstrip(b"=").decode()
                self.answer(200, {"access_token": "at", "token_type": "Bearer", "id_token": "eyJhbGciOiJSUzI1NiJ9." + payload + ".c2ln"})

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.issuer = f"http://127.0.0.1:{self.server.server_address[1]}"
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def close(self):
        self.server.shutdown()


def back_at_page(url):
    """Whether a navigation is the page's again, back from the provider
    (the callback is a redirect, which the browser does not stop at)."""
    return url.startswith(URL.rstrip("/") + "/") and "/api/" not in url


@pytest.fixture
def provider(case):
    email = f"oidc-{case.width}-{case.lang}@example.com"
    p = FakeProvider(email)
    yield p
    p.close()


def test_sign_in_through_a_provider(page, case, new_context, provider):
    """The owner adds the provider and assigns the e-mail; signed out,
    its button signs the user in through it."""
    key = f"fake-{case.width}-{case.lang}"
    user = f"oidc-{case.width}-{case.lang}"
    api_call("POST", "api/v1/users", {"name": user, "role": "viewer"})
    try:
        show_view(page, "system")
        card = page.locator("#sys-providers")
        assert card.is_visible(), "the owner sees System settings → Sign-in providers"
        assert page.locator("#providers-callback").inner_text() == URL.rstrip("/") + "/api/v1/login/oidc/callback", \
            "the redirect URI to register"
        page.locator("#provider-add").click()
        form = page.locator("#provider-new form")
        form.locator("[name=key]").fill(key)
        form.locator("[name=name]").fill("Fake " + case.lang)
        form.locator("[name=issuer]").fill(provider.issuer)
        form.locator("[name=client_id]").fill(CLIENT_ID)
        form.locator("[name=client_secret]").fill(SECRET)
        assert not form.locator("[name=auto_role]").is_visible(), "the role shows only while it creates users"
        named_buttons(page, "the provider's form")
        form.locator("button[type=submit]").click()
        panel = page.locator(f".provider-panel[data-key='{key}']")
        panel.wait_for(state="visible")
        assert panel.get_attribute("open") is not None, "the provider added is open"
        assert SECRET not in page.content(), "the secret is never shown"

        # its user's identity, in System settings → Users
        page.goto(URL)  # the providers read again for the users' panels
        page.wait_for_selector(".room", timeout=10000)
        show_view(page, "system")
        user_panel = page.locator(f".user-panel[data-name='{user}']")
        user_panel.locator("summary").click()
        box = user_panel.locator(".user-identities-box")
        box.wait_for(state="visible")
        box.locator("select[name=provider]").select_option(key)
        box.locator("input[name=email]").fill(provider.email.upper())
        box.locator(".add-identity").click()
        assert wait_until(page, f"!!document.querySelector(\".user-panel[data-name='{user}'] .user-identities li[data-id]\")", 5), \
            "the identity is listed"
        row = page.locator(f".user-panel[data-name='{user}'] .user-identities li[data-id]")
        assert provider.email in row.inner_text(), "by its e-mail, lower case"

        # signed out: the card's button, the provider, back signed in
        ctx = new_context()
        try:
            other = ctx.new_page()
            other.set_default_timeout(5000)
            other.goto(URL)
            assert wait_until(other, "document.getElementById('login-dialog').open", 5), "the sign-in card"
            button = other.locator(f"#login-providers button[data-provider='{key}']")
            button.wait_for(state="visible")
            assert ("Fake " + case.lang) in button.inner_text(), "the button names the provider"
            with other.expect_navigation(url=back_at_page, timeout=10000):
                button.click()
            other.wait_for_selector(".room", timeout=10000)
            assert other.evaluate("state.me && state.me.user && state.me.user.name") == user, "signed in as the user"
            assert "#oidc" not in other.url, "the code is gone from the address"
        finally:
            ctx.close()
        identities = api_call("GET", f"api/v1/users/{user}/identities")
        assert len(identities) == 1 and identities[0]["bound"], f"the identity is bound: {identities}"
    finally:
        try:
            api_call("DELETE", f"api/v1/providers/{key}")
        except Exception:
            pass
        api_call("DELETE", f"api/v1/users/{user}")


def test_a_refused_account(new_context, case, guard, provider):
    """An account the server does not know comes back to the card, which
    says so."""
    key = f"nobody-{case.width}-{case.lang}"
    api_call("POST", "api/v1/providers", {"key": key, "name": "Nobody", "issuer": provider.issuer, "client_id": CLIENT_ID, "client_secret": SECRET})
    try:
        ctx = new_context()
        page = ctx.new_page()
        page.set_default_timeout(5000)
        page.goto(URL)
        assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in card"
        with page.expect_navigation(url=back_at_page, timeout=10000):
            page.locator(f"#login-providers button[data-provider='{key}']").click()
        assert wait_until(page, "!document.getElementById('login-error').classList.contains('hidden')", 10), "the card says why"
        assert page.evaluate("document.getElementById('login-error').textContent") == page.evaluate("t('login.oidcError.refused')"), \
            "the account is not known here"
        assert "#oidc" not in page.url, "the reason is gone from the address"
        ctx.close()
    finally:
        api_call("DELETE", f"api/v1/providers/{key}")
