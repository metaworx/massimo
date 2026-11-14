# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The server's sign before the time of the last update (app.js
renderServerSign): connected, an update out, a new version running and
the server gone, each its icon in a frame and a label that says what a
click does."""
import json

from helpers import show_view, wait_until

# The middle of an element's digits: its baseline less half the height of
# a 0 in its font.
DIGITS_MIDDLE = """(sel) => {
  const e = document.querySelector(sel);
  const probe = document.createElement('span');
  probe.style.display = 'inline-block';
  e.appendChild(probe);
  const base = probe.getBoundingClientRect().bottom;
  probe.remove();
  const ctx = document.createElement('canvas').getContext('2d');
  ctx.font = getComputedStyle(e).font;
  return base - ctx.measureText('0').actualBoundingBoxAscent / 2;
}"""


def sign(page):
    """The sign's state, its icon and its label."""
    s = page.locator("#server-sign")
    return s.get_attribute("data-state"), s.locator("use").get_attribute("href"), s.get_attribute("aria-label")


def test_server_connected_and_update(page, case):
    """Connected: the plug in a green frame on the digits' middle, before
    the time. An update out, for the owner who may install it: a blue
    "i", its label naming the version; a click opens Software update."""
    state, icon, label = sign(page)
    assert (state, icon) == ("up", "#icon-plug-on") and label == page.evaluate("t('server.up')"), "connected"
    b = page.locator("#server-sign").bounding_box()
    assert b["x"] + b["width"] <= page.locator("#updated").bounding_box()["x"], "before the time"
    if case.width >= 600:
        middle = page.evaluate(DIGITS_MIDDLE, "#updated")
        assert abs(b["y"] + b["height"] / 2 - middle) <= 2, f"on the digits' middle ({b}, {middle})"
    real = page.evaluate("api('GET', 'api/v1/update')")
    out = dict(real, available=True, latest="v9.9.9")
    page.route("**/api/v1/update*", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(out)))
    try:
        page.evaluate("state.updateAt = 0; updateDue()")
        assert wait_until(page, "document.getElementById('server-sign').dataset.state === 'info'", 5), "an update out"
        state, icon, label = sign(page)
        assert icon == "#icon-info" and "v9.9.9" in label, f"a blue i, the version named ({label})"
        page.locator("#server-sign").click()
        page.locator("#sys-software").wait_for(state="visible")
        assert page.evaluate("isShown('system')"), "a click opens Software update"
    finally:
        page.unroute("**/api/v1/update*")
        page.evaluate("updState.info = null; renderServerSign()")
        show_view(page, "tiles")


def test_server_new_version(page, case):
    """The server runs another version while something is open: an amber
    triangle that blinks, and a click reloads the page."""
    show_view(page, "prefs")  # something open: no reload by itself
    real = page.evaluate("api('GET', 'healthz')")
    page.route("**/healthz", lambda route: route.fulfill(status=200, content_type="application/json",
                                                         body=json.dumps(dict(real, version="massimo v9.9.9-test"))))
    page.evaluate("checkVersion()")
    assert wait_until(page, "document.getElementById('server-sign').dataset.state === 'new'", 5), "a new version"
    state, icon, label = sign(page)
    assert icon == "#mark-warn" and label == page.evaluate("t('server.new')"), "a triangle, a click reloads"
    blink = page.evaluate("getComputedStyle(document.querySelector('#server-sign use.mark')).animationName")
    assert blink == "markblink", "the triangle blinks"
    page.evaluate("window.scrollTo({top: 0, behavior: 'instant'})")  # the header whole, the sign in it
    page.locator("#server-sign").wait_for(state="visible")
    with page.expect_navigation():
        page.locator("#server-sign").click()
    page.unroute("**/healthz")


def test_server_gone(page, case, watch):
    """The server gone: the open plug on a filled red square that blinks,
    the page's other signs greyed, the time that of the last update; back
    at the next answer."""
    watch.expected.append(("/healthz", 503))
    page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.some((d) => d.type === "shutter_contact"));
      const d = JSON.parse(JSON.stringify(r.devices.find((d) => d.type === "shutter_contact")));
      d.state.window_open = true;
      updateDevice(d);
    }""")
    before = page.locator("#updated").inner_text()
    page.route("**/healthz", lambda route: route.fulfill(status=503, body="gone"))
    try:
        page.evaluate("checkVersion()")
        assert wait_until(page, "document.getElementById('server-sign').dataset.state === 'down'", 5), "gone"
        state, icon, label = sign(page)
        assert icon == "#icon-plug-off" and str(page.evaluate("state.settings.poll")) in label, f"the open plug, the retry named ({label})"
        assert page.evaluate("getComputedStyle(document.getElementById('server-sign')).backgroundColor") != "rgba(0, 0, 0, 0)", "filled"
        assert page.evaluate("getComputedStyle(document.querySelector('#server-sign .ni')).animationName") == "markblink", "it blinks"
        assert "grayscale" in page.evaluate("getComputedStyle(document.querySelector('main .problems-sum')).filter"), "the other signs greyed"
        assert page.locator("#updated").inner_text() == before, "the time of the last update"
    finally:
        page.unroute("**/healthz")
    page.evaluate("checkVersion()")
    assert wait_until(page, "document.getElementById('server-sign').dataset.state === 'up'", 5), "back at the next answer"
    assert not page.evaluate("document.body.classList.contains('server-down')"), "the signs in colour again"
