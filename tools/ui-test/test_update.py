# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings → Software: the update channel, the buttons while the
update helper works, and Update asking in the page. GET /api/v1/update is
stubbed, and the setting update.channel where the channel is chosen, and
GET /api/v1/restart where the next start refuses the settings file;
nothing is installed."""
import json

import pytest

from helpers import VIEWER, frames, show_view, update_json, wait_held, wait_until


def test_busy_while_the_helper_installs(page):
    """While the update helper works, both buttons of the Software section
    are greyed out, also when the page only learns it from GET
    /api/v1/update (a reload during an update, or an update started from
    another browser), and come back once the helper reports done."""
    show_view(page, "maint")
    page.route("**/api/v1/update*", lambda route: route.fulfill(
        status=200, content_type="application/json", body=update_json("installing", "v0.0.1-test", True)))
    page.locator("#update-check").click()
    assert wait_until(page, "document.getElementById('update-check').disabled && document.getElementById('update-btn').disabled", 5), \
        "Software buttons greyed out while the helper installs"
    page.unroute("**/api/v1/update*")
    page.route("**/api/v1/update*", lambda route: route.fulfill(
        status=200, content_type="application/json", body=update_json("done", "v0.0.1-test", False)))
    assert wait_until(page, "!document.getElementById('update-check').disabled && !document.getElementById('update-btn').disabled", 10), \
        "Software buttons back once the helper is done"
    page.unroute("**/api/v1/update*")


def test_update_asks_in_the_page(page):
    """Update asks in the Software section, not in a browser dialog, and
    Cancel takes the question back without a request."""
    show_view(page, "maint")
    posts = []
    page.route("**/api/v1/update*", lambda route: posts.append(1) if route.request.method == "POST" else route.fulfill(
        status=200, content_type="application/json", body=update_json("up-to-date", "v0.0.1-test", True)))
    page.locator("#update-check").click()
    btn = page.locator("#update-btn")
    btn.wait_for(state="visible")
    btn.click()
    box = page.locator(".update .confirm-box")
    box.wait_for()  # once Update has read whether the next start refuses the settings file
    assert box.is_visible() and "v0.0.1-test" in box.inner_text() and btn.is_hidden(), "Update asks in the page"
    box.locator(".confirm-no").click()
    assert page.locator(".update .confirm-box").count() == 0 and btn.is_visible() and not posts, "Cancel takes it back, nothing sent"
    page.unroute("**/api/v1/update*")


def test_update_through_apt(page):
    """On a box the Debian package installed, Software names apt beside
    the version, and Update asks without promising the old version back:
    apt puts nothing back."""
    show_view(page, "maint")
    page.route("**/api/v1/update*", lambda route: route.fulfill(
        status=200, content_type="application/json", body=update_json("up-to-date", "v0.0.1-test", True, "apt")))
    page.locator("#update-check").click()
    assert wait_until(page, "document.getElementById('update-text').textContent.includes('apt')", 5), \
        "Software names apt"
    btn = page.locator("#update-btn")
    btn.wait_for(state="visible")
    plain = page.evaluate("t('update.confirm', {version: 'v0.0.1-test'})")
    btn.click()
    box = page.locator(".update .confirm-box")
    box.wait_for()
    assert box.is_visible() and "apt" in box.inner_text() and plain not in box.inner_text(), \
        "Update asks about apt, not with the hand install's rollback"
    box.locator(".confirm-no").click()
    page.unroute("**/api/v1/update*")


def test_version_line_from_the_source(page):
    """On a box apt updates, a release is offered once apt has it: one
    out on GitHub that apt has not yet is named as such, without Update;
    one the helper found nothing to install for is held back; an index
    that cannot be read says why."""
    show_view(page, "maint")
    body = json.loads(update_json("up-to-date", "v0.0.1-test", False, "apt"))
    body.update({"current": "v0.0.1-test", "latest": "v0.0.1-test", "released": "v0.0.2-test",
                 "release_url": "https://example.invalid/releases/v0.0.2-test"})
    page.route("**/api/v1/update*", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(body)))
    page.locator("#update-check").click()
    text = page.locator("#update-text")
    released = page.evaluate("t('update.released', {version: 'v0.0.2-test', manager: 'apt'})")
    assert wait_until(page, "document.getElementById('update-text').textContent.includes(%s)" % json.dumps(released), 5), \
        "the line names the release apt has not yet: %r" % text.text_content()
    assert page.evaluate("t('update.upToDate')") not in text.text_content(), "not up to date beside it"
    assert not page.locator("#update-btn").is_visible(), "no Update"
    assert "v0.0.2-test" in page.locator("#update-link").text_content(), "the notes are the release's"
    page.unroute("**/api/v1/update*")

    body.update({"latest": "v0.0.2-test", "held": "v0.0.2-test"})
    del body["released"]
    page.route("**/api/v1/update*", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(body)))
    page.locator("#update-check").click()
    held = page.evaluate("t('update.held', {version: 'v0.0.2-test'})")
    assert wait_until(page, "document.getElementById('update-text').textContent.includes(%s)" % json.dumps(held), 5), \
        "the line says the version is held back: %r" % text.text_content()
    assert not page.locator("#update-btn").is_visible(), "no Update while held"
    page.unroute("**/api/v1/update*")

    del body["held"], body["latest"]
    body["source_error"] = "GET https://massimo.example.invalid/apt/dists/alpha/main/binary-armhf/Packages.gz: 503"
    page.route("**/api/v1/update*", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(body)))
    page.locator("#update-check").click()
    failed = page.evaluate("t('update.sourceFailed', {manager: 'apt', error: %s})" % json.dumps(body["source_error"]))
    assert wait_until(page, "document.getElementById('update-text').textContent.includes(%s)" % json.dumps(failed), 5), \
        "the line says the index could not be read: %r" % text.text_content()
    page.unroute("**/api/v1/update*")


def channel_routes(page, manager=None, variable=None, suite=None):
    """Stubs GET /api/v1/update and the setting update.channel, the build's
    flavour alpha and no channel stored; variable is the channel
    MASSIMO_UPDATE_CHANNEL sets, suite the one apt follows. Returns the
    bodies of the settings PUTs."""
    state = {"stored": None}
    puts = []

    def effect():
        return variable or state["stored"] or suite or "alpha"

    def update(route):
        route.fulfill(status=200, content_type="application/json",
                      body=update_json("up-to-date", "v0.0.1-test", False, manager, effect(), suite))

    def settings(route):
        if route.request.method == "PUT":
            body = json.loads(route.request.post_data)
            puts.append(body)
            state["stored"] = body.get("update.channel")
            route.fulfill(status=200, content_type="application/json", body="{}")
            return
        view = {"value": state["stored"] or "alpha", "default": "alpha", "set": state["stored"] is not None,
                "in_effect": effect(), "applies": "now"}
        route.fulfill(status=200, content_type="application/json", body=json.dumps({"update.channel": view}))

    page.route("**/api/v1/update*", update)
    page.route("**/api/v1/settings", settings)
    return puts


def unroute_channel(page):
    page.unroute("**/api/v1/update*")
    page.unroute("**/api/v1/settings")


def test_channel_is_chosen(page):
    """A holder of settings chooses the update channel under Software: the
    choice is stored as update.channel, the build's flavour as null, and
    the release check asks again."""
    puts = channel_routes(page)
    show_view(page, "maint")
    sel = page.locator("#update-channel")
    sel.wait_for(state="visible")
    assert sel.is_enabled() and sel.input_value() == "", "the build's flavour is chosen, and can be changed"
    assert sel.locator("option").count() == 7, "the build's flavour and six channels"
    sel.select_option("beta")
    assert wait_until(page, "document.getElementById('update-text').textContent.includes('beta')", 5), \
        "the release check follows beta"
    assert puts == [{"update.channel": "beta"}], "beta is stored"
    assert wait_until(page, "document.getElementById('update-channel').value === 'beta'", 5) and sel.is_enabled(), "beta stays chosen"
    sel.select_option("")
    assert wait_until(page, "!document.getElementById('update-text').textContent.includes('beta')", 5), \
        "the release check is back on the build's flavour"
    assert puts == [{"update.channel": "beta"}, {"update.channel": None}], f"the build's flavour is stored as null ({puts})"
    unroute_channel(page)


def test_channel_on_apt(page):
    """On a box apt updates the box follows apt's suite until a channel is
    chosen, which the next update switches apt to: the note says so."""
    puts = channel_routes(page, manager="apt", suite="devel")
    show_view(page, "maint")
    sel = page.locator("#update-channel")
    sel.wait_for(state="visible")
    first = page.evaluate("t('update.channelSuite', {suite: 'devel'})")
    note = page.locator("#update-channel-note")
    assert sel.is_enabled() and sel.input_value() == "", "apt's suite is followed, and can be changed"
    assert sel.locator("option").first.inner_text() == first, "the first choice names apt's suite"
    assert note.inner_text() == page.evaluate("t('update.channelApt', {channel: 'devel'})"), "the note names the suite"
    sel.select_option("beta")
    switch = page.evaluate("t('update.channelAptSwitch', {suite: 'devel', channel: 'beta'})")
    assert wait_until(page, "document.getElementById('update-channel-note').textContent === %s" % json.dumps(switch), 5), \
        "the note says the next update switches apt to beta"
    assert puts == [{"update.channel": "beta"}], "beta is stored"
    unroute_channel(page)


def test_channel_fixed_by_the_variable(page):
    """MASSIMO_UPDATE_CHANNEL overrides the setting: the channel it sets is
    shown, fixed, and the note names the variable."""
    channel_routes(page, variable="nightly")
    show_view(page, "maint")
    sel = page.locator("#update-channel")
    sel.wait_for(state="visible")
    assert wait_until(page, "document.getElementById('update-channel').disabled", 5), "the channel is fixed"
    assert sel.input_value() == "nightly", "the variable's channel is shown"
    assert "MASSIMO_UPDATE_CHANNEL" in page.locator("#update-channel-note").inner_text(), "the note names the variable"
    unroute_channel(page)


def test_last_updates(page):
    """Software update lists the last three updates under the version
    lines, each the date first, then the version with its channel in
    brackets, then who: the newest still under way with who started it,
    one from outside Massimo through apt, one the helper rolled back after
    it ran, which says so, from before the entries kept their channel; a
    fourth is left out, and the list is gone without a history."""
    show_view(page, "maint")
    history = [
        {"at": "2026-11-14T18:00:00Z", "by": {"name": "anna", "display_name": "Anna"}, "from": "v0.0.2", "to": "v0.0.3",
         "channel": "devel"},
        {"at": "2026-11-10T07:00:00Z", "outside": True, "manager": "apt", "from": "v0.0.1", "channel": "alpha",
         "reached": "v0.0.2", "reached_at": "2026-11-10T07:00:00Z"},
        {"at": "2026-11-05T12:00:00Z", "by": {"name": "ben"}, "from": "v0.0.1", "to": "v0.0.2", "reached": "v0.0.2",
         "reached_at": "2026-11-05T12:01:00Z", "result": "rolled-back"},
        {"at": "2026-11-01T12:00:00Z", "from": "v0.0.0", "to": "v0.0.1", "reached": "v0.0.1", "reached_at": "2026-11-01T12:01:00Z"},
    ]
    body = json.loads(update_json("up-to-date", "v0.0.1-test", False))
    body["history"] = history
    page.route("**/api/v1/update*", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(body)))
    page.locator("#update-check").click()
    lines = page.locator("#update-history-list li")
    assert wait_until(page, "document.querySelectorAll('#update-history-list li').length === 3", 5), "three lines"
    texts = lines.all_text_contents()
    expect = page.evaluate("""() => {
        const when = (at) => fmtInstant(new Date(Date.parse(at)));
        const channel = (version, channel) => t('update.history.withChannel', {version, channel});
        return [
            [when('2026-11-14T18:00:00Z'), t('update.history.running', {version: channel('v0.0.3', 'devel')}),
             t('update.history.by', {name: 'Anna'})],
            [when('2026-11-10T07:00:00Z'), channel('v0.0.2', 'alpha'), t('update.history.outsideWith', {manager: 'apt'})],
            [when('2026-11-05T12:01:00Z'), t('update.history.ended', {version: 'v0.0.2', state: t('update.state.rolled-back')}),
             t('update.history.by', {name: 'ben'})],
        ].map((parts) => parts.join(' · '));
    }""")
    assert texts == expect, "the lines: %r, not %r" % (texts, expect)
    assert "v0.0.3 (devel)" in texts[0] and "v0.0.2 (alpha)" in texts[1], "the channel in brackets: %r" % texts
    assert page.locator("#update-history h4").is_visible(), "the heading"

    page.unroute("**/api/v1/update*")
    page.route("**/api/v1/update*", lambda route: route.fulfill(
        status=200, content_type="application/json", body=update_json("up-to-date", "v0.0.1-test", False)))
    page.locator("#update-check").click()
    assert wait_until(page, "document.getElementById('update-history').classList.contains('hidden')", 5), \
        "no list without a history"
    page.unroute("**/api/v1/update*")


def test_update_backup_line(page):
    """Where an update is out, a line under the version says that the
    server takes a backup of itself first, or, without a key, that the
    update goes on without one; where the last update's backup failed it
    says why, and Update without a backup asks as Update does, saying that
    no backup is taken first, and sends no_backup. The last updates name
    the backup each took first, or that it took none."""
    show_view(page, "maint")
    body = json.loads(update_json("up-to-date", "v0.0.1-test", True))
    body["backup"] = {"state": "ready"}
    posts = []

    def handle(route):
        if route.request.method == "POST":
            posts.append(json.loads(route.request.post_data or "null"))
            route.fulfill(status=202, content_type="application/json",
                          body=json.dumps({"ok": True, "requested": "v0.0.1-test", "channel": "alpha"}))
            return
        route.fulfill(status=200, content_type="application/json", body=json.dumps(body))

    def line_is(key, **values):
        text = page.evaluate("([k, v]) => t(k, v)", [key, values])
        assert wait_until(page, "document.getElementById('update-backup').textContent === %s" % json.dumps(text), 5), \
            "the line says %s: %r" % (key, page.locator("#update-backup").text_content())

    page.route("**/api/v1/update*", handle)
    page.locator("#update-check").click()
    line_is("update.backup.ready")
    assert not page.locator("#update-no-backup").is_visible(), "no Update without a backup while the backup works"

    body["backup"] = {"state": "no_key"}
    page.locator("#update-check").click()
    line_is("update.backup.noKey")
    assert not page.locator("#update-no-backup").is_visible(), "none without a key either"

    body["backup"] = {"state": "failed", "error": "no space left on device", "at": "2026-11-27T10:00:00Z"}
    body["history"] = [
        {"at": "2026-11-20T12:00:00Z", "from": "v0.0.0-test", "to": "v0.0.1-test", "channel": "alpha", "auto": True,
         "backup": "01a21e07-8cd9-72f9-a4b2-54277e08f80e", "reached": "v0.0.1-test", "reached_at": "2026-11-20T12:01:00Z"},
        {"at": "2026-11-10T12:00:00Z", "from": "v0.0.0-test", "to": "v0.0.0-test", "backup_skipped": "no_key",
         "reached": "v0.0.0-test", "reached_at": "2026-11-10T12:01:00Z"},
    ]
    page.locator("#update-check").click()
    line_is("update.backup.failed", error="no space left on device")
    without = page.locator("#update-no-backup")
    without.wait_for(state="visible")
    assert without.text_content() == page.evaluate("t('update.withoutBackup')"), "the button's text"
    texts = page.locator("#update-history-list li").all_text_contents()
    backup = page.evaluate("t('update.history.backup', {id: '54277e08f80e'})")
    no_key = page.evaluate("t('update.history.noKey')")
    assert len(texts) == 2 and texts[0].endswith(" · " + backup) and texts[1].endswith(" · " + no_key), \
        "the last updates name their backups: %r" % texts

    without.click()
    box = page.locator(".update .confirm-box")
    box.wait_for()
    assert page.evaluate("t('update.confirmNoBackup')") in box.inner_text(), "the question says no backup is taken: %r" % box.inner_text()
    assert box.locator(".confirm-yes").text_content() == page.evaluate("t('update.withoutBackup')"), "Yes says it"
    assert not posts, "nothing sent before Yes"
    box.locator(".confirm-yes").click()
    assert wait_until(page, "document.getElementById('update-helper').textContent === %s"
                      % json.dumps(page.evaluate("t('update.requested', {version: 'v0.0.1-test'})")), 5), "requested"
    assert posts == [{"no_backup": True}], "Update without a backup sends no_backup: %r" % posts
    page.unroute("**/api/v1/update*")


def test_update_being_started(page):
    """While the server's backup runs before the helper is asked, the
    helper's line says so and both buttons are off, until it is asked; a
    start that failed after the answer, not at its backup, says why."""
    show_view(page, "maint")
    body = json.loads(update_json("done", "v0.0.1-test", True))
    body["backup"] = {"state": "ready"}
    body["starting"] = {"step": "backup", "version": "v0.0.1-test", "since": "2026-11-27T10:00:00Z"}
    body["can_update"] = False
    page.route("**/api/v1/update*", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(body)))
    page.locator("#update-check").click()
    backing = page.evaluate("t('update.backingUp', {version: 'v0.0.1-test'})")
    assert wait_until(page, "document.getElementById('update-helper').textContent === %s" % json.dumps(backing), 5), \
        "the line says the backup runs: %r" % page.locator("#update-helper").text_content()
    assert wait_until(page, "document.getElementById('update-check').disabled", 5), "Check for updates off while it is started"

    del body["starting"]
    body["can_update"] = True
    body["start_error"] = {"error": "asking the update helper: permission denied", "at": "2026-11-27T10:01:00Z"}
    not_started = page.evaluate("t('update.notStarted', {error: 'asking the update helper: permission denied'})")
    assert wait_until(page, "document.getElementById('update-helper').textContent === %s" % json.dumps(not_started), 40), \
        "the line says why it did not start: %r" % page.locator("#update-helper").text_content()
    assert wait_until(page, "!document.getElementById('update-check').disabled", 40), "the buttons back"
    page.unroute("**/api/v1/update*")


def auto_routes(page, blocked=None, waits=None):
    """Stubs GET /api/v1/update with an update out, the automatic update
    as the settings stand and its next try, and the settings update.channel,
    update.auto and update.auto_window, off and 03:00-05:00 until a PUT
    stores another; blocked is the reason updates cannot be started,
    waits what the next try waits for at night (auto.waits).
    Returns the bodies of the settings PUTs."""
    state = {"update.auto": None, "update.auto_window": None}
    defaults = {"update.auto": "off", "update.auto_window": "03:00-05:00"}
    puts = []

    def value(key):
        return state[key] or defaults[key]

    def views():
        out = {"update.channel": {"value": "alpha", "default": "alpha", "set": False, "in_effect": "alpha", "applies": "now"}}
        for key in state:
            out[key] = {"value": value(key), "default": defaults[key], "set": state[key] is not None, "in_effect": value(key), "applies": "now"}
        return out

    def update(route):
        body = json.loads(update_json("up-to-date", "v0.0.2-test", True))
        body["auto"] = {"mode": value("update.auto"), "window": value("update.auto_window")}
        if value("update.auto") == "night":
            body["auto"]["next"] = "2026-11-25T03:00:00+01:00"
            if waits:
                body["auto"]["waits"] = waits
        if blocked:
            body.update({"blocked": blocked, "can_update": False})
        route.fulfill(status=200, content_type="application/json", body=json.dumps(body))

    def settings(route):
        if route.request.method == "PUT":
            body = json.loads(route.request.post_data)
            puts.append(body)
            for key, v in body.items():
                if key in state:
                    state[key] = v
        route.fulfill(status=200, content_type="application/json", body=json.dumps(views()))

    page.route("**/api/v1/update*", update)
    page.route("**/api/v1/settings", settings)
    return puts


def test_automatic_updates(page):
    """Software update sets the automatic update, saved at once: off by
    default, saying updates wait for someone; at night it names the
    window, which comes up with its two ends, and when the box tries
    next, on the server's clock whatever the browser's zone; an end
    changed is saved as its field is left."""
    puts = auto_routes(page)
    show_view(page, "maint")
    sel = page.locator("#update-auto")
    sel.wait_for(state="visible")
    note = page.locator("#update-auto-note")
    assert sel.is_enabled() and sel.input_value() == "off", "off by default"
    assert note.text_content() == page.evaluate("t('update.autoOff')"), "the note says off"
    assert not page.locator("#update-window-start").is_visible(), "no window while off"

    sel.select_option("night")
    assert wait_until(page, "!!document.getElementById('update-window-start')", 5), "the window comes up"
    assert wait_held(page, puts), "night is written: the window comes up before its PUT"  # the request itself, not the window
    assert puts == [{"update.auto": "night"}], "night is stored: %r" % puts
    night = page.evaluate("t('update.autoNight', {from: '03:00', to: '05:00'})")
    assert wait_until(page, "document.getElementById('update-auto-note').textContent === %s" % json.dumps(night), 5), \
        "the note names the window: %r" % note.text_content()
    assert page.locator("#update-window-start").input_value() == "03:00" and page.locator("#update-window-end").input_value() == "05:00", \
        "the window's ends"
    when = page.evaluate("""new Intl.DateTimeFormat(i18n.lang, {year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit',
        minute: '2-digit', hour12: false, timeZone: 'UTC'}).format(new Date(Date.UTC(2026, 10, 25, 3, 0)))""")
    next_try = page.evaluate("t('update.autoNext', {time: %s})" % json.dumps(when))
    assert wait_until(page, "(document.getElementById('update-auto-next') || {}).textContent === %s" % json.dumps(next_try), 5), \
        "the next try on the server's clock: %r" % page.locator("#update-window-start").evaluate("e => e.closest('.field').textContent")
    assert "03:00" in next_try, "03:00, the server's time: %r" % next_try

    start = page.locator("#update-window-start")
    start.fill("23:30")
    start.blur()
    assert wait_until(page, "document.getElementById('update-window-start').value === '23:30'", 5), "the start kept"
    assert wait_held(page, puts, n=2), "the window is written as the field is left: %r" % puts
    assert puts[1:] == [{"update.auto_window": "23:30-05:00"}], "the window is stored as the field is left: %r" % puts
    unroute_channel(page)


def test_automatic_updates_wait_for_the_settings_file(page):
    """While the server's next start refuses the settings file, the line
    under the window says the next try waits until the file is fixed, in
    place of when it comes, and the window's start is described by it."""
    auto_routes(page, waits="settings")
    show_view(page, "maint")
    sel = page.locator("#update-auto")
    sel.wait_for(state="visible")
    sel.select_option("night")
    waiting = page.evaluate("t('update.autoWaitsSettings')")
    assert wait_until(page, "(document.getElementById('update-auto-next') || {}).textContent === %s" % json.dumps(waiting), 5), \
        "the line says the try waits for the file: %r" % page.locator("#update-window-start").evaluate("e => e.closest('.field').textContent")
    assert page.locator("#update-window-start").get_attribute("aria-describedby") == "update-auto-next", "the start is described by it"
    unroute_channel(page)


def test_automatic_updates_blocked(page):
    """Where updates cannot be started from here, the automatic update is
    off, with the reason."""
    auto_routes(page, blocked="no update helper installed")
    show_view(page, "maint")
    sel = page.locator("#update-auto")
    sel.wait_for(state="visible")
    reason = page.evaluate("t('update.autoBlocked', {reason: 'no update helper installed'})")
    assert wait_until(page, "document.getElementById('update-auto').disabled", 5), "the choice is off"
    assert page.locator("#update-auto-note").text_content() == reason, "the note says why"
    unroute_channel(page)


def test_automatic_line(page):
    """An update the server started itself at night says so in the last
    updates, where a person's name would stand."""
    show_view(page, "maint")
    body = json.loads(update_json("up-to-date", "v0.0.1-test", False))
    body["history"] = [{"at": "2026-11-25T02:00:00Z", "auto": True, "from": "v0.0.1", "to": "v0.0.2", "channel": "alpha",
                        "reached": "v0.0.2", "reached_at": "2026-11-25T02:01:00Z"}]
    page.route("**/api/v1/update*", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(body)))
    page.locator("#update-check").click()
    assert wait_until(page, "document.querySelectorAll('#update-history-list li').length === 1", 5), "one line"
    want = page.evaluate("""[fmtInstant(new Date(Date.parse('2026-11-25T02:01:00Z'))),
        t('update.history.withChannel', {version: 'v0.0.2', channel: 'alpha'}), t('update.history.auto')].join(' · ')""")
    got = page.locator("#update-history-list li").text_content()
    assert got == want, "the line: %r, not %r" % (got, want)
    page.unroute("**/api/v1/update*")


def ask_routes(page):
    """Stubs GET /api/v1/update with v0.0.2-test due at night and the
    automatic update's question asked, and PUT /api/v1/update/answer: now
    answers as POST /api/v1/update does, tonight and skip end the
    question. Returns the bodies of the answers."""
    state = {"ask": True}
    puts = []

    def body():
        b = json.loads(update_json("up-to-date", "v0.0.2-test", True))
        b["auto"] = {"mode": "night", "window": "03:00-05:00", "next": "2026-11-25T03:00:00+01:00", "ask": state["ask"]}
        return b

    def handle(route):
        if route.request.method != "PUT":
            route.fulfill(status=200, content_type="application/json", body=json.dumps(body()))
            return
        a = json.loads(route.request.post_data)
        puts.append(a)
        state["ask"] = False
        if a["answer"] == "now":
            route.fulfill(status=202, content_type="application/json", body=json.dumps({"ok": True, "requested": a["version"], "channel": "alpha"}))
            return
        b = body()
        b["auto"]["answer"] = {"version": a["version"], "answer": a["answer"], "at": "2026-11-24T20:00:00Z"}
        route.fulfill(status=200, content_type="application/json", body=json.dumps(b))

    page.route("**/api/v1/update*", handle)
    page.route("**/api/v1/update/answer", handle)
    return puts


def unroute_ask(page):
    page.unroute("**/api/v1/update*")
    page.unroute("**/api/v1/update/answer")


def test_update_question_banner(page):
    """Where automatic updates are on and an update is due, a holder of
    update is asked under the header, on any page: the version, the
    window, and Install now, Tonight, Skip this version; Tonight answers
    for the box and the banner goes."""
    puts = ask_routes(page)
    page.evaluate("state.updateAt = 0; updateDue()")
    banner = page.locator("#update-ask")
    banner.wait_for(state="visible")
    text = page.evaluate("t('update.ask.text', {version: 'v0.0.2-test', from: '03:00', to: '05:00'})")
    assert page.locator("#update-ask-text").text_content() == text, "the question: %r" % banner.text_content()
    for answer, key in (("now", "update.ask.now"), ("tonight", "update.ask.tonight"), ("skip", "update.ask.skip")):
        assert page.locator("#update-ask-" + answer).text_content() == page.evaluate("t(%s)" % json.dumps(key)), answer
    page.locator("#update-ask-tonight").click()
    banner.wait_for(state="hidden")
    assert puts == [{"version": "v0.0.2-test", "answer": "tonight"}], "Tonight answered: %r" % puts
    unroute_ask(page)


def test_update_question_in_software(page):
    """Software update asks the same: Skip this version there answers for
    the box, and the question goes from the card and from under the
    header; Install now starts the update, which the card then says."""
    puts = ask_routes(page)
    show_view(page, "maint")
    page.locator("#update-check").click()
    page.locator("#update-card-ask").wait_for(state="visible")
    assert page.locator("#update-ask").is_visible(), "the banner too"
    page.locator("#update-card-ask-skip").click()
    page.locator("#update-card-ask").wait_for(state="hidden")
    page.locator("#update-ask").wait_for(state="hidden")
    assert puts == [{"version": "v0.0.2-test", "answer": "skip"}], "Skip answered: %r" % puts
    unroute_ask(page)

    puts = ask_routes(page)
    page.locator("#update-check").click()
    page.locator("#update-card-ask").wait_for(state="visible")
    page.locator("#update-card-ask-now").click()
    requested = page.evaluate("t('update.requested', {version: 'v0.0.2-test'})")
    assert wait_until(page, "document.getElementById('update-helper').textContent === %s" % json.dumps(requested), 5), \
        "the card says the update was requested: %r" % page.locator("#update-helper").text_content()
    assert puts == [{"version": "v0.0.2-test", "answer": "now"}], "Install now answered: %r" % puts
    unroute_ask(page)


def test_update_question_not_for_viewer(page):
    """A user without update is asked nothing, though the server asks."""
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(VIEWER)))
    page.evaluate("loadMe()")
    assert wait_until(page, "!hasCap('update') && !hasCap('settings')", 5), "the viewer's capabilities"
    ask_routes(page)
    assert page.evaluate("MassimoParts.software.load(false).then((u) => !!(u && u.auto && u.auto.ask))"), "the server asks"
    frames(page)
    assert not page.locator("#update-ask").is_visible(), "no banner for a viewer"
    unroute_ask(page)
    page.unroute("**/api/v1/me")


# What the next start refuses of the settings file, as GET /api/v1/restart
# gives it to a holder of update without restart: the restart not
# available, its refusals as a holder of restart sees them (AP
# StartConfiguration, D34-A).
REFUSAL = [{"settings": ["MASSIMO_TRUSTED_PROXIES"],
            "error": '-trusted-proxies or MASSIMO_TRUSTED_PROXIES: "10.0.0.300" is neither an address nor a CIDR'}]


def restart_route(page, answer, held=None):
    """Stubs GET /api/v1/restart, which serves Maintenance's own read too:
    answer["refused"] the next start's refusals, a list, or the status
    the read fails with, read at each request; with held, a read is held
    there once answer["hold"] is set, until the test lets it go
    (restart_answer)."""

    def handle(route):
        if held is not None and answer.get("hold"):
            held.append(route)
            return
        restart_answer(route, answer["refused"])

    page.route("**/api/v1/restart", handle)


def restart_answer(route, refused):
    """Answers a read of GET /api/v1/restart with refused, a list of the
    next start's refusals, or the status it fails with."""
    if isinstance(refused, int):
        route.fulfill(status=refused, content_type="application/json", body='{"error": "the settings file could not be checked"}')
        return
    route.fulfill(status=200, content_type="application/json", body=json.dumps({
        "available": False, "blocked": 'capability "restart" required', "file": "/etc/massimo/env",
        "read_at": "2026-11-25T10:00:00Z", "refused": refused}))


def maint_read(page):
    """Maintenance shown, with its own read of GET /api/v1/restart answered:
    the settings file it names is in its line, or the restart's line says
    why it could not ask."""
    show_view(page, "maint")
    assert wait_until(page, "document.getElementById('settings-file-text').textContent.includes('/etc/massimo/env')"
                      " || document.getElementById('restart-text').textContent.includes('could not be checked')", 5), \
        "Maintenance's read answered"


def update_question(page):
    """Update pressed, its question's text once it shows, taken back by Cancel."""
    page.locator("#update-btn").click()
    box = page.locator(".update .confirm-box")
    box.wait_for()
    text = box.locator(".confirm-text").text_content()
    box.locator(".confirm-no").click()
    assert page.locator(".update .confirm-box").count() == 0, "the question taken back"
    return text


def test_update_question_says_the_server_stays_down(page, watch):
    """While the server's next start refuses the settings file, Update
    reads it at the press, turning while it reads, once, and its question
    ends with the sentence that the server does not come back until the
    file is fixed (AP AutoUpdate, D12-A, D13-A), though the restart is not
    available to this caller: the answer's refused decides. Without a
    refusal Update asks as before, and after a failed read too."""
    watch.expected.append(("/api/v1/restart", 500))  # the failed read is the test's
    answer = {"refused": REFUSAL}
    held = []
    restart_route(page, answer, held)
    page.route("**/api/v1/update*", lambda route: route.fulfill(
        status=200, content_type="application/json", body=update_json("up-to-date", "v0.0.1-test", True)))
    try:
        maint_read(page)
        btn = page.locator("#update-btn")
        btn.wait_for(state="visible")
        plain, sentence = page.evaluate("[t('update.confirm', {version: 'v0.0.1-test'}), t('restart.confirmRefused')]")

        answer["hold"] = True
        btn.click()
        assert wait_held(page, held), "Update reads GET /api/v1/restart"
        frames(page)
        assert page.evaluate("document.getElementById('update-btn').classList.contains('busy') && document.getElementById('update-btn').disabled"), \
            "Update turns, off, while it reads"
        assert page.locator(".update .confirm-box").count() == 0, "no question before the answer"
        answer["hold"] = False
        restart_answer(held[0], REFUSAL)
        box = page.locator(".update .confirm-box")
        box.wait_for()
        assert box.locator(".confirm-text").text_content() == plain + " " + sentence, \
            "the question with the sentence: %r" % box.locator(".confirm-text").text_content()
        assert len(held) == 1, "one read (%d)" % len(held)
        assert not btn.evaluate("b => b.classList.contains('busy')"), "Update turns no more"
        box.locator(".confirm-no").click()

        answer["refused"] = []
        assert update_question(page) == plain, "without a refusal the question as before"
        answer["refused"] = 500
        assert update_question(page) == plain, "after a failed read the question as before"
    finally:
        page.unroute("**/api/v1/update*")
        page.unroute("**/api/v1/restart")


def test_update_question_through_apt_says_the_server_stays_down(page):
    """On a box apt updates, Update's question names apt and how to go
    back, and the sentence after it while the next start refuses the
    settings file: apt puts nothing back, and the old version would not
    start either."""
    restart_route(page, {"refused": REFUSAL})
    page.route("**/api/v1/update*", lambda route: route.fulfill(
        status=200, content_type="application/json", body=update_json("up-to-date", "v0.0.1-test", True, "apt")))
    try:
        maint_read(page)
        page.locator("#update-btn").wait_for(state="visible")
        want = page.evaluate("t('update.confirmManaged', {version: 'v0.0.1-test', manager: 'apt'}) + ' ' + t('restart.confirmRefused')")
        got = update_question(page)
        assert got == want, "apt's question with the sentence: %r" % got
    finally:
        page.unroute("**/api/v1/update*")
        page.unroute("**/api/v1/restart")


def install_now_asks(page, place, puts):
    """Install now pressed in place, the banner (update-ask) or the card's
    copy (update-card-ask), while the next start refuses the settings
    file: it asks there in place of the three answers, the focus on
    Cancel, with the update's question and the sentence; Cancel gives the
    three back, the focus on Install now, with nothing sent."""
    now = page.locator(f"#{place}-now")
    now.click()
    box = page.locator(f"#{place} .confirm-box")
    box.wait_for()
    want = page.evaluate("t('update.confirm', {version: 'v0.0.2-test'}) + ' ' + t('restart.confirmRefused')")
    assert box.locator(".confirm-text").text_content() == want, "the question with the sentence: %r" % box.text_content()
    assert box.locator(".confirm-yes").text_content() == page.evaluate("t('update.ask.now')"), "Yes is Install now"
    assert box.locator(".confirm-no").text_content() == page.evaluate("t('action.cancel')"), "No is Cancel"
    assert wait_until(page, f"document.activeElement === document.querySelector('#{place} .confirm-no')", 5), "the focus on Cancel"
    assert all(page.locator(f"#{place}-{a}").is_hidden() for a in ("now", "tonight", "skip")), "in place of the three answers"
    box.locator(".confirm-no").click()
    assert page.locator(f"#{place} .confirm-box").count() == 0, "Cancel takes it back"
    assert all(page.locator(f"#{place}-{a}").is_visible() for a in ("now", "tonight", "skip")), "the three answers back"
    assert wait_until(page, f"document.activeElement.id === '{place}-now'", 5), "the focus back on Install now"
    frames(page)
    assert puts == [], "nothing sent: %r" % puts


def test_install_now_asks_in_the_banner_while_refused(page):
    """While the next start refuses the settings file, Install now under
    the header asks first (AP AutoUpdate, D14-A): Cancel sends nothing,
    and the question's Install now answers now; the banner goes."""
    restart_route(page, {"refused": REFUSAL})
    puts = ask_routes(page)
    try:
        page.evaluate("state.updateAt = 0; updateDue()")
        page.locator("#update-ask").wait_for(state="visible")
        install_now_asks(page, "update-ask", puts)
        page.locator("#update-ask-now").click()
        page.locator("#update-ask .confirm-yes").click()
        assert wait_held(page, puts), "Install now sent: %r" % puts
        assert puts == [{"version": "v0.0.2-test", "answer": "now"}], "Install now answered: %r" % puts
        page.locator("#update-ask").wait_for(state="hidden", timeout=10000)  # at the following's first read, 3 s on
    finally:
        unroute_ask(page)
        page.unroute("**/api/v1/restart")


def test_install_now_asks_in_the_card_while_refused(page):
    """Install now in Software update asks first as well, in the card's
    copy alone, the banner keeping its three answers; the question's
    Install now answers now, which the card then says."""
    restart_route(page, {"refused": REFUSAL})
    puts = ask_routes(page)
    try:
        maint_read(page)
        page.locator("#update-check").click()
        page.locator("#update-card-ask").wait_for(state="visible")
        install_now_asks(page, "update-card-ask", puts)
        page.locator("#update-card-ask-now").click()
        page.locator("#update-card-ask .confirm-box").wait_for()
        assert page.locator("#update-ask .confirm-box").count() == 0 and page.locator("#update-ask-now").is_visible(), \
            "the banner keeps its three answers"
        page.locator("#update-card-ask .confirm-yes").click()
        requested = page.evaluate("t('update.requested', {version: 'v0.0.2-test'})")
        assert wait_until(page, "document.getElementById('update-helper').textContent === %s" % json.dumps(requested), 5), \
            "the card says the update was requested: %r" % page.locator("#update-helper").text_content()
        assert puts == [{"version": "v0.0.2-test", "answer": "now"}], "Install now answered: %r" % puts
    finally:
        unroute_ask(page)
        page.unroute("**/api/v1/restart")


@pytest.mark.parametrize("place", ["update-ask", "update-card-ask"])
@pytest.mark.parametrize("refused", ["none", "failed"])
def test_install_now_at_once(page, watch, place, refused):
    """Without a refusal, and after a failed read, Install now answers now
    at once, under the header and in the card, asking nothing."""
    if refused == "failed":
        watch.expected.append(("/api/v1/restart", 500))  # the failed read is the test's
    restart_route(page, {"refused": [] if refused == "none" else 500})
    puts = ask_routes(page)
    try:
        if place == "update-ask":
            page.evaluate("state.updateAt = 0; updateDue()")
        else:
            maint_read(page)
            page.locator("#update-check").click()
        page.locator(f"#{place}").wait_for(state="visible")
        page.locator(f"#{place}-now").click()
        assert wait_held(page, puts), "Install now sent at once: %r" % puts
        assert puts == [{"version": "v0.0.2-test", "answer": "now"}], "Install now answered at once: %r" % puts
        assert page.locator(".update-ask .confirm-box, .update-card-ask .confirm-box").count() == 0, "asked nothing"
    finally:
        unroute_ask(page)
        page.unroute("**/api/v1/restart")
