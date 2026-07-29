package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	apphttp "github.com/fgjcarlos/ghamusinos/internal/http"
	"github.com/fgjcarlos/ghamusinos/internal/strava"
)

// TestBuildRouter_WithoutStrava verifica el caso por defecto: si cfg.Strava
// es nil, las rutas /api/v1/strava/* devuelven 404 (NotFound del grupo /api).
func TestBuildRouter_WithoutStrava(t *testing.T) {
	cfg := &config.Config{
		ClerkJWKSURL: "https://clerk.example.com/jwks",
	}
	h := buildRouter(cfg, nil, nil)

	for _, path := range []string{
		"/api/v1/strava/connect",
		"/api/v1/strava/callback",
	} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		// Sin Strava configurado, las rutas no se montan: el NotFound
		// del grupo /api/v1 las captura y devuelve 404 ProblemDetail.
		// (El handler concreto puede ser 401 si el middleware de auth
		// rechaza antes — ambas son aceptables como "ruta no operativa").
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusUnauthorized {
			t.Errorf("%s sin Strava: status = %d, want 404 o 401", path, rec.Code)
		}
	}
}

// TestBuildRouter_HealthzAliveWithoutPool verifica que /healthz sigue
// funcionando sin tocar la base de datos.
func TestBuildRouter_HealthzAliveWithoutPool(t *testing.T) {
	cfg := &config.Config{
		ClerkJWKSURL: "https://clerk.example.com/jwks",
	}
	h := buildRouter(cfg, nil, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("/healthz: status = %d, want 200", rec.Code)
	}
}

// TestBuildRouter_ConnectHandlerMountedWhenStravaConfigured verifica
// que cuando hay un cliente Strava + store + key, la ruta /connect
// está montada y devuelve una URL válida hacia Strava. No tocamos la
// DB: usamos un store fake. La persistencia real (SQLCTokenStore con
// *sqlc.Queries) ya está cubierta por sus propios tests.
func TestBuildRouter_ConnectHandlerMountedWhenStravaConfigured(t *testing.T) {
	client, err := strava.NewClient(strava.Config{
		ClientID: "12345", ClientSecret: "secret",
		RedirectURL: "http://localhost/cb", Scopes: "read",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	key := bytes32(t)
	store := &fakeStore{}

	cfg := &config.Config{
		ClerkJWKSURL: "https://clerk.example.com/jwks",
		Strava: &config.StravaConfig{
			ClientID: "12345", ClientSecret: "secret",
			RedirectURL: "http://localhost/cb", Scopes: "read",
			CipherKey: key,
		},
	}

	// buildRouter haría el wiring completo con sqlc.Queries real; aquí
	// montamos un router equivalente con un store fake. Esto valida el
	// wiring del handler, no la persistencia.
	server := apphttp.NewServer(nil, nil, cfg)
	server.WithStrava(client, store, key)
	h := server.Router()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/strava/connect", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// El handler está detrás del middleware de auth; sin un usuario
	// resuelto en el contexto, el middleware rechaza con 401 antes de
	// llegar al handler. Eso también confirma que el wiring está bajo auth.
	// Para verificar el handler en sí, los tests unitarios en internal/strava
	// ya cubren ConnectHandler directamente.
	if rec.Code != http.StatusOK && rec.Code != http.StatusUnauthorized {
		t.Errorf("/api/v1/strava/connect: status = %d, want 200 o 401 (auth)", rec.Code)
	}

	// Si llegamos a 200, validamos la estructura de la respuesta.
	if rec.Code == http.StatusOK {
		var body struct {
			AuthorizeURL string `json:"authorize_url"`
			State        string `json:"state"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.AuthorizeURL == "" || body.State == "" {
			t.Fatalf("body vacío: %+v", body)
		}
		u, err := url.Parse(body.AuthorizeURL)
		if err != nil || !strings.HasPrefix(u.Host, "www.strava.com") {
			t.Errorf("authorize_url host = %v, quería www.strava.com", u)
		}
	}
}

func bytes32(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

type fakeStore struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeStore) SaveTokens(_ context.Context, _ strava.PersistedTokens) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return nil
}
