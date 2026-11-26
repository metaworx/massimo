# AP CommonPasswords v1.0: Common passwords from Pwned Passwords

## TL;DR

For the next version, built by no signal of this one:
a new password that is one of the commonest is refused,
by Have I Been Pwned's Pwned Passwords, alike in the page,
the command line and the API;
a switch turns the list off, and System info then warns.

- source → Pwned Passwords' range API, every range read once by a tool of its
  own (AccessEnds.D29-E)
- list → the top 10,000 by count, SHA-1 cut to 5 bytes, sorted: 50,000 bytes
- check → length, then common (as typed and lower-cased), then the names
- page → a SHA-1 of its own: no `crypto.subtle` over plain HTTP
- off → an option and a variable, for a test server, say;
  System info warns (AccessEnds.D43-A)
- credits → the terms' words, their one unsettled clause quoted
- open → D1 to D9, each with the option recommended

### Blocks

1. Scan: a tool reads every range and writes the commonest as cut hashes
2. Common: the list refused by the server, the page and the command line;
   the switch and its warning

## Discussion

### What the owner decided

The owner, 2026-11-26, at the completion gate of AP AccessEnds,
whose Block 5 was a list of common passwords:
"D29-E with a cli/env switch to turn it off - e.g. on test servers - showing a
warning in `System info` if set. goes in the next version. so hold off for now."
Then, on the proposal that moved that block into a plan of its own,
"PLAN++ as recommended".
So this plan is for the next version:
no signal of this version builds it, and its decisions are open,
each with the option recommended first.

### What it inherits

- **PasswordRules.D10-A, the aim:**
  a small embedded list of common passwords, the whole password compared,
  whatever its case, one file read by the page and by Go from the same bytes.
  Its source then, SecLists' list, did not hold:
  no plain terms of its own, so no list was built,
  and the rule refuses the names alone.
- **PasswordRules.D55-A, Openwall's `password.lst`,** chosen on its header's
  sentence "It is assumed to be in the public domain.": not taken.
  Its first 3,546 entries, the commonest, stand byte for byte in Openwall's
  wordlists collection of 2011, whose terms forbid charging for a copy and
  bundling it with a product that is charged for,
  which a program under AGPL-3.0-or-later cannot pass on (AccessEnds.D29).
- **AccessEnds.D29-E, the source:**
  Pwned Passwords, on the reading that its terms allow it,
  the one clause they leave unsettled said in the block's NOTE and in the
  credits.
- **AccessEnds.D43-A, the owner's addition:**
  a command-line option and an environment variable turn the list off,
  for a test server, say; where it is off,
  System info shows a warning that says so; their names are this plan's.
- **AccessEnds.D24-A, D26-A and D27-A,** taken for Openwall's list and kept:
  the entry's Licence cell left empty,
  so that System info reads "licence in its text"; the answers' order,
  the length, then common, then the names,
  so that whether a password is common does not depend on the names;
  the list fetched at the first key in a new password's field,
  a submit waiting for it 5 s at most,
  then checking the rest and saying so in the console.
  AccessEnds.D25-A, a pipeline in the note, and D28-A,
  the collection's terms in `NOTICE`, were Openwall's and do not apply.

### Open: the owner's

- **D1.
  Where the top N comes from.**
  - **A (recommended).**
    A full scan of the range API by Block 1's tool, once:
    today's data, the method repeatable though not the bytes,
    the corpus growing every day;
    about 45 GB sent and up to an hour.
  - **B.**
    The downloader's own scan with `curl` and `sort`,
    as one of its issues shows: no code, about 87 GB of disk.
  - **C.**
    The head of Pwned Passwords v8's file ordered by prevalence,
    if it is still served (not verified): the data of December 2021.
- **D2.
  How many.**
  - **A (recommended).**
    10,000: about 4,000 that can refuse, 50,000 bytes.
  - **B.**
    25,000: about 10,000 that can refuse (estimated), 125,000 bytes.
  - **C.**
    The short ones recovered and dropped: a cracking step and a tool of its own.
- **D3.
  Case.**
  - **A (recommended).**
    Refused where the password as typed or lower-cased is listed,
    so that `PASSWORD1` is as common as `password1` (D10-A's "whatever its
    case"); the false refusals twice as many.
  - **B.**
    As typed alone: `PASSWORD1` may pass.
- **D4.
  The cut.**
  - **A (recommended).**
    5 bytes: a password not listed is refused about once in 55 million,
    both lookups counted.
  - **B.**
    6 bytes: 10,000 bytes more, once in 14 billion.
  - **C.**
    4 bytes: once in 215,000, too often.
- **D5.
  The file's form.**
  - **A (recommended).**
    Binary, 50,000 bytes, read without parsing,
    marked binary in `.gitattributes`.
  - **B.**
    Lower-case hex, 110,000 bytes, readable by `grep`;
    never upper case (Analysis, "A trap of upper-case hex").
- **D6.
  The page's SHA-1.**
  - **A (recommended).**
    The page's own code always: one path, synchronous,
    as `passwordProblem` is.
  - **B.**
    `crypto.subtle` where there is one, the page's own otherwise:
    two paths, the check asynchronous.
- **D7.
  The credits' licence file.**
  - **A (recommended).**
    `LICENSE` holds the terms' words, with their addresses and the date read;
    the Licence cell empty (AccessEnds.D24-A).
  - **B.**
    `tools/credits` taught an entry of a notice alone:
    code and tests for one entry.
- **D8.
  What the switch reaches** (raised by AccessEnds.D43-A).
  - **A (recommended).**
    Every check of a new password:
    serve's API and its page, by the option or the variable,
    and `massimo user`, by the variable, which every command reads from the
    settings file too;
    so that a test server's own commands take what its page takes.
  - **B.**
    Serve alone; `massimo user` always checks the list.
- **D9.
  Where the warning shows** (raised by AccessEnds.D43-A).
  - **A (recommended).**
    System info: a warn badge on its card's head and a line among its facts,
    naming the option and the variable;
    and a Warn line in the log at the start.
  - **B.**
    System info's line alone.

### What stays as it is

- The rule's length and names (PasswordRules), the dice and its suggestions:
  a suggestion is never refused by the list (five words of EFF's list or
  twenty of 56 characters).
- AP SecondFactor, also for the next version: not this plan's.
- A release.

## Analysis

Read on main at
`chore(aps): +NOTE AccessEnds v1.1, +NOTE SecondFactor v1.0, the gate's decisions`,
2026-11-26, in the code alone.
The facts on Pwned Passwords were read on 2026-11-25,
between 18:19 and 18:25 CET, on the pages they name,
by an investigation for AccessEnds.D29;
where they say "run", four ranges were fetched; nothing of the project was sent.
The terms were read by an investigator, not by a lawyer.

### For Block 1: Pwned Passwords and the scan

- **The terms, in their words.**
  Have I Been Pwned's Terms of Use (<https://haveibeenpwned.com/TermsOfUse>,
  "last updated on 27 October 2026"), under "Changes to our Services":
  "Pwned Passwords (including the range API and downloadable hash sets) and the
  unauthenticated breach catalogue endpoints (including /breaches,
  /breach/{name},
  /latestbreach and /dataclasses) may be used for any lawful purpose,
  including within commercial products and for the benefit of third parties.
  The third-party, resale and Permitted Purpose restrictions in these Terms
  don't apply to them; the restriction on building a substantially similar
  service continues to apply."
  Among what "You are not permitted to" do:
  "to display results obtained from the Services to any person other than
  yourself without clear and visible attribution to Have I Been Pwned, […]
  Attribution isn't required for Pwned Passwords;".
  The API page (<https://haveibeenpwned.com/API/v3>, undated):
  "In order to help maximise adoption, there is no licencing or attribution
  requirements on the Pwned Passwords API, although it is welcomed if you would
  like to include it." and "There is no rate limit on the Pwned Passwords API."
  Troy Hunt, announcing its first version on 2017-08-03:
  "you're free to use the Pwned Passwords without identifying HIBP as the
  source, simply because I want to remove every possible barrier to use."
- **The unsettled clause.**
  Under "interfere with or disrupt the integrity of the performance of our
  Services in any way, including:", the same Terms forbid "modifying,
  reproducing, revising, transmitting, distributing, reverse engineering or
  altering our Services or HIBP Data, including redistributing your API Key or
  other login details, or its configuration, to any other party";
  "HIBP Data means any Material that is provided to you by us in order for you
  to use our Services."
  The Pwned Passwords sentence lifts "the third-party, resale and Permitted
  Purpose restrictions" by name, and not this one.
  Read as interference, which a list of cut hashes inside a program is not,
  it does not stand in the way;
  a literal reader could hold it against any redistribution.
  AccessEnds.D29-E builds on the first reading and says the clause in the
  block's NOTE and in the credits.
- **What the terms settle:**
  no licence in the sense of copyright, and no condition either:
  nothing forbids a fee, a bundle or a change,
  or asks that terms go with each copy,
  so nothing clashes with AGPL-3.0-or-later's "no further restrictions";
  the restriction kept, a substantially similar service,
  binds whoever uses the Services.
  Attribution is not required and is given all the same,
  which keeps clear of the API page's "Misrepresenting the source of the data
  as originating from somewhere other than Have I Been Pwned".
  "Have I Been Pwned®" is a registered trade mark of Superlative Enterprises Pty
  Ltd: its name in the credits, no logo.
- **The range API** (run, four ranges):
  `GET https://api.pwnedpasswords.com/range/<5 hex>` answers lines of
  `<35 upper-case hex>:<count>` in CRLF, in the hashes' order,
  about 2,000 a range (`00000` 2,510, `5BAA6` 1,978, `FFFFF` 2,047,
  `21BD1` 1,925), about 40 KB as sent, with a `Content-MD5`,
  in about 0.15 s each.
  The order is the hashes', so the top N needs all 16^5 = 1,048,576 ranges:
  about 2.2 billion hashes,
  45 GB sent and 87 GB unpacked (extrapolated from the four, ±10 %);
  6 minutes at 1 Gbit/s, an hour at 100 Mbit/s;
  Troy Hunt's took 26 minutes in 2022.
  A tool that keeps a heap of the top N needs no disk;
  the count of the N-th is unknown until the scan.
- **No ordered file today:**
  version 8 (2021-12-20) had a SHA-1 file ordered by prevalence;
  since 2022 the site names its downloader alone, which reads every range.
  Whether `downloads.pwnedpasswords.com` still serves version 8's file was
  not verified.
- **The UK NCSC's list of the top 100,000,** made from Pwned Passwords in 2019,
  is gone from its site (`404`), and its terms are not the data's.

### For Block 2: the list, the rule, the page and the switch

- **Today:**
  `internal/password` holds no list,
  its package comment saying so (`password.go:12-13`);
  `Check` (`:88`) takes the length (`ErrTooShort`, `ErrTooLong`),
  then the names (`ErrContext`, `:52-60`); nothing in `cmd/`,
  `internal/` or `tools/` imports `crypto/sha1`.
  The page's `passwordProblem` (`src/shared/password.tsx:99-106`) is
  synchronous, run at each key, and answers a `PasswordAnswer` (`:58`);
  its one cryptography is `crypto.getRandomValues`,
  the suggestion's (`src/shared/suggest.ts`).
  Both are held to `testdata/cases.json` (`TestCases`, `password_test.go:66`;
  `test_the_page_answers_each_case_as_go`, `test_password_rule.py:189`).
  The callers: `massimo user` (`cmd/massimo/user.go:221`, the rename's hint
  `:524`) and the users' API (`newPassword`,
  `internal/server/users_api.go:140-145`).
- **The shape:**
  a hash does not tell its password's length,
  which Troy Hunt left out on purpose (version 2),
  and the commonest are mostly short: of the NCSC's top 10,000 of 2019,
  3,864 lower-cased ones have 8 characters or more.
  So about 55 to 60 % of a top 10,000 can never refuse,
  the length being checked first, and about 4,000 can:
  near Openwall's cut of 3,510, and above the 3,000 of ASVS 6.2.4
  (an estimate from 2019's data).
  Dropping the short ones needs their texts, every short string hashed:
  a cracking step (D2-C).
- **The cut and its false refusals:**
  a password not listed is refused where its cut hash meets one of N,
  about N / 2^(8 × bytes), twice that with two lookups (D3-A):

| N | 4 bytes | 5 bytes | 6 bytes | binary, 5 bytes | hex, 5 bytes |
|---|---------|---------|---------|-----------------|--------------|
| 10,000 | 1 in 429,497 | 1 in 110 million | 1 in 28 billion | 50,000 bytes | 110,000 bytes |
| 25,000 | 1 in 171,799 | 1 in 44 million | 1 in 11 billion | 125,000 bytes | 275,000 bytes |

  At 10,000 and 5 bytes, 50,000 bytes against the word list's 62,144,
  served uncompressed as every file of the page is.
- **The page's SHA-1:**
  `crypto.subtle` is a secure context's alone (Web Crypto's `[SecureContext]`,
  from the specification, not run): undefined on `http://<a LAN address>`,
  and asynchronous where `passwordProblem` is not.
  So the page gets its own, `src/shared/sha1.ts`:
  FIPS 180-4 over `new TextEncoder().encode(password)`, about 60 lines,
  no library (Preact stays the one vendored), the same over HTTP and HTTPS;
  one or two 64-byte blocks a key, microseconds;
  a binary search of 10,000 takes 14 steps.
  A lone surrogate is encoded as U+FFFD,
  as Go's JSON decoder makes it of the API's body, so both hash the same bytes.
- **The server's SHA-1:**
  `crypto/sha1`, whose `Sum` panics under `GODEBUG=fips140=only`
  where `Write` returns an error;
  Massimo sets no `fips140`, but input never panics:
  `sha1.New()`, its error skipping the list and said in the log.
  AP SecondFactor's HMAC-SHA-1 shares the case.
- **The file and its route:**
  `internal/password/common.bin`, N × 5 bytes, sorted, each once,
  ties at the N-th count broken by the hashes' order;
  served at `GET /ui/common-passwords.bin` as `/ui/credits.json` is served from
  another package's bytes (`internal/server/ui.go:196-199`),
  `application/octet-stream`, with no token: the first owner's form has none.
- **A trap of upper-case hex:**
  a text file of 10 upper-case hex digits a line matches `tools/identifiers`'
  serial pattern, `\b[A-Z]{3}[0-9]{7}\b` (`identifiers_test.go:25`),
  about 20 times per 10,000 lines (simulated);
  lower case never, and binary never in 1 MB (D5).
- **Hashes hide nothing:**
  the top 10,000 SHA-1s fall to any word list in seconds;
  the hash is Pwned Passwords' form, not a secret.
- **The credits:**
  a `third_party/` directory with a `MASSIMO.md` table,
  a `LICENSE` and a `NOTICE` becomes an entry without a change to
  `tools/credits` (`noteRows`, `tools/credits/generate.go:561`; `licenceFiles`,
  `:384`; `classify`, `:490`; `entryOf`, `:322`),
  as `third_party/eff-wordlist/` does;
  an entry without a licence file stops the tool (`:342`),
  so the terms' words go into `LICENSE` (D7);
  a text named by no licence and an empty Licence cell read "licence in its
  text" (`src/sysinfo/Credits.tsx:44, 51`);
  `TestClassifyToday` needs the entry in `licencesToday`
  (`generate_test.go:372`).
- **The texts' widths:**
  the widest German line of the rule today, 39 characters,
  is 263 px of the Add-a-user panel's 279 px at 390 px,
  and the new German "now" text has 39 characters too (measured for AccessEnds'
  Block 5, not since).
- **The switch's models:**
  a switch flag, as `-announce` is (`newSwitchFlag`,
  `cmd/massimo/switch.go:63-74`; `serve.go:90-91`),
  its variable a name of `internal/env` (`env.go:88-89`; the names known,
  `:215`), which System info's settings in effect list;
  every command reads the settings file (`cmd/massimo/main.go:28`),
  `massimo user` too (D8);
  the page learns a server's setting before anyone signs in from `/healthz`,
  as it learns `password_reveal` (`internal/server/server.go:1427-1431`);
  `GET /api/v1/system` is System info's (`system_api.go:40-64, 82`),
  and its card shows a warn badge as the proxy's failing one is shown
  (`src/sysinfo/SysInfo.tsx:327-330`).
- **The tests' passwords:**
  each password of the fixtures (`OPERATOR_PASSWORD`,
  `tools/ui-test/helpers.py:40`, and the Go tests') is hashed against the
  built file in Block 2: it could not be before the list exists.

## Implementation Plan

For the next version: no signal of this version builds it.
Each block is a pull request of its own,
its run recorded in a NOTE (AGENTS.md §5.4); Block 2 waits for Block 1's run,
whose output it commits.

### Block 1: Scan

- **Goal:**
  a tool, run once by a maintainer,
  reads every range of Pwned Passwords' range API and writes the commonest N as
  SHA-1s cut to 5 bytes, sorted (D1-A, D2-A, D4-A, D5-A).
- **Changes:**
  1. `tools/pwnedtop/` (new, the standard library alone):
     the 1,048,576 ranges fetched by a pool of workers,
     with a user agent naming the tool and nothing of the project,
     each answer held to its `Content-MD5` as the downloader holds it,
     a failure tried again; a min-heap of (count, hash) of N,
     ties at the N-th count broken by the hashes' order; N × 5 bytes written,
     sorted; printed: the date, the ranges read, the entries seen,
     the N-th count and the file's SHA-256.
  2. Its flags: `-n` (10,000), `-bytes` (5), `-o` (the file), `-workers`.
  3. `tools/pwnedtop/README.md`:
     what it does and costs (about 45 GB and up to an hour),
     that its bytes cannot be had again (the corpus grows every day),
     and that the committed file is pinned by its test.
- **Tests:**
  `tools/pwnedtop`, on an `httptest` server of a few made-up ranges:
  the top N across ranges, ties, the cut, the order,
  an MD5 that differs tried again,
  and a range that fails at every try stopping the run with its name.
  Offline; seen failing before the tool.
- **Documents:**
  the README.
- **CHANGELOG:**
  none: `tools/` is not shipped (`:!tools/**`), which the message says.
- **Commit:**
  `feat(tools): A scan of Pwned Passwords writes the commonest as cut hashes`
- **Verification,** each by its exit code:
  `go test ./tools/pwnedtop/...`; `gofmt -l .` prints nothing;
  `go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...`;
  `GUIDELINES/shared/tools/check`.
  The run itself once, online, by the maintainer at Block 2's start,
  its printout kept for Block 2's `NOTICE`.

### Block 2: Common

- **Goal:**
  a new password among the commonest is refused by the server, the page and
  the command line alike, unless the switch turns the list off,
  which System info then says (AccessEnds.D29-E, D43-A, D24-A, D26-A, D27-A;
  D3-A, D6-A to D9-A).
- **Changes:**
  1. `internal/password/common.bin` (new): Block 1's output;
     `.gitattributes` marks it binary.
  2. `internal/password/password.go`:
     `go:embed`; `Common() []byte` for the route; `ErrCommon`, "password:
     a common password"; `Check`: the length,
     then the password as typed and lower-cased (`fold`),
     each by its SHA-1 cut to 5 bytes and a binary search,
     then the names (AccessEnds.D26-A); `sha1.New()`,
     an error skipping the list, never a panic;
     the list asked unless the switch turned it off,
     a value the start sets once, for every caller;
     the package comment loses its "no list" sentence,
     and `cases.json`'s description gains the answer `common` and a section
     `hashes`.
  3. `internal/server/ui.go`:
     `GET /ui/common-passwords.bin` beside `/ui/credits.json`,
     `application/octet-stream`, an ETag of the list's hash, no token;
     the file's comment names it.
  4. `/healthz` and `GET /api/v1/system` gain `common_passwords`,
     false where the switch turned the list off.
  5. `src/shared/sha1.ts` (new):
     the SHA-1 of a text's UTF-8 bytes, synchronous, with its `@file` header;
     `MassimoParts.sha1Hex` for the tests.
  6. `src/shared/password.tsx`:
     the answer `common`, `SAID.common`; the list fetched once,
     at the first key in a new password's field (AccessEnds.D27-A),
     as `wordList` fetches its words, held to its form (a multiple of 5 bytes,
     ascending), a failure tried again at the next call;
     `passwordProblem` asks it after the length,
     where it is loaded and `/healthz` says it is on,
     the line drawn again when it comes;
     a submit of the five forms waits for it 5 s at most,
     then checks the rest and warns in the console;
     `MassimoParts.commonPasswords`.
  7. The switch:
     `serve -common-passwords=off` and `MASSIMO_COMMON_PASSWORDS=off`,
     the names this block's to settle (AccessEnds.D43-A),
     a switch flag as `-announce` is, its variable in `internal/env`,
     read by `massimo user` too (D8-A);
     where it is off, a Warn line at the start,
     and on System info's card a warn badge and a line naming both (D9-A).
  8. `cmd/massimo/user.go`:
     the rename's hint for `ErrCommon` too.
  9. `third_party/pwned-passwords/` (new),
     in the form of `third_party/eff-wordlist/`: `LICENSE`,
     the terms' words with their addresses and the date read (D7-A); `NOTICE`,
     the source and its address, the scan's date, ranges and entries, N,
     the cut, the N-th count, the file's SHA-256,
     the attribution "Pwned Passwords,
     by Have I Been Pwned (https://haveibeenpwned.com)",
     and the unsettled clause quoted with the reading taken (AccessEnds.D29-E);
     `MASSIMO.md`, its Licence cell empty (AccessEnds.D24-A),
     and Block 1's command; `tools/credits/generate_test.go`'s `licencesToday`;
     `make credits`.
  10. `ui/i18n/*.json`, FR, IT, NL and PL machine-translated and marked:
      `password.common` "A common password:
      easy to guess." / "Ein häufiges Passwort: leicht zu raten.";
      `password.refusedCommon` "Refused: a common password." / "Abgelehnt:
      ein häufiges Passwort."; System info's warning,
      such as "The list of common passwords is off ({option}):
      any password of 8 to 64 characters is taken.";
      each measured at 390 px in the Add-a-user panel,
      the rule's line reserving the tallest (`RULE_TEXTS`).
- **Tests,** each seen failing on main first:
  - `internal/password`:
    `TestCommonList`: `common.bin` at its SHA-256, N × 5 bytes,
    strictly ascending, and `MASSIMO.md` naming the SHA-256, N and the date;
    `TestHashes`, the section `hashes`:
    FIPS 180's `abc` (`a9993e364706816aba3e25717850c26c9cd0d89d`),
    the empty text (`da39a3ee5e6b4b0d3255bfef95601890afd80709`),
    the 56-byte message (`84983e441c3bd26ebaae4aa1f95129e5e54670f1`),
    texts of 55, 56 and 64 bytes, multi-byte UTF-8 and an astral character;
    `cases.json`: `password1`, `PASSWORD1` and `12345678` common,
    `password1x` taken;
    `TestErrorTexts` gains `ErrCommon`; with the switch off, `password1` taken.
  - `internal/server`:
    `TestUsersPasswordRule`,
    `password1` answered `400` `password: a common password`,
    and taken on a server with the switch off; `TestUIServed`,
    `GET /ui/common-passwords.bin` `200`, its body `password.Common()`;
    `/healthz` and `GET /api/v1/system` say the switch.
  - `cmd/massimo`:
    `TestUserPasswordRule` refuses `password1`,
    and takes it with the variable off; serve takes the option and the variable,
    and refuses a value it does not take at the start.
  - `tools/ui-test/test_password_rule.py`:
    the parity test gives the `common` cases once the list is loaded,
    and every `hashes` case to `MassimoParts.sha1Hex`;
    `Password1` says the common line in each form;
    a submit with the list's fetch held sends nothing until it is let go,
    then is refused;
    a list that fails gives the length and the names and a console warning;
    with the switch off, the page takes `password1`.
  - `tools/ui-test/test_system_info.py`:
    the warning where the switch is off, none where it is on;
    the entry among the credits.
  - Every password of the tests' fixtures hashed against the built file,
    none in it.
- **Documents:**
  `docs/api-v1.md` ("Users", the `400`; `/healthz`; `GET /api/v1/system`);
  `docs/api-v1-openapi.yaml`, `src/api.d.ts` by `go run ./tools/apitypes`;
  `README.md` (`massimo user`, serve's switch);
  `packaging/README.md` and `docs/store.md` where the variables are listed;
  `docs/ui.md` (the line's texts, the rows of `password.tsx` and `sha1.ts`,
  the route, System info's warning).
- **CHANGELOG:**
  `CHANGELOG.d/Security/claude-commonpasswords.md`:
  "- A new password is refused where it is one of the commonest,
  by Have I Been Pwned's Pwned Passwords,
  unless serve's -common-passwords=off turns the list off,
  which System info then says."
- **Commit:**
  `fix(security:auth): A common password is refused, by Pwned Passwords`
- **Verification,** each by its exit code:
  `make ui`, then `MASSIMO_ENV_FILE=/dev/null go test ./...`;
  `MASSIMO_ENV_FILE=/dev/null MASSIMO_TEST_SCALE=2 go test -race -count=1 ./internal/server`;
  `gofmt -l .` prints nothing;
  `go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...`;
  `make credits`, then `go run ./tools/credits -check`;
  `go run ./tools/apitypes -check`; `make cross`; `make ui-check`;
  `PYTEST_ADDOPTS='test_password_rule.py test_password_names.py test_password_suggest.py test_first_owner.py test_users.py test_system_info.py' make ui-test`
  in all four cases; the whole suite once, `UI_TEST_CASES=1100-en make ui-test`;
  `GUIDELINES/shared/tools/changelog.sh check --staged --strict`;
  `GUIDELINES/shared/tools/check`.

## Proposed commit message

Registered with AP AccessEnds v1.2, in one commit of the plans' own:
`chore(aps): +AccessEnds v1.2, +CommonPasswords v1.0, -NOTE AccessEnds v1.1 owner_decisions_of_the_gate`,
its trailer
`File-revision-v1: GUIDELINES/wip/2026-11-26_16-58_AP_CommonPasswords_a_list_of_common_passwords.md::v1.0::2026-11-26::All::first revision, for the next version`.

Block 1: `feat(tools): A scan of Pwned Passwords writes the commonest as cut hashes`.

Block 2:

```
fix(security:auth): A common password is refused, by Pwned Passwords

For AP CommonPasswords v1.0, Block 2 (AccessEnds.D29-E, D43-A, D24-A, D26-A, D27-A; D3-A, D6-A to D9-A): a new password among the 10,000 commonest of Have I Been Pwned's Pwned Passwords is refused, as typed or lower-cased, by the users' API, massimo user and the page alike, the list held as SHA-1s cut to 5 bytes. serve's -common-passwords=off, or MASSIMO_COMMON_PASSWORDS=off, turns it off, for a test server, and System info then says so. The credits name the source, its terms and the one clause they leave unsettled.

Key changes:
- internal/password: common.bin, Block 1's scan; Check asks it after the length, before the names; ErrCommon.
- internal/server: GET /ui/common-passwords.bin; /healthz and /api/v1/system say whether the list is on.
- src/shared/sha1.ts, src/shared/password.tsx: the page's own SHA-1; the list fetched at the first key, a
  submit waiting for it 5 s at most.
- cmd/massimo: the switch and its variable, which massimo user reads too.
- third_party/pwned-passwords/: the terms' words, the NOTICE, the credits made again.
- Tests: TestCommonList, TestHashes, the common cases; the API's, the command line's and the page's
  refusals; the switch; System info's warning.
- Docs: docs/api-v1.md, the OpenAPI file, README.md, docs/ui.md; a Security fragment.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-26): first revision, from the owner's answer at the completion
  gate of AP AccessEnds (AccessEnds.D29-E, D43-A) and "PLAN++ as recommended":
  AccessEnds' Block 5 moved here for the next version,
  with what was found on Pwned Passwords; D1 to D9 open, the owner's.
