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
 * does: 8 to 64 code points, as typed; and the line under the field of a
 * new password, PasswordRule, says what the rule finds while it is typed
 * (D6-A). The sign-in, and the upgrade of a version-1 user at its sign-in
 * (login.js upgradePassword), hold no rule: they take the password a user
 * has.
 */

import { signal } from "@preact/signals";
import type { Ref } from "preact";
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "preact/hooks";
import { Field } from "./field";
import { Icon } from "./icon";
import { useLang } from "./lang";

/** Whether the password fields offer their eye: the server's setting auth.password_reveal, as /healthz says it; null until it has, when none shows one. */
export const reveal = signal<boolean | null>(null);

/** MIN_PASSWORD and MAX_PASSWORD are the fewest and the most code points of a new password: internal/password's MinLength and MaxLength. */
export const MIN_PASSWORD = 8;
export const MAX_PASSWORD = 64;

/** A refusal of the rule, in the answers of internal/password's testdata/cases.json; a password it takes is answered "" ("ok" there). */
export type PasswordAnswer = "too_short" | "too_long";

/**
 * passwordProblem is the rule a new password is held to, as Go's
 * password.Check holds it: too_short below MIN_PASSWORD code points,
 * too_long above MAX_PASSWORD, "" where it may be set. A code point
 * counts one ([...s], as utf8.RuneCountInString counts), however many
 * UTF-16 units or UTF-8 bytes it takes, and the password is counted as
 * typed, not normalised. context are the words it is checked against,
 * the user's names, taken from every caller and refusing nothing yet, as
 * Go's. The empty password is too short: where empty means unchanged or
 * keys only, the caller does not ask. MassimoParts.passwordProblem, so
 * that the UI test gives it every case Go is tested with.
 */
export function passwordProblem(password: string, _context: readonly string[] = []): PasswordAnswer | "" {
  const n = [...password].length;
  if (n < MIN_PASSWORD) return "too_short";
  if (n > MAX_PASSWORD) return "too_long";
  return "";
}

/** The words of each refusal under the field: now while it is typed, refused once a submit was refused for it. */
const SAID: Record<PasswordAnswer, { now: () => string; refused: () => string }> = {
  too_short: { now: () => t("password.tooShort"), refused: () => t("password.refusedShort") },
  too_long: { now: () => t("password.tooLong"), refused: () => t("password.refusedLong") },
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
 * (passwordProblem, with the context words), which its line says
 * already, so that the two need not be typed alike first; then the two
 * differ, or none is typed.
 */
export function newPasswordProblem(password: string, again: string, context: readonly string[]): NewPasswordProblem | null {
  const answer = password ? passwordProblem(password, context) : "";
  if (answer) return { text: passwordSaid(answer, true), rule: true };
  if (password !== again) return { text: t("setup.mismatch"), rule: false };
  if (!password) return { text: t("login.missing"), rule: false };
  return null;
}

/** The rule's line's texts beside the refusals': the hint while the field is empty, and a password the rule takes. */
const RULE_TEXTS = [() => t("password.hint"), ...Object.values(SAID).flatMap((w) => [w.now, w.refused]), () => t("password.fine")];

/**
 * PasswordRule is the line under a new password's field that says what
 * the rule finds while it is typed (AP PasswordRules, D6-A): the hint
 * with an i while the field is empty; then too short or too long with a
 * warning's sign, or that it can be set with a tick, words and a sign
 * and never a colour alone; refused, that a submit was refused for it,
 * until the next key. id is the line's, which the field names as its
 * description; value is what the field holds, context the rule's words,
 * read at each drawing. A status region whose text is written
 * only when what it says changes (Preact writes no text it already
 * holds), so that a screen reader hears each change once. Its place is
 * as tall as the tallest of its texts, each drawn hidden in the same
 * cell, so that the rows under it stay where they are as it changes.
 */
export function PasswordRule({ id, value, context, refused }: { id: string; value: string; context: () => readonly string[]; refused: boolean }) {
  useLang();
  const answer = value ? passwordProblem(value, context()) : "";
  const look = !value ? "" : answer ? (refused ? " bad refused" : " bad") : " ok";
  const text = !value ? t("password.hint") : answer ? passwordSaid(answer, refused) : t("password.fine");
  const sign = !value ? "icon-info" : answer ? "mark-warn" : "icon-check";
  return (
    <div class="pw-rule small">
      <p id={id} class={"pw-rule-line" + look} role="status">
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
}) {
  useLang();
  const { id, name, autocomplete, required, disabled, placeholder, inputCls, cls, reset, inputRef, describedBy, invalid, typed } = props;
  const on = reveal.value === true;
  const [shown, setShown] = useState(false);
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
  }, []);
  // hidden again at each new start the caller counts
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset is the caller's count of its starts, read for nothing else; the effect is for each, so it is listed.
  useLayoutEffect(() => {
    hide();
  }, [reset, hide]);
  // switched off, a password shown before is hidden, and stays so when the eye comes back
  useLayoutEffect(() => {
    if (!on) hide();
  }, [on, hide]);
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
  }, []);
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
  }, []);
  const visible = on && shown;
  // the type as it is while the field is in a closed fold, turned as it opens (the folds' observer)
  const held = own.current?.closest("details:not([open])") ? own.current.type : "";
  const type = held === "text" || held === "password" ? held : visible ? "text" : "password";
  const toggle = () => {
    const el = own.current;
    if (el && document.activeElement === el && el.selectionStart !== null && el.selectionEnd !== null) {
      caret.current = [el.selectionStart, el.selectionEnd, el.selectionDirection || "none"];
    }
    setShown(!visible);
  };
  const label = t(visible ? "password.hide" : "password.show");
  return (
    <span class={"pw-box" + (on ? " has-eye" : "") + (cls ? " " + cls : "")}>
      <input ref={bind} id={id} type={type as "text"} name={name} class={inputCls} autocomplete={autocomplete} autocapitalize="none"
        spellcheck={false} required={required} disabled={disabled} placeholder={placeholder} value={props.set ? props.value : undefined}
        aria-describedby={describedBy} aria-invalid={invalid ? "true" : undefined} onInput={props.set || typed ? (ev) => {
          const v = ev.currentTarget.value;
          props.set?.(v);
          typed?.(v);
        } : undefined} onChange={typed ? (ev) => typed(ev.currentTarget.value) : undefined} />
      {on && (
        <button type="button" class="pw-eye" aria-label={label} title={label} aria-pressed={visible ? "true" : "false"} aria-controls={id}
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
}) {
  const { label, id, hint, cls, rule, refused = 0, ...input } = props;
  const [typed, setTyped] = useState("");
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
  }, [props.reset]);
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
  // refused only while the value shown is still one the rule refuses: a field emptied or changed by its form (Reset, a
  // draft taken away, a save's success) is no longer
  const said = !!rule && refused !== keyed && !!value && !!passwordProblem(value, rule());
  const key = (v: string) => {
    if (own) setTyped(v);
    setKeyed(refused);
  };
  return (
    <Field label={label} id={id} wide hint={hint} cls={cls}>
      <PasswordInput {...input} inputRef={bind} id={id} cls="field-control" describedBy={rule ? id + "-rule" : undefined} invalid={said}
        typed={rule ? key : undefined} />
      {rule && <PasswordRule id={id + "-rule"} value={value} context={rule} refused={said} />}
    </Field>
  );
}

/** The two fields' values: the caller's state and its setters, or, not controlled, the refs the caller reads them through. */
type Pair =
  | { value: string; set: (v: string) => void; again: string; setAgain: (v: string) => void; inputRefs?: undefined; inputRef?: Ref<HTMLInputElement> }
  | { value?: undefined; set?: undefined; again?: undefined; setAgain?: undefined; inputRefs: [Ref<HTMLInputElement>, Ref<HTMLInputElement>]; inputRef?: undefined };

/**
 * NewPassword is the new password's two fields, each with its eye: ids
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
 * submits the rule refused.
 */
export function NewPassword(props: Pair & {
  ids: [string, string];
  names?: [string, string];
  label: string;
  required?: boolean;
  reset?: unknown;
  context: () => readonly string[];
  refused: number;
}) {
  const { ids, names, label, required, reset, context, refused } = props;
  const [first, second]: [One, One] = props.inputRefs
    ? [{ inputRef: props.inputRefs[0] }, { inputRef: props.inputRefs[1] }]
    : [{ value: props.value, set: props.set, inputRef: props.inputRef }, { value: props.again, set: props.setAgain }];
  return (
    <>
      <PasswordField label={label} id={ids[0]} name={names?.[0]} autocomplete="new-password" required={required} reset={reset} rule={context} refused={refused} {...first} />
      <PasswordField label={t("setup.again")} id={ids[1]} name={names?.[1]} autocomplete="new-password" required={required} reset={reset} {...second} />
    </>
  );
}
