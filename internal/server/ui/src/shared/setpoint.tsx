// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/setpoint.tsx
 *
 * A setpoint as the eQ-3 devices' displays show it, for every text that
 * shows one (AP RoomMajority, D6-A, D14-A): "OFF" for 4.5 °C, else the
 * temperature as fmtTemp gives it, no "ON" for 30.5 °C; and the preset
 * mark, the device's own symbol for a setpoint at a thermostat's eco or
 * comfort temperature, a crescent moon, a sun with rays, the two side by
 * side where they are the same (D10-A, D11-A, D12-D). The number fields
 * keep 4.5 (a holiday's, the week programme's, the configuration's). The
 * plain scripts have the same formatter, fmtSetpoint of app.js.
 */

import { Icon } from "./icon";

/** TEMP_OFF is the setpoint a device shows as OFF, its valve shut (cube.TempOff). */
export const TEMP_OFF = 4.5;

/**
 * fmtSetpoint is a setpoint as a device's display shows it: OFF for 4.5
 * °C, without a unit, else as fmtTemp gives it, unit the unit after the
 * number (" °C" unless given), "–" for none.
 */
export function fmtSetpoint(v: number | undefined | null, unit?: string): string {
  return v === TEMP_OFF ? t("temp.off") : fmtTemp(v, unit);
}

/** A setpoint's preset: a thermostat's eco temperature, its comfort temperature, or both where the two are the same. */
export type Preset = "eco" | "comfort" | "eco-comfort";

/**
 * presetOf is the preset a setpoint is of a thermostat with these eco and
 * comfort temperatures (a device's entry, eco_temp and comfort_temp):
 * eco at its eco temperature, comfort at its comfort temperature, both
 * where the two are the same and the setpoint is both; null for another
 * setpoint, for none, and for a thermostat whose temperatures are not
 * known.
 */
export function presetOf(setpoint: number | undefined | null, eco: number | undefined, comfort: number | undefined): Preset | null {
  if (setpoint === undefined || setpoint === null) return null;
  const atEco = eco !== undefined && setpoint === eco;
  const atComfort = comfort !== undefined && setpoint === comfort;
  if (atEco && atComfort) return "eco-comfort";
  if (atEco) return "eco";
  if (atComfort) return "comfort";
  return null;
}

/** presetMarkLabel names a preset, the mark's accessible name and its tooltip, in the configuration's words. */
export function presetMarkLabel(p: Preset): string {
  if (p === "eco") return t("preset.eco");
  if (p === "comfort") return t("preset.comfort");
  return t("preset.ecoComfort");
}

/** The symbol of each preset in the page's sprite (index.html). */
const PRESET_SYMBOL: Record<Preset, string> = { eco: "icon-eco", comfort: "icon-comfort", "eco-comfort": "icon-eco-comfort" };

/**
 * PresetMark is a preset's mark, an image named by its label for a screen
 * reader and in its tooltip; cls is its icon's class, which sizes and
 * colours it where it stands (a tile's ti, a device row's di).
 */
export function PresetMark({ preset, cls }: { preset: Preset; cls: string }) {
  const label = presetMarkLabel(preset);
  return (
    <span class={"preset-mark preset-" + preset} role="img" title={label} aria-label={label}>
      <Icon symbol={PRESET_SYMBOL[preset]} cls={cls} />
    </span>
  );
}
