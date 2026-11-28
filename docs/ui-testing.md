# Testing the web UI (v1.41.0)

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
  An event a test gives the page whose data a server's read would hold
  is told to `reads` as well, so that the page's next read keeps it:
  a device that joined by `pairingJoined` is in the pairing's read from then on
  (`test_pairing_updates.py`, `joined`), where a read after the event took it back:
  with a pairing read right after the event, the device told to the page alone
  was gone in three runs of three, and told to `reads` too stayed in three.
  A test does not set `state.cubes`, `state.found` or `state.version` in the page:
  the next poll takes them back,
  and a version other than the server's makes the page reload.
  A test that holds the poll and writes a field of a Cube
  writes `state.cubes` whole, as the page's loads do,
  never the field in place, which the page may not draw again
  (`test_header.py`'s `SET_READ`).
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
  once every request it started has been answered (`answered`).
- **A timed state held** (`with held_timers(page, *kinds):`, AP SteadyTests, Block 13):
  a test that checks a state the page takes back on a timer of its own,
  or that must come before a timer of the page's, holds it while it checks,
  rather than fitting its steps into the timer, which a slow runner
  or the retry's trace outlasts:
  "Copied" on a copy control (`copied`, 1400 ms, `copied()` of `components.js`),
  which a copy, the page shown again and its reads did not fit in
  (`test_maint_updates.py`);
  the toast (`toast`, 8 to 30 s), checked after an axe run,
  or taken back by its ×, Show or a probe, which its own time would take back too
  (`test_notifications.py`, `test_discover.py`, `test_connect.py`);
  the settings' push (`push`, a second after a change or a dismissed sign-in,
  `schedulePreferences` of `login.js`),
  which a test's later read must come before (`test_sign_in_resend.py`).
  The page sets its timer as always and an observer clears it at once,
  so nothing else of the page stops: a later copy, a × on the toast,
  a sign-in that takes the push go as they do;
  at the end what was held goes at once, as its timer would have by then.
  A look right after the wait that saw a state, within its time, holds nothing,
  nor a check of the fade itself.
  The hold is within one document: a block whose page was loaded anew fails at its end.
- **The same element** (`mark`, `replaced`, `frames` of `helpers.py`):
  a test that holds that data takes nothing from the user marks the elements
  of its selectors, lets the data come, waits two frames and a task,
  and asks which selectors no longer find the element it marked
  (`test_pairing_updates.py`, `test_repair_updates.py`).
- **One moment for several boxes** (`boxes(page, *targets)`, AP SteadyTests, Block 13):
  a check that compares where elements are reads their boxes at one moment,
  in one `page.evaluate`, not with a `bounding_box()` each:
  the page may draw between two calls,
  as the server's poll drew a device's values above the pairing form
  between the form's two reads, and the limit, read second,
  came out 78 px below the box it sits above (`test_pairing.py`).
  Each target, a selector or a `Locator`, is found as `bounding_box()` finds it,
  one element, waited for, and its box is the one `bounding_box()` gives,
  `None` for an element not drawn;
  where one was drawn anew between its finding and the read, all are read again.
  A box compared with nothing else the page draws,
  the place of a click or the window's width, is read alone.
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
- **What the reads keep** (`reads.keep(rf)`, AP ServerTraffic, D3-A):
  a device's data told through the rooms' read lasts until the server's next
  `state` or `config` event of the device, which brings it as the server has it
  and starts no read;
  the server sends them by itself, at its poll after an earlier test's write
  and at a follow-up's re-read of a C line, when no test can time them.
  With `keep`, the page reads its rooms again after each such event of the server's,
  once its own listeners have drawn it,
  so that what the test told is drawn again.
  Until that read is answered the server's data stay drawn,
  so a check of what was told waits for it (`wait_until`, `wait_for`)
  and does not look once (`test_new_batteries.py`, `Device.shows`).
  `PAGE_KIT` does it on every stream of the page, a reopened one too;
  an event a test dispatches is not the server's and starts no read,
  so a test that counts the rooms' reads after its own events still can
  (`test_room_marks.py`).
  It holds within one document, as a mark does,
  and `reads.clear` of the rooms' read forgets it.
  A test takes it where it tells a device through the rooms' read
  and then checks what it told while the server may speak:
  `test_confirm.py`, `test_room_marks.py`, `test_new_batteries.py`.
  Nothing keeps a device's state applied in the page alone,
  by `updateDevice` or a dispatched `state` event:
  it lasts until the rooms are loaded again.
  The poll does not load them while the stream is open,
  but the events that start a load do (`connected`, `metadata`, `unconfirmed` and others),
  among them those the server sends by itself.
- **What a test tells of a device or a room**
  (`reads.device(rf, **fields)`, `reads.room(rid, **fields)`, AP ToldData, D2-A):
  a device's fields, those of its `state` among them,
  or a room entry's, its presets, its `thermostat`
  and its virtual wall thermostat's `sources`,
  are in the rooms' read (`ROOMS`) from then on,
  each in place of its value told before, and a `None` takes a field out.
  `reads.device(rf, like=…)` tells of a device the server does not list,
  a twin or a wall thermostat:
  a copy of the server's entry of `like`, at the end of its room's devices
  or of those of the room `room_id` names.
  Both keep what they tell (`keep`):
  the device, or the room's thermostats and its virtual wall thermostat,
  so that it comes back after the server's own events of them
  and after any read of the page's own.
  The page shows it at its next read:
  the test reads the rooms (`loadRooms()`),
  or puts the same into the page itself where that is the event under test,
  and a check after it waits:
  `wait_until`, or `settled(page, look, ok)` where a look takes several calls
  (`test_room_marks.py`'s `Told`, `test_rooms.py`).
  A read that the patterns of several changes match takes each of them,
  so a test's own change under another spelling of the pattern goes with them
  (Playwright runs the later of two routes alone).
- **How old the values are** (`reads.age(rid, seconds, already=False)`,
  AP SteadyTests v1.1, Block 14):
  the server received the fake Cube's values when it connected,
  at its set's start, and the page counts their age from then,
  so an age grows with the run,
  "≥ 7 Min." in a short one and "≥ 17 Min." in a long one,
  and with it the width of what shows it.
  A test whose look depends on an age, its text or the width it takes,
  tells it: every value of the room's devices and of its virtual wall
  thermostat received `seconds` before the page's now,
  in the rooms' read (`reads.device`, the devices kept),
  and with `already` a value that was there when the server connected,
  which the page marks "≥",
  by the Cube's `observed_since` (`reads.cube`, beside its other fields told).
  It starts the page's reads and returns the age as the page says it,
  which the test's look waits for, a look at a time (`settled`),
  since a server's own event of a device draws the server's age
  until keep's read is answered:
  `test_rooms.py`'s device list, at two digits and the longest of each unit,
  `test_room_more.py`'s holiday tile, at two digits as a server's are all day,
  and `test_an_age_moves_on_with_the_minute`'s age an hour on,
  which says the same from two days on.
  A whole run with every age at its widest found no other test that depends on one.
  A test that tells the rooms' read a device's times itself
  tells its `received` the same way (`test_stale_and_expert`'s 30 hours).
- **A fold opened** (`open_fold`, `close_fold`, `toggle_fold` of `helpers.py`):
  a `<details>` fold's `toggle` event comes in a task of its own after the click,
  and a part that starts anew at it drops what the test did inside the fold before it:
  `#me-password` and `.user-panel` start their password fields anew.
  The helpers click the fold's summary and take that event,
  and the part's drawing after it, before the test acts inside the fold;
  a click that toggles nothing fails within seconds (`FOLD_TOGGLE`),
  not at the test's end.
- **A copy checked** (`grant_clipboard`, `copies`, `icon_after_last_line` of `helpers.py`,
  AP KeyLife, Block 1):
  `grant_clipboard` lets the page's origin read and write the clipboard,
  and empties it;
  `copies(page, sel, value, name)` checks a shared `Copy` with its icon,
  `sel` its text and the icon the button right after it:
  a button to the keyboard,
  it and its icon named `name`, which does not tell `value`,
  and a click on it, Enter and Space on it and a click on its icon,
  each copying `value`, the clipboard emptied before each;
  `icon_after_last_line` says whether its icon follows the text's last line,
  the lines measured by a range over the text,
  which gives them where a flex item made the text a block, too
  (`test_users.py`, `test_settings.py`).
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
- **The scale** (`scaled(seconds)`, `UI_TEST_SCALE`, AP SteadyTests, Block 3):
  each wait of the kit lasts its seconds times `UI_TEST_SCALE`,
  a positive number, 1 where it is not set,
  as `MASSIMO_TEST_SCALE` does for the Go tests:
  `wait_until`, `settled`, `wait_held`, `scroll_settled`, `Stream.wait`,
  `ApiReads.answered`, `wait_log_live` and the folds' helpers,
  and the page fixture's default timeout and its waits for the tiles and the stream.
  So a call site gives the time a thing takes on an unhurried machine,
  and a slow or loaded runner sets more.
  `run.sh` refuses a scale that is not a positive number,
  and its retry, which runs with the trace,
  runs at four times the scale given:
  with the trace, a test's set-up took up to 3.8 times as long (2026-11-26).
  A window that waits for nothing to come is not scaled:
  `ApiReads.answered`'s quiet time, `scroll_settled`'s still frames,
  a fixed `wait_for_timeout`.
  A test's own loop ends at `scaled(seconds)`,
  as `wait_count`, `back` and `wait_reads` do;
  a `timeout=` handed to Playwright is not scaled by itself.
- **The page's start** (`wait_started`, AP SteadyTests, Block 3):
  the page fixture and `owner_page` wait 30 s (scaled) for the room tiles,
  longer than the page's own limit of a read (`API_WAIT_MS`, 20 s):
  a read the server answers late is the page's to give up,
  and the page then says so on its message line.
  On a timeout the failure names the API reads still open (`ApiReads`)
  and the message line, so that it says which read the start waited for.
  What a reload brings is waited for too,
  not looked at once when the page is shown:
  the page's start goes on after it (`test_header.py`).
- **The page's reads all answered** (`answered(page, held=…)`, AP SteadyTests, Block 4):
  the one wait for a page that has read all it reads now,
  before a test counts what an action read ("one read", "nothing read")
  or checks what the answers drew:
  every read of the page's own, to the API and to `/healthz`, has its answer,
  but those of the routes the test holds (`held`, a list it may add to meanwhile),
  and for 200 ms none has started or ended and the page's height stood still.
  A read the page starts only after another has answered,
  the next of a burst (`shared/burst.ts`), the next of the page's start
  (`app.js`, `init`) or a load an answer sets off,
  comes within those 200 ms and is waited for in turn.
  The waits before it did not follow one:
  a request of the test's own that overtakes the page's
  (`test_maint_updates.py`'s `fetch('healthz')`) returns before the next read starts,
  and `ApiReads.answered`, which looked at the reads in flight at the start and the end
  of its quiet time, let a read start and end inside it;
  of four reads chained 150 ms after each other's answer,
  they returned after none and after one, the new wait after all four
  (three runs each, 1100-en, 2026-11-26).
  It reads the `ApiReads` the page fixture and `owner_page` make before the page's start
  (`api_reads`), measures its quiet time from the test,
  so that a page under a paused clock is waited for too,
  and fails naming the reads still open;
  the 200 ms are not scaled, its ceiling is.
  `ApiReads.answered` is the same wait, for a test that makes its own.
- **The request itself** (`wait_held`, `sent_requests`, AP SteadyTests, Block 13):
  a check of what a request carried, or that a write was not sent again,
  waits for the request, not for what the page draws before it sends it,
  nor for a fixed pause:
  the automatic update's window comes up before its setting's PUT (`test_update.py`),
  and the page pushes its settings a second after a sign-in,
  after the stream is open, the moment a test of a sign-in ends at otherwise
  (`test_sign_in_resend.py`);
  a request a test stubs reaches its stub before the test goes on and its route goes,
  or it reaches the server (`test_admin_updates.py`'s restores).
  `wait_held(page, sent, n=…)` waits until the list holds n requests:
  one a route of the test's fills as it answers (`hold`, a stub's own list),
  or `sent_requests(page, method, pattern)`,
  the page's requests as it sends them, for one that no route of the test's notes.
  A list a route fills is read after the wait, not after Playwright's
  `request` event of it, which comes before the route runs.
- **The answer taken** (`taken(page, predicate)`, AP SteadyTests v1.1, Block 13):
  a check of what an answer drew, or that it drew nothing,
  and a step the answer must come before,
  wait until the page has read the answer's body, not for the answer alone:
  Playwright's `expect_response` returns once the answer's head has come,
  and `.finished()` once its body has, on Playwright's side;
  the page reads the body later and goes on with it in that turn
  (`app.js`, `apiRequest`), its parts drawing what it brings.
  So a test's next step overtook it:
  `test_pairing_updates.py` told the page a device that joined
  right after the answer of a pairing read,
  the page then took that answer, which listed the devices without it,
  and the device was gone before Playwright looked
  (1100-de on CI, 2026-11-28; it passed in the retry).
  With the page reading that answer's body 150 ms after its head
  and the test looking 300 ms after the join,
  the old wait failed three runs of three and `taken` passed three (1100-de, 2026-11-28).
  `taken` is `expect_response` for a block, with its answer as `.value`,
  and at the block's end a wait until the page has read that answer's body:
  `TAKEN_KIT`, run at the block's start, wraps `Response`'s body methods
  in the document, each noting its answer's URL once the body is read,
  before the page's code that awaited it goes on in the same turn,
  so that what the answer draws is drawn by the test's next call
  (a part's effect after it by `frames`).
  An answer of the same URL whose body the page reads after the block began counts too.
  It holds within one document.
  Every test that waited for an answer as the page having it takes it,
  `.finished()` and `frames` ("its body read by the page too") among them:
  `test_admin_updates.py`, `test_integrations.py`, `test_log_updates.py`,
  `test_login.py`, `test_maint_updates.py`, `test_pairing_updates.py`,
  `test_personal_updates.py`, `test_sign_out_late.py` and `test_system_info.py`.
  A test that waits for what the answer drew (`wait_until`, a locator's `wait_for`)
  or for the page's reads (`answered`) needs none,
  nor one that reads the answer itself (`.value.json()`).
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
- **The old idioms counted** (`tools/ui-test/idioms/`, AP SteadyTests, Block 4):
  a Go test, as `tools/identifiers` is, so that `go test ./...` fails a new use,
  counts per test file three ways of doing what the kit does once:
  a fixed pause (`.wait_for_timeout(`), which outlasts nothing on a slow runner
  and waits for nothing in particular;
  a plain route (`.route(` of a page or a context, not `safe_route`),
  which races the page that closes with a request on its way;
  and a file's own definition of a name `helpers.py` defines at its top level,
  a function or a class, a copy that drifts from the kit's.
  Each has its list beside the test, a test file and its count a line:
  `wait_for_timeout.txt`, `page_route.txt` and `kit_names.txt`.
  A count may fall and may not rise.
  Above its listed one, none listed being 0, the test fails,
  naming what to use instead and this section;
  below it, the test fails too, asking for the line to be lowered to the new count
  in the same commit, so that the count cannot rise back to it,
  and taken out at 0; a line of a file that is gone is taken out.
  So a list holds what the files had when the guard came,
  120 fixed pauses in 34 files, 177 plain routes in 45 and no copy (2026-11-26),
  and falls as files move onto the kit.
  A helper several files need goes into `helpers.py` under a name of its own,
  and a file's helper that only shares a name with one of the kit's takes another;
  the files' copies went so before the guard came:
  `loaded` of `test_cube_updates.py` and `test_header_updates.py` is `load_cubes`,
  their `mark` the kit's, with `same_and_open`;
  `stub_me`, `open_cube_panel`, `same_box` and `scrolled_to` are the kit's,
  and `test_open_banner.py`'s `stub_me`, another thing, is `stub_mode`.
  A constant is not counted: a test file's `ROOMS` or `STALE` is another thing
  than the kit's of the same name.
  `go test ./tools/ui-test/idioms` runs it alone.
- **The page's state written** (`state_writes.txt`, AP SteadyTests, Block 6):
  the same test counts per test file the writes of the page's state in its page scripts,
  against a list of its own that falls as the others do:
  a field of `app.js`'s `state` assigned
  (`state.x = …`, `state.x.y += …`, `state.x[k] = …`),
  incremented or decremented, deleted, changed in place by an array's method
  (`state.x.push(…)`), or the state or a field of it as `Object.assign`'s target.
  A read (`state.x === …`) is none, nor a field of another object's state
  (`themeState`, `MassimoParts.log.state`), nor a Python dict named `state`.
  Data put into the page alone lasts until its next read or event
  (above, "The page's reads answered"):
  a test tells the page's reads what it wants shown,
  through `Reads` (`reads.cube`, `reads.device`, `reads.room`, `reads.change`),
  or the fake Cube, and its failure says so and points here.
  The list held 39 writes in 13 files when it came (2026-11-26),
  20 of them expert mode turned on or off for the page alone (`state.settings.expert`).
- **The server sets** run without the server's poll of the fake Cube
  (`serve -poll 0`, AP SteadyTests, Block 5, D4-A):
  the server sends its fake Cube no line of its own between a test's steps,
  and learns what the fake holds only where it connects or a test asks
  (below).
  With the poll, every 10 s, the device list and the events it brought
  fell into whichever test ran then.
  So the poll's interval in Maintenance is read-only in a UI run,
  set by `-poll`, Off (`test_cube_intervals.py`),
  and a test that chooses an interval chooses the refresh's.
  Their fake Cube has every memory slot free (`-free-slots 50`,
  AP ServerTraffic, D1-A):
  with the fake's default of one radiogram pending,
  the server reconnected to read its budget two minutes after every write,
  and the reconnect's burst,
  `metadata`, a `config` event per device and `connected`,
  took back what a test had told and read the page's data again,
  in whichever test ran then.
  A test that wants a reconnect plants one (`POST …/cubes/<serial>/refresh`);
  the structure checks' reconnect, twelve minutes after a change,
  and the refresh every six hours stay, and fall outside one test's time.
  What changes is their age:
  every value's, counted from the set's start (`reads.age`, above);
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
- **A test file starts from the recorded Cube** (`known_cube`, `conftest.py`,
  AP SteadyTests, Block 5):
  before a file's first test the worker's fake Cube goes back to the greeting
  it started with (`greeting.txt`) through its control's `POST /reset`
  (`massimo fakecube -control`, on a port of its own, `UI_TEST_CONTROL_PORT`):
  its devices, their states, its rooms and M lines,
  no device joined by pairing,
  and nothing an earlier file sent applied after the reset.
  The reset drops the server's connection;
  the fixture waits until the server has connected again and read the greeting,
  a `connected` event in the Cube's log newer than the one before the reset,
  and the Cube listed connected,
  for 15 s, scaled; on a timeout it names what it waited for and what it saw.
  A reset took 1.4 to 1.9 s (2026-11-26, six files),
  most of it the server's wait of a second before it connects again.
  So no earlier file's write, pairing or room list reaches a file,
  whichever worker runs it after whichever file.
  Before the reset, and after a file's last test, no write is on its way
  (`writes_shown`):
  where the server still waits for a device to show a write
  (a device's or a room's `confirmation`),
  the fixture reads the Cube again (`POST …/refresh`),
  whose greeting shows what the fake applied at once,
  and waits until none is left, for 20 s, scaled;
  one still on its way fails, naming the device or the room and the write's fields.
  Without the poll a setpoint is shown only at a connect,
  and a reset that came first would take it back:
  the server would count it as not confirmed five minutes later
  and send a room's setpoint again, into a later file.
  No file of the suite left one (2026-11-26, 1100-en, 75 files);
  a planted file that ended with a setpoint on its way
  left it pending into the next file without the refresh, and none with it.
  Within a file the tests share the fake Cube, the four cases among them:
  a test that changes the fake still puts it back.
  What the server keeps in its store, users, settings, backups,
  the reset does not touch:
  a file leaves them as it found them, which a check after its last test holds (below).
- **A test that needs a read asks for it:**
  where a test needs the server to read what the fake Cube now holds,
  a write the fake applied, it asks:
  `POST …/cubes/<serial>/refresh` reconnects and reads the whole greeting,
  the device list among it, and answers once the server has it;
  the page then learns of it by the events of the reconnect.
  No test waited for the server's poll when the suite first ran without it
  (2026-11-26, 1100-en): the five that failed chose the poll's interval,
  which `-poll` makes read-only, and choose the refresh's now.
- **The warnings seen** (`clear_preferences`, `conftest.py`, AP SteadyTests, Block 11):
  before every page of the page fixture the token's user's preferences go,
  its settings of the page, its schemes, its notification rules and its marks of the log,
  so that a test starts from the defaults as a fresh browser does,
  and then every warning of the log the server holds is marked seen
  (`PUT /api/v1/me/preferences/log.seen`, `all` the newest alert's id).
  A warning stays until a later event settles it or a person marks it
  (`docs/api-v1.md`, `GET /api/v1/log/alerts`):
  a `login_failed` until a sign-in of the same name,
  which the ten tests that refuse a sign-in on purpose never make
  (`test_login.py`, `test_oidc.py`, `test_opaque.py`; 1100-en, 2026-11-27).
  Unseen, it put the amber mark on ☰ for every later page of the worker,
  whose few pixels widened the header,
  and the tests of ☰'s marks and of the header's folding failed on it
  in whichever file came after one of those:
  21 of the 33 failures of the soak's first run (2026-11-26).
  A warning a test makes once its page has loaded is newer, and shows;
  a test that reads the log's alerts answers them itself (`alerts_route`),
  with ids above the server's.
- **An id of the test's own** (`own_id(what)` of `helpers.py`, AP SteadyTests, Block 11):
  32 hex digits, new at each call, `what` hex-encoded first
  (at most 8 bytes, for a reader of the server's log), then random ones,
  for what a test makes on the shared server that no API removes,
  so that its next run, a repeat on the same server among them,
  makes its own and meets nothing the last run left.
  An integration is only disabled at its end (`DELETE …/integrations/{uuid}`),
  its users, links and maximum kept,
  and a request from the same installation joins it:
  `test_register_and_link` took its installation id and its user's id there
  from the case alone, so its next repeat on the server
  found the integration of the one before, disabled,
  and waited in vain for a new one's panel
  (8 of the 33 failures of the soak's first run, 2026-11-26).
  Every test that registers an integration takes its installation id from `own_id`,
  and the ids of its users there
  (`test_integrations.py`, `test_users.py`,
  `test_admin_updates.py`'s `made_integrations`),
  and `test_notifications.py`'s push endpoint, which it removes, is one too.
  A name the test removes at its end, a user's or a provider's,
  may stay one of the case's.
  The server takes ten integration requests from an address in ten minutes,
  in its memory, so a run of the three tests that make one,
  each case three times on one server, meets its 429 (2026-11-27);
  the soak's, a case a job, makes nine at most.
- **A test's sessions ended** (`ended_sessions`, `conftest.py`, AP SteadyTests, Block 11):
  a sign-in makes a session on the server,
  in a context of the test's own (`new_context`, `owner_page`) or in its page,
  and a context that closes without a sign-out leaves it there,
  for whichever test the worker runs next.
  After every test the sessions it made are ended
  (`DELETE /api/v1/users/{name}/tokens/{id}`, as `undo`),
  those of every user but an integration's:
  the fixture notes them before the test
  and ends the new ones once its contexts have closed,
  set up before them all, the page's too,
  since pytest sets up a conftest's autouse fixtures in the order of their names
  and `ended_sessions` comes before `guard`, which loads the page.
  A key the test makes is the test's to revoke,
  as is anything else it makes (below).
  74 tests in 23 files left one (1100-en, 2026-11-27),
  each a sign-in it checks, a sign-out late or a page after a 401 among them.
- **A test file leaves the server as it found it**
  (`server_as_found`, `server_left`, `LEFT_FOR_GOOD` of `conftest.py`,
  AP SteadyTests, Block 11, D11-A):
  the four cases and the files of a worker share its server,
  so what one file left met whichever file ran next there,
  a test's repeat among them.
  After the fake Cube's reset, before a file's first test,
  `known_cube` notes what the API shows of the server:
  its users, but those of an integration that is disabled,
  with their sessions and keys and their identities;
  the roles, by their capabilities, whether disabled and their names;
  the providers; the integrations and the requests waiting, but those disabled;
  the settings' stored values; the Cubes configured;
  the token's user's push subscriptions;
  and whether the set's Cube pairs,
  since a pairing left running ends by its timer, in a later file,
  with a `pairing_ended` event.
  After the file's last test, beside the check of the writes on their way,
  it reads them again,
  and a file where they differ fails at its last test's tear-down,
  naming each: added, removed, or changed, with what it was and is.
  A test undoes what it made, in its `finally`, with `undo` and `cleanup`,
  and the kit what several do alike:
  `ended_sessions` the sessions, `clear_preferences` the token's user's preferences.
  What no API takes back is not counted, each in `LEFT_FOR_GOOD` with its reason:
  - the log's lines;
  - the log's warnings, which `clear_preferences` marks seen before every page (above);
  - an integration registered, disabled at its end,
    with its users and its role's count of them:
    a test registers it with an installation of its own (`own_id`, above);
  - a Cube a search found, which stays found while it answers a search,
    as the fake Cube does at every one:
    a test that needs none found tells its page's reads (`configured_only`);
  - the token's user's preferences, which the page fixture removes before every page.
  The sweep that made the list ran the whole suite once,
  1100-en shuffled by the soak's seed 45895569993,
  with a probe of the same reads before and after every test (2026-11-27):
  74 tests left a session, ten a warning, three a disabled integration,
  one a found Cube, and none a user, a setting, a role, a provider,
  an identity, a Cube configured, a push subscription or a running pairing.
  A planted file that adds a user and leaves it fails at its tear-down,
  naming the user.
  The second server of a set, of a Cube that is not connected, is not checked.
