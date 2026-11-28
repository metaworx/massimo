# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The event stream opens with tickets, never with the token, and after
a refused stream (a used ticket, which the server answers with 401) the
page opens it again with a new ticket; a ticket asked for one Cube opens
no other Cube's stream."""
import json

import pytest
from playwright.sync_api import Error

from conftest import api_call
from helpers import USED, safe_route, wait_held, wait_until

SERIAL = "KEQ0000100"
# A second Cube the test adds to the server, which never connects.
SECOND = "KEQ0000800"


@pytest.mark.used_ticket
def test_new_ticket_after_a_401(page, watch):
    for _ in range(25):  # the first stream request follows the ticket's
        if watch.streams:
            break
        page.wait_for_timeout(200)
    streams, refused = watch.streams, watch.refused
    assert len(refused) == 1 and streams and streams[0].endswith("ticket=" + USED), "the first stream request had the used ticket"
    assert wait_until(page, "state.eventsOpen", 10), "event stream open again after a 401"
    assert len(streams) >= 2, "a new stream request after the 401"
    assert all("ticket=" in u and "token=" not in u for u in streams), "stream opened with tickets only"
    tickets = [u.split("ticket=", 1)[1] for u in streams if "ticket=" in u]
    assert len(set(tickets)) == len(tickets), "a new ticket for every stream request"


def test_stream_handlers_catch_their_reloads(page, watch):
    """The events that make the page load again (hello, metadata,
    device_forgotten, connected, disconnected) leave no rejected promise
    uncaught when the load fails, as when the server is briefly gone; the
    test makes the loads fail and puts them back."""
    page.evaluate("""() => {
      window.realLoads = { loadCubes, loadRooms, refresh };
      const fail = () => Promise.reject(new Error("a reload that failed"));
      window.loadCubes = fail;
      window.loadRooms = fail;
      window.refresh = fail;
      for (const kind of ["hello", "metadata", "device_forgotten", "connected", "disconnected"]) {
        state.events.dispatchEvent(new MessageEvent(kind, { data: JSON.stringify({ kind, time: new Date().toISOString() }) }));
      }
    }""")
    page.wait_for_timeout(500)
    page.evaluate("() => Object.assign(window, window.realLoads)")
    uncaught = [p for p in watch.problems if "a reload that failed" in p]
    watch.problems[:] = [p for p in watch.problems if p not in uncaught]
    assert not uncaught, f"the reloads' failures are caught: {uncaught}"


def let_go(route):
    """Lets a held request of a ticket go to the server, answers it with
    the server's answer, and returns its ticket."""
    answer = route.fetch()
    ticket = answer.json()["ticket"]
    route.fulfill(response=answer)
    return ticket


def test_a_ticket_across_a_cube_switch(page, watch):
    """The stream's ticket asked for one Cube and answered once another is
    shown: the page drops it and asks for the shown Cube's, whose stream
    it opens with it (AP SteadyTests v1.2, Block 20). The first ticket is
    held across the switch, and the second Cube's rooms until the shown
    Cube's stream is open, so that the switch's own reconnect, which
    waits for them (selectCube), comes after. Fails on the page before,
    which opened the shown Cube's stream with the other Cube's ticket,
    a 401 in the console."""
    tickets, rooms = [], []

    def ticket(route):
        if route.request.method == "POST":
            tickets.append(route)
        else:
            route.fallback()

    added = held = False
    try:
        api_call("POST", "api/v1/cubes", {"serial": SECOND, "address": "127.0.0.1:9", "name": "Second"})
        added = True
        page.evaluate("loadCubes()")
        assert wait_until(page, "state.cubes.some((c) => c.serial === %s)" % json.dumps(SECOND), 5), "the second Cube listed"
        routed = safe_route(page, "**/events/ticket", ticket)
        rooms_routed = safe_route(page, "**/api/v1/cubes/%s/rooms?devices=all" % SECOND, rooms.append)
        held = True
        page.evaluate("() => { connectEvents(); }")
        assert wait_held(page, tickets), "the shown Cube's ticket asked for, held"
        assert SERIAL in tickets[0].request.url, f"for the shown Cube: {tickets[0].request.url}"
        page.evaluate("(s) => { selectCube(s); }", SECOND)
        assert wait_held(page, rooms), "the second Cube shown, its rooms asked for and held"
        since = len(watch.streams)
        let_go(tickets[0])
        assert wait_held(page, tickets, n=2), f"a ticket asked for again, the streams opened: {watch.streams[since:]}"
        assert SECOND in tickets[1].request.url, f"for the Cube shown: {tickets[1].request.url}"
        own = let_go(tickets[1])
        assert wait_until(page, "state.eventsOpen", 5), "the stream open"
        opened = watch.streams[since:]
        assert opened and all(f"/cubes/{SECOND}/events?" in u and "ticket=" + own in u for u in opened), \
            f"the shown Cube's stream opened with its own ticket alone: {opened}"
    finally:
        if held:
            page.unroute("**/events/ticket", routed)
            page.unroute("**/api/v1/cubes/%s/rooms?devices=all" % SECOND, rooms_routed)
            for route in tickets + rooms:
                try:
                    route.fallback()  # to the server, where the test has not let it go
                except Error:
                    pass  # answered already
        page.evaluate("(s) => { if (state.serial !== s) selectCube(s); }", SERIAL)
        wait_until(page, "state.serial === %s && state.eventsOpen" % json.dumps(SERIAL), 10)  # the first Cube's stream again
        if added:
            api_call("DELETE", "api/v1/cubes/" + SECOND)
        page.evaluate("loadCubes()")
