# Web UI tests and screenshots

`make ui-test` runs `run.sh`.
The script:

1. builds massimo;
2. starts a server set for each of four pytest-xdist workers
   (`UI_TEST_WORKERS`, default 4), each set:
   the fake Cube (`massimo fakecube`), which also answers discovery
   and has a device that joins pairing mode (`-pair 0a010d,0abcde,TEQ0000001`:
   a copy of the radiator thermostat 0a010d, two seconds after `n:`,
   again after it was removed), with `greeting.txt`,
   which is the protocol fixture from internal/protocol/cube/testdata plus an
   `L:` line with device states (measured temperatures included) and a window contact,
   "Fenster Kind 1", that reports an open window;
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
tools/ui-test/run.sh smoke test_log.py                # one file
tools/ui-test/run.sh smoke 'test_log.py::test_log_changes[1100-en-chromium]' -x
UI_TEST_WORKERS=1 make ui-test                        # in one process
```

`PYTEST_ADDOPTS` is pytest's own;
`run.sh smoke` hands the arguments after it to pytest,
whose working directory is `tools/ui-test/`, so files are named from there.
A test's id carries its case: `1100-en`, `390-en`, `1100-de`, `390-de`,
then the browser.

Of a failed test, pytest-playwright keeps
the screenshot and the video in `tools/ui-test/results/`,
one directory per test (gitignored, emptied at the start of every run).
The first pass records no trace:
its DOM snapshots make every step slower,
840 ms instead of 14 for a step in System settings at 1100 px,
and slow every wait with it.
When tests fail, `run.sh` runs them once more with the trace
(`--last-failed --tracing retain-on-failure`, the same arguments),
and keeps the trace of each that fails again in `tools/ui-test/results/retry/`;
it says whether they failed again, and the run fails either way:
a test that passes only the second time is flaky, not fixed.
`python -m playwright show-trace <trace.zip>` replays a trace step by step,
with the page's DOM, console and network at every step.

## The fixtures

`conftest.py` holds them:

- `case`:
  every test runs four times, at 1100 and 390 px,
  in English and in German (from the browser context's locale);
- `page`:
  a browser context of its own per test and case (pytest-playwright's `context`,
  with `browser_context_args` set from `case`),
  the token put into `localStorage` before the page loads,
  the page loaded with its room tiles and its event stream open;
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
  and the page is handed over before the stream is open again.

`fake_bridge.js` is the app's bridge for `test_connect.py`,
`window.massimo` as docs/app.md describes it,
recording every call in `window.__fakeBridge.calls`.

`helpers.py` holds the texts the tests expect in both languages,
the stubbed answers, `wait_until`, `named_buttons` and `found_text`,
`open_user_menu` and `sign_out` for the header's 👤 menu,
and `show_view`, which opens a page as a user does:
Rooms, the log and System settings with their header button,
or the log and System settings from ☰ on a narrow screen,
My profile and My settings from the 👤 menu.

The four cases share the one server of a worker,
so a test that changes the server's store puts it back;
what a test stores in the browser goes with its context.

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
  and the tile is drawn with the new state once the focus leaves;
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
- `test_backup.py`, System settings → Backup and restore against the fake Cube's store:
  the Cube's backup downloads as YAML,
  one taken by hand shows in the list as "by hand" and downloads,
  its plan says the Cube holds it already,
  it compares with the one before when there is one, and it is deleted again.
- `test_users.py`, System settings → Users and Roles as the owner:
  the owner and the operator the runner creates (`massimo user add operator`,
  password `op-pass`) are listed with the operator's tokens,
  and the six default roles with the operator's seven capabilities;
  in the operator's open panel, Create key shows the new key,
  lists every token once and sends one POST per click,
  and the test revokes its keys again;
  a guest made with an end date that has seconds is saved without the end date
  when its display name alone changes, and Save without a change sends nothing;
  a user's Remove asks in the panel, Keep gives the focus back to Remove,
  and a double click on Yes sends one DELETE and says it worked.
- `test_login.py`, in a fresh browser context without the stored token:
  the sign-in dialog opens, refuses a wrong password, signs in as the operator,
  who sees the tiles but neither Users, Roles nor Cubes,
  and is named in the 👤 menu, whose Sign out brings the dialog back and
  forgets the token;
  a sign-in at the page's start, and again after Sign out,
  starts the page once: one stream ticket and one read of `/me` each.
- `test_preferences.py`, the settings that follow the user:
  the poll and the scheme set in one browser reach the server and a fresh
  browser with the same token,
  the mode kept "only on this device" stays in its browser and the user's
  applies again when the switch is off; the guest the runner creates (`guest1`,
  password `guest-pass`) has no switches, and its poll stays in the browser;
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
  until a short page ends the paging;
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
  a tap opens it at the address found, and one that does not answer says so;
  an address typed is probed by HTTPS first and added,
  a private one as the LAN address;
  what is not an address says so and plain HTTP outside the home network is warned of;
  a self-signed certificate opens the fingerprint sheet,
  Cancel adds nothing and Trust pins it,
  and a changed certificate shows the old and the new fingerprint before the server opens;
  a server's settings open from ⚙ and a long press,
  save the biometric switch and Remove asks in place;
  discovery offers a server not in the list and moves a saved one's LAN address;
  the card of the server on the phone starts and stops it;
  a browser without the bridge sees the line saying the page is the app's;
  and the server's page in the app takes its token from the bridge,
  keeps none in the browser, and hands a sign-in and a sign-out to the bridge.
  axe-core (`axe-playwright-python`) finds no violation on the connect page
  in each state the tests open.
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
the Cube choice open, My profile, My settings, the Appearance editor,
System settings, its Maintenance with the store and the settings in effect
(fixed answers: the run's temporary directory and ports differ),
the Cube choice with a found Cube and System settings → Cubes,
at 1100 px in light and dark (`data-theme`), in English;
the Rooms page, ☰ open, the Cube choice open, My settings and System
settings also at 390 px,
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
the page's clock is fixed and the server's times are replaced by fixed ones
on the way to the page, the time zone is Europe/Zurich,
the event stream is a stub that sends nothing,
the first room's thermostat is made 30 hours old and stale,
and the version, the Software section,
the fake Cube's address and a found Cube are fixed answers.
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

CI's Screenshots job runs the suite (with `UI_TEST_REQUIRED=1`) and then takes the set
on every push to `main` and every pull request,
in Microsoft's Playwright image,
and uploads the PNGs as the artifact `ui-screenshots` (`GUIDELINES/project/CI.md`);
when a step fails it also uploads `tools/ui-test/results/`,
the failed tests' screenshots and videos and the retry's traces,
as the artifact `ui-test-results`.
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
  what it changes on the server it puts back.
