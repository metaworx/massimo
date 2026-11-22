// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/file.tsx
 *
 * A button that opens the browser's choice of a file, as components.js'
 * filePicker made one: a hidden file input of the types it accepts
 * beside the button; a file chosen goes to onFile, and the input is
 * emptied, so that the same file can be chosen again. The button's id
 * and the input's, and the input's name in the page's language, are the
 * caller's where it names them (the Appearance card's Import…, whose
 * input a test and a screen reader find).
 */

import { useRef } from "preact/hooks";

/** FileButton is the button (id, cls, label, off) and its hidden input of the types accept (inputId, inputLabel); onFile gets the file chosen. */
export function FileButton({ id, cls, label, accept, off, inputId, inputLabel, onFile }: {
  id?: string;
  cls?: string;
  label: string;
  accept: string;
  off?: boolean;
  inputId?: string;
  inputLabel?: string;
  onFile: (file: File) => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  return (
    <>
      <button type="button" id={id} class={cls} disabled={off} onClick={() => input.current?.click()}>{label}</button>
      <input ref={input} type="file" id={inputId} class="hidden" accept={accept} aria-label={inputLabel} onChange={(ev) => {
        const el = ev.currentTarget;
        const file = el.files?.[0];
        el.value = "";
        if (file) onFile(file);
      }} />
    </>
  );
}
