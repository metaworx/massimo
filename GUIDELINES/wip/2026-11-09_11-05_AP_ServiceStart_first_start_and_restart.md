# AP ServiceStart v1.0: First start, then every restart

## TL;DR

Every platform gets one rule.
An install or an upgrade restarts a service that is running.
A service that never ran on the template's settings stays stopped until the
owner starts it,
and the messages say how to give it an owner without a settings line.

- `postinst`:
  template unchanged → not started *and not restarted* ⇒ running →
  `deb-systemd-invoke restart`; stopped on the template → a message
- "still the template": `cmp` byte for byte → the settings lines only,
  comments and blank lines ignored
- `massimo install` on Linux, a packaged box, macOS and Windows → the same rule,
  the running state read first
- "put the Cubes and MASSIMO_API_TOKEN into …" → create the owner first with
  `massimo user add`, or start the service and create the owner in the web UI;
  Cubes are added there
- `/var/lib/massimo` → made by the package and by `install`,
  so `user add` works before the first start
- `test.sh`:
  stand-ins for `systemctl` and `deb-systemd-invoke` → each case tested in the
  container
- one pull request, opened after AptUpdate's first one merges

### Blocks

1. Package: the `postinst` rule, the state directory, the message;
   the cases in `test.sh`; packaging/README.md "Debian package"
2. Install: the rule on the four paths, the state directory, the token warning;
   tests; README.md "As a service" for Linux, Windows and macOS,
   packaging/README.md "Install by hand"
3. Texts: the token comment in `env.template`; the Homebrew caveats

## Discussion

The finding (2026-11-09, made in Block 11 of AP Packaging v1.1):
the Debian package's `postinst` starts the service,
or restarts it with the new binary,
only once `/etc/massimo/env` differs from the shipped template,
"so that a fresh box does not serve the LAN without a token",
and `massimo install` has the same rule.
Since then, the server works without either line.
While there are no users, the API answers the local network only,
and the web UI offers a first-owner form there.
A Cube found by discovery is added from System settings → Cubes → Found on the LAN.
So:

1. the message asks for lines that are not needed;
2. a box still on the template, started by hand and set up through the web UI,
   is never restarted by the package:
   after `apt upgrade` the old binary runs until a reboot;
3. the reason still has weight:
   a service started with no users leaves the first-owner form open to the
   whole LAN until someone claims it.

**The owner's decisions** (`PLAN++` as recommended, 2026-11-09):

- **D1-A:**
  restart what runs, and leave the first start to the owner.
  Not B, starting always, which opens every fresh box to the LAN from the
  install until it is claimed.
  Not C, the messages alone: then AptUpdate's update button would fail its
  health check on such a box, a risk AptUpdate's own plan lists.
  D, starting always with a setup code that the first-owner form asks for,
  would close finding 3 for real.
  It touches the server, the API, the UI and the apps,
  so it would be a plan of its own, and it fits on top of this one.
- **D2-A:**
  the message offers both ways.
  One is creating the owner before the first start with `massimo user add`,
  which leaves the LAN no window at all.
  The other is starting the service and creating the owner in the web UI.
  The package and `install` make the state directory for the first way.
- **D3-A:**
  the branch starts from main now.
  Its pull request opens after AptUpdate's first pull request (Blocks 1 to 3) merges,
  and this branch resolves the conflicts.

## Analysis

### What the rule protects today

Less than it says.
The package and every `massimo install` enable the service:
`deb-systemd-helper enable`, `systemctl enable`,
Windows' automatic start, launchd's `RunAtLoad`.
A box left on the template therefore starts at its next boot, unclaimed.
The rule only delays that start until the next boot.
A second `massimo install` restarts the service on the template too,
because there `startable` means "the settings file exists".
On Windows and macOS, `install` stops or boots out a running service
and starts it again on the same terms.

**A defect, found on the way.**
`cmp` compares byte for byte,
so an untouched settings file stops being "the template" as soon as a release
changes the template's comments. v0.12.0-alpha.1 did that (`MASSIMO_REBOOT`),
and the next release does it again (`MASSIMO_REBOOT`'s values,
`MASSIMO_ANNOUNCE`).
Every such upgrade starts an unclaimed service.
Each template so far sets one line only, `MASSIMO_LOG=…`:
comparing the lines that set something,
with comments and blank lines left out,
recognizes every one of them.

### The rule

For every platform, at the point where today's code starts or restarts:

| The settings | The service | Then |
|--------------|-------------|------|
| more than the template sets | any | started, or restarted (as today) |
| only what the template sets | running (`active`, `activating`, `reloading`) | restarted, so the new binary runs |
| only what the template sets | stopped | not started; the message |

`-no-start` keeps its meaning.
On Debian the running state is `systemctl is-active massimo.service`,
taken after `daemon-reload` and before the restart.
`activating` covers a service in `Restart=on-failure`'s pause:
the owner started it, and the new binary may be the fix it waits for.

### Where it is read

| Path | Running | "Only the template" against |
|------|---------|------------------------------|
| `packaging/deb/postinst` | `systemctl is-active` | `/usr/share/massimo/env.template` |
| `Install` (Linux) | `in.Output("systemctl", "is-active", "massimo.service")` | `envTemplate()` |
| `configure` (a packaged box) | the same | `/usr/share/massimo/env.template`, else `envTemplate()` |
| `InstallDarwin` | `loaded(Label)`, now through `in.Output(launchctl, "print", …)` | `darwinEnvTemplate()` |
| `InstallWindows` | the state `stopService` found before stopping it | `windowsEnvTemplate(p)` |

`loaded` moves from `in.Run` to `in.Output`.
It is the same read-only command, and the tests can answer it,
which the `in.Root == ""` guard prevented until now.
With `Root` set and no `Output` given, nothing is loaded, as before.
`settings()` and `winSettings()` report an existing file as filled in
only when it sets more than the template.
`packagedSettings()` compares the same way.

### The message

On Linux, from `postinst` and from `install`:

```
not started: until it has an owner, every device on the local network may use it
create the owner first: sudo -u massimo massimo -data /var/lib/massimo user add <name> -role owner
then start it: sudo systemctl start massimo
or start it and create the owner in the web UI; the Cubes are added there (System settings → Cubes)
```

On macOS the user is `_massimo` and the start is `launchctl bootstrap`.
On Windows, `massimo.exe -data C:\ProgramData\Massimo user add …`
runs from an administrator prompt, and the start is `sc start massimo`.
`postinst` prefixes each line with `massimo: `, as it does today.

### The state directory

systemd makes `/var/lib/massimo` at the first start (`StateDirectory=massimo`),
and the service user cannot create it in `/var/lib`,
so `user add` before the first start would fail.
`postinst` and the Linux `install` make it, `0750 massimo:massimo`,
when it is missing. macOS (`StateDir`) and Windows (`DataDir`) already make
theirs.
`apt purge` and `install -remove -purge` take it along, as today.

### Further texts that are out of date

- `checkSettings`:
  "no MASSIMO_API_TOKEN:
  the API is open to the LAN and the web UI cannot start updates".
  A user is enough for both (`updateBlocked`, `openAllowed`).
- `env.template`:
  "Needed to start updates from the web UI, and recommended anyway".
- packaging/README.md: "The next package upgrades the binary and restarts the
  service", untrue for a box on the template.
- The Homebrew caveats ask for the Cubes' addresses and the token.
  `brew upgrade` does not restart a service run by `brew services`;
  `brew services restart` does.
  brew never starts the service by itself, so there is no rule to change there.

### AptUpdate

AptUpdate's branch, not pushed yet, changes `postinst`, `test.sh`, `configure`,
`packaged_test.go`, both READMEs and the Homebrew formula.
Its risk "a server running on the template's settings is not restarted, and
the health check reports it" goes away with Block 1.
Per D3-A, this branch merges main after AptUpdate's first pull request
and resolves the conflicts here.
Its Block 4 meets the caveats of Block 3 later.

### What changes, by file

| File | Block | Change |
|------|-------|--------|
| `packaging/deb/postinst` | 1 | the rule, the comparison, the state directory, the message |
| `packaging/deb/test.sh` | 1 | stand-ins; the four cases; the state directory |
| `packaging/README.md` | 1, 2 | "Debian package"; "Install by hand" and "One owner of the binary" |
| `internal/install/install.go` | 2 | `onlyTemplate`, `active`, the rule in `Install` and `configure`, the state directory, the message, the token warning |
| `internal/install/darwin.go`, `windows.go` | 2 | the rule; `loaded` through `Output`; `stopService` reports what it found |
| `internal/install/*_test.go` | 2 | the cases |
| `README.md` | 2 | "As a service" for Linux, Windows and macOS |
| `packaging/env.template` | 3 | the token's comment |
| `packaging/homebrew/massimo.rb.in` | 3 | the caveats |
| `CHANGELOG.md` | 1, 2, 3 | a bullet each |

### Risks

- The container has no systemd.
  The stand-ins prove the script's choices, not systemd's behaviour,
  so a start on a real box stays the owner's check.
- Windows' path is compiled and vetted only:
  there are no tests of `InstallWindows`,
  and the owner's Windows service is off limits.
- A box that ran on the template and was stopped by its owner stays stopped
  after an upgrade, as it should.
  A box with filled-in settings is started by an upgrade, as today.

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, and `make cross`.
Block 1 also runs `make packages` and `make deb-test`.
Before every commit, `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD`
runs from the main checkout.
Nothing reaches a Cube, a Raspberry Pi, the Windows service or the Mac.
Each block is one functional commit, citing `AP ServiceStart v1.0, Block <n>`,
with its CHANGELOG bullet.

### Block 1: Package

1. `postinst`:
   a `settings` function printing the lines that set something, trimmed;
   `/var/lib/massimo` made `0750 massimo:massimo` when missing;
   under `/run/systemd/system`, after `daemon-reload`:
   restart when the settings set more than the template's
   or `systemctl is-active` says `active`, `activating` or `reloading`;
   else the message.
2. `test.sh`, while the settings are the template:
   stand-ins for `systemctl` and `deb-systemd-invoke` in `/usr/local/sbin`,
   which log their calls and answer `is-active` from a file,
   and `/run/systemd/system` made for the reinstalls only.
   The cases:
   - stopped → no restart, the message;
   - running → `restart massimo.service`;
   - the template with a comment changed, stopped → no restart;
   - a setting added → restart.
     The stand-ins are removed before the remaining steps;
     `/var/lib/massimo` is checked after the install,
     and its absence after the purge.
3. packaging/README.md, "Debian package": the rule, the two ways to an owner,
   the restart on upgrade.
4. CHANGELOG, Fixed: the package restarts a running service on upgrade,
   whatever the settings, and leaves an unclaimed box stopped.

**Verification:**
the test gate; `make packages` and `make deb-test` with every case passing.

### Block 2: Install

1. `install.go`:
   `settingLines` and `onlyTemplate`; `active()`;
   `settings()` and `packagedSettings()` through `onlyTemplate`;
   the rule in `Install` and `configure`; `/var/lib/massimo` in `Install`;
   `notStarted(userAdd, start)` for the message; the token warning's text.
2. `darwin.go`:
   `loaded` through `in.Output`, `len(out) > 0`; the rule;
   the message with `_massimo` and `launchctl bootstrap`.
3. `windows.go`:
   `stopService` returns whether the service ran;
   `winSettings` through `onlyTemplate`; the rule; the message.
4. Tests:
   - `onlyTemplate` with comments changed, blank lines, CRLF, a line added;
   - Linux: fresh → not started, the message names `user add`;
     again on the template, stopped → not started;
     on the template, `active` → restarted; the state directory made;
   - packaged: on the template, `active` → restarted;
   - macOS: on the template, loaded → `kickstart -k`;
   - the warning's new text.
5. README.md "As a service" (Linux, Windows, macOS),
   packaging/README.md "Install by hand" and "One owner of the binary":
   the rule and the message.
6. CHANGELOG, Changed: `massimo install` restarts a running service and leaves
   an unclaimed one stopped, on every platform,
   and says how to create the owner.

**Verification:**
the test gate; on Windows, `GOOS=windows go vet` and `make cross` only.

### Block 3: Texts

1. `env.template`:
   the token is the owner's key at the first start,
   one way among `massimo user add` and the web UI,
   and without a user or the token the API answers the local network only.
2. `massimo.rb.in`, caveats:
   nothing to fill in;
   `brew services start`, then the owner in the web UI,
   or `massimo -data #{var}/massimo user add` before the start;
   `brew services restart` after `brew upgrade`.
3. CHANGELOG, Changed: the template's and the formula's texts.

**Verification:**
the test gate; `render.sh` renders the formula,
and `ruby -c` accepts it where ruby is available.

## Proposed commit message

The plan's registration,
a commit of its own on main before the first project write:

```
chore(aps): +ServiceStart v1.0

One rule for when a service install or upgrade starts massimo: what runs is restarted, a box that never ran on the template's settings is left for its owner, and the messages say how to create the owner.

Key changes:
- GUIDELINES/wip/2026-11-09_11-05_AP_ServiceStart_first_start_and_restart.md: the plan, three blocks, the owner's D1-A, D2-A and D3-A.

File-revision-v1: GUIDELINES/wip/2026-11-09_11-05_AP_ServiceStart_first_start_and_restart.md::v1.0::2026-11-09::All::the first revision, by PLAN++ as recommended
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
`fix(packaging): The package restarts a running service on upgrade, and leaves an unclaimed box stopped`,
`feat(install): One start rule on every platform, and the way to the first owner`,
`docs(packaging): The template's token comment and the Homebrew caveats`.

## Change History

- v1.0 (2026-11-09): the first revision, from the finding of AP Packaging v1.1,
  Block 11, with D1-A, D2-A and D3-A.
