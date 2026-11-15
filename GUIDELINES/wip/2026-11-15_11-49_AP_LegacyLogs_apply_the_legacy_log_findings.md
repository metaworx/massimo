# AP LegacyLogs v1.0: Apply the legacy log findings

## TL;DR

Massimo takes over what the original MAX! software 1.4.6 and two
hardware tests showed about the Cube; each change brings its own docs.

- docs-only corrections → Block 1
- week programme: second telegram 0x08 → 0x10, padded (T130)
- move out: 0x23 last → first; anchor links only if T134 shows them
  reaching every member
- room command: M line's address → fallback to a radiator thermostat
- budget: no free slot → 50 pending; duty hold until the hour → steps,
  set in Maintenance
- stale: 24 h → 50 h radiator, 24 h wall
- battery: flag → amber `battery_weak`, held 15 min → red `battery_low`
- out-of-date entries: dropped → last values, marked
- found Cube in another network → hint; wall thermostat back on
  another mode → Warn
- room commands "not confirmed" per member, after D110

### Blocks

1. Docs corrections, with the code comments that say the same
2. Week programme: the second telegram 0x10, padded
3. Move out: 0x23 first
4. T134: one room-flagged link to an anchor, .25 room 1, then room 5
5. Anchor links, only if T134 shows them reaching every member
6. Room command: fallback to a radiator thermostat of the room
7. Names, and the `m:` index in decimal
8. Budget: no free slot is 50 pending
9. Duty hold: re-read in steps, the step in Maintenance
10. Stale: 50 h for radiator, 24 h for wall thermostats
11. Battery: amber and red, two notification kinds
12. A wall thermostat back on another mode: a Warn
13. Out-of-date entries keep their last values, marked
14. A found Cube in another network says so
15. Room commands confirmed per member, after D110

## Discussion

The owner's brief of 2026-11-14 asked for an analysis of the original
software's logs of two hosts (2026-02-12 to 03-05 and 05-05, and
2026-11-12 to 14) and of its jars, against Massimo's code and
`docs/cube-protocol.md`, without sending anything to a Cube.
It was followed by two hardware tests at the owner's word:

- **T130** (`EXEC`, 2026-11-15): a day of 8 switch points in the
  original's form;
- **T131** (`EXEC t131`): the owner took the batteries out of two wall
  thermostats.

Then came `PLAN` twice, the owner's answers, and
`PLAN++ as recommended`.

**The owner's decisions (2026-11-15):**

- **D134-A:** no free slot reads as 50 pending, once the budget is known.
- **D135-B:** a room command stays "not confirmed" until each member
  shows the setpoint, built here once D110 is on main.
- **D136-A:** 50 h for radiator thermostats, 24 h for wall thermostats,
  both settable.
- **D137-C**, the owner's two levels:
  - amber when the battery flag shows at all;
  - red when it is held;
  - amber clears when a long valve move passes without the flag.

  Worked out in the analysis of 04-58, section 2.
- **D138-A:** 0x23 first. One link per radiator thermostat until a test
  shows one anchor reaches them all. "PLAN a test after this": T134.
- **D139-A:** the original's second telegram; the fake Cube reads 0x10.
- **D140-A:** an out-of-date entry shows its last values, marked, with
  their age.
- **D141-A, the docs, as the owner then set it:**
  - every block updates the docs of its own change;
  - the docs-only corrections are Block 1;
  - general overview changes come at the end, best avoided. None is
    foreseen.
- **D142-A:**
  - comments with Block 1;
  - names and the `m:` index in Block 7;
  - the room-command fallback (G12) on its own, with its test.
- **D143-A**, with the step adjustable in System settings.
- **D147-A:** a Warn when a wall thermostat comes back on another mode.
- **D148-A:** a found Cube in another network says so, with the
  `netconfig` line.
- **D149-A+B:**
  - one notification when a device turns amber and one when it turns red;
  - amber has a kind of its own, `battery_weak`, beside `battery_low`.
- **D150, both rooms of .25:** room 1 (four radiator thermostats) first,
  then room 5 (two).
- **D151-A:** Block 15 stays the last block. Once every other block is
  merged, the gate offers to move it into a plan of its own.
- **D152-A:** `battery_weak` is on by default, as `battery_low` is.

**The transmit-error session.** The session "Investigate the MAX! Cube's
transmit error on hardware" owns D100 to D117.
- D102 and D110 are undecided and unplanned (its answer of 2026-11-15).
- Its docs PR is still to come. It covers four paragraphs of
  `docs/cube-protocol.md`:
  - the transmit-error entry in "Lessons from the hardware days";
  - the byte-4 sentence in "Observed polling";
  - the FrameType paragraph in "Pairing a device, as the original
    software does it";
  - the `r:` rows of "LAN commands verified against the original
    software".

This plan keeps off those four paragraphs.
So the naming of byte 5's bits (K4, N2) and the 0x81 attribution stay
with that session. The four overlaps were sent to it on the owner's
`EXEC tell`.

**Hardware.** Nothing goes to a Cube without the owner's go.
T134 runs under a gate of its own (`EXEC t134`), with the owner at the
room's wall thermostat.

**Still open, outside the blocks:**
- T132 (whether a radiator thermostat's daily report follows its
  decalcification hour) was not run.
- T133 (a second look at breno's duty readings and battery episodes,
  around 2026-11-22) is not scheduled.

## Analysis

The plan rests on four analyses in `wip/`:

- `2026-11-14_21-13_ANALYSIS_LegacyLogs_protocol_findings.md`:
  - what the docs get wrong (K1–K8);
  - new protocol facts (N1–N8);
  - gaps in the code, with locations (G1–G13);
  - the overlap with D102 (section 6);
  - proposed docs text (section 7).
- `2026-11-15_01-41_ANALYSIS_LegacyLogs_week_programme_test.md`: T130.
  The original's form, the second telegram marked 0x10 and padded to 6,
  held all 8 points within 17 s.
- `2026-11-15_03-24_ANALYSIS_LegacyLogs_valid_bit_test.md`: T131.
  - A silent wall thermostat loses the valid bit after 4¾ to 5¾ minutes,
    and gets it back within a minute.
  - Powered up again, it comes back in auto.
  - Its section 9 correction is in the analysis of 04-58.
- `2026-11-15_04-58_ANALYSIS_LegacyLogs_owner_answers.md`:
  - the battery episodes on breno and in February: no device was seen
    going from flickering to steady;
  - the decalcification as the week's long valve move;
  - T134's steps and cost;
  - the discovery hint.

**The code, on main of 2026-11-15:**

| Block | Where | Today |
|---|---|---|
| 2 | `internal/protocol/cube/commands.go:291` (`CmdSetDayProgram`), `internal/cubelan/fakecube/apply.go:243` | second telegram `day\|0x08`, not padded; the fake reads 0x08 |
| 3 | `internal/cubeio/move.go:56` (`MoveCommands`) | 0x23 after the 0x21s |
| 5 | `internal/cubeio/move.go:30-34` (`linkTarget`) | one room-flagged link per radiator thermostat |
| 6 | `internal/server/server.go:1823` | the room setpoint to the M line's room address, no fallback |
| 7 | `internal/protocol/cube/devicelist.go:15, 21, 73`, `metadata.go:28, 35, 192` | `IsAnswer`, `DST`, `Trailing`; the `m:` block index in hex |
| 8 | `internal/cubelan/budget.go:36-39` (`Pending`) | 0 when no slot is free |
| 9 | `internal/engine/queue.go:517-530` (`waitForBudget`) | holds until the next full hour |
| 10 | `internal/view/view.go:192` (`StaleAfter`) | 24 h for every thermostat; `stale_after` through the API only |
| 11 | `internal/notify/rules.go:122`, `internal/server/ui/tiles.js:404` | a notification at every off-to-on edge; one amber badge |
| 13 | `internal/protocol/cube/devicelist.go:86-88` | an out-of-date entry's values dropped |
| 14 | discovery, `connect.unreachable` | a found Cube that refuses TCP gets no reason |

## Implementation Plan

Every commit cites `AP LegacyLogs v1.0, Block <n>`.
Blocks 1 to 3 run on `claude/legacy-logs`, branched from main after this
plan's commit.
CHANGELOG fragments are named `CHANGELOG.d/<Heading>/claude-legacy-logs-<what>.md`.
Device names, serials and radio addresses appear in nothing written.

### Block 1: Docs corrections

The docs that are wrong today, and the code comments that say the same.
No behaviour changes. The text is the analysis of 21-13, section 7,
less what later blocks bring and less the transmit-error session's
paragraphs.

- `docs/cube-protocol.md`:
  - **K1, set temperature (0x40):** the summary line, the paragraph after
    a 0x40, and the C-line note. A member shows the setpoint when it
    acknowledges, within 1 to 19 s; a member that does not answer keeps
    the old one; `S:` says the Cube took the command, not the room.
  - **K3:** the idle disconnect counts from the UI's last call, and the
    keepalive is `e:something`.
  - **K8:** the `z:` duration's unit is open; the original names it
    `durationInMinutes`, and wakes rooms only before configuration
    writes.
  - **K6, a new section, "The H line":** the table of fields.
  - **Device reports (N3, K2):**
    - how often each type reports;
    - an idle radiator thermostat once a day, not on its
      decalcification day;
    - a radiator thermostat's measured temperature stays in L for 10 s
      to 13 min.
  - **Costs (N5)**, without the `r:` row.
  - **K5:** byte 6's 0x08 is automatic DST switching, not summer time.
  - **K7, in "LAN commands verified against the original software",**
    outside the `r:` rows: `setCubeTime` sends `v:<rules>,<seconds in hex>`.
- `docs/api-v1.md`:
  - **K1:** the setpoint shows once each member acknowledges it.
  - **K2:** a radiator thermostat's measured temperature comes once a
    day in manual mode, at the switch points in auto.
- Comments:
  - `internal/cubelan/times.go` (K1);
  - `internal/protocol/cube/devicelist.go` (K2, K5);
  - `internal/view/view.go` (K2);
  - `internal/protocol/cube/commands.go` (K7);
  - `internal/protocol/cube/metadata.go`: the trailing byte is the DST
    flag, and the room address is the room's master radiator thermostat.
- CHANGELOG fragment `Changed`, for the comments under `internal/` and
  the API document.

**Verification:**
- `MASSIMO_ENV_FILE=/dev/null go test ./...`, the identifiers test among
  them;
- `gofmt -l .`;
- `go vet` for linux, windows and darwin;
- `make cross`;
- `GUIDELINES/shared/tools/check`.

### Block 2: Week programme

- `internal/protocol/cube/commands.go`, `CmdSetDayProgram`: the second
  telegram's day byte gets bit 4 (0x10), and its points are padded to 6
  with the last one.
  - As `SetRoomWeekTemperatureProfile.createDayPartSpec` of 1.4.6 does.
  - Verified on hardware 2026-11-15 (T130).
- `internal/cubelan/fakecube/apply.go`: the fake reads 0x10 as the second
  part (D139-A); a padded point past 24:00 changes nothing.
- Tests with the exact lines:
  - `internal/protocol/cube/cube_test.go`: an 8-point and a 13-point day;
  - `internal/cubelan/fakecube/apply_test.go`: a long day read back
    through the fake;
  - every other test that builds a long day.
- `docs/cube-protocol.md`, "LAN commands verified against the original
  software": T130's paragraph (the analysis of 01-41, "For
  docs/cube-protocol.md").
- CHANGELOG fragment `Fixed`.

**Verification:** as Block 1, plus a grep for `0x08` in the day-program
paths.

### Block 3: Move out

- `internal/cubeio/move.go`, `MoveCommands`: the old room's 0x23 (remove
  group) goes first, then the 0x21s in both directions, as the capture
  in `docs/cube-protocol.md` and `DisassociateDeviceFromRoom` have it.
- The tests that check a move's lines exactly: `internal/cubeio`, and
  the server's room and device tests.
- `docs/cube-protocol.md` and `docs/api-v1.md`, where they describe
  Massimo's order.
- CHANGELOG fragment `Fixed`.

**Verification:** as Block 1.

### Block 4: T134

The test of the analysis of 04-58, section 3, in .25's room 1 first, then
in room 5.

- A script, `GUIDELINES/temp/legacy/t134_links.py`, on the pattern of
  `t130_week.py`:
  - it reads the proxy's greeting and checks the guards: every member
    valid without the error flag, the duty cycle at most 60 %, at least
    10 slots free;
  - it sends the steps out, in and repair one at a time, printing the
    lines with the addresses masked;
  - it reads serve's store for whether each radiator thermostat
    followed the wall thermostat.
- A TL;DR gate per room, giving:
  - the members by type;
  - the lines;
  - the cost (about 5 to 10 % on .25);
  - `EXEC t134` and the owner's three turns of the wall thermostat's wheel.
- Result: an analysis in `wip/`, and the finding in
  `docs/cube-protocol.md`, in a `docs(cube)` commit.

**Verification:**
- each radiator thermostat followed in the baseline and after "in";
- none followed after "out";
- a member that missed a step is repaired with today's per-member form.

### Block 5: Anchor links

Only if T134 shows the room flag reaching every member. Otherwise a NOTE
drops the block and the docs say why.

- `internal/cubeio/move.go`:
  - one room-flagged 0x20 or 0x21 to the anchor: the first other
    radiator thermostat, else the wall thermostat;
  - a direct one to each shutter contact and to the moved device for
    each member, as `AssociateNewDeviceToRoom` and
    `DisassociateDeviceFromRoom` send them.
- Tests with the exact lines; docs; CHANGELOG fragment `Changed`.

**Verification:** as Block 1.

### Block 6: Room command fallback

- `internal/server/server.go`, the room setpoint and mode: the M line's
  room address where it names a radiator thermostat of the room, else
  the room's first radiator thermostat (metadata order), else its wall
  thermostat, as the original's master and first radiator thermostat.
- Tests with the exact line sent to the fake Cube, with a room whose
  address is 0 or names a device of another room.
- `docs/api-v1.md`; CHANGELOG fragment `Fixed`.

**Verification:** as Block 1.

### Block 7: Names and the `m:` index

- `internal/protocol/cube`:
  - **devicelist.go:** `IsAnswer` becomes a name for byte 5's 0x04,
    "unknown, seen only with the error flag", and `DST` becomes
    automatic DST switching.
  - **metadata.go:** `Trailing` becomes the DST flag, and the room
    address field names the master radiator thermostat.
  - **`Hello`:** fields 9 and 10 (`stateCubeTime`, `ntpCounter`).
  - The API is unchanged.
- `metadata.go:192`:
  - the `m:` block index is written in decimal, as the original writes
    it;
  - the two forms differ from the eleventh block on;
  - a test with eleven blocks.
- Two commits: the names (`refactor`, no bullet, said in Key changes),
  and the index (`fix`, fragment `Fixed`).

**Verification:** as Block 1.

### Block 8: Budget

- `internal/cubelan/budget.go`, `Pending`: no free slot reads as 50
  pending, once a greeting or an `S:` gave the numbers. Only the
  portal's short H counts as unknown.
- Tests; `docs/api-v1.md` (`pending_radiograms`); CHANGELOG fragment
  `Fixed`.

**Verification:** as Block 1.

### Block 9: Duty hold

- `internal/engine/queue.go`, `waitForBudget`: after a duty-cycle hold,
  one command goes after each step (default 10 min) to refresh the
  numbers, as `SlotWait` does for memory slots. It no longer waits for
  the next full hour. `MaxDutyPct` (95) stays.
- The step is a store setting beside CubePoll's intervals:
  - System settings → Maintenance, "Re-read the radio budget after a
    hold every", with a line on what it does;
  - steps 5, 10, 15, 30 and 60 minutes;
  - applied at the next restart;
  - `fixed_by` where a flag decides;
  - the settings API.
- `internal/cubelan/budget.go`'s comment: probably a rolling 60 minutes
  (N6).
- Tests:
  - the queue with an injected clock;
  - the settings API;
  - `tools/ui-test` for the Maintenance row.
- Docs:
  - `docs/cube-protocol.md`: N6, the duty cycle falling within the hour;
  - `docs/api-v1.md` and the OpenAPI file;
  - `docs/ui.md`.
- i18n in all six languages; CHANGELOG fragment `Changed`.

**Verification:** as Block 1, plus `make ui-test` and `make race`.

### Block 10: Stale

- `internal/view`: the stale threshold by type.
  - `stale_after` becomes the radiator thermostats', default 50 h.
  - A new `stale_after_wall` keeps the wall thermostats' 24 h.
  - Both through the settings API, as `stale_after` is today.
  - `-stale-after` sets both, and `fixed_by` names it for both.
- Tests in `internal/view` and the settings API.
- `docs/api-v1.md` and the OpenAPI file: why an idle radiator
  thermostat changes only every 24 to 48 h (K2).
- CHANGELOG fragment `Changed`.

**Verification:** as Block 1.

### Block 11: Battery

- **A battery level per device: ok, weak or low.** It is kept by serve
  from the state events, and rebuilt at the start from the log's state
  rows (30 days by default, no migration).
  - **Weak:** the flag seen at all.
  - **Low:** the flag held for 15 minutes.
  - **Weak clears** after the device's next decalcification passes
    without the flag (its day and hour from its C line, 15 minutes after
    the hour), or after a valve move of 50 points or more with no flag
    within 10 minutes. A wall thermostat clears after 24 hours without
    the flag.
  - **Low clears** to ok after an hour without the flag.
- **API:** `battery: "ok" | "weak" | "low"` beside `battery_low`, which
  stays the bit.
- **Notifications:**
  - a new kind `battery_weak`, on by default (D152-A), sent when a device
    turns weak;
  - `battery_low`, sent when it turns low;
  - each once per level, nothing while the flag flickers;
  - the notification settings get a row and its translations.
- **UI:**
  - the tile's battery badge amber (`tone-warn`) for weak, red
    (`tone-bad`) for low;
  - `badge.batteryWeak` and the notification texts in all six languages.
- **Tests:**
  - the level rules with an injected clock, flickers and decalcification
    included;
  - the rebuild from the log;
  - the notification rules;
  - `tools/ui-test`.
- **Docs:**
  - `docs/cube-protocol.md`: the flickers, and the decalcification
    showing them (the analysis of 04-58, section 2);
  - `docs/api-v1.md` and the OpenAPI file;
  - `docs/ui.md`.
- CHANGELOG fragments `Added` (`battery_weak`) and `Changed`
  (`battery_low` at red).

**Verification:** as Block 1, plus `make ui-test` and `make race`.

### Block 12: A wall thermostat back on another mode

- serve's state log: when a wall thermostat regains its valid state after
  5 minutes or more without it, and its mode differs from its last valid
  one, a Warn names the old and the new mode and setpoint.
- A test with the fake Cube.
- `docs/cube-protocol.md`: a wall thermostat powered up again starts in
  auto on its programme (T131).
- CHANGELOG fragment `Added`.

**Verification:** as Block 1.

### Block 13: Out-of-date entries

- `internal/protocol/cube/devicelist.go`: an entry with 0x02 and without
  0x10 keeps its values, marked out of date.
- The view and the API:
  - `reported` stays false;
  - the state comes with `out_of_date: true` and the age Massimo keeps;
  - the OpenAPI file and `docs/api-v1.md`.
- The UI marks the values as out of date, in all six languages
  (`tools/ui-test`).
- `docs/cube-protocol.md`: the valid bit means "heard lately", with
  T131's timing (the analysis of 03-24, "For docs/cube-protocol.md").
- CHANGELOG fragment `Changed`.

**Verification:** as Block 1, plus `make ui-test`.

### Block 14: A found Cube in another network

- **Discovery:** a found Cube whose address lies in none of this host's
  interface networks is marked, in the API, as in another network.
- **The UI says so** where it lists found Cubes and where it says "does
  not answer at its addresses":
  - the Cube's address;
  - that a Cube without a DHCP lease uses 192.168.0.222;
  - the `netconfig` line that sets it;
  - in all six languages.
- **Tests:** the check with injected interface networks, the API,
  `tools/ui-test`.
- **Docs:** `docs/api-v1.md`, the OpenAPI file, `docs/ui.md`.
- CHANGELOG fragment `Added`.
- **Also `make lan-test`,** since the change touches discovery.

**Verification:** as Block 1, plus `make ui-test` and `make lan-test`.

### Block 15: Room commands confirmed per member

Once D110 (the transmit-error session) is on main:
- a room command's result and its room's state say "not confirmed" until
  each member shows the setpoint;
- after two polls without it, a Warn says so, as the original's
  `stateDirty` did.

Its API, docs and tests are written in the revision that opens it.
If it is the last block left, the gate offers to move it into a plan of
its own (D151-A).

**Verification:** as Block 1.

## Proposed commit messages

- `chore(aps): +LegacyLogs v1.0, +ANALYSIS LegacyLogs`: this plan and
  its four analyses, on main.
- Block 1: `docs(cube): A setpoint shows once each device has acknowledged it`
- Block 2: `fix(cube): The week programme's second telegram is marked 0x10 and padded`
- Block 3: `fix(cubeio): Moving a device out removes its group first`
- Block 4: `docs(cube): One room-flagged link and the room's members`
- Block 5: `feat(cubeio): A room link goes to the room's anchor`
- Block 6: `fix(server): A room command falls back to a radiator thermostat of the room`
- Block 7: `refactor(cube): Names for the flags the original software reads`,
  `fix(cube): The metadata write numbers its blocks in decimal`
- Block 8: `fix(cubelan): No free memory slot reads as 50 pending radiograms`
- Block 9: `feat(queue): A duty-cycle hold re-reads the budget in steps`
- Block 10: `feat(view): Radiator thermostats count as stale after 50 hours`
- Block 11: `feat(notify): A weak battery and a low one`
- Block 12: `feat(serve): A wall thermostat back on another mode is a warning`
- Block 13: `feat(view): An out-of-date entry keeps its last values`
- Block 14: `feat(discovery): A found Cube in another network says so`
- Block 15: in the revision that opens it.

## Change History

- v1.0 (2026-11-15): first revision, by `PLAN++` after `PLAN` and the
  owner's decisions D134 to D152.
