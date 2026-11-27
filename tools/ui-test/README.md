# Web UI tests and screenshots

`make ui-test` runs `run.sh`.
The script:

1. builds massimo, the page's parts first (`make ui`);
2. starts a server set for each of four pytest-xdist workers
   (`UI_TEST_WORKERS`, default 4), each set:
   the fake Cube (`massimo fakecube`), which also answers discovery
   and has a device that joins pairing mode (`-pair 0a010d,0abcde,TEQ0000001`:
   a copy of the radiator thermostat 0a010d, two seconds after `n:`,
   again after it was removed), with `greeting.txt`,
   which is the protocol fixture from internal/protocol/cube/testdata plus an
   `L:` line with device states (measured temperatures included),
   a window contact, "Fenster Kind 1", that reports an open window,
   and an eco button, "Eco-Taster Kind 2" (0a010e, `test_rooms.py`),
   each with its `C:` line as a Cube sends one for every device;
   `massimo serve` against the fake Cube,
   with the API token `ui-test-token`;
   the Cube comes from `MASSIMO_CUBE_KEQ0000100_ADDRESS` and `_NAME` ("Fake Cube"),
   which the server seeds into its store,
   `massimo.db` in the set's temporary directory;
   and the second server, of a Cube that is not connected (`test_home.py`);
3. waits until every program of every set says it listens on its own port
   and every server is connected to its fake Cube,
   and stops if one of them has stopped meanwhile:
   a server of another run on one of the ports would answer the same token;
   for the suite, it then takes a backup of each set's Cube by hand:
   a Cube without a backup gets one by itself a minute after the start,
   and one with a backup gets none during the run while its configuration
   stays as it was (an automatic backup waits 60 minutes after a change);
4. runs the pytest suite in this directory with `pytest-playwright` (Chromium),
   in the workers, a file's tests together in one worker (`--dist loadfile`),
   handing worker `gw<n>` set n's page address in `UI_TEST_URL`,
   the token in `UI_TEST_TOKEN` and the server's log in `UI_TEST_SERVE_LOG`
   (`conftest.py` takes them from `UI_TEST_URL_<n>` and the others).

Set 0 runs on loopback ports of its own:
the fake Cube on 62919 (`UI_TEST_CUBE_PORT`) with its discovery on 23919
(`UI_TEST_DISCOVERY_PORT`),
the server on 8119 (`UI_TEST_API_PORT`)
and the second server on 8120 (`UI_TEST_DOWN_PORT`, the server's + 1).
Set n takes the fake Cube's two ports + n and the servers' two + 2n,
so another session running the suite at the same time needs ports further apart
than twice its workers.
The programs run in empty temporary directories without the caller's environment,
so a `.env` or `MASSIMO_*` variables can't point them at real hardware.
All are stopped at the end.

`UI_TEST_WORKERS=1` runs the suite in one process against one set, as before
the workers; so does a machine without pytest-xdist, which `run.sh` says.

When Playwright, pytest or pytest-playwright is missing,
`make ui-test` says so and exits 0;
with `UI_TEST_REQUIRED=1`, which CI sets, it exits 1 instead,
as it does without pytest-xdist and more than one worker.

## Running all, one file, one test

```sh
make ui-test                                          # the whole suite
PYTEST_ADDOPTS='-k cubes' make ui-test                # the tests whose names match
PYTEST_ADDOPTS='-k "hidden and 390-de"' make ui-test  # one test in one case
UI_TEST_CASES=390-en,390-de make ui-test              # some cases; a CI job runs one
tools/ui-test/run.sh smoke test_log.py                # one file
tools/ui-test/run.sh smoke 'test_log.py::test_log_changes[1100-en-chromium]' -x
UI_TEST_WORKERS=1 make ui-test                        # in one process
KEEP=1 make ui-test                                   # the work directory kept
UI_TEST_SCALE=2 make ui-test                          # every wait twice as long
PYTEST_ADDOPTS='test_parts_guard.py' make ui-test    # the parts' guard alone
PYTEST_ADDOPTS='test_parts_guard.py -k rooms' make ui-test  # the guard of one region
```

`PYTEST_ADDOPTS` is pytest's own;
`run.sh smoke` hands the arguments after it to pytest,
whose working directory is `tools/ui-test/`, so files are named from there.
A test's id carries its case: `1100-en`, `390-en`, `1100-de`, `390-de`,
then the browser.

pytest-playwright keeps a failed test's screenshot in `tools/ui-test/results/`,
one directory per test (gitignored).
`run.sh` empties `results/` at the start of every run of the suite,
so that what it holds is always the last run's,
also of one that stopped before its first test,
and a passing run leaves it empty (AP CleanRuns, D3-A).
A run whose tests failed also keeps each set's `serve.log` and `fakecube.log`
in `tools/ui-test/results/logs/set<n>/`, copied at its end, after the retry,
so that they hold both passes,
before its cleanup removes the temporary directory that held them;
a passing run leaves none.
They say what the servers did meanwhile, a reconnect of their own among it,
which a screenshot does not show (AP ServerTraffic, D4-A),
and hold only the suite's test data:
the server redacts keys and tokens in its log.
The first pass records no trace and no video:
the trace's DOM snapshots make every step slower,
840 ms instead of 14 for a step in System settings at 1100 px,
and slow every wait with it;
the video took a quarter of a second of every test's set-up and tear-down:
1100-en took 250 s with it and 218 without, in four workers,
on 2026-11-19 (AP TestSpeed, D8-A).
When tests fail, `run.sh` runs them once more with both
(`--last-failed --video retain-on-failure --tracing retain-on-failure`,
the same arguments), at four times the scale (below),
and keeps the screenshot, the video and the trace of each that fails again
in `tools/ui-test/results/retry/`;
it says whether they failed again, and the run fails either way:
a test that passes only the second time is flaky, not fixed.
`PYTEST_ADDOPTS='--video retain-on-failure' make ui-test`
records the first pass's videos as well.
The first pass ends with its 20 slowest phases,
each a test's set-up, call or tear-down (`--durations=20`),
in CI's UI jobs' logs too, so that a run shows where its time went;
`PYTEST_ADDOPTS='--durations=0'` lists them all;
the retry lists none of its own.
`python -m playwright show-trace <trace.zip>` replays a trace step by step,
with the page's DOM, console and network at every step.

`UI_TEST_SCALE` stretches every wait of the suite:
each wait of `helpers.py`, and the page fixture's,
lasts its seconds times the scale,
a positive number, 1 where it is not set,
as `MASSIMO_TEST_SCALE` does for the Go tests.
A slow or loaded machine sets more;
`run.sh` refuses a value that is not a positive number.
Its retry runs at four times the scale given,
since the trace and the video slow the page as well:
a test's set-up, which loads the page, took up to 3.8 times as long
with them as without, a test's call twice as long at the median
(1100-en, three files, 2026-11-26).
A window that waits for nothing to come is not scaled
(`docs/ui-testing.md`, section 9, "The scale").

The run's sets, its `massimo` and the servers' logs and stores
are in its work directory,
`massimo-ui-test.XXXXXX` under `TMPDIR` (`/tmp` where it is not set),
which holds the run's PID in `pid` and goes at the run's end.
A run that was killed, whose cleanup did not run, leaves its own behind;
a later run removes each of the user's whose run is gone
and that is older than a day,
never one in use, so that another checkout's run keeps its own.
`KEEP=1` keeps the run's work directory, passed or failed,
with a file `kept` in it from the start, which the later runs leave alone,
and prints its path; it is then removed by hand (AP CleanRuns, D4-A).
`tools/workdir/workdir.sh`, which the emulator suite shares,
makes, sweeps and keeps it (`tools/workdir/README.md`; AP EmulatorRuns, D3-B).

## The fixtures

`conftest.py` holds them:

- `case`:
  every test runs four times, at 1100 and 390 px,
  in English and in German (from the browser context's locale);
- the mark `cases`, with case ids such as `1100-en`:
  the test runs in those cases alone,
  and is deselected in the others as the suite is collected,
  before a fixture loads a page for it (AP TestSpeed, D9-A),
  as `test_parts_guard.py` is;
  an id that is not a case fails the marked test in every case;
- `page`:
  a browser context of its own per test and case (pytest-playwright's `context`,
  with `browser_context_args` set from `case`),
  the token put into `localStorage` before the page loads,
  the page loaded with its room tiles and its event stream open;
  it waits 30 s for the tiles (`helpers.wait_started`, scaled),
  longer than the page's own limit of a read, 20 s,
  so that a late read fails as the page fails it;
  on a timeout it names the API reads still open and the page's message line;
- `watch`:
  what the page did, for a test or the guard to look at: the stream requests,
  the used ticket handed out, console and page errors,
  and in `expected` the refusals a test causes on purpose (the URL's end, the status);
- `guard` (autouse): a test fails on a console error, a page error,
  or a horizontal scroll left at its end;
  the refused stream of the stream test, the stubbed 503 of the discovery
  and what a test lists in `watch.expected` are expected;
- the marker `used_ticket`: the page's first stream gets a used ticket,
  which the server refuses with 401,
  and the page is handed over before the stream is open again;
- the marker `clock`: Playwright's clock installed before the page's scripts,
  its time flowing as the real one, which the test moves on (`page.clock.run_for`,
  `page.clock.fast_forward`);
  a test sets the page's wall time with `page.clock.set_system_time`
  and an aware `datetime`, which moves `Date` alone and runs no timer
  (a float would be taken as seconds and multiplied by 1000, as with `pause_at`);
  a test pauses it with `helpers.pause_clock(page)` alone,
  at the page's now and 50 ms on
  (and further on where that is past once the call arrives, as under a trace):
  at the pause Playwright runs once every timer due up to its time, a pending
  animation frame among them, and after it no timer and no frame of the page's
  comes until the test runs it, so that nothing due later than the pause has run;
  the helper hands Python's `pause_at` seconds, which it takes,
  where the page's `Date.now()` is milliseconds:
  given those, the clock jumped millennia ahead and fired every timer of the
  page's at once, its poll and its stream's renewal among them;
  a test module that calls `clock.pause_at` itself fails its collection
  with the helper's name (`conftest.py`),
  where the call is written `….clock.pause_at(`, not through another name for the clock;
- the mark `browser_context_args`, pytest-playwright's own:
  the test's context gets those arguments on top of the case's viewport and locale,
  `timezone_id` for a page in a zone of the test's own
  (`test_room_updates.py`, in `Europe/Zurich`);
- `failed_checks_forgotten` (autouse): after a test that opened a context
  without the token (`new_context`), one request with the token forgets the
  failed token checks its pages counted against 127.0.0.1;
  after every test, the set's store (`massimo.db` beside `UI_TEST_SERVE_LOG`)
  has none counted against an address, or the test errors at teardown;
- `reads`: the page's own reads answered with the server's data and what
  the test put in (`helpers.Reads`), so that the page's poll and the reads
  that events and actions start keep it:
  `reads.cube(serial, **fields)`, `reads.found(cube)`,
  `reads.change(pattern, fn)` and `reads.clear(pattern)`;
- `stream`: the page's event stream as the page took it, from a mark
  (`helpers.Stream`): `stream.mark()`, `stream.wait(what, kind, …)`,
  which on a timeout says what came since the mark, and `stream.since(mark)`.

The page fixture puts `helpers.PAGE_KIT` into the page before its own
scripts: it records the events the page's listeners take, for `stream`,
and lets `isolate_log` and `wait_log_live` work on the log;
the page's code is not changed.
`docs/ui-testing.md`, section 9, says when a test needs which.

`fake_bridge.js` is the app's bridge for `test_connect.py`,
`window.massimo` as docs/app.md describes it,
recording every call in `window.__fakeBridge.calls`.

`helpers.py` holds the texts the tests expect in both languages,
the stubbed answers, `wait_until`, `named_buttons` and `found_text`,
`scroll_settled`, which waits for a smooth scroll to end
before a test scrolls on its own
(Chromium's compositor can undo an instant scroll sent during it),
`open_user_menu` and `sign_out` for the header's 👤 menu,
and `show_view`, which opens a page as a user does:
Rooms, the log and System settings with their header button,
or the log and System settings from ☰ on a narrow screen,
My profile and My settings from the 👤 menu.
It also holds what the fixtures `reads` and `stream` are made of,
and `isolate_log` (the log holds the test's own lines alone),
`wait_log_live` (the log takes the stream's state events: in place of a
fixed wait of 1.6 s), `safe_route` (the one route handler that lets go
a request the page no longer waits for, and reports to the guard any
other error and a request it had to abort), `undo` (a cleanup's step
that says what it could not undo) and `cleanup`
(every step of a cleanup runs, the first error raised at the end).

The four cases share the one server of a worker,
so a test that changes the server's store puts it back;
what a test stores in the browser goes with its context.
So do the files of a worker, in whatever order the scheduler gives them,
and the API's lockout is the server's too:
a page without the token counts a failed token check with every request,
and ten within a minute, a wrong password among them,
lock the address out for five minutes (429),
hence `failed_checks_forgotten`.

## What the tests check

- `test_rooms.py`:
  the page loads with room tiles and the right language;
  a tile's device list and its configuration open,
  every configuration tab shows, and the configuration closes;
  the room with the open window shows "Open" on its tile, and only that room,
  and the badge goes when a state event closes the window;
  the measured temperature shows its age;
  after a state event makes the first room's thermostat 30 hours old and stale,
  the tile warns, the temperature is greyed,
  and the device list shows the stale badge and the values' ages,
  with the type, the radio address, the exact times
  and the server's start time in expert mode;
  a state event while a device's name is typed in the configuration keeps every key,
  and the tile is the one it was, also once the focus has left;
  a switch point's time typed part by part keeps the focus,
  marks the day as changed, and Send right after it sends the time (stubbed).
- `test_header.py`:
  the Log button's symbol is a sheet, taller than wide;
  the 👤 menu names the owner by the suite's key and offers Forget key,
  fits the window, and closes on Escape, on a click outside and on a choice,
  My settings opening the settings;
  on an open server (`/me` stubbed) it says so, without Sign out;
  a room's ⋯ and 👤 are one open at a time;
  each page puts its name into the address, back and forward move between
  them, and a reload opens the log and My profile again with the header in view;
  System settings in a viewer's address (`/me` stubbed) shows Rooms;
  at 1100 px the header is one line in its order,
  the brand, the time with ↻, the connection, the Cube choice, 🏠 👤 📄 ⚙;
  at 390 px 🏠 👤 ☰ with the Cube choice under them,
  and ☰ opens the log and System settings and is highlighted on them.
- `test_home.py`:
  Rooms on the second server, whose Cube sits at a closed port,
  says that the Cube is not connected, the address the server tries,
  that no data has come and what to check, and its button opens
  System settings → Cubes;
  with the Cube list stubbed, it says that no Cube is configured,
  and for a Cube that dropped after its rooms loaded it stands above the rooms,
  with when the last data came.
- `test_header_updates.py`, the header as parts (AP UiParts, Block 6),
  the Cubes' list stubbed:
  every Cube's problem list, open, stays the same element and open through
  a poll and lists the device with a problem the poll brought;
  the Cube choice, open, stays open with the focus on the chosen Cube
  and shows the Cube's new name and the settings' cog the poll brought.
- `test_room_updates.py`, the room tiles as parts (AP UiParts, Block 7),
  each case of the fresh look's probe a test, with a state event or a
  reload of the rooms between: More, ⋯ and a problem list stay open;
  the focus stays on +, also after Enter on it, and on a device's Lock;
  a text selected stays selected; a failed write's result line stays;
  the holiday form's temperature typed and its focus stay, also through a
  reload; the device list stays open; the virtual thermostat's address
  typed stays; a name typed in the configuration stays, also once the
  focus has left; a click on + counts with a state event or a reload
  between its press and its release, and from the configuration; the tile
  is the element it was; a new setpoint is shown;
  in the House panel a room unticked and the focus on its checkbox stay
  through a reload of the rooms.
  Seventeen of them failed on the page before the move.
  With them are the House tile's tests: its name, the Cube's;
  its form's one choice (AP RoomsPage v1.0, Block 7), what each choice
  offers and its defaults, the date hidden while Permanently,
  a typed temperature and end kept, a past end forgotten;
  its lowest and highest; a room renamed or removed followed;
  and tomorrow morning, the default end of the House form's Until and of
  a room's holiday form, on the evening before the spring change of the
  clocks and just after midnight on the day of the autumn one,
  the page in `Europe/Zurich` under Playwright's clock.
- `test_pairing_updates.py`, a Cube's pairing as a part (AP UiParts, Block 9),
  `GET …/pairing` answered as running with a device that joined:
  the joined device's form, its name and room, the countdown and Stop
  stay the elements they were, with what is typed and the focus,
  through the countdown, a pairing event, another device that joins
  and a reload of the Cubes;
  on Cube settings the time typed and its focus stay through a pairing event and a reload.
  Both failed on the page before the move.
  After a sign-out, and for another user signed in after an expired session,
  the joined device's form holds nothing typed, no request of the pairing goes
  while the sign-in is asked, and the Cube's rooms are asked for again
  before the form offers any.
- `test_repair_updates.py`, the repairs as parts (AP UiParts, Block 9),
  the repairs' routes stubbed with a preview:
  a device's box, its listing's name typed and its question open,
  with the focus in the name, stays the elements it was with Repair…
  through a state event, a reload of the rooms and a rename of the device;
  the Cube's box with its question open stays, the focus on No,
  through a rename of the Cube by a poll;
  a box with its question open is gone after a sign-out
  and for another user signed in after an expired session.
  Both failed on the page before the move.
- `test_admin_updates.py`, System settings' lists as parts (AP UiParts, Block 10,
  with AP UnsavedMarks, Block 5):
  a user's, a role's, a provider's and an integration's fields typed and not saved,
  and the new item's forms, stay the elements they were with what is typed
  (and the focus, through a reload) through a save in another panel,
  the list read again and System settings shown again;
  the form left, the section's pill shows its disk, which opens the panel with the
  focus in the form, ☰ the mark and its System settings the disk, red after a failed
  save and the worse of it and the log's warning; Reset clears them;
  a user gone from the server forgets its form;
  a new user's, role's and provider's form takes the focus in its first field as it
  is drawn, and what is typed at once in a later field stays there, under the paused
  clock;
  a capability refused for every role shows as stored;
  a Cube settings draft marks ☰ and its Cube settings (D24);
  a pill's item unfolds a folded card and puts the focus in the form;
  a user's, a role's and a provider's Remove asking stays and removes that item alone;
  a key made is gone at the next read;
  a capability's switch keeps the focus while it saves;
  an identity's Change closes when saved and is forgotten with the identity;
  an approval is followed across pages and not while its card is folded;
  "Integrations may ask" keeps the focus; the integrations are forgotten at a sign-out
  and for another user after a 401, and a list that answers late shows nothing;
  a running restore stays in sight, a plan goes with its backup, a plan that answers
  late shows nothing, and the backups are forgotten for another user after a 401;
  a backup's Delete asking and the format chosen stay, and a restore shown on
  Cube settings shows in Maintenance's fold, Cube backups';
  after a sign-out, and for another user signed in after an expired session,
  nothing typed, no list, no key made and no restore shown is left.
  The users, roles, providers, integrations and backups it makes it removes.
  The server takes ten registration requests of integrations per address in
  ten minutes: each case makes two real ones, `test_integrations.py`'s register
  and link and this file's approval test, and the other integrations tests
  answer their reads with stubs; the four cases of a worker make eight, so a
  local run of all four cases repeated within ten minutes meets the limit
  (429) until the ten minutes have passed or the servers are started anew.
- `test_maint_updates.py`, Maintenance and System info as parts (AP UiParts, Block 12),
  `GET /api/v1/update` stubbed, nothing installed, restarted or rebooted:
  Software update keeps a line selected, Update's question and its buttons
  through the header's check and Maintenance shown again;
  a newer release takes the question back; the channel stays on and keeps
  the focus while it saves; a sign-out, and another user signed in after an
  expired session, end the reading while the helper works and take the
  question back, and a reading that answers late starts nothing;
  while Maintenance is shown, a resync of the stream shows the newer release,
  Rooms shown reading nothing, a burst of events reads once and once after it,
  a burst left by its page makes no second read, an update asked for elsewhere
  is followed with its grace and takes the question back, a refused request
  leaves the reading its buttons, an old update event starts no reading,
  the helper's reading takes no event's read, the
  channel's own change event reads nothing behind its back, a question taken
  back leaves a focus elsewhere where it was, the next user is followed again,
  and a read asked for before the session ended is not made;
  an interval stays on and keeps the focus while it saves;
  the delay typed and the restart's question stay the elements they were
  through Maintenance shown again; the restart's question goes where the
  restart is no longer offered, the reboot's where a reboot waits; the session's
  end ends the following of a restart, a /healthz that answers late saying
  nothing, and forgets the delay typed and the questions;
  a setting set elsewhere shows while Maintenance is shown, Rooms reading
  nothing; a resync reads the restart's row once, nothing for Rooms, nothing
  while this page follows its restart; an interval's own change event reads
  nothing behind its back; another setting changed during the own PUT is read
  after it;
  the settings file read again shows what waits with nothing read;
  a read at warn reads what the next start refuses, one at info takes it away
  with nothing read;
  a reboot event keeps this page's wait, which follows at its time; an event
  keeps the restart's question and the delay, and takes the reboot's back
  where a reboot waits;
  System info keeps a text selected through a certificate's reading
  and a copy's "Copied", its folds and its copy icons through the page shown
  again; the session's end forgets its facts and the settings in effect, and a
  reading that answers late shows nothing; a resync reads the facts and the
  settings in effect once while it is shown, nothing for Rooms; a Cube changed
  reads the facts; a resync that finds the certificate being read reads it
  anew, also after the readings gave up; the session's end between two
  readings reads no more.
- `test_personal_updates.py`, Personal settings as parts (AP UiParts, Block 13,
  with AP UnsavedMarks, Block 5):
  My profile's display name and a new password typed stay the elements they
  were with what is typed through the personal page shown again, nothing sent;
  the display name left unsaved shows the card's pill, which unfolds the card
  with the focus in the form, ☰'s mark and its Personal settings' disk, and
  Reset clears them; a password typed marks its form changed and Reset empties
  it; after a sign-out, and for another user signed in after an expired
  session, nothing typed is left, and a read of the user that answers late
  draws nothing; in the app (fake_bridge.js) a linked account is named by its
  provider's name, and a current password filled in alone is no change;
  My settings' poll being typed stays, the element it was with the focus,
  when the user's settings arrive, and is saved as it is left;
  the scheme editor's name and a hex value half typed stay with the focus
  when the settings arrive, and Close drops the draft; a colour changed and
  left shows My settings' pill, which brings the editor back into sight from
  a folded card, ☰'s mark and Personal settings' disk, and Reset clears them;
  after a sign-out, and for another user after a 401, no scheme is being
  edited; Save as and Enter in the name save only a change (D204), and
  another scheme chosen or an import drops the draft;
  a notification's delay refused on its field stays, the element it was with
  its mark and the focus, and the name of this device typed too, through the
  personal page shown again, nothing sent; after a sign-out, and for another
  user after a 401, the name typed is gone, and a read of the devices that
  answers late draws nothing; a rule changed just before Sign out is sent
  once, under the owner's token, and nothing for the operator after it
  (D205), and one changed before another user's sign-in after a 401 is not
  sent; a read that finds a change waiting keeps it and sends it; the rules
  are off until the first read has answered.
- `test_data_updates.py`, the page's data as signals (AP UiParts, Block 15):
  on every page with a row, the log and Rooms, each data source applied once
  (the poll, the loads, the reload, a state event per device, the log's
  sign, the ages' minute, who is in, the settings and the language applied
  again) reads under `/api/v1/` what it read on the page before the data
  became signals, path for path: no part reads again because the data it
  draws from were written.
- `test_shell_updates.py`, the page's chrome as parts (AP UiParts, Block 14):
  every page with a row and Rooms, shown in turn twice with the poll held,
  read under `/api/v1/` what the page before the pages' store read at each
  showing, path for path, and nothing more at a change of page;
  Personal settings shown while a sign-out waits for the signed-out
  language's table reads nothing of My profile, and the page ends signed out;
  in that wait the rooms are gone, the stream closed, the poll stopped, the
  personal row says nobody and the tiles are shown (D211);
  the personal page's sections menu, an item in focus, stays the element it
  was through who is in read again and the language applied again; after
  changes of page in both orders of the rows, the cards' scroll margin is
  the shown row's height and an item of the sections menu brings its card's
  head to rest below the row.
- `test_sign_in_updates.py`, the sign-in as parts (AP UiParts, Block 14):
  in the app, Create the owner turns until the app has answered whether it
  kept the login and the card has closed, and a second submit meanwhile
  creates nothing; a name and
  a password typed, the focus, a refusal and the way chosen stay, the same
  elements, through a second 401, the providers read again, a change of
  language and who is in read again; the fields are emptied at an opening
  alone, never at the closing; the fingerprint's button keeps the user's
  name at a change of language, and stays the same element, turning, while
  the system's prompt is open and the status and the language change; the
  fingerprint's button is off while the password signs in, and from its
  press until its answer the card is working, both ways' Sign in, both
  Cancels and Select server off; a fingerprint's session that comes once
  the page signed out behind the card is ended at the server and kept by
  nobody, and a late refusal says nothing in a later opening, the app's
  status read again (AP AccessEnds, Block 1).
- `test_sign_out_late.py`, an answer that comes after a sign-out (AP
  UiParts, Block 14, D212): the search for Cubes, the House's writes to
  every room and in Eco, a pairing's Start, Cube settings' Save, a device
  removed and one put into a room, and a room's copy to others, each held,
  the session ended by a sign-out or as expired, the operator signed in,
  then answered: nothing of it is drawn, said, sent or started.
- `test_stream.py`:
  the event stream opens with tickets only, never with the token,
  and after a refused stream (the test hands out a used ticket first,
  which the server answers with 401) the page opens it again with a new ticket;
  the events that make the page load again leave no failed load uncaught.
- `test_discover.py`:
  the Cube choice's "Search for new Cubes…" shows "Searching…" until the server answers,
  then lists the Cube the fake Cube announces in discovery (KEQ0000000,
  not configured) as not selectable with its DHCP flag,
  with its env lines under the header and its `cube_found` in the log;
  an injected `cube_found` event adds another one live;
  a refused search (a stubbed 503) shows the server's message.
  The fake Cube answers discovery on loopback (`-discovery`,
  `UI_TEST_DISCOVERY_PORT`, 23919),
  and the server asks it there (`-discover-target`)
  from a discovery socket on a free loopback port (`-discover-listen 127.0.0.1:0`),
  so no set needs UDP 23272, which another program may hold.
- `test_cubes.py`, System settings → Cubes:
  a new name is saved and shown in the Cube choice,
  the settings file's name is offered as a differing line and "Take the file's
  value" brings it back;
  hiding the devices of the last room takes its tile away and showing them again
  brings it back; the found Cube KEQ0000000, its DHCP flag shown,
  is added and removed again (the test searches first where nothing was found yet),
  Remove asks in the panel,
  and Keep takes the question back;
  a redraw of the open panel asks for nothing again,
  so the focus stays in the pairing form being typed in
  (the test holds a request for the pairing until it has typed);
  Hide and Show again keep the name and the proxy address typed in the form.
- `test_settings.py`:
  My settings and System settings open, each alone, with their parts;
  with the Cube's panel open in System settings,
  English and then German chosen in My settings redraw its labels,
  cloned from a template, in that language, and the panel stays open;
  French chosen and German before French has come,
  French failing afterwards shows no error on the German page;
  My profile's Save is off until the display name differs,
  turns while it is sent (the test holds the PUT) and stores it;
  My profile lists the own keys, the suite's among them,
  and Create key and Revoke work there;
  a viewer (`/me` stubbed) has no ⚙, and System settings open as it signs in
  gives way to Rooms.
- `test_backup.py`, Cube settings → Backup and restore and Maintenance → Cube backups
  against the fake Cube's store:
  the Cube's backup downloads as YAML,
  one taken by hand shows in the list as "by hand" and downloads,
  its plan says the Cube holds it already,
  it compares with the one before when there is one, and it is deleted again;
  a role with `backup` alone (`/me` stubbed) has Maintenance with Cube backups
  and no System settings.
- `test_users.py`, System settings → Users and Roles as the owner:
  the owner and the operator the runner creates (`massimo user add operator`,
  password `op-password`) are listed with the operator's tokens,
  and the six default roles with the operator's seven capabilities;
  in the operator's open panel, Create key shows the new key,
  loads the tokens once and lists each once, and sends one POST per click,
  and the test revokes its keys again;
  a guest made with an end date that has seconds is saved without the end date
  when its display name alone changes, and Save without a change sends nothing;
  a user's Remove asks in the panel, Keep gives the focus back to Remove,
  and a double click on Yes sends one DELETE and says it worked;
  a Revoke the server never answers turns until the page's limit, under
  Playwright's clock, and then says that no answer came;
  every row of the sessions and keys shows its id, inside its list at
  390 px in six languages, and so do My profile's.
- `test_login.py`, in a fresh browser context without the stored token:
  the sign-in dialog opens in the middle of the window, its title centred,
  refuses a wrong password, signs in as the operator,
  who sees the tiles and Maintenance's Cube backups but no System settings,
  and is named in the 👤 menu, whose Sign out brings the dialog back and
  forgets the token;
  a sign-in at the page's start, and again after Sign out,
  starts the page once: one stream ticket and one read of `/me` each;
  the row "Password | API key" with the card's ☰ at its end is on one line,
  each button's text on one line,
  its group named "Sign in with" for a screen reader alone;
  a browser has no Cancel;
  a sign-in on a running page that replaces the session held signs that
  session out at the server with its own token, and a key replaced is not
  (`test_a_replaced_session_ends`); a tab whose session another tab's
  sign-in replaced leaves that tab's stored token alone at its 401;
  an answer of the password's way or of the API key's that comes once
  another way has signed in or set its token, once the card was closed and
  opened anew, or once the page signed out behind the card, makes no
  token, says nothing and takes no focus, a late password's session
  signed out at the server, and Sign in is ready in a card opened anew
  (AP AccessEnds, Block 1);
  My profile opened at `#profile?key=<id>` (`test_profile_marks_a_key`)
  marks that key and scrolls a row below the first screen into view,
  marks nothing for an id that is not one of the user's keys,
  and nothing once the address or the page shown changes.
- `test_password_eye.py`, the password fields' eye (AP MobileApp, D84-A):
  the sign-in's password, the first owner's two, My profile's three and the
  new user's each have it, which turns the field to text and back with the
  value, the caret and the focus kept, by a click and by the keyboard,
  labelled Show password and Hide password and pressed while it shows;
  an API key's field has none; the password hidden again at the submit, at
  the card's next opening and when My profile's fold or a user's panel
  opens again, the field drawn, in a browser and in the app;
  System settings → Users' switch writes `auth.password_reveal` (stubbed)
  and the page's fields follow at once, a password shown before hidden and
  staying so when the eye comes back; no eye while `/healthz` is held, and
  none once it says off, the browser's own hidden on every field.
- `test_sign_in_resend.py`, a request that meets an expired session (AP
  SignInResend), its 401 answered by a route:
  a write is never sent again after the sign-in, whoever signs in,
  it fails at once and says so where its result shows, also once the page
  is back, a setpoint, a backup's file, a Cube's and a role's Save, a
  settings change on the page's message line and a deletion among them;
  a read is sent again, and the stream's ticket waits for the sign-in;
  a stepped setpoint is dropped as the sign-in opens; another user sees and
  sends nothing of the first one's; and a read sent while the card is open,
  its 401 held back until the sign-in has answered and the card has closed,
  opens no second card and is sent again with the new session, the only
  one the sign-in left at the server (AP MobileApp, F36), and a write so
  held, by `api()` and by `apiRaw`, fails as not sent, no card opening.
- `test_first_owner.py`, the first owner in the sign-in card (AP FirstOwner),
  `/api/v1/me` stubbed as an open server:
  the card opens at the page's start with the first owner's form,
  no way to sign in, no provider, ☰ there, the name in focus,
  nothing of the page read behind it;
  Skip for now shows the open page with its banner, and a reload asks again;
  no card where the server is kept open, has no store, or has users;
  Create the owner turns until the card has closed, Skip for now and Cancel
  off meanwhile, a second submit creates nothing, one read of `/me` from the
  click to the page, a toast says it, with the login not kept in the app;
  a card closed while it turns starts the page once, and the late end leaves
  a later sign-in open;
  the passwords' mismatch first, the name checked 2 s after the last
  keystroke under the paused clock;
  a 401 or a 409 turns the card to the sign-in, nothing carried over,
  no write said not sent after the sign-in;
  a page left open learns of an owner made elsewhere from its stream's
  ticket refused, with the card shown and after Skip, the password's way
  chosen;
  the card no wider than the window, each button's text on one line, in a
  browser and in the app, at 390 px in six languages, before a press of the
  dice and after one, centred where it fits the room the window leaves it
  (868 of 900 px) and, where it is taller, at its top with the page
  scrolling to its end, the card showing all it holds, each control inside
  it and in view at some scroll (the sign-in card's tests keep it centred
  and within its room);
  on a phone of 640 px it starts at the top and scrolls; over
  plain HTTP in the app no sign-in's notice; on a touch screen no field in
  focus.
- `test_keep_open.py`, keeping a server without users open (AP FirstOwner,
  Block 3), `/api/v1/me` and the setting's write stubbed:
  Keep it open… shows the confirmation in the form's place, the card not
  moved, the owner's warning first, this page's address, the field in focus;
  Confirm off for every other address and on for this page's, typed or
  pasted, also reached as localhost; Enter with another address sends
  nothing; Back keeps what was typed; Confirm sends the setting once and the
  page starts once, a reload asks no more; a 401 or a 409 turns the card to
  the sign-in, a 403 or a network failure is said; on a touch screen the
  warning takes the focus; no Keep it open… where the server is kept open;
  the switch in System settings → Users turns it off once and stays off for
  that showing, and a refusal leaves it on; turned off, the card offers
  Keep it open… again, and kept open from it the switch is back; after a
  turn the card's ways out are on; the measures in six languages, before a
  press of the dice in the form and after one, as the form's, and the
  address copied in a browser and in the app without the clipboard's API.
- `test_ways.py`, the ways to the first owner on a running page (AP
  FirstOwner, Block 4), `/api/v1/me` stubbed: the banner's Create the
  owner…, ☰'s and Users' each open the card, the page behind it still (no
  poll, no stream) and Back to the page starts both again, the focus back
  on what opened it, nothing skipped; Hide hides the warning, kept after a
  reload, forgotten once the server has users, for that page alone where
  the storage refuses; no buttons without a store; ☰'s entries on an open
  server and with users; a hidden warning and ☰ to Create, the same page
  shown again; Users' section on an open server, and the owner listed
  after Create there; a setpoint under way lands; a refusal during Create
  does not turn the card, a read waits and a write is said not sent; the
  turn on a running page drops a stepped setpoint and another user's
  sign-in forgets the open page; no stream behind the card; a refusal
  during a Create that fails turns the card then; a half-typed form stays
  through the card; a refused read forgets the hiding; a storage that
  throws at the read; the warning's buttons at 390 px in six languages.
- `test_preferences.py`, the settings that follow the user:
  the poll and the scheme set in one browser reach the server and a fresh
  browser with the same token,
  the mode kept "only on this device" stays in its browser and the user's
  applies again when the switch is off; the guest the runner creates (`guest1`,
  password `visit-pass`) has no switches,
  and its poll stays in the browser, none sent;
  the operator's poll and scheme, changed just before Sign out,
  reach the operator's account and leave the browser,
  and the owner signing in next with its key gets neither;
  a scheme deleted in one browser stays deleted when another that had it loads.
  Since the four cases share the server,
  the page fixture removes the token's user's preferences before every page.
- `test_log.py`:
  the log opens, and a state event with changes shows what changed;
  `state` chosen as the kind shows only state events,
  the seven devices' at least, a device chosen shows only its events,
  all again shows the lines of before,
  and Older is offered as the page's size says,
  nothing older after a page shorter than 200 (with all events);
  with the server's answers stubbed, Older, under the lines, puts a page
  at the bottom, the view staying where it was,
  until a short page ends the paging,
  and where the server answers 409 to the newest line's id,
  it loads the log again, no line twice and none of the log before,
  also not after another choice took its place;
  under All Cubes a poll asks after the newest line's id,
  a 409 to it loads the log again once, each line once and none missing,
  a burst of 250 comes in one poll, the newest 20 shown,
  a backlog of more than ten pages loads the log again,
  on a server that ignores `after=` the poll ends,
  and adds no line from before the range,
  and a second poll asks nothing while one is on its way;
  with the page's `api()` holding the log's requests until the test answers
  them, an Older answer after Clear is dropped,
  a Cube switch takes the device chosen back to all,
  2200 lines stay when a live event comes,
  past 2000 lines without Older the oldest go from the lines and the view alike,
  and the error line shows with a kind chosen;
  a live event that comes while a page is held follows the page,
  once, also when the page holds it already;
  Clear is off with nothing chosen, and after a kind and a text sets them
  back and asks for every kind and device;
  the lines read newest first, a live event comes in on top,
  and a view scrolled down keeps its place;
  the last 20 events are the range at first, asked for without a time,
  without Older, and keep the newest 20 as live events come;
  the last hour asks since an hour ago and pages back within it,
  a line out of the hour goes at the minute's check,
  and with all events a line of another day shows its date;
  the range, the kind and the device chosen are in the address,
  in place of its entry, and a reload asks for them again,
  Clear takes the kind and the device out,
  and back from another page and the log's button keep the range;
  a link with them in another order opens the log with them,
  the address in its own order,
  and an address typed in its place shows its choices;
  unknown values and a device the Cube does not have leave a plain `#log`,
  and the log is asked for every device.
- `test_log_updates.py`, the log as a part (AP UiParts, Block 11),
  its reads of the Cube answered by the test (`StubLog`):
  a command's detail open with the focus on its button,
  a selection of text in a line,
  and the focus on a problem's Mark seen while another line's level changes
  stay the elements they were, the detail open, through a new line,
  every line drawn again, the rooms read again, the log loaded again
  and its problems read again;
  the search typed and a kind chosen stay through a new line, a load,
  another page and back (a forward guard);
  a choice's options stay the elements they were, with the focus,
  through the rooms and the Cubes read again;
  with a page of 200 lines a new line adds one element and removes none,
  every line drawn again and a load of the same events add and remove none;
  a line that came live keeps its element, with the focus, when a load brings it.
- `test_pairing.py`, pairing and removal:
  in System settings → Cubes, Pair a device with 5 seconds,
  the fake Cube's device joins with its serial,
  is named and put into the first room with the copy on,
  the row says so and the end of pairing is said;
  the server's log has the pairing and the copy;
  in the room's Devices tab, Remove from the Cube asks and removes it,
  and the room is as before.
  A device paired through the API while the page was not looking is listed under
  Devices without a room, where Remove asks and Keep takes it back,
  and Put into a room… with the copy off puts it into a room,
  the log without a copy;
  another is removed from there.
  With the body's `no-cubes`, the default manager's,
  the Cube's panel holds the pairing alone, and with `no-pair` too System settings →
  Cubes is gone.
  Each test stops pairing and removes the device at its start and end.
- `test_appearance.py`, My settings → Appearance:
  a scheme changes the computed `--bg`, the dark mode sets `data-theme`,
  the editor applies a colour live, grades the contrast
  and keeps a saved scheme in the browser;
  its hex fields take `ff0000` without the `#`, widen `#ABC` to `#aabbcc`
  and refuse `nope` with the colour kept,
  its badge column comes first so the colour inputs line up;
  Export downloads the scheme document,
  Import reads it back with a missing token filled from the default
  and an unknown one dropped,
  and a file that is not a scheme shows a message.
- `test_update.py`, System settings → Software:
  both buttons are greyed while the update helper installs (a stubbed
  `GET /api/v1/update`) and come back when it is done;
  Update asks in the page with the version,
  and Cancel takes the question back without a request.
- `test_connect.py`, the apps' connect page (`ui/connect.html`, docs/app.md),
  with `fake_bridge.js` as the app's bridge,
  put into the page by `add_init_script` and answering from a configuration
  the test gives it, which it may change while the page runs:
  each server shows what its probe found (on the LAN, remote, not reachable),
  a tap opens it at the address found with `open(id, url)` alone, one
  opening at a time, and one that does not answer says so;
  a server with a kept login has the sign-in icon, labelled, which opens it
  with `{signIn: true}`, and a key refused at a tap is said and gone;
  an address typed is probed by HTTPS first and added,
  a private one as the LAN address;
  what is not an address says so and plain HTTP outside the home network is warned of;
  a self-signed certificate opens the fingerprint sheet,
  Cancel adds nothing and Trust pins it,
  and a changed certificate shows the old and the new fingerprint before the server opens;
  a server's settings open from ⚙ and a long press,
  save the biometric switch, and Remove server, on a line of its own under
  Cancel and Save after a rule, with what it does, asks in place;
  discovery offers a server not in the list and moves a saved one's LAN address;
  the card of the server on the phone starts and stops it;
  Turn off, Forget all and Remove say what they revoked, their toasts naming
  the keys that stay valid, left or on the phone alone;
  Remove turns a kept login off first;
  a key left shows in its server's settings with Show in My profile,
  off while the server does not answer, which opens `<address>#profile?key=<id>`,
  Remove warns of it, and a session found there takes its line away;
  a session left, as the bridge's version 5 keeps one, shows in its
  server's settings, Sign out without an answer leaves one, Remove signs
  out first and warns of one left and names it;
  a browser without the bridge sees the line saying the page is the app's;
  and the server's page in the app takes its token from the bridge,
  keeps none in the browser, and hands a sign-in and a sign-out to the bridge;
  its sign-in card has Cancel left of each Sign in and of Create the owner,
  and "Select server…" last in its ☰, each calling `connect()`;
  with a login kept, the row with the fingerprint is on one line at 390 px
  in all six languages, the card centred,
  and the fingerprint's spinner turns inside its button in the icon's place;
  the card names its server under its title, in the app alone, with the
  name once where it is the address and an ⓘ for the instance id and the
  fingerprint, each copied with a tap, fitting the card at 390 px in all six
  languages (and in `test_keep_open.py` on the first owner's steps);
  ☰ in the app has This server's settings… and App settings… after Select
  server…, each calling the bridge, fitting ☰ in all six languages, and the
  connect page opens a server's settings from `#settings=<id>`;
  My profile in the app tags the key the phone keeps "this device's key",
  by `login.status()`'s `key_id`, and reads its keys once the sign-in's
  revoke is done, a key it revoked not marked but named in a toast;
  a server's settings name the kept login's key by its id.
  axe-core (`axe-playwright-python`) finds no violation on the connect page
  in each state the tests open.
- `test_notifications.py`, 👤 → Notifications:
  the rules show their defaults, a delay and a switch changed reach the
  owner's preference `notify`, a delay out of bounds is refused, and the page
  reads them back;
  this browser turned on and off against a fake `pushManager`,
  put into the page by `add_init_script` and a reload,
  since a headless browser has no push service:
  the worker registered beside the page and told the language,
  the subscription posted with its name and marked in the list,
  and removed at the server and in the browser;
  another device removed by its Remove;
  the toast of a notification from the stream and from the service worker,
  shown once;
  a guest's page, which says why it has none, and a browser without push;
  and the page's own `sw.js` taking a push that Chromium's DevTools deliver:
  the page's toast while the page is in front, and, made to see no window,
  the system's notification in the language the page told it.
  The subscriptions go to the loopback's discard port, so that a notification
  the server decides meanwhile reaches no network,
  and each test removes what it made.
  axe-core finds no violation on the page in each of its states.
- `test_parts_guard.py`, the guard of the page's parts (AP UiParts, Block 3;
  `docs/ui-testing.md`, section 8):
  for every region of the page in its table (`REGIONS`: its containers,
  and its views, which open its menus, panels and forms),
  around each data source (`SOURCES`: `poll(false)`, `loadCubes()`,
  `loadRooms()`, `refresh()`, a state event per device on the page's
  `EventSource`, `MassimoParts.log.loadAlerts()`, `MassimoParts.minute()`,
  `MassimoParts.log.poll()`,
  `applyStoredSettings()`, `loadMe()`),
  with the data as it was:
  every element a user can hold is the same element, every open `details`
  open, every text, number, date or time field holds what was typed into it,
  the focus where it was, and the first three again once the focus has left;
  every region of the page is checked, the list of those not moved gone
  with AP UiParts' Block 15;
  every request but a GET is refused and fails the test,
  so nothing reaches the fake Cube or the store.
  It runs in two of the four cases, 1100 px in English and 390 px in
  German, each width and each language once,
  and is deselected in the other two (`GUARD_CASES`, the mark `cases`).
- Every button on every page the tests open has an accessible name (its text,
  `aria-label`, `aria-labelledby` or `title`),
  and the pages have no horizontal scroll where the tests look for one.

Nothing is sent to the fake Cube but by `test_pairing.py`,
whose pairing, links, copy and removal it answers and forgets again;
the Cube changes of the other tests go to the server's store only.
The state events are applied in the page (`updateDevice`, or dispatched on the
page's `EventSource`), since the fake Cube does not change its states.
The tests reach the page's `state` through `page.evaluate`:
it is a top-level `const` of `app.js`, in the page's global scope,
and nothing is put on `window` for them.

## Screenshots

`make ui-screenshots` runs `run.sh screenshots`:
the same start-up, then `screenshots.py` in place of the suite.
It takes a fixed set of PNGs into `docs/screenshots/`,
named `<page>-<width>-<theme>-<lang>.png` and listed in one place,
`SETS` in the script:
the Rooms page, a tile's device list, the configuration's tabs Temperatures,
Devices, Week programme and Copy, the House panel, the log, the 👤 menu,
the Cube choice open, My profile, My settings, Notifications,
the Appearance editor, Cube settings, System settings,
Maintenance, System info with the store and the settings in effect
(fixed answers, as is its machine:
the run's temporary directory, ports and host differ),
the Cube choice with a found Cube, System settings → Cubes and ☰ open,
at 1100 px in light and dark (`data-theme`), in English;
the Rooms page, ☰ open, the Cube choice open, My settings, Cube settings,
System settings, Maintenance and System info also at 390 px,
and the Rooms page in German as well.
A shot is framed by its subject, set beside each page in `PAGES`:
the viewport (1100 × 800,
at phone width 390 × 844) where the page is the subject (the Rooms page,
the log, every phone shot), and the one element where one element is (a tile,
the House panel, a settings page or one of its sections,
the header with the found Cube), whole however tall, with 12 CSS px around it,
so that the sizes differ from shot to shot.
The PNGs it took before are removed first.

The set is retaken at every change of the UI, as the suite is run,
and committed with it.
It comes out the same on every run, byte for byte on one machine:
the page's clock is fixed and the server's times, its answers' Date header
among them, are replaced by fixed ones on the way to the page,
the time zone is Europe/Zurich,
the event stream is a stub that sends nothing,
the first room's thermostat is made 30 hours old and stale,
the version (in the log's line of the server's start too),
the Software section, the fake Cube's address, System info's machine
and a found Cube are fixed answers,
and Chromium rasters every tile whole (`--disable-partial-raster`):
by default it rasters again only the part of a tile that changed,
and an anti-aliased edge rastered in part comes out a level of 255 off
the same edge rastered whole,
so a few shots of every run changed on the header's rounded buttons.
Another machine's fonts give other pixels.
Nothing is compared: a shot that cannot be taken fails the run,
a changed picture does not.

`slideshow.sh` makes `docs/screenshots/slideshow.gif` from the main set (1100 px, light,
English) in its order, which it takes from `screenshots.py --list`:
every frame shrunk, never enlarged, to fit, centred,
whole and opaque on an 880 × 640 canvas, 3 seconds each, looping.
It needs ImageMagick;
`slideshow.sh --help` says the rest.
It lives here, next to what takes the pictures,
because the repository keeps its developer tools under `tools/`.

CI runs the suite (with `UI_TEST_REQUIRED=1`) in a job per case,
"UI test (1100-en)" to "UI test (390-de)",
and takes the set in a job of its own beside them, "UI test (screenshots)",
on every push to `main` and every pull request that changes the UI's paths,
in Microsoft's Playwright image,
and uploads the PNGs as the artifact `ui-screenshots` (`GUIDELINES/project/CI.md`);
when a case's step fails it also uploads `tools/ui-test/results/`,
the failed tests' screenshots, the retry's screenshots, videos and traces,
and the servers' logs,
as the artifact `ui-test-results-<case>`.
It does not make the slideshow, which needs ImageMagick:
`slideshow.sh` is run where the set is taken, and the GIF committed with it.

## Setup

`run.sh` uses `tools/ui-test/.venv/bin/python` when it exists, else `python3`;
`UI_TEST_PYTHON` picks another interpreter.

On a usual Linux or macOS:

```sh
python3 -m venv tools/ui-test/.venv
tools/ui-test/.venv/bin/pip install -r tools/ui-test/requirements.txt
tools/ui-test/.venv/bin/python -m playwright install chromium
```

On the Mint box (and other Debian/Ubuntu systems without `python3-venv`),
`python3 -m venv` fails because `ensurepip` is missing.
Without root, make the environment without pip and bootstrap pip into it:

```sh
python3 -m venv --without-pip tools/ui-test/.venv
curl -sSfo /tmp/get-pip.py https://bootstrap.pypa.io/get-pip.py
tools/ui-test/.venv/bin/python /tmp/get-pip.py
tools/ui-test/.venv/bin/pip install -r tools/ui-test/requirements.txt
tools/ui-test/.venv/bin/python -m playwright install chromium
```

With root, `sudo apt install python3-venv` makes the usual way work.
If Chromium reports missing system libraries,
`sudo tools/ui-test/.venv/bin/python -m playwright install-deps chromium` installs them.
Node is not needed.

## Notes for writing more tests

- The page's Content Security Policy forbids `eval`,
  so `page.wait_for_function` with a string fails.
  Wait with locators, or poll `page.evaluate` from Python.
- Several tiles have the same buttons,
  so scope locators to one tile (`page.locator(".room").first`) and,
  inside a configuration, to `.config-body`.
- A test's page has no routes but the ones the test sets,
  and a page without routes stops intercepting:
  a request held in a route handler is released (`route.continue_()`)
  before its route is removed, or Playwright lets it through itself.
- A test starts on a fresh page, so it opens what it needs (the settings,
  a panel) and does not rely on an earlier test;
  what it changes on the server it puts back;
  a step of the cleanup that may find nothing to undo goes through `undo`,
  never a `try` that swallows its error, so that a step that fails says so,
  and a cleanup of several steps through `cleanup`, so that a failed step
  leaves no later one undone.
- Data a test puts into the page goes through the fixture `reads`,
  not into `state`: the page's poll and the reads that events start
  take `state` back.
  A route handler of the test's is routed with `safe_route`,
  never wrapped in `try`/`except Exception`.
- A test of the log that sends events opens the log, calls
  `isolate_log(page, reads)` and `wait_log_live(page)`, then sends them;
  a test that waits for an event the server sends marks the stream before
  its action and waits with `stream.wait`,
  and finds the event's line by what the event names, not by `.first`.
- A wait that returns whether it was met is asserted where it is called.
- A test's own wait loop ends at `scaled(seconds)` of `helpers.py`,
  so that `UI_TEST_SCALE` stretches it as it does the kit's waits;
  a window that waits for nothing to come does not take it.
- What a reload or a click brings is waited for,
  not looked at once right after another wait:
  the page goes on after the moment the first wait saw
  (`test_header.py`, the reload of the log).
- "Nothing came" is checked against an end, not after a fixed wait:
  a read sent after the action through the same route,
  the page's state that says nothing waits,
  a count of what the action started,
  or a timer of the page's own due after the one it rules out (`docs/ui-testing.md`,
  section 9).
- A test that measures what the page does by itself,
  its drawings or times that stand still,
  holds the page's poll meanwhile (`with held_poll(page):`),
  which fails where something started it again.
