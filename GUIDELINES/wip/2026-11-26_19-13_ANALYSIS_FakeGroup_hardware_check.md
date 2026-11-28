# ANALYSIS FakeGroup: Set group and remove group on the test Cube

2026-11-26, session "Fix flaky tests" (fork for AP FakeGroup),
on the owner's `EXEC check` with D6, D7, D10-A, D11-A and D12-B.
The test Cube (firmware 1.1.3), through the proxy of the owner's Windows
service; one radiator thermostat paired to it, called "the thermostat" here,
alone in room 1 at the start.
Every line was built by `internal/protocol/cube`; times are local.

## TL;DR

- 0x23 naming the thermostat's own room → its C line's room byte 0
  within 45 s; slot back, no error flag
- 0x23 naming a room it is not in → refused by the thermostat:
  room byte unchanged, error flag set (failed radiogram 0x23)
- so the fake's design is (c): 0x22 sets the byte,
  0x23 clears it only where it names the current room,
  and otherwise flags the device
- a room's wakeup does not reach a device in no group:
  the original software's move of such a device failed that way
- a room setpoint addressed through the thermostat reached it although it
  was in no group, and cost 17 % of the duty cycle
- `z:1e` kept the thermostat awake at 70 s and not at 4 minutes:
  neither 30 seconds nor 30 minutes

## The steps

| Time | Lines (through the proxy) | Answer | Then |
|---|---|---|---|
| 17:34 | 0x23 room 1, no reset, no wakeup (direct, service stopped) | `S:01,1,32` discarded | the thermostat carried the error flag from a failed 0x03 before the check (no adaptation run yet) |
| 18:15 | `z:1e,G,01`, 0x23 room 1 | `A:`, `S:03,0,30` | 18:16: room byte **0**, slots 50, no flag |
| 18:37 | room 1 manual 19.5 °C, addressed through the thermostat | `S:03,0,31` | display and L entry 19.5 °C manual; slots back within 77 s; duty 3 → **20 %** |
| 18:54 | the original software's move to a new room (room 7): `z:1e,G,01`, 0x23 room 1, `m:` without the thermostat, `z:1e,G,07`, 0x22 room 7, `c:`, `z:1e,G,07`, 0x11 | all accepted but the 0x11 (discarded) | room byte still 0, error flag (failed 0x23); the Cube's room list without the thermostat, no room 7; the service showed "Gerät ohne Raum", the original software room 7 with it |
| 18:57 | `r:01,<address in hex>` (malformed: the address is base64), `z:1e,D,<thermostat>` | `S:17,1,32` refused, `A:` | duty +3 %, one slot held |
| 18:58 | `r:01,<address in base64>`, `z:1e,D,<thermostat>`, 0x22 room 7 | `S:1a,0,31`, `A:`, `S:1a,0,2f` | 18:59: room byte **7**, flag cleared; duty unchanged at 26 % |
| 19:00 | `m:` = the present list plus a room 7 with the thermostat | `A:` | room list and C line agree on room 7 |
| 19:00 | `z:1e,G,07`, 0x23 room 1 | `A:`, `S:1d,0,30` | room byte **7** for 4 minutes; error flag set (failed 0x23); duty 29 % |
| 19:05 | `r:01,<address in base64>` | `S:1d,0,32` | flag cleared |

The thermostat ends in room 7 without a flag;
room 1 stays in the Cube's list without devices, as the original software
left it.

## What it shows

**0x22** sets the C line's room byte to its group,
here also into a room the list did not have yet,
once the device hears it (18:58, after a device wakeup).

**0x23** clears the byte where it names the device's current room (18:15).
Naming another room, the device refuses it: the byte stays,
and the Cube flags the device with failed radiogram 0x23 (19:00).
A wakeup of the named room does not change that.

**A room's wakeup reaches its members only.**
After the 0x23 of 18:15 the thermostat was in no group;
the original software's `z:1e,G,01` and `z:1e,G,07` did not wake it,
its 0x23 room 1 failed and its 0x22 room 7 did not take.
A device wakeup (`z:1e,D,<address>`) did:
the 0x22 right after it took.
This matches the hardware day of 2026-11-16,
where a move's 0x23 failed and the C line kept the old room.

**For the server:** a move names the room list's room in its 0x23
(`moveDevice`, `buildPlace`); where the C line names another room,
or none, the device refuses it and is flagged, and the move's 0x22 is
sent to a device that a room's wakeup did not wake.
Naming the C line's room, and waking the device itself where it is in
no group, would follow what the Cube showed.

**A room setpoint** addressed through the thermostat (as the server
addresses a room) reached it although it was in no group;
whether a member not named in the address would take it was not tested.
It cost 17 % of the duty cycle in one radiogram.

**The service** refused room 1's setpoint while the C lines named no
thermostat for it: "room 1 has no thermostat", in English on a German page
(a task is offered for the translation).

## The `z:` duration (for "Massimo Fixer 1", AP ClockWakeup)

| Time | Line | Duty | Slots |
|---|---|---|---|
| 19:06:11 | `z:1e,G,07` | 29 → 32 % | |
| 19:07:21 | 0x22 room 7, 70 s after | 32 → 32 % | 48, then 50 within 21 s |
| 19:10:13 | 0x22 room 7, 4 min after | 32 → 41 % | back within 20 s |
| 19:11:14 | `z:1e,D,<thermostat>` | 41 → 43 % | one held for at least 55 s; no flag |

Earlier: `z:1e,D,…` at 18:57:31 cost 3 %;
the same and a 0x22 at 18:58:32, 61 s later, cost under 1 %.

The radiogram at 70 s was cheap, the one at 4 minutes cost 9 %:
the thermostat was awake after 30 s and asleep after 4 minutes,
so `1e` is neither 30 seconds nor 30 minutes.
A unit of a few seconds (30 × 4 s = 2 min) would fit,
or a stay-awake of the device's own after a radiogram;
radiograms at 2 and 3 minutes after a fresh wakeup would tell.
A device wakeup is accepted (`A:`), costs 2 to 3 %,
holds a slot for at least a minute, and did not flag the thermostat.

## Not settled

- Whether a room command reaches a member that its address does not name
  (a second device in the room would tell).
- The `z:` unit, between 70 s and 4 minutes for `1e`.
- Why one room setpoint and one late radiogram cost 17 % and 9 %.
