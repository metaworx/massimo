# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The server's sign, first in the header's pile (ui/src/header/Pile.tsx,
ServerSign.tsx): connected, an update out, a new version running and the
server gone, each its icon in its colour, named in the pile's label; what
the state offers to do is the second item of the pile's list."""
import json

from helpers import show_view, wait_until

SUM = "#cube-problems .problems-sum"
ITEM = "#cube-problems .problems-list .server-item"


def sign(page):
    """The sign's state, its icon and the pile's label."""
    return (page.locator(".pile").get_attribute("data-server"), page.locator(SUM + " .server-icon use").get_attribute("href"),
            page.locator(SUM).get_attribute("aria-label"))


def colour(page, var):
    """A colour variable of the page's style, as a computed colour."""
    return page.evaluate("""(v) => { const p = document.createElement('i'); p.style.color = 'var(' + v + ')';
      document.body.appendChild(p); const c = getComputedStyle(p).color; p.remove(); return c; }""", var)


def open_list(page):
    """Opens the pile's list, the header whole."""
    page.evaluate("window.scrollTo({top: 0, behavior: 'instant'})")
    page.locator(SUM).click()
    page.locator("#cube-problems .problems-list").wait_for(state="visible")


def test_server_connected_and_update(page, case):
    """Connected: the plug in green, first in the pill, the label opening
    with it; the list offers nothing but Refresh now. An update out, for
    the owner who may install it: a blue "i", the label naming the
    version; the list's second item opens Software update."""
    state, icon, label = sign(page)
    assert (state, icon) == ("up", "#icon-plug-on") and label.startswith(page.evaluate("t('server.up')")), f"connected ({label})"
    first = page.locator(SUM + " > svg").first
    assert "server-icon" in (first.get_attribute("class") or ""), "the sign first in the pill"
    assert first.evaluate("(e) => getComputedStyle(e).color") == colour(page, "--ok"), "in green"
    open_list(page)
    assert page.locator(ITEM).count() == 0, "nothing offered while connected"
    page.keyboard.press("Escape")
    real = page.evaluate("api('GET', 'api/v1/update')")
    out = dict(real, available=True, latest="v9.9.9")
    page.route("**/api/v1/update*", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(out)))
    try:
        page.evaluate("state.updateAt = 0; updateDue()")
        assert wait_until(page, "document.querySelector('.pile').dataset.server === 'info'", 5), "an update out"
        state, icon, label = sign(page)
        assert icon == "#icon-info" and "v9.9.9" in label, f"a blue i, the version named ({label})"
        open_list(page)
        items = page.locator("#cube-problems .problems-list > button")
        assert items.nth(1).get_attribute("class").find("server-item") >= 0 and "v9.9.9" in items.nth(1).inner_text(), \
            "the list's second item, Software update with the version"
        page.locator(ITEM).click()
        page.locator("#sys-software").wait_for(state="visible")
        assert page.evaluate("MassimoParts.pages.isShown('maint')"), "it opens Software update"
    finally:
        page.unroute("**/api/v1/update*")
        page.evaluate("MassimoParts.software.load(false)")  # the server's own answer again, which draws the sign
        show_view(page, "tiles")


def test_server_new_version(page, case):
    """The server runs another version while something is open: an amber
    triangle that blinks, and the list's second item reloads the page."""
    show_view(page, "prefs")  # something open: no reload by itself
    real = page.evaluate("api('GET', 'healthz')")
    page.route("**/healthz", lambda route: route.fulfill(status=200, content_type="application/json",
                                                         body=json.dumps(dict(real, version="massimo v9.9.9-test"))))
    page.evaluate("checkVersion()")
    assert wait_until(page, "document.querySelector('.pile').dataset.server === 'new'", 5), "a new version"
    state, icon, label = sign(page)
    assert icon == "#mark-warn" and label.startswith(page.evaluate("t('server.new')")), f"a triangle ({label})"
    blink = page.evaluate("getComputedStyle(document.querySelector('#cube-problems .server-icon use.mark')).animationName")
    assert blink == "markblink", "the triangle blinks"
    open_list(page)
    assert page.locator(ITEM).inner_text().strip() == page.evaluate("t('server.reload')"), "the reload offered"
    with page.expect_navigation():
        page.locator(ITEM).click()
    page.unroute("**/healthz")


def test_server_gone(page, case, watch):
    """The server gone: the open plug, the pill filled red, the plug
    blinking, the page's other signs greyed, the chosen Cube's age still
    shown, as the server last told it; back at the next answer."""
    watch.expected.append(("/healthz", 503))
    page.evaluate("""() => {
      const r = state.rooms.find((r) => r.devices.some((d) => d.type === "shutter_contact"));
      const d = JSON.parse(JSON.stringify(r.devices.find((d) => d.type === "shutter_contact")));
      d.state.window_open = true;
      updateDevice(d);
    }""")
    page.route("**/healthz", lambda route: route.fulfill(status=503, body="gone"))
    try:
        page.evaluate("checkVersion()")
        assert wait_until(page, "document.querySelector('.pile').dataset.server === 'down'", 5), "gone"
        state, icon, label = sign(page)
        assert icon == "#icon-plug-off" and str(page.evaluate("state.settings.poll")) in label, f"the open plug, the retry named ({label})"
        assert page.evaluate(f"getComputedStyle(document.querySelector('{SUM}')).backgroundColor") == colour(page, "--fill-bad"), \
            "the pill filled red"
        assert page.evaluate("getComputedStyle(document.querySelector('#cube-problems .server-icon')).animationName") == "markblink", "it blinks"
        assert "grayscale" not in page.evaluate("getComputedStyle(document.querySelector('#cube-problems .server-icon')).filter"), \
            "the plug itself in colour"
        assert "grayscale" in page.evaluate("getComputedStyle(document.querySelector('main .problems-sum')).filter"), "the other signs greyed"
    finally:
        page.unroute("**/healthz")
    page.evaluate("checkVersion()")
    assert wait_until(page, "document.querySelector('.pile').dataset.server === 'up'", 5), "back at the next answer"
    assert not page.evaluate("document.body.classList.contains('server-down')"), "the signs in colour again"
