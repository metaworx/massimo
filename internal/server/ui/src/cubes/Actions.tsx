// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/Actions.tsx
 *
 * A Cube's actions in one row at the bottom of its panel: Pair a device,
 * with its progress under the row (Pairing.tsx), and Repair… with its
 * box (the shared RepairBox); Reboot the
 * Cube and Remove, each asking in its own place. Reboot's question names
 * the radiograms pending as the Cube says now (D126).
 */

import type { Cube } from "../api";
import { cubePath } from "../shared/path";
import { Question, useAsk } from "../shared/question";
import { RepairBox, RepairButton, repairScope } from "../shared/repair";
import { Pairing } from "./Pairing";
import { rebootCube, removeCube } from "./send";
import { cubeUI } from "./store";

/**
 * Repair is the Cube's Repair… and the box of its repairs (the shared
 * RepairBox) under it.
 */
function Repair({ c }: { c: Cube }) {
  const scope = repairScope(c.serial, "cube");
  return (
    <div class="cube-repair-area needs-repair">
      <div class="actions cube-repair needs-repair">
        <RepairButton scope={scope} kind="cube" path={cubePath(c.serial)} label={c.name || c.serial} />
      </div>
      <RepairBox scope={scope} />
    </div>
  );
}

/**
 * Reboot is the Cube's Reboot, for a role with cubes.reboot: it asks in
 * place of the button, naming what a restart costs, the Cube gone for a
 * minute and its rooms refusing commands for a while, the radiograms
 * pending now and as of when, and for a Cube busy with another client that
 * the client may take it back; Yes sends the count it names.
 */
function Reboot({ c }: { c: Cube }) {
  const ask = useAsk<HTMLButtonElement>();
  const n = c.pending_radiograms || 0;
  if (ask.asking) {
    const parts = [t("cubes.rebootConfirm", { serial: c.serial })];
    if (n > 0) {
      const at = Date.parse(c.budget?.updated_at || "");
      parts.push(tn("cubes.rebootPending", n, { time: isNaN(at) ? "–" : fmtShortTime(at) }));
    }
    if (linkState(c) === "busy") parts.push(t("cubes.rebootBusy"));
    return (
      <Question text={parts.join(" ")} yes={t("cubes.reboot")} no={t("action.cancel")} onNo={() => ask.done()}
        onYes={async () => {
          await rebootCube(c.serial, n);
          ask.done();
        }} />
    );
  }
  return (
    <div class="actions cube-reboot needs-cubes-reboot">
      <button ref={ask.ref} type="button" class="cube-reboot-btn" onClick={ask.ask}>{t("cubes.reboot")}</button>
    </div>
  );
}

/** Remove stops serving the Cube, asking first in place of the button; Keep puts it back. */
function Remove({ serial }: { serial: string }) {
  const ask = useAsk<HTMLButtonElement>();
  if (ask.asking) {
    return (
      <Question text={t("cubes.removeConfirm", { serial })} yes={t("cubes.remove")} no={t("action.keep")} onNo={() => ask.done()}
        onYes={async () => {
          cubeUI.of(serial).open = false;
          if (!(await removeCube(serial))) ask.done();
        }} />
    );
  }
  return (
    <div class="cube-remove needs-cubes">
      <button ref={ask.ref} type="button" class="remove-cube" onClick={ask.ask}>{t("cubes.remove")}</button>
    </div>
  );
}

/** Actions is the row of a Cube's actions; Remove only for a Cube whose settings can be changed here. */
export function Actions({ c, editable }: { c: Cube; editable: boolean }) {
  return (
    <div class="cube-actions">
      <Pairing serial={c.serial} />
      <Reboot c={c} />
      <Repair c={c} />
      {editable && <Remove serial={c.serial} />}
    </div>
  );
}
