# AP PasswordRules v1.0: Rules and help for a new password

## TL;DR

A new password gets a rule of 8 to 64 characters and no common or obvious word,
said in words while it is typed, held alike by the page,
the command line and the server; the page offers a generated password,
and System info lists the credits and licences of everything the binary
includes.

- new password → 8 to 64 code points, wherever one is set;
  existing ones stay valid
- "a" for the first owner → refused, with the reason under the field
- score meter → none; one line of words, `role="status"`
- common passwords → about 2,100 refused,
  with the names of the user and the server
- one rule → `internal/password` in Go, `shared/password.tsx` in the page,
  the same cases
- dice beside the eye → five EFF words, or 20 characters
- System info → credits and licences, made by the build, checked by a test

### Blocks

1. Rule: 8 to 64 in Go: the command line, the users' API, the fixtures
2. Words: the rule and its line under the field in the page's five forms
3. Common: the list of common passwords and the context words, page and Go
4. Suggest: the dice beside the eye, words or characters, the word list
5. Credits: System info's credits and licences, their generator and test

## Discussion

### What is decided

The owner, 2026-11-24,
on `wip/2026-11-24_15-48_ANALYSIS_PasswordRules_rules_and_meter.md`
and Part 1 of `wip/2026-11-24_19-00_ANALYSIS_SecondFactor_generator_and_second_factor.md`,
at the gate of AP LooseEnds, whose numbers these are:

- **D4-A.
  At least 8 characters wherever a password is set:**
  the page and the command line.
  Existing passwords stay valid.
- **D6-A.
  No score meter.**
  A line of words under the field while typing: too short, too long,
  a common password, contains your name or the server's name;
  otherwise that it is fine.
  A `role="status"` region that speaks only when its message changes;
  nothing shown by colour alone.
  Not chosen: a hand-written meter (it would rate "Password1!" strong);
  nothing beyond the length.
- **D7-A. 8 to 64, fixed, counted in code points;**
  a hint that 15 characters or a few words are better.
  The upgrade of a version-1 user at sign-in is exempt.
  Not chosen: 15 (what NIST asks for a password used alone); an owner's setting.
- **D8-A.
  No required character classes.**
- **D9-A.
  No required score.**
- **D10-A.
  A small embedded list of common passwords:**
  SecLists' `10k-most-common.txt` kept to entries of 8 or more characters (about
  2,100 entries, 19 KB), plus the context words: the login name,
  the display name, `massimo`, `maxcube`, the server's name.
  Whole password, case-insensitive.
  One file, read by the page and by Go from the same bytes.
  A condition: SecLists is MIT-licensed,
  but the list's own origin and terms are not checked;
  Block 3 checks them first, and where they do not hold,
  the list is left out and the context words alone are refused (its step 3.1).
- **D11-A.
  The rule holds in the page's five forms** through one function in
  `shared/password.tsx`; in `massimo user` (`cmd/massimo/user.go`);
  and in the server for the plain `password` field of the users' API,
  which it sees (400).
  A client that registers by an OPAQUE `record` can still set any password:
  said in `docs/api-v1.md`.
- **D12-A.
  No owner's settings for these rules.**
- **D14-A.
  A generator in the page.**
- **D15-C.
  Words and characters, with a switch:**
  characters are 20 from an alphabet of 56 without the look-alikes `0 O 1 l I |`.
- **D16-A.
  EFF's long list of 7,776 English words in every language**
  (CC BY: an attribution).
- **D17-A.
  Five words, lower case, joined by `-`.**
- **D18-A.
  The word list is a static file under `internal/server/ui/`,** embedded,
  fetched when the generator is first used; the words alone, about 62 KB.
- **D19-A.
  A dice icon beside the eye** in the first new-password field of each form.
  A press fills the field or fields and shows the password in the field;
  another press gives another one.
  A line under the field then offers Words or Characters
  (the choice remembered in this browser) and Copy.
  The suggestion shows also where the server has switched the eye off
  (`auth.password_reveal`): for the suggested value,
  until it is edited, saved or the form closes.
  Labelled "Suggest a password", keyboard operable.
  Picking is unbiased (`crypto.getRandomValues` with rejection).
  A suggestion goes through the same checks as a typed password.
  The icon is Tabler's.
- **D29-A.
  No list of 100,000 held by the server;** not now.
- **D30-A.
  Credits and licences in System info, now**
  (Block 5): every library, list and icon set the binary includes,
  each with its licence text,
  made by the build from `go.mod`'s modules and what is vendored,
  so that it cannot fall behind; a test fails where an entry is missing;
  shown as a part of System info fetched when opened.

### Decided in the writing

Each the owner's to change:

- **Five blocks, not four.**
  The brief's Rule block held the Go side and all five forms of the page,
  with 52 short passwords in Go tests and 67 in the UI tests' fixtures:
  more than one reading.
  Block 1 is the Go side, Block 2 the page.
- **The rule's package is `internal/password`:**
  the length (Block 1), the list and the context words (Block 3),
  and the cases both languages are tested with (`testdata/cases.json`).
  The store, the server and the command line import it.
  The list lives beside it (`internal/password/common.txt`, `go:embed`),
  because `cmd/massimo` cannot embed a file of `internal/server/ui/`;
  the server serves the very bytes to the page at `/ui/common-passwords.txt`.
- **The alphabet of 56.**
  Letters and digits without `0 O 1 l I` are 57, and `|` is not among them:
  the lower-case `o` goes too, the look-alike of `0` and `O`,
  so that it is 56 and a byte is redrawn at 224 (4 × 56); about 116 bits for 20.
- **Not normalised.**
  A password is counted and registered as typed:
  NFC in one place and not in another would make one password two.
- **The server's name:**
  in the page, the first label of the page's host where it is no IP address
  and not `localhost`, and in the app the server's name (`serverOf`,
  `signin/app.ts`); in Go, the first label of `os.Hostname()`,
  and in the server also of the request's `Host`.
- **A list not loaded:**
  the page fetches it as a password form is first typed in;
  a submit waits for it (at most 5 s), and where it cannot be had,
  the length alone is checked and the console says so.
- **The credits are served as a static file** (`/ui/credits.json`),
  as the page's files are: licences are no secret,
  and no route of the API changes.

### Taken with PLAN++

These were open in the draft; the run takes them as below,
each the owner's to change, and the completion gate lists them.

- **D50-A.
  Context words: whole or contained.**
  `massimo` and `maxcube` have 7 letters and most login names fewer than 8:
  as whole passwords the length refuses them already,
  so D10-A's "whole password" refuses almost nothing for them,
  while D6-A's line says "contains your name".
  - **A (recommended).**
    The list is matched whole;
    a context word of 4 or more letters is refused where the password contains
    it, case-insensitive ("anna2026", "Massimo-1"); the generator draws again.
  - **B.**
    Whole password for both, as D10-A reads; the line says "is your name".
- **D51-A.
  The Users forms: typed once or twice.**
  Adding a user and setting a user's password in System settings → Users
  have one field today; the first owner's and My profile's have two.
  - **A (recommended).**
    One field, with the rule's line and the dice:
    the manager passes the password on, sees it with the eye or the suggestion,
    and sets it again where it was mistyped.
  - **B.**
    Two fields, as `NewPassword`: a row more in each panel.
- **D52-A.
  The credits: made at each build, or committed and checked.**
  `make ui` makes `parts.js` uncommitted; `api.d.ts` is committed and checked
  (`go run ./tools/apitypes -check`).
  - **A (recommended).**
    `make credits` writes `internal/credits/`, committed;
    a Go test and `go run ./tools/credits -check` fail where it differs from the
    modules the binary links and the files vendored.
    `go build` and `go install` work without `make`,
    a dependency's update shows its licence in its diff,
    and it cannot fall behind.
  - **B.**
    Made by every build target, as `parts.js`, not committed:
    the module cache's licence files are needed at each build.
- **D53-B.
  The command line prints the credits too.**
  The owner asked for the credits "under System info",
  and a new command is an addition to the command line he has not asked for:
  `massimo credits` is offered again at the completion gate.
  - **B (taken).**
    System info alone.
  - **A (recommended in the draft).**
    `massimo credits`, the list, and `massimo credits <name>`,
    one licence's text, from the same embedded file:
    a box without a browser can still say what it ships.
- **D54-A.
  `massimo user rename` and an old short password.**
  A rename registers the password again under the new name
  (a version-2 record holds the name), so it asks for it again.
  - **A (recommended).**
    The rule holds: a rename is a password set,
    and a user with a short one gives a new one there (or `-no-password`);
    the refusal says so, and the README too.
  - **B.**
    The old one is taken where it opens the user's current record,
    by an OPAQUE sign-in in the process (`StartOpaque`, `FinishOpaque`): medium,
    for a rare command on the box.

### What stays as it is

- The sign-in: any password that opens a user's record or hash, of any length.
- The version-1 upgrade at sign-in (`upgradePassword`, `login.js`):
  it registers the same password by a `record`, which the server cannot check
  and the page does not.
- A user with keys only (`-no-password`, an empty field when adding).
- An OIDC provider's own rules.

## Analysis

Read on main, 2026-11-24.

- **The page's one check** is `newPasswordProblem(password, again)`
  (`src/shared/password.tsx`): the two differ, or none is typed.
  It is called by the first owner's form (`src/signin/Owner.tsx`,
  at the submit) and by My profile's `PasswordForm` (`src/profile/Profile.tsx`),
  which is Change password or Set a password by `has_password`:
  two of the five forms.
- **The Users forms check nothing** (`src/admin/Users.tsx`):
  `UserForm`, a user's panel, and `NewUser`, adding one,
  each have one `PasswordField` typed once; empty is "unchanged" in a panel
  and "keys only" when adding.
- **The first owner's fields are not controlled:**
  `NewPassword` with `inputRefs`, read at the submit (UiParts D210).
  A line while typing needs the first field's input events:
  `PasswordInput` has no `onInput` without `set`.
  The first owner's name field is likewise read through `nameRef`.
- **The eye** is `PasswordInput`'s one button, inside its `pw-box`,
  shown only where `reveal` (from `/healthz`'s `password_reveal`) is true;
  the dice is a second button there, shown whatever `reveal` says.
- **Icons** are symbols of the sprite in `internal/server/ui/index.html`
  (`<symbol id="icon-…">`), drawn by `Icon` (`src/shared/icon.tsx`);
  Tabler's are its paths copied in,
  each listed in `third_party/tabler-icons/MASSIMO.md` (Tabler 3.24.0,
  four today).
- **The command line** asks in `readPassword` (`cmd/massimo/user.go`) for
  `user add`, `user passwd` and `user rename`,
  refusing only a mismatch or an empty password,
  and registers by OPAQUE in process: it holds the password.
  `rename` asks again because the record holds the name (D54).
- **The server** sees a plain `password` in `POST` and `PUT /api/v1/users…`
  (`userRequest.Password`, `internal/server/users_api.go`):
  version 1, hashed by `st.AddUser`, `st.AddFirstOwner`, `st.SetPassword`;
  a `record` it never sees through.
  The page sends a `record` everywhere (`registerPassword`, `login.js`).
- **Short passwords in fixtures:**
  52 `password` literals under 8 characters in seven Go test files of `internal/server`,
  and `"pw"`, `"new"`, `"x"` in `cmd/massimo/user_test.go`;
  only those that set a password through the API or the CLI matter;
  `tools/ui-test/run.sh` adds the operator with `op-pass` by `massimo user add`,
  typed in 67 places of 20 files;
  `test_users.py` sets `rm-pass` and `test_opaque.py` `v1-pass` through the plain path;
  the scripts of `packaging/` and `tools/` use passwords of 10 or more.
- **The page's files** are embedded by one `go:embed` line in `internal/server/ui.go`
  and served under `/ui/`, revalidated (`no-cache`, an ETag);
  `ui/vendor/README.md` and `TestVendoredOpaque` hold the vendored OPAQUE client
  to its SHA-256: the model for the two lists.
- **System info** (`src/sysinfo/SysInfo.tsx`) shows the facts of `GET /api/v1/system`,
  with the store and the settings in folds; nothing of credits or licences.
- **What the binary includes:**
  the Go toolchain's library;
  the modules of `go.mod` it links (OPAQUE's `bytemare` modules,
  `modernc.org/sqlite` and `libc`, `golang.org/x/…`, `google/uuid`, `yaml.v3`,
  …), `bytemare/ksf` patched in `third_party/bytemare-ksf/`;
  Preact and its signals (`third_party/preact/`), Tabler's icons,
  `ui/vendor/opaque.js` with the Rust crates inside it; no font.
  Each `third_party/` directory has a licence file and a note on its origin.

## Implementation Plan

Each block moves its documents with its code,
ends with the gate of `GUIDELINES/shared/GATE_WORKFLOW.md`,
and is a pull request of its own.
Blocks 2 to 4 touch the first owner's form and the sign-in card:
they start once AP LooseEnds' Block 2 is on main;
Block 1 touches no file of the page and does not wait.

### Block 1: Rule

- **Goal:**
  a password the command line or the server sets has 8 to 64 code points.
- **Changes:**
  1. `internal/password` (new): `MinLength` 8, `MaxLength` 64,
     `ErrTooShort`, `ErrTooLong`, `Check(pw string, context ...string) error`
     counting with `utf8.RuneCountInString`; its package comment;
     `testdata/cases.json`, the cases with their answers.
  2. `cmd/massimo/user.go`:
     `readPassword` checks after the mismatch, for `add`, `passwd` and `rename`,
     and says "at least 8 characters"; the usage text says the rule;
     for `rename` as D54 settles.
  3. `internal/server/users_api.go`:
     a plain `password` that fails the check is answered 400
     (`password: at least 8 characters`, `…at most 64…`), in `POST` and `PUT`,
     before anything is written; a `record` as today.
  4. Fixtures: the Go tests that set a password through the API or the CLI;
     `tools/ui-test/run.sh`'s operator `op-pass` → `op-password` in all 67 places
     and in `tools/ui-test/README.md`; `rm-pass`, `v1-pass` lengthened.
- **Tests:**
  `internal/password`: every case of `cases.json` (7 and 65 code points refused,
  8 and 64 taken, `äöü` counted as three); `cmd/massimo/user_test.go`: `add`,
  `passwd`, `rename` refuse 7 and take 8; `internal/server/users_api_test.go`:
  400 for a short or long `password` in each of the two routes,
  a short `record` taken; `login_test.go`:
  a user stored with a 2-character password still signs in and is upgraded.
  Each refusal seen failing before the change.
- **Documents:**
  `README.md` (`massimo user`), `docs/api-v1.md` ("Users": the rule, the 400,
  a `record` not checked), `docs/api-v1-openapi.yaml` (`password`:
  `minLength` 8, `maxLength` 64), `src/api.d.ts` by `make api-types`.
- **CHANGELOG:**
  `CHANGELOG.d/Security/claude-password-length.md`:
  "- A password set with `massimo user` or the users' API has 8 to 64
  characters; passwords set before stay valid."
- **Commit:**
  `fix(security:auth): A new password has 8 to 64 characters`
- **Verification:**
  `make ui`, then `MASSIMO_ENV_FILE=/dev/null go test ./...` exit 0;
  `make race` exit 0; `gofmt -l .` prints nothing; `go vet ./...` for linux,
  windows and darwin exit 0; `go run ./tools/apitypes -check` exit 0;
  `make cross` exit 0; `make ui-test` with `test_login.py`,
  `test_users.py` and `test_opaque.py` in all four cases exit 0 (the fixture's
  new password).

### Block 2: Words

- **Goal:**
  the page's five forms refuse what Block 1 refuses, and say why while typing.
- **Changes:**
  1. `src/shared/password.tsx`:
     `passwordProblem(password, context)`,
     the rule of `cases.json` in TypeScript (`[...s].length`);
     `newPasswordProblem(password, again, context)` calls it after the mismatch;
     `PasswordRule`, the hint ("At least 8 characters;
     15 or a few words are better.") and the `role="status"` line,
     its text set only when the problem changes,
     a sign beside the words where it is drawn, never colour alone;
     `NewPassword` draws it under the first field,
     and listens to the first field where it is not controlled;
     `PasswordField` takes `rule` for the single fields.
  2. `src/signin/Owner.tsx`, `src/profile/Profile.tsx`: the line,
     and the submit's check.
  3. `src/admin/Users.tsx`:
     `NewUser` and `UserForm` with the line,
     checked at the submit where the field is not empty (D51).
  4. A handle for the rule on `window.MassimoParts`,
     for the UI test of `cases.json`.
  5. `ui/i18n/*.json`:
     the hint, too short, too long, fine, in six languages, FR, IT,
     NL and PL machine-translated and marked; `style.css`.
- **Tests:**
  `tools/ui-test/test_password_rule.py` (new):
  each of the five forms refuses 7 characters and says why, takes 8;
  the line says too short, then fine, and is read once per change;
  the cases of `cases.json` through the handle give Go's answers;
  the line at 390 px in German.
  Seen failing before the change.
- **Documents:**
  `docs/ui.md` (the sign-in's first owner, My profile, Users),
  the rule's row in "The shared parts".
- **CHANGELOG:**
  `CHANGELOG.d/Security/claude-password-rule-page.md`:
  "- The page refuses a new password under 8 characters and says why under the
  field while it is typed."
- **Commit:**
  `fix(security:ui): The page says what a new password lacks while typing`
- **Verification:**
  `make ui-check` exit 0; `make ui-test` with the new file,
  `test_first_owner.py`, `test_users.py`,
  `test_password_eye.py` and `test_parts_guard.py` in all four cases exit 0;
  the whole UI suite in two cases; screenshots at 1100 and 390 px.

### Block 3: Common

- **Goal:**
  the commonest passwords and the user's and server's names are refused.
- **Changes:**
  1. **The list's origin, a stop.**
     What SecLists says of where `10k-most-common.txt` came from (its README,
     the file's history) and under what terms, read and recorded.
     It holds where the file is SecLists' own under its MIT licence,
     or its source plainly allows its copying.
     **Stop:** where it does not hold or cannot be found,
     no list enters the repository: steps 2 and 3 are left out,
     the line's "a common password" is not added,
     and the block's gate says so.
  2. `internal/password/common.txt`:
     the entries of 8 to 64 characters, lower-cased and without repeats,
     from a pinned commit of SecLists;
     `third_party/seclists-common/` with `LICENSE` and `MASSIMO.md` (the commit,
     the filter, the SHA-256 of both files); `go:embed`.
  3. `Check`:
     `ErrCommon`, the whole password lower-cased in the list.
  4. `ErrContext`:
     the context words as D50 settles, with `massimo` and `maxcube`;
     the command line passes the name, the display name and the host's name,
     the server the user's names and its own (an option of the server,
     `os.Hostname()` by default, and the request's `Host`).
  5. `/ui/common-passwords.txt`, served from `internal/password`'s bytes;
     the page fetches it once a password form is typed in;
     `passwordProblem` checks it and the context words,
     each form passing its names (the first owner's typed name;
     My profile's and a panel's user; `NewUser`'s typed name and display name).
  6. The line's "a common password" and "contains your name or the server's name",
     in six languages.
- **Tests:**
  `cases.json` grows by common,
  context and near cases (an entry of the list in capitals refused,
  the same entry with a word added taken), both languages answering alike;
  `common.txt` held to its SHA-256, sorted, unique, 8 to 64;
  the CLI and the API refuse `password1` and the user's name;
  a UI test of each message, and of a submit waiting for the list.
- **Documents:**
  `docs/api-v1.md` ("Users"), `README.md` (`massimo user`), `docs/ui.md`.
- **CHANGELOG:**
  `CHANGELOG.d/Security/claude-password-common.md`:
  "- A new password is refused where it is a common one or contains the user's
  or the server's name."
- **Commit:**
  `fix(security:auth): A common password or one's own name is refused`
- **Verification:**
  as Block 1's, with `make ui-check` and the UI test files of Block 2 in all
  four cases.

### Block 4: Suggest

- **Goal:**
  a press on the dice puts a good password in the form.
- **Changes:**
  1. `internal/server/ui/words-en.txt`:
     EFF's long list, the words alone; the `go:embed` line in `ui.go`;
     `third_party/eff-wordlist/` with `MASSIMO.md` (the source, its date,
     the SHA-256 of the original and of the file,
     the dice numbers dropped) and the licence as eff.org states it (CC BY;
     its page says 4.0 and its badge 3.0 US: both named).
  2. The icon: Tabler's `dice-5` copied as `icon-dice` into the sprite of `index.html`,
     its row in `third_party/tabler-icons/MASSIMO.md`.
  3. `src/shared/suggest.ts` (new):
     `words()` and `chars()` with `crypto.getRandomValues`,
     redrawn at or above `floor(2^32 / 7776) × 7776` and at or above 224,
     the alphabet of 56; the list fetched once.
  4. `src/shared/password.tsx`:
     `PasswordInput`'s `suggest`, the dice beside the eye,
     labelled "Suggest a password";
     the suggested value shown as text whatever `reveal` says,
     until it is edited, the form is saved or reset;
     `NewPassword` fills both fields, a controlled field by `set`,
     one not controlled by its ref and an input event; the line under it:
     Words or Characters (`shared/choices.tsx`,
     kept in `localStorage` within try and catch) and Copy (`shared/copy.tsx`);
     a suggestion goes through `passwordProblem` and is drawn again where
     refused.
  5. The first new-password field of the five forms (four parts) gets `suggest`;
     texts in six languages; `style.css` for two buttons in the box at 390 px.
- **Tests:**
  `internal/server`: `words-en.txt` has 7,776 unique words of `a-z` and `-`,
  held to its SHA-256; `test_password_suggest.py` (new):
  in each form both fields filled and taken by the rule;
  another press changes it; shown with the eye off, hidden again once edited;
  Characters gives 20 of the alphabet; the choice kept after a load;
  Copy's status; the keyboard reaches the dice;
  a thousand draws through the handle all within the rule.
  Seen failing before the change.
- **Documents:**
  `docs/ui.md` (the password fields, the shared part's row).
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-password-suggest.md`:
  "- A dice beside a new password's field suggests five words or twenty
  characters, and Copy copies it."
- **Commit:**
  `feat(ui): A dice beside a new password suggests one`
- **Verification:**
  `make ui-check` exit 0; `make ui`,
  then the Go tests of `internal/server` exit 0;
  `make ui-test` with the new file and Block 2's in all four cases exit 0;
  the whole UI suite in two cases; screenshots at 1100 and 390 px;
  the owner's check on a phone, by hand.

### Block 5: Credits

- **Goal:**
  System info lists everything the binary includes, each with its licence.
- **Changes:**
  1. `tools/credits` (new):
     the modules `go list -deps` finds for `./cmd/massimo` on linux,
     windows and darwin,
     each with its version and licence files from its module directory;
     Go itself (`$GOROOT/LICENSE`);
     each directory of `third_party/` and each file of `ui/vendor/` by its
     licence file and its note; the two lists; Massimo itself,
     AGPL-3.0-or-later;
     a licence named by its text where it is one of a few known,
     else "see the text".
     It writes, or with `-check` compares, as D52 settles.
  2. `internal/credits` (new): the list and the texts, `go:embed`;
     `make credits`.
  3. `/ui/credits.json`, served from it.
  4. `src/sysinfo/`:
     a fold "Credits and licences", fetched at its first opening:
     each entry's name, version, licence and origin,
     its text in a fold of its own; texts in six languages.
- **Tests:**
  `internal/credits`: a module the binary links without an entry fails,
  as does a `third_party/` directory, a `ui/vendor/` file or a list;
  `tools/credits -check` against a changed module list; `test_system_info.py`:
  the fold fetched once, Preact's entry with its text.
  Seen failing before the change.
- **Documents:**
  `docs/ui.md` ("System info"), `README.md` (the credits in System info),
  `third_party/preact/README.md` and `ui/vendor/README.md` (an update runs
  `make credits`).
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-credits.md`:
  "- System info lists the credits and licences of everything the server
  includes."
- **Commit:**
  `feat(server): System info lists the credits and licences`
- **Verification:**
  `make ui`, then `MASSIMO_ENV_FILE=/dev/null go test ./...` exit 0;
  `go run ./tools/credits -check` exit 0; `gofmt -l .` prints nothing;
  `go vet` for three platforms exit 0; `make cross` exit 0;
  `make ui-check` exit 0;
  `make ui-test` with `test_system_info.py` in all four cases exit 0.

## Proposed commit message

The plan: `chore(aps): +PasswordRules v1.0`.

Block 1:

```
fix(security:auth): A new password has 8 to 64 characters

AP PasswordRules v1.0, Block 1: a password set with massimo user add,
passwd or rename, or by the plain password field of the users' API, has
8 to 64 code points (D4-A, D7-A); the rule lives once in
internal/password, with the cases the page's check is held to later.
A record registered by OPAQUE is not seen and not checked (D11-A), and
passwords set before stay valid, as does the version-1 upgrade.

Key changes:
- internal/password: MinLength, MaxLength, Check and its errors; testdata/cases.json
- cmd/massimo/user.go: readPassword checks the rule; the usage says it
- internal/server/users_api.go: 400 for a plain password outside the rule
- internal/server/*_test.go, cmd/massimo/user_test.go: the rule's tests; fixtures lengthened
- tools/ui-test/: the operator's password op-password
- README.md, docs/api-v1.md, docs/api-v1-openapi.yaml, src/api.d.ts: the rule
- CHANGELOG.d/Security/claude-password-length.md: the bullet

Assisted-by: claude-code:claude-opus-5-5
```

Blocks 2 to 5: the summaries of their blocks above.

## Change History

- v1.0 (2026-11-24): first revision.
