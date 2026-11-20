// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/field.tsx
 *
 * A form's pieces as parts: the field row (.field, .field.wide for a
 * control on a line of its own) with a setting's help, a text field, a
 * number field, a select and a switch, a field's
 * draft against the value stored on the server (D59-A), and the form's
 * row of actions. A field that shows stored data keeps a draft (rule 1 of
 * AP UiParts): untouched it follows the stored value, changed it keeps
 * what is typed and says under it that the stored value changed
 * meanwhile, with Take theirs; the field draws that notice itself.
 */

import type { ComponentChildren, Ref } from "preact";
import { useId, useRef, useState } from "preact/hooks";
import { Notice } from "./badge";
import type { FormLook, Place } from "./forms";
import { HelpButton, HelpText } from "./help";
import { Icon, Mark } from "./icon";
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
 * draft. Without a place it is the component's state and goes with the
 * component; with one (a form's place, useForm of forms.ts) it is kept
 * there, its base with it, and outlives the component.
 */
export function useDraft<T>(stored: T, norm: (v: T) => unknown = (v) => v, place?: Place<T>): Draft<T> {
  const own = useRef({ base: stored, value: stored, stored });
  const [, redraw] = useState(0);
  const d = place ? place.slot(stored) : own.current;
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
      if (place) place.changed();
      else redraw((n) => n + 1);
    },
    take() {
      d.base = d.stored;
      d.value = d.stored;
      if (place) place.redraw();
      else redraw((n) => n + 1);
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
  /** The key of the setting's help (help.<key> of the texts): its button after the label, its explanation under the control. */
  help?: string;
  /** Classes the row adds to field (a capability's needs-…). */
  cls?: string;
}

/**
 * Field is a label with its control (fieldRow of components.js): the
 * control, given as children, carries the id and, without a unit, the
 * class field-control; a unit follows it, the two then the row's
 * .field-control; help is the key of the setting's help, its button
 * after the label and its explanation under the control (helpParts);
 * hint is a grey line under the row; cls adds classes to the row. With a
 * draft the row is followed by its "changed meanwhile" notice (show
 * names the value).
 */
export function Field<T>({ label, id, wide, unit, hint, help, cls, draft, show, children }: Row & { draft?: Draft<T>; show?: (v: T) => string; children: ComponentChildren }) {
  const helpId = useId();
  return (
    <>
      <div class={"field" + (wide ? " wide" : "") + (cls ? " " + cls : "")}>
        <label for={id}>{label}</label>
        {help && <HelpButton help={help} id={helpId} label={label} />}
        {unit ? (
          <span class="inline field-control">
            {children}
            <span class="muted">{unit}</span>
          </span>
        ) : children}
        {help && <HelpText help={help} id={helpId} />}
        {hint && <p class="muted small field-hint">{hint}</p>}
      </div>
      {draft && <Meanwhile draft={draft} show={show} />}
    </>
  );
}

/**
 * TextField is a row with a text input that shows a stored text, as its
 * draft: name, placeholder, maxLength and pattern (what a valid value
 * looks like), type (text by default; a password is no stored text),
 * required and disabled (off) are the input's; the row is wide unless
 * wide is false.
 */
export function TextField({ label, id, wide = true, unit, hint, help, cls, name, placeholder, type, maxLength, pattern, required, disabled, draft, show }: Row & {
  name: string;
  disabled?: boolean;
  placeholder?: string;
  type?: "text" | "email" | "url" | "search";
  maxLength?: number;
  pattern?: string;
  required?: boolean;
  draft: Draft<string>;
  show?: (v: string) => string;
}) {
  return (
    <Field label={label} id={id} wide={wide} unit={unit} hint={hint} help={help} cls={cls} draft={draft} show={show}>
      <input id={id} type={(type || "text") as "text"} name={name} class={unit ? undefined : "field-control"} autocomplete="off" spellcheck={false}
        placeholder={placeholder} maxLength={maxLength} pattern={pattern} required={required} disabled={disabled} value={draft.value}
        onInput={(ev) => draft.set(ev.currentTarget.value)} />
    </Field>
  );
}

/**
 * NumberField is a number input that shows a stored number, as its
 * draft of the input's text ("" for none; compare with numberNorm): min,
 * max and step are the input's, name, disabled and aria (its name for a
 * screen reader where no label names it) too. It draws its row with its
 * "changed meanwhile" notice, or with row false the input alone, inside
 * a row of the caller's (a field with two controls), whose notice the
 * caller then draws (Meanwhile).
 */
export function NumberField({ label, id, wide, unit, hint, help, cls, name, min, max, step, aria, disabled, draft, show, row = true }: Partial<Row> & {
  name?: string;
  min: number;
  max: number;
  step: number;
  aria?: string;
  disabled?: boolean;
  draft: Draft<string>;
  show?: (v: string) => string;
  row?: boolean;
}) {
  const input = (
    <input id={id} type="number" name={name} class={!row || unit ? undefined : "field-control"} min={String(min)} max={String(max)} step={String(step)}
      aria-label={aria} disabled={disabled} value={draft.value} onInput={(ev) => draft.set(ev.currentTarget.value)} />
  );
  if (!row || label === undefined || id === undefined) return input;
  return (
    <Field label={label} id={id} wide={wide} unit={unit} hint={hint} help={help} cls={cls} draft={draft} show={show}>
      {input}
    </Field>
  );
}

/** numberNorm is how a NumberField's draft compares: as a number, "" for none. */
export function numberNorm(v: string): unknown {
  return v.trim() === "" ? "" : Number(v);
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
 * onChange; name and disabled (off) are the select's, the row narrow
 * unless wide; inputRef gives it the focus. With row false it is the
 * select alone, inside a row of the caller's (a field with two controls),
 * whose notice the caller draws.
 */
export function Select({ label, id, wide, unit, hint, help, cls, name, options, value, onChange, disabled, draft, show, row = true, inputRef }: Partial<Row> & {
  name?: string;
  disabled?: boolean;
  options: SelectOption[];
  value?: string;
  onChange?: (v: string) => void;
  draft?: Draft<string>;
  show?: (v: string) => string;
  row?: boolean;
  inputRef?: Ref<HTMLSelectElement>;
}) {
  const opt = ([v, l, off]: [string, string, boolean?]) => <option key={v} value={v} disabled={off}>{l}</option>;
  const select = (
    <select ref={inputRef} id={id} name={name} class={!row || unit ? undefined : "field-control"} disabled={disabled} value={draft ? draft.value : value} onChange={(ev) => {
      const v = ev.currentTarget.value;
      if (draft) draft.set(v);
      if (onChange) onChange(v);
    }}>
      {options.map((o) => (Array.isArray(o) ? opt(o) : <optgroup key={"group:" + o.group} label={o.group}>{o.options.map(opt)}</optgroup>))}
    </select>
  );
  if (!row || label === undefined || id === undefined) return select;
  return (
    <Field label={label} id={id} wide={wide} unit={unit} hint={hint} help={help} cls={cls} draft={draft} show={show}>
      {select}
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
 * of a menu), with help, the key of the setting's help, its button after
 * the switch and its explanation under it; cls adds classes to the
 * switch; id, name, disabled and title are the checkbox's; children
 * follow the label.
 */
export function Switch({ name, label, checked, onChange, draft, show, row = true, help, cls, id, disabled, title, children }: {
  name?: string;
  label: string;
  checked?: boolean;
  onChange?: (on: boolean) => void;
  draft?: Draft<boolean>;
  show?: (v: boolean) => string;
  row?: boolean;
  help?: string;
  cls?: string;
  id?: string;
  disabled?: boolean;
  title?: string;
  children?: ComponentChildren;
}) {
  const helpId = useId();
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
        {help && <HelpButton help={help} id={helpId} label={label} />}
        {children}
        {help && <HelpText help={help} id={helpId} />}
      </div>
      {draft && <Meanwhile draft={draft} show={show} />}
    </>
  );
}

/** The tooltip of Save's sign, by the form's state. */
const LOOK_TITLE: Partial<Record<FormLook, string>> = { unsaved: "form.unsavedTitle", failed: "form.failedTitle", saved: "form.savedTitle" };

/**
 * LookSign is the sign before Save's text: the warning mark while the
 * form's changes are unsaved, the cross after a failed save, the check
 * after a save; nothing else.
 */
function LookSign({ look }: { look?: FormLook }) {
  if (look === "unsaved") return <Mark state="warn" />;
  if (look === "failed") return <Mark state="bad" />;
  if (look === "saved") return <Icon symbol="icon-check" cls="ni ok-check" />;
  return null;
}

/**
 * FormFooter is a form's row of actions (formFooter of components.js):
 * extra at its left end (resets, Remove), then at its right Reset, where
 * reset is given (off while the form is clean), Cancel, where cancel is
 * given (cancelLabel in place of "Cancel"; its class form-cancel, as the
 * plain footer's), and the submit button, submit its label, busy while
 * its request runs (useBusy) and off where off says nothing is to be
 * sent (sendWhenChanged). Where look gives the form's state (useForm of
 * forms.ts), Save shows it: clean, plain and off; changed, primary;
 * unsaved, primary with the warning mark before its text; saving, busy;
 * failed, with the cross; saved, plain and off with the check. Without
 * look it is primary, as it always was.
 */
export function FormFooter({ submit, busy, off, cancel, cancelLabel, reset, look, extra }: {
  submit?: string;
  busy?: boolean;
  off?: boolean;
  cancel?: () => void;
  cancelLabel?: string;
  reset?: () => void;
  look?: FormLook;
  extra?: ComponentChildren;
}) {
  const clean = look === "clean" || look === "saved";
  const turning = !!busy || look === "saving";
  const title = look && LOOK_TITLE[look];
  return (
    <div class="actions">
      {extra && <span class="actions-left">{extra}</span>}
      <span class="actions-right">
        {reset && <button type="button" class="form-reset" disabled={clean || turning} onClick={reset}>{t("action.reset")}</button>}
        {cancel && <button type="button" class="form-cancel" onClick={cancel}>{cancelLabel || t("action.cancel")}</button>}
        {submit && (
          <button type="submit" {...busyProps(clean ? "" : "primary", turning, off || clean)} title={title ? t(title) : undefined}>
            <LookSign look={look} />
            {submit}
          </button>
        )}
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
