# NOTE SafeFiles v1.0: how the run went

Where the run of Blocks 1 to 3, under the owner's `PLAN++`,
went differently from the plan's steps.
The blocks themselves stand.

## The branch

- **PR #173 was merged before the work branch began.**
  The plan has the branch start from `claude/discovery-core` so that the new
  pull request carries both;
  by then the owner had merged #173 into `main`,
  so `claude/safe-files` starts from `main`, which holds it,
  and the new pull request carries this plan's commits alone.

## Block 1: Package

- **Two test seams beside `chown`.**
  `readMetaOwner`, nil but in a test, changes the owner `readMeta` read,
  so that a test that is not root can see a write keep, or fail to keep,
  another user's ownership.
- **One test more: `TestOwnerKeptAsRoot`.**
  It runs as root only, in a container here:
  another user's owner and group kept, `KeepOwner: false` leaving root's,
  and a link that user planted not followed.
- **Where the tests ran.**
  Linux, as a user and as root in a container;
  Windows through interop, where `TestDanglingLinkRefused` is skipped,
  since making a link there takes a privilege;
  macOS in CI's macOS job only.

## Block 2: Writers

- **`Options.HeldWait` came with the move.**
  `install`'s Windows test gives up on a held image after 300 ms through its
  `renameHeldWait`, which `install` now passes on;
  the option is how long the rename over a held file is retried, 0 being five
  seconds.
  `renameHeld` went from `install` into that test, its only user.
- **The update helper's retries.**
  Its own retry was ten times 50 ms; it now has the package's five seconds.

## Block 3: Backups

- **The test guards the failure path, not the crash.**
  `TestBackupWholeOrAbsent` passes on the old code too,
  which also removed the copy when `VACUUM INTO` failed;
  what changed is a crash or a kill in the middle of the copy,
  which leaves the copy's name empty instead of holding a partial database,
  and which a test cannot bring about.
- **`docs/store.md` said "online backup".**
  The copy is SQLite's `VACUUM INTO`; the sentence says so now.
