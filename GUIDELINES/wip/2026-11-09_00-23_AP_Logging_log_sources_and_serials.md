# AP Logging v1.1: log sources and serials

## TL;DR

The log names devices by their S/N, says what changed,
and gains sources (the devices, the Cube, System, Configuration),
who made each change, and a level per line.
Seven blocks, a pull request each.

- a device in the log → its radio address → its S/N, the address in expert mode
- a removed device → `device_forgotten 0a0101` → "Removed thermostat TEQ0000001
  (Window)"
- `metadata`, `config` → the bare kind → what changed
- a reconnect → hello, metadata, a config per device → one `connected` line at
  info, the greeting's lines at debug (D9-B)
- a Cube named, settings, users, backups, reload, start, stop → audit or text
  log only → the log, with who (D2-A, D8-A)
- `events` → one Cube's rows → also the server's, `cube_id` NULL,
  with `source`, `serial`, `level`, `user_id` (migration 0009, D1-A, D7-A)
- the store's schema → eight migrations to read → one generated `schema.sql`
  (D10-A)

### Blocks

1. LogSerial: the S/N in the events, the routes and the UI; no migration
2. SchemaFile: `internal/store/schema.sql`, generated from the migrations and
   checked by a test
3. LogStore: migration 0009, the server-wide rows, `source=`, `level=`, `user=`
4. LogDetail: lines with their differences; the greeting folded into
   `connected`, its own lines at debug
5. ServerEvents: start, stop, restart, reload, reboot, update, backups, prunes,
   restore; a Cube's new address and its leaving the LAN
6. ChangeEvents: a line beside each change's audit row, with who; the command
   line's too
7. LogFilter: the source, level and who choices, the address, the documents

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

**Beside AP ServerAnnouncements**, registered on `main` on 2026-11-09:
its Block 3 plans a migration `0009_push_subscriptions.sql`,
and its Block 1 moves `serve`'s wiring from `cmd/massimo/serve.go` into
`internal/serve`.
Whichever migration merges second takes the next number, 0010;
Block 5 here writes the server's start and stop wherever the wiring lives then.

**The owner's input on the store:** an `events.user_uuid`, filled when a user
started the action, maybe indexed.
The plan takes `events.user_id INTEGER REFERENCES users (id)` instead (D7-A).
For a user created or deleted, the event's data holds the target user's
`users.id` and `uuid` (the owner's point), its login name and its display name
as they were then; the same for roles (id, key) and keys (id, uuid, name).

**Decided at the plan gates** of v1.0, by the owner:

- **D1-A.** System and Configuration events live in `events`, `cube_id` NULL,
  with a `source` column, shown in every Cube's log: one timeline, one cursor.
  The rebuild 0009 needs costs nothing worth measuring: the owner's Pi 1 is the
  only installed store, and a new one migrates an empty file.
- **D2-A.** Who is the display name as it is now, for everyone who reads the
  log: the store keeps `user_id` only, and the page and the stream add the name
  at read; a deleted user keeps its last.
- **D3-A.** The settings file read again: a System line naming the variables,
  never their values.
- **D4-A.** `…/devices/{rf}` takes the S/N as well, the answers keep both
  fields; **the UI sends the S/N** in every device request.
- **D5-A with D9-B.** A reconnect is one `connected` line at info, with a line
  for each difference; the greeting's hello, metadata and config lines are
  kept as well, each with its summary, at debug.
  The level is a column of `events`, chosen in the API and the UI.
  Each greeting message is also one Debug line of the server's text log.
- **D6** falls with D9-B: the greeting's lines are stored, at debug.
- **D7-A.** `events.user_id INTEGER REFERENCES users (id)`.
- **D8-A.** Every change made through the API or the command line gets a line,
  kind `change`, under Configuration, setpoints included;
  one about a device carries its S/N.
- **D10-A.** `internal/store/schema.sql`, generated from the migrations and
  checked by a test, for reading only; a store is still created by the
  migrations, the one path every upgrade takes.
- **D11-A.** `massimo store backup` stays a full copy, the log included.
- **D12-A.** The Pi 1 check times the insert only, with the owner's go.
- **The removed line** reads "Removed thermostat TEQ0000001 (Window)", the name
  as it was; without a name, "Removed thermostat TEQ0000001".

**Names chosen here**, routine and not the owner's decisions:
the blocks' names; the sources `device`, `cube`, `system` and `config`;
the levels `debug`, `info`, `warn` and `error`, as the text log names them;
the kinds in the Analysis' table; the event fields `serial`, `source`,
`level`, `greeting`, `cause`, `summary` and `by`;
the migration `0009_events_sources.sql`; the tests in the Implementation Plan;
the branches `claude/log-serial`, `claude/schema-file`, `claude/log-store`,
`claude/log-detail`, `claude/server-events`, `claude/change-events` and
`claude/log-filter`.

**Not in this plan:**

- a log without a Cube: the server-wide lines show in a Cube's log,
  so a server with no Cube has none to show;
- the audit in the UI: `GET /api/v1/audit` stays for holders of `audit`,
  the complete record behind the log;
- the server's text log, but for the greeting's Debug lines;
- who changed something at the Cube itself or in another app: the Cube does
  not say, so its answer's line has no who;
- a store backup without the log (D11-A);
- the backups' formats: the server's own are stored as JSON and converted on
  download; an uploaded restore file is kept as it was sent;
- a `VACUUM` after the rebuild: the store never runs one (`docs/store.md`).

**Who runs it:** this session.
The plan's own commits go straight onto `main`;
each block's branch starts after them, from `origin/main`,
or from the block before while that one is not merged yet (merged in, never
rebased), and becomes a pull request with a commit per block.

## Analysis

Read on `main` at 25ef827.
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
| Cube added, named, renamed, unnamed, its address, proxy, hidden devices, removed (`cube.add`, `cube.update`, `cube.remove`) | yes | no | Configuration, `change`, with who (6) |
| server settings (`settings.update`): `backups.*`, `log.keep_days`, `auth.password_login`, … | yes | no | Configuration (6) |
| users, roles, keys, capabilities | yes | no | Configuration (6) |
| the same by `massimo user …` | yes | no | Configuration, "by the command line" (6) |
| room renamed or deleted; device renamed, moved, removed | yes | `metadata`, `device_forgotten`, bare | the Cube's lines with their differences (1, 4), and the request's line with who (6) |
| device or room configuration, week program, lock | yes | `config`, bare | the device's line with its differences (4); the request's (6) |
| setpoint, mode | yes | `state` with its changes | unchanged; the request's line with who (6) |
| pairing started, ended, a device paired | yes | the pairing events | the Cube's lines; the request's (6) |
| restore planned, started; its steps and end | yes | `restore_step`, `restore_done` | System, with who started it (5) |
| backup by hand, deleted | yes | no | System, with who (5) |
| automatic backup, prune of backups, prune of the log | no | no | System (5) |
| restart, reload, reboot and its taking back, update started | yes | no | System, with who (5) |
| the settings file read again (SIGHUP, its watch, `POST /api/v1/reload`) | the request only | no | System, the variables' names (5) |
| an update's outcome, after the restart | no | no | System, from the helper's status file (5) |
| the server's start and stop | no | no | System (5) |
| a Cube's address from discovery (`note`) | no | no | the Cube's, `address_changed` (5) |
| a Cube found on the LAN, gone from it | no | `cube_found`, once per Cube | server-wide, `cube_found`, `cube_gone` (3, 5) |
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
The UI's device requests (`app.js`, `config.js`, `cubes.js`, `tiles.js`, six
call sites) put the radio address in the path, and the audit's target is
`device:` with whatever the path held.

**The removed device:** `Client.ForgetDevice` (`internal/cubelan/client.go`)
calls `Snapshot.Forget`, which drops the device's configuration, state and
metadata, and only then emits `EventForgotten`;
the pump finds no state, and the line has nothing but the radio address.
The fix: `Forget` returns the device's last metadata (type, serial, name, room)
or its configuration's serial, the event carries it, and the pump builds the
view from it.
The line reads "Removed thermostat TEQ0000001 (Window)", the name as it was.

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
The UI sends the S/N, and the audit's target becomes `device:TEQ0000001`
whichever form the path held; older rows keep theirs.

### Finding 4: what metadata and config lines say

The Cube greets every connection with an H line (hello: serial, firmware,
duty cycle, free slots, clock), its M line (the rooms and devices, sometimes in
parts), a C line per device and one of its own, then its L line (the states);
`Snapshot.apply` takes each on its own, and `connected` follows the L line.
Every M line emits `metadata` with no detail, the greeting's included,
and every C line `config`;
so a reconnect logs `hello`, `metadata` and one `config` per device,
the Cube's own included, even when nothing changed.
`state` already carries its differences (`view.Changes`).
An H line comes only in a greeting.
None of these is in the server's text log; it says "cube connected" at Info.

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

**A reconnect**, after a loss of five minutes with a room renamed meanwhile,
newest on top; at info:

```
20:05:12  metadata      Room "Bath" renamed "Bathroom", while disconnected
20:05:12  connected     12 devices in 5 rooms, firmware 1.1.3, duty cycle 3 %, 180 slots free; down for 5 min
20:00:07  disconnected  read tcp …: i/o timeout
```

and with debug chosen, the greeting's own lines among them:

```
20:05:12  metadata      Room "Bath" renamed "Bathroom", while disconnected
20:05:12  connected     12 devices in 5 rooms, firmware 1.1.3, duty cycle 3 %, 180 slots free; down for 5 min
20:05:12  config        Cube KEQ0000000  portal off, time zone CET/CEST
20:05:12  config        Window left      comfort 21.0, eco 17.0, offset 0.0, …
          … one per device, 12 in all …
20:05:12  metadata      5 rooms, 12 devices
20:05:12  hello         firmware 1.1.3, duty cycle 3 %, 180 slots free
20:00:07  disconnected  read tcp …: i/o timeout
```

The driver marks a greeting's H, M and C events (`greeting`) and says why the
connection began (`cause`: the start, after a loss, or a reconnect the server
asked for).
The server compares the greeting's metadata and configurations with what it
held before: in memory within a run, and after a restart the newest backup
(its rooms, devices and configurations, decoded), which the backup watch reads
at its first look anyway.
What differs follows `connected` at info, "while disconnected";
the greeting's own lines are stored at debug, each with its summary.
A reconnect the server asked for (the follow-up's, an address change) writes
no `connected` line at info, only its differences, and its greeting at debug.
Outside a greeting, a `config` with differences is at info, one without at
debug (a follow-up re-read that finds the same C line),
and a first one says "configuration read".
The stream sends every event with its level, since `app.js` reloads the rooms
on `metadata` and the Cubes on `hello`.
In the text log, `cubelan` writes one Debug line per greeting message:
"greeting: hello", "greeting: metadata", "greeting: config", "greeting: list",
each with its gist (firmware, duty cycle and slots; rooms and devices; the
device's S/N and type; the devices listed).

### Levels

`debug`, `info`, `warn` and `error`, as `MASSIMO_LOG_LEVEL` names them;
stored as slog's numbers (−4, 0, 4, 8), so that "this level and above" is
`level >= ?`.
Each happening takes the level the text log gives it:
`disconnected`, an address that differs from the configured one, an automatic
backup or a restore that failed, an update that failed are `warn`;
the greeting's own lines and a C line that changed nothing are `debug`;
the rest `info`.
`GET …/log?level=` keeps that level and above, `info` by default;
the stream sends every level, and the log view drops what is below its choice.
A client that read `kind=metadata` before now asks `level=debug` for the
greeting's.

### The store: migration 0009

`events.cube_id` is `NOT NULL REFERENCES cubes (id) ON DELETE CASCADE`
(`0001_initial.sql`), and SQLite cannot drop a NOT NULL with `ALTER TABLE`.
So 0009 rebuilds the table, the documented way, in the migration's one
transaction: a new table, the rows copied with their new columns, the old one
dropped, the new one renamed, the indexes made.
No table references `events`, so the foreign keys stay on.
Should AP ServerAnnouncements' migration merge first, this one is 0010.

```sql
CREATE TABLE events (
    id        INTEGER PRIMARY KEY,
    uuid      TEXT    NOT NULL UNIQUE,
    cube_id   INTEGER REFERENCES cubes (id) ON DELETE CASCADE,  -- NULL: the server's
    time      TEXT    NOT NULL,
    kind      TEXT    NOT NULL,
    source    TEXT    NOT NULL DEFAULT '',  -- device, cube, system, config; '' an older binary's row
    level     INTEGER NOT NULL DEFAULT 0,   -- slog's: -4 debug, 0 info, 4 warn, 8 error
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
- **The level has no index:** `level >= ?` is a range, and an index on it would
  give up the id order; it is checked on the rows the walk reads.
  An info page reads past the debug rows among them, about fourteen per
  reconnect with twelve devices; a page of warnings may read further back.
- **The backfill**, in the copy:
  `serial` from `data`'s `state.serial`, else from another row of the same Cube
  and radio address (one pass builds that map over the old index
  (cube_id, device), the copy uses it), else for the Cube's own C line from
  `cubes.serial` where `cubes.rf_address` matches;
  `source` `device` for a device's rows, `system` for the restore's,
  `cube` for the rest; `level` `warn` for `disconnected`, `debug` for `hello`,
  `info` for the rest; `user_id` NULL.
  A row it cannot place keeps `serial` empty and ages out with `log.keep_days`.
- **`min_reader` stays 1:** an older binary reads its Cube's rows by
  `cube_id = ?` and never meets a NULL one, and writes rows without the new
  columns, which take their defaults; the new binary fills `source`, `level`
  and `serial` of such rows (`source = ''`) at its start.
  A device page of an older binary walks the Cube's rows without its index,
  the 1.3 s at 90,000 rows that 0004 fixed: slow, not wrong.
- **The insert** gains two indexes; `TestEventSize` gives the bytes, and the
  owner's Raspberry Pi 1 the time, with the owner's go (D12-A):
  a batch of 100 events on 0008 and on 0009, and a page of warnings, with a
  test binary on a copy of its store.

### The page

A Cube's page is two index walks, each limited to the page's size and in id
order: the Cube's rows (`cube_id = ?`) and the server's (`cube_id IS NULL`),
with the same terms, merged in Go, the first `limit` kept.
One query with `cube_id = ? OR cube_id IS NULL` would sort.
The cursor stays one id, since both walks share one sequence;
a time bound becomes an id bound per walk, as now.
`TestEventsPageQuery` checks every shape's plan for a sort.
A device's page (`serial = ?`) needs the Cube's walk only.
The route gains `source=` (`device`, `cube`, `system`, `config`), `level=`
and `user=` (a login name, as the audit's route takes it);
`device=` takes a S/N or a radio address.

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
| `cube` | `connected`, `disconnected`, `hello`, `metadata`, `config` (the Cube's own), `pairing_started`, `pairing_ended`, `address_changed`, `cube_found`, `cube_gone` |
| `system` | `server_started`, `server_stopping`, `settings_read`, `restart`, `reboot`, `reboot_cancelled`, `update`, `update_result`, `backup`, `backup_deleted`, `backups_pruned`, `log_pruned`, `restore_started`, `restore_step`, `restore_done` |
| `config` | `change`: its `action` (the audit's, such as `cube.update`), its `target`, its `changes` from → to |

Server-wide rows (`cube_id` NULL): the server's start, stop, restart, reload,
reboot, update, prunes, `cube_found`, `cube_gone`, and the changes of settings,
users, roles, keys and capabilities.
A Cube's own rows: its connection, address, rooms, devices, pairing, backups,
restore, and its own changes (named, address, proxy, hidden, added, removed).
They reach every Cube's stream, and are stored once.

### The schema file

`internal/store/schema.sql` is the schema as the last migration leaves it:
`TestSchemaFile` applies every migration to an empty file, writes
`sqlite_master`'s statements in a fixed order (tables by name, then their
indexes), headed by the schema version and its `min_reader`,
and fails with the difference when the file says otherwise;
`go test ./internal/store -run TestSchemaFile -update` writes it again.
One file is enough, since git keeps each version's (`git show <tag>:…`).
It is for reading, and for a review: a migration's pull request shows its
effect as a difference of the file.
A store is still made by the migrations:
they also insert rows (roles and capabilities in five of the eight, the node
and tenant ids in Go), and SQLite keeps an altered table's statement apart from
a fresh one's, so a second way to make a store would need a test of its own.
`docs/store.md` section 5 keeps its readable listing and points to the file for
the complete one.

### What changes, by file

| File | Block | Change |
|------|-------|--------|
| `internal/cubelan/snapshot.go`, `client.go` | 1, 4 | `Forget` returns the last metadata; the forgotten event carries it; `greeting` and `cause` on the events; the greeting's Debug lines |
| `internal/server/server.go` | 1, 3–5 | the pump: `serial`, `source`, the Cube's own C line, the levels, the greeting folded; `logServer` for the server's rows |
| `internal/server/logdiff.go` (new) | 4 | the differences of metadata and configurations, the summaries |
| `internal/server/log.go` | 1, 3 | `device=` by S/N, `source=`, `level=`, `user=`, the two walks, `eventKinds` |
| `internal/server/config_api.go`, `server.go`, `audit.go` | 1 | `…/devices/{rf}` takes the S/N (D4-A), one helper for the five call sites; the target by S/N |
| `internal/server/discovery.go` | 3, 5 | `cubeFound` and `cubeGone` write server-wide rows; `note` writes `address_changed` |
| `internal/server/backups.go`, `backups_api.go`, `restore_api.go`, `restart_api.go`, `reboot_api.go`, `update_api.go` | 5 | their System lines |
| `cmd/massimo/serve.go` (or `internal/serve`), `reload.go` | 5 | start, stop, the settings file read again, the update's outcome |
| `internal/server/audit.go` and the handlers | 6 | the `change` line beside the audit row |
| `cmd/massimo/user.go` | 6 | the command line's `change` lines |
| `internal/store/schema.sql` (new), `store_test.go` | 2, 3 | the generated schema and its test |
| `internal/store/migrations/0009_events_sources.sql`, `events.go`, `migrate.go` | 3 | the rebuild, the backfill, the columns, the walks, the prune |
| `internal/server/ui/log.js`, `app.js`, `config.js`, `cubes.js`, `tiles.js`, `partials/log.html`, `i18n/*.json` | 1, 4–7 | the S/N, the lines' texts, the choices |
| `tools/ui-test/` | 1, 4, 7 | the log's S/N, lines and choices at 1100 and 390 px |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml` | 1, 3–6 | the events' fields and kinds, the route's parameters, the device path |
| `docs/ui.md` | 1, 4–7 | the log |
| `docs/store.md` | 2, 3 | the schema file, the schema, migration 0009, the page |
| `README.md` | 6 | `massimo user …` writes the log too |
| `CHANGELOG.md` | 1–7 | a bullet each under Unreleased |

### Risks

- **Two plans' migrations:** AP ServerAnnouncements also plans 0009;
  the second to merge renumbers, and its tests say so (`TestMigrationSetChecked`).
- **`serve`'s wiring moves** under AP ServerAnnouncements' Block 1:
  Block 5 follows it there.
- **The insert:** two indexes more per row; measured, not guessed.
- **A page of warnings** walks back past every info and debug row: measured on
  the Pi with the insert; an index per level is the fallback.
- **The reference after a restart** is the newest backup, taken some minutes
  after a change (`backups.auto_min`): a change made just before a stop can be
  logged again after the start.
- **The greeting at debug** adds about 20 kB per reconnect with thirty devices
  (`TestEventSize`'s 640 bytes a row), within `log.keep_days`.
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
One functional commit per block, citing `AP Logging v1.1, Block <n>`,
with its CHANGELOG bullet in the wrapped style of its neighbours.

### Block 1: LogSerial

1. `internal/cubelan`: `Snapshot.Forget` returns the device's last metadata
   (type, serial, name, room) or, for an unlisted device, its configuration's
   serial; `Event` gains `Gone`, set by `ForgetDevice`.
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
   `404` on a path and an empty page on the log;
   the audit's target is `device:` and the S/N, whichever form the path held.
   `TestDeviceRoutesTakeSerial`: each device route with the S/N sends the fake
   Cube the same line as with the radio address;
   `TestLogDeviceFilterBySerial`, `TestAuditTargetBySerial`.
4. UI: the log line names the device, else its S/N, with the radio address in
   expert mode only (as now); "Removed thermostat TEQ0000001 (Window)";
   the Cube's own line as "Cube KEQ0000000"; the device choice "Name ·
   TEQ0000001", its value and the address's `device=` the S/N, an old link's
   radio address read and replaced;
   every device request sends the S/N (`app.js`, `config.js`, `cubes.js`,
   `tiles.js`), the radio address only for a device without one;
   the fallbacks of `config.js`, `cubes.js` and `tiles.js` to the S/N.
   `tools/ui-test`: the log's S/N and the removed line.
5. `docs/api-v1.md` ("Identifiers", the device routes, `GET …/log`, the
   events' `serial` and `source`), the OpenAPI file, `docs/ui.md` ("Log").
6. `CHANGELOG.md`, Unreleased, Fixed:

   ```
   - The log names a device by its S/N, the radio address in expert mode
     only, and a removed one as "Removed thermostat TEQ0000001 (Window)";
     the device's routes take its S/N too (`docs/api-v1.md`, "Identifiers").
   ```

**Verification:** the test gate; the new tests fail before steps 1 to 3;
a device removed from the fake Cube logs its S/N.

### Block 2: SchemaFile

1. `internal/store/store_test.go`: `TestSchemaFile` and its `-update` flag.
2. `internal/store/schema.sql`, written by it at version 8.
3. `docs/store.md`, section 5 and "Migrations": the file, how it is made and
   checked.
4. `CHANGELOG.md`, Unreleased, Added:

   ```
   - `internal/store/schema.sql` is the store's complete schema, made from
     the migrations and checked by `go test` (`docs/store.md`,
     "Migrations").
   ```

**Verification:** the test gate; a column added by hand to the file fails the
test, `-update` brings it back.

### Block 3: LogStore

1. `internal/store/migrations/0009_events_sources.sql` and its Go step
   (`afterMigration[9]`): the rebuild and the backfill of the Analysis;
   `schema.sql` written again.
   `TestMigration9` (rows, ids and uuids kept; `serial`, `source` and `level`
   filled, the Cube's own C line placed; a NULL `cube_id` taken),
   `TestMigratesAVersionOneFile` through 0009,
   `TestOlderReaderOnNine` (the 0008 queries and insert on a 0009 file).
2. `internal/store/events.go`: `Event` gains `Source`, `Serial`, `Level`,
   `UserID`, `CubeID` 0 for the server's; `EventQuery` gains `Source`,
   `Serial`, `Level`, `UserID` and `Server` (with the server's rows);
   the two walks merged; the prune's pass for `cube_id IS NULL`;
   the start's fill of `source = ''`.
   `TestEventsPageQuery` for every new shape, `TestEventsWithServerRows`
   (the merge, the cursor, the time bounds), `TestEventsByLevel`,
   `TestPruneServerEvents`, `TestEventSize` updated.
3. `internal/server`: `logServer` stores a server-wide event once and sends
   it to every Cube's stream (and to the rings of Cubes without a row);
   `cube_found` moves to it; every event carries `level`;
   `GET …/log` takes `source=`, `level=` and `user=`;
   `by` filled at read and on the stream.
   `TestLogSourceFilter`, `TestLogLevelFilter`, `TestLogUserFilter`,
   `TestFoundCubeStoredOnce`.
4. `docs/store.md` (section 5's schema, "Migrations", stage (d)'s note),
   `docs/api-v1.md` and the OpenAPI file (`GET …/log`'s parameters, `level`,
   `by`, the server-wide events).
5. `CHANGELOG.md`, Unreleased, Added:

   ```
   - The log keeps the server's own events beside the Cube's, each with
     its source, its level and who started it; the store's migration 0009
     rebuilds its events table once (`docs/store.md`, "Migrations").
   ```

**Verification:** the test gate;
on the owner's Raspberry Pi 1, **only with the owner's go** (D12-A):
a store test binary for linux/arm (GOARM=6) run on a copy of the box's
`massimo.db` in a scratch directory, timing a batch of 100 events on 0008 and
on 0009, and a page of warnings; the service and its file untouched, only the
times leave the box; the times go into `docs/store.md` and a NOTE.

### Block 4: LogDetail

1. `internal/cubelan`: `greeting` on the H, M and C events of a greeting;
   `cause` on `EventConnected` (`start`, `loss`, `reconnect`);
   a Debug line per greeting message (D9-B).
2. `internal/server/logdiff.go`: the differences of two metadata (the table of
   Finding 4) and of two configurations of each type, hidden devices left out;
   the summaries of a hello, a metadata and a configuration;
   the `connected` summary (devices, rooms, firmware, duty cycle, free slots,
   down for).
   `TestMetadataChanges`, `TestConfigChanges`, `TestGreetingSummaries`,
   table tests with the recorded greeting.
3. The pump: `metadata` and `config` carry `changes`; a greeting's own events
   at debug with their summaries; `connected` at info with `summary`, after the
   start and a loss, its differences after it at info;
   the reference is the snapshot before the loss, and after a restart the
   newest backup.
   `TestReconnectAtInfoIsOneLine`, `TestGreetingAtDebug`,
   `TestChangedWhileDisconnected`, `TestChangedWhileServerDown`,
   `TestSameConfigAtDebug`.
4. UI: the texts of every change and summary, in all six languages;
   the log asks `level=info` (expert mode `debug`) until Block 7's choice.
   `tools/ui-test`: a reconnect's line.
5. `docs/api-v1.md` (the events' `changes`, `summary`, `greeting`, `cause`,
   the levels), the OpenAPI file, `docs/ui.md` ("Log").
6. `CHANGELOG.md`, Unreleased, Changed:

   ```
   - The log's metadata and config lines say what changed, and a reconnect
     is one line with the Cube's devices, rooms, firmware and duty cycle;
     the greeting's own lines are kept at debug (`docs/ui.md`, "Log").
   ```

**Verification:** the test gate; a fake Cube reconnecting with nothing changed
logs one line at info, with a room renamed in between two, and its greeting
at debug.

### Block 5: ServerEvents

1. `serve`'s start and stop, in `cmd/massimo/serve.go` or where AP
   ServerAnnouncements moved them: `server_started` (the version, the one
   before when it changed, kept in `settings`), `server_stopping` (why: a
   signal, a restart, an update, a reboot), written before the store's writer
   stops; `update_result` once from the helper's status file when it is newer
   than the last one logged.
2. `cmd/massimo/reload.go`: `settings_read`: why, the variables changed and
   those waiting for a restart, names only (D3-A), when something changed or
   a request asked.
3. `internal/server`: `restart`, `reboot`, `reboot_cancelled`, `update` with
   who; `backup` (its reason, with who for one by hand), `backup_deleted`,
   `backups_pruned`, `log_pruned` (only when rows went), `restore_started`;
   `restore_step` and `restore_done` under System;
   `address_changed` in `note`, `cube_gone` in `cubeGone`; the failures at
   `warn`.
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

### Block 6: ChangeEvents

1. `internal/server`: a table beside `actions` maps each audited action to its
   line: `change` under Configuration, a System kind of Block 5, or none
   (sign-in, sign-out, preferences, the search, the ticket); setpoints and
   modes included (D8-A).
   A successful request writes it next to its audit row, with `user_id`:
   its `action`, its `target` (a device by S/N), its `changes` from → to
   (a Cube's name "" → "Upstairs"); a user, role or key with its ids, uuid
   and names as they were then; never a secret.
   `TestEveryActionHasALine` (as the audit's test for `actions`),
   `TestCubeNamedIsLogged`, `TestUserChangeCarriesIds`, `TestNoSecretInLine`.
2. `cmd/massimo/user.go`: each command writes its `change` line with the
   audit row, straight into the store (`via: "cli"`).
   `TestUserCommandsLog`.
3. `by` for everyone who reads the log (D2-A).
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

### Block 7: LogFilter

1. UI: one choice beside the kind: all, System, Configuration, the Cube,
   then each device by name and S/N; a choice of level (info and above by
   default, warnings, debug); a choice of who for a server with users;
   the kind choice grouped by source.
   The address after `#log`: `source=`, `level=`, `device=` a S/N, `user=`.
   Every text in all six languages; phone width first.
2. `tools/ui-test`: the choices, the address, a reload, at 1100 and 390 px.
3. `docs/ui.md` ("Log"): the choices and the address.
4. `CHANGELOG.md`, Unreleased, Added:

   ```
   - The log chooses a source, System, Configuration, the Cube or one
     device, a level and who made a change, kept in the address after
     `#log` (`docs/ui.md`, "Log").
   ```

**Verification:** the test gate; `make ui-test` at 1100 and 390 px.

## Proposed commit message

The revision's commit, on `main`, before the run's first project write:

```
chore(aps): +Logging v1.1

The owner's answers at the gates of v1.0: the greeting's lines kept at debug beside the one connected line at info, with a level stored and chosen; the UI sends the S/N; a generated schema file; the Pi checks the insert only; the other decisions as recommended.

Key changes:
- GUIDELINES/wip/2026-11-09_00-23_AP_Logging_log_sources_and_serials.md: seven blocks, the schema file as Block 2 and the store before the lines' detail; the levels; the decisions settled; beside AP ServerAnnouncements' migration 0009.

File-revision-v1: GUIDELINES/wip/2026-11-09_00-23_AP_Logging_log_sources_and_serials.md::v1.1::2026-11-09::TL;DR, Discussion, Analysis, Implementation Plan::the owner's decisions D1 to D12, the levels and the schema file
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `fix(log): A device is named by its S/N, a removed one too`
2. `feat(store): The store's schema in one generated file`
3. `feat(store): The log keeps the server's events, their source and level`
4. `feat(log): Lines say what changed, the greeting's own at debug`
5. `feat(log): The server's own jobs and a Cube's address in the log`
6. `feat(log): Every change says who made it`
7. `feat(ui): The log chooses a source, a level and who`

## Change History

- v1.0 (2026-11-09): the first revision, from the owner's findings on
  v0.12.0-alpha.1 and the store input, after FoundCubes merged;
  D1 to D8 open at its gate.
- v1.1 (2026-11-09): the owner's answers at the gates of v1.0:
  D9-B, the greeting's lines at debug beside `connected` at info, with a level
  column chosen in the API and the UI, replacing D6; D4-A with the UI sending
  the S/N; D10-A, the schema file, as Block 2, with the store before the lines'
  detail; D12-A, the Pi times the insert only; the removed line with the name as
  it was; D1 to D3, D5, D7, D8 and D11 as recommended; beside AP
  ServerAnnouncements' migration 0009.
