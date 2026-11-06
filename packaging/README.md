# Running `massimo serve` as a service

`massimo serve` is meant to run all the time on a small always-on Linux box next
to the Cubes, such as a Raspberry Pi.
`massimo install` sets it up as a systemd service with the files of this directory,
which are compiled into the binary;
"Install by hand" below lists the same steps.
On Windows it installs a Windows service instead (see "Windows"),
on a Mac LaunchDaemons (see "macOS").

- `systemd/massimo.service`:
  the unit.
  Runs as the user `massimo`, reads its settings from `/etc/massimo/env`,
  listens on port 8099 on every interface (API and web UI;
  `install -listen` changes it),
  logs to `/var/log/massimo/serve.log` and the journal, and restarts on failure.
- `systemd/massimo-update.path` and `systemd/massimo-update.service`:
  the update helper,
  which installs a new release when the web UI or the API asks for one (see Updating).
- `logrotate/massimo`:
  weekly rotation of that log.
- `launchd/ch.metaworx.massimo.plist` and `launchd/ch.metaworx.massimo-update.plist`:
  the service and the update helper on macOS (see "macOS").
- `env.template`:
  the settings file every install starts from, embedded as well.
- `nfpm.yaml` and `deb/`:
  the Debian package `make packages` builds (see "Debian package").
- `apt/`:
  the apt repository the release workflow publishes (see "The apt repository").

The repository's `.gitattributes` keeps everything in this directory with LF line endings,
also in a Windows checkout with `core.autocrlf`,
so that the binary embeds the files byte for byte as reviewed.
A binary file added here needs its own `-text` line there.

Each Cube accepts exactly one TCP client.
Only one `massimo serve` can be connected to a Cube at a time,
so stop any other server (and close the original MAX! software) before starting this one;
afterwards, clients connect to the server's proxy instead of the Cube.

## Download or build

Every release on <https://github.com/metaworx/massimo/releases> has the binaries below,
`SHA256SUMS` and a signed manifest;
`sha256sum -c SHA256SUMS --ignore-missing` checks a download.
To build instead, pick the target for the board (all need a 32-bit or 64-bit
Linux to match):

| Board                                  | OS             | Target              |
|----------------------------------------|----------------|---------------------|
| Raspberry Pi 1 (A, B, B+), Zero, Zero W | 32-bit         | `make linux-armv6`  |
| Raspberry Pi 2, 3, 4, Zero 2 W         | 32-bit         | `make linux-armv7`  |
| Raspberry Pi 3, 4, 5, Zero 2 W         | 64-bit         | `make linux-arm64`  |
| PC, NAS                                | 64-bit         | `make linux`        |
| Mac (Intel / Apple silicon)            |                | `make darwin`       |
| Windows PC                             | 64-bit         | `make windows`      |

`uname -m` on the box says which: `armv6l` needs the armv6 build (the armv7
one dies there with "illegal instruction"), `armv7l` the armv7 build,
`aarch64` the arm64 build.

## Preparing a Raspberry Pi 1

The current Raspberry Pi OS Lite (32-bit) still runs on the Pi 1 and needs about 2.8 GB,
so a 4 GB card is enough.
Raspberry Pi Imager can preset the user, SSH key and hostname.
When writing the image by hand,
the boot partition's `user-data` (cloud-init) does the same on first boot,
for example:

```yaml
#cloud-config
hostname: massimo
users:
- name: pi
  groups: users,adm,sudo,netdev
  shell: /bin/bash
  sudo: ALL=(ALL) NOPASSWD:ALL
  ssh_authorized_keys:
  - ssh-ed25519 AAAA... you@host
enable_ssh: true
ssh_pwauth: false
```

The Pi 1 has no network over USB (its micro-USB port only powers it),
so it needs Ethernet.
The first boot takes about ten minutes.

## Install with `massimo install`

On the box, with the binary for it (from a release, or built and copied over),
as root:

```bash
curl -fLO https://github.com/metaworx/massimo/releases/latest/download/massimo-linux-armv6
chmod +x massimo-linux-armv6
sudo ./massimo-linux-armv6 install -dry-run
sudo ./massimo-linux-armv6 install
```

(`releases/latest` skips pre-releases; while there are only alphas,
take the URL from the releases page.)
It installs the binary as `/usr/local/bin/massimo`,
creates the system user `massimo` and `/etc/massimo/env` (a commented template,
or a copy of `-settings <.env>`), writes `massimo.service`,
the update helper `massimo-update.path` and `.service` and the log rotation,
enables them, and starts the service unless the settings are still the template.
It warns when a Cube's `_PROXY` address is not one of the box's addresses (serve
could not listen there and would restart every ten seconds;
install adds no addresses) and when `MASSIMO_API_TOKEN` is missing.
Fill in `/etc/massimo/env` as described below,
then `sudo systemctl start massimo`.

Running `install` again, from a newer binary,
replaces the binary and the units and restarts the service;
existing settings are kept,
and a unit that differs from the new one (edited with `systemctl edit --full`)
is kept as `<unit>.bak`.
Local changes belong in a drop-in (`systemctl edit massimo`),
which install never touches.
`-no-update-helper` leaves the helper out, `-no-start` only enables.

`-listen <addr>` moves the API and web UI off `:8099`,
for example to port 80 (`sudo massimo install -listen :80`,
or `192.168.1.200:80` for one address only).
It writes two drop-ins that later installs keep:
`massimo.service.d/listen.conf` sets `MASSIMO_LISTEN` and, below port 1024,
grants the service `CAP_NET_BIND_SERVICE`,
since the user `massimo` cannot bind such a port otherwise;
`massimo-update.service.d/listen.conf` sets `MASSIMO_HEALTH`,
where the update helper checks the restarted server.
Deleting both files (and `systemctl daemon-reload`,
`systemctl restart massimo`) goes back to `:8099`.

`massimo install -remove` stops and removes the service,
its units and the binary; `-purge` also removes the settings, the state,
the logs and the user.

## Debian package

Every release also carries a Debian package per architecture,
`massimo_<version>_amd64.deb`, `_arm64.deb` and `_armhf.deb`,
the release's version written as dpkg sorts it (`0.7.0~alpha.1` for `v0.7.0-alpha.1`).
The armhf one holds the armv6 binary and runs on every 32-bit Raspberry Pi OS,
the Pi 1 and the Zero included (Debian's own armhf needs a Pi 2 or later).
On a Debian, Ubuntu, Raspberry Pi OS or LoxBerry box:

```bash
curl -fLO https://github.com/metaworx/massimo/releases/download/<tag>/massimo_<version>_arm64.deb
sudo apt install ./massimo_<version>_arm64.deb
```

It installs the release binary as `/usr/bin/massimo`,
the unit as `/usr/lib/systemd/system/massimo.service` (the embedded one with
that path and without the update helper),
the log rotation and the marker `/usr/share/massimo/packaged`;
its scripts create the user `massimo` and `/etc/massimo/env` from the template
when they are missing, enable the service,
and start it once the file differs from the template,
so that a fresh box does not serve the LAN without a token.
Fill in `/etc/massimo/env` as "Install by hand" describes,
then `sudo systemctl start massimo`.
The next package upgrades the binary and restarts the service;
the settings and the store are left alone.

**One owner of the binary.**
On such a box apt owns `/usr/bin/massimo`, and the marker tells massimo so:
`massimo update` and the web UI's button still say when a newer release is out,
but leave the install to `sudo apt update && sudo apt install massimo`
(the apt repository below) or to the next release's package by hand.
`massimo install` keeps only what configures:
`-settings` over a settings file that is still the template,
`-listen` as the drop-ins of "Install with `massimo install`", then a restart.
`massimo install -remove` points at `apt remove massimo`;
`apt purge massimo` takes the settings, the store, the logs and the user too,
as `-remove -purge` does.
A box set up with `massimo install` keeps its own updater:
the package refuses to install over it until `sudo massimo install -remove` has run,
which keeps the settings and the store;
a `-listen` address is set again afterwards with `sudo massimo install -listen <addr>`.

The package is built by `make packages` (nfpm, at the version the Makefile pins,
through `go run`) from `nfpm.yaml`,
the scripts in `deb/` and the unit `deb/unit.sh` derives;
`make deb-test` installs,
checks and purges it in a Debian container (`deb/test.sh`),
as CI does on trixie and on bookworm. lintian reports two errors by design,
the statically linked binary and the missing Debian-format changelog (the
changelog is `CHANGELOG.md` on GitHub).

## The apt repository

The packages of the newest ten releases of every update channel are also in an
apt repository on the project's GitHub Pages,
<https://metaworx.github.io/massimo/apt/>, one suite per channel (`alpha`;
later `beta` and `stable`),
the same for every Debian and Ubuntu release since the binary is static,
signed with the repository's own OpenPGP key (RSA 4096;
not the Ed25519 release key of the manifest, which apt cannot use).
To follow a channel:

```bash
sudo curl -fsSLo /etc/apt/keyrings/massimo.asc https://metaworx.github.io/massimo/apt/massimo.asc
sudo tee /etc/apt/sources.list.d/massimo.sources <<'EOF'
Types: deb
URIs: https://metaworx.github.io/massimo/apt
Suites: alpha
Components: main
Signed-By: /etc/apt/keyrings/massimo.asc
EOF
sudo apt update && sudo apt install massimo
```

`apt upgrade` then installs each release as it is published,
and `apt install massimo=0.6.0~alpha.1` goes back to one still in the pool.
`unattended-upgrades` upgrades Debian's own origins only;
`"origin=Massimo";` in its `Unattended-Upgrade::Origins-Pattern` adds this repository.
The package `massimo-archive-keyring` carries the key as
`/usr/share/keyrings/massimo-archive-keyring.gpg`;
with `Signed-By:` pointing there instead of the downloaded file,
an extended or a rotated key arrives through apt
(the package's version is the key's expiry date).

The site is built by `packaging/apt/publish.sh` in the release workflow (the
pool from GitHub's releases, `apt-ftparchive`, `InRelease` signed by gpg,
the keyring package by nfpm) and deployed with `actions/deploy-pages`;
it has no state of its own,
so a release that is deleted on GitHub leaves the pool at the next release.
`packaging/apt/test.sh` builds it from `dist/` with a throwaway key,
verifies `InRelease`, and installs from it in a container,
as CI does on trixie and on bookworm (`make apt-test`).
Until the owner has made the key and set the secrets,
the workflow skips the steps.

The owner's steps, once:

- Make the key, offline:
  `gpg --quick-gen-key "Massimo apt repository" rsa4096 sign 2y`;
  the fingerprint is `<fpr>` below.
- Put `gpg --armor --export-secret-keys <fpr>` into the secret `MASSIMO_APT_KEY`
  of the `signing` environment and the passphrase into `MASSIMO_APT_KEY_PASSPHRASE`
  (GUIDELINES/project/CI.md, section 5).
- In the repository's settings, Pages → Source: GitHub Actions.
- Before the key expires,
  `gpg --quick-set-expire <fpr> 2y` and the secret again;
  the next release publishes the renewed key and a newer keyring package.

## Install by hand

From the build machine (here for a Pi 1 at `pi.local`, user `pi`):

```bash
make linux-armv6
scp dist/massimo-linux-armv6 packaging/systemd/massimo.service packaging/logrotate/massimo pi@pi.local:
```

On the Pi:

```bash
sudo install -m 0755 massimo-linux-armv6 /usr/local/bin/massimo
massimo version
sudo useradd --system --home-dir /var/lib/massimo --no-create-home --shell /usr/sbin/nologin massimo
sudo install -d -m 0750 -o massimo -g massimo /etc/massimo
sudo install -m 0644 massimo.service /etc/systemd/system/massimo.service
sudo install -m 0644 massimo /etc/logrotate.d/massimo
```

Then write `/etc/massimo/env` (same format as the `.env` described in the
main README) and make it readable only by the service:

```bash
sudo install -m 0600 -o massimo -g massimo /dev/null /etc/massimo/env
sudoedit /etc/massimo/env
```

What to set there:

- `MASSIMO_CUBE_<SN>_ADDRESS`, `_KEY`, `_IV` for every Cube.
- `MASSIMO_CUBE_<SN>_PROXY`:
  the address this box answers on for that Cube,
  one of the box's own IP addresses.
  Each Cube needs its own address,
  because port 62910 can be bound once per address.
  Keeping the box's own address free and adding one extra address per Cube is
  the tidiest setup; addresses outside the router's DHCP range,
  on Raspberry Pi OS (NetworkManager):

  ```bash
  sudo nmcli con mod "Wired connection 1" +ipv4.addresses 192.168.1.49/24 +ipv4.addresses 192.168.1.50/24
  sudo nmcli device reapply eth0
  ```

  For a first run without the proxy, add `-proxy off` to `ExecStart`
  (`sudo systemctl edit --full massimo`).
- `MASSIMO_CUBE_<SN>_PROXY_SERIAL` if the original software should see a
  serial other than the Cube's.
- `MASSIMO_API_TOKEN` to require a Bearer token on the API.
- `MASSIMO_LOG=/var/log/massimo/serve.log`, so that `massimo` commands run
  with `-env /etc/massimo/env` agree with the unit (which passes `-log`).

Check the settings and that the Pi reaches a Cube before enabling the
service (with no other server connected to that Cube):

```bash
sudo -u massimo massimo -env /etc/massimo/env config
sudo -u massimo massimo -env /etc/massimo/env status -sn <serial>
```

Start it and follow the log:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now massimo
journalctl -u massimo -f
```

From another machine:

```bash
curl -H "Authorization: Bearer <token>" http://<box>:8099/api/v1/cubes
massimo status -addr <proxy address>
```

and the web UI is at `http://<box>:8099/`.

## The store

`serve` keeps the Cubes, the Cubes discovery found, its settings and its
node identity in `massimo.db` in the data directory, which every
installer passes with `-data`:

| Where | Data directory | Who |
|---|---|---|
| Linux service | `/var/lib/massimo` (`StateDirectory=massimo`, 0750) | user `massimo` |
| macOS LaunchDaemon | `/var/lib/massimo` | user `_massimo` |
| Windows service | `C:\ProgramData\Massimo` (SYSTEM and Administrators only) | LocalSystem |
| Terminal run | the current directory, or `-data <dir>` / `MASSIMO_DATA` | you |

Next to `massimo.db` (mode 0600) live `massimo.db-wal` and `massimo.db-shm`
while the server runs, and `node.key` (0600), the box's Ed25519 key;
a copy of the database belongs with all of them,
or is made with `massimo store backup <file>`,
which is consistent while the server runs.
`sudo -u massimo massimo -data /var/lib/massimo store info` shows the schema version,
the node and the row counts; `massimo config` shows, per Cube,
which values the store has and which line of `/etc/massimo/env` differs.

**The first start after the update to a release with the store** needs nothing done.
The server creates `massimo.db`, migrates it to schema version 1,
and adds every Cube of `/etc/massimo/env` to it,
one log line each (`store created`, `schema migrated`,
`cube added from the settings file`);
the Cubes connect and the proxies listen as before, from the same values.
`/etc/massimo/env` keeps its lines.
From then on the store wins for a Cube it knows: its `_ADDRESS`, `_NAME`,
`_PROXY`,
`_PROXY_SERIAL` and `_IGNORE` lines only add a Cube the store does not know yet,
and a line that differs from the store is logged at every start.
Keys, IVs, the token, the listen address,
the log and the update variables stay in the file.
A known Cube is changed in the web UI under Settings → Cubes (or with
`PUT /api/v1/cubes/<SN>`), which applies at once, without a restart;
where a line of the file differs,
the Cube's panel says so and offers "Take the file's value".

A Windows service installed before this release runs without `-data` until
`install` is run again with the new binary;
such a service uses `C:\ProgramData\Massimo` anyway,
since its working directory is `System32`.
The systemd unit's working directory has always been `/var/lib/massimo`,
so an old unit with a new binary puts the file in the same place.

**Rollback** (`massimo update -rollback`, or the helper after a failed
health check) is safe: the previous binary knows no store, reads
`/etc/massimo/env` as it always did and leaves `massimo.db` alone.
Changes made in the store meanwhile are not in the file,
so the old binary serves the file's values.
Updating again opens the database as it was left and logs the lines of the file
that differ.
A later binary refuses a file only when a migration changed a meaning (none so far);
it then names the copy `massimo.db.before-<version>` it made before.

`install -remove` keeps the data directory, `-purge` removes it with the store.

## Moving to another network

- Cubes ship with DHCP on (factory fallback 192.168.0.222/16),
  so a Cube gets an address from the new router.
  Reserve it there, because a configured address is fixed.
  A Cube configured by serial only follows its discovery answers by itself;
  for one with an address, change it under Settings → Cubes in the web UI,
  or empty it there so that the Cube follows its discovery answers.
- The fixed proxy addresses added with `nmcli` stay on the interface when the box moves.
  On the new network they must be free and outside the router's DHCP range:
  remove them with `-ipv4.addresses` and add new ones as above.
  The address from DHCP itself follows the new router.
- A Cube that was moved may still answer ping but not TCP 62910 or UDP
  23272 until it is power-cycled.
- A Cube that already has a client (for example this server) accepts no second
  one and drops the connection attempt silently,
  so a test connection from elsewhere times out instead of being refused.
  Test through the proxy instead: `massimo status -addr <proxy address>`.

## Updating

Development happens on `main`;
`make release-alpha` promotes it to the release branch `alpha` and tags it,
and `.github/workflows/release.yml` builds, signs and publishes the release.
Each release belongs to an **update channel**, its release branch:
`alpha` for the `v0.1.0-alpha.N` releases,
later `beta` and a stable one the same way.
A release binary follows its channel;
a binary built from another branch follows that branch, which has no releases,
unless `MASSIMO_UPDATE_CHANNEL=alpha` is set (`massimo update -check` shows the channel).
A release is installed only when it is newer than the running version,
so switching the channel never goes back in version;
`-version <tag>` installs a given release,
older ones too. v0.1.0-alpha.10 and .11 were made before the branch was renamed
and follow `phase-0`; on those, set `MASSIMO_UPDATE_CHANNEL=alpha`.

By hand, as root:

```bash
sudo massimo update -check
sudo massimo update -restart massimo.service -health http://127.0.0.1:8099/healthz
```

It downloads the release's manifest and checks its signature against the public
key compiled into the running massimo,
downloads the binary for this machine (`massimo-linux-armv6` on a Pi 1)
and checks its SHA-256 against the manifest,
runs `<new binary> version` to make sure it starts here,
keeps the old binary as `/usr/local/bin/massimo.prev` and renames the new one into place.
With `-restart` it restarts the unit,
with `-health` it waits until `/healthz` reports the new version and otherwise
puts the old binary back (the failed one stays as `massimo.failed` until the
next update succeeds).

Back to the previous binary (once more goes forward again):

```bash
sudo massimo update -rollback -restart massimo.service
```

The release list comes from GitHub's API,
which allows 60 requests an hour per address without a token.
Behind an address many share (a CI runner,
a large NAT) the check can then fail with `403 Forbidden`;
`MASSIMO_GITHUB_TOKEN`, a GitHub token without any rights,
lifts the limit (in the settings for the server's check,
in the environment for `massimo update`).
It is sent to the API only, and changes nothing about what is installed:
that is still decided by the signature.

### From the web UI

The service runs as `massimo` and cannot write `/usr/local/bin`,
so the web UI only asks:
`POST /api/v1/update` writes `/var/lib/massimo/update-request`,
and the path unit starts the helper as root,
which does exactly the update above.
The request carries nothing but the channel name,
the helper reads nothing else the `massimo` user can write (in particular
not `/etc/massimo/env`),
and the release must verify against the compiled-in key,
so a compromised server can at most install the newest signed release.
Starting updates from the API needs `MASSIMO_API_TOKEN` to be set.

```bash
sudo install -m 0644 massimo-update.path massimo-update.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now massimo-update.path
```

`massimo.service` passes `-update-request /var/lib/massimo/update-request` to `serve`,
which enables the button under Settings → Software.
The helper's log is in `journalctl -u massimo-update`,
its last result in `/var/lib/massimo-update/status.json` and in the UI.
On macOS the same request starts the LaunchDaemon `ch.metaworx.massimo-update`
instead (see "macOS");
on Windows the server starts the helper itself (see "Windows").

The release key: the workflow signs with `MASSIMO_RELEASE_KEY`,
a secret of the repository's `signing` environment that only `v*` tags can use;
its public half is `internal/update/release-key.pem`.
A new key needs a release built with the new public key before releases signed
with it are accepted.

## Windows

From a command prompt opened with "Run as administrator", in the folder
holding `massimo-windows-amd64.exe` from a release and your settings (a
`.env` as described in the main README):

```bat
massimo-windows-amd64.exe install -dry-run -settings .env
massimo-windows-amd64.exe install -settings .env
```

It installs the binary as `C:\Program Files\Massimo\massimo.exe`,
the settings as `C:\ProgramData\Massimo\env` (without `-settings`,
a template to fill in) with the log `serve.log` next to them,
in a folder only SYSTEM and the Administrators can read,
since the settings hold the Cube keys.
It creates the service `massimo` (Massimo in the Services console):
automatic start, running as LocalSystem, restarted after 10 s on failure,
running `massimo -env C:\ProgramData\Massimo\env serve -listen :8099 -log …`;
and a firewall rule for the program on private and domain networks (not public ones),
which covers the API, the Cube proxy and discovery.
`-listen <addr>` changes the address, and installing again keeps it.
`sc start massimo`, `sc stop massimo` and `sc query massimo` control it;
`install -remove` deletes the service, the rule and the program folder,
`-purge` the settings and the log too.

Updates from an administrator prompt:

```bat
"C:\Program Files\Massimo\massimo.exe" update -restart massimo -health http://127.0.0.1:8099/healthz
```

Windows lets a running `.exe` be renamed but not replaced,
so the update renames `massimo.exe` to `massimo.exe.prev` and puts the new one
in its place, then restarts the service through the service manager;
`-rollback` works as on Linux.
With `-health`, a release that cannot run as a service is given up after about
35 s (the service manager's 30 s for a process to answer it, error 1053);
one that runs but never reports the new version on `/healthz` after `-health-timeout`,
two minutes.

The web UI starts updates too (Settings → Software,
or `POST /api/v1/update` with the API token).
Nothing on Windows watches a file, and the service already runs as LocalSystem,
so the server starts the helper itself:
it writes the channel into `C:\ProgramData\Massimo\update-request` and runs the
installed
`massimo.exe update -request … -status C:\ProgramData\Massimo\update-status.json -restart massimo -health http://127.0.0.1:8099/healthz` as a detached process, which outlives the service it stops and does exactly the update above (the `-health` URL follows the service's `-listen`).
The helper gets the server's environment without the settings (no Cube keys,
no API token; only `MASSIMO_GITHUB_TOKEN`),
and writes its output to `C:\ProgramData\Massimo\update.log`,
which is not rotated; its last result is in `update-status.json` and in the UI,
the server's `update requested` and `update helper started pid=…` lines in `serve.log`,
and the stop and start in `service.log` and the System event log.
The three files are in the data folder only SYSTEM and the Administrators can write,
and `-purge` removes them with it.
`install` passes `-update-request` and `-update-status` to `serve`;
`install -no-update-helper` leaves them out, which hides the button.
Should the helper die between stopping and starting the service,
the service stays stopped: `sc.exe start massimo` brings it back.

## macOS

On a Mac (macOS on Apple silicon or Intel),
with `massimo-darwin-arm64` or `massimo-darwin-amd64` from a release and your
settings in the current folder:

```sh
chmod +x massimo-darwin-arm64
sudo ./massimo-darwin-arm64 install -dry-run -settings .env
sudo ./massimo-darwin-arm64 install -settings .env
```

The release binaries are ad-hoc signed (which Apple silicon requires),
not notarized.
A download with `curl` runs as it is;
one through a browser carries the quarantine attribute,
and Gatekeeper refuses to start it until that is removed:
`xattr -d com.apple.quarantine massimo-darwin-arm64`.
The installed copy and the updates the helper downloads carry none.

The layout is the Linux one, since macOS keeps `/etc` and `/var` (as
`/private/etc` and `/private/var`) for the system's own daemons and
leaves them alone on system updates; so the paths here, in the API docs
and in the update helper are the same everywhere:

- the binary `/usr/local/bin/massimo` (the folder is made when missing,
  as on a fresh Apple silicon Mac);
- the hidden system user and group `_massimo` (made with `dscl`,
  the first free ID from 400; it has no shell and no login),
  which runs the server;
- the settings `/etc/massimo/env` (without `-settings`, a template to fill in),
  mode 0600, owned by `_massimo`;
- the state `/var/lib/massimo` and the helper's status
  `/var/lib/massimo-update/status.json`;
- the log `/var/log/massimo/serve.log`:
  launchd appends the server's output to it,
  so there is no `-log` and no `MASSIMO_LOG` (install warns if the settings set one,
  as every line would come twice). macOS has no `copytruncate` for a log a
  process keeps open, so it is not rotated;
  it grows by about what the Linux one does per week.
- the LaunchDaemon `ch.metaworx.massimo` in `/Library/LaunchDaemons`:
  started at boot without anyone logging in, restarted 10 s after it fails,
  running
  `massimo -env /etc/massimo/env serve -listen :8099 -update-request /var/lib/massimo/update-request`;
- the update helper `ch.metaworx.massimo-update` (root),
  which launchd starts when the server writes `/var/lib/massimo/update-request`:
  the counterpart of the systemd path unit,
  so the web UI can start updates as on Linux (see "From the web UI").
  Its log is `/var/log/massimo-update.log`.

`-listen <addr>` changes the address,
and installing again keeps it (it is read back from the installed property list);
the helper's health check follows it. macOS lets a user other than root listen
on a port below 1024 only on every address,
so `-listen :80` works and `-listen 192.168.1.5:80` does not (install warns).
With the application firewall on,
install allows incoming connections for the binary;
otherwise there is nothing to do.

Control it with `launchctl` (all with `sudo`):

```sh
launchctl print system/ch.metaworx.massimo                             # state, PID, last exit
launchctl kickstart -k system/ch.metaworx.massimo                      # restart
launchctl bootout system/ch.metaworx.massimo                           # stop until the next boot
launchctl bootstrap system /Library/LaunchDaemons/ch.metaworx.massimo.plist   # start again
```

Running `install` again replaces the binary and the property lists and restarts
the server.
`install -remove` unloads both LaunchDaemons and removes them and the binary;
`-purge` also the settings, the state, the logs and the user.
Updates by hand:

```sh
sudo massimo update -restart ch.metaworx.massimo -health http://127.0.0.1:8099/healthz
```

As on Linux the new binary is renamed into place (which macOS needs for a signed binary:
one overwritten in place is killed), the old one kept as `massimo.prev`,
and `-rollback` goes back.

## Resource use

Measured on a Raspberry Pi 1 Model B (BCM2835, 700 MHz,
512 MB of which 426 MB are left to Linux) with Raspberry Pi OS Lite 32-bit (trixie),
serving two Cubes with about 30 devices, the API,
the web UI and two proxy listeners:

- `massimo` resident memory: 7 MB after start,
  9 MB after five minutes (`VmRSS` in `/proc/<pid>/status`;
  the Pi kernel has memory accounting off,
  so `systemctl status` shows no memory line).
- CPU: under 1 % on average, about 1 s of CPU time for the first five minutes;
  both Cubes connected within a second of starting.
- The whole system uses about 110 MB (`free -m`), leaving over 300 MB available;
  the root filesystem needs 2.1 GB.
- The web UI's page loads in about 0.25 s over the 100 Mbit Ethernet.

## Learnings

From the install days on the Pi and the Mac,
kept here because they cost time to learn.

- Pi 1: Ethernet only, no networking over USB;
  the current Pi OS 32-bit still supports it and uses cloud-init user-data;
  about 9 MB RSS; memory cgroup accounting is off,
  so `systemctl status` shows no memory line;
  the first boot takes about 10 minutes.
- macOS: a signed binary overwritten in place gets SIGKILL,
  which is why `install` and `update` rename into place. launchd tests without
  root work in the `gui/<uid>` domain only while that user is logged in at the console;
  `user/<uid>` fails over SSH with "Bootstrap failed: 5";
  SIP blocks `launchctl setenv` in the system domain,
  so a root helper gets its environment only through its plist.
- GitHub's API allows 60 requests an hour per address without a token,
  which is what `MASSIMO_GITHUB_TOKEN` is for.
- For a sudoers line with a command list,
  check with `sudo -n <allowed command>`, not `sudo -n true`.
