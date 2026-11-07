// Terreno tab: OSM surface/technicality segments, loaded on demand.

import { useTrackTerrain } from '../hooks/useTrackTerrain';
import { surfaceLabel } from './format';
import styles from './RacePlanSection.module.css';

export interface TerrainPanelProps {
  token: string;
  trackId: string;
}

export function TerrainPanel({ token, trackId }: TerrainPanelProps) {
  const { data, loading, error, load } = useTrackTerrain(token, trackId);

  const hasSegments = data !== null && data.available && data.segments.length > 0;
  const noData = data !== null && (!data.available || data.segments.length === 0);

  return (
    <div className={styles.stack}>
      <button type="button" className={styles.action} onClick={() => void load()}>
        Analizar terreno
      </button>

      {loading && <p className={styles.meta}>Analizando terreno…</p>}

      {error !== null && (
        <p role="alert" className={styles.failed}>
          {error}
        </p>
      )}

      {noData && <p className={styles.empty}>Sin datos de terreno para esta ruta</p>}

      {hasSegments && (
        <ul className={styles.segmentList}>
          {data.segments.map((segment) => (
            <li key={`${segment.start_km}-${segment.end_km}`} className={styles.segmentItem}>
              <span className={styles.badge}>{surfaceLabel(segment.surface)}</span>
              <span className={styles.meta}>Técnica {segment.technicality}/5</span>
              {segment.name !== '' && <span>{segment.name}</span>}
              <span className={styles.mono}>
                {segment.start_km.toFixed(2)}–{segment.end_km.toFixed(2)} km
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
