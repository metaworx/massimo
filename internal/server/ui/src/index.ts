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
import "./chrome";
import type { ChromePart } from "./chrome";
import { signedOut as cubesSignedOut } from "./cubes";
import type { CubesPart } from "./cubes";
import "./header";
import type { HeaderPart } from "./header";
import "./leaving";
import type { LeavingPart } from "./leaving";
import "./rooms";
import type { RoomsPart } from "./rooms";
import { signedOut as integrationsSignedOut } from "./integrations";
import { forgetForms } from "./shared/forms";
import { languageChanged } from "./shared/lang";
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
  /**
   * A sign-out, or another user's sign-in after a 401 (login.js
   * forgetSession): every form's drafts and marks forgotten
   * (shared/forms.ts), every box of repairs (shared/repair.tsx), and
   * every Cube's pairing and rooms (cubes/), and System settings' lists
   * with what is typed in their new items' forms (admin/) and the
   * integrations with the readings of an approval (integrations/).
   */
  signedOut(): void;
  /** Where the page's chrome says that settings are unsaved: a section's pill, ☰'s mark joined with the log's, a menu item's disk (chrome/). */
  chrome: ChromePart;
  /** System settings → Cubes and Cube settings (cubes/). */
  cubes: CubesPart;
  /** The header: the Cube choice, the problems, the age, the server's sign, the certificate's line, the home notice (header/). */
  header: HeaderPart;
  /** Leaving the page, and signing out, while a settings form has changes not saved: the dialog that lists them (leaving/). */
  leaving: LeavingPart;
  /** The rooms: a tile per room with its device list (rooms/). */
  rooms: RoomsPart;
  /** System settings → Sign-in providers (admin/). */
  providers: ProvidersPart;
  /** System settings → Roles and Capabilities for every role (admin/). */
  roles: RolesPart;
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
register("signedOut", () => {
  newSession(); // first: an answer still on its way for the user who was is dropped
  forgetForms("");
  forgetRepairs();
  cubesSignedOut();
  adminSignedOut();
  integrationsSignedOut();
});
window.MassimoParts = parts as unknown as MassimoParts;
