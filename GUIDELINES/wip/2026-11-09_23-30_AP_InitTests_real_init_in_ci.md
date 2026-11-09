# AP InitTests v1.0: Real init systems in CI

## TL;DR

The two checks that ServiceStart and ServiceUser ran by hand,
under systemd in a container and with launchd on the owner's Mac,
become steps of the CI jobs that already build and install the packages,
so every pull request runs them.

- `deb` job: `test.sh`, a fake systemctl → also `systemd-test.sh`,
  systemd as the container's first process, on trixie and bookworm
- the upgrade from v0.12.0-alpha.1, the last release before `_massimo`,
  with the service running → renamed in place, restarted, signed in
- `macos` job: filled-in settings only → the template's branch first:
  not loaded, the message's owner line run as printed, restart and
  stay-stopped, the web UI's way, the purge
- `CI.md` and the `deb` job's comment:
  "the start on a real box is the owner's check" → covered

### Blocks

1. `packaging/deb/systemd-test.sh` and `make deb-systemd-test`:
   the image, five cases, each in a container of its own
2. The `deb` job: the step; its comment; `GUIDELINES/project/CI.md`
3. The `macos` job: `packaging/launchd/template-test.sh`, a step before the
   job's install; `CI.md`

## Discussion

Asked by the owner on 2026-11-09, after AP ServiceUser retired:
which of the session's manual tests are worth the suite.
Two caught what nothing else could:

- under systemd, `usermod` refused to rename the account of a running
  service, and the upgrade left the package half-configured;
  `test.sh`'s stand-in systemctl cannot see that;
- on the Mac, the owner line failed with `sudo: massimo: command not found`;
  CI's macOS job installs with filled-in settings only,
  so the template's branch, where the line is printed, never runs.

**The owner's decisions** (`PLAN++`, the gate's recommendations, 2026-11-09):

- **D1-A:** both, systemd and launchd.
  Not B, systemd only, which leaves the Mac's branch manual.
- **D2-A:** the upgrade starts from v0.12.0-alpha.1,
  the last release with the account `massimo`,
  so that the rename keeps its test after the next release.
  Not B, the newest release, which stops testing the rename then.
- **D3-A:** the systemd cases are a step of the `deb` job, on both of its
  images, with the packages it built.
  Not B, a job of its own, which would need them as an artifact.

Already covered, and not repeated:
the `.env` of a directory the service user cannot enter
(`cmd/massimo`, `TestServiceUserUnseenFile`);
the account's cases and the owner line's form (`internal/install`);
Windows (its job).
The Pis' upgrade is the session "Run Hardware/Owner tests"'s.

## Analysis

### systemd in a container

Measured on jaguar on 2026-11-09 (Docker 29, cgroup v2):
an image from `$DEB_IMAGE` with `systemd systemd-sysv dbus adduser
init-system-helpers logrotate curl procps sudo ca-certificates`,
started with `--privileged --cgroupns=private --tmpfs /run --tmpfs /run/lock`
and `/sbin/init`, reaches `degraded`
(getty and modules-load fail in a container, which is harmless).

- Debian's images ship `/usr/sbin/policy-rc.d`, which exits 101:
  `deb-systemd-invoke` then refuses every start and stop.
  The image removes it, as a real box has none.
- `--network none`: the dependencies are in the image,
  so apt installs the local packages offline,
  and a running `serve` reaches neither the LAN nor GitHub.
- apt's output in a file ends its lines with a carriage return;
  the cases strip it before comparing a line or running it.
- Each case starts a fresh container:
  an account left by one case changes the next.

GitHub's `ubuntu-latest` runners run Docker with cgroup v2 as jaguar does,
but have not run this yet; the first pull request shows it.

### The packages

The `deb` job's `make packages` builds the version `git describe` gives,
`0.12.0~alpha.1~<n>~g<sha>`, which dpkg sorts before `0.12.0~alpha.1`.
The upgrade case installs it with `--allow-downgrades`:
dpkg runs the new `postinst` with `configure <old version>` either way,
and the old `prerm` and `postrm` with `upgrade`,
so the path is an upgrade's.
After the next release the version sorts after it by itself.

The v0.12.0-alpha.1 package comes from its release:
`gh release download` with the job's token, where `gh` is there,
else `curl` from the release's download address; `OLD_DEB` names a file
instead, for a run without the network.
Its binary, taken out with `dpkg-deb -x`, is the hand install's.

### The cases

| Case | What it proves |
|------|----------------|
| `fresh` | `_massimo`, no `massimo`; not started, the message, its owner line as `postinst` prints it; that line, run from a 0700 home directory, creates the owner; started as `_massimo`, the API closed; `/var/lib/massimo` 0750 after a start and a restart; reinstalled running → restarted, no message; stopped → left, the message; the purge takes the account and the files |
| `upgrade` | v0.12.0-alpha.1 running with an owner made through the API, then the new package: `_massimo` with the old ID, no `massimo` and no group of it, restarted on the new binary as `_massimo`, the store and the settings `_massimo`'s, the owner signs in, 0750 |
| `person` | a person called massimo: untouched; the service runs as `_massimo`; the purge keeps the person |
| `takenover` | v0.12.0-alpha.1 on a box with a person called massimo, which took the person over, then the new package: the `chown` named; after it as printed and a restart, the service runs as `_massimo`, the person untouched |
| `install` | `massimo install` of the new binary over v0.12.0-alpha.1's hand install, running with an owner: stopped, renamed with its ID, restarted, the owner signs in; `-remove -purge` takes `_massimo`; beside a person called massimo, the person untouched and kept by the purge |

The driver, `systemd-test.sh <image>`, builds the image, fetches the old
package, and runs `systemd-cases.sh <case>` in a container per case,
mounting `dist/` read-only; it fails when a case does.
About 30 s a case on jaguar.

### launchd on the runner

GitHub's macOS runners give `sudo` without a password,
and the job already runs the LaunchDaemons there (`CI.md` section 7).
`template-test.sh <binary>` is the owner's Mac test without its sudo prompt:
`install` on the template (not loaded, the update helper loaded,
the message, `/var/lib/massimo` 0750, nothing on port 8099),
the owner line taken from the message and run as printed from a 0750
directory the service user cannot enter,
`launchctl bootstrap` as the message says and the API closed,
`install` again restarting the loaded job, a booted-out one left stopped,
an older template's comments still the template, a setting of its own
starting it, the web UI's way (open to the box, the first owner from the
form's route, then closed, `install` again restarting it),
and `-remove -purge` leaving no job, binary, directory, log, user or
firewall entry.
It runs before the job's own install, which needs the Mac clean,
and checks that it is.

## Implementation Plan

### Block 1: systemd in a container

1. `packaging/deb/systemd-test.sh`: the driver, as the Analysis says;
   `packaging/deb/systemd-cases.sh`: the five cases, `ok` and `FAIL` lines,
   a non-zero exit on a failure.
2. `Makefile`: `deb-systemd-test`, with `DEB_IMAGE`.
3. `packaging/README.md`, "Debian package": the tests, `make deb-systemd-test`.
4. `CHANGELOG.md`, `### Added`: a bullet.

**Verification:** `make packages`, then `make deb-systemd-test` on trixie
and bookworm on jaguar, every case passing;
each case fails with the fault it is there for put back
(the rename without the stop, a reused person's account,
the owner line without its full path).

### Block 2: the deb job

1. `.github/workflows/ci.yml`, `deb`: a step after `make deb-test`,
   `make deb-systemd-test DEB_IMAGE=${{ matrix.image }}`,
   with `GH_TOKEN` for the old package; the job's comment.
2. `GUIDELINES/project/CI.md`: section 2's row, section 7's line on the
   container without systemd; its version.
3. `CHANGELOG.md`: the bullet of Block 1 names CI.

**Verification:** the workflow parses (`yq` or Python's YAML);
the step runs on the pull request's first CI run, which the next signal
starts.

### Block 3: the macos job

1. `packaging/launchd/template-test.sh`, as the Analysis says.
2. `.github/workflows/ci.yml`, `macos`: a step before
   "Install, run, update and remove the LaunchDaemons" that builds the
   binary and runs it; its comment.
3. `GUIDELINES/project/CI.md`: section 2's row.
4. `CHANGELOG.md`: the bullet.

**Verification:** `bash -n` and `shellcheck`;
the step runs on the pull request's first CI run;
the owner's Mac can run the script by hand with the owner at the sudo
prompt.

## Proposed commit messages

Block 1:

```
test(packaging): The package under systemd, its upgrade from v0.12.0-alpha.1 included
…
```

Block 2:

```
ci(deb): The Debian package job runs the systemd cases
…
```

Block 3:

```
ci(macos): The macOS job installs on the template's settings first
…
```

## Change History

- v1.0 (2026-11-09): the first revision, by `PLAN++`: D1-A, D2-A, D3-A.
