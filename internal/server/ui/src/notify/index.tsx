// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/notify/index.tsx
 *
 * 👤 → Notifications as a part (AP UiParts, Block 13), drawn once into
 * its card's body, #notify-sec, as parts.js loads, and updated in place;
 * read each time the personal page is shown, the line emptied first, as
 * notify.js did. The plain code calls MassimoParts.notify: app.js
 * render at a change of language, login.js holdPushes and resumePushes
 * takeSave and save around a sign-in, and logout sendNow before the
 * session ends. The toast of a notification and the
 * service worker's language stay plain in notify.js.
 */

import { mounter, register } from "../shared/region";
import { NotifyCard } from "./Notify";
import { dropSave, loadNotify, saveRules, sendNow, takeSave } from "./send";
import { forgetNotify, notify, say } from "./store";
import { onView } from "../shared/pages";

const mount = mounter("notify-sec", () => <NotifyCard />);
mount();

/** The functions the plain code calls, MassimoParts.notify. */
const parts = register("notify", {
  /** render draws the card again (app.js, at a change of language). */
  render(): void {
    notify.ver++;
  },
  /** save sends the rules a second from now (login.js resumePushes). */
  save: saveRules,
  /** takeSave ends a save that waits and says whether one did (login.js holdPushes). */
  takeSave,
  /** sendNow sends a save that waits at once, under the user's token (login.js logout, D205). */
  sendNow,
});

/** MassimoParts.notify, as index.ts declares it. */
export type NotifyPart = typeof parts;

/** signedOut forgets what the card kept of the user who was signed in (MassimoParts.signedOut, index.ts); a save can no longer wait then (send.ts dropSave). */
export function signedOut(): void {
  dropSave();
  forgetNotify();
}

// the personal page shown again starts with an empty line, then reads the rules
onView("me", () => {
  say("", "");
  loadNotify();
});
