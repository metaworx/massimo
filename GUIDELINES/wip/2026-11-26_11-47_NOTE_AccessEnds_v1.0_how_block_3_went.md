# NOTE AccessEnds v1.0: how block 3 went

Where Block 3, OpenSetup, went differently from the plan,
the choices taken in the run, what the readings changed,
the log entry the owner asked for, and what verified it.

## What it made

The first regular user of a server that had none is told once
what was set up before them and still works, where anything was
(LooseEnds.D47-A with a log entry, D11-A to D15-A).
A second owner, and a server where nothing was set up, are told nothing.

- **What is listed** (`store.OpenSetup`):
  the integrations active with a live key,
  or whose approval's code can still collect one (`RequestLife`),
  by the name the owner gave, else the one reported;
  the sign-in providers enabled, one whose sign-in creates users said so;
  the roles added, and the default ones changed or disabled, by their names,
  owner and integration, which never change, left out;
  the capabilities turned off.
  A request still waiting is not listed (D15-A).
  Names, uuids and the keys of roles and capabilities only:
  never a key, a token, a password or a secret.
- **The page:**
  after "Owner … created and signed in.",
  the toast says "Still in effect from before you: {what}.
  Check it.", in the warn tone, for 30 s, with its close button (D13-A),
  never taller than the window, its Show under the text at phone width.
  Show opens System settings → Integrations,
  the card in view and its heading in focus,
  or the log with its title in focus where no integration is listed (D11-A).
  A sign-out takes the toast away.
- **The API:**
  the first owner's `POST /api/v1/users` answers `open_setup`, the same lists,
  only where there is any (the schemas `UserCreated` and `OpenSetup`).
- **The command line:**
  `massimo user add`, where it makes the first regular user whatever its role
  (D12-A), prints a second line and writes the log's line itself,
  "by the command line"; the store says whether the user is the first,
  in the transaction that writes it.
- **The start:**
  the owner `MASSIMO_API_TOKEN` makes,
  at a start and at a reload of the settings file,
  is told in the log as well (D14-A).
- **The log:**
  a line of the new kind `open_setup`, source system, at warn,
  a log alert until a person marks it seen,
  with its line in serve's log (For the completion gate, 2).
- **One commit:**
  "feat(server): The first owner is told what was set up while the server was
  open".

## Different from the plan

Against the plan's block, item by item:

1. **`OpenSetup` (change 1):**
   - An integration whose approval's code can still collect its key,
     until `RequestLife` after the approval,
     is listed beside one with a live key:
     whoever holds the code can still collect it.
     The plan said a live key only.
   - Owner and integration, the fixed roles, are left out;
     a role `integration-v0` is listed as any other.
     The first build left out `integration-v0` at version 1,
     which migration 0021 made;
     but a request on an open server can make a role of that key (201),
     which was then never listed.
     The exemption is dropped (What the reading found).
     A store that migration gave the role still has the regular users it was
     made for, and lists it only once all of them are gone.
   - The roles carry their names (`{key, name}`) in the store, the answer,
     the stored line, the OpenAPI file and `api.d.ts`;
     the plan's lists were of keys.
2. **The Warn line (change 2) is the `open_setup` event's own line** in serve's
   log (AP OneLog), not a second `slog` call.
   Its message is "the first regular user was made;
   what was set up before it still works: check it",
   where the plan's said "the first owner was made;
   what was set up while the server was open still works: check it":
   `user add` may make a user of any role (D12-A),
   and the line and the event carry that `role`.
   It has no `remote`: the request's address stays in the Info line
   `first owner created` beside it,
   out of the stored line that everyone who may read the log sees.
   It is said with a context of its own (`context.WithoutCancel`)
   once the owner is written, so that a client gone at that moment does not
   lose it for good.
3. **The start and the reload (change 3)** as the plan has it:
   the start reads `Bootstrap`'s result,
   and `SetToken` tells where it made the owner.
4. **The kind (change 4)** is also in `eventline.go` (its line's message)
   and in `logalerts.go`, an explicit rule that a mark alone settles it;
   its text in `src/log/text.ts` is built by `src/model/opensetup.ts`,
   which the toast shares.
5. **`user add` (change 5):**
   - "First" is decided by a new store call, `AddCommandUser`,
     which counts the regular users in the transaction that writes the user,
     not before the add as the plan said:
     with the page's first owner and `user add` started a moment apart,
     the count before the add printed the line for a user that was not first,
     and the log held two lines.
     `commandStep`, nil but in the tests,
     lets a test add a user from a second store inside that window.
   - The second line is "still in effect from before this user:
     1 integration (Kitchen); roles added or changed: viewer;
     check them in System settings", a part for each list joined by "; ",
     where the plan's example joined them by ", "
     and said "set up while the server was open" and "roles changed".
   - Integration names are printed without their control and format characters
     (`terminalText`, Unicode Cc and Cf),
     such a name coming from a request without a credential; beyond the plan,
     the same in `user list`'s SHOWN AS and in `user tokens`' labels,
     where an integration's key carries the name it reported.
   - The command line writes the stored line alone, no line of serve's;
     a serve running beside it writes none either, and its log lists the line.
6. **The toast (change 6):**
   - Show calls `showCard("sys-integrations", true)`,
     as `home.js`'s `showFound` does:
     a smooth scroll on a page just shown stops short.
   - Show for the log gives the log's title the focus,
     through a new `MassimoParts.log.focusHead`.
   - Show does nothing once the owner's session has ended,
     and a sign-out takes every toast away (`hideToast` in `login.js`'s
     `forgetSession`), not only this one.
   - The warn tone is new for the toast (`components.js`).
     The toast is a grid (`style.css`): its action under the text below 760 px
     and beside it from 760 px, its height capped at the window's less 32 px,
     its text scrolled between × at its top and the action at its bottom,
     both sticky.
     This holds for every toast with an action:
     the Cube search's Show sits under its line at phone width too.
7. **The texts (change 7)** say words that hold both ways (For the completion
   gate, 4): `setup.openSetup` "Still in effect from before you: {what}.
   Check it." / "Noch wirksam aus der Zeit vor dir: {what}.
   Prüfe es."; `log.sum.openSetup` "{who};
   still in effect from before this user: {what}",
   with `log.sum.openSetupOwner` "First owner {user}" and
   `log.sum.openSetupUser` "First user {user}, {role}",
   "by the command line" right after the user;
   the parts "roles added or changed" and "capabilities off",
   the integrations by plural forms, and `openSetup.createsUsers`, new.
   German Show is "Anzeigen", as `discover.show`, not the plan's "Zeigen".
   FR, IT, NL and PL machine-translated and marked;
   French reworded after the second reading.
8. **Tests beyond the plan's:**
   - `internal/store`:
     `TestAddCommandUserSaysFirst` (first beside an integration's users,
     not beside a regular or a disabled user,
     not for a refused add) and `TestAddCommandUserCountsInItsTransaction`,
     on the package's serial list;
     `TestOpenSetup` also holds a role `integration-v0` made by a request,
     a removed provider and a removed role.
   - `internal/server`:
     `TestFirstOwnerSaysWhatWasSetUp` also holds the answer's providers,
     `auto_create` and `capabilities_off`, a named role,
     "(creates users)" in serve's line and the provider's secret absent;
     `TestTokenOwnerSaysWhatWasSetUp` runs on `startServer` and holds the stored
     line and the alert at a start and at a reload.
   - `cmd/massimo`:
     `TestUserAddSaysWhatWasSetUp`, on the serial list,
     holds all four parts and a name holding ESC, U+202E,
     U+2066 and U+2069 in the second line, `user list` and `user tokens`.
   - UI: the toast without `open_setup` is a case of the new test,
     not an edit of `test_create_the_owner_from_the_card`,
     to stay clear of Block 4, then in review beside it;
     a list of 24 integrations stays inside the window at 390 and 1100 px,
     × and Show in its visible part; Show pressed by Enter;
     the card's place in the window;
     `test_a_sign_out_takes_the_owners_toast_away`;
     `test_log_open_setup_line` in `test_log.py`.
9. **Documents beyond the plan's:**
   the OpenAPI file's list of the server's own kinds names `open_setup`;
   `docs/ui.md`'s regions' table names `log.focusHead` and its caller;
   `docs/store.md` its commands;
   and the documents say what is listed is "as a rule" set up while the server
   was open, "else left by an owner since removed".
10. **The CHANGELOG fragment:**
    "The first regular user is told what was set up before them and still works,
    in the page, by `massimo user add` and in the log."
    The plan's said "The first owner is told what was set up while the server
    was open and still works, …".
    The commit's summary is the plan's.
11. **The verification:**
    the race run on the first tree alone;
    `make cross-check` on the first tree in place of `make cross`,
    `make cross` after the first rework;
    `GUIDELINES/shared/tools/check` by the coordinating session on the commit,
    the exported copy having no git.
    The block was built on Block 2's second commit, then in review,
    since both change `users_api.go`, and carried onto main's head.

## Choices the plan left open

- **The summary holds objects:**
  integrations with uuid and name, providers with key, name and `auto_create`,
  roles with key and name;
  a `LogValue` keeps serve's line to plain names.
  The other way: names alone.
- **`via` is set for the command line alone.**
  The other way: `api` and `token` too.
- **An integration's name** is the one the owner gave, else the one reported.
  The other way: the reported one alone.
- **"First" decided in the store's transaction,** by a call of its own.
  The other way: the count in the command, as the plan had it,
  which two processes at once get wrong.
- **A test of that window** through a step nil but in the tests,
  as `pruneStep` is.
  The other way: no test of the race, as the first rework had it.
- **No test of `context.WithoutCancel`:**
  nothing lets a test cancel between the owner's write and the read.
  The other way: a step for the tests there, as `commandStep` is.
- **Names stripped for the terminal,** not quoted.
  The other way: every name quoted (`%q`).
- **A sign-out hides every toast.**
  The other way: the first owner's alone, its Show alone checking the session.
- **The log's title in focus through `MassimoParts.log.focusHead`.**
  The other way: the focus left on the page's body, as the first build had it.
- **The toast's layout:**
  a grid, the action under the text below 760 px,
  the height capped and the text scrolled between a sticky × and a sticky
  action.
  The other way: the action beside the text at every width,
  the toast as tall as its text, as before.

## What the reading found

One builder built the block in an exported copy of Block 2's second commit.
Two readers read the first tree:
one the server, the store, the log, the command line and their security,
the other the page and the tests.

Confirmed: what counts as set up is right in every store state probed (requests
waiting, refused or merged; integrations disabled,
approved and not collected within and after `RequestLife`, keys revoked;
providers enabled, disabled and removed; roles made and removed;
default roles changed or disabled; the fixed roles; capabilities off);
`OpenSetup` reads no client's secret;
the eleven release dumps migrate with nothing listed,
the one without users giving no false alarm;
two first owners at once answer 201 and 409, with one line and one Warn;
`user add` beside a running serve prints both lines,
the serve then answers 401 and lists the line and its alert; no key, token,
password or secret is written anywhere,
and nobody gets access they did not have; the kind is named in all its places;
the toast's text is exact in English and German,
its warn edge visible in light and dark,
with no horizontal scroll at 390 or 1100 px;
and the new tests failed on the base for the reasons they name.

Two musts, done in the first rework:

- **The OpenAPI file's list of the server's own kinds stopped short** of
  `open_setup`.
- **Show did not bring the Integrations card into view:**
  3 s after Show its top was at 890 of 900 px at 1100 px,
  its focused heading off screen, and at 334 to 353 px at 390 px.
  Now it is at 116 px in all four cases,
  and the test checks the card's place; without the fix it failed at 889.8 px.

The shoulds, in eleven points, done in the first rework:

- **`integration-v0`'s exemption could be faked** by a request,
  and no test held it: with the rule turned to `case false`,
  every test still passed (Different from the plan, 1).
- **"First" was decided outside the add's transaction:**
  in three runs, the page's first owner and `user add` 0 to 0.1 s apart,
  the command printed the line for a user that was not first,
  and the log held two lines.
- **A client gone at the wrong moment lost the line for good**
  (Different from the plan, 2).
- **Integration names reached the terminal raw,** escape sequences included.
- **The words could be untrue:**
  `OpenSetup` also lists what an owner since removed left,
  removing the last regular user opening the server again;
  the coordinating session chose words that hold both ways (For the completion
  gate, 4).
- **The toast:**
  after a sign-out within its 30 s it stayed,
  naming integrations and roles over the sign-in card,
  and its Show unfolded a hidden card;
  Show for the log left the focus on the page's body;
  at 390 px Show beside the text left it a column of about 190 px,
  one integration and one role 254 px tall in German, a long list 671 px,
  with × pushed above the window.
  The four-integration list is now 457 px tall at 390 px in German.
- **The command line's parts and the lists inside them** were both joined by ",
  ".
- **The log line's "by the command line"** read as a third name.
- **Roles were shown by their keys,** a renamed default role by its default
  name.
- **What no test held:**
  the answer's providers, `auto_create` and `capabilities_off`;
  serve's "(creates users)"; the command line's other parts;
  the stored line and the alert at a start and at a reload;
  a removed provider and a removed role not listed.
- **Documents:**
  `docs/ui.md` left out an integration whose key can still be collected;
  `docs/store.md` and the OpenAPI file said every role that is not a default one
  is listed, true only once the exemption went;
  the fragment said "the first owner".

Left as it is, for the completion gate (3):
the alert of the stored line goes to every reader.

Both readers checked the rework, and every point held.
Eight shoulds more, three of the server's reader and five of the page's,
done in a second rework:

- **`TestAddCommandUserSaysFirst` did not hold the transaction:**
  with the count moved back before it, the test passed.
  `TestAddCommandUserCountsInItsTransaction` now fails with the count moved
  (first true).
- **`user tokens` printed labels raw.**
- **`terminalText` kept format characters**
  such as U+202E and U+2066 to U+2069, which reorder what a terminal shows.
- **Show stood below the toast's visible part with a long list at 390 px,**
  24 integrations, the toast covering the window for 30 s;
  it is sticky at the toast's bottom now.
- **Words:**
  `docs/ui.md`'s regions' table lacked `log.focusHead`;
  the fragment's "before it" for a user;
  the sign-out test's docstring said the toast was hidden on the page before,
  where it stayed; the French "d'avant vous", now "depuis avant votre arrivée".

Nobody but the builder read the second rework;
each of its changes taken back fails the test that holds it.

## Seen and left

- **Serve's line and the command line name roles by their keys,**
  the page and the stored line by their names.
  As it is.
- **The commit's summary keeps the plan's "while the server was open";**
  its body and every text say the words that hold both ways.
  As it is.
- **Every toast goes at a sign-out,** a notification's too,
  which fits what `forgetSession` promises;
  the page's reader went through each toast there is,
  none of which should outlive the user who left.
  As it is.
- **A screen reader may not announce the toast:**
  its text is set while it is hidden, as for every toast before the block.
  Older than the block; nobody's yet.
- **A German capability's label broke inside a word** in the first tree's toast,
  "Kommunikationsstörung|en", in the narrower column of before the rework;
  not looked at since.
  As it is.
- **Italian, Dutch and Polish** read understandable, literal in places;
  marked machine-translated.
  As it is.
- **The phone-width check of `test_the_owner_is_told_what_was_set_up`** asks
  that Show sits in the text's column;
  sticky, it is below the text's end in every case but the long one.
  The test's, as it is.
- **One UI test failed once in the first whole suite,**
  `test_maint_updates.py`'s
  `test_an_event_keeps_a_question_and_takes_back_one_whose_reboot_waits`,
  "the reboot read ([])", and passed on its retry.
  Not the block's file; nobody's yet.
- **`TestEventSize` under `-race`:**
  For the completion gate, 5.
- **No race run after the first tree:**
  CI's Race job runs it on the pull request.
- **A capability's own label can hold a colon and commas**
  (German: "Räume und Geräte reparieren: Kommunikationsstörungen zurücksetzen,
  …"), inside the toast's list of capabilities turned off, which is joined by ",
  ": the list can read ambiguously there.
  The texts' next change.

## For the completion gate

1. **What a user notices:**
   the first owner the page makes is told,
   in a toast with an orange edge for 30 s,
   what was set up before them and still works, by name, with Show;
   `massimo user add` says it on a second line;
   the log keeps it as a warning in the header's log icon until it is marked
   seen; and the API's answer carries it.
   Nothing changes for a second owner or a server where nothing was set up.
2. **The log entry, the owner's own addition ("with a log entry").**
   In serve's log, for the first owner the page makes,
   as the server's reader saw it on the reworked tree,
   a role `integration-v0` made by its own request beside the changed viewer:

   ```
   level=INFO msg="first owner created" source=server user=anna remote=127.0.0.1
   level=WARN msg="the first regular user was made; what was set up before it still works: check it" source=system kind=open_setup integrations=Kitchen role=owner roles=viewer,integration-v0 user=anna
   ```

   The Info line is the one before the block (here from the builder's run on the
   first tree); providers and capabilities off come as `providers=` and
   `capabilities_off=` where there are any, a provider whose sign-in creates
   users as `cloud (creates users)`.
   The stored line's summary,
   which the log card and `GET /api/v1/log/alerts` read:

   ```
   {"integrations":[{"name":"Kitchen","uuid":"…"}],"role":"owner","roles":[{"key":"viewer"},{"key":"integration-v0","name":"integration (before 1.0)"}],"user":"anna"}
   ```

   The log card shows such a line at warn, among the server's lines,
   as `test_log_open_setup_line` holds it: "First owner anna;
   still in effect from before this user: 1 integration: Kitchen;
   roles added or changed: Viewer, Night shift".
   - **At a start with `MASSIMO_API_TOKEN`:**
     the same Warn line with `user=owner`,
     after `owner created from MASSIMO_API_TOKEN` (the builder's run on the
     first tree); at a reload, held by a test alone.
   - **By `massimo user add`:**
     no line in serve's log;
     the stored line with `"role":"viewer"` and `"via":"cli"`,
     shown as the test has it, "First user bob, Viewer, by the command line;
     still in effect from before this user: 2 integrations: Kitchen, Garage";
     and the command prints "still in effect from before this user:
     1 integration (Kitchen); roles added or changed: viewer, integration-v0;
     check them in System settings" under its line `user carla added …`.
   - **None** for a second owner, a second user the command adds,
     and the first owner of an open store where nothing was set up;
     two first owners at once give one.
3. **The alert goes to every reader of the log:**
   the stored line is visible at read and the alerts route needs `read`,
   with `log.seen` kept per user,
   so every later viewer or guest gets the alert in the header until they mark
   it seen, though only an admin can act on it.
   It shows nothing new: the `change` lines already name the same integrations
   and roles at read.
   Left as it is for the owner;
   the other way, a visibility of the line's own for those who may act on it.
4. **The words hold both ways,** the coordinating session's choice:
   "from before you" in the toast, "from before this user" in the log and the
   command line, since what is listed is also what an owner since removed left,
   removing the last regular user opening the server again.
   The plan's "while the server was open" stays where the documents describe
   the usual case, and in the commit's summary.
   The owner's to change.
5. **`TestEventSize` fails under `-race`, older than the block:**
   "the events writer did not answer".
   The server's reader, `MASSIMO_TEST_SCALE=2`,
   package runs interleaved at a load average of 8 to 24 from other sessions:

   | Run | Result |
   |---|---|
   | the base, the package | failed, failed, passed |
   | the block's tree, the package | failed, failed, passed |
   | the block's tree without `TestOpenSetup` | passed 3 of 3 |
   | the base, the test alone | passed 5 of 5 |
   | the block's tree, the test alone | passed 5 of 5 |

   Its flush gives up after `eventsQuiet`, 10 s (`internal/store/events.go`),
   which `MASSIMO_TEST_SCALE` does not scale;
   under race and load the 2000 events take longer.
   For the unsteady tests' list of "Fix flaky tests";
   the plan's race command for this block can fail on main too.
6. **D41 is untouched:**
   `mayManage` and the role rules are as they were,
   and nothing of the block waits for it.
7. **Departures the owner may want to change:**
   the line's "first regular user" and its `role` (D12-A);
   an integration whose key can still be collected listed;
   `integration-v0` listed as any role;
   every toast gone at a sign-out;
   the Cube search's Show under its line at phone width;
   German "Anzeigen" (Different from the plan, 1, 2, 6 and 7).
8. **The open mode stays,** as the plan's "What stays as it is" says:
   until the first regular user,
   every request from the local network may do everything;
   this block tells of it afterwards, it does not close it sooner.

## What verified it

- **The builder, on the first tree,** each by its exit code, 0:
  `make ui`, `gofmt -l .` printing nothing,
  `go vet` for linux, windows and darwin, `go run ./tools/apitypes -check`,
  `make cross-check`, `make ui-check`;
  `reflow.sh --check` on the four documents and the fragment, 0 errors;
  `commit-msg.sh`.
  The whole `go test ./...`: 37 packages ok,
  `cmd/massimo` failing `TestParallelOrListed` (the serial list),
  then ok after the fix.
  `MASSIMO_TEST_SCALE=2` with `-race`: `internal/server` ok (422 s);
  `internal/store` failed `TestEventSize` twice,
  and passed alone and in a third run (For the completion gate, 5).
  UI: `test_first_owner.py`, `test_keep_open.py`, `test_ways.py`,
  `test_log.py` and `test_parts_guard.py` in all four cases, 494 passed;
  the whole suite in 1100-en, 841 passed and 1 failed (Seen and left).
  The six new tests failed on the base, each for the reason it names,
  the case without `open_setup` passing there by design.
  A real `serve` on loopback, no Cube:
  the page's way, `user add`, a start with the token and a clean open store,
  the lines of For the completion gate, 2;
  every log, answer, output and stored line searched for the three keys,
  the four passwords and the token, no hit.
  The scripts: `tools/ui-test/run.sh`, `tools/app-emulator/servers.sh` and
  `tools/store-fixture/make.sh` send `user add`'s output to `/dev/null`,
  the packaging checks compare only the postinst's owner hint, unchanged,
  and nothing reads the answer of `POST /api/v1/users`.
- **The server's reader, on the first tree:**
  the store states and the eleven dumps above;
  a real `serve` on loopback for two first owners at once,
  `user add` beside a running serve and the page's first owner and `user add`
  started 0, 0.05 and 0.1 s apart; a probe test of `integration-v0`,
  and the rule turned to `case false`; the block's tests and `TestFirstOwner`,
  `TestFirstOwnerOnce`, `TestLogKinds`, `TestSettleRulesKnowTheirKinds`,
  `TestOpenWarningAtTheStart`, `TestKeepOpenGoesWithTheFirstOwner`,
  `TestUserCommands`, `TestUserCommandsLog` and `TestParallelOrListed`, ok;
  the race runs of For the completion gate, 5;
  every output searched for secrets; `gofmt` on the changed files.
  It did not read the page, nor run the whole Go suite, the vets,
  `make cross` or a UI test.
- **The page's reader, on the first tree:**
  Show, its focus, the sign-out and the toast's heights measured in all four
  cases; the six languages read; the four Go tests on the tree, ok,
  and on the base, failing for their reasons; the new UI tests,
  `test_create_the_owner_from_the_card`,
  `test_log_server_lines` and `test_parts_guard.py` in all four cases,
  90 passed; the new tests on the base,
  12 failed for their reasons and 4 passed, the case without `open_setup`,
  by design; `reflow.sh --check` on the four documents and the fragment,
  and `commit-msg.sh`, 0.
  It did not run `make ui-check`, the whole suite, race or `make cross`.
- **The builder, after the first rework,** each by its exit code, 0:
  `make ui`, `make ui-check`, `gofmt`, the three vets, `apitypes -check`,
  `go test -count=1` of `internal/store`, `internal/server` and `cmd/massimo`,
  `make cross`, `reflow.sh --check` on `docs/ui.md`, `docs/api-v1.md`,
  `docs/store.md`, `README.md` and the fragment, `commit-msg.sh`.
  UI: the five files in all four cases, 502 passed;
  the whole suite in 1100-en, 844 passed.
  Nine Go mutations, each failing the test it names;
  on a copy without the page's changes, the eight new UI cases failing.
- **The server's reader, on the reworked tree:**
  a real `serve` on loopback: `user add` beside it,
  and the page's first owner and `user add` started 0 to 0.3 s apart,
  six runs, the command printing one line and the log holding one line in each;
  a role `integration-v0` made by a request, listed by name;
  the eleven dumps again;
  `make ui`, `go test -count=1 ./internal/store`,
  `cmd/massimo` with `-run User|OpenSetup|Parallel`,
  the server's open-setup, first-owner, log-kind, settle-rule and OpenAPI tests
  and `TestOpenWarningAtTheStart`, `tools/apitypes`, `gofmt`,
  and `go vet` of the store and the command, each 0;
  the count moved back before the transaction, the test still passing.
  It did not run the whole server suite, race, the windows and darwin vets,
  `make cross` or the UI.
- **The page's reader, on the reworked tree:**
  the card's top at 116 px for 3 s and the log's title in focus by Enter,
  in all four cases; the toast's layout in light and dark at 390 and 1100 px,
  24 integrations capped from 16 to 884 px, no horizontal scroll,
  the Cube search's and a notification's toast; `test_discover.py`,
  `test_battery_words.py`, `test_notifications.py`, `test_sign_out_late.py`,
  `test_sign_in_updates.py` and `test_connect.py` in all four cases, 336 passed;
  on a copy with the previous `Owner.tsx`, `style.css` and `login.js`,
  16 failed for their reasons and 4 passed, by design;
  the new tests and its own checks on the tree, 76 passed; `TestUI*`,
  `TestLogKinds`, `TestOpenAPICoversRoutes` and the block's Go tests;
  `tsc` and Biome; `reflow.sh --check` and `commit-msg.sh`.
- **The builder, after the second rework,** each by its exit code, 0:
  `make ui`, `make ui-check`, `gofmt`, the three vets,
  `go test -count=1` of `internal/store` and `cmd/massimo`,
  `reflow.sh --check` on `docs/ui.md` and the fragment, `commit-msg.sh`.
  UI: `test_first_owner.py`,
  `test_log.py` and `test_parts_guard.py` in all four cases,
  first 332 passed and 2 failed,
  the long case at 390 px on its own test's check of Show under the text,
  which the sticky Show no longer met; the check fixed, 334 passed.
  Each change taken back fails its test:
  the transaction test with the count moved, first true;
  the long case without the sticky rule,
  Show at 962 px in a toast ending at 884 px.
- **The coordinating session, on the commit:**
  the step carried from Block 2's commit onto main's head,
  "chore(aps): +NOTE MobileApp v1.4, phone check blocks 15 to 17",
  by a three-way merge of trees without conflict;
  main had meanwhile taken AP CapabilityTable (#457), RoomsPage's Block 7
  (#454) and Dependabot's `golang.org/x/net` bump (#441).
  In a clone with the tree: `commit-msg.sh`, 0 errors;
  `changelog.sh check --staged --strict`, 0;
  `GUIDELINES/shared/tools/check` over the range, twelve checks all well.
  `make ui`, the Go tests of `internal/server`,
  `internal/store` and `cmd/massimo`, and the whole UI suite in 1100-en,
  on the committed tree, after make ui: go test -count=1 of internal/server,
  internal/store and cmd/massimo, ok, and the whole UI suite in 1100-en,
  869 passed, exit 0.
- **Not run here:**
  `make lan-test`, nothing here touching discovery;
  `make app-emulator-test`, which CI's job Android emulator runs;
  the whole `go test ./...` and the race runs on the committed tree,
  which are the pull request's CI.
