package gpx

import (
	"context"
	"errors"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type mockGPXQuerier struct {
	sqlc.Querier
	created      sqlc.CreateGPXTrackParams
	getParams    sqlc.GetGPXTrackByIDParams
	listParams   sqlc.ListGPXTracksByUserParams
	deleteParams sqlc.DeleteGPXTrackParams
	track        sqlc.GpxTrack
	tracks       []sqlc.GpxTrack
	err          error
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
	got, err := NewSQLCStore(query).GetByID(context.Background(), userID, trackID)
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
	_, err := NewSQLCStore(nil).GetByID(ctx, pgtype.UUID{}, pgtype.UUID{})
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
	_, err := NewSQLCStore(query).GetByID(ctx, pgtype.UUID{}, pgtype.UUID{})
	require.ErrorContains(t, err, "get track")
	_, err = NewSQLCStore(query).List(ctx, pgtype.UUID{}, ListParams{})
	require.ErrorContains(t, err, "list tracks")
	require.ErrorContains(t, NewSQLCStore(query).Delete(ctx, pgtype.UUID{}, pgtype.UUID{}), "delete track")
}

func TestSQLCStoreRejectsInvalidStoredCoordinates(t *testing.T) {
	query := &mockGPXQuerier{track: databaseTrack(pgtype.UUID{}, pgtype.UUID{})}
	query.track.Coordinates = []byte(`not-json`)
	_, err := NewSQLCStore(query).GetByID(context.Background(), pgtype.UUID{}, pgtype.UUID{})
	require.ErrorContains(t, err, "unmarshal coordinates")
}

func databaseTrack(id, userID pgtype.UUID) sqlc.GpxTrack {
	return sqlc.GpxTrack{
		ID: id, UserID: userID, Name: "Trail", FileHash: "abc", FileSizeBytes: 2048,
		Coordinates: []byte(`[[40,-3,100],[41,-4,120]]`), MovingTimeS: 300,
		DifficultyScore: 30, DifficultyLabel: "intermediate", TrackType: "point-to-point",
	}
}
