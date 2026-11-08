# NOTE SettingsLayout v1.0: run blocks 8 to 12

How `EXEC++` from the updated `main` (2026-11-08) went where it differs
from the plan,
[AP SettingsLayout v1.0](2026-11-08_09-53_AP_SettingsLayout_settings_pages_layout.md).

## Deviations

1. **Block 8**: the restart and the settings file are rows of a CSS
   pattern, `.action-row`, in the Maintenance partial's markup, not an
   `actionRow` built by script, which would have had no caller.
2. **Block 9**: `configView` is a function of its own beside `runConfig`,
   made of the same helpers; reshaping `massimo config`'s printing to serve
   both would have risked its output. The server takes no secret on trust:
   it applies `SecretVariable` to whatever view it is handed.
3. **Block 10** decided three things the plan left open:
   - a holder of `passwords` sees every user (an owner's panel read-only,
     saying why), as the mock showed for a manager;
   - the users routes answer, per user, what the caller may do (`may`), so
     the page follows the server's rule rather than guess it;
   - refusals name the rule ("only an owner changes an owner", "the role …
     has capabilities yours lacks", "only an owner gives the role owner").
   Its CHANGELOG bullet is under Security, as its commit is a security fix.
4. **Block 11**: `passwordFields` was not written: My profile's form and the
   first owner's are markup, with no caller built by script.
5. **Block 12**: the screenshots of Maintenance's store and settings in
   effect come from fixed answers, as the run's temporary directory and
   ports differ; a shot of one element takes the header, the page's row
   and ↑ out of their fixed places first, through the elements' style,
   as the page's CSP refuses a style element.

## How it was verified

Every block's state ran the whole UI suite (`make ui-test`, 1100 and 390 px,
English and German): Block 8 here, Blocks 9, 10 and 11 in temporary
worktrees holding exactly their commit's tree; Block 12 changes no code the
suite runs, so Block 11's run is also its. Every block ran the Go checks
(`gofmt -l .`, `go vet` on linux, windows and darwin, `go test ./...`,
`make cross`). The screenshot set was retaken and looked at.
