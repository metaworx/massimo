# NOTE SteadyTests v1.0: how the run went

The owner's `PLAN++` of 2026-11-26 ran Blocks 1 to 6, 8 and 9;
Block 7 waits for AP FakeGroup, which has no pull request yet.
Each block's proofs are in its pull request's text.
This NOTE records where the run went otherwise than the plan's words,
and what it found outside the plan.

## The pull requests

- **Three, not two.**
  At the owner's question, Block 9 shipped ahead as #466,
  and #467 carried Blocks 1, 2 and 3.
  The version freeze of 0.17 came while they were built:
  both went into `maint/0.17`, the session merged `maint/0.17` upward
  into `main` (`merge: maint/0.17 into main, with SteadyTests Blocks 1 to 3
  and 9`), and #473 carries Blocks 4, 5, 6 and 8 against `main`,
  since Block 5 changes the product.
- **Builders:** Opus sub-agents, one per block group, their commits
  taken onto the pull request's branch; #473 was put together by a
  sub-agent too, which renumbered `docs/ui-testing.md`
  (v1.30.0 to v1.34.0) in its commits' order.

## Block by block

- **Block 1:** item 6's proof needed a check added after the test:
  the test's own assert runs before the stream closes,
  so it passes either way.
- **Block 2:**
  - the commits' component is `fakecube`, as the fake's history has it;
  - a held command is applied at the release itself,
    not `ApplyAfter` later, and the four tests that hold drop `ApplyAfter`;
  - `TestProgramTag` re-reads every 100 ms, 600 times;
    `TestEventSize` writes in the writer's batches of `flushAt`;
    `TestRoomProgramTagOfTheMembers` holds to its end;
  - a sixth commit scales the sweep's apply tests and `confirmWithin`,
    and a seventh says in `docs/testing.md` how a test holds the fake.
- **Block 3:**
  - the retry runs at four times the scale, not three:
    with the trace a test's set-up took up to 3.8 times as long;
  - `ApiReads.pending()` and `wait_started()` are new;
  - item 8 also scales the reloads' 10 s and waits after going back;
  - CI sets no `UI_TEST_SCALE` per job: the plan's Block 3 did not ask it.
- **Block 4:**
  - `ApiReads` counts `/healthz` too; its quiet time is measured from Python,
    so that a paused clock does not stop it; `answered` takes the routes
    a test holds;
  - other copies took the wait: `test_unsaved.py`'s `answered`,
    `test_parts_guard.py`'s `settle` (23 fixed pauses),
    `test_room_updates.py`'s `rooms_loaded`;
  - helpers that stayed in their files differ, or call conftest's
    `api_call`, which `helpers.py` cannot import,
    or write `state`, which Block 6 must count;
  - the guard is a Go package, `tools/ui-test/idioms/`, its lists beside it;
    it counts raw text, so a comment counts too.
    The lists: `wait_for_timeout` 120 in 34 files, plain `.route(` 177 in 45,
    local kit names 0.
- **Block 5:**
  - with `-poll 0` 5 tests failed, none for a device list:
    all chose the poll's interval, which the flag makes read-only;
    they now choose the refresh's, and a new test checks the poll's read-only;
  - the server's `PollInterval` of 0 stays the default and a negative one
    means never, as `CubeHold` does; `serve` maps a poll of 0 or less to never;
  - the reset also restores the NTP servers, ends a silence and releases
    a dead session, and drops a command acknowledged before it when it
    falls due; `f:` reads and writes the NTP servers under the fake's lock;
  - a second reading found that a write a reset takes back would stay
    pending and be sent again 5 minutes later;
    no file of the suite left one, and the fixture now asks the server to
    read the Cube again and waits before each reset, failing with the file
    that left it;
  - a reset takes 1.4 to 1.9 s, about 30 s per case.
- **Block 6:** 39 writes of `state` in 13 files, 20 of them
  `state.settings.expert`.
- **Block 8:**
  - `run.sh` passes `--no-loadscope-reorder` under a seed,
    and `pytest-xdist` is raised to 3.8, the first version with it;
  - the order is a sort by a hash of the seed and the file, then of the test,
    after pytest's own ordering, so that any selection keeps its order;
  - `soak_summary.py` writes one summary per job;
  - the soak also runs on a pull request that changes `ui-soak.yml` or
    `soak_summary.py`, since GitHub starts a workflow by hand only once it is
    on `main`: four jobs of about 30 minutes each time.
- **Block 9:**
  - a script builds the key, `packaging/app-gradle-cache.sh`,
    with two tests beside the Go caches';
  - the key carries the ISO week and the journal is cached,
    so that the cache does not grow;
  - its proof comes after the merge, since only `main` saves.

## Found outside the plan

- `GUIDELINES/project/CI.md` v1.75.0 is claimed by three commits on `main`:
  AP EmulatorRuns', AP MaintBranch's and Block 9's;
  Block 8 takes v1.76.0.
- Go, under load:
  - `TestPollSkipsABusyLine`: the fake records its lines one at a time,
    so an `f:` after a poll's `l:` is missed while the fake is silent;
  - `TestConfigWriteConfirmed`: its 300 ms apply races the writer's re-read;
  - `TestProxyRetriedUntilTheAddressIsThere`,
    `TestHeldReferenceLifetime` ("a newer setpoint of Massimo's drops it")
    and, on Windows, `TestDownloadOutlastsTheClientTimeout`,
    from the session of AP StartConfiguration;
  - `server/log.go`'s flush gives up after the same unscaled 10 s as
    item 2's, a product timer without an option.
- UI:
  - `test_a_joined_device_keeps_what_is_typed` waits 5 s for a row of a
    pairing told in the page alone;
  - `test_moving_inside_asks_nothing` logs a 401 after
    `test_vwt_settings_forgotten` in one shuffled order, the soak's first find;
  - `test_message_line_after_the_sign_in` and
    `test_dismissed_then_a_later_sign_in`: the page's settings sync after a
    sign-in lands before their check;
  - `test_system_info_keeps_its_folds_a_copy_and_a_selection`:
    its "copied" fade is not scaled;
  - waits not yet scaled: 90 `.room` waits after a test's own reload,
    116 literal 10 s timeouts, about 48 hand-written loops,
    and conftest's `api_call` timeout.

These are a v1.1's candidates, beside D6, the tests' front page.
