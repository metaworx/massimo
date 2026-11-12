# AP ActionLint v1.0: Pinned workflow linter

## TL;DR

actionlint becomes a Go tool pinned in a module of its own,
run by `make actionlint` and by a CI job,
so that no session has to download it by hand again,
and the findings on `main` are fixed first.

- actionlint: a binary fetched by hand, once, on the owner's word →
  `tools/actionlint/go.mod` with a `tool` line, `go tool -modfile=…`
- massimo's own `go.mod`, `go.sum` and govulncheck's scan: unchanged
- `make actionlint`; CI job "actionlint" in "All tests passed"'s needs
- Dependabot: a `gomod` entry for `tools/actionlint`, actionlint alone
- `main`'s 10 findings → 0, so that the job starts green

### Blocks

1. The tool module and `make actionlint`
2. The findings on `main` fixed
3. The CI job and Dependabot
4. Documents and the changelog

## Discussion

On 2026-11-12, after Block 17 of AP Channels, actionlint could not be run:
the binary the owner had authorized on 2026-11-11 was gone with the session's
scratch directory.
The owner asked how to fix that for good,
a submodule or vendored files as `lib/vendor.sources` in mssql-docker.

**The owner's decision** (`PLAN++ D61-A`, 2026-11-12):

- **D61-A:**
  a module of its own, `tools/actionlint/go.mod`,
  with `tool github.com/rhysd/actionlint/cmd/actionlint` (Go 1.24's `tool`
  directive);
  `make actionlint` runs `go tool -modfile=tools/actionlint/go.mod actionlint`,
  a CI job runs the same, Dependabot updates it.
  Not B, the `tool` line in massimo's `go.mod`, which would put
  actionlint's modules into the product's graph and its Dependabot group;
  not C, `go run …@version` as CI installs govulncheck, pinned but with no
  checksums in the repository and nothing Dependabot updates.
  Not a submodule: the whole actionlint repository in every worktree,
  still to be built, and empty in a new worktree without the hook.
  Not `vendor.sources`: it copies text files of the guidelines by blob id;
  here it would be a binary of 5 to 10 MB per platform in git.
- **The findings are fixed in the plan,** not let through (recommended in the
  gate, note 3): the job fails on any finding.

## Analysis

**Measured on 2026-11-12** (worktree at `main`, actionlint v1.7.12 built
by `go tool` from a trial module, shellcheck 0.9.0 from `/usr/bin`):

- `go mod init go.metaworx.ch/massimo/tools/actionlint`,
  `go get -tool github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`,
  then `go mod edit -go=1.26.0` (massimo's `go` line; `go mod init` writes
  the toolchain's, 1.26.8) and `go mod tidy`.
- `go tool -modfile=tools/actionlint/go.mod actionlint -version` from the
  repository root builds and runs it,
  and lints `.github/workflows/` of the root:
  `-modfile` changes the module file and its `go.sum`,
  not the directory the tool runs in.
- The `go.mod` marks `github.com/rhysd/actionlint` `// indirect` (a tool's
  module is),
  so Dependabot's `allow: dependency-type: direct` of the root entry would skip
  it: the new entry allows it by name.
- A nested module is left out of the root's `./...`, so `go test`,
  `go vet`, `gofmt -l .` and govulncheck see no change.
  `tools/**` is not shipped (`project.shipped`), so no fragment is needed
  for the module itself; the CI job and the Makefile are shipped paths.

**The findings on `main`** (`actionlint -oneline`, exit 1), all shellcheck's:

| Where | Finding |
|---|---|
| `ci.yml:395` (govulncheck) | SC2086 info: `$(go env GOPATH)/bin/…` unquoted at col 46 |
| `ci.yml:611`, `750`, `837`, `929`, `973` | SC2034 warning: `i` unused (retry loops `for i in 1 2 3`) |
| `ci.yml:973` | SC2317 info ×3: unreachable at line 30 of the script |
| `release.yml:266` | SC2016 info: an expression in single quotes |

Each is fixed in the code where the fix is right (`_` for an unused loop
counter, quotes),
or carries `# shellcheck disable=SC<n>` with its reason on the line above,
as `GUIDELINES/shared/lang/shell/LINTING.md` asks,
where the code is right as it is (such as a `$` meant literally,
or a function called through a trap).

**The CI job** runs where `.github/**` or `tools/actionlint/**` changed,
or always where that is simpler; it needs `changes` like the others,
uses `actions/setup-go` with `GO_VERSION`,
caches nothing beyond setup-go's module cache,
and is listed in "All tests passed"'s `needs`,
so the one required check covers it. shellcheck is on GitHub's Ubuntu runners.

## Implementation Plan

One branch, `claude/actionlint`, one pull request; the owner merges.
Each commit cites `AP ActionLint v1.0, Block <n>`.

### Block 1: The tool module and `make actionlint`

1. `tools/actionlint/go.mod` and `go.sum` as measured, `go 1.26.0`,
   actionlint at the newest release (v1.7.12 on 2026-11-12);
   a `tools/actionlint/README.md` of a few lines: what it is, the command,
   how to update (`go get -tool …@vX.Y.Z` in that directory).
2. Makefile: `actionlint:` running
   `go tool -modfile=tools/actionlint/go.mod actionlint`, in `.PHONY`,
   with a comment.
3. **Verification:**
   `make actionlint` builds and runs it (exit 1 on `main`'s findings until Block
   2); `go test ./...`, `go vet` and `gofmt -l .` unchanged.

### Block 2: The findings on `main` fixed

1. Each finding of the table, fixed or annotated with its reason.
2. **Verification:**
   `make actionlint` exits 0; the YAML parses;
   `bash -n` on each changed script where it can be extracted.

### Block 3: The CI job and Dependabot

1. `ci.yml`:
   a job `actionlint` (name "actionlint"), `needs: changes`,
   setup-go with `GO_VERSION`, `make actionlint`;
   added to the `tests` job's `needs`.
2. `.github/dependabot.yml`:
   `gomod` for `/tools/actionlint`, monthly,
   `allow: dependency-name: github.com/rhysd/actionlint`, prefix `build`,
   as the other entries.
3. **Verification:**
   `make actionlint` (it lints its own job);
   the pull request's run shows the job green and "All tests passed".

### Block 4: Documents and the changelog

1. `GUIDELINES/project/CI.md`:
   the job in section 2's list,
   the tool and its update in section 8 ("Dependabot");
   a version bump and its `File-revision-v1` trailer.
2. `GUIDELINES/project/_CONTRACT.md`'s "Lint command": `make actionlint`
   for a change under `.github/`; `AGENTS.md` and `GUIDELINES/README.md`
   regenerated as the guidelines' tools do it, not edited by hand.
3. `CONTRIBUTING.md` where it names the lint commands.
4. A `CHANGELOG.d/Changed/` fragment (the CI job and the Makefile are shipped
   paths).
5. **Verification:**
   `reflow.sh --check` no worse than `main`, `changelog.sh check --staged`,
   `check-citations.sh`.

## Proposed commit message

```text
ci(pipeline): actionlint pinned as a Go tool, run by make and CI

AP ActionLint v1.0, Blocks 1 to 4 (D61-A).

Key changes:
- tools/actionlint: a module of its own whose tool line pins actionlint;
  make actionlint runs it with go tool -modfile.
- main's ten shellcheck findings in ci.yml and release.yml fixed.
- CI's actionlint job, among All tests passed's needs; Dependabot
  updates the tool.

File-revision-v1: GUIDELINES/wip/2026-11-12_16-17_AP_ActionLint_pinned_workflow_linter.md::v1.0::2026-11-12::All::Blocks 1 to 4
Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-12): first revision, by the owner's `PLAN++ D61-A`.
