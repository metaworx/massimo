# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings' facts to copy and its order (AP UiPolish, Block 5):
each Cube proxy with the Cube's serial and the serial it answers
discovery with, both copyable; the sign-in providers' redirect URI
copyable; Sign-in providers and Integrations explained in their heads;
Maintenance in the owner's order, its titles alike."""
import json

from helpers import show_view, wait_until

AS = {"en": "as", "de": "als"}
LISTENING = {"en": "listening", "de": "horcht"}
SYSTEM = {"version": "massimo v0.0.1-test (go1.26.0, linux/arm64)", "started_at": "2026-11-13T08:00:00+01:00", "install": "apt",
          "os": "linux", "arch": "arm64", "hostname": "massimo-box", "client_ip": "192.0.2.20",
          "cube_proxies": [{"serial": "KEQ0000100", "discovery_serial": "KEQ0000999", "addr": "192.0.2.5:62910", "listening": True},
                           {"serial": "KEQ0000101", "addr": "192.0.2.6:62911", "listening": True}]}


def grant(page):
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
    page.evaluate("navigator.clipboard.writeText('')")


def copied(page, text):
    return wait_until(page, "navigator.clipboard.readText().then((t) => t === %r)" % text, 3)


def test_proxies_both_serials(page, case):
    """A proxy that answers discovery: the Cube's serial, "as" the serial
    it answers with, and its address, each with its copy icon; one that
    answers none: the Cube's serial and its address."""
    grant(page)
    page.route("**/api/v1/system", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(SYSTEM)))
    try:
        show_view(page, "info")
        assert wait_until(page, "document.querySelectorAll('#info-proxies .sys-proxy').length === 2", 5), "two listeners"
        lines = page.locator("#info-proxies .sys-proxy")
        first = lines.nth(0)
        assert first.inner_text().split() == ["KEQ0000100", AS[case.lang], "KEQ0000999", "192.0.2.5:62910", LISTENING[case.lang]], \
            f"the Cube's serial, the discovery serial, the address ({first.inner_text()!r})"
        assert first.locator(".copy-icon").count() == 3, "each with its copy icon"
        first.locator(".copy-icon").nth(1).click()
        assert copied(page, "KEQ0000999"), "the discovery serial copies"
        page.evaluate("navigator.clipboard.writeText('')")
        first.locator(".copy-icon").nth(0).click()
        assert copied(page, "KEQ0000100"), "the Cube's serial copies"
        assert lines.nth(1).inner_text().split() == ["KEQ0000101", "192.0.2.6:62911", LISTENING[case.lang]], "none where it answers no discovery"
    finally:
        page.unroute("**/api/v1/system")
        show_view(page, "tiles")


def test_callback_copies(page, case):
    """The sign-in providers' redirect URI copies by its icon."""
    grant(page)
    show_view(page, "system")
    try:
        url = page.evaluate("pageBase() + 'api/v1/login/oidc/callback'")
        cell = page.locator("#providers-callback")
        assert cell.inner_text().strip() == url, "the redirect URI"
        cell.locator(".copy-icon").click()
        assert copied(page, url), "copies by its icon"
    finally:
        show_view(page, "tiles")


def test_heads_explained(page, case):
    """Sign-in providers and Integrations explain themselves in their
    heads, as the Cubes do."""
    show_view(page, "system")
    try:
        for card, key in (("#sys-providers", "providers"), ("#sys-integrations", "integrations")):
            btn = page.locator(card + " .section-head .help-btn")
            assert btn.count() == 1, f"{card}: an ⓘ in its head"
            btn.click()
            text = page.locator("#" + btn.get_attribute("aria-controls"))
            assert text.is_visible() and text.inner_text() == page.evaluate(f"t('help.{key}')"), f"{card}: its explanation"
            btn.click()
    finally:
        show_view(page, "tiles")


def test_maintenance_order(page, case):
    """Maintenance: Announce on the LAN, the three Cube intervals (AP
    CubePoll v1.0), Settings file, Restart, Reboot the computer, their
    titles alike."""
    show_view(page, "maint")
    try:
        rows = page.evaluate("""() => [...document.querySelectorAll('#sys-maint-body > div')].map((d) => {
          const title = d.querySelector('.action-title');
          return [title ? title.dataset.i18n : '', title ? getComputedStyle(title).fontWeight : ''];
        })""")
        assert [r[0] for r in rows] == ["maint.announce", "maint.poll", "maint.refresh", "maint.discover", "maint.file", "maint.restart", "maint.reboot"], \
            f"the owner's order ({rows})"
        assert len({r[1] for r in rows}) == 1, f"the titles alike ({rows})"
    finally:
        show_view(page, "tiles")
