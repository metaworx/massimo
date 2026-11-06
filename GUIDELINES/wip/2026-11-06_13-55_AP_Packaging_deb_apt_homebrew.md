# AP Packaging v1.0: deb, apt repository, Homebrew tap

## TL;DR

Massimo gets packages beside its raw binaries, with one rule:
one owner of the binary per box, recorded by a marker the package ships,
so that `massimo update` defers to apt or brew where they own `/usr/bin/massimo`.
The Makefile, release.yml and the key split stay;
nfpm and a few scripts are added.

- `.deb` by nfpm: amd64, arm64,
  armhf from the GOARM=6 build (the Pi 1 included); `/usr/bin/massimo`,
  the unit derived from `packaging/systemd/`, no update helper;
  maintainer scripts do what `massimo install` does,
  with adduser and deb-systemd-helper
- marker `share/massimo/packaged` → `massimo update`,
  the web UI and `massimo install` say "managed by apt: …" and install nothing;
  `install` keeps `-settings` and `-listen`
- apt repository on the project's GitHub Pages,
  rebuilt from the last ten releases per channel at each release;
  suites = channels; RSA 4096 key in `signing`; the keyring package;
  CI proves it with a throwaway key
- Homebrew tap: a formula built from source, `brew services`,
  the marker for brew; rendered and pushed by a workflow on `release: published`
- no Windows signing for now (D5); rpm, MSI, pkg, container, LoxBerry wait (D8)

Rests on
[2026-11-06_13-05_ANALYSIS_Packaging_packages_signing_channels.md](2026-11-06_13-05_ANALYSIS_Packaging_packages_signing_channels.md)
and its addendum
[2026-11-06_13-41_ANALYSIS_Packaging_certum_windows_signing.md](2026-11-06_13-41_ANALYSIS_Packaging_certum_windows_signing.md);
the owner's decisions D1 to D8 are in the Discussion.

### Blocks

1. Deb: nfpm, the maintainer scripts, the derived unit,
   the marker and its readers, `make packages`,
   the packages in the release and the manifest, the container test,
   the documents
2. Apt: the publishing script, the keyring package,
   the Pages deploy behind the secret, the CI test with a throwaway key,
   the documents and the owner's steps
3. Homebrew: the formula template with its service block and test,
   the render script, the publish workflow, the macOS CI test, the documents

## Discussion

The analysis measured what is there (section 1),
set the rules every option keeps (section 2), weighed the packages,
repositories, signing routes and installers per platform (sections 3 to 5),
GoReleaser against nfpm with steps (section 6),
and put the blocks in order (section 9);
the addendum weighed Certum's three products for Windows.
The owner answered on 2026-11-06 with `PLAN++` and "no special windows signing ATM":

- **D1.**
  nfpm with the Makefile and release.yml as they are;
  GoReleaser's free edition cannot split the build from the signing across jobs,
  and its Homebrew support is casks.
- **D2.**
  The marker: `massimo update` and the web UI show the newer version and the
  manager's command and install nothing;
  the helper running apt or brew is Block 7, later.
- **D3.**
  The apt repository on the project's GitHub Pages, stateless,
  rebuilt from the releases.
- **D4.**
  A new RSA 4096 OpenPGP key with a two-year expiry, made by the owner offline,
  its private half in the `signing` environment,
  its public half in the repository.
- **D5, the owner's.**
  No Windows code signing now: neither SignPath nor Certum.
  Block 4 holds the place; the sign job's signature check comes with it.
- **D6.**
  The MSI and winget wait for a beta or stable channel (Block 6).
- **D7.**
  A tap with a formula built from source and `brew services`;
  bottles later if asked.
- **D8.**
  Blocks 5 to 10 wait for their conditions; the first wave is Blocks 1 to 3.

**Names chosen here**, routine and not decisions of the owner's:
the package `massimo`, the keyring package `massimo-archive-keyring`,
the marker `/usr/share/massimo/packaged` (`share/massimo/packaged` under brew's prefix),
the tap `metaworx/homebrew-massimo` with the formula `massimo-alpha` for the
channel `alpha` and `massimo` for a stable channel, the workflow `publish.yml`,
the site path `https://metaworx.github.io/massimo/apt/`,
the package maintainer the owner's public git identity.

**What the deb starts.**
The service is enabled and started only when `/etc/massimo/env` holds a `MASSIMO_` line,
as `massimo install` does,
so a fresh box is not serving the LAN without a token;
the postinst says what to fill in.

**The settings file is not a conffile.**
`postinst` creates `/etc/massimo/env` from the shipped template when missing
and never touches it afterwards,
so dpkg never asks about a file holding Cube keys;
`apt purge` removes it with the rest.

**Branch and pull request:**
this worktree's branch,
one pull request for the three blocks once the owner signals it,
`origin/main` merged into the branch first, never rebased;
nothing is pushed or opened without a signal.

**Later, as revisions of this plan**,
each from the analysis's section 9 when its condition holds:
Windows code signing (the owner's D5), the rpm (a request),
the WiX installer and winget (a beta or stable channel),
the helper running apt or brew from the button (Block 2 and a wish),
the container image on ghcr.io (a NAS user),
the LoxBerry plugin around the deb (Block 1 and a LoxBerry user),
and the notarized `.pkg` (Mac users off Homebrew).

**Who runs it:**
this session, under `PLAN++` for Blocks 1 to 3 with the owner's accounts left out:
no key is made, no secret set, no Pages enabled, no tap repository created;
every step that needs one is written, tested with a throwaway,
and skipped in CI until the secret exists.

## Analysis

What changes, by file:

| File | Block | Change |
|------|------:|--------|
| `packaging/nfpm.yaml` | 1 | new: the package from `dist/`, expanded from `VERSION`, `NFPM_ARCH`, `BINARY`; depends on adduser and init-system-helpers, recommends logrotate; xz |
| `packaging/deb/preinst`, `postinst`, `prerm`, `postrm` | 1 | new: the hand-install guard; the user, `/etc/massimo`, the settings from the template, enable and start through deb-systemd-helper; stop on remove; purge |
| `packaging/deb/unit.sh` | 1 | new: derives `dist/deb/massimo.service` from the embedded unit (`/usr/bin/massimo`, no `-update-request`), asserting both substitutions |
| `packaging/deb/packaged`, `copyright` | 1 | new: the marker (`apt`, the upgrade command); the DEP-5 copyright |
| `packaging/deb/test.sh` | 1 | new: the container test: install, the user and modes, the marker, `install` and `update` refusing, the kept settings, the guard, purge |
| `packaging/env.template`, `embed.go` | 1 | the settings template moves out of `install.go` and is embedded, so the deb ships the same bytes |
| `internal/install/install.go`, `packaged.go`, tests | 1 | `Packaged(root)` reads the marker at its absolute path and beside the binary; `Install` configures only on a packaged box, `Remove` refuses |
| `internal/install/darwin.go` | 1, 3 | `InstallDarwin` and `RemoveDarwin` refuse under brew |
| `cmd/massimo/update.go`, `serve.go` | 1 | `update` refuses on a packaged box (`-check` still reports); `serve` passes the manager to the server |
| `internal/server/update_api.go`, test | 1 | `UpdateOptions.Managed`, the first reason `updateBlocked` gives |
| `Makefile` | 1 | `NFPM`, `DEBVER`, `PACKAGES`, `packages`, `deb-test`; `release` builds them, SHA256SUMS and the manifest cover them |
| `.github/workflows/ci.yml` | 1, 2, 3 | the job Debian package (trixie and bookworm); the apt test; the formula on the macOS job |
| `.github/workflows/release.yml` | 1, 2 | the packages uploaded; the apt publish and Pages deploy behind `MASSIMO_APT_KEY` |
| `.github/workflows/publish.yml` | 3 | new: on `release: published`, the formula rendered and pushed to the tap behind `HOMEBREW_TAP_DEPLOY_KEY` |
| `packaging/apt/publish.sh`, `keyring.nfpm.yaml`, `test.sh` | 2 | new: the pool from the last ten releases per channel or from a directory, apt-ftparchive, InRelease, the keyring deb, the site; the test with a throwaway key in a container |
| `packaging/homebrew/massimo.rb.in`, `render.sh` | 3 | new: the formula template (source build, marker, service, test) and its renderer |
| `packaging/README.md` | 1, 2, 3 | "Debian package", "The apt repository", "Homebrew" |
| `README.md` | 1, 2, 3 | Installing: the apt route and the brew route |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml` | 1 | `blocked`'s reasons gain the package manager |
| `GUIDELINES/project/CI.md` | 1, 2, 3 | the jobs, the secrets, the release steps; v1.2.0 |
| `CHANGELOG.md` | 1, 2, 3 | a bullet per block under Unreleased |

**What stays:**
the raw binaries and their names (the updater asks for them by name),
`massimo install` on a box without a package,
the update helper and the button there, the manifest's shape (more assets,
same fields), the channel from the tag,
the two-job release with the key in `sign` only,
`make cross` and the CI jobs that exist.

**How the pieces fit:**

- The deb holds the release binary unchanged,
  so the manifest's SHA-256 of `massimo-linux-arm64` is also the SHA-256 of
  `/usr/bin/massimo`;
  `make release` lists the debs in SHA256SUMS and the manifest.
- `Packaged` is read by `install`,
  `update` and `serve` from `/usr/share/massimo/packaged` and from
  `<dir of the binary>/../share/massimo/packaged` (brew's Cellar);
  its two lines give the manager and the command, and the text "managed by apt:
  sudo apt …" is what `updateBlocked` returns first,
  what `massimo update` prints, and what `install` says before configuring only.
- The derived unit is the embedded one with two substitutions,
  made at build time,
  so a change to `massimo.service` reaches the deb on the next build.
- The apt site is derived from the releases:
  the publish script lists them with `gh`, takes the newest ten per channel,
  downloads their debs into `pool/<channel>/`,
  writes `dists/<channel>/` with apt-ftparchive and signs `InRelease`;
  the test feeds it a directory of debs and a throwaway key instead.
- The formula builds from the tag's source tarball with `std_go_args`,
  writes the marker under `share/massimo/`, serves with `brew services`,
  and tests itself by starting `serve` on a loopback port and reading `/healthz`.

**Risks:**

- nfpm's deb version of a `git describe` string (`v0.7.0-alpha.1-3-gabc`) is
  only for local and CI builds; a release has an exact tag.
  The Makefile names the file itself.
- The container test has no systemd:
  `deb-systemd-helper` works without one and `deb-systemd-invoke` does nothing,
  so start and restart are proved on a real box by the owner, not in CI.
- The macOS job's formula build adds Go's compile time to the job (a few minutes).
- The publish script's GitHub listing depends on `gh` and the release being published;
  it runs after `gh release create` in the same job.
- `apt-ftparchive` and `gpg` are on ubuntu-latest;
  nfpm comes through `go run` at a pinned version.

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`),
`gofmt -l .` empty, `go vet ./...` for linux, windows and darwin, `make cross`,
`shellcheck` and `bash -n` on every shell script,
and `GUIDELINES/shared/tools/check`.
Nothing reaches a Cube, the owner's boxes,
or the network beyond module and image downloads; no key,
token or real serial is written anywhere.
Each block is one or more functional commits citing `AP Packaging v1.0, Block <n>`,
each with its CHANGELOG bullet.

### Block 1: Deb

1. `packaging/env.template`:
   the Linux settings template moved out of `install.go`,
   embedded in `packaging/embed.go`; `envTemplate()` returns it,
   `darwinEnvTemplate` and `windowsEnvTemplate` keep their replacements.
2. `packaging/deb/unit.sh`:
   writes `$2` from `$1` with `/usr/bin/massimo` for `/usr/local/bin/massimo`
   and without ` -update-request /var/lib/massimo/update-request`,
   a header comment naming the package,
   and fails unless both substitutions matched.
3. `packaging/deb/preinst`:
   on `install`, refuse while `/etc/systemd/system/massimo.service` exists,
   naming `sudo massimo install -remove`.
   `postinst` on `configure`:
   `adduser --system --group --home /var/lib/massimo --no-create-home --shell /usr/sbin/nologin massimo` when missing; `/etc/massimo` 0750 and `/etc/massimo/env` 0600 from `/usr/share/massimo/env.template` when missing, both `massimo:massimo`; `deb-systemd-helper unmask` and `enable`; `systemctl daemon-reload` and `deb-systemd-invoke restart` when systemd runs and the settings hold a `MASSIMO_` line, else a message with the file to fill in and `systemctl start massimo`.
   `prerm` on `remove`: `deb-systemd-invoke stop`, `deb-systemd-helper disable`.
   `postrm` on `purge`: `/etc/massimo`, `/var/lib/massimo`, `/var/log/massimo`,
   `deb-systemd-helper purge`, `deluser --system massimo`.
   All four POSIX sh, shellcheck clean.
4. `packaging/deb/packaged` (`apt`,
   `sudo apt update && sudo apt install massimo`) and `copyright` (DEP-5,
   AGPL-3.0-or-later).
5. `packaging/nfpm.yaml`:
   name, `arch: ${NFPM_ARCH}` (`amd64`, `arm64`, `arm6`),
   `version: ${VERSION}` with the semver schema, maintainer,
   homepage `https://go.metaworx.ch/massimo`, licence, section `net`,
   priority `optional`, depends `adduser`, `init-system-helpers (>= 1.52)`,
   recommends `logrotate`, contents (the binary 0755, the derived unit,
   the logrotate file as config, the marker, the template, the copyright),
   the four scripts, `deb.compression: xz`, `deb.fields.Bugs`.
6. `Makefile`:
   `NFPM ?= go run github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.48.0`,
   `DEBVER` from `VERSION` (`v` dropped, `-` to `~`),
   `PACKAGES` (the three debs), `deb-unit`, `packages` (the unit,
   then nfpm per architecture with `SOURCE_DATE_EPOCH` from the commit),
   `deb-test` (the container test on `debian:trixie-slim`),
   `release: clean cross packages` with SHA256SUMS and the manifest over
   `BINARIES` and `PACKAGES`; `BINARIES` for the cross build only.
7. `internal/install/packaged.go`:
   `Manager{Name, Command}` with `String()` ("managed by apt: sudo apt …"),
   `Packaged(root string) (Manager, bool)` reading `root + PackagedPath` and,
   with no root, the marker beside the binary; `Install`:
   on a packaged box with `apt` or `dnf`, print the manager, skip the binary,
   the user, the units and enable, copy `-settings` when the file is missing,
   write the `-listen` drop-ins, `daemon-reload`, restart as today; `Remove`:
   refuse with `apt remove massimo` and `apt purge massimo`;
   `InstallDarwin` and `RemoveDarwin`:
   refuse under any manager ("installed by brew: brew services …").
8. `cmd/massimo/update.go`:
   after the flags, without `-check`,
   a packaged binary is refused with the manager's text (in helper mode the
   status says `failed`); `cmd/massimo/serve.go`:
   `upd.Managed` from `Packaged("")`.
   `internal/server/update_api.go`: `UpdateOptions.Managed`,
   returned first by `updateBlocked`.
9. `packaging/deb/test.sh`:
   in a Debian container with `dist/` mounted:
   `apt-get install ./massimo_*_amd64.deb`, then the user and group,
   `/etc/massimo` 0750, `env` 0600 owned by massimo and holding the template,
   the unit under `/usr/lib/systemd/system` with `/usr/bin/massimo` and no
   `-update-request`, the marker's text, `massimo version`,
   `massimo install -dry-run` saying "managed by apt" and not "install … as /usr/local",
   `massimo install -remove` failing with `apt remove`,
   `massimo update` failing with "managed by apt" and no network,
   a changed `env` kept across `--reinstall`, `apt-get purge` leaving no user,
   no `/etc/massimo`, no marker; then the guard:
   a unit in `/etc/systemd/system` makes the install fail.
10. `.github/workflows/ci.yml`:
    the job "Debian package" (ubuntu-latest, matrix `debian:trixie-slim`,
    `debian:bookworm-slim`): setup-go,
    `make linux linux-arm64 linux-armv6 packages`, the test script in the image,
    the debs as an artifact.
    `.github/workflows/release.yml`: `make release` builds the packages;
    `gh release create` uploads `dist/*.deb` beside the rest;
    the verify step checks SHA256SUMS over everything;
    the notes name the package.
11. Tests: `internal/install`: `TestPackagedMarker` (absent,
    present with two lines, one line, beside the binary),
    `TestInstallPackagedConfiguresOnly` (no binary written, no useradd,
    no units, the drop-in written, the settings copied, restart run),
    `TestRemovePackagedRefuses`, `TestEnvTemplateEmbedded`; `internal/server`:
    `TestUpdateManaged` (GET says `blocked` with the text and `available`,
    POST 403); `cmd/massimo`:
    `TestUpdateRefusesPackaged` through `-binary` on a temp layout.
12. Documents: `packaging/README.md` "Debian package" (what it installs,
    the marker, install from the file, the listen drop-in, remove and purge,
    moving a hand install over), the file list at the top; `README.md` Linux:
    "As a package"; `docs/api-v1.md` and the OpenAPI description;
    `GUIDELINES/project/CI.md` v1.2.0 sections 2 and 6.
13. The CHANGELOG bullet under Added: a Debian package per release (amd64,
    arm64, armhf), installed with `apt install ./massimo_<version>_<arch>.deb`;
    on such a box `massimo update` and the web UI show what is out and leave the
    install to apt (packaging/README.md, "Debian package").

**Verification:**
the test gate; `make packages` with a tag-shaped `VERSION` gives three debs
whose `dpkg-deb -I` shows the version `0.7.0~alpha.1`,
the depends and the scripts, and `dpkg-deb -c` the six paths;
`lintian` reports nothing worse than a warning; `make deb-test` passes here;
`shellcheck -s sh` is clean on the four scripts.

### Block 2: Apt

1. `packaging/apt/publish.sh` (bash, `set -euo pipefail`): `--out <dir>`,
   `--key <id>`, `--releases <n>` (10), `--repo metaworx/massimo`,
   `--from <dir> --channel <name>` for a local pool; lists the published,
   non-draft releases with `gh release list --json`,
   reads each tag's channel from its name,
   keeps the newest `n` per channel (`sort -V` within a channel),
   downloads `*.deb` into `pool/<channel>/`,
   adds the keyring deb (built with nfpm from the exported public key,
   `Architecture: all`, version `1.0.<key date>`),
   writes `dists/<channel>/main/binary-<arch>/Packages` and `.gz` through
   `apt-ftparchive packages` split by architecture,
   `Release` with Origin and Label `Massimo`, Suite and Codename the channel,
   the architectures and `main`,
   `InRelease` by `gpg --clearsign` and `Release.gpg` by `--detach-sign`,
   `massimo.asc` at the root,
   a `massimo.sources` sample and a one-page `index.html`.
2. `packaging/apt/keyring.nfpm.yaml`:
   `massimo-archive-keyring`,
   `/usr/share/keyrings/massimo-archive-keyring.gpg` from `${KEYRING}`.
3. `packaging/apt/test.sh`:
   a throwaway key in a temporary `GNUPGHOME` (`gpg --batch --quick-gen-key`),
   `publish.sh --from dist --channel alpha`,
   then a container with the site mounted at `/repo`:
   `massimo.asc` into `/etc/apt/keyrings`, a `.sources` with `file:///repo/apt`,
   `apt-get update` verifying `InRelease`,
   `apt-get install massimo massimo-archive-keyring`,
   `apt-cache policy massimo` naming the suite,
   the keyring in `/usr/share/keyrings`.
4. `.github/workflows/ci.yml`:
   the apt test after the deb test in the same job (amd64 only).
   `.github/workflows/release.yml`: in `sign`,
   after the release is published and only when `MASSIMO_APT_KEY` is set:
   import the key with its passphrase, `publish.sh --out site`,
   `actions/upload-pages-artifact`, `actions/deploy-pages`;
   `permissions` gain `pages: write` and `id-token: write`.
5. Documents: `packaging/README.md` "The apt repository" (the snippet with `Signed-By`,
   the suites as channels, `unattended-upgrades` with `origin=Massimo`,
   the keyring package, the key's rotation, and the owner's steps:
   making the key offline, the two secrets in `signing`,
   Pages set to GitHub Actions); `README.md` Linux;
   `GUIDELINES/project/CI.md` sections 5 and 6.
6. The CHANGELOG bullet under Added:
   an apt repository on the project's GitHub Pages, one suite per channel,
   signed with the repository key; the keyring package (packaging/README.md,
   "The apt repository").

**Verification:**
`packaging/apt/test.sh` passes here and in CI; `shellcheck` clean;
a release without the secret skips the steps (the `if` on the mapped environment
variable); the site's `InRelease` verifies with `gpgv` against `massimo.asc`.

### Block 3: Homebrew

1. `packaging/homebrew/massimo.rb.in`:
   class and name from the channel (`massimo-alpha`, `massimo`),
   `url` of the tag's tarball, `sha256`, `license "AGPL-3.0-or-later"`, `head`,
   `depends_on "go" => :build`,
   `install` with
   `std_go_args(ldflags: "-s -w -X main.version=v#{version} -X main.channel=<channel>")`
   and the marker written to `share/"massimo"/"packaged"` (`brew`,
   `brew upgrade <formula>`),
   `post_install` making `etc/"massimo"` and `var/"massimo"` and the settings
   from the template when missing,
   `service do` (`serve -listen :8099` with `-env` and `-data`, `keep_alive`,
   the log under `var/"log"`), `caveats` (the settings file, `brew services`,
   `sudo` for a daemon at boot, not to mix with `massimo install`),
   `test do` starting `serve -listen 127.0.0.1:<port> -proxy off` and reading `/healthz`.
2. `packaging/homebrew/render.sh`:
   `--tag`, `--channel`, `--url`,
   `--sha256` (computed from the URL when absent), `--out`; fills the template.
3. `.github/workflows/publish.yml`:
   on `release: published`, when `HOMEBREW_TAP_DEPLOY_KEY` is set:
   the channel from the tag name, `render.sh`,
   clone `metaworx/homebrew-massimo` over SSH with the deploy key,
   `Formula/<name>.rb`, commit and push.
4. `.github/workflows/ci.yml`, the macOS job:
   a tarball of the checkout with `git archive`,
   the formula rendered with a `file://` URL,
   `brew install --build-from-source`, `brew test`, the marker's text,
   `massimo update` and `massimo install -dry-run` refusing with "brew",
   `brew uninstall`.
5. Documents: `packaging/README.md` "Homebrew"; `README.md` macOS;
   `GUIDELINES/project/CI.md` sections 2 and 5.
6. The CHANGELOG bullet under Added: a Homebrew tap,
   `brew install metaworx/massimo/massimo-alpha`, built from source,
   run with `brew services`;
   on such a Mac `massimo update` leaves the install to brew (packaging/README.md,
   "Homebrew").

**Verification:**
`render.sh` on `v0.7.0-alpha.1` gives a formula `brew audit --strict` accepts
(where brew is available); the macOS CI job installs, tests and uninstalls it;
`shellcheck` clean.

## Proposed commit messages

Block 1,
`feat(packaging): a Debian package built with nfpm, and the updater deferring to apt`;
Block 2,
`feat(packaging): an apt repository on GitHub Pages, published from the releases`;
Block 3,
`feat(packaging): a Homebrew formula built from source, published to the tap`;
each citing `AP Packaging v1.0, Block <n>` and ending with the attribution trailer.

## Change History

- v1.0 (2026-11-06): first revision,
  from the analysis of 2026-11-06 and its Certum addendum; the owner's D1 to D8,
  D5 "no special Windows signing" for now.
