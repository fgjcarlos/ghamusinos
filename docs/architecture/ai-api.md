# API de IA (fase 1.5 · issue #17)

La puerta HTTP de los **análisis de IA opcionales**: pedir un análisis
(actividad, semanal o de carga), consultar su estado y listar los del
usuario. Vive dentro del grupo autenticado `/api/v1` y **no habla con el
proveedor**: crea la fila en `ai_analysis` y encola el job `ai_analysis`,
que es la única puerta de salida hacia un proveedor (ver
[Runtime de IA](ai-runtime.md)).

Con la IA apagada, no disponible o sin cola de jobs, el producto sigue
funcionando igual (S7): el estado se puede consultar siempre y la petición
de análisis responde `503` sin crear nada.

## Camino rápido

1. La web llama a `GET /api/v1/ai/status` y lee si la IA está disponible y
   si el usuario ha consentido.
2. Si puede, llama a `POST /api/v1/ai/analyses` con el objetivo.
3. Recibe `202` con el análisis en estado `pending` y lo sondea con
   `GET /api/v1/ai/analyses?kind=…&scope=…` hasta que el job lo cierre
   (`completed` o `failed`).

## Rutas

| Método | Ruta | Respuesta |
|---|---|---|
| `GET` | `/api/v1/ai/status` | `200` estado global + consentimiento del usuario |
| `POST` | `/api/v1/ai/analyses` | `202` análisis creado (en `pending`) |
| `GET` | `/api/v1/ai/analyses` | `200` listado del usuario (`{data, limit}`) |
| `GET` | `/api/v1/ai/analyses?kind=…&scope=…` | `200` un análisis, o `404` |

Las tres rutas exigen autenticación (`401` sin usuario) y están dentro del
grupo `/api/v1` que ya protege el middleware de Clerk.

## `GET /api/v1/ai/status`

```json
{
  "enabled": true,
  "available": true,
  "status": "ready",
  "provider": "openai",
  "consented": true
}
```

| Campo | Significado |
|---|---|
| `enabled` | El *feature flag* global (`AI_ENABLED`) |
| `available` | La IA se puede usar ahora mismo (flag + proveedor + credencial) |
| `status` | `disabled`, `unavailable` o `ready` (mismo vocabulary que `config.AIConfig.Status()`) |
| `provider` | Nombre del proveedor configurado; `null` si la IA no está disponible |
| `consented` | Si **el usuario autenticado** aceptó el aviso de transferencia |

**Nunca** devuelve la credencial ni el modelo configurado. Con la IA apagada
responde `200` con `enabled=false`, `available=false`, `status="disabled"` y
`provider=null`: la web puede pintar el estado "IA no disponible" sin tratar
un error.

## `POST /api/v1/ai/analyses`

Cuerpo:

```json
{ "kind": "activity", "activity_id": "33333333-3333-3333-3333-333333333333" }
{ "kind": "weekly" }
{ "kind": "load", "window_start": "2026-01-05", "window_days": 28 }
```

| Campo | Regla |
|---|---|
| `kind` | Obligatorio: `activity`, `weekly` o `load` |
| `activity_id` | Sólo con `kind=activity`: UUID de una actividad del usuario |
| `window_start` | Sólo con `kind=weekly`/`load`: `YYYY-MM-DD`. Por defecto, hoy (UTC) |
| `window_days` | Sólo con `kind=weekly`/`load`. Por defecto 7 (semanal) y 28 (carga). Máximo `ai.MaxWindowDays` (90) |

La ventana semanal es fija (7 días, la que usa el job): un `window_days`
distinto en un análisis semanal es un `422`, no un análisis silencioso de
otra ventana.

### Orden de las comprobaciones

Cada paso tiene su propio estado HTTP y ninguno crea nada cuando falla:

| # | Comprobación | Si falla |
|---|---|---|
| 1 | Usuario autenticado | `401` |
| 2 | IA global encendida **y** disponible | `503` |
| 3 | Cola de jobs disponible | `503` (sin crear fila) |
| 4 | Consentimiento del usuario (`users.ai_enabled`) | `403` |
| 5 | Cuerpo válido (`kind`, `activity_id`, ventana) | `422` |
| 6 | La actividad existe **y** es del usuario | `404` |

El consentimiento se **vuelve a comprobar aquí**, al encolar (S3): el job lo
vuelve a comprobar antes de enviar nada al proveedor.

El `scope` se construye con los helpers compartidos
(`ai.ActivityScope`, `ai.WeeklyScope`, `ai.LoadScope`), así que la fila de
`ai_analysis` y el job apuntan al mismo objetivo.

### Respuesta (`202`)

```json
{
  "id": "09000000-0000-0000-0000-000000000000",
  "kind": "activity",
  "scope": "activity:33333333-3333-3333-3333-333333333333",
  "status": "pending",
  "provider": null,
  "model": null,
  "output": null,
  "error_code": null,
  "error_message": null,
  "created_at": "2026-01-02T03:04:05Z",
  "updated_at": "2026-01-02T03:04:05Z"
}
```

El job se encola con `river.InsertOpts{MaxAttempts: jobs.AIAnalysisMaxAttempts(cfg.AI)}`
(un intento inicial más `AI_MAX_RETRIES`, nunca por encima de
`1 + config.MaxAIMaxRetries`).

## `GET /api/v1/ai/analyses`

Sin parámetros devuelve el listado del usuario, del análisis más reciente
al más antiguo:

```json
{ "data": [ { "id": "…", "kind": "weekly", "…": "…" } ], "limit": 20 }
```

| Parámetro | Regla |
|---|---|
| `limit` | Entero. Por defecto 20, máximo 100 (un valor mayor se acota) |
| `kind` + `scope` | Juntos: devuelven **un** análisis (`404` si no existe) |

Un `limit` que no es entero es un `422`. `kind` y `scope` tienen que venir
juntos: si sólo llega uno, es un `422`.

## Forma de un análisis

| Campo | Cuándo |
|---|---|
| `id`, `kind`, `scope` | Siempre |
| `status` | `pending`, `completed` o `failed` |
| `provider`, `model` | Cuando se saben (los que se usaron) |
| `output` | Sólo cuando el análisis se completó: `{summary, observations, cautions}` |
| `error_code`, `error_message` | Sólo cuando falló: el código y el mensaje **seguros** |
| `created_at`, `updated_at` | Auditoría (RFC 3339) |

El **payload** enviado al proveedor no se expone nunca, ni el cuerpo de
error del proveedor: sólo el código y el mensaje seguros que escribió la
aplicación.

## Errores (RFC 9457)

Todos los errores son `application/problem+json` con `type`, `title`,
`status`, `detail` e `instance` (el *request id*). Los errores de
validación añaden el miembro de extensión `errors` con el detalle por
campo (`kind`, `activity_id`, `window_start`, `window_days`, `limit`).

| Estado | Cuándo | `detail` (ejemplo) |
|---|---|---|
| `401` | Sin autenticar | `not authenticated` |
| `403` | Sin consentimiento | `ai consent required: accept the data transfer notice before requesting an analysis` |
| `404` | Actividad o análisis que no existe (o es de otro usuario) | `activity not found` |
| `422` | Petición inválida | `request validation failed` |
| `503` | IA apagada, no disponible o sin cola de jobs | `ai analysis unavailable: the global AI flag is disabled` |

## Aislamiento entre usuarios

Cada lectura y cada escritura va filtrada por `user_id`:

- `GetActivityForUser` filtra por `(id, user_id)`: la actividad de otro
  usuario **no existe** y su análisis es un `404`; nunca se analiza.
- `GetAIAnalysis` y `ListAIAnalyses` filtran por `user_id`: un usuario no ve
  los análisis de otro.
- `CreateAIAnalysis` escribe con el `user_id` del usuario autenticado.

## Cliente de TypeScript

`web/src/lib/api/ai.ts` expone las llamadas con los mismos tipos que el
backend:

| Export | Llamada |
|---|---|
| `fetchAIStatus(token)` | `GET /api/v1/ai/status` → `AIStatusResponse` |
| `createAIAnalysis(token, request)` | `POST /api/v1/ai/analyses` → `AIAnalysis` |
| `listAIAnalyses(token, { limit? })` | `GET /api/v1/ai/analyses` → `AIAnalysisListResponse` |
| `fetchAIAnalysis(token, kind, scope)` | `GET /api/v1/ai/analyses?kind=…&scope=…` → `AIAnalysis` |

Tipos exportados: `AIAnalysisKind`, `AIAnalysisStatus`, `AIStatus`,
`AIStatusResponse`, `AIAnalysisOutput`, `AIAnalysis`,
`AIAnalysisListResponse`, `AIAnalysisRequest`.

El cliente sigue las convenciones de `profile.ts`: cabecera `Bearer`, cuerpo
JSON y los fallos como `ApiError` con el `status`, el `detail` del problema
y el mapa `fields` de los errores por campo. El cuerpo de la petición nunca
mezcla campos de actividad y de ventana (el backend los rechaza).

## Dónde está cada cosa

| Pieza | Ruta |
|---|---|
| Handlers | `internal/http/handlers/ai_analysis.go` |
| Tests de los handlers | `internal/http/handlers/ai_analysis_test.go` |
| Mocks (store y encolador) | `internal/http/handlers/ai_analysis_mocks_test.go` |
| Montaje de las rutas | `internal/http/router.go` (grupo `/api/v1/ai`) |
| Test de montaje | `internal/http/ai_analysis_mocks_test.go` |
| Cliente de la API | `web/src/lib/api/ai.ts` |
| Tests del cliente | `web/src/lib/api/ai.test.ts` |
| Job que realiza el análisis | `internal/jobs/ai_analysis.go` |
| Claves de scope | `internal/ai/scope.go` |
| Persistencia | `internal/db/sqlc/ai_analysis.sql.go` |
| Contrato del payload | [Contrato de IA](ai-contract.md) |
| Consentimiento | [Consentimiento de IA](ai-consent.md) |
| Runtime y job | [Runtime de IA](ai-runtime.md) |

## Comprobación

```bash
TZ=UTC go test -count=1 ./internal/http/... ./internal/app
TZ=UTC go test -race -count=1 ./internal/http/... ./internal/app
TZ=UTC go vet ./internal/http/... ./internal/app
golangci-lint run ./internal/http/... ./internal/app/...
pnpm -C web test:run src/lib/api/ai.test.ts
pnpm -C web typecheck
```

## Checklist de revisión

- [ ] Las tres rutas están dentro del grupo autenticado `/api/v1`.
- [ ] El estado no filtra la credencial ni el modelo.
- [ ] El estado funciona con la IA apagada.
- [ ] Sin consentimiento: `403` y ninguna fila creada.
- [ ] Con la IA apagada o no disponible: `503` y ninguna fila creada.
- [ ] Sin cola de jobs: `503` y ninguna fila creada.
- [ ] Una actividad ajena o inexistente: `404` y nada creado.
- [ ] Un `kind`, una ventana o un límite inválidos: `422` y nada creado.
- [ ] Una petición válida crea la fila en `pending` con el scope documentado.
- [ ] El job se encola con `jobs.AIAnalysisMaxAttempts(cfg.AI)`.
- [ ] El listado sólo trae análisis del usuario autenticado y respeta el límite.
- [ ] Un análisis que no existe: `404`.
- [ ] Un análisis fallido expone sólo el código y el mensaje seguros.

## Siguiente paso

T8: los controles en la UI de actividad y de dashboard (pedir el análisis,
mostrar el resultado y el estado "IA no disponible") reutilizando este
cliente.
