// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/action.ts
 *
 * Running an action and saying how it went: a request, then a reload of
 * what it changed, then its outcome in the region's message line
 * (showResult of components.js), green, or red with the error. A region
 * makes one runner and keeps its own requests beside it; the button that
 * asked turns meanwhile with useBusy (question.tsx).
 */

/** A region's runner of actions (actionRunner). */
export interface ActionRunner {
  /** say shows text in the message line, ok green, else red; no text hides it. */
  say(text: string, ok?: boolean): void;
  /**
   * run sends fn's request, reloads, and says ok, or what fail makes of
   * the error's message; it resolves to whether the request was taken.
   */
  run(fn: () => Promise<unknown>, ok: string, fail: (error: string) => string): Promise<boolean>;
}

/**
 * actionRunner makes a region's runner: where gives the message line at
 * the time of saying (a region shown in two places has two), reload what
 * follows a request taken.
 */
export function actionRunner({ where, reload }: { where: () => HTMLElement | null; reload?: () => Promise<unknown> }): ActionRunner {
  const say = (text: string, ok?: boolean) => {
    const el = where();
    if (el) showResult(el, text, ok);
  };
  return {
    say,
    async run(fn, ok, fail) {
      try {
        await fn();
        if (reload) await reload();
        say(ok, true);
        return true;
      } catch (e) {
        say(fail((e as Error).message), false);
        return false;
      }
    },
  };
}
