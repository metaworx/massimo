# NOTE PasswordRules v1.0: no list of common passwords

What the first step of Block 3 found on 2026-11-25, before the block was built,
and what the block becomes.

## The list's origin does not hold

D10-A took SecLists' `10k-most-common.txt` on a condition,
and the plan's Block 3 opens with it as a stop:
the list enters the repository only where the file is SecLists' own under its
MIT licence, or its source plainly allows its copying.
Neither is so.
`wip/2026-11-25_03-26_ANALYSIS_PasswordRules_origins_of_the_lists.md`
has each source with its address and its words.

- **Not SecLists' own.**
  The file is a copy, byte for byte, of Mark Burnett's list of 2011,
  added to SecLists in 2014 with its source named.
  SecLists' MIT licence came in 2018 and says nothing of the files it collects;
  its contributors' guide asks only that the original author be credited.
- **Its source does not plainly allow its copying.**
  No statement was made of the list itself.
  The page that released it said "All Rights Reserved" in 2011,
  the site CC BY-NC-ND 3.0 in 2013,
  and a later version of the article, without the download, CC BY-SA 3.0.
  The words "public domain" belong to another release of 2015.

Whether a list of the most frequent passwords is protected at all is not decided
here: the plan's condition asks for plain terms, and there are none.

## What Block 3 becomes

The stop holds: no list enters the repository.
The block's goal is then the second half of its sentence:
the user's and the server's names are refused.

- **Left out,** steps 2, 3 and 5:
  no `internal/password/common.txt`, no `third_party/seclists-common/`,
  no `ErrCommon`, no `/ui/common-passwords.txt`,
  and nothing that the page fetches before a submit.
  "A list not loaded", under "Decided in the writing", has nothing to wait for.
- **Stays,** step 4:
  `ErrContext`, the context words as D50-A settles them:
  the user's login name and display name, the server's name,
  `massimo` and `maxcube`, each of 4 or more letters,
  refused where the password contains it, whatever its case.
- **The line,** step 6:
  "contains your name or the server's name", in six languages;
  "a common password" is not added.
- **The tests:**
  `cases.json` grows by the context and the near cases,
  both languages answering alike;
  the command line and the API refuse the user's name;
  a UI test of the message.
  Nothing of a list's checksum, and no submit that waits.
- **The CHANGELOG bullet and the commit** say what is built:
  a new password that contains the user's or the server's name is refused.
- **Blocks 4 and 5** stand:
  the generator draws again where the context refuses,
  and the credits list one list, EFF's.

## For the owner: D55, a list from another source

D10-A's aim, to refuse the commonest passwords, is not met by this.
Two other sources were read; the analysis has their terms and counts.

- **D55-A (recommended).**
  Openwall's `password.lst`, the list of John the Ripper:
  its own header says "It is assumed to be in the public domain."
  Of its first 10,000 entries, 3,510 have 8 to 64 characters, about 34 KB.
  The terms stand in the file itself, in its compiler's words;
  they are an assumption, not a licence.
  A small block of its own, as the plan's steps 2, 3 and 5 describe.
- **D55-B.**
  The UK NCSC's list of the 100,000 most used passwords:
  3,864 of its first 10,000.
  Its site's terms give the Open Government Licence v3.0 for the site's content
  but except material of third parties, which this list,
  made from another's data, may be; the file is no longer on the site.
- **D55-C.**
  No list: the length and the names alone, as Block 3 is built.

Nothing of D55 is built without the owner's word.
