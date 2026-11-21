# NOTE MobileApp v1.3: phone check block 14

The owner's check of Block 14 on the phone, on 2026-11-21
at the owner's `EXEC phone, D58-A, D59-A`,
after metaworx/massimo#390 ("fix(app): The second phone check's findings:
the sign-in card, Forget all revoking and the keys left") had merged:
how it ran, the six steps of Block 14's NOTE
(`wip/2026-11-21_03-25_NOTE_MobileApp_v1.3_how_block_14_went.md`,
"The phone check to propose"), what it found,
and the decisions taken on it.

## How it ran

As the check of Block 13
(`wip/2026-11-20_23-04_NOTE_MobileApp_v1.2_phone_check_block_13.md`):
the owner's phone over WSL's own `adb`,
the `foss` debug build of `main` before metaworx/massimo#397
(AP UiParts' sign-in half) landed, installed over the one before.
The rig's two servers, built from that `main`,
were both reached through `adb reverse`:

- **A**, `serve -tls self-signed` against the fake Cube, its store kept;
- **B**, a second `serve` with no Cube and at first no users,
  which showed the first owner's form.

The server's audit was read from each store,
the app's warnings from the phone's log; none came.
The wireless debugging link dropped twice;
a watcher on the host restored the two reverse ports.

Once, during step 3, A locked the phone's address out:
after an `adb reverse` connection stalled,
the WebView sent requests again with the session that had ended,
and ten refused tokens within a minute lock an address out for five minutes.
Restarting A cleared it, the rig's start clearing `auth_failures` (D64-A).

## The six steps

1. **The sign-in card**: passed, with F29, F30 and F31.
2. **Forget all**, one server holding a session and one not:
   not checked as planned, since both servers had been signed out before
   the step and neither held a session;
   it revoked both keys, with two fingerprint prompts,
   each server's login having a Keystore key of its own (D63-A).
3. **A key left**: Turn off with A stopped left the key valid,
   and A's settings named it,
   "A key of this phone is still valid here: key N of the user",
   with Show in My profile.
   The next sign-in as that user, whichever way,
   Show in My profile not needed,
   revoked it within a second: keys 40, 45, 50 and 53 in A's audit.
   Show in My profile marked the key under My profile; but see F37.
4. **Remove with a kept login**: passed;
   the key revoked by itself (keys 56 and 58),
   and the toast "… removed; its key is revoked at the server,
   and the app signed out there".
5. **Turn off while signed in**, Block 13's step 12 again: passed;
   A's audit shows key 43 revoked by the session, then the session's logout,
   in that order.
   F28 is settled: Turn off uses the session.
6. **A server asked at each return**: the cover asks once the app comes back
   to that server's page;
   left on the server list, the app comes back without asking,
   the list not covered, which the owner judged right; but see F40.

## Findings

- **F29** (D60-A): the card's place flickers when switching between
  Password and API key (the owner's video).
  Fixed on `main` by metaworx/massimo#397, after the build checked here;
  not yet seen on the phone.
- **F30** (D61): the first owner's form is not mandatory,
  and sits beside the sign-in.
- **F31** (D62-A): the sign-in card does not show which server it signs in to,
  and the app has no address bar.
- **F32**, open: a request that never answers leaves Revoke spinning for good.
  A time limit on the page's API calls is proposed;
  who builds it is still to be weighed.
- **F33**: Show in My profile is enabled while the server is unreachable,
  and tapped it fails with a toast;
  it should be disabled while there is no connection.
- **F34**: Remove sits beside Cancel and Save.
  The owner would put it on a line of its own below them, after a rule,
  as "Remove server" or with a line saying what it does:
  it removes the server from the list and,
  while the server is reachable, revokes this phone's session or key there.
- **F35** (D65): a password sign-in that turns the login by fingerprint on
  leaves its new key showing "never used", with no tag:
  the session came from the password, so "its key" does not apply,
  and nothing says the key is this phone's.
- **F36** (D68-A): a password sign-in that revoked a key left was followed
  by a second sign-in card;
  the owner signed in again, and the first session stayed valid at A,
  unused (sessions 47 and 48).
  Traced in the code, likely but not proven:
  a poll sent without a token while the card is open gets its 401 after
  `login/finish` has answered;
  `dropToken("")` forgets nothing, but `api()` still calls `askLogin`,
  which opens a fresh card;
  the second sign-in's `setToken` replaces the first session,
  and nothing signs that one out.
- **F37** (D67-A): Show in My profile marked a key that the sign-in it led to
  had already revoked, and its Revoke failed, the key not existing;
  A's audit has the revoke by the session,
  then a failed revoke two minutes later.
  Block 14's NOTE had listed it among the small things left.
- **F38** (D69-A): Remove of a server without a kept login forgets the app's
  session without signing it out; it stays valid at the server.
- **F39** (D70-A): Turn off, Forget all or Remove while the server is down
  forget a password session on the phone alone;
  it stays valid until it expires,
  and nothing records it, as `unrevoked` records keys.
- **F40** (D72): with a server asked at each return,
  the app left on the server list and brought back,
  opening that server from the list enters with the session kept in memory,
  without the fingerprint:
  the fingerprint asked at each return is bypassed.

## Decisions

- **D58-A**, at the signal: Remove with a kept login turns it off first,
  as Block 14 built it.
- **D59-A**, at the signal: the merge of metaworx/massimo#390,
  made with GitHub's button, stays as it is,
  though its summary fails the commit-msg check.
- **D60-A** (F29): on a touch screen the card focuses no field,
  and both ways keep one form height;
  built by AP UiParts in metaworx/massimo#397.
- **D61** (F30): the first owner's card, the owner's in AP UiParts' session
  and planned there as AP FirstOwner (on `main`, running):
  a card with three options, create the owner, skip for now, keep open;
  keep open as the server setting `auth.keep_open`, after a confirmation,
  and `kept_open` in `GET /api/v1/me`.
  FirstOwner.D18-A puts one task on this plan:
  the block of the phone's own server (Block 7, Local),
  open only on the phone itself, sets `auth.keep_open`,
  and clears it when that server is opened to the LAN.
- **D62-A** (F31): the app shows the server under the card's title,
  with an ⓘ for its details, from the bridge (the contract's version 5).
- **D63-A** (step 2): one wrapping key,
  so that Forget all asks the fingerprint once.
- **D64-A** (the lockout): the server does not count a token it issued,
  and that has ended, toward the address lockout.
- **D65-A** (F35): in the app the key the phone keeps is tagged
  "this device's key", from the bridge's key id,
  however the session was made.
- **D66-A**, the owner's idea: key ids shown at any time:
  a server's settings show the kept login's key id whenever one is kept,
  and My profile shows each row's id.
- **D67-A** (F37): My profile reads its list after the sign-in's revoke is done;
  a key the sign-in revoked is not marked, a toast names it instead.
- **D68-A** (F36): a 401 for a request sent before the page's current
  sign-in, with an older token or none, does not open the card:
  a read is sent again, a write fails as not sent;
  a session replaced is signed out, by the page and by the bridge.
- **D69-A** (F38): Remove signs the app's session out first.
- **D70-A** (F39): a session forgotten while its server is down is recorded
  like a key left, and signed out at the next contact.
- **D71-A**, the owner's question whether these checks belong to tests:
  each fix of the next revision comes with a test at the level that holds it
  (F36 a UI test that holds one 401 back until the sign-in has answered;
  D69 and D70 the app's JVM tests; F40's rule a function with its test),
  and a block of its own adds an emulator suite,
  the WebView with the bridge and the fingerprint simulated,
  run locally and in CI on a change under `app/`;
  the phone check keeps what an emulator cannot show.
- **D72-B** (F40): a server asked at each return asks
  each time it is opened from the list, whether the app was hidden or not.
- **D73-A**, the owner's question whether the app's settings and a server's
  could be entries of the page's main menu, shown in the page
  and saved through the bridge:
  two entries in the app alone, "This server's settings…" and
  "App settings…", hand over to the app's own page, as "Select server…" does;
  the server's page neither reads nor saves them,
  since a server that could would turn its own fingerprint off,
  trust another certificate or see the other servers.

## Raised at the end

To be weighed in the plan's next revision:

- the login by fingerprint as what opening a server from the list does,
  where its key is kept;
  a fingerprint that fails leads to the sign-in card, not back to the list;
  and the list's fingerprint button gives way to one that opens the sign-in
  card itself (a key, `***` or a sign-in icon);
- an evaluation of My profile's keys, since D65 to D67 change that section:
  an expiry, a re-key, the last use and its address shown,
  a key renewing itself over the API, a key revoking itself over the API,
  and a key shown again in the page after the password.

## Not checked

A look at a build with metaworx/massimo#397 was offered for after the check:
D60-A on the phone,
and the password manager's autofill with the hidden password form.
