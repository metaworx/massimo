# NOTE LogCursor v1.0: how the run went

Where Blocks 1 to 3 went differently from the plan, what verified them,
and what the run found beside them.

## Block 1, Product (pull request #383)

1. **The analysis was wrong on one point:**
   a page of the log is several walks, one per Cube and the server's,
   each its own statement, and the event writer can commit between two of them,
   so a page forward could skip a line for good;
   the second reader's probe lost 40 to 200 of 6000 lines in every run.
   `store.Events` now reads the newest id after the flush and bounds every walk
   of a page by it, backward pages of several walks too,
   since a client's first cursor is the top of such a page.
2. **The bound is a filter, not a range:**
   as a range SQLite took the Cube's index in place of the kind's,
   source's or serial's, 1.3 → 16.7 ms a page at 90,000 rows;
   `TestEventsPageQuery` holds each walk to its own index.
3. **The 409 moved into the store** (`store.ErrBeyondNewest`),
   from the same read of the newest id.
4. **Beyond the plan:**
   cmd/massimo's Windows stop test waited for a line with an id as a sign the
   line was stored, which the rings' ids made untrue;
   it waits for a stored `connected` line.
   A Cube without a row on a server with a store got its own case in the tests.
5. **A test fixed after the review:**
   the store's test of paging while a writer writes waited 20 s,
   which a machine at load 19 outran, and its writer then panicked the binary;
   it now reads until its writer ends.
   Block 2's reader found it; it went into #383 before the merge.
6. **CI:**
   `test_scroll.py::test_a_jump_down_stays_where_it_went[1100-en]` failed twice,
   on the room configuration's week programme left by earlier tests,
   and passed on a third run and in a local run of the whole case;
   AP UiParts' region, not the log.

## Block 2, Tests (pull request #387)

1. **The counts moved:**
   33 waits onto the cursor, six of them helpers with 73 callers;
   16 read at once on the start's barrier, where the plan counted 12; one kept,
   Block 1's own test of paging while writers write.
2. **Four helpers count from the log's start,** not from a mark:
   their callers mean the log's whole count.
3. **The second reader found two waits that lost an exact count**
   (TestAutoReset, TestSameConfigAtDebug): a line written twice passed;
   both check that none follows, and three faults of the cursor that
   `TestLogCursor` missed are covered.
4. **The package takes as long as before,** plain and under race.

## Block 3, Ui (pull request #388)

1. **Beyond the plan, Older asks first:**
   a Cube's own view follows the stream and never polls,
   so the poll's 409 would never catch a Cube that gains its row in the store
   while its log is open, its ids moving from the memory's to the store's;
   Older now asks `limit=1&after=<the newest id>` before it pages,
   and loads the log again on a 409.
   A guard by time was weighed and left: times do not follow ids.
2. **The poll reads page after page, ten at most,**
   and adds the lines once at the end;
   a backlog past ten pages, after a computer slept, loads the log once.
3. **No `log_after` flag in the UI:**
   the app serves only its connect page from its own files,
   so a page meets only the server that served it;
   lines at or below the cursor are dropped,
   which makes a downgrade's window harmless.
4. **`docs/ui.md` carries no version,** so none was raised.
5. **UiParts' Block 11 had not landed;** it carries the change into its part.

## Found, not changed

1. In a Cube's own view, the stream's resync reloads the page's data but not
   the log (`app.js`), so the lines of the gap are missing until a reload.
2. `store.ParseLevel` reads an unknown level as info.
3. `TestNewerWriteReplaces` still checks that nothing more came with a fixed
   sleep.
4. Without AUTOINCREMENT, a log pruned to empty gives out its ids again from 1:
   a client that asks only once the new ids have passed its cursor skips them
   unseen (documented, D10-A), and the alerts' seen marks share the reuse.
5. A Pi that boots at 1970 without a clock gives its rings small ids,
   which can equal the store's;
   `massimo.service` has no `After=time-sync.target`.

## Coordination

Session "UI refactor" was told, on the owner's word,
where the blocks meet AP UiParts: the OpenAPI file and `api.d.ts`,
the server's test files, and `log.js` against its Block 11,
whoever lands second carrying the other's change.

## Verification

Each block: a builder, an independent second reader whose points were fixed
before the commit, faults planted for each new or moved test,
`-race` at scale 2, the guidelines check, the pull request's run green.
