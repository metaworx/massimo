# NOTE TestSpeed v1.1: how blocks 1 and 2 went

Where Blocks 1 and 2 went differently from the plan, and what verified them.

## Block 1, Gates (pull request #371)

1. **The second reader found two gaps the builder's tests passed over:**
   GitHub reports a job no runner took as `abandoned` in `needs`, not `cancelled`,
   so D5-A would never have taken effect;
   and darwin-only code outside `cmd/massimo` (the Cube client's hold, the fake
   Cube's listener) was in no gate.
   A test now holds the gates to the files `go/build` builds per system.
2. **The record says `mac=` and `win=`:** a status description holds 140
   characters, and the record is now at the limit.
3. **CI.md went to v1.63.0:** another plan had raised it to v1.62.0 meanwhile.
4. **A network failure of the Android job** on its run was run again,
   and the first comment of a job that passed only on a re-run was posted.

## Block 2, Retries

1. **Larger than planned:** the two jobs moved into a reusable workflow,
   `platforms.yml`, called by CI as Platforms and by the nightly;
   the re-runs are a workflow of their own, `retry.yml`,
   since a run cannot re-run itself while it runs;
   the nightly's pick of what to ship moved out of `build.yml`.
2. **D15-B on the commit the nightly ships,** not on `main`'s head:
   the two differ only where the head is red, and the commit shipped is the one
   that must have passed.
3. **The second reader found two holes, fixed before the commit:**
   `retry.yml` could have acted on a pull request's run named Nightly;
   it now checks the workflow's path, the event, the repository and a tag's
   commit, and its script checks them again;
   and a re-run on `main` could have spared a job that had failed for real in
   an earlier attempt; a job is now spared only where no earlier attempt of it
   failed.
4. **A skipped night is red,** so that the owner is told.
5. **Seen only on GitHub:** what a caller is given for a called workflow's job
   that no runner took, whether a re-run takes cancelled jobs, and a release in a
   re-run attempt; every unknown answer fails closed.

## Verification

- **Block 1:** `make actionlint`, `shellcheck`, `go test ./packaging/`,
  the gate's step against sample paths and events, the guidelines check;
  its pull request's run, which ran both jobs.
- **Block 2:** the same, with the retry's, the pick's and the report's cases
  against a fake `gh` (a pull request's run refused in eleven ways,
  a re-run hiding a failure, an unreadable state), faults planted for each fix.
