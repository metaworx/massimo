# NOTE RoomMajority v1.2: how block 4 and the pull request's CI went

Where Block 4 and the pull request's CI went differently from the plan, and why;
how the run was done, what verified it, and what it leaves for the owner.
Built on 2026-11-24 on the branch `claude/room-majority`, pull request #410:
two test fixes,
"fix(rooms): Every place that reads one thermostat for its room follows the one order"
(Block 4) and
"test(ui-test): The House's answer after a sign-out leaves the room's tile
waiting for nothing".
Dead, below, is D9's: a thermostat the Cube holds no valid state for;
unreported, D21-A's: one that has not reported since the Cube's restart.

## How it ran

- **As for Blocks 1 to 3:** a builder on Opus 5.5 in the session's worktree,
  without a commit; the coordinating session ran the checks and made the signed commits.
- **A second reader,** an independent sub-agent on Opus, changing nothing in the tree,
  its probes and faults through `go test -overlay`;
  each round's findings fixed by the builder before the next.
- **Planted faults:** every new test shown failing on a fault planted for it;
  the reader planted its own, and each that no test caught got a test.
- **Nothing went to a real Cube.**

## The pull request's CI

- **The four UI jobs failed** on the v1.1 run's last push,
  on two tests of Block 3's own making:
  - `test_the_house_in_eco` held a detail read that Block 3's `presetTemps`
    no longer makes, the entry carrying eco and comfort,
    so its Eco setpoints went unheld to the worker's fake Cube;
  - `test_marks_and_off` measured in the viewport,
    and the header's line of a radiogram pending, which those writes left,
    moved the value 86.4 px at 390-de.

  Fixed by "test(ui-test): The late Eco test holds the thermostat's detail read again
  and stubs every setpoint" and "test(ui-test): The marks test measures the value,
  the chevrons and the icon slot in their own rows".
- **A gap beside them:** the House tile's session check after its room setpoint
  (`rooms/House.tsx`) had no test; found by the same fix, closed in its own commit,
  the House test above.
- **The Android emulator job,** new with MobileApp Block 18,
  ran on #410 only because the merge carried main's `app/` changes:
  `KeepTest.aServerIsAddedAndItsCertificateTrusted` did not run
  (testRunFailed, no reason in the log); a re-run passed.
  The branch touches nothing under `app/`.
- **Main merged three times,** each without a conflict:
  ReloadChecks (#406) with MobileApp Block 18 (#411); the plan's v1.2;
  the retirement of ReloadChecks and LogEdges.
- **The local guidelines check,** at the test fixes' push,
  read the branch's upstream range and reported main's
  "ReloadChecks: A reload warns of every setting the next start refuses,
  and the Maintenance card says so (#406)", a merge-button commit, as three errors;
  not ours, and CI's base..head leaves it out. At Block 4's push it was clean.

## Block 4, Sources

Different from the plan:

- **`internal/roomorder`,** new, below `internal/view` and `internal/cubeio`,
  holds Block 4's one function (`State`: `Majority`, `Newest`, `ByOrder`)
  and the programme helpers from `drift.go`;
  `cubeio.Dead` and `cubeio.RoomOrder` moved into it (`Dead`, `Of`).
- **Every thermostat dead,** as after a Cube restart:
  the room's configuration, programme and presets are decided over them all,
  as the addressee is, not refused by Block 2's dead rule;
  a 404 would hold the configuration panel for minutes to hours after each restart,
  and the recorded greeting has every thermostat dead.
- **While one thermostat is unreported,** D21-A's rule:
  the shown configuration (`GET …/config` and `…/program`, the presets,
  the virtual wall thermostat's own, the proxy's virtual C line)
  is decided over all members, the out-of-date passed over;
  without it, it flipped to whichever thermostat reported first.
- **The programme's tag** covers the members' programmes only:
  a change at a device still gives 412, a change of who is alive none.
- **`room.thermostat`** is not the first of `cubeio.RoomOrder`, as Block 4 had it,
  but the member whose setpoint the virtual wall thermostat shows:
  the first of its `sources.setpoint`, new (the pick first;
  during a boost the pick among those not boosting);
  the addressee only where no thermostat has a state.
  Radiograms keep the addressee.
  The page's `roomThermostat` reads the live `room.vwt.sources.setpoint` first,
  so SET follows a state change without a reload.
- **The page's reloads:** the rooms are read again on a config event that changed
  a thermostat's presets and on a state event whose thermostat the Cube gained or lost;
  `loadRooms` keeps one read in flight and one queued per session and Cube,
  a Cube change does not wait behind the previous Cube's read,
  and an answer for another serial is dropped.
- **The House's ECO and COMFORT** take the room's presets.
- **The virtual wall thermostat's own configuration,** its device detail
  and the proxy's virtual C line, by `roomConfig` too,
  the offset its real wall thermostat's own:
  Block 4 had not listed it; the builder found it, and it was added by decision here.
- **`TestRoomThermostatState`'s "reported last" case** now expects the majority's state.

The readings, two and a last round:

- **First:** SET followed the addressee, so a master alone at another setpoint
  showed on the tile while the virtual wall thermostat and the re-send took the majority;
  after a Cube restart the shown configuration flipped to the first to report;
  the tag gave a 412 where only who was alive had changed;
  the new reloads ran once per thermostat at a reconnect, the last answer winning;
  no test caught the master ignored or a boost's passes swapped;
  the docs misstated the boost group.
- **Second:** the page's `room.thermostat` went stale between reloads,
  so SET could show the outlier again;
  during a boost the entry named the boosting member;
  no test caught the pick-first order dropped;
  a Cube change's read waited behind the previous Cube's.
- **The last round,** the fixes of the second's findings, was not read again:
  each new test failed on the fault planted for it,
  and the coordinating session ran the checks.

## What verified it

Each exit 0:

- **The test fixes:** the two files in the four cases, 168 passed;
  then CI on their push, the emulator job after its re-run.
- **Block 4, at its last round:** `go test` of `internal/server`, `internal/view`,
  `internal/cubeio`, `internal/roomorder`, `internal/cubelan`, `internal/proxy`,
  `cmd/massimo`, `tools/identifiers`, `tools/apitypes`;
  `-race` of `internal/server` with `MASSIMO_TEST_SCALE=2`, as `make race` and CI run it;
  unscaled, it times out `TestLogAfterReadsABurstOnce/store` on this host
  under the added load;
  `make ui-check`; the UI run over 19 files in the four cases, 1045 passed.
- **At the commit,** the coordinating session: `make ui`, those Go packages,
  `gofmt -l .`, `go vet` for linux, windows and darwin, `make ui-check`,
  `GUIDELINES/shared/tools/check`.
- **Left to CI:** the whole Go suite, the race job, the cross builds
  and the whole UI suite.
- **CI on Block 4's push,** after the last merge of main: every check passed
  (28 passed, 6 skipped by their paths), the pull request mergeable.

## Seen and left

- `TestSameConfigAtDebug` (`internal/server`) failed in 3 of 23 runs
  while four faulted runs ran beside it, the re-read after `SetConfig`
  logging the old C line; clean alone and under `-race`; not Block 4's.

## For the owner

- **D22-A's hardware check** waits for the owner's word; its steps are D22's in
  `wip/2026-11-24_01-44_NOTE_RoomMajority_v1.1_how_blocks_1_to_3_went.md`,
  and Block 4 adds none.
- **No new question.** Block 4's deviations follow D9 and D21-A;
  two depart from Block 4's words and are for the owner's reading:
  the room entry's `thermostat` names the room's state, not the first of the order,
  and a room without a live thermostat shows its configuration over them all, not none.

The next free number is RoomMajority D23.
