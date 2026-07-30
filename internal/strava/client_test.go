package strava

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)

	// Sobreescribimos las URLs del paquete apuntando al server de tests
	// mediante NewClient + una Config que sabe a dónde ir. Para no
	// contaminar el paquete con "URLs configurables" (un ricket para
	// tests es preferible a un setter público), usamos un transport
	// que reescribe los hosts.
	cfg := Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost/callback",
		Scopes:       "read,activity:read",
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &rewriteTransport{
				fromTo: map[string]string{
					"www.strava.com": strings.TrimPrefix(srv.URL, "http://"),
				},
				base: http.DefaultTransport,
			},
		},
	}
	c, err := NewClient(cfg)
	if err != nil {
		srv.Close()
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv
}

// rewriteTransport es un http.RoundTripper que reescribe el Host del
// request para apuntar a un servidor httptest local. Es el truco estándar
// para testear código que tiene URLs hardcodeadas; lo hacemos interno al
// paquete para no exponer APIs solo-para-tests.
type rewriteTransport struct {
	fromTo map[string]string
	base   http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if newHost, ok := t.fromTo[req.URL.Host]; ok {
		req.URL.Host = newHost
		req.URL.Scheme = "http"
	}
	return t.base.RoundTrip(req)
}

// ─────────────────────────────────────────────────────────────────────────
// Client tests
// ─────────────────────────────────────────────────────────────────────────

func TestNewClient_RequiresCredentials(t *testing.T) {
	for _, tc := range []Config{
		{},
		{ClientID: "x"},
		{ClientSecret: "y"},
	} {
		if _, err := NewClient(tc); err == nil {
			t.Errorf("NewClient(%+v) sin credenciales: esperaba error", tc)
		}
	}
}

func TestAuthorizeURL(t *testing.T) {
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("AuthorizeURL no debe llamar al API: %s", r.URL.Path)
	}))
	defer srv.Close()

	got := c.AuthorizeURL("state-xyz")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("AuthorizeURL no parsea: %v", err)
	}
	if u.Host != "www.strava.com" || u.Path != "/oauth/authorize" {
		t.Errorf("AuthorizeURL host/path = %s%s, want www.strava.com/oauth/authorize", u.Host, u.Path)
	}
	q := u.Query()
	want := map[string]string{
		"client_id":       "test-client-id",
		"response_type":   "code",
		"redirect_uri":    "http://localhost/callback",
		"approval_prompt": "auto",
		"scope":           "read,activity:read",
		"state":           "state-xyz",
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("AuthorizeURL query[%s] = %q, want %q", k, q.Get(k), v)
		}
	}
}

func TestExchangeCode(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		gotForm = r.PostForm
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "AT",
			"refresh_token": "RT",
			"expires_at":    time.Now().Add(6 * time.Hour).Unix(),
			"athlete":       map[string]any{"id": 12345},
			"scope":         "read,activity:read",
		})
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		ClientID: "cid", ClientSecret: "csec", RedirectURL: "http://x/cb",
		HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	ts, err := c.ExchangeCode(context.Background(), "the-code")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if ts.AccessToken != "AT" || ts.RefreshToken != "RT" || ts.AthleteID != 12345 {
		t.Errorf("TokenSet mal decodificado: %+v", ts)
	}
	if gotForm.Get("grant_type") != "authorization_code" || gotForm.Get("code") != "the-code" {
		t.Errorf("form incorrecto: %v", gotForm)
	}
}

func TestRefresh(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		gotForm = r.PostForm
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "AT2",
			"refresh_token": "RT2",
			"expires_at":    time.Now().Add(6 * time.Hour).Unix(),
			"athlete":       map[string]any{"id": 12345},
		})
	}))
	defer srv.Close()

	c, _ := NewClient(Config{
		ClientID: "cid", ClientSecret: "csec",
		HTTPClient: srv.Client(),
	})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	ts, err := c.Refresh(context.Background(), "old-RT")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if ts.AccessToken != "AT2" || ts.RefreshToken != "RT2" {
		t.Errorf("Refresh no rotó tokens: %+v", ts)
	}
	if gotForm.Get("grant_type") != "refresh_token" {
		t.Errorf("grant_type = %q, want refresh_token", gotForm.Get("grant_type"))
	}
}

// TestDo_GoldenPath verifica que una llamada normal al API deserializa
// el JSON en out.
func TestDo_GoldenPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strava real path: /api/v3/athlete. El rewriteTransport conserva el path.
		if r.URL.Path != "/api/v3/athlete" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer AT" {
			http.Error(w, "missing bearer", http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"id":777,"username":"alice"}`)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	var out struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	}
	if err := c.Do(context.Background(), http.MethodGet, "/athlete", "AT", &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out.ID != 777 || out.Username != "alice" {
		t.Errorf("respuesta no deserializada: %+v", out)
	}
}

// TestDo_UnauthorizedNoRetry verifica que 401 NO se reintenta: si el
// cliente reintentara, el handler recibiría más de una petición.
func TestDo_UnauthorizedNoRetry(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.Error(w, "expired", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	err := c.Do(context.Background(), http.MethodGet, "/athlete", "AT", nil)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Do: err = %v, want ErrUnauthorized", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("hits = %d, want 1 (401 NO debe reintentarse)", got)
	}
}

// TestDo_RetryOn429 verifica que 429 con Retry-After SÍ se reintenta y
// eventualmente la respuesta correcta se devuelve.
func TestDo_RetryOn429(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	var out map[string]bool
	if err := c.Do(context.Background(), http.MethodGet, "/x", "AT", &out); err != nil {
		t.Fatalf("Do tras retries: %v", err)
	}
	if !out["ok"] {
		t.Errorf("respuesta no decodificada: %+v", out)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("hits = %d, want 3 (2 reintentos + 1 éxito)", got)
	}
}

// TestDo_RetryOn5xx verifica que 500 también reintenta.
func TestDo_RetryOn5xx(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 2 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	var out map[string]bool
	if err := c.Do(context.Background(), http.MethodGet, "/x", "AT", &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("hits = %d, want 2 (1 retry + 1 éxito)", got)
	}
}

// TestDo_Client4xxNoRetry verifica que un 400 (no 401/429) NO se reintenta.
func TestDo_Client4xxNoRetry(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.Error(w, `{"message":"bad"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	err := c.Do(context.Background(), http.MethodGet, "/x", "AT", nil)
	if err == nil || errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrRateLimited) {
		t.Fatalf("Do: err = %v, esperaba 400 genérico", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("hits = %d, want 1 (400 NO debe reintentarse)", got)
	}
}

// TestDo_ContextCanceled verifica que si ctx se cancela mientras el
// cliente espera una respuesta, devuelve ctx.Err().
func TestDo_ContextCanceled(t *testing.T) {
	// Handler que cuelga: nunca responde. El ctx vence antes de que
	// el cliente reciba nada y debe devolver ctx.Err() desde el Do.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := c.Do(ctx, http.MethodGet, "/x", "AT", nil)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Do: err = %v, esperaba context.DeadlineExceeded", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Rate limiter tests
// ─────────────────────────────────────────────────────────────────────────

// TestRateLimiter_AcquireConsumeTokens verifica que las primeras N
// llamadas pasan instantáneamente y la N+1ª bloquea hasta el refill.
func TestRateLimiter_AcquireConsumeTokens(t *testing.T) {
	r := newRateLimiter(3, time.Hour, 100)

	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if err := r.Acquire(ctx); err != nil {
			t.Fatalf("Acquire #%d: %v", i+1, err)
		}
	}

	// El cuartoAcquire debe bloquear; cancelamos a los 50ms para verificar
	// que devuelve ctx.Err() en lugar de quedarse colgado.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := r.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("4º Acquire: err = %v, want DeadlineExceeded", err)
	}
}

// TestRateLimiter_RefillAfterWindow verifica que tras la ventana, los
// tokens vuelven.
func TestRateLimiter_RefillAfterWindow(t *testing.T) {
	r := newRateLimiter(1, 50*time.Millisecond, 100)

	ctx := context.Background()
	if err := r.Acquire(ctx); err != nil {
		t.Fatalf("primer Acquire: %v", err)
	}

	// Esperar un poco más que la ventana.
	time.Sleep(80 * time.Millisecond)

	ctx2, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := r.Acquire(ctx2); err != nil {
		t.Fatalf("Acquire tras refill: %v", err)
	}
}

// TestRateLimiter_BothWindows verifica que el límite diario también
// cuenta independientemente del de 15min.
func TestRateLimiter_BothWindows(t *testing.T) {
	r := newRateLimiter(100, time.Hour, 2)

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := r.Acquire(ctx); err != nil {
			t.Fatalf("Acquire #%d: %v", i+1, err)
		}
	}

	// El tercero debe bloquear (la ventana diaria no se ha movido).
	ctx2, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := r.Acquire(ctx2); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("3er Acquire: err = %v, want DeadlineExceeded", err)
	}
}

// TestParseRetryAfter cubre los formatos de Retry-After (segundos y fecha).
func TestParseRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"0", 0},
		{"30", 30 * time.Second},
		{"abc", 0},
	} {
		got := parseRetryAfter(tc.in)
		if got != tc.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}

	// Caso fecha: el header HTTP tiene formato RFC1123 con GMT.
	future := time.Now().Add(60 * time.Second).UTC().Format(http.TimeFormat)
	got := parseRetryAfter(future)
	if got < 55*time.Second || got > 65*time.Second {
		t.Errorf("parseRetryAfter(future) = %v, want ~60s", got)
	}
}

// TestParseRetryAfter_ExpiredDate verifica que una fecha en el pasado
// devuelve 0 (no negativo, que rompería time.NewTimer).
func TestParseRetryAfter_ExpiredDate(t *testing.T) {
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	if d := parseRetryAfter(past); d != 0 {
		t.Errorf("parseRetryAfter(past) = %v, want 0", d)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// API Methods tests (Slice 2: issue #85)
// ─────────────────────────────────────────────────────────────────────────

// TestGetAthleteInfo verifica que GetAthlete devuelve la información
// del atleta autenticado desde GET /api/v3/athlete.
func TestGetAthleteInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/athlete" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":        12345,
			"username":  "testathlete",
			"firstname": "Test",
			"lastname":  "Athlete",
			"profile":   "https://example.com/profile.jpg",
		})
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	athlete, err := c.GetAthlete(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetAthlete: %v", err)
	}
	if athlete.ID != 12345 || athlete.Username != "testathlete" {
		t.Errorf("athlete not decoded correctly: %+v", athlete)
	}
}

// TestGetActivities verifica que GetActivities devuelve actividades
// paginadas desde GET /api/v3/athlete/activities con filtros de fecha.
func TestGetActivities(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/athlete/activities" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		gotQuery = r.URL.Query()
		// Mock response with 2 activities
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":                   111,
				"name":                 "Morning Run",
				"type":                 "Run",
				"start_date":           "2025-01-15T08:00:00Z",
				"distance":             10000.0,
				"moving_time":          3000,
				"elapsed_time":         3600,
				"total_elevation_gain": 50.0,
			},
			{
				"id":                   222,
				"name":                 "Evening Ride",
				"type":                 "Ride",
				"start_date":           "2025-01-14T18:00:00Z",
				"distance":             30000.0,
				"moving_time":          5400,
				"elapsed_time":         6000,
				"total_elevation_gain": 200.0,
			},
		})
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	after := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC)
	activities, err := c.GetActivities(context.Background(), "test-token", after, before, 1, 30)
	if err != nil {
		t.Fatalf("GetActivities: %v", err)
	}
	if len(activities) != 2 {
		t.Errorf("got %d activities, want 2", len(activities))
	}
	if activities[0].ID != 111 || activities[0].Name != "Morning Run" {
		t.Errorf("first activity incorrect: %+v", activities[0])
	}
	// Verify query parameters were sent
	if gotQuery.Get("per_page") != "30" || gotQuery.Get("page") != "1" {
		t.Errorf("pagination params incorrect: page=%s, per_page=%s", gotQuery.Get("page"), gotQuery.Get("per_page"))
	}
}

// TestGetActivity verifica que GetActivity devuelve una actividad
// individual desde GET /api/v3/activities/:id con detalles completos.
func TestGetActivity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/activities/111" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":                   111,
			"name":                 "Morning Run",
			"type":                 "Run",
			"start_date":           "2025-01-15T08:00:00Z",
			"distance":             10000.0,
			"moving_time":          3000,
			"elapsed_time":         3600,
			"total_elevation_gain": 50.0,
			"description":          "Great workout",
		})
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	activity, err := c.GetActivity(context.Background(), "test-token", 111)
	if err != nil {
		t.Fatalf("GetActivity: %v", err)
	}
	if activity.ID != 111 || activity.Name != "Morning Run" || activity.Description == "" {
		t.Errorf("activity not decoded correctly: %+v", activity)
	}
}

// TestGetStreams verifica que GetStreams devuelve streams de datos
// (time, heartrate, watts, etc.) desde GET /api/v3/activities/:id/streams.
func TestGetStreams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/activities/111/streams" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		gotQuery = r.URL.Query()
		// Mock response: Strava returns array of stream objects
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"type":          "time",
				"data":          []int{0, 1, 2, 3},
				"series_type":   "time",
				"original_size": 4,
				"resolution":    1,
			},
			{
				"type":          "heartrate",
				"data":          []float64{120, 125, 130, 135},
				"series_type":   "distance",
				"original_size": 4,
				"resolution":    250,
			},
			{
				"type":          "watts",
				"data":          []float64{250, 260, 270, 280},
				"series_type":   "distance",
				"original_size": 4,
				"resolution":    250,
			},
		})
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	streams, err := c.GetStreams(context.Background(), "test-token", 111, []string{"time", "heartrate", "watts"})
	if err != nil {
		t.Fatalf("GetStreams: %v", err)
	}
	// Check we got streams back
	if len(streams) == 0 {
		t.Errorf("streams is empty: %+v", streams)
	}
	// Verify query parameter was sent
	if gotQuery.Get("keys") == "" {
		t.Errorf("keys query parameter not sent")
	}
}

// TestGetAthleteInfo_BadToken verifica que un token inválido retorna error.
func TestGetAthleteInfo_BadToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	athlete, err := c.GetAthlete(context.Background(), "bad-token")
	if err == nil {
		t.Errorf("GetAthlete with bad token should error, got: %+v", athlete)
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

// TestGetActivities_EmptyList verifica que GetActivities devuelve lista vacía
// cuando no hay actividades en la ventana de fechas.
func TestGetActivities_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/athlete/activities" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	after := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(2025, 6, 30, 23, 59, 59, 0, time.UTC)
	activities, err := c.GetActivities(context.Background(), "test-token", after, before, 1, 30)
	if err != nil {
		t.Fatalf("GetActivities: %v", err)
	}
	if len(activities) != 0 {
		t.Errorf("got %d activities, want 0", len(activities))
	}
}

// TestGetActivities_MultiplePages verifica que se pueden solicitar diferentes páginas.
func TestGetActivities_MultiplePages(t *testing.T) {
	pageRequested := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/athlete/activities" {
			http.NotFound(w, r)
			return
		}
		pageRequested, _ = strconv.Atoi(r.URL.Query().Get("page"))
		// Return different data based on page
		if pageRequested == 2 {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":                   333,
					"name":                 "Page 2 Activity",
					"type":                 "Run",
					"start_date":           "2025-02-15T08:00:00Z",
					"distance":             5000.0,
					"moving_time":          1800,
					"elapsed_time":         1900,
					"total_elevation_gain": 10.0,
				},
			})
		} else {
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		}
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	after := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
	activities, err := c.GetActivities(context.Background(), "test-token", after, before, 2, 30)
	if err != nil {
		t.Fatalf("GetActivities page 2: %v", err)
	}
	if len(activities) != 1 || activities[0].ID != 333 {
		t.Errorf("page 2 activity not returned correctly: %+v", activities)
	}
	if pageRequested != 2 {
		t.Errorf("page requested = %d, want 2", pageRequested)
	}
}

// TestGetActivity_NotFound verifica que un ID inválido retorna error.
func TestGetActivity_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	activity, err := c.GetActivity(context.Background(), "test-token", 999999)
	if err == nil {
		t.Errorf("GetActivity with invalid ID should error, got: %+v", activity)
	}
}

// TestGetStreams_NoStreamsAvailable verifica que se devuelve lista vacía
// cuando la actividad no tiene streams.
func TestGetStreams_NoStreamsAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	streams, err := c.GetStreams(context.Background(), "test-token", 555, []string{"heartrate"})
	if err != nil {
		t.Fatalf("GetStreams: %v", err)
	}
	if len(streams) != 0 {
		t.Errorf("got %d streams, want 0", len(streams))
	}
}

// TestGetStreams_MultipleStreamTypes verifica que se pueden solicitar
// varios tipos de streams simultáneamente.
func TestGetStreams_MultipleStreamTypes(t *testing.T) {
	var gotKeys string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKeys = r.URL.Query().Get("keys")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"type":          "heartrate",
				"data":          []float64{120, 125},
				"series_type":   "distance",
				"original_size": 2,
				"resolution":    250,
			},
			{
				"type":          "cadence",
				"data":          []float64{90, 92},
				"series_type":   "distance",
				"original_size": 2,
				"resolution":    250,
			},
			{
				"type":          "watts",
				"data":          []float64{250, 260},
				"series_type":   "distance",
				"original_size": 2,
				"resolution":    250,
			},
		})
	}))
	defer srv.Close()

	c, _ := NewClient(Config{ClientID: "cid", ClientSecret: "csec", HTTPClient: srv.Client()})
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}

	streams, err := c.GetStreams(context.Background(), "test-token", 222, []string{"heartrate", "cadence", "watts"})
	if err != nil {
		t.Fatalf("GetStreams: %v", err)
	}
	if len(streams) != 3 {
		t.Errorf("got %d streams, want 3", len(streams))
	}
	// Verify that all requested types were included in the query
	if !strings.Contains(gotKeys, "heartrate") || !strings.Contains(gotKeys, "cadence") || !strings.Contains(gotKeys, "watts") {
		t.Errorf("keys query = %q, should include all requested types", gotKeys)
	}
}
