// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/Banner.tsx
 *
 * The warning under the header while the server asks for no key (login.js
 * renderOpenBanner before): a store without users, or no store and no
 * token, each with its own text. It cannot be dismissed, and goes once the
 * server asks who is there; drawn from who is in (shared/me.ts).
 */

import { hid } from "../shared/cls";
import { useLang } from "../shared/lang";
import { me } from "../shared/me";

/** Banner is the open server's warning, hidden in its place while the server asks who is there. */
export function Banner() {
  useLang();
  const mode = me.value?.mode;
  const open = mode === "open" || mode === "none";
  return (
    <p id="open-banner" class={"open-banner" + hid(!open)} role="note">
      <strong>{t("open.title")}</strong> <span id="open-banner-text">{open ? t(mode === "open" ? "open.text" : "open.textNoStore") : ""}</span>
    </p>
  );
}
