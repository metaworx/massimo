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
 * meanwhile, with Take theirs; the field draws that notice itself. A
 * free field, which shows no stored data (a time, a name offered), takes
 * its value and its set instead. A field's limit says what it takes
 * while a value is refused (Limit).
 */

import type { ComponentChildren, Ref } from "preact";
import { useRef, useState } from "preact/hooks";
import { Notice } from "./badge";
import type { FormLook, Place } from "./forms";
import { useHelp } from "./help";
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

/**
 * A field's limit: text says what the
 * field takes ("only 1 to 255", "longer than 32 bytes"), shown in amber
 * before the box while over says that the value is refused, the box then
 * outlined in red and the limit its description; cls is the line's class
 * beside the fixed field-limit.
 */
export interface Limit {
  text: string;
  over: boolean;
  cls?: string;
}

/** limitAria are the attributes of a control whose id is id while its limit is broken, refused and described by the limit: TextField's and NumberField's. */
function limitAria(id: string | undefined, limit: Limit | undefined) {
  if (!limit?.over || id === undefined) return {};
  return { "aria-invalid": "true" as const, "aria-describedby": id + "-limit" };
}

/** describedAria are limitAria's attributes with describedBy, the ids of what else describes the control (a note the caller draws under the field), added to its description. */
function describedAria(id: string | undefined, limit: Limit | undefined, describedBy: string | undefined): { "aria-invalid"?: "true"; "aria-describedby"?: string } {
  const a: { "aria-invalid"?: "true"; "aria-describedby"?: string } = limitAria(id, limit);
  const ids = [a["aria-describedby"], describedBy].filter(Boolean).join(" ");
  return ids ? { ...a, "aria-describedby": ids } : a;
}

/** LimitLine is the line of a field's limit, of the id id + "-limit", hidden in its place while the value is in it. */
function LimitLine({ id, limit }: { id: string; limit: Limit }) {
  return (
    <span id={id + "-limit"} class={"small warn-text field-limit" + (limit.cls ? " " + limit.cls : "") + (limit.over ? "" : " hidden")} role="status">
      {limit.text}
    </span>
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
  /** The hint's id, which the control names as its description (a note that changes with the choice: the update channel's). */
  hintId?: string;
  /** The key of the setting's help (help.<key> of the texts): its button after the label, its explanation under the control. */
  help?: string;
  /** Classes the row adds to field (a capability's needs-…). */
  cls?: string;
  /** The label's class: action-title for a row's title in bold (Maintenance's settings). */
  labelCls?: string;
  /** The ids of a note the caller draws beside the row that describes the control (a target's field warned of), its TextField's, NumberField's and Select's description. */
  describedBy?: string;
  /** The field's limit, its line before the control (limitAria gives the control its attributes). */
  limit?: Limit;
}

/**
 * Field is a label with its control (fieldRow of components.js): the
 * control, given as children, carries the id and, without a unit, the
 * class field-control; a unit follows it, the two then the row's
 * .field-control; help is the key of the setting's help, its button
 * after the label and its explanation under the control (helpParts);
 * hint is a grey line under the row, hintId its id; cls adds classes to the row,
 * labelCls to its label (action-title, a row's title in bold); limit
 * is the line of what the field takes, just before the control, in the
 * unit's line where there is one, else on a line of its own above the
 * box. With a draft the row is followed by its "changed meanwhile"
 * notice (show names the value).
 */
export function Field<T>({ label, id, wide, unit, hint, hintId, help, cls, labelCls, limit, draft, show, children }: Row & { draft?: Draft<T>; show?: (v: T) => string; children: ComponentChildren }) {
  const h = useHelp(help || "", label);
  const line = limit && <LimitLine id={id} limit={limit} />;
  return (
    <>
      <div class={"field" + (wide ? " wide" : "") + (cls ? " " + cls : "")}>
        <label for={id} class={labelCls}>{label}</label>
        {help && h.button}
        {unit ? (
          <span class="inline field-control">
            {line}
            {children}
            <span class="muted">{unit}</span>
          </span>
        ) : <>{line}{children}</>}
        {help && h.text}
        {hint && <p id={hintId} class="muted small field-hint">{hint}</p>}
      </div>
      {draft && <Meanwhile draft={draft} show={show} />}
    </>
  );
}

/**
 * A field's value: from a draft of a stored value, with its "changed
 * meanwhile" notice, or for a free field, which shows no stored data,
 * its value and set, told of every change (a part's own state).
 */
type Value<T> = { draft: Draft<T>; value?: undefined; set?: undefined } | { draft?: undefined; value: T; set: (v: T) => void };

/**
 * A text field's value: as Value, or not controlled at all, a field no
 * drawing writes, read through its inputRef at the submit (the sign-in
 * card's fields, UiParts D210, so that a password manager's fill stays).
 */
type TextValue = Value<string> | { draft?: undefined; value?: undefined; set?: undefined; inputRef: Ref<HTMLInputElement> };

/**
 * TextField is a row with a text input that shows a stored text, as its
 * draft, or a free text by value and set: name, placeholder, maxLength
 * and pattern (what a valid value looks like), type (text by default; a
 * password is no stored text), required and disabled (off) are the
 * input's, inputRef gives it the focus; limit marks a value refused (a
 * name too long), and the row is wide unless wide is false. A password
 * (type password, autocomplete new-password or current-password) is no
 * stored text: it is a free field by value and set, or a field not
 * controlled read through inputRef, never a draft, so that it is kept by
 * its form's part alone; inputCls is the input's class
 * beside field-control (mono for a key). With row false it is the input
 * alone, standing in a tile's head (the log's search), as plain as it is
 * written: aria its name for a screen reader, as NumberField's and
 * Select's, no name, autocomplete or spellcheck of its own. Without a
 * draft, a value and a set it is not controlled: no drawing writes what
 * it holds, and its caller reads it through inputRef.
 */
export function TextField({ label, id, wide = true, unit, hint, help, cls, limit, describedBy, name, placeholder, type, autocomplete, inputCls, maxLength, pattern, required, disabled, draft, value, set, show, inputRef, row = true, aria }: Omit<Row, "label"> & TextValue & {
  label?: string;
  row?: boolean;
  aria?: string;
  name?: string;
  disabled?: boolean;
  placeholder?: string;
  type?: "text" | "email" | "url" | "search" | "password";
  autocomplete?: string;
  inputCls?: string;
  maxLength?: number;
  pattern?: string;
  required?: boolean;
  show?: (v: string) => string;
  inputRef?: Ref<HTMLInputElement>;
}) {
  const input = (
    <input ref={inputRef} id={id} type={(type || "text") as "text"} name={name} class={[unit || !row ? "" : "field-control", inputCls || ""].filter(Boolean).join(" ") || undefined}
      autocomplete={row ? autocomplete || "off" : autocomplete} spellcheck={row ? false : undefined} aria-label={aria}
      placeholder={placeholder} maxLength={maxLength} pattern={pattern} required={required} disabled={disabled} value={draft ? draft.value : value}
      {...describedAria(id, limit, describedBy)} onInput={(ev) => (draft ? draft.set(ev.currentTarget.value) : set?.(ev.currentTarget.value))} />
  );
  if (!row) return input;
  return (
    <Field label={label || ""} id={id} wide={wide} unit={unit} hint={hint} help={help} cls={cls} limit={limit} draft={draft} show={show}>
      {input}
    </Field>
  );
}

/**
 * NumberField is a number input that shows a stored number, as its
 * draft of the input's text ("" for none; compare with numberNorm), or a
 * free number by value and set: min, max and step are the input's, name,
 * disabled and aria (its name for a screen reader where no label names
 * it) too, inputMode the keyboard a phone offers (numeric, for a whole
 * number: the reboot's delay), inputRef gives it the focus; limit marks
 * a value refused (one
 * out of range); onChange is told of the value as the field is left
 * changed (the change event: My settings' poll, saved then). It draws its row with its "changed meanwhile" notice,
 * or with row false the input alone, inside a row of the caller's (a
 * field with two controls), whose notice the caller then draws
 * (Meanwhile).
 */
export function NumberField({ label, id, wide, unit, hint, help, cls, limit, describedBy, name, min, max, step, aria, disabled, draft, value, set, show, row = true, inputRef, inputMode, onChange }: Partial<Row> & Value<string> & {
  name?: string;
  onChange?: (v: string) => void;
  inputMode?: "numeric" | "decimal";
  min: number;
  max: number;
  step: number;
  aria?: string;
  disabled?: boolean;
  show?: (v: string) => string;
  row?: boolean;
  inputRef?: Ref<HTMLInputElement>;
}) {
  const input = (
    <input ref={inputRef} id={id} type="number" name={name} class={!row || unit ? undefined : "field-control"} min={String(min)} max={String(max)}
      step={String(step)} inputMode={inputMode} aria-label={aria} disabled={disabled} value={draft ? draft.value : value} {...describedAria(id, limit, describedBy)}
      onInput={(ev) => (draft ? draft.set(ev.currentTarget.value) : set(ev.currentTarget.value))}
      onChange={onChange && ((ev) => onChange(ev.currentTarget.value))} />
  );
  if (!row || label === undefined || id === undefined) return input;
  return (
    <Field label={label} id={id} wide={wide} unit={unit} hint={hint} help={help} cls={cls} limit={limit} draft={draft} show={show}>
      {input}
    </Field>
  );
}

/** numberNorm is how a NumberField's draft compares: as a number, "" for none. */
export function numberNorm(v: string): unknown {
  return v.trim() === "" ? "" : Number(v);
}

/**
 * An option of a Select: its
 * value, its label and whether it cannot be chosen; or a group of
 * options under a heading.
 */
export type SelectOption = [value: string, label: string, disabled?: boolean] | { group: string; options: [string, string, boolean?][] };

/**
 * Select is a row with a select: its
 * options (SelectOption), the value chosen from a draft of a
 * stored value (with its "changed meanwhile" notice) or from value and
 * onChange; name and disabled (off) are the select's, the row narrow
 * unless wide; inputRef gives it the focus; hintId names the hint as its
 * description; before stands between the label with its help and the
 * select (a drawing's preview: the Appearance card's icons), after
 * follows the select in its row (the language's note of a machine
 * translation). With row false it is the select alone,
 * inside a row of the caller's (a field with two controls), whose notice
 * the caller draws, or standing alone in a tile's head (the log's
 * choices): aria is then its name for a screen reader, selectCls its own
 * class (hidden, for a choice left out).
 */
export function Select({ label, id, wide, unit, hint, hintId, help, cls, labelCls, describedBy, name, options, value, onChange, disabled, draft, show, row = true, inputRef, before, after, aria, selectCls }: Partial<Row> & {
  aria?: string;
  selectCls?: string;
  before?: ComponentChildren;
  after?: ComponentChildren;
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
    <select ref={inputRef} id={id} name={name} class={!row ? selectCls : unit ? undefined : "field-control"} disabled={disabled} aria-label={aria} aria-describedby={[hint ? hintId : "", describedBy || ""].filter(Boolean).join(" ") || undefined}
      value={draft ? draft.value : value} onChange={(ev) => {
      const v = ev.currentTarget.value;
      if (draft) draft.set(v);
      if (onChange) onChange(v);
    }}>
      {options.map((o) => (Array.isArray(o) ? opt(o) : <optgroup key={"group:" + o.group} label={o.group}>{o.options.map(opt)}</optgroup>))}
    </select>
  );
  if (!row || label === undefined || id === undefined) return select;
  return (
    <Field label={label} id={id} wide={wide} unit={unit} hint={hint} hintId={hintId} help={help} cls={cls} labelCls={labelCls} draft={draft} show={show}>
      {before}
      {select}
      {after}
    </Field>
  );
}

/**
 * DateTime is a field of a date and a time (datetime-local), its value
 * as the input takes it ("YYYY-MM-DDTHH:MM"); the API's time is made of
 * it where it is sent. id, cls, step, required and disabled are the
 * input's, ref gives it the focus; limit, with an id, says what it takes
 * in its line just before it while a value is refused (an end that has
 * passed), the field then marked and described by it, as a number
 * field's.
 */
export function DateTime({ value, set, id, cls, step, required, disabled, inputRef, limit }: {
  value: string;
  set: (v: string) => void;
  id?: string;
  cls?: string;
  step?: number;
  required?: boolean;
  disabled?: boolean;
  inputRef?: Ref<HTMLInputElement>;
  limit?: Limit;
}) {
  const input = (
    <input ref={inputRef} type="datetime-local" id={id} class={cls} step={step === undefined ? undefined : String(step)} required={required}
      disabled={disabled} value={value} {...limitAria(id, limit)} onInput={(ev) => set(ev.currentTarget.value)} />
  );
  if (!limit || id === undefined) return input;
  return <><LimitLine id={id} limit={limit} />{input}</>;
}

/**
 * Switch is a checkbox drawn as a switch with its label beside it: on
 * from checked, or from a draft of a stored setting, whose "changed
 * meanwhile" notice follows it; onChange gets its state. It draws its own
 * .field row unless row is false (a switch inside another row, or an item
 * of a menu), with help, the key of the setting's help, its button after
 * the switch and its explanation under it, and rowCls, classes the row
 * adds (a capability's needs-…); cls adds classes to the switch; id,
 * name, disabled and title are the checkbox's; aria names it where its
 * label is empty, which draws no text (a switch in a column headed for
 * it: My settings' "only on this device"), data are the checkbox's data-
 * attributes; badge is drawn inside the switch after its label (a
 * capability's state), children follow the switch.
 */
export function Switch({ name, label, checked, onChange, draft, show, row = true, help, rowCls, cls, id, disabled, title, aria, data, badge, children }: {
  name?: string;
  label: string;
  checked?: boolean;
  onChange?: (on: boolean) => void;
  draft?: Draft<boolean>;
  show?: (v: boolean) => string;
  row?: boolean;
  help?: string;
  rowCls?: string;
  cls?: string;
  id?: string;
  disabled?: boolean;
  title?: string;
  aria?: string;
  data?: Record<string, string>;
  badge?: ComponentChildren;
  children?: ComponentChildren;
}) {
  const h = useHelp(help || "", label);
  const attrs: Record<string, string> = {};
  for (const [k, v] of Object.entries(data || {})) attrs["data-" + k] = v;
  const sw = (
    <label class={cls ? "switch " + cls : "switch"} title={title}>
      <input id={id} type="checkbox" name={name} disabled={disabled} aria-label={aria} {...attrs} checked={draft ? draft.value : !!checked} onChange={(ev) => {
        const on = ev.currentTarget.checked;
        if (draft) draft.set(on);
        if (onChange) onChange(on);
      }} />
      <span class="slider" aria-hidden="true" />
      {label && <span class="switch-label">{label}</span>}
      {badge}
    </label>
  );
  if (!row) return <>{sw}{children}</>;
  return (
    <>
      <div class={rowCls ? "field " + rowCls : "field"}>
        {sw}
        {help && h.button}
        {children}
        {help && h.text}
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
 * SaveButton is a form's Save by its state (useForm's look), as
 * FormFooter draws it: clean, plain and off; changed, primary; unsaved,
 * primary with the warning mark before its text; saving, busy; failed,
 * with the cross; saved, plain and off with the check; without look,
 * primary. label is its text, busy and off as FormFooter's; type is
 * submit unless given, onClick runs where it is a button (the scheme
 * editor's Save as, inline with the name it saves under).
 */
export function SaveButton({ label, look, busy, off, type, onClick }: {
  label: string;
  look?: FormLook;
  busy?: boolean;
  off?: boolean;
  type?: "submit" | "button";
  onClick?: () => void;
}) {
  const clean = look === "clean" || look === "saved";
  const turning = !!busy || look === "saving";
  const title = look && LOOK_TITLE[look];
  return (
    <button type={type || "submit"} {...busyProps(clean ? "" : "primary", turning, off || clean)} title={title ? t(title) : undefined} onClick={onClick}>
      <LookSign look={look} />
      {label}
    </button>
  );
}

/**
 * FormFooter is a form's row of actions (formFooter of components.js):
 * extra at its left end (resets, Remove), then at its right Reset, where
 * reset is given (off while the form is clean), Cancel, where cancel is
 * given (cancelLabel in place of "Cancel"; its class form-cancel, as the
 * plain footer's), and the submit button, submit its label, busy while
 * its request runs (useBusy) and off where off says nothing is to be
 * sent. Where look gives the form's state (useForm of
 * forms.ts), Save shows it: clean, plain and off; changed, primary;
 * unsaved, primary with the warning mark before its text; saving, busy;
 * failed, with the cross; saved, plain and off with the check. Without
 * look it is primary, as it always was. after follows the submit button
 * (My profile's ⓘ behind a group's last button).
 */
export function FormFooter({ submit, busy, off, cancel, cancelLabel, reset, look, extra, after }: {
  submit?: string;
  busy?: boolean;
  off?: boolean;
  cancel?: () => void;
  cancelLabel?: string;
  reset?: () => void;
  look?: FormLook;
  extra?: ComponentChildren;
  after?: ComponentChildren;
}) {
  const clean = look === "clean" || look === "saved";
  const turning = !!busy || look === "saving";
  return (
    <div class="actions">
      {extra && <span class="actions-left">{extra}</span>}
      <span class="actions-right">
        {reset && <button type="button" class="form-reset" disabled={clean || turning} onClick={reset}>{t("action.reset")}</button>}
        {cancel && <button type="button" class="form-cancel" onClick={cancel}>{cancelLabel || t("action.cancel")}</button>}
        {submit && <SaveButton label={submit} look={look} busy={busy} off={off} />}
        {after}
      </span>
    </div>
  );
}

/**
 * Stepper is a number field between a − and a + button, which change it by step within min and max, rounded to
 * the step; an empty field steps from min. value is what the field shows,
 * set is told of every change, typed or stepped. id and cls are the
 * field's; hidden keeps the whole in its place unseen; disabled turns the
 * field and its buttons off, and leaves the field out of its form's check,
 * so that one hidden with a value off its step does not stop a submit;
 * children follow the + (a unit). The buttons are named half a degree
 * less and more: the first user of another unit adds their names as a
 * property.
 */
export function Stepper({ value, set, min, max, step, id, cls, hidden, disabled, children }: {
  value: string;
  set: (v: string) => void;
  min: number;
  max: number;
  step: number;
  id?: string;
  cls?: string;
  hidden?: boolean;
  disabled?: boolean;
  children?: ComponentChildren;
}) {
  const by = (d: number) => () => {
    const v = Number(value === "" ? min : value) + d;
    set(String(Math.min(max, Math.max(min, Math.round(v / step) * step))));
  };
  return (
    <span class={hidden ? "inline hidden" : "inline"}>
      <button type="button" class="step small" aria-label={t("temp.less")} disabled={disabled} onClick={by(-step)}>−</button>
      <input type="number" id={id} class={cls} min={String(min)} max={String(max)} step={String(step)} disabled={disabled} value={value}
        onInput={(ev) => set(ev.currentTarget.value)} />
      <button type="button" class="step small" aria-label={t("temp.more")} disabled={disabled} onClick={by(step)}>+</button>
      {children}
    </span>
  );
}
