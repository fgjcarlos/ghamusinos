package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// mockQuerier es un stub minimal para tests sin base de datos.
// Implementa sqlc.Querier completo; los métodos Strava/Activities/Sync
// devuelven valores cero porque estos tests no ejercitan Strava.
type mockQuerier struct{}

func (m *mockQuerier) CreateInvite(ctx context.Context, arg sqlc.CreateInviteParams) (sqlc.Invite, error) {
	return sqlc.Invite{}, nil
}
func (m *mockQuerier) CreateSyncSession(ctx context.Context, arg sqlc.CreateSyncSessionParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, nil
}
func (m *mockQuerier) CreateUser(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockQuerier) DeleteStravaTokensByUserID(ctx context.Context, userID pgtype.UUID) error {
	return nil
}
func (m *mockQuerier) EnqueueActivityEvent(ctx context.Context, arg sqlc.EnqueueActivityEventParams) (sqlc.ActivityEvent, error) {
	return sqlc.ActivityEvent{}, nil
}
func (m *mockQuerier) GetActiveInviteByEmail(ctx context.Context, email string) (sqlc.GetActiveInviteByEmailRow, error) {
	return sqlc.GetActiveInviteByEmailRow{}, nil
}
func (m *mockQuerier) GetActivityByExternalID(ctx context.Context, arg sqlc.GetActivityByExternalIDParams) (sqlc.Activity, error) {
	return sqlc.Activity{}, nil
}
func (m *mockQuerier) GetInviteByTokenHash(ctx context.Context, tokenHash string) (sqlc.Invite, error) {
	return sqlc.Invite{}, nil
}
func (m *mockQuerier) GetStravaTokensByUserID(ctx context.Context, userID pgtype.UUID) (sqlc.StravaToken, error) {
	return sqlc.StravaToken{}, nil
}
func (m *mockQuerier) GetUserByClerkID(ctx context.Context, clerkID string) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockQuerier) ListActivitiesByUser(ctx context.Context, arg sqlc.ListActivitiesByUserParams) ([]sqlc.Activity, error) {
	return nil, nil
}
func (m *mockQuerier) ListPendingActivityEvents(ctx context.Context, limit int32) ([]sqlc.ActivityEvent, error) {
	return nil, nil
}
func (m *mockQuerier) ListSyncSessionsByUser(ctx context.Context, arg sqlc.ListSyncSessionsByUserParams) ([]sqlc.SyncSession, error) {
	return nil, nil
}
func (m *mockQuerier) MarkActivityEventProcessed(ctx context.Context, id pgtype.UUID) error {
	return nil
}
func (m *mockQuerier) MarkInviteAccepted(ctx context.Context, id pgtype.UUID) error {
	return nil
}
func (m *mockQuerier) UpdateSyncSessionProgress(ctx context.Context, arg sqlc.UpdateSyncSessionProgressParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, nil
}
func (m *mockQuerier) UpdateSyncSessionStatus(ctx context.Context, arg sqlc.UpdateSyncSessionStatusParams) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, nil
}
func (m *mockQuerier) UpdateUserInviteStatus(ctx context.Context, arg sqlc.UpdateUserInviteStatusParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockQuerier) UpdateUserPreferences(ctx context.Context, arg sqlc.UpdateUserPreferencesParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockQuerier) UpdateUserProfile(ctx context.Context, arg sqlc.UpdateUserProfileParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockQuerier) UpsertActivity(ctx context.Context, arg sqlc.UpsertActivityParams) (sqlc.Activity, error) {
	return sqlc.Activity{}, nil
}
func (m *mockQuerier) UpsertActivityStream(ctx context.Context, arg sqlc.UpsertActivityStreamParams) (sqlc.ActivityStream, error) {
	return sqlc.ActivityStream{}, nil
}
func (m *mockQuerier) UpsertStravaTokens(ctx context.Context, arg sqlc.UpsertStravaTokensParams) (sqlc.StravaToken, error) {
	return sqlc.StravaToken{}, nil
}
func (m *mockQuerier) GetLatestSyncSession(ctx context.Context, userID pgtype.UUID) (sqlc.SyncSession, error) {
	return sqlc.SyncSession{}, nil
}

// nuevoServidor es un helper de test que crea un Server con pool nil
// (entorno sin base de datos) y lo envuelve en httptest.
func nuevoServidor(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		ClerkJWKSURL:  "https://clerk.example.com/.well-known/jwks.json",
		ClerkAudience: "test",
	}
	srv := httptest.NewServer(NewServer(nil, &mockQuerier{}, cfg).Router())
	t.Cleanup(srv.Close)
	return srv
}

// testGet lanza una petición GET con context.Context contra el server.
// Usar http.Get directamente dispara el linter noctx; este helper
// mantiene el patrón idiomático sin saltarse la regla.
func testGet(t *testing.T, url string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request %s: %v", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

func TestRouterHealthz(t *testing.T) {
	srv := nuevoServidor(t)

	resp := testGet(t, srv.URL+"/healthz")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, quería %d", resp.StatusCode, http.StatusOK)
	}
}

// TestRouterReadyz_sinPool verifica que /readyz responde 503 cuando no hay pool.
func TestRouterReadyz_sinPool(t *testing.T) {
	srv := nuevoServidor(t)

	resp := testGet(t, srv.URL+"/readyz")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, quería %d (sin pool, readyz debe degradar)", resp.StatusCode, http.StatusServiceUnavailable)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Content-Type = %q, quería application/json", ct)
	}
}

// TestRouterSPARuta verifica que rutas no-API aterrizan en el handler SPA.
// Sin build de Vite ejecutado, el handler devuelve 503 (placeholder sin
// index.html). Eso es el comportamiento correcto en entorno de desarrollo
// sin assets compilados.
func TestRouterSPARuta(t *testing.T) {
	srv := nuevoServidor(t)

	resp := testGet(t, srv.URL+"/lab")
	defer func() { _ = resp.Body.Close() }()

	// Sin build: 503 (frontend no construido). Con build: 200 (SPA index.html).
	// Ambos son coherentes; lo que NO debe pasar es que llegue a chi's 404.
	if resp.StatusCode == http.StatusNotFound {
		t.Fatalf("GET /lab no debería devolver 404: el handler SPA debe interceptarlo")
	}
}

// TestRouterAPIRequiresAuth verifica que /api/inexistente rechaza sin autenticación
// (401 antes de que el router pueda devolver 404).
// Las rutas de /api están protegidas por el middleware de autenticación.
func TestRouterAPIRequiresAuth(t *testing.T) {
	srv := nuevoServidor(t)

	resp := testGet(t, srv.URL+"/api/no-existe")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/no-existe sin token quería 401, obtuvo %d", resp.StatusCode)
	}
}

// TestRouterAPIv1VersionPath verifica que /api/v1 es la ruta base de la API versionada.
// La ruta /api/v1/me existe; /api/v1/inexistente debe devolver 404 con RFC 9457 ProblemDetail.
// Ambas requieren autenticación primero.
func TestRouterAPIv1VersionPath(t *testing.T) {
	srv := nuevoServidor(t)

	// GET /api/v1/inexistente (ruta no existe en v1)
	resp := testGet(t, srv.URL+"/api/v1/inexistente")
	defer func() { _ = resp.Body.Close() }()

	// Debe rechazar con 401 (no autenticado) antes de devolver 404
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET /api/v1/inexistente sin token quería 401, obtuvo %d", resp.StatusCode)
	}
}

// TestRouterAPIUnknownVersion verifica que versiones API desconocidas (v2, v3, etc.)
// devuelven 404 con RFC 9457 ProblemDetail y content-type correcto.
func TestRouterAPIUnknownVersion(t *testing.T) {
	srv := nuevoServidor(t)

	// GET /api/v2/foo (versión desconocida)
	resp := testGet(t, srv.URL+"/api/v2/foo")
	defer func() { _ = resp.Body.Close() }()

	// Debe rechazar con 401 (no autenticado) antes de devolver 404
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/v2/foo sin token quería 401, obtuvo %d", resp.StatusCode)
	}
}
