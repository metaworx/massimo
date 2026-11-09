# AP ServiceUser v1.0: The service account _massimo

## TL;DR

On Linux the service runs as `_massimo`, as it does on macOS,
so it can never take over a person's account called massimo.
An existing box is renamed in place.
The "not started" message's owner line works as printed, from any directory,
and on macOS too.

- account: any `massimo` reused → `_massimo`;
  our own `massimo` (home `/var/lib/massimo`) → renamed in place,
  `usermod -l` and `groupmod -n`, the UID kept;
  a person's `massimo` → left alone, never deleted by a purge
- `install.User` `massimo` → `_massimo`, the same as `install.DarwinUser`
- owner line `sudo -u massimo massimo -data …` →
  `sudo -u _massimo /usr/bin/massimo -env /etc/massimo/env -data /var/lib/massimo user add …`
  (`/usr/local/bin` for `massimo install`), through `install.ServiceCommand`
- `/var/lib/massimo` after a start: 755 → 750 (`StateDirectoryMode=0750`)
- AP EnvFile (another session) does the service user's defaults
  and merges main after this plan; whichever merges second shortens the owner line

### Blocks

1. The account: `_massimo` on Linux, the rename in place,
   the purge that spares a person; the unit, `postinst`, `postrm`,
   `install`, the tests, the account name in the usage texts and the documents
2. The owner line: `install.ServiceCommand`,
   the absolute binary with `-env` and `-data`, in `postinst`, `install`,
   the READMEs and the site; the ServiceStart bullets amended
3. The state directory: `StateDirectoryMode=0750` in the unit

## Discussion

The findings (2026-11-09, the real-box checks of AP ServiceStart,
under systemd in a container on jaguar and on the owner's Mac):

1. The owner line the "not started" message prints,
   `sudo -u massimo massimo -data /var/lib/massimo user add <name> -role owner`
   (`_massimo` on macOS), fails:
   on macOS with `sudo: massimo: command not found`, since sudo's PATH there
   has no `/usr/local/bin`;
   everywhere with `open .env: permission denied` when it is run from a
   directory the service user cannot enter, such as a home directory
   (700 on Debian, `drwxr-x---` on the Mac),
   since every command reads `./.env` first.
   `docs/reverse-proxy.md` already wrote the safe form,
   `cd / && sudo -u massimo massimo -env /etc/massimo/env …`.
2. systemd sets `/var/lib/massimo` to 755 at every start:
   the unit has `StateDirectory=massimo` without a mode,
   so the 750 that `postinst` and `install` make lasts until the first start.
   The files in it are 600; their names show.
3. The owner, on the account's name: "massimo" may be a person's login,
   Massimo being a common name in Italy.
   Today `postinst` (`getent passwd massimo || adduser --system …`) and
   `install` (`id -u massimo`) reuse any account of that name:
   on such a box the service runs as that person,
   and the Cube keys in `/etc/massimo` are that person's files.
   `massimo install -remove -purge` then runs `userdel massimo`,
   which deletes the person's account.

**The owner's decisions** (`PLAN++ as recommended`, 2026-11-09):

- **D1-A:** the owner line names the binary by its absolute path and gives
  `-env /etc/massimo/env -data /var/lib/massimo`,
  so it works from any directory and with any sudo PATH,
  and reads the service's own settings.
  Not B, `cd / &&` before it: longer, and without the service's settings.
- **D2:** dropped. Skipping an unreadable default `./.env` for everyone
  contradicts what the owner settled for AP EnvFile:
  as the service user the settings come from `/etc/massimo/env` alone,
  for every other user an unreadable `./.env` stays an error.
  That is AP EnvFile's.
- **D3-A:** `StateDirectoryMode=0750`.
  Not B, leaving 755.
- **D4-A:** the Linux account is `_massimo`, as on macOS.
  adduser refuses a regular user whose name starts with `_`,
  so no person has it.
  Not B, keeping `massimo` and refusing a person's account:
  a box of a person called massimo could not install the package
  without a manual step.
- **D5-A:** one plan for the account, the owner line and the mode,
  which touch the same files; the line is written once, with the new name.
  Not B, two plans, which change the message and the READMEs twice.

**With AP EnvFile** (the session "Ignore an unreadable default .env",
agreed through the owner on 2026-11-09):

- EnvFile does the service user's defaults: the settings from
  `/etc/massimo/env` alone and the data directory `/var/lib/massimo`,
  `-no-env`, how the binary recognises the service user
  (it recommends the owner of `/etc/massimo/env`, its D4-B, the owner's call),
  and roadmap item 12.
- This plan does the account and its name everywhere:
  the documents, the certbot hook,
  and the four usage texts in `cmd/massimo` (`install.go`, `store.go`,
  `tls.go`, `user.go`), where only the name changes;
  EnvFile rewrites those texts on top, with `install.ServiceCommand`.
- This plan merges first, before the next release cut,
  and tells that session; EnvFile then merges main.
  Whichever merges second shortens the owner line,
  to `sudo -u _massimo <bin> user add <name> -role owner` if EnvFile lands
  as settled.

## Analysis

### The account

Debian's tools take the name, checked in trixie and bookworm containers
on 2026-11-09:
`adduser --system --group --home /var/lib/massimo --no-create-home
--shell /usr/sbin/nologin _massimo` works (adduser 3.152 and 3.134);
`adduser` refuses a regular user `_person`;
`deluser --system massimo` leaves a person's `massimo` in place
("is not a system user").
Debian itself has `_apt`.

**The rename in place.**
`usermod -l _massimo massimo` and `groupmod -n _massimo massimo`
keep the UID and the GID.
Files belong to the UID,
so `/etc/massimo`, `/var/lib/massimo` and `/var/log/massimo` follow.
In the trixie container, `usermod -l` renamed the account while a process
ran as it, and the process ran on under the new name:
no stop is needed.
The restart with the new unit (`User=_massimo`) comes after,
by ServiceStart's rule.

**Ours, or a person's.**
Both `postinst` and `install` make the account with the home
`/var/lib/massimo`.
A `massimo` with that home is ours, and is renamed.
Any other `massimo` is a person's:
it is left alone, and `_massimo` is made beside it.

| Before | `postinst`, `install` | purge |
|--------|-----------------------|-------|
| `_massimo` | used | removed |
| `massimo`, home `/var/lib/massimo` | renamed `_massimo` | (renamed already) |
| `massimo`, any other home | left; `_massimo` made | `_massimo` removed, the person kept |
| none | `_massimo` made | removed |

`install -remove -purge` removes `_massimo`,
and a `massimo` whose home is `/var/lib/massimo`:
a hand install whose binary was updated by `massimo update` keeps the old
account until `install` runs again.
`postrm` purge does the same.

**Not handled:** a box where an older package or install took over a
person's `massimo`.
Its files belong to that person, and the new `_massimo` cannot read them.
The package has been public since v0.12.0-alpha.1,
with one known user, the owner.
The `CHANGELOG.md` bullet names the case.

**Where the name lives:**

| Place | Change |
|-------|--------|
| `internal/install/install.go` | `User = "_massimo"`; `user()` reads `getent passwd` through `Output`, renames ours, makes `_massimo`; the purge |
| `internal/install/darwin.go` | `DarwinUser` stays, the same name; `darwinEnvTemplate` no longer rewrites the user |
| `internal/install/windows_args.go` | the template's "Readable only by the user …" line, with the new name |
| `packaging/systemd/massimo.service` | `User=_massimo`, `Group=_massimo` |
| `packaging/deb/postinst`, `postrm` | the account and the rename; the purge |
| `packaging/env.template` | "Readable only by the user _massimo." (a comment: ServiceStart's rule compares setting lines only) |
| `packaging/deb/test.sh`, `internal/install` tests | the new name; the rename and a person's account |
| `cmd/massimo` usage: `install.go`, `store.go`, `tls.go`, `user.go` | the name only |
| `README.md`, `packaging/README.md` (certbot hook and DNS hooks included), `docs/store.md`, `docs/reverse-proxy.md`, `docs/update-windows.md`, `packaging/site/index.html` | the name |

`docs/handover-*` and `docs/review-*` record what was; they stay.
Windows runs the service as LocalSystem, and Homebrew as the user who
installed it: neither has an account to rename.

`user()` reads `getent passwd` through `in.Output`,
read-only and so also in a dry run, as `active()` reads `systemctl`.
Tests answer it.
With a root and no `Output` it reads nothing, so a test that gives neither
gets `useradd`, as today.

### The owner line

`install.ServiceCommand(bin string, args ...string) string`
is the line shown to people that runs `bin` as the service user:
`sudo -u _massimo <bin> <args>` on Linux and macOS,
and on Windows, where an administrator runs it, `& "<bin>" <args>`.
`bin` is quoted for the shell when it holds a space or a quote.
AP EnvFile's usage texts pass `os.Executable()`;
`install`'s messages pass the binary they installed:

| Path | Binary |
|------|--------|
| `postinst` (shell, a literal) | `/usr/bin/massimo` |
| `Install` (Linux), `InstallDarwin` | `BinPath`, `/usr/local/bin/massimo` |
| `configure` (a packaged box) | `/usr/bin/massimo` |
| `InstallWindows` | `p.Bin`, as today |

The line: `<ServiceCommand(bin)> -env /etc/massimo/env -data /var/lib/massimo user add <name> -role owner`,
on Windows `-data "<DataDir>" user add <name> -role owner` as today.
`/etc/massimo/env` is 600 and the service user's on every path,
so the service user reads it.
`-env` names the file, so `./.env` is not opened,
wherever the line is run from.

The ServiceStart bullets in `[Unreleased]` brought the line in;
a defect brought in and fixed within one release amends them (COMMIT.md 5.2).

### The state directory

`StateDirectoryMode=0750` beside `StateDirectory=massimo`.
systemd applies it at every start, also to a directory that exists.
The unit is the one the deb derives (`packaging/deb/unit.sh`) and the one
`install` writes.
Nothing but the service and root reads `/var/lib/massimo`:
the update and reboot helpers run as root.
`LogsDirectory=` stays 755, as `/var/log` directories are.

## Implementation Plan

### Block 1: The account

1. `internal/install/install.go`:
   `User = "_massimo"`, `formerUser = "massimo"`;
   `user()`: `_massimo` exists → used;
   `massimo` with the home `StateDir` → `usermod -l`, then `groupmod -n`
   when the group `massimo` exists and `_massimo` does not;
   else `useradd` as today;
   the purge removes `_massimo`, and `massimo` when its home is `StateDir`.
2. `internal/install/darwin.go`: `DarwinUser = User`, documented as the
   same name; `darwinEnvTemplate` without the user rewrite.
   `windows_args.go`: the template line with `User`.
3. `packaging/systemd/massimo.service`: `User=_massimo`, `Group=_massimo`.
   `packaging/env.template`: the comment.
4. `packaging/deb/postinst`: the account as in the table;
   `/etc/massimo`, `env` and `/var/lib/massimo` made for `_massimo`.
   `packaging/deb/postrm`: purge removes `_massimo`,
   and `massimo` when its home is `/var/lib/massimo`.
5. Tests:
   `internal/install`: the three cases of `user()`, the purge sparing a
   person, the chowns with the new name;
   `packaging/deb/test.sh`: the new name; an old system `massimo` with a
   file renamed in place, the UID and the file's owner kept;
   a person's `massimo` left with its UID and home, and kept by the purge.
6. The usage texts in `cmd/massimo` and the documents of the table: the name.
7. `CHANGELOG.md`, `### Security`: the account, the rename,
   the person kept, the case not handled.

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./internal/install/... ./cmd/...`;
`make packages` and `make deb-test` on trixie and bookworm;
the tests fail on the old code (the purge deleting a person's account,
the reuse);
`git grep` finds `massimo` as an account name only where intended.

### Block 2: The owner line

1. `internal/install`: `ServiceCommand` and its shell quoting, with tests
   for Linux, macOS and Windows.
2. The messages: `Install` and `configure` on Linux,
   `InstallDarwin`, `InstallWindows`, through `ServiceCommand`;
   `postinst`'s literal.
3. `README.md`, `packaging/README.md` (Linux, the package, macOS),
   `docs/reverse-proxy.md`, `packaging/site/index.html`: the line.
4. Tests: the message in `internal/install` (Linux, packaged, macOS);
   `test.sh` greps the line.
5. `CHANGELOG.md`: ServiceStart's two bullets amended.

**Verification:** the test gate;
in the systemd container, the line copied from the message and run from
a 700 home directory creates the owner;
`sync-docs.sh --check`; the site page checked with `tidy` if it is there.

### Block 3: The state directory

1. `packaging/systemd/massimo.service`: `StateDirectoryMode=0750`.
2. `test.sh`: the deb's unit carries it;
   an `internal/install` test: the unit `install` writes carries it.
3. `packaging/README.md`: the directory is 750; `CHANGELOG.md`, `### Security`.

**Verification:** in the systemd container,
`/var/lib/massimo` is 750 after a start and after a restart.

### The whole

The test gate of AGENTS.md (`go test`, `gofmt`, `go vet` for linux, windows
and darwin, `make cross`), `make deb-test` on trixie and bookworm,
`make apt-test`,
and under systemd in a Debian container on jaguar,
with systemd as its first process:
a fresh install; the upgrade from the v0.12.0-alpha.1 package with the
service running, renamed in place, restarted as `_massimo`, its store
readable; a box with a person's `massimo`; the purge.
The owner's Mac and Pis are not touched;
the Mac can repeat its test with the owner at the sudo prompt.

## Proposed commit messages

Block 1:

```
fix(security:packaging): The service runs as _massimo, and never as a person's account

On Linux the service account is _massimo, as on macOS, so that the Debian package and massimo install no longer take over a person's account called massimo, and a purge no longer deletes it; a box set up before is renamed in place (AP ServiceUser v1.0, Block 1).
…
```

Block 2:

```
fix(install): The owner line works from any directory, and on macOS
…
```

Block 3:

```
fix(security:packaging): /var/lib/massimo stays 750 at every start
…
```

## Change History

- v1.0 (2026-11-09): the first revision, by `PLAN++ as recommended`:
  D1-A, D3-A, D4-A, D5-A; D2 dropped for AP EnvFile.
