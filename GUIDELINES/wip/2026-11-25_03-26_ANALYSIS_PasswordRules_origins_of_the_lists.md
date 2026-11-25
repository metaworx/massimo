# Origins of the two word lists and the icon

Written 2026-11-25 by the session "UI refactor",
for the first step of AP PasswordRules' Block 3 ("The list's origin, a stop")
and for the word list and the icon of its Block 4.
Researched by an Opus sub-agent, which read each source it names;
the checksums, the counts and four of its quotes were checked by the session
against the copies it saved.

All sources read on 2026-11-25 (02:04 UTC), unauthenticated,
with a generic user agent.
Quotes are exact; `[…]` marks a cut.

## For the plan

- **The list of common passwords does not hold** (section 1):
  the plan's stop applies, and no list enters the repository.
  Block 3 refuses the context words alone;
  the NOTE "no list of common passwords" says what the block becomes,
  and another source is the owner's to decide.
- **EFF's long word list holds** (section 2): CC BY 4.0, with an attribution.
- **Tabler's `dice-5` holds** (section 3): MIT, at the version vendored.

## 1. The list of common passwords (SecLists `10k-most-common.txt`)

### Conclusion: DOES NOT HOLD

- **Not SecLists' own.**
  The file came into SecLists in 2014 as a copy of Mark Burnett's 2011 "10,000
  Most Common Passwords" zip, with the source named in the commit;
  it is byte-identical to the file in that zip (SHA-256 below).
  SecLists' MIT licence (added 2018,
  "Copyright (c) 2018 Daniel Miessler") is the repository's licence;
  nothing in SecLists claims or grants rights in Burnett's list,
  and its CONTRIBUTING asks contributors to credit the "original
  author/curator".
  So the file is in SecLists,
  but it is not "SecLists' own under its MIT licence".
- **Its source does not plainly allow copying.**
  Burnett's own statements about the pages carrying the list changed over time
  and conflict: at release (2011) the page footer said "All Rights Reserved";
  by 2013 the site footer said CC BY-NC-ND 3.0 (no derivatives, non-commercial);
  the rewritten article on Medium (captured 2016) says CC BY-SA 3.0,
  but in that version the download links had been removed,
  so it is not plain that the statement covers the list rather than the article.
  None of the statements is made about the list itself.
  The "public domain" statement is about a different release (the ten million
  username/password pairs, 2015), not the 10,000 list.
- Whether a frequency list of passwords is protected at all is not decided here.

### SecLists' terms

| What | URL | Quote |
|---|---|---|
| LICENSE (at the pinned commit) | https://raw.githubusercontent.com/danielmiessler/SecLists/fcde9140df969b1e4d7bb056122646dace4ed4cc/LICENSE | "MIT License" / "Copyright (c) 2018 Daniel Miessler" |
| LICENSE added | commit `7c705045`, 2018-06-13, "Create LICENSE" | (after the file's import in 2014) |
| README | https://raw.githubusercontent.com/danielmiessler/SecLists/fcde9140df969b1e4d7bb056122646dace4ed4cc/README.md | "This project is licensed under the [MIT license](LICENSE)." ; "### Attribution / See [CONTRIBUTORS.md](CONTRIBUTORS.md)" |
| CONTRIBUTING | https://raw.githubusercontent.com/danielmiessler/SecLists/fcde9140df969b1e4d7bb056122646dace4ed4cc/CONTRIBUTING.md | "Significant effort SHOULD be made to give attribution for these lists whenever possible, and if you are a list owner or know who the original author/curator is, please let us know so we can give proper credit." |
| CONTRIBUTORS | https://raw.githubusercontent.com/danielmiessler/SecLists/fcde9140df969b1e4d7bb056122646dace4ed4cc/CONTRIBUTORS.md | "Mark Burnett for the **10 million passwords** list" (the only Burnett credit; nothing for the 10k list) |
| Folder README | https://raw.githubusercontent.com/danielmiessler/SecLists/fcde9140df969b1e4d7bb056122646dace4ed4cc/Passwords/Common-Credentials/README.md | has no entry for `10k-most-common.txt` (entries for the xato 10-million files and others) |

Neither README nor CONTRIBUTING says anything about the terms of the files it collects.

### The file's history in SecLists

Traced with a blobless clone (`git log --no-renames` over every path containing
`10k*most*common`):

| Commit | Date | What |
|---|---|---|
| `ce9c9c395648dd6601395b0ff6dd17aa68172fbe` | 2014-10-06 | added as `Passwords/10k_most_common.txt`, message: "Add 10k most common" / "Source: http://xato.net/files/10k%20most%20common.zip" (10,000 lines, CRLF) |
| PR #8, https://github.com/danielmiessler/SecLists/pull/8 | merged 2015-01-12 | "Add 10k most common", body only: "Source: http://xato.net/files/10k%20most%20common.zip"; no comments |
| `7c47ae1d` | 2017-07-11 | moved to `Passwords/Common/10k_most_common.txt` |
| `25d4ac44` | 2017-08-23 | moved to `Passwords/CommonCreds/10k-most-common.txt` |
| `4f84b3c2` | 2018-03-05 | moved to `Passwords/Common-Credentials/10k-most-common.txt` |
| `778b1611` / `5e1dc9cc` | 2018-12-31 / 2019-01-07 | deleted by a bad merge, restored ("Fix #259 - Recover from bad merge") |
| `a3416ba7` | 2020-05-27 | "standardisze line endings" (CRLF to LF) |
| `913b3273`, PR #1344 https://github.com/danielmiessler/SecLists/pull/1344 | 2026-09-08 (merged 2026-09-09) | one line added, `66668888`, after `66666666`; PR: "commonly used WiFi password in China (see issue #1343)" |

Content check: the 2014 blob (SHA-256
`af55295852bcb0bc63e76cd22e342d9f7778b1802e76f0c502c73a35f652fd7a`) is identical
to `10k most common.txt` inside the archived xato.net zip (same SHA-256,
zip entry dated 2011-06-20);
the version before PR #1344 equals it with LF line ends;
today's adds only `66668888`.

### The source: Mark Burnett, xato.net

| Statement | URL (archived) | Quote |
|---|---|---|
| The release, 2011-06-20, "More Top Worst Passwords" | https://web.archive.org/web/20110624161107/http://xato.net:80/passwords/more-top-worst-passwords | "I have decided to release the following list of the top 10,000 most common passwords." ; "this list has been converted to all lowercase letters" ; "all passwords on this list are from publicly available sources" |
| Footer of that page at release | same capture | "Copyright (c)2008 Mark Burnett, All Rights Reserved." |
| Same post, 2013 capture, links to the zips | https://web.archive.org/web/20130705202159/http://xato.net:80/passwords/more-top-worst-passwords/ | links "10,000 Most Common Passwords List -> http://xato.net/files/10k%20most%20common.zip" |
| Footer in 2013 (site-wide) | same capture | "Copyright (C)2000-2012 Mark Burnett. Xato.net is written by Mark Burnett and is licensed under a Creative Commons Attribution-NonCommercial-NoDerivs 3.0 Unported License." |
| The zip itself | https://web.archive.org/web/20121028152027/http://xato.net/files/10k%20most%20common.zip | one file, `10k most common.txt`; no licence, readme or comment in the zip |
| The rewritten article on Medium (xato.net), dated "Jun 20, 2011", captured 2016-12-13 | https://web.archive.org/web/20161213014736/https://xato.net/10-000-top-passwords-6d6380716fe0 (now https://medium.com/xato-security/10-000-top-passwords-6d6380716fe0, which answers 403 to a plain client) | "[Links removed as they are quite old. See this article for a more updated list]" ; at its end: "This work by Mark Burnett is licensed under a Creative Commons Attribution-ShareAlike 3.0 Unported License." (link to creativecommons.org/licenses/by-sa/3.0) |
| A different release: ten million usernames and passwords, 2015 | https://web.archive.org/web/20150210024537/https://xato.net/passwords/ten-million-passwords/ | "a data set of ten million usernames and passwords that I am releasing to the public domain." ; "THIS DATA IS ONLY MADE AVAILABLE FOR ACADEMIC AND RESEARCH PURPOSES." ; page footer: "This work by Mark Burnett is licensed under a Creative Commons Attribution-ShareAlike 3.0 Unported License." |
| Its FAQ | https://web.archive.org/web/20150210145428/https://xato.net/passwords/ten-million-passwords-faq/ | "But if you just want the raw data for any purpose than protecting users the answer is no." (about his raw data, not the 10k list) |
| His GitHub (`m8urnett`, named on the Medium article) | https://api.github.com/users/m8urnett/repos | no repository with the 10k list or a statement of its terms |

Distinction: the "public domain" words belong to the 2015 ten-million release only.
For the 10,000 list there is no statement aimed at the list;
only page or site footers (All Rights Reserved, 2011; CC BY-NC-ND 3.0,
2012-13) and the article-level CC BY-SA 3.0 line on the later Medium version,
which no longer links the list.
Secondary (not relied on): a web search summary called the list "CC BY-SA 3.0",
from the Medium article.

### Pinned

| Fact | Value |
|---|---|
| SecLists default branch | `master` |
| Newest commit at reading | `fcde9140df969b1e4d7bb056122646dace4ed4cc` (2026-11-24T11:10:54Z, "[Github Action] Automated update to trickest wordlists") |
| Path | `Passwords/Common-Credentials/10k-most-common.txt` (earlier `Passwords/10k_most_common.txt`, `Passwords/Common/10k_most_common.txt`, `Passwords/CommonCreds/10k-most-common.txt`) |
| Raw URL | https://raw.githubusercontent.com/danielmiessler/SecLists/fcde9140df969b1e4d7bb056122646dace4ed4cc/Passwords/Common-Credentials/10k-most-common.txt |
| Size | 73,026 bytes, ASCII, LF, final newline |
| Lines | 10,001 (10,000 of Burnett's plus `66668888`) |
| SHA-256 | `68782d6a4a19a4768d5f15dd66bd534e7a33055cc755411e33f16d18c50fdcce` |
| Unique after lower-casing, 8 to 64 characters | **2,087** (2,086 in Burnett's original; `66668888` is the extra one). All entries are already lower case and unique; longest 18 characters |

### Alternatives

| | NCSC "100k most used passwords" (2019) | Openwall `password.lst` (John the Ripper) |
|---|---|---|
| URL | was https://www.ncsc.gov.uk/static-assets/documents/PwnedPasswordTop100k.txt (later `PwnedPasswordsTop100k.txt`, then `.json`); all three answer 404 today; copy taken from https://web.archive.org/web/20190422080836/https://www.ncsc.gov.uk/static-assets/documents/PwnedPasswordTop100k.txt | https://raw.githubusercontent.com/openwall/john/ea6b98c095559785903a10d51c47d90aafcc52c8/run/password.lst (head of `bleeding-jumbo`, 2026-11-16) |
| Announcement | https://www.ncsc.gov.uk/news/most-hacked-passwords-revealed-as-uk-cyber-survey-exposes-gaps-in-online-security (21 April 2019): "The list was created after breached usernames and passwords were collected and published on Have I Been Pwned by international web security expert Troy Hunt." | the file's own header |
| Terms | NCSC terms, https://www.ncsc.gov.uk/section/about-this-website/terms-and-conditions : "Content on the Websites is, unless stated otherwise, subject to Crown copyright. Unless we indicate […] that content on the Websites includes material under licence from third parties, you may use or reuse the content published on the Websites without prior permission but must adhere to and accept the terms of the Open Government Licence (OGL) v3.0." and "Where materials are stated to include material under licence from third parties, those materials are not licenced for re-use." No statement on the list itself found. Upstream, HIBP (https://haveibeenpwned.com/API/v3): "there is no licencing or attribution requirements on the Pwned Passwords API" (about the API, not this plaintext list) | "This list has been compiled by Solar Designer of Openwall Project in 1996 through 2022.  It is assumed to be in the public domain." |
| Content note | HIBP-derived, mixed case, 241 lines with non-ASCII bytes | first 3,546 entries: mid-1990s Unix and 2006-2010 public "top N" lists; later entries (2022) from "Pwned Passwords v8 (HIBP) 100+ hits overlap with RockYou" |
| Size, lines | 942,782 bytes, 100,000 lines, CRLF; SHA-256 `e4f02bd0a022235af2f150de7e0cfd47b283c73195243349119f81769ff6f62e` | 15,327,454 bytes, 1,795,708 lines (18 comment lines, 1,795,690 entries); SHA-256 `bd84dbc84ce113fcf0b9489a8d6f665e52db43b5332937ad7a216405cb8ab7cf` |
| First 10,000 lines: unique lower-cased, 8-64 chars | **3,864** | **3,510** (of the first 10,000 entries after the comments; the first 3,546 alone give 613) |
| Plainly allows copying? | probably OGL v3.0 (attribution), but unconfirmed: the file is no longer on the NCSC site, and the terms exclude third-party material, which a HIBP-derived list may be | yes in words ("assumed to be in the public domain"), from the author's own header; not a formal licence grant |

No third alternative was sought further.

## 2. EFF's long word list

| Fact | Value |
|---|---|
| File | https://www.eff.org/files/2016/07/18/eff_large_wordlist.txt (Last-Modified: Mon, 18 Jul 2016 22:41:51 GMT) |
| Article | https://www.eff.org/deeplinks/2016/07/new-wordlists-random-passphrases, "By Joseph Bonneau, July 19, 2016": "The result is our own list of 7,776 words [.txt]" |
| Dice page | https://www.eff.org/dice links the same file |
| Size, lines | 108,800 bytes, 7,776 lines, ASCII, LF |
| SHA-256 | `addd35536511597a02fa0a9ff1e5284677b8883b83e986e43f15a3db996b903e` |
| Check | every line `<5 dice digits 1-6><TAB><word>`; dice numbers 11111-66666 complete and in order; **7,776 unique words**, sorted |
| Characters | `a-z` and `-` (four words: `drop-down`, `felt-tip`, `t-shirt`, `yo-yo`) |
| Lengths | shortest 3 (e.g. `aim`, `art`), longest 9 (e.g. `abdominal`, `acclimate`) |

Licence, as eff.org states it today:

- Neither the article nor the dice page carries a licence statement of its own;
  their footers link "Copyright (CC BY)" to https://www.eff.org/copyright .
- That page's text: "Any and all original material on the EFF website may be
  freely distributed at will under the Creative Commons Attribution 4.0
  International License (CC-BY), unless otherwise noted.
  All material that is not original to EFF may require permission from the
  copyright holder to redistribute."
  (linked to https://creativecommons.org/licenses/by/4.0/).
- On the same page, beside that text,
  the badge image is a link `rel="license"` to
  http://creativecommons.org/licenses/by/3.0/us/ (image alt "Creative Commons
  License").
- So both are stated: the words say CC BY 4.0,
  the badge's link says CC BY 3.0 US.
  The word list is EFF's original material with no other note,
  so the policy covers it; the written statement is 4.0,
  the 3.0 US is only the badge's link target.
- Attribution EFF asks for, in its own words:
  "If you redistribute something you got from the EFF site,
  it is appreciated if you make it known where the file originated,
  so people can get more info or updated versions."
  (CC BY itself requires attribution.)

## 3. Tabler icon `dice-5`

| Fact | Value |
|---|---|
| Tag | `v3.24.0`, annotated tag object `3a41657c105e6a8f6b4a7c9d84a493942693e38f`, commit `01eb08a9bc8075f5bd6828ac30b0a3a038a9ef46`, tagged 2024-12-02 |
| Licence | https://raw.githubusercontent.com/tabler/tabler-icons/v3.24.0/LICENSE : "MIT License" / "Copyright (c) 2020-2024 Paweł Kuna" (standard MIT text; SHA-256 `d5a3f948ae5cb95bfc4d2f767ef2bf6a778815b0ddb2d1c48037744cf636518e`) |
| Icon | https://raw.githubusercontent.com/tabler/tabler-icons/v3.24.0/icons/outline/dice-5.svg : **exists** at that tag under that name (API: path `icons/outline/dice-5.svg`, 714 bytes, blob `ed7fd463ae36a5cfc838ab70001255d5427aefb3`, matching the download) |
| SHA-256 | `f17aa29f77a1717da7de1e96fc33d0794480f0c27f58667905a5fb777bfb9eab` |

Full text:

```svg
<!--
tags: [game, boardgame, roll, throw, cube, numbers, gambling]
version: "1.60"
unicode: "f08f"
category: Games
-->
<svg
  xmlns="http://www.w3.org/2000/svg"
  width="24"
  height="24"
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="2"
  stroke-linecap="round"
  stroke-linejoin="round"
>
  <path d="M3 3m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v14a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
  <circle cx="8.5" cy="8.5" r=".5" fill="currentColor" />
  <circle cx="15.5" cy="8.5" r=".5" fill="currentColor" />
  <circle cx="15.5" cy="15.5" r=".5" fill="currentColor" />
  <circle cx="8.5" cy="15.5" r=".5" fill="currentColor" />
  <circle cx="12" cy="12" r=".5" fill="currentColor" />
</svg>
```

## Not found or not confirmed

- No statement by Burnett aimed at the 10,000 list itself;
  the Medium article's current live page could not be read (403),
  only its 2016 archive.
- The NCSC list's terms: no statement on the file; it is gone from ncsc.gov.uk;
  the 2019 capture of the NCSC terms page is a stub,
  so only today's terms were read.
- Openwall's "assumed to be in the public domain" is the author's assumption,
  not a licence.
- One secondary source used only as a pointer:
  a web search summary (Medium article, NCSC file names).
- The counts: lower-cased, repeats removed, 8 to 64 characters kept,
  CRLF read as LF.
