# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A Cube's pairing follows the data in place (AP UiParts, Block 9): the
time typed, and a joined device's name and room, stay the elements they
were, with what is typed and the focus, through the countdown, a pairing
event, a device that joins and a reload of the Cubes; the countdown
changes its text in place. The pairing runs by the page's reads alone:
GET …/pairing is answered as running, with a device that joined (the
fixture reads), so that nothing is sent to the fake Cube. At a sign-out,
and for another user signed in after an expired session, nothing of a
Cube's pairing is left: the form is a new one, and the Cube's rooms are
asked for again."""
import json
from datetime import datetime, timedelta, timezone

from conftest import TOKEN, URL
from helpers import OPERATOR_PASSWORD, focus_in_turn, frames, hold, mark, owner_page, reads_of, replaced, show_view, sign_out, wait_held, wait_until
from test_login import sign_in
from test_unsaved import expire

SERIAL = "KEQ0000100"
PANEL = ".cube-panel[data-serial='%s']" % SERIAL
PAIRING = "**/api/v1/cubes/%s/pairing" % SERIAL
# Two devices that join, as GET …/pairing and a paired event give them.
JOINED = {"type": "radiator_thermostat", "rf_address": "0a0f02", "serial": "MEQ0000902"}
SECOND = {"type": "shutter_contact", "rf_address": "0a0f03", "serial": "MEQ0000903"}


def running(until):
    """The pairing state as running until until, JOINED among its devices."""
    def change(st):
        return dict(st or {}, active=True, until=until, paired=[JOINED])
    return change


def asked(page, call):
    """Runs call in the page and waits for the answer of the GET …/pairing it asks for."""
    with page.expect_response(lambda r: r.request.method == "GET" and r.url.endswith("/pairing")):
        page.evaluate(call)


def test_a_joined_device_keeps_what_is_typed(page, case, reads):
    """While the Cube pairs, a joined device's name typed and its room
    chosen, the focus in the name: the countdown moves on, a pairing
    event asks for the state, another device joins and the Cubes load
    again, and the form, its fields and the countdown are the elements
    they were, holding what was typed, the focus where it was."""
    until = (datetime.now(timezone.utc) + timedelta(minutes=5)).isoformat()
    reads.change(PAIRING, running(until))
    show_view(page, "system")
    panel = page.locator("#cubes-list " + PANEL)
    panel.locator(":scope > summary").click()
    row = panel.locator(".pair-device[data-rf='%s']" % JOINED["rf_address"])
    row.locator("form").wait_for(state="visible")
    assert wait_until(page, "document.querySelectorAll(%s).length > 1"
                      % json.dumps("#cubes-list %s .pair-device select[name=room] option" % PANEL), 5), "the Cube's rooms offered"
    base = "#cubes-list %s .pair-device[data-rf='%s']" % (PANEL, JOINED["rf_address"])
    held = [base + " form", base + " input[name=name]", base + " select[name=room]", "#cubes-list %s .pairing-countdown" % PANEL,
            "#cubes-list %s .pair-stop" % PANEL]
    name = row.locator("input[name=name]")
    room = row.locator("select[name=room]")
    room.select_option(index=1)
    chosen = room.input_value()
    name.fill("Hall radiator")
    mark(page, held)
    name.focus()
    countdown = "document.querySelector(%s).textContent" % json.dumps(held[3])
    was = page.evaluate(countdown)
    assert wait_until(page, "%s !== %s" % (countdown, json.dumps(was)), 5), "the countdown moves on"
    asked(page, "MassimoParts.cubes.pairingEvent(%s)" % json.dumps(SERIAL))
    page.evaluate("MassimoParts.cubes.pairingJoined(%s, %s)" % (json.dumps(SERIAL), json.dumps(SECOND)))
    panel.locator(".pair-device[data-rf='%s']" % SECOND["rf_address"]).wait_for(state="visible")
    page.evaluate("loadCubes()")
    frames(page)
    assert replaced(page, held) == [], "the pairing's elements are the ones they were"
    assert name.input_value() == "Hall radiator" and room.input_value() == chosen, "what was typed and chosen stays"
    assert name.evaluate("(el) => el === document.activeElement"), "the focus stays in the name"
    show_view(page, "tiles")


def test_the_time_typed_stays(page, case):
    """Pair a device opened on Cube settings and a time typed, the focus
    in it: a pairing event and a reload of the Cubes keep the field the
    element it was, with the time typed and the focus."""
    show_view(page, "cube")
    pairing = page.locator("#cube-page-body .cube-pairing")
    pairing.locator(".pair-open").click()
    secs = pairing.locator("input[name=seconds]")
    assert wait_until(page, "document.activeElement && document.activeElement.name === 'seconds'", 5), "the time has the focus"
    secs.fill("90")
    held = ["#cube-page-body .cube-pairing input[name=seconds]", "#cube-page-body .cube-pairing .pairing-form button[type=submit]",
            "#cube-page-body .cube-pairing .pairing-countdown"]
    mark(page, held)
    secs.focus()
    asked(page, "MassimoParts.cubes.pairingEvent(state.serial)")
    page.evaluate("loadCubes()")
    frames(page)
    assert replaced(page, held) == [], "the time's field and Start are the ones they were"
    assert secs.input_value() == "90", "the time typed stays"
    assert secs.evaluate("(el) => el === document.activeElement"), "the focus stays in the time"
    show_view(page, "tiles")


def with_pairing(me):
    """/me with pair, cubes, structure and rename among the capabilities:
    the operator, who signs in after the owner, sees the Cube's panel with
    its pairing and the form that puts a device into a room, as the page
    shows them (the server's own checks are not reached: nothing is sent)."""
    if me and isinstance(me.get("capabilities"), list):
        me["capabilities"] = sorted(set(me["capabilities"]) | {"pair", "cubes", "structure", "rename"})
    return me


def forgotten_after(new_context, guard, watch, leave):
    """The owner's page with a Cube pairing (GET …/pairing answered as
    running with JOINED), its form's name typed and a room chosen, the
    Cube's rooms offered; leave(page) ends the session, and the operator
    signs in. While the sign-in is asked no request of the pairing goes
    (its countdown does not run); then, the next user's rooms held (the
    page's load and the pairing's own read, which serves a Cube until the
    page's load has given its rooms), the form is a new element, with nothing typed and no room offered, and
    once they have come it offers the rooms the next user's load read: the
    chosen Cube's rooms are the page's own since UiParts D216, which the
    end of the session forgets and the next user's load reads again,
    where the pairing read them by a request of its own."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    reads = reads_of(page)
    until = (datetime.now(timezone.utc) + timedelta(minutes=5)).isoformat()
    reads.change(PAIRING, running(until))
    reads.change("**/api/v1/me", with_pairing)
    show_view(page, "system")
    panel = page.locator("#cubes-list " + PANEL)
    panel.locator(":scope > summary").click()
    sel = "#cubes-list %s .pair-device[data-rf='%s'] form" % (PANEL, JOINED["rf_address"])
    form = page.locator(sel)
    form.wait_for(state="visible")
    options = "document.querySelectorAll(%s).length" % json.dumps("#cubes-list %s .pair-device select[name=room] option" % PANEL)
    assert wait_until(page, options + " > 1", 5), "the Cube's rooms offered"
    form.locator("select[name=room]").select_option(index=1)
    form.locator("input[name=name]").fill("Hall radiator")
    mark(page, [sel])
    asked = []
    page.on("request", lambda r: asked.append(r.url) if r.method == "GET" and r.url.endswith("/pairing") else None)
    leave(page)
    assert wait_until(page, "document.getElementById('login-dialog').open", 5), "the sign-in asked for"
    since = len(asked)
    page.wait_for_timeout(3500)  # the countdown asks every third second where nothing reports it; nothing signals its absence
    assert len(asked) == since, f"no request of the pairing while the sign-in is asked: {asked[since:]}"
    held, sent = [], []
    hold(page, "**/api/v1/cubes/*/rooms?devices=all", "GET", held, sent)
    hold(page, "**/api/v1/cubes/*/rooms?devices=real", "GET", held, sent)
    try:
        # the sign-in by the dialog, the rooms' load held: sign_in waits for the tiles
        page.locator("#login-user").fill("operator")
        page.locator("#login-password").fill(OPERATOR_PASSWORD)
        page.locator("#login-form button[type=submit]").click()
        assert wait_until(page, "state.me && state.me.user && state.me.user.name === 'operator'", 10), "another user"
        assert wait_held(page, held), "the next user's rooms asked for, held"
        show_view(page, "system")
        panel = page.locator("#cubes-list " + PANEL)
        if panel.get_attribute("open") is None:
            panel.locator(":scope > summary").click()
        form.wait_for(state="visible")
        frames(page)
        assert replaced(page, [sel]) == [sel], "a new form, not the one typed into before"
        assert form.locator("input[name=name]").input_value() != "Hall radiator", "nothing typed left"
        assert page.evaluate(options) == 0, "no room offered from before while the next user's rooms are on their way"
    finally:
        page.unroute("**/api/v1/cubes/*/rooms?devices=all")
        page.unroute("**/api/v1/cubes/*/rooms?devices=real")
        for route in held:
            route.continue_()
    assert wait_until(page, options + " > 1", 5), "the rooms the next user's load read offered"


def test_sign_out_forgets_the_pairing(new_context, case, watch, guard):
    """Sign out with a pairing's form typed in: the next user finds a new
    form, the Cube's rooms asked for again."""
    forgotten_after(new_context, guard, watch, sign_out)


def test_another_user_after_a_401_forgets_the_pairing(new_context, case, watch, guard):
    """The session taken as expired with a pairing's form typed in:
    another user signed in finds a new form, the Cube's rooms asked for
    again."""
    forgotten_after(new_context, guard, watch, expire)


def test_pair_a_device_gives_the_time_the_focus_at_once(page, case):
    """Pair a device on Cube settings gives the focus to the time in the
    drawing that shows it, not a frame later. Fails on the page before,
    where an effect gave it a frame later."""
    show_view(page, "cube")
    pairing = "#cube-page-body .cube-pairing"
    try:
        assert focus_in_turn(page, pairing + " .pair-open", pairing + " input[name=seconds]"), \
            "the time takes the focus in the drawing that shows it"
    finally:
        page.locator(pairing + " .form-cancel").click()
    show_view(page, "tiles")


def test_put_into_a_room_gives_its_first_field_the_focus_at_once(page, case):
    """Put into a room… of a device without a room gives the focus to the
    form's first field in the drawing that shows it. Fails on the page
    before, where an effect gave it a frame later."""
    from test_sign_out_late import UNLISTED, unlisted_row
    row = unlisted_row(page)
    sel = "#unlisted .device[data-rf='%s']" % UNLISTED["rf_address"]
    try:
        assert focus_in_turn(page, sel + " .put-open", sel + " form.pair-device-form :is(input, select)"), \
            "the form's first field takes the focus in the drawing that shows it"
    finally:
        cancel = row.locator("form.pair-device-form .form-cancel")
        if cancel.count():
            cancel.click()
        reads_of(page).clear("**/api/v1/cubes/*/devices?devices=real")
        page.evaluate("loadRooms()")
