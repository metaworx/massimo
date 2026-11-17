// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/Pile.tsx
 *
 * The header's pile, between the logo and the Cube choice: the chosen
 * Cube's age above every Cube's problem list, which the server's sign
 * leads and which is a list whatever is wrong or not. Its list starts
 * with Refresh now, which polls without closing it and shows when the
 * chosen Cube was read, when the server last answered and the time now,
 * in the browser's time zone, moving on each second while the list is
 * open; then what the server's sign offers (ServerSign.tsx); then the
 * problems. Where the logo, the pile and the pages do not fit one line
 * of a phone's header, the pile takes a line of its own and the age moves
 * to its right (data-line="own", style.css). Built once and updated in
 * place, so that an open list stays open and follows the data.
 */

import { useEffect, useLayoutEffect, useRef, useState } from "preact/hooks";
import { chosenCube, problemsLabelOf } from "../model/cube";
import { Icon } from "../shared/icon";
import { MenuItem } from "../shared/menu";
import { Problems } from "../shared/problems";
import { useBusy } from "../shared/question";
import { cubeProblems } from "./CubeProblems";
import { ServerIcon, ServerItem, serverState, serverWords } from "./ServerSign";
import { Updated } from "./Updated";
import { load } from "./store";

/** clockOf is an instant (ms, on the browser's clock) as its time of day (fmtTimeOfDay of app.js); "–" where there is none. */
function clockOf(ms: number): string {
  return !ms || isNaN(ms) ? "–" : fmtTimeOfDay(new Date(ms));
}

/**
 * timesText is Refresh now's second line, three lines in one, all on the
 * browser's clock: when the chosen Cube was last read (its updated, the
 * last message the server took from it, moved off the server's clock by
 * serverSkew, as its age is), when the server last answered the page, and
 * the time now.
 */
function timesText(): string {
  const cube = chosenCube();
  const read = cube ? timeOf(cube.updated) : null;
  return [
    t("header.cubeRead", { time: clockOf(read ? read.getTime() - serverSkew : 0) }),
    t("header.serverReplied", { time: clockOf(state.repliedAt) }),
    t("header.timeNow", { time: clockOf(Date.now()) }),
  ].join("\n");
}

/**
 * RefreshNow is the list's first item: a poll now, turning while it runs,
 * the list left open to show its times, which move on each second while
 * the list is open (open, given by the pile, which draws it again when it
 * changes: a part that reads the load counter is not drawn again with its
 * parent otherwise).
 */
function RefreshNow({ open }: { open: boolean }) {
  load.useLoad();
  const busy = useBusy();
  const [, tick] = useState(0);
  useEffect(() => {
    if (!open) return undefined;
    const timer = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(timer);
  }, [open]);
  return (
    <MenuItem cls="refresh-now" icon={<Icon symbol="icon-refresh" cls="ni" />} label={t("header.refreshTitle")} sub={timesText()}
      subCls="refresh-times" keepOpen busy={busy.busy} onClick={() => void busy.run(() => poll(true))} />
  );
}

/**
 * ownLine says whether the pile needs a line of its own in the header's
 * row: below 600 px, where the logo, the pile as a column (the wider of
 * its age and its pill) and the pages are wider than the row.
 */
function ownLine(pile: HTMLElement, line: HTMLElement): boolean {
  if (matchMedia("(min-width: 600px)").matches) return false;
  const width = (el: Element | null) => (el && (el as HTMLElement).offsetParent ? el.getBoundingClientRect().width : 0);
  const style = getComputedStyle(line);
  // a gap between each two of the row's tracks, the empty one before the
  // pages too (style.css .top-line)
  const gaps = (parseFloat(style.columnGap) || 0) * Math.max(0, style.gridTemplateColumns.split(" ").length - 1);
  const column = Math.max(width(pile.querySelector(".pile-age")), width(pile.querySelector(".problems-sum")));
  const need = width(line.querySelector(".brand")) + column + width(line.querySelector(".nav")) + gaps;
  return need > line.clientWidth + 0.5;
}

/** Pile is the header's pile, in #header-pile. */
export function Pile() {
  load.useLoad();
  const [open, setOpen] = useState(false);
  const [own, setOwn] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  // the pile's line measured again whenever the row, the logo, the pages
  // (↑ joins them while compacted) or the pile change size
  useLayoutEffect(() => {
    const pile = ref.current;
    const line = pile && pile.closest<HTMLElement>(".top-line");
    if (!pile || !line) return undefined;
    let frame = 0;
    const later = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => setOwn(ownLine(pile, line)));
    };
    const ro = new ResizeObserver(later);
    for (const el of [line, pile, line.querySelector(".brand"), line.querySelector(".nav")]) if (el) ro.observe(el);
    return () => {
      ro.disconnect();
      cancelAnimationFrame(frame);
    };
  }, []);
  const st = serverState();
  const list = cubeProblems();
  // before there is a Cube, nothing is said of its problems
  const label = [serverWords(st), state.cubes.length ? problemsLabelOf(list) : ""].filter(Boolean).join(" · ");
  return (
    <div ref={ref} class="pile" data-line={own ? "own" : undefined} data-server={st}>
      <span class="pile-age"><Updated /></span>
      <span id="cube-problems" class="cube-problems">
        <Problems entries={list} label={label} ok={state.cubes.length ? t("problem.none") : undefined} always lead={<ServerIcon st={st} />}
          head={<><RefreshNow open={open} /><ServerItem st={st} /></>} onOpen={() => setOpen(true)} onClose={() => setOpen(false)} />
      </span>
    </div>
  );
}
