// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/look.ts
 *
 * The icons the person chose for the window and the radio (My settings →
 * Appearance) as a signal: theme.js raises it through MassimoParts.icons
 * when the choice changes (redrawSigns), and every sign reads it, so that
 * a part draws the new drawing without its region being told.
 */

import { signal } from "@preact/signals";
import { useLang } from "./lang";

/** Counts the changes of the icons chosen. */
const icons = signal(0);

/** iconsChanged takes a change of the icons chosen (theme.js redrawSigns). */
export function iconsChanged(): void {
  icons.value++;
}

/** useIcons reads the icons chosen in a part's drawing, so that the part is drawn again when they change. */
export function useIcons(): void {
  void icons.value;
}

/**
 * useLook reads the language and the icons chosen in a part's drawing,
 * the two things every part depends on beside its data (rule 2 of AP
 * UiParts): the part is drawn again when either changes.
 */
export function useLook(): void {
  useLang();
  useIcons();
}
