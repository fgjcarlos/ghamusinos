// AIAnalysisControl: control reutilizable del análisis opcional con IA
// (fase 1.5, issue #17, T8 — S3/S4/S7).
//
// El control es la única puerta de la web hacia un análisis de IA:
//
//   1. Lee el estado global con fetchAIStatus antes de nada (S3).
//   2. Sólo pide el análisis si la IA está disponible Y el usuario ha
//      consentido. Con la IA apagada, no disponible o sin consentimiento no
//      sale ninguna petición: el control explica qué falta y dónde se
//      resuelve (el perfil), y el resto de la aplicación sigue igual (S7).
//   3. Muestra el estado pendiente mientras el job corre.
//   4. Pinta la salida validada (resumen, observaciones, precauciones) o el
//      mensaje y el código de error seguros.
//
// Nunca se pinta el payload enviado al proveedor, la credencial, el modelo
// ni el contenido crudo del proveedor: sólo la salida validada y el código
// de error seguro.
//
// El control es "con estado" (lee el estado, pide y sondea); las pantallas
// sólo lo montan con el objetivo que quieren analizar.

import { useCallback, useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  createAIAnalysis,
  fetchAIAnalysis,
  fetchAIStatus,
  type AIAnalysis,
  type AIAnalysisKind,
  type AIAnalysisRequest,
  type AIStatusResponse,
} from '../../../lib/api/ai';
import { ApiError } from '../../../lib/api/types';
import { AI_MAX_POLL_ATTEMPTS, AI_POLL_INTERVAL_MS } from './constants';
import styles from './AIAnalysisControl.module.css';

/** Ruta del perfil: es donde vive el aviso de transferencia y la casilla
 *  de consentimiento. */
const PROFILE_PATH = '/perfil';

export interface AIAnalysisControlProps {
  /** Qué análisis se pide: `activity`, `weekly` o `load`. */
  kind: AIAnalysisKind;
  /** UUID de la actividad (sólo con `kind="activity"`). */
  activityId?: string;
  /** Inicio de la ventana en YYYY-MM-DD (sólo con `weekly` o `load`). */
  windowStart?: string;
  /** Longitud de la ventana en días (sólo con `weekly` o `load`). */
  windowDays?: number;
  /** Título del control. */
  title: string;
  /** Texto del botón que pide el análisis. */
  actionLabel?: string;
  /** Cuando es true, el control pide el análisis en cuanto sabe que la IA
   *  está disponible y consentida (el usuario ya ha pulsado "Analizar"). */
  autoStart?: boolean;
  /** Token de autenticación. Por defecto, `VITE_AUTH_TOKEN`. */
  token?: string;
}

/** Los estados por los que pasa el control. */
type ControlState =
  | { phase: 'status' }
  | { phase: 'unavailable'; status: AIStatusResponse }
  | { phase: 'not-consented' }
  | { phase: 'status-error' }
  | { phase: 'ready' }
  | { phase: 'requesting' }
  | { phase: 'pending'; analysis: AIAnalysis; attempt: number }
  | { phase: 'completed'; analysis: AIAnalysis }
  | { phase: 'failed'; analysis: AIAnalysis }
  | { phase: 'request-error'; message: string; status: number | null }
  | { phase: 'poll-error' }
  | { phase: 'timeout' };

/**
 * buildRequest arma el cuerpo de la petición sin mezclar los campos de una
 * actividad con los de una ventana (el backend rechaza la mezcla).
 */
function buildRequest(
  kind: AIAnalysisKind,
  activityId: string | undefined,
  windowStart: string | undefined,
  windowDays: number | undefined,
): AIAnalysisRequest {
  if (kind === 'activity') {
    return { kind: 'activity', activity_id: activityId ?? '' };
  }
  const request: AIAnalysisRequest = { kind };
  if (windowStart !== undefined) request.window_start = windowStart;
  if (windowDays !== undefined) request.window_days = windowDays;
  return request;
}

/**
 * describeRequestError traduce un fallo de la API a un mensaje que puede
 * entender quien entrena: nunca enseña el detalle técnico ni el cuerpo del
 * proveedor.
 */
function describeRequestError(error: unknown): { message: string; status: number | null } {
  if (error instanceof ApiError) {
    if (error.status === 403) {
      return {
        status: 403,
        message:
          'Antes de pedir un análisis tienes que aceptar el envío de tus datos de entrenamiento al proveedor de IA.',
      };
    }
    if (error.status === 404) {
      return { status: 404, message: 'No se ha encontrado la actividad que quieres analizar.' };
    }
    if (error.status === 503) {
      return { status: 503, message: 'La IA no está disponible en este momento.' };
    }
  }
  return {
    status: null,
    message: 'No se pudo pedir el análisis. Inténtalo de nuevo en un momento.',
  };
}

export function AIAnalysisControl({
  kind,
  activityId,
  windowStart,
  windowDays,
  title,
  actionLabel = 'Analizar con IA',
  autoStart = false,
  token: tokenProp,
}: AIAnalysisControlProps) {
  const token = tokenProp ?? import.meta.env.VITE_AUTH_TOKEN ?? '';
  const [state, setState] = useState<ControlState>({ phase: 'status' });
  // El arranque automático sólo ocurre una vez por montaje.
  const autoStarted = useRef(false);

  const request = useCallback(async () => {
    setState({ phase: 'requesting' });
    try {
      const analysis = await createAIAnalysis(
        token,
        buildRequest(kind, activityId, windowStart, windowDays),
      );
      if (analysis.status === 'completed') {
        setState({ phase: 'completed', analysis });
      } else if (analysis.status === 'failed') {
        setState({ phase: 'failed', analysis });
      } else {
        setState({ phase: 'pending', analysis, attempt: 0 });
      }
    } catch (error: unknown) {
      const described = describeRequestError(error);
      setState({ phase: 'request-error', message: described.message, status: described.status });
    }
  }, [token, kind, activityId, windowStart, windowDays]);

  // 1. El estado global, antes de ofrecer nada.
  useEffect(() => {
    let cancelled = false;
    setState({ phase: 'status' });
    if (!token) {
      setState({ phase: 'status-error' });
      return;
    }
    fetchAIStatus(token)
      .then((status) => {
        if (cancelled) return;
        if (!status.available) {
          setState({ phase: 'unavailable', status });
          return;
        }
        if (!status.consented) {
          setState({ phase: 'not-consented' });
          return;
        }
        setState({ phase: 'ready' });
        if (autoStart && !autoStarted.current) {
          autoStarted.current = true;
          void request();
        }
      })
      .catch(() => {
        if (cancelled) return;
        setState({ phase: 'status-error' });
      });
    return () => {
      cancelled = true;
    };
  }, [token, autoStart, request]);

  // 2. El sondeo del análisis mientras el job corre.
  useEffect(() => {
    if (state.phase !== 'pending') return;
    if (state.attempt >= AI_MAX_POLL_ATTEMPTS) {
      setState({ phase: 'timeout' });
      return;
    }
    let cancelled = false;
    const poll = () => {
      fetchAIAnalysis(token, state.analysis.kind, state.analysis.scope)
        .then((analysis) => {
          if (cancelled) return;
          if (analysis.status === 'completed') {
            setState({ phase: 'completed', analysis });
          } else if (analysis.status === 'failed') {
            setState({ phase: 'failed', analysis });
          } else {
            setState({ phase: 'pending', analysis, attempt: state.attempt + 1 });
          }
        })
        .catch(() => {
          if (cancelled) return;
          setState({ phase: 'poll-error' });
        });
    };
    if (state.attempt === 0) {
      poll();
      return () => {
        cancelled = true;
      };
    }
    const timer = setTimeout(poll, AI_POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [state, token]);

  const requestButton = (label: string, testId = 'ai-analysis-request') => (
    <button
      type="button"
      className={styles.request}
      data-testid={testId}
      onClick={() => void request()}
    >
      {label}
    </button>
  );

  const profileLink = () => (
    <Link className={styles.link} to={PROFILE_PATH}>
      Ir a mi perfil
    </Link>
  );

  const body = () => {
    switch (state.phase) {
      case 'status':
        return (
          <p className={styles.notice} role="status" data-testid="ai-analysis-status-loading">
            Comprobando si el análisis con IA está disponible…
          </p>
        );
      case 'unavailable':
        return (
          <div className={styles.notice} role="status" data-testid="ai-analysis-unavailable">
            <p>
              {state.status.status === 'disabled'
                ? 'La IA está desactivada. Puedes activarla desde tu perfil.'
                : 'La IA no está disponible en este momento. El resto de la aplicación sigue funcionando igual.'}
            </p>
            {profileLink()}
          </div>
        );
      case 'not-consented':
        return (
          <div className={styles.notice} role="status" data-testid="ai-analysis-not-consented">
            <p>
              Antes de pedir un análisis tienes que aceptar el envío de tus datos de entrenamiento
              al proveedor de IA. Puedes hacerlo desde tu perfil.
            </p>
            {profileLink()}
          </div>
        );
      case 'status-error':
        return (
          <p className={styles.notice} role="status" data-testid="ai-analysis-status-error">
            No se pudo comprobar si el análisis con IA está disponible. El resto de la aplicación
            sigue funcionando igual.
          </p>
        );
      case 'ready':
        return requestButton(actionLabel);
      case 'requesting':
        return (
          <p className={styles.notice} role="status" data-testid="ai-analysis-requesting">
            Enviando la solicitud…
          </p>
        );
      case 'pending':
        return (
          <p className={styles.notice} role="status" data-testid="ai-analysis-pending">
            Analizando con IA…
          </p>
        );
      case 'completed': {
        const output = state.analysis.output;
        return (
          <div className={styles.result} data-testid="ai-analysis-result">
            {output ? (
              <>
                <p className={styles.summary} data-testid="ai-analysis-summary">
                  {output.summary}
                </p>
                {output.observations.length > 0 ? (
                  <>
                    <h4 className={styles.subtitle}>Observaciones</h4>
                    <ul className={styles.list} data-testid="ai-analysis-observations">
                      {output.observations.map((observation) => (
                        <li key={observation}>{observation}</li>
                      ))}
                    </ul>
                  </>
                ) : null}
                {output.cautions.length > 0 ? (
                  <>
                    <h4 className={styles.subtitle}>Precauciones</h4>
                    <ul className={styles.list} data-testid="ai-analysis-cautions">
                      {output.cautions.map((caution) => (
                        <li key={caution}>{caution}</li>
                      ))}
                    </ul>
                  </>
                ) : null}
              </>
            ) : null}
            {requestButton('Volver a analizar', 'ai-analysis-retry')}
          </div>
        );
      }
      case 'failed':
        return (
          <div className={styles.failed} role="alert" data-testid="ai-analysis-failed">
            <p>No se pudo completar el análisis con IA.</p>
            {state.analysis.error_message ? (
              <p data-testid="ai-analysis-failed-message">{state.analysis.error_message}</p>
            ) : null}
            {state.analysis.error_code ? (
              <p className={styles.code} data-testid="ai-analysis-failed-code">
                Código: {state.analysis.error_code}
              </p>
            ) : null}
            {requestButton('Volver a intentarlo', 'ai-analysis-retry')}
          </div>
        );
      case 'request-error':
        return (
          <div className={styles.failed} role="alert" data-testid="ai-analysis-request-error">
            <p>{state.message}</p>
            {state.status === 403 ? profileLink() : null}
            {requestButton('Volver a intentarlo', 'ai-analysis-retry')}
          </div>
        );
      case 'poll-error':
        return (
          <div className={styles.failed} role="alert" data-testid="ai-analysis-poll-error">
            <p>Se perdió el seguimiento del análisis. Inténtalo de nuevo en un momento.</p>
            {requestButton('Volver a intentarlo', 'ai-analysis-retry')}
          </div>
        );
      case 'timeout':
        return (
          <div className={styles.failed} role="alert" data-testid="ai-analysis-timeout">
            <p>El análisis está tardando más de lo normal. Inténtalo de nuevo en un momento.</p>
            {requestButton('Volver a intentarlo', 'ai-analysis-retry')}
          </div>
        );
    }
  };

  return (
    <section className={styles.control} aria-label={title} data-testid="ai-analysis-control">
      <h3 className={styles.title}>{title}</h3>
      {body()}
    </section>
  );
}
