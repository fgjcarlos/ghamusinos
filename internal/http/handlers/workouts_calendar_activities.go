package handlers

import (
	"net/http"

	"github.com/google/uuid"
)

type calendarActivityDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	ScheduledOn string    `json:"scheduled_on"`
	Linked      bool      `json:"linked"`
}

// CalendarActivities handles GET /api/v1/workouts/calendar-activities
// (issue #313, T3): the bounded-by-request activity feed the calendar
// tray and day cells render. from/to follow the same strict civil-date
// contract as the list endpoint; scheduled_on is the user's local ISO
// date and linked marks activities justifying any session of the user,
// inside or outside the visible range.
func CalendarActivities(s WorkoutStore) http.Handler {
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
		rows, err := s.CalendarActivities(r.Context(), uid, from, to)
		if err != nil {
			workoutProblem(w, r, http.StatusInternalServerError, "failed to list calendar activities")
			return
		}
		data := make([]calendarActivityDTO, 0, len(rows))
		for _, a := range rows {
			data = append(data, calendarActivityDTO{ID: a.ID, Name: a.Name, ScheduledOn: a.ScheduledOn, Linked: a.Linked})
		}
		writeWorkoutJSON(w, http.StatusOK, struct {
			Activities []calendarActivityDTO `json:"activities"`
		}{Activities: data})
	})
}
