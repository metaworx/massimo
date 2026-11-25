# NOTE AutoUpdate v1.0: how the run went

Plan: `wip/2026-11-24_19-23_AP_AutoUpdate_install_updates_at_night.md`.
Run under the owner's `PLAN++` of 2026-11-24, with AP UpdateHistory v1.1.

## The pull request

All five blocks are #414, on #412's branch, not merged:

| Block | Commit |
|-------|--------|
| 1 | `feat(server/auto-update): The server checks for a release at each start and every six hours` |
| 2 | `feat(server/auto-update): A release is available only where apt or brew has it` |
| 3 | `feat(server/auto-update): The server installs a release at night while the radio is quiet` |
| 4 | `feat(ui/auto-update): Software update sets automatic updates and their window` |
| 5 | `feat(ui/auto-update): The page asks Install now, Tonight or Skip this version` |

Merged in: `merge: main, with the AutoUpdate plan, into the AutoUpdate branch`,
`merge: the UpdateHistory fixes (#412) into the AutoUpdate branch`,
and `merge: main, with the BackupWatch plan, into the AutoUpdate branch`.
#412 goes first; this pull request's base then moves to main.

## Where the run went differently from the plan

### Block 1: ServerCheck

1. **The first check a minute after the start, not at once.**
   A box that has just booted most likely has its network by then.
   Several tests run `Run` with a channel and a fake finder whose fields they
   read without a lock; an immediate check would race with them.
2. **The loop runs without a channel, and checks nothing.**
   It starts wherever there is a store; a channel set later,
   through the setting `update.channel`, is checked within a minute.
3. **The guard against GitHub has two layers.**
   The package's tests replace `defaultFinder` with one that panics,
   and `startServer` refuses a channel without a `Finder`.

### Block 2: SourceCheck

1. **The source is read only where GitHub's release is newer than the running version,**
   cached with the release check.
2. **The hold is in memory, set at a read.**
   `noteHelperOutcome` sets it where it ends the newest entry with `up-to-date`,
   the entry's `to` the version held; it needs a store.
   On a box without a source GitHub's release counts as the source.
   A refreshing check that finds the version ends it,
   POST's own check among them.
   `held` in the answer is the version alone.
3. **An unreadable source sets `source_error` alone,** `latest` and `released` empty.
4. **apt's index over https or `file:` only;** plain http is refused.
5. **brew's formula is read for the project's taps only,** `metaworx/massimo`
   and `metaworx/massimo-rolling`; any other tap,
   CI's `local/massimo` among them, is offered GitHub's release.
6. **The card's badge still says up to date** while the source lags or a version is held.
   The release notes' link names `released`.

### Block 3: NightInstall

1. **Once a night is the store row `update.auto_tried`,** written before each try,
   so that a failed start holds across a restart too;
   the plan had the history's newest automatic entry and a failed start in memory.
   A failed start adds no history entry.
2. **A read-only store tries nothing,** since it cannot record its try.
3. **A write counts as busy while its confirmation is open and not yet late.**
4. **`update_postponed` is kept in memory:**
   after a restart inside the window only what the new process saw counts.
5. **The window's fresh checks do not move the six-hour schedule.**
6. **`next` inside the window, before the night's try, is the moment itself.**

### Block 4: NightSetting

1. **The window is two native time inputs** in one `.field` row,
   not `TextField`, which has no time type.
2. **An end is saved as its field is left,** not at each change;
   the choice of Off or At night saves at once, as the channel does.
3. **The window's row shows only at night.**
4. **The next try is shown at the offset `auto.next` carries,**
   the server's own zone, without the zone's name.

### Block 5: UpdateAsk

1. **Install now keeps nothing;** only Tonight and Skip are kept,
   in `update.answer`.
2. **Tonight and Skip need automatic updates on** (409 while `update.auto` is `off`).
3. **409 also where no update is available,** a version held among them.
4. **A blocked box is not asked:**
   `ask` needs `can_update`.
5. **Skip affects only the night and the question;**
   Update still installs a skipped version.
6. **Install now through the answer is audited as `update.answer`,**
   with the `update` line and a history entry under the person's name.
7. **The banner has the card's background and the accent's border,**
   not the open warning's amber: it informs, it does not warn.

## Left to the owner

1. **The Mac:**
   the shape of brew's install receipt, `{"source": {"tap": …}}`,
   is untested on a real Homebrew install,
   and so is `serveBrewUpdate`'s formula read.
2. **A real night on an apt box:**
   the update installed in the window, the radio's hold and `update_postponed`.
3. **The paths the UI suite does not cover:**
   an answer refused under the buttons,
   and the question going in another browser at the answer's change line.
