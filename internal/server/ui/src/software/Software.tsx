// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/software/Software.tsx
 *
 * Maintenance → Software update as a part, built once and updated in
 * place: the update channel for a holder of settings, saved at once, the
 * automatic update at night and its window, the running version and
 * whether a newer release is out, the automatic update's question
 * (Ask.tsx), the release's
 * notes, the helper's last state, the last three updates, Check for
 * updates and Update, which asks in place of the buttons, saying that the
 * server stays down until the settings file is fixed while its next
 * start refuses it. A load that comes in leaves the channel's choice,
 * its focus and the question where they are; the card's badge says up to
 * date or what is out, also while the card is folded.
 */

import type { Ref } from "preact";
import { useLayoutEffect, useRef, useState } from "preact/hooks";
import type { Setting, UpdateEntry, UpdateInfo, UpdateStatus } from "../api";
import { hid } from "../shared/cls";
import { Field, Select } from "../shared/field";
import { BusyButton, busyProps, Question, useAsk, useSetAtOnce } from "../shared/question";
import { fmtServerClock } from "../shared/time";
import { update } from "../shared/update";
import { ask, data, loadUpdate, question, runUpdate, setAuto, setChannel, setWindow, UPDATE_FLAVOURS } from "./store";
import { asked, AskButtons, askText, updateQuestion } from "./Ask";
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
 * historyLine is one of the last updates: when; the version reached, or
 * the one asked for with how far it got (under way for the newest, else
 * not reached), the version with its channel in brackets, and the
 * helper's outcome where it has one, a rollback after the version reached
 * included; and by whom, automatically where the server started it at
 * night, or from outside Massimo with the package manager.
 */
function historyLine(e: UpdateEntry, newest: boolean): string {
  const withChannel = (version: string) => (version && e.channel ? t("update.history.withChannel", { version, channel: e.channel }) : version);
  const version = withChannel(e.reached || e.to || "");
  let what: string;
  if (e.result) what = t("update.history.ended", { version, state: stateText(e.result) });
  else if (e.reached) what = version;
  else what = t(newest ? "update.history.running" : "update.history.notReached", { version });
  const parts: string[] = [];
  const ms = Date.parse(e.reached_at || e.at || "");
  if (!isNaN(ms)) parts.push(fmtInstant(new Date(ms))); // app.js
  parts.push(what);
  if (e.outside) parts.push(e.manager ? t("update.history.outsideWith", { manager: e.manager }) : t("update.history.outside"));
  else if (e.auto) parts.push(t("update.history.auto"));
  else if (e.by) parts.push(t("update.history.by", { name: e.by.display_name || e.by.name || "" }));
  return parts.join(" · ");
}

/**
 * versionLine says the running version and whether a newer release is
 * out on the channel: on a box apt or brew updates, one the package
 * source has; GitHub's newest, where the source has it not yet; or that
 * the source could not be read. A version the update helper found
 * nothing to install for is held back until a check finds it.
 */
function versionLine(u: UpdateInfo): string {
  const version = u.current || t("update.unknownVersion");
  const manager = u.manager || "";
  const parts = [u.channel ? t("update.runningChannel", { version, channel: u.channel }) : t("update.running", { version })];
  // a package manager owns the binary: the Debian package's helper installs through apt
  if (manager) parts.push(t("update.manager", { manager }));
  if (u.check_error) parts.push(t("update.checkFailed", { error: u.check_error }));
  else if (u.source_error) parts.push(t("update.sourceFailed", { manager, error: u.source_error }));
  else if (!u.channel) parts.push(t("update.noChannel"));
  else if (!u.latest && !u.released) parts.push(t("update.noRelease"));
  else if (u.held) parts.push(t("update.held", { version: u.held }));
  else if (u.available) parts.push(t("update.available", { version: u.latest || "" }));
  else if (!u.released) parts.push(t("update.upToDate"));
  if (u.released && u.released !== u.held && !u.check_error && !u.source_error) parts.push(t("update.released", { version: u.released, manager }));
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

/**
 * Auto is the automatic update's choice, for a holder of settings, who
 * alone may change it: Off or At night (update.auto), saved at once as
 * the channel is, its note saying what it does; at night its window
 * (Window), and under it when the box will try next, or, where the
 * server says the try waits for the settings file (auto.waits), that it
 * waits until the file is fixed. Both are off, with the reason, where
 * updates cannot be started from here (blocked).
 */
function Auto({ u, auto, win }: { u: UpdateInfo; auto: Setting; win: Setting }) {
  useLook();
  const mode = useSetAtOnce(auto.value || "off");
  const stored = win.value || "03:00-05:00";
  const [from, to] = stored.split("-");
  const off = !!u.blocked;
  const night = mode.value === "night";
  const said = data.autoSaid;
  let note = night ? t("update.autoNight", { from: from || "", to: to || "" }) : t("update.autoOff");
  if (off) note = t("update.autoBlocked", { reason: u.blocked || "" });
  if (said) note = said();
  const a = u.auto;
  let hint = "";
  if (night && !off && a && a.next) {
    hint = a.waits === "settings" ? t("update.autoWaitsSettings") : t("update.autoNext", { time: fmtServerClock(a.next) });
  }
  const options: [string, string][] = [["off", t("update.auto.off")], ["night", t("update.auto.night")]];
  return (
    <>
      <Select label={t("update.auto")} id="update-auto" wide options={options} value={mode.value} disabled={off}
        hint={note} hintId="update-auto-note" onChange={(v) => mode.set(v, setAuto)} />
      {night && <Window stored={stored} off={off} hint={hint} />}
    </>
  );
}

/**
 * Window is the automatic update's window, its start and its end as
 * times on the server's clock: an end is saved as its field is left
 * changed, the window shown as chosen until the server's answer shows
 * what it keeps (a window it refuses, too short, goes back, and the
 * automatic update's note says why); an end left while the other's save
 * runs is saved after it. Under it hint, Auto's line of the next try.
 */
function Window({ stored, off, hint }: { stored: string; off: boolean; hint: string }) {
  const [from, to] = stored.split("-");
  const [edit, setEdit] = useState<{ from: string; to: string } | null>(null);
  // the edit as the fields hold it, and the save that runs, outside the drawing
  const held = useRef<{ from: string; to: string } | null>(null);
  const sending = useRef(false);
  const shown = edit || { from: from || "", to: to || "" };
  const change = (v: { from: string; to: string }) => {
    held.current = v;
    setEdit(v);
  };
  const leave = () => {
    const e = held.current;
    if (!e || sending.current) return;
    const w = e.from + "-" + e.to;
    if (w === stored || !e.from || !e.to) {
      held.current = null;
      setEdit(null);
      return;
    }
    sending.current = true;
    void setWindow(w).finally(() => {
      sending.current = false;
      if (held.current === e) {
        held.current = null;
        setEdit(null);
      } else leave(); // changed again while it saved
    });
  };
  return (
    <Field label={t("update.window")} id="update-window-start" wide hint={hint} hintId="update-auto-next">
      <span class="inline field-control">
        <input id="update-window-start" type="time" value={shown.from} disabled={off} aria-describedby={hint ? "update-auto-next" : undefined}
          onInput={(ev) => change({ ...shown, from: ev.currentTarget.value })} onBlur={leave} />
        <span class="muted">–</span>
        <input id="update-window-end" type="time" value={shown.to} disabled={off} aria-label={t("update.windowTo")}
          onInput={(ev) => change({ ...shown, to: ev.currentTarget.value })} onBlur={leave} />
      </span>
    </Field>
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
  const askedVersion = asked();
  return (
    <>
      {u && setting && can("settings") && <Channel u={u} setting={setting} />}
      {u && data.auto && data.window && can("settings") && <Auto u={u} auto={data.auto} win={data.window} />}
      <div ref={asking.row as Ref<HTMLDivElement>} class="update" aria-live="polite">
        <p id="update-text">{failed ? failed() : u ? versionLine(u) : "–"}</p>
        {askedVersion && (
          // the automatic update's question, as the banner under the header asks it (Ask.tsx)
          <div id="update-card-ask" class="update-card-ask">
            <span>{askText(askedVersion)}</span> <AskButtons prefix="update-card-ask" version={askedVersion} />
          </div>
        )}
        <p>
          <a id="update-link" class={u && u.release_url ? undefined : "hidden"} href={(u && u.release_url) || undefined} target="_blank" rel="noopener noreferrer">
            {u && u.release_url ? t("update.notes", { version: u.released || u.latest || "" }) : ""}
          </a>
        </p>
        <p id="update-helper" class="muted small">{said ? said() : u && u.helper ? t("update.last", { state: helperText(u.helper) }) : ""}</p>
        <div id="update-history" class={"update-history" + hid(!history.length)}>
          <h4>{t("update.history")}</h4>
          <ul id="update-history-list">{history.slice(0, 3).map((e, i) => <li key={i}>{historyLine(e, i === 0)}</li>)}</ul>
        </div>
        <div class={"actions" + hid(asking.asking)}>
          <BusyButton id="update-check" label={t("update.check")} off={busy} run={() => loadUpdate(true)} />
          <button ref={asking.ref} type="button" id="update-btn" {...busyProps("primary needs-update" + hid(!u || !u.can_update), busy || question.reading)} onClick={ask}>{u ? t("update.to", { version: u.latest || "" }) : t("update.button")}</button>
        </div>
        {asking.asking && (
          <Question text={updateQuestion(u, version, question.refused)}
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
