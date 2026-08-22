// Package strava contains Strava API clients and webhook handlers.
package strava

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// ActivityEventStore contains the persistence and queue operations used by the
// webhook handler.
type ActivityEventStore interface {
	GetUserIDByAthleteID(ctx context.Context, athleteID int64) (pgtype.UUID, error)
	EnqueueActivityEvent(ctx context.Context, arg sqlc.EnqueueActivityEventParams) (sqlc.ActivityEvent, error)
	EnqueueActivityEventJob(ctx context.Context, eventID string) error
}

// WebhookPayload is the event shape documented by Strava's Webhook Events API.
type WebhookPayload struct {
	ObjectType     string         `json:"object_type"`
	ObjectID       int64          `json:"object_id"`
	AspectType     string         `json:"aspect_type"`
	Updates        map[string]any `json:"updates"`
	OwnerID        int64          `json:"owner_id"`
	SubscriptionID int64          `json:"subscription_id"`
	EventTime      int64          `json:"event_time"`
}

// WebhookHandler receives POST callbacks from Strava. Strava does not sign
// event callbacks; subscription ownership is verified by the GET handshake.
func WebhookHandler(store ActivityEventStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		defer func() { _ = r.Body.Close() }()

		var payload WebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if payload.ObjectType == "" || payload.AspectType == "" || payload.ObjectID == 0 ||
			payload.OwnerID == 0 || payload.SubscriptionID == 0 || payload.EventTime == 0 {
			http.Error(w, "missing required event field", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		userID, err := store.GetUserIDByAthleteID(ctx, payload.OwnerID)
		if err != nil {
			http.Error(w, "failed to resolve athlete", http.StatusInternalServerError)
			return
		}
		if !userID.Valid {
			w.WriteHeader(http.StatusOK)
			return
		}

		event, err := store.EnqueueActivityEvent(ctx, sqlc.EnqueueActivityEventParams{
			UserID:         userID,
			ObjectType:     payload.ObjectType,
			AspectType:     payload.AspectType,
			ObjectID:       payload.ObjectID,
			OwnerID:        pgtype.Int8{Int64: payload.OwnerID, Valid: true},
			SubscriptionID: pgtype.Int8{Int64: payload.SubscriptionID, Valid: true},
			EventTime:      pgtype.Timestamptz{Time: time.Unix(payload.EventTime, 0).UTC(), Valid: true},
			RawPayload:     body,
		})
		if err != nil {
			http.Error(w, "failed to enqueue event", http.StatusInternalServerError)
			return
		}

		if err := store.EnqueueActivityEventJob(ctx, event.ID.String()); err != nil {
			http.Error(w, "failed to enqueue job", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// WebhookChallengeHandler validates Strava's subscription handshake and
// returns the challenge in the JSON shape required by Strava.
func WebhookChallengeHandler(verifyToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		challenge := q.Get("hub.challenge")
		token := q.Get("hub.verify_token")
		mode := q.Get("hub.mode")

		if mode != "subscribe" {
			http.Error(w, "invalid hub.mode", http.StatusBadRequest)
			return
		}

		if token != verifyToken {
			http.Error(w, "invalid hub.verify_token", http.StatusForbidden)
			return
		}

		if challenge == "" {
			http.Error(w, "missing hub.challenge", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"hub.challenge": challenge}); err != nil {
			http.Error(w, "failed to encode challenge", http.StatusInternalServerError)
		}
	}
}
