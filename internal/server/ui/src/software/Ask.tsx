// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/software/Ask.tsx
 *
 * The automatic update's question (AP AutoUpdate D3-A, D10-A): where
 * automatic updates are on and an update is due, GET /api/v1/update says
 * ask, and a holder of update is asked Install now, Tonight or Skip this
 * version, in a banner under the header on every page (#update-ask-slot)
 * and in Software update. The answer is the box's (PUT
 * /api/v1/update/answer): it ends the question in every browser, which
 * reads the versions again at its change event. Everyone else sees
 * nothing new.
 */

import { hid } from "../shared/cls";
import { useLang } from "../shared/lang";
import { can } from "../shared/me";
import { update } from "../shared/update";
import { answerUpdate, data } from "./store";

/** asked is the version the question asks about, "" where the page asks nothing: no update due, no night, an answer kept, or a user without update. */
export function asked(): string {
  const u = update.value;
  if (!u || !u.auto || !u.auto.ask || !u.available || !u.latest || !can("update")) return "";
  return u.latest;
}

/** AskButtons are the question's three answers, their ids prefix-now, prefix-tonight and prefix-skip, off while an answer is on its way; under them why the last one was refused. */
export function AskButtons({ prefix, version }: { prefix: string; version: string }) {
  useLang();
  const off = data.answering;
  const said = data.answerSaid;
  return (
    <>
      <span class="update-ask-actions">
        <button type="button" id={prefix + "-now"} class="primary" disabled={off} onClick={() => answerUpdate(version, "now")}>{t("update.ask.now")}</button>
        <button type="button" id={prefix + "-tonight"} disabled={off} onClick={() => answerUpdate(version, "tonight")}>{t("update.ask.tonight")}</button>
        <button type="button" id={prefix + "-skip"} disabled={off} onClick={() => answerUpdate(version, "skip")}>{t("update.ask.skip")}</button>
      </span>
      {said && <span id={prefix + "-said"} class="small warn-text" role="status">{said()}</span>}
    </>
  );
}

/** askText is the question: the version out, and the window it is installed in unless answered otherwise. */
export function askText(version: string): string {
  const w = (update.value?.auto?.window || "").split("-");
  return t("update.ask.text", { version, from: w[0] || "", to: w[1] || "" });
}

/** Ask is the banner under the header, hidden in its place while the page asks nothing. */
export function Ask() {
  useLang();
  const version = asked();
  return (
    <p id="update-ask" class={"update-ask" + hid(!version)} role="note">
      <span id="update-ask-text">{version ? askText(version) : ""}</span>
      {version && <AskButtons prefix="update-ask" version={version} />}
    </p>
  );
}
