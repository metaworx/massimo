# NOTE FirstOwner v1.0: how block 3 went

Where Block 3, KeepOpen, went differently from the plan,
the choices taken in the run, and what verified it.
The block gives the page its way in to the setting `auth.keep_open`:
a server without users can be kept open on purpose, from the card, and undone.

## What it made

- **"Keep it open…"** under the first owner's form in the sign-in card,
  beside Skip for now; not offered where the server is kept open already.
- **The confirmation,** in the form's place and of its height,
  so the card neither moves nor resizes:
  the owner's warning first and word for word,
  "Not recommended - ONLY use it on a local network you trust!";
  what keeping it open means; this page's address, which a tap copies;
  a field to enter it into, a line saying when the address differs;
  Back, and Keep it open, off until the address entered is this page's own.
- **The address counts as the browser reads it:** the same origin, scheme, host and port,
  its case, spaces, a default port typed and a slash at the end not mattering;
  the path "/" or the page's base, no user, query or fragment; no scheme, no match.
  It is a confirmation, not a lock:
  anybody on the local network may write the setting through the API.
- **Keep it open** writes the setting, reads who is in, and closes the card as Skip does;
  the page starts once; no card at any later load.
  A refusal for lack of a session, or because the server has users now,
  turns the card to the sign-in; any other is said under the field.
- **While a request of the card runs,** Create the owner or Keep it open,
  Skip for now, the app's Cancel, Back and the card's ☰ "Select server" are off.
- **System settings → Users** shows a switch while the server is kept open, on.
  Turned off, it is saved at once and stays in sight, off, for that showing;
  who is in is read again by a read that asks for no owner;
  the next load asks.
- **The banner's words are unchanged.** Fourteen new keys in six languages,
  French, Italian, Dutch and Polish machine-translated.
- **Documents:** `docs/ui.md`; `docs/api-v1.md`, two lines beside the setting;
  `docs/app.md`, where a login by fingerprint not kept is said in a toast
  on the first owner's form, a sentence Block 2 left untrue;
  the first start's text with its two copies, in `README.md` and the site's page.
- **Tests:** `tools/ui-test/test_keep_open.py`, new;
  `test_first_owner.py` and `test_parts_guard.py` changed.
- **One commit,** as the plan names it:
  "feat(ui): An open server is kept open after its address is entered".

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **A design turn for Blocks 3 and 4 together,**
   while Block 2 was being read; the coordinating session answered.
   Built in an exported copy of Block 2's branch head.
2. **D28-A.** The address is parsed by the browser's own reader of addresses
   and compared by origin.
   The other way: a plain text comparison,
   which refuses a typed default port and another spelling of the same host.
3. **D29-A.** No Paste button: the field and the copy alone.
   The other way: a button that reads the clipboard,
   with a permission prompt, in secure contexts only.
4. **D30-A.** The banner's words stay on a server kept open.
   The other way: "kept open on purpose" added.
5. **D31-A.** The switch turned off does nothing at once; the next load asks.
   The other way: the card opens at once.
6. **D32-B: the switch only undoes.**
   It is the coordinating session's choice, against the builder's recommendation,
   and it departs from the plan's words, "shown while the server has no users".
   The switch is shown while the server is kept open;
   keeping a server open is the card's confirmation alone,
   so that the confirmation the owner asked for,
   the warning and the address entered, has no one-tap way around it.
   The other way: a switch that works both ways, its hint the warning.
7. **D33-A.** The address table's rows are tested on the test server's own origin
   and with the page opened at `localhost`;
   an https page, an IPv6 host and a path prefix are covered by reading.
   The other way: an entry of the page for the tests alone.
8. **D44-A,** from the first reading.
   After the switch is turned off who is in is read again,
   so that the page knows what the server says,
   and that one read does not ask for the owner;
   every other read asks as before (D27).
   The builder had first left that read out,
   since it would have opened the card over System settings at once, against D31,
   and kept a state of its own for the load:
   the page then went on believing the server kept open.
   The other way, a reader's: the card opens by itself only while the page's start waits,
   never at a running page's read.
   Not taken, because the app's acceptance test and Block 2's tests
   open the card by a read of who is in on a running page.
9. **From AP MobileApp's session's reading of Block 2's pull request,** done here:
   the sentence in `docs/app.md`;
   the card's ☰ "Select server" off while a request of the card runs.
10. **The next free number** is FirstOwner D45;
    D34 to D41 are Block 4's, answered already, D42 and D43 Block 2's.

## What the readings found

The builder of Block 2, a sub-agent on Opus 5.5, built it;
Block 2's two readers read the recorded tree through git and ran tests in exports:
one the flow, the states, the session and who may do what;
the other the part and its look, the texts, the tests,
the documents, the tree and its message.

Both confirmed:
the steps with what is kept and emptied;
the focus, the field on a desktop, the warning on a touch screen;
the card's place and height measured and unchanged;
two Confirms send one request; each refusal as designed;
`login.js` unchanged but for the one word `loadMe` takes;
at 390 px in six languages, light and dark, in a browser and in the app,
no horizontal scroll and every button's text on one line.

One must: **after the switch was turned off the page kept believing the server kept open**
for the rest of the load, shown by a reader's test:
the card opened afterwards did not offer "Keep it open…",
and kept open again from the card, the switch did not come back.
Now the page reads who is in again, by a read that asks for no owner (D44-A).

Seven shoulds, done, among them:
the flag that turns the card's ways out off cleared at the turn to the sign-in,
where Select server had stayed off;
the copy's test asserting that something was copied, in a browser
and in the app without the clipboard;
`docs/ui.md` saying all the address check does, and that it is no lock;
the switch's refusal a text of its own, where it read ": refused …".

The second reading: both readers found the rework settled, with no new fault:
the page believes what the server says in every order tried,
a refused turn-off and a server that got users meanwhile among them;
only the switch's read asks for nobody.

## For AP MobileApp's session

- **Unchanged:** the bridge's calls.
- **New markup:** `#setup-keep-open` under the form;
  `form#keep-open` in the form's place,
  with `#keep-open-warning`, the address as a text that copies itself,
  `#keep-open-address`, `#keep-open-hint`, `#keep-open-msg`, Back and Keep it open;
  in System settings → Users `#keep-open-switch`.
- **The copy without the clipboard's API,** the app over plain HTTP,
  works by the page's other way;
  the card's "Select server" is off while a request of the card runs.
- **The phone as its own server** (D18-A):
  the setting is written by `PUT /api/v1/settings` with `auth.keep_open`;
  the page's confirmation is for a person.

## For Block 4

- `askLogin("owner")` opens the card on a running page;
  "Keep it open…" hides itself where the server is kept open.
- Whether the server is kept open is who is in's `kept_open` alone.
- The flag of a request under way is cleared at the turn to the sign-in;
  Block 4 keeps the card from turning during its own creation by a narrower one.

## Seen and left

- The address check takes a few odd spellings of the same address,
  a tab inside, a backslash for a slash: the same origin each;
  `docs/ui.md` says it as "as the browser reads it".

## What verified it

- **The builder, on the last tree,** each exit 0: `make ui`, `make ui-check`,
  the Go tests of `internal/server`, `sync-docs.sh --check`, `reflow.sh --check`
  on every changed Markdown file, `commit-msg.sh`;
  before the rework also `gofmt -l .` empty, `go run ./tools/apitypes -check`,
  `go vet` for linux, windows and darwin, the Go tests of `tools`.
- **UI, the builder's:** the whole suite in 1100-en and 390-de,
  1259 passed and 2 failed,
  both one test of the builder's own with a wrong expectation, corrected afterwards;
  `test_keep_open.py` in all four cases after it, 60 passed.
- **The readers' own runs** in exports of the new tree:
  `test_keep_open.py`, `test_first_owner.py`, `test_users.py`,
  `test_admin_updates.py`, `test_connect.py`, `test_login.py` and the guard
  in all four cases, 516 passed;
  seven files of the sign-in with a reader's three probes in two cases, 170 passed.
- **Shown failing:** on Block 2's tree the new file's tests fail where they should,
  its one forward guard named;
  the builder's two new tests of the must fail on the tree before the rework.
- **The coordinating session:** the recorded tree and the builder's copy equal;
  the message; the one change to `login.js` read; the changelog's check, strict;
  `GUIDELINES/shared/tools/check` on the commit in a scratch clone,
  its one finding a commit of main that is not this block's.
- **The merge of main** brought Block 2's merge with its NOTE
  and the fix of the pairing's test on main; nothing met.
- **Not run locally:** the whole UI suite on the final tree in all four cases, CI's.
