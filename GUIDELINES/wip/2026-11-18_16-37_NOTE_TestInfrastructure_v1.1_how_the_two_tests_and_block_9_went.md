# NOTE TestInfrastructure v1.1: how the two tests and block 9 went

Where the rest of Block 8 (the two tests the owner read, D19-A and D20-A)
and Block 9, Ci, went differently from the plan and the decisions,
and what verified them.

## The two tests (pull request #363)

1. **D19-A as decided:** `TestDisagreeStartsAgainOnceTheWriteConfirms`, a test of
   its own; `TestDisagreeResendsMassimosSetpoint` is unchanged.
   It fails where the disagreement is kept through the confirmation,
   and where the room is not left out while it confirms; the old test passed both.
2. **D20-A, measured on every node in view, not on one element above it:**
   the builder's first version took one element, the one it held the scroll
   anchoring would keep; the second reader traced Blink's own anchor
   (`FindAnchor`) at 101 jumps and found that pick matched it at 15,
   so that a correct page could fail.
   The test now keeps the top of every element and text node that reaches into
   the view at the jump and allows a forward move up to the farthest any of them
   went down: the anchor is always among them.
   It is looser where something inside the view grows, and safe by construction;
   the owner is asked at the completion gate.
3. **A flake on its run** that the change did not touch:
   `test_pile_list_as_wide_as_its_content` at 390 px (GlobalMenu, #362)
   failed in the first pass and passed in the second;
   the failed jobs were run again and passed, and the owner merged it.
   Its cause goes to the session that wrote it, "Web UI updates".
4. **The machine restarted** in the middle of the run (about 15:41);
   the two agents at work were resumed from their transcripts.

## Block 9 (pull request #367)

1. **D10-A with the owner's comment, as decided:**
   a job that passed only on a re-run is named in the summary, in a warning and
   in the report, and a comment on the pull request tells its author.
   Seen on the pull request itself: its first run cancelled while eleven jobs
   ran, the failed ones run again, and the comment named the eleven.
2. **Two commits:** `make race` and `make test-armv6` with the contract's test
   gate; then the workflow, the scripts and `CI.md`.
3. **Beyond the plan:** the contract's test gate lists `make race`'s ten
   packages, so the contract went to v1.16.0;
   All tests passed's steps that call GitHub got time limits of their own,
   after the second reader;
   the report of skipped tests keeps a test's held output to its first 1000
   and last 200 lines, and loses no line where one cannot be read.
4. **The skipped tests are reported by three jobs only:**
   Vet and test, Race, and Test (armv6, QEMU);
   the Windows and macOS jobs run their tests as before.
5. **Measured locally:** `make race` passed in 689 s, `internal/server` alone
   682 s; `make test-armv6` in 1027 s, `internal/server` 975 s:
   both past the 10 minutes they ran under before (D14-A).

## Verification

- **The two tests:** each with its planted fault, old and new;
  the Go test under `-race` at scale 2, 20, 10 and 3 times;
  `test_scroll.py` in the four cases and with the page's poll at 300 ms,
  the forward jump of 40 px failing and 40 px of growth passing;
  five synthetic pages against Blink's trace.
- **Block 9:** `make actionlint`, `shellcheck`, `go test ./packaging/`
  (both scripts against a fake `gh`, real attempt fixtures and made-up streams,
  planted faults failing them); `make race` and `make test-armv6` locally;
  the guidelines check (12 of 12); the re-run comment on the pull request.
- **Left unseen:** the skipped tests' table in a job's summary, which the API
  does not return; the run's page shows it.
- **Pull requests #363 and #367,** both merged.
