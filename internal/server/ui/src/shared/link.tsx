// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/link.tsx
 *
 * A link that opens in a tab of its own, as components.js' linkEl did: a
 * url that is not http(s), as one from the network may be, gives the text
 * alone.
 */

/** Link is a link to url, its text text, its tooltip the url; cls its class. */
export function Link({ url, text, cls }: { url?: string; text: string; cls?: string }) {
  if (!/^https?:\/\//i.test(url || "")) return <span class={cls}>{text}</span>;
  return <a class={cls} href={url} target="_blank" rel="noopener noreferrer" title={url}>{text}</a>;
}
