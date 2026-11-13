# Massimo

Open-source successor software for eQ-3 MAX! heating systems (MAX!
Cube LAN gateway, radiator and wall thermostats, window contacts,
eco switches) after the vendor's end of life.

Status: early development;
Phase 0 of [docs/design.md](docs/design.md) is done, Phase 1 in progress;
every release is a pre-release on the `alpha` channel (see [Versions](#versions)).

## Contents

- [Features](#features)
- [Documentation](#documentation)
- [Installing](#installing): [Linux](#linux), [Windows](#windows),
  [macOS](#macos), [From source](#from-source)
- [Server](#server), with [Screenshots](#screenshots)
- [Command line](#command-line), with the [Cube variables](#cube-variables)
- [Original software](#original-software)
- [Versions](#versions)
- [Legal](#legal)

## Features

![The Rooms page](docs/screenshots/rooms-1100-light-en.png)

- **The web UI**:
  rooms as tiles with the measured and the set temperature, modes and setpoints,
  the House panel, the week programme editor, the device list and the log;
  light and dark, colour schemes of your own, six languages,
  phone and desktop ([docs/ui.md](docs/ui.md)).
- **A JSON API** with an OpenAPI description, live events and a bearer token
  ([docs/api-v1.md](docs/api-v1.md)).
- **Pairing** a new device from the web UI or the API,
  put into a room with the room's settings as the original software does,
  and a device removed from the Cube again ([docs/ui.md](docs/ui.md)).
- **Backup and restore** of a Cube's configuration as YAML or JSON,
  from the command line and the API.
- **The server as a Cube** for other clients, the original software included:
  one Cube, many clients ([docs/proxy.md](docs/proxy.md)).
- **Discovery and network configuration** of Cubes on the LAN.
- **The radio duty cycle respected**:
  a command queue per Cube, the Cube's budget in the log.
- **A service** on Linux (systemd), Windows and macOS,
  with signed updates from the web UI or `massimo update`.
- **The original software** run with a protocol log (`massimo legacy`).

Platforms: Linux on amd64 and ARM (a Raspberry Pi 1 is enough),
Windows 64-bit, macOS on Apple silicon and Intel;
the web UI in any current browser, on a phone or a desktop.

## Documentation

<!-- BEGIN GENERATED PARTIAL docs/fragments/_documents.md - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->
Every document of the project, in the README and on the site's docs index
(`packaging/site/build.sh` builds a page of each one listed here, and fails
on a document under `docs/` that is not).

### Using it

- [README.md](README.md): what Massimo is, installing it, the server,
  the command line, the versions and channels
- [packaging/README.md](packaging/README.md): the packages and services,
  the apt repository, Homebrew, updates
- [docs/ui.md](docs/ui.md): using the web UI
- [docs/app.md](docs/app.md): the apps for Android and iOS
- [docs/proxy.md](docs/proxy.md): the server as a Cube for other clients
- [docs/homeassistant.md](docs/homeassistant.md): Home Assistant,
  through the proxy today and with an integration of its own later
- [docs/reverse-proxy.md](docs/reverse-proxy.md):
  `massimo serve` behind a reverse proxy
- [docs/oidc.md](docs/oidc.md): signing in through an OpenID Connect
  provider such as Nextcloud, Google or Microsoft

### The API

- [docs/api-v1.md](docs/api-v1.md),
  [docs/api-v1-openapi.yaml](docs/api-v1-openapi.yaml): the HTTP API

### How it works

- [docs/cube-protocol.md](docs/cube-protocol.md):
  everything learned about the Cube, the portal and the radio commands
- [docs/background.md](docs/background.md): the evaluation before the project,
  licence reasoning, routes to a public server, sources
- [docs/design.md](docs/design.md): the design proposal, target environments,
  phases and their status
- [docs/store.md](docs/store.md): the design of the server's persistent storage:
  engine, schema, users and roles, migration of installed boxes
- [docs/server.md](docs/server.md): the server mode: encryption at rest,
  the public server, sync, and what the store prepares for them
- [docs/update-windows.md](docs/update-windows.md):
  the design for updates from the web UI on Windows: the options,
  the recommendation, the changes per file, the CI and real-PC tests

### Contributing

- [GUIDELINES/README.md](GUIDELINES/README.md): how the project is worked on:
  the guidelines, the project's contract and its coding standard
- [docs/roadmap.md](docs/roadmap.md): what comes next, in order
- [docs/testing-writes.md](docs/testing-writes.md):
  the test plan for the configuration writes on real hardware,
  [docs/reports/write-test-2026-11-05.md](docs/reports/write-test-2026-11-05.md)
  its results
- [docs/testing-windows.md](docs/testing-windows.md):
  the test plan for the Windows service on a real PC,
  [docs/reports/windows-test-2026-11-05.md](docs/reports/windows-test-2026-11-05.md) its
  results
- [tools/ui-test/README.md](tools/ui-test/README.md): `make ui-test`,
  the web UI smoke test with Playwright, and its setup
- [docs/ui-testing.md](docs/ui-testing.md): testing the web UI:
  the smoke test, Cypress and Playwright compared, screenshots,
  the recommendation
<!-- END GENERATED PARTIAL docs/fragments/_documents.md -->

## Installing

Massimo is one binary per platform with the web UI inside;
there is nothing else to install.
Every [release](https://github.com/metaworx/massimo/releases) carries the binaries below,
`SHA256SUMS`, the signed manifest that `massimo update` verifies,
and the build record, `massimo-build.txt`.
Every release so far is a pre-release of the flavour `alpha` (see
[Versions](#versions)), which `releases/latest` skips,
so take the newest release from the page and put its tag in place of `<tag>` below.

| Asset | For |
|---|---|
| `massimo-linux-amd64` | PC or NAS with a 64-bit Linux |
| `massimo-linux-arm64` | Raspberry Pi 3, 4, 5 and Zero 2 W with a 64-bit OS (`uname -m` says `aarch64`) |
| `massimo-linux-armv7` | Raspberry Pi 2, 3, 4 and Zero 2 W with a 32-bit OS (`armv7l`) |
| `massimo-linux-armv6` | Raspberry Pi 1 (A, B, B+), Zero and Zero W (`armv6l`; the armv7 build dies there with "illegal instruction") |
| `massimo-windows-amd64.exe` | Windows PC, 64-bit |
| `massimo-darwin-arm64` | Mac with Apple silicon |
| `massimo-darwin-amd64` | Mac with an Intel processor |
| `massimo_<version>_<arch>.deb` | the Linux binary as a Debian package: `amd64`, `arm64`, `armhf` (every 32-bit Raspberry Pi OS), see [Linux](#linux) |

<!-- BEGIN GENERATED PARTIAL docs/fragments/_trust.md - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->
**A binary is only as trustworthy as the place it came from.**
Massimo runs as a service on your network and controls your heating,
so take it from the project's [releases](https://github.com/metaworx/massimo/releases),
its apt repository or its Homebrew tap, and never from a copy elsewhere:
a mirror, a forum post or a prepared image may carry a changed binary,
and no check of the project's can tell.
`SHA256SUMS` proves that a download arrived as the release has it,
not who built it:
it sits on the same page as the binaries,
and whoever could replace one could replace the other.
Once a binary runs,
`massimo update` checks every later release against the release key compiled into it,
and apt against the repository's key,
so what is left to trust is the first install.
To trust no build but your own, build it from the signed source:
[From source](https://github.com/metaworx/massimo#from-source).
<!-- END GENERATED PARTIAL docs/fragments/_trust.md -->

The build record says how a release was built:
the runner,
the Go version and every library of each binary with its checksum (`go version -m`,
which also reads a downloaded binary),
and the build's whole log;
`SHA256SUMS` and the signed manifest list it.
GitHub's build provenance says who built each file,
the record and `SHA256SUMS` included:
the project's release workflow, at which commit, on a GitHub runner,
signed through Sigstore rather than with the project's keys:

```bash
gh attestation verify massimo-linux-amd64 -R metaworx/massimo
```

`serve` reads its settings from the environment or from a `.env` in the
directory it is started from (see [Cube variables](#cube-variables)).
<!-- BEGIN GENERATED PARTIAL docs/fragments/_first-settings.md - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->
On the Cubes' LAN the server needs no settings to begin with:
while there are no users,
the web UI, `http://<the box>:8099/`,
offers the local network a form for the first owner,
and System settings → Cubes lists the Cubes the server finds,
each added with **Add**.
The settings take what discovery and the web UI cannot give:
the address of a Cube that discovery does not reach,
on another subnet or behind a VPN (`massimo discover` lists the Cubes it hears,
with serial and address),
and a token that becomes the owner's key at the first start,
which the API and the web UI then ask for,
so that no form for the first owner is open on the LAN:

```
MASSIMO_CUBE_KEQ0000000_ADDRESS=192.168.1.10
MASSIMO_API_TOKEN=choose-a-long-random-string
```
<!-- END GENERATED PARTIAL docs/fragments/_first-settings.md -->

Each Cube accepts a single TCP client,
so close the original MAX! software and stop any other server before `serve` connects;
from then on they connect to the server's proxy instead of the Cube,
see [Server](#server).
The proxy needs TCP port 62910 and discovery UDP port 23272 open to the LAN,
the announcement UDP port 5353,
and the API its port (8099 below) when it listens on a LAN address;
each subsection says what that means for the firewall.
WSL 2 in its default network mode has no LAN broadcast and takes no inbound LAN
connections, so a `serve` there can neither find Cubes nor answer as one,
and `massimo discover` finds nothing (mirrored networking mode on Windows 11 lifts that);
the service belongs on a Linux box or a Raspberry Pi,
a Windows PC or a Mac on the Cube's LAN.

### Linux

A Raspberry Pi is enough, a Pi 1 included
([resource use](packaging/README.md#resource-use));
[Preparing a Raspberry Pi 1](packaging/README.md#preparing-a-raspberry-pi-1)
sets one up without a screen.

**Try it.**
On the box, with the asset for it from the table (here the arm64 one):

```bash
curl -fLO https://github.com/metaworx/massimo/releases/download/<tag>/massimo-linux-arm64
curl -fLO https://github.com/metaworx/massimo/releases/download/<tag>/SHA256SUMS
sha256sum -c SHA256SUMS --ignore-missing
chmod +x massimo-linux-arm64
./massimo-linux-arm64 serve -listen 127.0.0.1:8099
```

with the `.env` from above in the same directory, if any,
then open http://127.0.0.1:8099/ and enter the token or create the first owner;
`-listen :8099` serves the page to the LAN as well,
`-proxy off` leaves the proxy out for a first run.
A firewall on the box must allow incoming 62910/tcp, 23272/udp and 5353/udp,
and 8099/tcp for the page on the LAN (with ufw:
`sudo ufw allow 62910/tcp && sudo ufw allow 23272/udp && sudo ufw allow 5353/udp && sudo ufw allow 8099/tcp`).

**As a service.**
On a Linux with systemd, as root (`-dry-run` only shows what would be done):

```bash
sudo ./massimo-linux-arm64 install -dry-run -settings .env
sudo ./massimo-linux-arm64 install -settings .env
```

installs the binary as `/usr/local/bin/massimo`,
creates the system user `_massimo`,
copies the `.env` to `/etc/massimo/env` (without `-settings` the commented template,
which is all a first start needs),
writes `massimo.service`,
the update helper `massimo-update.path` and `.service` and the log rotation,
enables them and starts the service,
which listens on `:8099` on every interface.
On the template's settings it only restarts a service that runs:
a server without users answers every device on the local network (see "Server"),
so the first start is the owner's,
after `sudo -u _massimo /usr/local/bin/massimo user add <name> -role owner`
or followed by the first-owner form of the web UI.
It warns when `MASSIMO_API_TOKEN` is missing.
`-listen :80` moves the API and web UI to another address (kept by later installs),
`-no-update-helper` leaves the helper out,
and removes it where an earlier install put it,
`-no-start` only enables.
With several Cubes each needs its own `MASSIMO_CUBE_<SERIAL>_PROXY` address on the box,
see [Install by hand](packaging/README.md#install-by-hand).
Afterwards the settings are in `/etc/massimo/env` (readable by `massimo` only),
the state in `/var/lib/massimo`,
the log in `/var/log/massimo/serve.log`
(JSON, rotated by `serve` itself, readable by the group `adm`) and in the journal:

```bash
sudo systemctl start massimo
sudo systemctl stop massimo
sudo systemctl status massimo
journalctl -u massimo -f                  # or: tail -f /var/log/massimo/serve.log
sudo massimo update -restart massimo.service -health http://127.0.0.1:8099/healthz
sudo massimo install -remove              # -purge: the settings, state, logs and user too
```

`update` installs the newest signed release the box's update channel takes
([Versions](#versions)) and puts the old
binary back when the restarted server does not report the new version
(`-rollback` goes back by hand);
the button under System settings → Software in the web UI asks the helper to do the same,
which needs `MASSIMO_API_TOKEN`.
Running `install` again from a newer binary replaces the binary and the units
and keeps the settings.
Details: [Install with
`massimo install`](packaging/README.md#install-with-massimo-install),
[Moving to another network](packaging/README.md#moving-to-another-network),
[Updating](packaging/README.md#updating).

**As a package.**
On Debian, Ubuntu, Raspberry Pi OS or LoxBerry the release's `.deb` does the
same install through apt (`sudo apt install ./massimo_<version>_arm64.deb`;
`_armhf` for a 32-bit Raspberry Pi OS, the Pi 1 included, `_amd64` for a PC),
creates the user and `/etc/massimo/env` to fill in,
and owns the binary from then on:
`massimo update` says when a release is out and leaves the install to apt,
the web UI's Update has the package's helper run apt for the project's source,
`apt purge massimo` removes everything.
The project's apt repository, <https://massimo.metaworx.ch/apt/>,
makes that an `apt upgrade`:
one suite per flavour, `devel` and `nightly` signed with a key of their own,
the snippet in [The apt repository](packaging/README.md#the-apt-repository),
and each channel's page on the site names its own suite.
The update channel chosen in the web UI switches the suite at the next update.
Details: [Debian package](packaging/README.md#debian-package).

### Windows

**Try it.**
In PowerShell, in a folder of your choice (`curl.exe` comes with Windows 10 and 11):

```powershell
curl.exe -fLO https://github.com/metaworx/massimo/releases/download/<tag>/massimo-windows-amd64.exe
curl.exe -fLO https://github.com/metaworx/massimo/releases/download/<tag>/SHA256SUMS
(Get-FileHash massimo-windows-amd64.exe -Algorithm SHA256).Hash.ToLower()
Select-String massimo-windows-amd64.exe SHA256SUMS
.\massimo-windows-amd64.exe serve -listen 127.0.0.1:8099
```

The two hashes must match.
With the `.env` from above in the same folder, open http://127.0.0.1:8099/.
The binary is not code-signed.
A download with `curl.exe` starts without a word;
one made with a browser carries the mark of the web,
and a double-click on it in Explorer meets SmartScreen's "Windows protected your
PC" dialog with "Unknown publisher"
([screenshot](docs/images/windows-smartscreen-unknown-publisher.png)):
"Run anyway" runs it, and starting it from a console shows no dialog at all.
The installed service and `massimo update` are not affected ([the test
report](docs/reports/windows-test-2026-11-05.md), observation 8).
For the proxy and discovery,
Windows Defender Firewall must let the program receive on TCP 62910, UDP
23272 and UDP 5353 on the private network (the installed service gets a rule for that,
a `serve` started by hand does not),
and the original MAX! software must not be running.

**As a service.**
In PowerShell opened with "Run as administrator", in that folder:

```powershell
.\massimo-windows-amd64.exe install -dry-run -settings .env
.\massimo-windows-amd64.exe install -settings .env
```

installs the binary as `C:\Program Files\Massimo\massimo.exe`,
the settings as `C:\ProgramData\Massimo\env` (without `-settings` the template,
and the first start is then the owner's, as on Linux)
with the logs `serve.log` and `service.log` next to them,
in a folder only SYSTEM and the Administrators can read,
the service `massimo` ("Massimo" in the Services console: automatic start,
LocalSystem, restarted after 10 s on failure, listening on `:8099`;
`-listen` changes that and later installs keep it) and the firewall rule
"Massimo" for the program on private and domain networks.
On a network Windows classes as public the rule does not apply and other devices
get no answer: `Get-NetConnectionProfile` shows the profile,
`Set-NetConnectionProfile` makes a home network private;
check it again after a reboot, which can match the network to another,
public profile.

```powershell
sc.exe start massimo
sc.exe stop massimo
sc.exe query massimo
Get-Content C:\ProgramData\Massimo\serve.log -Tail 20
.\massimo-windows-amd64.exe install -remove          # -purge: the settings and logs too
```

Details: [Windows](packaging/README.md#windows), and the test on a real
PC in [docs/reports/windows-test-2026-11-05.md](docs/reports/windows-test-2026-11-05.md).

**Update.**
An installed service is updated with the installed binary,
again in PowerShell opened with "Run as administrator":

```powershell
& "C:\Program Files\Massimo\massimo.exe" update -check
& "C:\Program Files\Massimo\massimo.exe" update -restart massimo -health http://127.0.0.1:8099/healthz
```

`-check` only says whether a newer signed release is out on the update channel.
Without it, `update` asks before replacing anything (`-yes` skips the question),
downloads the release's Windows binary and verifies it against the signed manifest,
test-runs it,
renames the running `C:\Program Files\Massimo\massimo.exe` to `massimo.exe.prev`,
puts the new one in its place,
and stops and starts the service `massimo` through the service manager.
With `-health` it then waits up to two minutes (`-health-timeout`) until
`/healthz` reports the new version,
and otherwise puts `massimo.exe.prev` back and keeps the failed binary as
`massimo.exe.failed`;
a binary that cannot run as a service is given up by the service manager after 30 s,
so that case ends after about 35 s.
With another `-listen` address give it in `-health`;
over `https://` the certificate is verified unless the host is the loopback
or one of the box's own addresses.
`-version <tag>` installs a given release, older ones too.

```powershell
& "C:\Program Files\Massimo\massimo.exe" update -rollback -restart massimo
Get-Content C:\ProgramData\Massimo\service.log -Tail 10
sc.exe query massimo
```

`-rollback` swaps `massimo.exe` and `massimo.exe.prev` back and restarts the
service (once more goes forward again).
The result is in the command's own output,
in `C:\ProgramData\Massimo\service.log` ("stop requested", "serve stopped",
"stopped", "started as a service", "running",
and any error that ended `serve`), and,
for the service manager's view,
in
`Get-WinEvent -FilterHashtable @{LogName='System'; ProviderName='Service Control Manager'} -MaxEvents 10`.
Running `install` again from a newer downloaded binary also replaces the binary
and keeps the settings.
The Update button under System settings → Software works as on Linux (with
`MASSIMO_API_TOKEN` set): the server starts the same `update` in the background,
which stops and starts the service,
rolls back when the new version does not come up,
and logs to `C:\ProgramData\Massimo\update.log`
([docs/update-windows.md](docs/update-windows.md)).
Details: [Windows](packaging/README.md#windows)
and [Updating](packaging/README.md#updating) in packaging/README.md.

### macOS

**Try it.**
In Terminal, with `massimo-darwin-arm64` on Apple silicon
or `massimo-darwin-amd64` on an Intel Mac:

```sh
curl -fLO https://github.com/metaworx/massimo/releases/download/<tag>/massimo-darwin-arm64
curl -fLO https://github.com/metaworx/massimo/releases/download/<tag>/SHA256SUMS
shasum -a 256 massimo-darwin-arm64 && grep massimo-darwin-arm64 SHA256SUMS
chmod +x massimo-darwin-arm64
./massimo-darwin-arm64 serve -listen 127.0.0.1:8099
```

The two hashes must match.
With the `.env` from above in the same folder, open http://127.0.0.1:8099/.
The binaries are ad-hoc signed, not notarized:
a download with `curl` runs as it is,
one through a browser carries the quarantine attribute and Gatekeeper refuses it
until `xattr -d com.apple.quarantine massimo-darwin-arm64`.
With the application firewall on,
the binary must be allowed to accept incoming connections for the proxy
and discovery (install does that for the service).

**As a service.**
As root, in that folder:

```sh
sudo ./massimo-darwin-arm64 install -dry-run -settings .env
sudo ./massimo-darwin-arm64 install -settings .env
```

installs the Linux layout: the binary `/usr/local/bin/massimo`,
the hidden user `_massimo`,
the settings `/etc/massimo/env` (without `-settings` the template,
and the first start is then the owner's, as on Linux),
the state `/var/lib/massimo`,
the log `/var/log/massimo/serve.log` (JSON, rotated by `serve`,
readable by the group `admin`),
and in `/Library/LaunchDaemons` the daemon `ch.metaworx.massimo` (started at
boot without a login, restarted 10 s after a failure, listening on `:8099`;
`-listen` changes that and later installs keep it) and the update helper
`ch.metaworx.massimo-update`.
With the application firewall on, install allows the binary.
Control it with `launchctl`, all with `sudo`:

```sh
launchctl print system/ch.metaworx.massimo                                   # state, PID, last exit
launchctl kickstart -k system/ch.metaworx.massimo                            # restart
launchctl bootout system/ch.metaworx.massimo                                 # stop until the next boot
launchctl bootstrap system /Library/LaunchDaemons/ch.metaworx.massimo.plist  # start again
tail -f /var/log/massimo/serve.log
massimo update -restart ch.metaworx.massimo -health http://127.0.0.1:8099/healthz
massimo install -remove                                                      # -purge: the settings, state, logs and user too
```

The button under System settings → Software in the web UI works as on Linux,
through the helper (its log is `/var/log/massimo-update.log`).
Details: [macOS](packaging/README.md#macos).

**With Homebrew.**
`brew install metaworx/massimo/massimo-alpha` builds the release from source
(nothing needs notarization), `brew services start massimo-alpha` runs it as you
(`sudo` for a daemon at boot),
and the settings are in `$(brew --prefix)/etc/massimo/env`;
brew then owns the binary and `brew upgrade` updates it,
which the web UI's Update runs as you under `brew services`
(by hand, `brew services restart massimo-alpha` then runs the new one),
while `massimo update` says when a release is out.
The rolling channels' formulae are in a tap of their own,
`brew install metaworx/massimo-rolling/massimo-devel` or `massimo-nightly`.
Details: [Homebrew](packaging/README.md#homebrew).

### From source

<!-- BEGIN GENERATED PARTIAL docs/fragments/_build.md - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->
Every release can be built from its signed tag,
with git, make and Go 1.26 or later (<https://go.dev/dl/>):

```bash
git clone https://github.com/metaworx/massimo.git
cd massimo
git -c gpg.ssh.allowedSignersFile=/dev/null verify-tag <tag>
git checkout <tag>
make
```

`verify-tag` checks the tag's signature.
With no signer it trusts (`/dev/null`), git prints the key's fingerprint,
then `No principal matched.`, and exits 1;
the fingerprint must be `SHA256:R3Ihevc2jMppLJtKIhmpfRmulpf7DXuWBFbCjaU3qcs`,
which the guidelines' README names as well, under "Verifying a release",
<https://gitlab.com/metaworx/open-source/mwx-guidelines>,
on a host this repository does not control.
The key itself is deliberately not given here:
whoever could change the tags could change the line naming it.

`make` runs the tests and builds `./massimo` for this machine,
`make build` without the tests;
`make linux-armv6` builds for a Raspberry Pi 1, `make linux-arm64` for a 64-bit
Raspberry Pi OS, `make windows` for Windows,
and `make cross` every release binary, all into `dist/`.
The binary goes in as a release's does,
`sudo ./massimo install -dry-run -settings .env`, then without `-dry-run`.
It follows the tag's flavour, `alpha` for an `-alpha.N` tag,
so the web UI and `massimo update` say when a release is out,
and they install the project's binary, checked against its signed manifest;
to stay on your own builds, build the next tag and run `install` again,
which replaces the binary and keeps the settings.
A tag up to v0.12.0-alpha.1 follows `HEAD` instead, which has no releases,
unless built with `make CHANNEL=alpha`;
a build of anything but a release tag is `devel`,
and follows the `devel` builds.
<!-- END GENERATED PARTIAL docs/fragments/_build.md -->

The other targets:
`make linux`, `make linux-armv7` and `make darwin` (both Mac architectures)
build one more release binary each;
`make build-release` adds the build record, `SHA256SUMS` and the release manifest;
`make vet`, `make test` and `make clean` do what they say;
`make race` runs the race detector on the packages whose goroutines meet,
and `make fuzz` each fuzz target for `FUZZTIME` (10 s).
A build from `main`, or any other branch, is of the flavour `devel`
and follows the `devel` builds,
unless `MASSIMO_UPDATE_CHANNEL` or the setting `update.channel` names another channel,
see [packaging/README.md](packaging/README.md#updating).

## Server

```bash
massimo serve -listen 127.0.0.1:8099
```

keeps every Cube from the environment connected (one connection and one command
queue per Cube) and serves a JSON API: `/api/v1/cubes`,
`/api/v1/cubes/{serial}` (with rooms;
`PUT` and `DELETE` change and remove the Cube, `POST /api/v1/cubes` adds one),
`/api/v1/settings` (the stale threshold, the intervals and the retentions),
`/devices`, `/devices/{rf}`,
`PUT …/devices/{rf}/setpoint` and `PUT …/rooms/{id}/setpoint` with
`{"mode":"manual","temp":21.5}`,
`{"mode":"auto"}` or `{"mode":"vacation","temp":17,"until":"2026-11-10 12:00"}`,
`PUT …/devices/{rf}/lock` with `{"locked":false}` for the key lock,
`/backup?format=yaml`,
`POST …/restore/plan` and `POST …/restore` to restore a backup the way
`massimo restore` does (plan first, then send;
`GET …/restore` for the progress), and `/events` as server-sent events;
see [docs/api-v1.md](docs/api-v1.md) and the OpenAPI description in
[docs/api-v1-openapi.yaml](docs/api-v1-openapi.yaml).
Who may do what: users with roles in the store, each request carrying a
session from `POST /api/v1/login` or an API key as `Authorization: Bearer`,
the role granting the capabilities a route needs (setpoints, configuration,
pairing, users, …) and `GET /api/v1/me` saying which;
the audit (`GET /api/v1/audit`) records every request that changes something
and the commands (`GET /api/v1/commands/{id}`) its steps on the Cube's queue,
with `Idempotency-Key` answering a retry without a second radio command.
`MASSIMO_API_TOKEN` (or `-token`) becomes the owner's key at the first start,
and is the one token without a store (`serve -addr`);
while there are no users the API is open to the local network only,
and the first owner is created with `massimo user add` or by the web UI from
the local network.
See docs/api-v1.md, "Authentication".
The web UI's settings and colour schemes follow a signed-in user with the
capability `preferences` to every browser (`/api/v1/me/preferences`),
each with a switch to keep it on one device; a guest's stay in its browser.
A web UI, plain HTML and JavaScript embedded in the binary,
is served at http://127.0.0.1:8099/ :
one tile per room with the measured and the set temperature with a thermometer
and a gauge icon, up/down chevrons that send two seconds after the last click,
AUTO,
MANU and HOLIDAY plus an info and a cog button for the device list and the configuration,
a House tile whose ECO, AUTO,
COMFORT and MANU open a one-screen panel with mode, duration and room selection,
vacation per room inline, the device list with coloured mode badges,
offsets and locks, live updates plus polling with a refresh button,
the Cube clock in the browser's or the Cube's time zone,
and a settings page with an Expert switch for the technical details, Rooms,
Log and Settings as icon buttons at the right of the header,
a per-room configuration area (name, temperatures, boost and descaling,
device names and offsets, moves,
the week programme with a day editor that edits the days sharing a programme together,
copy of rooms' settings to other rooms), a help button next to every setting,
and a log view of the Cube's events.
The page speaks English, German, French, Italian,
Dutch and Polish (from the browser, or chosen in the settings),
from the translation files in `internal/server/ui/i18n/`, one per language.
[docs/ui.md](docs/ui.md) describes how to use it.
It is meant for the LAN.
The listener defaults to localhost and plain HTTP.
`-tls files -tls-cert <pem> -tls-key <pem>` (or `MASSIMO_TLS_CERT`
and `MASSIMO_TLS_KEY` in the settings) serves the API and the UI over TLS from
any certificate,
the chain after the server's certificate as every ACME client writes it;
the files are re-read when they change,
so a renewal by certbot or acme.sh needs no restart,
only a deploy hook that copies the files where the service user can read them
(`massimo install -tls-cert … -tls-key …` does that once,
see [packaging/README.md](packaging/README.md)).
`-tls self-signed` needs no certificate:
the server makes one at its first start for the box's names (its hostname,
`<hostname>.local`, `localhost`, its addresses,
and `-tls-names` for a name you gave it) into the data directory's `tls/`,
valid 825 days and renewed 30 days before the end with the names it has;
every browser warns once per device:
compare the fingerprint in its certificate details with the one in the log,
in `massimo config` or in `massimo tls show` on the box,
before signing in, as the page asks (`/healthz` carries it too,
but over the connection in question).
A box whose address changed gets a new one with `massimo tls self-signed`,
which a running server takes up at once.
`-tls acme -tls-names <name>` gets a certificate from Let's Encrypt for a
DNS name that resolves to the box, and renews it by itself:
the validation reaches the box on port 443 (a router forwards its public
443 to the server's port; the name's record is the public address,
dynamic DNS where it changes), by TLS-ALPN-01 on the listener itself,
or with `-tls-http :80` by HTTP-01 on a plain port 80 as well,
which then redirects to `https://<name>:443/`,
the port the router forwards, whatever `-listen` says.
`-acme-email` gives the account a contact address;
the account key and the certificates live in the data directory's `tls/acme/`.
A client that names no host, as one does for an address (`https://127.0.0.1`,
the update helper's health check among them),
gets the certificate of the first name.
Let's Encrypt allows five failed orders an hour per name,
so experiments use its staging directory
(`-acme-directory https://acme-staging-v02.api.letsencrypt.org/directory`,
which `massimo install` takes too and writes into the settings).
`-acme-dns-hook <script>` takes the DNS-01 challenge instead:
the server asks the script to set and clear a TXT record under `_acme-challenge.<name>`,
so no port has to be open, the name may point at the box's LAN address,
and wildcards (`-tls-names '*.example.net,example.net'`) work;
`packaging/acme-dns-hooks/infomaniak.sh` and `cloudflare.sh` are the scripts
for a zone at Infomaniak or at Cloudflare (they need `curl` and `jq`);
for any other provider an ACME client that knows its API,
such as acme.sh, lego or certbot, writes the files of `-tls files` instead
(see [packaging/README.md](packaging/README.md)).
A certificate that ends within two weeks, or an order that has not succeeded,
is a warning in the log, in `/healthz` and under the page's header.
With TLS the server tells browsers to use `https://` for its host for a year
(`-hsts <days>`);
a browser that saw that refuses `http://` until the days are up,
so before switching TLS off, run the server once with `-hsts 0` over TLS,
which clears it.
A reverse proxy with TLS in front works as before and sets its own HSTS;
the UI needs the same token as the API either way.
`-trusted-proxies <addresses or CIDRs>` tells the server which peers'
`X-Forwarded-For` to believe,
so the lockout and the first-owner form see the real client,
and the page works under a path prefix such as `/massimo/`
([docs/reverse-proxy.md](docs/reverse-proxy.md), with nginx and Caddy samples).
`-log <file>` (or `MASSIMO_LOG`) appends the log,
including every device state change, to a file as well,
one JSON object a line, made 0640;
stderr, and so the journal, stays plain `key=value`.
`-log-format json|plain` (or `MASSIMO_LOG_FORMAT`) sets both,
and `-log-file-format` and `-log-process-format` (`MASSIMO_LOG_FILE_FORMAT`,
`MASSIMO_LOG_PROCESS_FORMAT`) win over it for the file and for stderr;
`-log-process-format none` writes nothing on stderr.
`serve` rotates the file itself, on every platform:
once it would grow past `-log-rotate-size` (10M),
at once when its last line is in another format,
and on SIGHUP and a reload from the API.
`serve.log.1` is the newest rotated file, the older ones are gzipped,
and the oldest is deleted only while more than `-log-keep` (8) are kept and
the others still cover `-log-keep-min` (5M), so that many small files stay
(`MASSIMO_LOG_ROTATE_SIZE`, `MASSIMO_LOG_KEEP`, `MASSIMO_LOG_KEEP_MIN`).
The old file's last line says so, and the new file's first,
which is a line `log_rotated` in the web UI's log too.
Every line says whose it is, `source=` (`system`, `server`, `cube`,
`device`, `queue`, `proxy`, `store`, …), and `cube=` where it concerns one;
every event of the web UI's log is one line as well,
with its `kind=` and, for a command's, `command=`,
written from the same record as the log's row.
`-log-level debug` (or `MASSIMO_LOG_LEVEL`; `info`, the default,
`warn` and `error`) adds the Debug lines,
such as every failed attempt to reach a lost Cube
and every command the proxy passes on to it;
the level is read at the start,
and a change in the settings file waits for the next restart, as listed below.
Every other command takes `-log-level` before its name,
`massimo -log-level debug watch`.
A restart from the API (`POST /api/v1/restart`, for the capability `restart`)
stops the server as SIGTERM does and exits with status 75,
which systemd, launchd and Homebrew answer with a start;
a Windows service runs `massimo restart` instead,
which stops and starts the service through the service manager.
`-restart` (or `MASSIMO_RESTART`) says when it is offered:
`auto` where a service manager starts the server again, so not in a terminal;
`exit` always, exiting 75 for whatever runs `serve`,
such as Docker's `--restart`;
`off` never.
`sudo massimo restart` restarts the installed server from a prompt as well.
`-allow-reboot` (or `MASSIMO_REBOOT=on`; `-allow-reboot=off` to say no)
lets the owner reboot the computer
`serve` runs on from the API and the web UI (`POST /api/v1/reboot`,
for the capability `reboot`, which only `owner` has),
60 seconds after the request or after the delay it names, 0 to 3600:
under systemd and launchd through the root helper
that `sudo massimo install -reboot-helper` installs
(and `-no-reboot-helper` takes out again),
as the Windows service by itself,
and never in a terminal, in Docker or as Homebrew's service
([packaging/README.md](packaging/README.md), "Rebooting from the web UI").
The switch is the owner's, off by default:
`on`, `true`, `yes`, `1` or `enabled` switch it on,
`off`, `false`, `no`, `0`, `disabled` or nothing off, in any case;
another value stops `serve` at the start,
and switches the reboot off with a warning when the settings file is read again.
`serve` applies a change of `MASSIMO_REBOOT` in the settings file at once,
unless `-allow-reboot` was given.
`serve` reads its settings file again while it runs:
it looks at it every five seconds,
and reads it once a change has held for one more look,
and on Unix also on SIGHUP (`systemctl reload massimo`),
which checkpoints the store too;
`POST /api/v1/reload` does what SIGHUP does, on every system
(System settings → Maintenance → **Reload now** in the web UI).
`MASSIMO_API_TOKEN` applies at once, as at a start, unless `-token` was given;
every other setting that changed is listed by `GET /api/v1/restart`
as applying at the next restart.
`-sighup` (or `MASSIMO_SIGHUP`) says what SIGHUP does:
`reload`, `stop` as SIGTERM does,
or `auto`, a reload under a service manager or after `nohup`,
and a stop in a terminal, where SIGHUP means that the terminal closed.
Every six hours (`-refresh`,
`0` to switch it off) `serve` reconnects to each Cube,
in turn with the queued commands, and re-reads everything the Cube holds,
so a write the Cube acknowledged but did not apply cannot leave the server's
copy wrong for long.
After a configuration write it reads the device's configuration again for about
six minutes until the device has taken it,
and after radio writes it reconnects once for the Cube's budget if radiograms
were pending, at most every 15 minutes (only while `-refresh` is on).
Writes that a client of the proxy (below) sends,
such as the original software's, are followed up the same way.
The log lists every wakeup and every radio command the Cube accepted,
with its duty cycle and free slots.
`serve` asks each Cube for its device list every minute (`-poll`),
and between these polls the connection's TCP keepalive probes the Cube.
A Cube that stops answering, as one whose network cable is out, is lost
once it leaves two polls in a row unanswered,
within two polls and ten seconds,
and on Linux already when three keepalive probes go unanswered,
about 30 seconds after the last traffic:
`serve` logs `cube connection lost`,
and its proxy stops answering discovery for it ([docs/proxy.md](docs/proxy.md)).
`serve` holds the lost connection open for 45 minutes (`-cube-hold`,
0 not at all), since a Cube keeps a session whose end it never saw
and refuses every other client meanwhile, for 41½ minutes on one occasion
([docs/cube-protocol.md](docs/cube-protocol.md),
"Lessons from the hardware days"):
after an outage of the network of the box running `serve`,
such as a router's restart,
the Cube answers on the held connection once the network is back,
and `serve` logs `cube connection re-established` with `held=true`.
The hold ends earlier when the Cube closes the connection,
when discovery finds the Cube at another address,
or when the Cube has answered discovery for three minutes without the connection resuming;
`serve` logs `cube held connection ended` and connects anew,
at most a minute apart, until the Cube answers.

The Cube serves only one LAN client at a time,
so `serve` is itself a Cube for everyone else:
it listens on port 62910 on every interface (`-proxy <addr>[:<port>]`,
repeatable, `-proxy off` to disable) and answers UDP discovery.
A listener that cannot listen,
on an address the host does not carry yet or a port in use,
is tried again, at most 30 seconds apart, until it listens.
Every other massimo command and the original software connect to the host
running `serve` (`-addr <serve host>`) and work beside it:
reads are answered from the server's copy,
writes go through its queue to the Cube.
The original software keeps only one discovery answer per serial,
and the real Cube answers first,
so give the server a different serial for discovery with `-proxy-serial`,
for example the real one with the first letter changed,
or block the Cube's UDP answers on the PC's firewall.
See [docs/proxy.md](docs/proxy.md).

`serve` announces itself on the LAN by mDNS (UDP port 5353) as `_massimo._tcp`,
so that `massimo discover`, the apps and any DNS-SD browser find it unasked,
such as `avahi-browse -r _massimo._tcp` or `dns-sd -B _massimo._tcp`:
the instance is the box's host name,
the SRV record its API's port on the host `massimo-<node>.local`,
and the TXT record its node (`node`), version (`v`),
whether it serves TLS (`tls`) and the certificate's SHA-256 (`fp`),
which an app compares before it trusts a self-signed certificate.
It is on by default where `-listen` is reachable from the LAN,
and off on a loopback address;
`-announce` (or `MASSIMO_ANNOUNCE`) switches it,
taking the same values as `MASSIMO_REBOOT`, and `-addr` announces nothing.
A switch alone is on, and its value goes after `=`, as `-announce=off`:
`serve` refuses `-announce off`, which would set it on and leave `off` and the
flags after it unread.
It shares port 5353 with the system's own responder (avahi, mDNSResponder,
Windows' mDNS);
where multicast is missing, as in a container without it,
`serve` logs that it is not announced and runs on.

### Screenshots

The web UI against the fake Cube, light and dark,
and a room's tile with its week programme:

![The Rooms page, light](docs/screenshots/rooms-1100-light-en.png)

![The Rooms page, dark](docs/screenshots/rooms-1100-dark-en.png)

![A room's tile with its week programme](docs/screenshots/config-week-1100-light-en.png)

The whole set is in [docs/screenshots/](docs/screenshots/),
taken by `make ui-screenshots` (tools/ui-test/README.md),
and as a slideshow:

![The web UI, page by page](docs/screenshots/slideshow.gif)

## Command line

`massimo help` lists the commands; the ones useful today:

```bash
massimo backup -addr <cube-ip> [-out backup.yaml]
```

saves rooms, device names, thermostat settings, week programs, NTP servers
and time zone rules, together with the raw protocol lines, as JSON or YAML.

```bash
massimo restore -addr <cube-ip> -from backup.yaml -dry-run
```

compares a backup (which may be edited by hand) with the Cube and shows what
would be written;
without `-dry-run` it asks and then sends only the differences,
respecting the radio duty cycle.
Each device's live state (mode, setpoint, measured temperature,
valve) is in the file for information;
`-state` re-applies mode and setpoint too.
Pairing cannot be restored over the LAN:
devices missing from the Cube are listed,
and the restore can be repeated after they have been paired again.

```bash
massimo status
massimo watch -addr <cube-ip>
massimo fakecube -greeting internal/protocol/cube/testdata/cube-fw0113-greeting.txt
```

`status` lists every device the Cube knows, grouped by room,
with its last state and whether the Cube holds state for it at all:
after a Cube restart every device is "not reported" until it transmits again,
window contacts only on a window change,
and the Cube discards room commands while a room has such a member,
so the room is marked not ready (`-json` for machines).
`watch` prints state changes as they happen;
`fakecube` serves recorded data on port 62910 so everything can be tried without hardware,
and with `-discovery 127.0.0.1:23273` it also answers UDP discovery as a further
Cube (`-discovery-serial`),
which `serve -discover-target 127.0.0.1:23273` finds.
`-pair 0a010d,0abcde,TEQ0000001` adds a device that joins pairing mode two seconds
after `n:`, a copy of the greeting's 0a010d as 0abcde in no room,
and joins again after it was removed (a fourth field sets the delay).

The fake carries out the radio commands it acknowledges, as the devices do:
a setpoint or mode (to every thermostat of the room for a room command),
a boost, which ends by itself after the device's boost time (`-boost-for` to
shorten it), a day's programme, the preset temperatures, the valve settings,
the key lock and a wall thermostat's display;
the device list and the C lines then show the change,
also in a new connection's greeting.
`-apply-after 30s` applies each command that long after the S reply,
as a device confirms it after its wakeup,
and `-apply=false` acknowledges only, as the fake did before.
`-fixture rooms` serves a built-in house instead of `-greeting`:
a room with a wall thermostat and two radiator thermostats,
one with three radiator thermostats at different temperatures,
and one with a shutter contact alone.
`-control 127.0.0.1:62911` serves a control over HTTP, on loopback only,
for tests that drive the fake from outside:
`PUT /devices/<rf>/state` with JSON such as `{"actual_temp": 18.5, "battery_low": true}`
(also `mode`, `setpoint`, `valve_pct`, `window_open`, `error`, `link_error`,
`panel_locked`, and `reported: false` for a device the Cube holds no state for),
`POST /devices/<rf>/remove` and `/restore` to take a device out of its room's
lists and bring it back, `POST /drop` to make the clients connect again and
read the greeting, and `GET /devices` for the device list;
`POST /silent` and `DELETE /silent` pull the Cube's cable and put it back
(the fake keeps its connections and holds every line meanwhile),
`POST /release` forgets a dead session (below),
and `GET /connections` counts the connections so far and open now.
`-single-client` serves one client at a time, as the Cube does:
while a client is connected, another client's dial times out (on Linux;
elsewhere its connection is closed at once).
A client that leaves while the fake is silent is a session whose end the outage lost,
and the fake holds it for `-hold-dead` (41 minutes by default,
as a Cube did on 2026-11-13), refusing every client meanwhile,
until `POST /release` or an `R` to `-discovery`, which restarts the fake.

```bash
massimo discover
```

`discover` lists the Cubes on the LAN (serial, IP, MAC, RF address, firmware,
the gateway it uses, and whether the Cube takes its address by DHCP or has a
static one) via the UDP broadcast the original software uses,
marking `proxy` a massimo serve that answers as a Cube:
it answers neither `N` nor `c`, and its MAC is not eQ-3's `00:1a:22`
or its address is an announced server's,
and then the massimo servers that announce themselves by mDNS (name, address,
TLS, version, node; `-json` prints both lists);
it needs UDP port 23272 and a real LAN interface (not WSL 2).

The portal commands, `portal` (a fake portal server that decrypted what a Cube sends)
and `portal-crypto` (the AES helpers for the Cube's portal encryption),
are gone: the Cube's firmware was not extracted,
so the key from its QR code serves nothing.
They are kept on the branch `archive/portal`.

```bash
massimo netconfig -addr <cube-ip>
massimo netconfig -addr <cube-ip> -static <ip> -gateway <gw> -netmask <mask> [-dns <a>[,<b>]]
massimo netconfig -addr <cube-ip> -dhcp
massimo netconfig -sn <serial> -broadcast -dhcp
```

`netconfig` reads a Cube's network configuration over UDP 23272 (`-json` too):
the addresses in use, and the stored static configuration,
which with DHCP on is only the fallback.
It writes a static address with DHCP off, or DHCP on.
A write shows the current and the new values and asks first (`-yes` skips that),
checks that the ip and the gateway lie in the netmask's subnet,
keeps the Cube's encryption flag as read,
and re-reads the result after the Cube's restart (a static address applies at
once and is followed by a restart, as the original software does;
DHCP makes the Cube restart by itself);
`-dry-run` prints the frame and sends nothing.
A wrong configuration strands the Cube on an address nothing can reach.
The way back is `netconfig -sn <serial> -broadcast -dhcp`,
broadcasts that only the Cube with that serial takes:
it reads the stored configuration first and changes only the DHCP flag, or,
when the read gets no answer, sends zeros for the addresses,
which loses the stored fallback (192.168.0.222/16 from the factory) without restoring it;
a Cube that moved networks may need a power cycle.

### Cube variables

The Cubes are configured in the environment or in `.env` in the current
directory (`KEY=VALUE` lines,
`-env <file>` or `MASSIMO_ENV_FILE` for another file,
`-no-env` for none;
the environment wins over the file).
On Linux and macOS the service user, the owner of `/etc/massimo/env`,
reads that file alone and keeps its data in `/var/lib/massimo`,
from any directory, as `sudo sshd` reads sshd's own configuration;
`-env`, `-no-env` or `MASSIMO_ENV_FILE` turns both off.
Keep the file out of git; `massimo config` shows what is in effect,
secrets redacted.

`serve` keeps the Cubes in its own store instead,
`massimo.db` in the data directory (`-data <dir>` before the command name,
or `MASSIMO_DATA`; else the current directory,
so a terminal run puts it next to `.env`, or the service user's;
the installed services pass the state directory,
see [packaging/README.md](packaging/README.md#the-store)).
At every start `serve` adds to the store each Cube of the file it does not know yet,
logged as `cube added from the settings file`;
that is how the first start after the update takes the file over,
and how a Cube added to the file by hand later is still picked up.
From then on the store wins for that Cube: `_ADDRESS`, `_NAME`, `_PROXY`,
`_PROXY_SERIAL` and `_IGNORE` are read from the store,
and a line of the file that differs is logged at the start
(`settings file differs from the store, the store wins`,
with both values) and shown on the Cube in `GET /api/v1/cubes` (`file_differs`)
and under System settings → Cubes in the web UI, which offers the file's value.
That page, and `POST /api/v1/cubes`, `PUT` and `DELETE /api/v1/cubes/{serial}`,
add a Cube, change its name, address, proxy and hidden devices,
and remove it while `serve` runs.
`massimo config` prints, per Cube,
which value is the store's and which line of the file differs.
`MASSIMO_API_TOKEN`, `MASSIMO_LISTEN`,
`MASSIMO_LOG`, `MASSIMO_LOG_LEVEL`, `MASSIMO_LOG_FORMAT` and the other `MASSIMO_LOG_…`,
the TLS and update variables and `MASSIMO_CUBE_SN` stay in the file only,
and every command other than `serve` keeps reading all of its lines as before.
`-addr` and `-sn` still pick a Cube for one run and write nothing for it;
`serve -addr` runs without the store altogether (no `massimo.db` is opened or created,
found Cubes are not kept).
`massimo store backup <file>` writes a consistent copy of the store,
also while `serve` runs (copying `massimo.db` alone can miss the last writes,
which are in `massimo.db-wal` until a checkpoint,
at the latest at a clean stop),
`store migrate` creates or migrates it without starting `serve`,
and `store info` prints its schema version, node and row counts
(also in the web UI, System settings → Maintenance → Store, as
`massimo config`'s settings are under Settings in effect, secrets left out).
The copy holds the database alone:
`opaque.key` and `node.key` beside it are kept with it,
since without `opaque.key` no password of `auth_version` 2 signs in
and no sign-in provider's client secret opens (packaging/README.md,
"The store").
Run as the service user they act on the service's store from any directory,
for example `sudo -u _massimo massimo store info`.

`massimo user` manages the users of the store from the box (docs/api-v1.md,
"Authentication"):
`user add <name> -role operator` asks for the password twice (`-no-password` for
a user with keys only,
`-password-stdin` to read it from standard input for scripts,
`-expires <RFC 3339>` for a guest's end), `user list`, `user passwd <name>`,
`user rename <name> <new name>`,
which asks for the password again since it goes with the name
(`-no-password` to leave the user with its keys),
`user disable <name>` and `user enable <name>`,
`user token <name> -label Loxone` prints a new API key once,
`user tokens <name>` lists the user's sessions and keys with their ids,
and `user revoke <name> [<token id>]` revokes one token or every one.
A password set with `user add` or `user passwd` is of `auth_version` 2:
the command registers it by OPAQUE in its own process,
with the browser's key stretching, so the web UI signs in with it and the
server keeps no hash (docs/store.md section 6).
A script signs in with an API key, not a password.
It opens the store beside a running `serve`;
run as the service user it acts on the service's,
for example `sudo -u _massimo massimo user add anna -role operator`.
Run by anyone else without `-data` or `MASSIMO_DATA`,
it uses a store in the current directory and creates none there:
it refuses, and names the service's command where the service is installed,
since a store of its own would be nobody's and `serve` would never see the change;
`store migrate` refuses the same.
Each change ends its line with the store it was written to,
and a store other than the service's draws a warning on the box that runs the service.
The audit records what it did with the remote `cli`,
and the log a change line of each, by the command line (docs/ui.md, "Log").
With `MASSIMO_API_TOKEN` set, the first start of `serve` creates the user
`owner` with the token as its API key, so the token keeps working;
without it, the first owner is created with `user add <name> -role owner`
or with the form the web UI offers on the local network while there are no users.

Users may also sign in through an OpenID Connect provider beside their password,
such as Nextcloud, Google or Microsoft:
the owner adds the provider in System settings → Sign-in providers
and assigns users their e-mail in System settings → Users;
what to register at the provider is in docs/oidc.md.

Every variable has a per-Cube form with the serial in it,
`MASSIMO_CUBE_<SERIAL>_ADDRESS` for `MASSIMO_CUBE_ADDRESS`, which wins for that Cube;
the plain form applies to any Cube unless the table says otherwise.

| Variable (per-Cube form) | Meaning |
|---|---|
| `MASSIMO_CUBE_<SERIAL>_ADDRESS` | The Cube's address, host[:port]. `serve` serves every Cube that has one. |
| `MASSIMO_CUBE_<SERIAL>_PROXY` | The address of a running `massimo serve` that answers as this Cube. Every command except `serve` connects to it instead of the Cube, because the Cube serves one client at a time and `serve` usually holds that connection; `serve` listens on it. When the proxy does not answer within two seconds, the command prints a warning and connects to the Cube's own address: during the few seconds `serve` restarts, for example after an update, that direct connection takes the Cube's only slot and keeps `serve` from reconnecting until the command ends. Before the command name, `-no-direct` (`massimo -no-direct status`, for scripts) makes that an error instead, and `-direct` skips the proxy and connects to the Cube itself even when the proxy answers; the two exclude each other, and `-addr` always wins. The plain form only with a single Cube. |
| `MASSIMO_CUBE_<SERIAL>_PROXY_SERIAL` | The serial `serve` uses in UDP discovery replies for this Cube, see [docs/proxy.md](docs/proxy.md). The plain form only with a single Cube. |
| `MASSIMO_CUBE_<SERIAL>_IGNORE` | Radio addresses to hide, comma separated, in addition to room 255 (below). |
| `MASSIMO_CUBE_<SERIAL>_NAME` | A user-friendly name the API and the UI show next to the serial. The plain form only with a single Cube. |
| `MASSIMO_CUBE_SN` | The Cube to use when no `-sn` is given. |
| `MASSIMO_DATA` | The data directory, where `serve` keeps `massimo.db` (also `-data <dir>` before the command name); default the current directory, `/var/lib/massimo` for the service user. |

`-sn <serial>` selects a Cube:
every command then reads that Cube's variables and, without an address,
discovers the Cube on the LAN; with `-addr` instead,
the serial the Cube reports selects them.
`-addr` always wins over the variables.
`serve -sn <serial>` serves that one Cube instead of every configured one.

`serve` serves every Cube its store has as configured.
From the file it takes over every Cube with an `_ADDRESS`, and also, by serial,
every Cube with a `_NAME` or a `_PROXY` but no `_ADDRESS`;
the one given with `-sn` or `MASSIMO_CUBE_SN` is served for the run when nothing
else is configured.
A Cube without an address is connected at the address it answers UDP discovery from,
and reconnected when that changes.
A key alone does not make `serve` take a Cube.
A lone `MASSIMO_CUBE_ADDRESS` without a serial is served from the file until the
Cube reports its serial, and then added to the store.
Beside the variables of one Cube, such as its `_PROXY`,
the plain `MASSIMO_CUBE_ADDRESS` is that Cube's when `MASSIMO_CUBE_SN` names it,
as for the other commands.
`serve` runs discovery at the start and every 30 minutes (`-discover-interval`,
`0` for only on request through the API,
but for a Cube it can only connect through discovery,
`-discover-target <host>` to ask given addresses instead of broadcasting,
`-discover-listen <addr>` for a socket on another address than UDP 23272,
which only a test's fake Cube answers to),
lists Cubes it finds that are not configured in the API (`GET /api/v1/cubes`
with `"configured": false`),
keeps them in the store so that they are still listed after a restart,
and logs a configured Cube that answers from another address than configured.
A lost Cube is searched for at once and every minute until it is back,
one with a fixed address only where the interval is not `0`,
and followed to its new address where it follows discovery.
`-poll`, `-refresh` and `-discover-interval` win for one run over the store's
settings `poll_interval`, `refresh_interval` and `discover_interval`,
and `-stale-after` over `stale_after`,
also below the hour the API allows, so that a test sees a thermostat go stale.

Room 255 is the room for hidden devices:
a device that is paired but never reports blocks every command to the room the
Cube counts it in, and the Cube never drops it,
so the way out is to park it in room 255 on the Cube (edit a backup and restore it,
see docs/cube-protocol.md).
Every massimo command,
the API and the proxy then hide that room and its devices without any configuration,
so `status` on another computer and the original software do not see them either,
and `restore` keeps them exactly as the Cube has them.
The devices in `MASSIMO_CUBE_<SERIAL>_IGNORE` (for `serve`,
the Cube's hidden list in the store) are hidden in the same way.
`-show-hidden` before the command name (`serve -show-hidden` for the server;
`-show-ghosts` and `-no-ignore` are older names) shows and touches them for one run,
for example to move a device out again.

## Original software

```bash
massimo legacy -out max-session.log
```

starts the original MAX! software (Java 8, installation directory
`C:\ProgramData\MAX` by default, `-dir` otherwise) and prints only the
protocol-relevant lines: API and gateway calls, radiograms, send results,
and the Cube search (the interfaces it binds to, the adapters it tries,
how many gateways it found).
The software never prints the LAN lines it writes, not even at debug level,
so `legacy` reconstructs them from the logged gateway calls (`s:`, `t:`, `m:`,
`z:`, `c:`, `l:`, `n:`, `x:`, `q:`) and prints each beneath its call,
marked `lan (derived)`.
`-out` saves the complete output, `-all` shows it, `-width` shortens long lines.
With `serve` running, point the software at the serve host rather than the Cube:
in its Cube list pick the entry with the serve host's address (started with
`-proxy-serial`, it appears under that serial).

## Versions

Development happens on `main`.
`make release <flavour>` tags `main` as the next release of that flavour,
`alpha`, `beta`, `rc` or `stable` (also `make release FLAVOUR=<flavour>`,
or `make release-alpha` and its siblings),
which CI tests, builds, signs and publishes,
and then fast-forwards the branch `release/<flavour>` to it.
Before the release commit it dumps the store the release writes
into `internal/store/testdata/releases/` and commits it,
for the migrations' tests of every later binary (docs/store.md, "Migrations");
that needs go, curl, jq, python3 and sqlite3, and about a minute.
Everything below `v1.0.0`, and every version with a suffix (`-alpha.N`,
`-beta.N`, `-rc.N`), is a pre-release;
only a plain `vX.Y.Z` from `v1.0.0` on is stable.
Below `v1.0.0` a minor release may break things.
The version is the newest one any release has:
`make release alpha` counts the suffix up (`v0.2.0-alpha.2`),
`make release beta` and `make release rc` cut that version's next beta or rc,
and `make release stable` that version itself, `v0.2.0`.
A new feature or breaking change starts the next minor
(`make release alpha BUMP=minor` gives `v0.3.0-alpha.1`),
a fix of a stable release the next patch (`make release stable BUMP=patch`),
and `BUMP=major` the next major.
The next tag never sorts before one already out:
no alpha after that version's beta, no pre-release after its stable release
(`packaging/next-version.sh` says which `BUMP` starts the next one).
`make release` without a flavour stops,
and so it does with `VERSION=` or `RELEASE=`:
those are the build's, `make build-release`,
which `make release` was until 2026-11-12.

A release is made on `main`, checked out, clean and equal to `origin/main`.
`make release <flavour>` cuts `CHANGELOG.md`'s `[Unreleased]` into the release's
section with `GUIDELINES/shared/tools/changelog.sh`,
commits it on `main` as `chore(release): vX.Y.Z-beta.N`, and tags that commit,
signed, with the section as the tag's message,
so the tag carries the release notes, and the GitHub release shows them.
It then pushes `main` and the tag together or not at all;
CI's run of the tag waits for the whole suite before it releases
(`GUIDELINES/project/CI.md`, "Releases"),
and moves `release/<flavour>` once the release is out,
so that the branch names what that flavour's boxes are offered.
It refuses when `main`'s head is released already,
when `changelog.sh check` finds something,
and when `[Unreleased]` holds no bullet.
`make release <flavour> DRY=1` shows what it would cut, tag and push,
and does none of it;
`ASSISTED_BY=agent:model` gives an agent's release commit its `Assisted-by` trailer.

Every release has a flavour, from the least stable to the most:
`devel`, every commit of `main` whose tests passed;
`nightly`, once a day the newest of those;
`alpha`, the `-alpha.N` tags; `beta`, the `-beta.N` tags;
`rc`, the release candidates, `-rc.N`; and `stable`, a plain `vX.Y.Z`.
`devel` and `nightly` are one GitHub release each,
`release/devel` and `release/nightly`, replaced in place,
signed with a key of their own that boxes accept for those two alone.
Every flavour is also an apt suite and a Homebrew formula,
`massimo-alpha` to `massimo` for stable,
and `massimo-devel` and `massimo-nightly` in the tap `metaworx/massimo-rolling`,
each holding its own releases and every more stable flavour's.
A box follows an update channel, one of those names,
and takes its channel's releases and every more stable flavour's:
a box on `alpha` takes a later beta, rc or stable release too.
Its channel is `MASSIMO_UPDATE_CHANNEL`, else the setting `update.channel`,
else its build's flavour;
every channel's newest build is at `release/<channel>`,
a branch for the numbered flavours, a tag for the rolling ones.
The documentation of each channel's newest build is on the site,
<https://massimo.metaworx.ch/docs/>, with `main`'s beside it
(see [packaging/README.md](packaging/README.md#the-site)).
Boxes get each release from `massimo update` or the web UI
(see [packaging/README.md](packaging/README.md#updating)).
`make` builds carry the version from `git describe`, `go run` builds
report the commit only with `go run -buildvcs=true`.

## Legal

Licensed under the [GNU Affero General Public License v3.0 or later](LICENSE).
The privacy policy of the software, the apps and the project's site is
<https://massimo.metaworx.ch/privacy.html>.

<!-- BEGIN GENERATED PARTIAL docs/fragments/_trademark.md - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->
"MAX!" and "eQ-3" are trademarks of eQ-3 AG.
This project is not affiliated with or endorsed by eQ-3.
<!-- END GENERATED PARTIAL docs/fragments/_trademark.md -->
