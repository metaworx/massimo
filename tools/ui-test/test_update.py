# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Settings → Software: the buttons while the update helper works, and
Update asking in the page. GET /api/v1/update is stubbed; nothing is
installed."""
from helpers import open_settings, update_json, wait_until


def test_busy_while_the_helper_installs(page):
    """While the update helper works, both buttons of the Software section
    are greyed out, also when the page only learns it from GET
    /api/v1/update (a reload during an update, or an update started from
    another browser), and come back once the helper reports done."""
    open_settings(page)
    page.route("**/api/v1/update*", lambda route: route.fulfill(
        status=200, content_type="application/json", body=update_json("installing", "v0.0.1-test", True)))
    page.locator("#update-check").click()
    assert wait_until(page, "document.getElementById('update-check').disabled && document.getElementById('update-btn').disabled", 5), \
        "Software buttons greyed out while the helper installs"
    page.unroute("**/api/v1/update*")
    page.route("**/api/v1/update*", lambda route: route.fulfill(
        status=200, content_type="application/json", body=update_json("done", "v0.0.1-test", False)))
    assert wait_until(page, "!document.getElementById('update-check').disabled && !document.getElementById('update-btn').disabled", 10), \
        "Software buttons back once the helper is done"
    page.unroute("**/api/v1/update*")


def test_update_asks_in_the_page(page):
    """Update asks in the Software section, not in a browser dialog, and
    Cancel takes the question back without a request."""
    open_settings(page)
    posts = []
    page.route("**/api/v1/update*", lambda route: posts.append(1) if route.request.method == "POST" else route.fulfill(
        status=200, content_type="application/json", body=update_json("up-to-date", "v0.0.1-test", True)))
    page.locator("#update-check").click()
    btn = page.locator("#update-btn")
    btn.wait_for(state="visible")
    btn.click()
    box = page.locator(".update .confirm-box")
    assert box.is_visible() and "v0.0.1-test" in box.inner_text() and btn.is_hidden(), "Update asks in the page"
    box.locator(".confirm-no").click()
    assert page.locator(".update .confirm-box").count() == 0 and btn.is_visible() and not posts, "Cancel takes it back, nothing sent"
    page.unroute("**/api/v1/update*")
