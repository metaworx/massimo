// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/Pile.tsx
 *
 * The header's pile, before the Cube choice at the row's right: every
 * Cube's problem list, which the server's sign leads and which is a list
 * whatever is wrong or not. Its list starts with Refresh now, which polls
 * without closing it and shows how long ago the chosen Cube was read and
 * the server last replied, and the current time (Times.tsx, on the
 * header's one clock); then what the server's sign offers
 * (ServerSign.tsx); then the problems. Built once and updated in place,
 * so that an open list stays open and follows the data.
 */

import { useState } from "preact/hooks";
import { problemsLabelOf } from "../model/cube";
import { Icon } from "../shared/icon";
import { MenuItem } from "../shared/menu";
import { Problems } from "../shared/problems";
import { useBusy } from "../shared/question";
import { nowAgain } from "./clock";
import { cubeProblems } from "./CubeProblems";
import { ServerIcon, ServerItem, serverState, serverWords } from "./ServerSign";
import { load } from "./store";
import { ListTimes } from "./Times";

/**
 * RefreshNow is the list's first item: a poll now, turning while it runs,
 * the list left open to show its times (ListTimes), which move on with
 * the header's clock while the list is open (open, given by the pile,
 * which draws it again when it changes: a part that reads the load
 * counter is not drawn again with its parent otherwise).
 */
function RefreshNow({ open }: { open: boolean }) {
  load.useLoad();
  const busy = useBusy();
  return (
    <MenuItem cls="refresh-now" icon={<Icon symbol="icon-refresh" cls="ni" />} label={t("header.refreshTitle")} sub={<ListTimes open={open} />}
      keepOpen busy={busy.busy} onClick={() => void busy.run(() => poll(true))} />
  );
}

/** Pile is the header's pile, in #header-pile. */
export function Pile() {
  load.useLoad();
  const [open, setOpen] = useState(false);
  const opened = () => {
    nowAgain(); // the times from now as the list opens, not from the clock's last second
    setOpen(true);
  };
  const st = serverState();
  const list = cubeProblems();
  // before there is a Cube, nothing is said of its problems
  const label = [serverWords(st), state.cubes.length ? problemsLabelOf(list) : ""].filter(Boolean).join(" · ");
  return (
    <div class="pile" data-server={st}>
      <span id="cube-problems" class="cube-problems">
        <Problems entries={list} label={label} ok={state.cubes.length ? t("problem.none") : undefined} always lead={<ServerIcon st={st} />}
          head={<><RefreshNow open={open} /><ServerItem st={st} /></>} onOpen={opened} onClose={() => setOpen(false)} />
      </span>
    </div>
  );
}
