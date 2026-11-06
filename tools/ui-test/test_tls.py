# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The certificate under the header: a self-signed certificate reported
by /healthz (stubbed) shows its fingerprint under the header and the
certificate in the expert details, and nothing shows without one.
/healthz is stubbed with the page's own version, so the page does not
reload."""
import json

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
    stub_health(page, SELF_SIGNED)
    assert wait_until(page, "!document.getElementById('tls-notice').classList.contains('hidden') && "
                      "document.getElementById('tls-fingerprint').textContent === 'SHA256:AB:CD:EF:01'", 5), \
        "self-signed notice with the fingerprint"
    cert = page.evaluate("document.getElementById('tls-cert').textContent")
    assert cert.startswith("self-signed · massimo.example.net, 192.0.2.10 · "), f"certificate in the expert details ({cert})"
    unstub_health(page)
