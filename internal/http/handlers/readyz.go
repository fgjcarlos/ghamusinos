package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

const readyzTimeout = 2 * time.Second

// DBPinger es la interfaz mínima que Readyz necesita del pool de base de datos.
// *pgxpool.Pool la satisface de forma nativa.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// Readyz devuelve un handler HTTP que comprueba la disponibilidad de la base de
// datos haciendo un Ping con un timeout de 2 s.
//
//   - Si pinger es nil o el Ping falla: 503 {"status":"degraded","db":"down"}
//   - Si el Ping tiene éxito:           200 {"status":"ok"}
//
// Content-Type siempre es application/json.
func Readyz(pinger DBPinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if pinger == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			escribirJSON(w, map[string]string{"status": "degraded", "db": "down"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), readyzTimeout)
		defer cancel()

		if err := pinger.Ping(ctx); err != nil {
			slog.Warn("readyz: ping a la base de datos fallido", "err", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			escribirJSON(w, map[string]string{"status": "degraded", "db": "down"})
			return
		}

		w.WriteHeader(http.StatusOK)
		escribirJSON(w, map[string]string{"status": "ok"})
	}
}

// escribirJSON codifica v como JSON y lo escribe en w; registra el error si
// la escritura falla (en ese punto los headers ya se han enviado).
func escribirJSON(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("handlers: fallo al escribir respuesta JSON", "err", err)
	}
}
