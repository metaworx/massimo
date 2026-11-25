# NOTE LooseEnds v1.2: how block 6 went

Where Block 6, Key, went differently from the plan,
the choices taken in the run, the check of one server test the owner asked for,
and what verified both.

## What it made

- **A refused API key is said** in the sign-in card, on the key's way,
  in a line of its own under the key's field:
  "The server does not know this API key.", in six languages,
  FR, IT, NL and PL machine-translated and marked.
  The field is emptied and has the focus.
- **The key is read with first:**
  `GET /api/v1/me` with the key as the bearer,
  and it becomes the page's token only once the server has taken it.
- **An answer that comes once its opening has ended** makes no token and writes
  nothing: another way signed in meanwhile, the card closed or opened anew,
  a sign-out.
- **The key's line and the providers' line** are drawn in the refusal's colour,
  as the password's line is.
- **One commit:**
  "fix(ui): The sign-in card says that an API key was refused".
  It came about at the owner's word of 2026-11-25 on the second walk along the
  first owner's path, "Yes, fix the refusal."
  (D43-A), built by the builder of Block 2, resumed for it.

## Where the refusal was lost

`submitKey` made the key the page's token at once (`setToken`,
which in the app hands it to the bridge to keep) and finished as a password's
sign-in does (`finishSignIn`).
At the start nobody is signed in, so the sign-in read nothing,
the card closed, and the start's read of who is in went out with the key.
That read was refused with 401: `api()` dropped the token,
and the 401 opened the card anew on the Password way, the field empty,
nothing said.

On a running page it was worse than the walk saw:
a wrong key cost the signed-in user their session.
`setToken` replaced the session and logged it out at the server;
the sign-in's read of who is in was then refused, and the page forgot the user.
The builder saw it on the old code: the page's token was empty afterwards.

## What the card says for each answer

In every case the card stays on the key's way, the focus in the key's field,
and the key is not the page's token.

| Answer | The card says | The key's field |
|--------|---------------|-----------------|
| 401, a key the server does not know | "The server does not know this API key.", in the page's language | emptied |
| 403, a user or an integration disabled | "Not signed in: {the server's words}" | emptied |
| 429, a lockout | "Not signed in: {the server's words}" | kept for the retry |
| a 5xx | "Not signed in: {the server's words}" | kept for the retry |
| the server not reached | "Not signed in: {the browser's words}" | kept for the retry |

## What is left of a refused key

- **The page's token:**
  nothing; `setToken` runs only after a 200.
- **Storage:**
  nothing; the key is in no storage, log, URL or shown text.
- **The app's bridge:**
  nothing; it is handed only a key the server took.
- **On a running page** the session stays the page's token, live at the server,
  and the user stays in,
  while the address is not locked out (For the completion gate, 1).

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **The key is read with first.**
   The plan left to the block where the refusal is lost and how it is caught.
   A read of its own, with the key alone as the bearer,
   to the page's own origin;
   its answer is handed to `finishSignIn`,
   so that the sign-in reads who is in no second time on a running page.
   The other way: the key made the token and taken back at the refusal,
   which leaves it with the bridge and costs a running page its session meanwhile.
2. **The start's card reads who is in twice on a right key:**
   the key's try, then the start's own read that waited for the card;
   before, once.
   Left and written down in the message and `docs/ui.md`;
   a right key counts nothing.
   The other way: the try's answer handed on to the start's read as well.
3. **The refusal has a line of its own under the key's field,**
   `#login-key-error`, not the card's message line the password's way writes.
4. **A 403 empties the field as a 401 does;**
   a lockout, a 5xx and a server not reached keep it for the retry.
   The plan said the field is empty at a refusal,
   and named the server not reached and a lockout among the other answers;
   the 403 and the 5xx are the block's.
5. **The colour rule takes the providers' line too,**
   which had no colour rule either, from before this block.
6. **Tests beyond the plan's:**
   the line in the password's refusal's colour;
   the right key after a refusal counted,
   one stream ticket and two reads of who is in; a 403, a 429,
   a 500 and a server not reached;
   a wrong key on a running page leaving the session,
   the page's token and the user;
   a key's answer held while the Password way signs in;
   a refusal held while the card is closed and opened anew.
   German is held by the suite's German cases.
7. **Beyond the plan's files:**
   `src/signin/store.ts` (the line's text, the focus "key"),
   `style.css`, and the two type declarations of the plain scripts.
   `docs/ui.md` and the card's head comment now say that a refused password keeps
   what was typed, where a refused key goes from its field.

## What the readings found

One builder built it in an exported copy of main.
Two readers read the recorded tree:
one the sign-in's code and its security, the other the tests, the texts,
the look and the documents.

Confirmed: the key goes to the page's own origin alone,
with no session beside it; a wrong key is one counted guess,
where the old flow sent several reads with it;
the old fault on a running page is gone; the app's bridge gets no refused key;
the line is one line at 390 px in six languages, and the card does not jump.

Two musts, done:

- **An answer after its opening had ended still made the key the page's token.**
  While the key's read was out, the fingerprint or the Password way could finish
  and close the card; the key's 200 then replaced that session's token,
  handed the key to the bridge and, in a browser, logged the first session out.
  A refusal let go after the card was closed and opened again wrote its line into
  the new opening, emptied its field and took the focus.
  Before the block the key's way had no such window.
  Now nothing is done after the read unless the session,
  the opening and its open state are unchanged; two tests hold it.
- **The new line had no colour:**
  it drew in the body's colour in light and dark,
  where the password's refusal is red.
  It takes the rule, with the providers' line; a test compares the two colours.

Six shoulds, done: the second read at the start's card written down (2 above);
the two texts that said every refusal keeps what was typed;
two of the three first tests failed on the tree before for the first test's
reason, and now check their own thing first;
the four answers without a test (6 above);
the test's helper checks the way before the text;
the new import stands with the other shared ones.

The second reader planted six faults in a copy,
each caught: the token set first, the card turned to the Password way,
the field not emptied, no focus, the session logged out at the server,
the bridge keeping the key.
The builder fixed one test of its own after the rework:
the late refusal's test reopened the card before the old opening had ended,
2 of 8 runs, and now waits for it.

The second reading of the code found nothing left that must change:
in every window traced, the late answer does nothing,
and the documents' new sentence on the lockout is exact.

## The check of `TestLogAfterReadsABurstOnce`

The owner asked whether this test, changed by Block 5, is fixed.
On main of 2026-11-25, in the builder's copy:

| Run | `store` | `ring` |
|-----|---------|--------|
| `-race -count=20`, `MASSIMO_TEST_SCALE=2`, alone | 20 of 20 | 20 of 20 |
| the same, beside a full UI run | 20 of 20 | 20 of 20 |
| armv6 emulation in Docker, `-count=5`, `MASSIMO_TEST_SCALE=4`, `GOPROXY=off` | 5 of 5 | 5 of 5 |
| plain `-count=50` of `/store` | 50 of 50 | not run |

No failure, so no change was needed and none was made.
Of the 60 most recent CI runs across all branches before the check,
two had failed, neither on this test.

## The slip in the run

One of the builder's background UI runs started from the worktree's root,
not from its copy.
It ran the worktree's own code, so its result says nothing of the block,
and rebuilt the worktree's ignored bundle; it also wrote a log one level above
the worktree, which the builder deleted.
Nothing tracked changed: `git status` in the worktree stayed clean,
checked by the coordinating session.
The run was made again in the copy.
It was the UI run beside which the second race run of the check above ran.

## Seen and left

- **The docstring of `test_a_late_key_answer_makes_no_token`** says it fails on
  the page before.
  Before the block it fails for another reason (the key sent no read of its own
  there); it fails for the reason it names against the block's first tree.
  The next change of `test_login.py`'s.
- **The focus assertion of `test_a_late_key_refusal_writes_nothing`**
  cannot fail either way: the key's field is hidden on the Password way.
  The next change of `test_login.py`'s.
- **The password's way has the same pattern** as the key's late answer,
  older than this block, and unchanged by it:
  the owner's, for a change of its own.

## For the completion gate

1. **A wrong key counts against the address's lockout,**
   and a locked address is refused even with a live session:
   after about ten wrong keys in a card opened from a running page,
   the signed-in user's own requests are refused too, until the lockout ends.
   The server's behaviour, older than this block;
   `docs/ui.md` says the session goes on "while the address is not locked out".
2. **"Not signed in: {the server's words}"** shows the server's English in a
   German page for a disabled user, a revoked integration or a lockout,
   as the password's way does.
3. **A right key at the start's card** costs one read of who is in more than
   before (2 above).
4. **The owner's check on a phone** of the key's way was not made.

## What verified it

- **The builder, on the first tree:**
  `make ui-check`, `gofmt`, `go vet` for linux, windows and darwin,
  `make cross`, the Go tests of the server, the command line and `serve`,
  `reflow.sh --check docs/ui.md` and `commit-msg.sh`, each exit 0;
  the plan's seven UI files in all four cases, 466 passed;
  the whole suite in 1100-en, 772 passed.
- **The builder, after the rework:**
  `make ui`, `make ui-check`, `gofmt`, the server's Go tests,
  `reflow.sh --check docs/ui.md` and `commit-msg.sh`, each exit 0;
  the seven files in all four cases, 490 passed;
  the whole suite in 1100-en, 778 passed.
- **Seen failing first:**
  the three first tests on the tree before, 12 of 12 in the four cases;
  after the rework,
  the two late-answer tests and the colour test on the first tree,
  the four other answers and the two reworked tests on the tree before,
  each for the reason it names.
- **The first reader:**
  `make ui`, `make ui-check` and the page's Go tests in its export, exit 0,
  and again `make ui` and `make ui-check` on the reworked tree;
  it ran no UI test.
- **The second reader:**
  the card by hand in all four cases, the app's variant with a fake bridge,
  the line at 390 px in six languages;
  the six planted faults;
  `make ui` and `make ui-check`, exit 0;
  the seven files in all four cases, 466 passed.
- **The coordinating session:**
  the rework's diff of `Card.tsx` and `style.css` read;
  on the commit `commit-msg.sh`, 0 errors, the strict changelog check passes,
  and `GUIDELINES/shared/tools/check`, twelve checks all well.
  `test_login.py`, `test_ways.py`, `test_first_owner.py`, `test_keep_open.py`,
  `test_sign_in_updates.py` and `test_sign_in_resend.py` in all four cases,
  424 passed; the whole UI suite in 1100-en, 799 passed.
