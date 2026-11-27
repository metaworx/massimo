// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/serverbackups/Keys.tsx
 *
 * Server backups' keys (AP ServerBackup, Block 4, D4-A, D17-C), for a
 * holder of backup.server, who reads them; the owner alone adds,
 * confirms and removes one. A row per key: its label, its kind, its
 * fingerprint with a copy of its public key, which is long for a
 * post-quantum one; a key made and not confirmed says until when it
 * waits, shows its secret part once, as text and as a file to save, while
 * this page holds it, and asks for its last six characters. Add a key
 * pastes a public key of the owner's own, or makes one of the kind
 * chosen, post-quantum first, classic second, each with its line; the
 * kind of the keys set is offered first, and a mix is refused with why.
 */

import type { Ref } from "preact";
import { useState } from "preact/hooks";
import type { ServerBackupKey } from "../api";
import { Adder } from "../shared/adder";
import { Badge, Notice, ResultLine } from "../shared/badge";
import { Copy, CopyIcon } from "../shared/copy";
import { Select, TextField } from "../shared/field";
import { useLook } from "../shared/look";
import { ownerOf } from "../shared/me";
import { Question, busyProps, useAsk, useBusy } from "../shared/question";
import { saveText } from "../shared/save";
import { fmtWhen } from "../shared/time";
import { addKey, confirmKey, removeKey } from "./send";
import { card, type Made, sb } from "./store";

/** keyName is a key as a text names it: its label, else its fingerprint. */
function keyName(k: ServerBackupKey): string {
  return k.label || k.fingerprint || "";
}

/** fileName is the name a key made is saved under: its label's letters and digits, else its uuid's end. */
function fileName(k: ServerBackupKey): string {
  const base = (k.label || "").replace(/[^\p{L}\p{N}._-]+/gu, "-").replace(/^-+|-+$/g, "") || (k.uuid || "").slice(-12);
  return "massimo-backup-key-" + base + ".txt";
}

/** MadeKey is a key made, shown once while this page holds it: the note, its secret part as text with its copy, and the key file to save. */
function MadeKey({ k, made }: { k: ServerBackupKey; made: Made }) {
  useLook();
  return (
    <div class="sb-made">
      <Notice role="note" text={t("serverbackup.key.madeNote")} />
      <p class="action-title">{t("serverbackup.key.identity")}</p>
      <Copy id={"sb-key-" + k.uuid + "-identity"} cls="mono sb-identity" text={made.identity} icon name={t("serverbackup.key.copyIdentity")} />
      <div class="actions sb-key-actions">
        <button type="button" class="sb-key-save" onClick={() => saveText(fileName(k), made.file)}>{t("serverbackup.key.save")}</button>
      </div>
    </div>
  );
}

/** Confirm is a pending key's last six characters and Confirm, for the owner: typed back they make it a key. */
function Confirm({ k }: { k: ServerBackupKey }) {
  useLook();
  const [six, setSix] = useState("");
  const send = useBusy();
  const id = "sb-key-" + k.uuid + "-six";
  const submit = (ev: Event) => {
    ev.preventDefault();
    void send.run(() => confirmKey(k, six.trim()));
  };
  return (
    <form class="sb-key-confirm" onSubmit={submit}>
      <TextField label={t("serverbackup.key.lastSix")} id={id} name="last_six" inputCls="mono" maxLength={6} required value={six} set={setSix}
        hint={t("serverbackup.key.lastSixHint")} />
      <div class="actions sb-key-actions">
        <button type="submit" {...busyProps("primary sb-key-confirm-btn", send.busy, six.trim().length !== 6)}>{t("serverbackup.key.confirm")}</button>
      </div>
    </form>
  );
}

/** KeyRow is a key: its label, kind and fingerprint with its public key's copy; pending, until when, the key made and its confirmation; Remove for the owner, which asks. */
function KeyRow({ k }: { k: ServerBackupKey }) {
  useLook();
  const owner = ownerOf();
  const ask = useAsk<HTMLButtonElement>();
  const made = card.made && card.made.uuid === k.uuid ? card.made : null;
  return (
    <li ref={ask.row as Ref<HTMLLIElement>} class="sb-key" data-uuid={k.uuid}>
      <div class="sb-row-head">
        <span class="panel-name">{keyName(k)}</span>
        <Badge kind="mode" text={t("serverbackup.key.kind." + k.kind)} />
        {k.pending && <Badge kind="warn" text={t("serverbackup.key.pendingBadge")} />}
      </div>
      <p class="muted small sb-key-fp">
        <span class="mono">{k.fingerprint}</span> <CopyIcon get={() => k.recipient || ""} label={t("serverbackup.key.copyRecipient", { name: keyName(k) })} />
      </p>
      {k.pending && k.expires_at && <p class="small sb-key-pending">{t("serverbackup.key.pending", { when: fmtWhen(k.expires_at) })}</p>}
      {made && <MadeKey k={k} made={made} />}
      {k.pending && owner && <Confirm k={k} />}
      {owner && (ask.asking ? (
        <Question text={t("serverbackup.key.removeAsk", { name: keyName(k) })} yes={t("serverbackup.key.remove")} no={t("action.keep")} boxRef={ask.box}
          onNo={() => ask.done()} onYes={async () => {
            if (!(await removeKey(k))) ask.done();
          }} />
      ) : (
        <div class="actions sb-key-actions">
          <button ref={ask.ref} type="button" class="sb-key-remove" onClick={ask.ask}>{t("serverbackup.key.remove")}</button>
        </div>
      ))}
    </li>
  );
}

/** NewKey is the form that adds a key: its label, then the way, a key made of one kind or a public key pasted, each with its line. */
function NewKey() {
  useLook();
  // the kind of the keys set first, so that the box's keys stay of one kind (D17-C); post-quantum where none is set
  const [way, setWay] = useState(sb.keys.length && !sb.keys.some((k) => k.kind === "pq") ? "classic" : "pq");
  const [label, setLabel] = useState("");
  const [recipient, setRecipient] = useState("");
  const add = useBusy();
  const submit = (ev: Event) => {
    ev.preventDefault();
    void add.run(() => addKey(label.trim(), way, recipient.trim())).then((ok) => {
      if (ok) card.addingKey = false;
    });
  };
  return (
    <form class="panel sb-key-form" onSubmit={submit}>
      <TextField label={t("serverbackup.key.label")} id="sb-key-label" name="label" maxLength={100} placeholder={t("serverbackup.key.labelPlaceholder")}
        value={label} set={setLabel} />
      <Select label={t("serverbackup.key.way")} id="sb-key-way" name="way" wide value={way} onChange={setWay} hint={t("serverbackup.key.line." + way)}
        hintId="sb-key-way-line" options={[["pq", t("serverbackup.key.way.pq")], ["classic", t("serverbackup.key.way.classic")], ["paste", t("serverbackup.key.way.paste")]]} />
      {way === "paste" && (
        <TextField label={t("serverbackup.key.recipient")} id="sb-key-recipient" name="recipient" inputCls="mono" required placeholder="age1…"
          value={recipient} set={setRecipient} />
      )}
      <div class="actions">
        <span class="actions-right">
          <button type="button" class="form-cancel" onClick={() => { card.addingKey = false; }}>{t("action.cancel")}</button>
          <button type="submit" {...busyProps("primary", add.busy)}>{t(way === "paste" ? "serverbackup.key.addOne" : "serverbackup.key.make")}</button>
        </span>
      </div>
    </form>
  );
}

/** Keys is the keys' part of the card: what a key is for, the rows, the form that adds one for the owner, and the line. */
export function Keys() {
  useLook();
  const owner = ownerOf();
  const keys = sb.keys;
  return (
    <section class="sb-group sb-keys" aria-labelledby="sb-keys-h">
      <h4 id="sb-keys-h" class="sb-group-title">{t("serverbackup.keys")}</h4>
      <p class="muted small">{t("serverbackup.keysText")}</p>
      {sb.keysLoaded && !keys.length && <p class="muted small sb-keys-none">{t("serverbackup.keysNone")}</p>}
      <ul class="sb-key-list">{keys.map((k) => <KeyRow key={k.uuid} k={k} />)}</ul>
      {owner ? (
        <Adder id="sb-key-add" formId="sb-key-new" label={t("serverbackup.key.add")} shown={card.addingKey} set={(v) => { card.addingKey = v; }}>
          <NewKey />
        </Adder>
      ) : <p class="muted small">{t("serverbackup.key.ownerOnly")}</p>}
      <ResultLine id="sb-keys-msg" cls="small cubes-msg" role="status" res={card.keys} />
    </section>
  );
}
