package gpx

import (
	"context"
	"io"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type Point struct {
	Lat  float64    `json:"lat"`
	Lon  float64    `json:"lon"`
	Ele  *float64   `json:"ele,omitempty"`
	Time *time.Time `json:"time,omitempty"`
}

type Track struct {
	ID            pgtype.UUID `json:"id"`
	UserID        pgtype.UUID `json:"user_id"`
	Name          string      `json:"name"`
	FileHash      string      `json:"file_hash"`
	FileSizeBytes int64       `json:"file_size_bytes"`
	Points        []Point     `json:"points"`
	TrackType     string      `json:"track_type"`
	Direction     string      `json:"direction,omitempty"`
}

type DifficultyLabel string

const (
	DifficultyBeginner     DifficultyLabel = "beginner"
	DifficultyIntermediate DifficultyLabel = "intermediate"
	DifficultyAdvanced     DifficultyLabel = "advanced"
	DifficultyPro          DifficultyLabel = "pro"
)

type Analysis struct {
	DistanceM       float64         `json:"distance_m"`
	MovingTimeS     int             `json:"moving_time_s"`
	DPlusM          float64         `json:"d_plus_m"`
	DMinusM         float64         `json:"d_minus_m"`
	MaxElevationM   *float64        `json:"max_elevation_m,omitempty"`
	MinElevationM   *float64        `json:"min_elevation_m,omitempty"`
	AverageSlopePct float64         `json:"avg_slope_pct"`
	MaxSlopePct     float64         `json:"max_slope_pct"`
	EffortIndex     float64         `json:"effort_index"`
	ITRAPoints      float64         `json:"itra_points"`
	LegBreakerIndex float64         `json:"leg_breaker_index"`
	EstimatedVAM    float64         `json:"estimated_vam"`
	DifficultyScore int             `json:"difficulty_score"`
	DifficultyLabel DifficultyLabel `json:"difficulty_label"`
	RunnabilityPct  float64         `json:"runnability_pct"`
}

type Climb struct {
	ID          pgtype.UUID `json:"id"`
	StartIdx    int         `json:"start_idx"`
	EndIdx      int         `json:"end_idx"`
	GainM       float64     `json:"gain_m"`
	DistanceM   float64     `json:"distance_m"`
	AvgSlopePct float64     `json:"avg_slope_pct"`
	IsKingClimb bool        `json:"is_king_climb"`
}

type RiskZone struct {
	ID       pgtype.UUID `json:"id"`
	StartIdx int         `json:"start_idx"`
	EndIdx   int         `json:"end_idx"`
	RiskType string      `json:"risk_type"`
	Severity string      `json:"severity"`
}

type StoredTrack struct {
	Track    Track    `json:"track"`
	Analysis Analysis `json:"analysis"`
}

type ListParams struct {
	Limit  int32
	Offset int32
}

type PaginatedTracks struct {
	Data    []StoredTrack `json:"data"`
	Total   int           `json:"total"`
	Limit   int32         `json:"limit"`
	Offset  int32         `json:"offset"`
	HasNext bool          `json:"has_next"`
}

type GPXParser interface {
	Parse(reader io.Reader) (*Track, error)
}

type GPXHasher interface {
	Hash(reader io.Reader) (string, error)
}

type GPXValidator interface {
	Validate(size int64, track *Track) error
}

type GPXAnalyzer interface {
	CalculateDistance(p1, p2 Point) float64
	CalculatePathDistance(points []Point) float64
	CalculateTotalDPlus(points []Point, threshold float64) float64
	CalculateTotalDMinus(points []Point, threshold float64) float64
	CalculateAverageSlope(distance, dPlus float64) float64
	CalculateEffortIndex(distanceKm, dPlus float64) float64
	CalculateITRAPoints(distanceKm, dPlus float64) float64
	CalculateLegBreakerIndex(slopes []float64) float64
	CalculateEstimatedVAM(distance, dPlus, timeSec float64) float64
	CalculateAdjustedSpeed(baseSpeed, slope float64) float64
	CalculateDifficulty(distance, dPlus, maxSlope float64) DifficultyLabel
	CalculateRunnabilityIndex(track *Track) float64
	CalculateMovingTime(points []Point, maxDeltaS int) int
}

type GPXStore interface {
	Create(ctx context.Context, track *Track, analysis *Analysis) error
	GetByID(ctx context.Context, userID pgtype.UUID, trackID pgtype.UUID) (*StoredTrack, error)
	List(ctx context.Context, userID pgtype.UUID, params ListParams) (*PaginatedTracks, error)
	Delete(ctx context.Context, userID pgtype.UUID, trackID pgtype.UUID) error
}
