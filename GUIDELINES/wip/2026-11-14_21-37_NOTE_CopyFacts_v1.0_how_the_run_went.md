# NOTE CopyFacts v1.0: how the run went

How the run of [AP CopyFacts v1.0](2026-11-14_19-05_AP_CopyFacts_copy_and_system_info.md)
went where it differed from the plan; its pull request is #259, and AP
CubeBadges v1.1's (#265) is stacked on it.

1. **Not one function, a few.** Block 1's `copyEl` stands on
   `copyable` (any element copies what a function gives, a button to the
   keyboard), `copyIcon` (the icon alone), `copyLabel`, `copied` (the
   mark and the polite `#copy-status`) and `copyText` (the clipboard,
   else a hidden text area and `execCommand`), since the header's
   version and System Info's copy something other than the text they
   show.
2. **The header's version** copies `state.version` whole; its tooltip
   keeps "(server version)" and adds "click to copy" (`copy.hint`), the
   copied text without it (D94-A).
3. **System Info's version** is the short one, copyable, and an icon
   copying the full version beside it.
4. **The proxies' test** stubs `cube_proxies` in `GET /api/v1/system`:
   the UI tests' server runs with `-proxy off`.
5. **The move of Store and Settings in effect** took `test_restart.py`
   along, and the screenshot of Maintenance no longer opens them.
6. **Main was merged in twice**: with AP AddressCheck (#254), and with
   the plans and NOTEs of the other sessions before #265 was opened.
7. **Verification**: #259's CI, All tests passed; its tests passed again
   in the whole UI suite of #265's branch, 756 on the first pass.
