// Package handlers: HTTP API de los análisis de IA opcionales (fase 1.5,
// T7 — S3/S4/S5/S6/S7).
//
// La API es la única puerta que usa la web para pedir un análisis:
//
//	GET  /api/v1/ai/status     → estado global + consentimiento del usuario
//	POST /api/v1/ai/analyses   → pide un análisis (actividad, semanal, carga)
//	GET  /api/v1/ai/analyses   → lista los análisis del usuario (o uno solo)
//
// Reglas del contrato (cada una con su propio estado HTTP):
//
//  1. La IA global debe estar encendida y disponible; si no, 503.
//  2. El usuario debe haber consentido; se vuelve a comprobar al encolar
//     (S3), si no, 403.
//  3. La actividad debe existir y ser del usuario autenticado; si no, 404.
//  4. El resto de la petición se valida; si no, 422.
//
// Cada lectura y cada escritura va filtrada por `user_id`: un usuario nunca
// ve ni sobrescribe el análisis de otro. El endpoint nunca devuelve la
// credencial, el modelo configurado ni el payload enviado al proveedor.
//
// La API no habla con el proveedor: crea la fila en `pending` y encola el
// job `ai_analysis`, que es la única puerta de salida (ver internal/jobs).

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"github.com/fgjcarlos/ghamusinos/internal/ai"
	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/config"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/jobs"
)

// Límites del contrato de la API de IA.
const (
	// aiDefaultListLimit es el número de análisis que devuelve el
	// listado cuando la petición no pide otro.
	aiDefaultListLimit = 20
	// aiMaxListLimit es el tope del listado: una respuesta nunca trae
	// más de 100 análisis.
	aiMaxListLimit = 100
	// aiDefaultLoadWindowDays es la ventana por defecto del análisis de
	// carga (28 días).
	aiDefaultLoadWindowDays = 28
)

// AIConsentStore es el subconjunto de sqlc.Querier que lee el
// consentimiento del usuario (`users.ai_enabled`).
type AIConsentStore interface {
	GetUserPreferencesByID(ctx context.Context, id pgtype.UUID) (sqlc.GetUserPreferencesByIDRow, error)
}

// AIAnalysisStore es el subconjunto de sqlc.Querier que necesita la API de
// IA. Todas las consultas filtran por `user_id`: ésa es la frontera de
// aislamiento entre usuarios.
type AIAnalysisStore interface {
	AIConsentStore
	GetActivityForUser(ctx context.Context, arg sqlc.GetActivityForUserParams) (sqlc.GetActivityForUserRow, error)
	CreateAIAnalysis(ctx context.Context, arg sqlc.CreateAIAnalysisParams) (sqlc.AiAnalysis, error)
	GetAIAnalysis(ctx context.Context, arg sqlc.GetAIAnalysisParams) (sqlc.AiAnalysis, error)
	ListAIAnalyses(ctx context.Context, arg sqlc.ListAIAnalysesParams) ([]sqlc.AiAnalysis, error)
}

// AIStatusResponse es el cuerpo de GET /api/v1/ai/status.
//
// No lleva la credencial ni el modelo: sólo el nombre del proveedor
// (cuando la IA está disponible) y el estado global.
type AIStatusResponse struct {
	// Enabled es el feature flag global (AI_ENABLED).
	Enabled bool `json:"enabled"`
	// Available indica que la IA se puede usar ahora mismo.
	Available bool `json:"available"`
	// Status es "disabled", "unavailable" o "ready".
	Status string `json:"status"`
	// Provider es el proveedor configurado; nil cuando la IA no está
	// disponible.
	Provider *string `json:"provider"`
	// Consented dice si el USUARIO autenticado ha aceptado el aviso de
	// transferencia.
	Consented bool `json:"consented"`
}

// AIAnalysisOutput es la interpretación validada de un análisis: un
// resumen, unas observaciones y unas cautiones. No hay métricas: el
// proveedor no puede informar de un número que no recibió.
type AIAnalysisOutput struct {
	Summary      string   `json:"summary"`
	Observations []string `json:"observations"`
	Cautions     []string `json:"cautions"`
}

// AIAnalysisResponse es el cuerpo de un análisis.
//
// El payload enviado al proveedor no se expone: sólo la salida validada y,
// cuando el análisis falló, el código y el mensaje seguros.
type AIAnalysisResponse struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Scope  string `json:"scope"`
	Status string `json:"status"`
	// Provider y Model son los que se usaron, si se saben.
	Provider *string `json:"provider"`
	Model    *string `json:"model"`
	// Output sólo está cuando el análisis se completó.
	Output *AIAnalysisOutput `json:"output"`
	// ErrorCode y ErrorMessage sólo están cuando el análisis falló.
	ErrorCode    *string `json:"error_code"`
	ErrorMessage *string `json:"error_message"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// AIAnalysisListResponse es el cuerpo del listado de GET
// /api/v1/ai/analyses.
type AIAnalysisListResponse struct {
	Data  []AIAnalysisResponse `json:"data"`
	Limit int                 `json:"limit"`
}

// aiAnalysisRequest es el cuerpo de POST /api/v1/ai/analyses.
//
// ActivityID sólo tiene sentido para kind="activity"; WindowStart y
// WindowDays sólo para kind="weekly" y kind="load".
type aiAnalysisRequest struct {
	Kind        string `json:"kind"`
	ActivityID  string `json:"activity_id"`
	WindowStart string `json:"window_start"`
	WindowDays  int    `json:"window_days"`
}

// GetAIStatus devuelve el estado de la IA y el consentimiento del usuario
// autenticado.
//
// 200 con el estado; 401 si no hay usuario en el contexto; 500 si no se
// pueden leer las preferencias.
//
// Funciona con la IA apagada: el estado global es "disabled" y el
// consentimiento se informa igual (S7).
func GetAIStatus(q AIConsentStore, cfg config.AIConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.AuthUser(r.Context())
		if user == nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewUnauthorized("not authenticated", requestID))
			return
		}

		userUUID, err := pgUUIDFromUserID(user.ID)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("invalid user id", requestID))
			return
		}

		prefs, err := q.GetUserPreferencesByID(r.Context(), userUUID)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("failed to fetch preferences", requestID))
			return
		}

		resp := AIStatusResponse{
			Enabled:   cfg.Enabled,
			Available: cfg.Available,
			Status:    cfg.Status(),
			Consented: prefs.AiEnabled,
		}
		// El nombre del proveedor sólo se expone cuando la IA está
		// disponible: la credencial y el modelo nunca salen de aquí.
		if cfg.Available && cfg.Provider != "" {
			provider := cfg.Provider
			resp.Provider = &provider
		}

		writeAIJSON(w, http.StatusOK, resp)
	})
}

// PostAIAnalysis pide un análisis y encola el job que lo realiza.
//
// 202 con el análisis creado (en estado "pending"); 401 si no hay usuario;
// 503 si la IA está apagada, no disponible o no hay cola de jobs; 403 si el
// usuario no ha consentido; 404 si la actividad no existe o no es suya;
// 422 si la petición no es válida; 500 en un fallo interno.
//
// El consentimiento se vuelve a comprobar aquí, al encolar (S3): el job lo
// vuelve a comprobar antes de enviar nada.
func PostAIAnalysis(q AIAnalysisStore, inserter jobs.RiverJobInserter, cfg config.AIConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.AuthUser(r.Context())
		if user == nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewUnauthorized("not authenticated", requestID))
			return
		}

		// 1. La IA global tiene que estar encendida y disponible.
		if !cfg.Enabled {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewProblem503("ai analysis unavailable: the global AI flag is disabled", requestID))
			return
		}
		if !cfg.Available {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewProblem503("ai analysis unavailable: the AI provider is not configured", requestID))
			return
		}
		// Sin cola de jobs no hay quien procese la fila: se responde
		// 503 sin crear nada.
		if inserter == nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewProblem503("ai analysis unavailable: the job queue is not configured", requestID))
			return
		}

		userUUID, err := pgUUIDFromUserID(user.ID)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("invalid user id", requestID))
			return
		}

		// 2. El consentimiento, comprobado de nuevo al encolar.
		prefs, err := q.GetUserPreferencesByID(r.Context(), userUUID)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("failed to fetch preferences", requestID))
			return
		}
		if !prefs.AiEnabled {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewForbidden(
				"ai consent required: accept the data transfer notice before requesting an analysis",
				requestID,
			))
			return
		}

		// 3. El cuerpo de la petición.
		var body aiAnalysisRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			requestID := middleware.GetReqID(r.Context())
			if IsBodyLimitError(err) {
				WriteProblem(w, NewPayloadTooLarge("request body too large", requestID))
				return
			}
			WriteProblem(w, NewBadRequest("invalid json body", requestID))
			return
		}

		kind := ai.Kind(strings.ToLower(strings.TrimSpace(body.Kind)))
		switch kind {
		case ai.KindActivity, ai.KindWeekly, ai.KindLoad:
		default:
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewUnprocessableEntityFields(
				"request validation failed",
				requestID,
				map[string]string{"kind": "kind must be one of activity | weekly | load"},
			))
			return
		}

		args := jobs.AIAnalysisArgs{
			UserID:       user.ID,
			AnalysisKind: string(kind),
		}

		switch kind {
		case ai.KindActivity:
			// 4. La actividad tiene que existir y ser del usuario: la
			//    consulta filtra por (id, user_id), así que una
			//    actividad ajena no existe (404) y nunca se analiza.
			activityUUID, err := pgUUIDFromUserID(strings.TrimSpace(body.ActivityID))
			if err != nil {
				requestID := middleware.GetReqID(r.Context())
				WriteProblem(w, NewUnprocessableEntityFields(
					"request validation failed",
					requestID,
					map[string]string{"activity_id": "activity_id is required and must be a valid UUID for kind activity"},
				))
				return
			}
			if body.WindowStart != "" || body.WindowDays != 0 {
				requestID := middleware.GetReqID(r.Context())
				WriteProblem(w, NewUnprocessableEntityFields(
					"request validation failed",
					requestID,
					map[string]string{"window_start": "window_start and window_days are only valid for kind weekly | load"},
				))
				return
			}
			if _, err := q.GetActivityForUser(r.Context(), sqlc.GetActivityForUserParams{
				ID:     activityUUID,
				UserID: userUUID,
			}); err != nil {
				requestID := middleware.GetReqID(r.Context())
				if errors.Is(err, pgx.ErrNoRows) {
					WriteProblem(w, NewNotFound("activity not found", requestID))
					return
				}
				WriteProblem(w, NewInternalError("failed to fetch the activity", requestID))
				return
			}

			activityID := activityUUID.String()
			args.ActivityID = activityID
			args.Scope = ai.ActivityScope(activityID)

		case ai.KindWeekly, ai.KindLoad:
			if strings.TrimSpace(body.ActivityID) != "" {
				requestID := middleware.GetReqID(r.Context())
				WriteProblem(w, NewUnprocessableEntityFields(
					"request validation failed",
					requestID,
					map[string]string{"activity_id": "activity_id is only valid for kind activity"},
				))
				return
			}

			windowDays, fieldErrors := aiWindowDays(kind, body.WindowDays)
			if len(fieldErrors) > 0 {
				requestID := middleware.GetReqID(r.Context())
				WriteProblem(w, NewUnprocessableEntityFields("request validation failed", requestID, fieldErrors))
				return
			}

			start, err := aiWindowStart(body.WindowStart)
			if err != nil {
				requestID := middleware.GetReqID(r.Context())
				WriteProblem(w, NewUnprocessableEntityFields(
					"request validation failed",
					requestID,
					map[string]string{"window_start": "window_start must be a date (YYYY-MM-DD)"},
				))
				return
			}

			args.WindowStart = start.Format(ai.ScopeDateLayout)
			args.WindowDays = windowDays
			if kind == ai.KindWeekly {
				args.Scope = ai.WeeklyScope(start)
			} else {
				args.Scope = ai.LoadScope(start, windowDays)
			}
		}

		// 5. La fila del análisis, en estado "pending".
		row, err := q.CreateAIAnalysis(r.Context(), sqlc.CreateAIAnalysisParams{
			UserID: userUUID,
			Kind:   string(kind),
			Scope:  args.Scope,
		})
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("failed to create the analysis", requestID))
			return
		}

		// 6. El job, con el presupuesto de intentos de la configuración.
		opts := &river.InsertOpts{MaxAttempts: jobs.AIAnalysisMaxAttempts(cfg)}
		if _, err := inserter.Insert(r.Context(), args, opts); err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewProblem503("ai analysis unavailable: failed to enqueue the analysis job", requestID))
			return
		}

		writeAIJSON(w, http.StatusAccepted, aiAnalysisResponse(row))
	})
}

// ListAIAnalyses lista los análisis del usuario autenticado (del más
// reciente al más antiguo) o devuelve uno solo.
//
// Sin parámetros devuelve el listado con un límite acotado (20 por defecto,
// 100 como máximo). Con `kind` y `scope` devuelve ese análisis concreto, o
// 404 si no existe.
//
// 200 con el resultado; 401 si no hay usuario; 422 si los parámetros no son
// válidos; 404 si el análisis pedido no existe; 500 en un fallo interno.
func ListAIAnalyses(q AIAnalysisStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.AuthUser(r.Context())
		if user == nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewUnauthorized("not authenticated", requestID))
			return
		}

		userUUID, err := pgUUIDFromUserID(user.ID)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("invalid user id", requestID))
			return
		}

		query := r.URL.Query()
		kind := strings.TrimSpace(query.Get("kind"))
		scope := strings.TrimSpace(query.Get("scope"))

		// Un análisis concreto: `kind` y `scope` van juntos.
		if kind != "" || scope != "" {
			if kind == "" || scope == "" {
				requestID := middleware.GetReqID(r.Context())
				WriteProblem(w, NewUnprocessableEntityFields(
					"request validation failed",
					requestID,
					map[string]string{"kind": "kind and scope must be provided together"},
				))
				return
			}
			parsedKind := ai.Kind(strings.ToLower(kind))
			switch parsedKind {
			case ai.KindActivity, ai.KindWeekly, ai.KindLoad:
			default:
				requestID := middleware.GetReqID(r.Context())
				WriteProblem(w, NewUnprocessableEntityFields(
					"request validation failed",
					requestID,
					map[string]string{"kind": "kind must be one of activity | weekly | load"},
				))
				return
			}

			row, err := q.GetAIAnalysis(r.Context(), sqlc.GetAIAnalysisParams{
				UserID: userUUID,
				Kind:   string(parsedKind),
				Scope:  scope,
			})
			if err != nil {
				requestID := middleware.GetReqID(r.Context())
				if errors.Is(err, pgx.ErrNoRows) {
					WriteProblem(w, NewNotFound("analysis not found", requestID))
					return
				}
				WriteProblem(w, NewInternalError("failed to fetch the analysis", requestID))
				return
			}
			writeAIJSON(w, http.StatusOK, aiAnalysisResponse(row))
			return
		}

		// El listado, con el límite acotado.
		limit := aiDefaultListLimit
		if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				requestID := middleware.GetReqID(r.Context())
				WriteProblem(w, NewUnprocessableEntityFields(
					"request validation failed",
					requestID,
					map[string]string{"limit": "limit must be an integer"},
				))
				return
			}
			switch {
			case parsed < 1:
				limit = aiDefaultListLimit
			case parsed > aiMaxListLimit:
				limit = aiMaxListLimit
			default:
				limit = parsed
			}
		}

		rows, err := q.ListAIAnalyses(r.Context(), sqlc.ListAIAnalysesParams{
			UserID: userUUID,
			Limit:  int32(limit),
		})
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			WriteProblem(w, NewInternalError("failed to list the analyses", requestID))
			return
		}

		items := make([]AIAnalysisResponse, 0, len(rows))
		for _, row := range rows {
			items = append(items, aiAnalysisResponse(row))
		}
		writeAIJSON(w, http.StatusOK, AIAnalysisListResponse{Data: items, Limit: limit})
	})
}

// aiWindowDays decide el número de días de la ventana y sus errores de
// validación.
//
// El análisis semanal tiene una ventana fija de 7 días (la que usa el job),
// así que un `window_days` distinto no se acepta en lugar de analizarse en
// silencio otra ventana. El análisis de carga acepta de 1 a ai.MaxWindowDays
// días.
func aiWindowDays(kind ai.Kind, requested int) (int, map[string]string) {
	if kind == ai.KindWeekly {
		if requested == 0 {
			return jobs.AIAnalysisWindowDays, nil
		}
		if requested != jobs.AIAnalysisWindowDays {
			return 0, map[string]string{
				"window_days": "window_days for kind weekly must be " + strconv.Itoa(jobs.AIAnalysisWindowDays),
			}
		}
		return requested, nil
	}

	if requested == 0 {
		return aiDefaultLoadWindowDays, nil
	}
	if requested < 1 || requested > ai.MaxWindowDays {
		return 0, map[string]string{
			"window_days": "window_days must be between 1 and " + strconv.Itoa(ai.MaxWindowDays),
		}
	}
	return requested, nil
}

// aiWindowStart deriva la fecha de inicio de la ventana.
//
// Sin `window_start` se usa hoy (UTC). Con `window_start` la fecha tiene que
// ser válida (YYYY-MM-DD).
func aiWindowStart(requested string) (time.Time, error) {
	trimmed := strings.TrimSpace(requested)
	if trimmed == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Parse(ai.ScopeDateLayout, trimmed)
}

// aiAnalysisResponse convierte una fila en la respuesta pública.
//
// El payload enviado al proveedor no sale de aquí: sólo la salida
// validada y, si el análisis falló, el código y el mensaje seguros.
func aiAnalysisResponse(row sqlc.AiAnalysis) AIAnalysisResponse {
	resp := AIAnalysisResponse{
		ID:        row.ID.String(),
		Kind:      row.Kind,
		Scope:     row.Scope,
		Status:    row.Status,
		CreatedAt: aiTimestamp(row.CreatedAt),
		UpdatedAt: aiTimestamp(row.UpdatedAt),
	}
	if row.Provider.Valid {
		provider := row.Provider.String
		resp.Provider = &provider
	}
	if row.Model.Valid {
		model := row.Model.String
		resp.Model = &model
	}
	if row.ErrorCode.Valid {
		code := row.ErrorCode.String
		resp.ErrorCode = &code
	}
	if row.ErrorMessage.Valid {
		message := row.ErrorMessage.String
		resp.ErrorMessage = &message
	}
	if len(row.Output) > 0 {
		var out ai.Output
		if err := json.Unmarshal(row.Output, &out); err == nil {
			resp.Output = &AIAnalysisOutput{
				Summary:      out.Summary,
				Observations: out.Observations,
				Cautions:     out.Cautions,
			}
		}
	}
	return resp
}

// aiTimestamp formatea una marca de tiempo de la base de datos.
func aiTimestamp(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}

// writeAIJSON escribe una respuesta JSON.
func writeAIJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	//nolint:errcheck
	json.NewEncoder(w).Encode(body)
}
