// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/serverbackups/Card.tsx
 *
 * Maintenance → Server backups as a part (AP ServerBackup, Block 4,
 * D4-A, D13-A, D14-C, D17-C): at its top, while no key is set, that the
 * server makes no backup until one is, else when it makes them; with
 * backup.server Back up now and Upload a backup…; then the backups, the
 * targets and, for a holder of backup.server, the keys. The card's head
 * says that backups are off without a key, counts what failed and the
 * backups kept, and marks a target two of its periods behind.
 */

import { useLayoutEffect } from "preact/hooks";
import { Notice, ResultLine } from "../shared/badge";
import { type BadgeSpec, cardBadges } from "../shared/card";
import { FileButton } from "../shared/file";
import { useLook } from "../shared/look";
import { can, ownerOf } from "../shared/me";
import { BusyButton } from "../shared/question";
import { Backups } from "./Backups";
import { Keys } from "./Keys";
import { backUpNow, upload } from "./send";
import { card, sb } from "./store";
import { Targets } from "./Targets";

/** badgesOf are the card head's badges: off without a key, the failures, a target behind, the backups kept. */
function badgesOf(): BadgeSpec[] {
  const l = sb.list;
  if (!l) return [];
  const list = l.backups || [];
  const failed = list.filter((b) => b.state === "failed").length + list.reduce((n, b) => n + (b.copies || []).filter((c) => c.state === "failed").length, 0);
  const out: BadgeSpec[] = [];
  if (!l.keys) out.push({ text: t("serverbackup.badge.off"), kind: "warn" });
  if (failed) out.push({ text: tn("serverbackup.badge.failed", failed), kind: "bad" });
  if ((l.targets || []).some((s) => s.enabled && s.behind)) out.push({ text: t("serverbackup.target.behind"), kind: "warn" });
  if (list.length) out.push({ text: tn("serverbackup.badge.count", list.length), kind: "mode" });
  return out;
}

/** Status is the card's top: that backups are off without a key, or when they are made; Back up now and Upload a backup… with backup.server; the line. */
function Status() {
  useLook();
  const l = sb.list;
  const server = can("backup.server");
  const noKey = !!l && !l.keys;
  return (
    <div class="action-row sb-status">
      <div>
        {noKey && (
          <Notice id="sb-no-key" role="note" text={t("serverbackup.noKey") + " " + t(ownerOf() ? "serverbackup.noKeyOwner" : "serverbackup.noKeyOther")} />
        )}
        {l && !noKey && <p id="sb-schedule" class="muted small">{t("serverbackup.schedule", { time: l.time || "" })}</p>}
        {sb.failed && <p class="small cubes-msg bad" role="status">{t("serverbackup.readFailed", { error: sb.failed })}</p>}
        <ResultLine id="sb-msg" cls="small cubes-msg" role="status" res={card.backups} />
      </div>
      {server && (
        <div class="actions sb-status-actions">
          <FileButton id="sb-upload" cls="sb-upload" label={t("serverbackup.upload")} accept=".zip,application/zip" off={card.uploading}
            inputId="sb-upload-file" inputLabel={t("serverbackup.uploadFile")} onFile={(f) => void upload(f)} />
          <BusyButton id="sb-now" cls="primary" label={t("serverbackup.now")} off={!l || noKey} run={backUpNow} />
        </div>
      )}
    </div>
  );
}

/** Card is the card's body, with its head's badges. */
export function Card() {
  useLook();
  const badges = badgesOf();
  const key = badges.map((b) => b.kind + ":" + b.text).join("|");
  // the card's head says it in its badges, also while it is folded
  // biome-ignore lint/correctness/useExhaustiveDependencies: badges is made anew at each drawing; key is what it says
  useLayoutEffect(() => {
    cardBadges("sys-server-backups", badges);
  }, [key]);
  return (
    <>
      <Status />
      <Backups />
      <Targets />
      {can("backup.server") && <Keys />}
    </>
  );
}
