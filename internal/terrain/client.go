// Package terrain derives surface type and technicality for a stored GPX
// track from OpenStreetMap data served by the Overpass API.
//
// The SPA is served with CSP connect-src 'self', so the browser never talks
// to Overpass directly: this package runs server-side and is fed only by
// coordinates the caller already owns.
package terrain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultOverpassEndpoint = "https://overpass-api.de/api/interpreter"
	overpassTimeout         = 25 * time.Second
	// overpassUserAgent is required by the Overpass usage policy: anonymous
	// clients without a descriptive agent may be blocked.
	overpassUserAgent = "ghamusinos/1.0 (+https://github.com/fgjcarlos/ghamusinos)"
	// searchRadiusM is the around: radius used to filter ways server-side.
	searchRadiusM = 50.0
	// maxQueryPoints caps the decimated polyline so the GET-style way()
	// filter stays inside Overpass URL/query limits.
	maxQueryPoints = 80
	// maxResponseBytes bounds how much of a third-party response we buffer.
	maxResponseBytes = 32 << 20
)

// LatLon is a WGS84 coordinate in degrees.
type LatLon struct {
	Lat float64
	Lon float64
}

// Way is one OSM way with its inline geometry and the tags we consume.
type Way struct {
	ID       int64
	Name     string
	Highway  string
	Surface  string
	Geometry []LatLon
}

// Client queries Overpass. It is safe for concurrent use.
type Client struct {
	httpClient *http.Client
	endpoint   string
}

// NewClient builds a client with the 25 s Overpass budget and the public
// Overpass endpoint.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: overpassTimeout},
		endpoint:   defaultOverpassEndpoint,
	}
}

// WaysNear returns the OSM ways tagged with highway within 50 m of the given
// polyline. The polyline is decimated to at most 80 uniformly sampled points
// before it is sent, so long tracks do not produce oversized queries.
func (c *Client) WaysNear(ctx context.Context, points []LatLon) ([]Way, error) {
	if len(points) == 0 {
		return []Way{}, nil
	}
	query := buildOverpassQuery(decimatePoints(points, maxQueryPoints))
	form := url.Values{}
	form.Set("data", query)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("terrain: build overpass request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", overpassUserAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("terrain: call overpass: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// Drain a small slice so the connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("terrain: overpass returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("terrain: read overpass response: %w", err)
	}
	var payload overpassResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("terrain: decode overpass response: %w", err)
	}
	ways := make([]Way, 0, len(payload.Elements))
	for _, element := range payload.Elements {
		if element.Type != "way" || len(element.Geometry) == 0 {
			continue
		}
		geometry := make([]LatLon, 0, len(element.Geometry))
		for _, node := range element.Geometry {
			geometry = append(geometry, LatLon(node))
		}
		ways = append(ways, Way{
			ID:       element.ID,
			Name:     element.Tags["name"],
			Highway:  element.Tags["highway"],
			Surface:  element.Tags["surface"],
			Geometry: geometry,
		})
	}
	return ways, nil
}

type overpassResponse struct {
	Elements []overpassElement `json:"elements"`
}

type overpassElement struct {
	Type     string            `json:"type"`
	ID       int64             `json:"id"`
	Tags     map[string]string `json:"tags"`
	Geometry []overpassNode    `json:"geometry"`
}

type overpassNode struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// buildOverpassQuery renders the around: query described in the feature spec.
func buildOverpassQuery(points []LatLon) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[out:json][timeout:25];way(around:%.1f", searchRadiusM)
	for _, point := range points {
		fmt.Fprintf(&b, ",%.6f,%.6f", point.Lat, point.Lon)
	}
	b.WriteString(`)["highway"];out tags geom;`)
	return b.String()
}

// decimatePoints picks at most maxPoints uniformly spaced points, keeping the
// first and the last one.
func decimatePoints(points []LatLon, maxPoints int) []LatLon {
	if maxPoints <= 1 || len(points) <= maxPoints {
		return points
	}
	decimated := make([]LatLon, 0, maxPoints)
	step := float64(len(points)-1) / float64(maxPoints-1)
	for i := 0; i < maxPoints; i++ {
		index := int(math.Round(float64(i) * step))
		if index >= len(points) {
			index = len(points) - 1
		}
		decimated = append(decimated, points[index])
	}
	return decimated
}
