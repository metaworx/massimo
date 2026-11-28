// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/maintenance/Limits.tsx
 *
 * Maintenance → Limits as a part (AP ServerLimits), built once and
 * updated in place, for a holder of settings: how long a window may stay
 * open before its sign warns, and before it alerts (Block 1); how long a
 * radiator and a wall thermostat may report nothing new before they count
 * as stale, and how long the log keeps the Cubes' events (Block 2). Each
 * is a list of steps saved at once and kept on while it saves, so that
 * the focus stays on it (D3-A), as the Cube intervals are: its first
 * choice the server's default, named with its value and sent as null, a
 * stored value that is no step listed as itself, a line under it saying
 * what it does, and under that how its last change went, a refusal in
 * the page's language. The steps the server would refuse are listed but
 * off, by the other limit's value in effect, equal allowed, so that the
 * alert never comes before the warning (D4-A). A threshold a flag fixes
 * for the run (-stale-after) is read-only, its line naming the flag; a
 * shorter log asks first in the row, since the entries it removes go at
 * once (D7-A), and Forever or a longer one is saved at once. The focus
 * stays on the list while it asks, so that the arrow keys move on through
 * the choices, each of which a closed list takes at once in Chromium off
 * the Mac: the question follows the choice, a choice that shortens
 * nothing takes it back, Tab reaches Cancel and Shorten, Escape is
 * Cancel; Shorten waits, off, while the row's save of an earlier choice
 * runs. Its store takes the question back where a read or a save leaves
 * its choice shortening nothing.
 */

import type { Ref } from "preact";
import { ResultLine } from "../shared/badge";
import { Select, type SelectOption } from "../shared/field";
import { useLook } from "../shared/look";
import { Question, useAsk, useSetAtOnce } from "../shared/question";
import { intervalLabel } from "./Maintenance";
import { LIMIT_DEFAULT, LIMITS, type LimitSetting, limitAsk, limitLowers, limitOrder, limitRefused, limitValue, saveLimit, settings } from "./store";

/** The seconds of a day. */
const DAY = 86400;

/**
 * limitLabel is a limit's value as its list names it: a duration in
 * minutes, in hours up to two days and in days from three; the log's
 * days in days, in years from two, Forever for 0.
 */
function limitLabel(lim: LimitSetting, n: number): string {
  if (isNaN(n)) return "–";
  if (lim.days) {
    if (n === 0) return t("limits.forever");
    return n >= 730 && n % 365 === 0 ? fmtUnit(n / 365, "year") : fmtUnit(n, "day");
  }
  if (n >= 3 * DAY && n % DAY === 0) return fmtUnit(n / DAY, "day");
  if (n >= 3600 && n % 3600 === 0) return fmtUnit(n / 3600, "hour");
  return intervalLabel(n);
}

/** limitHint is the line under a row: what it does, and where a flag fixes it for the run, the flag and its value. */
function limitHint(lim: LimitSetting): string {
  const v = settings.limits[lim.key];
  const what = t(lim.hint);
  if (!v || !v.fixed_by) return what;
  const fixed = t("maint.intervalFixed", { by: v.fixed_by, value: limitLabel(lim, limitValue(lim, v.in_effect)) });
  return what + " " + fixed.replace(/\.\.$/, "."); // a unit that ends in a full stop ("2 Min.") ends the sentence too
}

/**
 * LimitRow is one limit: Default with its value, the steps with a
 * stored value that is none of them added, each step the server would
 * refuse off; read-only where a flag fixes it; the line saying what it
 * does, which the select names as its description; the log's question
 * where a choice shortens it, the choice shown while it asks and the
 * focus left on the list; how its last change went.
 */
function LimitRow({ lim }: { lim: LimitSetting }) {
  useLook();
  const v = settings.limits[lim.key] || null;
  const value = v ? limitValue(lim, v.value) : NaN;
  const stored = !v || isNaN(value) ? "" : v.set ? String(value) : LIMIT_DEFAULT;
  const choice = useSetAtOnce(stored);
  const asking = useAsk<HTMLSelectElement>(lim.asksLower ? limitAsk : undefined);
  const def = v ? limitValue(lim, v.default) : NaN;
  const steps = !v || !v.set || isNaN(value) || lim.steps.includes(value) ? lim.steps : [...lim.steps, value].sort(limitOrder(lim));
  const options: SelectOption[] = !v ? [] : [
    [LIMIT_DEFAULT, t("limits.default", { value: limitLabel(lim, def) }), limitRefused(lim, def)],
    ...steps.map((n): SelectOption => [String(n), limitLabel(lim, n), limitRefused(lim, n)]),
  ];
  const save = (c: string) => choice.set(c, (x) => saveLimit(lim, x));
  const named = (c: string) => limitLabel(lim, c === LIMIT_DEFAULT ? def : Number(c));
  return (
    <div ref={asking.row as Ref<HTMLDivElement>} class="limit-row needs-settings" onKeyDown={(ev) => {
      // Escape on the list while it asks is Cancel, as in the question's box (which handles its own)
      if (ev.key !== "Escape" || !asking.asking) return;
      ev.preventDefault();
      asking.done();
    }}>
      <Select label={t(lim.title)} id={lim.id} labelCls="action-title" value={asking.asking ? limitAsk.choice : choice.value} options={options}
        disabled={!!(v && v.fixed_by)} inputRef={asking.ref} hint={limitHint(lim)} hintId={lim.id + "-text"}
        onChange={(x) => {
          if (limitLowers(lim, x)) {
            limitAsk.choice = x;
            asking.ask();
            return;
          }
          if (asking.asking) asking.done(false); // a choice that shortens nothing takes the question back
          save(x);
        }} />
      {asking.asking && (
        <Question text={t("limits.keepAsk", { value: named(limitAsk.choice) })} yes={t("limits.keepShorten")} no={t("action.cancel")} boxRef={asking.box}
          stay off={choice.busy} onNo={() => asking.done()}
          onYes={() => {
            const c = limitAsk.choice;
            asking.done();
            save(c);
            return Promise.resolve();
          }} />
      )}
      <ResultLine id={lim.id + "-msg"} cls="small cubes-msg" role="status" res={settings.limitSaid[lim.key]} />
    </div>
  );
}

/** Limits is the card's body: its rows in their order. */
export function Limits() {
  useLook();
  return <>{LIMITS.map((lim) => <LimitRow key={lim.key} lim={lim} />)}</>;
}
