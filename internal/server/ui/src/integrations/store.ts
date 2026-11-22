// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/integrations/store.ts
 *
 * What System settings → Integrations is drawn from, read and sent, line
 * for line as integrations.js did: the requests and the integrations
 * (GET /api/v1/integrations), the users of those whose panel is open
 * (…/{uuid}), the installations the last look at the LAN saw
 * (…/found), the Massimo users an integration's user may act as (GET
 * /api/v1/users), whether integrations may ask (the setting
 * integrations.requests); a request approved or refused, an integration
 * changed, accepting a new installation, revoked, a user linked; and the
 * reading of an approved integration again until its key is collected.
 * A change reads the integrations again and says how it went under the
 * section; an integration gone from a load that went through forgets its
 * forms and its panel's state; an answer after a sign-out is dropped
 * (shared/session.ts).
 */

import { signal } from "@preact/signals";
import type { Integration, User } from "../api";
import { actionRunner, type Outcome } from "../shared/action";
import { forgetForms } from "../shared/forms";
import { live, liveMap } from "../shared/live";
import { session, stillSession } from "../shared/session";
import { can, canManageUsersOf } from "../shared/me";

/** An installation seen on the LAN (GET /api/v1/integrations/found). */
export interface FoundIntegration {
  kind: string;
  name: string;
  instance_id: string;
  location?: string;
  url?: string;
  version?: string;
  address: string;
  /** The uuid of the integration or request of this installation, if any. */
  integration?: string;
}

/** Why the server does not look on the LAN: the found answer's announce. */
interface NotLooking {
  value?: string;
  in_effect?: string;
  fixed_by?: string;
  why?: string;
}

/** What the section shows, as the last loads that went through gave it. */
export const data = live({
  /** The requests and the integrations. */
  list: [] as Integration[],
  /** A load of them went through. */
  loaded: false,
  /** Per uuid, an open panel's integration with its users. */
  details: {} as Record<string, Integration>,
  /** The installations the last look at the LAN saw. */
  found: [] as FoundIntegration[],
  /** The server looks at the LAN. */
  looks: false,
  /** Why it does not. */
  announce: null as NotLooking | null,
  /** The Massimo users an integration's user may act as, null where they may not be read. */
  users: null as User[] | null,
});

/** Whether integrations may ask to be registered (the setting integrations.requests), null before it is read. */
export const requestsOn = signal<boolean | null>(null);

/** Per uuid, its panel open; a request's is open until it is closed. */
export const panelUI = liveMap(() => ({ open: false }));
/** Per request's uuid, its panel closed by the user. */
export const requestUI = liveMap(() => ({ closed: false }));

/** The outcome the section's message line shows. */
export const said = signal<Outcome | null>(null);

/** The timers reading an approved integration again until its key is collected, by uuid. */
const watch = new Map<string, ReturnType<typeof setTimeout>>();

/** The list's own element, which the part hands over (shown): the readings stop while it is not visible, off the page or in a folded card. */
let listEl: HTMLElement | null = null;

/** shown takes the list's element, as the part draws it (a ref), null as it goes. */
export function shown(el: HTMLElement | null): void {
  listEl = el;
}

/** REQUEST_LIFE is how long an approval waits for its key (RequestLife in internal/store), from which the reading's pace is counted. */
const REQUEST_LIFE = 10 * 60 * 1000;

/** INTEGRATION_CEILING is the role integration's set, what an integration may have at most (integrationCeiling in internal/server/auth.go). */
export const INTEGRATION_CEILING = ["read", "preferences", "setpoint", "lock", "configure", "rename", "structure", "pair", "discover", "backup", "restore",
  "repair", "guests", "cubes", "cubes.reboot", "settings", "update", "restart", "reboot", "audit"];

/** formKey is the key of one of an integration's settings forms (shared/forms.ts): its name or its maximum. */
export function formKey(uuid: string, form?: "name" | "max"): string {
  return "integrations:" + uuid + (form ? ":" + form : "");
}

/** integrationPath is an integration's path in the API, rest after it. */
export function integrationPath(uuid: string, rest = ""): string {
  return "api/v1/integrations/" + encodeURIComponent(uuid) + rest;
}

/**
 * keyState is an approved integration's key: "waiting" while Home
 * Assistant may still collect it, "expired" once it may not (active, no
 * key collected, request_expires passed), "" once collected or for any
 * other state.
 */
export function keyState(i: Integration): "" | "waiting" | "expired" {
  if (i.state !== "active" || i.key_issued_at) return "";
  const until = Date.parse(i.request_expires || "");
  if (isNaN(until)) return "";
  return Date.now() < until ? "waiting" : "expired";
}

/** loadDetails reads an integration with its users, for its open panel. */
export async function loadDetails(uuid: string): Promise<void> {
  const n = session();
  const i = await api<Integration>("GET", integrationPath(uuid));
  if (stillSession(n)) data.details = { ...data.details, [uuid]: i };
}

/**
 * loadIntegrations reads the requests and the integrations, and the users
 * of those whose panel is open; an integration the list no longer gives
 * forgets its forms. A load that fails throws before it and leaves the
 * list as it was.
 */
export async function loadIntegrations(): Promise<void> {
  if (!can("integrations")) return;
  const n = session();
  const list = (await api<Integration[]>("GET", "api/v1/integrations")) || [];
  if (!stillSession(n)) return; // signed out meanwhile: nothing of it is the next user's
  // a key collected while no reading followed it (another page shown, the card folded) is said all the same
  for (const i of list) {
    const was = data.list.find((x) => x.uuid === i.uuid);
    if (was && keyState(was) === "waiting" && !keyState(i) && i.key_issued_at) msg(t("integrations.collected", { name: i.name }), true);
  }
  const now = new Set(list.map((i) => i.uuid));
  for (const i of data.list) {
    if (now.has(i.uuid)) continue;
    forgetForms(formKey(i.uuid));
    panelUI.forget(i.uuid);
    requestUI.forget(i.uuid);
  }
  data.list = list;
  data.loaded = true;
  for (const i of list) if (panelUI.of(i.uuid).open) await loadDetails(i.uuid);
  // every approval whose key waits is read again, as the plain list's every drawing started it
  for (const i of list) if (keyState(i) === "waiting") watchApproval(i.uuid);
}

/** loadSeen reads what the last look at the LAN saw; refresh looks now. */
export async function loadSeen(refresh: boolean): Promise<void> {
  const n = session();
  const f = (await api<{ found: FoundIntegration[]; looks: boolean; announce?: NotLooking }>("GET", "api/v1/integrations/found" + (refresh ? "?refresh=1" : ""))) || {};
  if (!stillSession(n)) return;
  data.found = f.found || [];
  data.looks = !!f.looks;
  data.announce = f.announce || null;
}

/** loadLinkUsers reads the Massimo users an integration's user may be linked to: none of an integration; null where they may not be read. */
async function loadLinkUsers(): Promise<void> {
  if (!canManageUsersOf()) {
    data.users = null;
    return;
  }
  const n = session();
  let users: User[] | null;
  try {
    users = ((await api<User[]>("GET", "api/v1/users")) || []).filter((u) => !u.integration);
  } catch {
    users = null;
  }
  if (stillSession(n)) data.users = users;
}

/** refresh reads everything the section shows and draws it. */
export async function refresh(): Promise<void> {
  await loadIntegrations();
  await loadLinkUsers();
  await loadSeen(false);
}

/** loadRequestsSwitch reads whether integrations may ask, for a holder of settings, who alone sees the switch. */
export async function loadRequestsSwitch(): Promise<void> {
  if (!can("settings")) return;
  const n = session();
  try {
    const all = (await api<Record<string, { value?: string }>>("GET", "api/v1/settings")) || {};
    if (!stillSession(n)) return;
    const v = all["integrations.requests"];
    requestsOn.value = !v || v.value !== "false";
  } catch {
    // the switch stays as it was
  }
}

/** The section's runner: its message under the section, the integrations read again after a change. */
const runner = actionRunner({
  set: (res) => {
    said.value = res;
  },
  reload: loadIntegrations,
  bound: true,
});

/** msg shows text under the section, ok green, else red; no text hides it. */
export const msg = runner.say;

/** msgIn shows text as msg does where n, the session's number taken before an await, is still the session's. */
export function msgIn(n: number, text: string, ok: boolean): void {
  if (stillSession(n)) msg(text, ok);
}

/** run sends a change of the integration name, says ok, or that it failed and why. */
export function run(name: string, fn: () => Promise<unknown>, ok: string): Promise<boolean> {
  return runner.run(fn, ok, (error) => t("integrations.failed", { name, error }));
}

/** setRequests lets integrations ask, or not, saved at once; refused, the switch stays as stored. */
export async function setRequests(on: boolean): Promise<void> {
  const n = session();
  try {
    await api("PUT", "api/v1/settings", { "integrations.requests": on });
    if (!stillSession(n)) return;
    requestsOn.value = on;
    msg(t(on ? "integrations.requestsOn" : "integrations.requestsOff"), true);
  } catch (e) {
    if (stillSession(n)) msg(t("integrations.failed", { name: "", error: (e as Error).message }), false);
  }
}

/**
 * watchApproval reads an approved integration again until its key is
 * collected or the approval expires: every 3 s in its first minute, then
 * every 15 s; it stops while the list is not visible (off the page, in a
 * folded card), and every load of the list that went through starts it
 * again for each approval whose key waits; a sign-out stops it.
 */
export function watchApproval(uuid: string): void {
  if (watch.has(uuid)) return;
  const n = session(); // the session it reads for
  const schedule = (i: Integration) => {
    const expires = Date.parse(i.request_expires || "");
    const since = Date.now() - (expires - REQUEST_LIFE);
    const left = expires - Date.now();
    const pace = since < 60 * 1000 ? 3000 : 15000;
    watch.set(uuid, setTimeout(tick, Math.max(500, Math.min(pace, left + 500))));
  };
  const tick = async () => {
    watch.delete(uuid);
    if (!stillSession(n) || !listEl?.offsetParent) return;
    let i: Integration;
    try {
      i = await api<Integration>("GET", integrationPath(uuid));
    } catch {
      return;
    }
    if (!stillSession(n)) return; // signed out meanwhile: no answer kept, no next reading
    const at = data.list.findIndex((x) => x.uuid === uuid);
    const was = at >= 0 ? keyState(data.list[at]) : "";
    if (at >= 0) {
      const next = { ...data.list[at], ...i };
      delete next.users; // the list's entries carry none
      data.list = data.list.map((x, k) => (k === at ? next : x));
    }
    if (panelUI.of(uuid).open) data.details = { ...data.details, [uuid]: i };
    const now = keyState(i);
    if (now !== was) {
      if (was === "waiting" && !now) msg(t("integrations.collected", { name: i.name }), true);
      return;
    }
    if (now === "waiting") schedule(i);
  };
  const i = data.list.find((x) => x.uuid === uuid);
  if (i && keyState(i) === "waiting") schedule(i);
}

/**
 * forgetIntegrations forgets what the section kept of the user who was
 * signed in (MassimoParts.signedOut): the lists, the panels, the
 * readings of approvals with their timers, the message.
 */
export function forgetIntegrations(): void {
  for (const id of watch.values()) clearTimeout(id);
  watch.clear();
  data.list = [];
  data.loaded = false;
  data.details = {};
  data.found = [];
  data.looks = false;
  data.announce = null;
  data.users = null;
  requestsOn.value = null;
  panelUI.clear();
  requestUI.clear();
  said.value = null;
}
