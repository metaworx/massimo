# AP EnvFile v1.0: The service user's defaults

## TL;DR

Run as the service user and without `-env`,
every command reads the service's settings and store,
from whatever directory it is started in.
`-no-env` reads no settings file.

- service user = the owner of `/etc/massimo/env`; never root, never on Windows
- its settings file `./.env` → `/etc/massimo/env` alone;
  its data directory `.` → `/var/lib/massimo`
- `-env`, `MASSIMO_ENV_FILE`, `-no-env` → both defaults off;
  `-data`, `MASSIMO_DATA` → still win for the data
- TLS, the Cubes, the proxy, the log level → follow from those two
- root and every other user → `./.env` and `.` as today;
  an unreadable `./.env` stays an error
- `-no-env` → `MASSIMO_ENV_FILE` = `os.DevNull`; excludes `-env`
- usage examples → `install.ServiceCommand(<this binary>, …)`,
  without `-env` and `-data` on Linux and macOS
- `massimo config` → says why it chose the file and the directory

### Blocks

1. The service user's defaults: the rule, `main`, `config`,
   the usage texts and the documents; tests; CHANGELOG
2. `-no-env`: the option, the usage text, the README; tests; CHANGELOG

## Discussion

**The finding** (the owner, 2026-11-09):
`sudo -u massimo massimo -data /var/lib/massimo store backup <file>`,
the form `massimo store`'s usage text recommends,
run from the owner's home directory on a Raspberry Pi
(Debian 13, home directories 700), failed on both boxes with
`error: open .env: permission denied`.
sudo keeps the working directory, the service user cannot enter it,
and `env.LoadFile` lets only a missing file through.
`-env /dev/null` worked around it.
The usage texts of `user` and `tls`, the README lines and the
"not started" message's owner line (AP ServiceStart) give the same form;
the ServiceStart session saw it fail on systemd and on the owner's Mac.

Even from a directory it can enter, the service user did not get the
service's settings:
`tls show` and `tls self-signed` read `MASSIMO_TLS` and `MASSIMO_TLS_NAMES`,
which are in `/etc/massimo/env`, so they reported the wrong mode.

**The owner's decisions** (three EVALs, then `PLAN++ as recommended`,
2026-11-09):

- The service user reads `/etc/massimo/env` alone, never `./.env`,
  unless `-env` names a file:
  "running `sudo sshd` also runs the regular config files".
  This replaces the first proposal, `./.env` else the installed file.
- The other settings follow the same rule: the data directory too;
  TLS follows from the settings file and the data directory.
- **D1:** dropped (skipping an unreadable default `./.env`).
  For every user but the service user an unreadable `./.env` stays an error:
  "permission denied" says more than a later "no key".
- **D2-A:** root keeps `./.env` and `.`.
  `store`, `user` and `tls` run as root would leave root-owned files in
  `/var/lib/massimo`, and `update` would take the service's
  `MASSIMO_UPDATE_CHANNEL` unasked.
  Not B, root reading the service's settings as `sudo sshd` does.
- **D3-A:** `-no-env` is added:
  `os.DevNull` works on every platform (`NUL` on Windows)
  and says what it means.
  Not B, `-env /dev/null` alone.
- **D4-B:** the service user is the owner of `/etc/massimo/env`, root excluded.
  No list of names: it holds through AP ServiceUser's rename
  (`usermod -l`, the UID kept),
  on a box updated only by `massimo update` that still has the old account,
  and it never picks out a person called massimo on a box without the service.
  Not A, by name: `massimo` and `_massimo` both for a while,
  and `massimo` may be a person.
- **D5-A:** `-env` (and `MASSIMO_ENV_FILE`, `-no-env`) turns off both defaults:
  another settings file never reaches the live store by accident.
  Not B, `-env` turning off the settings file alone.

**With AP ServiceUser** (the session "Revisit the start rule for an
untouched env file", agreed between the sessions on 2026-11-09):

- ServiceUser does the account `_massimo` and its name everywhere,
  the documents and the four usage texts in `cmd/massimo` included,
  `install.ServiceCommand(bin string, args ...string) string` in
  `internal/install`, and the owner line.
  It merges first, before the next release cut,
  and tells this session.
- EnvFile does the defaults, `-no-env`, D4,
  the usage texts rewritten on top of ServiceUser's with
  `install.ServiceCommand` and this binary's path,
  and the documents' example lines shortened.
- Whichever merges second shortens the owner line:
  this plan, as settled, to `<ServiceCommand(bin)> user add <name> -role owner`.
- Roadmap item 12's entry ("`store info` run from a directory whose `.env`
  the service user cannot read") is pruned at this plan's retirement,
  as the contract (v1.4.0) has it.

## Analysis

### What the service passes, and what follows

The units (`packaging/systemd/massimo.service`,
`packaging/launchd/ch.metaworx.massimo.plist`) start
`massimo -env /etc/massimo/env -data /var/lib/massimo serve -listen …
-log … -update-request …`.
Every global option and command flag was compared with that
(2026-11-09):

| Setting | Default today | The service | As the service user |
|---|---|---|---|
| settings file | `./.env` | `/etc/massimo/env` | **its own default** |
| data directory | `.` | `/var/lib/massimo` | **its own default** |
| `massimo.db`, `node.key`, `opaque.key`, `tls/`, the ACME cache | the data directory | | follow |
| `-tls-cert`, `-tls-key` | `MASSIMO_TLS_CERT`, `_KEY` | absolute paths `install` writes into the settings | follow |
| `MASSIMO_TLS`, `_TLS_NAMES`, ACME, HSTS, the Cubes, `_PROXY`, `MASSIMO_CUBE_SN`, log level, update channel, token | the settings file | | follow |
| `-listen` (`MASSIMO_LISTEN`) | unset | the unit's `listen.conf`, the plist | not reachable |
| `-log`, `-update-request` | the settings file, none | the unit passes them | `serve` only |
| `-sn`, `-show-hidden`, `-direct`, output files | per run | | unchanged |

Three notes from it:

- `tls self-signed` puts a specific listen address into the certificate's
  names; as the service user `MASSIMO_LISTEN` is not set,
  since it lives in the unit, not in the settings file.
  Its usage text says to give the address with `-names`.
  Moving `MASSIMO_LISTEN` into the settings file is a change of its own
  (`install`, the update helper's health drop-in), not this plan's.
- Output files stay relative to the current directory
  (`store backup <file>`, `backup -out`, `portal -dir`, `legacy -out`):
  as the service user they need a path it may write.
  `store backup`'s example gives one; `store` creates the copy 600.
- A `serve` run by hand as the service user is a debugging run:
  it gets no defaults for `-log` and `-update-request`,
  and the template's `MASSIMO_LOG` covers `-log` anyway.

### Recognising the service user

On Linux and macOS:
`os.Stat(install.EnvPath)` succeeds,
`os.Geteuid()` is not 0,
and the file's owner (`Sys().(*syscall.Stat_t).Uid`) is the effective UID.
On Windows never: its service runs as LocalSystem with `-env` and `-data`,
and a service started without `-data` already uses
`%ProgramData%\Massimo` (`serviceDataDir`).
Homebrew's settings are `$(brew --prefix)/etc/massimo/env`,
and its service runs as the person who installed it: not affected.

A box where an older install made a person's `massimo` the service account
(AP ServiceUser's case not handled) makes that person the service user:
their runs read `/etc/massimo/env`, and `-env .env` reads their own.

A missing `/etc/massimo/env` means no service user.
An unreadable one, the service user's own,
fails as any unreadable settings file does (`LoadFile`):
the service would fail on it too.

### Where it lives

- `internal/env`: `File()` returns `MASSIMO_ENV_FILE`, else the default file,
  `.env` until `SetDefaultFile(path)` sets another.
  Setting `MASSIMO_ENV_FILE` instead would show it in `config`'s variables
  as coming from the environment, and hand it to child processes.
- `cmd/massimo`: `serviceDefaults`, decided in `main` after
  `globalOptions` and before `LoadFile`:
  `MASSIMO_ENV_FILE` unset and the service user →
  `env.SetDefaultFile(install.EnvPath)`,
  and `dataDir()` falls back to `install.StateDir`
  after `-data` and `MASSIMO_DATA` (which the settings file may set)
  and before `.`.
  The two paths and the effective UID are package variables,
  so that a test uses a file of its own in a temporary directory.
- `serve` gets `env.File()` for its settings file, the reloader and the
  watcher, so it follows without a change.

### What `config` says

- `env file:      /etc/massimo/env (loaded; the service user's default, -env for another)`
- `data dir:      /var/lib/massimo (…; the service user's default)`
- with `-no-env`: `env file:      none (-no-env)`

The API's config view (`GET /api/v1/config`) keeps its shape:
`file` is the file chosen.

### The usage texts

`serviceExample(args ...string) string` in `cmd/massimo` is the example line:

- Linux and macOS: `install.ServiceCommand(exe, args...)`,
  such as `sudo -u _massimo /usr/bin/massimo store info`;
- Windows, with no service user to run as, in an elevated shell:
  `install.ServiceCommand(exe, "-env", <Env>, "-data", <DataDir>, args...)`
  with `install.WindowsPaths`;
- `exe` is `os.Executable()`, `massimo` where that fails.

`store`, `user` and `tls` use it;
`main`'s usage says the rule in a paragraph.

### The documents

`README.md` ("Cube variables", the `MASSIMO_DATA` row, the paragraphs on
`store` and `user`), `packaging/README.md` (the store, `tls`,
the lines that check the settings before the first start),
`docs/store.md` (the data directories' table, the settings file's row),
`docs/reverse-proxy.md` (the owner line):
the rule where the defaults are described,
the example lines without `-env` and `-data`
where ServiceUser wrote them with both.
`docs/handover-*` and `docs/review-*` record what was; they stay.

## Implementation Plan

The work branch starts at main after this plan's register;
Block 1 starts once AP ServiceUser is on main.

### Block 1: The service user's defaults

1. Merge `origin/main` with ServiceUser into the branch
   (`-c submodule.recurse=false`).
2. `internal/env/dotenv.go`: the default file and `SetDefaultFile`.
3. `cmd/massimo`: `serviceUser()` in a Unix and a Windows file;
   the decision in `main`, in a function the tests call;
   `dataDir()`'s fallback; `config`'s two lines.
4. The usage texts: `serviceExample`; `store`, `user`, `tls`, `main`.
5. Tests:
   - the rule: the owner of the file without `-env` gets both defaults;
     `-env`, `MASSIMO_ENV_FILE`, UID 0, another owner and no file get none;
   - `dataDir()`: `-data`, then `MASSIMO_DATA` (also from the file),
     then the default, then `.`;
   - `serviceExample` on the platform the test runs on;
   - `config`'s lines with the reason.
6. The documents of the Analysis.
7. `CHANGELOG.md`, `### Fixed`.

**Verification:** the test gate of AGENTS.md
(`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .`,
`go vet` for linux, windows and darwin, `make cross`);
the rule's tests fail on the old code;
in a Debian trixie container on jaguar, with `_massimo` owning
`/etc/massimo/env` and `/var/lib/massimo`:
`store info`, `store backup`, `tls show` and `config` run as `_massimo`
from a 700 directory read the service's settings and store;
root and another user still read `./.env`; `-env` turns both off.

### Block 2: `-no-env`

1. `cmd/massimo/options.go`: `-no-env` sets `MASSIMO_ENV_FILE` to
   `os.DevNull`; with `-env`, in either order, an error.
2. `main`'s usage: the synopsis and its sentence; `README.md`.
3. `config`: `none (-no-env)`.
4. Tests in `options_test.go`: `-no-env`, and the two together.
5. `CHANGELOG.md`, `### Added`.

**Verification:** the test gate;
`massimo -no-env config` says `none (-no-env)`;
the options test cross-compiled for Windows and run through interop,
where `os.DevNull` is `NUL`.

### The whole

Push the branch and open the pull request;
the owner merges.

## Proposed commit messages

Block 1:

```
fix(cli): As the service user, the commands read the service's settings and store from any directory

Run as the owner of /etc/massimo/env, without -env, massimo reads its settings from that file alone and keeps its data in /var/lib/massimo, so store, user and tls work from a directory the service user cannot enter and see the service's own settings; before, they read ./.env and failed with open .env: permission denied in a home directory; AP EnvFile v1.0, Block 1.

Key changes:
- …

Assisted-by: claude-code:claude-opus-5-5
```

Block 2:

```
feat(cli): -no-env reads no settings file

…; AP EnvFile v1.0, Block 2.

Key changes:
- …

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-09): the first revision, by `PLAN++ as recommended`:
  D2-A, D3-A, D4-B, D5-A; D1 dropped.
