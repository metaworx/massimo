# NOTE MobileApp v1.6: how Block 19 went

The owner's `PLAN++` of 2026-11-28 on v1.6 built Block 19 on
`claude/mobile-list-menu`, from `main` at
`chore(aps): +NOTE MobileApp v1.6, how Block 21 went`, in the three commits
the plan names, and a fourth from the re-review of #497:

1. `feat(app): The list's ☰ in its header, with Exit, Store login and pull down`
2. `feat(app): A server's settings in three sections, its certificate in place`
3. `fix(app): A page that fails to load returns to the list`
4. `fix(app): A plain-HTTP server's Certificate section says so while it is probed`

The coordinating session pushed them, opened #497 and merged it on
2026-11-28 once its CI was green. The armv6 job's first run failed on
`TestRepairPlaceHoldsTheRoomsState` (the re-send not yet sent under QEMU),
a test of `internal/server` this block does not touch; its rerun passed.

## Where it went differently from the plan

- **App settings without "…"** takes the card's own text, `connect.app`;
  `connect.menuApp` is gone from the six files. The ☰ keeps its name,
  `connect.menu` ("Servers menu"), and is hidden in a browser, as the list is.
- **The version row** shows the build's version whole, the bridge's
  `about()` giving `versionName`; it is in the markup, filled where the
  bridge has `about`. Its copy closes ☰ a task after the copy's own
  listener, the focus back on ☰'s summary.
- **Exit's icon** is a power sign of Massimo's own (`icon-power`); the copy
  icon and the disk's symbols are copied into `connect.html`, which has a
  sprite of its own.
- **Store login's words**: its failures and a prompt not passed take the
  sign-in's, `login.keepError` and `login.keepFailed`; off while the server
  does not answer it takes Show in My profile's title, `connect.keyLeftShowOff`.
  The new keys are its name, its title (`connect.storeLoginTitle`, what it
  does), its toast and `connect.storeLoginPlain`, the title at an address of
  plain HTTP outside the home network, where it is off.
- **The kept login's line** is drawn with the session's lines now, again
  where it changed, so that it shows once a login is stored; `dropLoginLine`
  is gone.
- **Back's entry**: the page pushes its first history entry half a second
  after its load (`BACK_READY_MS`), since `onPageFinished` forgets the pages
  before the list and could otherwise take the base entry with it (a panel
  opened at the start by `#settings=<id>` or `#app-settings`); a take-back
  that meets no entry ends after a second (`BACK_MS`). The entry carries
  `{massimo: "open"}` as its state, which the UI tests wait for.
- **Back waits for a touch**: the page pushes its entry only once the user
  has touched it (`navigator.userActivation.hasBeenActive`), else at the
  first touch or key. The emulator found the WebView's `canGoBack()` false
  with `history.length` 2 where the entry was pushed after a script's click:
  Chromium's history intervention has Back pass over an entry a page adds
  before any user activation. So a panel the app's handover opens at the
  start has its entry at the first touch, and Back before that leaves the
  app; the emulator's Back case opens ☰ by a tap on the screen.
- **`onPageFinished`** forgets the pages before the list after the list's
  own `onPageStarted` alone, not at the end of a same-document entry of the
  page's, which starts no load.
- **Exit answers the page no more**: from Exit on, the bridge drops the
  page's messages, and a check the page asked for just before posts no
  code. The emulator's third run of `ListTest` had the session Exit dropped
  ended at once, 401 before the relaunch: the list's check, asked before
  Exit, had posted the code Exit had just left.
- **Back's question** is in place of Connection's Cancel and Save,
  Discard (`connect.discard`) or Stay (`leave.stay`, the web UI's), its
  text `connect.discardQuestion`; `confirmInPlace` tells an `onNo` where
  given, for Stay. While it shows there is no entry, as the plan has it:
  a second Back then leaves the app, the question kept for the return.
- **Cancel** no longer closes the settings: it gives the fields what is
  kept again, the settings closed by Back or ✕ (D94-A). Save closes them as
  before.
- **Add server and App settings** do not close a server's settings, as
  today: the plan's "close it without asking, as today" holds for another
  server's cog and a tap, while today these two open beside the settings.
  Back closes the panel opened last (`conn.panels`).
- **Connection's tab panel** is a `div` holding the form: axe refuses
  `role="tabpanel"` on a `form`.
- **The tabs' marks** sit at their tab's top right corner, taking no room
  from the names, and the tabs have 8 px of padding and 4 px between them:
  with the marks inline and the `.page-link`'s 10 px, German, Dutch and
  Italian wrapped at 390 px (the panel's content is 298 px wide there).
- **Login with nothing in it** says so, `connect.loginNone`, so that no tab
  panel is empty.
- **`signEl(kind, state, {cls, key, label})`**: the class, the mark's key
  and the label in one options object; with a label it is an image named
  for a screen reader.
- **The Certificate section** keeps what the last probe found while the
  next one runs, as the session's line does.
- **A Remove the server did not answer** leaves no server to probe again;
  only a Remove that failed probes it, the server staying.
- **`onReceivedError`** passes over a load while the certificate's dialog
  shows (`onReceivedSslError` loads the list), while the list is shown, and
  `ERR_ABORTED`, a load given up for another.
- **The emulator suite** stands a page that fails to load in with the
  server's address closed after the list's probe found it, the probe's
  finding kept, since nothing stops the host's `serve` (as Block 21's
  `CodesTest`); the pull down is a UiAutomator swipe from a quarter to three
  quarters of the screen.
- **The fake bridge** has `contract` (6 leaves version 7's calls out),
  `appVersion` and `storeLogin`, `keep`'s answers in turn.
- `make ui-check` takes the shared install in the main checkout's
  `GUIDELINES/temp/ui-check/` (the project's own `install.sh`), whose
  directory it touched; nothing in it was written anew.

## The review of the branch

A review of the branch before its push found these; each fix is folded into
the commit it belongs to:

- **Back after Remove server** (the first commit): Remove left "settings"
  among the panels open, so that Back pushed the entry again each time and
  never left the app. Every way the settings close now goes through one
  helper (`settingsClosed`), and Back drops a stale one.
- **The fingerprint sheet** (the first commit): Back closed the panel
  under it, and Trust still added the server. Back closes the sheet first,
  as its Cancel does, the page holding its entry while the sheet shows.
- **Exit and a check already running** (the first commit): Exit's flag is
  given to `Sessions` and `Codes` (`stopped`), so that a check whose answer
  comes after Exit writes no check nor record as held, revokes nothing, and
  posts no code; two JVM tests, seen failing with the check undone.
- **Store login without a strong biometric** (the first commit): `about()`
  also says `can`, asked again as a server's settings open; Store login is
  off then, its title `connect.storeLoginNoBiometric` (the words of the
  app's `err_no_biometric`), contract version 7 still.
- **A changed certificate** (the second commit): the Certificate section
  said the server does not answer; it shows the one trusted and the new one
  under `connect.fpChangedTitle` now, with `connect.fpOld` and
  `connect.fpNew`.
- **The first probe** (the second commit): the section said "does not
  answer" while it ran; it says that the certificate shows once the server
  answers.
- **What is kept, changed meanwhile** (the second commit): a name
  discovery heard or a LAN address found again, saved while the settings
  are open, marked Connection and would have been written back by Save.
  The fields still holding what the settings opened with take what is kept
  now (`takeKept`), a field the user changed keeping the change, and the
  head's name follows.
- **Stay** (the second commit) looks at the fields again, so that one put
  back meanwhile leaves Save and Cancel off.
- **A newer probe** (the third commit): Show in My profile and the
  handover's toast wait for the server's latest probe (`probed`), so that a
  network event meanwhile neither flashes the toast nor keeps My profile
  shut.
- **A server without a name** (the third commit) is named by its address
  in those toasts and in a tap's (`shownName`).
- **`onReceivedError`** (the third commit) no longer passes over a failure
  while the list is the page shown: the WebView may tell `onPageStarted`
  after it; the opened server's origin and `ERR_ABORTED` decide.
- **The tests**: `test_connect_sections` checks Back and ✕ from both
  sections; new cases for each point above in the UI suite; the emulator's
  `ListTest` has a changed certificate's case, the change stood in for by
  another certificate pinned: its dialog shows, and the list then says
  nothing of a server that does not answer. A load given up for another
  (`ERR_ABORTED`, Back while a server's page loads) has no reliable case
  there: the rig's server answers before a Back can come between.
- **Back's first entry** still waits half a second after the page's load
  (`BACK_READY_MS`): an event of the adapter after `clearHistory` would be
  one more part of the contract, and with the first touch it waits for now,
  the page pushes nothing before the user has touched it, by which time the
  list's load has ended.

## The re-review of #497

After the push, one commit on top, `fix(app): A plain-HTTP server's
Certificate section says so while it is probed`:

- **The nameless server's case** of `test_connect_unreachable_from_address`
  passed with `probed()` made a no-op: the toast named the server whether
  it came at the first probe's end or after the newer one. It asserts the
  probe's reason in the toast now, which only a finished probe carries; seen
  failing with `probed()` made a no-op.
- **Plain HTTP while a probe runs**: the Certificate section of a server of
  plain HTTP alone, nothing pinned, said that the certificate shows once the
  server answers during its first probe; it keeps the line of plain HTTP
  now. `test_connect_certificate` checks it, seen failing with the fix undone.
- The sections' bullet is accurate as it stands; `changelog.sh check
  --staged` accepts the commit without a change to it.

## Findings

- The connect page's sprite has no `mark-warn` nor `mark-x`, so the marks
  `badgeEl` leads a warning's or a failure's badge with ("not reachable",
  "certificate changed", "not encrypted") draw nothing there; not changed.
- "Forget all stored credentials" wraps onto two lines in ☰, whose list is
  320 px wide at most, at every width, as it did in the card's head.
- The toast that covered the panel's foot at 390 px was looked at again:
  with the sections the foot is Connection's Remove alone, and the toast is
  as it was; the UI tests still close it there.

- **The emulator's runs**: the first whole run had 26 of 27 green, the
  Back case failing as above; four runs of `ListTest` alone followed: the
  first still failing at Back (the `onPageFinished` change alone), the
  second with a log of `canGoBack()` (false), the third with the tap (Back
  green, Exit's race found), the fourth all green but the pull down, whose
  process the system killed ("depends on provider … FontsProvider in dying
  proc com.google.android.gms.persistent"), the emulator's, after three
  green runs of it. The fixes are folded into the first commit.

## Tests

On the committed tree, after the review's fixes were folded in:

- `make ui`, `go test ./...` (39 packages), `gofmt -l .` empty, `go vet` for
  linux, windows and darwin, `make cross-check`, `make ui-check`: all green.
  No Go file changed, so no race run.
- `test_connect.py` and `test_copy.py` in the four cases: 160 passed
  (156 before the review; 144 after the first commit, 148 after the second
  on the first build).
- The whole UI suite once, 1100-en: 890 passed.
- The JVM tests: 173, none failing (two new: a check and a post once Exit
  stopped them, the check's seen failing with its fix undone);
  `lintFossDebug` clean; `assembleFossDebugAndroidTest` built.
- The emulator suite, whole: 28 tests, 27 green; the one red,
  `ConnectPageTest.theBridgeAnswersTheConnectPage`, had no answer to its
  script within its 5 s while the app's main thread skipped 238 frames at
  the WebView's first start ("Skipped 238 frames", a Davey of 4 s); run
  again, its class passed. Before the review the whole suite had 26 of 27
  green, `CodesTest.turnOffWhileTheServerIsDownLeavesCodesTheNextContactTakes`
  red, its process killed by the system ("depends on provider …
  FontsProvider in dying proc com.google.android.gms.persistent"), as the
  pull down's once before; run again alone, it passed. The app holds
  Google's font provider, most likely through the AndroidX emoji support
  AppCompat brings (not read in the code): a death of Play services'
  process takes the app's with it, in whichever case runs then.
- `ListTest` alone after the review: 6 of 6.
- `GUIDELINES/shared/tools/check`: all well.

## Left for the owner's phone check

Block 19's Verification, each step after what it needs:

1. ☰ at the logo's right: the version row first, a tap copying the version
   and closing ☰; the entries in order, App settings without "…";
2. a password sign-in at A, its login kept and asked at each start, back to
   the list, Exit: the app gone from the recent apps; started again: the
   list; A's audit ending the session Exit dropped at that start's contact,
   by its code;
3. the phone turned to landscape: the cards wider than before; the servers
   sorted by name;
4. ☰ opened, Back: ☰ closed; A's settings opened, Back: closed; A's name
   changed, Back: the question, Stay, Back again, Discard; Back from the
   bare list: the app leaves;
5. A's settings: Connection first; Save and Cancel off until a change, and
   off again when it is undone; a name changed, Login opened and Connection
   again: the name kept, Connection's tab marked; Login shown, ✕:
   Connection shown with the question; Remove server under Connection's
   rule;
6. A's Login section: the session with Sign out, the kept login with its
   key's id and Turn off; A's Certificate section: the facts the dialog
   showed, in place; A stopped: the pinned fingerprint;
7. a password sign-in at B with "Enable login by fingerprint" off, back to
   the list, Store login in B's Login section: one prompt, the login kept;
   B's page: the new key tagged "this device's key" under My profile;
8. the list pulled down: every server checked again, as ↻ does;
9. A's `serve` stopped: within seconds, Show in My profile: the toast "not
   reachable", no WebView error page; a tap on A's row: back at the list,
   A not reachable, the reason said.

Also to look at on the phone: the tabs' marks at their corners, the toast
over the settings' foot at the phone's width, and a second Back while the
question of settings with changes shows, which leaves the app, as the plan
has it: on Android 8 to 11, where leaving ends the activity, the draft is
then lost; on 12 and later the task goes to the back and keeps it.
