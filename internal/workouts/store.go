package workouts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrNotFound se devuelve cuando la sesion no existe o es de otro usuario.
// El handler HTTP responde 404 en ambos casos: la respuesta no distingue
// los dos escenarios.
var ErrNotFound = errors.New("workouts: session not found")

// ErrActivityInUse se devuelve cuando la actividad ya esta vinculada a
// otra sesion: el UNIQUE(activity_id) de la migracion 00017 la rechaza.
// El handler HTTP responde 409.
var ErrActivityInUse = errors.New("workouts: activity already linked to another session")

// Querier es el subconjunto de sqlc.Querier que necesita el store. Es
// deliberadamente estrecho (mismo patron que internal/gpx): permite
// testear el store con un mock de cinco metodos en vez de la interfaz
// completa del proyecto.
type Querier interface {
	CreatePlannedSession(context.Context, sqlc.CreatePlannedSessionParams) (sqlc.PlannedSession, error)
	GetPlannedSession(context.Context, sqlc.GetPlannedSessionParams) (sqlc.PlannedSession, error)
	ListPlannedSessionsByRange(context.Context, sqlc.ListPlannedSessionsByRangeParams) ([]sqlc.ListPlannedSessionsByRangeRow, error)
	UpdatePlannedSession(context.Context, sqlc.UpdatePlannedSessionParams) (sqlc.PlannedSession, error)
	DeletePlannedSession(context.Context, sqlc.DeletePlannedSessionParams) (int64, error)
	LinkPlannedSessionActivity(context.Context, sqlc.LinkPlannedSessionActivityParams) (sqlc.PlannedSession, error)
	UnlinkPlannedSessionActivity(context.Context, sqlc.UnlinkPlannedSessionActivityParams) (sqlc.PlannedSession, error)
	// La comparacion plan vs real (#312) reutiliza queries existentes:
	// la zona del usuario (users.timezone) y las actividades del rango.
	GetUserPreferencesByID(context.Context, pgtype.UUID) (sqlc.GetUserPreferencesByIDRow, error)
	ListActivitiesInRange(context.Context, sqlc.ListActivitiesInRangeParams) ([]sqlc.ListActivitiesInRangeRow, error)
	// La bandeja del calendario (#313, T3) tiene su propia query: la
	// bandera linked vive en el SQL, no se calcula en Go.
	ListCalendarActivities(context.Context, sqlc.ListCalendarActivitiesParams) ([]sqlc.ListCalendarActivitiesRow, error)
}

// Store persiste las sesiones planificadas de un usuario. Todas las
// operaciones reciben el user_id del llamante y lo aplican siempre en la
// query: el aislamiento entre usuarios vive en el SQL, no en el Go.
type Store struct {
	q Querier
}

// NewStore devuelve un Store sobre el querier dado.
func NewStore(q Querier) *Store {
	return &Store{q: q}
}

// Create guarda una sesion nueva para el usuario y devuelve la fila
// persistida (con id y timestamps de la base de datos).
func (s *Store) Create(ctx context.Context, userID uuid.UUID, session Session) (Session, error) {
	session = NormalizeStatus(session)
	if err := Validate(session); err != nil {
		return Session{}, err
	}
	row, err := s.q.CreatePlannedSession(ctx, sqlc.CreatePlannedSessionParams{
		UserID:               toUUID(userID),
		ScheduledOn:          toDate(session.ScheduledOn),
		Title:                session.Title,
		Notes:                toText(session.Notes),
		WorkoutType:          string(session.WorkoutType),
		TargetDurationSec:    toInt4(session.TargetDurationSec),
		TargetDistanceM:      toInt4(session.TargetDistanceM),
		TargetElevationGainM: toInt4(session.TargetElevationGainM),
		TargetIntensity:      toIntensity(session.TargetIntensity),
	})
	if err != nil {
		return Session{}, fmt.Errorf("workouts: create session: %w", err)
	}
	return fromRow(row), nil
}

// Get devuelve la sesion del usuario. Si no existe o pertenece a otro
// usuario devuelve ErrNotFound.
func (s *Store) Get(ctx context.Context, userID, id uuid.UUID) (Session, error) {
	row, err := s.q.GetPlannedSession(ctx, sqlc.GetPlannedSessionParams{
		ID:     toUUID(id),
		UserID: toUUID(userID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("workouts: get session: %w", err)
	}
	return fromRow(row), nil
}

// ListRange devuelve las sesiones del usuario en la ventana [from, to]
// (ambos dias incluidos) y el total de filas que cumplen el filtro.
func (s *Store) ListRange(ctx context.Context, userID uuid.UUID, from, to time.Time, limit, offset int32) ([]Session, int64, error) {
	rows, err := s.q.ListPlannedSessionsByRange(ctx, sqlc.ListPlannedSessionsByRangeParams{
		UserID:   toUUID(userID),
		FromDate: toDate(from),
		ToDate:   toDate(to),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("workouts: list sessions: %w", err)
	}
	if len(rows) == 0 && offset > 0 {
		countRows, countErr := s.q.ListPlannedSessionsByRange(ctx, sqlc.ListPlannedSessionsByRangeParams{
			UserID: toUUID(userID), FromDate: toDate(from), ToDate: toDate(to), Limit: 1, Offset: 0,
		})
		if countErr != nil {
			return nil, 0, fmt.Errorf("workouts: count sessions: %w", countErr)
		}
		if len(countRows) > 0 {
			return []Session{}, countRows[0].TotalCount, nil
		}
		return []Session{}, 0, nil
	}
	sessions := make([]Session, 0, len(rows))
	var total int64
	for _, row := range rows {
		sessions = append(sessions, fromListRow(row))
		total = row.TotalCount
	}
	return sessions, total, nil
}

// Update guarda los cambios de una sesion del usuario. Si la sesion no
// existe o es de otro usuario devuelve ErrNotFound.
func (s *Store) Update(ctx context.Context, userID, id uuid.UUID, session Session) (Session, error) {
	session = NormalizeStatus(session)
	if err := Validate(session); err != nil {
		return Session{}, err
	}
	row, err := s.q.UpdatePlannedSession(ctx, sqlc.UpdatePlannedSessionParams{
		ID:                   toUUID(id),
		UserID:               toUUID(userID),
		ScheduledOn:          toDate(session.ScheduledOn),
		Title:                session.Title,
		Notes:                toText(session.Notes),
		WorkoutType:          string(session.WorkoutType),
		TargetDurationSec:    toInt4(session.TargetDurationSec),
		TargetDistanceM:      toInt4(session.TargetDistanceM),
		TargetElevationGainM: toInt4(session.TargetElevationGainM),
		TargetIntensity:      toIntensity(session.TargetIntensity),
		Status:               string(session.Status),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("workouts: update session: %w", err)
	}
	return fromRow(row), nil
}

// Delete borra una sesion del usuario. Si no existia o era de otro usuario
// devuelve ErrNotFound.
func (s *Store) Delete(ctx context.Context, userID, id uuid.UUID) error {
	rows, err := s.q.DeletePlannedSession(ctx, sqlc.DeletePlannedSessionParams{
		ID:     toUUID(id),
		UserID: toUUID(userID),
	})
	if err != nil {
		return fmt.Errorf("workouts: delete session: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// LinkActivity vincula una actividad real del usuario a una sesion y
// devuelve la sesion actualizada. La query comprueba en la misma
// sentencia que la sesion y la actividad son del usuario: si alguna no
// existe o es ajena devuelve ErrNotFound (404 sin distinguir el motivo).
// Vincular completa la sesion salvo que ya este marcada modified. Si la
// actividad ya justifica otra sesion, el UNIQUE(activity_id) de la
// migracion 00017 devuelve ErrActivityInUse (409).
func (s *Store) LinkActivity(ctx context.Context, userID, sessionID, activityID uuid.UUID) (Session, error) {
	row, err := s.q.LinkPlannedSessionActivity(ctx, sqlc.LinkPlannedSessionActivityParams{
		ID:         toUUID(sessionID),
		UserID:     toUUID(userID),
		ActivityID: toUUID(activityID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // UNIQUE_VIOLATION
		return Session{}, ErrActivityInUse
	}
	if err != nil {
		return Session{}, fmt.Errorf("workouts: link activity: %w", err)
	}
	return fromRow(row), nil
}

// UnlinkActivity desvincula la actividad de una sesion del usuario sin
// tocar el estado ni borrar la actividad. Si la sesion no existe o es de
// otro usuario devuelve ErrNotFound; desvincular una sesion sin enlace es
// un no-op que devuelve la sesion.
func (s *Store) UnlinkActivity(ctx context.Context, userID, sessionID uuid.UUID) (Session, error) {
	row, err := s.q.UnlinkPlannedSessionActivity(ctx, sqlc.UnlinkPlannedSessionActivityParams{
		ID:     toUUID(sessionID),
		UserID: toUUID(userID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("workouts: unlink activity: %w", err)
	}
	return fromRow(row), nil
}

// NormalizeStatus normaliza la ausencia de estado a planned, el DEFAULT
// de la tabla: una sesion recien construida no lleva estado y la base no
// admite vacio.
func NormalizeStatus(session Session) Session {
	if session.Status == "" {
		session.Status = StatusPlanned
	}
	return session
}

// fromRow convierte una fila de sqlc en el tipo de dominio.
func fromRow(row sqlc.PlannedSession) Session {
	return Session{
		ID:                   fromUUID(row.ID),
		UserID:               fromUUID(row.UserID),
		ScheduledOn:          row.ScheduledOn.Time,
		Title:                row.Title,
		Notes:                row.Notes.String,
		WorkoutType:          WorkoutType(row.WorkoutType),
		TargetDurationSec:    fromInt4(row.TargetDurationSec),
		TargetDistanceM:      fromInt4(row.TargetDistanceM),
		TargetElevationGainM: fromInt4(row.TargetElevationGainM),
		TargetIntensity:      fromIntensity(row.TargetIntensity),
		Status:               Status(row.Status),
		ActivityID:           fromUUIDPtr(row.ActivityID),
	}
}

// fromListRow convierte una fila del listado en el tipo de dominio.
func fromListRow(row sqlc.ListPlannedSessionsByRangeRow) Session {
	return Session{
		ID:                   fromUUID(row.ID),
		UserID:               fromUUID(row.UserID),
		ScheduledOn:          row.ScheduledOn.Time,
		Title:                row.Title,
		Notes:                row.Notes.String,
		WorkoutType:          WorkoutType(row.WorkoutType),
		TargetDurationSec:    fromInt4(row.TargetDurationSec),
		TargetDistanceM:      fromInt4(row.TargetDistanceM),
		TargetElevationGainM: fromInt4(row.TargetElevationGainM),
		TargetIntensity:      fromIntensity(row.TargetIntensity),
		Status:               Status(row.Status),
		ActivityID:           fromUUIDPtr(row.ActivityID),
	}
}

// toUUID convierte un uuid.UUID en el pgtype.UUID que espera pgx.
func toUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// fromUUID convierte el pgtype.UUID de la base de datos en uuid.UUID.
func fromUUID(id pgtype.UUID) uuid.UUID {
	return uuid.UUID(id.Bytes)
}

// fromUUIDPtr convierte un UUID opcional de la base de datos a *uuid.UUID
// (nil cuando la columna es NULL).
func fromUUIDPtr(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	out := uuid.UUID(id.Bytes)
	return &out
}

// toDate reduce un instante al dia civil en UTC. La columna es un DATE:
// el dia se resolvio antes (en la zona horaria del usuario) y la base de
// datos no guarda hora.
func toDate(t time.Time) pgtype.Date {
	t = t.UTC()
	return pgtype.Date{
		Time:             time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC),
		Valid:            true,
		InfinityModifier: pgtype.Finite,
	}
}

// toText convierte un texto vacio en NULL: la columna es opcional y "" no
// es lo mismo que "sin notas".
func toText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// toInt4 convierte un entero opcional en el pgtype.Int4 que espera pgx.
func toInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

// fromInt4 convierte un entero opcional de la base de datos a *int.
func fromInt4(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	out := int(v.Int32)
	return &out
}

// toIntensity convierte una zona opcional en el pgtype.Text que espera pgx.
func toIntensity(v *Intensity) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: string(*v), Valid: true}
}

// fromIntensity convierte la zona de la base de datos a *Intensity.
func fromIntensity(v pgtype.Text) *Intensity {
	if !v.Valid {
		return nil
	}
	out := Intensity(v.String)
	return &out
}
