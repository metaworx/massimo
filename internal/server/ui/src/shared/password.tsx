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
 * the form that holds it, or, in the card, not controlled at all and read
 * through their refs at the submit (UiParts D210); newPasswordProblem is
 * the one check of the pair.
 */

import { signal } from "@preact/signals";
import type { Ref } from "preact";
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "preact/hooks";
import { Field } from "./field";
import { Icon } from "./icon";
import { useLang } from "./lang";

/** Whether the password fields offer their eye: the server's setting auth.password_reveal, as /healthz says it; null until it has, when none shows one. */
export const reveal = signal<boolean | null>(null);

/** newPasswordProblem says what is wrong with a new password typed twice, "" for nothing: the two differ, or none is typed. */
export function newPasswordProblem(password: string, again: string): string {
  if (password !== again) return t("setup.mismatch");
  if (!password) return t("login.missing");
  return "";
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
 * (the sign-in card's openings) and at the form's submit, before the
 * form's own handler, so that a password manager reads a password field.
 * Shown as text, the keyboard is asked for no capitals, correction or
 * spelling. Without the server's word, or with the setting off, there is
 * no eye and the field stays a password's. id, name, autocomplete,
 * required, disabled and placeholder are the input's, inputCls its class
 * (field-control in a row, beside the box's own); value and set, or
 * inputRef alone, as One says. reset is whatever the caller counts as a
 * new start: the card's openings, a fold's open state; a password shown
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
}) {
  useLang();
  const { id, name, autocomplete, required, disabled, placeholder, inputCls, cls, reset, inputRef } = props;
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
        onInput={props.set ? (ev) => props.set?.(ev.currentTarget.value) : undefined} />
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
 * shown is hidden again as its fold closes).
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
}) {
  const { label, id, hint, cls, ...input } = props;
  return (
    <Field label={label} id={id} wide hint={hint} cls={cls}>
      <PasswordInput {...input} id={id} cls="field-control" />
    </Field>
  );
}

/** The two fields' values: the caller's state and its setters, or, not controlled, the refs the caller reads them through. */
type Pair =
  | { value: string; set: (v: string) => void; again: string; setAgain: (v: string) => void; inputRefs?: undefined }
  | { value?: undefined; set?: undefined; again?: undefined; setAgain?: undefined; inputRefs: [Ref<HTMLInputElement>, Ref<HTMLInputElement>] };

/**
 * NewPassword is the new password's two fields, each with its eye: ids
 * their inputs' ids, label the first's (Again the second's); value and
 * again are what the caller keeps of each, set and setAgain told of every
 * keystroke, or instead inputRefs, the fields not controlled (the sign-in
 * card's first owner's form); names their name attributes where the
 * caller's form names them (My profile's), none without (the first
 * owner's, as its plain markup had them); required marks both required
 * (the first owner's form); reset hides both again at each of its changes
 * (the card's openings, My profile's fold).
 */
export function NewPassword(props: Pair & {
  ids: [string, string];
  names?: [string, string];
  label: string;
  required?: boolean;
  reset?: unknown;
}) {
  const { ids, names, label, required, reset } = props;
  const [first, second]: [One, One] = props.inputRefs
    ? [{ inputRef: props.inputRefs[0] }, { inputRef: props.inputRefs[1] }]
    : [{ value: props.value, set: props.set }, { value: props.again, set: props.setAgain }];
  return (
    <>
      <PasswordField label={label} id={ids[0]} name={names?.[0]} autocomplete="new-password" required={required} reset={reset} {...first} />
      <PasswordField label={t("setup.again")} id={ids[1]} name={names?.[1]} autocomplete="new-password" required={required} reset={reset} {...second} />
    </>
  );
}
