// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/index.tsx
 *
 * The log as a part (AP UiParts, Block 11): its page drawn once into its
 * section (#logview) at log.js initLog and updated in place; its address
 * of its own handed to the pages' store (ownAddress), which reads it at a
 * reload, a link, back and forward and tells the log when it is shown;
 * the log's unseen problems handed to ☰ (alerts.ts). The plain code calls
 * MassimoParts.log through log.js' plain names (app.js' stream, poll,
 * loads and language; tiles.js' rfLabel; the UI tests), and reads the
 * page globals logState and logAlerts, which are this region's state.
 */

import { rfLabel } from "../model/device";
import { ownAddress } from "../shared/pages";
import { mounter, register } from "../shared/region";
import { logRangeStart, upperFirst } from "./text";
import { logAlerts, loadLogAlerts, renderLogSign } from "./alerts";
import { logHash, readLogHash } from "./address";
import { LOG_KINDS } from "./kinds";
import type { LogEvent, LogLine } from "./lines";
import { Log } from "./Log";
import { logState } from "./state";
import {
  appendLogEvent, fillLogSelects, loadLog, logChoicesChanged, logCubeChanged, logCubesPolled, logLineMatches, logLoadUsers, logOfChoices,
  logPollSet, pollLog, pruneLogRange, renderLog,
} from "./store";

const mount = mounter("logview", () => <Log />);

// The log's address of its own: read from an address of the log, written
// for it (a device the Cube does not have goes before the address takes
// it), told whether the log is shown, then that it was opened.
ownAddress("log", {
  read: readLogHash,
  write: () => {
    fillLogSelects();
    return logHash();
  },
  shown: (on) => {
    logState.open = on;
    logPollSet();
  },
  opened: () => {
    loadLog();
    logLoadUsers();
  },
});

/** The functions the plain code calls, MassimoParts.log (log.js). */
const log = register("log", {
  /** state is what the log keeps, the page global logState. */
  state: logState,
  /** alerts are the log's unseen problems, the page global logAlerts. */
  alerts: logAlerts,
  /** kinds are the kinds of event the server sends (app.js connectEvents). */
  kinds: LOG_KINDS,
  /** mount draws the log's page into its section (log.js initLog). */
  mount(): void {
    mount();
  },
  /** event adds a live event of the stream (log.js appendLogEvent). */
  event(ev: LogEvent): void {
    appendLogEvent(ev);
  },
  render: renderLog,
  load: loadLog,
  poll: pollLog,
  prune: pruneLogRange,
  cubesPolled: logCubesPolled,
  cubeChanged: logCubeChanged,
  selects: fillLogSelects,
  choicesChanged: logChoicesChanged,
  loadAlerts: loadLogAlerts,
  sign: renderLogSign,
  matches: (line: LogLine) => logLineMatches(line),
  ofChoices: (line: LogLine) => logOfChoices(line),
  rangeStart: logRangeStart,
  /** rfLabel is model/device.ts', which tiles.js reaches through the log's plain name until AP UiParts' Block 15 gives it a home of its own. */
  rfLabel,
  upperFirst,
});

/** MassimoParts.log, as index.ts declares it. */
export type LogPart = typeof log;
