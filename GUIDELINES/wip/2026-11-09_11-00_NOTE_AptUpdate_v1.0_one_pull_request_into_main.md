# NOTE AptUpdate v1.0: one pull request into main

How the two pull requests of
[AP AptUpdate v1.0](2026-11-09_04-42_AP_AptUpdate_update_button_apt_brew.md)
became one, under the owner's `EXEC` of 2026-11-09 to fix CI on #128
"so one fix closes both on merge of #128".
It follows [the run's NOTE](2026-11-09_06-01_NOTE_AptUpdate_v1.0_how_the_run_went.md),
whose first point it changes: #128 is no longer based on #127.

1. **#128 carries #127.**
   `origin/main`, with the apps' connect page (#125), was merged into
   `claude/brew-update`, and #128's base moved from `claude/apt-update` to `main`.
   Merging #128 merges Blocks 1 to 4, and GitHub marks #127 merged with it.
   #127's own conflict with `main`, in `CHANGELOG.md`, was resolved in that merge:
   main's bullet first, then the two update bullets.
2. **The Windows failure** was `TestBrewKegOf`:
   `brewKegOf` reads the keg with `filepath`,
   so on Windows the prefix comes back as `\opt\homebrew`.
   The test now expects it through `filepath.FromSlash`, and still runs there.
   The PowerShell error in the job came after that test failure,
   from the step that collects the service's logs.
3. **A new CI run.**
   A change of base starts no run, and a re-run reads the old base again;
   the run started by the push read the range from `claude/apt-update`,
   which takes in GitHub's "Merge pull request #125" commit,
   and its Guidelines job failed.
   #128 was closed and reopened, so the next run reads the range from `main`.
