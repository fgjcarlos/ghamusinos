// RouteClimbs: list of climbs for the route detail page. Issue 157.
// The king climb (is_king_climb=true) is highlighted with the
// accent color and the label "King climb".

import { DifficultyBadge } from '../../../ui/DifficultyBadge';
import type { NormalizedClimb } from '../normalize';
import styles from './RouteClimbs.module.css';

export interface RouteClimbsProps {
  climbs: NormalizedClimb[];
}

export function RouteClimbs({ climbs }: RouteClimbsProps) {
  if (climbs.length === 0) {
    return (
      <section className={styles.empty} data-testid="route-climbs-empty">
        <h2 className={styles.title}>Subidas</h2>
        <p>No se detectaron subidas relevantes en este track.</p>
      </section>
    );
  }
  return (
    <section className={styles.climbs} aria-label="Subidas del track">
      <h2 className={styles.title}>Subidas ({climbs.length})</h2>
      <ul className={styles.list}>
        {climbs.map((c, i) => (
          <li
            key={i}
            className={c.is_king_climb ? styles.itemKing : styles.item}
            data-testid={c.is_king_climb ? 'climb-king' : 'climb'}
          >
            <header className={styles.itemHeader}>
              {c.is_king_climb ? (
                <DifficultyBadge
                  level={climbLevel(c.avg_slope_pct)}
                  score={climbScore(c.gain_m, c.distance_m)}
                />
              ) : (
                <span className={styles.itemIndex}>#{i + 1}</span>
              )}
              {c.is_king_climb && <span className={styles.kingBadge}>Subida reina</span>}
            </header>
            <dl className={styles.stats}>
              <div>
                <dt>Desnivel</dt>
                <dd>{c.gain_m.toFixed(0)} m</dd>
              </div>
              <div>
                <dt>Distancia</dt>
                <dd>{(c.distance_m / 1000).toFixed(2)} km</dd>
              </div>
              <div>
                <dt>Pendiente</dt>
                <dd>{c.avg_slope_pct.toFixed(1)}%</dd>
              </div>
              {c.vam !== null && (
                <div>
                  <dt>VAM</dt>
                  <dd>{c.vam.toFixed(0)} m/h</dd>
                </div>
              )}
            </dl>
          </li>
        ))}
      </ul>
    </section>
  );
}

// Map an avg slope pct to the closest DifficultyBadge level.
function climbLevel(slopePct: number): 'beginner' | 'intermediate' | 'advanced' | 'pro' {
  if (slopePct < 4) return 'beginner';
  if (slopePct < 8) return 'intermediate';
  if (slopePct < 12) return 'advanced';
  return 'pro';
}

function climbScore(gainM: number, distanceM: number): number {
  if (distanceM <= 0) return 0;
  return Math.min(100, Math.round((gainM / distanceM) * 100 * 5));
}
