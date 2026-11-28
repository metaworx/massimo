// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/restart.ts
 *
 * The page following the server through a restart, wherever one is
 * asked for: Maintenance's Restart the server and the restore of a
 * server backup, which restarts it (AP ServerBackup, Block 5). The
 * server's start is read from /healthz, which needs no sign-in, before
 * the request and then every RESTART_POLL_MS until it reports another
 * start; the server back, the page reads its data and opens the stream
 * again (serverBack). A sign-out ends the following, as does its stop.
 */

import { session, stillSession } from "./session";

/** RESTART_POLL_MS is how often the page asks /healthz while the server restarts, RESTART_WAIT_MS how long it waits for it to come back. */
export const RESTART_POLL_MS = 2000;
export const RESTART_WAIT_MS = 2 * 60 * 1000;

/** startedAt is the server's start from /healthz, "" when it does not answer; /healthz is always open, so it needs no sign-in. */
export async function startedAt(): Promise<string> {
  try {
    const h = await apiRequest<{ started_at?: string }>("GET", "healthz", undefined, { auth: false });
    return (h && h.started_at) || "";
  } catch {
    return "";
  }
}

/** serverBack reads the data again and opens the stream again, the server back with a new start. */
export function serverBack(): void {
  checkVersion(); // header.js: the start in the settings, and a reload for a new version
  refresh().then((fresh) => {
    if (fresh) connectEvents(); // false: answered after the session ended
  });
}

/**
 * followStart asks /healthz every RESTART_POLL_MS until the server
 * answers with another start than before, then runs back with that
 * start and reads the data again (serverBack); after until (a time in
 * ms) it gives up and runs lost. A sign-out ends it, and so does the
 * function it returns.
 */
export function followStart(before: string, until: number, back: (now: string) => void, lost: () => void): () => void {
  const n = session();
  let timer: ReturnType<typeof setTimeout> | 0 = 0;
  let stopped = false;
  const step = async () => {
    timer = 0;
    const now = await startedAt();
    if (stopped || !stillSession(n)) return;
    if (now && now !== before) {
      back(now);
      serverBack();
      return;
    }
    if (Date.now() > until) {
      lost();
      return;
    }
    timer = setTimeout(step, RESTART_POLL_MS);
  };
  timer = setTimeout(step, RESTART_POLL_MS);
  return () => {
    stopped = true;
    if (timer) clearTimeout(timer);
    timer = 0;
  };
}
