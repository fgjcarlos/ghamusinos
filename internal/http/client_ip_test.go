// Tests del comportamiento seguro del middleware ClientIPFromXFF.
// Issue #64: reemplaza middleware.RealIP (SA1019, vulnerable a IP spoofing).
package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// peerAddr es el RemoteAddr que escribe httptest.NewRequestWithContext por defecto
// ("192.0.2.1:1234" según net/http docs). Lo usamos explícitamente para
// que el test no dependa del default.
const peerAddr = "203.0.113.5:54321"

// TestClientIPFromXFF_PeerOutsideTrustedCIDRIgnoresHeader verifica el caso
// crítico de seguridad: si el peer TCP NO pertenece a los CIDRs de
// confianza declarados, el middleware ignora el header X-Forwarded-For
// (sea cual sea su contenido) y deja r.RemoteAddr intacto. Esto cierra la
// vulnerabilidad de IP spoofing que motivaba el deprecation de RealIP.
func TestClientIPFromXFF_PeerOutsideTrustedCIDRIgnoresHeader(t *testing.T) {
	r := chi.NewRouter()
	// Solo declaramos 10.0.0.0/8 como proxy de confianza. El peer del
	// test (203.0.113.5) cae FUERA.
	r.Use(middleware.ClientIPFromXFF("10.0.0.0/8"))

	var capturedRemoteAddr string
	r.Get("/probe", func(w http.ResponseWriter, req *http.Request) {
		capturedRemoteAddr = req.RemoteAddr
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), "GET", "/probe", nil)
	req.RemoteAddr = peerAddr
	// Atacante intenta inyectar una IP interna.
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if capturedRemoteAddr != peerAddr {
		t.Errorf("r.RemoteAddr = %q, quería %q (peer TCP debe prevalecer cuando no está en CIDR de confianza)",
			capturedRemoteAddr, peerAddr)
	}
}

// TestClientIPFromXFF_TrustedPeerStripsXFF verifica el camino feliz: cuando
// el peer TCP SÍ está en los CIDRs de confianza, el middleware resuelve
// el cliente original saltando los proxies de la cadena XFF. El IP
// resuelto se lee con middleware.GetClientIP(ctx), NO con r.RemoteAddr
// (que sigue siendo el peer TCP). El cambio de comportamiento es
// deliberado: pasar de r.RemoteAddr al contexto es la forma idiomática
// de esta API.
func TestClientIPFromXFF_TrustedPeerStripsXFF(t *testing.T) {
	r := chi.NewRouter()
	r.Use(middleware.ClientIPFromXFF("10.0.0.0/8"))

	var resolvedClientIP string
	r.Get("/probe", func(w http.ResponseWriter, req *http.Request) {
		resolvedClientIP = middleware.GetClientIP(req.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Peer TCP = proxy de confianza (10.0.0.5). XFF trae la cadena
	// "10.0.0.5, 198.51.100.7" → el cliente original es 198.51.100.7
	// (right-to-left walk: 198.51.100.7 NO está en 10.0.0.0/8 → es el cliente).
	req := httptest.NewRequestWithContext(t.Context(), "GET", "/probe", nil)
	req.RemoteAddr = "10.0.0.5:443"
	req.Header.Set("X-Forwarded-For", "10.0.0.5, 198.51.100.7")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if !strings.Contains(resolvedClientIP, "198.51.100.7") {
		t.Errorf("GetClientIP = %q, quería que contenga 198.51.100.7 (cliente original tras proxy de confianza)",
			resolvedClientIP)
	}
}

// TestClientIPFromXFF_NoHeaderPreservesRemoteAddr verifica que sin header
// XFF y peer fuera del CIDR de confianza, el RemoteAddr del peer TCP se
// preserva intacto (caso normal: binario expuesto directamente a internet).
func TestClientIPFromXFF_NoHeaderPreservesRemoteAddr(t *testing.T) {
	r := chi.NewRouter()
	r.Use(middleware.ClientIPFromXFF("10.0.0.0/8"))

	var captured string
	r.Get("/probe", func(w http.ResponseWriter, req *http.Request) {
		captured = req.RemoteAddr
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(t.Context(), "GET", "/probe", nil)
	req.RemoteAddr = peerAddr
	// sin X-Forwarded-For
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if captured != peerAddr {
		t.Errorf("r.RemoteAddr = %q, quería %q", captured, peerAddr)
	}
}
