# NOTE AddressCheck v1.0: how the run went

The run of `PLAN++ as recommended` on 2026-11-14, Blocks 1 and 2 on
`claude/address-check` (PR #254), and where it went differently from
the plan.

## Block 1

- **The lost Cube's discovery test** (`discovery_test.go`,
  `TestALostFixedCubeWithAnIntervalIsSearchedFor`) waited for a Warn
  after every search; the plan did not name it.
  It now waits for the one Warn and three searches after it,
  and fails on a second.
- **The new tests** run over a stored Cube (`storedElsewhere`, on
  `startStored`) rather than over `fixedCube`:
  a Cube of `Options.Cubes` alone is one of the command line,
  which a `PUT` does not change.

## Block 2

- **The Cube's own badge** reads "found elsewhere", without a count;
  the Cubes card's counts the Cubes, as its other badges do.
- **The UI test's routes** let a request still under way go when they
  are removed, as `test_announce_setting.py` does:
  the first run failed once in twelve with "Route is already handled",
  the reload a `PUT` starts being under way at the `unroute`;
  twenty runs passed after.
  `test_proxy_not_listening` (AP ProxyRetry) has the same pattern
  without the guard and was left as it is.
- **The screenshots** were not retaken: no fixture has
  `found_elsewhere`, and the empty box takes no space.
- **The UI suite**: 732 passed at 1100 and 390 px, de and en.

## The branch

- Main gained a plan file only (`chore(aps): +TestRaces v1.0`)
  before the pull request; merged into the branch as the contract asks,
  without a conflict.
