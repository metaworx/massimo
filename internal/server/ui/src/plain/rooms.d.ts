// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/rooms.d.ts
 *
 * The globals of the plain script ui/rooms.js that the parts call: when
 * the server received the current value of a device's state field.
 */

import type { Device } from "../api";

declare global {
  /** When the server received the current value of a field of the device's state. */
  function received(d: Device, field: string): string | undefined;
}

export {};
