// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/field.tsx
 *
 * A form's pieces as parts: the field row (.field, .field.wide for a
 * control on a line of its own), a text field and a switch, a field's
 * draft against the value stored on the server (D59-A), and the form's
 * row of actions. A field that shows stored data keeps a draft (rule 1 of
 * AP UiParts): untouched it follows the stored value, changed it keeps
 * what is typed and says under it that the stored value changed
 * meanwhile, with Take theirs; the field draws that notice itself.
 */

import type { ComponentChildren } from "preact";
import { useRef, useState } from "preact/hooks";
import { Notice } from "./badge";
import { busyProps } from "./question";

/** A field's draft against its stored value (useDraft). */
export interface Draft<T> {
  /** What the field shows: the stored value, or what the user made of it. */
  value: T;
  /** The value stored now, as the last load gave it. */
  stored: T;
  /** The value differs from the one stored now: a save would send it. */
  changed: boolean;
  /** The stored value changed under a draft: the field says so. */
  meanwhile: boolean;
  /** The user's change. */
  set(v: T): void;
  /** Takes the value stored now, dropping the draft. */
  take(): void;
}

/**
 * useDraft keeps a field's draft against stored, the value on the server
 * as the last load gave it; norm is how two values compare (a trimmed
 * text, a serial in capitals, a list as a set). An untouched draft follows
 * stored; one the user changed keeps its value, and meanwhile says that
 * stored moved under it, until take() or a stored value equal to the
 * draft. It is the component's state: it goes with the component.
 */
export function useDraft<T>(stored: T, norm: (v: T) => unknown = (v) => v): Draft<T> {
  const r = useRef({ base: stored, value: stored, stored });
  const [, redraw] = useState(0);
  const d = r.current;
  d.stored = stored; // the latest, for take() called after a later draw
  const same = (a: T, b: T) => Object.is(norm(a), norm(b));
  if (!same(stored, d.base) && (same(d.value, d.base) || same(d.value, stored))) {
    d.base = stored;
    d.value = stored;
  }
  return {
    value: d.value,
    stored,
    changed: !same(d.value, stored),
    meanwhile: !same(stored, d.base),
    set(v) {
      d.value = v;
      redraw((n) => n + 1);
    },
    take() {
      d.base = d.stored;
      d.value = d.stored;
      redraw((n) => n + 1);
    },
  };
}

/** shown is a stored value as the notice names it by default: a list joined, else its text; a region passes its own show for more. */
function shown(v: unknown): string {
  return Array.isArray(v) ? v.join(", ") : String(v ?? "");
}

/**
 * Meanwhile says under a field that the stored value changed while it
 * was being edited, naming the new value, with Take theirs; nothing while
 * the draft has not met a change. show is how the value is named.
 */
export function Meanwhile<T>({ draft, show }: { draft: Draft<T>; show?: (v: T) => string }) {
  if (!draft.meanwhile) return null;
  return (
    <div class="field-meanwhile">
      <Notice text={t("field.changedMeanwhile", { value: (show || shown)(draft.stored) })} />
      <button type="button" class="take-theirs" onClick={draft.take}>{t("field.takeTheirs")}</button>
    </div>
  );
}

/** The properties a field row has beside its control. */
interface Row {
  /** The label. */
  label: string;
  /** The control's id, which the label is for. */
  id: string;
  /** The control on a line of its own. */
  wide?: boolean;
  /** A unit after the control. */
  unit?: string;
  /** A grey line under the row. */
  hint?: string;
}

/**
 * Field is a label with its control (fieldRow of components.js): the
 * control, given as children, carries the id and, without a unit, the
 * class field-control; a unit follows it, the two then the row's
 * .field-control; hint is a grey line under the row. With a draft the
 * row is followed by its "changed meanwhile" notice (show names the
 * value). A help button (helpParts) is not covered yet.
 */
export function Field<T>({ label, id, wide, unit, hint, draft, show, children }: Row & { draft?: Draft<T>; show?: (v: T) => string; children: ComponentChildren }) {
  return (
    <>
      <div class={wide ? "field wide" : "field"}>
        <label for={id}>{label}</label>
        {unit ? (
          <span class="inline field-control">
            {children}
            <span class="muted">{unit}</span>
          </span>
        ) : children}
        {hint && <p class="muted small field-hint">{hint}</p>}
      </div>
      {draft && <Meanwhile draft={draft} show={show} />}
    </>
  );
}

/**
 * TextField is a row with a text input that shows a stored text, as its
 * draft: name, placeholder and type (text by default; a password is no
 * stored text) are the input's;
 * the row is wide unless wide is false.
 */
export function TextField({ label, id, wide = true, unit, hint, name, placeholder, type, maxLength, draft, show }: Row & {
  name: string;
  placeholder?: string;
  type?: "text" | "email" | "url" | "search";
  maxLength?: number;
  draft: Draft<string>;
  show?: (v: string) => string;
}) {
  return (
    <Field label={label} id={id} wide={wide} unit={unit} hint={hint} draft={draft} show={show}>
      <input id={id} type={(type || "text") as "text"} name={name} class={unit ? undefined : "field-control"} autocomplete="off" spellcheck={false}
        placeholder={placeholder} maxLength={maxLength} value={draft.value} onInput={(ev) => draft.set(ev.currentTarget.value)} />
    </Field>
  );
}

/**
 * Switch is a checkbox drawn as a switch with its label beside it: on
 * from checked, or from a draft of a stored setting, whose "changed
 * meanwhile" notice follows it; onChange gets its state. It draws its own
 * .field row unless row is false (a switch inside another row); id,
 * disabled and title are the checkbox's; children follow the label (a
 * help button).
 */
export function Switch({ name, label, checked, onChange, draft, show, row = true, id, disabled, title, children }: {
  name: string;
  label: string;
  checked?: boolean;
  onChange?: (on: boolean) => void;
  draft?: Draft<boolean>;
  show?: (v: boolean) => string;
  row?: boolean;
  id?: string;
  disabled?: boolean;
  title?: string;
  children?: ComponentChildren;
}) {
  const sw = (
    <label class="switch" title={title}>
      <input id={id} type="checkbox" name={name} disabled={disabled} checked={draft ? draft.value : !!checked} onChange={(ev) => {
        const on = ev.currentTarget.checked;
        if (draft) draft.set(on);
        if (onChange) onChange(on);
      }} />
      <span class="slider" aria-hidden="true" />
      <span class="switch-label">{label}</span>
    </label>
  );
  if (!row) return <>{sw}{children}</>;
  return (
    <>
      <div class="field">
        {sw}
        {children}
      </div>
      {draft && <Meanwhile draft={draft} show={show} />}
    </>
  );
}

/**
 * FormFooter is a form's row of actions (formFooter of components.js):
 * extra at its left end (resets, Remove), then at its right Cancel, where
 * cancel is given (cancelLabel in place of "Cancel"), and the submit
 * button, submit its label, busy while its request runs (useBusy) and off
 * where off says nothing is to be sent (sendWhenChanged).
 */
export function FormFooter({ submit, busy, off, cancel, cancelLabel, extra }: {
  submit?: string;
  busy?: boolean;
  off?: boolean;
  cancel?: () => void;
  cancelLabel?: string;
  extra?: ComponentChildren;
}) {
  return (
    <div class="actions">
      {extra && <span class="actions-left">{extra}</span>}
      <span class="actions-right">
        {cancel && <button type="button" onClick={cancel}>{cancelLabel || t("action.cancel")}</button>}
        {submit && <button type="submit" {...busyProps("primary", !!busy, off)}>{submit}</button>}
      </span>
    </div>
  );
}
