# NOTE PasswordRules v1.0: how block 3 went

Where Block 3, Common, went differently from the plan
and from the NOTE that left its list out,
the choices taken in the run, and what verified it.

## What it made

A new password that contains a word of the user's login name or display name,
of the server's name, or `massimo` or `maxcube`, whatever the case,
is refused (D50-A), by `massimo user`, by the plain `password` of
`POST` and `PUT /api/v1/users…`, and by the page's five forms.
The rule is one sentence in both languages:
each name gives itself, trimmed of what is neither a letter, a mark
nor a digit at its ends, and each run of letters (with their marks) in it;
a word of 4 letters or more is refused where the password contains it,
both lower-cased a code point at a time by Unicode's simple mapping.
Length is checked first.

In Go it is `password.Check`, which answers `ErrContext`;
in the page `passwordProblem` of `shared/password.tsx`,
which answers `context`; `cases.json` holds both to the same cases.
The line under the field says "Contains the user's or the server's name."
and, after a refused submit, "Refused: has the user's or server's name.";
it follows a name changed after the password.
The API's 400 reads "password: contains the user's or the server's name"
and names no word.

- **One commit:**
  "fix(security:auth): A password with the user's or the server's name is refused".

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **No list of common passwords:**
   the block is the names alone,
   as
   `wip/2026-11-25_03-26_NOTE_PasswordRules_v1.0_no_list_of_common_passwords.md`
   says.
   No `common.txt`, `third_party/seclists-common/`, `ErrCommon` or
   `/ui/common-passwords.txt`; nothing the page fetches,
   and no test of a submit that waits.
   The commit's summary and the CHANGELOG fragment say what is built
   (`CHANGELOG.d/Security/claude-password-names.md`),
   not the plan's "A common password or one's own name is refused".
2. **`ErrContext`'s words,** which the plan does not give:
   "password: contains the user's or the server's name", the API's 400,
   naming no word, so that the answer tells a caller neither the host's name
   nor another user's names.
   `MinWord` (4) and `Always` (`massimo`, `maxcube`) are exported beside it.
3. **`HostWord`,** a function of `internal/password` the plan does not name:
   the first label of a host, without a port after a single colon;
   nothing for anything in brackets or with more than one colon,
   for an IPv4 address (strict: no leading zero, nothing above 255),
   or for `localhost` after the rule's own lower-casing.
   The command line, the server and the page's `hostWord` follow it.
4. **The server's option,** `ServerName`, as the plan's step 4 asks,
   the first label of the host's name where it is empty;
   `massimo serve` leaves it empty, and no flag sets it.
   The first build set it to the announced name, which nothing sets either;
   the wiring and its words were taken out.
5. **`cases.json` has a `hosts` list** beside its cases,
   18 hosts with the word each gives,
   held to `HostWord` in Go and to `hostWord` in the page.
   Its cases grew from 16 to 38: the context, the near cases,
   and two that show the simple lower-casing.
6. **`serverOf` moved to `shared/app.ts`,** with `CardServer`,
   out of `signin/app.ts`:
   the sign-in card and the rule's `serverNames` read the one function,
   since a shared part may import no region.
   The rule also leaves out an app's name that holds `://`;
   the comment says why the card still shows one.
7. **The handles:**
   `MassimoParts.hostWord`, for the UI test alone, as `passwordProblem` is;
   both pure.
8. **The fixtures whose password held its user's name,** renamed:
   - in Go, `anna-pass-N` to `a-pass-00N`, `anna-old-pass` to `a-old-pass`,
     `root-pass-1` to `r-pass-001`, `bert-pass-1` to `b-pass-001`,
     `guest-pass-1` to `g-pass-001`;
   - in the UI tests, the guest's `guest-pass` to `visit-pass` (`run.sh`,
     `tools/ui-test/README.md` and three tests), `owner-pass` to `own-secret-1`,
     `held-pass` to `kept-pass`, `busy-pass` to `slow-pass-1`,
     and Block 2's `rule-password` to `fixed-pw-01`.
   No assertion is weakened.
9. **`TestUITranslations`, a test of another plan,**
   no longer reads the keys of `t()` out of `ui/parts.js`,
   whose minifier had named a function `$t`, and the change failed it falsely;
   it reads the parts' sources, so nothing is lost.
10. **The Go tests' fixed host name:**
    a seam, `hostName`, in `internal/server`,
    set to `kitchen.example.org` in a new `internal/server/main_test.go`
    and in `cmd/massimo`'s TestMain, so that no test reads the machine's.
    The check the first build skipped where the host gave no word is a check.
11. **Beyond the plan's tests:**
    the first owner on an open server in `TestUsersPasswordNames`;
    the page's own host, at `kitchen.localhost`,
    which Chromium resolves to the loopback;
    the two ways an app's name gives no word;
    the stored display name in a user's panel and in My profile;
    the first owner's name followed by its `change` event.
12. **The first owner's name** is kept as typed through a new `typed`
    prop of `UserNameField` (input and change),
    and read again at each submit and opening,
    not through the ref Block 2's NOTE named.

## Choices the plan left open

- **How a name becomes words:**
  itself, trimmed at its ends,
  and each run of letters in it ("anna.berg" gives "berg").
  The other way: the whole name alone, which refuses almost nothing.
- **What a letter is:**
  Unicode's letters, counted a code point at a time;
  a combining mark stays in its run and does not count.
  The other way: ASCII letters alone.
- **The count of four is of letters,** not of characters (D50-A's 4):
  "op7", "2026" and "kim" give no word, "anna2026" gives two.
  The other way: characters, by which "2026" is a word.
- **The simple lower-casing,** alike in both languages:
  too little refused, never too much.
  The other way: a full case folding in both.
- **A display name sent beside the password in `PUT`** is a word too,
  with the stored one; a user's panel takes both alike.
  The other way: only the name the request leaves.
- **A rename refused for a name** gets the same hint as a short one,
  that a rename sets the password again.
  The other way: the hint for the length alone.
- **The app's name gives every run of letters of it,**
  where Go takes a host's first label alone:
  the page can only refuse more.
  The other way: `hostWord` applied to the app's name.

## Which names each side knows

| Where | The server's names |
|---|---|
| `massimo user` | the host's name |
| the users' API | the host's name, or `ServerName`, and the request's `Host` |
| the page in a browser | the host it was opened by |
| the page in the app | that host, and the name the app gives the server |

The page does not know the host's own name,
and its passwords go by an OPAQUE record, which the server never checks:
a page opened by an IP address takes a password with the host's name,
which `massimo user` on that box refuses,
as the plan's "The server's name" decides.
The commit message, `docs/ui.md` and `docs/api-v1.md` say so.

## What the reading found

One builder built it in an exported copy;
two readers read it, one the Go side and the rule's parity,
the other the page, its tests, the texts and the documents.
Nothing had to change.
The rule is one rule in both languages:
a reader's own scan of every code point agrees with the builder's;
the 400 names no word, the rights come before it,
and it stands where Block 1 placed the length.
The ten points of the rework, all done:

1. the serve wiring of `ServerName` and its words taken out;
2. the Go tests given a fixed host name, the skipped check made a check;
3. a Go case for the first owner on an open server;
4. `HostWord` and `hostWord` aligned, with five more hosts in `cases.json`;
5. two cases that show the simple lower-casing, said in their `why`;
6. `serverOf` moved to `shared/`, and the app's words said in the comment
   and in `docs/ui.md`;
7. tests of the four changes no test held:
   the panel's stored display name, the first owner's `change` event,
   My profile's display name, and the app's name left out;
8. the page's own host tested at `kitchen.localhost`;
9. the texts in German, French and Polish covering the display name,
   one term per language, measured again;
10. the message and the documents saying which names each side knows.

**Planted faults:**
the builder's first two were caught (a whole-string lower-casing,
the first owner's name read through its ref);
of the reader of the page's eleven, seven were caught and four not,
the four of point 7; the reader of the Go side took out the check,
the sent display name, the request's `Host` and the stored display name,
each failing a test; after the rework, the builder's six,
one for each new UI test, were each caught.

## The texts and their widths

The two texts, at 390 px, each one line, 20 px tall, after the rework:

| Language | "Contains…" | "Refused…" |
|---|---|---|
| en | 260.8 px | 261.6 px |
| de | 263.4 px | 247.4 px |
| fr | 265.0 px | 271.1 px |
| it | 259.4 px | 248.8 px |
| nl | 249.0 px | 266.3 px |
| pl | 259.5 px | 260.8 px |

Against 279 px in the Add-a-user panel and 297.8 px in the card.
The German and French texts now read "Enthält Namen von Benutzer oder Server."
and "Avec le nom de l'utilisateur ou du serveur.",
the Polish one term, "nazwa użytkownika".
FR, IT, NL and PL are machine-translated and marked.
The first owner's card's room is as Block 2 left it:
no text takes a second line, and `test_the_owner_card_measures` and
`test_keep_open_measures` pass unchanged.

## For the owner: D50-A's consequences

Each follows from D50-A, decided; none is a new decision:

- **A host named `home` or `server`** refuses every new password with that word,
  and the line says "the server's name".
- **For `massimo` and `maxcube`** the user reads the same line,
  "Contains the user's or the server's name.".
- **`PUT` refuses a password that holds the stored display name**
  even in the request that replaces it.
- **The lower-casing is the simple one:**
  a final sigma and a dotless i are not folded;
  too little is refused, never too much, alike in both languages.
- **After a refused submit,** a name changed back to the refused one
  shows "Refused" again until the next key in the password field:
  Block 2's design.

## Seen and left

- **A self `PUT` reaches the rule before the current password,**
  so it tells whether a word is the host's name;
  that adds nothing, since `GET /api/v1/system` gives the host name with
  "read" and mDNS announces it; the reader's, as it is.
- **Letters newer than Go's Unicode tables** are refused by the page alone;
  the rule's, as it is.
- **No text says what "the server's name" is,**
  and none names `massimo` or `maxcube`; the owner's.
- **The English "the user's"** in My profile and the first owner's form,
  not D6-A's "your name": one text serves the five forms; the owner's.
- **`test_admin_updates.py` types "secret-of-the-first"** beside the display
  name "Typed by the first" and never submits it; the test's, harmless.
- **`tools/ui-test/README.md`'s list of files** lacks `test_password_rule.py`
  and `test_password_names.py`, as it lacks many; the documents'.
- **The app emulator was not run;**
  CI's job runs it.

## For the later blocks

- **Block 4:**
  a suggestion goes through the rule with the form's names.
- A new text of the line fits one line at 279 px, the Add-a-user panel's room.
- The Go tests' host is `kitchen.example.org`:
  a fixture's password holds no `kitchen`.
- A new answer of the rule needs its texts in `SAID` and its cases in
  `cases.json`.

## For the completion gate

- **What a user notices:**
  a new password that holds the user's names, the server's name,
  `massimo` or `maxcube` is refused,
  and the line under the field says so while it is typed.
- **No list of common passwords:**
  D55, the owner's.
- **D50-A's consequences,** above.
- **The German and French wording:**
  "Enthält Namen von Benutzer oder Server." and
  "Avec le nom de l'utilisateur ou du serveur.".

## What verified it

- **The builder, on the first tree:**
  `make ui`, `gofmt -l` listing no file, `go vet` for linux, windows and darwin,
  `make ui-check`, `go run ./tools/apitypes -check`, `make cross`, each exit 0;
  `go test -count=1 ./...` and the race tests of `internal/password`,
  `internal/server` and `cmd/massimo`, exit 0;
  `reflow.sh --check` on five Markdown files, 0 errors;
  thirteen UI files in all four cases, 742 passed and 4 failed,
  all four `test_create_is_busy_until_answered` (its fixture);
  after that fix `test_users.py` in all four cases, 76 passed;
  the whole UI suite in 1100-en, 761 passed.
- **Seen failing first:**
  the five new Go tests on the old rule;
  the six tests of `test_password_names.py` on the base page.
- **The reader of the Go side:**
  the four packages' tests, `gofmt` and `go vet` (linux,
  and windows for `internal/password` and `cmd/massimo`), clean;
  a scan of every code point, Go 1.26 against Node 24; the mutations above.
- **The reader of the page:**
  `make ui` and `make ui-check`, clean; `go test` of `internal/password`,
  and of `internal/server` for `TestUI|TestShared|TestState|TestOpenAPI`, ok;
  `test_password_names.py`, `test_password_rule.py`, `test_first_owner.py`,
  `test_keep_open.py`,
  `test_users.py` and `test_parts_guard.py` in all four cases, 350 passed;
  its own widths at 390 px; its three tests of the page's host.
- **The builder, after the rework:**
  `make ui`, `make ui-check`, `gofmt`, `go vet` for linux, windows and darwin,
  `go run ./tools/apitypes -check`, `go run ./tools/credits -check`,
  the whole `go test -count=1 ./...`, `make cross`,
  `reflow.sh --check` on five Markdown files and `commit-msg.sh`, each exit 0;
  the thirteen UI files in all four cases, 766 passed;
  the whole UI suite in 1100-en, 766 passed.
  With the host's name set to `pass` in a user namespace (`unshare -r -u`),
  17 tests failed before and one after, `TestRollbackStoreRunsAsTheStoresOwner`,
  whose `chown` the namespace refuses without the name change too.
  Each new test seen failing without its line.
- **The coordinating session:**
  the message by `commit-msg.sh`, 0 errors; the strict changelog check, passed;
  `GUIDELINES/shared/tools/check`, twelve checks well;
  on the committed tree, on main, `make ui`, `make ui-check`,
  `go run ./tools/credits -check`, `go run ./tools/apitypes -check`
  and the tests of `internal/password`, `internal/server`, `cmd/massimo`
  and `tools/identifiers`, exit 0;
  `test_password_names.py`, `test_password_rule.py`, `test_first_owner.py`
  and `test_keep_open.py` in all four cases, 228 passed.
