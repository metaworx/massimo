# AP WorkDocs v1.0: working documents leave docs

## TL;DR

The rules that bind the work move into the project contract, and the
working documents leave `docs/`, so that what stays there is documentation
the site can publish whole.

- coding standard (end of `review-2026-11-05.md`) + the handover's
  "Conventions" → contract §3, v1.6.0 → v1.7.0; only what §3 lacks, brought
  up to date; stale lines dropped (Go 1.24, "one dependency", `make tag`)
- `docs/handover-session.md` → `GUIDELINES/wip/HANDOVER.md` (D11-A)
- the review's rest → `GUIDELINES/wip/review-2026-11-05.md` (D12-A)
- `docs/handover-write-test.md` → `docs/testing-writes.md`;
  the two reports → `docs/reports/` (D10-A)
- `docs/proposal-2026-11-05.md` → retired, cited nowhere
- every reference → the new path; `AGENTS.md`, `GUIDELINES/README.md`
  regenerated

### Blocks

1. Contract: the coding standard and the conventions in §3, v1.7.0
2. Moved: the working documents, and every reference to them

## Discussion

**Decided** by the owner, 2026-11-10, in the Site analysis and at the
TL;DR gate (`PLAN++ WorkDocs`, D10 to D12 as recommended):

- **The handovers and reviews leave `docs/`** for `GUIDELINES/wip/`.
- **D7-A.** The coding standard goes into the contract's §3, merged with
  what is there, the handover's "Conventions" with it.
  B, a `docs/codestyle.md`, would be published but read by an agent only
  when it thinks to; C, a file in `GUIDELINES/project/`, neither inlined nor
  published.
- **D8-A.** The handover is a standing document in `wip/`: no timestamp,
  rewritten in place, retired at the owner's discretion once no other
  document references it, as plans and analyses are.
  The shared rule is requested from the guidelines (a plan of its own
  there); `check-plans.sh` already ignores an undated file in `wip/`.
- **D10-A.** The write test's plan and the two test reports stay in `docs/`:
  README, `docs/update-windows.md` and `internal/server/restore_api.go`
  cite them as evidence, and nothing that outlives a change may cite `wip/`.
  B, all to `wip/` with the cited facts copied into each citing document:
  more work, and the evidence leaves the published docs.
- **D11-A.** `HANDOVER.md`: undated, so `check-plans.sh` passes it by, and
  it stands out among the dated files. B kept `handover-session.md`.
- **D12-A.** The review's findings, after the standard has left, go to
  `wip/` as a standing document too, retired at the owner's discretion.
  B retired it now.

**Open:**

- **D13. A generated list of documents in README.md** (the owner's question
  at the gate). Not in this plan's blocks; for AP Site.
  - **A (recommended).** One fragment, `docs/fragments/_documents.md`, the
    grouped list with a line per document; README.md includes it as a
    PARTIAL (`sync-docs.sh`, the mechanism the site's home already uses),
    and the site's docs index is built from the same fragment, its build
    failing on a `docs/*.md` the fragment does not list. No new tool.
  - **B.** A generator from each document's title, a new block kind in the
    guidelines: no descriptions to keep, but the titles say less than the
    list's lines do.
  - **C.** Kept by hand, as today.

## Analysis

### What the contract's §3 already holds

Of the standard's fifteen points, §3 already says, in other words:
secrets in code and documents, protocol bytes only in
`internal/protocol/cube`, hardware findings the day they are made, tests on
the fake Cube, the web UI's rules, documents moving with the code.
The commit points are the `[commit]` keys' generated form.

**New to §3**, checked against the code on 2026-11-10:

- Standard library first; `gofmt` and `go vet` clean; no `//nolint` in our
  code (the five are in `third_party/bytemare-ksf/`, vendored as it is).
- The SPDX line `AGPL-3.0-or-later` and the copyright line atop every file
  (`internal/server/auth.go`, `ui/components.js` carry them);
  JavaScript and CSS a one-line licence comment.
- Package comments say what the package is for; every exported symbol a
  comment that is a sentence; an unexported helper one when its name does
  not say it.
- Errors wrapped with context (`fmt.Errorf("serve: %w", err)`), sentinels in
  the package defining the condition, HTTP errors through `fail(status, …)`
  (`internal/server/server.go`); never a panic on input.
- `slog` key-value pairs: Info for what a user wants in the log, Warn for
  anything the Cube refused, Debug for per-line traffic; Error, used by the
  store for what failed on our side, is not in the standard and is added.
- Keys, IVs and tokens redacted in every printout; real room and device
  names only in the owner's pastes.
- The web UI: every icon-only control labelled, keyboard operable.

**Stale, dropped:** "Go 1.24" (`go.mod` says 1.26; the version is
`go.mod`'s), "one external dependency" (eight direct ones now),
"one feature per tag (`make tag`)" (`make release-alpha`), the commit form.

**From the handover's "Conventions"**, new to §3: the proxy's `-no-direct`
and `-direct`; an API client needs a token, `MASSIMO_API_TOKEN` the owner's,
every other user one of their own (401 without); the lessons of the hardware
days in `docs/portal-protocol.md` and `packaging/README.md`, "Learnings";
the owner's Windows service, the Pi and the Mac touched only on the owner's
word in the session.
The signed-in user's settings on the server is a fact of `docs/store.md`,
not a rule, and stays there.

### The references

`git grep` on 2026-11-10, beyond the files themselves:

| File | Cited from |
|------|------------|
| `handover-session.md` | the contract (twice, so `AGENTS.md` and `GUIDELINES/README.md`), `README.md`, `docs/roadmap.md`, `docs/ui-testing.md`, `docs/update-windows.md`, `docs/proposal-2026-11-05.md`, `docs/review-2026-11-05.md`, `internal/server/restore_api.go` |
| `handover-write-test.md` | `README.md`, `docs/portal-protocol.md`, `docs/write-test-2026-11-05.md` |
| `review-2026-11-05.md` | the contract, `README.md`, `docs/handover-session.md` |
| `windows-test-2026-11-05.md` | `README.md` (three times), `docs/update-windows.md` |
| `write-test-2026-11-05.md` | `README.md`, `internal/server/restore_api.go` |
| `proposal-2026-11-05.md` | nothing |

Also in `wip/`, left as they are: the Packaging analysis names
`docs/windows-test-2026-11-05.md` and the MobileApp analysis
`docs/handover-session.md` (UAMFs, never overwritten); AP MobileApp's
Block 10 names `docs/handover-session.md` as a file it will edit, and will
find it moved.

**Citing the handover in `wip/`.** AGENTS.md §4: *nothing that outlives
the change may cite* `wip/`. The contract and `docs/roadmap.md` name the
handover as the document to update; D8's rule, retired once nothing
references it, implies a standing document may be cited. Until the
guidelines carry that rule, the contract names it as the standing document
it is, and the documents that only point a reader at it (`README.md`'s list,
`docs/ui-testing.md`, `docs/update-windows.md`) do too.
`restore_api.go`'s comment, the one citation from code, goes to
`docs/portal-protocol.md` instead, which holds the 1.25 % per write.

## Implementation Plan

### Block 1: Contract

- `GUIDELINES/project/_CONTRACT.md` §3: its opening names §3 itself as the
  rules; the points of the Analysis that are new go into its bullets — a
  bullet **Go code** for the language's (standard library, headers,
  comments, errors, logging, `//nolint`), the rest into **Secrets and
  identities**, **The Cube**, **The web UI** and **Hosts**, and a bullet
  **The API** for the token; the documents bullet names
  `GUIDELINES/wip/HANDOVER.md`. Version 1.7.0, its history line.
- `AGENTS.md`, `GUIDELINES/README.md`: regenerated with `sync-docs.sh`.
- The review's standard and the handover's "Conventions" stay where they
  are until Block 2 moves the files; Block 2 removes them.
- No CHANGELOG bullet: `GUIDELINES/**` and `*.md` are not shipped.

**Verification:**

- `sync-docs.sh --check` clean; `reflow.sh` and `check-headings.sh` clean
  on the contract.
- `commit-msg.sh` on the message.

### Block 2: Moved

- `git mv docs/handover-session.md GUIDELINES/wip/HANDOVER.md`; its
  "Conventions" section becomes one line pointing at the contract's §3; its
  checkout section's line on the review's standard goes.
- `git mv docs/review-2026-11-05.md GUIDELINES/wip/review-2026-11-05.md`;
  its "Coding standard" section becomes one line pointing at the contract.
- `git mv docs/handover-write-test.md docs/testing-writes.md`; its title
  says it is the test plan.
- `git mv` the two reports to `docs/reports/`; their links to the test plan
  and each other updated.
- `git rm docs/proposal-2026-11-05.md`.
- Every reference of the Analysis's table to the new path; `README.md`'s
  list of documents: the handover and the review leave it, the test plan and
  reports at their new paths.
- `internal/server/restore_api.go`: the two comments cite
  `docs/reports/write-test-2026-11-05.md` and `docs/portal-protocol.md`.
- A CHANGELOG bullet, `restore_api.go` being shipped: under `### Changed`,
  the documents' moves.

**Verification:**

- `git grep` for each old path: none left outside `wip/` UAMFs and plans.
- `check-citations.sh`, `check-plans.sh`, `sync-docs.sh --check`,
  `reflow.sh` on the touched documents, `changelog.sh check`.
- `gofmt -l .`, `go vet ./...`, `MASSIMO_ENV_FILE=/dev/null go test ./...`
  (`tools/identifiers` scans the moved files).
- The CI's `check --commit-range origin/main..HEAD`.

## Proposed commit message

Block 1:

```
docs(contract): The coding standard and the session conventions join the contract

The project contract's §3 now holds the rules the coding standard of the 2026-11-05 review and the handover's conventions held, so that they are in every agent's context and in GUIDELINES/README.md, brought up to date with the code.

Key changes:
- §3 Go code: standard library first, the licence header, comments, errors, logging levels, no //nolint
- §3 Secrets, The Cube, The web UI, Hosts: printouts redacted, the proxy's -no-direct and -direct, labelled and keyboard-operable controls, the owner's machines only on the owner's word
- §3 The API: a client needs a token of its own
- §3 opening: §3 itself is the rules; the standard's stale lines (Go 1.24, one dependency, make tag) are not carried over
- AGENTS.md, GUIDELINES/README.md: regenerated
- No CHANGELOG bullet: GUIDELINES/ and the documents are not shipped

AP WorkDocs v1.0, Block 1.

File-revision-v1: GUIDELINES/project/_CONTRACT.md::v1.7.0::2026-11-10::3::The coding standard and the session conventions join the contract.
Assisted-by: claude-code:claude-opus-5-5
```

Block 2:

```
docs(working-docs): The working documents leave docs/

The session handover and the 2026-11-05 review move to GUIDELINES/wip/, so that docs/ holds only documentation; the write test's plan and the two test reports stay there as evidence the code and README cite.

Key changes:
- docs/handover-session.md → GUIDELINES/wip/HANDOVER.md, its conventions now the contract's
- docs/review-2026-11-05.md → GUIDELINES/wip/, its coding standard now the contract's
- docs/handover-write-test.md → docs/testing-writes.md; the two reports → docs/reports/
- docs/proposal-2026-11-05.md retired: nothing cites it
- every reference, README.md's list of documents and two comments in internal/server/restore_api.go included

AP WorkDocs v1.0, Block 2.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-10): the plan, from the TL;DR gate; D7-A, D8-A from the
  Site analysis, D10-A, D11-A, D12-A by the owner's `PLAN++ WorkDocs`;
  D13 open, for AP Site.
