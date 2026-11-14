# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The problem list, the same at every level (components.js problemsEl,
problemEntry): a heading whose own sign is fine is listed for the
problems under it, its badges stand under its name, and the entries under
it are indented to the name, a line down from its icon."""

# A list as AP CubeBadges builds it: a Cube with its own warning and,
# under it, a device of its with a failure; a Cube that is fine, its
# device with a warning; a Cube without any problem, not listed.
LIST = """() => {
  const dev = (title, tone, text) => ({ icon: "icon-radiator", title, sub: "MEQ0000101",
    items: [{ kind: "radio", icon: "icon-radio", sign: "radio", tone, text }] });
  const el = problemsEl([
    { sign: { kind: "net", state: "warn" }, title: "Cube held", sub: "KEQ0000100",
      items: [{ kind: "net", icon: "icon-net", sign: "net", tone: "warn", text: "held" }],
      subs: [dev("Radiator one", "bad", "transmit error")] },
    { sign: { kind: "net", state: "" }, title: "Cube fine", sub: "KEQ0000200", items: [], subs: [dev("Radiator two", "warn", "link error")] },
    { sign: { kind: "net", state: "" }, title: "Cube quiet", sub: "KEQ0000300", items: [], subs: [] },
  ], "problems");
  el.id = "test-problems";
  document.querySelector("main").prepend(el);
  el.open = true;
}"""

BOXES = """() => {
  const b = (e) => e.getBoundingClientRect();
  const list = document.querySelector("#test-problems .problems-list");
  return [...list.children].map((entry) => ({
    icon: b(entry.querySelector(":scope > .problem-icon")),
    name: b(entry.querySelector(":scope > .problem-who .problem-name")),
    flags: entry.querySelector(":scope > .problem-flags") ? b(entry.querySelector(":scope > .problem-flags")) : null,
    sub: entry.querySelector(":scope > .problem-subs .problem-entry .problem-icon")
      ? b(entry.querySelector(":scope > .problem-subs .problem-entry .problem-icon")) : null,
    line: getComputedStyle(entry.querySelector(":scope > .problem-subs") || entry).borderLeftStyle,
    lineX: entry.querySelector(":scope > .problem-subs") ? b(entry.querySelector(":scope > .problem-subs")).left : null,
    green: entry.querySelector(":scope > .problem-icon").classList.contains("tone-warn") ? "" : getComputedStyle(entry.querySelector(":scope > .problem-icon")).color,
  }));
}"""


def test_one_entry_format(page, case):
    """A Cube listed with its own warning and its device under it; a Cube
    whose own sign is fine listed, green, for its device's warning; a Cube
    without problems not listed. At every level the badges stand under
    the name, the entries under an entry start at its name, and a line
    runs down from its icon's middle."""
    page.evaluate(LIST)
    try:
        boxes = page.evaluate(BOXES)
        assert len(boxes) == 2, f"two Cubes listed, the quiet one not ({len(boxes)})"
        ok = page.evaluate("""() => { const p = document.createElement('span'); p.style.color = 'var(--ok)';
          document.body.appendChild(p); const c = getComputedStyle(p).color; p.remove(); return c; }""")
        assert boxes[1]["green"] == ok, f"the fine Cube's sign in green ({boxes[1]['green']} vs {ok})"
        held, fine = boxes
        assert abs(held["flags"]["left"] - held["name"]["left"]) <= 1, "the badges under the name, not the icon"
        for e in boxes:
            assert abs(e["sub"]["left"] - e["name"]["left"]) <= 1, f"the device's icon under the Cube's name ({e})"
            assert e["line"] == "solid", "a line down from the Cube's icon"
            middle = e["icon"]["left"] + e["icon"]["width"] / 2
            assert abs(e["lineX"] + 1 - middle) <= 1.5, f"from the icon's middle ({e['lineX']} vs {middle})"
        assert page.locator("#test-problems .problems-sum svg").count() == 2, "the summary: the net and the radio"
        assert page.locator("#test-problems .problem-entry").count() == 4, "two Cubes, a device under each"
        assert page.evaluate("document.documentElement.scrollWidth") <= case.width, "within the page's width"
    finally:
        page.evaluate("document.getElementById('test-problems').remove()")
