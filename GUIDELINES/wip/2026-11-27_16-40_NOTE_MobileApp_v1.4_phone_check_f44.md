# NOTE MobileApp v1.4: phone check F44

The first part of D103-A
(`wip/2026-11-27_16-39_NOTE_MobileApp_v1.4_decisions_after_the_phone_check.md`):
Block 16's step 3 repeated on the owner's phone on 2026-11-27,
at the owner's `EXEC phone F44`, with the server's page in the app read
over its DevTools socket, to see why the toast
`profile.keyRevokedAtSignIn` did not show on 2026-11-25 (F44, in
`wip/2026-11-25_23-06_NOTE_MobileApp_v1.4_phone_check_blocks_15_to_17.md`).

## How it ran

- **The app**: the debug build of that check, not installed again.
- **The server**: rig A alone, its build and store as then,
  against the fake Cube, through `adb reverse`.
- **The recorder**, on the app's WebView, read:
  - its navigations, the console, and the API's statuses,
    with no header and no body;
  - every 100 ms: the address's fragment, the page shown,
    whether the sign-in card was open, the toast,
    and the ids in the bridge's answer to the last `setToken`.
  One listener of its own logged each `viewshown` event;
  it changed nothing of the page or the app.
- **A's audit** was read after each step.

## The steps

1. A password sign-in at A, the login kept: session 75, key 76. Passed.
2. A stopped, then Turn off: the app's toast said the login was off
   and its key still valid at the server; key 76 left.
   But Show in My profile stayed on (F48).
3. A started, then ↻ on the list.
   - The open settings turned Show in My profile on by themselves.
   - Its tap opened A's sign-in card at `#profile?key=76`.
   - Then a password sign-in without keeping the login (session 77).
     The app revoked key 76 0.34 s after the sign-in
     and session 75 0.43 s after it,
     and the bridge answered `{revoked: [76, 75]}`.
   - My profile was shown once `/me` was read,
     and the key left the address.
   - The toast "Key 76 is revoked already: …" showed 0.5 s after
     the sign-in, at the screen's foot over My profile, for its 10 s.
     The owner saw it.
   - My profile listed sessions 69 and 77, and no key 76. Passed.

## Findings

- **F44, not reproduced**: the toast's whole path ran as the code has it.
  Of the two misses on 2026-11-25:
  - **the first** had its steps in the wrong order: no mark, so no toast,
    as designed;
  - **the second is not explained.** The toast shows at the screen's
    foot over My profile, not on the sign-in card, where the owner
    looked for it then. The phone's log of that day has the password
    manager's sheet over the screen's foot for 2 s of its 10.

  Nothing was found to fix.
  D103-A's two cases, an emulator case and a UI case whose bridge
  answers at once, are left to the next revision, so that a regression
  would show.
- **F48, new**: Show in My profile opens the server's page without
  the check a tap on the server's row makes first (`connect.js`).
  - **What happened**: in the seconds after A went down, before the
    list's next check, the settings offered it on.
    The tap showed the WebView's own error page,
    "Web page not available" (`net::ERR_CONNECTION_CLOSED`),
    with the server's address.
    Back led to the list, which then said "not reachable"
    and turned the entry off.
  - **Why**: the app's WebView client has no handler for a page that
    fails to load, so any way into a server's page in those seconds
    ends there.
- **Beside**: session 75, held when A went down and Turn off ran,
  was ended at the next sign-in.
  `unended` covers a server down within one process, as Block 16's
  step 5 had it; F43 and F47 need the process to end (D102).

## The owner's proposals

- **Pull down on the list refreshes it**, as ↻ does; today it does not.
  Taken as written, for the list's block.

## Decisions

- **Open**: D108 on F48, asked at the run's gate.
  Its option A: Show in My profile goes through the check of the
  server's row, and a server's page that fails to load returns to
  the list with the server marked not reachable.

## Not checked

- The other rig servers, and an app built from today's `main`.
