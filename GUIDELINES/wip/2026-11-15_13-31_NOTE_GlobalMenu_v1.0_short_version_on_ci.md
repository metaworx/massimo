# NOTE GlobalMenu v1.0: short version on CI

A fix of [AP GlobalMenu v1.0](2026-11-15_11-39_AP_GlobalMenu_menu_pages_backups_and_age.md)
after its run, in its pull request #289.

1. **CI's UI test at 390 px failed** in `test_header_on_one_baseline`
   and `test_server_connected_and_update`. CI's build has the short
   version "v0.0.0-dev". With the header Rooms and ☰ alone (Block 3) and
   the time a short age (Block 6), the version, the sign and the time
   now fit beside the logo on a phone, as `docs/ui.md` says they should
   where they fit. The baseline test took for granted that they never
   fit there; it passed locally only because a local build's version is
   long. The sign's test measured a frame before the header fits its
   facts again.
2. **Reproduced locally** with `GOFLAGS=-buildvcs=false`, which gives
   the test server CI's short version.
3. **The fix:** the baseline test sets a short version, beside the logo,
   and a long one, under it on a phone, so that the build's own version
   no longer decides what is tested; the sign's test waits for the refit
   (`set_version` in `helpers.py`). Both pass with the short and the
   long build, at 1100 and 390 px, in both languages. The page itself
   is unchanged.
