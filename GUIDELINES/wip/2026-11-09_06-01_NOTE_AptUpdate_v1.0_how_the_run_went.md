# NOTE AptUpdate v1.0: how the run went

How the four blocks of [AP AptUpdate v1.0](2026-11-09_04-42_AP_AptUpdate_update_button_apt_brew.md)
went where they differ from the plan, under the owner's `PLAN++ D2-B` of 2026-11-09,
paused by an `EVAL` and resumed by `EXEC chip`.

1. **Two pull requests, stacked.**
   Blocks 1 to 3 are #127 on `claude/apt-update`;
   Block 4 is #128 on `claude/brew-update`, based on the first,
   as the owner asked during the run.
   `origin/main` was merged into the first branch before its pull request,
   and that branch into the second.
2. **Block 1**:
   `install.AptSource` came with the helper, which needs it, not with Block 3;
   the version is read with `dpkg-query --showformat`;
   `apt-get update` exiting 0 with a "Failed to fetch" warning counts as failed,
   so that a fetch that did not happen is not "nothing newer";
   the health check's interval became a variable for the tests.
3. **Block 2**:
   the postinst also starts `massimo-update.path` when it is enabled,
   so that the helper watches from the install on, not only after a boot;
   the apt test proves that the helper waited for the lock:
   the lock holder exits cleanly, and the helper ran 5 s or more.
4. **Block 3**:
   the question before an update has a text of its own where a package manager
   owns the binary, since apt puts nothing back;
   a UI test covers the apt case with the API stubbed.
5. **Block 4**:
   - the job is restarted with `launchctl kickstart -k gui/<uid>/<label>`,
     not `brew services restart`, so that launchd itself starts the new binary
     even where the helper does not survive the restart;
   - the version brew installed is read from the formula's `opt` link,
     not from `brew list --versions`, which lists the old keg too;
   - `brew update` is left out where `HOMEBREW_NO_AUTO_UPDATE` is set,
     as `brew upgrade` leaves it out (CI's macOS job sets it);
   - the brew route is macOS only:
     under Linuxbrew the button keeps the manager's reason;
   - `helperEnv` is shared by the Windows starter and the brew one,
     and `restartService` became a variable for the tests.
6. **Measured**:
   the full UI suite took 26 minutes for its 440 runs;
   the owner chose to split it by case, as a plan of its own,
   in a session the app offers as a task.
