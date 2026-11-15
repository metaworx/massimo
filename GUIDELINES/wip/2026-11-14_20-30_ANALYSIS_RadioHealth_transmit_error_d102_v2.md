# Transmit error (D102): what the logs of February answer

2026-11-14, evening; follows `2026-11-14_19-15_ANALYSIS_RadioHealth_transmit_error_d102.md`.
Still read-only: nothing has been sent to any Cube.
Cubes by their own IP suffix: .21, .22, .23, .24, .25 (proxies .31–.35).
Devices by today's room and type in breno's store ("wall r3": the wall
thermostat of room 3); no serials, radio addresses or names.

## TL;DR

The original software's logs from the other host (2026-02-12 to 03-05,
1.2 GB) record the Cube's device list every 10 s and every command with the
Cube's answer. They answer most of what the hardware tests were for:

- **`r:` costs nothing and clears at once:** 4 resets in February, each
  answered with the duty and free slots unchanged (on .22 with 1 slot free),
  the flag gone at the next device list, 2–8 s later,
  for devices the Cube had not heard since its restart too.
  With the 18:09 sample, 5 of 5. It is the Cube's bookkeeping, not a radiogram.
- **A Cube restart clears every flag**, and the Cube flags again, within about 6 min,
  each device it has not heard by then (0xF1 wakeup, then 0x03 time).
- **Nothing else clears it, and nothing re-flags a reset device:**
  .22 watched for 19 hours (and the same 3½ days later), .21 for 10½ days,
  without a single change; .22's room-1 wall thermostat still clear 3½ days
  after its reset;
  .25's room-6 wall thermostat is still clear since 18:09 today.
- **.22's slots were already full in February:** 1 of 50 free on 2026-02-19.
  The original software's room command took the last one, held it for at
  least 4 hours at 0 % duty, and its next room command was discarded.
  The slots hold radiograms waiting for devices that do not answer;
  the flags are a separate matter.

So T1–T3 (the direct tests with serve stopped) are no longer needed.
What remains open is .22's slots.

## 1. What the logs hold

| Days | Cube the software held | Device lists | Commands |
|---|---|---|---|
| 2026-02-12 | .25, briefly | 6 | none |
| 2026-02-19 11:51–12:05 | .25, .21, .22 in turn | — | 8 room setpoints, 4 resets |
| 2026-02-19 16:08 – 02-20 10:59, again 02-23 09:18 | .22 | about 6 700 | 1 room setpoint, discarded |
| 2026-02-23 09:27 – 03-05 20:36 | .21 | about 89 000 | none |

The software logs `getDeviceList` with the raw L line every 10 s while its
window is open, `commitDeviceRadiogram` for every radiogram,
including `ResetDeviceRadiogram`, and the Cube's answer
(`DutyCycle`, `FreeMemorySlot`, `CommandDiscarded`).
The timeline was extracted by `GUIDELINES/temp/d102/legacy_timeline.py`.

## 2. The resets of 2026-02-19

| Time | Cube | Command | Lines sent | Cube's answers | Flag |
|---|---|---|---|---|---|
| 11:52:06 | .25 | room 3 manual 26 °C | `r:` wall r3, `r:` rad r3, `s:` 0x40 room 3 | 15 % 50 free, 15 % 50 free, 15 % 49 free | both gone at 11:52:14; the wall thermostat not heard since the restart |
| 11:55:50 | .22 | room 1 manual 23 °C | `r:` rad r1, `r:` wall r1, `s:` 0x40 room 1 | 1 % 1 free, 1 % 1 free, 1 % **0 free** | both gone at 11:55:52; the wall thermostat not heard since the restart |
| 16:08:46 | .22 | room 1 manual 23 °C | `s:` 0x40 room 1 (no flag left to reset) | 0 %, 0 free, **discarded** | — |

As the bytecode said: one `r:01` per flagged device of the room, then the command.

## 3. The answers

**Q1. Does `r:` cost radio time? No.**
Five resets, five answers with duty and slots as before; the flag gone at
the next device list (2–8 s), also for a device the Cube had not heard
since its restart, which could not have answered a radiogram.
The 2026-11-05 report's reading ("the queue's error reset" as a fifth
pending radiogram) does not hold; that radiogram was something else.

**Q2. Does the Cube clear it by itself? Only by restarting.**
- Between 11:57 and 11:59 on 2026-02-19, .25 went from 13 flagged devices to
  none, every entry without data, duty 15 % → 0 %: a restart.
- Within about 6 minutes .25 flagged four devices again, each one it had not
  heard yet: 0xF1 (wakeup), 10 s later 0x03 (time).
- No other clearing: .22 unchanged for the 19 hours watched and at a look
  3½ days later, .21 for 10½ days.
- A reset device stays clear: .22's wall r1 still 3½ days later;
  .25's room-6 wall thermostat since 18:09 today.

**Q3. What sets it? A radiogram the device does not answer.**
- The Cube's own wakeup and time push after its restart, to devices not yet heard.
- A client's room command to a room with such a device:
  .25 room 3 at 12:01:52, its wall thermostat flagged 0xF1 at 12:02:16, probably by that command's wakeup.
- The original software's daylight-saving push (0x81, a house
  configuration): on 2026-02-12 .25 had six devices flagged with it.
- No periodic push re-flagged anything in the 11½ days watched.

Today's 15 of 16 with 0x03 fit this:
time pushes after a restart, to devices that were silent at that moment.

**Q4. .22: unreachable or flagged, and its slots?**
- Flagged, mostly: nine of its ten flagged devices report today.
- The slots are another matter. 1 free on 2026-02-19 already; the resets
  left it at 1; the room-1 setpoint took the last one and held it at 0 % duty
  for at least 4 hours, waiting for the room's wall thermostat,
  which the Cube had not heard since its restart;
  the next room-1 setpoint was discarded.
  The Cube keeps such a radiogram until the device answers, or until it restarts,
  and spends no radio time on it meanwhile.
- So .22's 49 slots are radiograms waiting for devices that do not answer:
  the window contacts (out of service since they were moved),
  or devices without batteries, then or now.
  Which ones cannot be read from the Cube.
- **Now:** .22 takes one more radiogram. If it is for a room with a silent
  member (room 3 has one: a radiator thermostat not heard since the restart),
  .22 then refuses every command until it restarts or that device answers.

**Q5. When does the original software send `r:`? Confirmed in its logs:**
before each command, one `r:01` per flagged device in the command's scope,
silent devices included; never otherwise.

## 4. What remains

- **.22's slots:** a restart drops what they hold and clears its flags;
  the store then shows which devices .22 flags again in the first minutes
  (those it cannot reach at its start), whether the slots return to 50,
  and whether they fill again over the next days.
  Massimo's reboot (`POST /api/v1/cubes/{sn}/reboot`, owner-only,
  in the Cube's panel) marks the Cube as rebooting, so serve waits for it.
- **The window contacts:** a battery in one of .22's contacts before the
  restart would show whether it holds slots; the February log already shows
  the mechanism with a wall thermostat, so this adds little.
- **Passively:** whether .25's reset device stays clear (so far 2 h).

## 5. Held until the tests are done

D102, D110, D111, D113 and D116 (the analysis of 19:15, section 5), to be
brought back with these results. D112-A is settled.
New input for them: `r:` is free and a reset holds (D102, D110);
a restart re-flags whatever is silent at that moment (D102, D116);
a full queue blocks a Cube without any flag (D116: a slot watch).

## 6. For docs/cube-protocol.md

In addition to section 6 of the 19:15 analysis:
- `r:` takes no slot and no duty, clears at once (5 samples, 2026-02-19 and 11-14);
- a restart clears every flag; the Cube flags, within minutes, every device
  it has not heard by its first wakeup and time push;
- a radiogram for a device that does not answer holds its slot without
  spending radio time, until the device answers or the Cube restarts;
  a Cube with no slot left discards every command (2026-02-19, Cube firmware 1.1.3, logged as 275 = 0x0113);
- the original software's logs carry the raw L line every 10 s,
  a source for later findings.
