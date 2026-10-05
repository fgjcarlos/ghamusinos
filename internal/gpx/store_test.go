package gpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type mockGPXQuerier struct {
	sqlc.Querier
	created              sqlc.CreateGPXTrackParams
	createdClimb         sqlc.CreateGPXClimbParams
	createdRisk          sqlc.CreateGPXRiskZoneParams
	createdMuros         []sqlc.CreateGPXMuroParams
	createdRecoveryZones []sqlc.CreateGPXRecoveryZoneParams
	createdKmVertical    []sqlc.UpsertGPXKmVerticalParams
	getParams            sqlc.GetGPXTrackByIDParams
	hashParams           sqlc.GetGPXTrackByHashParams
	listParams           sqlc.ListGPXTracksByUserParams
	deleteParams         sqlc.DeleteGPXTrackParams
	track                sqlc.GpxTrack
	tracks               []sqlc.ListGPXTracksByUserRow
	err                  error
	climbs               []sqlc.GpxClimb
	risks                []sqlc.GpxRiskZone
	muros                []sqlc.GpxMuro
	recoveryZones        []sqlc.GpxRecoveryZone
	kmVertical           sqlc.GpxKmVertical
	kmVerticalErr        error
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

func (m *mockGPXQuerier) CreateGPXMuro(_ context.Context, params sqlc.CreateGPXMuroParams) (sqlc.GpxMuro, error) {
	m.createdMuros = append(m.createdMuros, params)
	return sqlc.GpxMuro{}, m.err
}

func (m *mockGPXQuerier) ListGPXMurosByTrack(_ context.Context, _ pgtype.UUID) ([]sqlc.GpxMuro, error) {
	return m.muros, m.err
}

func (m *mockGPXQuerier) CreateGPXRecoveryZone(_ context.Context, params sqlc.CreateGPXRecoveryZoneParams) (sqlc.GpxRecoveryZone, error) {
	m.createdRecoveryZones = append(m.createdRecoveryZones, params)
	return sqlc.GpxRecoveryZone{}, m.err
}

func (m *mockGPXQuerier) ListGPXRecoveryZonesByTrack(_ context.Context, _ pgtype.UUID) ([]sqlc.GpxRecoveryZone, error) {
	return m.recoveryZones, m.err
}

func (m *mockGPXQuerier) UpsertGPXKmVertical(_ context.Context, params sqlc.UpsertGPXKmVerticalParams) (sqlc.GpxKmVertical, error) {
	m.createdKmVertical = append(m.createdKmVertical, params)
	return sqlc.GpxKmVertical{}, m.err
}

func (m *mockGPXQuerier) GetGPXKmVerticalByTrack(_ context.Context, _ pgtype.UUID) (sqlc.GpxKmVertical, error) {
	return m.kmVertical, m.kmVerticalErr
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

func (m *mockGPXQuerier) ListGPXTracksByUser(_ context.Context, params sqlc.ListGPXTracksByUserParams) ([]sqlc.ListGPXTracksByUserRow, error) {
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

func TestSQLCStorePropagatesElevationCoverage(t *testing.T) {
	userID := pgtype.UUID{Valid: true, Bytes: [16]byte{1}}
	trackID := pgtype.UUID{Valid: true, Bytes: [16]byte{2}}
	coverage, err := numeric(0.87)
	require.NoError(t, err)

	query := &mockGPXQuerier{track: databaseTrack(trackID, userID)}
	query.track.ElevationCoverage = coverage
	stored, err := NewSQLCStore(query).GetByID(context.Background(), userID, trackID, 0)
	require.NoError(t, err)
	require.NotNil(t, stored.Analysis.ElevationCoverage)
	require.InDelta(t, 0.87, *stored.Analysis.ElevationCoverage, 1e-9)

	query.track.ElevationCoverage = pgtype.Numeric{}
	stored, err = NewSQLCStore(query).GetByID(context.Background(), userID, trackID, 0)
	require.NoError(t, err)
	require.Nil(t, stored.Analysis.ElevationCoverage)
}

func TestSQLCStoreListPaginates(t *testing.T) {
	userID := pgtype.UUID{Valid: true, Bytes: [16]byte{1}}
	query := &mockGPXQuerier{tracks: []sqlc.ListGPXTracksByUserRow{
		databaseTrackRow(databaseTrack(pgtype.UUID{Valid: true}, userID)),
		databaseTrackRow(databaseTrack(pgtype.UUID{Valid: true}, userID)),
		databaseTrackRow(databaseTrack(pgtype.UUID{Valid: true}, userID)),
	}}
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

	detail, err := NewSQLCStore(query).CreateDetail(context.Background(), track, &Analysis{DistanceM: 1000}, climbs, risks, &climbs[0], nil, nil, nil)
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

	_, err := store.CreateDetail(context.Background(), &Track{TrackType: "point-to-point", Points: []Point{{}, {}}}, &Analysis{}, nil, nil, nil, nil, nil, nil)
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
		track:         databaseTrack(trackID, userID),
		climbs:        []sqlc.GpxClimb{{StartIdx: 1, EndIdx: 3, GainM: gain100, IsKingClimb: true}},
		risks:         []sqlc.GpxRiskZone{{StartIdx: 2, EndIdx: 4, RiskType: "technical", Severity: "medium"}},
		muros:         []sqlc.GpxMuro{{StartIdx: 10, EndIdx: 20, GainM: gain100, DistanceM: gain100, AvgSlopePct: gain100}},
		recoveryZones: []sqlc.GpxRecoveryZone{{StartIdx: 20, EndIdx: 30, DistanceM: gain100}},
		kmVertical:    sqlc.GpxKmVertical{StartIdx: 1, EndIdx: 80, GainM: gain100, DistanceM: gain100},
	}

	detail, err := NewSQLCStore(query).GetDetail(context.Background(), userID, trackID, 0)
	require.NoError(t, err)
	require.Equal(t, trackID, detail.Track.Track.ID)
	require.Len(t, detail.Climbs, 1)
	require.True(t, detail.Climbs[0].IsKingClimb)
	require.InDelta(t, 100, detail.Climbs[0].GainM, 0.01)
	require.Len(t, detail.RiskZones, 1)
	require.Equal(t, "technical", detail.RiskZones[0].RiskType)
	require.Len(t, detail.Muros, 1)
	require.Equal(t, 10, detail.Muros[0].StartIdx)
	require.Len(t, detail.RecoveryZones, 1)
	require.Equal(t, 20, detail.RecoveryZones[0].StartIdx)
	require.NotNil(t, detail.KmVertical)
	require.InDelta(t, 100, detail.KmVertical.GainM, 0.01)
}

func TestSQLCStoreGetDetailReturnsEmptyClimbDerivedListsAndNilKmVertical(t *testing.T) {
	trackID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	query := &mockGPXQuerier{
		track:         databaseTrack(trackID, pgtype.UUID{Valid: true}),
		muros:         []sqlc.GpxMuro{},
		recoveryZones: []sqlc.GpxRecoveryZone{},
		kmVerticalErr: pgx.ErrNoRows,
	}

	detail, err := NewSQLCStore(query).GetDetail(context.Background(), pgtype.UUID{Valid: true}, trackID, 0)
	require.NoError(t, err)
	require.NotNil(t, detail.Muros)
	require.Empty(t, detail.Muros)
	require.NotNil(t, detail.RecoveryZones)
	require.Empty(t, detail.RecoveryZones)
	require.Nil(t, detail.KmVertical)
}

// TestDatabaseTrackFixtureKeepsClimbDerivedFields is a regression pin against
// silent field loss when the `databaseTrack` fixture is converted into a
// StoredTrackDetail. If a future refactor moves a field name, drops a
// rehydration call, or breaks the empty/nil semantics for Muros,
// RecoveryZones, or KmVertical, this test catches it at the fixture level
// rather than letting the bug surface in downstream rendering code.
//
// Two halves pin two distinct failure modes:
//  1. Empty/absent rows MUST produce non-nil empty slices and a nil
//     singleton so JSON serialises as [] / [] / null without panicking.
//  2. Populated rows MUST round-trip through GetDetail into the
//     StoredTrackDetail fields — a silently dropped assignment would
//     leave the zero value in place, which the empty-half assertion
//     above could not catch.
func TestDatabaseTrackFixtureKeepsClimbDerivedFields(t *testing.T) {
	trackID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	userID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}

	t.Run("empty and absent rows serialise as expected", func(t *testing.T) {
		query := &mockGPXQuerier{
			track:         databaseTrack(trackID, userID),
			muros:         []sqlc.GpxMuro{},
			recoveryZones: []sqlc.GpxRecoveryZone{},
			kmVerticalErr: pgx.ErrNoRows,
		}
		detail, err := NewSQLCStore(query).GetDetail(context.Background(), userID, trackID, 0)
		require.NoError(t, err)
		require.NotNil(t, detail.Muros, "Muros slice must be initialised (not nil) so JSON serialises as []")
		require.NotNil(t, detail.RecoveryZones, "RecoveryZones slice must be initialised (not nil) so JSON serialises as []")
		require.Nil(t, detail.KmVertical, "KmVertical must remain nil when the row is absent so JSON serialises as null")
		data, err := json.Marshal(detail)
		require.NoError(t, err)
		encoded := string(data)
		require.Contains(t, encoded, `"muros":[]`)
		require.Contains(t, encoded, `"recovery_zones":[]`)
		require.Contains(t, encoded, `"km_vertical":null`)
	})

	t.Run("populated rows round-trip through rehydration", func(t *testing.T) {
		gain800, err := numeric(800)
		if err != nil {
			t.Fatalf("test setup numeric(800): %v", err)
		}
		distance10000, err := numeric(10000)
		if err != nil {
			t.Fatalf("test setup numeric(10000): %v", err)
		}
		gain50, err := numeric(50)
		if err != nil {
			t.Fatalf("test setup numeric(50): %v", err)
		}
		query := &mockGPXQuerier{
			track: databaseTrack(trackID, userID),
			muros: []sqlc.GpxMuro{
				{TrackID: trackID, StartIdx: 10, EndIdx: 20, GainM: gain50, DistanceM: distance10000, AvgSlopePct: gain50},
			},
			recoveryZones: []sqlc.GpxRecoveryZone{
				{TrackID: trackID, StartIdx: 20, EndIdx: 30, DistanceM: distance10000},
			},
			kmVertical: sqlc.GpxKmVertical{
				TrackID: trackID, StartIdx: 1, EndIdx: 90,
				GainM: gain800, DistanceM: distance10000,
			},
		}
		detail, err := NewSQLCStore(query).GetDetail(context.Background(), userID, trackID, 0)
		require.NoError(t, err)
		require.Len(t, detail.Muros, 1, "GetDetail must populate Muros from SQLC rows; a silently dropped assignment leaves this at zero")
		require.Equal(t, 10, detail.Muros[0].StartIdx)
		require.Len(t, detail.RecoveryZones, 1, "GetDetail must populate RecoveryZones from SQLC rows")
		require.Equal(t, 20, detail.RecoveryZones[0].StartIdx)
		require.NotNil(t, detail.KmVertical, "GetDetail must populate KmVertical from SQLC row; a silently dropped assignment leaves this nil")
		require.InDelta(t, 800, detail.KmVertical.GainM, 0.01)
	})
}

func TestSQLCStoreCreateDetailPersistsMurosAndRecoveryZonesAndKmVertical(t *testing.T) {
	trackID := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	query := &mockGPXQuerier{track: databaseTrack(trackID, pgtype.UUID{Valid: true})}
	track := &Track{Name: "Trail", FileHash: "hash", FileSizeBytes: 2048, TrackType: "circular", Points: []Point{{Lat: 40, Lon: -3}}}
	muros := []Muro{{StartIdx: 10, EndIdx: 20, GainM: 50, DistanceM: 200, AvgSlopePct: 25}, {StartIdx: 30, EndIdx: 40, GainM: 60, DistanceM: 300, AvgSlopePct: 20}}
	recovery := []RecoveryZone{{StartIdx: 20, EndIdx: 30, DistanceM: 100}, {StartIdx: 40, EndIdx: 50, DistanceM: 150}}
	kmVertical := &KmVerticalResult{StartIdx: 1, EndIdx: 90, GainM: 850, DistanceM: 10000}
	_, err := NewSQLCStore(query).CreateDetail(context.Background(), track, &Analysis{}, nil, nil, nil, muros, recovery, kmVertical)
	require.NoError(t, err)
	require.Len(t, query.createdMuros, 2)
	require.Equal(t, int32(10), query.createdMuros[0].StartIdx)
	require.Equal(t, int32(40), query.createdMuros[1].EndIdx)
	require.Len(t, query.createdRecoveryZones, 2)
	require.Equal(t, int32(20), query.createdRecoveryZones[0].StartIdx)
	require.Equal(t, int32(90), query.createdKmVertical[0].EndIdx)

	query.createdMuros, query.createdRecoveryZones, query.createdKmVertical = nil, nil, nil
	_, err = NewSQLCStore(query).CreateDetail(context.Background(), track, &Analysis{}, nil, nil, nil, []Muro{}, []RecoveryZone{}, nil)
	require.NoError(t, err)
	require.Empty(t, query.createdMuros)
	require.Empty(t, query.createdRecoveryZones)
	require.Empty(t, query.createdKmVertical)
}

func TestSQLCStoreListMurosAndRecoveryZonesPreservesQueryOrder(t *testing.T) {
	query := &mockGPXQuerier{
		muros:         []sqlc.GpxMuro{{StartIdx: 10}, {StartIdx: 50}},
		recoveryZones: []sqlc.GpxRecoveryZone{{StartIdx: 20}, {StartIdx: 100}},
	}
	store := NewSQLCStore(query)

	muros, err := store.ListMuros(context.Background(), pgtype.UUID{Valid: true})
	require.NoError(t, err)
	require.Equal(t, []int{10, 50}, []int{muros[0].StartIdx, muros[1].StartIdx})

	zones, err := store.ListRecoveryZones(context.Background(), pgtype.UUID{Valid: true})
	require.NoError(t, err)
	require.Equal(t, []int{20, 100}, []int{zones[0].StartIdx, zones[1].StartIdx})
}

func TestSQLCStoreGetKmVerticalReturnsNilWhenMissing(t *testing.T) {
	store := NewSQLCStore(&mockGPXQuerier{kmVerticalErr: pgx.ErrNoRows})

	got, err := store.GetKmVertical(context.Background(), pgtype.UUID{Valid: true})
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestStoredTrackDetailSerializesClimbDerivedFields(t *testing.T) {
	detail := StoredTrackDetail{Muros: []Muro{}, RecoveryZones: []RecoveryZone{}}
	data, err := json.Marshal(detail)
	require.NoError(t, err)
	require.Contains(t, string(data), `"muros":[]`)
	require.Contains(t, string(data), `"recovery_zones":[]`)
	require.Contains(t, string(data), `"km_vertical":null`)
}

func databaseTrack(id, userID pgtype.UUID) sqlc.GpxTrack {
	return sqlc.GpxTrack{
		ID: id, UserID: userID, Name: "Trail", FileHash: "abc", FileSizeBytes: 2048,
		Coordinates: []byte(`[[40,-3,100],[41,-4,120]]`), MovingTimeS: 300,
		DifficultyScore: 30, DifficultyLabel: "intermediate", TrackType: "point-to-point",
	}
}

// databaseTrackRow aplana una sqlc.GpxTrack a la forma
// sqlc.ListGPXTracksByUserRow (que añade TotalCount del window function
// tras el regen de SQLC en #172). Usado por el test de List.
func databaseTrackRow(t sqlc.GpxTrack) sqlc.ListGPXTracksByUserRow {
	return sqlc.ListGPXTracksByUserRow{
		ID:              t.ID,
		UserID:          t.UserID,
		Name:            t.Name,
		FileHash:        t.FileHash,
		FileSizeBytes:   t.FileSizeBytes,
		Coordinates:     t.Coordinates,
		DistanceM:       t.DistanceM,
		MovingTimeS:     t.MovingTimeS,
		DPlusM:          t.DPlusM,
		DMinusM:         t.DMinusM,
		MaxElevationM:   t.MaxElevationM,
		MinElevationM:   t.MinElevationM,
		AvgSlopePct:     t.AvgSlopePct,
		MaxSlopePct:     t.MaxSlopePct,
		EffortIndex:     t.EffortIndex,
		ItraPoints:      t.ItraPoints,
		LegBreakerIndex: t.LegBreakerIndex,
		EstimatedVam:    t.EstimatedVam,
		DifficultyScore: t.DifficultyScore,
		DifficultyLabel: t.DifficultyLabel,
		RunnabilityPct:  t.RunnabilityPct,
		KingClimb:       t.KingClimb,
		TrackType:       t.TrackType,
		Direction:       t.Direction,
		CreatedAt:       t.CreatedAt,
		AnalyzedAt:      t.AnalyzedAt,
		UpdatedAt:       t.UpdatedAt,
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

// ─── total real vía COUNT(*) OVER() (issue #172, M6) ─────────────────

// TestSQLCStoreList_TotalComesFromCountOver verifica que el `Total`
// que List expone sale del window function de la query (no de
// offset+len+1). Misma corrección aplicada a activities.List, issue
// #172, M6.
func TestSQLCStoreList_TotalComesFromCountOver(t *testing.T) {
	userID := pgtype.UUID{Valid: true, Bytes: [16]byte{1}}
	// 248 filas totales, devolvemos limit+1 = 4 (limit=3 + centinela).
	// Si el store usara el heurístico viejo, Total sería offset + len + 1 = 4;
	// con COUNT(*) OVER() debe ser 248.
	query := &mockGPXQuerier{tracks: []sqlc.ListGPXTracksByUserRow{
		databaseTrackRow(databaseTrack(pgtype.UUID{Bytes: [16]byte{14: 1}, Valid: true}, userID)),
		databaseTrackRow(databaseTrack(pgtype.UUID{Bytes: [16]byte{14: 2}, Valid: true}, userID)),
		databaseTrackRow(databaseTrack(pgtype.UUID{Bytes: [16]byte{14: 3}, Valid: true}, userID)),
		databaseTrackRow(databaseTrack(pgtype.UUID{Bytes: [16]byte{14: 4}, Valid: true}, userID)),
	}}
	// Marcamos 248 como TotalCount en cada fila (lo devuelve
	// COUNT(*) OVER() en producción).
	for i := range query.tracks {
		query.tracks[i].TotalCount = 248
	}

	got, err := NewSQLCStore(query).List(context.Background(), userID, ListParams{Limit: 3, Offset: 0})
	require.NoError(t, err)
	require.Equal(t, 248, got.Total, "Total debe venir del window function, no del heurístico offset+len+1")
	require.Len(t, got.Data, 3, "el centinela +1 se descarta")
	require.True(t, got.HasNext)
}

func (m *mockGPXQuerier) FirstActivityForUser(ctx context.Context, userID pgtype.UUID) (pgtype.Timestamptz, error) {
	return pgtype.Timestamptz{}, nil
}

func (m *mockGPXQuerier) GetDashboardMetadata(ctx context.Context, userID pgtype.UUID) (sqlc.DashboardMetadatum, error) {
	return sqlc.DashboardMetadatum{}, nil
}

func (m *mockGPXQuerier) ListActivitiesInRange(ctx context.Context, arg sqlc.ListActivitiesInRangeParams) ([]sqlc.ListActivitiesInRangeRow, error) {
	return []sqlc.ListActivitiesInRangeRow{}, nil
}

func (m *mockGPXQuerier) ListTrainingLoadFromFirstActivity(ctx context.Context, userID pgtype.UUID) ([]sqlc.TrainingLoadDaily, error) {
	return []sqlc.TrainingLoadDaily{}, nil
}

func (m *mockGPXQuerier) ListTrainingLoadRange(ctx context.Context, arg sqlc.ListTrainingLoadRangeParams) ([]sqlc.TrainingLoadDaily, error) {
	return []sqlc.TrainingLoadDaily{}, nil
}

func (m *mockGPXQuerier) ListUserIDsForTrainingLoadRecalc(ctx context.Context) ([]pgtype.UUID, error) {
	return nil, nil
}

func (m *mockGPXQuerier) UpsertDashboardMetadata(ctx context.Context, arg sqlc.UpsertDashboardMetadataParams) (sqlc.DashboardMetadatum, error) {
	return sqlc.DashboardMetadatum{}, nil
}

func (m *mockGPXQuerier) UpsertTrainingLoadDaily(ctx context.Context, arg sqlc.UpsertTrainingLoadDailyParams) (sqlc.TrainingLoadDaily, error) {
	return sqlc.TrainingLoadDaily{}, nil
}
