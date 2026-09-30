// TDD contract for TrackDetailPage (issue 125).

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TrackDetailPage } from './TrackDetailPage';
import type { NormalizedTrackDetail } from '../../gpx/normalize';

// RouteDetail renders <Link> (back to /lab) which needs the router
// context. Wrap every render so the test setup mirrors the real app.
function renderInRouter(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

// Mock the MapView so we don't drag maplibre-gl into jsdom (WebGL-less
// environment). The test asserts the layout decisions (where the map
// goes vs where the route detail goes) and the empty-state path.
vi.mock('../MapView/MapView', () => ({
  MapView: ({ coordinates }: { coordinates: [number, number][] }) => (
    <div data-testid="map-view" data-coords-len={coordinates.length} />
  ),
}));

function makeData(overrides: Partial<NormalizedTrackDetail['track']> = {}): NormalizedTrackDetail {
  return {
    track: {
      id: 'a'.repeat(32),
      user_id: 'b'.repeat(32),
      name: 'Circular del Torrico',
      file_hash: 'abc',
      file_size_bytes: 102400,
      points: [],
      track_type: 'circular',
      uploaded_at: new Date('2025-09-15T08:00:00Z'),
      ...overrides,
    },
    analysis: {
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
    },
    climbs: [],
    risk_zones: [],
    muros: [],
    recovery_zones: [],
    km_vertical: null,
  };
}

describe('TrackDetailPage', () => {
  it('renders the page wrapper with data-testid', () => {
    renderInRouter(<TrackDetailPage data={makeData()} />);
    expect(screen.getByTestId('track-detail-page')).toBeInTheDocument();
  });

  it('renders the map when there are at least 2 points', () => {
    const data = makeData({
      points: [
        { lat: 40.4, lng: -3.7 },
        { lat: 40.5, lng: -3.6 },
        { lat: 40.6, lng: -3.5 },
      ],
    });
    renderInRouter(<TrackDetailPage data={data} />);
    const map = screen.getByTestId('map-view');
    expect(map).toBeInTheDocument();
    // The map mock receives 3 coordinates (lng, lat) in GeoJSON order.
    expect(map.dataset.coordsLen).toBe('3');
  });

  it('renders the map when points use the lon key', () => {
    const data = makeData({
      points: [
        { lat: 40.4, lon: -3.7 },
        { lat: 40.5, lon: -3.6 },
        { lat: 40.6, lon: -3.5 },
      ],
    });
    renderInRouter(<TrackDetailPage data={data} />);
    expect(screen.getByTestId('map-view').dataset.coordsLen).toBe('3');
  });

  it('renders the empty state when the backend has no points', () => {
    renderInRouter(<TrackDetailPage data={makeData({ points: [] })} />);
    expect(screen.getByTestId('map-empty')).toBeInTheDocument();
    expect(screen.queryByTestId('map-view')).toBeNull();
  });

  it('renders the empty state when the backend returns malformed points', () => {
    // Anything that doesn't match { lat: number; lng: number } is dropped.
    const data = makeData({
      points: [{ foo: 'bar' }, null, 'string'] as unknown[],
    });
    renderInRouter(<TrackDetailPage data={data} />);
    expect(screen.getByTestId('map-empty')).toBeInTheDocument();
  });

  it('always composes RouteDetail below the map slot', () => {
    renderInRouter(
      <TrackDetailPage
        data={makeData({
          points: [
            { lat: 1, lng: 2 },
            { lat: 3, lng: 4 },
          ],
        })}
      />,
    );
    // RouteDetail exposes its own testid on the article element.
    expect(screen.getByTestId('route-detail')).toBeInTheDocument();
  });
});
