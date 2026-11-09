// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// <massimo-week-program>: the week programme editor of a room, as the
// web UI's configuration area shows it and as a Home Assistant card can
// use it (AP HomeAssistant v1.0, Block 3; docs/ui.md, "Week programme").
//
// The element owns the editor's behaviour: the day being edited, the
// days edited together, "only this day", the switch points, the copy to
// other days, the resets and what differs from the loaded programme.
// It owns no state: its model (MassimoWeekProgram.model) is the host's,
// so that the edits outlive a redraw of the page around it. Its controls
// come from the host's kit, so that it looks as the page it is in:
//
//   kit = { t, elem, textEl, button, switchEl, fieldRow, numberInput,
//           fmtNum, fmtTemp, clamp, temps: { min, max, step } }
//
// t(key, values) is the page's translation, the keys those of prog.*,
// day.* and dayShort.*; the others build elements as components.js does.
// After every edit the element draws itself again and dispatches a
// bubbling "change" event; the host's Send asks dirtyDays().
"use strict";

class MassimoWeekProgram extends HTMLElement {
  static DAYS = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"];
  static MAX_POINTS = 13;

  // model turns the API's week programme into the editor's model: an
  // editable copy per day and the loaded one (orig) for the resets and
  // for telling the changed days, the day being edited, the selected
  // days (those with the edited day's programme, edited together unless
  // "only" is on), and the days ticked in the copy section.
  static model(prog) {
    const W = MassimoWeekProgram;
    const days = {};
    for (const day of W.DAYS) {
      const pts = prog && Array.isArray(prog[day]) ? prog[day] : [];
      days[day] = pts.map((p) => ({ temp: p.temp, until: p.until }));
      if (days[day].length === 0) days[day] = [{ temp: 17, until: "24:00" }];
    }
    const p = { days, orig: W.copyDays(days), source: prog ? prog.source : "", day: "monday", selected: new Set(), only: false, copyTo: new Set() };
    W.selectDay(p, "monday");
    return p;
  }

  static copyPoints(pts) {
    return pts.map((pt) => ({ temp: pt.temp, until: pt.until }));
  }

  static copyDays(days) {
    const out = {};
    for (const day of MassimoWeekProgram.DAYS) out[day] = MassimoWeekProgram.copyPoints(days[day]);
    return out;
  }

  static samePoints(a, b) {
    return a.length === b.length && a.every((pt, i) => pt.temp === b[i].temp && pt.until === b[i].until);
  }

  // dirtyDays lists the days whose programme differs from the loaded one.
  static dirtyDays(p) {
    return MassimoWeekProgram.DAYS.filter((d) => !MassimoWeekProgram.samePoints(p.days[d], p.orig[d]));
  }

  // selectDay makes day the edited day and selects every day that has
  // the same programme.
  static selectDay(p, day) {
    const W = MassimoWeekProgram;
    p.day = day;
    p.selected = new Set(W.DAYS.filter((d) => W.samePoints(p.days[d], p.days[day])));
    for (const d of p.selected) p.copyTo.delete(d);
  }

  // programEdited is called after every change to the edited day: with
  // "only" on, the change sets the day apart and the other days leave the
  // selection; with it off, the change goes to every selected day.
  static programEdited(p) {
    if (p.only) {
      p.selected = new Set([p.day]);
      return;
    }
    for (const d of p.selected) if (d !== p.day) p.days[d] = MassimoWeekProgram.copyPoints(p.days[p.day]);
  }

  static minutesOf(until) {
    const m = /^(\d{1,2}):(\d{2})$/.exec(until || "");
    return m ? Number(m[1]) * 60 + Number(m[2]) : 0;
  }

  static clockOf(minutes) {
    const pad = (n) => String(n).padStart(2, "0");
    return pad(Math.floor(minutes / 60)) + ":" + pad(minutes % 60);
  }

  // sortPoints puts a day's switch points in the order of their times.
  static sortPoints(pts) {
    pts.sort((a, b) => MassimoWeekProgram.minutesOf(a.until) - MassimoWeekProgram.minutesOf(b.until));
  }

  // validateDay says what is wrong with a day's switch points, "" when
  // nothing is, in the language t gives.
  static validateDay(pts, t) {
    const W = MassimoWeekProgram;
    if (pts.length < 1 || pts.length > W.MAX_POINTS) return t("prog.errPoints", { n: W.MAX_POINTS });
    let prev = 0;
    for (let i = 0; i < pts.length; i++) {
      const m = W.minutesOf(pts[i].until);
      if (i < pts.length - 1 && m <= prev) return t("prog.errTimes");
      prev = m;
    }
    if (pts[pts.length - 1].until !== "24:00") return t("prog.errEnd");
    return "";
  }

  constructor() {
    super();
    this.model = null; // MassimoWeekProgram.model(...), the host's
    this.kit = null;
    this.storedOn = ""; // the device the programme was read from, by name
    this.resetDayBtn = null;
    this.resetAllBtn = null;
  }

  connectedCallback() {
    this.render();
  }

  dayShort(day) {
    return this.kit.t("dayShort." + day);
  }

  dayLong(day) {
    return this.kit.t("day." + day);
  }

  dayList(days) {
    return days.map((d) => this.dayShort(d)).join(", ");
  }

  // dirtyDays lists the edited model's days that differ from the loaded.
  dirtyDays() {
    return this.model ? MassimoWeekProgram.dirtyDays(this.model) : [];
  }

  // changed draws the editor again after an edit and tells the host.
  changed() {
    this.render();
    this.dispatchEvent(new Event("change", { bubbles: true }));
  }

  // resetControls returns the two resets, "Reset day" and "Reset all",
  // for the host to place beside its Cancel and Send; the editor keeps
  // them enabled while there is something to reset.
  resetControls() {
    const W = MassimoWeekProgram;
    const k = this.kit;
    const p = this.model;
    this.resetDayBtn = k.button(k.t("prog.resetDay"), { cls: "prog-reset-day", title: k.t("prog.resetDayTitle", { day: this.dayLong(p.day) }) }, () => {
      p.days[p.day] = W.copyPoints(p.orig[p.day]);
      W.selectDay(p, p.day);
      this.changed();
    });
    this.resetAllBtn = k.button(k.t("prog.resetAll"), { cls: "prog-reset-all", title: k.t("prog.resetAllTitle") }, () => {
      p.days = W.copyDays(p.orig);
      p.copyTo = new Set();
      W.selectDay(p, p.day);
      this.changed();
    });
    this.mark();
    return [this.resetDayBtn, this.resetAllBtn];
  }

  // render draws the editor: the day buttons (a click edits that day and
  // selects every day with the same programme), the "Only {day}" switch,
  // the day as a bar and as rows of switch points, the copy section and
  // the note. The rows are drawn in the order of their times; a time
  // typed into one is marked in place (mark), not drawn again.
  render() {
    const W = MassimoWeekProgram;
    const p = this.model;
    const k = this.kit;
    if (!p || !k) return;
    W.sortPoints(p.days[p.day]);
    const days = k.elem("div", "day-bar week prog-days");
    days.setAttribute("role", "group");
    days.setAttribute("aria-label", k.t("prog.dayToEdit"));
    for (const day of W.DAYS) {
      const b = k.button(this.dayShort(day), { title: k.t("prog.dayTitle", { day: this.dayLong(day) }) }, () => {
        W.selectDay(p, day);
        this.changed();
      });
      b.dataset.day = day;
      days.appendChild(b);
    }
    const only = k.switchEl(k.t("prog.only", { day: this.dayLong(p.day) }), p.only, (on) => {
      p.only = on;
      this.changed();
    });
    const together = W.DAYS.filter((d) => p.selected.has(d));
    const scope = p.only || together.length < 2
      ? k.t("prog.appliesOnly", { day: this.dayLong(p.day) })
      : k.t("prog.appliesTogether", { days: this.dayList(together) });
    const note = k.textEl("p", "", "muted small prog-note");
    note.dataset.stored = this.storedOn ? k.t("prog.storedOn", { device: this.storedOn }) + " " : "";
    this.replaceChildren(
      days,
      k.fieldRow(null, only, { help: "onlyDay" }),
      k.textEl("p", scope, "muted small"),
      this.dayBar(p.days[p.day]),
      this.points(p.day),
      this.dayCopy(),
      note,
    );
    if (this.resetDayBtn) this.resetDayBtn.title = k.t("prog.resetDayTitle", { day: this.dayLong(p.day) });
    this.mark();
  }

  // mark shows which days differ from the loaded programme (on the day
  // buttons, in the note, on the resets) and which are edited together,
  // and draws the edited day's bar.
  mark() {
    const W = MassimoWeekProgram;
    const p = this.model;
    const k = this.kit;
    if (!p || !k) return;
    const dirty = new Set(W.dirtyDays(p));
    for (const b of this.querySelectorAll(".prog-days button")) {
      const day = b.dataset.day;
      const editing = p.day === day;
      const sel = p.selected.has(day);
      let name = this.dayLong(day);
      if (sel && !editing) name = k.t("prog.editedTogether", { day: name });
      if (dirty.has(day)) name = k.t("prog.changed", { day: name });
      b.className = "day" + (sel ? " selected" : "") + (editing ? " editing" : "") + (dirty.has(day) ? " dirty" : "");
      b.setAttribute("aria-label", name);
      b.setAttribute("aria-pressed", editing ? "true" : "false");
    }
    const bar = this.querySelector(".prog-bar");
    if (bar) {
      const sorted = W.copyPoints(p.days[p.day]);
      W.sortPoints(sorted);
      bar.replaceWith(this.dayBar(sorted));
    }
    const note = this.querySelector(".prog-note");
    if (note) note.textContent = note.dataset.stored + (dirty.size ? k.t("prog.sendNoteDays", { days: this.dayList([...dirty]) }) : k.t("prog.sendNote"));
    if (this.resetDayBtn) this.resetDayBtn.disabled = !dirty.has(p.day);
    if (this.resetAllBtn) this.resetAllBtn.disabled = dirty.size === 0;
  }

  // dayCopy is the copy section: every day as a toggle, the days already
  // sharing the edited day's programme greyed out, "All days", and Copy,
  // which gives the ticked days the edited day's programme. With "only"
  // off they then join the selection and are edited together.
  dayCopy() {
    const W = MassimoWeekProgram;
    const p = this.model;
    const k = this.kit;
    const title = k.t("prog.copyTo", { day: this.dayLong(p.day) });
    const row = k.elem("div", "day-bar week");
    const free = W.DAYS.filter((d) => !p.selected.has(d));
    for (const day of W.DAYS) {
      const on = p.copyTo.has(day);
      const has = p.selected.has(day);
      const b = k.button(this.dayShort(day), { cls: "day" + (on ? " chosen" : ""), aria: has ? k.t("prog.hasProgramme", { day: this.dayLong(day) }) : this.dayLong(day) }, () => {
        if (p.copyTo.has(day)) p.copyTo.delete(day);
        else p.copyTo.add(day);
        this.changed();
      });
      b.disabled = has;
      b.setAttribute("aria-pressed", on ? "true" : "false");
      row.appendChild(b);
    }
    const all = k.button(k.t("prog.allDays"), "day all", () => {
      for (const d of free) p.copyTo.add(d);
      this.changed();
    });
    all.disabled = free.length === 0 || free.every((d) => p.copyTo.has(d));
    const copy = k.button(k.t("prog.copy"), "day copy", () => {
      for (const d of p.copyTo) {
        p.days[d] = W.copyPoints(p.days[p.day]);
        if (!p.only) p.selected.add(d);
      }
      p.copyTo = new Set();
      this.changed();
    });
    copy.disabled = p.copyTo.size === 0;
    const box = k.elem("div", "day-copy", k.textEl("h4", title), row, k.elem("div", "day-bar", all, copy));
    box.setAttribute("role", "group");
    box.setAttribute("aria-label", title);
    return box;
  }

  // dayBar draws the day as segments proportional to their length.
  dayBar(points) {
    const W = MassimoWeekProgram;
    const k = this.kit;
    const bar = k.elem("div", "prog-bar");
    let start = 0;
    const temps = points.map((p) => p.temp);
    const lo = Math.min(...temps), hi = Math.max(...temps);
    for (const pt of points) {
      const end = W.minutesOf(pt.until);
      // A label only when the segment is wide enough to hold it.
      const seg = k.textEl("span", end - start >= 120 ? k.fmtNum(pt.temp, 1) + "°" : "", "prog-seg");
      seg.style.flexGrow = String(Math.max(1, end - start));
      const warm = hi > lo ? (pt.temp - lo) / (hi - lo) : 0.5;
      seg.style.setProperty("--warm", warm.toFixed(2));
      seg.title = W.clockOf(start) + " – " + pt.until + ": " + k.fmtTemp(pt.temp);
      bar.appendChild(seg);
      start = end;
    }
    return bar;
  }

  // points lists the switch points of a day as editable rows; every
  // change goes through programEdited.
  points(day) {
    const W = MassimoWeekProgram;
    const p = this.model;
    const k = this.kit;
    const pts = p.days[day];
    const edited = () => {
      W.programEdited(p);
      this.changed();
    };
    const list = k.elem("div", "prog-points");
    pts.forEach((pt, i) => {
      const last = i === pts.length - 1;
      const row = k.elem("div", "prog-point", k.textEl("span", k.t("prog.until"), "muted"));
      if (last) {
        row.appendChild(k.textEl("span", "24:00", "prog-end"));
      } else {
        const time = document.createElement("input");
        time.type = "time";
        time.step = "300";
        time.value = pt.until;
        time.required = true;
        time.setAttribute("aria-label", k.t("prog.pointUntil", { n: i + 1 }));
        // Chrome fires change at each part of a time typed: the value is
        // kept and marked in place, without a redraw, which would replace
        // the input in the middle of the time, or, once it is left, the
        // button being clicked; the rows take the order of their times at
        // the next draw, and the host sorts them before it sends.
        time.addEventListener("change", () => {
          if (!time.value) return;
          pt.until = time.value;
          W.programEdited(p);
          this.mark();
        });
        row.appendChild(time);
      }
      row.appendChild(k.textEl("span", k.t("prog.at"), "muted"));
      const temp = k.numberInput(pt.temp, k.temps.min, k.temps.max, k.temps.step, (v) => {
        if (v === undefined) return;
        pt.temp = k.clamp(v);
        edited();
      });
      temp.setAttribute("aria-label", k.t("prog.pointTemp", { n: i + 1 }));
      row.appendChild(temp);
      row.appendChild(k.textEl("span", "°C", "muted"));
      if (!last) {
        const rm = k.button("−", { cls: "step small", aria: k.t("prog.remove"), title: k.t("prog.removeTitle") }, () => {
          pts.splice(i, 1);
          edited();
        });
        row.appendChild(rm);
      } else if (pts.length < W.MAX_POINTS) {
        const add = k.button("+", { cls: "step small", aria: k.t("prog.add"), title: k.t("prog.addTitle") }, () => {
          const prev = pts.length > 1 ? W.minutesOf(pts[pts.length - 2].until) : 0;
          const mid = Math.round(((prev + 1440) / 2) / 5) * 5;
          pts.splice(pts.length - 1, 0, { temp: pt.temp, until: W.clockOf(Math.min(1435, Math.max(prev + 5, mid))) });
          edited();
        });
        row.appendChild(add);
      }
      list.appendChild(row);
    });
    return list;
  }
}

customElements.define("massimo-week-program", MassimoWeekProgram);
