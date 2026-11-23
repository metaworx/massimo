# AP ReloadChecks v1.0: The start's checks at a reload

## TL;DR

About 20 settings stop `serve` at its start, and a reload warns of 4 of them.
A typo in any other is taken silently, and the next restart leaves the box
without its web UI.
This plan gives the start and the reload one table of checks, guarded by a
test, and shows what the next start refuses in the log, the sign and the
Maintenance card.

- the start's checks inline in three places → one table, run by the start
  first and by a reload over what changed; the texts match by design
- a reload's 4 checks → every check without side effects
  (log rotation, HSTS, trusted proxies, restart, SIGHUP, reboot, announce,
  TLS, a Cube's `_IGNORE`)
- the reboot switch's bad value → Warn on stderr only →
  `settings_read`'s warning, one line
- `MASSIMO_RESTART`, `MASSIMO_SIGHUP` unknown → known
- a settings file that does not parse → Warn on stderr only → a warning in
  the log
- the Maintenance card → says what the next start refuses, without blocking

### Blocks

1. Known: `MASSIMO_RESTART` and `MASSIMO_SIGHUP` in `env.Known`
2. Table: one table of the start's checks, run at a reload; the reboot switch in it
3. Guard: a test that every setting the start refuses has its entry
4. Unreadable: a settings file that does not parse is a warning in the log
5. Card: the Maintenance card says what the next start refuses

## Discussion

### What the owner asked for

On 2026-11-23, after AP LogEdges' NOTE named the reload's other refused
settings and the reboot switch's warning ("Found, not changed", items 1 and 2):
`PLAN ReloadChecks`; then, at the TL;DR gate, `D8B`, `others: A`, `PLAN++`.

### What is decided

The owner, 2026-11-23:

- **D1-A. Which checks a reload runs:** the pure ones only.
  `MASSIMO_LISTEN`, `MASSIMO_LOG`, `MASSIMO_DATA` and the `-tls-http` bind
  stay out, and the docs say so: they fail on the box's state, and every
  packaged service fixes the first three by flags.
- **D2-A. Which settings trigger a check:** a check runs where one of its
  settings differs from the start and no flag fixed it,
  with a fixed flag's value where the check reads one.
- **D3-A. One source:** one table, which the start runs first and a reload
  runs too.
- **D4-A. The guard:** every variable `serve` reads is classified as refused
  or as taking any value, and the built binary runs per bad value;
  skipped with `-short`.
- **D5-A. Values in the warning:** the start's text, with a
  `SecretVariable`'s value replaced by its name.
- **D6-A. The reboot switch:** its refusal is `settings_read`'s warning;
  the reloader's own Warn goes.
- **D7-A. The token:** no change.
- **D8-B. A restart while a setting is refused:** the Maintenance card says
  what the next start refuses, without blocking the restart.
- **D9-A. A settings file that does not parse:** `settings_read` at warn with
  the parse error.
- **D10-A. `MASSIMO_RESTART` and `MASSIMO_SIGHUP`:** known, in Block 1.
- **D11-A. A bad `_IGNORE` of a Cube the store holds:** the start keeps
  refusing it; a reload warns.

## Analysis

The analysis this plan rests on is
`wip/2026-11-23_15-39_ANALYSIS_ReloadChecks_settings_the_start_refuses.md`,
with the table of every setting the start refuses.
What bears on the blocks:

- **The start's checks** are inline in `cmd/massimo/serve.go:105-183`,
  `internal/serve/tls.go:63-106` and `internal/serve/setup.go:44`;
  the reload's `startChecks` (`cmd/massimo/reload.go`, LogEdges Block 2)
  holds the log level and the three formats, each alone,
  over the pending settings.
- **The TLS refusals are pairings:** an unchanged `MASSIMO_TLS_HTTP` is refused
  once `MASSIMO_TLS` changes from acme; `OpenDNS01` creates a directory, so its
  checks need a pure half.
- **The reloader** keeps which settings a flag fixed, not the flags' values.
- **The reboot switch** applies at once at a reload; a bad value switches it
  off with a Warn on the reloader's log only.
- **`env.Known`** lacks `MASSIMO_RESTART` and `MASSIMO_SIGHUP`, so
  `massimo config` calls them "typo?" and a reload never lists them.
- **A file that does not parse** gets "settings file not read again" on the
  process log only, and `POST /api/v1/reload` a 500.
- **The Maintenance card** is a part (`internal/server/ui/src/maintenance/`)
  and lists the pending settings by name (`GET /api/v1/restart`).
- **LogEdges' fragment** `CHANGELOG.d/Fixed/claude-logedges-reload.md` is
  unreleased and covers the same fix; Blocks 2 to 4 amend it.

## Implementation Plan

A sub-agent on Opus builds each block without a commit,
a second reader reads it, and the session verifies it and makes the commit;
the five commits are one pull request.

### Block 1: Known

- `internal/env`: `MASSIMO_RESTART` and `MASSIMO_SIGHUP` in `Known` (D10-A).
- A test that every variable behind a flag of `serve` (`flagVars`) is known.
- A `Fixed` CHANGELOG fragment.

**Verification:** the test fails with either name taken out;
`go test ./internal/env ./cmd/massimo`.

### Block 2: Table

- `cmd/massimo/startchecks.go`: the table of the start's checks in the start's
  order, each with the variables it reads and the start's own parser (D3-A);
  `startRefusals(get, only)`.
- The start runs the table right after its flags are parsed and returns the
  first refusal as before; the inline checks go, and the inner "serve: "
  prefixes move out.
- The reload runs each check where one of its variables differs from the
  start and no flag fixed it, with a fixed flag's value where it reads one
  (D2-A); the reloader keeps the fixed flags' values.
- The pure checks only (D1-A): the log settings, rotation, HSTS, trusted
  proxies, restart, SIGHUP, the reboot switch, announce;
  `serve.CheckTLS`, the pure half of `tlsSource`, with
  `certs.CheckACMENames` and `CheckDNS01Hook`;
  the Cubes' `_IGNORE` through `serve.FileCubes` (D11-A).
- The reboot switch in the table: its refusal is `settings_read`'s warning,
  the reloader's own Warn goes (D6-A).
- A `SecretVariable`'s value never in the warning (D5-A).
- Tests: `TestReloadWarnsOfARefusedLogSetting` becomes the test of every entry;
  a pairing (an unchanged `MASSIMO_TLS_HTTP` with the mode changed);
  a flag's value used where it fixes a setting;
  the environment's value never warns;
  no `MASSIMO_TLS_KEY` path in the warning;
  the DNS-01 check leaves the data directory untouched;
  unit tests in `internal/certs` and `internal/serve`.
- Docs: the README's reload paragraph (and which settings a reload cannot
  check), `docs/api-v1.md` (`settings_read`, `POST /api/v1/reload`),
  `docs/ui.md`; the LogEdges fragment amended.

**Verification:** each test fails on a fault planted for it (an entry
missing, the checks over the pending settings only, a fixed flag's value
ignored, a check with a side effect, a secret in the warning);
`go test ./...`; `-race` at scale 2 for the touched packages;
`gofmt`, `go vet` for the three systems.

### Block 3: Guard

- `TestStartChecksCoverTheStart` (D4-A): every variable `serve` reads,
  classified as refused (a bad value, or a pairing) or as taking any value
  with a reason; the built binary run per bad value, its "error:" line
  compared with the table's; skipped with `-short`.
- The fragment amended, `cmd/**` being shipped.

**Verification:** it fails on a variable nobody classified, a refusal
without an entry, a text that drifted, and an entry the start does not
refuse; its time.

### Block 4: Unreadable

- A settings file that does not parse at a reload: `settings_read` at warn
  with the parse error, settled by the next good read (D9-A);
  the reloader's own Warn goes as in Block 2.
- Tests; `docs/api-v1.md`, the README; the fragment amended.

**Verification:** the test fails without the warning and without the
settling; `go test ./cmd/massimo ./internal/server`.

### Block 5: Card

- The server keeps what the last reload found the next start refuses;
  `GET /api/v1/restart` and `POST /api/v1/reload` answer it, `refused`,
  the setting's name and the start's text, a secret's value by its name;
  nothing is blocked (D8-B).
- The Maintenance part says what the next start refuses, beside the pending
  settings; its texts in all six languages (FR/IT/NL/PL machine-translated
  and marked).
- Tests: the API's answer; a UI test of the card in the four cases.
- Docs: `docs/api-v1.md`, the OpenAPI file and `api.d.ts` (`make api-types`),
  `docs/ui.md`; a CHANGELOG fragment.
- AP UiParts holds the page's regions; whoever lands second in
  `src/maintenance/` carries the other's change.

**Verification:** the changed UI test files in the four cases;
`make ui-check`; `go test ./internal/server`.

### The pull request

The five commits on one branch, `main` merged in before it;
the guidelines check over the range; the pull request's run green.
The owner merges it.

## Proposed commit messages

- The plan: `chore(aps): +ReloadChecks v1.0, +ANALYSIS ReloadChecks`
- Block 1: `fix(env): MASSIMO_RESTART and MASSIMO_SIGHUP are known settings`
- Block 2: `fix(serve): A reload warns of every setting the next start refuses`
- Block 3: `test(serve): Every setting the start refuses has its check at a reload`
- Block 4: `fix(serve): A settings file that does not parse is a warning in the log`
- Block 5: `feat(ui/maintenance): The Maintenance card says what the next start refuses`

## Change History

- v1.0 (2026-11-23): first revision, by `PLAN++`, from AP LogEdges' NOTE,
  items 1 and 2, with D1-A to D7-A, D8-B and D9-A to D11-A.
