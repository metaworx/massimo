// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/config.d.ts
 *
 * The globals of the plain script ui/config.js that the parts call: a
 * room's configuration, which stays plain inside its tile until its own
 * block (AP UiParts, Block 8), filled into the tile's .config-area.
 */

declare global {
  /** Opens the tile's configuration and loads it; el is the tile, whose menus it closes; tab the tab shown first. */
  function openConfig(room: PageRoom, el: HTMLElement | null, tab?: string): Promise<void>;
  /** Closes the tile's configuration. */
  function closeConfig(room: PageRoom): void;
  /** Fills the .config-area of the tile el with the room's configuration as it stands, or hides it. */
  function renderConfig(el: HTMLElement, room: PageRoom): void;
}

export {};
