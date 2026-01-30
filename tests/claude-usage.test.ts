import { describe, test, expect } from "bun:test";
import {
  SLEEP_START_HOUR,
  WAKE_HOUR,
  SLEEP_HOURS_PER_DAY,
  ACTIVE_HOURS_PER_DAY,
  offsetInCycle,
  getCycleStart,
  activeHoursInPartialCycle,
  elapsedActiveHoursBetween,
} from "../home/dot_local/bin/executable_claude-usage";

/** Helper: create a Date for a specific day/hour/minute */
function makeDate(
  year: number,
  month: number,
  day: number,
  hour: number,
  minute: number = 0,
  second: number = 0
): Date {
  return new Date(year, month - 1, day, hour, minute, second);
}

describe("constants", () => {
  test("active + sleep = 24 hours", () => {
    expect(ACTIVE_HOURS_PER_DAY + SLEEP_HOURS_PER_DAY).toBe(24);
  });

  test("wake hour = sleep start + sleep duration", () => {
    expect(WAKE_HOUR).toBe(SLEEP_START_HOUR + SLEEP_HOURS_PER_DAY);
  });
});

describe("offsetInCycle", () => {
  test("2:00 AM is offset 0", () => {
    expect(offsetInCycle(makeDate(2026, 1, 15, 2, 0))).toBe(0);
  });

  test("9:00 AM is offset 7 (sleep duration)", () => {
    expect(offsetInCycle(makeDate(2026, 1, 15, 9, 0))).toBe(SLEEP_HOURS_PER_DAY);
  });

  test("midnight is offset 22", () => {
    expect(offsetInCycle(makeDate(2026, 1, 15, 0, 0))).toBe(22);
  });

  test("1:59 AM is offset 23 + 59/60", () => {
    const offset = offsetInCycle(makeDate(2026, 1, 15, 1, 59));
    expect(offset).toBeCloseTo(23 + 59 / 60, 10);
  });

  test("offset is monotonically increasing from 2 AM through to next 1:59 AM", () => {
    let prev = -1;
    // Walk minute by minute from 2:00 AM to 1:59 AM next day
    for (let minuteOffset = 0; minuteOffset < 24 * 60; minuteOffset++) {
      const d = new Date(makeDate(2026, 1, 15, 2, 0).getTime() + minuteOffset * 60_000);
      const offset = offsetInCycle(d);
      expect(offset).toBeGreaterThan(prev);
      prev = offset;
    }
  });
});

describe("getCycleStart", () => {
  test("at 3 AM, cycle start is 2 AM same day", () => {
    const result = getCycleStart(makeDate(2026, 1, 15, 3, 0));
    expect(result.getHours()).toBe(2);
    expect(result.getDate()).toBe(15);
  });

  test("at 1 AM, cycle start is 2 AM previous day", () => {
    const result = getCycleStart(makeDate(2026, 1, 15, 1, 0));
    expect(result.getHours()).toBe(2);
    expect(result.getDate()).toBe(14);
  });

  test("at exactly 2 AM, cycle start is 2 AM same day", () => {
    const result = getCycleStart(makeDate(2026, 1, 15, 2, 0));
    expect(result.getHours()).toBe(2);
    expect(result.getDate()).toBe(15);
  });
});

describe("activeHoursInPartialCycle", () => {
  test("offset 0 (sleep start) returns 0", () => {
    expect(activeHoursInPartialCycle(0)).toBe(0);
  });

  test("offset during sleep returns 0", () => {
    expect(activeHoursInPartialCycle(3)).toBe(0);
    expect(activeHoursInPartialCycle(6.99)).toBe(0);
  });

  test("offset at sleep boundary returns 0", () => {
    expect(activeHoursInPartialCycle(SLEEP_HOURS_PER_DAY)).toBe(0);
  });

  test("offset just after wake returns small positive", () => {
    const result = activeHoursInPartialCycle(SLEEP_HOURS_PER_DAY + 0.01);
    expect(result).toBeCloseTo(0.01, 10);
  });

  test("offset at end of cycle returns ACTIVE_HOURS_PER_DAY", () => {
    expect(activeHoursInPartialCycle(24)).toBe(ACTIVE_HOURS_PER_DAY);
  });

  test("mid-active returns correct value", () => {
    // offset 15 = 8 hours into active period
    expect(activeHoursInPartialCycle(15)).toBe(15 - SLEEP_HOURS_PER_DAY);
  });
});

describe("elapsedActiveHoursBetween", () => {
  test("same timestamp returns 0", () => {
    const t = makeDate(2026, 1, 15, 12, 0);
    expect(elapsedActiveHoursBetween(t, t)).toBe(0);
  });

  test("1 hour during active period returns 1", () => {
    const start = makeDate(2026, 1, 15, 12, 0);
    const end = makeDate(2026, 1, 15, 13, 0);
    expect(elapsedActiveHoursBetween(start, end)).toBeCloseTo(1, 10);
  });

  test("full sleep period returns 0 active hours", () => {
    const start = makeDate(2026, 1, 15, 2, 0);
    const end = makeDate(2026, 1, 15, 9, 0);
    expect(elapsedActiveHoursBetween(start, end)).toBeCloseTo(0, 10);
  });

  test("full active period (9 AM to 2 AM) returns 17 hours", () => {
    const start = makeDate(2026, 1, 15, 9, 0);
    const end = makeDate(2026, 1, 16, 2, 0);
    expect(elapsedActiveHoursBetween(start, end)).toBeCloseTo(ACTIVE_HOURS_PER_DAY, 10);
  });

  test("full 24-hour cycle returns 17 active hours", () => {
    const start = makeDate(2026, 1, 15, 2, 0);
    const end = makeDate(2026, 1, 16, 2, 0);
    expect(elapsedActiveHoursBetween(start, end)).toBeCloseTo(ACTIVE_HOURS_PER_DAY, 10);
  });

  test("7 full days returns 7 * 17 = 119 active hours", () => {
    const start = makeDate(2026, 1, 10, 2, 0);
    const end = makeDate(2026, 1, 17, 2, 0);
    expect(elapsedActiveHoursBetween(start, end)).toBeCloseTo(7 * ACTIVE_HOURS_PER_DAY, 10);
  });

  test("across midnight is continuous (11 PM to 1 AM = 2 active hours)", () => {
    const start = makeDate(2026, 1, 15, 23, 0);
    const end = makeDate(2026, 1, 16, 1, 0);
    expect(elapsedActiveHoursBetween(start, end)).toBeCloseTo(2, 10);
  });
});

describe("no discontinuities at critical boundaries", () => {
  /**
   * Walk minute-by-minute through a full 7-day period and verify:
   * 1. The elapsed active hours never decrease (monotonically non-decreasing)
   * 2. The maximum jump between consecutive minutes is small (no sudden jumps)
   */
  test("elapsed active hours is monotonically non-decreasing (minute granularity, 7 days)", () => {
    const periodStart = makeDate(2026, 1, 10, 14, 30); // arbitrary start mid-afternoon
    const totalMinutes = 7 * 24 * 60;
    let prevValue = 0;
    let maxJump = 0;

    for (let m = 0; m <= totalMinutes; m++) {
      const now = new Date(periodStart.getTime() + m * 60_000);
      const elapsed = elapsedActiveHoursBetween(periodStart, now);

      // Must never decrease
      expect(elapsed).toBeGreaterThanOrEqual(prevValue);

      // Track maximum jump
      const jump = elapsed - prevValue;
      if (jump > maxJump) maxJump = jump;

      prevValue = elapsed;
    }

    // Maximum jump in 1 minute should be ~1/60 hour (during active) or 0 (during sleep)
    // Allow small floating point margin
    expect(maxJump).toBeLessThanOrEqual(1 / 60 + 1e-9);
  });

  test("no jump at 2:00 AM boundary (sleep start)", () => {
    const periodStart = makeDate(2026, 1, 10, 12, 0);

    // Check the minute before and after 2:00 AM
    const before = makeDate(2026, 1, 15, 1, 59);
    const at = makeDate(2026, 1, 15, 2, 0);
    const after = makeDate(2026, 1, 15, 2, 1);

    const valBefore = elapsedActiveHoursBetween(periodStart, before);
    const valAt = elapsedActiveHoursBetween(periodStart, at);
    const valAfter = elapsedActiveHoursBetween(periodStart, after);

    // 1:59 AM to 2:00 AM: still active, should increase by ~1/60
    expect(valAt - valBefore).toBeCloseTo(1 / 60, 4);
    // 2:00 AM to 2:01 AM: now sleeping, should be flat
    expect(valAfter - valAt).toBeCloseTo(0, 10);
  });

  test("no jump at 9:00 AM boundary (wake start)", () => {
    const periodStart = makeDate(2026, 1, 10, 12, 0);

    const before = makeDate(2026, 1, 15, 8, 59);
    const at = makeDate(2026, 1, 15, 9, 0);
    const after = makeDate(2026, 1, 15, 9, 1);

    const valBefore = elapsedActiveHoursBetween(periodStart, before);
    const valAt = elapsedActiveHoursBetween(periodStart, at);
    const valAfter = elapsedActiveHoursBetween(periodStart, after);

    // 8:59 AM to 9:00 AM: still sleeping, should be flat
    expect(valAt - valBefore).toBeCloseTo(0, 10);
    // 9:00 AM to 9:01 AM: now active, should increase by ~1/60
    expect(valAfter - valAt).toBeCloseTo(1 / 60, 4);
  });

  test("no jump at midnight boundary", () => {
    const periodStart = makeDate(2026, 1, 10, 12, 0);

    const before = makeDate(2026, 1, 14, 23, 59);
    const at = makeDate(2026, 1, 15, 0, 0);
    const after = makeDate(2026, 1, 15, 0, 1);

    const valBefore = elapsedActiveHoursBetween(periodStart, before);
    const valAt = elapsedActiveHoursBetween(periodStart, at);
    const valAfter = elapsedActiveHoursBetween(periodStart, after);

    // All active times - should increase smoothly
    const jump1 = valAt - valBefore;
    const jump2 = valAfter - valAt;

    expect(jump1).toBeCloseTo(1 / 60, 4);
    expect(jump2).toBeCloseTo(1 / 60, 4);
  });

  test("flat during sleep hours (2 AM - 9 AM)", () => {
    const periodStart = makeDate(2026, 1, 10, 12, 0);

    const sleepStart = elapsedActiveHoursBetween(periodStart, makeDate(2026, 1, 15, 2, 0));
    const sleepMid = elapsedActiveHoursBetween(periodStart, makeDate(2026, 1, 15, 5, 30));
    const sleepEnd = elapsedActiveHoursBetween(periodStart, makeDate(2026, 1, 15, 9, 0));

    expect(sleepMid).toBeCloseTo(sleepStart, 10);
    expect(sleepEnd).toBeCloseTo(sleepStart, 10);
  });

  test("monotonic with arbitrary period start times", () => {
    // Test with various period start times to catch alignment issues
    const startTimes = [
      makeDate(2026, 1, 10, 0, 0),   // midnight
      makeDate(2026, 1, 10, 1, 30),   // during sleep (before 2 AM boundary)
      makeDate(2026, 1, 10, 2, 0),    // exactly at sleep start
      makeDate(2026, 1, 10, 5, 0),    // mid-sleep
      makeDate(2026, 1, 10, 9, 0),    // exactly at wake
      makeDate(2026, 1, 10, 15, 45),  // mid-afternoon
      makeDate(2026, 1, 10, 23, 59),  // just before midnight
    ];

    for (const periodStart of startTimes) {
      let prevValue = 0;
      // Check every 10 minutes for 7 days
      const totalSteps = 7 * 24 * 6;
      for (let step = 0; step <= totalSteps; step++) {
        const now = new Date(periodStart.getTime() + step * 10 * 60_000);
        const elapsed = elapsedActiveHoursBetween(periodStart, now);
        expect(elapsed).toBeGreaterThanOrEqual(prevValue);
        prevValue = elapsed;
      }
    }
  });
});

describe("pace calculation sanity", () => {
  test("at period midpoint during active hours, elapsed should be roughly half total", () => {
    // 3.5 days into period, at noon (active time)
    const start = makeDate(2026, 1, 10, 12, 0);
    const midpoint = makeDate(2026, 1, 14, 0, 0); // 3.5 days later

    const elapsed = elapsedActiveHoursBetween(start, midpoint);
    const totalActive = 7 * ACTIVE_HOURS_PER_DAY;

    // Should be roughly 50% through, give or take
    const ratio = elapsed / totalActive;
    expect(ratio).toBeGreaterThan(0.35);
    expect(ratio).toBeLessThan(0.65);
  });
});
