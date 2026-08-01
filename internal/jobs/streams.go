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
func (w *ImportStravaStreamsWorker) Work(ctx context.Context, job *river.Job[ImportStravaStreamsArgs]) error {
	args := job.Args
	// Parse user ID using pgtype helper
	userID := pgtype.UUID{}
	if err := userID.Scan(args.UserID); err != nil {
		return fmt.Errorf("parse user ID: %w", err)
	}

	// Get activity by external ID
	activity, err := w.locator.GetActivityByExternalID(ctx, sqlc.GetActivityByExternalIDParams{
		UserID:     userID,
		ExternalID: args.StravaActivityID,
	})
	if err != nil {
		return fmt.Errorf("get activity by external ID: %w", err)
	}

	// Get valid access token
	accessToken, err := GetValidToken(ctx, w.querier, w.cipherKey, w.refresher, userID)
	if err != nil {
		return fmt.Errorf("get valid token: %w", err)
	}

	// Fetch streams from Strava
	streamTypes := []string{"heartrate", "watts", "cadence", "altitude", "latlng"}
	streams, err := w.fetcher.GetStreams(ctx, accessToken, args.StravaActivityID, streamTypes)
	if err != nil {
		return fmt.Errorf("fetch streams: %w", err)
	}

	// Upsert each stream
	var hrData []float64
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

		// Collect HR data if present
		if frame.Type == "heartrate" {
			for _, v := range frame.Data {
				if f, ok := v.(float64); ok {
					hrData = append(hrData, f)
				}
			}
		}
	}

	// Calculate and upsert HR zones if hr_max is set
	hrMax, err := w.zoneStore.GetUserHRMaxByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user hr_max: %w", err)
	}

	if hrMax.Valid && hrMax.Int16 > 0 && len(hrData) > 0 {
		zones := calcHRZones(hrData, int(hrMax.Int16))
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

// calcHRZones computes HR zones using Friel/Coggan 5-zone model.
// Zones: z1 <60%, z2 [60%,70%), z3 [70%,80%), z4 [80%,90%), z5 [90%,∞)
// Each sample is assumed to be 1 second.
func calcHRZones(hrStream []float64, hrMax int) HRZoneSeconds {
	if hrMax <= 0 || len(hrStream) == 0 {
		return HRZoneSeconds{}
	}

	zones := HRZoneSeconds{}
	threshold60 := float64(hrMax) * 0.60
	threshold70 := float64(hrMax) * 0.70
	threshold80 := float64(hrMax) * 0.80
	threshold90 := float64(hrMax) * 0.90

	for _, hr := range hrStream {
		if hr < threshold60 {
			zones.Z1++
		} else if hr < threshold70 {
			zones.Z2++
		} else if hr < threshold80 {
			zones.Z3++
		} else if hr < threshold90 {
			zones.Z4++
		} else {
			zones.Z5++
		}
	}

	return zones
}
