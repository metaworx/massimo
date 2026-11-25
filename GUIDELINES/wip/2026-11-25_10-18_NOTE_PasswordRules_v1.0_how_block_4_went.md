# NOTE PasswordRules v1.0: how block 4 went

Where Block 4, Suggest, went differently from the plan,
the choices taken in the run, and what verified it.

## What it made

The first new password of each of the page's five forms,
the first owner's, My profile's Set a password and Change password,
and in System settings → Users the adding of a user and a user's panel,
has a dice beside its eye, labelled "Suggest a password",
keyboard operable (D14-A, D19-A).
A press fills the form, both fields where it has two,
with five words of EFF's long list, lower case and joined by `-` (D16-A, D17-A),
or with twenty characters of the alphabet of 56 (D15-C),
and shows the suggestion as text,
also where `auth.password_reveal` has switched the eye off,
while the field holds it.
The line under the field says "Suggested; this password can be set.",
never the password.
From the first press on, a row under the line offers Words or Characters,
kept in this browser, and, while the field holds the suggestion, Copy,
through the page's copy.
The list is `internal/server/ui/words-en.txt`, EFF's words alone,
embedded and served at `/ui/words-en.txt`,
fetched once at the first press that needs it (D18-A).
`third_party/eff-wordlist/` holds its licence, its notice and its note,
and System info's credits list it as CC-BY-4.0.
The icon is Tabler's `dice-5`, whose five dots are filled circles.

- **Two commits:**
  "feat(ui): A dice beside a new password suggests one" and "fix(ui):
  Copy over plain HTTP keeps the focus where it was".

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **Built before Block 3 was read:**
   the block was built on Block 3's first tree, to save time,
   and carried onto Block 3's final commit by a three-way merge of trees;
   the two met in one place, the import lines of `src/shared/password.tsx`,
   resolved by hand.
   Two other sessions' pull requests were merged into main before it,
   an update's history and channel and a backup's watch;
   its merge onto them had no conflict,
   and no module or vendored file changed there.
2. **The line's texts, not a new status region:**
   the rule's line under the field gains three notes, "Suggested;
   this password can be set.", "Word list not loaded: try Characters."
   (a list that did not load alone) and "No suggestion could be made."
   (any other failure, and the bound), in place of a status region of its own.
   Each is one line at 390 px in six languages.
3. **The second field is filled and not shown:**
   Again takes the suggestion and stays a password field.
4. **A choice of kind suggests at once:**
   Words or Characters, chosen in the row, fills the form with one of that kind.
5. **Copy only while the field holds the suggestion,**
   and it copies the suggestion, never what was typed;
   the plan's line offered it from the first press.
6. **The row's coming and going:**
   it is not there before the first press,
   so that the first owner's card keeps its measures until the dice is used;
   once offered, a key never takes it away,
   so that nothing under the field moves while one types;
   it goes with Reset, a save or a new start.
7. **The list is held line by line,** not by its count:
   7,776 lines, each `a-z` with `-` inside, none twice,
   the file ending in a newline;
   anything else is a `ListError`, refused as a list that did not load.
8. **The licence's files:**
   `LICENSE` is Creative Commons' legal code of Attribution 4.0 International,
   byte for byte as the coordinating session fetched it on 2026-11-25,
   its size and SHA-256 in `third_party/eff-wordlist/MASSIMO.md`;
   eff.org's statements, both named (the page's words CC BY 4.0,
   the badge's link CC BY 3.0 US), the attribution, what was changed
   and both SHA-256 values are the `NOTICE`.
   The plan named one file, "the licence as eff.org states it";
   the first build had a statement of the builder's as `LICENSE`.
9. **A second commit, not in the plan:**
   "fix(ui): Copy over plain HTTP keeps the focus where it was".
   Over plain HTTP, `copyText` of `components.js` copies through a text area
   it selects and removes, which left the focus on the page's body;
   the focus now goes back to the element that had it.
   A fault older than the block,
   which the block's row puts into every password form;
   every copy of the page gains by it.
10. **The handle:**
    `MassimoParts.suggestPassword`, for the UI test's thousand draws.
11. **The credits' entry:**
    `eff_large_wordlist`, 2016-07-19, CC-BY-4.0, by `make credits`,
    `LICENSE` named by its title and `NOTICE` shown as a notice;
    32 entries now.
12. **`shared/choices.tsx` gains a look, `one`:**
    radio buttons in a row, for Words or Characters,
    kept under `massimo.suggest` through the page's `readStored` and `store`,
    which hold `localStorage` within try and catch.
13. **The CHANGELOG bullet,** in a user's words and one clause:
    "A dice beside a new password's field suggests a password of five words
    or twenty characters."
    The plan's had two, "…, and Copy copies it.".
14. **The UI suite as a whole ran in one case,** 1100-en, not the plan's two;
    the screenshots at 1100 and 390 px are of the first tree.

## Choices the plan left open

- **The bound of twenty draws:**
  a suggestion the rule refuses, one holding the user's or the server's name,
  is drawn again at most 20 times;
  then the line says "No suggestion could be made." and the field keeps what it
  had.
  The other way: no bound, or Characters taken in its place.
- **What ends a suggestion:**
  the field no longer holding the value filled,
  by a key, or emptied or changed from outside (Reset, a save),
  a submit, the eye, or a fold or the card closed;
  the field is a password's again and the eye as it was.
  The other way: the events themselves, a key, a save, a close,
  which the first build had and which let Reset leave the suggestion shown.
- **The count against late fills:**
  each field keeps a count that every press, a key, a submit,
  a new start and a change from outside raise;
  a suggestion still drawn fills only where the count and the session
  are as they were at the press and neither field has changed since.
  The other way: the dice held while the list loads.
- **Where the list is fetched:**
  the page's static file under `/ui/`, as the page's files are served,
  with no token, since the first owner's form has none;
  only a press of Words fetches it, Characters never does.
  The other way: the list inside the page's bundle, loaded with every page.
- **The words with a dash:**
  `drop-down`, `felt-tip`, `t-shirt` and `yo-yo` stay, and D17-A as decided:
  a suggestion holding one shows six or more parts,
  and a reader cannot tell the words apart by the dashes;
  the number of suggestions is unchanged, 7,776⁵ or about 64.6 bits,
  since none of `drop`, `felt`, `t` and `yo` is a word of the list.
  The other way: the four words left out, or another joint.

## The draws

A word is a 32-bit number of `crypto.getRandomValues`,
drawn again at or above `floor(2^32 / 7776) × 7776`, then taken modulo 7,776;
a character is a byte, drawn again at or above 224 (4 × 56),
then taken modulo 56.
The alphabet is the 56 letters and digits without `0 O o 1 l I`,
about 116 bits for twenty.
Nothing else gives a random number: no `Math.random`.

## Where the suggested password is, and where not

It is in the form's state, the field's value and,
at a press of Copy, the clipboard.
It is in no status line, label, title or console line,
in no storage (only the kind is kept,
under `massimo.suggest`) and in no request.
The second field holds it as a password field.

## What the reading found

One builder built it in an exported copy; two readers read it, one the draws,
the state and the files, the other the forms as a user meets them, the tests,
the texts and the documents.
The draws were unbiased, the alphabet exactly the 56,
the list `cut -f2` of EFF's byte for byte, the icon Tabler's,
and no suggestion could be set without passing the rule.

Two things had to change:

1. **The licence's text:**
   the first `LICENSE` was a statement, not a licence's text;
   the credits' classifier named it by no licence,
   since it held both titles, and the entry's name came from the note alone,
   against D30-A's "each with its licence text".
   Now the legal code and a `NOTICE`, as above.
2. **Reset did not end a suggestion:**
   in Change password and in a user's panel the emptied field stayed text,
   its eye pressed, the row in place.
   Now a suggestion ends where the field no longer holds it.

The rest of the first rework, ten points in all, each done:
a late fill no longer overwrites a later press, a typed password or a save;
Copy is offered only while the field holds the suggestion;
the list is held line by line; only a list that did not load says so,
any other failure "No suggestion could be made.";
the line is written anew at each fill,
so that a screen reader hears a second press;
three fixed waits of the test wait for what they check;
the CHANGELOG bullet is one clause;
and the second commit gives the focus back after Copy.

The second reading settled every first finding and found four more,
each done in a second rework:
a late fill still landed after the form's Reset and over a password typed into
Again; the row went and came back while the user typed,
moving the field 14 px under the caret in the first owner's card;
the CHANGELOG bullet read as one word picked of five;
and the note on the dashes said "no two draws give the same password",
where two different draws is what holds.
The coordinating session read the last rework's diff itself;
there was no third reading.

**Planted faults:**
the builder's three were caught (no redraw at the limit, the rule ignored,
a late suggestion filling the form);
the page's reader's nine were caught (`Math.random`, no redraw, no rule,
the password in the status line and in Copy's label, still shown after an edit,
two fetches, the choice not kept, Copy silent without a clipboard);
after the first rework its five more were caught (the count not raised at a key,
Reset not ending a suggestion, Copy after an edit, the line not written again,
a CRLF list taken),
and the first commit's tree failed the second's two copy tests.

## The card's height after a press

The first owner's card in the app at 390 by 900 px,
the window's room 868 px, measured by the builder and by the page's reader
in both readings:

| Language | Before | After a press |
|---|---|---|
| en | 771 px | 799 px |
| de | 839 px | 867 px |
| fr | 863 px | 890 px |
| it | 794 px | 822 px |
| nl | 817 px | 845 px |
| pl | 794 px | 822 px |

In French alone it is taller than the window, starts at its top and scrolls
(the page 922 px tall); in the browser all fit.
A card that still fits centres again, 14 px higher, the dice with it;
in French the dice moves 2 px.
`test_the_owner_card_measures` and `test_keep_open_measures` pass unchanged.

**D58, the whole suggestion on a line under the field:**
five words are wider than the field at every width,
at 390 px about three and a half words show,
in the first owner's card at 1100 px 336 px for up to 434 px of text,
so the password cannot be read whole, or written down;
Copy takes it whole.

- **A (recommended):**
  a line under the field shows the whole suggestion while it is shown;
  it costs about 40 px more after a press,
  and the app's card at 390 by 900 px would then be about en 839, de 907,
  fr 930, it 862, nl 885, pl 862 px, over the window's 868 in German,
  French and Dutch, where it starts at its top and scrolls.
- **B:**
  Copy alone, as built.

**D59, the dice moves 14 px at the first press** where the card is centred:
the row of 28 px comes with the press and the card centres again.

- **A (recommended):**
  leave it; a second tap at the dice's centre still lands on the dice.
- **B:**
  keep the card's top where it is while it is open,
  which needs a "fixed while open" rule in the card's centring tests and changes
  how the card's other states grow.

## The owner's check on a phone

The plan asks for it, by hand:

- the French card, which scrolls after a press;
- the 14 px of D59;
- the dice tapped with the keyboard open:
  the focus moves to the dice and the keyboard closes;
- a long suggestion in the field;
- Copy over plain HTTP on the LAN;
- the dice beside the eye in a narrow panel;
- whether a browser's password manager still offers to save a suggested
  password: not seen, the test browser has none.

## Seen and left

- **The line is empty for about 50 ms** at each rewrite, at most 53 ms measured,
  the same element and its cell 20 px tall; the tests wait for it; the block's,
  as it is.
- **The thousand draws show the rule and the shapes,** not uniformity,
  and the test does not claim it; the redraw tests hold the limits;
  the test's, honest as it stands.
- **One first run of `internal/server`'s tests failed once,**
  on a test not captured, in the page's reader's first reading;
  it passed three times after, and the builder's two whole runs
  and the second reading's two did not repeat it; nobody's yet.
- **The header's version and the connect page's copy** were not driven
  by hand after the second commit; their UI tests passed; the readers'.
- **No real screen reader, no phone, Chromium alone;**
  dark mode walked on the first tree only; the owner's check above.

## For the completion gate

- **What a user notices:**
  a dice beside a new password suggests one of five words or twenty characters,
  shown as text while the field holds it,
  with Words or Characters and Copy under the field;
  after a copy over plain HTTP the focus stays where it was.
- **D58 and D59,** above, the owner's.
- **The owner's check on a phone,** above.
- **The licence's text was fetched by the coordinating session,**
  Creative Commons' legal code, on 2026-11-25;
  its size and SHA-256 are in `third_party/eff-wordlist/MASSIMO.md`.

## What verified it

- **The builder, on the first tree:**
  `make ui`, `make ui-check`, `gofmt` listing no file,
  `go vet` for linux, windows and darwin, `make cross`,
  `go run ./tools/credits -check`, `go run ./tools/apitypes -check`,
  `reflow.sh --check` and `commit-msg.sh`, each exit 0;
  the whole `go test ./...`, 37 packages ok;
  the new `test_password_suggest.py` (11 tests), `TestWordList`
  and the file's check in `TestUIServed`, each seen failing on the base tree;
  the brief's eleven UI files in all four cases, 514 passed;
  the whole UI suite in 1100-en, 772 passed.
- **The reader of the draws, the state and the files, first reading:**
  `make ui` and `make ui-check`, `gofmt` and `go vet` of `internal/server`
  and `internal/credits`, their tests with `-count=1`, ok;
  `go run ./tools/credits -check`, current with 32 entries;
  the classifier run on both licence texts;
  the alphabet checked with Node; the list against EFF's original,
  the SHA-256 values and the icon's.
- **The reader of the forms, first reading:**
  `make ui` and `make ui-check`, exit 0; `internal/credits`'s tests ok;
  eight UI files in all four cases, 418 passed;
  the new test on the base, 11 of 11 failing;
  the five forms walked at 1100-en and 390-de, three of them dark;
  the card measured in six languages; nine planted faults.
- **The builder, after the first rework:**
  the merge onto Block 3 held,
  its own file and `test_password_names.py` 88 passed;
  each new test seen failing first; `make ui`, `make ui-check`,
  `make credits` twice with the same bytes, `go run ./tools/credits -check`,
  `go run ./tools/apitypes -check`, `gofmt`, `go vet` for linux,
  windows and darwin, `make cross`,
  `reflow.sh --check` on `docs/ui.md` and `MASSIMO.md`, each exit 0;
  the whole `go test -count=1 ./...` twice, 37 ok both times;
  the eleven UI files in all four cases, 550 passed;
  the files whose page copies something in all four cases, 536 passed;
  the whole UI suite in 1100-en, 781 passed.
- **Both readers, second reading,** each commit's tree alone:
  `make ui` and `make ui-check`, exit 0;
  `go test -count=1` of `internal/server`, `internal/credits`,
  `tools/credits` and `internal/password`, ok (the page's reader's twice);
  nine UI files in all four cases on the second commit's tree, 462 passed;
  the forms driven again at 1100-en and 390-de; five more planted faults.
- **The builder, after the second rework:**
  the four new tests seen failing on the previous tree; `make ui`,
  `make ui-check`, `make credits` twice with the same bytes,
  `go run ./tools/credits -check`,
  `go test -count=1` of `internal/server` and `internal/credits`
  and `commit-msg.sh` on both messages, exit 0; `gofmt` listing no file;
  `reflow.sh --check` on `docs/ui.md` and `MASSIMO.md`, 0 errors;
  `test_password_suggest.py` alone in 1100-en, 19 passed;
  eight UI files in all four cases, 454 passed, no retry;
  the first commit's tree alone built and passed `internal/server`
  and `internal/credits`.
- **The coordinating session:**
  the last rework's diff, read;
  `commit-msg.sh`, 0 errors for both; the strict changelog check,
  passed for each; `GUIDELINES/shared/tools/check`, twelve checks well;
  on the committed tree, `make ui`, `make ui-check`,
  `go run ./tools/credits -check`, `go run ./tools/apitypes -check`
  and the tests of `internal/server`, `internal/credits`,
  `internal/password`, `tools/credits` and `tools/identifiers`, exit 0.
  `test_password_suggest.py`, `test_copy.py`, `test_password_rule.py`,
  `test_password_names.py`, `test_first_owner.py` and `test_keep_open.py` in
  all four cases, 312 passed; the whole UI suite in 1100-en, 790 passed.
