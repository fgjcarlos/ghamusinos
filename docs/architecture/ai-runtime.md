# Runtime de IA (fase 1.5 · issue #17)

Fundamento de ejecución y persistencia de la IA opcional: la **configuración
global** (que nunca impide el arranque) y la tabla **`ai_analysis`** (un
análisis por usuario y objetivo). Ni el job (T5) ni la API (T6) existen
todavía; esta unidad deja el contrato cerrado para que ambos lo reutilicen
sin decidir nada por su cuenta.

Con la IA apagada o mal configurada, el producto funciona igual (S7): la
configuración inválida produce un aviso y el estado `unavailable`, nunca un
error de arranque.

## Camino rápido

1. Despliegue: `AI_ENABLED=true` + `AI_PROVIDER=openai` +
   `AI_OPENAI_API_KEY=…` (sólo si se quiere IA).
2. La app lee `config.Config.AI`; `cfg.AI.Status()` dice `ready`,
   `disabled` o `unavailable`.
3. Una petición de análisis llama a `CreateAIAnalysis` (crea o reutiliza la
   fila) y el job la cierra con `CompleteAIAnalysis` o `FailAIAnalysis`.

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

## Comprobación

```bash
TZ=UTC go test -count=1 ./internal/config
TZ=UTC go test -race -count=1 ./internal/config
TZ=UTC go vet ./internal/config
golangci-lint run ./internal/config/...
make generate

# Con PostgreSQL real (migraciones aplicadas):
TEST_DATABASE_URL=postgres://… TZ=UTC go test -tags integration -race ./internal/db/sqlc
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

## Siguiente paso

T5: el job de River que toma la fila `pending`, comprueba el consentimiento
antes de enviar y la cierra con `CompleteAIAnalysis` o `FailAIAnalysis`.
