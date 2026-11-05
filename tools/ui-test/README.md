# Web UI smoke test

`make ui-test` runs `run.sh`. The script:

1. builds massimo;
2. starts the fake Cube (`massimo fakecube`) with `greeting.txt`, which is the protocol fixture from internal/protocol/cube/testdata plus an `L:` line with device states;
3. starts `massimo serve` against the fake Cube;
4. runs `smoke.py` with Playwright (Chromium).

Both servers run on loopback ports of their own (62919 and 8119; set `UI_TEST_CUBE_PORT` and `UI_TEST_API_PORT` to change them). They run in an empty temporary directory without the caller's environment, so a `.env` or `MASSIMO_*` variables can't point them at real hardware. Both are stopped at the end.

The smoke test runs at 1100 and 390 px, in English and in German (from the browser's locale). It checks that:

- the page loads with room tiles and the right language;
- a tile's device list and its configuration open, every configuration tab shows, and the configuration closes;
- the log and the settings page open;
- there is no console error, no page error and no horizontal scroll.

It sends nothing to the fake Cube.

When Playwright for Python is missing, `make ui-test` says so and exits 0.

## Setup

`run.sh` uses `tools/ui-test/.venv/bin/python` when it exists, else `python3`; `UI_TEST_PYTHON` picks another interpreter.

On a usual Linux or macOS:

```sh
python3 -m venv tools/ui-test/.venv
tools/ui-test/.venv/bin/pip install -r tools/ui-test/requirements.txt
tools/ui-test/.venv/bin/python -m playwright install chromium
```

On the Mint box (and other Debian/Ubuntu systems without `python3-venv`), `python3 -m venv` fails because `ensurepip` is missing. Without root, make the environment without pip and bootstrap pip into it:

```sh
python3 -m venv --without-pip tools/ui-test/.venv
curl -sSfo /tmp/get-pip.py https://bootstrap.pypa.io/get-pip.py
tools/ui-test/.venv/bin/python /tmp/get-pip.py
tools/ui-test/.venv/bin/pip install -r tools/ui-test/requirements.txt
tools/ui-test/.venv/bin/python -m playwright install chromium
```

With root, `sudo apt install python3-venv` makes the usual way work. If Chromium reports missing system libraries, `sudo tools/ui-test/.venv/bin/python -m playwright install-deps chromium` installs them. Node is not needed.

## Notes for writing more checks

- The page's Content Security Policy forbids `eval`, so `page.wait_for_function` with a string fails. Wait with locators, or poll `page.evaluate` from Python.
- Several tiles have the same buttons, so scope locators to one tile (`page.locator(".room").first`) and, inside a configuration, to `.config-body`.
