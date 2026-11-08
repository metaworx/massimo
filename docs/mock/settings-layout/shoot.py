# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The mock's screenshots, from the file itself, into screenshots/.

usage: python shoot.py [<name-filter>]

Needs Playwright with Chromium, as tools/ui-test does (its venv serves:
tools/ui-test/.venv/bin/python shoot.py). Each shot is
<view>-<width>-<theme>.png; a page is shot whole, a menu by the part
of the header it drops into, a section by itself.
"""
import pathlib
import sys

from playwright.sync_api import sync_playwright

HERE = pathlib.Path(__file__).resolve().parent
OUT = HERE / "screenshots"
URL = (HERE / "index.html").as_uri()

# (name, query, width, theme, what): what is "page" for the whole page,
# "view" for the viewport, "top:<px>" for the header's top band, or a
# CSS selector for one element.
SHOTS = [
    ("system", "page=system", 1100, "light", "page"),
    ("system", "page=system", 1100, "dark", "page"),
    ("system", "page=system", 390, "light", "page"),
    ("system", "page=system", 390, "dark", "page"),
    ("system-folded", "page=system&fold=all", 1100, "light", "view"),
    ("system-folded", "page=system&fold=all", 390, "dark", "view"),
    ("system-scrolled", "page=system&scroll=sec-users", 1100, "light", "view"),
    ("system-scrolled", "page=system&scroll=sec-users", 390, "dark", "view"),
    ("system-back-to-top", "page=system&scroll=sec-users", 390, "light", "view"),
    ("sections-menu", "page=system&scroll=sec-users&menu=sections", 1100, "dark", "top:520"),
    ("sections-menu", "page=system&scroll=sec-users&menu=sections", 390, "light", "view"),
    ("cube-disconnected", "page=system&scroll=sec-users&cube=2", 1100, "dark", "top:260"),
    ("cube-disconnected", "page=system&scroll=sec-users&cube=2", 390, "light", "top:360"),
    ("maintenance-open", "page=system&open=store-info,config-info", 1100, "dark", "#sec-maint"),
    ("maintenance-open", "page=system&open=store-info,config-info", 390, "light", "#sec-maint"),
    ("users-as-manager", "page=system&as=manager", 1100, "light", "#sec-users"),
    ("users-as-manager", "page=system&as=manager", 390, "dark", "#sec-users"),
    ("prefs", "page=prefs", 1100, "light", "page"),
    ("prefs", "page=prefs", 1100, "dark", "page"),
    ("prefs", "page=prefs", 390, "light", "page"),
    ("prefs", "page=prefs", 390, "dark", "page"),
    ("profile-keys-only", "page=profile", 1100, "light", "page"),
    ("profile-keys-only", "page=profile", 390, "dark", "page"),
    ("user-menu", "page=system&menu=user", 1100, "light", "top:300"),
    ("user-menu", "page=system&menu=user", 390, "dark", "top:330"),
    ("user-tooltip", "page=system&tip=user", 1100, "dark", "top:140"),
    ("cube-choice", "page=system&menu=cube", 1100, "light", "top:340"),
    ("cube-choice", "page=system&menu=cube", 390, "dark", "top:470"),
    ("cube-names", "page=names", 1100, "light", "page"),
    ("cube-names", "page=names", 390, "dark", "page"),
]


def main():
    only = sys.argv[1] if len(sys.argv) > 1 else ""
    OUT.mkdir(exist_ok=True)
    with sync_playwright() as p:
        browser = p.chromium.launch()
        for name, query, width, theme, what in SHOTS:
            file = OUT / f"{name}-{width}-{theme}.png"
            if only and only not in file.name:
                continue
            page = browser.new_page(
                viewport={"width": width, "height": 800 if width > 600 else 844},
                color_scheme=theme,
                reduced_motion="reduce",
                device_scale_factor=1,
            )
            page.goto(f"{URL}?shot=1&theme={theme}&{query}")
            page.wait_for_load_state("load")
            # mock.js scrolls (scroll=) over two frames; the header then compacts
            page.wait_for_timeout(400)
            if what == "page":
                page.screenshot(path=file, full_page=True, animations="disabled", caret="hide")
            elif what == "view":
                page.screenshot(path=file, animations="disabled", caret="hide")
            elif what.startswith("top:"):
                h = int(what[4:])
                page.screenshot(path=file, clip={"x": 0, "y": 0, "width": width, "height": h}, animations="disabled", caret="hide")
            else:
                # a section by itself: without back to top floating over it,
                # and without the header and the page's row stuck over its top
                page.add_style_tag(content=".to-top { display: none !important; } .top, .page-bar { position: static !important; }")
                page.locator(what).screenshot(path=file, animations="disabled", caret="hide")
            page.close()
            print(file.relative_to(HERE))
        browser.close()


if __name__ == "__main__":
    main()
