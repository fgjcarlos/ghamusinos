// Package strava (continuación): handlers OAuth de Fase 1.2 (issue #14, AUD-02).
//
// Estos handlers implementan el flujo "Conectar con Strava" de ADR 0001:
//
//	GET /api/v1/strava/connect      (bajo auth)
//	  → devuelve {"authorize_url": "..."} con un state firmado que contiene
//	    el user_id. El frontend hace window.location.assign(url).
//
//	GET /strava/callback            (PÚBLICO, sin auth)
//	  → Strava redirige aquí tras el consentimiento. Verifica la firma
//	    HMAC-SHA256 del state, extrae user_id del payload, intercambia
//	    el code por tokens, los cifra y los guarda.
//
// # Diseño del state firmado (AUD-02, hallazgo C1+C2)
//
// El state es base64url(payload) + "." + base64url(HMAC-SHA256(payload, key)).
// El payload lleva {uid, nonce, exp}. No hace falta almacén: el HMAC demuestra
// que el state lo emitió este servidor, la expiración corta (~10 min) limita
// la ventana de reutilización, y el uid viaja con el state — el callback no
// necesita autenticación para saber a qué usuario vincular los tokens.
//
// El HMAC usa la misma STRAVA_CIPHER_KEY (32 bytes) que ya sirve para cifrar
// tokens. Reutilizar evita una variable de entorno nueva; el HMAC-SHA256 y
// el AES-256-GCM son primitivas distintas sobre la misma clave, lo cual es
// práctica estándar (HKDF sería la versión "correcta", pero aquí el secreto
// ya está rotando y la superficie de ataque es idéntica).
//
// # Lo que NO hace este paquete
//
//   - No envía al usuario al frontend tras conectar (front lo resuelve #90).
//   - No expone endpoints de "desconectar" (se hace con DeleteStravaTokensByUserID).
//   - No implementa refresh proactivo (es responsabilidad del job de ingest).
package strava

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/crypto"
)

// stateLifetime es la ventana en la que un state firmado es válido.
// 10 minutos es más que suficiente para el flujo OAuth humano de Strava
// (usuario abre navegador → confirma → vuelve) y limita el blast radius
// si un state se filtra antes de usarse.
const stateLifetime = 10 * time.Minute

// TokenStore es la interfaz mínima que los handlers OAuth necesitan para
// persistir tokens cifrados. La implementación concreta envuelve *sqlc.Queries
// (ver NewSQLCTokenStore en oauth_sqlc.go). Tests usan un fake.
type TokenStore interface {
	// SaveTokens persiste los tokens (ya cifrados) para el usuario.
	SaveTokens(ctx context.Context, t PersistedTokens) error
}

// RiverEnqueuer es el seam que el callback OAuth usa para encolar el job
// de backfill tras un intercambio de tokens exitoso. AUD-04 AC:
// "Conectar Strava encola un ImportStravaArgs".
//
// Mantener la interfaz aquí (en lugar de importar el paquete jobs)
// preserva la dirección del grafo: strava no depende de jobs. La
// implementación en producción la provee internal/app cuando construye
// el TokenStore SQLC y registra el River client.
type RiverEnqueuer interface {
	// EnqueueImportStrava encola un job ImportStravaArgs para el usuario.
	// Devuelve error si la encolación falla; HandleCallback propaga el
	// error al caller.
	EnqueueImportStrava(ctx context.Context, userID string) error
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

// oauthStatePayload es el cuerpo del state firmado. Vive ~10 minutos; no
// se persiste en ningún sitio (eso es el punto: sin almacén, sin migración,
// sin purga).
type oauthStatePayload struct {
	UserID string `json:"uid"`
	Nonce  string `json:"n"`
	Exp    int64  `json:"exp"` // unix seconds
}

// signState firma el payload con HMAC-SHA256 y devuelve el state
// codificado como base64url(payload) + "." + base64url(signature).
// La firma es verificable con verifyState usando la misma clave.
func signState(payload oauthStatePayload, cipherKey []byte) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal state payload: %w", err)
	}
	mac := hmac.New(sha256.New, cipherKey)
	mac.Write(body)
	sig := mac.Sum(nil)
	enc := base64.RawURLEncoding
	return enc.EncodeToString(body) + "." + enc.EncodeToString(sig), nil
}

// verifyState valida que el state venga firmado por este servidor y que
// no haya expirado. Devuelve el payload si todo OK; error tipado si no.
// Comparación constant-time en la firma para no abrir timing attacks.
func verifyState(state string, cipherKey []byte, now time.Time) (oauthStatePayload, error) {
	var zero oauthStatePayload
	if state == "" {
		return zero, errors.New("strava: empty state")
	}
	dot := strings.Index(state, ".")
	if dot < 0 {
		return zero, errors.New("strava: malformed state (no signature separator)")
	}
	enc := base64.RawURLEncoding
	body, err := enc.DecodeString(state[:dot])
	if err != nil {
		return zero, fmt.Errorf("strava: decode state body: %w", err)
	}
	sig, err := enc.DecodeString(state[dot+1:])
	if err != nil {
		return zero, fmt.Errorf("strava: decode state signature: %w", err)
	}
	mac := hmac.New(sha256.New, cipherKey)
	mac.Write(body)
	if !hmac.Equal(mac.Sum(nil), sig) {
		return zero, errors.New("strava: state signature mismatch")
	}
	var payload oauthStatePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return zero, fmt.Errorf("strava: unmarshal state payload: %w", err)
	}
	if payload.Exp == 0 || now.Unix() > payload.Exp {
		return zero, errors.New("strava: state expired")
	}
	if payload.UserID == "" {
		return zero, errors.New("strava: state missing user_id")
	}
	return payload, nil
}

// ConnectHandler devuelve el handler para /api/v1/strava/connect.
// Sigue montado detrás del middleware de auth (ruta bajo /api/v1/*). El
// user_id se obtiene del contexto y se mete dentro del state firmado;
// el callback no necesita volver a leerlo del contexto porque el state
// viaja en la query string de la redirección de Strava.
//
// Devuelve un JSON con `authorize_url` y `state`. El frontend hace
// window.location.assign(url) en vez de un <a href>, porque las
// redirecciones top-level del navegador no envían la cabecera Authorization
// y por tanto no llegan al handler autenticado. AUD-02, hallazgo C1.
func ConnectHandler(client *Client, cipherKey []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := auth.AuthUser(r.Context())
		if user == nil || user.ID == "" {
			http.Error(w, "strava: unauthenticated", http.StatusUnauthorized)
			return
		}
		nonce, err := randomNonce()
		if err != nil {
			http.Error(w, "strava: state generation failed", http.StatusInternalServerError)
			return
		}
		state, err := signState(oauthStatePayload{
			UserID: user.ID,
			Nonce:  nonce,
			Exp:    time.Now().Add(stateLifetime).Unix(),
		}, cipherKey)
		if err != nil {
			http.Error(w, "strava: state signing failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"authorize_url": client.AuthorizeURL(state),
		})
	}
}

// CallbackParams es el cuerpo del callback de Strava. Aislamos la query
// string en este struct para hacer el handler testeable sin http.Request.
type CallbackParams struct {
	Code  string
	State string
}

// CallbackResult es lo que el handler devuelve al caller (test).
type CallbackResult struct {
	UserID    string `json:"user_id"`
	AthleteID int64  `json:"athlete_id"`
	Scopes    string `json:"scopes"`
}

// CallbackHandler intercambia el code por tokens, los cifra y los persiste.
// Se monta en /strava/callback (PÚBLICO, sin auth). El user_id sale del
// state firmado; la cabecera Authorization no se usa en este flujo porque
// Strava redirige al navegador del usuario, que no lleva cabecera.
//
// En éxito, redirige a {frontendURL}/activities?connected=1.
// En error, redirige a {frontendURL}/?error={urlEncodedError}.
func CallbackHandler(client *Client, store TokenStore, enqueuer RiverEnqueuer, cipherKey []byte, frontendURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		params := CallbackParams{Code: q.Get("code"), State: q.Get("state")}

		_, err := HandleCallback(r.Context(), client, store, cipherKey, params, cipherKey, time.Now(), enqueuer)
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
//
// stateKey y cipherKey son la misma STRAVA_CIPHER_KEY; los parámetros
// van separados porque juegan papeles distintos (uno verifica el state,
// el otro cifra los tokens) y poder mockearlos en tests sin reasignar la
// clave mejora el aislamiento. En producción son idénticos.
//
// enqueuer es opcional: si es nil, HandleCallback omite el encolado del
// job de backfill (útil en tests que solo quieren cubrir el intercambio
// de tokens). En producción app.Run siempre lo provee.
func HandleCallback(ctx context.Context, client *Client, store TokenStore, cipherKey []byte, p CallbackParams, stateKey []byte, now time.Time, enqueuer RiverEnqueuer) (*CallbackResult, error) {
	if p.Code == "" {
		return nil, errOAuth("missing code", http.StatusBadRequest)
	}
	payload, err := verifyState(p.State, stateKey, now)
	if err != nil {
		return nil, errOAuth(err.Error(), http.StatusBadRequest)
	}
	userID := payload.UserID

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

	// AUD-04 AC: "Conectar Strava encola un ImportStravaArgs".
	// El encolado se hace tras SaveTokens exitoso. Si falla, devolvemos
	// error para que el caller (el handler HTTP) sepa reintentar; los
	// tokens ya están persistidos, lo cual no es problema porque el job
	// los re-leería en su próxima ejecución.
	if enqueuer != nil {
		if err := enqueuer.EnqueueImportStrava(ctx, userID); err != nil {
			return nil, fmt.Errorf("enqueue import_strava: %w", err)
		}
	}

	return &CallbackResult{
		UserID:    userID,
		AthleteID: ts.AthleteID,
		Scopes:    ts.Scopes,
	}, nil
}

// randomNonce genera 16 bytes aleatorios codificados en base64 URL-safe.
// 16 bytes (128 bits) bastan: la unicidad es decorativa (defiende contra
// colisiones de state en la ventana de 10 minutos), no criptográfica.
func randomNonce() (string, error) {
	b := make([]byte, 16)
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
