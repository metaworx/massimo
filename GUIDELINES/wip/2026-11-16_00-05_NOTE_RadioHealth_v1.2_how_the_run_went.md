# NOTE RadioHealth v1.2: how the run went

How AP RadioHealth v1.2
(`wip/2026-11-15_12-58_AP_RadioHealth_communication_errors_and_resets.md`)
was carried out on 2026-11-15 and 16, under the owner's `EXEC RadioHealth
1-2`, "the entire AP is due" and "Use sub agents", where the work went
differently from the plan's words. Its decisions are in four NOTEs beside
it:
`wip/2026-11-15_21-43_NOTE_RadioHealth_v1.2_open_decisions_of_the_run.md`,
`wip/2026-11-15_22-04_NOTE_RadioHealth_v1.2_owner_decisions_of_the_run.md`,
`wip/2026-11-15_23-02_NOTE_RadioHealth_v1.2_decisions_of_blocks_5_and_7.md`,
`wip/2026-11-15_23-04_NOTE_RadioHealth_v1.2_owner_decisions_of_blocks_5_to_7.md`.

## The run

- **Sub-agents** built the blocks in worktrees of their own, a common brief
  binding them (no hardware, placeholders only, signed commits gated on
  the checks' exit codes, only the touched UI test files locally); the
  session reviewed each branch, merged main into it, ran the checks again
  and opened the pull requests: #309 (Blocks 1, 3), #311 (2, 4), #312
  (10 to 12), #313 (9), #315 (6), #317 (5, 7), all merged, and #318 (the
  owner's decisions of the run: D180-B, D181, D182-A, D183-B, D185, D188).
- **Not built:** Block 8, which builds on AP RoomConfirm's Block 1
  (`wip/2026-11-15_20-17_AP_RoomConfirm_writes_confirmed_by_the_devices.md`,
  #316 open); Block 13, which waits for Block 11's re-link on the spare
  Cube and on hardware. Nothing was sent to a Cube in this run; the
  spare-Cube runs of Block 11 item 4 and Block 12 are still to come,
  under a gate of their own.
- **Decision numbers:** D175 to D179 and D195 to D198 were taken by other
  plans while this run named its own; its numbers were renumbered to D180
  and up.
- **The review** found one bug: Block 4 sent an automatic reset before the
  store's log of earlier resets was read, after a restart; fixed before
  its pull request, with a test.
- **The merges** found one interplay: Block 9's drift test read Block 4's
  automatic reset of its flagged member, an `r:`, as a correction; the
  test now looks for `s:` lines only.
- **Flakes seen under `-race` at a load of about 10**, each passing on a
  rerun: `TestVirtualWallThermostatEvents` once; one whole-package run of
  `internal/server` whose failing test was not recorded.

## Where the blocks differ from the plan's words

- **Block 1:** the types are `cube.RadiogramType` (`radiogram.go`),
  tested against the list in `docs/cube-protocol.md`; the `RF*`
  constants are typed from it. `failed_radiogram` also reaches the
  virtual wall thermostat's state, the state's times, the process log,
  the flag alert and `massimo status` and `watch`. The proxy's virtual
  entries never set byte 4; a test holds them byte for byte.
- **Block 2:** a command's rooms are found from its room-flagged `s:`
  lines; `cube.ParseResetError` skips the devices its own `r:` lines
  reset. A device setpoint, addressed as its room, resets the room's
  flagged members too (D110-A). A discarded line with nothing reset is
  not retried.
- **Block 3:** the badge's key is `badge.communicationError`; the 0x08
  badge's title is a sentence of Massimo's, then the failed radiogram;
  problem badges carry titles. The dated findings of
  `docs/cube-protocol.md` keep "transmit error".
- **Block 4:** a report is a change of a value the device sends itself;
  a reset counts once the Cube took it; the new kind `error_reset`;
  `Queue.Concerns` leaves a device alone while a command to it or its
  room waits or runs.
- **Block 5:** a reading counts when no `s:` line was accepted in the 15
  minutes before it; S replies are read through a wrapper of the queue's
  gateway; no reconnect of its own (D192-A); thresholds below 5 and below
  2 (D192-A); the API's `slots_low`; each `slots` event settles the one
  before it.
- **Block 6:** one kind, `error_watch`, with `what` back, lasting or
  clear. "Back" is an entry read after the Cube answered the reset; the
  timers follow the notify engine's pattern, not its code; no Error while
  the Cube is offline; the clear settles the Warn and the Error (D189-A).
  A device without a valid state since the Cube's restart starts its 24
  hours again at a `serve` restart (D191-A).
- **Block 7:** the kind `mismatch` and the field `mismatches`; a room
  master is checked only in a room with a thermostat; "unread" is no
  user's `log.seen` over the newest warning; also left out: a restore,
  and a room whose member's configuration write is still followed.
- **Block 9:** the re-send at the check 10 minutes after the report;
  Massimo's last setpoint kept in memory; a `not_resent` line; members
  each on their own programme agree (their C lines are Block 7's); a
  drifting member is never corrected; a room with an open window is left
  out; `Queue.Pending` and `Queue.Holding`; the day's limits seeded from
  the log.
- **Blocks 10 to 12:** migration 0022 gives `repair` to `integration`
  too, its ceiling; each repair has a preview (`GET`) and a run
  (`POST ?plan=<hash>`); a run is refused when its cost would reach the
  duty hold or leave fewer than 5 free slots; the re-link takes the
  linkable members only; "unlink all" sends no 0x23.
- **The decisions' follow-ups (#318):** D182-A covers the copy's 0x40
  step too; D183-B lets an `r:`-only command pass the duty hold as well
  (the owner accepted it); D180-B notifies once, at the first flag; D181
  ranks the radiograms by the original software's categories; D185 takes
  the lowest free n and cuts the room's name past 32 bytes; D188 keeps
  "a thermostat still shows it" for Massimo's setpoint.

## Open decisions (each written as recommended)

- **D199. One lost room command, two re-sends:** Block 8 re-sends an
  unconfirmed room command at about 5 minutes, Block 9 a disagreeing
  room's state at about 20 (the RoomConfirm session's note, 2026-11-15).
  - **A (recommended):** Block 9 leaves a room alone while it has an
    open or unconfirmed confirmation, and the two share one re-send a
    day per room.
  - **B:** Block 8 uses Block 9's re-send and its limit.
- **D200. Pairing's default name:** pairing starts with an empty name.
  - **A (recommended):** the same "<room> <type> <n>", prefilled.
  - **B:** leave it.
- **D201. A default name past 32 bytes** ("Wohnzimmer
  Heizkörperthermostat 1" is 34).
  - **A (recommended):** cut the room's name, as built.
  - **B:** "<type> <n>" alone.
  - **C:** shorter type words in names.
- **D202. A second flag set while the first stays set** is not notified.
  - **A (recommended):** each flag's start notified, within the half-hour
    limit.
  - **B:** one notification, as built.
- **D203. D188's first step keeps "while a thermostat still shows it".**
  - **A (recommended):** keep it, so that a setpoint nobody shows any more
    does not override later changes.
  - **B:** drop it.
- **D204. A repair's preview counts only its own resets,** not the
  queue's (Block 2, D182-A); an `r:` costs nothing.
  - **A (recommended):** count them too, the engine giving the prelude's
    list, a small follow-up.
  - **B:** leave it.
