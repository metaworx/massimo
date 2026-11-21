// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/identities.tsx
 *
 * A user's identities at the sign-in providers, wherever they are drawn:
 * in a user's panel of System settings → Users, the manager's, and in My
 * profile, one's own linked accounts. Each with its provider, its e-mail
 * or user id there, whether it is bound or waits for its first sign-in,
 * what the provider said of it and when, and Remove; where the caller
 * gives its forms (Changes), Change of the e-mail or user id while no
 * sign-in used it, a settings form kept by the identity (forms.ts); where
 * it asks for it, Assign, by an e-mail or by the user id at a provider;
 * and what the caller draws after the list (My profile's Link buttons).
 * The list is read when its place opens and again at every change of the
 * caller's counter while it is open. Moved from System settings' users
 * (AP UiParts, Block 13), its texts the users.… keys both places use.
 */

import type { ComponentChildren } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";
import type { Identity, LoginProvider, User } from "../api";
import { addIdentity, changeIdentity, loadIdentities, removeIdentity, type Run } from "./account";
import { Age } from "./badge";
import { hid } from "./cls";
import { FormFooter, Meanwhile, Select, TextField, useDraft } from "./field";
import { focusForm, forgetForms, hasChanges, useForm } from "./forms";
import { useLang } from "./lang";
import { BusyButton } from "./question";
import { session, stillSession } from "./session";

/**
 * The change forms of a user's identities, as the caller keeps them: the
 * key of an identity's form (forms.ts), the key all of the user's are
 * under, what is kept per form (its form shown), forgetting that, and
 * how the form is brought into sight from anywhere (its page shown, its
 * card unfolded, its panel open).
 */
export interface Changes {
  key(id: number): string;
  under: string;
  ui(key: string): { changing: boolean };
  forget(key: string): void;
  show(key: string): void;
}

/** Details is what the provider said of the account at its last sign-in or link: its name and user name, its groups, and when. */
function Details({ i }: { i: Identity }) {
  const parts: string[] = [];
  if (i.name) parts.push(i.name);
  if (i.username && i.username !== i.name) parts.push(i.username);
  if (i.email && i.subject && i.subject !== i.username) parts.push(t("users.identityUserID", { id: i.subject }));
  if (i.groups?.length) parts.push(t("users.identityGroups", { groups: i.groups.join(", ") }));
  return (
    <span class="muted small identity-details">
      {parts.length > 0 && parts.join(" · ") + " · "}
      {i.last_used_at ? <>{t("users.identityLastUsed") + " "}<Age at={i.last_used_at} plain /></> : t("users.neverUsed")}
    </span>
  );
}

/**
 * Change is the form that changes an identity's e-mail, or its user id
 * where it has no e-mail, while no sign-in used it: its draft kept by the
 * identity, Save by its state, Cancel back to the value stored and the
 * form closed, which gives the focus back to Change.
 */
function Change({ u, i, name, run, say, changes, done }: { u: User; i: Identity; name: string; run: Run; say: (text: string, ok: boolean) => void; changes: Changes; done: () => void }) {
  useLang();
  const bySubject = !i.email;
  const key = changes.key(i.id);
  const form = useForm({
    key,
    name: { key: "users.identityFormName", args: { name, account: i.email || i.subject } },
    open: () => {
      changes.show(key);
      changes.ui(key).changing = true;
      focusForm(key);
    },
  });
  const account = useDraft(bySubject ? i.subject : i.email, (v) => v.trim(), form.place("account"));
  form.track(account.changed, [account]);
  const submit = (ev: Event) => {
    ev.preventDefault();
    const value = account.value.trim();
    if (!value) {
      say(t(bySubject ? "users.identitySubjectNeeded" : "users.identityEmailNeeded"), false);
      return;
    }
    // taken, the form closes, as the plain one did
    form.save(() => changeIdentity(run, u.name, i, bySubject, value).then((ok) => {
      if (ok) done();
      return ok;
    }));
  };
  return (
    <>
      <form {...form.props} class="identity-change" onSubmit={submit}>
        <input type="text" name="account" class="field-control" autocomplete="off" spellcheck={false}
          aria-label={t(bySubject ? "users.identitySubject" : "users.identityEmail")} value={account.value} onInput={(ev) => account.set(ev.currentTarget.value)} />
        <FormFooter submit={t("action.save")} look={form.look} off={!account.changed} cancel={() => {
          form.reset();
          done();
        }} />
      </form>
      <Meanwhile draft={account} />
    </>
  );
}

/** Row is one of a user's identities, with Remove and, where the caller gives its forms, Change for one no sign-in used yet. */
function Row({ u, i, name, providers, run, say, changes }: { u: User; i: Identity; name: string; providers: LoginProvider[]; run: Run; say: (text: string, ok: boolean) => void; changes?: Changes }) {
  const key = changes ? changes.key(i.id) : "";
  const ui = changes ? changes.ui(key) : null;
  const changeRef = useRef<HTMLButtonElement>(null);
  const back = useRef(false);
  const known = providers.find((p) => p.key === i.provider);
  const provider = known ? known.name : i.provider;
  const account = i.email || t("users.identityUserID", { id: i.subject });
  const changeable = !!ui && !i.last_used_at;
  const changing = changeable && !!ui?.changing;
  // Cancel gives the focus back to Change, once it is shown again
  useEffect(() => {
    if (!changing && back.current) {
      back.current = false;
      changeRef.current?.focus();
    }
  }, [changing]);
  return (
    <li data-id={String(i.id)}>
      <span class="flag mode identity-provider">{provider}</span>
      <span class="identity-email">{account}</span>
      <span class="muted small identity-state">{t(i.bound ? "users.identityBound" : "users.identityWaiting")}</span>
      <span class="identity-buttons">
        {ui && (
          <button ref={changeRef} type="button" class={"change-identity" + hid(!changeable || changing)}
            aria-label={t("users.identityChangeLabel", { email: account, provider })} onClick={() => { ui.changing = true; }}>
            {t("users.identityChange")}
          </button>
        )}
        <BusyButton cls="remove-identity" label={t("users.identityRemove")} aria={t("users.identityRemoveLabel", { email: account, provider })}
          run={() => removeIdentity(run, u.name, i)} />
      </span>
      <Details i={i} />
      {changing && changes && ui && <Change u={u} i={i} name={name} run={run} say={say} changes={changes} done={() => {
        back.current = true;
        ui.changing = false;
      }} />}
    </li>
  );
}

/**
 * Assign is the choice of a provider, by e-mail or by the user id there,
 * and the account, with Assign; what is typed is the part's own.
 */
function Assign({ u, providers, run, say }: { u: User; providers: LoginProvider[]; run: Run; say: (text: string, ok: boolean) => void }) {
  const [chosen, setChosen] = useState("");
  const [kind, setKind] = useState("email");
  const [account, setAccount] = useState("");
  const provider = providers.some((p) => p.key === chosen) ? chosen : providers[0]?.key || "";
  const bySubject = kind === "subject";
  const id = (what: string) => "user-identity-" + what + "-" + u.name;
  return (
    <>
      <Select label={t("users.identityProvider")} id={id("provider")} name="provider" wide options={providers.map((p) => [p.key, p.name])} value={provider} onChange={setChosen} />
      <Select label={t("users.identityKind")} id={id("kind")} name="kind" wide options={[["email", t("users.identityByEmail")], ["subject", t("users.identityBySubject")]]}
        value={kind} onChange={setKind} />
      <TextField label={t(bySubject ? "users.identitySubject" : "users.identityEmail")} id={id("account")} name="account" type={bySubject ? "text" : "email"}
        value={account} set={setAccount} />
      <div class="actions">
        <span class="actions-right">
          <BusyButton cls="add-identity" label={t("users.identityAdd")} run={async () => {
            const value = account.trim();
            if (!value) {
              say(t(bySubject ? "users.identitySubjectNeeded" : "users.identityEmailNeeded"), false);
              return;
            }
            if (await addIdentity(run, u.name, provider, bySubject, value)) setAccount("");
          }} />
        </span>
      </div>
    </>
  );
}

/**
 * Identities is the box of a user's identities, hidden where off; open
 * says that its place is shown, reads, the caller's counter of changes,
 * asks for the list again by its change; providers are the enabled ones,
 * by whose names the rows name theirs; run and say are the caller's
 * runner and message line. title and none are the texts of its heading
 * and of an empty list, cls a class beside user-identities-box; changes
 * gives the rows Change (the manager's), assign adds Assign under the
 * list, after draws what follows the list once it is read, and quiet
 * draws nothing while the list read is empty and the caller offers
 * nothing after it (offer: My profile's Link buttons, none in the app,
 * where the rows are still named from every provider). With changes, the forms of identities a read no
 * longer gives, or that a sign-in made unchangeable, are forgotten with
 * what they hold, and so are all of them where the box goes off; closed,
 * the box reads only where one of them holds a draft.
 */
export function Identities({ u, off, open, reads, providers, run, say, title, none, cls, changes, assign, after, quiet, offer }: {
  u: User;
  off: boolean;
  open: boolean;
  reads: number;
  providers: LoginProvider[];
  run: Run;
  say: (text: string, ok: boolean) => void;
  title: string;
  none: string;
  cls?: string;
  changes?: Changes;
  assign?: boolean;
  after?: (list: Identity[]) => ComponentChildren;
  quiet?: boolean;
  offer?: boolean;
}) {
  useLang();
  const [list, setList] = useState<Identity[] | null>(null);
  const [error, setError] = useState("");
  const shown = useRef<Identity[]>([]);
  // read when its place opens and at every change of the caller's counter while it is open
  // biome-ignore lint/correctness/useExhaustiveDependencies: reads is the caller's change counter, which asks for the read again by its change alone; changes is the caller's, the same for the box's life.
  useEffect(() => {
    if (off) {
      if (!changes) return;
      // the box off (no provider enabled any more): what its change forms held goes, as their identities may be gone
      forgetForms(changes.under);
      for (const was of shown.current) changes.forget(changes.key(was.id));
      shown.current = [];
      return;
    }
    // closed, it reads again only where one of its change forms holds a draft, so that a gone identity's form is forgotten
    if (!open && !(changes && hasChanges(changes.under))) return;
    let stale = false;
    const n = session();
    loadIdentities(u.name).then((l) => {
      if (stale || !stillSession(n)) return;
      if (changes) {
        for (const was of shown.current) {
          if (l.some((x) => x.id === was.id && !x.last_used_at)) continue;
          const key = changes.key(was.id);
          forgetForms(key);
          changes.forget(key);
        }
      }
      shown.current = l;
      setList(l);
      setError("");
    }, (e: Error) => {
      if (stale || !stillSession(n)) return;
      setList(null);
      setError(e.message);
    });
    return () => {
      stale = true;
    };
  }, [open, off, reads, u.name]);
  if (quiet && !error && (!list || (!list.length && !offer))) return null;
  const name = u.display || u.display_name || u.name;
  return (
    <div class={"user-identities-box" + (cls ? " " + cls : "") + hid(off)}>
      <span class="muted small">{title}</span>
      <ul class="user-identities">
        {error && <li class="muted small bad">{error}</li>}
        {list && !list.length && <li class="muted small">{none}</li>}
        {(list || []).map((i) => <Row key={i.id} u={u} i={i} name={name} providers={providers} run={run} say={say} changes={changes} />)}
      </ul>
      {assign && !off && <Assign u={u} providers={providers} run={run} say={say} />}
      {list && after && after(list)}
    </div>
  );
}
