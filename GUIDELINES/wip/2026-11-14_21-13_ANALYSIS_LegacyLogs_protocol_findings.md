# The original software's logs (MAX! 1.4.6): what they teach about the Cube

2026-11-14, evening. An analysis for the owner; read-only.
**Nothing has been sent to any Cube**; breno was read only through its store
(`mode=ro`), and the jars only with `javap`.
No serials, radio addresses or names below.
Cubes are named by their own IP suffix (.21 to .25).
Devices are named by type and room id: `wall r3` is the wall thermostat of room 3.
February's devices carry the room ids of February's metadata; breno's carry today's.

The transmit-error topic (D102) belongs to the session "Investigate the MAX! Cube's transmit error on hardware".
Where this analysis touches it, section 6 says so. That session has not been told.

## TL;DR

- **The setpoint shows in L when the device acknowledges, not when the Cube accepts.**
  - The four February room setpoints that changed a value and reached their devices showed at the next poll, 1–19 s after the `S:`.
  - On .22 the radiogram sat in the last free slot and the setpoint never appeared, in 19 hours of polling.
  - D102's two lost room commands of 11-13 and 11-14 never showed either.
  - The docs and `cubelan/times.go` say "at once".
- **Radiator thermostats send their measured temperature once a day in manual mode.**
  - The report comes at 12:00 local, their decalcification hour, and not on Saturday, their decalcification day.
  - L shows the value for 10 s to 13 min (median 2.3 min), then 0 again.
  - The longest gap between changes is 48 h, Friday noon to Sunday noon.
  - So Massimo's 24 h stale threshold marks every idle radiator thermostat stale on Saturdays.
- **The valid bit drops without a restart.**
  - The entry is then "out of date": its values stay, frozen.
  - On .22 six silent devices dropped it in February.
  - On breno two wall thermostats dropped it for one poll each on 11-12 and 11-13, then came back unchanged.
- **The duty cycle falls within the hour**, so it is likely a rolling 60-minute share.
  - .25 fell from 21 % to 3 % between 14:05 and 14:42 UTC on 11-13, with no client command.
  - Massimo's queue waits for the next full hour.
- **Costs measured.**
  - `r:` is free, as D102 found.
  - A room 0x40 costs 3 % each time, 4 samples, rooms of 2 and 3 thermostats.
  - Its slot comes back within 3–6 s when the devices answer.
  - It is held for at least 3.9 days at 0 % duty when they do not: .22 at 0 of 50 free from 02-19 11:55 to 02-23 09:18.
- **Gaps in Massimo found:**
  - `pending_radiograms` says 0 when all 50 slots are in use.
  - The week programme's second telegram is marked 0x08; the original's bytecode uses 0x10 and pads to 6 points.
  - A move-out sends 0x23 after the 0x21s; the original and its capture send it first.
  - The battery-low notification fires on every flicker: 10 times in 3 days for one device on breno.
- **Facts the bytecode adds.** The original software:
  - reads three states from byte 5: Valid, OutOfDate, NotInitialized;
  - calls byte 6 bit 0x40 `transmitError`, which is Massimo's `link_error`;
  - never reads the radiator thermostat's valve or measured temperature;
  - never sends 0x81 DST;
  - can set the Cube's clock with `v:…,<hex seconds>`;
  - sends `q:` when the UI stops calling it, not after "30 s without user activity".

Open: four tests (D130–D133), all optional, and ten implementation decisions (D134–D143) that wait for them.

## 1. What was read

| Source | Span | What it gave |
|---|---|---|
| Other host's logs, `eq3.<date>.log[.zip]` | 02-12, 02-19 to 03-05, 05-05 | 97 000 polls: raw L every 10 s, the model, 11 greetings with H and C lines, 15 radiograms with their `S:` |
| This PC's logs | 11-12 to 11-14 | the software through Massimo's proxies (.31, .35): little new, the one-line-late errors already in the docs |
| `MaxEssentialsBackend-1.4.6.jar`, `MaxLocalBackend-1.4.6.jar` | — | the L/C/H/S parsers, the builders, the connection, the model |
| breno's store | 11-07 to 11-14 | 6 458 device-state rows, 69 commands, 7 backups with raw H and L |

The sessions the software held:

| Cube | When | Polls | L changes | Commands |
|---|---|---|---|---|
| .25 | 02-12 08:32, 02-19 11:51–12:05 (a restart 11:58), 11:59–16:08 | 1 432 | 150 | 4 room setpoints, 2 resets |
| .21 | 02-19 11:52–11:54; **02-23 09:27 – 03-05 20:36** | 88 545 | 3 530 | 5 room setpoints in 3 calls |
| .22 | 02-19 11:55–11:59; **02-19 16:08 – 02-20 10:59**; 02-23 09:18 | 6 639 | 4 | 2 room setpoints, 2 resets |

Scripts: `GUIDELINES/temp/legacy/` (`extract.py` builds a pickle in the session's scratchpad; `flags.py`, `fields.py`, `devtrace.py`, `cmdtl.py`, `radtemp.py`, `modelmap.py`, `modeldiff.py`, `bitstats.py`; `breno_*.py` piped to breno as `sudo -n python3 -I -`).
The two bytecode reports are in the scratchpad (`out/jar_lan.md`, `out/jar_model.md`), each claim with class and method; the contradictions below were checked again by hand.

## 2. Confirms the docs

- **Polling every 10 s** (cp:304).
  99.8 % of the 88 544 gaps on .21 were 10 s, and each `getMaxCubeState` sent exactly one `l:` (96 617 of each).
  The bytecode agrees: only the UI's call sends `l:`, and the backend has no timer of its own.
- **The 0x40 room payload** (cp:288-290): flags 0x04, the room id, `0x40|temp×2` for manual, as `commands.go` builds it.
  All 11 February room radiograms had this form.
- **A Cube with no free slot discards** (D102's v2): .22 at 0 free, 02-19 16:08:46, `S:00,1,00`, "Command discarded".
  The software neither retried nor sent the rest of the batch.
- **A restart clears every entry**, and the Cube flags, within 5 minutes, what it has not heard: .25 at 11:58 on 02-19 (D102).
  Two entries of room 4 were valid again in the first list after it, most other thermostats within 2.5 minutes; four were flagged instead.
- **The link bit (`link_error`) is the device's own and clears with its next transmission** (cp:534-536).
  After .25's restart, rad r3 and rad r6 came back with it set.
  rad r3 lost it 5 minutes later, at its next report; rad r6 kept it for the remaining 4 hours, since it reported nothing more.
- **The factory fallback** (cp:617-619): on 02-12 a Cube with DHCP on answered discovery from 192.168.0.222/16 and refused the TCP connect.
  It had no lease.
- **One wall thermostat per room**: the bytecode settles cp:405-414. `ConfigureDeviceValidator` allows 8 radiator thermostats, 1 wall thermostat and 8 shutter contacts per room, and room ids 1–19.
- **The wall thermostat's C line** (cp:422-424): the offset at 0xcc, as `config.go` has it.
  The rest of the original's wall-thermostat mapping is the radiator thermostat's copied, and wrong, but the software never reads it.
- **DST through `v:` only**: 1.4.6 builds 0x81 (`SetDeviceDaylightSavingMode`) but nothing references the class.
  It never sends it, as Massimo does not.

## 3. Corrects the docs

### K1. The setpoint shows when the device acknowledges

- **Docs:** cp:72-73, cp:526-531 and cp:756-757 say the setpoint shows in L at once.
  docs/api-v1.md:1442-1444 says the same, and :1777-1778 adds that the Cube changes it "before the devices confirm it".
  The comment at `internal/cubelan/times.go:22-25` says so too.
- **Evidence**, the February room setpoints, time from the `S:` to the L change:

  | Cube, room | Members | Shown |
  |---|---|---|
  | .25 r3, 11:52:06 | rad (valid), wall (no state) | rad +8 s; the wall had no state to show |
  | .21 r1, 11:53:30–39 (3×) | wall, 2 rad | all three +10 s after the last |
  | .25 r4, 11:59:57 | rad, wall | rad +1 s, wall +12 s |
  | .25 r3, 12:01:52 | rad | +3 s |
  | .22 r1, 11:55:50 | rad, wall (both silent) | **never** (19 h watched, slot held) |
  | .21 r2, 11:53:36 + :39 | wall, rad | 20.5 then 21.0: back to what it was, no change visible |

  On breno, D102's two room commands to .25's room 6 (11-13 17:25, 11-14 16:22) were never shown by either member.
- **Reading:** the Cube writes the new setpoint into a member's entry when that member acknowledges.
  A member that does not answer keeps its old setpoint, and a radiogram waiting in a slot shows nothing.
- **The original software** marks the room `stateDirty` after a 0x40 and clears it only with the next L that changes something in the room: no timer, no expected value.
  .21 r2 and .22 r1 stayed dirty until their sessions ended.
- **Recommendation:** the docs text in section 7. Then D135: Massimo can tell a client that a room command is not yet confirmed, as the original did.

### K2. When radiator thermostats report their measured temperature

- **Docs:** docs/api-v1.md:1782-1783 says "only now and then (after the valve moved)".
  The comments at `internal/protocol/cube/devicelist.go:100-101` and `internal/view/view.go:183-189` say the same.
- **Evidence:**
  - **February.** 42 appearances (bytes 9–10 non-zero), 41 of them in the hour from 12:00 local.
    30 were on .21 at 12:00:27–12:00:58; the other 11 on .25, right after its restart and while rad r3's setpoint was being changed at the device.
    On .21, all three radiator thermostats reported every day at 12:00:27–12:00:58 from 02-23 to 03-05, in manual 5 °C with the valve at 0.
    Saturday 02-28 is the exception, with no report.
    All rooms have the defaults "Saturday, 12:00" for decalcification.
  - The value stayed in L for 10 s to 759 s, median 138 s; then the bytes went back to 0.
  - **breno, November.** 47 of the 65 manual-mode radiator temperature changes fell in 11:00:5x UTC, which is 12:00 CET.
    There were 23 on Thursday 11-12 and 21 on Friday 11-13; on Saturday 11-14 only 3, at 11:04 and 11:07.
    In auto mode they come at every hour, with the switch points.
  - The longest gap between any change of an idle radiator thermostat's entry on .21 was 47.9–48.0 h, Friday 12:00 to Sunday 12:00.
    For wall thermostats it was 2.2 h and 7.6 h.
- **Reading:** an idle radiator thermostat reports once a day at its decalcification hour, except on its decalcification day.
  Whether it is that hour or noon itself is open, since every device here has 12:00 (D132).
- **Recommendation:** the docs text in section 7. D136: the stale threshold.

### K3. The idle disconnect

- **Docs:** cp:304-305 says the software disconnects "after 30 s without user activity".
- **Evidence:** .21 was held from 02-23 09:27 to 03-05 20:36, polled every 10 s, with no command in 10½ days.
- **Bytecode:** `ConnectionWatchDog`, every 5 s, sends `q:` when no API call came within `max.idleTimeBeforeLocalAppDisconnect`.
  Every poll is such a call. The value is a property outside the jars.
  So the timeout, about 30 s as observed, counts from the UI's last call: a closed window, a sleeping PC.
  When the connection itself idles, `CubeGateway.idleCall` sends the literal line `e:something` as a keepalive.
  It was logged once, on 05-05, after a two-month gap in the log (the PC asleep, presumably); the connection was dead and failed.
- **Recommendation:** the docs text in section 7.

### K4. Naming: "transmit error"

- **Docs:** cp:786-796 and the D102 analyses call byte 5's 0x08 the "transmit error".
- **Bytecode:** the original software's `transmitError` is byte 6's 0x40, Massimo's `link_error`.
  byte 5's 0x08 is its `radioState` (Error, ConfigFailed, AssignFailed, DeleteFailed, by byte 4).
  The logged model confirms it on 37 869 valid device states: `transmitError=true` exactly where byte 6 has 0x40.
- **Recommendation:** one sentence in the docs, so that a reader of the original's UI or logs does not confuse the two.
  Overlap with D102, section 6.

### K5. Byte 6, bit 0x08 is the DST setting, not "summer time now"

- Set in 99 % of the entries in February and in November, both in winter time.
  It is clear only on a few wall thermostats (0x10, 0x11, 0x51 in November).
- **Bytecode:** `setDaylightSaving`, read and never used.
- **Massimo:** `DST` (`devicelist.go:21, 79`), not shown in the API.
- **Recommendation:** rename the comment to "automatic DST switching on"; no API change (D142).

### K6. The H line, which the docs do not describe

| Field | Meaning (bytecode) | Seen in February (11 greetings) |
|---|---|---|
| 0 | serial | — |
| 1 | radio address, hex | — |
| 2 | firmware, hex (0113 = 275) | 0113 |
| 3 | ignored | always 00000000 |
| 4 | ignored (Massimo: connection id) | new at every connect |
| 5 | duty cycle, hex | 00–18 |
| 6 | free memory slots, hex | 32, 01, 00 |
| 7, 8 | date YYMMDD and time HHMM, each 2 hex digits | **the PC's local time to the minute at all 11 connects** |
| 9 | `stateCubeTime`, decimal | always 3 |
| 10 | `ntpCounter`, hex | always 0000 |

- The software shows the date and compares nothing with it.
  The model's `cubeDate` stays at the greeting's value for the whole session: 19.02 16:08 on 02-20 00:00.
  So the logs say nothing about drift over the 10 days.
- **Recommendation:** the table goes into the docs. Massimo could name fields 9 and 10 in `cube.Hello` (D142).

### K7. The original software can set the Cube's clock

- **Docs:** cp:286 says `v:` has "no time field".
  `commands.go:121-123` says "the Cube takes the time itself from NTP", and `:136-137` says `CmdSetTime` is "not observed from the original software".
- **Bytecode:** `MaxLocalApi.setCubeTime` ends in `v:<b64>,<Integer.toHexString(seconds)>`: lowercase hex, no padding, sent only when the value is > 0.
  Whether the UI ever calls it, and from which epoch it counts, lives in its JavaScript, which the jars do not hold.
  Massimo's 2000 epoch and `%08x` stay unconfirmed.
- **Recommendation:** correct the comments; no test (the Cube's clock was right at every connect).

### K8. Smaller ones

- **cp:287** writes `r:<count>,…`; the software always sends `r:01,` with one device per line (D102's v2 says so too).
- **cp:296**: the `z:` duration "30 s".
  The original's field is `durationInMinutes` (= 0x1e); the byte is the same either way, so the unit is open.
  The software wakes a room only before configuration writes and 0x82, never before 0x40, and only a room with a radiator thermostat.
- **cp:441-443**: the FrameType list (0x45, 0xF0 Reset, 0xF1 WakeUp, 0x41, 0xFF). D102 already has it.

## 4. New protocol facts

### N1. The three states of an entry, and the valid bit dropping without a restart

The original software reads byte 5 as three states (`createDeviceStateInfo`):

| Bits | State | What the entry holds |
|---|---|---|
| 0x10 and 0x02 | Valid | the device's current values |
| 0x02 alone | **OutOfDate** | the values of the device's last report, frozen |
| neither | NotInitialized | nothing (all zero), as after a restart |

It applies values only from a Valid entry; otherwise it keeps what it had.
Massimo shows no state at all for both OutOfDate and NotInitialized (`devicelist.go:86-88`).

The valid bit dropped without a restart:

- **.22, February:**
  - The values in its L did not change once in the 19 hours from 02-19 16:08 to 02-20 10:59 (6 639 polls): its devices were silent.
  - Six entries went from Valid to OutOfDate, every time while no client was connected:
    - wall r2, wall r3 and wall r5 between 11:59 and 16:08 on 02-19;
    - rad r1, rad r2 and rad r5 between 02-20 10:59 and 02-23 09:18.
  - The three radiator thermostats stayed Valid for at least 23 hours of that silence.
  - None had the error flag set by it.
- **breno, November:**
  - .25 wall r1 dropped it at 02:47:53 UTC on 11-12, back at the next poll with the same values.
  - .22 wall r3 did the same at 08:01:54 UTC on 11-13, its error flag set before and after.
  - Of the three drops of `reported` to false in the store, these two came without a restart.
- **Reading:** the Cube marks an entry out of date by itself: after a silence of some length, and briefly at certain moments.
  The two short drops are unexplained.
  The .22 drops fit a timeout of more than a day for radiator thermostats; the data cannot measure it (D131).
- **Recommendation:** the docs text. D140: whether Massimo shows an OutOfDate entry's values as "last known".

### N2. Byte 5, bit 0x04 is not "not an answer"

- In every entry of 1.2 GB of logs and 7 backups, 0x04 occurs only together with 0x08.
  The values seen are 0x0c, 0x0e and 0x1e; 0x12 (Valid) never has it.
- It is not the count of pending radiograms.
  In breno's backups .22 had 49 slots in use and one entry with 0x04; .21 had none in use and one.
- The original software does not read it.
  Massimo names it `IsAnswer` (inverted) "the entry answers a command" (`devicelist.go:15, 73`); nothing supports that.
- **Recommendation:** call it "unknown, seen only with the error flag" (D142). It overlaps D102 (section 6).

### N3. How often devices report

| Type | Interval |
|---|---|
| Wall thermostat | about every 3 min: median 3.1 min between changes of the measured temperature, 2.7 min at least (3 519 changes, two devices, 10½ days); the longest gap without change 7.6 h |
| Radiator thermostat, manual, valve still | once a day at 12:00, not on Saturday (K2); 48 h gaps |
| Radiator thermostat, auto | at the switch points (breno: every hour of the day) |
| Shutter contact | only on a window change: none of the 17 contacts on .25 and .22 transmitted in the 4 h and 19 h watched |

The original software reads no measured temperature from a radiator thermostat at all.
Its room temperature is the last wall thermostat's, or 0.0 without one.
Massimo reads both and keeps the last value with its age (`cubelan/times.go:117-145`), which is better.

### N4. Battery-low flickers on radiator thermostats

- On breno, one radiator thermostat of .25's room 4 turned battery-low on and off 10 times between 11-11 21:35 and 11-14, each time for 2–4 minutes.
  One of .21's room 1 did it 4 times, for 1–4 minutes.
- The flag shows under the load of the valve motor first.
- In February, .21's wall r1 had it on 02-19 and not on 02-23 (a battery change, unwatched).
- **Recommendation:** D137. The docs gain one sentence.

### N5. What radio commands cost

| Line | Duty | Slot | Samples |
|---|---|---|---|
| `r:01,…` | 0 | 0 | 4 in February, 1 in November (D102) |
| 0x40 to a room of 2 or 3 thermostats | +3 % each | 1, back within 3–6 s once the members answer | 4 consecutive pairs on .21 and .25 |
| 0x40 to a room whose members do not answer | 0 % while it waits | **held at least 3.9 days**: 0 free at the greetings of 02-19 11:58, 16:08 and 02-23 09:18 | .22 |

- The cost does not grow with the room's members: one group radiogram, one wake burst.
- 3 % is 1.1 s of the 36 s an hour allows.

### N6. The duty cycle is likely a rolling share, not reset on the hour

- **.22, 02-19:** 1 % at 11:55:50, 0 % at 11:58:08, no restart (the entries kept their values).
- **breno, .25 on 11-13**, with no client radio command before 17:25 that day:
  - 21 % at 08:02:28 UTC, 2 minutes after the hour;
  - 6 % at 08:52:57;
  - 21 % at 13:32 and still 21 % at 14:05:43, 5 minutes after the hour;
  - 3 % at 14:42:10.
- Of 19 falls between consecutive readings in the store, 3 had no full hour between them.
- **Massimo:** `internal/cubelan/budget.go:15-16` ("resets on the hour") and the queue's hold until "the next full hour plus a margin" (`internal/engine/queue.go:157-158`, `:520-523`) assume the hour.
- **Recommendation:** the docs say "probably a rolling 60 minutes". D143: re-read the budget rather than wait for the hour. Passive check: D133.

### N7. What the original software's model makes of L (for the comparison)

- **The room's setpoint and mode** come from its "master" radiator thermostat, the room address in the M line.
  - The jar sets that address to the first radiator thermostat added to the room (`metadata.go:35` says "first device").
  - Its room command goes to the master, else to the room's first radiator thermostat.
  - Massimo sends to the M line's address with no fallback (`internal/server/server.go:1807`).
- **`temperatureMode`** is Eco or Comfort when the setpoint equals that preset exactly, else Normal.
  The UI shows it as a moon or a sun; Massimo has no counterpart.
- **`isTemperatureControllable`**: the room has a radiator thermostat. A room of a wall thermostat alone is not controllable there, and gets no `z:`.
- **`setPointTemperatureValid`**: false only while the room's thermostats are NotInitialized; seen after .25's restart for 1–2 minutes.
- **Mode 3 (boost)** reads as "NotAvailable", and none of its room commands can send it.

### N8. The week programme's second telegram (bytecode)

- A day of more than 7 switch points goes out as two 0x10 telegrams.
- The original software marks the second with **bit 4 (0x10) of the day byte**: `createDayPartSpec`, `addBoolean(10, 4, part == 1)`.
- It pads the second telegram to 6 points with the last one.
- Massimo marks it with 0x08 and does not pad: `internal/protocol/cube/commands.go:281`, its test `cube_test.go:435-446`, the fake Cube `fakecube/apply.go:243`.
- No capture holds either form.
- **Recommendation:** D130 tests the original's form; D139 then changes the builder.

## 5. Gaps in Massimo's code

| # | Where | Gap | Evidence | Decision |
|---|---|---|---|---|
| G1 | `internal/cubelan/budget.go:38` | `Pending()` returns 0 when free slots are 0, so `pending_radiograms` reads 0 on a Cube whose 50 slots are all held | .22: 0 free for ≥ 3.9 days in February, 49 of 50 held on 11-11 | D134 |
| G2 | `internal/protocol/cube/commands.go:281` | second week-programme telegram 0x08, not padded | N8 | D130, D139 |
| G3 | `internal/cubeio/move.go:56` | move-out sends 0x23 after the 0x21s | cp:379-381 (capture), `DisassociateDeviceFromRoom` | D138 |
| G4 | `internal/cubeio/move.go:30-34` | one room-flagged 0x20/0x21 per radiator thermostat of the room; the original sends one to an anchor, the room flag reaching the rest | `AssociateNewDeviceToRoom` | D138 |
| G5 | `internal/view/view.go:192, :231` | `stale_after` 24 h counts an idle radiator thermostat stale every Saturday | K2: 48 h gaps | D136 |
| G6 | `internal/notify/rules.go:122` | a battery-low notification at every off-to-on edge | N4: 10 in 3 days for one device | D137 |
| G7 | `internal/engine/queue.go:520-523` | the duty hold waits for the next full hour | N6 | D143 |
| G8 | `internal/engine/queue.go`, API result | a room command answered `S:` not discarded is "acknowledged" though no member has taken it | K1, D102's two lost commands | D135 (with D110) |
| G9 | `internal/protocol/cube/devicelist.go:86-88` | an OutOfDate entry's frozen values are dropped | N1 | D140 |
| G10 | comments and docs | "the setpoint shows at once" (`cubelan/times.go:22-25`), "after the valve moved" (`devicelist.go:100-101`, `view.go:183-189`), `IsAnswer` (`devicelist.go:15, 73`), `DST` (`:21`), `CmdSetTime` "not observed" (`commands.go:136-137`), metadata "Trailing" is the DST byte and the room address is the master radiator thermostat (`metadata.go:28, 35`) | K1, K2, K5, K7, N2, N7 | D142 |
| G11 | `internal/protocol/cube/metadata.go:192` | `m:` index in hex; the original writes it in decimal; they differ from the eleventh block (19 000 characters) on, which no Cube here reaches | bytecode | D142 |
| G12 | `internal/server/server.go:1807` | a room command to the M line's address, without the original's fallback to the first radiator thermostat | N7 | D142 |
| G13 | discovery | a Cube answering from 192.168.0.222 with DHCP on (no lease) is listed like any other | 02-12 | D142 |

Checked and found right in Massimo, against the original:
- `t:` expects one `A:` per device, as the Cube answered on hardware; the original reads one.
- The window-open duration is read as a full byte × 5; the original's 3-bit table would turn 60 min into 20.
- Massimo keeps the stored minimum and maximum in 0x11; the original writes 4.5 and 30.5.
- Massimo reads the radiator thermostat's valve and temperature, and keeps a wall thermostat's 0 as "not measured".

Open, not decidable from the data: the window bits.
The original counts byte 6 bits 0–1 values 1 or 2 as open; Massimo reads bit 0x02 (`devicelist.go:84`).
They differ only for 1 and 3, and no contact in the logs or the backups ever reported.

## 6. Where this touches D102 (the transmit-error session)

1. **Byte 4 keeps the failed type after `r:`.**
   In all 4 February resets it stayed 0x03, and only .25's restart put 0x09 back.
   .22's rad r1 and wall r1 kept it for 3.9 days.
   D102's first analysis says the entry reset at 18:09 shows 0x09; worth a second look before its docs PR states it.
2. **The six 0x81 failures on .25 (02-12) were not the software's.**
   1.4.6 never sends 0x81 (section 2), so they were the Cube's own radiograms, or another tool's.
   D102's v2 attributes them to "the original software's daylight-saving push".
3. **K1 is evidence for D110:** neither member of a room shows a room setpoint it did not take.
4. **K4 (naming) and N2 (bit 0x04)** belong in the same docs paragraph as the flag.

D102's docs PR and this one would edit the same paragraphs of `docs/cube-protocol.md`.
Hence D141.

## 7. Proposed text for docs/cube-protocol.md

**Summary, replacing cp:72-73:**

```markdown
- **Set temperature (0x40)**:
  a device in a room is accepted only when addressed as its room.
  The setpoint shows in a member's L entry when that member acknowledges,
  within seconds for a device that answers; the mode only with its next report.
```

**Replacing cp:526-531:**

```markdown
After a 0x40 set-temperature command the Cube writes the new setpoint into a
member's L entry when the member acknowledges it, within 1 to 19 s in four
room commands of 2026-02-19 (Cube firmware 1.1.3, the original software 1.4.6, polled every 10 s);
the mode bits change only with the device's next report, about 45 s later in
one observation.
A member that does not answer keeps its old setpoint:
on one Cube whose last memory slot the command took, no member showed it in
19 hours of polling, and two room commands to a room whose members were both
in error (2026-11-13 and 14) were never shown either.
An `S:` not discarded therefore says the Cube took the command, not that the room did.
The original software marks such a room "dirty" until an L line changes it.
```

**Replacing cp:756-757:**

```markdown
  The Cube changes a device's C line only after the device acknowledged (1 to 4 min);
  the setpoint in L also follows the device's acknowledgement, the mode its next report.
```

**Replacing cp:304-305:**

```markdown
While the UI is open it calls the backend every 10 s, and each call sends one `l:`
(2026-02-23 to 03-05: 88 000 polls of one Cube, 99.8 % 10 s apart).
The backend sends `q:` when the UI stops calling for a time set outside the jars,
about 30 s, not after 30 s without user activity: one Cube was held 10½ days
without a command.
When the connection idles it sends the line `e:something` as a keepalive.
```

**New, after "Observed polling": the H line.**

```markdown
## The H line

The greeting's first line, as the original software 1.4.6 reads it
(`MaxCubeStateFactory.parseHeader`), with what eleven greetings of three Cubes
showed in February 2026 (firmware 1.1.3):

| Field | Content |
|---|---|
| 0 | serial |
| 1 | radio address, hex |
| 2 | firmware, hex: `0113` (275) for 1.1.3 |
| 3 | not read; always `00000000` |
| 4 | not read; new at every connection |
| 5 | duty cycle in %, hex |
| 6 | free memory slots, hex (`32` = 50 when idle) |
| 7 | date `YYMMDD`, two hex digits each |
| 8 | time `HHMM`, two hex digits each, local time |
| 9 | "stateCubeTime", decimal; always 3 |
| 10 | "ntpCounter", hex; always `0000` |

The date and time were the PC's local time to the minute at every connect.
The software shows them and compares nothing with them.
```

**New, in "Lessons from the hardware days" (or a section "Device reports"):**

```markdown
- How often devices report (2026-02-19 to 03-05, three Cubes, firmware 1.1.3,
  from the original software's logs, which hold the L line every 10 s):
  - a wall thermostat about every 3 minutes (2.7 min at least);
  - a radiator thermostat in manual mode with its valve still, once a day at
    12:00 local, its decalcification hour, and not on its decalcification day
    (Saturday): its entry changes only every 24 h, over a weekend 48 h.
    In auto mode it reports at the programme's switch points;
  - a shutter contact only when the window moves.
- A radiator thermostat's measured temperature (bytes 9 and 10) stays in its
  L entry only for 10 s to 13 min after such a report (median 2.3 min), then
  reads 0 again.
  The original software never shows it; Massimo keeps the last one with its age.
- The valid bit (0x10 of the first flags byte) can clear without a restart.
  The entry is then "out of date", as the original software calls it: flag
  0x02 alone, the values of the device's last report kept.
  On one Cube six devices that had gone silent lost it within hours to days
  (2026-02-19 to 23); on two others a wall thermostat lost it for a single
  minute and came back unchanged (2026-11-12 and 13).
- Battery-low flickers: a radiator thermostat showed it for 2 to 4 minutes at
  a time, ten times in three days (2026-11-11 to 14), presumably under the
  valve motor's load, before it stays on.
- The duty cycle falls within the hour: from 21 % at 14:05 to 3 % at 14:42 on
  2026-11-13 with no client command, and 21 % five minutes after a full hour.
  It is probably the share of the last 60 minutes, not a counter reset on the hour.
- Costs (2026-02-19, the original software): `r:` nothing; a 0x40 to a room
  of two or three thermostats 3 % (about 1.1 s of air time), its memory slot
  back within 3 to 6 s once the members answered.
  A 0x40 to a room whose members did not answer held its slot for at least
  3.9 days at 0 % duty, and the Cube, with no slot left, discarded every
  command after it.
- In the original software's terms, byte 6 bit 0x40 is the `transmitError`
  (Massimo's `link_error`), and byte 5 bit 0x08 decides its `radioState`.
  Byte 5 bit 0x04 was seen only together with 0x08 (0x0c, 0x0e, 0x1e).
  Byte 6 bit 0x08 is set in winter as in summer: the device's automatic DST
  switching, not summer time.
- A Cube with DHCP on and no lease answers discovery from its stored fallback,
  192.168.0.222/16 on a Cube from the factory (2026-02-12), and cannot be
  reached over TCP from another subnet.
```

**In "LAN commands verified against the original software":**

```markdown
The original software 1.4.6 can also set the Cube's clock:
`MaxLocalApi.setCubeTime` sends `v:<rules>,<seconds in hex>`, lowercase and
unpadded, only when the seconds are above 0; the epoch is in its UI, not in its jars.
It never sends 0x81 (DaylightSavingTimeMode): the class exists, nothing uses it.
A day of the week programme with more than 7 switch points goes as two 0x10
telegrams, the second marked with bit 4 (0x10) of the day byte and padded to
6 points with the last one (`SetRoomWeekTemperatureProfile.createDayPartSpec`).
*Unverified on hardware.*
```

**docs/api-v1.md:** :1442-1444 and :1777-1778 as K1 ("when the member acknowledges"); :1782-1783 as K2 ("once a day at its decalcification hour in manual mode, at the switch points in auto"); `stale` (:1786) to say that an idle radiator thermostat changes only every 24–48 h.

## 8. Hardware tests it suggests

| Test | What | Sends | Settles |
|---|---|---|---|
| T130 | one day of 8 switch points to one radiator thermostat in the original's form (second telegram 0x10, padded), then its C line read back after the device acknowledged | 1 `z:` + 2 × 0x10, about 16 % duty on that Cube, and as much again to restore the day | G2 |
| T131 | the owner takes the batteries out of one wall thermostat of an unused room; serve's store shows when its valid bit drops; batteries back | nothing | N1's timeout |
| T132 | one radiator thermostat's decalcification hour 12 → 13 (0x12), a day of watching, then back | 2 × 0x12, about 6 % | K2: decalcification hour or noon |
| T133 | passive: read breno's budget readings again in a week | nothing | N6 |

No test is needed for K1, K3, N5 or the costs: the logs settle them.

## 9. Open decisions

### Test decisions (the gate offers these)

- **D130. The week programme's second telegram (T130)**
  - **A (recommended).** Test the original's form on one device of the owner's choice, through serve's proxy, after a test gate with the exact lines.
    Both forms are unverified here, and a wrong second telegram corrupts any day of more than 7 points.
  - **B.** Change the builder to the original's form without a test.
    The original shipped for years with it, but Massimo's rule wants a hardware check.
  - **C.** Leave it; no day here has more than 7 points today.
- **D131. The valid-bit timeout (T131)**
  - **A (recommended).** Yes, one wall thermostat, when the owner is there anyway.
    It costs no radio time and tells whether an OutOfDate entry means "silent for N hours".
  - **B.** No; D140 does not depend on the number.
- **D132. The decalcification hour (T132)**
  - **A (recommended).** No. The stale fix (D136) works with 48 h either way, and the test costs 6 % and a day.
  - **B.** Yes, on one radiator thermostat.
- **D133. The duty window (T133)**
  - **A (recommended).** The passive re-read in a week, nothing sent.
  - **B.** Take N6 as it stands.

### Implementation decisions (they wait for the tests)

- **D134. `pending_radiograms` at 0 free slots (G1)**
  - **A (recommended).** 0 free is 50 pending, once the budget has been read.
    Only a greeting without fields 5 and 6 (the portal's short H) counts as unknown.
  - **B.** Leave it.
- **D135. A room command not yet taken (G8)**, decided with D110
  - **A (recommended).** The command result and the room's state say "not confirmed" until each member shows the setpoint.
    After two polls without it, say so in the log at Warn.
    This is what the original's `stateDirty` did.
  - **B.** Leave "acknowledged" as the end.
- **D136. Stale threshold (G5)**
  - **A (recommended).** 50 h for radiator thermostats, 24 h for wall thermostats, both still settable.
  - **B.** One default of 50 h.
- **D137. Battery-low notification (G6)**
  - **A (recommended).** Notify only when the flag has stayed for 10 minutes, at most once a day per device.
  - **B.** Leave it.
- **D138. Move order and links (G3, G4)**
  - **A (recommended).** 0x23 first, as captured.
    Keep one room-flagged link per radiator thermostat until a test shows one anchor reaches them all.
  - **B.** Both changes, untested.
- **D139. The week-programme builder (G2)**: after D130.
- **D140. OutOfDate entries (G9)**
  - **A (recommended).** Show their values as "last known", with the age Massimo already keeps.
    A frozen value with its age says more than nothing.
  - **B.** Leave it: no state without the valid bit.
- **D141. The docs change**
  - **A (recommended).** One PR from this session after D102's docs PR is merged, rebased by a merge of main, with section 7's text less what D102 states.
  - **B.** Hand section 7 to the D102 session for its PR.
- **D142. Small corrections (G10–G13)**
  - **A (recommended).** One commit of comments and names, and the room-command fallback (G12) as a separate test-backed change.
  - **B.** Comments only.
- **D143. The duty hold (G7)**
  - **A (recommended).** After a hold, re-read the budget at the next `S:` instead of assuming the hour, holding again in steps of 10 minutes.
  - **B.** Leave it: the Cube accepts commands at 100 % anyway.
