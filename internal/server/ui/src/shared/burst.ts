// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/burst.ts
 *
 * One read for a burst of events: a part that follows the server reads
 * again what an event of the stream says has changed, and several events
 * at once make one read in flight and at most one more after it, which
 * reads everything the events meanwhile asked for. A read asked for in
 * one session is not made in the next (shared/session.ts).
 */

import { session, stillSession } from "./session";

/**
 * burstReader makes the reader of a part: ask(what) asks for the reads
 * named, read(what) makes them, once at a time; what is asked for while a
 * read runs is read once after it, together, where shown() still says
 * the part's page is shown as that read starts.
 */
export function burstReader(read: (what: Set<string>) => Promise<unknown>, shown: () => boolean): (...what: string[]) => void {
  let running = false;
  let next: { n: number; what: Set<string> } | null = null;
  const go = async (what: Set<string>) => {
    if (!shown()) return; // the page left meanwhile: its showing reads everything anyway
    running = true;
    try {
      await read(what);
    } catch {
      // the read says its failure where it shows one; the next event reads again
    } finally {
      running = false;
    }
    const after = next;
    next = null;
    if (after && stillSession(after.n)) go(after.what);
  };
  return (...what) => {
    const n = session();
    if (running) {
      if (!next || !stillSession(next.n)) next = { n, what: new Set() };
      for (const w of what) next.what.add(w);
      return;
    }
    go(new Set(what));
  };
}
