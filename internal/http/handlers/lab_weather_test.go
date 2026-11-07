package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/gpx"
	"github.com/fgjcarlos/ghamusinos/internal/weather"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

const weatherTestTrackID = "00000000-0000-0000-0000-0000000000b2"

type weatherFakeGPXStore struct {
	detail *gpx.StoredTrackDetail
	err    error
	calls  int
}

func (s *weatherFakeGPXStore) GetDetail(_ context.Context, _, _ pgtype.UUID, _ int) (*gpx.StoredTrackDetail, error) {
	s.calls++
	return s.detail, s.err
}

type weatherFakeFetcher struct {
	forecast weather.Forecast
	err      error
	fn       func(lat, lon float64, at time.Time) (weather.Forecast, error)
	calls    int
	lats     []float64
	lons     []float64
	ats      []time.Time
}

func (f *weatherFakeFetcher) Forecast(_ context.Context, lat, lon float64, at time.Time) (weather.Forecast, error) {
	f.calls++
	f.lats = append(f.lats, lat)
	f.lons = append(f.lons, lon)
	f.ats = append(f.ats, at)
	if f.fn != nil {
		return f.fn(lat, lon, at)
	}
	return f.forecast, f.err
}

func weatherRequest(authenticated bool, trackID, at string) *http.Request {
	target := "/api/v1/gpx/" + trackID + "/weather"
	if at != "" {
		target += "?at=" + url.QueryEscape(at)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", trackID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	if authenticated {
		req = req.WithContext(auth.WithAuthUser(req.Context(), &auth.User{ID: "00000000-0000-0000-0000-000000000001"}))
	}
	return req
}

func weatherEle(value float64) *float64 { return &value }

// weatherTrack is a six point track whose strategic indices are all
// distinct: start 0, highest 2, midpoint 3 (len/2), end 5.
func weatherTrack() *gpx.StoredTrackDetail {
	return &gpx.StoredTrackDetail{
		Track: gpx.StoredTrack{Track: gpx.Track{Points: []gpx.Point{
			{Lat: 42.00, Lon: -8.00, Ele: weatherEle(100)},
			{Lat: 42.00, Lon: -8.01, Ele: weatherEle(150)},
			{Lat: 42.00, Lon: -8.02, Ele: weatherEle(200)},
			{Lat: 42.00, Lon: -8.03, Ele: weatherEle(120)},
			{Lat: 42.00, Lon: -8.04, Ele: weatherEle(180)},
			{Lat: 42.00, Lon: -8.05, Ele: weatherEle(90)},
		}}},
	}
}

type weatherTestPoint struct {
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

type weatherTestResponse struct {
	TrackID  string             `json:"track_id"`
	At       string             `json:"at"`
	Timezone string             `json:"timezone"`
	Points   []weatherTestPoint `json:"points"`
}

func weatherDecode(t *testing.T, recorder *httptest.ResponseRecorder) weatherTestResponse {
	t.Helper()
	var body weatherTestResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func weatherRoles(points []weatherTestPoint) []string {
	roles := make([]string, len(points))
	for i, point := range points {
		roles[i] = point.Role
	}
	return roles
}

func TestGetGPXWeatherRejectsUnauthenticatedRequest(t *testing.T) {
	weatherResponseCache.reset()
	recorder := httptest.NewRecorder()
	GetGPXWeather(&weatherFakeGPXStore{}, &weatherFakeFetcher{}).ServeHTTP(
		recorder, weatherRequest(false, weatherTestTrackID, "2026-11-08T07:00:00Z"))

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "not authenticated")
}

func TestGetGPXWeatherReturnsNotFoundForUnknownTrack(t *testing.T) {
	weatherResponseCache.reset()
	store := &weatherFakeGPXStore{err: pgx.ErrNoRows}
	recorder := httptest.NewRecorder()
	GetGPXWeather(store, &weatherFakeFetcher{}).ServeHTTP(
		recorder, weatherRequest(true, weatherTestTrackID, "2026-11-08T07:00:00Z"))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "GPX track not found")
}

func TestGetGPXWeatherRejectsInvalidAt(t *testing.T) {
	cases := []struct {
		name string
		at   string
	}{
		{"missing", ""},
		{"not a timestamp", "tomorrow"},
		{"date without time", "2026-11-08"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			weatherResponseCache.reset()
			store := &weatherFakeGPXStore{detail: weatherTrack()}
			fetcher := &weatherFakeFetcher{forecast: weather.Forecast{TemperatureC: 14, WindKmh: 10, WeatherCode: 3}}
			recorder := httptest.NewRecorder()
			GetGPXWeather(store, fetcher).ServeHTTP(recorder, weatherRequest(true, weatherTestTrackID, tc.at))

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Equal(t, 0, fetcher.calls)
		})
	}
}

func TestGetGPXWeatherReturnsFourStrategicPoints(t *testing.T) {
	weatherResponseCache.reset()
	store := &weatherFakeGPXStore{detail: weatherTrack()}
	fetcher := &weatherFakeFetcher{forecast: weather.Forecast{TemperatureC: 14.2, WindKmh: 12, WeatherCode: 3}}
	// 09:30+02:00 is 07:30Z: the response must normalise to UTC.
	requestedAt, err := time.Parse(time.RFC3339, "2026-11-08T09:30:00+02:00")
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	GetGPXWeather(store, fetcher).ServeHTTP(
		recorder, weatherRequest(true, weatherTestTrackID, "2026-11-08T09:30:00+02:00"))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	body := weatherDecode(t, recorder)
	require.Equal(t, weatherTestTrackID, body.TrackID)
	require.Equal(t, "2026-11-08T07:30:00Z", body.At)
	require.Equal(t, "UTC", body.Timezone)
	require.Len(t, body.Points, 4)
	require.Equal(t, []string{"start", "highest", "midpoint", "end"}, weatherRoles(body.Points))

	require.Equal(t, 0.0, body.Points[0].Km)
	for i := 1; i < len(body.Points); i++ {
		require.Greater(t, body.Points[i].Km, body.Points[i-1].Km, "kw must grow with track order")
	}
	// 0.01 degrees of longitude at 42N is ~0.827 km; the highest point is
	// index 2, so its cumulative distance is two steps.
	require.InDelta(t, 1.653, body.Points[1].Km, 0.01)

	// Start (index 0), highest (index 2), midpoint (index 3), end (index 5).
	require.Equal(t, -8.00, body.Points[0].Lon)
	require.Equal(t, 100.0, *body.Points[0].Ele)
	require.Equal(t, -8.02, body.Points[1].Lon)
	require.Equal(t, 200.0, *body.Points[1].Ele)
	require.Equal(t, -8.03, body.Points[2].Lon)
	require.Equal(t, 120.0, *body.Points[2].Ele)
	require.Equal(t, -8.05, body.Points[3].Lon)
	require.Equal(t, 90.0, *body.Points[3].Ele)

	for _, point := range body.Points {
		require.True(t, point.Available)
		require.Equal(t, "2026-11-08T07:00:00Z", point.Time)
		require.InDelta(t, 14.2, point.TemperatureC, 0.0001)
		require.InDelta(t, 12.0, point.WindKmh, 0.0001)
		require.Equal(t, 3, point.WeatherCode)
		require.Equal(t, "Nublado", point.Label)
	}

	require.Equal(t, 4, fetcher.calls)
	for _, gotAt := range fetcher.ats {
		require.True(t, gotAt.Equal(requestedAt), "fetcher must receive the requested instant")
	}
}

func TestGetGPXWeatherDeduplicatesRepeatedStrategicPoints(t *testing.T) {
	weatherResponseCache.reset()
	store := &weatherFakeGPXStore{detail: &gpx.StoredTrackDetail{
		Track: gpx.StoredTrack{Track: gpx.Track{Points: []gpx.Point{
			{Lat: 42.00, Lon: -8.00, Ele: weatherEle(100)},
			{Lat: 42.00, Lon: -8.01, Ele: weatherEle(300)},
		}}},
	}}
	fetcher := &weatherFakeFetcher{forecast: weather.Forecast{TemperatureC: 10, WindKmh: 5, WeatherCode: 0}}

	recorder := httptest.NewRecorder()
	GetGPXWeather(store, fetcher).ServeHTTP(
		recorder, weatherRequest(true, weatherTestTrackID, "2026-11-08T07:00:00Z"))

	require.Equal(t, http.StatusOK, recorder.Code)
	body := weatherDecode(t, recorder)
	// With two points, start=0 and highest=1; midpoint=1 and end=1 are
	// duplicates and must be skipped, keeping the first role that claimed
	// the index.
	require.Equal(t, []string{"start", "highest"}, weatherRoles(body.Points))
	require.Equal(t, 2, fetcher.calls)
}

func TestGetGPXWeatherSkipsUnavailablePoints(t *testing.T) {
	weatherResponseCache.reset()
	store := &weatherFakeGPXStore{detail: weatherTrack()}
	fetcher := &weatherFakeFetcher{fn: func(lat, lon float64, _ time.Time) (weather.Forecast, error) {
		if lon == -8.02 { // highest point only
			return weather.Forecast{}, weather.ErrOutOfRange
		}
		return weather.Forecast{TemperatureC: 9, WindKmh: 3, WeatherCode: 61}, nil
	}}

	recorder := httptest.NewRecorder()
	GetGPXWeather(store, fetcher).ServeHTTP(
		recorder, weatherRequest(true, weatherTestTrackID, "2026-11-08T07:00:00Z"))

	require.Equal(t, http.StatusOK, recorder.Code)
	body := weatherDecode(t, recorder)
	require.Equal(t, []string{"start", "midpoint", "end"}, weatherRoles(body.Points))
	require.Equal(t, "Lluvia", body.Points[0].Label)
}

func TestGetGPXWeatherUnprocessableWhenEveryPointIsOutOfRange(t *testing.T) {
	weatherResponseCache.reset()
	store := &weatherFakeGPXStore{detail: weatherTrack()}
	fetcher := &weatherFakeFetcher{err: weather.ErrOutOfRange}

	recorder := httptest.NewRecorder()
	GetGPXWeather(store, fetcher).ServeHTTP(
		recorder, weatherRequest(true, weatherTestTrackID, "2028-01-01T07:00:00Z"))

	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	require.Contains(t, recorder.Body.String(), "weather forecast not available for the requested date")
	require.Equal(t, 4, fetcher.calls)
}

func TestGetGPXWeatherReturnsServiceUnavailableOnFetchFailure(t *testing.T) {
	weatherResponseCache.reset()
	store := &weatherFakeGPXStore{detail: weatherTrack()}
	fetcher := &weatherFakeFetcher{err: errors.New("open-meteo is down")}

	recorder := httptest.NewRecorder()
	GetGPXWeather(store, fetcher).ServeHTTP(
		recorder, weatherRequest(true, weatherTestTrackID, "2026-11-08T07:00:00Z"))

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "application/problem+json", recorder.Header().Get("Content-Type"))
	require.Contains(t, recorder.Body.String(), "Service Unavailable")
}

func TestGetGPXWeatherCachesSuccessfulResponse(t *testing.T) {
	weatherResponseCache.reset()
	store := &weatherFakeGPXStore{detail: weatherTrack()}
	fetcher := &weatherFakeFetcher{forecast: weather.Forecast{TemperatureC: 11, WindKmh: 6, WeatherCode: 2}}

	first := httptest.NewRecorder()
	GetGPXWeather(store, fetcher).ServeHTTP(
		first, weatherRequest(true, weatherTestTrackID, "2026-11-08T12:00:00Z"))
	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, 4, fetcher.calls)

	second := httptest.NewRecorder()
	GetGPXWeather(store, fetcher).ServeHTTP(
		second, weatherRequest(true, weatherTestTrackID, "2026-11-08T12:00:00Z"))
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, first.Body.String(), second.Body.String())
	require.Equal(t, 4, fetcher.calls, "the second identical request must be served from cache")

	// A different hour is a different cache key and must refetch.
	third := httptest.NewRecorder()
	GetGPXWeather(store, fetcher).ServeHTTP(
		third, weatherRequest(true, weatherTestTrackID, "2026-11-08T13:00:00Z"))
	require.Equal(t, http.StatusOK, third.Code)
	require.Equal(t, 8, fetcher.calls)
}
