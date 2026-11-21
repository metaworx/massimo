// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/admin/Roles.tsx
 *
 * System settings → Roles and Capabilities for every role as parts: a
 * panel per role, built once per role and updated in place, with its
 * settings form, its display name and a switch per capability, kept by
 * the role's key (shared/forms.ts) so that what is changed outlives an
 * action in another panel, a reload of the roles and the panel's closing,
 * with Reset, Save by its state and the disk in the section's pill; its
 * Disable and its Remove, which asks in place of the actions; the form
 * that adds a role; and the switch per capability that disables it for
 * every role but the owner, saved at once and off while its request
 * runs. The owner has every capability and cannot be changed.
 */

import { useEffect, useState } from "preact/hooks";
import type { Role } from "../api";
import { capLabel } from "../model/cap";
import { Adder } from "../shared/adder";
import { Badge, ResultLine } from "../shared/badge";
import { type Choice, ChoiceGroup, Choices } from "../shared/choices";
import { hid } from "../shared/cls";
import { FormFooter, Meanwhile, Switch, TextField, useDraft } from "../shared/field";
import { Fold, FoldHead } from "../shared/fold";
import { focusForm, useForm } from "../shared/forms";
import { BusyButton, Question, busyProps, useAsk, useBusy, useSetAtOnce } from "../shared/question";
import { createRole, disableRole, removeRole, rolesMsg, saveRole, setCapability } from "./sendRoles";
import { adding, load, roleFormKey, roleUI, roles, said } from "./store";

/** openRole shows a role's settings and puts the focus into them, from any page: System settings shown, the role's panel open. */
function openRole(key: string): void {
  if (!isShown("system")) showView("system");
  showCard($("sys-roles")); // a folded card unfolded, its form in sight
  roleUI.of(key).open = true;
  focusForm(roleFormKey(key));
}

/**
 * capChoices are the capabilities as a choice of switches, each named by
 * its key in its tooltip, one disabled for every role marked; lock keeps
 * every one as it is (the owner's set).
 */
function capChoices(lock = false): Choice[] {
  return roles.caps.map((c) => [c.key, capLabel(c.key), false, {
    title: c.key,
    lock,
    badge: c.disabled ? <Badge kind="warn" text={t("roles.capOff")} /> : undefined,
  }]);
}

const trim = (v: string) => v.trim();
const asSet = (l: string[]) => [...l].sort().join(",");

/**
 * RoleForm is a role's settings: its display name and its set, Disable
 * and Remove, Reset and Save; the owner's, every capability on and
 * nothing to change, says so.
 */
function RoleForm({ r }: { r: Role }) {
  load.useLoad();
  const owner = r.key === "owner";
  const form = useForm({ key: roleFormKey(r.key), name: { key: "roles.formName", args: { name: roleLabel(r.key, r.name) } }, open: () => openRole(r.key) });
  const name = useDraft(r.name || "", trim, form.place("name"));
  const set = useDraft(r.capabilities || [], asSet, form.place("capabilities"));
  const ask = useAsk<HTMLButtonElement>();
  const body: Record<string, unknown> = {};
  if (!owner && name.changed) body.name = name.value.trim();
  if (!owner && set.changed) body.capabilities = set.value;
  const changed = Object.keys(body).length > 0;
  form.track(changed, [name, set]);
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (owner) return;
    if (!changed) {
      rolesMsg(t("users.unchanged"), true);
      return;
    }
    form.save(() => saveRole(r, body));
  };
  const left = (
    <>
      <BusyButton cls={"disable-role needs-roles-disable" + hid(owner)} label={t(r.disabled ? "roles.enable" : "roles.disable")} run={() => disableRole(r)} />
      <button ref={ask.ref} type="button" class={"remove-role" + hid(!!r.builtin || (r.users || 0) > 0)} onClick={ask.ask}>{t("roles.remove")}</button>
    </>
  );
  return (
    <form {...form.props} class="role-form" onSubmit={submit}>
      <TextField label={t("roles.name")} id={"role-" + r.key + "-name"} name="name" maxLength={64} placeholder={t("roles.nameHint")} disabled={owner} draft={name} />
      {owner && <p class="muted small role-owner-note">{t("roles.ownerNote")}</p>}
      <span class="muted small">{t("roles.capabilities")}</span>
      <div class="role-caps">
        <Choices cls="cap-grid" label={t("roles.capabilities")} options={capChoices(owner)} chosen={owner ? roles.caps.map((c) => c.key) : set.value}
          set={set.set} />
        <Meanwhile draft={set} show={(l) => l.map(capLabel).join(", ")} />
      </div>
      {ask.asking ? (
        <Question text={t("roles.removeConfirm", { role: r.key })} yes={t("roles.remove")} no={t("action.keep")} onNo={() => ask.done()}
          onYes={async () => {
            if (!(await removeRole(r))) ask.done();
          }} />
      ) : (
        <FormFooter submit={owner ? undefined : t("action.save")} look={form.look} reset={owner ? undefined : form.reset} off={!changed} extra={left} />
      )}
    </form>
  );
}

/** RolePanel is a role's panel: its head with its name, its key and its badges, built in, disabled and how many users have it; its form. */
function RolePanel({ r }: { r: Role }) {
  load.useLoad();
  const ui = roleUI.of(r.key);
  const badges = [];
  if (r.builtin) badges.push(<Badge key="builtin" kind="mode" text={t("roles.builtin")} />);
  if (r.disabled) badges.push(<Badge key="disabled" kind="bad" text={t("roles.disabled")} />);
  badges.push(<Badge key="users" kind="mode" text={tn("roles.users", r.users || 0)} />);
  return (
    <Fold cls="panel role-panel" open={ui.open} onToggle={(o) => { ui.open = o; }} data={{ key: r.key }}
      summary={<FoldHead name={roleLabel(r.key, r.name)} mono={r.key} badges={badges} />}>
      <RoleForm r={r} />
    </Fold>
  );
}

/** NewRole is the form that creates a role: its key, its display name and its set, read alone at first; what is typed is the form's own. */
function NewRole() {
  load.useLoad();
  const [key, setKey] = useState("");
  const [name, setName] = useState("");
  const [set, setSet] = useState(["read"]);
  const create = useBusy();
  const submit = (ev: Event) => {
    ev.preventDefault();
    const body: { key: string; capabilities: string[]; name?: string } = { key: key.trim().toLowerCase(), capabilities: set };
    if (name.trim()) body.name = name.trim();
    create.run(() => createRole(body)).then((ok) => {
      if (ok) adding.role = false;
    });
  };
  return (
    <form class="panel role-new" onSubmit={submit}>
      <TextField label={t("roles.key")} id="role-new-key" name="key" inputCls="mono" maxLength={32} required placeholder={t("roles.keyHint")} value={key} set={setKey} />
      <TextField label={t("roles.name")} id="role-new-name" name="name" maxLength={64} value={name} set={setName} />
      <span class="muted small">{t("roles.capabilities")}</span>
      <div class="role-caps">
        <Choices cls="cap-grid" label={t("roles.capabilities")} options={capChoices()} chosen={set} set={setSet} />
      </div>
      <div class="actions">
        <span class="actions-right">
          <button type="button" class="form-cancel" onClick={() => { adding.role = false; }}>{t("action.cancel")}</button>
          <button type="submit" {...busyProps("primary", create.busy)}>{t("roles.create")}</button>
        </span>
      </div>
    </form>
  );
}

/** RolesCard is System settings → Roles' body: a panel per role, the form that adds one, the message line. */
export function RolesCard() {
  load.useLoad();
  const list = roles.list || [];
  const count = roles.loaded ? tn("roles.badge.count", list.length) : "";
  // the card's head counts them, in the page's language, once they are read; nothing once forgotten
  useEffect(() => {
    sectionBadges($("sys-roles"), count ? [badgeEl(count, "mode")] : []);
  }, [count]);
  return (
    <>
      <div id="roles-list">{list.map((r) => <RolePanel key={r.key} r={r} />)}</div>
      <Adder id="role-add" formId="role-new" label={t("roles.add")} shown={adding.role} set={(v) => { adding.role = v; }}>
        <NewRole />
      </Adder>
      <ResultLine id="roles-msg" cls="small cubes-msg" role="status" res={said.roles} />
    </>
  );
}

/**
 * CapSwitch is one capability's switch for every role, saved at once
 * (useSetAtOnce) and on its own: the user's position shows while its
 * request runs, a second change of it meanwhile sends nothing, another
 * capability's switch goes on as it is, and refused it shows the
 * capability as stored again.
 */
function CapSwitch({ k, disabled }: { k: string; disabled: boolean }) {
  const on = useSetAtOnce(!disabled);
  return <Switch row={false} label={capLabel(k)} title={k} checked={on.value} onChange={(v) => on.set(v, (now) => setCapability(k, now))} />;
}

/**
 * CapsCard is Capabilities for every role: a switch per capability, on
 * while it is enabled for every role, each saved at once on its own
 * (CapSwitch), in the frame of a choice of several (ChoiceGroup).
 */
export function CapsCard() {
  load.useLoad();
  const caps = roles.caps;
  const off = caps.filter((c) => c.disabled).length;
  const badge = !roles.loaded ? "" : off ? tn("roles.badge.off", off) : t("roles.badge.allOn");
  // the card's head says how many are off, in the page's language, once they are read; nothing once forgotten
  useEffect(() => {
    sectionBadges($("sys-caps"), badge ? [badgeEl(badge, off ? "warn" : "ok")] : []);
  }, [badge, off]);
  return (
    <div id="caps-list">
      <ChoiceGroup cls="cap-grid" label={t("roles.capsTitle")}>
        {caps.map((c) => <CapSwitch key={c.key} k={c.key} disabled={c.disabled} />)}
      </ChoiceGroup>
    </div>
  );
}
