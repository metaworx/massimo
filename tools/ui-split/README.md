# Cutting the web UI's scripts along their regions

`split.py` cuts a script of the web UI (`internal/server/ui/`) into one
script per region of the page, by moves alone, so that later work on one
region touches only that region's files (AP UiParts, Block 1).
It is a transition tool: it goes when the plan ends, with this map.

```sh
python3 tools/ui-split/split.py                  # cut every script the map names
python3 tools/ui-split/split.py --only app.js    # cut one
python3 tools/ui-split/split.py --check          # the tree is the map's cut
python3 tools/ui-split/split.py --compare A B    # the cut of A is B
python3 tools/ui-split/split.py --list tiles.js  # a script's top-level items
python3 tools/ui-split/split.py --self-test      # the tool's own tests
```

Python 3 and its standard library; no network, no browser.

## What it does

- It reads each script the map names,
  and the scripts it has been cut into already, as **top-level items**.
  An item starts on a line at column 0 that is not blank,
  not a comment and not a closing bracket,
  and goes on over the lines that are indented,
  blank or start with a closing bracket:
  the scripts' indentation puts everything nested further in,
  as `TestUIScriptGlobals` reads them too.
  An item whose brackets do not close stops the tool.
- An item is a **declaration** (`function`, `async function`, `const`, `let`,
  `class`), named by its name, or anything else,
  named by `> ` and its first line:
  a statement such as `> document.addEventListener("DOMContentLoaded", init);`,
  a section heading (`> // --- header ---…`),
  or a comment that a blank line parts from what follows it.
- Each item moves with the **comment block directly above it**, byte for byte.
  A section heading is an item of its own, so it moves where the map puts it,
  as any other.
- Each script gets its items **in the map's order**.
  An item is written after the blank lines it had where it was read if it
  follows the same item as there, after one blank line otherwise,
  so the result is the same bytes each time.
- A **new script** opens with the SPDX and copyright lines, the map's
  `about` sentence as a comment, and `"use strict";`.
  A script that is there already keeps its header: everything up to its
  `"use strict";` line.
- It puts the new scripts into `ui.go`'s `//go:embed` line and the page's
  `<script src>` tags in `index.html`, in the place of the script cut,
  in the map's order.
  The tags' order is the order in which the scripts' top-level code and
  their `DOMContentLoaded` listeners run; a statement or a `const` or
  `let` initialiser that needs, at any depth of calls, an item of a
  script that loads after it stops the tool, unless the tree it started
  from had that already.
- It stops, and writes nothing,
  on an item of a cut script the map does not name,
  on a name the map gives that it does not find, or finds twice,
  and on a script the map gives no item.
- On a tree it has cut it changes nothing.

`--check` makes the cut in memory and fails where the tree differs.
`--compare A B` cuts a copy of A,
a directory or a git tree (a commit or a tree id),
and compares the UI's scripts, `index.html` and `ui.go` with B;
for two git trees it also names every other path the two differ in.
That is how each move is checked:
`split.py --only tiles.js --compare <commit>^ <commit>`.

## The map

`map.txt`, one group per script that is cut:

```
[tiles.js] rooms.js tiles.js devices.js unlisted.js house.js
about rooms.js: The Massimo web UI, the rooms: …
  a line indented continues the about line above it
rooms.js   STEP
rooms.js   > // --- room facts -------------------------------------------------------
tiles.js   renderRoom
```

- `[script] scripts…`:
  the script cut and the scripts it is cut into, in their load order;
  the script cut may be one of them and keep the rest.
- `about script: text`:
  the sentence of a new script's header, wrapped at 72 columns.
- Then every item of the group, with the script it belongs in, in the
  order the scripts get them: a declaration by its name, anything else
  by `> ` and its first line, as `--list` prints them.
- `#` starts a comment line.

## An open branch

A branch that was open while main took the moves follows each move so:

1. Merge main at the commit before the first move:
   the branch then has the tool and its map.
2. Run the tool with main's map (`split.py`, for the script main moved),
   and commit: the branch has made the same moves on its own code.
3. Merge the move itself with `-X ours`,
   which takes the branch's side where the two differ,
   in the moved code the branch had changed; the rest of the move's commit,
   its CHANGELOG fragment for one, comes in as it is.
   (`-s ours` would drop it,
   and the branch's later merge would delete it on main.)
4. Go on with main.

Where a branch added a top-level item to a script that is cut, the tool
stops at step 2 and names it: add it to the map, in the place and the
script it belongs in, and run the tool again.

## Cutting again after a merge of main

When main is merged into a branch that has cut a script, and main had
changed an item the branch moved, the merge brings main's version back
into the script cut (the change and the move meet there), while the
branch's older copy is in the script it moved to.
Run the tool again: the script cut's version takes the place of the copy,
in the copy's place, and leaves the script cut.
An item that is new on main stops the tool until the map places it.
An item that main deleted is not seen: the tool cannot tell a deletion
from a move, so the copy stays in the script it moved to.
List main's items against the map (`split.py --root <a tree of main>
--list <script>`), and take the copy and its line in the map out by
hand; `go test` finds one that reads an id the page no longer has.
Then `split.py --check`, and the tests, again.
