// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/repair.tsx
 *
 * The repairs of a Cube's rooms and devices as parts (docs/ui.md,
 * "Repairs"; docs/api-v1.md, "Repairs"), for a role with repair, wherever
 * a Cube, a room or a device is shown: Repair…, and the box of the
 * repairs offered there. Each says what it would send and what that
 * costs, from its preview (GET, which sends nothing), and asks before it
 * runs (POST with the preview's plan), "unlink all and re-link" twice. A
 * box's state is kept by its scope, live: open, the previews, a name
 * typed, a question open, a run under way and its outcome, so that a
 * tile, a row or a panel that follows data never closes a box or drops
 * what is typed in it. Moved from repair.js (AP UiParts, Block 9).
 */

import { signal } from "@preact/signals";
import { useEffect, useId, useRef } from "preact/hooks";
import type { RepairPlan, RepairResult } from "../api";
import type { Outcome } from "./action";
import { ResultLine } from "./badge";
import { TextField } from "./field";
import { Fold } from "./fold";
import { useLang } from "./lang";
import { live } from "./live";
import { defaultDeviceName, NAME_BYTES, nameFits } from "./name";
import { busyProps, Question } from "./question";
import { session, stillSession } from "./session";

/** What a box of repairs is for: the Cube, a room or a device of it. */
export type RepairKind = "cube" | "room" | "device";

/**
 * A repair a box offers: key names its texts (repair.<key>,
 * repair.<key>Help), route is the end of its path, needs a capability it
 * needs besides repair, and warn says that Run… warns first and asks
 * twice ("unlink all and re-link", D163-B).
 */
interface Repair {
  key: string;
  route: string;
  needs?: string;
  warn?: boolean;
}

/** REPAIRS are the repairs a box of each kind offers, in their order. */
const REPAIRS: Record<RepairKind, Repair[]> = {
  cube: [{ key: "reset", route: "/repair/reset" }],
  room: [
    { key: "roomReset", route: "/repair/reset" },
    { key: "master", route: "/repair/master" },
    { key: "relink", route: "/repair/relink" },
    { key: "unlinkRelink", route: "/repair/unlink-relink", needs: "structure", warn: true },
  ],
  device: [
    { key: "config", route: "/repair/config" },
    { key: "place", route: "/repair/place" },
    { key: "list", route: "/repair/list" },
  ],
};

/**
 * A repair of a box, live: its state, "loading", "ready" (its preview),
 * "none" (error, the preview's refusal or why it has nothing to do),
 * "name" (a listing that needs the room's name, room), "running", then
 * "ready" again with the outcome of the run (result); a listing keeps the
 * names it was read with (roomName, devName) and what their fields hold
 * (roomTyped, devTyped, the names read where undefined), and the line
 * under the device's name (note); the question open in place of Run…
 * (ask: confirm, or for "unlink all and re-link" the warning, then
 * again), and whether the lines' fold is open.
 */
interface Item {
  state: "loading" | "ready" | "none" | "name" | "running";
  preview: RepairPlan | null;
  error: string;
  room: number | undefined;
  roomName: string;
  devName: string | undefined;
  roomTyped: string | undefined;
  devTyped: string | undefined;
  note: Outcome | null;
  result: Outcome | null;
  ask: "" | "confirm" | "warn" | "again";
  lines: boolean;
}

/**
 * An open box: its scope, its kind, the API path its routes hang off, names (the
 * names of the devices of a room of its Cube, for the name a listing
 * offers), the repairs it leaves out, and its repairs by key.
 */
interface Box {
  scope: string;
  kind: RepairKind;
  path: string;
  names: (room: number) => string[];
  skip: string[];
  items: Record<string, Item>;
}

/** The open boxes by scope; a closed box is forgotten. */
const boxes = signal<Record<string, Box>>({});

/** repairScope is the scope of a box: the Cube, a room or a device of it. */
export function repairScope(serial: string, kind: RepairKind, id?: number | string): string {
  return serial + "|" + kind + (id === undefined ? "" : "|" + id);
}

/** What a box is opened with beside its scope, its kind and its path: the repairs it leaves out, and the names of a room's devices. */
export interface RepairOpts {
  skip?: string[];
  names?: (room: number) => string[];
}

/** toggleRepair opens a box, reading every preview, or closes it where it is open. */
export function toggleRepair(scope: string, kind: RepairKind, path: string, { skip, names }: RepairOpts = {}): void {
  const all = { ...boxes.value };
  if (all[scope]) {
    delete all[scope];
    boxes.value = all;
    return;
  }
  const box: Box = { scope, kind, path, names: names || (() => []), skip: skip || [], items: {} };
  all[scope] = box;
  boxes.value = all;
  loadRepairs(box);
}

/** repairsOf are the repairs a box offers: those it does not leave out, for which the user has what they need besides repair. */
function repairsOf(box: Box): Repair[] {
  return REPAIRS[box.kind].filter((r) => !box.skip.includes(r.key) && (!r.needs || hasCap(r.needs)));
}

/** newItem is a repair's state before its preview has come. */
function newItem(): Item {
  return live<Item>({ state: "loading", preview: null, error: "", room: undefined, roomName: "", devName: undefined, roomTyped: undefined,
    devTyped: undefined, note: null, result: null, ask: "", lines: false });
}

/** open says whether box is still the one open for its scope: a box closed, or closed and opened again, reads nothing more. */
function open(box: Box): boolean {
  return boxes.peek()[box.scope] === box;
}

/** loadRepairs reads every preview of a box, keeping a run's outcome. */
function loadRepairs(box: Box): void {
  for (const r of repairsOf(box)) {
    const item = box.items[r.key] || (box.items[r.key] = newItem());
    item.state = "loading";
    loadRepair(box, r, item);
  }
}

/**
 * repairQuery is the query of a repair's routes: a listing's room name,
 * and the name of a device the list lacks where it fits the Cube's bytes;
 * the name offered may not (D201), and stays until the user shortens it.
 */
function repairQuery(item: Item): string {
  const q: string[] = [];
  if (item.roomName) q.push("room_name=" + encodeURIComponent(item.roomName));
  if (item.devName && nameFits(item.devName)) q.push("name=" + encodeURIComponent(item.devName));
  return q.length ? "?" + q.join("&") : "";
}

/** listing says whether a preview is a listing of a device the Cube's list lacks, which asks for its name. */
function listing(r: Repair, p: RepairPlan | null): boolean {
  return r.key === "list" && !!p && p.details?.listed === false;
}

/**
 * loadRepair reads one repair's preview into item: a preview with
 * nothing to send says why, one that needs a room's name asks for it. A
 * listing of a device the list lacks takes the name it offers
 * (defaultDeviceName) the first time, and reads the preview again with it
 * where it fits; one too long is shown, marked, with nothing to run until
 * it is shortened (RepairItem).
 */
async function loadRepair(box: Box, r: Repair, item: Item): Promise<void> {
  // A question never outlives the preview it asked about: read again, it
  // is gone, and Run… asks again about the new one.
  item.ask = "";
  const n = session();
  try {
    const p = await api<RepairPlan>("GET", box.path + r.route + repairQuery(item));
    if (!stillSession(n)) return; // an answer after a sign-out: nothing of it for the next user
    const d = p.details || {};
    item.preview = p;
    if (d.code === "room_name_needed") {
      item.room = Number(d.room);
      item.state = "name";
    } else if (p.nothing) {
      item.error = p.nothing;
      item.state = "none";
    } else if (listing(r, p) && item.devName === undefined) {
      item.devName = defaultDeviceName(String(d.room_name || ""), typeName(String(d.type || "")), box.names(Number(d.to)));
      if (nameFits(item.devName)) return open(box) ? loadRepair(box, r, item) : undefined; // its plan is the server's default's
      item.state = "ready";
    } else {
      item.state = "ready";
    }
  } catch (e) {
    if (!stillSession(n)) return;
    item.preview = null;
    item.error = (e as Error).message;
    item.state = "none";
  }
}

/** repairCost says what a preview costs: its radiograms, their duty cycle and memory slots, or that it costs no radio time. */
function repairCost(p: RepairPlan): string {
  if (p.radiograms > 0) {
    return t("repair.costRadio", { radiograms: p.radiograms, duty: fmtNum(p.duty_cost_pct, 1), slots: p.slots, held: p.held_slots, wakeups: p.wakeups });
  }
  if (p.metadata) return t("repair.costMetadata");
  return t("repair.costResets", { n: p.resets });
}

/** repairOutcome says how a run went: the lines the Cube accepted, that the devices take it at their next wakeup, the flags still set. */
function repairOutcome(res: RepairResult): string {
  let lines = 0;
  let accepted = 0;
  for (const st of res.steps || []) {
    lines += st.lines || 0;
    accepted += st.accepted || 0;
  }
  const parts = [t("repair.done", { accepted, lines })];
  if (res.confirmation === "pending") parts.push(t("result.structurePending"));
  if (res.still_flagged) {
    parts.push(res.still_flagged.length ? t("repair.stillFlagged", { devices: res.still_flagged.join(", ") }) : t("repair.cleared"));
  }
  return parts.join(" ");
}

/** runRepair sends a repair with its preview's plan, says how it went, and reads every preview of the box again. */
async function runRepair(box: Box, r: Repair, item: Item): Promise<void> {
  const plan = item.preview?.plan || "";
  const q = repairQuery(item);
  item.state = "running";
  const n = session();
  try {
    const res = await api<RepairResult>("POST", box.path + r.route + (q ? q + "&" : "?") + "plan=" + encodeURIComponent(plan));
    if (!stillSession(n)) return; // an answer after a sign-out says nothing and reads nothing again
    item.result = { text: repairOutcome(res), ok: true };
  } catch (e) {
    if (!stillSession(n)) return;
    item.result = { text: (e as Error).message, ok: false };
  }
  // Read again only where the scope still has a box: this one, or one
  // opened again during the run, which shows the run's outcome.
  const now = boxes.peek()[box.scope];
  if (!now) return;
  if (now !== box) (now.items[r.key] || (now.items[r.key] = newItem())).result = item.result;
  loadRepairs(now);
}

/**
 * RepairButton is the button that opens and closes the box of scope, for
 * a thing of kind at path; label names what it repairs, for its
 * accessible name, and skip and names are the box's (RepairOpts).
 */
export function RepairButton({ scope, kind, path, label, skip, names }: { scope: string; kind: RepairKind; path: string; label?: string } & RepairOpts) {
  useLang(); // its texts follow the page's language
  const open = !!boxes.value[scope];
  return (
    <button type="button" class="repair-open needs-repair" aria-label={label ? t("repair.openLabel", { what: label }) : undefined}
      aria-expanded={open ? "true" : "false"} onClick={() => toggleRepair(scope, kind, path, { skip, names })}>
      {t("repair.open")}
    </button>
  );
}

/** RepairBox is the box of scope's repairs while it is open, under what it repairs; nothing while it is closed. */
export function RepairBox({ scope }: { scope: string }) {
  useLang(); // its texts follow the page's language
  const box = boxes.value[scope];
  if (!box) return null;
  return (
    <div class="panel repair-box needs-repair" role="group">
      <div class="repair-head">
        <h3 class="repair-title">{t("repair.title")}</h3>
        <button type="button" class="repair-close" aria-label={t("repair.close")} onClick={() => toggleRepair(scope, box.kind, box.path)}>
          {t("action.close")}
        </button>
      </div>
      {repairsOf(box).map((r) => <RepairItem key={r.key} box={box} r={r} item={box.items[r.key] || newItem()} />)}
    </div>
  );
}

/**
 * RepairItem is one repair of a box: its name and what it does, the
 * outcome of its last run, then by its state that its preview is being
 * read, why it has nothing to do, the room's name it asks for, or its
 * preview with Run…; a listing of a device the list lacks asks for the
 * device's name above its preview. Cancel or Escape on a question puts
 * the focus back on Run….
 */
function RepairItem({ box, r, item }: { box: Box; r: Repair; item: Item }) {
  useLang(); // its texts follow the page's language
  const runRef = useRef<HTMLButtonElement>(null);
  const back = useRef(false);
  // a question taken back: the focus on Run… once it is drawn again
  useEffect(() => {
    if (item.ask !== "" || !back.current) return;
    back.current = false;
    runRef.current?.focus();
  }, [item.ask]);
  const cancel = () => {
    back.current = true;
    item.ask = "";
  };
  const p = item.preview;
  const naming = listing(r, p);
  return (
    <div class="repair-item" data-repair={r.key}>
      <h4 class="repair-name">{t("repair." + r.key)}</h4>
      <p class="muted small repair-help">{t("repair." + r.key + "Help")}</p>
      {item.result && <p class={"repair-result " + (item.result.ok ? "ok" : "bad")}>{item.result.text}</p>}
      {item.state === "loading" && <p class="muted small">{t("repair.loading")}</p>}
      {item.state === "none" && <p class="muted small repair-none">{item.error}</p>}
      {item.state === "name" && <RoomNameForm box={box} r={r} item={item} />}
      {(item.state === "ready" || item.state === "running") && p && (
        <>
          {naming && <DeviceNameForm box={box} r={r} item={item} />}
          {/* A name too long for the Cube has no preview of its own: nothing runs until it is shortened, which its field says (D201). */}
          {(!naming || nameFits(item.devName || "")) && <Preview box={box} r={r} item={item} p={p} runRef={runRef} cancel={cancel} />}
        </>
      )}
    </div>
  );
}

/**
 * Preview is a preview: its cost, its notes, the lines in expert mode, a
 * refusal for now, and Run…, or in its place the question that asks
 * before it sends, naming the cost; "unlink all and re-link" warns first.
 */
function Preview({ box, r, item, p, runRef, cancel }: {
  box: Box;
  r: Repair;
  item: Item;
  p: RepairPlan;
  runRef: { current: HTMLButtonElement | null };
  cancel: () => void;
}) {
  useLang(); // its texts follow the page's language
  const lines: string[] = [];
  for (const st of p.steps) {
    if (st.room) lines.push("z:…,G," + String(st.room).padStart(2, "0"));
    lines.push(...(st.lines || []));
  }
  const run = async () => {
    item.ask = "";
    await runRepair(box, r, item);
  };
  let actions;
  if (item.ask === "confirm") {
    actions = <Question text={t("repair.confirm", { what: t("repair." + r.key), cost: repairCost(p) })} yes={t("repair.runNow")} no={t("action.cancel")}
      onYes={run} onNo={cancel} />;
  } else if (item.ask === "warn") {
    actions = <Warning cost={repairCost(p)} onNo={cancel} onGo={() => {
      item.ask = "again";
    }} />;
  } else if (item.ask === "again") {
    actions = <Question text={t("repair.unlinkRelinkConfirm")} yes={t("repair.unlinkRelinkNow")} no={t("action.cancel")} onYes={run} onNo={cancel} />;
  } else {
    const busy = item.state === "running";
    actions = (
      <div class="actions repair-actions">
        <button ref={runRef} type="button" {...busyProps("repair-run", busy)} onClick={() => {
            item.ask = r.warn ? "warn" : "confirm";
          }}>{t("repair.run")}</button>
      </div>
    );
  }
  return (
    <>
      <p class="small repair-cost">{repairCost(p)}</p>
      {p.notes.map((n, i) => <p key={i} class="muted small repair-note">{n}</p>)}
      {lines.length > 0 && (
        <Fold cls="repair-lines expert-only" open={item.lines} onToggle={(o) => {
          item.lines = o;
        }} summary={t("repair.lines", { n: lines.length })}>
          <pre class="mono small">{lines.join("\n")}</pre>
        </Fold>
      )}
      {p.refused ? <p class="repair-refused bad">{t("repair.refusedNow", { reason: p.refused })}</p> : actions}
    </>
  );
}

/**
 * Warning warns in place of Run… before "unlink all and re-link": what
 * it costs, that it has not yet run on hardware, and what a run that
 * stops halfway leaves; Cancel and Escape take it back, Continue asks
 * once more, by its own words. The focus goes to Cancel when it comes.
 */
function Warning({ cost, onNo, onGo }: { cost: string; onNo: () => void; onGo: () => void }) {
  useLang(); // its texts follow the page's language
  const id = useId();
  const no = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    no.current?.focus();
  }, []);
  return (
    <div class="panel repair-warning" role="group" aria-labelledby={id} onKeyDown={(ev) => {
      if (ev.key !== "Escape") return;
      ev.preventDefault();
      ev.stopPropagation();
      onNo();
    }}>
      <p id={id} class="repair-warning-text">{t("repair.unlinkRelinkWarning", { cost })}</p>
      <div class="actions">
        <button ref={no} type="button" class="repair-warning-no" onClick={onNo}>{t("action.cancel")}</button>
        <button type="button" class="repair-warning-yes" onClick={onGo}>{t("repair.continue")}</button>
      </div>
    </div>
  );
}

/** RoomNameForm asks for the name of the room a listing makes, then reads the preview with it. */
function RoomNameForm({ box, r, item }: { box: Box; r: Repair; item: Item }) {
  useLang(); // its texts follow the page's language
  const id = useId();
  const value = item.roomTyped ?? item.roomName;
  return (
    <form class="repair-name-form" onSubmit={(ev) => {
      ev.preventDefault();
      const name = value.trim();
      if (!name) return;
      item.roomName = name;
      item.state = "loading";
      loadRepair(box, r, item);
    }}>
      <TextField label={t("repair.roomName", { room: item.room ?? "" })} id={id} name="room_name" required maxLength={32} value={value}
        set={(v) => {
          item.roomTyped = v;
        }} />
      <div class="actions">
        <button type="submit" class="primary">{t("repair.preview")}</button>
      </div>
    </form>
  );
}

/**
 * DeviceNameForm asks for the name a device the Cube's list lacks is
 * listed under, offering the default, and reads the preview again with
 * the name changed; an empty name is refused in place, and one longer
 * than NAME_BYTES, the default included, is marked on its field as it is
 * typed and not read: the submit goes back to the field.
 */
function DeviceNameForm({ box, r, item }: { box: Box; r: Repair; item: Item }) {
  useLang(); // its texts follow the page's language
  const id = useId();
  const field = useRef<HTMLInputElement>(null);
  const value = item.devTyped ?? item.devName ?? "";
  const fits = nameFits(value);
  return (
    <form class="repair-device-name-form" onSubmit={(ev) => {
      ev.preventDefault();
      const name = value.trim();
      if (!name) {
        item.note = { text: t("repair.nameRequired"), ok: false };
        return;
      }
      if (!fits) {
        field.current?.focus();
        return;
      }
      item.note = null;
      if (name === item.devName) return;
      item.devName = name;
      item.state = "loading";
      loadRepair(box, r, item);
    }}>
      <TextField label={t("repair.deviceName")} id={id} name="name" required value={value} inputRef={field}
        set={(v) => {
          item.devTyped = v;
        }}
        limit={{ text: t("name.long", { n: NAME_BYTES }), over: !fits, cls: "repair-name-long" }} />
      <ResultLine res={item.note} cls="small repair-name-note" role="status" />
      <div class="actions">
        <button type="submit" class="primary">{t("repair.preview")}</button>
      </div>
    </form>
  );
}

/**
 * forgetRepairs closes every box, forgetting its previews, its names, its
 * questions and its outcomes: at a sign-out and for another user's
 * sign-in after a 401 (MassimoParts.signedOut), so that the next user
 * finds nothing of the last one's. A run under way reads nothing more.
 */
export function forgetRepairs(): void {
  boxes.value = {};
}
