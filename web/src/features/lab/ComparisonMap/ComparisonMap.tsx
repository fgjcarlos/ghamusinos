// ComparisonMap: MapLibre overlay for up to 3 tracks. Issue 126.
// Wraps maplibregl.Map with cleanup on unmount; each track polyline
// and start/end markers are color-coded against the per-track token.

import { useEffect, useRef } from 'react';
import * as maplibregl from 'maplibre-gl';
import type { LngLatBoundsLike, StyleSpecification } from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import styles from './ComparisonMap.module.css';

export type LngLat = [number, number];

// Mirrors the values in web/src/styles/tokens.css. MapLibre paints
// via its own JS style object where CSS variables don't reach, so
// these mirror the tokens; update both if the token changes.
const TRACK_COLORS = ['#7fc689', '#9e7a1f', '#8c3324'] as const;
//                            ^--gh-accent  ^--gh-warn-deep  ^--gh-risk-deep

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
  layers: [{ id: 'osm', type: 'raster', source: 'osm', minzoom: 0, maxzoom: 19 }],
};

export interface ComparisonMapProps {
  tracks: { name: string; coordinates: LngLat[] }[];
  height?: number;
}

function colorFor(i: number): string {
  return TRACK_COLORS[i] ?? TRACK_COLORS[TRACK_COLORS.length - 1];
}

export function ComparisonMap({ tracks, height = 360 }: ComparisonMapProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const markersRef = useRef<maplibregl.Marker[]>([]);

  useEffect(() => {
    if (!containerRef.current) return;
    const allCoords = tracks.flatMap((t) => t.coordinates);
    if (allCoords.length < 2) return;

    const lons = allCoords.map(([lng]) => lng);
    const lats = allCoords.map(([, lat]) => lat);
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
      // Clear any leftovers from a previous re-load.
      markersRef.current.forEach((m) => m.remove());
      markersRef.current = [];

      tracks.forEach((track, ti) => {
        if (track.coordinates.length < 2) return;
        const color = colorFor(ti);
        const sourceId = `track-${ti}`;
        const layerId = `track-line-${ti}`;
        if (map.getLayer(layerId)) map.removeLayer(layerId);
        if (map.getSource(sourceId)) map.removeSource(sourceId);
        map.addSource(sourceId, {
          type: 'geojson',
          data: {
            type: 'Feature',
            properties: {},
            geometry: { type: 'LineString', coordinates: track.coordinates },
          },
        });
        map.addLayer({
          id: layerId,
          type: 'line',
          source: sourceId,
          paint: { 'line-color': color, 'line-width': 3 },
        });
        const start = track.coordinates[0];
        const end = track.coordinates[track.coordinates.length - 1];
        if (start) {
          markersRef.current.push(
            new maplibregl.Marker({ color })
              .setLngLat(start)
              .setPopup(
                new maplibregl.Popup({ offset: 16 }).setHTML(
                  `<strong>${track.name}</strong> · inicio`,
                ),
              )
              .addTo(map),
          );
        }
        if (end) {
          markersRef.current.push(
            new maplibregl.Marker({ color })
              .setLngLat(end)
              .setPopup(
                new maplibregl.Popup({ offset: 16 }).setHTML(
                  `<strong>${track.name}</strong> · fin`,
                ),
              )
              .addTo(map),
          );
        }
      });
    };

    map.on('load', onLoad);

    return () => {
      markersRef.current.forEach((m) => m.remove());
      markersRef.current = [];
      map.off('load', onLoad);
      map.remove();
      mapRef.current = null;
    };
  }, [tracks]);

  return (
    <div
      ref={containerRef}
      className={styles.map}
      style={{ height: `${height}px` }}
      data-testid="comparison-map"
      role="region"
      aria-label="Mapa comparativo"
    />
  );
}
