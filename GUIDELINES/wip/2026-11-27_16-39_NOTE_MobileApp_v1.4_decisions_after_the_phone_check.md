# NOTE MobileApp v1.4: decisions after the phone check

The owner's answer at the gate of the phone check of Blocks 15 to 17
(`wip/2026-11-25_23-06_NOTE_MobileApp_v1.4_phone_check_blocks_15_to_17.md`),
on 2026-11-27: "all open decisions as recommended", and `EXEC phone F44`
(`wip/2026-11-27_16-40_NOTE_MobileApp_v1.4_phone_check_f44.md`).
Every decision open then is taken in its option A, recorded here
for the plan's next revision (D99-A), which folds this NOTE in.

## The list (the connect page)

- **D90-A**: one ☰, in the header at the logo's right,
  where a server's page has its ☰:
  the version row, a rule, Add server…, Sign out of all,
  Forget all stored credentials, a rule, App settings, Exit.
  The Servers card keeps ↻ alone.
- **D91-A**: Exit ends the app as a swipe from the recent apps does:
  the sessions held in memory dropped, nothing sent to a server,
  the app out of the recent apps (`finishAndRemoveTask`).
  With D102-A, a session it dropped is ended at the next sign-in
  as that user.
  Android only: the iOS shell has none.
- **D92-A**: the cards take the page's full width,
  up to the limit of a server's page (1100 px).
- **D93-A**: Back in a panel with changes asks once:
  discard the changes, or stay.
- **D94-A**: in a server's settings, Save and Cancel are off while
  the form holds what is stored, as the server's forms are;
  the panel is closed by Back or by a labelled ✕ in its head.
- **D95-A**: the servers sorted by the name shown
  (the server's name, else its address) in the phone's language's
  collation: case and accents ignored, numbers by their value.

## The menus and the cards

- **D96-A**: no "…" on an entry that only opens a page or a panel,
  in both menus (Select server, This server's settings, App settings);
  Add server… keeps it.
- **D97-A**: every copy, on a phone and in a browser, shows the page's
  toast "Copied" in place of the word at the element;
  in the app on Android 13 and later the system's preview shows beside it.
- **D98-A**: the new password's repetition is outlined in red
  while it is not empty and differs from the first,
  with the words "The two passwords differ" under it.
- **D104-A** (F45): a request's failure under My profile also shows
  in the page's toast; the message line keeps it after the toast goes.

## The sessions

- **D100-A** (F41): the app drops a session without a logout
  where the server has ended it itself:
  a new session made from the key, which ends the key's earlier one,
  and a key revoked with its sessions.
- **D102-A** (F43, F47): each server keeps the user and the id
  of the session held there, once its id is learnt; no secret.
  It is cleared where the session ends.
  At the app's start, one whose session is no longer held moves into
  `unended`, and the next sign-in as that user ends it, as today.
  Also:
  - a fingerprint tap learns its session's id at once;
  - the sessions held live with the process, not the activity,
    so that a change of dark mode or of the phone's language keeps them;
  - `Sessions.look` logs what it swallows.
- **D103-A** (F44): the toast seen on the phone first, then fixed
  for what that shows, with an emulator case and a UI case
  whose bridge answers at once.
  The first part ran under `EXEC phone F44`; its NOTE has the outcome.
- **D105-A** (F42): while a server does not answer, the list's
  "signed in as …" turns grey, and the settings' session line says
  that Sign out ends the session there once the server answers again.
- **D106-A** (F46): only the password routes answer a cancelled request
  as "too many password checks at once";
  elsewhere the audit says the client went away before the answer,
  the result `failed` as now.
  A server fix, in its own commit.
- **D107-A**: the trailing space autofill leaves after the user name
  stays: every sign-in path trims it, the server too.

## Where the work goes

- **D99-A**: AP MobileApp v1.5, in three blocks,
  the findings and decisions keeping their numbers:
  - **Block 19, the list**: the ☰, the version row, Exit, the width,
    Back, a server's settings, the order, Store login;
    `connect.html`, `connect.js`, `MainActivity.kt`.
  - **Block 20, the menu and the cards**: the server's ☰, the version row,
    the card's line, the command, ⓘ's copy icons, the copy's toast,
    the password pair, Sign out to the list, D104;
    shared parts, so the server's page in a browser changes too.
  - **Block 21, the sessions**: D102, D100, D103 and D105 in the app,
    D106 in the server.

## Taken as written

The owner's proposals of the phone check that asked nothing,
as the plan's next revision is to build them:

- **The version row**, first in both menus: the build's version
  on the list, the server's on its page; a tap on the row copies it,
  shows the copy's toast and closes the menu; the row keeps its copy icon.
- **Back** closes an open menu first, then an open panel
  (a server's settings, Add server, App settings), back to the list;
  only then does it leave the app.
- **A server's settings**: the certificate and key part below
  Cancel and Save, beside Remove server.
- **The server's ☰ in the app**: the version row, the pages,
  Personal settings, a rule, App settings, This server's settings,
  Select server, a rule, Sign out;
  in a browser, without the three app entries, the two rules draw as one.
- **The sign-in and first owner's cards**: the server's line
  (its name, its address, ⓘ) above the card's title,
  the address and ⓘ held together on one line.
- **The first owner's card**: `massimo user add` in monospace,
  on one line, copied by a tap or by its copy icon.
- **ⓘ's instance id and fingerprint**: each with its copy icon.
- **The new password and its repetition**: one eye, on the first field,
  showing and hiding both.
- **Sign out in a server's page**, in the app,
  ends the session and shows the app's list; in a browser as now.
- **Store login** in a server's settings, where the app holds a session
  there and keeps no login: the fingerprint asked once,
  then the key made by that session, as a password sign-in that keeps
  the login makes it; off while the server does not answer,
  its title saying why.
- **A manual order of the list**: a later plan, for the roadmap's owner.
