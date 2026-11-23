# AP MobileApp v1.4: the Android and iOS apps

## TL;DR

Roadmap items 16 and 17: an app for Android, on F-Droid and Google Play,
and one for iOS from the same source: the web UI in a thin native shell,
connected to a `serve` or with one embedded.
On `main`: the connect page, the Android shell, the release's jobs,
the login by fingerprint and two phone checks' fixes;
v1.4 adds the third check's findings.

- a tap → the fingerprint where a login is kept; cancel → the card
- the card → the server's name, address, ⓘ; a password's eye, D84 open
- ☰ → this server's, the app's settings
- My profile → "this device's key", every key's id; no second card
- a session forgotten → signed out, now or later; a token → its own revoke
- Forget all → one prompt; an ended token → no lockout
- an emulator suite → locally and in CI
- expiry, re-key → AP KeyLife, later
- first release → the client with the login; the core second

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
    the password's eye (D84)
16. Sessions and keys: no second card, this device's key and the ids,
    sessions signed out later, a token's own revoke (D80-B), a time limit
17. One prompt and the lockout: one wrapping key for Forget all,
    an ended token not counted
18. Emulator: an instrumented suite, `make app-emulator-test`, a CI job

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
- **D1-A** the application id `ch.metaworx.massimo`, **D2-A** `minSdk 26`
  with 24 in reserve, **D3-A** the app released with the server's tag
  (the NOTE of the plan gate);
- **D4-A** on a server's page the bridge has `token()` and `setToken(t)` without an id,
  checked by origin,
  **D5-A** the axe check in the UI suite (the NOTE of Block 2);
- **D6-A** the bridge is messages, not `@JavascriptInterface`,
  **D7-A** the Gradle wrapper is committed, **D8-A** `languages` gains `xml`,
  **D9-A** the icon is Massimo's mark,
  made by `make ui-brand` (the two NOTEs of Block 3);
- **D10-B** the phone test's findings in the page wait for this revision,
  **D11-A** this revision is the planning session's, and Block 8 becomes the
  login by fingerprint (the phone test of 2026-11-17);
- **D12 to D21** at Block 4's gate of 2026-11-17: the version file,
  the builds that carry the app, where it is signed,
  the sums and the attestation, the keys, the nightly app, the versionCode
  (`2026-11-17_18-27_NOTE_MobileApp_v1.0_block_4_refit.md`);
- **D22 to D31** at this revision's gate of 2026-11-18, below.

**Revision v1.4 (2026-11-21).**
Block 14 is on `main` (metaworx/massimo#390).
The owner checked it on the phone on 2026-11-21
(`2026-11-21_21-24_NOTE_MobileApp_v1.3_phone_check_block_14.md`):
it passed, F28 is settled,
and it found twelve points, F29 to F40,
which this revision gathers as Blocks 15 to 18.
The owner's decisions on them and on what the check raised are D58 to D83,
D84 open at this revision's gate;
those on My profile's keys, D75 to D79 and D81,
are for AP KeyLife, a plan to come.
AP FirstOwner's D18-A gives Block 7 a step.
How Block 14 was built is
`2026-11-21_03-25_NOTE_MobileApp_v1.3_how_block_14_went.md`.

**Revision v1.3 (2026-11-21).**
Block 13 is on `main` (pull requests 384 and 386).
The owner checked it on the phone on 2026-11-20
(`2026-11-20_23-04_NOTE_MobileApp_v1.2_phone_check_block_13.md`):
it passed, and found four points,
which this revision gathers as Block 14 with the decisions D54 to D57.

**Revision v1.2 (2026-11-20).**
Blocks 8 and 12 are on `main` (pull requests 365, 366 and 368).
The owner checked the login by fingerprint on the phone on 2026-11-20
(`2026-11-20_20-05_NOTE_MobileApp_v1.1_phone_check_block_8.md`):
it works, and the check found 24 points, two not reproduced,
which this revision gathers as Block 13 with the decisions D40 to D53.

**Revision v1.1 (2026-11-17 and 2026-11-18).**
Blocks 2 and 3 are on `main`: the connect page with the bridge's contract,
`docs/app.md`, and the Android shell in client mode.
On 2026-11-17 the app passed the plan's check on the owner's phone,
once one fault was fixed: discovery, a pinned self-signed certificate,
the sign-in by OPAQUE, the token across a restart
(`2026-11-17_14-17_NOTE_MobileApp_v1.0_phone_test.md`).
At that test the owner stated how the login is to work,
in place of the switch that guards the session token:
ten points, the NOTE's last section, which this revision turns into Block 8.
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

**Decided at Block 8's gate and since** (2026-11-18 to 2026-11-20),
each in its NOTE:

- **D32-B to D37-A** at Block 8's gate:
  "Select server" alone in the Cube choice; no Recents picture of a page
  asked at each return; one key per server; the PIN and a new fingerprint;
  no key over plain HTTP outside the private ranges; the user manual's first
  chapter (`2026-11-18_13-47_NOTE_MobileApp_v1.1_block_8_decisions.md`).
- **D38-A, D39-A** at Block 12's gate: the toasts, discovery's name
  (`2026-11-18_16-03_NOTE_MobileApp_v1.1_how_block_12_went.md`).
- **D40 to D53** at the phone check of 2026-11-20, numbered D38 to D51 there
  before Block 12's took D38 and D39
  (`2026-11-20_20-05_NOTE_MobileApp_v1.1_phone_check_block_8.md`):
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
D58 to D73 in the check's NOTE, the first two at its signal;
D74, D82 and AP KeyLife's below taken by the owner afterwards,
on the points the check raised, and recorded here alone:

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

**Open at this revision's gate:**

- **D84, the password's eye**, the owner's question whether it fits:
  a button with a closed eye at the right end of a password field,
  which shows the password in clear text and hides it again.
  - **A (recommended).** In the shared field (`src/shared/password.tsx`),
    so that every password field has it:
    the sign-in, the first owner's two, Personal settings' change of password;
    a labelled toggle (`aria-pressed`), keyboard operable,
    the field hidden again at each opening of the card and at the submit;
    Android's WebView has no eye of its own,
    and a browser's own (Edge's `::-ms-reveal`) is hidden,
    so that a field has one eye; in Block 15.
  - **B.** On the sign-in card's password alone, in Block 15.

**AP KeyLife, a plan of its own to come,**
after Block 16, which edits the same row of My profile (`TokenRow`,
`internal/server/ui/src/shared/tokens.tsx`),
with the owner's decisions on My profile's keys (2026-11-21),
the second point raised at the check's end:

- **D75**, the expiry, in the owner's words
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
- **D76-A**, the re-key: "New secret" on a key's row;
  the id and the label kept, the new secret shown once,
  the key's sessions ended but the caller's own.
- **D77-A, D78-A**: the last use and the last address are on every row already;
  nothing to build.
- **D79-A**, the renewal: none, since use renews a key under the idle limit;
  to be weighed again in KeyLife, now that a key may have an end date.
- **D81-A**: a key is never shown again: the store keeps only its hash,
  and the re-key is the way back.
- Two gaps found on the way, for KeyLife to close first:
  a session made from a key checks only the key's revoke, not its end
  (`stillAdmitted` in `internal/server/auth.go`, `liveKey` in
  `internal/store/tokens.go`), to close before any key ends;
  and any key can make more keys (`POST /api/v1/users/{name}/tokens`),
  so revoking a leaked key leaves the keys it made.

**Names chosen here**, routine and not decisions of the owner's:
the directories `app/core`, `app/android`, `app/ios`;
the Makefile targets `app-core-android` and `app-core-ios`;
the route `POST /api/v1/login/key` and its audit kind `login.key`;
the column `tokens.parent_id` and its migration, 0025;
the bridge's calls `login.status`, `login.keep`, `login.fingerprint`,
`login.forget`, `connect`, `forget` and `forgetAll`,
and with D32-A `serverNames` and `switchTo`;
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

**Not in this plan:**

- pairing by QR code (a code shown by the web UI with the URL, the
  fingerprint and a one-time token): a later revision;
- an APNs relay for iOS push, and iOS push at all in the first iOS release:
  decided when Block 9 starts;
- the sign-in through OpenID Connect inside the app's WebView: the login by
  fingerprint follows whichever sign-in succeeded, but a provider's page in
  the WebView is untested;
- the life of a key, its idle limit, end date and re-key: AP KeyLife, above;
- saving a file the page offers (a backup's download): the app says it
  cannot yet (the NOTE of Block 3); a later call of the bridge;
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
| the login by fingerprint, the way back to the servers | `login.*`, `connect`, `forget`, `forgetAll`; with D32-A `serverNames`, `switchTo` | 8 |
| a tap's prompt, the server under the card's title, the app's settings from ☰ | `open`'s rule and `{signIn: true}`, `login.status`'s `server`, `openServerSettings`, `openAppSettings` | 15 |
| the phone's key in My profile, the sessions left | `login.status`'s `key_id`, `setToken`'s answer, a server's `login_key` and `unended` | 16 |
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
a Gradle project (Kotlin, `compileSdk` and `targetSdk` 36, `minSdk 26`),
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

**The owner's design** (`2026-11-17_14-17_NOTE_MobileApp_v1.0_phone_test.md`,
its last section), in short:
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
| `login.forget()` | forgets the credentials kept for this address |
| `connect()` | shows the connect page: "Select server" |
| `serverNames()`, `switchTo(id)` | with D32-A: the stored servers' names and ids, the one shown marked; and the adapter opening another of them |

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
A fingerprint enrolled later invalidates the key:
what was kept is dropped, and the form offers the password.
What is sealed is bound to the address and port, the user's name and the
server's pinned fingerprint, so that it opens for nothing else.
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
Not seen yet: a version tag's signing,
which runs at the owner's next `make release`.
Dependabot has no entry for Gradle (D29-A, Block 12).

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
The tracks, as Block 4's refit NOTE proposes for Block 6's gate:
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

From `2026-11-17_14-17_NOTE_MobileApp_v1.0_phone_test.md`,
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
enabling it with Try again, the login, the three choices with the cover
and the restart, the PIN, "Select server", a revoked key ending its session.
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

The owner's check of Block 13 on 2026-11-20 passed
(`2026-11-20_23-04_NOTE_MobileApp_v1.2_phone_check_block_13.md`).
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

The owner's check of Block 14 on 2026-11-21 passed
(`2026-11-21_21-24_NOTE_MobileApp_v1.3_phone_check_block_14.md`):
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
Traced in the code, likely but not proven:
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

What changes, by file, in the blocks still to run:

| File | Block | Change |
|---|---|---|
| `app/core/*.go`, `go.mod`, `Makefile` | 1 | the bound package, the `tool` directive, the targets |
| a merge request to `fdroiddata` | 5 | the recipe; here `app/README.md`'s section |
| the Play Console | 6 | the owner's; `app/README.md` records the steps |
| `app/android/**` | 7, 11, 15 to 18 | the service and its server kept open; push; the tap's rule, the server's line and the two settings; the sessions left, Remove, this device's key; the wrapping key; the emulator suite |
| `app/ios/**`, `.github/workflows/ci.yml` | 9 | the Xcode project, the macOS job |
| `internal/server/ui/connect.html`, `connect.js`, `src/signin/`, `src/chrome/Menu.tsx`, `src/shared/app.ts`, `login.js`, `i18n/*.json`, `tools/ui-test/` | 15 | the tap, the sign-in icon, the server under the card's title, ☰'s two settings, Show in My profile, Remove server |
| `internal/server/ui/app.js`, `login.js`, `connect.js`, `src/shared/tokens.tsx`, `src/shared/app.ts`, `src/profile/`, `i18n/*.json`, `tools/ui-test/` | 16 | the second card, a session replaced, this device's key and the ids, the mark after the sign-in's revoke, the sessions left, the time limit |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml`; `internal/server/login_api.go`, `server.go`, `audit.go`, `src/api.d.ts`, their tests | 16 | a key's own revoke |
| `internal/store/tokens.go`, `internal/server/auth.go`, their tests, `docs/api-v1.md` | 17 | an ended token not counted toward the lockout |
| `app/android/app/src/androidTest/`, `app/android/app/build.gradle.kts`, `app/android/gradle/libs.versions.toml`, `Makefile`, `tools/app-emulator/`, `.github/workflows/ci.yml`, `GUIDELINES/project/CI.md`, `app/README.md` | 18 | the emulator suite, its target and its job |
| `docs/app.md`, `docs/ui.md`, `docs/user-manual.md` | 11, 15 to 17 | push; the contract's version 5, the list, the card, My profile, one prompt |
| `app/README.md`, `README.md`, `docs/roadmap.md`, `GUIDELINES/wip/HANDOVER.md` | 10 | the documents; the roadmap and the handover by the coordinating session |
| `CHANGELOG.d/` | 1, 7, 9, 11, 15 to 18 | a fragment per block |

## Implementation Plan

Each block ends with what the contract asks of a change:
`make ui` before anything compiles Go; locally the tests of what the block
touches (`go test` of the touched packages with `MASSIMO_ENV_FILE=/dev/null`,
the touched files of the UI suite in its four cases,
`make ui-check` for a part,
the Gradle tasks `:app:assembleFossDebug`, `:app:testFossDebugUnitTest`,
`:app:lintFossDebug` and `:app:lintNightlyDebug` in `app/android`, with
`JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64 ANDROID_HOME=$HOME/Android/Sdk`,
`make actionlint` for a workflow),
`gofmt -l .` empty, `go vet` for linux, windows and darwin,
and `GUIDELINES/shared/tools/check` before every push;
the whole suites are CI's, and so are the race run and the cross builds
but where a block's Verification names them.
Each new test is seen failing with its fix undone.
Nothing reaches a real Cube or a box: the fake Cube answers;
a phone is the owner's, used on the owner's word.
Every commit cites `AP MobileApp v1.4, Block <n>`;
a block's bullet is a fragment, `CHANGELOG.d/<Heading>/<branch>.md`;
a pull request per block.

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
`test_connect.py`.
Its NOTE is `2026-11-09_04-30_NOTE_MobileApp_v1.0_block_2_connect.md`.

### Block 3: Shell

Done on 2026-11-16, pull request 341, and checked on the owner's phone on
2026-11-17 with one fix, pull request 344.
Its NOTEs are `2026-11-09_04-50_NOTE_MobileApp_v1.0_block_3_shell.md`,
`2026-11-16_17-38_NOTE_MobileApp_v1.0_block_3_on_main.md` and
`2026-11-17_14-17_NOTE_MobileApp_v1.0_phone_test.md`.

### Block 4: Release

Done on 2026-11-17, pull requests 349 and 352,
under a gate of its own (D12 to D21):
the job "Android app" in `ci.yml` and in `build.yml`,
the three jobs of `release.yml`, the nightly app, `app/version.json`,
`packaging/app-version.sh` and `packaging/app-sign.sh`.
The owner's three keys are in `signing` and `signing-rolling`,
their certificates under `app/android/signing/`.
Its NOTEs are `2026-11-17_18-27_NOTE_MobileApp_v1.0_block_4_refit.md`,
`2026-11-17_18-53_NOTE_MobileApp_v1.0_how_block_4_went.md` and
`2026-11-18_12-43_NOTE_MobileApp_v1.0_first_signed_nightly.md`:
the first nightly APK's signature and sums were checked by hand on 2026-11-18.
Still to see: a version tag's signing of the APK and the AAB,
at the owner's next `make release`.

### Block 5: F-Droid

1. The recipe written and tried with `fdroid build` in the F-Droid buildserver
   image where possible, else reviewed against the recipes of the analysis;
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
   from the proposal in Block 4's refit NOTE;
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
and checked on the owner's phone on 2026-11-20.
Its NOTEs are `2026-11-18_13-47_NOTE_MobileApp_v1.1_block_8_decisions.md`,
`2026-11-18_16-38_NOTE_MobileApp_v1.1_how_block_8_went.md` and
`2026-11-20_20-05_NOTE_MobileApp_v1.1_phone_check_block_8.md`;
the check's findings are Block 13.

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

Done on 2026-11-18, pull request 366, under a gate of its own (D38, D39).
Its NOTE is `2026-11-18_16-03_NOTE_MobileApp_v1.1_how_block_12_went.md`.

### Block 13: Check

Done on 2026-11-20, pull requests 384 and 386, under v1.2's gate (D40 to D53).
Its NOTEs are `2026-11-20_22-18_NOTE_MobileApp_v1.2_how_block_13_went.md`
and, for its check on the owner's phone,
`2026-11-20_23-04_NOTE_MobileApp_v1.2_phone_check_block_13.md`.

### Block 14: Second check

Done on 2026-11-21, metaworx/massimo#390, under v1.3's gate (D54 to D57),
and checked on the owner's phone the same day, with D58-A and D59-A.
Its NOTEs are `2026-11-21_03-25_NOTE_MobileApp_v1.3_how_block_14_went.md`
and, for its check on the owner's phone,
`2026-11-21_21-24_NOTE_MobileApp_v1.3_phone_check_block_14.md`;
the check's findings are Blocks 15 to 18.

### Block 15: List and card

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

### Block 16: Sessions and keys

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

### Block 17: One prompt and the lockout

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

### Block 18: Emulator

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

## Proposed commit message

The revision's registration, a commit of its own on `main`:

```
chore(aps): +MobileApp v1.4

The owner's check of Block 14 on the phone found twelve points, F29 to F40; with the decisions taken on them, D58 to D83, D84 open at the gate, they become Blocks 15 to 18, Block 14 is marked done, Block 7 takes AP FirstOwner's step for the phone's own server, and My profile's keys are named as AP KeyLife, a plan to come.

Key changes:
- GUIDELINES/wip/2026-11-09_02-30_AP_MobileApp_android_ios_apps.md: v1.4: the third check's findings in the Analysis, with a tap's rule, one wrapping key for Forget all weighed and the lockout of an ended token; D58 to D83 recorded, D84 open; the password's eye in Block 15; Blocks 15 to 18; Block 7's step for the server kept open; Block 14 done; the table of files pruned of the blocks done; a stale line of "Not in this plan" dropped.

File-revision-v1: GUIDELINES/wip/2026-11-09_02-30_AP_MobileApp_android_ios_apps.md::v1.4::2026-11-23::TL;DR, Discussion, Analysis, Implementation Plan, Proposed commit message, Change History::Blocks 15 to 18, the findings of Block 14's phone check
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits still to come:

1. `feat(app): The server as app/core, bound by gomobile`
7. `feat(app): The Android app runs the server itself`
9. `feat(app): The iOS app`
10. `docs(app): The apps' documents`
11. `feat(app): Push through UnifiedPush`
15. `fix(app): A tap asks the fingerprint, and the sign-in card names its server, as the third check found`
16. `fix(ui): A late 401 opens no second card, and My profile tags this device's key`,
    and `fix(app): A session the app forgot is signed out at the next contact`;
    and `feat(api): A token revokes itself with DELETE /api/v1/me/token`
17. `fix(server): An ended token does not count toward the address lockout`,
    and `fix(app): Forget all asks the fingerprint once`
18. `test(app): The app's flows on an emulator, locally and in CI`

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
