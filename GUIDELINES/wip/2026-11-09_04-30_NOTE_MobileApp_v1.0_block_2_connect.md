# NOTE MobileApp v1.0: block 2 connect

How Block 2 went, run on 2026-11-09 at the owner's `EXEC`,
with D4-A and D5-A, the owner's at the block gate.

## Decisions

- **D4-A**:
  on a server's page the bridge has `token()` and `setToken(t)` without
  an id, and nothing else but `version`:
  the adapter answers for the server it opened, by the origin of the
  message, and a server's page can never ask for another server's token
  or learn its address.
  The Analysis table's `token(id)` and `setToken(id, t)` are gone.
- **D5-A**:
  `axe-playwright-python` (MIT, axe-core 4.12.1 inside, MPL-2.0) in
  `tools/ui-test/requirements.txt`; CI's pip step installs it, and it was
  installed into the main checkout's venv, `tools/ui-test/.venv`.

## Where the work went differently from the plan

The contract is `docs/app.md`, the one source for Blocks 3, 7, 8 and 9;
where it and the plan's Analysis table differ, `docs/app.md` holds.

1. **Every call returns a Promise**, and the adapter injects
   `window.massimo` at the document's start over its message channel:
   `WebViewCompat.addWebMessageListener` and
   `addDocumentStartJavaScript` on Android, which give the origin of each
   message that D4-A needs, a `WKScriptMessageHandlerWithReply` on iOS.
   The plan's Block 3 names `@JavascriptInterface`, which offers its
   object to every frame and origin and answers synchronously,
   so a biometric prompt could not answer through it.
2. **The calls changed shape:**
   `probe(urls, fp)` takes the fingerprint to accept beside those the
   system trusts, so that a server not yet saved is probed with the one
   the user just compared;
   `open(id, url)` names the address the probe chose, and the adapter
   refuses any other than the server's two;
   `save(server)` resolves to the server as saved, with the id the adapter
   gave it, and keeps `fp` as `trust` set it, `trust` being the one call
   that pins a certificate;
   `core` and `push` are objects present only where the app has them,
   and the page hides the card and the notification switch without them.
   The events are `found`, `network`, `core` and `notification`,
   the `found` event's fields named after the TXT keys of
   AP ServerAnnouncements Block 2 (`v` as `version`).
3. **The page's URLs.**
   `connect.html` carries `<base href="../">`, so that its URLs resolve as
   those of the page at `/` (`ui/i18n/de.json`, which `i18n.js` fetches),
   and its policy in a `<meta>` element for the app's copy,
   which is served without the server's header;
   the app serves its copy of `ui/` under `ui/` of its origin.
4. **An address typed** is probed at `https://host/`, `http://host/`,
   `https://host:8099/` and `http://host:8099/`, `host:port` by HTTPS and
   then HTTP; a private address becomes the LAN one, any other the remote.
5. **The Go tests** checked `index.html` alone:
   `TestUIPage` now checks the ids `connect.js` looks up against
   `connect.html` (`uiPageScripts`), and `TestUITranslations` the
   connect page's `data-i18n` keys.
6. **Beyond the plan's files**:
   `tools/ui-test/fake_bridge.js`, the fake bridge the tests inject,
   and `tools/ui-test/README.md` and `requirements.txt`.
   The server's page's token hook is tested in `test_connect.py` too.
