# NOTE RoomMajority v1.1: how blocks 1 to 3 went

Where Blocks 1 to 3 went differently from the plan, and why;
how the run was done, what verified it, and what it leaves for the owner.
Built on 2026-11-23 and 24 on the branch `claude/room-majority`,
a commit per block, the three one pull request:
"fix(rooms): One order speaks for a room,
and a re-send keeps its state from before a newcomer"
(Block 1),
"fix(copy): A thermostat put into a room takes the room's state and configuration"
(Block 2) and
"feat(ui/rooms): OFF for 4.5 °C, marks for eco and comfort,
and the setpoints of members that differ"
(Block 3).
Dead, below, is D9's: a thermostat the Cube holds no valid state for; live, the others.

## How it ran

- **A builder per block:** a sub-agent on Opus 5.5 in the session's worktree,
  without a commit; the coordinating session ran the checks and made the signed commit.
- **A second reader per block:** an independent sub-agent on Opus,
  changing nothing in the tree, its probes and faults through `go test -overlay`
  or scratch.
  Two rounds per block, and for Block 2 a first of the coordinating session's own;
  each round's findings fixed by the builder before the next,
  the main ones named under each block below.
- **Planted faults:** every new test shown failing on a fault planted for it,
  one at a time, the files restored and compared after, the plan's own among them.
  The readers planted their own; each that no test caught got a test.
- **Nothing went to a real Cube.**

## Block 1, Order

Different from the plan:

- **Equal newest times:** where the members with the newest change show one state,
  that state wins; the Analysis had two equal times fall to the order.
- **When a hold ends,** beyond the Analysis' agreement, 24 h and a later change:
  agreement counts only where two or more thermostats, or every one of the room,
  were compared;
  a change by hand to another state after the first check that judged the room
  ends it, as do a newer setpoint of Massimo's other than the held state
  and a held holiday's until, passed on the Cube's clock.
- **A later structure change** reuses a hold only while no check has judged the room,
  and never refreshes its time; the Analysis had it hold its own.
- **One re-send per hold** goes out past the day's limit (`why: today`).
- **A held auto with a temperature** becomes auto on the programme
  once a switch point has passed, RadioHealth.D188's rule for Massimo's own command.
- **What counts as a change:** `cubelan.StateTimes.Changes`, new:
  a first sighting, a boost's end back to its earlier setpoint
  and a holiday's end at its until (the Cube's clock, 30 min grace) are no one's.
  It takes in the Analysis' rule of a time equal to `snap.Since()`;
  `Snapshot.CubeClock` moved from the server into `cubelan` for it.
- **`no_state`** can no longer happen; its texts stay for older log lines,
  and the docs say so.
- **The held line** of the log names the rule that took the state, in the past
  (`log.disagree.before.<from>`).

The readings:

- **First:** a hold could go stale and win over a later hand change;
  a first sighting counted as the newest change, the 2026-11-23 pattern again;
  a check that scanned before a hold could drop it;
  the day's limit blocked a held state; a held holiday went out past its until.
- **Second:** a new test failed under `-race`, the test's fault;
  a hold kept for hours still beat a later hand change;
  a holiday's end was read in the host's time zone;
  a boost left by hand counted as no one's change.

## Block 2, Copy

Different from the plan:

- **`RoomConfig`** takes the members, the master and the device to leave out
  from its caller, as `RoomOrder` does.
- **The copy's 0x11** goes to the device and the room's live thermostats,
  not to every member.
- **A strict dead rule:** where every other thermostat is dead,
  the room has no configuration:
  the copy sends the 0x40 alone, the repair refuses (`NoMajorityError`, the dead counted),
  the report keeps its findings.
- **The window-open minutes** are decided on their own among the radiator thermostats;
  the repair keeps the device's own, the copy sends the room's.
- **The report** decides one room-level configuration, not each member against the others,
  and judges only live members, in a room with two or more.
  It judges nothing in a room while one of its thermostats has not reported
  since the Cube's restart, and passes over one out of date;
  the findings of those not judged stay,
  so that a restart clears nothing and warns of nothing again.
  It also runs when the Cube gains or loses a device's state (`reported`).
- **The repair** takes the report's reference, the device's own configuration counted,
  and refuses only a room without another live thermostat (D20).
- **The hold:** for thermostats only, also with `"copy": false`;
  `roomReference` leaves the moving device out where its C line names the room already.
  It is taken only once a link radiogram may have reached the room:
  `cubeio.Reached` over `engine.Result.Sent`, both new,
  a request whose time ran out counting as reached.
  Where none did, nothing is held and an earlier hold stays;
  the plan's drop on a failed link (`dropHold`) went.
- **A newer setpoint of Massimo's:** a state read before one of another state
  is not held, and the copy sends Massimo's in its place (D8-A).
- **A discarded D16-A room command** fails nothing:
  the PUT still answers 200, without `room_mode`.
- **The PUT's answer** always carries `copied`: the state sent and where from
  (`state`, `state_from`), why no configuration went (`config_skipped`), `room_mode`;
  the put form shows them, `help.putCopy` and `put.modeNotSet` rewritten.
- **Gone from `cubeio`'s repair:** `Agreement`, `majority`, `sameWeek`;
  `AddDevice` says whether its link half reached the room.
- **Kept by decision:** in a room of three, an out-of-date master can still clear
  and raise its partners' findings;
  a member that never reports after a Cube restart freezes the room's findings (D21).

The readings:

- **The coordinating session's:** a Cube restart cleared and raised the findings again;
  a device whose C line named the room already counted in the room's state.
- **The reader's first:** the copy wrote a minority's values over dead members;
  the report still churned at a restart in rooms of three or more;
  the put form's texts stale in six languages;
  a failed add wiped an earlier hold;
  five planted faults no test caught;
  the report and the repair could name different values.
- **The reader's second:** a room setpoint made during an add was undone by the copy;
  `Reached` said no where the request's time ran out while its links still went out,
  so no hold and no copy, the 2026-11-23 failure again;
  three more planted faults no test caught.

## Block 3, Marks

Different from the plan:

- **OFF in more texts** than D14-A lists:
  the tile's result labels, which lose their unit,
  the put form's copied state, the log's line of a setpoint change.
  The log's configuration lines keep 4.5, as the configuration does.
- **The House tile's writes** hold the members' line too,
  D17-A's "a write to the room".
- **When the members' line waits:**
  on the tile's own record (`ui.awaiting`, 5 min at most),
  since a pending confirmation reaches the page only when a device reports;
  also while a thermostat's mode or setpoint is pending,
  and while a window of the room is open,
  an out-of-date contact by its last report, as the room checks count it.
  AUTO's wait ends only at a state event showing agreeing setpoints taken since the write;
  a boost starts none; only a write the Cube took replaces one.
- **The tile's mark** is a badge at the top right of SET's value, out of the flow,
  after a fixed slot that overflowed 390 px in French, Italian and Polish.
  The value is as wide as 30.5°, and the phone's setpoint row narrower,
  since Italian overflowed it by about 4 px before the block;
  a test now draws the row in all six languages at 390 px.
- **The members** are separated by " · ".
- **The gauge** stays `aria-hidden`; only the mark is labelled.
- **The marks follow a configuration change at once:**
  a config event updates the device's entry,
  and More reads eco and comfort from it (`presetTemps`),
  with a detail request only where the entry has none.

The readings:

- **First:** AUTO's wait ended at once where the thermostats were in auto already,
  so the line flickered;
  the marks kept the old eco and comfort after a configuration change;
  the mark moved the setpoint row under the pointer;
  a failed write ended an earlier wait; a boost waited for nothing.
- **Second:** an unrelated report, a valve or a contact, still ended AUTO's wait;
  the fixed slot overflowed 390 px in three languages the suite does not run;
  the window rule differed from the room checks'.

## What verified it

Each exit 0:

- **Block 1:** `go test` of `internal/cubeio`, `internal/cubelan`, `internal/server`,
  `internal/view`, `internal/engine`, `internal/backup`, `tools/identifiers`;
  `-race` of `internal/server`, its hold tests at `-count=5`;
  `gofmt -l .`, `go vet` for linux, windows and darwin; `make ui-check`;
  `test_log.py` in the four cases, 148 passed; the guidelines check.
- **Block 2:** `go test` of `internal/cubeio`, `internal/cubelan`, `internal/server`,
  `internal/engine`, `internal/view`, `tools/identifiers`, `tools/apitypes`;
  `-race` of `internal/server` and `internal/engine`;
  `gofmt`, `go vet` for the three systems; `make ui-check`;
  `test_pairing.py` and `test_repair.py` in the four cases, 52 passed;
  the guidelines check.
- **Block 3:** `go test` of `internal/server`, `internal/view`, `cmd/massimo`,
  `tools/identifiers`, `tools/apitypes`; `gofmt`, `go vet` for the three systems;
  `make ui-check`; ten UI test files, `test_room_marks.py` new, 536 passed;
  the guidelines check.
- **At each commit,** the coordinating session:
  `make ui`, the block's Go packages, `make ui-check`,
  from Block 2 `GUIDELINES/shared/tools/check`, 12 checks, all well.
- **Left to CI:** the whole Go suite, the race job, the cross builds
  and the whole UI suite.

## Seen and left

- At 360 px, French and Italian still overflow the setpoint row by 2 to 7 px;
  the contract's phone width is 390 px.

## For the owner

- **D19. The other places where one member stands for its room.**
  D9's "the same in every place" may mean them, but no block listed them,
  and each keeps its own order:
  - the room's configuration and programme,
    `GET …/rooms/{id}/config` and `…/program`
    (`getRoomConfig` in `config_api.go`, `programOfRoom` in `etag.go`):
    the first radiator thermostat, else the wall thermostat;
  - the page's `roomThermostat` (`rooms.js`):
    the wall thermostat, else the first radiator thermostat;
    More's presets and the tile's mark read it;
  - the virtual wall thermostat's mode (`internal/view/vwt.go`):
    the wall thermostat's, else the radiator thermostat that reported last.

  Recommended A, so that the tile and More name the thermostat the room checks name.
  - **A.** A small follow-up plan that moves them onto `cubeio.RoomOrder`,
    the dead passed over.
  - **B.** Leave them; D9 then binds the radio's addressee and the references alone.
- **D20. The repair's reference.**
  The configuration repair now takes the report's room-level reference,
  the device's own configuration counted,
  where RadioHealth.D163-A spoke of "the others".
  Recommended A: judged against the others, in a reader's room of four
  the repair set a value the report had not named,
  and the report then named another member.
  - **A.** Keep it, so that the repair's preview names what the report named.
  - **B.** Back to the others, as D163-A says.
- **D21. Findings frozen until every thermostat has reported.**
  The report keeps a room's member findings while one of its thermostats
  has not reported since the Cube's restart;
  a device that never reports, say one without a battery, keeps them frozen.
  Recommended A: what it freezes was found before the restart,
  and a restart no longer clears and raises it again.
  - **A.** Keep it.
  - **B.** A time limit, say an hour after the restart,
    after which a thermostat that has not reported is passed over as out of date.
- **D22. Nothing tried on hardware.**
  The fake Cube cannot show two things:
  the Cube taking radiograms that pass over a dead addressee,
  and a newcomer taking the copy's 0x40 with a manual or holiday state,
  sent direct, without a Target.
  Recommended A; each step costs duty cycle, as every radio write does.
  - **A.** A check on the owner's word, through the proxy of a running `serve`:
    1. in a room of a wall thermostat and a radiator thermostat,
       the wall thermostat's batteries out until the Cube marks it out of date;
       then another radiator thermostat moved into the room,
       `PUT /api/v1/cubes/{sn}/devices/{rf}` with its `room_id`:
       the link radiograms name the live radiator thermostat, and the Cube takes them;
    2. in a room of one radiator thermostat set by hand to manual 5 °C,
       a wall thermostat put into it the same way:
       it shows manual 5 °C, the radiator keeps it,
       and the room checks find the two agreeing;
    3. step 2 again with the room on holiday until a date:
       the newcomer shows the holiday and its until.
  - **B.** None; the next add in daily use shows it.

The next free number is RoomMajority D23.
