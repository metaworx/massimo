# NOTE AccessEnds v1.2: how block 10 went

Where Block 10, Alternate, went differently from the plan,
the choices taken in the run, what the reading changed,
the dice's move measured again for D53-A, and what verified it.

## What it made

- **The dice gives five words and twenty characters by turns**
  (D44-A, D50-A, D51-A):
  five words at the first press and again at each new start (Reset, a save,
  the fold closed, the card's next opening), then the other kind at each press,
  whatever the last one filled; a key leaves the turn as it is.
  Its label, for a screen reader and as its tooltip,
  says what the next press gives:
  "Suggest five words" or "Suggest twenty characters", in six languages.
- **The row under the field is gone**
  (D49-A): Words or Characters with the kind this browser kept under
  `massimo.suggest`, read no more and a value kept there left where it is,
  and the row's Copy.
- **The shown suggestion copies itself,**
  in the owner's words at the gate of 2026-11-26,
  "like other strings":
  a tap, a click, Enter or Space copies it,
  "Copied" showing above it and said in the page's status region,
  its copy icon after it, both named "Copy the password".
  Each suggestion has a Copy of its own,
  so that "Copied", said of the last one, never shows over the next.
- **The line is no longer `aria-hidden`**
  (D52-A): the shared `Copy` gains `quiet`,
  its words hidden from a screen reader inside it, given only with a name,
  which its type enforces; `translate="no"` stays.
- **A first press adds the line's place alone,** 52.4 px where it added 80.
- **One commit:**
  "feat(ui): The dice alternates words and characters, and the shown suggestion
  copies on a tap", on `maint/0.17` (Different from the plan, 1).

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **Into 0.17:**
   since the version freeze of 2026-11-26 the block goes into 0.17.
   The coordinating session carried its step from the plan's v1.2 commit,
   which `maint/0.17` holds,
   onto that branch's head, "chore(aps): +NOTE StartConfiguration v1.3,
   how the run went", by a three-way merge of trees without conflict;
   the change is the step's, line for line.
   The pull request's base is `maint/0.17`; once it is merged,
   the coordinating session merges `maint/0.17` upward into `main`.
2. **`password.noList` says what the next press gives:**
   its text named Characters, which is gone.
   Now, in English, "No word list: press again for characters.",
   and alike in the other five.
   The first German text made the rule's line two lines in Add a user at 390 px;
   the builder measured candidates and took "Keine Wortliste:
   nochmals für Zeichen."; the line is one line in all six.
3. **`Choices` loses its look `one`** (`choices.tsx`, `docs/ui.md`):
   the row was its only user.
4. **`Copy`'s properties are a type, `CopyProps`,**
   in which `quiet` comes only with `name`,
   so that `make ui-check` refuses a quiet copy named after its own text,
   which would tell the password;
   the plan had the property alone.
   From the reading, as is the next.
5. **The line's Copy is keyed by the suggestion**
   (What the readings found).
6. **Tests the plan did not name:**
   - `test_a_name_that_is_a_word_is_drawn_again`,
     whose second press now gives characters,
     presses characters in between and asks the third press for words;
   - `reset_ends` presses until the dice names characters before Reset,
     so that five words after Reset prove the turn's reset;
   - `test_the_row_stays_while_one_types` is renamed
     `test_the_shown_line_stays_while_one_types`;
   - from the rework, `test_the_dice_names_its_next_kind_in_every_language`
     (1100-en; French, Italian, Dutch and Polish as the browser's language,
     against pinned strings,
     `DICE_SAYS`) and `test_the_rule_line_is_one_line_in_every_language`
     (390-en; Add a user in six languages, the word list refused);
     `test_copy` presses Space too, and copies before a press of the dice;
     `test_my_profiles_set_a_password` holds the label after a save,
     and `test_shown_with_the_eye_off` that the press after a key gives the kind
     the label names.
7. **`test_first_owner.py`:**
   the plan left Block 4's measure tests as they are;
   only the docstring of `test_the_owner_card_measures` changed,
   naming the shown line and the languages whose card is taller than its room.

## Choices the plan left open

- **The next kind is kept twice:**
  in a ref, which the press reads, and in a state, which the label reads,
  so that two presses before a redraw still alternate.
  The other way: a state alone.
- **Where the icon sits:**
  the copy pair across the line's width, the words breaking inside it;
  the icon right after the words while they take one line,
  and at the line's right end, between its lines, once they wrap.
  The other way: the icon at the line's end always.
- **The words in the body's colour.**
  The other way: the accent colour the row's Copy had.
- **The Copy keyed by the suggestion,**
  the press having given the focus to the dice,
  so that nothing the user holds is replaced.
  The other way: the mark ended when the text changes.
- **The other languages' labels pinned in the test,**
  not read from the i18n files: a planted wrong translation passed that way.
  The other way: the files' own words.

## The copy beside the page's other strings

The builder compared it with Keep it open…'s address in the same card,
the page's reader with System info's version, by a click,
Enter and Space at 390 and 1100 px.
Each is a span of `role="button"` and `tabindex` 0, class `copyable`,
the copy cursor, named by its `aria-label` and `title`.
A press gives the class `copied`,
"Copied" above it for about 1.4 s and in `#copy-status`,
the focus staying and the page not scrolling;
over plain HTTP the fallback copies too, on Space as well;
the icon behaves alike; the clipboard held exactly the suggestion.
The differences are the plan's:
the body's font at 16 px, not monospaced, and the name "Copy the password".

## What a screen reader hears

- **At a press:**
  the rule's line, written anew, "Suggested; this password can be set.".
  The dice keeps the focus, named now for the press after:
  a screen reader that speaks a name change of the focused control may say
  "Suggest twenty characters" right after five words came.
  D44-A as decided, and `docs/ui.md` says it;
  no real screen reader was tried (For the completion gate, 2).
- **The password is told nowhere:**
  the line is a paragraph holding two buttons named "Copy the password",
  the words `aria-hidden` inside the first;
  no node of Chromium's accessibility tree but the field holds the password,
  after words and after characters, nor any label, title or live region's
  announcement in the five forms;
  the sign-out's "not saved" dialog does not hold it.
- **Planted at run time by the builder,** each caught by the suite's
  `not_spoken`, which passes again once they are taken out:
  the words not hidden (in My profile's live region,
  and in the first owner's card, which has none, on Chromium's tree);
  the copy named after the password (its label); its title set to the password.
- **D52-B would have told it:**
  Chromium keeps a `role="button"`'s text in its accessibility tree,
  where the plan's D52-B said it leaves it out.

## The dice's label before each press

Read in the page at 390 and 1100 px by the builder and the page's reader:

| Language | Before presses 1 and 3 | Before presses 2 and 4 |
|----------|------------------------|------------------------|
| en | Suggest five words | Suggest twenty characters |
| de | Fünf Wörter vorschlagen | Zwanzig Zeichen vorschlagen |
| fr | Suggérer cinq mots | Suggérer vingt caractères |
| it | Suggerisci cinque parole | Suggerisci venti caratteri |
| nl | Vijf woorden voorstellen | Twintig tekens voorstellen |
| pl | Zaproponuj pięć słów | Zaproponuj dwadzieścia znaków |

With the word list failing, the presses said there was none,
then gave characters, then said there was none, the label following (D51-A);
after a key it stays (D50-A).
A change of the page's language does not restart the turn.

## The dice's move at the first press (D53-A)

How far the dice's centre moves up at the first press, in px,
measured as Block 4 measured it, by the builder on the plan's commit (before)
and on the block's tree (with the block), and again by the page's reader:

| The dice | en | de | fr | it | nl | pl |
|----------|----|----|----|----|----|----|
| first owner's card, app at 390 by 900 px, before | **40.0** | 14.3 | 2.7 | **36.9** | **25.3** | **36.9** |
| the same, with the block | **26.2** | 14.3 | 2.7 | **26.2** | **25.3** | **26.2** |
| first owner's card, browser at 390 px, before | **40.0** | **40.0** | **35.9** | **40.0** | **40.0** | **40.0** |
| the same, with the block | **26.2** | **26.2** | **26.2** | **26.2** | **26.2** | **26.2** |
| first owner's card, browser at 1100 px, before | **40.0** | **40.0** | **40.0** | **40.0** | **40.0** | **40.0** |
| the same, with the block | **26.2** | **26.2** | **26.2** | **26.2** | **26.2** | **26.2** |
| a user's panel, Add a user, My profile, before and with the block | 0 | 0 | 0 | 0 | 0 | 0 |

In bold, where a second tap at the first tap's place misses the dice,
33.2 px tall: with the block as before,
it lands only in the app in German and French,
where the card is taller than its room and starts at its top.
A second press moves the dice 0 everywhere.
In Dutch the card, 1.8 px over its room with the test machine's fonts,
starts at its top too, and the dice moves 25.3 px as before;
the plan's Analysis reckoned it among the cards that stay centred.

The first owner's card after a press, app at 390 by 900 px, the room 868 px:

| The card | en | de | fr | it | nl | pl |
|----------|----|----|----|----|----|----|
| before the block | 851 | 920 | 943 | 874 | 898 | 874 |
| with the block | 823.4 | 891.8 | 914.9 | 846.6 | 869.8 | 846.6 |

In a browser at 390 px every language fits, French at 848.6 px.
PasswordRules.D59-A stands, as the owner kept it at the gate of 2026-11-26.

## What the readings found

One builder built it in an exported copy of the plan's v1.2 commit;
two readers read the recorded tree,
one the page's code and the look, the other the tests and the documents.

Confirmed: the copy behaves as the page's other strings do;
no node, label or title holds the password;
the labels alternate in six languages;
every rule text is one line in six languages at 390 and 1100 px,
the app's window too;
the dice's move holds to the builder's numbers;
every changed test can fail, none waits on a fixed sleep,
and no assertion was weakened.

One must, done:

- **"Copied" stayed over a new suggestion:**
  the line's Copy was the same element from one fill to the next,
  and the page's `copied` keeps its mark for 1.4 s;
  a copy of the line or its icon, then a press of the dice within that time,
  showed "Copied" above the new suggestion while the clipboard held the old one,
  so that a user might keep a password other than the one shown.
  Now the Copy is keyed by the suggestion;
  `test_copy` holds it, seen failing on the first tree in 390-en and 1100-en.

Shoulds, done:

- **`quiet` without `name`** named the element "Copy {its text}":
  now the type refuses it (Different from the plan, 4).
- **What the documents said and no test held:**
  a save starting the turn anew, the labels in French, Italian,
  Dutch and Polish, Space on the line's copy,
  the line of a word list not had on one line at 390 px in six languages,
  the press after a key giving the kind the label names:
  each now a test (Different from the plan, 6).
- **The test's `clear`** left the class `copied`,
  so that a click just before could meet the assertion after Enter:
  it removes it now.
- **Words:**
  `docs/ui.md` says Dutch overflows "with the test machine's fonts (CI's
  differ)"; `test_first_owner.py`'s docstring names German, French and Dutch,
  rewrapped under 80 characters;
  `docs/ui.md` and the comment in `style.css` say where the icon is drawn,
  which the builder's first report had wrong for a line that wraps.

The page's reader's other point,
what a screen reader may hear at a press, is for the completion gate.
Left out of a test: the "Copied" bubble's own drawing,
which no copy test of the page checks;
the tests' reader saw it say "Copied" and "Kopiert", not clipped.
The coordinating session read the rework's diff of `password.tsx` and
`copy.tsx` and found nothing to change; no reader read the rework separately.

## The slip in the brief

The builder's brief gave the fake Cube's UI test port as 65600,
above 65535, which the server refused: the coordinating session's slip.
The builder used 61600 to 61603, the API and discovery ports as given,
in the first run and in the rework's.

## Seen and left

- **A third line at 390 px is likelier:**
  the icon and its gap take 30 px of the line,
  so five words take a third line in about one draw in 16 million in Add a user,
  one in 70 million in a user's panel,
  where Block 4 measured about 3 in 10^14;
  the place grows by that line and the first key gives it back,
  and `docs/ui.md` gives the numbers per form.
  D19-A's, as decided.
- **A value kept under `massimo.suggest`** stays in the browsers that kept one,
  read no more: the plan's, as decided.
- **The Polish line of a word list not had,**
  "naciśnij znów po znaki", is colloquial,
  acceptable for a marked machine translation: the next change of the texts'.
- **"Copied" covers the field's lower edge for 1.4 s,**
  as every copy of the page does: the page's copy, as it is.
- **The builder's first run of `internal/server`'s Go tests** failed
  `TestHeldReferenceLifetime`;
  it passed five times alone on the base and the tree,
  and the package passed on the rerun: not the block's; nobody's yet.
- **The builder's first whole UI run** failed
  `test_sign_in_resend.py`'s `test_stream_ticket_waits_for_the_sign_in`,
  which passed on its retry: not the block's; AP SteadyTests names it.
- **My profile's data coming in** while a suggestion is held is not confirmed:
  in the page's reader's probe the load did not bring the new display name
  within 8 s; a user's panel kept the copy, the focus, the value and the label.
  The next reading of the field's.
- **No real phone, screen reader or page translation;**
  right-to-left not looked at: the owner's check on a phone.

## For the completion gate

1. **The dice's move, measured again (D53-A):**
   26.2 px where it was 40;
   a second tap at the first tap's place still misses the dice in the app in
   English, Italian, Dutch and Polish and in a browser in every language.
   The owner kept PasswordRules.D59-A at the gate of 2026-11-26.
2. **What a screen reader may hear at a press:**
   the dice keeps the focus and its new name,
   "Suggest twenty characters", right after five words came,
   then the rule's line; nothing names the kind just given.
   D44-A as decided; the owner's check on a phone, with its screen reader.
3. **The first owner's card after a press is taller than its room**
   in the app at 390 by 900 px in German, French and,
   with the test machine's fonts, Dutch (by 1.8 px):
   it starts at its top and the page scrolls; in a browser every language fits.
4. **The builder's brief gave a fake Cube port above 65535,**
   the coordinating session's slip; 61600 used (The slip in the brief).
5. **The owner's check on a phone,** as the plan asks:
   the dice by turns, a tap on the line, the French card.
6. **What a user notices:**
   the dice gives five words, then twenty characters, by turns,
   its tooltip naming the next;
   Words or Characters and Copy under the field are gone;
   a tap on the suggestion under the field copies it.

## What verified it

- **The builder, on the first tree:**
  `make ui`, `make ui-check`,
  `gofmt -l .` printing nothing and `go vet` for linux, windows and darwin,
  each exit 0; `go test -count=1 ./internal/server/...`,
  exit 0 on the rerun (Seen and left); `reflow.sh --check docs/ui.md`, 0 errors;
  in a scratch clone `changelog.sh check --staged --strict`, 0,
  and `GUIDELINES/shared/tools/check`, twelve checks all well;
  the plan's nine UI files in all four cases, 520 passed;
  the whole UI suite in 1100-en, 868 passed and 1 failed,
  which passed on its retry (Seen and left).
  Seen failing: the new `test_password_suggest.py` on the base page in 390-en
  and 1100-en, 29 failed, each at a new assertion, and 12 passed,
  whose substance did not change;
  the three planted faults of What a screen reader hears.
  The dice's move and the card's height on the plan's commit and the tree,
  in six languages.
  `make cross` and `apitypes` not run: no Go code changed.
  No script in `packaging/`, `tools/`, `app/` or `.github/` presses the dice
  or reads the row.
- **The page's reader:**
  `make ui` and `make ui-check`, exit 0;
  `go test -run 'TestUI|TestShared|TestState' ./internal/server/`, ok;
  `test_password_suggest.py` and `test_copy.py` in all four cases, 90 passed;
  probes of its own in 390-en and 1100-en, whose first run's two failures were
  the probe's, passing on the rerun;
  the copy against System info's version; Chromium's tree read over CDP;
  the labels in six languages; how a suggestion ends;
  the look in light and dark in English, French and Polish at 390 and 1100 px
  and in the app's window; the dice's move and the heights again.
- **The tests' reader:**
  the three changed test files in all four cases, 202 passed;
  the new test file on the base with the label check turned off,
  in 1100-en and 390-de, 27 failed and 16 passed,
  each failure at a behaviour of the block;
  scratch tests of what the documents say, in 390-en and 1100-de, passed;
  `go test -run 'TestUI|TestShared|TestState' ./internal/server/`, ok;
  `reflow.sh --check`, 0 errors; `commit-msg.sh`, 0; the CHANGELOG fragment,
  the plan's text, three lines of at most 71 characters.
- **The builder, after the rework:**
  `make ui`, `make ui-check`, `gofmt -l .` printing nothing,
  `reflow.sh --check docs/ui.md` (0 errors) and `commit-msg.sh`, each exit 0;
  `go test -run 'TestUI|TestShared|TestState' ./internal/server`, ok;
  the plan's nine UI files in all four cases, 522 passed;
  the whole UI suite in 1100-en, 870 passed, 0 failed.
  Seen failing: the "Copied" test on the first tree in 390-en and 1100-en;
  the new and changed tests each on a planted fault (a save not restarting the
  turn, a key turning the press but not the label,
  Space taken out of the copy's keys, the first German text,
  a wrong French label); a quiet copy without a name, `make ui-check` exit 2.
- **The coordinating session:**
  the rework's diff read;
  the step carried onto `maint/0.17`'s head without conflict;
  in a clone with the commit `commit-msg.sh`, 0 errors and 0 warnings,
  `changelog.sh check --staged --strict`, 0,
  and `GUIDELINES/shared/tools/check` over the range, twelve checks all well.
- **Not run here:**
  any UI test on the carried tree;
  the whole `go test ./...` and the race runs, which are the pull request's CI;
  a real phone, a screen reader, a page's translation.
