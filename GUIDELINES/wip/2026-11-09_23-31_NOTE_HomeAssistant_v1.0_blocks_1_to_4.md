# NOTE HomeAssistant v1.0: blocks 1 to 4

How the `PLAN++` run of Blocks 1 to 4 went,
on the branch `claude/home-assistant`, where it differed from the plan,
and what it found on the way.

## Where it differs from the plan

- **Block 3, the element's looks.**
  The Analysis has the element in an open shadow root with styles of its own.
  It renders into the page's DOM instead,
  and builds its controls with a kit its host gives it:
  the project contract has the page's shared elements come from `components.js`,
  which a shadow root's own copies would duplicate,
  and in the page's DOM Massimo's styles and the suite's selectors reach it
  unchanged.
  The Home Assistant card (Block 8) gives a kit of its own,
  Home Assistant's controls, and so shows the editor in Home Assistant's look.
  `style.css` gives the element `display: contents`,
  so that its parts sit in the form's column as before.
- **Block 4, the roadmap.**
  Item 14 is not changed:
  the contract (v1.5.0) leaves `docs/roadmap.md` to the coordinating session,
  but for pruning a plan's own items when it retires.
  Naming Home Assistant there is the coordinating session's to do.
- **Block 1, a detail the plan did not name.**
  The last state event of a first connection carries the newest id,
  so that a client which read nothing else yet still has an id to resume from.

## Found on the way

- **The UI suite's `test_login.py` fails after `test_tls.py`.**
  Run in that order on one worker, all 12 of its tests fail
  ("the sign-in dialog opens without a token"), on `origin/main` as well
  (checked at "Merge pull request #153", 2026-11-09).
  With `--dist loadfile` the two files share a worker by chance,
  so a full run fails or passes by luck; run.sh then fails it as flaky.
  Not fixed here: a separate task was offered to the owner.
- **`test_log_opens` errored once** in its setup (390 px, English)
  and passed in three runs after; taken as flaky.

## Verification

Each block: `gofmt -l .` empty, `go vet` for linux, windows and darwin,
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `make cross`;
`go test -race` for `internal/server` (Block 1) and `internal/store` (Block 2);
`make ui-test` at 1100 and 390 px for Blocks 1 and 3,
the last full run 540 passed,
the new `test_week_program.py` 28 passed.
