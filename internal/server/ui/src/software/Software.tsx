// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/software/Software.tsx
 *
 * Maintenance → Software update as a part, built once and updated in
 * place: the update channel for a holder of settings, saved at once, the
 * running version and whether a newer release is out, the release's
 * notes, the helper's last state, the last three updates, Check for
 * updates and Update, which asks in place of the buttons. A load that
 * comes in leaves the channel's choice, its focus and the question where
 * they are; the card's badge says up to date or what is out, also while
 * the card is folded.
 */

import type { Ref } from "preact";
import { useLayoutEffect } from "preact/hooks";
import type { Setting, UpdateEntry, UpdateInfo, UpdateStatus } from "../api";
import { hid } from "../shared/cls";
import { Select } from "../shared/field";
import { BusyButton, busyProps, Question, useAsk, useSetAtOnce } from "../shared/question";
import { update } from "../shared/update";
import { ask, data, loadUpdate, question, runUpdate, setChannel, UPDATE_FLAVOURS } from "./store";
import { cardBadges } from "../shared/card";
import { can } from "../shared/me";
import { useLook } from "../shared/look";

/** stateText is a helper state's text, as the helper reports it where the page has none. */
function stateText(state: string): string {
  const key = "update.state." + state;
  const s = t(key);
  return s === key ? state : s;
}

/** helperText is the helper's last state, with the version, its message and when. */
function helperText(h: UpdateStatus): string {
  let s = stateText(h.state);
  if (h.to && h.state !== "up-to-date") s += " " + h.to;
  if (h.message) s += ": " + h.message;
  const when = h.time ? new Date(h.time) : null;
  if (when && !isNaN(when.getTime())) s += " (" + when.toLocaleString(i18n.lang) + ")";
  return s;
}

/**
 * historyLine is one of the last updates: the version reached, or the one
 * asked for with how far it got (under way for the newest, else not
 * reached, or the helper's outcome), when, and by whom or from outside
 * Massimo with the package manager.
 */
function historyLine(e: UpdateEntry, newest: boolean): string {
  const asked = e.to || "";
  let what = e.reached;
  if (!what && e.result) what = t("update.history.ended", { version: asked, state: stateText(e.result) });
  else if (!what) what = t(newest ? "update.history.running" : "update.history.notReached", { version: asked });
  const parts = [what];
  const ms = Date.parse(e.reached_at || e.at || "");
  if (!isNaN(ms)) parts.push(fmtInstant(new Date(ms))); // app.js
  if (e.outside) parts.push(e.manager ? t("update.history.outsideWith", { manager: e.manager }) : t("update.history.outside"));
  else if (e.by) parts.push(t("update.history.by", { name: e.by.display_name || e.by.name || "" }));
  return parts.join(" · ");
}

/** versionLine says the running version and whether a newer release is out on the channel. */
function versionLine(u: UpdateInfo): string {
  const version = u.current || t("update.unknownVersion");
  const parts = [u.channel ? t("update.runningChannel", { version, channel: u.channel }) : t("update.running", { version })];
  // a package manager owns the binary: the Debian package's helper installs through apt
  if (u.manager) parts.push(t("update.manager", { manager: u.manager }));
  if (u.check_error) parts.push(t("update.checkFailed", { error: u.check_error }));
  else if (!u.channel) parts.push(t("update.noChannel"));
  else if (!u.latest) parts.push(t("update.noRelease"));
  else if (u.available) parts.push(t("update.available", { version: u.latest }));
  else parts.push(t("update.upToDate"));
  if (u.available && u.blocked) parts.push(t("update.blocked", { reason: u.blocked }));
  return parts.join(" ");
}

/**
 * channelFixed says why the channel cannot be chosen here, "" where it
 * can: brew's formula decides it, or MASSIMO_UPDATE_CHANNEL overrides the
 * setting, so that the channel in effect is neither the setting's nor,
 * without one, apt's suite or the build's flavour.
 */
function channelFixed(u: UpdateInfo, setting: Setting): string {
  if (u.manager === "brew") {
    return t("update.channelBrew", { channel: u.channel, formula: u.channel === "stable" ? "massimo" : "massimo-" + u.channel });
  }
  const chosen = setting.set ? setting.value : u.suite || u.flavour;
  if (setting.in_effect !== chosen) return t("update.channelVariable", { channel: setting.in_effect || "" });
  return "";
}

/**
 * channelNote is what the choice does where it can be made: on a box apt
 * updates, the update helper switches apt's suite at the next update;
 * elsewhere the hint on the channels' order.
 */
function channelNote(u: UpdateInfo): string {
  if (u.manager !== "apt" || !u.suite) return t("update.channelHint");
  if (u.channel !== u.suite) return t("update.channelAptSwitch", { suite: u.suite, channel: u.channel });
  return t("update.channelApt", { channel: u.suite });
}

/**
 * Channel is the channel's choice, for a holder of settings, who alone
 * may change it: the build's flavour (apt's suite) or a channel of its
 * own, saved at once and kept on while it saves, so that the focus stays
 * on it; or the channel fixed, with the reason and the way to change it.
 */
function Channel({ u, setting }: { u: UpdateInfo; setting: Setting }) {
  useLook();
  const fixed = channelFixed(u, setting);
  const stored = fixed ? setting.in_effect || "" : setting.set ? setting.value : "";
  const choice = useSetAtOnce(stored);
  // Without a choice of its own the box follows apt's suite, or else its build's flavour.
  let none = u.flavour ? t("update.channelFlavour", { flavour: u.flavour }) : t("update.channelNone");
  if (u.manager === "apt" && u.suite) none = t("update.channelSuite", { suite: u.suite });
  const options: [string, string][] = [["", none], ...UPDATE_FLAVOURS.map((f): [string, string] => [f, t("update.flavour." + f)])];
  const said = data.channelSaid;
  return (
    <Select label={t("update.channel")} id="update-channel" wide options={options} value={choice.value} disabled={!!fixed}
      hint={said ? said() : fixed || channelNote(u)} hintId="update-channel-note" onChange={(v) => choice.set(v, setChannel)} />
  );
}

/** Software is the section's body. */
export function Software() {
  useLook();
  const asking = useAsk<HTMLButtonElement>(question);
  const u = update.value;
  const setting = data.setting;
  const busy = data.busy;
  const history = (u && u.history) || [];
  const known = !!u && !u.check_error && !!u.channel && !!u.latest;
  const badgeText = !known ? "" : u.available ? t("update.badge.available", { version: u.latest || "" }) : t("update.badge.upToDate");
  const badgeKind = u && u.available ? "warn" : "ok";
  // the card's badge, also while it is folded: up to date, or what is out
  useLayoutEffect(() => {
    cardBadges("sys-software", badgeText ? [{ text: badgeText, kind: badgeKind }] : []);
  }, [badgeText, badgeKind]);
  const failed = data.failed;
  const said = data.said;
  const version = question.version;
  return (
    <>
      {u && setting && can("settings") && <Channel u={u} setting={setting} />}
      <div ref={asking.row as Ref<HTMLDivElement>} class="update" aria-live="polite">
        <p id="update-text">{failed ? failed() : u ? versionLine(u) : "–"}</p>
        <p>
          <a id="update-link" class={u && u.release_url ? undefined : "hidden"} href={(u && u.release_url) || undefined} target="_blank" rel="noopener noreferrer">
            {u && u.release_url ? t("update.notes", { version: u.latest || "" }) : ""}
          </a>
        </p>
        <p id="update-helper" class="muted small">{said ? said() : u && u.helper ? t("update.last", { state: helperText(u.helper) }) : ""}</p>
        <div id="update-history" class={"update-history" + hid(!history.length)}>
          <h4>{t("update.history")}</h4>
          <ul id="update-history-list">{history.slice(0, 3).map((e, i) => <li key={i}>{historyLine(e, i === 0)}</li>)}</ul>
        </div>
        <div class={"actions" + hid(asking.asking)}>
          <BusyButton id="update-check" label={t("update.check")} off={busy} run={() => loadUpdate(true)} />
          <button ref={asking.ref} type="button" id="update-btn" {...busyProps("primary needs-update" + hid(!u || !u.can_update), busy)} onClick={ask}>{u ? t("update.to", { version: u.latest || "" }) : t("update.button")}</button>
        </div>
        {asking.asking && (
          // apt puts nothing back: the helper says how to go back instead
          <Question text={u && u.manager ? t("update.confirmManaged", { version, manager: u.manager }) : t("update.confirm", { version })}
            yes={t("update.to", { version })} no={t("action.cancel")} onNo={() => asking.done()} boxRef={asking.box}
            onYes={() => {
              asking.done(false);
              runUpdate();
              return Promise.resolve();
            }} />
        )}
      </div>
    </>
  );
}
