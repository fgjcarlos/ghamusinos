# Runtime de IA (fase 1.5 · issue #17)

Fundamento de ejecución y persistencia de la IA opcional: la **configuración
global** (que nunca impide el arranque), la tabla **`ai_analysis`** (un
análisis por usuario y objetivo) y el **job de River** que realiza un
análisis y lo persiste. La API (T7) todavía no existe; esta unidad deja el
contrato cerrado para que la reutilice sin decidir nada por su cuenta.

Con la IA apagada o mal configurada, el producto funciona igual (S7): la
configuración inválida produce un aviso y el estado `unavailable`, nunca un
error de arranque.

## Camino rápido

1. Despliegue: `AI_ENABLED=true` + `AI_PROVIDER=openai` +
   `AI_OPENAI_API_KEY=…` (sólo si se quiere IA).
2. La app lee `config.Config.AI`; `cfg.AI.Status()` dice `ready`,
   `disabled` o `unavailable`.
3. Una petición de análisis llama a `CreateAIAnalysis` (crea o reutiliza la
   fila) y encola el job `ai_analysis`.
4. El job comprueba el consentimiento y la configuración global, construye
   el payload agregado, llama al proveedor una vez y cierra la fila con
   `CompleteAIAnalysis` o `FailAIAnalysis`.

## Configuración global

| Variable | Por defecto | Qué hace |
|---|---|---|
| `AI_ENABLED` | `false` | Feature flag global. Acepta `1`, `true`, `yes`, `on` |
| `AI_PROVIDER` | — | Proveedor: `openai`, `claude` u `openrouter` (ADR 0002) |
| `AI_MODEL` | — | Modelo pedido al proveedor. Vacío = modelo por defecto del adaptador |
| `AI_OPENAI_API_KEY` | — | Credencial de OpenAI |
| `AI_CLAUDE_API_KEY` | — | Credencial de Claude |
| `AI_OPENROUTER_API_KEY` | — | Credencial de OpenRouter |
| `AI_TIMEOUT` | `30s` | Timeout de una llamada al proveedor |
| `AI_MAX_RETRIES` | `3` | Reintentos acotados (0..10) |

Reglas:

- **El flag apagado no necesita credencial.** Con `AI_ENABLED` apagado no se
  lee ninguna clave y no se envía nada a ninguna parte.
- **Cada proveedor tiene su clave y sólo se lee la del proveedor
  seleccionado.** Las claves de los demás proveedores no entran en la
  configuración en memoria.
- **Una IA mal configurada no impide el arranque.** `Load()` nunca falla por
  la IA: cada problema deja un aviso (`cfg.AI.Warnings`) y deja la IA en
  `unavailable` (`cfg.AI.UnavailableReason`).
- **Los avisos nunca filtran credenciales.** Sólo nombran variables de
  entorno y valores no secretos.
- **Los límites se validan siempre**, incluso con la IA apagada, para que un
  error tipográfico se vea antes de encender el flag.

### Estados

| Estado | Significado |
|---|---|
| `disabled` | `AI_ENABLED` apagado. La IA no se puede usar |
| `unavailable` | Flag encendido pero la configuración no permite analizar |
| `ready` | Flag encendido, proveedor válido y credencial presente |

## Persistencia: `ai_analysis`

Una fila por **usuario y objetivo de análisis**:

| Columna | Contrato |
|---|---|
| `id` | UUID, clave primaria |
| `user_id` | FK a `users(id)` con `ON DELETE CASCADE` |
| `kind` | `activity`, `weekly` o `load` (CHECK) |
| `scope` | El objetivo analizado: id de actividad o clave de ventana |
| `status` | `pending`, `completed` o `failed` (CHECK) |
| `provider` / `model` | Proveedor y modelo usados (texto seguro) |
| `payload` | `jsonb`: el payload agregado que se envió |
| `output` | `jsonb`: la respuesta validada (NULL si no hay) |
| `error_code` | Token seguro en minúsculas (NULL si no hay) |
| `error_message` | Frase corta de la aplicación (NULL si no hay) |
| `created_at` / `updated_at` | Auditoría |

Restricciones que sostienen el contrato:

- `UNIQUE (user_id, kind, scope)`: una petición repetida **actualiza** la
  misma fila en vez de duplicarla.
- `CHECK (error_code ~ '^[a-z0-9_]{1,64}$')` y `length(error_message) <=
  500`: el cuerpo del proveedor o una credencial **no caben** en las columnas
  de error.
- Índice `idx_ai_analysis_user_created (user_id, created_at DESC)` para el
  listado del usuario.

### Privacidad del payload almacenado

La columna `payload` guarda el payload **agregado** que se envió al
proveedor. `internal/ai` garantiza esa forma: sólo métricas numéricas
agregadas (distancia, duración, desnivel, frecuencia cardíaca media y carga
calculada), sin identidad, sin nombre, sin GPS y sin streams crudos. El
contrato completo está en [Contrato de IA](ai-contract.md) y el aviso de
transferencia en [Consentimiento de IA](ai-consent.md).

La columna `output` guarda la respuesta validada, que es interpretativa
(resumen, observaciones, cautiones) y no contiene métricas.

## Contrato exportado (Go)

### Configuración (`internal/config`)

```go
// Config
type Config struct {
    // …campos existentes…
    AI AIConfig
}

type AIConfig struct {
    Enabled           bool          // AI_ENABLED
    Available         bool          // flag + proveedor válido + credencial
    Provider          string        // "openai" | "claude" | "openrouter"
    Model             string        // "" = modelo por defecto del adaptador
    APIKey            string        // credencial del proveedor seleccionado
    Timeout           time.Duration // AI_TIMEOUT
    MaxRetries        int           // AI_MAX_RETRIES
    UnavailableReason string        // por qué no está disponible
    Warnings          []string      // avisos de misconfiguración
}

func (c AIConfig) Status() string // "disabled" | "unavailable" | "ready"
func (c AIConfig) LogWarnings(logger *slog.Logger)

const (
    AIProviderOpenAI     = "openai"
    AIProviderClaude     = "claude"
    AIProviderOpenRouter = "openrouter"

    AIStatusDisabled   = "disabled"
    AIStatusUnavailable = "unavailable"
    AIStatusReady      = "ready"

    DefaultAITimeout    = 30 * time.Second
    DefaultAIMaxRetries = 3
    MaxAIMaxRetries     = 10
)
```

Al arrancar, la app debe registrar los avisos (una vez) con
`cfg.AI.LogWarnings(slog.Default())` y montar la IA **sólo** si
`cfg.AI.Status() == config.AIStatusReady`.

### Consultas (`internal/db/sqlc`, generadas por sqlc)

| Query | Firma | Para qué |
|---|---|---|
| `CreateAIAnalysis` | `(ctx, CreateAIAnalysisParams) (AiAnalysis, error)` | Crear o reutilizar la fila (`pending`) |
| `GetAIAnalysis` | `(ctx, GetAIAnalysisParams) (AiAnalysis, error)` | Leer el análisis de un objetivo |
| `ListAIAnalyses` | `(ctx, ListAIAnalysesParams) ([]AiAnalysis, error)` | Listar los análisis del usuario |
| `CompleteAIAnalysis` | `(ctx, CompleteAIAnalysisParams) (AiAnalysis, error)` | Guardar payload + salida validada |
| `FailAIAnalysis` | `(ctx, FailAIAnalysisParams) (AiAnalysis, error)` | Guardar el error seguro |

Structs de parámetros (generados):

```go
type CreateAIAnalysisParams struct {
    UserID pgtype.UUID
    Kind   string
    Scope  string
}
type GetAIAnalysisParams struct {
    UserID pgtype.UUID
    Kind   string
    Scope  string
}
type ListAIAnalysesParams struct {
    UserID pgtype.UUID
    Limit  int32
}
type CompleteAIAnalysisParams struct {
    UserID   pgtype.UUID
    Kind     string
    Scope    string
    Provider pgtype.Text
    Model    pgtype.Text
    Payload  []byte
    Output   []byte
}
type FailAIAnalysisParams struct {
    UserID       pgtype.UUID
    Kind         string
    Scope        string
    Provider     pgtype.Text
    Model        pgtype.Text
    Payload      []byte
    ErrorCode    pgtype.Text
    ErrorMessage pgtype.Text
}
```

Modelo generado:

```go
type AiAnalysis struct {
    ID           pgtype.UUID
    UserID       pgtype.UUID
    Kind         string
    Scope        string
    Status       string
    Provider     pgtype.Text
    Model        pgtype.Text
    Payload      []byte
    Output       []byte
    ErrorCode    pgtype.Text
    ErrorMessage pgtype.Text
    CreatedAt    pgtype.Timestamptz
    UpdatedAt    pgtype.Timestamptz
}
```

**Aislamiento:** las cuatro consultas que no crean filtran siempre por
`user_id`. Un usuario no puede leer ni sobrescribir el análisis de otro:
`GetAIAnalysis`, `CompleteAIAnalysis` y `FailAIAnalysis` devuelven
`pgx.ErrNoRows` para un `user_id` ajeno.

**Aviso para la unidad T6 (API):** el repositorio genera la interfaz
`sqlc.Querier` con **todos** los métodos de todas las consultas
(`emit_interface: true` en `sqlc.yaml`), así que estas cinco consultas la
han hecho crecer. Los mocks que implementan la interfaz completa
(`internal/auth`, `internal/http`, `internal/http/handlers`) necesitan cinco
stubs nuevos:

```go
func (m *mock) CreateAIAnalysis(ctx context.Context, arg sqlc.CreateAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}
func (m *mock) GetAIAnalysis(ctx context.Context, arg sqlc.GetAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}
func (m *mock) ListAIAnalyses(ctx context.Context, arg sqlc.ListAIAnalysesParams) ([]sqlc.AiAnalysis, error) {
	return nil, nil
}
func (m *mock) CompleteAIAnalysis(ctx context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}
func (m *mock) FailAIAnalysis(ctx context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error) {
	return sqlc.AiAnalysis{}, nil
}
```

**Errores seguros:** `ErrorCode` es un token en minúsculas
(`provider_timeout`, `invalid_output`, …) y `ErrorMessage` una frase corta
escrita por la aplicación. El cuerpo que devuelve el proveedor nunca se
guarda.

## El job de análisis (T5)

`internal/jobs/ai_analysis.go` es la **única** puerta de salida hacia un
proveedor de IA. Hace un análisis y lo persiste; con la IA apagada o sin
consentimiento no sale nada y el producto sigue igual (S7).

### Camino rápido

1. La API (T7) encola `AIAnalysisArgs` con el scope del objetivo.
2. River llama a `AIAnalysisWorker.Work`.
3. El job comprueba el consentimiento, la configuración global y la
   ventana; construye el payload agregado; llama al proveedor una vez.
4. La fila queda `completed` (payload + salida) o `failed` (código y
   mensaje seguros).

### Orden de las comprobaciones

Cada paso es observable en los tests (`internal/jobs/ai_analysis_test.go`):

| # | Paso | Si falla |
|---|---|---|
| 1 | Consentimiento (`users.ai_enabled`) | Fila `failed` con `consent_revoked`, sin error y sin proveedor |
| 2 | Configuración global (`cfg.AI`) | Fila `failed` con `ai_disabled` o `ai_unavailable`, sin error y sin proveedor |
| 3 | Scope y ventana | Fila `failed` con `invalid_scope`, `invalid_window` o `window_too_large`, sin proveedor |
| 4 | Payload agregado | Fila `failed` con `not_found` (actividad ajena) o `invalid_metrics` |
| 5 | Proveedor (una llamada) | Fila `failed` con el `Reason` del proveedor; reintento sólo si `ai.Retryable(err)` |

El consentimiento se comprueba **antes** de tocar el proveedor: sin
`ai_enabled` no se construye ni se envía ningún payload.

### Qué entra en el payload

Sólo agregados ya almacenados. Nunca streams crudos, ni nombres, ni GPS,
ni texto libre.

| Análisis | Origen | Agregados |
|---|---|---|
| `activity` | La actividad del usuario (`GetActivityForUser`, filtrada por `user_id`) | Distancia, duración (tiempo en movimiento), desnivel, FC media y la carga almacenada del día |
| `weekly` | Actividades y `training_load_daily` de los 7 días | Sumas de distancia, duración y desnivel; FC media ponderada por duración; carga = suma de los `tss` almacenados |
| `load` | Actividades y `training_load_daily` de la ventana pedida | Igual que el semanal, con los días de la ventana |

- **Ausente ≠ cero.** Un agregado sin medir se omite; nunca se envía un `0`
  inventado.
- **La actividad de otro usuario no existe** para el job (`not_found`) y
  nunca se analiza.
- **La ventana está acotada** a `ai.MaxWindowDays` (90 días). Una ventana
  mayor falla con `window_too_large`: no se amplía en silencio.
- **Las ventanas se cuentan en los días locales** del usuario
  (`users.timezone`).
- Los límites del contrato de payload (`internal/ai`) siguen vigentes: si
  un agregado no cabe (por ejemplo, más de 7 días de tiempo total en una
  ventana de 90), el análisis falla con `invalid_metrics` en lugar de
  recortar los datos.

### Claves de scope

Una clave determinista por objetivo. La API y el job construyen **la misma**
clave con los mismos helpers (`internal/ai/scope.go`):

| Análisis | Clave | Helper |
|---|---|---|
| `activity` | `activity:<uuid>` | `ai.ActivityScope(id string) string` |
| `weekly` | `weekly:<YYYY-MM-DD>` (inicio de la ventana) | `ai.WeeklyScope(start time.Time) string` |
| `load` | `load:<YYYY-MM-DD>:<días>` | `ai.LoadScope(start time.Time, days int) string` |

`ai.ParseScope(scope string) (ai.ScopeTarget, error)` deshace la clave: el
job usa el scope para identificar el objetivo, de modo que el `activity_id`
o la ventana no tienen que viajar dos veces.

### Reintentos

| Tema | Decisión |
|---|---|
| Presupuesto | `jobs.AIAnalysisMaxAttempts(cfg)` = 1 + `AI_MAX_RETRIES` (nunca > `1 + config.MaxAIMaxRetries`) |
| Encolado | La API usa ese valor en `river.InsertOpts{MaxAttempts: …}` |
| Backoff | Lo aplica River con su política por defecto, que es exponencial (`ATTEMPT^4`) |
| Clasificación | `ai.Retryable(err)`: reintentable → el job devuelve el error; permanente → el job termina sin error |
| Estado de la fila | `failed` entre intentos; `completed` cuando un reintento lo consigue |

### Códigos de error seguros

`ai_analysis.error_code` cumple `^[a-z0-9_]{1,64}$` y
`error_message` tiene como mucho 500 caracteres. Ninguno lleva el cuerpo
del proveedor, el payload, la credencial ni la identidad del usuario.

| Código | Cuándo |
|---|---|
| `consent_revoked` | El usuario no aceptó el envío |
| `ai_disabled` | Flag global apagado |
| `ai_unavailable` | Configuración global no utilizable |
| `not_found` | El objetivo no existe o no es del usuario |
| `invalid_scope` | El scope no coincide con el objetivo |
| `invalid_window` / `window_too_large` | La ventana no es válida o supera el límite |
| `invalid_metrics` | Los agregados no caben en el contrato del payload |
| `provider_mismatch` | El analizador no es el proveedor configurado |
| `analysis_failed` | Fallo del proveedor sin motivo clasificado |
| `rate_limited`, `timeout`, `network`, `provider_error`, `unauthorized`, … | El `Reason` del error del proveedor (`ai.Reason`) |

### Contrato exportado (Go)

```go
// Jobs (internal/jobs/ai_analysis.go)
type AIAnalysisArgs struct {
    UserID       string `json:"user_id"`
    AnalysisKind string `json:"analysis_kind"`   // "activity" | "weekly" | "load"
    Scope        string `json:"scope"`
    ActivityID   string `json:"activity_id,omitempty"`
    WindowStart  string `json:"window_start,omitempty"` // YYYY-MM-DD
    WindowDays   int    `json:"window_days,omitempty"`
}

func (AIAnalysisArgs) Kind() string // "ai_analysis"

type AIAnalysisStore interface {
    GetUserPreferencesByID(ctx context.Context, id pgtype.UUID) (sqlc.GetUserPreferencesByIDRow, error)
    GetActivityForUser(ctx context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error)
    ListActivitiesInRange(ctx context.Context, arg sqlc.ListActivitiesInRangeParams) ([]sqlc.ListActivitiesInRangeRow, error)
    ListTrainingLoadRange(ctx context.Context, arg sqlc.ListTrainingLoadRangeParams) ([]sqlc.TrainingLoadDaily, error)
    CompleteAIAnalysis(ctx context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error)
    FailAIAnalysis(ctx context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error)
}

func NewAIAnalysisWorker(
    store AIAnalysisStore,
    analyzer ai.Analyzer,
    cfg config.AIConfig,
    logger *slog.Logger,
) *AIAnalysisWorker

func AIAnalysisMaxAttempts(cfg config.AIConfig) int
```

`*sqlc.Queries` satisface `AIAnalysisStore`. El analizador puede ser `nil`
(con la IA apagada): el job marca la fila como fallida sin llamar a nadie.

### Consulta nueva (sqlc)

| Query | Firma | Para qué |
|---|---|---|
| `GetActivityForUser` | `(ctx, GetActivityForUserParams) (GetActivityForUserRow, error)` | Actividad concreta **de un usuario**: el filtro por `user_id` es la frontera de aislamiento |

```go
type GetActivityForUserParams struct {
    ID     pgtype.UUID
    UserID pgtype.UUID
}
type GetActivityForUserRow struct {
    ID             pgtype.UUID
    UserID         pgtype.UUID
    SportType      string
    StartedAt      pgtype.Timestamptz
    ElapsedSeconds int32
    MovingSeconds  int32
    DistanceMeters pgtype.Numeric
    ElevationGainM pgtype.Numeric
    AvgHr          pgtype.Int2
}
```

La proyección es estrecha: sin nombre, sin `raw_payload` y sin streams.

## Dónde está cada cosa

| Pieza | Ruta |
|---|---|
| Configuración | `internal/config/config.go` |
| Tests de configuración | `internal/config/config_test.go` |
| Migración | `internal/db/migrations/00015_ai_analysis.sql` |
| Esquema SQLC | `internal/db/schema.sql` |
| Consultas | `internal/db/queries/ai_analysis.sql` |
| Código generado | `internal/db/sqlc/ai_analysis.sql.go` |
| Contrato estático | `internal/db/sqlc/ai_analysis_contract_test.go` |
| Contrato con PostgreSQL real | `internal/db/sqlc/ai_analysis_integration_test.go` |
| Job de análisis | `internal/jobs/ai_analysis.go` |
| Tests del job | `internal/jobs/ai_analysis_test.go` |
| Claves de scope | `internal/ai/scope.go` |
| Tests de scope | `internal/ai/scope_test.go` |

## Comprobación

```bash
TZ=UTC go test -count=1 ./internal/jobs ./internal/ai ./internal/db/sqlc
TZ=UTC go test -race -count=1 ./internal/jobs ./internal/ai
TZ=UTC go vet ./internal/jobs ./internal/ai
golangci-lint run ./internal/jobs/... ./internal/ai/...
make generate
```

## Checklist de revisión

- [ ] `AI_ENABLED` apagado por defecto y sin credencial obligatoria.
- [ ] Un proveedor inválido o una credencial ausente no impiden el arranque.
- [ ] Cada aviso nombra la variable de entorno y nunca una credencial.
- [ ] Sólo se carga la credencial del proveedor seleccionado.
- [ ] `ai_analysis` tiene `UNIQUE (user_id, kind, scope)` y FK en cascada.
- [ ] `kind` y `status` están acotados por `CHECK`.
- [ ] `error_code` y `error_message` no admiten un cuerpo de proveedor.
- [ ] Las consultas filtran por `user_id` (aislamiento entre usuarios).
- [ ] El código generado de sqlc está regenerado (`make generate`).
- [ ] El job comprueba el consentimiento antes de llamar al proveedor.
- [ ] El job llama al proveedor una vez, con el timeout de la configuración.
- [ ] El payload sólo lleva los cinco agregados, el tipo y la ventana.
- [ ] La actividad de otro usuario nunca se analiza (`not_found`).
- [ ] La ventana fuera de rango falla con un código, no se amplía.
- [ ] Un fallo reintentable devuelve el error; uno permanente, no.
- [ ] `error_code` y `error_message` nunca llevan payload, credencial ni identidad.

## Siguiente paso

T6: cablear la aplicación (arrancar River sin Strava, registrar el worker
de IA y el encolador, y mantener el producto arrancable con la IA apagada).

T7: la API pide el análisis (actividad, semanal, carga) y devuelve su
estado, reutilizando el scope, los helpers y el contrato del job.
