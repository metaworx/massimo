# AP MobileApp v1.6: the Android and iOS apps

## TL;DR

Roadmap items 16 and 17: the web UI in a thin shell for Android and iOS,
a client or stand-alone.
On `main`: Blocks 2 to 4, 8 and 12 to 18;
v1.6 adds a revoke code and a server's settings in sections.

- list → ☰ in its header, version row, Exit; full width, by name
- Back → menu, panel, out; pull down → ↻
- server's settings → Connection, Login, Certificate; Save in Connection
- failed load → the list
- server's ☰ → app entries last, no "…"; a copy → the toast
- cards → server above the title; command, ⓘ copyable
- new password → one eye, repetition red
- session held → recorded, ended after the process
- token forgotten → its revoke code at the next contact
- removed server's codes → 90 days
- no logout the server ended; cancelled revoke → no "password checks"
- sessions bound to the phone → a plan of its own
- open → D115, a copy's word: toast or bubble
- order → 21, 19, 20; first release the client with the login

### Blocks

1. Core: `app/core` bound by gomobile, the Makefile targets, a test
2. Connect: the page `ui/connect.html` and the bridge contract (done)
3. Shell: the Android app in client mode, Kotlin, `foss` and `play` (done)
4. Release: the Android CI job, reproducibility, signing, the version file,
   the APK and AAB on the release (done)
5. F-Droid: the recipe, the submission, our signature;
   the owner's developer verification
6. Play: the listing, Data safety, the signing key, the closed test
7. Local: the stand-alone mode on Android, the foreground service,
   the phone's own server kept open
8. Login: the login by fingerprint: the server's route and a session's key,
   the bridge's calls, the Keystore key bound to the fingerprint,
   the page's buttons, "Select server" in the Cube choice (done)
9. iOS: the Swift shell, the same bridge, the App Store (item 17)
10. Documents: `docs/app.md`, `app/README.md`, the README, the roadmap
11. Push: UnifiedPush and the embedded FCM distributor
12. Polish: the connect page's toasts and names,
    the self-signed notice in the app, the README's notes from the phone test,
    Dependabot for Gradle (done)
13. Check: the findings of the login's check on the owner's phone:
    the sign-in card, the menus, the server list, Turn off, the certificate,
    the appearance (done)
14. Second check: the findings of Block 13's check: the sign-in card,
    Forget all revoking, the keys left valid, Turn off with a session (done)
15. List and card: a tap asks the fingerprint, the card names its server,
    ☰'s two settings, Show in My profile and Remove in the server's settings,
    the password's eye with its switch (done)
16. Sessions and keys: no second card, this device's key and the ids,
    sessions signed out later, a token's own revoke (D80-B), a time limit (done)
17. One prompt and the lockout: one wrapping key for Forget all,
    an ended token not counted (done)
18. Emulator: an instrumented suite, `make app-emulator-test`, a CI job (done)
19. The list: ☰ in the header with the version row and Exit, the width,
    the order, Back, a server's settings in three sections, Store login,
    pull down, a page that fails to load
20. The menu and the cards: the server's ☰ and its version row,
    the card's server line, the command and ⓘ copyable, the copy's toast,
    the password pair, Sign out to the list, My profile's failures, the name trimmed
21. The sessions: the sessions held with the process and recorded,
    a forgotten token's revoke code and its route, no refused logout,
    the toast's two test cases, the list while a server does not answer;
    the server's cancelled request

## Discussion

**Where it comes from.**
The owner's words for item 16: run as a stand-alone service,
or connected to an existing Massimo; published on F-Droid and on Google Play;
item 17: the same for iOS, on the App Store.
Added on 2026-11-08 and 2026-11-09:
a route that builds an iOS package from the same source;
run the full service locally or connect to an existing one;
remember the address and choose a service's local or remote address by the
network the phone is on; several services to choose from; biometric login;
push notifications on certain events.

**What rests on the analysis.**
The investigation of 2026-11-08 and 2026-11-09 is beside this plan as
`2026-11-09_00-10_ANALYSIS_MobileApp_modes_stack_stores.md`;
its own title still says AndroidApp,
the plan's name before the owner chose MobileApp.
It holds the facts outside the repository with the date each was read:
F-Droid's inclusion policy and reproducible builds,
Android developer verification, Google Play's requirements,
Apple's guidelines and programme, gomobile's state, the platforms' stability,
UnifiedPush.

**Decided so far**, each recorded where it was taken:

- at the TL;DR gate of 2026-11-09: both modes, the client first;
  the web UI in a thin shell, a native adapter per platform,
  the Go core by gomobile; reproducible builds and our own signature on F-Droid,
  flavours `foss` and `play`;
  a personal account on Google Play with our own signing key,
  which runs the closed test of 12 testers for 14 days;
  `app/` in this repository; Web Push from the server as the one push transport;
  the toolchain in WSL;
- at the plan gate of 2026-11-09:
  **D1-A** the application id `ch.metaworx.massimo` on both stores,
  never changed after the first publish;
  **D2-A** `minSdk 26`, Android 8.0, with 24 in reserve, lowered only on request;
  **D3-A** the app released with the server's tag,
  the signed APK and the AAB on the same release (D12-A says how);
- at Block 2's gate:
  **D4-A** on a server's page the bridge has `token()` and `setToken(t)` without an id,
  and nothing else but `version`, the adapter answering for the server it opened
  by the message's origin, so that a page never asks for another server's token
  or learns its address;
  **D5-A** the axe check in the UI suite, `axe-playwright-python`
  (MIT, axe-core inside, MPL-2.0) in `tools/ui-test/requirements.txt`;
- at Block 3's gate, numbered D4 to D6 there before Block 2's D4 and D5 were seen,
  and at its move onto `main` on 2026-11-16:
  **D6-A** the bridge is messages over `addWebMessageListener`,
  not `@JavascriptInterface`, which offers its object to every frame and origin
  and answers synchronously, so a prompt could not answer through it;
  **D7-A** the Gradle wrapper is committed, `gradle-wrapper.jar` the one binary of
  `app/`, the distribution pinned by its SHA-256, no `gradlew.bat`;
  **D8-A** `languages` gains `xml`, with no `ignore = kotlin`,
  since `detect-languages.sh` sees the resources as XML and no Kotlin at all;
  **D9-A** the icon is Massimo's mark,
  made by `make ui-brand` (`tools/brand/make_brand.py`'s scope `android`);
- **D10-B** the phone test's findings in the page wait for this revision,
  **D11-A** this revision is the planning session's, and Block 8 becomes the
  login by fingerprint (the phone test of 2026-11-17);
- **D12 to D21** at Block 4's gate of 2026-11-17,
  numbered from D12 since the TL;DR gate had used D9 to D11 before the plan
  (`app/README.md` cites "D10-A of the plan", the toolchain in WSL):
  - **D12-A**, the version: `app/version.json`, committed, the changelog's manifest;
    `changelog.sh cut` writes its `version` in the release commit,
    `make release` its `versionCode` just before the cut,
    and the Guidelines job holds it to the changelog's newest release;
  - **D13-B**, the builds that carry the app: the version tags, and the nightly (D18-A);
  - **D14-A**, where it is signed: `build.yml` builds it unsigned,
    and "Sign the Android app" signs it in `signing`, beside `sign`,
    under the one approval;
  - **D15-D**, the sums and the attestation, the owner's A with C:
    the APK's and the AAB's lines join the release's `SHA256SUMS`,
    signed with the release key as `SHA256SUMS.sig`,
    and a job without a key attests the signed APK, the AAB and the new sums;
    `sign` signs the first `SHA256SUMS` too, so a release without the app has
    the signature as well, and `rolling` signs a rolling build's with the rolling key;
    the boxes' signed update manifest names no app file;
  - **D16-A**, the keys: PKCS#12 keystores, each a base64 secret with its password:
    `MASSIMO_APP_KEYSTORE` (alias `massimo`, the APK) and
    `MASSIMO_APP_UPLOAD_KEYSTORE` (alias `upload`, the AAB for Play) in `signing`,
    `MASSIMO_APP_ROLLING_KEYSTORE` (alias `rolling`, the nightly APK)
    in `signing-rolling`;
    their certificates under `app/android/signing/`,
    each signing compared with them before anything is published;
    without a secret or its certificate a release warns and carries no app;
  - **D17-A**, when the job Android app runs: as LAN's does,
    What to test's answer `app` and the Tested record's `app=`;
  - **D18-A**, the rolling APK: the nightly build's alone,
    `ch.metaworx.massimo.nightly`, "Massimo nightly", installed beside the release's,
    with an icon of its own, the mark in one colour with its hands carved,
    white on the night's dark blue;
    no devel build carries one, and no rolling build goes to Google Play;
  - **D19-A**: the nightly APK is signed inside `rolling`, before its one publish;
    a fault there warns, and the build goes out without the APK;
  - **D20-A**: "Publish the Android app" extends the sums,
    running only `sha256sum`, openssl and gh,
    after "Sign and publish" and "Sign the Android app",
    and checks the published `SHA256SUMS.sig` against the release key first;
  - **D21-A**, the versionCode: `MMmmPPbbbb`, the major at most 20, as Play allows,
    the minor and the patch at most 99,
    and the commits since the version's first tag at most 9999;
    the labels have no digits of their own, so only a version's first tag ends
    in `0000`;
    the release commit's number is counted before the commit exists,
    and `make release` checks it against the tag before the push;
- **D22 to D31** at this revision's gate of 2026-11-18, below.

**Revision v1.6 (2026-11-28).**
At v1.5's gate on 2026-11-27 the owner proposed a code that only revokes,
for D110, and sections in a server's settings, for D111,
and asked how a session could be bound to the phone.
The decisions on them, D109-A to D114-A, are below, each as recommended.
They change Blocks 19 and 21:
Block 19 builds the sections, Block 21 the revoke code,
its server half in a commit of its own.
An independent review of the revision, before it was presented, is folded in:
D116-A, taken under the owner's `PLAN++` as recommended,
bounds a removed server's codes;
and the blocks run 21, 19, 20, so that Block 20, which waits on D115 and on
AP ErrorCodes' bubble, comes last.
The bridge's contract takes version 6 with Block 21 and version 7 with Block 19,
in the order the blocks run.
AP KeyLife v1.0 is on `main` now, and is named by its path.
D115, found writing this revision, is open:
D97-A's toast and AP ErrorCodes' bubble both change what a copy shows.

**Revision v1.5 (2026-11-27).**
Blocks 15 to 18 are on `main` (metaworx/massimo#405, #408, #409 and #411),
and so are three decisions of v1.4's completion gate and the one after it,
D86-A and D87-A (metaworx/massimo#418) and D89-A (#420),
with D85-A and D88-A beside them.
The owner checked Blocks 15 to 17 on the phone on 2026-11-25:
every step passed but two, and the check found seven points, F41 to F47;
on 2026-11-27 the toast it had missed, F44, showed as it should,
and that run found F48.
The owner's decisions on them, D90 to D108, are below,
D108-A as recommended, the owner having named no option;
with D99-A they become Blocks 19 to 21.
This revision also folds in every NOTE of the plan, 27 of them,
the run NOTEs among them, at the owner's word:
what each recorded and still holds is in the body,
how each built block went under its "As built",
and the NOTEs' files are gone.
AP KeyLife v1.0 has taken over the decisions on keys v1.4 handed it.

**Revision v1.4 (2026-11-21).**
Block 14 is on `main` (metaworx/massimo#390).
The owner checked it on the phone on 2026-11-21:
it passed, F28 is settled,
and it found twelve points, F29 to F40,
which this revision gathers as Blocks 15 to 18.
The owner's decisions on them and on what the check raised are D58 to D83,
D84 open at this revision's gate;
those on My profile's keys, D75 to D79 and D81,
are for AP KeyLife, a plan to come.
AP FirstOwner's D18-A gives Block 7 a step.
How Block 14 was built is under its "As built" (v1.5).

**Revision v1.3 (2026-11-21).**
Block 13 is on `main` (pull requests 384 and 386).
The owner checked it on the phone on 2026-11-20:
it passed, and found four points,
which this revision gathers as Block 14 with the decisions D54 to D57.

**Revision v1.2 (2026-11-20).**
Blocks 8 and 12 are on `main` (pull requests 365, 366 and 368).
The owner checked the login by fingerprint on the phone on 2026-11-20:
it works, and the check found 24 points, two not reproduced,
which this revision gathers as Block 13 with the decisions D40 to D53.

**Revision v1.1 (2026-11-17 and 2026-11-18).**
Blocks 2 and 3 are on `main`: the connect page with the bridge's contract,
`docs/app.md`, and the Android shell in client mode.
On 2026-11-17 the app passed the plan's check on the owner's phone,
once one fault was fixed: discovery, a pinned self-signed certificate,
the sign-in by OPAQUE, the token across a restart (Block 3, "As built").
At that test the owner stated how the login is to work,
in place of the switch that guards the session token:
ten points, which this revision turns into Block 8
(the Analysis, "The owner's design").
Push, which shared Block 8, becomes Block 11,
so that Block 8 is the login alone;
the test's findings in the page and for the README become Block 12.
Block 4 ran under a gate of its own while this revision was written:
it is on `main`, its decisions are D12 to D21,
and this revision's therefore start at D22.
Two slips of v1.0 are corrected: the fees read "5 once" and "9 a year"
where $25 and $99 were meant, and "D9-A" named the personal account,
a decision of the TL;DR gate without a number in this plan.

**Decided at this revision's gate (2026-11-18):**

- **D22-A, the credential the phone keeps**:
  an API key of the user's own, labelled with the phone,
  made by the existing `POST /api/v1/users/{name}/tokens`;
  a new route turns it into a session;
  it is listed and revoked under My profile → Sessions and keys.
- **D23-A, who makes and holds it**:
  the adapter.
  The sign-in form learns from it that the login by fingerprint is there;
  where the user chooses it, the page asks the adapter,
  which makes the session at the server and hands the page that alone.
  The key never enters the page.
- **D24-C, when the fingerprint is asked** (the owner's):
  the user's choice, of three: each time the app comes back from being hidden;
  when the app starts; only after a logout.
- **D25-A, the PIN**:
  a fingerprint or face of the strong class,
  with the screen lock's PIN as the prompt's fallback on Android 11 and later;
  on Android 8 to 10 the fingerprint alone,
  since a Keystore key bound to the prompt takes no PIN there.
- **D26-A, "Select server" once signed in**:
  on the sign-in form, and in the app in the Cube choice's list,
  as the owner asked;
  the test had found no way back from a signed-in page but Android's Back.
- **D27-A, the phone test's two findings outside the app**:
  two lines of the roadmap's "Small ones", the coordinating session's to write;
  what they are is under "Not in this plan".
- **D28-A, the first release**:
  the client with the login: Blocks 12 and 8 after 4, then 5 and 6;
  the core and the stand-alone mode, Blocks 1 and 7, second.
  F-Droid's first recipe is then Gradle alone.
- **D29-A, Dependabot for Gradle**:
  a step of Block 12.
- **D30-A, where D24's choice is made**:
  on the phone, per server, in its entry on the connect page,
  where the switch was; "when the app starts" until the user chooses.
- **D31-A, a key and its sessions** (the owner's point):
  a session names the key that made it, in a column of its own,
  `tokens.parent_id`; revoking a key ends its sessions,
  and a new session by a key ends that key's earlier one.
  `tokens.origin`, which the owner asked about, is taken:
  every table has it, the id of the node that last changed the row,
  for the sync.

**Decided at Block 8's gate and since** (2026-11-18 to 2026-11-20):

- **D32-B to D37-A** at Block 8's gate, the owner's "as recommended",
  D32 and D33 left open by v1.1 and the others found by the gate;
  where they and the Analysis differ, they hold:
  - **D32-B**: one entry, "Select server", in the Cube choice,
    after the Cubes and the search;
    a server's page learns nothing of the other servers,
    `serverNames` and `switchTo` are not made, and `connect()` is the way back;
  - **D33-A**: while the page of a server asked at each return is shown,
    and only then, Recents keeps no picture of it:
    `setRecentsScreenshotEnabled(false)` from Android 13 (API 33),
    screenshots still allowed;
    `FLAG_SECURE` on Android 8 to 12, which forbids screenshots there too;
    set while such a page is shown, since the system takes the picture on the way out;
  - **D34-B**: one kept login per server, not per address,
    bound to the server's id, its `node`, its pinned fingerprint and the user's name,
    used at whichever address the page opened that answered with the server's `node`;
    a server's `login` is one name, or `""`;
  - **D35-A**: D25-A stands.
    A key that admits the screen lock (`AUTH_DEVICE_CREDENTIAL`, from Android 11)
    is not invalidated by a fingerprint enrolled later; one bound to the
    biometric alone (Android 8 to 10) is;
    turning the screen lock off invalidates it on every version
    (androidx's and the Keystore's references, read 2026-11-18);
  - **D36-A**: the kept key is never sent over plain HTTP to an address outside
    the private ranges (the connect page's warning's);
    there the card offers the password alone and says why;
    a key kept at the LAN address over plain HTTP is still sent there;
  - **D37-A**: Block 8 starts `docs/user-manual.md`, in English,
    with its chapter on the login by fingerprint in the app:
    what is kept, the three choices, the PIN and a new fingerprint, plain HTTP,
    and revoking the phone's key under My profile;
    listed in `docs/fragments/_documents.md` for the site;
    the manual shown in the app is a plan of its own,
    since rendering Markdown takes a build step the web UI has not;
    its roadmap line is the coordinating session's.
- **D38-A, D39-A** at Block 12's gate, numbered D33 and D34 there
  before Block 8's D32 to D37 were seen:
  - **D38-A**: every outcome of the connect page's list and add form is a toast:
    a server added, known, saved or removed, one that does not answer,
    no answer at an address, a certificate not trusted, and the failures, in red;
    an address typed that is not one stays under its field,
    and a server's open settings keep their own line;
  - **D39-A**: a server named by one of its addresses (its host and port)
    takes the name discovery hears for its node, saved;
    a name the user gave stays.
- **D40 to D53** at the phone check of 2026-11-20, numbered D38 to D51 there
  before Block 12's took D38 and D39:
  - **D40**: the fingerprint icon v2d, the outer line broken twice;
  - **D41-A**: "this device" on the session or key in use, "its key" on its key;
  - **D42-A**: "Enable login by fingerprint" on the first-owner form too;
  - **D43-A**: "Open at start", one server at most, on for the first added;
  - **D44-A**: no header while signed out; the version at the card's foot
    and first in its ☰;
  - **D45-A**: "Sign in with: Name | API key", "Select server" apart;
  - **D46-A**: the server list marks where the app holds a session,
    checked at each probe; Sign out per server and of all;
  - **D47-A**: the fingerprint per server; an app-wide lock may come later;
  - **D48-A**: a stronger border and shadow on every menu;
  - **D49**, the owner's variant of A: the sign-in row starts with a
    fingerprint button where a login is kept, which logs in at once;
    Name stays the pane chosen;
  - **D50-A**: a faint light glow around the menus in dark mode;
  - **D51-B**: a server's certificate as a line in its settings and a
    dialog with its subject, issuer, names, validity, fingerprint and
    when it was trusted;
  - **D52-A**: the findings as this revision's Block 13;
  - **D53-A**: an app-wide Appearance in an App settings card, applied to the
    list and to every server's page set to "as the system".
- **D54 to D57** at the gate of 2026-11-21, after the check of Block 13,
  recorded here:
  - **D54-C**, the owner's variant: "Select server …" in the card's ☰,
    and in the app a Cancel button left of "Sign in" that returns to the list;
    not a servers icon beside the ☰ (A), nor the ☰ alone (B);
  - **D55-A**: "Password" in place of "Name" in the row;
  - **D56-A**: Forget all revokes each key at its server as Turn off does,
    the fingerprint asked only where no session is held,
    and no prompt for the forgetting itself;
  - **D57-A**: a key that could not be revoked stays as its id and user,
    no secret, and the next session there revokes it;
    the server's settings name it, with "Show in My profile".
    Not a SHA-256 of the key with a search by hash (B, the owner's first
    thought): the app keeps the id already, a session may revoke its user's
    key by id, and a link marks the row better than a hash read by eye.
    Nor the key named alone (C).

**Decided at the check of Block 14 and since** (2026-11-21):
D58 to D73 at the check, the first two at its signal;
D74, D82 and AP KeyLife's below taken by the owner afterwards,
on the points the check raised:

- **D58-A**: Remove with a kept login turns it off first,
  as Block 14 built it.
- **D59-A**: the merge of metaworx/massimo#390, made with GitHub's button,
  stays as it is, though its summary fails the commit-msg check.
- **D60-A** (F29): on a touch screen the card focuses no field,
  and both ways keep one form height;
  built by AP UiParts in metaworx/massimo#397.
- **D61** (F30): the first owner's card is AP FirstOwner's,
  the owner's in AP UiParts' session:
  a card with three ways, create the owner, skip for now and keep open,
  the last the server setting `auth.keep_open`, after a confirmation;
  its D18-A gives Block 7 a step.
- **D62-A** (F31): the app shows the server under the sign-in card's title,
  its name and its full address, in place of the address bar it has not,
  with an ⓘ for its instance id and certificate fingerprint,
  from the bridge (the contract's version 5).
- **D63-A** (Forget all's two prompts): one wrapping key,
  so that Forget all asks the fingerprint once.
  It replaces Block 8's Keystore key per server (`LoginKey`);
  D34-B, one kept login per server bound to its id, node, fingerprint and user,
  stands, and so does D47-A, the fingerprint asked per server
  (the Analysis, "One prompt for Forget all").
- **D64-A**: the server does not count a token it issued and that has ended,
  revoked, logged out or expired, toward the address lockout.
- **D65-A** (F35): in the app the key the phone keeps is tagged
  "this device's key" on its row of My profile, from the bridge's key id,
  however the session was made.
- **D66-A**, the owner's idea: key ids shown at any time:
  the app's server settings show the kept login's key id whenever one is kept,
  and each row of My profile shows its id.
- **D67-A** (F37): My profile reads its list only once the sign-in's revoke
  of a key left is done; a key that sign-in revoked is not marked,
  and a toast names it.
- **D68-A** (F36): a 401 for a request sent before the page's current sign-in,
  with an older token or none, does not open the card:
  a read is sent again, a write fails as not sent;
  a session replaced is signed out, by the page and by the bridge.
  F36's cause is likely, not proven, so its UI test must fail on `main` first.
- **D69-A** (F38): Remove signs the app's session out before it forgets it.
- **D70-A** (F39): a session forgotten while its server is down is recorded
  like a key left, beside `unrevoked`, and signed out at the next contact.
- **D71-A**, the owner's question whether these checks belong to tests:
  each fix comes with a test at the level that holds it
  (F36 a UI test, D69-A and D70-A the app's JVM tests,
  F40's rule a pure function with its JVM test),
  and a block of its own adds an emulator suite,
  the WebView with the bridge and the fingerprint simulated,
  run locally and in CI on a change under `app/`;
  the owner's phone check keeps what an emulator cannot show:
  a real fingerprint, the phone maker's autofill, the look.
- **D72-B** (F40): a server asked at each return asks
  each time it is opened from the list, whether the app was hidden or not.
- **D73-A**, the owner's question whether the app's settings and a server's
  could be entries of the page's ☰,
  shown in the page and saved through the bridge:
  two entries in the app alone, "This server's settings…" and "App settings…",
  hand over to the app's own page, as "Select server…" does;
  the server's page neither reads nor saves them,
  since a server that could would turn its own fingerprint off,
  trust another certificate or see the other servers.
- **D74-A**, raised at the check's end: where a login by fingerprint is kept,
  tapping a server in the list asks the fingerprint
  (the prompt allows the device's PIN, so it has no "Use password" button);
  a cancel or a failure opens that server's sign-in card, not the list;
  the row's fingerprint button gives way to a sign-in icon,
  the mirror of the sign-out icon, labelled "Sign in with password or key",
  which opens the card without the prompt.
- **D82-A** (F32): a time limit on the page's API calls, in `apiRequest`:
  a call unanswered after about 20 s fails as "no answer",
  and its button stops spinning;
  it is shared code, so the session "UI refactor" is told.

- **D80-B** (a token's own revoke), the owner's,
  after asking how A works and whether B makes the flows simpler:
  also `DELETE /api/v1/me/token`,
  which revokes the bearer whatever it is, a session or a key,
  a key's sessions with it, audited as `token.revoke`:
  one call for any token, without a name or an id.
  The app's revoke with the key alone uses it
  (Turn off, Forget all and Remove where no session is held),
  and so can Home Assistant's integration.
  It does not replace a session's revoke of a key,
  whose bearer is not the key, and `POST /api/v1/logout` stays.
  Today a key revokes itself with `DELETE /api/v1/users/{name}/tokens/{id}`
  as its own bearer, which needs the user's name and the key's id.
- **D83-A** (the look at metaworx/massimo#397's card), the owner's:
  not done on the phone; its two checks, D60-A and the password manager's
  autofill, go into Block 15's phone check,
  on the card as metaworx/massimo#401 to #404 have left it.

**Decided at v1.4's gate** (2026-11-23),
the owner's "D84 yes but build it with a flag so it could be disabled",
and `EXEC++`:

- **D84-A, the password's eye, with a switch**:
  a button with a closed eye at the right end of a password field,
  which shows the password in clear text and hides it again,
  in the shared field (`src/shared/password.tsx`), so that every password field
  has it: the sign-in, the first owner's two, Personal settings' change of password;
  a labelled toggle (`aria-pressed`), keyboard operable,
  the field hidden again at each opening of the card and at the submit;
  Android's WebView has no eye of its own,
  and a browser's own (Edge's `::-ms-reveal`) is hidden, so that a field has one eye.
  Not B, the sign-in card's password alone.
  The switch: a server setting, on by default, among System settings' own,
  written by whoever may write the server's settings,
  learnt before anyone signs in, so that the card shows the eye or not from its
  first drawing; switched off, no field shows the eye,
  and a browser's own stays hidden.
  Built as `auth.password_reveal` (Block 15, "As built").
- **The run**: v1.4 committed onto `main`;
  Blocks 17, 16, 15 and 18 in that order, each on its own branch and pull request,
  each merged by the run's session once its CI was green,
  the next started from `main`;
  the four blocks' phone checks gathered into one phone session at the end;
  no retirement, no release, no roadmap edit.

**Taken over by AP KeyLife v1.0**
(`GUIDELINES/wip/2026-11-27_16-53_AP_KeyLife_the_life_of_api_keys.md`),
a plan of the session "Open Items", which has taken over the decisions on
keys and tokens (the owner, 2026-11-27):
the owner's decisions on My profile's keys of 2026-11-21,
the second point raised at Block 14's check's end,
which v1.4 handed to a plan to come, each marked below and none planned here.
D80-B stays this plan's, built in Block 16.
Two of KeyLife's decisions meet v1.5's steps:
KeyLife.D5-A in Block 19's Store login, and the session ends KeyLife adds
in Block 21's D100-A.

- **D75**, the expiry (taken over by AP KeyLife v1.0), in the owner's words
  "BA+B, where a key can be excluded from A. but not integration keys,
  they should also be subject to A, and have an optional B":
  an idle limit for API keys (A), a server setting,
  from which a key may be excluded, but not an integration's key,
  which is always subject to it;
  and an optional end date per key (B), an integration's key included.
  For KeyLife to weigh: an ended key breaks what holds it quietly,
  the app forgetting its kept login at the 401,
  Home Assistant having to register again,
  and the key of `MASSIMO_API_TOKEN`, which the bootstrap adds back only where
  its hash is unknown, no longer letting the owner in.
- **D76-A**, the re-key (taken over by AP KeyLife v1.0):
  "New secret" on a key's row;
  the id and the label kept, the new secret shown once,
  the key's sessions ended but the caller's own.
- **D77-A, D78-A** (taken over by AP KeyLife v1.0):
  the last use and the last address are on every row already; nothing to build.
- **D79-A**, the renewal (taken over by AP KeyLife v1.0):
  none, since use renews a key under the idle limit;
  weighed again as KeyLife.D4-A: no key moves its own end date.
- **D81-A** (taken over by AP KeyLife v1.0):
  a key is never shown again: the store keeps only its hash,
  and the re-key is the way back.
- Two gaps found on the way (taken over by AP KeyLife v1.0):
  a session made from a key checks only the key's revoke, not its end
  (`stillAdmitted` in `internal/server/auth.go`, `liveKey` in
  `internal/store/tokens.go`), to close before any key ends;
  and any key can make more keys (`POST /api/v1/users/{name}/tokens`),
  so revoking a leaked key leaves the keys it made.

**Decided after Block 18** (2026-11-24 and 2026-11-25),
at v1.4's completion gate and after it:

- **D85-A, KVM for the user**, the owner's "added":
  the owner put the user into the group `kvm`;
  a process started before that reaches `/dev/kvm` through `sg kvm -c '…'`,
  without a new login.
  A reboot or a new login waits for the other sessions still running.
- **D86-A, the settings file's key**, "small fix here" (metaworx/massimo#418):
  `DELETE /api/v1/users/{name}/tokens/{id}` answers `409` for the key that is the
  settings file's `MASSIMO_API_TOKEN`, with the words `DELETE /api/v1/me/token`
  answers since Block 16;
  `GET …/tokens` marks it `"from_settings": true`,
  and its row, under My profile and in System settings → Users,
  says "from the settings file: change or remove it there" in place of Revoke.
  The server finds the key by the hash of the file's token, which the store keeps
  unique, never by its label; no migration.
  A revoked key keeps its row and its hash, and `store.Bootstrap`,
  which reads live keys alone, then failed on that hash at every start
  ("store: bootstrap failed") until the file's token changed: that way is closed.
- **D87-A, the emulator suite in the test gate**,
  "and install the required emulator":
  the contract (v1.19.0) names `make app-emulator-test`
  for a change under `app/` or to what of the server the app drives,
  where KVM is open to the user; CI's job Android emulator runs it in any case;
  `tools/app-emulator/README.md` says how the user joins `kvm`.
- **D88-A, `compileSdk 37`** (2026-11-24, `EXEC D88-A`),
  on Dependabot's metaworx/massimo#369, which could not build:
  `androidx.core` 1.19.1 asks whoever depends on it to compile against 37,
  and the wrapper's bump in the same group failed with it.
  `compileSdk` rises to 37; `targetSdk` stays 36 and `minSdk` 26,
  so the app behaves on a phone as before;
  this replaces Block 3's 36 for the compile level and for nothing else.
  Dependabot's Gradle group is split in two, the libraries,
  and the build's own tools (the Gradle wrapper and the Android Gradle Plugin),
  so that a library that cannot build holds no wrapper back.
  Not B, an `ignore` for `androidx.core` from 1.19,
  since every AndroidX library will ask for 37 and an `ignore` withholds its fixes.
  It was first offered as "D40" on 2026-11-20, a number v1.2 took that day.
- **D89-A, the user that holds the settings file's key** (2026-11-25, `EXEC D89-A`,
  metaworx/massimo#420): `DELETE /api/v1/users/{name}` answers `409` for that user,
  "user <name> holds the settings file's MASSIMO_API_TOKEN:
  change or remove it in the settings file first",
  since the removal revoked the key with the user's tokens and Bootstrap then
  failed at every start;
  not B, Bootstrap making the key live again, nor C, leaving it.
  The server's `settingsKey` returns the key with its user's id,
  so that `deleteUser` checks it before `store.DeleteUser` in one lookup.
  The other paths that end a token end none of the file's key:
  a logout, a password set by another user and a key's next session end sessions
  alone; an integration's approval and revoke end its own keys, whose user is
  never an owner; Bootstrap revokes a key labelled as the file's whose hash is not;
  no migration ends a token.

**Decided at and after the phone check of Blocks 15 to 17**
(2026-11-25 to 2026-11-27):
D101-A at the check;
the others at its gate, the owner's "all open decisions as recommended",
each in its option A:

- **D101-A**, Forget all with the owner's own server in the list,
  the owner's "D101-A, go on":
  its key there was revoked and the phone signed out;
  the owner turns its login on again.

The list, the connect page (Block 19):

- **D90-A**: one ☰, in the header at the logo's right,
  where a server's page has its ☰:
  the version row, a rule, Add server…, Sign out of all,
  Forget all stored credentials, a rule, App settings, Exit.
  The Servers card keeps ↻ alone.
  Not two menus, one for the app and one for the servers.
- **D91-A**: Exit ends the app as a swipe from the recent apps does:
  the sessions held in memory dropped, nothing sent to a server,
  the app out of the recent apps (`finishAndRemoveTask`).
  With D102-A, a session it dropped is ended at the next sign-in as that user.
  Android only: the iOS shell has none.
- **D92-A**: the cards take the page's full width,
  up to the limit of a server's page (1100 px).
- **D93-A**: Back in a panel with changes asks once:
  discard the changes, or stay.
- **D94-A**: in a server's settings, Save and Cancel are off while
  the form holds what is stored, as the server's forms are;
  the panel is closed by Back or by a labelled ✕ in its head.
- **D95-A**: the servers sorted by the name shown
  (the server's name, else its address) in the phone's language's collation:
  case and accents ignored, numbers by their value.
  A manual order is a later plan, for the roadmap's owner.

The menus and the cards (Block 20):

- **D96-A**: no "…" on an entry that only opens a page or a panel,
  in both menus (Select server, This server's settings, App settings);
  Add server… keeps it.
- **D97-A**: every copy, on a phone and in a browser, shows the page's toast
  "Copied" in place of the word at the element;
  in the app on Android 13 and later the system's preview shows beside it
  (D115 asks how, beside AP ErrorCodes' bubble).
- **D98-A**: the new password's repetition is outlined in red
  while it is not empty and differs from the first,
  with the words "The two passwords differ" under it.
- **D104-A** (F45): a request's failure under My profile also shows
  in the page's toast; the message line keeps it after the toast goes.

The sessions (Block 21):

- **D100-A** (F41): the app drops a session without a logout
  where the server has ended it itself:
  a new session made from the key, which ends the key's earlier one,
  and a key revoked with its sessions.
- **D102-A** (F43, F47): each server keeps the user and the id
  of the session held there, once its id is learnt; no secret.
  It is cleared where the session ends.
  At the app's start, one whose session is no longer held moves into `unended`,
  and the next sign-in as that user ends it, as today.
  Also: a fingerprint tap learns its session's id at once;
  the sessions held live with the process, not the activity,
  so that a change of dark mode or of the phone's language keeps them;
  `Sessions.look` logs what it swallows.
- **D103-A** (F44): the toast seen on the phone first,
  then fixed for what that shows, with an emulator case and a UI case
  whose bridge answers at once.
  The first part ran on 2026-11-27 (`EXEC phone F44`):
  the toast showed, nothing was found to fix,
  and the two cases stay, so that a regression would show (Block 21).
- **D105-A** (F42): while a server does not answer, the list's
  "signed in as …" turns grey, and the settings' session line says
  that Sign out ends the session there once the server answers again
  (D110 asked how, the app keeping no secret it was asked to forget;
  D112-A's revoke code is how).
- **D106-A** (F46): only the password routes answer a cancelled request
  as "too many password checks at once";
  elsewhere the audit says the client went away before the answer,
  the result `failed` as now.
  A server fix, in its own commit.
- **D107-A**: the trailing space a password manager's autofill leaves after
  the user name stays: every sign-in path trims it, the server too.
  D99-A names no block for it; it goes with the cards, Block 20.

Where the work goes, and the last one:

- **D99-A**: this revision, v1.5, in three blocks,
  the findings and decisions keeping their numbers:
  Block 19 the list (the ☰, the version row, Exit, the width, Back,
  a server's settings, the order, Store login; `connect.html`, `connect.js`,
  `MainActivity.kt`);
  Block 20 the menu and the cards (the server's ☰, the version row,
  the card's line, the command, ⓘ's copy icons, the copy's toast,
  the password pair, Sign out to the list, D104), shared parts,
  so the server's page in a browser changes too;
  Block 21 the sessions (D102, D100, D103 and D105 in the app, D106 in the server).
- **D108-A** (F48), the recommended option, the owner having named none
  at the gate of the F44 run:
  Show in My profile goes through the reachability check a tap on the server's row
  makes, and a server's page whose main frame fails to load returns to the list
  with the server marked not reachable and the reason said,
  so that no way into a server's page ends on the WebView's own error page.
  The check, read in the code, is the same for both today (`openServer`);
  how Block 19 makes it hold is in the Analysis, "A page that fails to load".

**Taken as written**, the owner's proposals of the phone checks that asked nothing,
as this revision builds them:

- **the version row**, first in both menus: the build's version on the list,
  the server's on its page;
  a tap copies it, shows the copy's toast and closes the menu;
  the row keeps its copy icon (Blocks 19, 20);
- **Back** closes an open menu first, then an open panel
  (a server's settings, Add server, App settings), back to the list;
  only then does it leave the app (Block 19);
- **a server's settings**: the certificate and key part below Cancel and Save,
  beside Remove server (Block 19), which D111-A has replaced by three sections;
- **pull down on the list** refreshes it, as ↻ does
  (the F44 run's proposal; Block 19);
- **Store login** in a server's settings, where the app holds a session there
  and keeps no login: the fingerprint asked once, then the key made by that session,
  as a password sign-in that keeps the login makes it;
  off while the server does not answer, its title saying why (Block 19);
- **the server's ☰ in the app**: the version row, the pages, Personal settings,
  a rule, App settings, This server's settings, Select server, a rule, Sign out;
  in a browser, without the three app entries, the two rules draw as one (Block 20);
- **the sign-in and first owner's cards**: the server's line
  (its name, its address, ⓘ) above the card's title,
  the address and ⓘ held together on one line (Block 20);
- **the first owner's card**: `massimo user add` in monospace, on one line,
  copied by a tap or by its copy icon (Block 20);
- **ⓘ's instance id and fingerprint**: each with its copy icon (Block 20);
- **the new password and its repetition**: one eye, on the first field,
  showing and hiding both (Block 20);
- **Sign out in a server's page**, in the app, ends the session and shows the
  app's list; in a browser as now (Block 20);
- **a manual order of the list**: a later plan, for the roadmap's owner.

**Decided at v1.5's gate** (2026-11-27):
the owner proposed at the gate a code that only revokes, for D110,
and sections in a server's settings, for D111;
their evaluation offered D111 again, with D112 and D113;
the owner then asked how a session could be bound to the phone (D114),
and gave the signal "send the plan to a subagent for review.
If issues are easy to resolve, PLAN++; if issues require bigger design decisions,
show an updated PLAN instead", every open decision as recommended:

- **D109-A, a tap on a server's row**: it stays immediate,
  opening where the list's last probe found the server (`openServer`);
  a main frame that fails to load returns to the list (D108-A; Block 19, step 10).
  Not B, a probe before every tap,
  up to four seconds more where the server does not answer.
- **D110, the words of D105-A**: no option of its own any more; D112-A settles it.
  For a server that knows the revoke route, D105-A's words hold as first written:
  Sign out "ends the session there the next time the app reaches the server".
  For a server older than the route, which answers `404`,
  the app falls back to today's path, the session in `unended`
  and ended at the next sign-in there as that user,
  and its line then says so, in the words of a session left
  (`connect.sessionLeft`).
- **D111-A, revised: a server's settings in three sections**,
  switched as the server's page switches between its settings pages,
  in their look (`ui/style.css`, which the connect page loads),
  built by the plain script with `components.js`' builders and the roles of a
  tab list (`src/shared/tabs.tsx` is a part, and the connect page loads none):
  - **Connection**: the name, the LAN and remote addresses, Open at start,
    notifications, when the fingerprint is asked; then Cancel and Save;
    then Remove server under its rule (F34);
  - **Login**: the session with Sign out, the kept login with its key's id and
    Turn off, Store login, and the keys and sessions left with Show in My profile,
    each acting at once, with no Save;
  - **Certificate**: the certificate's facts in place, what `#cert-dialog` shows
    today, the dialog removed; where the server does not answer,
    the pinned fingerprint.

  Save and Cancel (D94-A) are Connection's alone.
  The draft is kept across the sections;
  Connection's tab carries the "changed here" mark while the form differs,
  as AP UnsavedMarks marks a settings page that has changes,
  and Back or ✕ asks once (D93-A).
  Login's tab carries a dot while a key or a session is left there.
  Connection opens first.
  Checked at 390 px in the six languages,
  every key in all six `ui/i18n/<lang>.json`, FR/IT/NL/PL machine-translated
  and marked.
  It replaces v1.5's D111, what moves below Save, in Block 19.
  Not B, one panel with every line that acts at once below Save.
- **D112-A, a revoke code** for a token the app forgets, the owner's proposal:
  - the code is `SHA-256("massimo revoke v1" ‖ secret)`, hex;
    the store keeps `SHA-256(code)` in a column of its own, `tokens.revoke_hash`,
    not the hash it keeps already (`store.HashToken`),
    with which the store, and every backup of it, could revoke every token;
    the server writes the column when it makes a token,
    and for an older one at its first use after the update;
  - `POST /api/v1/tokens/revoke`, without a token, its body `{"code": "<hex>"}`,
    revokes the one live token the code matches, a key's sessions with it
    (with AP KeyLife's Block 2, its heirs too),
    and answers `204` whether it matched or not, so that it tells nobody which
    codes are live; `400` for a malformed body; `429` and `5xx` mean later;
    the audit names the token's id, its user and the caller's address,
    never the code;
  - the app works out a token's code whenever it holds the secret,
    at a sign-in, a key's making and a fingerprint's unseal,
    and keeps it on disk, unsealed, beside D102-A's `{user, id}`:
    the code lets nothing in, so D70-A holds;
  - once it has dropped the token (Sign out, Turn off's key left, Forget all,
    Remove, Exit, the process ended: F43, F47) the app posts the code at each
    contact, a start, a return, ↻ or a probe; it drops it on `204`,
    falls back to `unended` and the keys left on `404`,
    and keeps it on a broken connection, `429` or `5xx`;
  - codes outlive Remove (F39): the app goes on posting a removed server's codes
    until they are taken, no longer than the tokens' life,
    and with D116-A no longer than 90 days;
  - a kept login sealed before this version has no code until its next unseal,
    and until then its key falls to today's path (the Analysis,
    "The gate of v1.5", "The app's side").

  Not B, the hash the store keeps as the code: no migration,
  but every copy of the store could revoke every token;
  nor C, D110-A's words now and the code in a later plan.
- **D113-A, who builds D112**: Block 21,
  the server half in a commit of its own, as D106-A's is;
  the session "Open Items" is told before Block 21 starts,
  since AP KeyLife's Block 2 changes the same revoke
  and its migrations draw on the same range of numbers:
  whichever lands second adapts.
  Not B, a block of AP KeyLife's, with Block 21 building D110-A's words meanwhile.
- **D114-A, sessions bound to the phone**: a plan of its own, later,
  proposed to the roadmap's owner, the coordinating session; not in this plan.
  The client would keep a private key, on Android in the Keystore,
  not exportable and unlocked by the fingerprint, and sign each request
  (DPoP, RFC 9449, or HTTP Message Signatures, RFC 9421);
  the server would keep the public key,
  and forgetting a session would be deleting its key,
  dead at once even while the server is down.
  A token is a bearer today, and need not be more against guessing:
  32 random bytes the server makes (`store.NewSecret`), worth nothing elsewhere,
  their SHA-256 unguessable;
  OPAQUE is a costly exchange of several messages, run once per sign-in,
  whose outcome is the token.
  The gap is that a bearer serves whoever reads it,
  which proof of possession closes, and a PAKE does not.
  D112-A stays meanwhile: it is small, covers the key Turn off leaves as well,
  and keeps its use as tidying once sessions are bound.

**Taken under the owner's `PLAN++`** (2026-11-28), raised by the review of this
revision, as recommended, and the owner's to change before Block 21's pull
request merges, as AP KeyLife recorded its D6 to D8:

- **D116-A, how long a removed server's codes stay**:
  every code in `gone` is dropped 90 days after the Remove at the latest,
  a session's after 30 days as planned.
  Remove means "forget this server":
  without a bound the app would go on reaching a removed server's addresses for
  years, and an address given to another host would go on receiving codes.
  A key left there stays in sight under My profile,
  and AP KeyLife's idle limit can end it.
  Not B, a key's code kept for as long as the key may live, which is without end.

**Open at this revision's gate**, found writing it, with its options,
the recommended first; it is to be decided before Block 20 alone,
which runs last:

- **D115, a copy's word: D97-A's toast or AP ErrorCodes' bubble.**
  D97-A shows "Copied" in the page's toast, in place of the word drawn at the
  element (Block 20, step 4).
  AP ErrorCodes' Block 6 (metaworx/massimo#489, open), at the owner's words
  there, makes the copy's word a bubble of its own, `showBubble(text, ctrl)`:
  above `ctrl`, or at the window's bottom centre without one.
  Both change `copied` (`components.js`), and the later would undo the other.
  - **A (recommended).** Both, through the bubble: every copy calls
    `showBubble` without a control, so that its word shows at the window's foot,
    where D97-A wants it seen on a phone, by the one builder the owner asked for;
    the toast stays the outcomes', and a copy no longer replaces one shown there.
    Block 20 then follows ErrorCodes' Block 6.
  - **B.** The bubble above the control for every copy; D97-A is dropped.
  - **C.** D97-A as written, the page's toast; ErrorCodes' bubble keeps its
    other users.

  The review agrees with A, for one more reason:
  the version row's tap closes ☰, a `<details>` (`src/shared/menu.tsx:86`),
  and the bubble of metaworx/massimo#489 hides itself once its control has no
  client rects (`followBubble`), so B's bubble would vanish at once in the very
  case D97-A is about.
  Two caveats of A: on Android 13 and later the system's clipboard preview also
  sits at the screen's foot, so Block 20's phone check looks at both at 390 px;
  and A changes AP ErrorCodes' "`copied(el, ok)` becomes `showBubble(t(…), el)`"
  and #489's test of the bubble above its control,
  so that plan's session is told.

The next free number is D117.

**Left open by the runs**, none of them a step of this revision,
each for a block that touches it or a plan of its own:

- a version tag's signing of the APK and the AAB, and the build provenance
  attestation of the signed APK, not seen yet: the owner's next `make release`;
- whether F-Droid's buildserver makes the same APK, and has platform 37: Block 5;
- `targetSdk 37`, which opts the app into Android 17's behaviour:
  a step of its own, with a phone check (D88-A);
- CI does not check `gradle-wrapper.jar` against Gradle's published sums,
  only the distribution's: offered at D88-A's gate and not taken;
- `login.keep`'s undo is lost where the activity is made anew during its prompt,
  and its new key stays valid, recorded nowhere (Block 14);
- the toast covers the open settings' foot at 390 px until it is closed
  (older than Block 14): Block 19 rebuilds that foot and looks again;
- androidx.biometric may cancel a second prompt started while the first closes,
  that key then left, not stuck (Block 14);
- a used stream ticket that an EventSource sends again still counts toward the
  lockout, outside D64-A; `making`, a keep making the app key, is cleared only by
  its prompt's answer, a gap that is theoretical (Block 17);
- `Bridge.keep` still reads the session on the main thread,
  the page awaiting `setToken` avoiding the race;
  `POST /api/v1/update` behind a running look could pass its 75 s limit,
  only in a rare double wait (Block 16);
- three UI test files give Playwright's clock milliseconds where it takes seconds,
  as `test_personal_updates.py` did and pass so far: a task of its own (Block 16);
- each emulator case of Blocks 15 to 17 seen failing once with its fix undone,
  which Block 18's Verification asked and KVM then barred: undone so far;
- disabling the user that holds the settings file's key, an end date that passes,
  or another role leave the key live but take the owner's rights from it,
  and the file then recovers no owner until that is undone; and a store whose
  key was revoked before D86-A keeps Bootstrap's error at each start until the
  file's token changes: left as they are (D89-A);
- the owner's own server's audit after D101-A's Forget all: not read.

**Names chosen here**, routine and not decisions of the owner's:
the directories `app/core`, `app/android`, `app/ios`;
the Makefile targets `app-core-android` and `app-core-ios`;
the route `POST /api/v1/login/key` and its audit kind `login.key`;
the column `tokens.parent_id` and its migration, 0025;
the bridge's calls `login.status`, `login.keep`, `login.fingerprint`,
`login.forget` (not built), `connect`, `forget` and `forgetAll`,
and with D32-A `serverNames` and `switchTo` (D32-B: not built);
a server's fields `login` and `ask`,
the latter one of `return`, `start` and `logout`;
the classes `LoginKey` and `LoginStore`; the branches `claude/mobile-core`,
`claude/mobile-login`, `claude/mobile-push` and `claude/mobile-polish`,
one per block.
With v1.4: on a server's page `login.status()`'s fields `server` and `key_id`,
`setToken`'s answer `{revoked}`,
and the calls `openServerSettings` and `openAppSettings`;
on the connect page `open`'s option `{signIn: true}`,
a server's fields `login_key` and `unended`, and `remove`'s `sessions`;
the class `Opening`, F40's rule;
the Keystore alias `massimo.login.app` of the one wrapping key;
the route `DELETE /api/v1/me/token`;
the target `make app-emulator-test`, its scripts in `tools/app-emulator/`,
and the job "Android emulator";
the branches `claude/mobile-list-card`, `claude/mobile-sessions-keys`,
`claude/mobile-one-prompt` and `claude/mobile-emulator`.
Built with them: the server setting `auth.password_reveal` and `/healthz`'s
`password_reveal`; the classes `AppKey` and `JvmKeys.kt`'s `FakePrompt`;
the test `EnrollTest` and the counter `tools/app-emulator/tally/`.
With v1.5: on the connect page the calls `exit()`, `about()` and `keep(id, url)`,
the event `refresh` and the handover `#unreachable=<id>`;
a server's field `held`; the class `MassimoApp`, the app's `Application`;
the library `androidx.swiperefreshlayout`;
the branches `claude/mobile-list-menu`, `claude/mobile-menu-cards`
and `claude/mobile-sessions-held`.
With v1.6: the route `POST /api/v1/tokens/revoke`, its error code
`revoke_code_malformed`, the column `tokens.revoke_hash` and its migration
`<NNNN>_token_revoke`, the store's `RevokeCode` and `RevokeByCode`;
in the app the class `RevokeCode`, `ServerApi.revokeCode`,
a key's or a session's left `code` and `taken`, and a server's record kept for
its codes after Remove, `gone`; on the connect page an entry's `pending`,
the sections `connection`, `login` and `certificate`, the classes
`server-tabs` and `server-section`, the template `tpl-cert`, `fillCert`,
and the builder `signEl` of `components.js`;
the audit's action `token.revoke_code` and its texts `log.action.token.revoke_code`;
the app's file `gone.json`; the texts `connect.sessionDown`,
`connect.sessionPending`, `connect.keyPending`, `connect.signedOutPending`,
`connect.loginOffPending` and `connect.removedPending`.

**Not in this plan:**

- pairing by QR code (a code shown by the web UI with the URL, the
  fingerprint and a one-time token): a later revision;
- an APNs relay for iOS push, and iOS push at all in the first iOS release:
  decided when Block 9 starts;
- the sign-in through OpenID Connect inside the app's WebView: the login by
  fingerprint follows whichever sign-in succeeded, but a provider's page in
  the WebView is untested;
- the life of a key, its idle limit, end date and re-key: AP KeyLife v1.0, above;
- sessions bound to the phone by proof of possession (D114-A),
  a key kept in the Keystore signing each request:
  a plan of its own, proposed to the roadmap's owner;
- a manual order of the server list (D95-A): a later plan, for the roadmap's owner;
- showing the user manual in the app (D37-A): a plan of its own;
- saving a file the page offers (a backup's download): the app says it
  cannot yet (Block 3, "As built"); a later call of the bridge;
- the phone test's two findings outside the app (D27-A),
  for the roadmap's "Small ones".
  The installer's Windows Firewall rule covers the Domain and Private profiles,
  and the owner's Wi-Fi was Public.
  It stays so by default (the owner, 2026-11-18);
  the Public profile is opened on request,
  by an option of the install command or, the owner's preference,
  from the interface, the Maintenance page perhaps,
  behind a UAC prompt and not the elevated installer.
  A service has no desktop to raise that prompt on,
  which the line's own planning has to settle.
  And `massimo tls show` reads the settings, not the running server;
- the Cubes of every signed-in server in the Cube choice,
  with "Log in…" for a server that is not signed in,
  the owner's thought of 2026-11-18, for later:
  a server's page cannot read another server,
  so the adapter would fetch each one's Cubes; a later revision;
- widgets, a wear app, Android Auto;
  e-mail or webhook delivery of notifications.

**Who runs it:**
sessions of their own, one per block, after this revision is on `main`.
Blocks 12 and 8 first (D28-A), Block 8 with a gate of its own for D32;
both change the sign-in form and the connect page,
so the later one merges `main` with the other's change before its pull request.
Then 5 and 6; Block 1 at any time and 7 after it, for the second release;
11 after 8, since both change the bridge; 9 after 8, on a Mac;
10 spread over the others.
v1.4's blocks: 17 at any time, its server's half first,
since the lockout hampers every check on the rig;
16 before 15, although numbered after it:
both change the connect page, the bridge and the card,
and the session a cancelled prompt of Block 15 leaves behind the card
is signed out by Block 16's rule for a session replaced (D68-A);
18 last, its suite covering the flows of 15 to 17 as built.
Blocks 15 and 16 change AP UiParts' parts (`src/signin/`,
`src/chrome/Menu.tsx`, `src/shared/tokens.tsx`, `src/profile/`),
the page's shared code (`app.js`, `login.js`)
and the card AP FirstOwner is reworking,
all of which the session "UI refactor" is changing:
it is told before each block starts,
and whichever lands second merges `main` first.
v1.5's blocks branch from `main` and target 0.18,
0.17 being frozen on `maint/0.17`; the plan and its revisions still go onto `main`.
21 first, its two server's commits first, since 19 builds Exit on its record
and both change `connect.js`;
then 19, rebuilding a server's settings around 21's lines into its sections;
20 last, since it waits on D115 and on AP ErrorCodes' bubble
(metaworx/massimo#489).
Block 19 builds nothing on Block 20: its version row calls `copied()` as every
copy does, and shows the copy's toast or bubble once Block 20 has landed.
Each starts from `main` once the one before has merged.
Before Block 21 starts, three sessions are told:
"Open Items" (D113-A), since AP KeyLife's Blocks 2 to 5 change the revoke,
the making of a token and its use, and take the migrations 0100 and 0101
(the list of what they would change is in the Analysis, "The gate of v1.5");
AP ServerBackup's, whose metaworx/massimo#492 takes 0100 too;
and AP ErrorCodes', whose #488 changes `userFail` and `passwordBusy`;
whichever lands second adapts.
Before Block 20 starts, AP ErrorCodes' session is told of D115.
Block 20 changes shared parts of the server's page (`src/chrome/`, `src/signin/`,
`src/shared/`, `src/profile/`, `components.js`, `login.js`):
any session changing them then is told before it starts.

## Analysis

### What the app is

One page of our own, `connect.html`, bundled in the app from the same
`ui/` sources the server embeds, and the server's page itself, loaded
from the server the user chose or from the embedded core.
The adapter is native, small, and does what a page cannot.
The two talk through one object, `window.massimo`,
whose contract is `docs/app.md` (Block 2):
every call a Promise over the platform's message channel,
the adapter checking each message's origin,
so that the connect page gets the server list and a server's page only what is its own.

| What the adapter does | Calls and events | Block |
|---|---|---|
| the saved servers | `servers`, `save`, `remove` | 3 |
| discovery of `_massimo._tcp` | `discover`, the event `found` | 3 |
| reaching an address, the pinned certificate | `probe`, `trust`, `open`, the event `network` | 3 |
| a server's session, kept on the phone | `token`, `setToken` | 3 |
| the login by fingerprint, the way back to the servers | `login.status`, `login.keep`, `login.fingerprint`, `connect`, `forget`, `forgetAll` (D32-B: no `serverNames`, `switchTo`) | 8 |
| the sessions held, the settings, the keys left | `session`, `signOut`, `signOutAll`, `settings`, `saveSettings`; a server's `unrevoked` | 13, 14 |
| a tap's prompt, the server under the card's title, the app's settings from ☰ | `open`'s rule and `{signIn: true}`, `login.status`'s `server`, `openServerSettings`, `openAppSettings` | 15 |
| the phone's key in My profile, the sessions left | `login.status`'s `key_id`, `setToken`'s answer, a server's `login_key` and `unended` | 16 |
| the list's ☰, Exit, Store login, pull down, a page that fails to load | `about`, `exit`, `keep`, the event `refresh`, the handover `#unreachable=<id>` | 19 |
| the session held, recorded across the process's end | a server's `held` | 21 |
| the server on the phone | `core.*`, the event `core` | 7 |
| push to this phone | `push.*`, the event `notification` | 11 |

The page keeps a server as `{id, name, node, lan, remote, fp, …}`:
`node` is the server's `node_id` from `/healthz`, so that a LAN address that
changed is found again by discovery, and the page takes the address whose
`node_id` matches, the LAN one first.
No Wi-Fi name is read.

**The server's page in the shell.**
The page asks `token()` once, at its start, hands a new session to `setToken`,
and keeps none in `localStorage`.
OPAQUE runs in the page as in a browser.
The event stream, the tickets, the preferences and the "only on this
device" settings work unchanged, the WebView's storage being per origin.
The `User-Agent` begins with the app and the phone,
so that the session's label under My profile names them.

**TLS and plain HTTP.**
A certificate from Let's Encrypt verifies by itself.
A self-signed one is pinned on first use: the page shows the fingerprint,
the user compares it with the server's log or `massimo tls show`,
`trust` stores it,
and a changed fingerprint blocks until it is trusted the same way.
Plain HTTP is allowed,
and the page warns for an address outside the private ranges.

### The core (Block 1)

`app/core` is a Go package in the root module (it imports `internal/serve`,
which a module of its own could not),
bound by `gomobile bind` (golang.org/x/mobile, BSD-3-Clause,
newest of 2026-10-11, `go 1.26.0`; the analysis, section 2):

- `Start(dataDir, listen, optionsJSON string) error`:
  opens the store in `dataDir`,
  starts the server on `listen` (`127.0.0.1:8099` by default,
  so that an open server is reachable by the page alone),
  no proxy and no announcement unless the options ask;
- `Stop()`, `Status() string` (JSON: running, the address, the Cubes,
  the version);
- an `Events` interface the adapter implements: `OnStatus(json)`,
  `OnNotification(json)`, the latter AP ServerAnnouncements' local deliverer.

Bound types stay within gomobile's: strings, ints, bools,
an interface of such methods; no slices but `[]byte`, no maps;
a panic across the boundary ends the process,
so every entry point recovers and returns an error.
`go.mod` keeps `golang.org/x/mobile` through a `tool` directive,
since the bound package does not import it but `gomobile bind` needs it in the graph.
The Makefile targets: `app-core-android` (`-target android -androidapi 26`,
the `.aar` into `app/android/core/`, which is gitignored),
`app-core-ios` (`-target ios,iossimulator`,
the XCFramework into `app/ios/Frameworks/`), both with `-trimpath`,
`-buildvcs=false` and `-ldflags=-buildid=` for reproducibility,
both needing the NDK or Xcode, which the targets check for and name.
A test, `app/core/core_test.go`, starts the core over a temporary data
directory against the fake Cube's greeting, reads `/healthz`, stops it:
plain `go test`, no gomobile.
Both targets depend on `ui`, as every target that compiles Go does:
the server embeds the web UI's bundle, which `make ui` builds.

### The connect page (Block 2)

Done: `ui/connect.html` and `connect.js`,
in the server's embed and bundled by the shell from the same sources,
with the contract in `docs/app.md` and the UI suite's `test_connect.py` over a
fake bridge that Playwright puts into the page.
What this revision changes on the page is Block 8's (the kept credentials)
and Block 12's (the toasts and the names).

### The Android shell (Blocks 3, 7 and 11)

`app/android`:
a Gradle project (Kotlin, `compileSdk 37` since D88-A, `targetSdk 36`, `minSdk 26`),
one module `app`, flavours `foss` and `play`,
the `ui/` files copied into its assets by a Gradle task,
never a second copy in git.

- **Client mode (Block 3), done**:
  `MainActivity` with the WebView and `WebViewAssetLoader` for the bundled page;
  `Bridge` over `addWebMessageListener` with `res/raw/bridge.js` injected at the
  document's start, and `Origins` checking every message; `ServerStore`;
  `Discovery` over `NsdManager`; `Prober` with the pinned trust manager;
  `TokenStore` under `KeystoreSealer`, an AES-GCM key of the Android Keystore;
  `Biometric` over `androidx.biometric`; the icon from Massimo's mark.
  The phone test of 2026-11-17 passed it;
  its one fault, the asset path, is fixed.
- **Stand-alone mode (Block 7)**:
  `CoreService`, a foreground service of type `connectedDevice` (for
  "interactions with external devices that require a … network connection";
  no six-hour limit, may start from `BOOT_COMPLETED`;
  its prerequisite met by declaring `CHANGE_WIFI_MULTICAST_STATE`) with a
  persistent notification; the core's data directory under the app's files;
  a `WifiLock`;
  the battery-optimization exemption asked for only when the user turns the server on,
  with the reason shown; an optional start at boot;
  "this phone is the server now: the Cube accepts one client" said in words;
  the LAN listener and the proxy off, a switch to let other devices connect,
  which requires a first owner as the server does on the LAN.
  The analysis, section 2, has Android's rules with their sources.
- **Push (Block 11)**:
  `org.unifiedpush.android:connector` in both flavours,
  the `embedded_fcm_distributor` in `play` only (a Firebase project and its
  `google-services.json` in the `play` source set, kept out of `foss`);
  the endpoint and keys posted to the server through the bridge (`POST /api/v1/me/push`);
  a received message rendered from the app's strings per kind into a notification channel,
  a tap opening the server's page;
  the embedded core's `OnNotification` into the same channel.

### The login by fingerprint (Block 8)

**What is there.**
A server's page hands its session to the adapter (`setToken`), which keeps it
sealed under a Keystore key and gives it back at the next start (`token`).
A server's switch "Ask for my fingerprint or PIN" puts a prompt before that:
the app's logic asks, the key itself is not bound to it.
The session lasts 30 days from its last use.

**The owner's design**, stated at the phone test of 2026-11-17
in place of the switch that guarded the session, in short:
the sign-in form offers to enable the login by fingerprint;
after a sign-in that succeeded the app asks for the fingerprint,
makes a new token on the server and keeps it behind the fingerprint,
or offers to repeat, or to sign in without keeping anything;
a logout ends the current session alone;
the sign-in form then offers the password or a button "Login by fingerprint",
which makes a new session from the kept token;
"Select server" leads back to the connect page;
a server's settings turn it off and delete what is kept,
removing the server does too, and "Forget all stored credentials" deletes all;
the server list marks a server with kept credentials,
and a fingerprint button on its row skips the sign-in form;
kept and matched are the server's address and port,
the user's name and the token, all three.

**The kept token is an API key of the user's own**
(D22-A).
Every user may make one for itself today (`POST /api/v1/users/{name}/tokens`,
"the same rules of access as the user"); it never expires,
its label is the phone's model and "login by fingerprint",
and My profile → Sessions and keys lists and revokes it.

**The server gains one route**, `POST /api/v1/login/key`:
the bearer is an API key, the optional body a `label`,
and the answer the session `POST /api/v1/login` answers:
`token`, `expires_at`, the user, the role, the capabilities.
A session as the bearer is `400`; a wrong or revoked key `401`,
counted as every failed token check is; a disabled user or role `403`.
It works where `auth.password_login` is off, since it sends no password.
Its audit kind is `login.key`; it makes no command.

**A session names its key**
(D31-A).
A session is extended at each use, 30 days from the last:
without a tie, a revoked key's session would live as long as the phone uses it.
`tokens` gains `parent_id`, the row of the key a session was made by,
empty for a session of the sign-in form,
as `commands.parent_id` names a command's parent;
migration 0025 adds the column, and nothing is rebuilt.
`origin` could not carry it: it is in every table,
the id of the node that made the last change, for the sync.
`RevokeToken` of a key revokes its live sessions with it,
and what they subscribed to push;
the new route revokes the key's earlier session when it makes the next,
so a phone holds one live session per kept key.
The list under My profile shows a session's key by its label.
A password set by someone else ends a user's sessions and leaves the keys,
the phone's among them, as it does for every API key today.

**The adapter holds the key, the page never does**
(D23-A).
On a server's page the bridge gains, in version 2 of the contract:

| Call | What the adapter does |
|---|---|
| `login.status()` | whether the phone can (a strong biometric enrolled), whether credentials are kept for this address, with the user's name, and when the fingerprint is asked |
| `login.keep(user)` | asks for the fingerprint; makes the key at the server with the page's session and seals it; resolves `false` where the prompt was not passed |
| `login.fingerprint()` | asks for the fingerprint; opens the key, makes a session at the server with it and resolves to that session; a key the server refuses is forgotten |
| `login.forget()` | not built: no page needs it, Turn off being the connect page's `forget(id, url)` at the address the probe chose |
| `connect()` | shows the connect page: "Select server" |
| `serverNames()`, `switchTo(id)` | not built (D32-B): a server's page learns nothing of the other servers |

The sign-in form asks `login.status()` when it is drawn,
so it knows whether the login by fingerprint is there before the user chooses.
On the connect page a server carries `login`,
the user whose key is kept or `""`, in place of `biometric`, and `ask`,
one of `return`, `start` and `logout`;
`forget(id)` and `forgetAll()` delete;
`open(id, url, {login: true})` opens the page to sign in at once.
The adapter makes its two requests itself, over the connection `Prober` pins:
the key's creation and the session's.

**The Keystore key requires the prompt for every use**
(D25-A): `setUserAuthenticationRequired`, a class 3 biometric,
with the screen lock as the prompt's fallback from Android 11 (a prompt with a
crypto object takes no device credential before it,
and no class 2 biometric at all; androidx's reference, read 2026-11-17).
Sealing needs the prompt as opening does,
so "ask for the fingerprint, then keep" is one step.
On Android 8 to 10 a fingerprint enrolled later invalidates the key:
what was kept is dropped, and the form offers the password;
from Android 11 the key, which admits the screen lock, survives it,
and turning the screen lock off invalidates it on every version (D35-A).
What is sealed is bound to the server's id, its node, its pinned fingerprint
and the user's name (D34-B), so that it opens for nothing else.
Built with one Keystore key per server;
Block 17 puts one wrapping key for the app in its place (D63-A).

**The flows.**

- *Enabling*:
  the form's switch "Enable login by fingerprint",
  shown where `login.status()` says the phone can; after the sign-in,
  `login.keep`; where it fails, "Try again" or "Sign in without it".
- *When the fingerprint is asked*
  (D24-C, D30-A): the user's choice per server,
  in its entry on the connect page;
  a server without kept credentials keeps its session as today.
  - `return`, each time the app comes back from being hidden:
    the session is in memory alone;
    the adapter covers the page when the app is hidden,
    and the prompt uncovers it, the session and the page as they were,
    so nothing is loaded again and no draft is lost.
    The app's own dialogs, the file chooser and the prompt itself,
    do not count as hidden.
    Android may end a hidden app: the return is then a start.
  - `start`, when the app starts, the choice until the user makes one:
    the session is in memory alone,
    so a start shows the form with "Login by fingerprint",
    or signs in at once from the row's button, and makes a new session.
  - `logout`, only after a logout:
    the session is kept sealed on the phone as today,
    and the fingerprint is asked on the sign-in form alone,
    after a logout or once the session ended.
- *Logout*:
  `POST /api/v1/logout` as today, the session's end; the key stays,
  and the form offers both ways.
- *Select server*
  (D26-A): the sign-in form's button, and in the app the Cube choice's list,
  the `Picker` of `src/header/Choice.tsx`,
  which the part draws for one Cube too: after the Cubes and the search,
  "Select server", which shows the connect page.
  With D32-A it heads a group: the stored servers by name,
  the one shown marked, and "All servers…" for the connect page.
  A browser's page has no bridge and shows none of it.
- *Turning it off* in a server's settings asks for the fingerprint once,
  revokes the key at the server and forgets it;
  where the prompt or the server fails, the phone forgets it alone.
  "Forget all stored credentials" and Remove forget without a prompt.
  A key only forgotten stays listed at its server under the phone's name,
  to revoke there.
- *The old switch* goes: a server that had it on starts without kept
  credentials and with its session forgotten.

### Release and the stores (Blocks 4, 5, 6)

**The pipeline as it is**
(AP Channels, after v1.0): a release is cut with `make release <flavour>`,
which commits the store's dump,
has `changelog.sh cut` make the release commit and the signed tag,
and pushes `main` and the tag; CI tests the tag,
`build.yml` builds every file without a secret and attests it,
and `release.yml` signs and publishes in the environment `signing`;
`release/devel` and `release/nightly` are rolling builds of `main`.
The ruleset requires the checks "All tests passed" and Guidelines.

**CI, as built**
(Block 4): the job "Android app" of `ci.yml`,
run where "What to test" answers `app`, and reported by "All tests passed":
the version file checked, the debug build with its unit tests and lint,
the release APK built at two paths and compared byte for byte,
then the AAB and the nightly APK,
all three signed with a key made for the run and checked against its certificate.
With Block 1 the job gains `make ui` and `make app-core-android` first.

**The release, as built**
(Block 4): `build.yml` builds a version tag's `foss` APK and `play` AAB,
and a nightly build's APK, unsigned and attested;
`release.yml` signs them in `signing` ("Sign the Android app"),
adds them to the release's `SHA256SUMS` with its `SHA256SUMS.sig`
("Publish the Android app") and attests the signed files.
The nightly app, `ch.metaworx.massimo.nightly`,
installs beside the release's and is signed inside `rolling`,
with a key of its own.
Each signing compares the signer with the certificates under `app/android/signing/`.
The signed APK is the `Binaries` URL F-Droid compares its rebuild with,
and `build.yml`'s unsigned one what a rebuild is compared with byte for byte.
The version is the tag's (D3-A): `app/version.json`,
the changelog's manifest, its `version` written by `changelog.sh cut`
and its `versionCode` by `make release`,
`MMmmPPbbbb` (D21-A, `packaging/app-version.sh`);
Gradle reads it, and F-Droid's update check reads it at the tag.
The nightly's path ran first on 2026-11-18 (Block 4, "As built");
not seen yet: a version tag's signing,
which runs at the owner's next `make release`.
Dependabot reads the app's Gradle files since Block 12 (D29-A),
in two groups since D88-A: the libraries, and the wrapper with the
Android Gradle Plugin.

**F-Droid** (the analysis, section 4): the recipe in `fdroiddata`,
`metadata/ch.metaworx.massimo.yml` under D1-A: AGPL-3.0-or-later,
the repository, `Binaries` and `AllowedAPKSigningKeys` (our key's SHA-256),
`AutoUpdateMode: Version`,
`UpdateCheckMode: Tags` with `UpdateCheckData` on `app/version.json`;
the build entry with `subdir`, `sudo` for a bootstrap Go,
`srclibs` building the pinned Go from source, `sdkmanager` for the pinned NDK,
gomobile installed from the module cache, the core built in `prebuild`,
`gradle: [foss]`, the module cache `scanignore`d:
the shape of sing-box's and syncthing-android's recipes.
Submitted as a Request For Packaging once the first tagged release carries the APK;
the review takes weeks.
Until Block 7 builds the core into the app the recipe is Gradle alone;
with the core, `make ui` runs before `gomobile bind`.
The owner's steps: a Google Play Console account also registers the app for
Android developer verification (from 2027 every certified Android phone installs
only apps of a verified developer, F-Droid's included; $25 once;
a personal account); the package name registered with the key's fingerprint;
a lost key cannot be re-registered.

**Google Play** (the analysis, section 5): the owner's personal account,
identity verified; the app created under D1-A;
Play App Signing with our own key uploaded before the first open-testing
or production release; the listing in English and German at least,
screenshots from the UI suite's set; the Data safety form,
"no data collected" (every byte goes to a server the user chose);
the foreground-service declaration for Block 7; target API 36;
internal testing at once for the owner's phones,
then the closed test with 12 testers opted in for 14 days,
which a personal account created after 2023-11-13 must run, then production.
The tracks, as proposed at Block 4's gate, where the owner asked how they work,
for Block 6's gate to decide
(a test is a track of the one app, with the same id and key,
chosen at each upload, a build promotable later):
an alpha to the internal test,
a beta and an rc to the closed test and later the open one,
a stable release to production, the 14-day closed test run on an alpha.
The AGPL is no issue on Play; the listing names the licence and the source.

### The iOS shell (Block 9)

`app/ios`:
an Xcode project with Swift Package Manager, no CocoaPods;
`WKWebView` with the bridge of the same contract,
a `WKUserScript` over a `WKScriptMessageHandlerWithReply` (`docs/app.md`);
the bundled `ui/` served by a `WKURLSchemeHandler`; `NWBrowser` for discovery;
`URLSession` probes with pinning in the authentication challenge;
the Keychain and `LocalAuthentication`, the kept key of Block 8 under an
access control that asks for Face ID or Touch ID;
the ATS keys `NSAllowsLocalNetworking` and `NSAllowsArbitraryLoadsInWebContent`;
the local-network usage description and `NSBonjourServices`;
the XCFramework from `make app-core-ios`;
the stand-alone mode while the app is open (iOS suspends an app about 30 s after
it leaves the foreground, so no 24/7 and no push from it);
no push in the first release unless a relay is decided.
The App Store: the owner enrolls as an individual ($99 a year;
a sole proprietor must), the seller name the owner's legal name;
an additional permission to the AGPL for App Store distribution,
which the owner can grant as the copyright holder (the analysis, section 1),
in `app/ios/LICENSE-EXCEPTION.md` and the listing;
guideline 4.2 is met by what the shell adds over a web page (discovery,
biometrics, the embedded server, pinning).
CI: a job on the macOS runner, `xcodebuild` for the simulator, no signing.

### The phone test's findings (Block 12)

From the phone test of 2026-11-17 (Block 3, "As built"),
what is neither the login nor the asset path fixed in the test:

- the connect page's messages ("added", "removed", "does not answer at its
  addresses") stay, the last even after a probe finds the server;
  the owner wants toasts, `showToast` of `components.js`,
  which the page loads, with the markup of `partials/toast.html`;
- a server added by its address is named by it,
  so its row shows `host:port` twice;
- the web UI's notice for a self-signed certificate ("compare the SHA-256
  fingerprint in your browser's certificate details") shows in the app,
  on the sign-in form (`login-tls`) and in the header (`src/header/Tls.tsx`,
  `header.js`), although the app pinned the fingerprint the user compared;
  the warnings of a certificate near its end stay;
- for `app/README.md`: the test's rig (a `serve -tls self-signed` against the
  fake Cube in WSL, `adb reverse`,
  the WebView's DevTools port of a debug build),
  a phone's VPN that does not allow LAN connections and so reaches no LAN server,
  and `adb reverse` stalling on one port.

### The phone check's findings (Block 13)

The owner's check of the login by fingerprint on 2026-11-20 passed:
the first owner made from the phone; enabling it, a cancelled prompt and Try again;
the key under My profile, the session made from it naming it;
the login after a logout; the three choices with the cover and the restart;
the PIN; "Select server" from the card; a revoked key ending its session at once;
a certificate changed while signed in, trusted again.
It ran on the `foss` debug build of `main` with Blocks 8 and 12,
installed over the one of 2026-11-17, its servers kept,
against a `serve -tls self-signed` with the fake Cube in WSL
reached through `adb reverse`;
what got in the way was none of it the app's:
WSL's reboot, which took the first rig and its store from the session's scratch
(the rig has lived in the project's own scratch since, with a script that
clears the address lockout at each start),
the wireless debugging link's drops, which a watcher reconnected every ten seconds,
and a Windows `adb.exe`, started from WSL to look,
which took the phone as a second host and put WSL's link offline.
F1 fell into that hour and went as it should when made on purpose;
F18 did not come back.
The two lockouts were the page's own (F7):
after the owner revoked its session, and after revoking its key,
the page sent the dead session with its polls and tickets,
ten failed checks within a minute,
every connection through `adb reverse` coming from `127.0.0.1`.
What it found, by where it is fixed:

| # | Finding | Fix |
|---|---|---|
| F7 | after a 401 the page keeps its dead session; its polls and tickets lock its own address out (seen twice) | the page drops the token at the first 401 and the app forgets it; what follows carries none and counts as nothing |
| F2 | My profile does not mark what is in use | D41-A; Revoke on "this device" signs out |
| F8 | the signed-out page shows the header | D44-A |
| F9, F11 | the switch's label below it; too little space under the card's logo | the label right of the switch; more margin |
| F10, F12 | "Or an API key" | D45-A, D49 |
| F4 | no switch on the first-owner form | D42-A |
| F13 | "Select server" only at the card's foot | also last in the card's ☰, and in the main ☰ before Sign out |
| F14 | the open menus hardly stand out in dark mode | D48-A, D50-A, every menu |
| F3 | the fingerprint icon's top lines run together | D40 |
| F21, F16 | Forget all under the list | a ☰ on the Servers card: Add server…, Sign out of all, Forget all stored credentials, App settings… |
| F22 | the Add a server card always shows | only for an empty list or from the ☰, with ✕; closed once a server is added; its search when it opens |
| F17 | a server's settings focus its name, and the keyboard covers half the screen | the panel takes the focus |
| F5 | no server opens at start | D43-A |
| F15 | the list does not show where the app is signed in | D46-A |
| F6 | Turn off sent no revoke and kept the session | sign out; revoke with the session, the key's id kept in clear; with the key after the prompt where no session is held; a failed revoke in the app's log |
| F23 | a server's certificate is not shown | D51-B |
| F24 | the WebView reported a light scheme while the phone was dark | D53-A, the theme's `android:isLightTheme` set for day and night |
| F19 | a revoked key's login met the lockout | F7 |
| F1, F18 | a changed certificate not opening; "after a logout" asking again after a restart | not reproduced: the first fell into the rig's outage |

The bridge's contract becomes version 3 (`docs/app.md`):
a server's `start` and `session` (the user of a session the app holds,
checked), `trusted_at`; a probe's `subject`, `issuer`, `names`,
`not_before` and `not_after` for HTTPS;
the calls `session(id, url)`, `signOut(id, url)`, `signOutAll()`,
`settings()` and `saveSettings(s)` on the connect page;
on a server's page, a dead session handed to `setToken(null)`.

### The second phone check's findings (Block 14)

The owner's check of Block 13 on 2026-11-20 passed, steps 1 to 10 and 12 to 15:
the Servers card's ☰, the Add card on demand with ×,
a server's settings without the keyboard, Open at start,
also after the app was swiped away,
the certificate's line and dialog;
the card without the header, the version at its foot and in its ☰,
the fingerprint first in the row, Select server in both ☰;
"this device" and "its key", the menus' border and glow;
the session's badge, Sign out and Sign out of all (each session's `logout`
in the audit); Turn off without a session, the prompt and the key revoked by itself;
each Appearance, the list and the server's page following it.
Step 11, a session revoked from another browser, was skipped,
the UI suite covering it (`test_a_dead_session_is_dropped`);
no address was locked out, and the app's log had no warning.
What it found:

| # | Finding | Fix |
|---|---|---|
| F25 | the fingerprint button's spinner pushes its icon out | the spinner in the icon's place while the login runs |
| F26 | Forget all forgets the kept logins on the phone alone; their keys stay valid | D56-A, D57-A |
| F27 | the sign-in card: not centred, nor its title; the "Sign in with" label; the ☰ apart from the row; "Name"; "Select server" at its foot | centred; no label; the ☰ at the row's end; D55-A; D54-C |
| F28 | Turn off while signed in: the audit shows the sign-out 14 s before the key revoked itself, no revoke by the session | a test of that path, its cause traced; a login kept without its key's id finds it through its session; step 12 again on the phone |

Revoking needs no secret:
the app keeps each key's id in clear beside the sealed key (Block 13),
and `DELETE /api/v1/users/{name}/tokens/{id}` takes any session of that user.
A key Turn off or Forget all could not revoke
therefore stays as its server, user and id until a session there revokes it.
The server keeps the key's SHA-256 (`store.HashToken`),
which the app needs neither keep nor show.

With Open at start the sign-in card is the app's first page,
where the back gesture closes the app;
Cancel is then the visible way to the list.

The bridge's contract becomes version 4 (`docs/app.md`):
a server's `unrevoked`, the keys left valid on it, each `{user, id}`;
`forgetAll`'s answer, what it revoked and where it could not;
and the way to open a server's page at My profile with a key marked.

### The phone check of Block 14 (Blocks 15 to 18)

The owner's check of Block 14 on 2026-11-21 passed,
on the `foss` debug build of `main` before metaworx/massimo#397,
against two rig servers through `adb reverse`:
A, a `serve -tls self-signed` against the fake Cube, its store kept,
and B, one with no Cube and at first no users:
the sign-in card; a key left, named in the server's settings,
and revoked within a second by the next sign-in as its user;
Remove with a kept login; Turn off while signed in,
the key revoked by the session and then the session's logout,
which settles F28; the cover at a return.
Forget all ran with neither server holding a session,
and asked twice, each login having a Keystore key of its own (D63-A).
Once, after an `adb reverse` connection stalled,
the WebView sent requests again with the session that had ended,
and the rig's server locked the phone's address out (D64-A).
What it found, by where it is fixed:

| # | Finding | Fix |
|---|---|---|
| F29 | the card's place flickers between Password and API key | D60-A, on `main` (metaworx/massimo#397); the look below |
| F30 | the first owner's form is not mandatory, and sits beside the sign-in | D61, AP FirstOwner |
| F31 | the card does not say which server it signs in to; the app has no address bar | D62-A, Block 15 |
| F32 | a request that never answers leaves Revoke spinning | D82-A, Block 16 |
| F33 | Show in My profile is on while the server is unreachable, and fails | off while there is no connection, Block 15 |
| F34 | Remove sits beside Cancel and Save | on a line of its own, Block 15 |
| F35 | the key kept after a password sign-in shows "never used", no tag | D65-A, D66-A, Block 16 |
| F36 | a second sign-in card after a sign-in that revoked a key left; the first session left valid | D68-A, Block 16 |
| F37 | Show in My profile marked a key the sign-in had revoked, and its Revoke failed | D67-A, Block 16 |
| F38 | Remove without a kept login forgets the session, still valid at the server | D69-A, Block 16 |
| F39 | Turn off, Forget all or Remove while the server is down leave a password session valid, recorded nowhere | D70-A, Block 16 |
| F40 | asked at each return, a server opened from the list enters without the fingerprint | D72-B, Block 15 |

The look at a build with metaworx/massimo#397,
offered for after the check (D60-A on the phone,
and the password manager's autofill with the hidden password form):

was not done (D83-A): its two checks go into Block 15's phone check,
the card having changed since
(AP FirstOwner, metaworx/massimo#401 to #403, and AP UiParts' last block, #404).

**Opening from the list** (D72-B, D74-A; Block 15).
Today a tap opens the page (`open(id, url)`, `connect.js` `serverRow`),
which asks `token()`:
a session the app holds in memory enters at once (F40),
and only the row's fingerprint button (`{login: true}`) asks first.
The cover asks only when the app comes back from being hidden
(`MainActivity.onStart`).
The two decisions, with D24-C's three choices, make one rule,
which the bridge applies, so that no page opens around it:

| Login kept | Asked | Session held | A tap |
|---|---|---|---|
| none | | either | opens the page, with its session or at its card, as today |
| kept | at each return | in memory | the prompt, then the page with that session (D72-B) |
| kept | at each start, or after a logout | yes | opens the page with it: the fingerprint was asked in this run, or is asked after a logout |
| kept | any | none | the prompt with the key, then the page with a new session (D74-A) |

A prompt cancelled or failed opens the page at its sign-in card:
the bridge withholds the session held for that opening (`token()` answers `null`),
so that the card's fingerprint button, the password and the key are there.
The sign-in icon opens the card so, without a prompt.
A session made there replaces the one withheld,
which Block 16's rule signs out (D68-A); hence 16 before 15.
Open at start is not a tap from the list and stays as it is,
the card with its fingerprint button: D74-A does not name it.
The rule is a pure function with a JVM test of every row (D71-A);
the prompt and the cancel are the emulator suite's (Block 18).

**The server under the card's title, and two entries of ☰**
(D62-A, D73-A; Block 15).
The card is AP UiParts' part (`src/signin/Card.tsx`),
which reads the app through `login.status()` at each opening
(`readApp`, `src/signin/app.ts`);
the server's line comes the same way:
`login.status()` gains `server`, the server's name, the address the page was
opened at, its `node` (the instance id of `/healthz`)
and its pinned fingerprint;
the line shows the name and the full address, the ⓘ the other two,
which the bridge has (`Bridge.Opened`, `ServerStore`).
A browser shows none, its address bar naming the server.
The two entries of ☰ (`src/chrome/Menu.tsx`, after "Select server…")
call two new calls of the server's page,
which show the connect page with that server's settings open, or App settings;
the bridge fills the server's id in,
and the page reads and saves nothing of them (D73-A).
AP FirstOwner's Blocks 2 to 4 and AP UiParts' last block change `login.js`,
the card and ☰ now; the session "UI refactor" is told before Block 15 starts.

**The second card** (F36, D68-A; Block 16).
Traced in the code, likely but not proven when D68-A was taken,
and proven since by Block 16's step 1, which failed on `main` in the four cases:
a poll sent without a token while the card is open meets its 401 after
`login/finish` has answered, once the card has closed;
`dropToken("")` forgets nothing (`app.js`), but `api()` still calls `askLogin`,
which opens a fresh card, the first one's promise being gone (`login.js`);
the second sign-in's `setToken` replaces the first session,
and nothing signs that one out.
A UI test that holds one tokenless 401 back until `login/finish` has answered
shows it, and must fail on `main` before the fix.
The fix compares the token a request carried with the one held now:
a 401 of another token, or of none, while the page holds one,
opens no card, a read being sent again and a write failing as not sent
(AP SignInResend's rule); a 401 of the token held stays as it is.
A session replaced is signed out once:
by the page's `setToken` in a browser,
and in the app by the bridge's `setToken`,
which holds what the page was handed and what `login.fingerprint` made
(`Sessions.put`);
a key replaced is not, since `POST /api/v1/logout` refuses a key.

**My profile's keys** (F35, F37, D65-A to D67-A; Block 16).
The row (`TokenRow`, `src/shared/tokens.tsx`,
one part for My profile and System settings → Users)
tags "this device" and "its key" from `GET /api/v1/me`'s token (D41-A),
so a key kept after a password sign-in, whose session is the password's,
has no tag.
The phone's key is the bridge's to name:
`login.status()` gains `key_id`, which the row compares;
the app's status moves from the card's `src/signin/app.ts` to
`src/shared/app.ts`, since a shared part knows no region
(`TestSharedPartsKnowNoRegion`).
A server's settings name the key's id from a server's new `login_key`.
F37: the bridge answers `setToken` before it revokes the keys left
(`Bridge.kt`, `Sessions.check` after the answer),
so My profile read a list that still had the key;
the bridge answers once they are revoked, with their ids,
and My profile's read of its list waits for that answer.
The row's id makes the row longer at 390 px,
which the UI suite measures in the six languages.

**Sessions the app forgot** (F38, F39, D69-A, D70-A; Block 16).
Remove forgets the session of a server without a kept login
(`sessions.forget` in `Bridge.kt` `remove`), not `signOut`;
Turn off, Forget all, Sign out and Sign out of all sign out,
but where the server does not answer the session is forgotten on the phone alone
(`Sessions.signOut`, `end`),
valid at the server until it ends by itself, 30 days after its last use.
A session ends by its own secret,
or, as a key left does, by its id with another session of its user
(`DELETE /api/v1/users/{name}/tokens/{id}`,
which any session of the user may call).
The app keeps no secret it was asked to forget:
it notes the session's id, which its last check found
(`GET /api/v1/me`'s `token.id`), in the server's `unended`, beside `unrevoked`,
and the next session it holds there for that user revokes it,
as it revokes a key left.
Remove, whose server goes with it, names the session in its answer,
as it names the keys left.
A session whose id was never learnt is forgotten on the phone alone,
as a key without one is.

**The time limit** (F32, D82-A; Block 16).
`apiRequest` (`app.js`) waits as long as the connection does,
so a request a stalled link never answers leaves its button busy.
The limit is about 20 s;
a route that waits longer by design passes its own:
a device's or a room's write waits for the Cube's answer,
up to two minutes when the radio budget is exhausted,
and the Cube search up to about 30 s while another program holds the discovery
port (`docs/api-v1.md`).
A write that went unanswered may still have been done, and its text says so.
It is shared code under every part, so the session "UI refactor" is told.

**A key's own revoke** (D80; Block 16).
The app revokes with the key alone where it holds no session
(`Sessions.revokeWithKey`, `ServerApi.revoke`):
`DELETE …/tokens/{id}` with the key as the bearer,
the user's name and the key's id kept beside the sealed key.
It sends `DELETE /api/v1/me/token` (D80-B),
and falls back to the old way at a server that answers `404` or `405`,
one older than the route,
which an app meets wherever a server is not updated yet.

**One prompt for Forget all** (D63-A; Block 17).
Block 8 made a Keystore key per server (`LoginKey.alias(id)`),
AES-256-GCM, every use behind the prompt:
`setUserAuthenticationParameters(0, AUTH_BIOMETRIC_STRONG | AUTH_DEVICE_CREDENTIAL)`
from Android 11, the strong biometric alone before (`LoginKey.kt`).
A validity of 0 lets a passed prompt authorise one operation of the one cipher
it was shown with (`Biometric.withKey`, a `CryptoObject`),
so a prompt opens one server's key, and Forget all asks once per server
where no session is held (`Revoker.forgetAll`, `Bridge.kt` `byKey`).
One prompt opens several only where one operation reaches them all.
The ways, settled from the code:

- **A (recommended), an app key under one wrapping key**, D63-A's form.
  One Keystore key, `massimo.login.app`, bound as today,
  seals a random 256-bit app key;
  each server's login is sealed under the app key in software,
  AES-GCM with today's associated data
  (the server's id, node and pinned fingerprint, and the user:
  `LoginStore.aad`),
  so that each still opens for its server alone.
  A prompt opens the app key for one step,
  keeping a login, opening one, turning one off, or Forget all,
  which opens every login it needs with it; the step drops it.
  Keeping a login still asks once,
  opening the app key, or making it the first time.
  Each server keeps its own `ask` (D24-C, D47-A):
  when the app asks is its own logic, as it is today
  (`SessionKeeper.memoryFor`, the cover, Block 15's rule),
  not the Keystore's.
  Its cost: the Keystore no longer keeps the servers apart.
  A prompt passed for one server gives, for its step,
  what opens every server's login, held in the app's memory,
  where code running in the app at that moment
  (a debugger on a debug build, a flaw in the app's own code)
  finds every kept key, not one.
  Every opening still needs a prompt, and a copy of the files opens nothing.
- **B, one sealed bundle**: every login in one blob under one Keystore key.
  One prompt opens all; but keeping or forgetting one rewrites the bundle,
  an opening and a sealing, two prompts for a keep.
- **C, keys with a validity**:
  each server's key kept, valid for some seconds after any passed prompt
  (`setUserAuthenticationParameters(T, …)`).
  One prompt opens them all within T;
  but such a key opens after the phone's own unlock too,
  without the app's prompt,
  and on Android 8 to 10 a new fingerprint no longer invalidates it
  (D35-A's rule holds for a key of validity 0 alone).
  Nor is it one wrapping key.

A follows D63-A and needs no decision of its own.
A login kept before Block 17 keeps its own key and its own prompt
until it is turned off, forgotten or kept anew,
so Forget all asks once for the app key and once per such login;
nothing is moved at the update, which would take a prompt per login.
A key the phone takes back (a fingerprint enrolled on Android 8 to 10,
the screen lock turned off) is now the one key:
every login opens no more at once, each key left by its id (D57-A),
as each went one by one before.

**The lockout** (D64-A; Block 17).
`authorize` counts every refused token a request offered
(`internal/server/auth.go`, its `s.limiter.fail`),
a revoked or expired one included,
since `TokenByHash` finds live rows alone (`internal/store/tokens.go`):
ten within a minute lock the address out for five minutes
(`DefaultAuthFailures`, `DefaultAuthWindow`, `DefaultAuthLockout`).
The page drops a dead token at its first 401 (F7, Block 13),
but a WebView that sends again what a stalled link held
locked the rig's own address out at the check.
A token the server issued and that ended is not a guess:
a guessed secret never hits its hash.
So a refused token is looked up among the ended,
revoked, past its end, or a session whose key was revoked,
and such a refusal is still `401`, uncounted;
an unknown token counts as before, and so does a sign-in's wrong password.
The cost is one more read of the store per refused token.

**The emulator** (D71-A; Block 18).
The one instrumented test, `ConnectPageTest.kt`, is run by hand on a phone
(`app/README.md`).
An emulator runs the WebView, the bridge and the Keystore as a phone does,
the fingerprint simulated by its console's `adb emu finger touch`,
which a test inside the emulator cannot send:
the make target's helper answers each prompt a test asks for in the log.
Its server is a `serve` on the host against the fake Cube,
reached at `127.0.0.1` through `adb reverse`, as the phone rig reaches one.
In CI, beside the job "Android app", where What to test answers `app`,
on a runner with KVM.
Its result goes into the Tested record's `app=`,
which then means both jobs passed:
the record has no room for a new word (`GUIDELINES/project/CI.md`, section 2).
The phone check keeps what an emulator cannot show:
a real fingerprint sensor, the phone maker's autofill, the look.

**The bridge's contract becomes version 5** (`docs/app.md`),
Blocks 15 and 16 together, the first to land raising the number,
the page testing for each call and field it uses, not for the number:
`open`'s rule and `{signIn: true}`, a server's `login_key` and `unended`,
`remove`'s `sessions`; on a server's page `login.status()`'s `server` and
`key_id`, `setToken`'s answer `{revoked}`,
`openServerSettings()` and `openAppSettings()`.
Block 16 landed first and raised it.

### The phone check of Blocks 15 to 17 (Blocks 19 to 21)

The owner's check on 2026-11-25, after metaworx/massimo#418 and #420 had merged,
on the `foss` debug build of `main` of that day,
installed over the one before, the app's data kept,
against three rig servers, each a `serve -tls self-signed` against the fake Cube
reached through `adb reverse`:
A, its store kept from the earlier checks;
B, its store kept, with a login kept on the phone from before the update;
C, new, with an empty store, which showed the first owner's form.
The steps went in four batches; each server's audit was read after each,
and the app's warnings from the phone's log, where none came.
A was stopped, started and once frozen (`SIGSTOP`, then `SIGCONT`)
between the owner's steps.

- **Block 15**, its nine steps, passed:
  a tap's prompt with the PIN as its other way, its cancel into the card;
  the sign-in icon; a server asked at each return asked at both taps,
  and the cover after Home; the card's line and its ⓘ on C's first owner's form,
  the fingerprint as the rig's certificate has it; ☰'s two entries;
  Show in My profile off with A stopped; Remove server's line;
  D83-A's look, the card standing still and the password manager's autofill
  through API key and back; the eye on the card and the first owner's form.
- **Block 16**: a password sign-in where a key was left made one card and one
  session, twice; the login turned on at a password sign-in tagged its key
  "this device's key", every row with its id, the id in the server's settings;
  Remove without a kept login logged out at A before the server went;
  a Revoke while A was frozen ended after about 20 s, saying so, but out of view
  (F45).
  Show in My profile after a sign-in that revoked its key marked nothing,
  as it should, but showed no toast (F44).
  Turn off with A stopped, then a sign-in: held for minutes, the key and the
  session left were named and ended 0.28 s and 0.36 s after the sign-in;
  held for 16 s, the key was, the session not (F43).
  Turn off without a session revoked the key by itself, as the emulator's
  `SessionsTest` has it.
- **Block 17** passed: Forget all asked once over three servers,
  A's key revoking itself (`DELETE /api/v1/me/token`)
  and C's revoked by C's session, in one second;
  a login kept after the update turned off and on with one prompt each,
  and B's kept before it opened with its own;
  a session logged out and sent again 14 times within a minute,
  `401` each time, no `429`, no failure counted, the key still valid.

On 2026-11-27 (`EXEC phone F44`, D103-A's first part)
Block 16's step 3 ran again at A alone, on the same build,
the server's page read over the WebView's DevTools socket
(its navigations, the console, the API's statuses without a header or body,
and every 100 ms the fragment, the page, the card, the toast and the ids of
`setToken`'s last answer), which changed nothing of the page or the app:
a password sign-in with the login kept (session 75, key 76);
A stopped, Turn off, the key left;
A started, ↻, Show in My profile, which opened the card at `#profile?key=76`;
a password sign-in without keeping the login (session 77):
the app revoked key 76 and session 75 0.34 s and 0.43 s after it,
the bridge answered `{revoked: [76, 75]}`,
My profile showed once `/me` was read, the key left the address,
and "Key 76 is revoked already: …" showed 0.5 s after the sign-in,
at the screen's foot over My profile, for its 10 s; the owner saw it.
Session 75, held when A went down and Turn off ran,
was ended at the next sign-in: `unended` holds within one process.

| # | Finding | Cause, read in the code | Fix |
|---|---|---|---|
| F41 | the app sends `POST /api/v1/logout` with a session the server ended already, a refused `logout` in the audit (B three times, C once): after a second key login, and after a Turn off or Forget all that revoked the key; not counted (D64-A) | a key login's `Sessions.put` returns an `Ending` that logs out the session it replaced, which `POST /api/v1/login/key` ended when it made the new one (D31-A); `revokeWithSession` signs out after the key's revoke a session that, made from that key, ended with it | D100-A, Block 21 |
| F42 | with A down while a session was held, the list showed "not reachable" and "signed in as …" side by side, and the settings offered Sign out, which said nothing of what it does then | `fillRow`'s badge and `fillFacts`' line follow the last check (`s.session`), not the probe's state; Sign out without the server leaves the session in `unended` | D105-A, Block 21 |
| F43 | a session held when A went down, 16 s after its sign-in, was not put into `unended` and stayed valid; one held for minutes was | not settled from the log: a session no check has seen has no id, and `leave` then forgets it on the phone alone; a fingerprint tap's session is seen by no check until the list's next probe (`Bridge.kt` `openKept` puts it without `Sessions.look`); or the process ended between the sign-in and Turn off, as F47's did | D102-A, Block 21 |
| F44 | the toast `profile.keyRevokedAtSignIn` did not show | not reproduced on 2026-11-27, the whole path running as the code has it; of the two misses, the first ran its steps in the wrong order, no mark and so no toast, as designed; the second is not explained: the toast shows at the screen's foot over My profile, not on the card where the owner looked, and the phone's log has the password manager's sheet over that foot for 2 s of its 10 | nothing to fix; D103-A's two test cases, Block 21 |
| F45 | a failure under My profile (the Revoke after the hang) is said at the card's foot, out of view where the card is long | `profileMsg` (`src/profile/send.ts`) writes the card's line alone | D104-A, Block 20 |
| F46 | A, thawed, ran the Revoke the phone had given up on; its audit says `failed` with "too many password checks at once", for a route that checks no password; the key stayed valid, as it should | `userFail` (`internal/server/users_api.go`) hands every store error to `passwordBusy` first, which takes `context.Canceled` and `context.DeadlineExceeded` for a password check that waited until its client went away | D106-A, Block 21 |
| F47 | a password session held when the app was swiped away stayed valid, not put into `unended`, and three later sign-ins as that user did not end it | the process ended: the sessions kept in memory (`SessionKeeper.memory`) and what was checked of them (`Sessions.checked`) go with it, a swipe's and lmkd's alike, and `unended` covers only what one process forgets (Turn off, Forget all, Sign out, Sign out of all, a server down) | D102-A, Block 21 |
| F48 | in the seconds after A went down, the settings offered Show in My profile, whose tap showed the WebView's own "Web page not available" (`net::ERR_CONNECTION_CLOSED`) with the address; Back led to the list, which then said "not reachable"; and after Turn off with A stopped it had stayed on | `openServer` (`connect.js`), which a tap on the row and Show in My profile both call, opens at once where the list's last probe found the server, and probes first only where it did not; nothing probes again between a server's fall and the list's next look; and `MainActivity`'s `Client` has no `onReceivedError`, so a main frame that fails to load shows the WebView's page | D108-A, Block 19 |

Beside the findings the owner proposed what D90 to D99 decided,
and the points taken as written (the Discussion);
and on 2026-11-27 that pulling the list down refreshes it, as ↻ does.

**The sessions held** (F43, F47; D102-A; Block 21).
`MainActivity.onCreate` makes the `SessionKeeper` and the `Sessions` of each activity.
The keeper holds in memory alone the session of a server with a kept login
asked at each return or at each start (`SessionKeeper.memoryFor`),
and seals every other one in `TokenStore`, which outlives the process;
`Sessions.checked` holds what the last check found of each:
the user, the token's row, its kind and when it was made.
Both go when the process ends, by a swipe from the recent apps or by Android
ending a hidden app, and with the activity too:
the manifest's `configChanges` lists orientation to density but not `uiMode`
or `locale`, so a change of the phone's dark mode or language makes the activity
anew, as the app's own Appearance does (AppCompat's night mode, D53-A),
and every session in memory is lost on the phone while it stays valid at the server.
Adding `uiMode` to `configChanges` would keep AppCompat from applying a new night
mode; so the sessions move to the process instead,
a holder of the app's own `Application` that every activity takes.
What a process's end loses no holder keeps:
so each server records the session held there once a check has learnt its id,
its user, id and `created_at` and no secret (`held`, beside `unended`),
cleared where the session ends;
at the next process's start a `held` whose session is no longer held,
none in memory, moves into `unended`,
and the next session the app holds there for that user ends it (`KeysLeft`),
as it ends any session left.
A sealed session is still held after the process's end, and stays as it is.
A session in memory when the process ends stays valid at the server until then,
or until it ends by itself 30 days after its last use.
A fingerprint tap's session is checked at once, as `setToken` checks a page's,
one more `GET /api/v1/me`, so that its id is known before anything can forget it.
`Sessions.look` today swallows a server's failure and a broken connection
without a word; it logs them, as `end` does, without any secret.

**A logout the server refused** (F41; D100-A; Block 21).
The server ends a session itself in two ways the app causes:
`POST /api/v1/login/key` ends the key's earlier session when it makes the next
(D31-A), and a key's revoke ends its sessions with it.
The app then sends `POST /api/v1/logout` with a session already ended:
`openKept`'s `Sessions.put` ends the session it replaced after the answer,
and `revokeWithSession` signs out after the revoke.
`Sessions` learns which key a session was made from: `GET /api/v1/me`'s
`token.key_id` (`ServerApi.Me.keyId`), or the kept key's id where `login.fingerprint`
made it;
a session replaced by one made from the same key,
or made from the key just revoked, is dropped without the logout,
and goes to no `unended`.
A session the server ended in a way the app did not cause meets its `401`,
which forgets it with no logout sent, as today.

**The list while a server does not answer** (F42; D105-A; Block 21).
The row's badge "signed in as …" (`fillRow`, `fill-ok session-badge`)
and the settings' session line (`fillFacts`, `connect.sessionLine`) follow the
session's last check, kept while the probe finds the server at neither address.
The badge turns grey while the probe's state is neither `lan` nor `remote`,
and the line says what Sign out does then:
it ends the session there the next time the app reaches the server (D112-A;
the Analysis, "The gate of v1.5", "The words").

**A cancelled request at the server** (F46; D106-A; Block 21).
`passwordBusy` answers `503` "too many password checks at once" for the store's
`ErrBusy` and for a context that ended, which is right where a password check or
hash waited for its turn until its client went away:
`POST /api/v1/login` calls it on its check's error,
OPAQUE's start and finish on `ErrBusy`.
`userFail` calls it first for every store error of the users' routes,
so a token's revoke whose client gave up is audited as a password check.
The routes that check or hash a password through `userFail`
(a password set, a user added with one, a registration)
keep that answer, through a helper of their own;
`userFail` answers a context that ended with words of its own,
"the client went away before the answer", the audit's result `failed` as now.

**The list** (D90-A to D95-A, D108-A, the points taken as written; Block 19).
The list's ☰ is the Servers card's (`#servers-menu` in its head, `connect.html`),
its entries `initMenu` and `renderMenu`'s;
it moves into `header.connect-top`, beside the brand,
where a server's page has its ☰, and the card keeps ↻.
The page has no version of the app today, and no call that gives one:
the bridge gains `about()`, the build's `versionName`.
Exit is the adapter's (`finishAndRemoveTask`);
with D102-A its dropped sessions go to `unended` at once.
`.connect-main` is 640 px wide at most (`style.css`); a server's page's limit is 1100 px.
`renderServers` draws the servers in the order saved;
it sorts them by the name shown with `Intl.Collator` of the page's language,
`sensitivity: "base"` and `numeric: true`, the store's order left as it is.
Back is the adapter's `OnBackPressedCallback`, on while the WebView can go back,
which calls `web.goBack()`; the list clears the history once it has loaded
(`onPageFinished`), so Back from it leaves the app.
A menu or a panel that opens pushes a history entry of the page's own
(`history.pushState`, the address unchanged), so that Back reaches the page as
`popstate`, which closes it, and a panel with changes asks first (D93-A);
with nothing open there is no entry, and Back leaves the app as today.
A server's settings (`settingsPanel`) have Cancel and Save always on,
no ✕, and the facts (`fillFacts`) and the kept login above them;
D111-A splits them into three sections (the Analysis, "The gate of v1.5").
Store login reuses the keep a server's page asks for (`Bridge.kt` `keep`),
from the connect page and at the probe's address: a new call, `keep(id, url)`.
Pull down: the WebView in a `SwipeRefreshLayout`
(`androidx.swiperefreshlayout`, a library new to the app),
on while the list is shown at its top, which sends the page the event `refresh`.

**A page that fails to load** (F48, D108-A; Block 19).
The check D108-A names is, in the code, the same for a tap and for Show in My
profile: `openServer` and the last probe.
What failed is that the probe was older than the server's fall.
So Show in My profile probes first, as `openServer` does for a server its last
probe did not find, and opens only a server that answers now;
an action of the open settings that the server did not answer
(Turn off, Sign out, Remove leaving a key or a session) probes it again,
so that Show in My profile turns off with its title at once;
and whatever still opens a page whose main frame fails to load
(a tap in those seconds, Open at start, a page's own link)
comes back to the list:
`MainActivity`'s `Client` gains `onReceivedError` for the main frame of the opened
server's origin, which drops the opening and shows the list with
`#unreachable=<id>`, the bridge's handover as `#settings=<id>` is;
the list probes that server at once, and says it is not reachable with the
probe's reason, as a tap's failed probe does (`connect.unreachable`).
A tap keeps opening on the last probe (D109-A).

**The menu and the cards** (the points taken as written, D96-A to D98-A,
D104-A, D107-A; Block 20).
☰ (`src/chrome/Menu.tsx`) draws the version row, the pages, Personal settings,
Select server, This server's settings, App settings, Sign out,
rules after the version and around the settings pages.
The version row (`VersionRow`, `src/shared/server.tsx`) copies the whole version,
says so at the element, and leaves ☰ open.
`copied` (`components.js`), which every copy of the page and the parts goes
through (`Copy`, `CopyIcon` of `src/shared/copy.tsx`; `copyable`, `copyIcon`,
`copyEl`), draws the word at the element (`data-copied`) and tells a hidden live
region, `#copy-status`; the toast is `showToast`, beside it in `components.js`.
The card's server line (`ServerLine`, `src/signin/Server.tsx`) follows
`#login-title` (`src/signin/Card.tsx`), its facts each a `Copy` without an icon.
The first owner's text, `setup.text`, carries `massimo user add` inside the
sentence (`src/signin/Owner.tsx`).
`NewPassword` (`src/shared/password.tsx`) is two `PasswordField`s, each with its
eye; `newPasswordProblem` says `setup.mismatch` at the submit alone;
its two users are the first owner's form and My profile's change of password.
`logout` (`login.js`) ends the session and opens the card again,
in the app as in a browser.
The card trims the name it sends (`Card.tsx`, `Owner.tsx`), and the server's
sign-in routes take it through `store.NormalName`, which trims:
D107-A holds where the block's tests show it does, each path checked.

**The bridge's contract** takes two versions (`docs/app.md`),
in the order the blocks run, the page testing for each call and field it uses:
version 6 with Block 21: a server's `held`, the session held there,
`{user, id, created_at}` and its code (the Analysis, "The gate of v1.5"),
kept in the app's file alone and never given to the page;
`unended` fed from a process that ended;
an entry of `unended` or `unrevoked` `pending: true` while its code waits,
the code itself never given to the page;
the answers of `signOut`, `forget`, `forgetAll` and `remove` saying what waits;
version 7 with Block 19: on the connect page `about()`, `exit()`,
`keep(id, url)`, the event `refresh` and the handover `#unreachable=<id>`.
Block 20 changes no call.

### The gate of v1.5 (Blocks 19 and 21)

Read on `main` at `chore(aps): +NOTE KeyLife v1.0, how the run went`,
2026-11-28, in the code alone; the lines are those of that commit.

**How the store keeps a token today** (D112-A).
A token is 32 random bytes, hex (`NewSecret`, `internal/store/tokens.go:63`),
shown once; the store keeps its SHA-256, hex, without a pepper (`HashToken`,
`:57`), in `tokens.hash`, unique (`internal/store/migrations/store.sql:317`),
and a revoked row keeps it, which is how `TokenEnded` (`:176`) tells a token
that ended from a guess.
The store makes the secret itself where it makes a token,
`addToken` (`:101`) and `AddKeySession` (`:256`),
and so holds it at that moment.
`AddTokenHash` (`:113`) is handed a hash alone, and nothing calls it but the
tests; the bootstrap writes the settings file's key itself (`keyIn`,
`internal/store/bootstrap.go:177`): a new row (`:180`), or an integration's
user's row of that hash made the file's key and live again (`:186`).
`RevokeToken` (`:210`) revokes a live token, a key's live sessions with it,
and what they subscribed to push; `ErrNoToken` where it is not live.
The server turns a presented secret into its hash in `resolve`
(`internal/server/auth.go:824`), the one place that holds every token's secret
at each request, and writes the token's last use in `touch` (`:959`),
at most once an hour (`touchEvery`, `:57`), through `TouchToken`
(`tokens.go:315`); `touch` is not handed the secret.

**Why a code of its own.**
The hash the store keeps would serve as a code at once, with no migration,
but whoever holds the store or a backup of it could then revoke any token:
the server's backups and a copy taken for a look would carry that power.
A code derived with a label, `RevokeCode(secret)`, the hex of
SHA-256(`"massimo revoke v1"` ‖ secret), the secret as the server gave it,
64 hex digits, is another value;
the store keeps `HashToken(code)` in `tokens.revoke_hash`,
from which no code can be had back.
So only a client that once held the secret can revoke the token by its code,
and the code lets nothing in: no request is ever authenticated by it.
The label holds a version, so that another derivation could follow beside it.

**The column, and the tokens made before it.**
The store writes `revoke_hash` where it makes the secret,
in `addToken` and `AddKeySession`,
and in AP KeyLife's `AddKey` and `RekeyToken` once those land;
neither `AddTokenHash` nor the bootstrap writes one.
A token made before the update has none, and the server writes it at the
token's first use after the update: `resolve` hands `touch` the secret,
and where the row has no `revoke_hash` (read with the row, `Token.RevokeKept`)
`touch` writes it with the use, whatever `touchEvery` says,
through `TouchToken`, which takes it and keeps a hash already written.
The settings file's key is passed over there (`settingsKey`'s rule,
`internal/server/login_api.go:451`: the hash of the file's token),
so that it gets none and is still touched at most once an hour.
It may hold one all the same: a key `addToken` made and named in the settings
file later, or an integration's key `keyIn` makes the file's;
so the route itself leaves that key live (below), as no revoke may end it
(D86-A, D89-A).
A token the server has not seen since its update has no `revoke_hash`,
so its code matches nothing; the app keeps a fallback for that (below).
The migration is additive, as `0025_token_parent.sql` is:
`ALTER TABLE tokens ADD COLUMN revoke_hash TEXT` and a unique index on it
where it is not NULL, `min_reader` 24, with a down part that drops both,
since every migration after 0024 comes with one
(`internal/store/migrate.go`, its header).
A binary for 24 to 26 reads the file and writes its tokens without the column,
which the next use with this binary fills.
`docs/store.md` lists the column in its tokens table (section 5, beside
`parent_id`, `:417`) and the migration among the migrations.

**Its number.**
`main`'s last migration is `0026_backup_pending.sql`, and so is `maint/0.17`'s;
since the freeze `main` numbers in the block from 0100
(`docs/store.md`, "Two lines of releases").
`loadMigrations` takes each number as the one before plus 1,
or the next multiple of 100 (`nextVersion`, `nextBlock`,
`internal/store/migrate.go:164`, `:173`), so a jump from 0026 to 0102 is
refused.
No 01xx file is on `main` yet, and three open pull requests claim 0100:
AP KeyLife's `0100_token_maker` (metaworx/massimo#481, with #482's
`0101_token_idle` and #484 and #490 stacked on it)
and AP ServerBackup's `0100_server_backups` (#492, with #493 to #495 on it).
So Block 21's is the number after `main`'s last when its pull request is opened,
0100 where none of those has merged,
and it is renumbered at its merge of `main` where one merges first:
`<NNNN>_token_revoke.sql` and its down part.

**The route.**
`POST /api/v1/tokens/revoke`, registered with `none` as the sign-ins are
(`internal/server/server.go:1503`), whose wrapper lets the request through
without a user (`auth.go:1008`); its body `{"code": "<64 hex digits>"}`.

- A server without a store (one token in the settings file) reaches the handler
  too, `s.opts.Store` nil: it answers `204` at once.
- A code that matches a live token's `revoke_hash` revokes it
  through the store's `RevokeByCode` and `RevokeToken`, a key's sessions with
  it; with AP KeyLife's Block 2, which makes `RevokeToken` take a key's heirs,
  they follow.
  The settings file's key is left live: the server hands the store the file's
  hash (`settingsKey`'s rule), which `RevokeByCode` passes over;
  with AP KeyLife the store knows that hash itself (`settingsHash`).
- It answers `204` whether the code matched, matched a token that had ended,
  matched the settings file's key, or matched nothing,
  so that it tells nobody which codes are live.
- A body that is not that object, or a code that is not 64 lower-case hex
  digits, is `400`, with the code `revoke_code_malformed` of AP ErrorCodes,
  whose Block 1 is on `main` (`errorCode`, `refuse`,
  `internal/server/errcodes.go`), its six texts `err.revoke_code_malformed`
  and its row in `docs/api-v1.md`'s "Error codes", which `TestErrorCodes` reads.
- The address's lockout applies, as for every route without a token,
  each of which checks it itself (`lockedOut`, `auth.go:1122`):
  `429` with `Retry-After` while the address is locked out.
  A code never counts toward the lockout, matched or not:
  one that matches nothing is a code taken already, a token that ended by itself,
  or one the server has not seen since its update,
  and 256 bits make guessing hopeless;
  counted, an app's codes would lock its own address out.
  `5xx` is the store's failure.
- What the route writes, since anyone may call it and nothing counts:
  today every request to a `none` route is an audit row of its own
  (`server.go:1462-1470`), an action outside `noCommand` (`audit.go:158`) also
  a row of `commands` keeping any `Idempotency-Key` (`beginCommand`,
  `commands.go:304`), and a `change` action's success a Configuration line in the
  log and in every open page's feed (`changeLines`, `audit.go:149`; `endAudit`,
  `:417`).
  So a code that matches nothing, one of the settings file's key, and a
  malformed body write no row of their own (`auditInfo.skip`, `audit.go:183`):
  they are counted per address and minute as refused requests are
  (`refusalAudit`, `audit.go:477`, `note` `:502`),
  with no change line and no Info line.
  A match alone writes its row, with the token as its target and its user as
  its subject (`auditTarget`, `auditSubject`), its change line,
  and an Info line as `deleteMyToken` writes one (`login_api.go:490`),
  "by code" in place of "by itself".
  The route takes an action of its own, `token.revoke_code`, in `noCommand`,
  with a `change` line and the six texts `log.action.token.revoke_code`:
  a revoke sends nothing to a Cube and its retry is the same `204`,
  while putting `token.revoke` into `noCommand` would also take the command row
  and the `Idempotency-Key` replay from both revokes that carry a token
  (`audit.go:58`, `:104`), which this plan does not touch.
  No row and no line ever carries the code, which the audit's `secretFields`
  leaves out already (`audit.go:163`).
- `docs/api-v1.md` beside `DELETE /api/v1/me/token`,
  `docs/api-v1-openapi.yaml` (`TestOpenAPICoversRoutes`) with `make api-types`,
  `TestAuditOfEveryWrite` (`internal/server/audit_test.go:47`) and
  `TestEveryActionHasALine` (`logchange_test.go:21`) for its action.

**AP KeyLife meets it** (D113-A).
KeyLife's Blocks 2 to 5 are open pull requests now
(metaworx/massimo#481, #482, #484, #490), and change what this half touches.
Where Block 21 lands first, they change, and the session "Open Items" is given
this list before Block 21 starts:

- `AddKey` writes `revoke_hash`, as `addToken` does;
- `RekeyToken` writes the new secret's `revoke_hash` on the key's row and puts
  the old one on the revoked row it inserts with the old hash,
  so that an old code does not revoke the key re-keyed;
- `tokenColumns` reads the column as `Token.RevokeKept`;
- Block 3's `touch`, which writes a session's use as its key's,
  still fills the presented token's own `revoke_hash`;
- the "live" of `RevokeByCode` follows KeyLife's rule for a live token
  (a key ended, a maker revoked), and its settings file's hash.

Where they land first, Block 21 takes them as they are. AP ErrorCodes' #488
changes `userFail`, `passwordBusy` (then `codePasswordBusy`) and `loginMode`,
which Block 21's first commit (D106-A) touches: whichever lands second adapts.

**The app's side** (D112-A; Block 21).
The app holds a token's secret at a sign-in (the page's `setToken`,
`Sessions.put`, `Sessions.kt:101`), at a key login (`Bridge.kt` `openKept`,
`:450`), at a key's making (`Bridge.kt` `keep`, `:481`; `AppKey.kt` `keep`,
`:170`) and at each unseal (`LoginStore.open`, from `AppKey.kt:265` and `:368`).

- A session's code is kept in its server's `held` (D102-A),
  `{user, id, created_at, code}`, written where a check learns its id
  (`Sessions.look`, `:133`), and only for a session (`Checked.kind` `session`);
  the kept login's in its record beside `keyId`
  (`KeptLogin`, `LoginStore.kt:24`), in clear, as the id is.
  A key the user typed into the card's "API key" way is not the app's:
  the card makes it the page's token (`setToken(key)`,
  `src/signin/Card.tsx:240`), and a check records it of kind `api`;
  it gets no `held` and no code, so nothing of it reaches `unended` after a new
  process or Exit, and forgetting it leaves it valid, as a logout does today
  (`leave` passes over keys, `Sessions.kt:313-314`; `end`'s `quietKey`, `:293`).
  A code of it would revoke the user's own key,
  and the settings file's key would then fail every start (D86-A).
- Where the app drops a token without its server ending it
  (`Sessions.leave`; the keys `Revoker`, `forgetLeavingKey` and Remove leave),
  the entry written into `unended` or `unrevoked`
  (`ServerStore.kt`: `SessionLeft`, `:36`; `KeyLeft`, `:21`) carries the code.
  At the next process's start a `held` whose session is no longer held moves
  there with its code (D102-A), and Exit (D91-A) moves them so at once.
- `held` and every code go to the file alone (`fileJson`):
  `toJson`, what the page sees and what Remove answers, carries no `held`,
  no code and no `taken` entry, and `pending: true` on an entry whose code waits;
  `save` keeps the stored lists as it keeps them today
  (`ServerStore.kt:177-178`), and `held` with them.
- At each contact the codes are posted:
  the connect page's `session(id, url)`, which follows every probe that finds a
  server (`checkSession`, `connect.js:254`, after `probeServer`, `:223`),
  at a start, a return, ↻ and a pull down, posts that server's codes at that
  address before its check (`ServerApi.revokeCode`, without the
  `Authorization` header `ServerApi.send` sets on every call today,
  `ServerApi.kt:150`); a code is no secret,
  so plain HTTP outside the private ranges carries it too.
  The adapter's `open` posts them too, at the address it opens,
  so that a server opened at the app's start (D43-A) takes them.
- On `204` the entry is marked `taken` and its code dropped:
  its line goes, and its tab's dot with it;
  the next session the app holds there for that user checks it as today
  (`KeysLeft.revoke`, `KeysLeft.kt:57`): the user's list no longer has a token
  the code ended, so the entry is dropped unasked,
  and one the server had not seen since its update is revoked then.
- A broken connection, `429` and `5xx` keep the code for the next contact;
  `400` drops it, logged.
  Any other answer, `404` first (a server older than the route: none has a route
  under `/api/v1/tokens`), and `401`, `403`, `405`, `409`, a redirect or a
  proxy's page of `200`, is a server that takes no codes: the code is dropped
  and the entry stays as today's, in today's words
  (`connect.keyLeft`, `connect.sessionLeft`).
- Remove posts the server's codes where the server answers,
  and keeps what is still to post in `gone`, a file of its own (`gone.json`,
  since `servers.json` is the list's array):
  the server's id, node, addresses and pinned fingerprint with the codes,
  never listed nor opened.
  The adapter probes those addresses at the app's start and each time the list is
  shown, and posts the codes only at an address where the probe finds the server
  at its node (`Addresses.answering`, `Addresses.kt:26`),
  as Sign out of all (`Sessions.kt:218`) and `Revoker` (`:213`) find theirs,
  so that an address given to another host spends no code.
  Each code is dropped on `204` or on a server that takes none,
  a session's 30 days after it was left, the longest a session lives unused,
  and every code 90 days after the Remove at the latest (D116-A).
- A kept login sealed before this version has no code until its next unseal;
  Turn off, Forget all or Remove that leave its key before then leave it as
  today, ended at the next sign-in there as that user.
- One pure function works the code out, `RevokeCode.of(secret)`,
  held to the Go function by one vector shared by
  `internal/store/tokens_test.go` and `RevokeCodeTest.kt`.

**The words** (D110, settled by D112-A; D105-A; Block 21).
While the server does not answer, the session's line says that Sign out ends
the session there the next time the app reaches the server
(`connect.sessionDown`); after a Sign out, a Turn off or a Remove that left a
token with its code, the toasts say so too (`connect.signedOutPending`,
`connect.loginOffPending`, `connect.removedPending`), and the Login section
lists the token as pending (`connect.sessionPending`, `connect.keyPending`).
A server older than the route answers `404` at the next contact,
and the entry's line then takes today's words.
`fillFacts` draws its lines again only where its key changed
(`connect.js:616`), and the key holds no probe state:
over plain HTTP `cert` is null whether the server answers or not,
so the key gains whether it answers, or the line of a server down would never
show there.

**A server's settings today** (D111-A; Block 19).
`settingsPanel` (`connect.js:477`) is one form (`formEl`, `panel server-panel`)
under the server's row: the name, the LAN and remote addresses,
Open at start, when the fingerprint is asked, `.server-facts` (`fillFacts`,
`:607`: the session with Sign out, the certificate's line with Certificate…,
each key left with Show in My profile, each session left),
the kept login with its key's id and Turn off, notifications where the app has
push, the message line, Cancel and Save (`formFooter`),
and after a rule Remove server with its line (`fillRemove`, `:538`).
Certificate… opens `#cert-dialog` (`connect.html:133-144`; `showCert`,
`connect.js:715`): the address, the subject, the issuer or "self-signed",
the names, the validity, the fingerprint and since when it is trusted,
offered only where the probe found a certificate at an address that answers.

**How the sections are built.**
The server's page switches its settings pages by the page bar's row
(`Switch`, `src/chrome/Bar.tsx:84`): `.page-link` buttons, each its icon and
its name, the one shown filled (`.current`), folded to their icons below 900 px
(`style.css:635`, `:654`);
the sign-in card's ways take that look unfolded (`.login-way`, `style.css:2197`).
The sections take it too, unfolded and without icons, their names alone,
since three names fit at 390 px:
`.page-link` buttons in a row of their own, `.server-tabs`,
the folding rule scoped to `.page-switch` so that it stays the page bar's.
`.page-link-label` ends a name too long in "…",
so the check at 390 px in the six languages refuses a name cut short.
The roles are a tab list's, as `src/shared/tabs.tsx` gives them to a part:
the row `role="tablist"` named by `connect.sections`,
each button `role="tab"` with `aria-selected` and `aria-controls`,
each section `role="tabpanel"` (`.server-section`); built with `elem` and
`button`.
All three sections are built when the panel opens and hidden but the one shown,
so that the draft, the focus and the lines `fillFacts` keeps in place outlive a
switch, and the emulator suite's clicks still find their buttons.
Connection is the form; Login and Certificate are none,
so that Enter in a field saves Connection alone.
Back or ✕ while Login or Certificate is shown and Connection differs shows
Connection and asks there, the focus on the question:
`confirmInPlace` asks inside an action row, and Connection's is hidden then.
The other ways the panel closes (another server's cog, a tap on a row,
Add server, App settings) close it without asking, as today:
D93-A names Back alone, and ✕ closes as Back does (D94-A).
The "changed here" mark is AP UnsavedMarks' disk
(`ItemMark`, `src/chrome/Menu.tsx:48`, a `Sign` of `src/shared/icon.tsx:55`),
which no plain script can draw yet: `components.js` has the signs' table
(`SIGNS`, `:164`) and `markEl` alone,
so the block writes `signEl(kind, state)` there, beside `markEl`.
Login's dot is a mark on its tab, named for a screen reader.

**The certificate in place.**
`#cert-dialog` becomes a `<template>` of `connect.html`, `tpl-cert`,
its rows the same and its ids turned into classes,
cloned into the Certificate section and filled by `fillCert`,
which is `showCert` drawn in place, and drawn again only where it changed,
as `fillFacts` is.
Where the server does not answer, the section shows the pinned fingerprint
and since when it is trusted (`s.fp`, `s.trusted_at`);
where none is pinned, that the certificate shows once the server answers;
over plain HTTP, that there is no certificate.
`connect.certTitle`, `connect.certButton` and `connect.certLine` go,
and so does the listener of `#cert-close` (`connect.js:1321`).

**The tests that move.**
`test_connect_certificate` (`tools/ui-test/test_connect.py:1010`) reads the
section in place of the dialog; the other cases of `test_connect.py` that use
the panel's lines (45 lines name their classes) open their section first.
The emulator suite clicks Turn off and Remove by their classes
(`SessionsTest.kt:31`, `:50`, `:69`) through `element.click()`
(`Page.kt:83`), which reaches a hidden section's button;
those cases open the section first all the same, as a user does,
and so do the cases Block 21 adds.

### Risks

- gomobile is experimental by its own README; the core's surface is small,
  and `-buildmode=c-shared` with a hand-written JNI layer is the fallback behind
  the same `app/core` API.
- Reproducibility: gomobile has leaked the checkout path into the library's build info;
  the CI check of Block 4 catches it before F-Droid does.
- The 14-day closed test delays production on Play; F-Droid and the
  GitHub release do not wait for it.
- A kept key is as strong as the phone's Keystore and its fingerprint;
  its revocation is one click under My profile, by the phone's name,
  and Turn off and Forget all revoke it at its server,
  or at the next session there where it was not reached.
- Android ends a hidden app often,
  so `return` and `start` ask for the fingerprint often;
  `logout` is the price's other side, and the choice is the user's (D24-C).
- The cover of `return` is the app's own, as the old switch's prompt was:
  it stops a hand on the unlocked phone, not a debugger.
  The picture Android keeps of a hidden app shows the page as it was,
  unless the window is marked secure, which also forbids screenshots:
  Block 8's gate says which.
- With D32-A a server's page learns the names of the user's other servers;
  with D32-B nothing.
- A phone without a strong biometric,
  or on Android 8 to 10 without a fingerprint, gets no login by fingerprint,
  and keeps its session as a browser does.
- One wrapping key (D63-A, Block 17): the Keystore no longer keeps the
  servers' keys apart; a prompt passed for one server gives, for its step,
  what opens every server's login, in the app's memory and dropped at the
  step's end.
  Every opening still asks, and a copy of the files still opens nothing.
- The emulator job (Block 18) adds minutes to every run that changes `app/`,
  beside the Android app job,
  and an emulator in CI is known to stall at its boot or at a prompt:
  the job has a time limit of its own and keeps the emulator's log,
  and a flaky case is fixed or taken out, not retried in a loop.
- The sign-in card and ☰ are shared with AP UiParts and AP FirstOwner,
  which the session "UI refactor" is changing now:
  Blocks 15 and 16 are announced to it before they start,
  the later of two merges `main` first,
  and the card's UI tests run in the four cases after that merge.
- The time limit (D82-A, Block 16): a route that waits longer by design and
  is not given its own limit fails as "no answer" while it still runs;
  a write without an answer may have been done, which its text says.
- An ended token not counted (D64-A, Block 17):
  a client that keeps sending one is no longer slowed down by the lockout,
  and gains nothing by it.
- The sessions with the process (D102-A, Block 21):
  a session in memory now outlives the activity, as long as the process lives;
  the cover still guards a return, and a process that ends still loses it,
  which `held` records so that the next sign-in ends it.
  A process ended by force before a check learnt a session's id leaves that
  session valid at the server until it ends by itself, as today.
- Back through the page's own history entries (Block 19):
  an entry pushed and never taken back would keep Back from leaving the app;
  the emulator suite checks that Back from the bare list leaves it.
- Pull down (Block 19) adds a library to the app,
  which Dependabot's library group then carries.
- The copy's toast (D97-A, Block 20) takes the toast's place for a moment:
  a copy while an outcome is shown replaces it, as any newer outcome does;
  with D115-A the copy's word is a bubble of its own, and replaces none.
- The revoke code (D112-A, Block 21) lies on the phone's disk unsealed
  while its token lives: whoever reads the app's files can end its sessions
  and its key, and nothing more; whoever sees a code posted ends a token the app
  was dropping anyway.
  A token the server has not seen since its update matches no code,
  and ends at the next sign-in as that user, as today;
  a removed server's codes are posted for 90 days at most (D116-A),
  and a key left there after that stays listed under My profile.
- The route without a token (Block 21) is one more door for anyone:
  it answers the same `204` to everything, writes a row, a change line and an
  Info line only where a code matches, counts the rest per address and minute,
  and stays behind the address's lockout;
  a loop of codes that match nothing costs a hash and a lookup each,
  and fills neither the store nor the log.
- The sections (D111-A, Block 19) put Remove and the key's lines one tap deeper;
  the Login tab's dot keeps a key or a session left in sight.

What changes, by file, in the blocks still to run:

| File | Block | Change |
|---|---|---|
| `app/core/*.go`, `go.mod`, `Makefile` | 1 | the bound package, the `tool` directive, the targets |
| a merge request to `fdroiddata` | 5 | the recipe; here `app/README.md`'s section |
| the Play Console | 6 | the owner's; `app/README.md` records the steps |
| `app/android/**` | 7, 11, 19 to 21 | the service and its server kept open; push; Exit, Back, pull down, Store login, `about`, a page that fails to load; the sessions with the process and `held`, a fingerprint tap's check, the revoke codes, `gone`, no refused logout, the emulator's cases |
| `app/ios/**`, `.github/workflows/ci.yml` | 9 | the Xcode project, the macOS job |
| `internal/server/ui/connect.html`, `connect.js`, `components.js`, `style.css`, `i18n/*.json`, `tools/ui-test/` | 19 | ☰ in the header with the version row and Exit, the width, the order, Back, a server's settings in three sections with the certificate in place, `signEl`, Store login, pull down, `#unreachable=<id>`, Show in My profile probing first |
| `internal/store/tokens.go`, its migration, `internal/server/auth.go`, a route of its own, `internal/server/audit.go`, `internal/server/errcodes.go`, `ui/i18n/*.json` (`err.*`, `log.action.*`), their tests, `docs/api-v1.md`, `docs/api-v1-openapi.yaml`, `docs/store.md` | 21 | the revoke code: `tokens.revoke_hash`, its fill at a token's making and use, `POST /api/v1/tokens/revoke`, its action `token.revoke_code` |
| `internal/server/ui/src/chrome/Menu.tsx`, `src/shared/server.tsx`, `src/shared/copy.tsx`, `src/shared/password.tsx`, `src/signin/`, `src/profile/`, `components.js`, `login.js`, `style.css`, `i18n/*.json`, `tools/ui-test/`; `internal/server/` where a sign-in route does not trim | 20 | the server's ☰, the version row, the copy's toast, the card's line, the command, the password pair, Sign out to the list, My profile's failures, the name trimmed |
| `internal/server/users_api.go`, its test, `docs/api-v1.md` | 21 | a cancelled request |
| `internal/server/ui/connect.js`, `i18n/*.json`, `tools/ui-test/` | 21 | the list while a server does not answer, the codes pending; D103-A's UI case |
| `docs/app.md`, `docs/ui.md`, `docs/user-manual.md` | 11, 19 to 21 | push; the contract's versions 6 and 7, the list, the menu and the cards, the sessions |
| `app/README.md`, `README.md`, `docs/roadmap.md`, `GUIDELINES/wip/HANDOVER.md` | 10 | the documents; the roadmap and the handover by the coordinating session |
| `CHANGELOG.d/` | 1, 7, 9, 11, 19 to 21 | a fragment per change |

## Implementation Plan

Each block ends with what the contract asks of a change:
`make ui` before anything compiles Go; locally the tests of what the block
touches (`go test` of the touched packages with `MASSIMO_ENV_FILE=/dev/null`,
the touched files of the UI suite in its four cases,
`make ui-check` for a part,
the Gradle tasks `:app:assembleFossDebug`, `:app:testFossDebugUnitTest`,
`:app:lintFossDebug` and `:app:lintNightlyDebug` in `app/android`, with
`JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64 ANDROID_HOME=$HOME/Android/Sdk`,
`make actionlint` for a workflow,
and since D87-A `make app-emulator-test` for a change under `app/` or to what
of the server the app drives, through `sg kvm -c '…'` in a session started
before D85-A),
`gofmt -l .` empty, `go vet` for linux, windows and darwin,
and `GUIDELINES/shared/tools/check` before every push;
the whole suites are CI's, and so are the race run and the cross builds
but where a block's Verification names them.
Each new test is seen failing with its fix undone.
`make api-types` goes with every change of the OpenAPI file
(Block 8, "As built").
Nothing reaches a real Cube or a box: the fake Cube answers;
a phone is the owner's, used on the owner's word.
Every commit cites `AP MobileApp v1.6, Block <n>`;
a block's bullet is a fragment, `CHANGELOG.d/<Heading>/<branch>.md`;
a pull request per block.
The app's bullets of Blocks 3 to 18 are frozen into 0.17 on `maint/0.17`:
a change of v1.5 to what they describe is a fragment of its own on `main`,
not an amendment of theirs, its heading the one COMMIT.md 5.2 gives it.

### Block 1: Core

1. `app/core/core.go`:
   `Start`, `Stop`, `Status`, the `Events` interface,
   the recover in every entry point; `core_test.go`.
2. `go.mod`:
   `golang.org/x/mobile` as a `tool`; `Makefile`: `app-core-android`,
   `app-core-ios`, depending on `ui`,
   with the checks for the NDK and Xcode and the reproducibility flags;
   `.gitignore` for the outputs.
3. The fragment, Added:

   ```
   - `app/core`, the server as a library for the apps, bound by gomobile
     into an `.aar` and an XCFramework (`make app-core-android`,
     `make app-core-ios`; `docs/app.md`).
   ```

**Verification:**
`go test ./app/core/`;
`make app-core-android` on a machine with the NDK produces the `.aar`,
and `gomobile bind` lists the bound symbols without a type complaint.

### Block 2: Connect

Done on 2026-11-09, pull request 125: the page,
the bridge's contract in `docs/app.md`, the texts in six languages,
`test_connect.py`, with D4-A and D5-A.

**As built:**
`docs/app.md` is the contract, the one source for Blocks 3, 7, 8 and 9;
where it and the Analysis differ, it holds.
Every call returns a Promise over a script injected at the document's start
(D6-A).
`probe(urls, fp)` takes the fingerprint to accept beside the system's,
so that a server not yet saved is probed with the one the user just compared;
`open(id, url)` names the address the probe chose, any other refused;
`save` resolves to the server as saved, with its id, and keeps `fp` as `trust`
set it, `trust` the one call that pins;
`core` and `push` exist only where the app has them;
the `found` event's fields are the TXT keys of AP ServerAnnouncements
(`v` as `version`).
`connect.html` carries `<base href="../">`, so that its URLs resolve as the page
at `/` does, and its policy in a `<meta>` for the app's copy, served without the
server's header.
An address typed is probed at `https://host/`, `http://host/`,
`https://host:8099/` and `http://host:8099/`, `host:port` by HTTPS then HTTP;
a private address becomes the LAN one, any other the remote.
`TestUIPage` checks the ids `connect.js` looks up against `connect.html`
(`uiPageScripts`), and `TestUITranslations` its `data-i18n` keys;
the tests' fake bridge is `tools/ui-test/fake_bridge.js`.

### Block 3: Shell

Done on 2026-11-16, pull request 341, and checked on the owner's phone on
2026-11-17 with one fix, pull request 344, with D6-A to D9-A.

**As built:**
built on 2026-11-09 and moved onto `main` a week later, at the owner's `EXEC D9-A`.
The choice of address is the page's: the page probes both and takes the one
whose `node` matches, so the JVM tests cover the probe's answers and the trust,
not a choice the native side does not make.
`androidx.core` stayed at 1.18.0, since 1.19.1 wants `compileSdk 37`
(D88-A raised it later).
The theme is AppCompat's DayNight, which `androidx.biometric`'s dialog before
Android 9 needs.
A server's page restores a backup through the system's file picker
(`onShowFileChooser`); its downloads are `blob:` URLs, which a WebView hands to
no one, so the app says it cannot save files yet and points to the browser.
The User-Agent begins `Massimo/<version> (Android <release>; <model>)`,
the WebView's own after it, since the server labels a session with its first
64 characters.
`-Pmassimo.ui=<dir>` takes the UI from another checkout.
The icon's layers are `make ui-brand`'s, in `app/android/app/src/main/res-brand/`:
the adaptive icon's foreground is the mark as the web UI's maskable square
places it, its monochrome layer the M in one colour with the hands carved.
The app's copy of `ui/` leaves out `src/` and the bundle `parts.js`,
the server page's, so the app needs no `make ui`;
the Kotlin and Gradle files and `bridge.js` open with the SPDX and copyright lines.
`changelog.sh update-contract` rendered `app/**` into the contract;
`sync-docs.sh` cannot run in a worktree whose `GUIDELINES/shared` is empty,
so its two renderings took the line by hand.
The phone test of 2026-11-17, with the owner on the `foss` debug build of
`main`, over wireless debugging from WSL's own `adb`, against a
`serve -tls self-signed` with the fake Cube in WSL (through `adb reverse`)
and the owner's Windows service on the LAN over plain HTTP:
`ConnectPageTest` 2 of 2; discovery found the Windows service by its
announcement; a self-signed server added, its fingerprint shown and pinned;
the first owner registered by OPAQUE from the phone, the store keeping a record
and no hash; sign out and in by OPAQUE; the session's label naming the phone
first; the token across a restart, from the Keystore-sealed store;
the old switch's prompt; a changed certificate shown old and new,
Trust re-pinning it, the session kept; a plain-HTTP server on the LAN,
once the phone's VPN let the LAN through.
Its one fault: the connect page did not load (`net::ERR_INVALID_RESPONSE`),
since `WebViewAssetLoader` hands a path handler the path after its prefix,
so `/ui/connect.html` reached `AssetsPathHandler` as `connect.html`;
the handler is given `ui/` and the path, and a file the app has not is
answered 404; only the instrumented test could see it.
Its findings in the page became Block 12,
the missing way back to the list Block 8 (D26-A),
and those outside the app D27-A's two lines.

### Block 4: Release

Done on 2026-11-17, pull requests 349 and 352,
under a gate of its own (D12 to D21):
the job "Android app" in `ci.yml` and in `build.yml`,
the three jobs of `release.yml`, the nightly app, `app/version.json`,
`packaging/app-version.sh` and `packaging/app-sign.sh`.
The owner's three keys are in `signing` and `signing-rolling`,
their certificates under `app/android/signing/`.
Still to see: a version tag's signing of the APK and the AAB,
at the owner's next `make release`.

**As built:**
the block's three steps, written against a pipeline AP Channels has rebuilt
since, were refitted at its gate and built so:
`ci.yml`'s job checks the version file, builds the debug app with its unit tests
and lint, builds the release APK in two places and compares them byte for byte,
then the AAB and the nightly APK, all three signed with a key made for the run,
checked against its certificate and refused against another;
`build.yml`'s job builds a version tag's APK and AAB and a nightly's APK,
its version from `git describe`, unsigned and attested, as the artifact
`release-app`, with `continue-on-error`, so that a failed app build leaves the
boxes' release standing;
`packaging/app-version.sh` is the formula's one place,
`packaging/app-sign.sh` the signing and the comparison, used by the three
signing places; a debug build's versionName ends in `-dev`.
The unsigned files are attested too, so that a rebuild, F-Droid's among them,
can be compared with what the release signed.
The nightly's icon composition (`src/nightly/res/mipmap-anydpi/ic_launcher.xml`)
and its background colour are written by hand, its layers `make ui-brand`'s;
a themed icon tints the same layer for both apps, the label telling them apart.
Beyond the steps: `app/android/signing/README.md`, `*.p12` in
`app/android/.gitignore`, the release notes naming `SHA256SUMS.sig`,
and `README.md` showing how to check it with the release key.
A scratch script made the three keystores and set the six secrets
without a password on a command line; it is not committed.
The job took four minutes in CI's run of the pull request,
and All tests passed wrote `app=true`.
The first nightly to carry the app, on 2026-11-18, was checked by hand
(`EXEC verify`): its `SHA256SUMS.sig` verifies against
`internal/update/rolling-key.pem` and the release key refuses it;
the APK's line in `SHA256SUMS` matches;
`apksigner verify --min-sdk-version 26` passes with schemes v2 and v3 and one
signer, `CN=Massimo rolling, O=The Massimo Authors`,
whose certificate is `app/android/signing/rolling-cert.pem`;
`aapt2` shows `ch.metaworx.massimo.nightly`, "Massimo nightly",
versionName `0.16.0-alpha.1-301-g<commit>` and versionCode `16000301`,
D21-A's code of 0.16.0 with 301 commits since its first tag.
Not checked: the signed APK's build provenance attestation, at the owner's word.

### Block 5: F-Droid

1. The recipe written and tried with `fdroid build` in the F-Droid buildserver
   image where possible, else reviewed against the recipes of the analysis;
   whether the buildserver has platform 37, which `compileSdk` needs since D88-A;
   once the core is built in, `make ui` before `gomobile bind`;
   `app/README.md`: the F-Droid section.
2. The owner: the Play Console account, developer verification,
   the package name registered with the key's SHA-256.
3. The Request For Packaging, or the merge request, to `fdroiddata`,
   once the first release with the APK is out.

**Verification:**
F-Droid's rebuild matches the release APK (their verification page says so),
which Block 4's two paths on one machine could not show;
the app appears on F-Droid under our signature.

### Block 6: Play

1. The owner: the app in the Play Console, Play App Signing with our key,
   the listing, the Data safety form, the foreground-service declaration
   once Block 7 is in, the internal testing track.
2. The tracks decided at the block's gate,
   from the proposal made at Block 4's gate (the Analysis, "Google Play");
   the closed test with 12 testers for 14 days; then production.
3. `app/README.md`:
   the Play section, the steps recorded.

**Verification:**
the internal track installs on the owner's phone and updates from the release;
production approved.

### Block 7: Local

1. `CoreService`, the notification channel, the boot receiver, the `WifiLock`,
   the battery exemption request, the data directory;
   the connect page's card wired through `core.*`;
   the notice about the Cube's one client; the switch for other devices.
2. The phone's own server kept open (AP FirstOwner, D18-A):
   open, and reached at the loopback address alone,
   it would ask for its first owner at every start;
   once the core is up without users, the adapter sets `auth.keep_open`
   with `PUT /api/v1/settings`,
   and sets it back to `false` before the switch for other devices opens the
   server to the LAN,
   which then asks for a first owner as a server on the LAN does.
   The store removes the setting anyway once a user exists,
   and refuses it on a server with users, `409`
   (AP FirstOwner, D19-A and D20-A).
3. Tests: the service's lifecycle in a unit test with a fake core,
   the setting set at the first start and cleared before the LAN;
   by hand on a phone with the fake Cube on the LAN.
4. `docs/app.md`:
   the stand-alone mode and its limits,
   the server kept open while it is the phone's alone.
5. The fragment, Added:

   ```
   - The Android app runs the server itself: the Cube on the LAN, the
     store on the phone, a foreground service that survives the screen
     going off; the phone then holds the Cube's one client (`docs/app.md`).
   ```

**Verification:**
the Gradle tasks; on a phone,
the embedded server serves the page from `127.0.0.1`
without asking for its first owner,
holds the fake Cube over an hour with the screen off, and frees it when stopped;
opened to the LAN it asks for its first owner, the setting off.

### Block 8: Login

Done on 2026-11-18, pull requests 365 (the server's route and
`tokens.parent_id`) and 368 (the bridge's version 2, the sign-in form,
the connect page, the Android adapter), under a gate of its own (D32 to D37),
and checked on the owner's phone on 2026-11-20;
the check's findings are Block 13.

**As built:**
D32-B to D37-A changed the steps: no `serverNames` or `switchTo`,
one `LoginKey` per server, the Recents picture for `return`,
the plain-HTTP rule, the user manual, and the Verification's new fingerprint
as D35-A says.
No `login.forget()` on a server's page: Turn off is the connect page's
`forget(id, url)`, at the address the probe chose, where the adapter revokes the
key; `open` resolves `true`, or `false` where `{login: true}`'s prompt was
dismissed, where version 1 resolved `null`.
A session counts only while its key does, in `TokenByHash`, the token list and
an open event stream: a binary for 24 opens a file of 25 and revokes a key
without its sessions, which a binary since 25 then refuses (`docs/store.md`);
migration 0025's down part drops the column and loses the tie alone,
the column having no foreign key so that `DROP COLUMN` can take it.
`TestRaiseOfAFilledStore` and `TestRaiseDownLosses` read the newest schema
(`undoneFromNewest`) where they had named 24;
`TestAuditOfEveryWrite` knows the login by key as its key's user's.
CI's `TestGeneratedFileCurrent` found `api.d.ts` stale after the OpenAPI file
gained `key_id`: `make api-types` belongs with every change of that file.
Two holes found reading the adapter again were fixed before the merge:
a Keystore key's generation that failed otherwise than with a
`GeneralSecurityException`, as it does where no fingerprint is enrolled,
escaped a keep on the main thread;
a Turn off whose key the Keystore refused otherwise than as invalidated
left the login kept.
Besides Turn off, Remove and Forget all, a kept login is forgotten
by a certificate trusted anew, a node that changed (the seal binds both, D34-B),
an invalidated Keystore key, and a key the server refuses (`401`).
The key's label is the phone's model and "login by fingerprint" in the phone's
language, cut to the server's 64 bytes.
The cover's prompt carries no key: it guards the session in memory;
the key's prompts carry the key's cipher;
the cover is not drawn for the app's own prompt or file chooser,
nor for a page signed out.
The user manual is English alone.
The pull requests' commits:
"feat(api): A session from an API key, POST /api/v1/login/key",
"fix(ui): The parts' API types carry a session's key_id",
"feat(app): The login by fingerprint" and
"fix(app): Every refusal of the Keystore ends a keep or a Turn off cleanly".

### Block 9: iOS

1. `app/ios`:
   the Xcode project and the adapter of the Analysis,
   the login by fingerprint through the Keychain;
   the XCFramework from `make app-core-ios`; the licence exception text.
2. `.github/workflows/ci.yml`:
   the macOS job.
3. The owner: the Apple Developer Program as an individual, TestFlight,
   the review.
4. `docs/app.md`.
5. The fragment, Added:

   ```
   - The iOS app (`app/ios`): the same page and bridge as Android's, with
     Bonjour discovery, the Keychain and Face ID or Touch ID; the server
     runs in it while the app is open (`docs/app.md`).
   ```

**Verification:**
the simulator build in CI; on an iPhone,
the checks of the phone test and of Block 8.

### Block 10: Documents

1. `docs/app.md` complete: the two modes, the bridge, the stores, the limits;
   `app/README.md`: the developer's setup; `README.md`:
   a section "The apps" with the store links.
2. The roadmap (items 16 and 17) and the handover: the coordinating session;
   this plan's lines pruned at its retirement by the session that retires it.

**Verification:**
`GUIDELINES/shared/tools/check` clean;
every bridge call in `docs/app.md` exists in both adapters.

### Block 11: Push

1. The UnifiedPush connector in both flavours,
   the embedded FCM distributor in `play`; the registration,
   the endpoint through the bridge to the server;
   the notification channels per kind, the texts in six languages;
   the embedded core's notifications into the same channels;
   the connect page's "Notify me on this phone".
2. Tests: the message rendering per kind; by hand with ntfy as the
   distributor against a `serve` with a subscription.
3. `docs/app.md`.
4. The fragment, Added:

   ```
   - The Android app is notified by the server through UnifiedPush, or
     through Google's service in the Play build (`docs/app.md`).
   ```

**Verification:**
the Gradle tasks; a window opened at the fake Cube notifies the phone after the delay,
with the app closed.

### Block 12: Polish

Done on 2026-11-18, pull request 366 on `claude/mobile-polish`,
under a gate of its own (D38, D39).

**As built:**
`connect.html` carries the toast's markup, a copy of `partials/toast.html`,
since the app's copy of `ui/` is not assembled from partials;
the list's message line `#servers-msg` is gone.
`connect.js`: `say` shows an outcome in the toast;
a probe that finds a server takes back the toast that said it does not answer;
`takeHeardName` gives a saved server discovery's name,
also to one added by its address whose node discovery heard before;
`showWhere` leaves out an address that is the row's name,
in the saved rows and the found ones.
The self-signed notice: `header.js` `renderTLS` tells the header part that the
page runs in the app (`appBridge()`, the signal `pinned` of `src/header/store.ts`),
and `Tls.tsx` leaves the sentence out; the sign-in's `#login-tls` stays hidden,
`renderTLS` toggling it already, so neither `partials/login.html` nor `login.js`
changed.
`app/README.md`: the rig, `adb reverse` stalling on one port,
a whole URL probing one connection where `host:port` probes two,
the WebView's DevTools socket, a phone's VPN that keeps LAN connections out.
`.github/dependabot.yml` reads `/app/android`, monthly, in one group
(two since D88-A), and `GUIDELINES/project/CI.md` section 8 says so.
Tests: `test_connect.py`'s toasts and names (`test_connect_names`),
`test_tls.py`'s app case (`test_app_leaves_out_self_signed_notice`).
Three commits, "fix(app): The connect page's toasts and names, the notice left out",
"docs(app): The phone test's rig and pitfalls in app/README.md" and
"chore(ci): Dependabot reads the app's Gradle files",
and two merges of `main`; the fragments under Fixed, the connect page having
shipped in 0.16.0-alpha.1, and Added.
The macOS job found no runner twice (GitHub's macOS arm64 capacity),
which the second merge's run passed.

### Block 13: Check

Done on 2026-11-20, pull requests 384 (`claude/mobile-check-ui`, the web UI's)
and 386 (`claude/mobile-check-app`, the app's, built on it),
under v1.2's gate (D40 to D53), and checked on the owner's phone the same day
(the Analysis, "The second phone check's findings").

**As built:**
the header goes only while the sign-in's card is open:
hidden whenever the page was signed out, it also hid in the moment before the
server says who is in, a blank page.
Once the page drops its token at a 401 (F7), its reads until the sign-in carry
none and are refused too; the server counts no request that offers no token
(`internal/server/auth.go`), so they lock nothing out
(`test_two_tiles_one_sign_in`).
The main ☰'s Select server is the app's alone (`test_main_menu.py`).
`MainActivity` is an `AppCompatActivity`, on which AppCompat's night mode works,
the mode set in `attachBaseContext`;
a change of Appearance makes the activity anew,
the connect page keeping `#app-settings` in its address.
Turn off revokes the key with the session first, then signs out,
since signing out first leaves nothing to revoke with;
a `403` or `401` there falls back to the key and the prompt,
and a `404`, or a `401` to the key itself, counts as revoked.
Open at start takes the LAN address as soon as it answers,
the remote one once the LAN one failed or has not answered within a second,
the window showing the app's background, about four seconds at most.
Sign out of all probes each server for its node before it sends the session,
so no token goes to what now holds an old LAN address.
`session` and `signOut` may go over plain HTTP outside the private ranges,
where the page sends its session anyway; the kept key never does.
A certificate's `subject` and `issuer` are RFC 4514 names,
the settings' line showing the common name.
The close buttons are "×", as the toast's; the Add card's × is hidden while the
list is empty; the page still searches at its start, for the saved servers'
moved addresses; Sign out of all is not asked about;
the ☰'s question clears the toast, which at 390 px covered its Yes.
A test certificate with names, `app/android/app/src/test/resources/san.pem`,
its key thrown away.
CI's Android app job failed once on Maven Central's "429 Too Many Requests";
and once #384 was merged, #386 and `main` had two merge bases, GitHub reporting a
conflict `git merge-tree` did not find, which a `merge:` of `main` cleared.

### Block 14: Second check

Done on 2026-11-21, metaworx/massimo#390 on `claude/mobile-check-fixes`,
under v1.3's gate (D54 to D57),
and checked on the owner's phone the same day, with D58-A and D59-A;
the check's findings are Blocks 15 to 18.

**As built:**
two commits, "fix(ui): The sign-in card centred, with Cancel and Password, as the
second check found" and "fix(app): Forget all revokes the keys, and a key left is
revoked at the next session", and a merge of `main`,
each part built by a sub-agent and read by two second readers.
The merge met AP UiParts' Block 10, which had moved My profile's sessions and
keys into `profile.js`: the marked key (`markProfileKey`, the row's
`token-marked`) went with them.
Remove with a kept login turns it off first, as Turn off does, then removes the
server, its question saying so and its toast naming the keys still valid
(D58-A); Forget all signs out of each server as Turn off does.
A login kept before Block 13 learns its key's id from `GET /api/v1/me`'s
`token.key_id`, kept with the login at the check that finds it.
Keys are left also where a login is forgotten because the phone no longer opens
its key (`KeyInvalidated`) or a certificate was trusted anew, the node the same;
a changed node leaves none, the key being another server's.
A key left is revoked only where the user's list has its id as kind `api`,
never the token in use, the key it was made from or the login's own key,
since a store rolled back may give an id again; an id the list does not have is
dropped unasked, and an empty answer drops nothing.
Turn off and Remove, given no address, look for the one that answers first.
The answers of version 4: `forget` `{revoked, left}`,
`forgetAll` `{revoked, left, local}` (server ids),
`remove` `{revoked, keys, local}` (key ids), `local` a key forgotten on the
phone alone, its id not known.
androidx.biometric shows no prompt and calls nothing back once the activity's
state is saved (Home between two prompts of Forget all):
such a prompt is answered at once as dismissed, by the library's own
`isStateSaved`, that key left.
"Select server…" is one text for the card's ☰, the main ☰ and the Cube choice;
Cancel is on the first owner's form in the app too;
at 390 px the row stays on one line in all six languages, so German's key
was not shortened.
F28's two defects, both fixed: a login kept before Block 13 had no key id,
so Turn off went the key's way with the prompt though a session was held;
and on that way the login was forgotten before the sign-out,
so a session in memory was forgotten on the phone, not ended;
the check (step 5) then showed the revoke by the session before its logout.
CI's first run failed `test_system_layout.py`'s `test_header_and_page_bar_stay`
once at 390-de, Integrations' line landing between its two readings;
"test(ui-test): The page bar's test waits for System settings' reads before it
measures" fixed it on the branch.

### Block 15: List and card

Done on 2026-11-23 and 2026-11-24, metaworx/massimo#409, after Block 16,
and checked on the owner's phone on 2026-11-25; its "As built" follows the steps.

The findings F31, F33, F34 and F40, with D62-A, D72-B, D73-A, D74-A and D84
(the Analysis, "The phone check of Block 14"),
in one pull request on `claude/mobile-list-card`, after Block 16.
Steps 4, 5 and 8 change AP UiParts' parts and AP FirstOwner's card:
the session "UI refactor" is told before the block starts,
and whichever lands second merges `main` first.

1. F40's rule (D72-B, D74-A): `Opening.kt`, a pure function of a server's
   `ask`, whether a login is kept, and whether a session is held and where,
   that says what a tap does: open the page,
   ask the prompt without the key and keep the session held in memory,
   or ask it with the key and make a session;
   the Analysis' table, row for row ("Opening from the list").
   `OpeningTest.kt`, a JVM test of every row.
2. The bridge's `open` (`Bridge.kt`): `open(id, url)` asks as the rule says,
   the prompt titled with the server's name;
   passed, it opens the page with the session held or the one made;
   cancelled or failed, it opens the page with the session withheld for that
   opening (`Opened`, `token()` answering `null`), so that its card shows;
   a key the server refuses is forgotten as today.
   `open(id, url, {signIn: true})` opens it so without a prompt;
   `{login: true}` stays, the rule's prompt with the key.
   Open at start (`MainActivity.openAtStart`) stays as it is.
3. The list (`connect.js` `serverRow` and `fillRow`, `connect.html`):
   a tap calls `open(id, url)` and leaves the asking to the bridge;
   where a login is kept, the row's fingerprint button gives way to a
   sign-in icon, `icon-signin`, the mirror of `icon-signout`,
   labelled "Sign in with password or key",
   which calls `open` with `{signIn: true}`;
   a refused key's toast and the list read again,
   as the fingerprint button's were.
4. The server under the card's title (F31, D62-A):
   `login.status()` gains `server`, `{name, url, node, fp}`,
   the server the page was opened for, at the address it was opened at
   (`Bridge.kt`, `login.status`);
   `src/signin/Card.tsx` shows the name and the full address
   (`https://host:port`) under `#login-title`, in the app alone,
   the name once where it is the address,
   with a labelled ⓘ that shows the instance id (`node`) and the
   certificate's fingerprint, each copied with one tap (`src/shared/copy.tsx`),
   an empty one left out;
   read through the card's existing hook (`readApp`, `src/signin/app.ts`);
   a line of its own after `#login-title`, not inside it
   (`test_keep_open.py` compares the title's text),
   and above AP FirstOwner's steps, so that it shows on the sign-in,
   the first owner's form and the "Keep it open…" confirmation alike.
5. Two entries of ☰ (D73-A): `src/chrome/Menu.tsx`, after "Select server…",
   in the app alone: "This server's settings…" and "App settings…",
   each shown where the bridge has its call and calling the server's page's
   new `openServerSettings()` or `openAppSettings()`
   (the hooks beside `appConnect` in `login.js`, `src/plain/login.d.ts`,
   `src/shared/app.ts`);
   the bridge shows the connect page with that server's settings open,
   or App settings (`Bridge.kt`, `Host.connectPage`, `MainActivity.kt`:
   `connect.html#settings=<id>` and `#app`),
   and `connect.js` opens the panel from the fragment at its start and clears it;
   the server's page passes no id and reads nothing of them;
   `Origins.kt` lets a server's page make the two calls.
6. Show in My profile (F33): `connect.js` `fillFacts`:
   the button off while the probe's state is neither `lan` nor `remote`,
   its title saying why, and drawn again when the state changes.
7. Remove server (F34): `connect.js` `settingsPanel`:
   Remove out of the form's footer, on a line of its own below Cancel and Save,
   after a rule, as "Remove server",
   with a line saying what it does: the server leaves this list,
   and while it answers this phone is signed out there and its key revoked;
   its question as before.
8. The password's eye (D84): `src/shared/password.tsx`:
   a button at the field's right end, inside its row,
   a closed eye while the password is hidden and an open one while shown,
   labelled "Show password" and "Hide password" (`aria-pressed`),
   keyboard operable; it turns the field's type between `password` and
   `text`, keeping its value, its caret and its `autocomplete`;
   hidden again at each opening of the card and at the submit;
   a browser's own eye (`::-ms-reveal`) hidden in `style.css`;
   with D84-A every password field, with D84-B the card's alone.
9. The texts: every new key in all six `ui/i18n/<lang>.json`,
   FR/IT/NL/PL machine-translated and marked;
   the app's `strings.xml` where a prompt's title is new.
10. The contract's version 5, with Block 16's
   (`docs/app.md`, `res/raw/bridge.js`, `Origins.kt`):
   `open`'s rule and `{signIn: true}`, `login.status()`'s `server`,
   `openServerSettings()` and `openAppSettings()`.
11. Tests: `OpeningTest.kt`;
    `test_connect.py` over `fake_bridge.js` at version 5:
    on the connect page, a tap's `open` without options,
    the sign-in icon's `{signIn: true}` and its label,
    Show in My profile off while the server does not answer,
    Remove's line and its question, a panel opened from the page's fragment;
    on a server's page, the card's line and its ⓘ, none in a browser,
    the two entries of ☰ and the calls they make;
    `test_main_menu.py`, ☰ in a browser without the two;
    the card's line and ☰ measured at 390 px in the six languages;
    the eye: the type turned and back with the value kept, its labels,
    hidden again at the next opening, in a browser and in the app.
12. `docs/app.md` ("The login by fingerprint": the tap, the cancel, the icon),
    `docs/ui.md` ("In the apps", the sign-in card, ☰),
    `docs/user-manual.md` where it tells the list and the card.
13. The fragments, under the headings COMMIT.md 5.2 gives them.

**Verification:**
`make ui`; `make ui-check`;
the touched UI test files (`test_connect.py`, `test_main_menu.py`,
`test_login.py`) in the four cases, at 1100 and 390 px;
`MASSIMO_ENV_FILE=/dev/null go test ./internal/server/...`
for the embedded files and `TestSharedPartsKnowNoRegion`;
`gofmt -l .` empty, `go vet` for linux, windows and darwin;
the Gradle tasks, `OpeningTest.kt` among the unit tests;
`GUIDELINES/shared/tools/check`.
The owner's phone check to propose at the block's completion gate:

1. a tap on a server with a kept login, asked at each start, no session held:
   the prompt, with the PIN as its other way;
   a cancel opening the server's card, its fingerprint button there;
2. the sign-in icon: the card at once, no prompt;
3. a server asked at each return, opened from the list twice without the app
   hidden between: asked both times; the cover after Home as before;
4. the card's line and its ⓘ, the fingerprint compared with `massimo tls show`;
5. ☰'s two entries, each landing on the connect page with its panel open;
6. Show in My profile off with the rig's `serve` stopped;
7. Remove server on its own line, its text, and its question;
8. the look D83-A moved here: switching between Password and API key,
   no field focused and the card standing still (D60-A);
   the password manager's autofill: the name tapped, its suggestions opened,
   API key chosen, a suggestion picked, Password chosen again:
   the name and the password filled, and the sign-in passing;
9. the password's eye on the card and on the first owner's form.

**As built:**
five commits, a merge of `main` (RefreshBatteries' Blocks 1 to 5, without a
conflict) and two fixes CI's runs called for:
"fix(security:app): A tap or Back into a server asked at each return asks for the
fingerprint first" (steps 1 to 3),
"feat(ui): In the app the sign-in card names the server it signs in to, with an ⓘ
for its id and fingerprint" (step 4),
"feat(app): A server's ☰ in the app has This server's settings… and App
settings…, on the app's own page" (step 5),
"fix(app): Show in My profile off while the server does not answer, Remove server
on a line of its own" (steps 6 and 7),
"feat(ui): Every password field has an eye showing the password, and
auth.password_reveal turns it off" (step 8);
built by a sub-agent, read by two second readers, each reading the rework twice.
What the readers found, each fixed in its step's commit:
Back from the list reloaded a server's page with its session and no prompt,
the WebView's history holding the list, the server and the list again:
the list taking a page's place withholds the opening of the server it left
(`Opened.leaving`), and the list once loaded clears the WebView's history,
so that Back from it leaves the app; hence `fix(security:app)`,
whose CHANGELOG amended the login's unreleased bullet rather than add a
Security fragment;
a withheld page's `setToken(null)` leaves the held session alone (`Opened.takes`);
a server's page navigating to the app's own page loads it without its fragment
(`Origins.byPage`), and the app's host answers anything but `GET` with `405`
(`Origins.appServes`), a form posted there escaping `shouldOverrideUrlLoading`;
a shown password inside a fold: the type never turns inside a closed `<details>`,
Chrome drawing nothing of the input otherwise, and a hide waits for the fold's
next opening;
the browser's own eye is hidden on the whole page, the API key's field and a
provider's client secret included;
the switch's test holds `/healthz`, proving that no eye shows before its answer;
switching the eye off and on brings no shown password back;
no Warn where a request's context ended before the setting's read
(`PasswordReveal`, `PasswordLogin`);
and Preact's property path rendered `autocorrect="off"` as "on",
now set as an attribute.
Departures: a tap over plain HTTP outside the home network opens the page at its
card; any failed tap where a login is kept reads the list again,
and the list makes one opening at a time;
the tap's prompt is titled with the existing `login_title`;
the withheld opening survives the activity made anew (`KEY_WITHHELD`),
and the cover leaves a page at its card alone;
`{login: true}` stays, though the list no longer uses it;
App settings' fragment is the existing `#app-settings`, not step 5's `#app`;
Show in My profile is turned on and off in place;
Remove server's rule is a CSS border, its line one of two texts, with a kept
login and without;
the card's password row is a `.login-field` with a `<label for>`;
the eye is on password fields alone;
D84-A's switch is `auth.password_reveal`, on by default,
carried by `/healthz` as `password_reveal`, always present and true without a
store, since no public route carried settings before a sign-in;
the page shows no eye until `/healthz` has answered;
its switch, "Show password button", is in System settings → Users,
for holders of `settings`.
Fragments: three new Added ones (`claude-mobile-list-card-server.md`,
`-menu.md`, `-eye.md`), and the app's unreleased bullets amended.
CI's first two runs failed `test_password_eye.py::test_my_profiles_password_eyes`,
each passing on its retry: the page hid a shown password at the fold's `toggle`,
which Chrome fires a task after the click, and a close and an opening close
together come as one event, so a shown password could be seen for a frame;
"fix(ui): A password shown is hidden at any toggle of its fold, also a close and
an opening taken as one" and "fix(ui): A password shown is hidden in the same task
its fold opens, before any paint" (a `MutationObserver` on the `open` of every
`<details>` around the field) settled it, five runs with four workers passing.
The phone check of 2026-11-25 passed its nine steps
(the Analysis, "The phone check of Blocks 15 to 17").

### Block 16: Sessions and keys

Done on 2026-11-23, metaworx/massimo#408, after Block 17,
and checked on the owner's phone on 2026-11-25; its "As built" follows the steps.

The findings F32 and F35 to F39, with D65-A to D70-A, D80-B and D82-A
(the Analysis, "The phone check of Block 14"),
in one pull request on `claude/mobile-sessions-keys`,
its first commit the test of step 1, failing.
Steps 2, 3, 6, 7 and 8 change AP UiParts' parts and the page's shared code
(`app.js`, `login.js`): the session "UI refactor" is told before the block
starts, and whichever lands second merges `main` first.
The contract's bullet "The web UI" (`/AGENTS.md`, rewritten by AP UiParts'
Block 15) binds these changes: a part gives the focus in the handler of the
user's action or in a layout effect, a request takes the session's number,
and a plain script looks up the skeleton's ids alone.

1. F36's test first (D68-A): in `tools/ui-test/test_sign_in_resend.py`,
   a read sent without a token while the card is open,
   its 401 held back by a route until `login/finish` has answered,
   then let through: one card, the read sent again with the new session,
   and the first session the only one at the server.
   On `main`'s code it must fail, a second card opening,
   which the block's NOTE records;
   should it pass there, F36 has another cause,
   and the block's gate says so before step 2.
2. A 401 of an earlier sign-in (D68-A): `app.js` `api()` and `apiRaw`:
   a 401 for a request that carried another token than the one the page
   holds now, or none, while it holds one, opens no card and drops nothing:
   a read is sent again with the token held,
   a write fails as not sent (`sentAgain`, AP SignInResend);
   a 401 of the token held stays as it is.
   AP UiParts' Block 15 (metaworx/massimo#404) gave every load the session's
   number (`MassimoParts.session()`, `stillSession`), so that an answer after
   a sign-out writes nothing; it left `api()`'s 401 path as it was,
   which still opens the card for a request sent before the sign-in:
   the test tells whether the number is the simpler test here.
3. A session replaced is signed out (D68-A):
   `login.js` `setToken`: in a browser, a session the new token replaces
   ends at the server (`POST /api/v1/logout` with it, not awaited,
   a refusal ignored);
   in the app the bridge's `setToken` ends the session it held
   (`Sessions.put`, which `login.fingerprint`'s session goes through too),
   and the page leaves it to the bridge, so that it ends once;
   a key replaced is not ended: the server refuses a key there with `400`,
   which the app's log leaves out.
   `login.js` `setToken` returns the bridge's promise,
   and the card and the first owner's form await it before `login.keep()`,
   whose session `Bridge.kt` reads on the main thread
   while `setToken`'s write runs on the io thread.
4. Remove signs out first (F38, D69-A):
   Remove's whole path into `Revoker.kt` (`remove`),
   so that `RevokerTest.kt` holds it:
   a kept login turned off first, as today (D58-A),
   else the session signed out at `url` or at the address that answers,
   then the server forgotten; `Bridge.kt` `remove` calls it.
5. Sessions left (F39, D70-A): `Sessions.kt` keeps the id of the session held
   as its last check found it (`GET /api/v1/me`'s `token.id`);
   where a session is forgotten without the server ending it
   (Turn off, Forget all, Sign out, Sign out of all, the server not answering),
   its `{user, id}` goes into the server's `unended`
   (`ServerStore.kt`, beside `unrevoked`, dropped where the node changes);
   `KeysLeft.kt` revokes it with the next session the app holds there for that
   user, as it revokes a key left:
   an id only where the user's list has it as kind `session`,
   never the token in use;
   Remove, whose server goes,
   names it in its answer (`sessions`, beside `keys`),
   its toast saying that it stays valid there until it ends;
   one whose id was never learnt is forgotten on the phone alone.
   The server's settings name a session left as they name a key left
   (`connect.js` `fillFacts`).
6. This device's key and the ids (F35, D65-A, D66-A):
   `login.status()` gains `key_id`, the id of the key the phone keeps for
   the server, `0` for none or not known;
   the app's status moves from `src/signin/app.ts` to `src/shared/app.ts`,
   read by the card and by My profile;
   `src/shared/tokens.tsx` `TokenRow`: the key of that id tagged
   "this device's key", in place of "its key" where both hold,
   however the session was made;
   each row shows its id, in My profile and in System settings → Users,
   the row being one part;
   `connect.js`: the kept login's line names its key's id whenever one is kept
   (a server's `login_key`, `Server.toJson`).
7. My profile after the sign-in's revoke (F37, D67-A):
   the bridge's `setToken` answers once the keys and sessions left there are
   revoked, with `{revoked: [ids]}` (`Bridge.kt`, `Sessions.check` before the
   answer; `null` before);
   `login.js` keeps that answer as a promise
   that My profile's read of its list waits for
   (`src/profile/`, `src/shared/tokens.tsx`);
   a key the address marks (`#profile?key=<id>`) among the ids revoked
   is not marked, and a toast names it.
8. The time limit (F32, D82-A): `app.js` `apiRequest`:
   a request unanswered after about 20 s is aborted and fails as
   "no answer from the server",
   and the button that waits for it is no longer busy
   (`whileBusy`, `BusyButton`);
   a route that waits longer by design passes its own limit
   (a device's or a room's write, up to two minutes; the Cube search;
   any other `docs/api-v1.md` says waits);
   a write's text says that it may have been done;
   the event stream and the `keepalive` sends stay as they are.
   The session "UI refactor" is told: it is shared code.
9. D80: `docs/api-v1.md` ("Tokens", `POST /api/v1/logout`)
   and `docs/api-v1-openapi.yaml`:
   a key revokes itself with `DELETE …/tokens/{id}` as its own bearer,
   its sessions with it; a session ends itself with `POST /api/v1/logout`.
10. D80-B: `DELETE /api/v1/me/token`
    (`internal/server/login_api.go`, `server.go`,
    `audit.go` as `token.revoke`):
    the bearer revoked, a session or a key, a key's sessions with it;
    `400` where the caller holds no token of a user
    (an open server, the server's one token without users);
    `docs/api-v1.md`, the OpenAPI file, `src/api.d.ts` by `make api-types`,
    `TestOpenAPICoversRoutes`;
    Go tests: a session and a key revoke themselves, the key's session with it,
    the bearer refused after;
    the app's revoke with the key alone (`Sessions.revokeWithKey`,
    `ServerApi.kt`) uses it, and the old route at a server that answers
    `404` or `405`; `ServerApiTest.kt`, `RevokerTest.kt`.
11. Tests: the UI suite: step 1's;
    `test_login.py`, a session replaced in a browser ended at the server;
    `test_connect.py`, on a server's page the tag and the ids by
    `login.status()`'s `key_id` and the toast of a key the sign-in revoked
    (the fake bridge's `setToken` answering `{revoked}`),
    on the connect page the kept login's key id, a session left with its line,
    Remove's `sessions`;
    `test_users.py`, the ids on the rows at 390 px,
    and Revoke ending after the limit, with Playwright's clock run forward;
    the JVM tests: `SessionsTest.kt` (a token put ends the one held,
    a key's `400` logs nothing, a check keeps the session's id),
    `RevokerTest.kt` (Remove's sign-out before the server goes,
    a server down leaving the session in `unended`),
    `KeysLeftTest.kt` (a session left revoked at the next session,
    never the token in use, never a key's id for a session's),
    `ServerStoreTest.kt` (`unended` kept, dropped with a changed node).
12. The contract's version 5 (`docs/app.md`), with Block 15's:
    `login.status()`'s `key_id`, `setToken`'s answer,
    a server's `login_key` and `unended`, `remove`'s `sessions`;
    `docs/ui.md` (My profile: this device's key, the ids,
    the mark after the sign-in's revoke; "In the apps": a session left),
    `docs/user-manual.md`.
13. The fragments, under the headings COMMIT.md 5.2 gives them.

**Verification:**
`make ui`; `make ui-check`;
the touched UI test files (`test_sign_in_resend.py`, `test_login.py`,
`test_connect.py`, `test_users.py`, `test_profile_help.py`) in the four cases,
at 1100 and 390 px;
`MASSIMO_ENV_FILE=/dev/null go test ./internal/server/...`,
`go test -race -count=1 ./internal/server/` and `make cross`;
`gofmt -l .` empty, `go vet` for linux, windows and darwin;
the Gradle tasks; `GUIDELINES/shared/tools/check`.
The owner's phone check to propose at the block's completion gate,
the server's audit read after each:

1. F36's path again, a password sign-in where a key is left:
   one card, and one session at the server;
2. the login turned on at a password sign-in:
   the new key tagged "this device's key" under My profile,
   every row with its id,
   and the key's id in the server's settings;
3. Show in My profile after a sign-in that revoked that key: no mark, the toast;
4. Remove of a server without a kept login: the logout at the server;
5. Turn off with the rig's `serve` stopped, then started and signed in again:
   the session left revoked within a second;
6. a Revoke while the rig's link hangs: it ends after the limit, saying so.

**As built:**
six commits and a merge of `main` that brought plan files alone:
"test(ui-test): A read refused for an earlier sign-in, the card closed, opens a
second card" (step 1),
"fix(ui): A 401 of an earlier sign-in opens no second card, and a session a
sign-in replaces is ended" (steps 2 and 3, the page's half),
"fix(ui): A request the server does not answer fails after its time limit, so that
no button turns for good" (step 8),
"feat(api): DELETE /api/v1/me/token revokes the token that calls, as the app's
kept key now revokes itself" (steps 9 and 10),
"fix(app): Remove signs out first, and a session forgotten without its server is
ended at the next sign-in" (steps 3 to 5, the app's half),
"feat(ui): My profile shows every key's id and in the app this device's key, read
after the sign-in's revoke" (steps 6 and 7);
built by a sub-agent, read by two second readers, each reading the rework twice.
Step 1's `test_late_refusal_of_an_earlier_sign_in` failed on `main` in the four
cases, each with "no second card", which confirmed F36's cause;
its 401 is held back until `login/finish` has answered and the card closed,
since the bug shows only once the card's promise is gone.
A late 401 is told by the token the request carried, not by the session's number,
since a sign-in with nobody signed in before raises none (step 2).
A replaced session is ended in a browser only where `/me` was read with it
(`meToken`), its kind being `session`, a key answering `400`;
in the app the bridge ends it after its answer, so that a stalled server holds
no card; a dead session's 401 no longer wipes a token another tab stored since.
The bridge's `setToken` checks every new session before it answers,
one more `GET /api/v1/me` per sign-in, learning its id (D70-A) and serving
`{revoked}` (D67-A); the app's sessions keep the token's kind.
A replaced session is noted in `unended` before the bridge answers and removed
once ended, so that an app ended meanwhile loses none;
a session left is revoked only where the listed one has its id and the same
non-empty `created_at` (`GET /api/v1/me`'s `token.created_at`),
one without a time dropped unasked;
`KeysLeft` revokes keys and sessions from one list and returns the ids revoked.
The time limits are set by path in one place, `waitOf`, each above the server's
own: 20 s by default; a write to a Cube and adding a Cube 330 s; a repair 630 s;
the search for Cubes 90 s; the look for an update 75 s;
the end of a provider's sign-in and the integrations found 45 s;
the stream's ticket the default; `apiRaw` (a backup's download, a restore) none,
as `docs/ui.md` says.
`DELETE /api/v1/me/token` is refused `409` for the settings file's
`MASSIMO_API_TOKEN`, which it could revoke beyond the bootstrap's reach,
and `400` on a user's behalf (`X-MassimoHA-On-Behalf-Of`),
which would revoke the integration's shared key under the acting user's name;
a key's own revoke needs no id, so a login kept before version 3 revokes itself
at a server with the route; `make api-types` wrote `src/api.d.ts` as it was.
"This device's key" shows in whichever user's panel lists the key.
The contract's version 5 was raised here.
Beyond the steps: `tools/ui-test/README.md`'s entry for `test_sign_in_resend.py`,
and `app/README.md`.
Fragments: new Fixed ones for the late 401 and the time limit,
the defects being in the released web UI; an Added one for the route;
the app's unreleased bullets amended.
CI's first run failed
`test_personal_updates.py::test_a_read_that_finds_a_change_waiting_keeps_it`
in the four cases: its `pause()` gave Playwright's clock milliseconds where it
takes seconds, firing every pending timer, the held read's new time limit among
them; "test(ui-test): The personal page's tests pause Playwright's clock at the
page's now, in seconds" fixed it.
The phone check of 2026-11-25 passed but F43, F44 and F45
(the Analysis, "The phone check of Blocks 15 to 17").

### Block 17: One prompt and the lockout

Done on 2026-11-23, metaworx/massimo#405, the first of v1.4's run,
and checked on the owner's phone on 2026-11-25; its "As built" follows the steps.

D63-A and D64-A (the Analysis, "One prompt for Forget all", "The lockout"),
in one pull request on `claude/mobile-one-prompt`,
the server's commit and the app's, the server's first.

1. The lockout (D64-A): `internal/store/tokens.go`:
   a lookup of a hash among the tokens that ended:
   revoked (`deleted_at`), past `expires_at`,
   or a session whose key was revoked;
   `internal/server/auth.go`: `resolve` tells such a token from an unknown one,
   and `authorize` refuses it as today, `401` with the same words,
   without counting it toward the address's lockout (`s.limiter.fail`),
   `POST /api/v1/login/key`'s key included;
   an unknown token counts as before, and the sign-in by name keeps its count.
   Tests (`internal/server/auth_test.go`, `internal/store/tokens_test.go`):
   more refusals than the limit (`DefaultAuthFailures`) within its window,
   with a revoked session, a session logged out, one expired and a revoked key:
   no `429`; as many unknown tokens: `429`, as today.
   `docs/api-v1.md` ("Authentication", the lockout).
2. The app key (D63-A): `LoginKey.kt`: one Keystore key, `massimo.login.app`,
   bound as today (every use behind the prompt,
   `setUserAuthenticationParameters(0, …)` from Android 11,
   the strong biometric alone before),
   seals the app key, 256 random bits, kept sealed beside the logins
   (`LoginStore.kt`);
   each server's login sealed under the app key in software,
   AES-GCM with today's associated data (`LoginStore.aad`);
   a prompt opens the app key for one step,
   keeping one login, opening one, turning one off, or Forget all,
   and the step drops it, its bytes cleared.
   Keeping a login asks once, as today:
   the app key opened, or made and sealed at the first keep.
3. Forget all asks once (`Revoker.kt`, `Bridge.kt` `byKey`):
   its first server that needs the key opens the app key,
   and every later one in that run opens from it;
   Turn off and Remove ask once, as before;
   the app key dropped at the run's end,
   the activity gone included (`onIo`).
4. A login kept before it keeps its own key and its own prompt until it is
   turned off, forgotten or kept anew:
   the app opens it by its own alias where one is there (`LoginKey.alias(id)`),
   and Forget all then asks once for the app key and once per such login.
5. A key the phone takes back (a fingerprint enrolled on Android 8 to 10,
   the screen lock turned off) is now the one key:
   every login sealed under it opens no more,
   and each is forgotten with its key left by its id (D57-A),
   as each was one by one before.
6. Tests: `LoginStoreTest.kt`, and a JVM test of the app key over a fake
   `KeySource`: one prompt to keep, one to open, one for Forget all over three
   servers that need the key; the app key dropped after each step;
   a login opening for its own server alone; a login kept before asking its own;
   the key taken back leaving every key; `RevokerTest.kt`;
   the real Keystore is Block 18's.
7. `docs/app.md` ("The login by fingerprint": one key for the app,
   one prompt for Forget all), `docs/api-v1.md` (step 1),
   `docs/user-manual.md` (Forget all).
8. The fragments, under the headings COMMIT.md 5.2 gives them.

**Verification:**
`make ui`;
`MASSIMO_ENV_FILE=/dev/null go test ./internal/server/... ./internal/store/...`;
`go test -race -count=1 ./internal/server/ ./internal/store/`;
`gofmt -l .` empty, `go vet` for linux, windows and darwin; `make cross`;
the Gradle tasks; `GUIDELINES/shared/tools/check`.
The owner's phone check to propose at the block's completion gate:

1. Forget all as Block 14's step 2 meant it, one server holding a session
   and two holding none: one prompt; the audit of each;
2. a login kept after the update, opened and turned off: one prompt each;
   a login kept before it, opened with its own;
3. on the rig, a session logged out sent again more often than the limit
   within a minute: `401` each time, no `429`.

**As built:**
three commits:
"fix(server): A token that has ended is refused without counting toward the
lockout" (step 1),
"fix(security:server): A guessed Bearer beside any token counts toward the
lockout", a hole the server's reader found in the code as it was,
and "fix(app): Forget all asks for the fingerprint once, one key for the app
sealing every login" (steps 2 to 6);
built by a sub-agent, read by two second readers, one per half;
CI passed every job at the first try.
The app key's steps are a class of their own, `AppKey`, so that a JVM test over a
fake key source runs them; `LoginStore` keeps the sealed app key under the
reserved name "app" and marks each login sealed under it (`byApp`);
the fake key source is a shared `JvmKeys.kt`, with a `FakePrompt`.
`Revoker.ByKey.close()` ends a run's step, also where the run throws,
and `MainActivity.onDestroy` calls `bridge.close()`, besides `onIo`.
A key the phone takes back: in a keep, the logins under it are forgotten first;
when opening, the step answers Gone for every later login under it and forgets
them at its end, each with its key left by its id (D57-A);
only the Keystore's own word counts as taken back, a bad tag or a permanently
invalidated key (`KeyInvalidated`) in the exception or its causes,
any other failure being Failed and forgetting nothing;
the first build took any security exception for it,
which would have forgotten every login at a passing error,
and a login kept before is held to the same narrowing.
A dismissed prompt holds for the rest of Forget all (`refused`), so "once" holds.
Forget all's prompt has a title of its own, `login_forget_all_title`.
`LoginKey.alias` takes a server's id alone (24 lowercase hex digits),
so that a page's `forget("app")` reaches no Keystore key.
Two keeps at once make one app key, the second answered Dismissed.
The server: a removed user's token also counts as ended,
and a store that cannot tell counts the token as a guess (`401`, counted,
never `500`); `docs/store.md`, the OpenAPI description of
`POST /api/v1/login/key` and `docs/ui.md` say the new rule;
`TestLoginKey` checks that a revoked key is not counted.
The hole: a wrong Bearer header beside a disabled user's, an expired guest's or
a disabled role's `?token=` was answered `403` and never counted,
and beside a live `?token=` on the event stream `200`,
with a `Deprecation` header on a wrong guess alone, which told a guess from the
key, the success clearing the address's count;
a guessed Bearer header is now refused `401` and counted whatever is beside it,
the query not read; a wrong ticket beside a forbidden Bearer token counts before
its `403`; a guessed Bearer header also hides a valid stream ticket sent with it,
which a browser's EventSource, sending no Bearer header, never meets.
Fragments: a Fixed one for the server, the lockout's rule having been released;
a Security one for the hole; the app's change amending Block 14's unreleased
bullet (`claude-mobile-check-app-menu.md`).
The phone check of 2026-11-25 passed its three steps
(the Analysis, "The phone check of Blocks 15 to 17").

### Block 18: Emulator

Done on 2026-11-24, metaworx/massimo#411, after Blocks 15 to 17;
its "As built" follows the steps, with D85-A to D87-A after it.

D71-A's suite (the Analysis, "The emulator"),
in one pull request on `claude/mobile-emulator`,
after Blocks 15 to 17, whose flows it covers.

1. The suite (`app/android/app/src/androidTest/kotlin/ch/metaworx/massimo/`):
   instrumented tests over the WebView and the bridge on an emulator
   (androidx.test, UiAutomator for the system's prompt;
   `app/android/gradle/libs.versions.toml`, `app/android/app/build.gradle.kts`),
   against a `serve` on the host with the fake Cube,
   a temporary store and a user,
   over HTTPS with a self-signed certificate,
   reached at `127.0.0.1` through `adb reverse`, as the phone rig reaches one;
   the server's address and the user passed as instrumentation arguments.
   The cases: a server added and its fingerprint trusted;
   the login kept after a password sign-in, one prompt;
   a tap with the prompt passed, and cancelled into the card (Block 15);
   a server asked at each return opened twice from the list,
   and the cover after Home;
   Forget all over two servers with one prompt (Block 17);
   Turn off with and without a session, and Remove signing out (Block 16),
   each read back from the server's audit;
   the card's line and ☰'s two entries;
   `ConnectPageTest.kt` kept.
2. The fingerprint: the make target sets a PIN (`adb shell locksettings set-pin`)
   and enrolls one finger with `adb emu finger touch`, once per emulator;
   in a test, each prompt it waits for is asked for by a line in the log under
   the suite's tag,
   which the target's helper answers with `adb emu finger touch`,
   the emulator's console being out of a test's reach;
   a test that cancels presses the prompt's Cancel through UiAutomator.
3. `Makefile`: `make app-emulator-test`:
   an AVD made where missing, from a pinned system image that has the
   fingerprint's settings, started without a window
   (`-no-window -no-audio -no-snapshot`);
   the fake Cube, the `serve` and the helper started;
   `:app:connectedFossDebugAndroidTest`; everything stopped after;
   its scripts and their README in `tools/app-emulator/`;
   it needs the SDK's emulator and KVM, and says so where either is missing.
4. CI (`.github/workflows/ci.yml`): a job "Android emulator" beside
   "Android app", run where What to test answers `app`,
   KVM given to the runner's user, the emulator and its image pinned,
   a time limit of its own; among All tests passed's needs;
   its result counted with "Android app"'s under the Tested record's `app=`,
   which has no room for a new word (`GUIDELINES/project/CI.md`, section 2),
   so that `app=true` holds only where both passed;
   on a failure the test report and the emulator's log as an artifact;
   `make actionlint`;
   `GUIDELINES/project/CI.md`:
   section 2's table and the Android app's paragraph.
5. `app/README.md` ("Building and testing"): the target, what it needs,
   and what it does not cover; `tools/app-emulator/README.md`.
6. What the phone check keeps:
   a real fingerprint sensor, the phone maker's autofill and the look;
   a later block's completion gate proposes only those on the phone.
7. The fragment, under the heading COMMIT.md 5.2 gives it.

**Verification:**
`make app-emulator-test` passes locally where KVM is there,
each case of Blocks 15 to 17 seen failing once with its fix undone;
the job passes on the block's pull request,
its time and any run it needed again named in the block's NOTE;
`make actionlint`; the Gradle tasks; `GUIDELINES/shared/tools/check`.
No phone check of its own.

**As built:**
two commits, "test(app): The app's flows on an emulator, locally and in CI"
(steps 1 to 7) and "test(app): The emulator suite counts what each run ran, and CI
keeps a pass's results", added after CI's first run;
built by a sub-agent and read by a second reader, a second builder doing the
rework.
The suite could not run on the development host, whose `/dev/kvm` the user could
not open until D85-A; the emulator 37.3.10 and the image android-34,
google_apis, x86_64, revision 14, were installed there.
CI's first run was the suite's first, and it passed:
the emulator booted by the builds' end, 223 s after its start,
the suite ran in about 4 minutes, the job in 10.
The second run counted 14 tests, none skipped, failed or left unrun:
`ConnectPageTest`'s two; `EnrollTest.aFingerIsEnrolled`;
`ForgetAllTest.forgetAllAsksOnceForTwoServers`;
`KeepTest`'s login kept after a password sign-in with one prompt,
and a server added with its certificate trusted;
`SessionsTest`'s Turn off with a session, Remove signing the session out first,
and Turn off without a session after the prompt;
`TapTest`'s prompt and its cancel into the card, the card naming its server with
☰'s two entries, Back from the list leaving the app and the server's page then
showing its card, the cover after Home, and a server asked at each return asking
at every tap.
The reader's findings, fixed in the first commit:
the results were kept only on a failure, and the job's own limit marks a run
cancelled, so the suite's step has a limit of 32 minutes inside the job's 45,
the upload runs on a failure or a cancellation,
`run.sh` takes the step's shell's place (`exec`) so the runner's signal reaches
it, and its cleanup takes the screens and reports home before it stops the
emulator;
a failed enrollment would have been tried by every test, about 44 minutes,
so the finger is enrolled once, before the suite, and each test's setup fails at
once without it;
the helper says a failed touch and answers the next;
the boot's checks are bounded (`timeout 10` each);
the device's log is read from the moment adb sees it,
the emulator's carrying the kernel's messages (`-show-kernel`);
the audit's key is passed in a file on the emulator, not on Gradle's command line;
a failure's message names its step, never its script, which may hold the test
password; `sdk.sh` leaves nothing of a failed download.
Departures: the finger is enrolled through Settings by UiAutomator, in
`EnrollTest`, run first, not by the make target (step 2);
a cancel is Back, Android 14's prompt with the PIN's fallback having no Cancel;
CI downloads the pinned archives itself, each held to its size and SHA-1 from
Google's repository (`sdk.sh`), `sdkmanager` installing no given revision;
the AVD is written by hand;
a setup that is not the case under test goes through the bridge;
the server's page reached again is reached by script, Back from the list no
longer reaching it;
the audit is read by the test over the pinned certificate with an owner key made
for the run;
What to test's `app` paths include `tools/app-emulator/`, and not the server's
page (`index.html`, `partials/`, `src/`), whose change the push to `main`
catches, as `GUIDELINES/project/CI.md` says with what adding it would cost;
the Tested record's `app=` needs both Android jobs (`packaging/tested-record.sh`
and its test);
the count (`tools/app-emulator/tally/`) checks each of the two Gradle runs'
JUnit results against the `@Test`s the sources declare, since Gradle passes a
run where a test was skipped by an assume or left out by a filter,
and a skip or a test not run fails under CI or `APP_EMU_STRICT=1`;
CI keeps a pass's results for a week;
`GUIDELINES/project/CI.md` went to v1.70.0, with the record, the time limits,
the count and section 8's pins, which no Dependabot entry reads.
The pinned image's WebView is Chrome 113 and never updates:
page code beyond it fails there first.
After the block, metaworx/massimo#418 (D86-A, D87-A) added
"test(app): The emulator suite keeps the lock screen away from the enrollment":
its first local runs, with `sg kvm -c`, showed the lock screen still up while the
PIN was set and Settings driven under it,
and `dumpsys window` opening with the last ANR's copy of the state;
`run.sh` reads the lock screen from the window manager's policy and the
activities' keyguard state alone, takes it away, with the PIN where it is set,
before the PIN, after it and before each Gradle run,
and keeps it away while Gradle runs with a guard (`results/keyguard.log`).
On a host with a load of about 21 on 16 cores the fifth run passed 12 of 14,
two time limits failing (a probe without an answer within four seconds,
the connect page's call within 59 s); CI's job passed with the guard.

### Block 19: The list

D90-A to D95-A, D96-A on the list's menu, D108-A, D109-A, D111-A,
and the points taken as written for the list:
the version row, Back, a server's settings, Store login and pull down
(the Analysis, "The phone check of Blocks 15 to 17" and "The gate of v1.5"),
in one pull request on `claude/mobile-list-menu`, from `main`, for 0.18,
after Block 21 and before Block 20.

1. ☰ in the header (D90-A): `connect.html`: `#servers-menu` moves from the Servers
   card's head into `header.connect-top`, at the brand's right,
   as a server's page has its ☰ (`nav-menu`);
   its entries: the version row, a rule, Add server…, Sign out of all,
   Forget all stored credentials, a rule, App settings, Exit;
   the card's head keeps ↻ alone;
   the entries keep their ids, so that `connect.js` `initMenu`, `renderMenu` and
   `menuButton` follow, and `askInCard` still asks under the list,
   the focus back to ☰.
2. The version row (taken as written): first in ☰, Massimo's mark and the build's
   version, from the bridge's new `about()` (`{version}`, the build's
   `versionName`, contract version 7), with its copy icon;
   a tap copies the whole version through `copied()`, as every copy goes,
   and closes ☰; no row where the bridge has no `about`.
   What the copy shows is Block 20's (D97-A, D115), which lands after:
   until then the word drawn at the element goes with the closed ☰.
3. Exit (D91-A): ☰'s last entry, where the bridge has `exit()`;
   `Bridge.kt` drops every session held in memory, each recorded one moving into
   `unended` with its revoke code (D102-A, D112-A, Block 21),
   sends nothing to a server, and `MainActivity` ends with
   `finishAndRemoveTask()`; the codes go at the next start's contact.
4. The width (D92-A): `style.css` `.connect-main` up to the page's 1100 px,
   in place of 640 px, the cards and the list at its full width below it.
5. The order (D95-A): `connect.js` `renderServers` draws the servers sorted by the
   name shown (`s.name`, else `hostOf` of its address) with `Intl.Collator`
   of the page's language, `sensitivity: "base"`, `numeric: true`;
   a server saved or renamed takes its place; `servers.json`'s order stays.
6. Back (taken as written, D93-A): a menu or a panel that opens (☰, a server's
   settings, Add a server, App settings) pushes a history entry of the page's own
   (`history.pushState`, the address unchanged) and takes it back as it closes
   by other means; `popstate` closes the open menu first, then the open panel;
   with nothing open there is no entry, and the adapter's Back leaves the app
   as today (`web.goBack()`, on while the WebView can go back).
   A server's settings whose Connection differs from what is stored ask once,
   in place, as Remove's question does (`confirmInPlace`):
   discard the changes, or stay, which pushes the entry again;
   while Login or Certificate is shown, Connection is shown first and asks
   there, the focus on the question, since `confirmInPlace` asks inside
   Connection's action row;
   a switch between the sections pushes no entry.
   The other ways the panel closes (another server's cog, a tap on a row,
   Add server, App settings) close it without asking, as today:
   D93-A names Back alone, and ✕ closes as Back does.
7. A server's settings in three sections (D111-A, D94-A, taken as written):
   `connect.js` `settingsPanel` becomes a panel with a head, the row of tabs and
   three sections, all built when it opens and hidden but the one shown,
   Connection first:
   - the head: the server's name and a labelled ✕ (`card-close`,
     `action.close`), which closes the panel as Back does, asking once where
     Connection differs, in Connection (step 6);
   - the row (`.server-tabs`, `role="tablist"`, named by `connect.sections`):
     Connection, Login and Certificate (`connect.tab.connection`,
     `connect.tab.login`, `connect.tab.certificate`), each a `.page-link`
     button with its name and no icon, `role="tab"`, `aria-selected` and
     `aria-controls`, the one shown filled (`.current`), built with
     `components.js`' `button`;
     `style.css` scopes `.page-link`'s folding below 900 px to `.page-switch`,
     so that the row keeps its names at 390 px, as `.login-way` does,
     and no name is cut short by `.page-link-label`'s "…";
   - **Connection** (`.server-section`, `role="tabpanel"`, the form):
     the name, the LAN and remote addresses, Open at start, notifications where
     the app has push, when the fingerprint is asked; the message line;
     Cancel and Save, off while the form holds what is stored, compared field
     by field at each input, as the server's forms are;
     then, after the rule, Remove server with its line (`fillRemove`, F34);
     the draft is the form's, so it outlives a switch;
     while it differs, Connection's tab carries the "changed here" mark,
     AP UnsavedMarks' disk in amber (`problem.unsaved` its name),
     drawn by a new builder of `components.js`, `signEl(kind, state)`,
     beside `markEl`, from `SIGNS`, as `src/shared/icon.tsx`'s `Sign` draws it,
     its row in `docs/ui.md` ("How the page is built");
   - **Login** (no form): what `fillFacts` writes of the session and the keys,
     in place and drawn again only where it changed:
     the session with Sign out, and while the server does not answer its line
     of D112-A (Block 21); the kept login with its key's id and Turn off;
     Store login (step 8); each key and session left or pending, with Show in
     My profile for a key; each acts at once, and none touches Connection's
     draft; while a key or a session is left or pending there, Login's tab
     carries a dot, named for a screen reader (`connect.tab.leftMark`);
   - **Certificate** (no form): `connect.html`'s `#cert-dialog` becomes the
     template `tpl-cert`, its rows the same, its ids turned into classes,
     cloned here and filled by `fillCert` (today's `showCert`), drawn again
     only where it changed: the address, the subject, the issuer or
     "self-signed", the names, the validity, the fingerprint and since when it
     is trusted; where the server does not answer, the pinned fingerprint and
     since when (`connect.certPinned`), or, where none is pinned, that it shows
     once the server answers (`connect.certWaiting`); over plain HTTP, that
     there is none (`connect.certPlain`);
     the dialog, Certificate…, its line and `#cert-close`'s listener go,
     with `connect.certTitle`, `connect.certButton` and `connect.certLine`.

   The toast that covered the foot at 390 px is looked at again.
8. Store login (taken as written): in the Login section, where the app holds a
   session there and keeps no login (`s.session`, not `s.login`),
   "Store login" calls the connect page's new `keep(id, url)` (version 7)
   at the probe's address;
   the bridge makes the key with the session held and asks the fingerprint once,
   as `login.keep` does for a server's page (`Bridge.kt` `keep`: the user from
   `GET /api/v1/me`, the key from `POST /api/v1/users/{name}/tokens`,
   sealed under the app key, its revoke code beside its id, D112-A),
   a prompt not passed revoking it again;
   off while the server does not answer, its title saying why, as Show in My
   profile is (`showsKeys`); never over plain HTTP outside the private ranges
   (D36-A); its toast says the login is kept.
   Under KeyLife.D5-A a key made by a session is no other key's heir, even where
   that session came from `POST /api/v1/login/key`, so the key Store login makes
   is not revoked with an older key of the phone's;
   under an idle limit (off by default) it stays valid while used,
   and with an end date it ends with the `401` the app already handles.
9. Pull down (taken as written): `MainActivity`: the WebView in a
   `SwipeRefreshLayout` (`androidx.swiperefreshlayout`,
   `app/android/gradle/libs.versions.toml`), on while the connect page is shown
   and scrolled to its top, which sends the page the event `refresh`
   (`Bridge.event`, version 7), the native spinner stopping as it is sent;
   `connect.js` `onBridgeEvent` runs what ↻ runs, `#probe-btn` turning meanwhile.
10. A page that fails to load (D108-A, F48):
    `MainActivity`'s `Client.onReceivedError` for the main frame of the opened
    server's origin drops the opening and shows the list with `#unreachable=<id>`
    (`Host.connectPage`, the bridge's handover, `Origins`),
    which `connect.js` takes at its start as `openFromAddress` takes
    `#settings=<id>`: that server probed at once, said not reachable with the
    probe's reason (`connect.unreachable`), its toast tied to the server so that a
    probe that finds it takes the toast back;
    Show in My profile (`fillFacts`) probes first, as `openServer` does for a
    server its last probe did not find, and opens My profile only where the
    server answers now, else the toast, the button turned off with its title;
    Turn off, Sign out or Remove that the server did not answer probe it again;
    a tap keeps opening on the last probe (D109-A).
11. D96-A on this menu: "App settings" without "…"; "Add server…" keeps it.
12. The texts: every new key in all six `ui/i18n/<lang>.json` (`connect.*`: Exit,
    the version row's name, Store login with its title and toast, the question
    of a panel with changes, the sections, their tabs and Login's mark, the
    certificate's three lines), FR/IT/NL/PL machine-translated and marked,
    and the keys that go removed from all six;
    `strings.xml` for Store login's prompt where `login_keep_title` does not serve.
13. The contract's version 7, after Block 21's 6
    (`docs/app.md`, `res/raw/bridge.js`, `Origins.kt`, `tools/ui-test/fake_bridge.js`):
    `about()`, `exit()`, `keep(id, url)`, the event `refresh`,
    the handover `#unreachable=<id>`.
14. Tests: `test_connect.py` over the fake bridge at version 7:
    ☰ in the header with its entries in order, the version row's tap copying the
    whole version (the fake bridge's clipboard) and closing ☰,
    Exit's call, none of the app's entries without its call;
    the cards wider than 640 px at 1100 and the page's width at 390;
    the servers in the collation's order (case, accents, "server 9" before
    "server 10"); `history.back()` closing ☰, then a panel,
    a panel with changes asking once, the bare list leaving no entry;
    the sections: Connection first, the tabs' roles and names, no name cut
    short at 390 px in any of the six languages, a switch keeping
    what is typed, Connection's mark while it differs and gone when it is undone,
    Back and ✕ from Login and from Certificate with Connection changed showing
    Connection with the question, the focus on it,
    Save and Cancel off until a change and off again when it is undone, ✕,
    Login's dot while a key or a session is left and gone when none is,
    Login's and Certificate's actions without a Save;
    `test_connect_certificate` reading the Certificate section in place of the
    dialog, with the pinned fingerprint while the server does not answer
    and the line of plain HTTP;
    the cases that use the panel's lines opening their section first;
    Store login's call, and off while the server does not answer;
    the `refresh` event probing every server;
    `#unreachable=<id>` marking its server, with the toast;
    Show in My profile probing first;
    each measured at 390 px in the six languages, the axe check on the panel.
    The emulator suite: Exit leaving the recent apps; Back from a panel and from
    the bare list; a page whose load fails, the rig's `serve` stopped after the
    probe, back at the list; Store login with one prompt and the key in the audit;
    the pull down; `SessionsTest`'s Turn off and Remove, and the cases Block 21
    adds, opening their section first.
    `OriginsTest.kt` for the new handover.
15. `docs/app.md` (the connect page: ☰, the version row, Exit, Back, the order,
    a server's settings and their sections, Store login, pull down, a page that
    fails to load; version 7), `docs/ui.md` ("In the apps"; `signEl` in "How the
    page is built"), `docs/user-manual.md` where it tells the list.
16. The fragments, under the headings COMMIT.md 5.2 gives them.

**Verification:**
`make ui`; `make ui-check`;
the touched UI test files (`test_connect.py`, `test_copy.py`) in the four cases,
at 1100 and 390 px;
`MASSIMO_ENV_FILE=/dev/null go test ./internal/server/...`
for the embedded files (`TestUIPage`, `TestUITranslations`, `TestUIScriptGlobals`);
`gofmt -l .` empty, `go vet` for linux, windows and darwin;
the Gradle tasks; `make app-emulator-test`; `GUIDELINES/shared/tools/check`.
The owner's phone check to propose at the block's completion gate,
each step after what it needs:

1. ☰ at the logo's right: the version row first, a tap copying the version and
   closing ☰; the entries in order, App settings without "…";
2. a password sign-in at A, its login kept and asked at each start,
   back to the list, Exit: the app gone from the recent apps;
   started again: the list; A's audit ending the session Exit dropped
   at that start's contact, by its code;
3. the phone turned to landscape: the cards wider than before;
   the servers sorted by name;
4. ☰ opened, Back: ☰ closed; A's settings opened, Back: closed;
   A's name changed, Back: the question, Stay, Back again, Discard;
   Back from the bare list: the app leaves;
5. A's settings: Connection first; Save and Cancel off until a change,
   and off again when it is undone; a name changed, Login opened and Connection
   again: the name kept, Connection's tab marked;
   Login shown, ✕: Connection shown with the question;
   Remove server under Connection's rule;
6. A's Login section: the session with Sign out, the kept login with its key's id
   and Turn off; A's Certificate section: the facts the dialog showed, in place;
   A stopped: the pinned fingerprint;
7. a password sign-in at B with "Enable login by fingerprint" off, back to the
   list, Store login in B's Login section: one prompt, the login kept;
   B's page: the new key tagged "this device's key" under My profile;
8. the list pulled down: every server checked again, as ↻ does;
9. A's `serve` stopped: within seconds, Show in My profile: the toast "not
   reachable", no WebView error page; a tap on A's row: back at the list,
   A not reachable, the reason said.

### Block 20: The menu and the cards

The points taken as written for the server's page,
D96-A, D97-A, D98-A, D104-A and D107-A
(the Analysis, "The menu and the cards"),
in one pull request on `claude/mobile-menu-cards`, from `main`, for 0.18,
after Blocks 21 and 19, the last of the three:
it waits on D115 and on AP ErrorCodes' Block 6 (metaworx/massimo#489).
Its parts are shared, so the server's page in a browser changes too:
whoever changes `src/chrome/`, `src/signin/`, `src/shared/`, `src/profile/`,
`components.js` or `login.js` then is told before the block starts,
and whichever lands second merges `main` first.
AP KeyLife v1.0's Block 1, on `main` since metaworx/massimo#477,
shows a key once with the shared copy part, so D97-A reaches that box too.
AP ErrorCodes' Block 6 (metaworx/massimo#489, open) turns `copied` into
`showBubble(text, ctrl)`, the word above the control,
where D97-A puts the page's toast: D115 asks which, before this block starts,
and that plan's session is told of the answer.
The list's version row (Block 19) calls `copied()`, so this block's copy word
reaches it too.

1. The server's ☰ (taken as written, `src/chrome/Menu.tsx`):
   the version row, the pages, Personal settings, a rule, App settings,
   This server's settings, Select server, a rule, Sign out (or Create the owner…);
   in a browser, without the three app entries, the two rules draw as one,
   the second left out while none of the three shows.
2. D96-A: no "…" on Select server, This server's settings and App settings
   (`app.selectServer`, `app.serverSettings`, `app.appSettings`),
   in ☰ and in the card's ☰.
3. The version row (`VersionRow`, `src/shared/server.tsx`):
   a tap copies the whole version, shows the copy's toast and closes ☰
   (`Dropdown`, `src/shared/menu.tsx`); the row keeps its copy icon
   (`Copy` with `icon`).
4. Every copy's toast (D97-A): `copied` (`components.js`), which the parts'
   `Copy` and `CopyIcon` and the plain scripts' `copyable`, `copyIcon` and `copyEl`
   go through, shows the page's toast "Copied" (`copy.done`, `showToast`),
   or `copy.failed`'s words in red, in place of the word drawn at the element
   (`data-copied` and its CSS go); the hidden live region `#copy-status` goes,
   the toast being a polite live region of its own (`partials/toast.html`,
   and its copy in `connect.html`); on the connect page as on a server's.
   With D115-A, where AP ErrorCodes' Block 6 has landed, the same through
   `showBubble` without a control, at the window's foot, in place of the toast.
5. The cards' server line (taken as written, `src/signin/Server.tsx`, `Card.tsx`):
   `ServerLine` above `#login-title`: the name, then the address and ⓘ in one
   element that does not break between them;
   on the sign-in, the first owner's form and Keep it open alike;
   `test_keep_open.py` still reads the title alone.
6. ⓘ's facts (taken as written): the instance id and the fingerprint,
   each a `Copy` with its icon.
7. The first owner's command (taken as written, `src/signin/Owner.tsx`):
   `setup.text` without the command, which follows on a line of its own,
   `massimo user add` in monospace, never wrapped,
   copied by a tap or by its icon (`Copy` with `icon`).
8. The password pair (taken as written, D98-A; `src/shared/password.tsx`
   `NewPassword`): one eye, on the first field, showing and hiding both,
   the second's input without one;
   the repetition outlined in red (`aria-invalid`) while it is not empty and
   differs from the first, "The two passwords differ." under it
   (`setup.mismatch`, reworded), both gone as they match;
   on the first owner's form and My profile's change of password, its two users;
   `auth.password_reveal` turns this eye off as every other.
9. Sign out in the app (taken as written, `login.js` `logout`):
   where the page runs in the app (`appConnect()`), once the session is ended
   and forgotten, the app's list shows (`connect()`) in place of that server's
   sign-in card; in a browser as now.
10. My profile's failures (D104-A, F45; `src/profile/send.ts`):
    a failure `profileMsg` says also shows in the page's toast, in red;
    the card's line keeps it after the toast goes.
11. The name trimmed (D107-A): the trailing space autofill leaves stays in the
    field; every sign-in path trims it: the card's password and key ways
    (`Card.tsx`), the first owner's name (`Owner.tsx`),
    and the server's `POST /api/v1/login`, OPAQUE's start and a registration
    (`store.NormalName`); each checked, and trimmed where one does not.
12. The texts: every new or changed key in all six `ui/i18n/<lang>.json`,
    FR/IT/NL/PL machine-translated and marked.
13. Tests: `test_main_menu.py` (the order in the app and in a browser, one rule
    there; the version row's tap: copied, the toast, ☰ closed);
    `test_connect.py` (the list's version row: the same word once ☰ closed);
    `test_copy.py` and `test_settings_copy.py` (the toast in place of the word);
    `test_login.py` and `test_first_owner.py` (the line above the title, the
    address and ⓘ on one line at 390 px in the six languages, the facts' icons,
    the command copied by a tap and by its icon);
    `test_password_eye.py` (one eye for the pair, both shown and hidden;
    the red outline and its line);
    `test_connect.py` on a server's page (Sign out to the list through the fake
    bridge's `connect`);
    `test_users.py` or `test_profile_help.py` (a failed Revoke in the toast and
    the line); `test_opaque.py` (a name with a trailing space signs in);
    a Go test of each sign-in route with a trailing space;
    the emulator suite: Sign out in a server's page landing on the list.
14. `docs/ui.md` (☰, the sign-in and first owner's cards, copying,
    the new password, My profile), `docs/app.md` ("How the page uses it":
    Sign out), `docs/user-manual.md`.
15. The fragments, under the headings COMMIT.md 5.2 gives them.

**Verification:**
`make ui`; `make ui-check`;
the touched UI test files in the four cases, at 1100 and 390 px;
`MASSIMO_ENV_FILE=/dev/null go test ./internal/server/...`,
`TestSharedPartsKnowNoRegion` among them;
`gofmt -l .` empty, `go vet` for linux, windows and darwin;
the Gradle tasks; `make app-emulator-test`; `GUIDELINES/shared/tools/check`.
The owner's phone check to propose at the block's completion gate,
each step after what it needs:

1. A's ☰: the order, the two rules, no "…";
   the version row's tap copying with the toast and closing ☰;
   the list's version row the same;
   at 390 px on Android 13 or later, the copy's word beside the system's
   clipboard preview at the screen's foot, both readable;
2. A's sign-in card: the server's line above the title,
   the address and ⓘ on one line; ⓘ's two facts copied by their icons,
   the toast each time;
3. C's first owner's card: the command on a line of its own,
   copied by a tap and by its icon;
4. C's password pair: one eye showing and hiding both;
   the repetition red while it differs, the line under it; the owner made;
5. A's card, the password manager's autofill filling the name with its space:
   the sign-in passes;
6. Sign out in A's page: the app's list;
7. A frozen, a Revoke under My profile: after the limit the failure in the
   toast, and in the line once the toast has gone;
8. a browser at A: ☰ with one rule where the app's entries are not.

### Block 21: The sessions

D100-A, D102-A, D103-A's two test cases, D105-A and D112-A's app half in the app,
D106-A and D112-A's server half in the server
(the Analysis, "The sessions held", "A logout the server refused",
"The list while a server does not answer", "A cancelled request at the server"
and "The gate of v1.5"),
in one pull request on `claude/mobile-sessions-held`, from `main`, for 0.18,
the first of v1.6's three;
its first two commits the server's, D106-A's and D112-A's, each of its own
(D113-A).
Before it starts the session "Open Items" is told (D113-A), with the list of
what AP KeyLife's blocks would change (the Analysis, "AP KeyLife meets it"),
and so are AP ServerBackup's session (the migration's number) and AP ErrorCodes'
(`userFail`); the block takes their blocks as `main` then has them.

1. A cancelled request (D106-A, F46): `internal/server/users_api.go`:
   `userFail` answers a context that ended with words of its own,
   "the client went away before the answer", and no longer hands it to
   `passwordBusy`; the routes that check or hash a password keep the password's
   answer, `POST /api/v1/login` calling `passwordBusy` itself (`login_api.go`)
   and a password set, a user added with one and a registration through a helper
   of their own beside `userFail`; the audit's result stays `failed`.
   A Go test (`users_api_test.go`): a token's revoke whose request is cancelled
   before the store answers is audited with the new words, not the password
   check's; a password check or hash that waited stays `503` as before.
   `docs/api-v1.md` where it says what `503` means.
   AP ErrorCodes' metaworx/massimo#488 changes `userFail`, `passwordBusy`
   (then `codePasswordBusy`) and `loginMode`: whichever lands second adapts.
2. The revoke code at the server (D112-A), a commit of its own:
   - `internal/store/migrations/<NNNN>_token_revoke.sql` and its down part,
     the number after `main`'s last when the pull request is opened:
     0100 where none of metaworx/massimo#481, #482 and #492 has merged,
     renumbered at the merge of `main` where one merges first
     (`loadMigrations` refuses a gap);
     `tokens.revoke_hash`, TEXT, and a unique index on it where it is not NULL,
     `min_reader` 24, additive, as `0025_token_parent.sql`;
     `store.sql` written again (`TestStoreSQL`);
   - `internal/store/tokens.go`: `RevokeCode(secret)`, the hex of
     SHA-256(`"massimo revoke v1"` ‖ secret); `revoke_hash`, `HashToken` of the
     code, written by `addToken` and `AddKeySession` (and by KeyLife's `AddKey`
     and `RekeyToken`, where they have landed: the list in the Analysis,
     "AP KeyLife meets it"); none by `AddTokenHash` nor by the bootstrap
     (`keyIn`); `Token.RevokeKept`, read with the row;
     `TouchToken` takes a revoke hash and keeps one already written;
     `RevokeByCode(ctx, code, keep)`, which revokes through `RevokeToken` the
     live token whose `revoke_hash` the code's hash is, but a token of the hash
     `keep`, the settings file's, and returns what it matched;
   - `internal/server/auth.go`: `resolve` hands `touch` the secret,
     and `touch` writes the revoke hash of a row without one at that use,
     whatever `touchEvery` says, but for the settings file's key
     (`settingsKey`'s rule), which gets none and is touched at most once an
     hour as before;
   - `POST /api/v1/tokens/revoke` (`server.go`, `none`), beside
     `deleteMyToken` in `login_api.go`:
     without a store, `204` at once;
     the address's lockout (`lockedOut`, its address part), `429` with
     `Retry-After`;
     the body `{"code"}`, 64 lower-case hex digits, else `400` with the new code
     `revoke_code_malformed` (`errcodes.go`, its six texts
     `err.revoke_code_malformed`);
     `RevokeByCode` with the settings file's hash as `keep`;
     `204` in every other case, and no count toward the lockout, matched or not;
     its action `token.revoke_code` (`actions`, in `noCommand`, a `change` line in
     `changeLines`, its six texts `log.action.token.revoke_code`):
     a match writes its row, the token as its target and its user as its
     subject, its change line and an Info line "by code";
     a code that matches nothing, one of the settings file's key and a malformed
     body write no row of their own (`auditInfo.skip`), and are counted per
     address and minute as refused requests are (`refusals.note`),
     with no change line and no Info line; no row nor line carries the code;
   - Go tests: `tokens_test.go` (the code of a fixed secret, the vector the app's
     test shares; the column written at the making of a session, a key and a
     key's session, none for `AddTokenHash`; `RevokeByCode` revoking the live
     token and a key's sessions, nothing for an ended token, an unknown code or
     the hash `keep`); `bootstrap_test.go` (the file's key, new and taken from an
     integration's user, gets no `revoke_hash`);
     the migration's (`TestMigrationFilesNamed`, `TestStoreSQL`,
     `TestMigratesReleasedStores`, `TestDowngradeFromTheStoresDownParts`);
     a route test of its own, `revoke_code_test.go`: a session revoked by its
     code and refused after, a key revoked with its session, `204` for a code
     that matches nothing and for one taken already, the token in the store
     unchanged then; a key made by `POST …/tokens` and then named in the settings
     file left live by its code, `204`; `400` for a malformed body with its code;
     `429` while the address is locked out, no failure counted;
     30 posts of codes that match nothing making one row of the minute,
     no row of `commands` and no line of the log;
     a match's row, change line and Info line, none of them with the code;
     a server without a store answering `204`;
     the fill at first use: a token made without the column revoked by its code
     after one request; the settings file's key never given one, and touched at
     most once an hour (no bypass of `touchEvery`);
     `TestOpenAPICoversRoutes`, `TestAuditOfEveryWrite`,
     `TestEveryActionHasALine`, `TestErrorCodes`;
   - `docs/api-v1.md` (beside `DELETE /api/v1/me/token`: the route, its `204`,
     its `400`, the lockout, what is audited; the row of
     `revoke_code_malformed` in "Error codes"), `docs/api-v1-openapi.yaml` with
     `make api-types`, `docs/store.md` (the column in section 5's `tokens`, beside
     `parent_id`; the migration among the migrations).
3. The sessions with the process (D102-A): `MassimoApp`, an `Application` of the
   app's own named in the manifest, holds the sessions kept in memory
   (`SessionKeeper`) and what was checked of them (`Sessions.checked`, moved into
   a holder the activity's `Sessions` takes);
   `MainActivity.onCreate` takes both from it, so that an activity made anew
   (the phone's dark mode or language, the app's Appearance) keeps them;
   `configChanges` stays as it is, AppCompat's night mode needing the new activity.
4. The session held, recorded (D102-A, D112-A): `ServerStore.kt`: a server's
   `held`, `{user, id, created_at, code}` of the session the app holds there,
   its code worked out from the secret, written where a check learns its id
   (`Sessions.look`), and only where the check found a session
   (`Checked.kind` `session`): a key the user typed into the card, which the page
   holds as its token, gets no `held` and no code;
   cleared where the session ends, goes into `unended` or is replaced;
   dropped where the node changes, as `unended` is;
   kept in the file alone (`fileJson`), never given to the page.
   At the process's start (`MassimoApp.onCreate`, before any activity)
   each `held` whose session the app no longer holds, none in memory,
   moves into `unended` with its code; a sealed session, still held, stays;
   Exit (Block 19) moves them so at once.
5. The revoke codes in the app (D112-A): `RevokeCode.kt`, `RevokeCode.of(secret)`;
   - the kept login's code in its record beside `keyId` (`LoginStore.kt`
     `KeptLogin`), written at the key's making (`AppKey.kt` `keep`) and at an
     unseal where it is missing (`LoginStore.open`), so that a login kept before
     this version has one from its next unseal;
     none for a key the user typed into the card;
   - `KeyLeft` and `SessionLeft` carry an optional `code` and `taken`:
     `Sessions.leave`, `Revoker`'s keys left, `forgetLeavingKey` and `Remove`
     write the code they have;
   - `ServerApi.revokeCode(base, fp, code)`: `POST /api/v1/tokens/revoke`
     without `Authorization`, the pinned fingerprint as for every call;
   - the connect page's `session(id, url)` posts the server's codes at that
     address before its check (`Sessions.look`), and `open` at the address it
     opens (a server opened at the start, D43-A), in the background:
     `204` marks the entry `taken` and drops its code; `404` drops the code, the
     entry staying as today; `400` drops it, logged; a broken connection, `429`
     and `5xx` keep it; `KeysLeft.revoke` drops a `taken` entry its list no longer
     has, and revokes one it still has;
   - any answer but `204`, `400`, `429` and `5xx` (`404` first, and `401`,
     `403`, `405`, `409`, a redirect, a proxy's page of `200`) is a server that
     takes no codes: the code dropped, the entry staying as today;
   - Remove: the server's codes posted where it answers, and those still to post
     kept in `gone`, a file of its own (`gone.json`,
     `{id, node, lan, remote, fp, codes}` per server),
     its addresses probed by the adapter at the app's start and each time the
     list is shown, and the codes posted only where the probe finds the server
     at its node (`Addresses.answering`);
     each dropped on `204` or on a server that takes none,
     a session's 30 days after it was left,
     every code 90 days after the Remove at the latest (D116-A);
   - `held` and every code go to the file alone (`fileJson`); `toJson`, which the
     page and Remove's answer see, carries `pending: true` on an entry with a
     code, no code, no `held` and no `taken` entry.
6. A fingerprint tap's session checked at once (D102-A, F43): `Bridge.kt`
   `openKept` checks the session it made (`Sessions.look`, one `GET /api/v1/me`)
   as `setToken` checks a page's, its id and code learnt before the page opens,
   the keys and sessions left there revoked as at any check.
7. `Sessions.look` logs what it swallows (D102-A): a server's failure and a broken
   connection go to the log as `end` writes them (`what`), without any secret,
   and so does a code's post, without the code.
8. No logout the server refused (D100-A, F41): `Sessions.kt`: `Checked` gains the
   key a session was made from, `GET /api/v1/me`'s `token.key_id`
   (`ServerApi.Me.keyId`), or the kept key's id where `login.fingerprint` made it;
   `put` drops a session that one made from the same key replaces without
   `POST /api/v1/logout`, the server having ended it when it made the new one
   (D31-A); `revokeWithSession` forgets without a logout a session made from the
   key it revoked, which ended with it (Turn off, Forget all, Remove);
   neither goes to `unended`, and a password session replaced is logged out as
   before.
   AP KeyLife v1.0 adds ways the server ends a session without the app's logout:
   a key reaching its end date or the idle limit ends the sessions made from it,
   and a re-key ends the key's sessions but the caller's own;
   D100-A's "the server ended it itself" covers these too, at the same `401`,
   which forgets the session with no logout sent (`Sessions.look`,
   the page's `setToken(null)`).
9. D103-A's two test cases (F44; no fix):
   in the emulator suite, Turn off with the rig's `serve` stopped, the key left;
   the server started, Show in My profile, a password sign-in:
   the toast `profile.keyRevokedAtSignIn` over My profile, and the key not marked;
   in the UI suite, `test_connect.py` on a server's page, the fake bridge's
   `setToken` answering `{revoked: [id]}` at once, before My profile reads its
   list: the toast, and no mark.
   Each seen failing with the toast's call undone.
   With D112-A a key left with its code is taken at the probe that finds the
   server, before the sign-in, and the toast then has nothing to say:
   the emulator case keeps its login as one kept before this version,
   its record without a code, so that the sign-in still revokes the key,
   as the toast needs; a second case lets the code take it, and no toast shows.
10. The list while a server does not answer (D105-A, F42; D112-A):
    `connect.js` `fillRow`: the badge "signed in as …" grey while the probe's
    state is neither `lan` nor `remote`; `fillFacts`: the session's line says
    then that Sign out ends the session there the next time the app reaches the
    server (`connect.sessionDown`); a key or a session left `pending` says it
    ends then (`connect.keyPending`, `connect.sessionPending`), one without a
    code in today's words; the toasts of Sign out, Turn off and Remove that left
    a code say so (`connect.signedOutPending`, `connect.loginOffPending`,
    `connect.removedPending`); `checkSession` reads the list again where the
    server had a code pending, so that a line taken goes;
    `fillFacts`' key (`connect.js:616`) gains whether the server answers,
    which over plain HTTP nothing else in it tells;
    all drawn again as the state changes.
11. The texts: every new key in all six `ui/i18n/<lang>.json`,
    FR/IT/NL/PL machine-translated and marked.
12. The contract's version 6 (`docs/app.md`, `res/raw/bridge.js`,
    `tools/ui-test/fake_bridge.js`), the first of v1.6's two, Block 19 taking 7:
    a server's `held`; `unended` fed at a new process;
    an entry's `pending`; the answers of `signOut`, `forget`, `forgetAll` and
    `remove` saying what waits for its code; a fingerprint tap's session checked.
13. Tests: the JVM tests: `SessionKeeperTest.kt` and `SessionsTest.kt`
    (one holder over two activities' `Sessions`; `held` written at a check with
    its code, cleared at an end, moved into `unended` with its code at a new
    process with nothing in memory, kept where sealed; `Checked`'s key; no logout
    for a session the same key replaced or the revoked key ended; `look`'s log,
    without a secret or a code),
    `ServerStoreTest.kt` (`held` and the entries' `code` and `taken` kept by
    `save`, dropped with the node; `toJson` and Remove's answer with no `held`,
    no code and no `taken` entry, and `pending: true` where a code waits;
    `gone.json` kept, and a code dropped after 30 days for a session and 90 days
    after the Remove for any);
    `SessionsTest.kt` also: a key typed into the card and checked gets no `held`
    and no code, and nothing of it lands in `unended` after a new process or
    after Exit;
    `KeysLeftTest.kt` (a session moved from `held` revoked at the next session;
    a `taken` entry dropped where the list has it not, revoked where it has it),
    `RevokerTest.kt` (Turn off, Forget all and Remove leaving their codes),
    `RevokeCodeTest.kt` (the vector of `tokens_test.go`),
    `ServerApiTest.kt` (`revokeCode` without `Authorization`; `204`, `400`,
    `429`, `5xx` and a broken connection, and `401`, `403`, `405`, `409`, a
    redirect and a page of `200` taken as `404`),
    a JVM case of `gone`: a probe that finds another node at a removed server's
    address posts no code,
    `LoginStoreTest.kt` (the code beside `keyId`, written at an unseal);
    the emulator suite: a password sign-in at a server asked at each start,
    the process ended after Home (`am kill`, as Android does), the app started,
    and the first session ended in the audit at that start's contact, by its code;
    the night mode turned on (`cmd uimode night yes`) while signed in,
    and still signed in; a fingerprint tap, the rig's `serve` stopped at once,
    Turn off, the server started, ↻, and the key and the session ended by their
    codes; Sign out with the server stopped, the server started, ↻, the session
    ended; Remove with the server stopped, the server started, the app started,
    the session ended; Turn off with a session made from the key, and no refused
    `logout` in the audit; D103-A's case;
    the UI suite: D105-A's and D112-A's lines in `test_connect.py` at 390 px in
    the six languages, over HTTPS and over plain HTTP (the line of a server down
    shown there too), a pending entry taken at a probe, and D103-A's case.
14. `docs/app.md` ("The login by fingerprint": the sessions held, a process that
    ends, Exit, the revoke codes and `gone`; version 6), `docs/ui.md` ("In the
    apps": the grey badge, the session's line, the pending lines),
    `docs/api-v1.md` (steps 1 and 2), `docs/user-manual.md`
    (what Sign out, Turn off and Remove do while a server does not answer).
15. The fragments, under the headings COMMIT.md 5.2 gives them:
    the server's of their own, D106-A's under Fixed, D112-A's under Added.

**Verification:**
`make ui`;
`MASSIMO_ENV_FILE=/dev/null go test ./internal/server/... ./internal/store/...`;
`go test -race -count=1 -timeout 30m ./internal/server/ ./internal/store/`;
`gofmt -l .` empty, `go vet` for linux, windows and darwin; `make cross-check`;
`make ui-check`; the touched UI test files (`test_connect.py`) in the four cases,
at 1100 and 390 px;
the Gradle tasks; `make app-emulator-test`; `GUIDELINES/shared/tools/check`.
The owner's phone check to propose at the block's completion gate,
the server's audit read after each, each step after what it needs:

1. at A, asked at each start: a password sign-in; the app swiped from the recent
   apps and started again: the first session ended within seconds, by its code
   (F47);
2. at A, asked at each start, signed in: the phone's dark mode turned on,
   then its language changed: still signed in each time, no prompt;
3. at A, asked at each return: a fingerprint tap; within seconds A's `serve`
   stopped, then Turn off; A started, ↻: the key and the session ended,
   no sign-in needed (F43);
4. at B: two fingerprint logins one after the other, then Turn off while signed
   in: no refused `logout` in B's audit (F41);
5. A frozen while a session is held: the list's "signed in as …" grey;
   the settings' session line saying Sign out ends it the next time the app
   reaches A; Sign out; A thawed, ↻: the session ended, its line gone;
6. A frozen, a Revoke under My profile given up after the limit; A thawed:
   its audit says the client went away, not "too many password checks" (F46);
7. A stopped, Remove A with its login kept; A started, the app started again:
   the key and the session ended, A no longer in the list.

## Proposed commit message

The revision's registration, a commit of its own on `main`:

```
chore(aps): +MobileApp v1.6

At v1.5's gate the owner proposed a code that only revokes a token the app forgets, and sections in a server's settings; with the decisions D109-A to D114-A, taken as recommended, Block 21 builds the revoke code, its server half in a commit of its own, and Block 19 the sections, the certificate dialog's facts in place. A review before the revision was presented is folded in: D116-A, taken under PLAN++, bounds a removed server's codes to 90 days, and the blocks run 21, 19, 20. D115, found writing it, asks before Block 20 how a copy's word meets AP ErrorCodes' bubble.

Key changes:
- GUIDELINES/wip/2026-11-09_02-30_AP_MobileApp_android_ios_apps.md: v1.6: D109-A to D114-A and D116-A recorded, D110 settled by D112-A, D115 open; the revoke code and the sections in the Analysis, read in the code, with the migration's number, what the route writes and what AP KeyLife's blocks would change; Block 19 with the three sections, Block 21 with the revoke code's server commit and app half and their tests; the order 21, 19, 20; the contract's versions 6 and 7; AP KeyLife named by its path; sessions bound to the phone among the plans to come.

File-revision-v1: GUIDELINES/wip/2026-11-09_02-30_AP_MobileApp_android_ios_apps.md::v1.6::2026-11-28::TL;DR, Discussion, Analysis, Implementation Plan, Proposed commit message, Change History::revoke code and sections in a server's settings
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits still to come:

1. `feat(app): The server as app/core, bound by gomobile`
7. `feat(app): The Android app runs the server itself`
9. `feat(app): The iOS app`
10. `docs(app): The apps' documents`
11. `feat(app): Push through UnifiedPush`
19. `feat(app): The list's ☰ in its header, with Exit, Store login and pull down`,
    `feat(app): A server's settings in three sections, its certificate in place`
    and `fix(app): A page that fails to load returns to the list`
20. `feat(ui): The menu's app entries last, every copy in the toast, one eye for a new password`
21. `fix(server): A cancelled request is not a password check that waited`,
    `feat(api): POST /api/v1/tokens/revoke ends a token by its revoke code`,
    then `fix(app): A session the app held is ended after its process, and no logout the server refused`
    and `feat(app): A token the app forgets is ended by its code at the next contact`

Blocks 5 and 6 commit nothing but `app/README.md`'s sections.

## Change History

- v1.0 (2026-11-09): the first revision, from the TL;DR gate of items 16
  and 17 and the owner's additions; D1 to D3 open at its gate.
- v1.1 (2026-11-18): the login by fingerprint of the owner's design at the phone test,
  as Block 8, with a session naming its key and the user's choice of when
  the fingerprint is asked; Push moved to Block 11;
  the phone test's findings as Block 12, with Dependabot for Gradle;
  Blocks 2, 3 and 4 marked done, D1 to D21 restated as decided;
  two slips of v1.0 corrected;
  D22 to D31 decided at its gate, D32 open for Block 8's.
- v1.2 (2026-11-20): Blocks 8 and 12 done; the owner's check of the login
  on the phone, its findings as Block 13 with the decisions D40 to D53
  (D38 to D51 at the check, renumbered after Block 12's D38 and D39);
  D32 to D39 recorded as decided.
- v1.3 (2026-11-21): Block 13 done; the owner's check of it on the phone,
  its findings as Block 14 with the decisions D54 to D57.
- v1.4 (2026-11-23): Block 14 done; the owner's check of it on the phone,
  its findings F29 to F40 as Blocks 15 to 18, with the decisions D58 to D83,
  D84 open at its gate;
  the look at metaworx/massimo#397's card folded into Block 15's check (D83-A);
  Block 7's step for the phone's own server kept open (AP FirstOwner, D18-A);
  My profile's keys named as AP KeyLife, to come;
  the table of files pruned of the blocks done,
  and the stale line on one credential for both addresses dropped from
  "Not in this plan", Block 8 having made it (D34-B).
- v1.5 (2026-11-27): Blocks 15 to 18 done, and the decisions taken since,
  D84-A with its switch and D85-A to D89-A;
  the owner's check of Blocks 15 to 17 on the phone and the run that looked again
  at F44, their findings F41 to F48 with their causes, as Blocks 19 to 21,
  with the decisions D90-A to D107-A and D108-A, taken as recommended,
  the owner having named no option, and the points taken as written;
  D109 to D111 open at its gate;
  D75, D76-A to D79-A, D81-A and the two gaps marked as taken over by
  AP KeyLife v1.0, D80-B staying this plan's;
  the table of files pruned of Blocks 15 to 18;
  every NOTE of the plan folded in, its substance in the body,
  how each built block went under its "As built",
  and its file removed: the run NOTEs too, at the owner's word,
  against AGENTS.md 5.4, which folds a NOTE recording a decision alone.
  The NOTEs folded:
  "NOTE MobileApp v1.0: gate decisions",
  "NOTE MobileApp v1.0: block 2 connect",
  "NOTE MobileApp v1.0: block 3 shell",
  "NOTE MobileApp v1.0: block 3 on main",
  "NOTE MobileApp v1.0: phone test",
  "NOTE MobileApp v1.0: block 4 refit",
  "NOTE MobileApp v1.0: how block 4 went",
  "NOTE MobileApp v1.0: first signed nightly",
  "NOTE MobileApp v1.1: block 8 decisions",
  "NOTE MobileApp v1.1: how block 12 went",
  "NOTE MobileApp v1.1: how block 8 went",
  "NOTE MobileApp v1.1: phone check block 8",
  "NOTE MobileApp v1.2: how block 13 went",
  "NOTE MobileApp v1.2: phone check block 13",
  "NOTE MobileApp v1.3: how block 14 went",
  "NOTE MobileApp v1.3: phone check block 14",
  "NOTE MobileApp v1.4: gate decisions",
  "NOTE MobileApp v1.4: how block 17 went",
  "NOTE MobileApp v1.4: how block 16 went",
  "NOTE MobileApp v1.4: how block 15 went",
  "NOTE MobileApp v1.4: how block 18 went",
  "NOTE MobileApp v1.4: compile sdk 37",
  "NOTE MobileApp v1.4: settings key and emulator gate",
  "NOTE MobileApp v1.4: settings key user",
  "NOTE MobileApp v1.4: phone check blocks 15 to 17",
  "NOTE MobileApp v1.4: decisions after the phone check",
  "NOTE MobileApp v1.4: phone check F44".
- v1.6 (2026-11-28): v1.5's open points decided at its gate, each as recommended:
  D109-A, a tap stays immediate; D110 settled by D112-A, D105-A's words holding
  where the server knows the revoke route; D111-A revised, a server's settings in
  three sections, Connection, Login and Certificate, the certificate dialog
  removed; D112-A, a revoke code for a token the app forgets, with
  `POST /api/v1/tokens/revoke` and `tokens.revoke_hash`; D113-A, Block 21 builds
  it, its server half in a commit of its own, the session "Open Items" told first;
  D114-A, sessions bound to the phone left to a plan of its own;
  the Analysis of both read in the code, with the migration's number;
  Blocks 19 and 21 rewritten for them, Block 20 touched where AP KeyLife's
  Block 1 has landed and AP ErrorCodes' bubble meets it;
  the bridge's contract versions 6 and 7 in the order the blocks run;
  AP KeyLife v1.0 named by its path;
  D115 open, found writing it: a copy's word, D97-A's toast or AP ErrorCodes'
  bubble.
  An independent review before the revision was presented, folded in:
  D116-A, a removed server's codes dropped after 90 days at the latest,
  taken under the owner's `PLAN++`; the order 21, 19, 20, Block 20 last;
  a key typed into the card given no code; the route writing a row, a change line
  and an Info line only for a match, the rest counted per minute, in an action of
  its own, `token.revoke_code`; the settings file's key left live by a code;
  the migration's number as `loadMigrations` takes it; a server without a store;
  every other answer taken as `404`; `held` and the codes in the app's file
  alone; the session's line over plain HTTP; the bootstrap's key without a code;
  `gone` in a file of its own and posted where the probe finds its node;
  Back and ✕ from Login or Certificate; what AP KeyLife's and AP ErrorCodes'
  blocks meet; the missing tests and documents; the line references of today's
  `main`.
