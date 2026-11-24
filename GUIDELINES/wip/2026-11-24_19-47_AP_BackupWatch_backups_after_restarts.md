# AP BackupWatch v1.0: Backups after restarts

## TL;DR

A Cube's change gets its automatic backup on time whatever the starts,
the Backup and restore card shows each Cube's list after any load,
and the backups of a Cube no longer served can be reached again.

- pending backup's windows in memory, reset by every start
  → kept in the store, read when the Cube's runtime starts
- a clean stop with a change pending → the automatic backup taken before the Cubes close
- a load or reload onto System settings → card empty, every time
  → each Cube's list read once the page knows its Cubes
- a clean Cube's short list, no reason given
  → "No change since the newest backup" under it
- a Cube taken out of the configuration → its backups kept, out of reach
  → listed, downloaded, compared and deleted, in a fold of their own
- `GET /api/v1/cubes` → a `backups` count per Cube

### Blocks

1. The windows in the store, and the backup at a clean stop
2. The lists after a load, and the line of a clean Cube
3. The backups of a Cube no longer served

## Discussion

### What the owner reported

On 2026-11-15: "Backup and restore: seems to be empty after each boot. where are the
older backups?"
The analysis this plan rests on is
`wip/2026-11-24_19-30_ANALYSIS_BackupWatch_backups_after_restarts_v2.md`.
It found nothing lost on the owner's box:
all 8 backups ever taken were in the store and listed.
It found two defects and a gap:

- every start resets the pending backup's windows;
- on main, the card is drawn empty after every load onto System settings;
- a Cube no longer served keeps its backups, but nothing reaches them.

### What is decided

The owner, 2026-11-24, `PLAN++ as recommended`, after the EVAL of D4-B.
Earlier, on the same day, the owner had answered `D1-C, D2-A, D3-A`.
This plan's next decision is D6.

- **D1-C. The windows across a start:** both.
  The windows are kept in the store per Cube and read again at the start.
  The pending automatic backup is taken at a clean stop, before the Cubes close.
- **D2-A. The page drawn before its Cubes:** fixed in this plan,
  with UI tests for every load that drew the card empty.
- **D3-A. Why there is no newer backup:** a line under a clean Cube's list,
  through `t()`, in all six languages.
- **D4-B. A Cube no longer served:** in this plan, scoped.
  Its backups can be listed, downloaded, compared and deleted.
  It takes no backup and no restore; "Restore from file" brings one onto another Cube.
- **D5-A. How the page learns which of them have backups:** a `backups` count per Cube row
  in `GET /api/v1/cubes`.

## Analysis

- **The windows** (`internal/server/backups.go`, `backupWatch`):
  - `dirtySince` and `lastChange` live in memory, beside `ref` (the newest backup's
    configuration hash) and `cur` (the hash at the last look).
  - The first look after a start (`backupCheck`, a minute after the start) finds `cur`
    empty, so the Cube is dirty from now: both windows start again.
  - So `backups.auto_max` ("at the latest two hours after the first change", `docs/ui.md`)
    does not hold across a start.
  - The list's `changed` and `due` (`backupWatch.state`) are empty until that first look.
  - The pending state goes to a table of its own (`backup_pending`, by `cube_id`),
    not to the `cubes` row, so that the row's `version` and `updated_at` keep meaning
    a change of configuration.
    The migration is additive (`docs/store.md`, "Rules"): it keeps `min_reader` 24
    and needs its down part, as every migration after 0024 does.
  - It is written when a Cube becomes dirty, at a new hash while dirty (at most once a
    minute, at the looks), and deleted when a backup is stored or the Cube is clean again.
- **The stop** (`internal/serve/serve.go`, `Instance.Run`;
  `internal/server/server.go`, `Run`):
  - The server's context is the signal's (SIGINT, SIGTERM, a SIGHUP that `-sighup` makes a
    stop; `cmd/massimo/serve.go`), so the Cubes' runtimes end with it.
  - A backup at the stop needs a step between the signal and that end, bounded within
    `stopGrace` (10 s).
  - The restart from the API and the update helper end serve through the same stop
    (`internal/server/restart_api.go`; the helper restarts the unit).
- **The page** (`internal/server/ui/src/backups/`, AP UiParts):
  - `onView("system")` reads the lists of `state.cubes` as they are when the page is shown.
  - `app.js` `init` shows the address's page before `refresh()` reads the Cubes,
    so on a load onto `#system` no list is read.
  - A box whose list never loaded draws nothing, so the card is empty.
  - On `#cube`, the same holds without a stored Cube.
  - Reproduced on main in 8 cases (the analysis, §1).
- **A Cube no longer served:**
  - `Server.cube(r)` finds runtimes only, so `…/backups` of a row with `configured` 0
    answers 404.
  - `GET /api/v1/cubes` lists every row; the page splits them into `state.cubes` and
    `state.found` (`app.js`, `isConfigured`).
  - Retention keeps the newest 10 of each Cube forever, so such backups stay in the store,
    reachable through `sqlite3` alone.

## Implementation Plan

- An Opus sub-agent builds each block in this worktree, one at a time, without a commit.
- The session reads the diff, runs the checks once, and makes the signed commit.
- The three blocks are one pull request, `claude/backup-watch`,
  with `main` merged in before it.
- Verification is local for what a block touches.
  The whole Go suite, `make race`, `make cross` and the whole UI matrix are left to CI.

### Block 1: The windows in the store, and the backup at a clean stop

- Migration `0026_backup_pending.sql` with its down part:
  a table `backup_pending` (the Cube's row, the hash seen, the first and the last change).
  It also goes in `store.sql` and the store's tests of migrations and down parts.
- `internal/store`: read, write and delete a Cube's pending state.
- `internal/server/backups.go`:
  - the pending state is read when the runtime starts,
    so that the list's `changed` and `due` are right from the first request;
  - at a look, the same hash as kept keeps the kept windows; another hash is a change now;
  - the state is written and deleted as the Analysis says.
- The stop:
  - on a clean stop, before the runtimes' context ends, each connected Cube with a backup
    pending gets its automatic backup;
  - the step is bounded (a few seconds in all, within `stopGrace`);
  - a Cube not connected, or one that fails, is logged and left: the next start reads its
    state from the store.
- Tests against the fake Cube:
  - a change, a restart of the server, and the backup due at the same time as without
    the restart;
  - `auto_max` across a restart;
  - a change while stopped is a change at the first look;
  - a clean stop with a change pending stores the backup before the store closes;
  - no write while a Cube is clean;
  - the migration and its down part.
- Docs: `docs/store.md` §9 (e) (the windows kept, the backup at a stop, the table in the
  schema and the migration's paragraph); `docs/ui.md` where it says when a backup follows;
  a `Fixed` CHANGELOG fragment.

**Verification:** `go test ./internal/store ./internal/server ./internal/serve`,
`gofmt -l .`, `go vet` for linux, windows and darwin.

### Block 2: The lists after a load, and the line of a clean Cube

- `src/backups`: a box reads its Cube's list when it is drawn and its list has not loaded,
  so that a load onto System settings or Cube settings shows each list once the page knows
  its Cubes.
- Under a clean Cube's list, which has backups and nothing pending:
  "No change since the newest backup: the server takes one after a change."
  It goes through `t()`, with its key in all six `i18n/<lang>.json`
  (FR, IT, NL, PL machine-translated and marked).
- UI tests in `tools/ui-test/test_backup.py`:
  - a first load and a reload onto `#system`, and a reload onto `#cube` without a stored
    Cube, each show the list;
  - the clean Cube's line, and no such line while a backup is pending.
- Docs: `docs/ui.md` (Backup and restore);
  a `Fixed` CHANGELOG fragment for the empty card, and an `Added` one for the line.

**Verification:** `make ui`, `make ui-check`; `test_backup.py` in the four cases
(1100 and 390 px, `en` and `de`); `go test ./internal/server` (the UI's Go tests).

### Block 3: The backups of a Cube no longer served

- Server (`internal/server/backups_api.go`):
  - `GET …/backups`, `GET …/backups/{id}`, `GET …/backups/{id}/diff` and
    `DELETE …/backups/{id}` find a Cube by its store row where no runtime serves it;
  - for such a Cube the list carries no `changed` or `due`;
  - `POST …/backups` and the restore routes answer `409` for a Cube that is not served;
  - capabilities as for a served Cube.
- `GET /api/v1/cubes`: a `backups` count per row, one grouped count in the store (D5-A).
- UI (`src/backups`): in System settings, after the served Cubes, a fold for each Cube row
  that is not served and has backups.
  It is marked "not served", and offers Download, Compare and Delete (with `restore`),
  with no Take or Restore. Texts in all six languages.
- Tests: the routes for a row that is not served (list, file, diff, delete, `409`);
  the count; the fold in `test_backup.py` (served Cubes first, no fold without backups).
- Docs: `docs/api-v1.md` (Backups, the Cube list's field) and `docs/api-v1-openapi.yaml`;
  `docs/ui.md`; `docs/store.md` §9 (e); an `Added` CHANGELOG fragment.

**Verification:** `go test ./internal/server ./internal/store`, `gofmt -l .`,
`go vet` for the three systems; `make ui`, `make ui-check`;
`test_backup.py` in the four cases.

### The pull request

- Branch `claude/backup-watch`, the blocks' commits,
  `main` merged in before the pull request.
- `GUIDELINES/shared/tools/check` over the range.
- The owner merges it; checking the windows and the stop on breno is the owner's.

## Proposed commit messages

- The plan: `chore(aps): +BackupWatch v1.0, +ANALYSIS BackupWatch`
- Block 1:
  `fix(server.backups): A change's automatic backup keeps its time across a restart`
- Block 2:
  `fix(ui.backups): Backup and restore shows each Cube's list after a load onto its page`
- Block 3:
  `feat(server.backups): The backups of a Cube no longer served can be listed and downloaded`

## Change History

- v1.0 (2026-11-24): first revision, by `PLAN++ as recommended`,
  with D1-C, D2-A, D3-A, D4-B and D5-A.
