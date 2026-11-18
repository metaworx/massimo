# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The apps' connect page, ui/connect.html (docs/app.md), with the fake
bridge of fake_bridge.js in place of an app's adapter, put into the page
by add_init_script as conftest.py puts the token: the server list and
what each probe found, a tap that opens a server, adding by address and
from discovery, the fingerprint sheet of a self-signed certificate and
of a changed one, a server's settings with when the fingerprint is
asked, the login by fingerprint from the row, Turn off and Forget all,
the card of the server on the phone; a browser without the bridge; and
the server's page taking its token from the bridge and handing it back,
its login by fingerprint and "Select server" (docs/app.md, contract
version 2).
axe-core (axe-playwright-python) finds no violation on the connect page
in each of its states the tests open.

Each test opens a browser context of its own with the case's viewport
and locale (new_context, as test_login.py does); the guard fails it on a
page error, and on a console error on the connect page, which sends
nothing to the server."""
import json
import os

from axe_playwright_python.sync_playwright import Axe

from conftest import TOKEN, URL, clear_preferences
from helpers import named_buttons, open_personal_page, wait_until

CONNECT = URL + "ui/connect.html"
FAKE_BRIDGE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "fake_bridge.js")

# The certificates' fingerprints, as a server writes them; made up, and
# put together here so that no file holds one.
FP1 = "SHA256:" + ":".join(["A1"] * 32)
FP2 = "SHA256:" + ":".join(["B2"] * 32)
NODE1 = "11111111-1111-4111-8111-111111111111"
NODE2 = "22222222-2222-4222-8222-222222222222"
NODE3 = "33333333-3333-4333-8333-333333333333"

HOME_LAN = "http://192.168.1.10:8099/"
HOME_REMOTE = "https://home.example.net/"
CABIN_LAN = "http://192.168.1.40:8099/"
OFFICE_REMOTE = "https://office.example.org/"

STATE = {"en": {"lan": "on the LAN", "remote": "remote", "offline": "not reachable", "cert": "certificate changed"},
         "de": {"lan": "im LAN", "remote": "von außen", "offline": "nicht erreichbar", "cert": "Zertifikat geändert"}}
NO_BRIDGE = {"en": "This page belongs to the Massimo app", "de": "Diese Seite gehört zur Massimo-App"}
FP_TITLE = {"en": "Check the certificate", "de": "Zertifikat prüfen"}
FP_CHANGED = {"en": "The certificate has changed", "de": "Das Zertifikat hat sich geändert"}


def server(sid, name, node, lan="", remote="", fp="", login="", ask="start"):
    return {"id": sid, "name": name, "node": node, "lan": lan, "remote": remote, "fp": fp, "login": login, "ask": ask, "push": False}


def open_connect(new_context, guard, watch, config=None):
    """The connect page in a context of its own, with the fake bridge
    answering from config, or without a bridge for None."""
    ctx = new_context()
    if config is not None:
        ctx.add_init_script(f"window.__fakeBridgeConfig = {json.dumps(config)};")
        ctx.add_init_script(path=FAKE_BRIDGE)
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("console", lambda m: watch.problems.append(f"console: {m.text}") if m.type == "error" else None)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(CONNECT)
    if config is not None:
        page.locator("#servers").wait_for(state="visible")
    return page


def calls(page, name=None):
    """The bridge calls the page made, or those of one name."""
    got = page.evaluate("window.__fakeBridge.calls")
    return [c for c in got if name is None or c[0] == name]


def row(page, name):
    return page.locator(".server-row", has=page.locator(".server-name", has_text=name))


def wait_state(page, name, text):
    """Waits until the server's row shows what its probe found."""
    badge = row(page, name).locator(".server-badges .flag").first
    badge.wait_for()
    end = 50
    while end and badge.inner_text() != text:
        page.wait_for_timeout(100)
        end -= 1
    assert badge.inner_text() == text, f"{name} shows {text!r}, not {badge.inner_text()!r}"


def assert_axe(page, where):
    """axe-core finds no violation on the page as it stands."""
    results = Axe().run(page)
    assert results.violations_count == 0, f"axe on {where}:\n{results.generate_report()}"


def test_connect_without_bridge(new_context, case, watch, guard):
    """A browser without the app's bridge sees a line saying the page is
    the app's, in its language, with a link to the web UI at the
    server's root, and none of the app's sections."""
    page = open_connect(new_context, guard, watch)
    note = page.locator("#no-bridge")
    note.wait_for(state="visible")
    assert wait_until(page, f"document.getElementById('no-bridge').textContent.includes({NO_BRIDGE[case.lang]!r})", 5), \
        "the line is in the page's language"
    assert page.locator("#no-bridge a").evaluate("(a) => a.href") == URL, "the link opens the web UI at the server's root"
    for section in ("#servers", "#add", "#core-card"):
        assert not page.locator(section).is_visible(), f"{section} is the app's only"
    assert_axe(page, "the page without the bridge")


def test_connect_list_and_open(new_context, case, watch, guard):
    """The servers show what their probe found: at the LAN address when
    its node answers there, else at the remote one, else not reachable;
    a tap opens a server at the address found, and one that does not
    answer is probed again and said to be unreachable. Without core in
    the bridge there is no card for the server on the phone."""
    config = {
        "servers": [server("s1", "Home", NODE1, lan=HOME_LAN, remote=HOME_REMOTE),
                    server("s2", "Cabin", NODE2, lan=CABIN_LAN),
                    server("s3", "Office", NODE3, lan="http://192.168.5.2:8099/", remote=OFFICE_REMOTE)],
        "answers": {HOME_LAN: {"node": NODE1}, HOME_REMOTE: {"node": NODE1, "fingerprint": FP1},
                    "http://192.168.5.2:8099/": {"node": NODE1}, OFFICE_REMOTE: {"node": NODE3, "fingerprint": FP2}},
    }
    page = open_connect(new_context, guard, watch, config)
    st = STATE[case.lang]
    wait_state(page, "Home", st["lan"])
    wait_state(page, "Cabin", st["offline"])
    wait_state(page, "Office", st["remote"])  # its LAN address answers as another server
    assert row(page, "Office").locator(".server-where").inner_text() == "office.example.org", "the address found is shown"
    probes = calls(page, "probe")
    assert ["probe", [HOME_LAN, HOME_REMOTE], ""] in probes, f"both addresses probed, the LAN one first: {probes}"
    assert not page.locator("#core-card").is_visible(), "no server on the phone without core"
    named_buttons(page, "the connect page")
    assert_axe(page, "the server list")

    row(page, "Home").locator(".server-open").click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'open')", 3), "a tap opens the server"
    assert calls(page, "open") == [["open", "s1", HOME_LAN]], "at the address its probe found"

    before = len(calls(page, "probe"))
    row(page, "Cabin").locator(".server-open").click()
    msg = page.locator("#servers-msg")
    msg.wait_for(state="visible")
    assert "Cabin" in msg.inner_text(), "a server that does not answer is said to be unreachable"
    assert len(calls(page, "probe")) > before, "and probed again first"
    assert len(calls(page, "open")) == 1, "it is not opened"


def test_connect_add_by_address(new_context, case, watch, guard):
    """An address typed is probed at the URLs it may mean, HTTPS first,
    and added at the first that answers, as a LAN server for a private
    address; what is not an address says so, and a plain-HTTP URL
    outside the home network is warned of."""
    lan = "http://192.168.1.20:8099/"
    page = open_connect(new_context, guard, watch, {"servers": [], "answers": {lan: {"node": NODE2}}})
    assert page.locator("#servers-empty").is_visible(), "an empty list says how to add one"
    address = page.locator("#add-address")
    address.fill("not an address!")
    page.locator("#add-form button[type=submit]").click()
    page.locator("#add-msg.bad").wait_for()

    address.fill("http://203.0.113.5/")
    assert page.locator("#add-warn").is_visible(), "plain HTTP outside the home network is warned of"
    address.fill("192.168.1.20:8099")
    assert not page.locator("#add-warn").is_visible(), "a private address is not"
    page.locator("#add-form button[type=submit]").click()
    page.locator("#add-msg.ok").wait_for()
    assert ["probe", ["https://192.168.1.20:8099/", lan], ""] in calls(page, "probe"), "HTTPS is tried first"
    saved = calls(page, "save")[-1][1]
    assert saved["lan"] == lan and saved["remote"] == "" and saved["node"] == NODE2, f"added as a LAN server: {saved}"
    assert address.input_value() == "", "the address is cleared"
    wait_state(page, "192.168.1.20:8099", STATE[case.lang]["lan"])
    assert not page.locator("#servers-empty").is_visible()


def test_connect_fingerprint_sheet(new_context, case, watch, guard):
    """A self-signed certificate opens the fingerprint sheet: Cancel adds
    nothing; Trust probes with the fingerprint, adds the server and pins
    it. When the server's certificate changes, the row says so, and a tap
    shows the old and the new fingerprint, and opens the server once the
    new one is trusted."""
    config = {"servers": [], "answers": {HOME_REMOTE: {"node": NODE1, "fingerprint": FP1, "self_signed": True}}}
    page = open_connect(new_context, guard, watch, config)
    dlg = page.locator("#fp-dialog")
    page.locator("#add-address").fill("home.example.net")
    page.locator("#add-form button[type=submit]").click()
    dlg.wait_for(state="visible")
    assert page.locator("#fp-title").inner_text() == FP_TITLE[case.lang]
    assert page.locator("#fp-new").inner_text() == FP1, "the sheet shows the server's fingerprint"
    assert not page.locator("#fp-old-row").is_visible(), "and no earlier one"
    named_buttons(page, "the fingerprint sheet")
    assert_axe(page, "the fingerprint sheet")
    page.locator("#fp-cancel").click()
    page.locator("#add-msg.bad").wait_for()
    assert calls(page, "save") == [], "Cancel adds nothing"

    page.locator("#add-form button[type=submit]").click()
    dlg.wait_for(state="visible")
    page.locator("#fp-trust").click()
    page.locator("#add-msg.ok").wait_for()
    assert ["probe", [HOME_REMOTE], FP1] in calls(page, "probe"), "probed again with the fingerprint trusted"
    sid = page.evaluate("window.__fakeBridge.servers[0].id")
    assert calls(page, "trust") == [["trust", sid, FP1]], "the fingerprint is pinned"
    assert calls(page, "save")[-1][1]["remote"] == HOME_REMOTE, "a public name is the remote address"
    wait_state(page, "home.example.net", STATE[case.lang]["remote"])

    page.evaluate(f"window.__fakeBridgeConfig.answers[{HOME_REMOTE!r}].fingerprint = {FP2!r}")
    page.locator("#probe-btn").click()
    wait_state(page, "home.example.net", STATE[case.lang]["cert"])
    row(page, "home.example.net").locator(".server-open").click()
    dlg.wait_for(state="visible")
    assert page.locator("#fp-title").inner_text() == FP_CHANGED[case.lang]
    assert page.locator("#fp-old").inner_text() == FP1 and page.locator("#fp-new").inner_text() == FP2, \
        "the old and the new fingerprint"
    assert_axe(page, "the changed certificate's sheet")
    page.locator("#fp-trust").click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'open')", 3), "the server opens once trusted"
    assert calls(page, "trust")[-1] == ["trust", sid, FP2]
    assert calls(page, "open") == [["open", sid, HOME_REMOTE]]


def test_connect_settings(new_context, case, watch, guard):
    """A server's settings open under its row from the cog and from a
    long press (the context menu): when the fingerprint is asked is
    saved, the notification switch is there where the app has push, a
    server without a kept login offers no Turn off, and Remove asks in
    place and forgets the server."""
    config = {"servers": [server("s1", "Home", NODE1, lan=HOME_LAN)], "answers": {HOME_LAN: {"node": NODE1}}, "push": True}
    page = open_connect(new_context, guard, watch, config)
    wait_state(page, "Home", STATE[case.lang]["lan"])
    home = row(page, "Home")
    home.locator(".server-open").click(button="right")
    panel = home.locator(".server-panel")
    panel.wait_for(state="visible")
    assert home.locator(".server-cog").get_attribute("aria-expanded") == "true"
    home.locator(".server-cog").click()
    panel.wait_for(state="detached")
    home.locator(".server-cog").click()
    panel.wait_for(state="visible")
    assert panel.locator("input[name=push]").count() == 1, "the notification switch, the app having push"
    assert panel.locator("input").first.input_value() == "Home"
    named_buttons(page, "a server's settings")
    assert_axe(page, "a server's settings")

    assert panel.locator("select[name=ask]").input_value() == "start", "when the app starts, until the user chooses"
    assert panel.locator(".server-login-off").count() == 0 and home.locator(".server-login").is_hidden(), "no kept login, no Turn off"
    panel.locator("select[name=ask]").select_option("return")
    panel.locator("button[type=submit]").click()
    panel.wait_for(state="detached")
    saved = calls(page, "save")[-1][1]
    assert saved["ask"] == "return" and saved["id"] == "s1" and saved["lan"] == HOME_LAN and "biometric" not in saved, f"the choice is saved: {saved}"
    page.locator("#servers-msg.ok").wait_for()

    home.locator(".server-cog").click()
    assert panel.locator("select[name=ask]").input_value() == "return", "and shown as saved"
    panel.locator(".server-remove").click()
    page.locator(".confirm-box .confirm-yes").click()
    home.wait_for(state="detached")
    assert calls(page, "remove") == [["remove", "s1"]]
    assert page.locator("#servers-empty").is_visible()


def test_connect_discovery(new_context, case, watch, guard):
    """A search lists the servers it finds that are not in the list, and
    one is added from there; a saved server found at another LAN address
    than the one that did not answer takes that address."""
    moved = "http://192.168.1.11:8099/"
    found = "http://192.168.1.30:8099/"
    config = {
        "servers": [server("s1", "Home", NODE1, lan=HOME_LAN)],
        "answers": {moved: {"node": NODE1}, found: {"node": NODE3}},
        "found": [{"name": "Home", "host": "192.168.1.11", "port": 8099, "node": NODE1, "version": "v0.0.0-test", "tls": False, "fp": ""},
                  {"name": "massimo-box", "host": "192.168.1.30", "port": 8099, "node": NODE3, "version": "v0.0.0-test", "tls": False, "fp": ""}],
    }
    page = open_connect(new_context, guard, watch, config)
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'discover')", 3), "the page searches at its start"
    assert calls(page, "discover") == [["discover", 5000]]
    box = page.locator(".found-server", has_text="massimo-box")
    box.wait_for()
    assert page.locator(".found-server").count() == 1, "a saved server is not offered again"
    assert box.locator(".found-where").inner_text() == "192.168.1.30:8099"
    wait_state(page, "Home", STATE[case.lang]["lan"])
    assert page.evaluate("window.__fakeBridge.servers[0].lan") == moved, "the LAN address that changed is taken"
    assert row(page, "Home").locator(".server-where").inner_text() == "192.168.1.11:8099"

    box.locator(".found-add").click()
    box.wait_for(state="detached")
    saved = calls(page, "save")[-1][1]
    assert saved["name"] == "massimo-box" and saved["lan"] == found and saved["node"] == NODE3, f"added from the search: {saved}"
    wait_state(page, "massimo-box", STATE[case.lang]["lan"])
    assert wait_until(page, "!document.getElementById('search-btn').disabled", 3), "the search ends"
    page.locator("#search-btn").click()
    assert wait_until(page, "window.__fakeBridge.calls.filter((c) => c[0] === 'discover').length === 2", 3), "Search again"


def test_connect_core_card(new_context, case, watch, guard):
    """Where the app has the server built in, its card says whether it
    runs, and Start and Stop are each on only where they do something."""
    config = {"servers": [], "core": {"running": False, "address": "http://127.0.0.1:8099/", "version": "v0.0.0-test"}}
    page = open_connect(new_context, guard, watch, config)
    card = page.locator("#core-card")
    card.wait_for(state="visible")
    status = page.locator("#core-status")
    assert wait_until(page, "document.getElementById('core-status').textContent !== ''", 3)
    assert page.locator("#core-stop").is_disabled() and not page.locator("#core-start").is_disabled()
    stopped = status.inner_text()
    page.locator("#core-start").click()
    assert wait_until(page, "document.getElementById('core-start').disabled && !document.getElementById('core-start').classList.contains('busy')", 3)
    assert "127.0.0.1:8099" in status.inner_text(), "running, at its address"
    assert not page.locator("#core-stop").is_disabled()
    page.locator("#core-stop").click()
    assert wait_until(page, "document.getElementById('core-stop').disabled && !document.getElementById('core-stop').classList.contains('busy')", 3)
    assert status.inner_text() == stopped
    assert [c[0] for c in calls(page) if c[0].startswith("core.")] == ["core.status", "core.start", "core.stop"]
    assert_axe(page, "the server on the phone")


def test_server_page_token_from_bridge(new_context, case, watch, guard):
    """In the app, the server's page takes its token from the bridge and
    keeps none in the browser: signed in with the app's token, Forget key
    hands the app null, a sign-in hands it the new session, and Sign out
    null again. The console is not watched: the page's requests without a
    token are refused on purpose, as in test_login.py."""
    clear_preferences()
    ctx = new_context()
    ctx.add_init_script(f"window.__fakeBridgeConfig = {json.dumps({'kind': 'server', 'token': TOKEN})};")
    ctx.add_init_script(path=FAKE_BRIDGE)
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    page.wait_for_selector(".room", timeout=10000)
    assert page.evaluate("state.token") == TOKEN, "the page has the app's token"
    assert page.evaluate("localStorage.getItem('massimo.token')") is None, "and the browser keeps none"
    assert calls(page, "token") == [["token"]], "asked once, at the start"

    open_personal_page(page)
    page.locator("#logout-btn").click()
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "Forget key asks who is there"
    assert calls(page, "setToken") == [["setToken", None]], "and the app forgets the key"

    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("op-pass")
    page.locator("#login-form button[type=submit]").click()
    # The rooms are still drawn behind the dialog: the session is waited for.
    assert wait_until(page, "window.__fakeBridge.calls.filter((c) => c[0] === 'setToken').length >= 2", 10), "the sign-in hands the app its session"
    page.wait_for_selector(".room", timeout=10000)
    sets = calls(page, "setToken")
    assert len(sets) == 2 and isinstance(sets[1][1], str) and sets[1][1] not in ("", TOKEN), f"the session goes to the app: {sets}"
    assert page.evaluate("state.token") == sets[1][1]
    assert page.evaluate("localStorage.getItem('massimo.token')") is None, "and not to the browser"

    open_personal_page(page)
    page.locator("#logout-btn").click()
    assert wait_until(page, "document.getElementById('login-dialog').open", 5)
    assert calls(page, "setToken")[-1] == ["setToken", None], "Sign out forgets the session in the app"


def test_connect_login_by_fingerprint(new_context, case, watch, guard):
    """A server with a kept login has the fingerprint on its row, labelled
    with the user, which opens it signed in (open with {login: true}); a
    dismissed prompt leaves the list as it was. Its settings name the
    user with Turn off, which forgets it at the address the probe chose;
    Forget all stored credentials asks in place and forgets every one."""
    config = {
        "servers": [server("s1", "Home", NODE1, lan=HOME_LAN, login="anna"), server("s2", "Cabin", NODE2, lan=CABIN_LAN, login="bob"),
                    server("s3", "Office", NODE3, remote=OFFICE_REMOTE)],
        "answers": {HOME_LAN: {"node": NODE1}, CABIN_LAN: {"node": NODE2}},
    }
    page = open_connect(new_context, guard, watch, config)
    wait_state(page, "Home", STATE[case.lang]["lan"])
    home, cabin, office = row(page, "Home"), row(page, "Cabin"), row(page, "Office")
    button = home.locator(".server-login")
    assert button.is_visible() and "anna" in (button.get_attribute("aria-label") or ""), "the row's fingerprint names the user"
    assert office.locator(".server-login").is_hidden(), "a server without a kept login has none"
    assert page.locator("#forget-all").is_visible(), "Forget all where a login is kept"
    named_buttons(page, "the list with kept logins")
    assert_axe(page, "the list with kept logins")

    button.click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'open')", 3)
    assert calls(page, "open")[-1] == ["open", "s1", HOME_LAN, {"login": True}], f"opened signed in: {calls(page, 'open')}"

    # Turn off: the fingerprint once, the key revoked at the probed address.
    home.locator(".server-cog").click()
    panel = home.locator(".server-panel")
    panel.wait_for(state="visible")
    assert "anna" in panel.locator(".server-login-kept").inner_text()
    panel.locator(".server-login-off").click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'forget')", 3)
    assert calls(page, "forget") == [["forget", "s1", HOME_LAN]]
    page.locator("#servers-msg.ok").wait_for()
    assert wait_until(page, "!document.querySelector(\".server-row[data-id='s1'] .server-login-kept\")", 3), "the settings drawn again without it"
    assert home.locator(".server-login").is_hidden(), "and the row without its fingerprint"

    # Forget all: asked in place, then every login forgotten.
    assert cabin.locator(".server-login").is_visible()
    page.locator("#forget-all").click()
    page.locator(".confirm-box .confirm-yes").click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'forgetAll')", 3)
    # the question stands in the row's place until the app answered
    assert wait_until(page, "!!document.getElementById('forget-all-row') && document.getElementById('forget-all-row').classList.contains('hidden')", 3), \
        "no login left, no Forget all"
    assert cabin.locator(".server-login").is_hidden()


def test_connect_login_refused(new_context, case, watch, guard):
    """A kept key the server refused is forgotten by the app: the open
    says why, and the row loses its fingerprint."""
    config = {"servers": [server("s1", "Home", NODE1, lan=HOME_LAN, login="anna")], "answers": {HOME_LAN: {"node": NODE1}},
              "openLogin": "error:the server no longer takes the kept key"}
    page = open_connect(new_context, guard, watch, config)
    wait_state(page, "Home", STATE[case.lang]["lan"])
    row(page, "Home").locator(".server-login").click()
    page.locator("#servers-msg:not(.hidden)").wait_for()
    assert "no longer takes" in page.locator("#servers-msg").inner_text()
    assert wait_until(page, "document.querySelector(\".server-row[data-id='s1'] .server-login\").classList.contains('hidden')", 3)


def server_page(new_context, guard, watch, login, token=None):
    """The server's page in the app, its bridge with the login by
    fingerprint as login describes it (fake_bridge.js)."""
    clear_preferences()
    ctx = new_context()
    ctx.add_init_script(f"window.__fakeBridgeConfig = {json.dumps({'kind': 'server', 'token': token, 'login': login})};")
    ctx.add_init_script(path=FAKE_BRIDGE)
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL)
    return page


def test_server_page_keeps_the_login(new_context, case, watch, guard):
    """On the sign-in form of a server's page in the app: the switch where
    the phone can and nothing is kept; a sign-in with it on asks the app
    to keep the login, a prompt not passed offers Try again and Sign in
    without it, and the second try keeps it; Select server goes back to
    the app's servers. The console is not watched: the page's requests
    without a token are refused on purpose, as in test_login.py."""
    page = server_page(new_context, guard, watch, {"status": {"can": True, "user": "", "ask": "start", "plain": False}, "keep": [False, True]})
    page.locator("#login-dialog[open]").wait_for()
    page.locator("#login-keep-row").wait_for(state="visible")
    assert page.locator("#login-fingerprint").is_hidden(), "nothing kept: no Login by fingerprint"
    assert page.locator("#login-servers").is_visible(), "Select server, the app having its servers"
    named_buttons(page, "the sign-in in the app")
    page.locator("#login-servers").click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'connect')", 3), "Select server calls the app"

    page.locator("#login-user").fill("operator")
    page.locator("#login-password").fill("op-pass")
    page.locator("#login-keep-row .slider").click()
    page.locator("#login-form button[type=submit]").click()
    page.locator("#login-keep-failed").wait_for(state="visible")
    assert calls(page, "login.keep") == [["login.keep"]], "the app is asked to keep it, once"
    sets = calls(page, "setToken")
    assert len(sets) == 1 and isinstance(sets[0][1], str), f"the session went to the app first: {sets}"
    assert page.locator("#login-form").is_hidden() and page.locator("#login-keep-retry").is_visible()
    named_buttons(page, "the login not kept")
    page.locator("#login-keep-retry").click()
    page.wait_for_selector(".room", timeout=10000)
    assert len(calls(page, "login.keep")) == 2, "Try again asks again"
    assert not page.evaluate("document.getElementById('login-dialog').open"), "and the page is signed in"

    # Signed in: the Cube choice ends with Select server.
    page.locator("#cube-menu summary").click()
    entry = page.locator("#cube-menu .menu-items button[data-value='__servers']")
    entry.wait_for(state="visible")
    entry.click()
    assert wait_until(page, "window.__fakeBridge.calls.filter((c) => c[0] === 'connect').length === 2", 3), "the Cube choice's Select server"


def test_server_page_login_by_fingerprint(new_context, case, watch, guard):
    """Where a key is kept, the sign-in form offers Login by fingerprint
    with the user's name, which signs the page in with the session the
    app made; a dismissed prompt leaves the form; over plain HTTP outside
    the home network the form says why it offers the password alone."""
    page = server_page(new_context, guard, watch, {"status": {"can": True, "user": "operator", "ask": "start", "plain": False}, "fingerprint": TOKEN})
    page.locator("#login-dialog[open]").wait_for()
    fingerprint = page.locator("#login-fingerprint")
    fingerprint.wait_for(state="visible")
    assert "operator" in page.locator("#login-fingerprint-user").inner_text()
    assert page.locator("#login-keep-row").is_hidden(), "a key kept: no switch"
    fingerprint.click()
    page.wait_for_selector(".room", timeout=10000)
    assert calls(page, "login.fingerprint") == [["login.fingerprint"]]
    assert page.evaluate("state.token") == TOKEN, "signed in with the app's session"
    assert page.evaluate("localStorage.getItem('massimo.token')") is None, "and the browser keeps none"

    plain = server_page(new_context, guard, watch, {"status": {"can": True, "user": "operator", "ask": "start", "plain": True}})
    plain.locator("#login-dialog[open]").wait_for()
    plain.locator("#login-fp-plain").wait_for(state="visible")
    assert plain.locator("#login-fingerprint").is_hidden() and plain.locator("#login-keep-row").is_hidden(), "plain HTTP: neither"

    dismissed = server_page(new_context, guard, watch, {"status": {"can": True, "user": "operator", "ask": "start", "plain": False}, "fingerprint": None})
    dismissed.locator("#login-fingerprint").wait_for(state="visible")
    dismissed.locator("#login-fingerprint").click()
    assert wait_until(dismissed, "window.__fakeBridge.calls.some((c) => c[0] === 'login.fingerprint')", 3)
    assert wait_until(dismissed, "!document.getElementById('login-fingerprint').classList.contains('busy')", 3)
    assert dismissed.evaluate("document.getElementById('login-dialog').open"), "a dismissed prompt leaves the form"
