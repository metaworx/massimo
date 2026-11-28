# AP ServerLimits v1.0: Limits set in the page

## TL;DR

The server's limits that only the settings API sets today become rows of a
card of their own on the Maintenance page, closing roadmap item 6 whole;
for 0.18, on `main`.

- window limits `window_warn_after`, `window_alert_after` → two rows,
  steps the server would refuse disabled (alert not under warning)
- stale thresholds `stale_after`,
  `stale_after_wall` and `log.keep_days` → three rows;
  read-only where `-stale-after` fixes them; a lower log choice asks first
- a limit changed → the page reads its rooms again,
  so the tiles' signs follow at once
- a custom role holding `settings` → may open the Maintenance page
- `-stale-after` given → a `PUT` no longer changes the value in effect;
  the flag wins, as `fixed_by` says
- documents → `docs/ui.md`, `docs/api-v1.md`; screenshots retaken

Start after #473 (AP SteadyTests), which changes the UI test kit and files
both blocks touch.

### Blocks

1. The window limits: a card of their own on the Maintenance page
2. The stale thresholds and the log's days, and `-stale-after`'s `fixed_by`

## Discussion

### What is decided

The owner, 2026-11-27:
"PLAN D2's fix on `main`" (D2 of the gate retiring nine plans:
AP Signs had left the window limits to roadmap item 6), then, at the TL;DR gate,
"PLAN++ all A".
Every decision below is taken as A.

- **D1-A.
  Scope: all of roadmap item 6.**
  - **A (taken).**
    The window limits, both stale thresholds and `log.keep_days`:
    one pattern, and item 6 closes whole.
  - **B.**
    The window limits alone.
  - **C.**
    A, and `audit.keep_days` and `commands.keep_days` too.
- **D2-A.
  Where: a card of its own on the Maintenance page.**
  - **A (taken).**
    After the Maintenance card, drawn by the maintenance region.
  - **B.**
    Rows in the Maintenance card, which would grow from 9 rows to 14.
  - **C.**
    A card on System settings: a new region,
    the duration helpers moved to `shared/`.
- **D3-A.
  The control: a `Select` of steps, saved at once.**
  - **A (taken).**
    As the Cube intervals are (`useSetAtOnce`), the default sent as `null`.
  - **B.**
    A number field, saved when left.
  - **C.**
    A form with Save and unsaved marks.
- **D4-A.
  The warning before the alert: refused steps disabled.**
  - **A (taken).**
    The steps the server would refuse are listed, but disabled;
    equal is allowed, as the server allows it.
  - **B.**
    Every step offered; the server refuses.
  - **C.**
    Raising the warning past the alert carries the alert along.
- **D5-A.
  A change shows at once: the page reads its rooms again.**
  - **A (taken).**
    At a `change` of a window limit or a stale threshold.
  - **B.**
    The server sends state events for the devices it affects.
  - **C.**
    Nothing until the next load of the rooms.
- **D6-A.
  Who sees it: `settings` added to `systemOf` and `hasSystem`.**
  - **A (taken).**
    A custom role holding `settings` may open the page its rows are on.
    No default role is affected:
    the owner and the integration role hold other capabilities of the page
    already, and a manager does not hold `settings`.
  - **B.**
    As it is.
- **D7-A.
  Lowering `log.keep_days`: a question first.**
  - **A (taken).**
    Steps from 14 days; a choice under the value in effect asks first,
    since the prune it starts cannot be undone.
  - **B.**
    Saved at once, every value offered.
- **D8-A.
  `-stale-after`'s `fixed_by`: fixed in Block 2.**
  - **A (taken).**
    With the flag given, a `PUT` of a stale threshold is stored and no longer
    changes the value in effect, so that `in_effect` and `fixed_by` agree;
    the stored value takes effect at a start without the flag.
    A Go case shows it.
  - **B.**
    Left as it is.

### What stays as it is

- The server's rules for these keys: their ranges, the default as `null`,
  the alert not under the warning, the prune at a lower `log.keep_days`.
- The refusal's English text:
  AP ErrorCodes' Block 4 gives it a code and the page's language.
- `audit.keep_days` and `commands.keep_days` (D1-C not taken).
- No unsaved marks on these rows (AP UnsavedMarks, D6-A).
- The roadmap: item 6 is pruned at this plan's retirement, by this session.
- Any retirement, a release.

## Analysis

Read on `main` at `merge: maint/0.17 into main, with +NOTE ErrorCodes v1.0`,
2026-11-27.

### The server

- **The keys.**
  `window_warn_after` and `window_alert_after` (`internal/store/settings.go`),
  defaults 5 min and 10 min (`internal/view/view.go`), range 1 min to 24 h,
  applied at once (`durationSettings` in `internal/server/settings_api.go`).
  `stale_after` (default 50 h) and `stale_after_wall` (24 h),
  range 1 h to 30 days, applied at once.
  `log.keep_days`: 0 to 3650, 0 keeping the log forever, default 30;
  a write prunes at once (`PruneEventsSoon`).
- **The rule.**
  The alert may not be under the warning; equal is allowed.
  A `PUT` is checked against the limits it leaves in effect,
  a key sent as `null` counting as its default;
  a refusal is a `400` in English.
- **The route.**
  `GET` and `PUT /api/v1/settings` need the capability `settings`:
  by default the owner's and the integration role's, not a manager's.
  A `PUT` takes any subset of keys.
  Each duration answers its value, default, value in effect and `fixed_by`,
  the flag that decides it instead (`Options.SettingsFixedBy`).
- **The Go tests** of the window limits are there (`TestWindowSettings`,
  `TestWindowTimes`).

### For Block 1: the Maintenance page

- **The pattern.**
  The Maintenance card (`src/maintenance/Maintenance.tsx`, `store.ts`):
  each Cube interval is a `Select` of steps saved at once (`useSetAtOnce`),
  its default sent as `null`, a refusal shown on a `ResultLine`,
  a hint line under the row;
  the region follows the stream's `change` events of its keys.
- **Who opens the page.**
  `systemOf` (`src/shared/me.ts`) and `hasSystem` (`login.js`),
  which must agree line for line,
  list the cards' capabilities and leave out `settings`;
  so a custom role with `settings` alone cannot open the page its rows are on
  (D6-A).
- **Where the limits take effect.**
  The page never reads them:
  the server writes each device's `window.warn_at` and `window.alert_at`,
  and the page compares them with its clock (`src/model/device.ts`).
  After a change the header's Cube entry follows at its next poll,
  but the tiles keep the old times until the rooms are read again,
  which nothing does while the stream is open (D5-A).
  The stale flags behave the same way.
- **Tests nearby:**
  `test_cube_intervals.py`, `test_maint_updates.py`,
  `test_refresh_now.py` (roles), `test_system_layout.py`,
  `test_parts_guard.py`, `test_rooms.py` (the window signs).
  #473 changes most of them and the kit's idioms:
  the new test file follows the kit as #473 leaves it.

### For Block 2: the rest of item 6

- **No row** for either stale threshold or `log.keep_days` today.
- **`-stale-after`** fixes both stale thresholds for a run
  (`cmd/massimo/serve.go`), and `GET` names it in `fixed_by`;
  but a `PUT` of either key still calls `SetStaleAfter` or `SetStaleAfterWall`,
  so the value in effect changes while `fixed_by` says the flag decides it
  (D8-A).
- **Lowering `log.keep_days`** prunes the log at once;
  the question of `shared/question.tsx` (`useAsk`, `Question`) is the shared
  element for asking first.

## Implementation Plan

Each block is one pull request against `main`, built on a branch from
`origin/main` once #473 has merged, Block 2 on Block 1's branch;
a second reader reads each block before its pull request;
the owner merges.

### Block 1: The window limits

- **Goal:**
  the two window limits set on the Maintenance page, in a card of their own,
  and the tiles' window signs following a change at once (D2-A, D3-A, D4-A,
  D5-A, D6-A).
- **Changes:**
  1. A card of its own after the Maintenance card on the Maintenance page:
     its frame in `ui/partials/` with its title and help slot,
     its part drawn by the maintenance region (`src/maintenance/`),
     shown to whoever holds `settings`, as the intervals' rows are.
     Its title is "Limits", a name that holds once Block 2 adds its rows.
  2. Two rows, "Window open: warning after" and "Window open: alert after":
     each a `Select` saved at once, as the Cube intervals are,
     its steps 1, 2, 3, 5, 10, 15, 20, 30 and 45 min, 1, 2, 4, 8, 12 and 24 h,
     its first choice the default, named with its value and sent as `null`;
     a stored value that is no step shown as itself;
     a refusal on a `ResultLine`; a hint line under each row.
  3. The steps the server would refuse are disabled (D4-A):
     in the warning's row each step above the alert in effect,
     in the alert's row each step under the warning in effect,
     the default choice included; equal stays allowed.
  4. At a `change` of either key, whether made on this page or another,
     the page reads its rooms again (D5-A),
     through the load that `app.js` already has.
     The builder checks who receives the stream's `change` of a setting:
     where a page without `settings` does not,
     the block says so and leaves the server as it is.
  5. `settings` added to `systemOf` (`src/shared/me.ts`) and `hasSystem`
     (`login.js`), line for line (D6-A).
  6. The texts' keys in all six `ui/i18n/<lang>.json`,
     FR, IT, NL and PL machine-translated and marked.
- **Tests:**
  a new `tools/ui-test/test_window_limits.py`:
  the card's two rows show the values in effect;
  a step chosen sends the exact `PUT` body, held at its route,
  and the default sends `null`;
  the refused steps are disabled in both rows, equal allowed,
  the default choice disabled where its value would be refused;
  a refusal shows on the row's `ResultLine`;
  a role without `settings` sees no card;
  a custom role holding `settings` alone opens the Maintenance page and the card;
  an open window's sign on its room's tile follows a change of the warning
  without a reload (the `clock` marker, as the window signs' tests of
  `test_rooms.py` set it).
  The whole suite once in one case (`-k 1100-en`) on the committed tree,
  since the card adds a part to a page other files test.
- **Documents:**
  `docs/ui.md` (Maintenance: the card and its rows; Signs:
  a window sign follows a change of the limits; The stream:
  a `change` of a window limit reads the rooms again; How the page is built:
  the part); `docs/api-v1.md`, at a device's `window`:
  `warn_at` and `alert_at` follow a change of the limits at once,
  and a client reads the devices again to see it;
  the Maintenance page's screenshots retaken (`make ui-screenshots`).
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-serverlimits-window-limits.md`:
  "- The window limits, how long a window may stay open before its sign turns
  amber and then red, are set on the Maintenance page."
- **Commit:**
  `feat(ui): The window limits set on the Maintenance page`
- **Verification,** each by its exit code:
  `make ui`; `gofmt -l .` printing nothing;
  `go vet ./...` for linux, windows and darwin;
  `MASSIMO_ENV_FILE=/dev/null go test ./...`;
  `make cross-check`; `make ui-check`;
  `make ui-test` of the changed and new test files in all four cases,
  and of the whole suite in one case;
  `reflow.sh --check` on both documents;
  `changelog.sh check --staged --strict`;
  `GUIDELINES/shared/tools/check` before the push.

### Block 2: The stale thresholds and the log's days

- **Goal:**
  the stale thresholds and `log.keep_days` set in the same card,
  a lower `log.keep_days` asked first,
  and a flag that fixes a stale threshold winning over a `PUT` (D1-A, D5-A,
  D7-A, D8-A).
- **Changes:**
  1. Server (D8-A, a commit of its own first):
     a `PUT` of `stale_after` or `stale_after_wall` while `fixed_by` names
     the flag is stored and not applied;
     `in_effect` stays the flag's, and the stored value takes effect at a
     start without the flag.
  2. Three rows in the card:
     "Radiator thermostat unreachable after", steps 12, 24 and 36 h, 50 h,
     3, 4, 5, 7, 14 and 30 days;
     "Wall thermostat unreachable after", steps 6, 12, 24, 36 and 48 h,
     3 and 7 days;
     "Keep the log", steps 14, 30, 60, 90, 180 and 365 days, 2, 5 and 10 years,
     and Forever (0).
     Each a `Select` saved at once, the default first and sent as `null`,
     a stored value that is no step shown as itself.
  3. A stale threshold that `fixed_by` names is shown read-only,
     its hint naming the flag.
  4. A `log.keep_days` choice under the value in effect asks first,
     with the shared `Question`, saying that the older entries go at once;
     No leaves the value as it was (D7-A).
     Forever and a higher choice are saved at once.
  5. At a `change` of a stale threshold the page reads its rooms again,
     as Block 1 does for the window limits (D5-A).
  6. The texts' keys in all six `ui/i18n/<lang>.json`.
- **Tests:**
  Go, `internal/server`: with `SettingsFixedBy` naming the stale thresholds,
  a `PUT` of each answers `200`, stores the value and leaves `in_effect`
  as the flag set it; without the flag it applies at once, as before.
  Seen failing before the change.
  UI, `test_window_limits.py`,
  renamed `test_limits.py` where the builder finds that clearer:
  the three rows' values and their exact `PUT` bodies;
  a fixed stale threshold read-only, its hint naming the flag;
  a lower log choice asking, No sending nothing, Yes sending the value;
  a higher one and Forever sent at once;
  a stale flag on a tile following a change of its threshold without a reload.
  The whole suite once in one case on the committed tree.
- **Documents:**
  `docs/ui.md` (Maintenance: the three rows, the question);
  `docs/api-v1.md` (settings: a key a flag fixes is stored and applies at a
  start without the flag);
  `README.md` at `-stale-after`, where its text says otherwise;
  the Maintenance page's screenshots retaken.
- **CHANGELOG:**
  `CHANGELOG.d/Fixed/claude-serverlimits-stale-after-fixed.md`:
  "- With `-stale-after` given,
  a stale threshold set through the API no longer changes the value in effect:
  the flag decides, as the settings say.";
  `CHANGELOG.d/Added/claude-serverlimits-stale-and-log.md`:
  "- The stale thresholds and the days the log keeps are set on the Maintenance
  page; a shorter log asks first."
- **Commits:**
  `fix(server): A stale threshold that -stale-after fixes stays the flag's`,
  then `feat(ui): The stale thresholds and the log's days set on the Maintenance page`.
- **Verification,** each by its exit code: as Block 1's,
  and `MASSIMO_TEST_SCALE=2 go test -race -count=1 ./internal/server`.

## Proposed commit message

The plan: `chore(aps): +ServerLimits v1.0`

Block 1:

```
feat(ui): The window limits set on the Maintenance page

AP ServerLimits v1.0, Block 1 (D2-A to D6-A): the two window limits, how long a window may stay open before its sign turns amber and then red, are set on the Maintenance page, in a card of their own, each a list of steps saved at once. The steps the server would refuse are disabled, so that the alert is never under the warning, and the page reads its rooms again at a change, so that the tiles' signs follow at once. A custom role holding settings may now open the page.

Key changes:
- internal/server/ui: the Limits card and its two rows; the rooms read again at a change of a window limit; settings in systemOf and hasSystem; the texts in six languages
- Tests: test_window_limits.py
- Docs: docs/ui.md, docs/api-v1.md, the Maintenance screenshots; an Added fragment

Assisted-by: claude-code:claude-opus-5-5
```

Block 2:

```
fix(server): A stale threshold that -stale-after fixes stays the flag's

AP ServerLimits v1.0, Block 2 (D8-A): with -stale-after given, a PUT of stale_after or stale_after_wall changed the value in effect while the settings named the flag as what decides it. It is now stored and applies at a start without the flag.

Key changes:
- internal/server/settings_api.go: a stale threshold a flag fixes is stored, not applied
- Tests: a PUT of each under the flag leaves in_effect as the flag set it
- Docs: docs/api-v1.md; a Fixed fragment

Assisted-by: claude-code:claude-opus-5-5
```

```
feat(ui): The stale thresholds and the log's days set on the Maintenance page

AP ServerLimits v1.0, Block 2 (D1-A, D5-A, D7-A): the Limits card takes the two stale thresholds and the days the log keeps, each a list of steps saved at once. A threshold that -stale-after fixes is shown read-only, a shorter log asks first since the prune cannot be undone, and the page reads its rooms again at a change of a threshold.

Key changes:
- internal/server/ui: three rows in the Limits card; the question before a shorter log; the texts in six languages
- Tests: the rows, the fixed threshold, the question, the stale flag following a change
- Docs: docs/ui.md, the Maintenance screenshots; an Added fragment

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-27): first revision,
  from the owner's "PLAN D2's fix on `main`" and, at its TL;DR gate,
  "PLAN++ all A": D1-A to D8-A taken.
