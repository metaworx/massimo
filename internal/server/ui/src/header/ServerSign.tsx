// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/ServerSign.tsx
 *
 * The server's sign before the time of the last update: an icon in a
 * frame, green while the server answers, blue while an update is out for
 * one who may install it, amber once the server runs a new version (its
 * triangle blinks), filled red after a poll without an answer (it blinks);
 * its label says what a click does, and a click does it. Built once and
 * updated in place.
 */

import { IconButton } from "../shared/icon";
import { pollNow } from "./send";
import { load } from "./store";

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

/**
 * serverAct does what the sign's state offers: a new version reloads the
 * page, an update out opens Software update, otherwise it polls now.
 */
function serverAct(): void {
  const st = serverState();
  if (st === "new") location.reload();
  else if (st === "info") {
    showView("maint");
    showCard($("sys-software"));
  } else pollNow();
}

/** ServerSign is the server's sign, an icon button. */
export function ServerSign() {
  load.useLoad();
  const st = serverState();
  const label = t("server." + st, { version: updateLatest(), seconds: state.settings.poll });
  return <IconButton id="server-sign" cls="server-sign" data-state={st} symbol={SERVER_ICON[st]} mark={st === "new"} label={label} onClick={serverAct} />;
}
