// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/maintenance/Limits.tsx
 *
 * Maintenance → Limits as a part (AP ServerLimits, Block 1), built once
 * and updated in place, for a holder of settings: how long a window may
 * stay open before its sign warns, and before it alerts. Each is a list
 * of steps saved at once and kept on while it saves, so that the focus
 * stays on it (D3-A), as the Cube intervals are: its first choice the
 * server's default, named with its value and sent as null, a stored
 * value that is no step listed as itself, a line under it saying what it
 * does, and under that how its last change went, a refusal in the page's
 * language. The steps the server would refuse are listed but off, by the
 * other limit's value in effect, equal allowed, so that the alert never
 * comes before the warning (D4-A).
 */

import { ResultLine } from "../shared/badge";
import { Select, type SelectOption } from "../shared/field";
import { useLook } from "../shared/look";
import { useSetAtOnce } from "../shared/question";
import { intervalLabel } from "./Maintenance";
import { LIMIT_DEFAULT, LIMITS, type LimitSetting, limitRefused, parseGoDuration, saveLimit, settings } from "./store";

/**
 * LimitRow is one limit: Default with its value, the steps with a
 * stored value that is none of them added, each step the server would
 * refuse off; the line saying what it does, which the select names as its
 * description; how its last change went.
 */
function LimitRow({ lim }: { lim: LimitSetting }) {
  useLook();
  const v = settings.limits[lim.key] || null;
  const value = v ? parseGoDuration(v.value) : NaN;
  const stored = !v || isNaN(value) ? "" : v.set ? String(value) : LIMIT_DEFAULT;
  const choice = useSetAtOnce(stored);
  const def = v ? parseGoDuration(v.default) : NaN;
  const steps = !v || !v.set || isNaN(value) || lim.steps.includes(value) ? lim.steps : [...lim.steps, value].sort((a, b) => a - b);
  const options: SelectOption[] = !v ? [] : [
    [LIMIT_DEFAULT, t("limits.default", { value: intervalLabel(def) }), limitRefused(lim, def)],
    ...steps.map((sec): SelectOption => [String(sec), intervalLabel(sec), limitRefused(lim, sec)]),
  ];
  return (
    <div class="limit-row needs-settings">
      <Select label={t(lim.title)} id={lim.id} labelCls="action-title" value={choice.value} options={options}
        hint={t(lim.hint)} hintId={lim.id + "-text"} onChange={(x) => choice.set(x, (c) => saveLimit(lim, c))} />
      <ResultLine id={lim.id + "-msg"} cls="small cubes-msg" role="status" res={settings.limitSaid[lim.key]} />
    </div>
  );
}

/** Limits is the card's body: its rows in their order. */
export function Limits() {
  useLook();
  return <>{LIMITS.map((lim) => <LimitRow key={lim.key} lim={lim} />)}</>;
}
