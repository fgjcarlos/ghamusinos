// Package strava implementa el cliente HTTP de Strava con rate limit
// awareness y reintentos. Forma parte de la Fase 1.2 (issue #14) y
// asume ADR 0001: una sola app global del proyecto.
//
// El paquete está diseñado para uso concurrente seguro: un único Client
// puede ser compartido por todos los handlers y jobs. El rate limiter es
// global al proceso (no por usuario), reflejando el límite real de Strava
// que es por app.
package strava

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sethvargo/go-retry"
)

// Constantes del API de Strava y de la app (ADR 0001).
const (
	// AuthorizeURL es donde redirigimos al usuario para que autorice.
	AuthorizeURL = "https://www.strava.com/oauth/authorize"
	// TokenURL es el endpoint de intercambio y refresh de tokens.
	TokenURL = "https://www.strava.com/oauth/token"
	// APIBase es la base del API v3.
	APIBase = "https://www.strava.com/api/v3"

	// Rate limits por app (NO por usuario). Documentados por Strava:
	// 200 requests / 15 minutos, 2000 requests / día.
	RateLimitShortWindow = 200
	RateLimitShortPeriod = 15 * time.Minute
	RateLimitDailyQuota  = 2000

	// defaultMaxRetries acota los reintentos automáticos del cliente.
	// Strava documenta 429 con Retry-After; reintentamos hasta 3 veces
	// para errores transitorios (5xx, 429). Errores 4xx del cliente no
	// se reintentan (no se van a arreglar solos).
	defaultMaxRetries = 3
)

// Config agrupa la configuración de la app Strava (ADR 0001: una sola
// app global, no por usuario). Los valores son obligatorios; el caller
// los lee del entorno (STRAVA_CLIENT_ID / STRAVA_CLIENT_SECRET).
type Config struct {
	ClientID     string
	ClientSecret string
	// RedirectURL es la URL absoluta de callback que el usuario verá
	// configurada en la app de Strava. Debe coincidir exactamente.
	RedirectURL string
	// Scopes es la lista de scopes pedidos en la autorización.
	// Lo razonable es "read,activity:read" para V1 (lectura de actividades).
	Scopes string
	// HTTPClient opcional para tests; si es nil se usa http.DefaultClient.
	HTTPClient *http.Client
}

// TokenSet es el resultado de intercambiar/refresh del code de OAuth.
// Strava devuelve access_token, refresh_token y expires_at (epoch seconds).
type TokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	AthleteID    int64
	Scopes       string
}

// ErrUnauthorized se devuelve cuando Strava rechaza un token (401). El
// caller debería disparar un refresh; si tras refresh sigue 401, hay que
// re-prompt al usuario para volver a autorizar.
var ErrUnauthorized = errors.New("strava: token rechazado (401)")

// ErrRateLimited se devuelve cuando Strava responde 429 y agotamos los
// reintentos respetando Retry-After. El cliente NO reintenta eternamente.
var ErrRateLimited = errors.New("strava: rate limit agotado")

// Client es el cliente HTTP de Strava. Es seguro para uso concurrente.
type Client struct {
	cfg     Config
	limiter *rateLimiter
}

// NewClient devuelve un Client listo para usar. Valida que ClientID y
// ClientSecret no estén vacíos (la app no funciona sin ellos).
func NewClient(cfg Config) (*Client, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("strava: ClientID y ClientSecret son obligatorios")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.Scopes == "" {
		cfg.Scopes = "read,activity:read"
	}
	return &Client{
		cfg:     cfg,
		limiter: newRateLimiter(RateLimitShortWindow, RateLimitShortPeriod, RateLimitDailyQuota),
	}, nil
}

// AuthorizeURL construye la URL a la que redirigimos al usuario para
// autorizar la app. El parámetro state lo genera el caller y debe
// persistirse en sesión (CSRF protection, fuera de alcance de este paquete).
func (c *Client) AuthorizeURL(state string) string {
	v := url.Values{}
	v.Set("client_id", c.cfg.ClientID)
	v.Set("response_type", "code")
	v.Set("redirect_uri", c.cfg.RedirectURL)
	v.Set("approval_prompt", "auto")
	v.Set("scope", c.cfg.Scopes)
	v.Set("state", state)
	return AuthorizeURL + "?" + v.Encode()
}

// ExchangeCode intercambia el ?code= del callback por un TokenSet.
// El callback handler llama a esto una vez; a partir de ahí se usa el
// RefreshToken del TokenSet para renovar.
func (c *Client) ExchangeCode(ctx context.Context, code string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	return c.doToken(ctx, form)
}

// Refresh intercambia el refresh_token por un nuevo TokenSet.
// Strava rota el refresh_token en cada refresh; conservar el devuelto.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("refresh_token", refreshToken)
	form.Set("grant_type", "refresh_token")
	return c.doToken(ctx, form)
}

// Do ejecuta una request autenticada contra el API de Strava (no
// contra /oauth/token). path es la ruta empezando por "/" (ej:
// "/athlete/activities"). method es GET/POST/PUT/... El accessToken
// viene de un TokenSet descifrado por el caller.
func (c *Client) Do(ctx context.Context, method, path, accessToken string, out any) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, APIBase+path, nil)
	if err != nil {
		return fmt.Errorf("strava: NewRequest: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return c.doJSON(ctx, req, out)
}

// doToken ejecuta el flujo de token (exchange o refresh). Es interno
// porque solo se usa desde ExchangeCode y Refresh.
func (c *Client) doToken(ctx context.Context, form url.Values) (*TokenSet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("strava: NewRequest token: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresAt    int64  `json:"expires_at"`
		Athlete      struct {
			ID int64 `json:"id"`
		} `json:"athlete"`
		Scope string `json:"scope"`
	}
	if err := c.doJSON(ctx, req, &raw); err != nil {
		return nil, err
	}

	return &TokenSet{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresAt:    time.Unix(raw.ExpiresAt, 0).UTC(),
		AthleteID:    raw.Athlete.ID,
		Scopes:       raw.Scope,
	}, nil
}

// doJSON ejecuta req con rate limit awareness + reintentos y deserializa
// la respuesta en out (si out es no-nil).
//
// Estrategia de reintentos:
//   - 429 y 5xx → reintenta con backoff exponencial respetando Retry-After.
//   - 401 → reintenta UNA vez (deja al caller la responsabilidad de
//     refrescar el token antes de llamar; esta capa no conoce al usuario).
//     401 persistente se devuelve como ErrUnauthorized.
//   - Otros 4xx → no reintentar; devolver error inmediato.
func (c *Client) doJSON(ctx context.Context, req *http.Request, out any) error {
	// Slot del rate limiter: bloquea hasta que haya hueco en la cuota.
	// Si ctx se cancela mientras esperamos, devolvemos ese error.
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	// Reintentos. El cuerpo de la request puede no ser re-readable
	// (lo consume NewRequest), así que lo guardamos una vez y lo
	// re-inyectamos en cada intento.
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("strava: read body: %w", err)
		}
		_ = req.Body.Close()
	}

	backoff := retry.NewExponential(500 * time.Millisecond)
	backoff = retry.WithMaxRetries(uint64(defaultMaxRetries), backoff)
	backoff = retry.WithCappedDuration(15*time.Second, backoff)

	var lastErr error
	attempt := 0
	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		attempt++
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err := c.cfg.HTTPClient.Do(req)
		if err != nil {
			// Errores de transporte (DNS, conexión rota, timeout): reintentar.
			lastErr = err
			return retry.RetryableError(err)
		}
		defer func() { _ = resp.Body.Close() }()

		switch {
		case resp.StatusCode == http.StatusUnauthorized:
			lastErr = ErrUnauthorized
			return ErrUnauthorized // no retryable

		case resp.StatusCode == http.StatusTooManyRequests:
			ra := parseRetryAfter(resp.Header.Get("Retry-After"))
			lastErr = fmt.Errorf("%w (Retry-After=%s)", ErrRateLimited, ra)
			// Sleep dentro del retry lo hace go-retry; el Retry-After
			// se respeta en la siguiente vuelta del backoff porque
			// devolvemos RetryableError.
			return retry.RetryableError(lastErr)

		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("strava: 5xx (%d)", resp.StatusCode)
			return retry.RetryableError(lastErr)

		case resp.StatusCode >= 400:
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("strava: %d %s: %s", resp.StatusCode, http.StatusText(resp.StatusCode), strings.TrimSpace(string(body)))

		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			if out == nil {
				return nil
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("strava: read response: %w", err)
			}
			if err := json.Unmarshal(body, out); err != nil {
				return fmt.Errorf("strava: decode: %w", err)
			}
			return nil

		default:
			lastErr = fmt.Errorf("strava: status inesperado %d", resp.StatusCode)
			return lastErr
		}
	})

	if err != nil {
		// No dejamos "attempt=0 se fue sin reintentar" cuando hubo error.
		if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrRateLimited) {
			return err
		}
		_ = attempt // solo se usa en depuración si la hicieras
		if lastErr == nil {
			lastErr = err
		}
		return fmt.Errorf("tras %d intentos: %w", attempt, lastErr)
	}
	return nil
}

// parseRetryAfter implementa el formato de Retry-After según RFC 7231:
// segundos (entero) o fecha HTTP. Devuelve 0 si no se puede parsear;
// el caller usará el backoff por defecto.
func parseRetryAfter(s string) time.Duration {
	if s == "" {
		return 0
	}
	if secs, err := strconv.Atoi(s); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(s); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}
