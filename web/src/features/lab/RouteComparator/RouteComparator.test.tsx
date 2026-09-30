import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { CompareResponse } from '../../../lib/api/types';
import { RouteComparator } from './RouteComparator';

vi.mock('../ComparisonMap/ComparisonMap', () => ({
  ComparisonMap: ({ tracks }: { tracks: { coordinates: [number, number][] }[] }) => (
    <pre data-testid="comparison-map-coordinates">{JSON.stringify(tracks)}</pre>
  ),
}));
vi.mock('../MetricsDiffTable/MetricsDiffTable', () => ({ MetricsDiffTable: () => null }));
vi.mock('../RiskZonesPanel/RiskZonesPanel', () => ({ RiskZonesPanel: () => null }));
vi.mock('../ComparisonElevationProfile/ComparisonElevationProfile', () => ({
  ComparisonElevationProfile: () => null,
}));

function makeResponse(points: unknown[]): CompareResponse {
  return {
    tracks: [
      {
        track: {
          track: { name: 'Wire shape', points },
        },
        risk_zones: [],
        muros: [],
        recovery_zones: [],
        km_vertical: null,
      },
    ],
    diff: {},
  } as unknown as CompareResponse;
}

describe('RouteComparator point normalization', () => {
  it('accepts Go lon points and normalizes them to lng for the map', () => {
    render(<RouteComparator data={makeResponse([{ lat: 40, lon: -3 }])} />);
    expect(screen.getByTestId('comparison-map-coordinates').textContent).toContain(
      '"coordinates":[[-3,40]]',
    );
  });
});
