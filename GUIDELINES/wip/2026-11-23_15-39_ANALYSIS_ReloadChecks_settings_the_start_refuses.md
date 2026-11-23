# ANALYSIS ReloadChecks: the settings the start refuses, at a reload

Read on main at 48f3496e, read only, by an Opus sub-agent of the session
that ran AP LogEdges. A start probe ran the built binary with 50 bad values
(loopback only, no discovery, `MASSIMO_ENV_FILE=/dev/null`); a reload probe
ran in process through `go test -overlay`. Neither is kept.
It follows AP LogEdges' NOTE, "Found, not changed", items 1 and 2.

## TL;DR

About 20 settings stop `serve` at its start; a reload warns of 4 of them
(LogEdges Block 2).
A typo in any other is taken silently, and the next restart leaves the box
without its web UI until someone fixes the file on the box.

- one table of checks, run by the start first and by a reload over what
  changed → the texts match by design; a guard test keeps it complete
- the reboot switch's bad value → off, with a Warn on stderr only →
  `settings_read`'s warning, one line
- `MASSIMO_RESTART`, `MASSIMO_SIGHUP` not in `env.Known` → `massimo config`
  says "typo?", a reload never lists them → known
- a settings file that does not parse → a Warn on stderr only →
  `settings_read` at warn
- `MASSIMO_LOG`, `MASSIMO_DATA`, `MASSIMO_LISTEN`, the `-tls-http` bind:
  not checkable without side effects; every packaged service fixes the
  first three by flags

### Blocks

1. Known: `MASSIMO_RESTART` and `MASSIMO_SIGHUP` in `env.Known`
2. Table: one table of the start's checks, run at a reload; the reboot switch in it
3. Guard: a test that every setting the start refuses has its entry
4. Unreadable: a settings file that does not parse is a warning
5. (only with D8-B or C) Restart: the Maintenance card says what the next start refuses

## What the start refuses

The process exits 1 for each (all confirmed by the probe).
"Reload": pending = listed for the next start; at once = applied by a reload.
"Pure": checkable at a reload without side effects.
The process environment always wins over the file;
the global `-data` and `-log-level` act through it.

| Setting | Where the start checks it | Reload | Fixed by | Pure | Today at a reload |
|---|---|---|---|---|---|
| the file's syntax | `main.go:26` `env.LoadFile` | the whole file | `-env`, `-no-env` | yes | Warn on stderr only; API 500; no `settings_read` |
| `MASSIMO_LOG_LEVEL` | `serve.go:126`, `parseLogLevel` | pending | `-log-level` | yes | warns (LogEdges) |
| the three log formats | `serve.go:105`, `formats` | pending | their flags | yes | warns (LogEdges) |
| `MASSIMO_LOG_ROTATE_SIZE`, `_KEEP`, `_KEEP_MIN` | `serve.go:109`, `logRotation.options`; even without `-log` | pending | their flags | yes | silent |
| `MASSIMO_LOG` | `serve.go:119`, `logfile.Open` | pending | `-log` (every service) | no, opens the file | silent |
| `MASSIMO_HSTS` | `serve.go:146`, `hstsDays` | pending | `-hsts` | yes | silent |
| `MASSIMO_TRUSTED_PROXIES` | `serve.go:150`, `ParseTrustedProxies` | pending | `-trusted-proxies` | yes | silent |
| `MASSIMO_RESTART` | `serve.go:154`, `restartBy` | **not listed** (not known) | `-restart` | yes | silent |
| `MASSIMO_SIGHUP` | `serve.go:159`, `sighupAction`; Windows too | **not listed** (not known) | `-sighup` | yes | silent |
| `MASSIMO_REBOOT` | `serve.go:86/180`, `parseSwitch` | at once (`reload.go:175`) | `-allow-reboot` | yes | switched off; Warn on stderr only |
| `MASSIMO_ANNOUNCE` | `serve.go:88/183` | pending | `-announce` | yes | silent |
| `MASSIMO_TLS` | `internal/serve/tls.go:106`, `tlsSource` | pending | `-tls` | yes | silent |
| TLS pairings (`_TLS_HTTP` or `_ACME_DNS_HOOK` without acme, both together) | `tls.go:66/69/72` | pending | their flags | yes | silent |
| `MASSIMO_TLS_CERT`, `_TLS_KEY` (both, stat, load) | `tls.go:79`, `certs/files.go:45,64` | pending | their flags | yes, reads files | silent |
| `MASSIMO_TLS_NAMES` with acme | `certs/acme.go:71`, `dns01.go:89` | pending | `-tls-names` | yes, once `OpenDNS01`'s directory creation is split off | silent |
| `MASSIMO_ACME_DNS_HOOK` (missing, not executable; no execute bit on Windows) | `dns01.go:104-111` | pending | `-acme-dns-hook` | yes, stat | silent |
| `MASSIMO_TLS_HTTP`, its bind | `internal/serve/serve.go:389`, after "api listening" | pending | `-tls-http` | no, binds | silent |
| `MASSIMO_DATA` | `internal/serve/serve.go:478`, `store.Open` | pending | global `-data` (every service) | no | silent |
| `MASSIMO_LISTEN` | `internal/serve/serve.go:331`, `net.Listen` | pending | `-listen` (every service) | syntax yes, bind no | silent |
| `MASSIMO_CUBE_<SN>_IGNORE`, the plain `_IGNORE` once a Cube is named | `internal/serve/setup.go:44` → `FileCubes` → `env.ParseList` | pending | none | yes | silent |

**Not refusals:**
- A bad Cube `_ADDRESS` or `_PROXY` costs only that Cube, which warns and
  retries. `_KEY` and `_IV` are no longer read.
- `MASSIMO_API_TOKEN` takes any value.
- The rest is ignored or only warned of: `_NAME`, `_PROXY_SERIAL`,
  `MASSIMO_PUBLIC_URL`, the update settings, the ACME e-mail and directory
  outside acme, and a failed ACME order, which is logged.
- **Beside the point:** `MASSIMO_UPDATE_API` and `MASSIMO_GITHUB_TOKEN` apply
  at the next update check, yet a reload lists them as waiting for the next
  start.

## One source of checks

- **Today:** the start checks inline, in `serve.go:105-183`, `tls.go:63-106`
  and `setup.go:44`. The reload's `startChecks` holds four entries, each
  checking its variable alone, over the pending settings.
  The TLS refusals depend on several settings together: an unchanged
  `MASSIMO_TLS_HTTP=:80` becomes a refusal when `MASSIMO_TLS` changes from
  acme to off.
  The reloader keeps which settings a flag fixed, not the flags' values.
- **The smallest shape:**
  - a table `[]startCheck{vars, check(get)}` in the start's order;
  - `startRefusals(get, only)`;
  - the start runs it right after its flags are parsed, `get` being a flag's
    value where one is given and `os.Getenv` otherwise, and the first error
    returns as `serve: %w`;
  - a reload runs each check where one of its variables differs from the
    start and no flag fixed it, with a fixed flag's value where it reads one;
  - TLS: `serve.CheckTLS`, the pure half of `tlsSource`, with
    `certs.CheckACMENames` and `CheckDNS01Hook`, without creating directories
    or certificates; `tlsSource` calls it first;
  - Cubes: the entry calls `serve.FileCubes`;
  - the inner "serve: " prefixes move out (`servetls.go:47`, `restart.go:106`,
    `sighup.go:44`, `tls.go:66-106`).
  About 150 lines and tests.
  A start with two faults may name a different first one than today.
- **The guard:** a test classifies every variable `serve` reads, as refused
  (with a bad value, or a pairing) or as taking any value (with a reason).
  It runs the built binary per bad value and compares its "error:" line with
  the table's.
  It catches a variable nobody classified, a refusal without an entry,
  a text that drifted, and an entry the start does not refuse;
  not a pairing nobody listed.
  About 45 entries and 30 runs, about 3 s, skipped with `-short`;
  `builtBinary` exists.

## The reboot switch

`MASSIMO_REBOOT=bogus` at a reload switches the reboot off,
with a Warn on the reloader's log only;
`settings_read` stays at info, the sign stays dark, and the next start refuses
the value.
In the table, it is checked like the pending ones, and `settings_read` carries
"the next start refuses: MASSIMO_REBOOT …"; the reloader's own Warn goes,
one line as in LogEdges Block 2.
The token: the start refuses none, and a failure to apply it is the store's.

## The Cube settings

A bad `_IGNORE` stops the whole start, even for a Cube the store holds,
whose own list wins: a typo in a line the store overrides takes the server
down at the next start.
`FileCubes` only reads the environment, so the reload's check is pure.
Making it a warning at the start would change behaviour.

## What changes

- **Web UI:** the log's text is unchanged, since the error shows under the
  summary. The sign's settle rule exists. The Maintenance card lists names
  only, and gains `MASSIMO_RESTART` and `MASSIMO_SIGHUP` once they are known.
- **API:** `settings_read` already carries `error`.
  In docs/api-v1.md, the `settings_read` sentence and the reload paragraph
  grow.
  The OpenAPI file changes only if the restart answers carry the refusal
  (D8-B or C).
- **The risk the plan is for:** with a refused setting, a restart from the
  API, an update or a reboot stops a server that will not come back.
  systemd and launchd retry every 10 s, and there is no web UI until the file
  is fixed on the box.
- **Who sees the values:** the log needs `read`, while
  `GET /api/v1/config` needs `settings`.
  The warning therefore quotes no value of a `SecretVariable`
  (the `TLS_KEY` path).
- **Windows and macOS:**
  - `MASSIMO_SIGHUP` is refused on Windows too, and the table keeps that.
  - Windows checks no execute bit and has no SIGHUP.
  - Every packaged service passes `-env`, `-data`, `-listen`, `-log` and
    `-update-request`, and macOS also `-log-process-format none`.

## Where it meets other plans

- **LogEdges:** all three blocks are merged and its retirement is open.
  This plan is its NOTE's items 1 and 2.
  It shares `reload.go`, `reload_test.go`, `systemlog.go`, `logalerts.go`
  and the fragment `CHANGELOG.d/Fixed/claude-logedges-reload.md`,
  which is unreleased and would be amended.
- **FirstOwner:** its `DropKeepOpen` in `reload()` sits next to the switch.
  It is adjacent, with no conflict.
- **UiParts:** the Maintenance region is a part (`src/maintenance/`).
  Only Block 5 would touch it.
- **Light:** UpdateHistory (`systemlog.go`), RadioHealth (`logalerts.go`),
  Packaging (its units pass the flags).

## Open decisions

**D1. Which checks a reload runs.**
- **A (recommended):** the pure ones only. `LISTEN`, `LOG`, `DATA` and the
  `TLS_HTTP` bind stay out, and the docs say so: they fail on the box's
  state, not on the value's form, and packaged boxes fix them by flags.
- **B:** near-checks too (the listen syntax, the log's directory);
  costs platform code and checks that pass where the start still fails.

**D2. Which settings trigger a check.**
- **A (recommended):** a check runs where one of its settings differs from the
  start and is not fixed, with a fixed flag's value where one applies;
  pairings need the whole set.
- **B:** every check at every reload; it would also warn of a certificate
  file gone since the start, a warning that settles only at the next reload.

**D3. One source.**
- **A (recommended):** one table, which the start runs first and a reload
  runs too.
- **B:** the inline checks plus a table beside them, with a test comparing
  the texts; costs two places to edit per check.

**D4. The guard.**
- **A (recommended):** classify every variable, and run the built binary per
  bad value (about 3 s, skipped with `-short`).
- **B:** in process only: every flag's variable has an entry or a reason;
  it catches only a variable nobody classified.

**D5. Values in the warning.**
- **A (recommended):** the start's text, with a `SecretVariable`'s value
  replaced by its name.
- **B:** the start's text as is; shows a path the config hides.
- **C:** names only; loses "want debug, info, …".

**D6. The reboot switch.**
- **A (recommended):** its refusal is `settings_read`'s warning; the
  reloader's own Warn goes.
- **B:** both lines.

**D7. The token.**
- **A (recommended):** no change.
- **B:** a failed `SetToken` as `settings_read` at error; a new level and
  settle case.

**D8. A restart while a setting is refused.**
- **A (recommended):** the log and the sign only, as after Block 2:
  the sign is in the header on every page, and the line says
  "the next start refuses: …".
- **B:** the card also says what the next start refuses, without blocking:
  an API field, the Maintenance part, texts in six languages, a UI test.
- **C:** `GET /api/v1/restart` says `refused`, `POST` answers 409, and the
  card says why. This was LogEdges' D3-B, not taken then, and it guards the
  button only, not an update or a reboot.

**D9. A settings file that does not parse.**
- **A (recommended):** `settings_read` at warn with the parse error.
  The next start of every command refuses such a file, and the sign misses
  it today.
- **B:** out of scope.

**D10. `MASSIMO_RESTART` and `MASSIMO_SIGHUP` not known.**
- **A (recommended):** this plan's Block 1, with a test that every flag's
  variable is known.
- **B:** a plan of their own.

**D11. A bad `_IGNORE` of a Cube the store holds.**
- **A (recommended):** the start keeps refusing it; a reload warns.
- **B:** a warning at the start; a change of behaviour, for a plan of its own.
