# AP UiStructure v1.0: Components, templates, partials

## TL;DR

The web UI keeps its rules (no build step, no framework,
the CSP as it is) and gets a structure in six blocks:
translations per language loaded on demand,
a `components.js` with one row style, the panels rewritten on it,
the JS-built frames as `<template>` elements,
the page assembled by Go from partials, and the documents.

- i18n.js 168 KB of the UI's 371 → the code only; `ui/i18n/<lang>.json`,
  English plus the chosen language; an ETag on every UI file
- 94 `createElement` sites, 20 `aria-label` set afterwards, four hand-built
  action rows → `components.js`
- `.setting` and `.cfg-field` → one `.field` row, inline; `wide` stacks
- `window.confirm` twice → `confirmBox`
- JS-built frames → `<template>` with `data-i18n`
- one `index.html` → partials assembled at start, a file per feature
- no library; lit-html stays an option for the live panels
- the smoke test's DOM unchanged; every button checked for a name

Rests on
[2026-11-06_04-56_ANALYSIS_UiStructure_spa_structure.md](2026-11-06_04-56_ANALYSIS_UiStructure_spa_structure.md);
the owner's decisions D1 to D8 are in the Discussion.

### Blocks

1. Translations: one JSON file per language, loaded on demand;
   an ETag on the UI files
2. `components.js`, the one row style, the button-name check in the smoke test
3. The panels on the components: cubes.js, config.js, theme.js, tiles.js,
   update.js, app.js
4. The JS-built frames as `<template>` elements
5. The page assembled by Go from partials
6. The documents: the contract's UI rule, docs/ui.md, the test README

## Discussion

The analysis compared keeping the UI as it is, components alone,
three ways of splitting `index.html`, lit-html and Vue (its section 5),
and recommended option 3a: components, then templates,
then partials assembled by Go, no library (its section 6).
The owner answered its section 7 on 2026-11-06:

- **D1.**
  A vendored JavaScript library is acceptable in principle.
  None is adopted here: lit-html's value is the in-place update, which only
  the coming pairing list and restore progress need; those panels decide.
- **D2.**
  The scripts may become ES modules.
  Not in this plan: nothing needs them until a library does,
  and the smoke test reaches `state` through the classic scripts' globals
  (docs/ui-testing.md question 4).
- **D3.**
  The partials assembled by Go come now, as Block 5,
  before the pairing and backup panels:
  each then adds a file of its own instead of both editing the one `index.html`
  at the same time.
- **D4.**
  One row style: the room configuration's inline row (label left,
  control right on one line, numbers right-aligned) is the default,
  under a neutral name `.field`;
  `.field.wide` stacks the control under the label at full width,
  for text inputs and long selects, which is today's `.setting`.
  The scheme editor's `.scheme-row`, a grid of four columns, is not a row of
  label and control and stays.
- **D5.**
  The two native `confirm()` dialogs (Remove a Cube,
  Update the server) become the in-page `confirmBox`: translated,
  styled by the scheme, keyboard operable,
  and the smoke test needs no dialog handler.
- **D6.**
  The pairing and the backup panels live in Settings as `<details>` panels like Cubes;
  `detailsPanel` is the component they need, no `tileHead`.
- **D7.**
  The button-name check goes into the smoke test: on every page it opens,
  every `button` has an accessible name (text, `aria-label` or `title`).
  It covers the buttons the scripts build, which are most of them,
  and runs in CI through the Screenshots job.
- **D8.**
  The translations are split: one JSON file per language under `ui/i18n/`,
  embedded and served; i18n.js keeps the code,
  loads English always and the chosen language before the first texts are filled;
  a Go test holds the files to English's keys and English to the keys the markup
  and the scripts use.
  With it an ETag on the UI files,
  since the server sends no gzip and no validator today,
  so every load downloads all 371 KB again.

**Not in this plan:**
lit-html and ES modules (D1, D2); the pairing, backup and restore,
server-gone and Settings → Server panels,
which are plans of their own written from these components (the analysis's
section 4.3 lists what each needs);
the placeholder names for the screenshot fixture,
an open decision of the owner's.

**Pull requests:**
two, on one branch `claude/ui-structure` with one functional commit per block:
Blocks 1 to 3 (translations, components,
the panels) and Blocks 4 to 6 (templates, partials, documents),
so that the first is reviewable in a sitting and the panels' sessions start
after the second.
`origin/main` is merged into the branch before each, never rebased.

**Who runs it:**
an Opus 5.5 session or subagent, as the owner asked for implementations;
`make ui-test` at 1100 and 390 px before every commit,
and `make ui-screenshots` in every block that changes a pixel,
since the set is retaken at every change of the UI and committed with it.

## Analysis

What changes, by file:

| File | Block | Change |
|------|------:|--------|
| `internal/server/ui/i18n.js` | 1 | the six tables out; `loadLanguage`, `languages()` from `LANGUAGE_NAMES`; `setLanguage` returns the load |
| `internal/server/ui/i18n/{en,de,fr,it,nl,pl}.json` | 1 | new: the tables, flat, key to text |
| `internal/server/ui.go` | 1, 5 | the embed list, `/ui/i18n/{lang}.json`, the ETag; the partials assembled at start |
| `internal/server/ui_test.go` | 1, 2, 5 | `TestUITranslations`, the 304; the file list; the ids of the assembled page |
| `internal/server/ui/components.js` | 2 | new: the components, moved from app.js, config.js and cubes.js, or written |
| `internal/server/ui/style.css` | 2 | `.field`, `.field.wide`; `.setting*`, `.cfg-field`, `.cfg-label`, `.cfg-head`, `.with-help` out |
| `internal/server/ui/index.html` | 2, 4, 5 | the rows' classes; the templates; the skeleton with two markers |
| `internal/server/ui/app.js` | 1, 2, 3 | awaits the language; the helpers out; `renderFound` on `foundRow`, `showMessage` on `showResult` |
| `internal/server/ui/config.js` | 2, 3, 4 | `field`, `switchEl`, `numberInput`, `selectInput`, `formActions`, `configForm` out; the move confirm; the tab bar from a template |
| `internal/server/ui/cubes.js` | 3, 4 | the switch, the actions row, the form, the found row, `window.confirm`; the panel from a template |
| `internal/server/ui/theme.js` | 3, 4 | the editor's rows and actions; the editor's frame and row from templates |
| `internal/server/ui/tiles.js` | 3 | the House checkboxes on `checkRow`, the delete confirm on `confirmBox`, `showTileResult` on `showResult` |
| `internal/server/ui/update.js` | 3 | `window.confirm` on `confirmBox` |
| `internal/server/ui/log.js` | 4 | the log line from a template |
| `internal/server/ui/partials/*.html` | 5 | new: the settings panels and the templates, a file per feature |
| `tools/ui-test/smoke.py` | 2, 4 | the button-name check; the language redraw check (it uses neither row class as a selector) |
| `docs/screenshots/*.png`, `slideshow.gif` | 2, 3 | retaken where a pixel changes |
| `CHANGELOG.md` | 1 to 5 | a bullet per block under Unreleased (`internal/**` is shipped) |
| `GUIDELINES/project/_CONTRACT.md`, `AGENTS.md`, `GUIDELINES/README.md` | 6 | the web UI bullet of section 3; the generated copies |
| `docs/ui.md`, `tools/ui-test/README.md` | 6 | how the page is built; the new checks |

**What stays:**
the ids, the classes the smoke test uses apart from the two row classes,
the four globals it reaches (`state`, `updateDevice`, `applySettings`,
`discovery`), the eight classic scripts in their order,
`theme.js`'s early apply, the CSP, `textContent` for every API text,
`data-i18n` and `applyI18n`, the room and device templates as they are,
`TestUIDefaultSchemeMatchesCSS` (it parses the scheme defaults of theme.js,
which do not move).

**Risks:**

- The row merge changes the settings page's look (Block 2);
  the retaken screenshots show it before the merge.
- The language fetch is one round trip before the texts are filled;
  the static English markup shows until then,
  as it does today until the scripts run.
  On the LAN that is milliseconds.
- A panel's markup moves next to its script (Block 5),
  which changes where people look; docs/ui.md says where (Block 6).
- The pairing, backup and store (b) sessions add keys:
  after Block 1 they add them to the JSON files, one per language,
  and the Go test tells them what is missing.

**Measured on 2026-11-06 at `99fffa7`** (the analysis's section 1):
the UI is 371 KB in ten files,
i18n.js 168 KB of it with six tables of about 26 KB; 94 `createElement` sites,
80 `textEl`, 27 `button` calls with 20 `aria-label` set afterwards;
the server sends no gzip,
no `ETag` and no `Last-Modified` (embedded files have no modification time),
with `Cache-Control: no-cache`.

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`),
`gofmt -l .` empty, `go vet ./...` for linux, windows and darwin,
`make cross` where Go changed,
`make ui-test` at 1100 and 390 px in English and German,
`make ui-screenshots` where a pixel changed,
and `GUIDELINES/shared/tools/check`.
Each block is one functional commit citing `AP UiStructure v1.0, Block <n>`,
with its CHANGELOG bullet.

### Block 1: Translations per language, loaded on demand

1. `internal/server/ui/i18n/en.json` to `pl.json`: the six tables of i18n.js
   as flat JSON objects, key to text, the machine-translated ones keeping
   `i18n.machineNote`.
   Made once from i18n.js by a throwaway script in `GUIDELINES/temp/`;
   the Go test of step 5 is what holds them right.
2. `i18n.js`:
   the tables go, `I18N` starts empty;
   `LANGUAGE_NAMES` is the list of languages and `languages()` returns its keys;
   `loadLanguage(lang)` fetches `/ui/i18n/<lang>.json` into `I18N[lang]` once
   (English always,
   the chosen language when it is another) and returns a promise;
   `setLanguage(setting)` returns that promise; `t`, `tn`, `fmtNum`,
   `applyI18n` and `machineNote` unchanged.
3. `app.js`:
   the start awaits `setLanguage` before the first `applyI18n(document)`;
   the language change in the settings awaits it before applying;
   a failed fetch shows the English texts and the page's message line.
4. `ui.go`:
   `//go:embed` adds `ui/i18n/*.json`;
   `GET /ui/i18n/{lang}` serves the file (JSON by extension);
   every UI file and the page carry `ETag: "<version>/<name>"` so that
   `http.ServeFileFS` answers 304 to `If-None-Match`;
   `Cache-Control: no-cache` stays.
5. `ui_test.go`:
   `TestUITranslations`: each file parses;
   every key of the other five is in English;
   English covers every `data-i18n` and `data-i18n-attr` key of index.html
   and every `t("…")` and `tn("…")` literal of the scripts,
   `tn`'s with `.one` and `.other`; the test names what is missing.
   `TestUIServed` lists the six files and checks a 304 with the ETag.
6. The CHANGELOG bullet under Changed: the web UI loads the texts of the
   chosen language only, and the browser revalidates the UI's files instead
   of downloading them again.

**Verification:**
the test gate; in the browser's network tab a load fetches `en.json` and,
in German, `de.json` and nothing more of the translations;
`curl -sI` of `/ui/app.js` shows the ETag and a second request with
`If-None-Match` answers 304; the smoke test's German run passes.

### Block 2: `components.js`, one row style, the button-name check

1. `internal/server/ui/components.js`, loaded after i18n.js and before app.js:
   `textEl`, `button(label, {cls, aria, title}, onClick)`, `submitButton`,
   `badgeEl`, `iconEl`, `iconButton(symbol, aria, onClick)`, `switchEl`,
   `formEl(onSubmit, cls)`, `formFooter({submit, cancel, extra})`,
   `fieldRow(label, control, {unit, help, hint, id, wide})`, `fillOptions`,
   `numberInput`, `stepper(input, step, min, max)`, `showResult(el, text, ok)`,
   `confirmBox(text, {yes, no}, onYes, onNo)`, `checkRow`, `fromTemplate(id)`,
   `downloadBlob(name, blob)` and `filePicker(accept, onFile)`;
   moved from app.js,
   config.js and cubes.js where they exist (`field` and `fieldRow` become the
   one `fieldRow`), written where they do not.
   `ageEl`, `helpParts` and `fmtTemp` stay in app.js.
2. `style.css`:
   `.field` with the inline layout of today's `.cfg-field`,
   `.field.wide` with the stacked layout of today's `.setting`; `.setting`,
   `.setting-control`, `.cfg-field`, `.cfg-label`,
   `.cfg-head` and `.with-help` go;
   `index.html`'s six settings rows take `.field.wide` or `.field` as their control asks.
3. index.html's script list,
   ui.go's embed list and `TestUIServed` gain components.js.
4. `smoke.py`:
   a new check on every page the test opens:
   every `button` has an accessible name (its text, `aria-label` or `title`).
   Its selectors need no edit: it finds the rows by id, not by class.
5. `make ui-screenshots`:
   the set retaken, since the rows changed.
6. The CHANGELOG bullet under Changed: the settings and the room
   configuration share one row layout.

**Verification:**
the test gate; `grep -c 'createElement("button")'` over `ui/*.js` is zero
outside components.js; no `.setting` or `.cfg-field` is left in the UI files;
the settings and the configuration tabs looked at in the retaken screenshots at
1100 and 390 px.

### Block 3: The panels on the components

One commit per file where a file's change stands on its own, else one
commit for the block:

1. `cubes.js`:
   the hand-built switch, actions row and form on `switchEl`,
   `formFooter` and `formEl`;
   `cubePanel`'s details and summary on `detailsPanel`;
   `foundRow` shared with app.js's `renderFound`;
   `removeCube`'s `window.confirm` on `confirmBox`; `cubesMsg` on `showResult`.
2. `config.js`:
   the move confirm on `confirmBox`; the device form's row on `fieldRow`;
   `renderPoints`'s plus and minus on `stepper`;
   `setConfigResult` on `showResult`.
3. `theme.js`:
   the editor's name row and actions on `fieldRow` and `formFooter`;
   `schemeMessage` and `hexMessage` on `showResult`;
   `exportScheme` on `downloadBlob`, the import on `filePicker`.
4. `tiles.js`:
   the House room checkboxes on `checkRow`;
   the room's delete confirm on `confirmBox`; `showTileResult` on `showResult`.
5. `update.js`:
   `startUpdate`'s `window.confirm` on `confirmBox`.
6. `app.js`:
   `showMessage` on `showResult`;
   the 20 `setAttribute("aria-label")` sites on `button`'s `aria` option.
7. `make ui-screenshots` where a pixel changed;
   the CHANGELOG bullet under Changed: the confirmations are in the page,
   translated.

**Verification:**
`make ui-test` after each file at both widths and in both languages;
`window.confirm` does not occur;
the `createElement` count over `ui/*.js` is at most 50 (94 today);
any selector the smoke test had to change is listed in the commit message.

### Block 4: The JS-built frames as `<template>` elements

1. `index.html` gains `<template>` elements with `data-i18n` for the
   configuration tab bar, the Cube panel (summary, differs notice, form,
   hidden editor), the scheme editor's frame and its row, the found-Cube row,
   the log line and the House room checkbox; the scripts clone them through
   `fromTemplate` and wire them with `addEventListener`.
2. `applyI18n(document)` fills template content already; a clone made later
   takes the texts of the language then current.
3. `smoke.py`:
   a new check that a language change in the settings redraws cloned content
   (the Cube panel open, German chosen, its labels German).
4. The CHANGELOG bullet under Changed, if anything visible changed; else none,
   and the commit says so.

**Verification:**
the test gate; `grep -c innerHTML ui/*.js` is zero;
the `createElement` count is at most 30;
the screenshots retaken are byte for byte those of Block 3 where no pixel was
meant to change.

### Block 5: The page assembled by Go from partials

1. `index.html` becomes the skeleton: head, the icons, the header, the sections,
   the token dialog and the scripts, with two marker comments,
   `<!-- partials: settings -->` inside the settings body and
   `<!-- partials: templates -->` before the scripts.
2. `internal/server/ui/partials/`:
   the settings panels (`cubes.html`, `appearance.html`,
   `software.html`) and the templates (`config.html`, `cube-panel.html`,
   `scheme.html`, `found.html`, `log.html`, `room.html`, `device.html`),
   each next to the script that uses it by name.
3. `ui.go`:
   the embed adds `ui/partials/*.html`;
   `uiRoutes` assembles the page once at start, in an order listed in Go,
   into the bytes it serves at `/` with the ETag of Block 1;
   `/ui/index.html` and the partials are not served on their own.
4. `ui_test.go`:
   every `$("…")` id literal of the scripts exists in the assembled page;
   every partial file is listed exactly once and every listed file exists;
   neither marker is left in the output.
5. The CHANGELOG bullet is not needed unless something visible changed; the
   commit says so.

**Verification:**
the test gate; `curl -s /` is one document with the same `<script>` tags as before;
`make ui-test` unchanged; `make ui-screenshots` byte for byte Block 4's set.

### Block 6: The documents

1. `GUIDELINES/project/_CONTRACT.md` section 3, the web UI bullet:
   shared elements through `components.js`, no `innerHTML`,
   a feature's markup in its partial, its texts in `ui/i18n/<lang>.json`,
   one row style; the version raised,
   `GUIDELINES/shared/tools/sync-docs.sh` regenerates AGENTS.md and GUIDELINES/README.md.
2. `docs/ui.md`:
   a section on how the page is built (the skeleton and the partials,
   the templates, the components,
   the translation files) and how a panel and a key are added.
3. `tools/ui-test/README.md`:
   the button-name and the language redraw checks.
4. `docs/handover-session.md` and `docs/roadmap.md` are the coordinating session's,
   at the end, not this block's.

**Verification:**
`GUIDELINES/shared/tools/check` clean; `sync-docs.sh` leaves no diff;
the commit carries the contract's `File-revision-v1` trailer.

## Proposed commit message

The plan's registration, a commit of its own before the first project write:

```
chore(aps): +UiStructure v1.0, +ANALYSIS UiStructure

The web UI's structure work (one translation file per language, a components file with one row style, the JS-built frames as templates, the page assembled from partials) needs its plan registered before the first project write, and the analysis it rests on comes with it.

Key changes:
- GUIDELINES/wip/2026-11-06_10-08_AP_UiStructure_components_templates_partials.md: the plan, six blocks, the owner's decisions D1 to D8 in its Discussion.
- GUIDELINES/wip/2026-11-06_04-56_ANALYSIS_UiStructure_spa_structure.md: the analysis, copied from messages/ with its timestamp.

File-revision-v1: GUIDELINES/wip/2026-11-06_10-08_AP_UiStructure_components_templates_partials.md::v1.0::2026-11-06::All::the first revision
File-revision-v1: GUIDELINES/wip/2026-11-06_04-56_ANALYSIS_UiStructure_spa_structure.md::v1::2026-11-06::All::the analysis the plan rests on
Assisted-by: claude-code:claude-fable-5-1
```

The blocks' commits, each written at its gate:

1. `feat(ui): the translations one file per language, loaded on demand`
2. `refactor(ui): components.js and one row style`
3. `refactor(ui): the panels built from the components`
4. `refactor(ui): the JS-built frames as template elements`
5. `refactor(ui): the page assembled from partials at start`
6. `docs(ui): how the page is built`

## Change History

- v1.0 (2026-11-06): the first revision, from the analysis's section 6 and
  the owner's decisions D1 to D8 of the same day.
