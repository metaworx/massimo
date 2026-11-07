# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The warning under the header while the server is open. GET
/api/v1/me is stubbed to say so; the test server itself asks for a
key."""
import json

from helpers import wait_until

SHOWN = "!document.getElementById('open-banner').classList.contains('hidden')"


def stub_me(page, mode):
    """GET /api/v1/me answers as a server in the given mode would, to a
    caller that is not on the local network (no first-owner form)."""
    body = json.dumps({"mode": mode, "user": None, "role": None, "capabilities": [], "setup": False})
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=body))


def test_open_server_banner(page):
    """No banner while the server asks who is there; on an open server a
    banner under the header's rule says so and how to close it, with its
    own text for a server without a store; it goes once the server asks
    again."""
    banner = page.locator("#open-banner")
    assert banner.is_hidden(), "no banner on a server with users"
    stub_me(page, "open")
    page.evaluate("loadMe()")
    assert wait_until(page, SHOWN, 5), "the banner on an open server"
    text = banner.inner_text()
    assert page.evaluate("t('open.title')") in text and page.evaluate("t('open.text')") in text, "it says the server is open and what to do"
    header = page.locator("header.top").bounding_box()
    assert banner.bounding_box()["y"] >= header["y"] + header["height"] - 1, "under the header's rule"
    page.unroute("**/api/v1/me")
    stub_me(page, "none")
    page.evaluate("loadMe()")
    assert wait_until(page, "document.getElementById('open-banner-text').textContent === t('open.textNoStore')", 5), \
        "the text for a server without a store"
    page.unroute("**/api/v1/me")
    page.evaluate("loadMe()")
    assert wait_until(page, "document.getElementById('open-banner').classList.contains('hidden')", 5), "gone once the server asks again"
