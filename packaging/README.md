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
  Runs as the user `_massimo`, reads its settings from `/etc/massimo/env`,
  listens on port 8099 on every interface (API and web UI;
  `install -listen` changes it),
  logs to `/var/log/massimo/serve.log` and the journal, and restarts on failure
  and after a restart from the API,
  which exits with 75 (`RestartForceExitStatus`, not counted as a failure).
  `systemctl reload massimo` sends SIGHUP (`ExecReload`):
  the settings file read again and the store checkpointed.
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
- `homebrew/`:
  the Homebrew formula's template and renderer (see "Homebrew").
- `site/`:
  the pages of the project's site, <https://massimo.metaworx.ch>,
  the home with the install routes and the privacy policy,
  which the apt repository's publishing puts beside it (see "The apt repository").

Every service starts `serve` with `-env <settings file>`,
so a line there reaches it without a change to its command line.
`MASSIMO_LOG_LEVEL=debug` adds the Debug lines to the log
from the service's next restart,
such as every failed attempt to reach a lost Cube
and every command the proxy passes on to it;
the log then grows faster, so remove the line again after the diagnosis.
`env.template` does not carry it:
the package and `install` start a stopped service only when the settings
set more than the template does, comments and blank lines aside,
so a new setting there would make an untouched file look filled in after an upgrade.

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
creates the system user `_massimo`, its state directory `/var/lib/massimo`
and `/etc/massimo/env` (the commented template,
or a copy of `-settings <.env>`), writes `massimo.service`,
the update helper `massimo-update.path` and `.service` and the log rotation,
and enables them.
An account `massimo` that an older release made, with the home
`/var/lib/massimo`, is renamed `_massimo` in place:
it keeps its ID, so its files and a running service follow.
Any other `massimo` is a person's, and is left alone, also by `-remove -purge`.
Where an older release ran the service as such a person's account,
its files are still that person's, which `_massimo` cannot read:
`install` says so and names the `chown -R` that hands them to the service.
It starts the service when the settings set more than the template does
(its comments and blank lines aside),
and restarts it when it runs;
a box that never ran on the template's settings is left for its owner,
because a server without users answers every device on the local network,
as README.md, "Server", says.
It warns when a Cube's `_PROXY` address is not one of the box's addresses (serve
could not listen there and would restart every ten seconds;
install adds no addresses) and when `MASSIMO_API_TOKEN` is missing.
Nothing needs to go into `/etc/massimo/env` for the first start.
Either create the owner first and then start the service:

```bash
sudo -u _massimo /usr/local/bin/massimo user add <name> -role owner
sudo systemctl start massimo
```

or start it and create the owner in the web UI from the local network,
which until then may use it unasked;
the Cubes are added there too (System settings → Cubes, "Found on the LAN").
The binary's full path and `-env` let the first line run from any
directory, a home directory the service user may not enter included,
and through a sudo whose PATH lacks `/usr/local/bin`, as on macOS.

Running `install` again, from a newer binary,
replaces the binary and the units and restarts the service by the same rule;
existing settings are kept,
and a unit that differs from the new one (edited with `systemctl edit --full`)
is kept as `<unit>.bak`.
Local changes belong in a drop-in (`systemctl edit massimo`),
which install never touches.
`-no-update-helper` leaves the helper out,
and on a box an earlier install gave it disables `massimo-update.path`
and removes both units and their drop-in, as macOS unloads its own;
`-reboot-helper` adds the reboot helper and `-no-reboot-helper` takes it out
again (see "Rebooting from the web UI");
`-no-start` only enables.

`-listen <addr>` moves the API and web UI off `:8099`,
for example to port 80 (`sudo massimo install -listen :80`,
or `192.168.1.200:80` for one address only).
It writes two drop-ins that later installs keep:
`massimo.service.d/listen.conf` sets `MASSIMO_LISTEN` and, below port 1024,
grants the service `CAP_NET_BIND_SERVICE`,
since the user `_massimo` cannot bind such a port otherwise;
`massimo-update.service.d/listen.conf` sets `MASSIMO_HEALTH`,
where the update helper checks the restarted server.
Deleting both files (and `systemctl daemon-reload`,
`systemctl restart massimo`) goes back to `:8099`.

`-tls-cert <pem> -tls-key <pem>` makes the service serve `https://`:
the files are copied to `/etc/massimo/cert.pem` and `key.pem`,
owned by `_massimo` and mode 0600, and `MASSIMO_TLS=files`,
`MASSIMO_TLS_CERT` and `MASSIMO_TLS_KEY` are set in `/etc/massimo/env`,
which later installs keep.
The server re-reads the files when they change, so a renewal needs no restart:
certbot keeps its files under `/etc/letsencrypt/live/<name>/`,
readable by root only, so a deploy hook copies them over
(`--deploy-hook 'cp "$RENEWED_LINEAGE"/fullchain.pem /etc/massimo/cert.pem; cp "$RENEWED_LINEAGE"/privkey.pem /etc/massimo/key.pem; chown _massimo:_massimo /etc/massimo/*.pem'`).
The update helper then checks `https://127.0.0.1:8099/healthz`
(`massimo-update.service.d/listen.conf`),
without verifying the certificate of a loopback address,
nor of one of the box's own addresses (a specific `-listen`),
which a connection to never leaves the box.
`-tls self-signed [-tls-names <list>]` sets `MASSIMO_TLS=self-signed` instead:
the server makes a certificate for the box's names at its first start into
`/var/lib/massimo/tls/`, renews it 30 days before it expires,
and logs its fingerprint,
which `massimo tls show` prints too (as the user `_massimo`,
which reads the service's settings and data directory from any directory);
browsers warn once per device.
After a move to another address,
`sudo -u _massimo massimo tls self-signed` makes a new one,
served at the next connection.
`-tls acme -tls-names <name> [-acme-email <address>] [-acme-directory <url>] [-tls-http :80]` sets
`MASSIMO_TLS=acme` with the name, the contact,
the CA's directory and the helper address:
the server orders the certificate from Let's Encrypt at its start
(from its staging directory with
`-acme-directory https://acme-staging-v02.api.letsencrypt.org/directory`,
the way to a first try,
since the production one allows five failed orders an hour),
into `/var/lib/massimo/tls/acme/`, and renews it by itself.
The name must resolve to the box's public address and the router forward
port 443 to the service's port (`-listen :443` with the capability,
or the forward to `:8099`);
with `-tls-http :80` the listen drop-in grants the capability for port 80 too.
`massimo tls show` (as `_massimo`) prints what was issued.
`-acme-dns-hook <script>` with `-tls acme -tls-names …` takes the DNS-01
challenge instead of a port: the server runs
`<script> set _acme-challenge.<name> <value>` before the validation and
`<script> clear …` after it, waits until the resolver shows the record,
and keeps the certificate and its key in `/var/lib/massimo/tls/acme/`.
The name may point at the box's LAN address,
and wildcards work (`-tls-names '*.example.net,example.net'`).
The script runs as `_massimo` with the server's environment less the `MASSIMO_` variables,
so its own secret comes from a file;
a run ends after two minutes, with every process it started.
`packaging/acme-dns-hooks/infomaniak.sh` is the script for a zone hosted at
Infomaniak (its name servers, not merely registered there):
it needs `curl` and `jq`,
and a token with the DNS scope from the API section of the Infomaniak
manager in `/etc/massimo/infomaniak.token` (owned by `_massimo`, mode 0600).
It hands the token to `curl` on its standard input, never on its command line,
which every local user can read,
and gives each call 30 seconds.
It ran against the real API on 2026-11-08;
the test runs it against a fake one.

```bash
sudo install -D -o _massimo -g _massimo -m 0755 packaging/acme-dns-hooks/infomaniak.sh /usr/local/lib/massimo/infomaniak.sh
sudo install -o _massimo -g _massimo -m 0600 /dev/stdin /etc/massimo/infomaniak.token <<< "$INFOMANIAK_TOKEN"
sudo massimo install -tls acme -tls-names massimo.example.net -acme-dns-hook /usr/local/lib/massimo/infomaniak.sh
```

`packaging/acme-dns-hooks/cloudflare.sh` is the script for a zone at Cloudflare,
with the same needs and the same care for its token:
an API token with the permission Zone → DNS → Edit on the zone,
made from the template "Edit zone DNS" under My Profile → API Tokens,
in `/etc/massimo/cloudflare.token` (owned by `_massimo`, mode 0600).
It finds the zone by name;
where the token may not list it,
a line `CLOUDFLARE_ZONE_ID=<id>` in `/etc/massimo/env` names it
(the id is on the zone's overview page):
the script gets every variable of the settings file but the `MASSIMO_` ones.
The test runs it against a fake of Cloudflare's API.

```bash
sudo install -D -o _massimo -g _massimo -m 0755 packaging/acme-dns-hooks/cloudflare.sh /usr/local/lib/massimo/cloudflare.sh
sudo install -o _massimo -g _massimo -m 0600 /dev/stdin /etc/massimo/cloudflare.token <<< "$CLOUDFLARE_TOKEN"
sudo massimo install -tls acme -tls-names massimo.example.net -acme-dns-hook /usr/local/lib/massimo/cloudflare.sh
```

A box with the Debian package has both scripts already,
as `/usr/share/massimo/acme-dns-hooks/infomaniak.sh` and `cloudflare.sh`:
there the token file and the `massimo install` line are all it takes,
with `-acme-dns-hook /usr/share/massimo/acme-dns-hooks/<provider>.sh`.

Without the hook, an external client into `-tls-cert` and `-tls-key` does the same:
`certbot certonly --authenticator dns-infomaniak -d massimo.example.net` with
the plugin `certbot-dns-infomaniak` (`pip install`) and its credentials file
(`dns_infomaniak_token = …`, mode 0600, `--dns-infomaniak-credentials`),
plus the deploy hook above that copies the files;
certbot's renewal service sees no shell environment,
so the token goes into the credentials file or the unit's `Environment=`.
`lego --dns infomaniak -d massimo.example.net run` with
`INFOMANIAK_ACCESS_TOKEN` is the other way.
An install with another `-tls` mode removes the variables of the one before
(`MASSIMO_TLS_CERT`, `MASSIMO_TLS_KEY`, `MASSIMO_TLS_NAMES`,
`MASSIMO_ACME_EMAIL`, `MASSIMO_ACME_DIRECTORY`,
`MASSIMO_TLS_HTTP`, `MASSIMO_ACME_DNS_HOOK`)
and sets those the new one is given;
an install without `-tls` keeps them all.
`-tls off` switches it off again;
run the service once with `-hsts 0` over TLS first,
since a browser that saw the server's HSTS header refuses `http://` to it
for a year otherwise.

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
that path),
the update helper's `massimo-update.path` and the package's own `massimo-update.service`,
enabled (see "Updates through apt" below),
the log rotation and the marker `/usr/share/massimo/packaged`,
the DNS-01 hooks `/usr/share/massimo/acme-dns-hooks/infomaniak.sh` and `cloudflare.sh`
(see `-acme-dns-hook` above),
and the reboot helper's `massimo-reboot.path` and `.service`, disabled
(see "Rebooting from the web UI");
its scripts create the user `_massimo`, `/etc/massimo/env` from the template
and the state directory `/var/lib/massimo` when they are missing,
and enable the service.
The account `massimo` of an older package is renamed `_massimo` in place,
keeping its ID, so its files and a running service follow;
a person's `massimo` is left alone, also by a purge.
Where an older package ran the service as a person's `massimo`,
the files are still that person's, and the service cannot read them;
the package says so and names
`sudo chown -R _massimo:_massimo /etc/massimo /var/lib/massimo /var/log/massimo`.
A script of your own that names `massimo`, such as a certbot deploy hook,
needs the new name.
They start it only when the settings set more than the template does
(its comments and blank lines aside),
because a server without users answers every device on the local network,
as README.md, "Server", says:
on a fresh box the first start is the owner's.
Nothing needs to go into `/etc/massimo/env` for that.
Either create the owner first and then start the service:

```bash
sudo -u _massimo /usr/bin/massimo user add <name> -role owner
sudo systemctl start massimo
```

or start it and create the owner in the web UI from the local network,
which until then may use it unasked.
The Cubes are added there too, under System settings → Cubes,
"Found on the LAN";
`/etc/massimo/env` takes what the web UI does not,
as "Install by hand" describes.
The service listens on the LAN, so it announces itself by mDNS (README.md,
"Server"; `MASSIMO_ANNOUNCE=off` stops it);
a firewall on the box lets in 5353/udp for that,
beside 62910/tcp and 23272/udp for the proxy and 8099/tcp for the page (with ufw:
`sudo ufw allow 5353/udp`).
The next package upgrades the binary and restarts a running service,
whatever the settings, and leaves a stopped one stopped,
unless its settings are filled in;
the settings and the store are left alone.

**One owner of the binary.**
On such a box apt owns `/usr/bin/massimo`, and the marker tells massimo so:
`massimo update` run by hand still says when a newer release is out,
but leaves the install to `sudo apt update && sudo apt install massimo`
(the apt repository below) or to the next release's package by hand,
and the web UI's button has the package's update helper run apt
(see "Updates through apt" below).
`massimo install` keeps only what configures:
`-settings` over a settings file that is still the template,
`-tls` and its options into the settings as "Install with `massimo install`" puts them,
`-listen` as the listen drop-in there and the update helper's health drop-in,
the update helper's path unit enabled, or disabled with `-no-update-helper`,
`-reboot-helper` and `-no-reboot-helper` as the reboot helper's path unit
enabled or disabled,
then a restart, by the rule of "Install with `massimo install`".
`massimo install -remove` points at `apt remove massimo`;
`apt purge massimo` takes the settings, the store, the logs and the user too,
as `-remove -purge` does.
A box set up with `massimo install` keeps its own updater:
the package refuses to install over it until `sudo massimo install -remove` has run,
which keeps the settings and the store;
a `-listen` address is set again afterwards with `sudo massimo install -listen <addr>`.

**Updates through apt.**
The package's update helper is the one of "From the web UI" below,
started by the same request file,
but with apt as the owner it installs no release itself:
it takes the request, whose channel it ignores,
and runs apt for the massimo source of "The apt repository" alone,

```bash
apt-get update -o Dir::Etc::sourcelist=/etc/apt/sources.list.d/massimo.sources \
  -o Dir::Etc::sourceparts=- -o APT::Get::List-Cleanup=0
apt-get install -y --only-upgrade -o Dpkg::Options::=--force-confdef \
  -o Dpkg::Options::=--force-confold -o DPkg::Lock::Timeout=120 massimo
```

so it follows that source's suite and does what `apt upgrade` does for massimo,
the other sources' lists left as they are.
While another apt run holds apt's lock (`unattended-upgrades` among them)
it waits up to two minutes for each command,
since `apt-get update` does not wait itself,
then reports apt as busy.
The server follows the source's suite for its check as well,
`Suites:` of `/etc/apt/sources.list.d/massimo.sources`:
the web UI says "Installed by apt" with that channel,
and refuses an update while `MASSIMO_UPDATE_CHANNEL` names another one,
while the source is missing (a package installed from its file alone),
or while the helper is off.
The package's `postinst` restarts the service,
and the helper waits until `/healthz` reports the version apt installed.
When it does not, the helper reports that, with the way back,
`sudo apt install massimo=<previous version>`,
and downgrades nothing:
an older binary would run on a store the newer one may have migrated.
"Nothing newer" is reported when the GitHub release is out before the apt
repository has it.
Its sandbox is lighter than the hand install's,
since apt and the package's scripts write `/usr`, `/etc` and `/var`;
its log is `journalctl -u massimo-update`,
its last result `/var/lib/massimo-update/status.json`.
`sudo massimo install -no-update-helper` turns it off,
and a later package keeps it off;
`sudo massimo install` turns it on again.

The package is built by `make packages` (nfpm, at the version the Makefile pins,
through `go run`) from `nfpm.yaml`,
the scripts in `deb/` and the unit `deb/unit.sh` derives;
`make deb-test` installs,
checks and purges it in a Debian container (`deb/test.sh`),
as CI does on trixie and on bookworm.
That container has no systemd;
`make deb-systemd-test` runs the cases that need one (`deb/systemd-test.sh`),
each in a container of its own with systemd as its first process:
the first start left to the owner and the owner line as printed,
the restart of a running service, the upgrade from v0.12.0-alpha.1
with the service running, a person called massimo, a box an older release
took over, and `massimo install` over v0.12.0-alpha.1's hand install.
lintian reports two errors by design,
the statically linked binary and the missing Debian-format changelog (the
changelog is `CHANGELOG.md` on GitHub).

## The apt repository

The packages of the newest ten releases of every update channel are also in an
apt repository on the project's site,
<https://massimo.metaworx.ch/apt/>, one suite per channel (`alpha`;
later `beta` and `stable`),
the same for every Debian and Ubuntu release since the binary is static,
signed with the repository's own OpenPGP key (RSA 4096;
not the Ed25519 release key of the manifest, which apt cannot use).
To follow a channel, install the keyring package first, then the sources:

<!-- BEGIN GENERATED PARTIAL docs/fragments/_apt-sources.md - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->
```bash
curl -fsSLo /tmp/massimo-archive-keyring.deb https://massimo.metaworx.ch/apt/massimo-archive-keyring.deb
sudo apt install /tmp/massimo-archive-keyring.deb
sudo tee /etc/apt/sources.list.d/massimo.sources <<'EOF'
Types: deb
URIs: https://massimo.metaworx.ch/apt
Suites: alpha
Components: main
Signed-By: /usr/share/keyrings/massimo-archive-keyring.gpg
EOF
sudo apt update && sudo apt install massimo
```
<!-- END GENERATED PARTIAL docs/fragments/_apt-sources.md -->

`apt upgrade` then installs each release as it is published,
and `apt install massimo=0.6.0~alpha.1` goes back to one still in the pool.
`unattended-upgrades` upgrades Debian's own origins only;
`"origin=Massimo";` in its `Unattended-Upgrade::Origins-Pattern` adds this repository.
The package `massimo-archive-keyring` carries the key as
`/usr/share/keyrings/massimo-archive-keyring.gpg`, the file `Signed-By:` names,
and is in the suite as well,
so that apt verifies with the key of the newest keyring package,
and an extended key arrives through apt.
Its version is the time of the publish, so it only goes up.
The first download is the one trusted through HTTPS alone,
as a downloaded key would be;
the key itself is on the site too, as `massimo.asc`, for a check by hand.
A new key would reach the boxes only through a keyring package carrying both
keys on a site the old key still signs, which `publish.sh` does not make:
it signs with one key.

The repository is built by `packaging/apt/publish.sh` in the release
workflow (the pool from GitHub's releases, `apt-ftparchive`, `InRelease`
signed by gpg, the keyring package by nfpm) and put on the site with the
pages and the docs (see "The site");
it has no state of its own,
so a release that is deleted on GitHub leaves the pool at the next release.
Each release also carries the signed index as `massimo-apt-index.tar.gz`,
so that the site can be rebuilt from `main` without the key,
which only the release tags reach:
`apt/dists/` as signed, `apt/massimo.asc`, `apt/massimo.sources`,
`apt/massimo-archive-keyring.deb`, and `apt/pool.tsv`,
one line per package of the pool, tab-separated:
its path under `apt/`, its SHA-256,
and the download URL of the release file it is,
or `included` for a package in the tarball itself (the keyring package);
`publish.sh --index-asset <file>` writes it.
`packaging/apt/test.sh` builds it from `dist/` with a throwaway key,
verifies `InRelease` and the index asset,
builds the site from the repository and again from the index asset alone,
and installs from it in a container as the snippet above does,
then upgrades to a newer package through the update helper while another
process holds apt's lock,
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
- Then Environments → `github-pages` → Deployment branches and tags:
  add a rule for the tags `v*`.
  The environment GitHub makes for Pages deploys from the default branch only,
  and a release's deploy runs for its tag,
  so without the rule a release's deploy is refused;
  a push to `main` deploys under the default rule.
- The domain (the CNAME, the custom domain and HTTPS set on 2026-11-09):
  verify `metaworx.ch` for the account under its settings → Pages,
  with the TXT record GitHub names, so that no other account can claim it;
  at the DNS provider, a CNAME record
  `massimo.metaworx.ch` → `metaworx.github.io` (the account's address,
  without the repository's name);
  in the repository's settings, Pages → Custom domain `massimo.metaworx.ch`,
  then "Enforce HTTPS" once GitHub has the certificate.
  A site deployed by a workflow needs no `CNAME` file.
- Before the key expires,
  `gpg --quick-set-expire <fpr> 2y` and the secret again;
  the next release publishes the renewed key and a newer keyring package.

### Moving from the old address

Until 2026-11-09 the repository was at `https://metaworx.github.io/massimo/apt`,
which a box set up before then still names in `/etc/apt/sources.list.d/massimo.sources`.
GitHub redirects that address to the new one.
When the domain was set up, the redirect went through `http://` first,
and apt refused it:

```text
Redirection from https to 'http://massimo.metaworx.ch/…' is forbidden
```

Since later that day it goes straight to `https://` and apt follows it,
but the redirect is GitHub's and not documented,
so change the line once:

```bash
sudo sed -i 's#https://metaworx.github.io/massimo/apt#https://massimo.metaworx.ch/apt#' /etc/apt/sources.list.d/massimo.sources && sudo apt update
```

The file keeps its name, and the keyring package and the key stay as they are.

## The site

The project's site, <https://massimo.metaworx.ch>, is built by
`packaging/site/build.sh`, without a key, and deployed to GitHub Pages by
`.github/workflows/site.yml`, which the release workflow calls at every run:
a release, a push to `main` (the devel build) and the nightly build.
It holds:

- `apt/`, the apt repository: the one a release's run has just signed, or,
  in every other run, the newest release's `massimo-apt-index.tar.gz`, its
  pool downloaded by `pool.tsv` and every package checked by its sum.
  No release carries one before the first release made after 2026-11-10;
  until then only a release with the key deploys,
  since a deploy replaces the whole site and one without `apt/` would take
  the repository offline (`build.sh` exits 3, and the job ends green).
- At the root, the pages of `site/`:
  `index.html`, the home with the install routes, its suites filled in,
  `privacy.html`, the privacy policy of the project and its apps,
  which the app stores' listings link,
  and `site.css`, the stylesheet of every page.
- Under `docs/<channel>/`, the documentation of each channel,
  built from its newest build, `release/<channel>`
  (a branch for `alpha`, `beta` and `stable`,
  a tag for `devel` and `nightly`), and under `docs/main/` from `main`.
  The documents are those `docs/fragments/_documents.md` lists,
  the list in README.md's "Documentation" too;
  each is converted by the guidelines' `md_html.py`,
  with the heading ids GitHub gives, links between the documents to their
  pages, and every other link to GitHub at that commit, or to the guidelines
  at its pin, by `packaging/site/site.py` into `doc.html.in`.
  `docs/channels.json` names the channels built,
  a selector on every page (`docs.js`) changes to the same page of another,
  and `docs/` sends a reader to the most stable one.
  A build without the list (every build before 2026-11-10) gets no docs:
  its `docs/` still held the working documents.

The build fails on a document under `docs/` missing from the list
(for `main`), on a link to a file the commit does not have,
and on a link to a page or a heading that is not there.
CI's `Site` job builds the pages and the docs of every pull request that way,
and `make site` those of the checkout into `dist/site`;
the links are root-relative, so read it served,
`python3 -m http.server -d dist/site`.

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
sudo useradd --system --user-group --home-dir /var/lib/massimo --no-create-home --shell /usr/sbin/nologin _massimo
sudo install -d -m 0750 -o _massimo -g _massimo /etc/massimo
sudo install -m 0644 massimo.service /etc/systemd/system/massimo.service
sudo install -m 0644 massimo /etc/logrotate.d/massimo
```

Then write `/etc/massimo/env` (same format as the `.env` described in the
main README) and make it readable only by the service:

```bash
sudo install -m 0600 -o _massimo -g _massimo /dev/null /etc/massimo/env
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
  as `_massimo`, which read `/etc/massimo/env`, agree with the unit
  (which passes `-log`).

Check the settings and that the Pi reaches a Cube before enabling the
service (with no other server connected to that Cube):

```bash
sudo -u _massimo massimo config
sudo -u _massimo massimo status -sn <serial>
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
| Linux service | `/var/lib/massimo` (`StateDirectory=massimo`, `StateDirectoryMode=0750`) | user `_massimo` |
| macOS LaunchDaemon | `/var/lib/massimo` | user `_massimo` |
| Windows service | `C:\ProgramData\Massimo` (SYSTEM and Administrators only) | LocalSystem |
| A command as `_massimo` (Linux, macOS) | `/var/lib/massimo`, unless `-env`, `-data` or `MASSIMO_DATA` | user `_massimo` |
| Terminal run | the current directory, or `-data <dir>` / `MASSIMO_DATA` | you |

Next to `massimo.db` (mode 0600) live `massimo.db-wal` and `massimo.db-shm`
while the server runs, `node.key` (0600), the box's Ed25519 key,
and `opaque.key` (0600), the server's OPAQUE keys,
without which no password of `auth_version` 2 signs in;
a copy of the database belongs with all of them,
or is made with `massimo store backup <file>`,
which is consistent while the server runs.
`sudo -u _massimo massimo store info` shows the schema version,
the node and the row counts; `massimo config` shows, per Cube,
which values the store has and which line of `/etc/massimo/env` differs.
The Cubes' events, the log the web UI shows, are in `massimo.db` too,
for the days `log.keep_days` says (30 by default, `GET`/`PUT /api/v1/settings`):
a state event takes about 0.65 KB with its indexes (measured),
so a house of thirty devices that sends 3000 events a day adds about 2 MB a day,
60 MB over the 30 days.
The Cubes' backups are there as well, about 11 KB each for thirty devices,
taken only after a change and kept as the `backups.*` settings say
(docs/api-v1.md, "Backups"), a few megabytes at most.
The file stops growing at that size, since the server reuses the pages the daily
prune frees and never runs `VACUUM`;
`massimo store backup` writes a compact copy.

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
A known Cube is changed in the web UI under System settings → Cubes (or with
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
  for one with an address, change it under System settings → Cubes in the web UI,
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
`make release-alpha` tags it, CI tests the tag, and `.github/workflows/release.yml`
builds, signs and publishes the release, then fast-forwards the release branch
`release/alpha` to it.
Each release has a **flavour**, from the least stable to the most:
`devel`, every commit of `main` whose tests passed;
`nightly`, once a day the newest of those;
`alpha`, the `vX.Y.Z-alpha.N` releases;
later `beta` (`-beta.N`) and `stable` (a plain `vX.Y.Z`).
`devel` and `nightly` are one GitHub release each, `release/devel` and
`release/nightly`, replaced in place by the next build;
the others are a release per tag.
A box follows an **update channel**, one of those names,
and takes the newest release of its channel's flavour or a more stable one:
a box on `alpha` takes a beta or stable release too,
one on `devel` everything.
The channel is `MASSIMO_UPDATE_CHANNEL` where set,
else the setting `update.channel` (`PUT /api/v1/settings`,
`{"update.channel": "devel"}`; the web UI's selector is still to come),
else the flavour of the running binary;
`massimo update -channel <name>` uses another for one run,
and `massimo update -check` shows the channel and where it came from.
Before the web UI's Update installs a release of another flavour,
the server stores its channel as `update.channel` where nothing names one,
so that a box on `beta` stays on `beta` after a stable release;
`massimo update` by hand cannot write the server's store, and says so instead.
A build of anything but a release tag is of the flavour `devel`.
A release is installed only when it is newer than the running version,
so switching the channel never goes back in version;
`-version <tag>` installs a given release (`release/devel` the devel build),
older ones too.
A check fetches the signed manifests of the releases its channel takes alone,
known by the tag's label (`-alpha.N` is `alpha`, a plain tag `stable`)
or the rolling build's tag;
neither is signed, the manifest decides.
v0.1.0-alpha.10 and .11 were made before the branch was renamed
and are signed for `phase-0` under alpha's label,
so a check on `phase-0` finds no release;
on those, set `MASSIMO_UPDATE_CHANNEL=alpha`.

By hand, as root:

```bash
sudo massimo update -check
sudo massimo update -restart massimo.service -health http://127.0.0.1:8099/healthz
```

It downloads the release's manifest and checks its signature against the public
key compiled into the running massimo,
downloads the binary for this machine (`massimo-linux-armv6` on a Pi 1),
for as long as the line needs but given up after a minute without data,
and checks its SHA-256 against the manifest,
runs `<new binary> version` to make sure it starts here,
keeps the old binary as `/usr/local/bin/massimo.prev` and renames the new one into place.
With `-restart` it restarts the unit,
with `-health` it waits until `/healthz` reports the new version and otherwise
puts the old binary back (the failed one stays as `massimo.failed` until the
next update succeeds).
Where the service has TLS the URL is `https://…`;
the helper does not verify the certificate of a loopback address,
nor of an address one of the box's interfaces has when the check starts;
any other host is verified.

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
With it the release's files, its manifest, signature and binary,
come through the API too rather than from github.com's download pages,
which on 2026-11-08 held requests without a token until the check gave up.
It is sent to the API only, and changes nothing about what is installed:
that is still decided by the signature.

### From the web UI

The service runs as `_massimo` and cannot write `/usr/local/bin`,
so the web UI only asks:
`POST /api/v1/update` writes `/var/lib/massimo/update-request`,
and the path unit starts the helper as root,
which does exactly the update above.
The request carries nothing but the channel name,
the helper reads nothing else the `_massimo` user can write (in particular
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
which enables the button under System settings → Software.
The helper's log is in `journalctl -u massimo-update`,
its last result in `/var/lib/massimo-update/status.json` and in the UI.
On macOS the same request starts the LaunchDaemon `ch.metaworx.massimo-update`
instead (see "macOS");
on Windows the server starts the helper itself (see "Windows");
the Debian package's own helper runs apt (see "Debian package").

The release key: the workflow signs with `MASSIMO_RELEASE_KEY`,
a secret of the repository's `signing` environment that only `v*` tags can use,
and where each run waits for the owner's approval;
its public half is `internal/update/release-key.pem`.
The rolling key signs the `devel` and `nightly` builds:
`MASSIMO_ROLLING_KEY`, a secret of the `signing-rolling` environment,
which only `main` can use;
its public half is `internal/update/rolling-key.pem`,
and a binary accepts it for those two flavours alone,
so that a key `main` can reach signs nothing a box on `alpha` or above takes.
`RELEASE_BRANCH_DEPLOY_KEY`, in `signing` too, is the private half of a deploy key
with write access, with which the release fast-forwards `release/<flavour>`.
The repository's rulesets keep the `v*` tags from being moved or deleted
and let the `release/*` branches only fast-forward
(GUIDELINES/project/CI.md, section 6).
A new key needs a release built with the new public key before releases signed
with it are accepted.

## Rebooting from the web UI

The owner can reboot the computer from System settings → Maintenance
(`POST /api/v1/reboot`, for the capability `reboot`, which only `owner` has),
60 seconds after asking or after the delay given there, 0 to 3600,
and take the reboot back meanwhile.
Two things make it so, and both are the owner's:
`MASSIMO_REBOOT=on` in the settings file (or `serve -allow-reboot`),
which `serve` applies at once (`true`, `yes`, `1` and `enabled` too,
in any case),
and on Linux and macOS a root helper that only root installs:

```bash
sudo massimo install -reboot-helper
```

| Where `serve` runs | Who reboots | Needs |
|---|---|---|
| Linux, systemd's service (`massimo install`, the Debian package, Raspberry Pi OS) | `massimo-reboot.path`, which starts `massimo-reboot.service` as root | `install -reboot-helper` and the switch |
| macOS, `massimo install`'s LaunchDaemon | the LaunchDaemon `ch.metaworx.massimo-reboot`, as root | `install -reboot-helper` and the switch |
| the Windows service | the service itself (LocalSystem), `shutdown.exe /r` | the switch |
| Homebrew's service, a terminal, Docker, Linux without systemd | nobody | – |

Where something is missing the page says what, as `GET /api/v1/reboot` does.

**Why a helper, and no sudo.**
`massimo.service` runs with `NoNewPrivileges=yes`,
so `sudo` cannot raise privileges from it whatever sudoers says,
and the sandbox stays as it is.
As for updates, the server only asks:
once the delay is over it writes `/var/lib/massimo/reboot-request`,
the path unit (on macOS launchd's `WatchPaths`) starts the helper,
and the helper removes the request and reboots (`systemctl reboot`,
on macOS `shutdown -r now`).
It reads nothing, not even the request,
so a compromised server can at most reboot the box,
and only where the owner switched that on and root installed the helper.
The request goes before the reboot, so a reboot never starts the next one;
`serve` removes a request the helper has not taken within 30 seconds,
and one it finds at its start.
`serve` knows the helper is there by the link that enabling the path unit makes,
`/etc/systemd/system/paths.target.wants/massimo-reboot.path`,
and on macOS by `/Library/LaunchDaemons/ch.metaworx.massimo-reboot.plist`;
both are root's.
Windows needs no helper: the service runs as LocalSystem,
which may shut the computer down,
and only SYSTEM and the Administrators write its settings file.

**By hand**, as for the update helper:

```bash
sudo install -m 0644 massimo-reboot.path massimo-reboot.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now massimo-reboot.path
```

The Debian package ships both units in `/usr/lib/systemd/system/`, disabled;
`sudo massimo install -reboot-helper` enables the path unit
(`sudo systemctl enable --now massimo-reboot.path` does the same),
and an upgrade keeps it enabled.
`-no-reboot-helper` disables it again, and on a hand install removes the units;
installing again without either flag keeps what is there.
The helper's log is `journalctl -u massimo-reboot`,
on macOS `/var/log/massimo-reboot.log`.

The system stops the service as at every shutdown,
so the store is closed as at every stop;
the thermostats keep their programmes meanwhile,
and the Cube is connected again once the box is back.
A box that does not come back (a failing SD card, a kernel update half done)
needs someone at it: there is no rollback for a reboot.

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
the template) with the log `serve.log` next to them,
in a folder only SYSTEM and the Administrators can read,
since the settings hold the Cube keys.
It creates the service `massimo` (Massimo in the Services console):
automatic start, running as LocalSystem, restarted after 10 s on failure,
running `massimo -env C:\ProgramData\Massimo\env serve -listen :8099 -log …`;
and a firewall rule for the program on private and domain networks (not public ones),
which covers the API, the Cube proxy, discovery and the mDNS announcement.
`-listen <addr>` changes the address, and installing again keeps it.
It starts the service by the rule of "Install with `massimo install`":
a box that never ran on the template's settings is left for its owner,
who creates the owner first from an administrator PowerShell,
`& "C:\Program Files\Massimo\massimo.exe" -data "C:\ProgramData\Massimo" user add <name> -role owner`,
or after `sc.exe start massimo` in the web UI.
`-tls-cert <pem> -tls-key <pem>` copies the files into `C:\ProgramData\Massimo\`
and sets `MASSIMO_TLS`,
`MASSIMO_TLS_CERT` and `MASSIMO_TLS_KEY` in the settings, as on Linux;
the helper's `-health` URL follows (`https://`).
A restart from the API starts `massimo restart -service massimo` detached,
which stops and starts the service through the service manager;
`massimo restart` from an administrator prompt does the same.
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

The web UI starts updates too (System settings → Software,
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
- the settings `/etc/massimo/env` (without `-settings`, the template),
  mode 0600, owned by `_massimo`;
  the service starts by the rule of "Install with `massimo install`",
  the owner made first with
  `sudo -u _massimo /usr/local/bin/massimo user add <name> -role owner`;
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
the helper's health check follows it.
`-tls-cert` and `-tls-key` work as on Linux,
the files beside the settings owned by `_massimo`,
and the helper's health check follows (`https://`). macOS lets a user other than
root listen on a port below 1024 only on every address,
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

## Homebrew

On a Mac with Homebrew the tap `metaworx/massimo` carries a formula per update channel,
`massimo-alpha` for `alpha` (and `massimo` once there is a stable channel),
built from the release's source on the Mac, so that nothing needs notarization:

```sh
brew install metaworx/massimo/massimo-alpha
brew services start massimo-alpha
```

The formula builds `bin/massimo` with Go,
writes the settings template to `$(brew --prefix)/etc/massimo/env` when there is none,
keeps the store in `$(brew --prefix)/var/massimo` and the log in
`$(brew --prefix)/var/log/massimo.log`,
and its service runs `massimo serve -listen :8099` as you while you are logged in;
`sudo brew services start massimo-alpha` installs a LaunchDaemon that runs as
root at boot instead.
Nothing needs to go into the settings file for a start:
brew never starts the service by itself,
and until it has an owner every device on the local network may use it,
so create the owner in the web UI at once,
or before the start with
`massimo -data $(brew --prefix)/var/massimo user add <name> -role owner`;
the Cubes are added there too.
`brew upgrade` does not restart a running service;
`brew services restart massimo-alpha` does.

One owner of the binary: brew owns it,
and the marker under the formula's `share/massimo` tells massimo so.
`massimo update` says when a release is out and leaves it to
`brew upgrade massimo-alpha`; `massimo install` refuses,
since that is the layout of a Mac without Homebrew (on an Intel Mac both would
write to `/usr/local/bin`).

**Updates from the web UI.**
brew refuses to run as root, so there is no root helper:
under `brew services` as you, the server starts the helper itself,
this binary in its helper mode, as you and in a session of its own,
with the request and its last result in `$(brew --prefix)/var/massimo`
and its log in `$(brew --prefix)/var/log/massimo-update.log`.
It runs `brew update` (left out where `HOMEBREW_NO_AUTO_UPDATE` is set,
as `brew upgrade` leaves it out) and `brew upgrade massimo-alpha`,
which builds the release from source, so it takes minutes;
then it restarts the job with `launchctl kickstart -k`,
whose service runs the formula's `opt` binary, now the new one,
and waits until `/healthz` reports it.
Nothing is downgraded.
The channel is the formula's;
`MASSIMO_UPDATE_CHANNEL` naming another one keeps the button off, with the reason.
A server that `sudo brew services` runs as root,
or one started by hand, keeps the button off as well:
`brew upgrade massimo-alpha` and a restart of the service are then yours.
`brew uninstall massimo-alpha` removes it;
the settings under `etc/massimo` stay, as brew keeps them.

The formula is rendered from `homebrew/massimo.rb.in` by `homebrew/render.sh`
for each release,
conflicting with the tap's other formulae (all install `bin/massimo`),
and pushed to the tap with a deploy key by `.github/workflows/publish.yml`,
which the release workflow calls once the release is published;
CI's macOS job renders it for a tarball of the checkout,
builds and tests it, checks the marker,
and upgrades it to a second rendering through the update helper's command.
Until the owner has made the tap and the key, the workflow skips the push.

The owner's steps, once:

- Create the repository `metaworx/homebrew-massimo`;
  the workflow writes `Formula/<name>.rb`.
- Make a deploy key, `ssh-keygen -t ed25519 -f tap-deploy-key -N ''`:
  the public half as a deploy key of the tap with write access,
  the private half as the repository secret `HOMEBREW_TAP_DEPLOY_KEY` of this repository
  (GUIDELINES/project/CI.md, section 5).
- For the newest release, published before the tap existed:
  `gh workflow run publish.yml -f tag=<tag>`;
  every later release pushes its formula itself.

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
