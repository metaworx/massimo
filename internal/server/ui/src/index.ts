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

import { minuteTick } from "./shared/data"; // first: the page's data are signals before any part draws from them
import { signedOut as adminSignedOut } from "./admin";
import type { UsersPart } from "./admin";
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
import { serverEvent as logEvent, signedOut as logSignedOut } from "./log";
import type { LogPart } from "./log";
import { signedOut as notifySignedOut } from "./notify";
import type { NotifyPart } from "./notify";
import { signedOut as prefsSignedOut } from "./prefs";
import { signedOut as profileSignedOut } from "./profile";
import "./rooms";
import type { RoomsPart } from "./rooms";
import { signedOut as integrationsSignedOut } from "./integrations";
import { signedOut as signinSignedOut } from "./signin";
import type { SignInPart } from "./signin";
import { serverEvent as softwareEvent, signedOut as softwareSignedOut } from "./software";
import { serverEvent as maintenanceEvent, signedOut as maintenanceSignedOut } from "./maintenance";
import { serverEvent as sysinfoEvent, signedOut as sysinfoSignedOut } from "./sysinfo";
import type { ServerEvent } from "./model/event";
import type { SoftwarePart } from "./software";
import { forgetForms } from "./shared/forms";
import { languageChanged } from "./shared/lang";
import { pagesPart } from "./shared/pages";
import { iconsChanged } from "./shared/look";
import { parts, register } from "./shared/region";
import { forgetRepairs } from "./shared/repair";
import { hostWord, passwordProblem } from "./shared/password";
import { newSession, session, stillSession } from "./shared/session";

/** The parts the plain code calls, by region. */
export interface MassimoParts {
  /** The page's language changed (app.js applyLanguage): every part draws its texts again. */
  language(): void;
  /** The icons chosen for the window and the radio changed (theme.js redrawSigns): every sign draws them again. */
  icons(): void;
  /** The page's minute, at once (shared/data.ts minuteNow): the ages and the log's time range follow it; the UI tests' handle of the clock. */
  minute(): void;
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
   * still waiting (notify/), and the log's lines, the filter typed, its
   * users, the commands' details and every Cube's rooms (log/), and the
   * sign-in card's choice still waited for and its lines (signin/).
   */
  signedOut(): void;
  /** The session's number now, which a plain load takes before its request (app.js, rooms.js, login.js; shared/session.ts). */
  session(): number;
  /** Whether the number a plain load took is still the session's: an answer after a sign-out does nothing (UiParts D212). */
  stillSession(n: number): boolean;
  /**
   * A server's event of the stream (app.js connectEvents): Software update, Maintenance and System info read
   * again what it says has changed while their page is shown, and a
   * Cube's log shown loads again at a resync (log/).
   */
  serverEvent(ev: ServerEvent): void;
  /** The rule a new password is held to (shared/password.tsx): "too_short", "too_long" or "" for one it takes, as Go's internal/password answers its testdata/cases.json; the UI test gives it every case. */
  passwordProblem: typeof passwordProblem;
  /** The server's name a host gives (shared/password.tsx), as Go's password.HostWord answers the hosts of its testdata/cases.json; the UI test gives it every host. */
  hostWord: typeof hostWord;
  /** The pages of the main area: which is shown, its address, who may open it (shared/pages.ts; app.js, home.js, login.js and header.js call it). */
  pages: typeof pagesPart;
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
  /** The log: its lines, its choices, its address and its unseen problems (log/; app.js' loads, poll, stream and language call it, and the UI tests read its state, its alerts and its matches). */
  log: LogPart;
  /** 👤 → Notifications: the rules, this browser and the devices (notify/). */
  notify: NotifyPart;
  /** The rooms: a tile per room with its device list (rooms/). */
  rooms: RoomsPart;
  /** Maintenance → Software update (software/). */
  software: SoftwarePart;
  /** System settings → Users: the password sign-in and a panel per user (admin/). */
  users: UsersPart;
  /** The sign-in's card, what askLogin of login.js hands it and the app's status for it (signin/). */
  signIn: SignInPart;
}

declare global {
  interface Window {
    /** The page's parts, registered when ui/parts.js runs. */
    MassimoParts: MassimoParts;
  }
}

register("language", languageChanged);
register("icons", iconsChanged);
register("minute", minuteTick);
register("pages", pagesPart);
register("passwordProblem", passwordProblem);
register("hostWord", hostWord);
register("serverEvent", (ev: ServerEvent) => {
  logEvent(ev);
  softwareEvent(ev);
  maintenanceEvent(ev);
  sysinfoEvent(ev);
});
register("session", session);
register("stillSession", stillSession);
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
  profileSignedOut();
  prefsSignedOut();
  notifySignedOut();
  logSignedOut();
  // the sign-in card last: the session's number and who was in go first (D211), the regions' data after
  // them, and only then the card's own lines, which the next opening shows
  signinSignedOut();
});
window.MassimoParts = parts as unknown as MassimoParts;
