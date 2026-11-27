import { useCallback } from 'react';
import { getReport } from '../../lib/api/reports';
import { Button } from '../../ui';
import { format, km, metres } from '../races/race-format';
import { DailyLoadChart } from './DailyLoadChart';
import { ef, pace, percent, periodLabel } from './report-format';
import { useReportsData } from './useReports';
import styles from './Reports.module.css';

export function ReportDetailPanel({ auth, id }: { auth: string; id: string }) {
  const load = useCallback((signal: AbortSignal) => getReport(auth, id, signal), [auth, id]);
  const { result, refresh } = useReportsData(load);
  const payload = result.phase === 'ready' ? result.data.payload : null;

  return (
    <section aria-labelledby="report-detail-title">
      <h2 id="report-detail-title">Detalle del reporte</h2>
      <p>
        Métricas orientativas: EF y GAP se recalculan a partir de las actividades y el volumen
        registrado no predice el rendimiento.
      </p>
      {result.phase === 'loading' && <p role="status">Cargando detalle…</p>}
      {result.phase === 'error' && (
        <div>
          <p role="alert">No se pudo cargar el detalle del reporte.</p>
          <Button variant="secondary" onClick={refresh}>
            Reintentar detalle
          </Button>
        </div>
      )}
      {payload && result.phase === 'ready' && (
        <>
          <h3>{periodLabel(payload.period_start, payload.period_end)}</h3>
          <p className={styles.metadata}>
            {payload.days} días
            {payload.block && ` · ${payload.block.name}`}
            {payload.race && ` · ${payload.race.name} (${payload.race.days_to_race} días)`}
          </p>
          <h3>Carga y fatiga</h3>
          <dl className={styles.stats}>
            <div>
              <dt>TSS</dt>
              <dd>{format.format(payload.load.tss)}</dd>
            </div>
            <div>
              <dt>CTL al inicio</dt>
              <dd>{format.format(payload.load.ctl_start)}</dd>
            </div>
            <div>
              <dt>ATL al inicio</dt>
              <dd>{format.format(payload.load.atl_start)}</dd>
            </div>
            <div>
              <dt>TSB al inicio</dt>
              <dd>{format.format(payload.load.tsb_start)}</dd>
            </div>
            <div>
              <dt>CTL al cierre</dt>
              <dd>{format.format(payload.load.ctl_end)}</dd>
            </div>
            <div>
              <dt>ATL al cierre</dt>
              <dd>{format.format(payload.load.atl_end)}</dd>
            </div>
            <div>
              <dt>TSB al cierre</dt>
              <dd>{format.format(payload.load.tsb_end)}</dd>
            </div>
          </dl>
          <DailyLoadChart daily={payload.load.daily} />
          <h3>Volumen</h3>
          <dl className={styles.stats}>
            <div>
              <dt>Actividades</dt>
              <dd>{payload.volume.activities}</dd>
            </div>
            <div>
              <dt>Distancia</dt>
              <dd>{km(payload.volume.distance_m)}</dd>
            </div>
            <div>
              <dt>Desnivel positivo</dt>
              <dd>{metres(payload.volume.elevation_gain_m)}</dd>
            </div>
          </dl>
          <h3>Rendimiento orientativo</h3>
          <dl className={styles.stats}>
            <div>
              <dt>EF medio</dt>
              <dd>{ef(payload.performance.avg_ef)}</dd>
            </div>
            <div>
              <dt>Muestras de EF</dt>
              <dd>{payload.performance.ef_samples}</dd>
            </div>
            <div>
              <dt>GAP medio</dt>
              <dd>{pace(payload.performance.avg_gap_sec_per_km)}</dd>
            </div>
            <div>
              <dt>Muestras de GAP</dt>
              <dd>{payload.performance.gap_samples}</dd>
            </div>
          </dl>
          <h3>Cumplimiento del plan</h3>
          <dl className={styles.stats}>
            <div>
              <dt>Planificadas</dt>
              <dd>{payload.plan.planned}</dd>
            </div>
            <div>
              <dt>Completadas</dt>
              <dd>{payload.plan.completed}</dd>
            </div>
            <div>
              <dt>Omitidas</dt>
              <dd>{payload.plan.skipped}</dd>
            </div>
            <div>
              <dt>Modificadas</dt>
              <dd>{payload.plan.modified}</dd>
            </div>
            <div>
              <dt>Cumplimiento</dt>
              <dd>{percent(payload.plan.adherence)}</dd>
            </div>
          </dl>
          <h3>Comparación con el periodo anterior</h3>
          <dl className={styles.stats}>
            <div>
              <dt>Periodo</dt>
              <dd>{periodLabel(payload.previous.period_start, payload.previous.period_end)}</dd>
            </div>
            <div>
              <dt>TSS</dt>
              <dd>{format.format(payload.previous.tss)}</dd>
            </div>
            <div>
              <dt>Actividades</dt>
              <dd>{payload.previous.activities}</dd>
            </div>
            <div>
              <dt>Distancia</dt>
              <dd>{km(payload.previous.distance_m)}</dd>
            </div>
            <div>
              <dt>Desnivel positivo</dt>
              <dd>{metres(payload.previous.elevation_gain_m)}</dd>
            </div>
          </dl>
        </>
      )}
    </section>
  );
}
