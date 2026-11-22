// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/forms.ts
 *
 * A settings form that keeps its changes until they are saved or reset
 * (AP UnsavedMarks): the drafts of its fields kept by the form's key,
 * beyond the part that draws them, so that what is typed outlives the
 * form's closing and its being drawn in another place; the form's state,
 * which its Save shows (FormFooter); and the forms with changes not
 * saved as a signal, of which a problem list near each shows the unsaved
 * and the failed ones as an item that opens the form. A form is changed
 * while a draft differs from its stored value; unsaved once the focus has
 * left it, it has been taken away, or a minute has passed since the last
 * change; failed after a save that did not go through, until one does, a
 * Reset, or nothing left to send; saved for as long as a result line
 * shows a success (RESULT_FADE_MS). A region chooses the keys, such as
 * "<kind>:<id>", and forgets the forms of what is gone; the page forgets
 * them all at a sign-out. Every form with changes, the merely changed
 * among them, is what leaving the page would drop: the page asks for
 * them, and an attempt to leave counts each changed form as left.
 */

import { signal, type Signal } from "@preact/signals";
import { useEffect, useLayoutEffect, useRef } from "preact/hooks";
import { fader } from "./action";
import type { Item } from "./problems";

/**
 * How long after the last change a changed form is marked unsaved, in
 * ms. A test shortens it with Playwright's clock (tools/ui-test,
 * test_unsaved.py), not with a setting.
 */
const UNSAVED_AFTER_MS = 60_000;

/** A form's state as its Save shows it. */
export type FormLook = "clean" | "changed" | "unsaved" | "saving" | "failed" | "saved";

/** The text that names a form, a text key with its arguments, said in the page's language when it is shown. */
export interface FormName {
  key: string;
  args?: Record<string, string>;
}

/**
 * A form with changes not saved, as the registry lists it: its key, its
 * name, its state (changed, also while it saves; unsaved; failed) and how
 * to open it.
 */
interface Pending {
  key: string;
  name: FormName;
  state: "changed" | "unsaved" | "failed";
  open: () => void;
}

/** A field's draft kept in its form's store: the value shown, the stored value it started from (base) and the stored value now. */
interface Slot<T> {
  base: T;
  value: T;
  stored: T;
}

/** Where a field keeps its draft (useDraft): its slot, and the form told of a change. */
export interface Place<T> {
  /** slot is the field's draft, made from stored the first time. */
  slot(stored: T): Slot<T>;
  /**
   * known is the stored value the field's draft holds now, undefined
   * before it has one: a form whose stored values load later draws its
   * drafts from it at once, before they have come again.
   */
  known(): T | undefined;
  /** changed tells the form of the user's change. */
  changed(): void;
  /** redraw draws the form again, after a draft was taken. */
  redraw(): void;
}

/** What a form part hands its store at each drawing, for its Reset and its save. */
interface Taker {
  take(): void;
}

/** A form's record, kept by its key across its parts. */
interface FormRecord {
  key: string;
  slots: Map<string, Slot<unknown>>;
  /** Counts the changes of its drafts: the form's part reads it, so that it is drawn again. */
  ver: Signal<number>;
  /** "", unsaved, saving, failed or saved; with changed, the look. */
  phase: Signal<string>;
  /** A draft differs from its stored value, as the part last drew it. */
  changed: boolean;
  drafts: Taker[];
  name?: FormName;
  /** How its part last said to open it. */
  open?: () => void;
  /** The registry's open, the same function for the record's life, which calls the latest open. */
  opener: () => void;
  timer?: ReturnType<typeof setTimeout>;
  /** The form is to take the focus once it is shown (focusForm). */
  focus: boolean;
  /** The saved check, which fades as a result line does (fader). */
  said?: (res: { text: string; ok: boolean } | null) => void;
}

const records = new Map<string, FormRecord>();

/** The forms with changes not saved, changed, unsaved or failed, in the order they came. */
const pending = signal<Pending[]>([]);

/** under says whether a form's key k is key or one under it, key + ":…". */
function under(k: string, key: string): boolean {
  return k === key || k.startsWith(key + ":");
}

/** recordOf is the form's record, made the first time its key is asked for. */
function recordOf(key: string): FormRecord {
  let r = records.get(key);
  if (!r) {
    const made: FormRecord = { key, slots: new Map(), ver: signal(0), phase: signal(""), changed: false, drafts: [], focus: false, opener: () => made.open?.() };
    r = made;
    records.set(key, r);
  }
  return r;
}

/** lookOf is a form's state as its Save shows it. */
function lookOf(r: FormRecord): FormLook {
  const p = r.phase.peek();
  if (p === "saving") return "saving";
  if (!r.changed) return p === "saved" ? "saved" : "clean";
  if (p === "failed" || p === "unsaved") return p;
  return "changed";
}

/** publish lists the forms with changes not saved again, where the list differs by a key, a state or a name. */
function publish(): void {
  const list: Pending[] = [];
  for (const r of records.values()) {
    const look = lookOf(r);
    if (!r.name || look === "clean" || look === "saved") continue;
    list.push({ key: r.key, name: r.name, state: look === "unsaved" || look === "failed" ? look : "changed", open: r.opener });
  }
  const was = pending.peek();
  const same = was.length === list.length && was.every((u, i) => u.key === list[i].key && u.state === list[i].state
    && JSON.stringify(u.name) === JSON.stringify(list[i].name));
  if (!same) pending.value = list;
}

/** setPhase sets a form's phase and lists the unsaved forms again. */
function setPhase(r: FormRecord, p: string): void {
  r.phase.value = p;
  publish();
}

/** left marks a changed form unsaved: the focus went elsewhere, it was taken away, or a minute passed; a failed one stays failed. */
function left(r: FormRecord): void {
  const p = r.phase.peek();
  if (r.changed && (p === "" || p === "saved")) setPhase(r, "unsaved");
}

/** touched is the user's change: back to changed from unsaved, and the minute starts again. */
function touched(r: FormRecord): void {
  if (r.phase.peek() === "unsaved" || r.phase.peek() === "saved") r.phase.value = "";
  clearTimeout(r.timer);
  r.timer = setTimeout(() => left(r), UNSAVED_AFTER_MS);
  r.ver.value++;
  publish();
}

/**
 * forgetForms forgets the form of key and those whose key begins with
 * key + ":", their drafts and their marks: what they belong to is gone (a
 * Cube removed); with "" every form, at a sign-out. A form still drawn
 * shows its stored values again.
 */
export function forgetForms(key: string): void {
  for (const [k, r] of [...records]) {
    if (key && !under(k, key)) continue;
    clearTimeout(r.timer);
    records.delete(k);
    r.ver.value++;
  }
  publish();
}

/**
 * focusForm puts the focus into the form of key once it is shown: its
 * first field that is on. The region shows it first (opens its panel,
 * shows its page).
 */
export function focusForm(key: string): void {
  const r = recordOf(key);
  r.focus = true;
  r.ver.value++;
}

/**
 * hasChanges says whether the form of key, or one under it, has changes
 * not saved: changed, saving, unsaved or failed; not once it is saved or
 * reset. A part shows what holds such a form while it has them (a row
 * otherwise for expert mode alone).
 */
export function hasChanges(key: string): boolean {
  return pending.value.some((u) => under(u.key, key));
}

/**
 * unsavedItems are the items of a problem list for the unsaved and the
 * failed forms whose key is key or begins with key + ":", not those
 * merely changed: the disk, amber while unsaved, red after a failed save,
 * its text naming the form, its click opening it.
 */
export function unsavedItems(key: string): Item[] {
  return pending.value.filter((u) => u.state !== "changed" && under(u.key, key)).map(itemOf);
}

/**
 * unsavedTone is the worse state of the unsaved and the failed forms
 * whose key is one of keys or begins with one + ":", as their disks
 * show it: "bad" where a save failed, else "warn" where one is unsaved,
 * "" where none is; the merely changed count for nothing, as in
 * unsavedItems. A part that reads it is drawn again when it changes.
 */
export function unsavedTone(keys: string[]): "" | "warn" | "bad" {
  let tone: "" | "warn" | "bad" = "";
  for (const u of pending.value) {
    if (u.state === "changed" || !keys.some((k) => under(u.key, k))) continue;
    if (u.state === "failed") return "bad";
    tone = "warn";
  }
  return tone;
}

/**
 * changedItems are the items of every form with changes not saved, the
 * merely changed ones and those saving too, as unsavedItems draws them:
 * what leaving the page would drop. A part that reads them is drawn
 * again when they change.
 */
export function changedItems(): Item[] {
  return pending.value.map(itemOf);
}

/** itemOf is a form's item in a list: the disk, red after a failed save, else amber, its text naming the form, its click opening it. */
function itemOf(u: Pending): Item {
  const name = t(u.name.key, u.name.args || {});
  return {
    kind: "unsaved",
    icon: "icon-disk",
    sign: "disk",
    tone: u.state === "failed" ? "bad" : "warn",
    text: t(u.state === "failed" ? "form.failedItem" : "form.unsavedItem", { form: name }),
    action: u.open,
  };
}

/**
 * leaveForms counts every changed form as left, as the focus leaving it
 * does: an attempt to leave the page, so that a form still being typed
 * in is unsaved, with its sign and its disk, where the user stays.
 */
export function leaveForms(): void {
  for (const r of records.values()) left(r);
}

/** What a form part gets from useForm. */
export interface Form {
  /** The form's state, which FormFooter's Save shows, read after track. */
  readonly look: FormLook;
  /** The props of the form element: its ref, and its focus leaving it. */
  props: {
    ref: { current: HTMLFormElement | null };
    onFocusOut: (ev: FocusEvent) => void;
    onPointerDown: () => void;
  };
  /** place is where a field of the form keeps its draft (useDraft). */
  place<T>(field: string): Place<T>;
  /**
   * track tells the form, at each drawing after its fields, whether a
   * draft differs from its stored value and which drafts it has, for its
   * Reset and its save.
   */
  track(changed: boolean, drafts: Taker[]): void;
  /** save runs the form's request, which resolves to whether it was taken; taken, every draft takes the stored value. */
  save(fn: () => Promise<boolean>): void;
  /** reset takes the stored values into every draft and clears a failed save. */
  reset(): void;
}

/**
 * useForm is a settings form kept by key: name names it in the problem
 * list's item, open shows it and puts the focus into it (focusForm). The
 * saved check stays as long as a result line's success (RESULT_FADE_MS),
 * the message of the same save. Its fields
 * keep their drafts at its place(field) (useDraft), its part calls track
 * after them, puts props on the form element and gives look, save and
 * reset to its footer.
 */
export function useForm({ key, name, open }: { key: string; name: FormName; open: () => void }): Form {
  const r = recordOf(key);
  void r.ver.value; // a change of a draft, a Reset, a forgetting: drawn again
  void r.phase.value;
  r.name = name;
  r.open = open;
  if (!r.said) {
    r.said = fader((res) => {
      if (res) setPhase(r, res.ok ? "saved" : "failed");
      else if (r.phase.peek() === "saved") setPhase(r, "");
    }, RESULT_FADE_MS);
  }
  const ref = useRef<HTMLFormElement>(null);
  const pressed = useRef(false);
  // after every drawing, once track has said what changed: a form with nothing left to send is clean again; publish does nothing where nothing did
  useEffect(() => {
    const now = recordOf(key);
    if (!now.changed && (now.phase.peek() === "unsaved" || now.phase.peek() === "failed")) now.phase.value = "";
    publish();
  });
  // a form opened from its unsaved item (focusForm) takes the focus in the drawing that shows it
  useLayoutEffect(() => {
    const now = recordOf(key);
    const el = ref.current;
    if (now.focus && el) {
      now.focus = false; // for this drawing alone: a form that is not shown now does not take the focus later
      if (!el.getClientRects().length) return; // not shown: nothing takes the focus
      const first = el.querySelector<HTMLElement>("input:not([disabled]), select:not([disabled]), textarea:not([disabled])");
      (first || el).focus();
    }
  });
  // taken away while changed: unsaved
  useEffect(() => () => {
    const now = records.get(key);
    if (now) left(now);
  }, [key]);
  return {
    get look() {
      return lookOf(r); // after track, which says what changed in this drawing
    },
    props: {
      ref,
      onFocusOut(ev) {
        const to = ev.relatedTarget as Node | null;
        if (pressed.current || (to && ref.current?.contains(to))) return;
        left(r);
      },
      // a press inside the form on what takes no focus leaves the focus nowhere: not a leaving
      onPointerDown() {
        pressed.current = true;
        setTimeout(() => {
          pressed.current = false;
        });
      },
    },
    place<T>(field: string): Place<T> {
      return {
        slot(stored) {
          let s = r.slots.get(field) as Slot<T> | undefined;
          if (!s) {
            s = { base: stored, value: stored, stored };
            r.slots.set(field, s as Slot<unknown>);
          }
          return s;
        },
        known: () => (r.slots.get(field) as Slot<T> | undefined)?.stored,
        changed: () => touched(r),
        redraw: () => {
          r.ver.value++;
        },
      };
    },
    track(changed, drafts) {
      r.changed = changed;
      r.drafts = drafts;
    },
    save(fn) {
      if (r.phase.peek() === "saving") return;
      clearTimeout(r.timer);
      setPhase(r, "saving");
      fn().then((ok) => {
        if (ok) {
          for (const d of r.drafts) d.take();
          r.changed = false; // taken away before its next drawing, it is not unsaved
        }
        r.said?.({ text: "", ok });
      }, () => r.said?.({ text: "", ok: false }));
    },
    reset() {
      clearTimeout(r.timer);
      for (const d of r.drafts) d.take();
      r.changed = false; // a Cancel that resets and closes leaves nothing unsaved
      r.said?.(null);
      setPhase(r, "");
    },
  };
}
