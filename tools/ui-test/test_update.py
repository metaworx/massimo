# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings → Software: the update channel, the buttons while the
update helper works, and Update asking in the page. GET /api/v1/update is
stubbed, and the setting update.channel where the channel is chosen;
nothing is installed."""
import json

from helpers import show_view, update_json, wait_until


def test_busy_while_the_helper_installs(page):
    """While the update helper works, both buttons of the Software section
    are greyed out, also when the page only learns it from GET
    /api/v1/update (a reload during an update, or an update started from
    another browser), and come back once the helper reports done."""
    show_view(page, "system")
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
    show_view(page, "system")
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


def test_update_through_apt(page):
    """On a box the Debian package installed, Software names apt beside
    the version, and Update asks without promising the old version back:
    apt puts nothing back."""
    show_view(page, "system")
    page.route("**/api/v1/update*", lambda route: route.fulfill(
        status=200, content_type="application/json", body=update_json("up-to-date", "v0.0.1-test", True, "apt")))
    page.locator("#update-check").click()
    assert wait_until(page, "document.getElementById('update-text').textContent.includes('apt')", 5), \
        "Software names apt"
    btn = page.locator("#update-btn")
    btn.wait_for(state="visible")
    plain = page.evaluate("t('update.confirm', {version: 'v0.0.1-test'})")
    btn.click()
    box = page.locator(".update .confirm-box")
    assert box.is_visible() and "apt" in box.inner_text() and plain not in box.inner_text(), \
        "Update asks about apt, not with the hand install's rollback"
    box.locator(".confirm-no").click()
    page.unroute("**/api/v1/update*")


def channel_routes(page, manager=None, variable=None, suite=None):
    """Stubs GET /api/v1/update and the setting update.channel, the build's
    flavour alpha and no channel stored; variable is the channel
    MASSIMO_UPDATE_CHANNEL sets, suite the one apt follows. Returns the
    bodies of the settings PUTs."""
    state = {"stored": None}
    puts = []

    def effect():
        return variable or state["stored"] or suite or "alpha"

    def update(route):
        route.fulfill(status=200, content_type="application/json",
                      body=update_json("up-to-date", "v0.0.1-test", False, manager, effect(), suite))

    def settings(route):
        if route.request.method == "PUT":
            body = json.loads(route.request.post_data)
            puts.append(body)
            state["stored"] = body.get("update.channel")
            route.fulfill(status=200, content_type="application/json", body="{}")
            return
        view = {"value": state["stored"] or "alpha", "default": "alpha", "set": state["stored"] is not None,
                "in_effect": effect(), "applies": "now"}
        route.fulfill(status=200, content_type="application/json", body=json.dumps({"update.channel": view}))

    page.route("**/api/v1/update*", update)
    page.route("**/api/v1/settings", settings)
    return puts


def unroute_channel(page):
    page.unroute("**/api/v1/update*")
    page.unroute("**/api/v1/settings")


def test_channel_is_chosen(page):
    """A holder of settings chooses the update channel under Software: the
    choice is stored as update.channel, the build's flavour as null, and
    the release check asks again."""
    puts = channel_routes(page)
    show_view(page, "system")
    sel = page.locator("#update-channel")
    sel.wait_for(state="visible")
    assert sel.is_enabled() and sel.input_value() == "", "the build's flavour is chosen, and can be changed"
    assert sel.locator("option").count() == 7, "the build's flavour and six channels"
    sel.select_option("beta")
    assert wait_until(page, "document.getElementById('update-text').textContent.includes('beta')", 5), \
        "the release check follows beta"
    assert puts == [{"update.channel": "beta"}], "beta is stored"
    assert sel.input_value() == "beta" and sel.is_enabled(), "beta stays chosen"
    sel.select_option("")
    assert wait_until(page, "!document.getElementById('update-text').textContent.includes('beta')", 5), \
        "the release check is back on the build's flavour"
    assert puts[-1] == {"update.channel": None}, "the build's flavour is stored as null"
    unroute_channel(page)


def test_channel_on_apt(page):
    """On a box apt updates the box follows apt's suite until a channel is
    chosen, which the next update switches apt to: the note says so."""
    puts = channel_routes(page, manager="apt", suite="devel")
    show_view(page, "system")
    sel = page.locator("#update-channel")
    sel.wait_for(state="visible")
    first = page.evaluate("t('update.channelSuite', {suite: 'devel'})")
    note = page.locator("#update-channel-note")
    assert sel.is_enabled() and sel.input_value() == "", "apt's suite is followed, and can be changed"
    assert sel.locator("option").first.inner_text() == first, "the first choice names apt's suite"
    assert note.inner_text() == page.evaluate("t('update.channelApt', {channel: 'devel'})"), "the note names the suite"
    sel.select_option("beta")
    switch = page.evaluate("t('update.channelAptSwitch', {suite: 'devel', channel: 'beta'})")
    assert wait_until(page, "document.getElementById('update-channel-note').textContent === %s" % json.dumps(switch), 5), \
        "the note says the next update switches apt to beta"
    assert puts == [{"update.channel": "beta"}], "beta is stored"
    unroute_channel(page)


def test_channel_fixed_by_the_variable(page):
    """MASSIMO_UPDATE_CHANNEL overrides the setting: the channel it sets is
    shown, fixed, and the note names the variable."""
    channel_routes(page, variable="nightly")
    show_view(page, "system")
    sel = page.locator("#update-channel")
    sel.wait_for(state="visible")
    assert wait_until(page, "document.getElementById('update-channel').disabled", 5), "the channel is fixed"
    assert sel.input_value() == "nightly", "the variable's channel is shown"
    assert "MASSIMO_UPDATE_CHANNEL" in page.locator("#update-channel-note").inner_text(), "the note names the variable"
    unroute_channel(page)
