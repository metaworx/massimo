# NOTE MobileApp v1.4: phone check blocks 15 to 17

The owner's check of Blocks 15, 16 and 17 on the phone, on 2026-11-25,
at the owner's `EXEC phone`, after metaworx/massimo#418 and #420 had merged:
how it ran, the steps of the three blocks' Verification,
what it found, and the decisions taken and still open.

## How it ran

As the check of Block 14
(`wip/2026-11-21_21-24_NOTE_MobileApp_v1.3_phone_check_block_14.md`):
the owner's phone over WSL's own `adb`,
the `foss` debug build of `main` at
"chore(aps): +NOTE AutoUpdate v1.1, how block 7 went",
installed over the one before, the app's data kept.
The rig's three servers, built from that `main`, all reached through
`adb reverse` and run with `-tls self-signed` against the fake Cube:

- **A**, its store kept from the earlier checks;
- **B**, its store kept, with a login kept on the phone from before the update;
- **C**, a new one with an empty store, which showed the first owner's form.

The steps went in four batches, the owner reporting each.
Each server's audit was read from its store after each batch,
and the app's warnings from the phone's log; none came.
A was stopped, started and once frozen (`SIGSTOP`, then `SIGCONT`)
between the owner's steps.
D101-A put the owner's own server, which keeps a login on the phone,
in Forget all's reach; its audit was not read.
A watcher kept the reverse ports through the wireless debugging link.

## The steps

### Block 15

1. A tap on a server asked when the app starts, no session held:
   the prompt with the PIN as its other way; its cancel opened the card,
   with its fingerprint button. Passed.
2. The sign-in icon: the card at once, no prompt. Passed.
3. A server asked at each return, opened from the list twice:
   asked both times; the cover after Home until the fingerprint. Passed.
4. The card's line and its ⓘ, on C's first owner's form:
   the server's name and full address, the instance id and the fingerprint
   as the rig's certificate has them. Passed.
5. ☰'s two entries, This server's settings… and App settings…,
   each landing on the connect page with its panel open. Passed.
6. Show in My profile off, its title saying why, with A stopped. Passed.
7. Remove server on its own line, its text and its question. Passed.
8. The look: switching between Password and API key,
   no field focused and the card standing still;
   the password manager's autofill through API key and back,
   the name and the password filled and the sign-in passing. Passed.
9. The password's eye on the card and on the first owner's form. Passed.

### Block 16

1. A password sign-in where a key is left: one card, one session.
   Passed twice (A's sessions 65 and 68).
2. The login turned on at a password sign-in, on B:
   the new key tagged "this device's key" under My profile,
   every row with its id, the id the server's settings name. Passed.
3. Show in My profile after a sign-in that revoked that key:
   no mark, as it should be, but **no toast** (F44).
   The first try ran its steps in the wrong order, the sign-in first,
   the session's mistake; the second, in the right order, showed none either.
4. Remove of a server without a kept login: the app's logout at A
   before the server went (session 74). Passed.
5. Turn off with A stopped, then started and signed in again:
   - first, with a session held for 16 s:
     the key left was revoked within a second (key 63),
     but the session was never ended (session 62, F43);
   - again, with a session held for minutes:
     the settings named both the key and the session left
     ("A session of this phone is still valid here: session 66 …,
     ended at the next sign-in as that user"),
     and the next sign-in revoked the key 0.28 s after it
     and the session 0.36 s after it. Passed.
6. A Revoke while A hung (frozen):
   it ended after about 20 s, the button no longer busy,
   the message saying so; but out of view (F45). Passed.

### Block 17

1. Forget all, one server holding a session (C) and the others none:
   one prompt; A's key revoked by itself (`DELETE /api/v1/me/token`),
   C's key revoked by C's session, in one second. Passed.
2. A login kept after the update, turned off and on, one prompt each;
   B's login kept before the update opened with its own prompt. Passed.
3. On the rig, a session logged out and sent again 14 times within a minute:
   `401` each time, no `429`, no failure counted, the key still valid. Passed.

Beside the steps:
Turn off without a session revoked the key by itself (A's key 73),
as the emulator suite's `SessionsTest` has it.

## Findings

- **F41**: the app sends `POST /api/v1/logout` with a session the server
  has already ended, and the audit holds a `logout` refused with `401`:
  after a second key login, which ends the key's earlier session,
  and after a Turn off or Forget all whose revoke ended the key's sessions
  (D80-B). Seen on B three times and on C once.
  Not counted (D64-A), but a refused row at each return by fingerprint.
- **F42**: with A down while the app held a session there,
  the list showed "not reachable" and "signed in as …" side by side,
  and the settings offered Sign out.
  Sign out works without the server and leaves the session to `unended`,
  but nothing says so.
- **F43**: a session held when the server went down, 16 s after its sign-in,
  was not put into `unended` and stayed valid at the server;
  held for minutes, it was (Block 16's step 5).
- **F44**: the toast `profile.keyRevokedAtSignIn` does not show
  (Block 16's step 3).
- **F45**: a request's failure under My profile (the Revoke after the hang)
  is said at the card's bottom, out of view where the card is long.
- **F46**, the server: A, thawed, read the Revoke the phone had given up on;
  `userFail` mapped the cancelled request to the password check's answer,
  and the audit says the revoke `failed` with
  "too many password checks at once", a route that checks no password.
  The key stayed valid, as it should.
- **F47**: a password session held when the app was swiped away from
  the recent apps stayed valid at the server,
  not put into `unended`, and three later sign-ins as that user
  did not revoke it.
  `unended` covers Turn off, Forget all, Sign out, Sign out of all
  and a server down (D70-A), not the app's process ending.

## The owner's proposals

Raised during the check, beyond its steps.

- **The list** (the connect page): its ☰ in the header beside the logo,
  where a server's page has its ☰, with the app's version row and an Exit;
  or two menus, one for the app and one for the servers (D90);
  the cards the page's full width (D92);
  Back closing an open panel (a server's settings, Add server, App settings);
  in a server's settings the certificate and key part below Cancel and Save,
  and Cancel and Save on only with changes, as the server's forms are (D94);
  the servers sorted by name (D95), a manual order later.
- **A server's ☰**: a rule after Personal settings;
  the app's entries in the order App settings, This server's settings,
  Select server, then a rule; no "…" on them (D96).
- **The version row**, in both menus:
  a tap copies the version, shows the copy's toast and closes the menu.
- **The cards**: the server's line above the card's title,
  ⓘ held to the address;
  on the first owner's card `massimo user add` in monospace,
  on one line and copyable;
  ⓘ's instance id and fingerprint copyable with an icon;
  one toast for every copy (D97).
- **The new password and its repetition**: one eye for both;
  the repetition outlined in red while it differs (D98).
- **The user name trimmed**: the password manager's autofill adds a space after it.
- **Sign out in a server's page** goes to the app's list,
  not to that server's sign-in card.
- **Store login** in a server's settings, where a session is held and no key:
  the fingerprint asked, then a key made there,
  for a password sign-in that did not keep the login.

## Decisions

- **D101-A**, Forget all with the owner's own server in the list:
  the owner's word, "D101-A, go on".
  Its key there was revoked and the phone signed out;
  the owner turns its login on again.
- **Open**, D90 to D100 on the proposals, and the decisions
  on F42 to F47 and the last three proposals: asked at the check's gate.

## Not checked

- The owner's server's audit, on the Pi.
- The last three proposals, the findings' causes and their fixes:
  for the plan's next revision.
