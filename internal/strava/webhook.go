// Package strava (continuación): handlers de webhooks de Fase 1.2 (issue #86).
//
// Estos handlers implementan la recepción y validación de webhooks de Strava:
//
//	POST /api/v1/strava/webhook
//	  → recibe eventos de Strava, valida la firma HMAC-SHA256,
//	    y los encola en la tabla activity_events.
//
//	GET /api/v1/strava/webhook?hub.mode=subscribe&hub.challenge=...&hub.verify_token=...
//	  → responde el hub.challenge para completar el handshake de suscripción.
//
// # Alcance de este handler (issue #86)
//
//   - El handshake hub.challenge funciona y devuelve el challenge verbatim.
//   - La firma HMAC-SHA256 se valida correctamente contra STRAVA_WEBHOOK_SECRET.
//   - Los eventos se encolan en activity_events con deduplicación por external_id.
//   - Los eventos encolados disparan un job River IngestActivityEventWorker (stub en Slice 4).
//
// # Lo que NO hace este handler
//
//   - No procesa el evento inmediatamente (lo hace el job River).
//   - No implementa retry lógic (es responsabilidad de River).
//   - No expone endpoints de "desuscripción" (por ahora).
package strava

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// ActivityEventStore es la interfaz que los handlers de webhook necesitan
// para persistir y encolar eventos. Envuelve *sqlc.Queries + River.
type ActivityEventStore interface {
	EnqueueActivityEvent(ctx context.Context, arg sqlc.EnqueueActivityEventParams) (sqlc.ActivityEvent, error)
	EnqueueRiverJob(ctx context.Context, jobArgs interface{}) error
}

// WebhookPayload es la estructura del JSON que Strava envía en el webhook.
type WebhookPayload struct {
	ObjectType string `json:"object_type"`
	AspectType string `json:"aspect_type"`
	ObjectID   int64  `json:"object_id"`
	ExternalID string `json:"external_id"`
	Athlete    struct {
		ID int64 `json:"id"`
	} `json:"athlete"`
	OwnerID int64 `json:"owner_id"`
}

// verifyHMACSignature verifica que la firma HMAC-SHA256 en el header
// X-Strava-Signature coincida con el hash del cuerpo usando el secret.
// Devuelve error si la firma no es válida.
func verifyHMACSignature(body []byte, signature string, secret string) error {
	if signature == "" {
		return fmt.Errorf("missing X-Strava-Signature header")
	}

	// Strava envía la firma como "v0=<hex>"
	if len(signature) < 3 || signature[:3] != "v0=" {
		return fmt.Errorf("invalid signature format")
	}

	expectedHex := signature[3:] // Strip "v0=" prefix

	// Calcular HMAC-SHA256
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	calculatedHex := hex.EncodeToString(h.Sum(nil))

	// Comparación en tiempo constante para evitar timing attacks
	if !hmac.Equal([]byte(calculatedHex), []byte(expectedHex)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// WebhookHandler devuelve un handler para POST /api/v1/strava/webhook.
// El handler valida la firma HMAC-SHA256, parsea el JSON, y encola el evento
// en activity_events. Un job River IngestActivityEventWorker lo procesa luego.
func WebhookHandler(webhookSecret string, store ActivityEventStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Leer el cuerpo completo para validar la firma
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Verificar firma HMAC
		signature := r.Header.Get("X-Strava-Signature")
		if err := verifyHMACSignature(body, signature, webhookSecret); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Parsear el JSON
		var payload WebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		// Si no hay external_id, rechazar
		if payload.ExternalID == "" {
			http.Error(w, "missing external_id", http.StatusBadRequest)
			return
		}

		// Encolar el evento en la base de datos
		ctx := r.Context()
		var userID pgtype.UUID
		// TODO: cuando implementemos resolución de user_id desde athlete.id, aqui va
		// Por ahora dejamos el user_id como NULL para Slice 5a

		event, err := store.EnqueueActivityEvent(ctx, sqlc.EnqueueActivityEventParams{
			ExternalID: payload.ExternalID,
			UserID:     userID,
			ObjectType: payload.ObjectType,
			AspectType: payload.AspectType,
			ObjectID:   payload.ObjectID,
			RawPayload: body, // Guardamos el JSON crudo como []byte
		})
		if err != nil {
			http.Error(w, "failed to enqueue event", http.StatusInternalServerError)
			return
		}

		// Encolar el job River para procesar el evento
		jobArgs := IngestActivityEventArgs{
			EventID: event.ID.String(),
		}
		if err := store.EnqueueRiverJob(ctx, jobArgs); err != nil {
			http.Error(w, "failed to enqueue job", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

// WebhookChallengeHandler devuelve un handler para GET /api/v1/strava/webhook.
// El handler implementa el handshake hub.challenge de Strava: recibe
// hub.challenge y hub.verify_token en query string, valida el token,
// y devuelve el challenge verbatim.
func WebhookChallengeHandler(verifyToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		challenge := q.Get("hub.challenge")
		token := q.Get("hub.verify_token")
		mode := q.Get("hub.mode")

		// Validar parámetros
		if mode != "subscribe" {
			http.Error(w, "invalid hub.mode", http.StatusBadRequest)
			return
		}

		if token != verifyToken {
			http.Error(w, "invalid hub.verify_token", http.StatusForbidden)
			return
		}

		if challenge == "" {
			http.Error(w, "missing hub.challenge", http.StatusBadRequest)
			return
		}

		// Responder el challenge verbatim
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, challenge)
	}
}

// IngestActivityEventArgs son los argumentos del job River para procesar
// un evento de actividad. Stub para Slice 4; Slice 5a lo implementará.
type IngestActivityEventArgs struct {
	EventID string
}

// Kind retorna el identificador del job type.
func (a IngestActivityEventArgs) Kind() string {
	return "ingest_activity_event"
}
