// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/pairing.d.ts
 *
 * The globals of the plain script ui/pairing.js that the parts call: a
 * Cube's pairing, which stays plain inside its panel until its own block
 * (AP UiParts, Block 9), filled into the panel's .cube-pairing.
 */

declare global {
  /** Asks for a Cube's pairing state and draws its pairing again. */
  function loadPairing(serial: string): Promise<void>;
  /** Loads the rooms of a Cube that is not the one shown, for the pairing's room choice. */
  function loadCubeRooms(serial: string): Promise<void>;
  /** Fills the .cube-pairing of the Cube's panel with its pairing, as it stands. */
  function redrawPairing(serial: string): void;
}

export {};
