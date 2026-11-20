// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/file.tsx
 *
 * A button that opens the browser's choice of a file, as filePicker of
 * components.js makes one: a hidden file input of the types it accepts
 * beside the button; a file chosen goes to onFile, and the input is
 * emptied, so that the same file can be chosen again.
 */

import { useRef } from "preact/hooks";

/** FileButton is the button (cls, label, off) and its hidden input of the types accept; onFile gets the file chosen. */
export function FileButton({ cls, label, accept, off, onFile }: { cls?: string; label: string; accept: string; off?: boolean; onFile: (file: File) => void }) {
  const input = useRef<HTMLInputElement>(null);
  return (
    <>
      <button type="button" class={cls} disabled={off} onClick={() => input.current?.click()}>{label}</button>
      <input ref={input} type="file" class="hidden" accept={accept} onChange={(ev) => {
        const el = ev.currentTarget;
        const file = el.files?.[0];
        el.value = "";
        if (file) onFile(file);
      }} />
    </>
  );
}
