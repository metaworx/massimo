# NOTE MobileApp v1.1: block 8 decisions

The owner's answers of 2026-11-18 to Block 8's gate,
given with `EXEC++ as recommended`:
D32 and D33, which the plan left open for this gate,
and D34 to D37, which the gate found.
Where this NOTE and the plan's Analysis or Block 8 differ, this NOTE holds.

## Decisions

- **D32-B, the stored servers in the Cube choice**:
  one entry, "Select server", after the Cubes and the search,
  which shows the connect page.
  A server's page learns nothing of the other servers:
  the bridge's calls `serverNames` and `switchTo` are not made,
  and `connect()` is the one way back.
- **D33-A, the picture of a hidden app**:
  while the page of a server whose `ask` is `return` is shown,
  and only then, Recents keeps no picture of it:
  `setRecentsScreenshotEnabled(false)` on Android 13 and later
  (API 33), which leaves screenshots allowed;
  `FLAG_SECURE` on Android 8 to 12, which forbids screenshots and screen
  recording of that page there too.
  Either is set while such a page is shown, not as the app leaves it,
  since the system takes the picture on the way out.
  The connect page and the pages of servers asked at `start` or `logout`
  keep their picture.
- **D34-B, a server's two addresses**:
  one kept credential per server, not per address,
  bound to the server's id, its `node`, its pinned fingerprint and the user's name,
  and used at whichever of its two addresses the page opened,
  that address having answered with the server's `node`.
  A server's `login` is the one user's name, or `""`.
  The plan's "Not in this plan" line on one credential for both addresses
  moves into Block 8.
- **D35-A, the PIN and a new fingerprint**:
  D25-A stands. androidx's and the Keystore's references (read 2026-11-18):
  a key whose authentication admits the screen lock
  (`AUTH_DEVICE_CREDENTIAL`, from Android 11) is not invalidated
  when a fingerprint is enrolled;
  one bound to the biometric alone (Android 8 to 10) is.
  Turning the screen lock off invalidates the key on every version.
  The Analysis' "A fingerprint enrolled later invalidates the key"
  holds for Android 8 to 10 alone,
  and the Verification's "a newly enrolled fingerprint ending it" becomes:
  on Android 11 and later the kept key survives a new fingerprint,
  and turning the screen lock off ends it.
  The user manual says so (D37-A).
- **D36-A, the key over plain HTTP** (follows from D34-B):
  the kept key is never sent over plain HTTP to an address outside the
  private ranges (those of the connect page's warning, `docs/app.md`);
  there the sign-in form offers the password alone and says why.
  A key enabled at the LAN address over plain HTTP is still sent there.
- **D37-A, the user manual**:
  Block 8 starts `docs/user-manual.md` with its chapter on the login by
  fingerprint in the app:
  what is kept, the three choices of when it is asked,
  the PIN and a new fingerprint (D35-A), plain HTTP (D36-A),
  and revoking the phone's key under My profile;
  listed in `docs/fragments/_documents.md`, so that the site publishes it.
  Showing the manual in the app, from the main menu, the sign-in form and the
  connect page, is a plan of its own:
  rendering Markdown takes a build step the web UI does not have,
  and the manual's languages want deciding first.
  Its line on the roadmap is the coordinating session's.

## What changes in Block 8

- Step 3, the contract: no `serverNames` or `switchTo`;
  a server's `login` is one name (D34-B).
- Step 5, the Cube choice: the one entry "Select server" (D32-B).
- Step 7, the adapter: one `LoginKey` per server;
  the Recents picture for `return` (D33-A); the plain-HTTP rule (D36-A).
- Step 9, the documents: `docs/user-manual.md` and its line in
  `docs/fragments/_documents.md` (D37-A).
- The Verification: the new fingerprint as D35-A says.
