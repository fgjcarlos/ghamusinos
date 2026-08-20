// Package http — security middleware.
//
// SecurityHeaders aplica una serie de cabeceras defensivas a cada
// respuesta (CSP, X-Frame-Options, Referrer-Policy, Permissions-Policy
// y HSTS opcional). Configurables vía SecurityHeadersConfig.
//
// BodyLimit envuelve el handler en http.MaxBytesHandler con el límite
// dado. Cuando el cuerpo excede el límite, http.MaxBytesHandler hace
// que los reads subsiguientes de r.Body devuelvan *http.MaxBytesError;
// cada handler que lea el body debe usar handlers.IsBodyLimitError
// (errors.As) para responder 413 application/problem+json en lugar de
// propagar como 500 genérico.
//
// Issue #26.
package http

import "net/http"

// SecurityHeadersConfig configura qué cabeceras defensivas aplica el
// middleware. Los ceros son seguros (no aplican); el operador elige.
type SecurityHeadersConfig struct {
	// CSPReportOnly controla si CSP se aplica bloqueando (false) o sólo
	// reportando (true). Útil durante despliegue inicial.
	CSPReportOnly bool
	// HSTSEnabled activa Strict-Transport-Security. Por defecto apagado
	// porque el binario en local corre sobre http://; en prod debe estar
	// detrás de un proxy TLS.
	HSTSEnabled bool
}

// SecurityHeaders añade cabeceras defensivas a cada respuesta.
// Lista: X-Content-Type-Options, X-Frame-Options, Referrer-Policy,
// Content-Security-Policy (SPA), Permissions-Policy, y HSTS opcional.
func SecurityHeaders(cfg SecurityHeadersConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			// CSP mínima para la SPA: self-only, sin scripts inline,
			// sin objetos, frame-ancestors 'none'.
			csp := "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
			if cfg.CSPReportOnly {
				w.Header().Set("Content-Security-Policy-Report-Only", csp)
			} else {
				w.Header().Set("Content-Security-Policy", csp)
			}
			w.Header().Set("Permissions-Policy", "camera=(), geolocation=(), microphone=(), payment=()")
			if cfg.HSTSEnabled {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// BodyLimit aplica http.MaxBytesHandler al handler. Cualquier read
// de r.Body que exceda `maxBytes` devolverá *http.MaxBytesError;
// los handlers deben usar handlers.IsBodyLimitError para responder
// 413. Esta función es el wrapper global — la traducción a 413 es
// responsabilidad del handler (issue #26).
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.MaxBytesHandler(next, maxBytes)
	}
}
