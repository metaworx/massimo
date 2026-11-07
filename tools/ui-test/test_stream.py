# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The event stream opens with tickets, never with the token, and after
a refused stream (a used ticket, which the server answers with 401) the
page opens it again with a new ticket."""
import pytest

from helpers import USED, wait_until


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
