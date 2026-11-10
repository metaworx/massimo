# AP TokenlessRequests v1.0: not counted as guesses

## TL;DR

A request to the API that offers no token, no ticket and no `?token=` is
answered 401 as before, but no longer counts toward its address's lockout;
wrong tokens, wrong tickets and wrong passwords still do.

- `authorize`: every refused request → counted → only one that offered
  something: a `Bearer` header, even an empty one, a non-empty `?ticket=` or
  `?token=` on the stream (D1-A)
- the web UI before its sign-in, a scanner, a proxy without
  `TrustedProxies` → no lockout from requests without a token
- a locked-out address → 429 on every `/api` request, without a token
  too (D2-A)
- `docs/api-v1.md`, `docs/store.md`, the CHANGELOG

### Blocks

1. Uncounted: the check in `authorize`, its tests, the two documents, the
   CHANGELOG

## Discussion

**The question** came from AP UiLockout (retired): the web UI suite's
`test_login.py` failed after `test_tls.py`, whose pages without the token
had counted eight failed token checks against 127.0.0.1.
Another session's reading of the same failure in CI added that a request
without a token is no guess, yet counts.

**Decided** at the TL;DR gate, by the owner's `PLAN++` with the recommended
options:

- **D1-A.**
  Offered is a `Bearer` header, even an empty one, or a non-empty `?ticket=`
  or `?token=` on the event stream: a header that is there but wrong is a
  guess.
  B, a non-empty secret only, would leave `Bearer ` with nothing after it
  uncounted too, for nothing.
- **D2-A.**
  A request without a token from an address that is locked out is 429, as
  now and as `docs/api-v1.md` promises ("every `/api` request").
  B, 401, would let the page open its sign-in dialog and show the 429 there,
  but changes the documented contract and the page's start; a plan of its
  own if wanted.

## Analysis

`authorize` (`internal/server/auth.go`) answers a request that does not
resolve to a principal, and is neither forbidden nor an error of the store,
with 401, and counts a failed check against the address first
(`s.limiter.fail`), whatever the request offered.
The sign-ins count against the same key, `addr:<ip>`.
Ten in a minute lock the address out for five minutes.

**Why it does not protect anything.**
A request without a token guesses nothing, so counting it adds nothing
against guessing; it only uses up the ten failures meant for the guesses.

**What it costs.**

- The web UI sends no `Authorization` header without a token
  (`app.js`, `apiRequest` and `apiRaw`), and each load of the page before
  its sign-in is one failed check: about ten reloads in a minute, or nine
  and a mistyped password, lock the address out.
- Behind a reverse proxy that is not in `TrustedProxies`, every client has
  the proxy's address (`clientIP`): the internet's scanners, which send
  requests to `/api/...` without a token all the time, lock everyone out.
- A page whose token expired still counts: its token is wrong.

**The change.**
`authorize` notes whether the request offered anything: an `Authorization`
header beginning `Bearer `, or one that is just `Bearer`, as an empty one
arrives once Go has trimmed its space, or, on the event stream, a non-empty
`ticket` or `token` parameter.
Only then does a refusal count; otherwise it is the same 401 with the same
message, "missing or wrong token".
The lockout's check before it stays as it is, so a locked-out address gets
429 whatever it sends (D2-A).
An `Authorization` header of another scheme (`Basic …`) offers nothing this
API takes, and does not count.

**Tests** (`internal/server/auth_test.go`), with the limit at three:

- requests without a token, on a route and on the stream: 401 each, and
  the right token afterwards is 200, not 429;
- `Bearer ` with nothing after it counts: three lock the address out;
- the existing `TestFailedTokensLockTheAddressOut` keeps wrong tokens and
  wrong tickets counted, and the lockout covering the stream;
- with a store, requests without a token leave no row in `auth_failures`;
- `TestRefusedRequestsAuditedPerMinute` (`audit_test.go`), which alternated
  requests without a token and with a wrong one and so relied on both
  counting, sends thirty: the ten wrong tokens lock the address out at the
  twentieth, and the audit row still counts every refusal.

## Implementation Plan

### Block 1: Uncounted

- `internal/server/auth.go`, `authorize`: an `offered` flag, set by a
  `Bearer` header and by a non-empty `ticket` or `token` on the stream; the
  default branch counts the failure only when it is set; its comment says
  why.
- `internal/server/auth_test.go`, `internal/server/audit_test.go`: the tests
  above.
- `docs/api-v1.md`, "Authentication" and "Errors": failed token checks are
  wrong tokens and tickets; a request without one is 401 and not counted.
- `docs/store.md`, "The lockout": the same.
- `CHANGELOG.md`, `[Unreleased]`, Changed: the bullet below.

**Verification:**

- `MASSIMO_ENV_FILE=/dev/null go test ./...`
- `go test -race -count=1 ./internal/server/`
- `make cross`
- `gofmt -l .` empty; `go vet ./...`, and with `GOOS=windows` and
  `GOOS=darwin`

## Proposed commit message

```
fix(server): A request without a token no longer counts toward the lockout

A request to the API that offers no token, no stream ticket and no ?token= is answered 401 as before, but no longer counts as a failed token check against its address; wrong tokens, wrong tickets and wrong passwords still do.

Key changes:
- authorize counts a refused request only when it offered a Bearer header, even an empty one,
  or a non-empty ticket or token on the event stream
- a locked-out address is still answered 429 on every /api request, with or without a token
- docs/api-v1.md and docs/store.md say which requests count; tests in auth_test.go, and the audit's
  test sends thirty refused requests, since only its wrong tokens now count

A request without a token guesses nothing, so counting it added no protection and used up the ten failures meant for the guesses: ten loads of the web UI's page before its sign-in within a minute locked the address out for five minutes, and behind a reverse proxy not listed in TrustedProxies, the internet's scanners locked out every user behind it.

AP TokenlessRequests v1.0, Block 1.

File-revision-v1: GUIDELINES/wip/2026-11-10_21-09_AP_TokenlessRequests_not_counted_as_guesses.md::v1.0::2026-11-10::All::the plan, D1-A, D2-A at its TL;DR gate
Assisted-by: claude-code:claude-opus-5-5
```

CHANGELOG bullet, `[Unreleased]`, Changed:

```
- A request to the API without a token, ticket or `?token=` is still
  refused with 401 but no longer counts toward the address's lockout:
  only wrong tokens, tickets and passwords do (`docs/api-v1.md`).
```

## Change History

- v1.0 (2026-11-10): the plan, from the TL;DR gate; D1-A, D2-A by the
  owner's `PLAN++`.
