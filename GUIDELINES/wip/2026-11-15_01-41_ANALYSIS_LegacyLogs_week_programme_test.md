# T130: the week programme's second telegram, tested

2026-11-15, 00:39–00:41 UTC; follows `2026-11-14_21-13_ANALYSIS_LegacyLogs_protocol_findings.md`
(D130-A, D144-A, D145-A).
One radiator thermostat of one of the owner's Cubes (.24, room 4, alone in its room,
in manual mode), Cube firmware 1.1.3, through serve's proxy (.34), from the owner's PC.
No serials or addresses below; `<rf>` stands for the device's address.

## TL;DR

- **The original software's form works.**
  - A day of 8 switch points was sent in two 0x10 telegrams, the second marked with bit 4 (0x10) of the day byte and padded to 6 points.
  - The device's C line held all 8 points, exactly as sent, within 17 s.
- **Restored at once**: the device's Tuesday is its 5 points again, confirmed by the C line.
  The device is valid with no error flag. Two minutes after the restore the Cube stood at 7 % with 49 of 50 slots free: one slot was still held.
- **Massimo's form (0x08, unpadded) was not tried.**
  No thermostat in breno's latest backups has a day of more than 7 points (90 days of 3 points, 169 of 5).
  So Massimo has never sent its form to these devices, and nothing needs repair.
- **D139 can now be decided.** Recommended: the original's form.

## What was sent

The device's Tuesday was `17.0@06:00, 21.0@09:00, 17.0@17:00, 21.0@23:00, 17.0@24:00`.
The test day was the same day as 8 points, its longest segments split, so the heating would not change:
`17.0@03:00, 17.0@06:00, 21.0@09:00, 17.0@13:00, 17.0@17:00, 21.0@20:00, 21.0@23:00, 17.0@24:00`.

| UTC | Line | Cube's answer |
|---|---|---|
| 00:39:24 | `z:1e,G,04` | `A:` |
| 00:39:24 | `s:` `00 00 10 000000 <rf> 00` `03` `4424 4448 546c 449c 44cc 54f0 5514` | `S:` 6 %, not discarded, 48 free |
| 00:39:25 | `s:` `00 00 10 000000 <rf> 00` `13` `4520 4520 4520 4520 4520 4520` | `S:` 6 %, not discarded, 47 free |
| 00:39:41 | serve's re-read: the C line changed | Tuesday as sent, 8 points |
| 00:40:14 | `z:1e,G,04`, then `s:` `… 00` `03` `4448 546c 44cc 5514 4520 4520 4520` (the restore) | `A:`, then `S:` 6 %, not discarded, 48 free |
| 00:40:29 | serve's re-read | Tuesday's 5 points again |
| 00:42:15 | serve's reconnect for the budget | 7 %, 49 free |

Before the test: duty 4 %, 50 free, L flags `12 19` (valid, initialized; manual).
After: duty 7 %, 49 free (00:42:15), flags `12 19`.

## What it shows

- **The second telegram's mark is bit 4 (0x10) of the day byte**, as in `SetRoomWeekTemperatureProfile.createDayPartSpec` of the original software 1.4.6.
  The Cube put the 8th point into slot 7, with the padding after it.
- **The padding is harmless.** The device stores the 6 points; the C line reads up to the first 24:00 point.
- **The Cube updates its C line within 17 s** of a device-addressed 0x10 to an awake device.
  serve's first re-read saw it; the docs say "1 to 4 min" for other writes.
- **Cost:**
  - The wakeup and two telegrams moved the duty from 4 % to 6 %; with the restore's wakeup and telegram it stood at 7 % two minutes later.
  - Three slots were held at most, the wakeup apparently holding one too.
    One was still held at 00:42:15, though both writes had shown in the C line; which radiogram holds it the Cube does not say.
  - That is less than the "+8 % for one day of 0x10" of 2026-11-04; the Cube reports whole percent, and the rolling share (N6) may take its time to show.
- **Not shown:** whether the Cube or the device would also take 0x08.
  It need not be known: Massimo can send the form that works.

## For the decisions

- **D139. The week-programme builder**
  - **A (recommended).** The original's form:
    - `CmdSetDayProgram` marks the second telegram with 0x10 and pads it to 6 points with the last one (`internal/protocol/cube/commands.go:281`);
    - its test changes with it (`cube_test.go:445`);
    - the fake Cube reads 0x10 (`internal/cubelan/fakecube/apply.go:243`);
    - the docs say "verified on hardware 2026-11-15".
  - **B.** Accept both marks in the fake Cube, send the original's: the same for the Cubes, a broader fake.
- **D130** is settled by this test.
- **D131** (T131, the battery test) waits for you: tell me the UTC time the batteries came out.
- **D133**: the duty readings are to be read again around 2026-11-22.

## For docs/cube-protocol.md

In "LAN commands verified against the original software", replacing the *unverified* paragraph proposed in section 7 of the analysis of 2026-11-14:

```markdown
A day of the week programme with more than 7 switch points goes as two 0x10
telegrams: the first with points 1 to 7, the second with points 8 to 13,
marked with bit 4 (0x10) of the day byte and padded to 6 points with the last
one, as the original software 1.4.6 builds it
(`SetRoomWeekTemperatureProfile.createDayPartSpec`).
Verified on hardware 2026-11-15 (Cube firmware 1.1.3, one radiator thermostat
of one of the owner's Cubes, through serve's proxy, after a `z:` room wakeup):
the C line held all 8 points of a test day within 17 s, and the day was
written back afterwards.
Two wakeups and three telegrams (the test and the restore) cost 3 % of the
duty cycle; up to three memory slots were held, one still two minutes later.
```

## How it was run

`GUIDELINES/temp/legacy/t130_week.py` from the owner's PC against the proxy:
- first without `--send` (the greeting only);
- then `--form orig --expect <Tuesday> --send`;
- then `--restore --send`.

The script's guards held: the day matched the gate, the device was valid without the error flag, the budget was above the limits.
The store's rows were read afterwards with `breno_t130_after.py` and `breno_longdays.py` (read-only).
