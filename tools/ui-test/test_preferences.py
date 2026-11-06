# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The settings that follow the user (docs/ui.md, "Settings"): the
token's user, the owner, keeps them on the server, and a fresh browser
signed in with the same token takes them; a setting kept for this device
only stays in its browser; a guest (run.sh creates guest1, password
guest-pass) has no switches and keeps its settings in the browser. The
page fixture removes the owner's preferences before every page."""
import json
import urllib.request

from conftest import TOKEN, URL, clear_preferences
from helpers import wait_until


def server_ui():
    """The owner's ui preference on the server, None when there is none."""
    req = urllib.request.Request(URL.rstrip("/") + "/api/v1/me/preferences", headers={"Authorization": "Bearer " + TOKEN})
    with urllib.request.urlopen(req, timeout=5) as resp:
        return json.loads(resp.read()).get("ui")


def wait_server(page, check, seconds=5):
    """Waits until the server's ui satisfies check, the page's writes
    being debounced by a second."""
    end = seconds * 5
    for _ in range(end):
        ui = server_ui()
        if ui is not None and check(ui):
            return ui
        page.wait_for_timeout(200)
    return server_ui()


def fresh(new_context):
    """A browser context of its own with the token and nothing else."""
    ctx = new_context()
    ctx.add_init_script(f"localStorage.setItem('massimo.token', {TOKEN!r})")
    page = ctx.new_page()
    page.set_default_timeout(5000)
    page.goto(URL)
    page.wait_for_selector(".room", timeout=10000)
    return page


def test_settings_follow_the_user(page, new_context, case):
    """The poll and the scheme set in one browser are on the server and
    in a fresh one; the mode kept for this device only is not."""
    assert wait_until(page, "!document.body.classList.contains('no-sync')", 5), "the owner's settings follow the owner"
    ui = wait_server(page, lambda ui: ui.get("poll") == 10)
    assert ui is not None and ui["poll"] == 10, "the first page hands its settings to the server"
    page.locator("#view-settings").click()
    switches = page.locator("input[data-device-only]")
    assert switches.count() == 6, "a switch under each of the six settings"
    assert switches.first.get_attribute("aria-label"), "the switch is named for its row"
    page.locator("#poll-input").fill("30")
    page.locator("#poll-input").dispatch_event("change")
    page.locator("#scheme-select").select_option("contrast")
    page.locator("input[data-device-only=theme]").check(force=True)
    page.locator("#theme-select").select_option("dark")
    ui = wait_server(page, lambda ui: ui.get("poll") == 30 and ui.get("scheme") == "contrast")
    assert ui["poll"] == 30 and ui["scheme"] == "contrast", "the poll and the scheme are on the server"
    assert ui["theme"] == "system", "the mode kept for this device stays off the server"
    assert json.loads(page.evaluate("localStorage.getItem('massimo.device')")) == {"theme": "dark"}, "this device's mode in the browser"

    other = fresh(new_context)
    assert wait_until(other, "state.settings.poll === 30 && state.settings.scheme === 'contrast'", 5), "a fresh browser takes the user's settings"
    assert other.evaluate("state.settings.theme") == "system", "but not the other device's mode"
    assert other.evaluate("localStorage.getItem('massimo.scheme')") == "contrast", "the browser keeps the user's values as its cache"
    other.context.close()

    # Off again, the user's mode applies on this device too.
    page.locator("input[data-device-only=theme]").uncheck(force=True)
    assert wait_until(page, "state.settings.theme === 'system'", 5), "the user's mode applies again"
    assert page.evaluate("localStorage.getItem('massimo.device')") is None, "nothing kept for this device"


def test_guest_keeps_settings_in_the_browser(new_context, case, watch, guard):
    """A guest signs in, has no switches and no note, and a change of its
    poll stays in the browser."""
    clear_preferences()
    ctx = new_context()
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in dialog"
    page.locator("#login-user").fill("guest1")
    page.locator("#login-password").fill("guest-pass")
    page.locator("#login-form button[type=submit]").click()
    page.wait_for_selector(".room", timeout=10000)
    assert page.evaluate("document.body.classList.contains('no-sync')"), "a guest's settings do not follow it"
    page.locator("#view-settings").click()
    assert not page.locator("input[data-device-only=poll]").is_visible(), "no switches for a guest"
    page.locator("#poll-input").fill("45")
    page.locator("#poll-input").dispatch_event("change")
    page.wait_for_timeout(1500)
    assert page.evaluate("localStorage.getItem('massimo.poll')") == "45", "the guest's poll in the browser"
    assert server_ui() is None, "nothing reached the server"
