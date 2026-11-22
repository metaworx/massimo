// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/kv.tsx
 *
 * A list of facts as a part (components.js' kvList before): a name and its
 * value each, the values in a column of their own, a value that is an
 * identifier or a path in the monospaced font; and a list of counts by
 * name (CountList).
 */

/** A fact of KvList: its name, its value, and whether the value is monospaced. */
export type Fact = [name: string, value: string, mono?: boolean];

/** KvList is the list (dl.kv) of facts, every one given; the caller leaves out those it does not show. */
export function KvList({ facts }: { facts: Fact[] }) {
  return (
    <dl class="kv">
      {facts.map(([name, value, mono]) => [
        <dt key={"t:" + name}>{name}</dt>,
        <dd key={"d:" + name} class={mono ? "mono" : undefined}>{value}</dd>,
      ])}
    </dl>
  );
}

/** CountList is a list of counts by name (ul.kv-rows), the names sorted, in grey, the numbers right-aligned, in as many columns as there is room for. */
export function CountList({ counts }: { counts: Record<string, number> }) {
  return (
    <ul class="kv-rows">
      {Object.keys(counts).sort().map((name) => <li key={name}><span>{name}</span><span>{fmtNum(counts[name], 0)}</span></li>)}
    </ul>
  );
}
