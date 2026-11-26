import type { FormEvent } from 'react';
import type { Priority, RaceInput } from '../../lib/api/races';
import { Button, Field } from '../../ui';
import styles from './Races.module.css';

export function RaceForm({
  initial,
  busy,
  onSave,
  onCancel,
}: {
  initial?: RaceInput;
  busy: boolean;
  onSave: (input: RaceInput) => void;
  onCancel: () => void;
}) {
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    onSave({
      name: String(form.get('name')).trim(),
      race_date: String(form.get('race_date')),
      distance_m: Number(form.get('distance_m')),
      elevation_gain_m: Number(form.get('elevation_gain_m')),
      priority: form.get('priority') as Priority,
      notes: String(form.get('notes')),
    });
  }
  return (
    <form onSubmit={submit} aria-busy={busy}>
      <fieldset className={styles.form} disabled={busy}>
        <Field label="Nombre">
          <input
            name="name"
            required
            maxLength={200}
            pattern={String.raw`.*\S.*`}
            defaultValue={initial?.name ?? ''}
          />
        </Field>
        <Field label="Fecha">
          <input
            name="race_date"
            type="date"
            required
            max="9999-12-31"
            defaultValue={initial?.race_date ?? ''}
          />
        </Field>
        <Field label="Distancia (metros)">
          <input
            name="distance_m"
            type="number"
            required
            min="1"
            max="2147483647"
            defaultValue={initial?.distance_m ?? ''}
          />
        </Field>
        <Field label="Desnivel positivo (metros)">
          <input
            name="elevation_gain_m"
            type="number"
            required
            min="0"
            max="2147483647"
            defaultValue={initial?.elevation_gain_m ?? 0}
          />
        </Field>
        <Field label="Prioridad">
          <select name="priority" defaultValue={initial?.priority ?? 'A'}>
            <option value="A">A</option>
            <option value="B">B</option>
            <option value="C">C</option>
          </select>
        </Field>
        <Field label="Notas">
          <textarea name="notes" maxLength={2000} defaultValue={initial?.notes ?? ''} />
        </Field>
        <div className={styles.actions}>
          <Button type="submit">{busy ? 'Guardando…' : 'Guardar carrera'}</Button>
          <Button type="button" variant="secondary" onClick={onCancel}>
            Cancelar
          </Button>
        </div>
      </fieldset>
    </form>
  );
}
