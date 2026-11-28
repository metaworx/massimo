// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/maintenance/Maintenance.tsx
 *
 * Maintenance → Maintenance as a part, built once and updated in place,
 * in the owner's order: the announcement on the LAN and the Cube
 * intervals, each saved at once and kept on while it saves, so that the
 * focus stays on it (with settings), and under the refresh's Refresh
 * now, every Cube read again at once (with restart); the settings file
 * read again and the server's restart, which asks in place of its
 * button, with the settings that wait for the next start and what that
 * start refuses, which blocks nothing (with restart); the computer's
 * reboot with its delay, which asks naming it, and Take the reboot back
 * (with reboot). A load that comes in leaves the delay typed, a question and
 * the focus where they are; the card's head counts what waits for the
 * next start and says that start fails while it refuses the settings
 * file, which the restart's and the reboot's questions then say too.
 */

import { Fragment, type Ref } from "preact";
import { useLayoutEffect, useRef } from "preact/hooks";
import type { Setting } from "../api";
import { Notice, ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { NumberField, Select } from "../shared/field";
import { BusyButton, busyProps, Question, useAsk, useSetAtOnce } from "../shared/question";
import {
  cancelReboot, INTERVALS, type Interval, intervalWaits, announceWaits, parseGoDuration, reboot, REBOOT_DELAY_MAX, rebootAsk, rebootDelay, refreshNow, reloadSettings, restart, restartAsk, runReboot, runRefreshNow, runRestart, saveInterval, setAnnounce, settings, waiting,
} from "./store";
import { type BadgeSpec, cardBadges } from "../shared/card";
import { useLook } from "../shared/look";

/** intervalLabel is seconds in the language's short units ("10 s", "6 h", "1 min, 30 s"), Off for 0; the Limits card's steps too (Limits.tsx). */
export function intervalLabel(sec: number): string {
  if (isNaN(sec)) return "–";
  if (sec === 0) return t("maint.off");
  const parts: string[] = [];
  for (const [unit, n] of [["day", 86400], ["hour", 3600], ["minute", 60], ["second", 1]] as [string, number][]) {
    if (sec >= n) {
      parts.push(fmtUnit(Math.floor(sec / n), unit)); // app.js
      sec %= n;
    }
  }
  return parts.join(", ");
}

/**
 * Announce is the announcement on the LAN: Automatic, On or Off, and what
 * it does, whether the server announces itself now, that a change waits
 * for the next start, or, where -announce or MASSIMO_ANNOUNCE decides, the
 * choice read-only with what sets it (AP AnnounceSetting, D15-A).
 */
function Announce() {
  useLook();
  const v = settings.announce;
  const choice = useSetAtOnce(v ? v.value || "auto" : "auto");
  let text = "";
  if (v) {
    const now = t(v.in_effect === "on" ? "maint.announceNowOn" : "maint.announceNowOff");
    text = v.fixed_by ? t("maint.announceFixed", { by: v.fixed_by }) + " " + now : t("maint.announceHint") + " " + (announceWaits(v) ? t("maint.announceWaits") : now);
  }
  return (
    <div class="announce-row needs-settings">
      <Select label={t("maint.announce")} id="announce-mode" labelCls="action-title" value={choice.value} disabled={!!(v && v.fixed_by)}
        options={[["auto", t("maint.announceAuto")], ["on", t("maint.announceOn")], ["off", t("maint.announceOff")]]}
        onChange={(x) => choice.set(x, setAnnounce)} />
      <p id="announce-text" class="muted small field-hint">{text}</p>
      <ResultLine id="announce-msg" cls="small cubes-msg" role="status" res={settings.announceSaid} />
    </div>
  );
}

/** stateLine says what an interval does and this run's value, that a change waits for the restart, or what sets it. */
function stateLine(iv: Interval, v: Setting): string {
  const now = intervalLabel(parseGoDuration(v.in_effect));
  const state = v.fixed_by ? t("maint.intervalFixed", { by: v.fixed_by, value: now })
    : intervalWaits(v) ? t("maint.intervalWaits", { value: now })
      : t("maint.intervalNow", { value: now });
  // a unit that ends in a full stop ("10 Sek.") ends the sentence too
  return t(iv.hint) + " " + state.replace(/\.\.$/, ".");
}

/**
 * IntervalRow is one Cube interval: its steps, a stored value that is
 * none of them added, the line saying what it does; read-only with what
 * sets it where a flag decides (as the announcement, D15-A).
 */
function IntervalRow({ iv }: { iv: Interval }) {
  useLook();
  const v = settings.intervals[iv.key] || null;
  const value = v ? parseGoDuration(v.value) : NaN;
  const choice = useSetAtOnce(v ? String(value) : "");
  const steps = !v ? [] : isNaN(value) || iv.steps.includes(value) ? iv.steps : [...iv.steps, value].sort((a, b) => a - b);
  return (
    <div class="interval-row needs-settings">
      <Select label={t(iv.title)} id={iv.id} labelCls="action-title" value={choice.value} disabled={!!(v && v.fixed_by)}
        options={steps.map((sec): [string, string] => [String(sec), intervalLabel(sec)])}
        onChange={(x) => choice.set(x, (s) => saveInterval(iv, Number(s)))} />
      <p id={iv.id + "-text"} class="muted small field-hint">{v ? stateLine(iv, v) : ""}</p>
      <ResultLine id={iv.id + "-msg"} cls="small cubes-msg" role="status" res={settings.intervalSaid[iv.key]} />
    </div>
  );
}

/**
 * RefreshNow is the row under the refresh's interval (with restart, so
 * shown to a manager without the interval): Refresh now reads every Cube
 * the server serves again at once, one after the other, turning until
 * the last has answered, and each Cube's outcome shows under the row as
 * its answer comes (AP RefreshBatteries, D10-B).
 */
function RefreshNow() {
  useLook();
  const cubes = state.cubes;
  const said = refreshNow.said;
  return (
    <div class="action-row refresh-now needs-restart">
      <div>
        <p class="action-title">{t("maint.refreshNowTitle")}</p>
        <p class="muted small">{t("maint.refreshNowText")}</p>
        <div class="refresh-results">
          {cubes.map((c) => <ResultLine key={c.serial} cls="small cubes-msg refresh-result" role="status" res={said[c.serial]} />)}
        </div>
      </div>
      <div class="actions">
        <button type="button" id="refresh-now" {...busyProps("", refreshNow.running, !cubes.length)} onClick={runRefreshNow}>{t("maint.refreshNow")}</button>
      </div>
    </div>
  );
}

/**
 * downUntilFixed is the restart's or the reboot's question, with the
 * sentence that the server stays down until the settings file is fixed
 * while the next start refuses it (AP StartConfiguration, D10-C): both
 * questions promise a server back otherwise.
 */
function downUntilFixed(question: string): string {
  return restart.refused.length ? question + " " + t("restart.confirmRefused") : question;
}

/**
 * Restart is the restart's row: what it does or why it is not offered,
 * its button, which asks in its place; then what waits for it, and what
 * the next start refuses, a line each, the server's text as text: the
 * owner reads it where the restart is asked for, and the button stays
 * (AP ReloadChecks, D8-B), its question saying the server stays down.
 */
function Restart() {
  useLook();
  const asking = useAsk<HTMLButtonElement>(restartAsk);
  const text = restart.text;
  const pending = restart.pending;
  const refused = restart.refused;
  return (
    <>
      <div ref={asking.row as Ref<HTMLDivElement>} class="action-row restart needs-restart">
        <div>
          <p class="action-title">{t("maint.restart")}</p>
          <p id="restart-text" class={"muted small" + hid(!text)}>{text ? text() : ""}</p>
        </div>
        <div class={"actions" + hid(asking.asking)}>
          <button ref={asking.ref} type="button" id="restart-btn" class={restart.shown ? undefined : "hidden"} disabled={restart.off} onClick={asking.ask}>
            {t("restart.button")}
          </button>
        </div>
        {asking.asking && (
          <Question text={downUntilFixed(t("restart.confirm"))} yes={t("restart.button")} no={t("action.cancel")} onNo={() => asking.done()} boxRef={asking.box}
            onYes={() => {
              asking.done(false);
              return runRestart();
            }} />
        )}
      </div>
      <Notice id="restart-pending" cls="small needs-restart" hidden={!pending.length} text={pending.length ? t("restart.pending", { names: pending.join(", ") }) : ""} />
      <Notice id="restart-refused" cls="small needs-restart restart-refused" role="status" hidden={!refused.length} text={refused.length ? t("restart.refused") : ""}>
        {refused.map((f, i) => (
          <span key={i} class="restart-refusal">{f.settings && f.settings.length ? t("restart.refusal", { names: f.settings.join(", "), error: f.error }) : f.error}</span>
        ))}
      </Notice>
    </>
  );
}

/**
 * Reboot is the reboot's row: what it does, why it is not offered, when
 * a reboot that waits comes or how the following goes; the delay, which
 * the question names, Reboot, which asks in its place, saying the server
 * stays down while the next start refuses the settings file, and Take the
 * reboot back.
 */
function Reboot() {
  useLook();
  const asking = useAsk<HTMLButtonElement>(rebootAsk);
  const delayRef = useRef<HTMLInputElement>(null);
  const text = reboot.text;
  const start = () => {
    const delay = rebootDelay();
    if (delay === null) {
      reboot.msg = { text: t("reboot.badDelay", { max: REBOOT_DELAY_MAX }), ok: false };
      delayRef.current?.focus();
      return;
    }
    reboot.msg = null;
    rebootAsk.delay = delay;
    asking.ask();
  };
  const delay = rebootAsk.delay;
  return (
    <div ref={asking.row as Ref<HTMLDivElement>} class="action-row reboot needs-reboot">
      <div>
        <p class="action-title">{t("maint.reboot")}</p>
        <p id="reboot-text" class={"muted small" + hid(!text)}>{text ? text() : ""}</p>
        {reboot.shown && (
          <NumberField label={t("maint.rebootDelay")} id="reboot-delay" cls="reboot-delay" min={0} max={REBOOT_DELAY_MAX} step={1} unit={t("unit.seconds")}
            inputMode="numeric" value={reboot.delay} set={(v) => {
              reboot.delay = v;
            }} inputRef={delayRef} />
        )}
        <ResultLine id="reboot-msg" cls="small cubes-msg" role="status" res={reboot.msg} />
      </div>
      <div class={"actions" + hid(asking.asking)}>
        <button ref={asking.ref} type="button" id="reboot-btn" class={reboot.shown ? undefined : "hidden"} disabled={reboot.off} onClick={start}>{t("reboot.button")}</button>
        <BusyButton id="reboot-cancel" cls={reboot.cancel ? "" : "hidden"} label={t("reboot.cancel")} run={cancelReboot} />
      </div>
      {asking.asking && (
        <Question text={downUntilFixed(delay === 0 ? t("reboot.confirmNow") : tn("reboot.confirm", delay))} yes={t("reboot.button")} no={t("action.cancel")}
          onNo={() => asking.done()} boxRef={asking.box}
          onYes={() => {
            asking.done(false);
            return runReboot(delay);
          }} />
      )}
    </div>
  );
}

/** Maintenance is the card's body, with its head's badges. */
export function Maintenance() {
  useLook();
  const n = waiting();
  const badge = n ? tn("maint.badge.pending", n) : "";
  const fails = restart.refused.length ? t("maint.badge.refused") : "";
  // the card's head counts what waits for the next start, and says that start fails while it refuses the settings file (D9-A)
  useLayoutEffect(() => {
    const list: BadgeSpec[] = [];
    if (badge) list.push({ text: badge, kind: "warn" });
    if (fails) list.push({ text: fails, kind: "warn" });
    cardBadges("sys-maint", list);
  }, [badge, fails]);
  const file = restart.file;
  return (
    <>
      <Announce />
      {INTERVALS.map((iv) => (
        <Fragment key={iv.key}>
          <IntervalRow iv={iv} />
          {iv.key === "refresh_interval" && <RefreshNow />}
        </Fragment>
      ))}
      <div class="action-row needs-restart">
        <div>
          <p class="action-title">{t("maint.file")}</p>
          <p id="settings-file-text" class="muted small">{file ? file() : ""}</p>
          <ResultLine id="settings-reload-msg" cls="small cubes-msg" role="status" res={restart.reloaded} />
        </div>
        <div class="actions"><BusyButton id="settings-reload" label={t("maint.reload")} run={reloadSettings} /></div>
      </div>
      <Restart />
      <Reboot />
    </>
  );
}
