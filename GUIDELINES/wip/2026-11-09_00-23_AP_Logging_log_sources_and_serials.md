# AP Logging v1.0: log sources and serials

## TL;DR

The log names devices by their S/N, says what changed,
and gains four sources to filter on: the devices, the Cube, System and
Configuration, each change with who made it.
Six blocks, a pull request each; block 1 needs no migration.

- a device in the log → its radio address → its S/N, the address in expert mode
- a removed device → `device_forgotten 0a0101` → "Removed thermostat TEQ0000001"
- `metadata`, `config` → the bare kind → what changed, nothing when nothing did
- a reconnect → hello, metadata, a config per device → one `connected` line
- a Cube named, settings, users, backups, reload, start, stop → audit or text
  log only → the log, with who (D2-A, D8-A)
- `events` → one Cube's rows → also the server's, `cube_id` NULL,
  with `source`, `serial`, `user_id` (migration 0009, D1-A, D7-A)

### Blocks

1. LogSerial: the S/N in the events, the routes and the UI; no migration
2. LogDetail: metadata and config lines list their differences; the greeting
   folded into `connected`
3. LogStore: migration 0009, the server-wide rows, `source=` and `user=`
4. ServerEvents: start, stop, restart, reload, reboot, update, backups, prunes,
   restore; a Cube's new address and its leaving the LAN
5. ChangeEvents: a line beside each change's audit row, with who; the command
   line's too
6. LogFilter: the source and who choices, the address, the documents

## Discussion

**The owner's findings** of 2026-11-08, from v0.12.0-alpha.1 on the boxes:

1. Naming, renaming or unnaming a Cube doesn't appear in the log;
   the owner asked which configuration changes are missing (the audit below).
2. The log should have sources to filter on: System (reboot, reload, restart,
   backup, restore, …), the Cubes (connect, disconnect, address changed,
   pairing) and Config (configuration changes).
3. The S/N everywhere but in radio packets: a device removed from the Cube
   showed its radio address, and should read "Removed <type> <S/N>".
4. Lines that only say "metadata" don't help: what should they say?

**The order** is the owner's: FoundCubes first (merged as pull request 118,
retired in 60962c5), then Logging, then AptUpdate;
and one plan with blocks, not several plans.
FoundCubes left the hand-off this plan builds on:
`cubeFound` and `cubeGone` in `internal/server/discovery.go` are the one place
each where a Cube becomes found or gone, both called without `foundMu` held;
a Cube at a new address is `note`'s change of the address.

**The owner's input on the store:** an `events.user_uuid`, filled when a user
started the action, maybe indexed.
This plan recommends `events.user_id INTEGER REFERENCES users (id)` instead
(D7, with the reasons there).
For a user created or deleted, the event's data holds the target user's
`users.id` and `uuid` (the owner's point), its login name and its display name
as they were then; the same for roles (id, key) and keys (id, uuid, name).

**Open at the plan gate**, each recommended option first:

- **D1. Where System and Configuration events live.**
  A: in `events`, `cube_id` NULL and a `source` column, shown in every Cube's
  log: one timeline, one cursor; SQLite cannot drop a NOT NULL, so migration
  0009 rebuilds the table once (measured on the Pi 1, Block 3).
  B: a table and route of their own, `GET /api/v1/log`: no rebuild, but the UI
  pages two logs, and a Cube's log doesn't show that it was named.
  C: the UI merges the audit in: two tables cannot be paged by one id, and the
  audit's capability shows more than a log reader should see.
- **D2. Who appears in the log.**
  A: the display name, for everyone who reads the log, and the lines of users,
  roles and keys with it: in a household that is what the log is for.
  B: the name and those lines only for holders of `audit` or `users`,
  "a user" for the others; the stream then filters per subscriber.
- **D3. The settings file read again.**
  A: a System line naming the variables that changed and those waiting for a
  restart, never their values, which may be secrets.
  B: nothing beyond the server's text log.
- **D4. The API path `…/devices/{rf}`.**
  A: takes the S/N as well, the answers keep both fields: nothing breaks.
  B: the S/N only: an incompatible change for every client.
- **D5. A greeting's lines.**
  A: folded into one `connected` line, plus a line for each thing that changed
  while disconnected; the stream still sends them, marked, for the tiles.
  B: kept as lines, each with its summary: a reconnect stays a dozen lines.
- **D6. A greeting's lines that change nothing, in the store.**
  A: not stored; the backups keep the raw lines.
  B: stored and hidden in the UI: about 20 kB per reconnect with thirty devices.
- **D7. The acting user in `events`.**
  A: `user_id INTEGER REFERENCES users (id)`: the rowid is the internal key
  (`docs/store.md`, "Identifiers"), as `audit.user_id`, `updated_by` and
  `invited_by` have it; users are tombstoned, never deleted, so it never
  dangles; an export maps it to the uuid.
  B: `user_uuid TEXT`, the owner's first idea: readable without a join, but
  36 bytes a row and an index on text, and the only uuid reference in the file.
- **D8. Which changes get a line with who.**
  A: every change made through the API or the command line, kind `change`,
  under Configuration; one about a device carries its S/N, so the device's
  choice shows it too; setpoints included; the Cube's answer follows as its
  own line.
  B: as A, without setpoints and modes, which are no configuration: who set
  one stays in the audit.
  C: only the server's own configuration (Cubes, settings, users, roles, keys);
  a change sent to the Cube shows only as the Cube's answer.

**Names chosen here**, routine and not the owner's decisions:
the blocks' names; the sources `device`, `cube`, `system` and `config`;
the kinds in the Analysis' table; the event fields `serial`, `source`,
`greeting`, `cause`, `summary` and `by`;
the migration `0009_events_sources.sql`; the tests in the Implementation Plan;
the branches `claude/log-serial`, `claude/log-detail`, `claude/log-store`,
`claude/server-events`, `claude/change-events` and `claude/log-filter`.

**Not in this plan:**

- a log without a Cube: with D1-A the server-wide lines show in a Cube's log,
  so a server with no Cube has none to show;
- the audit in the UI: `GET /api/v1/audit` stays for holders of `audit`,
  the complete record behind the log;
- the server's text log: its lines stay as they are;
- who changed something at the Cube itself or in another app: the Cube does
  not say, so its answer's line has no who;
- a `VACUUM` after the rebuild: the store never runs one (`docs/store.md`).

**Who runs it:** this session, after the owner's `EXEC`.
The plan's own commit goes straight onto `main`;
each block's branch starts after it, from `origin/main`,
or from the block before while that one is not merged yet (merged in, never
rebased), and becomes a pull request with a commit per block.

## Analysis

Read on `main` at 0dfda58, with FoundCubes merged.
Placeholders stand for real identifiers: `KEQ0000000` a Cube,
`TEQ0000001` a device, `0a0101` a radio address.

### What the log holds now

The log (`internal/server/log.go`) is one Cube's event stream:
the driver's events (`connected`, `disconnected`, `hello`, `metadata`, `config`,
`state`, the pairing's three, `device_forgotten`), `cube_found`, and a
restore's `restore_step` and `restore_done`.
`remember` hands each to the store's writer, one row of `events` with the
Cube's `cube_id`, or to the runtime's ring of 500 for a Cube without a row.
`cube_found` goes to every Cube's stream (`broadcast`), so today it is stored
once per configured Cube.

Separately, the audit (`internal/server/audit.go`, table `audit`) records every
request that changes something, the refused ones included:
who (`user_id`, `token_id`, NULL for the bootstrap `MASSIMO_API_TOKEN`), from
where, the action, the target, the request with its secrets removed.
`GET /api/v1/audit` exists (capability `audit`); the web UI never calls it.
The command line's `massimo user add|passwd|disable|enable|token|revoke`
writes its own audit rows (`store.RemoteCLI`, `cmd/massimo/user.go`);
`massimo store backup|migrate|info` changes no configuration.

### Finding 1: what the log misses

| Change | Audit | Log now | After the plan |
|--------|-------|---------|----------------|
| Cube added, named, renamed, unnamed, its address, proxy, hidden devices, removed (`cube.add`, `cube.update`, `cube.remove`) | yes | no | Configuration, `change`, with who (5) |
| server settings (`settings.update`): `backups.*`, `log.keep_days`, `auth.password_login`, … | yes | no | Configuration (5) |
| users, roles, keys, capabilities | yes | no | Configuration (5) |
| the same by `massimo user …` | yes | no | Configuration, "by the command line" (5) |
| room renamed or deleted; device renamed, moved, removed | yes | `metadata`, `device_forgotten`, bare | the Cube's lines with their differences (1, 2), and the request's line with who (5, D8) |
| device or room configuration, week program, lock | yes | `config`, bare | the device's line with its differences (2); the request's (5) |
| setpoint, mode | yes | `state` with its changes | unchanged; the request's line with who (5, D8-A) |
| pairing started, ended, a device paired | yes | the pairing events | the Cube's lines; the request's (5) |
| restore planned, started; its steps and end | yes | `restore_step`, `restore_done` | System, with who started it (4) |
| backup by hand, deleted | yes | no | System, with who (4) |
| automatic backup, prune of backups, prune of the log | no | no | System (4) |
| restart, reload, reboot and its taking back, update started | yes | no | System, with who (4) |
| the settings file read again (SIGHUP, its watch, `POST /api/v1/reload`) | the request only | no | System, the variables' names (4, D3-A) |
| an update's outcome, after the restart | no | no | System, from the helper's status file (4) |
| the server's start and stop | no | no | System (4) |
| a Cube's address from discovery (`note`) | no | no | the Cube's, `address_changed` (4) |
| a Cube found on the LAN, gone from it | no | `cube_found`, once per Cube | server-wide, `cube_found`, `cube_gone` (3, 4) |
| sign-in, sign-out, preferences, a search | yes | no | stays in the audit only |

### Finding 3: the radio address

Every event's `device` is the radio address (`pump` in `server.go`).
Its `state`, the device view (`view.DeviceOf`), carries the serial, and is set
only while the device has a state in the snapshot:
a device that never reported, or the Cube itself, gets no view.
The store's `events.device`, the route's `device=`, the address
`#log?device=…` and the UI (`logDeviceName`, `fillLogSelects`) all use the
radio address; the line falls back to it when the device has no name,
as do the device lists in `config.js`, `cubes.js` and `tiles.js`.

**The removed device:** `Client.ForgetDevice` (`internal/cubelan/client.go`)
calls `Snapshot.Forget`, which drops the device's configuration, state and
metadata, and only then emits `EventForgotten`;
the pump finds no state, and the line has nothing but the radio address.
The fix: `Forget` returns the device's last metadata (type, serial, name, room)
and configuration's serial, the event carries it, and the pump builds the view
from it.

**The `config 0a0101` line at a reconnect** is the Cube's own C line:
the Cube is a device of type `cube` (`cube.DeviceCube`), its C line emits
`config` like any other (`Snapshot.apply`), and it has no state.
It becomes the Cube's line, "Cube KEQ0000000".

The radio address and the serial are both fixed in a device, one to one;
the S/N is the one printed on it.
So the store keeps the radio address, the protocol's key, and gains the serial
(Block 3); the routes take either, since they cannot be confused:
a radio address is six hex digits, a serial ten characters.
Block 1 maps a serial to its radio address through the snapshot;
Block 3's column then finds a device the Cube no longer has.

### Finding 4: what metadata and config lines say

Every M line emits `metadata` with no detail, the greeting's included,
and every C line `config`;
so a reconnect logs `hello`, `metadata` and one `config` per device,
the Cube's own included, even when nothing changed.
`state` already carries its differences (`view.Changes`).
An H line comes only in a greeting.

The M line holds the rooms (id, name, the room's radio group) and the devices
(type, radio address, serial, name, room), so its differences are exact:

| Change | The line |
|--------|----------|
| a room added | Room "Kitchen" added |
| a room renamed | Room "Bath" renamed "Bathroom" |
| a room deleted | Room "Bath" deleted |
| a device renamed | TEQ0000001 "Window" renamed "Window left" |
| a device moved | TEQ0000001 "Window" moved from Bath to Kitchen |
| a device added to the list | TEQ0000001 "Window" added to Kitchen |
| a device left out of the list | TEQ0000001 "Window" left the list |

A line about one device carries its S/N, so the device's choice finds it.
A C line's differences are its decoded fields (`cube.ThermostatConfig`,
`WallConfig`, `CubeConfig`): "comfort 21.0 → 21.5", "offset 0.0 → −0.5",
"Monday's program changed", "portal off", "time zone CET".
Hidden devices stay out, as their events do now.

**A reconnect** becomes one line:
"connected: 12 devices in 5 rooms, firmware 1.1.3, duty cycle 3 %, 180 slots
free", after a loss also "down for 5 min".
The driver marks a greeting's H, M and C events (`greeting`) and says why the
connection began (`cause`: the start, after a loss, or a reconnect the server
asked for).
The server compares the greeting's metadata and configurations with what it
held before: in memory within a run, and after a restart the newest backup
(its rooms, devices and configurations, decoded), which the backup watch reads
at its first look anyway.
What differs follows `connected` as its own lines, "changed while
disconnected"; nothing when nothing did.
A reconnect the server asked for (the follow-up's, an address change) writes
no `connected` line, only its differences.
The stream still sends every event, marked, since `app.js` reloads the rooms
on `metadata` and the Cubes on `hello`; the log neither stores (D6-A) nor
draws the marked ones.
Outside a greeting, a `config` with no difference writes nothing (a follow-up
re-read that finds the same C line), and one with no previous value says
"configuration read".

### The store: migration 0009

`events.cube_id` is `NOT NULL REFERENCES cubes (id) ON DELETE CASCADE`
(`0001_initial.sql`), and SQLite cannot drop a NOT NULL with `ALTER TABLE`.
So 0009 rebuilds the table, the documented way, in the migration's one
transaction: a new table, the rows copied with their new columns, the old one
dropped, the new one renamed, the indexes made.
No table references `events`, so the foreign keys stay on.

```sql
CREATE TABLE events (
    id        INTEGER PRIMARY KEY,
    uuid      TEXT    NOT NULL UNIQUE,
    cube_id   INTEGER REFERENCES cubes (id) ON DELETE CASCADE,  -- NULL: the server's
    time      TEXT    NOT NULL,
    kind      TEXT    NOT NULL,
    source    TEXT    NOT NULL DEFAULT '',  -- device, cube, system, config; '' an older binary's row
    device    TEXT    NOT NULL DEFAULT '',  -- radio address, '' when none
    serial    TEXT    NOT NULL DEFAULT '',  -- the device's S/N, or the Cube's for its own lines
    user_id   INTEGER REFERENCES users (id),  -- who started it; NULL: the Cube, the server, the bootstrap token
    data      TEXT    NOT NULL
);
CREATE INDEX events_cube ON events (cube_id);
CREATE INDEX events_cube_time ON events (cube_id, time);
CREATE INDEX events_cube_kind ON events (cube_id, kind);
CREATE INDEX events_cube_source ON events (cube_id, source);
CREATE INDEX events_cube_serial ON events (cube_id, serial);
CREATE INDEX events_user ON events (user_id) WHERE user_id IS NOT NULL;
```

- **`events_cube (cube_id)` stays:** migration 0004 says why.
  SQLite appends the rowid to every index, so this one pages the unfiltered log
  newest first by id without a sort, and it serves the server's rows with
  `cube_id IS NULL` the same way.
- **The others:** (cube_id, device) becomes (cube_id, serial);
  (cube_id, source) and the partial `events_user` are new;
  (cube_id, time) stays for the prune, which gains a pass for `cube_id IS NULL`,
  and for time ranges; (cube_id, kind) stays.
- **The backfill**, in the copy:
  `serial` from `data`'s `state.serial`, else from another row of the same Cube
  and radio address (one pass builds that map over the old index
  (cube_id, device), the copy uses it), else for the Cube's own C line from
  `cubes.serial` where `cubes.rf_address` matches;
  `source` `device` for a device's rows, `system` for the restore's,
  `cube` for the rest; `user_id` NULL.
  A row it cannot place keeps `serial` empty and ages out with `log.keep_days`.
- **`min_reader` stays 1:** an older binary reads its Cube's rows by
  `cube_id = ?` and never meets a NULL one, and writes rows without the new
  columns, which take their defaults; the new binary fills `source` and
  `serial` of such rows (`source = ''`) at its start.
  A device page of an older binary walks the Cube's rows without its index,
  the 1.3 s at 90,000 rows that 0004 fixed: slow, not wrong.
- **The cost:** every row is written once more, in one transaction,
  and the WAL holds the new table until the checkpoint:
  at the default 30 days, about 60 MB on an SD card (`docs/store.md`, the
  events' note), so up to twice that on the card for a while.
  `serve` answers only after the migration.
  Measured on the owner's Raspberry Pi 1, with the owner's go: the migration on
  a copy of its store, and a batch of 100 events before and after.

### The page

A Cube's page is two index walks, each limited to the page's size and in id
order: the Cube's rows (`cube_id = ?`) and the server's (`cube_id IS NULL`),
with the same terms, merged in Go, the first `limit` kept.
One query with `cube_id = ? OR cube_id IS NULL` would sort.
The cursor stays one id, since both walks share one sequence;
a time bound becomes an id bound per walk, as now.
`TestEventsPageQuery` checks every shape's plan for a sort.
A device's page (`serial = ?`) needs the Cube's walk only.
The route gains `source=` (`device`, `cube`, `system`, `config`) and `user=`
(a login name, as the audit's route takes it); `device=` takes a S/N or a
radio address.

### Who

The request's principal names the user, a key's events its key's user
(`principal.user`); the bootstrap `MASSIMO_API_TOKEN`, a server without a
store and the command line have none.
`user_id` is stored, the event's `data` carries no name;
the page and the stream add `by` (`name`, `display_name`) from `users` at
read, the name as it is now, a tombstoned user's as it was.
A line of the command line says so (`via: "cli"`).
An automatic job, a Cube's answer and a radio event have no who.

### Sources and kinds

| Source | Kinds |
|--------|-------|
| `device` | `state`, `config` (a device's C line), `paired`, `device_forgotten` |
| `cube` | `connected`, `disconnected`, `metadata`, `config` (the Cube's own), `pairing_started`, `pairing_ended`, `address_changed`, `cube_found`, `cube_gone` |
| `system` | `server_started`, `server_stopping`, `settings_read`, `restart`, `reboot`, `reboot_cancelled`, `update`, `update_result`, `backup`, `backup_deleted`, `backups_pruned`, `log_pruned`, `restore_started`, `restore_step`, `restore_done` |
| `config` | `change`: its `action` (the audit's, such as `cube.update`), its `target`, its `changes` from → to |

`hello` stays a kind of the stream, never a line of the log (D5-A).
Server-wide rows (`cube_id` NULL): the server's start, stop, restart, reload,
reboot, update, prunes, `cube_found`, `cube_gone`, and the changes of settings,
users, roles, keys and capabilities.
A Cube's own rows: its connection, address, rooms, devices, pairing, backups,
restore, and its own changes (named, address, proxy, hidden, added, removed).
They reach every Cube's stream, and are stored once.

### What changes, by file

| File | Block | Change |
|------|-------|--------|
| `internal/cubelan/snapshot.go`, `client.go` | 1, 2 | `Forget` returns the last metadata; the forgotten event carries it; `greeting` and `cause` on the events |
| `internal/server/server.go` | 1–4 | the pump: `serial`, `source`, the Cube's own C line, the greeting folded; `logServer` for the server's rows |
| `internal/server/logdiff.go` (new) | 2 | the differences of metadata and configurations, and the `connected` summary |
| `internal/server/log.go` | 1, 3 | `device=` by S/N, `source=`, `user=`, the two walks, `eventKinds` |
| `internal/server/config_api.go`, `server.go` | 1 | `…/devices/{rf}` takes the S/N (D4-A), one helper for the five call sites |
| `internal/server/discovery.go` | 3, 4 | `cubeFound` and `cubeGone` write server-wide rows; `note` writes `address_changed` |
| `internal/server/backups.go`, `backups_api.go`, `restore_api.go`, `restart_api.go`, `reboot_api.go`, `update_api.go` | 4 | their System lines |
| `cmd/massimo/serve.go`, `reload.go` | 4 | start, stop, the settings file read again, the update's outcome |
| `internal/server/audit.go` and the handlers | 5 | the `change` line beside the audit row |
| `cmd/massimo/user.go` | 5 | the command line's `change` lines |
| `internal/store/migrations/0009_events_sources.sql`, `events.go`, `migrate.go` | 3 | the rebuild, the backfill, the columns, the walks, the prune |
| `internal/server/ui/log.js`, `config.js`, `cubes.js`, `tiles.js`, `partials/log.html`, `i18n/*.json` | 1, 2, 4–6 | the S/N, the lines' texts, the choices |
| `tools/ui-test/` | 1, 2, 6 | the log's S/N, lines and choices at 1100 and 390 px |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml` | 1–5 | the events' fields and kinds, the route's parameters, the device path |
| `docs/ui.md` | 1, 2, 4–6 | the log |
| `docs/store.md` | 3 | the schema, migration 0009, the page |
| `README.md` | 5 | `massimo user …` writes the log too |
| `CHANGELOG.md` | 1–6 | a bullet each under Unreleased |

### Risks

- **The rebuild on the Pi:** its time and its space on the SD card are unknown
  until measured; if too slow, the copy can run in chunks of 5000 as the prune
  does, at the price of a migration that is not one transaction.
- **The insert:** two indexes more per row; `TestEventSize` gives the bytes,
  the Pi the time.
- **The reference after a restart** is the newest backup, taken some minutes
  after a change (`backups.auto_min`): a change made just before a stop can be
  logged again after the start.
- **Attributing who** relies on the request alone (D8-A): nothing guesses
  which of the Cube's answers belongs to which request.
- **The stream** grows by the server's events on every Cube's stream;
  the UI's handlers ignore kinds they don't know.

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`, `make race`,
`make ui-test` at 1100 and 390 px where a UI file changes
(`UI_TEST_PYTHON` to the main checkout's venv, and free ports),
and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD` from the
main checkout.
Nothing reaches a real Cube, a box or the network: the fake Cube answers,
and no new command is sent to a Cube.
A route that takes a new form gets a test that checks the line sent to the
fake Cube, and `TestOpenAPICoversRoutes` holds the OpenAPI file to the routes.
Every UI text goes through `t()` or `data-i18n` in all six languages,
FR, IT, NL and PL machine-translated and marked.
One functional commit per block, citing `AP Logging v1.0, Block <n>`,
with its CHANGELOG bullet in the wrapped style of its neighbours.

### Block 1: LogSerial

1. `internal/cubelan`: `Snapshot.Forget` returns the device's last metadata
   (type, serial, name, room and the room's name) or, for an unlisted device,
   its configuration's serial; `Event` gains `Gone`, set by `ForgetDevice`.
   `TestForgetDeviceCarriesLastView`.
2. `internal/server/server.go`, the pump: every device event carries `serial`,
   from the metadata or the configuration, with or without a state;
   `device_forgotten` gets the view built from `Gone`;
   the Cube's own C line gets `source: "cube"` and the Cube's serial, every
   other device event `source: "device"`.
   `TestLogEventsCarrySerial` (a device without a state included),
   `TestLogForgottenDeviceHasSerial`, `TestCubeOwnConfigIsTheCubes`.
3. `internal/server`: one helper resolves a path's or a query's device, a S/N
   or a radio address, through the snapshot, for `GET …/log?device=` and the
   five call sites of `…/devices/{rf}` (D4-A); a S/N the Cube doesn't have is
   `404` on a path and an empty page on the log.
   `TestDeviceRoutesTakeSerial`: each device route with the S/N sends the fake
   Cube the same line as with the radio address;
   `TestLogDeviceFilterBySerial`.
4. UI: the log line names the device, else its S/N, with the radio address in
   expert mode only (as now); "Removed thermostat TEQ0000001 "Window" (Kitchen)";
   the Cube's own line as "Cube KEQ0000000"; the device choice "Name ·
   TEQ0000001", its value and the address's `device=` the S/N, an old link's
   radio address read and replaced;
   the fallbacks of `config.js`, `cubes.js` and `tiles.js` to the S/N.
   `tools/ui-test`: the log's S/N and the removed line.
5. `docs/api-v1.md` ("Identifiers", the device routes, `GET …/log`, the
   events' `serial` and `source`), the OpenAPI file, `docs/ui.md` ("Log").
6. `CHANGELOG.md`, Unreleased, Fixed:

   ```
   - The log names a device by its S/N, the radio address in expert mode
     only, and a removed one as "Removed thermostat TEQ0000001"; the
     device's routes take its S/N too (`docs/api-v1.md`, "Identifiers").
   ```

**Verification:** the test gate; the new tests fail before steps 1 to 3;
a device removed from the fake Cube logs its S/N.

### Block 2: LogDetail

1. `internal/cubelan`: `greeting` on the H, M and C events of a greeting;
   `cause` on `EventConnected` (`start`, `loss`, `reconnect`).
2. `internal/server/logdiff.go`: the differences of two metadata (the table of
   Finding 4) and of two configurations of each type, hidden devices left out;
   the `connected` summary (devices, rooms, firmware, duty cycle, free slots,
   down for).
   `TestMetadataChanges`, `TestConfigChanges`, table tests with the recorded
   greeting.
3. The pump: `metadata` and `config` carry `changes`; a greeting's marked
   events go to the stream and not to the log (D5-A, D6-A);
   `connected` carries `summary`, after the start and a loss;
   the reference is the snapshot before the loss, and after a restart the
   newest backup.
   `TestReconnectIsOneLine`, `TestChangedWhileDisconnected`,
   `TestChangedWhileServerDown`, `TestSameConfigWritesNothing`.
4. UI: the texts of every change and of the summary, in all six languages;
   the log view skips the marked events. `tools/ui-test`: a reconnect's line.
5. `docs/api-v1.md` (the events' `changes`, `summary`, `greeting`, `cause`),
   the OpenAPI file, `docs/ui.md` ("Log").
6. `CHANGELOG.md`, Unreleased, Changed:

   ```
   - The log's metadata and config lines say what changed, and a reconnect
     is one line with the Cube's devices, rooms, firmware and duty cycle
     (`docs/ui.md`, "Log").
   ```

**Verification:** the test gate; a fake Cube reconnecting with nothing changed
logs one line, with a room renamed in between two.

### Block 3: LogStore

1. `internal/store/migrations/0009_events_sources.sql` and its Go step
   (`afterMigration[9]`): the rebuild and the backfill of the Analysis.
   `TestMigration9` (rows, ids and uuids kept; `serial` and `source` filled,
   the Cube's own C line placed; a NULL `cube_id` taken),
   `TestMigratesAVersionOneFile` through 0009,
   `TestOlderReaderOnNine` (the 0008 queries and insert on a 0009 file).
2. `internal/store/events.go`: `Event` gains `Source`, `Serial`, `UserID`,
   `CubeID` 0 for the server's; `EventQuery` gains `Source`, `Serial`,
   `UserID` and `Server` (with the server's rows); the two walks merged;
   the prune's pass for `cube_id IS NULL`; the start's fill of `source = ''`.
   `TestEventsPageQuery` for every new shape, `TestEventsWithServerRows`
   (the merge, the cursor, the time bounds), `TestPruneServerEvents`,
   `TestEventSize` updated.
3. `internal/server`: `logServer` stores a server-wide event once and sends
   it to every Cube's stream (and to the rings of Cubes without a row);
   `cube_found` moves to it; `GET …/log` takes `source=` and `user=`;
   `by` filled at read and on the stream.
   `TestLogSourceFilter`, `TestLogUserFilter`, `TestFoundCubeStoredOnce`.
4. `docs/store.md` (section 5's schema, "Migrations", stage (d)'s note),
   `docs/api-v1.md` and the OpenAPI file (`GET …/log`'s parameters, `by`,
   the server-wide events).
5. `CHANGELOG.md`, Unreleased, Added:

   ```
   - The log keeps the server's own events beside the Cube's, each with
     its source and who started it; the store's migration 0009 rebuilds
     its events table once (`docs/store.md`, "Migrations").
   ```

**Verification:** the test gate;
on the owner's Raspberry Pi 1, **only with the owner's go**: a store test
binary for linux/arm (GOARM=6) run on a copy of the box's `massimo.db` in a
scratch directory, timing the migration and a batch of 100 events on 0008 and
on 0009; the service and its file untouched, only the times leave the box;
the times go into `docs/store.md` and a NOTE.

### Block 4: ServerEvents

1. `cmd/massimo/serve.go`: `server_started` (the version, the one before when
   it changed, kept in `settings`), `server_stopping` (why: a signal, a
   restart, an update, a reboot), written before the store's writer stops;
   `update_result` once from the helper's status file when it is newer than
   the last one logged.
2. `cmd/massimo/reload.go`: `settings_read`: why, the variables changed and
   those waiting for a restart, names only (D3-A), when something changed or
   a request asked.
3. `internal/server`: `restart`, `reboot`, `reboot_cancelled`, `update` with
   who; `backup` (its reason, with who for one by hand), `backup_deleted`,
   `backups_pruned`, `log_pruned` (only when rows went), `restore_started`;
   `restore_step` and `restore_done` under System;
   `address_changed` in `note`, `cube_gone` in `cubeGone`.
4. UI: their texts in all six languages.
5. `docs/api-v1.md` and the OpenAPI file (the kinds), `docs/ui.md` ("Log").
6. Tests: `TestServerStartAndStopLogged`, `TestSettingsReadNamesOnly`,
   `TestUpdateResultLoggedOnce`, `TestBackupsLogged`,
   `TestSystemRequestsCarryWho`, `TestAddressChangedLogged`,
   `TestCubeGoneLogged`.
7. `CHANGELOG.md`, Unreleased, Added:

   ```
   - The log shows the server's start, stop, restart, reload, reboot,
     update, backups and prunes, and a Cube's new address or its leaving
     the LAN (`docs/ui.md`, "Log").
   ```

**Verification:** the test gate; `serve` against the fake Cube, stopped and
started, then a backup by hand, a reload and a removed fake Cube's discovery:
each one line.

### Block 5: ChangeEvents

1. `internal/server`: a table beside `actions` maps each audited action to its
   line: `change` under Configuration, a System kind of Block 4, or none (sign-in,
   sign-out, preferences, the search, the ticket); with D8-A setpoints and
   modes included.
   A successful request writes it next to its audit row, with `user_id`:
   its `action`, its `target` (a device by S/N), its `changes` from → to
   (a Cube's name "" → "Upstairs"); a user, role or key with its ids, uuid
   and names as they were then; never a secret.
   `TestEveryActionHasALine` (as the audit's test for `actions`),
   `TestCubeNamedIsLogged`, `TestUserChangeCarriesIds`, `TestNoSecretInLine`.
2. `cmd/massimo/user.go`: each command writes its `change` line with the
   audit row, straight into the store (`via: "cli"`).
   `TestUserCommandsLog`.
3. D2-A: `by` for everyone who reads the log (with D2-B: for holders of
   `audit` or `users`, filtered per subscriber).
4. UI: the texts per action, in all six languages, "by Anna".
5. `docs/api-v1.md` and the OpenAPI file (`change`, `by`), `docs/ui.md`,
   `README.md` (`massimo user …`).
6. `CHANGELOG.md`, Unreleased, Added:

   ```
   - Every change made through the API or `massimo user` is in the log,
     with who made it: a Cube named, a setting, a user, a role, a key
     (`docs/ui.md`, "Log").
   ```

**Verification:** the test gate; `tools/ui-test`: a Cube named in the UI shows
its line with the user's name.

### Block 6: LogFilter

1. UI: one choice beside the kind: all, System, Configuration, the Cube,
   then each device by name and S/N; a choice of who for a server with users;
   the kind choice grouped by source.
   The address after `#log`: `source=`, `device=` a S/N, `user=`.
   Every text in all six languages; phone width first.
2. `tools/ui-test`: the choices, the address, a reload, at 1100 and 390 px.
3. `docs/ui.md` ("Log"): the choices and the address.
4. `CHANGELOG.md`, Unreleased, Added:

   ```
   - The log chooses a source, System, Configuration, the Cube or one
     device, and who made a change, kept in the address after `#log`
     (`docs/ui.md`, "Log").
   ```

**Verification:** the test gate; `make ui-test` at 1100 and 390 px.

## Proposed commit message

The plan's registration, a commit of its own on `main`
before the first project write:

```
chore(aps): +Logging v1.0

The log names devices by their radio address, writes bare metadata and config lines, and shows nothing of a Cube's naming, the server's configuration or its own jobs; the plan gives it the S/N, readable lines and four sources with who made each change, the second of the three plans from the owner's tests of v0.12.0-alpha.1.

Key changes:
- GUIDELINES/wip/2026-11-09_00-23_AP_Logging_log_sources_and_serials.md: the plan, six blocks, the audit of what the log misses in its Analysis.

File-revision-v1: GUIDELINES/wip/2026-11-09_00-23_AP_Logging_log_sources_and_serials.md::v1.0::2026-11-09::All::the first revision, by PLAN+
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `fix(log): A device is named by its S/N, a removed one too`
2. `feat(log): Metadata and config lines say what changed`
3. `feat(store): The log keeps the server's events, their source and who`
4. `feat(log): The server's own jobs and a Cube's address in the log`
5. `feat(log): Every change says who made it`
6. `feat(ui): The log chooses a source and who`

## Change History

- v1.0 (2026-11-09): the first revision, from the owner's findings on
  v0.12.0-alpha.1 and the store input, after FoundCubes merged;
  D1 to D8 open at its gate.
