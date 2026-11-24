// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/tokens.tsx
 *
 * A user's sessions and API keys, wherever they are drawn: in a user's
 * panel of System settings → Users and in My profile. Each with its
 * kind, its id, label, the key a session was made from, when and from
 * where it was last used, and Revoke; of one's own user the token the
 * page is signed in with says "this device" and the key that session was
 * made of "its key", and Revoke on "this device" signs out; in the app the
 * key the phone keeps for the server (its status' key_id, shared/app.ts)
 * says "this device's key", in place of "its key" where both hold,
 * however the session was made (AP MobileApp, D65-A, D66-A); the label of
 * a new key with Create key, and the key just made, shown once. The list
 * is read when its place opens and again at every change of the caller's
 * counter while it is open, in the app once the app has revoked what the
 * last sign-in revokes (D67-A); the key just made is shown by the read
 * that follows its making and gone at the next, kept in this part's own
 * state alone. An API key the caller names is marked, its row tinted,
 * and the caller is told of every read drawn, with that row, to bring it
 * into view (My profile's key of its address). Moved from System
 * settings' users (AP UiParts, Block 13), its texts the users.… keys both
 * places use.
 */

import { useEffect, useLayoutEffect, useRef, useState } from "preact/hooks";
import type { Token, User } from "../api";
import { createKey, loadTokens, revokeOwn, revokeToken, type Run } from "./account";
import { app, appKeyId, signInRevoked } from "./app";
import { Age, Badge } from "./badge";
import { hid } from "./cls";
import { TextField } from "./field";
import { useHelp } from "./help";
import { useLang } from "./lang";
import { isMe, own } from "./me";
import { BusyButton } from "./question";
import { session, stillSession } from "./session";

/**
 * TokenRow is one session or key, with its id and Revoke; labels names a
 * session's key by the list's labels; marked tints it (token-marked). Of
 * one's own user, the token the page is signed in with says "this
 * device", the key that session was made of "its key"; in the app the key
 * the phone keeps, of the id deviceKey, says "this device's key" in its
 * place; Revoke on "this device" signs out.
 */
function TokenRow({ u, tk, labels, run, marked, deviceKey }: {
  u: User;
  tk: Token;
  labels: Map<number, string>;
  run: Run;
  marked: boolean;
  deviceKey: number;
}) {
  const kind = t("users.kind." + tk.kind);
  const mine = isMe(u.name) ? own.value : null;
  const current = !!mine && mine.id === tk.id;
  const device = tk.kind === "api" && deviceKey > 0 && tk.id === deviceKey;
  const itsKey = !device && !!mine && !!mine.key_id && mine.key_id === tk.id;
  const tag = current ? "users.thisDevice" : device ? "users.thisDeviceKey" : itsKey ? "users.itsKey" : "";
  const cls = [current ? "token-current" : device ? "token-device-key" : itsKey ? "token-its-key" : "", marked ? "token-marked" : ""].filter(Boolean).join(" ") || undefined;
  return (
    <li data-id={String(tk.id)} class={cls}>
      <span class="flag mode token-kind">{kind}</span>
      <span class="muted small token-id">{t("users.tokenId", { id: tk.id })}</span>
      {tag && <Badge kind="ok" text={t(tag)} />}
      <span class="token-label">{tk.label || ""}</span>
      <span class={"muted small token-key" + hid(!tk.key_id)}>{tk.key_id ? t("users.fromKey", { label: labels.get(tk.key_id) || "#" + tk.key_id }) : ""}</span>
      <span class="muted small token-used">
        {tk.last_used_at ? <>{t("users.lastUsed", { from: tk.last_used_from || "?" }) + " "}<Age at={tk.last_used_at} plain /></> : t("users.neverUsed")}
      </span>
      <BusyButton cls="revoke-token" label={t("users.revoke")} aria={t("users.revokeLabel", { label: tk.label || kind })}
        run={() => (current ? revokeOwn(u.name, tk) : revokeToken(run, u.name, tk))} />
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
 * Revoke, and offers no new key (an integration's user's).
 */
export function Tokens({ u, off, open, reads, run, say, labelId, help, marked, drawn, listOnly }: {
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
}) {
  useLang();
  const deviceKey = appKeyId(app.value); // the key the phone keeps for this server, in the app
  const helps = useHelp(help || "", t("users.tokens"));
  const [list, setList] = useState<Token[] | null>(null);
  const [error, setError] = useState("");
  const [shown, setShown] = useState<string | null>(null);
  const made = useRef<string | null>(null);
  const [label, setLabel] = useState("");
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
    if (await createKey(run, u.name, l, (key) => { made.current = key; })) setLabel("");
  };
  return (
    <div class={"user-tokens-box" + hid(off)}>
      <span class="muted small">{t("users.tokens")}</span>
      <ul ref={ul} class="user-tokens">
        {error && <li class="muted small bad">{error}</li>}
        {list && !list.length && <li class="muted small">{t("users.tokensNone")}</li>}
        {(list || []).map((tk) => (
          <TokenRow key={tk.id} u={u} tk={tk} labels={labels} run={run} marked={tk.kind === "api" && tk.id === marked} deviceKey={deviceKey} />
        ))}
      </ul>
      {!listOnly && (
        <>
          <div class={"user-key-box" + hid(!shown)}>
            <p class="small">{t("users.keyShown")}</p>
            <p class="mono user-key">{shown}</p>
          </div>
          <TextField label={t("users.newKey")} id={labelId} name="label" maxLength={64} placeholder={t("users.keyLabel")} value={label} set={setLabel} />
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
