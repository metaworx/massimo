// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/dialog.tsx
 *
 * The dialog that lists the forms with changes not saved, as a part (AP
 * UnsavedMarks): a modal <dialog> of the page, as the fingerprint sheet
 * of connect.html is, with its title, a line that says what is asked, a
 * button per form, which closes the dialog and opens the form, and its
 * own buttons: the one that stays, which Escape is too, and one that
 * goes on where there is something to go on to. Opened, it puts the
 * focus on the button that stays; closed, the browser gives the focus
 * back to where it was, and a form opened from it takes it from there.
 * However it is closed, by a button, by Escape or because it is no
 * longer shown, its close event says what follows: what the button does,
 * else a Stay.
 */

import { useId, useLayoutEffect, useRef } from "preact/hooks";
import { Sign } from "./icon";
import type { Item } from "./problems";

/**
 * FormsDialog asks while shown: title and text, then forms, the items of
 * the forms with changes (forms.ts changedItems), each a button that
 * closes the dialog, runs onStay and opens its form (its action); stay
 * labels the button that closes it and runs onStay, as Escape does; go,
 * where given, is a button that closes it and runs its onGo, its label
 * given.
 */
export function FormsDialog({ shown, title, text, forms, stay, onStay, go }: {
  shown: boolean;
  title: string;
  text: string;
  forms: Item[];
  stay: string;
  onStay: () => void;
  go?: { label: string; onGo: () => void };
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const stayRef = useRef<HTMLButtonElement>(null);
  const after = useRef<(() => void) | null>(null);
  const titleId = useId();
  const textId = useId();
  // opened and closed as shown says, Stay taking the focus in the drawing that opens it; a button closes it itself (end)
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (shown && !el.open) {
      el.showModal();
      stayRef.current?.focus();
    } else if (!shown && el.open) {
      el.close();
    }
  }, [shown]);
  // a button closes the dialog, then what it does runs once it is closed (onClose): the browser has given the focus back by then, and a form opened from it takes it
  const end = (then: () => void) => {
    after.current = then;
    if (ref.current?.open) ref.current.close();
    else done();
  };
  // closed by a button, by Escape (the browser's own close), or as shown says: a close without a button of its own stays
  const done = () => {
    const then = after.current || onStay;
    after.current = null;
    then();
  };
  return (
    <dialog ref={ref} class="forms-dialog" aria-labelledby={titleId} aria-describedby={textId} onClose={done}>
      <h3 id={titleId}>{title}</h3>
      <p id={textId}>{text}</p>
      <ul class="dialog-forms">
        {forms.map((it, i) => (
          <li key={i + ":" + it.text}>
            <button type="button" class="dialog-form" onClick={() => end(() => {
              onStay();
              it.action?.();
            })}>
              <Sign kind={it.sign || "disk"} state={it.tone || "warn"} />
              <span>{it.text}</span>
            </button>
          </li>
        ))}
      </ul>
      <div class="actions">
        <button ref={stayRef} type="button" class={go ? "dialog-stay" : "dialog-stay primary"} onClick={() => end(onStay)}>{stay}</button>
        {go && <button type="button" class="dialog-go" onClick={() => end(go.onGo)}>{go.label}</button>}
      </div>
    </dialog>
  );
}
