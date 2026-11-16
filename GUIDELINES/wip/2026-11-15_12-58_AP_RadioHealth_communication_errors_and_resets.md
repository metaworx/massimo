# AP RadioHealth v1.3: Communication errors, checks and repair

## TL;DR

Massimo finds, resets and repairs what goes wrong between the Cube and its
devices; a command counts by its results, the original software is a
baseline. Built by sub-agents on 2026-11-15 and 16: Blocks 1–7 and 9–12
merged, the run's decisions with them; left are Block 8, Block 14 and,
after the spare-Cube run, Block 13.

- **Errors:** byte 4 → the failed radiogram; 0x08 "communication error",
  0x40 "link error"
- **Resets:** a command's flagged devices first, its room's and those it
  addresses; a flagged device that reports, once a day
- **Watched:** slots (< 5 free a Warn, < 2 an Error), flags back or
  lasting, C lines against the list, disagreeing rooms, drift
- **Repair:** the `repair` capability; each repair previewed, then run;
  "unlink all" apart
- **Next:** Block 8 → an unconfirmed room command re-sent once, one
  re-send a day per room with Block 9's (D199-A); Block 14 → the run's
  follow-ups

### Blocks

1. Byte 4: the failed radiogram, decoded and logged (merged, #309)
2. Room commands reset their flagged members first, for 0x08 only (merged, #311)
3. "Communication error": the names in the UI, the log and the docs (merged, #309)
4. A flagged device that reports is reset, once a day at most (merged, #311)
5. A Cube whose slots stay full is warned of (merged, #317)
6. A flag that keeps coming back is reported (merged, #315)
7. C lines and room masters checked against the metadata, and each room (merged, #317)
8. An unconfirmed room command is re-sent once
9. Disagreeing members and programme drift are reported (merged, #313)
10. The `repair` capability (merged, #312)
11. Repair actions: resets, configuration, links, rooms (merged, #312; item 4's spare-Cube run to come)
12. "Unlink all and re-link", behind its own warning (merged, #312; its spare-Cube run to come)
13. A room whose link error lasts is re-linked, where it is safe
14. The run's follow-ups: pairing's default name, a name too long, each flag notified, the preview's resets

## Discussion

The D102 work (2026-11-14 and 15) asked whether Massimo should reset a
device's transmit error itself. The tests are done, D115 the last; the
analyses this plan rests on are
`wip/2026-11-14_19-15_ANALYSIS_RadioHealth_transmit_error_d102.md`,
`wip/2026-11-14_20-30_ANALYSIS_RadioHealth_transmit_error_d102_v2.md` and
`wip/2026-11-15_11-44_ANALYSIS_RadioHealth_transmit_error_d102_v3.md`.
Where they and this plan differ, this plan holds. The decision numbers are
this plan's own.

The owner, 2026-11-15: "D 153 A, 154 A, 156 A … PLAN+ RadioHealth +
DeviceUnPairingFix"; then, on what the plan is for: "we cannot know for
sure, that what the legacy app sends as commands to the cube work reliably
in all circumstances … take the legacy protocol as a baseline but be
vigilant to logical errors also there … One of the most important things
would be to focus on how to reliably recover from erroneous situations";
and "We don't need to know the internals of the radio signals. We need to
know the working commands we need to send to the CUBE in order to get
reliable results." Then "D 162 A, 163 A … with B as a separate repair step
with a special warning and confirmation", and "PLAN+ both". To the gate
of v1.1: "102 B - with a warning if it comes back with the next report?",
"157 EVAL B - as long as we know all devices have reported recently …
only once, reporting as warning, marked seen if the auto-repair clears it.
an error if it stays for more than 24hrs? what would mark a 'clear'?",
and "does D102 need a 'job queue'?" To the run of v1.2: "EXEC RadioHealth
1-2", "the entire AP is due", "Use sub agents if you can split the work
onto multiple sessions"; and to its last gate "D203 A, PLAN++".

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
- **D102-B, with a warning:** one `r:` when a flagged device reports, at
  most once a day; its flag back by its next report, or later that day,
  a Warn and no more automatic resets for it until it stays clear a day;
  still flagged after 24 hours, an Error. The flag is set only by a failed
  radiogram of the Cube, so a flag back so soon means the Cube failed again.
- **D110-A:** one `r:01` per flagged member of the numbered room before a
  room command; T135 showed the room number deciding every member.
- **D111-A:** 0x08 only; 0x40 is the devices' own report.
- **D164-A:** detection, reset and repair in this plan; AP LegacyLogs kept
  its Block 15, which became AP RoomConfirm, on which Block 8 builds.
- **D165-A:** C lines and room masters against the metadata at every
  greeting, C line and metadata write, at no radio cost.
- **D166-A:** a new `repair` capability, the owner's and `manager`'s by a
  migration, in the integration role's ceiling; "unlink all and re-link"
  needs `repair` and `structure`.
- **No job queue** for D102 (the owner's question): see the Analysis.

**Settled in the run** (2026-11-15 and 16; the questions as asked, with
the owner's words, are in the four decision NOTEs beside the plan,
`wip/2026-11-15_21-43_NOTE_RadioHealth_v1.2_open_decisions_of_the_run.md`,
`wip/2026-11-15_22-04_NOTE_RadioHealth_v1.2_owner_decisions_of_the_run.md`,
`wip/2026-11-15_23-02_NOTE_RadioHealth_v1.2_decisions_of_blocks_5_and_7.md`,
`wip/2026-11-15_23-04_NOTE_RadioHealth_v1.2_owner_decisions_of_blocks_5_to_7.md`;
how the run went in
`wip/2026-11-16_00-05_NOTE_RadioHealth_v1.2_how_the_run_went.md`):

- **D180-B:** the `device_error` notification names the communication
  error or the link error, with the radiogram that failed (#318).
- **D181:** the flag alert names, of the radiograms that failed while the
  flag stayed set, the most severe, then the oldest; severity by the
  original software's categories: Assignment and Remove, then
  Configuration, then Control, then the rest (#318).
- **D182-A:** a line sent directly to a flagged device that is not the
  command's Target gets an `r:` first too (#318).
- **D183-B:** a command of `r:` lines alone passes the duty and free-slot
  holds; an `r:` takes no air time and no slot (#318).
- **D184-A:** a repair's preview counts 1.25 % per radiogram until the
  spare-Cube run measures it.
- **D185-A:** a device the list lacks is listed under the name given,
  else "<room> <type> <n>" (#318); a room the list lacks is named by the
  user. D201 changes the long name.
- **D186:** moot.
- **D187-B:** a wall thermostat back from new batteries is taken as it
  is; the owner: "if someone changes the batteries, they can also verify
  if it's on auto or manu or whatever."
- **D188-A, refined:** a disagreeing room's re-send takes Massimo's
  setpoint within the 15 minutes (auto again where a switch point passed
  since), else what most of its thermostats show, else the wall
  thermostat's state, else only the report (#318).
- **D189-A:** the clear's Info settles the Warn and the Error.
- **D190-A:** the kind "error watch".
- **D191-A:** a device without a valid state since the Cube's restart
  starts its 24 hours again at a `serve` restart.
- **D192-A, with thresholds:** no reconnect of its own (the refresh
  interval is in Maintenance); fewer than 5 free slots at two readings an
  hour apart a Warn, fewer than 2 an Error (#317).
- **D193-B:** the original software's room-list writes through the proxy
  get no quiet minutes; "such a user probably knows what they do."
- **D194-A:** the UI fixture's window contact has its C line; the full
  suite is CI's (#317).
- **D199-A:** one lost room command, one extra write: Block 9 leaves a
  room alone while it has an open or unconfirmed confirmation, and Block
  8's re-send and Block 9's share one re-send a day per room (Block 8).
- **D200-A:** pairing prefills "<room> <type> <n>" as well (Block 14).
- **D201, the owner's own:** "leave it too long with the warning mechanism
  of a too long value before saving (similar to the limit used in the
  paring time) so the user needs to correct the value but they know what
  it is. In case of no user interaction, drop the room name." (Block 14).
- **D202-A:** each flag's start is notified, within the half-hour limit
  (Block 14).
- **D203-A:** Massimo's setpoint is re-sent only while a thermostat of the
  room still shows it, so that people's later changes are not undone.
- **D204-A:** a repair's preview counts the queue's resets too (Block 14).

**Open, written here as recommended:**

- **D157. An automatic re-link when a member's 0x40 lasts**
  - **B, with conditions (recommended).** Block 13:
    - only when every thermostat of the room reported within 15 minutes
      and no member has 0x08; window contacts left out of the condition,
      and a silent partner reported as such, which a re-link cannot fix;
    - no wakeup to probe: it makes devices listen, not report;
    - once per room and day, a Warn when it acts, marked seen when it
      clears;
    - cleared: the Cube's free slots back at their count before it, then
      every member that had 0x40 without it at two reports in a row, and
      no new 0x08;
    - an Error when 0x40 lasts 24 hours, repaired or not;
    - only after Block 11's manual re-link passed on the spare Cube and on
      hardware; its setting off until then.
  - **A.** None automatic: Block 6 names a lasting 0x40, Block 11's
    re-link is the owner's.

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
- **Time:** no job queue is needed. Each Cube is polled every 10 s, and a
  changed entry is an event: D102's reset and its warning are checked at
  the device's report. "Still flagged after 24 hours" is a timer keyed by
  the device, set when the flag shows and cancelled when it clears, as the
  notify engine's `delay` and `cancel` work (`internal/notify/rules.go`);
  its periodic `scan` covers a device gone silent. The times (flagged
  since, last reset) are kept per device and seeded at `serve`'s start from
  the store's log of flag changes, so a restart does not set them back.
- **C lines and metadata:** a device's room for the Cube is its C line's
  room byte, not the metadata's; the previous owner's unlisted devices,
  their C lines in rooms 4 and 5, blocked those rooms' commands. The Cube
  changes a C line only once the device acknowledged, 1 to 4 minutes later.
- **Confirmations** (AP RoomConfirm, on main since #316): a setpoint, key
  lock or display write opens a confirmation per device, a room's write
  one per visible thermostat of the room by its C line; 5 minutes after
  the Cube's answer, the devices that do not show it are `unconfirmed`
  (an event at warn naming each, with what it shows and its flags), and a
  device that shows it later is `confirmed_late`. Tested on hardware
  2026-11-15: a room of three radiator thermostats confirmed manual and
  auto in 20 to 25 s.

The code:

- `internal/protocol/cube`: `radiogram.go`, the radiogram types and their
  categories; `DeviceState.FailedRadiogram`.
- `internal/engine/queue.go`: the prelude's resets (the Target, a room's
  flagged members, the devices a command addresses directly), the holds
  an `r:`-only command passes; `Queue.Concerns`, `Pending`, `Holding`.
- `internal/server`: `autoreset.go` (Block 4), `errorwatch.go` (6),
  `slots.go` (5), `congruence.go` (7), `drift.go` (9, its re-send and
  daily limit), `repair_api.go` (10 to 12), `logalerts.go`; and AP
  RoomConfirm's `confirm.go`, `rt.roomConfirmation(room)` a room's open
  confirmation, nil for none, and `setpoint()` running `rt.intend(…)`
  before `rt.setpointConfirm(…)`.
- `internal/cubeio/rooms.go`, `move.go`: AP DeviceUnPairingFix's link
  half, which Block 11's re-link uses.

## Implementation Plan

Every block brings its `CHANGELOG.d/` fragment and its docs; the UI's
texts in all six languages (FR, IT, NL, PL machine-translated, marked).

### Block 1: Byte 4, the failed radiogram

**Done** (#309); how it went in the NOTE of the run.

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

**Done** (#311), with D182-A's direct lines (#318).

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

**Done** (#309), with D180-B's notification (#318).

1. The badge for 0x08 reads "communication error", its title the failed
   radiogram; "link error" for 0x40 with the original's sentence; the log
   the same.
2. `docs/ui.md`, `docs/api-v1.md` (the fields' meaning), `README.md`.
3. `tools/ui-test/`: the badge and its title at 1100 and 390 px.

- **Verification:** `make ui-test`, the Go gate, `GUIDELINES/shared/tools/check`.

### Block 4: A flagged device that reports is reset (D102-B)

**Done** (#311), with D183-B (#318).

1. `serve`, at each changed entry of a device whose 0x08 is set (a new
   valid entry, its values changed): one `r:01`, unless its last
   automatic reset is less than a day old or Block 6 holds it; at Info;
   not while a command of Massimo to its room is in flight.
2. The device's last reset kept, and seeded at start from the store's log.
3. Tests against the fake Cube: a report, one `r:`; another the same day,
   none.

- **Verification:** the Go gate, `make race`, `GUIDELINES/shared/tools/check`.

### Block 5: A Cube whose slots stay full

**Done** (#317), with D192-A's thresholds: fewer than 5 free a Warn,
fewer than 2 an Error, each at two readings an hour apart.

1. Free slots at 1 or 0 at two readings an hour apart, nothing of Massimo
   in flight: a Warn and an alert, once; back above, Info.
2. The Cube's panel says so and points to its Reboot (owner-only), which
   drops them.
3. Tests with a greeting of 1 free, read twice.

- **Verification:** the Go gate, `make ui-test`, `GUIDELINES/shared/tools/check`.

### Block 6: A flag that keeps coming back (D102-B)

**Done** (#315).

1. A device whose 0x08 is back by its next report after a reset, or later
   that day: a Warn, once a day, with the failed radiogram; Block 4 leaves
   it alone until it stays clear for a day.
2. A device flagged for 24 hours: an Error, through a timer keyed by the
   device (the notify engine's `delay`, cancelled when the flag clears);
   the flag's start seeded from the store's log at start.
3. A 0x40 that lasts 24 hours: an Error the same way; Block 13 may act
   before it.
4. Tests with the fake Cube and the notify engine's clock.

- **Verification:** the Go gate, `GUIDELINES/shared/tools/check`.

### Block 7: C lines against the metadata (D165-A)

**Done** (#317), with D194-A.

1. At every greeting, every C line, and every metadata write, without
   radio: a device whose C line's room differs from the metadata's; a
   C line in a room with no metadata entry (it blocks that room); a
   metadata entry the Cube has no C line for; a type that differs;
   a room whose master in the metadata (the room's address in the M line)
   is no device of the Cube, a device of another room, or no thermostat,
   warned only while no unread warning for it exists.
   Hidden and parked devices, and the minutes after Massimo's own
   structure change (until the C line follows, 5 minutes at most), are
   left out.
2. Each finding a Warn once and a mark on the device and its room, with
   the two rooms named; cleared at Info when it agrees again.
3. Within a room: members whose week programme or temperatures differ
   (D162-A, third tier), reported the same way.
4. Tests: fake Cube greetings with each case, a room master among them.

- **Verification:** the Go gate, `make ui-test`, `GUIDELINES/shared/tools/check`.

### Block 8: An unconfirmed room command is re-sent once (D162-A, D199-A)

On AP RoomConfirm's confirmations (`internal/server/confirm.go`).

1. A room write of Massimo's own whose confirmation is `unconfirmed` (the
   `unconfirmed` event, 5 minutes after the Cube's answer): the same
   write re-sent once as a room command, at low priority and behind the
   duty hold; the queue resets the room's flagged members first (Block 2).
   Still not shown by every member 5 minutes after the re-send: a Warn
   naming the members and what each shows.
2. Not re-sent: a write a newer write to the room superseded; a LAN proxy
   client's write, only reported (as D193-B leaves a proxy client's
   room-list writes to it); a room with a command of Massimo's waiting
   or running.
3. One re-send a day per room, shared with Block 9's (D199-A): a room
   re-sent by either is not re-sent by the other that day, and Block 9
   leaves a room alone while `rt.roomConfirmation(room)` is open or
   unconfirmed. The day's re-sends seeded from the store's log at start.
4. Tests against the fake Cube with a member that ignores the write: the
   re-send's exact lines with a flagged member's `r:`, no second re-send
   that day, Block 9 quiet while the confirmation is open and after
   Block 8's re-send, a superseded write not re-sent, the Warn.

- **Verification:** the Go gate, `make race`, the touched UI test files,
  `GUIDELINES/shared/tools/check`.

### Block 9: Disagreeing members and programme drift (D162-A)

**Done** (#313), with D188-A refined and D203-A (#318).

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

**Done** (#312).

1. `internal/server/auth.go`: `repair` in `capabilities` and in the
   integration role's ceiling; a migration seeding it for `manager`.
2. `docs/store.md` section 6, `docs/api-v1.md`, the UI's capability names.
3. Tests: the seed, a role without it refused.

- **Verification:** the Go gate, `GUIDELINES/shared/tools/check`.

### Block 11: Repair actions (D163-A)

**Done** (#312), with D185-A (#318); item 4's spare-Cube run is still to
come, under a gate of its own.

Each a route under `repair`, each showing its cost before it runs, each
judged afterwards by what the Cube shows (lines accepted, slots back,
no flag), in the Cube's, the room's and the device's panels:

1. Reset the communication errors of a room or of a Cube: one `r:01` per
   flagged device, free.
2. Re-send a device's configuration (temperatures, week programme) from
   its room's agreeing members: Block 7's third case.
3. Place a device in the room the metadata lists, or list it where its
   C line has it (metadata only, no radio): Block 7's first case.
   Name a thermostat of the room as its master, its wall thermostat,
   else its first radiator thermostat (D167 of AP DeviceUnPairingFix,
   metadata only): Block 7's master case.
4. Re-link a room, adding only: each member's group and links again,
   with AP DeviceUnPairingFix's link half; first on the spare Cube.
5. Tests: the exact lines of each against the fake Cube.

- **Verification:** the Go gate, `make ui-test`, `GUIDELINES/shared/tools/check`;
  item 4 on the spare Cube, under a gate of its own.

### Block 12: "Unlink all and re-link" (D163-B, apart)

**Done** (#312); its spare-Cube run is still to come.

1. A route needing `repair` and `structure`: every link of a room
   removed, then built again with the link half; the UI's warning names
   the cost and the risk of a run that stops halfway, and a second
   confirmation follows.
2. Tried first on the spare Cube, under a gate of its own, judged by the
   room's setpoints and wheel turns reaching every member.

- **Verification:** the Go gate, `make ui-test`, the spare Cube's run.

### Block 13: An automatic re-link where it is safe (D157-B)

Only once Block 11's re-link has passed on the spare Cube and on hardware.

1. A member whose 0x40 lasts an hour, every thermostat of its room
   reported within 15 minutes, no member with 0x08: the room re-linked
   once (Block 11's re-link, adding only), a Warn naming it; once per room
   and day, behind the duty hold.
2. Cleared: the slots back at their count, then no 0x40 at two reports in
   a row of each member that had it, and no new 0x08: the Warn marked
   seen. Not cleared within the day: Block 6's Error.
3. A setting in System settings → Maintenance, off by default until the
   hardware run; six languages.
4. Tests against the fake Cube: the conditions met and not met.

- **Verification:** the Go gate, `make ui-test`, `GUIDELINES/shared/tools/check`.

### Block 14: The run's follow-ups (D200-A, D201, D202-A, D204-A)

One commit each.

1. **Pairing's default name (D200-A):** the pairing form's name field and
   "Put into a room…" prefilled with "<room> <type> <n>" in the page's
   language, as the repair "list" does; the server applies the same
   default where a placement gives no name.
2. **A default name too long (D201):** a default name past the Cube's 32
   bytes stays in the field as it is, with a warning on the field, and
   cannot be saved until the user shortens it, as the pairing time's
   limit works; where no user is there (the API without a name), the
   room's name is dropped: "<type> <n>". This replaces the cut of the
   room's name of D185 (#318).
3. **Each flag notified (D202-A):** the `device_error` notification for
   each flag's start, the communication error's and the link error's,
   within the notification's half-hour limit.
4. **The preview's resets (D204-A):** the engine gives the resets its
   prelude would send for a command (the Target, a room's flagged
   members, the flagged devices it addresses directly), and a repair's
   preview counts its resets from them.
5. Tests for each, the exact lines and names against the fake Cube; the
   texts in six languages.

- **Verification:** the Go gate, `make race`, the touched UI test files,
  `GUIDELINES/shared/tools/check`.

## Proposed commit message

For Block 8:

```
feat(serve/confirm): An unconfirmed room command is re-sent once, its flagged members reset first

A room write of Massimo's that its members do not show 5 minutes after the Cube's answer is re-sent once, the room's flagged members reset first, and a Warn names the members that still do not show it; the re-send and the room checks' share one a day per room (AP RadioHealth v1.3, Block 8).

Key changes:
- internal/server: the re-send of an unconfirmed room write, its daily limit shared with drift.go
- docs/api-v1.md, the log's texts in six languages, the tests against the fake Cube

Assisted-by: claude-code:claude-opus-5-5
```

For Block 14, one commit each: `feat(ui/pairing): …` (item 1),
`fix(api/names): …` (item 2), `feat(notify/errors): …` (item 3),
`fix(api/repair): …` (item 4).

## Change History

- v1.0 (2026-11-15): first revision; D112-A, D116-A, D153-A, D154-A, D156-A
  settled; D102, D110, D111, D157 open.
- v1.1 (2026-11-15): the original as a baseline, a command judged by its
  results; T134 and T135 in the analysis; D162-A and D163-A (B apart)
  settled; D164, D165, D166 added, D165 with the owner's room-master
  check; Blocks 7–12 (checks and repair); Block 2 takes the room's
  members by their C lines.
- v1.2 (2026-11-15): folds NOTE RadioHealth v1.1: owner decisions;
  D102-B with its warning and Error, D110-A, D111-A, D164-A, D165-A,
  D166-A settled; no job queue for D102; D157-B with conditions, Block 13;
  the room master named in D167's order.
- v1.3 (2026-11-16): Blocks 1–7 and 9–12 done (#309, #311, #312, #313,
  #315, #317) and the run's decisions in #318; the decisions of NOTE
  RadioHealth v1.2: open decisions of the run, owner decisions of the
  run, decisions of blocks 5 and 7, and owner decisions of blocks 5 to 7
  taken into the Discussion (the four stay beside the plan, since NOTE
  RadioHealth v1.2: how the run went cites them); D199-A to D204-A
  settled, D201 as the owner's own; Block 8 on AP RoomConfirm with D199-A;
  Block 14 added; the Analysis and the code list as built.
