// BlocksPanel: create, edit and delete training blocks (fase 2.4, #317).
// The list itself is loaded by the page because the block selector of the
// generate form reads the same rows.

import { useCallback, useState } from 'react';
import { deleteBlock, saveBlock, type Block, type BlockInput } from '../../lib/api/blocks';
import { listRaceOptions } from '../../lib/api/races';
import { ApiError } from '../../lib/api/types';
import { Button } from '../../ui';
import { BlockForm } from './BlockForm';
import { useReportsData } from './useReports';
import styles from './Reports.module.css';

const describeFailure = (failure: unknown) => {
  const status = failure instanceof ApiError ? failure.status : 0;
  if (status === 409) return 'El bloque se solapa con otro bloque existente.';
  if (status === 422) return 'Revisa los datos: el nombre y el rango de fechas no son válidos.';
  if (status === 404) return 'La carrera elegida no existe. Revisa el bloque.';
  return 'No se pudo guardar el bloque.';
};

export function BlocksPanel({
  auth,
  blocks,
  onChanged,
}: {
  auth: string;
  blocks: Block[];
  onChanged: () => void;
}) {
  const loadRaces = useCallback((signal: AbortSignal) => listRaceOptions(auth, signal), [auth]);
  const races = useReportsData(loadRaces);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<Block | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function save(input: BlockInput) {
    setBusy(true);
    setError('');
    try {
      await saveBlock(auth, input, editing?.id);
      setCreating(false);
      setEditing(null);
      onChanged();
    } catch (failure) {
      setError(describeFailure(failure));
    } finally {
      setBusy(false);
    }
  }

  async function remove(block: Block) {
    if (!window.confirm(`¿Borrar el bloque ${block.name}? Su reporte también se borrará.`)) return;
    setBusy(true);
    setError('');
    try {
      await deleteBlock(auth, block.id);
      onChanged();
    } catch {
      setError('No se pudo borrar el bloque.');
    } finally {
      setBusy(false);
    }
  }

  const raceName = (id: string | null) =>
    id && races.result.phase === 'ready'
      ? (races.result.data.find((race) => race.id === id)?.name ?? null)
      : null;
  const form = creating || editing !== null;

  return (
    <>
      {error && !form && (
        <p role="alert" className={styles.error}>
          {error}
        </p>
      )}
      {blocks.length === 0 && !form && <p>Todavía no hay bloques.</p>}
      {blocks.length > 0 && (
        <ul className={styles.rows}>
          {blocks.map((block) => (
            <li key={block.id}>
              <div className={styles.header}>
                <strong>{block.name}</strong>
                <div className={styles.actions}>
                  <Button
                    variant="secondary"
                    disabled={busy}
                    onClick={() => {
                      setCreating(false);
                      setEditing(block);
                      setError('');
                    }}
                  >
                    Editar bloque<span className={styles.srOnly}> {block.name}</span>
                  </Button>
                  <Button variant="destructive" disabled={busy} onClick={() => void remove(block)}>
                    Borrar bloque<span className={styles.srOnly}> {block.name}</span>
                  </Button>
                </div>
              </div>
              <div className={styles.metadata}>
                <time dateTime={block.start_date}>{block.start_date}</time>
                <span>→</span>
                <time dateTime={block.end_date}>{block.end_date}</time>
                {raceName(block.race_id) && <span>Carrera: {raceName(block.race_id)}</span>}
              </div>
            </li>
          ))}
        </ul>
      )}
      {form ? (
        <BlockForm
          {...(editing ? { initial: editing } : {})}
          races={races.result.phase === 'ready' ? races.result.data : []}
          busy={busy}
          error={error}
          onSave={(input) => void save(input)}
          onCancel={() => {
            setCreating(false);
            setEditing(null);
            setError('');
          }}
        />
      ) : (
        <Button
          onClick={() => {
            setCreating(true);
            setError('');
          }}
        >
          Nuevo bloque
        </Button>
      )}
    </>
  );
}
