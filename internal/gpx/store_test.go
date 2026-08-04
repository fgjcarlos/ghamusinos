package gpx

import (
	"context"
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
}

func (m *mockGPXQuerier) CreateGPXTrack(_ context.Context, params sqlc.CreateGPXTrackParams) (sqlc.GpxTrack, error) {
	m.created = params
	return m.track, nil
}

func (m *mockGPXQuerier) GetGPXTrackByID(_ context.Context, params sqlc.GetGPXTrackByIDParams) (sqlc.GpxTrack, error) {
	m.getParams = params
	return m.track, nil
}

func (m *mockGPXQuerier) ListGPXTracksByUser(_ context.Context, params sqlc.ListGPXTracksByUserParams) ([]sqlc.GpxTrack, error) {
	m.listParams = params
	return m.tracks, nil
}

func (m *mockGPXQuerier) DeleteGPXTrack(_ context.Context, params sqlc.DeleteGPXTrackParams) error {
	m.deleteParams = params
	return nil
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

func databaseTrack(id, userID pgtype.UUID) sqlc.GpxTrack {
	return sqlc.GpxTrack{
		ID: id, UserID: userID, Name: "Trail", FileHash: "abc", FileSizeBytes: 2048,
		Coordinates: []byte(`[[40,-3,100],[41,-4,120]]`), MovingTimeS: 300,
		DifficultyScore: 30, DifficultyLabel: "intermediate", TrackType: "point-to-point",
	}
}
