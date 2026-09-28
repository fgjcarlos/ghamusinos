// Package handlers: user-preferences HTTP handlers. Issue #159.
//
// GET  /api/v1/me/preferences → devuelve las preferencias del usuario
//   autenticado (hr_max, lthr, ftp, level, timezone, ai_enabled).
// PATCH /api/v1/me/preferences → actualiza con validación estricta:
//   valores fuera de rango devuelven 422 con ProblemDetail y
//   cross-field check (lthr > hr_max) produce un aviso no un error.

package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
)

// GetPreferences returns the authenticated user's preferences.
// 200 with the JSON object; 401 if no user in context; 500 otherwise.
func GetPreferences(q sqlc.Querier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.AuthUser(r.Context())
		if user == nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewUnauthorized("not authenticated", requestID)
			WriteProblem(w, problem)
			return
		}

		userID, err := uuid.Parse(user.ID)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewInternalError("invalid user id format", requestID)
			WriteProblem(w, problem)
			return
		}

		row, err := q.GetUserPreferencesByID(r.Context(), pgtype.UUID{Bytes: userID, Valid: true})
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewInternalError("failed to fetch preferences", requestID)
			WriteProblem(w, problem)
			return
		}

		resp := map[string]interface{}{
			"hr_max":     pgIntToNullableNumber(row.HrMax),
			"lthr":       pgIntToNullableNumber(row.Lthr),
			"ftp":        pgIntToNullableNumber(row.Ftp),
			"level":      pgTextToNullable(row.Level),
			"timezone":   row.Timezone,
			"ai_enabled": row.AiEnabled,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck
		json.NewEncoder(w).Encode(resp)
	})
}

// PatchPreferences validates the body and updates the preferences.
// 200 on success; 401 if no user; 422 if any field invalid; 500 otherwise.
func PatchPreferences(q sqlc.Querier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.AuthUser(r.Context())
		if user == nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewUnauthorized("not authenticated", requestID)
			WriteProblem(w, problem)
			return
		}

		userID, err := uuid.Parse(user.ID)
		if err != nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewInternalError("invalid user id format", requestID)
			WriteProblem(w, problem)
			return
		}

		var body preferencesPatch
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewBadRequest("invalid json body", requestID)
			WriteProblem(w, problem)
			return
		}

		if msg := body.validate(); msg != "" {
			requestID := middleware.GetReqID(r.Context())
			problem := NewUnprocessableEntity(msg, requestID)
			WriteProblem(w, problem)
			return
		}

		// Cross-field check: Lthr > HrMax → warning (advisory), not error.
		// Athletes can have odd readings; we don't block them.
		var warning string
		if !isNilInt2(body.Lthr) && !isNilInt2(body.HrMax) && body.Lthr.Int16 > body.HrMax.Int16 {
			warning = "lthr is greater than hr_max — verify the readings"
		}

		params := sqlc.UpdateUserPreferencesParams{
			ID:        pgtype.UUID{Bytes: userID, Valid: true},
			HrMax:     derefInt2(body.HrMax),
			Lthr:      derefInt2(body.Lthr),
			Ftp:       derefInt2(body.Ftp),
			Level:     body.Level,
			Timezone:  body.Timezone,
			AiEnabled: body.AiEnabled,
		}

		if _, err := q.UpdateUserPreferences(r.Context(), params); err != nil {
			requestID := middleware.GetReqID(r.Context())
			problem := NewInternalError("failed to update preferences", requestID)
			WriteProblem(w, problem)
			return
		}

		resp := map[string]interface{}{
			"hr_max":     pgIntToNullableNumber(params.HrMax),
			"lthr":       pgIntToNullableNumber(params.Lthr),
			"ftp":        pgIntToNullableNumber(params.Ftp),
			"level":      pgTextToNullable(params.Level),
			"timezone":   params.Timezone,
			"ai_enabled": params.AiEnabled,
			"warning":    warning,
		}
		if warning == "" {
			delete(resp, "warning")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck
		json.NewEncoder(w).Encode(resp)
	})
}

// preferencesPatch is the decoded request body. Pointer fields map to
// nullable int2 columns; non-pointer fields are required.
type preferencesPatch struct {
	HrMax     *pgtype.Int2 `json:"hr_max"`
	Lthr      *pgtype.Int2 `json:"lthr"`
	Ftp       *pgtype.Int2 `json:"ftp"`
	Level     pgtype.Text  `json:"level"`
	Timezone  string       `json:"timezone"`
	AiEnabled bool         `json:"ai_enabled"`
}

// validate enforces la tabla de reglas de #159. Devuelve el primer
// mensaje de error o cadena vacía si todo es válido.
func (b *preferencesPatch) validate() string {
	if !isNilInt2(b.HrMax) {
		if v := int(b.HrMax.Int16); v < 1 || v > 260 {
			return "hr_max must be between 1 and 260 (or null)"
		}
	}
	if !isNilInt2(b.Lthr) {
		if v := int(b.Lthr.Int16); v < 1 || v > 260 {
			return "lthr must be between 1 and 260 (or null)"
		}
	}
	if !isNilInt2(b.Ftp) {
		if v := int(b.Ftp.Int16); v < 1 || v > 2000 {
			return "ftp must be between 1 and 2000 (or null)"
		}
	}
	if b.Level.Valid {
		switch b.Level.String {
		case "beginner", "intermediate", "advanced":
			// ok
		default:
			return "level must be one of beginner | intermediate | advanced (or null)"
		}
	}
	if b.Timezone == "" {
		return "timezone is required and must be a non-empty IANA name"
	}
	if _, err := time.LoadLocation(b.Timezone); err != nil {
		return "timezone is not a valid IANA name: " + b.Timezone
	}
	return ""
}

func isNilInt2(v *pgtype.Int2) bool {
	return v == nil || !v.Valid
}

// derefInt2 unwraps a nullable *pgtype.Int2 to its pgtype.Int2 value
// (zero value if the pointer is nil). Used when constructing the
// SQLC params struct, whose fields are not pointers.
func derefInt2(v *pgtype.Int2) pgtype.Int2 {
	if v == nil {
		return pgtype.Int2{}
	}
	return *v
}

func pgIntToNullableNumber(v pgtype.Int2) interface{} {
	if !v.Valid {
		return nil
	}
	return v.Int16
}

func pgTextToNullable(v pgtype.Text) interface{} {
	if !v.Valid {
		return nil
	}
	return v.String
}
