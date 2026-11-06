# ANALYSIS UiStructure: The structure of the web UI

An evaluation of the single-page web UI under `internal/server/ui/`:
whether to split `index.html`, whether to adopt a template system,
which common components the code repeats, the options side by side,
and a recommendation with the outline of an Action Plan.
Nothing here is implemented; no project file changes with this document.
Every figure was measured on 2026-11-06 in the main checkout at `99fffa7`,
unless it is marked as an estimate.
Library sizes are estimates from memory, since this session had no network;
they are marked so and are to be measured on the files themselves
before a decision rests on them.
Serials and names are the fixture's placeholders or none at all.

## 1. What is there today

### 1.1 The files

| File | Lines | Bytes | gzip | Functions | Commits |
|------|------:|------:|-----:|----------:|--------:|
| index.html | 331 | 20,424 | 4,768 | – | 21 |
| style.css | 871 | 29,040 | 6,816 | – | 23 |
| i18n.js | 3,083 | 167,730 | 42,494 | 8 | 15 |
| app.js | 909 | 31,793 | 10,032 | 60 | 20 |
| tiles.js | 880 | 33,172 | 8,921 | 54 | 13 |
| config.js | 957 | 37,229 | 9,688 | 45 | 7 |
| theme.js | 648 | 25,436 | 7,755 | 38 | 2 |
| cubes.js | 338 | 12,742 | 3,927 | 19 | 1 |
| log.js | 228 | 8,548 | 2,960 | 14 | 6 |
| update.js | 120 | 4,608 | 1,751 | 8 | 3 |
| total | 8,365 | 370,722 | 99,112 | 246 | |

The UI is two days old: its first commit is `f02b534` of 2026-11-04,
and 333 commits later nothing in it is entrenched.
A plain `go build` of `cmd/massimo` on linux/amd64 is 20,062,696 bytes;
the ten UI files are 1.8 % of that.
`i18n.js` alone is 45 % of the UI's bytes (474 keys in English,
six languages, one table each).

### 1.2 How the page is built

`index.html` holds, in this order:
ten SVG symbols (three of them unused alternatives, lines 11 to 27),
the header (28 to 72), the settings section (77 to 155, the longest block),
the House tile (157 to 200), the rooms, unlisted and log sections (202 to 216),
the token dialog (219 to 226), the room template (228 to 305),
the device template (307 to 320) and eight classic `<script>` tags.
It carries 86 ids, 40 buttons, 77 `data-i18n` and 24 `data-i18n-attr` attributes,
seven `data-help` slots, two `<template>`, two `<details>`, one `<dialog>`,
three `<form>`, and no inline handler.

The eight scripts are classic scripts loaded in order,
sharing globals (`state`, `$`, `t`, `renderRooms`, `updateDevice`, …);
there is no `import` or `export`.
`theme.js` runs last and applies the stored scheme the moment it runs,
before the page is drawn, so that a scheme does not flash the default.
The room tile and the device row are the two places where markup lives in
`<template>` elements and is cloned with `cloneNode`;
every other piece the scripts show is built with `document.createElement`.

### 1.3 Where the scripts build markup

| File | `createElement` | `textEl` | `button` | `addEventListener` | `querySelector` | `$()` |
|------|---:|---:|---:|---:|---:|---:|
| app.js | 12 (+2 NS) | 13 | 1 | 18 | 4 | 40 |
| tiles.js | 4 | 13 | 1 | 35 | 66 | 31 |
| config.js | 37 | 29 | 16 | 7 | 3 | 0 |
| cubes.js | 22 | 14 | 5 | 5 | 4 | 4 |
| theme.js | 16 | 6 | 4 | 12 | 2 | 19 |
| log.js | 1 | 5 | 0 | 6 | 0 | 10 |
| update.js | 0 | 0 | 0 | 4 | 0 | 12 |
| total | 92 (+2) | 80 | 27 | 87 | 79 | 116 |

The helper columns count call sites without the definition.
The 94 `createElement` sites by tag: `div` 33, `span` 13, `input` 13,
`option` 9, `label` 4, `ul` 3, `li` 3, `button` 3, `optgroup` 2, `form` 2,
`details`, `summary`, `select`, `fieldset`, `datalist`, `a` one each,
and one with a variable tag inside `textEl`.
`innerHTML` does not occur; `textContent` occurs 58 times, all but one an assignment.
`tiles.js` is the file that works on cloned templates,
hence its 66 `querySelector` calls; `config.js`, `cubes.js` and `theme.js` build
their panels from scratch, hence their 75 `createElement` calls.

### 1.4 The shared helpers that exist

| Helper | Defined in | Uses | What it builds |
|--------|-----------|-----:|----------------|
| `textEl(tag, text, cls)` | app.js | 80 | any element with text |
| `button(label, cls, onClick)` | app.js | 27 | a `type=button` with a listener |
| `submitButton(label)` | app.js | 3 | the primary submit |
| `badgeEl(text, kind)` | app.js | 4 | a `.flag` badge |
| `iconEl(symbol, cls)` | app.js | 2 | an SVG `<use>` of a page symbol |
| `ageEl(iso, stale, plain)` | app.js | 7 | the age of a value with its tooltip |
| `helpParts(key, label)` | app.js | 3 | the ? button and its explanation |
| `switchEl(label, checked, onChange)` | config.js | 6 | the slide switch |
| `field(label, input, unit, help)` | config.js | 15 | a labelled control (`.cfg-field`) |
| `withHelp(el, help, label)` | config.js | 2 | a control that has its own label, plus ? |
| `numberInput(value, min, max, step, onChange)` | config.js | 13 | a number input |
| `selectInput(options, value, onChange)` | config.js | 2 | a select with options |
| `formActions(onCancel, sendLabel, extra)` | config.js | 5 | the Cancel and Send row |
| `configForm(onSubmit)` | config.js | 6 | a form with `preventDefault` |
| `fieldRow(c, name, labelKey, value, hint)` | cubes.js | 4 | a labelled text input (`.setting`) |
| `fillDatalist(dl, serial, hidden)` | cubes.js | 2 | the options of a datalist |

The review of 2026-11-05 (duplication item 6) asked for the small helpers to move
to `app.js` and be shared; the API, number and badge helpers did move,
the form helpers of `config.js` and `cubes.js` did not, and `theme.js`,
written after both, uses none of them.

### 1.5 What is built by hand although a helper exists, or twice

| Pattern | In index.html | In JS by helper | In JS by hand (file: function) |
|---------|--------------:|----------------:|--------------------------------|
| a button with aria-label or title | 13 | – | 20 `setAttribute("aria-label")` after a `button()` call |
| a badge | 2 (`.pill`, `.window-open`) | 4 | – |
| a slide switch | 1 (expert toggle) | 6 | cubes.js: `cubePanel` (follow) |
| a labelled row (`.setting` or `.cfg-field`) | 6 | 15 + 4 | theme.js: `renderSchemeSet` (19 rows), `renderSchemeEditor` (name row) |
| a form | 3 | 6 | cubes.js: `cubePanel` |
| the Cancel and Send row (`.actions`) | 6 | 5 | cubes.js: `cubePanel`; config.js: `renderDeviceForm`, move confirm; theme.js: `renderSchemeEditor` |
| a `<details>` panel with a summary | 2 | – | cubes.js: `cubePanel` (name, serial, badges) |
| a confirmation | 1 (`.delete-confirm` in the room template) | – | config.js: move confirm (same markup); `window.confirm` in cubes.js `removeCube` and update.js `startUpdate` |
| a tile head | 5 | – | – |
| a found-Cube row (serial, address, Add) | – | – | app.js: `renderFound`; cubes.js: `foundRow` (the same row twice) |
| a select filled with options | – | 3 | app.js: `renderCubeSelect`, `renderLangSelect`; theme.js: `renderSchemeSelect` |
| a −/+ stepper around a number | 3 | – | config.js: `renderPoints` (+ and − as `step small`) |
| "how it went" line with ok/bad | 4 (`#message`, `#house-result`, `#cubes-msg`, `#scheme-msg`) | – | `showMessage`, `showTileResult`, `setConfigResult`, `cubesMsg`, `schemeMessage`, `hexMessage`: six functions |
| a checkbox with a label | – | – | tiles.js: `renderHouseRooms` |

Two row styles exist for the same thing: `.setting` (the settings page,
`fieldRow`, flex with the control on its own line)
and `.cfg-field` (the room configuration, `field`, label left, control right).
The scheme editor has a third, `.scheme-row`, a grid of four columns.
There is no `<table>`, no `<progress>` and one `type=file` input in the UI.

### 1.6 What the rules and the smoke test hold on to

`tools/ui-test/smoke.py` (495 lines) finds the page through 70 distinct
selectors (27 ids, about 36 classes, the rest tags and attributes),
reads it through 32 `page.evaluate` calls, and reaches four globals of the
page's scope: `state`, `updateDevice`, `applySettings` and `discovery`.
Any restructuring that keeps the ids, the classes and those four names keeps
the test; `docs/ui-testing.md` section 1 says why `state` is reachable only
because the scripts are classic.
The test runs on the developer's machine only (GUIDELINES/project/CI.md section 2),
so every block of a restructuring is verified by one person's run;
`docs/ui-testing.md` step 2 (the job in CI) would make that run everyone's.

The rules (docs/review-2026-11-05.md, coding standard; AGENTS.md section 3):
no build step, no framework, no inline handlers, API text only through
`textContent`, every text through `i18n.js`, CSS variables for light and dark,
phone width first.
`ui.go` sends `default-src 'self'; img-src 'self' data:; frame-ancestors 'none';
base-uri 'self'; form-action 'self'`: no `'unsafe-inline'`, no `'unsafe-eval'`,
and no source but the server itself, so a library has to come out of the binary.
`ui_test.go` holds the list of files `go:embed` serves (`TestUIServed`)
and parses `style.css` and `theme.js` as text (`TestUIDefaultSchemeMatchesCSS`);
both are touched by any change of the file set.

## 2. Splitting index.html

### 2.1 What a split would address, and what it would not

`index.html` is 331 lines; the settings section is 79 of them and is the block
that will grow: Cubes and Appearance are there already, and Server,
Backup and restore, Pairing (if it lives there), Users and Loxone are coming.
An estimate: the settings section alone reaches 250 to 350 lines once those are in.
The rest of the file is stable.
But the markup people look for most is not in `index.html` at all:
the configuration area, the Cube panel and the scheme editor are JS
(75 `createElement` calls in three files).
A split of the file by page therefore helps the settings page and little else;
the split that matters is section 2.5's.

### 2.2 Option A: partials merged by a build step

Each page and panel in `ui/partials/<name>.html`,
a `//go:generate go run ./internal/server/ui/gen` that concatenates them into
`index.html` in a fixed order, and the generated file committed and embedded.
No Node, no tool but Go, and the embedded output stays one file.
What it costs: a generated file in the tree that can go stale,
which needs a check (a Go test that regenerates in memory and compares,
as `TestUIDefaultSchemeMatchesCSS` compares two files today;
CI's Guidelines job already checks generated documents the same way);
`go generate` is a step to remember, and the contract says there is none.
A Make target is the same with a different name.
This is the heaviest of the three ways to split, for no gain over A'.

### 2.3 Option A': partials assembled by the server at start

Embed `ui/partials/*.html` instead of one `index.html`,
and let `uiRoutes` concatenate them once, in a listed order,
into the bytes it serves at `/`.
There is no build step, nothing generated, nothing to keep current;
the browser still receives one document, with the same CSP and the same ids.
The cost is about twenty lines of Go, an updated `TestUIServed`,
and that `ui/index.html` no longer exists as a file:
a skeleton partial (`head`, the icons, the header, `main`'s opening,
the scripts) frames the others.
A Go test can then also check that every id the scripts ask for
(`$("…")`, 116 sites) exists in the assembled page,
which catches a partial left out of the list.
The risk is small: the DOM the smoke test sees does not change.
An IDE validates a fragment file as well as a whole page.
Each feature's markup can then sit next to its script:
`cubes.html` with the Cubes block and the Cube panel's template beside `cubes.js`,
`theme.html` beside `theme.js`, and so on.

### 2.4 Option B: partials fetched at run time

Each panel fetched with `fetch("/ui/settings.html")` and inserted with
`innerHTML` (or parsed into a `<template>` and adopted).
Under the CSP it works: `connect-src` falls back to `default-src 'self'`,
and CSP does not govern `innerHTML`; a `<script>` inside inserted markup never runs,
and an inline handler in it would be blocked as anywhere.
Is `innerHTML` of a same-origin static file acceptable under the "textContent
only" rule?
As a matter of trust, yes:
the partial is the author's markup out of the binary,
the same trust as `index.html` itself, and no API string is in it.
As a matter of keeping the rule, no:
today `innerHTML` does not occur at all, which is a one-line grep to check;
"innerHTML only for partials" is a rule with an exception,
and the exception is the place a later change would put an API string.
What it costs beyond that:
the page paints its header and then waits for the partials
(`Cache-Control: no-cache`, so each is revalidated on every load);
`init()` runs on `DOMContentLoaded`, when `$("poll-input")`, `$("theme-select")`
and the rest would not exist yet, so the whole initialisation order changes
to "after the partials";
`applyI18n(document)` and `initHelpSlots()` must run again after each insertion;
the smoke test's `wait_for_selector(".room")` would still work,
but every check that clicks into settings right after loading would race the fetch.
Verdict: rejected; A' gives the same files on disk at no run-time cost.

### 2.5 Option C: the markup the scripts build moves into templates

Keep one document (or A' partials) and move the frames the scripts build into
`<template>` elements cloned with `cloneNode`, as the room tile and the device
row are built already.
Candidates, from what section 1.5 lists:
the configuration tab bar, a configuration field, the Cancel and Send row,
the Cube panel (summary, differs notice, form, hidden editor), the found-Cube row,
the scheme editor's frame and its row, the log line and the House room checkbox.
What it gives: markup readable as markup, with `data-i18n` inside
(`applyI18n` already fills template content and so does a later clone),
the rules kept (a clone is wired with `addEventListener`,
text is set with `textContent`), and the smoke test's classes living in the
markup where they can be seen.
What it costs: the script still has to find the parts and wire them,
as `renderRoom` does with its 13 `querySelector` calls per tile,
so the structure and the code that fills it are in two files;
`index.html` (or the partials) grows by an estimated 150 to 250 lines.
This is the right split for frames and rows; a leaf such as a badge is one
`textEl` call and stays code.
It is independent of 2.2 to 2.4, and it is what the project already does.

### 2.6 An aside: the split that would change the page's weight

`i18n.js` is 168 KB raw (42 KB gzipped), five sixths of it the five languages
the user has not chosen.
One file per language, fetched by `setLanguage`
(same origin, allowed by the CSP, cached by the browser),
would cut an estimated 120 KB raw from every load.
It is off this brief's question, and the page loads from the LAN,
so it is noted and not pursued.

## 3. A template system

### 3.1 The constraints

A library has to run under `default-src 'self'`:
no `new Function`, no `eval`, no inline `<style>` it creates
(theme.js already works around that with `style.setProperty`).
It has to be served from the binary, so its file is vendored by hand
(downloaded once, committed, updated by hand, listed for the guidelines'
vendored-files check; `govulncheck` does not see it) and counts against the binary.
It has to coexist with eight classic scripts and their globals,
or those become modules.
Its licence has to sit with AGPL-3.0 (MIT and BSD do).
And it has to be editable by someone without a JS toolchain:
no compile step, and markup that an editor still recognises.

### 3.2 The candidates

| Candidate | Size, min+gzip (estimate) | `new Function` | Under this CSP | Build or Node | Script form |
|-----------|---------------------------|----------------|----------------|---------------|-------------|
| Vue 3, full build (runtime + compiler) | about 50 KB | yes, for every template | blocked without `'unsafe-eval'` | none | global or ESM |
| Vue 3, runtime-only | about 35 KB | no | yes, with precompiled templates | the SFC or DOM compiler, in Node | ESM, global |
| petite-vue | about 6 KB | yes | blocked | none | global |
| Alpine.js, CSP build | about 15 KB | no | yes, expressions limited to names and paths | none | global |
| Preact + htm | about 4 KB + 1 KB | no | yes | none | ESM, UMD available |
| lit-html alone | about 7 KB (under 20 KB raw) | no | yes | none | ESM only |
| Lit (LitElement) | about 5 KB on top | no | yes, with shadow DOM caveats | none | ESM only |
| own: `<template>` + `cloneNode` + a helper | 0 | no | yes | none | classic, as now |

**Vue**, honestly: its own documentation says the full build compiles templates
with `new Function` and is not usable under a CSP without `'unsafe-eval'`.
Adding `'unsafe-eval'` would be the first `unsafe-` word in the policy that was
added as a security fix (review of 2026-11-05, security item 6), and it would
weaken the one protection the page has beyond `textContent`.
The runtime-only build is CSP-clean but needs the templates compiled ahead,
which is `@vue/compiler-sfc` or `@vue/compiler-dom` under Vite or a script,
that is Node and a build step, against two decisions of `docs/ui-testing.md`.
A Go build step that runs the Vue compiler in an embedded JS engine is possible
in principle and a stunt in practice.
Render functions written by hand with `h()` need no compiler and no eval,
but then there is no template, and `h("div", {class: "setting"}, …)` is
`createElement` with a shorter name.
Vue also brings its own state model (reactive proxies), its own event syntax
in markup (`@click`), its own i18n expectations and a 35 KB runtime for a UI
whose scripts are 154 KB unminified without translations.
Verdict: not for this project, with or without a build step.

**petite-vue** evaluates its `v-if`, `:class` and `@click` expressions with
`new Function`; the CSP blocks it.
Out.

**Alpine.js** in its CSP build evaluates nothing, but its model is behaviour in
attributes of the markup: `x-data`, `x-show`, `x-on:click="save"`.
That is not an inline handler for the CSP, and it is one for the project's rule
in spirit: the wiring moves from the scripts into the markup.
It would help the static settings page a little and the JS-built panels not at all.
Out.

**Preact with htm** parses `html\`<div class=setting>…</div>\`` at run time
without eval, renders a virtual DOM into a container and diffs on the next render,
so an input keeps its node, its focus and its value across renders
(the problem `rerenderRoom`'s `rendering` flag works around today).
It is 5 KB, MIT, has UMD builds that load as classic scripts,
and coexists page by page: `render(…, $("cubes-list"))` and nothing else changes.
It asks for a different way of thinking (a function of state returning the view,
re-run on every change), which is close to what `rerenderRoom`, `renderCubes`
and `renderSchemeEditor` do by hand already.
The markup lives in JS strings; an editor without a plugin sees a string.

**lit-html** alone (without LitElement) does the same with less:
`html\`…\`` is parsed once into a `<template>` (the static strings go through
`innerHTML` of a template element, which is the library's own code on the
author's text and not an API string), `render(tpl, container)` updates in place,
text bindings `${name}` become text nodes and never markup unless one writes
`unsafeHTML`, event bindings `@click=${fn}` are `addEventListener` calls the
library makes.
So the two rules hold by construction, and the one new rule is "no `unsafeHTML`".
It renders into the light DOM, so `style.css`, its 179 classes and the 19 colour
tokens apply as they do now, and the smoke test's selectors are whatever the
templates say.
It is under 20 KB raw (an estimate), BSD-3-Clause, and ships as ES modules only.
That is the hurdle: a classic script cannot `import` it.
Either every script becomes a module (`export`/`import`, `state` exported from
`app.js`; the smoke test then needs the test hook of `docs/ui-testing.md`
question 4, because a module has no global scope to evaluate in),
or a small module `lib.js` imports it and puts `html` and `render` on `window`
for the classic scripts, which keeps everything else as it is at the price of
two more globals in a page that lives on globals.
Script order is kept in either case, if the classic scripts get `defer`:
a module script and `defer` classic scripts
run in document order after parsing, and `theme.js`'s early apply is no earlier
today, since the scripts sit at the end of `body`.

**Lit** (LitElement) adds custom elements with shadow DOM.
A shadow root does not see `style.css`, so every component would carry its own
styles; the tokens would still cross the boundary, since CSS custom properties
inherit, but 29 KB of CSS would be split per component,
and a browser without constructed stylesheets would fall back to `<style>`
elements that `style-src 'self'` blocks.
Not for this UI.

**The project's own**: `<template>` elements cloned with `cloneNode` plus a
helper of its own (`fromTemplate(id)`, `el(tag, attrs, children)`), that is
section 2.5 with the components of section 4.
It adds nothing to the binary and changes no script form.
What it lacks against lit-html is the in-place update:
a panel is rebuilt and replaced, which is what the UI does today,
and which costs focus and scroll in a panel that redraws while the user types,
hence the `rendering` guard in `tiles.js` and the "draw only while no panel is
open" rule of `cubes.js`.
Panels with live events while the user edits are the pairing list and the
restore progress; both are coming.

### 3.3 What each does to the project's parts

| | i18n.js | CSS variables, schemes | Smoke test | Editing without a toolchain |
|---|---|---|---|---|
| Vue (build) | `t()` as a global property in templates; the plural helper too; the `data-i18n` scan goes | unchanged | selectors kept where templates keep the classes; `state` becomes a store, every `page.evaluate` of it rewritten | needs the toolchain to see the result |
| Preact + htm | `${t("…")}` in templates | unchanged | selectors kept; `state` kept if classic scripts stay | HTML in strings; no compile |
| lit-html | the same | unchanged (light DOM) | selectors kept; `state` kept with the `lib.js` shim, else a test hook | HTML in strings; no compile |
| templates + components | `data-i18n` in templates, filled by `applyI18n` as now | unchanged | selectors move into markup, otherwise unchanged | plain HTML and plain JS, as now |

## 4. Extracting common components

### 4.1 The inventory

Independent of sections 2 and 3, a `components.js` loaded after `i18n.js` and
before `app.js` (the place the helpers of `app.js` have now),
holding what sections 1.4 and 1.5 found, each a function returning an element:

| Component | Replaces | Signature (a proposal) |
|-----------|----------|-------------------------|
| `button` | the existing one, plus 20 `setAttribute("aria-label")` sites | `button(label, {cls, aria, title}, onClick)` |
| `iconButton` | the 12 icon-only buttons of the markup, if one is ever built in JS; `helpParts`'s button | `iconButton(symbol, aria, onClick)` |
| `badgeEl` | the existing one | as now |
| `switchEl` | `config.js`'s, and the hand-built one in `cubes.js` | `switchEl(label, checked, onChange)` |
| `settingRow` | `field` (15), `fieldRow` (4), the `.setting` rows of the markup, the scheme name row | `settingRow(label, control, {unit, help, hint, id})` |
| `formEl` | `configForm` (6), the hand-built form of `cubes.js` | `formEl(onSubmit, cls)` |
| `formFooter` | `formActions` (5) and the four hand-built `.actions` rows | `formFooter({submit, cancel, extra})` |
| `detailsPanel` | `cubePanel`'s details and summary; the found-Cubes box | `detailsPanel({name, mono, badges, open, onToggle}, body)` |
| `confirmBox` | the move confirm, the room's `.delete-confirm`, and the two `window.confirm` calls | `confirmBox(text, {yes, no}, onYes, onNo)` |
| `fillOptions` | the 9 `createElement("option")` sites, `selectInput` | `fillOptions(select, [[value, label]], {groups})` |
| `numberInput`, `stepper` | `numberInput` (13), the three −/+ pairs of the markup, the +/− of the programme points | `stepper(input, step, min, max)` |
| `resultEl`, `showResult` | the six "how it went" functions | `showResult(el, text, ok)` |
| `foundRow` | `renderFound`'s and `cubes.js`'s copies | one function for both |
| `checkRow` | the House room checkboxes | `checkRow(label, checked, onChange)` |
| `fromTemplate` | `$("…").content.firstElementChild.cloneNode(true)`, twice today | `fromTemplate(id)` |

`ageEl`, `helpParts` and `fmtTemp` stay in `app.js`:
they read `state` and `i18n`, and they are already shared.
The tile head stays markup: all five are in `index.html`,
and a `tileHead(title, tools)` is needed only if a new page is built in JS.

### 4.2 How far that alone gets the project

An estimate: 40 to 60 of the 94 `createElement` sites go,
the four hand-built `.actions` rows and the hand-built switch go,
the two row styles become one (a decision, open question 4),
every button with an icon or without a visible label gets its `aria-label`
through one signature, and the next panel is written from a dozen building
blocks instead of from `cubes.js` as the model.
What it does not give: in-place updates (section 3.2, lit-html) and a markup
view of the JS-built frames (section 2.5, templates).
Both can be added later on top of the same components,
since a component that returns an element is what a template clone and a
lit-html template both produce.

### 4.3 What the coming features need

From `docs/roadmap.md`, `docs/handover-session.md` and `docs/api-v1.md`:

- **Pairing panel** (`POST`, `GET`, `DELETE …/pairing`; events `pairing_started`,
  `paired`, `pairing_ended`): a Start with a seconds input,
  a status line with "until" counting down (an `ageEl` for a time ahead),
  Stop, a list of paired devices that grows with each `paired` event,
  and per device the name field, the room select and Send, which is the
  name-and-room half of `renderDeviceForm` today; a result line.
  Needs: `settingRow`, `numberInput`, `fillOptions`, `formEl`, `formFooter`,
  `badgeEl` (the type), `showResult`, `detailsPanel` if it lives in Settings,
  and a countdown variant of `ageEl`.
- **Backup and restore** (`GET …/backup`, `POST …/restore/plan`,
  `POST …/restore`, `GET …/restore`; events `restore_step`, `restore_done`):
  a Download (the token goes in a header, so a `fetch` and a `blob:` link,
  which `exportScheme` does already: extract `downloadBlob(name, blob)`),
  a file picker (`scheme-file` is the model: extract `filePicker(accept, onFile)`),
  option switches (`portal`, `state`, four `skip` groups: six `switchEl`),
  Plan, then the steps as rows with a radio flag and the cost
  (the UI has no table; a step list of rows with a status badge is the component
  to write), Send behind a `confirmBox`, and the progress as the rows take their
  result from the events (the case for in-place updates).
  Needs: the above plus `confirmBox`, `showResult`, a step list.
- **The notice when the server is gone**: the page-level `.notice` and `#message`
  exist; what is new is a blocked state (`setBlocked(on)`: a body class,
  every form's controls disabled, the notice shown) and polling until the server
  answers again.
  Needs: a `notice(el, text, kind)` and `setBlocked`.
- **Settings → Server** (`GET` and `PUT /api/v1/settings`): four duration rows
  with the default and "applies at restart" as hint, `null` for the default,
  a Save and a result line.
  Needs: `settingRow` with `hint`, `formEl`, `formFooter`, `showResult`,
  exactly `cubePanel`'s form without the panel.
- **Login and users** (store stage (b), the capability model): a login dialog
  (the token dialog is the model: `dialogForm(title, fields, submit)`),
  a users list of `detailsPanel`s with role badges, a role editor of
  `switchEl` per capability, `confirmBox` for a removal,
  and controls hidden or disabled by capability, which `button`'s options could
  take (`{can: "…"}`).
- **Loxone** (later): a settings block of rows; the same components.

So the components cover the five features; what they add new is a step list,
a file picker, a download helper, a countdown, a blocked state and a dialog form.

## 5. The options compared

Effort is in developer days and an estimate; "rules" is the coding standard's UI
bullet; "smoke test" is what changes in `smoke.py`.

| Option | Build step | Node | CSP | Binary | Effort | Rules | Smoke test | The next features |
|--------|-----------|------|-----|--------|-------:|-------|------------|-------------------|
| 1. Keep as is | none | no | unchanged | +0 | 0 | unchanged | unchanged | each panel repeats `cubes.js`'s hand-built rows; an estimated 1,000 to 1,500 lines of the same kind for the four panels |
| 2. Components only | none | no | unchanged | +0 | 2 to 3 | stronger: aria and `textContent` in one place | selectors unchanged; about 10 edits where the two row styles merge | building blocks for every form and row; no in-place updates |
| 3a. Partials assembled by Go at start, plus templates, plus components | none (20 lines of Go at start) | no | unchanged | +0 | 3 to 4 | unchanged | unchanged; the DOM is the same | markup per feature beside its script; frames readable as HTML |
| 3b. Partials fetched at run time, plus components | none | no | unchanged, `innerHTML` enters | +0 | 4 to 5 | an exception to "no innerHTML" | init order and timing rewritten | slower first paint for nothing 3a does not give |
| 3c. Partials merged by `go generate`, plus components | `go generate` and a currency check | no | unchanged | +0 | 3 to 4 | unchanged | unchanged | a generated file to keep current |
| 4. lit-html plus components | none; one vendored file | no (downloaded once by hand) | ok; new rule: no `unsafeHTML` | +under 20 KB raw (estimate) | 2 for the shim and the first panel; 5 to 8 for every JS-built panel | hold by construction | selectors unchanged; `state` reachable with the shim, else a test hook | in-place updates, templates as HTML in JS, less wiring code |
| 5. Vue with a build step | Vite and the SFC compiler | yes | runtime-only build ok; full build blocked | +about 35 KB gzipped runtime plus the build output | 10 to 15: a rewrite | rewritten: markup carries `@click`, i18n through a plugin | selectors mostly kept; every `state` evaluate rewritten | the best tooling, against two decisions of the project and with the smoke test as the only net |

Preact with htm sits between 4 and 5: 5 KB, classic scripts possible through
its UMD builds, a virtual DOM instead of lit-html's in-place templates;
the migration cost is that of 4 with a larger change of thinking.

## 6. Recommendation

Take option 3a in this order: the components first (section 4),
then `<template>` elements for the frames the scripts build (section 2.5),
then the partials assembled by Go at start (section 2.3) when the settings page
gets its next panel; and do not adopt a template library now.
The reasons:

- It keeps every rule and both decisions of `docs/ui-testing.md`
  (no Node, no build step), and the CSP as it is.
- It costs an estimated three to four days and nothing in the binary.
- The DOM the smoke test knows does not change, so the test verifies each block
  as it is, and nothing else does; that is why the blocks are small and each ends
  with `make ui-test`.
- The components should land before the pairing and backup panels,
  which would otherwise copy `cubes.js`'s hand-built rows and double the work
  of extracting them later; the UI is two days old, so now is the cheap moment.
- lit-html is the one library that fits the constraints,
  and its value is the in-place update, which the UI does not need until a panel
  redraws under the user's hands while events arrive.
  The restore progress and the pairing list are the first such panels;
  if their hand-rolled updates turn out ugly, lit-html goes in for those panels
  alone, page by page, on top of the same components, once the owner has
  answered open questions 1 and 2.
  Deciding it now would be deciding on an estimate.
- Vue is ruled out by the CSP in its no-build form and by the Node decision in
  its build form, and would rewrite a UI that works.

### 6.1 The Action Plan it would take

An outline, for the owner to turn into an AP with `PLAN` if agreed;
each block is one pull request and ends with its verification.

1. **`components.js`** (`refactor(ui)`): the file with `textEl`, `button`
   (with `aria` and `title` options), `submitButton`, `badgeEl`, `iconEl`,
   `switchEl`, `formEl`, `formFooter`, `settingRow`, `fillOptions`,
   `numberInput`, `stepper`, `showResult`, `confirmBox`, `fromTemplate`,
   moved or written; `app.js` and `config.js` lose their copies;
   `index.html`'s script list, `ui.go`'s embed list and `TestUIServed` gain the file.
   Verification: `go test ./internal/server`, `make ui-test` with its 404 checks
   unchanged, and `grep -c 'createElement("button")' ui/*.js` is zero outside
   `components.js`.
2. **The callers** (`refactor(ui)`, one commit per file): `cubes.js`
   (the switch, the actions row, `fieldRow` → `settingRow`, `foundRow` shared
   with `app.js`), `config.js` (the move confirm, the device form's row),
   `theme.js` (the editor's rows and actions), `tiles.js` (the House checkboxes,
   the delete confirm through `confirmBox`), `update.js` and `cubes.js`
   (`window.confirm` → `confirmBox`, or not, open question 5).
   Verification: `make ui-test` at both widths and in both languages after each file;
   the `createElement` count down from 94 to at most 50 (estimate);
   `smoke.py`'s selector edits listed in the commit.
3. **Templates for the frames** (`refactor(ui)`): the configuration tab bar,
   the Cube panel, the scheme editor's frame and row, the found-Cube row and the
   log line as `<template>` elements with `data-i18n`, cloned through `fromTemplate`.
   Verification: `make ui-test`; a language change in the settings redraws the
   cloned content (a new check); `grep -c innerHTML ui/*.js` is zero.
4. **Partials assembled at start** (`refactor(ui)`, optional, open question 3):
   `ui/partials/` with a skeleton and one file per feature, the order listed in
   `ui.go`, concatenated once in `uiRoutes`; a Go test that every id the scripts
   ask for exists in the assembled page.
   Verification: `go test ./internal/server`, `make ui-test`, and `curl /` shows
   one document with the same `<script>` tags.
5. **The documents** (`docs(ui)`): the coding standard's UI bullet gains
   "shared elements through `components.js`, no `innerHTML`",
   `docs/ui.md` is unchanged (nothing the user sees changes),
   `CHANGELOG.md` takes a bullet per block since `internal/**` is shipped,
   `docs/handover-session.md` and `docs/roadmap.md` by the coordinating session.
   Verification: `GUIDELINES/shared/tools/check`.

The pairing and backup panels then come as blocks of their own plans,
written from the components; their briefs can name the components they use.

## 7. Open questions for the owner

1. Is a vendored JavaScript library acceptable at all, in principle:
   one file of under 20 KB, BSD-3-Clause, downloaded once, committed,
   updated by hand and listed for the guidelines' vendored-files check?
   If never, section 3 is closed and the UI stays library-free;
   if yes, lit-html is the candidate for the panels with live updates, later.
2. May the eight scripts become ES modules when that day comes,
   with `state` exposed for the tests (`docs/ui-testing.md` question 4),
   or should they stay classic scripts with a `lib.js` shim that puts the
   library's two functions on `window`?
3. The partials assembled by Go (block 4): with the next settings panel,
   now, or not at all?
4. The two row styles, `.setting` on the settings page and `.cfg-field` in the
   room configuration: merge to one look (which one), or keep both and give
   `settingRow` a style option?
5. The two native `confirm()` dialogs (Remove a Cube, Update the server):
   replace with the in-page `confirmBox` (translated, styled, keyboard operable)
   or keep native for destructive actions?
6. Where do the pairing and the backup panels live: in Settings as `<details>`
   panels like Cubes, or as pages of their own behind a fourth button in the header?
   Pages of their own need a `tileHead` component; panels do not.
7. Should the icon-only buttons' `aria-label` be enforced by a Go test over the
   embedded files (every `icon-btn`, `tool-btn` and `step` with an `aria-label`
   or a `data-i18n-attr` naming one), as `TestUIDefaultSchemeMatchesCSS` enforces
   the schemes?
