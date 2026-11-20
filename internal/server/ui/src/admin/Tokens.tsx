// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/admin/Tokens.tsx
 *
 * A user's sessions and API keys in its panel of System settings → Users,
 * as users.js drew them: each with its kind, label, the key a session was
 * made from, when and from where it was last used, and Revoke; the label
 * of a new key with Create key, and the key just made, shown once. The
 * list is read when the panel opens and after every change of the users
 * while it is open; the key just made is shown by the read that follows
 * its making and gone at the next, kept in this part's own state alone.
 * My profile draws the same plain (profile.js) until AP UiParts' Block
 * 13, which takes this part to shared/.
 */

import { useEffect, useRef, useState } from "preact/hooks";
import type { Token, User } from "../api";
import { Age } from "../shared/badge";
import { hid } from "../shared/cls";
import { TextField } from "../shared/field";
import { BusyButton } from "../shared/question";
import { load, reads } from "./store";
import { createKey, loadTokens, revokeToken, usersMsg } from "./sendUsers";

/** TokenRow is one session or key, with Revoke; labels names a session's key by the list's labels. */
function TokenRow({ u, tk, labels }: { u: User; tk: Token; labels: Map<number, string> }) {
  const kind = t("users.kind." + tk.kind);
  return (
    <li data-id={String(tk.id)}>
      <span class="flag mode token-kind">{kind}</span>
      <span class="token-label">{tk.label || ""}</span>
      <span class={"muted small token-key" + hid(!tk.key_id)}>{tk.key_id ? t("users.fromKey", { label: labels.get(tk.key_id) || "#" + tk.key_id }) : ""}</span>
      <span class="muted small token-used">
        {tk.last_used_at ? <>{t("users.lastUsed", { from: tk.last_used_from || "?" }) + " "}<Age at={tk.last_used_at} plain /></> : t("users.neverUsed")}
      </span>
      <BusyButton cls="revoke-token" label={t("users.revoke")} aria={t("users.revokeLabel", { label: tk.label || kind })} run={() => revokeToken(u.name, tk)} />
    </li>
  );
}

/**
 * Tokens is the box, hidden where off (the keys are the manager's and
 * the user's own), read while open says the panel is.
 */
export function Tokens({ u, off, open }: { u: User; off: boolean; open: boolean }) {
  load.useLoad();
  const n = reads.value;
  const [list, setList] = useState<Token[] | null>(null);
  const [error, setError] = useState("");
  const [shown, setShown] = useState<string | null>(null);
  const made = useRef<string | null>(null);
  const [label, setLabel] = useState("");
  // read when the panel opens and at every change of the users while it is open; the key just made shown by this read alone
  // biome-ignore lint/correctness/useExhaustiveDependencies: n is the users' change counter, which asks for the read again by its change alone.
  useEffect(() => {
    if (!open || off) return;
    let stale = false;
    loadTokens(u.name).then((l) => {
      if (stale) return;
      setList(l);
      setError("");
      setShown(made.current);
      made.current = null;
    }, (e: Error) => {
      if (stale) return;
      setList(null);
      setError(e.message);
    });
    return () => {
      stale = true;
    };
  }, [open, off, n, u.name]);
  const labels = new Map((list || []).map((tk) => [tk.id, tk.label || t("users.kind." + tk.kind)]));
  const make = async () => {
    const l = label.trim();
    if (!l) {
      usersMsg(t("users.keyLabelNeeded"), false);
      return;
    }
    if (await createKey(u.name, l, (key) => { made.current = key; })) setLabel("");
  };
  return (
    <div class={"user-tokens-box" + hid(off)}>
      <span class="muted small">{t("users.tokens")}</span>
      <ul class="user-tokens">
        {error && <li class="muted small bad">{error}</li>}
        {list && !list.length && <li class="muted small">{t("users.tokensNone")}</li>}
        {(list || []).map((tk) => <TokenRow key={tk.id} u={u} tk={tk} labels={labels} />)}
      </ul>
      <div class={"user-key-box" + hid(!shown)}>
        <p class="small">{t("users.keyShown")}</p>
        <p class="mono user-key">{shown}</p>
      </div>
      <TextField label={t("users.newKey")} id={"user-key-label-" + u.name} name="label" maxLength={64} placeholder={t("users.keyLabel")} value={label} set={setLabel} />
      <div class="actions">
        <span class="actions-right">
          <BusyButton cls="create-key" label={t("users.createKey")} run={make} />
        </span>
      </div>
    </div>
  );
}
