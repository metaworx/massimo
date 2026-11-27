// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/serverbackups/store.ts
 *
 * What Maintenance → Server backups is drawn from (AP ServerBackup,
 * Block 4): the backups with their copies, each target's state, how
 * many keys are set and the daily time (GET /api/v1/server-backups, for
 * a holder of backup.retry, which backup.server includes); the targets
 * with their fields and their retention, the keys, and the kinds of
 * target with the fields of each one's form, read once (GET
 * /api/v1/server-backup-targets, …/server-backup-keys and
 * …/server-backup-targets/kinds), for a holder of backup.server. Beside
 * them what the card keeps beyond its parts: a new key's or target's form
 * shown, the key just made with its secret part, held until it is
 * confirmed, a fold open per backup and per target, and the message
 * lines. A sign-out forgets it all, the secret part first.
 */

import type { ServerBackupKey, ServerBackupKind, ServerBackupList, ServerBackupPolicy, ServerBackupTarget } from "../api";
import type { Outcome } from "../shared/action";
import { forgetForms } from "../shared/forms";
import { live, liveMap } from "../shared/live";

/** What the card shows, as the last reads gave it. */
export const sb = live({
  /** The backups, the targets' states, the keys set and the daily time, null before the list was read. */
  list: null as ServerBackupList | null,
  /** Why the list could not be read, "" where it was. */
  failed: "",
  /** The targets with their fields and retention, read for a holder of backup.server. */
  targets: [] as ServerBackupTarget[],
  /** The targets were read. */
  targetsLoaded: false,
  /** The keys, a pending one with its end, read for a holder of backup.server. */
  keys: [] as ServerBackupKey[],
  /** The keys were read. */
  keysLoaded: false,
  /** The kinds of target this server knows and their fields, read once; null before. */
  kinds: null as ServerBackupKind[] | null,
});

/** A key just made: its uuid, its secret part and the key file, shown until it is confirmed (D4-A). */
export interface Made {
  uuid: string;
  identity: string;
  file: string;
}

/** What the card keeps beyond its parts: the new key's and the new target's forms shown, the key made, an upload under way, and the message lines. */
export const card = live({
  /** The form that adds a key is shown. */
  addingKey: false,
  /** The form that adds a target is shown. */
  addingTarget: false,
  /** The key made, with its secret part, until it is confirmed or gone. */
  made: null as Made | null,
  /** A file is being uploaded. */
  uploading: false,
  /** How Back up now or an upload went. */
  backups: null as Outcome | null,
  /** How a target added or removed went, or why the targets could not be read. */
  targets: null as Outcome | null,
  /** How a key added, confirmed or removed went, or why the keys could not be read. */
  keys: null as Outcome | null,
});

/** Per backup, by its uuid: its fold open, and how its last action went. */
export const backupUI = liveMap(() => ({ open: false, said: null as Outcome | null }));

/** Per target, by its uuid: its panel open, and how its last test went. */
export const targetUI = liveMap(() => ({ open: false, said: null as Outcome | null }));

/** The counts of a target's retention, in the form's order (serverbackup.Policy). */
export const KEEPS = ["last", "daily", "weekly", "monthly", "yearly"] as const;

/** One count of a retention. */
export type KeepKey = (typeof KEEPS)[number];

/** What a target takes, in the form's order (serverbackup.Takes). */
export const TAKES = ["every", "daily", "weekly", "monthly"] as const;

/** DEFAULT_KEEP is the retention of a target added without one, the server's (serverbackup.DefaultPolicy). */
export const DEFAULT_KEEP: ServerBackupPolicy = { daily: 7, weekly: 4, monthly: 6 };

/** targetFormKey is the key of a target's settings form (shared/forms.ts), under the card's "serverbackups". */
export function targetFormKey(uuid: string): string {
  return "serverbackups:" + uuid;
}

/** quiet takes the card's messages back, as Maintenance is shown. */
export function quiet(): void {
  card.backups = null;
  card.targets = null;
  card.keys = null;
  for (const b of sb.list?.backups || []) backupUI.of(b.uuid || "").said = null;
  for (const tg of sb.targets) targetUI.of(tg.uuid || "").said = null;
}

/**
 * forgetServerBackups forgets what the card kept of the user who was
 * signed in (MassimoParts.signedOut): the key made with its secret part
 * first, the lists, the folds, the forms shown with their drafts and the
 * messages; the part is drawn from nothing.
 */
export function forgetServerBackups(): void {
  card.made = null;
  for (const tg of sb.targets) forgetForms(targetFormKey(tg.uuid || ""));
  sb.list = null;
  sb.failed = "";
  sb.targets = [];
  sb.targetsLoaded = false;
  sb.keys = [];
  sb.keysLoaded = false;
  sb.kinds = null;
  card.addingKey = false;
  card.addingTarget = false;
  card.uploading = false;
  card.backups = null;
  card.targets = null;
  card.keys = null;
  backupUI.clear();
  targetUI.clear();
}
