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

// ErrNumericConversion marca un valor que pgtype.Numeric.Scan rechazó
// (p.ej. +Inf, -Inf, NaN propagado desde una división por cero en el
// analizador). El handler lo traduce a 422 con "el análisis produjo un
// valor no representable" en vez de devolver un 500 opaco. Antes el
// código paniqueaba (issue #170, M8).
var ErrNumericConversion = errors.New("gpx: numeric conversion failed (non-representable value)")

// analysisNumerics precomputa todos los pgtype.Numeric que se persisten
// en una fila de gpx_tracks a partir de un *Analysis. Concentrar la
// conversión aquí permite que createTrack no tenga que tejer un error
// por cada campo (doce) y, sobre todo, que cualquier +Inf/-Inf/NaN del
// analizador se detecte y se devuelva con ErrNumericConversion en vez
// de panickear (issue #170, M8).
type analysisNumerics struct {
	DistanceM, DPlusM, DMinusM                             pgtype.Numeric
	MaxElevationM, MinElevationM                           pgtype.Numeric
	AverageSlopePct, MaxSlopePct                           pgtype.Numeric
	EffortIndex, ITRAPoints, LegBreakerIndex, EstimatedVAM pgtype.Numeric
	RunnabilityPct                                         pgtype.Numeric
}

// toAnalysisNumerics convierte cada campo numérico de a a
// pgtype.Numeric. Falla rápido en el primer valor no representable,
// devolviendo ErrNumericConversion envuelto con el nombre del campo
// para que el handler pueda construir el 422 con contexto.
func toAnalysisNumerics(a *Analysis) (analysisNumerics, error) {
	var n analysisNumerics
	var err error

	if n.DistanceM, err = numeric(a.DistanceM); err != nil {
		return analysisNumerics{}, fmt.Errorf("%w: distance_m", ErrNumericConversion)
	}
	if n.DPlusM, err = numeric(a.DPlusM); err != nil {
		return analysisNumerics{}, fmt.Errorf("%w: d_plus_m", ErrNumericConversion)
	}
	if n.DMinusM, err = numeric(a.DMinusM); err != nil {
		return analysisNumerics{}, fmt.Errorf("%w: d_minus_m", ErrNumericConversion)
	}
	if n.AverageSlopePct, err = numeric(a.AverageSlopePct); err != nil {
		return analysisNumerics{}, fmt.Errorf("%w: average_slope_pct", ErrNumericConversion)
	}
	if n.MaxSlopePct, err = numeric(a.MaxSlopePct); err != nil {
		return analysisNumerics{}, fmt.Errorf("%w: max_slope_pct", ErrNumericConversion)
	}
	if n.EffortIndex, err = numeric(a.EffortIndex); err != nil {
		return analysisNumerics{}, fmt.Errorf("%w: effort_index", ErrNumericConversion)
	}
	if n.ITRAPoints, err = numeric(a.ITRAPoints); err != nil {
		return analysisNumerics{}, fmt.Errorf("%w: itra_points", ErrNumericConversion)
	}
	if n.LegBreakerIndex, err = numeric(a.LegBreakerIndex); err != nil {
		return analysisNumerics{}, fmt.Errorf("%w: leg_breaker_index", ErrNumericConversion)
	}
	if n.EstimatedVAM, err = numeric(a.EstimatedVAM); err != nil {
		return analysisNumerics{}, fmt.Errorf("%w: estimated_vam", ErrNumericConversion)
	}
	if n.RunnabilityPct, err = numeric(a.RunnabilityPct); err != nil {
		return analysisNumerics{}, fmt.Errorf("%w: runnability_pct", ErrNumericConversion)
	}
	if n.MaxElevationM, err = optionalNumeric(a.MaxElevationM); err != nil {
		return analysisNumerics{}, fmt.Errorf("%w: max_elevation_m", ErrNumericConversion)
	}
	if n.MinElevationM, err = optionalNumeric(a.MinElevationM); err != nil {
		return analysisNumerics{}, fmt.Errorf("%w: min_elevation_m", ErrNumericConversion)
	}
	return n, nil
}

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
	ListGPXTracksByUser(context.Context, sqlc.ListGPXTracksByUserParams) ([]sqlc.ListGPXTracksByUserRow, error)
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
	// Convertimos todos los campos numéricos del análisis una sola vez.
	// Si alguno es +Inf/-Inf/NaN, ErrNumericConversion burbujea al
	// handler, que responde 422 en vez del 500 opaco anterior
	// (issue #170, M8).
	n, err := toAnalysisNumerics(analysis)
	if err != nil {
		return sqlc.GpxTrack{}, err
	}
	row, err := s.q.CreateGPXTrack(ctx, sqlc.CreateGPXTrackParams{
		UserID:          track.UserID,
		Name:            track.Name,
		FileHash:        track.FileHash,
		FileSizeBytes:   track.FileSizeBytes,
		Coordinates:     coordinates,
		DistanceM:       n.DistanceM,
		MovingTimeS:     int32(analysis.MovingTimeS),
		DPlusM:          n.DPlusM,
		DMinusM:         n.DMinusM,
		MaxElevationM:   n.MaxElevationM,
		MinElevationM:   n.MinElevationM,
		AvgSlopePct:     n.AverageSlopePct,
		MaxSlopePct:     n.MaxSlopePct,
		EffortIndex:     n.EffortIndex,
		ItraPoints:      n.ITRAPoints,
		LegBreakerIndex: n.LegBreakerIndex,
		EstimatedVam:    n.EstimatedVAM,
		DifficultyScore: int32(analysis.DifficultyScore),
		DifficultyLabel: string(analysis.DifficultyLabel),
		RunnabilityPct:  n.RunnabilityPct,
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
		gainM, err := numeric(climb.GainM)
		if err != nil {
			return nil, fmt.Errorf("%w: climb.gain_m", ErrNumericConversion)
		}
		distM, err := numeric(climb.DistanceM)
		if err != nil {
			return nil, fmt.Errorf("%w: climb.distance_m", ErrNumericConversion)
		}
		slope, err := numeric(climb.AvgSlopePct)
		if err != nil {
			return nil, fmt.Errorf("%w: climb.avg_slope_pct", ErrNumericConversion)
		}
		_, err = s.q.CreateGPXClimb(ctx, sqlc.CreateGPXClimbParams{
			TrackID: row.ID, IsKingClimb: climb.IsKingClimb, StartIdx: int32(climb.StartIdx), EndIdx: int32(climb.EndIdx),
			GainM: gainM, DistanceM: distM, AvgSlopePct: slope,
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
	stored, err := storedTrack(row, 0)
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
	return storedTrack(row, 0)
}

func (s *SQLCStore) GetDetail(ctx context.Context, userID, trackID pgtype.UUID, resolution int) (*StoredTrackDetail, error) {
	track, err := s.GetByID(ctx, userID, trackID, resolution)
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

func (s *SQLCStore) GetByID(ctx context.Context, userID, trackID pgtype.UUID, resolution int) (*StoredTrack, error) {
	if s == nil || s.q == nil {
		return nil, fmt.Errorf("gpx: store is not initialized")
	}
	row, err := s.q.GetGPXTrackByID(ctx, sqlc.GetGPXTrackByIDParams{ID: trackID, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("gpx: get track: %w", err)
	}
	return storedTrack(row, resolution)
}

func (s *SQLCStore) List(ctx context.Context, userID pgtype.UUID, params ListParams) (*PaginatedTracks, error) {
	if s == nil || s.q == nil {
		return nil, fmt.Errorf("gpx: store is not initialized")
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	// Pedimos limit+1 para detectar has_next sin segundo query (mismo
	// patrón que ya usábamos). LIMIT $2 se ejecuta antes que
	// COUNT(*) OVER() en Postgres, así que TotalCount en cada fila
	// sigue siendo el total real subyacente al WHERE, no el LIMIT+1.
	rows, err := s.q.ListGPXTracksByUser(ctx, sqlc.ListGPXTracksByUserParams{
		UserID: userID,
		Limit:  limit + 1,
		Offset: params.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("gpx: list tracks: %w", err)
	}
	// Total real vía COUNT(*) OVER() (issue #172, M6). Todas las filas
	// traen el mismo valor; si la lista está vacía no hay total.
	var total int64
	if len(rows) > 0 {
		total = rows[0].TotalCount
	}
	hasNext := len(rows) > int(limit)
	if hasNext {
		rows = rows[:int(limit)]
	}
	result := &PaginatedTracks{Data: make([]StoredTrack, 0, len(rows)), Limit: limit, Offset: params.Offset, HasNext: hasNext, Total: int(total)}
	for _, row := range rows {
		item, convertErr := storedTrack(listRowToGpxTrack(row), 0)
		if convertErr != nil {
			return nil, convertErr
		}
		result.Data = append(result.Data, *item)
	}
	return result, nil
}

// listRowToGpxTrack aplana el row generado por ListGPXTracksByUser (que
// lleva el TotalCount del window function) a la forma sqlc.GpxTrack que
// espera storedTrack. Solo se llama desde List, que es el único punto
// que consume la nueva query con TotalCount.
func listRowToGpxTrack(row sqlc.ListGPXTracksByUserRow) sqlc.GpxTrack {
	return sqlc.GpxTrack{
		ID:              row.ID,
		UserID:          row.UserID,
		Name:            row.Name,
		FileHash:        row.FileHash,
		FileSizeBytes:   row.FileSizeBytes,
		Coordinates:     row.Coordinates,
		DistanceM:       row.DistanceM,
		MovingTimeS:     row.MovingTimeS,
		DPlusM:          row.DPlusM,
		DMinusM:         row.DMinusM,
		MaxElevationM:   row.MaxElevationM,
		MinElevationM:   row.MinElevationM,
		AvgSlopePct:     row.AvgSlopePct,
		MaxSlopePct:     row.MaxSlopePct,
		EffortIndex:     row.EffortIndex,
		ItraPoints:      row.ItraPoints,
		LegBreakerIndex: row.LegBreakerIndex,
		EstimatedVam:    row.EstimatedVam,
		DifficultyScore: row.DifficultyScore,
		DifficultyLabel: row.DifficultyLabel,
		RunnabilityPct:  row.RunnabilityPct,
		KingClimb:       row.KingClimb,
		TrackType:       row.TrackType,
		Direction:       row.Direction,
		CreatedAt:       row.CreatedAt,
		AnalyzedAt:      row.AnalyzedAt,
		UpdatedAt:       row.UpdatedAt,
	}
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

func storedTrack(row sqlc.GpxTrack, resolution int) (*StoredTrack, error) {
	points, err := unmarshalCoordinates(row.Coordinates)
	if err != nil {
		return nil, err
	}
	// Submuestreo aquí, no en el handler: el JSONB de coordinates puede
	// traer 200 000 puntos y queremos dejar de moverlos lo antes posible
	// (issue #170, M9). resolution <= 0 cae a DefaultResolution dentro
	// de subsamplePoints.
	points = subsamplePoints(points, resolution)
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

// DefaultResolution es el número objetivo de columnas que GetGPX
// devuelve cuando el caller no especifica uno. El cliente de detalle de
// ruta (1280 px de ancho, issue #04 del rediseño) tiene menos columnas
// que eso; el margen es para que el submuestreo no se note en pantalla.
// Ajustar si el rediseño cambia el ancho.
const DefaultResolution = 2000

// subsamplePoints reduce la lista de puntos a como mucho ~2*resolution
// puntos, agrupando por columna X y conservando, en cada grupo, los
// puntos con elevación mínima y máxima. Si solo hay un extremo
// distinto en el grupo, devuelve uno. Esto preserva los picos y valles
// del perfil (algo que un downsampling 1-de-cada-N aplana) y deja en
// unos 2 000 – 4 000 puntos la respuesta por defecto para un GPX de
// 200 000 puntos (issue #170, M9).
//
// Casos límite:
//   - n <= resolution: devuelve los puntos tal cual.
//   - grupo sin elevación: devuelve el primer punto del grupo (evita
//     perder horizontalidad sin inventar elevación).
//   - resolution <= 0: cae a DefaultResolution.
func subsamplePoints(points []Point, resolution int) []Point {
	if resolution <= 0 {
		resolution = DefaultResolution
	}
	n := len(points)
	if n <= resolution {
		return points
	}
	// bucketSize por techo: si resolution no divide n, el último
	// grupo cubre el resto y no perdemos puntos por el camino.
	bucketSize := (n + resolution - 1) / resolution
	out := make([]Point, 0, 2*resolution)
	for start := 0; start < n; start += bucketSize {
		end := start + bucketSize
		if end > n {
			end = n
		}
		minIdx, maxIdx := -1, -1
		for i := start; i < end; i++ {
			if points[i].Ele == nil {
				continue
			}
			if minIdx == -1 || *points[i].Ele < *points[minIdx].Ele {
				minIdx = i
			}
			if maxIdx == -1 || *points[i].Ele > *points[maxIdx].Ele {
				maxIdx = i
			}
		}
		switch {
		case minIdx == -1 && maxIdx == -1:
			// Grupo sin elevación: mantenemos el primer punto del grupo
			// para no perder su tramo horizontal.
			out = append(out, points[start])
		case minIdx == maxIdx:
			out = append(out, points[minIdx])
		default:
			// Preservamos el orden X: índice menor primero.
			if minIdx < maxIdx {
				out = append(out, points[minIdx], points[maxIdx])
			} else {
				out = append(out, points[maxIdx], points[minIdx])
			}
		}
	}
	return out
}

// numeric serializa un float64 a pgtype.Numeric. Devuelve error en vez
// de panicar para que un +Inf propagado desde el análisis no tumbe la
// goroutine del handler (issue #170, M8). El handler traduce a 422 con
// "el análisis produjo un valor no representable" en vez de a un 500
// opaco.
func numeric(value float64) (pgtype.Numeric, error) {
	var result pgtype.Numeric
	if err := result.Scan(strconv.FormatFloat(value, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}, fmt.Errorf("%w (input=%v): %v", ErrNumericConversion, value, err)
	}
	return result, nil
}

func optionalNumeric(value *float64) (pgtype.Numeric, error) {
	if value == nil {
		return pgtype.Numeric{}, nil
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
