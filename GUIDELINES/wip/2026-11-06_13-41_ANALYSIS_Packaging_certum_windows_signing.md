# Analysis Packaging: Certum's open-source code signing as the Windows alternative

Written on 2026-11-06 for the `EVAL` of the three Certum products,
as an addendum to `2026-11-06_13-05_ANALYSIS_Packaging_packages_signing_channels.md`,
section 4.2.
Every page was read on 2026-11-06; a date in brackets is the page's own.
Nothing here holds a key, a token or an identity document.

## 1. The three products

| Product | Price | What it is | Year two |
|---|---|---|---|
| Open Source Code Signing, set (`open-source-code-signing.html`) | from 69 EUR, plus courier shipping (35 EUR to the UK in one report) | the certificate on a cryptoCertum 3.7 card the size of a SIM, with a USB reader; signing needs proCertum CardManager and the reader's driver | the card and reader are kept; a new certificate is the "code" product |
| Open Source Code Signing, code (`open-source-code-signing-code.html`) | from 25 EUR | the certificate alone, as an activation code "for customers who have their own cryptoCertum cryptographic card and reader" | the same |
| Open Source Code Signing in the Cloud (`open-source-code-signing-on-simplysign.html`) | from 49 EUR (58 USD on the US store) | the key in Certum's SimplySign cloud, a "virtual cryptographic card": the SimplySign mobile app (one device) gives one-time codes, SimplySign Desktop mounts the card; "a certificate usage limit of 5000 signatures per month" | 49 EUR again |

What the three share:

- **Validity one year.**
  Certum's product comparison lists the Open Source variant at "1 year" against
  "1-3 years" for Standard and EV;
  a buyer wrote in May 2026 "Open source certificates can't be renewed and I
  need to buy a new one every year";
  a certificate issued on 2025-10-30 ran to 2026-10-30.
  A new certificate means a new key pair made at activation,
  and Microsoft says SmartScreen reputation "does not carry over" to a renewed
  certificate with a new key (2026-04-17),
  so the publisher reputation restarts every year.
- **The publisher is the person.**
  "Issued certificate will contain 'Open Source Developer' phrase added to the
  Common name field" and "Organization field will be set to 'Open Source Developer'";
  a real one reads
  `CN=Open Source Developer, <first name> <last name>, O=Open Source Developer, L=…, C=…`,
  issued by `Certum Code Signing 2021 CA`.
  Every SmartScreen and UAC dialog shows the owner's name.
- **Individuals only, open source only.**
  "Issued only to individuals";
  Certum "may revoke it if it is used to sign commercially distributed software".
- **The documents.**
  Identity by "automatic identity verification (recommended)" (IDNow in one report:
  a photographed ID and a head movement), or at a registration point, a notary,
  or photos holding the ID; "a utility bill … issued in your name";
  "the website address of your ongoing Open Source project",
  which "must be publicly available and indicate clearly the subscriber's
  relationship with the project" (one buyer was asked to put his name on his
  GitHub profile).
- **Time.**
  Two days of processing in one account (2025-10),
  "a day or two" in another (2026-05), plus the courier for the card;
  others found the support slow and the activation tooling clumsy (a SignService
  installer that "is NOT the same as their SimplySign or SmartSign applications",
  a wrong driver link, `certutil -repairstore` after "hours of problems").
- **Stock.**
  All three pages said "Product is out of stock" when read today (one also "In stock"),
  the US store too; people bought in May 2026,
  so this is to be checked with Certum before counting on it.
- **The key is in hardware** (card: CC EAL6+; cloud: FIPS 140-2 level 3),
  as the CA/Browser Forum requires since 2023; RSA 3072 at least.

## 2. Signing from CI with each

**The card** stays where the reader is.
Both write-ups sign by hand on Windows with signtool
(`signtool sign /sha1 <thumbprint> /fd SHA256 /td SHA256 /tr http://time.certum.pl <file>`); the shop says Linux and macOS tools are supported, but no account of unattended use with this card was found, and the CSP asks for the PIN through a dialog.
Two ways to put it into the release:

- **By hand**:
  release.yml's `build` job uploads the unsigned `.exe` as an artifact;
  the owner downloads it on jaguar, signs it with the card,
  uploads it to the draft release with `gh release upload --clobber`
  and dispatches the sign job,
  which first verifies the Authenticode signature against the expected
  certificate (`osslsigncode verify`),
  then makes and signs the manifest as today.
  One manual step of a few minutes per release,
  on the Windows PC with the card in it; the pipeline otherwise unchanged.
- **A self-hosted runner on jaguar**:
  GitHub's hardening guide says self-hosted runners "should almost never be used
  for public repositories",
  because "any user can open pull requests against the repository and compromise
  the environment",
  and that such runners "can be persistently compromised by untrusted code in a workflow".
  Not for this project.

**The cloud** key is reachable from anywhere,
after a login with the e-mail and a six-digit one-time code from the mobile app;
there is no password, so the code is the whole second factor.
Two community tools make that unattended by taking the TOTP seed out of the
activation QR code and keeping it as a CI secret:

- **ssign**
  (MIT, Rust; github.com/Le-Syl21/ssign, created 2026-07-10,
  v0.1.10 released 2026-11-06, 18 stars):
  talks to the SimplySign cloud over HTTPS "by itself: no SimplySign Desktop,
  no GUI, no container, no PKCS#11 stack", on Linux, macOS and Windows;
  signs PE files with a Certum timestamp (`ssign -e you@example.com -T 123456 app.exe`,
  or with the seed); MSI "work in progress",
  the other types through its PKCS#11 module with osslsigncode;
  "an independent client, not affiliated with or endorsed by Certum";
  "The TOTP seed is a long-lived secret and needs protecting".
- **certum-cloud-code-sign**
  (Apache-2.0; github.com/jay0lee/certum-cloud-code-sign, created 2026-09-24,
  0 stars):
  installs SimplySign Desktop on a Windows runner and types the e-mail and the
  generated code into it; the README itself calls the login "fragile".

What that model costs: whoever holds the seed signs as the owner, unattended,
from anywhere, for a year;
GitHub's `signing` environment limits it to `v*` tags and can require the
owner's approval of the job,
which gives back the one click per release SignPath has,
but a leak of the secret is a leak of the signature,
and the way out is to re-enrol the mobile app.
Certum's terms were searched for a word on copying the one-time-code seed out of
the app and none was found; the app is "activated on only one device",
and the Polish regulations speak of a PIN "known only to the subscriber".
It is at least outside what Certum designed.

## 3. Against SignPath Foundation

| | SignPath Foundation | Certum cloud, ssign | Certum card, by hand |
|---|---|---|---|
| Money | 0 | 49 EUR a year | 69 EUR and shipping, then 25 EUR a year |
| Certainty | discretionary; "reputation" references wanted; weeks to months | money and identity; about a week | the same, plus the courier |
| Publisher shown | SignPath Foundation | Open Source Developer, the owner's name | the same |
| Reputation | the shared certificate's, since mid-2025 | restarts yearly with the new certificate | the same |
| Key in CI | none; SignPath's HSM, build origin verified | the TOTP seed, a long-lived secret, in the `signing` environment | none; the card on jaguar |
| Per release | one approval click | none, or one if the environment requires a reviewer | a manual signing step on the Windows PC |
| Tooling | SignPath's own GitHub action (v3) | a four-month-old third-party client at v0.1 | signtool |
| Later, the MSI | nested in one request | ssign's MSI support unfinished; osslsigncode through its PKCS#11 module | signtool |
| Terms | the project's README section, MFA | individuals only, no commercial software; seed extraction outside the design | the same |

## 4. Recommendation

SignPath first, Certum's cloud variant as the named fallback,
the card not at all:

- SignPath costs nothing but patience, keeps every key out of CI,
  names no person, and inherits a reputation;
  the unsigned route works meanwhile and the project is an alpha with few users.
- If SignPath declines, or has not answered two months after the application,
  Certum in the cloud with ssign in a job of its own under the `signing`
  environment with a required reviewer: 49 EUR a year,
  a signature within about a week,
  with the owner's name on the dialogs and the seed in GitHub's secrets as the
  known costs.
- The card is the cheapest over the years but ties every release to the Windows
  PC with the card in it, and the only way to automate it, a self-hosted runner,
  is what GitHub warns against for a public repository.
- Whichever: the Authenticode signature is verified in the sign job before the
  manifest is made,
  and the Windows CI job checks the newest release's signature once there is one.

## 5. Sources

https://shop.certum.eu/open-source-code-signing.html;
https://shop.certum.eu/open-source-code-signing-code.html;
https://shop.certum.eu/open-source-code-signing-on-simplysign.html;
https://certum.store/open-source-code-signing-on-simplysign.html;
https://support.certum.eu/en/code-signing-required-documents/;
https://www.certum.eu/en/cert_offer_code_signing/ (validity one year);
https://support.certum.eu/en/?p=7791 (SimplySign activation and login);
https://piers.rocks/2025/10/30/certum-open-source-code-sign.html (2025-10-30);
https://www.msz.it/a-cheap-code-signing-certificate-for-open-source-projects-by-certum-asseco-an-honest-review-walkthrough/ (2020-06-20); https://en.delphipraxis.net/topic/15454-cheapest-codesign-149-anyone-used-this/ (2026-05); https://engineered.at/articles/how-to-automate-signing-your-windows-app-with-certum-s-simplysign-app (2025-07-11); https://le-syl21.github.io/ssign/ and https://github.com/Le-Syl21/ssign; https://github.com/jay0lee/certum-cloud-code-sign; https://github.com/altintasutku/hece/issues/49 (2026-11-02, another project's SignPath-first, Certum-fallback decision); https://learn.microsoft.com/en-us/answers/questions/5857071/how-can-a-small-software-publisher-build-smartscre (2026-04-17); https://docs.github.com/en/actions/security-for-github-actions/security-guides/security-hardening-for-github-actions.
