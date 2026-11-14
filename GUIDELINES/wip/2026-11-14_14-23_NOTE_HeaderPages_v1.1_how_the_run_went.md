# NOTE HeaderPages v1.1: how the run went

How the run of Blocks 5 and 6 of
[AP HeaderPages v1.1](2026-11-13_20-35_AP_HeaderPages_header_line_and_personal_page.md)
went where it differed from the plan; their pull request is #242.

1. **On a phone the version and the time stand under the mark together.**
   They are one item of the header's row (`.top-facts`), which does not
   fit beside the mark and the page buttons at 390 px. The version is cut
   with … where both do not fit, as a build between releases has a long
   one. `test_header.py` no longer expects the version on the mark's
   baseline under 600 px; it checks both on one line, under the mark.
2. **The refresh icon** moved 3 px down by `position: relative`, which
   leaves the line where it was; the test measures its middle against the
   digits' middle, from the height of a 0 in the time's font.
3. **The scroll position** is restored again while the page grows for a
   second after it is shown, until the user scrolls, as a page that loads
   when shown would otherwise clamp it.
4. **AP Signs** puts the server's sign before the refresh button in the
   same row. Where both pull requests meet, the sign goes into
   `.top-facts` before the button.
5. **Verification**: `make ui-test` whole (684 passed),
   `go test ./internal/server/` and `GUIDELINES/shared/tools/check`;
   the header looked at, 1100 and 390 px, light and dark.
