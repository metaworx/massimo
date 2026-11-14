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

from conftest import TOKEN, URL, api_call, clear_preferences
from helpers import show_view, sign_out, wait_until


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
    show_view(page, "prefs")
    switches = page.locator("input[data-device-only]")
    assert switches.count() == 7, "a switch under each of the seven settings"
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
    show_view(page, "prefs")
    assert not page.locator("input[data-device-only=poll]").is_visible(), "no switches for a guest"
    page.locator("#poll-input").fill("45")
    page.locator("#poll-input").dispatch_event("change")
    page.wait_for_timeout(1500)
    assert page.evaluate("localStorage.getItem('massimo.poll')") == "45", "the guest's poll in the browser"
    assert server_ui() is None, "nothing reached the server"


def operator_key():
    """An API key of the operator run.sh creates, for the test's own
    calls, and its id: the operator's password is of auth_version 2, which
    signs in by OPAQUE in the browser and not by POST /api/v1/login."""
    key = api_call("POST", "api/v1/users/operator/tokens", {"label": "ui-test preferences"})
    return key["token"], key["id"]


def clear_user(token):
    """A user's preferences removed, by its own token."""
    for key in ("ui", "schemes"):
        api_call("DELETE", "api/v1/me/preferences/" + key, token=token)


def test_log_out_leaves_nothing_for_the_next_user(new_context, case, watch, guard):
    """Log out sends a change still waiting under the user's own token,
    then the browser forgets the user's settings and schemes: the next to
    sign in, the owner with its key, neither sees them nor gets them into
    its account. What is kept for this device only stays."""
    op, op_id = operator_key()
    try:
        clear_user(op)
        clear_preferences()
        ctx = new_context()
        page = ctx.new_page()
        guard.page = page
        page.set_default_timeout(5000)
        page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
        page.goto(URL)
        assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in dialog"
        page.locator("#login-user").fill("operator")
        page.locator("#login-password").fill("op-pass")
        page.locator("#login-form button[type=submit]").click()
        page.wait_for_selector(".room", timeout=10000)
        assert wait_until(page, "!document.body.classList.contains('no-sync')", 5), "the operator's settings follow it"
        show_view(page, "prefs")
        page.locator("input[data-device-only=clock]").check(force=True)
        page.locator("#poll-input").fill("37")
        page.locator("#poll-input").dispatch_event("change")
        page.evaluate("putCustomScheme('Operator only', schemeById('massimo').light, schemeById('massimo').dark)")
        sign_out(page)  # within the second the change waits
        assert wait_until(page, "document.getElementById('login-dialog').open", 5), "Sign out brings the dialog back"
        assert page.evaluate("localStorage.getItem('massimo.poll')") is None, "the operator's poll is forgotten"
        assert page.evaluate("localStorage.getItem('massimo.schemes')") is None, "the operator's schemes are forgotten"
        assert json.loads(page.evaluate("localStorage.getItem('massimo.device')") or "{}") == {"clock": "browser"}, \
            "what is kept for this device stays"
        page.locator("details.login-token summary").click()
        page.locator("#login-token").fill(TOKEN)
        page.locator("#login-key-form button[type=submit]").click()
        page.wait_for_selector(".room", timeout=10000)
        assert wait_until(page, "!document.body.classList.contains('no-sync')", 5), "the owner's settings follow it"
        page.wait_for_timeout(2000)  # the page's writes, a second after a change
        owner = api_call("GET", "api/v1/me/preferences") or {}
        assert (owner.get("ui") or {}).get("poll") != 37, "the operator's poll did not reach the owner"
        assert not any(s.get("name") == "Operator only" for s in (owner.get("schemes") or {}).get("schemes") or []), \
            "the operator's scheme did not reach the owner"
        assert page.evaluate("state.settings.poll") != 37 and not page.evaluate("themeState.custom.some((c) => c.name === 'Operator only')"), \
            "the owner does not see them"
        mine = api_call("GET", "api/v1/me/preferences", token=op) or {}
        assert (mine.get("ui") or {}).get("poll") == 37, "the change made just before Log out is the operator's on the server"
        assert any(s.get("name") == "Operator only" for s in (mine.get("schemes") or {}).get("schemes") or []), \
            "and so is the scheme"
    finally:
        clear_user(op)
        api_call("DELETE", "api/v1/users/operator/tokens/%d" % op_id)
        clear_preferences()


def server_schemes():
    """The names of the owner's schemes on the server, None for none."""
    doc = (api_call("GET", "api/v1/me/preferences") or {}).get("schemes")
    return None if doc is None else [s.get("name") for s in doc.get("schemes") or []]


def test_a_deleted_scheme_stays_deleted(page, new_context, case):
    """Once the server has the user's list of schemes it is the list: a
    scheme deleted in one browser does not come back from another that
    still has it, at that browser's next load."""
    try:
        page.evaluate("putCustomScheme('Shared', schemeById('massimo').light, schemeById('massimo').dark)")
        for _ in range(25):
            if "Shared" in (server_schemes() or []):
                break
            page.wait_for_timeout(200)
        assert "Shared" in (server_schemes() or []), "the scheme reaches the server"
        other = fresh(new_context)
        assert wait_until(other, "themeState.custom.some((c) => c.name === 'Shared')", 5), "another browser takes it"
        page.evaluate("themeState.custom = themeState.custom.filter((c) => c.name !== 'Shared'); storeCustomSchemes();")
        for _ in range(25):
            if server_schemes() == []:
                break
            page.wait_for_timeout(200)
        assert server_schemes() == [], "the deletion reaches the server"
        other.reload()
        other.wait_for_selector(".room", timeout=10000)
        other.wait_for_timeout(2000)  # a push of the old list would have come by now
        assert server_schemes() == [], "the other browser does not bring it back"
        assert not other.evaluate("themeState.custom.some((c) => c.name === 'Shared')"), "and drops it itself"
        other.context.close()
    finally:
        clear_preferences()


def test_device_column(page, case):
    """My settings: every "only on this device" switch in one column at
    the right end of its row, headed at the top, its tooltip naming its
    setting; the page at most 640 px wide."""
    assert wait_until(page, "!document.body.classList.contains('no-sync')", 5), "the owner's settings follow the owner"
    show_view(page, "prefs")
    cells = page.evaluate("""[...document.querySelectorAll('#prefs .dev-row > .dev-cell')]
        .map((c) => [Math.round(c.getBoundingClientRect().left), Math.round(c.getBoundingClientRect().right)])""")
    assert len(cells) == 8 and len({c[0] for c in cells}) == 1, f"one column, a cell per row ({cells})"
    head = page.evaluate("document.querySelector('#prefs .dev-colhead').getBoundingClientRect().left")
    assert round(head) == cells[0][0], "the column headed at the top"
    body = page.evaluate("document.querySelector('#prefs .settings-body').getBoundingClientRect().right "
                         "- parseFloat(getComputedStyle(document.querySelector('#prefs .settings-body')).paddingRight)")
    assert abs(cells[0][1] - body) <= 1, "at the right end of the rows"
    for sw in page.locator("#prefs input[data-device-only]").all():
        title = sw.evaluate("(i) => i.closest('label').title")
        assert title and title == sw.get_attribute("aria-label"), f"the tooltip names the setting ({title!r})"
    assert page.evaluate("document.getElementById('prefs').getBoundingClientRect().width") <= 640, "My settings at most 640 px"
    show_view(page, "tiles")
