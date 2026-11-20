# NOTE MobileApp v1.2: how block 13 went

Block 13, the phone check's findings, run under the owner's `PLAN++`
on 2026-11-20 as two pull requests:
the web UI's, metaworx/massimo#384 (`claude/mobile-check-ui`),
and the app's, metaworx/massimo#386 (`claude/mobile-check-app`),
which builds on it; the owner merged both the same day.
Where the work went differently from the block's steps:

## The web UI (#384)

- **The header while the page waits.**
  Hiding the header whenever the page is signed out also hid it in the
  moment before the server says who is in, a blank page.
  The header goes only while the sign-in's card is open;
  until the answer, its logo and name show alone, as before.
- **A dead session's reads.**
  Once the page drops its token at a 401 (F7),
  its reads until the sign-in go without one and are refused too.
  The server does not count a request that offers no token against the
  address (`internal/server/auth.go`), so they lock nothing out;
  `test_two_tiles_one_sign_in` expects them.
- **The main ☰'s Select server** is listed in `test_main_menu.py` as the
  app's alone, hidden in a browser.
- **CI**: the Android app job failed once on Maven Central's
  "429 Too Many Requests" before anything was built;
  its rerun passed.

## The app (#386)

Built by a sub-agent from a brief, checked here before the commit:

- **`MainActivity`** was a `FragmentActivity`, on which AppCompat's night mode
  has no effect: it is an `AppCompatActivity`, the mode set in
  `attachBaseContext`.
  A change of Appearance recreates the activity;
  the connect page keeps `#app-settings` in its address so that App settings
  open again.
- **Turn off** revokes the key with the session first, then signs out:
  signing out first would leave nothing to revoke with.
  A 403 or 401 there falls back to the key and the prompt;
  a 404 counts as revoked, and so does a 401 to the key itself.
- **Open at start** takes the LAN address as soon as it answers,
  the remote one once the LAN one failed or has not answered within a second;
  the window shows the app's background meanwhile,
  about four seconds at most where both time out.
- **Sign out of all** probes each server for its node before it sends the
  session, so that no token goes to what now holds an old LAN address.
- **Plain HTTP**: `session` and `signOut` may go over plain HTTP outside the
  private ranges, where the page sends its session anyway;
  the kept key never does.
- **The certificate**: `subject` and `issuer` are RFC 4514 names,
  the settings' line shows the common name.
- **Small choices**: the close buttons are "×", as the toast's;
  the Add card's × is hidden while the list is empty;
  the page still searches at its start, for the saved servers' moved
  addresses, as well as when the Add card opens;
  Sign out of all is not asked about;
  the ☰'s question clears the toast, which at 390 px covered its Yes.
- **Tests**: a test certificate with names, `app/android/app/src/test/resources/san.pem`,
  its key thrown away; the instrumented test expects the bridge's version 3.
- **A conflict that was none.**
  Once #384 was merged, #386 and `main` had two merge bases,
  and GitHub reported a conflict where `git merge-tree` found none;
  a `merge:` of `main`, its tree the branch's own, cleared it.

## Not checked

The app on a phone:
the server opened at start, the session marks, Sign out and Sign out of all,
Turn off with and without a session, the certificate's dialog,
and each Appearance with the WebView's scheme.
That is the owner's check, with the steps of Block 13's Verification.
