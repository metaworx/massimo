// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/mode.tsx
 *
 * The choice of the mode, light, dark or as the system, as one drawing
 * for its two places: My settings' Appearance card and the sign-in
 * card's ☰ (AP UiParts, Block 14). It shows the mode in effect,
 * state.settings.theme of app.js, drawn again where the settings were
 * applied (shared/settings.ts), and a choice goes to chooseMode of
 * theme.js, which applies and keeps it.
 */

import { Select } from "./field";
import { useLang } from "./lang";
import { applied } from "./settings";

/** ModeSelect is the mode's row: id its select's, label the row's, help the setting's help where the place has one, wide the select on a line of its own. */
export function ModeSelect({ id, label, help, wide }: { id: string; label: string; help?: string; wide?: boolean }) {
  useLang();
  applied.useLoad();
  return (
    <Select label={label} id={id} help={help} wide={wide} value={state.settings.theme} onChange={chooseMode}
      options={[["system", t("appearance.modeSystem")], ["light", t("appearance.modeLight")], ["dark", t("appearance.modeDark")]]} />
  );
}
