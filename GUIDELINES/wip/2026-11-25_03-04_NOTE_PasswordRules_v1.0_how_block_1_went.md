# NOTE PasswordRules v1.0: how block 1 went

Where Block 1, Rule, went differently from the plan,
the choices taken in the run, and what verified it.
It touches no file of the page: the page's forms are Block 2's.

## What it made

A password that `massimo user add`, `passwd` or `rename` sets,
or the plain `password` of `POST` and `PUT /api/v1/users…`,
the first owner's included, has 8 to 64 code points (D4-A, D7-A).
The rule is one function, `Check` of the new package `internal/password`,
with its cases in `testdata/cases.json`,
which Block 2's check in the page reads too.
The command line says "password: at least 8 characters" (or "at most 64"),
and the API answers 400 with the same words,
before anything of the request is written.

- **One commit:**
  "fix(security:auth): A new password has 8 to 64 characters".

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **The sign-in of a short old password is held in `opaque_test.go`,
   not `login_test.go`:**
   `TestOpaqueUpgrade`'s user is now stored with a password of two characters
   straight through the store, as one set before the rule,
   so the one test holds both the sign-in and the upgrade by a record.
   `TestUsersPasswordRule` signs in with such a password as well.
2. **The operator's password was in 72 places of 20 files, not 67.**
   All became `op-password`; the tests now take it from one name,
   `OPERATOR_PASSWORD` in `tools/ui-test/helpers.py`,
   used in 69 places of 18 test files.
   Four literals stay: the constant itself, `run.sh`, which adds the operator,
   `tools/ui-test/README.md` and a docstring of `test_login.py`.
   One script of `test_sign_in_resend.py` gets the constant as an argument.
   `rm-pass` and `v1-pass` became `rm-password` and `v1-password`.
3. **The Go fixtures:**
   only a password set through the API or the command line was lengthened.
   One stored straight through the store keeps its length:
   it stands for a password set before the rule.
4. **`tools/apitypes` refused `minLength`,** which the OpenAPI file now carries:
   it joins the keywords left out of the TypeScript, as `maxLength` is,
   in the generator and its README.
5. **`cmd/massimo/parallel_test.go`:**
   the new `TestUserPasswordRule` sets `MASSIMO_DATA` and swaps stdin and stdout,
   so it is on the list of serial tests.
6. **`docs/store.md`,** not in the plan, describes `massimo user` too,
   and says the rule.
7. **The OpenAPI file:**
   `minLength` 8 and `maxLength` 64 on `POST`'s `password`,
   whose description says that an empty one counts as absent;
   `PUT`'s says the rule in words alone, since its empty password removes one.
8. **The CHANGELOG bullet** has the plan's words and, in a third line,
   that `massimo user rename` asks for a new password where the old one is
   shorter (D54-A's consequence).
9. **A test the plan did not name:**
   an integration's user given a password of one character is still answered 403,
   that it has no password at all, not 400.
   It failed when the rule was put before that refusal.

## Choices the plan left open

- **`cases.json`'s shape:**
  `{"cases": [{"password", "context", "answer", "why"}]}`, the answer `ok`,
  `too_short` or `too_long`, described in the package's comment.
  Its passwords avoid common words and their own context words,
  so that Block 3 need not change them.
  The other way: a flat list without `why`.
- **`Check(pw string, words ...string)`,** named `words`,
  not the plan's `context`,
  which shadowed the package `context` in `cmd/massimo/user.go`;
  the cases file keeps the key `context`.
  Every caller already passes the user's names,
  and `Check` refuses nothing for them yet.
- **The error texts are built from `MinLength` and `MaxLength`,**
  so that the two cannot drift.
- **Where the check stands in the routes:**
  in `POST` after the access checks and after the audit's target,
  so that a refused add still names its target in the audit,
  and before the store,
  so that a 409 for a name in use comes only after the rule;
  in `PUT` after the refusal of an integration's user and the ranks,
  and before the current password and the proof,
  so that a refused try neither uses up a proof nor counts as a failed current
  password.
  The other way: the rule first, which would tell an integration's user
  that its password is short.
- **`TestCasesCountCodePoints`** asks for every pair of edge (7, 8, 64, 65)
  and kind (ASCII, two-byte letters, beyond the basic plane),
  twelve, and the kinds do not overlap.

## Where the rule holds, and where not

- **Held:**
  the command line's `add`, `passwd` and `rename`, on a terminal and with
  `-password-stdin`, after the mismatch;
  a plain `password` in `POST` (the first owner's too) and in `PUT`.
- **A record registered by OPAQUE:**
  never seen, so not checked (D11-A): in `POST`, in `PUT`,
  at the register route, and at the upgrade of a version-1 user at its sign-in.
  `docs/api-v1.md` says that such a client holds its users to the rule itself.
- **An empty password:**
  a user with keys only, or a password removed: not checked.
- **A sign-in:**
  any password that opens a user's record or hash, of any length;
  the rehash at a sign-in keeps the same password.
- **The store's own functions** (`AddUser`, `SetPassword`) have no rule,
  so that passwords set before,
  the tests' own stores and the removal keep working.
- **`Bootstrap` and a provider's sign-in** set no password.

## What the reading found

One builder built it in an exported copy;
two readers read it, one the Go code and its security,
the other the tests, the cases, the documents and the fixtures.
Nothing that must change.
The reader of the code found the rule at every door that takes a plain password,
and a 400 that tells nothing of whether a name exists or what it holds:
every check that could tell comes before it.
The other reader saw the new refusals fail on the code before the block,
and the guards of what is exempt fail where the rule was moved too far
(before the empty password, into the sign-in);
and recounted every case's code points, bytes and UTF-16 units.

Five small points, done:

- the rename's refusal, its usage text and the README say what `-no-password`
  does: it leaves the user with its keys only;
- `context` renamed `words` (above);
- the CHANGELOG bullet names the rename's consequence;
- two more cases, 65 characters beyond the basic plane (too long)
  and 64 two-byte letters (taken), with the test asking for every pair;
- the OpenAPI description says that an empty password counts as absent.

## Where it met AP LooseEnds on main

Block 3 of AP LooseEnds (#421) adds tests to the same files of `tools/ui-test`.
The two meet in the import lines of `test_login.py` and `test_preferences.py`:
each side added a name, `OPERATOR_PASSWORD` here, `pause_clock` there;
both are kept.
One new test of #421 in `test_login.py` typed the old `op-pass`;
the merge changes it to `OPERATOR_PASSWORD`.
The builder's `grep` for a missed literal found it.

## Seen and left

- **A refused password leaves no audit row on the command line,**
  where the API keeps one with its target and the body redacted.
  Probably right, since nothing was tried against the store; the owner's.
- **`massimo user rename` checks a bad new name only after asking for the
  password;** before this block too; the owner's.
- **`fill_owner(password="pw")` in `test_first_owner.py`** types two characters
  into the first owner's form; Block 2's.
- **The page's five forms** still take any password,
  since the page sends a record; Block 2's.

## For the later blocks

- **`cases.json`** is the one list both languages are held to:
  each case a password, its context words, an answer of `ok`,
  `too_short` or `too_long`, and a `why` for the reader;
  the Go test reads it with unknown fields refused, so a new key,
  such as Block 3's answers, goes into the Go reader as well.
  Block 2's UI test holds the page's check to it, through the plan's handle on
  `window.MassimoParts`.
- **`fill_owner(password="pw")`** in `tools/ui-test/test_first_owner.py`
  passes now only because the page checks nothing;
  Block 2 lengthens it.
- **`Check` takes the words** (the user's login name and display name,
  from the command line and the server) and refuses nothing for them yet:
  Block 3 adds `ErrCommon` and `ErrContext` behind the same calls.
- **A missed fixture**, after a merge that brings new UI tests, is found in
  `tools/ui-test` by
  `grep -rnE '(^|[^-[:alnum:]])op-pass([^-[:alnum:]]|$)' --include=*.py --include=*.sh --include=*.md . | grep -v op-password`,
  and any other short plain password by
  `grep -rnE '"password": *"[^"]{0,7}"' --include=*.py .`.

## For the completion gate

- **What a user notices:**
  `massimo user` and the users' API refuse a new password under 8 or over 64
  characters and say why; the page's forms do not yet (Block 2).
- **D54-A at a rename:**
  a user whose old password is shorter is asked for a new one,
  or `-no-password`, which leaves it with its keys only.
- **A client that registers by a record is not checked:**
  the server never sees the password in it.

## What verified it

- **The builder, on the first tree:**
  `make ui`, `gofmt`, `go vet` for linux, windows and darwin,
  the whole Go suite, `make cross`, `make ui-check`,
  `go run ./tools/apitypes -check`, each exit 0;
  the race detector over `internal/server`,
  `internal/password` and `cmd/massimo`, exit 0; `test_login.py`,
  `test_users.py`, `test_opaque.py`,
  `test_parts_guard.py` and `test_sign_in_resend.py` in all four cases,
  262 passed.
  The UI test files whose only change is the name and its import are left to CI.
- **Seen failing first:**
  each refusal on the code before the block;
  `TestCases` with a count of bytes and with one of UTF-16 units;
  without the two new cases, the test of the edges names both.
- **The builder, after the five points:**
  `gofmt`, `go vet` for three platforms, the tests of `internal/password`,
  `internal/server` and `cmd/massimo`, `go run ./tools/apitypes -check`
  and `make ui-check`, each exit 0.
- **The readers:**
  the tests of `internal/password`, `internal/server`,
  `internal/store` and `cmd/massimo`, and of every package; `test_login.py`,
  `test_users.py` and `test_opaque.py` in one case, 36 passed.
- **The coordinating session:**
  the commit's message by `commit-msg.sh`, 0 errors;
  the strict changelog check, passed.
