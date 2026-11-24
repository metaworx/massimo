# ANALYSIS BackupWatch v2: backups after restarts, on today's main (2026-11-24)

The owner's report (2026-11-15):
"Backup and restore: seems to be empty after each boot. where are the older backups?"
The owner's answer to the first gate (2026-11-24):
"restart from `main`"; D1-C, D2-A, D3-A; "EVAL D4:B".
This revision re-checks every finding on `origin/main` (`chore(aps): +UpdateHistory v1.1`,
846 commits after the first look) and weighs D4-B.

## TL;DR

On main, the page's own defect is now worse than on 11-15.
Every load or reload straight onto System settings draws the Backup and restore card
with **an empty list**, whether or not the Cube is stored.
The page reloads itself after each update, so this is "empty after each boot".
The server side is as before:
nothing is lost, and every start resets the pending backup's timer.

- C1 (only after a change): unchanged.
- C2 (a start resets the windows): unchanged;
  `internal/server/backups.go` is the same file.
- C3 (UI): reproduced on main, 8 cases; every load on `#system` → empty card.
- C4 (an unserved Cube's backups out of reach): unchanged;
  the command line cannot reach them either.
- Settled: D1-C, D2-A, D3-A. D4-B weighed: **recommended now** (it was A), scoped (§3).
- New: D5, how the page learns which unserved Cubes have backups.

### Blocks

1. Server: the pending backup's windows kept in the store,
   and the backup taken at a clean stop (D1-C)
2. UI: the lists load once the page knows its Cubes,
   and the "unchanged since" line (D2-A, D3-A)
3. Server and UI: the backups of a Cube no longer served,
   to list, download, compare and delete (D4-B, D5)

## 1. What main changed, finding by finding

**The evidence of 11-15 stands.**
breno's store held all 8 backups ever taken (ids 1–8, sequence 8, no tombstones,
no prune or delete).
The page always received its Cube's list,
and a device removed at 00:32 got its backup at 02:01,
not 01:32, because of the starts at 00:33 and 00:59.
breno was not looked at again for this revision.

**C1, only after a change:** same rule, same documents.

**C2, a start resets the windows:**
`backupWatch` still keeps `dirtySince` and `lastChange`
in memory only; no commit since touched `internal/server/backups.go` or `backups_api.go`.
The store now keeps each file in `backup_data` (migration 0024, schema 24),
which changes nothing here.

**C3, the page draws before it knows its Cubes:** the UI is now TypeScript parts
(AP UiParts).
`9f06a87f feat(ui): Backup and restore per Cube, in Cube settings and System settings`
(11-15 12:39, after breno's build of that morning) made the card a fold per Cube.
`src/backups/index.tsx` loads the lists in `onView("system")`
for `state.cubes` as they are at that moment.
`app.js` `init` still shows the address's page (`MassimoParts.pages.showAddress()`)
before `refresh()` has read the Cubes.
On a load onto `#system`, `state.cubes` is empty, so no list is read.
The folds then come with the Cubes, each with `loaded` false,
and nothing reads their lists.
A box that has not loaded draws no row and no "No backups kept yet.",
so the card is just empty.

Reproduced with a build of main (`make ui`, then `go build`), the fake Cube
and two backups taken by hand, in Playwright:

| Case | Rows shown |
|------|-----------:|
| A. first open on `#system`, no Cube stored | 0 |
| B. reload on `#system`, Cube stored | 0 |
| C. reload on `#system`, stored Cube removed | 0 |
| D. open on `#cube` (Cube settings), Cube stored | 2 |
| E. reload on `#cube`, stored Cube removed | 0 |
| F. open on the tiles, then System settings | 2 |
| G. page kept open on System settings across a restart of serve | 2 |
| H. reload on `#system` after that restart | 0 |

So since 11-15 12:39 the card is empty after every reload onto System settings,
including the reload `checkVersion` makes after an update. It is filled once the page is
opened again from the menu. Cube settings is empty only without a stored Cube (E).

**C4, an unserved Cube's backups:** `s.cube(r)` still needs a served Cube,
so `…/backups` of a row with `configured` 0 answers 404.
`massimo backup` reads a live Cube and `massimo store backup` copies the store file:
neither lists the stored backups of a Cube. Only `sqlite3` reaches them.

## 2. Settled

- **D1-C.** Both: the windows kept in the store (`dirtySince`, `lastChange` and the hash
  they were seen at, per Cube), and the pending automatic backup taken at a clean stop
  (update, restart, SIGTERM) before the store closes, within a few seconds.
  The plan has to put the stop's backup before the runtimes and the store close
  (`internal/serve`'s shutdown), and skip it for a Cube that is not connected.
- **D2-A.** In this plan. On main the fix is: a box reads its list when it is drawn
  and has not yet loaded (or the part reads every Cube's list once `refresh()` has brought
  them while System settings or Cube settings is shown).
  UI tests for the cases A, B, C, E and H above.
- **D3-A.** The line under a Cube's list while it is clean ("Unchanged since the newest backup:
  the server takes one after a change"), through `t()` with its key in all six languages,
  in `src/backups/Backups.tsx`.

## 3. D4-B weighed: the backups of a Cube no longer served, in this plan

**For.**
- The main use of these backups is a Cube that was replaced or taken out of the configuration.
  Its backups are its configuration (rooms, devices, week programs, pairings),
  the very thing a new Cube is to receive.
- Today they are kept, and retention keeps the newest 10 of each Cube forever.
  Yet nothing in Massimo shows them, and only `sqlite3` gets them out.
  On breno that is the earlier Cube's backup of 11-09.
- Once downloaded, the existing "Restore from file" puts such a backup onto the new Cube.
  So list and download are enough; no new restore path is needed.

**Against.**
- About one block: a route change, the OpenAPI file, a UI fold, Go and UI tests.
- The backups routes become the one place where `/cubes/{sn}/…` answers for a Cube that is not
  served. Everything else answers 404 there.

**Scope recommended (D4-B as the plan would build it):**
- **Server.** `GET …/backups`, `GET …/backups/{id}`, `…/diff` and `DELETE` find the Cube
  by its store row when no runtime serves it.
  `POST …/backups` and the restore routes keep needing a served, connected Cube:
  for a row alone they answer 409 "not served".
  The list of such a row carries no `changed`/`due`.
- **UI.** System settings → Backup and restore shows, after the served Cubes,
  a fold for each Cube row that is not served and has backups, marked "not served".
  Its box has Download and Compare, Delete for `restore`, and no Take or Restore.
  Rows without backups get no fold, so the found Cubes and proxies on breno add none.
- **Docs.** `docs/api-v1.md` and the OpenAPI file for the routes;
  `docs/ui.md` and `docs/store.md` §9 (e) for the fold.
- **Not in it:** restoring one Cube's stored backup straight onto another
  (`restore/plan?backup=` of another row). That is a decision of its own, for later.

**Recommendation: B, scoped as above.**
The data is kept and worth most at exactly the moment it is out of reach.
The cost is one block, and the third block of a plan already touching both the routes
and the card.

## 4. Open

- **D4.** Answered above: **B (recommended)**, scoped. **A**, not now, leaves the backups
  reachable through `sqlite3` only.
- **D5. How the page learns which unserved Cubes have backups (with D4-B)**
  - **A (recommended).** A `backups` count per Cube row in `GET /api/v1/cubes`,
    one grouped count in the store.
    The page reads that list already, and a fold is drawn only where the count is above 0.
    Cost: one field in the API, the OpenAPI file and a test.
  - **B.** A new `GET /api/v1/backups` with every row's list in one answer.
    It would also spare System settings its one request per Cube.
    Cost: a new route, its tests and its OpenAPI entry.
  - **C.** A request per found Cube, as for the served ones.
    Cost: a 404 or an empty answer for every found Cube and proxy at each opening.

## 5. How this was checked

- The worktree detached at `origin/main`. The UI was bundled with `make ui`
  (`parts.js`, ignored), and `massimo` was built from it.
- A local serve and the fake Cube on loopback ports, with a scratch data directory
  under `GUIDELINES/temp/`.
- Nothing on breno, nothing sent to a real Cube.

<!-- MWX-CONFIG-v1:
[headings]
	numbered = false
-->
<!-- The numbered sections are named by their numbers in the text; the
     TL;DR stands outside the count. -->
