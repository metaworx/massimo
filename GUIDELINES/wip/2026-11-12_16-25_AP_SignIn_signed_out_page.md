# AP SignIn v1.0: signed-out page

## TL;DR

While the page asks who is there, it shows Massimo, its version and the sign-in card,
and nothing of the last session;
the card sits in the page rather than over it,
holds one password field,
and its ☰ menu sets the language and the mode before anyone signs in.

- Sign out → page still drawn behind the dialog (rooms, Cube, clock, nav)
  → Cube data cleared, polling stopped; header = name + version only
- sign-in → modal `<dialog>` (`showModal`, rest of the page inert)
  → the same `<dialog>`, non-modal (`show`), a card in the empty page
- one form, two password fields (password, API key), no `name`
  → the password form alone with `name="username"`/`"password"`;
  the API key in a form of its own
- ☰ on the card → language and mode (light, dark, as the system),
  the same settings as My settings

### Blocks

1. Signed out: the page's class while asked, the header and main hidden,
   Sign out clears the Cube's data; ui-test; docs/ui.md
2. The card: non-modal, one password field, the key's form; ui-tests; docs/ui.md
3. The card's menu: language and mode, i18n in six languages; ui-test; docs/ui.md

## Discussion

The owner found on 2026-11-12, on breno's server (v0.14.0-alpha.1):

- after Sign out, the rooms, the Cube choice, the clock and the buttons
  stay drawn behind the sign-in dialog;
- KeePassXC-Browser in Firefox does not recognise the password field,
  and cannot be pointed at it by hand;
  in Chrome its dropdown on the name field opens,
  and choosing an entry only closes it,
  while filling from the toolbar's icon works.

The owner asked that the header show only the name and the version while
signed out, any setting such as the language going into a ☰ menu in the
sign-in card.

**Why KeePassXC fails, as evaluated:**
`showModal()` puts the dialog in the browser's top layer and makes the rest
of the document inert.
The extension draws its dropdown and its field picker into the document,
so a click on them never reaches them:
the dropdown closes, nothing is filled.
Filling from the toolbar needs no click in the page, and works.
The form also holds two password fields (the password and the API key),
which a password manager may take for a change of password,
and its fields have no `name`.

**The owner's decisions**, 2026-11-12, `PLAN++ SignIn` on the evaluation's gates:

- **D11-A.**
  The sign-in is a card in the page, not a modal dialog:
  nothing is behind it to hide, and nothing is inert.
- **D12-A.**
  The card's ☰ holds the language and the mode,
  both kept per browser and so usable signed out.

**Names chosen here**, routine and not decisions of the owner's:
the class `signed-out` on `body`;
the `<dialog id="login-dialog">` kept, opened with `show()`,
so that the scripts and the tests that ask whether it is open stay as they are;
`#login-key-form` for the API key, its button "Sign in" (`login.submit`);
`#login-menu`, `#login-lang`, `#login-mode`;
one new text, `login.menu`;
the branch `claude/sign-in`.

**Not in this plan:**
sign-in through OpenID Connect (AP Oidc, whose provider buttons will sit on this card);
the colour scheme, which stays in My settings;
the app's connect page (`connect.html`), which has no sign-in.

**Who runs it:**
this session, under the owner's `PLAN++ SignIn` of 2026-11-12:
the plan, its registration on main, and the three blocks committed on `claude/sign-in`.
No gate stated a push of the branch for this plan,
so the completion gate offers the push and the pull request.

## Analysis

**How the sign-in starts** (`login.js`):
`api()` meets a 401 and calls `askLogin()`,
which opens the dialog and resolves when a session or a key is in,
or false when the dialog is closed.
The page's start (`loadMeAndPreferences`) and Sign out wait on it;
a session that ends while the page runs is signed in again by `afterLogin`,
which reloads everything.
Sign out (`logout`) forgets the token and the user's settings, closes the stream,
and calls `loadMeAndPreferences`, which meets the 401;
nothing clears `state.cubes`, `state.rooms` or the drawn tiles,
and the poll keeps its timer.

**What the class hides:**
in the header everything but `.brand` (the name and `#version-short`):
`.updated-line`, `#clock-plain`, `.cube`, `.nav`, `#reload-btn`, `#tls-notice`,
`#cube-found`, `#budget`, `#pending`;
and `main`.
The class is set when the dialog opens and removed when it closes,
by a sign-in or by a close (only a script closes it once it is non-modal).
A session that ends while the page runs hides the page too,
and its data reloads after the sign-in as before;
only Sign out clears the data, since the next to sign in may be someone else.

**Non-modal:**
an open non-modal `<dialog>` is in the document's flow,
answers `open` as before, takes no Escape and has no backdrop;
the CSS places it as a card under the header.
`test_restart.py` closes it by script and still can.

**The key's form:**
today one submit serves both; a key typed wins.
With a form of its own, Enter in the key's field and its button sign in with the key;
`test_preferences.py` clicks that button.

**The menu:**
the language choice is filled as My settings' (`renderLangSelect`),
the mode's options are those of `#theme-select`;
both call the same setters (`setSetting`, `changeLanguage`, `chooseMode`),
so a choice on the card is the choice in My settings,
kept in the browser and, once a user with `preferences` signs in,
taken over or replaced as `loadPreferences` does today.

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `internal/server/ui/login.js` | 1, 2, 3 | `signed-out` while asked; `clearCubeData` at Sign out; `show()`; the key's form; the menu |
| `internal/server/ui/app.js` | 1, 3 | `clearCubeData` (state, tiles, header, poll); `renderLangSelect` fills both choices |
| `internal/server/ui/index.html` | 2, 3 | the dialog's two forms, `name`s, the ☰ menu |
| `internal/server/ui/style.css` | 1, 2, 3 | `body.signed-out`; the card's place; the menu |
| `internal/server/ui/i18n/*.json` | 3 | `login.menu` in all six |
| `tools/ui-test/test_login.py`, `test_preferences.py` | 1, 2, 3 | Sign out leaves an empty page; non-modal; one password field; the key's form; the menu |
| `docs/ui.md` | 1, 2, 3 | the sign-in |
| `CHANGELOG.d/` | 1, 2, 3 | a fragment per bullet |

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`GUIDELINES/shared/tools/check`,
and `make ui-test` (1100 and 390 px), its new and changed tests first.
Nothing reaches a Cube or a box.
One functional commit per block, citing `AP SignIn v1.0, Block <n>`.

### Block 1: Signed out

1. `login.js`: `askLogin` sets `signed-out` on `body` as it opens the dialog,
   `done` removes it.
   `logout` calls `clearCubeData()` before it asks.
2. `app.js`: `clearCubeData()` empties the Cubes, the found ones, the rooms,
   the devices' details and the tiles' states,
   stops the poll, forgets the last poll,
   and draws the tiles, the Cube choice and the header empty.
3. `style.css`: `body.signed-out` hides the header's parts but `.brand`, and `main`.
4. `tools/ui-test/test_login.py`: after Sign out no room tile is drawn or in
   `state`, and of the header only the name and the version are visible;
   after the sign-in the rooms are back.
   Run first on the old page, where it must fail.
5. `docs/ui.md`, the opening: what the page shows while it asks.
6. `CHANGELOG.d/Fixed/claude-sign-in-signed-out.md`:

   ```
   - Signed out, the web UI shows only its name, its version and the
     sign-in, where Sign out left the last user's rooms, Cube and clock
     drawn behind it.
   ```

**Verification:** the test gate.

### Block 2: The card

1. `login.js`: `show()` for `showModal()`;
   the key's form submits the key, the password form the name and the password.
2. `index.html`: `#login-form` with `method="post"`, the inputs with
   `name="username"` and `name="password"`;
   the `<details>` of the key holds `#login-key-form` with `#login-token`
   and its button.
3. `style.css`: the dialog while signed out a card under the header,
   centred, at phone width the page's width less its margins.
4. Tests: `test_login.py`, the dialog is open and not modal
   (`:modal` false), the login form holds one password field
   and its fields have their `name`s, a click beside the card reaches the page;
   `test_preferences.py`, the key signs in through its own form.
5. `docs/ui.md`: the card, the key's form.
6. `CHANGELOG.d/Fixed/claude-sign-in-card.md`:

   ```
   - The sign-in is a card in the page instead of a modal dialog, with one
     password field, so that password managers such as KeePassXC fill it.
   ```

**Verification:** the test gate;
the owner's KeePassXC check in Firefox and Chrome against a scratch server or
the branch on a box, which the completion gate asks for.

### Block 3: The card's menu

1. `index.html`: in the card's head a `<details id="login-menu" class="menu">`
   whose ☰ button is labelled `login.menu`,
   with `#login-lang` and `#login-mode` as `.field` rows,
   labelled `settings.language` and `appearance.mode`,
   the mode's options those of `#theme-select`.
2. `app.js`: `renderLangSelect` fills `#login-lang` as `#lang-select`;
   `login.js` wires both choices to `setSetting`/`changeLanguage` and `chooseMode`,
   and keeps them in step with My settings (`applyAppearance`, `applyLanguage`).
3. `style.css`: the menu in the card's head.
4. `i18n/*.json`: `login.menu`, "Language and appearance",
   in all six, FR/IT/NL/PL machine-translated as their files are marked.
5. `tools/ui-test/test_login.py`: signed out, the menu changes the language
   (the card's title follows) and the mode (`data-theme`),
   and My settings shows both after the sign-in.
6. `docs/ui.md`: the menu.
7. `CHANGELOG.d/Added/claude-sign-in-menu.md`:

   ```
   - The sign-in card's ☰ menu sets the language and the light or dark
     mode before anyone signs in.
   ```

**Verification:** the test gate;
the card at 390 px with the menu open, from the suite's screenshot.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work branch:

```
chore(aps): +SignIn v1.0

After Sign out the web UI kept the last user's rooms drawn behind the sign-in dialog, and KeePassXC could not fill the modal dialog's password field; the owner asked for a signed-out page that shows only the name and the version, with a menu for the language in the sign-in card. PLAN++ SignIn of 2026-11-12, with D11-A and D12-A.

Key changes:
- GUIDELINES/wip/2026-11-12_16-25_AP_SignIn_signed_out_page.md: the plan, three blocks, the decisions in its Discussion

File-revision-v1: GUIDELINES/wip/2026-11-12_16-25_AP_SignIn_signed_out_page.md::v1.0::2026-11-12::All::the first revision, by PLAN++ SignIn
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
Block 1, `fix(ui): Signed out, the page shows only its name, its version and the sign-in`;
Block 2, `fix(ui): The sign-in is a card in the page with one password field`;
Block 3, `feat(ui): The sign-in card's menu sets the language and the mode`.

## Change History

- v1.0 (2026-11-12): the first revision,
  from the owner's findings on breno and the owner's D11-A and D12-A of the same day.
