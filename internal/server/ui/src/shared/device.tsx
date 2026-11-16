// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/device.tsx
 *
 * A device's serial number and radio address as a part (deviceIds of
 * components.js): "S/N MEQ0000103 · RF 0a0103", the radio address in
 * expert mode only, or always where it is what the user types.
 */

/** DeviceIds is a device's serial (it may have none) and radio address; always shows the address outside expert mode too. */
export function DeviceIds({ serial, rf, always }: { serial?: string; rf?: string; always?: boolean }) {
  return (
    <span class="device-ids mono">
      {serial && <span>{t("device.sn", { serial })}</span>}
      {rf && <span class={always ? undefined : "expert-only"}>{(serial ? " · " : "") + t("device.rf", { rf })}</span>}
    </span>
  );
}
