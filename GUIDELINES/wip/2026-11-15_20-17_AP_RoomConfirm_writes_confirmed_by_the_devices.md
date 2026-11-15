# AP RoomConfirm v1.0: Writes confirmed by the devices

## TL;DR

A write to a room or a device says "not confirmed" where its devices do
not show it in time, instead of "ok" once the Cube has taken it. It costs
no radio time.

- **Today:** `{"ok": true}` from the Cube's answer → **then:**
  `"confirmation": "pending"`, done once every device shows it.
- **By the L entry, at each poll:** setpoint, mode, boost, vacation, key
  lock, display mode; 5 minutes, then not confirmed (D177-A).
- **By the C line, which the follow-up re-reads:** presets, offset,
  window-open, boost, decalcification, valve, week programme; compared
  with what was written, not only "changed" (D178-A′).
- **Display mode:** followed by the C line, which has none → by the L entry.
- **Shown:** the answer, the room's and the device's `confirmation`, an
  amber badge on the tile, a Warn in the log; no notification (D179-A).
- **Writers:** the API, and LAN proxy clients where the payload decodes.

### Blocks

1. Writes confirmed by the L entry
2. Configuration writes compared with the C line
3. The UI: the tile's badge, the device's mark, the log's line

## Discussion

The owner, 2026-11-15, on the TL;DR gate of `PLAN Block 15`:
"D178: what about device offset and other device-specific settings like
the WT's display mode? EVAL"; then, on the EVAL's gate,
"PLAN++ RoomConfirm, retire LegacyLogs in the commit that RoomConfirm
registers".

Decisions, as the EVAL left them:

- **D175-A:** a plan of its own, from the session that wrote AP LegacyLogs.
- **D176-A:** it does not wait for D110, AP RadioHealth's Block 2 (a
  room command resets its flagged members first). The check only reads
  states; Block 2 changes the queue, this plan the answer and the
  follow-up loop. Before Block 2, the check shows the losses that a
  flagged member causes today.
- **D177-A:** an L-entry write is not confirmed 5 minutes after the
  Cube's answer, checked at every poll.
- **D178-A′:** every write a device reports back, with one
  `confirmation` field and one Warn:
  - by the L entry: setpoint, mode, boost, vacation, key lock and display
    mode, to a room or a single device;
  - by the C line the follow-up already re-reads, now compared with what
    was written: the preset temperatures and the offset, window-open,
    boost and decalcification, the valve settings and the week programme,
    on the follow-up's schedule of about 6 minutes.
- **D179-A:** the command's answer, the room and the device in the API,
  an amber badge on the tile after the deadline (nothing while pending),
  a Warn in the log naming the devices and their flags; no notification.

**Taken over** from the plan that read the original software's logs, its
last block: a room command's result and its room's state say "not
confirmed" until each member shows the setpoint, and a Warn says so, as
the original software's `stateDirty` did. Its "after two polls" is 5
minutes here (D177-A): at the 10 s poll, two polls are 20 s, and members
came after 70 and 122 s.

**Clients of the LAN proxy:** the follow-up already follows their writes
as it follows the API's (`proxyWrote`). This plan does the same where
their payload decodes, so that the original software's writes are
confirmed too; a write that does not decode keeps today's check.

**After this plan:** AP RadioHealth's Block 8 (an unconfirmed room
command re-sent once, its flagged members reset first) builds on Block 1.

## Analysis

The analyses this plan rests on, in `wip/`:
`2026-11-14_21-13_ANALYSIS_LegacyLogs_protocol_findings.md` (K1, G8, D135),
`2026-11-15_04-58_ANALYSIS_LegacyLogs_owner_answers.md` (D135-B),
`2026-11-15_01-41_ANALYSIS_LegacyLogs_week_programme_test.md` (T130),
`2026-11-15_03-24_ANALYSIS_LegacyLogs_valid_bit_test.md` (T131),
`2026-11-15_13-55_ANALYSIS_LegacyLogs_link_test_v2.md` (T134) and
`2026-11-15_14-16_ANALYSIS_LegacyLogs_room_setpoint_test.md` (T135).

**What the Cube reports, and when** (`docs/cube-protocol.md`):

- **The L entry** takes a new setpoint and mode only when the device has
  acknowledged it (K1). A member that does not answer keeps its old ones,
  and a radiogram waiting in a memory slot shows nothing.
- **The C line** changes only when the device has acknowledged a
  configuration write. T130: a written day of 8 switch points was in the
  C line exactly as sent within 17 s.
- **The display mode** of a wall thermostat is byte 7 of its L entry
  (0x01 the setpoint, 0x04 the measured temperature). It is not part of
  the C record (`internal/protocol/cube/config.go`, `WallConfig`).
- **The valid bit** means "heard lately" (T131): an entry out of date
  keeps its last values and cannot confirm anything until it is heard
  again.
- **A room command** sets every member of the room it numbers (T135).
  Room-flagged link radiograms are delivered per member, at each one's
  next wakeup (T134).

**How long devices take** (serve's store on breno, read-only, the room
commands of the API since 2026-11-13):

| Cube | Room | Members showed the setpoint after |
|------|------|-----------------------------------|
| .25 | 1 | 10 s, 70 s, 122 s |
| .22 | 3 | 54 s |
| .25 | 6 | never, twice (D102's lost commands, flagged members) |

**Today's code:**

- `setpoint()` in `internal/server/server.go` answers
  `{"ok": true, "discarded", "budget"}` once the queue has the Cube's
  `S:` answer. Nothing looks at the devices afterwards.
- **The follow-up** (`internal/server/followup.go`) re-reads a written
  device's C line at 15 s, 30 s, 1, 2 and 2 minutes, about 6 minutes in
  all. It asks only whether the line differs from the one right after the
  write, logs "taken over" or "unchanged" at Info, and tells neither the
  API nor the UI. A display-mode write is followed by the C line, which
  never shows it.
- **The structure read-back** of AP DeviceUnPairingFix's Block 3
  (`internal/server/structure.go`) is the pattern to follow: a watch begun
  before the write, checked in the follow-up loop, an answer of
  `"confirmation": "pending"`, and an event that names what was not
  confirmed and the members' flags.
- **Proxy clients:** `cube.ParseRadioCommand` decodes the header only;
  `proxyWrote` sorts by the command byte.

**Costs:** none on the radio. The L entries come with the poll, and the C
lines with the follow-up's re-reads, which are LAN requests.

## Implementation Plan

### Block 1: Writes confirmed by the L entry

1. `internal/server/confirm.go`: what each device is expected to show,
   per runtime, by field, with the write it came from, when the Cube
   answered and the deadline. A newer write of a field replaces that
   field's expectation.
   - manual: mode and setpoint; auto: mode; boost: mode; vacation: mode,
     setpoint and end;
   - key lock: the lock; display mode: byte 7 (`display_actual`).
2. **Begun** after the Cube's answer:
   - `setpoint()`, for a room every thermostat of the room by its C line,
     the hidden ones left out, else the one device;
   - the key-lock and the display-mode writes;
   - `proxyWrote` for 0x40, the key lock and 0x82, with
     `cube.ParseRadioCommand` keeping the payload.
3. **Checked** at every state of a device (the pump) and at the deadline,
   5 minutes by default, an option for the tests:
   - a device that shows every field is done;
   - the rest are not confirmed: one event of the new kind `unconfirmed`
     at Warn per write, naming each device, what it shows, and its error
     flag, link error and out-of-date mark;
   - a device that shows it later is done then, with an Info line.
   - Kept in memory only.
4. **The answer** of a write carries `"confirmation": "pending"`.
   A device has `confirmation` (`state` pending or unconfirmed, `since`,
   the fields expected) while one is open; a room has it from its
   members, with the devices that have not confirmed.
5. **The display mode** leaves the C-line follow-up, in `config_api.go`
   and in `proxyWrote`.
6. **The kind** `unconfirmed` in `eventKinds`, the UI's `LOG_KINDS` and
   the kind's name in the six languages, as `TestLogKinds` holds them
   together.
7. **Tests** against the fake Cube, with a short deadline: a room
   command its members take; a member that does not (the fake Cube told to
   ignore it), with the event's members and flags; a later show; a newer
   write that replaces one; a proxy client's 0x40; a display write left
   out of the C-line follow-up.
8. **Docs:** `docs/api-v1.md` (the answer, devices, rooms, the kind),
   `docs/api-v1-openapi.yaml`, `docs/proxy.md` (its clients' writes).
9. CHANGELOG fragment `Added`.

- **Verification:** the Go gate, `make race`, `make cross`, the lint
  commands, `GUIDELINES/shared/tools/check`.

### Block 2: Configuration writes compared with the C line

1. **What was written** goes with the device into the follow-up:
   - 0x11, the temperatures: comfort, eco, maximum, minimum, offset,
     window-open temperature and time;
   - 0x12, the valve: boost time and valve, decalcification day and
     hour, maximum valve, valve offset;
   - 0x10, a day of the week programme.
2. **Each re-read** compares the parsed C line (`ThermostatConfig`,
   `WallConfig`) with it, field by field:
   - all shown: done, "taken over by the device" at Info as now;
   - after the last re-read, about 6 minutes: not confirmed, the same
     `unconfirmed` event naming the fields that differ, and the
     device's `confirmation` with them.
3. **Room writes** follow each thermostat of the room, as now.
4. **Proxy clients:** their 0x10, 0x11 and 0x12 are decoded the same
   way; one that does not decode keeps today's "changed at all".
5. **Tests** against the fake Cube, with short delays: a write the C line
   shows; one it does not, with the fields; a week programme's day; a
   proxy client's 0x11.
6. **Docs:** `docs/api-v1.md`, `docs/api-v1-openapi.yaml`.
7. CHANGELOG fragment `Changed`.

- **Verification:** the Go gate, `make race`, the lint commands,
  `GUIDELINES/shared/tools/check`.

### Block 3: The UI

1. **The room tile:** an amber "not confirmed" badge once its room's
   `confirmation` is unconfirmed, its title naming the devices and what
   they show; nothing while pending.
2. **A device**, in its room's detail and its settings: the same mark,
   with the fields of a configuration not confirmed.
3. **The log:** the `unconfirmed` line's summary, the devices by name
   and what they show.
4. **Texts** through `t()` in all six languages, FR, IT, NL and PL
   machine-translated; phone width first.
5. **Tests:** `tools/ui-test` with the answers stubbed, at 1100 and
   390 px.
6. **Docs:** `docs/ui.md`.
7. CHANGELOG fragment `Added`.

- **Verification:** as Block 1, plus `make ui-test`.

## Proposed commit messages

- `chore(aps): +RoomConfirm v1.0, -LegacyLogs`: this plan registered,
  and the plan it takes its first block from retired, with its four NOTEs;
  the six analyses above stay, under their names. Where that plan cannot
  retire yet, because another plan still names one of its NOTEs, this
  plan is registered alone (`chore(aps): +RoomConfirm v1.0`), and the
  retirement is a commit of its own once nothing names it.
- Block 1: `feat(server): A write is confirmed once the devices show it`
- Block 2: `feat(server): A configuration write is compared with the C line`
- Block 3: `feat(ui): A room or a device that has not confirmed a write says so`

## Change History

- **v1.0** (2026-11-15): first version, on the owner's
  `PLAN++ RoomConfirm`, with D175-A, D176-A, D177-A, D178-A′ and D179-A.
