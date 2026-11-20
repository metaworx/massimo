# NOTE MobileApp v1.1: phone check block 8

The owner's check of Block 8, the login by fingerprint,
on 2026-11-20 at the owner's `EXEC phone`, after pull requests 365 and 368
had merged: how it ran, what passed, what it found,
and the decisions taken on the way.
Its findings are revision v1.2's Block 13.

## How it ran

- **The phone**: the owner's Samsung Galaxy A56 (`SM-A566B`), Android 17,
  over wireless debugging from WSL's own `adb`.
- **The app**: the `foss` debug build of `main` with Blocks 8 and 12,
  installed over the debug build of the phone test of 2026-11-17,
  its saved servers kept.
- **The server**: a `serve -tls self-signed` against the fake Cube in WSL,
  on `127.0.0.1:9443`, reached by the phone through `adb reverse`;
  the rig in the project's `GUIDELINES/temp/mobileapp-rig/`, which a restart
  of WSL leaves in place, with a script that clears the address lockout at
  each start.

What got in the way, none of it the app's:

- WSL rebooted once and took the first rig, which sat in the session's
  scratch directory: the server's store and certificate were made anew.
- The wireless debugging connection dropped several times, and the forwarded
  port with it; a watcher reconnected it every ten seconds.
- A Windows `adb.exe`, started from WSL to look, connected to the phone as a
  second host and put WSL's connection offline until the owner turned
  wireless debugging on again.

## What passed

- The first owner, made from the phone.
- Enabling the login at the sign-in; cancelling the prompt, then Try again.
- The key "SM-A566B, login by fingerprint" under My profile, the session
  made from it naming it.
- Login by fingerprint after a logout.
- Each of the three choices: at each return, the cover and its unlock;
  at start, the fingerprint after a restart; after a logout, signed in
  across a restart.
- The screen lock's PIN in place of the fingerprint.
- "Select server" from the sign-in form.
- Revoking the key under My profile ends its session at once.
- A certificate changed while signed in: the dialog, the list, Trust,
  signed in again.

## Findings

F1 to F24, each with its fix, are in the plan's Analysis,
"The phone check's findings (Block 13)". Two were not reproduced:

- **F1**, a server whose certificate changed that did not open after Trust,
  fell into the hour the rig was down (`serve` not running after WSL's
  reboot, the forwarded port lost); reproduced on purpose with a new
  certificate while signed in, it went as it should.
- **F18**, "after a logout" asking for the fingerprint after a restart,
  did not come back.

The two lockouts of the check were the page's own (F7):
after the owner revoked the page's session, and after revoking its key,
the page went on sending the dead session with its polls and stream
tickets, ten failed checks within a minute, and every connection through
`adb reverse` comes from `127.0.0.1`.

## Decisions

Numbered D38 to D51 at the check, before the plan's D38 and D39 were seen
to be Block 12's; the plan carries them as D40 to D53, each two higher:

| At the check | In the plan | Decided |
|---|---|---|
| D38 | D40 | the fingerprint icon v2d: v2b's outer line, broken twice, and v2c's middle line |
| D39-A | D41-A | "this device" on the session or key in use, "its key" on the key it was made from |
| D40-A | D42-A | "Enable login by fingerprint" on the first-owner form too |
| D41-A | D43-A | "Open at start" in a server's settings, one server at most, on for the first added |
| D42-A | D44-A | no header while signed out, in browsers and the app; the version at the card's foot and first in its ☰ |
| D43-A | D45-A | "Sign in with: Name \| API key", "Select server" apart |
| D44-A | D46-A | the server list marks where the app holds a session, checked at each probe, with Sign out per server and of all |
| D45-A | D47-A | the fingerprint per server; an app-wide lock may come later |
| D46-A | D48-A | a stronger border and shadow on every menu |
| D47 | D49 | the owner's variant of A: the sign-in row starts with a fingerprint button where a login is kept, which logs in at once; Name stays the pane chosen |
| D48-A | D50-A | a faint light glow around every menu in dark mode |
| D49-B | D51-B | a server's certificate as a line in its settings and a dialog with its subject, issuer, names, validity, fingerprint and when it was trusted |
| D50-A | D52-A | the findings as revision v1.2's Block 13 |
| D51-A | D53-A | an app-wide Appearance in an App settings card |

The owner's points beside them, which the plan's Block 13 carries:
the Servers card's ☰ (Add server…, Sign out of all, Forget all stored
credentials), the Add a server card only on demand with a ✕,
"Select server" in the card's ☰ and in the main ☰ before Sign out,
and a server's settings that do not open the keyboard.
