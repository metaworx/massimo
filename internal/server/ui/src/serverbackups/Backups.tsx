// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/serverbackups/Backups.tsx
 *
 * Server backups' list (AP ServerBackup, Block 4, D13-A): a fold per
 * backup, newest first, built once per backup and updated in place, its
 * head when it was made, why (the trigger), its size and its marks
 * (failed, a copy failed, pinned, made by another box); inside, who
 * asked, its label and version, a failed one's error and next try, its
 * copies per target (there, missing, failed with the error, or not
 * there) and the actions who is in may take: with backup.server
 * Download, Check, Pin or Unpin, and on a copy Push, Pull and Delete,
 * which asks, saying so where no other copy is there, and the backup
 * goes from the list with its last copy; for the owner alone Restore…
 * and Check with the key…, which open the dialog (Restore.tsx, AP
 * ServerBackup, Block 5); with
 * backup.retry Retry where something of it failed; with backup.server
 * Delete, which asks, of a backup no target holds a copy of. A holder of
 * backup.retry alone sees the state and Retry, nothing more.
 */

import type { Ref } from "preact";
import type { ServerBackup, ServerBackupCopy, ServerBackupTargetState } from "../api";
import { Badge, ResultLine } from "../shared/badge";
import { Fold, FoldHead } from "../shared/fold";
import { useLook } from "../shared/look";
import { can, ownerOf } from "../shared/me";
import { BusyButton, Question, useAsk } from "../shared/question";
import { fmtWhen } from "../shared/time";
import { openRestore } from "./restoring";
import { check, deleteBackup, deleteCopy, download, pin, pull, push, retry } from "./send";
import { backupUI, sb } from "./store";

/** sizeText is a file's size, in kB below a megabyte, else in MB. */
export function sizeText(n?: number): string {
  if (!n) return "";
  return n < 1024 * 1024 ? t("serverbackup.kB", { n: fmtNum(n / 1024, 1) }) : t("serverbackup.MB", { n: fmtNum(n / (1024 * 1024), 1) });
}

/** pinnedText says until when a backup is pinned, "" where it is not: a pin by hand lasts until it is taken back. */
function pinnedText(b: ServerBackup): string {
  const until = Date.parse(b.pinned_until || "");
  if (isNaN(until) || until <= Date.now()) return "";
  return new Date(until).getUTCFullYear() >= 9999 ? t("serverbackup.pinned") : t("serverbackup.pinnedUntil", { when: fmtWhen(b.pinned_until || "") });
}

/** COPY_KIND is a copy's state as its badge's kind: nothing marked for one that is there or not there, a warning for one missing, a failure for one that failed. */
const COPY_KIND: Record<string, string> = { there: "mode", none: "mode", missing: "warn", failed: "bad" };

/**
 * CopyRow is a backup's copy on a target, or its place where the target
 * has none: the target, the copy's state, a failed one's error and next
 * try; with backup.server Push where it is not there, Pull to this box
 * from another target, and Delete, which asks in the row, saying where
 * no other copy is there (alone); last is the backup's last copy of any
 * state, which takes the backup from the list.
 */
function CopyRow({ b, target, c, alone, last, localThere }: {
  b: ServerBackup;
  target?: ServerBackupTargetState;
  c?: ServerBackupCopy;
  alone: boolean;
  last: boolean;
  localThere: boolean;
}) {
  useLook();
  const ask = useAsk<HTMLButtonElement>();
  const server = can("backup.server");
  const state = c?.state || "none";
  const name = target?.name || c?.target_name || "";
  const error = state === "failed" && c?.error ? c.error : "";
  const next = state === "failed" && c?.next_try ? t("serverbackup.nextTry", { when: fmtWhen(c.next_try) }) : "";
  const canPush = server && !!target && state !== "there" && b.state === "made";
  const canPull = server && !!c && state === "there" && target?.kind !== "local" && !localThere;
  return (
    <li ref={ask.row as Ref<HTMLLIElement>} class="sb-copy" data-target={target?.uuid || c?.target} data-state={state}>
      <div class="sb-row-head">
        <span class="sb-copy-name">{name}</span>
        <Badge kind={COPY_KIND[state] || "mode"} text={t("serverbackup.copy." + state)} />
        {c?.pulled && <Badge kind="mode" text={t("serverbackup.copy.pulled")} />}
        {server && !ask.asking && (
          <span class="actions sb-copy-actions">
            {canPush && <BusyButton cls="small sb-push" label={t("serverbackup.push")} run={() => push(b, target as ServerBackupTargetState)} />}
            {canPull && <BusyButton cls="small sb-pull" label={t("serverbackup.pull")} run={() => pull(b, c as ServerBackupCopy)} />}
            {c && <button ref={ask.ref} type="button" class="small sb-copy-delete" onClick={ask.ask}>{t("action.delete")}</button>}
          </span>
        )}
      </div>
      {(error || next) && <p class="small sb-copy-error">{[error, next].filter(Boolean).join(" · ")}</p>}
      {ask.asking && c && (
        <Question text={t(alone ? "serverbackup.deleteLastAsk" : "serverbackup.deleteAsk", { target: name })} yes={t("action.delete")} no={t("action.cancel")}
          boxRef={ask.box} onNo={() => ask.done()} onYes={async () => {
            // refused, the question stays, the message under the backup
            if (await deleteCopy(b, c, last)) ask.done(false);
          }} />
      )}
    </li>
  );
}

/**
 * DeleteBackup is Delete of a backup no target holds a copy of, one that
 * failed or whose copies are missing, for a holder of backup.server: it
 * asks in its place first, and the backup goes from the list (DELETE
 * …/{id}); refused, the question stays, the message under the backup.
 */
function DeleteBackup({ b }: { b: ServerBackup }) {
  useLook();
  const ask = useAsk<HTMLButtonElement>();
  if (!ask.asking) return <button ref={ask.ref} type="button" class="small sb-backup-delete" onClick={ask.ask}>{t("action.delete")}</button>;
  return (
    <Question text={t("serverbackup.deleteBackupAsk", { when: b.time ? fmtWhen(b.time) : "" })} yes={t("action.delete")} no={t("action.cancel")}
      boxRef={ask.box} onNo={() => ask.done()} onYes={async () => {
        if (await deleteBackup(b)) ask.done(false);
      }} />
  );
}

/** facts are a backup's lines inside its fold: who asked, its label, the version that made it, and a failed one's error with its next try. */
function facts(b: ServerBackup): string[] {
  const out = [];
  const who = [b.by ? t("serverbackup.by", { user: b.by }) : "", b.version ? t("serverbackup.version", { version: b.version }) : ""].filter(Boolean).join(" · ");
  if (who) out.push(who);
  if (b.label) out.push(t("serverbackup.label", { label: b.label }));
  if (!b.this_box && b.node) out.push(t("serverbackup.otherBoxText", { node: b.node }));
  return out;
}

/** BackupRow is a backup's fold: its head; inside, its facts, a failed one's error, its copies per target and its actions; its line. */
function BackupRow({ b, targets }: { b: ServerBackup; targets: ServerBackupTargetState[] }) {
  useLook();
  const uuid = b.uuid || "";
  const ui = backupUI.of(uuid);
  const server = can("backup.server");
  const copies = b.copies || [];
  const failed = copies.filter((c) => c.state === "failed").length;
  const there = copies.filter((c) => c.state === "there");
  const localThere = there.some((c) => targets.find((s) => s.uuid === c.target)?.kind === "local");
  const mayRetry = can("backup.retry") && (b.state === "failed" || failed > 0);
  const pinned = pinnedText(b);
  // a copy per target, those of a target the list no longer has after them
  const rows = [
    ...targets.map((s) => ({ key: s.uuid || "", target: s as ServerBackupTargetState | undefined, c: copies.find((c) => c.target === s.uuid) })),
    ...copies.filter((c) => !targets.some((s) => s.uuid === c.target)).map((c) => ({ key: c.target || "", target: undefined, c })),
  ];
  const badges = [
    <span key="trigger" class="flag mode sb-trigger">{t("serverbackup.trigger." + b.trigger)}</span>,
    b.state === "failed" && <Badge key="failed" kind="bad" text={t("serverbackup.failed")} />,
    failed > 0 && <Badge key="copy" kind="bad" text={t("serverbackup.copyFailed")} />,
    pinned && <Badge key="pinned" kind="mode" text={pinned} />,
    !b.this_box && !!b.node && <Badge key="other" kind="warn" text={t("serverbackup.otherBox")} />,
  ];
  return (
    <Fold cls="panel sb-backup" open={ui.open} onToggle={(o) => { ui.open = o; }} data={{ uuid, state: b.state || "" }}
      summary={<FoldHead name={b.time ? fmtWhen(b.time) : ""} after={<span class="muted small sb-size">{sizeText(b.size)}</span>} badges={badges} />}>
      {facts(b).map((x, i) => <p key={i} class="muted small sb-fact">{x}</p>)}
      {b.state === "failed" && (
        <p class="small sb-backup-error">
          {[b.error || "", b.next_try ? t("serverbackup.nextTry", { when: fmtWhen(b.next_try) }) : ""].filter(Boolean).join(" · ")}
        </p>
      )}
      {b.state !== "failed" && (
        <ul class="sb-copies" aria-label={t("serverbackup.copies")}>
          {rows.map((r) => <CopyRow key={r.key} b={b} target={r.target} c={r.c} alone={!!r.c && !there.some((x) => x !== r.c)}
              last={copies.length <= 1 && !!r.c} localThere={localThere} />)}
        </ul>
      )}
      <div class="actions sb-backup-actions">
        {server && b.state === "made" && there.length > 0 && <BusyButton cls="small sb-download" label={t("serverbackup.download")} run={() => download(b)} />}
        {server && b.state === "made" && there.length > 0 && <BusyButton cls="small sb-check" label={t("serverbackup.check")} run={() => check(b)} />}
        {server && b.state === "made" && <BusyButton cls="small sb-pin" label={t(pinned ? "serverbackup.unpin" : "serverbackup.pin")} run={() => pin(b, !pinned)} />}
        {ownerOf() && b.state === "made" && there.length > 0 && (
          <>
            <button type="button" class="small sb-restore" onClick={() => openRestore(b, "restore")}>{t("serverbackup.restore")}</button>
            <button type="button" class="small sb-check-key" onClick={() => openRestore(b, "check")}>{t("serverbackup.checkKey")}</button>
          </>
        )}
        {mayRetry && <BusyButton cls="small sb-retry" label={t("serverbackup.retry")} run={() => retry(b)} />}
        {server && !there.length && <DeleteBackup b={b} />}
      </div>
      <ResultLine cls="small cubes-msg sb-backup-msg" role="status" res={ui.said} />
    </Fold>
  );
}

/** Backups is the backups' part of the card: a fold per backup, newest first, or that there is none yet. */
export function Backups() {
  useLook();
  const l = sb.list;
  const list = l?.backups || [];
  const targets = l?.targets || [];
  return (
    <section class="sb-group sb-backups" aria-labelledby="sb-backups-h">
      <h4 id="sb-backups-h" class="sb-group-title">{t("serverbackup.backups")}</h4>
      {l && !list.length && <p class="muted small sb-none">{t("serverbackup.none")}</p>}
      <div class="sb-backup-list">{list.map((b) => <BackupRow key={b.uuid} b={b} targets={targets} />)}</div>
    </section>
  );
}
