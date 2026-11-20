# NOTE MobileApp v1.2: phone check block 13

The owner's check of Block 13, on 2026-11-20 at the owner's `EXEC phone`,
after pull requests 384 and 386 had merged:
what passed, what the server's audit shows, and what it found.

## How it ran

As the check of Block 8 (`wip/2026-11-20_20-05_NOTE_MobileApp_v1.1_phone_check_block_8.md`):
the owner's phone over WSL's own `adb`,
the `foss` debug build of `main` installed over the one before,
its saved servers kept;
the rig's `serve -tls self-signed` against the fake Cube rebuilt from `main`,
its store and certificate kept, reached through `adb reverse`.
The app's warnings were read from the phone's log, none came;
the server's audit was read from the rig's store.

## What passed

The owner's steps 1 to 10 and 12 to 15:

- the Servers card's ☰, the Add card on demand with ×,
  a server's settings without the keyboard, Open at start,
  the certificate's line and dialog;
- the sign-in card without the header, the version at its foot and in its ☰,
  the fingerprint first in the row, logging in at once,
  Select server in both ☰;
- "this device" and "its key" under My profile, the menus' border and glow;
- the session's badge, Sign out and Sign out of all
  (the audit's `logout` of each session);
- Turn off without a session: the prompt, then the key revoked by itself;
- Open at start after the app was swiped away;
- each Appearance, the list and the server's page following it.

Step 11, a session revoked from another browser, was skipped;
the UI suite covers it (`test_a_dead_session_is_dropped`).
No address was locked out (`auth_failures` empty).

## Findings

- **F25**: while the fingerprint's login runs, its button's spinner pushes the
  icon out of the button.
- **F26**: Forget all stored credentials forgets the kept logins on the phone
  but revokes no key at its server, and asks no fingerprint;
  Turn off in a server's settings does revoke.
  The rig's server kept one such key, valid and never used.
- **F27**: the sign-in card, as the owner would have it:
  centred in the window; the title centred; no "Sign in with" label;
  the ☰ in the row of the ways; "Password" in place of "Name";
  Select server not as a button at the card's foot,
  but in the ☰ alone or as a servers icon beside it.
  A sketch rendered from the page shows both,
  and that German's "API-Schlüssel" wraps in a row with the icon.
- **F28**: Turn off while signed in, step 12, is not in the audit as built:
  the session's sign-out comes fourteen seconds before the key revoked itself,
  the way without a session, and no revoke by the session is recorded.
  Either the session was signed out first, or Turn off did not use it;
  a second try of step 12 tells which.
