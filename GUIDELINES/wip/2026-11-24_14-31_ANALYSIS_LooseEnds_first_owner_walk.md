# Walk-through: the first owner's path on a real open server

Written 2026-11-24 by the session "UI refactor".
An Opus sub-agent walked it with a headless browser;
nothing was fixed and no project file changed.
Checked here: no process and no port left, the working tree clean,
three of the findings read in the code, four screenshots looked at.

## TL;DR

The path works as the plan's NOTEs say,
at 1100 px in English and at 390 px in German.
One real trap, known before and now seen:
approving an integration on an open server turns the page into a sign-in nobody can pass.
Six smaller findings, none of them blocking.

## How it ran

- **The server:**
  main's head, built in the worktree;
  `massimo fakecube` and `massimo serve` on loopback ports of their own,
  with the flags the UI tests use and `MASSIMO_ENV_FILE=/dev/null`.
  A fresh empty store for each scene, no API token, no user:
  the log said `auth=open`.
- **No hardware and no LAN:**
  everything stayed on 127.0.0.1.
- **Five scenes, 77 screenshots:**
  A the whole path (1100 px, English), B two browsers at once,
  C the card from a running page, D the main steps again at 390 px in German,
  E the trap.

## What works

| Step | Seen |
|---|---|
| The card at the start | Title "No users yet"; Name, Password, Again; the focus in Name; the page hidden behind it; nothing shows before the card |
| Skip for now | The page, the warning with "Create the owner…" and Hide; ☰ has "Create the owner…" where Sign out would be; a reload asks again |
| Keep it open… | The warning word for word; the address right; Copy copies it; a wrong address keeps the button off; upper case and a slash at the end are taken; after it a reload asks nothing |
| Users while kept open | The line, "Create the owner…", the switch |
| The three ways to the card | Banner, ☰ and Users each open it; "Back to the page" gives the page back as it was, its scroll too, and the focus to what opened the card |
| Hide | The warning stays away after a reload; ☰ keeps its entry |
| The switch off | No card pops up; the next reload asks again |
| Create the owner | A toast; the same page again as the owner's; no warning; ☰ shows Sign out; Users lists the owner |
| The server afterwards | No token: 401. The owner's PUT of keep-open true: 409 |
| Sign out and in | A wrong password is refused; the right one gets in; a fresh browser gets the sign-in, not the owner's form |
| Two browsers | The second Create gets "The owner exists now: sign in."; the store ends with one user |
| 390 px, German | No text cut off, no overflow, no sideways scroll |

## Findings

1. **The trap: an integration approved on an open server locks the page out.**
   A request sent as an integration would,
   then Approve in System settings → Integrations.
   Within about two seconds the page's next read was refused and the card turned
   to a plain sign-in.
   The store then held two users, both the integration's,
   neither with a password: nobody can get in through the page,
   and nothing warns before Approve.
   `massimo user add … -role owner` on the box rescued it.
2. **The German sign-in shows the server's English refusal:**
   "Nicht angemeldet: wrong user or password".
   The server's text is put into the page's sentence as it comes.
3. **The owner's form keeps an old message.**
   After a mismatch, with Again corrected and a bad name given,
   the name's rule shows and "The passwords differ." stays.
   On a mismatch the focus goes to Create, not to Again.
4. **Two reads without a session when the start card turns to the sign-in.**
   In the second browser the page had not started;
   at the turn it asked for the Cubes and the log's alerts, both refused,
   with console errors.
   Harmless, but the start is meant to wait for the card.
5. **Escape does nothing in a card opened from the running page.**
   Only "Back to the page" leaves it.
   By keyboard it feels like a trap.
6. **No minimum length for a password.**
   "a" was taken for the owner.
   Neither the form nor the server checks a length; the server cannot,
   since it never sees the password.
7. **Small things.**
   - German: "Lege hier, aus dem lokalen Netz, den Eigentümer an,
     oder auf der Box mit massimo user add." reads awkwardly.
   - The switch keeps its label "This server is kept open without an owner"
     while it is off.
   - After Skip and after Keep it open the focus is on the page's body
     (FirstOwner D24-A chose this).

## Not done

- The copy control and the card in the Android app; no touch run.
- The race of two creations at the same instant (the server's 409);
  the second browser took the 401 path.

## Screenshots

Seventy-seven were taken; they are not tracked.
