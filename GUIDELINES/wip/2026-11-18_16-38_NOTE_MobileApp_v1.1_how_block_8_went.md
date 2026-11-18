# NOTE MobileApp v1.1: how block 8 went

Block 8, the login by fingerprint, run on 2026-11-18 at the owner's
`EXEC++ as recommended`, after the decisions of
`2026-11-18_13-47_NOTE_MobileApp_v1.1_block_8_decisions.md` (D32-B,
D33-A, D34-B, D35-A, D36-A, D37-A).
Two pull requests, as the plan says:
the server's, "AP MobileApp v1.1, Block 8: a session from an API key,
POST /api/v1/login/key", with the commits
"feat(api): A session from an API key, POST /api/v1/login/key" and
"fix(ui): The parts' API types carry a session's key_id";
and the app's, "AP MobileApp v1.1, Block 8: the login by fingerprint",
with "feat(app): The login by fingerprint" and
"fix(app): Every refusal of the Keystore ends a keep or a Turn off cleanly",
built on the first.
The owner merged the server's on 2026-11-18; the app's is open,
its CI green once a macOS job that found no runner was run again.
The plan stays open.

## Where the work went differently from the plan

- **No `login.forget()` on a server's page.**
  No page needs it: Turn off is the connect page's, and it is
  `forget(id, url)`, with the address the probe chose, since the adapter
  revokes the key there.
  `open` resolves `true`, or `false` where `{login: true}`'s prompt was
  dismissed, where version 1 resolved `null`.
- **A session counts only while its key does**, in `TokenByHash`, the
  token list and an open event stream, beyond what the plan named:
  a binary for 24 opens a file of 25 and revokes a key without its
  sessions, which a binary since 25 then refuses.
  The gate named it; `docs/store.md` says so.
  Migration 0025's down part drops the column and loses the tie alone,
  as the store's rule asks; the column has no foreign key, so that
  `DROP COLUMN` can take it.
- **Tests that named the newest schema**:
  `TestRaiseOfAFilledStore` and `TestRaiseDownLosses` expected version 24
  where they meant the newest; they read it now (`undoneFromNewest`).
  `TestAuditOfEveryWrite` took every `/api/v1/login/…` for an anonymous
  sign-in; the login by key is its key's user's.
- **`api.d.ts` was missed**: CI's `TestGeneratedFileCurrent`
  (`tools/apitypes`) found the parts' types stale after the OpenAPI file
  gained `key_id`; `make api-types` made them again, in the second commit.
  `make api-types` belongs with every change of the OpenAPI file.
- **Two holes found reading the adapter again**, fixed in the second
  commit of the app's pull request:
  a Keystore key's generation that failed otherwise than with a
  `GeneralSecurityException`, as it does where no fingerprint is enrolled,
  escaped a keep on the main thread;
  a Turn off whose key the Keystore refused otherwise than as invalidated
  left the login kept.
- **What forgets a kept login** besides Turn off, Remove and Forget all:
  a certificate trusted anew, and a node that changed,
  since the seal is bound to both (D34-B);
  an invalidated Keystore key, and a key the server refuses (401).
- **The key's label** is the phone's model and "login by fingerprint" in
  the phone's language, cut to the server's 64 bytes.
- **The cover's prompt** carries no key: it guards the session in memory,
  as the Risks say; the key's prompts carry the key's cipher.
  The cover is not drawn for the app's own prompt or file chooser,
  nor for a page signed out.
- **The user manual** (D37-A) is English alone, its one chapter the
  login by fingerprint; showing it in the app is the plan of its own
  D37-A names.
- **The plan's second fragment** ran to four lines; it is three
  (`CHANGELOG.d/Added/claude-mobile-login.md`).
- **Block 12 had not merged** when the app's pull request opened;
  it merged `main` as it stood, with GlobalMenu's change to the Cube choice
  (one comment to join). Where Block 12 lands first, `main` is merged
  into the app's branch again before it merges.

## What was run, and what not

- Locally:
  - `go test` of `internal/store`, `internal/server` and
    `tools/identifiers`, `gofmt -l .`, and `go vet ./...` for linux,
    windows and darwin;
  - `make ui-check`;
  - `test_users.py`, `test_connect.py`, `test_login.py`,
    `test_discover.py` and `test_header.py` in the four cases;
  - the Gradle tasks `:app:assembleFossDebug`,
    `:app:testFossDebugUnitTest` (49 tests) and `:app:lintFossDebug`.
- CI's: the whole suite, the race run, the cross builds, the Android job.
- **Not run**: the prompt, the Keystore keys bound to it, the cover and
  the Recents picture exist on a phone alone.
  The owner's check, the plan's Verification of Block 8 as D35-A amends it:
  - enabling at the sign-in, "Try again", then a logout and the login by
    fingerprint;
  - each of the three choices, the row's button,
    and "Select server" from the sign-in form and from the Cube choice;
  - Turn off revoking the key at the server;
  - a key revoked under My profile ending its session at the next request;
  - on Android 11 and later the PIN as the fallback, and a new fingerprint
    leaving the key; turning the screen lock off ending it;
  - "each return": the cover, and no picture in Recents with screenshots
    still allowed;
  - a server that had version 1's switch on starting signed out.
