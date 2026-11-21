# ANALYSIS LogEdges v2: the five log findings left by AP LogCursor

Read on main at 3f7a7e41, read only, by two Opus sub-agents of the session
that ran AP LogCursor; their probes ran through `go test -overlay` from
the session's scratch directory and are not kept.
The findings are the five of LogCursor's NOTE, "Found, not changed".

v2, at 1d5a3500, after the feedback of session "UI refactor":
finding 1 is fixed on main by UiParts Block 11 (#396);
the log's id comment Block 1 changes is now in the part, `src/log/store.ts`;
what Block 1 does to the 409, which the part relies on, is stated.

## TL;DR

Two of the five are worth a fix of a few lines,
one is a test that can pass when it should fail,
one is fixed on main by another plan, and one is harmless.

- **1. A Cube's log after a stream resync** → real, display only;
  fixed on main by UiParts Block 11 (#396, UiParts.D208) → no block here.
- **2. Unknown log level** → the API already answers 400;
  the one silent typo is `MASSIMO_LOG_LEVEL` read again while serve runs,
  which stops the next start → a warning at the reload.
- **3. `TestNewerWriteReplaces`' sleep** → a planted fault passes it →
  the log cursor, read to the re-send's report.
- **4. Ids reused after a prune to empty** → real, rare;
  the alert sign goes silent for months → the prune keeps the newest row,
  one line, no migration.
- **5. A Pi without a clock** → systemd and timesyncd set the clock forward
  before serve starts → a sentence in `docs/store.md`.

### Blocks

1. Store: the prune keeps the log's newest row (4), and the clock's sentence (5)
2. Reload: a log level or format that does not parse is warned of (2)
3. Test: `TestNewerWriteReplaces` reads on to the re-send's report (3)

## 1. A Cube's log after a stream resync

- **Confirmed.**
  The resync handler (`internal/server/ui/app.js:898-903`) calls `burst()`,
  `restoreResynced()`, `serverEvent({kind:"resync"})` and `refresh()`,
  nothing of the log;
  a Cube's own view takes new lines only from the stream (`log.js:979-984`),
  and a resynced stream replays nothing (`server.go:2145-2154`).
- **When:** the server cannot resume from `last_event_id`
  (`resume.go:132-153`): a restart, an update or a reboot (new epoch);
  the Cube's runtime made anew (`runtimes.go:113`);
  more than 1024 of the Cube's events since that id
  (a laptop asleep, a phone's tab in the background).
- **Seen:** only with the log page open.
  The gap's lines (`server_stopping`, `server_started`, `connected`, …)
  are missing with no sign of a gap; Older does not fill it;
  showing the log again, a choice, Clear or a reload brings them back.
- **All Cubes has no gap:** its poll reads `after=` every 10 s
  (`log.js:1150-1181`).
- **Severity:** low, display only.
- **Fixed on main** by UiParts Block 11 (#396), in a commit of its own,
  "fix(ui): A Cube's log reads on when the stream starts over"
  (UiParts.D208): at a resync a Cube's own log shown reads forward,
  `after=<the newest stored id held>`, page after page with the poll's
  limits, a 409 or a longer backlog leading to a load;
  held back while the page's own load, Older or read-on runs,
  and run once after it;
  where the log holds no stored line (a load that failed, a log that loaded
  empty), it loads instead.
  Its tests are in `tools/ui-test/test_log_updates.py`;
  the Older pages, the place and what is open are kept.
  The finding's lines above are of the plain `app.js` and `log.js` before it.
- **Nothing left here;** a stopgap in `log.js` is moot.

## 2. `store.ParseLevel` reads an unknown level as info

- **Confirmed, partly:** an unknown name returns `(0, false)`, and 0 is
  `LevelInfo` (`internal/store/events.go:63, 86-98`);
  every caller reading outside input checks `ok`.

| Caller | A typo today | Silent |
|---|---|---|
| API `level=`, both log routes (`log.go:165-170`) | 400 `bad level "warnn"`; tested (`logsources_test.go:94`); also takes any case and an undocumented `warning` | no |
| The server's own levels (`log.go:62`, `:449`, `eventline.go:174`, `logalerts.go:209`) | only the 40 literals the code sets reach it | only a code typo |
| Command line and env file, `parseLogLevel` (`cmd/massimo/logging.go:30-42`) | `-log-level deubg` exits; `MASSIMO_LOG_LEVEL=deubg` stops serve at its start, and systemd restarts it into the same error every 10 s | no |
| **Settings file read again** while serve runs: file watch, SIGHUP, `POST /api/v1/reload` (`reload.go:130-158`) | taken with an Info line, `at_next_restart=MASSIMO_LOG_LEVEL`, no warning; System info shows "invalid" to whoever looks; the next restart leaves the server down | **yes** |
| UI address `#log?level=warnn` (`pages.js:85-86`) | falls back to info, as range, kind and device do | by design |

- **Clients:** Home Assistant and the app call no log route;
  nothing changes for API clients.
- **Fix:** at the reload, a Warn line where `MASSIMO_LOG_LEVEL` or a log format
  does not parse (the start refuses both, `logging.go:115-123`);
  it reaches the process log and the API's log, so the log's warning sign
  shows it.
  About 10 lines in `cmd/massimo/reload.go`, a test beside
  `TestSettingsReloader`, a sentence in the README's reload paragraph,
  a `Fixed` fragment.
- **Not proposed:** one parser for both (dropping the API's `warning` breaks
  it); refusing `POST /api/v1/restart` while a bad value waits
  (an API change for a rare case).

## 3. `TestNewerWriteReplaces` checks with a fixed sleep

- **Confirmed:** `internal/server/confirm_test.go:169-190`,
  a fixed 300 ms at `:186`, then exactly one `not_confirmed` line expected.
- **It passes when it should fail:** with the re-send's report written as
  `not_confirmed` (`confirm.go:421`), the second warning comes about 600 ms
  after the first, after the sleep; the test passes
  (`TestRoomCommandNotConfirmed` and `TestUnconfirmedRoomCommandResent`
  catch it).
  What the test is about, the older write replaced, fails before the sleep.
- **No flake:** 20 runs under `-race` at scale 2 green.
- **Fix:** the log cursor with an end: read to the re-send's report
  (`resend_not_confirmed`), then `logLines` finds no second `not_confirmed`.
  It caught all three planted faults, 20 runs green;
  0.94 → 1.24 s, since it waits out the re-send's deadline.
  `confirm_test.go` only; no docs; no fragment ("only tests change").
- **A sibling, not proposed:** `resend_test.go:355` sleeps 200 ms to show no
  re-send follows a proxy write; nothing follows that report,
  so there is no end to read to.

## 4. Ids reused after a prune to empty

- **Confirmed:** `log.id` is `INTEGER PRIMARY KEY` without AUTOINCREMENT
  (`store.sql:181-182`, `docs/store.md:659-662`).
  The only thing that deletes log rows is the retention prune
  (`events.go:491`); no route clears the log, rebuilds and the downgrade keep
  the ids.
  Probe: 1010 old rows pruned, the next row gets id 1;
  once 1031 new rows exist, a page after 1010 starts at 1011,
  and rows 1–1010 never reach that client.
- **When:** a prune where the newest row is older than `log.keep_days`
  (30 by default):
  - a start after the box was off that long; the start prune runs before the
    first batch, and reuses unless the writer's 1 s timer lands mid-prune
    (id 1 at 1,000 and 20,000 rows on a PC, not at 90,000);
  - a running server that logged nothing that long,
    or `keep_days` lowered below the gap.
  Rare, never in an active house.
- **Effect:**
  - **the seen marks:** `log.seen.all` keeps the old high id
    (`logseen.go:61`, `logalerts.go:394`), so every new warning at or below
    it is hidden, and the log's sign stays quiet until the new ids pass the
    old mark, about as long as the store's history took;
  - **the room-master mismatch** counts a reused id as seen at once and warns
    again every minute while the finding holds (`congruence.go:72`,
    `:383-413`, `:503-533`);
  - the web UI: a 409 and a reload, harmless;
    but the log's part keys a line by its Cube's serial and the event's id,
    so an id given out twice could let one event inherit another's open
    detail or focus;
    Home Assistant reads no log, and the app is the web UI.
- **Severity:** rare, medium where it happens.
- **Fix:** the prune keeps the log's newest row,
  `AND id < (SELECT max(id) FROM log)`: max(id) never falls, so no id is given
  twice; the kept row goes at the next prune once anything newer exists.
  No schema change; older binaries are unaffected.
  Probed: next id 1011 after a full prune; the start's race no longer reuses;
  the store and server suites pass.
  Cost: one line kept past `keep_days`.
  Files: `events.go` (the DELETE and its comment at `:562-565`),
  comments in `log.go:361-365` and the log part's `logGone`
  (`internal/server/ui/src/log/store.ts:424-433`),
  a test beside `TestEventsPruned`, `docs/store.md:659-662` and `:693`,
  `docs/api-v1.md:3299`, a `Fixed` fragment.
  It reverses LogCursor.D10-A ("documented, left").
- **What it does to the 409**, which the part relies on in three places
  (the poll, the read-on at a resync, the check before Older):
  one case goes, the log emptied by the prune,
  since a cursor once given out never again lies beyond the newest id.
  The others stay: a store restored from an older copy,
  a Cube that gains its row in the store, a server without a store
  restarted with its clock set back.
  The part needs no change; `logGone`'s comment loses that case.
  The part is also closed against an id given out twice.
- **AUTOINCREMENT instead:** migration 0026 rebuilding `log`,
  min_reader kept at 24, a down part under StoreRaise.D1-D that remakes
  `log` exactly as `store.sql` has it and drops its `sqlite_sequence` row;
  1.7 s at 90,000 rows on a PC, perhaps a minute on a Pi 1, unmeasured;
  the WAL grows by about 66 MB once.
- **An epoch in the cursor or `/healthz`:** catches a restart or a restore,
  not a prune while running, and nothing for the seen marks.

## 5. The ring's ids on a Pi without a clock

- **Confirmed, partly:** the ring's seed is the wall clock with no floor
  (`server.go:813`), and no unit orders on the clock
  (`packaging/systemd/massimo.service:8-9`; the deb's unit is the same file;
  the plist and the Windows service have none, on machines with a clock).
- **But a Debian or Raspberry Pi OS box never starts at 1970:**
  systemd sets the clock forward at boot to its build time or
  `/usr/lib/clock-epoch`;
  systemd-timesyncd, ordered before `sysinit.target`, to its clock file;
  Raspberry Pi OS also ships fake-hwclock.
  The seed is at least about 1.6×10^15, against store ids of 10^5–10^6.
- **What breaks:** nothing reachable.
  A Cube that gains its row gets a 409 and a reload, as designed;
  a cursor from an earlier run that is ahead gets a 409;
  one that is behind gets the new ring, nothing repeated;
  the seen marks use store ids only.
- **Fix:** a sentence in `docs/store.md`:
  a clock behind the previous run gives a 409 and a reload.
- **Not proposed:**
  `After=time-set.target` changes nothing where timesyncd runs;
  `After=time-sync.target` does nothing on a default install and,
  where `systemd-time-wait-sync` is enabled, never starts a box without an
  NTP server;
  a seed above the store's newest id is already 9 orders of magnitude there.

## Where it meets other plans

- **UiParts:** finding 1 was fixed by its Block 11 (#396).
  Block 1 here changes one comment in `src/log/store.ts`;
  UiParts' Block 15, not started, removes `log.js` and touches `src/log/`:
  whoever lands second carries the other's change.
  Nothing else meets: Block 11 touched no Go but `ui.go` and `ui_test.go`.
- **StoreRaise:** only under AUTOINCREMENT, not proposed.
- **LogCursor:** Block 1 reverses its D10-A;
  LogCursor retires with UiParts (D1-C), so its files are not touched.
- **Packaging, MobileApp:** not touched.

## Open decisions

**D1. A Cube's log after a resync** - settled: A, fixed by UiParts Block 11
(#396).

**D2. `ParseLevel` and the API's `level=`.**
- **A (recommended):** no change; the API already answers 400.
- **B:** one parser for both; dropping `warning` breaks the API.

**D3. A bad log level in the settings file read again.**
- **A (recommended):** a Warn line at the reload, since today the typo is
  silent and stops the next start.
- **B:** also refuse `POST /api/v1/restart` while it waits; costs an API change.
- **C:** nothing.

**D4. The sleep in `TestNewerWriteReplaces`.**
- **A (recommended):** the log cursor read to the re-send's report; the
  documented pattern, and it catches the second warning; 0.3 s more.
- **B:** drop the sleep and the count; proves only the replacement.

**D5. Ids reused after a prune.**
- **A (recommended):** the prune keeps the newest row; one line, no migration.
- **B:** AUTOINCREMENT by migration 0026; costs a rebuild at the upgrade and a
  down part.
- **C:** leave it documented; costs the silent alert sign and the minute's
  warning loop where it happens.

**D6. The unit and the clock.**
- **A (recommended):** no unit change, a sentence in `docs/store.md`.
- **B:** `After=time-set.target`; a shipped change with no effect where
  timesyncd runs.
- **C:** `After=time-sync.target`; a box without NTP may never start.

**D7. An epoch per run for log cursors.**
- **A (recommended):** no; the 409 covers what D5-A leaves.
- **B:** yes, in `/healthz` with a UI reload; costs an API field, a feature
  flag and UI tests.

**D8. Where the work goes.**
- **A (recommended):** a plan of its own, AP LogEdges, its three blocks in one
  pull request, a commit each, since each is a few lines.
- **B:** AP LogCursor v1.1; costs reopening a plan whose retirement now lies
  with "UI refactor".
- **C:** a pull request per block; three CI runs for a few lines each.
