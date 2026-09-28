// PreferencesContainer: page-level container for the /perfil route.
// Issue 159. Owns the GET + PATCH round-trip, local form state, and
// the loading/error/saving state machine. Renders PreferencesForm +
// HRZonePreview + ConnectionCard.

import { useEffect, useState } from 'react';
import { getPreferences, patchPreferences } from '../../lib/api/profile';
import { ApiError } from '../../lib/api/types';
import { PreferencesForm, type PreferencesFormValues } from './PreferencesForm';
import { HRZonePreview } from './HRZonePreview';
import { ConnectionCard, type StravaConnectionState } from './ConnectionCard';
import styles from './PreferencesContainer.module.css';

export interface PreferencesContainerProps {
  /** Mock or real Strava connection state. Container doesn't fetch it. */
  strava: StravaConnectionState;
  /** Mock or real busy flag for Strava connection. */
  stravaBusy?: boolean;
}

const EMPTY: PreferencesFormValues = {
  hr_max: null,
  lthr: null,
  ftp: null,
  level: null,
  timezone: '',
  ai_enabled: false,
};

type SaveStatus = 'idle' | 'saving' | 'saved' | 'error';

export function PreferencesContainer({ strava, stravaBusy }: PreferencesContainerProps) {
  const [values, setValues] = useState<PreferencesFormValues>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string | undefined>>({});
  const [warning, setWarning] = useState<string | undefined>(undefined);
  const [status, setStatus] = useState<SaveStatus>('idle');

  useEffect(() => {
    const token = import.meta.env.VITE_AUTH_TOKEN;
    if (!token) {
      // No token: leave defaults. The /perfil route should not be
      // reachable without auth; this is a defensive fallback.
      return;
    }
    let cancelled = false;
    getPreferences(token)
      .then((data) => {
        if (cancelled) return;
        setValues({
          hr_max: data.hr_max ?? null,
          lthr: data.lthr ?? null,
          ftp: data.ftp ?? null,
          level: (data.level ?? null) as PreferencesFormValues['level'],
          timezone: data.timezone,
          ai_enabled: data.ai_enabled,
        });
      })
      .catch(() => {
        // Swallow fetch errors silently — the form shows defaults and
        // the user can still edit and try to save.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function onChange<K extends keyof PreferencesFormValues>(
    key: K,
    value: PreferencesFormValues[K],
  ) {
    setValues((prev) => ({ ...prev, [key]: value }));
    setStatus('idle');
    // Clear any per-field error on edit.
    setErrors((prev) => ({ ...prev, [key]: undefined }));
  }

  function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const token = import.meta.env.VITE_AUTH_TOKEN;
    if (!token) {
      setErrors({ timezone: 'Falta token de autenticación' });
      return;
    }
    setStatus('saving');
    setErrors({});
    setWarning(undefined);
    patchPreferences(token, values)
      .then((res) => {
        setStatus('saved');
        setWarning(res.warning);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 422) {
          // Surface field-level errors from the problem detail.
          const detail = err.message;
          setErrors({ timezone: detail });
          setStatus('error');
        } else {
          setStatus('error');
          setErrors({});
        }
      });
  }

  return (
    <section className={styles.container}>
      <header className={styles.header}>
        <h1 className={styles.heading}>Perfil</h1>
        {status === 'saved' && (
          <p className={styles.statusOk} role="status" data-testid="status-saved">
            Preferencias guardadas.
          </p>
        )}
        {status === 'error' && (
          <p className={styles.statusErr} role="alert" data-testid="status-error">
            No se pudieron guardar las preferencias.
          </p>
        )}
      </header>

      <PreferencesForm
        values={values}
        errors={errors}
        warning={warning}
        busy={status === 'saving'}
        onChange={onChange}
        onSubmit={onSubmit}
      />

      <div className={styles.row}>
        <HRZonePreview hr_max={values.hr_max} />
        <ConnectionCard state={strava} busy={stravaBusy} />
      </div>
    </section>
  );
}
