# Transmit error (D102): what the flag is, and the tests that settle it

2026-11-14, evening. Read-only so far: breno's store, the five proxies' greetings
(nothing sent), the original software 1.4.6's logs and its jar.
**Nothing has been sent to any Cube in this session.**
No serials, radio addresses or names below; Cubes are named by their proxy.

## TL;DR

- **The flag says "my last radiogram to this device failed".**
  The original software reads L entry byte 4 (Massimo: "unknown") as the
  type of that radiogram, but only while the flag is set.
  On breno now: **16 flagged entries on four Cubes, 15 with 0x03 TimeInformation**,
  which the Cube sends by itself, and 1 with 0xF1 WakeUp.
  Entries without the flag carry 0x09, the one reset at 18:09 included.
- **It stays until `r:`**, for days, while the device keeps reporting:
  one wall thermostat for 24 h until the 18:09 reset,
  nine devices on the Cube behind .32 at every connect since 2026-11-11.
- **While it is set, room commands are lost without a word** (new):
  two room setpoints to a room whose two devices were both flagged
  (2026-11-13 and 14) were accepted (`S` not discarded, one slot),
  and neither device ever took them.
  Massimo's queue resets only before a command addressed to a device,
  so a room setpoint gets no `r:`.
- **The original software resets before every command, never on its own**:
  one `r:01,<address>` per flagged device in the command's scope,
  which for a room setpoint is every device of the room.
- **Open, for the hardware test:** whether `r:` costs radio time or a slot
  (one sample says no; a 2026-11-05 report suggests yes, for a contact),
  whether the flag comes back by itself, and what holds 49 of 50 slots on one Cube.

## 1. The flag, as the original software reads it

`DeviceStateSerializer.parseDeviceList` (MaxEssentialsBackend 1.4.6) splits
each L entry into: 3 bytes address, **1 byte the failed radiogram's type**,
1 byte flags (0x10 valid, 0x08 radio error, 0x02 initialized), then the state.
With 0x08 set, it looks byte 4 up in `FrameType` and gets the device's
`RadioState`:

| Byte 4 | FrameType category | RadioState | Reset before a room command? |
|---|---|---|---|
| 0x03 TimeInformation, other "None" types | None | Error | yes |
| 0x10–0x12 configuration, 0x82 | Configuration | ConfigFailed | yes |
| 0x40, 0x41, 0x43, 0x44, 0x80 control | Control | ConfigFailed | yes |
| 0x20–0x23 links and groups | Assignment | AssignFailed | no; only before a device association |
| 0xF0 Reset | Remove | DeleteFailed | no; only before a removal |

Without the flag it ignores byte 4 (0x09 in every such entry on breno).
The docs already note byte 4 "changed after failed transmissions"
(values 0x09, 0x03, 0xF1, 0xF6) and the 68 % night of 2026-11-04 had byte 4 = 0x03:
the Cube's own time pushes to unmounted thermostats.

**A slip in the docs:** the FrameType list in `docs/cube-protocol.md` gives
0xF0 WakeUp, 0xF1 Reset and 0x70 CurrentTemperatureAndHumidity.
The jar's constructor (`String, int, byte, category`) gives **0xF0 Reset,
0xF1 WakeUp, 0x45 CurrentTemperatureAndHumidity**, and also lists
0x41 SetPointTemperature and 0xFF Test.
No code uses these codes (wakeups go as `z:`), so nothing sent is wrong.

## 2. The five questions

**Q1. Does `r:` cost radio time?** *Open.*
- 18:09, one wall thermostat on the Cube behind .35:
  `S:03,0,32`, duty as before, 50 of 50 slots; the flag gone at serve's next poll, 13 s later.
- Against it: `docs/reports/write-test-2026-11-05.md`, step 7c:
  "5 radiograms (the move's 4 and the queue's error reset)", and
  "the first window cycle delivered only the error reset" — for a window contact.
  That reads as if `r:` queued a radiogram for a sleeping device.
- Duty is reported in whole percent of 36 s; a short radiogram without the
  1 s wake burst would not show alone. T1 below settles it with a device that
  cannot answer: a queued radiogram shows as a slot and the flag stays;
  a bookkeeping reset clears at once with no slot.

**Q2. Does the Cube clear it by itself?** *Partly answered.*
- Not by the device's reports: 24 h on .35 (reports every 2–3 min),
  at least 3 days for nine devices on .32 (readings changing all along).
- Not by time, nor by serve reconnecting (.32: the same nine at all 21 greetings since).
- After a successful radiogram: no case seen. The Cube seems to send none
  to a flagged device (section 3), so this may not arise.
- After a Cube restart: not seen. Three flagged entries have the valid bit
  clear (no report since the Cube's restart), on .32, .33 and .35:
  the Cube flagged them after its restart, without ever hearing them.
  Whether a restart clears the flag first is T4 (optional).

**Q3. What sets it?** *Answered for these cases.*
- The Cube's own radiograms: 15 of 16 flagged entries failed on 0x03
  TimeInformation; no client sent a radio command to .32 or .33 in the
  store's history (since 2026-11-11).
- The two room-6 devices on .35 were silent at four connects on 2026-11-13
  (13:32–16:12 UTC, no valid entry) and came back at 17:16–17:17 already
  flagged: the Cube's time push failed while they were off.
- That the flag needs the Cube's retries first (not one lost radiogram)
  is likely, not shown.

**Q4. The Cube behind .32 (ten flagged, 1 slot free).** *Partly answered.*
- Nine of the ten report (valid bit, values changing): the Cube hears them.
  They are flagged after a failed time push (0x03 ×8, 0xF1 ×1),
  not out of reach. The tenth has not reported since the Cube's restart.
- 1 of 50 slots free at every connect since 2026-11-11 19:36 UTC,
  duty 1 % (7 and 9 % at two night-time connects),
  and Massimo has never sent it a radio command, directly or through its proxy.
  So the slots are not Massimo's, and the Cube is not retrying hard.
- Not the flagged devices' radiograms, most likely: on .35 a radiogram to the
  flagged room freed its slot within 2 min once, within 4 h the other time.
  Candidates: radiograms waiting for the seven window contacts that have
  not transmitted since the Cube's restart (their entries all zero).
- T3 resets the nine and watches the slots; a Cube restart would drop
  whatever is queued, so it is not proposed for this Cube.

**Q5. Does the original software send `r:`, and when?** *Answered (bytecode).*
- `ResetRadiogramBuilder` wraps every client command: if the command makes
  radiograms, it puts one `ResetDeviceRadiogram` (`r:01,<address>`) per
  device in front, for each device whose `RadioState` asks for it (table above).
- The scope comes from the command's annotation:
  room setpoint and mode (`SetRoomPermanentMode`, `SetRoomAutoMode`,
  `SetRoomTemporaryMode`): **every device of the room**;
  room configuration (presets, week programme): the room;
  DST mode: every room and every device; association and removal: the device.
- It never resets on its own: no timer, nothing on a report.
  Its logs since 2026-11-12 hold one radiogram (an `s:` through the proxy
  to a room without flags) and no `r:`.

## 3. New: room commands to a flagged room are lost

Room 6 of the Cube behind .35 holds a wall thermostat and a radiator
thermostat, both flagged from 2026-11-13 17:16 UTC.

| When (UTC) | Command | Cube's answer | What the devices reported after |
|---|---|---|---|
| 11-13 17:25 | room 6 manual 5.5 °C | not discarded, 49 free | still auto at every report to 17:41 |
| 11-14 16:22 | room 6 auto | not discarded, 49 free; 50 free 2 min later | still manual 20 °C at every report to 17:10 |

The 2026-11-04 capture saw a device command to a flagged device discarded
(`S:14,1,31`); a room command is accepted and dropped.
Massimo's queue (`internal/engine/queue.go`) sends `r:` only for
`cmd.Target`, which a room setpoint leaves zero (`internal/server/server.go`,
`setpoint`), and retries only a discard.
It also resets for `link_error` (byte 6, 0x40), which the original software
never does.

## 4. The tests proposed

All three directly to the Cubes, with serve stopped on breno,
by `GUIDELINES/temp/d102/rtest.py` (run on breno; tried against the fake Cube):
it reads the greeting, picks the flagged entries by a rule, refuses to send
if their number is not the one expected, sends the `r:` lines, then `l:`
every 2 s, and prints flags, byte 4, duty and slots without addresses.

| Step | Cube (own address) | Lines sent | Shows |
|---|---|---|---|
| S1 | — | `sudo systemctl stop massimo` | the five Cubes free; heating runs on the Cubes, the UI and HA pause |
| T1 | behind .33 (.23), quiet: duty 0 %, 50 free | `r:01` for its one flagged device, the one not heard since the restart; `l:` ×16 over 30 s; `q:` | radiogram or bookkeeping: a slot, duty, a flag that stays — or none |
| T2 | behind .35 (.25) | `r:01` for the flagged radiator in room 6 (reporting, valve moving); `l:` ×16; `q:` | a second sample, a device that needs the wake burst: +3 % duty if `r:` transmits |
| T3 | behind .32 (.22) | `r:01` for one of its nine reporting flagged devices, then one `r:08` line for the other eight; `l:` ×31 over 60 s; `q:` | the count form; whether the slots change (1 free) |
| S2 | — | `sudo systemctl start massimo`, then check the five connect | serve's next greetings give fresh duty and slots |

- **The window:** about 6 minutes; on 2026-11-14 00:26 a 4-second stop
  reconnected all five at once.
  A Cube that refuses the direct connection is skipped, not waited for.
- **Radio cost:** none if `r:` is bookkeeping; otherwise one radiogram per
  device: about 3 % each with the wake burst, so at most about 30 % on .32,
  where duty is 1 %. T1's device has not been heard since the Cube's restart, so probably
  cannot answer: if `r:` transmits, the Cube retries it, like the time pushes that took the 68 % night.
- **Afterwards, passively:** the store shows whether and when each reset
  device is flagged again, and with which byte 4: the Cube's next time push.
  Nothing more is sent.
- **Not proposed now:** T4, a restart of the Cube behind .33 (UDP `R`) to see
  whether a restart clears the flag; T5, a room setpoint 0.5 °C off and back
  on a flagged room, then after its reset, to see a room command lost and
  then delivered. Each follows from T1–T3 if still needed.

## 5. Open decisions

- **D102. A flagged device that reports**
  - **B (recommended, if T1 shows no slot and no duty).** Massimo sends `r:`
    for it at its report, at most once a day per device, logged at Info.
    The flag blocks every client radiogram to the device, its room's included,
    and is nearly always the Cube's own failed time push; the report shows
    the device is up. Cost: one `r:` a day per such device, free if T1 says so.
  - **C.** Reset only before a command that reaches the device, as the
    original software does (D110 alone). No reset traffic;
    the red badge stays on idle devices for days, as on .32 now.
  - **A.** Amber "earlier transmit error". Says the wrong thing:
    the flag is not history, commands to the device are dropped until `r:`.
- **D110. Room commands to a room with a flagged device** (independent of D102)
  - **A (recommended).** The queue resets every flagged device of the room
    before a room command, as the original software does,
    in one `r:` line if T3 shows the count form works, else one line each.
    Two of two room setpoints were lost without it.
  - **B.** Leave it: a room setpoint is lost while any member is flagged,
    and the answer says "ok".
- **D111. `link_error` alone**
  - **A (recommended).** Reset for the error bit only, as the original
    software; skip a device whose failed radiogram was a link (0x20–0x23)
    unless the command is a link change. `link_error` is the device's own
    report and clears with its next transmission (2026-11-05).
  - **B.** Keep resetting for it too: harmless if `r:` is free, but noise in the log.
- **D112. Byte 4**
  - **A (recommended).** Decode it in `internal/protocol/cube` as the failed
    radiogram's type, log it with the flag ("transmit error: time information")
    and show it in the device's details.
  - **B.** Leave it raw.
- **D113. The docs change**
  - **A (recommended).** One PR from this session tonight:
    the 18:09 finding, today's passive findings (byte 4, the lost room
    commands, the original software's resets, the FrameType slip),
    and T1–T3's results if they run tonight.
    The unpushed commit on `claude/reset-error-finding` (jolly-yalow) is then
    dropped there: you tell that session, or let me message it.
  - **B.** Build on that branch's commit (cherry-pick) and add to it:
    keeps its wording, but that session must still drop its copy,
    or the two conflict when it merges main.

## 6. docs/cube-protocol.md, proposed

- "Lessons from the hardware days": one entry for the flag —
  set by a failed radiogram, nearly always the Cube's own time push;
  kept for days while the device reports; cleared by `r:` (18:09, and T1–T3);
  room commands to a flagged room accepted and lost (the two cases above).
- The byte-4 paragraph: what byte 4 is (the failed radiogram's type,
  read only with the flag), 0x09 without it.
- The FrameType list: 0x41, 0x45, 0xF0 Reset, 0xF1 WakeUp, 0xFF Test.
- "LAN commands verified against the original software": when it sends `r:`
  (before every command, per device, by scope), from the 1.4.6 jar.
- All with the date and firmware 1.1.3, the Cubes as "one of the owner's Cubes".

## How it was read

Scripts in `GUIDELINES/temp/d102/` (piped to breno over SSH, `sudo -n python3 -I`,
`file:/var/lib/massimo/massimo.db?mode=ro`): `timeline.py`, `cuberows.py`,
`commands.py`, `room6.py`, `before.py`, `slots.py`, `c7cmds.py`, `audit.py`;
`greet.py` for the proxies' greetings (connect and read, nothing sent).
The jar: copied to the session's scratchpad and read with `javap -c`
(`ResetRadiogramBuilder`, `ResetDeviceRadiogram`, `DeviceStateSerializer`,
`FrameType`, `FrameTypeCategory`, the client commands' `ClientCommandInfo`).
