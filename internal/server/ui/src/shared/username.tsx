// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/username.tsx
 *
 * A new user's login name and its check, for the two forms that make a
 * user: System settings → Users' Add a user and the first owner's form
 * (AP UiParts, Block 14; users.js and login.js before). The name is
 * checked NAME_CHECK_MS after the last keystroke and when the field is
 * left, against the store's rule and the names in use; a keystroke that
 * fixes it clears the line under the field at once; a refusal the server
 * words alike (wordNameError) goes to the same line.
 */

import type { Ref } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";
import { ResultLine } from "./badge";
import { Field } from "./field";

/** USER_NAME is the store's rule for a user's name (userName in internal/store/users.go), after NormalName: trimmed and lower case. */
const USER_NAME = /^[a-z0-9][a-z0-9._@-]{0,63}$/;

/** NAME_CHECK_MS is how long a name being typed waits before it is checked: the owner asked for a check once the field has not changed for 2 s. */
const NAME_CHECK_MS = 2000;

/** nameProblem says what is wrong with a new user's name, "" for nothing: the store's rule, or a name in use among taken. */
function nameProblem(name: string, taken: string[]): string {
  const n = name.trim().toLowerCase();
  if (!n) return "";
  if (!USER_NAME.test(n)) return t("users.nameBad");
  if ((taken || []).includes(n)) return t("users.nameTaken", { name: n });
  return "";
}

/** wordNameError words the server's refusal of a new user's name as the check does, or returns "" for another error: 409, the name is in use; 400 about the name, the rule. */
export function wordNameError(e: Error & { status?: number }, name: string): string {
  if (e.status === 409) return t("users.nameTaken", { name: name.trim().toLowerCase() });
  if (e.status === 400 && /user name/.test(e.message)) return t("users.nameBad");
  return "";
}

/** A name's check as a form keeps it (useNameCheck): what is wrong now, and how the field and the submit ask. */
export interface NameCheck {
  /** What the line under the field says, "" for nothing. */
  problem: string;
  /** say puts a text on the line, a refusal the server worded. */
  say(text: string): void;
  /** check checks v at once and reports whether it may be sent. */
  check(v: string): boolean;
  /** typed is told of every keystroke: a fixed name clears the line, a wrong one is checked NAME_CHECK_MS later. */
  typed(v: string): void;
}

/** useNameCheck is a new name's check, against the names taken returns. */
export function useNameCheck(taken: () => string[]): NameCheck {
  const [problem, setProblem] = useState("");
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  const check = (v: string) => {
    clearTimeout(timer.current);
    const p = nameProblem(v, taken());
    setProblem(p);
    return !p;
  };
  return {
    problem,
    say: setProblem,
    check,
    typed(v) {
      clearTimeout(timer.current);
      if (!nameProblem(v, taken())) setProblem("");
      else timer.current = setTimeout(() => check(v), NAME_CHECK_MS);
    },
  };
}

/**
 * UserNameField is a new user's name as a field row: the input of id,
 * the line msgId under it that says what is wrong (the input marked
 * invalid and described by it), checked by nc as it is typed and left;
 * value and set are the form's own; name, autocomplete and placeholder
 * are the input's (the first owner's form names it not, as its plain
 * markup did, so that a password manager pairs that form as before),
 * inputRef gives it the focus.
 */
export function UserNameField({ label, id, msgId, value, set, nc, name, autocomplete, placeholder, inputRef }: {
  label: string;
  id: string;
  msgId: string;
  value: string;
  set: (v: string) => void;
  nc: NameCheck;
  name?: string;
  autocomplete: string;
  placeholder?: string;
  inputRef?: Ref<HTMLInputElement>;
}) {
  return (
    <Field label={label} id={id} wide>
      <input ref={inputRef} id={id} type="text" name={name} class="field-control" autocomplete={autocomplete} autocapitalize="none" spellcheck={false}
        maxLength={64} required placeholder={placeholder} value={value} aria-invalid={nc.problem ? "true" : undefined} aria-describedby={msgId}
        onInput={(ev) => {
          const v = ev.currentTarget.value;
          set(v);
          nc.typed(v);
        }}
        onBlur={() => nc.check(value)} />
      <ResultLine id={msgId} cls="small cubes-msg field-hint" role="status" res={nc.problem ? { text: nc.problem, ok: false } : null} />
    </Field>
  );
}
