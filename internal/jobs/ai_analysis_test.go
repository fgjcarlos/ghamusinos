package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/ai"
	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// ─────────────────────────────────────────────────────────────────────────
// Fixtures
// ─────────────────────────────────────────────────────────────────────────

const (
	testAIUserID      = "11111111-1111-4111-8111-111111111111"
	testAIActivityID  = "33333333-3333-4333-8333-333333333333"
	testAIAPIKey      = "sk-test-secret-credential"
	testAIModel       = "test-model"
	testAIWindowStart = "2026-03-02"
)

// safeErrorCode es el patrón que impone el CHECK de ai_analysis.error_code.
var safeErrorCode = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// fakeAIStore es el doble de la persistencia que usa el job. No toca la
// base de datos: cada test decide qué filas "existen".
type fakeAIStore struct {
	prefs       sqlc.GetUserPreferencesByIDRow
	prefsErr    error
	activity    sqlc.GetActivityForUserRow
	activityErr error

	activities    []sqlc.ListActivitiesInRangeRow
	activitiesErr error
	loads         []sqlc.TrainingLoadDaily
	loadsErr      error

	completeErr error
	failErr     error

	completed []sqlc.CompleteAIAnalysisParams
	failed    []sqlc.FailAIAnalysisParams

	activityLookups int
	rangeLookups    int
	loadLookups     int
	rangeParams     sqlc.ListActivitiesInRangeParams
	loadParams      sqlc.ListTrainingLoadRangeParams
}

func (s *fakeAIStore) GetUserPreferencesByID(_ context.Context, _ pgtype.UUID) (sqlc.GetUserPreferencesByIDRow, error) {
	if s.prefsErr != nil {
		return sqlc.GetUserPreferencesByIDRow{}, s.prefsErr
	}
	return s.prefs, nil
}

func (s *fakeAIStore) GetActivityForUser(_ context.Context, _ sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error) {
	s.activityLookups++
	if s.activityErr != nil {
		return sqlc.GetActivityForUserRow{}, s.activityErr
	}
	return s.activity, nil
}

func (s *fakeAIStore) ListActivitiesInRange(_ context.Context, arg sqlc.ListActivitiesInRangeParams) ([]sqlc.ListActivitiesInRangeRow, error) {
	s.rangeLookups++
	s.rangeParams = arg
	if s.activitiesErr != nil {
		return nil, s.activitiesErr
	}
	return s.activities, nil
}

func (s *fakeAIStore) ListTrainingLoadRange(_ context.Context, arg sqlc.ListTrainingLoadRangeParams) ([]sqlc.TrainingLoadDaily, error) {
	s.loadLookups++
	s.loadParams = arg
	if s.loadsErr != nil {
		return nil, s.loadsErr
	}
	return s.loads, nil
}

func (s *fakeAIStore) CompleteAIAnalysis(_ context.Context, arg sqlc.CompleteAIAnalysisParams) (sqlc.AiAnalysis, error) {
	if s.completeErr != nil {
		return sqlc.AiAnalysis{}, s.completeErr
	}
	s.completed = append(s.completed, arg)
	return sqlc.AiAnalysis{}, nil
}

func (s *fakeAIStore) FailAIAnalysis(_ context.Context, arg sqlc.FailAIAnalysisParams) (sqlc.AiAnalysis, error) {
	if s.failErr != nil {
		return sqlc.AiAnalysis{}, s.failErr
	}
	s.failed = append(s.failed, arg)
	return sqlc.AiAnalysis{}, nil
}

// fakeAIAnalyzer es el doble del proveedor: cuenta las llamadas y recuerda
// el payload exacto que recibió.
type fakeAIAnalyzer struct {
	provider ai.Provider
	out      ai.Output
	err      error

	calls       int
	payload     ai.Payload
	payloadJSON []byte
	deadline    time.Time
	hasDeadline bool
}

func (f *fakeAIAnalyzer) Provider() ai.Provider { return f.provider }

func (f *fakeAIAnalyzer) Analyze(ctx context.Context, payload ai.Payload) (ai.Output, error) {
	f.calls++
	f.payload = payload
	raw, _ := json.Marshal(payload)
	f.payloadJSON = raw
	if deadline, ok := ctx.Deadline(); ok {
		f.hasDeadline = true
		f.deadline = deadline
	}
	return f.out, f.err
}

// aiConfigReady es la configuración global de IA lista para usar.
func aiConfigReady() config.AIConfig {
	return config.AIConfig{
		Enabled:    true,
		Available:  true,
		Provider:   string(ai.ProviderOpenAI),
		Model:      testAIModel,
		APIKey:     testAIAPIKey,
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}
}

// consentedPreferences son las preferencias de un usuario que aceptó.
func consentedPreferences() sqlc.GetUserPreferencesByIDRow {
	return sqlc.GetUserPreferencesByIDRow{
		Timezone:  "UTC",
		AiEnabled: true,
	}
}

// numericFloat convierte un texto numérico en pgtype.Numeric.
func numericFloat(t *testing.T, value string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(value); err != nil {
		t.Fatalf("numeric %q: %v", value, err)
	}
	return n
}

// activityRow es una actividad almacenada con los cinco determinantes.
func activityRow(t *testing.T) sqlc.GetActivityForUserRow {
	t.Helper()
	return sqlc.GetActivityForUserRow{
		ID:             pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		StartedAt:      pgtype.Timestamptz{Time: time.Date(2026, 3, 2, 7, 0, 0, 0, time.UTC), Valid: true},
		ElapsedSeconds: 3700,
		MovingSeconds:  3600,
		DistanceMeters: numericFloat(t, "10000.00"),
		ElevationGainM: numericFloat(t, "150.00"),
		AvgHr:          pgtype.Int2{Int16: 145, Valid: true},
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Regla 1: el consentimiento se comprueba antes de hablar con el proveedor
// ─────────────────────────────────────────────────────────────────────────

func TestAIAnalysisWorker_ConsentRevokedBlocksProviderCall(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: sqlc.GetUserPreferencesByIDRow{Timezone: "UTC", AiEnabled: false}}
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
		ActivityID:   testAIActivityID,
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil (sin consentimiento no es un fallo transitorio)", err)
	}
	if analyzer.calls != 0 {
		t.Errorf("el proveedor recibió %d llamadas, quiere 0 sin consentimiento", analyzer.calls)
	}
	if len(store.failed) != 1 {
		t.Fatalf("filas fallidas = %d, quiere 1", len(store.failed))
	}
	if got := store.failed[0].ErrorCode.String; got != "consent_revoked" {
		t.Errorf("error_code = %q, quiere %q", got, "consent_revoked")
	}
	if len(store.completed) != 0 {
		t.Errorf("filas completadas = %d, quiere 0", len(store.completed))
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Regla 2: la configuración global apagada o no disponible bloquea el envío
// ─────────────────────────────────────────────────────────────────────────

func TestAIAnalysisWorker_GlobalAIDisabledBlocksProviderCall(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI}
	cfg := aiConfigReady()
	cfg.Enabled = false
	cfg.Available = false
	worker := NewAIAnalysisWorker(store, analyzer, cfg, nil)

	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
		ActivityID:   testAIActivityID,
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil", err)
	}
	if analyzer.calls != 0 {
		t.Errorf("el proveedor recibió %d llamadas, quiere 0 con la IA apagada", analyzer.calls)
	}
	if len(store.failed) != 1 {
		t.Fatalf("filas fallidas = %d, quiere 1", len(store.failed))
	}
	if got := store.failed[0].ErrorCode.String; got != "ai_disabled" {
		t.Errorf("error_code = %q, quiere %q", got, "ai_disabled")
	}
}

func TestAIAnalysisWorker_GlobalAIUnavailableBlocksProviderCall(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI}
	cfg := aiConfigReady()
	cfg.Available = false
	worker := NewAIAnalysisWorker(store, analyzer, cfg, nil)

	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
		ActivityID:   testAIActivityID,
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil", err)
	}
	if analyzer.calls != 0 {
		t.Errorf("el proveedor recibió %d llamadas, quiere 0 con la IA no disponible", analyzer.calls)
	}
	if len(store.failed) != 1 {
		t.Fatalf("filas fallidas = %d, quiere 1", len(store.failed))
	}
	if got := store.failed[0].ErrorCode.String; got != "ai_unavailable" {
		t.Errorf("error_code = %q, quiere %q", got, "ai_unavailable")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Regla 3 y 5: payload agregado y fila completada
// ─────────────────────────────────────────────────────────────────────────

func TestAIAnalysisWorker_CompletesRowWithExactPayloadAndOutput(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	store.activity = activityRow(t)
	store.loads = []sqlc.TrainingLoadDaily{{Tss: 42}}
	out := ai.Output{Summary: "Sesión sólida", Observations: []string{"Ritmo constante"}, Cautions: []string{}}
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI, out: out}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
		ActivityID:   testAIActivityID,
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil", err)
	}
	if analyzer.calls != 1 {
		t.Fatalf("llamadas al proveedor = %d, quiere exactamente 1", analyzer.calls)
	}
	if len(store.completed) != 1 {
		t.Fatalf("filas completadas = %d, quiere 1", len(store.completed))
	}
	if len(store.failed) != 0 {
		t.Fatalf("filas fallidas = %d, quiere 0", len(store.failed))
	}

	completed := store.completed[0]
	if completed.Provider.String != string(ai.ProviderOpenAI) {
		t.Errorf("provider = %q, quiere %q", completed.Provider.String, ai.ProviderOpenAI)
	}
	if completed.Model.String != testAIModel {
		t.Errorf("model = %q, quiere %q", completed.Model.String, testAIModel)
	}

	// El payload guardado es exactamente el que salió hacia el proveedor.
	wantPayload, err := json.Marshal(analyzer.payload)
	if err != nil {
		t.Fatalf("marshal del payload: %v", err)
	}
	if string(completed.Payload) != string(wantPayload) {
		t.Errorf("payload guardado = %s, quiere %s", completed.Payload, wantPayload)
	}
	if string(completed.Payload) != string(analyzer.payloadJSON) {
		t.Errorf("payload guardado = %s, quiere el enviado %s", completed.Payload, analyzer.payloadJSON)
	}

	wantOutput, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal de la salida: %v", err)
	}
	if string(completed.Output) != string(wantOutput) {
		t.Errorf("output guardado = %s, quiere %s", completed.Output, wantOutput)
	}
}

func TestAIAnalysisWorker_PayloadCarriesOnlyAggregatesKindAndWindow(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	store.activities = []sqlc.ListActivitiesInRangeRow{
		{
			StartedAt:      pgtype.Timestamptz{Time: time.Date(2026, 3, 2, 7, 0, 0, 0, time.UTC), Valid: true},
			MovingSeconds:  3600,
			DistanceMeters: numericFloat(t, "10000.00"),
			ElevationGainM: numericFloat(t, "150.00"),
			AvgHr:          pgtype.Int2{Int16: 145, Valid: true},
		},
		{
			StartedAt:      pgtype.Timestamptz{Time: time.Date(2026, 3, 3, 7, 0, 0, 0, time.UTC), Valid: true},
			MovingSeconds:  1800,
			DistanceMeters: numericFloat(t, "5000.00"),
			ElevationGainM: numericFloat(t, "50.00"),
			AvgHr:          pgtype.Int2{Int16: 155, Valid: true},
		},
	}
	store.loads = []sqlc.TrainingLoadDaily{{Tss: 30}, {Tss: 12}}
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI, out: ai.Output{Summary: "Semana", Observations: []string{}, Cautions: []string{}}}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindWeekly),
		Scope:        ai.WeeklyScope(time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)),
		WindowStart:  testAIWindowStart,
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil", err)
	}
	if analyzer.calls != 1 {
		t.Fatalf("llamadas al proveedor = %d, quiere exactamente 1", analyzer.calls)
	}

	var decoded map[string]any
	if err := json.Unmarshal(analyzer.payloadJSON, &decoded); err != nil {
		t.Fatalf("el payload no es JSON: %v", err)
	}
	wantTop := map[string]bool{"kind": true, "metrics": true, "window_days": true}
	for key := range decoded {
		if !wantTop[key] {
			t.Errorf("el payload tiene la clave %q, que no está en la lista blanca", key)
		}
	}
	if decoded["kind"] != string(ai.KindWeekly) {
		t.Errorf("kind = %v, quiere %q", decoded["kind"], ai.KindWeekly)
	}
	if decoded["window_days"] != float64(7) {
		t.Errorf("window_days = %v, quiere 7", decoded["window_days"])
	}

	metrics, ok := decoded["metrics"].(map[string]any)
	if !ok {
		t.Fatalf("metrics = %v, quiere un objeto", decoded["metrics"])
	}
	wantMetrics := map[string]bool{
		"distance_meters":       true,
		"duration_seconds":      true,
		"elevation_gain_meters": true,
		"avg_heart_rate":        true,
		"training_load":         true,
	}
	for key := range metrics {
		if !wantMetrics[key] {
			t.Errorf("metrics tiene la clave %q, que no está en la lista blanca", key)
		}
	}
	for key := range wantMetrics {
		if _, ok := metrics[key]; !ok {
			t.Errorf("metrics no tiene la clave %q", key)
		}
	}
	if metrics["distance_meters"] != float64(15000) {
		t.Errorf("distance_meters = %v, quiere 15000 (suma de las actividades)", metrics["distance_meters"])
	}
	if metrics["duration_seconds"] != float64(5400) {
		t.Errorf("duration_seconds = %v, quiere 5400 (suma de los tiempos en movimiento)", metrics["duration_seconds"])
	}
	if metrics["elevation_gain_meters"] != float64(200) {
		t.Errorf("elevation_gain_meters = %v, quiere 200", metrics["elevation_gain_meters"])
	}
	// Media ponderada por duración: (145*3600 + 155*1800) / 5400.
	if metrics["avg_heart_rate"] != float64(148.33333333333334) {
		t.Errorf("avg_heart_rate = %v, quiere la media ponderada por duración", metrics["avg_heart_rate"])
	}
	if metrics["training_load"] != float64(42) {
		t.Errorf("training_load = %v, quiere 42 (suma de los tss almacenados)", metrics["training_load"])
	}
}

func TestAIAnalysisWorker_MissingMetricIsOmitted(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	row := activityRow(t)
	row.ElevationGainM = pgtype.Numeric{} // sin desnivel medido
	row.AvgHr = pgtype.Int2{}             // sin FC media
	store.activity = row
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI, out: ai.Output{Summary: "Sesión", Observations: []string{}, Cautions: []string{}}}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
		ActivityID:   testAIActivityID,
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil", err)
	}
	if analyzer.calls != 1 {
		t.Fatalf("llamadas al proveedor = %d, quiere exactamente 1", analyzer.calls)
	}

	var decoded struct {
		Metrics map[string]any `json:"metrics"`
	}
	if err := json.Unmarshal(analyzer.payloadJSON, &decoded); err != nil {
		t.Fatalf("el payload no es JSON: %v", err)
	}
	if _, ok := decoded.Metrics["elevation_gain_meters"]; ok {
		t.Errorf("elevation_gain_meters presente sin medición: %s", analyzer.payloadJSON)
	}
	if _, ok := decoded.Metrics["avg_heart_rate"]; ok {
		t.Errorf("avg_heart_rate presente sin medición: %s", analyzer.payloadJSON)
	}
	if _, ok := decoded.Metrics["distance_meters"]; !ok {
		t.Errorf("distance_meters ausente sin motivo: %s", analyzer.payloadJSON)
	}
	if strings.Contains(string(analyzer.payloadJSON), ":0") {
		t.Errorf("el payload envía un cero inventado: %s", analyzer.payloadJSON)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Regla 3: la ventana está acotada
// ─────────────────────────────────────────────────────────────────────────

func TestAIAnalysisWorker_WindowBoundIsEnforced(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindLoad),
		Scope:        ai.LoadScope(time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC), ai.MaxWindowDays+1),
		WindowStart:  testAIWindowStart,
		WindowDays:   ai.MaxWindowDays + 1,
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil (una ventana fuera de rango no se reintenta)", err)
	}
	if analyzer.calls != 0 {
		t.Errorf("el proveedor recibió %d llamadas, quiere 0 con una ventana fuera de rango", analyzer.calls)
	}
	if len(store.failed) != 1 {
		t.Fatalf("filas fallidas = %d, quiere 1", len(store.failed))
	}
	if got := store.failed[0].ErrorCode.String; got != "window_too_large" {
		t.Errorf("error_code = %q, quiere %q", got, "window_too_large")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Regla 3: una actividad de otro usuario no se analiza nunca
// ─────────────────────────────────────────────────────────────────────────

func TestAIAnalysisWorker_ActivityOfAnotherUserIsNeverAnalysed(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	store.activityErr = pgx.ErrNoRows
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
		ActivityID:   testAIActivityID,
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil", err)
	}
	if analyzer.calls != 0 {
		t.Errorf("el proveedor recibió %d llamadas, quiere 0 para una actividad ajena", analyzer.calls)
	}
	if len(store.failed) != 1 {
		t.Fatalf("filas fallidas = %d, quiere 1", len(store.failed))
	}
	if got := store.failed[0].ErrorCode.String; got != "not_found" {
		t.Errorf("error_code = %q, quiere %q", got, "not_found")
	}
	if len(store.completed) != 0 {
		t.Errorf("filas completadas = %d, quiere 0", len(store.completed))
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Regla 6: clasificación de fallos y reintentos
// ─────────────────────────────────────────────────────────────────────────

func TestAIAnalysisWorker_RetryableErrorReturnsErrorAndFailsRow(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	store.activity = activityRow(t)
	providerErr := &ai.Error{
		Provider: ai.ProviderOpenAI,
		Kind:     ai.ErrorKindRetryable,
		Reason:   ai.ReasonRateLimited,
		Status:   429,
	}
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI, err: providerErr}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
		ActivityID:   testAIActivityID,
	})
	if err == nil {
		t.Fatal("run() = nil, quiere un error para que River reintente")
	}
	if !ai.Retryable(err) {
		t.Errorf("el error %v no se puede reintentar", err)
	}
	if len(store.failed) != 1 {
		t.Fatalf("filas fallidas = %d, quiere 1 (la fila queda fallida entre intentos)", len(store.failed))
	}
	if got := store.failed[0].ErrorCode.String; got != string(ai.ReasonRateLimited) {
		t.Errorf("error_code = %q, quiere %q", got, ai.ReasonRateLimited)
	}
	if len(store.completed) != 0 {
		t.Errorf("filas completadas = %d, quiere 0", len(store.completed))
	}
}

func TestAIAnalysisWorker_PermanentErrorReturnsNoErrorAndFailsRow(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	store.activity = activityRow(t)
	providerErr := &ai.Error{
		Provider: ai.ProviderOpenAI,
		Kind:     ai.ErrorKindPermanent,
		Reason:   ai.ReasonUnauthorized,
		Status:   401,
	}
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI, err: providerErr}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
		ActivityID:   testAIActivityID,
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil (un fallo permanente no se reintenta)", err)
	}
	if len(store.failed) != 1 {
		t.Fatalf("filas fallidas = %d, quiere 1", len(store.failed))
	}
	if got := store.failed[0].ErrorCode.String; got != string(ai.ReasonUnauthorized) {
		t.Errorf("error_code = %q, quiere %q", got, ai.ReasonUnauthorized)
	}
}

func TestAIAnalysisWorker_RowStaysFailedUntilARetrySucceeds(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	store.activity = activityRow(t)
	analyzer := &fakeAIAnalyzer{
		provider: ai.ProviderOpenAI,
		err: &ai.Error{
			Provider: ai.ProviderOpenAI,
			Kind:     ai.ErrorKindRetryable,
			Reason:   ai.ReasonProviderError,
			Status:   503,
		},
	}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)
	args := AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
		ActivityID:   testAIActivityID,
	}

	if err := worker.run(context.Background(), args); err == nil {
		t.Fatal("run() = nil en el primer intento, quiere un error reintentable")
	}
	if len(store.failed) != 1 || len(store.completed) != 0 {
		t.Fatalf("tras el fallo: fallidas=%d completadas=%d, quiere 1 y 0", len(store.failed), len(store.completed))
	}

	// El reintento sí funciona: la misma fila pasa a completada.
	analyzer.err = nil
	analyzer.out = ai.Output{Summary: "Ya salió", Observations: []string{}, Cautions: []string{}}
	if err := worker.run(context.Background(), args); err != nil {
		t.Fatalf("run() = %v en el reintento, quiere nil", err)
	}
	if len(store.completed) != 1 {
		t.Fatalf("filas completadas = %d, quiere 1 tras el reintento", len(store.completed))
	}
	if len(store.failed) != 1 {
		t.Fatalf("filas fallidas = %d, quiere 1 (el histórico del fallo no se borra)", len(store.failed))
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Regla 6: el error guardado es seguro
// ─────────────────────────────────────────────────────────────────────────

func TestAIAnalysisWorker_FailureCodeAndMessageAreSafe(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	store.activity = activityRow(t)
	providerErr := &ai.Error{
		Provider: ai.ProviderOpenAI,
		Kind:     ai.ErrorKindPermanent,
		Reason:   ai.ReasonSchemaViolation,
		Status:   400,
		Err:      errors.New("cuerpo del proveedor con datos del usuario"),
	}
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI, err: providerErr}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
		ActivityID:   testAIActivityID,
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil", err)
	}
	if len(store.failed) != 1 {
		t.Fatalf("filas fallidas = %d, quiere 1", len(store.failed))
	}
	failure := store.failed[0]

	if !safeErrorCode.MatchString(failure.ErrorCode.String) {
		t.Errorf("error_code %q no cumple ^[a-z0-9_]{1,64}$", failure.ErrorCode.String)
	}
	message := failure.ErrorMessage.String
	if len([]rune(message)) > 500 {
		t.Errorf("error_message tiene %d caracteres, quiere como mucho 500", len([]rune(message)))
	}
	forbidden := []string{
		testAIAPIKey,
		testAIUserID,
		testAIActivityID,
		"cuerpo del proveedor",
		"distance_meters",
		"10000",
	}
	for _, needle := range forbidden {
		if strings.Contains(message, needle) {
			t.Errorf("error_message %q contiene %q", message, needle)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Regla 4: la llamada al proveedor está acotada por el timeout
// ─────────────────────────────────────────────────────────────────────────

func TestAIAnalysisWorker_AnalyzerCallIsBoundedByTimeout(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	store.activity = activityRow(t)
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI, out: ai.Output{Summary: "Ok", Observations: []string{}, Cautions: []string{}}}
	cfg := aiConfigReady()
	cfg.Timeout = 5 * time.Second
	worker := NewAIAnalysisWorker(store, analyzer, cfg, nil)

	if err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
		ActivityID:   testAIActivityID,
	}); err != nil {
		t.Fatalf("run() = %v, quiere nil", err)
	}
	if !analyzer.hasDeadline {
		t.Error("la llamada al proveedor no tenía deadline: el timeout de la configuración no se aplicó")
	}
	if analyzer.deadline.After(time.Now().Add(cfg.Timeout + time.Second)) {
		t.Errorf("el deadline %s está más allá del timeout configurado (%s)", analyzer.deadline, cfg.Timeout)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// El scope identifica el objetivo
// ─────────────────────────────────────────────────────────────────────────

func TestAIAnalysisWorker_IdentifiesActivityFromScope(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	store.activity = activityRow(t)
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI, out: ai.Output{Summary: "Ok", Observations: []string{}, Cautions: []string{}}}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	// Sin activity_id: el scope es lo que identifica la actividad.
	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil", err)
	}
	if store.activityLookups != 1 {
		t.Errorf("búsquedas de la actividad = %d, quiere 1", store.activityLookups)
	}
	if analyzer.calls != 1 {
		t.Errorf("llamadas al proveedor = %d, quiere 1", analyzer.calls)
	}
	if len(store.completed) != 1 {
		t.Fatalf("filas completadas = %d, quiere 1", len(store.completed))
	}
	if store.completed[0].Scope != ai.ActivityScope(testAIActivityID) {
		t.Errorf("scope = %q, quiere %q", store.completed[0].Scope, ai.ActivityScope(testAIActivityID))
	}
}

func TestAIAnalysisWorker_ScopeMismatchFailsWithoutProviderCall(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	store.activity = activityRow(t)
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	otherActivity := "44444444-4444-4444-8444-444444444444"
	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindActivity),
		Scope:        ai.ActivityScope(testAIActivityID),
		ActivityID:   otherActivity,
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil", err)
	}
	if analyzer.calls != 0 {
		t.Errorf("el proveedor recibió %d llamadas, quiere 0 con un scope inconsistente", analyzer.calls)
	}
	if len(store.failed) != 1 {
		t.Fatalf("filas fallidas = %d, quiere 1", len(store.failed))
	}
	if got := store.failed[0].ErrorCode.String; got != "invalid_scope" {
		t.Errorf("error_code = %q, quiere %q", got, "invalid_scope")
	}
}

func TestAIAnalysisWorker_LoadWindowComesFromScope(t *testing.T) {
	t.Parallel()

	store := &fakeAIStore{prefs: consentedPreferences()}
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI, out: ai.Output{Summary: "Ok", Observations: []string{}, Cautions: []string{}}}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	start := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	err := worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindLoad),
		Scope:        ai.LoadScope(start, 28),
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil", err)
	}
	if store.rangeLookups != 1 {
		t.Fatalf("búsquedas de actividades = %d, quiere 1", store.rangeLookups)
	}
	wantFrom := start
	wantTo := start.AddDate(0, 0, 28).Add(-time.Nanosecond)
	if !store.rangeParams.StartedAt.Time.Equal(wantFrom) {
		t.Errorf("inicio de la ventana = %s, quiere %s", store.rangeParams.StartedAt.Time, wantFrom)
	}
	if !store.rangeParams.StartedAt_2.Time.Equal(wantTo) {
		t.Errorf("fin de la ventana = %s, quiere %s", store.rangeParams.StartedAt_2.Time, wantTo)
	}
	wantDay := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	if !store.loadParams.Day.Time.Equal(wantDay) {
		t.Errorf("primer día de la carga = %s, quiere %s", store.loadParams.Day.Time, wantDay)
	}
	wantLastDay := time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)
	if !store.loadParams.Day_2.Time.Equal(wantLastDay) {
		t.Errorf("último día de la carga = %s, quiere %s", store.loadParams.Day_2.Time, wantLastDay)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// La ventana se cuenta en los días locales del usuario
// ─────────────────────────────────────────────────────────────────────────

func TestAIAnalysisWorker_WeeklyWindowUsesUserTimezone(t *testing.T) {
	t.Parallel()

	madrid, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Skipf("la base de datos de zonas horarias no está disponible: %v", err)
	}

	store := &fakeAIStore{prefs: sqlc.GetUserPreferencesByIDRow{Timezone: "Europe/Madrid", AiEnabled: true}}
	analyzer := &fakeAIAnalyzer{provider: ai.ProviderOpenAI, out: ai.Output{Summary: "Ok", Observations: []string{}, Cautions: []string{}}}
	worker := NewAIAnalysisWorker(store, analyzer, aiConfigReady(), nil)

	err = worker.run(context.Background(), AIAnalysisArgs{
		UserID:       testAIUserID,
		AnalysisKind: string(ai.KindWeekly),
		Scope:        ai.WeeklyScope(time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)),
		WindowStart:  testAIWindowStart,
	})
	if err != nil {
		t.Fatalf("run() = %v, quiere nil", err)
	}
	wantFrom := time.Date(2026, 3, 2, 0, 0, 0, 0, madrid)
	if !store.rangeParams.StartedAt.Time.Equal(wantFrom) {
		t.Errorf("inicio de la ventana = %s, quiere %s (medianoche local)", store.rangeParams.StartedAt.Time, wantFrom)
	}
	wantDay := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	if !store.loadParams.Day.Time.Equal(wantDay) {
		t.Errorf("primer día de la carga = %s, quiere %s", store.loadParams.Day.Time, wantDay)
	}
	if !store.loadParams.Day_2.Time.Equal(wantDay.AddDate(0, 0, 6)) {
		t.Errorf("último día de la carga = %s, quiere %s", store.loadParams.Day_2.Time, wantDay.AddDate(0, 0, 6))
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Regla 7: presupuesto de reintentos
// ─────────────────────────────────────────────────────────────────────────

func TestAIAnalysisMaxAttempts(t *testing.T) {
	t.Parallel()

	cfg := aiConfigReady()
	cfg.MaxRetries = 3
	if got := AIAnalysisMaxAttempts(cfg); got != 4 {
		t.Errorf("AIAnalysisMaxAttempts(3 reintentos) = %d, quiere 4 (un intento inicial más los reintentos)", got)
	}

	cfg.MaxRetries = 0
	if got := AIAnalysisMaxAttempts(cfg); got != 1 {
		t.Errorf("AIAnalysisMaxAttempts(0 reintentos) = %d, quiere 1", got)
	}

	cfg.MaxRetries = config.MaxAIMaxRetries + 5
	if got := AIAnalysisMaxAttempts(cfg); got != 1+config.MaxAIMaxRetries {
		t.Errorf("AIAnalysisMaxAttempts(%d reintentos) = %d, quiere %d", cfg.MaxRetries, got, 1+config.MaxAIMaxRetries)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// El identificador del job es estable
// ─────────────────────────────────────────────────────────────────────────

func TestAIAnalysisArgsKind(t *testing.T) {
	t.Parallel()

	if got := (AIAnalysisArgs{}).Kind(); got != "ai_analysis" {
		t.Errorf("Kind() = %q, quiere %q", got, "ai_analysis")
	}
}
