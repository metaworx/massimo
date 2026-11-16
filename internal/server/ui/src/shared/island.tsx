// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/island.tsx
 *
 * A plain piece inside a part: the part draws the element empty and
 * leaves its content to plain code, which fills it when the island is
 * there and again where one of its keys or the language changed, so that
 * a focused control in it stays when data comes in. The plain code may
 * redraw inside the element by itself; it never replaces the element.
 */

import { useLayoutEffect, useRef } from "preact/hooks";
import { useLang } from "./lang";

/**
 * Island is the element (tag, a div by default, with cls) that fill
 * fills; keys are what its content is drawn from, a list of the same
 * length at every draw.
 */
export function Island({ tag, cls, keys, fill }: { tag?: "div" | "p" | "span"; cls: string; keys: unknown[]; fill: (el: HTMLElement) => void }) {
  const ref = useRef<HTMLElement>(null);
  const lang = useLang();
  // biome-ignore lint/correctness/useExhaustiveDependencies: keys are the island's dependencies by design, and the language, which fill reads; fill is a new function at every draw, and listed it would fill the island again at every poll and lose what the user holds in it.
  useLayoutEffect(() => {
    if (ref.current) fill(ref.current);
  }, [...keys, lang]);
  if (tag === "p") return <p ref={ref as { current: HTMLParagraphElement | null }} class={cls} />;
  if (tag === "span") return <span ref={ref as { current: HTMLSpanElement | null }} class={cls} />;
  return <div ref={ref as { current: HTMLDivElement | null }} class={cls} />;
}
