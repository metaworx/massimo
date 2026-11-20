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
 * rooms. Built once per room and updated in place, with the focus in it
 * or not, so that what is typed, a click and the focus stay when data
 * comes in; a field that shows stored data keeps a draft with its
 * "changed meanwhile" notice. Each tab but Copy is a settings form of its own
 * (useConfigForm), which keeps its edit when the tab or the
 * configuration closes, and whose Save says its state, with Reset beside
 * it and Cancel, which resets and closes; the week programme keeps its own
 * resets. The Copy tab is a command, as the House panel is, and no
 * settings form. What it keeps and sends is configuration.ts.
 */

import { useRef, useState } from "preact/hooks";
import type { Draft } from "../shared/field";
import { ResultLine } from "../shared/badge";
import { Choices } from "../shared/choices";
import { FormFooter, TextField, numberNorm, useDraft } from "../shared/field";
import { Island } from "../shared/island";
import { NAME_BYTES } from "../shared/name";
import { Tabs } from "../shared/tabs";
import { ConfigDevices, NumberRows, type NumberRow } from "./ConfigDevices";
import {
  cancelOf,
  changedFields,
  closeConfig,
  configOf,
  configWrite,
  reloadProgram,
  sending,
  sendCopy,
  sendProgram,
  sendRoomConfig,
  sendRoomName,
  sourceName,
  stored,
  TEMP_FIELDS,
  useConfigForm,
  type ConfigField,
  type ConfigState,
  type Programme,
} from "./configuration";
import { load, tickOf } from "./store";

/** The configuration's tabs, by their keys, with the classes a capability hides them by. */
const TABS: { key: string; cls?: string }[] = [{ key: "room", cls: "needs-rename" }, { key: "temps" }, { key: "devices" }, { key: "program" }, { key: "copy" }];

const trim = (v: string) => v.trim();

/** RoomTab is the room's name, at most NAME_BYTES bytes, with its Send. */
function RoomTab({ room, cs }: { room: PageRoom; cs: ConfigState }) {
  load.useLoad();
  void tickOf(room.id).n;
  const form = useConfigForm(room, "room", "room", { key: "cfg.form.room", args: { room: room.name } });
  const name = useDraft(room.name, trim, form.place<string>("name"));
  const changed = name.value.trim() !== room.name;
  form.track(changed, [name]);
  const s = sending(cs, "room");
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (cs.sending || !changed) return;
    form.save(() => configWrite(cs, "room", () => sendRoomName(room, cs, name.value)));
  };
  return (
    <form {...form.props} class="cfg-form" onSubmit={submit}>
      <TextField label={t("cfg.roomName")} id={"cfg-" + room.id + "-name"} name="name" help="roomName" maxLength={NAME_BYTES} required draft={name} />
      <p class="muted small">{t("cfg.nameHint", { n: NAME_BYTES })}</p>
      <FormFooter cancel={cancelOf(form, room)} reset={form.reset} look={form.look} submit={t("action.send")} busy={s.busy} off={!changed || s.other} />
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
  void cs.cfg;
  const form = useConfigForm(room, "temps", "temps", { key: "cfg.form.temps", args: { room: room.name } });
  const rows = tempRows();
  const drafts: Partial<Record<ConfigField, Draft<string>>> = {};
  // one draft per field, in the same order at every drawing
  drafts.comfort_temp = useDraft(stored(cs, "comfort_temp"), numberNorm, form.place("comfort_temp"));
  drafts.eco_temp = useDraft(stored(cs, "eco_temp"), numberNorm, form.place("eco_temp"));
  drafts.window_open_temp = useDraft(stored(cs, "window_open_temp"), numberNorm, form.place("window_open_temp"));
  drafts.window_open_minutes = useDraft(stored(cs, "window_open_minutes"), numberNorm, form.place("window_open_minutes"));
  drafts.max_setpoint = useDraft(stored(cs, "max_setpoint"), numberNorm, form.place("max_setpoint"));
  drafts.min_setpoint = useDraft(stored(cs, "min_setpoint"), numberNorm, form.place("min_setpoint"));
  const body = changedFields(cs, TEMP_FIELDS, drafts);
  const changed = Object.keys(body).length > 0;
  form.track(changed, Object.values(drafts));
  const s = sending(cs, "temps");
  const src = cs.cfg?.source;
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (cs.sending || !changed) return;
    form.save(() => configWrite(cs, "temps", () => sendRoomConfig(room, cs, body, t("cfg.tab.temps"))));
  };
  return (
    <form {...form.props} class="cfg-form" onSubmit={submit}>
      <NumberRows room={room} rows={rows} drafts={drafts} />
      <p class="muted small">{src ? t("cfg.valuesFrom", { device: sourceName(cs.cfg, src) }) : t("cfg.noThermostat")}</p>
      <FormFooter cancel={cancelOf(form, room)} reset={form.reset} look={form.look} submit={t("action.send")} busy={s.busy} off={!changed || s.other} />
    </form>
  );
}

/**
 * kept is the programme the tab edits, kept in its form's place: the one
 * loaded the first time, and a programme loaded later where nothing of
 * the kept one differs from what it was loaded as, or where the user
 * loaded it again (replace); otherwise the edits stay, with the tag they
 * were loaded with, which a write then names.
 */
function kept(slot: { base: Programme | null; value: Programme | null; stored: Programme | null }, loaded: Programme | null): Programme | null {
  if (slot.stored !== loaded) {
    const edited = slot.value;
    if (loaded && (loaded.replace || !edited || MassimoWeekProgram.dirtyDays(edited.model).length === 0)) {
      slot.value = loaded;
      slot.base = loaded;
    }
    slot.stored = loaded;
  }
  return slot.value;
}

/**
 * ProgramTab is the week programme: the editor <massimo-week-program>
 * on the programme kept (kept), made once in an element the part leaves
 * to it and given the model again where another replaced it, its two
 * resets beside Cancel and Send; Send is on while a day differs from the
 * programme loaded, and from a key typed in the editor until its change
 * says whether it does. Its form's Cancel puts every day back as loaded.
 */
function ProgramTab({ room, cs }: { room: PageRoom; cs: ConfigState }) {
  load.useLoad();
  const form = useConfigForm(room, "program", "program", { key: "cfg.form.program", args: { room: room.name } });
  const [typed, setTyped] = useState(false);
  const [, redraw] = useState(0);
  const editor = useRef<MassimoWeekProgram | null>(null);
  const at = form.place<Programme | null>("program");
  const prog = kept(at.slot(cs.program), cs.program);
  const p = prog?.model || null;
  const storedOn = sourceName(cs.cfg, p?.source);
  const dirty = p ? MassimoWeekProgram.dirtyDays(p).length > 0 : false;
  const changed = dirty || typed;
  form.track(changed, [{
    take() {
      if (!p) return;
      p.days = MassimoWeekProgram.copyDays(p.orig);
      p.copyTo = new Set();
      MassimoWeekProgram.selectDay(p, p.day);
      editor.current?.render(); // the days put back, no longer marked changed
    },
  }]);
  const s = sending(cs, "program");
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (cs.sending || !prog || !changed) return;
    form.save(() => configWrite(cs, "program", () => sendProgram(room, cs, prog)).then((ok) => {
      editor.current?.render(); // the days taken are no longer marked changed
      redraw((n) => n + 1);
      return ok;
    }));
  };
  return (
    <form {...form.props} class="cfg-form" onSubmit={submit} onInput={() => {
      setTyped(true);
      at.changed(); // the user's edit: the form's minute starts again, an unsaved form is changed again
    }} onChange={() => {
      // a change in the editor: what differs is known again
      setTyped(false);
      at.changed();
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
      <FormFooter cancel={cancelOf(form, room)} look={form.look} submit={t("action.send")} busy={s.busy} off={!changed || s.other}
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
