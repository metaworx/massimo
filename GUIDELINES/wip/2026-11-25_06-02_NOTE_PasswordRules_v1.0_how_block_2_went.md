# NOTE PasswordRules v1.0: how block 2 went

Where Block 2, Words, went differently from the plan,
the choices taken in the run, and what verified it.

## What it made

The page's five forms that set a password,
the first owner's, My profile's Change password and Set a password,
and in System settings → Users the adding of a user and a user's panel,
refuse what Block 1 refuses, a new password outside 8 to 64 code points,
and say why under the field while it is typed (D6-A, D7-A, D11-A).
The first owner's form took "a" before.

The line under the field is its description and a `role="status"` region,
written only when what it says changes:
the hint with an i while the field is empty,
then "Too short" or "Too long" with a warning's sign,
or "This password can be set." with a tick; never a colour alone.
Its place is as tall as its tallest text, so no row under it moves.
A submit the rule refuses, also by Enter in the field, sends nothing:
the line says "Refused: too short, at least 8 characters."
(or too long), the field is marked `aria-invalid` and has the focus,
until the next key and while the field still holds a refused password.

The rule is one function,
`passwordProblem(password, context)` in `shared/password.tsx`,
which answers in the words of `cases.json` (`too_short`, `too_long`,
`""` for `ok`) and takes the user's names as its context words,
refusing nothing for them yet, as Go's `Check`.
The UI test gives it every case through `MassimoParts.passwordProblem`.
The Users forms keep one field (D51-A).

- **One commit:**
  "fix(security:ui): The page says what a new password lacks while typing".

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **The rule's refusal comes before the mismatch,** against the plan's "after
   the mismatch": with a first field too short and a second that differs,
   the plan's order said "do not match" first and "too short" only once the two
   matched, two rounds, while the line said "too short" all along.
   A first field the rule takes with a second that differs is still a mismatch,
   said as before.
2. **The hint's words:**
   "Min. 8 characters; better 15 or a few words."
   (in German "Ab 8 Zeichen; besser 15 oder einige Wörter."),
   not the plan's "At least 8 characters; 15 or a few words are better.".
   The plan's took two lines at 390 px in four languages and made the first
   owner's card in the app taller than the test's window in French;
   every text of the line now fits one line at 390 px in six languages.
   The first build's "At least 8;
   15 or a few words are better." read as eight words, in all six languages.
   The French hint reads "Min. 8 signes ; mieux 15 ou quelques mots.":
   every wording with "caractères" and "quelques mots" was too wide (below).
3. **A refusal is said in one place,** the line under the field,
   in all five forms alike, with texts of its own ("Refused: …")
   and `aria-invalid` with the field's outline, none of which the plan names.
   The form's own message line says nothing of it,
   so that nothing on the page still says "too short"
   once the line says the password can be set.
   One red, `--bad`, for the refused line, its sign and the outline.
4. **`PasswordRule`'s shape:**
   a part of its own taking the line's `id`, the `value`,
   `context` (a function read at each drawing) and `refused`,
   drawn by `PasswordField`'s `rule`,
   which is the function that gives the context words,
   and `refused`, the caller's count of the submits the rule refused.
   `newPasswordProblem` answers `{text, rule}`, not a text,
   so that the caller knows the refusal is the first field's;
   `passwordSaid` looks a refusal's words up from its answer in a typed table,
   `SAID`.
   `NewPassword` takes `context` and `refused`.
5. **The handle,** `MassimoParts.passwordProblem`,
   exists for the UI test alone,
   which the common rule "no property that exists for a test" forbids
   and the plan's step 4 asks for; the plan's step was taken.
   It is pure, and a page script can do nothing harmful with it.
6. **The fixtures,** which the plan does not name:
   `"pw"` became `"owner-pass"` in `test_first_owner.py` (its `fill_owner`
   included), `test_ways.py`, `test_sign_in_updates.py` and `test_users.py`;
   in `test_ways.py` "skipped", "back" and "escaped" grew too.
   None of them weakens an assertion;
   in `test_users.py` two checks of the name now refuse alone.
7. **The first owner's field, not controlled,**
   is followed by its keystrokes and its `change` events,
   and read at every submit, before the form's own handler,
   not by input events alone:
   a password manager's fill that fires no event is read.
8. **In Users a second submit while a save or a creation runs** does nothing,
   before the rule is asked, as the form does for any other second submit.
9. **Tests beyond the plan's:**
   the rule before the mismatch;
   the refusal by Enter in the field and by the submit;
   the first owner's fill without an event, before and after a refusal;
   Reset after a refusal in My profile and in a user's panel;
   the line staying the same element;
   the upgrade's short password;
   an empty field in Users not checked.
10. **The whole UI suite** ran in one case, on the first tree, not in two.

## Choices the plan left open

- **The line's text replaces the hint while typing.**
  The other way: a hint that stays and a status line of its own,
  a row more in each form.
- **The line keeps its place** by drawing each of its texts hidden,
  `aria-hidden`, in the same grid cell.
  The other way: a fixed height, which a language's longer text would break.
- **A refused submit's words differ from the typed ones,**
  so that the status region's text changes and a screen reader hears the refusal
  even where the line said "too short" already.
  The other way: the same words, silent at the submit.
- **The hint carries a sign, an i,** in line with the other states: without one,
  beside the empty adding form's placeholder "empty: a user with keys only",
  it read as if an empty field were refused.
  The other way: the hint without a sign.
- **"Nothing sent" is proven by counting:**
  the one creation, `PUT` or registration that follows a refusal is the only one
  sent; in a user's panel the `/register` requests are counted with the `PUT`.
  The other way: a fixed wait, which is a race.
- **The upgrade's test calls `upgradePassword` itself**
  and says in its docstring that it does not hold a rule placed in the sign-in;
  the server's side is `TestOpaqueUpgrade`'s.
  The other way: through a sign-in, where an OPAQUE sign-in started and never
  finished holds 127.0.0.1 for about a minute and blocks other tests' sign-ins.
- **The `biome-ignore` on the line's reset** gives a reason for both its uses:
  the card's openings and successes, and a fold's open state.

## Where the rule holds, and where not

- **Held:**
  the five forms, at the submit, by the button and by Enter in the field.
- **An empty field in a user's panel** leaves the password as it is,
  and **an empty field when adding a user** makes a user with keys only:
  neither is checked.
- **The sign-in card** takes a password of any length, as before.
- **The upgrade of a version-1 user at its sign-in** registers the same short
  password, and the page does not refuse it.
- **The browser's own required check** on an empty Again in the first owner's
  form comes before the rule's refusal:
  the user sees "Please fill out this field" on Again,
  while the line already says "too short".
  It is left as it is; to change it, the form would need `noValidate`.
  My profile's Again is not required, and refuses at once.

## Where JavaScript's count and Go's can differ

A lone surrogate counts one in JavaScript and cannot be held in Go,
and bytes that are not UTF-8 count one each in Go and cannot be held in a
JavaScript string; `cases.json` can hold neither.
Neither reaches the other side:
the page sends only an OPAQUE record, its encoder replacing a lone surrogate,
and Go checks only the command line's and the plain API's passwords.

## What the reading found

One builder built it in an exported copy;
two readers read it, one the parts,
the other the tests, the fixtures, the texts, the look and the documents.
A refused password is sent on no path,
the rule agrees with Go's on all sixteen cases,
and the coupling of the card's emptying with the line is exact.
Two things had to change, one in each round:

- **The form's line contradicted the field's line:**
  in the first owner's form and in My profile the refusal was written under the
  field and in the form's message line;
  after an eighth character the field's line said the password can be set
  and the form's still said "Too short", until the next submit.
  Now the line alone says it (above).
- **A field emptied by its form stayed marked invalid:**
  after Reset in My profile and in a user's panel,
  where a draft was taken away,
  where a panel's save succeeded after a refusal,
  and in the first owner's form after a fill that passes,
  `aria-invalid` and the outline stayed over the field.
  Now the mark and "Refused" hold only while the value shown is still refused.

The rest of the first round, done:

- in Users a refused submit by Enter in the field was silent;
  now the line says "Refused" there too;
- the mismatch came before the rule; now the rule first;
- the first owner's line followed input events alone; now `change` too,
  and the field is read at the submit;
- the hint read as eight words; it names its unit;
- the empty adding form's hint had no sign and was indented; now an i, in line;
- the test watched only the line's text node,
  and a status region replaced at every key passed;
  now it holds that the element stays the same one;
- two fixed waits before "nothing sent"; now the count;
- the upgrade's test did not say what it holds; its docstring does;
- the docstring of the cases' test says where the counts can differ;
- the `biome-ignore`'s reason fitted one of its two uses; now both.

And of the second:

- a user's panel's check counted only the `PUT`; now the registration too;
- the French hint said "15 ou des mots", which reads as "or words";
  now "quelques mots";
- the outline was red and the line orange; now one red;
- the commit message quotes the hint and says the order and the refusal's place.

**Planted faults:**
in the first reading, ten were caught and one was not,
the status region replaced at every key, which the second tree catches;
the builder planted four in the first owner's form after the first rework,
each caught, one only after its step blurred the field before the submit;
in the second reading, three more were caught (the rule after the mismatch,
the refusal never cleared, the form's line written as before),
and the Users forms sending despite a refusal was caught too.

## The card's room and D57

The first owner's card in the app,
in the test's window of 390 by 900 px, must stay within 868 px.
Its room left, the same before and after this block's last tree:

| Language | Room left |
|---|---|
| en | 97 px |
| de | 29 px |
| fr | 5 px |
| it | 74 px |
| nl | 51 px |
| pl | 74 px |

The line has 298 px in the card and 283 px in a user's panel.
The plan's hint would have taken en 317 px, de 346, fr 339 and nl 317.
The hint now takes en 276 px, de 276, fr 275, it 272 and nl 272,
pl 244 in the builder's measure, 243 in the reader's,
whose widest text in Polish is a refusal of 256 px;
the widest refusal is German's, 270 px.
Every wording in French with "caractères" and "quelques mots" measured 318 px
or more.

The line keeps the height of its tallest text,
so a second line in any state costs 20 px in every state.
The reader measured, in its own words, Block 3's texts:
"contains your name or the server's name" 285 px in German,
which wraps in a user's panel at 390 px;
"A common password…" 318 px in Dutch, which wraps everywhere;
and a row such as Block 4's of about 40 px breaks French and German.

- **D57, for the owner:**
  the first owner's card in the app has no height left for a further line.
  - **A (recommended).**
    The card's measure tests take a card taller than the window as right where it
    starts at its top and the page scrolls:
    the stylesheet does that already,
    and a real phone, about 750 px tall, scrolls the English card today.
  - **B.**
    A shorter text above the form.
  - **C.**
    Every later text kept to one line, as Blocks 3 and 4 are now told.

## Seen and left

- **A second Enter with no key between** announces nothing new:
  the field keeps the focus, `aria-invalid` and the words already heard;
  the owner's.
- **The browser's required check on the first owner's Again** comes first
  (above); the owner's.
- **My profile pushes its UI preferences** (`PUT /me/preferences/ui`),
  so its test counts only the users' and the sign-in routes; the test's,
  as it is.
- **The upgrade's test** does not hold a rule placed in the sign-in (above);
  the server's side is `TestOpaqueUpgrade`'s.
- **No screen reader was heard:**
  what one says at a refusal and at the next key is reasoned from the status
  region's changes; the owner's.
- **The card's room** for Blocks 3 and 4: D57, the owner's.

## For the later blocks

- **Block 3:**
  each new text of the line on one line at 283 px in six languages.
  The first owner's context words are read through a ref while drawing,
  so a line that depends on the name must be redrawn by the name field
  (`UserNameField`'s `onInput` in `Owner.tsx`).
  My profile and Users pass an empty string as a word where there is no display
  name: `passwordProblem` or its callers drop empty words.
  A new answer needs its text in the table `SAID`,
  without which the type check fails, and its refusal's text.
- **Block 4:**
  its row under the field comes with the first press, not from the start.
  A block that adds a part to a page runs the whole UI suite in one case
  before its pull request.
- **A missed fixture,** a new password under 8 characters typed into one of the
  five forms, is found in `tools/ui-test` by
  `grep -nE '(setup-password|setup-again|me-new|me-again|user-[^"]*-password|name=password\]).*\.(fill|press_sequentially|type)\("[^"]{1,7}"\)' tools/ui-test/*.py`,
  and one passed through a helper's argument by
  `grep -nE '(password|again)="[^"]{1,7}"' tools/ui-test/*.py`.

## For the completion gate

- **What a user notices:**
  a line under each new password says while it is typed whether it is too short,
  too long or can be set, and a refused submit says so there and sends nothing.
- **The order:**
  the rule's refusal comes before the mismatch, against the plan's words.
- **The hint's words** are not the plan's:
  "Min. 8 characters; better 15 or a few words.",
  and in French "signes", not "caractères", to fit one line.
- **D57:**
  the card's room, A, B or C.

## Where it met Block 5 on main

Block 5, merged before it, changed the six language files, `docs/ui.md`
and `style.css` too.
The three-way merge of the trees had no conflict,
and every key of both is in all six files.

## What verified it

- **The builder, on the first tree:**
  `make ui`, `gofmt -l` listing no file, `go vet` for linux, windows and darwin,
  `make ui-check`, `make cross`, each exit 0;
  `go test -count=1` of `internal/server`,
  `internal/password` and `tools/identifiers`, exit 0;
  `reflow.sh --check docs/ui.md`, 0 errors;
  the nine files the brief named with `test_ways.py`
  and `test_sign_in_updates.py` in all four cases, 550 passed;
  the whole UI suite in 1100-en, 751 passed.
- **Seen failing first:**
  6 of the new file's 7 tests on the tree before, each for the reason it names;
  the upgrade's test, a forward guard, with a length check placed in
  `upgradePassword` for the run and removed.
- **The builder, after the first rework:**
  the same checks and `commit-msg.sh`, each exit 0;
  the eleven files in all four cases, 550 passed;
  5 of the 7 tests failing on the tree before the rework,
  and the four planted faults caught.
- **The builder, after the second rework:**
  `make ui`, `make ui-check`, `gofmt`,
  `commit-msg.sh` and `reflow.sh --check docs/ui.md`, each exit 0;
  `test_password_rule.py`, `test_first_owner.py`, `test_keep_open.py`,
  `test_users.py`, `test_profile_help.py`,
  `test_login.py` and `test_parts_guard.py` in all four cases, 382 passed;
  each new test seen failing on the tree before.
- **The reader of the parts:**
  `make ui`, `tsc --noEmit` and Biome's lint, exit 0, on both trees.
- **The reader of the tests and the look:**
  the planted faults in 1100-en; the shots of each form at 1100 px in English
  and 390 px in German, light and dark;
  on the second tree `test_password_rule.py`, `test_first_owner.py`,
  `test_keep_open.py`, `test_users.py` and `test_parts_guard.py`,
  with its own probes, in all four cases, 342 passed.
- **The coordinating session:**
  the message by `commit-msg.sh`, 0 errors; the strict changelog check, passed;
  `GUIDELINES/shared/tools/check`, twelve checks well; on the committed tree,
  on main with Block 5, `make ui`, `make ui-check`,
  `go run ./tools/credits -check` and the tests of `internal/server`,
  `internal/credits` and `tools/identifiers`, exit 0.
  `test_password_rule.py`, `test_system_info.py`, `test_parts_guard.py`,
  `test_restart.py`,
  `test_first_owner.py` and `test_keep_open.py` in all four cases, 322 passed;
  the whole UI suite in 1100-en, 755 passed.
