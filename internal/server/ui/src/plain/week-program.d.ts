// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/plain/week-program.d.ts
 *
 * The globals of the plain script ui/week-program.js that the parts call:
 * the week programme editor, <massimo-week-program>, which stays the
 * element it is (a Home Assistant card loads it by itself), its model
 * and what tells a model's changed days, checks a day and copies its
 * switch points.
 */

declare global {
  /** A switch point of a day: the temperature until a time, "HH:MM". */
  interface WeekPoint {
    temp: number;
    until: string;
  }
  /**
   * The editor's model of a room's week programme (MassimoWeekProgram.model):
   * an editable copy per day, the loaded one (orig), the device it was
   * read from, the day edited, the days selected with it, "only" and the
   * days ticked to copy to.
   */
  interface WeekModel {
    days: Record<string, WeekPoint[]>;
    orig: Record<string, WeekPoint[]>;
    source: string;
    day: string;
    selected: Set<string>;
    only: boolean;
    copyTo: Set<string>;
  }
  /** What the editor builds with: the page's translation, its plain builders and temperatures (config.js weekKit). */
  interface WeekKit {
    t: (key: string, values?: Record<string, string | number>) => string;
  }
  /** The week programme editor, <massimo-week-program>. */
  class MassimoWeekProgram extends HTMLElement {
    /** The weekdays, "monday" to "sunday". */
    static DAYS: string[];
    /** The model of an API's week programme; null gives every day 17 °C. */
    static model(prog: unknown): WeekModel;
    /** A day's switch points copied. */
    static copyPoints(pts: WeekPoint[]): WeekPoint[];
    /** Every day's switch points copied. */
    static copyDays(days: Record<string, WeekPoint[]>): Record<string, WeekPoint[]>;
    /** The days of the model that differ from the loaded programme. */
    static dirtyDays(p: WeekModel): string[];
    /** Puts a day's switch points in the order of their times. */
    static sortPoints(pts: WeekPoint[]): void;
    /** What is wrong with a day's switch points, "" where nothing is. */
    static validateDay(pts: WeekPoint[], t: (key: string, values?: Record<string, string | number>) => string): string;
    /** Edits a day ("monday" …) and selects every day with its programme. */
    static selectDay(p: WeekModel, day: string): void;
    /** The model edited, the host's. */
    model: WeekModel | null;
    /** What the editor builds with. */
    kit: WeekKit | null;
    /** The device the programme was read from, by name. */
    storedOn: string;
    /** Draws the editor. */
    render(): void;
    /** The two resets, "Reset day" and "Reset all", for the host to place beside its Cancel and Send. */
    resetControls(): HTMLButtonElement[];
  }
}

export {};
