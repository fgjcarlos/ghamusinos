package weather

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newWeatherTestClient wires a Client against a local httptest server so
// no test ever reaches Open-Meteo.
func newWeatherTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return newClient(server.Client(), server.URL)
}

func TestWeatherClientForecastSelectsTheMatchingUTCHour(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	client := newWeatherTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"hourly": {
				"time": ["2026-11-08T06:00", "2026-11-08T07:00", "2026-11-08T08:00"],
				"temperature_2m": [8.5, 14.25, 16.0],
				"weathercode": [1, 3, 61],
				"windspeed_10m": [4.0, 12.5, 20.0]
			}
		}`))
	})

	// 09:45+02:00 is 07:45Z, so the 07:00Z hourly bucket wins.
	at := time.Date(2026, 11, 8, 9, 45, 0, 0, time.FixedZone("CET", 2*60*60))
	forecast, err := client.Forecast(context.Background(), 42.1, -8.1, at)
	require.NoError(t, err)
	require.Equal(t, Forecast{TemperatureC: 14.25, WindKmh: 12.5, WeatherCode: 3}, forecast)

	require.Equal(t, "/v1/forecast", gotPath)
	require.Equal(t, "42.1", gotQuery.Get("latitude"))
	require.Equal(t, "-8.1", gotQuery.Get("longitude"))
	require.Equal(t, "temperature_2m,weathercode,windspeed_10m", gotQuery.Get("hourly"))
	require.Equal(t, "16", gotQuery.Get("forecast_days"))
	require.Equal(t, "UTC", gotQuery.Get("timezone"))
}

func TestWeatherClientForecastReturnsErrOutOfRangeWhenHourIsMissing(t *testing.T) {
	client := newWeatherTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"hourly": {
				"time": ["2026-11-08T06:00", "2026-11-08T07:00"],
				"temperature_2m": [8.5, 9.5],
				"weathercode": [0, 1],
				"windspeed_10m": [4.0, 5.0]
			}
		}`))
	})

	_, err := client.Forecast(context.Background(), 42.1, -8.1, time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC))
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrOutOfRange), "want errors.Is(err, ErrOutOfRange), got %v", err)
}

func TestWeatherClientForecastRejectsNon2xxResponses(t *testing.T) {
	client := newWeatherTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":true,"reason":"internal"}`))
	})

	_, err := client.Forecast(context.Background(), 42.1, -8.1, time.Date(2026, 11, 8, 7, 0, 0, 0, time.UTC))
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrOutOfRange))
}

func TestWeatherClientForecastRejectsMalformedBodies(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"invalid json", `{"hourly":`},
		{"short temperature series", `{"hourly":{"time":["2026-11-08T07:00"],"temperature_2m":[],"weathercode":[3],"windspeed_10m":[12]}}`},
		{"short weathercode series", `{"hourly":{"time":["2026-11-08T07:00"],"temperature_2m":[14],"weathercode":[],"windspeed_10m":[12]}}`},
		{"short windspeed series", `{"hourly":{"time":["2026-11-08T07:00"],"temperature_2m":[14],"weathercode":[3],"windspeed_10m":[]}}`},
		{"unparsable hour", `{"hourly":{"time":["not-an-hour"],"temperature_2m":[14],"weathercode":[3],"windspeed_10m":[12]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newWeatherTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			})
			_, err := client.Forecast(context.Background(), 42.1, -8.1, time.Date(2026, 11, 8, 7, 0, 0, 0, time.UTC))
			require.Error(t, err)
			require.False(t, errors.Is(err, ErrOutOfRange))
		})
	}
}

func TestWeatherLabel(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "Despejado"},
		{1, "Mayormente despejado"},
		{2, "Parcialmente nublado"},
		{3, "Nublado"},
		{45, "Niebla"},
		{48, "Niebla"},
		{51, "Llovizna"},
		{53, "Llovizna"},
		{55, "Llovizna"},
		{61, "Lluvia"},
		{63, "Lluvia"},
		{65, "Lluvia"},
		{71, "Nieve"},
		{73, "Nieve"},
		{75, "Nieve"},
		{77, "Nieve"},
		{80, "Chubascos"},
		{81, "Chubascos"},
		{82, "Chubascos"},
		{85, "Chubascos de nieve"},
		{86, "Chubascos de nieve"},
		{95, "Tormenta"},
		{96, "Tormenta"},
		{99, "Tormenta"},
		{4, "Desconocido"},
		{47, "Desconocido"},
		{60, "Desconocido"},
		{70, "Desconocido"},
		{83, "Desconocido"},
		{90, "Desconocido"},
		{100, "Desconocido"},
		{-1, "Desconocido"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, WeatherLabel(tc.code), "code %d", tc.code)
	}
}
