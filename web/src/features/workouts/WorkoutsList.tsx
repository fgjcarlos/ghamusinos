import type { Workout } from '../../lib/api/workout-types';
import { Button, Chip } from '../../ui';
import { groupWorkouts } from './workout-model';
import styles from './Workouts.module.css';

const typeLabels: Record<Workout['workout_type'], string> = {
  rodaje: 'Rodaje',
  tirada_larga: 'Tirada larga',
  series: 'Series',
  cuestas: 'Cuestas',
  tempo: 'Tempo',
  fartlek: 'Fartlek',
  fuerza: 'Fuerza',
  recuperacion: 'Recuperación',
  otro: 'Otro',
};
const dayLabel = (date: string) =>
  new Intl.DateTimeFormat('es-ES', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(new Date(`${date}T00:00:00Z`));

export interface WorkoutsListProps {
  workouts: Workout[];
  onEdit: (workout: Workout) => void;
  onDelete: (workout: Workout) => void;
  busy: boolean;
  hasNext: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
}

export function WorkoutsList({
  workouts,
  onEdit,
  onDelete,
  busy,
  hasNext,
  loadingMore,
  onLoadMore,
}: WorkoutsListProps) {
  return (
    <div className={styles.groups}>
      {groupWorkouts(workouts).map((group) => (
        <section className={styles.day} key={group.date} aria-labelledby={`day-${group.date}`}>
          <h2 id={`day-${group.date}`} className={styles.dayTitle}>
            {dayLabel(group.date)}
          </h2>
          <ul className={styles.sessions}>
            {group.items.map((item) => (
              <li key={item.id} className={styles.session}>
                <div className={styles.sessionHeading}>
                  <h3>{item.title}</h3>
                  <Chip tone="accent">{typeLabels[item.workout_type]}</Chip>
                </div>
                {item.notes && <p className={styles.notes}>{item.notes}</p>}
                <ul className={styles.targets} aria-label={`Objetivos de ${item.title}`}>
                  {item.target_duration_sec !== null && (
                    <li>Duración: {item.target_duration_sec} s</li>
                  )}
                  {item.target_distance_m !== null && (
                    <li>Distancia: {item.target_distance_m} m</li>
                  )}
                  {item.target_elevation_gain_m !== null && (
                    <li>D+: {item.target_elevation_gain_m} m</li>
                  )}
                  {item.target_intensity !== null && (
                    <li>Intensidad: {item.target_intensity.toUpperCase()}</li>
                  )}
                </ul>
                <div className={styles.sessionActions}>
                  <Button variant="secondary" disabled={busy} onClick={() => onEdit(item)}>
                    Editar {item.title}
                  </Button>
                  <Button variant="destructive" disabled={busy} onClick={() => onDelete(item)}>
                    Borrar {item.title}
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        </section>
      ))}
      {hasNext && (
        <Button disabled={busy || loadingMore} onClick={onLoadMore}>
          {loadingMore ? 'Cargando…' : 'Cargar más'}
        </Button>
      )}
    </div>
  );
}

export function WorkoutsSkeleton() {
  return (
    <div className={styles.skeleton} role="status" aria-label="Cargando entrenos" aria-busy="true">
      <span />
      <span />
      <span />
    </div>
  );
}
