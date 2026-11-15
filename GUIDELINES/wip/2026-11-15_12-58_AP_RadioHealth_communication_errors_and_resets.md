# AP RadioHealth v1.0: Communication errors and resets

## TL;DR

Massimo handles a device's error flag (byte 5's 0x08) as the original MAX!
software 1.4.6 does in practice, names it as that software's UI does, and
watches what the Cube shows nowhere but in its greeting.

- **Byte 4:** "unknown" → the failed radiogram's type, read only with the flag (D112-A)
- **Room command:** no reset → one `r:01` per flagged member first (D110)
- **What is reset:** 0x08 or 0x40 → 0x08 only (D111)
- **Names:** "transmit error" → "communication error", its text by byte 4;
  "link error" kept, with the original's sentence (D154-A)
- **A flagged device that reports:** flag kept → one `r:` at its report,
  at most once a day (D102)
- **Slots:** unwatched → a warning when a Cube's free slots stay at 1 or 0
- **A flag that keeps coming back:** unseen → a Warn

### Blocks

1. Byte 4: the failed radiogram, decoded and logged
2. Room commands reset their flagged members first, for 0x08 only
3. "Communication error": the names in the UI, the log and the docs
4. A flagged device that reports is reset, once a day at most
5. A Cube whose slots stay full is warned of
6. A flag that keeps coming back is reported

## Discussion

The D102 work (2026-11-14 and 15) asked whether Massimo should reset a
device's transmit error itself. The tests are done, D115 the last; the
three analyses this plan rests on are
`wip/2026-11-14_19-15_ANALYSIS_RadioHealth_transmit_error_d102.md`,
`wip/2026-11-14_20-30_ANALYSIS_RadioHealth_transmit_error_d102_v2.md` and
`wip/2026-11-15_11-44_ANALYSIS_RadioHealth_transmit_error_d102_v3.md`.
Where they and this plan differ, this plan holds: the findings of
2026-11-15 afternoon (below) came after them.

The owner, 2026-11-15: "D 153 A, 154 A, 156 A … PLAN+ RadioHealth +
DeviceUnPairingFix".

**Settled:**

- **D112-A** (2026-11-14): byte 4 decoded as the failed radiogram's type,
  logged with the flag and shown in the device's details.
- **D116-A:** one plan, RadioHealth, after the tests: D110–D112 first, then
  D102 rate-limited, then the slot warning and the "keeps coming back"
  report. Its last item, a link repair, is D157 below.
- **D153-A:** the analyses rest here, in `wip/`, under this plan's name.
- **D154-A:** byte 5's 0x08 is called "communication error", as the
  original's UI calls it ("Communication error!", "Kommunikationsstörung!");
  "link error" stays for byte 6's 0x40, which the original names
  `transmitError` and explains as "The radio communication with another MAX!
  device in this room is disturbed." The API fields `error` and
  `link_error` keep their names.
- **D156-A:** Massimo's removal and moves are AP DeviceUnPairingFix's,
  not this plan's.
- **D113:** moot; the findings are in `docs/cube-protocol.md` (#283, #287).

**Open, each with its recommendation:**

- **D102. A flagged device that reports**
  - **B (recommended).** Massimo sends one `r:` for it when it reports,
    at most once a day per device, logged at Info (Block 4).
    The flag nearly always comes from the Cube's own time push after its
    restart; the report shows the device is up, and the flag is then only
    noise in the UI that also makes a client's room command to an
    all-flagged room get lost. `r:` costs no radio time and no slot.
  - **C.** Reset only before a command, as the original software does
    (Block 2 alone): no traffic of its own; the badge stays on an idle
    device for days.
  - **A.** Show it as an amber "earlier error": says the wrong thing,
    since the flag stays until a reset.
- **D110. Room commands to a room with flagged members**
  - **A (recommended).** The queue sends one `r:01,<address>` for every
    member whose 0x08 is set before a room command, silent members
    included, as the original software does in practice: its category
    rule never applies (`FrameType`'s map is emptied at start-up), so every
    flagged device in the scope is reset. One line per device, as the
    original sends it; the count form with several addresses is
    unverified.
  - **B.** Leave it: a room command to an all-flagged room is accepted,
    answered "ok" and lost.
- **D111. What is reset**
  - **A (recommended).** 0x08 only, as the original: byte 6's 0x40 is the
    device's own report of a disturbed link to a room partner, which `r:`
    does not touch and which clears with the device's next transmission
    that gets through.
  - **B.** Keep resetting for 0x40 as well: harmless, but a line that does
    nothing, logged as a reset.
- **D157. A link repair** (D116-A's last item)
  - **A (recommended).** Not in this plan: 0x40 is the devices' own
    report, the Cube cannot repair a link between two devices, and the
    original's advice is about distance, barriers and batteries.
    Block 6's report names it where it lasts.
  - **B.** A block that re-sends a room's links when a member's 0x40
    lasts a day: radio time for a cause the Cube cannot see.

## Analysis

What is known, with the evidence in the three analyses and in
`docs/cube-protocol.md` ("Lessons from the hardware days", the byte at
offset 4, the LAN commands verified against the original software):

- **The flag** (byte 5, 0x08) says the Cube's last radiogram to the device
  failed; byte 4 gives its type and keeps it after an `r:` (0x03 and 0x21
  seen), so byte 4 is read only with the flag.
  Nearly always the Cube's own time push after its restart (0x03), also a
  client's radiograms (0x21 of a removal, 2026-11-15).
- **What clears it:** `r:` (no radio time, no slot, five of five) or a Cube
  restart; never the device's own reports (a day, 19 hours, 10½ days
  watched). A reset device stayed clear for days.
- **Room commands:** to a room whose members were all flagged, accepted
  and never carried out (2026-11-13 and 14). A room-flagged radiogram was
  accepted with four of five members flagged (2026-11-15).
- **The original software in practice:** before every command it resets
  every flagged device in the command's scope, whatever byte 4; seen
  2026-11-15 before a room mode command (four `r:01`, then the 0x40).
  It never resets on its own, and never for 0x40.
- **Slots:** a radiogram for a device that does not answer holds its slot
  without radio time until the device answers or the Cube restarts.
  One Cube held 49 of 50 from February until its restart on 2026-11-14,
  shown nowhere but in its greetings; with one slot left it accepts one
  more radiogram, and if that one waits too, every command after it is
  discarded.
- **The names cross:** the original reads 0x08 as its radio error
  (`RadioState`) and names 0x40 `transmitError` (parameter `unreachable`);
  Massimo's badge says "transmit error" for 0x08.

The code:

- `internal/protocol/cube/devicelist.go`, `decodeDeviceState`: byte 4
  "unknown"; `DeviceState` has `Error` (0x08) and `LinkError` (0x40).
- `internal/engine/queue.go`: `deviceInError` is `Error || LinkError`;
  an `r:` goes only to `cmd.Target`, before the lines and once more after a
  discard; a room setpoint (`internal/server/server.go`, `setpoint`) has
  no target, so it gets none.
- `internal/server/ui/tiles.js`: `badge.transmitError` (tone bad) and
  `badge.linkError` (warn); `log.js`: `log.error`, `log.linkError`;
  the texts in the six `ui/i18n/*.json`.
- `internal/server/logalerts.go`: the alerts on a device's flags.
- `docs/api-v1.md`: "`error` is the Cube's transmit error flag,
  `link_error` a failed link with a partner device"; `docs/ui.md`.
- **AP LegacyLogs**, Block 15 (room commands "not confirmed" per member,
  D135-B), waits for D110: Block 2 here.
- Commands from clients behind the proxy pass through unchanged
  (`Passthrough`): the original software resets for itself; Home Assistant
  goes through Massimo's API and gets the queue's resets.

## Implementation Plan

### Block 1: Byte 4, the failed radiogram

1. `internal/protocol/cube`: the radiogram types (`FrameType` codes and
   names of the original's jar, 0x00 … 0xFF) as a Go type with a name for
   the log; `DeviceState.FailedRadiogram`, set from byte 4 only with 0x08;
   the decoder's comment for offset 4.
2. The device state's JSON gains `failed_radiogram` (omitted without the
   flag); the OpenAPI file and `docs/api-v1.md`.
3. The log's state change for the flag carries the type
   ("communication error: time information" once Block 3 renames it).
4. Tests: entries captured on hardware with 0x03, 0x21, 0xF1, and 0x09
   without the flag; an unknown code (0xf6) kept as a number.

- **Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `TestOpenAPICoversRoutes`, `gofmt -l .`, `go vet` on three platforms,
  `GUIDELINES/shared/tools/check`.

### Block 2: Room commands reset their flagged members first

1. `internal/engine/queue.go`: before a room command's radio lines, one
   `r:01,<address>` per member of the room whose 0x08 is set, the members
   as the Cube lists them (metadata, and a C line naming the room);
   after a discard, the same once more and one retry.
2. `deviceInError` reads 0x08 only (D111-A).
3. Each reset logged at Info with the device and the room.
4. Tests against the fake Cube, the exact lines: a room setpoint with two
   flagged members (two `r:01`, then the `s:`); a member with 0x40 only
   (no `r:`); a device command to a device with 0x40 only (no `r:`).
5. `docs/cube-protocol.md` and `docs/api-v1.md`: what Massimo sends.

- **Verification:** the Go gate, `make race`, `make cross`, the lint
  commands, `GUIDELINES/shared/tools/check`.

### Block 3: "Communication error"

1. UI: the badge for 0x08 reads "communication error", its title the
   failed radiogram from Block 1 ("the Cube's time push did not reach
   it"); the badge for 0x40 keeps "link error", its title the original's
   sentence. The log's texts the same.
2. The six `ui/i18n/<lang>.json` (FR, IT, NL, PL machine-translated and
   marked); `docs/ui.md`, `docs/api-v1.md` (the fields' meaning, names
   unchanged), `README.md` where it names the flag.
3. `tools/ui-test/`: the badge and its title at 1100 and 390 px.

- **Verification:** `make ui-test`, the Go gate, `GUIDELINES/shared/tools/check`.

### Block 4: A flagged device that reports is reset (D102-B)

1. `serve`: when a device whose 0x08 is set reports (a new valid entry
   with its values changed), the engine queues one `r:01` for it, at most
   once a day per device (kept in memory; a restart of `serve` may reset
   one more), logged at Info.
2. Not while the device's room has a command of Massimo in flight.
3. Tests against the fake Cube: a report → one `r:`; a second report the
   same day → none.

- **Verification:** the Go gate, `make race`, `GUIDELINES/shared/tools/check`.

### Block 5: A Cube whose slots stay full

1. `serve`: a Cube whose free slots are 1 or 0 at two readings (greeting
   or `S:` reply) an hour or more apart, with no command of Massimo in
   flight, is logged once at Warn and raised as an alert;
   back above 1, once at Info.
2. The Cube's panel says so: the slots held for devices that do not
   answer, and that a restart of the Cube (its Reboot, owner-only)
   drops them.
3. Six languages; `docs/ui.md`, `docs/api-v1.md` where the alert is listed.
4. Tests: the fake Cube with a greeting of 1 free, read twice.

- **Verification:** the Go gate, `make ui-test`, `GUIDELINES/shared/tools/check`.

### Block 6: A flag that keeps coming back

1. A device flagged again within a day of a reset (Block 2 or 4) is
   logged once a day at Warn, with the failed radiogram;
   Block 4 leaves it alone until it stays clear for a day.
2. A device whose 0x40 lasts a day is named the same way (D157-A).
3. Tests with the fake Cube.

- **Verification:** the Go gate, `GUIDELINES/shared/tools/check`.

Each block brings its `CHANGELOG.d/` fragment and the roadmap's pruning.

## Proposed commit message

For Block 1:

```
feat(cube/errors): The failed radiogram of a communication error

Byte 4 of an L entry is decoded as the type of the Cube's last radiogram to the device that failed, read only with the error flag, logged with it and given in the device's state (AP RadioHealth v1.0, Block 1).

Assisted-by: claude-code:claude-opus-5-5
```

For Block 2:

```
fix(engine/queue): Room commands reset their flagged members first

The queue sends one r: for every member of the room whose error flag is set before a room command, as the original software does, and no longer resets for the link error alone (AP RadioHealth v1.0, Block 2).

Assisted-by: claude-code:claude-opus-5-5
```

Blocks 3 to 6 follow the same form, each with its scope:
`feat(ui/errors)`, `feat(serve/errors)`, `feat(serve/slots)`,
`feat(serve/errors)`.

## Change History

- v1.0 (2026-11-15): first revision; D112-A, D116-A, D153-A, D154-A, D156-A
  settled; D102, D110, D111, D157 open.
