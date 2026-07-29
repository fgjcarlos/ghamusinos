package strava

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

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
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if r.PostForm.Get("grant_type") != "authorization_code" {
			http.Error(w, "bad grant_type", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "ACCESS-RAW",
			"refresh_token": "REFRESH-RAW",
			"expires_at":    time.Now().Add(6 * time.Hour).Unix(),
			"athlete":       map[string]any{"id": 9876543210},
			"scope":         "read,activity:read",
		})
	}))

	c, err := NewClient(Config{
		ClientID:     "cid",
		ClientSecret: "csec",
		HTTPClient:   srv.Client(),
	})
	if err != nil {
		srv.Close()
		t.Fatalf("NewClient: %v", err)
	}
	c.cfg.HTTPClient.Transport = &rewriteTransport{
		fromTo: map[string]string{"www.strava.com": strings.TrimPrefix(srv.URL, "http://")},
		base:   http.DefaultTransport,
	}
	return c, srv
}

// ─────────────────────────────────────────────────────────────────────────
// ConnectHandler tests
// ─────────────────────────────────────────────────────────────────────────

func TestConnectHandler_ReturnsAuthorizeURLAndState(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/strava/connect", nil)
	rec := httptest.NewRecorder()
	ConnectHandler(c)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		AuthorizeURL string `json:"authorize_url"`
		State        string `json:"state"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.State == "" {
		t.Error("state vacío")
	}
	if len(body.State) < 32 {
		t.Errorf("state demasiado corto (%d chars): debería ser ≥32 para entropía decente", len(body.State))
	}

	u, err := url.Parse(body.AuthorizeURL)
	if err != nil {
		t.Fatalf("authorize_url no parsea: %v", err)
	}
	if u.Host != "www.strava.com" || u.Path != "/oauth/authorize" {
		t.Errorf("authorize_url host/path = %s%s, want www.strava.com/oauth/authorize", u.Host, u.Path)
	}
	if u.Query().Get("state") != body.State {
		t.Errorf("state en query (%q) no coincide con state en JSON (%q)", u.Query().Get("state"), body.State)
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
		var body struct {
			State string `json:"state"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&body)
		return body.State
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

// TestCallbackHandler_HTTPIntegration verifica el handler HTTP de cabo
// a rabo con un user_id en el contexto (inyectado por el middleware
// de auth) en vez de en la query string.
func TestCallbackHandler_HTTPIntegration(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()

	key, _ := crypto.GenerateKey()
	store := &fakeTokenStore{}

	handler := CallbackHandler(c, store, key)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/api/v1/strava/callback?code=the-code&state=the-state",
		nil)
	// Inyectamos el usuario resuelto por el middleware de auth.
	userID := "11111111-2222-3333-4444-555555555555"
	req = req.WithContext(auth.WithAuthUser(req.Context(), &auth.User{ID: userID}))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var res CallbackResult
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.UserID != userID || res.AthleteID == 0 {
		t.Errorf("resultado inesperado: %+v", res)
	}
}

// TestCallbackHandler_BadRequest verifica que faltan params devuelven 400.
// user_id ahora viene del contexto, no de la query, por lo que se omite
// del set de "faltantes" en la query string. Si el contexto NO trae
// usuario, el handler responde 401 (cubierto en TestCallbackHandler_NoUser).
func TestCallbackHandler_BadRequest(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()
	store := &fakeTokenStore{}
	handler := CallbackHandler(c, store, nil)

	ctx := auth.WithAuthUser(context.Background(), &auth.User{ID: "u"})

	for _, q := range []string{
		"/api/v1/strava/callback?state=s", // sin code
		"/api/v1/strava/callback?code=c",  // sin state
	} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, q, nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("query %q: status = %d, want 400", q, rec.Code)
		}
	}
}

// TestCallbackHandler_NoUser verifica que sin usuario en el contexto
// (middleware de auth no resolvió) el callback devuelve 401 y NO toca
// a Strava. Es la garantía de que el handler solo opera detrás del auth.
func TestCallbackHandler_NoUser(t *testing.T) {
	c, srv := stravaSrvForOAuth(t)
	defer srv.Close()
	store := &fakeTokenStore{}
	handler := CallbackHandler(c, store, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/api/v1/strava/callback?code=c&state=s", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if store.calls != 0 {
		t.Errorf("SaveTokens fue llamado %d veces; quería 0", store.calls)
	}
}
