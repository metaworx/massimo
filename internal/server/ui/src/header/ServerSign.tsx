// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/ServerSign.tsx
 *
 * The server's sign, first in the header's pile: its icon in its state's
 * colour, green while the server answers, blue while an update is out for
 * one who may install it, amber once the server runs a new version (its
 * triangle blinks), red after a poll without an answer (it blinks, and the
 * pile is filled red); and, while its state offers something to do, that
 * as an item of the pile's list: Software update, or the reload for the
 * new version.
 */

import { Icon } from "../shared/icon";
import { MenuItem } from "../shared/menu";

/** The icon of each state of the server's sign. */
const SERVER_ICON: Record<string, string> = { up: "icon-plug-on", info: "icon-info", new: "mark-warn", down: "icon-plug-off" };

/**
 * serverState is the state the server's sign shows: "down" while the
 * last poll failed, "new" once the server runs another version than the
 * page, "info" while an update is out for one who may install it
 * (updateOut of update.js), "up" otherwise.
 */
export function serverState(): string {
  if (state.serverDown) return "down";
  if (state.newVersion) return "new";
  return updateOut() ? "info" : "up";
}

/** serverWords say the server's state, for the pile's label and tooltip. */
export function serverWords(st: string): string {
  return t("server." + st, { version: updateLatest(), seconds: state.settings.poll });
}

/** ServerIcon is the server's sign in the pile's summary, its state its class. */
export function ServerIcon({ st }: { st: string }) {
  return <Icon symbol={SERVER_ICON[st]} cls={"ni server-icon state-" + st} mark={st === "new"} />;
}

/** openSoftware opens Software update on Maintenance. */
function openSoftware(): void {
  showView("maint");
  showCard($("sys-software"));
}

/**
 * ServerItem is what the server's state offers, an item of the pile's
 * list: while an update is out, Software update; once the server runs a
 * new version, the page's reload; nothing otherwise, where Refresh now is
 * what there is to do.
 */
export function ServerItem({ st }: { st: string }) {
  if (st === "info") {
    return <MenuItem cls="server-item" icon={<Icon symbol="icon-info" cls="ni" />} label={t("server.toUpdate", { version: updateLatest() })} onClick={openSoftware} />;
  }
  if (st === "new") {
    return <MenuItem cls="server-item" icon={<Icon symbol="icon-refresh" cls="ni" />} label={t("server.reload")} onClick={() => location.reload()} />;
  }
  return null;
}
