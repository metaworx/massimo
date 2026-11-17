// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/leaving/index.tsx
 *
 * Leaving the page while a settings form has changes not saved (AP
 * UnsavedMarks, Block 3, D8-A and D9-A): closing, reloading or leaving
 * the page, the browser's own prompt asks, a browser allowing nothing
 * else there; where the user stays, the page's dialog (the shared
 * FormsDialog) lists those forms, each opening its form. At a sign-out,
 * which drops them, the dialog asks first (login.js logout). Moving
 * inside the page, to another of its pages or another Cube, leaves the
 * page loaded and asks nothing. A region of its own, drawn once into its
 * place at the end of the page as parts.js loads, since the page's chrome
 * is plain and no region may import another: it knows the forms only
 * through shared/forms.ts.
 */

import { effect, signal } from "@preact/signals";
import { useEffect } from "preact/hooks";
import { FormsDialog } from "../shared/dialog";
import { changedItems, leaveForms } from "../shared/forms";
import { useLang } from "../shared/lang";
import { mounter, register } from "../shared/region";

/**
 * What the dialog asks while shown: {} after the browser's prompt, where
 * the user stayed; at a sign-out, go, the label of the button that signs
 * out; null while it is closed.
 */
const asking = signal<{ go?: string } | null>(null);

/** The sign-out waiting for its answer (signOut): true to sign out, false to stay. */
let answer: ((go: boolean) => void) | null = null;

/** settle closes the dialog and answers a sign-out waiting for it. */
function settle(go: boolean): void {
  asking.value = null;
  const a = answer;
  answer = null;
  a?.(go);
}

/**
 * STAY_AFTER_MS is how long after the browser's prompt the dialog comes,
 * in ms. A browser says nothing to the page of the user's answer, and a
 * page that is left runs on until the next one comes; shown at once, the
 * dialog would come on a page that is leaving. The user who stays loses
 * nothing meanwhile: the forms are unsaved at once, with their disks.
 */
const STAY_AFTER_MS = 1000;

/** The dialog that comes after the browser's prompt, while it waits. */
let stayTimer: ReturnType<typeof setTimeout> | undefined;

/**
 * beforeUnload asks the browser for its prompt while a form has changes
 * not saved, merely changed or saving ones too, and counts every changed
 * form as left; STAY_AFTER_MS later, where the page is still there, the
 * dialog lists them. A page that goes before takes the dialog with it
 * (pageHide).
 */
function beforeUnload(ev: BeforeUnloadEvent): void {
  if (!changedItems().length) return;
  leaveForms();
  ev.preventDefault();
  ev.returnValue = ""; // the browsers that ask only where it is set
  clearTimeout(stayTimer);
  stayTimer = setTimeout(() => {
    if (!asking.peek() && changedItems().length) asking.value = {};
  }, STAY_AFTER_MS);
}

/**
 * pageHide takes back the dialog of a stay, and one that waits to come,
 * as the page goes: a page the browser keeps for its back button comes
 * back without it. A sign-out's question stays.
 */
function pageHide(): void {
  clearTimeout(stayTimer);
  if (asking.peek() && !asking.peek()?.go) asking.value = null;
}

/**
 * Leaving is the dialog, shown while asking and a form has changes; with
 * none left, the dialog of a stay closes and a sign-out goes on, nothing
 * being lost any more.
 */
function Leaving() {
  useLang(); // its texts in the page's language
  const forms = changedItems();
  const a = asking.value;
  const shown = !!a && forms.length > 0;
  // every form saved, reset or forgotten while it asks
  const gone = !!a && !forms.length;
  useEffect(() => {
    if (gone) settle(!!asking.peek()?.go);
  }, [gone]);
  const signOut = a?.go;
  return (
    <FormsDialog
      shown={shown}
      title={t("leave.title")}
      text={t(signOut ? "leave.signOutText" : "leave.stayText")}
      forms={forms}
      stay={t(signOut ? "leave.stay" : "action.close")}
      onStay={() => settle(false)}
      go={signOut ? { label: signOut, onGo: () => settle(true) } : undefined}
    />
  );
}

/**
 * The browser's prompt and the dialog's taking back, only while a form
 * has changes: a page with a beforeunload listener can lose the browser's
 * back and forward cache. Added the moment the forms' list has one, as
 * the list is set rather than at the next drawing, and removed once it
 * has none; the effect itself is the page's for its life.
 */
let listening = false;
effect(() => {
  const any = changedItems().length > 0;
  if (any === listening) return;
  listening = any;
  if (any) {
    window.addEventListener("beforeunload", beforeUnload);
    window.addEventListener("pagehide", pageHide);
  } else {
    window.removeEventListener("beforeunload", beforeUnload);
    window.removeEventListener("pagehide", pageHide);
  }
});

/** The dialog's place, the element of the page with the id leaving. */
const mount = mounter("leaving", () => <Leaving />);
mount();

/** The functions the plain code calls, MassimoParts.leaving. */
const leaving = register("leaving", {
  /**
   * signOut asks first where a form has changes not saved, which a
   * sign-out drops: it resolves true to sign out, false to stay, and at
   * once true where no form has any; label names the button that signs
   * out as the page's own does (Sign out, Forget key).
   */
  signOut(label: string): Promise<boolean> {
    if (!mount() || !changedItems().length) return Promise.resolve(true);
    leaveForms();
    answer?.(false);
    return new Promise((resolve) => {
      answer = resolve;
      asking.value = { go: label };
    });
  },
  /** changes says whether a form has changes not saved, which a reload would drop (header.js somethingOpen). */
  changes(): boolean {
    return changedItems().length > 0;
  },
});

/** MassimoParts.leaving, as index.ts declares it. */
export type LeavingPart = typeof leaving;
