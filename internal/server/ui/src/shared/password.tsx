// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/password.tsx
 *
 * A new password typed twice, for the two forms that set one: My
 * profile's Change password and the first owner's form in the sign-in
 * card (AP UiParts, Block 14; AP FirstOwner). The two fields are free
 * fields of the caller's own state, never a draft and in no store, so
 * that the password goes with the form that holds it, or, in the card,
 * not controlled at all and read through their refs at the submit
 * (UiParts D210); newPasswordProblem is the one check of the pair.
 */

import type { Ref } from "preact";
import { TextField } from "./field";

/** newPasswordProblem says what is wrong with a new password typed twice, "" for nothing: the two differ, or none is typed. */
export function newPasswordProblem(password: string, again: string): string {
  if (password !== again) return t("setup.mismatch");
  if (!password) return t("login.missing");
  return "";
}

/** The two fields' values: the caller's state and its setters, or, not controlled, the refs the caller reads them through. */
type Pair =
  | { value: string; set: (v: string) => void; again: string; setAgain: (v: string) => void; inputRefs?: undefined }
  | { value?: undefined; set?: undefined; again?: undefined; setAgain?: undefined; inputRefs: [Ref<HTMLInputElement>, Ref<HTMLInputElement>] };

/**
 * NewPassword is the new password's two fields: ids their inputs' ids,
 * label the first's (Again the second's); value and again are what the
 * caller keeps of each, set and setAgain told of every keystroke, or
 * instead inputRefs, the fields not controlled (the sign-in card's
 * first owner's form); names their name attributes where the caller's
 * form names them (My profile's), none without (the first owner's, as
 * its plain markup had them); required marks both required (the first
 * owner's form).
 */
export function NewPassword(props: Pair & {
  ids: [string, string];
  names?: [string, string];
  label: string;
  required?: boolean;
}) {
  const { ids, names, label, required } = props;
  const [first, second] = props.inputRefs
    ? [{ inputRef: props.inputRefs[0] }, { inputRef: props.inputRefs[1] }]
    : [{ value: props.value, set: props.set }, { value: props.again, set: props.setAgain }];
  return (
    <>
      <TextField label={label} id={ids[0]} name={names?.[0]} type="password" autocomplete="new-password" required={required} {...first} />
      <TextField label={t("setup.again")} id={ids[1]} name={names?.[1]} type="password" autocomplete="new-password" required={required} {...second} />
    </>
  );
}
