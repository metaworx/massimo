# AP AptUpdate v1.1: Update button through apt and brew

## TL;DR

The web UI's update button works where apt or brew owns massimo:
a root helper of the `.deb` runs apt for the massimo source only,
and on a Homebrew box the server starts a helper that runs brew.
v1.1 adds Block 5: a package built between two releases sorts after the one it follows.

- Blocks 1 to 4 merged (#127 within #128); a real apt box passed
- marker `apt` + helper enabled → the button; "managed by apt" → "installed by apt"
- `massimo-update.path` + the deb's own `.service` → `massimo update -request …`
- `apt-get update` of `massimo.sources` only, retried on its lock;
  `apt-get install --only-upgrade massimo`, lock timeout 120 s
- `/healthz` must report the new version; no downgrade, the way back named
- brew: `update`, `upgrade`, `services restart` from a detached helper; never as root
- `DEBVER` between tags: `0.12.0~alpha.1~117~g3d7588f` → `0.12.0~alpha.1+117.g3d7588f`
- Block 5: its own pull request, from `main`

### Blocks

1. AptHelper: `massimo update`'s helper mode runs apt on an apt box,
   the lock retries, the states, the health check, the tests
2. AptPackage: the deb's helper units, enabled; `unit.sh` keeps the request flag;
   the maintainer scripts; `massimo install` on a packaged box;
   a real upgrade in the apt test; `packaging/README.md`
3. AptButton: the server offers the update on an apt box with the helper on,
   the suite as the channel, `manager` in the API, the UI's text in six languages,
   the documents
4. BrewUpdate: the server starts a brew helper on a Homebrew box,
   the helper runs brew, the label check fixed, the macOS CI upgrade,
   the documents; its own pull request
5. DebVersion: a build between tags gets its `git describe` part behind a `+`,
   `debTag` reads it back, the systemd cases upgrade without `--allow-downgrades`,
   `packaging/README.md`; its own pull request

## Discussion

The owner wants this:
"Installs on hosts with apt packages and apt sources should still be available
from the web interface."
Before this plan, a package install made the server refuse
(`UpdateOptions.Managed`, the first reason `updateBlocked` gave),
and the UI showed the newer version and the apt command.

**What this plan takes over**, from the Debian package's work so far:
the web UI's update button running apt or brew through a helper
where the package marker names a manager, both halves.
The rule that a packaged box's server and `massimo update` install nothing
ends here for the button;
`massimo update` run by hand keeps refusing and names the manager's command.

**The owner's decisions:**

- **Route 1** (2026-11-09): a plan of its own, not a block of another plan.
- **D1-A** (2026-11-09):
  the helper installed and enabled on a package install,
  as `massimo install` enables the tarball's.
  It does only what `apt upgrade` would do.
- **D2-B** (2026-11-09): the Homebrew half as Block 4 now;
  a real Mac is available later that day.
- **D3-A** (2026-11-09): after a failed health check the helper reports and does
  not downgrade; the status names the way back.
- **`PLAN++ D2-B`** (2026-11-09), with D1-A and D3-A as recommended,
  and then: "make Block 4 it's own pr so that 1-3 can already land".
- **D10-A** (2026-11-10, "Yes, run them"):
  this session ran the real apt box's test itself, over SSH.
- **D9-A** (2026-11-10, `PLAN+ F1 fix`):
  finding F1, a build between tags sorting before its tag, is fixed in this plan.

**Where it stands** (2026-11-10):
Blocks 1 to 4 are on `main`: #128, which carried #127, merged on 2026-11-09.
A Raspberry Pi 5 without a Cube ran the helper end to end
through a local repository
([the NOTE](2026-11-10_00-53_NOTE_AptUpdate_v1.0_real_box_run.md)).
It took two test builds, `v0.12.0-alpha.1.1` and `.2`,
because a build from `main` sorts below the release (F1).
Still open beside Block 5:
the button against the signed repository, once a release is out;
the real Mac (D11, the owner's: hand it to the session "Run Hardware/Owner tests",
or install Homebrew there for it).
Retiring the plan stays the owner's call (D7).

**Names chosen here**, routine and not the owner's decisions:

- the deb's helper unit `packaging/deb/massimo-update.service`;
  the path unit is `packaging/systemd/massimo-update.path`, unchanged
- no new command or flag for the helper:
  the marker decides the route in `massimo update`'s helper mode
- `install.UpdatePathWant`, the link enabling `massimo-update.path` makes,
  and `install.AptSource`, `/etc/apt/sources.list.d/massimo.sources`
- `massimo install` on a packaged box: the helper enabled unless `-no-update-helper`,
  as on a hand install; no new flag
- the API field `manager` (`apt`, `brew`) in `GET /api/v1/update`
- the branches `claude/apt-update` (Blocks 1 to 3),
  `claude/brew-update` (Block 4, from the first) and
  `claude/deb-version` (Block 5, from `main`)

**Not in this plan:**
rpm and dnf, which have no package yet;
an automatic downgrade (D3-A);
`massimo update` by hand on a packaged box, which keeps refusing;
the URIs of the boxes still on the old repository address,
which are edited by hand as `packaging/README.md` says;
the version of a build with no tag behind it (`git describe --always`),
which was never a valid package version and stays as it is.

**Who runs it:**
this session, under the owner's `PLAN++ D2-B` of 2026-11-09 for Blocks 1 to 4;
Block 5 waits for the owner's signal at this revision's gate.
Nothing is sent to a real box or the Mac without the owner's word.

## Analysis

### Measured on 2026-11-09

In `debian:bookworm-slim` (apt 2.6.1) and `debian:trixie-slim` (apt 3.0.4),
an fcntl lock held by a second process:

| Lock held | Command | Result |
|-----------|---------|--------|
| `/var/lib/apt/lists/lock` | `apt-get update`, `DPkg::Lock::Timeout=30` | exit 100 at once: "Could not get lock" |
| `/var/lib/apt/lists/lock` | `apt-get update` | the same |
| `/var/lib/dpkg/lock-frontend`, 12 s | `apt-get install --only-upgrade`, timeout 30 | waits ("Waiting for cache lock"), exit 0 after 11 s |
| `/var/lib/dpkg/lock-frontend`, 8 s | the same, timeout 3 | exit 100 after 3 s: "Unable to acquire the dpkg frontend lock" |

So `DPkg::Lock::Timeout` covers the install and not the update:
the helper retries `apt-get update` itself.
unattended-upgrades takes both locks in turn
(`apt-daily.service` updates, `apt-daily-upgrade.service` installs).

A deb822 file as `Dir::Etc::sourcelist`
(`sources.list.d/debian.sources`, `--print-uris`) is read as such,
and `Dir::Etc::sourceparts=-` warns only when the source list itself is unreadable.

### The apt route

The helper unit runs, as root:

```
/usr/bin/massimo update -request /var/lib/massimo/update-request \
  -status /var/lib/massimo-update/status.json -health ${MASSIMO_HEALTH}
```

In helper mode with a marker naming `apt`, `massimo update` takes the request
(its content, the channel, is ignored: apt follows the source's suite)
and runs, with `DEBIAN_FRONTEND=noninteractive` and `LC_ALL=C`:

1. `apt-get update -q -o Dir::Etc::sourcelist=sources.list.d/massimo.sources
   -o Dir::Etc::sourceparts=- -o APT::Get::List-Cleanup=0`:
   only the massimo source, the other lists kept;
   on exit 100 with "Could not get lock" again every 5 s, for 2 minutes;
2. `apt-get install -y -q --only-upgrade -o Dpkg::Options::=--force-confdef
   -o Dpkg::Options::=--force-confold -o DPkg::Lock::Timeout=120 massimo`;
3. the version before and after from `dpkg-query -W -f='${Version}' massimo`,
   as a tag `v` + the version with `~` → `-` (the Makefile's `DEBVER` reversed;
   Block 5 adds the part behind a `+`).

Status: `checking` during 1, `installing` during 2,
`up-to-date` when the version did not change
("apt has nothing newer yet": the GitHub release is out before the repository),
`restarting` while `/healthz` is asked for the new tag,
then `done` or `failed`.
The package's `postinst` restarts `massimo.service`;
the helper does not restart it a second time.
A health check that fails is reported with the version apt installed and the
way back, `sudo apt install massimo=<previous version>`;
nothing is downgraded (D3-A),
since an older binary would run on a store the newer one may have migrated.
A busy lock is reported as "apt is busy (another apt run, perhaps
unattended-upgrades); try again later".
A missing `massimo.sources` fails at once and names the file.

The commands are fixed in the binary; nothing from the request reaches them.
The tests replace them through a variable, as `packaged` is replaced today.

### The deb's helper unit

The tarball's `massimo-update.service` cannot run dpkg:
`ProtectSystem=strict` with only `/usr/local/bin` and `/var/lib/massimo` writable,
`PrivateDevices`, and the binary at `/usr/local/bin`.
apt and the maintainer scripts write `/usr`, `/etc` and `/var`,
add the user, run `systemctl daemon-reload` and restart the service.
The deb's own unit keeps `NoNewPrivileges`, `ProtectHome=read-only`, `PrivateTmp`,
`ProtectKernelTunables`, `ProtectKernelModules` and `ProtectControlGroups`,
and drops the rest.
The sandbox has been proven since:
under systemd in a container, and on the real box of 2026-11-10.

`packaging/deb/unit.sh` stops dropping `-update-request` from `massimo.service`.
`postinst` enables `massimo-update.path` with debhelper's snippet for a unit it
enables (`unmask`; `was-enabled` → `enable`, else `update-state`),
so a box upgraded from a package without the helper gets it enabled,
and one whose owner disabled it keeps it off.
`prerm` stops it on removal; `postrm` masks it on removal and purges its state
and `/var/lib/massimo-update` on purge.

### The server

`cmd/massimo/serve.go` on a marker naming `apt`:

- the channel is the first `Suites:` value of `install.AptSource`;
- `MASSIMO_UPDATE_CHANNEL` set to another channel → the reason
  "apt follows the suite <suite> of <file>; MASSIMO_UPDATE_CHANNEL=<c> is not used here";
- no source file → the reason names the file and the apt command;
- otherwise `Managed` stays empty and `Manager` is `apt`;
- `UpdateOptions.Helper`, asked at each request as the reboot helper's link is:
  "the update helper is off: sudo massimo install, or sudo apt update && sudo apt
  install massimo" while `install.UpdatePathWant` is missing.

`GET /api/v1/update` gains `manager`.
The UI says "installed by apt" beside the running version,
through `t()` in all six languages.
The version check stays GitHub's:
the release run publishes the release and the apt repository together.

### Homebrew (Block 4)

brew refuses to run as root and runs as the service's user,
so the server starts the helper itself, as on Windows:
this binary in helper mode, detached in a session of its own (`Setsid`),
so that it outlives the restart of the job it was started from;
its output goes to `var/log/massimo-update.log`.
The request and status files lie in `var/massimo/`.
The helper finds brew's prefix and the formula from its own path
(`<prefix>/Cellar/<formula>/<version>/bin/massimo`), and runs
`<prefix>/bin/brew update --quiet`, `brew upgrade <formula>` and
`brew services restart <formula>`, then waits for `/healthz`.
The formula builds from source, so the upgrade compiles: minutes, not seconds.
A server brew started as root (`sudo brew services`) gets the reason
"brew does not run as root:
brew upgrade <formula>, then sudo brew services restart <formula>".

**Found in existing code:** `homebrewLabel` is `homebrew.mxcl.massimo`,
but the channel's formula is `massimo-alpha`, whose job is `homebrew.mxcl.massimo-alpha`;
`rebootBy` then misses Homebrew's job and names a missing reboot helper
instead of "Homebrew's service runs as a user".
`restartBy` is unaffected (launchd is the parent either way).
Block 4 matches the label by its prefix.

### A build between tags (Block 5)

`DEBVER` is the tag without its `v`, every `-` as `~`.
A build after a tag, `v0.12.0-alpha.1-117-g3d7588f`,
becomes `0.12.0~alpha.1~117~g3d7588f`,
and dpkg sorts a `~` before the end of a version: before `0.12.0~alpha.1`.
So apt takes the release for the newer package.
`apt upgrade` and the helper take such a box back to the release's code,
and the build installs over the release only with `--allow-downgrades`.
Since the account is `_massimo`, that step back breaks the box:
v0.12.0-alpha.1's postinst adds a second account `massimo`,
and serve fails with `open /etc/massimo/env: permission denied`
(measured under systemd in a container, 2026-11-10).
A release is not affected: a tag has no part after it.
Only packages built by hand between tags are,
and the CI jobs that install them.

**The fix:** the part `git describe` adds goes behind a `+`, its `-` as `.`:

| `VERSION` | `DEBVER` today | `DEBVER` then |
|-----------|----------------|---------------|
| `v0.12.0-alpha.1` | `0.12.0~alpha.1` | the same |
| `v0.12.0-alpha.1-117-g3d7588f` | `0.12.0~alpha.1~117~g3d7588f` | `0.12.0~alpha.1+117.g3d7588f` |
| `v0.12.0-alpha.1-117-g3d7588f-dirty` | `0.12.0~alpha.1~117~g3d7588f~dirty` | `0.12.0~alpha.1+117.g3d7588f.dirty` |
| `v0.12.0-alpha.1-dirty` | `0.12.0~alpha.1~dirty` | `0.12.0~alpha.1+dirty` |
| `v1.0.0-3-gabcdef0` | `1.0.0~3~gabcdef0` | `1.0.0+3.gabcdef0` |
| `v0.12.0-alpha.1.1` | `0.12.0~alpha.1.1` | the same |

With `sed -E`, `$$` in the Makefile:
`s/^v//; s/-([0-9]+)-g([0-9a-f]+)-dirty$/+\1.g\2.dirty/;
s/-([0-9]+)-g([0-9a-f]+)$/+\1.g\2/; s/-dirty$/+dirty/; s/-/~/g`.
After the first three expressions no `-` is left behind the `+`.
No label or `t` command, so BSD sed reads it as GNU sed does.

Checked on 2026-11-10 with `dpkg --compare-versions`:

- `0.12.0~alpha.1` < `0.12.0~alpha.1+117.g3d7588f` < `0.12.0~alpha.1+225.gd8c2016`
  < `0.12.0~alpha.2` < `0.12.0` < `0.13.0~alpha.1`;
- `1.0.0` < `1.0.0+3.gabcdef0` < `1.0.1~alpha.1`;
- `0.12.0~alpha.1` < `0.12.0~alpha.1+dirty`,
  and a dirty build after its clean one;
- an installed `0.12.0~alpha.1~117~g3d7588f` < `0.12.0~alpha.1+117.g3d7588f`:
  a box on the old form upgrades onto the new one.

nfpm builds the `+` form as it is
(`make packages DEBVER=0.12.0~alpha.1+225.gd8c2016`:
the file names and the control `Version` agree).
`packaging/apt/publish.sh` reads no version from a file name.
`packaging/apt/test.sh`'s newer package, `$old+t1`, still sorts after `$old`.

**`debTag` reads it back:**
the part before the `+` with `~` → `-`,
then a `-` and the part after it with `.` → `-`:
`0.12.0~alpha.1+117.g3d7588f` → `v0.12.0-alpha.1-117-g3d7588f`,
which is what the binary reports,
and `waitHealthy` compares that string exactly.
A version without a `+`, the old form included, reads as before.

**CI proves the order from then on:**
`packaging/deb/systemd-cases.sh` installs v0.12.0-alpha.1, then the build.
Its `inst` passes `--allow-downgrades` today, because the build sorted below.
Without the flag, `apt-get -y` refuses a downgrade,
so the upgrade cases fail if a build ever sorts below the release again.

Nothing else makes or reads a package version:
`git grep` finds only `DEBVER`, `debTag` and its test,
the systemd cases' flag and `packaging/README.md`.

### What changes, by file

| File | Block | Change |
|------|-------|--------|
| `cmd/massimo/update.go`, `update_apt.go`, tests | 1 | the apt route in helper mode, its commands, the lock retries, the version as a tag |
| `packaging/deb/massimo-update.service` | 2 | new: the deb's helper unit |
| `packaging/nfpm.yaml`, `packaging/deb/unit.sh`, `postinst`, `prerm`, `postrm` | 2 | the helper's units shipped and enabled, the request flag kept |
| `internal/install/install.go`, `listen.go`, tests | 2 | on a packaged box the helper enabled or disabled, the health drop-in |
| `packaging/deb/test.sh`, `packaging/apt/test.sh` | 2 | the units, the link, the flag; a real upgrade between two versions, with a lock held |
| `packaging/README.md` | 2 | "Debian package": the helper, what it runs, how to turn it off |
| `internal/install/packaged.go` or a new file, tests | 3 | `AptSource`, the suite read, `UpdatePathWant` |
| `cmd/massimo/serve.go`, tests | 3 | the apt options |
| `internal/server/update_api.go`, test | 3 | `Manager`, `Helper`, `manager` in the view |
| `internal/server/ui/update.js`, `ui/i18n/*.json` | 3 | "installed by apt" |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml`, `docs/ui.md`, `README.md` | 3 | the field, the text, the update section |
| `cmd/massimo/update_brew.go`, `serve.go`, `restart.go`, `reboot.go`, tests | 4 | the brew helper, its starter, the label |
| `packaging/homebrew/massimo.rb.in`, `.github/workflows/ci.yml` | 4 | the caveats; the macOS job upgrades between two renders |
| `packaging/README.md`, `README.md`, `docs/ui.md` | 4 | "Homebrew": the button |
| `Makefile` | 5 | `DEBVER` with the `+`, its comment |
| `cmd/massimo/update_apt.go`, `update_apt_test.go` | 5 | `debTag` reads the `+`; the cases |
| `packaging/deb/systemd-cases.sh` | 5 | `inst` without `--allow-downgrades`, its comment |
| `packaging/README.md` | 5 | "Debian package": a build between releases |
| `CHANGELOG.md`, `CHANGELOG.d/Fixed/` | 1 to 5 | one bullet for apt, amended by Blocks 2 and 3; one for brew; Block 5's as a fragment |

`docs/update-windows.md` is unaffected: nothing changes on Windows.

### Risks

- `apt-get update` of one source leaves the other lists as they were;
  a dependency newer than those lists know is not found,
  and the install fails with apt's message, as `apt upgrade` would with stale lists.
- The postinst restarts the service only when it runs or `/etc/massimo/env`
  sets more than the template; a stopped server on the template's settings stays stopped,
  and the health check reports it.
- launchd may stop a helper started from its job despite the new session;
  the real Mac decides, and the CI job's upgrade runs without `brew services`.
- dpkg sorts `+` before `.`, so a build between tags sorts below a hand-made
  version such as the test builds' `0.12.0~alpha.1.1`.
  The box of 2026-11-10 is on such a build;
  it takes the next release, and a build from `main` only by hand.

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD`.
Nothing reaches a Cube, a real box or the Mac;
the containers and CI are the evidence.
One functional commit per block, citing `AP AptUpdate v1.0, Block <n>`
(Block 5: `v1.1`), with its CHANGELOG bullet.

### Block 1: AptHelper

Done, in #127.

1. `cmd/massimo/update_apt.go`: the commands as fixed argument lists,
   the run through a replaceable variable, the lock retry
   (exit 100 and "Could not get lock", every 5 s for 2 minutes),
   `dpkg-query` for the version, the tag conversion.
2. `cmd/massimo/update.go`: in helper mode a marker naming `apt` takes the apt route,
   with the states and messages of the Analysis;
   without `-request` the refusal stays.
3. Tests with a stand-in apt-get: the commands as run,
   up-to-date, done, a lock released after two tries, a lock never released,
   an install failing, a health check failing (the way back in the message),
   the source file missing, the request's channel ignored.
4. CHANGELOG, Added: the update helper upgrades massimo through apt on a box the
   Debian package installed.

**Verification:** the test gate; the stand-in sees exactly the two command lines.

### Block 2: AptPackage

Done, in #127.

1. `packaging/deb/massimo-update.service`, as the Analysis says;
   `nfpm.yaml` ships it and `packaging/systemd/massimo-update.path`.
2. `packaging/deb/unit.sh` keeps `-update-request`; its comment and check change.
3. `postinst`, `prerm`, `postrm` as the Analysis says.
4. `internal/install`: `configure` enables `massimo-update.path` and writes the
   health drop-in unless `-no-update-helper`, which disables the path unit and
   removes the drop-in; tests.
5. `packaging/deb/test.sh`: the units, the enabled link, the flag in the unit,
   `install -no-update-helper -dry-run`, a reinstall keeping a disabled helper off,
   the purge taking the link and `/var/lib/massimo-update`.
6. `packaging/apt/test.sh`: a second package from the amd64 one with a higher
   version (`dpkg-deb -R`, `-b`), both in the repository;
   the container installs the older, writes the request,
   runs the helper's command without `-health`
   while a second process holds the lists lock for 8 s,
   and finds the newer version installed and the status `done`.
7. `packaging/README.md`, "Debian package": the helper, what it runs,
   the lock, the way back, `-no-update-helper`.
8. CHANGELOG: the Block 1 bullet amended: the package ships the helper, enabled.

**Verification:** the test gate; `make packages`, `make deb-test` and
`make apt-test` on trixie and bookworm.

### Block 3: AptButton

Done, in #127.

1. `internal/install`: `AptSource`, the first suite read from it, `UpdatePathWant`; tests.
2. `cmd/massimo/serve.go`: the apt options of the Analysis; tests over a root
   with the marker, the source, the link and the variable.
3. `internal/server/update_api.go`: `Manager`, `Helper`, `manager` in the view;
   tests: the button offered, the helper off, the channel refused,
   the request written.
4. `internal/server/ui/update.js` and the six `ui/i18n/*.json`
   (FR, IT, NL, PL machine-translated and marked): "installed by apt".
5. `docs/api-v1.md` and the OpenAPI file: `manager`;
   `docs/ui.md` and `README.md`: the button on a packaged box.
6. CHANGELOG: the bullet amended: the web UI's update button upgrades through apt.

**Verification:** the test gate, `TestOpenAPICoversRoutes`,
`make ui-test` at 1100 and 390 px.
Then the branch merges `origin/main`, is pushed and gets its pull request.

### Block 4: BrewUpdate

Done, in #128, with the fix of its Windows test.

On `claude/brew-update`, from Block 3's commit; its own pull request,
based on the first one's branch.

1. `cmd/massimo/update_brew.go`: the prefix and formula from the binary's path,
   the three brew commands through a replaceable variable,
   the version from `brew list --versions <formula>`; the states as for apt.
2. `cmd/massimo/serve.go`: on a marker naming `brew`, not root,
   the request and status in `var/massimo/`, the detached starter (`Setsid`),
   `Manager` `brew`; root gets the reason of the Analysis.
3. `restart.go`, `reboot.go`: the Homebrew label matched by its prefix; tests.
4. `.github/workflows/ci.yml`, macOS job: a second render of the formula with a
   higher version in the local tap, the helper's command run against the first,
   the newer version installed.
5. `packaging/homebrew/massimo.rb.in`'s caveats, `packaging/README.md`, "Homebrew",
   `README.md`, `docs/ui.md`.
6. CHANGELOG, Added: the web UI's update button upgrades through brew.

**Verification:** the test gate, the macOS job;
the real Mac with the owner's go.

### Block 5: DebVersion

On `claude/deb-version`, from `main` after this revision's commit;
its own pull request.

1. `Makefile`: `DEBVER` as the Analysis says;
   its comment: a tag's form, and a build between tags behind a `+`,
   which sorts after the tag it follows and before the next.
2. `cmd/massimo/update_apt.go`: `debTag` cuts at the first `+`;
   its comment names both forms.
3. `cmd/massimo/update_apt_test.go`, `TestDebTag`:
   the `+` forms of the Analysis's table (with `.dirty`, `+dirty`, a release
   without a pre-release), the old `~` form, and the cases it has.
4. `packaging/deb/systemd-cases.sh`: `inst` without `--allow-downgrades`;
   its comment says why: a build that sorted below the release would fail here.
5. `packaging/README.md`, "Debian package":
   a package built between releases, its version, and where dpkg sorts it.
6. The changelog's fragment `CHANGELOG.d/Fixed/claude-deb-version.md`
   (COMMIT.md §5.5): a package built between two releases sorts after the one
   it follows, so apt no longer takes it back to that release.

**Verification:** the test gate;
`make packages` at a commit after the tag: the files named `…+<n>.g<sha>_…`;
`dpkg --compare-versions` of that version against the tag it follows (gt)
and the next pre-release (lt);
`make deb-test`, `make deb-systemd-test` (all five cases) and `make apt-test`,
on trixie and bookworm.
Then the branch is pushed and gets its pull request.

## Proposed commit message

This revision, a commit of its own on `main` before Block 5's branch:

```
chore(aps): +AptUpdate v1.1

Block 5 added to the plan: a package built between two releases gets its git describe part behind a +, so that dpkg sorts it after the release it follows; with the owner's D9-A and D10-A, and Blocks 1 to 4 marked done.

Key changes:
- GUIDELINES/wip/2026-11-09_04-42_AP_AptUpdate_update_button_apt_brew.md: v1.1, Block 5 (DebVersion), the finding F1 measured, where the plan stands.

File-revision-v1: GUIDELINES/wip/2026-11-09_04-42_AP_AptUpdate_update_button_apt_brew.md::v1.1::2026-11-10::All::Block 5 for F1; D9-A, D10-A; Blocks 1 to 4 done
Assisted-by: claude-code:claude-opus-5-5
```

Block 5's commit:
`fix(packaging): A package built between releases sorts after the one it follows`,
citing `AP AptUpdate v1.1, Block 5`.

The blocks' earlier commits:
`feat(update): The update helper upgrades through apt on a packaged box`,
`feat(packaging): The Debian package ships the update helper, enabled`,
`feat(server): The update button on a box apt owns`,
`feat(update): The update button through brew on a Homebrew box`.

## Change History

- v1.0 (2026-11-09): the first revision, from the owner's wish and the Debian
  package's carried-over helper, with route 1, D1-A, D2-B and D3-A,
  and Block 4 as a pull request of its own.
- v1.1 (2026-11-10): Block 5 (DebVersion) for finding F1, a build between tags
  sorting before its tag, with D9-A ("PLAN+ F1 fix") and D10-A ("Yes, run them");
  Blocks 1 to 4 marked done; the real apt box's run and its NOTE named;
  the sandbox risk dropped, proven; the start-rule risk brought up to ServiceStart;
  Block 5's bullet a fragment, as the guidelines now ask.
