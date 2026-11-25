# NOTE AccessEnds v1.0: how block 4 went

Where Block 4, Tall, went differently from the plan,
the choices taken in the run, what the readings changed,
the dice's move measured for PasswordRules.D59, and what verified it.

## What it made

- **The first owner's card is measured where the page places it**
  (PasswordRules.D57-A, D16-A to D18-A):
  `assert_login_card_placed` of `tools/ui-test/helpers.py` takes a card within
  the room the window leaves it, the window less the body's padding
  (868 of the suite's 900 px), as centred, as before;
  a taller one has its top at the body's padding at the page's start and its
  bottom inside the window once the wheel has scrolled the page to its end.
  Either way the card shows all it holds, each control of it shown lies inside
  the card's box and in view at some scroll, and there is no horizontal scroll.
  `test_the_owner_card_measures`, `test_keep_open_measures` and
  `test_the_owner_card_on_a_short_screen` take it,
  the first two before a press of the dice and after one,
  at 390 px in six languages.
  The sign-in card's tests keep the strict helper,
  which now measures "fits" against the room and checks the card and its
  controls the same way.
- **A suggested password shows whole under its field**
  (PasswordRules.D58-A, D19-A to D22-A):
  while the field holds the suggestion, a line right under it,
  before the rule's line, shows it in the body's font at 16 px,
  broken after a "-", `aria-hidden="true"` and `translate="no"`.
  Its place is two lines tall from the first press until the row of Words or
  Characters and Copy goes, empty while the suggestion is not shown.
- **A dialog that closes ends the suggestion it holds:**
  the sign-in card dismissed keeps its fields' values, but no line with the
  password and no field shown as text.
- **The dice's move at the first press is measured**
  (D23), below.
- **Two commits:**
  "test(ui-test): The first owner's card taller than the window is measured at
  its top" and "feat(ui): A suggested password shows whole under its field".

## The dice's move at the first press (D23)

How far the dice's centre moves up at the first press, in px,
measured by the builder on the plan's commit (before) and on the block's first
tree (with the block), and again by the page's reader, to 0.1 px,
in light and dark alike;
neither rework moved the dice, and it was not measured a third time:

| The dice | en | de | fr | it | nl | pl |
|----------|----|----|----|----|----|----|
| first owner's card, app at 390 by 900 px, before | 13.8 | 13.8 | 2.7 | 13.8 | 13.8 | 13.8 |
| the same, with the block | **40.0** | 14.3 | 2.7 | **36.9** | **25.3** | **36.9** |
| first owner's card, browser at 390 px, before | 13.8 | 13.8 | 13.8 | 13.8 | 13.8 | 13.8 |
| the same, with the block | **40.0** | **40.0** | **35.9** | **40.0** | **40.0** | **40.0** |
| first owner's card, browser at 1100 px, before | 13.8 | 13.8 | 13.8 | 13.8 | 13.8 | 13.8 |
| the same, with the block | **40.0** | **40.0** | **40.0** | **40.0** | **40.0** | **40.0** |
| a user's panel, Add a user, My profile, before and with the block | 0 | 0 | 0 | 0 | 0 | 0 |

In bold, where a second tap at the first tap's place no longer lands on the
dice.
The dice is 33.2 px tall, not the 36 px the plan's text of D59 gives,
so a tap at its centre still lands while the move is under 16.6 px;
the builder checked each case with `elementFromPoint`,
and the page's reader again: before the block every second tap lands; with it,
only in the app in German and French,
and in every form but the first owner's card.

A card that still fits centres again by half of what the press adds:
28 px before the block, 80 px with it (the line's place 52 px, the row 28 px).
A card the press makes taller than its room starts at its top,
and moves by its top before the press less 16 px.
The other forms are not centred and do not move.

The first owner's card after a press, app at 390 by 900 px, the room 868 px:

| The card | en | de | fr | it | nl | pl |
|----------|----|----|----|----|----|----|
| before the block | 799 | 867 | 890 | 822 | 845 | 822 |
| with the block | 851.1 | 919.5 | 942.6 | 874.3 | 897.5 | 874.3 |

With the block it is taller than its room in every language but English,
and taller than the window in German and French alone;
in a browser at 390 px only the French card is taller than its room,
876.3 px.

## What a screen reader hears

At a press, the polite live regions of My profile and Users tell what they told
before the block:
the rule's line, which turns from "Min. 8 characters; better 15 or a few words."
to "Suggested; this password can be set.",
and Words, Characters and Copy, which come.
The password is in nothing told, nor in the suite's `SPOKEN`;
Chromium's accessibility tree holds it on the field and its own text alone,
on the trees before and after the block.
The raw text of those regions holds it,
inside the `aria-hidden` line (D22-A as decided).

## Different from the plan

Against the plan's block, item by item:

1. **`helpers.py`, `assert_login_card_placed`:**
   the plan's "every control of the card in view at one of the two" became each
   control shown, a `[role=button]` among them (Copy, `#keep-open-origin`),
   inside the card's box and in view at some scroll,
   between the page's start and the end the wheel reaches;
   and in both branches the card shows all it holds,
   its content no taller than its box,
   so that a card that cuts or scrolls what overflows it fails (What the reading
   found).
   The page's end is reached by the wheel (Choices).
2. **`assert_login_card_centred`** measures "fits" against the room,
   as the plan has it,
   and checks the card's content and its controls as the new helper does.
3. **The measure tests** as the plan has them;
   `test_keep_open_measures` also checks `wrapped` in the default language,
   and, in the second commit,
   that the confirmation after a press has the form's height (`same_box`),
   which the plan's Analysis states and nothing asserted.
4. **`password.tsx`:**
   the line as the plan has it, but `translate={false}`, not the string "no":
   Preact sets the property, and the string would turn translation on;
   the vendored JSX type takes only a boolean.
   Measured in the page: the attribute "no", the property false.
   Beyond the plan, the suggestion ends as the dialog the field is in closes
   (the coordinating session's choice after the first reading):
   the line empties, the field is a password's again and keeps its value,
   and a suggestion still drawn fills nothing.
5. **`style.css`:**
   `.pw-shown` as the plan has it, `flex-basis: 100%`, 16 px,
   a `min-height` of two lines at a line height of 1.45,
   `overflow-wrap: anywhere`;
   its comment says the three-line exception (8).
6. **No new text,** as the plan has it.
7. **The first commit's tests:**
   the French card after a press in the app at 390 px (890 px on that tree)
   fails main's strict helper on the centring and the new strict one on its
   height, and passes the new helper, as the plan has it.
   The plan's planted fault,
   the body cut to the window's height with its overflow hidden,
   fails the new helper on the card's top (4.9 px instead of 16),
   not on its reach: the card stays centred in the cut body.
   With the card also placed from the top it fails on the reach,
   its bottom at 906 px, which the wheel cannot scroll to.
8. **"Five words never need three lines" does not hold** (the plan's Analysis;
   D19-A's "52.4 px always"): words can repeat,
   and the line breaks only after a "-",
   so the list's widest word five times takes three lines at 390 px in Add a
   user (298 px), a user's panel (302 px) and the first owner's card (317 px),
   two at My profile's 328 px and wider.
   With the test machine's fonts, the chance computed from the words' widths
   is 3e-14 at 298 px, 4e-15 at 302 px, 1e-19 at 317 px and 0 from 328 px;
   in 1,500 laid-out draws per form and 200,000 simulated, none took three.
   D19-A is kept as built: the place grows by a line for those draws alone,
   and the first key gives it back,
   moving what is under it (For the completion gate, 3).
9. **"Twenty characters always fit one line" does not hold either** (the
   Analysis; D21-A's words): twenty `m` are 299.2 px,
   wider than the line's 298 px in Add a user at 390 px,
   and take two lines there, seen in the page; the chance is about 3e-29 there,
   and 0 from 302 px.
   `docs/ui.md` gives both as less than one in 10^13 with the test machine's
   fonts, CI's differing.
10. **`check_suggest`'s "at most two lines,
    not wider than the field"** became `shown_line`:
    the line as wide as its row, its computed font 16 px,
    broken only after a "-",
    and no more lines than the widest suggestion of its kind takes at that
    width, measured in a hidden copy of the line: three at most,
    two wherever three of the widest words fit a line.
11. **The new widest test,** `test_the_widest_suggestion_in_a_users_panel`,
    draws the list's widest word five times, the true worst case,
    not the five widest words, which take two lines everywhere;
    it asserts the count the words' widths give in its browser
    (three at 390 px with the test machine's fonts),
    in 390-en and 390-de alone.
12. **`SPOKEN`** leaves out text under `aria-hidden`, as the plan has it,
    and `not_spoken` also reads Chromium's accessibility tree:
    no node but the field and its own text may hold the password,
    since hidden text that an `aria-describedby` points to is still read out.
13. **Tests beyond the plan's:**
    the line emptied at a submit that keeps the value (Again changed),
    its place kept; gone after My profile's save and after a user's panel is
    closed and opened; ended at the card's close
    (`test_the_shown_line_ends_as_the_card_closes`); no line under Again.
14. **"Taller than the window in five languages"** (the plan's tests,
    and D58-A's text):
    taller than the room the window leaves it in five,
    than the window in two (The dice's move).
    `docs/ui.md`, both messages' bodies, the README and the helpers say
    "taller than the room the window leaves it (868 of 900 px)";
    the first commit's summary keeps the plan's words.
15. **The dice is 33.2 px tall,** not the 36 px of the plan's text of D59.
16. **The documents:**
    `docs/ui.md` as the plan has it,
    and `tools/ui-test/README.md` in the first commit,
    what the two measure tests hold.
17. **The CHANGELOG and the commits** as the plan has them.
18. **The verification:**
    the plan's ten UI files in all four cases and the whole UI suite in 1100-en
    ran on the first rework's tree, not on the last;
    on the last tree five files in all four cases, and four on the tree carried
    onto main (What verified it);
    the first commit's last tree alone ran its two measure files in 390-en,
    the four cases on the first rework's;
    the screenshots at 1100 and 390 px are of the first trees;
    the owner's check on a phone was not made.

## Choices the plan left open

- **The page's end reached by the wheel,** over the card's own padding:
  a script's scroll still moves a page whose body hides its overflow,
  so a check of the reach would pass a broken page.
  The other way: `scrollTo`.
- **Shared helpers:**
  `LOGIN_CARD`, `LOGIN_CARD_LOOK`, `controls_reached` and `centred_across`
  serve both card helpers;
  `each_language` and `dice_pressed` live in `test_first_owner.py`,
  and `test_keep_open.py` takes them from there.
  The other way: a copy in each file.
- **The eye on a shown suggestion empties the line,**
  as it turns the field into a password's again.
  The other way: the line kept while the field still holds the suggestion.
- **The widest test in 390-en and 390-de alone,**
  the only width where a third line can come.
  The other way: every case.
- **The line's bound measured,** a hidden copy of the line with the widest
  suggestion of its kind, so that CI's fonts get their own count.
  The other way: a fixed width of 328 px.
- **The dialog's close watched in the shared part,**
  a `MutationObserver` on the open attribute of the field's closest `<dialog>`,
  only where the field has a dice, and disconnected with the field;
  generic, as `PasswordInput` already watches its `closest("details")`.
  The other way: a property from `Owner.tsx`, Block 3's file.
- **The keep-open confirmation compared from the page's start,**
  since the click on Keep it open… scrolls it into view.

## What the reading found

One builder built the block in an exported copy of main with the plan.
Two readers read the first trees:
one the page's code, the five forms, the look and the dice's move;
the other the tests of both commits, the documents and the messages.

Confirmed:
the line shows and ends where the plan says in the five forms,
and a second press moves nothing (0.0 px in all 30 cases of the owner's card);
data coming in, another client renaming a user while that user's panel held a
suggestion and the focus, leaves the line, the value, Copy and the focus;
a sign-out with three suggestions open leaves no line, no password in the page
and nothing in storage;
the password is in no address, title, storage, `aria-*` reference or console;
the look holds in six languages, light and dark,
at 390 and 1100 px and in the app's window,
the line 16 px in the body's font, its contrast 17.1 light and 13.8 dark,
no horizontal scroll;
the dice's numbers, the heights and `docs/ui.md`'s 80 px and 40 px hold;
the planted faults fail the new helper;
no test sleeps, and none is weakened but `SPOKEN` for hidden text.

One must, done: **"taller than the window" was not what was measured**
(Different from the plan, 14): Italian,
Dutch and Polish fit the window and only pass its room;
the README said the sign-in card's tests keep it no taller than the window,
where they hold the room.

The shoulds, done in the first rework:

- **A cut card that fits passed:**
  capped at the room with its overflow hidden or scrolled,
  its bottom row cut, the French card passed every helper;
  the controls are checked in both branches now.
  Their pairing by `zip` failed a correct card taller than twice the window;
  it is gone.
- **`SPOKEN` and hidden text that a reference points to**
  (Different from the plan, 12).
- **`shown_line` held too little:**
  any one to three lines in every case, no width,
  no font (Different from the plan, 10).
- **The widest test** said three lines and asserted two or more (Different from
  the plan, 11).
- **Twenty characters**
  (Different from the plan, 9).
- **The place's words:**
  "empty while the field holds anything else" is "empty while the suggestion
  is not shown", which holds after the eye and at a submit too.
- **The line in a card that closes:**
  after Skip for now the hidden card still held the suggestion as page text
  until its next opening, where `docs/ui.md` said it went at the close.
  The coordinating session chose to end it at the close (Different from the plan, 4).
- **The three-line draw moves what is under it** at the first key,
  11.6 px in a centred card and 23.2 px in a card at its top,
  against "nothing under the field moves"; D19-A is kept, and `docs/ui.md`,
  `password.tsx` and `style.css` say the exception.
- **What no test held**
  (Different from the plan, 13, and the keep-open confirmation, 3).
- **Docstrings:**
  `check_suggest`, `edited` and `reset_ends` name the shown line and the rule's
  line.

The page's reader read the rework.
The dialog's close is sound:
Skip, Escape and Back on a card the user opened and a script's
`dialog.close()` each leave the line empty, no password in the page's text,
the field's value kept and its type a password's;
nothing ends early while the card stays open (a change of language,
Keep it open… and Back, a turn to the sign-in);
it costs one observer in the page, the owner's field's,
and the shared part names no region (`TestSharedPartsKnowNoRegion` passes).
The accessibility check and `shown_line` catch its planted faults.
Three shoulds, two done in a second rework:

- **A card cut below its controls still passed:**
  the English card (851 px) capped at 800 px with its overflow hidden lost its
  version line and its padding, and both helpers passed;
  Copy was not among the controls.
  Both helpers now assert that the card shows all it holds,
  and `[role=button]` is among the controls.
- **The short screen's docstring** said "in view at one of the two";
  it says "at some scroll" now.
- **A turn to the sign-in leaves the suggestion shown,**
  recorded, not changed (For the completion gate, 4).

No reader read the second rework, and there was no third reading.

**Planted faults,** each caught at the assertion it names:
the builder's two of the plan's (Different from the plan, 7);
the second reader's body cut and centred, the card from the top,
`html { overflow: hidden }` and a control moved out of the card;
after the first rework the builder's caps at the room on the second tree and at
800 px on the first, a planted `aria-describedby` to the line, a 13.6 px line,
a line 200 px wide, `word-break: break-all`,
and the keep-open steps of different heights;
the page's reader's `aria-describedby`,
`aria-hidden` removed (which `SPOKEN` alone misses,
the owner's card being in no live region), a 0.85rem line and a 250 px one;
after the second,
the builder's caps at 868 px hidden or scrolled and at 800 px on the French
card, on both trees, and at 800 px on the English card of the last tree.
A correct card taller than twice a window of 420 px passes the new helper and
fails the old; caps that cut nothing pass.

## Seen and left

- **"Taller than the window" is still said** where the signed-out page's rule
  is described, in `docs/ui.md` and in the comment of `style.css`,
  from before the block; the room is meant.
  The next change of those lines'.
- **A one-line suggestion leaves an empty line before "Suggested",**
  most draws at 1100 px;
  and at 1100 px the fields of Users, Add a user and My profile (466 to
  496 px) show five words whole, so the line repeats them.
  Costs of D19-A and D58-A as decided, not faults; the owner's.
- **The field itself is inside the polite live regions** of My profile
  and Users, shown as text while it holds the suggestion,
  on the tree before the block too.
  Older than the block; nobody's yet.
- **One observer stands in every page,**
  the first owner's field's, mounted in the sign-in card from the start,
  signed in or not; the other forms find no dialog and observe nothing.
  The block's, as it is.
- **The chances in `docs/ui.md` are the test machine's fonts';**
  `shown_line` and the widest test take their counts from the widths in the
  browser they run in.
  As it is.
- **The first commit's summary says "window",** as the plan words it;
  its body says the room.
  As it is.

## For the completion gate

1. **What a user notices:**
   after a press of the dice the whole suggestion shows on a line right under
   the field; that line holds a place two lines tall from the first press on,
   blank after a key; a screen reader does not hear it.
   In the app at 390 by 900 px the first owner's card after a press is taller
   than its room in every language but English,
   starts at its top and the page scrolls; in a browser at 390 px, French alone.
2. **PasswordRules.D59 again, with the measured move**
   (D23-A; The dice's move): D59-A's reason,
   that a second tap at the first tap's place still lands on the dice,
   held everywhere before the block,
   at 13.8 px (2.7 px in the French card in the app); with it,
   it holds in the app in German (14.3 px) and French (2.7 px) and in the forms
   that do not centre, and fails in the first owner's card everywhere else,
   at 25.3 to 40.0 px, the dice being 33.2 px tall.
   - **A.**
     Leave the move.
   - **B.**
     Keep the card's top where it is while a suggestion's row is shown (D23-A);
     PasswordRules' NOTE "how block 4 went" named its cost:
     a "fixed while open" rule in the card's measure tests,
     and a change to how the card's other states grow.
3. **D19 again, with the three-line exception**
   (Different from the plan, 8): in the rarest draws at 390 px,
   less than one in 10^13 with the test machine's fonts,
   five words take a third line in Add a user,
   a user's panel and the first owner's card; the place grows by that line,
   and the first key gives it back,
   moving what is under it 11.6 px in a centred card,
   23.2 px in a card at its top.
   - **A.**
     As built: a place of two lines, the exception said in the documents.
   - **B,** the plan's.
     The place as tall as the last suggestion, kept after a key:
     a second press can move things by a line.
   - **D,** new, the builder's.
     A place three lines tall, so that nothing moves:
     a line, 23.2 px, more after every press in every form,
     the first owner's card taller and the dice's move larger where it fits.
4. **A turn to the sign-in leaves the suggestion shown:**
   the turn (`toSignIn`, at another request's 401 or 409) closes no dialog,
   so the owner's suggestion stays in the page,
   the field text and the line holding it, hidden with its form,
   until the card closes.
   A turn after the card's own submit finds the line empty already;
   only a request of another part meets it without a submit.
   The other way: end the suggestion when the owner's form is hidden too.
5. **The line ended at the card's close** is the coordinating session's choice,
   the owner's to change: the shared part watches the dialog it is in.
6. **The plan's numbers that did not hold:**
   five words never on three lines, twenty characters always on one,
   the dice 36 px,
   the card taller than the window in five languages (Different from the plan,
   8, 9, 14 and 15).
7. **No real phone, screen reader or page translation was tried:**
   the line is `aria-hidden` and `translate="no"` by the page's attributes and
   Chromium's accessibility tree alone.
   The plan's check on a phone, by hand, is the owner's:
   the line, the French card scrolling, the dice's move.
8. **The whole UI suite did not run on the last tree;**
   this pull request's CI runs it.

## What verified it

- **The builder, on the first trees,** each by its exit code, 0: `make ui`,
  `make ui-check`, `gofmt -l .` printing nothing, `go vet` for linux,
  windows and darwin,
  `MASSIMO_ENV_FILE=/dev/null go test -count=1 ./internal/server/...`,
  `commit-msg.sh` on both messages;
  `reflow.sh --check` on `docs/ui.md` and `tools/ui-test/README.md`, 0 errors;
  `GUIDELINES/shared/tools/check` in a scratch clone on the plan's commit
  and each tree, twelve checks all well; `changelog.sh check --staged --strict`,
  the first tree owing no bullet, the second carrying its own.
  The first tree's `test_first_owner.py`,
  `test_keep_open.py` and `test_parts_guard.py` in all four cases, 222 passed;
  the plan's ten files on the second, 594 passed and 2 failed (the widest test,
  three lines: Different from the plan, 8),
  then `test_password_suggest.py` in all four cases, 78 passed;
  the whole UI suite in 1100-en, 814 passed.
  Seen failing: the first commit's helpers by a scratch probe (Different from
  the plan, 7);
  the new test file on the first tree's page in 390-en and 1100-en, 13 failed,
  each at the line's absence.
  `make cross` not run: no Go code changed.
  The grep for scripts that press the dice found the two UI test files alone;
  no rule for users, passwords or sign-in changed.
- **The page's reader, on the first trees:**
  `make ui` and `make ui-check`, exit 0;
  `go test -run 'TestUI|TestShared|TestState|TestOpenAPI' ./internal/server`,
  ok; `reflow.sh --check`, 0 errors;
  nine probe tests of its own in 390-en and 1100-en,
  the other languages through the browser's languages;
  the five forms looked at and photographed in six languages at 390 and 1100 px,
  the owner's card also in the app's window, light and dark,
  the others dark in English and German;
  the dice's move and the heights measured again.
  It ran no UI file of the suite.
- **The tests' reader, on the first trees:**
  `test_first_owner.py`, `test_keep_open.py`, `test_password_suggest.py`,
  `test_parts_guard.py`,
  `test_login.py`'s `test_sign_in_and_out` and `test_connect.py`'s two strict
  callers in all four cases on the second tree, 312 passed;
  the measure and short-screen tests on the first, 20 passed;
  the second tree's `test_password_suggest.py` on the first tree's page in
  390-en, 7 failed and 13 passed, each failure at the line's absence;
  Chromium's accessibility tree read over CDP; `commit-msg.sh` on both messages,
  0; `reflow.sh --check`, 0 errors.
- **The builder, after the first rework:**
  `make ui`, `make ui-check`, `gofmt -l .` printing nothing,
  `go test -count=1 ./internal/server`, `commit-msg.sh` on both messages,
  each exit 0; `reflow.sh --check` on `docs/ui.md` and the README, 0 errors;
  the scratch clone's check, twelve all well on both trees;
  the strict changelog check, as before.
  The first tree's three files in all four cases, 222 passed;
  the plan's ten files on the second in all four cases, 600 passed;
  the whole UI suite in 1100-en, 815 passed.
  Seen failing: the new test file on the first tree's page in 390-en, 7 tests,
  each at the line's absence;
  `test_the_shown_line_ends_as_the_card_closes` on the second tree before the
  rework; and the planted faults of What the reading found.
- **The page's reader, after the first rework:**
  `make ui` and `make ui-check`, exit 0;
  `go test -count=1 -run 'TestUI|TestShared|TestState' ./internal/server`, ok;
  `reflow.sh --check`, 0 errors; `commit-msg.sh` on both messages, 0;
  the touched test files and `test_parts_guard.py` in 390-en and 1100-de,
  215 passed, one worker retried after its own probe hung;
  the dialog's close driven in 390-en and 1100-de.
- **The builder, after the second rework:**
  `make ui`, `make ui-check` and `commit-msg.sh` on both messages, exit 0;
  `reflow.sh --check tools/ui-test/README.md`, 0 errors;
  the scratch clone's check, twelve all well on both trees.
  The first tree's `test_first_owner.py` and `test_keep_open.py` in 390-en,
  41 passed with two probes; `test_first_owner.py`, `test_keep_open.py`,
  `test_login.py`,
  `test_connect.py` and `test_parts_guard.py` on the second tree in all four
  cases, 414 passed.
  The second commit's own change was unchanged by this rework.
- **The coordinating session:**
  the two steps carried from the plan's commit onto main's head,
  "docs(api): The capability tables name every route the server guards".
  The first merged without conflict.
  The second met one conflict in `tools/ui-test/test_password_suggest.py`:
  main had taken AP ToldData's pull request, which moved `toggle_fold` and
  `open_fold` into `helpers.py`, and the block had added `scroll_width` to the
  same import line;
  resolved by hand, one import line holding the three,
  the file's own copies gone as main has them.
  On the carried tree `make ui` and `make ui-check`, exit 0;
  `test_password_suggest.py`, `test_first_owner.py`, `test_keep_open.py` and
  `test_parts_guard.py` in all four cases, 304 passed.
  In a clone with each commit, `commit-msg.sh`, 0 errors and 0 warnings,
  and `changelog.sh check --staged --strict`, 0;
  `GUIDELINES/shared/tools/check` over both commits, twelve checks all well.
- **Not run here:**
  the whole UI suite and the plan's ten files on the last tree;
  the whole `go test ./...`, which is the pull request's CI;
  a real phone, a screen reader, a page's translation.
