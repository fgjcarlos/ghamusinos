// TDD contract for SyncStatusChip (issue 155).
// Renders the Strava sync indicator in AppShell's top bar.
// In this issue the date is passed by prop; in 06 it comes from
// GET /api/v1/sync/status.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SyncStatusChip, formatRelativeTime } from './SyncStatusChip';

describe('formatRelativeTime', () => {
  const now = new Date('2026-01-15T12:00:00Z');

  it('returns "ahora" for less than 60 seconds ago', () => {
    expect(formatRelativeTime(new Date('2026-01-15T11:59:30Z'), now)).toBe('ahora');
  });

  it('returns "hace N min" for under an hour', () => {
    expect(formatRelativeTime(new Date('2026-01-15T11:48:00Z'), now)).toBe('hace 12 min');
  });

  it('returns "hace N h" for under a day', () => {
    expect(formatRelativeTime(new Date('2026-01-15T09:00:00Z'), now)).toBe('hace 3 h');
  });

  it('returns "hace N d" for one day or more', () => {
    expect(formatRelativeTime(new Date('2026-01-13T12:00:00Z'), now)).toBe('hace 2 d');
  });
});

describe('SyncStatusChip', () => {
  const now = new Date('2026-01-15T12:00:00Z');

  it('renders "Strava · ahora" when lastSync is just now', () => {
    render(<SyncStatusChip lastSync={new Date('2026-01-15T11:59:30Z')} now={now} />);
    expect(screen.getByTestId('sync-status')).toHaveTextContent(/Strava · ahora/i);
  });

  it('renders the relative time inside the chip', () => {
    render(<SyncStatusChip lastSync={new Date('2026-01-15T09:00:00Z')} now={now} />);
    expect(screen.getByTestId('sync-status')).toHaveTextContent(/Strava · hace 3 h/i);
  });

  it('renders an idle state (no text, gray dot) when lastSync is null', () => {
    render(<SyncStatusChip lastSync={null} now={now} />);
    const chip = screen.getByTestId('sync-status');
    expect(chip.textContent).toBe('');
    expect(chip.dataset.state).toBe('idle');
  });

  it('renders an active state (with text, accent dot) when lastSync is set', () => {
    render(<SyncStatusChip lastSync={new Date('2026-01-15T09:00:00Z')} now={now} />);
    const chip = screen.getByTestId('sync-status');
    expect(chip.dataset.state).toBe('active');
  });
});
