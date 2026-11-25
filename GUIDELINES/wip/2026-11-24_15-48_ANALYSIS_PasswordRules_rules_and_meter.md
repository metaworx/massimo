# Evaluation: a password strength meter and requirements for a new password

Written 2026-11-24 by the session "UI refactor", for the owner's question:
"EVAL a password strength meter and additional requirements:
required character classes, required length, required complexity."
Drafted by an Opus sub-agent, which opened each source it names;
four of its statements about the code were checked by the session.
The owner took D6 to D12 as recommended, each its option A;
AP PasswordRules carries them.

## TL;DR

- No score meter.
  Use **live checks in words** instead: too short, a common password,
  contains your name.
- **8 characters minimum, fixed** (the owner's decision), **64 maximum**,
  counted in characters.
  A hint recommends 15 or a passphrase.
- **No character classes and no minimum score.**
  NIST forbids such rules; OWASP and ASVS advise against them.
- **An embedded blocklist** of about 2,100 common passwords,
  plus the context words.
  Enforced in the page, in `massimo user`,
  and by the server where it sees the password.
- **No owner's setting** for now.

## What is so today

- **Page, first owner** (`src/signin/Owner.tsx`):
  `NewPassword` twice and `newPasswordProblem`.
  That checks only that the two fields match and are not empty,
  so "a" is accepted.
- **Page, My profile** (`src/profile/Profile.tsx`, Change / Set a password):
  the same pair and the same check.
- **Page, System settings → Users** (`src/admin/Users.tsx`): adding a user,
  and setting a user's password in its panel, use **one `PasswordField`,
  typed once, with no check at all**.
  When adding, empty means keys only.
  In a panel, empty means unchanged.
- **Command line** (`cmd/massimo/user.go`, `readPassword`): `user add`,
  `user passwd` and `user rename` ask twice on a terminal or take
  `-password-stdin`.
  They refuse only a mismatch or an empty password.
  The CLI runs OPAQUE in process (`AddUserOpaque`, `SetPasswordOpaque`),
  so **it sees the password** and Go can check it.
- **Server**:
  `POST/PUT /api/v1/users` still takes a plain `password` (auth_version 1,
  the server hashes it).
  It also takes a `record` + `ksf` (version 2, OPAQUE).
  On the record path the server sees nothing.
  On the `password` path **it does see the password** and could check it
  (`internal/server/users_api.go`).
- **Upgrade**:
  a version-1 user signing in is moved to version 2 with the same password
  (`login.js`, `upgradePassword`).
  A new rule must not apply there.
- **No maximum** anywhere.
- **A setting before sign-in**:
  `GET /healthz` answers `password_reveal` from `auth.password_reveal`
  (`internal/server/server.go`, `store.PasswordRevealKey`).
  `header.js` hands it to the `reveal` signal in `shared/password.tsx`.
  A minimum length or a list version could travel the same way.

## What the guidance says

| Source | Min. length | Max. length | Composition | Blocklist | Meter |
|---|---|---|---|---|---|
| [NIST SP 800-63B-4](https://pages.nist.gov/800-63-4/sp800-63b.html), final July 2025 ([CSRC](https://csrc.nist.gov/pubs/sp/800/63/b/4/final)), §3.1.1.2 | **15** for a password used alone; 8 only within MFA (SHALL) | at least 64 permitted (SHOULD) | "SHALL NOT impose other composition rules"; ASCII, space and Unicode accepted, each code point counts as one, NFC SHOULD | SHALL check against common, expected or compromised values and context words, whole password, saying why | none named; SHALL "offer guidance", above all after a refusal |
| [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html) | under 8 weak with MFA, under 15 weak without | at least 64 | no rules limiting characters; all characters, Unicode and whitespace too | block common and breached passwords (Pwned Passwords) | "Include a password strength meter", suggests zxcvbn-ts |
| [OWASP ASVS 5.0, V6](https://github.com/OWASP/ASVS/blob/master/5.0/en/0x15-V6-Authentication.md) | 6.2.1 (L1): at least 8, 15 "strongly recommended" | 6.2.9 (L2): at least 64 permitted | 6.2.5 (L1): any composition | 6.2.4 (L1): top 3,000 that pass the policy; 6.2.11 (L2): context words; 6.2.12 (L2): breached set | not covered |

All three also say: allow paste and password managers, no periodic change,
no truncation.

In Massimo the password is a **single factor**.
An OIDC provider's MFA does not protect a password sign-in.
So by NIST, 8 is short of the 15 it asks for.
By ASVS L1, 8 is the floor.

## Open decisions

- **D6.
  A strength meter**
  - **A (recommended).
    Checks, not a meter.**
    A text line under the first field lists what is wrong while the user types:
    too short, too long, a common password,
    contains the name or the server's name.
    Otherwise it says "OK".
    It is a `role="status"` live region that announces only when the message
    changes, and it is plain text, so nothing depends on colour.
    All five forms get it.
    Reason: it says only what it can be sure of,
    and it is the guidance NIST asks for.
    Cost: small, about 60 lines and about 6 keys × 6 languages.
  - **B.
    A hand-written meter** (weak / fair / strong, a bar plus the word in text).
    What it can measure honestly: length, how many kinds of character,
    repeats (`aaaa`), runs (`abcd`, `1234`), keyboard rows (`qwerty`, `asdf`),
    the user's name and the server's name, and the blocklist.
    What it cannot measure: dictionary words and names in six languages,
    leetspeak (`P@ssw0rd`), dates, word patterns,
    and how attackers actually guess.
    A naive one rates "Password1!" strong and a long passphrase "fair".
    **A misleading meter is worse than none**,
    because users then trust "strong".
    Cost: medium, about 200 lines, about 10 keys × 6 languages,
    UI tests for every level, and a promise we would have to keep tuning.
  - **C.
    Nothing beyond the length check.**
    No cost, but it falls short of NIST's guidance and of a blocklist refusal
    that says why.
- **D7.
  The required length and who sets it**
  - **A (recommended).
    Fixed: 8 minimum, 64 maximum**, counted in Unicode code points,
    not UTF-16 units.
    Below the field a hint: "At least 8 characters;
    15 or a few words are better."
    Existing shorter passwords stay valid, since nothing can check them.
    The version-1 upgrade at sign-in is exempt.
    Reason: it is the owner's decision, it matches ASVS L1,
    and the hint points to NIST's 15.
    Cost: small.
  - **B.
    Fixed at 15.**
    This matches NIST for a single factor.
    Cost: a harder sign-up on a phone, against the owner's decision,
    and more changes to test fixtures.
  - **C.
    An owner's setting from 8 to 64, default 8.**
    Cost: medium.
    It needs a store key, a switch row in Users, a `/healthz` field,
    the CLI reading the store, texts, and tests.
    It is of little use on a home server with few users.
- **D8.
  Required character classes**
  - **A (recommended).
    None.**
    NIST says SHALL NOT, and OWASP and ASVS say no rules.
    Such rules push users to `Password1!`.
    Cost: none.
  - **B.
    At least three of four classes.**
    Cost: against all three sources, plus texts and an annoyed user.
    It does not stop the commonest pattern.
- **D9.
  A required complexity (a minimum meter score that refuses)**
  - **A (recommended).
    None.**
    A hand-written score is a guess (D6 B).
    Refusing on a guess turns a misleading hint into a wrong rule,
    and it hides a composition rule under another name.
    The blocklist (D10) refuses only what is known to be bad.
    Cost: none.
  - **B.
    Refuse below "fair".**
    Cost: everything D6 B costs, plus refusals the user cannot understand,
    and the same rule needed again in Go for the CLI.
- **D10.
  A blocklist of common passwords**
  - **A (recommended).
    Embedded, small.**
    SecLists `10k-most-common.txt` kept to entries of 8 or more characters:
    **2,087 entries, 19 KB, 10 KB gzipped** (measured).
    One file under `internal/server/ui/`, embedded once.
    The page fetches it as a static file when a password form opens,
    and Go reads the same bytes through `go:embed` for the CLI and the server.
    The check is whole-password and case-insensitive.
    Context words are added: the login name, the display name, `massimo`,
    `maxcube`, and the server's name.
    Reason: it covers ASVS 6.2.4 (top 3,000 that pass the policy) and NIST's
    "commonly used" values.
    Licence: the repository is **MIT** (Daniel Miessler).
    Where the list itself came from is not verified (see Sources).
    Cost: small to medium.
  - **B.
    The NCSC list of 100,000** (`100k-most-used-passwords-NCSC.txt`,
    same repository), kept to 8 or more characters: 47,324 entries, 460 KB,
    224 KB gzipped (measured).
    Cost: a heavier fetch on a phone.
    It adds breached passwords (ASVS 6.2.12, L2).
  - **C.
    Have I Been Pwned's range API.**
    Cost: needs the internet and sends hash prefixes to a third party.
    It fails on a LAN-only server, and it is a new outside dependency.
- **D11.
  Where a rule is enforced**
  - **A (recommended).
    In the page** (all five forms,
    through one function in `shared/password.tsx`),
    **in `massimo user`** (`readPassword`, Go),
    **and in the server for the plain `password` field**, which it sees.
    The rule lives once in Go and once in TypeScript,
    with a test that both refuse the same cases.
    An API client that sends a `record` can still register any password,
    and nothing can stop that: the server never sees it.
    That client already holds `passwords` or `users`, or is the first owner,
    so it is the owner's own tool and choice.
    Say so in `docs/api-v1.md`.
    Cost: small.
  - **B.
    The page and CLI only.**
    This leaves the `password` path open as well.
    Cost: none now, but it is an easy hole to close.
- **D12.
  Settings**
  - **A (recommended).
    None now.**
    The rules are fixed, so the first owner's form needs nothing before sign-in.
    If a setting comes later (D7 C),
    it travels in `/healthz` the way `password_reveal` does.
    Cost: none.
  - **B.
    A minimum-length setting and a blocklist switch.**
    Cost: medium for each.
    Each needs a store key, a row in Users with its hint, a `/healthz` field,
    the CLI reading the store, keys × 6 languages,
    and tests on an open server and before the first owner exists.

## What it would take

| Option | Files | Texts | Tests | Size |
|---|---|---|---|---|
| D6 A + D7 A | `shared/password.tsx` (`newPasswordProblem` and a single-field variant, the status line), `signin/Owner.tsx`, `profile/Profile.tsx`, `admin/Users.tsx` (both fields); `docs/ui.md` | about 6 keys × 6 | `make ui-test`: each form refuses 7 characters and says why; the status line's live region; fixtures under 8 characters (`"pw"`, `"op-pass"`, `"de"`) where they go through the page | small |
| D7 A in Go | `cmd/massimo/user.go` (`readPassword`; `rename` re-asks, so an old short password must be replaced there), a shared check in `internal/store` or a small package | CLI messages, English | `user_test.go` | small |
| D10 A | the list file and its licence note under `third_party/` or `ui/`; a loader in the page; `go:embed` in Go | 2 keys × 6 | a common password refused in the page and in the CLI; the name refused | small to medium |
| D11 A | `internal/server/users_api.go` (the `password` field), `docs/api-v1.md`, the OpenAPI file | none (API English) | an API test: 400 on a short or common `password`; a `record` taken | small |

## Sources

- NIST SP 800-63B-4, HTML: https://pages.nist.gov/800-63-4/sp800-63b.html.
  Opened. §3.1.1.1 and §3.1.1.2 read;
  the rest of the page (about two thirds) was not read.
- NIST CSRC record, status "Final", 31 July 2025,
  supersedes SP 800-63B of 2 March 2020:
  https://csrc.nist.gov/pubs/sp/800/63/b/4/final.
  Opened.
- OWASP Authentication Cheat Sheet,
  "Implement Proper Password Strength Controls":
  https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html.
  Opened.
- OWASP ASVS 5.0, V6 Authentication:
  https://github.com/OWASP/ASVS/blob/master/5.0/en/0x15-V6-Authentication.md.
  Opened (the master branch, not a tagged release).
- SecLists licence (MIT) and the list directory:
  https://github.com/danielmiessler/SecLists/blob/master/LICENSE,
  https://github.com/danielmiessler/SecLists/tree/master/Passwords/Common-Credentials.
  Opened; the sizes were measured on the raw files.
  Where `10k-most-common.txt` itself came from, and under what terms,
  was not checked.
- NCSC blog on the 100k list: the page is removed (archived only).
  Not used.
