// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/model/cap.ts
 *
 * A capability as the page names it: what it allows, in the page's
 * language, for a role's set, Capabilities for every role and an
 * integration's maximum.
 */

/** capLabel is what a capability allows, in the page's language. */
export function capLabel(c: string): string {
  return t("cap." + c);
}
