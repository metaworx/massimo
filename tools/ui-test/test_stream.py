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
