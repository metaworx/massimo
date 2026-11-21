# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The page's chrome as parts (AP UiParts, Block 14): the pages, ☰ and
the settings pages' rows follow what the page holds without taking from
the user what it holds, and without reading more than the plain page
read.

The pages shown are a signal (ui/src/shared/pages.ts): a part that read
the page shown in a drawing or an effect would follow it and could read
again at each change of page. The regions ask isShown, which follows
nothing; the first test counts every read of each showing of every page
against what the page before the store read. The rows (ui/src/chrome/
Bar.tsx) keep their sections menu through who is in read again, and the
row of the page shown alone gives the cards' scroll margin its height."""
from collections import Counter

import pytest

from conftest import URL, api_call
from helpers import frames, hold, held_poll, open_personal_page, scroll_settled, show_view, wait_held, wait_until
from test_login import sign_in
from test_preferences import clear_user, operator_key

# The walk: every page with a row and Rooms, twice.
WALK = ("system", "maint", "info", "me", "cube", "tiles") * 2

# What each showing of WALK reads under /api/v1/, path by path with its
# count, on a page just loaded with the test's fixture, the poll held:
# taken from the page before the pages' store (the tree of AP UiParts'
# Block 13, an export of it, 2026-11-21), the same in 1100-en and 390-de
# and in each of three walks of a page but for the Cube's devices, read
# at the first showing of Cube settings alone.
READS = (
    ("system", {"cubes/KEQ0000100/backups": 1, "integrations": 1, "integrations/found": 1, "login/providers": 1, "providers": 1, "roles": 1, "settings": 2, "users": 2}),
    ("maint", {"reboot": 1, "restart": 1, "settings": 2, "update": 1}),
    ("info", {"config": 1, "store": 1, "system": 1}),
    ("me", {"login/providers": 1, "me/preferences": 1, "me/push": 1, "users/owner": 1, "users/owner/identities": 1, "users/owner/tokens": 1}),
    ("cube", {"cubes/KEQ0000100/backups": 1, "cubes/KEQ0000100/devices": 1, "cubes/KEQ0000100/pairing": 1, "cubes/KEQ0000100/rooms": 1}),
    ("tiles", {}),
    ("system", {"cubes/KEQ0000100/backups": 1, "integrations": 1, "integrations/found": 1, "login/providers": 1, "providers": 1, "roles": 1, "settings": 2, "users": 2}),
    ("maint", {"reboot": 1, "restart": 1, "settings": 2, "update": 1}),
    ("info", {"config": 1, "store": 1, "system": 1}),
    ("me", {"login/providers": 1, "me/preferences": 1, "me/push": 1, "users/owner": 1, "users/owner/identities": 1, "users/owner/tokens": 1}),
    ("cube", {"cubes/KEQ0000100/backups": 1, "cubes/KEQ0000100/pairing": 1, "cubes/KEQ0000100/rooms": 1}),
    ("tiles", {}),
)

# The page's own requests counted while they run: fetch wrapped once.
INFLIGHT = """() => { if (window.__shellInflight !== undefined) return; window.__shellInflight = 0; const f = window.fetch;
  window.fetch = function (...a) { window.__shellInflight++; return f.apply(this, a).finally(() => { window.__shellInflight--; }); }; }"""
# Settled: no request of the page's own running for 200 ms, within 5 s.
SETTLED = """async () => { for (let quiet = 0, n = 0; n < 100; n++) { await new Promise((d) => setTimeout(d, 50));
  quiet = window.__shellInflight ? 0 : quiet + 1; if (quiet >= 4) return true; } return false; }"""


def test_pages_read_at_each_showing_what_they_read(page, case):
    """Every page with a row and Rooms shown in turn, twice, the poll
    held: each showing reads under /api/v1/ what the page before the
    pages' store read, path for path, and nothing more at a change of
    page. A forward guard: the page before the store passes it too, from
    which READS was taken."""
    page.evaluate(INFLIGHT)
    show_view(page, "tiles")
    assert page.evaluate(SETTLED), "Rooms comes to rest"
    got, cur = [], []

    def note(r):
        if r.method == "GET" and "/api/v1/" in r.url:
            cur.append(r.url.split("/api/v1/", 1)[1].split("?")[0])

    page.on("request", note)
    try:
        with held_poll(page):
            for view in WALK:
                cur.clear()
                show_view(page, view)
                assert page.evaluate(SETTLED), f"{view} comes to rest"
                got.append((view, dict(Counter(cur))))
    finally:
        page.remove_listener("request", note)
    for (view, want), (_, reads) in zip(READS, got):
        assert reads == want, f"{view}: read {reads}, the page before the store read {want}"


def test_sections_menu_kept_through_who_is_in_and_the_language(page, case):
    """The personal page's sections menu open, the focus on My settings'
    item: who is in read again, as at every sign-in and change of the
    user, and the language applied again keep the menu open, the item the
    element it was, with the focus. The plain row emptied the menu and
    filled it anew at both (pageBar), and the item had gone."""
    show_view(page, "me")
    menu = page.locator("#me .sec-menu")
    menu.locator("summary").click()
    item = menu.locator(".toc-link[data-target='prefs']")
    item.focus()
    page.evaluate("window.__shellItem = document.querySelector(\"#me .sec-menu .toc-link[data-target='prefs']\")")
    for read in ("loadMe()", "applyLanguage()"):
        page.evaluate(f"(async () => {{ await {read}; }})()")
        frames(page)  # the row drawn again after the read
        assert page.evaluate("window.__shellItem.isConnected"), f"the item the element it was after {read}"
        assert page.evaluate("document.activeElement === window.__shellItem"), f"with the focus after {read}"
        assert menu.get_attribute("open") is not None, f"the menu open after {read}"
    page.keyboard.press("Escape")
    show_view(page, "tiles")


# Changes of page, each order of the rows on the page among them: from
# the page whose row the page holds later to the one it holds earlier and
# back, as the rows' observers report.
CHANGES = (("info", "system"), ("system", "me"), ("me", "maint"), ("cube", "info"))
SECTIONS = {"system": "#system", "me": "#me", "maint": "#maint-page", "info": "#info-page", "cube": "#cube-page"}


def test_the_cards_come_to_rest_below_the_row_after_a_change_of_page(page, case):
    """After each change of page, the cards' scroll margin (--bar-h) is the
    height of the row of the page shown, and an item of its sections menu
    brings a card with room below it to rest with its head below the row.
    Every row wrote its
    height there, a hidden one 0, in the order they were made, so that
    after some changes the 0 of the page left came last and the card's
    head went under the row: from Personal settings to Maintenance on
    the page before Block 14, from System info to System settings on its
    first tree."""
    for before, after in CHANGES:
        show_view(page, before)
        show_view(page, after)
        frames(page)  # the row of the page shown measured
        bar = SECTIONS[after] + " .page-bar"
        got = page.evaluate("(sel) => [getComputedStyle(document.documentElement).getPropertyValue('--bar-h').trim(),"
                            " document.querySelector(sel).offsetHeight + 'px']", bar)
        assert got[0] == got[1], f"{before} → {after}: --bar-h {got[0]}, the row {got[1]}"
        menu = page.locator(SECTIONS[after] + " .sec-menu")
        if not menu.count():
            continue  # System info has no sections menu
        # room below every card, so that the one chosen can come to the top
        page.evaluate("(sel) => { const d = document.createElement('div'); d.className = 'test-spacer'; d.style.height = '3000px';"
                      " document.querySelector(sel).appendChild(d); }", SECTIONS[after])
        menu.locator("summary").click()
        item = menu.locator(".toc-link:visible").nth(1)  # the second: the first is at the top already
        target = item.get_attribute("data-target")
        item.click()
        assert scroll_settled(page), f"{before} → {after}: the menu's scroll comes to rest"
        rest = page.evaluate("([sel, id]) => [document.getElementById(id).getBoundingClientRect().top,"
                             " document.querySelector(sel).getBoundingClientRect().bottom]", [bar, target])
        page.evaluate("document.querySelectorAll('.test-spacer').forEach((e) => e.remove()); window.scrollTo({top: 0, behavior: 'instant'})")
        assert rest[0] >= rest[1] - 1, f"{before} → {after}: {target}'s head at {rest[0]}, under the row ending at {rest[1]}"
    show_view(page, "tiles")


@pytest.mark.cases("1100-de", "390-de")
def test_no_read_of_my_profile_while_signing_out(new_context, case, watch, guard):
    """A sign-out forgets the plain state.me first, then waits for the
    signed-out language where it differs from the user's, and only then
    clears who is in for the parts: Personal settings shown in that wait
    reads nothing of My profile, which the parts would have read with no
    name and no token (GET api/v1/users/), and the page ends signed out.
    The operator's language is French on a page loaded in German, which
    therefore has not loaded German's table: its file is held through the
    wait. A forward guard on the page before the pages' store, whose
    plain check read state.me."""
    op, op_id = operator_key()
    try:
        clear_user(op)
        ctx = new_context()  # German, the case's locale
        page = ctx.new_page()
        guard.page = page
        page.set_default_timeout(5000)
        page.on("pageerror", lambda e: watch.problems.append(f"page error: {e}"))
        page.goto(URL)
        sign_in(page, "operator", "op-pass")
        page.evaluate("setSetting('lang', 'fr')")
        page.reload()  # the operator's French alone loaded, German not
        page.wait_for_selector(".room", timeout=10000)
        assert wait_until(page, "i18n.lang === 'fr' && !I18N.de", 5), "the page in French, German not loaded"
        users = []
        page.on("request", lambda r: users.append(r.url) if "/api/v1/users/" in r.url else None)
        held, sent = [], []
        hold(page, "**/ui/i18n/de.json", "GET", held, sent)
        try:
            page.locator("#nav-menu > summary").click()
            page.locator("#menu-logout").click()
            assert wait_held(page, held), "the sign-out waits for German's table"
            open_personal_page(page)  # in the wait
            page.locator("#me").wait_for(state="visible")
            held[0].continue_()
        finally:
            page.unroute("**/ui/i18n/de.json")
        assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the page ends signed out, the sign-in asked for"
        assert not users, f"nothing of My profile read during the sign-out: {users}"
    finally:
        clear_user(op)
        api_call("DELETE", "api/v1/users/operator/tokens/%d" % op_id)
