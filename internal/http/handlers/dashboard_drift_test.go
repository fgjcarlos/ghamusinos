package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// driftMockQuerier implementa el subconjunto de sqlc.Querier que necesita
// el handler de cardiac drift (DA-007). Embebe activitiesMockQuerier para
// heredar el resto de stubs zero-value.
type driftMockQuerier struct {
	*activitiesMockQuerier

	rows []sqlc.ListHeartRateStreamsInRangeRow
	err  error
	// lastParams guarda los parámetros de la última llamada para poder
	// comprobar el rango que el handler pasa a la query.
	lastParams sqlc.ListHeartRateStreamsInRangeParams
}

func (m *driftMockQuerier) ListHeartRateStreamsInRange(_ context.Context, arg sqlc.ListHeartRateStreamsInRangeParams) ([]sqlc.ListHeartRateStreamsInRangeRow, error) {
	m.lastParams = arg
	if m.err != nil {
		return nil, m.err
	}
	return m.rows, nil
}

func newDriftMock(t *testing.T) *driftMockQuerier {
	t.Helper()
	return &driftMockQuerier{activitiesMockQuerier: newActivitiesMockQuerier(t)}
}

// driftUUID construye un pgtype.UUID válido a partir de un byte repetido.
func driftUUID(b byte) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte{b, b, b, b, b, b, b, b, b, b, b, b, b, b, b, b}, Valid: true}
}

// driftRequestAt ejecuta el handler de drift con la URL dada.
func driftRequestAt(t *testing.T, q *driftMockQuerier, url string, withAuth bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if withAuth {
		req = withDevAuthRequest(req)
	}
	rec := httptest.NewRecorder()
	GetCardiacDrift(q).ServeHTTP(rec, req)
	return rec
}

// TestCardiacDrift_401WithoutAuth: sin usuario autenticado → 401 con
// ProblemDetail (RFC 9457).
func TestCardiacDrift_401WithoutAuth(t *testing.T) {
	rec := driftRequestAt(t, newDriftMock(t), "/api/v1/dashboard/cardiac-drift?from=2024-01-01&to=2024-01-31", false)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var problem map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&problem))
	require.Equal(t, float64(http.StatusUnauthorized), problem["status"])
	require.Contains(t, problem, "title")
}

// TestCardiacDrift_400BadRange: rango inválido o invertido → 400.
func TestCardiacDrift_400BadRange(t *testing.T) {
	cases := []string{
		"/api/v1/dashboard/cardiac-drift?to=hola",
		"/api/v1/dashboard/cardiac-drift?from=bad",
		"/api/v1/dashboard/cardiac-drift?from=2024-02-01&to=2024-01-01",
	}
	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			rec := driftRequestAt(t, newDriftMock(t), url, true)
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

// TestCardiacDrift_500InternalError: fallo de la query → 500.
func TestCardiacDrift_500InternalError(t *testing.T) {
	mock := newDriftMock(t)
	mock.err = errors.New("boom")

	rec := driftRequestAt(t, mock, "/api/v1/dashboard/cardiac-drift?from=2024-01-01&to=2024-01-31", true)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestCardiacDrift_OKWithActivities: 200 con el shape de DA-007 y el drift
// calculado sobre un array plano de números (modelo del repo).
func TestCardiacDrift_OKWithActivities(t *testing.T) {
	mock := newDriftMock(t)
	mock.rows = []sqlc.ListHeartRateStreamsInRangeRow{
		{
			ActivityID: driftUUID(0x11),
			Data:       []byte(`[100,100,102,102,148,148,150,150]`),
		},
	}

	rec := driftRequestAt(t, mock, "/api/v1/dashboard/cardiac-drift?from=2024-01-01&to=2024-01-31", true)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, "2024-01-01", body["from"])
	require.Equal(t, "2024-01-31", body["to"])
	require.Equal(t, true, body["available"])

	activities, ok := body["activities"].([]interface{})
	require.True(t, ok)
	require.Len(t, activities, 1)

	entry := activities[0].(map[string]interface{})
	require.Equal(t, "11111111-1111-1111-1111-111111111111", entry["activity_id"])
	require.Equal(t, 50.0, entry["drift_pct"])
	require.Equal(t, float64(8), entry["sample_count"])

	// El rango se pasa a la query como [from 00:00 UTC, to+1d 00:00 UTC).
	require.True(t, mock.lastParams.StartedAt.Valid)
	require.Equal(t, 2024, mock.lastParams.StartedAt.Time.Year())
	require.Equal(t, 1, mock.lastParams.StartedAt.Time.Day())
	require.True(t, mock.lastParams.StartedAt_2.Valid)
	require.Equal(t, 2024, mock.lastParams.StartedAt_2.Time.Year())
	require.Equal(t, time.February, mock.lastParams.StartedAt_2.Time.Month())
	require.Equal(t, 1, mock.lastParams.StartedAt_2.Time.Day(), "el límite superior es exclusivo (00:00 UTC del día siguiente)")
}

// TestCardiacDrift_OKAvailableFalse: sin streams HR en el rango → 200 con
// available=false y activities vacío (no null).
func TestCardiacDrift_OKAvailableFalse(t *testing.T) {
	mock := newDriftMock(t)

	rec := driftRequestAt(t, mock, "/api/v1/dashboard/cardiac-drift?from=2024-01-01&to=2024-01-31", true)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, false, body["available"])
	require.Equal(t, "2024-01-01", body["from"])
	require.Equal(t, "2024-01-31", body["to"])

	activities, ok := body["activities"].([]interface{})
	require.True(t, ok, "activities debe ser un array vacío, no null")
	require.Len(t, activities, 0)
}

// TestCardiacDrift_PairsParsing: el JSONB con pares [time, value] (forma
// real de la API de Strava) se interpreta igual que el array plano.
func TestCardiacDrift_PairsParsing(t *testing.T) {
	mock := newDriftMock(t)
	mock.rows = []sqlc.ListHeartRateStreamsInRangeRow{
		{
			ActivityID: driftUUID(0x22),
			Data:       []byte(`[[0,100],[1,100],[2,102],[3,102],[4,148],[5,148],[6,150],[7,150]]`),
		},
	}

	rec := driftRequestAt(t, mock, "/api/v1/dashboard/cardiac-drift?from=2024-01-01&to=2024-01-31", true)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, true, body["available"])

	activities := body["activities"].([]interface{})
	require.Len(t, activities, 1)

	entry := activities[0].(map[string]interface{})
	require.Equal(t, "22222222-2222-2222-2222-222222222222", entry["activity_id"])
	require.Equal(t, 50.0, entry["drift_pct"])
	require.Equal(t, float64(8), entry["sample_count"])
}

// TestCardiacDrift_OmitsActivitiesWithFewerThan2Samples: una actividad con
// menos de 2 muestras usables se omite del array (y no marca available).
func TestCardiacDrift_OmitsActivitiesWithFewerThan2Samples(t *testing.T) {
	mock := newDriftMock(t)
	mock.rows = []sqlc.ListHeartRateStreamsInRangeRow{
		{ActivityID: driftUUID(0x33), Data: []byte(`[140]`)},           // 1 muestra
		{ActivityID: driftUUID(0x44), Data: []byte(`[0,-3]`)},          // sin muestras válidas
		{ActivityID: driftUUID(0x55), Data: []byte(`[]`)},              // stream vacío
		{ActivityID: driftUUID(0x66), Data: []byte(`not json at all`)}, // stream corrupto
	}

	rec := driftRequestAt(t, mock, "/api/v1/dashboard/cardiac-drift?from=2024-01-01&to=2024-01-31", true)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, false, body["available"])

	activities := body["activities"].([]interface{})
	require.Len(t, activities, 0)
}

// TestCardiacDrift_IgnoresZeroAndNegativeSamples: los valores 0 o negativos
// se ignoran al parsear; solo las muestras usables cuentan.
func TestCardiacDrift_IgnoresZeroAndNegativeSamples(t *testing.T) {
	mock := newDriftMock(t)
	mock.rows = []sqlc.ListHeartRateStreamsInRangeRow{
		{ActivityID: driftUUID(0x77), Data: []byte(`[100,0,102,102,148,148,150,150,-20]`)},
	}

	rec := driftRequestAt(t, mock, "/api/v1/dashboard/cardiac-drift?from=2024-01-01&to=2024-01-31", true)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, true, body["available"])

	activities := body["activities"].([]interface{})
	require.Len(t, activities, 1)

	entry := activities[0].(map[string]interface{})
	require.Equal(t, float64(7), entry["sample_count"], "0 y los negativos no son muestras")
	// 7 muestras útiles: 100,102,102,148,148,150,150 → primer cuartil 101,
	// tercer cuartil 150 → (150-101)/101*100 = 48.51.
	require.Equal(t, 48.51, entry["drift_pct"])
}

// TestParseHeartRateSamples: contrato del parser puro (array plano y pares).
func TestParseHeartRateSamples(t *testing.T) {
	cases := []struct {
		name string
		data string
		want []int
	}{
		{name: "array plano", data: `[100,110,120]`, want: []int{100, 110, 120}},
		{name: "pares time/value", data: `[[0,100],[1,110]]`, want: []int{100, 110}},
		{name: "mezcla plano y pares", data: `[120,[1,130],140]`, want: []int{120, 130, 140}},
		{name: "ignora cero y negativos", data: `[0,-1,120]`, want: []int{120}},
		{name: "ignora no numéricos", data: `["x",120]`, want: []int{120}},
		{name: "ignora pares incompletos", data: `[[0],120]`, want: []int{120}},
		{name: "array vacío", data: `[]`, want: []int{}},
		{name: "json inválido", data: `nope`, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseHeartRateSamples([]byte(tc.data))
			if tc.want == nil {
				require.Nil(t, got)
				return
			}
			require.Equal(t, tc.want, got)
		})
	}
}
