// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/admin/Providers.tsx
 *
 * System settings → Sign-in providers as a part: the redirect URI to
 * register, with its copy icon; a panel per provider, built once per
 * provider and updated in place, with its settings form, kept by the
 * provider's key (shared/forms.ts) so that what is typed outlives an
 * action in another panel, a reload of the list and the panel's closing,
 * with Remove, which asks in place of the actions, Reset, Save by its
 * state and the disk in the section's pill; and the form that adds one,
 * with its key. The client secret is written only: typed, it is the
 * form's own state, never in the forms' store, and sent with the next
 * Save.
 */

import { useEffect, useState } from "preact/hooks";
import type { Provider } from "../api";
import { Adder } from "../shared/adder";
import { Badge, ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { Copy } from "../shared/copy";
import { type Draft, FormFooter, Select, Switch, TextField, useDraft } from "../shared/field";
import { Fold, FoldHead } from "../shared/fold";
import { focusForm, type Place, useForm } from "../shared/forms";
import { Question, busyProps, useAsk, useBusy } from "../shared/question";
import { addProvider, autoRoleOptions, removeProvider, saveProvider } from "./sendProviders";
import { adding, load, providerFormKey, providers, providerUI, said } from "./store";
import { cardBadges, showCard } from "../shared/card";

/** openProvider shows a provider's settings and puts the focus into them, from any page: System settings shown, its panel open. */
function openProvider(key: string): void {
  if (!isShown("system")) showView("system");
  showCard("sys-providers"); // a folded card unfolded, its form in sight
  providerUI.of(key).open = true;
  focusForm(providerFormKey(key));
}

const trim = (v: string) => v.trim();
/** listOf is a field's comma-separated list, its empty entries left out. */
const listOf = (v: string) => v.split(",").map((x) => x.trim()).filter(Boolean);
const asList = (v: string) => listOf(v).join(",");

/** The fields of a provider's form, each a draft against the provider as stored, or against nothing for a new one. */
interface Drafts {
  name: Draft<string>;
  issuer: Draft<string>;
  clientId: Draft<string>;
  scopes: Draft<string>;
  allowed: Draft<string>;
  claim: Draft<string>;
  enabled: Draft<boolean>;
  auto: Draft<boolean>;
  role: Draft<string>;
  domains: Draft<string>;
}

/** useDrafts are a provider's fields, kept at place's field where given (a provider's form), else the part's own (the new one's). */
function useDrafts(p: Provider | null, place?: <T>(field: string) => Place<T>): Drafts {
  return {
    name: useDraft(p?.name || "", trim, place?.("name")),
    issuer: useDraft(p?.issuer || "", trim, place?.("issuer")),
    clientId: useDraft(p?.client_id || "", trim, place?.("client_id")),
    scopes: useDraft(p?.scopes || "", trim, place?.("scopes")),
    allowed: useDraft((p?.allowed_groups || []).join(", "), asList, place?.("allowed_groups")),
    claim: useDraft(p?.groups_claim || "", trim, place?.("groups_claim")),
    enabled: useDraft(p ? p.enabled : true, undefined, place?.("enabled")),
    auto: useDraft(!!p?.auto_create, undefined, place?.("auto_create")),
    role: useDraft(p ? p.auto_role : "viewer", undefined, place?.("auto_role")),
    domains: useDraft((p?.auto_domains || []).join(", "), asList, place?.("auto_domains")),
  };
}

/** bodyOf is what Save sends, every field as providers.js sent it; the secret only where one is typed, or for a new provider. */
function bodyOf(d: Drafts, secret: string, isNew: boolean): Record<string, unknown> {
  const body: Record<string, unknown> = {
    name: d.name.value.trim(),
    issuer: d.issuer.value.trim(),
    client_id: d.clientId.value.trim(),
    scopes: d.scopes.value.trim(),
    enabled: d.enabled.value,
    auto_create: d.auto.value,
    auto_role: d.role.value,
    auto_domains: listOf(d.domains.value),
    allowed_groups: listOf(d.allowed.value),
    groups_claim: d.claim.value.trim(),
  };
  if (secret || isNew) body.client_secret = secret;
  return body;
}

/** Fields are a provider's fields, the ids under id; the secret written only, its line saying whether one is set. */
function Fields({ id, p, d, secret, setSecret }: { id: string; p: Provider | null; d: Drafts; secret: string; setSecret: (v: string) => void }) {
  const f = (field: string) => "provider-" + id + "-" + field;
  const auto = "auto-only" + hid(!d.auto.value);
  return (
    <>
      <TextField label={t("providers.name")} id={f("name")} name="name" maxLength={64} placeholder="Nextcloud" draft={d.name} />
      <TextField label={t("providers.issuer")} id={f("issuer")} name="issuer" type="url" placeholder="https://cloud.example.com" draft={d.issuer} />
      <TextField label={t("providers.clientId")} id={f("client_id")} name="client_id" draft={d.clientId} />
      <TextField label={t("providers.clientSecret")} id={f("client_secret")} name="client_secret" type="password" value={secret} set={setSecret}
        hint={t(p?.secret_lost ? "providers.secretLost" : p?.has_secret ? "providers.secretKept" : "providers.secretNone")} />
      <TextField label={t("providers.scopes")} id={f("scopes")} name="scopes" placeholder="openid email profile" draft={d.scopes} />
      <TextField label={t("providers.allowedGroups")} id={f("allowed_groups")} name="allowed_groups" placeholder="family, admins" hint={t("providers.groupsHint")}
        draft={d.allowed} />
      <TextField label={t("providers.groupsClaim")} id={f("groups_claim")} name="groups_claim" maxLength={64} placeholder="groups" draft={d.claim} />
      <Switch name="enabled" label={t("providers.enabled")} draft={d.enabled} />
      <Switch name="auto_create" label={t("providers.autoCreate")} draft={d.auto} />
      <Select label={t("providers.autoRole")} id={f("auto_role")} name="auto_role" wide cls={auto} options={autoRoleOptions(p?.auto_role)} draft={d.role} />
      <TextField label={t("providers.autoDomains")} id={f("auto_domains")} name="auto_domains" cls={auto} placeholder="example.com, example.org"
        hint={t("providers.domainsHint")} draft={d.domains} />
    </>
  );
}

/**
 * ProviderForm is a provider's settings in its panel: its fields, Remove,
 * Reset and Save, which sends every field, and the secret where one is
 * typed; Save for an enabled provider first has the server read its
 * discovery document.
 */
function ProviderForm({ p }: { p: Provider }) {
  load.useLoad();
  const form = useForm({ key: providerFormKey(p.key), name: { key: "providers.formName", args: { name: p.name || p.key } }, open: () => openProvider(p.key) });
  const d = useDrafts(p, form.place);
  // written only: the part's own state, not the forms' store
  const [secret, setSecret] = useState("");
  const ask = useAsk<HTMLButtonElement>();
  const drafts = Object.values(d) as Draft<unknown>[];
  const changed = drafts.some((x) => x.changed) || secret !== "";
  form.track(changed, [...drafts, { take: () => setSecret("") }]);
  const submit = (ev: Event) => {
    ev.preventDefault();
    form.save(() => saveProvider(p, bodyOf(d, secret, false)));
  };
  return (
    <form {...form.props} class="provider-form" onSubmit={submit}>
      <Fields id={p.key} p={p} d={d} secret={secret} setSecret={(v) => {
        setSecret(v);
        form.place("client_secret").changed();
      }} />
      {ask.asking ? (
        <Question text={t("providers.removeConfirm", { name: p.name || p.key })} yes={t("providers.remove")} no={t("action.keep")} onNo={() => ask.done()}
          onYes={async () => {
            if (!(await removeProvider(p))) ask.done();
          }} />
      ) : (
        <FormFooter submit={t("action.save")} look={form.look} reset={form.reset} off={!changed}
          extra={<button ref={ask.ref} type="button" class="remove-provider" onClick={ask.ask}>{t("providers.remove")}</button>} />
      )}
    </form>
  );
}

/** NewProvider is the form that adds a provider: its key, then the fields of a provider; what is typed is the form's own, the secret too. */
function NewProvider() {
  load.useLoad();
  const d = useDrafts(null);
  const [key, setKey] = useState("");
  const [secret, setSecret] = useState("");
  const add = useBusy();
  const submit = (ev: Event) => {
    ev.preventDefault();
    const body = { ...bodyOf(d, secret, true), key: key.trim().toLowerCase() };
    add.run(() => addProvider(body)).then((ok) => {
      if (ok) adding.provider = false;
    });
  };
  return (
    <form class="panel provider-form" onSubmit={submit}>
      <TextField label={t("providers.key")} id="provider-new-key" name="key" cls="provider-key" maxLength={32} placeholder="nextcloud" value={key} set={setKey} />
      <Fields id="new" p={null} d={d} secret={secret} setSecret={setSecret} />
      <div class="actions">
        <span class="actions-right">
          <button type="button" class="form-cancel" onClick={() => { adding.provider = false; }}>{t("action.cancel")}</button>
          <button type="submit" {...busyProps("primary", add.busy)}>{t("action.save")}</button>
        </span>
      </div>
    </form>
  );
}

/** ProviderPanel is a provider's panel: its head with its name, its key and its badges, disabled and creates users; its form. */
function ProviderPanel({ p }: { p: Provider }) {
  load.useLoad();
  const ui = providerUI.of(p.key);
  const badges = [];
  if (!p.enabled) badges.push(<Badge key="off" kind="bad" text={t("providers.off")} />);
  if (p.auto_create) badges.push(<Badge key="auto" kind="warn" text={t("providers.createsUsers")} />);
  return (
    <Fold cls="panel provider-panel" open={ui.open} onToggle={(o) => { ui.open = o; }} data={{ key: p.key }}
      summary={<FoldHead name={p.name || p.key} mono={p.key} badges={badges} />}>
      <ProviderForm p={p} />
    </Fold>
  );
}

/** ProvidersCard is the section's body: what to register at a provider, the panels, the form that adds one, the message line. */
export function ProvidersCard() {
  load.useLoad();
  const list = providers.list;
  const callback = pageBase() + "api/v1/login/oidc/callback";
  const count = providers.loaded && list.length ? String(list.length) : "";
  // the card's head counts them, once they are read; nothing for none or once forgotten
  useEffect(() => {
    cardBadges("sys-providers", count ? [{ text: count, kind: "mode" }] : []);
  }, [count]);
  return (
    <>
      <p class="muted small">{t("providers.text")}</p>
      <p class="small providers-callback">
        <span>{t("providers.callback")}</span> <span id="providers-callback"><Copy text={callback} label={callback} icon /></span>
      </p>
      <div id="providers-list">
        {providers.loaded && !list.length && <p class="muted small">{t("providers.none")}</p>}
        {list.map((p) => <ProviderPanel key={p.key} p={p} />)}
      </div>
      <Adder id="provider-add" formId="provider-new" label={t("providers.add")} shown={adding.provider} set={(v) => { adding.provider = v; }}>
        <NewProvider />
      </Adder>
      <ResultLine id="providers-msg" cls="small cubes-msg" role="status" res={said.providers} />
    </>
  );
}

