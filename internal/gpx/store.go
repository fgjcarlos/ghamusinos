package gpx

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type SQLCStore struct {
	q sqlc.Querier
}

func NewSQLCStore(q sqlc.Querier) *SQLCStore {
	return &SQLCStore{q: q}
}

func (s *SQLCStore) Create(ctx context.Context, track *Track, analysis *Analysis) error {
	if s == nil || s.q == nil {
		return fmt.Errorf("gpx: store is not initialized")
	}
	if track == nil || analysis == nil {
		return fmt.Errorf("gpx: track and analysis are required")
	}
	coordinates, err := marshalCoordinates(track.Points)
	if err != nil {
		return err
	}
	_, err = s.q.CreateGPXTrack(ctx, sqlc.CreateGPXTrackParams{
		UserID:          track.UserID,
		Name:            track.Name,
		FileHash:        track.FileHash,
		FileSizeBytes:   track.FileSizeBytes,
		Coordinates:     coordinates,
		DistanceM:       numeric(analysis.DistanceM),
		MovingTimeS:     int32(analysis.MovingTimeS),
		DPlusM:          numeric(analysis.DPlusM),
		DMinusM:         numeric(analysis.DMinusM),
		MaxElevationM:   optionalNumeric(analysis.MaxElevationM),
		MinElevationM:   optionalNumeric(analysis.MinElevationM),
		AvgSlopePct:     numeric(analysis.AverageSlopePct),
		MaxSlopePct:     numeric(analysis.MaxSlopePct),
		EffortIndex:     numeric(analysis.EffortIndex),
		ItraPoints:      numeric(analysis.ITRAPoints),
		LegBreakerIndex: numeric(analysis.LegBreakerIndex),
		EstimatedVam:    numeric(analysis.EstimatedVAM),
		DifficultyScore: int32(analysis.DifficultyScore),
		DifficultyLabel: string(analysis.DifficultyLabel),
		RunnabilityPct:  numeric(analysis.RunnabilityPct),
		TrackType:       track.TrackType,
		Direction:       pgtype.Text{String: track.Direction, Valid: track.Direction != ""},
	})
	if err != nil {
		return fmt.Errorf("gpx: create track: %w", err)
	}
	return nil
}

func (s *SQLCStore) GetByID(ctx context.Context, userID, trackID pgtype.UUID) (*StoredTrack, error) {
	if s == nil || s.q == nil {
		return nil, fmt.Errorf("gpx: store is not initialized")
	}
	row, err := s.q.GetGPXTrackByID(ctx, sqlc.GetGPXTrackByIDParams{ID: trackID, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("gpx: get track: %w", err)
	}
	return storedTrack(row)
}

func (s *SQLCStore) List(ctx context.Context, userID pgtype.UUID, params ListParams) (*PaginatedTracks, error) {
	if s == nil || s.q == nil {
		return nil, fmt.Errorf("gpx: store is not initialized")
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.q.ListGPXTracksByUser(ctx, sqlc.ListGPXTracksByUserParams{
		UserID: userID,
		Limit:  limit + 1,
		Offset: params.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("gpx: list tracks: %w", err)
	}
	hasNext := len(rows) > int(limit)
	if hasNext {
		rows = rows[:int(limit)]
	}
	result := &PaginatedTracks{Data: make([]StoredTrack, 0, len(rows)), Limit: limit, Offset: params.Offset, HasNext: hasNext}
	for _, row := range rows {
		item, convertErr := storedTrack(row)
		if convertErr != nil {
			return nil, convertErr
		}
		result.Data = append(result.Data, *item)
	}
	result.Total = int(params.Offset) + len(result.Data)
	if hasNext {
		result.Total++
	}
	return result, nil
}

func (s *SQLCStore) Delete(ctx context.Context, userID, trackID pgtype.UUID) error {
	if s == nil || s.q == nil {
		return fmt.Errorf("gpx: store is not initialized")
	}
	if err := s.q.DeleteGPXTrack(ctx, sqlc.DeleteGPXTrackParams{ID: trackID, UserID: userID}); err != nil {
		return fmt.Errorf("gpx: delete track: %w", err)
	}
	return nil
}

func marshalCoordinates(points []Point) ([]byte, error) {
	coordinates := make([][3]any, len(points))
	for i, point := range points {
		coordinates[i] = [3]any{point.Lat, point.Lon, point.Ele}
	}
	data, err := json.Marshal(coordinates)
	if err != nil {
		return nil, fmt.Errorf("gpx: marshal coordinates: %w", err)
	}
	return data, nil
}

func storedTrack(row sqlc.GpxTrack) (*StoredTrack, error) {
	points, err := unmarshalCoordinates(row.Coordinates)
	if err != nil {
		return nil, err
	}
	return &StoredTrack{
		Track: Track{
			ID: row.ID, UserID: row.UserID, Name: row.Name, FileHash: row.FileHash,
			FileSizeBytes: row.FileSizeBytes, Points: points, TrackType: row.TrackType,
			Direction: row.Direction.String,
		},
		Analysis: Analysis{
			DistanceM: numericValue(row.DistanceM), MovingTimeS: int(row.MovingTimeS),
			DPlusM: numericValue(row.DPlusM), DMinusM: numericValue(row.DMinusM),
			MaxElevationM: numericPointer(row.MaxElevationM), MinElevationM: numericPointer(row.MinElevationM),
			AverageSlopePct: numericValue(row.AvgSlopePct), MaxSlopePct: numericValue(row.MaxSlopePct),
			EffortIndex: numericValue(row.EffortIndex), ITRAPoints: numericValue(row.ItraPoints),
			LegBreakerIndex: numericValue(row.LegBreakerIndex), EstimatedVAM: numericValue(row.EstimatedVam),
			DifficultyScore: int(row.DifficultyScore), DifficultyLabel: DifficultyLabel(row.DifficultyLabel),
			RunnabilityPct: numericValue(row.RunnabilityPct),
		},
	}, nil
}

func unmarshalCoordinates(data []byte) ([]Point, error) {
	var coordinates [][]*float64
	if err := json.Unmarshal(data, &coordinates); err != nil {
		return nil, fmt.Errorf("gpx: unmarshal coordinates: %w", err)
	}
	points := make([]Point, 0, len(coordinates))
	for _, coordinate := range coordinates {
		if len(coordinate) < 2 || coordinate[0] == nil || coordinate[1] == nil {
			return nil, fmt.Errorf("gpx: invalid stored coordinate")
		}
		point := Point{Lat: *coordinate[0], Lon: *coordinate[1]}
		if len(coordinate) > 2 {
			point.Ele = coordinate[2]
		}
		points = append(points, point)
	}
	return points, nil
}

func numeric(value float64) pgtype.Numeric {
	var result pgtype.Numeric
	if err := result.Scan(strconv.FormatFloat(value, 'f', -1, 64)); err != nil {
		panic(fmt.Sprintf("gpx: convert numeric: %v", err))
	}
	return result
}

func optionalNumeric(value *float64) pgtype.Numeric {
	if value == nil {
		return pgtype.Numeric{}
	}
	return numeric(*value)
}

func numericValue(value pgtype.Numeric) float64 {
	converted, err := value.Float64Value()
	if err != nil || !converted.Valid {
		return 0
	}
	return converted.Float64
}

func numericPointer(value pgtype.Numeric) *float64 {
	converted, err := value.Float64Value()
	if err != nil || !converted.Valid {
		return nil
	}
	return &converted.Float64
}
