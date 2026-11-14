# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Each person's window and radio icons (AP IconChoice): the opening sash
(the default) or the paned window, the Wi-Fi fan (the default) or the
tower, each with its marks, in a tile's head, in its problem list and as
a window contact's type icon.

The states come from state events applied in the page (updateDevice),
since the fake Cube does not change its states; the choice is set in the
page's settings and the rooms drawn again."""

# Chooses the drawings in the page's settings and draws the rooms again.
CHOOSE = """([win, radio]) => {
  if (win) state.settings.iconWindow = win; else delete state.settings.iconWindow;
  if (radio) state.settings.iconRadio = radio; else delete state.settings.iconRadio;
  renderRooms();
}"""

# Gives the room with the window contact a window open for `ago` minutes
# (the server's limits of 5 and 10) and its first thermostat a lost link
# (a warning); returns the room's id.
PROBLEMS = """(ago) => {
  const r = state.rooms.find((r) => r.devices.some((d) => d.type === "shutter_contact"));
  const w = JSON.parse(JSON.stringify(r.devices.find((d) => d.type === "shutter_contact")));
  const since = Date.now() - ago * 60000;
  const iso = (m) => new Date(since + m * 60000).toISOString();
  w.state.window_open = true;
  w.window = { open_since: iso(0), warn_at: iso(5), alert_at: iso(10) };
  updateDevice(w);
  const th = r.devices.find(isThermostat);
  if (th) {
    const d = JSON.parse(JSON.stringify(th));
    d.state.link_error = true;
    updateDevice(d);
  }
  return r.id;
}"""

# The symbols of each choice: the window open (plain, marked amber,
# marked red), its contact's type icon; the radio marked amber.
DRAWN = {
    ("sash", "wifi"): {"open": "#icon-window-sash-open", "warn": "#icon-window-sash-open+#mark-sash-warn",
                       "bad": "#icon-window-sash-open+#mark-sash-x", "type": "#icon-window-sash",
                       "radio": "#icon-radio-small+#mark-corner-warn"},
    ("paned", "tower"): {"open": "#icon-window-open", "warn": "#icon-window-open-small+#mark-corner-warn",
                         "bad": "#icon-window-open-small+#mark-corner-x", "type": "#icon-window",
                         "radio": "#icon-tower-small+#mark-tower-warn"},
}


def icons(summary):
    """The icons of a problems' summary, each by its symbols."""
    return [svg.evaluate("(s) => [...s.querySelectorAll('use')].map((u) => u.getAttribute('href')).join('+')")
            for svg in summary.locator("svg").all()]


def check(page, rid, drawn):
    """The room's head, its list and its device list drawn as chosen."""
    room = page.locator('.room[data-id="%s"]' % rid)
    page.evaluate(PROBLEMS, 11)
    head = room.locator(".tile-head .problems-sum")
    assert sorted(icons(head)) == sorted([drawn["radio"], drawn["bad"]]), f"the radio amber, the window red ({icons(head)})"
    page.evaluate(PROBLEMS, 6)
    assert drawn["warn"] in icons(head), f"the window amber ({icons(head)})"
    page.evaluate(PROBLEMS, 1)
    assert drawn["open"] in icons(head), f"the window open briefly, plain ({icons(head)})"
    head.click()
    contact = room.locator(".tile-head .problem-entry", has=page.locator(".flag.fill-open"))
    assert contact.locator(":scope > .problem-icon use").get_attribute("href") == drawn["type"], "the contact's type icon in the list"
    assert contact.locator(".flag.fill-open use").get_attribute("href") == drawn["open"], "the open window's badge led by its icon"
    page.keyboard.press("Escape")
    room.locator(".room-name-btn").click()
    row = room.locator(".device", has=page.locator(".device-type-icon")).filter(
        has=page.locator(".flag.fill-open")).first
    assert row.locator(".device-type-icon use").get_attribute("href") == drawn["type"], "the contact's type icon in the device list"
    room.locator(".room-name-btn").click()


def test_the_defaults(page, case):
    """Without a choice: the opening sash and the Wi-Fi fan."""
    page.evaluate(CHOOSE, [None, None])
    rid = page.evaluate(PROBLEMS, 11)
    check(page, rid, DRAWN[("sash", "wifi")])


def test_the_other_drawings(page, case):
    """The paned window and the tower; back to the sash and the fan, the
    rooms drawn again."""
    page.evaluate(CHOOSE, ["paned", "tower"])
    rid = page.evaluate(PROBLEMS, 11)
    check(page, rid, DRAWN[("paned", "tower")])
    page.evaluate(CHOOSE, ["sash", "wifi"])
    check(page, rid, DRAWN[("sash", "wifi")])
