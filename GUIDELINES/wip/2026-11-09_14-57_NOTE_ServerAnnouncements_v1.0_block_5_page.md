# NOTE ServerAnnouncements v1.0: block 5 page

How Block 5 went, run on 2026-11-09 at the owner's `EXEC++ 4-5`.

## Where the work went differently from the plan

1. **Block 5 started before Block 4 was merged.**
   The plan merges a block's pull request before the next block that
   depends on it starts; the owner's `EXEC++ 4-5` asked for both in one
   run, so Block 5's branch, `claude/server-announcements-page`, started
   from Block 4's.
   Block 4 (#136) was merged while Block 5 ran, and the branch merged
   `origin/main` with it before its pull request, which is based on `main`.
2. **A notification's text is shared.**
   `ui/notifytext.js` renders a notification from the keys
   `notify.<kind>.title` and `.body`; the page's toast and the service
   worker, which has no `i18n.js`, both load it.
3. **The worker learns the language from the page.**
   The plan had the page write the `ui` preference's language into the
   worker's storage at sign-in.
   The page posts the language it shows at its start and at each change of
   language; the worker keeps it, and that language's table, in its cache,
   for a push that comes while the server is out of reach.
4. **A push while the page is in front is the page's toast.**
   The worker hands it to the page in front rather than showing the
   system's notification, and the page shows one toast for the stream's
   copy and the push's of one notification.
5. **The page's place and order.**
   Notifications is a page of its own in the 👤 menu, as My settings is,
   at `#notify`;
   the kinds are listed with the window and frost first, the informative
   ones, off by default, last.
   A device's name is asked before turning on; left empty, the server takes
   the browser's `User-Agent`.
6. **The rules are read with every preference** (`GET /api/v1/me/preferences`),
   since `GET …/preferences/notify` is a `404` until the user has set one,
   which the browser logs as an error.
7. **The icons** are the header's house in white on the page's accent,
   192 and 512 px, rendered from an SVG by the suite's headless Chromium;
   the 512 is the maskable one too, its house inside the safe zone.
   The manifest's colours are the light page's; the dark one is a
   `theme-color` of its own.
8. **The CSP names `worker-src` and `manifest-src`,** which `default-src`
   covered already.
9. **The tests:**
   - a fake `pushManager` stands in for a push service, which a headless
     browser lacks, put in by an init script and a reload;
   - the page's own `sw.js` takes a push that Chromium's DevTools deliver:
     the toast while the page is in front, and, the worker made to see no
     window and to keep what it would show, the system's notification in
     the language told; a headless browser has no notification permission,
     so no notification is shown for real;
   - `clear_preferences` removes the owner's `notify` before every page.
10. **The screenshot set** gains Notifications at 1100 px in light and dark,
    as a browser shows it that has not been asked yet, since a headless one
    denies notifications; the 👤 menu's shots and the slideshow change with it.
11. **The plan's manual check was not run.**
    No phone was used: the page's install, the permission asked once, and a
    window opened at the fake Cube notifying after the delay are the owner's
    to try with the go.
12. **The quality pass had no IDE.**
    `node --check` for the scripts, `py_compile` for the suite, a JSON parse
    of the manifest, `gofmt` and `go vet` stood in for it.
