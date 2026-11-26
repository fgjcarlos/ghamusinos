import { useCallback, useState, type FormEvent } from 'react';
import type {
  Workout,
  WorkoutInput,
  WorkoutType,
  WorkoutIntensity,
} from '../../lib/api/workout-types';
import { listRaceOptions } from '../../lib/api/races';
import { useRaceData } from '../races/useRaceData';
import { Button, Field } from '../../ui';
import { validateWorkoutValues, workoutValuesFromForm } from './workout-model';
import styles from './Workouts.module.css';

const types: WorkoutType[] = [
  'rodaje',
  'tirada_larga',
  'series',
  'cuestas',
  'tempo',
  'fartlek',
  'fuerza',
  'recuperacion',
  'otro',
];
const targets = {
  target_duration_sec: 'Duración (segundos)',
  target_distance_m: 'Distancia (metros)',
  target_elevation_gain_m: 'Desnivel positivo (metros)',
} as const;

export interface WorkoutFormProps {
  /** Prefill; the calendar (#313) passes only the day to create from. */
  initial?: Partial<Workout>;
  onSave: (value: WorkoutInput) => Promise<void>;
  onCancel: () => void;
  busy: boolean;
  dateFrom?: string;
  dateTo?: string;
}

export function WorkoutForm({
  initial,
  onSave,
  onCancel,
  busy,
  dateFrom,
  dateTo,
}: WorkoutFormProps) {
  const [errors, setErrors] = useState<Record<string, string>>({});
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const values = workoutValuesFromForm(form);
    const validation = validateWorkoutValues(values);
    for (const input of form.querySelectorAll<HTMLInputElement>('input[type="number"]')) {
      if (input.validity.badInput) validation[input.name] = 'Introduce un número válido.';
    }
    if (
      !validation.scheduled_on &&
      ((dateFrom && values.scheduled_on < dateFrom) || (dateTo && values.scheduled_on > dateTo))
    )
      validation.scheduled_on = `La fecha debe estar entre ${dateFrom} y ${dateTo}.`;
    setErrors(validation);
    if (Object.keys(validation).length) return;
    const numberOrNull = (key: keyof typeof targets) =>
      values[key] === '' ? null : Number(values[key]);
    const race = new FormData(form).get('race_id');
    await onSave({
      scheduled_on: values.scheduled_on,
      title: values.title,
      notes: values.notes,
      workout_type: values.workout_type as WorkoutType,
      target_duration_sec: numberOrNull('target_duration_sec'),
      target_distance_m: numberOrNull('target_distance_m'),
      target_elevation_gain_m: numberOrNull('target_elevation_gain_m'),
      target_intensity: (values.target_intensity || null) as WorkoutIntensity | null,
      ...(typeof race === 'string' && race !== (initial?.race_id ?? '')
        ? { race_id: race || null }
        : {}),
    });
  }
  const errorProps = (name: string) => ({
    error: errors[name],
    errorId: errors[name] ? `${name}-error` : undefined,
  });
  return (
    <form className={styles.form} onSubmit={submit} noValidate aria-busy={busy}>
      <fieldset disabled={busy} className={styles.fieldset}>
        <Field label="Fecha" {...errorProps('scheduled_on')}>
          <input
            name="scheduled_on"
            type="date"
            required
            defaultValue={initial?.scheduled_on ?? ''}
            aria-invalid={!!errors.scheduled_on}
            aria-describedby={errors.scheduled_on ? 'scheduled_on-error' : undefined}
          />
        </Field>
        <Field label="Título" {...errorProps('title')}>
          <input
            name="title"
            required
            defaultValue={initial?.title ?? ''}
            aria-invalid={!!errors.title}
            aria-describedby={errors.title ? 'title-error' : undefined}
          />
        </Field>
        <Field label="Tipo" {...errorProps('workout_type')}>
          <select
            name="workout_type"
            defaultValue={initial?.workout_type ?? 'rodaje'}
            aria-invalid={!!errors.workout_type}
            aria-describedby={errors.workout_type ? 'workout_type-error' : undefined}
          >
            {types.map((type) => (
              <option key={type} value={type}>
                {type.replace('_', ' ')}
              </option>
            ))}
          </select>
        </Field>
        <WorkoutRaceField initialId={initial?.race_id ?? ''} />
        <Field label="Notas" {...errorProps('notes')}>
          <textarea
            name="notes"
            defaultValue={initial?.notes ?? ''}
            aria-invalid={!!errors.notes}
            aria-describedby={errors.notes ? 'notes-error' : undefined}
          />
        </Field>
        {Object.entries(targets).map(([key, label]) => (
          <Field key={key} label={label} {...errorProps(key)}>
            <input
              name={key}
              type="number"
              min="0"
              max="2147483647"
              step="1"
              defaultValue={(initial?.[key as keyof Workout] as number | null) ?? ''}
              aria-invalid={!!errors[key]}
              aria-describedby={errors[key] ? `${key}-error` : undefined}
            />
          </Field>
        ))}
        <Field label="Intensidad" {...errorProps('target_intensity')}>
          <select
            name="target_intensity"
            defaultValue={initial?.target_intensity ?? ''}
            aria-invalid={!!errors.target_intensity}
            aria-describedby={errors.target_intensity ? 'target_intensity-error' : undefined}
          >
            <option value="">Sin objetivo</option>
            {['z1', 'z2', 'z3', 'z4', 'z5'].map((zone) => (
              <option key={zone} value={zone}>
                {zone.toUpperCase()}
              </option>
            ))}
          </select>
        </Field>
        <div className={styles.formActions}>
          <Button type="submit" disabled={busy}>
            {busy ? 'Guardando…' : 'Guardar'}
          </Button>
          <Button type="button" variant="secondary" disabled={busy} onClick={onCancel}>
            Cancelar
          </Button>
        </div>
      </fieldset>
    </form>
  );
}

function WorkoutRaceField({ initialId }: { initialId: string }) {
  const token = import.meta.env.VITE_AUTH_TOKEN || '';
  const load = useCallback(
    (signal: AbortSignal) => (token ? listRaceOptions(token, signal) : Promise.resolve([])),
    [token],
  );
  const { result, retry } = useRaceData(load);
  const [choice, setChoice] = useState(initialId);
  const races = result.phase === 'ready' ? result.data : [];
  return (
    <div>
      <Field label="Carrera objetivo">
        <select
          name="race_id"
          value={choice}
          disabled={result.phase !== 'ready'}
          onChange={(event) => setChoice(event.target.value)}
        >
          <option value="">Sin carrera</option>
          {initialId && !races.some(({ id }) => id === initialId) && (
            <option value={initialId}>Carrera vinculada</option>
          )}
          {races.map((race) => (
            <option key={race.id} value={race.id}>
              {race.name} ({race.race_date})
            </option>
          ))}
        </select>
      </Field>
      {result.phase === 'loading' && <small role="status">Cargando carreras objetivo…</small>}
      {result.phase === 'error' && (
        <small role="status">
          No se pudieron cargar las carreras.{' '}
          <button type="button" onClick={retry}>
            Reintentar
          </button>
        </small>
      )}
    </div>
  );
}
