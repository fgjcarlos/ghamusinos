// TDD contract for the calendar civil-date model (#313, T3).
//
// Civil dates are 'YYYY-MM-DD' strings carried as UTC instants
// (T00:00:00Z) for weekday math only: no local/browser parsing. "Today"
// resolves through the authenticated user's IANA zone, mirroring the
// backend LocalDate helper (a local midnight can be skipped or
// repeated, so we never construct it).

import { describe, expect, it } from 'vitest';
import {
  addDays,
  addMonths,
  monthDays,
  monthLabel,
  rangeOf,
  resolveZone,
  todayKey,
  weekDays,
  weekLabel,
  weekStart,
} from './calendar-model';

describe('calendar-model', () => {
  it('resolves today in the user zone, not the browser zone', () => {
    const now = new Date('2026-11-25T23:30:00Z');
    expect(todayKey(now, 'Europe/Madrid')).toBe('2026-11-26');
    expect(todayKey(now, 'UTC')).toBe('2026-11-25');
    expect(todayKey(now, 'Pacific/Auckland')).toBe('2026-11-26');
  });

  it('falls back to UTC for an invalid stored zone', () => {
    expect(resolveZone('Europe/Madrid')).toBe('Europe/Madrid');
    expect(resolveZone('Not/AZone')).toBe('UTC');
    expect(todayKey(new Date('2026-11-25T23:30:00Z'), resolveZone('Not/AZone'))).toBe('2026-11-25');
  });

  it('adds days across month boundaries on UTC carriers', () => {
    expect(addDays('2026-11-30', 1)).toBe('2026-12-01');
    expect(addDays('2026-03-01', -1)).toBe('2026-02-28');
  });

  it('adds months clamping to the target month length', () => {
    expect(addMonths('2026-01-31', 1)).toBe('2026-02-28');
    expect(addMonths('2026-03-31', -1)).toBe('2026-02-28');
    expect(addMonths('2026-11-05', -1)).toBe('2026-10-05');
  });

  it('uses Monday-based weeks', () => {
    expect(weekStart('2026-11-26')).toBe('2026-11-23'); // jueves -> lunes
    expect(weekStart('2026-11-23')).toBe('2026-11-23'); // lunes -> lunes
    const days = weekDays('2026-11-26');
    expect(days).toHaveLength(7);
    expect(days[0]).toBe('2026-11-23');
    expect(days[6]).toBe('2026-11-29');
  });

  it('builds a fixed six-week month grid starting on Monday', () => {
    const days = monthDays('2026-11-05');
    expect(days).toHaveLength(42);
    expect(days[0]).toBe('2026-10-26'); // lunes de la semana del 1 de noviembre
    expect(days[41]).toBe('2026-12-06');
  });

  it('derives the inclusive visible range from the grid', () => {
    expect(rangeOf(weekDays('2026-11-26'))).toEqual({ from: '2026-11-23', to: '2026-11-29' });
    expect(rangeOf(monthDays('2026-11-05'))).toEqual({ from: '2026-10-26', to: '2026-12-06' });
  });

  it('labels periods in Spanish from UTC carriers', () => {
    expect(monthLabel('2026-11-05')).toBe('Noviembre de 2026');
    expect(weekLabel('2026-11-26')).toBe('23 – 29 de noviembre de 2026');
    expect(weekLabel('2026-11-30')).toBe('30 de noviembre – 6 de diciembre de 2026');
  });
});
