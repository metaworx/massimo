# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings → Users and Roles as the owner (the test token is the
owner's): the operator run.sh creates is listed with the owner, the
roles with the six default ones."""

import json
import re

import pytest

from conftest import URL, api_call
from helpers import (KEY_COPY, NAME_BAD, NAME_TAKEN, assert_busy, cleanup, close_fold, copies, frames, grant_clipboard, hold, icon_after_last_line, named_buttons,
                     open_fold, safe_route, scroll_width, show_view, sign_out, undo, wait_held, wait_until)


def test_users_and_roles(page, case):
    """The operator is listed with its tokens, the roles with the
    operator's seven capabilities on, every button has a name, and nothing
    overflows."""
    show_view(page, "system")
    users = page.locator(".users-settings")
    assert users.is_visible(), "System settings → Users shown to the owner"
    assert wait_until(page, "document.querySelectorAll('.user-panel').length >= 2", 5), "the users are listed"
    assert page.locator(".user-panel[data-name='operator']").count() == 1, "the operator is listed"
    page.locator(".user-panel[data-name='operator'] summary").click()
    assert wait_until(page, "document.querySelectorAll(\".user-panel[data-name='operator'] .user-tokens li\").length >= 1", 5), \
        "the operator's tokens are listed"
    named_buttons(page, "System settings → Users")
    roles = page.locator(".roles-settings")
    assert roles.is_visible() and wait_until(page, "document.querySelectorAll('.role-panel').length >= 6", 5), "the roles are listed"
    page.locator(".role-panel[data-key='operator'] summary").click()
    assert page.locator(".role-panel[data-key='operator'] .cap-grid input:checked").count() == 7, "the operator's seven capabilities are on"
    named_buttons(page, "System settings → Roles")
    assert scroll_width(page) <= case.width, "no horizontal scroll in System settings → Users and Roles"
    show_view(page, "tiles")


PANEL_HEAD = """(sel) => {
    const s = document.querySelector(sel + ' > summary');
    const box = (q) => s.querySelector(q).getBoundingClientRect();
    const name = box('.panel-name'), key = box('.mono'), badges = box('.panel-badges');
    return {
        gap: key.left - name.right,
        sameLine: Math.abs(key.top - name.top) < 4,
        right: s.getBoundingClientRect().right - badges.right,
    };
}"""


def test_panel_heads(page, case):
    """A user's and a role's head (FoldHead of shared/fold.tsx): the name
    and the login or key apart, the badges at the right edge, and on the
    signed-in owner's own panel the you badge first."""
    show_view(page, "system")
    assert wait_until(page, "document.querySelectorAll('.user-panel').length >= 2 && "
                            "document.querySelectorAll('.role-panel').length >= 6", 5), "the users and roles are listed"
    for sel in (".user-panel[data-name='operator']", ".role-panel[data-key='operator']", ".user-panel[data-name='owner']"):
        geo = page.evaluate(PANEL_HEAD, sel)
        assert geo["gap"] >= 6 or not geo["sameLine"], f"{sel}: the name and the key apart ({geo})"
        assert geo["right"] <= 1, f"{sel}: the badges at the right edge ({geo})"
    first = page.locator(".user-panel[data-name='owner'] .panel-badges > :first-child")
    assert "ok" in (first.get_attribute("class") or ""), "you is the first of the owner's badges"
    show_view(page, "tiles")


INTEGRATION = "01a1e000-0000-7000-8000-000000000001"
PHONE = INTEGRATION + ":" + "a" * 32
INTEGRATION_USERS = [
    {"name": INTEGRATION + ":", "display_name": "Home", "display": "Home", "role": "integration", "disabled": False, "has_password": False,
     "auth_version": 1, "created_at": "2026-11-16T10:00:00Z", "updated_at": "2026-11-16T10:00:00Z",
     "may": {"manage": True, "password": True}, "integration": {"key": INTEGRATION, "uuid": INTEGRATION}},
    {"name": PHONE, "display_name": "Anna's phone", "display": "Home › Anna's phone, as operator", "role": "integration",
     "disabled": False, "has_password": False, "auth_version": 1, "created_at": "2026-11-16T10:00:00Z", "updated_at": "2026-11-16T10:00:00Z",
     "may": {"manage": True, "password": True},
     "integration": {"key": INTEGRATION, "uuid": INTEGRATION, "id": "a" * 32, "name": "Anna's phone", "linked_to": "operator"}},
]


def test_integration_users_by_their_chain(page, case, reads):
    """An integration's users (the list stubbed) are shown by their
    chain, "<the integration's name> › <the user's>, as <login name>",
    without their login name, the uuid and the id; the user they act as
    names them so (AP StoreRaise v1.2, Block 4)."""
    def answer(body):
        for u in body:
            if u["name"] == "operator":
                u["links"] = [{"integration": INTEGRATION, "id": "a" * 32, "name": "Anna's phone", "display": "Home › Anna's phone"}]
        return body + INTEGRATION_USERS

    reads.change("**/api/v1/users", answer)
    try:
        show_view(page, "system")
        phone = f".user-panel[data-name='{PHONE}']"
        assert wait_until(page, f"!!document.querySelector(\"{phone}\")", 5), "the integration's user listed"
        assert page.locator(f"{phone} > summary .panel-name").inner_text() == "Home › Anna's phone, as operator", "by its chain"
        assert page.locator(f"{phone} > summary .mono").count() == 0, "without its login name"
        assert page.evaluate("t('users.integrationUser')") in page.locator(f"{phone} > summary").inner_text(), "marked an integration's user"
        own = f".user-panel[data-name='{INTEGRATION}:']"
        assert page.locator(f"{own} > summary .panel-name").inner_text() == "Home", "the integration's own by its name"
        links = page.locator(".user-panel[data-name='operator'] .user-integration-links").text_content()
        assert "Home › Anna's phone" in links and INTEGRATION not in links, f"the operator's panel names it by its chain: {links}"
        named_buttons(page, "System settings → Users, an integration's users")
        assert scroll_width(page) <= case.width, "no horizontal scroll with an integration's users"
    finally:
        reads.clear("**/api/v1/users")
    show_view(page, "tiles")


PROVIDERS = [{"key": "cloud", "name": "Cloud"}]


def test_an_integrations_user_has_no_way_in(page, case, reads):
    """An integration's users (the list stubbed, a provider enabled) are
    offered no password, no new key and no identity to assign, and the
    line says why; one that holds a password and a key from before is
    offered the password's removal, which sends an empty one, and the
    key's Revoke; a regular user's panel offers all three as before (AP
    LooseEnds, D1)."""
    held = [dict(u, has_password=u["name"] == PHONE) for u in INTEGRATION_USERS]
    reads.change("**/api/v1/users", lambda body: body + held)
    reads.change("**/api/v1/login/providers", lambda body: PROVIDERS)
    held_key = {"id": 901, "kind": "api", "label": "from before", "created_at": "2026-11-16T10:00:00Z"}

    def keys_and_identities(route):
        # the stubbed users' identities none, the phone's user one key
        mine = "/tokens" in route.request.url and route.request.url.split("/tokens")[0].endswith("a" * 32)
        route.fulfill(status=200, content_type="application/json", body=json.dumps([held_key] if mine else []))

    sent = []

    def put_phone(route):
        if route.request.method != "PUT":
            route.fallback()
            return
        sent.append(route.request.post_data_json)
        route.fulfill(status=200, content_type="application/json", body=json.dumps(dict(held[1], has_password=False)))

    stubbed = f"**/api/v1/users/{INTEGRATION}*/**"
    routed = safe_route(page, stubbed, keys_and_identities)
    phone_path = re.compile(r"/api/v1/users/" + INTEGRATION + r"(%3A|:)" + "a" * 32 + r"$")
    put_routed = safe_route(page, phone_path, put_phone)
    try:
        show_view(page, "system")
        line = page.evaluate("t('users.integrationNoSignIn')")
        for name in (INTEGRATION + ":", PHONE, "operator"):
            panel = page.locator(f".user-panel[data-name='{name}']")
            panel.wait_for()
            panel.locator("> summary").click()
            regular = name == "operator"
            assert panel.locator("input[name='password']").count() == (1 if regular else 0), f"{name}: a password offered"
            assert panel.locator(".create-key").count() == (1 if regular else 0), f"{name}: a new key offered"
            assert panel.locator(f"[id='user-identity-provider-{name}']").count() == (1 if regular else 0), f"{name}: an identity to assign offered"
            said = panel.locator(".user-no-sign-in")
            assert said.count() == (0 if regular else 1), f"{name}: the line"
            if not regular:
                assert said.inner_text() == line, f"{name}: the line's words"
            assert panel.locator(".remove-password").count() == (1 if name == PHONE else 0), f"{name}: the held password's removal"
            # an integration's key is always held to the idle limit: no Keep while unused; the regular user's,
            # which the owner manages, has it (AP KeyLife, Block 3); its Change… has the end date alone (Block 4)
            assert panel.locator(".key-keep").count() == (1 if regular else 0), f"{name}: Create key's Keep while unused"
            if name == PHONE:
                row = panel.locator(f".user-tokens li[data-id='{held_key['id']}']")
                row.wait_for()
                assert row.locator(".revoke-token").count() == 1, "the held key's Revoke"
                row.locator(".change-key").click()
                form = row.locator("form.token-change")
                form.wait_for()
                assert form.locator(".token-end-input").count() == 1, "Change… of an integration's key: its end date"
                assert form.locator("input[name=idle_exempt]").count() == 0, "Change… of an integration's key: no Keep while unused"
                form.locator(".form-cancel").click()
                with page.expect_request(lambda r: r.method == "PUT" and phone_path.search(r.url)):
                    panel.locator(".remove-password").click()
                assert sent == [{"password": ""}], f"the removal sends an empty password: {sent}"
            assert panel.locator(".remove-user").count() == 1, f"{name}: still removable"
            panel.locator("> summary").click()
        named_buttons(page, "System settings → Users, an integration's users without a way in")
        assert scroll_width(page) <= case.width, "no horizontal scroll"
    finally:
        page.unroute(phone_path, put_routed)
        page.unroute(stubbed, routed)
        reads.clear("**/api/v1/users")
        reads.clear("**/api/v1/login/providers")
    show_view(page, "tiles")


def role_choice(panel):
    """The keys a panel's role choice offers, in its order."""
    return panel.locator("select[name='role'] option").evaluate_all("(os) => os.map((o) => o.value)")


def test_an_integrations_role_choice_stays_within_its_maximum(page, case, reads):
    """An integration's users (the list stubbed, the server's may.roles
    guest and integration) are offered those roles alone; one stored as
    owner, beyond its integration's maximum, is offered owner too,
    chosen, with the line that it acts within the maximum all the same,
    which goes while another role is chosen. The operator's own panel,
    as the server answers it, still offers owner and offers no
    integration, nor does Add a user (AP AccessEnds, D3 to D5). Fails
    on the page before: every role offered."""
    within = ["guest", "integration"]
    stubbed = [dict(INTEGRATION_USERS[0], may=dict(INTEGRATION_USERS[0]["may"], roles=within)),
               dict(INTEGRATION_USERS[1], role="owner", may=dict(INTEGRATION_USERS[1]["may"], roles=within, beyond=True))]
    reads.change("**/api/v1/users", lambda body: body + stubbed)
    held, routed = none_of_theirs(page)
    try:
        show_view(page, "system")
        line = page.evaluate("t('users.roleAboveMax')")
        own = page.locator(f".user-panel[data-name='{INTEGRATION}:']")
        own.wait_for()
        own.locator("> summary").click()
        assert role_choice(own) == within, f"the integration's own user: {role_choice(own)}"
        assert own.locator(f"[id='user-{INTEGRATION}:-role-hint']").count() == 0, "no line under a role within the maximum"
        phone = page.locator(f".user-panel[data-name='{PHONE}']")
        phone.locator("> summary").click()
        select = phone.locator("select[name='role']")
        assert role_choice(phone) == within + ["owner"], f"the stored owner kept: {role_choice(phone)}"
        assert select.input_value() == "owner", "the stored owner chosen"
        hint = phone.locator(f"[id='user-{PHONE}-role-hint']")
        assert hint.is_visible() and hint.inner_text() == line, "the line under it"
        assert select.get_attribute("aria-describedby") == f"user-{PHONE}-role-hint", "the choice described by the line"
        select.select_option("guest")
        hint.wait_for(state="detached")  # no line while a role within the maximum is chosen
        select.select_option("owner")
        hint.wait_for(state="visible")  # the line again with the stored owner chosen back
        operator = page.locator(".user-panel[data-name='operator']")
        operator.locator("> summary").click()
        offered = role_choice(operator)
        assert "owner" in offered and "integration" not in offered, f"the operator's own panel: {offered}"
        page.locator("#user-add").click()
        new = page.locator("#user-new-role")
        new.wait_for()
        offered = new.locator("option").evaluate_all("(os) => os.map((o) => o.value)")
        assert "integration" not in offered and "viewer" in offered, f"Add a user: {offered}"
        named_buttons(page, "System settings → Users, an integration's role choice")
        assert scroll_width(page) <= case.width, "no horizontal scroll"
        page.locator("#user-new .form-cancel").click()
    finally:
        page.unroute(held, routed)
        reads.clear("**/api/v1/users")
    show_view(page, "tiles")


def none_of_theirs(page):
    """The stubbed users' sessions, keys and identities, which an open
    panel reads, answered with none; the pattern and its handler."""
    pattern = f"**/api/v1/users/{INTEGRATION}*/**"
    return pattern, safe_route(page, pattern, lambda route: route.fulfill(status=200, content_type="application/json", body="[]"))


def test_a_chosen_role_stays_when_the_maximum_narrows(page, case, reads):
    """An integration's user offered operator, guest, viewer and
    integration (the list stubbed), operator chosen and not saved; the
    list read again with may.roles narrowed to guest and integration, as
    after a lower maximum: the choice still shows operator, with the line
    that the user acts within its maximum all the same, and Save stays
    on; the stored role chosen back, operator is offered no more (AP
    AccessEnds, D4). Fails where the choice kept the stored role alone:
    it showed nothing, while Save would send operator."""
    roles = {"now": ["operator", "guest", "viewer", "integration"]}
    reads.change("**/api/v1/users", lambda body: body + [dict(INTEGRATION_USERS[1], may=dict(INTEGRATION_USERS[1]["may"], roles=roles["now"]))])
    pattern, routed = none_of_theirs(page)
    try:
        show_view(page, "system")
        phone = page.locator(f".user-panel[data-name='{PHONE}']")
        phone.wait_for()
        phone.locator("> summary").click()
        select = phone.locator("select[name='role']")
        hint = phone.locator(f"[id='user-{PHONE}-role-hint']")
        select.select_option("operator")
        assert hint.count() == 0, "no line under a role within the maximum"
        roles["now"] = ["guest", "integration"]
        page.evaluate("() => MassimoParts.users.load()")  # the users read again, as a change in Integrations reads them
        hint.wait_for(state="visible")
        assert select.input_value() == "operator", f"the role chosen still shown: {select.input_value()!r}"
        assert role_choice(phone) == ["guest", "integration", "operator"], f"and offered: {role_choice(phone)}"
        assert hint.inner_text() == page.evaluate("t('users.roleAboveMax')"), "with the line"
        assert phone.locator("form button[type=submit]").is_enabled(), "Save still on"
        select.select_option("integration")
        hint.wait_for(state="detached")
        assert wait_until(page, f"[...document.querySelectorAll(\"[id='user-{PHONE}-role'] option\")].map((o) => o.value).join() === 'guest,integration'", 5), \
            f"the stored role chosen back: operator offered no more ({role_choice(phone)})"
    finally:
        page.unroute(pattern, routed)
        reads.clear("**/api/v1/users")
    show_view(page, "tiles")


def test_a_role_beyond_the_maximum_is_offered_no_more_once_saved(page, case, reads):
    """An integration's user stored as owner, beyond its maximum (the
    list stubbed, may.roles guest and integration, the PUT stubbed), given
    guest by Save: guest is sent, the list read again gives it, and owner
    is offered no more, nor the line (AP AccessEnds, D4). Fails on the
    page before: every role offered, owner among them."""
    stored = {"role": "owner"}
    within = ["guest", "integration"]

    def theirs():
        return dict(INTEGRATION_USERS[1], role=stored["role"], may=dict(INTEGRATION_USERS[1]["may"], roles=within))

    reads.change("**/api/v1/users", lambda body: body + [theirs()])
    sent = []

    def put(route):
        if route.request.method != "PUT":
            route.fallback()
            return
        sent.append(route.request.post_data_json)
        stored["role"] = sent[-1].get("role", stored["role"])
        route.fulfill(status=200, content_type="application/json", body=json.dumps(theirs()))

    phone_path = re.compile(r"/api/v1/users/" + INTEGRATION + r"(%3A|:)" + "a" * 32 + r"$")
    put_routed = safe_route(page, phone_path, put)
    pattern, routed = none_of_theirs(page)
    try:
        show_view(page, "system")
        phone = page.locator(f".user-panel[data-name='{PHONE}']")
        phone.wait_for()
        phone.locator("> summary").click()
        assert role_choice(phone) == within + ["owner"], f"the stored owner kept: {role_choice(phone)}"
        phone.locator("select[name='role']").select_option("guest")
        phone.locator("form button[type=submit]").click()
        assert wait_until(page, "document.getElementById('users-msg').classList.contains('ok')", 5), "saved"
        assert sent == [{"role": "guest"}], f"guest sent: {sent}"
        assert wait_until(page, f"[...document.querySelectorAll(\"[id='user-{PHONE}-role'] option\")].map((o) => o.value).join() === 'guest,integration'", 5), \
            f"owner offered no more: {role_choice(phone)}"
        assert phone.locator("select[name='role']").input_value() == "guest", "guest chosen, as stored"
        assert phone.locator(f"[id='user-{PHONE}-role-hint']").count() == 0, "no line"
    finally:
        page.unroute(pattern, routed)
        page.unroute(phone_path, put_routed)
        reads.clear("**/api/v1/users")
    show_view(page, "tiles")


def test_profile_of_an_integrations_user(page, case, reads):
    """My profile of an integration's user, in with a key it holds from
    before (/me stubbed), says it has no sign-in of its own in place of
    the password, lists its keys with Revoke and without a new one, and
    offers no Link; the regular owner first shows what is looked for
    (AP LooseEnds, D1)."""
    def as_integration(body):
        if body.get("user"):
            body["user"]["integration"] = {"key": INTEGRATION, "uuid": INTEGRATION, "id": "b" * 32, "name": "Anna"}
        return body

    reads.change("**/api/v1/login/providers", lambda body: PROVIDERS)
    # a key of the owner's beside the suite's, which is the settings
    # file's and has no Revoke (test_settings_file_key_has_no_revoke)
    other = api_call("POST", "api/v1/users/owner/tokens", {"label": f"ui-test-integration-profile-{case.width}-{case.lang}"})
    try:
        page.evaluate("history.replaceState(null, '', location.pathname)")
        page.reload()
        page.wait_for_selector(".room", timeout=10000)
        show_view(page, "profile")
        page.locator("#profile-identities button[data-provider]").first.wait_for()
        assert page.locator("#me-password").count() == 1, "the regular owner: the password offered"
        assert page.locator("#profile-tokens .create-key").count() == 1, "the regular owner: a new key offered"
        reads.change("**/api/v1/me", as_integration)
        page.evaluate("history.replaceState(null, '', location.pathname)")
        page.reload()
        page.wait_for_selector(".room", timeout=10000)
        show_view(page, "profile")
        said = page.locator("#me-no-sign-in")
        said.wait_for()
        assert said.inner_text() == page.evaluate("t('users.integrationNoSignIn')"), "the line"
        assert page.locator("#me-password").count() == 0, "no password offered"
        page.locator(f"#profile-tokens .user-tokens li[data-id='{other['id']}']").wait_for()
        assert page.locator(f"#profile-tokens .user-tokens li[data-id='{other['id']}'] .revoke-token").count() == 1, "its keys with Revoke"
        assert page.locator("#profile-tokens .create-key").count() == 0, "no new key offered"
        assert page.locator("#profile-identities button[data-provider]").count() == 0, "no Link offered"
    finally:
        api_call("DELETE", "api/v1/users/owner/tokens/%d" % other["id"])
        reads.clear("**/api/v1/me")
        reads.clear("**/api/v1/login/providers")
        page.evaluate("history.replaceState(null, '', location.pathname)")
        page.reload()
        page.wait_for_selector(".room", timeout=10000)
    show_view(page, "tiles")


def revoke_keys(prefix):
    """The operator's API keys whose label starts with prefix are
    revoked, so that the four cases leave the server as they found it."""
    for tk in api_call("GET", "api/v1/users/operator/tokens") or []:
        if (tk.get("label") or "").startswith(prefix):
            api_call("DELETE", "api/v1/users/operator/tokens/%d" % tk["id"])


def test_new_key_shown_once(page, case):
    """In an open user panel, Create key shows the new key, which the
    server shows only once, lists every token once, and sends one POST
    per click: the panel loads its tokens through one path, not through
    the toggle of a panel built open and a direct call both. The key
    shown copies as the page's other texts do (AP KeyLife, D1-A): a
    button to the keyboard named Copy the key, not by the key, with its
    copy icon right after the key's last line; a click, Enter, Space and
    the icon each copy it. Fails on the page before: the key a paragraph
    alone."""
    prefix = f"ui-test-key-{case.width}-{case.lang}-"
    revoke_keys(prefix)
    grant_clipboard(page)
    posts, loads = [], []
    page.on("request", lambda r: posts.append(r.url) if r.method == "POST" and r.url.endswith("/users/operator/tokens") else None)
    page.on("request", lambda r: loads.append(r.url) if r.method == "GET" and r.url.endswith("/users/operator/tokens") else None)
    show_view(page, "system")
    panel = page.locator(".user-panel[data-name='operator']")
    panel.wait_for()
    panel.locator("summary").click()
    tokens = ".user-panel[data-name='operator'] .user-tokens li"
    assert wait_until(page, "document.querySelectorAll(%r).length >= 1" % tokens, 5), "the operator's tokens are listed"
    try:
        for n in (1, 2):
            before = len(loads)
            panel.locator("input[name=label]").fill(prefix + str(n))
            panel.locator(".create-key").click()
            assert wait_until(page, "document.querySelectorAll('.user-panel[data-name=\"operator\"] li[data-id]').length > 0 && "
                                    "[...document.querySelectorAll(%r)].some((li) => li.textContent.includes(%r))" % (tokens, prefix + str(n)), 5), \
                f"key {n} is listed"
            # listed, the panel built open has drawn its load's answer; a second
            # load, the toggle's of the panel built open, would have been asked
            # for before it: counted, not waited for
            assert len(loads) - before == 1, f"one load of the tokens for key {n}, not {len(loads) - before}"
            key = panel.locator(".user-key-box")
            shown = panel.locator(".user-key").inner_text().strip()
            assert key.is_visible() and shown, f"key {n} is shown, once"
            sel = ".user-panel[data-name='operator'] .user-key"
            placed, m = icon_after_last_line(page, sel)
            assert placed, f"key {n}: its copy icon right after its last line: {m}"
            copies(page, sel, shown, KEY_COPY[case.lang])
            ids = page.evaluate("[...document.querySelectorAll(%r)].map((li) => li.dataset.id).filter((id) => id)" % tokens)
            assert len(ids) == len(set(ids)), f"every token once after key {n}: {ids}"
            assert len(posts) == n, f"one POST per click, {len(posts)} after {n}"
    finally:
        revoke_keys(prefix)
    show_view(page, "tiles")


def test_a_key_session_names_its_key(page, case):
    """A session made from a key (POST /api/v1/login/key, the apps' login
    by fingerprint) says in the user's token list which key made it, by
    the key's label; revoking the key ends the session with it."""
    prefix = f"ui-test-keysession-{case.width}-{case.lang}-"
    revoke_keys(prefix)
    key = api_call("POST", "api/v1/users/operator/tokens", {"label": prefix + "phone"})
    session = api_call("POST", "api/v1/login/key", {"label": prefix + "session"}, token=key["token"])
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='operator']")
        panel.wait_for()
        panel.locator("summary").click()
        row = panel.locator(".user-tokens li", has_text=prefix + "session")
        row.wait_for(timeout=5000)
        named = row.locator(".token-key")
        assert named.is_visible() and prefix + "phone" in named.inner_text(), "the session names the key that made it"
        assert panel.locator(".user-tokens li[data-id='%d'] .token-key" % key["id"]).is_hidden(), "the key itself names none"
    finally:
        # the key alone: its session goes with it
        api_call("DELETE", "api/v1/users/operator/tokens/%d" % key["id"])
        left = [tk for tk in api_call("GET", "api/v1/users/operator/tokens") or [] if (tk.get("label") or "").startswith(prefix)]
        revoke_keys(prefix)
    assert session["token"] and not left, f"revoking the key ended its session: {left}"
    show_view(page, "tiles")


MADE_BY = {"en": "made by the API key {label}", "de": "erstellt mit dem API-Schlüssel {label}"}
REVOKED_WITH_HEIRS = {"en": "Token {id} revoked, with the keys it made: {ids}.",
                      "de": "Token {id} widerrufen, mit den Schlüsseln, die damit erstellt wurden: {ids}."}


def test_a_key_made_by_a_key_names_its_maker(page, case):
    """A key made with another API key as the bearer (AP KeyLife, Block 2,
    D5-A) says in its row which key made it, by the maker's label where
    the list holds the maker, else by its id: the maker itself was made
    with the suite's token, the owner's settings-file key, which the
    operator's list does not hold. Revoking the maker says which keys
    went with it, and they are listed no more."""
    prefix = f"ui-test-heirs-{case.width}-{case.lang}-"
    revoke_keys(prefix)
    file_key = api_call("GET", "api/v1/me")["token"]["id"]
    maker = api_call("POST", "api/v1/users/operator/tokens", {"label": prefix + "maker"})
    heir = api_call("POST", "api/v1/users/operator/tokens", {"label": prefix + "heir"}, token=maker["token"])
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='operator']")
        panel.wait_for()
        panel.locator("summary").click()
        heir_row = panel.locator(f".user-tokens li[data-id='{heir['id']}']")
        heir_row.wait_for(timeout=5000)
        maker_row = panel.locator(f".user-tokens li[data-id='{maker['id']}']")
        assert heir_row.locator(".token-maker").inner_text() == MADE_BY[case.lang].format(label=prefix + "maker"), "the heir names its maker by its label"
        assert maker_row.locator(".token-maker").inner_text() == MADE_BY[case.lang].format(label="#%d" % file_key), "the maker names its own by its id"
        assert scroll_width(page) <= case.width, "no horizontal scroll"
        maker_row.locator(".revoke-token").click()
        said = REVOKED_WITH_HEIRS[case.lang].format(id=maker["id"], ids=heir["id"])
        assert wait_until(page, "document.getElementById('users-msg').textContent.includes(%r)" % said, 5), \
            f"the users' line names the heirs: {page.locator('#users-msg').text_content()}"
        assert wait_until(page, "!document.querySelector(%r)" % f".user-panel[data-name='operator'] .user-tokens li[data-id='{heir['id']}']", 5), \
            "the heir is listed no more"
    finally:
        revoke_keys(prefix)
    show_view(page, "tiles")


TOKEN_ID = {"en": "id {id}", "de": "ID {id}"}


def test_token_rows_show_their_ids(page, case):
    """Every row of a user's sessions and keys shows its id (AP MobileApp,
    D66-A), in System settings → Users as in My profile, the row one part;
    at 390 px in all six languages each row stays inside its list and the
    page has no horizontal scroll, the id making the row longer."""
    prefix = f"ui-test-ids-{case.width}-{case.lang}-"
    revoke_keys(prefix)
    key = api_call("POST", "api/v1/users/operator/tokens", {"label": prefix + "a label to fill the row at 390 px"})
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='operator']")
        panel.wait_for()
        panel.locator("summary").click()
        panel.locator(f".user-tokens li[data-id='{key['id']}']").wait_for(timeout=5000)
        ids = "(sel) => [...document.querySelectorAll(sel)].map((li) => [li.dataset.id, (li.querySelector('.token-id') || {}).textContent])"
        rows = page.evaluate(ids, ".user-panel[data-name='operator'] .user-tokens li[data-id]")
        assert rows and all(text == TOKEN_ID[case.lang].format(id=i) for i, text in rows), f"every row its id: {rows}"
        langs = ("en", "de", "fr", "it", "nl", "pl") if case.width == 390 else (case.lang,)
        for lang in langs:
            page.evaluate("async (l) => { await changeLanguage(l); applyLanguage(); }", lang)
            frames(page)
            m = page.evaluate("""() => [...document.querySelectorAll(".user-panel[data-name='operator'] .user-tokens li[data-id]")].map((li) => {
              const r = li.getBoundingClientRect(), list = li.parentElement.getBoundingClientRect();
              const id = li.querySelector('.token-id').getBoundingClientRect();
              return {id: li.dataset.id, inside: r.left >= list.left - 0.5 && r.right <= list.right + 0.5 && id.right <= list.right + 0.5};
            })""")
            assert m and all(x["inside"] for x in m), f"{lang}: every row and its id inside the list: {m}"
            assert scroll_width(page) <= case.width, f"no horizontal scroll in {lang}"
        page.evaluate("async (l) => { await changeLanguage(l); applyLanguage(); }", case.lang)
        show_view(page, "profile")
        page.locator("#profile-tokens li[data-id]").first.wait_for(timeout=5000)
        mine = page.evaluate(ids, "#profile-tokens li[data-id]")
        assert mine and all(text == TOKEN_ID[case.lang].format(id=i) for i, text in mine), f"My profile's rows their ids: {mine}"
    finally:
        revoke_keys(prefix)
    show_view(page, "tiles")


FROM_SETTINGS = {"en": "from the settings file: change or remove it there",
                 "de": "aus der Einstellungsdatei: dort ändern oder entfernen"}


def test_settings_file_key_has_no_revoke(page, case):
    """The settings file's key, the suite's token, says in its row that it
    comes from the settings file and offers no Revoke, in the owner's
    panel of System settings → Users as in My profile, where it is this
    device; another key of the owner's offers Revoke. The list's answer
    marks it (AP MobileApp v1.4, D86-A); the server's refusal of its
    revoke is TestSettingsFileKey's, so that a regression there does not
    revoke the suite's token."""
    prefix = f"ui-test-file-key-{case.width}-{case.lang}-"
    file_key = api_call("GET", "api/v1/me")["token"]["id"]
    other = api_call("POST", "api/v1/users/owner/tokens", {"label": prefix + "other"})
    try:
        marked = [tk["id"] for tk in api_call("GET", "api/v1/users/owner/tokens") or [] if tk.get("from_settings")]
        assert marked == [file_key], f"the list marks the settings file's key {file_key} alone: {marked}"
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='owner']")
        panel.wait_for()
        panel.locator("summary").click()
        for where, rows in (("System settings → Users", ".user-panel[data-name='owner'] .user-tokens"), ("My profile", "#profile-tokens .user-tokens")):
            if where == "My profile":
                show_view(page, "profile")
            row = page.locator(f"{rows} li[data-id='{file_key}']")
            row.wait_for(timeout=5000)
            page.locator(f"{rows} li[data-id='{other['id']}']").wait_for(timeout=5000)
            assert row.locator(".revoke-token").count() == 0, f"{where}: no Revoke on the settings file's key"
            assert row.locator(".token-settings").inner_text() == FROM_SETTINGS[case.lang], f"{where}: it says where it comes from"
            assert page.locator(f"{rows} li[data-id='{other['id']}'] .revoke-token").count() == 1, f"{where}: another key offers Revoke"
            assert page.locator(f"{rows} .token-settings").count() == 1, f"{where}: that row alone says so"
        assert scroll_width(page) <= case.width, "no horizontal scroll"
    finally:
        api_call("DELETE", "api/v1/users/owner/tokens/%d" % other["id"])
    show_view(page, "tiles")


KEY_IDLE_SAVED = {"en": "API keys now end after 30 days unused.", "de": "API-Schlüssel enden jetzt nach 30 Tagen ohne Nutzung."}


def test_key_idle_setting(page, case):
    """System settings → Users: API keys end unused after, the setting
    auth.key_idle_days (AP KeyLife, Block 3, D2-A), 0 by default, in days,
    with its line; a value typed is saved as the field is left, one PUT,
    the users' line saying so, and the next showing of the page reads it
    again; one out of 0 to 3650 is refused in the field, its limit saying
    so, and sends nothing; emptied, the field goes back to the value
    stored. The setting goes back to its default at the end."""
    puts = []
    page.on("request", lambda r: puts.append(r.post_data_json) if r.method == "PUT" and r.url.endswith("/api/v1/settings") else None)
    sel = "#key-idle-days"
    try:
        show_view(page, "system")
        field = page.locator(sel)
        field.wait_for()
        assert wait_until(page, "document.querySelector(%r).value === '0'" % sel, 5), f"0 by default: {field.input_value()}"
        row = page.locator(".field.key-idle")
        assert row.locator("label").inner_text() == page.evaluate("t('users.keyIdleDays')"), "its label"
        assert row.locator(".field-hint").inner_text() == page.evaluate("t('users.keyIdleHint')"), "its line"
        assert page.evaluate("t('users.keyIdleUnit')") in row.inner_text(), "in days"
        field.fill("4000")
        field.press("Tab")
        limit = page.locator(sel + "-limit")
        assert limit.is_visible() and field.get_attribute("aria-invalid") == "true", "out of 0 to 3650: refused in the field"
        field.fill("")
        field.press("Tab")
        assert wait_until(page, "document.querySelector(%r).value === '0'" % sel, 5), "emptied: back to the value stored"
        assert limit.is_hidden() and puts == [], f"nothing sent: {puts}"
        field.fill("30")
        with page.expect_request(lambda r: r.method == "PUT" and r.url.endswith("/api/v1/settings")):
            field.press("Tab")
        assert wait_until(page, "document.getElementById('users-msg').textContent.includes(%r)" % KEY_IDLE_SAVED[case.lang], 5), \
            f"the users' line: {page.locator('#users-msg').text_content()}"
        assert puts == [{"auth.key_idle_days": 30}], f"one PUT: {puts}"
        assert api_call("GET", "api/v1/settings")["auth.key_idle_days"]["value"] == "30", "stored"
        api_call("PUT", "api/v1/settings", {"auth.key_idle_days": 45})
        show_view(page, "tiles")
        show_view(page, "system")
        assert wait_until(page, "document.querySelector(%r).value === '45'" % sel, 5), "read again at the next showing"
        assert scroll_width(page) <= case.width, "no horizontal scroll"
    finally:
        api_call("PUT", "api/v1/settings", {"auth.key_idle_days": None})
    show_view(page, "tiles")


# the page's own form of an API time, fmtWhen of shared/time.ts
WHEN = """(iso) => new Intl.DateTimeFormat(i18n.lang, { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit",
  hour12: false }).format(new Date(iso))"""


def test_a_keys_life_line(page, case, reads):
    """Under the idle limit (AP KeyLife, Block 3; the list stubbed), a
    key's line says when it ends unless used, that it is kept while
    unused, or, ended, the badge Ended and since when it was unused; a key
    with none of them says nothing of its life; at 390 px in every
    language each row stays inside its list."""
    prefix = f"ui-test-life-{case.width}-{case.lang}-"
    revoke_keys(prefix)
    keys = {what: api_call("POST", "api/v1/users/operator/tokens", {"label": prefix + what}) for what in ("ends", "kept", "ended", "plain")}
    future, past = "2026-12-24T09:30:00Z", "2026-11-01T08:00:00Z"

    def life(body):
        for tk in body or []:
            if tk["id"] == keys["ends"]["id"]:
                tk["idle_ends_at"] = future
            elif tk["id"] == keys["kept"]["id"]:
                tk["idle_exempt"] = True
            elif tk["id"] == keys["ended"]["id"]:
                tk.update(ended="idle", idle_ends_at=past, last_used_at=past, last_used_from="192.0.2.7")
        return body
    pattern = "**/api/v1/users/operator/tokens"
    reads.change(pattern, life)
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='operator']")
        panel.wait_for()
        panel.locator("summary").click()
        row = lambda what: panel.locator(f".user-tokens li[data-id='{keys[what]['id']}']")
        row("ended").wait_for(timeout=5000)
        want = page.evaluate("""([future, past]) => {
          const at = """ + WHEN + """;
          return { ends: t('users.idleEnds', { date: at(future) }), kept: t('users.idleKept'), ended: t('users.ended'), since: t('users.endedIdle', { date: at(past) }) };
        }""", [future, past])
        assert row("ends").locator(".token-life").inner_text() == want["ends"], "ends unless used"
        assert row("kept").locator(".token-life").inner_text() == want["kept"], "kept while unused"
        assert row("ended").locator(".flag.bad").inner_text().strip() == want["ended"], "the badge Ended"
        assert row("ended").locator(".token-life").inner_text() == want["since"], "unused since"
        assert row("plain").locator(".token-life").count() == 0, "a key with none of them"
        langs = ("en", "de", "fr", "it", "nl", "pl") if case.width == 390 else (case.lang,)
        for lang in langs:
            page.evaluate("async (l) => { await changeLanguage(l); applyLanguage(); }", lang)
            frames(page)
            m = page.evaluate("""() => [...document.querySelectorAll(".user-panel[data-name='operator'] .user-tokens li[data-id]")].map((li) => {
              const r = li.getBoundingClientRect(), list = li.parentElement.getBoundingClientRect();
              return {id: li.dataset.id, inside: r.left >= list.left - 0.5 && r.right <= list.right + 0.5};
            })""")
            assert m and all(x["inside"] for x in m), f"{lang}: every row inside the list: {m}"
            assert scroll_width(page) <= case.width, f"no horizontal scroll in {lang}"
        page.evaluate("async (l) => { await changeLanguage(l); applyLanguage(); }", case.lang)
    finally:
        reads.clear(pattern)
        revoke_keys(prefix)
    show_view(page, "tiles")


def test_keep_while_unused(page, case):
    """For who manages the user, the owner of the operator (AP KeyLife,
    Block 3, D7-A): a key's Change… opens a small form, the focus on its
    switch Keep while unused, which shows the key's state, Save off until
    it is changed; Cancel sends nothing and gives the focus back to
    Change…; Save sends one PUT with idle_exempt, the line follows and the
    focus is back on Change…. Create key's switch sends idle_exempt, and
    the key made says it is kept. No Change… on the settings file's key,
    in the owner's panel nor in My profile, where it is this device."""
    prefix = f"ui-test-keep-{case.width}-{case.lang}-"
    revoke_keys(prefix)
    plain = api_call("POST", "api/v1/users/operator/tokens", {"label": prefix + "plain"})
    puts, posts = [], []
    page.on("request", lambda r: puts.append(r.post_data_json) if r.method == "PUT" and "/users/operator/tokens/" in r.url else None)
    page.on("request", lambda r: posts.append(r.post_data_json) if r.method == "POST" and r.url.endswith("/users/operator/tokens") else None)
    focused = "document.activeElement && document.activeElement.matches(%r)"
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='operator']")
        panel.wait_for()
        panel.locator("summary").click()
        row = panel.locator(f".user-tokens li[data-id='{plain['id']}']")
        row.wait_for(timeout=5000)
        change = row.locator(".change-key")
        assert change.is_visible() and change.get_attribute("aria-label") == page.evaluate("(l) => t('users.changeKeyLabel', { label: l })", prefix + "plain"), \
            "Change…, named by the key"
        change.click()
        form = row.locator("form.token-change")
        form.wait_for()
        switch = form.locator("input[name=idle_exempt]")
        assert not switch.is_checked(), "the switch shows the key held to the limit"
        assert wait_until(page, focused % ".token-change input[name=idle_exempt]", 3), "the focus on the switch"
        save = form.locator("button[type=submit]")
        assert save.is_disabled(), "Save off until it is changed"
        form.locator(".form-cancel").click()
        assert wait_until(page, "!document.querySelector(%r)" % f"li[data-id='{plain['id']}'] form.token-change", 3), "Cancel closes the form"
        assert wait_until(page, focused % f"li[data-id='{plain['id']}'] .change-key", 3), "the focus back on Change…"
        assert puts == [], f"Cancel sends nothing: {puts}"
        change.click()
        form.wait_for()
        form.locator("label.switch .slider").click()
        with page.expect_request(lambda r: r.method == "PUT" and "/users/operator/tokens/" in r.url):
            save.click()
        assert puts == [{"idle_exempt": True}], f"one PUT with idle_exempt: {puts}"
        kept = page.evaluate("t('users.idleKept')")
        assert wait_until(page, "(document.querySelector(%r) || {}).textContent === %r" % (f"li[data-id='{plain['id']}'] .token-life", kept), 5), "the line follows"
        assert wait_until(page, focused % f"li[data-id='{plain['id']}'] .change-key", 3), "the focus back on Change… after Save"
        assert api_call("GET", "api/v1/users/operator/tokens") and \
            [tk for tk in api_call("GET", "api/v1/users/operator/tokens") if tk["id"] == plain["id"]][0].get("idle_exempt"), "stored"
        # Create key's switch
        panel.locator("label.switch.key-keep .slider").click()
        panel.locator("input[name=label]").fill(prefix + "made kept")
        with page.expect_request(lambda r: r.method == "POST" and r.url.endswith("/users/operator/tokens")):
            panel.locator(".create-key").click()
        assert posts == [{"label": prefix + "made kept", "idle_exempt": True}], f"the switch sends idle_exempt: {posts}"
        made = panel.locator(".user-tokens li", has_text=prefix + "made kept")
        made.wait_for(timeout=5000)
        assert made.locator(".token-life").inner_text() == kept, "the key made is kept"
        assert not panel.locator("label.switch.key-keep input").is_checked(), "the switch off again for the next key"
        named_buttons(page, "System settings → Users, a key's Change…")
        assert scroll_width(page) <= case.width, "no horizontal scroll"
        # the settings file's key: no Change…, in the owner's panel and in My profile
        file_key = api_call("GET", "api/v1/me")["token"]["id"]
        owner = page.locator(".user-panel[data-name='owner']")
        owner.locator("summary").click()
        owner.locator(f".user-tokens li[data-id='{file_key}']").wait_for(timeout=5000)
        assert owner.locator(f".user-tokens li[data-id='{file_key}'] .change-key").count() == 0, "none on the settings file's key"
        show_view(page, "profile")
        page.locator(f"#profile-tokens li[data-id='{file_key}']").wait_for(timeout=5000)
        assert page.locator(f"#profile-tokens li[data-id='{file_key}'] .change-key").count() == 0, "none in My profile on this device's key"
    finally:
        revoke_keys(prefix)
    show_view(page, "tiles")


def test_no_keep_for_who_manages_nobody(new_context, case, watch, guard):
    """A user who manages nobody, a viewer, keeps the idle limit on its own
    keys (AP KeyLife, D7-A): My profile lists its key with Revoke, but
    offers no Keep while unused, at Create key nor in Change…, which has
    the key's end date alone, the user's own to move (Block 4, D4-A)."""
    from test_login import sign_in
    name = f"idle-{case.width}-{case.lang}"
    api_call("POST", "api/v1/users", {"name": name, "role": "viewer", "password": "pass-word-kv-71"})
    try:
        key = api_call("POST", f"api/v1/users/{name}/tokens", {"label": "a key of the viewer's"})
        ctx = new_context()
        page = ctx.new_page()
        guard.page = page
        page.set_default_timeout(5000)
        page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
        page.goto(URL)
        sign_in(page, name, "pass-word-kv-71")
        show_view(page, "profile")
        row = page.locator(f"#profile-tokens li[data-id='{key['id']}']")
        row.wait_for(timeout=5000)
        assert row.locator(".revoke-token").count() == 1, "its Revoke"
        assert page.locator("#profile-tokens .create-key").count() == 1, "Create key"
        assert page.locator("#profile-tokens .key-keep").count() == 0, "no Keep while unused"
        row.locator(".change-key").click()
        form = row.locator("form.token-change")
        form.wait_for()
        assert form.locator(".token-end-input").count() == 1, "Change…: the key's end date"
        assert form.locator("input[name=idle_exempt]").count() == 0, "Change…: no Keep while unused"
        # the user's own end date, moved signed in, is taken (D4-A)
        form.locator(".token-end-input").fill("2031-02-03T12:30")
        with page.expect_response(lambda r: r.request.method == "PUT" and f"/users/{name}/tokens/" in r.url and r.status == 200):
            form.locator("button[type=submit]").click()
        row.locator(".token-end").wait_for(timeout=5000)
        sign_out(page)
    finally:
        api_call("DELETE", "api/v1/users/" + name)


def test_a_keys_end_date(page, case, watch):
    """A key's end date (AP KeyLife, Block 4): Create key's field Ends,
    with its label and its line, sends expires_at, the browser's time as
    an API time, the key made saying when it ends and the field empty
    again; a key made with the field empty sends none. Change… shows the
    date, Save off until it is changed; a new date is one PUT and the line
    follows; one that has passed is refused by the server, the form
    staying open and the users' line saying why; No end empties the field
    and Save sends null, the line gone, and the key stored without an
    end."""
    prefix = f"ui-test-end-{case.width}-{case.lang}-"
    revoke_keys(prefix)
    puts, posts = [], []
    page.on("request", lambda r: puts.append(r.post_data_json) if r.method == "PUT" and "/users/operator/tokens/" in r.url else None)
    page.on("request", lambda r: posts.append(r.post_data_json) if r.method == "POST" and r.url.endswith("/users/operator/tokens") else None)
    iso = lambda local: page.evaluate("(l) => new Date(l).toISOString()", local)
    ends = lambda at: page.evaluate("(iso) => t('users.keyEnds', { date: (" + WHEN + ")(iso) })", at)
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='operator']")
        panel.wait_for()
        panel.locator("summary").click()
        field = panel.locator("#user-key-label-operator-end")
        field.wait_for()
        row_of = panel.locator(".field.key-end")
        assert row_of.locator("label").inner_text() == page.evaluate("t('users.keyEndsField')"), "the field's label"
        assert row_of.locator(".field-hint").inner_text() == page.evaluate("t('users.keyEndsHint')"), "its line"
        field.fill("2031-02-03T12:30")
        panel.locator("input[name=label]").fill(prefix + "dated")
        with page.expect_request(lambda r: r.method == "POST" and r.url.endswith("/users/operator/tokens")):
            panel.locator(".create-key").click()
        first = iso("2031-02-03T12:30")
        assert posts == [{"label": prefix + "dated", "expires_at": first}], f"Create key sends expires_at: {posts}"
        made = panel.locator(".user-tokens li", has_text=prefix + "dated")
        made.wait_for(timeout=5000)
        assert made.locator(".token-end").inner_text() == ends(first), "the key made says when it ends"
        assert wait_until(page, "document.querySelector('#user-key-label-operator-end').value === ''", 3), "the field empty again"
        panel.locator("input[name=label]").fill(prefix + "plain")
        with page.expect_request(lambda r: r.method == "POST" and r.url.endswith("/users/operator/tokens")):
            panel.locator(".create-key").click()
        assert posts[-1] == {"label": prefix + "plain"}, f"a key made with the field empty sends no end: {posts}"

        # Change… shows the date and moves it
        made.locator(".change-key").click()
        form = made.locator("form.token-change")
        form.wait_for()
        end = form.locator(".token-end-input")
        assert end.input_value() == "2031-02-03T12:30", "Change… shows the key's end"
        assert form.locator("label", has_text=page.evaluate("t('users.keyEndsField')")).count() == 1, "the field's label"
        save = form.locator("button[type=submit]")
        assert save.is_disabled(), "Save off until it is changed"
        end.fill("2032-03-04T13:00")
        with page.expect_request(lambda r: r.method == "PUT" and "/users/operator/tokens/" in r.url):
            save.click()
        second = iso("2032-03-04T13:00")
        assert puts == [{"expires_at": second}], f"one PUT with expires_at: {puts}"
        assert wait_until(page, "(document.querySelector(%r) || {}).textContent === %r" % (f"li[data-id='{made.get_attribute('data-id')}'] .token-end", ends(second)), 5), \
            "the line follows"

        # a date that has passed: refused, the form left open; the refusal is on purpose
        watch.expected.append((f"/api/v1/users/operator/tokens/{made.get_attribute('data-id')}", 400))
        made.locator(".change-key").click()
        form.wait_for()
        end.fill("2020-01-01T00:00")
        with page.expect_response(lambda r: r.request.method == "PUT" and "/users/operator/tokens/" in r.url and r.status == 400):
            save.click()
        assert wait_until(page, "document.getElementById('users-msg').textContent.includes('has passed')", 5), \
            f"the users' line says why: {page.locator('#users-msg').text_content()}"
        assert form.is_visible(), "the form left open"

        # No end: the field emptied, null sent, the line gone
        form.locator(".token-no-end").click()
        assert end.input_value() == "", "No end empties the field"
        with page.expect_request(lambda r: r.method == "PUT" and "/users/operator/tokens/" in r.url):
            save.click()
        assert puts[-1] == {"expires_at": None}, f"No end sends null: {puts}"
        assert wait_until(page, "!document.querySelector(%r)" % f"li[data-id='{made.get_attribute('data-id')}'] .token-end", 5), "the line gone"
        stored = [tk for tk in api_call("GET", "api/v1/users/operator/tokens") if (tk.get("label") or "") == prefix + "dated"]
        assert stored and "expires_at" not in stored[0], f"stored without an end: {stored}"
        named_buttons(page, "System settings → Users, a key's Change… with its end")
        assert scroll_width(page) <= case.width, "no horizontal scroll"
    finally:
        revoke_keys(prefix)
    show_view(page, "tiles")


def test_a_keys_end_line(page, case, reads):
    """A key's line of its end date (AP KeyLife, Block 4; the list
    stubbed): ends … where it has one, beside ends … unless used only
    where that comes first, and kept while unused as before; past it, the
    badge Ended and ended …; at 390 px in every language each row, its
    Change… form open, stays inside its list."""
    prefix = f"ui-test-endline-{case.width}-{case.lang}-"
    revoke_keys(prefix)
    keys = {what: api_call("POST", "api/v1/users/operator/tokens", {"label": prefix + what}) for what in ("dated", "idle first", "kept", "expired")}
    end, sooner, later, past = "2026-12-24T09:30:00Z", "2026-12-10T08:00:00Z", "2027-01-15T08:00:00Z", "2026-11-01T08:00:00Z"

    def life(body):
        for tk in body or []:
            if tk["id"] == keys["dated"]["id"]:
                tk.update(expires_at=end, idle_ends_at=later)
            elif tk["id"] == keys["idle first"]["id"]:
                tk.update(expires_at=end, idle_ends_at=sooner)
            elif tk["id"] == keys["kept"]["id"]:
                tk.update(expires_at=end, idle_exempt=True)
            elif tk["id"] == keys["expired"]["id"]:
                tk.update(ended="expired", expires_at=past)
        return body
    pattern = "**/api/v1/users/operator/tokens"
    reads.change(pattern, life)
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='operator']")
        panel.wait_for()
        panel.locator("summary").click()
        row = lambda what: panel.locator(f".user-tokens li[data-id='{keys[what]['id']}']")
        row("expired").wait_for(timeout=5000)
        want = page.evaluate("""([end, sooner, past]) => {
          const at = """ + WHEN + """;
          return { ends: t('users.keyEnds', { date: at(end) }), idle: t('users.idleEnds', { date: at(sooner) }), kept: t('users.idleKept'),
                   ended: t('users.ended'), past: t('users.endedExpired', { date: at(past) }) };
        }""", [end, sooner, past])
        assert row("dated").locator(".token-end").inner_text() == want["ends"], "ends …"
        assert row("dated").locator(".token-life").count() == 0, "no idle end after the end date"
        assert row("idle first").locator(".token-end").inner_text() == want["ends"], "ends …, the idle end first"
        assert row("idle first").locator(".token-life").inner_text() == want["idle"], "ends … unless used, before the end date"
        assert row("kept").locator(".token-end").inner_text() == want["ends"], "ends …, kept while unused"
        assert row("kept").locator(".token-life").inner_text() == want["kept"], "kept while unused"
        assert row("expired").locator(".flag.bad").inner_text().strip() == want["ended"], "the badge Ended"
        assert row("expired").locator(".token-life").inner_text() == want["past"], "ended …"
        assert row("expired").locator(".token-end").count() == 0, "no ends … past it"
        row("dated").locator(".change-key").click()
        row("dated").locator("form.token-change").wait_for()
        langs = ("en", "de", "fr", "it", "nl", "pl") if case.width == 390 else (case.lang,)
        for lang in langs:
            page.evaluate("async (l) => { await changeLanguage(l); applyLanguage(); }", lang)
            frames(page)
            m = page.evaluate("""() => [...document.querySelectorAll(".user-panel[data-name='operator'] .user-tokens li[data-id]")].map((li) => {
              const r = li.getBoundingClientRect(), list = li.parentElement.getBoundingClientRect();
              return {id: li.dataset.id, inside: r.left >= list.left - 0.5 && r.right <= list.right + 0.5};
            })""")
            assert m and all(x["inside"] for x in m), f"{lang}: every row inside the list: {m}"
            assert scroll_width(page) <= case.width, f"no horizontal scroll in {lang}"
        page.evaluate("async (l) => { await changeLanguage(l); applyLanguage(); }", case.lang)
    finally:
        reads.clear(pattern)
        revoke_keys(prefix)
    show_view(page, "tiles")


# a datetime field half typed: it reads as empty, as one emptied on purpose does
HALF = "(el) => el.validity.badInput && el.value === ''"
FOCUSED = "document.activeElement && document.activeElement.matches(%r)"


def test_a_half_typed_end_makes_no_key(page, case):
    """At Create key, a date half typed in Ends, which reads as empty, makes
    no key (AP KeyLife, Block 4): the users' line says that the date is not
    complete and the focus goes back into the field. On the page before, it
    made a key without an end date."""
    prefix = f"ui-test-halfmake-{case.width}-{case.lang}-"
    revoke_keys(prefix)
    posts = []
    page.on("request", lambda r: posts.append(r.post_data_json) if r.method == "POST" and r.url.endswith("/users/operator/tokens") else None)
    said = "document.getElementById('users-msg').textContent.includes(%r)" % page.evaluate("t('users.keyEndBad')")
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='operator']")
        panel.wait_for()
        panel.locator("summary").click()
        field = panel.locator("#user-key-label-operator-end")
        field.wait_for()
        field.focus()
        page.keyboard.type("02")
        assert field.evaluate(HALF), "Create key's date half typed"
        panel.locator("input[name=label]").fill(prefix + "half")
        panel.locator(".create-key").click()
        assert wait_until(page, said, 5), f"the users' line: {page.locator('#users-msg').text_content()}"
        assert wait_until(page, FOCUSED % "#user-key-label-operator-end", 3), "the focus in Create key's field"
        assert posts == [], f"a date half typed makes no key: {posts}"
    finally:
        revoke_keys(prefix)
    show_view(page, "tiles")


def test_a_half_typed_end_is_refused(page, case):
    """In Change… of a key with an end date, a date half typed, which reads
    as empty as No end does, is no choice of none (AP KeyLife, Block 4):
    Save sends nothing, the users' line says that the date is not complete
    and the focus goes back into the field, the key's end kept; No end then
    empties the field, and Save sends null. On the page before, the
    browser's own check of the form held the save, with its bubble alone."""
    prefix = f"ui-test-halfend-{case.width}-{case.lang}-"
    revoke_keys(prefix)
    dated = api_call("POST", "api/v1/users/operator/tokens", {"label": prefix + "dated", "expires_at": "2031-02-03T11:30:00Z"})
    puts = []
    page.on("request", lambda r: puts.append(r.post_data_json) if r.method == "PUT" and "/users/operator/tokens/" in r.url else None)
    said = "document.getElementById('users-msg').textContent.includes(%r)" % page.evaluate("t('users.keyEndBad')")
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='operator']")
        panel.wait_for()
        panel.locator("summary").click()
        row = panel.locator(f".user-tokens li[data-id='{dated['id']}']")
        row.wait_for(timeout=5000)
        row.locator(".change-key").click()
        form = row.locator("form.token-change")
        form.wait_for()
        end = form.locator(".token-end-input")
        assert end.input_value() != "", "Change… shows the key's end"
        end.focus()
        page.keyboard.press("Backspace")
        assert end.evaluate(HALF), "a part of the date cleared: half typed, reading as empty"
        save = form.locator("button[type=submit]")
        save.click()
        assert wait_until(page, said, 5), f"the users' line: {page.locator('#users-msg').text_content()}"
        assert wait_until(page, FOCUSED % f"li[data-id='{dated['id']}'] .token-end-input", 3), "the focus back in the field"
        assert puts == [], f"a date half typed sends nothing: {puts}"
        stored = [tk for tk in api_call("GET", "api/v1/users/operator/tokens") if tk["id"] == dated["id"]]
        assert stored and stored[0].get("expires_at") == "2031-02-03T11:30:00Z", f"the key's end kept: {stored}"
        form.locator(".token-no-end").click()
        assert end.evaluate("(el) => !el.validity.badInput && el.value === ''"), "No end empties the field whole"
        with page.expect_request(lambda r: r.method == "PUT" and "/users/operator/tokens/" in r.url):
            save.click()
        assert puts == [{"expires_at": None}], f"No end sends null: {puts}"
    finally:
        revoke_keys(prefix)
    show_view(page, "tiles")


NO_ANSWER = {"en": "no answer from the server; it may have been done all the same",
             "de": "keine Antwort vom Server; es wurde vielleicht trotzdem ausgeführt"}


@pytest.mark.clock
def test_revoke_ends_after_the_limit(page, case, watch):
    """A Revoke the server never answers, as over a link that stalls (AP
    MobileApp, F32, D82-A): its button turns until the page's limit of
    about 20 seconds, Playwright's clock run forward to it, and then no
    longer, the users' line saying that no answer came and that it may
    have been done all the same. The routes that wait longer by design
    have limits a little above the server's own (waitOf)."""
    limits = page.evaluate("""() => [
      ["PUT", "api/v1/cubes/KEQ0000100/devices/0a0101"], ["DELETE", "api/v1/cubes/KEQ0000100/devices/0a0101"],
      ["PUT", "api/v1/cubes/KEQ0000100/rooms/1/config"], ["PUT", "api/v1/cubes/KEQ0000100/rooms/1/program"],
      ["PUT", "api/v1/cubes/KEQ0000100/rooms/1/setpoint"], ["POST", "api/v1/cubes"],
      ["POST", "api/v1/cubes/KEQ0000100/rooms/1/repair/reset?plan=1"], ["POST", "api/v1/cubes/KEQ0000100/repair/reset?plan=1"],
      ["POST", "api/v1/cubes/KEQ0000100/events/ticket"], ["POST", "api/v1/cubes/discover"],
      ["GET", "api/v1/update?refresh=1"], ["POST", "api/v1/update"], ["GET", "api/v1/integrations/found?refresh=1"],
      ["POST", "api/v1/login/oidc/finish"], ["GET", "api/v1/cubes"], ["GET", "api/v1/cubes/KEQ0000100/rooms?devices=all"],
      ["DELETE", "api/v1/users/operator/tokens/3"],
    ].map(([m, p]) => waitOf(m, p))""")
    assert limits == [330000, 330000, 330000, 330000, 330000, 330000, 630000, 630000, 20000, 90000, 75000, 75000, 45000, 45000,
                      20000, 20000, 20000], f"the limits: {limits}"
    prefix = f"ui-test-limit-{case.width}-{case.lang}-"
    revoke_keys(prefix)
    key = api_call("POST", "api/v1/users/operator/tokens", {"label": prefix + "key"})
    path = "**/api/v1/users/operator/tokens/%d" % key["id"]
    held, sent = [], []
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='operator']")
        panel.wait_for()
        panel.locator("summary").click()
        button = f".user-panel[data-name='operator'] .user-tokens li[data-id='{key['id']}'] .revoke-token"
        page.locator(button).wait_for(timeout=5000)
        hold(page, path, "DELETE", held, sent)
        page.locator(button).click()
        assert wait_held(page, held), "the revoke is sent"
        assert_busy(page, button, "Revoke")
        page.clock.run_for(19_000)
        assert_busy(page, button, "Revoke before the limit")
        page.clock.run_for(2_000)
        assert wait_until(page, "!document.querySelector(%r).classList.contains('busy')" % button, 5), "Revoke turns no more after the limit"
        assert not page.locator(button).is_disabled(), "and is on again"
        line = page.locator("#users-msg")
        assert wait_until(page, "document.getElementById('users-msg').textContent.includes(%r)" % NO_ANSWER[case.lang], 5), \
            f"the users' line says why: {line.text_content()}"
    finally:
        page.unroute(path)
        revoke_keys(prefix)
    show_view(page, "tiles")


def remove_user(name):
    """A user the test made, gone again; none is fine (a 404)."""
    undo(f"the user {name}", lambda: api_call("DELETE", "api/v1/users/" + name))


def test_an_unchanged_end_date_is_not_sent(page, case):
    """A user with an end date whose display name alone changes is saved
    without the end date: the form's minute and the server's time with
    its seconds and offset are the same end; Save is off while nothing
    differs, so nothing is sent. The test makes the guest and removes it."""
    name = f"end-{case.width}-{case.lang}"
    remove_user(name)
    api_call("POST", "api/v1/users", {"name": name, "role": "guest", "password": "end-pass", "expires_at": "2099-01-02T10:20:30+01:00"})
    puts = []
    page.on("request", lambda r: puts.append(r.post_data_json) if r.method == "PUT" and r.url.endswith("/users/" + name) else None)
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='%s']" % name)
        panel.wait_for()
        panel.locator("summary").click()
        save = panel.locator("form button[type=submit]")
        assert save.is_disabled(), "Save is off while nothing differs from the user"
        panel.locator("input[name=display_name]").fill("Guest")
        assert save.is_enabled(), "a change turns Save on"
        panel.locator("input[name=display_name]").fill("")
        assert save.is_disabled(), "Save is off again once the change is undone"
        page.wait_for_timeout(300)
        assert not puts, f"nothing is sent without a change: {puts}"
        panel.locator("input[name=display_name]").fill("Guest with an end")
        # the save reads the list again, then the open panel's sessions and keys: waited for, so
        # that the cleanup's removal does not come before that read, which then meets a 404
        with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith(f"/users/{name}/tokens")):
            save.click()
        assert wait_until(page, "document.querySelector(\".user-panel[data-name='%s'] .panel-name\").textContent === 'Guest with an end'" % name, 5), \
            "the display name is saved"
        assert puts == [{"display_name": "Guest with an end"}], f"the display name alone is sent: {puts}"
    finally:
        remove_user(name)
    show_view(page, "tiles")


def test_a_double_click_on_yes_removes_once(page, case):
    """Remove asks in the panel, and a double click on its Yes sends one
    DELETE: Yes is off while the removal runs, so the message says the
    user is removed, not that a second DELETE found nobody. The test
    makes the user."""
    name = f"rm-{case.width}-{case.lang}"
    remove_user(name)
    api_call("POST", "api/v1/users", {"name": name, "role": "viewer", "password": "rm-password"})
    deletes = []
    page.on("request", lambda r: deletes.append(r.url) if r.method == "DELETE" and r.url.endswith("/users/" + name) else None)
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='%s']" % name)
        panel.wait_for()
        panel.locator("summary").click()
        panel.locator(".remove-user").click()
        box = panel.locator(".confirm-box")
        assert box.is_visible() and panel.locator(".remove-user").count() == 0, "Remove asks in the panel"
        box.locator(".confirm-no").click()
        assert panel.locator(".confirm-box").count() == 0 and \
            page.evaluate("document.activeElement && document.activeElement.classList.contains('remove-user')"), \
            "Keep takes the question back and the focus to Remove"
        panel.locator(".remove-user").click()
        panel.locator(".confirm-box .confirm-yes").dblclick()
        assert wait_until(page, "!document.querySelector(\".user-panel[data-name='%s']\")" % name, 5), "the user is removed"
        page.wait_for_timeout(300)
        assert len(deletes) == 1, f"one DELETE for a double click: {len(deletes)}"
        assert "ok" in page.locator("#users-msg").get_attribute("class"), "the message says it worked"
    finally:
        remove_user(name)
    show_view(page, "tiles")


def test_a_refused_removal_says_the_servers_words(page, case, watch):
    """A removal the server refuses (409) says the server's words in the
    users' line, as a failure, and the user's panel stays, Remove in place
    of the question again: here the words for the user that holds the
    settings file's key (AP MobileApp v1.4, D89-A). The test makes the
    user and answers its DELETE itself, so that nothing is removed and the
    suite's own owner, which holds that key, is never asked for; the
    server's refusal is TestSettingsFileKeyHolder's."""
    name = f"held-{case.width}-{case.lang}"
    remove_user(name)
    api_call("POST", "api/v1/users", {"name": name, "role": "viewer", "password": "kept-pass"})
    words = f"user {name} holds the settings file's MASSIMO_API_TOKEN: change or remove it in the settings file first"
    path = "**/api/v1/users/" + name
    deletes = []

    def refuse(route):
        if route.request.method == "DELETE":
            deletes.append(route.request.url)
            route.fulfill(status=409, content_type="application/json", body=json.dumps({"error": words}))
        else:
            route.continue_()

    watch.expected.append(("/api/v1/users/" + name, 409))
    page.route(path, refuse)
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='%s']" % name)
        panel.wait_for()
        panel.locator("summary").click()
        panel.locator(".remove-user").click()
        panel.locator(".confirm-box .confirm-yes").click()
        line = page.locator("#users-msg")
        assert wait_until(page, "document.getElementById('users-msg').textContent.includes(%r)" % words, 5), \
            f"the users' line says the server's words: {line.text_content()}"
        assert line.text_content() == f"{name}: {words}", f"after the user's name, as the page's other failures: {line.text_content()}"
        assert "bad" in line.get_attribute("class").split(), "as a failure"
        assert len(deletes) == 1, f"one DELETE: {deletes}"
        assert panel.count() == 1 and panel.locator(".confirm-box").count() == 0 and panel.locator(".remove-user").is_visible(), \
            "the user's panel stays, with Remove in place of the question"
        assert scroll_width(page) <= case.width, "no horizontal scroll with the long words"
    finally:
        page.unroute(path)
        remove_user(name)
    show_view(page, "tiles")


def test_create_is_busy_until_answered(page, case):
    """Create is off with a spinner from its click until the server
    answers, so that a second click or Enter sends nothing: on the Pi 1
    the hashing took 4.6 s and a second click was refused as "user
    exists". The test holds the POST, then lets it go."""
    name = f"busy-{case.width}-{case.lang}"
    remove_user(name)
    held, posts = [], []
    hold(page, "**/api/v1/users", "POST", held, posts)
    try:
        show_view(page, "system")
        page.locator("#user-add").click()
        form = page.locator("#user-new form")
        form.locator("input[name=name]").fill(name)
        form.locator("input[name=password]").fill("slow-pass-1")
        form.locator("button[type=submit]").click()
        assert wait_held(page, held), "the POST is sent"
        assert_busy(page, "#user-new button[type=submit]", "Create")
        form.locator("button[type=submit]").click(force=True)
        form.locator("input[name=name]").press("Enter")
        page.wait_for_timeout(300)
        assert len(posts) == 1, f"one POST while the first is unanswered: {len(posts)}"
        for route in held:
            route.continue_()
        assert wait_until(page, "!document.querySelector('#user-new form') && document.getElementById('users-msg').classList.contains('ok')", 5), \
            "the user is created and the form closed"
        assert page.locator(".user-panel[data-name='%s']" % name).count() == 1, "the user is listed once"
        assert len(posts) == 1, f"one POST in all: {len(posts)}"
    finally:
        page.unroute("**/api/v1/users")
        remove_user(name)
    show_view(page, "tiles")


def test_role_save_is_off_until_a_change(page, case):
    """A role's Save is off while its name and set are as stored; a
    capability switched turns it on, switched back off again. Nothing
    is sent."""
    show_view(page, "system")
    panel = page.locator(".role-panel[data-key='operator']")
    assert wait_until(page, "!!document.querySelector(\".role-panel[data-key='operator']\")", 5), "the roles are listed"
    panel.locator("summary").click()
    save = panel.locator("form button[type=submit]")
    assert save.is_disabled(), "Save is off with the role as stored"
    switch = panel.locator(".cap-grid .switch").first
    switch.click()
    assert save.is_enabled(), "a capability switched turns Save on"
    switch.click()
    assert save.is_disabled(), "switched back, nothing differs"
    show_view(page, "tiles")


def test_restart_held_with_update_or_reboot(page, case):
    """In a role's set, and in a new role's form, restart shows on, off
    and with why while update or reboot is on, since each includes it,
    and free again, as stored, with them off; Save sends the set as
    switched, without what is held alone, and the role is stored so."""
    key = f"inc{case.width}{case.lang}"
    undo(f"the role {key}", lambda: api_call("DELETE", "api/v1/roles/" + key))
    api_call("POST", "api/v1/roles", {"key": key, "capabilities": ["read"]})
    try:
        show_view(page, "system")
        sel = f".role-panel[data-key='{key}']"
        assert wait_until(page, f"!!document.querySelector(\"{sel}\")", 5), "the role is listed"
        panel = page.locator(sel)
        panel.locator("summary").click()
        why = page.evaluate("t('cap.included')")

        def held(form, holder):
            """restart's switch while holder's is on, and once it is off again."""
            restart = form.locator(".cap-grid .switch[title='restart']")
            box = restart.locator("input")
            assert not box.is_checked() and box.is_enabled() and restart.locator(".flag").count() == 0, "restart off and free, as stored"
            form.locator(f".cap-grid .switch[title='{holder}']").click()
            assert box.is_checked() and box.is_disabled(), f"restart on and off with {holder} on"
            assert restart.locator(".flag").inner_text() == why, f"with why: {restart.inner_text()!r}"
            form.locator(f".cap-grid .switch[title='{holder}']").click()
            assert not box.is_checked() and box.is_enabled() and restart.locator(".flag").count() == 0, f"restart free again with {holder} off"

        for holder in ("update", "reboot"):
            held(panel, holder)
        puts = []
        page.on("request", lambda r: puts.append(r.post_data_json) if r.method == "PUT" and r.url.endswith("/api/v1/roles/" + key) else None)
        panel.locator(".cap-grid .switch[title='update']").click()
        panel.locator("form button[type=submit]").click()
        assert wait_until(page, "document.getElementById('roles-msg').classList.contains('ok')", 5), "the role saved"
        assert [p.get("capabilities") for p in puts] == [["read", "update"]], f"sent {puts}"
        stored = [r for r in api_call("GET", "api/v1/roles")["roles"] if r["key"] == key]
        assert stored and stored[0]["capabilities"] == ["read", "update"], f"stored {stored}"
        restart = panel.locator(".cap-grid .switch[title='restart'] input")
        assert restart.is_checked() and restart.is_disabled(), "the role as saved: restart held with update"

        page.locator("#role-add").click()
        form = page.locator("#role-new")
        form.wait_for()
        for holder in ("update", "reboot"):
            held(form, holder)
        form.locator(".form-cancel").click()
    finally:
        undo(f"the role {key}", lambda: api_call("DELETE", "api/v1/roles/" + key))
    show_view(page, "tiles")


def name_msg(page, selector):
    """The name check's message under a field: its text, or None while
    hidden."""
    return page.evaluate("(() => { const p = document.querySelector(%r); return p && !p.classList.contains('hidden') ? p.textContent : null; })()"
                         % selector)


def test_new_user_name_checked_while_typed(page, case, watch):
    """A new user's name is checked 2 s after the last keystroke and when
    the field is left, with the page's own words; a fixed name clears the
    message at once; a name in use is reported; Create with a bad name
    sends nothing, and a name the server finds in use is worded alike."""
    msg = "#user-new-name-msg"
    posts = []
    page.on("request", lambda r: posts.append(r.url) if r.method == "POST" and r.url.endswith("/api/v1/users") else None)
    show_view(page, "system")
    page.locator("#user-add").click()
    name = page.locator("#user-new-name")
    name.fill("test manager")
    page.wait_for_timeout(500)
    assert name_msg(page, msg) is None, "nothing is said while the name is being typed"
    assert wait_until(page, "!document.querySelector(%r).classList.contains('hidden')" % msg, 4), "the name is checked after 2 s"
    assert name_msg(page, msg) == NAME_BAD[case.lang], f"the page's words, without store: {name_msg(page, msg)}"
    assert name.get_attribute("aria-invalid") == "true" and name.get_attribute("aria-describedby") == "user-new-name-msg", \
        "the field is marked and described by the message"
    name.fill("test-manager")
    assert name_msg(page, msg) is None and name.get_attribute("aria-invalid") is None, "a fixed name clears the message at once"
    name.fill("x y")
    name.press("Tab")
    assert name_msg(page, msg) == NAME_BAD[case.lang], "leaving the field checks at once"
    name.fill("Operator")
    name.press("Tab")
    assert name_msg(page, msg) == NAME_TAKEN[case.lang].format(name="operator"), "a listed name is in use, in lower case"
    name.fill("bad name")
    page.locator("#user-new input[name=password]").fill("own-secret-1")
    page.locator("#user-new button[type=submit]").click()
    page.wait_for_timeout(300)
    assert name_msg(page, msg) == NAME_BAD[case.lang] and not posts, f"Create with a bad name sends nothing: {posts}"
    late = f"late-{case.width}-{case.lang}"
    remove_user(late)
    watch.expected.append(("/api/v1/users", 409))
    api_call("POST", "api/v1/users", {"name": late, "role": "viewer"})  # after the list was read
    try:
        name.fill(late)
        page.locator("#user-new button[type=submit]").click()
        assert wait_until(page, "!document.querySelector(%r).classList.contains('hidden')" % msg, 5), "the server's 409 is said"
        assert name_msg(page, msg) == NAME_TAKEN[case.lang].format(name=late), "worded as the page's check words it"
        assert "store:" not in page.locator("#users-msg").inner_text(), "no store: in the list's message"
    finally:
        remove_user(late)
    page.locator("#user-new .form-cancel").click()
    show_view(page, "tiles")


def test_first_owner_name_checked(page, case):
    """The first owner's form, in the sign-in card (AP FirstOwner), checks
    its name the same way; with a bad name, Create the owner sends
    nothing; in a browser it has no Cancel, which is the app's
    (test_connect.py). /api/v1/me is stubbed as an open server without
    users, which opens the card, and put back; Skip for now closes it."""
    posts = []
    page.on("request", lambda r: posts.append(r.url) if r.method == "POST" and r.url.endswith("/api/v1/users") else None)
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body='{"mode": "open", "setup": true}'))
    try:
        page.evaluate("loadMe()")
        assert wait_until(page, "document.getElementById('login-dialog').open && "
                                "!document.getElementById('setup').classList.contains('hidden')", 5), "the first owner's form shows in the card"
        assert page.locator("#setup-cancel").is_hidden(), "no Cancel in a browser"
        name = page.locator("#setup-user")
        name.fill("test manager")
        name.press("Tab")
        assert name_msg(page, "#setup-name-msg") == NAME_BAD[case.lang], "leaving the field checks it"
        page.locator("#setup-password").fill("own-secret-1")
        page.locator("#setup-again").fill("own-secret-1")
        page.locator("#setup-form button[type=submit]").click()
        page.wait_for_timeout(300)
        assert not posts, f"Create the owner with a bad name sends nothing: {posts}"
        name.fill("owner")
        assert name_msg(page, "#setup-name-msg") is None, "a good name clears the message"
        page.locator("#setup-skip").click()
    finally:
        page.unroute("**/api/v1/me")
        page.evaluate("loadMe()")
    assert wait_until(page, "!document.getElementById('login-dialog').open", 5), "the card goes again"


def test_manager_sets_passwords_but_not_the_owners(new_context, case, watch, guard):
    """A manager (guests and passwords) sees every user under System
    settings → Users: the owner's panel says that only an owner changes an
    owner and offers nothing, an operator's offers its password alone,
    and the role choice offers no owner."""
    from test_login import sign_in
    name = f"mgr-{case.width}-{case.lang}"
    api_call("POST", "api/v1/users", {"name": name, "role": "manager", "password": "mgr-pass-1"})
    try:
        ctx = new_context()
        page = ctx.new_page()
        guard.page = page
        page.set_default_timeout(5000)
        page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
        page.goto(URL)
        sign_in(page, name, "mgr-pass-1")
        show_view(page, "system")
        owner = page.locator(".user-panel[data-name='owner']")
        owner.wait_for()
        owner.locator("summary").click()
        want = page.evaluate("t('users.ownerOnly')")
        assert owner.locator("form").is_hidden() and want in owner.inner_text(), "only an owner changes an owner"
        op = page.locator(".user-panel[data-name='operator']")
        op.locator("summary").click()
        form = op.locator("form")
        assert form.locator("input[name=password]").is_visible(), "the operator's password offered"
        assert form.locator("input[name=display_name]").is_hidden() and form.locator("select[name=role]").is_hidden(), \
            "nothing else of it"
        assert op.locator(".user-tokens-box").is_hidden(), "nor its keys"
        page.locator("#user-add").click()
        options = page.locator("#user-new select[name=role] option").evaluate_all("(os) => os.map((o) => o.value)")
        assert options and "owner" not in options, f"no owner role to give ({options})"
        sign_out(page)
    finally:
        api_call("DELETE", "api/v1/users/" + name)


def test_an_integrations_holder_manages_its_users(new_context, case, watch, guard):
    """A user of a role of read and integrations alone, signed in:
    System settings shows Users, which lists an integration's users and
    the user itself, no regular user; an integration's user's panel
    disables it and Save is answered 200, with no line under its role,
    integration, which lies within the maximum; the integration's own
    user and its system user offer no Remove; there is no Add a user, and
    the Integrations card offers no Link; every button is named and
    nothing overflows (AP AccessEnds, D46-A). The manager, which does not
    outrank integration either, sees no line under that role. Fails on
    the tree before, where Users was drawn for a holder of users, guests
    or passwords alone."""
    from test_integrations import HEADER, _end, raw_call
    from test_login import sign_in
    tag = f"{case.width}{case.lang}"
    key, name, manager = f"igs{tag}", f"igs-{tag}", f"igm-{tag}"
    instance = ("b9%04d%s" % (case.width, case.lang)).encode().hex().ljust(32, "0")[:32]
    subject = ("b9u%04d%s" % (case.width, case.lang)).encode().hex().ljust(32, "c")[:32]
    status, req = raw_call("POST", "api/v1/integrations/requests",
                           {"kind": "homeassistant", "name": f"Home {tag}", "instance_id": instance, "url": "http://192.0.2.21:8123", "version": "2026.11.1"})
    assert status == 201, "the request is taken"
    uuid = req["id"]
    theirs = [uuid + ":", uuid + ":system", uuid + ":" + subject]
    api_call("POST", "api/v1/roles", {"key": key, "capabilities": ["read", "integrations"]})
    api_call("POST", "api/v1/users", {"name": name, "role": key, "password": "holder-pass-1"})
    api_call("POST", "api/v1/users", {"name": manager, "role": "manager", "password": "manager-pass-1"})
    try:
        api_call("POST", f"api/v1/integrations/{uuid}/approve", {"max_caps": ["read", "preferences", "setpoint"]})
        status, got = raw_call("GET", f"api/v1/integrations/requests/{uuid}?code={req['code']}")
        assert status == 200 and got["key"], "the key collected"
        on_behalf = f"id={subject}; level=view; name=Anna; instance={instance}"
        status, me = raw_call("GET", "api/v1/me", headers={"Authorization": "Bearer " + got["key"], HEADER: on_behalf})
        assert status == 200 and me["acts_for"]["id"] == subject, "the integration's user made by its first call"
        ctx = new_context()
        page = ctx.new_page()
        guard.page = page
        page.set_default_timeout(5000)
        page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
        page.goto(URL)
        sign_in(page, name, "holder-pass-1")
        show_view(page, "system")
        assert page.locator(".users-settings").is_visible(), "Users shown to a holder of integrations"
        for user in theirs + [name]:
            page.locator(f".user-panel[data-name='{user}']").wait_for()
        listed = page.locator(".user-panel").evaluate_all("(ps) => ps.map((p) => p.dataset.name)")
        assert all(u == name or ":" in u for u in listed), f"an integration's users and itself alone: {listed}"
        assert page.locator("#user-add").count() == 0, "no Add a user"
        for user in theirs[:2]:
            own = f".user-panel[data-name='{user}']"
            open_fold(page, own)
            assert page.locator(f"{own} .remove-user").is_hidden(), f"{user}: no Remove for the integration's own users"
            close_fold(page, own)
        sel = f".user-panel[data-name='{theirs[2]}']"
        panel = page.locator(sel)
        open_fold(page, sel)
        assert panel.locator("select[name='role']").is_visible() and panel.locator(".remove-user").is_visible(), "its role and Remove offered"
        assert page.locator(f"[id='user-{theirs[2]}-role-hint']").count() == 0, "no line under integration, which lies within the maximum"
        disabled = panel.locator("input[name='disabled']")
        panel.locator("label.switch:has(input[name='disabled'])").click()
        assert disabled.is_checked(), "Disabled switched on"
        path = re.compile(r"/api/v1/users/" + uuid + r"(%3A|:)" + subject + r"$")
        with page.expect_response(lambda r: r.request.method == "PUT" and path.search(r.url)) as answer:
            panel.locator("form button[type=submit]").click()
        assert answer.value.status == 200, f"Save answered {answer.value.status}"
        assert wait_until(page, f"!!document.querySelector(\"{sel} > summary .flag.bad\")", 5), "the user marked disabled"
        named_buttons(page, "System settings → Users, a holder of integrations")
        assert scroll_width(page) <= case.width, "no horizontal scroll"
        ig = f".integration-panel[data-uuid='{uuid}']"
        page.locator(ig).wait_for()
        open_fold(page, ig)
        page.locator(f"{ig} .integration-user[data-id='{subject}']").wait_for()
        assert page.locator(f"{ig} .link-integration-user").count() == 0, "no Link in Integrations for a holder of integrations alone"
        sign_out(page)
        sign_in(page, manager, "manager-pass-1")
        show_view(page, "system")
        own = f".user-panel[data-name='{theirs[0]}']"
        page.locator(own).wait_for()
        open_fold(page, own)
        assert page.locator(f"{own} select[name='role']").is_visible(), "the manager offered its role"
        assert page.locator(f"[id='user-{theirs[0]}-role-hint']").count() == 0, "no line under integration for the manager"
        assert page.locator(f"{own} .remove-user").is_hidden(), "no Remove for the integration's own user"
        sign_out(page)
    finally:
        cleanup(lambda: undo(f"the user {name}", lambda: api_call("DELETE", "api/v1/users/" + name)),
                lambda: undo(f"the user {manager}", lambda: api_call("DELETE", "api/v1/users/" + manager)),
                lambda: undo(f"the role {key}", lambda: api_call("DELETE", "api/v1/roles/" + key)),
                lambda: _end(uuid),
                lambda: undo(f"the user {theirs[2]}", lambda: api_call("DELETE", "api/v1/users/" + theirs[2])))


def test_remove_the_password_follows_may_password(page, case, reads):
    """An integration's user holding a password from before (the list
    stubbed) whom the signed-in user may manage and whose password it may
    not set (may.password false, as for a holder of integrations): its
    panel offers Remove, and not Remove the password, which the server
    would refuse. Fails where Remove the password followed may.manage."""
    stubbed = [dict(INTEGRATION_USERS[1], has_password=True, may={"manage": True, "password": False, "roles": [], "beyond": False})]
    reads.change("**/api/v1/users", lambda body: body + stubbed)
    held, routed = none_of_theirs(page)
    try:
        show_view(page, "system")
        sel = f".user-panel[data-name='{PHONE}']"
        page.locator(sel).wait_for()
        open_fold(page, sel)
        assert page.locator(f"{sel} .remove-user").is_visible(), "Remove offered"
        assert page.locator(f"{sel} .remove-password").count() == 0, "no Remove the password"
        close_fold(page, sel)
    finally:
        page.unroute(held, routed)
        reads.clear("**/api/v1/users")
    show_view(page, "tiles")


def test_add_a_user_for_users_or_guests_alone(page, case):
    """Add a user is drawn for who may add a user, a holder of users or
    guests, and goes for a holder of passwords or integrations alone, whom
    the server refuses a new user (who is in read again as each, the
    page's own capabilities). Fails where it was drawn for every reader
    of Users."""
    show_view(page, "system")
    add = page.locator("#user-add")
    add.wait_for()
    try:
        for caps, drawn in ((["read", "passwords"], False), (["read", "integrations"], False), (["read", "guests"], True), (["read", "users"], True)):
            page.evaluate("(caps) => { state.me = { ...state.me, mode: 'users', capabilities: caps }; }", caps)
            assert wait_until(page, f"document.querySelectorAll('#user-add').length === {1 if drawn else 0}", 5), f"Add a user {'drawn' if drawn else 'gone'} with {caps}"
    finally:
        page.evaluate("loadMe()")
    show_view(page, "tiles")


def test_profile_offers_a_first_password(page, case):
    """My profile of a user with keys only (the owner made from the API
    token) offers Set a password, says what it adds, and asks for no
    current password, which the user has not."""
    assert page.evaluate("state.me.user && !state.me.user.has_password"), "the test's owner has keys only"
    show_view(page, "profile")
    box = page.locator("#me-password")
    assert box.is_visible(), "the own password offered"
    assert page.locator("#me-password-title").inner_text() == page.evaluate("t('me.setPassword')"), "as Set a password"
    box.locator("summary").click()
    assert page.locator("#me-password-hint").is_visible(), "what a password adds"
    assert page.locator("#me-current").is_hidden() and page.locator("#me-new").is_visible(), "no current password asked"
    box.locator("summary").click()
    show_view(page, "tiles")
