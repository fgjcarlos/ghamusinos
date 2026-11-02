# Contrato de IA (fase 1.5 · issue #17)

El paquete `internal/ai` define **qué puede salir del sistema hacia un
proveedor de IA** y **qué forma debe tener la respuesta**. Es el contrato
compartido por todos los proveedores (OpenAI, Claude, OpenRouter): el dominio
no conoce al proveedor, y el proveedor no conoce al dominio.

Con la IA desactivada o caída, el producto funciona igual (S7): este paquete
no tiene efectos secundarios, no lee configuración, no hace red y no toca la
base de datos.

## Camino rápido

1. El producto construye el payload con `ai.NewBuilder(kind)`.
2. El adaptador HTTP (fuera de esta unidad) envía ese payload y recibe JSON.
3. El adaptador valida la respuesta con `ai.ValidateOutput(raw, payload)`.
4. Si la validación falla, la respuesta se descarta como si el proveedor
   hubiera fallado.

## Qué sale del sistema

El payload **solo** lleva el tipo de análisis, la ventana acotada y cinco
agregados numéricos. No existe ningún campo para identidad, nombre, GPS,
streams crudos, marcas de tiempo ni texto libre: no se puede filtrar por
error porque no hay por dónde.

| Clave JSON | Significado | Límite | Se omite si |
|---|---|---|---|
| `kind` | Análisis pedido (`activity`, `weekly`, `load`) | — | nunca |
| `window_days` | Ventana analizada, en días | `0..90` | `0` (sin ventana) |
| `distance_meters` | Distancia recorrida | `0..1 000 000` m | sin medir |
| `duration_seconds` | Tiempo en movimiento | `0..604 800` s | sin medir |
| `elevation_gain_meters` | Desnivel acumulado | `0..50 000` m | sin medir |
| `avg_heart_rate` | Frecuencia cardiaca media | `>0..240` bpm | sin medir |
| `training_load` | Carga calculada | `0..1 000` | sin medir |

Reglas del payload:

- **Ausente ≠ cero.** Un agregado sin medir se omite del JSON; nunca se envía
  un `0` inventado. Un `0` explícito sí se envía (un día de descanso tiene
  carga real `0`).
- **`avg_heart_rate = 0` es un error**, no un cero: una media sobre una
  muestra no vacía nunca da 0, así que el valor no se midió.
- **Se rechaza** cualquier valor no finito (NaN, ±Inf), negativo o por encima
  del límite de la tabla. El error lo devuelve `Build()`.
- **La ventana está acotada** a 90 días, para que el análisis semanal y de
  carga no puedan convertirse en una extracción de histórico.

### Los tres análisis

| `kind` | Qué analiza | Cuándo se pide |
|---|---|---|
| `activity` | Una actividad | Bajo demanda (S4) |
| `weekly` | La ventana semanal | Bajo demanda (S4) |
| `load` | La ventana de carga | Bajo demanda (S4) |

## Qué vuelve del sistema

La respuesta es **interpretativa**: un resumen, observaciones y cautiones. No
hay ningún campo de métricas, así que el proveedor no puede devolver un
número que no recibió, ni añadir campos extra.

El esquema es uno solo y estricto, y es el mismo que se entrega a todos los
proveedores (`ai.OutputSchema()`):

```json
{
  "name": "training_analysis",
  "strict": true,
  "schema": {
    "type": "object",
    "additionalProperties": false,
    "required": ["summary", "observations", "cautions"],
    "properties": {
      "summary": { "type": "string", "minLength": 1, "maxLength": 1200 },
      "observations": { "type": "array", "maxItems": 10, "items": { "type": "string", "minLength": 1, "maxLength": 500 } },
      "cautions": { "type": "array", "maxItems": 10, "items": { "type": "string", "minLength": 1, "maxLength": 500 } }
    }
  }
}
```

El validador de Go aplica el mismo contrato que el esquema:

| Regla | Comportamiento |
|---|---|
| Campos obligatorios | `summary`, `observations` y `cautions` deben estar presentes |
| Campos desconocidos | Error (nada de `metrics` ni campos extra) |
| Tipos incorrectos | Error (`summary` texto, listas de texto) |
| `null` | Error, también en la raíz |
| JSON de más | Error: solo se acepta un documento |
| Longitudes | Resumen ≤ 1200 caracteres; cada ítem ≤ 500; ≤ 10 observaciones y ≤ 10 cautiones |
| Métrica inventada | Error: un número que no se envió al proveedor se rechaza |
| Diagnóstico | Error: el análisis interpreta el entrenamiento, no diagnostica |

El número permitido por respuesta es exactamente el que iba en el payload
(enviados + `window_days`). Si el modelo dice "12 %" o "48 horas", la
respuesta se descarta: ese dato no vino de nosotros y no debe aparecer como
si fuera medido. Las comparaciones y porcentajes los calcula la aplicación,
que sí tiene los datos.

El vocabulario de diagnóstico (cardiopatía, arritmia, diagnóstico,
enfermedad, lesión, medicación, etc.) está vetado en las tres listas.

## Contrato exportado (Go)

Todo lo que un adaptador o el dominio necesita ya existe en `internal/ai`:

```go
// Payload (payload.go)
type Kind string
const (
    KindActivity Kind = "activity"
    KindWeekly   Kind = "weekly"
    KindLoad     Kind = "load"
)

const (
    MaxDistanceMeters      = 1_000_000.0
    MaxDurationSeconds     = 604_800.0
    MaxElevationGainMeters = 50_000.0
    MaxAvgHeartRate        = 240.0
    MaxTrainingLoad        = 1_000.0
    MaxWindowDays          = 90
)

type Metrics struct {
    DistanceMeters      *float64
    DurationSeconds     *float64
    ElevationGainMeters *float64
    AvgHeartRate        *float64
    TrainingLoad        *float64
}

type Payload struct {
    Kind       Kind
    Metrics    Metrics
    WindowDays int
}

func (p Payload) MarshalJSON() ([]byte, error)

type Builder struct{ /* ... */ }
func NewBuilder(kind Kind) *Builder
func (b *Builder) WindowDays(days int) *Builder
func (b *Builder) DistanceMeters(value float64) *Builder
func (b *Builder) DurationSeconds(value float64) *Builder
func (b *Builder) ElevationGainMeters(value float64) *Builder
func (b *Builder) AvgHeartRate(value float64) *Builder
func (b *Builder) TrainingLoad(value float64) *Builder
func (b *Builder) Build() (Payload, error)

// Salida (output.go)
type Output struct {
    Summary      string   `json:"summary"`
    Observations []string `json:"observations"`
    Cautions     []string `json:"cautions"`
}

type Schema struct {
    Name   string          `json:"name"`
    Strict bool            `json:"strict"`
    Schema json.RawMessage `json:"schema"`
}

const OutputSchemaName = "training_analysis"
const (
    MaxSummaryLength     = 1200
    MaxObservationLength = 500
    MaxObservations      = 10
    MaxCautions          = 10
)

func OutputSchema() Schema
func ValidateOutput(raw []byte, sent Payload) error
func ParseOutput(raw []byte, sent Payload) (Output, error)
```

`ValidateOutput` y `ParseOutput` reciben el payload **enviado**: es lo que
delata una métrica inventada.

## Fuera de alcance de esta unidad

- Adaptadores HTTP por proveedor (OpenAI, Claude, OpenRouter): siguiente
  unidad, sobre este contrato.
- Configuración, feature flag, jobs, persistencia y API: unidades
  posteriores.
- Consentimiento del usuario: ver
  [Consentimiento de IA](ai-consent.md).

## Comprobación

```bash
TZ=UTC go test ./internal/ai
TZ=UTC go test -race ./internal/ai
TZ=UTC go vet ./internal/ai
```

## Checklist

- [ ] El payload solo contiene `kind`, `window_days` y los cinco agregados.
- [ ] Un agregado sin medir está ausente, nunca en `0`.
- [ ] Los valores imposibles (negativos, no finitos, fuera de rango) se
      rechazan en `Build()`.
- [ ] La respuesta se valida con el mismo esquema y el mismo validador para
      todos los proveedores.
- [ ] Una respuesta con métrica inventada o diagnóstico se descarta.

## Referencia

- [ADR 0002 — Proveedor de IA](../decisions/0002-ai-provider.md)
