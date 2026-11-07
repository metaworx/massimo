# AP LogView v1.1: Log view newest first, with a range

## TL;DR

The Log view reads newest first, Clear resets the filter,
and one choice limits the view by count or by time;
the log route gets the lower time bound it needs for that.

- `GET …/log` → `?since=<RFC 3339 time>`, in the store and the in-memory ring;
  tests, `docs/api-v1.md`, the OpenAPI file
- Clear → text filter, kind and device back to "all", and a reload;
  off while nothing is set; the range stays
- order → newest at the top, Older events at the bottom;
  live lines come in at the top, a view scrolled down stays where it is
- range → last 20 events (the default), last 10 minutes, last hour,
  today, this week, this month, all;
  Older events pages back within it; a line of another day shows its date
- rolling: "last 20" keeps the newest 20, a time range drops what falls out

### Blocks

1. `since` on `GET …/log`
2. Clear resets the filter
3. Newest at the top
4. The range choice
5. Log screenshots retaken, branch pushed, pull request

## Discussion

Three findings on the Log view from the Pi, v0.10.0-alpha.1,
passed on by a teammate session on 2026-11-07
(filtering by kind and device works there):

1. Clear should clear the filter, not empty the view.
2. The newest entries at the top.
3. One dropdown that limits the view by count or by time:
   last 20 events, last 10 minutes, last hour, today, this week,
   this month, all.

Settled as recommended at the TL;DR gates of 2026-11-07
(`PLAN++ SendButtons` took the recommendations it did not name):

- **D8-A.** A plan of its own, run after the header and settings work,
  its branch started from `claude/navigation` (pull request 97),
  with its own pull request.
- **D9-A.** A new `since` parameter: one query per page whatever the range,
  rather than paging back with `before` until the bound,
  which for "this month" on a busy Cube is many pages of 1000 on a Pi 1.
- **D10-A.** The default range is the last 20 events:
  it fits a phone screen, and the choice is beside it.
- **D11-A.** Rolling: "last 20" keeps the newest 20 as events arrive,
  and a time range drops a line once it falls outside,
  checked each minute with the page's age timer.

## Analysis

- **The route.** `parseLogQuery` in `internal/server/log.go` reads
  `limit`, `before`, `kind` and `device`; `storedLog` asks `store.Events`,
  `filterEvents` the in-memory ring.
  `eventsQuery` in `internal/store/events.go` turns a `before` time into an id bound
  (`SELECT max(id) … time < ?`) plus `time < ?`;
  index `events_cube_time (cube_id, time)` serves a `since` the same way.
  `MaxEventsPage` is 1000.
- **The view.** `log.js`: `renderLog` draws oldest first and sticks to the bottom;
  `appendLogEvent` adds a live line at the end; `loadOlder` pages back
  (`LOG_PAGE = 200`); `log-clear` empties `#log-lines` only.
  Line times are `HH:MM:SS`.
- **A route change** needs a test of the exact query, `docs/api-v1.md`,
  and `docs/api-v1-openapi.yaml`, which `TestOpenAPICoversRoutes` holds to the routes.

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...`
(`-race` for `internal/server` and `internal/store`),
`make cross`, `make ui-test` at 1100 and 390 px in English and German
(from a worktree: `UI_TEST_PYTHON` to the main venv, free `UI_TEST_*_PORT`s),
`gofmt -l .` empty, `go vet ./...` for linux, windows and darwin,
and `GUIDELINES/shared/tools/changelog.sh check`.
Nothing reaches a Cube: the tests use the fake Cube with the recorded greeting.
A held route (`page.route` collecting the route, `route.continue_()` later)
stands for a slow server in the tests.
The branch starts from `claude/navigation` (pull request 97)
and merges `origin/main` before its pull request.
Each block is one functional commit citing `AP LogView v1.1, Block <n>`,
with its CHANGELOG bullet; `docs/ui.md` "Log" moves with the view.

### Block 1: LogSince

1. `internal/store/events.go`: `EventQuery.Since`, an id bound
   (`SELECT min(id) … time >= ?`) plus `time >= ?`.
2. `internal/server/log.go`: `since` parsed as RFC 3339,
   400 for a bad one or one after `before`; the ring filtered the same way.
3. Tests in `internal/store` and `internal/server`;
   `docs/api-v1.md` and the OpenAPI file.

**Verification:** the gate; `TestOpenAPICoversRoutes`.

### Block 2: LogClear

1. `log.js`: Clear sets the text filter empty, kind and device to all,
   and reloads; it is off while nothing is set.
2. Tests: Clear after a kind and a text, the lines back, Clear off.

**Verification:** the gate.

### Block 3: NewestFirst

1. `log.js`: `renderLog` draws newest first, Older events at the bottom;
   a live line goes in at the top, and a view scrolled away from the top
   keeps its place.
2. Tests: the first line is the newest; a live event comes in on top.

**Verification:** the gate.

### Block 4: LogRange

1. `index.html` (the log head) and `log.js`: `#log-range` with the seven choices;
   `logQuery` sends `limit=20` or `since`; Older events pages back within the range;
   a line of another day shows its date; the range is rolling (D11-A).
2. Texts in all six `ui/i18n/<lang>.json`.
3. Tests: last 20 shows at most 20; last hour drops an older stub event;
   Older events stops at the range.

**Verification:** the gate.

### Block 5: Screenshots

1. `screenshots.py` retakes the log shots; the others come out the same.
2. `origin/main` merged, the gate once more,
   the branch pushed and the pull request opened.

**Verification:** only the log shots change.

## Proposed commit message

Block 1, for the form the others follow:

```text
feat(api): Take a lower time bound on the Cube's log

GET /api/v1/cubes/{sn}/log takes since, an RFC 3339 time, for the stored log and the in-memory ring alike, so that a view of the last hour is one query (AP LogView v1.1, Block 1).

Key changes:
- internal/store/events.go: EventQuery.Since, bound by id on the index the before bound uses.
- internal/server/log.go: since parsed, refused when bad or after before.
- docs/api-v1.md, docs/api-v1-openapi.yaml: the parameter.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-07): The first revision, from the TL;DR gates of 2026-11-07:
  D8-A, D9-A, D10-A, D11-A.
- v1.1 (2026-11-07): The block gate stated in the Implementation Plan,
  folding in NOTE LogView v1.0: block gate stated;
  the branch's start named, no longer cited from AP Navigation, which retires.
