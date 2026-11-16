// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/counter.ts
 *
 * A region's load counter: while the page's data stay its plain objects
 * (AP UiParts, until Block 15), the counter says "draw this region again",
 * raised where the data were loaded or changed. Every part of the region
 * reads it with use(), which also reads the language and the icons
 * chosen, the other things a part depends on beside its properties (rule
 * 2 of AP UiParts).
 */

import { signal } from "@preact/signals";
import { useLang } from "./lang";
import { useIcons } from "./look";

/** A region's counter: bump() after a load, use() in every part of the region. */
export interface Counter {
  /** bump says that the region's data were loaded or changed. */
  bump(): void;
  /** use reads the counter, the language and the icons chosen in a part's drawing. */
  use(): void;
}

/** counter makes a region's load counter. */
export function counter(): Counter {
  const n = signal(0);
  return {
    bump() {
      n.value++;
    },
    use() {
      void n.value;
      useLang();
      useIcons();
    },
  };
}
