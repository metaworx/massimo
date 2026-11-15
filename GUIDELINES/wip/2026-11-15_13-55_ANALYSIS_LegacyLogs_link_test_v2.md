# T134: one room-flagged link radiogram and the room's members (v2)

2026-11-15, 12:05–12:35 UTC, for AP LegacyLogs v1.0, Block 4 (D150), on the owner's `EXEC t134`.
- **Where:** .25's room 1, with one wall thermostat and three radiator thermostats; Cube firmware 1.1.3.
- **How it was sent:** through serve's proxy (.35).
- **Who did what:** the owner turned the wheels at the devices, and serve's store on breno was read, read-only.
- **Labels:** rad1 is the anchor, the radiator thermostat the radiograms were addressed to; rad2 and rad3 are the others, in metadata order.

No names, serials or addresses below.

**This version replaces the reading of the first version of 13:36**, whose TL;DR said one room-flagged radiogram "did not make every radiator thermostat drop the wall thermostat". That was read too early: two of its deliveries were still pending when the owner turned the wheels. The timeline and the measurements are unchanged.

## TL;DR

- **Open, not "no".** The Cube took the one room-flagged 0x21 as one delivery per member of the group, about one memory slot each.
  - rad1, the device it was addressed to, took it at once: the wall thermostat lost a partner 3 minutes later.
  - Two deliveries stayed pending until the next wakeup (step 3's, 10 minutes later). The owner's wheel turns came while rad2 and rad3 probably still had the wall thermostat as a partner, and passed every change on to it.
- **The owner's decision (D154-A):** the question is not worth more radio time. It only concerns the link radiograms of a move or a removal, never a setpoint, and Massimo's one link radiogram per radiator thermostat works. Block 5 is dropped; the link code is left to AP DeviceUnPairingFix of the session "MAX! transmit error".
- **What the Cube sent:** only what was asked, and its own repeats.
  - Asked: 2 wakeups, a 0x23 and a 0x22 to the wall thermostat, and one room-flagged 0x21 and one 0x20 to rad1.
  - Its own repeats: duty rose from 13 % to 19 % with no command in between.
  - No other command reached the Cube from 12:00 to 12:45. The wheel turns were device to device.
- **The room is whole:** the link error is gone, all four devices are at 21.0 °C, and all 50 slots are free.

## The timeline (UTC)

| Time | What | Seen |
|---|---|---|
| 12:06–12:09 | **Baseline.** The owner turned rad2 to 21.5 °C, then the wall thermostat to 21.0 °C | every device followed both turns within 20 s; duty 7 %, 50 slots free |
| 12:09:53 | **Out:** `z:` room 1, then 0x23 to the wall thermostat, then one 0x21 to rad1 with the room flag, naming the wall thermostat | all accepted; free slots 50 → 47 → 45; duty 10 % |
| 12:11 | the owner turned rad2 to 21.5 °C | rad1 and rad2 at :23, the wall thermostat at :33, rad3 only at 12:12:41 |
| 12:12:01 | the Cube's greeting | 48 free: 2 slots still held |
| 12:12:41 | — | **the wall thermostat's link error set**: the slowly blinking antenna, "connection to taught-in MAX! components lost" |
| 12:15 | the owner turned rad1 to 21.0 °C | rad1 and rad2 at :21, the wall thermostat at :41, rad3 at 12:16:41 |
| 12:18 | the owner turned rad3 to 20.5 °C | rad3 and rad1 at :31, the wall thermostat at :41, rad2 at 12:19:41 |
| 12:20:45 | **In:** `z:` room 1, then one 0x20 to rad1 with the room flag, naming the wall thermostat, then 0x22 to the wall thermostat | all accepted; free slots 48 → 44 → 43; duty 13 % |
| 12:24:41 | the Cube's greeting | 49 free, duty 19 %, with no command since |
| 12:26:11 | the Cube's greeting | 50 free |
| 12:30:41 | — | **the link error gone**, before the owner touched anything |
| about 12:31–12:33 | the owner turned rad1 to 21.0 °C, twice | rad1 went back to 20.5 °C, once within half a second and once after about 20 s; no other device followed; the Cube had nothing pending |
| 12:34:11 | the owner turned rad1 to 21.0 °C a third time | all followed within 10 s |

## What it shows

- **The Cube delivers a room-flagged link radiogram to every member of the group.**
  - One radiogram took about one slot per member: 47 → 45 here with one freed at the same moment, 48 → 44 with the wakeup in the in step.
  - The original software's removal of 2026-11-15, 12:30 local, found by the session "MAX! transmit error", took 4 slots in a room of 4.
- **The addressed member took it at once, the others only at the next wakeup.**
  - The wall thermostat's link error came 3 minutes after the out step: rad1 had dropped it.
  - Two slots stayed held from the out step until after the in step's wakeup, and the Cube spent 6 % of the duty cycle on its own retries.
  - rad2 and rad3 therefore likely took the late 0x21 and then the 0x20, in that order; the link error cleared at 12:30:41.
- **So the wheel turns of step 2 do not answer the question.**
  - With rad2 and rad3 still linked, every change reached the wall thermostat directly or through them.
  - A decisive run would wait until the Cube's slots are free again, with an extra wakeup where they hang, before turning a wheel. If every member has dropped the wall thermostat, no device can relay to it any more.
- **Not run, by the owner's decision (D154-A).**

## For docs/cube-protocol.md

With Block 4's docs commit, in "Pairing a device, as the original software does it", after the moves:

```markdown
The Cube takes a link radiogram with the room flag (0x20, 0x21) as one delivery
per member of the room's group, about one memory slot each (2026-11-15, Cube
firmware 1.1.3, one of the owner's rooms with three radiator thermostats; the
original software's removal the same day took 4 slots in a room of 4).
The member the radiogram is addressed to took it at once; the others were
still pending ten minutes later and took it with the next wakeup of the room.
Whether every member then acts on it was not settled: the devices pass setpoint
changes on among themselves, so a member that kept a partner hides one that
dropped it.
Massimo sends one room-flagged link radiogram to each radiator thermostat.
```

## A lesson for the next test through the proxy

The proxy of .35 shows each room's wall thermostat as serve's virtual one, named after the room.
- **The radiograms still went to the real device:** the proxy translates a radiogram to the virtual wall thermostat, or naming it as a partner, to the real one. Serve's log shows step 1's 0x23 went to the real wall thermostat.
- **The script now skips virtual devices** (`VWT` serials). Read a test's devices with that in mind.

## How it was run

All in `GUIDELINES/temp/legacy/`:
- `t134_links.py` against the proxy: the steps `show`, `out` and `in`. Its lines were checked against Massimo's builders byte for byte.
- `t134_watch.py`, `breno_t134_events.py`, `breno_t134_targets.py` and `breno_t134_all.py` on breno's store, read-only.

Nothing else was sent to any Cube.
