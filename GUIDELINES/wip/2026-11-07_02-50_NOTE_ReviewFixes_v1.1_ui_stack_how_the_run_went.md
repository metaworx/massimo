# NOTE ReviewFixes v1.1: ui stack how the run went

How Blocks 4 and 5 of
[AP ReviewFixes v1.1](2026-11-07_01-32_AP_ReviewFixes_review_fixes.md),
the stack without Go, went where they differ from the plan,
under one `EXEC++` on 2026-11-07.
The finding numbers are those of
[the analysis](2026-11-07_01-03_ANALYSIS_ReviewFixes_main_since_2026-11-05.md).

1. **Branches and pull requests.**
   Block 4 is `claude/review-ui` against `main` (#71),
   Block 5 `claude/review-ui-2` against `claude/review-ui`, as AP Tls did.
   `origin/main` had not moved since the plan's registration,
   so there was nothing to merge into `claude/review-ui`.
   No Go file is touched, test helpers included:
   the test's own API calls are Python (`tools/ui-test/conftest.py`,
   `api_call`).
2. **Every finding reproduced first.**
   Each fix's test failed on main before the fix, as its commit message says,
   but for findings 4 and 32, which no test can show (item 3),
   and lows 70 and 71, refactors that the fixes of 67 and 64 carry;
   nothing is recorded as not reproduced.
3. **Block 4.**
   - Low 63: `make packages` gives nfpm `DEBVER`,
     with `version_schema: none` in `packaging/nfpm.yaml` so that nfpm takes it as it is;
     `packaging/deb/test.sh` compares the control Version with the file name.
   - Finding 35: the keyring version is the publish's UTC time to the **second**,
     `1.0.<YYYYMMDDHHMMSS>`, not to the minute as the plan wrote:
     two publishes within a minute stay ordered,
     and the test publishes a second key a second after the first.
     Found on the way, and only documented:
     a *new* key would reach no box even with a newer keyring package,
     since the site is then signed with a key the boxes do not hold yet;
     that takes a keyring package with both keys on a site the old key still signs,
     which `publish.sh` does not make (packaging/README.md says so).
     It is for a later revision; no block names it.
   - Finding 31: the bootstrap installs the keyring package first from
     `apt/massimo-archive-keyring.deb`, a name that does not change,
     and the snippet's `Signed-By` names the package's file;
     `massimo.asc` stays on the site for a check by hand,
     instead of serving the first download as the plan's parenthesis had it.
     Found on the way and fixed with it: the keyring package installed its key
     with mode 0600 (nfpm copied the temporary export's mode),
     so apt, reading it as `_apt`, found the repository unsigned.
   - Finding 4: `publish.yml` is a reusable workflow (`workflow_call`,
     called by release.yml's new `homebrew` job after `sign`) and can be run by
     hand (`workflow_dispatch` with a tag) for a release already out;
     a run for a formula the tap has already changes nothing.
     No test can show a workflow that never starts:
     actionlint v1.7.7 passes both workflows and catches a misspelled secret in the call.
     **The proof is the next release's run,
     with `HOMEBREW_TAP_DEPLOY_KEY` set.**
   - Finding 32: documentation only;
     the repository has no `github-pages` environment yet (the API answers 404),
     so the refusal shows only after the owner sets the Pages source.
   - Not done, for the coordinating session:
     the plan's Block 4 asks the roadmap's packaging steps to gain the Pages rule,
     and the roadmap to note that the next release proves the Homebrew job,
     but `docs/roadmap.md` is the coordinating session's alone, as section 3
     of the contract says.
     The lines for it:
     - Environments → `github-pages` → Deployment branches and tags:
       a rule for the tags `v*`.
     - Once the tap exists: `gh workflow run publish.yml -f tag=<newest release>`.
     - The next release with the tap's key set proves the Homebrew job (finding 4).
4. **Block 5.**
   - Findings 5 and 27: a panel built open loads at once (users) or keeps what
     it holds (Cubes), and its toggle handler loads only a panel the user opens,
     its name not yet in the set of open panels.
     The plan's other way, the listener added after `open`, would not help:
     setting `open` queues the toggle in a task of its own,
     which reaches a listener added afterwards.
     Found with 5: New API key got another listener at every load of the tokens,
     so a panel closed and opened again sent two POSTs; it is wired once.
     Opening the settings now loads the pairing and rooms of the panels still open.
   - Finding 29: Log out sends a change still waiting under the user's token,
     rather than cancelling it, then forgets the user's values and schemes,
     for a user whose settings followed it;
     a guest's settings are the browser's own and stay.
     Log out's own requests no longer open the sign-in dialog on a session already gone.
   - Finding 30: no tombstones; a scheme made in a browser that did not sync,
     while the server already had a list,
     is dropped at that browser's next sign-in.
   - Finding 34: the time input redraws nothing,
     neither while it is typed nor when it is left:
     a redraw on blur took away the Send button being clicked,
     which the test showed.
     A typed time is marked in place (`markProgram`: day buttons, bar, note,
     resets), the rows take the order of their times at the next draw,
     and Send sorts.
   - Low 64 with low 71 in one commit, low 67 with low 70 in one,
     so that neither refactor goes without the fix and the bullet it carries;
     the request helper is `apiRequest` in app.js, beside `api()`,
     not in components.js, which holds the page's elements.
   - No new text, so no translation file changed.
5. **Verification.**
   Block 4: `make deb-test` on trixie and `make apt-test` on trixie and bookworm here,
   CI's Debian package job on both,
   shellcheck and `bash -n` on the changed scripts, a YAML parse and actionlint.
   Block 5: the whole pytest suite at 1100 and 390 px in English and German,
   176 passed, 52 of them the 13 new tests' four cases;
   `make ui-screenshots` took the 27 shots byte for byte as committed,
   so none is in the commits;
   `MASSIMO_ENV_FILE=/dev/null go test ./internal/server/...` and `make cross`,
   `node --check` on the changed scripts and `py_compile` on the tests;
   the quality pass ran on these fallbacks, no IDE being attached.
   `GUIDELINES/shared/tools/check --commit-range` is clean on both branches.
6. **For the merge.**
   Both stacks add `### Fixed` under `[Unreleased]`,
   so the second stack to reach `main` resolves that one heading in `CHANGELOG.md`.
