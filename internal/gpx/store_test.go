package gpx

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type mockGPXQuerier struct {
	sqlc.Querier
	created      sqlc.CreateGPXTrackParams
	createdClimb sqlc.CreateGPXClimbParams
	createdRisk  sqlc.CreateGPXRiskZoneParams
	getParams    sqlc.GetGPXTrackByIDParams
	hashParams   sqlc.GetGPXTrackByHashParams
	listParams   sqlc.ListGPXTracksByUserParams
	deleteParams sqlc.DeleteGPXTrackParams
	track        sqlc.GpxTrack
	tracks       []sqlc.GpxTrack
	err          error
	climbs       []sqlc.GpxClimb
	risks        []sqlc.GpxRiskZone
}

type mockGPXTransactionRunner struct {
	query gpxQuerier
	runs  int
}

func (m *mockGPXTransactionRunner) WithinTransaction(ctx context.Context, fn func(gpxQuerier) error) error {
	m.runs++
	return fn(m.query)
}

func (m *mockGPXQuerier) CreateGPXClimb(_ context.Context, params sqlc.CreateGPXClimbParams) (sqlc.GpxClimb, error) {
	m.createdClimb = params
	return sqlc.GpxClimb{}, m.err
}

func (m *mockGPXQuerier) CreateGPXRiskZone(_ context.Context, params sqlc.CreateGPXRiskZoneParams) (sqlc.GpxRiskZone, error) {
	m.createdRisk = params
	return sqlc.GpxRiskZone{}, m.err
}

func (m *mockGPXQuerier) GetGPXTrackByHash(_ context.Context, params sqlc.GetGPXTrackByHashParams) (sqlc.GpxTrack, error) {
	m.hashParams = params
	return m.track, m.err
}

func (m *mockGPXQuerier) ListGPXClimbsByTrack(_ context.Context, _ pgtype.UUID) ([]sqlc.GpxClimb, error) {
	return m.climbs, m.err
}

func (m *mockGPXQuerier) ListGPXRiskZonesByTrack(_ context.Context, _ pgtype.UUID) ([]sqlc.GpxRiskZone, error) {
	return m.risks, m.err
}

func (m *mockGPXQuerier) CreateGPXTrack(_ context.Context, params sqlc.CreateGPXTrackParams) (sqlc.GpxTrack, error) {
	m.created = params
	return m.track, m.err
}

func (m *mockGPXQuerier) GetGPXTrackByID(_ context.Context, params sqlc.GetGPXTrackByIDParams) (sqlc.GpxTrack, error) {
	m.getParams = params
	return m.track, m.err
}

func (m *mockGPXQuerier) ListGPXTracksByUser(_ context.Context, params sqlc.ListGPXTracksByUserParams) ([]sqlc.GpxTrack, error) {
	m.listParams = params
	return m.tracks, m.err
}

func (m *mockGPXQuerier) DeleteGPXTrack(_ context.Context, params sqlc.DeleteGPXTrackParams) error {
	m.deleteParams = params
	return m.err
}

func TestSQLCStoreCreateMapsTrackAndAnalysis(t *testing.T) {
	query := &mockGPXQuerier{}
	track := &Track{Name: "Trail", FileHash: "abc", FileSizeBytes: 2048, TrackType: "point-to-point", Points: []Point{{Lat: 40, Lon: -3}, {Lat: 41, Lon: -4}}}
	analysis := &Analysis{DistanceM: 1000, MovingTimeS: 300, DifficultyScore: 30, DifficultyLabel: DifficultyIntermediate}
	require.NoError(t, NewSQLCStore(query).Create(context.Background(), track, analysis))
	require.Equal(t, "Trail", query.created.Name)
	require.Equal(t, int32(300), query.created.MovingTimeS)
	require.JSONEq(t, `[[40,-3,null],[41,-4,null]]`, string(query.created.Coordinates))
}

func TestSQLCStoreGetByIDScopesByUser(t *testing.T) {
	userID := pgtype.UUID{Valid: true, Bytes: [16]byte{1}}
	trackID := pgtype.UUID{Valid: true, Bytes: [16]byte{2}}
	query := &mockGPXQuerier{track: databaseTrack(trackID, userID)}
	got, err := NewSQLCStore(query).GetByID(context.Background(), userID, trackID, 0)
	require.NoError(t, err)
	require.Equal(t, userID, query.getParams.UserID)
	require.Equal(t, trackID, got.Track.ID)
}

func TestSQLCStoreListPaginates(t *testing.T) {
	userID := pgtype.UUID{Valid: true, Bytes: [16]byte{1}}
	query := &mockGPXQuerier{tracks: []sqlc.GpxTrack{databaseTrack(pgtype.UUID{Valid: true}, userID), databaseTrack(pgtype.UUID{Valid: true}, userID), databaseTrack(pgtype.UUID{Valid: true}, userID)}}
	got, err := NewSQLCStore(query).List(context.Background(), userID, ListParams{Limit: 2, Offset: 4})
	require.NoError(t, err)
	require.Equal(t, int32(3), query.listParams.Limit)
	require.Len(t, got.Data, 2)
	require.True(t, got.HasNext)
}

func TestSQLCStoreDeleteScopesByUser(t *testing.T) {
	userID := pgtype.UUID{Valid: true, Bytes: [16]byte{1}}
	trackID := pgtype.UUID{Valid: true, Bytes: [16]byte{2}}
	query := &mockGPXQuerier{}
	require.NoError(t, NewSQLCStore(query).Delete(context.Background(), userID, trackID))
	require.Equal(t, userID, query.deleteParams.UserID)
	require.Equal(t, trackID, query.deleteParams.ID)
}

func TestSQLCStoreRejectsMissingDependencies(t *testing.T) {
	ctx := context.Background()
	require.Error(t, NewSQLCStore(nil).Create(ctx, &Track{}, &Analysis{}))
	require.Error(t, NewSQLCStore(&mockGPXQuerier{}).Create(ctx, nil, &Analysis{}))
	_, err := NewSQLCStore(nil).GetByID(ctx, pgtype.UUID{}, pgtype.UUID{}, 0)
	require.Error(t, err)
	_, err = NewSQLCStore(nil).List(ctx, pgtype.UUID{}, ListParams{})
	require.Error(t, err)
	require.Error(t, NewSQLCStore(nil).Delete(ctx, pgtype.UUID{}, pgtype.UUID{}))
}

func TestSQLCStoreWrapsQueryErrors(t *testing.T) {
	query := &mockGPXQuerier{err: errors.New("database unavailable")}
	ctx := context.Background()
	track := &Track{Name: "Trail", TrackType: "point-to-point", Points: []Point{{}, {}}}
	require.ErrorContains(t, NewSQLCStore(query).Create(ctx, track, &Analysis{}), "create track")
	_, err := NewSQLCStore(query).GetByID(ctx, pgtype.UUID{}, pgtype.UUID{}, 0)
	require.ErrorContains(t, err, "get track")
	_, err = NewSQLCStore(query).List(ctx, pgtype.UUID{}, ListParams{})
	require.ErrorContains(t, err, "list tracks")
	require.ErrorContains(t, NewSQLCStore(query).Delete(ctx, pgtype.UUID{}, pgtype.UUID{}), "delete track")
}

func TestSQLCStoreRejectsInvalidStoredCoordinates(t *testing.T) {
	query := &mockGPXQuerier{track: databaseTrack(pgtype.UUID{}, pgtype.UUID{})}
	query.track.Coordinates = []byte(`not-json`)
	_, err := NewSQLCStore(query).GetByID(context.Background(), pgtype.UUID{}, pgtype.UUID{}, 0)
	require.ErrorContains(t, err, "unmarshal coordinates")
}

func TestSQLCStoreCreateDetailPersistsClimbsAndRiskZones(t *testing.T) {
	trackID := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	query := &mockGPXQuerier{track: databaseTrack(trackID, pgtype.UUID{Valid: true})}
	track := &Track{Name: "Trail", FileHash: "hash", FileSizeBytes: 2048, TrackType: "circular", Points: []Point{{Lat: 40, Lon: -3}, {Lat: 41, Lon: -4}}}
	climbs := []Climb{{StartIdx: 1, EndIdx: 4, GainM: 120, DistanceM: 600, AvgSlopePct: 20, IsKingClimb: true}}
	risks := []RiskZone{{StartIdx: 2, EndIdx: 3, RiskType: "steep", Severity: "high"}}

	detail, err := NewSQLCStore(query).CreateDetail(context.Background(), track, &Analysis{DistanceM: 1000}, climbs, risks, &climbs[0])
	require.NoError(t, err)
	require.Equal(t, trackID, detail.Track.Track.ID)
	require.Equal(t, trackID, query.createdClimb.TrackID)
	require.Equal(t, int32(1), query.createdClimb.StartIdx)
	require.True(t, query.createdClimb.IsKingClimb)
	require.Equal(t, trackID, query.createdRisk.TrackID)
	require.Equal(t, "steep", query.createdRisk.RiskType)
	require.JSONEq(t, `{"start_idx":1,"end_idx":4,"gain_m":120,"distance_m":600,"avg_slope_pct":20,"is_king_climb":true}`, string(query.created.KingClimb))
}

func TestSQLCStoreCreateDetailUsesTransactionRunner(t *testing.T) {
	query := &mockGPXQuerier{track: databaseTrack(pgtype.UUID{Valid: true}, pgtype.UUID{Valid: true})}
	runner := &mockGPXTransactionRunner{query: query}
	store := newTransactionalSQLCStore(query, runner)

	_, err := store.CreateDetail(context.Background(), &Track{TrackType: "point-to-point", Points: []Point{{}, {}}}, &Analysis{}, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, runner.runs)
}

func TestSQLCStoreFindByHashScopesDuplicateToUser(t *testing.T) {
	userID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	query := &mockGPXQuerier{track: databaseTrack(pgtype.UUID{Bytes: [16]byte{2}, Valid: true}, userID)}

	stored, err := NewSQLCStore(query).FindByHash(context.Background(), userID, "same-hash")
	require.NoError(t, err)
	require.Equal(t, userID, query.hashParams.UserID)
	require.Equal(t, "same-hash", query.hashParams.FileHash)
	require.Equal(t, query.track.ID, stored.Track.ID)
}

func TestSQLCStoreGetDetailHydratesChildren(t *testing.T) {
	userID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	trackID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	gain100, err := numeric(100)
	if err != nil {
		t.Fatalf("test setup numeric(100): %v", err)
	}
	query := &mockGPXQuerier{
		track:  databaseTrack(trackID, userID),
		climbs: []sqlc.GpxClimb{{StartIdx: 1, EndIdx: 3, GainM: gain100, IsKingClimb: true}},
		risks:  []sqlc.GpxRiskZone{{StartIdx: 2, EndIdx: 4, RiskType: "technical", Severity: "medium"}},
	}

	detail, err := NewSQLCStore(query).GetDetail(context.Background(), userID, trackID, 0)
	require.NoError(t, err)
	require.Equal(t, trackID, detail.Track.Track.ID)
	require.Len(t, detail.Climbs, 1)
	require.True(t, detail.Climbs[0].IsKingClimb)
	require.InDelta(t, 100, detail.Climbs[0].GainM, 0.01)
	require.Len(t, detail.RiskZones, 1)
	require.Equal(t, "technical", detail.RiskZones[0].RiskType)
}

func databaseTrack(id, userID pgtype.UUID) sqlc.GpxTrack {
	return sqlc.GpxTrack{
		ID: id, UserID: userID, Name: "Trail", FileHash: "abc", FileSizeBytes: 2048,
		Coordinates: []byte(`[[40,-3,100],[41,-4,120]]`), MovingTimeS: 300,
		DifficultyScore: 30, DifficultyLabel: "intermediate", TrackType: "point-to-point",
	}
}

// ─── submuestreo (issue #170, M9) ───────────────────────────────────────

// TestSubsamplePoints_NoOpForSmallInput: si el track ya tiene menos
// puntos que la resolución pedida, devolvemos los puntos tal cual, sin
// perder información ni inventar duplicados.
func TestSubsamplePoints_NoOpForSmallInput(t *testing.T) {
	pts := make([]Point, 0, 5)
	for i := 0; i < 5; i++ {
		pts = append(pts, Point{Lat: float64(i), Lon: float64(i), Ele: ptr(float64(i * 10))})
	}
	got := subsamplePoints(pts, 100)
	require.Equal(t, pts, got)
}

// TestSubsamplePoints_KeepsGlobalExtrema: el pico y el valle de elevación
// del track completo aparecen en la respuesta submuestreada, incluso si
// caen en grupos distintos. AC del issue #170, M9.
func TestSubsamplePoints_KeepsGlobalExtrema(t *testing.T) {
	pts := make([]Point, 0, 100)
	for i := 0; i < 100; i++ {
		// Valle en el punto 25, pico en el 75, resto llano.
		ele := 100.0
		if i == 25 {
			ele = 50
		}
		if i == 75 {
			ele = 250
		}
		pts = append(pts, Point{Lat: float64(i), Lon: float64(i), Ele: ptr(ele)})
	}
	got := subsamplePoints(pts, 10) // 10 grupos → ~20 puntos
	var minEle, maxEle float64
	minEle, maxEle = 1e9, -1e9
	for _, p := range got {
		if p.Ele == nil {
			continue
		}
		if *p.Ele < minEle {
			minEle = *p.Ele
		}
		if *p.Ele > maxEle {
			maxEle = *p.Ele
		}
	}
	require.InDelta(t, 50, minEle, 0, "el valle global debe sobrevivir al submuestreo")
	require.InDelta(t, 250, maxEle, 0, "el pico global debe sobrevivir al submuestreo")
}

// TestSubsamplePoints_OutputSizeProportionalToResolution: con n=1000 y
// resolution=10 esperamos ~20 puntos (10 grupos × 2 extremos). El cap
// del issue es "como mucho ~2 000 puntos por defecto" (DefaultResolution).
func TestSubsamplePoints_OutputSizeProportionalToResolution(t *testing.T) {
	pts := make([]Point, 0, 1000)
	for i := 0; i < 1000; i++ {
		pts = append(pts, Point{Lat: float64(i), Lon: float64(i), Ele: ptr(float64(i % 100))})
	}
	got := subsamplePoints(pts, 10)
	// 10 grupos → hasta 20 puntos. El test exige ≤ 2*resolution; los
	// cuellos finos (buckets donde min==max) pueden bajar el conteo.
	require.LessOrEqual(t, len(got), 2*10)
	require.Greater(t, len(got), 10, "el submuestreo no debería colapsar a menos de resolution")
}

// TestSubsamplePoints_BucketWithoutElevation: si un grupo entero de
// puntos carece de <ele>, devolvemos al menos el primer punto del grupo
// para no perder el tramo horizontal.
func TestSubsamplePoints_BucketWithoutElevation(t *testing.T) {
	pts := []Point{
		{Lat: 0, Lon: 0, Ele: ptr(100)},
		{Lat: 1, Lon: 1, Ele: ptr(110)},
		{Lat: 2, Lon: 2, Ele: nil}, // grupo sin elevación: queda solo
		{Lat: 3, Lon: 3, Ele: nil},
		{Lat: 4, Lon: 4, Ele: ptr(150)},
	}
	got := subsamplePoints(pts, 2) // 2 grupos: [0,1] y [2,3,4]
	// Grupo 1 ([0,1]): min=100, max=110 → 2 puntos.
	// Grupo 2 ([2,3,4]): sin elevación → primer punto (idx 2).
	require.Len(t, got, 3)
	require.NotNil(t, got[2].Ele, "el grupo con elevación debe sobrevivir íntegro")
}

// TestSQLCStoreGetByIDAppliesResolution: end-to-end a través del store.
// Un track de 1000 puntos en la BD debe salir con ~20 puntos cuando
// llamamos GetByID(_, _, 10).
func TestSQLCStoreGetByIDAppliesResolution(t *testing.T) {
	userID := pgtype.UUID{Valid: true, Bytes: [16]byte{1}}
	trackID := pgtype.UUID{Valid: true, Bytes: [16]byte{2}}
	coords := []string{}
	for i := 0; i < 1000; i++ {
		coords = append(coords, fmt.Sprintf("[%d,%d,%d]", i, i, i%100))
	}
	row := databaseTrack(trackID, userID)
	row.Coordinates = []byte("[" + strings.Join(coords, ",") + "]")
	query := &mockGPXQuerier{track: row}

	track, err := NewSQLCStore(query).GetByID(context.Background(), userID, trackID, 10)
	require.NoError(t, err)
	require.LessOrEqual(t, len(track.Track.Points), 2*10, "el store debe aplicar la resolución pedida")
}

// helpers ────────────────────────────────────────────────────────────────

func ptr(v float64) *float64 { return &v }
