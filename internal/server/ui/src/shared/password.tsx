// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/password.tsx
 *
 * The password fields, every one of the page's (AP MobileApp, D84-A): a
 * field with its eye, the button at its right end inside its row that
 * shows the password in clear text and hides it again (PasswordInput),
 * its row (PasswordField), and a new password typed twice, for the forms
 * that set one: My profile's Change password and the first owner's form
 * in the sign-in card (AP UiParts, Block 14; AP FirstOwner). The eye is
 * offered where the server's setting auth.password_reveal is on, as
 * /healthz says it before anyone signs in (reveal, header.js
 * checkVersion), and none before it has said; the browser's own eye
 * (Edge's) is hidden on every field of the page in each case (style.css),
 * so that a field has one eye at most. The fields are free fields of the caller's own
 * state, never a draft and in no store, so that the password goes with
 * the form that holds it, or, in the card, not controlled at all, read
 * through their refs at the submit (UiParts D210) and emptied by the card
 * at its openings and after a success (AP LooseEnds, D13);
 * newPasswordProblem is the one check of the pair. A new password is held
 * to the rule of internal/password (AP PasswordRules, D4-A, D7-A, D11-A),
 * passwordProblem, which answers its cases (testdata/cases.json) as Go
 * does: 8 to 64 code points, as typed, and no word of its context, the
 * user's names and the server's (D50-A; passwordRefusal, which gives it
 * the server's, serverNames); and the line under the field of a new
 * password, PasswordRule, says what the rule finds while it is typed
 * (D6-A). The first new password of each form that sets one has a dice
 * beside its eye that suggests one (D14-A to D19-A; shared/suggest.ts),
 * five words and twenty characters by turns (AP AccessEnds, D44-A),
 * fills the form with it and shows it, in the field and whole in a line
 * under it (D58-A), where a tap copies it. The sign-in, and the upgrade
 * of a version-1 user at its sign-in (login.js upgradePassword), hold no
 * rule: they take the password a user has.
 */

import { signal } from "@preact/signals";
import type { Ref } from "preact";
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "preact/hooks";
import { app, serverOf } from "./app";
import { Copy } from "./copy";
import { Field } from "./field";
import { Icon } from "./icon";
import { useLang } from "./lang";
import { session, stillSession } from "./session";
import { ListError, type SuggestKind, suggestPassword } from "./suggest";

/** Whether the password fields offer their eye: the server's setting auth.password_reveal, as /healthz says it; null until it has, when none shows one. */
export const reveal = signal<boolean | null>(null);

/** MIN_PASSWORD and MAX_PASSWORD are the fewest and the most code points of a new password: internal/password's MinLength and MaxLength. */
export const MIN_PASSWORD = 8;
export const MAX_PASSWORD = 64;

/** A refusal of the rule, in the answers of internal/password's testdata/cases.json; a password it takes is answered "" ("ok" there). */
export type PasswordAnswer = "too_short" | "too_long" | "context";

/** MIN_WORD is the fewest letters a word of the context has: internal/password's MinWord. */
const MIN_WORD = 4;

/** ALWAYS are the names every new password is checked against beside the caller's: internal/password's Always. */
const ALWAYS = ["massimo", "maxcube"];

/** fold lower-cases s a code point at a time, by Unicode's simple mapping as Go's unicode.ToLower: the first code point of each one's toLowerCase, which is longer only for İ (i and a combining dot, where the simple mapping is i). */
function fold(s: string): string {
  return [...s].map((c) => [...c.toLowerCase()][0] || "").join("");
}

/** contextWords are the words of ALWAYS and of names, as passwordProblem takes them, lower-cased: each name trimmed of what is neither a letter, a mark nor a digit at either end, and each run of letters and marks in it, where it has MIN_WORD letters or more. */
function contextWords(names: readonly string[]): string[] {
  const out: string[] = [];
  const add = (w: string) => {
    if ((w.match(/\p{L}/gu) || []).length >= MIN_WORD) out.push(fold(w));
  };
  for (const name of [...ALWAYS, ...names]) {
    add(name.replace(/^[^\p{L}\p{M}\p{Nd}]+|[^\p{L}\p{M}\p{Nd}]+$/gu, ""));
    for (const run of name.match(/[\p{L}\p{M}]+/gu) || []) add(run);
  }
  return out;
}

/**
 * passwordProblem is the rule a new password is held to, as Go's
 * password.Check holds it: too_short below MIN_PASSWORD code points,
 * too_long above MAX_PASSWORD, then context where it contains a word of
 * its context, "" where it may be set. A code point counts one ([...s],
 * as utf8.RuneCountInString counts), however many UTF-16 units or UTF-8
 * bytes it takes, and the password is counted as typed, not normalised.
 * context are the names it is checked against beside massimo and
 * maxcube, as Go's: each gives the words contextWords says, and the
 * password and the words are compared lower-cased a code point at a
 * time (fold), so that both languages answer alike. The empty password
 * is too short: where empty means unchanged or keys only, the caller
 * does not ask. MassimoParts.passwordProblem, so that the UI test gives
 * it every case Go is tested with.
 */
export function passwordProblem(password: string, context: readonly string[] = []): PasswordAnswer | "" {
  const n = [...password].length;
  if (n < MIN_PASSWORD) return "too_short";
  if (n > MAX_PASSWORD) return "too_long";
  const folded = fold(password);
  if (contextWords(context).some((w) => folded.includes(w))) return "context";
  return "";
}

/**
 * hostWord is the server's name a host gives, as Go's password.HostWord
 * answers: the first label of a host name, without what follows a single
 * colon, its port; "" for anything in brackets or with more than one
 * colon (an IPv6 address), for an IPv4 address, and for localhost,
 * lower-cased as the rule lower-cases (fold), which name no server.
 * MassimoParts.hostWord, so that the UI test gives it every host of
 * cases.json.
 */
export function hostWord(host: string): string {
  let h = host.trim();
  if (h.startsWith("[") || h.split(":").length > 2) return ""; // an IPv6 address, with its port or not
  h = h.split(":")[0];
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(h) && h.split(".").every((n) => Number(n) <= 255 && (n === "0" || !n.startsWith("0")))) return "";
  const label = h.split(".")[0];
  return fold(label) === "localhost" ? "" : label;
}

/**
 * serverNames are the server's names a new password must not contain:
 * the first label of the page's host (hostWord), and in the app the name
 * it gives this server (shared/app.ts serverOf, the sign-in card's, AP
 * MobileApp, D62-A), none where that name is the server's address or
 * its host and port, as the card leaves it out, and none where it is an
 * address of another form (a "://" in it), which the card still shows as
 * the name but which names no server. A name gives the rule every run of
 * letters in it, where Go takes of a host its first label alone: the
 * page can only refuse more. A signal's read, so that a part that draws
 * the rule follows the app's status.
 */
export function serverNames(): string[] {
  const name = serverOf(app.value)?.name || "";
  return [hostWord(location.hostname), name.includes("://") ? "" : name];
}

/** passwordRefusal is the rule's answer for a new password on this page: passwordProblem with the user's names given and the server's (serverNames). */
export function passwordRefusal(password: string, names: readonly string[]): PasswordAnswer | "" {
  return passwordProblem(password, [...names, ...serverNames()]);
}

/** The words of each refusal under the field: now while it is typed, refused once a submit was refused for it. */
const SAID: Record<PasswordAnswer, { now: () => string; refused: () => string }> = {
  too_short: { now: () => t("password.tooShort"), refused: () => t("password.refusedShort") },
  too_long: { now: () => t("password.tooLong"), refused: () => t("password.refusedLong") },
  context: { now: () => t("password.context"), refused: () => t("password.refusedContext") },
};

/** passwordSaid is the words of a refusal of the rule; refused, those of a submit refused for it. */
export function passwordSaid(answer: PasswordAnswer, refused = false): string {
  return SAID[answer][refused ? "refused" : "now"]();
}

/** What newPasswordProblem finds: its words, and whether it is the rule's, the first field's to change, not the pair's. */
export interface NewPasswordProblem {
  text: string;
  rule: boolean;
}

/**
 * newPasswordProblem says what is wrong with a new password typed twice,
 * null for nothing: the first typed and broken by the rule
 * (passwordRefusal, with the user's names), which its line says
 * already, so that the two need not be typed alike first; then the two
 * differ, or none is typed.
 */
export function newPasswordProblem(password: string, again: string, context: readonly string[]): NewPasswordProblem | null {
  const answer = password ? passwordRefusal(password, context) : "";
  if (answer) return { text: passwordSaid(answer, true), rule: true };
  if (password !== again) return { text: t("setup.mismatch"), rule: false };
  if (!password) return { text: t("login.missing"), rule: false };
  return null;
}

/** What the line says of a suggestion (PasswordField's suggest): one shown, unchanged since; the list not loaded; none the rule takes found. */
export type SuggestNote = "" | "suggested" | "nolist" | "none";

/** The words of each note of a suggestion but the empty one. */
const NOTES: Record<Exclude<SuggestNote, "">, () => string> = {
  suggested: () => t("password.suggested"),
  nolist: () => t("password.noList"),
  none: () => t("password.noSuggestion"),
};

/** The rule's line's texts beside the refusals': the hint while the field is empty, a password the rule takes, and the notes of a suggestion. */
const RULE_TEXTS = [() => t("password.hint"), ...Object.values(SAID).flatMap((w) => [w.now, w.refused]), () => t("password.fine"), ...Object.values(NOTES)];

/**
 * PasswordRule is the line under a new password's field that says what
 * the rule finds while it is typed (AP PasswordRules, D6-A): the hint
 * with an i while the field is empty; then too short or too long with a
 * warning's sign, or that it can be set with a tick, words and a sign
 * and never a colour alone; refused, that a submit was refused for it,
 * until the next key. id is the line's, which the field names as its
 * description; value is what the field holds, context the rule's words,
 * read at each drawing. note is what it says of a suggestion: one shown
 * and unchanged ("suggested") in place of the words of a password taken,
 * which a rule's refusal still comes before; the word list not loaded or
 * no suggestion found ("nolist", "none") with a warning's sign, in place
 * of all else, until the next key or press; fresh, a count raised at each
 * suggestion filled, writes it anew, emptied and then written, so that a
 * screen reader hears it again. It never holds the password.
 * A status region whose text is written
 * only when what it says changes (Preact writes no text it already
 * holds), so that a screen reader hears each change once. Its place is
 * as tall as the tallest of its texts, each drawn hidden in the same
 * cell, so that the rows under it stay where they are as it changes.
 */
export function PasswordRule({ id, value, context, refused, note = "", fresh = 0 }: {
  id: string;
  value: string;
  context: () => readonly string[];
  refused: boolean;
  note?: SuggestNote;
  fresh?: number;
}) {
  useLang();
  const lineRef = useRef<HTMLParagraphElement>(null);
  const latest = useRef("");
  const answer = value ? passwordRefusal(value, context()) : "";
  const failed = note === "nolist" || note === "none";
  const look = failed ? " bad" : !value ? "" : answer ? (refused ? " bad refused" : " bad") : " ok";
  const text = failed ? NOTES[note]() : !value ? t("password.hint") : answer ? passwordSaid(answer, refused) : note ? NOTES[note]() : t("password.fine");
  const sign = failed ? "mark-warn" : !value ? "icon-info" : answer ? "mark-warn" : "icon-check";
  latest.current = text;
  // written anew at each change of fresh, as showBubble() of components.js writes its status: emptied, then the words, so
  // that a screen reader hears a text it holds already; the words as they are by then
  useLayoutEffect(() => {
    const words = lineRef.current?.lastChild;
    if (!fresh || !(words instanceof Text)) return;
    words.data = "";
    const timer = setTimeout(() => {
      words.data = latest.current;
    }, 50);
    return () => {
      clearTimeout(timer);
      words.data = latest.current;
    };
  }, [fresh]);
  return (
    <div class="pw-rule small">
      <p ref={lineRef} id={id} class={"pw-rule-line" + look} role="status">
        <Icon key={sign} symbol={sign} cls="ni pw-rule-sign" />
        {text}
      </p>
      <span class="pw-rule-room" aria-hidden="true">
        {RULE_TEXTS.map((said, i) => <span key={i}>{said()}</span>)}
      </span>
    </div>
  );
}

/** setRef gives a ref of the caller's the element, or null. */
function setRef<T>(r: Ref<T> | undefined, v: T | null): void {
  if (typeof r === "function") r(v);
  else if (r) r.current = v;
}

/** toggled says whether a fold the records are of was closed or opened since they began: its open attribute before one of them other than it is now (a set to the state it has is none). */
function toggled(records: MutationRecord[]): boolean {
  return records.some((r) => (r.oldValue !== null) !== (r.target as Element).hasAttribute("open"));
}

/** A password field's value: the caller's state and its setter, or, not controlled, the ref the caller reads it through. */
type One = { value: string; set: (v: string) => void; inputRef?: Ref<HTMLInputElement> } | { value?: undefined; set?: undefined; inputRef: Ref<HTMLInputElement> };

/**
 * PasswordInput is a password's input with its eye: a button at the
 * field's right end, inside its box, a closed eye while the password is
 * hidden and an open one while it shows, labelled Show password and Hide
 * password and pressed while it shows; keyboard operable as a button, and
 * a click that keeps the focus in the field, so that a phone's keyboard
 * neither opens nor closes. It turns the input's type between password
 * and text, keeping its value, its caret where the field has the focus,
 * and its autocomplete; the password hidden again at each change of reset
 * (the sign-in card's openings and successes) and at the form's submit, before the
 * form's own handler, so that a password manager reads a password field.
 * Shown as text, the keyboard is asked for no capitals, correction or
 * spelling. Without the server's word, or with the setting off, there is
 * no eye and the field stays a password's. id, name, autocomplete,
 * required, disabled and placeholder are the input's, inputCls its class
 * (field-control in a row, beside the box's own); value and set, or
 * inputRef alone, as One says; describedBy is the id of the line that
 * describes it (the rule's), invalid marks it refused, typed is told of
 * each keystroke too and of each change event (a fill that sends no
 * input event). reset is whatever the caller counts as a
 * new start: the card's openings and its successes, a fold's open
 * state; a password shown
 * is also hidden whenever a fold the field is in is closed and opened,
 * however fast, in the very task that opens it, before a paint or a read
 * can come between (an observer of the folds' open attribute, whose call
 * is that task's microtask, not the toggle event, which comes in a task
 * of its own and, two toggles pressed before it, once for both);
 * switched off, a password shown is hidden, and stays hidden when the eye
 * comes back. The type never turns while the field is in a closed fold (a
 * details): Chrome draws nothing of an input whose type turned there once
 * the fold opens, so it is turned as the fold opens, before it is painted.
 * suggest, given, draws the dice before the eye, which calls its press,
 * labelled by what that press gives, its next: Suggest five words or
 * Suggest twenty characters (AP AccessEnds, D44-A); while its shown is
 * true the password shows as text, eye or none (a suggestion, D19-A), and
 * the eye, pressed then, hides it (its hide) and the eye's own state is
 * as it was. shown, given, is the state whether the password shows,
 * held by the caller in place of the field's own, so that two fields
 * show and hide together (a new password and its repetition, NewPassword:
 * AP MobileApp, Block 20); with eye false the field draws no eye of its
 * own and shows as shown says, hidden as every field is at the submit,
 * at a fold's closing and with the setting off; controls are the ids of
 * the other fields the eye shows and hides.
 */
export function PasswordInput(props: One & {
  id: string;
  name?: string;
  autocomplete: string;
  required?: boolean;
  disabled?: boolean;
  placeholder?: string;
  inputCls?: string;
  cls?: string;
  reset?: unknown;
  describedBy?: string;
  invalid?: boolean;
  typed?: (v: string) => void;
  suggest?: { shown: boolean; press: () => void; hide: () => void; next: SuggestKind };
  shown?: [boolean, (v: boolean) => void];
  eye?: boolean;
  controls?: string;
}) {
  useLang();
  const { id, name, autocomplete, required, disabled, placeholder, inputCls, cls, reset, inputRef, describedBy, invalid, typed, suggest, controls } = props;
  const reveals = reveal.value === true;
  const on = reveals && props.eye !== false;
  const ownShown = useState(false);
  const [shown, setShown] = props.shown || ownShown;
  const own = useRef<HTMLInputElement | null>(null);
  const caret = useRef<[number, number, "forward" | "backward" | "none"] | null>(null);
  // the keyboard asked for no correction once the password shows as text: an attribute, not the property Chrome and
  // Safari read as a boolean, which a string would turn on
  const bind = useCallback((el: HTMLInputElement | null) => {
    own.current = el;
    if (el) el.setAttribute("autocorrect", "off");
    setRef(inputRef, el);
  }, [inputRef]);
  const [, redraw] = useState(0);
  // a hide waiting for the closed fold the field is in to open, the type held meanwhile (below): Chrome draws an input
  // whose type turned inside a closed details as nothing when it opens
  const pending = useRef(false);
  // hide hides the password: at once, or, the field in a closed fold, as the fold opens again (the observer below)
  const hide = useCallback(() => {
    if (own.current?.closest("details:not([open])")) {
      pending.current = true;
      return;
    }
    setShown(false);
  }, [setShown]);
  // hidden again at each new start the caller counts
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset is the caller's count of its starts, read for nothing else; the effect is for each, so it is listed.
  useLayoutEffect(() => {
    hide();
  }, [reset, hide]);
  // switched off, a password shown before is hidden, and stays so when the eye comes back
  useLayoutEffect(() => {
    if (!reveals) hide();
  }, [reveals, hide]);
  // hidden again whenever a fold the field is in is closed and opened, in the very task that opens it: an observer of
  // the folds' open attribute is called in that task's microtasks, before a paint or a read can come, where the toggle
  // event comes in a task of its own, later on a loaded page, and once for a close and an opening pressed before it.
  // Closed, a password shown waits, its type held; opened, the type turns on the element at once, the state following
  useEffect(() => {
    const folds: Element[] = [];
    for (let d = own.current?.closest("details"); d; d = d.parentElement?.closest("details")) folds.push(d);
    if (!folds.length) return;
    const watch = new MutationObserver((records) => {
      const el = own.current;
      if (!el || !toggled(records)) return;
      if (el.closest("details:not([open])")) {
        if (el.type === "text") pending.current = true;
        return;
      }
      if (!pending.current && el.type !== "text") return;
      pending.current = false;
      el.type = "password";
      setShown(false);
      redraw((n) => n + 1);
    });
    for (const d of folds) watch.observe(d, { attributes: true, attributeFilter: ["open"], attributeOldValue: true });
    return () => watch.disconnect();
  }, [setShown]); // a state's setter, the field's own or the pair's (NewPassword), which stays the same
  // the caret where it was once the type has turned, the field still in focus
  // biome-ignore lint/correctness/useExhaustiveDependencies: shown is what turned the type; the effect follows each turn, so it is listed.
  useLayoutEffect(() => {
    const c = caret.current;
    caret.current = null;
    const el = own.current;
    if (c && el && document.activeElement === el) el.setSelectionRange(c[0], c[1], c[2]);
  }, [shown]);
  // hidden again at the submit, in the capture before the form's own handler, the type turned at once
  useEffect(() => {
    const form = own.current?.form;
    if (!form) return;
    const hide = () => {
      if (own.current) own.current.type = "password";
      setShown(false);
    };
    form.addEventListener("submit", hide, true);
    return () => form.removeEventListener("submit", hide, true);
  }, [setShown]);
  const forced = !!suggest?.shown;
  const visible = forced || (reveals && shown);
  // the type as it is while the field is in a closed fold, turned as it opens (the folds' observer)
  const held = own.current?.closest("details:not([open])") ? own.current.type : "";
  const type = held === "text" || held === "password" ? held : visible ? "text" : "password";
  const toggle = () => {
    const el = own.current;
    if (el && document.activeElement === el && el.selectionStart !== null && el.selectionEnd !== null) {
      caret.current = [el.selectionStart, el.selectionEnd, el.selectionDirection || "none"];
    }
    if (forced) suggest?.hide();
    setShown(!visible);
  };
  const label = t(visible ? "password.hide" : "password.show");
  const dice = t(suggest?.next === "chars" ? "password.suggestChars" : "password.suggestWords");
  return (
    <span class={"pw-box" + (on ? " has-eye" : "") + (suggest ? " has-dice" : "") + (cls ? " " + cls : "")}>
      <input ref={bind} id={id} type={type as "text"} name={name} class={inputCls} autocomplete={autocomplete} autocapitalize="none"
        spellcheck={false} required={required} disabled={disabled} placeholder={placeholder} value={props.set ? props.value : undefined}
        aria-describedby={describedBy} aria-invalid={invalid ? "true" : undefined} onInput={props.set || typed ? (ev) => {
          const v = ev.currentTarget.value;
          props.set?.(v);
          typed?.(v);
        } : undefined} onChange={typed ? (ev) => typed(ev.currentTarget.value) : undefined} />
      {suggest && (
        // the focus on the dice, in the press's handler, where a browser gives a click on a button none (Safari)
        <button type="button" class="pw-dice" aria-label={dice} title={dice} aria-controls={id} disabled={disabled}
          onClick={(ev) => { ev.currentTarget.focus(); suggest.press(); }}>
          <Icon symbol="icon-dice" cls="ni" />
        </button>
      )}
      {on && (
        <button type="button" class="pw-eye" aria-label={label} title={label} aria-pressed={visible ? "true" : "false"} aria-controls={controls ? id + " " + controls : id}
          disabled={disabled} onMouseDown={(ev) => ev.preventDefault()} onClick={toggle}>
          <Icon symbol={visible ? "icon-eye" : "icon-eye-off"} cls="ni" />
        </button>
      )}
    </span>
  );
}

/**
 * PasswordField is a row (.field, wide) with a password's input and its
 * eye: label and id the row's and the input's, hint a grey line under
 * it, cls the row's classes; the rest as PasswordInput's (reset, a fold's
 * open state for My profile's and a user's panel's, so that a password
 * shown is hidden again as its fold closes). rule, given for a new
 * password, draws the rule's line under the field (PasswordRule, of the
 * id id + "-rule", the input's description) and gives the rule its
 * context words; the line follows the value, or, the field not
 * controlled, its keystrokes and change events, and the field as it is
 * at each submit, and starts again from the empty field at each change
 * of reset (the sign-in card's openings and successes, at which the card
 * empties its fields). refused counts the caller's submits the rule
 * refused (its handler gives the field the focus): at each, until the
 * next key in the field, and only while the value shown is still one the
 * rule refuses, the line says that it was refused and the field is
 * marked invalid; the form's own message line says nothing of it, so
 * that nothing contradicts the line once the password can be set.
 *
 * suggest, given with rule, offers the dice (PasswordInput's suggest): a
 * press draws a suggestion of the kind the field gives next
 * (shared/suggest.ts), five words at the first press and at each new
 * start, then twenty characters and five words by turns: each press
 * turns it, whatever it fills, and a key leaves it, so that the next
 * press gives the other kind of the last, as the dice's label says (AP
 * AccessEnds, D44-A, D50-A, D51-A). A suggestion is drawn
 * again where the rule refuses it, fills the field with it, a controlled
 * one by its set, one not controlled by its element and an input event,
 * so that the line and the caller's state follow, fills also (the form's
 * other field, which also.get reads), and shows it as text, eye or none,
 * while the field holds it: edited, emptied or changed by its form
 * (Reset, a save), or submitted, or the dialog it is in closed (the
 * sign-in card's dismissal, which keeps the fields' values), or at a new
 * start (reset), it is a password's field again. The rule's line says
 * that a password was
 * suggested, never which, written anew at each fill. While the field
 * holds the suggestion, a line right under it shows the suggestion
 * whole (AP PasswordRules, D58-A), which the field, narrower, shows a
 * part of: in the body's font at 16 px, a text that copies itself on a
 * tap, a click, Enter or Space and says "Copied", as the page's other
 * strings do, its copy icon after it (Copy; AP AccessEnds, D44-A, D49-A),
 * a Copy of each suggestion its own, so that Copied never shows over the
 * next, both named Copy the password, its words quiet, hidden from a screen
 * reader, since the field, text then, gives the value and the rule's
 * line says that it was suggested, and words read out inside the live
 * regions of My profile and Users would speak the password (D52-A);
 * translate="no", so that a page's translation leaves its words alone
 * (AP AccessEnds, D20-A to D22-A). The line keeps its place, two lines
 * tall, from the first press on, empty while the suggestion is not shown
 * (AP AccessEnds, D19-A). A key never takes that place away, so that
 * nothing under the field moves between presses or while one types, but
 * after the rarest draws at a phone's width, whose five words take a
 * third line, which the first key gives back: it goes with a value
 * changed from outside (Reset, a save) and at a new start. It is not
 * there before, so that a form keeps its height until the dice is used. A
 * suggestion drawn while another press, a key in the field, a submit, a
 * change from outside, the dialog's close, a new start or a sign-out
 * came, or after either field changed, fills nothing. The word
 * list not had says so in the line ("nolist"); any other failure, that
 * no suggestion could be made.
 *
 * problem, given, is what is wrong with the value now (the repetition of
 * a new password that differs from it, NewPassword), "" for nothing:
 * said in red on a line under the field, which describes the field and
 * marks it invalid while it says something; room is the longest text the
 * line may say, whose place the line keeps while it says nothing, so
 * that nothing under the field moves as the problem comes and goes (as
 * the rule's line keeps its place); shown, eye and controls are
 * PasswordInput's.
 */
export function PasswordField(props: One & {
  label: string;
  id: string;
  name?: string;
  autocomplete: string;
  required?: boolean;
  placeholder?: string;
  hint?: string;
  cls?: string;
  reset?: unknown;
  rule?: () => readonly string[];
  refused?: number;
  suggest?: boolean;
  also?: { set: (v: string) => void; get: () => string };
  problem?: string;
  room?: string;
  shown?: [boolean, (v: boolean) => void];
  eye?: boolean;
  controls?: string;
}) {
  const { label, id, hint, cls, rule, refused = 0, suggest, also, problem, room, ...input } = props;
  const [typed, setTyped] = useState("");
  // the suggestion: the shown line's place offered from the first press, the value filled (a suggestion while the field
  // still holds it), what the line says of one, and the count of the line's fills. turn rises at every press, every
  // key in the field, every submit and every new start: a suggestion drawn while it rose fills nothing
  const [offered, setOffered] = useState(false);
  const [filled, setFilled] = useState("");
  const [note, setNote] = useState<SuggestNote>("");
  const [fills, setFills] = useState(0);
  const turn = useRef(0);
  const filling = useRef(false);
  // the kind the next press gives, five words at each new start and the other kind after each press (AP AccessEnds,
  // D50-A, D51-A): a ref for the press, which a second press may follow before the field is drawn again, and its
  // state for the dice's label
  const nextKind = useRef<SuggestKind>("words");
  const [next, setNext] = useState<SuggestKind>("words");
  // the value as the field's keys and fills last left it: one that differs came from outside (Reset, a save)
  const known = useRef<string | undefined>(undefined);
  // the refusal the line said last until a key: the count of refusals at the last key, or at the last new start
  const [keyed, setKeyed] = useState(refused);
  const el = useRef<HTMLInputElement | null>(null);
  const callerRef = props.inputRef;
  const bind = useCallback((v: HTMLInputElement | null) => {
    el.current = v;
    setRef(callerRef, v);
  }, [callerRef]);
  // a new start: a field not controlled starts again empty, where its caller empties it, and no refusal is said
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset is the caller's mark of a new start (the card's openings and successes, or a fold's open state), read for nothing else; the effect is for each change, so it is listed.
  useLayoutEffect(() => {
    setTyped("");
    setKeyed(refused);
    turn.current++;
    known.current = undefined;
    setOffered(false);
    setFilled("");
    setNote("");
    nextKind.current = "words";
    setNext("words");
  }, [props.reset]);
  // a suggestion shown is hidden at the submit, in the capture before the form's own handler, as the eye's password,
  // and one still drawn fills nothing after it
  useEffect(() => {
    const form = el.current?.form;
    if (!form || !suggest) return;
    const hide = () => {
      turn.current++;
      setFilled("");
    };
    form.addEventListener("submit", hide, true);
    return () => form.removeEventListener("submit", hide, true);
  }, [suggest]);
  // a suggestion shown ends as the dialog the field is in closes (the sign-in card's dismissal, which keeps what the
  // fields hold), so that no line holds the password in a card nobody sees; one still drawn fills nothing after it
  useEffect(() => {
    const dialog = el.current?.closest("dialog");
    if (!dialog || !suggest) return;
    const watch = new MutationObserver(() => {
      if (dialog.open) return;
      turn.current++;
      setFilled("");
    });
    watch.observe(dialog, { attributes: true, attributeFilter: ["open"] });
    return () => watch.disconnect();
  }, [suggest]);
  // at each submit a field not controlled is read as it is, in the capture before the form's own handler, so that the
  // line says what the submit reads, also of a fill that sent no event
  const controlled = !!props.set;
  useEffect(() => {
    const form = el.current?.form;
    if (!form || controlled) return;
    const read = () => {
      if (el.current) setTyped(el.current.value);
    };
    form.addEventListener("submit", read, true);
    return () => form.removeEventListener("submit", read, true);
  }, [controlled]);
  const own = rule && !props.set;
  const value = props.set ? props.value : typed;
  // a suggestion while the field holds what was filled: emptied or changed, by a key or by its form (Reset, a save's
  // success), it is a password's field again, the eye as it was, and the shown line is empty
  const suggested = filled !== "" && value === filled;
  // refused only while the value shown is still one the rule refuses: a field emptied or changed by its form (Reset, a
  // draft taken away, a save's success) is no longer
  const said = !!rule && refused !== keyed && !!value && !!passwordRefusal(value, rule());
  // a value changed from outside, not by a key or a fill (the form's Reset, a save's success): the suggestion and its
  // line's place end, the next press gives five words, and one still drawn fills nothing
  useLayoutEffect(() => {
    if (known.current === undefined || value === known.current) return;
    known.current = undefined;
    turn.current++;
    setOffered(false);
    setFilled("");
    setNote("");
    nextKind.current = "words";
    setNext("words");
  }, [value]);
  const key = (v: string) => {
    known.current = v;
    if (own) setTyped(v);
    setKeyed(refused);
    if (filling.current) return;
    turn.current++; // a key: a suggestion still drawn does not replace what is typed
    setFilled("");
    setNote("");
  };
  // the field filled with a suggestion, and the form's other field (also); the line and the caller's state follow, and
  // the line is written anew, so that a screen reader hears that another password was suggested
  const fill = (v: string) => {
    known.current = v;
    filling.current = true;
    try {
      if (props.set) props.set(v);
      else if (el.current) {
        el.current.value = v;
        el.current.dispatchEvent(new Event("input", { bubbles: true }));
      }
    } finally {
      filling.current = false;
    }
    also?.set(v);
    setFilled(v);
    setNote("");
    setFills((f) => f + 1);
  };
  // a press gives the kind next names and turns it at once, whatever it fills (D51-A): after a list not had, the next
  // press gives twenty characters, which need none
  const press = async () => {
    const k = nextKind.current;
    nextKind.current = k === "words" ? "chars" : "words";
    setNext(nextKind.current);
    const n = session();
    const mine = ++turn.current;
    // what the two fields hold at the press: a suggestion that comes after either changed fills nothing
    const held = [el.current?.value ?? "", also?.get() ?? ""];
    setOffered(true);
    let v: string | null;
    try {
      v = await suggestPassword(k, (p) => !!passwordRefusal(p, rule ? rule() : []));
    } catch (e) {
      if (stillSession(n) && mine === turn.current) setNote(e instanceof ListError ? "nolist" : "none");
      return;
    }
    if (!stillSession(n) || mine !== turn.current || (el.current?.value ?? "") !== held[0] || (also?.get() ?? "") !== held[1]) return;
    if (v === null) setNote("none");
    else fill(v);
  };
  const dice = suggest && rule ? { shown: suggested, press, hide: () => setFilled(""), next } : undefined;
  return (
    <Field label={label} id={id} wide hint={hint} cls={cls}>
      <PasswordInput {...input} inputRef={bind} id={id} cls="field-control" describedBy={rule ? id + "-rule" : problem ? id + "-problem" : undefined}
        invalid={said || !!problem} typed={rule ? key : undefined} suggest={dice} />
      {dice && offered && (
        // the suggestion whole while the field holds it, a text that copies itself with its icon, its words quiet, its
        // place there from the first press; a Copy of each suggestion its own (its key), so that Copied, said of the last
        // one, never shows over the next, the press having given the focus to the dice; translate false, the property,
        // writes the attribute translate="no", where a string would turn it on
        <p id={id + "-shown"} class="pw-shown" translate={false}>
          {suggested && <Copy key={filled} text={filled} name={t("password.copyName")} icon quiet cls="pw-copy" />}
        </p>
      )}
      {rule && <PasswordRule id={id + "-rule"} value={value} context={rule} refused={said} note={note || (suggested ? "suggested" : "")} fresh={fills} />}
      {(problem || room) && (
        // the line and, hidden in the same cell, the longest text it may say, so that its place stays while it is empty
        <p class="small pw-problem">
          {room && <span class="pw-problem-room" aria-hidden="true">{room}</span>}
          <span id={id + "-problem"} class="pw-problem-now">{problem || ""}</span>
        </p>
      )}
    </Field>
  );
}

/** The two fields' values: the caller's state and its setters, or, not controlled, the refs the caller reads them through. */
type Pair =
  | { value: string; set: (v: string) => void; again: string; setAgain: (v: string) => void; inputRefs?: undefined; inputRef?: Ref<HTMLInputElement> }
  | { value?: undefined; set?: undefined; again?: undefined; setAgain?: undefined; inputRefs: [Ref<HTMLInputElement>, Ref<HTMLInputElement>]; inputRef?: undefined };

/**
 * NewPassword is the new password's two fields, one eye for both: ids
 * their inputs' ids, label the first's (Again the second's); value and
 * again are what the caller keeps of each, set and setAgain told of every
 * keystroke, with inputRef the first's ref, for its focus, or instead
 * inputRefs, the fields not controlled (the sign-in card's first owner's
 * form); names their name attributes where the caller's form names them
 * (My profile's), none without (the first owner's, as its plain markup
 * had them); required marks both required (the first owner's form);
 * reset hides both again at each of its changes (the card's openings, My
 * profile's fold); context are the rule's words, and the rule's line is
 * the first field's (PasswordField's rule), refused its count of the
 * submits the rule refused; suggest gives the first field its dice
 * (PasswordField's suggest), whose suggestion fills the second too: by
 * setAgain, or the field not controlled, by its element and an input
 * event; it shows whole under the first alone. One eye, the first
 * field's, shows and hides both, the second drawing none (AP MobileApp,
 * Block 20), and the setting auth.password_reveal turns it off as every
 * other. The second is outlined in red while it is not empty and differs
 * from the first, "The two passwords differ." under it (setup.mismatch,
 * D98-A), both gone as they match, the line's place kept, so that nothing
 * under the pair moves as it comes and goes: of the caller's values, or, the
 * fields not controlled, of what their keystrokes, fills and change
 * events leave, read again at each submit and empty again at each new
 * start (reset), at which the caller empties them.
 */
export function NewPassword(props: Pair & {
  ids: [string, string];
  names?: [string, string];
  label: string;
  required?: boolean;
  reset?: unknown;
  context: () => readonly string[];
  refused: number;
  suggest?: boolean;
}) {
  const { ids, names, label, required, reset, context, refused, suggest } = props;
  // the second field's element, for a suggestion where it is not controlled; the caller's ref told too
  const againEl = useRef<HTMLInputElement | null>(null);
  const againRef = props.inputRefs?.[1];
  const bindAgain = useCallback((v: HTMLInputElement | null) => {
    againEl.current = v;
    setRef(againRef, v);
  }, [againRef]);
  // the first field's element where it is not controlled, for the pair's comparison; the caller's ref told too
  const firstEl = useRef<HTMLInputElement | null>(null);
  const firstRef = props.inputRefs?.[0];
  const bindFirst = useCallback((v: HTMLInputElement | null) => {
    firstEl.current = v;
    setRef(firstRef, v);
  }, [firstRef]);
  const [first, second]: [One, One] = props.inputRefs
    ? [{ inputRef: bindFirst }, { inputRef: bindAgain }]
    : [{ value: props.value, set: props.set, inputRef: props.inputRef }, { value: props.again, set: props.setAgain, inputRef: bindAgain }];
  // one eye for both: the state whether the two show, the first field's eye its button
  const pair = useState(false);
  // the two values where the fields are not controlled, as their keystrokes, fills and change events leave them
  const controlled = !props.inputRefs;
  const [typed, setTyped] = useState<[string, string]>(["", ""]);
  useEffect(() => {
    const a = firstEl.current;
    const b = againEl.current;
    if (controlled || !a || !b) return;
    const read = () => setTyped([a.value, b.value]);
    for (const el of [a, b]) {
      el.addEventListener("input", read);
      el.addEventListener("change", read);
    }
    const form = a.form;
    form?.addEventListener("submit", read, true);
    return () => {
      for (const el of [a, b]) {
        el.removeEventListener("input", read);
        el.removeEventListener("change", read);
      }
      form?.removeEventListener("submit", read, true);
    };
  }, [controlled]);
  // a new start: the caller empties fields not controlled
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset is the caller's mark of a new start, read for nothing else; the effect is for each change, so it is listed.
  useLayoutEffect(() => {
    setTyped(["", ""]);
  }, [reset]);
  const [p1, p2] = controlled ? [props.value ?? "", props.again ?? ""] : typed;
  const differ = p2 !== "" && p2 !== p1;
  const setAgain = props.setAgain;
  const also = {
    set: (v: string) => {
      if (setAgain) setAgain(v);
      else if (againEl.current) {
        againEl.current.value = v;
        againEl.current.dispatchEvent(new Event("input", { bubbles: true }));
      }
    },
    get: () => againEl.current?.value ?? "",
  };
  return (
    <>
      <PasswordField label={label} id={ids[0]} name={names?.[0]} autocomplete="new-password" required={required} reset={reset} rule={context} refused={refused}
        suggest={suggest} also={also} shown={pair} controls={ids[1]} {...first} />
      <PasswordField label={t("setup.again")} id={ids[1]} name={names?.[1]} autocomplete="new-password" required={required} reset={reset} shown={pair} eye={false}
        problem={differ ? t("setup.mismatch") : ""} room={t("setup.mismatch")} {...second} />
    </>
  );
}
