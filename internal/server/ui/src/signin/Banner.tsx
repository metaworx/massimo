// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/Banner.tsx
 *
 * The warning under the header while the server asks for no key (login.js
 * renderOpenBanner before): a store without users, or no store and no
 * token, each with its own text, drawn from who is in (shared/me.ts). On
 * a store without users (AP FirstOwner, Block 4) "Create the owner…"
 * opens the sign-in card with the first owner's form where the owner can
 * be created from here, and Hide hides the warning, kept in the browser
 * while the server has no owner (D2): it goes once the server asks who is
 * there, and so does the hiding, so that a server open again shows it. A
 * server without a store keeps its warning as it is: it can get no owner
 * from the page (D39).
 */

import { useEffect, useRef } from "preact/hooks";
import { hid } from "../shared/cls";
import { useLang } from "../shared/lang";
import { firstOwnerOf, me } from "../shared/me";
import { mainMenuButton } from "../shared/menu";
import { askOwner } from "../shared/owner";
import { bannerAgain, card, hideBanner } from "./store";

/** Banner is the open server's warning, hidden in its place while the server asks who is there, or where it was hidden. */
export function Banner() {
  useLang();
  const m = me.value;
  const mode = m?.mode;
  const open = mode === "open" || mode === "none";
  const hidden = card.bannerHidden;
  const createRef = useRef<HTMLButtonElement>(null);
  // the hiding forgotten once the server has users (who is in says users), not before: a hidden warning stays hidden
  // while the server has no owner
  useEffect(() => {
    if (mode === "users") bannerAgain();
  }, [mode]);
  return (
    <p id="open-banner" class={"open-banner" + hid(!open || (mode === "open" && hidden))} role="note">
      <strong>{t("open.title")}</strong> <span id="open-banner-text">{open ? t(mode === "open" ? "open.text" : "open.textNoStore") : ""}</span>
      {mode === "open" && (
        <span class="open-banner-actions">
          <button ref={createRef} type="button" id="open-banner-create" class={hid(!firstOwnerOf(m)).trim() || undefined}
            onClick={() => askOwner(createRef.current)}>{t("setup.ask")}</button>
          <button type="button" id="open-banner-hide" aria-label={t("open.hideLabel")} onClick={() => {
            hideBanner();
            // the warning gone, the focus to ☰'s button rather than to nothing; given in the handler of the user's own
            // action, the button being in the page already (a drawing that shows an element gives it in a layout effect)
            mainMenuButton.current?.focus();
          }}>{t("open.hide")}</button>
        </span>
      )}
    </p>
  );
}
