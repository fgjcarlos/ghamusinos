package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/workouts"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// WorkoutStore is the persistence contract used by planned-workout HTTP handlers.
type WorkoutStore interface {
	Create(context.Context, uuid.UUID, workouts.Session) (workouts.Session, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (workouts.Session, error)
	ListRange(context.Context, uuid.UUID, time.Time, time.Time, int32, int32) ([]workouts.Session, int64, error)
	Update(context.Context, uuid.UUID, uuid.UUID, workouts.Session) (workouts.Session, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
	LinkActivity(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (workouts.Session, error)
	UnlinkActivity(context.Context, uuid.UUID, uuid.UUID) (workouts.Session, error)
	Compare(context.Context, uuid.UUID, time.Time, time.Time, workouts.CompareGroup) ([]workouts.ComparePeriod, error)
	CalendarActivities(context.Context, uuid.UUID, time.Time, time.Time) ([]workouts.CalendarActivity, error)
}

const workoutDefaultLimit = 20
const workoutMaxLimit = 100

type workoutDTO struct {
	ID                   uuid.UUID            `json:"id"`
	ScheduledOn          string               `json:"scheduled_on"`
	Title                string               `json:"title"`
	Notes                string               `json:"notes"`
	WorkoutType          workouts.WorkoutType `json:"workout_type"`
	TargetDurationSec    *int                 `json:"target_duration_sec"`
	TargetDistanceM      *int                 `json:"target_distance_m"`
	TargetElevationGainM *int                 `json:"target_elevation_gain_m"`
	TargetIntensity      *workouts.Intensity  `json:"target_intensity"`
	Status               workouts.Status      `json:"status"`
	ActivityID           *uuid.UUID           `json:"activity_id"`
}

func workoutResponse(s workouts.Session) workoutDTO {
	// El estado vacio solo aparece en sesiones recien construidas: en la
	// base siempre hay estado (DEFAULT 'planned').
	status := s.Status
	if status == "" {
		status = workouts.StatusPlanned
	}
	return workoutDTO{ID: s.ID, ScheduledOn: s.ScheduledOn.UTC().Format("2006-01-02"), Title: s.Title, Notes: s.Notes, WorkoutType: s.WorkoutType, TargetDurationSec: s.TargetDurationSec, TargetDistanceM: s.TargetDistanceM, TargetElevationGainM: s.TargetElevationGainM, TargetIntensity: s.TargetIntensity, Status: status, ActivityID: s.ActivityID}
}
func workoutAuth(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id := middleware.GetReqID(r.Context())
	u := auth.AuthUser(r.Context())
	if u == nil {
		WriteProblem(w, NewUnauthorized("not authenticated", id))
		return uuid.Nil, false
	}
	uid, e := uuid.Parse(u.ID)
	if e != nil {
		WriteProblem(w, NewInternalError("invalid user id format", id))
		return uuid.Nil, false
	}
	return uid, true
}
func writeWorkoutJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func decodeWorkoutJSON(r *http.Request, dst any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
func writeWorkoutDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	if IsBodyLimitError(err) {
		WriteProblem(w, NewPayloadTooLarge("request body exceeds the maximum size", middleware.GetReqID(r.Context())))
		return
	}
	workoutProblem(w, r, http.StatusBadRequest, "invalid json body")
}

func workoutProblem(w http.ResponseWriter, r *http.Request, status int, detail string) {
	p := NewBadRequest(detail, middleware.GetReqID(r.Context()))
	if status == http.StatusUnprocessableEntity {
		p = NewUnprocessableEntity(detail, middleware.GetReqID(r.Context()))
	}
	if status == http.StatusNotFound {
		p = NewNotFound(detail, middleware.GetReqID(r.Context()))
	}
	if status == http.StatusInternalServerError {
		p = NewInternalError(detail, middleware.GetReqID(r.Context()))
	}
	WriteProblem(w, p)
}

// workoutConflict escribe el 409 del enlace duplicado (UNIQUE de
// activity_id, issue #311) con el mismo formato RFC 9457 que el resto.
func workoutConflict(w http.ResponseWriter, r *http.Request, detail string) {
	WriteProblem(w, ProblemDetail{
		Type:     "about:blank",
		Title:    "Conflict",
		Status:   http.StatusConflict,
		Detail:   detail,
		Instance: middleware.GetReqID(r.Context()),
	})
}
func parseWorkoutDate(raw string) (time.Time, error) {
	d, e := time.Parse("2006-01-02", raw)
	if e != nil || d.Format("2006-01-02") != raw {
		return time.Time{}, fmt.Errorf("date must use YYYY-MM-DD")
	}
	return d.UTC(), nil
}

// CreateWorkout handles POST /api/v1/workouts.
func CreateWorkout(s WorkoutStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		var b struct {
			ScheduledOn          *string               `json:"scheduled_on"`
			Title                *string               `json:"title"`
			Notes                *string               `json:"notes"`
			WorkoutType          *workouts.WorkoutType `json:"workout_type"`
			TargetDurationSec    *int                  `json:"target_duration_sec"`
			TargetDistanceM      *int                  `json:"target_distance_m"`
			TargetElevationGainM *int                  `json:"target_elevation_gain_m"`
			TargetIntensity      *workouts.Intensity   `json:"target_intensity"`
		}
		if err := decodeWorkoutJSON(r, &b); err != nil {
			writeWorkoutDecodeError(w, r, err)
			return
		}
		if b.ScheduledOn == nil || b.Title == nil || b.WorkoutType == nil {
			workoutProblem(w, r, http.StatusBadRequest, "scheduled_on, title and workout_type are required")
			return
		}
		date, err := parseWorkoutDate(*b.ScheduledOn)
		if err != nil {
			workoutProblem(w, r, http.StatusBadRequest, err.Error())
			return
		}
		v := workouts.Session{ScheduledOn: date, Title: *b.Title, WorkoutType: *b.WorkoutType, TargetDurationSec: b.TargetDurationSec, TargetDistanceM: b.TargetDistanceM, TargetElevationGainM: b.TargetElevationGainM, TargetIntensity: b.TargetIntensity, Status: workouts.StatusPlanned}
		if !workoutIntsFitDatabase(v.TargetDurationSec, v.TargetDistanceM, v.TargetElevationGainM) {
			workoutProblem(w, r, http.StatusUnprocessableEntity, "target values must fit a 32-bit integer")
			return
		}
		if b.Notes != nil {
			v.Notes = *b.Notes
		}
		if err := workouts.Validate(v); err != nil {
			workoutProblem(w, r, http.StatusUnprocessableEntity, err.Error())
			return
		}
		result, err := s.Create(r.Context(), uid, v)
		if err != nil {
			if errors.Is(err, workouts.ErrValidation) {
				workoutProblem(w, r, http.StatusUnprocessableEntity, err.Error())
			} else {
				workoutProblem(w, r, http.StatusInternalServerError, "failed to create workout")
			}
			return
		}
		writeWorkoutJSON(w, http.StatusCreated, workoutResponse(result))
	})
}

// ListWorkouts handles GET /api/v1/workouts.
func ListWorkouts(s WorkoutStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		from, e := parseWorkoutDate(q.Get("from"))
		if e != nil {
			workoutProblem(w, r, http.StatusBadRequest, "from must be a valid YYYY-MM-DD date")
			return
		}
		to, e := parseWorkoutDate(q.Get("to"))
		if e != nil {
			workoutProblem(w, r, http.StatusBadRequest, "to must be a valid YYYY-MM-DD date")
			return
		}
		if to.Before(from) {
			workoutProblem(w, r, http.StatusBadRequest, "to must not be before from")
			return
		}
		limit := workoutDefaultLimit
		if raw := q.Get("limit"); raw != "" {
			n, err := strconv.ParseInt(raw, 10, 32)
			if err != nil || n < 1 || n > workoutMaxLimit {
				workoutProblem(w, r, http.StatusBadRequest, "limit must be between 1 and 100")
				return
			}
			limit = int(n)
		}
		var offset int64
		if raw := q.Get("offset"); raw != "" {
			var err error
			offset, err = strconv.ParseInt(raw, 10, 32)
			if err != nil || offset < 0 || offset > math.MaxInt32 {
				workoutProblem(w, r, http.StatusBadRequest, "offset must be a non-negative integer")
				return
			}
		}
		rows, total, err := s.ListRange(r.Context(), uid, from, to, int32(limit), int32(offset))
		if err != nil {
			workoutProblem(w, r, http.StatusInternalServerError, "failed to list workouts")
			return
		}
		data := make([]workoutDTO, 0, len(rows))
		for _, v := range rows {
			data = append(data, workoutResponse(v))
		}
		writeWorkoutJSON(w, http.StatusOK, map[string]any{"data": data, "total": total, "limit": limit, "offset": offset, "has_next": offset+int64(len(rows)) < total})
	})
}
func workoutID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		workoutProblem(w, r, http.StatusNotFound, "workout not found")
		return uuid.Nil, false
	}
	return id, true
}

// GetWorkout handles GET /api/v1/workouts/{id}.
func GetWorkout(s WorkoutStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		id, ok := workoutID(w, r)
		if !ok {
			return
		}
		v, err := s.Get(r.Context(), uid, id)
		if errors.Is(err, workouts.ErrNotFound) {
			workoutProblem(w, r, http.StatusNotFound, "workout not found")
			return
		}
		if err != nil {
			workoutProblem(w, r, http.StatusInternalServerError, "failed to get workout")
			return
		}
		writeWorkoutJSON(w, http.StatusOK, workoutResponse(v))
	})
}

// PatchWorkout handles PATCH /api/v1/workouts/{id}.
func PatchWorkout(s WorkoutStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		id, ok := workoutID(w, r)
		if !ok {
			return
		}
		var fields map[string]json.RawMessage
		if err := decodeWorkoutJSON(r, &fields); err != nil {
			writeWorkoutDecodeError(w, r, err)
			return
		}
		if len(fields) == 0 {
			workoutProblem(w, r, http.StatusBadRequest, "patch body must not be empty")
			return
		}
		v, err := s.Get(r.Context(), uid, id)
		if errors.Is(err, workouts.ErrNotFound) {
			workoutProblem(w, r, http.StatusNotFound, "workout not found")
			return
		}
		if err != nil {
			workoutProblem(w, r, http.StatusInternalServerError, "failed to get workout")
			return
		}
		v = workouts.NormalizeStatus(v)
		var scheduled string
		for k, raw := range fields {
			switch k {
			case "scheduled_on":
				if string(raw) == "null" || json.Unmarshal(raw, &scheduled) != nil {
					workoutProblem(w, r, http.StatusBadRequest, "scheduled_on must be a date")
					return
				}
				date, e := parseWorkoutDate(scheduled)
				if e != nil {
					workoutProblem(w, r, http.StatusBadRequest, e.Error())
					return
				}
				v.ScheduledOn = date
			case "title":
				if string(raw) == "null" || json.Unmarshal(raw, &v.Title) != nil {
					workoutProblem(w, r, http.StatusBadRequest, "title must be a string")
					return
				}
			case "notes":
				if string(raw) == "null" || json.Unmarshal(raw, &v.Notes) != nil {
					workoutProblem(w, r, http.StatusBadRequest, "notes must be a string")
					return
				}
			case "workout_type":
				if string(raw) == "null" || json.Unmarshal(raw, &v.WorkoutType) != nil {
					workoutProblem(w, r, http.StatusBadRequest, "workout_type must be a string")
					return
				}
			case "target_duration_sec":
				if err := decodeNullableInt(raw, &v.TargetDurationSec); err != nil {
					if errors.Is(err, workouts.ErrValidation) {
						workoutProblem(w, r, http.StatusUnprocessableEntity, err.Error())
					} else {
						workoutProblem(w, r, http.StatusBadRequest, "target_duration_sec must be an integer or null")
					}
					return
				}
			case "target_distance_m":
				if err := decodeNullableInt(raw, &v.TargetDistanceM); err != nil {
					if errors.Is(err, workouts.ErrValidation) {
						workoutProblem(w, r, http.StatusUnprocessableEntity, err.Error())
					} else {
						workoutProblem(w, r, http.StatusBadRequest, "target_distance_m must be an integer or null")
					}
					return
				}
			case "target_elevation_gain_m":
				if err := decodeNullableInt(raw, &v.TargetElevationGainM); err != nil {
					if errors.Is(err, workouts.ErrValidation) {
						workoutProblem(w, r, http.StatusUnprocessableEntity, err.Error())
					} else {
						workoutProblem(w, r, http.StatusBadRequest, "target_elevation_gain_m must be an integer or null")
					}
					return
				}
			case "target_intensity":
				if string(raw) == "null" {
					v.TargetIntensity = nil
				} else {
					var i workouts.Intensity
					if json.Unmarshal(raw, &i) != nil {
						workoutProblem(w, r, http.StatusBadRequest, "target_intensity must be a string or null")
						return
					}
					v.TargetIntensity = &i
				}
			case "status":
				if string(raw) == "null" || json.Unmarshal(raw, &v.Status) != nil {
					workoutProblem(w, r, http.StatusBadRequest, "status must be a string")
					return
				}
			default:
				workoutProblem(w, r, http.StatusBadRequest, "unknown field: "+k)
				return
			}
		}
		if err := workouts.Validate(v); err != nil {
			workoutProblem(w, r, http.StatusUnprocessableEntity, err.Error())
			return
		}
		result, err := s.Update(r.Context(), uid, id, v)
		if errors.Is(err, workouts.ErrNotFound) {
			workoutProblem(w, r, http.StatusNotFound, "workout not found")
			return
		}
		if err != nil {
			workoutProblem(w, r, http.StatusInternalServerError, "failed to update workout")
			return
		}
		writeWorkoutJSON(w, http.StatusOK, workoutResponse(result))
	})
}
func decodeNullableInt(raw json.RawMessage, out **int) error {
	if string(raw) == "null" {
		*out = nil
		return nil
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil {
		return errors.New("invalid integer")
	}
	if n < math.MinInt32 || n > math.MaxInt32 {
		return fmt.Errorf("%w: target value must fit a 32-bit integer", workouts.ErrValidation)
	}
	v := int(n)
	*out = &v
	return nil
}

func workoutIntsFitDatabase(values ...*int) bool {
	for _, value := range values {
		if value != nil && (int64(*value) < math.MinInt32 || int64(*value) > math.MaxInt32) {
			return false
		}
	}
	return true
}

// DeleteWorkout handles DELETE /api/v1/workouts/{id}.
func DeleteWorkout(s WorkoutStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		id, ok := workoutID(w, r)
		if !ok {
			return
		}
		err := s.Delete(r.Context(), uid, id)
		if errors.Is(err, workouts.ErrNotFound) {
			workoutProblem(w, r, http.StatusNotFound, "workout not found")
			return
		}
		if err != nil {
			workoutProblem(w, r, http.StatusInternalServerError, "failed to delete workout")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// LinkWorkoutActivity handles PUT /api/v1/workouts/{id}/activity (issue
// #311): vincula una actividad real del usuario a la sesion. Vincular
// completa la sesion salvo que ya este marcada modified. Una sesion o
// actividad ajena o inexistente es 404 sin distinguir el motivo; una
// actividad ya vinculada a otra sesion es 409.
func LinkWorkoutActivity(s WorkoutStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		id, ok := workoutID(w, r)
		if !ok {
			return
		}
		var b struct {
			ActivityID *string `json:"activity_id"`
		}
		if err := decodeWorkoutJSON(r, &b); err != nil {
			writeWorkoutDecodeError(w, r, err)
			return
		}
		if b.ActivityID == nil {
			workoutProblem(w, r, http.StatusBadRequest, "activity_id is required")
			return
		}
		activityID, err := uuid.Parse(*b.ActivityID)
		if err != nil {
			workoutProblem(w, r, http.StatusBadRequest, "activity_id must be a UUID")
			return
		}
		v, err := s.LinkActivity(r.Context(), uid, id, activityID)
		if errors.Is(err, workouts.ErrNotFound) {
			workoutProblem(w, r, http.StatusNotFound, "workout or activity not found")
			return
		}
		if errors.Is(err, workouts.ErrActivityInUse) {
			workoutConflict(w, r, "activity already linked to another session")
			return
		}
		if err != nil {
			workoutProblem(w, r, http.StatusInternalServerError, "failed to link activity")
			return
		}
		writeWorkoutJSON(w, http.StatusOK, workoutResponse(v))
	})
}

// UnlinkWorkoutActivity handles DELETE /api/v1/workouts/{id}/activity
// (issue #311): desvincula la actividad dejando activity_id a null, sin
// cambiar el estado de la sesion y sin borrar la actividad.
func UnlinkWorkoutActivity(s WorkoutStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		id, ok := workoutID(w, r)
		if !ok {
			return
		}
		v, err := s.UnlinkActivity(r.Context(), uid, id)
		if errors.Is(err, workouts.ErrNotFound) {
			workoutProblem(w, r, http.StatusNotFound, "workout not found")
			return
		}
		if err != nil {
			workoutProblem(w, r, http.StatusInternalServerError, "failed to unlink activity")
			return
		}
		writeWorkoutJSON(w, http.StatusOK, workoutResponse(v))
	})
}

// CompareWorkouts handles GET /api/v1/workouts/compare (issue #312):
// carga planificada frente a carga realizada por periodo de calendario
// en la zona horaria del usuario. El contrato del rango es el mismo del
// listado: fechas civiles estrictas YYYY-MM-DD, inclusivas, from <= to;
// group es obligatorio y vale day o week.
func CompareWorkouts(s WorkoutStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := workoutAuth(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		from, e := parseWorkoutDate(q.Get("from"))
		if e != nil {
			workoutProblem(w, r, http.StatusBadRequest, "from must be a valid YYYY-MM-DD date")
			return
		}
		to, e := parseWorkoutDate(q.Get("to"))
		if e != nil {
			workoutProblem(w, r, http.StatusBadRequest, "to must be a valid YYYY-MM-DD date")
			return
		}
		if to.Before(from) {
			workoutProblem(w, r, http.StatusBadRequest, "to must not be before from")
			return
		}
		var group workouts.CompareGroup
		switch q.Get("group") {
		case string(workouts.CompareGroupDay):
			group = workouts.CompareGroupDay
		case string(workouts.CompareGroupWeek):
			group = workouts.CompareGroupWeek
		default:
			workoutProblem(w, r, http.StatusBadRequest, "group must be day or week")
			return
		}
		periods, err := s.Compare(r.Context(), uid, from, to, group)
		if err != nil {
			workoutProblem(w, r, http.StatusInternalServerError, "failed to compare workouts")
			return
		}
		if periods == nil {
			periods = []workouts.ComparePeriod{}
		}
		writeWorkoutJSON(w, http.StatusOK, struct {
			Periods []workouts.ComparePeriod `json:"periods"`
		}{Periods: periods})
	})
}
