// TDD contract for ElevationProfile (issue 156).
// Pure presentational: takes a ProjectedTrack and renders SVG.
// Tests assert observable contract (SVG structure, aria-label, band
// count) — not literal CSS values.

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ElevationProfile } from './ElevationProfile';
import type { ProjectedTrack } from './projectTrack';

function makeProjected(overrides: Partial<ProjectedTrack> = {}): ProjectedTrack {
  return {
    path: 'M 0 100 L 50 80 L 100 90',
    area: 'M 0 176 L 0 100 L 100 90 L 100 176 Z',
    bands: [],
    yTicks: [
      { y: 80, x: 36, label: '500m', value: 500 },
      { y: 100, x: 36, label: '750m', value: 750 },
    ],
    xTicks: [
      { y: 192, x: 100, label: '0m', value: 0 },
      { y: 192, x: 400, label: '500m', value: 500 },
    ],
    ...overrides,
  };
}

describe('ElevationProfile', () => {
  it('renders an SVG element', () => {
    render(<ElevationProfile projected={makeProjected()} width={800} height={200} />);
    expect(screen.getByTestId('elevation-profile')).toBeInTheDocument();
  });

  it('uses the provided width and height for the viewBox', () => {
    render(<ElevationProfile projected={makeProjected()} width={600} height={150} />);
    const svg = screen.getByTestId('elevation-profile');
    expect(svg.getAttribute('viewBox')).toBe('0 0 600 150');
    expect(svg.getAttribute('width')).toBe('600');
    expect(svg.getAttribute('height')).toBe('150');
  });

  it('renders one rect per band plus a baseline line', () => {
    const bands = [
      { x: 50, width: 100, type: 'climb' as const },
      { x: 200, width: 80, type: 'risk' as const },
      { x: 320, width: 60, type: 'climb' as const },
    ];
    const { container } = render(
      <ElevationProfile projected={makeProjected({ bands })} width={800} height={200} />,
    );
    // One <rect> per band; the baseline is a <line>, not a <rect>.
    const rects = container.querySelectorAll('rect');
    expect(rects).toHaveLength(bands.length);
    // The baseline is rendered as a <line> at the bottom of the plot.
    const lines = container.querySelectorAll('line');
    expect(lines.length).toBeGreaterThanOrEqual(1);
  });

  it('renders an accessible label for each band', () => {
    const bands = [
      { x: 50, width: 100, type: 'climb' as const },
      { x: 200, width: 80, type: 'risk' as const },
    ];
    render(<ElevationProfile projected={makeProjected({ bands })} width={800} height={200} />);
    // Band labels are exposed via aria-label on the rect; the <title>
    // child of an SVG rect is unreliable in jsdom. Query by aria-label.
    expect(screen.getByLabelText(/Subida 1/)).toBeInTheDocument();
    // The risk band is the second band (i = 1), so bandLabel returns
    // "Zona de riesgo 2", not 1.
    expect(screen.getByLabelText(/Zona de riesgo 2/)).toBeInTheDocument();
  });

  it('renders a polyline path with the projected path data', () => {
    const path = 'M 44 176 L 88 80 L 132 90';
    const { container } = render(
      <ElevationProfile projected={makeProjected({ path })} width={800} height={200} />,
    );
    const paths = container.querySelectorAll('path');
    expect(paths.length).toBeGreaterThanOrEqual(1);
    const linePath = Array.from(paths).find((p) => p.getAttribute('d') === path);
    expect(linePath).toBeTruthy();
  });

  it('renders tick labels for both axes', () => {
    render(<ElevationProfile projected={makeProjected()} width={800} height={200} />);
    // '750m' is unique to the y-axis in this fixture.
    expect(screen.getByText('750m')).toBeInTheDocument();
    // '500m' appears on both axes — verify both with getAllByText.
    expect(screen.getAllByText('500m').length).toBeGreaterThanOrEqual(2);
  });

  it('has an aria-label mentioning the elevation range', () => {
    render(<ElevationProfile projected={makeProjected()} width={800} height={200} />);
    const aria = screen.getByTestId('elevation-profile').getAttribute('aria-label') || '';
    expect(aria.toLowerCase()).toContain('perfil de elevación');
  });
});
