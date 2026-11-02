// Job de River que realiza UN análisis de IA sobre los agregados
// deterministas de un usuario.
//
// El job es la única puerta de salida hacia un proveedor de IA, así que
// aplica las garantías de la fase 1.5 (issue #17) en este orden:
//
//  1. Consentimiento: se lee `users.ai_enabled` ANTES de tocar el
//     proveedor. Sin consentimiento la fila se marca fallida con el código
//     seguro `consent_revoked` y el job termina sin error: la falta de
//     consentimiento no es un fallo transitorio y no se reintenta.
//  2. Configuración global: con la IA apagada o no disponible la fila se
//     marca fallida (`ai_disabled` / `ai_unavailable`) y el job termina sin
//     error. Con la IA apagada el producto sigue funcionando igual (S7).
//  3. Payload agregado: se construye sólo con métricas ya almacenadas
//     (distancia, duración, desnivel, FC media y carga). Nunca se leen
//     streams crudos ni campos de texto libre, y una métrica sin medir se
//     omite en lugar de enviarse como cero.
//  4. Ownership: la actividad se carga por (id, user_id). La actividad de
//     otro usuario no existe para el job (`not_found`) y nunca se analiza.
//  5. Proveedor: una sola llamada al proveedor configurado, con el contexto
//     acotado por `AI_TIMEOUT`. Nunca se cambia de proveedor.
//  6. Persistencia: `CompleteAIAnalysis` con el payload exacto enviado y la
//     salida validada, o `FailAIAnalysis` con un código y un mensaje
//     seguros (nunca el cuerpo del proveedor, el payload, la credencial ni
//     la identidad del usuario).
//  7. Reintentos: un fallo reintentable (`ai.Retryable`) se devuelve para
//     que River lo reintente con su backoff exponencial; un fallo
//     permanente termina sin error. La fila queda fallida entre intentos y
//     pasa a completada cuando un reintento lo consigue.
//
// El presupuesto de intentos lo fija `AIAnalysisMaxAttempts` (un intento
// inicial más `AI_MAX_RETRIES` reintentos, nunca por encima de
// `config.MaxAIMaxRetries`); el backoff lo aplica River con su política
// por defecto (exponencial).

package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"github.com/fgjcarlos/ghamusinos/internal/ai"
	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// AIAnalysisJobKind es el identificador del job en River.
const AIAnalysisJobKind = "ai_analysis"

// AIAnalysisWindowDays es la ventana del análisis semanal, en días.
const AIAnalysisWindowDays = 7

// Códigos de error seguros que se guardan en ai_analysis.error_code.
//
// Todos cumplen el CHECK del esquema (^[a-z0-9_]{1,64}$). Ninguno lleva
// información del proveedor, del payload ni del usuario.
const (
	// AIErrorCodeConsentRevoked: el usuario no aceptó el envío.
	AIErrorCodeConsentRevoked = "consent_revoked"
	// AIErrorCodeDisabled: la IA está apagada por el flag global.
	AIErrorCodeDisabled = "ai_disabled"
	// AIErrorCodeUnavailable: la configuración global no permite analizar.
	AIErrorCodeUnavailable = "ai_unavailable"
	// AIErrorCodeNotFound: el objetivo del análisis no existe (o no es del
	// usuario del job).
	AIErrorCodeNotFound = "not_found"
	// AIErrorCodeInvalidScope: la clave del análisis no coincide con el
	// objetivo solicitado.
	AIErrorCodeInvalidScope = "invalid_scope"
	// AIErrorCodeInvalidWindow: la ventana no es válida.
	AIErrorCodeInvalidWindow = "invalid_window"
	// AIErrorCodeWindowTooLarge: la ventana supera el límite permitted.
	AIErrorCodeWindowTooLarge = "window_too_large"
	// AIErrorCodeInvalidMetrics: los agregados no caben en el contrato del
	// payload (o no se pueden construir).
	AIErrorCodeInvalidMetrics = "invalid_metrics"
	// AIErrorCodeProviderMismatch: el analizador no es el proveedor
	// configurado.
	AIErrorCodeProviderMismatch = "provider_mismatch"
	// AIErrorCodeAnalysisFailed: el proveedor falló sin un motivo
	// clasificado.
	AIErrorCodeAnalysisFailed = "analysis_failed"
)

// Mensajes seguros que se guardan en ai_analysis.error_message.
//
// Son frases cortas de la aplicación: no contienen el cuerpo del
// proveedor, ni el payload, ni la credencial, ni la identidad del usuario.
const (
	aiMessageConsentRevoked   = "El usuario no ha aceptado el envío de sus datos al proveedor de IA."
	aiMessageDisabled         = "La IA está desactivada por el flag global."
	aiMessageUnavailable      = "La IA no está disponible: la configuración global no permite analizar."
	aiMessageNotFound         = "El objetivo del análisis no existe."
	aiMessageInvalidScope     = "La clave del análisis no coincide con el objetivo solicitado."
	aiMessageInvalidWindow    = "La ventana del análisis no es válida."
	aiMessageWindowTooLarge   = "La ventana del análisis supera el límite permitido."
	aiMessageInvalidMetrics   = "Los agregados del análisis no se pueden enviar al proveedor."
	aiMessageProviderMismatch = "El analizador no corresponde al proveedor configurado."
	aiMessageAnalysisFailed   = "El análisis con el proveedor de IA no se pudo completar."
)

// aiMaxMessageLength es el tope del CHECK de ai_analysis.error_message.
const aiMaxMessageLength = 500

// AIAnalysisArgs son los argumentos del job "ai_analysis".
//
// La API (T7) los construye con los mismos helpers de scope que usa el
// job (ai.ActivityScope, ai.WeeklyScope, ai.LoadScope), de modo que la
// fila de ai_analysis y el job apuntan al mismo objetivo.
type AIAnalysisArgs struct {
	// UserID es el UUID interno del usuario (obligatorio).
	UserID string `json:"user_id"`
	// AnalysisKind es el tipo de análisis: "activity", "weekly" o "load".
	AnalysisKind string `json:"analysis_kind"`
	// Scope es la clave determinista del objetivo. Si viene vacía, el job
	// la deriva de los argumentos; si no coincide con la derivada, el
	// análisis falla sin llegar al proveedor.
	Scope string `json:"scope"`
	// ActivityID es el UUID de la actividad (sólo kind="activity").
	ActivityID string `json:"activity_id,omitempty"`
	// WindowStart es la fecha (YYYY-MM-DD) del inicio de la ventana, en la
	// zona horaria del usuario (kind="weekly" y kind="load").
	WindowStart string `json:"window_start,omitempty"`
	// WindowDays es el número de días de la ventana (kind="load").
	WindowDays int `json:"window_days,omitempty"`
}

// Kind es el identificador del job en River.
func (AIAnalysisArgs) Kind() string { return AIAnalysisJobKind }

// AIAnalysisStore es el subconjunto de sqlc.Querier que necesita el job.
//
// *sqlc.Queries lo satisface. Las consultas están acotadas por user_id:
// el job nunca puede leer ni escribir datos de otro usuario.
type AIAnalysisStore interface {
	GetUserPreferencesByID(ctx context.Context, id pgtype.UUID) (sqlc.GetUserPreferencesByIDRow, error)
	GetActivityForUser(ctx context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error)
	ListActivitiesInRange(ctx context.Context, arg sqlc.ListActivitiesInRangeParams) ([]sqlc.ListActivitiesInRangeRow, error)
	ListTrainingLoadRange(ctx context.Context, arg sqlc.ListTrainingLoadRangeParams) ([]sqlc.TrainingLoadDaily, error)
	CompleteAIAnalysis(ctx context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error)
	FailAIAnalysis(ctx context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error)
}

// AIAnalysisWorker es el worker de River que realiza un análisis de IA.
type AIAnalysisWorker struct {
	river.WorkerDefaults[AIAnalysisArgs]

	store    AIAnalysisStore
	analyzer ai.Analyzer
	cfg      config.AIConfig
	logger   *slog.Logger
}

// NewAIAnalysisWorker construye el worker con sus dependencias.
//
// store es la persistencia (sqlc.Queries en producción). analyzer es el
// proveedor ya construido: puede ser nil cuando la IA está apagada, y en
// ese caso el job marca la fila como fallida sin llamar a nadie. cfg es la
// configuración global de la IA (flag, proveedor, modelo, credencial,
// timeout y reintentos). logger puede ser nil (slog.Default()).
//
// El worker nunca construye el analizador ni cambia de proveedor: usa
// exactamente el que recibe, y comprueba que sea el configurado.
func NewAIAnalysisWorker(
	store AIAnalysisStore,
	analyzer ai.Analyzer,
	cfg config.AIConfig,
	logger *slog.Logger,
) *AIAnalysisWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &AIAnalysisWorker{
		store:    store,
		analyzer: analyzer,
		cfg:      cfg,
		logger:   logger,
	}
}

// AIAnalysisMaxAttempts devuelve el número total de intentos del job: un
// intento inicial más los reintentos configurados (AI_MAX_RETRIES),
// nunca por encima de 1+config.MaxAIMaxRetries.
//
// El encolado usa este valor en river.InsertOpts{MaxAttempts: …}. El
// backoff entre intentos lo aplica River con su política por defecto, que
// es exponencial (ATTEMPT^4).
func AIAnalysisMaxAttempts(cfg config.AIConfig) int {
	retries := cfg.MaxRetries
	if retries < 0 {
		retries = 0
	}
	if retries > config.MaxAIMaxRetries {
		retries = config.MaxAIMaxRetries
	}
	return 1 + retries
}

// Work es el punto de entrada de River.
func (w *AIAnalysisWorker) Work(ctx context.Context, job *river.Job[AIAnalysisArgs]) error {
	return w.run(ctx, job.Args)
}

// aiTarget es el objetivo de un análisis, ya resuelto.
type aiTarget struct {
	kind         ai.Kind
	scope        string
	derivedScope string
	activityID   string
	activityUUID pgtype.UUID
	windowStart  time.Time
	windowDays   int
}

// run hace el trabajo completo. Está separado de Work para que los tests
// puedan ejercitarlo sin construir un job de River.
func (w *AIAnalysisWorker) run(ctx context.Context, args AIAnalysisArgs) error {
	userID, err := parseAIUserID(args.UserID)
	if err != nil {
		return fmt.Errorf("ai_analysis: user_id inválido: %w", err)
	}

	kind, err := parseAIKind(args.AnalysisKind)
	if err != nil {
		return fmt.Errorf("ai_analysis: tipo de análisis inválido: %w", err)
	}

	target, err := resolveAITarget(kind, args)
	if errors.Is(err, errAIScopeMismatch) {
		// El scope y los campos explícitos no concuerdan: la fila se
		// marca fallida sin tocar el proveedor.
		return w.failAnalysis(
			ctx, userID,
			aiTarget{kind: kind, scope: args.Scope},
			AIErrorCodeInvalidScope, aiMessageInvalidScope, nil,
		)
	}
	if err != nil {
		return fmt.Errorf("ai_analysis: objetivo inválido: %w", err)
	}

	// 1. Consentimiento, antes de cualquier contacto con el proveedor.
	prefs, err := w.store.GetUserPreferencesByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("ai_analysis: leer las preferencias del usuario: %w", err)
	}
	if !prefs.AiEnabled {
		return w.failAnalysis(ctx, userID, target, AIErrorCodeConsentRevoked, aiMessageConsentRevoked, nil)
	}

	// 2. Configuración global: con la IA apagada o no disponible no sale
	//    nada, y el job no es un error (S7).
	if !w.cfg.Enabled {
		return w.failAnalysis(ctx, userID, target, AIErrorCodeDisabled, aiMessageDisabled, nil)
	}
	if !w.cfg.Available {
		return w.failAnalysis(ctx, userID, target, AIErrorCodeUnavailable, aiMessageUnavailable, nil)
	}
	if w.analyzer == nil {
		return w.failAnalysis(ctx, userID, target, AIErrorCodeUnavailable, aiMessageUnavailable, nil)
	}
	provider, err := ai.ParseProvider(w.cfg.Provider)
	if err != nil {
		return w.failAnalysis(ctx, userID, target, AIErrorCodeUnavailable, aiMessageUnavailable, nil)
	}
	// El proveedor es el configurado y sólo ese: nunca se cambia.
	if w.analyzer.Provider() != provider {
		return w.failAnalysis(ctx, userID, target, AIErrorCodeProviderMismatch, aiMessageProviderMismatch, nil)
	}

	// 3. Ventana acotada: una ventana fuera de rango no se amplía en
	//    silencio, el análisis falla con un código seguro.
	if kind != ai.KindActivity {
		if target.windowDays <= 0 {
			return w.failAnalysis(ctx, userID, target, AIErrorCodeInvalidWindow, aiMessageInvalidWindow, nil)
		}
		if target.windowDays > ai.MaxWindowDays {
			return w.failAnalysis(ctx, userID, target, AIErrorCodeWindowTooLarge, aiMessageWindowTooLarge, nil)
		}
	}

	// 4. Payload agregado, construido sólo con métricas almacenadas.
	payload, payloadJSON, err := w.buildPayload(ctx, userID, target, prefs)
	switch {
	case errors.Is(err, errAINotFound):
		return w.failAnalysis(ctx, userID, target, AIErrorCodeNotFound, aiMessageNotFound, nil)
	case errors.Is(err, errAIInvalidMetrics):
		return w.failAnalysis(ctx, userID, target, AIErrorCodeInvalidMetrics, aiMessageInvalidMetrics, nil)
	case err != nil:
		return err
	}

	// 5. Una sola llamada al proveedor, con el contexto acotado por el
	//    timeout de la configuración.
	callCtx, cancel := context.WithTimeout(ctx, w.timeout())
	defer cancel()

	out, err := w.analyzer.Analyze(callCtx, payload)
	if err != nil {
		code := aiFailureCode(err)
		if storeErr := w.failAnalysis(ctx, userID, target, code, aiFailureMessage(code), payloadJSON); storeErr != nil {
			return storeErr
		}
		if ai.Retryable(err) {
			// El fallo se puede resolver solo: River reintenta con su
			// backoff y la fila queda fallida hasta que un reintento lo
			// consiga.
			return fmt.Errorf("ai_analysis: fallo reintentable del proveedor: %w", err)
		}
		w.logger.Warn("ai_analysis: fallo permanente del proveedor",
			slog.String("code", code),
			slog.String("kind", string(kind)),
		)
		return nil
	}

	// 6. La fila queda completada con el payload exacto enviado y la
	//    salida validada.
	outputJSON, err := json.Marshal(out)
	if err != nil {
		return w.failAnalysis(ctx, userID, target, AIErrorCodeAnalysisFailed, aiMessageAnalysisFailed, payloadJSON)
	}
	if _, err := w.store.CompleteAIAnalysis(ctx, sqlc.CompleteAIAnalysisParams{
		UserID:   userID,
		Kind:     string(kind),
		Scope:    target.scope,
		Provider: pgtype.Text{String: string(provider), Valid: true},
		Model:    pgtype.Text{String: w.cfg.Model, Valid: w.cfg.Model != ""},
		Payload:  payloadJSON,
		Output:   outputJSON,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// La fila ya no existe: no hay nada que reintentar.
			w.logger.Warn("ai_analysis: la fila del análisis ya no existe")
			return nil
		}
		return fmt.Errorf("ai_analysis: guardar el análisis completado: %w", err)
	}

	w.logger.Info("ai_analysis: análisis completado",
		slog.String("kind", string(kind)),
		slog.String("provider", string(provider)),
	)
	return nil
}

// buildPayload construye el payload agregado y su JSON exacto.
//
// Devuelve errAINotFound si el objetivo no existe (o no pertenece al
// usuario) y errAIInvalidMetrics si los agregados no se pueden construir.
func (w *AIAnalysisWorker) buildPayload(
	ctx context.Context,
	userID pgtype.UUID,
	target aiTarget,
	prefs sqlc.GetUserPreferencesByIDRow,
) (ai.Payload, []byte, error) {
	location := aiLocation(prefs.Timezone)

	var (
		aggregates aiAggregates
		windowDays int
	)

	switch target.kind {
	case ai.KindActivity:
		row, err := w.store.GetActivityForUser(ctx, sqlc.GetActivityForUserParams{
			ID:     target.activityUUID,
			UserID: userID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// La actividad no existe o es de otro usuario: no se
				// analiza nunca.
				return ai.Payload{}, nil, errAINotFound
			}
			return ai.Payload{}, nil, fmt.Errorf("ai_analysis: leer la actividad: %w", err)
		}
		aggregates = aiAggregatesFromActivity(row)
		aggregates.trainingLoad, aggregates.hasTrainingLoad = w.dayTrainingLoad(
			ctx, userID, row.StartedAt.Time.In(location), location,
		)
	case ai.KindWeekly, ai.KindLoad:
		start := time.Date(
			target.windowStart.Year(), target.windowStart.Month(), target.windowStart.Day(),
			0, 0, 0, 0, location,
		)
		end := start.AddDate(0, 0, target.windowDays)
		rows, err := w.store.ListActivitiesInRange(ctx, sqlc.ListActivitiesInRangeParams{
			UserID:      userID,
			StartedAt:   pgtype.Timestamptz{Time: start, Valid: true},
			StartedAt_2: pgtype.Timestamptz{Time: end.Add(-time.Nanosecond), Valid: true},
		})
		if err != nil {
			return ai.Payload{}, nil, fmt.Errorf("ai_analysis: listar las actividades de la ventana: %w", err)
		}
		aggregates = aiAggregatesFromActivities(rows)

		loads, err := w.store.ListTrainingLoadRange(ctx, sqlc.ListTrainingLoadRangeParams{
			UserID: userID,
			Day:    aiDate(start),
			Day_2:  aiDate(end.AddDate(0, 0, -1)),
		})
		if err != nil {
			return ai.Payload{}, nil, fmt.Errorf("ai_analysis: listar la carga de la ventana: %w", err)
		}
		aggregates.trainingLoad, aggregates.hasTrainingLoad = aiSumTrainingLoad(loads)
		windowDays = target.windowDays
	default:
		return ai.Payload{}, nil, errAIInvalidMetrics
	}

	payload, err := aggregates.Build(target.kind, windowDays)
	if err != nil {
		return ai.Payload{}, nil, errAIInvalidMetrics
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ai.Payload{}, nil, errAIInvalidMetrics
	}
	return payload, raw, nil
}

// dayTrainingLoad devuelve el tss almacenado del día de una actividad.
func (w *AIAnalysisWorker) dayTrainingLoad(
	ctx context.Context,
	userID pgtype.UUID,
	moment time.Time,
	location *time.Location,
) (float64, bool) {
	day := time.Date(moment.Year(), moment.Month(), moment.Day(), 0, 0, 0, 0, location)
	rows, err := w.store.ListTrainingLoadRange(ctx, sqlc.ListTrainingLoadRangeParams{
		UserID: userID,
		Day:    aiDate(day),
		Day_2:  aiDate(day),
	})
	if err != nil || len(rows) == 0 {
		return 0, false
	}
	total, _ := aiSumTrainingLoad(rows)
	return total, true
}

// timeout es el deadline de la llamada al proveedor.
func (w *AIAnalysisWorker) timeout() time.Duration {
	if w.cfg.Timeout > 0 {
		return w.cfg.Timeout
	}
	return config.DefaultAITimeout
}

// failAnalysis marca la fila como fallida con un código y un mensaje
// seguros, y devuelve el error de la persistencia (si lo hubo).
//
// Un error de base de datos se devuelve para que River lo reintente: la
// fila no se ha podido marcar y el estado real sigue siendo "pending".
func (w *AIAnalysisWorker) failAnalysis(
	ctx context.Context,
	userID pgtype.UUID,
	target aiTarget,
	code string,
	message string,
	payloadJSON []byte,
) error {
	safeCode := sanitizeAICode(code)
	safeMessage := truncateAIMessage(message)
	if len(payloadJSON) == 0 {
		payloadJSON = []byte("{}")
	}

	_, err := w.store.FailAIAnalysis(ctx, sqlc.FailAIAnalysisParams{
		UserID:       userID,
		Kind:         string(target.kind),
		Scope:        target.scope,
		Provider:     pgtype.Text{String: w.cfg.Provider, Valid: w.cfg.Provider != ""},
		Model:        pgtype.Text{String: w.cfg.Model, Valid: w.cfg.Model != ""},
		Payload:      payloadJSON,
		ErrorCode:    pgtype.Text{String: safeCode, Valid: true},
		ErrorMessage: pgtype.Text{String: safeMessage, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			w.logger.Warn("ai_analysis: la fila del análisis ya no existe")
			return nil
		}
		return fmt.Errorf("ai_analysis: guardar el fallo del análisis: %w", err)
	}

	w.logger.Warn("ai_analysis: análisis fallido",
		slog.String("code", safeCode),
		slog.String("kind", string(target.kind)),
	)
	return nil
}

// aiAggregates son los cinco agregados del payload.
//
// Un campo no presente (has* a false) se omite del payload: nunca se
// envía un cero inventado.
type aiAggregates struct {
	distance        float64
	hasDistance     bool
	duration        float64
	hasDuration     bool
	elevation       float64
	hasElevation    bool
	avgHeartRate    float64
	hasAvgHeartRate bool
	trainingLoad    float64
	hasTrainingLoad bool
}

// Build construye el payload con los agregados disponibles.
//
// Los valores se validan con el contrato de internal/ai: si un agregado no
// cabe (por ejemplo, una ventana de 90 días con más de 7 días de tiempo
// total en movimiento), Build devuelve un error y el job falla con un
// código seguro, en lugar de recortar los datos.
func (a aiAggregates) Build(kind ai.Kind, windowDays int) (ai.Payload, error) {
	builder := ai.NewBuilder(kind)
	if windowDays > 0 {
		builder = builder.WindowDays(windowDays)
	}
	if a.hasDistance {
		builder = builder.DistanceMeters(a.distance)
	}
	if a.hasDuration {
		builder = builder.DurationSeconds(a.duration)
	}
	if a.hasElevation {
		builder = builder.ElevationGainMeters(a.elevation)
	}
	if a.hasAvgHeartRate && a.avgHeartRate > 0 {
		builder = builder.AvgHeartRate(a.avgHeartRate)
	}
	if a.hasTrainingLoad {
		builder = builder.TrainingLoad(a.trainingLoad)
	}
	return builder.Build()
}

// aiAggregatesFromActivity extrae los agregados de una actividad.
func aiAggregatesFromActivity(row sqlc.GetActivityForUserRow) aiAggregates {
	aggregates := aiAggregates{}
	if value, ok := aiNumericValue(row.DistanceMeters); ok {
		aggregates.distance = value
		aggregates.hasDistance = true
	}
	if value, ok := aiNumericValue(row.ElevationGainM); ok {
		aggregates.elevation = value
		aggregates.hasElevation = true
	}
	if row.MovingSeconds > 0 {
		aggregates.duration = float64(row.MovingSeconds)
		aggregates.hasDuration = true
	}
	if row.AvgHr.Valid && row.AvgHr.Int16 > 0 {
		aggregates.avgHeartRate = float64(row.AvgHr.Int16)
		aggregates.hasAvgHeartRate = true
	}
	return aggregates
}

// aiAggregatesFromActivities agrega las actividades de una ventana.
//
//   - distancia, duración y desnivel: sumas de los valores almacenados.
//   - FC media: media ponderada por la duración de cada actividad. Sin
//     duración con la que ponderar, es la media simple de los valores
//     medidos.
//
// No se recalcula ninguna fórmula de carga: la carga sale de los tss ya
// almacenados.
func aiAggregatesFromActivities(rows []sqlc.ListActivitiesInRangeRow) aiAggregates {
	aggregates := aiAggregates{}
	var (
		hrSum    float64
		hrWeight float64
		hrCount  int
		hrTotal  float64
	)

	for _, row := range rows {
		if value, ok := aiNumericValue(row.DistanceMeters); ok {
			aggregates.distance += value
			aggregates.hasDistance = true
		}
		if value, ok := aiNumericValue(row.ElevationGainM); ok {
			aggregates.elevation += value
			aggregates.hasElevation = true
		}
		if row.MovingSeconds > 0 {
			aggregates.duration += float64(row.MovingSeconds)
			aggregates.hasDuration = true
		}
		if row.AvgHr.Valid && row.AvgHr.Int16 > 0 {
			weight := float64(row.MovingSeconds)
			if weight <= 0 {
				weight = float64(row.ElapsedSeconds)
			}
			hrSum += float64(row.AvgHr.Int16) * weight
			hrWeight += weight
			hrTotal += float64(row.AvgHr.Int16)
			hrCount++
		}
	}

	switch {
	case hrWeight > 0:
		aggregates.avgHeartRate = hrSum / hrWeight
		aggregates.hasAvgHeartRate = true
	case hrCount > 0:
		aggregates.avgHeartRate = hrTotal / float64(hrCount)
		aggregates.hasAvgHeartRate = true
	}
	return aggregates
}

// aiSumTrainingLoad suma los tss almacenados de una serie diaria.
//
// Si no hay filas no hay valor almacenado: la carga se omite del payload
// en lugar de enviar un cero inventado.
func aiSumTrainingLoad(rows []sqlc.TrainingLoadDaily) (float64, bool) {
	if len(rows) == 0 {
		return 0, false
	}
	total := 0.0
	for _, row := range rows {
		total += row.Tss
	}
	return total, true
}

// aiNumericValue convierte un NUMERIC nullable a float64.
func aiNumericValue(value pgtype.Numeric) (float64, bool) {
	if !value.Valid {
		return 0, false
	}
	converted, err := value.Float64Value()
	if err != nil || !converted.Valid {
		return 0, false
	}
	return converted.Float64, true
}

// aiFailureCode traduce el error del proveedor a un código seguro.
func aiFailureCode(err error) string {
	var providerErr *ai.Error
	if errors.As(err, &providerErr) && providerErr.Reason != "" {
		return string(providerErr.Reason)
	}
	return AIErrorCodeAnalysisFailed
}

// aiFailureMessage devuelve el mensaje seguro de un fallo del proveedor.
func aiFailureMessage(code string) string {
	switch code {
	case AIErrorCodeConsentRevoked:
		return aiMessageConsentRevoked
	case AIErrorCodeDisabled:
		return aiMessageDisabled
	case AIErrorCodeUnavailable:
		return aiMessageUnavailable
	case AIErrorCodeNotFound:
		return aiMessageNotFound
	case AIErrorCodeInvalidScope:
		return aiMessageInvalidScope
	case AIErrorCodeInvalidWindow:
		return aiMessageInvalidWindow
	case AIErrorCodeWindowTooLarge:
		return aiMessageWindowTooLarge
	case AIErrorCodeInvalidMetrics:
		return aiMessageInvalidMetrics
	case AIErrorCodeProviderMismatch:
		return aiMessageProviderMismatch
	default:
		return aiMessageAnalysisFailed
	}
}

// sanitizeAICode deja el código en la forma que exige el CHECK del
// esquema: minúsculas, dígitos y guion bajo, y como mucho 64 caracteres.
func sanitizeAICode(code string) string {
	lowered := strings.ToLower(strings.TrimSpace(code))
	var builder strings.Builder
	for _, char := range lowered {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9', char == '_':
			builder.WriteRune(char)
		default:
			builder.WriteRune('_')
		}
	}
	result := builder.String()
	if result == "" {
		return AIErrorCodeAnalysisFailed
	}
	if len(result) > 64 {
		result = result[:64]
	}
	return result
}

// truncateAIMessage acota el mensaje al límite del CHECK del esquema.
func truncateAIMessage(message string) string {
	runes := []rune(message)
	if len(runes) > aiMaxMessageLength {
		return string(runes[:aiMaxMessageLength])
	}
	return message
}

// parseAIUserID convierte el user_id de los argumentos en un UUID.
func parseAIUserID(value string) (pgtype.UUID, error) {
	var userID pgtype.UUID
	if err := userID.Scan(value); err != nil {
		return pgtype.UUID{}, fmt.Errorf("user_id %q no es un UUID: %w", value, err)
	}
	return userID, nil
}

// parseAIKind convierte el tipo de análisis de los argumentos.
func parseAIKind(value string) (ai.Kind, error) {
	kind := ai.Kind(strings.ToLower(strings.TrimSpace(value)))
	switch kind {
	case ai.KindActivity, ai.KindWeekly, ai.KindLoad:
		return kind, nil
	default:
		return "", fmt.Errorf("tipo de análisis %q desconocido", value)
	}
}

// resolveAITarget deriva el objetivo del análisis y su clave de scope.
//
// El scope es la fuente de verdad: identifica el objetivo (la actividad o
// la ventana). Los campos explícitos de los argumentos son opcionales, pero
// si vienen tienen que concordar con el scope; si no, se devuelve
// errAIScopeMismatch.
func resolveAITarget(kind ai.Kind, args AIAnalysisArgs) (aiTarget, error) {
	target := aiTarget{kind: kind, scope: args.Scope}

	if args.Scope != "" {
		parsed, err := ai.ParseScope(args.Scope)
		if err != nil {
			return aiTarget{}, err
		}
		if parsed.Kind != kind {
			return aiTarget{}, fmt.Errorf("el scope %q no corresponde al tipo de análisis %q", args.Scope, kind)
		}
		switch parsed.Kind {
		case ai.KindActivity:
			activityUUID, err := parseAIUserID(parsed.ActivityID)
			if err != nil {
				return aiTarget{}, fmt.Errorf("activity_id inválido: %w", err)
			}
			target.activityID = parsed.ActivityID
			target.activityUUID = activityUUID
		case ai.KindWeekly:
			target.windowStart = parsed.Date
			target.windowDays = AIAnalysisWindowDays
		case ai.KindLoad:
			target.windowStart = parsed.Date
			target.windowDays = parsed.Days
		}
		target.derivedScope = args.Scope

		// Los campos explícitos, si vienen, tienen que concordar.
		if args.ActivityID != "" && args.ActivityID != target.activityID {
			return aiTarget{}, errAIScopeMismatch
		}
		if args.WindowStart != "" && args.WindowStart != target.windowStart.Format(ai.ScopeDateLayout) {
			return aiTarget{}, errAIScopeMismatch
		}
		if kind == ai.KindLoad && args.WindowDays != 0 && args.WindowDays != target.windowDays {
			return aiTarget{}, errAIScopeMismatch
		}
		return target, nil
	}

	switch kind {
	case ai.KindActivity:
		if args.ActivityID == "" {
			return aiTarget{}, errors.New("falta activity_id")
		}
		activityUUID, err := parseAIUserID(args.ActivityID)
		if err != nil {
			return aiTarget{}, fmt.Errorf("activity_id inválido: %w", err)
		}
		target.activityID = args.ActivityID
		target.activityUUID = activityUUID
		target.derivedScope = ai.ActivityScope(args.ActivityID)
	case ai.KindWeekly, ai.KindLoad:
		if args.WindowStart == "" {
			return aiTarget{}, errors.New("falta window_start")
		}
		start, err := time.Parse(ai.ScopeDateLayout, args.WindowStart)
		if err != nil {
			return aiTarget{}, fmt.Errorf("window_start %q no es una fecha: %w", args.WindowStart, err)
		}
		target.windowStart = start
		target.windowDays = AIAnalysisWindowDays
		target.derivedScope = ai.WeeklyScope(start)
		if kind == ai.KindLoad {
			target.windowDays = args.WindowDays
			target.derivedScope = ai.LoadScope(start, target.windowDays)
		}
	default:
		return aiTarget{}, fmt.Errorf("tipo de análisis %q desconocido", kind)
	}

	target.scope = target.derivedScope
	return target, nil
}

// aiLocation devuelve la zona horaria del usuario, con UTC como respaldo.
func aiLocation(timezone string) *time.Location {
	if timezone == "" {
		return time.UTC
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.UTC
	}
	return location
}

// aiDate devuelve el día civil del instante en su propia zona horaria.
//
// No convierte a UTC: la ventana del usuario se cuenta en sus días
// locales, así que un desfase de zona no puede mover el primer día.
func aiDate(moment time.Time) pgtype.Date {
	return pgtype.Date{
		Time:             time.Date(moment.Year(), moment.Month(), moment.Day(), 0, 0, 0, 0, time.UTC),
		Valid:            true,
		InfinityModifier: pgtype.Finite,
	}
}

// Errores internos del job.
var (
	// errAINotFound: el objetivo no existe o no pertenece al usuario.
	errAINotFound = errors.New("ai_analysis: objetivo no encontrado")
	// errAIInvalidMetrics: los agregados no se pueden construir.
	errAIInvalidMetrics = errors.New("ai_analysis: agregados no válidos")
	// errAIScopeMismatch: el scope y los campos explícitos no concuerdan.
	errAIScopeMismatch = errors.New("ai_analysis: el scope no coincide con el objetivo")
)
