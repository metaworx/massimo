// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/suggest.ts
 *
 * A suggested password (AP PasswordRules, D14-A to D19-A): five words of
 * EFF's long list, lower case and joined by "-" (D16-A, D17-A), or twenty
 * characters of an alphabet of 56, letters and digits without the
 * look-alikes 0 O o 1 l I (D15-C). Picked without bias: each word by a
 * 32-bit number of crypto.getRandomValues, drawn again at or above the
 * largest multiple of 7,776 below 2^32, each character by a byte, drawn
 * again at or above 224 (4 × 56). The list, ui/words-en.txt, is the
 * page's static file, fetched once at the first suggestion that needs
 * it (D18-A) and held to the list's form, 7,776 words of a-z with "-"
 * inside, none twice; a fetch that fails, or a file of another form, is
 * a ListError and is tried again at the next. A
 * suggestion the caller's rule refuses is drawn again, at most TRIES
 * times. The kind chosen, words or characters, is kept in this browser
 * (readStored, store), and for this page where the browser keeps
 * nothing. Nothing here logs or keeps a password.
 */

import { signal } from "@preact/signals";

/** The two kinds of a suggestion. */
export type SuggestKind = "words" | "chars";

/** WORDS is the number of words of a suggestion; CHARS its number of characters. */
export const WORDS = 5;
export const CHARS = 20;

/** LIST_SIZE is the number of words of EFF's long list, 6^5, one per throw of five dice. */
export const LIST_SIZE = 7776;

/** ALPHABET is the characters a suggestion of characters draws from: letters and digits without 0 O o 1 l I, 56. */
export const ALPHABET = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz";

/** TRIES is the most suggestions drawn for one press where the rule refuses them (a name of the user's that is a word of the list). */
export const TRIES = 20;

/** WORD_LIMIT is where a 32-bit number is drawn again: the largest multiple of LIST_SIZE at or below 2^32. */
const WORD_LIMIT = Math.floor(2 ** 32 / LIST_SIZE) * LIST_SIZE;

/** CHAR_LIMIT is where a byte is drawn again: the largest multiple of the alphabet's 56 at or below 256. */
const CHAR_LIMIT = Math.floor(256 / ALPHABET.length) * ALPHABET.length;

/** The key the kind chosen is kept under in this browser. */
const KIND_KEY = "massimo.suggest";

/** suggestKind is the kind of suggestion chosen last in this browser, words where none was. */
export const suggestKind = signal<SuggestKind>(readStored(KIND_KEY) === "chars" ? "chars" : "words");

/** chooseKind makes kind the one suggested from now on, and keeps it in this browser where it can. */
export function chooseKind(kind: SuggestKind): void {
  suggestKind.value = kind;
  store(KIND_KEY, kind);
}

/** pick is a number below n, without bias: a 32-bit number drawn again at or above the largest multiple of n below 2^32. */
function pick(n: number, limit: number): number {
  const one = new Uint32Array(1);
  for (;;) {
    crypto.getRandomValues(one);
    if (one[0] < limit) return one[0] % n;
  }
}

/** words is a suggestion of WORDS words of list, joined by "-": each picked by pick, without bias. */
export function words(list: readonly string[]): string {
  if (list.length !== LIST_SIZE) throw new Error("suggest: the word list has " + list.length + " words, not " + LIST_SIZE);
  const out: string[] = [];
  for (let i = 0; i < WORDS; i++) out.push(list[pick(LIST_SIZE, WORD_LIMIT)]);
  return out.join("-");
}

/** chars is a suggestion of CHARS characters of ALPHABET: bytes drawn CHARS at a time, each at or above CHAR_LIMIT drawn again. */
export function chars(): string {
  const bytes = new Uint8Array(CHARS);
  let out = "";
  while (out.length < CHARS) {
    crypto.getRandomValues(bytes);
    for (const b of bytes) if (b < CHAR_LIMIT && out.length < CHARS) out += ALPHABET[b % ALPHABET.length];
  }
  return out;
}

/** A WORD is a word of the list's form: a-z, with "-" inside. */
const WORD = /^[a-z]+(-[a-z]+)*$/;

/** ListError is the word list not had: its fetch failed, or the file is not 7,776 words of the list's form, each once. */
export class ListError extends Error {}

/** listOf is the words of the file text, or a ListError where it is not LIST_SIZE lines, each a WORD, none twice, every line ending in a newline. */
function listOf(text: string): string[] {
  const list = text.endsWith("\n") ? text.slice(0, -1).split("\n") : [];
  if (list.length !== LIST_SIZE) throw new ListError("suggest: ui/words-en.txt has " + list.length + " lines, not " + LIST_SIZE);
  const bad = list.find((w) => !WORD.test(w));
  if (bad !== undefined) throw new ListError("suggest: ui/words-en.txt holds a line that is no word: " + JSON.stringify(bad));
  if (new Set(list).size !== LIST_SIZE) throw new ListError("suggest: ui/words-en.txt holds a word twice");
  return list;
}

/** The list's load: its words once fetched, or the fetch that runs, which a second press meanwhile waits for. */
let loaded: string[] | null = null;
let loading: Promise<string[]> | null = null;

/**
 * wordList is EFF's long list, the page's ui/words-en.txt: fetched at
 * the first call, the fetch running shared by the calls meanwhile, the
 * words kept for the page's life; a fetch that fails, or a file not of
 * the list's form (listOf), rejects with a ListError and is tried again
 * at the next call.
 */
export function wordList(): Promise<string[]> {
  if (loaded) return Promise.resolve(loaded);
  if (!loading) {
    loading = (async () => {
      try {
        let text: string;
        try {
          const resp = await fetch("ui/words-en.txt");
          if (!resp.ok) throw new Error("answered " + resp.status);
          text = await resp.text();
        } catch (e) {
          throw new ListError("suggest: ui/words-en.txt not fetched: " + (e as Error).message);
        }
        loaded = listOf(text);
        return loaded;
      } finally {
        loading = null;
      }
    })();
  }
  return loading;
}

/**
 * suggestPassword is a suggestion of kind that refused does not refuse:
 * drawn again where it does, at most TRIES times, null where every one
 * was refused. A suggestion of words waits for the list (wordList) and
 * rejects with a ListError where it cannot be had; a failure of the
 * browser's random numbers rejects with its own error. MassimoParts.suggestPassword, so that
 * the UI test draws a thousand and holds each to the rule.
 */
export async function suggestPassword(kind: SuggestKind, refused: (password: string) => boolean = () => false): Promise<string | null> {
  const list = kind === "words" ? await wordList() : null;
  for (let i = 0; i < TRIES; i++) {
    const p = list ? words(list) : chars();
    if (!refused(p)) return p;
  }
  return null;
}
