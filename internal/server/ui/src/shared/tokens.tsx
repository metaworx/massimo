// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/tokens.tsx
 *
 * A user's sessions and API keys, wherever they are drawn: in a user's
 * panel of System settings → Users and in My profile. Each with its
 * kind, its id, label, the key a session was made from, the key a key
 * was made by (made_by, AP KeyLife, D5-A), when and from where it was
 * last used, and Revoke, whose message names the keys that went with it;
 * of one's own user the token the
 * page is signed in with says "this device" and the key that session was
 * made of "its key", and Revoke on "this device" signs out; in the app the
 * key the phone keeps for the server (its status' key_id, shared/app.ts)
 * says "this device's key", in place of "its key" where both hold,
 * however the session was made (AP MobileApp, D65-A, D66-A); the label of
 * a new key with Create key, and the key just made, shown once: a Copy of
 * it with its icon, keyed by the key, copied by a click, Enter or Space on
 * it or by the icon, both named Copy the key and not by the key, so that
 * neither a screen reader nor the tooltip says it again (AP KeyLife,
 * D1-A). The list
 * is read when its place opens and again at every change of the caller's
 * counter while it is open, in the app once the app has revoked what the
 * last sign-in revokes (D67-A); the key just made is shown by the read
 * that follows its making and gone at the next, kept in this part's own
 * state alone. An API key the caller names is marked, its row tinted,
 * and the caller is told of every read drawn, with that row, to bring it
 * into view (My profile's key of its address). The key that is the
 * settings file's MASSIMO_API_TOKEN (from_settings in the list) says so
 * in place of Revoke, which the server refuses for it: it is changed or
 * removed in the file (AP MobileApp v1.4, D86-A). Under the idle limit
 * (AP KeyLife, Block 3) a key's line says when it ends unless used, that
 * it is kept while unused, or, ended, Ended and since when it was unused;
 * for a caller who may keep a key from the limit (keeps: who manages the
 * user, D7-A) Create key has the switch Keep while unused, and a key's
 * row Change…, a small form with that switch, Save and Cancel, on none of
 * an integration's keys, the settings file's key or the key the page is
 * signed in with. A key's end date (Block 4) is on its line, "ends …",
 * or, past it, Ended and when it ended; Create key has the field Ends,
 * empty for none, and for a caller who may move a key's end date (ends:
 * the key's user or who manages it, D4-A) a key's row has Change… with
 * that field and No end, on an integration's key too, the switch there
 * for who keeps alone, and never on the settings file's key nor the key
 * the page is signed in with. Moved from System settings' users (AP
 * UiParts, Block 13), its texts the users.… keys both places use.
 */

import { useEffect, useLayoutEffect, useRef, useState } from "preact/hooks";
import type { Token, User } from "../api";
import { createKey, loadTokens, revokeOwn, revokeToken, setTokenLife, type Run, type TokenLife } from "./account";
import { app, appKeyId, signInRevoked } from "./app";
import { Age, Badge } from "./badge";
import { hid } from "./cls";
import { Copy } from "./copy";
import { DateTime, Field, FormFooter, Switch, TextField, useDraft } from "./field";
import { useHelp } from "./help";
import { useLang } from "./lang";
import { isMe, own } from "./me";
import { BusyButton, useBusy } from "./question";
import { session, stillSession } from "./session";
import { fmtWhen, isoOf, localOf } from "./time";

/**
 * KeyLife is an API key's line of its life (AP KeyLife, Blocks 3 and 4):
 * ended, the badge Ended with since when it was unused (idle) or when its
 * end came (expired); else when its end date comes, where it has one, and
 * kept while unused, or when it ends unless used where the idle limit
 * holds it and that comes before its end date; nothing for a session, nor
 * for the settings file's key, whose own line says where it comes from.
 */
function KeyLife({ tk }: { tk: Token }) {
  if (tk.kind !== "api" || tk.from_settings) return null;
  if (tk.ended) {
    const text = tk.ended === "idle" ? t("users.endedIdle", { date: fmtWhen(tk.last_used_at || tk.created_at) })
      : t("users.endedExpired", { date: tk.expires_at ? fmtWhen(tk.expires_at) : "?" });
    return (
      <>
        <Badge kind="bad" text={t("users.ended")} />
        <span class="muted small token-life">{text}</span>
      </>
    );
  }
  const end = tk.expires_at ? <span class="muted small token-end">{t("users.keyEnds", { date: fmtWhen(tk.expires_at) })}</span> : null;
  const first = !tk.expires_at || (!!tk.idle_ends_at && Date.parse(tk.idle_ends_at) < Date.parse(tk.expires_at));
  const idle = tk.idle_exempt ? <span class="muted small token-life">{t("users.idleKept")}</span>
    : tk.idle_ends_at && first ? <span class="muted small token-life">{t("users.idleEnds", { date: fmtWhen(tk.idle_ends_at) })}</span>
    : null;
  return end || idle ? <>{end}{idle}</> : null;
}

/**
 * KeyChange is Change… of an API key's life (AP KeyLife, Blocks 3 and
 * 4): where keeps, the switch Keep while unused, its draft against the
 * key's idle_exempt; where ends, the field Ends, its draft against the
 * key's end date, empty for none, and No end, which empties it; Save, off
 * until one is changed, which sends what changed by PUT …/tokens/{id}
 * (setTokenLife) and closes the form once it is taken, and Cancel, which
 * closes it; done closes it, giving the focus back to Change…. The form's
 * first control takes the focus as it comes, in its layout effect. id is
 * the end field's.
 */
function KeyChange({ u, tk, run, done, keeps, ends, id }: { u: User; tk: Token; run: Run; done: () => void; keeps: boolean; ends: boolean; id: string }) {
  useLang();
  const keep = useDraft(!!tk.idle_exempt);
  const end = useDraft(localOf(tk.expires_at));
  const busy = useBusy();
  const form = useRef<HTMLFormElement>(null);
  // the form comes where Change… was pressed: the focus on its first control
  useLayoutEffect(() => {
    form.current?.querySelector<HTMLInputElement>("input")?.focus();
  }, []);
  const changed = (keeps && keep.changed) || (ends && end.changed);
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (busy.busy || !changed) return;
    const life: TokenLife = {};
    if (keeps && keep.changed) life.idle_exempt = keep.value;
    if (ends && end.changed) life.expires_at = isoOf(end.value);
    busy.run(() => setTokenLife(run, u.name, tk, life)).then((ok) => {
      if (ok) done();
    });
  };
  return (
    <form ref={form} class="token-change" onSubmit={submit}>
      {keeps && <Switch label={t("users.keepUnused")} name="idle_exempt" draft={keep} cls="token-keep" />}
      {ends && (
        <Field label={t("users.keyEndsField")} id={id} cls="token-end-field">
          <span class="inline field-control">
            <DateTime id={id} cls="token-end-input" step={60} value={end.value} set={end.set} />
            <button type="button" class="token-no-end" onClick={() => end.set("")}>{t("users.keyNoEnd")}</button>
          </span>
        </Field>
      )}
      <FormFooter submit={t("action.save")} busy={busy.busy} off={!changed} cancel={done} />
    </form>
  );
}

/**
 * TokenRow is one session or key, with its id and Revoke; labels names a
 * session's key and a key's maker by the list's labels, else by its id;
 * marked tints it (token-marked). Of
 * one's own user, the token the page is signed in with says "this
 * device", the key that session was made of "its key"; in the app the key
 * the phone keeps, of the id deviceKey, says "this device's key" in its
 * place; Revoke on "this device" signs out. The settings file's key has
 * no Revoke, and says where it is changed in its place. A key's line
 * says its life (KeyLife); where keeps or ends, an API key's row has
 * Change… (KeyChange), with the switch where keeps and the end date where
 * ends, but the settings file's key and the key the page is signed in
 * with, a key changing nothing of its own life (AP KeyLife, D4-A); Cancel
 * and a Save taken give the focus back to Change…. idBase is the start of
 * the ids of its fields.
 */
function TokenRow({ u, tk, labels, run, marked, deviceKey, keeps, ends, idBase }: {
  u: User;
  tk: Token;
  labels: Map<number, string>;
  run: Run;
  marked: boolean;
  deviceKey: number;
  keeps: boolean;
  ends: boolean;
  idBase: string;
}) {
  const [changing, setChanging] = useState(false);
  const changeRef = useRef<HTMLButtonElement>(null);
  const back = useRef(false);
  const kind = t("users.kind." + tk.kind);
  const mine = isMe(u.name) ? own.value : null;
  const current = !!mine && mine.id === tk.id;
  const device = tk.kind === "api" && deviceKey > 0 && tk.id === deviceKey;
  const itsKey = !device && !!mine && !!mine.key_id && mine.key_id === tk.id;
  const tag = current ? "users.thisDevice" : device ? "users.thisDeviceKey" : itsKey ? "users.itsKey" : "";
  const cls = [current ? "token-current" : device ? "token-device-key" : itsKey ? "token-its-key" : "", marked ? "token-marked" : ""].filter(Boolean).join(" ") || undefined;
  const changeable = (keeps || ends) && tk.kind === "api" && !tk.from_settings && !current;
  // the form closed: the focus back on Change…, in the drawing that shows it again
  useLayoutEffect(() => {
    if (!changing && back.current) {
      back.current = false;
      changeRef.current?.focus();
    }
  }, [changing]);
  return (
    <li data-id={String(tk.id)} class={cls}>
      <span class="flag mode token-kind">{kind}</span>
      <span class="muted small token-id">{t("users.tokenId", { id: tk.id })}</span>
      {tag && <Badge kind="ok" text={t(tag)} />}
      <span class="token-label">{tk.label || ""}</span>
      <span class={"muted small token-key" + hid(!tk.key_id)}>{tk.key_id ? t("users.fromKey", { label: labels.get(tk.key_id) || "#" + tk.key_id }) : ""}</span>
      <span class={"muted small token-maker" + hid(!tk.made_by)}>{tk.made_by ? t("users.madeBy", { label: labels.get(tk.made_by) || "#" + tk.made_by }) : ""}</span>
      <span class="muted small token-used">
        {tk.last_used_at ? <>{t("users.lastUsed", { from: tk.last_used_from || "?" }) + " "}<Age at={tk.last_used_at} plain /></> : t("users.neverUsed")}
      </span>
      <KeyLife tk={tk} />
      {tk.from_settings
        ? <span class="muted small token-settings">{t("users.fromSettings")}</span>
        : (
          <span class="token-buttons">
            {changeable && (
              <button ref={changeRef} type="button" class={"change-key" + hid(changing)} aria-label={t("users.changeKeyLabel", { label: tk.label || kind })}
                onClick={() => setChanging(true)}>
                {t("users.changeKey")}
              </button>
            )}
            <BusyButton cls="revoke-token" label={t("users.revoke")} aria={t("users.revokeLabel", { label: tk.label || kind })}
              run={() => (current ? revokeOwn(u.name, tk) : revokeToken(run, u.name, tk))} />
          </span>
        )}
      {changeable && changing && <KeyChange u={u} tk={tk} run={run} keeps={keeps} ends={ends} id={idBase + "-end-" + tk.id} done={() => {
        back.current = true;
        setChanging(false);
      }} />}
    </li>
  );
}

/**
 * Tokens is the box of a user's sessions and keys, hidden where off;
 * open says that its place is shown (a user's panel open, My profile
 * read), and reads, the caller's counter of changes, asks for the list
 * again by its change; run sends a change through the caller's runner
 * and say tells its message line why nothing was sent; labelId is the
 * new key's field's id; help, where given, is the key of the group's
 * help, its button after Create key and its explanation at the box's
 * end (My profile's); marked, where given, is the id of the API key
 * whose row is tinted, a session of that id marking nothing; drawn, where
 * given, is told each time the list a read gave is drawn, with the row
 * marked or null (My profile's); listOnly lists them, each with its
 * Revoke, and offers no new key (an integration's user's); keeps says
 * that the caller may keep the user's keys from the idle limit, who
 * manages the user (AP KeyLife, D7-A): Create key's switch Keep while
 * unused, the part's own state until a key is made, and each key's
 * Change…; ends says that the caller may move the user's keys' end
 * dates, the user itself or who manages it (Block 4, D4-A): each key's
 * Change…, an integration's key's too. Create key's field Ends, empty for
 * none, is the part's own state until a key is made.
 */
export function Tokens({ u, off, open, reads, run, say, labelId, help, marked, drawn, listOnly, keeps = false, ends = false }: {
  u: User;
  off: boolean;
  open: boolean;
  reads: number;
  run: Run;
  say: (text: string, ok: boolean) => void;
  labelId: string;
  help?: string;
  marked?: number | null;
  drawn?: (row: HTMLElement | null) => void;
  listOnly?: boolean;
  keeps?: boolean;
  ends?: boolean;
}) {
  useLang();
  const deviceKey = appKeyId(app.value); // the key the phone keeps for this server, in the app
  const helps = useHelp(help || "", t("users.tokens"));
  const [list, setList] = useState<Token[] | null>(null);
  const [error, setError] = useState("");
  const [shown, setShown] = useState<string | null>(null);
  const made = useRef<string | null>(null);
  const [label, setLabel] = useState("");
  const [keep, setKeep] = useState(false);
  const [end, setEnd] = useState("");
  const ul = useRef<HTMLUListElement>(null);
  // read when its place opens and at every change of the caller's counter while it is open; the key just made shown by this read alone
  // biome-ignore lint/correctness/useExhaustiveDependencies: reads is the caller's change counter, which asks for the read again by its change alone.
  useEffect(() => {
    if (!open || off) return;
    let stale = false;
    const n = session();
    // in the app, once the last sign-in's revoke of the keys and sessions left is done: none of those is listed (D67-A)
    signInRevoked().then(() => (stale || !stillSession(n) ? null : loadTokens(u.name))).then((l) => {
      if (l === null) return;
      if (stale || !stillSession(n)) return;
      setList(l);
      setError("");
      setShown(made.current);
      made.current = null;
    }, (e: Error) => {
      if (stale || !stillSession(n)) return;
      setList(null);
      setError(e.message);
    });
    return () => {
      stale = true;
    };
  }, [open, off, reads, u.name]);
  // the list a read gave drawn: the caller told, with the row marked, in the drawing's own turn
  // biome-ignore lint/correctness/useExhaustiveDependencies: a read drawn alone is told, by the list it gave; drawn is the caller's, read as it is then.
  useLayoutEffect(() => {
    if (list && drawn) drawn(ul.current?.querySelector<HTMLElement>("li.token-marked") || null);
  }, [list]);
  const labels = new Map((list || []).map((tk) => [tk.id, tk.label || t("users.kind." + tk.kind)]));
  const make = async () => {
    const l = label.trim();
    if (!l) {
      say(t("users.keyLabelNeeded"), false);
      return;
    }
    if (await createKey(run, u.name, l, (key) => { made.current = key; }, keeps && keep, isoOf(end))) {
      setLabel("");
      setKeep(false);
      setEnd("");
    }
  };
  return (
    <div class={"user-tokens-box" + hid(off)}>
      <span class="muted small">{t("users.tokens")}</span>
      <ul ref={ul} class="user-tokens">
        {error && <li class="muted small bad">{error}</li>}
        {list && !list.length && <li class="muted small">{t("users.tokensNone")}</li>}
        {(list || []).map((tk) => (
          <TokenRow key={tk.id} u={u} tk={tk} labels={labels} run={run} marked={tk.kind === "api" && tk.id === marked} deviceKey={deviceKey} keeps={keeps && !listOnly}
            ends={ends} idBase={labelId} />
        ))}
      </ul>
      {!listOnly && (
        <>
          <div class={"user-key-box" + hid(!shown)}>
            <p class="small">{t("users.keyShown")}</p>
            <p><Copy key={shown || ""} text={shown || ""} name={t("users.keyCopy")} icon cls="mono user-key" /></p>
          </div>
          <TextField label={t("users.newKey")} id={labelId} name="label" maxLength={64} placeholder={t("users.keyLabel")} value={label} set={setLabel} />
          {keeps && <Switch label={t("users.keepUnused")} id={labelId + "-keep"} name="idle_exempt" cls="key-keep" checked={keep} onChange={setKeep} />}
          <Field label={t("users.keyEndsField")} id={labelId + "-end"} cls="key-end" hint={t("users.keyEndsHint")} hintId={labelId + "-end-hint"}>
            <DateTime id={labelId + "-end"} cls="field-control" step={60} value={end} set={setEnd} />
          </Field>
          <div class="actions">
            <span class="actions-right">
              <BusyButton cls="create-key" label={t("users.createKey")} run={make} />
              {help && helps.button}
            </span>
          </div>
          {help && helps.text}
        </>
      )}
    </div>
  );
}
