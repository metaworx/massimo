# T131: how long a silent wall thermostat keeps its valid bit

2026-11-15, 01:45–02:25 UTC; follows `2026-11-14_21-13_ANALYSIS_LegacyLogs_protocol_findings.md`
(D131-A) and `2026-11-15_01-41_ANALYSIS_LegacyLogs_week_programme_test.md`.
The owner took the batteries out of two wall thermostats of one Cube (.22, rooms 3 and 4), Cube firmware 1.1.3.
The times were read from serve's store on breno (read-only), which serve fills from its `l:` poll once a minute.
**Nothing was sent to any Cube** by the test.
No names, serials or addresses below.

## TL;DR

- **The valid bit means "heard lately".**
  - Room 3's wall thermostat lost it 4¾ to 5¾ minutes after its batteries came out, at about two missed 3-minute reports.
  - It had it again within a minute of the batteries going back.
  - Room 4's did the same.
- **It explains the February drops and the November flickers:**
  - .22's wall thermostats in February lost it within hours, when their batteries came out or they went out of range.
  - The radiator thermostats, which report once a day when idle, kept it for more than 23 hours.
  - The one-minute drops of 11-12 and 11-13 were one or two missed reports.
- **New: a wall thermostat whose batteries go back in starts in auto, on its week programme.**
  - Room 3's had been on manual 6 °C and came back on auto at 5.5 °C.
  - The room's radiator thermostat has shown no change since, so the two may now disagree.
- D131 is settled, and D140 (showing an out-of-date entry's values) gains its premise: such an entry's values are those of a device that has stopped answering for minutes, not days.

## The timeline (UTC)

| Time | Room 3 | Room 4 |
|---|---|---|
| until 01:28:15 | valid, manual 6 °C, reporting (rows only on a change) | valid, auto |
| 01:47:15–01:50:15 | — | a mode change to auto, then the programme's 14.5 °C (presumably the owner's change at the display) |
| **01:47:30** | **batteries out** (owner) | — |
| 01:52:15 | still valid (no row) | — |
| **01:53:15** | **valid bit clear**: every value gone | — |
| **02:04** | **batteries back** (owner) | — |
| **02:04:15** | **valid again**: auto, setpoint 5.5 °C, 11.6 °C measured | — |
| 02:07–02:22 | reporting every 3 minutes, 12.4 to 11.8 °C | — |
| 02:12:15 | — | **valid bit clear** (removal time not noted) |
| 02:16:15 | — | **valid again**: auto, 16 °C |
| 02:17:15–02:19:15 | — | setpoint 5.5, then a command through serve's queue (02:18:48), then manual 5.5 °C |

The other three wall thermostats of .22 stayed valid and kept reporting throughout.

## What it shows

- **The timeout.**
  - The Cube cleared room 3's valid bit between 01:52:15 and 01:53:15: 4¾ to 5¾ minutes after the batteries came out.
  - The device's last report came at most one cycle, about 3 minutes, before that, so about 5 to 9 minutes after it.
  - A wall thermostat reports about every 3 minutes (2.7 min at least, the February logs), so the Cube gives up after two missed reports, or a fixed 5 to 6 minutes.
  - Which of the two cannot be told from one device type.
- **Not the same for every type.**
  - .21's radiator thermostats, idle in manual mode, report once a day and stayed valid for 10½ days in February.
  - .22's silent ones of February kept the bit for at least 23 hours, and lost it within 3.9 days.
  - So the timeout follows each type's expected report, or the radiator thermostat's is about two days.
  - No contact or eco switch, which report only on events, was ever seen to lose it while silent (they are never valid after a restart until they transmit).
- **The return.** The valid bit came back at the first poll after the batteries went in, with fresh values.
  The device announces itself at once, as it does after a restart of the Cube.
- **The side effect.**
  - A wall thermostat powered up again runs its programme: room 3's went from manual 6 °C to auto 5.5 °C.
  - Its radiator thermostat (manual, reporting only now and then) showed no change by 02:25.
  - Room 4 was set to manual 5.5 °C at 02:18:48 through serve's queue.

## For the decisions

- **D131**: settled.
- **D140. OutOfDate entries**
  - **A (recommended).** Show a wall thermostat's frozen values as "last known, out of date" with their age: they are minutes old, not days.
  - **B.** Leave it: no state without the valid bit.
- **D147 (new). A wall thermostat that comes back on auto**
  - **A (recommended).** When a wall thermostat regains its valid state after more than 5 minutes without it, and its mode changed, Massimo logs it at Warn: "came back on auto, was manual 6 °C".
    A notification of the device-error kind follows if enabled.
    A battery change otherwise silently returns the room to its programme.
  - **B.** Nothing: the mode change is already in the log as a state change.

## For docs/cube-protocol.md

Replacing the valid-bit entry proposed in section 7 of the analysis of 2026-11-14:

```markdown
- The valid bit (0x10 of the first flags byte) says the Cube heard the device
  lately (2026-11-15, Cube firmware 1.1.3, two wall thermostats of one of the
  owner's Cubes, their batteries taken out and put back by the owner):
  a wall thermostat lost it 4¾ to 5¾ minutes after its batteries came out,
  about two of its 3-minute reports, and had it again within a minute of the
  batteries going back.
  The entry is then "out of date", as the original software calls it: flag
  0x02 alone, the values of the last report kept.
  Radiator thermostats, which report once a day when idle, kept it for more
  than 23 hours of silence (2026-02-19/20); wall thermostats that missed a
  report or two lost it for a minute (2026-11-12 and 13).
- A wall thermostat whose batteries go back in starts in auto mode on its week
  programme: one that had been on manual 6 °C came back on auto at 5.5 °C
  (2026-11-15), its room's radiator thermostat still on manual.
```

## How it was read

`GUIDELINES/temp/legacy/t131_watch.py`, one look at breno's store per run, repeated every 10 minutes by `t131_loop.sh` from the owner's PC (stopped after both returns), plus two read-only queries for the rows of rooms 3 and 4.
The rooms were found from the owner's names in the Cube's latest backup; the names were used only inside the query.
