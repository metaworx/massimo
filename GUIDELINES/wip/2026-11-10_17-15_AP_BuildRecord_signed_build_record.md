# AP BuildRecord v1.0: Signed build record and provenance

## TL;DR

Every release, rolling builds included, carries a signed record of how it
was built, and GitHub's build provenance for every file it publishes, so
that anyone can check the build environment and the libraries without
trusting the log page of a run.

- build log in the run only → `massimo-build.txt` in the release:
  the runner, the toolchain, the command, `go version -m` of every binary
  and of nfpm, then the whole log
- listed in `SHA256SUMS` and the manifest → covered by the manifest's
  signature, the release key's or the rolling key's
- + `actions/attest-build-provenance` in the build job: one attestation
  for every binary, package, `SHA256SUMS`, the manifest and the record;
  `gh attestation verify <file> -R metaworx/massimo`
- the build job holds no secret, so the record holds none

### Blocks

1. The build record: `packaging/build-record.sh`, `make release` logs and
   lists it
2. Build provenance in `release.yml`'s build job
3. Documents and the changelog

## Discussion

Asked by the owner on 2026-11-10: the log of the step "Build every binary
and package, SHA256SUMS and the manifest" as a signed file in the release,
to verify the exact build environment and libraries.

**The owner's decisions** (`PLAN++`, 2026-11-10, "as recommended"):

- **D1-A:** one file, `massimo-build.txt`: a header (runner image and
  version, `go version`, the `go env` values that shape a build, the
  `make` command, nfpm's version), `go version -m` of each binary, then
  the full log. Not B, the raw log only.
- **D2-A:** listed in `SHA256SUMS` and the manifest, so the existing
  signature covers it. Not B, a signature of its own.
- **D3-A:** GitHub build provenance attestations as well, signed through
  Sigstore, not with our keys; free for a public repository.
  Not B, not now.

**The owner's question on D3** ("does that cover ALL files, or at least
the SHA256SUMS file?"): all of them. The attestation's subjects are every
binary, every Debian package, `SHA256SUMS`, the manifest and the build
record, each by its SHA-256; only `massimo-release.json.sig` is not,
since the signing job makes it after the build and it is a signature
itself.

## Analysis

**What the binaries already carry.** `go version -m` on a release binary
shows the Go version, every dependency with its version and `h1:`
checksum, `-trimpath`, `CGO_ENABLED`, `GOOS`/`GOARCH`/`GOAMD64`/`GOARM`
and the commit (`vcs.revision`); `-s -w` leaves the build information in.
It does not show the `-ldflags` (Go leaves them out under `-trimpath`),
the runner, the `make` command, or nfpm, which builds the packages through
`go run` and leaves no trace in them. The record adds those, and puts the
binaries' information where it can be read without the binary.

**Capturing the log.** `make release` runs its build as a sub-make piped
through `tee` into `dist/build.log`, under bash with `pipefail`
(target-specific `SHELL` and `.SHELLFLAGS`, GNU make), so that a failed
build still fails; `packaging/build-record.sh` then writes the record
from the log, and the log goes. VERSION, FLAVOUR and RELEASE reach the
sub-make on its command line, as `make` passes them on. Locally
`make release` writes the same record, the runner lines saying it was not
GitHub.

**nfpm's dependencies.** `go run pkg@version` builds nfpm in the build
cache and keeps no binary; the record script builds it again with
`go install` into a temporary directory and records its `go version -m`.
It is the same module graph, checked against the checksum database, so the
list is exact; the binary is not the one that ran.

**Secrets.** The build job holds none (D2-A of AP Channels): its
environment has no secret, and the log is what the run's page shows.
The record prints selected `go env` values only, not the whole
environment.

**Provenance.** `actions/attest-build-provenance` v4.2.2, pinned by its
commit, in the build job after `make release`: it signs an in-toto
statement through Sigstore's public instance with the job's OIDC token,
naming the repository, the workflow, the commit and the runner, and
stores it with GitHub's attestations API. It needs `id-token: write` and
`attestations: write` on the build job, and `attestations: write` granted
by ci.yml's caller job. In the build job, not the signing jobs: those hold
the keys and run nothing but openssl and gh, and the provenance is the
build's statement. A build whose release is then refused leaves an
attestation of files nobody publishes, which harms nothing. Each devel
build adds one, as each tag does.

**Consumers of the assets.** The updater looks assets up by name
(`internal/update`); apt's `publish.sh` takes `*.deb` alone; the sign and
rolling jobs upload `dist/massimo-*`, which takes the record along, and
the rolling job keeps every file it uploaded. Nothing else reads them.

## Implementation Plan

### Block 1: The build record

1. `packaging/build-record.sh <log> <binary>…`: prints the header,
   `go version -m` of each binary, nfpm's, then the log.
2. `Makefile`: `release` runs `release-files` (cross, packages) through
   `tee`, writes `dist/massimo-build.txt`, and lists it in `SHA256SUMS`
   and the manifest.

**Verification:** `make release` locally: the record is there, in
`SHA256SUMS` and the manifest; a failing build fails `make release`;
`shellcheck`; `go test ./...`.

### Block 2: Build provenance

1. `release.yml`'s build job: `permissions` with `contents: read`,
   `id-token: write`, `attestations: write`; the attest step after the
   build, its subjects `dist/massimo-*`, `dist/*.deb`, `dist/SHA256SUMS`.
2. `ci.yml`'s release job grants `attestations: write`.
3. The header comment of `release.yml` names the record and the
   provenance.

**Verification:** YAML parses; after the merge, the devel build's run
shows the attestation, and `gh attestation verify` passes on its files.

### Block 3: Documents

1. `README.md` ("Versions") or `packaging/README.md`: how to read the
   record and verify the provenance.
2. `GUIDELINES/project/CI.md`: the record and the provenance.
3. `CHANGELOG.md`: an Added bullet.

**Verification:** the full test gate, `make cross`, reflow of the changed
documents.

## Proposed commit message

```
feat(pipeline): Every release carries its signed build record and provenance

Key changes:
- `make release` logs its build and writes massimo-build.txt: the runner,
  the toolchain, the command, go version -m of every binary and of nfpm,
  then the log; SHA256SUMS and the signed manifest list it
- the build job attests every file it built through GitHub's build
  provenance (Sigstore); gh attestation verify checks them
- README.md and CI.md say how to verify a build

Plan: AP BuildRecord v1.0 (Blocks 1-3)

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-10): first revision, D1-A, D2-A, D3-A.
