# Web UI tests and screenshots

`make ui-test` runs `run.sh`.
The script:

1. builds massimo;
2. starts the fake Cube (`massimo fakecube`), which also answers discovery,
   with `greeting.txt`,
   which is the protocol fixture from internal/protocol/cube/testdata plus an
   `L:` line with device states (measured temperatures included) and a window contact,
   "Fenster Kind 1", that reports an open window;
3. starts `massimo serve` against the fake Cube,
   with the API token `ui-test-token`;
   the Cube comes from `MASSIMO_CUBE_KEQ0537249_ADDRESS` and `_NAME` ("Fake Cube"),
   which the server seeds into its store,
   `massimo.db` in the temporary directory;
4. runs the pytest suite in this directory with `pytest-playwright` (Chromium),
   handing it the page's address in `UI_TEST_URL` and the token in `UI_TEST_TOKEN`.

Both servers run on loopback ports of their own (62919 and 8119;
set `UI_TEST_CUBE_PORT` and `UI_TEST_API_PORT` to change them).
They run in an empty temporary directory without the caller's environment,
so a `.env` or `MASSIMO_*` variables can't point them at real hardware.
Both are stopped at the end.

When Playwright, pytest or pytest-playwright is missing,
`make ui-test` says so and exits 0;
with `UI_TEST_REQUIRED=1`, which CI sets, it exits 1 instead.

## Running all, one file, one test

```sh
make ui-test                                          # the whole suite
PYTEST_ADDOPTS='-k cubes' make ui-test                # the tests whose names match
PYTEST_ADDOPTS='-k "hidden and 390-de"' make ui-test  # one test in one case
tools/ui-test/run.sh smoke test_log.py                # one file
tools/ui-test/run.sh smoke 'test_log.py::test_log_changes[1100-en-chromium]' -x
```

`PYTEST_ADDOPTS` is pytest's own;
`run.sh smoke` hands the arguments after it to pytest,
whose working directory is `tools/ui-test/`, so files are named from there.
A test's id carries its case: `1100-en`, `390-en`, `1100-de`, `390-de`,
then the browser.

Of a failed test, pytest-playwright keeps the trace,
the screenshot and the video in `tools/ui-test/results/`,
one directory per test (gitignored, emptied at the start of every run);
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
  the used ticket handed out, console and page errors;
- `guard` (autouse): a test fails on a console error, a page error,
  or a horizontal scroll left at its end;
  the refused stream of the stream test and the stubbed 503 of the discovery are expected;
- the marker `used_ticket`: the page's first stream gets a used ticket,
  which the server refuses with 401,
  and the page is handed over before the stream is open again.

`helpers.py` holds the texts the tests expect in both languages,
the stubbed answers, `wait_until`, `named_buttons` and `found_text`.

The four cases share the one server,
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
  and the server's start time in expert mode.
- `test_stream.py`:
  the event stream opens with tickets only, never with the token,
  and after a refused stream (the test hands out a used ticket first,
  which the server answers with 401) the page opens it again with a new ticket.
- `test_discover.py`:
  the Cube choice's "Search for new Cubes…" shows "Searching…" until the server answers,
  then lists the Cube the fake Cube announces in discovery (KEQ0000000,
  not configured) as not selectable with its DHCP flag,
  with its env lines under the header and its `cube_found` in the log;
  an injected `cube_found` event adds another one live;
  a refused search (a stubbed 503) shows the server's message.
  The fake Cube answers discovery on loopback (`-discovery`,
  `UI_TEST_DISCOVERY_PORT`, 23919),
  and the server asks it there (`-discover-target`).
  The server's own discovery socket is UDP 23272 as always,
  so where another program holds that port the search answers 503
  and the tests that need an answer are skipped.
- `test_cubes.py`, Settings → Cubes:
  a new name is saved and shown in the Cube choice,
  the settings file's name is offered as a differing line and "Take the file's
  value" brings it back;
  hiding the devices of the last room takes its tile away and showing them again
  brings it back; the found Cube KEQ0000000, its DHCP flag shown,
  is added and removed again (the test searches first where nothing was found yet,
  and is skipped without a discovery answer), Remove asks in the panel,
  and Keep takes the question back.
- `test_settings.py`:
  the settings page opens;
  with the Cube's panel open,
  English and then German chosen in the settings redraw its labels,
  cloned from a template, in that language, and the panel stays open.
- `test_users.py`, Settings → Users and Roles as the owner:
  the owner and the operator the runner creates (`massimo user add operator`,
  password `op-pass`) are listed with the operator's tokens,
  and the six default roles with the operator's six capabilities.
- `test_login.py`, in a fresh browser context without the stored token:
  the sign-in dialog opens, refuses a wrong password, signs in as the operator,
  who sees the tiles but neither Users, Roles nor Cubes,
  and Log out brings the dialog back and forgets the token.
- `test_log.py`:
  the log opens, and a state event with changes shows what changed.
- `test_appearance.py`, Settings → Appearance:
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
- `test_update.py`, Settings → Software:
  both buttons are greyed while the update helper installs (a stubbed
  `GET /api/v1/update`) and come back when it is done;
  Update asks in the page with the version,
  and Cancel takes the question back without a request.
- Every button on every page the tests open has an accessible name (its text,
  `aria-label`, `aria-labelledby` or `title`),
  and the pages have no horizontal scroll where the tests look for one.

Nothing is sent to the fake Cube;
the Cube changes go to the server's store only.
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
Devices, Week programme and Copy, the House panel, the log, the settings,
the Appearance editor, the Cube choice with a found Cube and Settings → Cubes,
at 1100 px in light and dark (`data-theme`), in English;
the Rooms and the Settings page also at 390 px,
and the Rooms page in German as well.
A shot is framed by its subject, set beside each page in `PAGES`:
the viewport (1100 × 800,
at phone width 390 × 844) where the page is the subject (the Rooms page,
the log, every phone shot), and the one element where one element is (a tile,
the House panel, the Settings panel or one of its sections,
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
the failed tests' traces, screenshots and videos,
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
