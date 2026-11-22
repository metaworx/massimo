// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/Unlisted.tsx
 *
 * The devices without a room, at the end of Rooms: a device just paired,
 * or one the original software paired through the proxy and never put
 * into a room, each its device's row with Put into a room… and Remove from
 * the Cube (capability structure). The form that puts it into a room is
 * the shared PutForm, as the pairing has it, over the chosen Cube's rooms;
 * Remove from the Cube asks in the row. Built once per device and updated
 * in place, so that the form keeps what is typed and the focus when data
 * comes in.
 */

import { useLayoutEffect, useRef, useState } from "preact/hooks";
import type { Device } from "../api";
import { actionRunner } from "../shared/action";
import { newPut, PutForm } from "../shared/put";
import { Question, useAsk } from "../shared/question";
import { session, stillSession } from "../shared/session";
import { DeviceRow } from "./Devices";
import { useLook } from "../shared/look";
import { removeFromCube } from "./send";

/**
 * said says under the devices without a room how the last action went,
 * the device's row being gone after it, or on the page's message line
 * when the section went with its last device; a success fades. It only
 * says, never runs: the removal's answer and the put form's after a
 * sign-out are dropped where they come (below, and shared/put.tsx).
 */
const said = actionRunner({ where: () => ($("unlisted")?.classList.contains("hidden") ? $("message") : $("unlisted-msg")), fade: RESULT_FADE_MS });

/**
 * Put is the form that puts the device into a room (the shared PutForm)
 * over the chosen Cube's rooms, the focus in its first field when it
 * opens; what is typed is kept while it is open. Its outcome is said
 * under the devices without a room, the row being gone after it, and so
 * is a refusal where the device went into its room all the same (a copy
 * that stopped).
 */
function Put({ d, done, cancel }: { d: Device; done: () => void; cancel: () => void }) {
  const [put] = useState(newPut);
  return (
    <PutForm id={"pair-" + (state.roomsOf || "") + "-" + d.rf_address} serial={state.roomsOf || ""} dev={d} rooms={state.rooms} put={put} reload={loadRooms}
      focus onCancel={cancel}
      onDone={() => {
        done();
        if (put.result) said.say(put.result.text, true);
      }}
      onFail={() => {
        if (!state.unlisted.some((x) => x.rf_address === d.rf_address) && put.result) said.say(put.result.text, false);
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
  const back = useRef(false);
  // Cancel puts the focus back on Put into a room… in the drawing that shows it again
  useLayoutEffect(() => {
    if (form || !back.current) return;
    back.current = false;
    put.current?.focus();
  }, [form]);
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
        back.current = true;
        setForm(false);
      }} />}
      {ask.asking && (
        <Question text={t("device.removeConfirm", { device: label })} yes={t("device.remove")} no={t("action.keep")} onNo={() => ask.done()}
          onYes={async () => {
            const n = session();
            const res = await removeFromCube(d);
            ask.done(false);
            if (res && stillSession(n)) said.say(res.text, res.ok); // an answer after a sign-out says nothing
          }} />
      )}
    </DeviceRow>
  );
}

/** Unlisted is the list of the devices without a room, in #unlisted-devices; its section is shown and hidden by rooms.js. */
export function Unlisted() {
  useLook(); // one of them reported: state.unlisted, a signal (shared/data.ts)
  return <>{state.unlisted.map((d) => <UnlistedRow key={(state.roomsOf || "") + ":" + d.rf_address} d={d} />)}</>;
}
