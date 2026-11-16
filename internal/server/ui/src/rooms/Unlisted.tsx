// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/Unlisted.tsx
 *
 * The devices without a room, at the end of Rooms: a device just paired,
 * or one the original software paired through the proxy and never put
 * into a room, each its device's row with Put into a room… and Remove from
 * the Cube (capability structure). The form that puts it into a room stays
 * plain (pairing.js pairDeviceForm, AP UiParts Block 9) in an element the
 * part leaves to it; Remove from the Cube asks in the row. Built once per
 * device and updated in place, so that the form keeps what is typed and
 * the focus when data comes in.
 */

import { useRef, useState } from "preact/hooks";
import type { Device } from "../api";
import { actionRunner } from "../shared/action";
import { Island } from "../shared/island";
import { Question, useAsk } from "../shared/question";
import { DeviceRow } from "./Devices";
import { load, tickOf } from "./store";

/**
 * said says under the devices without a room how the last action went,
 * the device's row being gone after it, or on the page's message line
 * when the section went with its last device; a success fades.
 */
const said = actionRunner({ where: () => ($("unlisted")?.classList.contains("hidden") ? $("message") : $("unlisted-msg")), fade: RESULT_FADE_MS });

/** The key of the devices without a room's counter (tickOf), which a state event of one of them raises (MassimoParts.rooms.unlisted). */
export const UNLISTED = -1;

/**
 * Put is the form that puts the device into a room, plain (pairDeviceForm,
 * an Island), filled when it opens, again where the chosen Cube's rooms or
 * the language changed; what is typed is kept across a fill (e). The focus
 * goes to its first field when it opens.
 */
function Put({ d, done, cancel }: { d: Device; done: () => void; cancel: () => void }) {
  const e = useRef<Record<string, unknown>>({});
  const focus = useRef(true);
  const rooms = state.rooms.map((r) => r.id + ":" + r.name).join("|");
  return (
    <Island cls="part-slot" keys={[d.rf_address, state.serial, rooms]} fill={(el) => {
      const edit = e.current as { result?: { text: string; ok: boolean } };
      el.replaceChildren(pairDeviceForm(state.serial || "", d, e.current, {
        onDone: () => {
          done();
          if (edit.result) said.say(edit.result.text, true);
        },
        onFail: () => {
          // A copy that stopped put the device into its room all the same.
          if (!state.unlisted.some((x) => x.rf_address === d.rf_address) && edit.result) said.say(edit.result.text, false);
        },
        onCancel: cancel,
      }));
      if (focus.current) el.querySelector<HTMLElement>("form select, form input")?.focus();
      focus.current = false;
    }} />
  );
}

/**
 * UnlistedRow is a device without a room: its row with Put into a room…
 * and Remove from the Cube while neither is under way, and under it the
 * form that puts it into a room or the question whether to remove it;
 * Cancel and Keep put the focus back on the button that opened them.
 */
function UnlistedRow({ d }: { d: Device }) {
  const [form, setForm] = useState(false);
  const ask = useAsk<HTMLButtonElement>();
  const put = useRef<HTMLButtonElement>(null);
  const label = deviceName(d);
  const actions = !form && !ask.asking && (
    <>
      <button ref={put} type="button" class="needs-structure put-open" aria-label={t("pairing.putOpenLabel", { device: label })}
        onClick={() => setForm(true)}>{t("pairing.putOpen")}</button>
      <button ref={ask.ref} type="button" class="needs-structure remove-device" aria-label={t("device.removeLabel", { device: label })}
        onClick={ask.ask}>{t("device.remove")}</button>
    </>
  );
  return (
    <DeviceRow d={d} actions={actions}>
      {form && <Put d={d} done={() => setForm(false)} cancel={() => {
        setForm(false);
        setTimeout(() => put.current?.focus(), 0); // once the row shows its buttons again
      }} />}
      {ask.asking && (
        <Question text={t("device.removeConfirm", { device: label })} yes={t("device.remove")} no={t("action.keep")} onNo={() => ask.done()}
          onYes={async () => {
            const res = await removeFromCube(d);
            ask.done(false);
            said.say(res.text, res.ok);
          }} />
      )}
    </DeviceRow>
  );
}

/** Unlisted is the list of the devices without a room, in #unlisted-devices; its section is shown and hidden by rooms.js. */
export function Unlisted() {
  load.use();
  void tickOf(UNLISTED).n; // drawn again when one of them reported
  return <>{state.unlisted.map((d) => <UnlistedRow key={(state.roomsOf || "") + ":" + d.rf_address} d={d} />)}</>;
}
