// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/server.tsx
 *
 * The server's version as a signal, as /healthz gave it when the page
 * loaded (header.js checkVersion hands it to MassimoParts.chrome.version),
 * and the first row of a ☰ that shows it: Massimo's mark and the short
 * version, ☰'s copying itself, the sign-in card's as text; the sign-in
 * card's foot reads it too.
 */

import { signal } from "@preact/signals";
import { Brand } from "./brand";
import { Copy } from "./copy";
import { useLang } from "./lang";

/** The server's whole version, as /healthz gave it when the page loaded; "" until then. */
export const version = signal("");

/** VersionRow is a ☰'s first row: Massimo's mark and the short version in the element of id, a text that copies the whole version with copy, plain text without. */
export function VersionRow({ id, copy }: { id: string; copy?: boolean }) {
  useLang();
  const v = version.value;
  const short = v ? splitVersion(v).short : "";
  return (
    <p class="menu-version">
      <span class="menu-brand" data-brand aria-hidden="true"><Brand /></span>
      {copy && v ? <Copy id={id} cls="mono small" text={short} value={v} name={t("header.versionTitle", { version: v }) + " · " + t("copy.hint")} />
        : <span id={id} class="mono small">{short}</span>}
    </p>
  );
}
