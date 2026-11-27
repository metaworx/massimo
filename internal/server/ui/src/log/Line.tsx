// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/Line.tsx
 *
 * A line of the log (AP UiParts, Block 11; log.js renderLogLine and
 * renderLogCommand, the template tpl-log-line): the time, the kind, the
 * device when the line has one, by its room and name (logDeviceName),
 * with its S/N as the tooltip and, with its radio address, in expert
 * mode, and the text. The Cube's own configuration is the Cube's, by its
 * S/N; a removed device is named by the text alone. Under All Cubes a
 * Cube's line names its Cube first, "Upstairs › Kitchen › Window", and
 * the Cube's own lines by it alone. An unsettled problem not marked seen
 * is highlighted, with its mark and Mark seen; a command's line has its
 * button, which opens the command's detail under it. A line is drawn
 * once and updated in place: its alert (levelOf) and its detail are read
 * by the line itself, so that a change of either draws that line alone.
 */

import { useComputed } from "@preact/signals";
import { useLayoutEffect, useRef } from "preact/hooks";
import { Mark } from "../shared/icon";
import { levelOf } from "./alerts";
import type { LogLine, Loose } from "./lines";
import { logState } from "./state";
import { markFocus, seen, toggleLogCommand } from "./store";
import { logDeviceName, logDeviceSerial, logKind, logLineCube, logNamePath, logText, logWord } from "./text";
import { useLook } from "../shared/look";

/** CommandDetail is a command's detail: its action, target, state and who asked, its steps, and its audit row for a user with audit. */
function CommandDetail({ id }: { id: number }) {
  useLook();
  const c = logState.commands.get(id);
  if (!c) {
    return (
      <div class="log-cmd-detail">
        <p>{t("log.cmd.loading")}</p>
      </div>
    );
  }
  if (c.error) {
    return (
      <div class="log-cmd-detail">
        <p>{String(c.error)}</p>
      </div>
    );
  }
  const state = logWord("log.cmdState.", c.state);
  let head = t("log.cmd.head", { action: c.action, target: c.target || "–", state });
  const who = (c.audit && c.audit.real_user && c.audit.real_user.display) || c.user;
  if (who) head = t("log.by", { text: head, name: who });
  const steps = ((c.steps || []) as Loose[]).map((st) => {
    const r = st.result || {};
    let text = t("log.cmd.step", { state: logWord("log.cmdState.", st.state), target: st.target || "–" });
    if (r.discarded) text += "; " + t("log.cmd.discarded");
    if (r.error) text += "; " + refusalText(r, c.cube || "", String(r.error));
    return text;
  });
  const a = c.audit;
  return (
    <div class="log-cmd-detail">
      <p class="log-cmd-head">{head}</p>
      {steps.length > 0 && (
        <ol class="log-cmd-steps">
          {steps.map((text, i) => <li key={i}>{text}</li>)}
        </ol>
      )}
      {a && (
        <p class="log-cmd-audit">
          {t("log.cmd.audit", { action: a.action, user: (a.real_user && a.real_user.display) || a.user || "–", remote: a.remote || "–", result: logWord("log.auditResult.", a.result) })}
        </p>
      )}
    </div>
  );
}

/**
 * MarkSeen is a problem's Mark seen, which takes the focus where the
 * Mark seen clicked before it hands it on (markFocus), in the drawing's
 * own turn; drawn for a problem's line alone, so that only those follow
 * where the focus goes.
 */
function MarkSeen({ id }: { id: number }) {
  useLook();
  const wanted = useComputed(() => markFocus.value === id).value;
  const ref = useRef<HTMLButtonElement>(null);
  useLayoutEffect(() => {
    if (!wanted) return;
    if (ref.current) ref.current.focus();
    markFocus.value = null; // taken, button or not
  }, [wanted]);
  return (
    <button ref={ref} type="button" class="subtle log-mark" onClick={() => seen(id)}>
      {t("log.markSeen")}
    </button>
  );
}

/** Line is one line of the log, as renderLogLine drew it. */
export function Line({ line }: { line: LogLine }) {
  useLook();
  const open = useComputed(() => !!line.command && logState.openCommands.has(line.command));
  const alert = levelOf(line.id).value;
  const tone = alert === "error" ? "bad" : alert ? "warn" : "";
  const cube = logLineCube(line);
  let name = "";
  let serial = "";
  if (line.device && !line.gone && line.source !== "cube") {
    name = logDeviceName(line);
    serial = logDeviceSerial(line);
  } else if (cube) {
    name = cube;
  } else if (line.device && !line.gone) {
    name = t("log.cubeDevice", { serial: line.serial || line.device });
  }
  const sn = serial && name !== serial ? serial : "";
  return (
    <div class={"log-line kind-" + line.kind + (tone ? " log-alert log-alert-" + tone : "")} data-id={line.id ? String(line.id) : undefined}>
      <span class="log-time mono">{fmtTimeOfDay(line.time)}</span>
      <span class="log-kind">
        {tone && <Mark key={tone} state={tone} blink={"log-line-" + line.id} />}
        {logKind(line.kind)}
      </span>
      {name && <span class="log-device" title={sn || undefined}>{cube && name !== cube ? logNamePath(cube, name) : name}</span>}
      {sn && <span class="mono expert-only log-sn">{sn}</span>}
      {line.device && <span class="mono expert-only log-rf">{line.device}</span>}
      <span class="log-text">{logText(line)}</span>
      {tone && <MarkSeen id={line.id} />}
      {line.command > 0 && (
        <button type="button" class="subtle log-cmd" title={t("log.cmd.title")} aria-expanded={open.value ? "true" : "false"} onClick={() => toggleLogCommand(line.command)}>
          {t("log.cmd.open", { id: line.command })}
        </button>
      )}
      {open.value && <CommandDetail id={line.command} />}
    </div>
  );
}
