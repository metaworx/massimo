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
fingerprint is asked, the panel taking the focus, a tap that leaves the
fingerprint to the app and the row's sign-in icon, Turn off and Forget all with the keys they
leave valid, the sessions the app holds with Sign out and Sign out of
all, a server's certificate, App settings with the appearance, the card
of the server on the phone; a browser without the bridge; and the
server's page taking its token from the bridge and handing it back, its
login by fingerprint and "Select server…" (docs/app.md, contract
version 4).
axe-core (axe-playwright-python) finds no violation on the connect page
in each of its states the tests open.

Each test opens a browser context of its own with the case's viewport
and locale (new_context, as test_login.py does); the guard fails it on a
page error, and on a console error on the connect page, which sends
nothing to the server."""
import json
import os

from axe_playwright_python.sync_playwright import Axe

from conftest import TOKEN, URL, api_call, clear_preferences
from helpers import (OPERATOR_PASSWORD, assert_login_card_centred, assert_login_row, cleanup, held_timers, named_buttons, open_personal_page, undo,
                     wait_until)

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


def server(sid, name, node, lan="", remote="", fp="", login="", ask="start", start=False, session=None, trusted_at="", unrevoked=None,
           unended=None, login_key=0):
    return {"id": sid, "name": name, "node": node, "lan": lan, "remote": remote, "fp": fp, "login": login, "login_key": login_key, "ask": ask,
            "push": False, "start": start, "session": session, "trusted_at": trusted_at, "unrevoked": unrevoked or [], "unended": unended or []}


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
    with held_timers(page, "toast"):  # shown until the probe takes it back, not until its 10 s are up
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
    assert panel.locator(".server-login-off").count() == 0 and home.locator(".server-signin").is_hidden(), "no kept login, no Turn off"
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
    close_toast(page)  # at 390 px it covers the panel's foot
    assert panel.locator("select[name=ask]").input_value() == "return", "and shown as saved"
    assert panel.locator("input[name=start]").is_checked()
    # Remove server on a line of its own under Cancel and Save, after a rule, with what it does (AP MobileApp, F34)
    box = panel.locator(".server-remove-box")
    assert panel.evaluate("""(f) => f.lastElementChild.classList.contains('server-remove-box')
      && f.lastElementChild.previousElementSibling.classList.contains('actions')"""), "under the form's footer, last"
    assert panel.locator(".server-remove").count() == 1 and box.locator(".server-remove").count() == 1, "out of the footer"
    assert box.locator(".server-remove").inner_text() == REMOVE_SERVER[case.lang]
    assert box.locator(".server-remove-hint").inner_text() == REMOVE_HINT[case.lang], "the line saying what it does"
    m = panel.evaluate("""(f) => { const foot = f.querySelector(':scope > .actions').getBoundingClientRect();
      const box = f.querySelector('.server-remove-box'); const r = box.getBoundingClientRect(); const cs = getComputedStyle(box);
      return {below: r.top >= foot.bottom, rule: cs.borderTopStyle === 'solid' && parseFloat(cs.borderTopWidth) >= 1}; }""")
    assert m["below"] and m["rule"], f"below Cancel and Save, after a rule: {m}"
    named_buttons(page, "a server's settings with Remove server")
    assert_axe(page, "a server's settings with Remove server")
    panel.locator(".server-remove").click()
    question = box.locator(".confirm-box")
    question.wait_for(state="visible")
    assert REMOVE_QUESTION[case.lang] in question.inner_text(), f"its question as before: {question.inner_text()!r}"
    assert panel.locator(":scope > .actions button[type=submit]").is_visible(), "asked in Remove's place, Save stays"
    page.locator(".confirm-box .confirm-no").click()
    box.locator(".server-remove").wait_for(state="visible")
    assert page.evaluate("document.activeElement.classList.contains('server-remove')"), "No gives the focus back to Remove server"
    panel.locator(".server-remove").click()
    page.locator(".confirm-box .confirm-yes").click()
    home.wait_for(state="detached")
    assert calls(page, "remove") == [["remove", "s1", HOME_LAN]], "at the address the probe chose"
    wait_toast(page, "Home")
    assert page.evaluate("document.activeElement.closest('#servers-menu') !== null"), "the focus goes to the ☰"


REMOVE_SERVER = {"en": "Remove server", "de": "Server entfernen"}
REMOVE_HINT = {"en": "The server leaves this list; where it answers, this phone is signed out there first.",
               "de": "Der Server wird aus dieser Liste entfernt; antwortet er, wird dieses Telefon dort zuerst abgemeldet."}
REMOVE_HINT_LOGIN = {"en": "The server leaves this list; where it answers, this phone is signed out there first and its key revoked.",
                     "de": "Der Server wird aus dieser Liste entfernt; antwortet er, wird dieses Telefon dort zuerst abgemeldet und sein Schlüssel widerrufen."}
REMOVE_QUESTION = {"en": "Remove Home from the app, with its sign-in?", "de": "Home mit seiner Anmeldung aus der App entfernen?"}
SHOW_OFF = {"en": "Off until the server answers", "de": "Aus, bis der Server antwortet"}


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
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
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


SIGN_IN_ICON = {"en": "Sign in with password or key", "de": "Mit Passwort oder Schlüssel anmelden"}


def test_connect_login_by_fingerprint(new_context, case, watch, guard):
    """A tap on a server opens it with open(id, url) alone, the app asking
    for the fingerprint first as its rule says (AP MobileApp, D72-B,
    D74-A); a server with a kept login has the sign-in icon on its row,
    labelled "Sign in with password or key", which opens its sign-in card
    without the prompt (open with {signIn: true}). Its settings name the
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
    button = home.locator(".server-signin")
    assert button.is_visible(), "the sign-in icon where a login is kept"
    assert button.get_attribute("aria-label") == SIGN_IN_ICON[case.lang] and button.get_attribute("title") == SIGN_IN_ICON[case.lang], \
        f"labelled: {button.get_attribute('aria-label')!r}"
    assert button.locator("use").get_attribute("href") == "#icon-signin", "the mirror of the sign-out icon"
    assert office.locator(".server-signin").is_hidden(), "a server without a kept login has none"
    assert page.locator("#icon-signin").count() == 1, "the icon is the page's"
    named_buttons(page, "the list with kept logins")
    assert_axe(page, "the list with kept logins")

    home.locator(".server-open").click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'open')", 3)
    assert calls(page, "open") == [["open", "s1", HOME_LAN]], f"a tap leaves the asking to the app: {calls(page, 'open')}"
    assert wait_until(page, "!conn.opening", 3), "one opening at a time, the first answered"
    button.click()
    assert wait_until(page, "window.__fakeBridge.calls.filter((c) => c[0] === 'open').length === 2", 3)
    assert calls(page, "open")[-1] == ["open", "s1", HOME_LAN, {"signIn": True}], f"the sign-in card, no prompt: {calls(page, 'open')}"
    # one opening at a time: a second tap while the app's prompt shows asks nothing
    assert wait_until(page, "!conn.opening", 3)
    page.evaluate("""() => { const b = window.massimo; const open = b.open; window.__open = open;
      b.open = (...a) => new Promise((resolve) => { window.__letOpenGo = () => resolve(open.apply(b, a)); }); }""")
    home.locator(".server-open").click()
    assert wait_until(page, "typeof window.__letOpenGo === 'function'", 3)
    home.locator(".server-open").click()
    page.evaluate("() => { window.__letOpenGo(); }")
    assert wait_until(page, "!conn.opening", 3)
    assert len(calls(page, "open")) == 3, f"the second tap asked nothing: {calls(page, 'open')}"
    page.evaluate("() => { window.massimo.open = window.__open; }")

    # Turn off: the fingerprint once, the key revoked at the probed address.
    home.locator(".server-cog").click()
    panel = home.locator(".server-panel")
    panel.wait_for(state="visible")
    assert "anna" in panel.locator(".server-login-kept").inner_text()
    panel.locator(".server-login-off").click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'forget')", 3)
    assert calls(page, "forget") == [["forget", "s1", HOME_LAN]]
    wait_toast(page, LOGIN_OFF_DONE[case.lang])
    assert wait_until(page, "!document.querySelector(\".server-row[data-id='s1'] .server-login-kept\")", 3), "the settings drawn again without it"
    assert home.locator(".server-signin").is_hidden(), "and the row without its sign-in icon"

    # Forget all, from the ☰ where a login is kept: asked in the card
    # under the list, then every login forgotten.
    assert cabin.locator(".server-signin").is_visible()
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
    wait_toast(page, FORGOT_ALL[case.lang])
    assert cabin.locator(".server-signin").is_hidden()
    page.locator("#servers-menu > summary").click()
    page.locator("#menu-add").wait_for(state="visible")
    assert page.locator("#menu-forget-all").is_hidden(), "no login left, no Forget all"
    page.keyboard.press("Escape")


LOGIN_OFF_DONE = {"en": "The login by fingerprint of Home is off: its key is revoked at the server",
                  "de": "Die Anmeldung mit Fingerabdruck bei Home ist aus: ihr Schlüssel ist beim Server widerrufen"}
FORGOT_ALL = {"en": "Every stored login is forgotten, and its key revoked at its server.",
              "de": "Alle gespeicherten Anmeldungen sind vergessen und ihre Schlüssel bei den Servern widerrufen."}
FORGET_ALL_SAYS = {"en": ["Each key is revoked at its server too", "the app signed out there"],
                   "de": ["Jeder Schlüssel wird auch bei seinem Server widerrufen", "die App dort abgemeldet"]}
FORGOT_PHONE = {"en": "Every stored login is forgotten.", "de": "Alle gespeicherten Anmeldungen sind vergessen."}
FORGOT_LEFT_ON = {"en": "The key on Cabin stays valid until the app signs in there again as the same user",
                  "de": "Der Schlüssel bei Cabin bleibt gültig, bis die App sich dort wieder als derselbe Benutzer anmeldet"}
FORGOT_LOCAL_ON = {"en": "The key on Shed is still listed there under My profile", "de": "Der Schlüssel bei Shed steht dort noch unter Mein Profil"}
LOGIN_OFF_LEFT = {"en": "is off on this phone, and the app signed out; its key is still valid at the server",
                  "de": "ist auf diesem Telefon aus, und die App ist abgemeldet; ihr Schlüssel gilt beim Server noch"}
KEY_LEFT = {"en": "A key of this phone is still valid here: key {id} of {user}.", "de": "Ein Schlüssel dieses Telefons gilt hier noch: Schlüssel {id} von {user}."}
SHOW_IN_PROFILE = {"en": "Show in My profile", "de": "In Mein Profil zeigen"}
SHOW_KEY = {"en": "Show in My profile, key {id}", "de": "In Mein Profil zeigen, Schlüssel {id}"}
REMOVE_KEYS_LEFT = {"en": "Key 47 of this phone stays valid there", "de": "Schlüssel 47 dieses Telefons bleibt dort gültig"}
REMOVE_LOGIN = {"en": "Its login by fingerprint is turned off first: its key is revoked at the server where it answers",
                "de": "Die Anmeldung mit Fingerabdruck wird zuerst ausgeschaltet: ihr Schlüssel wird beim Server widerrufen, wo er antwortet"}
REMOVED_KEYS = {"en": "Key 50 of this phone is still valid there", "de": "Schlüssel 50 dieses Telefons gilt dort noch"}

NODE4, NODE5, NODE6, NODE7 = ("44444444-4444-4444-8444-444444444444", "55555555-5555-4555-8555-555555555555",
                              "66666666-6666-4666-8666-666666666666", "77777777-7777-4777-8777-777777777777")
BARN_LAN = "http://192.168.1.50:8099/"
SHED_LAN = "http://192.168.1.60:8099/"
ATTIC_LAN = "http://192.168.1.70:8099/"
GARAGE_LAN = "http://192.168.1.80:8099/"


def close_toast(page):
    """Closes the toast, which at 390 px covers the foot of the open settings."""
    if page.locator("#toast").is_visible():
        page.locator("#toast-close").click()
    assert toast_gone(page)


def settings_of(page, name):
    """Opens a server's settings from its cog and gives the panel."""
    r = row(page, name)
    r.locator(".server-cog").click()
    panel = r.locator(".server-panel")
    panel.wait_for(state="visible")
    return panel


def test_connect_keys_left(new_context, case, watch, guard):
    """The keys Turn off, Forget all and Remove could not revoke (AP
    MobileApp, D56-A, D57-A; the bridge's version 4), as the app's Revoker
    leaves them: Turn off that leaves one says so, at an address that
    answers and without one, and the open settings show it at once; Remove
    with a kept login asks to turn it off first and names the key it could
    not revoke; Remove warns of a key left; Forget all's question says each
    key is revoked and the app signed out, and its toast names the servers
    that keep a key, left or on the phone alone, and not one whose key was
    revoked; a server's settings show each key left with its id and user
    and Show in My profile, named for its key, which opens the server's page
    at My profile with the key marked (open with <address>#profile?key=<id>);
    a session found there revokes them, and the line goes."""
    config = {
        "servers": [server("s1", "Home", NODE1, lan=HOME_LAN, login="anna"), server("s2", "Cabin", NODE2, lan=CABIN_LAN, login="bob"),
                    server("s3", "Office", NODE3, remote=OFFICE_REMOTE, unrevoked=[{"user": "anna", "id": 47}]),
                    server("s4", "Barn", NODE4, lan=BARN_LAN, login="carl"), server("s5", "Shed", NODE5, lan=SHED_LAN, login="dora"),
                    server("s6", "Attic", NODE6, lan=ATTIC_LAN, login="eve"), server("s7", "Garage", NODE7, lan=GARAGE_LAN, login="fay")],
        "answers": {HOME_LAN: {"node": NODE1}, CABIN_LAN: {"node": NODE2}, OFFICE_REMOTE: {"node": NODE3}, BARN_LAN: {"node": NODE4},
                    GARAGE_LAN: {"node": NODE7}},
        "keys": {"s1": {"user": "anna", "id": 45}, "s2": {"user": "bob", "id": 46}, "s4": {"user": "carl", "id": 48},
                 "s5": {"user": "dora", "id": 0}, "s6": {"user": "eve", "id": 49}, "s7": {"user": "fay", "id": 50}},
        "unrevokable": ["s1", "s2", "s7"],
    }
    page = open_connect(new_context, guard, watch, config)
    wait_state(page, "Home", STATE[case.lang]["lan"])
    wait_state(page, "Attic", STATE[case.lang]["offline"])

    # Turn off that cannot revoke, at an address that answers: the toast
    # says so, the line comes at once
    panel = settings_of(page, "Home")
    assert panel.locator(".server-key-left").count() == 0, "no key left yet"
    panel.locator(".server-login-off").click()
    wait_toast(page, LOGIN_OFF_LEFT[case.lang])
    line = panel.locator(".server-key-left")
    line.wait_for(state="visible")
    assert KEY_LEFT[case.lang].format(id=45, user="anna") in line.inner_text()
    row(page, "Home").locator(".server-cog").click()
    close_toast(page)
    # and at none: Attic does not answer, its key left all the same
    panel = settings_of(page, "Attic")
    panel.locator(".server-login-off").click()
    wait_toast(page, LOGIN_OFF_LEFT[case.lang])
    assert calls(page, "forget")[-1] == ["forget", "s6", ""]
    panel.locator(".server-key-left").wait_for(state="visible")
    assert KEY_LEFT[case.lang].format(id=49, user="eve") in panel.locator(".server-key-left").inner_text()
    # Show in My profile off while Attic does not answer, its title saying why, on once it answers (AP MobileApp, F33)
    show = panel.locator(".server-key-show")
    assert show.is_disabled() and show.get_attribute("title") == SHOW_OFF[case.lang], f"off, saying why: {show.get_attribute('title')!r}"
    page.evaluate(f"""() => {{ window.__fakeBridgeConfig.answers[{ATTIC_LAN!r}] = {{node: {NODE6!r}}};
      window.dispatchEvent(new CustomEvent('massimo', {{ detail: {{ type: 'network' }} }})); }}""")
    assert wait_until(page, "!document.querySelector(\".server-row[data-id='s6'] .server-key-show\").disabled", 5), "on once the server answers"
    assert not show.get_attribute("title"), "nothing to say then"
    row(page, "Attic").locator(".server-cog").click()
    close_toast(page)

    # Remove with a kept login: turned off first, the key it could not
    # revoke named
    panel = settings_of(page, "Garage")
    assert panel.locator(".server-remove-hint").inner_text() == REMOVE_HINT_LOGIN[case.lang], "with a kept login, its key revoked too"
    panel.locator(".server-remove").click()
    box = page.locator(".confirm-box")
    box.wait_for(state="visible")
    assert REMOVE_LOGIN[case.lang] in box.inner_text(), f"Remove says it turns the login off: {box.inner_text()!r}"
    box.locator(".confirm-yes").click()
    row(page, "Garage").wait_for(state="detached")
    assert calls(page, "remove") == [["remove", "s7", GARAGE_LAN]]
    wait_toast(page, REMOVED_KEYS[case.lang])
    close_toast(page)

    # Remove warns of a key left; No keeps the server
    panel = settings_of(page, "Office")
    assert KEY_LEFT[case.lang].format(id=47, user="anna") in panel.locator(".server-key-left").inner_text()
    panel.locator(".server-remove").click()
    box = page.locator(".confirm-box")
    box.wait_for(state="visible")
    assert REMOVE_KEYS_LEFT[case.lang] in box.inner_text(), f"Remove warns of the key: {box.inner_text()!r}"
    box.locator(".confirm-no").click()
    assert len(calls(page, "remove")) == 1, "No keeps it"
    # a session found there revokes the key: the list read again, the line gone
    page.evaluate("() => { window.__fakeBridgeConfig.sessions = { s3: 'anna' }; window.dispatchEvent(new CustomEvent('massimo', { detail: { type: 'network' } })); }")
    assert wait_until(page, "!document.querySelector(\".server-row[data-id='s3'] .server-key-left\")", 5), "the key's line goes"
    row(page, "Office").locator(".server-cog").click()

    # Forget all: the question says what it does; Cabin's key left, Shed's
    # forgotten on the phone alone (no id, no address), Barn's revoked
    menu_entry(page, "menu-forget-all").click()
    ask = page.locator("#servers-ask .confirm-box")
    ask.wait_for(state="visible")
    for said in FORGET_ALL_SAYS[case.lang]:
        assert said in ask.inner_text(), f"the question says {said!r}: {ask.inner_text()!r}"
    ask.locator(".confirm-yes").click()
    ask.wait_for(state="detached")
    wait_toast(page, FORGOT_LEFT_ON[case.lang])
    text = page.locator("#toast").inner_text()
    assert FORGOT_PHONE[case.lang] in text and FORGOT_LOCAL_ON[case.lang] in text, f"the toast names both: {text!r}"
    assert "Barn" not in text, f"a revoked key is not named: {text!r}"
    assert calls(page, "forgetAll") == [["forgetAll"]]
    assert row(page, "Cabin").locator(".server-signin").is_hidden()
    close_toast(page)

    # Cabin's key left, shown in its settings and in My profile
    panel = settings_of(page, "Cabin")
    line = panel.locator(".server-key-left")
    assert line.count() == 1 and KEY_LEFT[case.lang].format(id=46, user="bob") in line.inner_text()
    show = line.locator(".server-key-show")
    assert show.inner_text() == SHOW_IN_PROFILE[case.lang]
    assert show.get_attribute("aria-label") == SHOW_KEY[case.lang].format(id=46), "named for its key"
    named_buttons(page, "a server's settings with a key left")
    assert_axe(page, "a server's settings with a key left")
    show.click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'open')", 3)
    assert calls(page, "open") == [["open", "s2", CABIN_LAN + "#profile?key=46"]], f"My profile with the key marked: {calls(page, 'open')}"


LOGIN_OFF_LOCAL = {"en": "The login by fingerprint of Home is off on this phone, and the app signed out; its key is still listed at the server under "
                         "My profile, to revoke there.",
                   "de": "Die Anmeldung mit Fingerabdruck bei Home ist auf diesem Telefon aus, und die App ist abgemeldet; ihr Schlüssel steht beim "
                         "Server noch unter Mein Profil und lässt sich dort widerrufen."}
REMOVED_REVOKED = {"en": "Cabin removed; its key is revoked at the server, and the app signed out there.",
                   "de": "Cabin entfernt; sein Schlüssel ist beim Server widerrufen, und die App ist dort abgemeldet."}
REMOVED_BOTH = {"en": ["Barn removed.", "Key 48 of this phone is still valid there", "Its login's key is still listed at the server under My profile"],
                "de": ["Barn entfernt.", "Schlüssel 48 dieses Telefons gilt dort noch", "Der Schlüssel seiner Anmeldung steht beim Server noch unter Mein Profil"]}


def removed(page, name):
    """Removes a server from its settings, Yes to the question."""
    panel = settings_of(page, name)
    panel.locator(".server-remove").click()
    page.locator(".confirm-box .confirm-yes").click()
    row(page, name).wait_for(state="detached")


def test_connect_turn_off_and_remove_say_how_the_key_went(new_context, case, watch, guard):
    """What Turn off and Remove say of a login's key (AP MobileApp,
    Block 14): Turn off of a login kept without its key's id that the
    server does not revoke, the key listed at the server alone; Remove of a
    server whose key the server revokes; and Remove of one with an older key
    left whose login's key is listed at the server alone, both said."""
    config = {
        "servers": [server("s1", "Home", NODE1, lan=HOME_LAN, login="anna"), server("s2", "Cabin", NODE2, lan=CABIN_LAN, login="bob"),
                    server("s4", "Barn", NODE4, lan=BARN_LAN, login="carl", unrevoked=[{"user": "carl", "id": 48}])],
        "answers": {HOME_LAN: {"node": NODE1}, CABIN_LAN: {"node": NODE2}, BARN_LAN: {"node": NODE4}},
        "keys": {"s1": {"user": "anna", "id": 0}, "s2": {"user": "bob", "id": 46}, "s4": {"user": "carl", "id": 0}},
        "unrevokable": ["s1", "s4"],
    }
    page = open_connect(new_context, guard, watch, config)
    wait_state(page, "Home", STATE[case.lang]["lan"])
    wait_state(page, "Barn", STATE[case.lang]["lan"])
    panel = settings_of(page, "Home")
    panel.locator(".server-login-off").click()
    wait_toast(page, LOGIN_OFF_LOCAL[case.lang])
    assert panel.locator(".server-key-left").count() == 0, "nothing left by an id it does not know"
    row(page, "Home").locator(".server-cog").click()
    close_toast(page)
    removed(page, "Cabin")
    wait_toast(page, REMOVED_REVOKED[case.lang])
    assert calls(page, "remove") == [["remove", "s2", CABIN_LAN]]
    close_toast(page)
    removed(page, "Barn")
    wait_toast(page, REMOVED_BOTH[case.lang][0])
    text = page.locator("#toast").inner_text()
    assert all(part in text for part in REMOVED_BOTH[case.lang]), f"both said: {text!r}"


def test_connect_login_refused(new_context, case, watch, guard):
    """A kept key the server refused at a tap is forgotten by the app: the
    open says why in the toast, and the list read again loses the row's
    sign-in icon."""
    config = {"servers": [server("s1", "Home", NODE1, lan=HOME_LAN, login="anna")], "answers": {HOME_LAN: {"node": NODE1}},
              "openLogin": "error:the server no longer takes the kept key"}
    page = open_connect(new_context, guard, watch, config)
    wait_state(page, "Home", STATE[case.lang]["lan"])
    before = len(calls(page, "servers"))
    row(page, "Home").locator(".server-open").click()
    wait_toast(page, "no longer takes", bad=True)
    assert wait_until(page, "document.querySelector(\".server-row[data-id='s1'] .server-signin\").classList.contains('hidden')", 3)
    assert len(calls(page, "servers")) > before, "the list read again"


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


SESSION_LEFT = {"en": "A session of this phone is still valid here: session {id} of {user}, ended at the next sign-in as that user.",
                "de": "Eine Sitzung dieses Telefons gilt hier noch: Sitzung {id} von {user}, beendet bei der nächsten Anmeldung als dieser Benutzer."}
REMOVE_SESSIONS_LEFT = {"en": "Session 11 of this phone stays valid there until it ends", "de": "Sitzung 11 dieses Telefons bleibt dort gültig"}
REMOVED_SESSIONS = {"en": "Session 11 of this phone is still valid there until it ends", "de": "Sitzung 11 dieses Telefons gilt dort noch"}
REMOVED = {"en": "Home removed.", "de": "Home entfernt."}
SIGNED_OUT_LOCAL = {"en": "the session is ended there at the next sign-in as that user",
                    "de": "die Sitzung wird dort bei der nächsten Anmeldung als dieser Benutzer beendet"}


def test_connect_sessions_left(new_context, case, watch, guard):
    """The sessions the app forgot without the server ending them (AP
    MobileApp, F39, D70-A; the bridge's version 5): a server's settings
    name each with its id and user; Sign out where the server does not
    answer leaves the session, its line there at once; Remove signs out
    first (F38, D69-A), where the server answers nothing is left, and
    where a session is left its question warns of it and its toast names
    it."""
    config = {
        "servers": [server("s1", "Home", NODE1, lan=HOME_LAN, session="anna"), server("s2", "Cabin", NODE2, lan=CABIN_LAN, session="bob"),
                    server("s3", "Office", NODE3, remote=OFFICE_REMOTE, unended=[{"user": "carl", "id": 11}])],
        "answers": {HOME_LAN: {"node": NODE1}},
        "sessionIds": {"s1": {"user": "anna", "id": 7}, "s2": {"user": "bob", "id": 9}},
    }
    page = open_connect(new_context, guard, watch, config)
    st = STATE[case.lang]
    wait_state(page, "Home", st["lan"])
    wait_state(page, "Cabin", st["offline"])

    panel = settings_of(page, "Office")
    line = panel.locator(".server-session-left")
    assert line.count() == 1 and SESSION_LEFT[case.lang].format(id=11, user="carl") in line.inner_text(), "the session left named"
    named_buttons(page, "a server's settings with a session left")
    assert_axe(page, "a server's settings with a session left")
    row(page, "Office").locator(".server-cog").click()

    # Sign out with the server not answering: the session left, its line at once
    panel = settings_of(page, "Cabin")
    panel.locator(".server-sign-out").click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'signOut')", 3)
    assert calls(page, "signOut") == [["signOut", "s2", ""]], "no address that answers"
    wait_toast(page, SIGNED_OUT_LOCAL[case.lang])
    panel.locator(".server-session-left").wait_for(state="visible")
    assert SESSION_LEFT[case.lang].format(id=9, user="bob") in panel.locator(".server-session-left").inner_text()
    row(page, "Cabin").locator(".server-cog").click()
    close_toast(page)

    # Remove of a server that answers: signed out there first, nothing left
    removed(page, "Home")
    assert calls(page, "remove") == [["remove", "s1", HOME_LAN]]
    wait_toast(page, REMOVED[case.lang])
    assert "9" not in page.locator("#toast").inner_text() and "7" not in page.locator("#toast").inner_text(), "no session named"
    close_toast(page)

    # Remove of one with a session left: the question warns, the toast names it
    panel = settings_of(page, "Office")
    panel.locator(".server-remove").click()
    box = page.locator(".confirm-box")
    box.wait_for(state="visible")
    assert REMOVE_SESSIONS_LEFT[case.lang] in box.inner_text(), f"Remove warns of the session: {box.inner_text()!r}"
    box.locator(".confirm-yes").click()
    row(page, "Office").wait_for(state="detached")
    wait_toast(page, REMOVED_SESSIONS[case.lang])


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


def server_page(new_context, guard, watch, login, token=None, address="", more=None):
    """The server's page in the app at address after the server's, its
    bridge with the login by fingerprint as login describes it and what
    more adds to its configuration (fake_bridge.js)."""
    clear_preferences()
    ctx = new_context()
    ctx.add_init_script(f"window.__fakeBridgeConfig = {json.dumps({'kind': 'server', 'token': token, 'login': login, **(more or {})})};")
    ctx.add_init_script(path=FAKE_BRIDGE)
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(URL + address)
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
    page.locator("#login-password").fill(OPERATOR_PASSWORD)
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
    order = page.evaluate("""() => { const ids = []; let e = document.getElementById('menu-servers');
      for (let i = 0; e && i < 4; i++, e = e.nextElementSibling) ids.push(e.id); return ids; }""")
    assert order == ["menu-servers", "menu-server-settings", "menu-app-settings", "menu-logout"], f"then the app's two settings, Sign out last: {order}"
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


APP_ENTRIES = {"en": ["This server's settings…", "App settings…"], "de": ["Einstellungen dieses Servers…", "App-Einstellungen…"]}
LANGS = ("en", "de", "fr", "it", "nl", "pl")

# ☰ inside the window, and the app's entries each on one line (counted by a Range over its label), at the case's width
MENU_FITS = """() => {
  const items = document.querySelector('#nav-menu .menu-items').getBoundingClientRect();
  const vw = document.documentElement.clientWidth;
  const wrapped = [...document.querySelectorAll('#menu-servers, #menu-server-settings, #menu-app-settings')].filter((b) => {
    const tops = new Set();
    for (const n of b.querySelectorAll('.menu-item-label')) {
      const r = document.createRange();
      r.selectNodeContents(n);
      for (const rect of r.getClientRects()) tops.add(Math.round(rect.top));
    }
    return tops.size > 1;
  }).map((b) => b.id);
  return {inside: items.left >= 0 && items.right <= vw, wrapped};
}"""


def test_server_page_menu_app_entries(new_context, case, watch, guard):
    """In the app ☰ has, after Select server…, This server's settings…
    and App settings… (AP MobileApp, D73-A), each where the bridge has
    its call, each calling it with nothing of the page's: the bridge
    fills the server in; they fit ☰ at the case's width in the six
    languages; an app before contract version 5 has neither."""
    page = server_page(new_context, guard, watch, {"status": {"can": True, "user": "", "ask": "start", "plain": False}}, token=TOKEN)
    page.wait_for_selector(".room", timeout=10000)
    page.locator("#nav-menu summary").click()
    entries = [page.locator("#menu-server-settings"), page.locator("#menu-app-settings")]
    for e in entries:
        e.wait_for(state="visible")
    assert [e.inner_text().strip() for e in entries] == APP_ENTRIES[case.lang], "named"
    assert [e.locator("svg use").get_attribute("href") for e in entries] == ["#icon-cog", "#icon-sliders"], "with their icons"
    named_buttons(page, "☰ in the app")
    for lang in LANGS:
        page.evaluate(f"() => {{ setSetting('lang', {lang!r}); return changeLanguage({lang!r}).then(applyLanguage); }}")
        assert wait_until(page, f"i18n.lang === {lang!r}", 5), lang
        if page.locator("#nav-menu").get_attribute("open") is None:
            page.locator("#nav-menu summary").click()
        fits = page.evaluate(MENU_FITS)
        assert fits["inside"] and not fits["wrapped"], f"☰ fits in {lang}: {fits}"
    page.evaluate("() => { setSetting('lang', 'auto'); return changeLanguage('auto').then(applyLanguage); }")
    if page.locator("#nav-menu").get_attribute("open") is None:
        page.locator("#nav-menu summary").click()
    entries[0].click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'openServerSettings')", 3), "This server's settings… calls the app"
    page.locator("#nav-menu summary").click()
    entries[1].click()
    assert wait_until(page, "window.__fakeBridge.calls.some((c) => c[0] === 'openAppSettings')", 3), "App settings… calls the app"
    assert [c for c in calls(page) if c[0].startswith("open")] == [["openServerSettings"], ["openAppSettings"]], "each once, with no argument"

    # the entries are the bridge's calls, a login by fingerprint configured or not
    plain = server_page(new_context, guard, watch, None, token=TOKEN)
    plain.wait_for_selector(".room", timeout=10000)
    plain.locator("#nav-menu summary").click()
    plain.locator("#menu-server-settings").wait_for(state="visible")
    assert plain.locator("#menu-app-settings").is_visible(), "both, without the login by fingerprint"

    older = server_page(new_context, guard, watch, {"status": {"can": True, "user": "", "ask": "start", "plain": False}}, token=TOKEN,
                        more={"appEntries": False})
    older.wait_for_selector(".room", timeout=10000)
    older.locator("#nav-menu summary").click()
    older.locator("#menu-servers").wait_for(state="visible")
    assert older.locator("#menu-server-settings").is_hidden() and older.locator("#menu-app-settings").is_hidden(), "an app before version 5: neither"


def test_connect_opens_what_the_address_names(new_context, case, watch, guard):
    """The app loads the connect page with #settings=<id> from a server's
    ☰ and with #app-settings (AP MobileApp, D73-A): the page opens that
    server's settings at its start, the panel taking the focus, and takes
    the fragment from the address; an id that is not the list's opens
    nothing; #app-settings opens App settings."""
    config = {"servers": [server("s1", "Home", NODE1, lan=HOME_LAN), server("s2", "Cabin", NODE2, lan=CABIN_LAN)],
              "answers": {HOME_LAN: {"node": NODE1}, CABIN_LAN: {"node": NODE2}}}
    ctx = new_context()
    ctx.add_init_script(f"window.__fakeBridgeConfig = {json.dumps(config)};")
    ctx.add_init_script(path=FAKE_BRIDGE)
    page = ctx.new_page()
    guard.page = page
    page.set_default_timeout(5000)
    page.on("console", lambda m: watch.problems.append(f"console: {m.text}") if m.type == "error" else None)
    page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
    page.goto(CONNECT + "#settings=s2")
    panel = row(page, "Cabin").locator(".server-panel")
    panel.wait_for(state="visible")
    assert page.evaluate("document.activeElement.classList.contains('server-panel')"), "the panel takes the focus"
    assert row(page, "Cabin").locator(".server-cog").get_attribute("aria-expanded") == "true"
    assert page.evaluate("location.hash") == "", "the fragment taken from the address"
    assert row(page, "Home").locator(".server-panel").count() == 0
    page.goto("about:blank")  # a load of its own, not a change of the fragment
    page.goto(CONNECT + "#settings=nope")
    page.locator("#servers").wait_for(state="visible")
    wait_state(page, "Home", STATE[case.lang]["lan"])
    assert page.locator(".server-panel").count() == 0 and page.evaluate("location.hash") == "", "an id not in the list opens nothing"
    page.goto("about:blank")
    page.goto(CONNECT + "#app-settings")
    page.locator("#app-card").wait_for(state="visible")


SERVER_MORE = {"en": "About this server", "de": "Über diesen Server"}
SERVER_FACTS = {"en": ["Instance id", "Certificate fingerprint"], "de": ["Instanz-ID", "Fingerabdruck des Zertifikats"]}

# the server's line and its facts inside the card, nothing past its padding
LINE_FITS = """() => {
  const card = document.getElementById('login-dialog');
  const cs = getComputedStyle(card);
  const box = card.getBoundingClientRect();
  const left = box.left + parseFloat(cs.paddingLeft) - 0.5;
  const right = box.right - parseFloat(cs.paddingRight) + 0.5;
  const out = [...document.querySelectorAll('#login-server, #login-server *')].filter((e) => {
    const r = e.getBoundingClientRect();
    return r.width && (r.left < left || r.right > right);
  }).map((e) => e.id || e.tagName);
  return {out, wide: card.scrollWidth > card.clientWidth};
}"""


def test_server_page_names_its_server(new_context, case, watch, guard):
    """In the app the sign-in card names the server it signs in to (AP
    MobileApp, F31, D62-A), on a line of its own after the title, whose
    text stays the title's: the server's name and the full address the
    page was opened at, the name once where it is the address, with a
    labelled ⓘ that shows the instance id and the pinned fingerprint,
    each copied with a tap, an empty one left out and the ⓘ too where
    both are, the ⓘ closed again at the next opening; the line and its
    facts fit the card at 390 px in the six languages; a browser shows
    no line."""
    address = URL.rstrip("/")
    status = {"can": True, "user": "", "ask": "start", "plain": False, "server": {"name": "Home", "url": URL, "node": NODE1, "fp": FP1}}
    page = server_page(new_context, guard, watch, {"status": status})
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
    page.locator("#login-dialog[open]").wait_for()
    line = page.locator("#login-server")
    line.wait_for(state="visible")
    assert page.evaluate("document.getElementById('login-title').nextElementSibling.id") == "login-server", "a line of its own after the title"
    assert page.locator("#login-title").inner_text() == TITLES[case.lang], "the title's text as it was"
    assert page.locator("#login-server-name").inner_text() == "Home"
    assert page.locator("#login-server-address").inner_text() == address, "the full address the page was opened at"
    more = page.locator("#login-server-more")
    assert more.get_attribute("aria-label") == SERVER_MORE[case.lang] and more.get_attribute("aria-expanded") == "false", "the ⓘ, labelled"
    facts = page.locator("#login-server-facts")
    assert facts.is_hidden(), "its facts hidden until asked for"
    assert_login_card_centred(page, "the card naming its server")
    named_buttons(page, "the card naming its server")
    more.click()
    facts.wait_for(state="visible")
    assert more.get_attribute("aria-expanded") == "true"
    assert page.locator("#login-server-facts dt").all_inner_texts() == SERVER_FACTS[case.lang]
    assert page.locator("#login-server-node").inner_text() == NODE1 and page.locator("#login-server-fp").inner_text() == FP1
    for fact, value in (("#login-server-node", NODE1), ("#login-server-fp", FP1)):
        page.evaluate("navigator.clipboard.writeText('')")
        page.locator(fact).click()
        assert wait_until(page, f"navigator.clipboard.readText().then((t) => t === {value!r})", 3), f"{fact} copied with a tap"
    fits = page.evaluate(LINE_FITS)
    assert not fits["out"] and not fits["wide"], f"the line and its facts inside the card: {fits}"
    if case.width == 390:
        page.locator("#login-menu summary").click()
        for lang, title in OTHER_TITLES.items():
            page.locator("#login-lang").select_option(lang)
            assert wait_until(page, f"document.getElementById('login-title').textContent === {title!r}", 5), f"the card in {lang}"
            fits = page.evaluate(LINE_FITS)
            assert not fits["out"] and not fits["wide"], f"the line and its facts inside the card in {lang}: {fits}"
        page.locator("#login-lang").select_option("auto")
        assert wait_until(page, f"document.getElementById('login-title').textContent === {TITLES[case.lang]!r}", 5)
        page.keyboard.press("Escape")
    # the next opening: the ⓘ closed again
    page.evaluate("""() => new Promise((done) => { const d = document.getElementById('login-dialog');
      d.addEventListener('close', () => setTimeout(() => { askLogin(); done(); }, 0), { once: true }); d.close(); })""")
    assert wait_until(page, "document.getElementById('login-dialog').open && document.getElementById('login-server-more').getAttribute('aria-expanded') === 'false'", 5), \
        "the ⓘ closed at the next opening"
    assert facts.is_hidden()

    # named by its address: the address once; no fingerprint pinned: its fact left out
    named = server_page(new_context, guard, watch, {"status": dict(status, server={"name": URL.split("//")[1].rstrip("/"), "url": URL, "node": NODE1, "fp": ""})})
    named.locator("#login-server").wait_for(state="visible")
    assert named.locator("#login-server-name").count() == 0 and named.locator("#login-server-address").inner_text() == address, "the name once"
    named.locator("#login-server-more").click()
    assert named.locator("#login-server-facts dt").all_inner_texts() == SERVER_FACTS[case.lang][:1], "the fingerprint left out"
    # neither known: no ⓘ
    bare = server_page(new_context, guard, watch, {"status": dict(status, server={"name": "Home", "url": URL, "node": "", "fp": ""})})
    bare.locator("#login-server").wait_for(state="visible")
    assert bare.locator("#login-server-more").count() == 0 and bare.locator("#login-server-facts").count() == 0, "no ⓘ"

    # a browser: no line, its address bar names the server
    ctx = new_context()
    browser = ctx.new_page()
    guard.page = browser
    browser.goto(URL)
    browser.locator("#login-dialog[open]").wait_for()
    browser.locator("#login-title").wait_for(state="visible")
    assert browser.locator("#login-server").count() == 0, "no line in a browser"


THIS_DEVICE_KEY = {"en": "this device's key", "de": "Schlüssel dieses Geräts"}
THIS_DEVICE = {"en": "this device", "de": "dieses Gerät"}
TOKEN_ID = {"en": "id {id}", "de": "ID {id}"}
REVOKED_AT_SIGN_IN = {"en": "Key {id} is revoked already", "de": "Schlüssel {id} ist schon widerrufen"}


def test_server_page_this_devices_key(new_context, case, watch, guard):
    """My profile in the app after a password sign-in where the phone
    keeps a key of the user's (AP MobileApp, F35, F37; D65-A, D66-A,
    D67-A): that key's row says "this device's key", by login.status()'s
    key_id, the session's "this device", and every row shows its id;
    opened at #profile?key=<id> for a key the sign-in revoked, as the app
    revokes a key left before setToken answers {revoked}, the list is
    read after that revoke, so the key is not listed nor marked, and a
    toast names it."""
    key = api_call("POST", "api/v1/users/operator/tokens", {"label": f"ui-test-device-key-{case.width}-{case.lang}"})
    gone = api_call("POST", "api/v1/users/operator/tokens", {"label": f"ui-test-left-{case.width}-{case.lang}"})
    made = []  # the sign-in's session, revoked at the end
    try:
        status = {"can": True, "user": "operator", "ask": "start", "plain": False, "key_id": key["id"]}
        page = server_page(new_context, guard, watch, {"status": status}, address=f"#profile?key={gone['id']}",
                           more={"revokeAtSignIn": {"user": "operator", "ids": [gone["id"]], "ms": 1500}})
        order = []
        page.on("request", lambda r: order.append(r.method + " " + r.url.split("/api/v1/")[-1]) if "/users/operator/tokens" in r.url else None)
        page.locator("#login-dialog[open]").wait_for()
        page.locator("#login-user").fill("operator")
        page.locator("#login-password").fill(OPERATOR_PASSWORD)
        page.locator("#login-form button[type=submit]").click()
        row = page.locator(f"#profile-tokens li[data-id='{key['id']}']")
        row.wait_for(timeout=10000)
        assert wait_until(page, "!document.getElementById('toast').classList.contains('hidden') && document.getElementById('toast').textContent"
                                f".includes({json.dumps(REVOKED_AT_SIGN_IN[case.lang].format(id=gone['id']))})", 5), \
            f"a toast names the key the sign-in revoked: {page.locator('#toast').inner_text()!r}"
        assert order and order[0] == f"DELETE users/operator/tokens/{gone['id']}", f"the list read after the sign-in's revoke: {order}"
        assert page.locator(f"#profile-tokens li[data-id='{gone['id']}']").count() == 0, "the key revoked is not listed"
        assert page.locator("#profile-tokens li.token-marked").count() == 0, "nor marked"
        assert page.evaluate("location.hash") == "#profile", "the key dropped from the address"
        assert "token-device-key" in (row.get_attribute("class") or "") and THIS_DEVICE_KEY[case.lang] in row.inner_text(), \
            f"the phone's key tagged: {row.inner_text()!r}"
        session = page.evaluate("state.me.token.id")
        made.append(session)
        current = page.locator(f"#profile-tokens li[data-id='{session}']")
        assert THIS_DEVICE[case.lang] in current.inner_text() and THIS_DEVICE_KEY[case.lang] not in current.inner_text(), "the session: this device"
        rows = page.evaluate("[...document.querySelectorAll('#profile-tokens li[data-id]')].map((li) => [li.dataset.id, "
                             "(li.querySelector('.token-id') || {}).textContent])")
        assert rows and all(text == TOKEN_ID[case.lang].format(id=i) for i, text in rows), f"every row its id: {rows}"
        named_buttons(page, "My profile in the app")
    finally:
        cleanup(lambda: undo("the device's key", lambda: api_call("DELETE", "api/v1/users/operator/tokens/%d" % key["id"])),
                lambda: undo("the key left", lambda: api_call("DELETE", "api/v1/users/operator/tokens/%d" % gone["id"])),
                *[lambda tid=tid: undo("the session", lambda: api_call("DELETE", "api/v1/users/operator/tokens/%d" % tid)) for tid in made])


def test_server_page_revoked_at_once(new_context, case, watch, guard):
    """D103-A's case of the UI suite (AP MobileApp, F44): My profile opened
    at #profile?key=<id> for a key the sign-in revokes, the bridge's
    setToken answering {revoked: [id]} at once, before My profile reads its
    list: the toast names the key, and the key is not marked."""
    gone = api_call("POST", "api/v1/users/operator/tokens", {"label": f"ui-test-left-at-once-{case.width}-{case.lang}"})
    made = []  # the sign-in's session, revoked at the end
    try:
        status = {"can": True, "user": "operator", "ask": "start", "plain": False}
        page = server_page(new_context, guard, watch, {"status": status}, address=f"#profile?key={gone['id']}",
                           more={"revokeAtSignIn": {"user": "operator", "ids": [gone["id"]], "ms": 0}})
        page.locator("#login-dialog[open]").wait_for()
        page.locator("#login-user").fill("operator")
        page.locator("#login-password").fill(OPERATOR_PASSWORD)
        page.locator("#login-form button[type=submit]").click()
        assert wait_until(page, "!document.getElementById('toast').classList.contains('hidden') && document.getElementById('toast').textContent"
                                f".includes({json.dumps(REVOKED_AT_SIGN_IN[case.lang].format(id=gone['id']))})", 10), \
            f"a toast names the key the sign-in revoked at once: {page.locator('#toast').inner_text()!r}"
        page.locator("#profile-tokens li[data-id]").first.wait_for(timeout=10000)
        made.append(page.evaluate("state.me.token.id"))
        assert page.locator(f"#profile-tokens li[data-id='{gone['id']}']").count() == 0, "the key revoked is not listed"
        assert page.locator("#profile-tokens li.token-marked").count() == 0, "nor marked"
    finally:
        cleanup(lambda: undo("the key left", lambda: api_call("DELETE", "api/v1/users/operator/tokens/%d" % gone["id"])),
                *[lambda tid=tid: undo("the session", lambda: api_call("DELETE", "api/v1/users/operator/tokens/%d" % tid)) for tid in made])


LOGIN_KEPT_KEY = {"en": "Login by fingerprint: anna, key 45", "de": "Anmeldung mit Fingerabdruck: anna, Schlüssel 45"}
LOGIN_KEPT = {"en": "Login by fingerprint: bob", "de": "Anmeldung mit Fingerabdruck: bob"}


def test_connect_login_key_id(new_context, case, watch, guard):
    """A server's settings name the kept login's key by its id whenever
    the app knows it (login_key, the bridge's version 5; AP MobileApp,
    D66-A), and not where it does not."""
    config = {"servers": [server("s1", "Home", NODE1, lan=HOME_LAN, login="anna", login_key=45), server("s2", "Cabin", NODE2, lan=CABIN_LAN, login="bob")],
              "answers": {HOME_LAN: {"node": NODE1}, CABIN_LAN: {"node": NODE2}}}
    page = open_connect(new_context, guard, watch, config)
    wait_state(page, "Home", STATE[case.lang]["lan"])
    panel = settings_of(page, "Home")
    assert panel.locator(".server-login-kept").inner_text().startswith(LOGIN_KEPT_KEY[case.lang]), panel.locator(".server-login-kept").inner_text()
    row(page, "Home").locator(".server-cog").click()
    panel = settings_of(page, "Cabin")
    text = panel.locator(".server-login-kept").inner_text()
    assert text.startswith(LOGIN_KEPT[case.lang]) and "45" not in text, f"no id it does not know: {text!r}"
