// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/notify/Notify.tsx
 *
 * 👤 → Notifications as a part, drawn once into its card and updated in
 * place: why the signed-in user has none, or this browser's state with
 * the name of this device and Turn on or Turn off; a row per kind with
 * its switch and its delay or threshold, each saved a second after the
 * last change, a value out of bounds marked on its field with the range
 * before it and not sent until the field holds one inside; the user's
 * devices with Remove; and the line under the lists. Each rule is saved
 * as it is changed, as notify.js saved it: a rule has no Save, so the
 * card has no marks (UnsavedMarks D6-A).
 */

import { useState } from "preact/hooks";
import type { PushSubscription } from "../api";
import { Badge, Notice, ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { type Limit, NumberField, Switch, TextField, useDraft } from "../shared/field";
import { useLang } from "../shared/lang";
import { me } from "../shared/me";
import { BusyButton } from "../shared/question";
import { fmtWhen } from "../shared/time";
import { removeDevice, rules, saveRules, shown, thisBrowser, turnOff, turnOn, unavailable, unsupported } from "./send";
import { AFTER, BELOW, KINDS, notify, said } from "./store";

/**
 * Value is a rule's delay (after_min, whole minutes) or threshold
 * (below_c), as its field is left: within bounds it is taken and saved;
 * out of them the field is marked and the range shown before it, and
 * nothing is saved until it holds one inside, whatever a save meanwhile
 * says. What is typed is a draft of the rule shown, so that a read of the
 * rules leaves it.
 */
function Value({ kind, k, bounds, step, label, unit }: { kind: string; k: "after_min" | "below_c"; bounds: [number, number]; step: number; label: string; unit: string }) {
  void notify.ver;
  const r = shown()[kind];
  const draft = useDraft(String(r[k] ?? ""), (v) => (v.trim() === "" ? "" : Number(v)));
  const [over, setOver] = useState(false);
  const limit: Limit = { text: t("notify.range", { min: bounds[0], max: bounds[1] }), over };
  return (
    <NumberField label={label} id={"notify-" + kind + "-value"} unit={unit} min={bounds[0]} max={bounds[1]} step={step} limit={limit}
      disabled={!r.on || !notify.loaded} value={draft.value} set={draft.set} onChange={(text) => {
        const v = text === "" ? undefined : Number(text);
        const ok = typeof v === "number" && v >= bounds[0] && v <= bounds[1] && (k !== "after_min" || Number.isInteger(v));
        setOver(!ok);
        if (!ok) return;
        rules()[kind][k] = v;
        notify.ver++;
        saveRules();
      }} />
  );
}

/** RuleRow is a kind's row: its switch, with its help, and its delay or threshold, off while the kind is; both off until the rules were read. */
function RuleRow({ kind }: { kind: string }) {
  void notify.ver;
  const r = shown()[kind];
  return (
    <div class="notify-row" data-kind={kind}>
      <Switch label={t("notify." + kind + ".label")} help={"notify." + kind} checked={r.on} disabled={!notify.loaded} onChange={(on) => {
        rules()[kind].on = on;
        notify.ver++;
        saveRules();
      }} />
      {"after_min" in r && <Value kind={kind} k="after_min" bounds={AFTER} step={1} label={t("notify.after")} unit={t("notify.minutes")} />}
      {"below_c" in r && <Value kind={kind} k="below_c" bounds={BELOW} step={0.5} label={t("notify.below")} unit="°C" />}
    </div>
  );
}

/** Device is one of the user's devices: its name, whether it is this browser, when it was added and last told, the failures since, and Remove. */
function Device({ s }: { s: PushSubscription }) {
  const facts = [t("notify.added", { when: s.created_at ? fmtWhen(s.created_at) : "" })];
  if (s.last_ok_at) facts.push(t("notify.lastOk", { when: fmtWhen(s.last_ok_at) }));
  if (s.failures) facts.push(t("notify.failures", { n: s.failures }));
  return (
    <div class="notify-device">
      <span class="notify-device-name">
        <span>{s.label || t("notify.noLabel")}</span>
        {s.endpoint === notify.endpoint && <> <Badge kind="ok" text={t("notify.thisBrowser")} /></>}
      </span>
      <BusyButton label={t("notify.remove")} run={() => removeDevice(s)} />
      <p class="muted small">{facts.join(" · ")}</p>
    </div>
  );
}

/** ThisDevice is "On this device": whether this browser is told, why it cannot be, its name to be given, and the button that turns it on or off. */
function ThisDevice() {
  void notify.ver;
  const why = unsupported();
  const on = !!thisBrowser();
  return (
    <>
      <h3 class="notify-group">{t("notify.thisDevice")}</h3>
      <p id="notify-device-state" class="small notify-state" role="status">{why || t(on ? "notify.isOn" : "notify.isOff")}</p>
      <div id="notify-label-row" class={hid(!!why || on).trim() || undefined}>
        <TextField label={t("notify.label")} id="notify-label" name="label" help="notify.label" maxLength={100}
          placeholder={t("notify.labelPlaceholder")} value={notify.label} set={(v) => { notify.label = v; }} />
      </div>
      <div class="actions">
        <span class="actions-right">
          <BusyButton id="notify-off" cls={hid(!on).trim()} label={t("notify.turnOff")} run={turnOff} />
          <BusyButton id="notify-on" cls={"primary" + hid(!!why || on)} label={t("notify.turnOn")} run={turnOn} />
        </span>
      </div>
    </>
  );
}

/** NotifyCard is the card's body: why there are none, or this device, the rules, the devices and the line. */
export function NotifyCard() {
  useLang();
  void me.value; // who is in decides whether there are any
  void notify.ver;
  const none = unavailable();
  return (
    <>
      <Notice id="notify-none" role="note" hidden={!none} text={none} />
      <div id="notify-body" class={"notify" + hid(!!none)}>
        {!none && <ThisDevice />}
        <h3 class="notify-group">{t("notify.events")}</h3>
        <div id="notify-rules">{!none && KINDS.map(([kind]) => <RuleRow key={kind} kind={kind} />)}</div>
        <h3 class="notify-group">{t("notify.devices")}</h3>
        <div id="notify-devices">
          {!none && !notify.subs.length && <p class="muted small">{t("notify.noDevices")}</p>}
          {!none && notify.subs.map((s) => <Device key={s.id} s={s} />)}
        </div>
        <ResultLine id="notify-msg" cls="small cubes-msg" role="status" res={said.res} />
      </div>
    </>
  );
}
