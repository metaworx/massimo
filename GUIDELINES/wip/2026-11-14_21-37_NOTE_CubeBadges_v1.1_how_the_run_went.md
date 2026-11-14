# NOTE CubeBadges v1.1: how the run went

How the run of Blocks 7 to 9 of
[AP CubeBadges v1.1](2026-11-14_13-05_AP_CubeBadges_cube_signs_and_problems.md)
went where it differed from the plan; its pull request, #265, is
stacked on AP CopyFacts' (#259).

1. **A test of v1.0 fixed on the way.** `test_cube_panel_pill`'s
   "nothing wrong" check depended on the time: the fake Cube's window,
   open five minutes, became a problem. It stubs a list without problems
   now, a commit of its own citing v1.0, Block 4.
2. **The panel's actions row** (Block 8) takes the pairing's and the
   reboot's parts as they are (`display: contents`), so that their own
   redraws stay as they were; the layout test measures in one go, as the
   page scrolled between its reads.
3. **`host` for a Cube too** (Block 9): a found Cube names the server a
   proxy of it runs on, which says that another server holds it; the
   plan named it for a proxy only.
4. **An unknown answer at an announced server's address is a proxy**,
   as `massimo discover` marks it.
5. **The servers are asked beside each run**, for as long, by multicast
   or at the discovery targets' addresses; a run that hears none keeps
   the last. serve's tests find none (`TestMain`).
6. **The problem list learnt information**: an entry and those under it
   listed without a problem (`info`), an element in the summary (the
   found Cubes' sign), an element as icon, and a detail with a link
   (`tNodes`, `linkEl`, which links an http(s) url only). The group is
   for everyone; its link to System settings for a role with cubes; the
   Settings group no longer counts the found Cubes.
7. **A proxy has no Add and no lines** for the settings file, as the
   mock drew it: it is another server's.
8. **A discovery fix found by the tests**: the UI test of adding and
   removing a found Cube failed under load, as once before. The fake
   Cube answers from this host; added, it went lost before Remove, and
   its search took the fake's answer for the server's own proxy's and
   settled it gone. An answer a Cube gave is never the server's own now:
   `fix(server): Discovery hears a served Cube on the server's own
   host`, with a Go test that fails without it.
9. **Four commits for Block 9**: the fix, the API
   (`feat(api): A found entry says what it is and which Massimo serves
   it`), the UI with the plan's message, and `docs/ui.md` in a docs
   commit, which Blocks 7 and 8 had left as it was.
10. **The scroll test's premise was the found line**: it brings a line of
    its own under the header, hidden in the same layout by a rule added
    through the CSSOM, since the page's policy takes no style element; a
    line hidden by an observer, a microtask late, reproduced the old
    flicker.
11. **The LAN test's check 4** asserts that the searcher names the box's
    server for the Cube, its name and url; its README's text of check 4,
    stale since one entry per serial, follows.
12. **Main went into #259's branch, and that into this one**, before the
    pull request.
13. **Verification**: `go test ./...`, `make race`, `make cross`,
    `gofmt`, the three `go vet`, `make ui-test` whole (756 on the first
    pass), `make lan-test` and `GUIDELINES/shared/tools/check`.
