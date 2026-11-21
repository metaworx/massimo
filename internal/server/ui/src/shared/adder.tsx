// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/adder.tsx
 *
 * The end of a list that adds an item (Add a user, Add a role, Add a
 * provider): the new item's form in its place while it is shown, and the
 * button that shows it, hidden meanwhile. Shown, the form takes the focus
 * in its first field; closed, by its Cancel or once the item is added,
 * the focus goes back to the button. The caller keeps whether it is
 * shown (live state, so that it outlives a reload of the list) and draws
 * the form.
 */

import type { ComponentChildren } from "preact";
import { useEffect, useRef } from "preact/hooks";
import { hid } from "./cls";

/**
 * Adder is the form's place, of the id formId, holding children while
 * shown, and the row with the button of the id id and the label label,
 * which calls set(true); the caller's form calls set(false) to close.
 */
export function Adder({ id, formId, label, shown, set, children }: {
  id: string;
  formId: string;
  label: string;
  shown: boolean;
  set: (shown: boolean) => void;
  children?: ComponentChildren;
}) {
  const box = useRef<HTMLDivElement>(null);
  const btn = useRef<HTMLButtonElement>(null);
  const was = useRef(shown);
  // a form just shown takes the focus in its first field, one just closed gives it back to the button
  useEffect(() => {
    if (was.current === shown) return;
    was.current = shown;
    if (shown) box.current?.querySelector<HTMLElement>("input:not([disabled]), select:not([disabled])")?.focus();
    else if (box.current?.contains(document.activeElement) || document.activeElement === document.body) btn.current?.focus();
  }, [shown]);
  return (
    <>
      <div ref={box} id={formId}>{shown && children}</div>
      <div class="actions">
        <span class="actions-left">
          <button ref={btn} type="button" id={id} class={hid(shown).trim() || undefined} onClick={() => set(true)}>{label}</button>
        </span>
      </div>
    </>
  );
}
