# Spike: the room tile as a Preact part and as a Svelte part

Asked for with `EXEC spike` on 2026-11-15, for D122: the room tile built
twice, each with its real toolchain, and measured against today's.

Everything is scratch under `GUIDELINES/temp/ui-spike/` (its `README.md`
says how it is put together and how to remove it). No tracked file was
changed, nothing committed. The packages were installed and every build
run in a container that sees that directory only; nothing went to a real
Cube.

## TL;DR

Both work, and they differ less than I said they would.
Each drew the tile correctly on its first run, keeps all twenty cases of
which today's tile loses sixteen, and passes the page's security policy.
The two expectations I put E first on did not hold up: the Svelte tile is
no shorter than the Preact one, and Svelte lost typed text too, in its
own way. What is left between them is the toolchain, which is D's.

- **Kept under incoming data:** today 4 of 20 → Preact 20 → Svelte 20
  (19 as first written).
- **Tests as they are:** 222 of the suite's 224, both; the two that
  fail test the workaround.
- **Code:** today 699 lines → Preact 586 → Svelte 580.
- **Toolchain:** 5 packages, 0.1 s build, 0.4 s check (Preact) against
  46 packages, 4 s, 8 to 11 s (Svelte).
- **Bundle:** 40 kB (Preact) against 74 kB (Svelte).
- **D122:** back to **D**. E stays a sound second.

## 1. What was built

The tile's drawing, in each framework, with the same markup and classes
as `partials/room.html`, so that the style sheet and the tests' selectors
hold: the head with its problem list and menu, the two temperatures, the
stepper, the mode buttons with More, the holiday form, the delete
question, the result line, the device list with the virtual wall
thermostat's settings.

| Part of today's page | In the spike |
|---|---|
| the 14 functions of `tiles.js` that draw the tile, and its two partials | written again as parts |
| badge, icon, sign, mark, age and the problem list (`components.js`, `app.js`) | written again as small parts |
| the tile's actions (`stepSetpoint`, `tileWrite`, `sendPreset`, …) | plain, as they are, but for their redraw calls |
| a room's facts (`roomThermostat`, `deviceProblems`, …) | plain, called by the parts |
| the configuration (`config.js`) | plain and untouched, filled into a place the part leaves alone |
| the House panel, the devices without a room | plain |

How the two halves meet, the same for both:

- **The page calls the part** through five functions: the rooms were
  loaded, a room's data changed, draw a room again, show a result, and a
  new `state.ui[id]`.
- **`state.ui[id]` became live:** the plain actions set `ui.pending` or
  `ui.sending` as before, and the tile follows. That is why they could
  stay as they are.
- **The part calls the page's globals**, which a file of 68 lines
  declares to TypeScript.
- **Served** as one file in place of `ui/tiles.js`, through a build
  overlay, so neither `index.html` nor `ui.go` changed.

## 2. The numbers

| | Today | Preact (D) | Svelte (E) |
|---|---|---|---|
| **Cases kept of 20** (section 4) | 4 | 20 | 20; 19 as first written |
| **An input showing a stored value keeps what is typed** | – | no | yes |
| **Tile tests passing as they are** (21 tests, four cases each) | 21 | 19 | 19 |
| **Whole suite passing as it is** (224 tests, two cases each) | – | 222 | 222 |
| **Lines of code, drawing and its logic** | 699 | 586 | 580 |
| **The same in characters** | 28.5 kB | 25.4 kB | 26.8 kB |
| **Bundle, minified** | – | 40.1 kB | 73.6 kB |
| **Bundle, compressed** | – | 14.5 kB | 26.8 kB |
| **Of that the runtime** (a part of one button) | – | 21.0 kB | 24.6 kB |
| **Build** | none | 0.12 s | 3.8 to 4.5 s |
| **Type check** | none | 0.4 s | 7.8 to 10.9 s |
| **Packages installed** | 0 | 5, in 3 s | 46, in 22 s |
| **Security policy** | holds | holds | holds |
| **Type errors in the first write** | – | 0 | 3 |

- The lines are the drawing alone: the actions stayed plain in both.
  Both parts add 144 lines of types, which today has none of.
- Preact's five packages are the two it ships and TypeScript; esbuild
  was built from its Go module, which needs only `golang.org/x/sys`.
- The times are from inside the container on this machine, another
  session's suite running beside them.
- Both type checks were shown to work: a misspelt field of the API and a
  wrong property were planted, and each check named both.

## 3. What the tests say

- **19 of the 21 tile tests pass unchanged**, in all four cases, against
  either part (`test_rooms`, `test_room_more`, `test_send`,
  `test_icon_choice`).
- **The two that fail** are the same for both, and fail in their last
  line: `test_typing_in_the_configuration_survives_a_state_event` and
  `test_a_redraw_of_the_rooms_keeps_a_click_in_the_configuration`.
  Each first checks what matters (the typing survives, the click counts),
  which passes, and then that the tile was built again once the focus
  left. A part is never built again, so that line tests the workaround
  and goes with it.
- **The whole suite:** 222 of its 224 tests pass against either part,
  in the two cases run (1100 px English and 390 px German, 448 runs
  each). The two are those above.
  - Against Preact a pairing test failed once in one case, waiting 30 s
    for the server on a loaded machine; it passed when run again.
  - The whole run against Preact was made before its last fix (the
    language, section 5). After it nine files were run again, the tile's
    four with pairing, preferences, settings, stream and home: 80 of 84
    runs, the same two tests failing.
- **No handle was needed.** The tests call `state.rooms`, `updateDevice`,
  `renderRooms` and `rerenderRoom`, and all of them stay in the plain
  half while the parts move. The cost I named for the suite comes at the
  end of the move, not during it.

## 4. What the probe says

The fresh look's sixteen cases for the tile, and four more: the rooms
loaded again under a typed field and under a click, the tile's element
still the same one, and a new setpoint shown.

| Case | Today | Preact | Svelte |
|---|---|---|---|
| More menu, room menu, problem list open | lost | kept | kept |
| focus on +, also after pressing it | lost | kept | kept |
| text selected | lost | kept | kept |
| result line of a failed write | lost | kept | kept |
| holiday form: typed temperature, its focus | lost | kept | kept |
| the same, the rooms loaded again | lost | kept | kept |
| device list open | kept | kept | kept |
| device list: focus on Lock | lost | kept | kept |
| virtual thermostat: address typed | lost | kept | **lost, then fixed** |
| configuration: name typed, also once the focus left | kept | kept | kept |
| a click on + with data between press and release (3 cases) | lost | kept | kept |
| the tile's element is the one it was | no | yes | yes |
| a new setpoint is shown | yes | yes | yes |

None of it needed code to save and put back: `markSending`, the saved
holiday draft, the deferred redraw of the tile all went.

**Each framework lost typed text once, in its own way.** Both were seen
in the browser:

- **Preact:** an input written the short way, `value={stored}`, is set
  back to the stored value whenever its part is drawn again. As read in
  its source, now measured. The tile has no such input: its forms keep
  what is typed as their own state, which is how one writes them in
  React's model.
- **Svelte:** the same input keeps what is typed. But the virtual
  thermostat's form loads its stored address in an effect, and as I
  first wrote it the effect ran again at every change of the room's
  data, and put the stored address over the typed one. One line fixed it
  (the effect now depends on the device's address, not on the object it
  comes from). Preact's effect names what it depends on, and did not
  have this.

So "typed text is kept by default" is true of Svelte for a bound value,
and not for a draft that an effect fills, which is how a form with
loaded settings is written. Neither removes the need for a check that
types into a field, lets data come in and reads it again.

Each also went stale once in my first write, in the way its model
allows: Svelte ran an effect too often, Preact drew a part too seldom
(section 5). Both are one-line faults, and both passed the tile's tests.

## 5. What it took

- **Preact:** correct on its first run, no type error. Two faults found
  later, neither by the tile's tests:
  - the plain configuration hides itself when closed and leaves its
    content, since it was always given a new tile; the part's place for
    it is now emptied (Svelte's got the same line);
  - after a change of the page's language the device rows kept their
    English. A part that reads a signal is no longer drawn again with
    its parent while its properties are the same, which I had not
    expected of React's model. Two parts now read the list's counter.
- **Svelte:** built and drew on its first run. Three type errors, two of
  them a trick of mine the checker rightly refused; the effect above.
  It followed the change of language at once, since every text there
  had to be made to follow from the start.
- **Data stayed plain** in both, with a counter per room that says
  "work this one out again". In Svelte that puts a tile's values into one
  computed block, where Preact computes them in the part's function. With
  the data itself in signals or `$state` both would lose the counters.
- **`data-i18n` does not survive in a part:** `applyI18n` would write
  over text the part owns. Both use `t()` in the markup and follow a
  language change through the list's counter.
- **The page's helpers that write into an element** (`fillAge`,
  `keepBlink`, `dropdown`, `menuArrows`, `confirmBox`) were used as they
  are, from a reference to the element.

## 6. What it means for D122

In the comparison of 14:44 I put E first, and named what would change my
mind: "the spike showing that the tile is no shorter in Svelte". It is
no shorter: 580 lines against 586.

| Reason for E | After the spike |
|---|---|
| typed text kept by default | true for a bound value; a draft filled by an effect was lost |
| less code | not here: equal, and both a sixth under today's |
| markup close to the partials | true, and TSX was no harder to write |
| the compiler refuses an agent's mistakes | both checks caught what was planted; Svelte's takes 25 times as long |

Against that, E's costs were confirmed and measured: nine times the
packages, a build of four seconds against a tenth of one, an older
TypeScript, a bundle nearly twice the size, and the parts outside code
scanning.

**I recommend D.** The two keep the same things and cost the same to
write; D's toolchain is the lighter by every number here, and it leaves
the build in Go. E is a sound second, and the better choice for someone
who values its template format more than those numbers.

What the spike says beyond D against E:

- **The move can go part by part** as the fresh look proposed: plain
  code and parts side by side, the suite green but for the tests of the
  workarounds.
- **Lines are not what a framework saves.** What goes is the code that
  had to be remembered at every place.
- **The guard should type.** The fresh look's identity check would not
  have caught either loss above: the element stayed, its value did not.

## 7. Limits

- One part, written once by one agent in an afternoon. Another hand
  would get other line counts; the two were written to the same shape on
  purpose.
- The actions and the data model were not moved, so nothing here says
  how signals or `$state` carry the page's data.
- The whole suite ran in two of its four cases.
- Preact 11 is six weeks old; nothing in the spike tripped on it.
- Svelte's bundle grows with each part more than Preact's does (49 kB
  against 19 kB for the tile beyond the runtime). On a LAN that decides
  nothing; over the whole page it is worth knowing.

## 8. Decisions

**D122. A rendering library** (order changed by the spike)

- **D (recommended).** Preact with signals in TSX, bundled by esbuild
  from Go.
- **E.** Svelte 5 with TypeScript, built by Vite.
- **A.** None, written by hand: where "no build step" stays.
- **B.** lit-html or Lit without a build.
- **C.** Preact with htm without a build.

**D130. The guard also types** (new; with any of D122's options)

- **A (recommended).** The generic check of D124 also types into every
  field of a part, lets data come in and reads the field again.
- **B.** Typed text is tested part by part only.

**D129** (where the bundle lives) falls away with D122-D: the bundler
is Go, a line before the build wherever Go is compiled.
**D123 to D128** stand as before.

<!-- MWX-CONFIG-v1:
[headings]
	numbered = false
-->
<!-- The numbered sections are named by their numbers in the text; the
     TL;DR and what follows the last of them stand outside the count. -->
