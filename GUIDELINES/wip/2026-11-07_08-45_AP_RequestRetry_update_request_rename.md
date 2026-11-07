# AP RequestRetry v1.0: Retry the update request's rename

## TL;DR

A second `POST /api/v1/update` on Windows can answer 500,
its rename over `update-request` meeting the helper's open handle;
the rename waits and tries again before it reports 500,
and one request at a time is written.

- `WriteRequest`'s `os.Rename` → `renameRetrying`,
  as the status file has it (Windows: 50 ms apart, ten tries)
- after the retry: `Start` → `ErrHelperRunning` → 409 "(starting)";
  the leftover request goes to the next run, as the code allows
- the server: a lock around the request's write and the helper's start (D1)
- tests first, failing on `main`:
  the request held open as the helper holds it, on Windows through interop;
  two POSTs at once
- `GUIDELINES/project/CI.md` §7 v1.10.1: "the 409 comes first" corrected
- two bullets under Fixed in `CHANGELOG.md`

### Blocks

1. RequestRetry: the tests, the retried rename and the lock,
   CI.md and CHANGELOG.md;
   verified by the test gate, the Windows test binary through interop,
   and the pull request's Windows service job.

## Discussion

### D1. The lock around the request: A, include it

D13 at the TL;DR gate of 2026-11-07, settled as recommended by `PLAN++`.
Two POSTs at once, from two tabs or a double click,
both pass the status check and both write `update-request.tmp`;
the second rename then finds no `.tmp` or the helper's handle,
the same 500 by another way.
A lock held across the write and the start makes the second wait for the first,
whose started helper then answers it with 409.
B, the retry alone, leaves that way open.

## Analysis

### 1. The failure

#72's Windows service job, first attempt of run 42325046860 (2026-11-07):

```text
"error": "asking the update helper: rename C:\ProgramData\Massimo\update-request.tmp C:\ProgramData\Massimo\update-request: Access is denied."
##[error]the second POST answered 500, want 409
```

The re-run passed: a race of a few milliseconds.

### 2. Why the status check lets the second POST through

`startUpdate` (`internal/server/update_api.go`) answers 409 while the helper's
status says running, and after `WriteRequest` when `Start` reports
`update.ErrHelperRunning`.
Between the helper's `TakeRequest`,
which opens, reads, closes and removes the request,
and its first status, neither holds:
the second POST reaches `WriteRequest`.
`TakeRequest` opens with `openNoFollow`, on Windows `os.Open`,
which Go opens without `FILE_SHARE_DELETE`;
a rename over a file open that way fails with "Access is denied"
until the handle is closed, a few milliseconds later.

### 3. The status file has the retry already

`WriteStatus` renames through `renameRetrying` (`internal/update/helper.go`),
ten tries 50 ms apart on Windows and none elsewhere,
for the server reading the status in the same moment;
`WriteRequest` has a plain `os.Rename`.
The same helper serves both, so there is one rule for the two files.

### 4. After a successful retry

`Start` then reports `ErrHelperRunning`,
since the helper it started is still at work,
and the answer is 409 "(starting)", which the CI job wants.
The request written stays;
the existing comment says why that is harmless:
it names the same channel and goes to the next run,
and on Windows only the server starts the helper.
On Linux and macOS a rename over an open file succeeds;
the retry never runs there.

### 5. Risks

A POST may wait up to half a second for the rename,
and a second POST for the first's write and start;
the GETs are not locked.

## Implementation Plan

### Block 1: RequestRetry

1. Tests, each failing on `main` first:
   - `internal/update/helper_windows_test.go`:
     `TestWriteRequestWhileTheHelperReadsIt` holds the request open as
     `TakeRequest` does and writes a new one;
     run on the Windows side through interop,
     the test binary cross-compiled with `GOOS=windows`,
     in a temporary directory.
   - `internal/server/update_api_test.go`:
     `TestUpdatePostsOneAtATime`,
     two POSTs at once with a starter that notes an overlap,
     want one 202 and one 409 and no overlap.
2. `WriteRequest` renames through `renameRetrying`; its comment says why.
3. `startUpdate` holds the server's update lock across `WriteRequest` and `Start`.
4. `GUIDELINES/project/CI.md` §7, v1.10.1:
   the second POST can meet the helper's open request,
   which the retry rides out.
5. `CHANGELOG.md`, under Fixed, a bullet for each commit.

**Verification:**
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `-race` for `internal/server`,
`gofmt -l .` empty, `go vet` for linux, windows and darwin, `make cross`,
`GUIDELINES/shared/tools/check --commit-range origin/main..HEAD`;
the Windows test binary of `internal/update` through interop;
the pull request's Windows service job.

## Proposed commit message

```text
fix(update): retry the request's rename while the helper reads it

On Windows a second POST /api/v1/update that came while the helper was reading the request answered 500, its rename over update-request refused while the helper's handle was open; WriteRequest now renames through renameRetrying as the status file does, and the POST answers 409 from the starter (AP RequestRetry v1.0, Block 1).
```

```text
fix(server.update): write one update request at a time

Two POSTs /api/v1/update at once both wrote update-request.tmp and could answer 500 from the second rename; the server's update lock now covers the request's write and the helper's start, so the second waits and answers 409 (AP RequestRetry v1.0, Block 1, D1).
```

## Change History

- v1.0 (2026-11-07): the first revision,
  from the TL;DR gate of the same morning;
  D1 (D13 at that gate) settled as recommended by `PLAN++`.
