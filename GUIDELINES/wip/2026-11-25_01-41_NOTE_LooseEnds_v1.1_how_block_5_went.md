# NOTE LooseEnds v1.1: how block 5 went

Where Block 5, ServerTests, went differently from the plan,
the choices taken in the run, and what verified it.
It changes test files of `internal/server` and `docs/testing.md` alone;
nothing shipped behaves differently.

## What it made

Four tests that failed now and then on a slow or loaded runner wait for what they check:

- **`TestConfigWriteConfirmed`:**
  the fake Cube applies the write after 300 ms,
  and the test's own re-reads come after 50 ms, then 500 ms and a second,
  both scaled.
  The wait ends on the write closing or on a `not_confirmed` line,
  which fails the test by name, not by a timeout.
- **`TestProxyRetryStops`:**
  the old address is never given back, so the test holds it throughout.
  The held ports count the listens at each address,
  and after each step the test sees that no further one comes.
  The dial to a freed port, which another test's listener could answer, is gone.
- **`TestLogOfEveryCubeWithoutStore`:**
  it waits until the fake Cube has its six state lines and the second Cube's log
  its `disconnected`, then reads.
- **`TestLogAfterReadsABurstOnce`:**
  the one wait over the whole burst is a wait for progress.
  The test fails where no new line of the burst is read for the scaled wait,
  and says how far each reader and writer got.
- **One commit:**
  "test(server): Four tests wait for what they check, not for a fixed time".

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **`confirmWithin` stays at a minute.**
   The plan scaled it to three seconds,
   on the premise that a write that lost the race stays open until that
   deadline.
   It does not: a configuration write is judged at the follow-up's last re-read,
   and the deadline skips such writes.
   A reader confirmed it in the code.
2. **The burst is 1000 lines per writer, not 3000,** on AP Flakes' measurements:
   it still gives fifteen pages of the fake Cube's log, twenty of every Cube's
   and about forty store batches, in a third of the time.
   With a wait for progress the size no longer makes the test fail;
   it only costs time.
   The other way: 3000, at about three times the time.
3. **One planted delay per test, not the same for the old and the new form,**
   where the new form makes the old delay impossible:
   the new retry test holds the port, so another listener can only try and fail.
4. **`TestLogAfterReadsABurstOnce/store` is in this block and in AP Flakes' open
   D8,** which recommends that it stays here.
   That plan's session was told what this block changes,
   and that its analysis names a wait this block removes.

## What the reading found

One builder built it in an exported copy;
one reader read the recorded tree and ran the tests in its own export.
No defect.
For each of the four tests the reader planted a fault in the product,
at the heart of what the test protects, and saw the new test fail:
a check of a configuration that always reports a field missing;
a stop that neither cancels nor waits; a log read from the first Cube alone;
a page of the log off by one line, and a writer that stops.
Nothing an old test caught is lost.

Two small points, done:
the failure of the confirmation test names what was seen,
not a timing race alone;
the comment at the retry test's end says why a fixed time is right there (the
stop waits for the slot's end before it answers,
and the test holds the address).

## Seen and left

- **`GUIDELINES/shared/COMMIT.md` §5.1 asks every commit that touches a shipped
  path for a CHANGELOG bullet, and exempts no test file.**
  The project's test-only commits write "No CHANGELOG bullet:
  only tests change", and so does this one.
  The rule and the practice differ: the owner's to settle,
  by a sentence in the rule or by taking test files out of the shipped paths.
- **`TestProxyRetryStops` sometimes takes six to twelve seconds** even where it passes,
  before this change too.
- **`docs/testing.md`'s paragraph on a `-race` run by hand** is AP Flakes' to
  write; untouched here.

## What verified it

- **The builder:**
  each old test seen failing under its planted delay,
  each new one passing with it.
  The four tests under the race detector, twenty times each at scale 2,
  alone and beside `make race`: 80 of 80, twice.
  Under armv6 emulation, five times each at scale 4: pass.
  `gofmt`, `go vet` for linux, windows and darwin, the package's tests,
  each exit 0.
- **The reader:**
  the planted product faults, each failing its test;
  the four tests five times each under the race detector; the package once.
- **The coordinating session:**
  the tree's six files; the commit after its message check and the changelog
  check as the project runs it for a test-only commit; the merge with main,
  which brought NOTEs alone; on the merge, `GUIDELINES/shared/tools/check`,
  twelve checks all well.
- **CI's job "Test (armv6, QEMU)"** on the pull request is the last word.
