# Testing the web UI (v1.24.0)

An evaluation of a Cypress suite for the web UI, against the alternatives:
what the Playwright smoke test covers today and what it cannot do well,
what Cypress would give and what it would cost,
the other runners on the same footing,
how each would take a fixed set of screenshots,
a recommendation with the steps to get there,
and the questions left for the owner;
then, apart from the evaluation, the guard of the page's parts (section 8)
and what a test does about the page's own reads and its stream (section 9).
Nothing of sections 1 to 7 is implemented; no test code changes with them.
The figures were measured on 2026-11-06 on the development machine (WSL 2)
and on the CI runs of that day, unless a section says otherwise.

## 1. What the smoke test does today

`make ui-test` runs `tools/ui-test/run.sh`,
which builds `massimo`, starts the fake Cube with `tools/ui-test/greeting.txt`
(discovery answered on a loopback port),
starts `massimo serve` against it with the API token `ui-test-token`,
both in an empty temporary directory under `env -i`,
waits until the server reports the Cube connected,
and runs `tools/ui-test/smoke.py` with Playwright for Python and Chromium.
`tools/ui-test/README.md` lists what the script checks.

### Coverage

The script runs four times, at 1100 and 390 px, in English and German,
each in a browser context of its own against the one server.
Each run goes through, in order: the tiles, a device list,
every configuration tab, the log and the settings;
the update section greyed while the helper installs (a stubbed `GET /api/v1/update`);
the event stream opened with tickets only, after a refused ticket;
the open-window badge and the stale warning,
through state events applied in the page; discovery (held,
answered by the fake Cube, a stubbed 503) and a live `cube_found`;
Settings → Cubes (a name saved and taken back from the settings file,
hidden devices, a found Cube added and removed);
a state event's changes in the log; and Settings → Appearance (schemes,
the dark mode, the editor, export and import of a scheme file).
Every run fails on a console error, a page error or a horizontal scroll.

On 2026-11-06 a run printed 404 `ok` lines and no failure
(the brief that asked for this document counted 336, before the latest checks),
and `make ui-test` took 49 s from start to end, the Go build included.
The installation it needs: a virtual environment of 80 MB (Playwright 1.64.1,
71 MB of it,
which bundles its own Node driver) and Chromium in `~/.cache/ms-playwright`,
238 MB, plus 154 MB for the headless shell.

How it reaches into the page:

- **Stubbed routes** with `page.route`:
  the first `POST …/events/ticket` gets a used ticket,
  the discovery request is held and released, or answered 503,
  and `GET /api/v1/update` answers the helper's states.
- **Events** are injected into the page's real `EventSource`:
  `state.events.dispatchEvent(new MessageEvent("cube_found", …))`,
  or the page's `updateDevice()` is called with a changed device,
  because the fake Cube does not change its states and so the server never sends one.
- **The page's state** is read with `page.evaluate("state.eventsOpen")` and the like.
  `state` is a top-level `const` in a classic script:
  it is in the page's global scope but not a property of `window`.
  Playwright evaluates through the DevTools protocol,
  which the page's Content-Security-Policy does not apply to;
  `page.wait_for_function` with a string does not work under that policy,
  so the script polls from Python (`wait_until`).
- **The token** is put into `localStorage` before the page loads (`add_init_script`).
- **The language** comes from the browser context's locale.

### What it cannot do well

- **One script, one long path.**
  The 404 checks are lines in seven functions that run in order on one page.
  A timeout ends the rest of that run,
  which reports one failure for many checks, and no check can be run on its own.
  A check depends on what an earlier one left behind:
  the four runs share one server,
  so `cubes()` puts every name and hidden device back.
- **No fixtures.**
  Each run builds its context by hand in the main loop,
  the console filter is a closure with the run's lists,
  and the strings per language are dictionaries at the top.
  A new panel (pairing, backup and restore) means a new function,
  appended to the path and to the module docstring.
- **No component isolation.**
  Nothing renders one tile or one panel with data of its own;
  every check goes through the whole page against the server.
- **Nothing kept on failure.**
  A failing check prints `FAIL` and a line; there is no trace,
  no screenshot and no video, so a failure on another machine is retold,
  not shown.
- **Python next to Go and JavaScript.**
  The project's code is Go and plain JavaScript; the test is the only Python,
  with its own setup (the Mint box needs `get-pip.py`,
  `tools/ui-test/README.md`).
- **Not in CI.**
  `GUIDELINES/project/CI.md` section 2 leaves it to the developer's machine,
  and `run.sh` exits 0 where Playwright is missing,
  so a pull request whose author has no venv is not tested at all.
- **Two of six languages.**
  French, Italian, Dutch and Polish are never loaded by a test.

What it does well, and what any replacement has to keep:
the real page with its real CSP against the real server and the fake Cube,
started per run and isolated from `.env`;
the stream with its single-use tickets; two widths and two languages;
a run of under a minute.

## 2. What a Cypress suite would give

### End-to-end tests

Cypress would run specs against the same setup:
`run.sh` (or the `start-server-and-test` package) starts the fake Cube and the server,
and `cypress run` visits `http://127.0.0.1:8119/`.
The owner's Nextcloud app does the same against a running Nextcloud
(`metaworx/file_checksum_search`, `cypress.config.cjs`, `tests/e2e/`),
with `cypress-io/github-action` waiting on the URL.
Specs per area (`tiles.cy.js`, `stream.cy.js`, `cubes.cy.js`,
`appearance.cy.js`, …) would replace the seven functions,
each with its own `beforeEach`, and each runnable on its own with `--spec`.

### Component tests

Cypress component testing does not fit this UI.
It mounts a component through a framework adapter (React, Vue, Angular, Svelte)
and serves the spec through a bundler's dev server (Vite or webpack);
both are configured as `component.devServer`.
The UI has neither: eight classic scripts that share globals (`state`, `$`, `t`,
`renderRooms`, `updateDevice`) and are loaded in order by `index.html`,
with no `export` and no `import`.
A custom dev server could serve a page that loads those scripts and calls
`renderRooms()` with fixture data,
but that page would be a second `index.html` that has to be kept in step with
the real one, it would need Vite or a hand-written server in `node_modules`,
and it would run without the server's CSP.
The same isolation is reached more cheaply in an end-to-end run:
answer every `/api/v1/…` call from fixtures (`cy.intercept` or `page.route`)
so that the real page renders data the test chose, without the server.
Pure functions (`fmtTemp`, the language fallback,
the scheme import's validation) can be tested in the page with an evaluate,
as the smoke test does.

### Fixtures and intercepts

`cy.intercept` covers every stub the smoke test has:
`cy.intercept({ method: "POST", url: "**/events/ticket", times: 1 }, { ticket: USED })`
for the refused ticket, a `statusCode: 503` body for the refused search,
a handler that holds the discovery request until a promise resolves,
and fixtures in `cypress/fixtures/*.json` for the update states.
Spying on the stream's request (`cy.intercept("GET", "**/events?*")`)
records its URLs as the smoke test's `request` listener does.

### Debugging

The interactive runner shows every command with the DOM before and after it
(the time-travel debugger), which is the quickest way to write a check.
`cypress run` takes a screenshot at a failing step and can record a video per spec;
the owner's project keeps the screenshots as an artifact and switches video off.

### The owner's familiarity

The owner writes and reads Cypress specs already,
and has a CI action for them (`.github/actions/cypress-run`:
the binary cached by version, `cypress-io/github-action` with `install: false`,
screenshots uploaded on failure).
That project's lessons carry over:
build state per spec, `invoke()` rather than `then()` for rendered text,
text-free selectors where the text is not the point.
It also shows where the owner draws the line: its component tests are Vitest,
not Cypress.

### What it costs

- **A Node toolchain in the repository**:
  `package.json`, `package-lock.json`, a `.gitignore` entry for `node_modules`,
  a Node version to pin (`.nvmrc`),
  and Dependabot or by-hand updates of the lockfile.
  Today the repository has none, and the contract says the UI has no build step;
  a test-only `package.json` keeps that true but is a second dependency manager.
- **The binary**:
  `cypress` 16.2.1 is 4.7 MB on npm,
  and its install script downloads the Electron-based binary into `~/.cache/Cypress`:
  361 MB unpacked for 15.20.0 on this machine,
  about 240 MB to download as the owner's action measures it.
- **A browser on the runner**:
  Electron comes with the binary; `ubuntu-latest` has Chrome,
  which the owner's action selects.
- **A CI job** like the owner's: checkout, Go,
  `npm ci` with `CYPRESS_INSTALL_BINARY=0`, the cached binary,
  `make` the servers, `cypress run`, screenshots on failure.
  The owner's job takes 8 to 10 minutes, of which Cypress 6.5 to 9.5,
  but most of that is Nextcloud;
  this UI's 404 checks take 40 s in Playwright, and Cypress adds a start-up of
  about 10 to 20 s and runs specs one after the other.
  A fair estimate is 3 to 4 minutes for the job, cache warm.
- **Keeping it green in six languages and two widths**:
  Cypress has no projects; the matrix is a loop in each spec or a run per
  combination (`--config viewportWidth=390 --env lang=de`).
  The browser's language is fixed per launch (`--lang`,
  as the owner's config does),
  so a language per test means overriding `navigator.languages` in
  `onBeforeLoad` or setting the page's own language setting in `localStorage`.
- **Duplication**:
  until every check is moved,
  the smoke test and the suite test the same page twice,
  in two languages of test code.

### What Cypress makes harder here

- **The CSP.**
  Cypress runs the page in an iframe of its runner and, by default,
  removes the `Content-Security-Policy` header,
  which carries `frame-ancestors 'none'` (`internal/server/ui.go`) and would
  forbid that iframe.
  `experimentalCspAllowList` keeps some directives,
  but not the ones that would stop the runner.
  The page would then be tested without the policy it ships with,
  and the smoke test's checks that matter under it (the scheme export as a `blob:` link,
  no `eval`) would pass for the wrong reason.
- **The page's state.**
  `cy.window().its("state")` finds nothing: `state` is a `const`,
  not on `window`.
  Reading it needs `win.eval("state")`,
  which works only because the CSP is gone,
  or a hook in `app.js` that exposes it for tests.
- **The event stream.**
  See section 4.

## 3. The alternatives

### Keep and extend the Python Playwright test

Move `smoke.py` to pytest with `pytest-playwright`:

- **Structure**:
  a test function per check group (`test_tiles`, `test_stream`, `test_cubes`,
  …), in files per area, each runnable with `-k`.
- **Fixtures** in `conftest.py`:
  a session fixture that does what `run.sh` does (build,
  start the fake Cube and the server under `env -i`, wait, stop),
  a `page` fixture with the token, the locale and the viewport,
  an autouse fixture that fails a test on a console error,
  a page error or a horizontal scroll;
  `pytest.mark.parametrize` over the widths and the languages.
- **Traces, screenshots and video on failure**:
  `--tracing retain-on-failure --screenshot only-on-failure --video retain-on-failure`
  are options of `pytest-playwright`;
  the trace opens with `playwright show-trace` or at trace.playwright.dev,
  with every action, the DOM, the network and the console.
- **Parallel**:
  `pytest-xdist` with a server per worker, on ports of its own,
  so that no test has to put things back for the next.
- **CI**:
  a job with `actions/setup-python`,
  `pip install -r tools/ui-test/requirements.txt`,
  `python -m playwright install --with-deps chromium` (the browsers cached by version),
  or the `mcr.microsoft.com/playwright/python` container of the pinned version.

Cost: nothing new in the repository but `pytest` and `pytest-playwright` in
`requirements.txt`; the 404 checks move almost line for line,
since the calls stay the same.
It stays Python.

### Move to Playwright Test in JavaScript

`@playwright/test` (1.65.0;
`playwright-core` is 14 MB unpacked) with the same Chromium:

- the test runner with fixtures (test- and worker-scoped),
  `projects` for the widths and languages (a 2 × 6 matrix is twelve lines of config),
  workers in parallel by default, retries,
  and an HTML report with the traces in it;
- `webServer` in the config starts commands and waits for a URL,
  or a worker fixture starts a server per worker;
- `expect(locator).toHaveText()` and the other web-first assertions retry by themselves,
  where the smoke test polls;
- `toHaveScreenshot()` for visual comparisons, built in (section 5);
- the UI mode (`npx playwright test --ui`) with a time-travel view much like Cypress's,
  and `codegen` to record a first draft of a test.

Cost: the same Node toolchain as Cypress (`package.json`, the lockfile,
`node_modules`), without Cypress's binary:
the browsers are the 392 MB the Python setup already has.
The tests would be in the UI's own language.
The 404 checks are rewritten, not moved, which is the most work of the three
after Cypress.

### Both Cypress and Playwright

Cypress for the flows the owner writes by hand and debugs in its runner,
Playwright for what Cypress cannot do (the CSP, a top-level page,
the locale per context).
This costs both toolchains, two CI jobs and two places to look for a check,
for a UI of ten files.
It is not worth it here.

## 4. The project's needs, runner by runner

| Need | Python Playwright | Playwright Test (JS) | Cypress |
|------|-------------------|----------------------|---------|
| Fake Cube and server per run | session fixture or `run.sh`; per worker with `pytest-xdist` | `webServer` or a worker fixture | outside the runner (`run.sh`, `start-server-and-test`) or `cy.task`; specs run one after the other |
| Event stream (SSE) | dispatch on the page's `EventSource`, as today | the same | the same, through `win.eval` or a test hook |
| CSP without inline scripts | enforced: the page is top-level with its real header | enforced | removed by default; `frame-ancestors 'none'` cannot stay |
| Single-use tickets | `page.route`, first call only | `page.route` with `times: 1` | `cy.intercept` with `times: 1` |
| Token in `localStorage` | `add_init_script` | `addInitScript` or `storageState` | `onBeforeLoad` |
| Six languages | `locale` per context, parametrized | `locale` per project | `--lang` per launch, or `navigator.languages` overridden per test |
| Two widths | `viewport` per context, parametrized | `viewport` per project | `cy.viewport()` per test |
| Hardware-free | `env -i` in the launcher | the same | the same; `CYPRESS_*` variables only |
| Trace or video on failure | `pytest-playwright` options | built in, in the HTML report | screenshots and video built in; the time-travel only in the interactive runner |
| Component isolation | every API route from fixtures | the same; component testing needs a framework | the same; component testing needs a framework and a bundler |
| New toolchain | none (pytest) | Node, `package.json` | Node, `package.json`, a 361 MB binary |

### The event stream

The page opens `GET /api/v1/cubes/{serial}/events?ticket=…` as an `EventSource`.
No runner can push events into a stream it intercepts:

- `page.route` (both Playwrights) and `cy.intercept` answer with a whole body.
  A body of `event: state\ndata: {…}\n\n` lines reaches the page,
  but the response then ends, the page's `onerror` closes the stream,
  and it opens a new one with a new ticket after 2 s.
  That is useful for the reconnect itself and nothing else.
- Playwright's `routeWebSocket` is for WebSockets, not server-sent events.
- `cy.intercept` with a response handler (`req.continue((res) => …)`) waits for
  the whole response, which a stream never finishes;
  only a spy without a handler lets the stream through.

So all three do what the smoke test does: let the real stream open
(its URL and ticket are recorded and checked),
and inject events by dispatching a `MessageEvent` on the page's `EventSource`,
which runs the page's own listeners.
In Playwright that is an evaluate in the page's global scope;
in Cypress it needs `state.events`, which section 2 says how to reach.
A truer test would have the server send the event:
a control port on the fake Cube that changes a device's state,
so that the server sends a real `state` event through the real stream.
That is a change to `massimo fakecube`, open question 3.

### The work ahead

The UI work in docs/roadmap.md and GUIDELINES/wip/HANDOVER.md adds,
in the order given there:
the pairing panel (a `paired` event and `…/pairing` routes),
backup and restore (an upload, a plan with its cost, a run with progress),
the notice when the server is gone, and later users and a login.
Each needs stubbed routes and events, which all three runners do the same way;
the server-lost notice needs the server stopped under a running page,
which a fixture that owns the server process does most simply
(a session fixture in pytest, a worker fixture in Playwright Test);
the login needs a context per user,
which Playwright's `storageState` and pytest fixtures give directly.

## 5. Screenshots as a product of the test run

The owner's Nextcloud app takes its store screenshots with a spec,
`tests/e2e/screenshots.cy.js`:
it runs only with `CYPRESS_capture=1`, puts the instance into one known state,
takes each capture under the name of the file it replaces,
and an `after:screenshot` hook in `cypress.config.cjs` moves it into `docs/Screenshots/`;
`scripts/slideshow.sh` then builds the store's animated GIF from them with ImageMagick
(each shot fitted to 940 × 700 on a 960 × 720 canvas, 3.5 s a frame, every frame
whole and opaque, checked with `identify` before the target is replaced).
A separate session is adding a screenshot job for Massimo with the existing
Python Playwright setup;
this section compares the runners and does not depend on that work.

### The set

The pages: the rooms, a device list, the configuration tabs, the log,
the settings, the appearance editor and Settings → Cubes; in light and dark,
at 1100 and 390 px, in English and German.
Seven pages, two modes, two widths and two languages make 56 pictures,
more where each configuration tab is a picture of its own.

### Reproducible data

The fake Cube's greeting fixes the rooms, devices and states.
What still moves from run to run:

- **Times**:
  the ages next to values, the Cube clock in the header,
  the server's start time in expert mode, the found Cube's `found_at`.
  Playwright's clock (`page.clock.set_fixed_time()` in Python,
  `page.clock.setFixedTime()` in JavaScript) fixes `Date` while timers still run,
  so polling goes on.
  Cypress's `cy.clock(now, ["Date"])` does the same when limited to `Date`.
  What the server reports (its start time) is masked
  (Playwright's `mask` option of `screenshot`) or answered from a fixture.
- **The serial and address**:
  the fixture's placeholders only, as everywhere.
- **Fonts and rendering**:
  the same Chromium version and the same system fonts give the same pixels.
  Taken on different machines, every picture changes;
  taken only in CI (or in the Playwright container),
  they change only when the UI does.

### Light and dark

Playwright sets `color_scheme="dark"` per context, which the page's
`prefers-color-scheme` query follows (`theme.js`).
Cypress has no option for it; in Chrome it takes a DevTools command
(`Emulation.setEmulatedMedia` through `Cypress.automation`).
In all three the page's own setting is simpler and tests the page's path:
`massimo.theme` set to `dark` in `localStorage` before the load.

### How each runner takes them

- **Python Playwright**:
  a test or script that walks the pages and calls
  `page.screenshot(path=…, full_page=…)` or `locator.screenshot()` for a panel,
  one context per mode, width and language.
  No hook is needed: the path is chosen at the call.
- **Playwright Test**:
  the same calls in a project of its own (`grep: /@screenshots/` or a separate config),
  the matrix from `projects`;
  `toHaveScreenshot()` could later compare against the committed set and fail on
  an unintended change (visual regression), with thresholds for anti-aliasing.
- **Cypress**:
  `cy.screenshot(name, { capture: "viewport" })`,
  saved under `cypress/screenshots/<spec>/` and moved by an `after:screenshot` hook,
  as the owner's config does.
  The headless window must be at least as large as the viewport,
  or the capture is scaled down (the owner's config sets the window to 1600 × 1000
  for that reason).
  Visual comparison needs a plugin.

### Where they are kept

- **Committed under `docs/screenshots/`**:
  a small set for README.md and docs/ui.md, in English, light and dark,
  at the two widths: eight to twelve pictures.
  A PNG of these pages should be 50 to 200 KB (an estimate),
  so the set is 1 to 2 MB, and every retake adds that to the history;
  so the set is retaken when the UI changes, not on every run.
- **A CI artifact**:
  the whole matrix of every run, kept for the artifact's retention,
  for review of a UI pull request without checking it out.
- **Both** is the recommendation: the artifact always, the committed set by hand
  (a `make screenshots` target) when a pull request changes what it shows.

### A slideshow

An animated GIF for the README needs the frames of one size:
the 1100 px shots fitted to a canvas as the owner's script does,
the 390 px ones either in a slideshow of their own or two side by side on one frame.
ImageMagick's `convert` (or `magick`) is what `slideshow.sh` uses;
`ffmpeg`, which Playwright downloads for its videos, could do the same.
The owner's script carries over almost unchanged,
with the frame list from a file instead of `appinfo/info.xml`.
A video of a walk through the page is an alternative:
Playwright records one per context (`record_video_dir`),
and Cypress one per spec.

## 6. Recommendation

Keep Playwright, move the smoke test to pytest with `pytest-playwright`,
and run it in CI; do not adopt Cypress for this UI.
Cypress would bring the owner's familiar runner, but here it would test the page
without its CSP, reach the page's state only around that, add a Node toolchain
and a 361 MB binary, and change nothing about the event stream,
which every runner can only inject into.
Playwright Test in JavaScript is the better runner,
and the step to take once the repository gets a Node toolchain for another reason
(a linter for the UI's JavaScript, for example; open question 1);
the pytest structure moves to it test by test.

### Steps

Steps 1 and 2 are done (2026-11-06): `tools/ui-test/` is a pytest suite,
and CI's Screenshots job runs it with `UI_TEST_REQUIRED=1`
and uploads the traces of a failure, in place of a job of its own.

1. **First pull request, `test(ui-test)`**:
   `tools/ui-test/` as a pytest suite.
   `conftest.py` with the server fixture (what `run.sh` does now,
   `env -i` included),
   the page fixture parametrized over 1100 and 390 px and English and German,
   the autouse guard for console errors, page errors and horizontal scroll,
   and traces, screenshots and video retained on failure.
   The checks of `smoke.py` move into test files by area, none dropped;
   `smoke.py` goes in the same pull request, so nothing is tested twice.
   `requirements.txt` pins `playwright`, `pytest` and `pytest-playwright`;
   `make ui-test` stays the entry point and still says so and exits 0 without Playwright,
   unless `UI_TEST_REQUIRED=1`.
   `tools/ui-test/README.md` and the contract's test gate say how to run one test.
2. **Second pull request, `ci(workflows)`**:
   a job "Web UI" in `ci.yml`: `ubuntu-latest`, `actions/setup-go`,
   `actions/setup-python`, `pip install -r tools/ui-test/requirements.txt`,
   the browsers from `actions/cache` keyed on the Playwright version,
   `python -m playwright install --with-deps chromium`,
   `UI_TEST_REQUIRED=1 make ui-test`,
   and `actions/upload-artifact` of the traces and screenshots on failure.
   GUIDELINES/project/CI.md sections 2 and 3 say why it runs and what it needs;
   the `.github/` change takes a CHANGELOG bullet.
   Since 2026-11-06 the Screenshots job runs in Microsoft's Playwright image instead,
   with no `actions/setup-python`, browser cache or `playwright install`
   (GUIDELINES/project/CI.md section 3), and a job of its own would do the same.
3. **Third**:
   the four machine-translated languages,
   a short pass at 390 px each (the page loads in its language,
   every configuration tab and settings section opens,
   no console error and no horizontal scroll), where texts are longest.
4. **Then**:
   the screenshot set of section 5 on the same fixtures (the separate session's job,
   moved under the pytest fixtures where it is not there already),
   and a server per worker with `pytest-xdist` once the suite takes more than a minute.

### What it costs

- **A developer's day**:
  the first pull request is about one day (404 checks to move,
  mostly mechanically, and the fixtures);
  the CI job two to three hours with its iterations;
  the four languages two hours.
  The same suite in Cypress would be three to five days:
  every check rewritten in its chained, asynchronous style,
  the CSP and `state` worked around, the server started outside the runner,
  and the `package.json` set up.
  In Playwright Test, two to three days.
- **CI minutes**:
  the job is estimated at 2.5 to 3.5 minutes (Go build about 40 s,
  Python and the cached Chromium about 40 s,
  `--with-deps` system packages about 30 s, the suite about 45 s).
  It runs in parallel with the others, and the longest job, "Vet and test",
  took 4 min 47 s on 2026-11-06, so the wall time of a CI run does not grow.
  The repository is public, so GitHub's standard runners cost no paid minutes.
- **On the developer's machine**:
  the venv and browsers already installed stay (80 MB and 392 MB);
  `pytest-playwright` adds under 1 MB.

## 7. Open questions for the owner

1. Is a Node toolchain (`package.json`, a lockfile,
   `node_modules` ignored) acceptable in the repository at all,
   for tests or for a linter of the UI's JavaScript?
   If yes, Playwright Test in JavaScript is the next step after the pytest suite;
   if no, the pytest suite is where the UI tests stay.
2. Should the "Web UI" job be required for merging from the start,
   or advisory for a few weeks while it proves stable?
3. Should the fake Cube get a control port that changes a device's state,
   so that a test sees a real `state` event through the real stream
   instead of one dispatched in the page?
4. May `app.js` expose `state` for tests (for example `window.massimoState`),
   or should tests keep reaching it through an evaluate in the page's scope?
5. Should all six languages run the whole suite, or English and German the whole
   and the other four the short pass of step 3?
6. The committed screenshots: which pictures, at which widths and modes,
   retaken by hand or by a workflow that opens a pull request,
   and is a slideshow GIF wanted for README.md?
7. Is visual regression (a pull request failing on a changed picture) wanted,
   which would favour Playwright Test's `toHaveScreenshot()`?

## 8. The guard of the page's parts

The page's regions are parts that are built once and updated in place
(AP UiParts), which draw from the page's data as signals.
`tools/ui-test/test_parts_guard.py` is the check that holds them to it:
data that comes in with nothing changed takes nothing away from what the
user holds.
It knows nothing of a part, so a region drawn anew fails it without a test
of its own.
It runs in two of the suite's four cases, 1100 px in English and 390 px
in German: each width and each language once.
In the other two its tests are deselected as the suite is collected,
by the mark `cases` that `GUARD_CASES` gives the file (`conftest.py`),
so that none loads a page only to be skipped:
the 28 skips of 1100-de took 42 s of the guard's worker, under load,
before the mark (AP TestSpeed, D9-A).

### What it checks

The file's table, `REGIONS`, cuts the page into its regions:
the header's Cube choice, its problems list and ☰ with the facts,
the home notice, the room tiles, a tile's configuration, the House tile,
the devices without a room, the Cubes list, Cube settings, pairing,
users, roles, capabilities, providers, integrations, backups, the log,
software update, maintenance, system info, my settings, appearance,
notifications, my profile, the page's chrome, the sign-in dialog,
the first owner's form in it with its step that keeps the server open,
the open server's warning and Users on a server without users (AP FirstOwner),
and ↑.
Each has the CSS selectors of its containers,
what inside them belongs to another region,
and its views: how a user brings it on screen and opens what it has to open,
such as a tile's ⋯ and More menus, its problems list, its holiday form,
a device list with the virtual thermostat's settings in expert mode,
each configuration tab, a Cube's panel, the pairing form,
a user's panel with the new user's form, the scheme editor.

In each view, around each data source applied with the data as it was
(`poll(false)`, `loadCubes()`, `loadRooms()`, `refresh()`,
a `state` event per device on the page's `EventSource` as the server sends it,
`MassimoParts.log.loadAlerts()`, the page's minute
(`MassimoParts.minute()`), the log's `MassimoParts.log.poll()`,
the settings applied again, `applyStoredSettings()`, and who is in read again, `loadMe()`,
every one in every region, since every part follows the data it reads, UiParts D221),
and around the region's own (`Region.sources`):
the header's check for an update, `MassimoParts.software.load(false)`,
the page shown again, the `viewshown` event its loaders take,
and the stream's server events of the kinds a region follows,
dispatched on the page's stream, a forward guard for the three regions
(the plain page read nothing for these events),
after each of which the guard waits until the page's requests have answered,
and a round whose requests had not answered within 5 s fails with that as its reason,
with the timer's own poll stopped:

1. **Identity:** every element a user can hold that the region shows
   (a button, a field, a select, a summary, a link, a `details`,
   anything with a `tabindex`) is the same element, still in the document;
2. **Open:** every `details` that was open is open;
3. **Typed:** every field that takes text, a number, a date or a time holds
   the value typed into it, valid for its type;
4. **Focus:** the element of the region that had the focus has it;

and 1 to 3 once more after the focus has left the region,
where a redraw put off while the user was in it comes.
Typing sets the value and sends an `input` event, as a key does;
no `change` event, no Enter, no click on a button, so nothing is saved.
A failure names the region, the view, the data source, the check and the
element, by its path from the nearest ancestor with an id.

The guard sends nothing:
every request to the API other than a GET is refused and fails the test,
so nothing reaches the fake Cube and nothing changes in the server's store.
Two things the page itself does as it loads are not the guard's:
its first sync of the settings to the server, which it schedules because
conftest removed them, is dropped;
and the event stream's ticket of a page loaded during a view
(the home notice's, on the second server) is let through and named in
the test's output.

### How the regions came to pass it

While the regions moved, a list, `NOT_MOVED`, named those not yet moved that
failed the guard, each with a line on what it lost, and skipped them;
it went with the move's end (AP UiParts, Block 15).
On 2026-11-15 it held the room tiles, a tile's configuration,
the House tile, the Cubes list and Cube settings, which lose what the user
holds at a state event, a rooms reload or a poll
(Cube settings its head's problem pill, which the fixture's open window
brings once it is five minutes old),
and the three regions the fixture cannot show:
the header's problems list (the fixture's Cube has nothing wrong the header
lists before that time, and a test cannot set it),
the devices without a room (one joins only through pairing, a write to the Cube)
and the sign-in dialog (a page without a token loads no data while it is open).
The other regions passed although none had moved:
they are filled in place, or drawn again only by the user's own action,
which the guard does not apply.
The Cubes list and Cube settings left the list with their move to parts
(AP UiParts, Block 5), the header's problems list with the header's
(Block 6), with a view of the list open and one of the Cube choice open
with the Cubes found, the room tiles and the House tile with theirs (Block 7).
The devices without a room left it with the rooms too: their view answers
`GET …/devices` with the server's answer and one device without a room
(`UNLISTED_STUB`), and opens its form that puts it into a room.
The sign-in left it last (Block 14): its views open its card on a page signed in,
as a 401 opens it, and its sources are a second `askLogin`, the providers read again,
the language drawn again and who is in read again;
the first owner's form, a region of its own, shows by a stub of `GET /api/v1/me`.
The list was empty since.
A tile's configuration left it with its own move (Block 8), in a view per tab;
before, a data event filled it again, but while the focus was in it,
and once the focus left, the fill put off came: every element was new,
and what was typed and not yet taken by a `change` event was gone.
Since the fixture has nothing in a Cube's problem list from the start and
finds no Cube without a search, which is a POST, their views answer
`GET /api/v1/cubes` with the server's answer and, the same at every load,
a device with a low battery, a line of the settings file that differs, an
address found elsewhere, a proxy that does not listen, and a Cube and a
Massimo proxy found (`CUBES_STUB`, `FOUND_STUB`): a region whose fixture
lacks what it shows can be given it so, with GETs alone.
The pairing's second view does so for a pairing that runs (AP UiParts, Block 9):
`GET …/pairing` answers it as running for ten minutes more,
with a device that joined (`JOINED_STUB`), whose form holds a room chosen,
since a pairing starts only by a POST.
The repairs' views, a room's box under its tile, a device's under its row
and the Cube's in its panel, each with a question open,
answer every `GET …/repair/…` with a preview that has something to send (`REPAIR_STUB`,
by `RepairStub`),
a listing's as of a device the Cube's list lacks,
so that the device's box asks for its name;
a run is a POST, which the guard refuses.
System settings' lists (AP UiParts, Block 10) are checked with a panel open,
every field of its form typed, a question asked in its place and the new item's
form beside it: a user's Remove asking; a provider's panel, `GET /api/v1/providers`
answered with one provider off (`PROVIDER_STUB`), since the fixture has none;
a request and an integration with a user and the choice of whom it acts as, and
an installation seen on the LAN with Register's address asked for,
the integrations' reads answered so (`REQ_STUB`, `IG_STUB`, `IG_USER`, `FOUND_IG`);
a kept backup's Delete asking on Cube settings, run.sh's backup taken by hand.
These views passed on the page before the move as well, since none of the
guard's sources draws the lists again: from now on they guard that a poll, a
state event or a reload of the Cubes and the rooms leaves them as they were,
and a comment beside each says so;
the reads of their own (an action in another panel, the list read again,
the page shown again) are `test_admin_updates.py`'s.
Software update (AP UiParts, Block 12) is checked with Update asking in the section
and the channel's choice above it, the focus on it,
`GET /api/v1/update` answered with an update offered (`UPDATE_STUB`),
around the header's check and Maintenance shown again as well;
it passed on the page before the move too, as its comment says,
since `update.js` filled it in place but for a select's options and the last updates' lines,
which nobody holds: what it lost is `test_maint_updates.py`'s.
Maintenance is checked with the restart asking in its row and a delay typed for the reboot,
`GET /api/v1/restart` and `GET /api/v1/reboot` answered as on a box with a service manager
(`RESTART_STUB`, `REBOOT_STUB`), around Maintenance shown again as well;
it passed on the page before the move too: `maintenance.js` lost the focus of a setting
while it saved, which `test_maint_updates.py` shows.
System info is checked with its folds open, the credits' and Preact's licence among them,
around the page shown again as well,
which failed on the page before the move:
`system-info.js` made the version's and the proxies' copy icons and the proxy's link anew
at each read.
My profile (AP UiParts, Block 13) is checked around the personal page shown again as well,
which failed on the page before the move:
`profile.js` made the display name's form and the sessions and keys anew at each show.
My settings are checked around the settings applied again as well, as when the user's
arrive from the server at a sign-in (`applyStoredSettings()`),
which failed on the page before the move: `applySettings` set the poll typed back;
the Appearance card too, with the scheme editor open,
which failed as well: `applyAppearance` made the editor anew.
Notifications are checked around the personal page shown again,
which failed on the page before the move: `notify.js` made the rules and the devices anew.
The page's chrome (AP UiParts, Block 14) is checked with ☰ open, the focus on an entry,
and with the personal page's sections menu open, the focus on an item,
each around who is in read again as well (`loadMe()`), as at a sign-in;
the personal page's failed on the page before the move, where `pageBar` emptied the menu
at every read of who is in.
System settings' sections menu open,
and a card folded and opened again, the focus on its fold,
passed on the page before the move too, where nothing drew a card's head again.
The log (AP UiParts, Block 11) is two regions here:
its section with its head, checked as it is and with the kind's choice in focus,
which passed on the page before the move too, where nothing built the head again;
and its lines, `#log-lines`, which no round types into
(the search typed filters every line out),
checked with the log's reads of the Cube answered with the log's lines of
`helpers.py` (`StubLog`, which `test_log_updates.py` uses too),
two of them problems not yet seen
(`GET /api/v1/log/alerts` answered so) and a command's line with its detail:
a problem's Mark seen in focus, and the command's detail open with the focus
on its button; both failed on the page before the move,
where `loadRooms()` and `refresh()` built every line again.
Its stubs are taken away once its views are done (`Region.after`).

### A region that changes

A block that changes a region, or adds one,
adds a view where the part has something to open that the table does not
open yet, and corrects the region's selectors where the part changed them;
a new region gets its line in `REGIONS`.
One region alone runs with
`PYTEST_ADDOPTS='test_parts_guard.py -k <region>' make ui-test`.
The guard bites: with the room tiles and the Cubes list checked,
it failed on the page of 2026-11-15, the tiles at every state event and
rooms reload, the Cubes list's closed panels at every poll.

## 9. The page's own reads and its stream

The page takes its data anew at every read:
its poll (every 10 s by default, `startPolling` in `app.js`)
reads the Cubes, the server's health and the log's alerts,
and events and actions start reads of their own
(`loadCubes()`, `loadRooms()`, `refresh()`).
Data a test put into the page alone is gone at the next one,
and a line of the server's comes into the log among the test's.
The suite's helpers for it are in `tools/ui-test/helpers.py`
(AP TestInfrastructure, Block 7):

- **The page's reads answered** (`Reads`, the fixture `reads`):
  what a test tells it is in the server's answer from then on,
  every GET of the path answered with the server's JSON as the test changes it,
  until the test clears it or its page closes;
  other methods go on to the test's other routes or the server.
  `reads.cube(serial, **fields)` puts fields into a Cube's entry of
  `GET /api/v1/cubes`, in place of those told before;
  `reads.found(cube)` lists a found Cube, once per serial;
  `reads.change(pattern, fn)` changes any read; `reads.clear(pattern)` ends it.
  The test then shows it by a read of the page's own (`loadCubes()`),
  and the page's poll keeps it.
  A test does not set `state.cubes`, `state.found` or `state.version` in the page:
  the next poll takes them back,
  and a version other than the server's makes the page reload.
- **The poll held** (`with held_poll(page):`):
  a test that measures what the page does by itself,
  a count of its drawings or times that stand still,
  holds the page's poll while it measures,
  since a poll draws again and brings new times,
  so that what it measures does not depend on the poll's period:
  the drawings of an unsaved form (`test_unsaved.py`)
  and the pile's times (`test_header.py`).
  The hold clears the poll's timer and starts the poll again at its end.
  Whatever calls `startPolling()` meanwhile ends the hold unseen:
  `applySettings()` (a setting changed, expert mode, a sign-in),
  the poll's field, a sign-out;
  so the hold fails at its end where the timer is no longer the one it cleared.
  What a poll itself draws the test counts for one poll of its own (`poll(false)`),
  once every request it started has been answered.
- **The same element** (`mark`, `replaced`, `frames` of `helpers.py`):
  a test that holds that data takes nothing from the user marks the elements
  of its selectors, lets the data come, waits two frames and a task,
  and asks which selectors no longer find the element it marked
  (`test_pairing_updates.py`, `test_repair_updates.py`).
- **The repairs' routes** (`RepairStub` of `helpers.py`, by `safe_route`):
  a test's previews by repair, or a function of the query,
  and with `runs` the runs kept and answered;
  the guard's without, so that a run meets its refusal.
  `state_events` sends a state event per device as the server does,
  the script the guard's round sends too (`STATE_EVENTS_JS`).
- **What each source reads** (`test_data_updates.py`):
  on every page, shown and come to rest with the poll held,
  each data source and the stream's events that make the page read are applied once,
  and the requests they cause, of every method, path by path, are its `TABLE`,
  taken on the page before the data became signals (AP UiParts, Block 15):
  a part's effect that read again at each write of the data it follows fails it.
  What each showing of a page reads is `test_shell_updates.py`'s.
- **What the reads do not keep yet:**
  a device's state applied in the page (`updateDevice`, a dispatched `state` event)
  lasts until the rooms are loaded again.
  The poll does not load them while the stream is open,
  but the events that start a load do (`connected`, `metadata`, `unconfirmed` and others),
  among them those the server sends by itself.
- **The stream watched** (`Stream`, the fixture `stream`):
  every page of the page fixture has `PAGE_KIT` before its own scripts,
  which records each event the page's listeners take,
the server's and the test's, in order,
  with how many times the page handed it to the log (`MassimoParts.log.event`).
  `stream.mark()` says "from now on";
  `stream.wait(what, kind, match=…, mark=…, server=…)` returns the first match from the mark
  and skips the rest, and on a timeout says what did come since the mark.
  A mark holds within one document.
- **A log of the test's own lines** (`isolate_log(page, reads)`):
  the lines of the events the server sends are kept out of the log
  (the page's other work on them goes on),
  and the log's reads of the Cube are answered with no line,
  so that the log starts empty and `.first` of a kind is the test's;
  a test that answers the log's reads itself gives no `reads`.
- **The log live** (`wait_log_live(page)`):
  the page leaves a stream's first 1.5 s out of the log,
  the burst of states a connection starts with,
  and only for `state` events.
  A probe that the log never shows is sent until the page hands one on to the log,
  in place of a fixed wait of 1.6 s.
- **One safe route** (`safe_route(page, pattern, handler)`):
  on Playwright's error in a handler the request is aborted:
  where the page closed with it on its way or it was answered already,
  the abort fails too and nothing more happens;
  where the page still waited for it, as when the server did not answer `route.fetch()`,
  the abort goes to the guard, which fails the test.
  Anything else the handler raises goes to the guard as well, and the request is aborted.
  The guard's problems are printed as they come,
  and a test that fails first carries them in its report.
- **Waits and cleanups:**
  a wait that returns whether it was met is asserted where it is called
  (`assert wait_until(…)`, `assert scroll_settled(…)`, `assert wait_held(…)`),
  so that a timeout fails there and not three steps later;
  a cleanup's step is `undo(what, call)`,
  which takes a 404 (or the statuses given) for nothing to undo
  and fails the test on anything else, naming what it could not undo;
  `cleanup(*steps)` runs every step of a cleanup, one that fails among them,
  and raises the first error once all have run.
- **Nothing came, against an end:**
  a check that an action brought nothing has an end of its own,
  not a fixed wait that a slow runner outlasts:
  a read the page sends after the action through the same route,
  which a request the action sends before it waits for anything comes before
  (`test_backup.py`: its POST goes out in the click,
  before the first `await` of `whileBusy`);
  the page's own state that says nothing waits to be sent,
  a timer it has not set (`test_preferences.py`);
  a count of the requests the action started, taken once its answer is drawn
  (`test_users.py`);
  or a timer of the page's own that is due after the one it rules out (`test_header.py`).
- **The test's own line:**
  a line of the server's that the test's action made is found by what the
  stream's event names, its command's number,
  not by `.first` of its kind:
  the log holds the server's earlier lines of that kind,
  the four cases' among them (`test_log.py`).
- **The server sets** keep the server's 10 s poll of the fake Cube,
  which brings no line of its own (the fake Cube's states do not change).
  What changes is their age:
  the window contact's window, open from the start (`greeting.txt`),
  counts as open long five minutes after its set started,
  and as open too long after ten (`window_warn_after`, `window_alert_after`),
  so from then on the Cube's problems list MEQ0000001, its panel's pill too.
  A test that reads a Cube's problems from the server holds whether they list it or not;
  one that needs none tells `reads`.
  They take no automatic backup during the run:
  a Cube without a backup gets one a minute after the start,
  whatever `backups.auto_min` and `backups.auto_max` say (`internal/server/backups.go`),
  so `run.sh` takes one by hand as soon as each set is up.
  With it the Cube is clean,
  and an automatic backup waits for a change of its configuration and then `backups.auto_min`, 60 minutes;
  a test that deletes the newest backup leaves the one taken at the start,
  so the Cube does not count as one without a backup.
  This is quiet, not off: no setting or flag of `serve` turns them off.
