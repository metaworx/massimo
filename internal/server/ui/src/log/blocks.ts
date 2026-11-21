// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/blocks.ts
 *
 * The log's lines in blocks (AP UiParts, Block 11), so that a new line
 * costs the drawing of its block and not of every line: a block is a run
 * of lines of logState.lines, a stable order, cut by the lines' keys and
 * kept by them, whatever the filter and the choices show of it: a line
 * keeps its block, and so its element, while it is among the lines, shown
 * or not. The lines a load brings fill blocks of about BLOCK from the
 * oldest up; a new line on top joins the newest block while it has room;
 * a line that comes between two of a block joins it. A block is drawn
 * with those of its lines the view shows, and handed to Preact as the
 * same element while they are the same, which it leaves as it is. A
 * block stands for no element of the page: its lines are the box's
 * children, as before.
 */

import type { VNode } from "preact";
import type { LogLine } from "./lines";

/** BLOCK is how many lines a block holds before the next begins. */
export const BLOCK = 200;

/** The block of each line, by the line's key. */
let blockOf = new Map<string, number>();

/** The last block given out. */
let lastBlock = 0;

/** Each block's lines shown at its last drawing, and its element. */
let made = new Map<number, { lines: LogLine[]; el: VNode }>();

/**
 * cut gives every line of lines (oldest first) its block: the one it has,
 * else that of the line before it where the line after it has the same
 * or where that block has room, else a new one; a block met again after
 * another began is given anew from there, so that no block comes apart.
 */
function cut(lines: LogLine[]): void {
  const next = new Map<string, number>();
  const size = new Map<number, number>();
  const closed = new Set<number>();
  const moved = new Map<number, number>();
  // the block of the next line that has one, for each line, from the end
  const ahead: (number | undefined)[] = new Array(lines.length);
  for (let i = lines.length - 1, b: number | undefined; i >= 0; i--) {
    ahead[i] = b;
    const own = blockOf.get(lines[i].key || "");
    if (own !== undefined) b = own;
  }
  let prev = 0;
  for (let i = 0; i < lines.length; i++) {
    const key = lines[i].key || "";
    let b = blockOf.get(key);
    while (b !== undefined && closed.has(b)) {
      let to = moved.get(b);
      if (to === undefined) {
        to = ++lastBlock;
        moved.set(b, to);
      }
      b = to;
    }
    if (b === undefined) b = prev && (ahead[i] === prev || (size.get(prev) || 0) < BLOCK) ? prev : ++lastBlock;
    if (prev && b !== prev) closed.add(prev);
    prev = b;
    size.set(b, (size.get(b) || 0) + 1);
    next.set(key, b);
  }
  blockOf = next;
}

/**
 * blocks are the elements of view's lines (newest first), a block each,
 * newest first, the blocks cut over lines (oldest first, view's lines
 * among them); make makes a block's element from its id and its lines.
 */
export function blocks(view: LogLine[], lines: LogLine[], make: (id: number, lines: LogLine[]) => VNode): VNode[] {
  cut(lines);
  const runs: { id: number; lines: LogLine[] }[] = [];
  for (const line of view) {
    const id = blockOf.get(line.key || "") ?? 0;
    const run = runs[runs.length - 1];
    if (run && run.id === id) run.lines.push(line);
    else runs.push({ id, lines: [line] });
  }
  const out: VNode[] = [];
  const next = new Map<number, { lines: LogLine[]; el: VNode }>();
  for (const run of runs) {
    if (next.has(run.id)) run.id = ++lastBlock; // not one of lines: a block of its own
    const was = made.get(run.id);
    const same = !!was && was.lines.length === run.lines.length && was.lines.every((l, j) => l === run.lines[j]);
    const el = same && was ? was.el : make(run.id, run.lines);
    next.set(run.id, { lines: run.lines, el });
    out.push(el);
  }
  made = next;
  return out;
}
