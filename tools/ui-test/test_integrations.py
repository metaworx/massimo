# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings → Integrations (AP IntegrationUsers v1.0, Block 6), as
the owner: an integration asks to be registered, its request waits with
its code, the owner approves it with a narrower maximum, the integration
collects its key and acts for a user of its own, whom the owner links to
the operator, proposed by name; the users list then marks the link. Each
case has an installation of its own, revoked at the end."""
import json
import ssl
import urllib.request

from conftest import URL, api_call
from helpers import cleanup, named_buttons, reads_of, safe_route, scroll_width, show_view, undo, wait_until

HEADER = "X-MassimoHA-On-Behalf-Of"


def raw_call(method, path, body=None, headers=None):
    """A request without the test token: the integration's own."""
    tls = ssl.create_default_context()
    tls.check_hostname, tls.verify_mode = False, ssl.CERT_NONE
    data = json.dumps(body).encode() if body is not None else None
    h = dict(headers or {})
    if data is not None:
        h["Content-Type"] = "application/json"
    req = urllib.request.Request(URL.rstrip("/") + "/" + path.lstrip("/"), method=method, headers=h, data=data)
    with urllib.request.urlopen(req, timeout=5, context=tls) as resp:
        text = resp.read()
        return resp.status, json.loads(text) if text else None


def test_register_and_link(page, case):
    """The request shows its code; Approve with cubes.reboot off gives the
    integration that maximum; its user named like the operator is
    proposed for the link, Link links it, and the operator's panel then
    names it. After Approve the panel waits for the key and follows its
    collection; an approval expired shows Register (v1.1, Block 8)."""
    instance = ("%04d%s" % (case.width, case.lang)).encode().hex().ljust(32, "0")[:32]
    subject = ("a1%04d%s" % (case.width, case.lang)).encode().hex().ljust(32, "b")[:32]
    status, req = raw_call("POST", "api/v1/integrations/requests",
                           {"kind": "homeassistant", "name": f"Casa {case.width} {case.lang}", "instance_id": instance,
                            "url": "http://192.0.2.20:8123", "version": "2026.11.1"})
    assert status == 201 and req["code"], "the request is taken"
    uuid = req["id"]
    try:
        show_view(page, "system")
        panel = page.locator(f".integration-request[data-uuid='{uuid}']")
        panel.wait_for()
        assert req["code"] in panel.inner_text(), "the request shows its code"
        assert panel.locator("summary .mono").inner_text() == req["code"], "the code in the panel's head"
        panel.get_by_role("checkbox", name=page.evaluate("t('cap.cubes.reboot')"), exact=True).uncheck(force=True)
        named_buttons(page, "System settings → Integrations, a request")
        panel.locator(".approve-integration").click()
        approved = page.locator(f".integration-panel[data-uuid='{uuid}']")
        approved.wait_for()
        sel = f".integration-panel[data-uuid='{uuid}']"
        page.locator(f"{sel} .integration-key-waiting").wait_for(state="attached")
        got = next(i for i in api_call("GET", "api/v1/integrations") if i["uuid"] == uuid)
        assert got["state"] == "active" and "cubes.reboot" not in got["max_caps"] and "setpoint" in got["max_caps"], \
            f"approved with the maximum chosen: {got}"

        status, key = raw_call("GET", f"api/v1/integrations/requests/{uuid}?code={req['code']}")
        assert status == 200 and key["key"], "the key collected"
        # AP IntegrationUsers v1.1, Block 8: the panel follows the collection
        assert wait_until(page, f"!document.querySelector(\"{sel} .integration-key-waiting\")", 8), \
            "the panel follows the collection"
        want = page.evaluate("(n) => t('integrations.collected', {name: n})", f"Casa {case.width} {case.lang}")
        assert page.locator("#integrations-msg").inner_text() == want, "and says so"
        approved = page.locator(sel)
        on_behalf = f"id={subject}; level=control; name=operator; instance={instance}"
        status, me = raw_call("GET", "api/v1/me", headers={"Authorization": "Bearer " + key["key"], HEADER: on_behalf})
        assert status == 200 and me["acts_for"]["id"] == subject, "the integration acts for its user"

        approved.locator("summary").click()
        row = approved.locator(f".integration-user[data-id='{subject}']")
        row.wait_for()
        select = row.locator("select")
        assert select.input_value() == "operator", "the operator proposed by name"
        named_buttons(page, "System settings → Integrations, an integration")
        assert scroll_width(page) <= case.width, "no horizontal scroll in System settings → Integrations"
        row.locator(".link-integration-user").click()
        assert wait_until(page, f"(() => {{ const b = document.querySelector(\".integration-panel[data-uuid='{uuid}'] "
                                f".integration-user[data-id='{subject}'] .flag.ok\"); return !!b; }})()", 5), "the user marked linked"
        users = api_call("GET", f"api/v1/integrations/{uuid}")["users"]
        assert any(u["id"] == subject and u["linked"] and u["user"] == "operator" for u in users), f"linked to the operator: {users}"

        show_view(page, "tiles")
        show_view(page, "system")
        op = page.locator(".user-panel[data-name='operator']")
        op.wait_for()
        assert wait_until(page, "!!document.querySelector(\".user-panel[data-name='operator'] .user-integration-links\")", 5), \
            "the operator's panel names the integration user linked to it"
        _rename(page, case, uuid, subject)
        _approval_expired(page, case, uuid)
    finally:
        # never acted, or not approved: nothing linked (a 404)
        cleanup(lambda: undo("the integration user's link",
                             lambda: api_call("PUT", f"api/v1/integrations/{uuid}/users/{subject}", {"user": None})),
                lambda: _end(uuid))


def _rename(page, case, uuid, subject):
    """The integration's name, set in its panel, is the name its users
    are shown under, by the chain "<name> › <user>, as <login name>";
    the name Home Assistant reports stays beside it (AP StoreRaise v1.2,
    Block 4)."""
    reported = f"Casa {case.width} {case.lang}"
    named = f"Home {case.width} {case.lang}"
    sel = f".integration-panel[data-uuid='{uuid}']"
    user_sel = f".user-panel[data-name='{uuid}:{subject}']"
    head = page.locator(f"{user_sel} > summary .panel-name")
    head.wait_for()
    assert head.inner_text() == f"{reported} › operator, as operator", f"the chain under the reported name: {head.inner_text()}"
    assert page.locator(f"{user_sel} > summary .mono").count() == 0, "no login name: the uuid and the id"
    if not page.locator(sel).evaluate("d => d.open"):
        page.locator(f"{sel} > summary").click()
    field = page.locator(f"#integration-name-{uuid}")
    save = page.locator(f"{sel} form.integration-name button[type=submit]")
    assert field.input_value() == reported and save.is_disabled(), "the name as stored, Save off"
    field.fill(named)
    assert save.is_enabled(), "Save on once the name differs"
    save.click()
    want = page.evaluate("(n) => t('integrations.nameSaved', {name: n})", named)
    assert wait_until(page, f"document.querySelector('#integrations-msg').textContent === {json.dumps(want)}", 5), "and says so"
    got = api_call("GET", f"api/v1/integrations/{uuid}")
    assert got["name"] == named and got["reported_name"] == reported, f"the name given, the one reported kept: {got}"
    panel = page.locator(sel)
    assert panel.locator("summary .panel-name").inner_text() == named, "the panel under its new name"
    assert reported in panel.locator("dl.kv").inner_text(), "the name Home Assistant reports, where it differs"
    show_view(page, "tiles")
    show_view(page, "system")
    assert wait_until(page, f"(() => {{ const h = document.querySelector(\"{user_sel} > summary .panel-name\"); "
                            f"return !!h && h.textContent === {json.dumps(named + ' › operator, as operator')}; }})()", 5), \
        "the user shown under the new name"
    named_buttons(page, "System settings → Integrations, renamed")
    assert scroll_width(page) <= case.width, "no horizontal scroll with an integration's users"


def _approval_expired(page, case, uuid):
    """An approval whose time passed without its key collected (the list
    stubbed, AP IntegrationUsers v1.1, Block 8) shows approval expired, in
    the panel's head and in it, with Register."""
    sel = f".integration-panel[data-uuid='{uuid}']"

    def expired(body):
        for i in body:
            if i.get("uuid") == uuid:
                i.pop("key_issued_at", None)
                i["request_expires"] = "2026-01-01T00:00:00Z"
        return body

    reads = reads_of(page)
    reads.change("**/api/v1/integrations", expired)
    try:
        show_view(page, "tiles")
        show_view(page, "system")
        notice = page.locator(f"{sel} .integration-approval-expired")
        notice.wait_for(state="attached")
        assert page.evaluate("t('integrations.approvalExpired')") in page.locator(f"{sel} summary").inner_text(), \
            "approval expired in the panel's head"
        if not page.locator(sel).evaluate("d => d.open"):
            page.locator(f"{sel} summary").click()
        _record_opens(page)
        notice.locator(".register-integration").click()
        assert page.evaluate("window.__opened") == ["http://192.0.2.20:8123/_my_redirect/config_flow_start?domain=massimo"], \
            "Register again"
        named_buttons(page, "System settings → Integrations, approval expired")
        assert scroll_width(page) <= case.width, "no horizontal scroll with approval expired"
    finally:
        reads.clear("**/api/v1/integrations")


def _end(uuid):
    """The case's request refused, or its integration revoked."""
    for i in api_call("GET", "api/v1/integrations"):
        if i["uuid"] != uuid:
            continue
        if i["state"] == "pending":
            api_call("POST", f"api/v1/integrations/{uuid}/refuse")
        elif i["state"] == "active":
            api_call("DELETE", f"api/v1/integrations/{uuid}")


def found_box(page):
    """System settings → Integrations, Seen on the LAN, opened afresh so
    that the card reads the found answer again."""
    show_view(page, "tiles")
    show_view(page, "system")
    box = page.locator("#integrations-found")
    box.locator("p").first.wait_for()
    return box


def test_why_the_server_does_not_look(page, case):
    """The test server listens on the loopback without -announce, so it
    does not look for Home Assistant on the LAN, and Seen on the LAN says
    why (AP LookReason v1.0): Automatic on the loopback; Off stored; On
    stored, which waits for the restart; where MASSIMO_ANNOUNCE decides,
    that, without the link. Open Maintenance opens the announcement's
    card. The setting is put back."""
    api_call("PUT", "api/v1/settings", {"announce": None})
    try:
        for stored, key in ((None, "foundLoopback"), ("off", "foundOff"), ("on", "foundRestart")):
            api_call("PUT", "api/v1/settings", {"announce": stored})
            box = found_box(page)
            want = page.evaluate(f"t('integrations.{key}')")
            assert box.locator("p").first.inner_text() == want, f"the text for {stored or 'auto'}"
            assert page.locator("#integrations-open-maint").is_visible(), f"Open Maintenance for {stored or 'auto'}"
            assert not page.locator("#integrations-look").is_visible(), "no Look again where the server does not look"
        named_buttons(page, "System settings → Integrations, not looking")
        assert scroll_width(page) <= case.width, "no horizontal scroll with the reason"
        page.locator("#integrations-open-maint").click()
        page.locator("#maint-page").wait_for(state="visible")
        assert wait_until(page, "document.querySelector('#sys-maint').getBoundingClientRect().top < innerHeight", 5), \
            "Maintenance opens with the announcement's card in view"
        assert page.locator("#announce-mode").is_visible(), "the choice there"
    finally:
        api_call("PUT", "api/v1/settings", {"announce": None})
        show_view(page, "tiles")


def test_why_not_where_the_variable_decides(page, case, reads):
    """Where MASSIMO_ANNOUNCE decides, Seen on the LAN names it, and no
    link leads to a choice Maintenance cannot change."""

    def fixed(body):
        body["announce"] = {"value": "auto", "in_effect": "off", "fixed_by": "MASSIMO_ANNOUNCE", "why": "fixed"}
        return body

    reads.change("**/api/v1/integrations/found*", fixed)
    try:
        box = found_box(page)
        want = page.evaluate("t('integrations.foundFixed', {by: 'MASSIMO_ANNOUNCE'})")
        assert box.locator("p").first.inner_text() == want and "MASSIMO_ANNOUNCE=off" in want, "the variable named"
        assert page.locator("#integrations-open-maint").count() == 0, "no link where Maintenance cannot change it"
    finally:
        show_view(page, "tiles")
        reads.clear("**/api/v1/integrations/found*")


def test_why_not_without_settings(page, case, reads):
    """A user with integrations but without settings (/me stubbed) reads
    why, without the link to a choice the user may not make."""

    def without_settings(body):
        body["capabilities"] = [c for c in body.get("capabilities") or [] if c != "settings"]
        return body

    reads.change("**/api/v1/me", without_settings)
    try:
        page.evaluate("history.replaceState(null, '', location.pathname)")
        page.reload()
        page.wait_for_selector(".room", timeout=10000)
        assert page.evaluate("state.me.mode === 'users' && !hasCap('settings') && hasCap('integrations')"), "integrations, not settings"
        box = found_box(page)
        assert box.locator("p").first.inner_text() == page.evaluate("t('integrations.foundLoopback')"), "the reason"
        assert page.locator("#integrations-open-maint").count() == 0, "no link without settings"
    finally:
        reads.clear("**/api/v1/me")
        page.evaluate("history.replaceState(null, '', location.pathname)")
        page.reload()
        page.wait_for_selector(".room", timeout=10000)


def _record_opens(page):
    """window.open recorded instead of opening a tab."""
    page.evaluate("() => { window.__opened = []; window.open = (u) => { window.__opened.push(String(u)); return null; }; }")


def test_seen_on_the_lan_register(page, case):
    """Seen on the LAN (AP IntegrationUsers v1.1, Block 8, found answer
    stubbed): an announced address is a link with a copy icon, and
    Register opens Home Assistant's page that adds Massimo; where none is
    announced, Register asks for it first, filled in with the IP address
    and Home Assistant's port, and refuses what is not an http(s) address."""

    def handle(route):
        route.fulfill(status=200, content_type="application/json", body=json.dumps({"looks": True, "found": [
            {"name": "ha-one", "location": "Casa Uno", "url": "http://192.0.2.21:8123", "address": "192.0.2.21"},
            {"name": "ha-two", "location": "Casa Due", "address": "192.0.2.22"},
        ]}))

    handle = safe_route(page, "**/api/v1/integrations/found*", handle)
    try:
        show_view(page, "tiles")
        show_view(page, "system")
        items = page.locator("#integrations-found .integrations-found-list li")
        items.nth(1).wait_for()
        one, two = items.nth(0), items.nth(1)
        assert one.locator("a").get_attribute("href") == "http://192.0.2.21:8123", "the announced address a link"
        assert one.locator(".copy-pair .copy-icon, .copy-pair button").count() >= 1, "with a copy icon"
        assert two.locator(".copy-pair").count() == 1 and two.locator("a").count() == 0, "the bare address copyable, no link"
        named_buttons(page, "System settings → Integrations, Seen on the LAN")
        assert scroll_width(page) <= case.width, "no horizontal scroll with Register"
        _record_opens(page)
        one.locator(".register-integration").click()
        assert page.evaluate("window.__opened") == ["http://192.0.2.21:8123/_my_redirect/config_flow_start?domain=massimo"], \
            "Register opens Home Assistant's page"
        two.locator(".register-integration").click()
        field = two.locator(".register-address input")
        field.wait_for()
        assert field.input_value() == "http://192.0.2.22:8123", "the address asked for, filled in"
        field.fill("ftp://192.0.2.22")
        two.locator(".register-open").click()
        assert two.locator(".register-address .bad").is_visible(), "an address not http(s) refused"
        field.fill("https://ha.example.org/")
        two.locator(".register-open").click()
        assert page.evaluate("window.__opened")[-1] == "https://ha.example.org/_my_redirect/config_flow_start?domain=massimo", \
            "the address given opens"
        assert scroll_width(page) <= case.width, "no horizontal scroll with the address asked for"
    finally:
        show_view(page, "tiles")
        page.unroute("**/api/v1/integrations/found*", handle)

