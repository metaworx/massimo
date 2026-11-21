// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/brand.tsx
 *
 * Massimo's mark as a part (brandMarks of components.js, which draws it
 * over the skeleton's elements marked data-brand): the full logo and the
 * small mark from the sprite (ui/brand/brand.svg, make ui-brand,
 * tools/brand/README.md), each part with the class that colours it
 * (ui/brand/brand.css), the cover first; the page's style shows one of
 * the two, and the big name the sign-in's card and the first owner's
 * form open with (BrandName). The drawings are hidden from screen readers. A copy of
 * BRAND_PARTS and brandMarks of components.js, which stay for the
 * skeleton's marks, the connect page's among them: connect.html loads no
 * parts.js.
 */

/** The parts of the mark in the sprite, each with its class: the cover first. */
const PARTS: [string, string][] = [
  ["cover", "brand-cover"],
  ["back", "brand-back"],
  ["m", "brand-m"],
  ["hour", "brand-hands"],
  ["minute", "brand-hands"],
];

/** Drawing is one of the two drawings: its class, the sprite's prefix and its parts. */
function Drawing({ cls, prefix, parts }: { cls: string; prefix: string; parts: [string, string][] }) {
  return (
    <svg class={cls} aria-hidden="true" focusable="false">
      {parts.map(([part, partCls]) => <use key={part} href={"ui/brand/brand.svg#" + prefix + "-" + part} class={partCls} />)}
    </svg>
  );
}

/** Brand is Massimo's logo and small mark, for a place a part draws (☰'s first row). */
export function Brand() {
  return (
    <>
      <Drawing cls="brand-logo" prefix="logo" parts={[...PARTS, ["letters", "brand-letters"], ["dot", "brand-hands"]]} />
      <Drawing cls="brand-mark" prefix="mark" parts={PARTS} />
    </>
  );
}

/**
 * BrandName is Massimo's big name, the word under the logo once its
 * sprite has come, marked data-brand as the skeleton's (brandMarks of
 * components.js passes over it, drawn already, and the page's style and
 * its test find every mark by it): the sign-in's card and the first
 * owner's form.
 */
export function BrandName() {
  return (
    <div class="brand-name brand-big" data-brand>
      <span class="brand-word"><span class="brand-initial">M</span><span class="brand-rest">assimo</span></span>
      <Brand />
    </div>
  );
}
