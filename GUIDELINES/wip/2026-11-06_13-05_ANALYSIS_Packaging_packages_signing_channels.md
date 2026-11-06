# Analysis Packaging: packages, signing and channels

Written on 2026-11-06 from the code on `main` after v0.7.0-alpha.1 (Makefile,
.github/workflows/release.yml and ci.yml, internal/update, internal/install,
packaging/, cmd/massimo/install.go and update.go, internal/server/update_api.go,
docs/update-windows.md, docs/windows-test-2026-11-05.md, docs/design.md,
docs/roadmap.md item 9, GUIDELINES/project/CI.md),
from the latest CI and release runs on GitHub,
and from web research whose sources are dated in section 11.
It answers the questions of the PLAN request of 2026-11-06:
what a user gets and does per platform, who signs what with which key,
how a package-managed box and `massimo update` share one binary,
what `massimo install` becomes, the Raspberry Pi 1 in deb terms, LoxBerry,
what each channel costs, and the blocks with an effort each.
Nothing in it was run against hardware, and nothing here holds a key or a token.

Sections: 1 today, 2 the rules every option keeps, 3 Linux, 4 Windows,
5 macOS, 6 CI and the build, 7 what a user gets and does,
8 what each channel costs, 9 the blocks, 10 open decisions, 11 sources.

## 1. Today

**The release.**
`make release-alpha` tags `main`'s head `vX.Y.Z-alpha.N` and fast-forwards `alpha`;
the tag starts release.yml.
Its `build` job (ubuntu-latest,
2 min 41 s for v0.7.0-alpha.1 on 2026-11-06) runs `go vet`,
`go test` and `make release`:
seven static binaries
(`CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=<tag> -X main.channel=<channel>"`) for linux amd64, arm64, armv7 and armv6, darwin amd64 and arm64 and windows amd64, `SHA256SUMS`, and `massimo-release.json`, the manifest `mkmanifest` writes: tag, commit, channel, prerelease, the SHA-256 of every binary.
Its `sign` job (11 s) runs in the `signing` environment,
which only `v*` tags may use,
signs the manifest with the Ed25519 key `MASSIMO_RELEASE_KEY` through `openssl pkeyutl`,
verifies the signature against `internal/update/release-key.pem`,
the public key compiled into every binary, and publishes the release with `gh`.
The job holds the key and runs nothing but openssl and gh;
the third-party actions and the toolchain run in `build`, which holds no secret.
Each binary is 13 to 14 MB since the store brought modernc.org/sqlite;
a release is 95 MB of assets on GitHub.

**The channel** is read from the tag's name (`v0.7.0-alpha.1` is on `alpha`),
else from the `channel:` line of the tag message,
else from the one branch holding the commit;
it is compiled in as `main.channel`, and `MASSIMO_UPDATE_CHANNEL` overrides it.
`update.Latest` takes the newest release whose signed manifest names the channel.

**The updater.**
`massimo update` lists the releases through GitHub's API, verifies the manifest,
downloads the binary `update.AssetName()` names for the platform (`massimo-linux-armv6`,
from the `GOARM` the build recorded), checks its SHA-256 against the manifest,
runs `<new> version` as a smoke test,
keeps the running binary as `<binary>.prev`, renames the new one into place,
restarts the service and, with `-health`,
rolls back unless `/healthz` reports the new version.
The web UI's button writes a request file;
a systemd path unit starts the root helper on Linux,
launchd's `WatchPaths` on macOS,
and on Windows the server starts the helper itself as a detached process
(implemented 2026-11-06, docs/update-windows.md;
the real-PC test 7b is still owed).
`updateBlocked` in internal/server/update_api.go is where a reason not to update
is reported: no token, no request file, a development build, an invalid channel;
the web UI shows that text in place of the button.

**The installer.**
`massimo install` is the binary installing itself:
on Linux `/usr/local/bin/massimo`, the system user `massimo`,
`/etc/massimo/env` (0600, a commented template or a copy of `-settings`),
`massimo.service`,
`massimo-update.path` and `.service` in `/etc/systemd/system`,
the logrotate file, drop-ins for `-listen`, enable, restart;
on macOS the same layout with the hidden user `_massimo` and two LaunchDaemons;
on Windows `C:\Program Files\Massimo\massimo.exe`, `C:\ProgramData\Massimo\env`,
the service `massimo` as LocalSystem and a firewall rule.
`-remove` undoes it, `-purge` removes the settings, state, logs and user.
The service files are embedded from `packaging/` with `go:embed` and written
byte for byte.
Installing again from a newer binary replaces binary and units and keeps the settings.

**What a user sees today.**
On Windows a browser download carries the mark of the web,
and a double-click meets SmartScreen's "Windows protected your PC,
Unknown publisher" (docs/windows-test-2026-11-05.md, observation 8,
with the screenshot); a download with `curl.exe`,
the installed service and the updater are not affected.
On a Windows 11 PC with Smart App Control on,
Microsoft says an unsigned file is blocked wherever it comes from until it has a
reputation (Microsoft, 2026-05-04),
which no release of an unsigned project ever gets.
On macOS the binaries are ad-hoc signed, not notarized:
a browser download is refused by Gatekeeper until `xattr -d com.apple.quarantine`,
a `curl` download runs.
On Linux there is nothing to warn about; there is also nothing for `apt`.

**CI.**
Seven jobs on every push (run 42212243116 on 2026-11-06):
Vet and test with `make cross` 4 min, Screenshots 2 min 36 s,
Windows service 1 min 41 s, macOS LaunchDaemon 1 min 6 s,
govulncheck linux and windows about 20 s each, Guidelines 11 s.
The repository is public, so GitHub-hosted runners cost nothing in money,
macOS and Windows included; what a job costs is wall-clock time.

## 2. The rules every option keeps

1. **One binary per platform, the same bytes in every channel.**
   The raw binary on the release, the one inside the `.deb` and the rpm,
   the one the Windows installer carries and the one a Homebrew bottle would
   hold are the same file, so the signed manifest's SHA-256 covers all of them,
   and the updater's smoke test,
   swap and rollback keep working on a box where it still owns the binary.
   No build-time flag says "packaged": that is decided on the box (rule 3).
2. **The manifest covers every asset.**
   `mkmanifest` takes file names; the packages and the installer go in,
   so that a later block can let the helper verify and install a package,
   and so that a person can check any download against one signature.
   The manifest is made over the final files,
   after Windows code signing has changed the `.exe`.
3. **One owner of the binary per box, recorded on the box.**
   Either the package manager owns `/usr/bin/massimo` (apt, dnf, brew),
   or `massimo update` owns `/usr/local/bin/massimo` (`massimo install`),
   never both.
   A package ships a marker, `<dir of the binary>/../share/massimo/packaged`
   (`/usr/share/massimo/packaged` for the deb and the rpm,
   `Cellar/massimo/<version>/share/massimo/packaged` for Homebrew):
   its first line names the manager, its second the upgrade command.
   `massimo update` and `updateBlocked` read it, ahead of every other reason,
   and answer "managed by apt: sudo apt update && sudo apt install massimo";
   the web UI still says that a newer release is out and shows that command in
   place of the button.
   `massimo install` reads it too,
   at the absolute path as well as beside its own binary,
   so that a raw release binary run on a packaged box is refused as well,
   and keeps only what configures:
   `-settings` and `-listen` (a drop-in applies to a packaged unit as to any),
   then the restart; `-remove` points at `apt remove` and `apt purge`.
   On Windows the self-updater stays the owner (section 4.6),
   so there is no marker there.
4. **The key split stays.**
   Whatever builds the release runs without a signing key;
   the job that holds one runs nothing but the signer and `gh`.
   The apt repository's OpenPGP key joins the `signing` environment and is used
   in that job; Windows code signing happens on SignPath's side,
   with no key in CI at all.
5. **A package does what `massimo install` does,
   with the platform's own tools.**
   The user, the directories,
   enable and start are the package's maintainer scripts (adduser,
   deb-systemd-helper); the Go code does not run at install time.
   The units a package ships are the embedded ones with two differences,
   the binary's path (`/usr/bin/massimo`,
   since a package may not install into `/usr/local`) and no `-update-request`,
   derived at build time from `packaging/systemd/` by a script,
   with a test that the two differ in nothing else.
6. **Settings survive.**
   `/etc/massimo/env` is created by the maintainer scripts when missing
   and never overwritten,
   the store in `/var/lib/massimo` is untouched by upgrades,
   and a hand install converted to the package keeps both:
   `massimo install -remove` without `-purge` leaves them,
   and the package picks them up.
7. **Nothing reaches a real Cube from CI**, as today:
   the package tests run the maintainer scripts in a container against the fake Cube.

## 3. Linux

### 3.1 The `.deb`

Built by nfpm (v2.48.0, 2026-10-12),
a single static binary with one YAML and four scripts,
from the release binaries `make release` already builds; no `debian/` directory,
no source package, no Debian toolchain.
That is the practical route for one maintainer:
Debian proper would want every Go dependency packaged separately (policy 4.13
and the Go team's dh-golang), and an upstream-built deb is what headscale,
rclone, Tailscale, Syncthing and NetBird ship.

What the package holds, and what its scripts do:

| Path | What | Mode, owner |
|---|---|---|
| `/usr/bin/massimo` | the release binary, unchanged | 0755 root |
| `/usr/lib/systemd/system/massimo.service` | the embedded unit with `/usr/bin/massimo` and without `-update-request` | 0644 root |
| `/etc/logrotate.d/massimo` | the embedded file, a conffile | 0644 root |
| `/usr/share/massimo/packaged` | `apt`, then the upgrade command | 0644 root |
| `/usr/share/massimo/env.template` | the template `install` writes today | 0644 root |
| `/usr/share/doc/massimo/copyright`, `README.md` | licence and the README | 0644 root |

- `postinst` on configure:
  `adduser --system --group --home /var/lib/massimo --no-create-home --shell /usr/sbin/nologin massimo` (Debian policy 9.2.2 asks for adduser); `/etc/massimo` 0750 and `/etc/massimo/env` 0600, both `massimo:massimo`, the file copied from the template only when missing (a file created by the maintainer scripts, policy 10.7.4, so that no conffile prompt ever asks about secrets); then the debhelper snippets, `deb-systemd-helper enable massimo.service`, `systemctl daemon-reload` and `deb-systemd-invoke restart` only when systemd runs.
  The service is started only when the settings hold a `MASSIMO_` line,
  as `install` does today,
  so that a fresh box is not serving the LAN with no token;
  the message says what to fill in and `systemctl start massimo`.
  `/var/lib/massimo` and `/var/log/massimo` come from `StateDirectory=`
  and `LogsDirectory=`, as they do now.
- `preinst` refuses while `/etc/systemd/system/massimo.service` exists,
  with "a hand install is in place:
  sudo massimo install -remove keeps the settings,
  then install the package again";
  that is the migration path for the Pi on `alpha`, and it is tested.
- `prerm` stops the service on remove, `postrm purge` removes `/etc/massimo`,
  `/var/lib/massimo`, `/var/log/massimo` and the user,
  as `install -remove -purge` does; `apt remove` keeps them,
  as `install -remove` does.
- `Depends: adduser, init-system-helpers`; `Recommends: logrotate`.
- Version: nfpm writes `0.7.0~alpha.1` from the tag, which sorts before `0.7.0`;
  `massimo version` keeps printing the tag.
- Architectures: `amd64`, `arm64`,
  `armhf` from the GOARM=6 binary (section 3.4).
- Reproducible: nfpm takes `mtime` from `SOURCE_DATE_EPOCH`,
  set to the commit time.
- Tested in CI: a job installs the deb in a Debian container
  (`apt install ./massimo_*_amd64.deb`), checks the user, the modes, the marker,
  that `massimo install` refuses, that `massimo update` says "managed by apt",
  that a hand install blocks the package, and that `apt purge` leaves nothing;
  `deb-systemd-helper` tolerates a container without systemd.
- The box's own listen address:
  `systemctl edit massimo` with `Environment=MASSIMO_LISTEN=:80`
  and `AmbientCapabilities=CAP_NET_BIND_SERVICE`,
  which is exactly the drop-in `massimo install -listen :80` writes,
  and still writes on a packaged box.

### 3.2 The apt repository

**Where.**
GitHub Pages of the project itself,
deployed from the release workflow with `actions/upload-pages-artifact`
and `actions/deploy-pages` (no `gh-pages` branch,
no git history growing with binaries),
at `https://metaworx.github.io/massimo/apt/` (a custom domain can front it later).
The repository is stateless:
at each release the sign job downloads the packages of the last ten releases of
every channel from GitHub, runs `apt-ftparchive` over the pool,
signs `InRelease` with `gpg --clearsign`, and deploys the site.
Pages allows 1 GB and a soft 100 GB a month;
ten releases of three debs are about 150 MB,
and the pool holds several versions,
so `apt install massimo=0.6.0~alpha.1` is a rollback.

**Suites are the channels.**
`Suites: alpha`, later `beta` and `stable`, `Components: main`,
the same for every Debian and Ubuntu release since a static binary does not care
about the codename; `Origin: Massimo` and `Label: Massimo` in the Release file,
so that unattended-upgrades can be told `"origin=Massimo"` in its
`Origins-Pattern` (its default upgrades Debian's origins only).
A box picks its channel in its `.sources` file, which is the apt counterpart of
`MASSIMO_UPDATE_CHANNEL`, and `massimo update -check` keeps saying what is out.

**The key.**
A new OpenPGP key, RSA 4096, made by the owner offline,
with an expiry of two years (extended and republished,
the keyring package carrying the new copy);
the private half armored in `MASSIMO_APT_KEY` with `MASSIMO_APT_KEY_PASSPHRASE`
in the `signing` environment, used in the sign job only;
the public half committed as `packaging/apt/massimo-archive-keyring.asc`
and served by the site.
RSA 4096 rather than Ed25519: trixie's apt verifies with Sequoia's `sqv`,
bookworm's with `gpgv`,
Ed25519 works with both in practice but one sqv bug report rejected an Ed25519
signature (2025-09), and nothing is gained by the smaller key.
`apt-key` is gone in trixie,
so the snippet uses `Signed-By:` in a deb822 `.sources` file,
as Debian's wiki asks (2025-11-02):

```
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

A `massimo-archive-keyring` deb (nfpm,
ten lines) ships the key in `/usr/share/keyrings` and the `.sources` file,
so that a key rotation reaches every box through apt itself.

**The alternatives.**
Cloudsmith hosts open-source repositories free (50 GB, 200 GB a month,
no application,
attribution required) and would add rpm and more without any site of ours,
at the price of an account, a token in CI and a dependency on their terms;
Caddy uses it.
The openSUSE Build Service builds from source and has no armv6 builders and no
Raspbian trixie target; a Launchpad PPA is Ubuntu only, amd64 by default,
and rejects binary uploads.
Debs on the GitHub release alone (`apt install ./massimo_….deb`,
as headscale does) give no `apt upgrade`,
and then the self-updater would have to stay the owner of a dpkg-owned file,
which is the fight to avoid.

### 3.3 rpm

nfpm makes an rpm from the same configuration with rpm-flavoured scripts
(`getent`/`useradd`, `%systemd_post`-like calls) in about half a day,
and `dnf install ./massimo-0.7.0~alpha.1.x86_64.rpm` works on Fedora and openSUSE.
A dnf repository (`createrepo_c` plus a signature with the same key) is another half day,
and a COPR needs a spec file and builds from source.
Nobody has asked, and the target environments (design.md: Raspberry Pi,
LoxBerry, NAS, a Linux box) are Debian-shaped, so:
the rpm on the release once the deb exists, no repository until asked.

### 3.4 The Raspberry Pi 1 in deb terms

- Raspberry Pi OS (32-bit) on trixie, images of 2026-10-06, is "Compatible with:
  All Raspberry Pi models", the Pi 1 and the Zero included;
  the 64-bit image lists the 3, 4, 5, 400, 500,
  the compute modules and the Zero 2 W, and excludes the 2B, the Zero and the 1.
  The project's own measurement on a Pi 1 with trixie 32-bit (packaging/README.md,
  "Resource use") agrees.
- Its dpkg architecture is `armhf`, but it is built for ARMv6 with VFPv2,
  unlike Debian's `armhf`,
  which needs ARMv7 with VFPv3 (Debian wiki RaspberryPi; Raspbian FAQ;
  the Pi 64-bit announcement of 2022-02-02:
  "every device we have ever manufactured").
  Debian's `armel` would be the Pi 1's architecture under Debian proper,
  and trixie is the last release that has it.
- nfpm maps both GOARM 6 and GOARM 7 to `Architecture: armhf`,
  and GoReleaser's own page says so ("both ARMv6 and ARMv7 have the same
  architecture name: armhf");
  the closed issue goreleaser#2721 is a Raspbian box dying with "illegal
  instruction" on an armv7 deb.
  So the repository carries one `armhf` package, built from the GOARM=6 binary,
  which runs on the Pi 1 and Zero and on every 32-bit Pi after them.
  What GOARM=6 costs on an ARMv7 core, from Go's runtime sources:
  a kernel helper instead of `DMB` for memory barriers and compare-and-swap,
  and VFPv2 instead of VFPv3;
  for a daemon exchanging a few lines a second with a Cube, nothing measurable,
  though no source quantifies it.
- The raw `massimo-linux-armv7` stays a release asset:
  the updater on an existing armv7 hand install asks for it by name.

### 3.5 LoxBerry

LoxBerry 4.0.1.5 (2026-11-02) is a DietPi image on Debian 13 for new installs
(bookworm installs are still serviced), on the Pi 1 to 4,
other boards and x86_64 VMs; its plugin targets are `armv6l`, `armv7l`,
`aarch64`, `x86_64` and `riscv64`.
A plugin is a zip with `plugin.cfg`, an icon, a `webfrontend/htmlauth` page,
`preroot.sh`/`postroot.sh` run as root, `uninstall/uninstall` as root,
and a `dpkg/<uname -m>/` directory whose `.deb` files LoxBerry installs with `dpkg -i`;
`[AUTOUPDATE]` in `plugin.cfg` points at a `release.cfg` the box checks daily.
There is no registry: a plugin is installed by URL,
and a wiki page lists it. systemd units are common practice in plugins (the
author's own LMS4Lox plugin downloads a deb and enables its unit).

So a LoxBerry plugin is a thin wrapper around the deb:
`dpkg/armv6l/` and `dpkg/armv7l/` holding the armhf deb,
`dpkg/aarch64/` the arm64 one,
`dpkg/x86_64/` the amd64 one (or `postroot.sh` fetching them from the release,
as LMS4Lox does), a page linking to the web UI on port 8099,
`uninstall` running `apt purge massimo`,
and `release.cfg` generated at each release.
About a day and a half, once the deb exists and a LoxBerry user asks;
the Loxone integration of design.md section 4 is a separate, later matter,
and the plugin would then grow its virtual-output configuration.

### 3.6 NAS and containers

A Synology (DSM 7, x64 and aarch64 only) takes an SPK,
which SynoCommunity builds from source through a recipe someone maintains,
and shows "not verified by Synology" for every third-party package;
a QNAP takes a QPKG through the QDK or the Qnapclub store; neither has dpkg.
What a NAS does take is a container:
a `FROM scratch` image with the static binary and the CA certificates,
multi-arch (amd64, arm64, arm/v6, arm/v7),
pushed to ghcr.io with the `GITHUB_TOKEN`,
run with `--network host` (the proxy binds one address per Cube and discovery is
a broadcast,
which a bridge network breaks) and `/etc/massimo` and `/var/lib/massimo` as volumes.
The marker inside the image says `image` and "pull the next tag",
so the update button is off and the UI shows what is out.
About a day, and it covers Unraid and a Home Assistant add-on later;
it can wait.

### 3.7 What `massimo install` becomes, and the coexistence

On a hand-installed box nothing changes:
`install` and `update` stay as they are, the helper and the button included.
On a packaged box (rule 3):

| Command | Today | On a packaged box |
|---|---|---|
| `massimo install` | binary, user, settings, units, drop-ins, enable, restart | refuses the binary, user and units; copies `-settings` when the file is missing; writes the `-listen` drop-ins; `daemon-reload`, restart |
| `massimo install -remove [-purge]` | stops and removes everything | refuses, naming `apt remove massimo` and `apt purge massimo` |
| `massimo update`, the web UI's button | downloads, swaps, restarts, rolls back | "managed by apt: sudo apt update && sudo apt install massimo", the newer version shown, nothing installed |
| `apt upgrade` | not possible | replaces `/usr/bin/massimo`, restarts the service, keeps the settings and the store |

The three ways the two could share a box, and why the first is recommended:

- **A.
  The updater defers to the package manager** (above).
  Smallest change, one owner,
  apt's signature instead of the manifest's on that box;
  the web UI loses the button on packaged boxes and shows the command instead.
- **B.
  The helper runs the package manager.**
  The deb ships the path unit and a helper whose command is
  `apt-get install --only-upgrade massimo` when the marker says `apt`,
  `brew upgrade massimo` for Homebrew; the button works everywhere,
  apt stays the owner, the status file reports the steps.
  Lost: the `.prev` rollback and the health check as they are (apt has no rollback;
  `apt install massimo=<previous>` from the pool is the way back, by hand).
  A day on top of A; it can wait, and A is its first half.
- **C.
  The updater installs the package itself**:
  the helper downloads the `.deb` named in the manifest,
  verifies it and runs `dpkg -i`.
  Two signature systems for one file,
  dpkg's state written behind apt's back (`apt upgrade` then reinstalls the same version),
  and nothing apt does not do better.
  Not recommended.

The Pi on `alpha` today, with `-listen :80`:
`sudo massimo install -remove` (keeps `/etc/massimo/env`, the store,
the user and the log), the snippet of 3.2, `sudo apt install massimo`,
`sudo massimo install -listen :80` (allowed on a packaged box;
the drop-in was removed with the hand install), done.

## 4. Windows

### 4.1 Code signing through SignPath Foundation

What SignPath's terms and documentation say,
read on 2026-11-06 (sources in section 11):

- **Eligibility.**
  An OSI-approved licence without commercial dual-licensing,
  no proprietary component, actively maintained,
  already released in the form to be signed,
  a download page describing the program.
  Then "SignPath Foundation performs initial verification of project reputation
  and control",
  and for executables "we require a certain verifiable reputation",
  which the terms do not define,
  and acceptance is discretionary ("under no obligation to accept your project").
  In practice (mail archives and project wikis of 2026):
  a project was refused for lack of "articles, blog posts, forum discussions,
  mentions in open-source communities",
  another for too few "external public visibility signals";
  a Go tool with 361 stars (dnsmonster) is listed,
  and single-maintainer projects are accepted (one person as author,
  reviewer and approver).
  The risk for Massimo is not its size but its age and the absence of
  third-party mentions;
  a forum thread or two (the FHEM and LoxBerry communities,
  where the MAX! users are) and a documented download page are what the
  reviewers look for.
- **What the project provides.**
  The application form (repository, licence, download page, description,
  contact);
  a "Code signing policy" section in the README with the sentence "Free code
  signing provided by SignPath.io, certificate by SignPath Foundation",
  the team roles (authors, reviewers, approvers;
  one person may hold all three) and a privacy statement;
  multi-factor authentication on GitHub and SignPath for every team member;
  the product name equal to the project name and a consistent product version;
  and "Binaries must be verifiably built from source".
- **The build in CI.**
  The unsigned `.exe` is uploaded with `actions/upload-artifact` v4 (a zip);
  `signpath/github-action-submit-signing-request@v3` (v3.0,
  2026-09-10) submits it with the organisation id,
  the project and policy slugs and an API token,
  waits for completion (default 600 s, configurable;
  Pulse waits 2400 s) and downloads the signed artifact.
  SignPath checks that the build ran in a GitHub workflow on GitHub-hosted runners;
  re-runs can be forbidden by policy.
  Two signing policies: `test-signing` (a self-signed certificate, automatic,
  for pull requests) and `release-signing`,
  which the Foundation's terms make manual:
  "Every release needs manual approval for signing", by an approver, by e-mail,
  in practice the maintainer.
- **The certificate.**
  Measured on a signed release of frpmgr:
  subject `O=SignPath Foundation, CN=SignPath Foundation`,
  issued by GlobalSign (OV, policy 2.23.140.1.4.1),
  valid 2025-06-20 to 2027-09-07, timestamped by DigiCert,
  the key on SignPath's HSM.
  The project's name does not appear:
  Windows shows the publisher "SignPath Foundation".
  One certificate serves every project,
  which one maintainer declined it for (reputation shared with strangers).
- **File types.**
  PE files (`.exe`, `.dll`), MSI, MSIX, CAB, PowerShell,
  zip as a container with nested files, and also deb, rpm and OCI images.
  An Inno Setup installer is a PE file and is signed as one (frpmgr signs its `.exe`,
  its MSI and its `setup.exe`); an MSI can carry nested `pe-file` entries,
  so one request signs the bare `.exe`, the `.exe` inside the MSI and the MSI.
- **Turnaround and cost.**
  Free.
  The application took days to weeks for the projects that wrote about it,
  "weeks to months" for one;
  a signing request takes as long as the approval click.
  No quota was found.
- **SmartScreen.**
  Microsoft (2026-05-04):
  reputation is per publisher certificate and per file hash;
  a signed binary is "flagged as unrecognized until reputation accumulates",
  with the publisher name shown,
  and it "can take several weeks and hundreds of clean installs";
  EV certificates no longer bypass it (since 2024),
  and nothing can be submitted by hand.
  SignPath's certificate has been in use across many projects since mid-2025,
  so publisher reputation exists; the file hash is new at every release.
  Projects report a softer warning "at first" and none reports a lasting block;
  the only hard evidence either way is Microsoft's own table.
  What signing certainly changes: the dialog names a publisher,
  each release inherits the certificate's reputation instead of starting from nothing,
  and Smart App Control stops blocking the file outright.

### 4.2 The alternatives

| Route | Money | Who may | In CI | Read |
|---|---|---|---|---|
| Azure Artifact Signing (ex Trusted Signing, GA 2026-01-12) | 9.99 USD a month plus an Azure subscription | organisations with three years of verifiable tax history in listed countries (Switzerland on one of two Microsoft pages); individuals only in the USA and Canada | a GitHub Action, 3-day certificates re-issued daily, no hardware | 2026-05-21, 2026-10-08 |
| Certum Open Source Code Signing | from 69 EUR (card and reader) or 49 EUR (cloud), both out of stock when read | individuals, with identity documents; the certificate says "Open Source Developer" | a card reader, or a phone app; automated only by unofficial tricks | 2026-11-06 |
| An OV certificate from a CA | 150 to 300 USD a year, plus a hardware token or a cloud HSM (required since 2023-06-01) | organisations | a cloud-HSM subscription or a self-hosted runner with a token | 2026-03-25 |
| Self-signed | 0 | anyone | trivial | "same behavior as no signature" (Microsoft) |

SignPath is the one route that is free,
works from GitHub-hosted runners and keeps no key in CI;
Artifact Signing is the fallback if metaworx qualifies as an organisation
and SignPath declines.

### 4.3 The installer

What an installer adds to a signed `.exe` that installs itself:
an entry in Programs and Features,
`msiexec /qn` or `/VERYSILENT` for unattended installs,
an uninstaller a Windows user looks for, and a package type winget prefers.
What it must not do is register the service a second way:
`massimo install` already creates the service, the ACLs,
the recovery actions and the firewall rule,
and a WiX `ServiceInstall` or an Inno `[Run]` doing the same would drift.
So the installer owns the files and calls the binary for the service,
through a new `massimo install -service-only`,
which neither copies the binary nor, on `-remove`,
deletes the program folder (the installer does both).

| | WiX MSI | Inno Setup | MSIX | winget `portable` only |
|---|---|---|---|---|
| Version read | v7.0.1 (2026-10-26); `dotnet tool`; the EULA must be accepted from v7, free below 10 000 USD revenue | 7.1.1 (2026-10-28); free for non-commercial use | — | manifest schema 1.12 |
| Builds on | a Windows runner only (Linux explicitly not planned) | Windows (Wine images pin Inno 6) | — | nothing to build |
| The service | a deferred custom action `massimo install -service-only`, and `-remove` before `RemoveFiles` | `[Run]` and `[UninstallRun]` with the same commands | `desktop6:Service`, but the package must be signed with a certificate the machine trusts: not viable | the user runs `massimo install` as today |
| SignPath | one request: MSI with the nested `.exe`, and the bare `.exe` beside it | two requests per release: the `.exe`, then the installer built from it | — | one request |
| winget type | `wix` | `inno` | `msix` | `portable`, a command alias on PATH |
| Effort | 3 days | 2 days | — | half a day |

Recommended: WiX, when a beta or stable channel exists and the winget channel is
opened with it,
because it is one approval click per release and the MSI is what winget
and administrators expect;
until then the signed `.exe` and `massimo install` as documented today,
which the test report found works,
and whose SmartScreen dialog is what signing addresses.
Inno Setup is the smaller script and the fallback if WiX's custom action gives trouble.

### 4.4 Channels: winget, Scoop, Chocolatey

- **winget** (microsoft/winget-pkgs):
  a manifest per version with the installer's URL on the release (it "must be
  the ISV's release location") and its SHA-256,
  validated by running the installer unelevated in isolation with antivirus scans;
  silent install required; signatures are checked for MSIX only;
  the pull request "may take several cycles".
  `wingetcreate update --submit` in a workflow on `release: published` automates it;
  GoReleaser's winget publisher covers zip archives in its free edition,
  MSI in Pro.
  A `portable` manifest for the bare `.exe` is possible today
  (`winget install metaworx.massimo`, `massimo` on PATH,
  then `massimo install` as administrator),
  but winget-pkgs is not a place for alphas,
  and a known case (vim, 2023) shows an unsigned winget install failing silently
  on SmartScreen until the builds were signed.
  So: winget with the MSI, when a beta or stable channel exists.
- **Scoop**:
  the main bucket wants 500 stars;
  an own bucket (`metaworx/scoop-massimo`) is a JSON file with autoupdate from
  the checksums file, updated every four hours by Scoop's workflow.
  It installs the portable `.exe` into the user's profile,
  which is not what the service runs,
  so `massimo install` is still the step that matters; half a day, after winget,
  if anyone asks.
- **Chocolatey**:
  human moderation of every version, days to weeks;
  not worth it for a service installer.

### 4.5 SmartScreen, winget and the mark of the web

A browser download gets the mark; `curl.exe`,
Go's HTTP client (the updater) and winget's downloads of a trusted source do
not meet SmartScreen the same way,
and the service manager never consults it (test report, observation 8).
Signing changes the dialog for the download that a person double-clicks,
and removes the Smart App Control block.

### 4.6 The self-updater on Windows

The updater stays the owner of `C:\Program Files\Massimo\massimo.exe`:
it is tested on every push with a rollback,
Windows has no package manager that upgrades unattended,
and the installer is a bootstrap.
After an installer-based install, a self-update leaves one thing stale:
the version Programs and Features and winget show,
which comes from the registry's `DisplayVersion`;
the helper rewrites it after a successful update
(golang.org/x/sys/windows/registry is already a dependency),
and a later MSI's major upgrade removes the old product and installs the new one
whatever the file on disk is.
The alternative, the installer as the only updater (the helper downloads the MSI
named in the manifest and runs `msiexec /qn`),
keeps the registry honest by itself but replaces the tested swap and rollback
with msiexec's, and is not recommended.

## 5. macOS

### 5.1 Gatekeeper today

Sequoia removed the Control-click override (Apple, 2024-08-06):
an unsigned or unnotarized program from a browser is refused,
and the way through is System Settings > Privacy & Security > Open Anyway.
Tahoe (26) goes further:
in a test of 2026-08-11 an unsigned app was blocked in every condition,
while an ad-hoc signed, unquarantined binary "launched with no warning".
Gatekeeper acts on the quarantine attribute,
which `curl` does not set and Homebrew does not set for formula builds and bottles.
So the documented route (`curl`,
then `sudo ./massimo-darwin-arm64 install`) keeps working,
a browser download keeps needing `xattr -d`,
and an unnotarized `.pkg` would be refused like any other download.

### 5.2 Homebrew

A tap (`metaworx/homebrew-massimo`) with a formula that builds from source:
`depends_on "go" => :build`,
`system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=v#{version} -X main.channel=#{channel}")`, the marker written to `share/massimo/packaged` ("brew", "brew upgrade massimo"), a `service do` block (`massimo -env #{etc}/massimo/env -data #{var}/massimo serve -listen :8099`, `keep_alive true`, the log under `var/log`), and a test that starts the fake Cube and runs `massimo status -addr` against it.
Brew verifies the source tarball's SHA-256 and builds locally,
so nothing needs notarization, and the binary is not quarantined.
`brew services start massimo` runs it as the logged-in user from `~/Library/LaunchAgents`,
which stops when nobody is logged in;
`sudo brew services start massimo` installs a LaunchDaemon that runs as root
(`--sudo-service-user` could name `_massimo`,
but creating that user stays `massimo install`'s job).
The caveats say so, and that `massimo install` is the hand-install route for a
`_massimo` daemon from the raw binary, not to be mixed with brew (rule 3;
on an Intel Mac brew's prefix is `/usr/local`,
where `massimo install` would overwrite brew's symlink).
A workflow on `release: published` bumps `url`,
`sha256` and `version` in the tap with a deploy key.
While only `alpha` exists the formula `massimo` follows it;
with a stable channel `massimo` follows stable and `massimo-alpha` the channel.

Not recommended: GoReleaser's Homebrew support,
which has moved from formulas to casks of prebuilt binaries ("hackyish formulas",
deprecated since v2.10); casks are quarantined,
so unsigned binaries need notarization or a post-install `xattr` hook that
Homebrew 5.0 (2025-11-12) is removing from its own casks by September 2026.
Bottles for the tap (built by `brew test-bot` on GitHub's macOS runners,
hosted on GitHub releases) save the user the Go build; a day, later,
if the build time annoys anyone. homebrew-core wants 75 stars or 30 forks and a
repository older than 30 days; not now.

### 5.3 A `.pkg` with notarization

The Apple Developer Program costs 99 USD a year (CHF 109 in the Swiss listing),
individuals may join,
and the Developer ID Application and Installer certificates come with it.
The pkg's `postinstall` runs as root and would call `massimo install -service-only`;
the binary is signed with the hardened runtime and a timestamp,
the pkg with the Installer certificate,
`notarytool` submits it (most within five minutes,
98 percent within fifteen) and the ticket is stapled.
It needs a macOS runner for `pkgbuild`, `productsign` and `notarytool`,
and five secrets (the certificate as a p12 with its password,
an App Store Connect API key with its id and issuer).
Then a Mac user double-clicks a file and Gatekeeper says nothing.
Two days plus the fee;
it waits until there are Mac users who are not on Homebrew,
and the ad-hoc signed `curl` route holds meanwhile.

### 5.4 The updater on a Mac

A hand install keeps the self-updater, as today;
a brew install defers to brew through the marker, as apt does on Linux.

## 6. CI and the build

### 6.1 GoReleaser, or nfpm and a few steps

GoReleaser v2.20.0 (2026-11-04) does in one YAML what the Makefile
and release.yml do and more: builds, archives, checksums, nfpm packages,
the GitHub release, winget and Scoop manifests, SBOMs.
Two things stop it fitting here:

- **The key split.**
  GoReleaser builds, signs and publishes in one process;
  a signature step inside it (`signs:` with an `openssl` command) puts the
  release key into the environment of the job that runs the toolchain and the
  third-party actions, which release.yml was written to avoid.
  Splitting the build from the publish across jobs is the `split`/`merge` feature,
  Pro only (165 USD a year), as is importing prebuilt binaries.
  Running it with a draft release and finishing in the sign job is possible,
  but then its publishers (winget, Scoop,
  Homebrew) run before the release is public and point at a draft.
- **Homebrew.**
  Its formulas are deprecated in favour of casks of prebuilt binaries,
  which need notarization (5.2).

So: **nfpm and a few steps.**
`make packages` after `make release` (nfpm over `dist/`,
with the derived unit and the scripts under `packaging/deb/`),
the apt publishing step in the sign job,
the SignPath job before the manifest is made,
and a `publish.yml` on `release: published` for the tap bump and, later, winget.
Each step is a script the project already reads and reviews,
and `make release` still shows locally what a release holds.
GoReleaser remains the right answer the day the key split is given up or Pro is bought.

### 6.2 The manifest and the channels

The manifest needs two things from all of this:
to be made over the final files (so the SignPath job runs before `mkmanifest`),
and to list every asset, the packages and the installer included,
which it does by taking file names.
The channel flows unchanged: the tag name gives it, the build compiles it in,
the manifest states it, the apt suite is named after it,
and the tap formula is named after it when a second channel exists.
The release notes' line ("Install or update with `massimo update`") gains the
apt snippet and the brew command.

The release workflow, in order:

| Job | Runner | Needs | Holds | Time |
|---|---|---|---|---|
| build | ubuntu | — | nothing | 3 min (today 2 min 41 s; nfpm adds seconds) |
| installer (block 6) | windows | build | nothing | 2 min, WiX |
| windows-sign (block 3) | ubuntu | build, installer | the SignPath API token | upload, the owner's approval, download: minutes, bounded by a timeout of 40 min |
| sign | ubuntu, `signing` environment | windows-sign | the release key, the apt key | 1 min (today 11 s; the apt rebuild and the Pages deploy add the rest) |
| publish.yml | ubuntu, on `release: published` | the release | a deploy key for the tap, later a winget token | 1 min |
| notarize (block 10) | macos, `signing` environment | build | the Apple certificate and API key | 3 to 15 min |

The `sign` job stays the only one that runs with a key of ours,
and still runs only openssl, gpg, apt-ftparchive and gh.
The Windows CI job gains a check that the newest alpha's `.exe` carries a
signature (`Get-AuthenticodeSignature`), once one does.

### 6.3 Reproducible builds

The Makefile already has the three that matter: `CGO_ENABLED=0`, `-trimpath`,
and a version from `-X` rather than from the build host.
`-buildvcs` stays on auto:
on a clean checkout of a tagged commit the stamped revision,
time and modified flag are constants,
and `massimo version` without the Makefile relies on them.
What drifts is the toolchain:
`setup-go` with `check-latest: true` takes the newest 1.26.x on the day,
so a rebuild months later may differ by a patch release;
`massimo version` prints the toolchain (`go1.26.10`),
and the manifest should record it too (one field in `mkmanifest`),
so that a rebuild can pin it. nfpm's `mtime` from `SOURCE_DATE_EPOCH` (the
commit time) makes the deb and rpm byte-stable as well.
GitHub's build provenance attestation (`actions/attest-build-provenance`) is a
cheap addition in the build job and needs no key; optional.

### 6.4 Secrets

| Secret | Where | Used by | Whose | Block |
|---|---|---|---|---|
| `MASSIMO_RELEASE_KEY` | `signing` environment | `sign` | exists | — |
| `MASSIMO_APT_KEY`, `MASSIMO_APT_KEY_PASSPHRASE` | `signing` environment | `sign`, the apt step | the owner makes the key offline; public half in the repository | 2 |
| `SIGNPATH_API_TOKEN`; organisation id and project slug as variables | repository secret, scoped to the release workflow | `windows-sign` | SignPath issues it to the project's CI user | 3 |
| `HOMEBREW_TAP_DEPLOY_KEY` | `signing` environment or `publish.yml` | the tap bump | an SSH deploy key with write access to the tap repository | 4 |
| `WINGET_TOKEN` | `publish.yml` | `wingetcreate` | a fine-grained token for the fork of winget-pkgs | 6 |
| `APPLE_CERT_P12`, `APPLE_CERT_PASSWORD`, `APPLE_API_KEY`, `APPLE_API_KEY_ID`, `APPLE_API_ISSUER` | `signing` environment | `notarize` | the owner's Apple account | 10 |
| `GITHUB_TOKEN` | provided | `pages: write` and `id-token: write` for the Pages deploy, `packages: write` for ghcr.io | — | 2, 8 |

No key is ever in the repository;
the public halves are (`release-key.pem` today, the apt key's `.asc` then).
Every row goes into GUIDELINES/project/CI.md section 5 with its block.

### 6.5 What CI costs

Nothing in money: the repository is public.
In time: a release today takes 3 min from tag to published;
with the deb, the apt rebuild and the Pages deploy about 4 min;
with SignPath, plus the minutes until the owner approves;
with the MSI, plus 2 min on a Windows runner;
with notarization, plus 3 to 15 min on a macOS runner.
Every push's CI gains one container job for the deb (about a minute).

## 7. What a user gets and does

| Platform | Install | Update | Remove | Who signs |
|---|---|---|---|---|
| Linux, Debian family (Raspberry Pi OS, Debian, Ubuntu, LoxBerry) | the `.sources` snippet and the key, `apt install massimo`; or `apt install ./massimo_*.deb` from the release | `apt upgrade` (unattended-upgrades with `origin=Massimo`); the UI shows the newer version and the command | `apt remove massimo`; `apt purge massimo` takes settings, store, logs and user | the apt key on `InRelease`; the manifest's Ed25519 key covers the deb on the release |
| Linux, hand install (any systemd box) | as today: the raw binary, `sudo ./massimo-linux-armv6 install -settings .env` | as today: `massimo update` or the button, with rollback | `massimo install -remove [-purge]` | the manifest's Ed25519 key |
| Linux, rpm family | `dnf install ./massimo-*.rpm` from the release | the next rpm by hand; the UI shows the newer version | `dnf remove massimo` | the manifest; the rpm itself with the apt key, optionally |
| NAS, Unraid, a container host | `docker run --network host ghcr.io/metaworx/massimo` with two volumes | pull the next tag; the UI shows the newer version | remove the container | the manifest; a cosign signature on the image, optionally |
| Windows | the signed MSI (double-click, or `winget install`), which runs `massimo install -service-only`; or the signed `.exe` and `massimo install` as today | as today: the button or `massimo update`, with rollback; `winget upgrade` runs the new MSI | Programs and Features, `winget uninstall`, or `massimo install -remove [-purge]` | SignPath Foundation's certificate on the `.exe` and the MSI; the manifest |
| macOS, Homebrew | `brew install metaworx/massimo/massimo`, `brew services start massimo` (as the user) or `sudo brew services start massimo` | `brew upgrade massimo`; the UI shows the newer version and the command | `brew services stop massimo`, `brew uninstall massimo` | nobody: brew checks the source tarball's SHA-256 and builds locally |
| macOS, hand install | as today: the raw binary (`xattr -d` after a browser download), `sudo ./massimo-darwin-arm64 install -settings .env` | as today, with rollback | `massimo install -remove [-purge]` | ad-hoc today; Developer ID and notarization with the `.pkg` later |

## 8. What each channel costs

| Channel | Money | Release time | Per release, by hand | Developer days |
|---|---|---|---|---|
| GitHub release (today) | 0 | 3 min | nothing | 0 |
| `.deb` on the release | 0 | +10 s | nothing | 2 |
| apt repository on GitHub Pages | 0 | +1 min | nothing; the key's expiry every two years | 1.5 |
| SignPath signing | 0 | +the approval wait | one approval click (two with Inno) | 1, plus the application's calendar time |
| Homebrew tap, from source | 0 | +10 s | nothing | 1 |
| rpm on the release | 0 | +10 s | nothing | 0.5 |
| WiX MSI and winget | 0 | +2 min | nothing; winget's review of the automated pull request | 3 |
| updates through apt or brew from the button | 0 | 0 | nothing | 1 |
| container image on ghcr.io | 0 | +1 min | nothing | 1 |
| LoxBerry plugin | 0 | +10 s | nothing | 1.5 |
| `.pkg` with notarization | 99 USD a year | +3 to 15 min | nothing; the certificate's renewal every five years | 2 |

## 9. The blocks

In the recommended order, with the effort in developer days;
"waits" means the block is not started until the condition holds.

1. **The `.deb`** (2 days): nfpm configuration, the four scripts,
   the derived unit with its drift test,
   the marker and its readers (`updateBlocked`, `massimo update`,
   `massimo install` in configure-only mode), `make packages`,
   the packages in release.yml and in the manifest, the container job in ci.yml,
   packaging/README.md ("Debian package") and README.md.
2. **The apt repository** (1.5 days): the key (the owner),
   `packaging/apt/` with the publishing script (the pool from the last ten
   releases per channel, `apt-ftparchive`, `InRelease`),
   the Pages deploy in the sign job, the keyring package,
   the `.sources` snippet and the unattended-upgrades line in the docs,
   a CI test that installs from a `file://` copy of the generated repository,
   CI.md section 5.
3. **Windows code signing through SignPath** (1 day,
   and the application's weeks, so the owner applies first):
   the README's code signing policy section, the SignPath project,
   policies and artifact configuration,
   the `windows-sign` job before the manifest,
   a signature check in the sign job and in the Windows CI job,
   packaging/README.md ("Windows").
4. **The Homebrew tap** (1 day): the tap repository (the owner),
   the formula with the service block, the marker and the test,
   the bump workflow with its deploy key, README.md ("macOS").
5. **The rpm** (0.5 day, waits for a request): nfpm's rpm section, the scripts,
   a Fedora container test, the docs.
6. **The Windows MSI and winget** (3 days, waits for a beta or stable channel):
   `install -service-only`, the WiX project with its custom actions,
   the Windows runner job, the MSI in the SignPath request,
   the Windows CI job installing and removing through msiexec,
   `wingetcreate` in publish.yml, the docs.
7. **Updates through apt or brew from the button** (1 day,
   waits for block 2 and a wish for it): the helper units in the deb,
   the helper running the package manager when the marker says so,
   the status file, tests against a local repository.
8. **The container image** (1 day, waits for a NAS user): the Dockerfile,
   the multi-arch push, the marker, the docs.
9. **The LoxBerry plugin** (1.5 days, waits for block 1 and a LoxBerry user):
   the plugin skeleton,
   the per-architecture deb directories or the fetching `postroot.sh`, the page,
   `release.cfg` at each release, the wiki page.
10. **The `.pkg` with notarization** (2 days and 99 USD a year,
    waits for Mac users off Homebrew): the Apple account (the owner),
    the macOS job, the five secrets, the pkg, the docs.

Blocks 1 to 4 are the first wave, 5.5 developer days;
block 3's application should be sent before block 1 starts,
since its calendar time is the longest.

## 10. Open decisions

- **D1.
  The build tooling.**
  - **A (recommended).**
    nfpm with the Makefile and release.yml as they are:
    keeps the key split and the reviewed scripts; every channel is one step.
  - **B.**
    GoReleaser OSS in one job: the release key in the toolchain's job,
    Homebrew only as casks.
  - **C.**
    GoReleaser Pro (165 USD a year) with split builds: the key split kept,
    a paid dependency.
- **D2.
  The updater on a packaged box.**
  - **A (recommended).**
    The marker; the updater and the UI show the newer version and the manager's command,
    install nothing.
  - **B.**
    The helper runs apt or brew from the button: block 7, a day on top of A,
    no rollback.
  - **C.**
    The helper installs the deb itself: two signers for one file,
    dpkg behind apt's back.
- **D3.
  Where the apt repository lives.**
  - **A (recommended).**
    The project's GitHub Pages,
    rebuilt from the last ten releases per channel at each release; no state,
    no account.
  - **B.**
    Cloudsmith's free open-source tier: rpm and more for free, an account,
    a token and their terms.
  - **C.**
    Debs on the release only: no `apt upgrade`,
    and the updater would have to own a dpkg-owned file.
- **D4.
  The repository key.**
  - **A (recommended).**
    A new RSA 4096 OpenPGP key, two-year expiry, in `signing` with a passphrase:
    accepted by every apt in the field.
  - **B.**
    Ed25519 OpenPGP: smaller, one sqv bug report against it, nothing gained.
- **D5.
  Windows signing.**
  - **A (recommended).**
    SignPath Foundation, applied for now: free, no key in CI,
    the publisher reads "SignPath Foundation".
  - **B.**
    Azure Artifact Signing: 10 USD a month,
    needs metaworx as an organisation with three years of tax history;
    the publisher would read metaworx.
  - **C.**
    Certum's open-source certificate: 69 EUR, individuals only, CI by tricks.
  - **D.**
    None: the SmartScreen dialog stays, Smart App Control blocks.
- **D6.
  The Windows installer.**
  - **A (recommended).**
    WiX MSI with a custom action calling `install -service-only`, with winget,
    when a beta or stable channel exists: one approval per release.
  - **B.**
    Inno Setup now: a smaller script, two approvals per release,
    alphas on winget.
  - **C.**
    A winget `portable` manifest of the bare `.exe` now: half a day,
    alphas on winget, the user still runs `massimo install`.
- **D7.
  Homebrew.**
  - **A (recommended).**
    A tap with a formula built from source, `brew services`,
    the updater deferring: no notarization needed.
  - **B.**
    A plus bottles built by `brew test-bot`: a day more,
    no Go build for the user.
  - **C.**
    A cask of the prebuilt binaries: needs notarization,
    or the quarantine hook Homebrew is removing.
- **D8.
  What waits.**
  - **A (recommended).**
    Blocks 5 to 10 wait for their conditions in section 9;
    the first wave is blocks 1 to 4.
  - **B.**
    The container image (8) in the first wave: a day, and a NAS answer today.
  - **C.**
    The `.pkg` (10) in the first wave: 99 USD a year and two days now.

## 11. Sources

Every page read on 2026-11-06; a date in brackets is the page's own.

SignPath and Windows signing: https://signpath.org/terms;
https://signpath.io/product/open-source (2026-11-05);
https://docs.signpath.io/trusted-build-systems/github;
https://docs.signpath.io/projects;
https://docs.signpath.io/artifact-configuration/reference;
https://github.com/signpath/github-action-submit-signing-request (v3.0,
2026-09-10);
https://www.mail-archive.com/denemo-devel@gnu.org/msg12066.html (2026-03 to 2026-04);
https://super-productivity.com/code-signing/ (2026-01-15);
https://raw.githubusercontent.com/rcourtman/Pulse/main/docs/CODE_SIGNING_POLICY.md;
https://github.com/koho/frpmgr (its README and releaser.yml;
the certificate measured on its v1.27.2 setup);
https://github.com/orgs/LegacyUpdate/discussions/442 (2026-03-18);
https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation
(2026-05-04);
https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options
(2026-08-29);
https://learn.microsoft.com/en-us/azure/artifact-signing/quickstart (2026-05-21);
https://azure.microsoft.com/en-us/products/trusted-signing;
https://shop.certum.eu/open-source-code-signing.html;
https://www.ssl.com/guide/esigner-pricing-for-code-signing/ (2026-03-25).

Windows installers and channels:
https://github.com/wixtoolset/wix/releases (v7.0.1, 2026-10-26);
https://docs.firegiant.com/wix/osmf/;
https://github.com/wixtoolset/issues/issues/4381;
https://jrsoftware.org/isdl.php (7.1.1, 2026-10-28);
https://jrsoftware.org/ishelp/topic_runsection.htm;
https://learn.microsoft.com/en-us/uwp/schemas/appxpackage/uapmanifestschema/element-desktop6-service (2026-10-12); https://learn.microsoft.com/en-us/windows/msix/package/signing-package-overview (2026-04-14); https://learn.microsoft.com/en-us/windows/package-manager/package/manifest (2026-09-14); https://github.com/microsoft/winget-pkgs/blob/master/doc/Validation.md; https://github.com/microsoft/winget-create; https://github.com/vim/vim-win32-installer/issues/319; https://github.com/ScoopInstaller/Scoop/wiki/Criteria-for-including-apps-in-the-main-bucket; https://docs.chocolatey.org/en-us/community-repository/moderation/.

macOS: https://developer.apple.com/support/enrollment/;
https://apps.apple.com/ch/app/apple-developer/id640199958 (CHF 109);
https://developer.apple.com/help/account/reference/certificate-types;
https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution; https://developer.apple.com/documentation/security/customizing-the-notarization-workflow; https://developer.apple.com/news/?id=saqachfa (2024-08-06); https://support.apple.com/en-euro/102445 (2026-05-27); https://eclecticlight.co/2026/08/11/how-can-you-run-code-that-hasnt-been-notarised/ (2026-08-11); https://developer.apple.com/forums/thread/729336; https://docs.brew.sh/Formula-Cookbook; https://docs.brew.sh/Language-Specific-Formulae; https://docs.brew.sh/Manpage; https://docs.brew.sh/Package-Acceptance-Policy; https://brew.sh/2025/11/12/homebrew-5.0.0/ (2025-11-12); https://goreleaser.com/customization/homebrew_casks/; https://goreleaser.com/deprecations/; https://goreleaser.com/customization/notarize/.

Linux, GoReleaser, nfpm, apt:
https://github.com/goreleaser/goreleaser/releases/tag/v2.20.0 (2026-11-04);
https://goreleaser.com/pro/;
https://goreleaser.com/customization/nfpm/;
https://goreleaser.com/customization/prebuilt/;
https://goreleaser.com/customization/sign/;
https://nfpm.goreleaser.com/docs/configuration;
https://nfpm.goreleaser.com/docs/arch-mapping;
https://github.com/goreleaser/goreleaser/issues/2721 (2021-12);
https://github.com/goreleaser/nfpm/issues/734 (2023-12);
https://www.debian.org/doc/debian-policy/ch-opersys.html (9.2.2, 9.3);
https://www.debian.org/doc/debian-policy/ch-source.html (4.13);
https://go-team.pages.debian.net/packaging.html;
https://manpages.debian.org/trixie/debhelper/dh_installsystemd.1.en.html;
https://manpages.debian.org/trixie/apt/sources.list.5.en.html;
https://manpages.debian.org/trixie/apt/apt-secure.8.en.html;
https://wiki.debian.org/DebianRepository/UseThirdParty (2025-11-02);
https://packages.debian.org/trixie/apt (sqv);
https://salsa.debian.org/apt-team/apt/-/raw/main/debian/default-sequoia.config;
https://gitlab.com/sequoia-pgp/sequoia-sqv/-/issues/18 (2025-09);
https://raw.githubusercontent.com/mvo5/unattended-upgrades/master/data/50unattended-upgrades.Debian;
https://docs.github.com/en/pages/getting-started-with-github-pages/github-pages-limits;
https://github.com/morph027/apt-repo-action (v3.10, 2026-11-02);
https://docs.cloudsmith.com/open-source-hosting-policy;
https://packagecloud.io/pricing;
https://build.opensuse.org/repositories/Debian:13;
https://ubuntu.com/docs/launchpad/user/reference/packaging/ppas/ppa/index.html;
https://headscale.net/stable/setup/install/official/;
https://pkgs.tailscale.com/stable/;
https://apt.syncthing.net/;
https://caddyserver.com/docs/install;
https://smallstep.com/docs/step-cli/installation/;
https://docs.copr.fedorainfracloud.org/user_documentation.html;
https://go.dev/blog/rebuild (2023-08-28);
https://pkg.go.dev/cmd/go (GOARM, -buildvcs, -trimpath);
https://github.com/actions/setup-go;
https://endoflife.date/debian;
https://endoflife.date/ubuntu.

Raspberry Pi, LoxBerry, NAS:
https://www.raspberrypi.com/software/operating-systems/ (images of 2026-10-06);
https://www.raspberrypi.com/news/trixie-the-new-version-of-raspberry-pi-os/ (2025-10-02);
https://wiki.debian.org/RaspberryPi (2022-05-31);
https://www.raspbian.org/RaspbianFAQ;
https://www.raspberrypi.com/news/raspberry-pi-os-64-bit/ (2022-02-02);
https://www.debian.org/releases/trixie/release-notes/issues.en.html (5.1.3,
armel); https://go.dev/wiki/GoArm;
https://raw.githubusercontent.com/golang/go/master/src/runtime/sys_linux_arm.s;
https://github.com/mschlenstedt/Loxberry/releases (4.0.1.5, 2026-11-02);
https://wiki.loxberry.de/installation_von_loxberry/die_installation_von_loxberry/start
(2026-06-05);
https://wiki.loxberry.de/entwickler/plugin_fur_den_loxberry_entwickeln_ab_version_1x/start; https://wiki.loxberry.de/haufig_gestellte_fragen_faq/automatische_und_manuelle_plugin_updates (2022-09-10); https://github.com/mschlenstedt/LoxBerry-Plugin-LMS4Lox/blob/master/postroot.sh; https://docs.synocommunity.com/developer-guide/package-types/go/; https://docs.synocommunity.com/reference/architectures/; https://docs.synocommunity.com/user-guide/installation/; https://github.com/qnap-dev/QDK; https://www.qnap.com/en/software/container-station; https://github.com/Entware/Entware/wiki/Install-on-Synology-NAS.

The CI numbers are from `gh run view 42212243116` and `gh run view 42152771989`,
the asset sizes from `gh release view v0.7.0-alpha.1`, on 2026-11-06.
