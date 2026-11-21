// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/cubes/Form.tsx
 *
 * A Cube's settings in its panel, for a role with cubes: a notice per line
 * of the settings file that differs, with Take the file's value; the
 * fields, each a draft against the value stored (D59-A), kept by the
 * Cube's serial beyond the panel (shared/forms.ts), the proxy's state
 * under its address, the hidden devices with Hide and Show again, and
 * Reset and Save, which says whether the changes are saved.
 */

import { useRef, useState } from "preact/hooks";
import type { Cube, FileDifference } from "../api";
import { cubeFormKey } from "../model/cube";
import { Notice } from "../shared/badge";
import { DeviceIds } from "../shared/device";
import { FormFooter, Switch, TextField, useDraft } from "../shared/field";
import { focusForm, useForm } from "../shared/forms";
import { busyProps, useBusy } from "../shared/question";
import { msg, saveCube } from "./send";
import { cubeUI, deviceOf, devices, load } from "./store";
import { valueText } from "./util";
import { isShown, showView } from "../shared/pages";

/** Differs is a line of the settings file that says something else than the store, with Take the file's value. */
function Differs({ c, d }: { c: Cube; d: FileDifference }) {
  const take = useBusy();
  return (
    <div class="cube-differs needs-cubes">
      <p class="small">{t("cubes.differs", { variable: d.variable, file: valueText(d.file), store: valueText(d.store) })}</p>
      <button type="button" {...busyProps("take-file", take.busy)} data-field={d.field}
        onClick={() => take.run(() => saveCube(c.serial, { [d.field]: d.file }))}>
        {t("cubes.takeFile")}
      </button>
    </div>
  );
}

/** ProxyState says under the proxy address where the Cube's proxy listens, and for a listener that does not, why and since when. */
function ProxyState({ c }: { c: Cube }) {
  return (
    <div class="cube-proxy-state">
      {(c.proxy_failing || []).map((f) => (
        <Notice key={f.addr} text={t("proxy.notListening", { addr: f.addr, error: f.error })} since={f.since} next={f.next_try} />
      ))}
      {(c.proxy_listening || []).length > 0 && (
        <p class="muted small cube-listening">{t("cubes.listening", { addr: (c.proxy_listening || []).join(", ") })}</p>
      )}
    </div>
  );
}

/**
 * Hidden is the list of hidden devices being edited, with Show again per
 * entry, and the field that takes a radio address to hide, the Cube's
 * devices to choose from; list is the draft, set changes it.
 */
function Hidden({ c, list, set }: { c: Cube; list: string[]; set: (l: string[]) => void }) {
  const input = useRef<HTMLInputElement>(null);
  const [typed, setTyped] = useState("");
  const listId = "cube-devices-" + c.serial;
  const offered = (devices.value[c.serial] || []).filter((d) => !list.includes(d.rf_address));
  const hide = () => {
    const rf = typed.trim().toLowerCase();
    if (!/^[0-9a-f]{6}$/.test(rf)) {
      msg(t("cubes.hideBad", { value: typed.trim() }), false);
      return;
    }
    if (!list.includes(rf)) set([...list, rf]);
    msg("");
    setTyped("");
  };
  return (
    <div class="cube-hidden">
      <span class="cube-hidden-title">{t("cubes.hidden")}</span>
      <ul class="cube-hidden-list">
        {!list.length && <li class="muted small">{t("cubes.hiddenNone")}</li>}
        {list.map((rf) => {
          const d = deviceOf(rf);
          const label = d?.name ? d.name + " (" + rf + ")" : rf;
          return (
            <li key={rf}>
              <span class="hidden-device">
                {d?.name && <span>{d.name}</span>}
                <DeviceIds serial={d?.serial} rf={rf} always />
              </span>
              <button type="button" class="subtle" aria-label={t("cubes.unhideLabel", { device: label })} onClick={() => {
                set(list.filter((x) => x !== rf));
                input.current?.focus(); // its own button goes
              }}>
                {t("cubes.unhide")}
              </button>
            </li>
          );
        })}
      </ul>
      <div class="inline cube-hide-add">
        <input ref={input} type="text" name="hide" class="mono" autocomplete="off" spellcheck={false} placeholder="0abc12" aria-label={t("cubes.hideLabel")}
          list={listId} value={typed} onInput={(ev) => setTyped(ev.currentTarget.value)}
          onKeyDown={(ev) => {
            if (ev.key !== "Enter") return;
            ev.preventDefault();
            hide();
          }} />
        <datalist id={listId}>
          {offered.map((d) => <option key={d.rf_address} value={d.rf_address}>{deviceName(d)}</option>)}
        </datalist>
        <button type="button" onClick={hide}>{t("cubes.hide")}</button>
      </div>
    </div>
  );
}

const trim = (v: string) => v.trim();
const upper = (v: string) => v.trim().toUpperCase();
const asSet = (l: string[]) => [...l].sort().join(",");

/**
 * openForm shows a Cube's settings and puts the focus into them: its
 * panel opened in System settings while that page is shown, else Cube
 * settings with the Cube chosen.
 */
function openForm(serial: string): void {
  if (isShown("system")) {
    cubeUI.of(serial).open = true;
  } else {
    if (state.serial !== serial) selectCube(serial);
    if (!isShown("cube")) showView("cube");
  }
  focusForm(cubeFormKey(serial));
}

/**
 * Form is the Cube's settings: the file's differing lines before it, then
 * its fields, each a draft (useDraft) that the shared TextField and Switch
 * draw with its "changed meanwhile" notice: a field the user has not
 * touched follows a value stored meanwhile, by another client or the
 * settings file; one the user changed keeps the draft and says that the
 * stored value changed, with Take theirs. The hidden list is a draft too,
 * for its state alone: it says nothing of a change meanwhile. The drafts
 * are kept by the Cube's serial (useForm), so that what is typed stays
 * when the panel closes and from System settings to Cube settings and
 * back, until it is saved, reset or the Cube is gone; the Cube's problem
 * lists show the disk while it is unsaved. Save sends the fields that
 * differ from the Cube as the list last gave it.
 */
export function Form({ c }: { c: Cube }) {
  load.useLoad();
  const form = useForm({ key: cubeFormKey(c.serial), name: { key: "nav.cubeTitle" }, open: () => openForm(c.serial) });
  const name = useDraft(c.name || "", trim, form.place("name"));
  const address = useDraft(c.configured_address || "", trim, form.place("address"));
  const follow = useDraft(!!c.follow_discovery, undefined, form.place("follow"));
  const proxy = useDraft(c.proxy_address || "", trim, form.place("proxy"));
  const proxySerial = useDraft((c.proxy_serial || "").toUpperCase(), upper, form.place("proxy_serial"));
  const virtual = useDraft(!!c.proxy_virtual, undefined, form.place("virtual"));
  const hidden = useDraft(c.hidden || [], asSet, form.place("hidden"));
  const body: Record<string, unknown> = {};
  if (name.changed) body.name = name.value.trim();
  if (address.changed) body.address = address.value.trim();
  if (follow.changed) body.follow_discovery = follow.value;
  if (proxy.changed) body.proxy_address = proxy.value.trim();
  if (proxySerial.changed) body.proxy_serial = proxySerial.value.trim();
  if (virtual.changed) body.proxy_virtual = virtual.value;
  if (hidden.changed) body.hidden = hidden.value;
  const changed = Object.keys(body).length > 0;
  form.track(changed, [name, address, follow, proxy, proxySerial, virtual, hidden]);
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (!changed) {
      msg(t("cubes.unchanged"), true);
      return;
    }
    form.save(() => saveCube(c.serial, body));
  };
  const id = (field: string) => "cube-" + c.serial + "-" + field;
  const placeholder = !c.configured_address && c.address ? t("cubes.addressFound", { address: c.address }) : t("cubes.addressHint");
  return (
    <>
      {(c.file_differs || []).map((d) => <Differs key={d.field} c={c} d={d} />)}
      <form {...form.props} class="cube-form needs-cubes" onSubmit={submit}>
        <div class="cube-fields">
          <TextField label={t("cubes.name")} id={id("name")} name="name" draft={name} show={valueText} />
          <TextField label={t("cubes.address")} id={id("address")} name="address" draft={address} show={valueText} placeholder={placeholder} />
          <Switch name="follow" label={t("cubes.follow")} draft={follow} show={valueText} />
          <TextField label={t("cubes.proxy")} id={id("proxy")} name="proxy" draft={proxy} show={valueText} placeholder={t("cubes.proxyHint")} />
          <ProxyState c={c} />
          <TextField label={t("cubes.proxySerial")} id={id("proxy_serial")} name="proxy_serial" draft={proxySerial} show={valueText}
            placeholder={t("cubes.proxySerialHint")} />
          <Switch name="proxy_virtual" label={t("cubes.proxyVirtual")} draft={virtual} show={valueText} />
        </div>
        <Hidden c={c} list={hidden.value} set={hidden.set} />
        <FormFooter submit={t("cubes.save")} look={form.look} reset={form.reset} off={!changed} />
      </form>
    </>
  );
}
