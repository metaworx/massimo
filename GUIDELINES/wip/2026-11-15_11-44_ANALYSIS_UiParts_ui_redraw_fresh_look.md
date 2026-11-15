# EVAL: the page's redraws, a fresh look

The owner's question, 2026-11-15: the page builds its parts again when
data comes, and loses what the user had open, focused or typed unless
code saves it and puts it back. That saving is a hack every place needs
again. What is a stable, maintainable way?

Read on `main` as of 2026-11-15, which holds the latest UI: the pull
requests #272 (UiPolish) and #281 (PageMenu) were merged that morning.
Nothing was changed, planned or committed.

"The first analysis" below is the EVAL of the session "Web UI updates"
of the same day, 10:40, with its decisions D118 to D121. I read it only
after sections 1 to 4 were settled.

## TL;DR

The page does not have one redraw problem but three, and 22 of 31 cases
tried in a browser against the fake Cube lose something.
The cure is the one the first analysis names: build a part once per key
and update it in place. I reached it independently and agree.
I differ on three things: the store, the guard, and the reasons about a
library.

- **Root:** an element is the data's picture and the user's place at
  once; replacing it to change the picture takes the place with it.
- **Three classes:** data replaces the part (poll, stream) → the user's
  own action replaces it (a step, a toggle, a Save elsewhere in a list)
  → a part left alone to protect it goes stale.
- **Workarounds today:** about thirty, in seven families.
- **Recommended:** parts built once per key, **one** update path each,
  listeners that read the current data, keys that are global.
  Hand-written, no library.
- **Not a store with diffing:** one write function per collection and a
  plain notifier are enough.
- **The guard:** two generic checks that need no test per part
  (nothing interactive is replaced when the data is the same; a part
  updated from A to B looks like one built from B), plus tests where
  data really changes.
- **A library:** only lit-html and Preact with htm pass the page's
  security policy without a build step. Neither is needed; lit-html is
  the one to take if the hand-written updates drift.
- **Two defects found on the way,** both in a part already filled in
  place: Cube settings' Reboot asks with the Cube as it was when the
  page opened, and a file difference that comes later shows its badge
  but not its row.

## 1. How data reaches the page

Three sources write the page's state and then call the render functions
they know by name:

| Source | When | What it calls |
|---|---|---|
| The poll (`poll`, app.js) | every 10 s (the user's setting) | `loadCubes` → `cubesPolled`, `renderCubeSelect`, `renderCubeProblems`, `renderHomeNotice`, `logCubesPolled`, `renderHeader`; `loadRooms` → `renderRooms` only while the stream is down or on Refresh; `loadLogAlerts` → `renderLog` where the alerts changed |
| The event stream (`connectEvents`) | whenever a device reports, and on `hello`, `metadata`, `connected`, `paired`, `device_forgotten`, `resync` | `state` → `updateDevice` → `rerenderRoom` (the whole tile); the others → `loadRooms` → `renderRooms` (every tile) or `refresh` |
| An action's answer | after a click | `cubeAction` → `refresh` + `renderCubes`; `userAction` → `renderUsers`; the same for roles, providers, integrations, backups; a tile's own steps → `rerenderRoom`; the log's toggle → `renderLog` |

A render function clones a `<template>`, fills it, attaches listeners
that hold the data object it was given (`room`, `c`, `u`), and replaces
the old element (`replaceWith`, `replaceChildren`, `clear` + append).

Three places already work the other way and have no trouble: the
header's facts, the Software card and System info are static markup
filled by id.

## 2. What keeps the user's state today

About thirty devices, in seven families:

| Family | Where |
|---|---|
| **1. State copied out of the DOM** and applied again at each build | `state.ui` per room (open, vacation with its two values, confirm, config with tab, edit, copy and result, hold); `unlistedUI`; `cubesUI` (open, edits, foundOpen, linesOpen, pairing with form, seconds, msg and edits); the `open` sets of users, roles, providers and integrations; `usersUI.keys`; `logState.openCommands`; `helpOpen`; `backupUI.diff`; the House panel's `wasChecked` |
| **2. The redraw skipped** while the user is in the part | `renderCubeSelect` (menu open); `renderCubeProblems` (list open); `fillCubeHead` (pill open); `cubesPolled` (a panel open, or busy); `updateDevice` (an unlisted row with its form) |
| **3. The redraw put off** until the focus leaves | `configFocused`, `deferred` and the `focusout` timer in `renderRoom`; `renderRooms` drawing around the kept tile; `planWindowRedraw` |
| **4. A second fill beside the build** | `fillCubeFacts` with `fillLinkState`, `fillAddressState`, `fillProxyState`, `fillCubeHead`, `syncForm`; `drawSetpoint`, `redrawSending`; `renderCountdown`; `devicesLoaded`; `fillHiddenList`; `redrawPairing`; change detection by `dataset.key` and `dataset.value` |
| **5. A running request copied** so that the new button turns too | `ui.sending` + `markSending`; `p.sending` + `markPairing`; `c.sending` + `markConfigSending`; `e.busy`; `ui.removing` |
| **6. Side effects of building guarded** | the "built open" checks in the toggle handlers of `cubePanel` and `userPanel`; `cubesUI.page`; `rendering` with `queueMicrotask` in `rerenderRoom`; `temp.dataset.set` |
| **7. An animation's phase carried over** | `keepBlink` with `marksSeen` |

Even `<massimo-week-program>` is shaped by it: its header says it "owns
no state … so that the edits outlive a redraw of the page around it".

## 3. What loses state

Tried in Chromium against the fake Cube and a server of its own, with a
scratch script (`GUIDELINES/temp/ui-redraw/`, this worktree; it asserts
nothing and may be deleted). Each case sets the page up as a user
would, lets data come the way the page gets it, and reads the state
again. 31 cases: 9 kept, 22 lost. The table in full is in the appendix.

### Class 1: data replaces the part

| Part | Lost when a device of the room reports |
|---|---|
| Room tile | the More menu, the ⋯ menu and the problems list close |
| | the focus on any button of the tile (the stepper, Lock in the device list) |
| | a text selection in the tile |
| | the result line of a write that failed |
| | the holiday form: a temperature typed and not yet left goes back, the focus goes |
| | the virtual wall thermostat's form (expert mode): the address typed |
| | **a click:** a report between the press and the release of a click on + drops the click |

| Part | Lost at a poll |
|---|---|
| Cubes list, every panel closed | a Cube's problems pill closes (**the reported bug**), a selected serial is deselected, the focus on a panel's head goes |
| House panel (a rooms reload) | the focus on a room's checkbox |

### Class 2: the user's own action replaces the part

- **The stepper by keyboard:** Enter on + redraws the tile, and the
  focus is gone after every step.
- **The log:** opening a command's detail redraws every line, and the
  focus is gone.
- **Users:** Create key or Revoke in a panel reloads and redraws the
  whole list; a display name typed in that panel is gone. By reading,
  not tried: so is one typed in another open panel, and roles,
  providers and integrations redraw their lists the same way after
  every action.
- **The configuration's own workaround:** a click from the
  configuration onto the same tile's + moves the focus out, the
  deferred redraw replaces the tile before the release, and the click
  is dropped.

### Class 3: a part left alone goes stale

- **The header's Cube pill**, open, stops following the data: the
  server had two devices with a problem, the open list showed one.
- **Cube settings** is filled in place and never built again while it
  is shown. Two things built once do not follow:
  - **Reboot's question** uses the Cube of the first draw. With 7
    radiograms pending, which the panel's head showed, the question did
    not mention them. By the code of `cube_reboot.go` the server refuses
    such a reboot (409), so nothing is dropped unasked, but the page
    cannot reboot that Cube until its panel is built again.
  - **A file difference** that comes later shows its badge in the top
    line but not its row with "Take the file's value".

Class 3 matters for the choice of cure: it is what goes wrong with
in-place updates when the build and the update are two pieces of code.

## 4. My recommendation

The same base as the first analysis, with four rules that make it hold.

1. **A part is built once per key and never replaced because data
   came.** What the browser keeps on an element then stays without any
   code: open, focus, caret, selection, scroll, a click under way, an
   animation's phase, a value typed and not yet sent.
2. **One update path.** Build is: clone the static template, attach the
   listeners, call `update(data)`. Nothing that depends on data is
   written in the build. Class 3 above is what a second path costs.
3. **Listeners read the part's current data** (`el.item`, or a lookup by
   key when the event fires), never the object the build was given.
4. **A key names its thing in the whole page**, and a change of whose
   list it is empties the list first.
   A room's id alone is not a key: ids repeat from Cube to Cube, so a
   Cube switch would hand Cube A's Kitchen tile, with its typed holiday
   temperature and its pending setpoint, to Cube B's room 1. The key is
   serial and id; a Cube switch and a sign-out drop every part, as
   `selectCube` and `clearCubeData` drop `state.ui` today.

Helpers, all in `components.js`, by my estimate about 150 lines
together:

- **`keyedList(box, items, keyOf, build)`**: existing children updated,
  new ones built, gone ones removed, moved only where the order changed.
- **`setText`, `setAttr`, `setHidden`**: write only what differs, so
  that a selection and a tooltip survive.
- **A field binder**, `syncForm` made general: an untouched field
  follows the stored value; a changed one keeps the draft and says
  "changed meanwhile" with "Take theirs" (D59-A), where the stored value
  can change under it.
- **`problemsEl` with an `update`**: its icons and its list are filled
  again, its `<details>` is never made again. That one change serves
  every pill: the header, a Cube's head, a tile, a device row.

Small leaves without state (an icon, a badge) may still be replaced
where their value changed. A mark that is not replaced keeps its
blinking by itself, so `keepBlink` goes.

Why this fits here: the render functions already clone a static
template and fill it line by line (`renderRoom`, `cubePanel`). Making
the fill run again is a refactoring of each function: listeners move to
the build, appends become `keyedList` or `setText`. No markup moves.

What goes: families 1 to 7 above, but for what must outlive the element
(the page shown, each page's scroll, a pending setpoint and its timer,
the server's pairing state).

What no approach removes, a framework neither:

- a draft whose stored value changed (the field binder's notice);
- a part that goes while the user is in it (a device removed elsewhere):
  the focus needs a place to go, and the draft is gone for a reason;
- displays that change by the clock (ages, an open window's state):
  their timers call `update` as they call the redraw today.

## 5. The first analysis: where I agree and where not

**Agree:**

- the cause of the reported bug (`cubesPolled`'s third path), confirmed
  in the browser;
- no further guard for the pill (its D120-A);
- no generic morph, for its reason, and three more: the fresh tree's
  defaults would overwrite `open`, `value` and `disabled` unless each is
  exempted, which is the same hack in another place; properties set on
  elements do not travel (`d.syncForm`, `row.control`,
  `box.focusFirst`, the week editor's `kit` and `model`); `busyWas` is
  keyed by the element;
- build once per key, update in place, `keyedList`, listeners on the
  current item, a full build at a language change;
- one plan, part by part (its D119-A).

**Its D121 is settled by events:** #272 and #281 are merged; a plan
starts from `main`.

**Differ:**

1. **The damage is wider than it says.** It names the pills and says the
   tiles were not tried. Tried, the list is section 3: focus, selection,
   clicks, typed values, the user's own actions, and stale parts.
   The guard has to cover all three classes, not only "a poll came".
2. **The store.** It makes a store that compares and emits added,
   changed and removed the base of everything, built first.
   I would not:
   - `keyedList` already finds what is new and gone, and an update that
     writes only what differs makes "unchanged" cost nothing in the DOM.
     A diffing store is a second answer to the same question.
   - Parts that change by the clock must update when no data changed.
   - It doubles what each block has to move.
   - Subscribers run in the order the scripts load, where today
     `loadCubes` says the order in one place.
   What I would take from it: **one write function per collection**
   (`setCubes`, `setRooms`, `setDevice`) that the poll, the stream and
   an action's answer all go through, and a plain notifier in the style
   of `onView`, so that app.js stops naming cubes.js' and log.js'
   functions. About 20 lines, no comparing.
3. **`setValue` keeps a draft silently.** Save would then overwrite a
   value someone else stored meanwhile without a word. The Cubes' form
   already says so (D59-A); the helper should be that, made general.
4. **A room's id as the key** leaks state from Cube to Cube (rule 4).
5. **The guard.** One hand-written test per part guards the parts
   someone remembered. Section 7 adds two checks that guard the parts
   nobody thought of.
6. **A library is dismissed in two lines.** The verdict stands, but the
   reasons are in section 6, and they are not "the contract says no".

## 6. How frameworks handle this

Every framework solves this problem, and all in one of two ways.

- **Reconciliation (React, Preact, Vue, lit-html):** the code describes
  the whole part again; the library compares with what it drew last and
  touches only what differs. An element at the same place with the same
  key is the same element, with its state; another key is a new one.
  Listeners are attached again at each render, so they never hold old
  data. An input is *controlled* (the data wins, the code stores every
  keystroke) or *uncontrolled* (the DOM keeps it).
- **Fine-grained reactivity (Solid, Svelte, signals):** a part's code
  runs once and builds its elements; each binding subscribes to the
  values it reads and updates alone. Nothing is ever built again.
  This is exactly "build once, update in place", written by a compiler.

The recommendation in section 4 is the second model by hand, with the
first model's rule about keys.

What the page's policy allows decides most of the rest:
`script-src 'self' 'wasm-unsafe-eval'`, so nothing that compiles
expressions at run time (`new Function`); no inline styles, so nothing
that writes `style` attributes.

Sizes measured on 2026-11-15 from the published files (minified / gzip).
The page's own scripts are 562 kB / 159 kB in 13,700 lines.

| Candidate | Size | No build step | Passes the policy | What it would solve here | What it costs |
|---|---|---|---|---|---|
| **lit-html** 3.2 | 7.3 / 3.2 kB; with `repeat` and `live` 11.6 / 5.7 kB in five module files | yes | yes (no eval; `innerHTML` once, on its own `<template>`, static strings only) | classes 1 to 3: one template per part is build and update; listeners rebound each render; keyed lists | BSD-3, vendored like `opaque.js` with a hash test; a 5-line module shim, as the page's scripts are classic; all dynamic markup moves from `ui/partials/` into JS strings, `data-i18n` there becomes `t()`; three contract lines |
| **Preact + htm** 10.26 / 3.1 | 13.3 / 5.4 kB, one file | yes | yes | the same, and state inside components: the `open` sets and `state.ui` dissolve | MIT, vendored; a rewrite into a component tree, every draft controlled; "no framework" goes for good |
| **React** 18 | 143 / 47 kB | only with `createElement` or htm; React 19 has no script build | yes | as Preact | ten times Preact's size for the same model |
| **Vue** 3.5 | 158 / 57 kB with the template compiler; 100 / 38 kB without | templates need the compiler | **no** with the compiler (`Function`); yes without, but then templates are precompiled (a build) or written as `h()` calls | as Preact | the largest; a build step in practice |
| **petite-vue** 0.4 | 16.9 / 7.1 kB | yes, made for it | **no** (`new Function`, `with`) | — | ruled out; no release since 2022 |
| **Solid** 1.9 | 82 kB unminified / 20 kB | JSX needs its compiler; its tagged templates use `new Function` | **no** without a build | the model of section 4, automated | a build step |
| **Svelte** 5 | compiler output | **no**, it is a compiler | — | as Solid | a build step |
| **Signals alone** (`@preact/signals-core`) | 4.4 / 1.5 kB | yes | yes | the store, with subscriptions found automatically | a dependency for 20 lines of our own |
| **morphdom** 2.7 / **idiomorph** 0.7 | 12.2 / 3.1 kB; 9.3 / 3.3 kB | yes | yes | class 1 only, and only for elements it matches | old listeners on kept elements; fresh defaults over the user's state; no keys, so wrong matches; classes 2 and 3 stay |

On morphing as htmx uses it: there the server sends HTML and the
behaviour is declared in attributes, so a kept element has nothing
stale on it. Massimo's elements carry closures and properties. To make
a morph safe, every listener would first have to read current data by
key, which is rule 3 and most of the work of section 4.

**Adopting one, the true cost and gain.** Only lit-html and Preact with
htm are candidates.

- **The rewrite is the same files either way** (tiles.js, cubes.js,
  config.js, the settings lists, the log's lines), but deeper with a
  library: the markup moves too, 27 partials and their `data-i18n`.
- **The contract** changes in three lines: "no framework"; "no
  `innerHTML`" (lit-html's own use); "a feature's markup is its partial".
  "No build step" holds. A vendored script has a precedent (`opaque.js`,
  435 kB, "taken knowingly").
- **The gain over section 4 is one thing:** build and update cannot
  drift, and listeners cannot be stale, by construction instead of by
  rule and test.
- **What it does not buy:** drafts against stored values, parts that go
  away, global keys. Those stay our code in every framework.

So: not now. The hand-written way is a refactoring of code that is
already half in that shape, and the convergence check of section 7 buys
most of the library's one gain. But section 4 does not close the door:
a part with `update(data)` in a `keyedList` is exactly the shape a
lit-html island has, so it could be taken later for the inside of the
parts that prove hard, without undoing anything.

## 7. Migration and the guard

### The guard

Three kinds of test, in `tools/ui-test`:

1. **Identity, generic.** A helper marks every interactive element of
   the page shown (`a`, `button`, `input`, `select`, `textarea`,
   `summary`, `details`, `[tabindex]`), then runs a poll, a rooms reload
   and a state event for every device, **with the data unchanged**, and
   asserts: none is disconnected, the focus is where it was, every open
   `<details>` is open, every value and the selection are as before.
   Called in each state the suite already reaches (tiles closed, with
   the device list, the configuration's tabs, the holiday form; the
   Cubes list closed and open; Cube settings; each settings list; the
   log). It needs no knowledge of a part, so a new part built anew
   fails it without anyone writing its test.
2. **Convergence, generic.** For each part and a handful of data
   variants (a link held, a problem, a file difference, radiograms
   pending, a window open, mixed modes, a holiday, a stale device):
   a part built from A and updated to B has the same markup as one built
   from B. This is what catches class 3.
3. **Per part, where data must change:** a draft and a stored value
   that changes (the notice); a part removed while focused; a Cube
   switch and a sign-out leave no part of the old list; Reboot's
   question after the pending count changed.

During the migration, check 1 carries a list of the containers not yet
moved (`#rooms`, `#cubes-list`, `#users-list`, …). Each block deletes
its entries. An entry that is no longer needed fails the test, as in
`tools/identifiers`' `allowed.txt`, so the list cannot outlive its use.

### The order

Each block ships alone, with its part of `docs/ui.md`:

1. **The guard, the helpers and the rule.** Checks 1 and 2 with the
   list of what is not moved yet; `keyedList`, the setters, the field
   binder; the rule in the contract and in `docs/ui.md`, "How the page
   is built", which holds for every new part from then on. No visible
   change.
2. **The pill.** `problemsEl` with `update`. Every pill stops closing
   wherever its holder is already kept.
3. **Cubes.** The list and Cube settings, the found fold, the badges.
   Fixes the reported bug and the two defects of class 3.
   `cubesPolled`'s paths, `cubesUI.open`, `edits`, `busy` and the
   skipped redraws go.
4. **The header.** The Cube choice and its pill follow the data while
   open.
5. **The tiles.** A tile, its device rows, the unlisted devices, the
   House panel's rooms. `rerenderRoom` becomes `update`;
   `configFocused`, `deferred`, the copies of running requests and
   `keepBlink` go.
6. **The configuration.** Its tabs as keyed parts.
7. **The settings lists.** Users, roles, providers, integrations,
   backups: a Save in one panel no longer empties another.
8. **The log.** Its lines keyed by id; a toggle updates one line.
9. **The end.** The list in check 1 is empty and removed, with what is
   left of the seven families.

Blocks 2 and 3 are the test of the approach: if their updates need
more code than the builds they replace, or check 2 keeps finding
drift, D122 is asked again before block 5.

## 8. Decisions

**D122. A rendering library**

- **A (recommended).** None: parts and updates written by hand, as in
  section 4. No contract change, no markup moved; drift is held by rule
  and by check 2.
- **B.** lit-html for the inside of each part. Drift impossible by
  construction; costs a vendored library, three contract lines, and the
  dynamic markup moved from the partials into JS.
- **C.** Preact with htm. The conventional model, state inside
  components; costs a rewrite into a component tree and "no framework"
  for good.

**D123. The store**

- **A (recommended).** One write function per collection and a plain
  notifier, no comparing. About 20 lines; the order of the parts stays
  in one place.
- **B.** The first analysis's store, which compares and emits added,
  changed and removed. Skips the work for unchanged items; a second
  mechanism beside `keyedList`, built before anything else moves.
- **C.** Nothing: the loaders keep calling the parts by name. Least
  code; app.js keeps knowing every part.

**D124. The guard**

- **A (recommended).** The two generic checks and the per-part tests of
  section 7. Guards parts nobody thought of, and stale content.
- **B.** A hand-written test per part only, as the first analysis has
  it. Less to build first; a part without a test is not guarded, and
  class 3 is not covered.
- **C.** A, and a check of the sources: `replaceWith`,
  `replaceChildren` and `clear` outside `components.js` fail `go test`
  unless listed with a reason. Catches it before a browser runs; one
  more list to keep.

**D125. What is in the plan**

- **A (recommended).** All nine blocks: also the user's own actions,
  the settings lists and the log. One rule for the whole page.
- **B.** Blocks 1 to 5 now (the guard, the pill, Cubes, the header,
  the tiles); the configuration, the lists and the log when next
  touched. The reported bug and the worst losses are fixed; two ways of
  drawing live side by side for longer, and the list in check 1 stays.

**D126. The two defects in Cube settings** (Reboot's question, the file
difference's row)

- **A (recommended).** With block 3, which removes their cause.
- **B.** Now, as a fix of its own: `rebootCube` reads the Cube from the
  state by its serial, and `fillCubeFacts` draws the difference rows.
  Small and safe; touches code block 3 rewrites.

**D127. Where the rule is written**

- **A (recommended).** One sentence in the contract's "The web UI",
  where an agent trips over it, and the detail in `docs/ui.md`: *a part
  is built once per key and updated in place; data never replaces an
  element the user can hold.*
- **B.** `docs/ui.md` only. The contract stays short; a new part is
  more likely to be built the old way.

## 9. Smaller points for a plan

- **A Cube's panel is drawn in one place at a time**, as its ids are its
  serial's (System settings or Cube settings). Kept parts need either
  one element moved between the two, or ids that name the place.
- **`helpSeq` and `fieldSeq`** make ids that differ at every build.
  Check 2 needs them stable, or left out of the comparison.
- **The language change, the expert switch and the icon choice** are
  the user's own, rare acts: the first builds everything again, the
  second is a class on the body, the third an `update` of every part.
- **Texts at 390 px and in six languages** do not change: no markup
  moves.

## Appendix: the cases tried

Chromium at 1100 px, English, the fake Cube with the suite's greeting;
the timer's own poll stopped, so that every redraw is the case's.
"state" is `updateDevice` with a device as it is, "poll" is
`poll(false)`, "rooms" is `loadRooms()`.

| Result | Part | Case | By |
|---|---|---|---|
| lost | Room tile | More menu open | state |
| lost | Room tile | ⋯ menu open | state |
| lost | Room tile | problems list open | state |
| lost | Room tile | focus on the stepper's + | state |
| lost | Room tile | focus on + after pressing it with Enter | own redraw |
| lost | Room tile | text selected in the tile | state |
| lost | Room tile | result line of a failed write | state |
| lost | Room tile | holiday form: temperature typed, field not left | state |
| lost | Room tile | holiday form: focus in the field | state |
| kept | Room tile | device list open (`state.ui`) | state |
| lost | Room tile | device list: focus on Lock | state |
| lost | Room tile | virtual thermostat's form: address typed | state |
| lost | Room tile | a click on +, a state event between press and release | state |
| kept | Configuration | name typed, focus in it (the deferred redraw) | state |
| kept | Configuration | name typed, then the focus left it | deferred |
| lost | Configuration | a click on the same tile's +, a redraw deferred | deferred |
| kept | House panel | a room unticked (`wasChecked`) | rooms |
| lost | House panel | focus on a room's checkbox | rooms |
| kept | Header | Cubes' problems list open | poll |
| lost | Header | the open list follows a new problem | poll |
| lost | Cubes list | a Cube's pill open, its panel closed | poll |
| lost | Cubes list | serial selected in a closed panel's head | poll |
| lost | Cubes list | focus on a closed panel's head | poll |
| kept | Cubes list | a Cube's pill open, its panel open | poll |
| kept | Cubes list | name typed in an open panel (`syncForm`) | poll |
| lost | Cube settings | a file difference that comes later: its row | poll |
| lost | Cube settings | Reboot's question names the radiograms pending now | poll |
| kept | Users | panel open (`usersUI.open`) | action |
| lost | Users | display name typed, then Create key in the same panel | action |
| lost | Log | focus on a command's button after opening it | own redraw |
| kept | Log | a command's detail open (`openCommands`) | redraw |

<!-- MWX-CONFIG-v1:
[headings]
	numbered = false
-->
<!-- The numbered sections are named by their numbers in the text; the
     TL;DR and what follows the last of them stand outside the count. -->
