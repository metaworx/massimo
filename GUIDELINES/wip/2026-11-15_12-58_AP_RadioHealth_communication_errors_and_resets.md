# AP RadioHealth v1.1: Communication errors, checks and repair

## TL;DR

Massimo finds, resets and repairs what goes wrong between the Cube and its
devices: the error flag (byte 5's 0x08), lost room commands, disagreeing
members, and a Cube whose rooms differ from its list. The original software
is the baseline, not the proof: a command counts by its results.

- **Byte 4:** "unknown" → the failed radiogram's type, with the flag (D112-A)
- **Room command:** no reset → one `r:01` per flagged member of the
  numbered room first, for 0x08 only (D110, D111)
- **Names:** "transmit error" → "communication error" (D154-A)
- **A flagged device that reports:** reset once a day at most (D102)
- **Watched:** full slots, returning flags, C lines and room masters
  against the metadata (D165), unconfirmed commands, disagreeing members,
  programme drift (D162-A)
- **Repair:** a `repair` capability, owner and manager (D166); resets,
  re-sent configuration, re-linked rooms; "unlink all" a step of its own (D163)

### Blocks

1. Byte 4: the failed radiogram, decoded and logged
2. Room commands reset their flagged members first, for 0x08 only
3. "Communication error": the names in the UI, the log and the docs
4. A flagged device that reports is reset, once a day at most
5. A Cube whose slots stay full is warned of
6. A flag that keeps coming back is reported
7. C lines and room masters checked against the metadata, and each room
8. An unconfirmed room command is re-sent once
9. Disagreeing members and programme drift are reported
10. The `repair` capability
11. Repair actions: resets, configuration, links, rooms
12. "Unlink all and re-link", behind its own warning

## Discussion

The D102 work (2026-11-14 and 15) asked whether Massimo should reset a
device's transmit error itself. The tests are done, D115 the last; the
analyses this plan rests on are
`wip/2026-11-14_19-15_ANALYSIS_RadioHealth_transmit_error_d102.md`,
`wip/2026-11-14_20-30_ANALYSIS_RadioHealth_transmit_error_d102_v2.md` and
`wip/2026-11-15_11-44_ANALYSIS_RadioHealth_transmit_error_d102_v3.md`.
Where they and this plan differ, this plan holds.

The owner, 2026-11-15: "D 153 A, 154 A, 156 A … PLAN+ RadioHealth +
DeviceUnPairingFix"; then, on what the plan is for: "we cannot know for
sure, that what the legacy app sends as commands to the cube work reliably
in all circumstances … take the legacy protocol as a baseline but be
vigilant to logical errors also there … One of the most important things
would be to focus on how to reliably recover from erroneous situations";
and "We don't need to know the internals of the radio signals. We need to
know the working commands we need to send to the CUBE in order to get
reliable results." Then "D 162 A, 163 A … with B as a separate repair step
with a special warning and confirmation", and "PLAN+ both".

**Settled:**

- **D112-A** (2026-11-14): byte 4 decoded, logged with the flag and shown
  in the device's details.
- **D116-A:** one plan, after the tests; its link repair is D157.
- **D153-A:** the analyses rest here.
- **D154-A:** byte 5's 0x08 is called "communication error", as the
  original's UI calls it ("Communication error!", "Kommunikationsstörung!");
  "link error" stays for byte 6's 0x40, which the original names
  `transmitError` and explains as "The radio communication with another
  MAX! device in this room is disturbed." The API fields keep their names.
- **D156-A:** removal and moves are AP DeviceUnPairingFix's.
- **D162-A:** checks in three tiers, corrections only where no person's
  intent can be behind the difference: Massimo's own room commands
  confirmed per member and re-sent once; members of a room that disagree
  reported, then the room's state re-sent once; programme and clock drift
  reported only. Each correction rate-limited and behind the duty hold.
- **D163-A, with B apart:** owner-and-manager repair actions, each with
  its cost shown first: reset the communication errors of a room or a
  Cube, re-send a device's configuration, re-link a room adding only;
  "unlink all and re-link" is a step of its own, behind a warning and a
  second confirmation.
- **D113:** moot (#283, #287).

**Open, written here as recommended:**

- **D102. A flagged device that reports**
  - **B (recommended).** One `r:` when it reports, at most once a day per
    device, logged at Info (Block 4): the flag is then stale, and an
    all-flagged room loses room commands. `r:` costs nothing.
  - **C.** Reset only before commands (Block 2 alone).
  - **A.** An amber "earlier error": wrong, the flag stays until a reset.
- **D110. Room commands to a room with flagged members**
  - **A (recommended).** One `r:01,<address>` for each member of the
    numbered room whose 0x08 is set, silent members included, before the
    room command: the original does so in practice, and T135 showed the
    room number deciding every member.
  - **B.** Leave it: a room command to an all-flagged room is lost.
- **D111. What is reset**
  - **A (recommended).** 0x08 only; 0x40 is the devices' own report.
  - **B.** 0x40 as well: a line that does nothing.
- **D157. An automatic link repair for 0x40**
  - **A (recommended).** None: the Cube cannot repair a link between two
    devices; Block 6 names a lasting 0x40, and Block 11's re-link is
    there for the owner.
  - **B.** Re-link a room automatically when a member's 0x40 lasts a day.
- **D164. Where the checks and repair go**
  - **A (recommended).** Here, Blocks 7–12: detection, reset and repair in
    one plan. AP LegacyLogs keeps its Block 15 (room commands "not
    confirmed" per member, D135-B); Block 8 builds on it.
  - **B.** A plan of its own: three plans on the same flags and rooms.
- **D165. C lines against the metadata**
  - **A (recommended).** At every greeting, every C line after a change,
    and every metadata write, at no radio cost (Block 7); with the owner's
    addition, whether each room's master in the metadata is a thermostat
    of that room.
  - **B.** At the greeting alone: a half-done move shows hours later.
- **D166. The capability**
  - **A (recommended).** A new `repair`: the owner's, `manager`'s by a
    migration, in the integration role's ceiling as `structure` is.
    "Unlink all and re-link" needs `repair` and `structure`.
  - **B.** The existing ones (`setpoint`, `configure`, `structure`):
    nothing to grant or withhold repair as a whole.

## Analysis

What is known (the analyses; `docs/cube-protocol.md`, "Lessons from the
hardware days", the byte at offset 4, "Set temperature (0x40)
addressing"; AP LegacyLogs' analyses of T134 and T135 on `main`):

- **The flag** (0x08): the Cube's last radiogram to the device failed;
  byte 4 names it and keeps its value after an `r:`, so it is read only
  with the flag. Nearly always the Cube's own time push (0x03), also a
  client's radiograms (0x21, 2026-11-15).
- **Clearing it:** `r:` (no radio time, no slot) or a Cube restart, never
  the device's reports. A reset device stayed clear for days.
- **Room commands:** a room-flagged 0x40 sets every member of the
  numbered room, whichever member its header names, one slot and 3 % each
  (T135); a device of another room named in the header takes it too.
  The devices do not pass a Cube's command on among themselves; they do
  pass on a wheel turn. To an all-flagged room, room commands were
  accepted and lost (2026-11-13 and 14).
- **The original software** resets every flagged device in a command's
  scope before it, whatever byte 4 (its category rule never applies:
  `FrameType`'s map is emptied at start-up); never on its own, never
  for 0x40.
- **Slots:** a radiogram for a device that does not answer holds its slot
  until the device answers or the Cube restarts; one Cube held 49 of 50
  for months, shown only in its greetings. Room-flagged link radiograms
  took a slot per member, the addressed member's at once, the others' at
  the next wakeup (T134).
- **Names:** the original reads 0x08 as its radio error and names 0x40
  `transmitError` (parameter `unreachable`).
- **C lines and metadata:** a device's room for the Cube is its C line's
  room byte, not the metadata's; the previous owner's unlisted devices,
  their C lines in rooms 4 and 5, blocked those rooms' commands. The Cube
  changes a C line only once the device acknowledged, 1 to 4 minutes later.

The code:

- `internal/protocol/cube/devicelist.go`: byte 4 "unknown".
- `internal/engine/queue.go`: `deviceInError` is `Error || LinkError`; an
  `r:` only for `cmd.Target`; a room setpoint has none.
- `internal/view/view.go`: a virtual wall thermostat's `Consistent`, false
  while the room's thermostats disagree on mode, setpoint or until:
  the start of Block 9.
- `internal/server/auth.go`, `capabilities`, and `docs/store.md`
  section 6: the capabilities and the roles' seed; `manager` holds
  `structure`, `pair`, `restore`.
- `internal/server/ui/tiles.js`, `log.js`, the six `ui/i18n/*.json`;
  `internal/server/logalerts.go`; `docs/api-v1.md`, `docs/ui.md`.
- Re-linking a room and placing a device in its listed room need AP
  DeviceUnPairingFix's link half (its Block 2): Block 11 waits for it.

## Implementation Plan

Every block brings its `CHANGELOG.d/` fragment and its docs; the UI's
texts in all six languages (FR, IT, NL, PL machine-translated, marked).

### Block 1: Byte 4, the failed radiogram

1. `internal/protocol/cube`: the radiogram types (codes and names of the
   original's jar) as a Go type; `DeviceState.FailedRadiogram`, set from
   byte 4 only with 0x08; the decoder's comment for offset 4.
2. The device state's `failed_radiogram` in the API (omitted without the
   flag), the OpenAPI file, `docs/api-v1.md`.
3. The log's flag change names the type.
4. Tests: captured entries with 0x03, 0x21, 0xF1, and 0x09 without the
   flag; an unknown code kept as a number.

- **Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `TestOpenAPICoversRoutes`, `gofmt -l .`, `go vet` on three platforms,
  `GUIDELINES/shared/tools/check`.

### Block 2: Room commands reset their flagged members first

1. `internal/engine/queue.go`: before a room command's radio lines, one
   `r:01,<address>` per device whose C line names the room and whose 0x08
   is set; after a discard, the same once more and one retry.
2. `deviceInError` reads 0x08 only (D111-A).
3. Each reset at Info, with the device and the room.
4. Tests against the fake Cube, exact lines: two flagged members, a
   member with 0x40 alone, a device command with 0x40 alone.

- **Verification:** the Go gate, `make race`, `make cross`, the lint
  commands, `GUIDELINES/shared/tools/check`.

### Block 3: "Communication error"

1. The badge for 0x08 reads "communication error", its title the failed
   radiogram; "link error" for 0x40 with the original's sentence; the log
   the same.
2. `docs/ui.md`, `docs/api-v1.md` (the fields' meaning), `README.md`.
3. `tools/ui-test/`: the badge and its title at 1100 and 390 px.

- **Verification:** `make ui-test`, the Go gate, `GUIDELINES/shared/tools/check`.

### Block 4: A flagged device that reports is reset (D102-B)

1. `serve`: a device whose 0x08 is set and which reports (a new valid
   entry, values changed) gets one `r:01`, at most once a day, at Info;
   not while a command of Massimo to its room is in flight.
2. Tests against the fake Cube.

- **Verification:** the Go gate, `make race`, `GUIDELINES/shared/tools/check`.

### Block 5: A Cube whose slots stay full

1. Free slots at 1 or 0 at two readings an hour apart, nothing of Massimo
   in flight: a Warn and an alert, once; back above, Info.
2. The Cube's panel says so and points to its Reboot (owner-only), which
   drops them.
3. Tests with a greeting of 1 free, read twice.

- **Verification:** the Go gate, `make ui-test`, `GUIDELINES/shared/tools/check`.

### Block 6: A flag that keeps coming back

1. A device flagged again within a day of a reset: a Warn once a day with
   the failed radiogram; Block 4 leaves it alone until it stays clear a day.
2. A 0x40 that lasts a day is named the same way (D157-A).
3. Tests with the fake Cube.

- **Verification:** the Go gate, `GUIDELINES/shared/tools/check`.

### Block 7: C lines against the metadata (D165-A)

1. At every greeting, every C line, and every metadata write, without
   radio: a device whose C line's room differs from the metadata's; a
   C line in a room with no metadata entry (it blocks that room); a
   metadata entry the Cube has no C line for; a type that differs;
   a room whose master in the metadata (the room's address in the M line)
   is no device of the Cube, a device of another room, or no thermostat.
   Hidden and parked devices, and the minutes after Massimo's own
   structure change (until the C line follows, 5 minutes at most), are
   left out.
2. Each finding a Warn once and a mark on the device and its room, with
   the two rooms named; cleared at Info when it agrees again.
3. Within a room: members whose week programme or temperatures differ
   (D162-A, third tier), reported the same way.
4. Tests: fake Cube greetings with each case, a room master among them.

- **Verification:** the Go gate, `make ui-test`, `GUIDELINES/shared/tools/check`.

### Block 8: An unconfirmed room command is re-sent once (D162-A)

1. On AP LegacyLogs' Block 15 (a room command "not confirmed" until each
   member shows it): a command still unconfirmed after a few minutes is
   re-sent once, its flagged members reset first (Block 2); still
   unconfirmed, a Warn naming the members.
2. Rate-limited per room, behind the duty hold.
3. Tests against the fake Cube with a member that ignores the command.

- **Verification:** the Go gate, `make race`, `GUIDELINES/shared/tools/check`.

### Block 9: Disagreeing members and programme drift (D162-A)

1. Members of a room whose mode or setpoint differ for more than
   10 minutes (the view's `Consistent`): reported; at a second check, the
   room's state as Massimo last set it, or else its wall thermostat's,
   re-sent once.
2. In auto mode, a member that does not follow a switch point of the
   room's programme within minutes: reported, its clock or programme
   suspect, with its failed radiogram where flagged; never corrected.
3. Tests with the fake Cube's members set apart.

- **Verification:** the Go gate, `GUIDELINES/shared/tools/check`.

### Block 10: The `repair` capability (D166-A)

1. `internal/server/auth.go`: `repair` in `capabilities` and in the
   integration role's ceiling; a migration seeding it for `manager`.
2. `docs/store.md` section 6, `docs/api-v1.md`, the UI's capability names.
3. Tests: the seed, a role without it refused.

- **Verification:** the Go gate, `GUIDELINES/shared/tools/check`.

### Block 11: Repair actions (D163-A)

Each a route under `repair`, each showing its cost before it runs, each
judged afterwards by what the Cube shows (lines accepted, slots back,
no flag), in the Cube's, the room's and the device's panels:

1. Reset the communication errors of a room or of a Cube: one `r:01` per
   flagged device, free.
2. Re-send a device's configuration (temperatures, week programme) from
   its room's agreeing members: Block 7's third case.
3. Place a device in the room the metadata lists, or list it where its
   C line has it (metadata only, no radio): Block 7's first case.
   Name a thermostat of the room as its master, its first radiator
   thermostat, else its wall thermostat (metadata only): Block 7's
   master case.
4. Re-link a room, adding only: each member's group and links again,
   with AP DeviceUnPairingFix's link half; first on the spare Cube.
5. Tests: the exact lines of each against the fake Cube.

- **Verification:** the Go gate, `make ui-test`, `GUIDELINES/shared/tools/check`;
  item 4 on the spare Cube, under a gate of its own.

### Block 12: "Unlink all and re-link" (D163-B, apart)

1. A route needing `repair` and `structure`: every link of a room
   removed, then built again with the link half; the UI's warning names
   the cost and the risk of a run that stops halfway, and a second
   confirmation follows.
2. Tried first on the spare Cube, under a gate of its own, judged by the
   room's setpoints and wheel turns reaching every member.

- **Verification:** the Go gate, `make ui-test`, the spare Cube's run.

## Proposed commit message

For Block 1:

```
feat(cube/errors): The failed radiogram of a communication error

Byte 4 of an L entry is decoded as the type of the Cube's last radiogram to the device that failed, read only with the error flag, logged with it and given in the device's state (AP RadioHealth v1.1, Block 1).

Key changes:
- internal/protocol/cube: the radiogram types; DeviceState.FailedRadiogram
- the API's failed_radiogram, the OpenAPI file, docs/api-v1.md
- the log's flag change names the failed radiogram

Assisted-by: claude-code:claude-opus-5-5
```

For Block 2:

```
fix(engine/queue): Room commands reset their flagged members first

The queue sends one r: for every device of the numbered room whose error flag is set before a room command, as the original software does, and no longer resets for the link error alone (AP RadioHealth v1.1, Block 2).

Key changes:
- internal/engine/queue.go: the room's flagged members reset before a room command; deviceInError reads 0x08 only
- the exact-line tests against the fake Cube

Assisted-by: claude-code:claude-opus-5-5
```

The other blocks follow the same form: `feat(ui/errors)`,
`feat(serve/errors)`, `feat(serve/slots)`, `feat(serve/errors)`,
`feat(serve/congruence)`, `feat(engine/confirm)`, `feat(serve/drift)`,
`feat(auth/repair)`, `feat(api/repair)`, `feat(api/repair)`.

## Change History

- v1.0 (2026-11-15): first revision; D112-A, D116-A, D153-A, D154-A, D156-A
  settled; D102, D110, D111, D157 open.
- v1.1 (2026-11-15): the original as a baseline, a command judged by its
  results; T134 and T135 in the analysis; D162-A and D163-A (B apart)
  settled; D164, D165, D166 added, D165 with the owner's room-master
  check; Blocks 7–12 (checks and repair); Block 2 takes the room's
  members by their C lines.
