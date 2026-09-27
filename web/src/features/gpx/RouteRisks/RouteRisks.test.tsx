// TDD contract for RouteRisks (issue 157).

import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { RouteRisks } from './RouteRisks';
import type { NormalizedRiskZone } from '../normalize';

function makeRisk(overrides: Partial<NormalizedRiskZone> = {}): NormalizedRiskZone {
  return {
    start_idx: 0,
    end_idx: 10,
    category: 'steep',
    severity: 0.5,
    ...overrides,
  };
}

describe('RouteRisks', () => {
  it('renders an empty state when no risks', () => {
    render(<RouteRisks risks={[]} />);
    expect(screen.getByTestId('route-risks-empty')).toBeInTheDocument();
    expect(screen.getByText(/no se detectaron zonas de riesgo/i)).toBeInTheDocument();
  });

  it('renders the count in the heading', () => {
    render(<RouteRisks risks={[makeRisk(), makeRisk()]} />);
    expect(screen.getByRole('heading', { name: /Zonas de riesgo \(2\)/ })).toBeInTheDocument();
  });

  it('renders one item per risk', () => {
    render(
      <RouteRisks
        risks={[
          makeRisk({ category: 'steep' }),
          makeRisk({ category: 'technical' }),
          makeRisk({ category: 'exposure' }),
        ]}
      />,
    );
    expect(screen.getAllByTestId('risk')).toHaveLength(3);
  });

  it('uses high severity for severity >= 0.7', () => {
    render(<RouteRisks risks={[makeRisk({ severity: 0.85, category: 'steep' })]} />);
    const item = screen.getByTestId('risk');
    // SeverityPill exposes data-testid="severity-{level}".
    expect(within(item).getByTestId('severity-high')).toBeInTheDocument();
  });

  it('uses medium severity for severity < 0.7', () => {
    render(<RouteRisks risks={[makeRisk({ severity: 0.4, category: 'technical' })]} />);
    const item = screen.getByTestId('risk');
    expect(within(item).getByTestId('severity-medium')).toBeInTheDocument();
  });

  it('labels each risk with its category', () => {
    render(
      <RouteRisks
        risks={[
          makeRisk({ category: 'steep' }),
          makeRisk({ category: 'technical' }),
          makeRisk({ category: 'exposure' }),
        ]}
      />,
    );
    expect(screen.getByText('Pendiente')).toBeInTheDocument();
    expect(screen.getByText('Técnico')).toBeInTheDocument();
    expect(screen.getByText('Exposición')).toBeInTheDocument();
  });
});
