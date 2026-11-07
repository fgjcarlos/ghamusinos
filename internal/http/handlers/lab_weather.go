package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/fgjcarlos/ghamusinos/internal/weather"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// WeatherFetcher is the slice of weather.Client the handler needs. Defined
// locally so tests can inject a fake and never touch the network.
type WeatherFetcher interface {
	Forecast(ctx context.Context, lat, lon float64, at time.Time) (weather.Forecast, error)
}

const (
	weatherCacheTTL       = time.Hour
	weatherCacheKeyLayout = "2006-01-02T15"
	weatherEarthRadiusKm  = 6371.0088
)

// weatherStrategicPoint is one entry of the response after index selection.
type weatherStrategicPoint struct {
	role  string
	index int
}

type weatherPointResponse struct {
	Role         string   `json:"role"`
	Km           float64  `json:"km"`
	Lat          float64  `json:"lat"`
	Lon          float64  `json:"lon"`
	Ele          *float64 `json:"ele"`
	Time         string   `json:"time"`
	TemperatureC float64  `json:"temperature_c"`
	WindKmh      float64  `json:"wind_kmh"`
	WeatherCode  int      `json:"weather_code"`
	Label        string   `json:"label"`
	Available    bool     `json:"available"`
}

type weatherResponse struct {
	TrackID  string                 `json:"track_id"`
	At       string                 `json:"at"`
	Timezone string                 `json:"timezone"`
	Points   []weatherPointResponse `json:"points"`
}

type weatherCacheEntry struct {
	body      []byte
	expiresAt time.Time
}

// weatherResponseCache holds serialised 200 responses for one hour. The key
// is "track id + UTC hour", so the same track and race hour share a single
// upstream call chain.
type weatherResponseCacheStore struct {
	mu      sync.Mutex
	entries map[string]weatherCacheEntry
}

var weatherResponseCache = &weatherResponseCacheStore{entries: make(map[string]weatherCacheEntry)}

func (c *weatherResponseCacheStore) get(key string, now time.Time) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !now.Before(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return entry.body, true
}

func (c *weatherResponseCacheStore) put(key string, body []byte, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Opportunistic prune: entries are cheap and the map stays small, but
	// expired bodies should not survive longer than their TTL.
	for cacheKey, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, cacheKey)
		}
	}
	c.entries[key] = weatherCacheEntry{body: body, expiresAt: now.Add(weatherCacheTTL)}
}

// reset empties the cache. Only tests use it; production never needs an
// explicit flush because entries expire on read.
func (c *weatherResponseCacheStore) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]weatherCacheEntry)
}

// GetGPXWeather returns the Open-Meteo forecast at four strategic points of
// one of the authenticated user's tracks: start, highest point, midpoint and
// end. Query param `at` is the race instant in RFC3339; it is normalised to
// UTC. Coordinates always come from the stored track — the client cannot
// point the backend at an arbitrary location.
func GetGPXWeather(store GPXDetailStore, fetcher WeatherFetcher) http.Handler {
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

		rawAt := r.URL.Query().Get("at")
		at, err := time.Parse(time.RFC3339, rawAt)
		if err != nil {
			WriteProblem(w, NewBadRequest("at must be an RFC3339 timestamp", requestID))
			return
		}
		at = at.UTC()

		trackID := pgtype.UUID{Bytes: parsedID, Valid: true}
		detail, err := store.GetDetail(r.Context(), parseUserID(user.ID), trackID, 0)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				WriteProblem(w, NewNotFound("GPX track not found", requestID))
				return
			}
			WriteProblem(w, NewInternalError("failed to fetch GPX track", requestID))
			return
		}

		points := detail.Track.Track.Points
		strategic := weatherStrategicPoints(points)
		if len(strategic) == 0 {
			WriteProblem(w, NewUnprocessableEntity("GPX track has no points", requestID))
			return
		}

		cacheKey := parsedID.String() + at.Format(weatherCacheKeyLayout)
		if cached, ok := weatherResponseCache.get(cacheKey, time.Now()); ok {
			writeWeatherJSON(w, cached)
			return
		}

		cumulativeKm := weatherCumulativeKm(points)
		response := weatherResponse{
			TrackID:  parsedID.String(),
			At:       at.Format(time.RFC3339),
			Timezone: "UTC",
			Points:   make([]weatherPointResponse, 0, len(strategic)),
		}
		for _, candidate := range strategic {
			point := points[candidate.index]
			forecast, err := fetcher.Forecast(r.Context(), point.Lat, point.Lon, at)
			if err != nil {
				if errors.Is(err, weather.ErrOutOfRange) {
					continue
				}
				WriteProblem(w, newWeatherUnavailable("weather forecast unavailable", requestID))
				return
			}
			response.Points = append(response.Points, weatherPointResponse{
				Role:         candidate.role,
				Km:           math.Round(cumulativeKm[candidate.index]*1000) / 1000,
				Lat:          point.Lat,
				Lon:          point.Lon,
				Ele:          point.Ele,
				Time:         at.Truncate(time.Hour).Format(time.RFC3339),
				TemperatureC: forecast.TemperatureC,
				WindKmh:      forecast.WindKmh,
				WeatherCode:  forecast.WeatherCode,
				Label:        weather.WeatherLabel(forecast.WeatherCode),
				Available:    true,
			})
		}

		if len(response.Points) == 0 {
			WriteProblem(w, NewUnprocessableEntity(
				"weather forecast not available for the requested date", requestID))
			return
		}

		body, err := json.Marshal(response)
		if err != nil {
			WriteProblem(w, NewInternalError("failed to encode weather response", requestID))
			return
		}
		weatherResponseCache.put(cacheKey, body, time.Now())
		writeWeatherJSON(w, body)
	})
}

// weatherStrategicPoints picks start, highest, midpoint and end, in that
// order. Repeated indices keep the first role that claimed them, so a short
// track never reports the same point twice.
func weatherStrategicPoints(points []gpx.Point) []weatherStrategicPoint {
	if len(points) == 0 {
		return nil
	}
	candidates := []weatherStrategicPoint{
		{role: "start", index: 0},
		{role: "highest", index: weatherHighestIndex(points)},
		{role: "midpoint", index: len(points) / 2},
		{role: "end", index: len(points) - 1},
	}
	chosen := make([]weatherStrategicPoint, 0, len(candidates))
	seen := make(map[int]bool, len(candidates))
	for _, candidate := range candidates {
		if candidate.index < 0 || seen[candidate.index] {
			continue
		}
		seen[candidate.index] = true
		chosen = append(chosen, candidate)
	}
	return chosen
}

// weatherHighestIndex returns the index of the highest point, ignoring
// points without elevation. It returns -1 when the track has no elevation
// data at all; the highest role is then skipped.
func weatherHighestIndex(points []gpx.Point) int {
	best := -1
	var bestEle float64
	for i, point := range points {
		if point.Ele == nil {
			continue
		}
		if best == -1 || *point.Ele > bestEle {
			best, bestEle = i, *point.Ele
		}
	}
	return best
}

// weatherCumulativeKm returns the cumulative haversine distance in km at
// every point of the track.
func weatherCumulativeKm(points []gpx.Point) []float64 {
	km := make([]float64, len(points))
	for i := 1; i < len(points); i++ {
		km[i] = km[i-1] + weatherHaversineKm(points[i-1], points[i])
	}
	return km
}

func weatherHaversineKm(a, b gpx.Point) float64 {
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLon := (b.Lon - a.Lon) * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * weatherEarthRadiusKm * math.Asin(math.Sqrt(h))
}

func writeWeatherJSON(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // headers are already sent; nothing left to recover.
	w.Write(body)
}

// newWeatherUnavailable is the 503 used when the upstream forecast call
// fails for a reason other than "outside the forecast horizon". The section
// degrades; the track detail itself keeps working.
func newWeatherUnavailable(detail, instance string) ProblemDetail {
	return ProblemDetail{
		Type:     "about:blank",
		Title:    "Service Unavailable",
		Status:   http.StatusServiceUnavailable,
		Detail:   detail,
		Instance: instance,
	}
}
