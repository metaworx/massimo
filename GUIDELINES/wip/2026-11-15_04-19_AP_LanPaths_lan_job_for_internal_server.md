# AP LanPaths v1.0: LAN job for internal/server

## TL;DR

A pull request that changes `internal/server`'s Go files runs CI's LAN job,
since discovery's runs are decided there and the LAN test checks them.

- the LAN filter → plus `^internal/server/[^/]+$`: the package's files, not `ui/` (D1-A)
- `ci.yml`'s comments, CI.md v1.53.0 (the table's row, section 2's list) → name it
- cost, the week to 2026-11-14 → LAN on 90 of 144 merges instead of 57, about 3 min each,
  beside Vet and test (4-5 min), so no run takes longer

### Blocks

1. The filter, its comments, CI.md and the fragment

## Discussion

AP LostSearch's run, 2026-11-13, found that its Block 1,
the change to `internal/server/discovery.go` and `server.go`,
would not alone have run the LAN test whose step 10 checks it;
its pull request ran the job only through `tools/lan-test/`.
The owner asked on 2026-11-14 to plan adding `internal/server` to the job's paths.

**The owner's decision**, 2026-11-15, `PLAN++ as recommended`:

- **D1-A.** Any file directly in `internal/server/` (`^internal/server/[^/]+$`),
  so that `ui/` does not count: the rule holds as files are added or split,
  and a pull request of the web UI alone still skips the job.

**Not in this plan:** the contract's test gate, which already asks for
`make lan-test` "for a change to discovery".

**Who runs it:**
this session, under the owner's `PLAN++` of 2026-11-15,
whose gate stated: the plan written, registered on main, built on a branch,
committed, pushed and the pull request opened, without a merge or a retirement.
Branch `claude/lan-paths`.

## Analysis

**Today**, the job What to test (`.github/workflows/ci.yml`) answers `lan=false`
for a pull request unless its changes match
`^(internal/announce/|internal/serve/|internal/proxy/|internal/cubelan/|cmd/massimo/(discover|serve)\.go$|tools/lan-test/|\.github/workflows/ci\.yml$)`.
`internal/server` holds `discovery.go` (the runs, `needSearch`, following an address),
`server.go` (the loss waking a search) and the proxy's wiring,
and has one subdirectory, `ui/`, the web UI, which the LAN test does not reach.

**Measured** over the 144 merges of `main` from 2026-11-07 to 2026-11-14:
the filter matched 57; 61 changed a file directly in `internal/server/`,
33 of them matching nothing else, so 90 with the change.
The LAN job took about 3 minutes on a pull request, Vet and test 4 to 5,
and they run side by side.
In the last 60 runs the job passed 17 times, was skipped 34 times
and cancelled 3 times by a newer push; it failed none.

| File | Change |
|------|--------|
| `.github/workflows/ci.yml` | the pattern; the comments of What to test and of the LAN job |
| `GUIDELINES/project/CI.md` | v1.53.0: section 2's table row and the LAN job's list |
| `CHANGELOG.d/Changed/claude-lan-paths.md` | the fragment |

## Implementation Plan

Nothing reaches GitHub but the plan's registration, the branch and the pull request.
One functional commit, citing `AP LanPaths v1.0, Block 1`.

### Block 1: The filter

1. `ci.yml`: `internal/server/[^/]+$` in the LAN pattern, after `internal/proxy/`;
   What to test's comment and the LAN job's comment name the server's discovery.
2. `CI.md`: the table's row ("discovery" there covers it, the list in section 2 names it).
3. `CHANGELOG.d/Changed/claude-lan-paths.md`:

   ```
   - CI's LAN test runs on a pull request that changes the server's
     own Go files, where discovery's runs are decided.
   ```

**Verification:**
the pattern, run against the changed files of past merges,
matches #225's Block 1 alone (`internal/server/discovery.go`, `server.go`, the tests),
not a change of `internal/server/ui/` alone, and still every path it matched;
`make actionlint`; the guidelines check;
the pull request's run has the LAN job, through `ci.yml`.
A pull request changing `internal/server` alone is the first to show it live.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work branch:

```
chore(aps): +LanPaths v1.0

A pull request that changes internal/server alone, where discovery's runs are decided, skips CI's LAN job, which checks them. The owner decided on 2026-11-15 to add the package's own files, not ui/. PLAN++ LanPaths, with D1-A.

Key changes:
- GUIDELINES/wip/2026-11-15_04-19_AP_LanPaths_lan_job_for_internal_server.md: the plan, one block

File-revision-v1: GUIDELINES/wip/2026-11-15_04-19_AP_LanPaths_lan_job_for_internal_server.md::v1.0::2026-11-15::All::the first revision, by PLAN++ LanPaths
Assisted-by: claude-code:claude-opus-5-5
```

Block 1: `ci(pipeline): A pull request changing the server's Go files runs the LAN test`.

## Change History

- v1.0 (2026-11-15): the first revision, from the owner's D1-A.
