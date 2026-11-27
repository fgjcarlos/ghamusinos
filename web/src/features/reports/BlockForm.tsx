import type { FormEvent } from 'react';
import type { Block, BlockInput } from '../../lib/api/blocks';
import type { Race } from '../../lib/api/races';
import { Button, Field } from '../../ui';
import styles from './Reports.module.css';

export function BlockForm({
  initial,
  races,
  busy,
  error,
  onSave,
  onCancel,
}: {
  initial?: Block;
  races: Race[];
  busy: boolean;
  error: string;
  onSave: (input: BlockInput) => void;
  onCancel: () => void;
}) {
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const race = String(form.get('race_id') ?? '');
    onSave({
      name: String(form.get('name')).trim(),
      start_date: String(form.get('start_date')),
      end_date: String(form.get('end_date')),
      race_id: race === '' ? null : race,
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
        <Field label="Inicio">
          <input
            name="start_date"
            type="date"
            required
            max="9999-12-31"
            defaultValue={initial?.start_date ?? ''}
          />
        </Field>
        <Field label="Fin">
          <input
            name="end_date"
            type="date"
            required
            max="9999-12-31"
            defaultValue={initial?.end_date ?? ''}
          />
        </Field>
        <Field label="Carrera objetivo">
          <select name="race_id" defaultValue={initial?.race_id ?? ''}>
            <option value="">Sin carrera</option>
            {races.map((race) => (
              <option key={race.id} value={race.id}>
                {race.name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Notas">
          <textarea name="notes" maxLength={2000} defaultValue={initial?.notes ?? ''} />
        </Field>
      </fieldset>
      {error && (
        <p role="alert" className={styles.error}>
          {error}
        </p>
      )}
      <div className={styles.actions}>
        <Button type="submit" disabled={busy}>
          Guardar bloque
        </Button>
        <Button type="button" variant="secondary" disabled={busy} onClick={onCancel}>
          Cancelar
        </Button>
      </div>
    </form>
  );
}
