package workouts

import (
	"context"
	"fmt"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// CalendarActivity es una actividad real del usuario tal y como la ve el
// calendario (#313, T3): el dia civil scheduled_on ya esta resuelto en
// la zona horaria del usuario (users.timezone) y linked dice si la
// actividad justifica alguna sesion de ese usuario, este o no en el
// rango visible.
type CalendarActivity struct {
	ID          uuid.UUID
	Name        string
	ScheduledOn string
	Linked      bool
}

// CalendarActivities devuelve las actividades del usuario cuyo dia
// civil (en su zona horaria) cae en la ventana inclusiva [from, to].
// La busqueda SQL usa una ventana UTC acolchada (-1/+2 dias) y el
// filtro exacto es por fecha civil, igual que Compare (#312): una
// medianoche local puede saltarse o repetirse.
func (s *Store) CalendarActivities(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]CalendarActivity, error) {
	loc, err := s.userZone(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListCalendarActivities(ctx, sqlc.ListCalendarActivitiesParams{
		UserID:      toUUID(userID),
		StartedAt:   pgtype.Timestamptz{Time: from.AddDate(0, 0, -1), Valid: true},
		StartedAt_2: pgtype.Timestamptz{Time: to.AddDate(0, 0, 2), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("workouts: calendar activities: %w", err)
	}
	fromKey := from.Format("2006-01-02")
	toKey := to.Format("2006-01-02")
	out := make([]CalendarActivity, 0, len(rows))
	for _, row := range rows {
		key := LocalDate(row.StartedAt.Time, loc).Format("2006-01-02")
		if key < fromKey || key > toKey {
			continue
		}
		out = append(out, CalendarActivity{ID: fromUUID(row.ID), Name: row.Name, ScheduledOn: key, Linked: row.Linked})
	}
	return out, nil
}
