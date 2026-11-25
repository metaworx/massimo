# EFF's long word list

The web UI suggests a new password of five words drawn from EFF's long word
list (AP PasswordRules, D16-A, D17-A): the Electronic Frontier Foundation's
7,776 English words, one for each throw of five dice, published on
2016-07-19 at <https://www.eff.org/files/2016/07/18/eff_large_wordlist.txt>
(its `Last-Modified` 2016-07-18) with Joseph Bonneau's article
<https://www.eff.org/deeplinks/2016/07/new-wordlists-random-passphrases>.
eff.org puts its own material under CC BY 4.0 in its words and links a badge
to CC BY 3.0 US; `NOTICE` quotes both, the attribution EFF asks for, what was
changed and both SHA-256 values.
`LICENSE` is Creative Commons' legal code of Attribution 4.0 International,
byte for byte as fetched on 2026-11-25 from
<https://creativecommons.org/licenses/by/4.0/legalcode.txt>
(18,657 bytes, SHA-256
`9ba9550ad48438d0836ddab3da480b3b69ffa0aac7b7878b5a0039e7ab429411`).

The copy is `internal/server/ui/words-en.txt`, served at `/ui/words-en.txt`
and fetched by the page at the first suggestion of words
(`ui/src/shared/suggest.ts`):
the words alone, one a line, in the list's order, the dice numbers and the
tab before each word dropped (`cut -f2`); nothing else changed.
`TestWordList` (`internal/server/ui_test.go`) holds the file to its SHA-256
and to 7,776 unique words of `a-z` and `-`.

| File | Bytes | SHA-256 |
|------|-------|---------|
| `eff_large_wordlist.txt`, the original | 108,800 | `addd35536511597a02fa0a9ff1e5284677b8883b83e986e43f15a3db996b903e` |
| `words-en.txt`, the copy | 62,144 | `6d557f0693958fb5e650b68b5bee585eb82cf4da32965505c789e924743bc522` |

Four of the words hold a `-`: `drop-down`, `felt-tip`, `t-shirt` and
`yo-yo`.
The page joins its five words with `-` (D17-A), so a suggestion holding one
of them shows six or more parts, and a reader cannot tell the words apart
by the dashes.
The number of suggestions is not changed by it: each is five draws of 7,776,
7,776⁵ or about 64.6 bits, and none of `drop`, `felt`, `t` and `yo` is a
word of the list, so a suggestion reads back into its five words in one way
alone and two different draws never give the same password.

System info's credits (`make credits`, `tools/credits/README.md`) take the
entry from this table:

| Directory | Package | Version | Licence |
|-----------|---------|---------|---------|
| `.` | `eff_large_wordlist` | 2016-07-19 | CC-BY-4.0, `LICENSE` |
