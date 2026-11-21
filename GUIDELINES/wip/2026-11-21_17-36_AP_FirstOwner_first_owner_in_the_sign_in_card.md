# AP FirstOwner v1.0: first owner in the sign-in card

## TL;DR

A server without users asks whoever opens its page to create the owner,
in the sign-in card,
and can be kept open on purpose instead, by a setting on the server.

- open server → the sign-in card with the create-owner form, at every load
- three ways out → create the owner · skip for now · keep open
- keep open → a confirmation: the warning, the full address,
  the address entered → a setting on the server
- kept open → no card; the page loads to Rooms; the banner stays
- undone → a switch in System settings; gone once a user exists
- no store → no card, as today
- the banner → "Create the owner…", and Hide, kept per browser
- ☰ → "Create the owner…" where Sign out stands
- untouched → the API's open mode, the app's bridge

### Blocks

1. Server: the setting, what `/me` says of it, the first owner created once
2. Card: the create-owner form in the sign-in card, skip, when it opens
3. KeepOpen: the confirmation with the address, the switch that undoes it
4. Ways: the banner's button and hiding, ☰'s entry, Users' section

## Discussion

### What is decided

The owner, 2026-11-21, on the phone check's finding
that the first owner's form is far down the page and asks nobody
(AP MobileApp's F30, its D61);
asked through that plan's session, evaluated by this one,
decided in this session, in the owner's words where quoted:

- **D1. Keep open is remembered on the server.**
  "With the card getting three options:
  1. create an owner 2. skip for now 3. keep open";
  "the create-owner form is shown instead of the login-form every time
  until either a user exists or the keep-open setting is set";
  "once the keep-open setting is set, nor the login-from,
  nor the create-owner form are shown.
  instead the house panel is loaded and shown."
- **D2. The banner can be hidden.**
  "as long as there is no owner,
  a user may decide to hide the open-server banner -
  stored the local browser/app."
- **D3. A plan of its own, by this session,**
  built after AP UiParts' sign-in half is on main.
- **D4. Keep open asks first:**
  a plain button under the form, "opening a confirmation with
  warning text (the one mentioned), a text showing the full url,
  an text input to copy the full proto://host:port info into
  and a confirmation button".
  The warning: "Not recommended - ONLY use it on a local network you trust!"
- **D5-A. Keep open is undone by a switch in System settings,**
  and is cleared by itself when the first user is created.
- **D6-A. The setting lives with the server's other settings, in its store.**
  A server without a store gets no card and stays as it is.
- **D7-A. On a server kept open the owner can still be created:**
  from the banner's button, unless the banner is hidden,
  and from System settings → Users.
- **D8-A. The address in the confirmation can be copied with one tap,**
  so that on a phone it is pasted, not typed.
- **D9-A. An entry "Create the owner…" in the main menu ☰,**
  where Sign out stands on a server with users.

### Decided in the writing

Each as recommended, and each the owner's to change:

- **D10-A. "Skip for now" holds for that load of the page.**
  Nothing is stored; the next load asks again,
  as "every time" says.
- **D11-A. The setting is `auth.keep_open`,** a switch among the server's settings,
  off unless set, read and written by the settings' routes as any other.
  On an open server everybody on the local network may write it:
  that is what open means.
  `GET /api/v1/me` says `kept_open` on an open server,
  so that the page knows at its first read whether to ask.
- **D12-A. Cleared when the first user is created** means:
  the setting counts only while the server has no users,
  and the server removes it where it finds users and the setting both:
  as the first owner is created by the API, at its start and at a reload
  (the owner made by `massimo user add` or from `MASSIMO_API_TOKEN`).
  Should a server ever lose all its users, it asks again.
- **D13-A. The address entered is compared with the page's own origin,**
  scheme, host and port as the browser has them:
  letters' case and a slash at the end do not matter;
  Confirm is off until it matches.
- **D14-A. System settings → Users, on a server without users, opens the card.**
  One form, not two: the server refuses any first user but an owner,
  and the section's own form offers roles.
- **D15-A. The first owner is created once.**
  Two requests at the same moment can both create an owner today
  (the server reads its mode once per request,
  and the store does not look again):
  the store checks inside its transaction, and the second request is refused.
- **D16-A. The tests:** the server's behaviour in Go tests;
  the page's in UI tests that answer `/me` and the settings themselves,
  as the tests of the open banner do.
  No test runs a second server.

### Still open

- **D17. Integration users count as users.**
  An integration approved on an open server makes users of the role
  `integration`, and the server then counts as one with users:
  no card asks, nobody can sign in, and only `massimo user add` helps.
  Found in the writing; it is so on main today.
  - **A (recommended).** Named here and left to AP IntegrationUsers,
    whose subject it is; this plan's card asks while `/me` says open.
  - **B.** In Block 1: an integration cannot be approved
    while the server has no owner. It changes that plan's flow.
- **D18. The phone as its own server.**
  AP MobileApp's local server runs open at a loopback address:
  the app would be asked at every start until it is kept open.
  - **A (recommended).** Nothing special here:
    that plan's block sets the setting or creates an owner when it is built.
  - **B.** No card where the page is reached at a loopback address:
    wrong where the same server also answers the local network.

### What stays as it is

- The API's open mode: every request from the local network may do everything,
  and a caller beyond it is refused.
- A server without a store: its banner, no owner, no card.
- `massimo user add`, `MASSIMO_API_TOKEN` and `-token`.
- The app: its bridge, its Cancel and its switch for the login by fingerprint
  on the first owner's form; "skip" and "keep open" need nothing of it.
  A server's line under the card's title in the app is AP MobileApp's (its D62).

## Analysis

Read on main and on the sign-in half's tree, which is not on main yet;
this plan builds on it (D3).

- **The sign-in card never shows on an open server.**
  It opens only after a request was refused for lack of a session
  (`askLogin`, `login.js`), and an open server refuses none.
  The card's part takes the waiting request when it is opened
  and knows two ways, the password's and the key's:
  it needs a third state, and a way to open with no request behind it.
- **The first owner's form is a tile among the rooms,**
  mounted while `/me` says `setup`,
  which on an open server is always the case:
  a caller beyond the local network is refused before `/me` answers.
- **The card's forms share one height** (AP UiParts D224,
  for the phone): the create-owner form is taller than both.
- **A setting is cheap on the server:**
  rows of key and value, no migration;
  a switch is one entry among the switches (`settings_api.go`)
  and a key with its reader in `internal/store/settings.go`.
  The settings' type in the page does not change; `/me`'s does,
  and `make api-types` follows the OpenAPI file.
- **Nothing hooks the first user's creation today,** and it has four paths:
  the API, `massimo user add` (a process of its own, on the store),
  the owner made from `MASSIMO_API_TOKEN` at a start, and at a reload.
  Hence D12's form.
- **The banner has no control,** and nothing of it is stored.
- **The shared question** has a text and two buttons;
  it takes no field, and its Yes cannot wait for one.
  A text that copies itself, a text field, a switch saved at once
  and a busy button that can be off exist as shared parts.
- **No UI test runs against an open server:**
  every one answers `/me` itself (D16).
- **☰ hides Sign out on an open server already;**
  the new entry takes that place.

## Implementation Plan

Each block moves its documents with its code,
and ends with the gate of `GUIDELINES/shared/GATE_WORKFLOW.md`.
Block 2 starts once AP UiParts' sign-in half is on main;
Block 1 does not wait for it.

### Block 1: Server

- **The setting** `auth.keep_open`: its key and reader in the store,
  its entry among the switches, its default off.
- **`GET /api/v1/me`:** `kept_open`, true on an open server where the setting is set,
  absent otherwise.
- **Removed with users** (D12): in the transaction that creates the first owner
  by the API; at the server's start and at a reload,
  where the server has users and the setting both.
- **The first owner once** (D15): the store's check in its transaction;
  the API answers the second request `409`.
- **Documents:** `docs/api-v1.md` ("Authentication", `/me`, the settings),
  `docs/api-v1-openapi.yaml`, `internal/server/ui/src/api.d.ts`
  by `make api-types`, `docs/store.md`.
- **Tests:** the setting set, read and reset on an open server;
  `/me` in each mode; the setting gone after the first owner by each path;
  two first owners at once.

**Verification:** the test gate of the project contract
(`make ui`, then `MASSIMO_ENV_FILE=/dev/null go test ./...`),
`make race` for the touched packages, `gofmt` and `go vet` for three platforms,
`go run ./tools/apitypes -check`, `TestOpenAPICoversRoutes`.

### Block 2: Card

- **The card's third state,** the first owner's:
  the create-owner form in the sign-in card, with its name's check,
  its two passwords, the app's switch and Cancel as they are today;
  its tile among the rooms goes.
- **When it opens:** at the page's start and after each read of who is in,
  where the server is open, may have an owner (D6) and is not kept open,
  and this load has not skipped it.
  The page behind it is hidden as at a sign-in.
- **Create the owner:** today's request, in today's order; then the page, signed in.
- **Skip for now** (D10): the card closes, the page goes on as today.
- **The card's height** on a phone (UiParts D224) with the third form,
  and no focus put into a field on a touch screen.
- **Documents:** `docs/ui.md` (the sign-in), the README's first start
  (`docs/fragments/`), `packaging/README.md` where it tells the first start.
- **Tests:** the card on an open server, with the form; none where the server
  is kept open, has no store, or has users; skip, and the card again at the next
  load; the owner created; the guard's region of the form, in the card;
  the card's measures at 390 px in six languages.

**Verification:** `make ui-check`; the sign-in's UI test files and the guard
in all four cases; the whole UI suite in two cases;
screenshots of the card at 390 and 1100 px.

### Block 3: KeepOpen

- **The button** under the form, and **its confirmation in the card** (D4, D8, D13):
  the warning, the page's origin as a text that copies itself,
  a field for it, Confirm off until it matches, and the way back to the form.
- **Confirm** writes the setting, reads who is in again, and the page loads.
  A refusal is said in the card.
- **The switch that undoes it** (D5): in System settings → Users,
  shown while the server has no users, saved at once.
  Off again, the next load asks.
- **The texts** in six languages, the four machine-translated ones marked.
- **Documents:** `docs/ui.md` (the sign-in; System settings → Users),
  `docs/api-v1.md`'s words on an open server.
- **Tests:** Confirm off for a wrong address and on for the right one,
  entered and pasted; the request sent; a refusal; the page after it;
  the switch.

**Verification:** as Block 2's, with `test_users.py` and `test_admin_updates.py`.

### Block 4: Ways

- **The banner:** "Create the owner…", which opens the card with the form,
  and Hide, kept in the browser's storage for that server (D2);
  a hidden banner stays hidden while there is no owner.
- **☰:** "Create the owner…" where Sign out stands (D9),
  on a server that may have an owner and has none.
- **System settings → Users** on such a server opens the card (D14).
- **Documents:** `docs/ui.md` ("Header", the banner, Users).
- **Tests:** each of the three opens the card; the banner hidden and still hidden
  after a load; ☰'s entries on an open server and on one with users;
  the guard's views of ☰ and the banner.

**Verification:** as Block 2's, with `test_main_menu.py` and `test_open_banner.py`;
at the end the whole UI suite in all four cases and `make cross`.

## Proposed commit messages

- The plan: `chore(aps): +FirstOwner v1.0`
- Block 1:
  `feat(server): An open server can be kept open by a setting`,
  `fix(server): The first owner is created once`
- Block 2:
  `feat(ui): A server without users asks for its owner in the sign-in card`
- Block 3:
  `feat(ui): An open server is kept open after its address is entered`
- Block 4:
  `feat(ui): The owner can be created from the banner, the menu and Users`

## Change History

- v1.0 (2026-11-21): first revision, by `PLAN+`,
  from the owner's decisions on the phone check's finding
  that the first owner's form asks nobody (AP MobileApp's F30):
  D1 to D9 as the owner chose, D10-A to D16-A as recommended,
  D17 and D18 open.
