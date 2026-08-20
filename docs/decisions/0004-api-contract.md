# ADR 0004 — Sin contrato formal de API por ahora (fase 1.1 → 1.3)

- **Estado:** aceptado
- **Fecha:** 2026-08-20
- **Contexto:** V1, fase 1.1 (base) hasta 1.3 (laboratorio GPX). Issue #40:
  las fases 1.2–1.6 añadirán ~15–20 endpoints y hoy no existe contrato formal
  entre el backend Go y el frontend TS — cada `fetch` será sin tipos y la
  deriva entre ambos lados será inevitable si no se decide algo.

## Decisión

**Adoptar la opción C: nada por ahora, revisar en 1.3** (cuando el laboratorio
GPX tenga al menos 3 endpoints estables para comparar contra datos reales).

No se introduce `huma`, `oapi-codegen`, ni tipos TS manuales en esta fase.

## Opciones evaluadas

| Opción | Qué implica | Pros | Contras (con datos del repo actual) |
|---|---|---|---|
| **A. huma** | Framework Go que genera OpenAPI 3.1 desde los handlers | Menos código manual, encaja con Chi, genera spec viva | Curva de aprendizaje; opinionated en errores; los handlers actuales usan `chi.Route` + `handlers.ProblemDetail` (RFC 9457) y huma asume JSON Schema propio — refactor amplio. |
| **B. oapi-codegen** | Spec OpenAPI manual → handlers/clientes generados | Control total del spec, genera tipos TS exactos | Spec manual se desactualiza sin guard; con 0 endpoints hoy, escribir el spec a mano es trabajo teórico. |
| **C. Nada por ahora** | Aceptar deriva hasta fase 1.3, decidir con datos | Cero coste ahora; los ~3 endpoints de Strava (fase 1.2) y los de GPX (1.3) enseñarán qué encaja mejor | Deriva tolerable durante 2–3 sprints. |
| **D. Tipos TS manuales** | `web/src/lib/api/types.ts` versionado a mano | Simple, sin toolchain nueva | Drift garantizado sin disciplina ni guard de generación. |

## Por qué C

- **Cobertura real < 5 endpoints.** Strava (fase 1.2) traerá OAuth connect/callback/disconnect (~3 endpoints con shapes distintos: redirect vs JSON). GPX (fase 1.3) añadirá upload, list, get, compare. Hasta no tener esos 7–10 endpoints funcionando y entendiendo qué patrón se repite (URL params, query strings, problem details, paginación), cualquier framework elegido será sobredimensionado o estará mal encajado.
- **El router actual es estable.** `internal/http/router.go` ya tiene `handlers.ProblemDetail` (RFC 9457), middleware de auth, `RequestID`, y separación `/api/v1/...`. Migrar a huma implicaría reescribir ~12 handlers existentes y reentrenar la generación de OpenAPI desde chi.
- **Cero coste de reversibilidad.** Cuando llegue 1.3, los datos dirán qué elegir. Si elegimos A o B tarde, lo hacemos contra una API más madura, no contra una en construcción.
- **El derive se mitiga con disciplina, no con framework.** Mientras tanto: revisión de PRs bloquea mismatches obvios; los handlers Go tienen tests con golden JSON que el frontend puede copiar como referencia.

## Consecuencias

- **Aceptado:** habrá drift menor entre shapes Go↔TS hasta 1.3. Aceptable porque: el frontend tiene pocos endpoints aún, los shapes son simples, y la auditoría es continua.
- **Trigger para reabrir #40 (decisión A/B/D):**
  - Llegamos a 1.3 y hay ≥ 5 endpoints nuevos (los datos confirman complejidad).
  - Aparece un cliente distinto del SPA propio (CLI, integración externa, SDK público).
  - El derive causa un bug de producción (no se ha dado; queremos reaccionar, no anticipar).
- **Mientras tanto:** cada handler Go nuevo debe tener un test que serializa su respuesta (golden JSON) — esto ya es práctica del repo y lo mantiene vivo sin herramienta nueva.

## Lo que NO se hace aquí

- No se añade `huma`, `oapi-codegen`, ni generadores de tipos TS.
- No se escribe un OpenAPI spec a mano hoy (sólo cuando la opción A o B se adopte, con datos reales debajo).
- No se crea `web/src/lib/api/types.ts` como contrato manual (opción D descartada explícitamente).

Refs #40