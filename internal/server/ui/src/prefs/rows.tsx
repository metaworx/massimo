// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/prefs/rows.tsx
 *
 * A row of My settings (.dev-row): its setting's field, and in a column
 * of its own at the right the switch that keeps the setting on this
 * device alone (renderDeviceSwitches of prefs.js), named for its row,
 * shown where the settings follow the user (needs-sync). My settings'
 * rows and the Appearance card's are drawn with it.
 */

import type { ComponentChildren } from "preact";
import { Switch } from "../shared/field";
import { load } from "./store";

/** DeviceOnly is a setting's switch that keeps its value on this device alone, its tooltip and name the setting's. */
function DeviceOnly({ setting }: { setting: string }) {
  load.useLoad(); // whether the setting is kept here is the plain code's, drawn again by the counter
  const label = t("settings.thisDeviceFor", { setting: t(SETTING_LABELS[setting]) });
  return (
    <span class="dev-cell needs-sync">
      <Switch label="" aria={label} title={label} row={false} data={{ "device-only": setting }} checked={deviceOnly(setting)} onChange={(on) => {
        setDeviceOnly(setting, on);
        load.bump();
      }} />
    </span>
  );
}

/** DevRow is a row with its field as children and the switch of setting beside it, or an empty cell without one; last ends the column's rounded foot. */
export function DevRow({ setting, last, children }: { setting?: string; last?: boolean; children: ComponentChildren }) {
  return (
    <div class={last ? "dev-row dev-last" : "dev-row"}>
      {children}
      {setting ? <DeviceOnly setting={setting} /> : <span class="dev-cell needs-sync" />}
    </div>
  );
}
