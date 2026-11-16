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

import "./cubes";
import type { CubesPart } from "./cubes";
import "./header";
import type { HeaderPart } from "./header";
import "./rooms";
import type { RoomsPart } from "./rooms";
import { languageChanged } from "./shared/lang";
import { iconsChanged } from "./shared/look";
import { parts, register } from "./shared/region";

/** The parts the plain code calls, by region. */
export interface MassimoParts {
  /** The page's language changed (app.js applyLanguage): every part draws its texts again. */
  language(): void;
  /** The icons chosen for the window and the radio changed (theme.js redrawSigns): every sign draws them again. */
  icons(): void;
  /** System settings → Cubes and Cube settings (cubes/). */
  cubes: CubesPart;
  /** The header: the Cube choice, the problems, the age, the server's sign, the certificate's line, the home notice (header/). */
  header: HeaderPart;
  /** The rooms: a tile per room with its device list (rooms/). */
  rooms: RoomsPart;
}

declare global {
  interface Window {
    /** The page's parts, registered when ui/parts.js runs. */
    MassimoParts: MassimoParts;
  }
}

register("language", languageChanged);
register("icons", iconsChanged);
window.MassimoParts = parts as unknown as MassimoParts;
