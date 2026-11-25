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
 * reads the versions again at its change event. While the server's next
 * start refuses the settings file, Install now asks first, where it was
 * pressed, with the update's question and the sentence that the server
 * stays down until the file is fixed (D14-A). Everyone else sees nothing
 * new.
 */

import type { UpdateInfo } from "../api";
import { hid } from "../shared/cls";
import { useLang } from "../shared/lang";
import { can } from "../shared/me";
import { busyProps, Question, useAsk } from "../shared/question";
import { update } from "../shared/update";
import { answerUpdate, type AskPlace, data, installNow, nowAsks } from "./store";

/** asked is the version the question asks about, "" where the page asks nothing: no update due, no night, an answer kept, or a user without update. */
export function asked(): string {
  const u = update.value;
  if (!u || !u.auto || !u.auto.ask || !u.available || !u.latest || !can("update")) return "";
  return u.latest;
}

/**
 * updateQuestion is the update's question about version: on a box a
 * package manager updates, that the last update says how to go back,
 * since it puts nothing back; elsewhere that the old version is put back
 * where the new one does not come up; and where refused, the server's
 * next start refusing the settings file, after it the sentence that the
 * server does not come back until the file is fixed, as downUntilFixed
 * (maintenance/) joins it to the restart's (D13-A): the old version
 * would not start either.
 */
export function updateQuestion(u: UpdateInfo | null, version: string, refused: boolean): string {
  // apt puts nothing back: the helper says how to go back instead
  const q = u && u.manager ? t("update.confirmManaged", { version, manager: u.manager }) : t("update.confirm", { version });
  return refused ? q + " " + t("restart.confirmRefused") : q;
}

/**
 * AskButtons are the question's three answers, their ids prefix-now,
 * prefix-tonight and prefix-skip, off while an answer is on its way or
 * Install now reads; under them why the last one was refused. Install
 * now reads first whether the server's next start refuses the settings
 * file (installNow), turning meanwhile: where it does, it asks in place
 * of the three, Yes answering now and Cancel giving them back with
 * nothing sent; where it does not, or the read failed, it answers now at
 * once.
 */
export function AskButtons({ prefix, version }: { prefix: AskPlace; version: string }) {
  useLang();
  const held = nowAsks[prefix];
  const asking = useAsk<HTMLButtonElement>(held);
  const reading = held.reading;
  const off = data.answering || reading;
  const said = data.answerSaid;
  const askedAbout = held.version;
  return (
    <>
      <span class={"update-ask-actions" + hid(asking.asking)}>
        <button ref={asking.ref} type="button" id={prefix + "-now"} {...busyProps("primary", reading, data.answering)} onClick={() => installNow(prefix, version)}>{t("update.ask.now")}</button>
        <button type="button" id={prefix + "-tonight"} disabled={off} onClick={() => answerUpdate(version, "tonight")}>{t("update.ask.tonight")}</button>
        <button type="button" id={prefix + "-skip"} disabled={off} onClick={() => answerUpdate(version, "skip")}>{t("update.ask.skip")}</button>
      </span>
      {asking.asking && (
        <Question text={updateQuestion(update.value, askedAbout, true)} yes={t("update.ask.now")} no={t("action.cancel")} onNo={() => asking.done()} boxRef={asking.box}
          onYes={() => {
            asking.done(false);
            return answerUpdate(askedAbout, "now");
          }} />
      )}
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
    <div id="update-ask" class={"update-ask" + hid(!version)} role="note">
      <span id="update-ask-text">{version ? askText(version) : ""}</span>
      {version && <AskButtons prefix="update-ask" version={version} />}
    </div>
  );
}
