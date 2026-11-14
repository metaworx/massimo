# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Copying (components.js copyable, copyIcon, copyEl): a text that copies
on a click, its copy icon, Enter, and the fallback a page over plain HTTP
needs, where the browser gives it no clipboard."""
from helpers import wait_until

DONE = {"en": "Copied", "de": "Kopiert"}

# A copyable serial with its icon, put at the top of the page.
MAKE = """() => {
  const el = copyEl("KEQ0000100", { label: "the serial", icon: true });
  el.id = "test-copy";
  document.querySelector("main").prepend(el);
}"""


def clipboard(page):
    return page.evaluate("navigator.clipboard.readText()")


def test_copy(page, case):
    """A click on the text copies it, and so does its icon and Enter on
    it; "Copied" shows on it and is said; without the browser's clipboard
    the fallback copies all the same."""
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
    page.evaluate(MAKE)
    try:
        text = page.locator("#test-copy .mono")
        assert text.get_attribute("role") == "button" and text.get_attribute("tabindex") == "0", "a button to the keyboard"
        assert "the serial" in (text.get_attribute("aria-label") or ""), "named by what it copies"
        page.evaluate("navigator.clipboard.writeText('')")
        text.click()
        assert wait_until(page, "navigator.clipboard.readText().then((t) => t === 'KEQ0000100')", 3), "a click copies"
        assert "copied" in text.get_attribute("class"), "Copied shows on it"
        assert wait_until(page, "document.getElementById('copy-status').textContent === %r" % DONE[case.lang], 3), "and is said"
        page.evaluate("navigator.clipboard.writeText('')")
        page.locator("#test-copy .copy-icon").click()
        assert wait_until(page, "navigator.clipboard.readText().then((t) => t === 'KEQ0000100')", 3), "the icon copies"
        page.evaluate("navigator.clipboard.writeText('')")
        text.focus()
        page.keyboard.press("Enter")
        assert wait_until(page, "navigator.clipboard.readText().then((t) => t === 'KEQ0000100')", 3), "Enter copies"
        # Plain HTTP: no navigator.clipboard for the page; the fallback copies.
        page.evaluate("navigator.clipboard.writeText('')")
        page.evaluate("""() => {
          window.__clip = navigator.clipboard;
          Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true });
        }""")
        text.click()
        page.evaluate("""() => Object.defineProperty(navigator, "clipboard", { value: window.__clip, configurable: true })""")
        assert clipboard(page) == "KEQ0000100", "the fallback copies"
    finally:
        page.evaluate("document.getElementById('test-copy').remove()")
