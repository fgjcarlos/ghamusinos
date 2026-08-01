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
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/crypto"
)

// TokenStore es la interfaz mínima que los handlers OAuth necesitan para
// persistir tokens cifrados. La implementación concreta envuelve *sqlc.Queries
// (ver NewSQLCTokenStore en oauth_sqlc.go). Tests usan un fake.
type TokenStore interface {
	// SaveTokens persiste los tokens (ya cifrados) para el usuario.
	SaveTokens(ctx context.Context, t PersistedTokens) error
}

// PersistedTokens es el sobre que SaveTokens recibe: los ciphertexts
// ya en base64 y los metadatos que NO se cifran.
type PersistedTokens struct {
	UserID        string
	AccessCipher  string
	RefreshCipher string
	ExpiresAt     time.Time
	AthleteID     int64
	Scopes        string
}

// ConnectHandler devuelve el handler para /api/v1/strava/connect.
// El handler genera un state CSRF, construye la URL de Strava y redirige
// al usuario directo con un 302 (Found).
func ConnectHandler(client *Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := randomState()
		if err != nil {
			http.Error(w, "strava: state generation failed", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, client.AuthorizeURL(state), http.StatusFound)
	}
}

// CallbackParams es el cuerpo del POST que el frontend (o un proxy) hace
// tras recibir el callback de Strava. Aislamos la query string en este
// struct para hacer el handler testeable sin http.Request.
type CallbackParams struct {
	Code  string
	State string
}

// CallbackResult es lo que el handler devuelve al caller (frontend o test).
type CallbackResult struct {
	UserID    string `json:"user_id"`
	AthleteID int64  `json:"athlete_id"`
	Scopes    string `json:"scopes"`
}

// CallbackHandler intercambia el code por tokens, los cifra y los persiste.
//
// El user_id se obtiene del contexto de la request (inyectado por el
// middleware de autenticación ResolveMiddleware). Este handler se monta
// SIEMPRE detrás del middleware de auth, por lo que puede confiar en
// que el contexto contiene un usuario resuelto.
//
// En éxito, redirige a `{frontendURL}/activities?connected=1`.
// En error, redirige a `{frontendURL}/?error={urlEncodedError}`.
//
// El parámetro state sigue siendo obligatorio (defensa CSRF mínima);
// la validación completa contra la sesión de Clerk queda fuera de
// este esqueleto (TODO cuando se conecte el sistema de sesiones).
func CallbackHandler(client *Client, store TokenStore, cipherKey []byte, frontendURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.AuthUser(r.Context())
		if user == nil || user.ID == "" {
			http.Redirect(w, r, frontendURL+"/?error=unauthenticated", http.StatusFound)
			return
		}
		userID := user.ID

		q := r.URL.Query()
		params := CallbackParams{Code: q.Get("code"), State: q.Get("state")}

		_, err := HandleCallback(r.Context(), client, store, cipherKey, userID, params)
		if err != nil {
			http.Redirect(w, r, frontendURL+"/?error="+url.QueryEscape(err.Error()), http.StatusFound)
			return
		}

		http.Redirect(w, r, frontendURL+"/activities?connected=1", http.StatusFound)
	}
}

// HandleCallback es la lógica pura del callback (sin http). Separar el
// handler HTTP del procesamiento facilita los tests y deja una función
// reutilizable cuando llegue un job River que también intercambie codes.
func HandleCallback(ctx context.Context, client *Client, store TokenStore, cipherKey []byte, userID string, p CallbackParams) (*CallbackResult, error) {
	if p.Code == "" {
		return nil, errOAuth("missing code", http.StatusBadRequest)
	}
	if p.State == "" {
		return nil, errOAuth("missing state", http.StatusBadRequest)
	}
	// TODO(clerk): validar state contra la sesión/CSRF store real de Clerk.
	// En este esqueleto solo exigimos que venga no-vacío.
	if userID == "" {
		return nil, errOAuth("missing user_id", http.StatusBadRequest)
	}

	ts, err := client.ExchangeCode(ctx, p.Code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	accessCipher, err := crypto.Encrypt([]byte(ts.AccessToken), cipherKey)
	if err != nil {
		return nil, fmt.Errorf("cipher access: %w", err)
	}
	refreshCipher, err := crypto.Encrypt([]byte(ts.RefreshToken), cipherKey)
	if err != nil {
		return nil, fmt.Errorf("cipher refresh: %w", err)
	}

	if err := store.SaveTokens(ctx, PersistedTokens{
		UserID:        userID,
		AccessCipher:  accessCipher,
		RefreshCipher: refreshCipher,
		ExpiresAt:     ts.ExpiresAt,
		AthleteID:     ts.AthleteID,
		Scopes:        ts.Scopes,
	}); err != nil {
		return nil, fmt.Errorf("persist tokens: %w", err)
	}

	return &CallbackResult{
		UserID:    userID,
		AthleteID: ts.AthleteID,
		Scopes:    ts.Scopes,
	}, nil
}

// randomState genera un state CSRF de 32 bytes codificado en base64 URL-safe.
// Se devuelve error solo si el sistema de random falla, lo que es
// excepcional y aborta la operación (no fallback).
func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// oauthError es el error interno del paquete: lleva un status HTTP para
// que el handler lo traduzca sin tener que mapear casos manualmente.
type oauthError struct {
	msg    string
	status int
}

func (e *oauthError) Error() string { return e.msg }

func errOAuth(msg string, status int) *oauthError {
	return &oauthError{msg: msg, status: status}
}

func writeOAuthError(w http.ResponseWriter, err error) {
	var oe *oauthError
	if errors.As(err, &oe) {
		http.Error(w, oe.msg, oe.status)
		return
	}
	http.Error(w, err.Error(), http.StatusBadGateway)
}

// errOAuthMissingEnv es exported para tests que quieran construir errores
// sin HTTP (no se usa en runtime).
var errOAuthMissingEnv = errors.New("strava: STRAVA_CIPHER_KEY debe tener 32 bytes base64")
