// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/index.tsx
 *
 * The sign-in as parts (AP UiParts, Block 14): the first owner's form
 * into #setup-slot and the open server's warning into #open-banner-slot,
 * drawn once as parts.js loads and from then on by who is in
 * (shared/me.ts), which login.js applyCaps hands over at every read of it.
 */

import { mounter } from "../shared/region";
import { Banner } from "./Banner";
import { Setup } from "./Setup";

mounter("setup-slot", () => <Setup />)();
mounter("open-banner-slot", () => <Banner />)();
