// Package weather is a small Open-Meteo client for the GPX race lab.
//
// The SPA is served with a `connect-src 'self'` Content Security Policy, so
// the browser can never call Open-Meteo directly. The Go backend owns that
// call and derives the coordinates from the authenticated user's stored
// track.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ErrOutOfRange reports that the requested instant has no hourly entry in
// the Open-Meteo response. Callers use errors.Is to tell a forecast that
// simply does not exist yet (or no longer exists) apart from a transport or
// parsing failure.
var ErrOutOfRange = errors.New("weather: requested hour is outside the forecast range")

const (
	defaultBaseURL = "https://api.open-meteo.com"
	requestTimeout = 10 * time.Second
	// maxResponseBytes caps how much of the response we are willing to
	// read. A 16 day hourly payload is well under 100 KiB.
	maxResponseBytes = 1 << 20
	hourlyTimeLayout = "2006-01-02T15:04"
)

// Forecast is the slice of an Open-Meteo hourly entry this app needs.
type Forecast struct {
	TemperatureC float64
	WindKmh      float64
	WeatherCode  int
}

// Client talks to the Open-Meteo forecast API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient returns a Client with a 10s HTTP timeout pointed at the public
// Open-Meteo endpoint.
func NewClient() *Client {
	return newClient(&http.Client{Timeout: requestTimeout}, defaultBaseURL)
}

// newClient is the seam tests use to point the client at httptest servers.
func newClient(httpClient *http.Client, baseURL string) *Client {
	return &Client{httpClient: httpClient, baseURL: baseURL}
}

// Forecast fetches the hourly forecast for the given coordinates and returns
// the entry that matches at, truncated to the hour in UTC.
func (c *Client) Forecast(ctx context.Context, lat, lon float64, at time.Time) (Forecast, error) {
	endpoint, err := url.Parse(c.baseURL + "/v1/forecast")
	if err != nil {
		return Forecast{}, fmt.Errorf("weather: build endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("latitude", strconv.FormatFloat(lat, 'f', -1, 64))
	query.Set("longitude", strconv.FormatFloat(lon, 'f', -1, 64))
	query.Set("hourly", "temperature_2m,weathercode,windspeed_10m")
	query.Set("forecast_days", "16")
	query.Set("timezone", "UTC")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Forecast{}, fmt.Errorf("weather: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Forecast{}, fmt.Errorf("weather: request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Forecast{}, fmt.Errorf("weather: unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Hourly struct {
			Time          []string  `json:"time"`
			Temperature2m []float64 `json:"temperature_2m"`
			WeatherCode   []int     `json:"weathercode"`
			WindSpeed10m  []float64 `json:"windspeed_10m"`
		} `json:"hourly"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&payload); err != nil {
		return Forecast{}, fmt.Errorf("weather: decode response: %w", err)
	}

	hourly := payload.Hourly
	if len(hourly.Time) == 0 {
		return Forecast{}, errors.New("weather: response has no hourly entries")
	}
	if len(hourly.Temperature2m) != len(hourly.Time) ||
		len(hourly.WeatherCode) != len(hourly.Time) ||
		len(hourly.WindSpeed10m) != len(hourly.Time) {
		return Forecast{}, errors.New("weather: response hourly series have mismatched lengths")
	}

	target := at.UTC().Truncate(time.Hour)
	for i, raw := range hourly.Time {
		hour, err := parseHourlyTime(raw)
		if err != nil {
			return Forecast{}, fmt.Errorf("weather: parse hourly time %q: %w", raw, err)
		}
		if !hour.Equal(target) {
			continue
		}
		return Forecast{
			TemperatureC: hourly.Temperature2m[i],
			WindKmh:      hourly.WindSpeed10m[i],
			WeatherCode:  hourly.WeatherCode[i],
		}, nil
	}

	return Forecast{}, ErrOutOfRange
}

// parseHourlyTime accepts both Open-Meteo's minute-precision timestamps
// ("2026-11-08T07:00", no zone because timezone=UTC) and RFC3339.
func parseHourlyTime(raw string) (time.Time, error) {
	if parsed, err := time.Parse(hourlyTimeLayout, raw); err == nil {
		return parsed.UTC(), nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unsupported hour format")
}

// WeatherLabel maps an Open-Meteo WMO weather code to the Spanish label the
// UI shows. Unknown codes degrade to "Desconocido"; they are never an error.
func WeatherLabel(code int) string {
	switch {
	case code == 0:
		return "Despejado"
	case code == 1:
		return "Mayormente despejado"
	case code == 2:
		return "Parcialmente nublado"
	case code == 3:
		return "Nublado"
	case code == 45 || code == 48:
		return "Niebla"
	case code >= 51 && code <= 55:
		return "Llovizna"
	case code >= 61 && code <= 65:
		return "Lluvia"
	case code >= 71 && code <= 77:
		return "Nieve"
	case code >= 80 && code <= 82:
		return "Chubascos"
	case code >= 85 && code <= 86:
		return "Chubascos de nieve"
	case code >= 95 && code <= 99:
		return "Tormenta"
	default:
		return "Desconocido"
	}
}
