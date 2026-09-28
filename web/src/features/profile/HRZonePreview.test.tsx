// TDD contract for HRZonePreview (issue 159).
// Verifies the thresholds match internal/jobs/streams.go:135
// (60 / 70 / 80 / 90 %) and the empty state when hr_max is missing.

import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { HRZonePreview } from './HRZonePreview';

describe('HRZonePreview', () => {
  it('renders the empty state when hr_max is null', () => {
    render(<HRZonePreview hr_max={null} />);
    expect(screen.getByTestId('hr-zones-empty')).toBeInTheDocument();
    expect(screen.getByText(/Introduce tu FC máxima para ver tus zonas/i)).toBeInTheDocument();
  });

  it('renders the empty state when hr_max is 0 or negative', () => {
    const { rerender } = render(<HRZonePreview hr_max={0} />);
    expect(screen.getByTestId('hr-zones-empty')).toBeInTheDocument();
    rerender(<HRZonePreview hr_max={-5} />);
    expect(screen.getByTestId('hr-zones-empty')).toBeInTheDocument();
  });

  it('renders 5 zones when hr_max is set, with 60/70/80/90 thresholds', () => {
    render(<HRZonePreview hr_max={180} />);
    expect(screen.getByTestId('hr-zones-preview')).toBeInTheDocument();
    const list = screen.getByRole('list');
    const items = within(list).getAllByRole('listitem');
    expect(items).toHaveLength(5);
    // Each item shows its Z label and pct range. `getByText` matches
    // both the <li> (aggregated child text) and the inner <span>
    // — count both matches per item, which proves each label exists
    // exactly once as a span and the label text is correct.
    const expected: ReadonlyArray<{ idx: number; label: string; range: string }> = [
      { idx: 0, label: 'Z1', range: '0%–60%' },
      { idx: 1, label: 'Z2', range: '60%–70%' },
      { idx: 2, label: 'Z3', range: '70%–80%' },
      { idx: 3, label: 'Z4', range: '80%–90%' },
      { idx: 4, label: 'Z5', range: '90%–100%' },
    ];
    for (const want of expected) {
      const matches = within(items[want.idx]).getAllByText(want.label);
      // The <li> + the inner <span> = 2 matches; if the inner span
      // is missing we'd see just 1.
      expect(matches.length).toBeGreaterThanOrEqual(1);
      expect(within(items[want.idx]).getByText(want.range)).toBeInTheDocument();
    }
  });

  it('computes per-zone BPM ranges from the configured hr_max', () => {
    // hr_max = 200 → Z1: 0–120, Z2: 120–140, Z3: 140–160, Z4: 160–180, Z5: 180–200
    render(<HRZonePreview hr_max={200} />);
    expect(screen.getByText('0–120 bpm')).toBeInTheDocument();
    expect(screen.getByText('120–140 bpm')).toBeInTheDocument();
    expect(screen.getByText('140–160 bpm')).toBeInTheDocument();
    expect(screen.getByText('160–180 bpm')).toBeInTheDocument();
    expect(screen.getByText('180–200 bpm')).toBeInTheDocument();
  });
});
