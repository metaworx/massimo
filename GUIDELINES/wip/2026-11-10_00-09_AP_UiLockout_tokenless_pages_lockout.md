# AP UiLockout v1.0: tokenless pages lock out

## TL;DR

The web UI suite's files share a worker's server, and the pages a test
opens without the token count failed token checks against 127.0.0.1;
test_tls.py leaves 8, and test_login.py's wrong password, the 10th,
locks the address out for 5 minutes.
After every test the conftest forgets them, and checks that none is left.

- tokenless page load → one failed token check, `addr:127.0.0.1`
- test_tls.py → 8 left; test_connect.py, test_login.py → 1 each
- test_login.py after test_tls.py → 429 for 5 min → 12 failed
- after a test with `new_context` → one call with the token → count gone (D1-A)
- after every test → no `addr:` row in the set's `massimo.db`, else an error
  at teardown (D2-A)

### Blocks

1. Forgotten: the fixture that forgets and checks, the conftest's and the
   README's text

## Discussion

**Decided** at the TL;DR gate, by the owner's `PLAN++` with the recommended
options:

- **D1-A.**
  The conftest forgets the failed checks after every test that opened a
  context without the token (`new_context`): test_tls.py, test_login.py,
  test_connect.py and any later one, also when the test fails.
  B, a call at the end of each such test, is visible in the test but can be
  forgotten.
- **D2-A.**
  The check looks at the `addr:` rows only, the key every file shares.
  B, the `user:` rows too: test_opaque.py leaves eight, for throwaway users
  of its own, and no API forgets them.

## Analysis

Reproduced on main (06222b5), one worker, `test_tls.py test_login.py
-p no:randomly`: 12 failed, 16 passed;
the server's log: `api: address locked out after failed logins`
at test_login.py's first wrong password.

`authorize` (`internal/server/auth.go`) counts a request without a token as
a failed token check against the address, in the store's `auth_failures`,
key `addr:<ip>`; the sign-ins count against the same key.
Ten in a window of a minute lock the address out for five minutes (429).
A request whose token checks out forgets the count of an address not locked
out (`Succeed`).

`auth_failures` after each test, read from the set's `massimo.db`:

| After | `addr:127.0.0.1` |
|-------|------------------|
| `test_sign_in_shows_self_signed_notice`, each case | 2, 4, 6, 8 |
| test_login.py's first page | 9 |
| its wrong password | locked out |
| test_login.py alone, each test | 1 |
| test_connect.py, each `test_server_page_token_from_bridge` | 1 |

Every other file leaves none: its pages carry the token, and each checked
token forgets the count.
The one-minute window is why it looks flaky: a slower run or another order
of the files lets the count expire.

**The forgetting** is a call to the API with the token from the test
itself, `GET /api/v1/me`: the same address, 127.0.0.1, as the browser's.
It runs in the teardown of an autouse fixture, which is set up before
pytest-playwright's `new_context` and so torn down after it, once its
contexts are closed and can send nothing more.

**The check** reads `auth_failures` from the set's store,
`massimo.db` beside `UI_TEST_SERVE_LOG`, read-only, after the forgetting;
so it also fails when the forgetting stops working, or the address is
locked out.
Without `UI_TEST_SERVE_LOG` (a server started by hand) it is skipped.

## Implementation Plan

### Block 1: Forgotten

- `tools/ui-test/conftest.py`: an autouse fixture whose teardown, after a
  test that used `new_context`, calls `GET api/v1/me` with the token, and
  then, after every test, fails when the set's store has an `addr:` row in
  `auth_failures`, naming its count and lockout;
  the module docstring says so.
- `tools/ui-test/README.md`: the same, where the suite's sharing of a
  server is described.
- No CHANGELOG bullet: `tools/**` is not shipped.

**Verification:**

- The reproduction above: 28 passed.
- With the forgetting switched off, test_tls.py's sign-in test errors at
  teardown with the count; switched on again.
- `make ui-test`, 4 workers: passes.
- `gofmt`/`go vet` untouched (no Go change).

## Proposed commit message

```
test(ui): Tests that open pages without the token forget the failed checks

A page without the token counts a failed token check against 127.0.0.1, and the files of the web UI suite share their worker's server: test_tls.py left eight, so test_login.py's wrong password, the tenth within a minute, locked the address out for five minutes and failed all twelve of its tests whenever it ran after test_tls.py on the same worker.

Key changes:
- conftest.py: after a test that opened a context without the token, one request with the token
  forgets the address's failed checks
- conftest.py: after every test, the set's store has no failed checks counted against an address,
  or the test errors at teardown
- README.md: the files are independent of the lockout, and the check that keeps them so

AP UiLockout v1.0, Block 1.

File-revision-v1: GUIDELINES/wip/2026-11-10_00-09_AP_UiLockout_tokenless_pages_lockout.md::v1.0::2026-11-10::All::the plan, D1-A, D2-A at its TL;DR gate
Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-10): the plan, from the TL;DR gate; D1-A, D2-A by the
  owner's `PLAN++`.
