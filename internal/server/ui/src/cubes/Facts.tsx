// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/Facts.tsx
 *
 * A Cube's facts at the top of its panel: where it is, what it runs, its
 * clock and its radio budget; the serial, the address and the radio
 * address copy with a click on them or on their copy icon.
 */

import type { Cube } from "../api";
import { hostPort } from "../model/cube";
import { Copy } from "../shared/copy";

/** Facts is the table of a Cube's facts, "–" for one not known. */
export function Facts({ c }: { c: Cube }) {
  const { host, port } = hostPort(c.address);
  const f = budgetFacts(c);
  const clock = clockText(c);
  const rows: [string, string, string, boolean, boolean][] = [
    // [fact, label, value, monospaced, copies]
    ["serial", t("cubes.serial"), c.serial, true, true],
    ["host", t("budget.address"), host, true, true],
    ["port", t("cubes.port"), port, true, false],
    ["firmware", t("budget.firmware"), c.firmware || "", false, false],
    ["radio", t("budget.radio"), c.rf_address || "", true, true],
    ["clock", t("budget.clock"), clock.text, false, false],
    ["duty", t("budget.duty"), f.duty, false, false],
    ["slots", t("budget.slots"), f.slots, false, false],
    ["pending", t("budget.pending"), f.pending, false, false],
    ["queued", t("budget.queued"), f.queued, false, false],
  ];
  return (
    <table class="info-table cube-info">
      <tbody>
        {rows.map(([fact, label, value, mono, copies]) => (
          <tr key={fact}>
            <th scope="row">{label}</th>
            <td class={mono ? "mono" : undefined} data-fact={fact} data-copy={copies ? "" : undefined} title={fact === "clock" ? clock.title : undefined}>
              {value && copies ? <Copy text={value} label={label} icon /> : value || "–"}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
