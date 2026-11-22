// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/integrations/Integrations.tsx
 *
 * System settings → Integrations as parts, built once per item and
 * updated in place: the switch that lets integrations ask, saved at once;
 * each request waiting in its panel, with what it may do at most and what
 * it was registered as before, Refuse, which asks first, and Approve;
 * each integration in its panel, with its name and its maximum, each a
 * settings form kept by the integration (shared/forms.ts) with Reset,
 * Save by its state and the disk in the section's pill, Enabled, saved at
 * once, its users each with who it acts as and Link, and Revoke, which
 * asks first; the installations seen on the LAN, with Register, or why
 * the server does not look; Look again.
 */

import { useLayoutEffect, useRef, useState } from "preact/hooks";
import type { Integration, IntegrationUser } from "../api";
import { capLabel } from "../model/cap";
import { session } from "../shared/session";
import { Age, Badge, ResultLine } from "../shared/badge";
import { Choices } from "../shared/choices";
import { hid } from "../shared/cls";
import { Copy, CopyPair } from "../shared/copy";
import { Field, FormFooter, Meanwhile, Select, Switch, TextField, useDraft } from "../shared/field";
import { Fold, FoldHead } from "../shared/fold";
import { focusForm, useForm } from "../shared/forms";
import { type Fact, KvList } from "../shared/kv";
import { Link } from "../shared/link";
import { BusyButton, Question, useAsk, useSetAtOnce } from "../shared/question";
import {
  type FoundIntegration, INTEGRATION_CEILING, data, formKey, integrationPath, keyState, loadDetails, loadIntegrations, loadSeen, panelUI, msgIn, requestUI, requestsOn, run, said, setRequests, shown, watchApproval,
} from "./store";
import { type BadgeSpec, cardBadges, showCard } from "../shared/card";
import { isShown, mayShow, showView } from "../shared/pages";
import { can } from "../shared/me";
import { useLook } from "../shared/look";

/** capOptions are the capabilities of the ceiling as a choice of switches. */
function capOptions(): [string, string][] {
  return INTEGRATION_CEILING.map((c) => [c, capLabel(c)]);
}

/** haRegisterURL is the page of Home Assistant at base that adds the Massimo integration. */
function haRegisterURL(base: string): string {
  return base.replace(/\/+$/, "") + "/_my_redirect/config_flow_start?domain=massimo";
}

/** httpURL is s as an http(s) URL, or null. */
function httpURL(s: string | undefined): URL | null {
  try {
    const u = new URL((s || "").trim());
    return u.protocol === "http:" || u.protocol === "https:" ? u : null;
  } catch {
    return null;
  }
}

/**
 * Register opens the page of a Home Assistant at url that adds the
 * Massimo integration; where url is not known, it first asks for Home
 * Assistant's address, filled in from address.
 */
function Register({ url, address, name, id }: { url?: string; address: string; name: string; id: string }) {
  const known = httpURL(url);
  const [shown, setShown] = useState(false);
  const [typed, setTyped] = useState(address ? "http://" + address + ":8123" : "http://");
  const [bad, setBad] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  // the address asked for: the focus in it in the drawing that shows it
  useLayoutEffect(() => {
    if (shown) input.current?.focus();
  }, [shown]);
  const go = () => {
    const u = httpURL(typed);
    setBad(!u);
    if (u) window.open(haRegisterURL(u.origin + u.pathname), "_blank", "noopener");
  };
  const field = "register-" + id;
  return (
    <div class="integration-register">
      <button type="button" class="register-integration" aria-label={t("integrations.registerLabel", { name })} onClick={() => {
        if (known && url) window.open(haRegisterURL(url), "_blank", "noopener");
        else setShown(!shown);
      }}>
        {t("integrations.register")}
      </button>
      {!known && (
        <Field label={t("integrations.registerAddress")} id={field} wide cls={"register-address" + hid(!shown)} hint={t("integrations.registerHint")}>
          <span class="inline">
            <input ref={input} id={field} type="url" value={typed} onInput={(ev) => setTyped(ev.currentTarget.value)} onKeyDown={(ev) => {
              if (ev.key === "Enter") go();
            }} />
            <button type="button" class="register-open" onClick={go}>{t("integrations.registerOpen")}</button>
          </span>
          <ResultLine cls="small cubes-msg" role="status" res={bad ? { text: t("integrations.registerBad"), ok: false } : null} />
        </Field>
      )}
    </div>
  );
}

/**
 * RequestPanel is a request waiting: its code to compare, where it came
 * from, whether the LAN saw its installation, the maximum, what it was
 * registered as before, Refuse and Approve. What is chosen is the
 * panel's own, until it is approved or refused.
 */
function RequestPanel({ r }: { r: Integration }) {
  useLook();
  const ui = requestUI.of(r.uuid);
  // an integration of the same kind registered before, the same installation first: approved as it, it keeps its users and links
  const earlier = data.list.filter((i) => i.kind === r.kind && i.state !== "pending" && (i.state as string) !== "refused")
    .sort((a, b) => Number(b.instance_id === r.instance_id) - Number(a.instance_id === r.instance_id));
  const [caps, setCaps] = useState(INTEGRATION_CEILING);
  const [attach, setAttach] = useState(earlier.length && earlier[0].instance_id === r.instance_id ? earlier[0].uuid : "");
  const ask = useAsk<HTMLButtonElement>();
  const expires = Date.parse(r.request_expires || "");
  return (
    <Fold cls="panel integration-request" open={!ui.closed} onToggle={(o) => { ui.closed = !o; }} data={{ uuid: r.uuid }}
      summary={<FoldHead name={r.name || r.kind} mono={r.request_code}
        badges={[<Badge key="seen" kind={r.seen ? "ok" : "warn"} text={t(r.seen ? "integrations.seen" : "integrations.notSeen")} />]} />}>
      <div class="integration-request-body">
        <KvList facts={[
          [t("integrations.code"), r.request_code || "", true],
          [t("integrations.from"), r.request_remote || "", true],
          [t("integrations.instance"), r.instance_id, true],
          [t("integrations.url"), r.url || ""],
          [t("integrations.version"), r.version || ""],
          [t("integrations.expires"), isNaN(expires) ? "" : fmtShortTime(expires)],
        ].filter(([, v]) => v) as Fact[]} />
        <Choices legend={t("integrations.max")} label={t("integrations.max")} cls="integration-caps" options={capOptions()} chosen={caps} set={setCaps} />
        <Select label={t("integrations.attach")} id={"integration-attach-" + r.uuid} wide cls={hid(!earlier.length).trim() || undefined} hint={t("integrations.attachHint")}
          options={[["", t("integrations.attachNone")], ...earlier.map((i): [string, string] => [i.uuid, i.name + " (" + i.uuid.slice(0, 8) + ")"])]}
          value={attach} onChange={setAttach} />
        {ask.asking ? (
          <Question text={t("integrations.refuseConfirm", { name: r.name })} yes={t("integrations.refuse")} no={t("action.keep")} onNo={() => ask.done()}
            onYes={async () => {
              if (!(await run(r.name, () => api("POST", integrationPath(r.uuid, "/refuse")), t("integrations.refused", { name: r.name })))) ask.done();
            }} />
        ) : (
          <div class="actions">
            <span class="actions-left"><button ref={ask.ref} type="button" class="refuse-integration" onClick={ask.ask}>{t("integrations.refuse")}</button></span>
            <span class="actions-right">
              <BusyButton cls="primary approve-integration" label={t("integrations.approve")} run={async () => {
                const req: Record<string, unknown> = { max_caps: caps };
                if (attach) req.attach = attach;
                if (await run(r.name, () => api("POST", integrationPath(r.uuid, "/approve"), req), t("integrations.approved", { name: r.name }))) {
                  watchApproval(attach || r.uuid);
                }
              }} />
            </span>
          </div>
        )}
      </div>
    </Fold>
  );
}

/** openForm shows one of an integration's forms and puts the focus into it, from any page: System settings shown, its panel open. */
function openForm(uuid: string, form: "name" | "max"): void {
  if (!isShown("system")) showView("system");
  panelUI.of(uuid).open = true;
  focusForm(formKey(uuid, form));
}

const trim = (v: string) => v.trim();
const asSet = (l: string[]) => [...l].sort().join(",");

/**
 * NameForm is an integration's name, which its users' names begin with on
 * the page, with Reset and Save the name, off while the name is as stored
 * or empty.
 */
function NameForm({ i }: { i: Integration }) {
  useLook();
  const form = useForm({ key: formKey(i.uuid, "name"), name: { key: "integrations.nameFormName", args: { name: i.name } }, open: () => openForm(i.uuid, "name") });
  const name = useDraft(i.name || "", trim, form.place("name"));
  const changed = name.changed && name.value.trim() !== "";
  form.track(name.changed, [name]);
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (!changed) return;
    const v = name.value.trim();
    form.save(() => run(i.name, () => api("PUT", integrationPath(i.uuid), { name: v }), t("integrations.nameSaved", { name: v })));
  };
  return (
    <form {...form.props} class="integration-name" onSubmit={submit}>
      <TextField label={t("integrations.name")} id={"integration-name-" + i.uuid} name="integration-name" maxLength={64} hint={t("integrations.nameHint")} draft={name} />
      <FormFooter submit={t("integrations.saveName")} look={form.look} reset={form.reset} off={!changed} />
    </form>
  );
}

/** MaxForm is an integration's maximum, a switch per capability of the ceiling, with Reset and Save the maximum. */
function MaxForm({ i }: { i: Integration }) {
  useLook();
  const form = useForm({ key: formKey(i.uuid, "max"), name: { key: "integrations.maxFormName", args: { name: i.name } }, open: () => openForm(i.uuid, "max") });
  const max = useDraft(i.max_caps || [], asSet, form.place("max_caps"));
  form.track(max.changed, [max]);
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (!max.changed) return;
    const caps = max.value;
    form.save(() => run(i.name, () => api("PUT", integrationPath(i.uuid), { max_caps: caps }), t("integrations.saved", { name: i.name })));
  };
  return (
    <form {...form.props} class="integration-max" onSubmit={submit}>
      <Choices legend={t("integrations.max")} label={t("integrations.max")} cls="integration-caps" options={capOptions()} chosen={max.value} set={max.set} />
      <Meanwhile draft={max} show={(l) => l.map(capLabel).join(", ")} />
      <FormFooter submit={t("integrations.saveMax")} look={form.look} reset={form.reset} off={!max.changed} />
    </form>
  );
}

/** linkOptions are the Massimo users an integration's user may act as: its own user first, then the users, the proposal marked. */
function linkOptions(u: IntegrationUser): [string, string][] {
  const out: [string, string][] = [["", t("integrations.ownUser", { name: u.name || u.id.slice(0, 8) })]];
  for (const m of data.users || []) {
    const label = m.display_name ? m.display_name + " (" + m.name + ")" : m.name;
    out.push([m.name, m.name === u.proposal ? t("integrations.proposed", { name: label }) : label]);
  }
  if (u.linked && !out.some(([v]) => v === u.user)) out.push([u.user, u.user]);
  return out;
}

/**
 * UserRow is one of an integration's users: its name there, its level and
 * when it acted last, and who it acts as, with Link where the users may
 * be read; the choice follows what is stored until it is changed.
 */
function UserRow({ i, u }: { i: Integration; u: IntegrationUser }) {
  const system = u.id === "system";
  const name = system ? t("integrations.system") : u.name || u.id;
  const acts = useDraft(u.linked ? u.user : u.proposal || "");
  const choose = !system && !!data.users;
  return (
    <li class="integration-user" data-id={u.id}>
      <div class="integration-user-head">
        <span class="integration-user-name">{name}</span>
        {!system && <span class="mono muted small">{u.id.slice(0, 8)}</span>}
        {u.system_generated && <Badge kind="mode" text={t("integrations.systemGenerated")} />}
        {u.last_level && <Badge kind="mode" text={t("integrations.level." + u.last_level)} />}
        {u.linked && <Badge kind="ok" text={t("integrations.linkedBadge")} />}
      </div>
      <p class="muted small">{u.last_used_at ? <>{t("integrations.lastUsed") + " "}<Age at={u.last_used_at} plain /></> : t("users.neverUsed")}</p>
      {!choose && <p class="small">{t("integrations.actsAsUser", { name: u.linked ? u.user : u.display || u.user })}</p>}
      {choose && (
        <Field label={t("integrations.actsAs")} id={"integration-user-" + i.uuid + "-" + u.id} wide
          hint={!u.linked && u.proposal ? t("integrations.proposalHint", { name: u.proposal }) : undefined} draft={acts}>
          <span class="inline">
            <Select id={"integration-user-" + i.uuid + "-" + u.id} row={false} options={linkOptions(u)} draft={acts} />
            <BusyButton cls="link-integration-user" label={t("integrations.link")} aria={t("integrations.linkLabel", { name })} run={() => {
              const user = acts.value || null;
              const ok = user ? t("integrations.linked", { name, user }) : t("integrations.unlinked", { name });
              return run(i.name, () => api("PUT", integrationPath(i.uuid, "/users/" + encodeURIComponent(u.id)), { user }), ok);
            }} />
          </span>
        </Field>
      )}
    </li>
  );
}

/**
 * IntegrationPanel is an integration: what it is, its name, a new
 * installation to accept, Enabled, its maximum, its users, read when the
 * panel opens, and Revoke.
 */
function IntegrationPanel({ i }: { i: Integration }) {
  useLook();
  const ui = panelUI.of(i.uuid);
  const key = keyState(i);
  const enabled = useSetAtOnce(i.state === "active");
  const ask = useAsk<HTMLButtonElement>();
  const badges = [];
  if (i.state === "disabled") badges.push(<Badge key="off" kind="bad" text={t("integrations.disabled")} />);
  if (i.pending_instance) badges.push(<Badge key="instance" kind="warn" text={t("integrations.pendingInstance")} />);
  if (key === "expired") badges.push(<Badge key="expired" kind="warn" text={t("integrations.approvalExpired")} />);
  const issued = Date.parse(i.key_issued_at || "");
  const users = data.details[i.uuid]?.users;
  return (
    <Fold cls="panel integration-panel" open={ui.open} data={{ uuid: i.uuid }} summary={<FoldHead name={i.name} badges={badges} />} onToggle={(o) => {
      ui.open = o;
      if (!o) return;
      const n = session();
      loadDetails(i.uuid).catch((e: Error) => msgIn(n, t("integrations.failed", { name: i.name, error: e.message }), false));
    }}>
      <div class="integration-body">
        {key === "waiting" && (
          <p class="notice small integration-key-waiting">{t("integrations.keyWaiting", { time: fmtShortTime(Date.parse(i.request_expires || "")) })}</p>
        )}
        {key === "expired" && (
          <div class="notice integration-approval-expired">
            <p>{t("integrations.approvalExpiredText")}</p>
            <Register url={i.url} address="" name={i.name} id={i.uuid} />
          </div>
        )}
        <NameForm i={i} />
        <KvList facts={[
          [t("integrations.reportedName"), i.reported_name && i.reported_name !== i.name ? i.reported_name : ""],
          [t("integrations.uuid"), i.uuid, true],
          [t("integrations.instance"), i.instance_id, true],
          [t("integrations.url"), i.url || ""],
          [t("integrations.version"), i.version || ""],
          [t("integrations.keyIssued"), isNaN(issued) ? t("integrations.keyNotIssued") : fmtShortTime(issued)],
          [t("integrations.seenLabel"), t(i.seen ? "integrations.seen" : "integrations.notSeen")],
        ].filter(([, v]) => v) as Fact[]} />
        {i.pending_instance && (
          <div class="notice integration-instance">
            <p>{t("integrations.pendingInstanceText", { id: i.pending_instance })}</p>
            <div class="actions">
              <BusyButton cls="accept-instance" label={t("integrations.acceptInstance")}
                run={() => run(i.name, () => api("POST", integrationPath(i.uuid, "/accept-instance")), t("integrations.accepted", { name: i.name }))} />
            </div>
          </div>
        )}
        <Switch id={"integration-enabled-" + i.uuid} name="enabled" label={t("integrations.enabled")} checked={enabled.value}
          onChange={(v) => enabled.set(v, (on) => run(i.name, () => api("PUT", integrationPath(i.uuid), { state: on ? "active" : "disabled" }),
            t(on ? "integrations.enabledOn" : "integrations.enabledOff", { name: i.name })))} />
        <MaxForm i={i} />
        <h4>{t("integrations.users")}</h4>
        <ul class="integration-users">
          {users && !users.length && <li class="muted small">{t("integrations.usersNone")}</li>}
          {(users || []).map((u) => <UserRow key={u.id} i={i} u={u} />)}
        </ul>
        {ask.asking ? (
          <Question text={t("integrations.revokeConfirm", { name: i.name })} yes={t("integrations.revoke")} no={t("action.keep")} onNo={() => ask.done()}
            onYes={async () => {
              if (!(await run(i.name, () => api("DELETE", integrationPath(i.uuid)), t("integrations.revoked", { name: i.name })))) ask.done();
            }} />
        ) : (
          <div class="actions">
            <span class="actions-left"><button ref={ask.ref} type="button" class="revoke-integration" onClick={ask.ask}>{t("integrations.revoke")}</button></span>
          </div>
        )}
      </div>
    </Fold>
  );
}

/** NOT_LOOKING are the texts of why the server does not look on the LAN, by the found answer's announce.why (AP LookReason). */
const NOT_LOOKING: Record<string, string> = {
  off: "integrations.foundOff", loopback: "integrations.foundLoopback", restart: "integrations.foundRestart", fixed: "integrations.foundFixed",
};

/** NotLooking says why the server does not look on the LAN and, where Maintenance can change it, opens Maintenance. */
function NotLooking() {
  const a = data.announce || {};
  const key = (a.why && NOT_LOOKING[a.why]) || "integrations.foundNot";
  const opens = !!a.why && !!NOT_LOOKING[a.why] && a.why !== "fixed" && can("settings") && mayShow("maint");
  return (
    <>
      <p class="muted small">{t(key, { by: a.fixed_by || "" })}</p>
      {opens && (
        <div class="actions">
          <span class="actions-left">
            <button type="button" id="integrations-open-maint" onClick={() => {
              showView("maint");
              showCard("sys-maint");
            }}>
              {t("integrations.openMaintenance")}
            </button>
          </span>
        </div>
      )}
    </>
  );
}

/** Seen is an installation seen on the LAN: its name, its address as a link or as text, either copyable, registered or not, and Register. */
function Seen({ f, n }: { f: FoundIntegration; n: number }) {
  const name = f.location || f.name;
  const url = httpURL(f.url) ? f.url || "" : "";
  return (
    <li>
      <span class="integration-found-name">{name}</span>{" "}
      {url ? (
        <CopyPair get={() => url} label={t("copy.what", { what: url })}><Link url={url} text={url} cls="mono small" /></CopyPair>
      ) : (
        <Copy text={f.url || f.address} icon cls="mono muted small" />
      )}{" "}
      <Badge kind={f.integration ? "ok" : "mode"} text={t(f.integration ? "integrations.registered" : "integrations.notRegistered")} />
      {!f.integration && <Register url={f.url} address={f.address} name={name} id={"found-" + n} />}
    </li>
  );
}

/** IntegrationsCard is the section's body. */
export function IntegrationsCard() {
  useLook();
  const list = data.list;
  const waiting = list.filter((i) => i.state === "pending");
  const registered = list.filter((i) => i.state === "active" || i.state === "disabled");
  const asking = useSetAtOnce(!!requestsOn.value);
  const badges = data.loaded ? [waiting.length ? tn("integrations.badge.waiting", waiting.length) : "", registered.length ? String(registered.length) : ""] : null;
  const counts = badges ? badges.join("|") : "";
  // the card's head counts the requests and the integrations, once they are read; nothing once forgotten
  // biome-ignore lint/correctness/useExhaustiveDependencies: counts is the badges' texts joined, which change with them; badges is a new list at every drawing.
  useLayoutEffect(() => {
    const out: BadgeSpec[] = [];
    if (badges?.[0]) out.push({ text: badges[0], kind: "warn" });
    if (badges?.[1]) out.push({ text: badges[1], kind: "mode" });
    cardBadges("sys-integrations", out);
  }, [counts]);
  return (
    <>
      <p class="muted small">{t("integrations.text")}</p>
      <Switch id="integration-requests" rowCls="needs-settings" label={t("integrations.requests")} checked={asking.value}
        onChange={(v) => asking.set(v, setRequests)} />
      <div id="integration-requests-list">
        {waiting.length > 0 && <h4>{t("integrations.waiting")}</h4>}
        {waiting.map((r) => <RequestPanel key={r.uuid} r={r} />)}
      </div>
      <div id="integrations-list" ref={shown}>
        {data.loaded && !registered.length && <p class="muted small">{t("integrations.none")}</p>}
        {registered.map((i) => <IntegrationPanel key={i.uuid} i={i} />)}
      </div>
      <h4 class="integrations-found-h">{t("integrations.found")}</h4>
      <div id="integrations-found">
        {data.loaded && !data.looks && <NotLooking />}
        {data.looks && !data.found.length && <p class="muted small">{t("integrations.foundNone")}</p>}
        {data.looks && data.found.length > 0 && (
          <ul class="integrations-found-list">{data.found.map((f, n) => <Seen key={f.instance_id + "|" + f.address} f={f} n={n} />)}</ul>
        )}
      </div>
      <div class="actions">
        <span class="actions-left">
          <BusyButton id="integrations-look" cls={hid(!data.looks).trim()} label={t("integrations.lookAgain")} run={async () => {
            const n = session();
            try {
              await loadSeen(true);
              await loadIntegrations();
              msgIn(n, tn("integrations.lookDone", data.found.length), true);
            } catch (e) {
              msgIn(n, t("integrations.failed", { name: "", error: (e as Error).message }), false);
            }
          }} />
        </span>
      </div>
      <ResultLine id="integrations-msg" cls="small cubes-msg" role="status" res={said.value} />
    </>
  );
}
