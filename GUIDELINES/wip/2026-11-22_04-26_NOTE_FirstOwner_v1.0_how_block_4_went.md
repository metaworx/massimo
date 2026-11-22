# NOTE FirstOwner v1.0: how block 4 went

Where Block 4, Ways, went differently from the plan,
the choices taken in the run, and what verified it.
It is the plan's last block.
Before it, once the page had gone on without an owner, skipped or kept open,
only a reload asked again, and the open server's warning could not be put away.

## What it made

- **The warning** of an open server with a store has "Create the owner…" and Hide.
  Hide hides it in this browser while the server has no owner,
  one key per origin in the browser's storage,
  and is forgotten as soon as this browser learns that the server has users.
  A server without a store keeps its warning as it is, without buttons.
- **☰** has "Create the owner…" where Sign out stands,
  on a server that can have an owner and has none, kept open or not;
  not on the card's own ☰.
- **System settings → Users** on a server without users shows a section of its own:
  a line, "Create the owner…",
  and the switch that undoes keep open while the server is kept open;
  no password sign-in, list or Add a user there.
  With users the section is as it was.
- **Each opens the sign-in card with the first owner's form on the running page.**
  The page is hidden behind it and stands still:
  its poll stopped and its stream closed until the card is left.
  The button under the form reads "Back to the page" there,
  marks nothing as skipped and gives the focus back to what opened the card.
  After Create the owner the page it was opened on is shown again, read as the owner.
- **A request refused while Create the owner runs,**
  the owner existing already and the page's session not yet,
  no longer turns the card:
  a read waits for the sign-in the creation ends in and is sent again under the owner;
  a write is said not sent.
  Where Create then ends without a session, the card turns to the sign-in.
- **The turn to the sign-in on a running page,**
  an owner made elsewhere while the card was up,
  answers what the open page had under way as a refused request's sign-in does:
  the pushes held, a setpoint still waiting dropped;
  another user's sign-in forgets the open page.
- **Whether the server is kept open** is one answer for the card, Users and the rest,
  from who is in (`shared/owner.ts`).
- **Texts:** five new keys in six languages, French, Italian, Dutch and Polish
  machine-translated; `docs/ui.md` on the three ways and the card on a running page.
- **Tests:** `tools/ui-test/test_ways.py`, new;
  `test_main_menu.py`, `test_parts_guard.py` and `test_first_owner.py` changed;
  `test_connect.py` and `test_login.py` unchanged and passing.
- **One commit,** as the plan names it:
  "feat(ui): The owner can be created from the banner, the menu and Users".

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **Designed with Block 3** in one design turn;
   started while Block 3 was being read, on Block 3's tree,
   and taken onto Block 3's commit when that existed.
2. **D34-A.** The page is hidden behind a card the user opened, as at every opening.
   The other way: the card over the page, with a backdrop.
3. **D35-B: the page behind that card stands still.**
   It is the coordinating session's choice, against the builder's recommendation.
   Its poll is stopped and its stream closed,
   both started again when the card is left.
   Block 2's first reading had shown why:
   between the moment the owner is made and the moment the page holds its session
   every request of the open page is refused,
   which turned the card to "The owner exists now: sign in" in the middle of Create
   and said a write was not sent.
   What still goes on behind the card:
   the log's own poll under All Cubes, an update being followed,
   a restart being waited for, none of which opens the stream there;
   a read of theirs in that window waits and is sent again under the owner.
   The other way: the poll and the stream go on.
4. **D36-A.** On a card the user opened the button under the form reads "Back to the page"
   and marks nothing as skipped.
   The other way: "Skip for now", as at the start.
5. **D37-A.** After Create the owner on a running page the same page is shown again.
   The other way: Rooms, as after a sign-in.
6. **D38-A.** One storage key per origin for the hidden warning:
   two servers behind one origin share it.
   The other way: a key named with the page's base path.
7. **D39-B: a server without a store keeps its warning as it is, without buttons.**
   It is the coordinating session's choice, against the builder's recommendation.
   The plan lists it under what stays as it is
   ("A server without a store: its banner, no owner, no card"),
   and hidden there the warning would be hidden for good,
   on the one kind of server that can never get an owner from the page.
   The other way: Hide offered there, no Create.
8. **D40-A.** No "Create the owner…" on the card's own ☰.
   The other way: the entry there too.
9. **D41-A.** "Users opens the card" is a button in the section.
   The other way: opening the section opens the card.
10. **D45-A,** from the first reading.
    The warning's hiding is forgotten as soon as this browser learns of users:
    at every opening of the sign-in and every turn to it, a refusal meaning users,
    not only where who is in is later read as a user's.
    Before, a browser that was never signed in kept the warning hidden
    on a server that had got an owner elsewhere, lost all its users and was open again.
    The other way: as first built.
11. **The plan is built with this block:**
    its four blocks are on main once this one is merged.
    Its retirement is not part of the run; it waits for the owner's word.
    The next free number is FirstOwner D46.

## What the readings found

The builder of Blocks 2 and 3 built it;
the same two readers read the recorded tree through git and ran tests in exports:
one `login.js`, the card opened on a running page and the session;
the other the three ways' parts and their look, the warning's storage,
the texts, the tests, the documents, the tree and its message.

Both confirmed:
the sign-in on a server with users unchanged, the new branches unable to run there;
the page standing still and started again exactly once,
one poll interval and one more ticket of the stream per opening;
the turn, then another user's sign-in, forgets the open page;
a setpoint stepped just before the opening lands; no turn while Create runs;
the warning, ☰'s entry and Users right for each kind of server,
with users the Users section element for element as before;
the guard catches a replaced button.

One must: the message named two files the tree does not change.

Eight shoulds, done, among them:
the words on "standing still" say exactly what stops and what goes on,
and a restart's end no longer opens the stream behind the card;
a refusal not turned while Create ran turns the card when Create ends without a session,
where Create failing on the network had left the owner's form though the server had users;
Hide reaches ☰'s button through a shared holder, not a selector into the header;
D45; tests for a half-typed form of System settings kept through the card,
a storage that throws at the read, and the warning's buttons in six languages.

The second reading: both readers found the rework settled.
The stream opens exactly once however the card is left, and never stays closed;
behind a card that stands still the page learns of an owner made elsewhere
through the card's own requests, or, once the card is left, the stream and the poll,
a test of Block 2 changed to say so;
the hiding is forgotten at a sign-in's opening and at a turn, not at the owner's form's.
Two small points on tests were done.

## For AP MobileApp's session

- **Unchanged:** the bridge's calls; `test_connect.py`:
  its check that Select server comes just before Sign out holds,
  ☰'s new entry being placed after Sign out's.
- **New markup:** `#menu-owner` in ☰;
  the warning's `#open-banner-create` and `#open-banner-hide`;
  `#users-open` with `#users-create-owner` in System settings → Users.
  In the app the card's Cancel and Select server behave as in Block 3.
- **The hidden warning** is kept per origin in the WebView's storage.

## For AP UiParts' Block 15, which lands after this block

- **Plain names this block newly uses** from `login.js` and `app.js`:
  `state.pollTimer`, `closeEvents`, `startPolling`, `showView`, `currentView`,
  `connectEvents`, which now opens no stream while a card stands over a running page;
  from parts: `askLogin` (`shared/owner.ts`), the storage's `readStored` and `store`.
- **Page globals the new tests call:** `loadMe`, `askLogin`, `api`, `connectEvents`,
  `registerPassword`, `signIn`, `state`, `currentView`, `t`, `localStorage`.

## Seen and left

- **A test of the log from AP UiParts' Block 11,**
  `test_a_late_question_of_older_reaches_nothing_of_the_next`,
  failed once and passed on its retry in each whole-suite run of this block.
  It counts every line of the log, also the next user's own live ones;
  the page is right: a reader read it and waited for a late answer, and none lands.
  It is corrected in AP UiParts' Block 15.
- **Behind a card the user opened** the log's poll, an update's follow
  and a restart's wait go on (D35).

## What verified it

- **The builder, on the last tree,** each exit 0:
  `make ui`, `gofmt -l .` empty, `make ui-check`, `go run ./tools/apitypes -check`,
  `go vet` for linux, windows and darwin, the Go tests of `internal/server` and `tools`,
  `make cross`, `sync-docs.sh --check`,
  `reflow.sh --check` on every changed Markdown file, `commit-msg.sh`.
- **UI, the builder's:** the whole suite in all four cases, 2535 passed
  and 1 failed, which passed on its retry, the log's test under "Seen and left";
  the block's files, the guard, `test_login.py` and `test_connect.py` in all four cases.
- **The readers' own runs** in exports of the new tree:
  nine files of the ways and the sign-in, the guard among them, in all four cases,
  458 passed; seven files of the sign-in with a reader's six probes in two cases,
  196 and 6 passed.
- **Shown failing:** on the tree before the rework the new tests of the stream,
  of the refusal during a failed creation and of the hiding;
  on Block 3's tree the new file's tests, but the forward guard without a store.
- **The coordinating session:** the recorded tree and the builder's copy equal;
  the message; the changes to `login.js` and `app.js` read; the changelog's check, strict;
  `GUIDELINES/shared/tools/check` on the commit in a scratch clone.
