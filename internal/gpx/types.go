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
	DistanceM   float64 `json:"distance_m"`
	MovingTimeS int     `json:"moving_time_s"`
	// DPlusM and DMinusM are nullable because a track with insufficient
	// elevation coverage (< CoverageMinThreshold) does not publish a
	// number — a zero would be a lie. Coverage captures the fraction
	// of the track with usable elevation, so the frontend can decide
	// whether to warn. Issue #171, A8.
	DPlusM            *float64        `json:"d_plus_m"`
	DMinusM           *float64        `json:"d_minus_m"`
	ElevationCoverage *float64        `json:"elevation_coverage,omitempty"`
	MaxElevationM     *float64        `json:"max_elevation_m,omitempty"`
	MinElevationM     *float64        `json:"min_elevation_m,omitempty"`
	AverageSlopePct   float64         `json:"avg_slope_pct"`
	MaxSlopePct       float64         `json:"max_slope_pct"`
	EffortIndex       float64         `json:"effort_index"`
	ITRAPoints        float64         `json:"itra_points"`
	LegBreakerIndex   float64         `json:"leg_breaker_index"`
	EstimatedVAM      float64         `json:"estimated_vam"`
	DifficultyScore   int             `json:"difficulty_score"`
	DifficultyLabel   DifficultyLabel `json:"difficulty_label"`
	RunnabilityPct    float64         `json:"runnability_pct"`
}

// CoverageStatus captures the policy for what the elevation analysis
// considered usable. The thresholds live in this package as exported
// constants so callers and tests reference a single source of truth.
type CoverageStatus int

const (
	// CoverageFull: coverage ≥ CoverageFullThreshold. The D+/D- values
	// are trusted as-is.
	CoverageFull CoverageStatus = iota
	// CoveragePartial: CoverageMinThreshold ≤ coverage < CoverageFullThreshold.
	// The D+/D- values are usable but flagged for the UI to mark as
	// partial estimates.
	CoveragePartial
	// CoverageInsufficient: coverage < CoverageMinThreshold. The D+/D-
	// values are nil — we do not publish a number we cannot stand behind.
	CoverageInsufficient
)

const (
	// CoverageFullThreshold is the coverage above which the elevation
	// analysis is considered authoritative.
	CoverageFullThreshold = 0.95
	// CoverageMinThreshold is the minimum coverage required to publish a
	// D+/D- value at all. Below this we return nil and status
	// Insufficient.
	CoverageMinThreshold = 0.50
)

// ElevationResult is the output of CalculateElevation. DPlusM and
// DMinusM are nil exactly when Status == CoverageInsufficient — i.e.,
// when coverage is too low to trust the calculation. Coverage is always
// populated (it can be zero if no point has elevation).
type ElevationResult struct {
	DPlusM   *float64
	DMinusM  *float64
	Coverage float64
	Status   CoverageStatus
}

type Climb struct {
	ID          pgtype.UUID `json:"id"`
	StartIdx    int         `json:"start_idx"`
	EndIdx      int         `json:"end_idx"`
	GainM       float64     `json:"gain_m"`
	DistanceM   float64     `json:"distance_m"`
	AvgSlopePct float64     `json:"avg_slope_pct"`
	IsKingClimb bool        `json:"is_king_climb"`
	VAM         *float64    `json:"vam,omitempty"`
}

type Muro struct {
	StartIdx    int     `json:"start_idx"`
	EndIdx      int     `json:"end_idx"`
	GainM       float64 `json:"gain_m"`
	DistanceM   float64 `json:"distance_m"`
	AvgSlopePct float64 `json:"avg_slope_pct"`
}

type RecoveryZone struct {
	StartIdx  int     `json:"start_idx"`
	EndIdx    int     `json:"end_idx"`
	DistanceM float64 `json:"distance_m"`
}

type KmVerticalResult struct {
	StartIdx  int     `json:"start_idx"`
	EndIdx    int     `json:"end_idx"`
	GainM     float64 `json:"gain_m"`
	DistanceM float64 `json:"distance_m"`
}

type RiskZone struct {
	ID       pgtype.UUID `json:"id"`
	StartIdx int         `json:"start_idx"`
	EndIdx   int         `json:"end_idx"`
	RiskType string      `json:"risk_type"`
	Severity string      `json:"severity"`
}

type TrackTypeResult struct {
	Type      string `json:"type"`
	Direction string `json:"direction,omitempty"`
}

type StoredTrack struct {
	Track    Track    `json:"track"`
	Analysis Analysis `json:"analysis"`
}

type StoredTrackDetail struct {
	Track     StoredTrack `json:"track"`
	Climbs    []Climb     `json:"climbs"`
	RiskZones []RiskZone  `json:"risk_zones"`
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
	// CalculateElevation returns the cumulative positive and negative
	// elevation change plus the fraction of the track's path distance
	// that had usable elevation. When coverage falls below
	// CoverageMinThreshold the returned D+/D- are nil and Status is
	// CoverageInsufficient — a zero would be a lie about the data.
	// Algorithm: accumulates per contiguous tramo of valid elevation,
	// resetting at gaps, so a hueco never fabricates the jump across it.
	// Issue #171, A8.
	CalculateElevation(points []Point, threshold float64) ElevationResult
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

type ClimbDetector interface {
	FindAllClimbs(track *Track) ([]Climb, error)
	FindKingClimb(track *Track, climbs []Climb) (*Climb, error)
	FindMuros(track *Track) ([]Muro, error)
	FindRecoveryZones(track *Track, climbs []Climb) ([]RecoveryZone, error)
	FindKmVertical(track *Track) (*KmVerticalResult, error)
}

type RiskZoneDetector interface {
	Detect(track *Track) ([]RiskZone, error)
}

type TrackTypeDetector interface {
	Detect(track *Track) (TrackTypeResult, error)
}

type GPXStore interface {
	Create(ctx context.Context, track *Track, analysis *Analysis) error
	CreateDetail(ctx context.Context, track *Track, analysis *Analysis, climbs []Climb, riskZones []RiskZone, kingClimb *Climb) (*StoredTrackDetail, error)
	// GetByID and GetDetail accept resolution in points-per-detail; 0 or
	// negative falls back to gpx.DefaultResolution (2000). Resolutions
	// smaller than the track length trigger min/max-per-bucket
	// subsampling (issue #170, M9).
	GetByID(ctx context.Context, userID pgtype.UUID, trackID pgtype.UUID, resolution int) (*StoredTrack, error)
	GetDetail(ctx context.Context, userID pgtype.UUID, trackID pgtype.UUID, resolution int) (*StoredTrackDetail, error)
	FindByHash(ctx context.Context, userID pgtype.UUID, fileHash string) (*StoredTrack, error)
	ListClimbs(ctx context.Context, trackID pgtype.UUID) ([]Climb, error)
	ListRiskZones(ctx context.Context, trackID pgtype.UUID) ([]RiskZone, error)
	List(ctx context.Context, userID pgtype.UUID, params ListParams) (*PaginatedTracks, error)
	Delete(ctx context.Context, userID pgtype.UUID, trackID pgtype.UUID) error
}
