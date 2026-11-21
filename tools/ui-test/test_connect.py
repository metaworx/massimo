# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The apps' connect page, ui/connect.html (docs/app.md), with the fake
bridge of fake_bridge.js in place of an app's adapter, put into the page
by add_init_script as conftest.py puts the token: the server list and
what each probe found, a tap that opens a server, adding by address and
from discovery in the Add card, shown for an empty list or from the
list's ☰, the names a server is shown by, the toast saying what an
action came to, the fingerprint sheet of a self-signed certificate and
of a changed one, a server's settings with Open at start and when the
fingerprint is asked, the panel taking the focus, the login by
fingerprint from the row, Turn off and Forget all, the sessions the app
holds with Sign out and Sign out of all, a server's certificate, App
settings with the appearance, the card of the server on the phone; a
browser without the bridge; and the server's page taking its token from
the bridge and handing it back, its login by fingerprint and "Select
server" (docs/app.md, contract version 3).
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
from helpers import assert_login_card_centred, assert_login_row, named_buttons, open_personal_page, wait_until

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


def server(sid, name, node, lan="", remote="", fp="", login="", ask="start", start=False, session=None, trusted_at=""):
    return {"id": sid, "name": name, "node": node, "lan": lan, "remote": remote, "fp": fp, "login": login, "ask": ask, "push": False,
            "start": start, "session": session, "trusted_at": trusted_at}


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


def wait_toast(page, text="", bad=False):
    """Waits until the page's toast says text, with the red edge of a
    failure or without it."""
    js = ("(() => { const el = document.getElementById('toast');"
          f" return !el.classList.contains('hidden') && el.classList.contains('bad') === {json.dumps(bad)}"
          f" && el.textContent.includes({json.dumps(text)}); }})()")
    assert wait_until(page, js, 5), f"the toast says {text!r}, bad {bad}: {page.locator('#toast').inner_text()!r}"


def toast_gone(page):
    """Waits until the page's toast is hidden."""
    return wait_until(page, "document.getElementById('toast').classList.contains('hidden')", 5)


def assert_axe(page, where):
    """axe-core finds no violation on the page as it stands."""
    results = Axe().run(page)
    assert results.violations_count == 0, f"axe on {where}:\n{results.generate_report()}"


def menu_entry(page, entry):
    """Opens the Servers card's ☰ and gives its entry of that id, shown."""
    page.locator("#servers-menu > summary").click()
    item = page.locator(f"#servers-menu #{entry}")
    item.wait_for(state="visible")
    return item


def open_add(page):
    """Shows the Add card from the ☰ (Add server…)."""
    menu_entry(page, "menu-add").click()
    page.locator("#add").wait_for(state="visible")


def discovers(page):
    """How many searches the page asked for."""
    return len(calls(page, "discover"))


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
    for section in ("#servers", "#add", "#app-card", "#core-card"):
        assert not page.locator(section).is_visible(), f"{section} is the app's only"
    assert_axe(page, "the page without the bridge")


def test_connect_list_and_open(new_context, case, watch, guard):
    """The servers show what their probe found: at the LAN address when
    its node answers there, else at the remote one, else not reachable;
    a tap opens a server at the address found, and one that does not
    answer is probed again and said to be unreachable in the toast,
    which a later probe that finds it takes back. Without core in the
    bridge there is no card for the server on the phone."""
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
    assert page.locator("#add").is_hidden(), "a list that is not empty has no Add card until the ☰ asks"
    named_buttons(page, "the connect page")
    assert_axe(page, "the server list")

    row(page, "Home").locator(".server-open").click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'open')", 3), "a tap opens the server"
    assert calls(page, "open") == [["open", "s1", HOME_LAN]], "at the address its probe found"

    before = len(calls(page, "probe"))
    row(page, "Cabin").locator(".server-open").click()
    wait_toast(page, "Cabin", bad=True)  # a server that does not answer is said to be unreachable
    assert len(calls(page, "probe")) > before, "and probed again first"
    assert len(calls(page, "open")) == 1, "it is not opened"
    named_buttons(page, "the toast")
    assert_axe(page, "the toast")

    page.evaluate(f"window.__fakeBridgeConfig.answers[{CABIN_LAN!r}] = {{node: {NODE2!r}}}")
    page.locator("#probe-btn").click()
    wait_state(page, "Cabin", st["lan"])
    assert toast_gone(page), "a probe that finds the server takes the toast back"


def test_connect_add_by_address(new_context, case, watch, guard):
    """An address typed is probed at the URLs it may mean, HTTPS first,
    and added at the first that answers, as a LAN server for a private
    address, which the toast says; what is not an address says so under
    the field, and a plain-HTTP URL outside the home network is warned
    of."""
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
    wait_toast(page, "192.168.1.20:8099")
    assert page.locator("#add-msg").is_hidden(), "the line under the field is taken back"
    assert ["probe", ["https://192.168.1.20:8099/", lan], ""] in calls(page, "probe"), "HTTPS is tried first"
    saved = calls(page, "save")[-1][1]
    assert saved["lan"] == lan and saved["remote"] == "" and saved["node"] == NODE2, f"added as a LAN server: {saved}"
    assert saved["start"] is True, "the first server added opens at start (AP MobileApp, D43-A)"
    assert address.input_value() == "", "the address is cleared"
    wait_state(page, "192.168.1.20:8099", STATE[case.lang]["lan"])
    assert not page.locator("#servers-empty").is_visible()
    assert page.locator("#add").is_hidden(), "the Add card closes once a server is added"
    assert page.evaluate("document.activeElement.classList.contains('server-open')"), "and the new server's row takes the focus"


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
    wait_toast(page, bad=True)
    assert calls(page, "save") == [], "Cancel adds nothing"

    page.locator("#add-form button[type=submit]").click()
    dlg.wait_for(state="visible")
    page.locator("#fp-trust").click()
    wait_toast(page, "home.example.net")
    assert ["probe", [HOME_REMOTE], FP1] in calls(page, "probe"), "probed again with the fingerprint trusted"
    sid = page.evaluate("window.__fakeBridge.servers[0].id")
    assert calls(page, "trust") == [["trust", sid, FP1]], "the fingerprint is pinned"
    assert page.locator("#add").is_hidden(), "the Add card closes once the server is added"
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
    long press (the context menu), the panel taking the focus and not
    its name's field, which would open a phone's keyboard (AP MobileApp,
    F17): when the fingerprint is asked is saved, Open at start is on at
    one server at most (D43-A), the notification switch is there where
    the app has push, a server without a kept login offers no Turn off,
    and Remove asks in place and forgets the server."""
    config = {"servers": [server("s1", "Home", NODE1, lan=HOME_LAN), server("s2", "Cabin", NODE2, lan=CABIN_LAN, start=True)],
              "answers": {HOME_LAN: {"node": NODE1}, CABIN_LAN: {"node": NODE2}}, "push": True}
    page = open_connect(new_context, guard, watch, config)
    wait_state(page, "Home", STATE[case.lang]["lan"])
    home = row(page, "Home")
    home.locator(".server-open").click(button="right")
    panel = home.locator(".server-panel")
    panel.wait_for(state="visible")
    assert home.locator(".server-cog").get_attribute("aria-expanded") == "true"
    assert page.evaluate("document.activeElement.classList.contains('server-panel')"), "the panel takes the focus, not a field"
    home.locator(".server-cog").click()
    panel.wait_for(state="detached")
    home.locator(".server-cog").click()
    panel.wait_for(state="visible")
    assert page.evaluate("document.activeElement.tagName") == "FORM", "the panel takes the focus again"
    assert panel.locator("input[name=push]").count() == 1, "the notification switch, the app having push"
    assert panel.locator("input").first.input_value() == "Home"
    assert not panel.locator("input[name=start]").is_checked(), "Open at start is another server's"
    named_buttons(page, "a server's settings")
    assert_axe(page, "a server's settings")

    assert panel.locator("select[name=ask]").input_value() == "start", "when the app starts, until the user chooses"
    assert panel.locator(".server-login-off").count() == 0 and home.locator(".server-login").is_hidden(), "no kept login, no Turn off"
    panel.locator("select[name=ask]").select_option("return")
    panel.locator("label.switch", has=page.locator("input[name=start]")).locator(".slider").click()
    panel.locator("button[type=submit]").click()
    panel.wait_for(state="detached")
    saved = calls(page, "save")[-1][1]
    assert saved["ask"] == "return" and saved["id"] == "s1" and saved["lan"] == HOME_LAN and "biometric" not in saved, f"the choice is saved: {saved}"
    assert saved["start"] is True, f"Open at start is saved: {saved}"
    assert page.evaluate("window.__fakeBridge.servers.map((s) => s.start)") == [True, False], "and is off at the other server"
    wait_toast(page, "Home")
    row(page, "Cabin").locator(".server-cog").click()
    cabin_panel = row(page, "Cabin").locator(".server-panel")
    cabin_panel.wait_for(state="visible")
    assert not cabin_panel.locator("input[name=start]").is_checked(), "the list read again: Cabin no longer opens at start"
    row(page, "Cabin").locator(".server-cog").click()
    cabin_panel.wait_for(state="detached")

    home.locator(".server-cog").click()
    assert panel.locator("select[name=ask]").input_value() == "return", "and shown as saved"
    assert panel.locator("input[name=start]").is_checked()
    panel.locator(".server-remove").click()
    page.locator(".confirm-box .confirm-yes").click()
    home.wait_for(state="detached")
    assert calls(page, "remove") == [["remove", "s1"]]
    wait_toast(page, "Home")
    assert page.evaluate("document.activeElement.closest('#servers-menu') !== null"), "the focus goes to the ☰"


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
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'discover')", 3), "the page searches at its start, for the list too"
    assert calls(page, "discover") == [["discover", 5000]]
    assert page.locator("#add").is_hidden(), "the Add card waits for the ☰"
    wait_state(page, "Home", STATE[case.lang]["lan"])
    assert wait_until(page, "!document.getElementById('search-btn').disabled", 3), "the first search ends"
    open_add(page)
    assert wait_until(page, "window.__fakeBridge.calls.filter((c) => c[0] === 'discover').length === 2", 3), "the Add card searches as it opens"
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
    assert saved["start"] is False, "only the first server opens at start"
    wait_state(page, "massimo-box", STATE[case.lang]["lan"])
    assert page.locator("#add").is_hidden(), "the Add card closes once a server is added"
    assert wait_until(page, "!document.getElementById('search-btn').disabled", 3), "the search ends"
    open_add(page)
    assert wait_until(page, "!document.getElementById('search-btn').disabled", 3), "its search ends"
    page.locator("#search-btn").click()
    assert wait_until(page, "window.__fakeBridge.calls.filter((c) => c[0] === 'discover').length === 4", 3), "Search again"


def service(name, host, node):
    """A service discovery finds, as the adapter sends it."""
    return {"name": name, "host": host, "port": 8099, "node": node, "version": "v0.0.0-test", "tls": False, "fp": ""}


def test_connect_names(new_context, case, watch, guard):
    """A server added by its address is named by it, and its row shows
    the address once; the name discovery hears for its node replaces
    that name, heard before the server is added or after, while a name
    the user gave stays; a server found without a name shows its address
    once."""
    cellar = "http://192.168.1.20:8099/"
    attic = "http://192.168.1.30:8099/"
    config = {"servers": [], "answers": {cellar: {"node": NODE2}, attic: {"node": NODE3}},
              "found": [service("Attic", "192.168.1.30", NODE3), service("", "192.168.1.50", NODE1)]}
    page = open_connect(new_context, guard, watch, config)
    page.locator(".found-server", has_text="Attic").wait_for()
    nameless = page.locator(".found-server", has_text="192.168.1.50:8099")
    nameless.wait_for()
    assert nameless.locator(".found-where").is_hidden(), "a server found without a name shows its address once"
    assert wait_until(page, "!document.getElementById('search-btn').disabled", 3), "the first search ends"
    address = page.locator("#add-address")
    submit = page.locator("#add-form button[type=submit]")

    address.fill("192.168.1.20:8099")
    submit.click()
    wait_toast(page, "192.168.1.20:8099")
    by_address = row(page, "192.168.1.20:8099")
    by_address.wait_for()
    assert by_address.locator(".server-where").is_hidden(), "a server named by its address shows it once"

    open_add(page)  # closed once the first was added, and searching again
    assert wait_until(page, "!document.getElementById('search-btn').disabled", 3), "the Add card's search ends"
    address.fill("192.168.1.30:8099")
    submit.click()
    wait_toast(page, "Attic")
    attic_row = row(page, "Attic")
    attic_row.wait_for()
    assert attic_row.locator(".server-where").inner_text() == "192.168.1.30:8099", "the address beside the name heard"
    assert page.locator(".found-server", has_text="Attic").count() == 0, "and it is no longer offered"

    page.evaluate(f"window.__fakeBridgeConfig.found = [{json.dumps(service('Cellar', '192.168.1.20', NODE2))}]")
    open_add(page)  # its search hears the name
    cellar_row = row(page, "Cellar")
    cellar_row.wait_for()
    assert cellar_row.locator(".server-where").inner_text() == "192.168.1.20:8099"
    assert page.evaluate(f"window.__fakeBridge.servers.find((s) => s.node === {NODE2!r}).name") == "Cellar", "the name is saved"
    assert wait_until(page, "!document.getElementById('search-btn').disabled", 3), "the second search ends"

    cellar_row.locator(".server-cog").click()
    panel = cellar_row.locator(".server-panel")
    panel.locator("input").first.fill("Basement")
    panel.locator("button[type=submit]").click()
    panel.wait_for(state="detached")
    searched = discovers(page)
    page.locator("#search-btn").click()
    assert wait_until(page, f"window.__fakeBridge.calls.filter((c) => c[0] === 'discover').length === {searched + 1}", 3), "Search again"
    assert wait_until(page, "!document.getElementById('search-btn').disabled", 3), "the third search ends"
    assert row(page, "Basement").count() == 1 and row(page, "Cellar").count() == 0, "a name the user gave stays"
    assert page.evaluate(f"window.__fakeBridge.servers.find((s) => s.node === {NODE2!r}).name") == "Basement"
    assert_axe(page, "the names")


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
    wait_toast(page, "Home")
    assert wait_until(page, "!document.querySelector(\".server-row[data-id='s1'] .server-login-kept\")", 3), "the settings drawn again without it"
    assert home.locator(".server-login").is_hidden(), "and the row without its fingerprint"

    # Forget all, from the ☰ where a login is kept: asked in the card
    # under the list, then every login forgotten.
    assert cabin.locator(".server-login").is_visible()
    menu_entry(page, "menu-forget-all").click()
    ask = page.locator("#servers-ask .confirm-box")
    ask.wait_for(state="visible")
    assert page.evaluate("document.activeElement.classList.contains('confirm-no')"), "the question takes the focus"
    named_buttons(page, "Forget all's question")
    assert_axe(page, "Forget all's question")
    ask.locator(".confirm-no").click()
    ask.wait_for(state="detached")
    assert calls(page, "forgetAll") == [], "No forgets nothing"
    menu_entry(page, "menu-forget-all").click()
    page.locator("#servers-ask .confirm-yes").click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'forgetAll')", 3)
    ask.wait_for(state="detached")
    assert cabin.locator(".server-login").is_hidden()
    page.locator("#servers-menu > summary").click()
    page.locator("#menu-add").wait_for(state="visible")
    assert page.locator("#menu-forget-all").is_hidden(), "no login left, no Forget all"
    page.keyboard.press("Escape")


def test_connect_login_refused(new_context, case, watch, guard):
    """A kept key the server refused is forgotten by the app: the open
    says why, and the row loses its fingerprint."""
    config = {"servers": [server("s1", "Home", NODE1, lan=HOME_LAN, login="anna")], "answers": {HOME_LAN: {"node": NODE1}},
              "openLogin": "error:the server no longer takes the kept key"}
    page = open_connect(new_context, guard, watch, config)
    wait_state(page, "Home", STATE[case.lang]["lan"])
    row(page, "Home").locator(".server-login").click()
    wait_toast(page, "no longer takes", bad=True)
    assert wait_until(page, "document.querySelector(\".server-row[data-id='s1'] .server-login\").classList.contains('hidden')", 3)


SIGNED_OUT_ALL = {"en": "Signed out of every server", "de": "Bei jedem Server abgemeldet"}
CERT_SELF = {"en": "the certificate itself (self-signed)", "de": "das Zertifikat selbst (selbst signiert)"}


def test_connect_menu_and_add_card(new_context, case, watch, guard):
    """The Servers card's ☰ (AP MobileApp, F21, F22): Add server…, Sign
    out of all and App settings…, Forget all stored credentials only where
    a login is kept. Add server… shows the Add card, which takes the focus
    and not its address field, searches as it opens, and closes with ×;
    the last server removed leaves the card shown without ×."""
    config = {"servers": [server("s1", "Home", NODE1, lan=HOME_LAN)], "answers": {HOME_LAN: {"node": NODE1}}}
    page = open_connect(new_context, guard, watch, config)
    wait_state(page, "Home", STATE[case.lang]["lan"])
    assert page.locator("#add").is_hidden()
    menu_entry(page, "menu-add")
    for entry in ("menu-sign-out-all", "menu-app"):
        assert page.locator(f"#{entry}").is_visible(), f"{entry} in the ☰"
    assert page.locator("#menu-forget-all").is_hidden(), "no login kept, no Forget all"
    named_buttons(page, "the list's ☰")
    assert_axe(page, "the list's ☰")
    page.keyboard.press("Escape")
    assert wait_until(page, "!document.getElementById('servers-menu').open", 3), "Escape closes the ☰"
    assert wait_until(page, "!document.getElementById('search-btn').disabled", 3), "the start's search ends"
    before = discovers(page)
    open_add(page)
    assert page.evaluate("document.activeElement.id") == "add", "the card takes the focus, not its address field"
    assert wait_until(page, f"window.__fakeBridge.calls.filter((c) => c[0] === 'discover').length === {before + 1}", 3), "and searches"
    close = page.locator("#add-close")
    assert close.is_visible(), "× where the list is not empty"
    named_buttons(page, "the Add card from the ☰")
    assert_axe(page, "the Add card from the ☰")
    close.click()
    page.locator("#add").wait_for(state="hidden")
    assert page.evaluate("document.activeElement.closest('#servers-menu') !== null"), "the focus back on the ☰"

    home = row(page, "Home")
    home.locator(".server-cog").click()
    home.locator(".server-remove").click()
    page.locator(".confirm-box .confirm-yes").click()
    page.locator("#add").wait_for(state="visible")
    assert page.locator("#add-close").is_hidden(), "an empty list keeps the Add card"
    assert page.locator("#servers-empty").is_visible()
    assert page.evaluate("document.activeElement.id") == "add", "which takes the focus"
    page.locator("#servers-menu > summary").click()
    page.locator("#menu-add").wait_for(state="visible")
    assert page.locator("#menu-sign-out-all").is_hidden(), "no server, no Sign out of all"


def test_connect_sessions(new_context, case, watch, guard):
    """The list marks a server where the app holds a session with its
    user, checked at each probe at the address found (session, AP
    MobileApp, D46-A); one the server refused loses its mark, one not
    checked keeps what the app said. Sign out in a server's settings ends
    it there and keeps what is typed; Sign out of all, in the ☰, ends
    every one."""
    config = {
        "servers": [server("s1", "Home", NODE1, lan=HOME_LAN, session="anna"), server("s2", "Cabin", NODE2, lan=CABIN_LAN, session="bob"),
                    server("s3", "Office", NODE3, remote=OFFICE_REMOTE, session="carla")],
        "answers": {HOME_LAN: {"node": NODE1}, CABIN_LAN: {"node": NODE2}},
        "sessions": {"s2": None},
    }
    page = open_connect(new_context, guard, watch, config)
    st = STATE[case.lang]
    wait_state(page, "Home", st["lan"])
    wait_state(page, "Cabin", st["lan"])
    wait_state(page, "Office", st["offline"])
    assert wait_until(page, "window.__fakeBridge.calls.filter((c) => c[0] === 'session').length === 2", 3), "both answering servers checked"
    assert sorted(calls(page, "session")) == [["session", "s1", HOME_LAN], ["session", "s2", CABIN_LAN]], "at the address the probe found"
    assert wait_until(page, "!document.querySelector(\".server-row[data-id='s2'] .session-badge\")", 3), "a session the server refused loses its mark"
    assert "anna" in row(page, "Home").locator(".session-badge").inner_text(), "the user is named"
    assert "carla" in row(page, "Office").locator(".session-badge").inner_text(), "one not checked keeps what the app said"
    named_buttons(page, "the sessions")
    assert_axe(page, "the sessions")

    home = row(page, "Home")
    home.locator(".server-cog").click()
    panel = home.locator(".server-panel")
    line = panel.locator(".server-session")
    line.wait_for()
    assert "anna" in line.inner_text()
    named_buttons(page, "a server's session")
    assert_axe(page, "a server's session")
    panel.locator("input").first.fill("Home sweet")
    line.locator(".server-sign-out").click()
    wait_toast(page, "Home")
    assert calls(page, "signOut") == [["signOut", "s1", HOME_LAN]], "signed out where the probe found it"
    line.wait_for(state="detached")
    assert home.locator(".session-badge").count() == 0, "and the mark is gone"
    assert panel.locator("input").first.input_value() == "Home sweet", "the settings keep what is typed"
    home.locator(".server-cog").click()
    panel.wait_for(state="detached")

    menu_entry(page, "menu-sign-out-all").click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'signOutAll')", 3)
    wait_toast(page, SIGNED_OUT_ALL[case.lang])
    assert wait_until(page, "!document.querySelector('.session-badge')", 3), "no mark left"


def test_connect_certificate(new_context, case, watch, guard):
    """A server reached over HTTPS shows its certificate in its settings:
    a line with its subject's name and the end of its validity, and
    Certificate…, a dialog with the subject, the issuer, the names, the
    validity, the fingerprint and since when it is trusted (AP MobileApp,
    D51-B); a server over plain HTTP has no such line."""
    lan = "https://192.168.1.10:8443/"
    config = {
        "servers": [server("s1", "Home", NODE1, lan=lan, fp=FP1, trusted_at="2026-11-20T08:30:00Z"), server("s2", "Cabin", NODE2, lan=CABIN_LAN)],
        "answers": {lan: {"node": NODE1, "fingerprint": FP1, "self_signed": True, "subject": "CN=massimo.local", "names": ["massimo.local", "192.168.1.10"],
                          "not_before": "2026-11-01T12:00:00Z", "not_after": "2027-11-01T12:00:00Z"},
                    CABIN_LAN: {"node": NODE2}},
    }
    page = open_connect(new_context, guard, watch, config)
    wait_state(page, "Home", STATE[case.lang]["lan"])
    wait_state(page, "Cabin", STATE[case.lang]["lan"])
    cabin = row(page, "Cabin")
    cabin.locator(".server-cog").click()
    cabin.locator(".server-panel").wait_for(state="visible")
    assert cabin.locator(".server-cert").count() == 0, "plain HTTP: no certificate"
    cabin.locator(".server-cog").click()

    home = row(page, "Home")
    home.locator(".server-cog").click()
    line = home.locator(".server-panel .server-cert")
    line.wait_for()
    assert "massimo.local" in line.inner_text() and "2027" in line.inner_text(), f"its subject's name and its end: {line.inner_text()!r}"
    named_buttons(page, "a server's certificate line")
    line.locator(".server-cert-open").click()
    dlg = page.locator("#cert-dialog")
    dlg.wait_for(state="visible")
    assert page.locator("#cert-subject").inner_text() == "CN=massimo.local"
    assert page.locator("#cert-issuer").inner_text() == CERT_SELF[case.lang]
    assert page.locator("#cert-names").inner_text() == "massimo.local, 192.168.1.10"
    assert "2026" in page.locator("#cert-from").inner_text() and "2027" in page.locator("#cert-until").inner_text()
    assert page.locator("#cert-fp").inner_text() == FP1
    assert "2026" in page.locator("#cert-trusted").inner_text(), "since when it is trusted"
    assert page.evaluate("document.activeElement.id") == "cert-close"
    named_buttons(page, "the certificate's dialog")
    assert_axe(page, "the certificate's dialog")
    page.locator("#cert-close").click()
    dlg.wait_for(state="hidden")
    line.locator(".server-cert-open").click()
    dlg.wait_for(state="visible")
    page.keyboard.press("Escape")
    dlg.wait_for(state="hidden")


def test_connect_app_settings(new_context, case, watch, guard):
    """App settings, from the ☰ (AP MobileApp, D53-A): the appearance the
    app keeps, as the system at first; another is saved in the app and
    given to the page at once; the page loaded again with the card open,
    as the app does once it took another appearance, opens it again; ×
    closes it; an appearance the app refused is said in the card."""
    config = {"servers": [server("s1", "Home", NODE1, lan=HOME_LAN)], "answers": {HOME_LAN: {"node": NODE1}}}
    page = open_connect(new_context, guard, watch, config)
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'settings')", 3), "the page reads the app's settings"
    assert page.evaluate("document.documentElement.getAttribute('data-theme')") is None, "as the system"
    menu_entry(page, "menu-app").click()
    card = page.locator("#app-card")
    card.wait_for(state="visible")
    assert page.evaluate("document.activeElement.id") == "app-card"
    select = page.locator("#app-appearance")
    assert select.input_value() == "system"
    named_buttons(page, "App settings")
    assert_axe(page, "App settings")
    select.select_option("dark")
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'saveSettings')", 3)
    assert calls(page, "saveSettings") == [["saveSettings", {"appearance": "dark"}]]
    assert wait_until(page, "document.documentElement.getAttribute('data-theme') === 'dark'", 3), "the page takes it at once"
    assert_axe(page, "App settings in the dark")
    assert page.evaluate("location.hash") == "#app-settings"

    page.reload()
    page.locator("#servers").wait_for(state="visible")
    card.wait_for(state="visible")
    assert wait_until(page, "document.getElementById('app-appearance').value === 'system'", 3), "as the app said after the load"
    page.evaluate("window.__fakeBridgeConfig.saveSettingsError = 'the phone said no'")
    select.select_option("light")
    page.locator("#app-msg.bad").wait_for()
    assert "the phone said no" in page.locator("#app-msg").inner_text()
    assert select.input_value() == "system", "the choice as it was"
    page.locator("#app-close").click()
    card.wait_for(state="hidden")
    assert page.evaluate("location.hash") == ""
    assert page.evaluate("document.activeElement.closest('#servers-menu') !== null"), "the focus back on the ☰"


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


SELECT_SERVER = {"en": "Select server…", "de": "Server wählen…"}
# the sign-in card's title in the suite's two languages and in the four
# others, for the row in each (FR/IT/NL/PL machine-translated)
TITLES = {"en": "Sign in", "de": "Anmelden"}
OTHER_TITLES = {"fr": "Connexion", "it": "Accedi", "nl": "Aanmelden", "pl": "Logowanie"}


def connects(page, n):
    """Whether the page has called the app's connect() n times by now."""
    return wait_until(page, f"window.__fakeBridge.calls.filter((c) => c[0] === 'connect').length === {n}", 3)


def test_server_page_keeps_the_login(new_context, case, watch, guard):
    """On the sign-in form of a server's page in the app: the switch where
    the phone can and nothing is kept; a sign-in with it on asks the app
    to keep the login, a prompt not passed offers Try again and Sign in
    without it, and the second try keeps it; Cancel, left of each form's
    Sign in, and Select server…, last in the card's ☰, go back to the
    app's servers (AP MobileApp, D54-C), as the Cube choice's entry and
    the main ☰'s do once signed in. The console is not watched: the
    page's requests without a token are refused on purpose, as in
    test_login.py."""
    page = server_page(new_context, guard, watch, {"status": {"can": True, "user": "", "ask": "start", "plain": False}, "keep": [False, True]})
    page.locator("#login-dialog[open]").wait_for()
    page.locator("#login-keep-row").wait_for(state="visible")
    assert page.locator("#login-way-fp").is_hidden(), "nothing kept: no Login by fingerprint"
    assert page.locator("#login-servers").count() == 0, "no Select server at the card's foot"
    cancel = page.locator("#login-cancel")
    assert cancel.is_visible(), "Cancel, the app having its servers"
    assert cancel.evaluate("(b) => b.nextElementSibling.type === 'submit' && b.getBoundingClientRect().right <= b.nextElementSibling.getBoundingClientRect().left"), \
        "left of Sign in"
    named_buttons(page, "the sign-in in the app")
    cancel.click()
    assert connects(page, 1), "Cancel calls the app"
    # the card's ☰, Select server… last in it (AP MobileApp, Block 13: F13)
    page.locator("#login-menu summary").click()
    last = page.locator("#login-menu .menu-items > :last-child")
    assert last.get_attribute("id") == "login-menu-servers" and last.is_visible(), "Select server…, last in the card's ☰"
    assert last.inner_text().strip() == SELECT_SERVER[case.lang]
    last.click()
    assert connects(page, 2), "the card's ☰ calls the app, as Cancel does"
    if page.locator("#login-menu").get_attribute("open") is not None:
        page.locator("#login-menu summary").click()
    # the API key's form has its Cancel too
    page.locator("#login-ways .login-way[data-way='key']").click()
    page.locator("#login-key-cancel").click()
    assert connects(page, 3), "the API key's Cancel calls the app"
    page.locator("#login-ways .login-way[data-way='password']").click()

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

    # Signed in: the Cube choice ends with Select server, and the main ☰
    # has it just before Sign out.
    page.locator("#cube-menu summary").click()
    entry = page.locator("#cube-menu .menu-items button[data-value='__servers']")
    entry.wait_for(state="visible")
    entry.click()
    assert connects(page, 4), "the Cube choice's Select server…"
    page.locator("#nav-menu summary").click()
    servers = page.locator("#menu-servers")
    servers.wait_for(state="visible")
    assert page.evaluate("document.getElementById('menu-servers').nextElementSibling.id") == "menu-logout", "just before Sign out"
    assert servers.inner_text().strip() == SELECT_SERVER[case.lang], "named as the card's ☰ names it"
    servers.click()
    assert connects(page, 5), "the main ☰'s Select server…"


def test_server_page_login_by_fingerprint(new_context, case, watch, guard):
    """Where a key is kept, the sign-in's row starts with the fingerprint,
    named with the user's name, the password's form staying the one
    chosen, and the row with it stays on one line, the card centred (AP
    MobileApp, Block 14); it signs the page in at once with the session
    the app made (AP MobileApp, D49); at 390 px the row is on one line
    in French, Italian, Dutch and Polish as well, chosen in the card's ☰;
    while its login runs, its spinner turns inside the button in the
    icon's place (Block 14: F25); a dismissed prompt leaves the form;
    over plain HTTP outside the home network the form says why it offers
    the password alone."""
    page = server_page(new_context, guard, watch, {"status": {"can": True, "user": "operator", "ask": "start", "plain": False}, "fingerprint": TOKEN})
    page.locator("#login-dialog[open]").wait_for()
    fingerprint = page.locator("#login-way-fp")
    fingerprint.wait_for(state="visible")
    assert "operator" in fingerprint.get_attribute("aria-label"), "named with the user's name"
    assert page.evaluate("document.querySelector('#login-ways .login-way').id") == "login-way-fp", "first in the row"
    assert page.locator("#login-ways .login-way[data-way='password']").get_attribute("aria-pressed") == "true", "Password stays chosen"
    assert_login_row(page, "the sign-in in the app, the fingerprint kept")
    assert_login_card_centred(page, "the sign-in in the app")
    if case.width == 390:
        # the widest row at a phone's width in the other four languages too,
        # chosen in the card's ☰, then back to the browser's
        page.locator("#login-menu summary").click()
        for lang, title in OTHER_TITLES.items():
            page.locator("#login-lang").select_option(lang)
            assert wait_until(page, f"document.getElementById('login-title').textContent === {title!r}", 5), f"the card in {lang}"
            assert_login_row(page, f"the sign-in in the app in {lang}")
        page.locator("#login-lang").select_option("auto")
        assert wait_until(page, f"document.getElementById('login-title').textContent === {TITLES[case.lang]!r}", 5), "the browser's language again"
        page.keyboard.press("Escape")
    assert page.locator("#login-form").is_visible()
    assert page.locator("#login-keep-row").is_hidden(), "a key kept: no switch"
    fingerprint.click()
    page.wait_for_selector(".room", timeout=10000)
    assert calls(page, "login.fingerprint") == [["login.fingerprint"]]
    assert page.evaluate("state.token") == TOKEN, "signed in with the app's session"
    assert page.evaluate("localStorage.getItem('massimo.token')") is None, "and the browser keeps none"

    plain = server_page(new_context, guard, watch, {"status": {"can": True, "user": "operator", "ask": "start", "plain": True}})
    plain.locator("#login-dialog[open]").wait_for()
    plain.locator("#login-fp-plain").wait_for(state="visible")
    assert plain.locator("#login-way-fp").is_hidden() and plain.locator("#login-keep-row").is_hidden(), "plain HTTP: neither"

    dismissed = server_page(new_context, guard, watch, {"status": {"can": True, "user": "operator", "ask": "start", "plain": False}, "fingerprint": None})
    button = dismissed.locator("#login-way-fp")
    button.wait_for(state="visible")
    size = button.evaluate("(b) => [b.offsetWidth, b.offsetHeight]")
    # the prompt held open until the test lets it go, so that the spinner
    # can be looked at
    dismissed.evaluate("""() => { const login = window.massimo.login; const answer = login.fingerprint;
      login.fingerprint = () => new Promise((resolve) => { window.__letFingerprintGo = () => resolve(answer.call(login)); }); }""")
    button.click()
    assert wait_until(dismissed, "document.getElementById('login-way-fp').classList.contains('busy')", 3), "the fingerprint's button turns"
    spin = button.evaluate("""(b) => { const ring = getComputedStyle(b, '::after');
      return {size: [b.offsetWidth, b.offsetHeight], icon: getComputedStyle(b.querySelector('svg')).display,
              ring: [parseFloat(ring.width), parseFloat(ring.height)], content: ring.content}; }""")
    assert spin["size"] == size, f"the button keeps its size while it turns: {spin['size']}, not {size}"
    assert spin["icon"] == "none", "the spinner in the icon's place"
    assert spin["content"] != "none" and all(0 < r <= s for r, s in zip(spin["ring"], size)), f"the spinner inside the button: {spin}"
    dismissed.evaluate("() => { window.__letFingerprintGo(); }")
    assert wait_until(dismissed, "window.__fakeBridge.calls.some((c) => c[0] === 'login.fingerprint')", 3)
    assert wait_until(dismissed, "!document.getElementById('login-way-fp').classList.contains('busy')", 3)
    assert button.evaluate("(b) => getComputedStyle(b.querySelector('svg')).display") != "none", "the icon back"
    assert dismissed.evaluate("document.getElementById('login-dialog').open"), "a dismissed prompt leaves the form"


def test_server_page_first_owner_keeps_the_login(new_context, case, watch, guard):
    """In the app, the first owner's form offers the login by fingerprint
    as the sign-in does (AP MobileApp, D42-A): its switch where the phone
    can and nothing is kept, none over plain HTTP; and Cancel left of
    Create the owner, back to the app's servers (D54-C). /api/v1/me is
    stubbed as an open server without users once the page is in."""
    for plain in (False, True):
        page = server_page(new_context, guard, watch, {"status": {"can": True, "user": "", "ask": "start", "plain": plain}}, token=TOKEN)
        page.wait_for_selector(".room", timeout=10000)
        page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body='{"mode": "open", "setup": true}'))
        page.evaluate("loadMe()")
        assert wait_until(page, "!document.getElementById('setup').classList.contains('hidden')", 5), "the first owner's form shows"
        if plain:
            page.wait_for_timeout(300)
            assert page.locator("#setup-keep-row").is_hidden(), "plain HTTP: no switch"
        else:
            page.locator("#setup-keep-row").wait_for(state="visible")
            assert not page.locator("#setup-keep").is_checked(), "off at first"
            named_buttons(page, "the first owner's form in the app")
            cancel = page.locator("#setup-cancel")
            assert cancel.is_visible() and cancel.evaluate("(b) => b.nextElementSibling.type === 'submit'"), "Cancel left of Create the owner"
            cancel.click()
            assert connects(page, 1), "Cancel calls the app"
        page.unroute("**/api/v1/me")
