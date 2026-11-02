# Tasks: phase-1-5-optional-ai

> Cadena sobre la rama de la épica (`feat/phase-1-5-optional-ai`, base
> `main@a6adff0`). Delivery=ask-on-risk. Budget=400 LoC por unidad
> (varias unidades lo superaron: son las "coherent slices" que el
> documento de la feature permite). TDD posture: strict_tdd=false a
> nivel workspace; RED-GREEN-REFACTOR por comportamiento dentro de cada
> unidad, con el comando de test anotado.
>
> Issue epic: #17. Specs referenciadas: S1..S8.
> Unidades entregadas: 9 commits de trabajo (T1, T2, T4, T3, T5, T6, T10,
> T7, T8) + esta unidad documental (T9).
>
> **Orden real de la cadena** (base → punta):
> `99db0a9` → `49f8585` → `fc6c54f` → `8d823f9` → `5cdd64c` → `b8d9360`
> → `67be97a` → `4de56a5` → `8265703`.

## Fase A — Contrato del dominio y adaptadores

> Sin configuración, sin persistencia y sin red. Es la base de la que
> dependen el job, la API y la web.

- [x] **1.1** (T1, S5/S7) — Lista blanca del *payload* y contrato de salida estricto
- Acción: crear `internal/ai/payload.go` (`Kind`, `Metrics`, `Payload`,
  `Builder`, `NewBuilder`, `Build`, `MarshalJSON`) con los cinco agregados
  y sus topes; crear `internal/ai/output.go` (`Output`, `OutputSchema()`,
  `ValidateOutput`, `ParseOutput`) con el esquema único, el validador Go
  equivalente, el vocabulario de diagnóstico vetado y el veto a las
  métricas inventadas; documentar el contrato en
  `docs/architecture/ai-contract.md`.
- Comando de test: `TZ=UTC go test -count=1 ./internal/ai` y
  `TZ=UTC go vet ./internal/ai`.
- Resultado esperado: un agregado sin medir se omite (nunca `0`
  inventado); `avg_heart_rate = 0` se rechaza; valores no finitos,
  negativos o fuera de rango se rechazan en `Build()`; una respuesta con
  campo desconocido, `null`, JSON de más, longitud fuera de rango,
  métrica inventada o diagnóstico se rechaza.
- Work-unit commit: `99db0a9` — `feat(ai): payload allowlist and shared strict output contract` (1 396 líneas añadidas, 635 de test; 5 ficheros).

- [x] **1.2** (T3, S2/S7) — Adaptadores de proveedor en el orden de ADR 0002
- Acción: crear `internal/ai/analyzer.go` (interfaz `Analyzer`,
  `ProviderConfig`, `NewAnalyzer` sin *failover*, `Error`/`ErrorKind`/
  `Reason`, `Retryable`, `SupportedProviders`, `ParseProvider`) y los tres
  adaptadores `openai.go`, `claude.go`, `openrouter.go`; cada uno envía
  el esquema compartido y valida la respuesta con el mismo validador.
- Comando de test: `TZ=UTC go test -count=1 ./internal/ai` (los
  adaptadores usan `httptest` con reescritura de host: **ninguna**
  petición real, ninguna credencial real, ningún coste).
- Resultado esperado: el proveedor activo es el configurado; un fallo
  **no** cambia de proveedor; una llamada = un intento HTTP; sin
  redirecciones; respuesta acotada a 1 MiB; sólo `application/json`;
  el mensaje de error no lleva *payload*, credencial ni cuerpo del
  proveedor; 429/5xx/red/*timeout* son reintentables y 401/403/4xx
  distintas de 429, 404, rechazo, JSON inválido, esquema inválido,
  *redirect* y respuesta inesperada son permanentes.
- Work-unit commit: `8d823f9` — `feat(ai): provider adapters for OpenAI, Claude and OpenRouter` (2 466 añadidas / 12 borradas, 1 391 de test; 10 ficheros).

## Fase B — Consentimiento y configuración global

> Las dos piezas que hacen que la IA sea *opcional* y no una decisión
> implícita.

- [x] **2.1** (T2, S3/S5) — Consentimiento explícito y nuevo para todas las cuentas
- Acción: crear la migración `internal/db/migrations/00014_ai_explicit_opt_in.sql`
  (default `false` + reset de los `true` heredados + `Down` que no
  restaura el consentimento), alinear `internal/db/schema.sql`, y
  documentar el contrato en `docs/architecture/ai-consent.md`; añadir el
  aviso de transferencia al formulario de preferencias
  (`web/src/features/profile/PreferencesForm.tsx`) con la lista de
  agregados que **sí** salen y los que **no**; el campo `ai_enabled` se
  suma a `PATCH /api/v1/me/preferences` con semántica de omisión
  (`false`).
- Comando de test: `TZ=UTC go test -count=1 ./internal/http/handlers
  ./internal/auth` y `pnpm -C web --config.verify-deps-before-run=false
  test:run src/features/profile src/lib/api/profile.test.ts`.
- Resultado esperado: `ai_enabled` omitido se guarda `false`; sólo un
  `true` explícito concede consentimiento; el `Down` no reescribe filas
  ni restaura `DEFAULT true`; el resto de las preferencias se guarda
  igual que antes; el aviso nombra proveedor, agregados permitidos y
  exclusiones.
- Work-unit commit: `49f8585` — `feat(ai): require fresh explicit consent for AI transfer` (746 añadidas / 2 borradas, 529 de test; 11 ficheros).

- [x] **2.2** (T4, S3/S6/S7) — Configuración global que no bloquea el arranque + persistencia
- Acción: añadir `config.AIConfig` a `internal/config/config.go`
  (`AI_ENABLED`, `AI_PROVIDER`, `AI_MODEL`, credencial por proveedor,
  `AI_TIMEOUT`, `AI_MAX_RETRIES`, `Status()`, `LogWarnings`), con
  `loadAIConfig()` que **nunca** devuelve error; crear la migración
  `00015_ai_analysis.sql` (tabla `ai_analysis` con `UNIQUE (user_id,
  kind, scope)` y `CHECK` sobre `kind`, `status`, `provider`, `model`,
  `error_code` y `error_message`), las cinco consultas
  `internal/db/queries/ai_analysis.sql` y el código generado; documentar
  en `docs/architecture/ai-runtime.md`.
- Comando de test: `TZ=UTC go test -race -count=1 ./...` (incluye el
  *ripple* de la interfaz `sqlc.Querier`: cinco *stubs* nuevos en los
  mocks de `internal/auth`, `internal/http` e `internal/http/handlers`).
- Resultado esperado: con `AI_ENABLED` apagado no se lee ninguna
  credencial; un proveedor inválido, una credencial ausente o un
  `AI_MODEL` vacío dejan la IA `unavailable` con aviso **sin impedir el
  arranque**; los avisos sólo nombran variables de entorno; las
  consultas que no crean filtran siempre por `user_id`.
- Work-unit commit: `fc6c54f` — `feat(ai): global AI configuration and ai_analysis storage` (2 201 añadidas, 1 199 de test; 14 ficheros).

## Fase C — Ejecución

> El job es la única puerta de salida hacia un proveedor.

- [x] **3.1** (T5, S3/S4/S6/S7) — Job de River `ai_analysis`
- Acción: crear `internal/jobs/ai_analysis.go` (`AIAnalysisArgs`,
  `AIAnalysisWorker`, `Work`, `AIAnalysisMaxAttempts`, códigos de error
  seguros) e `internal/ai/scope.go` (claves de *scope*); añadir las
  consultas `GetActivityForUser` y `ListActivitiesInRange`.
- Comando de test: `TZ=UTC go test -count=1 ./internal/jobs ./internal/ai
  ./internal/db/sqlc` y `TZ=UTC go vet ./internal/jobs ./internal/ai`.
- Resultado esperado: el consentimiento se comprueba **antes** de tocar
  el proveedor (`consent_revoked`, sin error y sin proveedor); la IA
  apagada o no disponible deja `ai_disabled` / `ai_unavailable` sin
  error; una actividad ajena deja `not_found`; una ventana fuera de
  rango deja `invalid_window` / `window_too_large`; los agregados que no
  caben dejan `invalid_metrics`; el proveedor se llama **una** vez; un
  fallo reintentable devuelve el error (River aplica el *backoff*) y uno
  permanente termina sin error.
- Work-unit commit: `5cdd64c` — `feat(ai): River job for the on-demand AI analysis` (2 362 añadidas / 14 borradas, 1 115 de test; 11 ficheros).

- [x] **3.2** (T6, S3/S6/S7) — Cableado de la aplicación
- Acción: en `internal/app/app.go` extraer `buildAIAnalyzer(cfg)`, arrancar
  River con Strava **o** con IA activa, y registrar el worker de IA en
  `internal/jobs/workers.go` siempre que exista analizador.
- Comando de test: `TZ=UTC go test -count=1 ./internal/app ./internal/jobs`
  y `TZ=UTC go vet ./internal/app ./internal/jobs`.
- Resultado esperado: sin Strava pero con IA activa la cola de jobs
  arranca; sin analizador el worker no se registra y la IA se reporta no
  disponible; con la IA apagada el arranque es idéntico al de antes.
- Work-unit commit: `b8d9360` — `feat(ai): wire the AI worker so River runs without Strava` (149 añadidas / 15 borradas, 72 de test; 3 ficheros).

- [x] **3.3** (T10, S5/S7) — Topes del *payload* proporcionales a la ventana
- Acción: corregir el defecto de contrato detectado en la unidad 3.1: los
  topes de los agregados *sumados* pasan a ser el tope por día × días de
  la ventana (`ai.LimitsFor(kind, windowDays)`); los topes por actividad
  no cambian; la FC media nunca escala; el máximo de ventana (90 días)
  se mantiene.
- Comando de test: `TZ=UTC go test -count=1 ./internal/ai ./internal/jobs`
  y `TZ=UTC go vet ./internal/ai ./internal/jobs`.
- Resultado esperado: una ventana de 28 días admite los totales de 28
  días; una actividad conserva sus topes por actividad; una FC media
  por encima del tope se rechaza en cualquier ventana; una ventana de
  más de 90 días se rechaza.
- Work-unit commit: `67be97a` — `fix(ai): scale the payload ceilings with the analysed window` (400 añadidas / 27 borradas, 246 de test; 6 ficheros).

## Fase D — Superficie de producto

- [x] **4.1** (T7, S3/S4/S5/S6/S7) — API de análisis bajo demanda + cliente TS
- Acción: crear `internal/http/handlers/ai_analysis.go`
  (`GetAIStatus`, `PostAIAnalysis`, `ListAIAnalyses`), montarlas en
  `internal/http/router.go` bajo el grupo autenticado `/api/v1/ai`, y
  escribir el cliente `web/src/lib/api/ai.ts` con los tipos espejo.
- Comando de test: `TZ=UTC go test -count=1 ./internal/http/... ./internal/app`,
  `TZ=UTC go vet ./internal/http/... ./internal/app` y
  `pnpm -C web test:run src/lib/api/ai.test.ts`.
- Resultado esperado: `GET /api/v1/ai/status` responde `200` incluso con
  la IA apagada y nunca expone la credencial ni el modelo; `POST`
  comprueba en orden disponibilidad → cola → consentimiento → validez →
  pertenencia y responde `503`/`403`/`422`/`404` **sin crear fila**; una
  petición válida responde `202` con la fila en `pending`; el listado
  respeta el límite (20 por defecto, 100 como máximo) y sólo trae
  análisis del usuario autenticado.
- Work-unit commit: `4de56a5` — `feat(api): endpoints to request and read the optional AI analyses` (2 213 añadidas, 1 107 de test; 8 ficheros).

- [x] **4.2** (T8, S3/S4/S7) — Controles de análisis en las pantallas existentes
- Acción: crear el control compartido
  `web/src/features/ai/AIAnalysisControl/` (leído del estado antes de
  ofrecer nada, *poll* cada 2 s con tope de 60 intentos), montarlo por
  actividad en `ActivityList`, y añadir `DashboardAIAnalysis` +
  `web/src/features/dashboard/aiWindow.ts` al panel de rendimiento.
- Comando de test: `pnpm -C web test:run src/features/ai src/features/activities
  src/features/profile` y `pnpm -C web lint`.
- Resultado esperado: con la IA desactivada, no disponible o sin
  consentimiento **no sale ninguna petición** y se explica dónde
  resolverlo; con la IA disponible y consentido el botón pide el
  análisis y se sondea hasta `completed`/`failed`; sólo se pinta la
  salida validada y el código de error seguro.
- Work-unit commit: `8265703` — `feat(web): on-demand AI analysis controls in the existing screens` (1 475 añadidas / 16 borradas, 627 de test; 20 ficheros).

## Fase E — Cierre

- [x] **5.1** (T9, S1-S8) — Verificación independiente y especificación canónica
- Acción: verificación funcional, de privacidad y de aislamiento de toda
  la fase; cierre de los *deltas* de esta carpeta
  (`proposal.md`, `tasks.md`, `design.md`, `specs/ai-analysis-backend`,
  `specs/ai-analysis-api`, `specs/ai-analysis-web`) y `archive-report.md`.
  **No** se promotes los *deltas* a `openspec/specs/`: ese paso es el
  cierre final de la épica y lo ejecuta el padre.
- Comando de test: `ls -R openspec/changes/phase-1-5-optional-ai` y
  `git diff --stat openspec/changes/phase-1-5-optional-ai`.
- Resultado esperado: los tres *deltas* describen el comportamiento
  entregado, cada requisito declara su proyecto y su comando de test, y
  cada S1..S8 queda cubierto por al menos un escenario.
- Work-unit commit: pendiente (lo confirma el padre).

## Notas de continuidad

- **Desajuste conocido en el documento de la feature**: la fila de T3
  sigue diciendo `pending` / `commit: none`, pero los adaptadores
  **sí** se entregaron, en `8d823f9` (ver 1.2). El padre debe
  actualizar `odd/tasks/phase-1-5-optional-ai.md` para que la fila
  refleje el estado real.
- **Defecto de contrato encontrado por el worker de 3.1 y aceptado**: los
  topes del *payload* de T1 estaban calibrados para una actividad
  (duración 7 días, carga 1000), así que una ventana semanal o de carga
  que los superara fallaba como `invalid_metrics`. Corregido en 3.3.
- **Riesgos aceptados por el padre en 2.2**: (a) el *ripple* de la
  interfaz `sqlc.Querier` (cinco *stubs* nuevos en los mocks de
  `internal/auth`, `internal/http` e `internal/http/handlers`);
  (b) un `AI_MODEL` vacío deja la IA `unavailable` con aviso, en vez de
  asumir un modelo por defecto del adaptador (ningún nombre de modelo
  se inventa).
