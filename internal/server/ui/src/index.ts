// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The entry point of the page's parts (AP UiParts), which make ui bundles
// into ui/parts.js, loaded after the plain scripts. The plain code reaches
// a part through window.MassimoParts.<region>: each region that becomes a
// part adds its member to MassimoParts here, with the functions the plain
// code calls (docs/ui.md, "How the page is built"). No region is a part
// yet.

/** The parts the plain code calls, by region. */
export interface MassimoParts {}

declare global {
  interface Window {
    /** The page's parts, registered when ui/parts.js runs. */
    MassimoParts: MassimoParts;
  }
}

window.MassimoParts = {};
