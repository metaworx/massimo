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
