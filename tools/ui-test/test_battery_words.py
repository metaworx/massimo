# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The battery words (AP RefreshBatteries v1.0, Block 5, D12-D): a
battery level says what to do, "Batteries weak, replace soon" and
"Batteries low, replace now", while the API's codes stay ok, weak and
low (D13-A). The words in a device row's problem list and the tile's,
inside the list and the window at 390 px too, where they wrap in the
list's 240 px as a long badge does; in the log, the level's line with
the level before in a short word, and the device's flag as the battery
warning; in the notification settings and a notification's toast."""
import time

from playwright.sync_api import Error as PlaywrightError

from helpers import isolate_log, scroll_width, show_view, wait_log_live, wait_until
from test_new_batteries import Device

WORDS = {
    "en": {
        "weak": "Batteries weak, replace soon",
        "low": "Batteries low, replace now",
        "kind": "batteries to replace",
        "lines": ["Batteries ok (new batteries, before: low), by Anna",
                  "Batteries low, replace now (before: weak)",
                  "Batteries weak, replace soon (before: ok)"],
        "flags": ["battery warning off", "battery warning"],
        "body_low": "Radiator (Bath): replace the batteries now.",
        "body_weak": "Radiator (Bath): replace the batteries soon.",
        "help_low": "replace the batteries now.",
        "help_weak": "so replace them soon.",
    },
    "de": {
        "weak": "Batterien schwach, bald wechseln",
        "low": "Batterien leer, jetzt wechseln",
        "kind": "Batterien wechseln",
        "lines": ["Batterien ok (neue Batterien, vorher: leer), durch Anna",
                  "Batterien leer, jetzt wechseln (vorher: schwach)",
                  "Batterien schwach, bald wechseln (vorher: ok)"],
        "flags": ["Batteriewarnung aus", "Batteriewarnung"],
        "body_low": "Radiator (Bath): Batterien jetzt wechseln.",
        "body_weak": "Radiator (Bath): Batterien bald wechseln.",
        "help_low": "Batterien jetzt wechseln.",
        "help_weak": "also bald wechseln.",
    },
}

# A badge in its open problem list: whether the list is open, its box,
# the list's, the window's width, and the lines its words take. The lists of a device's row and
# a tile's head are 240 px wide at every width (.problems-list), so the
# longer words wrap there, inside the badge.
FIT = """(flag) => {
  const r = flag.getBoundingClientRect();
  const list = flag.closest(".problems-list").getBoundingClientRect();
  const text = [...flag.childNodes].find((n) => n.nodeType === 3 && n.textContent.trim());
  const range = document.createRange();
  range.selectNodeContents(text);
  const lines = new Set([...range.getClientRects()].map((x) => Math.round(x.top))).size;
  return { open: flag.closest("details").open, left: r.left, right: r.right, listLeft: list.left, listRight: list.right,
    vw: document.documentElement.clientWidth, lines, words: text.textContent };
}"""


def assert_badge(page, case, problems, words, where):
    """The problem list of problems (its details) opened shows the badge
    with words, whole: inside the list and the window, on two lines at
    most, without a horizontal scroll; the list closed again. A server's
    event of the kept device draws the server's level, and keep's read
    draws the told one again, each in a list of its own, closed: the list
    is opened again until the badge shows in an open list, and measured
    in that look (FIT)."""
    flag = problems.locator(".problems-list .flag", has_text=words).first
    end = time.time() + 5
    while True:
        try:
            if not problems.evaluate("(d) => d.open", timeout=1000):
                problems.locator(":scope > summary").click(timeout=1000)
            flag.wait_for(state="visible", timeout=1000)
            fit = flag.evaluate(FIT, timeout=1000)
            if fit["open"]:
                break
        except PlaywrightError:
            pass
        assert time.time() < end, f"{where}: the list open with its badge"
    assert fit["words"] == words, f"{where}: the badge says {fit['words']!r}, want {words!r}"
    assert fit["left"] >= fit["listLeft"] - 0.5 and fit["right"] <= fit["listRight"] + 0.5, f"{where}: inside its list ({fit})"
    assert fit["left"] >= 0 and fit["right"] <= fit["vw"] + 0.5, f"{where}: inside the window ({fit})"
    assert fit["lines"] <= 2, f"{where}: the words on two lines at most ({fit})"
    assert scroll_width(page) <= case.width, f"{where}: no horizontal scroll"
    page.keyboard.press("Escape")


def test_battery_words_in_the_problem_lists(page, case, reads):
    """A weak level's badge says to replace the batteries soon, a low
    one's now, in the device row's problem list and the tile head's,
    whole at every width; the row's problems name the kind as the
    batteries to replace."""
    words = WORDS[case.lang]
    dev = Device(page, reads)
    try:
        for level in ("weak", "low"):
            dev.told(level, level == "low")
            dev.open()
            label = dev.row.locator(".device-badges .problems-sum").get_attribute("aria-label") or ""
            assert label == page.evaluate("(list) => t('problem.label', { list })", words["kind"]), \
                f"{level}: the row's problems named as the batteries to replace ({label!r})"
            assert_badge(page, case, dev.row.locator(".device-badges details.problems"), words[level], f"{level}, the device's row")
            assert_badge(page, case, dev.tile.locator(".tile-head details.problems").first, words[level], f"{level}, the tile's head")
    finally:
        dev.told("ok", False)


def test_battery_words_in_the_log(page, case, reads):
    """The level's line says the level's words and the level before in
    a short word, new batteries and who marked them in it; the device's
    own flag is the battery warning, on and off, not a level."""
    words = WORDS[case.lang]
    show_view(page, "log")
    isolate_log(page, reads)
    wait_log_live(page)
    page.evaluate("""() => {
      const d = JSON.parse(JSON.stringify(state.rooms.flatMap((r) => r.devices).find((d) => isThermostat(d) && !d.virtual && d.serial)));
      const send = (kind, ev) => state.events.dispatchEvent(new MessageEvent(kind, { data: JSON.stringify(Object.assign({ kind,
        time: new Date().toISOString(), level: "info", device: d.rf_address, serial: d.serial }, ev)) }));
      send("state", { state: d, changes: { battery_low: { from: false, to: true } } });
      send("battery", { source: "device", summary: { level: "weak", from: "ok" } });
      send("battery", { source: "device", level: "warn", summary: { level: "low", from: "weak" } });
      send("state", { state: d, changes: { battery_low: { from: true, to: false } } });
      send("battery", { source: "device", by: { name: "anna", display_name: "Anna" }, summary: { level: "ok", from: "low", cause: "replaced" } });
    }""")
    assert wait_until(page, "document.querySelectorAll('#log-lines .log-line.kind-battery').length >= 3"
                      " && document.querySelectorAll('#log-lines .log-line.kind-state').length >= 2", 5), "the five lines"
    lines = page.locator("#log-lines .log-line.kind-battery .log-text")
    got = [lines.nth(i).inner_text() for i in range(3)]
    assert got == words["lines"], f"the level's lines, newest first: {got}"
    flags = page.locator("#log-lines .log-line.kind-state .log-text")
    got = [flags.nth(i).inner_text() for i in range(2)]
    assert got == words["flags"], f"the flag's lines, newest first: {got}"
    assert scroll_width(page) <= case.width, "no horizontal scroll"
    show_view(page, "tiles")


def test_battery_words_in_the_notifications(page, case):
    """The notification settings name the two kinds by the levels' words,
    their help says to replace the batteries; a notification's toast
    says the level and when to replace them."""
    words = WORDS[case.lang]
    show_view(page, "notify")
    for kind, level in (("battery_low", "low"), ("battery_weak", "weak")):
        row = page.locator(f"#notify-rules .notify-row[data-kind={kind}]")
        assert row.locator(".switch-label").inner_text() == words[level], f"{kind}: its switch"
        help_text = row.locator(".help-text").text_content()
        assert help_text == page.evaluate(f"t('help.notify.{kind}')") and words["help_" + level] in help_text, \
            f"{kind}: its help says to replace the batteries ({help_text!r})"
    assert scroll_width(page) <= case.width, "no horizontal scroll"
    show_view(page, "tiles")
    toast = page.locator("#toast")
    for i, (kind, level) in enumerate((("battery_low", "low"), ("battery_weak", "weak"))):
        n = {"kind": kind, "cube": "KEQ0000100", "device": "0a0103", "serial": "MEQ0000003", "device_name": "Radiator",
             "room": 3, "room_name": "Bath", "values": {"level": level}, "time": f"2026-11-23T20:0{i}:00Z", "url": "/"}
        page.evaluate("""(n) => state.events.dispatchEvent(new MessageEvent("notification", { data: JSON.stringify({
          kind: "notification", notification: n }) }))""", n)
        toast.wait_for(state="visible")
        assert page.locator("#toast-title").inner_text() == words[level], f"{kind}: the toast's title"
        assert page.locator("#toast-body").inner_text() == words["body_" + level], f"{kind}: the toast's body"
        page.locator("#toast-close").click()
        toast.wait_for(state="hidden")
