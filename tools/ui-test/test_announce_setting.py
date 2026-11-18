# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings → Maintenance, Announce on the LAN (AP AnnounceSetting
v1.0, Block 2), as the owner: the test server listens on the loopback
without -announce, so the choice is Automatic and the server does not
announce itself. On is stored and waits for the next restart; where
-announce or MASSIMO_ANNOUNCE decides, the choice is read-only and says
so. Each case puts the setting back."""

from conftest import api_call
from helpers import named_buttons, scroll_width, show_view, wait_until


def announce_setting():
    return (api_call("GET", "api/v1/settings") or {}).get("announce") or {}


def test_announce_choice(page, case):
    """Automatic at first, the hint saying the server does not announce
    itself; On is stored and the hint says it waits for the restart."""
    api_call("PUT", "api/v1/settings", {"announce": None})
    try:
        show_view(page, "maint")
        sel = page.locator("#announce-mode")
        assert wait_until(page, "document.querySelector('#announce-text').textContent.length > 0", 5), "the hint is filled"
        assert sel.is_enabled() and sel.input_value() == "auto", "Automatic, and the choice open"
        before = page.locator("#announce-text").inner_text()
        sel.select_option("on")
        assert wait_until(page, "!document.querySelector('#announce-msg').classList.contains('hidden')", 5), "the change says how it went"
        assert "ok" in (page.locator("#announce-msg").get_attribute("class") or ""), "saved"
        assert announce_setting().get("value") == "on", "on stored"
        assert announce_setting().get("in_effect") == "off", "in effect only at the next start"
        after = page.locator("#announce-text").inner_text()
        assert after != before, "the hint says the change waits for the restart"
        named_buttons(page, "System settings → Maintenance")
        assert scroll_width(page) <= case.width, "no horizontal scroll in System settings → Maintenance"
    finally:
        api_call("PUT", "api/v1/settings", {"announce": None})
        show_view(page, "tiles")


def test_announce_fixed(page, case, reads):
    """Where the variable decides, the choice is read-only and names it."""

    def fixed(body):
        if "announce" in body:
            body["announce"]["fixed_by"] = "MASSIMO_ANNOUNCE"
            body["announce"]["in_effect"] = "on"
        return body

    reads.change("**/api/v1/settings", fixed)
    try:
        show_view(page, "maint")
        assert wait_until(page, "document.querySelector('#announce-mode').disabled", 5), "read-only where the variable decides"
        assert "MASSIMO_ANNOUNCE" in page.locator("#announce-text").inner_text(), "the hint names what decides"
    finally:
        show_view(page, "tiles")
        reads.clear("**/api/v1/settings")
