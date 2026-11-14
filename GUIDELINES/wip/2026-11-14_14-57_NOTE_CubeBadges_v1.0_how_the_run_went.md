# NOTE CubeBadges v1.0: how the run went

How the run of [AP CubeBadges v1.0](2026-11-14_13-05_AP_CubeBadges_cube_signs_and_problems.md)
went where it differed from the plan; its pull request, #251, is
stacked on AP Signs' (#246).

1. **No `link_reason`.**
   AP LinkOutage's `link.state` already says why a Cube is not connected
   (held, stale, busy, rebooting, offline), and the page maps it to the
   network's sign (`LINK_SIGN`). Block 1 adds `found_address` alone.
2. **`problems` are device views.**
   Each Cube's devices with a problem come as the device list shows them,
   with their room's name, so that the page builds their badges with the
   tiles' own `deviceProblems`. The configuration's warnings were in the
   list already (`proxy_failing`, `file_differs`), and so are the Cubes
   found and not served; no `warnings` field was needed.
3. **The header's settings group** is an entry of the same list, the cog
   with a triangle, for one with the settings right. A window open
   briefly is a state, not a problem, and is not listed there.
4. **The Cube choice stays a menu of the page** (`pickerMenu`), which
   took icons already; `connIcon` and its two texts go, the network's
   sign in their place.
5. **A click on a Cube's pill** opens its list and not the panel: the
   pill's own summary toggles it alone, and a click in its list is kept
   from the panel's head. The source badge moves from the head to the top
   line while the panel is open.
6. **The drafts** follow the plan for the six fields of the form; the
   hidden devices' list keeps its own editor's edits, as before.
7. **The tests** read a Cube panel's badges from its top line, and open
   a panel by its own summary (`:scope > summary`), not the pill's.
   Two did not at first: `test_cube_panel_columns`, and
   `test_language_redraw`, which fails in the whole suite only, once the
   fake Cube's window has been open five minutes and is a problem of the
   Cube, its pill then in every panel's head.
8. **AP Signs' branch took in main** (#241, #242) and moved the server's
   sign into `.top-facts`; this branch took Signs in by a merge.
9. **Verification**: `make ui-test` whole (the language test fixed
   after it), `go test ./...`, `make race`,
   `make cross`, `gofmt`, the three `go vet` and
   `GUIDELINES/shared/tools/check`; the header's pill, the closed and the
   open panel looked at, light and dark, 1100 and 390 px.
