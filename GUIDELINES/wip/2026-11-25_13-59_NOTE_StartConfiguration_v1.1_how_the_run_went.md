# NOTE StartConfiguration v1.1: how the run went

Plan: `wip/2026-11-25_00-36_AP_StartConfiguration_the_settings_file_and_the_start.md`.
The run of v1.1's Blocks 12 and 13, by the owner's `PLAN++` of 2026-11-25,
each built by a sub-agent, read by an independent second reader, its points
fixed, and verified and committed by the session.

## The pull requests

| Block | Pull request | State |
|-------|--------------|-------|
| 13 SettingsRefused | #440 | merged by the owner |
| 12 Include | #443 | open, `main` merged in after #440 |

## Where the run went differently from the plan

- **Block 13's summary line** says "a file serve cannot read", where the
  plan's says "a file serve refuses".
  The reader showed that install refuses only what serve's parser refuses:
  a value the start's own checks refuse, such as a bad `MASSIMO_HSTS`, is
  still copied.
  The docs, the comments and the fragment say the same, and install was
  not extended to the start's checks, which stay outside D38-B.
- **Block 13: a directory as `-settings`** is refused before any write too,
  with the system's own error text; a test holds it.
- **Block 12: Block 6's fragment removed.**
  `CHANGELOG.d/Changed/claude-restart-read-with-update.md` was in no release,
  and its "without `restart`, `available` is false" no longer holds;
  the new fragment says what changed since the release (COMMIT.md §5).
- **Block 12: how the include knows a disabled capability.**
  `store.Effective` returns the disabled list it already reads, and the
  principal carries it, so the include needs no second query.
  The include is taken in `principal.has`, after every bound.
- **Block 12: the integration's maximum** got a UI test of its own beside
  the role editor's, at the reader's point.
- **Block 12: a disabled `restart` through an integration**, by its key and
  acting for a user, was untested until the reader planted it;
  `TestIncludeAfterTheIntegrationsBounds` holds it now.

## Settled by D37

- The v1.0 run's open point 2, `reboot` reading `GET /api/v1/restart`:
  a holder of `reboot` holds `restart` now, so the reboot's question shows
  what the next start refuses to every holder of `reboot`, unless `restart`
  is disabled for every role (`docs/ui.md`).

## Found outside the plan, not changed

- A `-settings` path that is not there fails only at the copy, after the
  binary and the directories are written; on Windows after a running
  service was stopped.
  Where a settings file is kept, it passes without a word.
  Older than this plan; `TestInstallSettingsNotThere` pins today's
  behaviour.
- Install checks how the `-settings` file parses, not its values:
  a value the start refuses is installed, and the service does not start.
- A kept settings file with a line too long, or in UTF-16, still installs
  with "cannot check", and its TLS drop-ins fall back to plain http.
- `docs/store.md` §6's `update` row does not name
  `PUT /api/v1/update/answer`, which AP AutoUpdate added to
  `docs/api-v1.md` only.
- AP AutoUpdate's plan still describes D34-A's answer to a caller without
  `restart`, `available` false, which Block 12 removes.
