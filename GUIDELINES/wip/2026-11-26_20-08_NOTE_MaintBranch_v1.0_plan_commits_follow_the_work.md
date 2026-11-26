# NOTE MaintBranch v1.0: plan commits follow the work

After the freeze of 0.17 on 2026-11-26, a session retired AP RoomsPage on
`main`. That left its files on `maint/0.17`, since the upward merge carries
changes from the maintenance branch to `main` and never back.
The other direction hurts more: `commit-msg.sh` refuses a commit that
names a plan and a block of it where that plan is not committed in the
branch's tree, so a pull request into `maint/0.17` cannot cite a plan
registered on `main` after the freeze.

The owner's decisions of 2026-11-26 (EXEC D5-A D6-A):

- **D5-A.** A plan's own commits (its registration, revisions, NOTEs,
  analyses and retirement) go onto the branch its work merges into:
  `maint/<x.y>` while any of its blocks ship in x.y, `main` otherwise.
  The upward merge brings them to `main`, so each is made once.
  A plan open at the freeze is on both branches.
  Its retirement goes onto `maint/<x.y>` if its work shipped in x.y,
  and onto `main` if it did not.
  RoomsPage's retirement was put onto `maint/0.17` as well.
- **D6-A.** The branch keeps the name `maint/<x.y>`, after git's own
  `maint`. It suits a branch that takes alphas before the release and
  fixes after it. `release/<x.y>` would clash with `release/<flavour>`
  and its ruleset. `stable/<x.y>` says "released" before anything is,
  and `<x.y>.x` reads like a version.

The contract's "Branches and pull requests" takes D5-A in a pull request
against `maint/0.17`, so that both lines carry it.
