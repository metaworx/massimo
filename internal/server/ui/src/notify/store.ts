// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/notify/store.ts
 *
 * What the notifications' card is drawn from, as notify.js kept it: the
 * kinds in the page's order with their defaults (the server's), the
 * user's rules once read, its subscriptions, this browser's endpoint,
 * the name of this device typed, and the line under the lists with whose
 * outcome it shows. A rule changed waits a second for the next before it
 * is sent (save.ts).
 */

import type { PushSubscription } from "../api";
import type { Outcome } from "../shared/action";
import { live } from "../shared/live";

/** A kind's rule: on or off, and its delay (after_min) or threshold (below_c) where it has one. */
export interface Rule {
  on: boolean;
  after_min?: number;
  below_c?: number;
}

/** The rules by kind. */
export type Rules = Record<string, Rule>;

/** The kinds in the page's order, each with its default as the server's (internal/notify, Defaults). */
export const KINDS: [string, Rule][] = [
  ["window_open", { on: true, after_min: 30 }],
  ["frost", { on: true, below_c: 8 }],
  ["battery_low", { on: true }],
  ["battery_weak", { on: true }],
  ["device_error", { on: true }],
  ["cube_offline", { on: true, after_min: 10 }],
  ["cube_online", { on: true }],
  ["device_stale", { on: false }],
  ["paired", { on: false }],
  ["cube_found", { on: false }],
  ["update", { on: false }],
];

/** The bounds of a delay and of the threshold, the server's. */
export const AFTER: [number, number] = [1, 1440];
export const BELOW: [number, number] = [-10, 20];

/** defaults are the rules of a user who has set none. */
export function defaults(): Rules {
  const rules: Rules = {};
  for (const [kind, def] of KINDS) rules[kind] = { ...def };
  return rules;
}

/** rulesOf reads a notify preference over the defaults, as the server does: a kind or a value it leaves out keeps the default. */
export function rulesOf(doc: unknown): Rules {
  const rules = defaults();
  if (!doc || typeof doc !== "object") return rules;
  for (const [kind] of KINDS) {
    const r = (doc as Record<string, unknown>)[kind] as Record<string, unknown> | undefined;
    if (!r || typeof r !== "object") continue;
    if (typeof r.on === "boolean") rules[kind].on = r.on;
    if ("after_min" in rules[kind] && Number.isInteger(r.after_min)) rules[kind].after_min = r.after_min as number;
    if ("below_c" in rules[kind] && typeof r.below_c === "number") rules[kind].below_c = r.below_c;
  }
  return rules;
}

/**
 * The user's rules once read (null before: the defaults are shown, off
 * until this session's first read has answered, so that no change sends
 * them over the rules the user has), loaded once it has, ver
 * counting their changes made here; its subscriptions
 * (GET /api/v1/me/push); this browser's subscription's endpoint, "" for
 * none; the name of this device typed for turning it on.
 */
export const notify = live({ rules: null as Rules | null, loaded: false, ver: 0, subs: [] as PushSubscription[], endpoint: "", label: "" });

/** The line under the lists: whose outcome it shows ("load", "save", "device"), and the outcome. */
export const said = live({ from: "", res: null as Outcome | null });

/**
 * say says on the line how something went, from naming whose outcome it
 * is: no text clears the line only where it shows from's own outcome, so
 * that the read after turning this browser on keeps what that said; from
 * "" clears whatever it shows.
 */
export function say(from: string, text: string, ok?: boolean): void {
  if (!text && from && said.from !== from) return;
  said.from = text ? from : "";
  said.res = text ? { text, ok: !!ok } : null;
}

/** forgetNotify forgets what the card kept of the user who was signed in (MassimoParts.signedOut): the rules, the devices, the name typed, the line. */
export function forgetNotify(): void {
  notify.rules = null;
  notify.loaded = false;
  notify.subs = [];
  notify.endpoint = "";
  notify.label = "";
  notify.ver++;
  said.from = "";
  said.res = null;
}
