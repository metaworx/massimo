# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Smoke test of the Massimo web UI against a running server.

usage: smoke.py <url>   (tools/ui-test/run.sh starts the server)

At 1100 and 390 px wide, in English and German: the page loads with
room tiles, a tile's device list and its configuration open, the log
and the settings page open, and the language follows the browser. Any
console error, page error or horizontal scroll fails the test.
"""
import sys

from playwright.sync_api import sync_playwright

url = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8119/"
failures = []


def check(cond, what):
    print(("ok    " if cond else "FAIL  ") + what)
    if not cond:
        failures.append(what)


def run(page, label, lang):
    page.goto(url)
    page.wait_for_selector(".room", timeout=10000)
    check(page.locator(".room").count() > 0, f"{label}: room tiles shown")
    check(page.evaluate("document.documentElement.lang") == lang, f"{label}: page language {lang}")
    tile = page.locator(".room").first
    tile.locator(".room-name-btn").click()
    page.wait_for_selector(".room .device", timeout=5000)
    check(tile.locator(".device").count() > 0, f"{label}: device list opens")
    tile.locator(".tile-cog").click()
    page.wait_for_selector(".room .config-body", timeout=5000)
    tabs = tile.locator(".config-tabs [role=tab]")
    check(tabs.count() >= 4, f"{label}: configuration opens with its tabs")
    for name in tabs.all_text_contents():
        tabs.filter(has_text=name).first.click()
        check(tile.locator(".config-body").is_visible(), f"{label}: tab {name.strip()}")
    tile.locator(".config-tabs .close").click()
    check(tile.locator(".config-body").count() == 0, f"{label}: configuration closes")
    width = page.viewport_size["width"]
    check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll on the tiles")
    page.locator("#view-log").click()
    check(page.locator("#logview").is_visible(), f"{label}: log opens")
    page.locator("#view-settings").click()
    check(page.locator("#settings").is_visible(), f"{label}: settings open")
    page.wait_for_timeout(500)
    check(page.evaluate("document.documentElement.scrollWidth") <= width, f"{label}: no horizontal scroll on the settings")
    page.locator("#view-tiles").click()
    check(page.locator(".room").first.is_visible(), f"{label}: back to the tiles")


with sync_playwright() as pw:
    browser = pw.chromium.launch()
    for locale, lang in (("en-US", "en"), ("de-DE", "de")):
        for width in (1100, 390):
            label = f"{width} px {lang}"
            ctx = browser.new_context(viewport={"width": width, "height": 900}, locale=locale)
            page = ctx.new_page()
            page.set_default_timeout(5000)
            page.on("console", lambda m, label=label: check(m.type != "error", f"{label}: console: {m.text}") if m.type == "error" else None)
            page.on("pageerror", lambda e, label=label: check(False, f"{label}: page error: {e}"))
            try:
                run(page, label, lang)
            except Exception as e:  # a timeout or a missing element
                check(False, f"{label}: {e}")
            ctx.close()
    browser.close()

if failures:
    print(f"ui-test: {len(failures)} failure(s)")
    sys.exit(1)
print("ui-test: passed")
