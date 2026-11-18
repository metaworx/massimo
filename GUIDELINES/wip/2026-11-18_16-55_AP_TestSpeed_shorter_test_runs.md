# AP TestSpeed v1.0: shorter test runs

## TL;DR

The test runs take long, and the macOS job is often cancelled for want of a runner.
This plan cuts what runs where it is not needed, then what is slow
(from the analysis the owner asked for on 2026-11-18, the next revision).

- macOS and Windows jobs on every pull request with code → only where their code
  or the announcement changed; always on a tag and in the nightly run
- the announcement's paths → one list, used by the LAN, macOS and Windows gates
- the rest of the run time → the analysis of why the tests are slow, folded into v1.1

### Blocks

1. Gates: the macOS and Windows jobs gated by path on a pull request

## Discussion

### What the owner asked for

On 2026-11-18, in these words:
"the macos launch daemon job gets cancelled more often than not due to the lack of
an available runner. could we gate it more specifically, that it runs only when
it's relevant code is changed. should we apply a similar logic to the windows
service then?"
Then: "would the macos service not also announce itself on the network? [...]
I think it should - as well as the linux build.
so I would extend D1-A's path list to include the part of D2A that is
announce-specific".
And, at the end of AP TestInfrastructure:
"please analyse why the tests have become so slow (long-running) and if there's
anything that can be done to cut the overall run time".

### What is decided

The owner, 2026-11-18, `EXEC as recommended`:

- **D1-A, as the owner extended it.**
  On a pull request the macOS job runs for a change to its own paths
  (`internal/install`, `internal/update`, `internal/safefile`,
  `cmd/massimo`'s install, update and service files, the darwin packaging,
  `go.mod` and `go.sum`, its job in `ci.yml`)
  or to the announcement's paths;
  it always runs on a tag, for a release, and in the nightly run.
- **D2-A.**
  The Windows job the same, with its own paths, the announcement's,
  and `internal/cubelan` and the server's discovery and search files,
  which its unit tests cover.
- **D3-A.**
  The announcement's paths are kept once, in "What to test",
  and used by the LAN, macOS and Windows gates.
- **D4-A.**
  This is the first block of a plan for the tests' run time, this one.

### Still open

Asked by the owner while v1.0 was written ("would [D1-]B not be better?"),
gated at Block 1's commit gate:

- **D5.
  A macOS or Windows job that never got a runner on a pull request.**
  - A (recommended):
    it counts as not run, so the pull request is not held,
    and the Tested record leaves that job out,
    so that the push of the merged tree runs it and a failure shows on `main`,
    its author told.
  - B: D1-B as offered: it counts as not run, and nothing runs it later
    but the next tag or nightly run.
  - C: it fails All tests passed, as today; a re-run is needed.

## Analysis

From `.github/workflows/ci.yml` on `main` (2026-11-18):

- **macOS LaunchDaemon** runs the unit tests of `internal/update/...`,
  `internal/install/...`, `cmd/...` and `internal/safefile/...`;
  installs, runs, updates and removes the LaunchDaemons;
  and checks the announcement beside Apple's mDNSResponder,
  which `dns-sd` must find and resolve.
- **Windows service** runs the same unit tests and `internal/cubelan/...`,
  the server's tests matching
  `Update|Discovery|Search|FailedRun|FoundCube|RemovedCubeIsListedAsFound|FollowsItsAddress`;
  installs and runs the service;
  and checks the announcement beside Windows' own mDNS,
  `massimo discover` finding it by multicast.
- **Both** run today for any change of code not tested yet
  (`suite`, `code`, `tested` of What to test).
  In the last 40 runs, the macOS job ran 12 times, passed 11 and was cancelled once;
  a pull request's run is also cancelled by the next push to its branch
  (`cancel-in-progress`), and a cancelled job fails All tests passed.
- **The LAN job already has a path gate,** in What to test:
  `internal/announce/`, `internal/serve/`, `internal/sharedport/`,
  `internal/proxy/`, `internal/server/*`, `internal/cubelan/`,
  `cmd/massimo/(discover|serve).go`, `go.(mod|sum)`, `tools/lan-test/` and `ci.yml`;
  its Tested record carries `lan=unneeded`,
  so that the push of the tested tree does not run what its pull request did not need.
  The new gates follow it.
- **A compile error on darwin or windows** is caught on every pull request by
  Vet and test's `GOOS=darwin` and `GOOS=windows` vet;
  `cmd/massimo` imports nearly everything,
  so a gate on Go dependencies would run the jobs almost always:
  the gates are lists of the paths that hold the platforms' behaviour.

## Implementation Plan

A sub-agent on Opus builds a block without a commit, a second reader reads it,
and the session verifies it and makes the commits.

### Block 1: Gates

- What to test: `macos` and `windows` outputs beside `lan`,
  from the jobs' own path lists and one announcement list (D3-A),
  which the LAN gate uses too;
  on a push and a tag they stay true, and the Tested record carries
  `macos=unneeded` and `windows=unneeded` as it carries `lan=unneeded`.
- The macOS and Windows jobs' conditions read their outputs;
  All tests passed counts a job skipped by its gate as passed, as it counts the LAN job.
- The nightly run and a tag's run always run both.
- `GUIDELINES/project/CI.md`: the gates and their lists.

**Verification:**
`make actionlint`; the gate's logic against sample path lists
(each list's paths, a path of neither, a push, a tag);
the pull request's own run, which touches `ci.yml` and so runs both jobs.

### Further blocks

From the analysis of why the tests are slow, in v1.1.

## Proposed commit messages

- The plan: `chore(aps): +TestSpeed v1.0`
- Block 1:
  `ci(tests): The macOS and Windows jobs run where their code or the announcement changed`

## Change History

- v1.0 (2026-11-18): first revision, by `EXEC as recommended`,
  with D1-A as the owner extended it and D2-A to D4-A;
  one block, the rest of the plan to come from the analysis of why the tests
  are slow.
