# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""My settings → Appearance: a scheme changes the page's colours, the mode
sets data-theme, the editor applies live and keeps a saved scheme in the
browser, its hex fields take what is typed, and Export downloads the
scheme document and Import reads it back, both under the page's
Content-Security-Policy. What is stored is the browser context's, so
nothing is left for the next test."""
import json
from pathlib import Path

from helpers import CONTRAST_SCHEME, HEX_BAD, HEX_LABEL, NOT_A_SCHEME, SAVED, named_buttons, scroll_width, show_view, wait_until


def css_var(page, name):
    return page.evaluate(f"getComputedStyle(document.documentElement).getPropertyValue('{name}').trim()")


def test_schemes_editor_export_import(page, case, tmp_path):
    width, lang = case.width, case.lang
    show_view(page, "prefs")
    bg = lambda: css_var(page, "--bg")
    stored = lambda: page.evaluate("JSON.parse(localStorage.getItem('massimo.schemes'))")
    default_bg = bg()
    sel = page.locator("#scheme-select")
    assert sel.locator("option").count() >= 3, "built-in schemes in the choice"
    sel.select_option("contrast")
    assert bg() == "#ffffff" and bg() != default_bg, "a scheme changes the computed --bg"
    assert sel.locator("option:checked").inner_text() == CONTRAST_SCHEME[lang], f"scheme named in {lang}"
    assert page.evaluate("localStorage.getItem('massimo.scheme')") == "contrast", "scheme stored"
    mode = page.locator("#theme-select")
    mode.select_option("dark")
    assert page.evaluate("document.documentElement.dataset.theme") == "dark", "mode dark sets data-theme"
    assert bg() == "#000000", "the dark set in dark mode"
    assert page.evaluate("localStorage.getItem('massimo.theme')") == "dark", "mode stored"
    mode.select_option("system")
    assert not page.evaluate("document.documentElement.hasAttribute('data-theme')"), "mode system removes data-theme"
    page.locator("#scheme-customise").click()
    editor = page.locator("#scheme-editor")
    assert editor.is_visible() and editor.locator("input[type=color]").count() == 40, "editor with a colour input per token and set"
    assert editor.locator(".contrast.ok").count() == 16, "contrast badges, all ok for a built-in scheme: seven backgrounds and the icons' amber, twice"
    assert editor.locator("#scheme-delete").is_disabled(), "no Delete for a built-in scheme"
    named_buttons(page, "the scheme editor")
    scheme_hex(page, lang)
    page.locator("#scheme-light-bg").fill("#112233")
    assert bg() == "#112233", "the editor applies live"
    assert editor.locator(".contrast.bad").count() == 1, "a bad contrast is graded"
    assert scroll_width(page) <= width, "no horizontal scroll with the editor"
    page.locator("#scheme-name").fill("Test scheme")
    editor.locator("button.primary").click()
    assert sel.input_value() == "custom:Test scheme", "Save as chooses the saved scheme"
    assert page.locator("#scheme-msg").inner_text() == SAVED[lang], f"saved message in {lang}"
    kept = stored()
    assert len(kept) == 1 and kept[0]["name"] == "Test scheme" and kept[0]["light"]["bg"] == "#112233" \
        and len(kept[0]["light"]) == 20 and len(kept[0]["dark"]) == 20, "the scheme is in the browser"
    assert not editor.locator("#scheme-delete").is_disabled(), "Delete for the user's scheme"
    with page.expect_download() as dl:
        page.locator("#scheme-export").click()
    doc = json.loads(Path(dl.value.path()).read_text())
    assert dl.value.suggested_filename == "massimo-scheme-test-scheme.json", "export file name"
    assert doc.get("format") == "massimo-scheme/1" and doc.get("name") == "Test scheme" and doc["light"]["bg"] == "#112233" \
        and len(doc["light"]) == 20 and len(doc["dark"]) == 20 and doc["dark"]["bg"] == "#000000", "export is the scheme document"
    editor.locator("#scheme-delete").click()
    assert sel.input_value() == "massimo" and bg() == default_bg and editor.is_hidden(), "Delete goes back to the default"
    assert page.evaluate("localStorage.getItem('massimo.schemes')") is None, "nothing stored after Delete"
    doc["name"] = "Round trip"
    del doc["dark"]["bg"]
    doc["light"]["nope"] = "#123456"
    good = tmp_path / "scheme.json"
    good.write_text(json.dumps(doc))
    page.locator("#scheme-file").set_input_files(str(good))
    assert wait_until(page, "state.settings.scheme === 'custom:Round trip'", 5), "Import chooses the imported scheme"
    assert bg() == "#112233", "Import round-trips the colours"
    assert stored()[0]["dark"]["bg"] == "#151514", "a missing token takes the default"
    assert "nope" not in stored()[0]["light"], "an unknown token is ignored"
    bad = tmp_path / "bad.json"
    bad.write_text('{"hello": 1}')
    page.locator("#scheme-file").set_input_files(str(bad))
    msg = page.locator("#scheme-msg")
    assert wait_until(page, "document.getElementById('scheme-msg').classList.contains('bad')", 5) \
        and msg.inner_text() == NOT_A_SCHEME[lang], f"a bad file shows a message in {lang}"
    assert page.evaluate("state.settings.scheme") == "custom:Round trip", "a bad file changes nothing"
    page.locator("#scheme-customise").click()
    assert editor.is_visible() and scroll_width(page) <= width, "no horizontal scroll with the editor open on the imported scheme"
    editor.locator(".actions-right button").click()
    assert editor.is_hidden(), "Close hides the editor"
    sel.select_option("massimo")
    assert bg() == default_bg, "back to the default scheme"
    show_view(page, "tiles")


def scheme_hex(page, lang):
    """The editor's hex fields: a value typed without the # applies, #ABC
    is widened to #aabbcc, a value that is not a colour is refused and
    the colour stays, Escape puts the field back, and the picker writes
    the field. The badge, or its empty place, comes before the colour,
    so the colour inputs and the hex fields of a set line up. Run with
    the editor open on the high-contrast scheme; the next steps of
    test_schemes_editor_export_import grade the colours it leaves."""
    line = lambda: css_var(page, "--line")
    hex_field = page.locator("#scheme-light-line-hex")
    picker = page.locator("#scheme-light-line")
    msg = page.locator("#scheme-hex-msg")
    assert hex_field.get_attribute("aria-label") == HEX_LABEL[lang], f"hex field labelled in {lang}"
    hex_field.fill("ff0000")
    assert line() == "#ff0000" and picker.input_value() == "#ff0000", "ff0000 typed without # applies live"
    hex_field.press("Tab")
    assert hex_field.input_value() == "#ff0000", "the hex field is normalised on change"
    hex_field.fill("#ABC")
    hex_field.press("Tab")
    assert hex_field.input_value() == "#aabbcc" and line() == "#aabbcc", "#ABC becomes #aabbcc"
    hex_field.fill("nope")
    hex_field.press("Tab")
    assert hex_field.get_attribute("aria-invalid") == "true" and line() == "#aabbcc" and picker.input_value() == "#aabbcc", \
        "nope is refused and the colour stays"
    assert msg.is_visible() and msg.inner_text() == HEX_BAD[lang], f"refused hex explained in {lang}"
    hex_field.focus()
    hex_field.press("Escape")
    assert hex_field.input_value() == "#aabbcc" and hex_field.get_attribute("aria-invalid") is None and msg.is_hidden(), \
        "Escape puts the hex field back"
    picker.fill("#123456")
    assert hex_field.input_value() == "#123456" and line() == "#123456", "the picker writes the hex field"
    for i, mode in enumerate(("light", "dark")):
        cols = page.evaluate("""(i) => [...document.querySelectorAll('#scheme-editor .scheme-set')[i].querySelectorAll('.scheme-row')]
            .map((r) => {
              const x = (sel) => Math.round(r.querySelector(sel).getBoundingClientRect().left * 10) / 10;
              return {badge: x('.contrast'), colour: x('input[type=color]'), hex: x('.scheme-hex'),
                      real: !r.querySelector('.contrast').classList.contains('none')};
            })""", i)
        assert len(cols) == 20 and any(c["real"] for c in cols) and any(not c["real"] for c in cols) \
            and len({c["colour"] for c in cols}) == 1 and len({c["hex"] for c in cols}) == 1 \
            and len({c["badge"] for c in cols}) == 1 and all(c["badge"] < c["colour"] < c["hex"] for c in cols), \
            f"badge column first, colours and hex fields aligned in the {mode} set"
    mark = page.locator("#scheme-light-warn-mark").locator("xpath=ancestor::*[contains(@class, 'scheme-row')]").locator(".contrast")
    assert mark.get_attribute("class") == "contrast ok" and "3" < mark.inner_text(), \
        "the icons' amber graded by its contrast on a tile, at 3 for a graphic"
