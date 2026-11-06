# Web UI smoke test and screenshots

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
4. runs `smoke.py` with Playwright (Chromium).

Both servers run on loopback ports of their own (62919 and 8119;
set `UI_TEST_CUBE_PORT` and `UI_TEST_API_PORT` to change them).
They run in an empty temporary directory without the caller's environment,
so a `.env` or `MASSIMO_*` variables can't point them at real hardware.
Both are stopped at the end.

The smoke test runs at 1100 and 390 px,
in English and in German (from the browser's locale).
It checks that:

- the page loads with room tiles and the right language;
- a tile's device list and its configuration open,
  every configuration tab shows, and the configuration closes;
- the log and the settings page open;
- the event stream opens with tickets only, never with the token,
  and after a refused stream (the test hands out a used ticket first,
  which the server answers with 401) the page opens it again with a new ticket;
- the room with the open window shows "Open" on its tile, and only that room;
  the badge goes when a state event closes the window;
- the measured temperature shows its age;
  after a state event makes the first room's thermostat 30 hours old and stale,
  the tile warns, the temperature is greyed,
  and the device list shows the stale badge and the values' ages,
  with the exact times in expert mode;
- the Cube choice's "Search for new Cubes…" shows "Searching…" until the server answers,
  then lists the Cube the fake Cube announces in discovery (KEQ0000000,
  not configured) as not selectable,
  with its env lines under the header and its `cube_found` in the log;
  an injected `cube_found` event adds another one live;
  a refused search (a stubbed 503) shows the server's message.
  The fake Cube answers discovery on loopback (`-discovery`,
  `UI_TEST_DISCOVERY_PORT`, 23919) and the server asks it there
  (`-discover-target`).
  The server's own discovery socket is UDP 23272 as always,
  so where another program holds that port the search answers 503
  and the test skips the checks that need an answer;
- Settings → Cubes: a new name is saved and shown in the Cube choice,
  the settings file's name is offered as a differing line and "Take the file's
  value" brings it back,
  hiding the devices of the last room takes its tile away and showing them again
  brings it back,
  and the found Cube KEQ0000000 is added and removed again (skipped without a
  discovery answer); everything is put back,
  since the four runs share one server;
  Remove asks in the panel, and Keep takes the question back;
- the Cube's panel open,
  English and then German chosen in the settings redraw its labels,
  cloned from a template, in that language, and the panel stays open;
- Settings → Software: Update asks in the page with the version,
  and Cancel takes the question back without a request;
- every button on every page the test opens has an accessible name (its text,
  `aria-label`, `aria-labelledby` or `title`);
- there is no console error, no page error and no horizontal scroll.

It sends nothing to the fake Cube;
the Cube changes go to the server's store only.
The state events of the last two points are applied in the page (`updateDevice`),
since the fake Cube does not change its states.

When Playwright for Python is missing, `make ui-test` says so and exits 0.

## Screenshots

`make ui-screenshots` runs `run.sh screenshots`:
the same start-up, then `screenshots.py` in place of the smoke test.
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

The set is retaken at every change of the UI, as the smoke test is run,
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

CI's Screenshots job runs the smoke test and then takes the set and the
slideshow on every push and pull request,
and uploads them as the artifact `ui-screenshots` (`GUIDELINES/project/CI.md`).

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

## Notes for writing more checks

- The page's Content Security Policy forbids `eval`,
  so `page.wait_for_function` with a string fails.
  Wait with locators, or poll `page.evaluate` from Python.
- Several tiles have the same buttons,
  so scope locators to one tile (`page.locator(".room").first`) and,
  inside a configuration, to `.config-body`.
