package terrain

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTerrainTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewClient()
	client.endpoint = server.URL
	return client, server
}

func TestTerrainWaysNearSendsAroundQueryForHighways(t *testing.T) {
	var gotMethod, gotUserAgent, gotQuery string
	client, _ := newTerrainTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotUserAgent = r.Header.Get("User-Agent")
		require.NoError(t, r.ParseForm())
		gotQuery = r.PostFormValue("data")
		_, _ = fmt.Fprint(w, `{"elements":[]}`)
	})

	points := []LatLon{{Lat: 40.1, Lon: -3.2}, {Lat: 40.2, Lon: -3.3}}
	ways, err := client.WaysNear(context.Background(), points)

	require.NoError(t, err)
	require.Empty(t, ways)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Contains(t, gotUserAgent, "ghamusinos")
	require.Contains(t, gotQuery, `[out:json][timeout:25]`)
	require.Contains(t, gotQuery, `around:50.0,40.100000,-3.200000,40.200000,-3.300000`)
	require.Contains(t, gotQuery, `["highway"]`)
	require.Contains(t, gotQuery, `out tags geom`)
}

func TestTerrainWaysNearParsesWayGeometryAndIgnoresOtherElements(t *testing.T) {
	client, _ := newTerrainTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"elements":[
			{"type":"way","id":42,"tags":{"highway":"track","surface":"gravel","name":"Camino"},"geometry":[{"lat":40.0,"lon":-3.0},{"lat":40.0001,"lon":-3.0001}]},
			{"type":"node","id":7,"lat":40.0,"lon":-3.0}
		]}`)
	})

	ways, err := client.WaysNear(context.Background(), []LatLon{{Lat: 40.0, Lon: -3.0}})

	require.NoError(t, err)
	require.Len(t, ways, 1)
	require.Equal(t, int64(42), ways[0].ID)
	require.Equal(t, "Camino", ways[0].Name)
	require.Equal(t, "track", ways[0].Highway)
	require.Equal(t, "gravel", ways[0].Surface)
	require.Equal(t, []LatLon{{Lat: 40.0, Lon: -3.0}, {Lat: 40.0001, Lon: -3.0001}}, ways[0].Geometry)
}

func TestTerrainWaysNearReturnsEmptySliceForEmptyElements(t *testing.T) {
	client, _ := newTerrainTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"elements":[]}`)
	})

	ways, err := client.WaysNear(context.Background(), []LatLon{{Lat: 40.0, Lon: -3.0}})

	require.NoError(t, err)
	require.NotNil(t, ways)
	require.Empty(t, ways)
}

func TestTerrainWaysNearFailsOnNon2xxResponse(t *testing.T) {
	client, _ := newTerrainTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = fmt.Fprint(w, `rate limited`)
	})

	_, err := client.WaysNear(context.Background(), []LatLon{{Lat: 40.0, Lon: -3.0}})

	require.Error(t, err)
	require.Contains(t, err.Error(), "429")
}

func TestTerrainWaysNearFailsOnMalformedJSON(t *testing.T) {
	client, _ := newTerrainTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `not json`)
	})

	_, err := client.WaysNear(context.Background(), []LatLon{{Lat: 40.0, Lon: -3.0}})

	require.Error(t, err)
}

func TestTerrainWaysNearFailsWhenContextDeadlineExceeded(t *testing.T) {
	client, _ := newTerrainTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = fmt.Fprint(w, `{"elements":[]}`)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := client.WaysNear(ctx, []LatLon{{Lat: 40.0, Lon: -3.0}})

	require.Error(t, err)
}

func TestTerrainWaysNearDecimatesPolylineToEightyPoints(t *testing.T) {
	var gotQuery string
	client, _ := newTerrainTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		gotQuery = r.PostFormValue("data")
		_, _ = fmt.Fprint(w, `{"elements":[]}`)
	})

	points := make([]LatLon, 0, 500)
	for i := 0; i < 500; i++ {
		points = append(points, LatLon{Lat: 40 + float64(i)/100000, Lon: -3 - float64(i)/100000})
	}
	_, err := client.WaysNear(context.Background(), points)

	require.NoError(t, err)
	coords := gotQuery[strings.Index(gotQuery, "around:50.0,")+len("around:50.0,") : strings.Index(gotQuery, ")[")]
	pairs := strings.Split(coords, ",")
	require.Len(t, pairs, 80*2)
	// Uniform sampling keeps the first and last track points.
	require.Equal(t, "40.000000", pairs[0])
	require.Equal(t, "-3.000000", pairs[1])
	require.Equal(t, "40.004990", pairs[len(pairs)-2])
	require.Equal(t, "-3.004990", pairs[len(pairs)-1])
}

func TestTerrainNewClientUsesOverpassEndpointAndTimeout(t *testing.T) {
	client := NewClient()

	require.Equal(t, "https://overpass-api.de/api/interpreter", client.endpoint)
	require.NotNil(t, client.httpClient)
	require.Equal(t, 25*time.Second, client.httpClient.Timeout)
}
