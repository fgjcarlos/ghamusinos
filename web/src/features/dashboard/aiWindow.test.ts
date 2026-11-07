// TDD contract for the AI windows of the dashboard (fase 1.5, T8 — S4).
// The weekly window is always 7 days; the load window follows the selected
// period but never exceeds the 90 days the API accepts.

import { describe, it, expect } from 'vitest';
import { AI_MAX_WINDOW_DAYS, loadWindow, shiftDays, weeklyWindow } from './aiWindow';

describe('aiWindow', () => {
  it('moves a date by a number of days', () => {
    expect(shiftDays('2026-01-31', -6)).toBe('2026-01-25');
    expect(shiftDays('2026-01-31', 1)).toBe('2026-02-01');
  });

  it('builds the weekly window ending on the last day of the panel', () => {
    expect(weeklyWindow('2026-01-31')).toEqual({
      windowStart: '2026-01-25',
      windowDays: 7,
    });
  });

  it('builds the load window of the selected period', () => {
    expect(loadWindow('2026-01-31', 30)).toEqual({
      windowStart: '2026-01-02',
      windowDays: 30,
    });
  });

  it('clamps the load window to the maximum the API accepts', () => {
    expect(loadWindow('2026-01-31', 365)).toEqual({
      windowStart: '2025-11-03',
      windowDays: AI_MAX_WINDOW_DAYS,
    });
  });

  it('never builds a window shorter than a day', () => {
    expect(loadWindow('2026-01-31', 0).windowDays).toBe(1);
  });
});
