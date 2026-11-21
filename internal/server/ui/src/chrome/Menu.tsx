// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/chrome/Menu.tsx
 *
 * ☰, the page's one menu, and the header's page buttons (AP GlobalMenu,
 * AP UiParts Block 14): ↑ while the header is compacted, Rooms, and ☰
 * with every page but Rooms, at every width, each for the roles that may
 * open it (their needs-* classes, login.js applyCaps): the log; the
 * settings pages; Personal settings, named after who is in in its
 * tooltip; in the app Select server; and Sign out, as the personal page's
 * button says it. Its first row is Massimo's icon and the server's
 * version, which copies itself, a rule under it. ☰ carries the worse of
 * the log's mark and the unsaved settings' (AP UnsavedMarks, Block 5), an
 * entry the disk where a page it leads to holds a form unsaved or failed
 * (shared/forms.ts), and its name and tooltip say both. The entry of the
 * page shown is current, and ☰ filled while one of its pages is shown;
 * a choice closes ☰ (dropdown of components.js). Drawn once into the
 * header's nav, the details kept, so that ☰ stays open through data.
 */

import { Brand } from "../shared/brand";
import { Copy } from "../shared/copy";
import { unsavedTone } from "../shared/forms";
import { Icon, IconButton, Mark, Sign } from "../shared/icon";
import { useLang } from "../shared/lang";
import { me, whoOf } from "../shared/me";
import { Dropdown, MenuItem } from "../shared/menu";
import { page, showView } from "../shared/pages";
import { MENU_PAGES } from "./pages";
import { toTop } from "./scroll";
import { logSign, version } from "./store";

/** worse is the worse of two tones: bad over warn over none. */
function worse(a: "" | "warn" | "bad", b: "" | "warn" | "bad"): "" | "warn" | "bad" {
  return a === "bad" || b === "bad" ? "bad" : a || b;
}

/** ItemMark is a menu item's disk, amber while a form of keys is unsaved, red where a save failed; nothing else. */
function ItemMark({ keys }: { keys: string[] }) {
  useLang();
  const tone = unsavedTone(keys);
  if (!tone) return null;
  return <Sign kind="disk" state={tone} cls="menu-unsaved" label={t("problem.unsaved")} />;
}

/** toServers goes back to the app's servers, where the page is in the app. */
function toServers(): void {
  const go = appConnect();
  if (go) go().catch(() => {});
}

/** Menu is ☰ with its entries; view is the page shown. */
function Menu({ view }: { view: string }) {
  useLang();
  const log = logSign.value;
  const v = version.value;
  const who = whoOf(me.value);
  const keys = MENU_PAGES.flatMap((p) => p.forms);
  const forms = unsavedTone(keys);
  const tone = worse(log.tone, forms);
  const extra = [log.count ? t("nav.logAlerts", { count: log.count }) : "", forms ? t("problem.unsaved") : ""].filter(Boolean);
  const label = [t("nav.more"), ...extra].join(", ");
  const title = [t("nav.moreTitle"), ...extra].join(" · ");
  const short = v ? splitVersion(v).short : "";
  const servers = !!appConnect();
  const entry = (p: (typeof MENU_PAGES)[number]) => (
    <MenuItem key={p.view} id={p.id} cls={p.needs || undefined} current={view === p.view ? "page" : undefined}
      icon={p.view === "log" ? <Sign kind="log" state={log.tone} blink="log-sign" /> : <Icon symbol={p.icon} cls="ni" />}
      label={t(p.key)} aria={p.view === "log" && log.count ? t("nav.logAlerts", { count: log.count }) : undefined}
      title={p.view === "me" && who.name ? t("nav.userNamed", { name: who.name }) : undefined}
      after={p.forms.length ? <ItemMark keys={p.forms} /> : undefined} onClick={() => showView(p.view)} />
  );
  return (
    <Dropdown id="nav-menu" cls="nav-menu" summaryCls={view in MENU_VIEWS ? "nav-btn active" : "nav-btn"} label={label} title={title}
      summary={
        <>
          <Icon symbol="icon-menu" cls="ni" />
          <span class="menu-mark">{tone && <Mark state={tone} blink="log-sign-menu" />}</span>
        </>
      }>
      <p class="menu-version">
        <span class="menu-brand" aria-hidden="true"><Brand /></span>
        {v ? <Copy id="version-short" cls="mono small" text={short} value={v} name={t("header.versionTitle", { version: v }) + " · " + t("copy.hint")} />
          : <span id="version-short" class="mono small" />}
      </p>
      <hr class="menu-sep" />
      {entry(MENU_PAGES[0])}
      <hr class="menu-sep" />
      {MENU_PAGES.slice(1, 5).map(entry)}
      <hr class="menu-sep" />
      {entry(MENU_PAGES[5])}
      <MenuItem id="menu-servers" cls={servers ? undefined : "hidden"} icon={<Icon symbol="icon-servers" cls="ni" />} label={t("app.selectServer")}
        onClick={toServers} />
      <MenuItem id="menu-logout" cls={who.signOut ? undefined : "hidden"} icon={<Icon symbol={who.icon} cls="ni" useId="menu-logout-icon" />}
        label={who.signOut || t("me.signOut")} labelId="menu-logout-label" onClick={() => logout()} />
    </Dropdown>
  );
}

/** The pages of ☰, by their views. */
const MENU_VIEWS: Record<string, true> = Object.fromEntries(MENU_PAGES.map((p) => [p.view, true]));

/** Nav is the header's page buttons: ↑ while the header is compacted, Rooms, and ☰. */
export function Nav() {
  useLang();
  const view = page.value;
  const rooms = view === "tiles";
  return (
    <>
      <IconButton symbol="icon-up" id="top-btn" cls="icon-btn nav-btn top-btn" label={t("page.toTop")} title={t("page.toTopTitle")} onClick={toTop} />
      <IconButton symbol="icon-house" id="view-tiles" cls={rooms ? "icon-btn nav-btn active" : "icon-btn nav-btn"} aria-current={rooms ? "page" : undefined}
        label={t("nav.rooms")} title={t("nav.roomsTitle")} onClick={() => showView("tiles")} />
      <Menu view={view} />
    </>
  );
}
