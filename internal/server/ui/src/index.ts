// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/index.ts
 *
 * The entry point of the page's parts (AP UiParts), which make ui bundles
 * into ui/parts.js, loaded after the plain scripts. The plain code reaches
 * a part through window.MassimoParts.<region>: each region that becomes a
 * part registers its functions there (shared/region.ts register) and is
 * imported here (docs/ui.md, "How the page is built"). What every part
 * shares is in shared/; the plain globals a part calls are declared in
 * plain/, a file per script.
 */

import { signedOut as adminSignedOut } from "./admin";
import type { ProvidersPart, RolesPart, UsersPart } from "./admin";
import { signedOut as backupsSignedOut } from "./backups";
import type { BackupsPart } from "./backups";
import "./chrome";
import type { ChromePart } from "./chrome";
import { signedOut as cubesSignedOut } from "./cubes";
import type { CubesPart } from "./cubes";
import "./header";
import type { HeaderPart } from "./header";
import "./leaving";
import type { LeavingPart } from "./leaving";
import { signedOut as notifySignedOut } from "./notify";
import type { NotifyPart } from "./notify";
import { signedOut as prefsSignedOut } from "./prefs";
import type { PrefsPart } from "./prefs";
import { signedOut as profileSignedOut } from "./profile";
import type { ProfilePart } from "./profile";
import "./rooms";
import type { RoomsPart } from "./rooms";
import { signedOut as integrationsSignedOut } from "./integrations";
import { serverEvent as softwareEvent, signedOut as softwareSignedOut } from "./software";
import { serverEvent as maintenanceEvent, signedOut as maintenanceSignedOut } from "./maintenance";
import { serverEvent as sysinfoEvent, signedOut as sysinfoSignedOut } from "./sysinfo";
import type { Me } from "./api";
import type { ServerEvent } from "./model/event";
import type { SoftwarePart } from "./software";
import { forgetForms } from "./shared/forms";
import { languageChanged } from "./shared/lang";
import { meChanged } from "./shared/me";
import { iconsChanged } from "./shared/look";
import { parts, register } from "./shared/region";
import { forgetRepairs } from "./shared/repair";
import { newSession } from "./shared/session";

/** The parts the plain code calls, by region. */
export interface MassimoParts {
  /** The page's language changed (app.js applyLanguage): every part draws its texts again. */
  language(): void;
  /** The icons chosen for the window and the radio changed (theme.js redrawSigns): every sign draws them again. */
  icons(): void;
  /** Who the page is signed in as, at every read of it (login.js applyCaps): a user's sessions say "this device" by its token, and My profile is drawn for it (shared/me.ts). */
  me(m: Me | null): void;
  /**
   * A sign-out, or another user's sign-in after a 401 (login.js
   * forgetSession): every form's drafts and marks forgotten
   * (shared/forms.ts), every box of repairs (shared/repair.tsx), and
   * every Cube's pairing and rooms (cubes/), and System settings' lists
   * with what is typed in their new items' forms (admin/) and the
   * integrations with the readings of an approval (integrations/), and
   * every Cube's backups with a restore shown (backups/), and Software
   * update's reading while the helper works, its question and the
   * channel's setting (software/), and Maintenance's following of a
   * restart or a reboot, its questions, the delay typed and its settings
   * (maintenance/), and System info with its readings and its clock
   * (sysinfo/), and who was in (shared/me.ts) with My profile's user, its
   * link shown and its messages (profile/), the passwords typed there
   * going with its part, and the scheme editor with its draft (prefs/),
   * and the notifications' rules, devices, the name typed and a save
   * still waiting (notify/).
   */
  signedOut(): void;
  /**
   * A server's event of the stream (update.js serverEvent, from app.js
   * connectEvents): Software update, Maintenance and System info read
   * again what it says has changed while their page is shown.
   */
  serverEvent(ev: ServerEvent): void;
  /** Backup and restore: Cube settings' box and System settings' fold per Cube (backups/). */
  backups: BackupsPart;
  /** Where the page's chrome says that settings are unsaved: a section's pill, ☰'s mark joined with the log's, a menu item's disk (chrome/). */
  chrome: ChromePart;
  /** System settings → Cubes and Cube settings (cubes/). */
  cubes: CubesPart;
  /** The header: the Cube choice, the problems, the age, the server's sign, the certificate's line, the home notice (header/). */
  header: HeaderPart;
  /** Leaving the page, and signing out, while a settings form has changes not saved: the dialog that lists them (leaving/). */
  leaving: LeavingPart;
  /** 👤 → Notifications: the rules, this browser and the devices (notify/). */
  notify: NotifyPart;
  /** My settings: their rows and the Appearance card with the scheme editor, drawn again where the settings were applied (prefs/). */
  prefs: PrefsPart;
  /** My profile: the API key its address marks (profile/). */
  profile: ProfilePart;
  /** The rooms: a tile per room with its device list (rooms/). */
  rooms: RoomsPart;
  /** System settings → Sign-in providers (admin/). */
  providers: ProvidersPart;
  /** System settings → Roles and Capabilities for every role (admin/). */
  roles: RolesPart;
  /** Maintenance → Software update (software/). */
  software: SoftwarePart;
  /** System settings → Users: the password sign-in and a panel per user (admin/). */
  users: UsersPart;
}

declare global {
  interface Window {
    /** The page's parts, registered when ui/parts.js runs. */
    MassimoParts: MassimoParts;
  }
}

register("language", languageChanged);
register("icons", iconsChanged);
register("me", meChanged);
register("serverEvent", (ev: ServerEvent) => {
  softwareEvent(ev);
  maintenanceEvent(ev);
  sysinfoEvent(ev);
});
register("signedOut", () => {
  newSession(); // first: an answer still on its way for the user who was is dropped
  forgetForms("");
  forgetRepairs();
  cubesSignedOut();
  adminSignedOut();
  integrationsSignedOut();
  backupsSignedOut();
  softwareSignedOut();
  maintenanceSignedOut();
  sysinfoSignedOut();
  meChanged(null); // My profile's parts go, with what is typed in them
  profileSignedOut();
  prefsSignedOut();
  notifySignedOut();
});
window.MassimoParts = parts as unknown as MassimoParts;
