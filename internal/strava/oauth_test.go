// Package strava (continuación): handlers OAuth de Fase 1.2 (issue #14).
//
// Estos handlers implementan el flujo "Conectar con Strava" de ADR 0001:
//
//	GET /api/v1/strava/connect?user_id=<uuid>
//	  → redirige al usuario a la pantalla de consentimiento de Strava.
//
//	GET /api/v1/strava/callback?code=...&state=...
//	  → Strava redirige aquí tras el consentimiento. Intercambiamos
//	    el code por tokens, los ciframos y los guardamos.
//
// # Alcance de este esqueleto (issue #14, opción A)
//
//   - El handshake OAuth funciona y persiste tokens cifrados.
//   - El parámetro state se valida no-vacío (CSRF real se delega a Clerk
//     en producción; ver TODO).
//   - user_id se recibe por query para evitar acoplar este stub al
//     middleware de auth — la wiring real se hace en la fase de
//     autenticación cuando se conecte Clerk.
//
// # Lo que NO hace este esqueleto
//
//   - No envía al usuario al frontend tras conectar (front lo resuelve #90).
//   - No expone endpoints de "desconectar" (se hace con DeleteStravaTokensByUserID,
//     ya cubierto en sqlc).
//   - No implementa refresh proactivo (es responsabilidad del job de ingest).
package strava

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/crypto"
)

// fakeTokenStore implementa TokenStore para tests sin tocar la DB.
// Registra la última llamada para aserciones.
type fakeTokenStore struct {
	mu     sync.Mutex
	saved  []PersistedTokens
	calls  int
	failOn error // si está set, SaveTokens devuelve este error
}

func (f *fakeTokenStore) SaveTokens(_ context.Context, t PersistedTokens) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.failOn != nil {
		return f.failOn
	}
	f.saved = append(f.saved, t)
	return nil
}

func (f *fakeTokenStore) last() (PersistedTokens, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.saved) == 0 {
		return PersistedTokens{}, false
	}
	return f.saved[len(f.saved)-1], true
}

// stravaSrvForOAuth levanta un httptest que responde a /oauth/token con
// un token válido. Devuelve un Client ya apuntando al servidor y el
// servidor (para defer Close).
func stravaSrvForOAuth(t *testing.T) (*Client, *httptest.Server) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// La mayoría de endpoints devuelven error; solo /oauth/token tiene
		// una respuesta ficticia para simular el flujo de intercambio.
		if r.URL.Path == "/oauth/token" {
			// Respuesta ficticia que HandleCallback espera.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"token_type": "Bearer",
				"expires_in": 21600,
				"expires_at": 1568775134,
				"access_token": "ACCESS-RAW",
				"refresh_token": "REFRESH-RAW",
				"athlete": {
					"id": 9876543210,
					"resource_state": 2
				},
				"scope": "read,activity:read"
			}`))
			return
		}

		// Otros endpoints devuelven error genérico. Los tests que necesiten
		// respuestas específicas pueden extender este srv.
		http.Error(w, "not implemented", http.StatusNotImplemented)
	}))

	cfg := Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://localhost/callback",
		Scopes:       "activity:read",
		HTTPClient: &http.Client{
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
		t.Fatalf("NewClient: %v", err)
	}

	return c, srv
}

// ─────────────────────────────────────────────────────────────────────────
// Connect tests
// ─────────────────────────────────────────────────────────────────────────

// TestConnectHandler_RedirectsToStrava verifica que ConnectHandler redirija
// con un 302 directo a Strava (no JSON).
func TestConnectHandler_RedirectsToStrava(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/strava/connect", nil)
	rec := httptest.NewRecorder()
	ConnectHandler(c)(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "https://www.strava.com/oauth/authorize?") {
		t.Errorf("Location = %q, want strava.com prefix", location)
	}

	// Verificar state en query
	u, _ := url.Parse(location)
	if state := u.Query().Get("state"); state == "" {
		t.Error("state missing in redirect")
	} else if len(state) < 32 {
		t.Errorf("state demasiado corto (%d chars): debería ser ≥32", len(state))
	}
}

// TestConnectHandler_StateUnique verifica que dos llamadas generan
// states distintos (CSRF protection depende de esto).
func TestConnectHandler_StateUnique(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()

	get := func() string {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/strava/connect", nil)
		rec := httptest.NewRecorder()
		ConnectHandler(c)(rec, req)
		u, _ := url.Parse(rec.Header().Get("Location"))
		return u.Query().Get("state")
	}

	s1, s2 := get(), get()
	if s1 == "" || s2 == "" {
		t.Fatal("states vacíos")
	}
	if s1 == s2 {
		t.Error("dos connect devolvieron el mismo state: random roto")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Callback tests
// ─────────────────────────────────────────────────────────────────────────

// TestHandleCallback_GoldenPath: code válido → tokens cifrados y
// persistidos. Verificamos que lo guardado está cifrado (no aparece el
// token raw) y que se descifra correctamente al pedirlo.
func TestHandleCallback_GoldenPath(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	store := &fakeTokenStore{}

	res, err := HandleCallback(context.Background(), c, store, key, "11111111-2222-3333-4444-555555555555", CallbackParams{
		Code:  "valid-code",
		State: "csrf-state",
	})
	if err != nil {
		t.Fatalf("HandleCallback: %v", err)
	}
	if res.UserID != "11111111-2222-3333-4444-555555555555" || res.AthleteID != 9876543210 {
		t.Errorf("resultado inesperado: %+v", res)
	}

	saved, ok := store.last()
	if !ok {
		t.Fatal("store no recibió SaveTokens")
	}
	if saved.AccessCipher == "" || saved.RefreshCipher == "" {
		t.Fatal("ciphers vacíos")
	}
	if saved.AccessCipher == "ACCESS-RAW" || saved.RefreshCipher == "REFRESH-RAW" {
		t.Error("los tokens se guardaron en claro, no cifrados")
	}

	// Descifrar y comparar.
	access, err := crypto.Decrypt(saved.AccessCipher, key)
	if err != nil {
		t.Fatalf("decrypt access: %v", err)
	}
	if string(access) != "ACCESS-RAW" {
		t.Errorf("access descifrado = %q, want ACCESS-RAW", access)
	}
	refresh, err := crypto.Decrypt(saved.RefreshCipher, key)
	if err != nil {
		t.Fatalf("decrypt refresh: %v", err)
	}
	if string(refresh) != "REFRESH-RAW" {
		t.Errorf("refresh descifrado = %q, want REFRESH-RAW", refresh)
	}
}

// TestHandleCallback_MissingCode verifica el guard contra input vacío.
func TestHandleCallback_MissingCode(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()
	store := &fakeTokenStore{}

	_, err := HandleCallback(context.Background(), c, store, nil, "u", CallbackParams{Code: "", State: "s"})
	if err == nil {
		t.Fatal("esperaba error por code vacío")
	}
	if !strings.Contains(err.Error(), "missing code") {
		t.Errorf("error = %v, esperaba 'missing code'", err)
	}
}

// TestHandleCallback_MissingState cubre la otra validación de input.
func TestHandleCallback_MissingState(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()
	store := &fakeTokenStore{}

	_, err := HandleCallback(context.Background(), c, store, nil, "u", CallbackParams{Code: "c", State: ""})
	if err == nil || !strings.Contains(err.Error(), "missing state") {
		t.Errorf("err = %v, esperaba 'missing state'", err)
	}
}

// TestHandleCallback_MissingUserID cubre el caso de user_id vacío.
func TestHandleCallback_MissingUserID(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()
	store := &fakeTokenStore{}

	_, err := HandleCallback(context.Background(), c, store, nil, "", CallbackParams{Code: "c", State: "s"})
	if err == nil || !strings.Contains(err.Error(), "missing user_id") {
		t.Errorf("err = %v, esperaba 'missing user_id'", err)
	}
}

// TestHandleCallback_StoreError verifica que un error del store se
// propaga (no se silencia y devuelve un resultado vacío).
func TestHandleCallback_StoreError(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()
	store := &fakeTokenStore{failOn: errors.New("db caída")}
	key, _ := crypto.GenerateKey()

	_, err := HandleCallback(context.Background(), c, store, key, "u", CallbackParams{Code: "c", State: "s"})
	if err == nil || !strings.Contains(err.Error(), "persist tokens") {
		t.Errorf("err = %v, esperaba envolver 'persist tokens'", err)
	}
}

// TestCallbackHandler_RedirectsOnSuccess verifica que en éxito redirija a
// /activities?connected=1 con un 302.
func TestCallbackHandler_RedirectsOnSuccess(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()

	key, _ := crypto.GenerateKey()
	store := &fakeTokenStore{}

	frontendURL := "http://localhost:5173"
	handler := CallbackHandler(c, store, key, frontendURL)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/api/v1/strava/callback?code=the-code&state=the-state",
		nil)
	// Inyectamos el usuario resuelto por el middleware de auth.
	userID := "11111111-2222-3333-4444-555555555555"
	req = req.WithContext(auth.WithAuthUser(req.Context(), &auth.User{ID: userID}))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body = %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if !strings.HasSuffix(location, "/activities?connected=1") {
		t.Errorf("Location = %q, want to end with /activities?connected=1", location)
	}
}

// TestCallbackHandler_BadRequest verifica que faltan params devuelven un redirect de error.
// user_id ahora viene del contexto, no de la query, por lo que se omite
// del set de "faltantes" en la query string. Si el contexto NO trae
// usuario, el handler responde 302 a /?error=... (cubierto en TestCallbackHandler_NoUser).
func TestCallbackHandler_BadRequest(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()
	store := &fakeTokenStore{}
	frontendURL := "http://localhost:5173"
	handler := CallbackHandler(c, store, nil, frontendURL)

	ctx := auth.WithAuthUser(context.Background(), &auth.User{ID: "u"})

	for _, q := range []string{
		"/api/v1/strava/callback?state=s", // sin code
		"/api/v1/strava/callback?code=c",  // sin state
	} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, q, nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusFound {
			t.Errorf("query %q: status = %d, want 302", q, rec.Code)
		}
		location := rec.Header().Get("Location")
		if !strings.Contains(location, "/?error=") {
			t.Errorf("query %q: Location = %q, want /?error=...", q, location)
		}
	}
}

// TestCallbackHandler_NoUser verifica que sin usuario en el contexto
// (middleware de auth no resolvió) el callback devuelve 302 a /?error=unauthenticated
// y NO toca a Strava. Es la garantía de que el handler solo opera detrás del auth.
func TestCallbackHandler_NoUser(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()
	store := &fakeTokenStore{}
	frontendURL := "http://localhost:5173"
	handler := CallbackHandler(c, store, nil, frontendURL)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/api/v1/strava/callback?code=c&state=s", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want 302", rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "/?error=unauthenticated") {
		t.Errorf("Location = %q, want /?error=unauthenticated", location)
	}
	if store.calls != 0 {
		t.Errorf("SaveTokens fue llamado %d veces; quería 0", store.calls)
	}
}
