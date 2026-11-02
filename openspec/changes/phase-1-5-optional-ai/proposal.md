# Proposal: phase-1-5-optional-ai

> Fase 1.5 close-out slice. Cierra la issue epic #17 ("Fase 1.5 —
> IA opcional") entregando la **interpretación con IA como capacidad
> opcional**: interfaz multi-proveedor (OpenAI → Claude → OpenRouter,
> ADR 0002), *feature flag* global + consentimiento por usuario,
> tres análisis bajo demanda (actividad / semanal / carga),
> *payload* agregado con esquema de salida validado e idéntico entre
> proveedores, y reintentos con *backoff* ejecutados en jobs de River
> con persistencia en `ai_analysis`.
>
> User-confirmed delivery strategy: **cadena sobre la rama de la épica**
> (rama integradora `feat/phase-1-5-optional-ai`, unidades de trabajo
> pequeñas, revisión progresiva antes de incorporar la épica a `main`).
> Base: `main@a6adff0`. Entregado: 9 work-unit commits, ~13.4 k líneas
> añadidas (≈6.9 k de ellas de test).

## Intent

La petición original del usuario fue literalmente *"Vamos con la issue
#17"*, y el objetivo de la issue es **"Enriquecer la interpretación sin
hacerla obligatoria"**. Esa última frase es la restricción que gobierna
todo el diseño de la fase: la IA **enriquece**, nunca **sustituye** ni
**bloquea**.

El repositorio ya calculaba y persistía todas las métricas deterministas
que un análisis de entrenamiento necesita (`activities` con distancia,
duración, desnivel y FC media; `training_load_daily` con la carga diaria;
`user_preferences` con zona horaria y rangos), pero no existía ninguna
superficie que las **interpretara**. `docs/decisions/0002-ai-provider.md`
ya fijaba el proveedor y su orden de preferencia; `users.ai_enabled`
existe desde la migración `00003_user_preferences.sql` pero con
`DEFAULT true`, es decir, **sin valor probatorio**: una casilla marcada
que nadie ha visto nunca puede probar que el usuario aceptó transferir
sus datos a un tercero.

Por eso esta fase no es "añadir un botón de IA" sino **cerrar un
consentimiento que no existía** y, sobre esa base, abrir una salida
*opcional* y auditada:

1. **Consentimiento real y explícito** (S3): la migración `00014` pone el
   valor por defecto en `false` y resetea los `true` heredados, de modo
   que **toda** cuenta debe volver a aceptar el aviso de transferencia.
   Omitir el campo nunca concede consentimiento.
2. **Puerta de salida única y auditada** (S5, S6): un `payload` agregado
   (lista blanca de cinco métricas) construido por un *builder* sin
   campos de identidad, nombre, GPS ni *streams*; una respuesta validada
   con **un** esquema estricto, el mismo para los tres proveedores; una
   tabla `ai_analysis` que guarda exactamente lo que se envió y lo que
   volvió.
3. **Todo bajo demanda** (S4): nada se calcula ni se cobra sin que el
   usuario pulse un botón. Los tres análisis (actividad, semanal, carga)
   se piden desde las pantallas que ya existen.
4. **Degradación sin drama** (S7): con la IA apagada, mal configurada o
   caída, el producto determinista funciona **igual**. La configuración
   inválida no impide el arranque; el job termina sin error; la API
   responde `503`/`403` con un detalle accionable.

## Scope

### In scope

- **Contrato del dominio** (`internal/ai`, proyecto Go `.`):
  - `Payload`/`Builder` con lista blanca: `kind`, `window_days`,
    `distance_meters`, `duration_seconds`, `elevation_gain_meters`,
    `avg_heart_rate`, `training_load`.
  - Topes **por ventana** (`ai.LimitsFor(kind, windowDays)`): el tope de
    un agregado *sumado* es el tope por día × días de la ventana; la
    actividad conserva sus topes por actividad; la FC media nunca escala.
  - Un único esquema de salida estricto (`ai.OutputSchema()`) y un único
    validador Go (`ai.ValidateOutput`, `ai.ParseOutput`) reutilizado por
    los tres proveedores.
  - Claves de *scope* deterministas (`ai.ActivityScope`, `ai.WeeklyScope`,
    `ai.LoadScope`) compartidas por la API y el job.
- **Adaptadores de proveedor** (`internal/ai`, Go `.`): `ai.Analyzer` +
  `ai.NewAnalyzer` (sin *failover* implícito), un adaptador por proveedor
  en el orden de ADR 0002 (OpenAI → Claude → OpenRouter), clasificación
  de fallos reintentable/permanente y sanitización de errores.
- **Configuración global** (`internal/config`, Go `.`): `AI_ENABLED`
  (apagado por defecto), `AI_PROVIDER`, `AI_MODEL`, una credencial por
  proveedor, `AI_TIMEOUT`, `AI_MAX_RETRIES`. `Load()` **nunca** falla por
  la IA: cada problema deja un aviso y el estado `unavailable`.
- **Consentimiento por usuario** (Go `.` + `web/`): migración `00014`,
  aviso de transferencia en el formulario de preferencias, y las mismas
  semánticas de omisión en `PATCH /api/v1/me/preferences`.
- **Persistencia** (Go `.`): migración `00015` con la tabla `ai_analysis`
  (`UNIQUE (user_id, kind, scope)`, `CHECK` sobre `kind`, `status`,
  `error_code` y `error_message`), las cinco consultas *sqlc* y el
  generador de código.
- **Job de River** (Go `.`): `ai_analysis`, única puerta de salida hacia
  un proveedor; re-comprueba el consentimiento antes de enviar; construye
  el *payload* sólo con métricas almacenadas; llama al proveedor una vez;
  persiste el resultado; clasifica el fallo para el *backoff*.
- **API** (Go `.`): `GET /api/v1/ai/status`, `POST /api/v1/ai/analyses`,
  `GET /api/v1/ai/analyses` dentro del grupo autenticado `/api/v1`, con
  errores RFC 9457.
- **Web** (`web/`): cliente `web/src/lib/api/ai.ts`, control compartido
  `web/src/features/ai/AIAnalysisControl/`, disparador por actividad en
  `/actividades` y controles semanal + de carga en `/dashboard`.
- **Tests** en cada frontera: `internal/ai` (contrato y adaptadores con
  `httptest`, **nunca** peticiones reales), `internal/config`,
  `internal/jobs`, `internal/http/handlers`, `internal/app` y la suite
  web (`vitest`).

### Out of scope

- **Cualquier SDK de proveedor.** Los adaptadores usan `net/http` de la
  biblioteca estándar: cero dependencias nuevas.
- **IA automática o programada.** Todo es bajo demanda (S4); no hay
  cron, ni *webhook* que dispare un análisis.
- **Continuidad entre proveedores.** Un fallo nunca salta a otro
  proveedor por su cuenta (S5, resolución del usuario).
- **Métricas crudas, GPS, identidad o texto libre** en el *payload*. No
  hay por dónde filtrarlos: no existen campos para ellos.
- **Diagnóstico médico.** El vocabulario de diagnóstico está vetado en la
  salida validada.
- **Cualquier dato que no sea agregado**: nada de *streams*, nombres,
  correos ni correos electrónicos del proveedor.
- **Promoción de los *deltas* a `openspec/specs/`**: es el paso final de
  la épica y lo hace el padre, no esta unidad.
- **Descargas de *backfill***, comparador, OAuth/Strava, GPX: los mismos
  límites que fijaron las fases 1.3 y 1.4.

## Approach

### Decisiones de arquitectura (resumen; detalle en `design.md`)

| # | Decisión | Motivo |
|---|---|---|
| D1 | Interfaz `ai.Analyzer` + registro en el orden de ADR 0002 | El dominio no conoce al proveedor; el proveedor no conoce al dominio |
| D2 | Sin *failover* implícito | Transferir datos a otro proveedor sin permiso es una transferencia no consentida |
| D3 | Una sola llamada HTTP por `Analyze` | Los reintentos son del job, con *backoff* y presupuesto acotado |
| D4 | *Payload* agregado con lista blanca | La privacidad se sostiene por construcción (no hay campo por el que filtrar) |
| D5 | Topes por ventana (`ai.LimitsFor`) | Una ventana de N días es N veces un día; los topes fijos dejaban el semanal y la carga inservibles |
| D6 | Un único esquema de salida estricto | La salida aceptada no depende del proveedor; una métrica inventada se rechaza |
| D7 | Flag global que nunca bloquea el arranque | Con la IA mal configurada el producto determinista sigue igual (S7) |
| D8 | Consentimiento explícito por usuario, comprobado dos veces | "Al encolar" (API) y "antes de enviar" (job) |
| D9 | Job de River como única puerta de salida | Un solo punto donde auditar y bloquear la salida de datos |
| D10 | Persistencia en `ai_analysis` con columnas de error acotadas | El cuerpo del proveedor o una credencial no caben en el esquema |
| D11 | API que no habla con el proveedor | La API encola; el job es el que llama. La API es rápida y auditable |
| D12 | Control web que lee el estado antes de ofrecer nada | Con la IA apagada o sin consentimiento no sale ninguna petición |

### Impacto

| Área | Impacto |
|---|---|
| Migraciones | `00014_ai_explicit_opt_in.sql` (cambia el *default* de `users.ai_enabled` a `false` y resetea los `true` heredados) y `00015_ai_analysis.sql` (tabla `ai_analysis`) |
| Configuración | Nuevas variables `AI_ENABLED`, `AI_PROVIDER`, `AI_MODEL`, `AI_OPENAI_API_KEY`, `AI_CLAUDE_API_KEY`, `AI_OPENROUTER_API_KEY`, `AI_TIMEOUT`, `AI_MAX_RETRIES` |
| API | Tres rutas nuevas bajo `/api/v1/ai`, todas autenticadas |
| Jobs | Worker `ai_analysis`; River arranca con Strava **o** con IA activa |
| Web | Un cliente nuevo, un control compartido y dos montajes (actividades y panel) |
| Dependencias | **Ninguna** nueva en Go ni en `web/` (sólo `net/http` y la biblioteca estándar de TS) |
| Superficie pública | El contrato de `GET`/`PATCH /api/v1/me/preferences` no cambia: `ai_enabled` se suma como campo opcional con semántica de omisión |
| Rompe algo existente | No: `/healthz`, los endpoints deterministas y las preferencias siguen igual |

### Rollback

El *rollback* es por unidad de trabajo (cada commit es un *work-unit* con
su propio *down*), en **orden inverso** al de entrega.

1. **`8265703` (web)** — revertir elimina `web/src/features/ai/`,
   `web/src/features/dashboard/DashboardAIAnalysis.*`,
   `web/src/features/dashboard/aiWindow.*` y el disparador de
   `ActivityList`. La web vuelve a las pantallas deterministas. El
   backend sigue sirviendo `/api/v1/ai/*` (no hay consumidor).
2. **`4de56a5` (API)** — revertir elimina las tres rutas y el cliente
   `web/src/lib/api/ai.ts`. No hay ningún `ai_analysis` en curso: las
   filas ya escritas quedan como caché (obsoletas, sin consumidor).
3. **`67be97a` (topes por ventana)** — revertir devuelve los topes
   fijos; el semanal y la carga vuelven a fallar con `invalid_metrics`.
   Es un *revert* seguro: sólo cambia la validación.
4. **`b8d9360` (cableado)** — revertir devuelve el arranque de River a
   "sólo con Strava". Con la IA activa y sin Strava la cola no arranca:
   la API responde `503` en `POST /api/v1/ai/analyses`.
5. **`5cdd64c` (job)** — revertir elimina el worker `ai_analysis`. Las
   filas `pending` quedan sin procesar (no se reintentan).
6. **`8d823f9` (adaptadores)** — revertir elimina OpenAI/Claude/
   OpenRouter. `internal/ai` conserva el contrato del *payload* y del
   esquema; el job no encuentra analizador y marca `ai_unavailable`.
7. **`fc6c54f` (configuración + persistencia)** — revertir deja la
   tabla `ai_analysis` en disco (la migración `00015` es la que la
   borra) y `config.AI` sin usar. **La migración `00015` se revierte
   con `goose Down`** y elimina la tabla: no hay datos que conservar (los
   análisis son caché interpretativa reproducible bajo demanda).
8. **`49f8585` (consentimiento)** — ⚠️ el `goose Down` de `00014` es
   **deliberadamente conservador**: no reescribe ningún valor almacenado
   y **no restaura `DEFAULT true`**, porque eso consentiría
   automáticamente a las cuentas creadas después del *rollback*. Es
   decir: el *rollback* nunca concede consentimiento; a lo sumo lo
   revoca. Una cuenta que había aceptado antes del *rollback* sigue
   aceptada, y las que habían sido reseteadas siguen reseteadas.
9. **`99db0a9` (contrato)** — revertir elimina `internal/ai`. Es la
   base de 5, 6 y 7, así que **debe revertirse el último**.

**Peor caso (revertir la unidad 9 entera):** el producto vuelve al estado
`main@a6adff0`: sin `internal/ai`, sin configuración de IA, sin tabla
`ai_analysis`, sin worker, sin rutas `/api/v1/ai/*` y sin controles web.
`users.ai_enabled` queda en `false` para toda cuenta y el aviso de
transferencia desaparece del perfil (la casilla se conserva: el campo
sigue existiendo, pero nada lo usa). **Ningún dato de usuario sale del
sistema en ese estado.**

### Proyectos en alcance y comandos de test

| Proyecto | Comando de test | Gates de calidad |
|---|---|---|
| `.` (Go) | `make test` (`GOTOOLCHAIN=local go test -race ./...`) | `make lint`, `make vet`, `make fmt` |
| `web/` | `pnpm -C web test:run` | `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web format:check` |

No existe un comando agregado de *workspace* (ver
`openspec/config.yaml:testing.strict_tdd_rationale`): las dos columnas
se ejecutan explícitamente.

## Verification

Comandos ejecutados por unidad (los mismos que se repiten en
`tasks.md` y en `archive-report.md`):

```bash
# Go (por unidad, según lo que toca)
TZ=UTC go test -count=1 ./internal/ai
TZ=UTC go test -count=1 ./internal/config ./internal/db/sqlc
TZ=UTC go test -count=1 ./internal/jobs ./internal/ai ./internal/db/sqlc
TZ=UTC go test -count=1 ./internal/http/... ./internal/app
TZ=UTC go test -race -count=1 ./...
TZ=UTC go vet ./internal/...

# Web
pnpm -C web test:run src/lib/api/ai.test.ts
pnpm -C web test:run src/features/ai src/features/dashboard src/features/profile
pnpm -C web test:run
```

### Fallo ambiental conocido

`pnpm -C web typecheck` **no** se puede ejecutar en este entorno: el
árbol de `web/node_modules` del sandbox no contiene `echarts`, que la
fase 1.4 (`283faf6`) ya había añadido a `web/package.json`. El mismo
comando falla de forma idéntica sobre el commit base, así que el fallo
**no** lo causa esta fase. El resto de la suite web (`vitest`) sí corre.

## Ready for Spec

**Sí.** Los *deltas* de esta carpeta describen el comportamiento
**entregado** (no la intención) de las tres capacidades:
`ai-analysis-backend`, `ai-analysis-api` y `ai-analysis-web`. La
promoción a `openspec/specs/` corresponde al paso final de la épica y
la ejecuta el padre.
