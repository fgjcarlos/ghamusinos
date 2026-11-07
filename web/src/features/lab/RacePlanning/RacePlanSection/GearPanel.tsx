// Equipo tab: mandatory/recommended gear derived from duration, D+, altitude
// and the weather forecast when one has been loaded.

import { useMemo, useState } from 'react';
import type { TrackWeatherResponse } from '../../../../lib/api/lab';
import { calculateRequiredGear } from '../model/gear';
import { dPlusM } from '../model/trackCoords';
import type { GearContext, GearItem, TrackPoint } from '../model/types';
import { isRainyCode } from './format';
import styles from './RacePlanSection.module.css';

export interface GearPanelProps {
  points: TrackPoint[];
  estimatedTimeMin: number;
  weather: TrackWeatherResponse | null;
}

type WeatherContext = Pick<GearContext, 'minTemperatureC' | 'maxWindKmh' | 'rainy'>;

export function GearPanel({ points, estimatedTimeMin, weather }: GearPanelProps) {
  const [checked, setChecked] = useState<ReadonlySet<string>>(() => new Set<string>());

  const weatherContext = useMemo<WeatherContext>(() => {
    if (weather === null || weather.points.length === 0) return {};
    const available = weather.points.filter((point) => point.available);
    const source = available.length > 0 ? available : weather.points;
    let minTemperatureC = Number.POSITIVE_INFINITY;
    let maxWindKmh = 0;
    for (const point of source) {
      minTemperatureC = Math.min(minTemperatureC, point.temperature_c);
      maxWindKmh = Math.max(maxWindKmh, point.wind_kmh);
    }
    return {
      minTemperatureC,
      maxWindKmh,
      rainy: weather.points.some((point) => isRainyCode(point.weather_code)),
    };
  }, [weather]);

  const checklist = useMemo(() => {
    let maxAltitudeM = 0;
    for (const point of points) {
      if (point.ele !== null && point.ele > maxAltitudeM) maxAltitudeM = point.ele;
    }
    return calculateRequiredGear({
      durationHours: estimatedTimeMin / 60,
      dPlusM: dPlusM(points),
      maxAltitudeM,
      ...weatherContext,
    });
  }, [points, estimatedTimeMin, weatherContext]);

  const items = [...checklist.mandatory, ...checklist.recommended];

  function toggle(id: string) {
    setChecked((previous) => {
      const next = new Set(previous);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function renderItems(list: GearItem[], emptyText: string) {
    if (list.length === 0) return <p className={styles.meta}>{emptyText}</p>;
    return (
      <ul className={styles.checklist}>
        {list.map((item) => (
          <li key={item.id}>
            <label className={styles.checkItem}>
              <input
                type="checkbox"
                checked={checked.has(item.id)}
                onChange={() => toggle(item.id)}
              />
              <span className={styles.checkName}>{item.name}</span>
              <span className={styles.checkReason}>{item.reason}</span>
            </label>
          </li>
        ))}
      </ul>
    );
  }

  return (
    <div className={styles.stack}>
      <p className={styles.meta}>
        Marcados: {checked.size} / {items.length}
      </p>

      <section aria-label="Equipo obligatorio">
        <h4 className={styles.subheading}>Equipo obligatorio</h4>
        {renderItems(checklist.mandatory, 'Sin equipo obligatorio adicional.')}
      </section>

      <section aria-label="Equipo recomendado">
        <h4 className={styles.subheading}>Equipo recomendado</h4>
        {renderItems(checklist.recommended, 'Sin equipo recomendado adicional.')}
      </section>
    </div>
  );
}
