# NOTE ServiceUser v1.0: how the run went

Blocks 1 to 3 ran on the branch `claude/service-user`
under `PLAN++ as recommended` (2026-11-09),
with one commit more than the plan has blocks.

## Where it went differently

1. **The rename needs the service stopped.**
   The plan's Analysis says `usermod -l` renamed the account while a process
   ran as it, so no stop is needed.
   That was measured with a process started outside systemd.
   Under systemd, the upgrade from the v0.12.0-alpha.1 package with the
   service running failed in `postinst`:
   `usermod: user massimo is currently used by process 488`,
   exit 8, the package left half-configured.
   The commit after Block 3,
   `fix(packaging): A running service is stopped for the account's rename`,
   stops a running service before the rename,
   in `postinst` with `deb-systemd-invoke` and in `install` with `systemctl`,
   and the start rule starts it again, since it ran.
   A rename that fails all the same ends `postinst` with a message
   naming `dpkg --configure -a`.
2. **The case not handled is named in Block 1's commit message,
   not in its `CHANGELOG.md` bullet,** which would have needed a fourth line.
   A box where an older release took over a person's `massimo`:
   under systemd the service then fails at its start with
   `open /etc/massimo/env: permission denied`,
   because the files stay that person's,
   until the owner hands `/etc/massimo`, `/var/lib/massimo` and
   `/var/log/massimo` to `_massimo`.
3. **The macOS template was picked by the account's name**
   (`in.svcUser == DarwinUser`), which the shared name made true on Linux too;
   a field `darwin` picks it now (Block 1).
4. `docs/reverse-proxy.md` drops its `cd / &&`: `-env` covers it (Block 2).

## Measured

- Under systemd in a Debian trixie container on jaguar, without network,
  34 checks pass:
  - a fresh install: `_massimo`, no `massimo`, the message's line as the
    plan gives it, not started;
  - that line, copied from the message and run from a 700 home directory,
    creates the owner; started as `_massimo`, the API closed;
    `/var/lib/massimo` 750 after a start and after a restart;
  - the upgrade from the v0.12.0-alpha.1 package, the service running with
    an owner made in the web UI's way: `_massimo` with the old ID,
    no `massimo` and no group of it left,
    restarted on the new binary as `_massimo`, the store and the settings
    `_massimo`'s, the owner signs in;
  - a person called massimo: untouched, the service as `_massimo`,
    kept by the purge;
  - `massimo install` over v0.12.0-alpha.1's hand install, running with its
    owner: stopped, renamed with the ID kept, restarted, the owner signs in;
    beside a person called massimo: untouched, kept by `-remove -purge`.
- `make deb-test` on trixie and bookworm, `make apt-test`, the test gate,
  and the `internal/install` and `cmd/massimo` tests on Windows through
  interop, its `ProgramData` a temp directory
  (the live service's `service.log` unchanged).
- Mutations: an `ownAccount` that ignores the home fails the Go tests'
  person cases; a `postinst` that renames any `massimo` fails `test.sh`;
  an `install` that forgets it stopped the service leaves it stopped,
  and `TestServiceAccount` fails.

Not run: the owner's Mac, which needs the owner at the sudo prompt,
and the Raspberry Pis, which get the change with the next release.
