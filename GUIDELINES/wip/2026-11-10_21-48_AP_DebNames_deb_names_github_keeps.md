# AP DebNames v1.0: Deb names GitHub keeps

## TL;DR

The Debian packages are named as GitHub will name their assets, so that
`SHA256SUMS`, the signed manifest and the release agree, and a release
fails where GitHub renames a file it uploads.

- file name `massimo_0.13.0~alpha.2_amd64.deb`
  → `massimo_0.13.0.alpha.2_amd64.deb`; control `Version` keeps `~`
- rolling job: debs deleted after each move → kept
- `sha256sum -c SHA256SUMS --ignore-missing`: debs skipped → checked
- `packaging/README.md`: `curl` of the tilde name, a 404 → the dotted name
- release.yml: no check after upload → every uploaded name must be an
  asset of the release, the rolling job's checked before it deletes
- v0.13.0-alpha.2: left as published, a line in its notes (D2-B)

### Blocks

1. File names with dots, the documents, a fragment
2. The asset-name check in release.yml, a fragment
3. The line in v0.13.0-alpha.2's release notes

## Discussion

The owner asked on 2026-11-10 why GitHub shows the debs with dots, and
what that breaks. The findings were presented as a TL;DR gate.

**The owner's decisions** (`PLAN++ D2-B`, 2026-11-10):

- **D1-A:** the file names use `.` where the version has `~`; the control
  file's `Version` stays `0.13.0~alpha.2`, so that dpkg still sorts a
  pre-release first. Not B, `SHA256SUMS` and the manifest written with
  GitHub's names (`dist/` would disagree with its own sums, and GitHub's
  rule would be copied); not C, another separator (`-` reads as a Debian
  revision, `_` splits `name_version_arch`).
- **D2-B:** the published v0.13.0-alpha.2 keeps its `SHA256SUMS`; its
  notes get the line that checks the debs. Not A, a dotted `SHA256SUMS`
  uploaded over it.
- **D3-A:** the asset-name check in release.yml (as recommended).

## Analysis

**What GitHub does.** An uploaded asset's `~` becomes `.`; the tilde URL,
raw or `%7E`, is a 404. `DEBVER` turns every `-` of the version into `~`
(`v0.13.0-alpha.2` → `0.13.0~alpha.2`, a rolling build's
`v0.13.0-alpha.2-1-g7d170c3` → `0.13.0~alpha.2~1~g7d170c3`).

**The consumers.**
- `massimo update` downloads only `AssetName()`, a binary: unaffected.
- `packaging/apt/publish.sh` downloads `*.deb` by GitHub's names and
  writes `pool.tsv` with them; apt reads the version from the control
  file. The dotted names are the ones its pool already holds: unaffected.
  `--from <dir>` copies whatever the directory holds.
- `packaging/site/build.sh` follows `pool.tsv`: unaffected.
- release.yml's rolling job keeps the `basename`s of its files and deletes
  every other asset; with the dotted names those are the files it
  uploaded. `release/devel` holds no deb since its last move.
- `sha256sum -c --ignore-missing` finds no file of a deb's line.
- `packaging/deb/systemd-test.sh` names v0.12.0-alpha.1's deb by its
  dotted name already.

**The file name.** `DEBFILE = $(subst ~,.,$(DEBVER))`, the file
`massimo_$(DEBFILE)_<arch>.deb`, nfpm given `VERSION=$(DEBVER)` as
before. The file name no longer spells the version dpkg compares; apt,
dpkg and the pool read the control file, and nothing here sorts by name.

**The check.** After `gh release create` (a numbered release, and a
rolling release made for the first time) and after the rolling job's
uploads, before it moves the tag and deletes: each `basename` of the
uploaded files must be one of the release's asset names, else the step
fails with the names GitHub lacks. A rename then turns the run red
instead of losing or orphaning files; the rolling release keeps its old
tag and its old files.

## Implementation Plan

### Block 1: File names with dots

1. Makefile: `DEBFILE`, `PACKAGES` and the three `nfpm package -t` paths
   with it; the comment above says why the name and the version differ.
2. `packaging/README.md`, "Debian package": the file name carries the
   version with dots, the package the version with `~`.
3. `CHANGELOG.d/Fixed/deb-asset-names.md`.

**Verification:** `make packages VERSION=v0.13.0-alpha.2`: the three
files with dots, `dpkg-deb -f <file> Version` says `0.13.0~alpha.2`;
`make release` in a scratch copy: `SHA256SUMS` and the manifest name the
files `dist/` holds; the test gate, `make cross`.

### Block 2: The asset-name check

1. release.yml, "Publish the GitHub release": the check after
   `gh release create`.
2. release.yml, the rolling "Publish": the check after `create` and after
   the uploads, before the tag moves; its comment says why.
3. `CHANGELOG.d/Fixed/release-asset-names-checked.md`.

**Verification:** the workflow parses (actionlint where installed); the
check's shell run by hand against v0.13.0-alpha.2 fails on the tilde
names and passes on dotted ones; `go test ./...`.

### Block 3: v0.13.0-alpha.2's notes

1. After the pull request is merged: `gh release edit v0.13.0-alpha.2
   --notes-file` with its notes and one line:
   `sed 's/~/./g' SHA256SUMS | sha256sum -c --ignore-missing` checks the
   debs, whose names GitHub wrote with dots.

**Verification:** `gh release view v0.13.0-alpha.2` shows the line.

## Proposed commit message

Block 1:

```
fix(packaging): Deb file names are the ones GitHub keeps

GitHub turns the ~ of an uploaded asset's name into a dot, so the
release's debs were not the files SHA256SUMS and the manifest name: sha256sum -c skipped them, the rolling job deleted them after each upload, and packaging/README.md's curl line was a 404.

Key changes:
- Makefile: the deb's file name has the version with dots (DEBFILE),
  its control Version keeps the ~ dpkg sorts by
- packaging/README.md: the file name and the version say so

Plan: AP DebNames v1.0 (Block 1)

Assisted-by: claude-code:claude-opus-5-5
```

Block 2:

```
fix(pipeline): A release fails where GitHub renames a file it uploads

Key changes:
- release.yml: after each upload every file's name must be an asset of
  the release; the rolling job checks before it moves the tag and
  deletes

Plan: AP DebNames v1.0 (Block 2)

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-10): first revision, D1-A, D2-B, D3-A.
