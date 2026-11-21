// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/notify.d.ts
 *
 * The globals of the plain script ui/notify.js that the parts call: the
 * service worker's registration, and the page's language told to it.
 */

declare global {
  /** The service worker's registration of the page, null for none. */
  function workerRegistration(): Promise<ServiceWorkerRegistration | null>;
  /** Tells the service worker the page's language, in which it shows a notification. */
  function tellWorkerLanguage(): Promise<void>;
}

export {};
