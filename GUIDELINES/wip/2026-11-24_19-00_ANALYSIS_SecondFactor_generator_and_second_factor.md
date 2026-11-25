# Evaluation: a password generator, and a second factor

Written 2026-11-24 by the session "UI refactor", for the owner's two questions:
"can we offer a generator? random char-sequence or words?" and
"can we add MFA (TOTP and maybe token-by-e-mail)?"
Drafted by an Opus sub-agent, which opened each source it names
and measured the lists it downloaded;
five of its statements about the code were checked by the session.
The owner's answers: D14-A, D15-C, D16-A to D18-A,
D19 as a dice icon beside the eye, D20-A to D25-A, D26-B and D27-A.
D28 became three plans: Part 1 is built by AP PasswordRules,
Part 2 by AP SecondFactor.

## TL;DR

- **Generator: yes, small.**
  One "Suggest" control in the shared password fields:
  five words from EFF's long list (about 65 bits),
  shown once with Copy and Another.
  No new library.
- **Word lists:**
  English (EFF) everywhere first.
  German and French CC0 lists exist and Italian is CC BY 4.0.
  No Dutch or Polish list with a clear licence was found.
- **Second factor: TOTP first**, with recovery codes.
  Go's standard library covers it.
  The QR code is the one piece that needs vendored code.
- **No e-mail codes:**
  Massimo cannot send e-mail,
  and NIST says e-mail SHALL NOT be an out-of-band factor.
- **Passkeys later**, as a plan of their own.
  They need a domain name and a trusted certificate,
  which a box reached by IP does not have.
- **Covered:**
  password sign-ins (OPAQUE and version 1).
  **Not covered:** API keys, the box and OIDC, which is the provider's job.
  Making a key requires the second factor.

## Part 1: a generator

### What is so today

- **Forms that set a password:**
  the first owner's form (`src/signin/Owner.tsx`,
  `NewPassword` read through refs) and My profile's Change and Set a password,
  which is one form (`src/profile/Profile.tsx`, `NewPassword`).
  In System settings → Users (`src/admin/Users.tsx`) there are two more:
  adding a user (line 237) and a user's panel (line 146),
  each a single `PasswordField`.
- **Every new-password field already carries `autocomplete="new-password"`**
  (`shared/password.tsx` lines 253-254, `Users.tsx` 146 and 237).
  Google says this attribute "enables password generation" in Chrome and Google
  Password Manager.
  Desktop Chrome and phone browsers with a manager are covered.
- **Who is left without a generator:**
  a wall tablet or a shared browser with no manager signed in,
  someone who types the password on another device,
  and the Android app's WebView.
  Android's autofill framework works from Android 8,
  but no page I opened says whether it generates passwords inside a WebView,
  so this is not verified.
  The app's README leaves "the phone maker's autofill" to the check by hand on a
  phone.
- **The eye switch:**
  `auth.password_reveal` reaches the page before sign-in through `/healthz`
  and the `reveal` signal in `shared/password.tsx`.
  With it off, no field shows its eye.
- **The page is served uncompressed.**
  Nothing in `internal/server` gzips, so raw sizes are what a phone downloads.

### Open decisions

- **D14.
  A generator at all**
  - **A (recommended).
    Yes, in the page.**
    It helps exactly the people without a manager, and costs little.
    Cost: small (below).
  - **B.
    No; rely on browsers and managers.**
    No cost, but it leaves the tablet and the app without help.
- **D15.
  Words or characters**
  - **A (recommended).
    Words only:**
    a passphrase can be read off one screen and typed on another,
    such as a wall tablet or a phone.
    A user who has a manager uses the manager's own generator.
  - **B.
    Characters only:**
    20 characters from a 56-character alphabet, which leaves out `0 O 1 l I |`,
    about 116 bits.
    It is hard to type by hand.
  - **C.
    Both, with a switch.**
    Cost: a choice in five places and about 3 more keys × 6 languages.
- **D16.
  Which list, and in which language**
  - **A (recommended).
    EFF's long list in every language.**
    7,776 words, 3 to 9 letters, plain ASCII (four have a hyphen),
    about 12.9 bits a word.
    It is EFF's own material, under CC BY ("unless otherwise noted",
    eff.org/copyright), so it needs one attribution line in `third_party/`.
    Size of the words alone: 62,144 bytes raw, 24,491 bytes gzipped (measured).
  - **B.
    One list per language, English where none is clean.**
    - **German:**
      `dys2p/wordlists-de`, 7,776 words, no umlauts,
      CC0 or Unlicense. 71,429 bytes raw, 28,490 gzipped.
      Words are 5 to 19 letters measured, although the README says 3 to 12.
    - **French:**
      Tango's list for Tails, in `ulif/diceware`. 7,776 words,
      CC0 (that repository's COPYRIGHT file), 21,067 bytes gzipped.
      One word has a capital ("Internet").
    - **Italian:**
      in `ulif/diceware`, 8,192 words (13 bits), CC BY 4.0,
      20,785 bytes gzipped.
    - **Dutch:**
      mko.re's list from OpenTaal has 7,776 words but no licence stated.
    - **Polish:**
      `MaciekTalaska/diceware-pl` states no licence and is based on another
      author's list.
    - Cost: four lists, four attributions, and long German words.
      Six such words can pass 64 characters,
      so a draw over the limit must be redrawn.
      The language would also change with the UI setting.
- **D17.
  How many words, and the separator**
  - **A (recommended).
    Five words, lower case, joined by `-`:**
    64.6 bits.
    With EFF's list the result is 19 to 49 characters,
    so it always meets the 8-to-64 rule and NIST's 15.
    OPAQUE runs argon2id and the lockout stops online guessing,
    so this is ample.
  - **B.
    Four words:**
    51.7 bits, 15 to 39 characters.
    Enough online; weaker if a store and its `opaque.key` are stolen together.
  - **C.
    Six words:**
    77.5 bits; long to type on a phone.
- **D18.
  Where the list lives**
  - **A (recommended).
    A static file under `ui/`,
    embedded with `go:embed` and fetched when Suggest is first pressed.**
    This is the same path as the blocklist of D10, so no page load pays for it.
    Keep only the words, without the dice numbers: 62 KB.
  - **B.
    Inside `parts.js`.**
    Cost: 62 KB on every page load, uncompressed.
- **D19.
  The control and the eye switch**
  - **A (recommended).**
    A **Suggest** button sits under the first new-password field in all five
    forms, through `NewPassword` and a single-field variant of `PasswordField`.
    It fills both fields.
    It shows the words once in a read-only line with **Copy**
    (`navigator.clipboard.writeText`) and **Another**,
    and a status line says "A password was put in both fields".
    The words are not read out on every change.
    Every control is labelled and is a `<button>`, and the line wraps at 390 px.
    It shows the words **whatever the eye switch says**:
    the switch is about the eye on what a user types,
    and a password the user never saw cannot be kept.
    The line goes away when the form is saved, reset or closed.
    Cost: small.
  - **B.
    No generator while the switch is off.**
    Cost: owners who turned the eye off lose the feature.
  - **C.
    Copy only while the switch is off.**
    Cost: a password can be saved that nobody saw or kept,
    which means a lockout.
- **Unbiased picking:**
  use `crypto.getRandomValues` with rejection.
  For words, draw a `Uint32` and redraw at or above `floor(2^32 / 7776) × 7776`,
  then take the remainder.
  For characters, draw bytes and redraw at or above 224 (4 × 56).
  The generated password then goes through the D6 to D11 checks like a typed
  one, with no path of its own.

### What it would take

| Part | Files | Texts | Tests | Size |
|---|---|---|---|---|
| D14-A to D19-A | `shared/password.tsx` (`Suggest`, the picker), the four form files, `ui/words-en.txt` and `third_party/eff-wordlist/` (licence), `docs/ui.md` | about 5 keys × 6 (Suggest, Copy, Another, the status line, Copied) | `make ui-test`: both fields filled, the result passes the rule, Another changes it, the line still shows with the eye off, Copy gives a status; a Go test that the file has 7,776 unique words | small |
| D16-B extra | three more lists and attributions, a choice by language, a redraw over 64 | none | one case per list | small to medium |

## Part 2: a second factor

### What is so today

- **One place makes every session:**
  `openSessionFrom` in `internal/server/login_api.go`.
  It is called by the version-1 `POST /api/v1/login`,
  the OPAQUE finish (`opaque_api.go` line 289),
  OIDC's finish (`oidc.go` line 1030) and `POST /api/v1/login/key` (with a key
  id).
  A second step fits there: for a password sign-in by a user with a factor,
  it answers a pending step instead of a session.
- **A pattern exists:**
  `issueProof` already issues a short-lived,
  single-use proof after a password check (for the version-1 upgrade and for
  one's own password change).
- **Sessions last 30 days from their last use** (`SessionLifetime`, `auth.go`).
  NIST AAL2 says reauthentication SHOULD come within 24 hours.
  A second factor at sign-in therefore guards a session for a month.
- **The app** keeps an API key made with the page's session (`login.keep()`,
  `POST /api/v1/users/{name}/tokens`).
  Its fingerprint login then uses `login/key` and never meets a second step.
- **Lockout:**
  `internal/store/lockout.go` counts failures per address and per name:
  10 in a minute lock for 5 minutes, and the counts survive a restart.
- **Sealed secrets:**
  `internal/store/secret.go` uses AES-256-GCM with a key derived from
  `opaque.key` and additional data per secret.
  A TOTP secret can be kept there,
  which means a copied `massimo.db` alone does not hold it.
- **No e-mail anywhere:**
  no SMTP in `internal/notify` (Web Push only, `internal/push`),
  in the settings, or anywhere in the Go code.
  Users have no e-mail field.
  The only e-mails stored are OIDC identities' (`user_identities.email`).
- **Clocks:**
  the page already measures the server's clock against its own from the `Date`
  header (`serverSkew`, `header.js`).
  The server does not know whether its own clock is synchronised.
  On Linux the kernel says so through `adjtimex` (`STA_UNSYNC`),
  which `golang.org/x/sys/unix` (already a dependency) exposes.

### What the guidance says

| Source | Says |
|---|---|
| [RFC 6238](https://www.rfc-editor.org/rfc/rfc6238) (TOTP) | 30-second step; RECOMMEND at most one step back for network delay; the drift may be recorded per token; "MUST NOT accept the second attempt of the OTP"; HMAC-SHA-1 by default, SHA-256/512 allowed; key the length of the HMAC output |
| [RFC 4226](https://www.rfc-editor.org/rfc/rfc4226) (HOTP) | secret at least 128 bits, 160 recommended; at least 6 digits; throttle failures across sessions |
| [NIST SP 800-63B-4](https://pages.nist.gov/800-63-4/sp800-63b.html) | e-mail "SHALL NOT be used for out-of-band authentication" (§3.1.3.1); an OTP is accepted once; OTP "is not phishing-resistant"; at most 100 consecutive failures per authenticator (§3.2.2); AAL2 SHALL offer a phishing-resistant option (§2.2); reauthentication SHOULD come within 24 h at AAL2 |
| [OWASP MFA Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Multifactor_Authentication_Cheat_Sheet.html) | TOTP widely used, phishable; e-mail "relies entirely on the security of the email account"; single-use recovery codes at setup; reauthenticate with a factor before changing factors; notify on change and on a failed second step; require it for admins and for password changes |
| [WebAuthn Level 3](https://www.w3.org/TR/webauthn-3/), Rec. 25 Aug 2026 | only in secure contexts (TLS without errors); the RP ID is a valid domain; `getPublicKey()` gives DER SubjectPublicKeyInfo for Go's `x509.ParsePKIXPublicKey`; authenticator data is a fixed byte layout |

### Open decisions

- **D20.
  Which second factor**
  - **A (recommended).
    TOTP**, which every authenticator app reads.
    It works by IP, over plain HTTP on the LAN,
    with a self-signed certificate and in the WebView.
    Go's standard library gives `crypto/hmac`, `crypto/sha1`,
    `encoding/base32` and `crypto/rand`,
    so no dependency is needed for the codes.
    Cost: medium.
  - **B.
    Passkeys (WebAuthn).**
    Phishing-resistant, and the only option that meets NIST §2.2.
    With `getPublicKey()` the server needs no CBOR for "none" attestation,
    only `crypto/ecdsa` and `ed25519`.
    Cost: large.
    It needs HTTPS that the browser trusts and a domain name: no IP address,
    and a self-signed certificate does not count.
    A passkey belongs to one name,
    so the LAN name and the remote name need one each.
    Whether the app's WebView passes WebAuthn through was not checked.
  - **C.
    Codes by e-mail**
    (D27).
    **D.
    None:** point users to an OIDC provider's MFA, which works today.
- **D21.
  The QR code at enrolment**
  - **A (recommended).
    Vendored `rsc.io/qr`**
    (BSD-3-Clause, Russ Cox, last push 2022;
    the pure-Go `coding` and `gf256` packages, about 27 KB of source,
    without the cgo `libqrencode`) under `third_party/`.
    The server draws its modules as an SVG of `<rect>`s at enrolment.
    The page rule holds: no library in the page.
    Cost: small.
  - **B.
    Vendor Nayuki's `qrcodegen.ts`**
    (MIT, 989 lines, 41 KB source, 12 KB gzipped) into the page.
    Cost: a second page library, against "Preact is the only UI library";
    it needs the owner's exception.
  - **C.
    No QR:**
    the secret as text in groups of four,
    plus an `otpauth://totp/Massimo:<name>?secret=…&issuer=Massimo` link,
    which a phone may open in its authenticator.
    Cost: none, but typing 32 characters on a desktop.
    Show this beside A in any case.
  - **D.
    Our own QR encoder** (byte mode, level M, versions up to about 10).
    Cost: medium, and Reed-Solomon code to test.
- **D22.
  The second step in the sign-in**
  - **A (recommended).**
    `openSessionFrom` answers
    `{"second": "totp", "pending": "…", "expires_at": …}` for a password sign-in
    (keyID 0, no provider) by a user with TOTP.
    The pending token lasts 5 minutes, is used once and is held to the address,
    like `issueProof`.
    `POST /api/v1/login/second` with `{pending, code}` or `{pending, recovery}`
    then answers the session.
    The sign-in card (`src/signin/Card.tsx`) shows a code field
    (`autocomplete="one-time-code"`,
    `inputmode="numeric"`) with "Use a recovery code" and Back.
    `purpose: "proof"` asks for the code too, so a password change needs both.
  - **B.
    The code in the same request as the password.**
    Cost: it gives away that the password was right before the code is checked,
    and OPAQUE's finish does not fit it.
- **D23.
  What it covers**
  - **A (recommended).**
    - **Password sign-ins**, OPAQUE and version 1: covered.
      Both go through `openSessionFrom`.
    - **OIDC:**
      exempt, which is right; the provider's own MFA does that job.
    - **API keys and an integration's key:**
      exempt, which is right; a key is a machine secret.
    - **Making a key** (`POST …/tokens`, the app's `login.keep()`):
      only from a session that passed the second factor.
      Otherwise a stolen password plus a session would give a key that never
      expires: the one real hole.
    - **The app's fingerprint login:**
      a key made that way; right.
    - **The command line and `MASSIMO_API_TOKEN`:**
      exempt, which is right; whoever holds the box holds everything.
    - **Enrolling:**
      offers to end the user's other sessions and keys.
  - **B.
    A second step after OIDC too.**
    Cost: two factors asked twice.
- **D24.
  The clock**
  - **A (recommended).**
    Accept one step either side (±30 s).
    Keep the last step used per user, so that a code is taken once.
    Enrolment requires a first correct code,
    which shows that the two clocks agree.
    The page shows a line in the code step when `serverSkew` is over 30 s ("This
    server's clock is 3 min off; codes may fail").
    On Linux, `/api/v1/system` reports whether the clock is synchronised
    (`adjtimex`).
    Cost: small.
  - **B. ±2 steps (about 89 s, RFC 6238's example).**
    Cost: more guesses per code.
  - **C.
    Remember a drift per user**
    (RFC 6238 §6).
    Cost: medium; it only helps a clock that drifts slowly.
- **D25.
  Failures, recovery and reset**
  - **A (recommended).**
    - A wrong code counts against the name and the address,
      as a failed login does.
    - Ten single-use recovery codes, each about 50 bits,
      kept as SHA-256 and shown once at enrolment.
    - Reset by a user with `users` (or the owner) in the user's panel,
      which ends the user's sessions.
    - On the box: `massimo user totp <name> off`.
    - The user is told in the log and by push where subscribed.
    - Turning TOTP off oneself requires a code.
  - **B.
    No recovery codes, owner reset only.**
    Cost: an owner who loses the phone has only the box.
- **D26.
  Policy**
  - **A (recommended).
    Each user's choice, in My profile.**
    An open server has no users, so nothing applies there.
    The first owner enrols after creation, not in the first form.
    Cost: within D20-A.
  - **B.
    The owner may require it per role** (a role flag;
    a user without one enrols at the next sign-in).
    Cost: medium, as a later block.
- **D27.
  A code by e-mail**
  - **A (recommended).
    No.**
    It would need a mail server's settings (host, port, TLS, account,
    sealed password), an address per user with its verification,
    and sending and bounce handling.
    NIST forbids e-mail as out-of-band, and OWASP doubts it.
    At home the mailbox is often on the same phone.
    Cost: none.
  - **B.
    Later, only to send a reset link to the owner.**
    Cost: medium to large, all of the above.
- **D28.
  The plan**
  - **A (recommended).**
    The generator goes into the follow-ups' plan with D6 to D12.
    TOTP is a plan of its own ("SecondFactor"), and passkeys come later,
    separately.

### What it would take (D20-A to D26-A)

| Block | What | Size |
|---|---|---|
| store | a migration: `user_factors` (sealed secret, last step, enabled time); `recovery_codes` (hash, used); pending second steps in memory | small |
| server | TOTP check; `openSessionFrom` split; `login/second`; enrol, confirm, off and reset routes; key making held to a factor session; audit, log and push | medium |
| QR | `third_party/rsc-qr` plus an SVG writer | small |
| page | the card's code step; My profile's enrolment (QR, text, link, first code, recovery codes, off); the Users reset; about 25 keys × 6 | medium |
| app | the code step needs nothing (it is the page); `login.keep()` refused without a factor session, with its message | small |
| CLI | `massimo user totp <name> off`, and `list` showing who has one | small |
| docs, tests | `api-v1.md`, OpenAPI, `store.md`, `ui.md`; RFC 6238 Appendix B vectors; replay, window and lockout cases; UI test of the sign-in with a code | medium |

Overall: medium to large, about seven blocks.

## Sources

- EFF wordlists: https://www.eff.org/dice.
  Opened; the files were downloaded and measured.
  Licence: https://www.eff.org/copyright.
  Opened; the text says CC BY 4.0 while its badge links CC BY 3.0 US.
- German: https://codeberg.org/fossarchive/wordlists-de (a fork of
  github.com/dys2p/wordlists-de).
  README and LICENSE (CC0) opened; the lists were measured.
- French and Italian: https://github.com/ulif/diceware, its README,
  `COPYRIGHT` and wordlist files.
  Opened and measured.
- Dutch: https://mko.re/blog/diceware-nl/.
  Opened; no licence found.
  Polish: https://github.com/MaciekTalaska/diceware-pl.
  Listed through the API; no licence file.
- Google, the `autocomplete` attribute and password generation:
  https://developer.chrome.com/docs/identity/automated-password-change.
  Opened.
  Android autofill:
  https://developer.android.com/identity/autofill/autofill-optimize.
  Opened; WebView is not covered there.
  Apple's Password AutoFill page could not be read.
- RFC 6238, RFC 4226, NIST SP 800-63B-4 (§2.2, §3.1.3, §3.1.4-5, §3.2.2 read;
  the rest not), OWASP MFA Cheat Sheet, WebAuthn Level 3: opened,
  links in the table.
- `rsc.io/qr`:
  https://github.com/rsc/qr.
  Opened (licence and files).
  Nayuki: https://github.com/nayuki/QR-Code-generator.
  Opened; `qrcodegen.ts` was measured.
