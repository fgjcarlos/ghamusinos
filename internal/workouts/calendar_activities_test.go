package workouts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetUserPreferencesByID ya extiende mockQuerier (compare_test.go); la
// bandeja del calendario (#313, T3) anade su propia query dedicada.

func (m *mockQuerier) ListCalendarActivities(_ context.Context, arg sqlc.ListCalendarActivitiesParams) ([]sqlc.ListCalendarActivitiesRow, error) {
	m.calendarArgs = arg
	m.calendarCalls++
	if m.calendarErr != nil {
		return nil, m.calendarErr
	}
	return m.calendarRows, nil
}

func calendarActivityRow(id, userID uuid.UUID, startedAt time.Time, linked bool) sqlc.ListCalendarActivitiesRow {
	return sqlc.ListCalendarActivitiesRow{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		Name:      "Rodaje matinal",
		StartedAt: pgtype.Timestamptz{Time: startedAt, Valid: true},
		Linked:    linked,
	}
}

// TestStoreCalendarActivitiesBucketsInUserZone: scheduled_on es la fecha
// civil en users.timezone (Europe/Madrid). Una actividad a las 23:30Z
// del 25 cae en el 26 local, y la bandera linked pasa tal cual.
func TestStoreCalendarActivitiesBucketsInUserZone(t *testing.T) {
	activityID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	mock := &mockQuerier{
		prefs: sqlc.GetUserPreferencesByIDRow{Timezone: "Europe/Madrid"},
		calendarRows: []sqlc.ListCalendarActivitiesRow{
			calendarActivityRow(activityID, compareUser, time.Date(2026, 11, 25, 23, 30, 0, 0, time.UTC), true),
		},
	}
	from := day(time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC))
	to := day(time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))
	got, err := NewStore(mock).CalendarActivities(context.Background(), compareUser, from, to)
	if err != nil {
		t.Fatalf("CalendarActivities() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("CalendarActivities() = %d rows, want 1", len(got))
	}
	a := got[0]
	if a.ID != activityID || a.Name != "Rodaje matinal" {
		t.Fatalf("CalendarActivities()[0] = %+v, want id %s y nombre Rodaje matinal", a, activityID)
	}
	if a.ScheduledOn != "2026-11-26" {
		t.Fatalf("ScheduledOn = %q, want 2026-11-26 (23:30Z del 25 es el 26 en Madrid)", a.ScheduledOn)
	}
	if !a.Linked {
		t.Fatalf("Linked = false, want true (la query se lo pasa tal cual)")
	}
}

// TestStoreCalendarActivitiesPadsWindowAndFiltersLocal: la ventana SQL va
// acolchada (-1/+2 dias) para no perder medianoches locales, y Go
// recorta por fecha civil exacta: una actividad del 23 local (antes de
// from) y otra del 1 de diciembre local (despues de to) no se devuelven
// aunque esten dentro de la ventana acolchada.
func TestStoreCalendarActivitiesPadsWindowAndFiltersLocal(t *testing.T) {
	before := calendarActivityRow(uuid.New(), compareUser, time.Date(2026, 11, 23, 22, 0, 0, 0, time.UTC), false) // 23:00 del 23 en Madrid
	after := calendarActivityRow(uuid.New(), compareUser, time.Date(2026, 12, 1, 0, 30, 0, 0, time.UTC), false)   // 01:30 del 1 dic en Madrid
	inside := calendarActivityRow(uuid.New(), compareUser, time.Date(2026, 11, 24, 6, 0, 0, 0, time.UTC), false)  // 07:00 del 24 en Madrid
	mock := &mockQuerier{
		prefs:        sqlc.GetUserPreferencesByIDRow{Timezone: "Europe/Madrid"},
		calendarRows: []sqlc.ListCalendarActivitiesRow{before, after, inside},
	}
	from := day(time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC))
	to := day(time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))
	got, err := NewStore(mock).CalendarActivities(context.Background(), compareUser, from, to)
	if err != nil {
		t.Fatalf("CalendarActivities() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != fromUUID(inside.ID) {
		t.Fatalf("CalendarActivities() = %+v, want solo la actividad del 24 local", got)
	}
	// La ventana acolchada es la misma decision que Compare (#312).
	if !mock.calendarArgs.StartedAt.Time.Equal(from.AddDate(0, 0, -1)) {
		t.Fatalf("ventana inicial = %v, want %v", mock.calendarArgs.StartedAt.Time, from.AddDate(0, 0, -1))
	}
	if !mock.calendarArgs.StartedAt_2.Time.Equal(to.AddDate(0, 0, 2)) {
		t.Fatalf("ventana final = %v, want %v", mock.calendarArgs.StartedAt_2.Time, to.AddDate(0, 0, 2))
	}
	if mock.calendarArgs.UserID != toUUID(compareUser) {
		t.Fatalf("query user = %v, want %v", mock.calendarArgs.UserID, toUUID(compareUser))
	}
}

// TestStoreCalendarActivitiesErrors: una zona invalida cae a UTC (no es
// fallo del endpoint) y un error de query se propaga envuelto.
func TestStoreCalendarActivitiesErrors(t *testing.T) {
	mock := &mockQuerier{prefs: sqlc.GetUserPreferencesByIDRow{Timezone: "Not/AZone"}}
	got, err := NewStore(mock).CalendarActivities(context.Background(), compareUser, day(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)), day(time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("zona invalida no debe fallar: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("zona invalida devolvio %d filas, want 0", len(got))
	}
	boom := errors.New("db down")
	mock2 := &mockQuerier{prefs: sqlc.GetUserPreferencesByIDRow{Timezone: "UTC"}, calendarErr: boom}
	if _, err := NewStore(mock2).CalendarActivities(context.Background(), compareUser, day(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)), day(time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))); !errors.Is(err, boom) {
		t.Fatalf("error = %v, want envuelto %v", err, boom)
	}
}
