// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/model/cap.ts
 *
 * A capability as the page names it: what it allows, in the page's
 * language, for a role's set, Capabilities for every role and an
 * integration's maximum; and what one includes, which a role's set and
 * an integration's maximum show on with it.
 */

/** capLabel is what a capability allows, in the page's language. */
export function capLabel(c: string): string {
  return t("cap." + c);
}

/**
 * CAP_INCLUDES is what a capability includes (includes in
 * internal/server/auth.go, AP StartConfiguration D37 and D40-A): its holder
 * passes every check of those it names, which no stored set lists.
 */
export const CAP_INCLUDES: Record<string, string[]> = { update: ["restart"], reboot: ["restart"], "backup.server": ["backup.retry"] };

/**
 * includedHint is why a capability's switch shows on and cannot be changed
 * while chosen has one that includes it (CAP_INCLUDES), for Choices' held;
 * undefined while none does.
 */
export function includedHint(c: string, chosen: string[]): string | undefined {
  return chosen.some((h) => (CAP_INCLUDES[h] || []).includes(c)) ? t("cap.included") : undefined;
}
