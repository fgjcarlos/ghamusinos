package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// StreamFetcher defines the contract for fetching streams from Strava.
type StreamFetcher interface {
	GetStreams(ctx context.Context, accessToken string, activityID int64, types []string) ([]strava.StreamFrame, error)
}

// StreamInserter defines the contract for upserting activity streams.
type StreamInserter interface {
	UpsertActivityStream(ctx context.Context, arg sqlc.UpsertActivityStreamParams) (sqlc.ActivityStream, error)
}

// HRZoneStore defines the contract for HR zone operations.
type HRZoneStore interface {
	GetUserHRMaxByID(ctx context.Context, userID pgtype.UUID) (pgtype.Int2, error)
	UpsertHRZones(ctx context.Context, arg sqlc.UpsertHRZonesParams) (sqlc.HrZone, error)
}

// ActivityLocator defines the contract for locating activities by external ID.
type ActivityLocator interface {
	GetActivityByExternalID(ctx context.Context, arg sqlc.GetActivityByExternalIDParams) (sqlc.Activity, error)
}

// Work implements the River worker interface for ImportStravaStreamsWorker.
// The struct is defined in workers.go for registration with River.
//
// AUD-05 (issue #166): the previous version looked up the activity by
// (userID, externalID) without specifying external_source, so the SQL query
// (which has external_source = $2 in the WHERE) never matched and zero
// streams ever landed in the database. The fix adds ExternalSource: "strava"
// to the lookup.
func (w *ImportStravaStreamsWorker) Work(ctx context.Context, job *river.Job[ImportStravaStreamsArgs]) error {
	args := job.Args
	// Parse user ID using pgtype helper
	userID := pgtype.UUID{}
	if err := userID.Scan(args.UserID); err != nil {
		return fmt.Errorf("parse user ID: %w", err)
	}

	// Get activity by external ID. AUD-05 (issue #166): the query
	// (queries/activities.sql:1-9) requires external_source in the WHERE
	// clause; passing an empty string here meant the row never matched and
	// every Strava stream was effectively dropped. Strava activities are
	// always tagged with source="strava" in this codebase.
	activity, err := w.locator.GetActivityByExternalID(ctx, sqlc.GetActivityByExternalIDParams{
		UserID:         userID,
		ExternalSource: "strava",
		ExternalID:     args.StravaActivityID,
	})
	if err != nil {
		return fmt.Errorf("get activity by external ID: %w", err)
	}

	// Get valid access token
	accessToken, err := GetValidToken(ctx, w.querier, w.cipherKey, w.refresher, userID)
	if err != nil {
		return fmt.Errorf("get valid token: %w", err)
	}

	// Fetch streams from Strava. We ask for the high-resolution time-based
	// streams; the API returns them with Resolution = seconds per sample.
	// With Resolution = 1 each sample is 1s; with higher values the data
	// has been downsampled (rare for HR, common for latlng).
	streamTypes := []string{"heartrate", "watts", "cadence", "altitude", "latlng"}
	streams, err := w.fetcher.GetStreams(ctx, accessToken, args.StravaActivityID, streamTypes)
	if err != nil {
		return fmt.Errorf("fetch streams: %w", err)
	}

	// Upsert each stream
	for _, frame := range streams {
		// Convert stream data to JSON
		dataJSON, err := json.Marshal(frame.Data)
		if err != nil {
			return fmt.Errorf("marshal stream data: %w", err)
		}

		// Upsert stream
		_, err = w.inserter.UpsertActivityStream(ctx, sqlc.UpsertActivityStreamParams{
			ActivityID: activity.ID,
			StreamType: frame.Type,
			Data:       dataJSON,
		})
		if err != nil {
			return fmt.Errorf("upsert stream: %w", err)
		}
	}

	// Build the HR sample list from the frames we already fetched. AUD-05
	// (issue #166): calcHRZones counts samples as 1s each, which is wrong
	// when the Strava API down-samples (Resolution > 1). We multiply by
	// Resolution so zone seconds reflect real durations. Resolution
	// defaults to 1 second-per-sample when the API returns 0 (Strava's
	// convention for "1 Hz"). When the frame is nil we leave Resolution
	// at its zero value (also 1) — no HR stream means no HR zones anyway.
	var hrFrame *strava.StreamFrame
	for i := range streams {
		if streams[i].Type == "heartrate" {
			hrFrame = &streams[i]
			break
		}
	}
	hrData := hrSamplesFromFrame(hrFrame)
	secondsPerSample := 1
	if hrFrame != nil && hrFrame.Resolution > 0 {
		secondsPerSample = hrFrame.Resolution
	}

	// Calculate and upsert HR zones if hr_max is set
	hrMax, err := w.zoneStore.GetUserHRMaxByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user hr_max: %w", err)
	}

	if hrMax.Valid && hrMax.Int16 > 0 && len(hrData) > 0 {
		zones := calcHRZonesScaled(hrData, int(hrMax.Int16), secondsPerSample)
		_, err = w.zoneStore.UpsertHRZones(ctx, sqlc.UpsertHRZonesParams{
			ActivityID: activity.ID,
			Z1Seconds:  int32(zones.Z1),
			Z2Seconds:  int32(zones.Z2),
			Z3Seconds:  int32(zones.Z3),
			Z4Seconds:  int32(zones.Z4),
			Z5Seconds:  int32(zones.Z5),
			ComputedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
		if err != nil {
			return fmt.Errorf("upsert hr zones: %w", err)
		}
	}

	return nil
}

// HRZoneSeconds holds the time spent in each HR zone.
type HRZoneSeconds struct {
	Z1 int
	Z2 int
	Z3 int
	Z4 int
	Z5 int
}

// hrSamplesFromFrame flattens a StreamFrame's Data ([]interface{}) into a
// []float64 of HR values. If the frame is nil or empty the result is nil.
//
// AUD-05 (issue #166): the returned slice intentionally does NOT multiply
// by Resolution. calcHRZones takes a secondsPerSample parameter so the
// same scaling path applies to any future time-based stream (watts, cadence).
func hrSamplesFromFrame(frame *strava.StreamFrame) []float64 {
	if frame == nil || len(frame.Data) == 0 {
		return nil
	}
	out := make([]float64, 0, len(frame.Data))
	for _, v := range frame.Data {
		if f, ok := v.(float64); ok {
			out = append(out, f)
		}
	}
	return out
}

// calcHRZonesSecondsPerSample is the number of seconds each HR sample
// represents. Strava returns streams with Resolution = seconds-per-sample
// (Resolution = 1 means 1Hz).
//
// Zone thresholds: 60/70/80/90 % of hrMax. The schema comment for hr_zones
// used to say "50/60/70/80/90" but the original code used 60/70/80/90, so
// AUD-05 keeps the code and updates the schema comment (see migration
// 00004_strava_activities.sql — the "Calculadas a partir de streams HR"
// block). Z1 starts at the lowest threshold so the 5-zone shape is preserved.
func calcHRZones(hrStream []float64, hrMax int) HRZoneSeconds {
	return calcHRZonesScaled(hrStream, hrMax, 1)
}

// calcHRZonesScaled is the workhorse; passing secondsPerSample > 1 handles
// Strava's down-sampled streams (Resolution > 1). Each sample contributes
// secondsPerSample seconds to the matching zone.
//
// AUD-05 AC: "Las zonas de FC de un stream con resolution: 'medium' dan los
// mismos segundos que el mismo esfuerzo en high". The medium Resolution
// doubles the sample interval, so each sample represents more seconds; the
// math must reflect that or the totals will be wrong by the down-sampling
// factor.
func calcHRZonesScaled(hrStream []float64, hrMax int, secondsPerSample int) HRZoneSeconds {
	if hrMax <= 0 || len(hrStream) == 0 {
		return HRZoneSeconds{}
	}
	if secondsPerSample < 1 {
		secondsPerSample = 1
	}

	zones := HRZoneSeconds{}
	threshold60 := float64(hrMax) * 0.60
	threshold70 := float64(hrMax) * 0.70
	threshold80 := float64(hrMax) * 0.80
	threshold90 := float64(hrMax) * 0.90

	for _, hr := range hrStream {
		switch {
		case hr < threshold60:
			zones.Z1 += secondsPerSample
		case hr < threshold70:
			zones.Z2 += secondsPerSample
		case hr < threshold80:
			zones.Z3 += secondsPerSample
		case hr < threshold90:
			zones.Z4 += secondsPerSample
		default:
			zones.Z5 += secondsPerSample
		}
	}

	return zones
}
