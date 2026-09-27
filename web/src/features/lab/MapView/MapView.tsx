// MapView: 2D map showing the track polyline. Issue 125.
// Wraps maplibre-gl with cleanup on unmount. Uses OpenStreetMap
// raster tiles (no API key, no auth) for development. The CSP will
// need to allow `img-src` and `worker-src` to those tiles; that's a
// separate follow-up.

import { useEffect, useRef } from 'react';
import * as maplibregl from 'maplibre-gl';
import type { LngLatBoundsLike, StyleSpecification } from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import styles from './MapView.module.css';

export type LngLat = [number, number];

export interface MapViewProps {
  coordinates: LngLat[];
  height?: number;
  startLabel?: string;
  endLabel?: string;
}

// Maplibre paints via its own JS style object — CSS variables from
// tokens.css don't reach there. These mirror the exact values in
// web/src/styles/tokens.css; if the token changes, update both.
const ACCENT_COLOR = '#7fc689'; // --gh-accent
const RISK_COLOR = '#f0735a'; // --gh-risk

const OSM_RASTER_STYLE: StyleSpecification = {
  version: 8,
  sources: {
    osm: {
      type: 'raster',
      tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
      tileSize: 256,
      attribution: '© OpenStreetMap contributors',
    },
  },
  layers: [
    {
      id: 'osm',
      type: 'raster',
      source: 'osm',
      minzoom: 0,
      maxzoom: 19,
    },
  ],
};

export function MapView({
  coordinates,
  height = 360,
  startLabel = 'Inicio',
  endLabel = 'Fin',
}: MapViewProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const markersRef = useRef<maplibregl.Marker[]>([]);

  useEffect(() => {
    if (!containerRef.current || coordinates.length < 2) return;

    const lons = coordinates.map(([lng]) => lng);
    const lats = coordinates.map(([, lat]) => lat);
    const bounds: LngLatBoundsLike = [
      [Math.min(...lons), Math.min(...lats)],
      [Math.max(...lons), Math.max(...lats)],
    ];

    const map = new maplibregl.Map({
      container: containerRef.current,
      style: OSM_RASTER_STYLE,
      bounds,
      fitBoundsOptions: { padding: 40 },
    });
    mapRef.current = map;

    const onLoad = () => {
      // Drop the previous polyline + markers if a re-load happens.
      if (map.getLayer('track-line')) map.removeLayer('track-line');
      if (map.getSource('track')) map.removeSource('track');
      markersRef.current.forEach((m) => m.remove());
      markersRef.current = [];

      map.addSource('track', {
        type: 'geojson',
        data: {
          type: 'Feature',
          properties: {},
          geometry: {
            type: 'LineString',
            coordinates,
          },
        },
      });
      map.addLayer({
        id: 'track-line',
        type: 'line',
        source: 'track',
        paint: {
          // Token colour: green for the route line.
          'line-color': ACCENT_COLOR,
          'line-width': 3,
        },
      });

      const start = coordinates[0];
      const end = coordinates[coordinates.length - 1];
      if (start) {
        const startMarker = new maplibregl.Marker({ color: ACCENT_COLOR })
          .setLngLat(start)
          .setPopup(new maplibregl.Popup({ offset: 16 }).setHTML(`<strong>${startLabel}</strong>`))
          .addTo(map);
        markersRef.current.push(startMarker);
      }
      if (end) {
        const endMarker = new maplibregl.Marker({ color: RISK_COLOR })
          .setLngLat(end)
          .setPopup(new maplibregl.Popup({ offset: 16 }).setHTML(`<strong>${endLabel}</strong>`))
          .addTo(map);
        markersRef.current.push(endMarker);
      }
    };

    map.on('load', onLoad);

    return () => {
      markersRef.current.forEach((m) => m.remove());
      markersRef.current = [];
      map.off('load', onLoad);
      map.remove();
      mapRef.current = null;
    };
  }, [coordinates, startLabel, endLabel]);

  return (
    <div
      ref={containerRef}
      className={styles.map}
      style={{ height: `${height}px` }}
      data-testid="map-view"
      role="region"
      aria-label="Mapa del track"
    />
  );
}
