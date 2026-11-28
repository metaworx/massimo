# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Each person's window and radio icons (AP IconChoice): the opening sash
(the default) or the paned window, the Wi-Fi fan (the default) or the
tower, each with its marks, in a tile's head, in its problem list and as
a window contact's type icon; the choice in My settings → Appearance,
kept as the other settings are, for the person or for this device.

The states come from state events applied in the page (updateDevice),
since the fake Cube does not change its states, and are told to the
page's reads of the rooms as well, the devices kept (Reads.device; AP
ToldData, Block 4): a server's own state event of one draws the
server's, keep's read the told ones again, so each look at the head's
icons waits for them, and the list is opened again where a redraw
closed it. The first tests set the choice in the page's settings and
draw the signs again."""
import json

from conftest import server_holds_settings
from helpers import settled, show_view, wait_until
from test_room_more import open_list

# Chooses the drawings in the page's settings and draws what shows them
# again, as a change of the choice does (theme.js redrawSigns); no choice
# is no value: the settings are a live object, whose keys a delete does
# not take away (ui/src/shared/data.ts).
CHOOSE = """([win, radio]) => {
  state.settings.iconWindow = win || undefined;
  state.settings.iconRadio = radio || undefined;
  redrawSigns();
}"""

# Gives the room with the window contact a window open for `ago` minutes
# (the server's limits of 5 and 10) and its first thermostat a lost link
# (a warning); returns the room's id, the contact's radio address and
# window, and the thermostat's radio address.
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
  return { id: r.id, contact: w.rf_address, window: w.window, thermostat: th ? th.rf_address : null };
}"""


def problems(page, reads, ago):
    """PROBLEMS in the page and told to the rooms' read, the contact and
    the thermostat kept (Reads.device); returns the room's id."""
    got = page.evaluate(PROBLEMS, ago)
    reads.device(got["contact"], state={"window_open": True}, window=got["window"])
    if got["thermostat"]:
        reads.device(got["thermostat"], state={"link_error": True})
    return got["id"]


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


def check(page, reads, rid, drawn):
    """The room's head, its list and its device list drawn as chosen, each
    look at the head's icons waited for (settled), the list opened again
    until it shows the contact as chosen."""
    room = page.locator('.room[data-id="%s"]' % rid)
    head = room.locator(".tile-head .problems-sum")

    def shows(ok, what):
        held, got = settled(page, lambda: icons(head), ok)
        assert held, f"{what} ({got})"

    problems(page, reads, 11)
    shows(lambda i: sorted(i) == sorted([drawn["radio"], drawn["bad"]]), "the radio amber, the window red")
    problems(page, reads, 6)
    shows(lambda i: drawn["warn"] in i, "the window amber")
    problems(page, reads, 1)
    shows(lambda i: drawn["open"] in i, "the window open briefly, plain")
    open_list(room, """(([type, open]) => (d) => [...d.querySelectorAll('.problem-entry')].some((e) => !!e.querySelector('.flag.fill-open')
      && e.querySelector(':scope > .problem-icon use')?.getAttribute('href') === type
      && e.querySelector('.flag.fill-open use')?.getAttribute('href') === open))(%s)""" % json.dumps([drawn["type"], drawn["open"]]),
              "the contact's type icon in the list, the open window's badge led by its icon")
    page.keyboard.press("Escape")
    room.locator(".room-name-btn").click()
    row = room.locator(".device", has=page.locator(".device-type-icon")).filter(
        has=page.locator(".flag.fill-open")).first
    assert row.locator(".device-type-icon use").get_attribute("href") == drawn["type"], "the contact's type icon in the device list"
    room.locator(".room-name-btn").click()


def test_the_defaults(page, case, reads):
    """Without a choice: the opening sash and the Wi-Fi fan."""
    page.evaluate(CHOOSE, [None, None])
    rid = problems(page, reads, 11)
    check(page, reads, rid, DRAWN[("sash", "wifi")])


def test_the_other_drawings(page, case, reads):
    """The paned window and the tower; back to the sash and the fan, the
    rooms drawn again."""
    page.evaluate(CHOOSE, ["paned", "tower"])
    rid = problems(page, reads, 11)
    check(page, reads, rid, DRAWN[("paned", "tower")])
    page.evaluate(CHOOSE, ["sash", "wifi"])
    check(page, reads, rid, DRAWN[("sash", "wifi")])


def preview(page, which):
    """The symbols of a choice's preview in My settings."""
    return icons(page.locator("#icon-%s-preview" % which))


def test_the_choice_in_my_settings(page, case, reads):
    """My settings → Appearance: the opening sash and the Wi-Fi fan
    chosen, each previewed as the page draws it; the paned window and the
    tower chosen, kept in the browser, the previews and the rooms drawn
    again; kept across a reload; kept for this device; the defaults
    stored as nothing."""
    show_view(page, "prefs")
    win, radio = page.locator("#icon-window-select"), page.locator("#icon-radio-select")
    assert (win.input_value(), radio.input_value()) == ("sash", "wifi"), "the defaults"
    assert preview(page, "window") == [DRAWN[("sash", "wifi")][k] for k in ("type", "open", "warn", "bad")], \
        f"the sash: closed, open, amber, red ({preview(page, 'window')})"
    assert preview(page, "radio") == ["#icon-radio-small+#mark-corner-warn", "#icon-radio-small+#mark-corner-x"], \
        f"the fan, amber and red ({preview(page, 'radio')})"
    win.select_option("paned")
    radio.select_option("tower")
    assert page.evaluate("[localStorage.getItem('massimo.iconWindow'), localStorage.getItem('massimo.iconRadio')]") == ["paned", "tower"], \
        "the choices kept"
    assert preview(page, "window")[0] == "#icon-window" and preview(page, "radio")[0] == "#icon-tower-small+#mark-tower-warn", \
        f"the previews drawn again ({preview(page, 'window')}, {preview(page, 'radio')})"
    show_view(page, "tiles")
    rid = problems(page, reads, 11)
    check(page, reads, rid, DRAWN[("paned", "tower")])
    # the server holds none of the settings, so that the reload takes the browser's, whichever of the reload's read and
    # the push of the choices comes first, the defaults the page's start pushed not among them
    server_holds_settings(False)
    page.reload()
    page.wait_for_selector(".room", timeout=10000)
    assert page.evaluate("[state.settings.iconWindow, state.settings.iconRadio]") == ["paned", "tower"], "kept across a reload"
    show_view(page, "prefs")
    page.locator("input[data-device-only='iconWindow']").evaluate("(i) => i.closest('label').click()")
    assert wait_until(page, "JSON.parse(localStorage.getItem('massimo.device') || '{}').iconWindow === 'paned'", 5), \
        "kept for this device"
    win.select_option("sash")
    assert page.evaluate("JSON.parse(localStorage.getItem('massimo.device')).iconWindow") == "sash", "this device's value changed"
    page.locator("input[data-device-only='iconWindow']").evaluate("(i) => i.closest('label').click()")
    assert wait_until(page, "!('iconWindow' in JSON.parse(localStorage.getItem('massimo.device') || '{}'))", 5), "the device's value gone"
    assert win.input_value() == "paned", "the person's value again"
    win.select_option("sash")
    radio.select_option("wifi")
    assert page.evaluate("[localStorage.getItem('massimo.iconWindow'), localStorage.getItem('massimo.iconRadio')]") == [None, None], \
        "the defaults stored as nothing"
    show_view(page, "tiles")
