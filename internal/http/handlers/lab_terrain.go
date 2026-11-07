package handlers

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/terrain"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// TerrainFetcher is the slice of terrain.Client the handler depends on, so
// tests can inject a fake without touching the network.
type TerrainFetcher interface {
	WaysNear(ctx context.Context, points []terrain.LatLon) ([]terrain.Way, error)
}

const terrainCacheTTL = 24 * time.Hour

// terrainSegmentResponse is the stable JSON contract of the endpoint.
type terrainSegmentResponse struct {
	TrackID   string            `json:"track_id"`
	Available bool              `json:"available"`
	Segments  []terrain.Segment `json:"segments"`
}

type terrainCacheEntry struct {
	response  terrainSegmentResponse
	expiresAt time.Time
}

// terrainResponseCache is an in-memory TTL cache for successful terrain
// responses (including the empty result). It is guarded because the handler
// is shared across concurrent requests.
type terrainResponseCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	entries map[uuid.UUID]terrainCacheEntry
}

func newTerrainResponseCache(ttl time.Duration) *terrainResponseCache {
	return &terrainResponseCache{
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[uuid.UUID]terrainCacheEntry),
	}
}

func (c *terrainResponseCache) get(trackID uuid.UUID) (terrainSegmentResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[trackID]
	if !ok {
		return terrainSegmentResponse{}, false
	}
	if c.now().After(entry.expiresAt) {
		delete(c.entries, trackID)
		return terrainSegmentResponse{}, false
	}
	return entry.response, true
}

func (c *terrainResponseCache) set(trackID uuid.UUID, response terrainSegmentResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[trackID] = terrainCacheEntry{response: response, expiresAt: c.now().Add(c.ttl)}
}

// GetGPXTerrain serves surface/technicality segments for one owned GPX track.
//
// Coordinates come exclusively from the stored track: the caller cannot pass
// a URL or coordinates, which keeps the server-side Overpass call free of any
// SSRF surface and consistent with CSP connect-src 'self'.
func GetGPXTerrain(store GPXDetailStore, fetcher TerrainFetcher) http.Handler {
	cache := newTerrainResponseCache(terrainCacheTTL)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetReqID(r.Context())
		user := auth.AuthUser(r.Context())
		if user == nil {
			WriteProblem(w, NewUnauthorized("not authenticated", requestID))
			return
		}
		parsedID, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			WriteProblem(w, NewNotFound("GPX track not found", requestID))
			return
		}
		trackID := pgtype.UUID{Bytes: parsedID, Valid: true}
		// Ownership check runs before the cache lookup: a cached response
		// must never be served for a track the caller does not own.
		detail, err := store.GetDetail(r.Context(), parseUserID(user.ID), trackID, 0)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				WriteProblem(w, NewNotFound("GPX track not found", requestID))
				return
			}
			WriteProblem(w, NewInternalError("failed to fetch GPX track", requestID))
			return
		}
		if detail == nil {
			WriteProblem(w, NewInternalError("failed to fetch GPX track", requestID))
			return
		}
		if response, ok := cache.get(parsedID); ok {
			writeTerrainResponse(w, response)
			return
		}
		storedPoints := detail.Track.Track.Points
		points := make([]terrain.LatLon, 0, len(storedPoints))
		for _, point := range storedPoints {
			points = append(points, terrain.LatLon{Lat: point.Lat, Lon: point.Lon})
		}
		ways, err := fetcher.WaysNear(r.Context(), points)
		if err != nil {
			WriteProblem(w, NewProblem503("terrain data temporarily unavailable", requestID))
			return
		}
		segments := terrain.MapSegments(points, ways)
		if segments == nil {
			segments = []terrain.Segment{}
		}
		response := terrainSegmentResponse{
			TrackID:   parsedID.String(),
			Available: len(segments) > 0,
			Segments:  segments,
		}
		// Only successful responses are cached; a 503 must be retried.
		cache.set(parsedID, response)
		writeTerrainResponse(w, response)
	})
}

func writeTerrainResponse(w http.ResponseWriter, response terrainSegmentResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	escribirJSON(w, response)
}
