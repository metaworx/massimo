// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/chrome/Cards.tsx
 *
 * What a section card's head shows besides its title (AP UiParts, Block
 * 14): its badges, which the region that fills the card gives by the
 * card's id (cardBadges of shared/card.ts), drawn into the head's
 * span.section-badges; and its pill (AP UnsavedMarks, Block 5,
 * UiParts.D188), the disk of each unsaved or failed settings form of the
 * section under one entry, each item naming its form and opening it from
 * any page, drawn into the head's span.section-pill by data-forms, the
 * beginnings of the forms' keys (shared/forms.ts), and data-title, the
 * text key of the entry's name. The card's frame, title, fold and body
 * are the page's skeleton (shared/card.ts).
 */

import { render } from "preact";
import { Badge } from "../shared/badge";
import { badgesOf } from "../shared/card";
import { unsavedItems } from "../shared/forms";
import { useLang } from "../shared/lang";
import { kindsLabel, Problems } from "../shared/problems";

/** keysOf are the beginnings of the forms' keys a place names in data-forms, space-separated. */
export function keysOf(el: Element): string[] {
  return (el.getAttribute("data-forms") || "").split(/\s+/).filter(Boolean);
}

/** CardBadges are a card's badges, by its id: what its region gave last, nothing before. */
export function CardBadges({ id }: { id: string }) {
  const list = badgesOf(id).value;
  return <>{list.map((b) => <Badge key={b.kind + ":" + b.text} kind={b.kind} text={b.text} />)}</>;
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

/**
 * drawCards draws the badges and the pill of every card of the page,
 * once, into the places the skeleton has for them: a card's badges by the
 * id of the card or the page that holds the place.
 */
export function drawCards(): void {
  for (const el of document.querySelectorAll<HTMLElement>(".section-badges")) {
    const owner = el.closest(".section-card, .settings-page");
    if (owner && owner.id) render(<CardBadges id={owner.id} />, el);
  }
  for (const el of document.querySelectorAll<HTMLElement>(".section-pill[data-forms]")) {
    render(<SectionPill keys={keysOf(el)} title={el.getAttribute("data-title") || ""} />, el);
  }
}
