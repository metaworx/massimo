// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/Pairing.tsx
 *
 * A Cube's pairing in its panel (AP UiParts, Block 9), for a role with
 * pair: Pair a device; the time and Start; while pairing runs, the
 * countdown and Stop; the last outcome; and each device that joined, its
 * type and serial with the form that puts it into a room (the shared
 * PutForm), or how that went. Built once per Cube and updated in place
 * from its flow (flow.ts): the countdown changes its text, a pairing
 * event or a device that joins adds what is new, and nothing the user
 * holds is made again.
 */

import { useEffect, useRef } from "preact/hooks";
import type { NewDevice } from "../api";
import { ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { DeviceIds } from "../shared/device";
import { FormFooter, NumberField } from "../shared/field";
import { PutForm } from "../shared/put";
import { busyProps } from "../shared/question";
import { cubeRooms, flowOf, loadCubeRooms, secondsOk, startPairing, stopPairing } from "./flow";
import { useLook } from "../shared/look";

/**
 * Joined is a device that joined: its type, serial and radio address,
 * and the form that puts it into one of the Cube's rooms, or how that
 * went once it is in its room.
 */
function Joined({ serial, d }: { serial: string; d: NewDevice }) {
  useLook();
  const put = flowOf(serial).puts[d.rf_address];
  const rooms = cubeRooms.value[serial] || [];
  // The rooms are read again; for the Cube shown, Rooms' tiles and devices without a room too.
  const reload = () => (serial === state.serial ? Promise.all([loadRooms(), loadCubeRooms(serial)]) : loadCubeRooms(serial));
  return (
    <div class="pair-device" data-rf={d.rf_address}>
      <div class="pair-device-head">
        <span class="pair-device-type">{typeName(d.type)}</span>
        <DeviceIds serial={d.serial} rf={d.rf_address} />
      </div>
      {!put ? null : put.done ? <ResultLine res={put.result} cls="small pair-result" role="status" />
        : <PutForm id={"pair-" + serial + "-" + d.rf_address} serial={serial} dev={d} rooms={rooms} put={put} reload={reload} />}
    </div>
  );
}

/**
 * Pairing is a Cube's pairing, in the panel's .cube-pairing, which lends
 * its parts to the panel's row of actions: Pair a device opens the time
 * with Start and puts the focus into the time. The time takes 1 to 255 s,
 * the longest a Cube keeps (cube.MaxInclusion); one it refuses is marked
 * as it is typed, and Start then goes back to the field instead.
 */
export function Pairing({ serial }: { serial: string }) {
  useLook();
  const f = flowOf(serial);
  const secs = useRef<HTMLInputElement>(null);
  const focus = useRef(false);
  // Pair a device asked for the time: the focus goes into it once it is shown.
  useEffect(() => {
    if (!f.form || !focus.current) return;
    focus.current = false;
    secs.current?.focus();
  }, [f.form]);
  const ok = secondsOk(f.seconds);
  const left = f.left;
  return (
    <div class="cube-pairing needs-pair">
      <button type="button" class={"pair-open" + hid(f.form || f.active)} onClick={() => {
        f.form = true;
        f.msg = null;
        focus.current = true;
      }}>{t("pairing.open")}</button>
      <form class={"pairing-form" + hid(!f.form || f.active)} noValidate onSubmit={(ev) => {
        ev.preventDefault();
        if (!ok) {
          secs.current?.focus();
          return;
        }
        startPairing(serial);
      }}>
        <NumberField label={t("pairing.seconds")} id={"pair-seconds-" + serial} name="seconds" min={1} max={255} step={1} unit={t("settings.seconds")}
          value={f.seconds} set={(v) => {
            f.seconds = v;
          }} inputRef={secs} limit={{ text: t("pairing.secondsRange"), over: !ok, cls: "pairing-range" }} />
        <p class="muted small pairing-how">{t("pairing.how")}</p>
        <FormFooter submit={t("pairing.start")} busy={f.sending === "start"} cancel={() => {
          f.form = false;
        }} />
      </form>
      <div class={"pairing-running" + hid(!f.active)}>
        <p class="pairing-countdown" role="status">{f.active ? (left > 0 ? t("pairing.left", { seconds: left }) : t("pairing.ending")) : ""}</p>
        <div class="actions">
          <span class="actions-right">
            <button type="button" {...busyProps("pair-stop", f.sending === "stop")} onClick={() => stopPairing(serial)}>{t("pairing.stop")}</button>
          </span>
        </div>
      </div>
      <ResultLine res={f.msg} cls="small pairing-msg" role="status" />
      <div class="pairing-devices">
        {f.paired.map((d) => <Joined key={d.rf_address} serial={serial} d={d} />)}
      </div>
    </div>
  );
}
