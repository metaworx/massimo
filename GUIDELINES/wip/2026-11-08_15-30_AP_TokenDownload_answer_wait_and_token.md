# AP TokenDownload v1.0: Answer wait, token downloads

## TL;DR

Every request to GitHub gets 10 s for its answer and is sent again when none
comes; with a token, a release's files come through GitHub's API,
as its list already does.

- `do`:
  each attempt 10 s to its answer (status and headers) → a missing answer
  retried like a 504; the body keeps its bounds (the client's 2 min,
  a download's idle minute)
- the server's 30 s check: a file that never answers → attempts at 0,
  12 and 27 s
- with `MASSIMO_GITHUB_TOKEN`: manifest, signature, binary from
  `api.github.com/…/releases/assets/<id>`, `Accept: application/octet-stream`
  → not github.com's download pages; the token to the API only,
  dropped by Go on the redirect
- without a token: the URLs as now
- `answerWait` a variable for the tests
- CHANGELOG: the retry bullet amended, one bullet for the token
- docs: `packaging/README.md`, `GUIDELINES/project/CI.md` §5,
  the two workflow comments

### Blocks

1. The answer wait, then the token's path: `internal/update/update.go`,
   tests, docs, CHANGELOG, a commit each
2. The branch pushed, its pull request opened

## Discussion

On 2026-11-08 the Windows service job of `main`'s run 42477324189 failed
after ReleaseRetry had merged:
the server's release check right after the service started (11:50:34 UTC)
and the check of the update's POST a minute later both ended

```
Get "https://github.com/metaworx/massimo/releases/download/v0.11.0-alpha.1/massimo-release.json": context deadline exceeded
```

one attempt each, hanging for the check's whole 30 s,
and `POST /api/v1/update` answered 502.
The release list, through the API with the job's token, had come.
GitHub had an incident open since 10:36 UTC,
"Increase in timeouts for some unauthenticated users".
The owner asked whether the failing download was an install script or a test
helper: it is neither, it is the installed service's own update check,
the code every box runs.

The TL;DR gate proposed fetching the release files through the API when a token
is set.
The owner proposed instead the plain download first, 10 or 20 s per attempt,
retried twice, then the token, retried once.
The evaluation kept the timeout per attempt and the token's path,
and recommended against the order:
three plain attempts of 10 s and their waits take 37 s for one file,
past the server's 30 s, so the fallback would never be reached there,
and where a token is set it would always wait the failure out first.
What the order would save is a few API requests per check.
The owner answered `PLAN++`:

- **D1-A.**
  With a token the files come through the API from the first attempt;
  every attempt, with a token or without,
  gets 10 s for its answer and is retried as ReleaseRetry retries a 504.
  Not taken, D1-B: the owner's order, with the server's budget at about 90 s.
- **10 s per attempt**, not 20 s: two full attempts fit in the 30 s check,
  where 20 s leaves room for one.

The plan is committed into `main` before the work branches off.

## Analysis

- **Where it hung.**
  net/http's error names the request of the hop that failed,
  the redirect's target when the target hangs (checked with a server that
  redirects to one that never answers: the error names the second).
  The error names `github.com/…/releases/download/…`,
  so github.com's own answer, its redirect to the file host, never came.
  The API's asset URL does not go through github.com:
  the API answers the redirect to the file host itself.
- **The requests** (`internal/update/update.go`): `get` serves `list` (the API,
  the token sent there only,
  as `strings.HasPrefix(url, c.API+"/")` decides) and `verify` (the manifest
  and its signature, from `browser_download_url`), with `c.HTTP`,
  whose `Timeout` of two minutes covers each attempt with its body;
  `Download` copies that client with `Timeout` 0,
  bounded by the context and by `Idle` (`DownloadIdle`, a minute),
  whose stall timer is set again before each attempt.
  Both send through `do`, which retries 502, 503, 504 and a transport error
  that is no timeout while the context lasts (ReleaseRetry).
- **The callers**:
  the server's `latest` (`internal/server/update_api.go`) gives `Latest` 30 s
  and keeps the result, an error too, for `checkEvery` (10 minutes);
  `massimo update` (`cmd/massimo/update.go`,
  the helper the POST starts) calls `Latest` or `ByTag`, then `Download`,
  with no deadline of its own.
- **The answer wait.**
  An attempt gets its own context,
  ended with `errNoAnswer` after `answerWait` unless the answer (status
  and headers, after any redirects) came first; the timer is stopped then,
  and the context ends when the body is closed.
  A wait that ran out at the moment the answer came counts as no answer,
  its body closed, since its context is ending.
  The error reads `GET <url>: no answer within 10s`,
  ` (n attempts)` added as now.
  It does not bound the body: the release list may be some MiB, which a Pi
  on a slow line takes longer than 10 s to receive.
- **What is retried.**
  A missing answer is, while the caller's context lasts:
  it is the client's own short wait, not one it gave its full time.
  The client's `Timeout` and the caller's context are still not;
  nor are the answers ReleaseRetry leaves alone (403, 429, 404, 500).
- **The budget.**
  In the server's check a file that never answers is sent at 0,
  12 and 27 s (10 s, a wait of 2, 10 s, a wait of 5),
  the third cut off by the deadline; the check stays at 30 s.
  In `Download` the idle minute is longer than the answer wait, so the
  answer wait ends a hanging start first.
- **The API's asset URL.**
  GitHub's release list gives each asset a `url`,
  `https://api.github.com/repos/<repo>/releases/assets/<id>`;
  a GET of it with `Accept: application/octet-stream` answers 302 to the file
  host,
  and without that header the asset's description in JSON. net/http drops
  `Authorization` on a redirect to another hostname,
  so the token goes to the API only.
  `verify` takes that `url` where a token is set and it starts with
  `c.API + "/"`, else `browser_download_url` as now; whatever the list says,
  the token goes nowhere but `c.API`.
- **Quota.**
  With a token a check costs up to three more API requests (manifest
  and signature of the release found, one more pair for each release skipped),
  an update one more for the binary; the check runs at most every 10 minutes,
  against 1,000 requests an hour for CI's token and 5,000 for a personal one.
- **Without a token** nothing changes but the answer wait.
  `MASSIMO_UPDATE_API`, CI's stubbed release in stage 2,
  clears the token (`NewClient`),
  so the stub's `browser_download_url`s are fetched as now and it needs no
  `url`.
- **Tests** (`internal/update/update_test.go`, `newFakeGitHub`):
  it serves the API and the files from one server;
  it gains a hang (a path held without an answer until the request ends) and the
  assets' `url`,
  served as the file for `application/octet-stream` and as JSON otherwise,
  or as a redirect to another hostname,
  which a transport that dials the fake for any address reaches.
- **Docs**:
  `packaging/README.md` says the token is sent to the API only and lifts its
  limit;
  `GUIDELINES/project/CI.md` §5 names `MASSIMO_GITHUB_TOKEN` "for the release
  checks against the API's rate limit";
  the Windows and macOS jobs' comments in `ci.yml` say it only lifts that limit;
  `TokenVar`'s comment says the same.

## Implementation Plan

The branch `claude/token-download` from `main` after this plan's commit.

### Block 1: The answer wait and the token's path

Commit 1, the answer wait:

- `internal/update/update.go`:
  - `answerWait = 10 * time.Second`, a variable for the tests,
    and `errNoAnswer`;
  - `try(ctx, hc, newReq)`:
    one attempt under its own context,
    ended with `errNoAnswer` after `answerWait` unless the answer came;
    the response's body ends that context when it is closed;
  - `do` sends each attempt through `try`,
    `newReq` taking the attempt's context;
  - `retryable`:
    an error that is `errNoAnswer` is retried while `ctx` lasts.
- `internal/update/update_test.go`:
  - `fakeGitHub.hangFirst(path, n)`:
    the first n requests get no answer;
  - `TestRetriesWithoutAnAnswer`:
    the manifest hangs twice, `ByTag` succeeds after three requests to it;
  - `TestNoAnswerGivesUp`:
    the list hangs always: three requests,
    the error says `no answer within` and `(3 attempts)`;
  - `TestDownloadRetriesWithoutAnAnswer`:
    the binary hangs once, is downloaded on the second request and verifies;
  - `TestRetryable`:
    a missing answer is retried, not after the context's end.
- `CHANGELOG.md`, the retry bullet under `### Changed` amended:

  ```
  - The update check and an update's download try again twice, after 2
    and 5 seconds, when GitHub answers 502, 503 or 504, gives no answer
    within 10 seconds or the connection fails before an answer.
  ```

Commit 2, the token's path:

- `internal/update/update.go`:
  - the assets of `ghRelease` gain `API string json:"url"`;
  - `verify` keeps for each asset the API's URL where a token is set and it is
    on `c.API`, else `browser_download_url`;
  - `c.request(ctx, url, accept)`, shared by `get` and `Download`:
    a request to `c.API` carries the token, and one for a release file there
    asks for `application/octet-stream`;
  - `TokenVar`'s comment: the list and, with the token, the files.
- `internal/update/update_test.go`:
  - `fakeGitHub`:
    the assets' `url`, served as above,
    and a redirect of them to another hostname;
  - `TestTokenFetchesFilesThroughTheAPI`:
    with a token the manifest,
    the signature and the binary come from the asset paths with the token,
    `/dl/` gets no request,
    and the file host the API redirects to gets no token;
    without a token `/dl/` serves them as now.
- `packaging/README.md` (the paragraph on `MASSIMO_GITHUB_TOKEN`),
  `GUIDELINES/project/CI.md` §5 (v1.13.2) and the two comments in
  `.github/workflows/ci.yml`:
  with the token the release files come through the API too.
- `CHANGELOG.md`, `### Changed`:

  ```
  - With `MASSIMO_GITHUB_TOKEN` set, an update's files come through
    GitHub's API as its release list does, not from github.com's
    download pages (`packaging/README.md`, "Updating").
  ```

  (its wording fitted to the section's name and the width when written)

**Verification:**
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `go test -race ./internal/update`,
`make cross`, `gofmt -l .` empty, `go vet` on linux, windows and darwin,
the Guidelines `check` over the branch's commits.

### Block 2: The pull request

- Push `claude/token-download`, open its pull request.

**Verification:**
the pull request's checks are green;
the Windows service and macOS LaunchDaemon jobs fetch the release files through
the API (their tokens are set).

## Proposed commit message

- `feat(update): A request without an answer in 10 seconds is sent again`
- `feat(update): With a token, a release's files come through GitHub's API`

## Change History

- v1.0 (2026-11-08): the first revision, from the TL;DR gate on the
  owner's report of run 42477324189 and the evaluation of the owner's
  order (the plain download first, then the token);
  D1-A and 10 s per attempt by `PLAN++`;
  committed into `main` before the branch.
