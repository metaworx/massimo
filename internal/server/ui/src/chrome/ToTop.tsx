// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/chrome/ToTop.tsx
 *
 * Back to top at the bottom right of every page once it is a screen
 * down (AP UiParts, Block 14; backToTop of components.js), drawn into
 * #to-top-slot at the end of the page's main; the header's ↑, shown while
 * the header is compacted, is the page buttons' (Nav).
 */

import { useEffect, useState } from "preact/hooks";
import { IconButton } from "../shared/icon";
import { useLang } from "../shared/lang";
import { toTop } from "./scroll";

/** ToTop is ↑ at the bottom right, hidden until the page is a screen down; it takes the page to its top (toTop). */
export function ToTop() {
  useLang();
  const [shown, setShown] = useState(false);
  useEffect(() => {
    const show = () => setShown(window.scrollY >= window.innerHeight);
    addEventListener("scroll", show, { passive: true });
    addEventListener("resize", show, { passive: true });
    show();
    return () => {
      removeEventListener("scroll", show);
      removeEventListener("resize", show);
    };
  }, []);
  return <IconButton symbol="icon-up" id="to-top" cls={"icon-btn to-top" + (shown ? "" : " hidden")} label={t("page.toTop")} title={t("page.toTopTitle")}
    onClick={toTop} />;
}
