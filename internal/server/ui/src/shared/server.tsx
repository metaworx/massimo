// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/server.tsx
 *
 * The server's version as a signal, as /healthz gave it when the page
 * loaded (header.js checkVersion hands it to MassimoParts.chrome.version),
 * and the first row of a ☰ that shows it: Massimo's mark and the short
 * version, ☰'s copying itself, with its copy icon, the sign-in card's as
 * text; the sign-in card's foot reads it too. ☰'s copy closes ☰, the
 * focus back on its summary, and says so at the window's foot, as the
 * app's list says its own version's copy (AP MobileApp, D115-D; a bubble
 * above the row would go with ☰).
 */

import { signal } from "@preact/signals";
import { Brand } from "./brand";
import { Copy } from "./copy";
import { useLang } from "./lang";

/** The server's whole version, as /healthz gave it when the page loaded; "" until then. */
export const version = signal("");

/** closeMenu closes the menu that holds el, the focus back on its summary: a ☰'s version row, once its copy has started. */
function closeMenu(el: HTMLElement): void {
  const menu = el.closest("details");
  if (!menu) return;
  menu.open = false;
  menu.querySelector<HTMLElement>(":scope > summary")?.focus();
}

/** VersionRow is a ☰'s first row: Massimo's mark and the short version in the element of id, with copy a text that copies the whole version, its copy icon after it, which closes ☰, plain text without. */
export function VersionRow({ id, copy }: { id: string; copy?: boolean }) {
  useLang();
  const v = version.value;
  const short = v ? splitVersion(v).short : "";
  return (
    <p class="menu-version">
      <span class="menu-brand" data-brand aria-hidden="true"><Brand /></span>
      {copy && v ? <Copy id={id} cls="mono small" text={short} value={v} name={t("header.versionTitle", { version: v }) + " · " + t("copy.hint")} icon foot
        after={closeMenu} />
        : <span id={id} class="mono small">{short}</span>}
    </p>
  );
}
