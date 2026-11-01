// TDD contract for the dashboard period model (Fase 1.4, DW-001).
// Exactly four windows, 30d by default, and a window that ends today.

import { describe, it, expect } from 'vitest';
import { DEFAULT_PERIOD, PERIOD_OPTIONS, periodRange } from './period';

describe('dashboard periods', () => {
  it('offers exactly the four windows 7d, 30d, 90d and 1y', () => {
    expect(PERIOD_OPTIONS.map((option) => option.id)).toEqual(['7d', '30d', '90d', '1y']);
  });

  it('defaults to 30d', () => {
    expect(DEFAULT_PERIOD).toBe('30d');
  });

  it('computes a 7d window ending today', () => {
    const range = periodRange('7d', new Date('2026-01-15T12:30:00Z'));
    expect(range.to).toBe('2026-01-15');
    expect(range.from).toBe('2026-01-09');
  });

  it('computes a 30d window ending today', () => {
    const range = periodRange('30d', new Date('2026-01-15T12:30:00Z'));
    expect(range.to).toBe('2026-01-15');
    expect(range.from).toBe('2025-12-17');
  });

  it('computes a 90d window ending today', () => {
    const range = periodRange('90d', new Date('2026-01-15T12:30:00Z'));
    expect(range.to).toBe('2026-01-15');
    expect(range.from).toBe('2025-10-18');
  });

  it('computes a 1y window ending today', () => {
    const range = periodRange('1y', new Date('2026-01-15T12:30:00Z'));
    expect(range.to).toBe('2026-01-15');
    expect(range.from).toBe('2025-01-16');
  });
});
