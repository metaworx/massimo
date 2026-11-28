# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings → Integrations (AP IntegrationUsers v1.0, Block 6), as
the owner: an integration asks to be registered, its request waits with
its code, the owner approves it with a narrower maximum, the integration
collects its key and acts for a user of its own, whom the owner links to
the operator, proposed by name; the users list then marks the link. Each
run of a case has an installation of its own (helpers.own_id), revoked
at the end: a revoked integration stays, disabled, and a request from
its installation would join it."""
import json
import ssl
import urllib.request

from conftest import TOKEN, URL, api_call
from helpers import (OPERATOR_PASSWORD, ApiReads, cleanup, focus_in_turn, frames, hold, named_buttons, own_id, owner_page, reads_of, safe_route,
                     scroll_width, show_view, taken, undo, wait_held, wait_until)
from test_admin_updates import out, stub_integrations
from test_login import sign_in

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
    collection; an approval expired shows Register (v1.1, Block 8); the
    Users head counts the integration's two users at once, without
    System settings shown again (AP AccessEnds, D10)."""
    # of this run's own: its next repeat on the server would join this one's integration, disabled
    instance, subject = own_id(f"{case.width}{case.lang}"), own_id(f"a1{case.width}{case.lang}")
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
        counted = "(n) => document.querySelector('#sys-users .section-badges').textContent === tn('roles.users', n)"
        before = len(api_call("GET", "api/v1/users"))
        assert wait_until(page, f"({counted})({before})", 5), "the Users head counts the users"
        panel.locator(".approve-integration").click()
        approved = page.locator(f".integration-panel[data-uuid='{uuid}']")
        approved.wait_for()
        # AP AccessEnds, D10: the integration's own user and its system user
        assert wait_until(page, f"({counted})({before + 2})", 5), "after Approve the Users head counts two more, System settings not shown again"
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
    that the card reads the found answer again, and that answer drawn.
    A `p` in the box is no sign of it: the part keeps the box, and the
    answer an earlier showing drew, through a new read; so the box is
    returned once the read this showing starts has been answered and
    drawn."""
    show_view(page, "tiles")
    with page.expect_request(lambda r: r.method == "GET" and "/api/v1/integrations/found" in r.url, timeout=10000) as asked:
        show_view(page, "system")
    answer = asked.value.response()
    assert answer is not None and answer.ok, "the found answer read"
    answer.finished()
    frames(page)
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


def test_nothing_of_the_lan_before_its_answer(page, case):
    """Seen on the LAN says nothing while the LAN's answer has not come,
    the list of integrations already in, nor offers Look again; once it
    comes, why the server does not look (AP LooseEnds, Block 3). Fails on
    the page before: the general "does not look", without its reason,
    showed as soon as the list was in."""
    held, sent = [], []
    hold(page, "**/api/v1/integrations/found*", "GET", held, sent)
    try:
        show_view(page, "system")
        assert wait_held(page, held, 10), "the LAN's answer held, the list and the users read before it"
        frames(page)
        box = page.locator("#integrations-found")
        assert box.inner_text() == "", f"nothing of the LAN before its answer: {box.inner_text()!r}"
        assert not page.locator("#integrations-look").is_visible(), "no Look again"
        held.pop().continue_()
        assert wait_until(page, "(document.querySelector('#integrations-found p') || {}).textContent === t('integrations.foundLoopback')", 5), \
            "the reason once the answer has come"
    finally:
        for route in held:
            route.continue_()
        page.unroute("**/api/v1/integrations/found*")
        show_view(page, "tiles")


def is_read(end):
    """A test of a GET whose path ends with end."""
    return lambda r: r.method == "GET" and r.url.split("?")[0].endswith(end)


def test_the_newest_read_of_the_integrations_wins(page, case):
    """Two showings of System settings whose reads of the integrations
    overlap, each answered with the server's list as it was at its
    request: a request asking to be registered (test_parts_guard.py's,
    put into the list, since the server takes ten requests from one
    address in ten minutes) comes between the first two reads and goes
    between the next two. The first read answered last leaves the
    second's list, with the request; answered in their order, the second
    read's list follows the first's, without it (AP LooseEnds, Block 3).
    Fails on the page before: the older answer, written last, took the
    request out."""
    from test_parts_guard import GUARD_REQ, REQ_STUB
    held = []
    asks = {"now": False}

    def handle(route):
        """Each GET of the list held, with the server's answer at its request, the request in it while it asks."""
        if route.request.method != "GET":
            route.fallback()
            return
        answer = route.fetch()
        body = answer.json() or []
        if asks["now"]:
            body = [REQ_STUB] + body
        held.append((route, json.dumps(body)))

    routed = safe_route(page, "**/api/v1/integrations", handle)
    request = page.locator(f".integration-request[data-uuid='{GUARD_REQ}']")

    def showing(n):
        """System settings shown again, its read of the list the n-th held."""
        show_view(page, "tiles")
        show_view(page, "system")
        assert wait_held(page, held, 10, n), f"read {n} of the list held"

    def answer(n, then):
        """The n-th held read answered, once the read its showing sends next, then, has been sent."""
        route, body = held[n - 1]
        with page.expect_request(then, timeout=5000):
            route.fulfill(status=200, content_type="application/json", body=body)
        frames(page)

    answered = set()
    try:
        showing(1)
        asks["now"] = True
        showing(2)
        answer(2, is_read("/api/v1/integrations/found"))
        answered.add(2)
        request.wait_for()
        answer(1, is_read("/api/v1/users"))
        answered.add(1)
        assert request.count() == 1, "the older read, answered last, leaves the newer list"
        showing(3)
        asks["now"] = False
        showing(4)
        answer(3, is_read("/api/v1/users"))
        answered.add(3)
        assert request.count() == 1, "still asking at the third read"
        answer(4, is_read("/api/v1/users"))
        answered.add(4)
        request.wait_for(state="detached")
    finally:
        page.unroute("**/api/v1/integrations", routed)
        for n, (route, body) in enumerate(held, 1):
            if n not in answered:
                route.fulfill(status=200, content_type="application/json", body=body)
        show_view(page, "tiles")


def test_the_newest_read_of_a_panel_wins(page, case, reads):
    """An integration's panel (test_parts_guard.py's integration, the list
    stubbed) opened, its read of the integration held, while System
    settings shown again reads it once more, as every load of the list
    does for an open panel: the opening's read answered last leaves the
    newer read's users (AP LooseEnds, Block 3). Fails on the page before:
    the older answer, written last, put its user back."""
    from test_parts_guard import GUARD_IG, IG_STUB, IG_USER
    old_user = dict(IG_USER, id="c" * 32, name="Old user")
    held = []

    def details(route):
        """The first read of the integration held with an older user, the others answered with the newer."""
        if not held:
            held.append(route)
            return
        route.fulfill(status=200, content_type="application/json", body=json.dumps(dict(IG_STUB, users=[IG_USER])))

    reads.change("**/api/v1/integrations", lambda body: [IG_STUB])
    routed = safe_route(page, f"**/api/v1/integrations/{GUARD_IG}", details)
    answered = []
    try:
        show_view(page, "system")
        panel = page.locator(f".integration-panel[data-uuid='{GUARD_IG}']")
        panel.locator("> summary").click()
        assert wait_held(page, held, 10), "the opening's read held"
        show_view(page, "tiles")
        show_view(page, "system")
        newer = panel.locator(f".integration-user[data-id='{IG_USER['id']}']")
        newer.wait_for()
        with taken(page, f"**/api/v1/integrations/{GUARD_IG}"):
            held[0].fulfill(status=200, content_type="application/json", body=json.dumps(dict(IG_STUB, users=[old_user])))
            answered.append(1)
        frames(page)
        assert panel.locator(f".integration-user[data-id='{old_user['id']}']").count() == 0, "the older read's user not drawn"
        assert newer.count() == 1, "the newer read's user stays"
    finally:
        if held and not answered:
            held[0].fulfill(status=200, content_type="application/json", body=json.dumps(dict(IG_STUB, users=[IG_USER])))
        page.unroute(f"**/api/v1/integrations/{GUARD_IG}", routed)
        reads.clear("**/api/v1/integrations")
        show_view(page, "tiles")


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
        assert focus_in_turn(page, "#integrations-found .integrations-found-list li:nth-child(2) .register-integration",
                             "#integrations-found .integrations-found-list li:nth-child(2) .register-address input"), \
            "the address asked for takes the focus in the drawing that shows it"
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


USERS_COUNTED = "(n) => document.querySelector('#sys-users .section-badges').textContent === tn('roles.users', n)"


def test_users_read_again_after_a_refusal_and_a_removal(page, case):
    """A request refused and an integration removed (both stubbed, so
    that nothing is registered): each time the Users head counts what
    the users' read gives, without System settings shown again, and the
    change's outcome is said while that read is still held (AP
    AccessEnds, D10). Fails where the reload read the integrations alone,
    and where the outcome waited for the users."""
    guard_ig, guard_req = stub_integrations(page)
    safe_route(page, f"**/api/v1/integrations/{guard_req}/refuse",
               lambda route: route.fulfill(status=200, content_type="application/json", body='{"ok": true}'))
    extra = {"n": 0}

    def more(body):
        # n users more than the server has, each a viewer of its own name
        return body + [dict(body[0], name=f"zz-extra-{k}", display=f"zz-extra-{k}", role="viewer", integration=None) for k in range(extra["n"])]

    reads_of(page).change("**/api/v1/users", more)
    held = []
    try:
        show_view(page, "system")
        before = len(api_call("GET", "api/v1/users"))
        assert wait_until(page, f"({USERS_COUNTED})({before})", 5), "the Users head counts the users"
        hold(page, "**/api/v1/users", "GET", held, [])
        for what, sel, ask, said in (("refused", f".integration-request[data-uuid='{guard_req}']", ".refuse-integration", "integrations.refused"),
                                     ("removed", f".integration-panel[data-uuid='{guard_ig}']", ".revoke-integration", "integrations.revoked")):
            extra["n"] += 1
            panel = page.locator(sel)
            if page.evaluate("(s) => document.querySelector(s).tagName === 'DETAILS' && !document.querySelector(s).open", sel):
                panel.locator("> summary").click()
            panel.locator(ask).click()
            panel.locator(".confirm-yes").click()
            assert wait_held(page, held), f"{what}: the users read again"
            words = page.evaluate(f"(n) => t({said!r}, {{name: n}})", "Guard request" if what == "refused" else "Guard home")
            assert wait_until(page, f"(() => {{ const m = document.getElementById('integrations-msg'); "
                                    f"return m.classList.contains('ok') && m.textContent === {json.dumps(words)}; }})()", 5), \
                f"{what}: the outcome said while the users' read is held"
            for route in held:
                route.fallback()
            held.clear()
            assert wait_until(page, f"({USERS_COUNTED})({before + extra['n']})", 5), f"{what}: the Users head counts what the read gave"
    finally:
        for route in held:
            route.fallback()
        page.unroute("**/api/v1/users")
    show_view(page, "tiles")


def test_an_older_users_answer_writes_nothing(page, case):
    """A request refused, the users' read that follows it held; an
    integration removed, whose users' read answers; then the older
    answer, which counts one user fewer, let go: the Users head still
    counts what the newer read gave (AP AccessEnds, D10). Fails where
    the users' read kept no number for its newest: the older answer
    wrote its list over the newer."""
    guard_ig, guard_req = stub_integrations(page)
    safe_route(page, f"**/api/v1/integrations/{guard_req}/refuse",
               lambda route: route.fulfill(status=200, content_type="application/json", body='{"ok": true}'))
    extra = {"n": 0}

    def more(body):
        return body + [dict(body[0], name=f"zz-extra-{k}", display=f"zz-extra-{k}", role="viewer", integration=None) for k in range(extra["n"])]

    reads_of(page).change("**/api/v1/users", more)
    held = []
    try:
        show_view(page, "system")
        before = len(api_call("GET", "api/v1/users"))
        assert wait_until(page, f"({USERS_COUNTED})({before})", 5), "the Users head counts the users"
        hold(page, "**/api/v1/users", "GET", held, [])
        for n, (sel, ask) in enumerate(((f".integration-request[data-uuid='{guard_req}']", ".refuse-integration"),
                                        (f".integration-panel[data-uuid='{guard_ig}']", ".revoke-integration")), 1):
            panel = page.locator(sel)
            if page.evaluate("(s) => document.querySelector(s).tagName === 'DETAILS' && !document.querySelector(s).open", sel):
                panel.locator("> summary").click()
            panel.locator(ask).click()
            panel.locator(".confirm-yes").click()
            assert wait_held(page, held, n=n), f"users read {n} held"
        extra["n"] = 2
        held[1].fallback()
        assert wait_until(page, f"({USERS_COUNTED})({before + 2})", 5), "the newer read counted"
        extra["n"] = 1
        with taken(page, lambda r: r.request.method == "GET" and r.url.endswith("/api/v1/users")):
            held[0].fallback()  # the older read answered
        frames(page)
        assert page.evaluate(f"({USERS_COUNTED})({before + 2})"), \
            f"the head still counts the newer read: {page.locator('#sys-users .section-badges').inner_text()!r}"
    finally:
        page.unroute("**/api/v1/users")
    show_view(page, "tiles")


def test_a_sign_out_during_the_reload_reads_no_users(new_context, case, watch, guard):
    """An integration's name saved (the integrations stubbed), the read
    of the integrations that follows held, a sign-out, the read let go:
    no read of the users goes out after the sign-out, neither then nor
    once the operator, who may not manage users, signs in (AP
    AccessEnds, D10). Fails where the reload asked who may manage users
    after its await without the session's number: the users were read
    with no key after the sign-out, and again with the operator's after
    the sign-in."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    guard_ig, _ = stub_integrations(page)
    users = []
    page.on("request", lambda r: users.append(r.headers.get("authorization", "")) if r.method == "GET" and r.url.endswith("/api/v1/users") else None)
    show_view(page, "system")
    panel = page.locator(f".integration-panel[data-uuid='{guard_ig}']")
    panel.wait_for()
    if not panel.evaluate("(d) => d.open"):
        panel.locator("> summary").click()
    page.locator(f"#integration-name-{guard_ig}").fill(f"Renamed {case.width} {case.lang}")
    lists = []
    hold(page, "**/api/v1/integrations", "GET", lists, [])
    panel.locator(".integration-name button[type=submit]").click()
    assert wait_held(page, lists), "the read that follows the save held"
    reads = ApiReads(page)
    mark = len(users)
    out(page)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "signed out"
    for route in lists:
        route.fallback()
    assert reads.answered(10), f"the page's reads answered: {reads.open}"
    assert users[mark:] == [], f"no users read after the sign-out: {users[mark:]}"
    sign_in(page, "operator", OPERATOR_PASSWORD)
    assert reads.answered(10), f"the operator's reads answered: {reads.open}"
    assert users[mark:] == [], f"none after the operator's sign-in: {users[mark:]}"

