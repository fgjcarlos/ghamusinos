// Ventanas de los análisis de IA del panel de rendimiento (fase 1.5,
// issue #17, T8 — S4).
//
// El backend acepta una ventana que empieza en `window_start` y dura
// `window_days`:
//
//   - el análisis semanal tiene una ventana fija de 7 días,
//   - el análisis de carga acepta entre 1 y 90 días.
//
// Las dos ventanas terminan el día del panel (`to`, hoy en UTC), así que
// el análisis siempre habla de los días que el usuario está viendo.

/** AI_MAX_WINDOW_DAYS es el tope de días que admite la API. */
export const AI_MAX_WINDOW_DAYS = 90;

/** AI_WEEKLY_WINDOW_DAYS es la duración fija de la ventana semanal. */
export const AI_WEEKLY_WINDOW_DAYS = 7;

/** AIWindow es la ventana que se pide al backend. */
export interface AIWindow {
  /** Inicio de la ventana (YYYY-MM-DD). */
  windowStart: string;
  /** Duración de la ventana en días. */
  windowDays: number;
}

/**
 * shiftDays mueve una fecha YYYY-MM-DD un número de días (negativo hacia
 * atrás) y devuelve la fecha movida.
 */
export function shiftDays(date: string, days: number): string {
  const parsed = new Date(`${date}T00:00:00Z`);
  parsed.setUTCDate(parsed.getUTCDate() + days);
  return parsed.toISOString().slice(0, 10);
}

/**
 * weeklyWindow devuelve la ventana semanal: los 7 días que terminan en
 * `to`.
 */
export function weeklyWindow(to: string): AIWindow {
  return {
    windowStart: shiftDays(to, -(AI_WEEKLY_WINDOW_DAYS - 1)),
    windowDays: AI_WEEKLY_WINDOW_DAYS,
  };
}

/**
 * loadWindow devuelve la ventana de carga: los últimos `days` días que
 * terminan en `to`, acotados al máximo que acepta la API (90 días). Un
 * periodo más largo (por ejemplo, el último año) se recorta en lugar de
 * fallar: la API no acepta más de 90 días.
 */
export function loadWindow(to: string, days: number): AIWindow {
  const windowDays = Math.min(Math.max(Math.trunc(days), 1), AI_MAX_WINDOW_DAYS);
  return {
    windowStart: shiftDays(to, -(windowDays - 1)),
    windowDays,
  };
}
