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

import type { ComponentChildren, Ref } from "preact";
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
 * draft: name, placeholder, maxLength and pattern (what a valid value
 * looks like) and type (text by default; a password is no stored text)
 * are the input's; the row is wide unless wide is false.
 */
export function TextField({ label, id, wide = true, unit, hint, name, placeholder, type, maxLength, pattern, draft, show }: Row & {
  name: string;
  placeholder?: string;
  type?: "text" | "email" | "url" | "search";
  maxLength?: number;
  pattern?: string;
  draft: Draft<string>;
  show?: (v: string) => string;
}) {
  return (
    <Field label={label} id={id} wide={wide} unit={unit} hint={hint} draft={draft} show={show}>
      <input id={id} type={(type || "text") as "text"} name={name} class={unit ? undefined : "field-control"} autocomplete="off" spellcheck={false}
        placeholder={placeholder} maxLength={maxLength} pattern={pattern} value={draft.value} onInput={(ev) => draft.set(ev.currentTarget.value)} />
    </Field>
  );
}

/**
 * An option of a Select, as fillOptions of components.js takes it: its
 * value, its label and whether it cannot be chosen; or a group of
 * options under a heading.
 */
export type SelectOption = [value: string, label: string, disabled?: boolean] | { group: string; options: [string, string, boolean?][] };

/**
 * Select is a row with a select (selectInput of components.js): its
 * options as fillOptions takes them, the value chosen from a draft of a
 * stored value (with its "changed meanwhile" notice) or from value and
 * onChange; name is the select's, the row narrow unless wide.
 */
export function Select({ label, id, wide, unit, hint, name, options, value, onChange, draft, show }: Row & {
  name?: string;
  options: SelectOption[];
  value?: string;
  onChange?: (v: string) => void;
  draft?: Draft<string>;
  show?: (v: string) => string;
}) {
  const opt = ([v, l, off]: [string, string, boolean?]) => <option key={v} value={v} disabled={off}>{l}</option>;
  return (
    <Field label={label} id={id} wide={wide} unit={unit} hint={hint} draft={draft} show={show}>
      <select id={id} name={name} class={unit ? undefined : "field-control"} value={draft ? draft.value : value} onChange={(ev) => {
        const v = ev.currentTarget.value;
        if (draft) draft.set(v);
        if (onChange) onChange(v);
      }}>
        {options.map((o) => (Array.isArray(o) ? opt(o) : <optgroup key={"group:" + o.group} label={o.group}>{o.options.map(opt)}</optgroup>))}
      </select>
    </Field>
  );
}

/**
 * DateTime is a field of a date and a time (datetime-local), its value
 * as the input takes it ("YYYY-MM-DDTHH:MM"); the API's time is made of
 * it where it is sent. id, cls, step, required and disabled are the
 * input's, ref gives it the focus.
 */
export function DateTime({ value, set, id, cls, step, required, disabled, inputRef }: {
  value: string;
  set: (v: string) => void;
  id?: string;
  cls?: string;
  step?: number;
  required?: boolean;
  disabled?: boolean;
  inputRef?: Ref<HTMLInputElement>;
}) {
  return (
    <input ref={inputRef} type="datetime-local" id={id} class={cls} step={step === undefined ? undefined : String(step)} required={required}
      disabled={disabled} value={value} onInput={(ev) => set(ev.currentTarget.value)} />
  );
}

/**
 * Switch is a checkbox drawn as a switch with its label beside it: on
 * from checked, or from a draft of a stored setting, whose "changed
 * meanwhile" notice follows it; onChange gets its state. It draws its own
 * .field row unless row is false (a switch inside another row, or an item
 * of a menu); cls adds classes to the switch; id, name, disabled and
 * title are the checkbox's; children follow the label (a help button).
 */
export function Switch({ name, label, checked, onChange, draft, show, row = true, cls, id, disabled, title, children }: {
  name?: string;
  label: string;
  checked?: boolean;
  onChange?: (on: boolean) => void;
  draft?: Draft<boolean>;
  show?: (v: boolean) => string;
  row?: boolean;
  cls?: string;
  id?: string;
  disabled?: boolean;
  title?: string;
  children?: ComponentChildren;
}) {
  const sw = (
    <label class={cls ? "switch " + cls : "switch"} title={title}>
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
 * cancel is given (cancelLabel in place of "Cancel"; its class
 * form-cancel, as the plain footer's), and the submit button, submit its
 * label, busy while its request runs (useBusy) and off where off says
 * nothing is to be sent (sendWhenChanged).
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
        {cancel && <button type="button" class="form-cancel" onClick={cancel}>{cancelLabel || t("action.cancel")}</button>}
        {submit && <button type="submit" {...busyProps("primary", !!busy, off)}>{submit}</button>}
      </span>
    </div>
  );
}

/**
 * Stepper is a number field between a − and a + button (stepper of
 * components.js), which change it by step within min and max, rounded to
 * the step; an empty field steps from min. value is what the field shows,
 * set is told of every change, typed or stepped. id and cls are the
 * field's; hidden keeps the whole in its place unseen; children follow the
 * + (a unit). The buttons are named half a degree less and more: the first
 * user of another unit adds their names as a property.
 */
export function Stepper({ value, set, min, max, step, id, cls, hidden, children }: {
  value: string;
  set: (v: string) => void;
  min: number;
  max: number;
  step: number;
  id?: string;
  cls?: string;
  hidden?: boolean;
  children?: ComponentChildren;
}) {
  const by = (d: number) => () => {
    const v = Number(value === "" ? min : value) + d;
    set(String(Math.min(max, Math.max(min, Math.round(v / step) * step))));
  };
  return (
    <span class={hidden ? "inline hidden" : "inline"}>
      <button type="button" class="step small" aria-label={t("temp.less")} onClick={by(-step)}>−</button>
      <input type="number" id={id} class={cls} min={String(min)} max={String(max)} step={String(step)} value={value}
        onInput={(ev) => set(ev.currentTarget.value)} />
      <button type="button" class="step small" aria-label={t("temp.more")} onClick={by(step)}>+</button>
      {children}
    </span>
  );
}
