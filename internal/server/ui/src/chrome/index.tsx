// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/chrome/index.tsx
 *
 * Where the page's chrome says that settings are unsaved (AP
 * UnsavedMarks, Block 5, UiParts.D188): a pill in a section's head for
 * the settings forms of the section, each item naming its form and
 * opening it from any page; on ☰ the worse of the log's mark and the
 * unsaved sign; and on a menu item the disk where a page it leads to
 * holds a form unsaved or failed. The section heads and the menu are
 * plain until AP UiParts' Block 14 and the log's mark is plain until its
 * Block 11: they have places of their own here, part-slots the plain
 * code leaves alone, each saying by data-forms which forms it shows, the
 * beginnings of their keys (shared/forms.ts), and a pill by data-title
 * the text key of its entry's name. The log tells its count and tone
 * through MassimoParts.chrome.log (log.js renderLogSign). A region of its
 * own, drawn once into its places as parts.js loads, since no region may
 * import another: it knows the forms through shared/forms.ts alone.
 */

import { signal } from "@preact/signals";
import { render } from "preact";
import { useLayoutEffect, useRef } from "preact/hooks";
import { unsavedItems, unsavedTone } from "../shared/forms";
import { Mark, Sign } from "../shared/icon";
import { useLang } from "../shared/lang";
import { kindsLabel, Problems } from "../shared/problems";
import { register } from "../shared/region";

/** The log's unseen problems as ☰ shows them: their count and the tone of the worst, "" for none (log.js renderLogSign). */
const logSign = signal<{ count: number; tone: "" | "warn" | "bad" }>({ count: 0, tone: "" });

/** keysOf are the beginnings of the forms' keys a place names in data-forms, space-separated. */
function keysOf(el: Element): string[] {
  return (el.getAttribute("data-forms") || "").split(/\s+/).filter(Boolean);
}

/**
 * SectionPill is a section head's pill: the disk of each unsaved or
 * failed form of keys, under one entry named title, each item opening its
 * form; nothing while there is none.
 */
function SectionPill({ keys, title }: { keys: string[]; title: string }) {
  useLang();
  const items = keys.flatMap(unsavedItems);
  if (!items.length) return null;
  return <Problems entries={[{ title: t(title), items }]} label={kindsLabel(items, ["unsaved"])} />;
}

/** worse is the worse of two tones: bad over warn over none. */
function worse(a: "" | "warn" | "bad", b: "" | "warn" | "bad"): "" | "warn" | "bad" {
  return a === "bad" || b === "bad" ? "bad" : a || b;
}

/**
 * MenuMark is ☰'s mark, in its summary: the worse of the log's tone and
 * the forms' of keys, the pages ☰ leads to, as a triangle or a cross; and
 * the summary's name and tooltip, which add the log's count and that
 * settings are unsaved.
 */
function MenuMark({ keys }: { keys: string[] }) {
  useLang();
  const ref = useRef<HTMLSpanElement>(null);
  const log = logSign.value;
  const forms = unsavedTone(keys);
  const tone = worse(log.tone, forms);
  const extra = [log.count ? t("nav.logAlerts", { count: log.count }) : "", forms ? t("problem.unsaved") : ""].filter(Boolean);
  const label = [t("nav.more"), ...extra].join(", ");
  const title = [t("nav.moreTitle"), ...extra].join(" · ");
  // the summary is the menu's, plain until AP UiParts' Block 14: its name and tooltip set from here, its content left alone
  useLayoutEffect(() => {
    const sum = ref.current?.closest("summary");
    if (!sum) return;
    sum.setAttribute("aria-label", label);
    sum.title = title;
  }, [label, title]);
  return <span ref={ref} class="menu-mark">{tone && <Mark state={tone} blink="log-sign-menu" />}</span>;
}

/** ItemMark is a menu item's disk, amber while a form of keys is unsaved, red where a save failed; nothing else. */
function ItemMark({ keys }: { keys: string[] }) {
  useLang();
  const tone = unsavedTone(keys);
  if (!tone) return null;
  return <Sign kind="disk" state={tone} cls="menu-unsaved" label={t("problem.unsaved")} />;
}

// drawn once into the places the page has for them
for (const el of document.querySelectorAll(".section-pill[data-forms]")) {
  render(<SectionPill keys={keysOf(el)} title={el.getAttribute("data-title") || ""} />, el);
}
const menu = document.getElementById("nav-menu");
const items = menu ? [...menu.querySelectorAll(".menu-item-mark[data-forms]")] : [];
for (const el of items) render(<ItemMark keys={keysOf(el)} />, el);
const mark = document.getElementById("nav-menu-mark");
if (mark) render(<MenuMark keys={items.flatMap(keysOf)} />, mark);

/** The functions the plain code calls, MassimoParts.chrome. */
const chrome = register("chrome", {
  /** log takes the log's unseen problems, their count and the worst one's tone, for ☰'s mark and name (log.js renderLogSign). */
  log(count: number, tone: "" | "warn" | "bad"): void {
    const was = logSign.peek();
    if (was.count !== count || was.tone !== tone) logSign.value = { count, tone };
  },
});

/** MassimoParts.chrome, as index.ts declares it. */
export type ChromePart = typeof chrome;
