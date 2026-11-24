// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/model/event.ts
 *
 * A server's event of the stream as the parts that follow the server
 * read it (docs/api-v1.md, "Events"): its kind, its time, its level and
 * its summary; and which settings a change event says were set.
 */

/** A server's event of the stream, as app.js hands it on (the event's JSON). */
export interface ServerEvent {
  kind: string;
  time?: string;
  /** How much it matters: debug, info, warn or error, none being info. */
  level?: string;
  summary?: Record<string, unknown>;
}

/** settingsChanged is the settings a change event says were set (PUT /api/v1/settings, the action settings.update), none for another change. */
export function settingsChanged(ev: ServerEvent): string[] {
  const s = ev.summary || {};
  if (ev.kind !== "change" || s.action !== "settings.update" || !s.set || typeof s.set !== "object") return [];
  return Object.keys(s.set as object);
}
