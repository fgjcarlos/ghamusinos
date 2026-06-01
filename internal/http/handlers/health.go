// Package handlers contiene los handlers HTTP del servidor.
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Health responde el estado de vida del servicio. Es una ruta pública usada
// por orquestadores y checks de despliegue.
func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		slog.Error("healthz: fallo al escribir respuesta", "err", err)
	}
}
