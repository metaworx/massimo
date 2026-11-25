# AP AccessEnds v1.1: Follow-ups of two gates

## TL;DR

The decisions the completion gate of AP LooseEnds and AP PasswordRules listed,
each as its option A, D47-A with a log entry,
and the credits' versions (D40): eight blocks, a pull request and a NOTE each.

- late sign-in answer → does nothing, its session ended (LooseEnds.D44)
- integration's user's roles → within its maximum, `integration` again (D45)
- open server's Users → "No owner yet", its integration's users listed (D46)
- first owner → told what the open server left: page, API, CLI, start;
  a log alert (D47)
- tall owner's card → right at its top (D57);
  the suggestion whole under the field (D58); the dice's move measured,
  D59 again (D23)
- common passwords → Openwall's list, if D29 lets it (D55)
- OPAQUE's crates → their licence texts in the credits (D56)
- test files → not shipped, no bullet owed (D48)
- Go modules' versions → read from the binary, not the list:
  a bump fails no test (D40)

### Blocks

1. Late: every way of the sign-in card checks its opening after its answer
2. Users: the roles an integration's user may get; the open server's Users
3. OpenSetup: what was set up while the server was open, said once and logged
4. Tall: the card's measure for a card taller than the window;
   the suggestion's line
5. Common: Openwall's list of common passwords, if D29 lets it
6. Crates: the licence texts of the OPAQUE client's Rust crates
7. Changelog: the shipped paths leave out the test files
8. Versions: the credits take each Go module's version from the binary

## Discussion

### What is decided

The owner, 2026-11-25,
at the completion gate of AP LooseEnds and AP PasswordRules,
in the owner's words: "D47A: with a log entry",
"send the ten unsteady tests to "Fix flaky tests"" and "PLAN++ as recommended".
So each decision the gate listed is taken as its option A,
and this plan is built under PLAN++, as both were.

- **LooseEnds.D44-A.
  The password's way's late answers are closed** as LooseEnds' Block 6 closed
  the key's, with its tests.
  The fingerprint's way and a provider's button have the same window (Analysis);
  "as Block 6 closed the key's" takes them too.
  It extends LooseEnds.D43-A's check to every way of the card.
- **LooseEnds.D45-A.
  An integration's user's Role list offers only the roles within its
  integration's maximum.**
  Whatever is chosen, the user is held to that maximum, as today.
- **LooseEnds.D46-A.
  Users on an open server says "No owner yet",
  with the integration's users listed, read-only.**
  It reverses LooseEnds.D42-A in part:
  D42-A kept "No users yet" and said that Users lists none of an integration's
  users while the server is open; now Users lists them.
  The first owner's card keeps its title as D42-A left it (D9).
- **LooseEnds.D47-A, with a log entry.
  The first owner's creation says, once,
  what was set up while the server was open.**
  "With a log entry" is the owner's own addition:
  the log keeps it beside what the page says.
- **LooseEnds.D48-A.
  The project's shipped paths leave out the test files,**
  in D48-A's words `**/*_test.go` and the test scripts under `packaging/`;
  D35 takes the other test files with them.
  It ends the practice LooseEnds' Analysis "For Block 5" describes,
  a test-only commit leaning on COMMIT.md §5.2's one reason.
- **PasswordRules.D55-A.
  Openwall's `password.lst`, John the Ripper's list,**
  whose header says "It is assumed to be in the public domain."
  It gives PasswordRules.D10-A's aim another source than SecLists',
  which did not hold
  (`wip/2026-11-25_03-26_NOTE_PasswordRules_v1.0_no_list_of_common_passwords.md`).
  Chosen on the header's sentence; D29 puts to the owner what the findings add.
- **PasswordRules.D56-A.
  The crates' licence texts,
  collected "consistent with the artifact",** in a file beside the vendored
  OPAQUE client, which the credits take up by its name.
  It replaces PasswordRules' Block 5 sentence that their texts are
  not reproduced.
  What "consistent with the artifact" can honestly mean is in the Analysis,
  "For Block 6", and the file says it in its head.
- **PasswordRules.D57-A.
  The card's measure tests take a card taller than the window as right,
  where it starts at its top and the page scrolls.**
  The stylesheet does so already (AP MobileApp, Block 14: F27).
- **PasswordRules.D58-A.
  A line under the field shows the whole suggestion while it is shown.**
  It adds to PasswordRules.D19-A, which showed it in the field alone.
  With it the app's card at 390 by 900 px is taller than the window in five
  languages, measured, not three as the gate said: it goes with D57-A.
- **PasswordRules.D59-A.
  The dice's move at the first press is left.**
  Its reason, that a second tap at the dice's centre still lands on the dice,
  held for 14 px; with D58-A the move is about 40 px where the card fits,
  more than the dice's 36 px: D23.

### Taken with PLAN++

The choices the findings raised, numbered in block order,
each taken as its option A;
each the owner's to change, and the completion gate lists them.

For Block 1, Late:

- **D1-A.
  The first owner's form.**
  Its window is closed already:
  `card.working` turns off every way out of the card while it creates,
  and `card.creating` keeps another request's 401 from turning the card;
  only a script's close of the dialog reaches its late `setToken`.
  - **A (taken).**
    Left as it is, `docs/ui.md` saying why.
  - **B.**
    The opening's check there too: uniform, but a late owner's session dropped
    leaves the new owner signed out, for a window no user reaches.
- **D2-A.
  A late answer that made a session.**
  - **A (taken).**
    Ended: the password's by `POST /api/v1/logout` with it;
    the fingerprint's by handing the app's bridge the page's own token back,
    which ends the session it held.
    Nothing live is left that nobody holds.
  - **B.**
    Dropped and left to expire: a live session per late answer, until its end.

For Block 2, Users:

- **D3-A.
  Who computes the roles a user may get.**
  - **A (taken).**
    The server, as `may.roles` of each user:
    it alone reads the roles' sets and the maximum for every caller who may
    manage users,
    and the rank rule and the refusal of `integration` come with it;
    one rule for the page and the API.
  - **B.**
    The maximum in the user's integration mark, the page intersecting it with
    the roles it reads: blind where the caller may not read the roles.
  - **C.**
    The page reads the Integrations region's list: crosses regions,
    needs `integrations`.
- **D4-A.
  A user stored with a role above its maximum** (a Save before this block,
  a maximum lowered, a role widened, an older store).
  - **A (taken).**
    Its role stays in the list, selected, with a line under it:
    it acts within its integration's maximum all the same;
    another role chosen is sent, and the old one is not offered again.
  - **B.**
    A disabled option with the line: a disabled selected option reads oddly to
    a screen reader.
- **D5-A.
  The role `integration` for an integration's user.**
  - **A (taken).**
    `mayAssign` gives it back to an integration's own user, and to no other;
    offered in its list.
    Today one Save away from it is for good (403).
  - **B.**
    The server as it is; `integration` offered only as the current role.
- **D6-A.
  A role beyond the maximum sent to the API.**
  - **A (taken).**
    Taken, as today: the calls are capped whatever the role,
    a stored role drifts above by later changes in any case,
    and `TestIntegrationUserHasNoWayIn` gives such a role on purpose;
    `may.roles` is what the page offers.
  - **B.**
    400 in `PUT /api/v1/users/{name}`: the API agrees with the page;
    that test's set-up moves to the store,
    and `docs/api-v1.md`'s "whatever its role" changes.
  - **C.**
    The role fixed at `integration`, every change refused,
    as one investigation proposed: it undoes D45-A's offer.
- **D7-A.
  A row of the open server's list.**
  - **A (taken).**
    A plain row, the user's name and badges, nothing to open or press:
    nothing the server would take from anyone on the local network.
  - **B.**
    The usual panel with a read-only flag: every control to guard,
    and the server would still take each change.
- **D8-A.
  The head's badge on an open server.**
  - **A (taken).**
    "No owner yet", warn, whatever the users.
  - **B.**
    "No owner yet" and a second badge counting an integration's users.
- **D9-A.
  The line under the head.**
  - **A (taken).**
    With an integration's users listed, a line of its own:
    "No owner yet: the server is open to everyone on the local network.
    An integration's users act through it alone and sign in nowhere."; without,
    `users.noOwner` as now, and the first owner's card keeps its title,
    as D42-A left both.
  - **B.**
    `users.noOwner` always: "No users yet" above a list of users reads false.
- **D10-A.
  The Users list after an integration's change.**
  - **A (taken).**
    Read again after an approval, a refusal and a removal in Integrations,
    with an owner as on an open server, where the caller may manage users.
  - **B.**
    As it is: right once System settings is shown again.

For Block 3, OpenSetup:

- **D11-A.
  Where it is said once.**
  - **A (taken).**
    The toast after the creation, and a stored line of the log at warn:
    a log alert until a person marks it seen.
    The toast for this page, the alert for every browser and a reload;
    nothing new to keep.
  - **B.**
    A step in the card before it closes: surer to be read,
    but a step more, its height (D57) and its focus, and Create's turn longer.
  - **C.**
    The log alone.
- **D12-A.
  The command line's trigger.**
  - **A (taken).**
    The first regular user `massimo user add` makes, whatever its role:
    then the open server closes; the line says the role.
  - **B.**
    `-role owner` alone.
- **D13-A.
  The toast's time.**
  - **A (taken).**
    30 s, with its close button.
  - **B.**
    The default 10 s.
- **D14-A.
  The owner made from `MASSIMO_API_TOKEN`.**
  - **A (taken).**
    The same warning and log line where `Bootstrap` makes it:
    at the start and at a reload of the settings file.
  - **B.**
    Left out: the token's owner is the box's, set up on the box.
- **D15-A.
  Integration requests still waiting.**
  - **A (taken).**
    Not listed: they give nothing until approved,
    which the Integrations card asks for.
  - **B.**
    Listed as waiting.

For Block 4, Tall:

- **D16-A.
  Where the tall rule applies.**
  - **A (taken).**
    A helper of its own, taken by the two measure tests and the short screen's;
    the sign-in card's tests keep the strict one,
    so that a sign-in card that grows by accident still fails.
  - **B.**
    `assert_login_card_centred` takes both ways, for every caller.
- **D17-A.
  "Reached" for a tall card.**
  - **A (taken).**
    Its top at the body's padding at scroll 0,
    its bottom inside the window at the page's end:
    one check that covers every control.
  - **B.**
    Each control scrolled into view in turn: slower, the same result.
- **D18-A.
  The strict helper's "fits".**
  - **A (taken).**
    The room, the window less the body's padding (868 px),
    so that its message says "does not fit" between 868 and 900 px.
  - **B.**
    The window's height, as now.
- **D19-A.
  The line's place after an edit.**
  - **A (taken).**
    From the first press until the row goes, a place two lines tall,
    empty after a key: nothing moves between presses or while one types;
    52.4 px always.
  - **B.**
    As tall as the last suggestion, kept after a key:
    29.2 px in most draws, but a second press can move things by a line.
  - **C.**
    It comes and goes with the suggestion: the field moves under the caret,
    the fault PasswordRules' Block 4 removed.
- **D20-A.
  Where under the field.**
  - **A (taken).**
    Right under it, before the rule's line:
    the password beside its field, then what the rule says of it.
  - **B.**
    Between the rule's line and the row.
  - **C.**
    Inside the row.
- **D21-A.
  The line's font.**
  - **A (taken).**
    The body's, 16 px: readable to copy by hand,
    twenty characters always on one line; the alphabet has no look-alikes.
  - **B.**
    Monospace: five words almost always two lines.
  - **C.**
    The small 13.6 px: harder to read.
- **D22-A.
  What a screen reader hears of it.**
  - **A (taken).**
    Nothing: `aria-hidden="true"`,
    and `translate="no"` against a page's translation; the field,
    text while it holds the suggestion, gives the value,
    and the rule's line says "Suggested".
    The suggestion tests' `SPOKEN` leaves out text under `aria-hidden`,
    and a test holds that the line is so.
  - **B.**
    Readable, outside any live region:
    the live regions of My profile and Users moved.
- **D23-A.
  The dice's move, PasswordRules.D59.**
  - **A (taken).**
    D58-A built as above, the move measured in the block in each case
    and language,
    and D59 put to the owner again at the completion gate with the numbers,
    its option B there being to keep the card's top where it is while a
    suggestion's row is shown.
  - **B.**
    D59-B taken into this block now.

For Block 5, Common, each going with the block under D29-A (D29 itself is open,
below):

- **D24-A.
  The entry's licence in the credits.**
  - **A (taken).**
    The Licence cell of `MASSIMO.md` empty:
    System info reads "licence in its text" and shows the header,
    nothing the source does not say.
  - **B.**
    A made-up `LicenseRef-…` identifier, which looks like a licence.
- **D25-A.
  How the file is made again.**
  - **A (taken).**
    The one-line pipeline in `MASSIMO.md` and a test that holds the file to its
    SHA-256, as EFF's list is held; no tool: the 15 MB source is not vendored,
    so no check could run offline.
  - **B.**
    A tool, `go run ./tools/commonlist <password.lst>`:
    more code for the same bytes.
- **D26-A.
  The order of the answers.**
  - **A (taken).**
    Length, then common, then the names:
    whether a password is common does not depend on the names.
  - **B.**
    The names before common.
- **D27-A.
  Where the page gets the list.**
  - **A (taken).**
    Fetched at the first key in a new password's field;
    a submit waits for it 5 s at most, then checks the rest and says so in the
    console (PasswordRules' "A list not loaded").
  - **B.**
    In the bundle: 34 KB more at every load, the bytes in two places.
- **D28-A.
  The wordlists collection's terms.**
  - **A (taken).**
    Quoted in `NOTICE` beside the header,
    so that nobody reads the header as the whole story.
  - **B.**
    The header alone.

For Block 6, Crates:

- **D30-A.
  What the tool repeats.**
  - **A (taken).**
    The resolved list is committed, each crate's name, version and checksum;
    a Go tool only fetches those archives, checks each checksum,
    takes the texts and writes the file.
    The resolution is described, not run again,
    so the list cannot change unseen.
  - **B.**
    The whole resolution in Go, run again at each update:
    more code, which gains only at the next update of `opaque.js`.
- **D31-A.
  Rust's own parts.**
  - **A (taken).**
    `dlmalloc` 0.2.11 and `hashbrown` 0.16.1 added from crates.io; `core`,
    `alloc` and `std` named by the compiler's commit, their texts left out:
    they are in Rust's repository,
    and their own dependencies at that commit are unknown.
  - **B.**
    The 51 crates alone.
- **D32-A.
  `gloo-utils` 0.2.0, without a licence file in its archive.**
  - **A (taken).**
    Said in the file, with its declared licence.
  - **B.**
    MIT and Apache-2.0 taken from its repository at the tag:
    a source not read yet.
- **D33-A.
  The file served at `/ui/vendor/`.**
  - **A (taken).**
    No: the credits carry it, and `go:embed` would hold its 178 KB twice.
  - **B.**
    Yes, as `opaque.LICENSE` is.
- **D34-A.
  Folding.**
  - **A (taken).**
    Byte-identical texts alone; every other text as it came.
  - **B.**
    Texts equal once whitespace is collapsed folded too:
    46 texts, a little less, but no longer as they came.

For Block 7, Changelog:

- **D35-A.
  The test files D48-A did not name.**
  - **A (taken).**
    They leave too: `testdata/` at any depth (`:!**/testdata/**`)
    and the app's unit and instrumented tests;
    none of them reaches a user.
  - **B.**
    `:!internal/**/testdata/**`: the same today.
  - **C.**
    D48-A's two groups alone.
- **D36-A.
  The test kit, `internal/testkit/` and `internal/push/pushtest/`.**
  - **A (taken).**
    Stays shipped: it is no `_test.go`,
    nothing guards against a non-test import of it,
    and a change to it can change what a contributor sees (`make race`'s scale).
  - **B.**
    Left out too, with a Go test that the binary's dependencies hold neither.
- **D37-A.
  Words beside the list.**
  - **A (taken).**
    The config's comment alone:
    the contract's generated block shows the `:!` lines,
    and its sentence before them stays true.
  - **B.**
    A sentence in the contract's "Tests" bullet too, which repeats the list.
- **D38-A.
  The "No CHANGELOG bullet" line.**
  - **A (taken).**
    It stays: a test-only commit says it with its reason, only tests change,
    which are not shipped (an exemption is said out loud, COMMIT.md §5.1);
    a commit under a shipped path that writes no bullet for another reason says
    that reason, as now.
  - **B.**
    Dropped for a test-only commit, against §5.1.
- **D39-A.
  CI's tools under `packaging/`,
  `go-test-report.sh` and `tested-record.sh`.**
  - **A (taken).**
    They stay shipped, as `.github/` is;
    their tests leave with `:!*_test.go`.
  - **B.**
    Left out too.

### Decided after the first blocks

The owner, 2026-11-25, while Blocks 1, 2, 4 and 6 were built:

- **D40-A.
  The credits take each Go module's version from the binary.**
  `internal/credits/credits.json` records every Go module's version,
  and `tools/credits`' `TestGeneratedFileCurrent` compares the file byte for
  byte with a fresh run,
  so every bump in `go.mod` fails CI until `make credits` runs:
  seen on a Dependabot pull request (`golang.org/x/net` 0.62.0 to 0.63.0),
  in the jobs Vet and test and Test (armv6, QEMU).
  The fault is PasswordRules' Block 5's.
  Reported by the session "Fix flaky tests" on the owner's word there.
  It is Block 8, built on Block 6, both in `tools/credits`.
  Not chosen: CI making the list again on Dependabot's pull requests
  (the workflow scope and a second commit on each);
  `make credits` by hand at each bump.

### Open: the owner's

- **D29, on PasswordRules.D55: the wordlists collection's licence.**
  The owner chose Openwall's list on its header's sentence alone.
  The findings add: Openwall's wordlists collection publishes the first 3,546
  entries of `password.lst`, byte for byte,
  under terms that forbid charging for a copy and bundling the collection with a
  product that is charged for; 613 of the cut's 3,510 entries, the commonest,
  come from that part.
  The header's sentence was added to John the Ripper's copy alone, in 2013;
  it is the compiler's assumption, not a licence,
  and it says nothing of the sources of the entries added in 2022.
  Massimo is distributed under AGPL-3.0-or-later, which lets anyone sell a copy;
  terms that forbid charging cannot be passed on with it,
  so the list would ship on John's copy's sentence alone, an assumption.
  - **A (recommended).**
    No list for now (PasswordRules.D55-C): Block 5 dropped,
    the completion gate saying why.
  - **B.**
    Built as chosen: the header quoted as the author's assumption,
    the collection's terms beside it (Block 5 as written).
  - **C.**
    The owner asks Openwall first; Block 5 waits for the answer.

- **D41, who manages an integration's user stored as guest.**
  Today any `guests` holder may disable, rename and remove such a user:
  a role of the owner's such as the seeded manager,
  and another integration's call, whose ceiling holds `guests` (`auth.go`,
  `integrationCeiling`); run on main and on Block 2's tree,
  200 each (`users_api.go`, `mayManage`).
  Disabling it stops its calls even where it is linked to a Massimo user:
  the chain checks the integration's user before the linked one (`onbehalf.go`).
  Block 2's Role list steers an owner who narrows such a user to guest.
  Older than this plan.
  - **With a link,** the owner's question: there are two users.
    The linked Massimo user is a regular user and managed as any,
    under every option below (a `guests` holder,
    an integration's call with `guests` among them,
    manages it where it is a guest).
    The integration's user behind the link is the one this decision is about.
    A call through an integration acting for a linked user is capped by its
    chain (the key, the ceiling, the integration's user's role,
    the linked user's role, the maximum);
    under A it manages its own integration's users and regular guests within
    that, never another integration's users.
  - **A (recommended).**
    An integration's user is managed by a holder of `users`, by rank,
    and by its own integration's calls;
    a `guests` holder of no integration,
    and another integration's call, no longer manage it.
    Regular guests stay as they are.
  - **A2.**
    As A, and a call through an integration manages no regular user either;
    the ceiling's `guests` then serves its own users alone.
  - **B.**
    As it is, said in the documents and beside the Role list.

### What stays as it is

- The open mode: every request from the local network may do everything,
  writes to roles and to an integration's users included;
  both investigations saw it (a role's set widened, an integration's user given
  the role owner and removed, each without a credential).
  A decision of its own for the owner, which the completion gate names.
- AP SecondFactor, at its plan gate: not this plan's.
- Any retirement, of this plan, AP LooseEnds or AP PasswordRules:
  the owner's word.
- A release.
- The ten unsteady tests: another session's, "Fix flaky tests".

## Analysis

Read on main at `chore(aps): +NOTE StartConfiguration v1.1, how the run went`,
2026-11-25, by five investigations: each fact below was read in the code,
and where it says "run", seen on a server of its own against the fake Cube with
a fresh store, or measured in an exported copy with the suite's Chromium.

### For Block 1: the late answers of the sign-in card

- **The key's way** (`submitKey`,
  `src/signin/Card.tsx`) takes the session's number (`session()`,
  `src/shared/session.ts`) and the card's opening (`ask()`,
  `src/signin/store.ts`) at the submit, and after its read checks `still()`,
  the same session, the same opening still the card's, and open,
  on both branches, before it writes a line, sets a token or asks for the focus.
- **Every other way awaits without it:**
  - the password's (`submitPassword`):
    a refusal writes the card's line and takes the focus into Password,
    in whatever opening the card has by then;
    a session becomes the page's token (`setToken`,
    `login.js`) over the one another way set meanwhile,
    in a browser into storage, in the app into the bridge,
    while `state.me` stays the other way's:
    every request then goes as the password's user under what the page shows of
    the key's.
    The worst of them, reached by a password signed in and a key sent while
    OPAQUE's argon2id turns, a second or more on a slow phone;
  - the fingerprint's (`fingerprint`):
    a refusal writes its line likewise; a token replaces the page's,
    and the bridge holds the fingerprint's session already;
  - a provider's button (`startProvider`,
    `login.js`) awaits `POST /api/v1/login/oidc/{key}`,
    which fetches the provider's discovery first,
    then leaves the page for the provider,
    even where another way has signed in meanwhile;
    a late refusal writes into a later opening.
- **Closed already:**
  the key's way (LooseEnds' Block 6); `finishSignIn`,
  which checks `ask.closed` after each await; the provider's return,
  read in `init` before any card; keep open; and the first owner's form,
  by `card.working` and `card.creating` (D1).
- **How an opening ends early:**
  the card the server asks for cannot be dismissed, so by another way finishing,
  or on a running page by a sign-out and a new card;
  no way's button is off while another way is busy.
- **A session a late answer made and the page drops stays live:**
  `setToken` ends a replaced session in a browser only where who is in was read
  with it and it is a session (D2).
- **The key's two tests** (`test_login.py`,
  `test_a_late_key_answer_makes_no_token`,
  `test_a_late_key_refusal_writes_nothing`) are the model;
  LooseEnds' Block 6 NOTE left two things of them to the file's next change:
  the first's docstring names a reason it fails for on Block 6's first tree
  alone, and the second's focus assertion cannot fail,
  the key's field being hidden on the Password way
  (`wip/2026-11-25_11-15_NOTE_LooseEnds_v1.2_how_block_6_went.md`,
  "Seen and left").

### For Block 2: the Role list and the open server's Users

- **An integration's maximum is a set of capabilities** (`max_caps`),
  not a role.
  Every call of its users is capped to the role's set ∩ the role
  `integration`'s ceiling (`integrationCeiling`, `auth.go`) ∩ `max_caps`,
  whatever the user's role (`principalFor`).
  So a role is within the maximum where its set lies within `max_caps` and the
  ceiling; the role owner has no set (`Capabilities` empty,
  every capability by code), so a subset test would take it:
  it is left out by its key.
- **The seeded roles:**
  guest holds read, setpoint and configure; operator,
  guest and viewer lie within the ceiling;
  manager holds `integrations` and `passwords`, outside it;
  `integration` is the ceiling itself (run).
- **`roleOptions`** (`src/admin/sendUsers.ts`) offers every role read,
  or `DEFAULT_ROLES` (`login.js`), owner only to an owner,
  the current one kept; nothing looks at `u.integration`,
  so an owner is offered every role for an integration's user.
  It offers `integration` to every user and in Add a user,
  which `mayAssign` (`internal/server/users_api.go`) refuses to anyone;
  an integration's user moved off it by one Save cannot get it back (run: 403,
  for its own user too).
  `DEFAULT_ROLES` has no other reader: Providers' choice is `autoRoleOptions`.
- **The page cannot compute "within" for every caller:**
  the Users region reads no integration;
  the maximum is in `GET /api/v1/integrations` (capability `integrations`),
  the roles' sets in `GET /api/v1/roles` (capability `roles`).
  `listUsers` reads the integrations already (`integrationMarks`).
- **The server takes a role above the maximum:**
  `putUser` asks `mayManage` and `mayAssign` alone (run:
  owner given to `<uuid>:system` on an open server, 200);
  `TestIntegrationUserHasNoWayIn` does it on purpose,
  and `docs/api-v1.md` says "whatever its role".
  A stored role is also above it after a lowered maximum,
  a widened role or an older store (D4).
- **On an open server** `GET /api/v1/users` answers every user,
  `may` all true, the open principal holding `users`;
  after an approval, `<uuid>:` and `<uuid>:system` (run).
  `UsersCard` counts `users.list` in its head but draws `OpenServer` alone:
  "0 users" right after an approval in the page,
  "2 users" beside "No users yet" once System settings is shown again (run).
  An approval reloads the integrations alone (the Integrations runner's
  `reload`), so the count is stale with an owner too (read).
- **What a panel's controls would do there** (run, no credential): display name,
  role, disabled, end date and Remove are taken; a password, a key,
  an identity or a link refused for an integration's user (`noWayIn`).
  So read-only means nothing to press (D7).
- **Texts and ids:**
  no "No owner yet" exists in any language;
  `askOwner` (`src/shared/owner.ts`) opens the owner's form,
  which `OpenServer` offers already.
  `test_ways.py`'s `test_create_from_users_shows_the_owner` asserts no
  `#users-list`, `#password-login` or `#user-add` on an open server,
  and many tests count `.user-panel`: the new list takes neither.
- **Where the findings differ:**
  the investigation of D47 proposed fixing an integration's user's role at
  `integration` and refusing every change, having not followed the cap;
  the cap holds (`principalFor`), and D45-A offers roles within the maximum,
  so that proposal is D6's option C.

### For Block 3: what an open server leaves to its owner

- **What a request without a credential stores on an open server** that still
  gives access or sends data once an owner exists:
  - an integration approved:
    its key works in users mode as the integration's user,
    within the ceiling and its maximum (run); its maximum, state and links;
  - a sign-in provider: it signs users in once the server has users,
    with `auto_create` making users of `auto_role`,
    never owner nor a role with an admin capability (read:
    it needs a reachable issuer);
  - roles changed or added and capabilities turned off:
    `PUT /api/v1/roles/viewer` with `users` added answered 200,
    so every viewer made later may manage users (run);
  - an integration's user's role (Block 2).
  - Not: the settings (none gives access;
    `auth.keep_open` goes with the first owner),
    Cubes (the Cube protocol alone), a user's keys, identities,
    pushes or preferences (refused: no regular user).
  - Each such write is already a `change` line of the log, with no "by" (run).
- **Where the first owner is made, and what is said:**
  - `postUser` (`users_api.go`), open mode, answers 201 with the user's view and
    logs `first owner created` at Info, which the log card does not show:
    it shows stored events alone (run);
  - the page (`src/signin/Owner.tsx`) drops the POST's answer and shows the
    toast `setup.done` once `finishSignIn` has ended the ask;
  - `massimo user add` (`cmd/massimo/user.go`) opens the store and adds a user
    of any role, so on an open store its first regular user closes the server;
    it writes the audit row and a `change` event (`logChange`) and prints one
    line;
  - `MASSIMO_API_TOKEN` makes the owner where no regular user exists
    (`Bootstrap`, `internal/store/bootstrap.go`):
    at the start (`internal/server/server.go`,
    which drops `Bootstrap`'s result) and at a reload of the settings file
    (`SetToken`, `auth.go`, which logs `owner_created`).
    The investigation named the reload's place alone; D14 takes both.
- **The log:**
  a server's own line of the log card is a system event
  (`logServer(systemEvent(kind, userID, summary))`, `server.go`,
  `systemlog.go`), its kind in `eventKinds` (`log.go`),
  `LOG_KINDS` (`src/log/kinds.ts`),
  a case of `src/log/text.ts` and `log.kind.<kind>` in six languages,
  which `TestLogKinds` holds together;
  an event with no level gets info (`settle`), so the new one says warn.
  A stored line at warn or above is a log alert (`logalerts.go`),
  marked in the header's log icon until a person marks it seen,
  kept per user across a reload and a second browser;
  a kind without a settle rule is settled by a mark alone.
  The server's form for an open server's state is a Warn line in lower case,
  a colon and an imperative (`warnIntegrationUsersOnly`).
- **The toast** (`showToast`, `components.js`) takes a tone,
  a time and an action; 10 s by default.

### For Block 4: the card's height and the whole suggestion

- **The stylesheet:**
  a signed-out page with the card open is a centred flex column,
  `min-height` 100vh and 100dvh,
  with 16 px padding above and below (`style.css`),
  whose comment says a taller card starts at its top and the page scrolls (AP
  MobileApp, Block 14: F27).
  In the tests' 900 px the room is 868 px.
- **`assert_login_card_centred`** (`tools/ui-test/helpers.py`) asserts the card
  shorter than the window, then as much room above as below,
  left as right and the title centred, each within 2 px:
  between 868 and 900 px it says "fits" and fails on the centring.
  Its callers: `test_the_owner_card_measures` and
  `test_the_card_asks_at_the_start` (`test_first_owner.py`),
  `test_keep_open_measures` (`test_keep_open.py`),
  `test_sign_in_and_out` (`test_login.py`) and two in `test_connect.py`.
  `test_the_owner_card_on_a_short_screen` holds the tall rule in three asserts
  of its own.
  Nothing measures the card after a press:
  in French in the app at 390 px it is 890 px today.
- **Heights after a press,** the owner's card in the app at 390 by 900 px,
  measured with the test machine's fonts (today / with the line in a place of
  two lines): en 799 / 851, de 867 / 919, fr 890 / 943, it 822 / 874,
  nl 845 / 897, pl 822 / 874 px.
  In the browser at 390 px French with two lines is 876 px; the rest fit.
  The confirmation to keep the server open has the form's height in every case.
- **The suggestion:**
  five words are wider than the field at every width,
  about three and a half show at 390 px.
  In the body's font at 16 px one line costs 29.2 px and two 52.4 px;
  five words fit one line in 62 % of draws at the owner's card's 317 px,
  never need three (the widest, 495 px, is under twice a panel's 283 px);
  twenty characters always fit one line.
- **The live regions:**
  My profile's form and Users' body are polite live regions (`Profile.tsx`,
  `partials/users.html`):
  a readable line there would speak the password as it appears.
  The suggestion tests' `SPOKEN` (`test_password_suggest.py`) takes the text of
  every live region, so a prototype failed 16 checks there,
  and 4 of `test_the_row_stays_while_one_types`, which holds that nothing under
  the field moves at a key (run).
- **The dice's move (D23):**
  a card that fits centres again by half of what a press adds: 14 px today,
  40 px with the two-line place (app 390 en: its top from 64.5 to 24.5 px),
  more than the dice's 36 px;
  a card that starts at its top moves by its top before less 16 px (app 390:
  de 14, nl 25, it and pl 37 px).

### For Block 5: Openwall's list

- **Today:**
  no list (`internal/password`'s package comment); `Check` takes the length,
  then the context words; the page's `passwordProblem` answers `too_short`,
  `too_long` or `context`; both are held to `testdata/cases.json`.
  `ui/words-en.txt` and `/ui/credits.json` are the models of a file served to
  the page, the second from another package's bytes (`ui.go`).
- **The source:**
  `password.lst` at John the Ripper's last commit to touch it,
  2022-01-20 ("Default wordlist: Start with the previous tiny list"),
  15,327,454 bytes,
  SHA-256 `bd84dbc84ce113fcf0b9489a8d6f665e52db43b5332937ad7a216405cb8ab7cf`,
  the same at today's head.
  Its header: compiled by Solar Designer of Openwall 1996 to 2022,
  "It is assumed to be in the public domain."
  (since 2013); its first 3,546 entries are mid-1990s Unix passwords and 2006 to
  2010 "top N" lists,
  the later ones from Pwned Passwords v8 overlapped with RockYou.
- **What Openwall says elsewhere:**
  John the Ripper's `LICENSE` (GPL version 2 or later,
  relaxed terms per source file) does not name the list.
  The wordlists collection's licence:
  "You're allowed to use and redistribute this wordlists collection or parts
  thereof, with or without modification,
  provided that credit is given where it is due,
  any modified versions are marked as such,
  this license is kept intact and included with each copy,
  and NO FEE IS CHARGED FOR OBTAINING A COPY except as negotiated with the
  copyright holder.
  […] You're also not permitted to bundle this wordlists collection with a
  product you charge for."
  Its `passwords/password.gz` of 2011 holds 3,546 entries,
  byte for byte the first 3,546 of today's `password.lst` (run): D29.
- **The cut:**
  the 18 comment lines out, the first 10,000 entries,
  those of 8 to 64 characters, lower-cased, repeats out, sorted bytewise:
  3,510 lines, 33,834 bytes,
  SHA-256 `c9c01713d12ffe61c580c9f57a2924360a39739fa19da0e6314915168ceb11bd`;
  613 of them from the first 3,546 entries.
  All ASCII, the longest 20 characters;
  none matches the identifiers' patterns or a suggestion's shape,
  and no password of the tests' fixtures is in it (run).
- **The credits:**
  a `third_party/` directory with a `MASSIMO.md` table,
  a `LICENSE` and a `NOTICE` becomes an entry with no change to `tools/credits`;
  an empty Licence cell and a text named by no licence give "licence in its
  text"; `TestClassifyToday` needs the entry in `licencesToday`.
- **The texts' widths** were not measured: the widest German line today,
  39 characters, is 263 px of the Add-a-user panel's 279 px,
  and the new German "now" text has 39 characters.

### For Block 6: the crates' licence texts

- **What is vendored:**
  `opaque.js` (held by `TestVendoredOpaque`),
  `opaque.LICENSE` (the package's MIT text) and `README.md`,
  whose table names `@serenity-kit/opaque` 1.1.0 and Includes 21 crates:
  the 19 the WebAssembly names in its paths, `dlmalloc` and `hashbrown`;
  it says their texts are not reproduced,
  as `credits.includes` and `includesLine` do.
  `TestVendoredOpaqueCrates` holds Includes to the paths, both ways.
- **What "consistent with the artifact" can honestly mean:**
  - npm's provenance names the package's commit (2026-02-01,
    "Version Packages (#172)") and its release workflow;
    npm published 1.1.0 at 2026-02-01 14:40:35 UTC.
    The commit's `Cargo.toml` has edition 2018 and no resolver key,
    so resolver 1, and there is no lock file.
  - Resolved today, 4 of the 19 pins come out otherwise.
    With the 19 pinned and every other crate at the newest of its caret series
    published before the release, no crate of the closure is newer than the
    release and none is yanked; and 18 of the 19 pins are themselves the newest
    of their series then (the manifest pins the 19th exactly),
    so the rule gives the artifact's own versions back.
  - The closure for wasm32-unknown-unknown without dev and build dependencies
    and the 7 proc-macro crates: 51 crates, the 19 and 32 more,
    walked by hand, since cargo 1.75 cannot read an edition-2024 manifest;
    each archive's SHA-256 equals the index's checksum (run).
  - So: the 19 versions the WebAssembly names; every other crate at the newest
    of its series on crates.io at npm's publication;
    the manifest at the attested commit, resolver 1,
    the release build without macros and build scripts.
    A reconstruction, not the build's lock, which nobody has;
    the file says so in its head.
- **The texts:**
  the files at each crate's root named `LICENSE*`, `LICENCE*`, `COPYING*`,
  `NOTICE*` or `UNLICENSE*`, and the one its `license-file` names, CRLF to LF:
  99 files, 50 distinct by their bytes;
  the file in the proposed form 178,165 bytes,
  179,538 with `dlmalloc` and `hashbrown`, under the generator's 256 KiB cap;
  all valid UTF-8.
  `gloo-utils` 0.2.0, declared MIT OR Apache-2.0, has no licence file in its
  archive (D32).
- **How the credits take it up by its name,** with no change to `tools/credits`
  (read): `uiVendorEntries` passes the stem `opaque` to `licenceFiles`,
  which takes every `opaque.<x>` whose name is a licence's,
  `LICENSE-3RD-PARTY` among them, and makes it no entry of its own;
  it sorts after `opaque.LICENSE`, which stays the first;
  `classify` names it by no licence, so the entry reads Mixed: "MIT,
  other licences among its files", and the command line "MIT and others".
  `TestClassifyToday` needs it in `licencesToday`.
- **Growth:**
  `credits.json` from about 209 KB to about 391 KB,
  fetched uncompressed at the fold's first opening; the binary about 182 KB.
  A later `opaque.js` with more crates could pass the cap and stop
  `make credits`.

### For Block 7: the shipped paths and the test files

- **`project.shipped`** (`GUIDELINES/config.ini`) takes `cmd/**`, `internal/**`,
  `packaging/**`, `app/**` and the rest, and exempts no test file.
- **The check** (`changelog.sh check --staged`):
  a test-only commit passes without `--strict` by COMMIT.md §5.2's warning ("if
  the one in [Unreleased] it would amend is accurate"), and is refused with it;
  the message's "No CHANGELOG bullet" line is not read;
  where `[Unreleased]` is empty and no fragment exists,
  the plain check refuses too,
  as right after a release's cut (run in a scratch clone).
  CI's Guidelines job stages nothing, so it never applies the per-commit rule,
  but it holds the contract's generated block to the config.
- **The practice:**
  of the last 400 commits, 22 changed only test files among their shipped paths,
  none with a fragment, 21 with a "No CHANGELOG bullet" line;
  8 more changed the test kit.
- **No test file reaches a user:**
  273 `_test.go` files under `cmd/`, `internal/` and `packaging/`,
  which `go build` leaves out; `testdata/`,
  which no non-test `go:embed` reads (CI's fake Cube,
  the release's dump check and `make release` do);
  the five package test scripts (`make deb-test`, `deb-systemd-test`,
  `apt-test`, CI's macOS job); the app's `src/test/` and `src/androidTest/`;
  `nfpm.yaml` packages none.
  `make release` commits a store dump under `internal/store/testdata/releases/`
  with no check: afterwards it touches no shipped path.
- **Not tests, and shipped:**
  `packaging/go-test-report.sh` and `packaging/tested-record.sh`,
  CI's tools (D39); `internal/cubelan/fakecube/`,
  in the binary as `massimo fakecube`; the test kit,
  imported by tests alone but no `_test.go` (D36).
- **The form:**
  without glob magic `*` crosses `/`,
  so `:!*_test.go` takes a test file at any depth, the root's included;
  `:!**/*_test.go` needs a `/`.
  Both take all 273 today.
- **What else reads the list:**
  `fingerprint.sh`, to refuse a path both untested and shipped and as a guard on
  the site's shortcut; which tests CI runs does not depend on it,
  so a test-only commit still runs the whole suite.
- **The contract:**
  its block is written by `changelog.sh update-contract`,
  inlined by `sync-docs.sh` into `AGENTS.md` and `GUIDELINES/README.md`;
  its version rises with a `File-revision-v1` trailer,
  without which `check-versions` fails (tried).

### For Block 8: the credits' versions

- **Today:**
  `tools/credits` writes each Go module's version into
  `internal/credits/credits.json` (24 modules),
  which the binary embeds and `/ui/credits.json` and `massimo credits` serve;
  Go's own entry takes its version at run time already (`internal/credits`,
  `GoVersion`, from `runtime.Version()`, PasswordRules' Block 5).
  `TestGeneratedFileCurrent` compares the committed file with a fresh run,
  so a version bump in `go.mod` alone fails it.
- **What the binary knows:**
  `runtime/debug.ReadBuildInfo` lists the modules it was built with,
  each with its version and, for a replaced one, its replacement.
- **What still fails, as it should:**
  a module added or removed, and a licence text that changes with a bump.
- **The vendored entries** (the page's libraries, the OPAQUE client,
  the word list) take their versions from their notes, as now:
  their bump is a change of the file anyway.

## Implementation Plan

Each block moves its documents with its code and is a pull request of its own,
its run recorded in a NOTE (AGENTS.md §5.4);
the completion gate (`GUIDELINES/shared/GATE_WORKFLOW.md` §7.7) lists D1 to D39
and puts D59 again (D23).
The builders may run several blocks at once; Block 5 waits for D29,
and nothing of D41 is built before the owner's answer.

**Files more than one block touches,** which the coordinator merges in turn,
each later pull request merging `origin/main` first:

- `docs/ui.md`:
  Blocks 1 to 6.
- `internal/server/ui/i18n/*.json`:
  Blocks 2, 3, 5 and 6.
- `internal/server/users_api.go` and `users_api_test.go`: Blocks 2 and 3.
- `docs/api-v1.md`, `docs/api-v1-openapi.yaml` and the generated `src/api.d.ts`:
  Blocks 2, 3 and 5;
  a conflict in `api.d.ts` is settled by `go run ./tools/apitypes`, not by hand.
- `internal/server/ui/style.css`:
  Blocks 2 and 4; `src/shared/password.tsx`: Blocks 4 and 5.
- `cmd/massimo/user.go` and `user_test.go`: Blocks 3 and 5.
- `tools/ui-test/test_first_owner.py`:
  Blocks 3 and 4.
- `internal/credits/` (written by `make credits`),
  `tools/credits/generate_test.go` and `internal/server/ui_test.go`:
  Blocks 5 and 6;
  a conflict in `internal/credits/` is settled by `make credits` again.
- `src/signin/`:
  Block 1 (`Card.tsx`, `store.ts`) and Block 3 (`Owner.tsx`), different files.
- `tools/credits/`, `internal/credits/` and `cmd/massimo/credits.go`:
  Blocks 6 and 8; Block 8 is built on Block 6's commit.
- Block 7 shares none.

Block 4 is best merged before Block 5:
Block 5's texts can make the rule's line taller, which Block 4's measure sees.

### Block 1: Late

- **Goal:**
  an answer of any way of the sign-in card that comes once its opening has ended
  writes nothing, takes no focus and sets no token,
  and a session it made is ended (LooseEnds.D44-A, D1, D2).
- **Changes:**
  1. `src/signin/store.ts`:
     `opening()`, beside `ask()`: the card's opening and the session's number
     taken at a submit's start, and `still()`,
     whether an answer after an await may still act on them;
     `submitKey` takes it in place of its own line, its behaviour the same.
  2. `src/signin/Card.tsx`, `submitPassword`:
     the opening taken where the ask is taken now; after `signIn`,
     a refusal writes its line and asks for the focus only while `still()`;
     a session that comes late sets no token and is ended (`endSession`);
     after the keeping nothing more is needed (`finishSignIn` checks
     `ask.closed`, `askKeep` drops).
  3. `fingerprint`:
     the same two checks after `login.fingerprint()`; a late token is not set,
     and the bridge is handed the page's own token back
     (`setToken(state.token)`),
     which ends the fingerprint's session it held (D2).
  4. The providers' buttons:
     `startProvider` (`login.js`) returns the provider's address,
     and the card leaves the page for it only while `still()`;
     a refusal writes `card.providersSaid` likewise.
  5. `login.js`:
     `endSession(token)` beside `setToken`, `POST /api/v1/logout` with it,
     not awaited, a refusal ignored;
     `src/plain/login.d.ts` declares it and `startProvider`'s new type.
  6. The first owner's form: unchanged (D1).
  7. `Card.tsx`'s `@file` header, whose last sentence says it of the key alone,
     says it of every way.
- **Tests,** in `tools/ui-test/`,
  each seen failing on main first for the reason it names:
  - `test_login.py`:
    `test_a_late_password_answer_makes_no_token`:
    the password's `login/finish` held, the key signs in,
    the held answer let go: the page's token and its storage keep the key,
    who is in is the key's,
    and the operator's live sessions are those before the test;
    `test_a_late_password_refusal_writes_nothing`:
    on a running page a wrong password's `login/start` held,
    the card closed and opened anew, a name typed and the focus put:
    the refusal let go writes no line and takes no focus.
  - `test_login.py`, the key's two tests as LooseEnds' Block 6 NOTE left them:
    `test_a_late_key_answer_makes_no_token`'s docstring names the reason it
    fails for;
    `test_a_late_key_refusal_writes_nothing`'s focus assertion made able to
    fail, the focus on a control the new opening shows.
  - `test_sign_in_updates.py`:
    `test_a_late_fingerprint_makes_no_token`: the fake bridge's
    `login.fingerprint` held from the page, the fingerprint tapped,
    the password signs in, then the finger answers:
    the page's token and the bridge's are the password's session,
    the bridge's last `setToken` having handed it back;
    a late refusal writes no line.
  - `test_oidc.py`:
    `test_a_late_provider_start_stays_on_the_page`: the provider's start held,
    its button pressed, the password signs in, the start let go:
    the page stays where it is, and nothing is written into a later opening.
- **Documents:**
  `docs/ui.md` (the sign-in: the late answer's paragraph says every way,
  and why the first owner's form needs no check).
- **CHANGELOG:**
  `CHANGELOG.d/Fixed/claude-accessends-late-sign-in.md`:
  "- A way of the sign-in card that answers once another has signed in changes
  nothing: its session is ended, and no provider's page opens."
- **Commit:**
  `fix(ui): A sign-in's answer after another way signed in changes nothing`
- **Verification,** each by its exit code: `make ui-check`; `make ui`,
  then `MASSIMO_ENV_FILE=/dev/null go test ./internal/server/...` (`TestUIPage`,
  `TestUINoHTMLInjection`, `TestUIFileHeaders`);
  `PYTEST_ADDOPTS='test_login.py test_sign_in_updates.py test_oidc.py test_first_owner.py test_ways.py test_keep_open.py test_sign_in_resend.py test_parts_guard.py' make ui-test` in all four cases; the whole suite once, `UI_TEST_CASES=1100-en make ui-test`; `make app-emulator-test` where KVM is open (the bridge's `setToken`), CI's job Android emulator in any case; `GUIDELINES/shared/tools/check`.
  No Go changes: no `go vet`, no `make cross`.

### Block 2: Users

- **Goal:**
  an integration's user's Role list offers the roles within its integration's
  maximum and `integration` (LooseEnds.D45-A, D3 to D6);
  then Users on an open server says "No owner yet" and lists an integration's
  users, with nothing to press (LooseEnds.D46-A, D7 to D10).
- **Changes,** the first commit (D45):
  1. `internal/server/users_api.go`:
     `userMay` gains `Roles []string` (`json:"roles,omitempty"`),
     the keys of the roles the caller may give the user,
     disabled ones too as the list shows them, by `mayAssign`;
     for an integration's user only `integration` and the roles whose
     capabilities lie within the ceiling and its integration's `max_caps`,
     owner never, by its key.
     `listUsers` reads the roles once and the integrations once a request,
     the second as `integrationMarks` does;
     `putUser`'s answer the same for its user.
  2. `mayAssign` takes the user: `integration` may be given back to an
     integration's own user, and is refused to any other as now (D5).
     No refusal of a role beyond the maximum (D6).
  3. `src/admin/sendUsers.ts`:
     `roleOptions(current, u)` offers `u.may.roles` where given,
     in the list's order, the current one kept;
     without them, from an older server, as now;
     Add a user's choice leaves out `integration`,
     which the server refuses to a new user.
  4. `src/admin/Users.tsx`:
     `UserForm` passes the user;
     the role's `Select` has the hint `users.roleAboveMax` where the user is an
     integration's and its role is not among `may.roles` (D4).
  5. `ui/i18n/*.json`:
     `users.roleAboveMax`, "Beyond its integration's maximum:
     it acts within that maximum all the same." / "Über dem Maximum seiner
     Integration: es handelt trotzdem nur innerhalb dieses Maximums.".
- **Changes,** the second commit (D46):
  6. `src/admin/Users.tsx`, `UsersCard` on an open server:
     the head's badge `users.noOwnerYet`, warn, in place of the count (D8);
     `OpenServer` lists `users.list` under its line in `#users-open-list`,
     a row a user keyed by its name: its name and the badges of `badges(u)`,
     no fold, form, keys, identities or Remove (D7);
     under it a button `users.openIntegrations` that shows Integrations
     (`showCard("sys-integrations")`).
  7. The line `users.noOwnerWithIntegrations` where an integration's users are
     listed, `users.noOwner` otherwise (D9).
  8. `src/integrations/store.ts`:
     the runner's reload also reads the Users list
     (`window.MassimoParts.users.load()`) where the caller may manage users,
     as `src/profile/send.ts` does (D10).
  9. `ui/i18n/*.json`:
     `users.noOwnerYet` "No owner yet" / "Noch kein Eigentümer";
     `users.noOwnerWithIntegrations`, D9's words / "Noch kein Eigentümer:
     der Server steht allen im lokalen Netz offen.
     Die Benutzer einer Integration handeln nur über sie und melden sich
     nirgends an.";
     `users.openIntegrations` "Open Integrations" / "Integrationen öffnen".
     In both commits FR, IT, NL and PL machine-translated and marked;
     `style.css` for the rows, phone width first.
- **Tests,** each seen failing on main first:
  - `internal/server/users_api_test.go`:
    `TestAssignableRolesOfAnIntegrationsUser`:
    an integration with the maximum read, setpoint and configure (`registerHA`):
    `may.roles` of its two users is guest and `integration`, not owner, manager,
    operator or viewer;
    a regular user's holds owner to viewer and no `integration`;
    the maximum widened to the ceiling brings operator and viewer in; viewer,
    then `integration`, given to the integration's user, 200 each,
    and `integration` to a regular user, 403.
  - `tools/ui-test/test_users.py`:
    `test_an_integrations_role_choice_stays_within_its_maximum`:
    the users' read answered with `INTEGRATION_USERS` given `may.roles`
    guest and `integration`, one of them stored as owner:
    the first offers guest and integration;
    the second guest, integration and owner, owner selected, with the hint;
    the operator's own panel still offers owner;
    Add a user offers no `integration`; every button named,
    no horizontal scroll.
  - `test_ways.py`:
    `test_users_on_an_open_server_list_an_integrations_users`:
    an open server whose users' read is `INTEGRATION_USERS`:
    the badge "No owner yet", the two rows by their chain with their badges,
    nothing to press in the list, Create the owner shown,
    Open Integrations shows its card;
    with no users, the badge, no list and `users.noOwner`.
  - `test_integrations.py`, `test_register_and_link`:
    after Approve the Users head counts two more without System settings shown
    again.
  - `test_parts_guard.py`:
    a view of the region `users-open` with the list drawn,
    so that a new read of who is in keeps its rows.
- **Documents:**
  `docs/api-v1.md` ("Users": `may.roles`; "Integrations":
  an integration's user's roles, `integration` given back);
  `docs/api-v1-openapi.yaml` (`may.roles`),
  `src/api.d.ts` by `go run ./tools/apitypes`; `docs/ui.md` ("Users":
  an integration's user's role choice and its line; the open server's badge,
  list and way to Integrations; the part's paragraph).
- **CHANGELOG:**
  `CHANGELOG.d/Fixed/claude-accessends-integration-roles.md`:
  "- Users offers an integration's user only the roles within its integration's
  maximum, and the role integration again.";
  `CHANGELOG.d/Fixed/claude-accessends-open-users.md`:
  "- Users on a server without an owner says so and lists an integration's
  users."
- **Commits:**
  `fix(api): An integration's user is offered the roles within its maximum`,
  `fix(ui): Users on an open server lists an integration's users, no owner yet`.
- **Verification,** each by its exit code:
  `make ui`, then `MASSIMO_ENV_FILE=/dev/null go test ./...`;
  `MASSIMO_ENV_FILE=/dev/null MASSIMO_TEST_SCALE=2 go test -race -count=1 ./internal/server`;
  `gofmt -l .` prints nothing;
  `go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...`;
  `go run ./tools/apitypes -check`; `make cross`; `make ui-check`;
  `PYTEST_ADDOPTS='test_users.py test_ways.py test_keep_open.py test_integrations.py test_first_owner.py test_parts_guard.py' make ui-test`
  in all four cases; the whole suite once, `UI_TEST_CASES=1100-en make ui-test`;
  `GUIDELINES/shared/tools/check`.

### Block 3: OpenSetup

- **Goal:**
  the first owner is told once, by the page, the API,
  the command line and the start,
  what was set up while the server was open and still works,
  and the log keeps it as a log alert until it is seen (LooseEnds.D47-A with a
  log entry, D11 to D15).
- **Changes:**
  1. `internal/store/opensetup.go` (new): `OpenSetup(ctx)`:
     the integrations active with a live key (uuid and name),
     the sign-in providers enabled (`auto_create` marked),
     the roles added or changed (not built in,
     or built in at a version above 1), the capabilities turned off; `Empty()`.
     At the first regular user it is what was set up while there was none.
     Requests still waiting are not listed (D15).
  2. `internal/server/users_api.go`, `postUser` on an open server,
     once the owner is written, where `OpenSetup` is not empty:
     a Warn line, "the first owner was made; what was set up while the server
     was open still works: check it", with `user`, `integrations`,
     `providers`, `roles`, `capabilities_off` and `remote`;
     a system event of the new kind `open_setup`, level warn,
     its summary the user and the four lists (`logServer`),
     settled by a person's mark alone;
     and the answer's new field `open_setup`, the same lists, only then.
     `first owner created` stays.
  3. `internal/server/server.go` at the start and `SetToken` (`auth.go`) at a
     reload of the settings file: where `Bootstrap` made the owner from
     `MASSIMO_API_TOKEN`, the same line and event (D14);
     the start reads `Bootstrap`'s result, which it drops today.
  4. The kind in `eventKinds` (`log.go`), `LOG_KINDS` (`src/log/kinds.ts`),
     its summary in `src/log/text.ts`.
  5. `cmd/massimo/user.go`, `add`: the regular users counted before;
     where there were none and `OpenSetup` is not empty after the add,
     the same event written into the store (`InsertEvent`, source system,
     level warn, `via: "cli"`), and a second line on standard output,
     such as "set up while the server was open, still in effect:
     1 integration (kitchen), roles changed: viewer;
     check them in System settings" (D12).
  6. `src/signin/Owner.tsx`:
     the creation keeps the POST's answer; where it holds `open_setup`,
     the toast says `setup.done`, then `setup.openSetup` with what,
     in the warn tone, for 30 s with its close button (D13),
     with an action Show that opens System settings → Integrations,
     or the log where no integration is listed (D11).
  7. `ui/i18n/*.json`, FR, IT, NL and PL machine-translated and marked:
     `setup.openSetup` "Set up while the server was open, and still in effect:
     {what}.
     Check it." / "Eingerichtet, als der Server offen war, und noch wirksam:
     {what}.
     Prüfe es."; `setup.openSetupShow` "Show" / "Zeigen";
     `log.kind.open_setup` "open server" / "offener Server";
     `log.sum.openSetup` "First owner {user};
     still in effect from the open server: {what}" / "Erster Eigentümer {user};
     noch wirksam aus dem offenen Server: {what}"; the parts of what,
     `openSetup.integrations`, `openSetup.providers`, `openSetup.roles`,
     `openSetup.capabilities`.
- **Tests,** each seen failing on main first:
  - `internal/store`:
    `TestOpenSetup`: an empty store lists nothing; an approved integration,
    an enabled provider,
    a changed viewer and a capability turned off are each listed;
    a waiting request,
    a refused or revoked integration and a disabled provider are not.
  - `internal/server/users_api_test.go`:
    `TestFirstOwnerSaysWhatWasSetUp`:
    on an open store an integration requested and approved and a role changed,
    then the first owner: the answer's `open_setup` lists them,
    the store holds one `open_setup` line at warn,
    `GET /api/v1/log/alerts` lists it for the owner,
    and the server's log has the Warn line (the test kit's log watch);
    on a clean open store, no field and no line.
  - `internal/server/keepopen_test.go`, beside `TestOpenWarningAtTheStart`:
    `TestTokenOwnerSaysWhatWasSetUp`: a server started with
    `MASSIMO_API_TOKEN` on a store holding an approved integration and no
    regular user writes the line, and so does a reload that brings the token;
    one with a regular user does not.
  - `cmd/massimo/user_test.go`:
    `TestUserAddSaysWhatWasSetUp`: a store with an approved integration,
    `user add anna -role viewer -password-stdin`:
    the second line and the stored event; none on a clean store,
    none for a second user.
  - `tools/ui-test/test_first_owner.py`:
    `test_the_owner_is_told_what_was_set_up`: the creation answered with an
    `open_setup` that names an integration: the toast says `setup.done`,
    then `setup.openSetup` with its name, warn, and Show opens Integrations;
    `test_create_the_owner_from_the_card` holds the toast without it.
  - `TestLogKinds` and the log's text cases fail until the kind is in all four
    places.
- **Documents:**
  `docs/api-v1.md` ("Authentication": the first owner and `open_setup`;
  the log's server lines and the alerts settled by a mark name `open_setup`);
  `docs/api-v1-openapi.yaml` (the 201 of `POST /api/v1/users`,
  a schema `OpenSetup`), `src/api.d.ts` by `go run ./tools/apitypes`;
  `README.md` (`massimo user add` on an open store);
  `docs/store.md` section 6; `docs/ui.md` (the first owner's toast,
  the log's kinds).
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-accessends-open-setup.md`:
  "- The first owner is told what was set up while the server was open and still
  works, in the page, by `massimo user add` and in the log."
- **Commit:**
  `feat(server): The first owner is told what was set up while the server was open`
- **Verification,** each by its exit code:
  `make ui`, then `MASSIMO_ENV_FILE=/dev/null go test ./...`;
  `MASSIMO_ENV_FILE=/dev/null MASSIMO_TEST_SCALE=2 go test -race -count=1 ./internal/server ./internal/store`;
  `gofmt -l .` prints nothing; `go vet` for linux, windows and darwin;
  `go run ./tools/apitypes -check`; `make cross`; `make ui-check`;
  `PYTEST_ADDOPTS='test_first_owner.py test_keep_open.py test_ways.py test_log.py test_parts_guard.py' make ui-test`
  in all four cases; the whole suite once, `UI_TEST_CASES=1100-en make ui-test`;
  `GUIDELINES/shared/tools/check`.

### Block 4: Tall

- **Goal:**
  the card's measure tests take a card taller than the window as right where it
  starts at its top and the page scrolls (PasswordRules.D57-A, D16 to D18);
  then a line under the field shows the whole suggestion while it is shown
  (PasswordRules.D58-A, D19 to D22), and the dice's move is measured (D23).
- **Changes,** the first commit (D57, tests alone):
  1. `tools/ui-test/helpers.py`:
     `assert_login_card_placed(page, what)` beside `assert_login_card_centred`:
     the room is the window less the body's padding;
     a card within it is centred as now;
     a taller one has its top at the body's padding at scroll 0 (16 ± 1 px)
     and its bottom inside the window at the page's end,
     every control of the card in view at one of the two,
     no horizontal scroll, left and right and the title still centred (D17).
  2. `assert_login_card_centred` measures "fits" against the room (D18).
  3. `test_first_owner.py`'s `test_the_owner_card_measures`
     and `test_keep_open.py`'s `test_keep_open_measures` take the new helper,
     and measure the card after a press of the dice too,
     in each language at 390 px,
     by a local press (`test_password_suggest` imports `test_first_owner`);
     `test_the_owner_card_on_a_short_screen` takes it for its three asserts.
     The sign-in card's callers keep the strict helper (D16).
- **Changes,** the second commit (D58):
  4. `src/shared/password.tsx`, `PasswordField`:
     while the field holds the suggestion,
     `<p id={id + "-shown"} class="pw-shown" aria-hidden="true" translate="no">`
     with it, right under the field, before the rule's line (D20, D22);
     its place two lines tall from the first press until the row goes,
     empty after a key (D19);
     the doc comments of `PasswordField` and `NewPassword`.
  5. `style.css`, beside `.pw-suggest`: `.pw-shown`, the whole width,
     the body's font at 16 px (D21), `overflow-wrap: anywhere` as a guard,
     a `min-height` of two lines; the comment above it.
  6. No new text: the line holds the password alone,
     and the rule's line keeps "Suggested; this password can be set."
- **Tests:**
  - First commit: the owner's card after a press in the app at 390 px in French
    fails the strict helper on main and passes the new one; a fault planted,
    the body cut to the window's height with its overflow hidden,
    fails the new helper's reach.
  - Second commit, `tools/ui-test/test_password_suggest.py`,
    each new assertion failing before (no line):
    `SPOKEN` leaves out text under `aria-hidden="true"`; `check_suggest`:
    no line before the first press, after it the field's value (words,
    then Characters), `aria-hidden` and `translate="no"`, at most two lines,
    not wider than the field, in the five forms; `edited` and `reset_ends`:
    no password in the line after a key, gone after Reset;
    `test_the_row_stays_while_one_types`:
    `PLACES` gains the line's top and height, unchanged by a key;
    `test_shown_with_the_eye_off`: the line with the eye off; new,
    the widest five words (`STUB_RANDOM`) in a user's panel at 390 px:
    two lines, inside the field, no horizontal scroll.
    The first commit's measure tests then see the card taller than the window
    in five languages in the app at 390 px, and pass.
  - The dice's move at the first press measured in each case and language,
    the numbers in the block's NOTE (D23).
- **Documents:**
  `docs/ui.md` (the password fields: the line, when it shows,
  why a screen reader does not hear it,
  and the heights after a press in place of the 28 px and the French card;
  the card's rule for a taller card stands).
- **CHANGELOG:**
  the first commit none (`tools/` is not shipped, `:!tools/**`);
  the second amends `CHANGELOG.d/Added/claude-password-suggest.md`,
  the dice not released yet (COMMIT.md §5.2):
  "- A dice beside a new password's field suggests a password of five words
  or twenty characters, shown whole under the field."
- **Commits:**
  `test(ui-test): The first owner's card taller than the window is measured at its top`,
  `feat(ui): A suggested password shows whole under its field`.
- **Verification,** each by its exit code: `make ui-check`; `make ui`,
  then `MASSIMO_ENV_FILE=/dev/null go test ./internal/server/...`;
  on the first commit's tree alone,
  `PYTEST_ADDOPTS='test_first_owner.py test_keep_open.py' make ui-test` in all
  four cases; on the second,
  `PYTEST_ADDOPTS='test_password_suggest.py test_first_owner.py test_keep_open.py test_password_rule.py test_password_names.py test_password_eye.py test_copy.py test_login.py test_connect.py test_parts_guard.py' make ui-test` in all four cases; the whole suite once, `UI_TEST_CASES=1100-en make ui-test`; screenshots at 1100 and 390 px; `GUIDELINES/shared/tools/check`.
  The owner's check on a phone, by hand: the line, the French card scrolling,
  the dice's move.

### Block 5: Common

**It waits for D29:**
written as D29-B builds it; dropped under D29-A, the completion gate saying why,
and waiting under D29-C for Openwall's answer;
D24 to D28 go with it.

- **Goal:**
  the commonest passwords are refused, from Openwall's `password.lst`
  (PasswordRules.D55-A, D24 to D28).
- **Changes:**
  1. `internal/password/common.txt` (new): the cut,
     made by the pipeline in `MASSIMO.md` from `password.lst` at John the
     Ripper's commit of 2022-01-20: 3,510 lines, 33,834 bytes, SHA-256
     `c9c01713d12ffe61c580c9f57a2924360a39739fa19da0e6314915168ceb11bd`.
  2. `internal/password/password.go`:
     `go:embed`; the set built once; `Common() []byte` for the server's route;
     `ErrCommon`, "password: a common password"; `Check`: the length,
     then the whole password lower-cased in the set, then the names (D26);
     the package comment loses its "no list" sentence,
     and the cases' description gains the answer `common`.
  3. `internal/server/ui.go`:
     `GET /ui/common-passwords.txt`, as `/ui/credits.json` is:
     `password.Common()`, `text/plain; charset=utf-8`, an ETag of the version,
     no token (the first owner's form has none); the file's comment names it.
  4. `src/shared/password.tsx`:
     the answer `common`, `SAID.common`; `commonPasswords()`,
     fetched once as the word list is (`src/shared/suggest.ts`), its form held,
     a failure tried again at the next call,
     started by the first key in a new password's field (D27);
     `passwordProblem` checks the list after the length where it is loaded,
     and the line redraws when it comes;
     a submit of the five forms waits for it 5 s at most,
     then checks the rest and warns in the console;
     `MassimoParts.commonPasswords` for the tests.
  5. `cmd/massimo/user.go`:
     the rename's hint for `ErrCommon` too.
  6. `third_party/openwall-passwords/` (new): `LICENSE`,
     the source's 18 header lines as they stand; `NOTICE`,
     in the form of `third_party/eff-wordlist/NOTICE`:
     the address at the pinned commit, both SHA-256 values, what was changed,
     that the header's sentence is the compiler's assumption and no licence,
     that the 2022 entries come from Pwned Passwords v8 and RockYou as the
     header says, and the collection's licence of the first 3,546 entries,
     quoted (D28); `MASSIMO.md`, its table's Licence cell empty (D24),
     and the pipeline (D25).
  7. `ui/i18n/*.json`, FR, IT, NL and PL machine-translated and marked:
     `password.common` "A common password:
     easy to guess." / "Ein häufiges Passwort: leicht zu raten.";
     `password.refusedCommon` "Refused: a common password." / "Abgelehnt:
     ein häufiges Passwort.";
     each measured at 390 px within the Add-a-user panel,
     `RULE_TEXTS` reserving the tallest.
  8. `make credits`:
     the directory becomes an entry;
     `tools/credits/generate_test.go`'s `licencesToday` gains it.
- **Tests,** each seen failing on main first:
  - `internal/password`:
    `TestCommonList`: `common.txt` at its SHA-256, 3,510 lines, sorted, unique,
    each 8 to 64, lower case, printable ASCII, a final newline,
    and `MASSIMO.md` naming both SHA-256 values;
    `cases.json` grows by the answer `common` (`password1`,
    `PASSWORD1` and `12345678` common, `password1x` ok,
    a common entry holding a name common by D26);
    `TestErrorTexts` gains `ErrCommon`.
  - `TestUsersPasswordRule` (`internal/server/users_api_test.go`):
    `password1` answered 400 `password: a common password`;
    `TestUserPasswordRule` (`cmd/massimo/user_test.go`): refused.
  - `TestUIServed`:
    `GET /ui/common-passwords.txt` 200, `text/plain`,
    its body `password.Common()`.
  - `tools/ui-test/test_password_rule.py`:
    `Password1` says the common line in each form;
    the parity test awaits the list first;
    a submit with the list's fetch held sends nothing until it is let go,
    then is refused;
    a list that fails gives the length and the names and a console warning.
- **Documents:**
  `docs/api-v1.md` ("Users": the 400);
  `docs/api-v1-openapi.yaml` (the two `password` descriptions),
  `src/api.d.ts` by `go run ./tools/apitypes`; `README.md` (`massimo user`);
  `docs/ui.md` (the line's texts, the `password.tsx` row,
  the route and its fetch).
- **CHANGELOG:**
  `CHANGELOG.d/Security/claude-password-names.md` amended,
  not released yet (COMMIT.md §5.2):
  "- A new password is refused where it is a common one or contains the
  user's or the server's name."
- **Commit:**
  `fix(security:auth): A common password is refused`
- **Verification,** each by its exit code:
  `make ui`, then `MASSIMO_ENV_FILE=/dev/null go test ./...`;
  `MASSIMO_ENV_FILE=/dev/null MASSIMO_TEST_SCALE=2 go test -race -count=1 ./internal/server`;
  `gofmt -l .` prints nothing; `go vet` for linux, windows and darwin;
  `make credits`, then `go run ./tools/credits -check`;
  `go run ./tools/apitypes -check`; `make cross`; `make ui-check`;
  `PYTEST_ADDOPTS='test_password_rule.py test_password_names.py test_password_suggest.py test_first_owner.py test_users.py test_system_info.py' make ui-test`
  in all four cases; the whole suite once, `UI_TEST_CASES=1100-en make ui-test`;
  `GUIDELINES/shared/tools/check`.

### Block 6: Crates

- **Goal:**
  the credits hold the licence texts of the Rust crates inside the vendored
  OPAQUE client, collected consistent with the artifact (PasswordRules.D56-A,
  D30 to D34).
- **Changes:**
  1. `tools/opaquecrates/` (new, Go, with a README): from the committed list,
     each crate's name, version and crates.io checksum (D30),
     it fetches each archive, checks its checksum,
     takes the files at its root named `LICENSE*`, `LICENCE*`, `COPYING*`,
     `NOTICE*` or `UNLICENSE*` and the one its `license-file` names, CRLF to LF,
     folds byte-identical texts alone (D34) and writes the file.
     No Rust is needed.
  2. The list: the 51 crates of the closure for wasm32-unknown-unknown,
     the 19 the WebAssembly names and 32 resolved,
     with `dlmalloc` 0.2.11 and `hashbrown` 0.16.1 (D31),
     at the file's head or in `opaque.crates.txt` beside it;
     how it was resolved said, not run again.
  3. `internal/server/ui/vendor/opaque.LICENSE-3RD-PARTY` (new): its head says,
     in its own words, what it holds and how the versions were had:
     the 19 the WebAssembly names;
     every other crate at the newest of its series on crates.io when npm
     published the package, 2026-02-01 14:40:35 UTC,
     resolved from the manifest at the commit npm's provenance names,
     resolver 1, without macros and build scripts; a reconstruction,
     not the build's lock, which nobody has; Rust's `core`,
     `alloc` and `std` named by the compiler's commit,
     their texts not here (D31).
     Then each crate `==> <name> <version> (<declared licence>) <==`,
     each file `--- <file>` and its text, or the same text as one above;
     `gloo-utils` 0.2.0: "No licence file in its archive; declared MIT OR
     Apache-2.0" (D32).
     Not in `go:embed`: it reaches the page through `credits.json` (D33).
  4. `internal/server/ui/vendor/README.md`:
     "not reproduced here" becomes what the file holds and how its versions were
     had; the update's steps gain the tool, then `make credits`.
  5. The Includes text: `credits.includes` in six languages, "Includes {what};
     their licence texts are among its files." / "Enthält {what};
     deren Lizenztexte stehen unter seinen Dateien.", FR, IT,
     NL and PL machine-translated and marked;
     `includesLine` (`cmd/massimo/credits.go`);
     the comment of `Entry.Includes` (`internal/credits/credits.go`).
  6. `make credits`:
     the OPAQUE entry takes the file by its name and reads Mixed;
     `tools/credits/generate_test.go`'s `licencesToday` gains the file.
- **Tests,** each seen failing on main first:
  - `internal/server/ui_test.go`, `TestVendoredOpaqueCrates` grown:
    every crate of the WebAssembly's paths has a head at the same version;
    every head a file or the line that its archive has none;
    every "the same text as" names a head above it;
    the head names the commit and the cut-off.
    Offline.
  - `tools/opaquecrates`:
    its own test on a made-up index in a temporary directory: the folding,
    a crate without a licence file, an archive whose checksum differs refused.
  - `tools/credits`:
    `TestClassifyToday` with the new file.
  - `tools/ui-test/test_system_info.py`:
    the OPAQUE entry lists `opaque.LICENSE-3RD-PARTY` among its files,
    with the new Includes text.
- **Documents:**
  `docs/ui.md` ("System info"),
  `tools/credits/README.md` (a vendored file's further licence files).
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-credits.md` amended,
  not released yet (COMMIT.md §5.2):
  "- System info lists the credits and licences of everything the server
  includes, the OPAQUE client's crates among them,
  and `massimo credits` prints them."
- **Commit:**
  `fix(server): The credits hold the licence texts of the OPAQUE client's crates`
- **Verification,** each by its exit code:
  `go run ./tools/opaquecrates` once, online, its file committed;
  then offline `make ui`, `MASSIMO_ENV_FILE=/dev/null go test ./...`;
  `make credits` twice with the same bytes, `go run ./tools/credits -check`;
  `gofmt -l .` prints nothing; `go vet` for linux, windows and darwin;
  `make cross`;
  `PYTEST_ADDOPTS='test_system_info.py test_parts_guard.py' make ui-test` in
  all four cases; the whole suite once, `UI_TEST_CASES=1100-en make ui-test`;
  `GUIDELINES/shared/tools/check`.

### Block 7: Changelog

- **Goal:**
  a commit that changes only tests touches no shipped path,
  and owes no CHANGELOG bullet (LooseEnds.D48-A, D35 to D39).
- **Changes:**
  1. `GUIDELINES/config.ini`, `project.shipped`, after the site's pages,
     by hand (`git config --add` would put the lines after `changelog =`):
     a comment, that the tests are not shipped and a commit changing only them
     owes no bullet (D37), then `:!*_test.go`, `:!**/testdata/**` (D35),
     `:!packaging/apt/test.sh`, `:!packaging/deb/test.sh`,
     `:!packaging/deb/systemd-test.sh`, `:!packaging/deb/systemd-cases.sh`,
     `:!packaging/launchd/template-test.sh`,
     `:!app/android/app/src/test/**` and `:!app/android/app/src/androidTest/**`.
     The test kit and CI's tools stay shipped (D36, D39).
  2. `GUIDELINES/shared/tools/changelog.sh update-contract` writes the block of
     `GUIDELINES/project/_CONTRACT.md` again; its title v1.19.0 to v1.20.0.
  3. `GUIDELINES/shared/tools/sync-docs.sh` writes `AGENTS.md` and
     `GUIDELINES/README.md` again.
  Nothing under `GUIDELINES/shared/` changes.
- **Tests:**
  none of the suite; the tools, in a scratch clone with the change committed,
  each by its exit code with `changelog.sh check --staged --strict`:
  `internal/store/audit_test.go` alone staged, 0,
  "owes no bullet" (1 before the change); with `internal/store/audit.go`, 1;
  every group of test files at once, 0; with `packaging/deb/postinst` too, 1.
- **Documents:**
  the config's comment and the contract's generated block;
  `GUIDELINES/project/CI.md` as it is, CI not changing.
- **CHANGELOG:**
  none: every path is under `GUIDELINES/` or a `.md` file, none shipped;
  `Key changes` says so ("No CHANGELOG bullet: none of these paths is shipped",
  D38).
- **Commit:**
  `docs(contract): A commit changing only tests owes no CHANGELOG bullet`,
  with the trailer
  `File-revision-v1: GUIDELINES/project/_CONTRACT.md::v1.20.0::<date>::1::<what>`.
- **Verification,** each by its exit code:
  the four checks above in the scratch clone;
  `GUIDELINES/shared/tools/sync-docs.sh --check`;
  `GUIDELINES/shared/tools/check` (the contract's block, the versions,
  the headings, the citations);
  `GUIDELINES/shared/tools/commit-msg.sh` on the message, 0 errors;
  `changelog.sh check --staged --strict --message <file>` on the block's own
  commit, "owes no bullet".
  `config.ini` is not `[ci] untested`,
  so CI runs the whole suite on the pull request.

### Block 8: Versions

- **Goal:**
  a bump of a Go module in `go.mod` fails no test;
  the credits show the version the binary was built with (D40-A).
- **Changes:**
  1. `tools/credits`:
     a Go module's entry carries no version in `internal/credits/credits.json`;
     a vendored entry keeps its note's.
  2. `internal/credits`:
     each Go module's version is filled when the list is read,
     from `debug.ReadBuildInfo` (a replaced module: its replacement's version),
     as Go's own version is from `runtime.Version()`;
     a module the build information lacks shows no version.
  3. `/ui/credits.json`, System info and `massimo credits` show the versions as
     before.
  4. `make credits` again.
- **Tests:**
  `internal/credits`: the versions filled are the build information's,
  a replaced module's too;
  `tools/credits`: the generated file holds no Go module's version,
  and `TestGeneratedFileCurrent` passes with a module's version changed in its
  list of modules (the test's own stub) and fails with a module added;
  `cmd/massimo`: `massimo credits` shows a module's version.
  Each seen failing before the change.
- **Documents:**
  `tools/credits/README.md` (what the file holds, what a bump needs),
  `docs/ui.md` (the credits), `README.md` where it names the versions.
- **CHANGELOG:**
  `CHANGELOG.d/Fixed/claude-accessends-credits-versions.md`:
  "- The credits take each Go module's version from the binary,
  so a dependency's update no longer needs the list made again."
- **Commit:**
  `fix(server): The credits take each Go module's version from the binary`
- **Verification,** each by its exit code:
  `go test -count=1` of `internal/credits`, `tools/credits`,
  `cmd/massimo` and `internal/server`; `make credits` twice, the same bytes;
  `go run ./tools/credits -check`; in a scratch copy,
  the bump of that Dependabot pull request (`golang.org/x/net` 0.62.0 to 0.63.0,
  from the Go module proxy),
  then `go test ./tools/credits/...` passes and `massimo credits` shows 0.63.0;
  `make ui-test` with `test_system_info.py` and `test_parts_guard.py` in all
  four cases; `make cross`.

## Proposed commit message

The plan: `chore(aps): +AccessEnds v1.0`

Block 1:

```
fix(ui): A sign-in's answer after another way signed in changes nothing

For AP AccessEnds v1.0, Block 1 (LooseEnds.D44-A, D1-A, D2-A): every way of the sign-in card now does what the key's way does since LooseEnds' Block 6. An answer that comes once its opening has ended (another way signed in meanwhile, the card closed or opened anew, a sign-out) writes no line, takes no focus and sets no token: a password's or a fingerprint's session that comes late is ended, the page keeping the token the other way set, and a provider's start that comes late leaves the page where it is. The first owner's form keeps its own guard, which no user reaches past.

Key changes:
- src/signin/store.ts: opening(), the opening and the session taken at a submit, and whether an answer
  may still act on them; submitKey takes it.
- src/signin/Card.tsx: the password's, the fingerprint's and the providers' ways check it after each
  answer; the header says every way.
- login.js, src/plain/login.d.ts: endSession logs a late session out; startProvider returns the
  provider's address for the card to open.
- Tests: test_login.py (a late password's session and refusal; the key's two tests as LooseEnds'
  Block 6 NOTE left them), test_sign_in_updates.py (a late fingerprint), test_oidc.py (a late
  provider's start).
- Docs: docs/ui.md (the sign-in, every way's late answer); a Fixed fragment.

Assisted-by: claude-code:claude-opus-5-5
```

Blocks 2 to 8: the summaries of their blocks above.

## Change History

- v1.0 (2026-11-25): first revision, from the owner's answer at the completion
  gate of AP LooseEnds and AP PasswordRules: LooseEnds.D44-A to D48-A,
  D47-A with a log entry, and PasswordRules.D55-A to D59-A, under PLAN++;
  D1 to D39 taken with PLAN++, but D29, the owner's, on which Block 5 waits.
- v1.1 (2026-11-25): Block 8, Versions, at the owner's word (D40-A):
  the credits take each Go module's version from the binary,
  since every bump in `go.mod` failed CI until the list was made again;
  D41 open, the owner's, with what a link to a Massimo user changes.
  Block 7 is merged; Blocks 1 to 6 stand as they are.
