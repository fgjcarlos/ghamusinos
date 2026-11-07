// Constantes del control de análisis con IA (fase 1.5, T8).
//
// Viven en su propio módulo (y no en el .tsx) para que el componente
// exporte sólo componentes y para que los tests puedan usar los mismos
// valores sin adivinar numbers mágicos.

/**
 * AI_POLL_INTERVAL_MS es cada cuánto se consulta el estado de un análisis
 * que sigue en curso. El job corre en segundo plano: 2 s es suficiente
 * para notar el resultado sin martillear la API.
 */
export const AI_POLL_INTERVAL_MS = 2000;

/**
 * AI_MAX_POLL_ATTEMPTS es el número máximo de consultas por análisis (2 s
 * cada una, unos 2 minutos). Si se agota, el control lo dice y deja pedir
 * otro: nunca se queda esperando en silencio.
 */
export const AI_MAX_POLL_ATTEMPTS = 60;
