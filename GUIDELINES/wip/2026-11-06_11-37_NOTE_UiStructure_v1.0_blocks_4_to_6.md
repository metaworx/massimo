# NOTE UiStructure v1.0: blocks 4 to 6

How Blocks 4 to 6 of
[AP UiStructure v1.0](2026-11-06_10-08_AP_UiStructure_components_templates_partials.md)
went where they differ from the plan.

1. **The House room checkbox**
   (Block 4) is a template, `tpl-house-room`, cloned by tiles.js `houseRoomRow`;
   `checkRow`, whose only caller it was, left components.js.
   `detailsPanel` has no caller since the Cube panel is a template with its summary,
   and stays for the pairing and backup panels (D6), as `stepper` stays unused.
2. **`fragmentFromTemplate`**
   (Block 4) joined components.js:
   the scheme editor's frame is several elements that go into `#scheme-editor`
   as they are.
3. **The DOM**
   (Block 4) is the one of Block 3, compared node by node in both languages,
   but for `data-tab` on the configuration tabs,
   `data-mode` on the scheme sets and six classes the scripts find the parts by
   (`found-serial`, `found-facts`, `found-extra`, `log-rf`, `scheme-reset`,
   `scheme-close`).
   `createElement` over `ui/*.js` is 27, 24 of them in components.js;
   `innerHTML` occurs nowhere.
4. **slideshow.gif**
   (Block 4) was retaken although no shot changed:
   it still showed Block 2's cubes and appearance shots,
   which Block 3 had retaken.
5. **A fourth settings partial**, `page.html` (Block 5),
   holds the reload note and Forget token,
   which stand between Appearance and Software; a fifth template partial,
   `house.html`, holds the House checkbox.
   Each partial opens with a comment naming its script,
   which the served page carries; apart from those and blank lines,
   the assembled page is Block 4's index.html line for line.
6. **The tests**
   (Block 5) go beyond the plan's three checks: no id twice in the page,
   the `fromTemplate` and `fragmentFromTemplate` literals checked as `$()`'s are,
   `GET /` the assembled bytes,
   and `TestUIPageMarkers` for the indentation and the refused markers;
   `TestUITranslations` reads the assembled page.
7. **One screenshot run**
   (Block 5) gave `settings-390-light-en.png` with 9 pixels of the Settings
   tile's rounded corner anti-aliased differently;
   the two runs after it were byte for byte Block 4's set,
   which is what is committed.
8. **`check` reports `json` undeclared** by `detect-languages.sh --check`:
   the translation files of Block 1 brought it,
   and `GUIDELINES/config.ini` is not this run's to change.
   It wants the owner's choice: declare `json` in `project.languages`,
   or `ignore = json` under `[languages]`.
