// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/live.ts
 *
 * Live state: plain objects whose properties are signals underneath, so
 * that the page's plain code reads and writes them as before and a part
 * that read one is drawn again when it changes; and a map of them by key,
 * for what a region keeps per place (a panel open, by its item's key)
 * across its part's being drawn in another place.
 */

import { signal } from "@preact/signals";

/**
 * live makes an object with the properties of init, each a signal
 * underneath: read and written as a property, read by a part as a
 * signal.
 */
export function live<T extends object>(init: T): T {
  const out = {} as T;
  for (const k of Object.keys(init) as (keyof T)[]) {
    const s = signal(init[k]);
    Object.defineProperty(out, k, { enumerable: true, get: () => s.value, set: (v) => { s.value = v; } });
  }
  return out;
}

/**
 * liveMap keeps a live object per key, made by make the first time the
 * key is asked for; forget forgets one (what it was kept for is gone),
 * clear forgets them all (a sign-out, a change of whose list it is).
 */
export function liveMap<T extends object>(make: () => T): { of(key: string): T; forget(key: string): void; clear(): void } {
  const all = new Map<string, T>();
  return {
    of(key) {
      let v = all.get(key);
      if (!v) all.set(key, (v = live(make())));
      return v;
    },
    forget(key) {
      all.delete(key);
    },
    clear() {
      all.clear();
    },
  };
}
