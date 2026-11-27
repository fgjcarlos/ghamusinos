// /reportes: weekly, monthly and per-block reports (fase 2.4, #320).
// The selector only enqueues the generation (202); the list refreshes a
// bounded number of times until the snapshot shows up, and the user can
// ask again manually.

import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { listBlocks } from '../../lib/api/blocks';
import {
  generateReport,
  listReports,
  type GenerateReportInput,
  type QueuedReport,
  type ReportKind,
} from '../../lib/api/reports';
import { ApiError } from '../../lib/api/types';
import { Button, EmptyState, Field } from '../../ui';
import { km, metres, format } from '../races/race-format';
import { BlocksPanel } from './BlocksPanel';
import { ReportDetailPanel } from './ReportDetail';
import { ef, kindLabel, localDay, pace, percent, periodLabel } from './report-format';
import { useReportsData } from './useReports';
import styles from './Reports.module.css';

const token = () => import.meta.env.VITE_AUTH_TOKEN || '';
const POLL_MS = 2000;
const MAX_POLLS = 5;
const kindOptions: { value: ReportKind; label: string }[] = [
  { value: 'week', label: 'Semana' },
  { value: 'month', label: 'Mes' },
  { value: 'block', label: 'Bloque' },
];

export function ReportsPage() {
  const auth = token();
  const [kind, setKind] = useState<ReportKind>('week');
  const [date, setDate] = useState(localDay);
  const [blockId, setBlockId] = useState('');
  const [selected, setSelected] = useState<string | null>(null);
  const [pending, setPending] = useState<QueuedReport | null>(null);
  const [polls, setPolls] = useState(0);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState('');

  const loadList = useCallback(
    (signal: AbortSignal) => listReports(auth, kind, signal),
    [auth, kind],
  );
  const { result: list, refresh: refreshList } = useReportsData(loadList);
  const loadBlocks = useCallback((signal: AbortSignal) => listBlocks(auth, 100, 0, signal), [auth]);
  const { result: blockList, refresh: refreshBlocks } = useReportsData(loadBlocks);

  // Counting refreshes (not arrivals) keeps the bound exact: a stale list
  // rendered while the request is in flight spends no attempt.
  useEffect(() => {
    if (!pending || polls >= MAX_POLLS) return;
    const timer = setTimeout(() => {
      setPolls((value) => value + 1);
      refreshList();
    }, POLL_MS);
    return () => clearTimeout(timer);
  }, [pending, polls, refreshList]);

  // The generated report arrives asynchronously: select it as soon as it
  // shows up in the list.
  useEffect(() => {
    if (!pending || list.phase !== 'ready') return;
    const found = list.data.data.find(
      (item) => item.kind === pending.kind && item.period_start === pending.period_start,
    );
    if (!found) return;
    setPending(null);
    setPolls(0);
    setSelected(found.id);
  }, [pending, list]);

  function changeKind(next: ReportKind) {
    setKind(next);
    setSelected(null);
    setPending(null);
    setPolls(0);
    setNotice('');
  }

  async function generate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setNotice('');
    try {
      const input: GenerateReportInput =
        kind === 'block' ? { kind, block_id: blockId } : { kind, date };
      const queued = await generateReport(auth, input);
      setPending(queued);
      setPolls(0);
      refreshList();
    } catch (failure) {
      setNotice(
        failure instanceof ApiError && failure.status === 503
          ? 'La generación no está disponible ahora mismo. Vuelve a intentarlo.'
          : 'No se pudo generar el reporte. Revisa el periodo.',
      );
    } finally {
      setBusy(false);
    }
  }

  const blocks = blockList.phase === 'ready' ? blockList.data.data : [];

  return (
    <section className={styles.page} aria-labelledby="reports-title">
      <header className={styles.header}>
        <h1 id="reports-title">Reportes</h1>
      </header>
      <p>Resumen semanal, mensual y por bloque de tu entrenamiento.</p>

      <section aria-labelledby="reports-period-title">
        <h2 id="reports-period-title">Periodo</h2>
        <form className={styles.form} onSubmit={generate}>
          <Field label="Tipo de periodo">
            <select value={kind} onChange={(event) => changeKind(event.target.value as ReportKind)}>
              {kindOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </Field>
          {kind === 'block' ? (
            <Field label="Bloque">
              <select value={blockId} onChange={(event) => setBlockId(event.target.value)}>
                <option value="">Elige un bloque</option>
                {blocks.map((block) => (
                  <option key={block.id} value={block.id}>
                    {block.name} ({block.start_date} – {block.end_date})
                  </option>
                ))}
              </select>
            </Field>
          ) : (
            <Field label="Día del periodo">
              <input
                type="date"
                value={date}
                required
                max="9999-12-31"
                onChange={(event) => setDate(event.target.value)}
              />
            </Field>
          )}
          <Button type="submit" disabled={busy || (kind === 'block' && !blockId)}>
            Generar reporte
          </Button>
        </form>
        {notice && (
          <p role="alert" className={styles.error}>
            {notice}
          </p>
        )}
        {pending && (
          <p role="status">
            {polls >= MAX_POLLS
              ? `El reporte ${periodLabel(pending.period_start, pending.period_end)} todavía no está listo.`
              : `Generando el reporte ${periodLabel(pending.period_start, pending.period_end)}…`}{' '}
            <Button
              variant="secondary"
              onClick={() => {
                setPolls(0);
                refreshList();
              }}
            >
              Volver a comprobar
            </Button>
          </p>
        )}
      </section>

      <section aria-labelledby="reports-list-title">
        <h2 id="reports-list-title">Reportes de {kindLabel[kind]}</h2>
        {list.phase === 'loading' && <p role="status">Cargando reportes…</p>}
        {list.phase === 'error' && (
          <div>
            <p role="alert">No se pudieron cargar los reportes.</p>
            <Button variant="secondary" onClick={refreshList}>
              Reintentar
            </Button>
          </div>
        )}
        {list.phase === 'ready' &&
          (list.data.data.length === 0 ? (
            <EmptyState message="Todavía no hay reportes de este periodo." />
          ) : (
            <ul className={styles.rows}>
              {list.data.data.map((item) => (
                <li key={item.id}>
                  <button
                    type="button"
                    className={styles.period}
                    onClick={() => setSelected(item.id)}
                  >
                    {periodLabel(item.period_start, item.period_end)}
                  </button>
                  <p className={styles.metadata}>
                    TSS {format.format(item.summary.tss)} · {item.summary.activities} actividades ·{' '}
                    {km(item.summary.distance_m)} · D+ {metres(item.summary.elevation_gain_m)} · EF{' '}
                    {ef(item.summary.avg_ef)} · GAP {pace(item.summary.avg_gap_sec_per_km)} ·
                    Cumplimiento {percent(item.summary.adherence)}
                  </p>
                </li>
              ))}
            </ul>
          ))}
      </section>

      {selected && <ReportDetailPanel key={selected} auth={auth} id={selected} />}

      <section aria-labelledby="blocks-title">
        <h2 id="blocks-title">Bloques de entrenamiento</h2>
        <p>Agrupa semanas de entrenamiento en bloques, con o sin carrera objetivo.</p>
        {blockList.phase === 'loading' && <p role="status">Cargando bloques…</p>}
        {blockList.phase === 'error' && (
          <div>
            <p role="alert">No se pudieron cargar los bloques.</p>
            <Button variant="secondary" onClick={refreshBlocks}>
              Reintentar bloques
            </Button>
          </div>
        )}
        {blockList.phase === 'ready' && (
          <BlocksPanel auth={auth} blocks={blocks} onChanged={refreshBlocks} />
        )}
      </section>
    </section>
  );
}
