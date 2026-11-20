// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/rooms/Config.tsx
 *
 * A room's configuration in its tile, under the cog: its tabs, and the
 * tab shown with its Send and the result line of the last write. Room,
 * the name; Temperatures, the room's six temperatures; Devices, the
 * radiator settings and a form per device (ConfigDevices.tsx); Week
 * programme, the editor <massimo-week-program> (week-program.js), which
 * stays the element it is in a place the part leaves to it, its resets
 * beside Cancel and Send; Copy, this room's stored settings to other
 * rooms, a command, as the House panel is. Built once per room and updated in place, with the focus in it
 * or not, so that what is typed, a click and the focus stay when data
 * comes in; a field that shows stored data keeps a draft with its
 * "changed meanwhile" notice. What it keeps and sends is
 * configuration.ts.
 */

import { useRef, useState } from "preact/hooks";
import { ResultLine } from "../shared/badge";
import { Choices } from "../shared/choices";
import { FormFooter, TextField, numberNorm, useDraft, type Draft } from "../shared/field";
import { Island } from "../shared/island";
import { Tabs } from "../shared/tabs";
import { ConfigDevices, NumberRows, type NumberRow } from "./ConfigDevices";
import {
  changedFields,
  closeConfig,
  configOf,
  configWrite,
  placeOf,
  reloadProgram,
  sending,
  sendCopy,
  sendProgram,
  sendRoomConfig,
  sendRoomName,
  sourceName,
  stored,
  TEMP_FIELDS,
  type ConfigField,
  type ConfigState,
} from "./configuration";
import { load, tickOf } from "./store";

/** The configuration's tabs, by their keys, with the classes a capability hides them by. */
const TABS: { key: string; cls?: string }[] = [{ key: "room", cls: "needs-rename" }, { key: "temps" }, { key: "devices" }, { key: "program" }, { key: "copy" }];

const trim = (v: string) => v.trim();

/** RoomTab is the room's name, at most NAME_BYTES bytes, with its Send. */
function RoomTab({ room, cs }: { room: PageRoom; cs: ConfigState }) {
  load.useLoad();
  void tickOf(room.id).n;
  void cs.ver;
  const name = useDraft(room.name, trim, placeOf<string>(cs, "room", "name"));
  const changed = name.value.trim() !== room.name;
  const s = sending(cs, "room");
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (cs.sending) return;
    void configWrite(cs, "room", () => sendRoomName(room, cs, name.value));
  };
  return (
    <form class="cfg-form" onSubmit={submit}>
      <TextField label={t("cfg.roomName")} id={"cfg-" + room.id + "-name"} name="name" help="roomName" maxLength={NAME_BYTES} required draft={name} />
      <p class="muted small">{t("cfg.nameHint", { n: NAME_BYTES })}</p>
      <FormFooter cancel={() => closeConfig(room)} submit={t("action.send")} busy={s.busy} off={!changed || s.other} />
    </form>
  );
}

/** The Temperatures tab's fields, in their order. */
function tempRows(): NumberRow[] {
  return [
    { f: "comfort_temp", label: "cfg.comfort", help: "comfort", min: MIN_TEMP, max: MAX_TEMP, step: STEP },
    { f: "eco_temp", label: "cfg.eco", help: "eco", min: MIN_TEMP, max: MAX_TEMP, step: STEP },
    { f: "window_open_temp", label: "cfg.windowTemp", help: "windowTemp", min: MIN_TEMP, max: MAX_TEMP, step: STEP },
    { f: "window_open_minutes", label: "cfg.windowMinutes", help: "windowMinutes", min: 0, max: 60, step: 5, unit: "unit.minutes" },
    { f: "max_setpoint", label: "cfg.maxTemp", help: "maxTemp", min: MIN_TEMP, max: MAX_TEMP, step: STEP },
    { f: "min_setpoint", label: "cfg.minTemp", help: "minTemp", min: MIN_TEMP, max: MAX_TEMP, step: STEP },
  ];
}

/**
 * TempsTab is the room's temperatures, each a draft against the
 * configuration as loaded, where its values come from, and its Send,
 * which writes the fields that differ.
 */
function TempsTab({ room, cs }: { room: PageRoom; cs: ConfigState }) {
  load.useLoad();
  void cs.ver;
  void cs.cfg;
  const rows = tempRows();
  const drafts: Partial<Record<ConfigField, Draft<string>>> = {};
  // one draft per field, in the same order at every drawing
  drafts.comfort_temp = useDraft(stored(cs, "comfort_temp"), numberNorm, placeOf(cs, "temps", "comfort_temp"));
  drafts.eco_temp = useDraft(stored(cs, "eco_temp"), numberNorm, placeOf(cs, "temps", "eco_temp"));
  drafts.window_open_temp = useDraft(stored(cs, "window_open_temp"), numberNorm, placeOf(cs, "temps", "window_open_temp"));
  drafts.window_open_minutes = useDraft(stored(cs, "window_open_minutes"), numberNorm, placeOf(cs, "temps", "window_open_minutes"));
  drafts.max_setpoint = useDraft(stored(cs, "max_setpoint"), numberNorm, placeOf(cs, "temps", "max_setpoint"));
  drafts.min_setpoint = useDraft(stored(cs, "min_setpoint"), numberNorm, placeOf(cs, "temps", "min_setpoint"));
  const body = changedFields(cs, TEMP_FIELDS, drafts);
  const s = sending(cs, "temps");
  const src = cs.cfg?.source;
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (cs.sending) return;
    void configWrite(cs, "temps", () => sendRoomConfig(room, cs, body, t("cfg.tab.temps")));
  };
  return (
    <form class="cfg-form" onSubmit={submit}>
      <NumberRows room={room} rows={rows} drafts={drafts} />
      <p class="muted small">{src ? t("cfg.valuesFrom", { device: sourceName(cs.cfg, src) }) : t("cfg.noThermostat")}</p>
      <FormFooter cancel={() => closeConfig(room)} submit={t("action.send")} busy={s.busy} off={Object.keys(body).length === 0 || s.other} />
    </form>
  );
}

/**
 * ProgramTab is the week programme: the editor <massimo-week-program>
 * on the room's model, made once in an element the part leaves to it and
 * given the model again where a reload replaced it, its two resets
 * beside Cancel and Send; Send is on while a day differs from the
 * programme loaded, and from a key typed in the editor until its change
 * says whether it does.
 */
function ProgramTab({ room, cs }: { room: PageRoom; cs: ConfigState }) {
  load.useLoad();
  void cs.ver;
  const prog = cs.program;
  const p = prog?.model || null;
  const [typed, setTyped] = useState(false);
  const [, redraw] = useState(0);
  const editor = useRef<MassimoWeekProgram | null>(null);
  const storedOn = sourceName(cs.cfg, p?.source);
  const dirty = p ? MassimoWeekProgram.dirtyDays(p).length > 0 : false;
  const s = sending(cs, "program");
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (cs.sending || !prog) return;
    void configWrite(cs, "program", () => sendProgram(room, cs, prog)).then(() => {
      editor.current?.render(); // the days taken are no longer marked changed
      redraw((n) => n + 1);
    });
  };
  return (
    <form class="cfg-form" onSubmit={submit} onInput={() => setTyped(true)} onChange={() => {
      // a change in the editor: what differs is known again
      setTyped(false);
      redraw((n) => n + 1);
    }}>
      <Island cls="prog-slot" keys={[p, storedOn]} fill={(el) => {
        let ed = editor.current;
        if (!ed) {
          ed = document.createElement("massimo-week-program") as MassimoWeekProgram;
          ed.kit = weekKit();
          editor.current = ed;
        }
        ed.model = p;
        ed.storedOn = storedOn;
        if (ed.parentElement !== el) el.replaceChildren(ed); // connected, it draws itself
        else ed.render();
      }} />
      <FormFooter cancel={() => closeConfig(room)} submit={t("action.send")} busy={s.busy} off={!(dirty || typed) || s.other}
        extra={<Island tag="span" cls="prog-resets" keys={[p]} fill={(el) => el.replaceChildren(...(editor.current?.resetControls() || []))} />} />
    </form>
  );
}

/**
 * CopyTab copies this room's stored settings to other rooms, a command
 * and no settings form, as the House panel is: the rooms with a
 * thermostat to choose, with All rooms and None, and which of the
 * settings, kept in the configuration's state while it is open and
 * across a close; its button is on while a room and a setting are
 * chosen, and a copy that went through clears the rooms chosen, the
 * settings as the user set them.
 */
function CopyTab({ room, cs }: { room: PageRoom; cs: ConfigState }) {
  load.useLoad();
  const others = state.rooms.filter((r) => r.id !== room.id && roomThermostat(r));
  const ids = cs.copyRooms.map(Number).filter((id) => others.some((r) => r.id === id));
  const what = cs.copyWhat;
  const k = { rooms: ids, temps: what.includes("temps"), program: what.includes("program"), radiator: what.includes("radiator") };
  const can = ids.length > 0 && (k.temps || k.radiator || (k.program && !!cs.program));
  const s = sending(cs, "copy");
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (cs.sending || !can) return;
    void configWrite(cs, "copy", () => sendCopy(cs, k)).then((ok) => {
      if (ok) cs.copyRooms = [];
    });
  };
  return (
    <form class="cfg-form" onSubmit={submit}>
      <p class="muted small">{t("copy.intro")}</p>
      <Choices title={t("copy.rooms")} label={t("copy.roomsAria")} options={others.map((r) => [String(r.id), r.name])} chosen={ids.map(String)}
        set={(chosen) => {
          cs.copyRooms = chosen;
        }} all={{ all: t("copy.allRooms"), none: t("copy.none") }} empty={t("copy.noOther")} />
      <Choices title={t("copy.settings")} label={t("copy.settingsAria")} chosen={what} set={(chosen) => {
        cs.copyWhat = chosen;
      }} options={[["temps", t("copy.temps")], ["program", t("copy.program"), !cs.program], ["radiator", t("copy.radiator")]]} />
      <FormFooter cancel={() => closeConfig(room)} submit={t("copy.send")} busy={s.busy} off={!can || s.other} />
    </form>
  );
}

/**
 * Body is the open configuration: its tabs and the close, then the tab
 * shown, or that it loads, or why it could not be loaded with Close; the
 * result line of the last write under the tab, with Load the current
 * programme where the programme changed elsewhere.
 */
function Body({ room }: { room: PageRoom }) {
  load.useLoad();
  void tickOf(room.id).n;
  const cs = configOf(room.id);
  const tab = cs.tab;
  const res = cs.result;
  return (
    <>
      <Tabs cls="config-tabs" listCls="config-tablist" label={t("cfg.sections")} current={tab}
        tabs={TABS.map((tb) => ({ ...tb, label: t("cfg.tab." + tb.key) }))} onPick={(key) => {
          cs.tab = key;
          cs.result = null;
        }}>
        <button type="button" class="tab close" aria-label={t("cfg.close")} title={t("cfg.closeTitle")} onClick={() => closeConfig(room)}>×</button>
      </Tabs>
      <div class="config-body">
        {cs.loading && <p class="muted">{t("cfg.loading")}</p>}
        {!cs.loading && cs.error && (
          <>
            <p class="tile-result bad">{t("cfg.cannotLoad", { error: cs.error })}</p>
            <button type="button" onClick={() => closeConfig(room)}>{t("action.close")}</button>
          </>
        )}
        {!cs.loading && !cs.error && (
          <>
            {tab === "room" && <RoomTab room={room} cs={cs} />}
            {tab === "temps" && <TempsTab room={room} cs={cs} />}
            {tab === "devices" && <ConfigDevices room={room} cs={cs} />}
            {tab === "program" && <ProgramTab room={room} cs={cs} />}
            {tab === "copy" && <CopyTab room={room} cs={cs} />}
            <ResultLine res={res} cls="tile-result" />
            {res?.reload && <button type="button" class="prog-reload" onClick={() => reloadProgram(room, cs)}>{t("prog.reload")}</button>}
          </>
        )}
      </div>
    </>
  );
}

/** Config is the tile's configuration area: hidden and empty while closed, else its tabs and the tab shown. */
export function Config({ room, ui }: { room: PageRoom; ui: RoomUI }) {
  load.useLoad();
  const open = ui.config;
  return <div class={open ? "config-area" : "config-area hidden"}>{open && <Body room={room} />}</div>;
}
