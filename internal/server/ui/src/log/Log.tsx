// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/Log.tsx
 *
 * The log's page (AP UiParts, Block 11; partials/log-view.html and
 * log.js initLog): its head with the title, the text filter, the choices
 * of Cubes, kind, source or device, level, who and how far back, Clear
 * and Mark all as seen; the lines, newest first; and Older events. Drawn
 * once into its section and updated in place: a new line adds its
 * element and builds no other, and draws its block alone (blocks.ts);
 * each line's element is kept while it is the same event, and the
 * scroll moves as log.js moved it (store.ts).
 */

import type { VNode } from "preact";
import { useLayoutEffect, useRef } from "preact/hooks";
import { Select, TextField } from "../shared/field";
import { logAlerts, markAllLogSeen } from "./alerts";
import { blocks } from "./blocks";
import { clearOff, cubeOptions, cubesHidden, deviceOptions, deviceValue, kindOptions, levelOptions, rangeOptions, userOptions, usersHidden } from "./choices";
import { LOG_COUNT_RANGE, LOG_RANGES } from "./kinds";
import { Line } from "./Line";
import type { LogLine } from "./lines";
import { logState } from "./state";
import { clearLogFilter, drawn, fillLogSelects, filterTyped, loadOlder, logChoicesChanged, markFocus, scrolled, setBox, view } from "./store";
import { useLook } from "../shared/look";

/** The element of each line, made once: the list's drawing hands Preact the same element again, which it leaves as it is. */
const made = new WeakMap<LogLine, VNode>();

/** lineEl is a line's element, by its key (store.ts keyLines). */
function lineEl(line: LogLine): VNode {
  let el = made.get(line);
  if (!el) {
    el = <Line key={line.key} line={line} />;
    made.set(line, el);
  }
  return el;
}

/** Block is a block of lines (blocks.ts): its lines' elements, standing in the box itself. */
function Block({ lines }: { lines: LogLine[] }) {
  return <>{lines.map(lineEl)}</>;
}

/** blockEl makes a block's element. */
function blockEl(id: number, lines: LogLine[]): VNode {
  return <Block key={"b" + id} lines={lines} />;
}

/** Lines are the lines drawn, newest first, in blocks, in the box whose scroll the store moves. */
function Lines() {
  const ref = useRef<HTMLDivElement>(null);
  const lines = view.value;
  useLayoutEffect(() => {
    setBox(ref.current);
    return () => setBox(null);
  }, []);
  // after every drawing of the lines: the scroll steps their change queued
  useLayoutEffect(() => {
    drawn();
  });
  return (
    <div id="log-lines" class="log-lines" ref={ref} onScroll={(ev) => scrolled(ev.currentTarget.scrollTop)}>
      {blocks(lines, logState.lines, blockEl)}
    </div>
  );
}

/** Older shows whether Older can page further back; the newest LOG_COUNT have no Older, a time range pages back within it. */
function Older() {
  useLook();
  const can = logState.more && logState.cursor !== null;
  return (
    <div class={"log-older" + (logState.range === LOG_COUNT_RANGE ? " hidden" : "")}>
      <button type="button" id="log-older" class="subtle" disabled={!can || logState.loading} onClick={() => loadOlder()}>
        {t(can ? "log.older" : "log.noOlder")}
      </button>
    </div>
  );
}

/** Head is the log's head: the title, the filter, the choices, Clear and Mark all as seen. */
function Head() {
  useLook();
  const searchRef = useRef<HTMLInputElement>(null);
  const toSearch = markFocus.value === "search";
  // the focus handed on from the last Mark seen, in the drawing's own turn
  useLayoutEffect(() => {
    if (!toSearch) return;
    if (searchRef.current) searchRef.current.focus();
    markFocus.value = null; // taken, field or not
  }, [toSearch]);
  return (
    <header class="tile-head">
      <h2>{t("log.title")}</h2>
      <TextField row={false} id="log-filter" type="search" aria={t("log.filterAria")} placeholder={t("log.filter")} value={logState.filter} set={filterTyped} inputRef={searchRef} />
      <Select row={false} id="log-cubes" aria={t("log.cubesAria")} selectCls={cubesHidden() ? "hidden" : undefined} options={cubeOptions()} value={logState.cubes}
        onChange={(v) => {
          logState.cubes = v === "all" ? "all" : "";
          fillLogSelects(); // the devices of every Cube, or the header's alone
          logChoicesChanged();
        }} />
      <Select row={false} id="log-kind" aria={t("log.kindAria")} options={kindOptions()} value={logState.kind}
        onChange={(v) => {
          logState.kind = v;
          logChoicesChanged();
        }} />
      <Select row={false} id="log-device" aria={t("log.deviceAria")} options={deviceOptions()} value={deviceValue()}
        onChange={(v) => {
          logState.source = v.startsWith("src:") ? v.slice(4) : "";
          logState.device = v.startsWith("src:") ? "" : v;
          logChoicesChanged();
        }} />
      <Select row={false} id="log-level" aria={t("log.levelAria")} options={levelOptions()} value={logState.level}
        onChange={(v) => {
          logState.level = ["debug", "info", "warn"].includes(v) ? v : "info";
          logChoicesChanged();
        }} />
      <Select row={false} id="log-user" aria={t("log.userAria")} selectCls={usersHidden() ? "hidden" : undefined} options={userOptions()} value={logState.user}
        onChange={(v) => {
          logState.user = v;
          logChoicesChanged();
        }} />
      <Select row={false} id="log-range" aria={t("log.rangeAria")} options={rangeOptions()} value={logState.range}
        onChange={(v) => {
          logState.range = LOG_RANGES.includes(v) ? v : LOG_COUNT_RANGE;
          logChoicesChanged();
        }} />
      <button type="button" id="log-clear" disabled={clearOff()} onClick={clearLogFilter}>
        {t("log.clear")}
      </button>
      <button type="button" id="log-seen-all" class={logAlerts.list.length ? undefined : "hidden"} onClick={markAllLogSeen}>
        {t("log.markAllSeen")}
      </button>
    </header>
  );
}

/** Log is the log's page, drawn into its section (#logview). */
export function Log() {
  return (
    <>
      <Head />
      <Lines />
      <Older />
    </>
  );
}
