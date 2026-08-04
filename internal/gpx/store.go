package gpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SQLCStore struct {
	q            gpxQuerier
	transactions transactionRunner
}

type transactionRunner interface {
	WithinTransaction(context.Context, func(gpxQuerier) error) error
}

type pgxTransactionRunner struct {
	pool *pgxpool.Pool
}

// gpxQuerier keeps GPX persistence isolated from sqlc's generated global
// Querier interface so adding GPX queries does not break unrelated mocks.
type gpxQuerier interface {
	CreateGPXTrack(context.Context, sqlc.CreateGPXTrackParams) (sqlc.GpxTrack, error)
	CreateGPXClimb(context.Context, sqlc.CreateGPXClimbParams) (sqlc.GpxClimb, error)
	CreateGPXRiskZone(context.Context, sqlc.CreateGPXRiskZoneParams) (sqlc.GpxRiskZone, error)
	GetGPXTrackByID(context.Context, sqlc.GetGPXTrackByIDParams) (sqlc.GpxTrack, error)
	GetGPXTrackByHash(context.Context, sqlc.GetGPXTrackByHashParams) (sqlc.GpxTrack, error)
	ListGPXClimbsByTrack(context.Context, pgtype.UUID) ([]sqlc.GpxClimb, error)
	ListGPXRiskZonesByTrack(context.Context, pgtype.UUID) ([]sqlc.GpxRiskZone, error)
	ListGPXTracksByUser(context.Context, sqlc.ListGPXTracksByUserParams) ([]sqlc.GpxTrack, error)
	DeleteGPXTrack(context.Context, sqlc.DeleteGPXTrackParams) error
}

func NewSQLCStore(q gpxQuerier) *SQLCStore {
	return &SQLCStore{q: q}
}

func NewTransactionalSQLCStore(pool *pgxpool.Pool) *SQLCStore {
	if pool == nil {
		return NewSQLCStore(nil)
	}
	return newTransactionalSQLCStore(sqlc.New(pool), pgxTransactionRunner{pool: pool})
}

func newTransactionalSQLCStore(q gpxQuerier, runner transactionRunner) *SQLCStore {
	return &SQLCStore{q: q, transactions: runner}
}

func (r pgxTransactionRunner) WithinTransaction(ctx context.Context, fn func(gpxQuerier) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("gpx: begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			slog.Warn("gpx: rollback transaction", "error", rollbackErr)
		}
	}()
	if err := fn(sqlc.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("gpx: commit transaction: %w", err)
	}
	return nil
}

func (s *SQLCStore) Create(ctx context.Context, track *Track, analysis *Analysis) error {
	if s == nil || s.q == nil {
		return fmt.Errorf("gpx: store is not initialized")
	}
	if track == nil || analysis == nil {
		return fmt.Errorf("gpx: track and analysis are required")
	}
	_, err := s.createTrack(ctx, track, analysis, nil)
	return err
}

func (s *SQLCStore) createTrack(ctx context.Context, track *Track, analysis *Analysis, kingClimb *Climb) (sqlc.GpxTrack, error) {
	coordinates, err := marshalCoordinates(track.Points)
	if err != nil {
		return sqlc.GpxTrack{}, err
	}
	kingJSON, err := marshalKingClimb(kingClimb)
	if err != nil {
		return sqlc.GpxTrack{}, err
	}
	row, err := s.q.CreateGPXTrack(ctx, sqlc.CreateGPXTrackParams{
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
		KingClimb:       kingJSON,
		TrackType:       track.TrackType,
		Direction:       pgtype.Text{String: track.Direction, Valid: track.Direction != ""},
	})
	if err != nil {
		return sqlc.GpxTrack{}, fmt.Errorf("gpx: create track: %w", err)
	}
	return row, nil
}

func (s *SQLCStore) CreateDetail(ctx context.Context, track *Track, analysis *Analysis, climbs []Climb, riskZones []RiskZone, kingClimb *Climb) (*StoredTrackDetail, error) {
	if s == nil || s.q == nil {
		return nil, fmt.Errorf("gpx: store is not initialized")
	}
	if track == nil || analysis == nil {
		return nil, fmt.Errorf("gpx: track and analysis are required")
	}
	if s.transactions != nil {
		var detail *StoredTrackDetail
		err := s.transactions.WithinTransaction(ctx, func(q gpxQuerier) error {
			var createErr error
			detail, createErr = NewSQLCStore(q).createDetail(ctx, track, analysis, climbs, riskZones, kingClimb)
			return createErr
		})
		return detail, err
	}
	return s.createDetail(ctx, track, analysis, climbs, riskZones, kingClimb)
}

func (s *SQLCStore) createDetail(ctx context.Context, track *Track, analysis *Analysis, climbs []Climb, riskZones []RiskZone, kingClimb *Climb) (*StoredTrackDetail, error) {
	row, err := s.createTrack(ctx, track, analysis, kingClimb)
	if err != nil {
		return nil, err
	}
	for _, climb := range climbs {
		_, err = s.q.CreateGPXClimb(ctx, sqlc.CreateGPXClimbParams{
			TrackID: row.ID, IsKingClimb: climb.IsKingClimb, StartIdx: int32(climb.StartIdx), EndIdx: int32(climb.EndIdx),
			GainM: numeric(climb.GainM), DistanceM: numeric(climb.DistanceM), AvgSlopePct: numeric(climb.AvgSlopePct),
		})
		if err != nil {
			return nil, fmt.Errorf("gpx: create climb: %w", err)
		}
	}
	for _, zone := range riskZones {
		_, err = s.q.CreateGPXRiskZone(ctx, sqlc.CreateGPXRiskZoneParams{
			TrackID: row.ID, StartIdx: int32(zone.StartIdx), EndIdx: int32(zone.EndIdx), RiskType: zone.RiskType, Severity: zone.Severity,
		})
		if err != nil {
			return nil, fmt.Errorf("gpx: create risk zone: %w", err)
		}
	}
	stored, err := storedTrack(row)
	if err != nil {
		return nil, err
	}
	return &StoredTrackDetail{Track: *stored, Climbs: climbs, RiskZones: riskZones}, nil
}

func (s *SQLCStore) FindByHash(ctx context.Context, userID pgtype.UUID, fileHash string) (*StoredTrack, error) {
	if s == nil || s.q == nil {
		return nil, fmt.Errorf("gpx: store is not initialized")
	}
	row, err := s.q.GetGPXTrackByHash(ctx, sqlc.GetGPXTrackByHashParams{UserID: userID, FileHash: fileHash})
	if err != nil {
		return nil, fmt.Errorf("gpx: find track by hash: %w", err)
	}
	return storedTrack(row)
}

func (s *SQLCStore) GetDetail(ctx context.Context, userID, trackID pgtype.UUID) (*StoredTrackDetail, error) {
	track, err := s.GetByID(ctx, userID, trackID)
	if err != nil {
		return nil, err
	}
	climbs, err := s.ListClimbs(ctx, trackID)
	if err != nil {
		return nil, err
	}
	risks, err := s.ListRiskZones(ctx, trackID)
	if err != nil {
		return nil, err
	}
	return &StoredTrackDetail{Track: *track, Climbs: climbs, RiskZones: risks}, nil
}

func (s *SQLCStore) ListClimbs(ctx context.Context, trackID pgtype.UUID) ([]Climb, error) {
	if s == nil || s.q == nil {
		return nil, fmt.Errorf("gpx: store is not initialized")
	}
	rows, err := s.q.ListGPXClimbsByTrack(ctx, trackID)
	if err != nil {
		return nil, fmt.Errorf("gpx: list climbs: %w", err)
	}
	climbs := make([]Climb, 0, len(rows))
	for _, row := range rows {
		climbs = append(climbs, Climb{ID: row.ID, StartIdx: int(row.StartIdx), EndIdx: int(row.EndIdx), GainM: numericValue(row.GainM), DistanceM: numericValue(row.DistanceM), AvgSlopePct: numericValue(row.AvgSlopePct), IsKingClimb: row.IsKingClimb})
	}
	return climbs, nil
}

func (s *SQLCStore) ListRiskZones(ctx context.Context, trackID pgtype.UUID) ([]RiskZone, error) {
	if s == nil || s.q == nil {
		return nil, fmt.Errorf("gpx: store is not initialized")
	}
	rows, err := s.q.ListGPXRiskZonesByTrack(ctx, trackID)
	if err != nil {
		return nil, fmt.Errorf("gpx: list risk zones: %w", err)
	}
	zones := make([]RiskZone, 0, len(rows))
	for _, row := range rows {
		zones = append(zones, RiskZone{ID: row.ID, StartIdx: int(row.StartIdx), EndIdx: int(row.EndIdx), RiskType: row.RiskType, Severity: row.Severity})
	}
	return zones, nil
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

func marshalKingClimb(climb *Climb) ([]byte, error) {
	if climb == nil {
		return nil, nil
	}
	value := struct {
		StartIdx    int     `json:"start_idx"`
		EndIdx      int     `json:"end_idx"`
		GainM       float64 `json:"gain_m"`
		DistanceM   float64 `json:"distance_m"`
		AvgSlopePct float64 `json:"avg_slope_pct"`
		IsKingClimb bool    `json:"is_king_climb"`
	}{climb.StartIdx, climb.EndIdx, climb.GainM, climb.DistanceM, climb.AvgSlopePct, climb.IsKingClimb}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("gpx: marshal king climb: %w", err)
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
