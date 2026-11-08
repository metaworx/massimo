# AP FoundCubes v1.0: removed and gone Cubes

## TL;DR

Two faults the owner found on a Raspberry Pi with v0.12.0-alpha.1:
a removed Cube stays in the Cube menu as configured and disconnected,
and a search never drops a Cube it found before.
Both sit in the server's found map (`internal/server/discovery.go`);
the web UI only shows what `GET /api/v1/cubes` says.

- found entry's `configured` → kept from the last discovery answer or the
  store, stale once the Cube is removed → set when the entry is handed out
- found list → every Cube that ever answered → the Cubes that answered the
  latest complete run (D1-A)
- after a restart → the store's found Cubes at once → none until the first
  run (D2-A)
- a Cube found, a Cube gone → one function each, `cubeFound` and `cubeGone`,
  where the Logging plan hooks its events
- reproduced on all three routes with the fake Cube; no UI change

### Blocks

1. RemovedFound: the found list's `configured` flag, a test per route,
   the CHANGELOG
2. LatestRun: a complete run decides the found list, found and gone in one
   place each, the docs, the CHANGELOG

## Discussion

**The owner's report** of 2026-11-08, from v0.12.0-alpha.1 on a Raspberry Pi:
after a Cube is removed under System settings → Cubes,
the header's Cube menu (`renderCubeSelect` in `internal/server/ui/app.js`)
still lists it as a configured Cube that is disconnected.
That Cube had been added from the discovery list in the web UI.
And a search keeps every Cube that answered once:
the owner wants a search, and the periodic discovery, to list only the Cubes
that answered that run, the others leaving the menu and System settings →
Cubes, their row in the store keeping `last_seen`.

**The owner agreed** to reproduce all three ways a Cube gets configured,
each with the fake Cube:
the discovery list in the UI (the owner's case),
`MASSIMO_CUBE_<SN>_ADDRESS` in a settings file,
and the plain `MASSIMO_CUBE_ADDRESS`.
The order of the plans is the owner's too:
FoundCubes first, then Logging (one plan with blocks), then AptUpdate.

**Open at the plan gate:**

- **D1. A Cube that no longer answers.**
  A (recommended): gone from the list at once, at the end of the first
  complete run it did not answer.
  B: listed greyed with "last seen …" for a day, then gone; its costs are
  in the Analysis.
  With D1-B the plan gets a v1.1 with a third block for the UI.
- **D2. The list after a restart.**
  A (recommended): empty until the first complete run of the new process;
  with the default interval that run starts with `serve`.
  B: the store's found Cubes until the first complete run;
  with `-discover-interval 0` that is until a search, which is fault 2 again.

**Names chosen here**, routine and not decisions of the owner's:
the blocks RemovedFound and LatestRun;
the functions `cubeFound` and `cubeGone` and the end of a run, `settle`;
an entry's states `loaded`, `present` and `gone`;
the tests named in the Implementation Plan;
the branch `claude/found-cubes`.

**Not in this plan:**

- log events for a Cube found, gone or at a new address:
  the Logging plan writes them, in `cubeFound`, `cubeGone` and `note`;
- a `cube_gone` event on the stream: the UI reloads the list at every poll,
  and Logging decides what the stream and the log carry;
- writing `last_seen` at every answer (see D1-B);
- the own-answer filter: a Cube that answers from serve's own host with a
  configured serial is taken for the proxy's own answer (`ownAnswer`).
  Only the fake Cube on loopback does that, and the tests work around it.

**Who runs it:** this session, after the owner's `EXEC`.
The plan's own commit goes straight onto `main`;
the work branch `claude/found-cubes` starts after it, from `origin/main`,
and becomes one pull request with a commit per block.

## Analysis

### Fault 1: the reproduction

Built from `main` at 680631c and run against `massimo fakecube` with the UI
suite's greeting (serial KEQ0000100), which also answered discovery on
127.0.0.1;
`serve` had a store of its own in a scratch directory,
`-proxy off`, `-discover-interval 0`, and the fake Cube as `-discover-target`.
Route 1 ran with `MASSIMO_ENV_FILE=/dev/null`,
routes 2 and 3 with a scratch settings file naming only 127.0.0.1.
Nothing reached a real Cube or a box.
What `GET /api/v1/cubes` said about the Cube:

| After | 1: discovery list | 2: `MASSIMO_CUBE_<SN>_ADDRESS` | 3: `MASSIMO_CUBE_ADDRESS` |
|-------|-------------------|--------------------------------|---------------------------|
| the add, then the removal, no restart between | found, `configured: false` | - | - |
| a restart, then the removal | **`configured: true`, no `connected`** | **the same** | **the same** |
| reading the settings file again | - | unchanged | unchanged |
| a search | found, `configured: false` | the same | the same |
| a restart | found, not served | the same | the same |

Routes 2 and 3 needed a search before the file named the Cube,
so that its row had a `last_seen`:
on loopback the answers of a configured Cube are skipped as the proxy's own.

**The cause:** `foundCube.Configured` is kept on the map's entry.
`note` sets it at every discovery answer (`rt != nil`),
and `loadFound` copies it from the row at the start.
`unconfigured` decides which entries to list from the runtimes,
but hands each one out with the flag it kept.
After a removal that flag is `true` until discovery hears the Cube again.
The UI's `isConfigured` (`c.configured !== false`) takes such an entry for a
configured Cube, and draws it disconnected since it has no `connected`.

On the owner's Pi either path sets the flag:
`serve` runs discovery at the start and every 30 minutes,
which notes every configured Cube that answers from the LAN,
and every restart reads the flag back from the row.
It lasts until the next run hears the Cube (at most 30 minutes by default,
or a search) or the next restart.
The test of the LAN path calls `note` with an answer from 192.0.2.10
(TEST-NET-1) for a Cube with a fixed address, which sends nothing.

**Nothing brings the runtime back:**

- `deleteCube` writes `configured = 0`, source `ui`,
  and `Seed` adds back only a row whose source is `discovery`
  (`TestSeedFoundAndRemoved`);
- at the start the plain address's lines go to the serial learned before
  (`seed.plain_serial`), and `Seed` leaves that removed row alone too;
- reading the settings file again (SIGHUP, `POST /api/v1/reload`,
  the file watch) applies the token and the reboot switch only;
  a Cube's variables wait for the next start;
- `NoteDiscovered` never sets `configured`.

### Fault 2: the found map

`s.found` gets an entry per serial at every answer and loses none;
`loadFound` fills it at the start from every row with a `last_seen`.
`unconfigured` lists every entry without a runtime.

Each entry gets a state, kept in memory only:

| State | Means | Listed |
|-------|-------|--------|
| `loaded` | read from the store at the start, not heard in this process yet | no (D2-A) |
| `present` | answered the latest complete run, or the run going | when not configured |
| `gone` | did not answer the latest complete run | no |

- **An answer** (`note`): a new serial, or a `gone` entry, becomes `present`
  and `cubeFound` is called; a `loaded` one becomes `present` silently,
  so that a restart announces nothing a second time, as now.
- **The end of a complete run** (`settle`): a `present` entry that did not
  answer becomes `gone` and `cubeGone` is called; a `loaded` one becomes
  `gone` silently, or a Cube gone for a year would be logged at every start.
- **A run that ends with an error** (no discovery port, the server stopping)
  changes no state: it cannot tell a Cube gone from one not heard.

A complete run is one whose `Discover` returned without an error;
the responder sends the request twice, 50 ms apart, and waits 3 s.
The periodic run, the run every minute while a Cube has no address,
and a search are all such runs (`discover`).

`cubeFound` holds what `note` does now for a Cube that is not configured:
the line "cube found on the LAN, not configured" and the `cube_found` event.
`cubeGone` writes "cube gone from the LAN" at Info, with the serial and the
last address, for a Cube that is not configured.
Both are called without `foundMu` held, so that the Logging plan can write
to the store from them.
The third point Logging names, a Cube at a new address, stays in `note`,
where the entry's address changes;
for a Cube that follows discovery `note` already logs
"cube address from discovery".

A `gone` entry stays in memory:
`summary` takes a configured Cube's MAC and `last_seen` from it,
and `addCube` and `applyCube` the last address of one that follows discovery.
The row in the store is not touched:
its `last_seen` stays what it was
(written once per run of the store, or when an answer changes the row,
as `NoteDiscovered`'s comment says).

What changes outside:

- `GET /api/v1/cubes` lists the found Cubes that are `present` and not
  configured;
- `POST /api/v1/cubes/discover` still answers every Cube that answered,
  configured or not;
- `cube_found` comes for a new serial, or a Cube back after a run without it;
  still not after a restart;
- the UI reloads the list at every poll and after a search,
  so a gone Cube leaves the menu and the line under the header by itself.

**D1-B costs:** the found object gets a `gone` flag,
the UI a greyed row with "last seen …" in all six languages and a
`make ui-test` case.
And the day would count from `last_seen`, which the store writes once per
process: after a restart, a Cube heard an hour before could count from the
previous start and vanish at once,
unless `last_seen` is written at every answer,
an SD card write per Cube every 30 minutes that the store avoids on purpose.

**D2-B costs:** with `-discover-interval 0` a Cube gone for months is listed
after every restart until a search.
With the default interval the start's run fills the list about 3 s after the
start either way; the UI shows it at its next poll.

**Risks:**

- UDP loss: if both requests or the answer are lost, a found Cube leaves the
  list for one interval and is announced again when it answers,
  a found and gone pair in the log.
  Should it show on the boxes, a later change can wait for two missed runs.
- `TestFoundCubeSurvivesRestart` pins "listed before any discovery";
  D2-A changes that check, and the one that matters stays:
  no second announcement.
- Locks: `unconfigured` holds `foundMu` and asks `configured` (`s.mu`),
  as now; `settle` collects under `foundMu` and calls `cubeGone` after.

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `internal/server/discovery.go` | 1, 2 | `Configured` on the copies only; the states, `settle`, `cubeFound`, `cubeGone` |
| `internal/server/store.go` | 1, 2 | `loadFound`: no `Configured`, the state `loaded` |
| `internal/server/cubes_api_test.go` | 1 | `TestRemovedCubeIsListedAsFound` |
| `cmd/massimo/serve_test.go` | 1 | `TestServeRemovedCubeStaysRemoved` |
| `internal/server/discovery_test.go` | 2 | `TestSearchListsOnlyCubesThatAnswered`, `TestFailedRunChangesNothing` |
| `internal/server/store_test.go` | 2 | `TestFoundCubeSurvivesRestart` for D2-A |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml` | 2 | the found list, `cube_found` |
| `docs/ui.md` | 2 | the Cube choice and the Cubes panel |
| `CHANGELOG.md` | 1, 2 | a bullet each under Unreleased, Fixed |

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`make race`, and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD`
from the main checkout.
Nothing reaches a real Cube, a box or the network: the fake Cube answers,
and the address 192.0.2.10 only ever goes into `note`.
One functional commit per block, citing `AP FoundCubes v1.0, Block <n>`,
with its CHANGELOG bullet, written in the wrapped style of its neighbours.

### Block 1: RemovedFound

1. `internal/server/cubes_api_test.go`: `TestRemovedCubeIsListedAsFound`,
   a subtest per route:
   - *discovery list*: a found Cube (`fakecube.StartDiscovery`,
     as `TestAddFoundCube`) added with `POST /api/v1/cubes`,
     the server started again over the same store with the row's
     configuration, as `serveSetup` gives it, then `DELETE`;
   - *settings file*: `startStored`'s row (source `env`, a fixed address),
     an answer from 192.0.2.10 through `note`, then `DELETE`;
   - *plain address*: a Cube with `SeedFromFile` and `FileCubes[""]`,
     its row once `seedLearned` has written it,
     an answer from 192.0.2.10 through `note`, then `DELETE`.

   Each then finds the serial once in `GET /api/v1/cubes`,
   with `configured` false and no `connected`,
   and `GET /api/v1/cubes/{sn}` answering 404.
   Run on the old code first, where every subtest must fail.
2. `cmd/massimo/serve_test.go`: `TestServeRemovedCubeStaysRemoved`:
   the removed rows (`configured = 0`, source `ui`) of KEQ0000001, with a line
   `MASSIMO_CUBE_KEQ0000001_ADDRESS`, and of KEQ0000002, the plain address's
   learned serial; `serveSetup` serves neither and adds no plain Cube.
   It passes on the old code: it guards the restart of routes 2 and 3.
3. `internal/server/discovery.go`: `foundCube.Configured` set on the copies
   handed out, `note`'s answer and `unconfigured`'s list (false),
   never on the map's entry; its comment says why.
   `internal/server/store.go`: `loadFound` no longer copies it from the row.
4. `CHANGELOG.md`, Unreleased, Fixed:

   ```
   - A Cube removed under System settings → Cubes is listed as found and
     not configured, no longer as a configured Cube that is disconnected,
     once discovery had heard it or serve had restarted since its add.
   ```

**Verification:**
the test gate;
`TestRemovedCubeIsListedAsFound` fails before step 3 and passes after;
the reproduction's three routes, run again with the new binary,
list the removed Cube with `configured: false` after a restart.

### Block 2: LatestRun

1. `internal/server/discovery_test.go`:
   - `TestSearchListsOnlyCubesThatAnswered`: two found Cubes (KEQ0000000 and
     KEQ0000001, a `StartDiscovery` each) listed after a search;
     one closed, and the next search lists the other only,
     the closed one's row keeping its `last_seen`;
     the closed one answering again at the same address,
     and the next search lists both and sends its `cube_found` a second time.
   - `TestFailedRunChangesNothing`: after a search,
     `discoverOnce` with a cancelled context leaves the list as it was.
2. `internal/server/store_test.go`: `TestFoundCubeSurvivesRestart` (D2-A):
   after the restart nothing is listed before a run,
   the found Cube is once a run has heard it,
   and no second `cube_found` is sent.
3. `internal/server/discovery.go`: the state on `foundCube`
   (unexported, not in the JSON);
   `note` makes an entry `present` and calls `cubeFound` for a new or gone one;
   `discoverOnce` calls `settle` with the run's serials after a complete run;
   `cubeFound` with what `note` logs and sends now,
   `cubeGone` with its Info line;
   `unconfigured` lists `present` entries only.
   `internal/server/store.go`: `loadFound` marks its entries `loaded`.
   The package comment says what a found Cube is now.
4. `docs/api-v1.md`: `GET /api/v1/cubes` (a found Cube is listed while it
   answers; its row keeps `last_seen`; after a restart from the first run),
   `POST /api/v1/cubes/discover` (when `cube_found` comes);
   `docs/api-v1-openapi.yaml`: the same in the descriptions of those routes
   and of `DELETE /api/v1/cubes/{sn}`;
   `docs/ui.md`: the Cube choice and the Cubes panel.
5. `CHANGELOG.md`, Unreleased, Fixed:

   ```
   - A search, and the periodic discovery, list only the Cubes that
     answered it: one that no longer answers leaves the Cube choice and
     System settings → Cubes (`docs/api-v1.md`, "GET /api/v1/cubes").
   ```

**Verification:**
the test gate;
the new tests fail on Block 1's code and pass after step 3;
`make ui-test` at 1100 and 390 px, though no UI file changes,
since the page reads the list
(`UI_TEST_PYTHON` to the main checkout's venv, and free ports);
the reproduction run again: a search after the fake Cube's discovery stops
lists nothing, and the row keeps its `last_seen`.

## Proposed commit message

The plan's registration, a commit of its own on `main`
before the first project write:

```
chore(aps): +FoundCubes v1.0

A Cube removed under System settings → Cubes stays in the Cube menu as configured and disconnected, and a search never drops a Cube it found before; the plan fixes both in the server's found map, the first of the three plans from the owner's tests of v0.12.0-alpha.1 on a Raspberry Pi.

Key changes:
- GUIDELINES/wip/2026-11-08_20-56_AP_FoundCubes_removed_and_gone_cubes.md: the plan, two blocks, the reproduction of all three routes in its Analysis.

File-revision-v1: GUIDELINES/wip/2026-11-08_20-56_AP_FoundCubes_removed_and_gone_cubes.md::v1.0::2026-11-08::All::the first revision, by PLAN+
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `fix(discovery): A removed Cube is listed as found, not as configured`
2. `fix(discovery): A run lists only the Cubes that answered it`

## Change History

- v1.0 (2026-11-08): the first revision, from the owner's tests of
  v0.12.0-alpha.1 on a Raspberry Pi, with the reproduction of the three
  routes; D1 and D2 open at its gate.
