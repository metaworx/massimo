# AP OneLog v1.1: One log, tagged lines

## TL;DR

The store's `events` become `log`, whose rows may name their command;
the process log tags every line, writes JSON to its file,
rotates the file itself, and is written from the same records as the log rows.

- `-log-format`, `-log-file-format`, `-log-process-format` `json|plain`
  (`none` for stderr): specific → general → default (stderr plain, file JSON)
- `-log` file 0644 → 0640, group `adm` through a set-group-id directory
- rotation in serve: size, a format change, SIGHUP;
  kept by count and a minimum total; logrotate dropped; the Mac on `-log`
- a rotation says so: the old file's last line,
  then `log_rotated` in the new file, the store and on stderr
- `source=`, `kind=`, `cube=`, `command=` on every process-log line
- `events` → `log` + `command_id` + `visibility` (migration 0014);
  `commands` with `AUTOINCREMENT`, a pruned id never given out again
- one write path: an event is its slog line; duplicate `slog` calls go
- command steps, queue holds, sign-ins, lockouts → log rows
- proxy clients → commands rows; connections, refusals, deletes → log rows
- `audit.keep_days`, 365; the Log page shows proxy rows and a row's command

### Blocks

1. Process-log formats and the log file's permissions
2. Rotation in serve, its notices, the deb and the Mac
3. Tags on every process-log line
4. Store: migration 0014 for `log` and `commands`
5. One write path for events and their slog lines
6. Commands, queue holds, sign-ins and lockouts as log rows
7. The proxy's clients: commands rows and connection lines
8. Audit retention, `audit.keep_days`
9. The Log page: proxy rows, a row's command and its steps

## Discussion

The owner asked on 2026-11-09 whether a client of the LAN proxy, such as
Home Assistant's MAX! integration, leaves log entries.
It leaves lines in the process log (journald,
`serve.log`) and nothing in the store: the proxy never emits an event,
its commands have no row in `commands` (no parent request, `commands.go`,
`enqueued`),
and the device's `state` line that follows a proxy write names nobody.
The conversation then went over the three records - `events` (what happened),
`audit` (who asked for what),
`commands` (what became of a request on the radio) -
and the owner proposed one log as the master, with the others as details.

**The owner's decisions**
(TL;DR gates of 2026-11-10 and 2026-11-11, `PLAN++` as recommended):

- **D1-A:**
  one file, the process log, every line tagged with its source, kind,
  Cube and command, so that a tool can pick the audit or a command out of it;
  no `-audit` or `-commands` file.
- **D2-B:**
  JSON, with `-log-format json|plain`; without a flag,
  plain on stderr and JSON in a file.
- **D5:**
  `-log-file-format` and `-log-process-format` beside `-log-format`,
  which applies to both; the more specific flag wins over the general one,
  which wins over the default; each has its `MASSIMO_LOG_…` variable,
  a flag winning over its variable as every serve flag does.
- **D3-A:**
  the log file 0640, readable by the admin group (`adm` on Debian),
  in this plan.
- **D10-A:**
  serve rotates its own `-log` file on every platform: at a size,
  at once when the format changes, and on SIGHUP where it is a reload;
  logrotate's names, so that its files join the retention;
  the oldest deleted only while more than N are kept and the rest still covers a
  minimum, so that many small files stay.
- **D4-A:**
  the Mac passes `-log` as the other platforms do;
  launchd's `StandardErrorPath` becomes `serve.stderr.log` with
  `-log-process-format none`, for what the Go runtime prints itself.
- **D11-A:**
  rotate at 10 MB, keep 8, delete past 8 only while the rest covers 5 MB;
  `-log-rotate-size`, `-log-keep`, `-log-keep-min`.
- **D12-A:**
  every rotated file gzipped but the newest, as logrotate's `delaycompress`.
- **D6-A:**
  a command's `queued` and `sent` at debug, acknowledged at info, discarded,
  retried and failed at warn.
- **D7-A:**
  `audit.keep_days`, 365 by default, 0 keeps the audit.
- **D8-A:**
  `events` renamed `log` in the migration that adds its columns.
- **D9-A:**
  the Log page in this plan.

**After Block 1** (block gate of 2026-11-11, `PLAN++ D13A D14B`):

- **D13-A:**
  a rotation says so twice: the old file's last line, in that file's format,
  with the reason and the new name; and, once the new file is open,
  a `log_rotated` system event - the new file's first line,
  a row in the store and a line on stderr -
  its summary the files kept and deleted.
  The event is sent after the writer's lock is released,
  since a line logged from inside the writer would wait for that lock.
- **D14-B:**
  `commands` made again with `AUTOINCREMENT` in migration 0014,
  so that a pruned command's id is never given to the next one (the finding of
  the session of AP BackupIds:
  without it SQLite gives a new row the highest id plus one,
  so the prune of the newest command hands its id out again,
  and `GET /api/v1/commands/{id}` read before the prune names another command).
  Foreign keys are off around that migration,
  since `DROP TABLE` with them on deletes the rows first,
  which would set every `audit.command_id` to NULL; not A,
  a prune that keeps the newest command, which leaves the schema as it is.

Settled on the way: migration 0013 is AP BackupIds' (merged as pull request
#178), so this plan's migration is 0014;
the master/detail link runs from the log row to the command (`log.command_id`),
since one command has many lines - its attempt, a discard and retry,
its outcome, the device's new state.

## Analysis

**Where the process log goes today.**
`serve` writes slog text to stderr, and with `-log` (`MASSIMO_LOG`) to the
file as well (`serve.go`, an `io.MultiWriter`, one `TextHandler`).

| Platform | Lines go to |
|----------|-------------|
| systemd | journald (stderr) and `/var/log/massimo/serve.log` (`-log`), rotated weekly by `packaging/logrotate/massimo` (`copytruncate`) |
| launchd | `/var/log/massimo/serve.log`, launchd's `StandardErrorPath`; no `-log`; never rotated |
| Windows service | `serve.log` in the data directory (`-log`); never rotated; `service.log` is the wrapper's |
| a terminal | stderr |

`serve` creates the file 0644 (`os.OpenFile(…, 0o644)`);
`copytruncate` keeps that mode.
The Mac's installer makes it 0640 (`darwin.go`, `logFile`).
On Linux the unit's `LogsDirectory=massimo` makes the directory,
`_massimo:_massimo` 0755; systemd resets its owner at every start,
so a group of the directory's own needs the line gone,
and the postinst and `massimo install` make the directory instead.

**Group `adm`.**
serve runs as `_massimo`, which is no member of `adm` and must not become one
(it would read every system log);
a file created in a set-group-id directory takes the directory's group,
membership or not, so `/var/log/massimo` becomes `_massimo:adm` 2750 and
every file serve creates there is `_massimo:adm` 0640.
Where `adm` does not exist the directory keeps the service user's group.

**Two formats, two outputs.**
Go 1.26's `slog.NewMultiHandler` fans one record out to a text handler on
stderr and a JSON handler on the file, both at the one `logLevel`.
A plain line starts with `time=`, a JSON line with `{`:
the first byte of the file's last line says its format,
read from at most the last 64 KiB.

**What reads the log.**
CI greps the Windows job's `serve.log` for `update helper started`,
which a JSON line still holds;
the mDNS jobs grep a redirected stderr for `msg="announced on the LAN"`,
which stays plain; the Mac job greps launchd's file, which Block 2 moves.
`tools/store-fixture/make.sh` redirects stderr.

**The three records** (schema in `internal/store/migrations/store.sql`):

| | events | audit | commands |
|---|---|---|---|
| row | one thing that happened | one changing request, refused ones too | one request, its radio steps beneath it |
| written | in batches, in the background; may drop under load | at once, in the request | at once, then updated |
| kept | `log.keep_days`, 30 | never pruned, though `docs/store.md` §5 says it is | `commands.keep_days` |
| read with | `read` | `audit` | `read`, by id |
| link | - | `command_id` → commands | - |

**Only in the process log today**, at info and above:
the proxy (connections, the commands it passes, its deletes and refusals);
the Cube link's retries (`cube reconnecting`, `poll failed`);
the queue's holds (duty cycle, memory slots, a discard and retry);
the follow-up (`budget refreshed after writes`);
discovery and the listeners; TLS; push delivery; startup warnings;
the store's own faults, which can never go into the store.

## Implementation Plan

Blocks 1 and 2 go into one pull request,
so that no release has JSON in a file that is not rotated at the change.
Each block is its own commit or commits, with its fragment in `CHANGELOG.d/`.

### Block 1: Process-log formats and the log file's permissions

1. `cmd/massimo/logging.go`:
   a format type (`json`, `plain`, and `none` for stderr alone),
   `parseLogFormat`,
   and `newServeLogger(stderr, file, formats)` that builds a `TextHandler`
   or `JSONHandler` per output and `slog.NewMultiHandler` over those not `none`,
   at the one `logLevel`;
   `newLogger` of the other commands stays plain on stderr.
2. `cmd/massimo/serve.go`:
   `-log-format`, `-log-file-format`, `-log-process-format`,
   defaulting to `MASSIMO_LOG_FORMAT`, `MASSIMO_LOG_FILE_FORMAT`,
   `MASSIMO_LOG_PROCESS_FORMAT`; the file's format is `-log-file-format`,
   else `-log-format`, else `json`; stderr's is `-log-process-format`,
   else `-log-format`, else `plain`;
   an unknown value is an error naming the flag.
3. The file opened 0640; a file of the service user's own that is wider is
   narrowed to 0640 at the open.
4. `internal/env`:
   the three variables known; `cmd/massimo/reload.go`'s `flagVars`:
   the three flags, so that a change applies at the next start.
5. Linux: `packaging/systemd/massimo.service` without `LogsDirectory=`;
   `packaging/deb/postinst` and `massimo install` make `/var/log/massimo`
   `_massimo:adm` 2750 (the service user's group where `adm` is missing),
   and narrow `serve.log*` there to 0640 in that group;
   `packaging/deb/test.sh` and the install tests check it.
6. Documents: `README.md` (serve's flags, the log), `packaging/env.template`,
   `packaging/README.md`, `docs/server.md` where it names the log's format.

**Verification:**
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .`, `go vet` for linux,
windows and darwin, `make cross`, `make deb-systemd-test`; tests:
each flag's precedence over the general one and the default, `none`,
an unknown value, both outputs from one record,
the file's mode on a new and on a wider existing file.

### Block 2: Rotation in serve, its notices, the deb and the Mac

1. A small package (`internal/logfile`):
   a writer over the `-log` file that rotates `serve.log` → `serve.log.1`,
   the older ones up by one, gzipping all but `.1`,
   when the file reaches `-log-rotate-size` (10 MB),
   when its last line's format differs from the one it writes (at the open),
   and on request;
   it deletes the oldest only while more than `-log-keep` (8) are kept and the
   rest covers `-log-keep-min` (5 MB); a rename it cannot make is a warning,
   and the writer appends.
2. serve rotates on SIGHUP where SIGHUP is a reload,
   and on `POST /api/v1/reload`, which does what SIGHUP does.
3. The notices (D13-A): the writer ends the old file with a line in its format,
   `log rotated`, with the reason (`size`, `format`, `sighup`,
   `reload`) and the new name; once its lock is released,
   serve sends the `log_rotated` system event (source `system`,
   its summary the reason, the files kept and deleted),
   which is the new file's first line, a row in the store and a line on stderr;
   `docs/api-v1.md` lists the kind.
4. The flags with their `MASSIMO_LOG_…` variables, in `flagVars`.
5. The deb: `packaging/logrotate/massimo` and the `logrotate` dependency go,
   the conffile removed through the maintainer scripts;
   the files logrotate made keep their names and join the retention.
6. The Mac: the plist passes `-log` and `-log-process-format none`;
   `StandardErrorPath` is `serve.stderr.log`;
   `massimo install` writes it so, and CI's Mac job greps the new file.
7. Documents: `README.md`, `packaging/README.md`, `docs/update-windows.md`
   (serve.log is rotated now).

**Verification:**
as Block 1, and `make lan-test` is not needed; tests: rotation at the size,
at a format change, on request; retention by count and minimum;
gzip of all but the newest; logrotate's existing names adopted;
a refused rename; the old file's last line and the event after it.

### Block 3: Tags on every process-log line

Every package's logger is `With("source", …)` (`proxy`, `cube`, `queue`,
`api`, `tls`, `store`, `discovery`, `push`, `system`), and `cube=` where the
line concerns one; `kind=` and `command=` come with Block 5.

**Verification:**
the test gate; a test that every logger the server hands out carries a source.

### Block 4: Store: migration 0014 for `log` and `commands`

1. `0014_log.sql`:
   the table made again as `log` (as 0009 made `events`), every row copied,
   with `command_id` (a reference to `commands`, `ON DELETE SET NULL`)
   and `visibility` (`read` or `audit`, `read` by default),
   its indexes, one on `command_id`.
2. In the same migration `commands` made again with
   `INTEGER PRIMARY KEY AUTOINCREMENT` (D14-B), every row copied with its id,
   `parent_id` and the rest, its indexes made again,
   `sqlite_sequence` starting at its highest id;
   the migration runner runs a migration that declares `-- foreign_keys: off`
   with foreign keys off (set outside its transaction),
   and checks `PRAGMA foreign_key_check` before the commit,
   so that `audit.command_id`, `log.command_id` and `parent_id` keep their rows;
   a test that the audit's links survive the migration,
   and that a pruned newest command's id is not given out again.
3. `store.sql` written again
   (`go test ./internal/store -run TestStoreSQL -update`);
   the dump of v0.14.0-alpha.1, the newest release,
   added as `internal/store/testdata/releases/v0.14.0-alpha.1.sql` by
   `tools/store-fixture/make.sh v0.14.0-alpha.1` (`docs/store.md`,
   "Migrations");
   `TestMigratesReleasedStores` migrates every dump through 0014 and checks the
   commands' `sqlite_sequence` beside the backups'.
4. `GET …/log?command=<id>`;
   rows of `audit` visibility only for a caller with `audit`; `docs/api-v1.md`,
   the OpenAPI file, `docs/store.md`.

**Verification:**
the test gate with `-race` for `internal/store` and `internal/server`;
the migration's copy, the commands' ids and the audit's links,
the filter, the visibility.

### Block 5: One write path for events and their slog lines

`emit` and `logServer` write the event's slog line from the event itself (level,
kind, source, device, serial, command, by),
and the separate `slog` calls that say the same go (`device state`,
`cube connected`, …);
the process log is then the log's rows and the lines the store cannot hold.

**Verification:**
the test gate; a test that an event's line and row agree.

### Block 6: Commands, queue holds, sign-ins and lockouts as log rows

The queue's observer writes a log row per step with its `command_id` (D6-A
levels); the duty-cycle and memory-slot holds are warn rows; sign-ins,
failed ones and lockouts are rows of `audit` visibility;
a `change` row carries its command.

**Verification:**
the test gate; a setpoint's rows by `?command=`.

### Block 7: The proxy's clients: commands rows and connection lines

A command a proxy client passes on is a `commands` row, its `origin` the
client's address and no user, its step beneath it;
a client's connection, disconnection and refusal, its deletes,
and the proxy's warnings are log rows of source `proxy`.

**Verification:**
the test gate; `make lan-test`;
a proxy client's setpoint found by `?command=` with its `state` row.

### Block 8: Audit retention

`audit.keep_days` (365, 0 keeps), pruned beside the log, in chunks;
`docs/store.md` §5 says so.

**Verification:**
the test gate; the prune past the setting, and 0.

### Block 9: The Log page

The source filter knows `proxy`;
a row with a command opens the command and its steps,
and its audit row for a user with `audit`;
texts in all six `ui/i18n/<lang>.json`; `docs/ui.md`.

**Verification:**
`make ui-test` at 1100 and 390 px.

## Proposed commit message

Block 2:

```
feat(serve): serve rotates its log file and says so

serve rotates its -log file itself, at a size, at a change of its format and on a reload, keeps it by count and a minimum total, and says so at the end of the old file and in a log_rotated event (AP OneLog, Block 2).

Key changes:
- internal/logfile: the rotating writer, gzip of all but the newest, logrotate's names adopted, the retention by -log-keep and -log-keep-min
- serve: the rotation on SIGHUP and POST /api/v1/reload, the old file's last line and the log_rotated event after it
- the deb without logrotate; the Mac's plist on -log, its stderr in serve.stderr.log
- README.md, packaging/README.md, docs/update-windows.md and docs/api-v1.md

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- **v1.0** (2026-11-11): the first revision, by `PLAN++` as recommended,
  D10 with SIGHUP.
- **v1.1** (2026-11-11): after Block 1, by `PLAN++ D13A D14B`:
  the rotation's notices in Block 2 (D13-A);
  `commands` with `AUTOINCREMENT` in Block 4 (D14-B),
  with the migration runner's switch for foreign keys;
  Block 4 adds the dump of v0.14.0-alpha.1, released after v1.0.
