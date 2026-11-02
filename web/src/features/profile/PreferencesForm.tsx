// PreferencesForm: presentational form for user training preferences.
// Issue 159. Pure render — the container owns fetch + PATCH + saving
// state. Errors from the backend land next to the field that failed,
// not in a generic banner.
//
// Phase 1.5 (T2): the AI checkbox is an explicit consent switch for
// sending aggregate training metrics to the configured external AI
// provider. The transfer notice is rendered next to the checkbox so
// the user always reads it before checking the box; the consent is
// only granted when the user checks the box and saves the form.

import { useId } from 'react';
import styles from './PreferencesForm.module.css';

export interface PreferencesFormValues {
  hr_max: number | null;
  lthr: number | null;
  ftp: number | null;
  level: 'beginner' | 'intermediate' | 'advanced' | null;
  timezone: string;
  ai_enabled: boolean;
}

export interface PreferencesFormErrors {
  hr_max?: string | undefined;
  lthr?: string | undefined;
  ftp?: string | undefined;
  level?: string | undefined;
  timezone?: string | undefined;
}

export interface PreferencesFormProps {
  values: PreferencesFormValues;
  errors: PreferencesFormErrors;
  warning?: string | undefined;
  busy: boolean;
  onChange: <K extends keyof PreferencesFormValues>(
    key: K,
    value: PreferencesFormValues[K],
  ) => void;
  onSubmit: (e: React.FormEvent) => void;
}

function parseIntOrNull(s: string): number | null {
  if (s.trim() === '') return null;
  const n = Number.parseInt(s, 10);
  return Number.isFinite(n) ? n : null;
}

export function PreferencesForm({
  values,
  errors,
  warning,
  busy,
  onChange,
  onSubmit,
}: PreferencesFormProps) {
  const ids = {
    hr_max: useId(),
    lthr: useId(),
    ftp: useId(),
    level: useId(),
    timezone: useId(),
    ai: useId(),
    aiNotice: useId(),
  };

  return (
    <form className={styles.form} onSubmit={onSubmit} aria-label="Preferencias">
      <fieldset className={styles.fieldset} disabled={busy}>
        <legend className={styles.legend}>Métricas de entrenamiento</legend>

        <Field
          id={ids.hr_max}
          label="FC máxima (bpm)"
          type="number"
          value={values.hr_max}
          error={errors.hr_max}
          hint="1–260 bpm. Vacío si no la conoces."
          onChange={(v) => onChange('hr_max', parseIntOrNull(v))}
        />
        <Field
          id={ids.lthr}
          label="Umbral de lactato (bpm)"
          type="number"
          value={values.lthr}
          error={errors.lthr}
          hint="1–260 bpm. Vacío si no lo conoces."
          onChange={(v) => onChange('lthr', parseIntOrNull(v))}
        />
        <Field
          id={ids.ftp}
          label="FTP (watios)"
          type="number"
          value={values.ftp}
          error={errors.ftp}
          hint="1–2000 W. Vacío si no la conoces."
          onChange={(v) => onChange('ftp', parseIntOrNull(v))}
        />
        <div className={styles.field}>
          <label htmlFor={ids.level}>Nivel</label>
          <select
            id={ids.level}
            value={values.level ?? ''}
            onChange={(e) =>
              onChange(
                'level',
                e.target.value === ''
                  ? null
                  : (e.target.value as 'beginner' | 'intermediate' | 'advanced'),
              )
            }
          >
            <option value="">—</option>
            <option value="beginner">Inicial</option>
            <option value="intermediate">Intermedio</option>
            <option value="advanced">Avanzado</option>
          </select>
          {errors.level && (
            <p className={styles.error} role="alert">
              {errors.level}
            </p>
          )}
        </div>
      </fieldset>

      <fieldset className={styles.fieldset} disabled={busy}>
        <legend className={styles.legend}>Preferencias generales</legend>

        <Field
          id={ids.timezone}
          label="Zona horaria (IANA)"
          type="text"
          value={values.timezone}
          error={errors.timezone}
          hint='Ej. "Europe/Madrid", "America/New_York". Obligatorio.'
          onChange={(v) => onChange('timezone', v)}
        />

        <div className={styles.field}>
          <label className={styles.checkboxLabel}>
            <input
              id={ids.ai}
              type="checkbox"
              checked={values.ai_enabled}
              aria-describedby={ids.aiNotice}
              onChange={(e) => onChange('ai_enabled', e.target.checked)}
            />
            <span>Autorizar el envío de mis métricas al proveedor de IA</span>
          </label>
          <p className={styles.notice} id={ids.aiNotice} data-testid="ai-consent-notice">
            Al marcar esta casilla y guardar, autorizas el envío de tus datos de entrenamiento al
            proveedor de IA externo configurado (OpenAI, Claude u OpenRouter). Solo se envían
            métricas agregadas: distancia, duración, desnivel, frecuencia cardíaca y carga
            calculada. No se envía ningún dato de identificación: ni tu nombre, ni tu correo
            electrónico, ni tus coordenadas GPS, ni los streams crudos de las actividades. Mientras
            no aceptes, la IA permanece desactivada. Puedes revocar el consentimiento en cualquier
            momento desmarcando la casilla y guardando los cambios.
          </p>
        </div>
      </fieldset>

      {warning && (
        <p className={styles.warning} role="status">
          {warning}
        </p>
      )}

      <div className={styles.actions}>
        <button
          type="submit"
          className={styles.submit}
          disabled={busy}
          data-testid="preferences-submit"
        >
          {busy ? 'Guardando…' : 'Guardar preferencias'}
        </button>
      </div>
    </form>
  );
}

interface FieldProps {
  id: string;
  label: string;
  type: 'number' | 'text';
  value: number | string | null;
  error?: string | undefined;
  hint?: string | undefined;
  onChange: (v: string) => void;
}

function Field({ id, label, type, value, error, hint, onChange }: FieldProps) {
  const displayValue = value === null || value === undefined ? '' : String(value);
  return (
    <div className={styles.field}>
      <label htmlFor={id}>{label}</label>
      <input id={id} type={type} value={displayValue} onChange={(e) => onChange(e.target.value)} />
      {hint && !error && <p className={styles.hint}>{hint}</p>}
      {error && (
        <p className={styles.error} role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
