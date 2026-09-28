// TDD contract for RouteMetrics (issue 157).

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { RouteMetrics } from './RouteMetrics';
import type { NormalizedAnalysis } from '../normalize';

function makeAnalysis(overrides: Partial<NormalizedAnalysis> = {}): NormalizedAnalysis {
  return {
    distance_m: 12345,
    moving_time_s: 3600,
    d_plus_m: 500,
    d_minus_m: 480,
    max_elevation_m: 1500,
    min_elevation_m: 300,
    elevation_coverage: null,
    avg_slope_pct: 4.2,
    max_slope_pct: 18.7,
    effort_index: 87.3,
    itra_points: 320,
    leg_breaker_index: 12.4,
    estimated_vam: 850,
    difficulty_score: 75,
    difficulty_label: 'hard',
    runnability_pct: 78.5,
    ...overrides,
  };
}

describe('RouteMetrics', () => {
  it('renders 4 main metric labels', () => {
    render(<RouteMetrics analysis={makeAnalysis()} />);
    expect(screen.getByText('Distancia')).toBeInTheDocument();
    expect(screen.getByText('Desnivel +')).toBeInTheDocument();
    expect(screen.getByText('Desnivel −')).toBeInTheDocument();
    expect(screen.getByText('Tiempo en mov.')).toBeInTheDocument();
  });

  it('renders 6 derived metric labels', () => {
    render(<RouteMetrics analysis={makeAnalysis()} />);
    expect(screen.getByText('Pendiente media')).toBeInTheDocument();
    expect(screen.getByText('Pendiente máx')).toBeInTheDocument();
    expect(screen.getByText('ITRA')).toBeInTheDocument();
    expect(screen.getByText('Índice esfuerzo')).toBeInTheDocument();
    expect(screen.getByText('VAM estimada')).toBeInTheDocument();
    expect(screen.getByText('Corribilidad')).toBeInTheDocument();
  });

  it('formats distance as km with one decimal', () => {
    render(<RouteMetrics analysis={makeAnalysis({ distance_m: 12345 })} />);
    expect(screen.getByText(/12\.3 km/)).toBeInTheDocument();
  });

  it('formats elevation as integer meters', () => {
    render(<RouteMetrics analysis={makeAnalysis({ d_plus_m: 500 })} />);
    expect(screen.getByText(/500 m/)).toBeInTheDocument();
  });

  it('formats moving time as h+m when >= 1h', () => {
    render(<RouteMetrics analysis={makeAnalysis({ moving_time_s: 3600 })} />);
    expect(screen.getByText('1h 0m')).toBeInTheDocument();
  });

  it('formats moving time as m when < 1h', () => {
    render(<RouteMetrics analysis={makeAnalysis({ moving_time_s: 600 })} />);
    expect(screen.getByText('10m')).toBeInTheDocument();
  });
});
