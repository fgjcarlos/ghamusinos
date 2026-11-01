// TDD contract for CardiacDriftPanel (Fase 1.4, DW-006).
// The panel lists the activities with a computable drift.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { CardiacDriftActivity } from '../../../lib/api/dashboard';
import { CardiacDriftPanel } from './CardiacDriftPanel';

const ACTIVITIES: CardiacDriftActivity[] = [
  { activity_id: 'act-1', drift_pct: 4.25, sample_count: 900 },
  { activity_id: 'act-2', drift_pct: 6, sample_count: 1200 },
];

describe('CardiacDriftPanel', () => {
  it('renders the panel container', () => {
    render(<CardiacDriftPanel activities={ACTIVITIES} />);
    expect(screen.getByTestId('cardiac-drift-panel')).toBeInTheDocument();
  });

  it('lists one row per activity with its drift and sample count', () => {
    render(<CardiacDriftPanel activities={ACTIVITIES} />);
    const panel = screen.getByTestId('cardiac-drift-panel');
    expect(panel).toHaveTextContent('act-1');
    expect(panel).toHaveTextContent('4.3%');
    expect(panel).toHaveTextContent('900 muestras');
    expect(panel).toHaveTextContent('act-2');
  });

  it('renders the average drift of the activities', () => {
    render(<CardiacDriftPanel activities={ACTIVITIES} />);
    expect(screen.getByTestId('cardiac-drift-average')).toHaveTextContent('media 5.1%');
  });
});
