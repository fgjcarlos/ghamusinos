# Design: phase-1-5-optional-ai

## Decisions

### D1 — Interfaz `ai.Analyzer` + orden de proveedores de ADR 0002
- **Context:** el producto admite tres proveedores (OpenAI, Claude, OpenRouter) y no puede depender de ninguno: el dominio no debe saber qué hay al otro lado, y el adaptador no debe saber nada de workouts ni de usuarios.
- **Decision:** `internal/ai/analyzer.go` define la interfaz `Analyzer { Provider() Provider; Analyze(ctx, Payload) (Output, error) }`, la configuración `ProviderConfig{Provider, Model, APIKey, HTTPClient}` y el registro `SupportedProviders()` con el orden de ADR 0002 (`openai`, `claude`, `openrouter`). `NewAnalyzer` devuelve el adaptador del proveedor configurado.
- **Rationale:** el orden de ADR 0002 es la preferencia de producto; al vivir en un único registro, cambiarlo es un cambio de una línea y no una modification en tres adaptadores.
- **Alternatives:** un `switch` por proveedor dentro del job; una interfaz por proveedor; un SDK oficial por proveedor (descartado: tres dependencias nuevas y tres contratos que versionar).
- **Tradeoffs:** el registro no es extensible en runtime (aceptado: tres proveedores son un número cerrado hasta que aparezca un cuarto).

### D2 — Sin *failover* implícito entre proveedores
- **Context:** un proveedor puede fallar por cuota o por indisponibilidad; "probar con otro" es la reacción más natural… y también una transferencia de datos a un receptor que el usuario no ha elegido.
- **Decision:** un fallo de un proveedor **nunca** salta a otro. `NewAnalyzer` devuelve exactamente el proveedor configurado o un error; los adaptadores no se encadenan.
- **Rationale:** el consentimiento es por receptor ("el proveedor de IA configurado"). Un *failover* implícito convertiría un consentimiento explícito en un permiso difuso, y además enviaría las métricas a un receptor distinto del que la cuenta acepta.
- **Alternatives:** cadena de proveedores con el mismo esquema (descartado por esta razón); *failover* sólo tras un aviso explícito al usuario (descartado: añadiría un estado de UI y una auditoría que la fase no pide).
- **Tradeoffs:** el proveedor caído deja la fila en `failed` con el código del motivo; el usuario puede reintentar cuando el proveedor vuelva. La UI no distingue "otro proveedor habría funcionado".

### D3 — Una llamada HTTP por `Analyze`; los reintentos son del job
- **Context:** los reintentos con *backoff* son un requisito de la issue, pero el *backoff* necesita estado (intentos, último error) que un `http.Client` no tiene.
- **Decision:** `Analyze` hace **un** intento HTTP y clasifica el fallo como reintentable o permanente (`ai.Error.Kind`, `ai.Retryable`). El presupuesto de intentos y el *backoff* viven en el job de River.
- **Rationale:** River ya tiene cola, reintentos y *backoff* exponencial (`ATTEMPT^4`); duplicar esa máquina dentro del adaptador sería reinventarla y la haría invisible.
- **Alternatives:** reintentos dentro del adaptador (rechazado: sin *backoff*, sin trazabilidad y con el coste de proveedor sin controlar); reintentos en la API (rechazado: la API es síncrona y no debe esperar al proveedor).
- **Tradeoffs:** un fallo reintentable deja la fila `failed` entre intentos y el usuario ve un estado intermedio "fallido" antes del reintento; a cambio, el estado es honesto y la auditoría es completa.

### D4 — *Payload* agregado con lista blanca (`internal/ai`)
- **Context:** la issue pide "payload builder + schema de salida validado"; el usuario eligió "sólo métricas agregadas: distancia, duración, desnivel, FC y carga calculada; sin identidad, nombres, GPS ni streams crudos".
- **Decision:** `ai.Payload` tiene exactamente seis campos JSON (`kind`, `window_days` y cinco agregados). No existe ningún campo de identidad, nombre, correo, GPS, marca de tiempo ni texto libre. Un agregado **sin medir se omite** (nunca `0` inventado); un `0` explícito sí se envía (un día de descanso tiene carga real `0`). `avg_heart_rate = 0` es un error, no un cero. `Build()` rechaza valores no finitos, negativos o fuera de rango.
- **Rationale:** la privacidad se sostiene **por construcción**: no hay por dónde filtrar. Una lista de campos prohibidos sería una lista que hay que mantener; una lista blanca es una lista que hay que ampliar para que algo salga.
- **Alternatives:** serializar la fila de `activities` completa (descartado: arrastra nombre y payload crudo); un esquema JSON externo con `"additionalProperties": false` (descartado: el *payload* no es un JSON libre, es una estructura Go).
- **Tradeoffs:** el *payload* no admite preguntas al proveedor sobre la actividad concreta (ni tipo de deporte, ni fecha): es el precio de no enviar identidad. La ventana (`window_days`) es el único contexto temporal.

### D5 — Topes por ventana: `ai.LimitsFor(kind, windowDays)`
- **Context:** los topes de T1 (`MaxDistanceMeters = 1 000 000`, `MaxDurationSeconds = 604 800`, `MaxElevationGainMeters = 50 000`, `MaxTrainingLoad = 1 000`, `MaxAvgHeartRate = 240`) están calibrados para **una** actividad. Una ventana de 28 días que suma 28 días de datos los supera y se rechazaba como `invalid_metrics`: el semanal y el de carga quedaban inservibles (defecto encontrado en la unidad 3.1, corregido en 3.3).
- **Decision:** el tope de un agregado **sumado** es el tope por día × días de la ventana: `tope(métrica, N días) = topePorDía(métrica) × N`. `kind=activity` nunca escala. La FC media **nunca** escala (es una media, no una suma). `windowDays` se acota a `1..MaxWindowDays` (90) y una ventana mayor se rechaza.
- **Rationale:** el tope por día es el mismo que el de una actividad, así que **ningún dato válido para una actividad queda fuera de la ventana que lo contiene**, y el escalado no amplía la ventana (sigue acotada a 90 días).
- **Alternatives:** topes absolutos por tipo de análisis (descartado: obliga a inventar tres juegos de topes y vuelve frágil el contrato); sin topes (descartado: el *payload* dejaría de ser un contrato verificable).
- **Tradeoffs:** el contrato necesita que la API declare la ventana al construir el *payload*; por eso `ai.LimitsFor` es parte de la API pública del paquete.

### D6 — Un único esquema de salida estricto, validado en Go
- **Context:** "schema de salida validado e idéntico entre proveedores" (S5). Tres proveedores, tres formatos de respuesta y el riesgo de que cada uno acepte algo distinto.
- **Decision:** `ai.OutputSchema()` devuelve **un** esquema JSON (`training_analysis`, `strict: true`, `additionalProperties: false`, `summary` + `observations` + `cautions` con longitudes y topes acotados) y ese mismo esquema se entrega a los tres proveedores (OpenAI/OpenRouter: `response_format.json_schema`; Claude: herramienta forzada con `input_schema`). La respuesta se normaliza y se valida con `ai.ValidateOutput` / `ai.ParseOutput`, el mismo validador para todos. El validador rechaza campos desconocidos, tipos incorrectos, `null`, JSON de más, longitudes fuera de rango, el **vocabulario de diagnóstico** y **cualquier número que no se haya enviado** (sentidos + `window_days`).
- **Rationale:** la salida aceptada no depende del proveedor; y el veto a las métricas inventadas implementa la regla del producto "la IA interpreta datos; no inventa métricas": las comparaciones y los porcentajes los calcula la aplicación, que sí tiene los datos.
- **Alternatives:** validar en el cliente (descartado: el usuario vería una salida sin validar); un esquema por proveedor (descartado: es exactamente lo que S5 prohíbe); permitir métricas en la salida (descartado: abriría la puerta a que el modelo invente cifras).
- **Tradeoffs:** un modelo que responde con un comentario alrededor del JSON se rechaza (`invalid_json`); y una respuesta con un 12 % no utilizable se pierde. Se prefiere perder una respuesta a mostrar un dato no medido.

### D7 — El *feature flag* global nunca bloquea el arranque
- **Context:** una IA opcional que puede impedir que la aplicación arranque es una IA obligatoria por la puerta de atrás.
- **Decision:** `config.Load()` **nunca** falla por la IA: `loadAIConfig()` no devuelve error. Cada problema (proveedor no admitido, `AI_MODEL` vacío, credencial ausente, `AI_TIMEOUT` o `AI_MAX_RETRIES` inválidos) deja un aviso en `cfg.AI.Warnings` y la IA en estado `unavailable` (`Status()` ∈ {`disabled`, `unavailable`, `ready`}). Con `AI_ENABLED` apagado **no se lee ninguna credencial**. Sólo se carga la credencial del proveedor seleccionado.
- **Rationale:** S7 ("con IA desactivada o caída, el producto funciona igual") es un criterio de aceptación de la issue: la degradación tiene que ser un estado, no un error de arranque.
- **Alternatives:** validar la configuración de IA y fallar el arranque (descartado: rompe S7); un modelo por defecto del adaptador cuando `AI_MODEL` está vacío (descartado: inventaría un nombre de modelo que fallaría en la llamada; la unidad 2.2 lo corrigió a propósito).
- **Tradeoffs:** un despliegue con la IA mal configurada arranca "bien" y sólo falla cuando alguien pide un análisis; el aviso y el estado `unavailable` son la única señal, y por eso la web los muestra en lugar de fallar en silencio.

### D8 — Consentimiento explícito por usuario, comprobado dos veces
- **Context:** `users.ai_enabled` nació con `DEFAULT true`, así que un `true` almacenado **no prueba** que el usuario aceptase alguna vez el aviso de transferencia. El usuario eligió "exigir nueva aceptación a todos (recomendado)".
- **Decision:** la migración `00014` pone el default en `false` y resetea los `true` heredados. El `Down` no reescribe filas ni restaura `DEFAULT true` (restaurarlo consentiría automáticamente a las cuentas creadas después del *rollback*). En `PATCH /api/v1/me/preferences`, un `ai_enabled` omitido se guarda `false`: **omitir nunca es consentir**. El consentimiento se comprueba **al encolar** (API → `403`) y **antes de enviar** (job → `consent_revoked`), de modo que revocar entre la petición y la ejecución detiene el envío.
- **Rationale:** la invalidação es irreversible a propósito: la información que distinguía un consentimiento real del valor heredado ya no existe.
- **Alternatives:** una columna de versión del aviso (descartado: amplía el contrato público del perfil); comprobar el consentimiento sólo en la API (descartado: deja una ventana entre encolar y enviar).
- **Tradeoffs:** el backend no puede verificar que el cliente haya mostrado el aviso: un cliente API podría enviar `ai_enabled: true` sin leerlo. La mitigación es la acción deliberada (casilla) + el reseteo (ningún consentimiento heredado sobrevive).

### D9 — El job de River es la única puerta de salida
- **Context:** "reintentos con backoff, ejecución en jobs" (S6). Si tres sitios pudieran llamar al proveedor, cada uno tendría que reimplementar consentimiento, *payload*, validación y sanitización.
- **Decision:** `internal/jobs/ai_analysis.go` es la única unidad que llama a un proveedor. La API **no** habla con el proveedor: crea la fila `pending` y encola. El job: (1) re-comprueba el consentimiento, (2) comprueba la configuración global, (3) comprueba la ventana, (4) construye el *payload* con métricas almacenadas, (5) llama al proveedor una vez, (6) persiste `completed` o `failed`, (7) decide el reintento según `ai.Retryable(err)`.
- **Rationale:** un único punto donde auditar "qué sale del sistema" y "bajo qué permiso".
- **Alternatives:** la API llama al proveedor directamente (descartado: bloquea la petición y pierde el *backoff*); un worker genérico de "IA" reutilizable (descartado: todavía no hay un segundo tipo de análisis).
- **Tradeoffs:** la latencia del análisis no está en la respuesta HTTP (el usuario ve `pending` y sondea); el resultado es reproducible bajo demanda, y por eso las filas se pueden re-crear sin miedo.

### D10 — Persistencia en `ai_analysis` con columnas de error acotadas
- **Context:** hace falta guardar qué se envió, qué volvió y por qué falló, sin que el esquema admita un cuerpo de proveedor o una credencial.
- **Decision:** la tabla `ai_analysis` (migración `00015`) tiene una fila por `(user_id, kind, scope)` (`UNIQUE`), `status ∈ {pending, completed, failed}`, `provider`/`model`, `payload` (jsonb), `output` (jsonb), `error_code` (`CHECK ~ '^[a-z0-9_]{1,64}$'`) y `error_message` (`length <= 500`). La repetición de una petición **actualiza** la misma fila y la reinicia en `pending` (`ON CONFLICT DO UPDATE`).
- **Rationale:** los `CHECK` son la garantía estructural: un cuerpo de proveedor o una credencial **no caben** en esas columnas, ni siquiera por un error de programación.
- **Alternatives:** guardar el análisis en el perfil del usuario (descartado: mezcla datos de cuenta con datos de análisis); guardar el cuerpo crudo del proveedor (descartado: es exactamente lo que la columna `error_code` evita).
- **Tradeoffs:** el *payload* enviado queda en la base de datos (es agregado y por eso se acepta); la tabla es caché interpretativa y se puede borrar sin pérdida funcional.

### D11 — Presupuesto de reintentos acotado y códigos de error seguros
- **Context:** "reintentos con backoff" (S6) sin dejar que un proveedor caído consuma la cola.
- **Decision:** `AIAnalysisMaxAttempts(cfg) = 1 + AI_MAX_RETRIES`, con `AI_MAX_RETRIES ∈ [0, 10]` (por defecto 3) y el valor usado en `river.InsertOpts{MaxAttempts}`. El *backoff* es el de River (exponencial, `ATTEMPT^4`). La clasificación la decide el error: reintentables (`rate_limited`, `provider_error`, `network`, `timeout`) → el job **devuelve** el error para que River reintente; permanentes (`unauthorized`, `forbidden`, `bad_request`, `invalid_model`, `refusal`, `truncated`, `invalid_json`, `non_json_response`, `oversized_body`, `redirect`, `unexpected_response`, `schema_violation`) → el job **termina sin error**. Los códigos propios del producto (`consent_revoked`, `ai_disabled`, `ai_unavailable`, `not_found`, `invalid_scope`, `invalid_window`, `window_too_large`, `invalid_metrics`, `provider_mismatch`, `analysis_failed`) son tokens en minúsculas con un mensaje corto escrito por la aplicación.
- **Rationale:** el presupuesto se calcula de la configuración, no está cableado: el mismo número gobierna el encolado y el producto.
- **Alternatives:** reintentos ilimitados (descartado: coste y cola); reintentar también los fallos permanentes (descartado: un 401 no se arregla esperando).
- **Tradeoffs:** con `AI_MAX_RETRIES = 3` un proveedor lento agota la cola en pocos minutos; el estado `failed` con su código es la señal que ve el usuario.

### D12 — La API comprueba en orden y no crea nada cuando rechaza
- **Context:** una petición de análisis puede ser inválida por cinco motivos distintos y cada uno merece un código distinto (503 / 403 / 422 / 404 / 401).
- **Decision:** `POST /api/v1/ai/analyses` comprueba, en este orden: usuario autenticado (401), IA encendida y disponible (503), cola de jobs presente (503), consentimiento (403), cuerpo válido (422) y pertenencia del objetivo (404). Sólo después crea la fila `pending` y encola el job con el presupuesto de intentos. Ningún camino de rechazo deja una fila.
- **Rationale:** el orden hace que el error más accionable llegue primero (sin IA no tiene sentido hablar de consentimiento) y que el estado 503 sea la respuesta "el producto funciona igual".
- **Alternatives:** un único `400` con un código de negocio (descartado: el repo ya usa RFC 9457 con estados HTTP); validar el cuerpo antes del consentimiento (descartado: filtraría información de la cuenta a un cliente sin consentir).
- **Tradeoffs:** el backend no puede verificar que el cliente haya mostrado el aviso; con la IA apagada, el cliente API recibe `503` sin poder distinguir "apagada" de "no disponible" (el `detail` sí lo dice).

### D13 — El control web lee el estado antes de ofrecer nada
- **Context:** la web es el cliente mayoritario. Si monta el botón sin mirar el estado, un producto con la IA apagada emite peticiones que van a recibir `503`.
- **Decision:** `AIAnalysisControl` lee `GET /api/v1/ai/status` al montar. Con la IA desactivada, no disponible o sin consentimiento **no sale ninguna petición**: el control explica qué falta y enlaza a `/perfil`. Sólo cuando el estado es `ready` y hay consentimiento se ofrece el botón; entonces pide el análisis y lo sondea cada 2 s (máximo 60 intentos).
- **Rationale:** "con IA desactivada o caída, el producto funciona igual" (S7) es también una afirmación sobre el tráfico de red: no hay peticiones que puedan fallar.
- **Alternatives:** montar el botón siempre y dejar que el `503` se pinte (descartado: ruido para el usuario); el botón deshabilitado sin explicación (descartado: el usuario no sabe qué hacer).
- **Tradeoffs:** la web depende de un endpoint de estado para poder ser útil; a cambio, el estado `disabled` y `not consented` son explicados, no inferidos.

### D14 — El análisis de carga sigue el periodo del panel; el semanal va fijado a 7 días
- **Context:** la *scope* de un análisis (`ai_analysis.scope`) es la clave `kind:…` y **no codifica la longitud de la ventana** en el caso semanal: `weekly:<YYYY-MM-DD>`. Dos semanales con ventana distinta colisionarían en la misma fila.
- **Decision:** el análisis semanal tiene una ventana **fija de 7 días** (`jobs.AIAnalysisWindowDays`): un `window_days` distinto en la API es un `422`, no un análisis silencioso de otra ventana. El análisis de carga sí codifica los días (`load:<YYYY-MM-DD>:<días>`) y acepta de 1 a 90 días, con 28 por defecto; la web lo acopla al periodo elegido en el panel.
- **Rationale:** la clave de *scope* es la identidad de la fila; si no distingue dos ventanas distintas, deja de ser una identidad.
- **Alternatives:** codificar los días también en la clave semanal (descartado: cambiaría la clave ya entregada y obligaría a migrar filas); permitir cualquier ventana semanal (descartado: colisiones silenciosas).
- **Tradeoffs:** el semanal no es configurable en longitud; es el precio de una clave de *scope* estable y no ambigua.

### D15 — River arranca con Strava **o** con IA activa
- **Context:** antes de esta fase, la cola de jobs arrancaba sólo con credenciales de Strava. Una instancia sin Strava se quedaba sin cola, y con ella se quedaba sin análisis de IA.
- **Decision:** `internal/app/app.go` arranca River cuando `cfg.Strava != nil` **o** hay analizador de IA, y `internal/jobs/workers.go` registra el worker `ai_analysis` siempre que exista analizador. `buildAIAnalyzer(cfg)` devuelve `nil` (sin error) cuando la IA está apagada, no disponible o mal configurada.
- **Rationale:** la IA es una funcionalidad opcional **pero completa**: no depende de Strava.
- **Alternatives:** exigir Strava para la IA (descartado: acopla dos funcionalidades independientes); una cola separada para la IA (descartado: duplica la infraestructura de River).
- **Tradeoffs:** una instancia con IA activa y sin Strava ahora abre la cola (más recursos, un *worker* más); la API sin ninguna de las dos sigue en modo *API-only*.

## Architecture overview

```text
                    ┌──────────────────────── web/ (React 19 + Vite) ───────────────────────┐
                    │  /actividades            /dashboard                                 │
                    │   └ AIAnalysisControl      └ AIAnalysisControl (weekly | load)       │
                    │            │                          │                                │
                    │            │  GET /api/v1/ai/status   │  POST /api/v1/ai/analyses      │
                    │            │  POST /api/v1/ai/analyses│  GET /api/v1/ai/analyses      │
                    └────────────┼──────────────────────────┼───────────────────────────────┘
                                 ▼                          ▼
┌──────────────────────────────────────────────────────────────────────────────────────────────┐
│ Go (chi + pgx + goose + River)                                                               │
│                                                                                              │
│  router /api/v1/ai ──► handlers/ai_analysis.go                                               │
│      ├─ 503 si IA no disponible / sin cola      (no crea fila)                              │
│      ├─ 403 si no hay consentimiento             (no crea fila)                              │
│      ├─ 422 cuerpo inválido | 404 objetivo ajeno  (no crea fila)                             │
│      └─ 202 crea ai_analysis(pending) + encola AIAnalysisArgs{MaxAttempts: 1+AI_MAX_RETRIES} │
│                          │                                                                   │
│                          ▼                                                                   │
│  jobs/ai_analysis.go  (ÚNICA puerta de salida)                                              │
│      1 consentimiento → 2 config global → 3 ventana → 4 payload agregado → 5 proveedor       │
│      6 persistir (completed | failed) → 7 retry? (ai.Retryable)                              │
│                          │                                                                   │
│                          ▼  ai.Analyzer (D1)                                                 │
│              openai.go │ claude.go │ openrouter.go   (orden ADR 0002, sin failover, D2)      │
│                                                                                              │
│  ai/payload.go (lista blanca, topes por ventana)   ai/output.go (esquema único + validador)  │
└──────────────────────────────────────────────────────────────────────────────┬───────────────┘
                                ▲                                            │
                                │ sólo métricas agregadas (D4)               ▼
                     ┌──────────┴───────────┐                  proveedor externo (1 intento, D3)
                     │  PostgreSQL 16       │                  ┌────────────────────────────┐
                     │  ai_analysis (00015)  │                  │ OpenAI → Claude → OpenRouter│
                     └──────────────────────┘                  └────────────────────────────┘
```

## Data flow

### Petición de análisis (usuario → resultado)

1. El control web lee `GET /api/v1/ai/status`. Si `status != "ready"` o
   `consented == false`, **no sale ninguna petición** (D13).
2. `POST /api/v1/ai/analyses` con `kind` + objetivo. La API valida en
   orden y, si pasa, crea la fila `pending` (`ON CONFLICT` la reinicia) y
   encola el job con el presupuesto de intentos (D11, D12).
3. El job comprueba el consentimiento **otra vez** (D8) antes de
   cualquier contacto con el proveedor; si falta, deja `failed` con
   `consent_revoked` y **no** llama a nadie.
4. El job construye el *payload* con métricas ya almacenadas: para
   `activity`, la fila de la actividad cargada por `(id, user_id)`; para
   `weekly`/`load`, las actividades y la carga diaria de la ventana,
   contando los días en la zona horaria del usuario (`users.timezone`).
5. El job llama al proveedor configurado **una vez**, con el contexto
   acotado por `AI_TIMEOUT`, y valida la respuesta con el validador
   compartido.
6. El job persiste `completed` (payload + salida validada) o `failed`
   (código y mensaje seguros). Un fallo reintentable devuelve el error:
   River reintenta con su *backoff* y la fila pasa a `completed` si el
   reintento lo consigue.

### Configuración y arranque

- `config.Load()` lee la IA con `loadAIConfig()`, que **nunca** falla
  (D7). `app.run()` construye el analizador con `buildAIAnalyzer(cfg)`
  (sólo si `cfg.AI.Available`), arranca River con Strava **o** IA (D15) y
  registra el worker de IA si hay analizador.
- Con `AI_ENABLED` apagado: no se lee ninguna credencial, no hay
  analizador, no hay worker de IA y la API responde `503` en
  `POST /api/v1/ai/analyses`.

## Sequence diagram — petición bajo demanda

```text
 Usuario        web (control)        API Go              ai_analysis        River        Proveedor
    │                │                  │                    │                │              │
    │─pulsa "Analizar con IA"─────────►│                    │                │              │
    │                │─POST /api/v1/ai/analyses──────────────►│                │              │
    │                │                  │─comprueba: flag/cola/consentimiento/ cuerpo/ │
    │                │                  │  pertenencia────────────────────────────────►  │
    │                │                  │─INSERT ai_analysis(pending)──►│                │
    │                │                  │─encola AIAnalysisArgs{MaxAttempts}────────►│ │
    │                │◄─202 {status:"pending"}──────────────────│                │              │
    │                │─GET ...?kind&scope (cada 2 s)───────────►│                │              │
    │                │◄─200 {status:"pending"}──────────────────│                │              │
    │                │                  │                    │  ── job ──►  │              │
    │                │                  │                    │   1 consentimiento (users.ai_enabled)│
    │                │                  │                    │   2 config global │              │
    │                │                  │                    │   3 ventana ≤ 90 d│              │
    │                │                  │                    │   4 payload agregado ────────────► │
    │                │                  │                    │   5 una llamada  │  ── HTTP ──►  │
    │                │                  │                    │                │◄─ JSON ───────│
    │                │                  │                    │   6 validar con el esquema único │
    │                │                  │                    │   7 UPDATE completed |              │
    │                │─GET ...?kind&scope─────────────────────►│                │              │
    │                │◄─200 {status:"completed", output{summary,observations,cautions}}───│
    │◄─pinta la salida validada──────│                    │                │              │
```

## Reglas de degradación

| Situación | Comportamiento observable | Estado / código |
|---|---|---|
| `AI_ENABLED` apagado | No se lee ninguna credencial; la API responde `200` en el estado y `503` al pedir análisis; el control no pide nada | `status:"disabled"` |
| Proveedor inválido, credencial ausente o `AI_MODEL` vacío | Aviso en el arranque; el arranque **no** falla; no hay analizador ni worker | `status:"unavailable"` |
| Sin cola de jobs (ni Strava ni IA activa) | `POST` responde `503` sin crear fila | `503` |
| Sin consentimiento | `POST` responde `403` sin crear fila; el control explica y enlaza a `/perfil` | `403` |
| Consentimiento revocado entre encolar y enviar | El job marca la fila `failed` y **no** llama al proveedor | `failed` / `consent_revoked` |
| Proveedor caído (5xx, red, 429) | Fila `failed` con el motivo; el job devuelve el error y River reintenta con *backoff* | `failed` / `rate_limited`, `provider_error`, `network`, `timeout` |
| Proveedor con error permanente (401, 403, 4xx, esquema inválido) | Fila `failed`; el job termina **sin** error: no se reintenta | `failed` / `unauthorized`, `forbidden`, `bad_request`, `invalid_model`, `refusal`, `schema_violation`, … |
| Respuesta con métrica inventada o con vocabulario de diagnóstico | La respuesta se descarta como un fallo del proveedor | `failed` / `schema_violation` |
| Encolado fallido (River no acepta el insert) | La API responde `503`; la fila queda en `pending` hasta que la misma petición la reinicia | `pending` (⚠ ver `archive-report.md`) |
| Todo lo anterior | El resto del producto (actividades, panel, `/healthz`, preferencias) funciona **igual** | — |

## Contrato de cableado Go API ↔ cliente TS

El cliente de `web/` es el espejo exacto del JSON del backend; no hay
transformación intermedia.

| Go (`internal/http/handlers/ai_analysis.go`) | TypeScript (`web/src/lib/api/ai.ts`) | Tipo |
|---|---|---|
| `AIStatusResponse` | `AIStatusResponse` | `{ enabled, available, status, provider, consented }` |
| `AIAnalysisOutput` | `AIAnalysisOutput` | `{ summary, observations, cautions }` |
| `AIAnalysisResponse` | `AIAnalysis` | `{ id, kind, scope, status, provider, model, output, error_code, error_message, created_at, updated_at }` |
| `AIAnalysisListResponse` | `AIAnalysisListResponse` | `{ data: AIAnalysis[], limit: number }` |
| `aiAnalysisRequest` | `AIAnalysisRequest` | `{ kind, activity_id?, window_start?, window_days? }` |

Reglas del contrato:

- `kind` ∈ `activity | weekly | load`; el cuerpo **nunca** mezcla campos
  de actividad (`activity_id`) con campos de ventana
  (`window_start`, `window_days`): el backend lo rechaza con `422`.
- `status` ∈ `pending | completed | failed`; `status` (global) ∈
  `disabled | unavailable | ready`. Son vocabularios distintos: el
  primero describe un análisis, el segundo la feature.
- El `payload` enviado al proveedor **no** existe en el contrato HTTP:
  la API no lo expone (ni siquiera para el usuario que lo pidió).
- El cliente convierte los errores en `ApiError` con `status`, `detail`
  del problema y el mapa `fields` de los errores por campo (misma
  convención que `profile.ts`).

## File-by-file por unidad

### Unidad 1.1 — Contrato del dominio (`.`)
- `internal/ai/payload.go`: `Kind`, `Metrics`, `Payload`, `Builder`,
  `Limits`, `LimitsFor`, `Build`, `MarshalJSON`.
- `internal/ai/output.go`: `Output`, `Schema`, `OutputSchema()`,
  `ValidateOutput`, `ParseOutput`, límites de longitud y vocabulario
  vetado.
- `docs/architecture/ai-contract.md`: el contrato en prosa (qué sale,
  qué vuelve, qué se rechaza).

### Unidad 1.2 — Adaptadores (`.`)
- `internal/ai/analyzer.go`: `Analyzer`, `ProviderConfig`, `NewAnalyzer`,
  `Error`, `ErrorKind`, `Reason`, `Retryable`, `SupportedProviders`,
  `ParseProvider`, límites (`MaxResponseBytes`, `DefaultHTTPTimeout`).
- `internal/ai/openai.go`, `claude.go`, `openrouter.go`: un adaptador por
  proveedor, un intento, sin redirecciones, respuesta acotada.

### Unidad 2.1 — Consentimiento (`.` + `web/`)
- `internal/db/migrations/00014_ai_explicit_opt_in.sql`,
  `internal/db/schema.sql`,
  `internal/http/handlers/me_preferences.go` (campo `ai_enabled`),
  `web/src/features/profile/PreferencesForm.tsx` (aviso de transferencia),
  `docs/architecture/ai-consent.md`.

### Unidad 2.2 — Configuración y persistencia (`.`)
- `internal/config/config.go` (`AIConfig`, `loadAIConfig`, `Status`,
  `LogWarnings`).
- `internal/db/migrations/00015_ai_analysis.sql`,
  `internal/db/queries/ai_analysis.sql`, `internal/db/sqlc/ai_analysis.sql.go`,
  `internal/db/schema.sql`.
- `docs/architecture/ai-runtime.md`.

### Unidad 3.1 — Job (`.`)
- `internal/jobs/ai_analysis.go` (args, worker, códigos seguros, presupuesto
  de intentos), `internal/ai/scope.go` (claves de *scope*),
  `internal/db/queries/activities_ext.sql` (`GetActivityForUser`,
  `ListActivitiesInRange`).

### Unidad 3.2 — Cableado (`.`)
- `internal/app/app.go` (`buildAIAnalyzer`, arranque de River),
  `internal/jobs/workers.go` (registro del worker).

### Unidad 3.3 — Topes por ventana (`.`)
- `internal/ai/payload.go` (`LimitsFor`), `internal/ai/payload_window_test.go`.

### Unidad 4.1 — API (`.` + `web/`)
- `internal/http/handlers/ai_analysis.go`, `internal/http/router.go`
  (grupo `/api/v1/ai`), `web/src/lib/api/ai.ts`, `web/src/lib/api/ai.test.ts`,
  `docs/architecture/ai-api.md`.

### Unidad 4.2 — Web (`web/`)
- `web/src/features/ai/AIAnalysisControl/` (control compartido y sus
  constantes), `web/src/features/activities/ActivityList/ActivityList.tsx`
  (disparador por actividad),
  `web/src/features/dashboard/DashboardAIAnalysis.tsx` y
  `web/src/features/dashboard/aiWindow.ts`.
