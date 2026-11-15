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
from helpers import named_buttons, scroll_width, show_view, wait_until

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
    names it."""
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
        panel.locator("input[name='cubes.reboot']").uncheck(force=True)
        named_buttons(page, "System settings → Integrations, a request")
        panel.locator(".approve-integration").click()
        approved = page.locator(f".integration-panel[data-uuid='{uuid}']")
        approved.wait_for()
        got = next(i for i in api_call("GET", "api/v1/integrations") if i["uuid"] == uuid)
        assert got["state"] == "active" and "cubes.reboot" not in got["max_caps"] and "setpoint" in got["max_caps"], \
            f"approved with the maximum chosen: {got}"

        status, key = raw_call("GET", f"api/v1/integrations/requests/{uuid}?code={req['code']}")
        assert status == 200 and key["key"], "the key collected"
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
    finally:
        try:
            api_call("PUT", f"api/v1/integrations/{uuid}/users/{subject}", {"user": None})
        except Exception:  # never acted, or not approved: nothing linked
            pass
        _end(uuid)


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


def test_why_not_where_the_variable_decides(page, case):
    """Where MASSIMO_ANNOUNCE decides, Seen on the LAN names it, and no
    link leads to a choice Maintenance cannot change."""

    def handle(route):
        try:
            resp = route.fetch()
            body = resp.json()
            body["announce"] = {"value": "auto", "in_effect": "off", "fixed_by": "MASSIMO_ANNOUNCE", "why": "fixed"}
            route.fulfill(response=resp, body=json.dumps(body))
        except Exception:  # Route is already handled, or the page closed
            pass

    page.route("**/api/v1/integrations/found*", handle)
    try:
        box = found_box(page)
        want = page.evaluate("t('integrations.foundFixed', {by: 'MASSIMO_ANNOUNCE'})")
        assert box.locator("p").first.inner_text() == want and "MASSIMO_ANNOUNCE=off" in want, "the variable named"
        assert page.locator("#integrations-open-maint").count() == 0, "no link where Maintenance cannot change it"
    finally:
        show_view(page, "tiles")
        page.unroute("**/api/v1/integrations/found*", handle)


def test_why_not_without_settings(page, case):
    """A user with integrations but without settings (/me stubbed) reads
    why, without the link to a choice the user may not make."""

    def handle(route):
        try:
            resp = route.fetch()
            body = resp.json()
            body["capabilities"] = [c for c in body.get("capabilities") or [] if c != "settings"]
            route.fulfill(response=resp, body=json.dumps(body))
        except Exception:  # Route is already handled, or the page closed
            pass

    page.route("**/api/v1/me", handle)
    try:
        page.evaluate("history.replaceState(null, '', location.pathname)")
        page.reload()
        page.wait_for_selector(".room", timeout=10000)
        assert page.evaluate("state.me.mode === 'users' && !hasCap('settings') && hasCap('integrations')"), "integrations, not settings"
        box = found_box(page)
        assert box.locator("p").first.inner_text() == page.evaluate("t('integrations.foundLoopback')"), "the reason"
        assert page.locator("#integrations-open-maint").count() == 0, "no link without settings"
    finally:
        page.unroute("**/api/v1/me", handle)
        page.evaluate("history.replaceState(null, '', location.pathname)")
        page.reload()
        page.wait_for_selector(".room", timeout=10000)
