// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/chrome/Bar.tsx
 *
 * A page's row kept at the top under the header (AP UiParts, Block 14;
 * pageBar, pageSwitch, sectionButtons and fitPageTabs of components.js,
 * markSettingsPages of pages.js and renderMe of login.js before): its
 * title; on a settings page, in the title's place, a button for every
 * settings page in ☰'s order, so that each keeps its place, its icon and
 * its name, the icon alone below 900 px but for the page shown
 * (style.css), the page shown filled and marked current, a click on it
 * opening nothing, each hidden from a role that may not open its page by
 * that page's needs class; the title stays for a screen reader, and shows
 * in the buttons' place where the role may open no other settings page;
 * System info's badges (shared/card.ts); on the personal page who is in
 * and Sign out, in the words ☰ says it with (whoOf of shared/me.ts), and
 * My profile for a user alone; and the sections menu, an item per card of
 * the page, marking the card in view as the page scrolls, each bringing
 * its card into view. The row's height is --bar-h, for the cards' scroll
 * margin. Drawn once into the page's [data-bar] place, and in place after.
 */

import { type RefObject, render } from "preact";
import { useEffect, useLayoutEffect, useRef, useState } from "preact/hooks";
import { showCard } from "../shared/card";
import { Icon } from "../shared/icon";
import { useLang } from "../shared/lang";
import { me, whoOf } from "../shared/me";
import { Dropdown, MenuItem } from "../shared/menu";
import { isShown, mayShow, page, showView } from "../shared/pages";
import { CardBadges } from "./Cards";
import { SETTINGS_PAGES } from "./pages";

/** A page's row: its section's id, its title's key and id, and what else it holds. */
interface BarPage {
  view: string;
  section: string;
  title: string;
  titleId: string;
  pages?: boolean;
  sections?: boolean;
  badges?: boolean;
  me?: boolean;
}

/** The rows, by their place's data-bar. */
const BARS: Record<string, BarPage> = {
  me: { view: "me", section: "me", title: "nav.meTitle", titleId: "me-title", sections: true, me: true },
  system: { view: "system", section: "system", title: "nav.settingsTitle", titleId: "system-title", pages: true, sections: true },
  maint: { view: "maint", section: "maint-page", title: "nav.maintTitle", titleId: "maint-title", pages: true, sections: true },
  info: { view: "info", section: "info-page", title: "nav.infoTitle", titleId: "info-title", pages: true, badges: true },
  cube: { view: "cube", section: "cube-page", title: "nav.cubeTitle", titleId: "cube-page-title", pages: true, sections: true },
};

/** A card of a page as its sections menu lists it, read once from the page's skeleton. */
interface Card {
  id: string;
  key: string;
  icon: string;
  needs: string;
}

/** What a page's sections menu lists: its section cards, and a part without a card's fold marked page-section (Cube settings' panel, named by its data-toc). */
const PAGE_SECTIONS = ".section-card, .page-section";

/** cardsOf are the cards of a page, each named by its data-toc key or else its title's, with its icon (data-icon) and its needs-* classes. */
function cardsOf(section: HTMLElement): Card[] {
  return [...section.querySelectorAll<HTMLElement>(PAGE_SECTIONS)].map((card) => ({
    id: card.id,
    key: card.dataset.toc || card.querySelector(".section-title")?.getAttribute("data-i18n") || "",
    icon: card.dataset.icon || "",
    needs: [...card.classList].filter((c) => c.startsWith("needs-")).join(" "),
  }));
}

/**
 * Switch is a settings page's buttons, view the page it is on: as wide on
 * every page below 900 px for the one shown, the longest name among the
 * pages the role may open (--tab-name), measured while its page is shown,
 * as its own width, to the fraction, also of a name hidden on a phone.
 */
function Switch({ view, single, shown }: { view: string; single: boolean; shown: boolean }) {
  const lang = useLang();
  const ref = useRef<HTMLElement>(null);
  // biome-ignore lint/correctness/useExhaustiveDependencies: measured again when the page is shown, the language changes or who is in, which changes the buttons shown (its needs classes) and single; the names are read from the drawing itself.
  useLayoutEffect(() => {
    const nav = ref.current;
    if (!nav || !shown || !nav.offsetParent) return;
    const range = document.createRange();
    const widths = [...nav.querySelectorAll<HTMLElement>(".page-link")].filter((b) => b.offsetParent).map((b) => {
      const label = b.querySelector(".page-link-label");
      if (!label) return 0;
      range.selectNodeContents(label);
      return range.getBoundingClientRect().width;
    });
    if (widths.length) nav.style.setProperty("--tab-name", Math.ceil(Math.max(...widths)) + "px");
  }, [shown, lang, single, me.value]);
  return (
    <nav ref={ref} class="page-switch" aria-label={t("page.switch")}>
      {SETTINGS_PAGES.map((p) => {
        const here = p.view === view;
        return (
          <button key={p.view} type="button" class={"page-link " + p.needs + (here ? " current" : "")} data-view={p.view} aria-current={here ? "page" : undefined}
            title={t(p.key)} onClick={() => here || showView(p.view)}>
            <Icon symbol={p.icon} cls="ni" />
            <span class="page-link-label">{t(p.key)}</span>
          </button>
        );
      })}
    </nav>
  );
}

/**
 * Sections is a page's sections menu, in the row bar: an item per card
 * shown, the one in view marked as the page scrolls and as the menu
 * opens; hidden are the ids of cards not shown.
 */
function Sections({ view, bar, cards, hidden }: { view: string; bar: RefObject<HTMLDivElement | null>; cards: Card[]; hidden: string[] }) {
  useLang();
  const [inView, setInView] = useState<string | null>(null);
  // the card in view: the lowest top that has passed under the header and
  // this row; of cards side by side, the first
  const mark = useRef(() => {});
  mark.current = () => {
    const section = document.getElementById(BARS[view]?.section || view);
    if (!section || !bar.current || !isShown(view)) return;
    const line = bar.current.getBoundingClientRect().bottom + 24;
    let best: string | null = null;
    let bestTop = -Infinity;
    for (const card of section.querySelectorAll<HTMLElement>(PAGE_SECTIONS)) {
      if (!card.offsetParent) continue; // not shown to this role
      const y = Math.round(card.getBoundingClientRect().top);
      if (y <= line && y > bestTop) {
        best = card.id;
        bestTop = y;
      }
    }
    setInView(best);
  };
  useEffect(() => {
    const on = () => mark.current();
    addEventListener("scroll", on, { passive: true });
    return () => removeEventListener("scroll", on);
  }, []);
  return (
    <Dropdown cls="sec-menu" label={t("page.sections")} title={t("page.sectionsTitle")} summary={<Icon symbol="icon-list" cls="ni" />}
      onOpen={() => mark.current()} onClose={() => mark.current()}>
      {cards.filter((c) => !hidden.includes(c.id)).map((c) => (
        <MenuItem key={c.id} cls={"toc-link" + (c.needs ? " " + c.needs : "")} data={{ target: c.id }} current={inView === c.id}
          icon={c.icon ? <Icon symbol={c.icon} cls="ni" /> : undefined} label={t(c.key)} onClick={() => showCard(c.id)} />
      ))}
    </Dropdown>
  );
}

/** Bar is a page's row, of the page b names; cards are its page's, read once. */
function Bar({ b, cards }: { b: BarPage; cards: Card[] }) {
  useLang();
  const ref = useRef<HTMLDivElement>(null);
  const shown = page.value === b.view;
  const who = whoOf(me.value);
  const single = SETTINGS_PAGES.filter((p) => mayShow(p.view)).length < 2;
  // the row's height for the cards' scroll margin, written by the row of
  // the page shown alone, as it is shown and as it changes size: a hidden
  // row's 0 written after it brought a card's head under the row
  // biome-ignore lint/correctness/useExhaustiveDependencies: shown is the dependency by design, the row measured again as its page is shown; b.view is the row's for its life.
  useLayoutEffect(() => {
    const bar = ref.current;
    if (!bar) return;
    const set = () => {
      if (isShown(b.view) && bar.offsetHeight) document.documentElement.style.setProperty("--bar-h", bar.offsetHeight + "px");
    };
    set();
    const ro = new ResizeObserver(set);
    ro.observe(bar);
    return () => ro.disconnect();
  }, [shown]);
  // My profile is the personal page's for a user alone (an open server and the server's token have none): its card hidden, its item left out
  useLayoutEffect(() => {
    if (b.me) document.getElementById("profile")?.classList.toggle("hidden", !who.profile);
  }, [b.me, who.profile]);
  const title = <h2 id={b.titleId}>{t(b.title)}</h2>;
  return (
    <div ref={ref} class={"page-bar" + (b.pages ? " has-switch" : "") + (b.pages && single ? " single-page" : "")}>
      {b.me ? (
        <div class="page-title">
          {title}
          <p id="me-line" class={"me-line small" + (who.line ? "" : " hidden")}>{who.line}</p>
        </div>
      ) : title}
      {b.pages && <Switch view={b.view} single={single} shown={shown} />}
      {b.badges && <span class="section-badges"><CardBadges id={b.section} /></span>}
      {b.me && (
        <button type="button" id="logout-btn" class={"logout-btn" + (who.signOut ? "" : " hidden")} onClick={() => logout()}>
          <Icon symbol={who.icon} cls="ni" useId="logout-icon" />
          <span id="logout-label">{who.signOut || t("me.signOut")}</span>
        </button>
      )}
      {b.sections && <Sections view={b.view} bar={ref} cards={cards} hidden={b.me && !who.profile ? ["profile"] : []} />}
    </div>
  );
}

/** drawBars draws each page's row into its place, once, with the cards its page holds. */
export function drawBars(): void {
  for (const el of document.querySelectorAll<HTMLElement>("[data-bar]")) {
    const b = BARS[el.dataset.bar || ""];
    const section = b && document.getElementById(b.section);
    if (b && section) render(<Bar b={b} cards={cardsOf(section)} />, el);
  }
}
