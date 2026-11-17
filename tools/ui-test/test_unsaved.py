# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Cube settings keep what is typed until it is saved or reset, and say
where it is unsaved (AP UnsavedMarks, Block 1): Save plain and off while
nothing is changed, primary once something is, with the warning mark
before its text once the focus has left the form or a minute has passed,
turning while it saves, with the check for a while after a save and the
cross after a failed one, whether the server was not reached or refused
the value; Reset beside it. While a Cube's settings are unsaved, the
Cube's problem lists, in its panel's head and in the header, show the
disk, amber, red after a failed save, and a click on it opens the form
with the focus in it. What is typed goes with the page from System
settings to Cube settings and back, and is forgotten when the Cube is no
longer served.

The page runs under Playwright's clock (the marker clock), which the
tests move on for the minute and for the check's five seconds. No PUT
reaches the server: each is held, answered or refused by a route, so the
store and the fake Cube keep their settings."""
import json

import pytest

from helpers import hold, show_view, wait_held, wait_until

pytestmark = pytest.mark.clock

PANEL = "#cubes-list .cube-panel[data-serial='KEQ0000100']"
PAGE_PANEL = "#cube-page-body .cube-panel[data-serial='KEQ0000100']"
CUBE_PATH = "**/api/v1/cubes/KEQ0000100"
UNSAVED = {"en": "Cube settings: not saved", "de": "Cube-Einstellungen: nicht gespeichert"}
FAILED = {"en": "Cube settings: saving failed", "de": "Cube-Einstellungen: Speichern fehlgeschlagen"}
REFUSED = "address: not a host:port"


def open_panel(page):
    """System settings → Cubes with the fake Cube's panel open; its form."""
    show_view(page, "system")
    panel = page.locator(PANEL)
    if panel.get_attribute("open") is None:
        panel.locator(":scope > summary .cube-head-who").click()
    form = panel.locator("form.cube-form")
    form.wait_for(state="visible")
    return panel, form


def save_state(form):
    """Save's state: [off, primary, busy, the class of its first child's sign or ""]."""
    return form.locator("button[type=submit]").evaluate("""(b) => {
      const s = b.firstElementChild;
      const sign = !s ? "" : s.matches(".mark-sign.tone-warn") ? "warn" : s.matches(".mark-sign.tone-bad") ? "bad" : s.matches(".ok-check") ? "ok" : "?";
      return [b.disabled, b.classList.contains("primary"), b.classList.contains("busy"), sign];
    }""")


def leave(page):
    """The focus taken out of the form, by a click beside it in the
    panel, on the Cube's facts' corner, which takes no focus."""
    page.locator(PANEL + " .cube-info").click(position={"x": 1, "y": 1})


def item(page, where):
    """The unsaved item, a button, in a problem list; where its place."""
    return page.locator(where + " .problems-list .problem-action")


def wait_save(page, form, want, what):
    """Waits until Save is in the state want (save_state), what naming it."""
    for _ in range(25):
        if save_state(form) == want:
            return
        page.wait_for_timeout(200)
    assert save_state(form) == want, f"{what}: Save is {save_state(form)}, not {want}"


def test_save_shows_the_state(page, case):
    """Clean, changed, unsaved once the focus left, changed again at the
    next key, saving, then saved with the check, which goes after five
    seconds; Reset off while clean."""
    panel, form = open_panel(page)
    name = form.locator("input[name=name]")
    reset = form.locator(".form-reset")
    assert save_state(form) == [True, False, False, ""], "clean: off, plain, no sign"
    assert reset.is_disabled(), "Reset off while clean"
    name.fill("Typed name")
    wait_save(page, form, [False, True, False, ""], "changed: on, primary, no sign")
    assert reset.is_enabled(), "Reset on once changed"
    assert item(page, PANEL + " > summary .cube-pill").count() == 0, "no disk while the user is in the form"
    leave(page)
    wait_save(page, form, [False, True, False, "warn"], "left")
    assert form.locator("button[type=submit] > .mark-sign").count() == 1, "the sign a child of the button, before its text"
    pill = item(page, PANEL + " > summary .cube-pill")
    assert pill.count() == 1 and pill.text_content().strip() == UNSAVED[case.lang], f"the disk in the panel's pill: {pill.all_text_contents()}"
    assert page.locator(PANEL + " > summary .cube-pill .problems-sum use[href='#icon-disk-small']").count() == 1, "the disk in its summary"
    name.press("End")
    name.type("!")
    assert save_state(form) == [False, True, False, ""], "a key in the form: changed again"
    held, sent = [], []
    hold(page, CUBE_PATH, "PUT", held, sent)
    try:
        form.locator("button[type=submit]").click()
        assert wait_held(page, held), "the PUT is sent"
        assert save_state(form)[:3] == [True, True, True], "saving: off and turning"
        assert sent == [{"name": "Typed name!"}], f"the name: {sent}"
        for route in held:
            route.fulfill(status=200, content_type="application/json", body="{}")
        wait_save(page, form, [True, False, False, "ok"], "saved")
        assert name.input_value() == "Fake Cube", "the form shows the Cube as the server has it"
        assert item(page, PANEL + " > summary .cube-pill").count() == 0, "no disk once saved"
        page.clock.fast_forward(5100)
        wait_save(page, form, [True, False, False, ""], "five seconds after the save")
    finally:
        page.unroute(CUBE_PATH)


def test_unsaved_a_minute_after_the_last_change(page, case):
    """The focus kept in the form: amber a minute after the last change,
    not before; Reset takes the stored value back."""
    panel, form = open_panel(page)
    name = form.locator("input[name=name]")
    name.fill("Slow typist")
    page.clock.fast_forward(30_000)
    page.wait_for_timeout(300)
    assert save_state(form)[3] == "", "half a minute: still changed"
    name.type("s")
    page.clock.fast_forward(45_000)
    page.wait_for_timeout(300)
    assert save_state(form)[3] == "", "the minute starts again at each change"
    page.clock.fast_forward(16_000)
    wait_save(page, form, [False, True, False, "warn"], "a minute after the last change")
    assert name.evaluate("(e) => document.activeElement === e"), "the focus stayed in the field"
    form.locator(".form-reset").click()
    assert name.input_value() == "Fake Cube", "Reset takes the stored value"
    wait_save(page, form, [True, False, False, ""], "reset")


@pytest.mark.parametrize("how", ["not reached", "refused"])
def test_failed_save_turns_red(page, case, watch, how):
    """A save the server does not answer, and one it refuses: the cross on
    Save, the message line red with the error, the disk red; no field
    outlined; Reset takes the stored values and clears it."""
    panel, form = open_panel(page)
    address = form.locator("input[name=address]")

    def handle(route):
        if route.request.method != "PUT":
            route.continue_()
        elif how == "not reached":
            route.abort()
        else:
            route.fulfill(status=400, content_type="application/json", body=json.dumps({"error": REFUSED}))

    watch.expected.append(("/cubes/KEQ0000100", "ERR_FAILED" if how == "not reached" else 400))
    page.route(CUBE_PATH, handle)
    try:
        address.fill("not an address")
        form.locator("button[type=submit]").click()
        wait_save(page, form, [False, True, False, "bad"], "failed")
        line = page.locator("#cubes-msg")
        assert line.is_visible() and "KEQ0000100" in line.inner_text(), f"the message line says it: {line.inner_text()}"
        if how == "refused":
            assert REFUSED in line.inner_text(), "with the server's error"
        assert form.locator("[aria-invalid=true]").count() == 0, "no field outlined"
        assert address.input_value() == "not an address", "the draft kept"
        leave(page)
        pill = item(page, PANEL + " > summary .cube-pill")
        assert pill.count() == 1 and pill.text_content().strip() == FAILED[case.lang], f"the disk says it: {pill.all_text_contents()}"
        assert pill.locator(".flag.bad").count() == 1, "red"
        address.type("x")
        assert save_state(form)[3] == "bad", "a key keeps it red"
        form.locator(".form-reset").click()
        wait_save(page, form, [True, False, False, ""], "reset")
        assert address.input_value() == page.evaluate("state.cubes.find((c) => c.serial === 'KEQ0000100').configured_address || ''"), \
            "Reset takes the stored address"
        assert item(page, PANEL + " > summary .cube-pill").count() == 0, "no disk after Reset"
    finally:
        page.unroute(CUBE_PATH)


def test_typing_kept_and_the_disk_opens_it(page, case):
    """What is typed in System settings stays on Cube settings and back;
    from Rooms, the header's problem list shows the disk under the Cube,
    and a click on it opens Cube settings with the focus in the form."""
    panel, form = open_panel(page)
    form.locator("input[name=name]").fill("Kept across pages")
    form.locator("input[name=proxy]").fill("127.0.0.1:62999")
    show_view(page, "cube")
    page_form = page.locator(PAGE_PANEL + " form.cube-form")
    page_form.wait_for(state="visible")
    assert page_form.locator("input[name=name]").input_value() == "Kept across pages", "the name typed, on Cube settings"
    assert page_form.locator("input[name=proxy]").input_value() == "127.0.0.1:62999", "the proxy typed, on Cube settings"
    assert save_state(page_form)[3] == "warn", "the form taken away while changed: unsaved"
    show_view(page, "tiles")
    page.locator("#cube-problems .problems-sum").click()
    disk = item(page, "#cube-problems")
    assert disk.count() == 1 and disk.inner_text().strip() == UNSAVED[case.lang], f"the header's list shows the disk: {disk.all_inner_texts()}"
    disk.click()
    page.locator("#cube-page").wait_for(state="visible")
    name = page.locator(PAGE_PANEL + " form.cube-form input[name=name]")
    assert wait_until(page, "document.activeElement === document.querySelector(%s)" % json.dumps(PAGE_PANEL + " form.cube-form input[name=name]"), 5), \
        "the focus in the form's first field"
    assert name.input_value() == "Kept across pages", "with what was typed"
    show_view(page, "system")
    _, form = open_panel(page)
    assert form.locator("input[name=name]").input_value() == "Kept across pages", "and back in System settings"
    form.locator(".form-reset").click()
    assert page.locator("#cube-problems .problems-list .problem-action").count() == 0, "Reset takes the disk away"


def test_forgotten_when_the_cube_goes(page, case):
    """A Cube the list no longer gives takes what was typed for it: back
    in the list, its form shows the stored values and no disk."""
    panel, form = open_panel(page)
    form.locator("input[name=name]").fill("Typed for a Cube that goes")
    leave(page)
    wait_save(page, form, [False, True, False, "warn"], "left")

    def none(route):
        try:
            all_ = route.fetch().json()
            route.fulfill(status=200, content_type="application/json", body=json.dumps([c for c in all_ if c.get("configured") is False]))
        except Exception:  # the page closed
            pass

    page.route("**/api/v1/cubes", none)
    try:
        page.evaluate("loadCubes()")
        assert wait_until(page, "!state.cubes.length", 5), "the list gives no Cube"
    finally:
        page.unroute("**/api/v1/cubes")
    page.evaluate("loadCubes()")
    assert wait_until(page, "state.cubes.some((c) => c.serial === 'KEQ0000100')", 5), "the Cube back"
    panel, form = open_panel(page)
    assert form.locator("input[name=name]").input_value() == "Fake Cube", "its stored name"
    assert save_state(form) == [True, False, False, ""], "clean"
    assert item(page, PANEL + " > summary .cube-pill").count() == 0, "no disk"


@pytest.mark.parametrize("how", ["reset", "saved"])
def test_taken_away_after_reset_or_save(page, case, how):
    """A form reset and at once taken away, as a Cancel that resets and
    closes does (Cube settings shown in the same task as the click, so
    that the form is not drawn again before it goes), or one saved and
    then taken away: nothing is left unsaved, no disk anywhere, and the
    form shows the stored values where it is drawn next."""
    panel, form = open_panel(page)
    form.locator("input[name=name]").fill("Typed, then " + how)
    leave(page)
    wait_save(page, form, [False, True, False, "warn"], "left")
    page.route(CUBE_PATH, lambda route: route.fulfill(status=200, content_type="application/json", body="{}")
               if route.request.method == "PUT" else route.continue_())
    try:
        if how == "reset":
            page.evaluate("(sel) => { document.querySelector(sel).click(); showView('cube'); }", PANEL + " form.cube-form .form-reset")
        else:
            form.locator("button[type=submit]").click()
            wait_save(page, form, [True, False, False, "ok"], "saved")
            page.evaluate("() => showView('cube')")
        page_form = page.locator(PAGE_PANEL + " form.cube-form")
        page_form.wait_for(state="visible")
        page.wait_for_timeout(300)
        assert page_form.locator("input[name=name]").input_value() == "Fake Cube", "the stored name"
        assert save_state(page_form)[3] in ("", "ok"), f"no warning on Save: {save_state(page_form)}"
        assert page.locator(PAGE_PANEL + " > summary .cube-pill .problem-action").count() == 0, "no disk in the panel's pill"
        assert page.locator("#cube-problems .problem-action").count() == 0, "no disk in the header's list"
    finally:
        page.unroute(CUBE_PATH)
