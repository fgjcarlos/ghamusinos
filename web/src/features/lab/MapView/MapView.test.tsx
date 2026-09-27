// TDD contract for MapView (issue 125).
// Pure DOM test: we assert the placeholder div with data-testid, and
// the integration with maplibre-gl is left to a smoke test in the
// browser (jsdom doesn't render a real canvas).

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MapView } from './MapView';

// maplibre-gl imports a CSS side-effect at module load and touches the
// DOM on the global window object. jsdom doesn't implement WebGL/canvas
// so we don't actually instantiate a map; we only assert the component
// shell. The full map behaviour is covered by browser smoke tests.
//
// vi.mock factory is hoisted to the top of the file, BEFORE the
// top-level class declarations below — referencing those classes
// inside the factory triggers "Cannot access X before initialization".
// So we define the fakes inline.
vi.mock('maplibre-gl', () => {
  class FakeMap {
    constructor() {
      // intentionally empty
    }
    on(): FakeMap {
      return this;
    }
    off(): FakeMap {
      return this;
    }
    remove(): void {
      // intentionally empty
    }
    getLayer(): undefined {
      return undefined;
    }
    getSource(): undefined {
      return undefined;
    }
    removeLayer(): void {
      // intentionally empty
    }
    removeSource(): void {
      // intentionally empty
    }
    addSource(): void {
      // intentionally empty
    }
    addLayer(): void {
      // intentionally empty
    }
  }

  class FakeMarker {
    constructor() {
      // intentionally empty
    }
    setLngLat(): FakeMarker {
      return this;
    }
    setPopup(): FakeMarker {
      return this;
    }
    addTo(): FakeMarker {
      return this;
    }
    remove(): void {
      // intentionally empty
    }
  }

  class FakePopup {
    constructor() {
      // intentionally empty
    }
    setHTML(): FakePopup {
      return this;
    }
  }

  return {
    default: FakeMap,
    Map: FakeMap,
    Marker: FakeMarker,
    Popup: FakePopup,
  };
});

describe('MapView', () => {
  it('renders the map container with the testid', () => {
    const coords: [number, number][] = [
      [-3.7, 40.4],
      [-3.6, 40.5],
    ];
    render(<MapView coordinates={coords} />);
    expect(screen.getByTestId('map-view')).toBeInTheDocument();
  });

  it('has the accessible role and label', () => {
    render(
      <MapView
        coordinates={[
          [0, 0],
          [1, 1],
        ]}
      />,
    );
    const el = screen.getByTestId('map-view');
    expect(el.tagName).toBe('DIV');
    expect(el.getAttribute('role')).toBe('region');
    expect(el.getAttribute('aria-label')).toBe('Mapa del track');
  });

  it('respects the height prop', () => {
    render(<MapView coordinates={[[0, 0]]} height={480} />);
    const el = screen.getByTestId('map-view');
    expect(el.style.height).toBe('480px');
  });

  it('renders nothing extra when coordinates are too few (no crash)', () => {
    // < 2 coordinates: the useEffect bails out, the container still
    // mounts but map is never instantiated. No assertion on internal
    // state — just confirm the container renders.
    render(<MapView coordinates={[[0, 0]]} />);
    expect(screen.getByTestId('map-view')).toBeInTheDocument();
  });
});
