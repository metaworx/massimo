# NOTE UiStructure v1.0: blocks 1 to 3

How Blocks 1 to 3 of
[AP UiStructure v1.0](2026-11-06_10-08_AP_UiStructure_components_templates_partials.md)
went where they differ from the plan.

1. **The ETag's version**
   (Block 1) is the server version's short form (`v0.7.0-alpha.1`,
   without "massimo" and the build details, which hold spaces an ETag may not),
   and a `-dirty` or unversioned build adds the time the server started,
   since two such builds can serve different files under one version.
2. **Measured**
   (Block 1, Chromium against the fake Cube):
   a fresh load fetched 370,722 bytes before,
   233,537 after in English and 259,315 in German (`en.json` and `de.json`,
   nothing more of the translations); a reload revalidates every file,
   0 body bytes.
3. **Components beyond the plan's list**
   (Block 2): `clear` moved with the others,
   `elem(tag, cls, ...children)` added in Block 3 for the plain frames,
   `selectInput` kept on `fillOptions`, `detailsPanel` written in Block 2;
   `button` takes a class string or `{cls, aria, title}`,
   `iconButton` and `switchEl` an optional fourth argument.
4. **`stepper`** is written and not used:
   `renderPoints`' − and + remove and add a switch point,
   they are not a stepper around a value, so they stay buttons (Block 3 step 2).
5. **`foundRow`** lives in cubes.js beside `addCube`,
   since it uses `ageEl` and `addCube`;
   Settings → Cubes now shows a found Cube's firmware and age as the header does.
6. **The 20 `aria-label` sites**
   (Block 3 step 6): 9 were buttons and use `button`'s option now;
   11 name inputs, groups and a badge and stay.
7. **The row style**
   (Block 2): the labels are muted in the settings too, as in the configuration,
   and a switch's help button sits at the row's end; the clock,
   language and colour-scheme choices and every text field are wide,
   poll and mode inline.
8. **Beyond the six result functions**,
   the House panel's result also goes through `showResult`;
   the scheme import's file input is built by `filePicker` (index.html loses it);
   the room template keeps an empty `.room-confirm` for its `confirmBox`.
9. **The smoke test** answers Remove in the panel instead of a browser dialog,
   and gained `update_confirm` (Update asks in the page,
   Cancel sends nothing) besides the planned button-name check.
10. **Commits**:
    Block 3 is six commits, one per file (cubes.js with app.js's found list,
    config.js, theme.js, tiles.js, update.js, app.js);
    `createElement` over `ui/*.js` is 35, 26 of them in components.js.
11. **The set has no configuration shot at 390 px**;
    the inline rows were judged at 390 px from a shot taken for the purpose,
    not committed.
