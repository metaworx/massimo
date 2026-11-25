# NOTE LooseEnds v1.1: how block 3 went

Where Block 3, Page, went differently from the plan,
the choices taken in the run, and what verified it.

## What it made

- **The page polls once the start's card has closed.**
  Before, the page's start began the poll before it knew who was in:
  behind the sign-in card or the first owner's card it read the Cubes and the log's
  alerts every interval, on a server with users each time refused.
  The walk along the first owner's path saw two such refusals turn a second
  browser's card.
  Now the start begins the poll beside its first read, after the card.
- **A setting applied starts no poll that does not run.**
  The interval changed behind a card the user opened,
  by this browser or another, leaves the poll stopped;
  leaving the card starts it at the new pace.
- **Integrations shows the newest of its reads.**
  Each of its reads takes a number of its kind and of the session,
  and an answer that is no longer the newest asked for is dropped;
  so is one integration's own read.
- **Nothing is said of the LAN before the LAN's answer:**
  the line "This server does not look for Home Assistant on the LAN" waited for
  the list alone and showed for a moment on every server.
- **Two commits:**
  "fix(ui): The page polls once the start's card has closed" and "fix(ui):
  Integrations shows its newest reads and waits for the LAN's answer".

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **The rooms' load after a change of Cube is not in the block:**
   AP RoomMajority's Block 4 had fixed it on main before the plan was written out.
   The poll before the start's card took its place,
   moved here from the card's block, since it is the page's start.
2. **My settings' poll field restarts the poll only where it runs,** as a
   setting applied does.
   The other way: it starts a stopped poll, as before.
3. **One integration's own read takes a number too.**
   The plan numbered the three reads of the card; the first reading showed
   that a panel opened and changed at once could show the opening's older answer.
4. **"The newest" is the newest asked for.**
   Where a newer read fails, the older one is dropped all the same,
   and what was shown stays until the next read.
   Nothing stays busy and nothing is lost for good.
   The other way: keep an older answer where the newer one fails.
5. **The changelog names the reads' order.**
   The plan gave the Integrations card no bullet, as a part not released yet.
   The second reader found the older list drawn over the newer in the released
   card too, so the step amends that card's bullet;
   the LAN's line came with the parts' card and has none.
6. **The suite's kit:**
   `helpers.wait_held` takes a count, to wait for several held requests.
7. **The overlap's test puts a stub into the server's list** where it first made
   real requests:
   two per case met the server's limit of ten requests in ten minutes from one
   address.

## What the readings found

One builder built it in an exported copy on top of Block 2.
The two readers of Block 2 read the recorded trees through git and ran tests in
their own exports: one the code, the other the tests,
the documents and the messages.

Neither found a defect.
Confirmed, by walking every start and every way the poll starts and stops:
the poll starts exactly once on each, and no state leaves it stopped;
the stream does not open behind the start's card;
Block 2's emptying of the card's fields is still reached on its paths;
each new test fails on the tree before for the reason it names.

One must, done: the changelog's bullet (5 above).
Five shoulds, done: a stray edit in a test file; a sentence of `docs/ui.md`;
tests for the interval changed while the poll is stopped and for My settings' field;
the number for one integration's read (3 above); a comment (4 above).

## What else changed by it, worth knowing

- **The version check runs after the start's card.**
  Before, a server updated meanwhile could make the page load itself again
  behind the card, which does not count as something open,
  and empty what was typed there.
  Now that first check after the start comes with the poll.
- **Behind the start's card the server's sign,
  the certificate's line and the version at the card's foot** are as the start
  read them, not renewed every interval.

## Seen and left

- **A newer read of the LAN that fails** leaves an older look's answer unused (4 above).
- **`test_preferences.py`'s test of the poll field** cannot fail on the tree
  before: the field restarted the poll there too.
  It guards against a field that restarts nothing,
  and was seen failing on such a change.

## What verified it

- **The builder, on the last tree:**
  `make ui`, `make ui-check`, `gofmt`, `go vet` for linux, windows and darwin,
  the Go tests of the server, each exit 0;
  the ten UI test files of its brief in all four cases, 550 passed,
  and after the rework the six it changed, 362 passed.
- **Seen failing first:**
  the poll's tests (three and five reads behind the card, not one),
  the LAN's line, the overlap in both orders, a panel's older answer.
- **The readers' own runs** in their exports, in two cases: 16 and 22 passed.
- **The coordinating session:**
  the two trees and their fragments;
  both commits made after the strict changelog check of each;
  the merge with main, in which no file met; on the merge,
  `GUIDELINES/shared/tools/check`, twelve checks all well,
  `make ui` and the page's Go tests.
