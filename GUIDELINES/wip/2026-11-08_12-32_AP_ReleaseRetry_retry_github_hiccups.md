# AP ReleaseRetry v1.0: Retry GitHub's hiccups

## TL;DR

The update client tries a request twice more when GitHub answers 502, 503
or 504, or the connection fails before an answer, rather than giving up at once.

- `get` (release list, manifest, signature): one attempt → up to three,
  2 s then 5 s apart, the waits cut short by the caller's context
- `Download`: the same until the body starts; a failure inside it ends the
  download as now
- not retried: 403, 429 (a rate limit), 404, a timeout, a context that ended
- the last error names its attempts: `… 504 Gateway Timeout (3 attempts)`
- `retryWaits` a variable, so that the tests do not wait
- a CHANGELOG bullet under Changed; CI unchanged

### Blocks

1. The retry in `get` and `Download`: `internal/update/update.go`, tests, CHANGELOG
2. The branch pushed, its pull request opened

## Discussion

On 2026-11-08 the Windows service and macOS LaunchDaemon jobs of pull requests
107 and 109 failed together:
GitHub answered `504 Gateway Timeout` for `v0.11.0-alpha.1`'s
`massimo-release.json` from 09:11:50 to at least 09:14:23,
each answer up to a minute in coming.
The owner asked whether the jobs need a token against a rate limit:
they have one, for the API, and the download needs none;
a retry was proposed instead, and the owner answered its TL;DR gate with
`PLAN++` as recommended:

- **D1-A.** Three attempts, 2 s and 5 s apart:
  GitHub's short outages pass, and the server's check stays inside its 30 s.
  Waiting up to a minute would ride out more by hand,
  but the server's budget cuts it off, and the UI waits.
- **D2-A.** CI stays as it is: an outage of minutes still fails the jobs
  that update against the real alpha channel, and a re-run of the failed jobs
  is the remedy.
  A loop of five minutes in those steps would make each run that long
  whenever GitHub is down.

This retry would not have saved those four jobs; it serves the short
outages, on every box.
The plan is committed into `main` before the work branches off,
as the owner asked of CodeqlAlerts.

## Analysis

- **The requests** (`internal/update/update.go`):
  `get` serves `list` (the API's release list, the token sent there only)
  and `verify` (the manifest and its signature, from the release's download
  URLs), with `c.HTTP`, whose `Timeout` is two minutes (`NewClient`);
  `Download` sends its own request with that client's `Timeout` at 0,
  bounded by the context and by `Idle`, a pause in the data
  (`DownloadIdle`, a minute), whose timer starts before the request.
  Each returns `GET <url>: <status>` on anything but 200.
- **The callers**: the server's check (`internal/server/update_api.go`,
  `latest`) gives `Latest` 30 s and keeps the result, an error too,
  for `checkEvery` (10 minutes);
  `massimo update` (`cmd/massimo/update.go`) calls `Latest` or `ByTag`,
  then `Download`, with no deadline of its own.
- **What is retried.** 502, 503 and 504 are a gateway's or a server's
  passing state.
  A transport error before any answer (a refused or reset connection, a name
  that did not resolve) is too, unless it is a timeout - a request the client
  gave two minutes, three times over, is not worth it - or the context's end.
  403 and 429 are GitHub's rate limits, where seconds do not help and the
  token does; every other status is an answer.
- **The waits** stop at the context's end,
  and the last answer is returned as it came, with its attempts counted.
  The server's check, three requests of up to three attempts each,
  stays bounded by its 30 s; an answer that takes a minute, as on
  2026-11-08, meets that deadline first.
- **`Download`'s stall timer** is set again before each attempt,
  so that the idle time counts from the request being sent, as now;
  a response whose body has started is not retried,
  since `w` holds its bytes and the caller discards them on an error.
- **Bodies** of a retried response are drained (a few KiB) and closed,
  so that the connection is reused.
- **Messages**: an error after one attempt reads as now;
  after more it ends ` (n attempts)`.
- **Tests** (`internal/update/update_test.go`, `newFakeGitHub`):
  no current test meets a 5xx or a closed port,
  so the default waits slow none of them;
  the new ones set `retryWaits` to milliseconds.
- **Docs**: none describe the client's errors;
  `docs/update-windows.md` names only `WriteStatus`'s rename retry.

## Implementation Plan

The branch `claude/release-retry` from `main` after this plan's commit.

### Block 1: The retry

- `internal/update/update.go`:
  - `retryWaits = []time.Duration{2 * time.Second, 5 * time.Second}`,
    a variable for the tests;
  - `retryable(ctx, resp, err)`: 502, 503, 504, or a transport error that is
    no timeout while `ctx` lasts;
  - `do(ctx, hc, newReq)`: sends, waits and sends again as `retryable` says,
    returns the last response or error and the attempts;
  - `get` and `Download` send through it, `Download` setting its stall timer
    again in `newReq`; their errors end ` (n attempts)` where n > 1.
- `internal/update/update_test.go`:
  - `TestRetriesGatewayErrors`: the manifest answers 504 twice, then 200:
    `ByTag` succeeds after three requests to it;
  - `TestRetryGivesUp`: the list answers 503 always: three requests,
    the error ends `(3 attempts)`;
  - `TestNoRetryOfAnAnswer`: 404 and 429 get one request each;
  - `TestRetryWaitEndsWithTheContext`: a cancel during the wait returns at once;
  - `TestDownloadRetriesItsStart`: the binary answers 503, then 200,
    and verifies.
- `CHANGELOG.md`, `### Changed`:

  ```
  - The update check and an update's download try again twice, after 2
    and 5 seconds, when GitHub answers 502, 503 or 504 or the connection
    fails before an answer.
  ```

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`,
`go test -race ./internal/update`, `make cross`, `gofmt -l .` empty,
`go vet` on linux, windows and darwin.

### Block 2: The pull request

- Push `claude/release-retry`, open its pull request.

**Verification:** the pull request's checks are green.

## Proposed commit message

- `feat(update): A 502, 503 or 504 from GitHub is tried again twice`

## Change History

- v1.0 (2026-11-08): the first revision, from the TL;DR gate of the owner's
  `PLAN the retry`; D1-A and D2-A by `PLAN++` as recommended;
  committed into `main` before the branch, as the owner asked.
