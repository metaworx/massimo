# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The certificate under the header: a self-signed certificate reported
by /healthz (stubbed) brings a notice under the header, and in the
sign-in dialog, asking to compare the browser's fingerprint with the
server's, and shows none itself; the certificate is in the expert
details, one that ends within two weeks is a warning in the page's
language, and nothing shows without one. /healthz is stubbed with the
page's own version, so the page does not reload."""
import json

from conftest import URL
from helpers import wait_until

SELF_SIGNED = {"source": "self-signed", "names": ["massimo.example.net", "192.0.2.10"], "not_after": "2029-02-08T12:00:00Z",
               "fingerprint": "SHA256:AB:CD:EF:01", "self_signed": True}


def stub_health(page, tls):
    """/healthz answers with the page's own version and the given
    certificate, and the header is refreshed."""
    body = json.dumps({"ok": True, "version": page.evaluate("state.version"), "started_at": "2026-11-06T12:00:00Z", "tls": tls})
    page.route("**/healthz", lambda route: route.fulfill(status=200, content_type="application/json", body=body))
    page.locator("#refresh-btn").click()


def unstub_health(page):
    page.unroute("**/healthz")
    page.locator("#refresh-btn").click()
    assert wait_until(page, "document.getElementById('tls-notice').classList.contains('hidden')", 5), "notice gone without TLS"


def test_nothing_without_tls(page):
    assert page.locator("#tls-notice").is_hidden(), "no certificate notice without TLS"
    assert page.evaluate("document.getElementById('tls-cert').textContent") == "–", "no certificate in the expert details"


def test_self_signed_notice(page):
    """The notice asks for the comparison in the browser and shows no
    fingerprint: the page comes over the very connection in question."""
    stub_health(page, SELF_SIGNED)
    assert wait_until(page, "!document.getElementById('tls-notice').classList.contains('hidden')", 5), "self-signed notice"
    assert "massimo tls show" in page.locator("#tls-notice").inner_text(), "the notice names massimo tls show"
    assert "AB:CD:EF:01" not in page.locator("body").inner_text(), "no fingerprint in the page"
    cert = page.evaluate("document.getElementById('tls-cert').textContent")
    assert cert.startswith("self-signed · massimo.example.net, 192.0.2.10 · "), f"certificate in the expert details ({cert})"
    unstub_health(page)


EXPIRES = {"en": "expires in 5 days", "de": "läuft in 5 Tagen ab"}


def test_expiry_warning(page, case):
    """A certificate that ends within two weeks is a warning in the
    page's language, without a fingerprint."""
    import time

    soon = {"source": "acme", "names": ["massimo.example.net"], "fingerprint": "SHA256:AB:CD:EF:02", "self_signed": False,
            "not_after": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(time.time() + 5 * 86400 + 3600)),
            "warning": "the certificate expires in 5 days"}
    stub_health(page, soon)
    assert wait_until(page, "!document.getElementById('tls-notice').classList.contains('hidden')", 5), "expiry warning shown"
    assert EXPIRES[case.lang] in page.locator("#tls-notice-text").inner_text(), f"expiry warning in {case.lang}"
    unstub_health(page)


def fulfil_with(body):
    """A route handler answering with the body, as JSON."""
    return lambda route: route.fulfill(status=200, content_type="application/json", body=body)


def test_sign_in_shows_self_signed_notice(new_context, case, watch, guard):
    """Without a token the sign-in dialog opens over the page, and over
    the header's notice: with a self-signed certificate it carries the
    notice itself, before the password; without TLS it does not."""
    for tls, shown in ((SELF_SIGNED, True), (None, False)):
        ctx = new_context()  # the case's viewport and locale, without the stored token
        body = json.dumps({"ok": True, "version": "massimo v0.0.0-test", "started_at": "2026-11-06T12:00:00Z", "tls": tls})
        ctx.route("**/healthz", fulfil_with(body))
        page = ctx.new_page()
        guard.page = page
        page.set_default_timeout(5000)
        page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
        page.goto(URL)
        assert wait_until(page, "document.getElementById('login-dialog').open && state.version !== null", 5), \
            "the sign-in dialog opens and /healthz was read"
        notice = page.locator("#login-tls")
        if shown:
            assert wait_until(page, "!document.getElementById('login-tls').classList.contains('hidden')", 5), \
                "the sign-in dialog shows the self-signed notice"
            assert notice.is_visible(), "the notice is visible over the backdrop"
            assert "massimo tls show" in notice.inner_text(), "the notice names massimo tls show"
            assert "AB:CD:EF:01" not in page.locator("#login-dialog").inner_text(), "no fingerprint in the dialog"
        else:
            assert notice.is_hidden(), "no notice in the sign-in dialog without TLS"
        ctx.close()
