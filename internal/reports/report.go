// Package reports builds the unified training report snapshots (week, month and
// training block) and persists them idempotently.
package reports

import "github.com/google/uuid"

// SchemaVersion is the version of the payload documented in
// docs/architecture/report-snapshot.md. It lives inside the payload because the
// table has a single payload column.
const SchemaVersion = 1

// Kind is the period type of a report.
type Kind string

const (
	KindWeek  Kind = "week"
	KindMonth Kind = "month"
	KindBlock Kind = "block"
)

// Snapshot is the version 1 payload of a report.
type Snapshot struct {
	SchemaVersion int         `json:"schema_version"`
	Kind          Kind        `json:"kind"`
	PeriodStart   string      `json:"period_start"`
	PeriodEnd     string      `json:"period_end"`
	Days          int         `json:"days"`
	Block         *BlockRef   `json:"block"`
	Race          *RaceRef    `json:"race"`
	Load          Load        `json:"load"`
	Volume        Volume      `json:"volume"`
	Performance   Performance `json:"performance"`
	Plan          Plan        `json:"plan"`
	Previous      Previous    `json:"previous"`
}

// BlockRef identifies the training block of a block report.
type BlockRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// RaceRef identifies the target race of a block, with the days left from Now.
type RaceRef struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	RaceDate   string    `json:"race_date"`
	Priority   string    `json:"priority"`
	DaysToRace int       `json:"days_to_race"`
}

// Load is the training load of the period: total TSS, the zero-filled daily
// series and CTL/ATL/TSB at the close of the day before the period and of its
// last day.
type Load struct {
	TSS      float64     `json:"tss"`
	Daily    []DailyLoad `json:"daily"`
	CTLStart float64     `json:"ctl_start"`
	ATLStart float64     `json:"atl_start"`
	TSBStart float64     `json:"tsb_start"`
	CTLEnd   float64     `json:"ctl_end"`
	ATLEnd   float64     `json:"atl_end"`
	TSBEnd   float64     `json:"tsb_end"`
}

// DailyLoad is one day of the period; days without load are present with zero.
type DailyLoad struct {
	Date          string  `json:"date"`
	TSS           float64 `json:"tss"`
	ActivityCount int     `json:"activity_count"`
}

// Volume is the training volume of the activities that started in the period.
type Volume struct {
	Activities     int     `json:"activities"`
	DistanceM      float64 `json:"distance_m"`
	ElevationGainM float64 `json:"elevation_gain_m"`
	MovingSeconds  int64   `json:"moving_seconds"`
}

// Performance is orientative: activity averages are not stored, so the report
// recomputes them. An average is null when the period has no samples.
type Performance struct {
	AvgEF          *float64 `json:"avg_ef"`
	EFSamples      int      `json:"ef_samples"`
	AvgGAPSecPerKm *float64 `json:"avg_gap_sec_per_km"`
	GAPSamples     int      `json:"gap_samples"`
}

// Plan is the compliance of the planned sessions of the period.
type Plan struct {
	Planned   int      `json:"planned"`
	Completed int      `json:"completed"`
	Skipped   int      `json:"skipped"`
	Modified  int      `json:"modified"`
	Adherence *float64 `json:"adherence"`
}

// Previous is the comparison period (the previous calendar month, or the same
// number of days right before the period) and its main numbers.
type Previous struct {
	PeriodStart    string   `json:"period_start"`
	PeriodEnd      string   `json:"period_end"`
	Activities     int      `json:"activities"`
	TSS            float64  `json:"tss"`
	DistanceM      float64  `json:"distance_m"`
	ElevationGainM float64  `json:"elevation_gain_m"`
	AvgEF          *float64 `json:"avg_ef"`
	AvgGAPSecPerKm *float64 `json:"avg_gap_sec_per_km"`
}
